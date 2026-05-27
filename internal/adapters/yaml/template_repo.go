package yaml

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/KazuhaHub/passwall-sub-panel/internal/domain"
	"github.com/KazuhaHub/passwall-sub-panel/internal/ports"
)

// TemplateRepo implements ports.TemplateRepo. Each template is one YAML file
// under config/templates/.
//
// mtime cache: List/Get are on the /sub render hot path. Pre-cache the
// every render did ReadDir + ReadFile + yaml.Unmarshal for every template
// file — a polling fleet at hundreds of req/min was dominated by this
// I/O + parse work. The cache stores the parsed *domain.Template keyed
// by file mtime; an admin edit (via Save) increments mtime so the next
// read picks up the new content without us tracking writes ourselves.
type TemplateRepo struct {
	dir   string
	mu    sync.RWMutex
	cache sync.Map // map[string]templateCacheEntry — key = absolute file path
}

type templateCacheEntry struct {
	mtime time.Time
	value *domain.Template
}

func NewTemplateRepo(configDir string) (*TemplateRepo, error) {
	dir := filepath.Join(configDir, "templates")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &TemplateRepo{dir: dir}, nil
}

type templateFile struct {
	Slug            string   `yaml:"slug"`
	Name            string   `yaml:"name"`
	ClientType      string   `yaml:"client_type"`
	IsDefault       bool     `yaml:"is_default"`
	RuleSets        []string `yaml:"rule_sets"`
	ProxyGroupOrder []string `yaml:"proxy_group_order"`
	Content         string   `yaml:"content"`
}

func (r *TemplateRepo) List(ctx context.Context) ([]*domain.Template, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entries, err := os.ReadDir(r.dir)
	if err != nil {
		return nil, err
	}
	out := []*domain.Template{}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		t, err := r.readFile(filepath.Join(r.dir, e.Name()))
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", e.Name(), err)
		}
		out = append(out, t)
	}
	return out, nil
}

func (r *TemplateRepo) ListPaged(ctx context.Context, p ports.Pagination) ([]*domain.Template, int64, error) {
	all, err := r.List(ctx)
	if err != nil {
		return nil, 0, err
	}
	filtered := all[:0]
	for _, t := range all {
		if keywordMatch(p.Keyword, t.Slug, t.Name, string(t.ClientType)) {
			filtered = append(filtered, t)
		}
	}
	switch p.SortBy {
	case "name":
		sortBy(filtered, p.SortDir, func(a, b *domain.Template) bool { return a.Name < b.Name })
	case "client_type":
		sortBy(filtered, p.SortDir, func(a, b *domain.Template) bool {
			if a.ClientType != b.ClientType {
				return a.ClientType < b.ClientType
			}
			return a.Slug < b.Slug
		})
	default: // "slug" or unrecognized
		sortBy(filtered, p.SortDir, func(a, b *domain.Template) bool { return a.Slug < b.Slug })
	}
	page, total := slicePage(filtered, p)
	return page, total, nil
}

func (r *TemplateRepo) GetBySlug(ctx context.Context, slug string) (*domain.Template, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, err := r.pathOf(slug)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(p); err != nil {
		if os.IsNotExist(err) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return r.readFile(p)
}

// GetDefault returns the template marked IsDefault for the given client type,
// falling back to the first template of that client type, or ErrNotFound.
func (r *TemplateRepo) GetDefault(ctx context.Context, ct domain.ClientType) (*domain.Template, error) {
	all, err := r.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, t := range all {
		if t.ClientType == ct && t.IsDefault {
			return t, nil
		}
	}
	for _, t := range all {
		if t.ClientType == ct {
			return t, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (r *TemplateRepo) Save(ctx context.Context, t *domain.Template) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if t.Slug == "" {
		return fmt.Errorf("%w: template slug empty", domain.ErrValidation)
	}
	p, err := r.pathOf(t.Slug)
	if err != nil {
		return err
	}
	doc := templateFile{
		Slug:            t.Slug,
		Name:            t.Name,
		ClientType:      string(t.ClientType),
		IsDefault:       t.IsDefault,
		RuleSets:        t.RuleSets,
		ProxyGroupOrder: t.ProxyGroupOrder,
		Content:         t.Content,
	}
	if err := writeYAML(p, doc); err != nil {
		return err
	}
	r.cache.Delete(p)
	return nil
}

func (r *TemplateRepo) Delete(ctx context.Context, slug string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, err := r.pathOf(slug)
	if err != nil {
		return err
	}
	r.cache.Delete(p)
	return os.Remove(p)
}

func (r *TemplateRepo) pathOf(slug string) (string, error) {
	return pathForSafeSlug(r.dir, slug, "template")
}

func (r *TemplateRepo) readFile(path string) (*domain.Template, error) {
	// mtime cache: skip ReadFile+Unmarshal when the file on disk matches
	// the cached mtime. Stat is one syscall; the saved work is the
	// ReadFile + the YAML parse, both of which dominate at /sub render
	// rate. A failed Stat falls through to the legacy slow path so we
	// surface whatever the real error is (rather than masking it with a
	// cache-bypass).
	if st, err := os.Stat(path); err == nil {
		if v, ok := r.cache.Load(path); ok {
			entry := v.(templateCacheEntry)
			if entry.mtime.Equal(st.ModTime()) {
				return entry.value, nil
			}
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var doc templateFile
		if err := yaml.Unmarshal(b, &doc); err != nil {
			return nil, err
		}
		t := &domain.Template{
			Slug:            doc.Slug,
			Name:            doc.Name,
			ClientType:      domain.ClientType(doc.ClientType),
			IsDefault:       doc.IsDefault,
			RuleSets:        doc.RuleSets,
			ProxyGroupOrder: doc.ProxyGroupOrder,
			Content:         doc.Content,
		}
		r.cache.Store(path, templateCacheEntry{mtime: st.ModTime(), value: t})
		return t, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc templateFile
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return nil, err
	}
	return &domain.Template{
		Slug:            doc.Slug,
		Name:            doc.Name,
		ClientType:      domain.ClientType(doc.ClientType),
		IsDefault:       doc.IsDefault,
		RuleSets:        doc.RuleSets,
		ProxyGroupOrder: doc.ProxyGroupOrder,
		Content:         doc.Content,
	}, nil
}
