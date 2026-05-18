package mysql

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/KazuhaHub/passwall-sub-panel/internal/domain"
	"github.com/KazuhaHub/passwall-sub-panel/internal/ports"
)

// separatorRepo is the GORM-backed implementation of ports.SeparatorRepo.
// Operates on the `nodes_separator` table — see separatorRow in schema.go
// and domain.SeparatorEntry for the entity shape.
type separatorRepo struct{ db *gorm.DB }

// Static interface assertion so a method signature drift breaks build.
var _ ports.SeparatorRepo = (*separatorRepo)(nil)

func (r *separatorRepo) Create(ctx context.Context, s *domain.SeparatorEntry) error {
	if s.CreatedAt.IsZero() {
		s.CreatedAt = time.Now()
	}
	row := separatorFromDomain(s)
	if err := r.db.WithContext(ctx).Create(row).Error; err != nil {
		return err
	}
	s.ID = row.ID
	return nil
}

func (r *separatorRepo) Update(ctx context.Context, s *domain.SeparatorEntry) error {
	row := separatorFromDomain(s)
	return r.db.WithContext(ctx).Save(row).Error
}

func (r *separatorRepo) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&separatorRow{}, id).Error
}

func (r *separatorRepo) GetByID(ctx context.Context, id int64) (*domain.SeparatorEntry, error) {
	var row separatorRow
	if err := r.db.WithContext(ctx).First(&row, id).Error; err != nil {
		return nil, wrapNotFound(err)
	}
	return row.toDomain(), nil
}

func (r *separatorRepo) List(ctx context.Context) ([]*domain.SeparatorEntry, error) {
	var rows []separatorRow
	if err := r.db.WithContext(ctx).Order("sort_order ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]*domain.SeparatorEntry, 0, len(rows))
	for i := range rows {
		out = append(out, rows[i].toDomain())
	}
	return out, nil
}

func (r *separatorRepo) ListEnabled(ctx context.Context) ([]*domain.SeparatorEntry, error) {
	var rows []separatorRow
	if err := r.db.WithContext(ctx).
		Where("enabled = ?", true).
		Order("sort_order ASC, id ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]*domain.SeparatorEntry, 0, len(rows))
	for i := range rows {
		out = append(out, rows[i].toDomain())
	}
	return out, nil
}
