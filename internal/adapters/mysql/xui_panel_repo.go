package mysql

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/KazuhaHub/passwall-sub-panel/internal/domain"
)

type xuiPanelRepo struct{ db *gorm.DB }

func (r *xuiPanelRepo) List(ctx context.Context) ([]*domain.XUIPanel, error) {
	var rows []xuiPanelRow
	if err := r.db.WithContext(ctx).Order("id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]*domain.XUIPanel, len(rows))
	for i := range rows {
		panel, err := rows[i].toDomain()
		if err != nil {
			return nil, err
		}
		out[i] = panel
	}
	return out, nil
}

func (r *xuiPanelRepo) GetByID(ctx context.Context, id int64) (*domain.XUIPanel, error) {
	var row xuiPanelRow
	if err := r.db.WithContext(ctx).First(&row, id).Error; err != nil {
		return nil, wrapNotFound(err)
	}
	return row.toDomain()
}

func (r *xuiPanelRepo) GetByName(ctx context.Context, name string) (*domain.XUIPanel, error) {
	var row xuiPanelRow
	if err := r.db.WithContext(ctx).Where("name = ?", name).First(&row).Error; err != nil {
		return nil, wrapNotFound(err)
	}
	return row.toDomain()
}

func (r *xuiPanelRepo) Save(ctx context.Context, p *domain.XUIPanel) error {
	if p.Name == "" {
		return fmt.Errorf("%w: panel name required", domain.ErrValidation)
	}
	if p.URL == "" {
		return fmt.Errorf("%w: panel url required", domain.ErrValidation)
	}
	row, err := xuiPanelFromDomain(p)
	if err != nil {
		return err
	}
	if err := r.db.WithContext(ctx).Save(row).Error; err != nil {
		return err
	}
	p.ID = row.ID
	return nil
}

func (r *xuiPanelRepo) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&xuiPanelRow{}, id).Error
}
