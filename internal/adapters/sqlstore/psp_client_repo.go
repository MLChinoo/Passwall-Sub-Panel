package sqlstore

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/KazuhaHub/passwall-sub-panel/internal/domain"
	"github.com/KazuhaHub/passwall-sub-panel/internal/ports"
)

// pspClientRow is the v3.9.0 first-class client table (domain.PSPClient): one
// row per (user, panel, credClass), panel-wide unique by (panel_id, email) to
// mirror 3X-UI's own clients-keyed-by-email model. Credentials are stored (full
// symmetry — render reads them, no on-the-fly derivation); counters mirror
// ownershipRow but per (user,panel,credClass) instead of per (user,node).
type pspClientRow struct {
	ID        int64  `gorm:"primaryKey;autoIncrement"`
	UserID    int64  `gorm:"index;not null"`
	PanelID   int64  `gorm:"not null;uniqueIndex:uk_psp_client,priority:1"`
	Email     string `gorm:"size:255;not null;uniqueIndex:uk_psp_client,priority:2"`
	CredClass int    `gorm:"not null;default:0"`
	UUID      string `gorm:"size:36;not null;default:''"`
	Password  string `gorm:"size:128;not null;default:''"`
	CreatedAt time.Time

	LifetimeUpBytes    int64 `gorm:"default:0"`
	LifetimeDownBytes  int64 `gorm:"default:0"`
	LifetimeTotalBytes int64 `gorm:"default:0"`

	LastRawUpBytes    int64 `gorm:"default:0"`
	LastRawDownBytes  int64 `gorm:"default:0"`
	LastRawTotalBytes int64 `gorm:"default:0"`

	PeriodBaselineUpBytes    int64 `gorm:"default:0"`
	PeriodBaselineDownBytes  int64 `gorm:"default:0"`
	PeriodBaselineTotalBytes int64 `gorm:"default:0"`
}

func (pspClientRow) TableName() string { return "psp_clients" }

// pspClientInboundRow is the attachment junction (domain.PSPClientInbound):
// which inbounds (PSP nodes) a client is attached to, unique per (client, node).
type pspClientInboundRow struct {
	ID           int64  `gorm:"primaryKey;autoIncrement"`
	ClientID     int64  `gorm:"not null;index;uniqueIndex:uk_psp_client_inbound,priority:1"`
	NodeID       int64  `gorm:"not null;uniqueIndex:uk_psp_client_inbound,priority:2"`
	FlowOverride string `gorm:"size:64;not null;default:''"`
	Provisioned  bool   `gorm:"default:false"`
}

func (pspClientInboundRow) TableName() string { return "psp_client_inbounds" }

func pspClientToRow(c *domain.PSPClient) pspClientRow {
	return pspClientRow{
		ID:                       c.ID,
		UserID:                   c.UserID,
		PanelID:                  c.PanelID,
		Email:                    c.Email,
		CredClass:                c.CredClass,
		UUID:                     c.UUID,
		Password:                 c.Password,
		CreatedAt:                c.CreatedAt,
		LifetimeUpBytes:          c.LifetimeUpBytes,
		LifetimeDownBytes:        c.LifetimeDownBytes,
		LifetimeTotalBytes:       c.LifetimeTotalBytes,
		LastRawUpBytes:           c.LastRawUpBytes,
		LastRawDownBytes:         c.LastRawDownBytes,
		LastRawTotalBytes:        c.LastRawTotalBytes,
		PeriodBaselineUpBytes:    c.PeriodBaselineUpBytes,
		PeriodBaselineDownBytes:  c.PeriodBaselineDownBytes,
		PeriodBaselineTotalBytes: c.PeriodBaselineTotalBytes,
	}
}

func rowToPSPClient(r *pspClientRow) *domain.PSPClient {
	return &domain.PSPClient{
		ID:                       r.ID,
		UserID:                   r.UserID,
		PanelID:                  r.PanelID,
		Email:                    r.Email,
		CredClass:                r.CredClass,
		UUID:                     r.UUID,
		Password:                 r.Password,
		CreatedAt:                r.CreatedAt,
		LifetimeUpBytes:          r.LifetimeUpBytes,
		LifetimeDownBytes:        r.LifetimeDownBytes,
		LifetimeTotalBytes:       r.LifetimeTotalBytes,
		LastRawUpBytes:           r.LastRawUpBytes,
		LastRawDownBytes:         r.LastRawDownBytes,
		LastRawTotalBytes:        r.LastRawTotalBytes,
		PeriodBaselineUpBytes:    r.PeriodBaselineUpBytes,
		PeriodBaselineDownBytes:  r.PeriodBaselineDownBytes,
		PeriodBaselineTotalBytes: r.PeriodBaselineTotalBytes,
	}
}

type pspClientRepo struct{ db *gorm.DB }

// Upsert creates the client (panel_id, email) or, if it exists, updates ONLY
// its identity + credential columns — never the traffic counters, which the poll
// owns via UpdateCounters and would otherwise be clobbered by an identity write
// carrying zero counters. A brand-new row is created with whatever counters the
// caller supplies (the migration seeds merged counters this way); an existing
// row keeps its counters + created_at. Returns the row ID.
func (r *pspClientRepo) Upsert(ctx context.Context, c *domain.PSPClient) (int64, error) {
	if c == nil {
		return 0, errors.New("Upsert: nil client")
	}
	var existing pspClientRow
	err := r.db.WithContext(ctx).
		Where("panel_id = ? AND email = ?", c.PanelID, c.Email).
		First(&existing).Error
	switch {
	case err == nil:
		if uerr := r.db.WithContext(ctx).
			Model(&pspClientRow{}).
			Where("id = ?", existing.ID).
			Updates(map[string]any{
				"user_id":    c.UserID,
				"cred_class": c.CredClass,
				"uuid":       c.UUID,
				"password":   c.Password,
			}).Error; uerr != nil {
			return 0, uerr
		}
		return existing.ID, nil
	case errors.Is(err, gorm.ErrRecordNotFound):
		row := pspClientToRow(c)
		if row.CreatedAt.IsZero() {
			row.CreatedAt = time.Now()
		}
		if cerr := r.db.WithContext(ctx).Create(&row).Error; cerr != nil {
			return 0, cerr
		}
		return row.ID, nil
	default:
		return 0, err
	}
}

func (r *pspClientRepo) GetByID(ctx context.Context, id int64) (*domain.PSPClient, error) {
	var row pspClientRow
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return rowToPSPClient(&row), nil
}

func (r *pspClientRepo) GetByEmail(ctx context.Context, panelID int64, email string) (*domain.PSPClient, error) {
	var row pspClientRow
	if err := r.db.WithContext(ctx).
		Where("panel_id = ? AND email = ?", panelID, email).
		First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return rowToPSPClient(&row), nil
}

func (r *pspClientRepo) ListByUser(ctx context.Context, userID int64) ([]*domain.PSPClient, error) {
	var rows []pspClientRow
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("panel_id, cred_class").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]*domain.PSPClient, len(rows))
	for i := range rows {
		out[i] = rowToPSPClient(&rows[i])
	}
	return out, nil
}

func (r *pspClientRepo) DeleteByEmail(ctx context.Context, panelID int64, email string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row pspClientRow
		err := tx.Where("panel_id = ? AND email = ?", panelID, email).First(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil // idempotent
		}
		if err != nil {
			return err
		}
		if err := tx.Where("client_id = ?", row.ID).Delete(&pspClientInboundRow{}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", row.ID).Delete(&pspClientRow{}).Error
	})
}

// SetInbounds reconciles the client's attachment set to the desired nodes via an
// ADDITIVE DIFF (not delete-all-recreate): rows for nodes no longer desired are
// removed, missing nodes are inserted (provisioned=false), and a still-desired
// node's row is kept — preserving its Provisioned flag and only updating
// FlowOverride. This is load-bearing: the shadow dual-write calls SetInbounds on
// every membership resync, and a delete-recreate would clobber the reconcile
// service's per-attachment Provisioned signal (HOLE #7). A node removed and later
// re-added correctly comes back with provisioned=false (a fresh attachment).
func (r *pspClientRepo) SetInbounds(ctx context.Context, clientID int64, inbounds []domain.PSPClientInbound) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing []pspClientInboundRow
		if err := tx.Where("client_id = ?", clientID).Find(&existing).Error; err != nil {
			return err
		}
		curByNode := make(map[int64]pspClientInboundRow, len(existing))
		for _, e := range existing {
			curByNode[e.NodeID] = e
		}
		desiredNodes := make(map[int64]struct{}, len(inbounds))
		for _, in := range inbounds {
			desiredNodes[in.NodeID] = struct{}{}
		}

		// Remove rows whose node is no longer desired.
		for _, e := range existing {
			if _, ok := desiredNodes[e.NodeID]; !ok {
				if err := tx.Where("id = ?", e.ID).Delete(&pspClientInboundRow{}).Error; err != nil {
					return err
				}
			}
		}
		// Insert missing; update flow-only on existing (Provisioned preserved).
		for _, in := range inbounds {
			cur, ok := curByNode[in.NodeID]
			if !ok {
				if err := tx.Create(&pspClientInboundRow{
					ClientID:     clientID,
					NodeID:       in.NodeID,
					FlowOverride: in.FlowOverride,
				}).Error; err != nil {
					return err
				}
				continue
			}
			if cur.FlowOverride != in.FlowOverride {
				if err := tx.Model(&pspClientInboundRow{}).Where("id = ?", cur.ID).
					Update("flow_override", in.FlowOverride).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// MarkInboundProvisioned sets the per-(client, node) Provisioned flag — called by
// the reconcile service only after a GetClient read-back confirms the shared
// client is attached to that node's inbound in 3X-UI. No-op if the attachment
// row doesn't exist.
func (r *pspClientRepo) MarkInboundProvisioned(ctx context.Context, clientID, nodeID int64, provisioned bool) error {
	return r.db.WithContext(ctx).
		Model(&pspClientInboundRow{}).
		Where("client_id = ? AND node_id = ?", clientID, nodeID).
		Update("provisioned", provisioned).Error
}

func (r *pspClientRepo) ListInbounds(ctx context.Context, clientID int64) ([]domain.PSPClientInbound, error) {
	var rows []pspClientInboundRow
	if err := r.db.WithContext(ctx).
		Where("client_id = ?", clientID).
		Order("node_id").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]domain.PSPClientInbound, len(rows))
	for i, row := range rows {
		out[i] = domain.PSPClientInbound{
			ClientID:     row.ClientID,
			NodeID:       row.NodeID,
			FlowOverride: row.FlowOverride,
			Provisioned:  row.Provisioned,
		}
	}
	return out, nil
}

// counterColumns is the narrow column set UpdateCounters writes — same scope as
// OwnershipRepo.UpdateCounters so the traffic poll never clobbers identity /
// credential / attachment state held by other writers.
func pspClientCounterMap(c *domain.PSPClient) map[string]any {
	return map[string]any{
		"lifetime_up_bytes":           c.LifetimeUpBytes,
		"lifetime_down_bytes":         c.LifetimeDownBytes,
		"lifetime_total_bytes":        c.LifetimeTotalBytes,
		"last_raw_up_bytes":           c.LastRawUpBytes,
		"last_raw_down_bytes":         c.LastRawDownBytes,
		"last_raw_total_bytes":        c.LastRawTotalBytes,
		"period_baseline_up_bytes":    c.PeriodBaselineUpBytes,
		"period_baseline_down_bytes":  c.PeriodBaselineDownBytes,
		"period_baseline_total_bytes": c.PeriodBaselineTotalBytes,
	}
}

func (r *pspClientRepo) UpdateCounters(ctx context.Context, c *domain.PSPClient) error {
	if c == nil || c.ID == 0 {
		return errors.New("UpdateCounters: client ID required")
	}
	return r.db.WithContext(ctx).
		Model(&pspClientRow{}).
		Where("id = ?", c.ID).
		Updates(pspClientCounterMap(c)).Error
}

func (r *pspClientRepo) BatchUpdateCounters(ctx context.Context, items []*domain.PSPClient) error {
	if len(items) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, c := range items {
			if c == nil || c.ID == 0 {
				return errors.New("BatchUpdateCounters: client ID required")
			}
			if err := tx.Model(&pspClientRow{}).
				Where("id = ?", c.ID).
				Updates(pspClientCounterMap(c)).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

var _ ports.PSPClientRepo = (*pspClientRepo)(nil)
