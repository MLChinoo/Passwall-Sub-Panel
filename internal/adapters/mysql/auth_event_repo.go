package mysql

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/KazuhaHub/passwall-sub-panel/internal/domain"
	"github.com/KazuhaHub/passwall-sub-panel/internal/ports"
)

// authEventRepo backs the first-class authentication-event log. Mirrors
// auditRepo's shape (Insert / filtered List / DeleteBefore-for-retention).
type authEventRepo struct{ db *gorm.DB }

func (r *authEventRepo) Insert(ctx context.Context, e *domain.AuthEvent) error {
	row := authEventRow{
		UserID:  e.UserID,
		UPN:     e.UPN,
		Method:  string(e.Method),
		Outcome: string(e.Outcome),
		Reason:  e.Reason,
		IP:      e.IP,
		UA:      e.UA,
		At:      e.At,
	}
	if row.At.IsZero() {
		row.At = time.Now()
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return err
	}
	e.ID = row.ID
	e.At = row.At
	return nil
}

func (r *authEventRepo) List(ctx context.Context, filter ports.AuthEventFilter) ([]*domain.AuthEvent, int64, error) {
	q := r.db.WithContext(ctx).Model(&authEventRow{})
	if filter.UserID != nil {
		q = q.Where("user_id = ?", *filter.UserID)
	}
	if filter.Method != "" {
		q = q.Where("method = ?", filter.Method)
	}
	if filter.Outcome != "" {
		q = q.Where("outcome = ?", filter.Outcome)
	}
	if kw := keywordLike(filter.Search); kw != "" {
		q = q.Where(likeCols("upn", "ip", "ua", "reason"), kw, kw, kw, kw)
	}
	if filter.Since != nil {
		q = q.Where("at >= ?", *filter.Since)
	}
	if filter.Until != nil {
		q = q.Where("at <= ?", *filter.Until)
	}
	// `at` is non-unique under bursts, so pin "at, id" (see auditRepo.List).
	if filter.SortBy == "" {
		filter.SortBy = "at"
	}
	if filter.SortDir == "" {
		filter.SortDir = "desc"
	}
	pagedQ := applyPagination(q.Session(&gorm.Session{}), filter.Pagination, authEventSortAllowlist, "at").Order("id DESC")
	var rows []authEventRow
	if err := pagedQ.Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	total, err := inferTotalOrCount(q, filter.Pagination, len(rows))
	if err != nil {
		return nil, 0, err
	}
	out := make([]*domain.AuthEvent, len(rows))
	for i := range rows {
		out[i] = rows[i].toDomain()
	}
	return out, total, nil
}

var authEventSortAllowlist = map[string]string{
	"at": "at",
	"id": "id",
}

func (r *authEventRepo) DeleteBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	res := r.db.WithContext(ctx).Where("at < ?", cutoff).Delete(&authEventRow{})
	return res.RowsAffected, res.Error
}
