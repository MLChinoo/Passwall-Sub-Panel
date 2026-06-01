// Package rollup downsamples the raw 5-minute traffic snapshot tables
// into per-hour UTC delta buckets. The hourly tables are the SOLE source
// for the admin / user traffic charts (HistoryFor / NodeHistoryFor read
// them directly); raw is kept only as a short 7-day window feeding this
// rollup (and the live quota math), while long-window history lives on the
// *_hourly tables, gated by the admin-tunable TrafficHistoryDays setting.
//
// Only the USER and NODE tiers are rolled up — those are the only tiers any
// chart reads. The per-client tier was previously rolled up too but nothing
// ever read client_traffic_snapshots_hourly, so it was pure write-only dead
// storage (the largest table by far); that rollup was removed. Existing
// client_*_hourly rows age out via PruneHourlyBefore.
//
// Why per-hour-UTC + group-in-Go-by-local-TZ at query time (rather than
// pre-bucketing per panel TZ): industry-standard pattern (Prometheus,
// InfluxDB, ClickHouse). Storing UTC keeps the data canonical, and a 24h
// LA day is just a defined UTC range — query layer does the math. The
// alternative (rolling up in panel TZ) bakes the TZ choice into storage,
// which corrupts the data the moment anyone queries from a different TZ.
//
// Design:
//   - Aggregation: each (entity, UTC-hour) bucket's delta is MAX-floor, where
//     floor is the LOWER of the bucket's own MIN and — when the immediately-
//     preceding hour for the same entity is present — that previous hour's MAX
//     ("carry-in"). The raw snapshots store *Lifetime* cumulative counters
//     (monotonic, managed by internal/service/traffic's LastRaw delta logic),
//     so MAX-MIN within a bucket is the in-hour delta; carrying the previous
//     hour's MAX additionally captures the traffic that accrued ACROSS the hour
//     boundary (between the last sample of one hour and the first of the next),
//     which a plain per-bucket MAX-MIN silently drops (~one poll interval per
//     hour, ≈8% on a 5-min cadence). A counter reset across the boundary
//     (prevMax > thisMin) makes floor fall back to thisMin so the delta can't
//     go negative; a non-adjacent previous bucket (a multi-hour data gap) is
//     NOT used as carry-in — we can't know when that gap's traffic happened, so
//     we keep the conservative in-hour MAX-MIN.
//   - Idempotency: each *_hourly table has UNIQUE(entity_id, bucket_start),
//     and writes go through GORM's OnConflict clause ("upsert"). Re-running
//     against the same raw rows is a no-op (or harmlessly upserts the same
//     values).
//   - Two passes: RollupOnce re-emits EVERY computed bucket (the hourly
//     cleanup loop's rollup-before-prune pass + first-run backfill). RollupRecent
//     emits only buckets in the last few hours — called every traffic poll so
//     the chart's "today" stays live without re-upserting the entire raw window
//     every 5 minutes. Both compute carry-in from the full raw set, so
//     RollupRecent's recent buckets are just as accurate; RollupRecent simply
//     skips writing the older, already-final buckets.
//   - Boundary: we roll up through the CURRENT UTC hour (including the partial,
//     still-filling hour). Because every write is an idempotent upsert, the
//     current hour's bucket simply grows toward its final delta on each pass —
//     so the chart's "today" stays live (≤ one poll interval stale). The
//     whole-hour-aligned raw prune (7d) never touches the current hour.
package rollup

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/KazuhaHub/passwall-sub-panel/internal/pkg/log"
)

// rollupRecentEmitWindow bounds how far back RollupRecent (the per-poll pass)
// re-upserts buckets. Anything older than this is left to the hourly cleanup
// pass — it's already final, so re-writing it every 5 minutes is pure write
// amplification. A few hours of overlap absorbs delayed/missed poll cycles.
const rollupRecentEmitWindow = 3 * time.Hour

// Service is the rollup orchestrator. One instance per process; safe for
// concurrent ticks because every write goes through an idempotent upsert.
type Service struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Service {
	return &Service{db: db}
}

// RollupOnce aggregates the raw user/node tables into their *_hourly tables,
// re-emitting EVERY bucket. Idempotent (upsert). Used by the hourly cleanup
// loop (rollup-before-prune) and first-run backfill.
func (s *Service) RollupOnce(ctx context.Context) error {
	return s.rollup(ctx, time.Time{})
}

// RollupRecent aggregates the same tables but only re-emits buckets within the
// last rollupRecentEmitWindow. Called every traffic poll to keep the open hour
// (the chart's "today") live without re-upserting the whole raw window each
// cycle. Carry-in is still computed from the full raw set, so recent buckets
// are identical to what RollupOnce would write.
func (s *Service) RollupRecent(ctx context.Context) error {
	return s.rollup(ctx, time.Now().Add(-rollupRecentEmitWindow))
}

// rollup runs the user + node passes. emitSince bounds which buckets are
// upserted (zero = all); see emitRows.
func (s *Service) rollup(ctx context.Context, emitSince time.Time) error {
	// Include the current, still-filling hour: rows captured up to now. The
	// idempotent upsert + per-poll cadence keep the open hour's bucket current.
	cutoff := time.Now()

	if userRows, err := s.rollupUser(ctx, cutoff, emitSince); err != nil {
		return fmt.Errorf("user rollup: %w", err)
	} else if userRows > 0 {
		log.Info("traffic rollup (user)", "buckets_upserted", userRows)
	}

	if nodeRows, err := s.rollupNode(ctx, cutoff, emitSince); err != nil {
		return fmt.Errorf("node rollup: %w", err)
	} else if nodeRows > 0 {
		log.Info("traffic rollup (node)", "buckets_upserted", nodeRows)
	}
	return nil
}

// ---- shared aggregation ---------------------------------------------------

// rawSample is one cumulative-counter reading for some entity (user or node).
type rawSample struct {
	entityID   int64
	upBytes    int64
	downBytes  int64
	totalBytes int64
	capturedAt time.Time
}

type entityHourKey struct {
	id     int64
	bucket time.Time
}

type hourAgg struct {
	minUp, maxUp, minDown, maxDown, minTotal, maxTotal int64
	seen                                               bool
}

// aggregate folds raw samples into per-(entity, UTC-hour) min/max boxes,
// skipping rows at/after cutoff.
//
// Cutoff filtering happens in Go rather than SQL: the captured_at column
// roundtrips through GORM as a time.Time, but the SQLite driver stores times
// as TZ-offset-bearing strings whose lexicographic order doesn't match the
// semantic order when values straddle TZ boundaries. raw is bounded to a small
// window (default 7 days) so the table scan is cheap; doing the filter
// post-fetch gives unambiguous time.Time.Before() semantics.
func aggregate(raws []rawSample, cutoff time.Time) map[entityHourKey]*hourAgg {
	buckets := make(map[entityHourKey]*hourAgg, len(raws)/4+1)
	for _, r := range raws {
		if !r.capturedAt.Before(cutoff) {
			continue
		}
		k := entityHourKey{id: r.entityID, bucket: hourFloor(r.capturedAt)}
		b := buckets[k]
		if b == nil {
			b = &hourAgg{}
			buckets[k] = b
		}
		if !b.seen {
			b.minUp, b.maxUp = r.upBytes, r.upBytes
			b.minDown, b.maxDown = r.downBytes, r.downBytes
			b.minTotal, b.maxTotal = r.totalBytes, r.totalBytes
			b.seen = true
			continue
		}
		if r.upBytes < b.minUp {
			b.minUp = r.upBytes
		}
		if r.upBytes > b.maxUp {
			b.maxUp = r.upBytes
		}
		if r.downBytes < b.minDown {
			b.minDown = r.downBytes
		}
		if r.downBytes > b.maxDown {
			b.maxDown = r.downBytes
		}
		if r.totalBytes < b.minTotal {
			b.minTotal = r.totalBytes
		}
		if r.totalBytes > b.maxTotal {
			b.maxTotal = r.totalBytes
		}
	}
	return buckets
}

// emitRows turns the per-hour min/max boxes into upsert rows of in-hour deltas,
// threading the previous hour's MAX as a carry-in floor (see the package doc).
// idCol is "user_id" or "node_id". Only buckets with bucket_start >= emitSince
// are returned (zero emitSince = all). Zero-traffic buckets are dropped (the
// chart treats a missing bucket as zero, so this is lossless).
func emitRows(buckets map[entityHourKey]*hourAgg, idCol string, emitSince time.Time) []map[string]any {
	rows := make([]map[string]any, 0, len(buckets))
	for k, b := range buckets {
		if !emitSince.IsZero() && k.bucket.Before(emitSince) {
			continue
		}
		var prevUp, prevDown, prevTotal *int64
		if prev := buckets[entityHourKey{id: k.id, bucket: k.bucket.Add(-time.Hour)}]; prev != nil {
			prevUp, prevDown, prevTotal = &prev.maxUp, &prev.maxDown, &prev.maxTotal
		}
		up := hourDelta(b.maxUp, b.minUp, prevUp)
		down := hourDelta(b.maxDown, b.minDown, prevDown)
		total := hourDelta(b.maxTotal, b.minTotal, prevTotal)
		if up == 0 && down == 0 && total == 0 {
			continue
		}
		rows = append(rows, map[string]any{
			idCol:          k.id,
			"bucket_start": k.bucket,
			"up_bytes":     up,
			"down_bytes":   down,
			"total_bytes":  total,
		})
	}
	return rows
}

// hourDelta returns max - floor, where floor is the lower of the bucket's MIN
// and the previous adjacent hour's MAX (carry-in). A reset across the boundary
// (prevMax > min) keeps floor at min so the result can't go negative.
func hourDelta(max, min int64, prevMax *int64) int64 {
	floor := min
	if prevMax != nil && *prevMax < floor {
		floor = *prevMax
	}
	if d := max - floor; d > 0 {
		return d
	}
	return 0
}

// ---- user-level rollup ----------------------------------------------------

type userRawRow struct {
	UserID     int64
	UpBytes    int64
	DownBytes  int64
	TotalBytes int64
	CapturedAt time.Time
}

func (s *Service) rollupUser(ctx context.Context, cutoff, emitSince time.Time) (int64, error) {
	var raws []userRawRow
	if err := s.db.WithContext(ctx).
		Table("traffic_snapshots").
		Select("user_id, up_bytes, down_bytes, total_bytes, captured_at").
		Find(&raws).Error; err != nil {
		return 0, err
	}
	if len(raws) == 0 {
		return 0, nil
	}
	samples := make([]rawSample, len(raws))
	for i, r := range raws {
		samples[i] = rawSample{entityID: r.UserID, upBytes: r.UpBytes, downBytes: r.DownBytes, totalBytes: r.TotalBytes, capturedAt: r.CapturedAt}
	}
	rows := emitRows(aggregate(samples, cutoff), "user_id", emitSince)
	return s.upsert(ctx, "traffic_snapshots_hourly", []string{"user_id", "bucket_start"}, rows)
}

// ---- node-level rollup ----------------------------------------------------

type nodeRawRow struct {
	NodeID     int64
	UpBytes    int64
	DownBytes  int64
	TotalBytes int64
	CapturedAt time.Time
}

func (s *Service) rollupNode(ctx context.Context, cutoff, emitSince time.Time) (int64, error) {
	var raws []nodeRawRow
	if err := s.db.WithContext(ctx).
		Table("node_traffic_snapshots").
		Select("node_id, up_bytes, down_bytes, total_bytes, captured_at").
		Find(&raws).Error; err != nil {
		return 0, err
	}
	if len(raws) == 0 {
		return 0, nil
	}
	samples := make([]rawSample, len(raws))
	for i, r := range raws {
		samples[i] = rawSample{entityID: r.NodeID, upBytes: r.UpBytes, downBytes: r.DownBytes, totalBytes: r.TotalBytes, capturedAt: r.CapturedAt}
	}
	rows := emitRows(aggregate(samples, cutoff), "node_id", emitSince)
	return s.upsert(ctx, "node_traffic_snapshots_hourly", []string{"node_id", "bucket_start"}, rows)
}

// upsert pushes the batched rows through GORM's OnConflict clause so MySQL
// uses "ON DUPLICATE KEY UPDATE" and SQLite uses "ON CONFLICT(...) DO
// UPDATE" — both keyed by the unique-index columns supplied. Idempotent.
func (s *Service) upsert(ctx context.Context, table string, conflictCols []string, rows []map[string]any) (int64, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	conflict := make([]clause.Column, len(conflictCols))
	for i, c := range conflictCols {
		conflict[i] = clause.Column{Name: c}
	}
	res := s.db.WithContext(ctx).
		Table(table).
		Clauses(clause.OnConflict{
			Columns:   conflict,
			DoUpdates: clause.AssignmentColumns([]string{"up_bytes", "down_bytes", "total_bytes"}),
		}).
		CreateInBatches(rows, 500)
	return res.RowsAffected, res.Error
}

// hourFloor truncates t to the start of its containing UTC hour. The
// rollup pipeline canonicalises everything to UTC; per-TZ query bucketing
// happens at read time.
func hourFloor(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, time.UTC)
}
