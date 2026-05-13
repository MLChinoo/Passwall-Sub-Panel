package mysql

import (
	"context"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/KazuhaHub/passwall-sub-panel/internal/domain"
)

type ruleSetRepo struct{ db *gorm.DB }

func (r *ruleSetRepo) List(ctx context.Context) ([]*domain.RuleSet, error) {
	var rows []ruleSetRow
	if err := r.db.WithContext(ctx).Order("sort ASC, slug ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]*domain.RuleSet, len(rows))
	for i := range rows {
		out[i] = ruleSetToDomain(&rows[i])
	}
	return out, nil
}

func (r *ruleSetRepo) GetBySlug(ctx context.Context, slug string) (*domain.RuleSet, error) {
	var row ruleSetRow
	if err := r.db.WithContext(ctx).Where("slug = ?", slug).First(&row).Error; err != nil {
		return nil, wrapNotFound(err)
	}
	return ruleSetToDomain(&row), nil
}

func (r *ruleSetRepo) Save(ctx context.Context, rs *domain.RuleSet) error {
	if rs.Slug == "" {
		return fmt.Errorf("%w: rule set slug empty", domain.ErrValidation)
	}
	row := ruleSetRow{
		Slug:    rs.Slug,
		Name:    rs.Name,
		Sort:    rs.Sort,
		Enabled: rs.Enabled,
		Content: rs.Content,
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "slug"}},
		UpdateAll: true,
	}).Create(&row).Error
}

func (r *ruleSetRepo) Delete(ctx context.Context, slug string) error {
	return r.db.WithContext(ctx).Where("slug = ?", slug).Delete(&ruleSetRow{}).Error
}

func ruleSetToDomain(row *ruleSetRow) *domain.RuleSet {
	return &domain.RuleSet{
		Slug:    row.Slug,
		Name:    row.Name,
		Sort:    row.Sort,
		Enabled: row.Enabled,
		Content: row.Content,
	}
}
