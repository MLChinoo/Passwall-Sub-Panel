package mysql

import (
	"context"

	"gorm.io/gorm"

	"github.com/KazuhaHub/passwall-sub-panel/internal/domain"
)

type ownershipRepo struct{ db *gorm.DB }

func (r *ownershipRepo) Add(ctx context.Context, e *domain.XUIClientEntry) error {
	row := ownershipFromDomain(e)
	if err := r.db.WithContext(ctx).Create(row).Error; err != nil {
		return err
	}
	e.ID = row.ID
	e.CreatedAt = row.CreatedAt
	return nil
}

func (r *ownershipRepo) Remove(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&ownershipRow{}, id).Error
}

func (r *ownershipRepo) RemoveByMatch(ctx context.Context, panelID int64, inboundID int, email string) error {
	return r.db.WithContext(ctx).
		Where("panel_id = ? AND inbound_id = ? AND client_email = ?", panelID, inboundID, email).
		Delete(&ownershipRow{}).Error
}

func (r *ownershipRepo) GetByMatch(ctx context.Context, panelID int64, inboundID int, email string) (*domain.XUIClientEntry, error) {
	var row ownershipRow
	err := r.db.WithContext(ctx).
		Where("panel_id = ? AND inbound_id = ? AND client_email = ?", panelID, inboundID, email).
		First(&row).Error
	if err != nil {
		return nil, wrapNotFound(err)
	}
	return row.toDomain(), nil
}

func (r *ownershipRepo) ListByUser(ctx context.Context, userID int64) ([]*domain.XUIClientEntry, error) {
	var rows []ownershipRow
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]*domain.XUIClientEntry, len(rows))
	for i := range rows {
		out[i] = rows[i].toDomain()
	}
	return out, nil
}

func (r *ownershipRepo) ListByInbound(ctx context.Context, panelID int64, inboundID int) ([]*domain.XUIClientEntry, error) {
	var rows []ownershipRow
	err := r.db.WithContext(ctx).
		Where("panel_id = ? AND inbound_id = ?", panelID, inboundID).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]*domain.XUIClientEntry, len(rows))
	for i := range rows {
		out[i] = rows[i].toDomain()
	}
	return out, nil
}

func (r *ownershipRepo) UpdateUUID(ctx context.Context, panelID int64, inboundID int, email, newUUID string) error {
	return r.db.WithContext(ctx).Model(&ownershipRow{}).
		Where("panel_id = ? AND inbound_id = ? AND client_email = ?", panelID, inboundID, email).
		Update("client_uuid", newUUID).Error
}

func (r *ownershipRepo) UpdatePanelName(ctx context.Context, panelID int64, panelName string) error {
	return r.db.WithContext(ctx).
		Model(&ownershipRow{}).
		Where("panel_id = ?", panelID).
		Update("panel_name", panelName).Error
}

func (r *ownershipRepo) Exists(ctx context.Context, panelID int64, inboundID int, email string) (bool, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&ownershipRow{}).
		Where("panel_id = ? AND inbound_id = ? AND client_email = ?", panelID, inboundID, email).
		Count(&n).Error
	return n > 0, err
}
