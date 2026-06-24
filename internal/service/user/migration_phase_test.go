package user

import (
	"context"
	"testing"

	"github.com/KazuhaHub/passwall-sub-panel/internal/ports"
)

type phaseOwnership struct {
	ports.OwnershipRepo
	pending []int64
	err     error
}

func (o phaseOwnership) DistinctUserIDs(context.Context) ([]int64, error) {
	return o.pending, o.err
}

// SharedMigrationComplete reports whether any user still sits on the legacy per-node
// ownership model. It drives the reconcile loop's heal cadence: while incomplete the
// heavy shared-client heal runs every tick (converge fast); once complete it drops to
// a slow drift backstop.
func TestSharedMigrationComplete(t *testing.T) {
	ctx := context.Background()

	// Un-migrated users present → NOT complete.
	mig := &Service{ownership: phaseOwnership{pending: []int64{1, 2}}}
	if done, err := mig.SharedMigrationComplete(ctx); err != nil || done {
		t.Fatalf("with un-migrated users: done=%v err=%v, want done=false", done, err)
	}

	// Zero un-migrated users (table emptied or dropped) → complete.
	doneSvc := &Service{ownership: phaseOwnership{pending: nil}}
	if done, err := doneSvc.SharedMigrationComplete(ctx); err != nil || !done {
		t.Fatalf("with no un-migrated users: done=%v err=%v, want done=true", done, err)
	}

	// Nil ownership (shared model not wired / fresh install) → complete (nothing to migrate).
	nilSvc := &Service{}
	if done, err := nilSvc.SharedMigrationComplete(ctx); err != nil || !done {
		t.Fatalf("nil ownership: done=%v err=%v, want done=true", done, err)
	}
}
