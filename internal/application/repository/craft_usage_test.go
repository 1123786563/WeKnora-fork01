package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/modules/commercial"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// openCraftUsageDB applies the real SQLite migrations (000043_craft_usage
// included); a subtest named "postgres" runs the same assertions against an
// isolated PostgreSQL schema when TRPC_TEST_POSTGRES_DSN is set.
func openCraftUsageDB(t *testing.T) *gorm.DB {
	t.Helper()
	return openRunTestDB(t)
}

// usageFact builds one valid physical-attempt observation.
func usageFact(tenant uint64, runID, delegationID, callID, attemptID, runtime string) craft.UsageFact {
	return craft.UsageFact{
		RunID:        runID,
		DelegationID: delegationID,
		CallID:       callID,
		AttemptID:    attemptID,
		Runtime:      runtime,
		ModelID:      "gpt-test",
		Funding:      commercial.FundingPlatform,
		Status:       craft.UsageStatusReported,
		TenantID:     tenant,
		Input:        120,
		Output:       30,
		Cached:       20,
	}
}

// TestCraftUsageStoreAppendCountsPhysicalAttemptsOnce: appending the same
// physical attempt observation again (redelivery, worker replay) keeps ONE
// row and ONE delivery event; a different observation under the same
// identity is a conflict that must travel through Correct.
func TestCraftUsageStoreAppendCountsPhysicalAttemptsOnce(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			db := openCraftUsageDB(t)
			store := NewCraftUsageStore(db)
			ctx := context.Background()
			fact := usageFact(1, "run-1", "", "call-1", "att-1", craft.RuntimeMain)

			require.NoError(t, store.Append(ctx, fact))
			require.NoError(t, store.Append(ctx, fact), "redelivery must be idempotent")

			var rows int64
			require.NoError(t, db.Table("craft_usage_facts").
				Where("tenant_id = ?", uint64(1)).Count(&rows).Error)
			require.EqualValues(t, 1, rows)

			changed := fact
			changed.Output = 99
			err := store.Append(ctx, changed)
			require.ErrorIs(t, err, craft.ErrConflict, "changed observation must not overwrite")
			require.NoError(t, db.Table("craft_usage_facts").
				Where("tenant_id = ?", uint64(1)).Count(&rows).Error)
			require.EqualValues(t, 1, rows)

			facts, err := store.Facts(ctx, 1, "run-1")
			require.NoError(t, err)
			require.Len(t, facts, 1)
			require.Equal(t, fact.Output, facts[0].Output, "original observation kept")
			require.Equal(t, craft.UsageKey(1, "call-1", "att-1"), facts[0].ID)

			events, err := store.ListUsageEvents(ctx, 1, "call-1", "att-1")
			require.NoError(t, err)
			require.Len(t, events, 1, "one delivery event per revision")
			require.Equal(t, "usage:"+facts[0].ID+":1", events[0].EventKey)
		})
	}
}

// TestCraftUsageStoreCorrectionAppendsRevisionNeverOverwrites: a stream-break
// unknown stays recorded as revision 1 with no token counts, the late usage
// arrives as a corrected revision 2, both revisions stay readable (the
// reconciliation record), and the attempt still counts as ONE physical fact.
func TestCraftUsageStoreCorrectionAppendsRevisionNeverOverwrites(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			db := openCraftUsageDB(t)
			store := NewCraftUsageStore(db)
			ctx := context.Background()

			broken := usageFact(1, "run-1", "dlg-1", "call-1", "att-1", craft.RuntimeOC)
			broken.Input, broken.Output, broken.Cached = 0, 0, 0
			broken.Status = craft.UsageStatusUnknown
			require.NoError(t, store.Append(ctx, broken))

			late := broken
			late.Input, late.Output, late.Cached = 200, 80, 40
			late.Status = craft.UsageStatusCorrected
			require.NoError(t, store.Correct(ctx, late))
			require.NoError(t, store.Correct(ctx, late), "correction replay is idempotent")

			revisions, err := store.Revisions(ctx, 1, "call-1", "att-1")
			require.NoError(t, err)
			require.Len(t, revisions, 2, "revision trail preserved")
			require.Equal(t, craft.UsageStatusUnknown, revisions[0].Status)
			require.Zero(t, revisions[0].Input, "unknown revision carries no numbers")
			require.Equal(t, craft.UsageStatusCorrected, revisions[1].Status)
			require.EqualValues(t, 200, revisions[1].Input)

			facts, err := store.Facts(ctx, 1, "run-1")
			require.NoError(t, err)
			require.Len(t, facts, 1, "a correction is not a second physical fact")
			require.Equal(t, craft.UsageStatusCorrected, facts[0].Status)

			events, err := store.ListUsageEvents(ctx, 1, "call-1", "att-1")
			require.NoError(t, err)
			require.Len(t, events, 2, "both revisions are delivered downstream")

			err = store.Correct(ctx, usageFact(1, "run-1", "dlg-1", "call-x", "att-x", craft.RuntimeOC))
			require.ErrorIs(t, err, craft.ErrUsageNotRecorded)
		})
	}
}

// TestCraftUsageStoreCrossTenantSameCallIDNeverMixes: the unique constraint
// and the usage key both include the tenant, so two tenants reusing the same
// call id record two independent facts and never see each other's rows.
func TestCraftUsageStoreCrossTenantSameCallIDNeverMixes(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			db := openCraftUsageDB(t)
			store := NewCraftUsageStore(db)
			ctx := context.Background()

			a := usageFact(1, "run-1", "", "call-1", "att-1", craft.RuntimeMain)
			b := usageFact(2, "run-2", "", "call-1", "att-1", craft.RuntimeMain)
			b.Output = 77
			require.NoError(t, store.Append(ctx, a))
			require.NoError(t, store.Append(ctx, b))

			factsA, err := store.Facts(ctx, 1, "run-1")
			require.NoError(t, err)
			require.Len(t, factsA, 1)
			require.EqualValues(t, a.Output, factsA[0].Output)
			factsB, err := store.Facts(ctx, 2, "run-2")
			require.NoError(t, err)
			require.Len(t, factsB, 1)
			require.EqualValues(t, b.Output, factsB[0].Output)
			require.NotEqual(t, factsA[0].ID, factsB[0].ID)

			// Correcting tenant 2's attempt must not touch tenant 1's fact.
			late := b
			late.Input, late.Output, late.Cached = 10, 5, 0
			require.NoError(t, store.Correct(ctx, late))
			factsA2, err := store.Facts(ctx, 1, "run-1")
			require.NoError(t, err)
			require.Len(t, factsA2, 1)
			require.EqualValues(t, a.Output, factsA2[0].Output)
		})
	}
}

// TestCraftUsageOutboxDeliveryStateTracksStableIdentity: every revision
// carries a stable event key; failed deliveries count retries, the dead
// state parks exhausted events, and a delivered mark is idempotent under
// the same key so the OpenMeter worker can redelivery-safely retry.
func TestCraftUsageOutboxDeliveryStateTracksStableIdentity(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			db := openCraftUsageDB(t)
			store := NewCraftUsageStore(db)
			ctx := context.Background()

			fact := usageFact(1, "run-1", "", "call-1", "att-1", craft.RuntimeMain)
			require.NoError(t, store.Append(ctx, fact))

			pending, err := store.PendingUsageEvents(ctx, 10)
			require.NoError(t, err)
			require.Len(t, pending, 1)
			key := pending[0].EventKey
			require.NotEmpty(t, key)

			require.NoError(t, store.FailUsageEventDelivery(ctx, key))
			events, err := store.ListUsageEvents(ctx, 1, "call-1", "att-1")
			require.NoError(t, err)
			require.Len(t, events, 1)
			require.EqualValues(t, 1, events[0].AttemptCount)
			require.Equal(t, CraftUsageOutboxPending, events[0].State)

			require.NoError(t, store.MarkUsageEventDelivered(ctx, key))
			require.NoError(t, store.MarkUsageEventDelivered(ctx, key), "re-mark is idempotent")
			events, err = store.ListUsageEvents(ctx, 1, "call-1", "att-1")
			require.NoError(t, err)
			require.Equal(t, CraftUsageOutboxSent, events[0].State)

			pending, err = store.PendingUsageEvents(ctx, 10)
			require.NoError(t, err)
			require.Empty(t, pending, "sent events leave the pending queue")

			err = store.MarkUsageEventDelivered(ctx, "usage:nonexistent:1")
			require.ErrorIs(t, err, ErrUsageEventUnknown)

			dead := usageFact(1, "run-1", "", "call-2", "att-1", craft.RuntimeMain)
			require.NoError(t, store.Append(ctx, dead))
			deadKey := fmt.Sprintf("usage:%s:1", craft.UsageKey(1, "call-2", "att-1"))
			for i := 0; i < craftUsageDeliveryAttempts; i++ {
				require.NoError(t, store.FailUsageEventDelivery(ctx, deadKey))
			}
			events, err = store.ListUsageEvents(ctx, 1, "call-2", "att-1")
			require.NoError(t, err)
			require.Len(t, events, 1)
			require.Equal(t, CraftUsageOutboxDead, events[0].State)
			require.EqualValues(t, craftUsageDeliveryAttempts, events[0].AttemptCount)
		})
	}
}

// TestCraftUsageStoreRejectsUntrustedFacts: invalid identity, unknown
// runtime, fabricated unknown numbers and additive cached tokens are
// rejected before anything is persisted.
func TestCraftUsageStoreRejectsUntrustedFacts(t *testing.T) {
	db := openCraftUsageDB(t)
	store := NewCraftUsageStore(db)
	ctx := context.Background()

	bad := usageFact(0, "run-1", "", "call-1", "att-1", craft.RuntimeMain)
	require.ErrorIs(t, store.Append(ctx, bad), craft.ErrInvalidInput)

	noDelegation := usageFact(1, "run-1", "", "call-1", "att-1", craft.RuntimeOC)
	require.ErrorIs(t, store.Append(ctx, noDelegation), craft.ErrInvalidInput)

	additive := usageFact(1, "run-1", "", "call-1", "att-1", craft.RuntimeMain)
	additive.Cached = additive.Input + 1
	require.ErrorIs(t, store.Append(ctx, additive), craft.ErrInvalidInput)

	spoofed := usageFact(1, "run-1", "", "call-1", "att-1", craft.RuntimeMain)
	spoofed.ID = craft.UsageKey(2, "call-1", "att-1")
	require.ErrorIs(t, store.Append(ctx, spoofed), craft.ErrInvalidInput)

	var rows int64
	require.NoError(t, db.Table("craft_usage_facts").Count(&rows).Error)
	require.EqualValues(t, 0, rows, "nothing persisted")
}

// seedCraftUsageRun inserts one agent_runs row. The durable run table has no
// user_id column: the sessions.user_id scope is stored as owner_id at
// admission (types.SessionOwnerIDFromContext), so that is the attribution
// column the daily-bucket fold joins through.
func seedCraftUsageRun(t *testing.T, db *gorm.DB, tenant uint64, runID, owner string) {
	t.Helper()
	require.NoError(t, db.Exec(
		`INSERT INTO agent_runs (tenant_id, run_id, session_id, owner_id, request_id,
		   assistant_message_id, request_hash, snapshot, deadline)
		 VALUES (?, ?, 's1', ?, ?, ?, ?, '{}', ?)`,
		tenant, runID, owner, "req-"+runID, "am-"+runID, "hash-"+runID,
		time.Now().Add(time.Hour),
	).Error)
}

// craftBucket fetches the tenant's single bucket row for one user+model under
// the craft flow.
func craftBucket(t *testing.T, db *gorm.DB, tenant uint64, user string) types.UserUsage {
	t.Helper()
	var row types.UserUsage
	require.NoError(t, db.Where("tenant_id = ? AND user_id = ? AND model = ? AND flow = ?",
		tenant, user, "gpt-test", types.UsageFlowCraft).Take(&row).Error)
	return row
}

// TestCraftUsageStoreFoldsFactsIntoUserUsageDailyBuckets pins the SP12 craft
// ingestion point: every first-recorded revision folds its token counts into
// the owning user's (tenant, user, UTC day, model, craft) bucket inside the
// SAME transaction as the fact row. Replays fold nothing (the content-equal
// no-op runs first), distinct attempts of the same run accumulate in place,
// a fact whose run matches no agent_runs row attributes to the empty user,
// and cost stays zero — craft pricing belongs to the commercial outbox
// pipeline, the daily bucket records the token dimension only.
func TestCraftUsageStoreFoldsFactsIntoUserUsageDailyBuckets(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			db := openCraftUsageDB(t)
			store := NewCraftUsageStore(db)
			ctx := context.Background()
			seedCraftUsageRun(t, db, 1, "run-1", "u1")
			day := time.Now().UTC().Format("2006-01-02")

			first := usageFact(1, "run-1", "", "call-1", "att-1", craft.RuntimeMain)
			require.NoError(t, store.Append(ctx, first))
			require.NoError(t, store.Append(ctx, first), "replay must not fold the bucket twice")

			second := usageFact(1, "run-1", "", "call-2", "att-1", craft.RuntimeMain)
			require.NoError(t, store.Append(ctx, second))

			// No agent_runs row for run-x: attribution falls back to the
			// empty user instead of dropping the bucket.
			orphan := usageFact(1, "run-x", "", "call-3", "att-1", craft.RuntimeMain)
			require.NoError(t, store.Append(ctx, orphan))

			// A correction is a NEW revision: the bucket keeps the first
			// observation's numbers and the corrected revision must never
			// re-accumulate them.
			corrected := first
			corrected.Output = 90
			corrected.Status = craft.UsageStatusCorrected
			require.NoError(t, store.Correct(ctx, corrected))
			require.NoError(t, store.Correct(ctx, corrected), "correction replay folds nothing")

			var buckets int64
			require.NoError(t, db.Table("user_usage").Where("tenant_id = ?", uint64(1)).Count(&buckets).Error)
			require.EqualValues(t, 2, buckets, "one bucket for u1, one for the empty user")

			u1 := craftBucket(t, db, 1, "u1")
			require.Equal(t, day, u1.WindowStart.UTC().Format("2006-01-02"), "bucket day is the fact's UTC observation day")
			require.EqualValues(t, 240, u1.InputTokens, "two distinct attempts accumulate: 120+120")
			require.EqualValues(t, 60, u1.OutputTokens, "corrections must not re-accumulate: 30+30, not 30+30+90")
			require.EqualValues(t, 40, u1.CacheReadTokens, "cached tokens fold into cache_read: 20+20")
			require.Zero(t, u1.CacheWriteTokens, "craft facts carry no cache-write dimension")
			require.Zero(t, u1.CostMicrocredits, "pricing stays with the commercial pipeline")

			anon := craftBucket(t, db, 1, "")
			require.EqualValues(t, 120, anon.InputTokens, "unmatched run attributes to the empty user")
			require.Zero(t, anon.CostMicrocredits)
		})
	}
}

// TestCraftUsageStoreUnknownFirstRevisionKeepsBucketAtFirstObservation pins
// the chosen correction semantics (SP12 ruling): the daily bucket records
// only the FIRST recorded revision's numbers. A stream-break unknown (zero
// tokens) followed by a late correction with real numbers leaves the bucket
// at the first observation — the corrected revision's tokens flow through the
// commercial outbox pipeline, never back into the bucket. This trades a
// possible bucket undercount for never double-counting a superseded
// observation.
func TestCraftUsageStoreUnknownFirstRevisionKeepsBucketAtFirstObservation(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			db := openCraftUsageDB(t)
			store := NewCraftUsageStore(db)
			ctx := context.Background()
			seedCraftUsageRun(t, db, 1, "run-1", "u1")

			broken := usageFact(1, "run-1", "dlg-1", "call-1", "att-1", craft.RuntimeOC)
			broken.Input, broken.Output, broken.Cached = 0, 0, 0
			broken.Status = craft.UsageStatusUnknown
			require.NoError(t, store.Append(ctx, broken))

			late := broken
			late.Input, late.Output, late.Cached = 200, 80, 40
			late.Status = craft.UsageStatusCorrected
			require.NoError(t, store.Correct(ctx, late))

			u1 := craftBucket(t, db, 1, "u1")
			require.Zero(t, u1.InputTokens, "bucket keeps the first (unknown) observation's numbers")
			require.Zero(t, u1.OutputTokens)
			require.Zero(t, u1.CacheReadTokens)
		})
	}
}
