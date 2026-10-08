package career

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type blockingRuleQuotaGate struct {
	entered chan struct{}
	release chan struct{}
	mu      sync.Mutex
	calls   int
}

func (g *blockingRuleQuotaGate) AdmitSearch(context.Context, Scope, string, string) error {
	g.mu.Lock()
	g.calls++
	g.mu.Unlock()
	select {
	case g.entered <- struct{}{}:
	default:
	}
	<-g.release
	return nil
}

// searchRuleClockBase is the deterministic clock every set_rule call uses
// until a test overrides Office.searchRuleNow.
var searchRuleClockBase = time.Date(2026, 9, 25, 8, 0, 0, 0, time.UTC)

func newSearchRuleOffice(t *testing.T) (*Office, *scriptedTransport, *fakeSearchQuotaGate, context.Context) {
	t.Helper()
	o, ctx := newSourceImportOffice(t, "rule-owner", 96)
	policy := &stubSourcePolicy{approvedHosts: map[string]bool{"jobs.example.test": true}}
	transport := &scriptedTransport{policy: policy, scripts: map[string]scriptedFetch{
		searchFixtureURL(): {result: SourceFetchResult{StatusCode: 200, ContentType: "text/html", Text: searchFixtureListing, FinalURL: searchFixtureURL(), Complete: true}},
	}}
	o.sourcePolicy = policy
	o.sourceTransport = transport
	o.searchRegistry = &stubSearchRegistry{sources: []VettedSearchSource{{
		ID:                "src-fixture",
		Label:             "Fixture Jobs",
		SearchURLTemplate: "https://jobs.example.test/listings?q={query}",
		AccessMethods:     []string{"public https listing page"},
		Cities:            []string{"Hangzhou"},
	}}}
	gate := &fakeSearchQuotaGate{}
	o.searchQuotaGate = gate
	o.searchRuleNow = func() time.Time { return searchRuleClockBase }
	return o, transport, gate, ctx
}

func ruleInput(requestID string, intervalMinutes uint64, status string) SetRuleInput {
	return SetRuleInput{RequestID: requestID, Query: searchFixtureQuery, IntervalMinutes: intervalMinutes, Status: status, ExpectedRevision: 0}
}

func countRows(t *testing.T, o *Office, model any) int64 {
	t.Helper()
	var count int64
	require.NoError(t, o.db.Model(model).Count(&count).Error)
	return count
}

// ---- 1. set_rule stores one rule and replays by request ID ------------------

func TestSetRuleStoresRuleAndReplayIsIdempotent(t *testing.T) {
	o, _, _, ctx := newSearchRuleOffice(t)
	create := ruleInput("rule-set-1", 60, RuleStatusEnabled)
	receipt, err := o.SetRule(ctx, create)
	require.NoError(t, err)
	require.Equal(t, RuleKindSet, receipt.Kind)
	require.NotEmpty(t, receipt.RuleID)
	require.Equal(t, RuleStatusEnabled, receipt.Status)
	require.Equal(t, searchFixtureQuery, receipt.Query)
	require.Equal(t, uint64(60), receipt.IntervalMinutes)
	require.NotNil(t, receipt.NextDueAt, "an enabled rule schedules its first due time")
	require.Equal(t, searchRuleClockBase.Add(time.Hour), *receipt.NextDueAt)

	// Exact replay returns the original receipt and changes nothing.
	replay, err := o.SetRule(ctx, create)
	require.NoError(t, err)
	firstJSON, _ := json.Marshal(receipt)
	replayJSON, _ := json.Marshal(replay)
	require.JSONEq(t, string(firstJSON), string(replayJSON))
	require.EqualValues(t, 1, countRows(t, o, &searchRuleRecord{}))

	// The same request ID with changed content is a typed conflict.
	changed := create
	changed.IntervalMinutes = 30
	_, err = o.SetRule(ctx, changed)
	require.ErrorIs(t, err, ErrIdempotencyConflict)

	found, err := o.FindRuleReceipt(ctx, "rule-set-1")
	require.NoError(t, err)
	require.Equal(t, receipt, found)

	// Update house semantics: a stale expected revision conflicts, a fresh
	// request ID pauses the rule and cancels the due plan.
	_, err = o.Act(ctx, "propose", "", "preference.city", "Hangzhou", "rule-seed", 0, Source{Kind: "manual"})
	require.NoError(t, err)
	pause := ruleInput("rule-set-2", 60, RuleStatusPaused)
	pause.RuleID = receipt.RuleID
	_, err = o.SetRule(ctx, pause)
	require.ErrorIs(t, err, ErrRevisionConflict)
	pause.ExpectedRevision = 1
	paused, err := o.SetRule(ctx, pause)
	require.NoError(t, err)
	require.Equal(t, RuleStatusPaused, paused.Status)
	require.Nil(t, paused.NextDueAt, "a paused rule holds no due plan")
	require.Equal(t, uint64(2), paused.Revision, "each rule write bumps the rule revision")
	view, err := o.Rule(ctx, receipt.RuleID)
	require.NoError(t, err)
	require.Equal(t, RuleStatusPaused, view.Status)
	require.Nil(t, view.NextDueAt)
}

// ---- 2. disabled rules never trigger or enqueue -----------------------------

func TestDisabledRuleNeverTriggersOrEnqueues(t *testing.T) {
	o, transport, gate, ctx := newSearchRuleOffice(t)
	disabled := ruleInput("rule-off-1", 60, RuleStatusDisabled)
	receipt, err := o.SetRule(ctx, disabled)
	require.NoError(t, err)
	require.Nil(t, receipt.NextDueAt, "a disabled rule never schedules")

	outcomes, err := o.TriggerDueRules(ctx, searchRuleClockBase.Add(24*time.Hour))
	require.NoError(t, err)
	require.Empty(t, outcomes)

	// A previously enabled rule stops the moment it is disabled.
	enabled, err := o.SetRule(ctx, ruleInput("rule-off-2", 60, RuleStatusEnabled))
	require.NoError(t, err)
	require.NotNil(t, enabled.NextDueAt)
	off := ruleInput("rule-off-3", 60, RuleStatusDisabled)
	off.RuleID = enabled.RuleID
	_, err = o.SetRule(ctx, off)
	require.NoError(t, err)

	outcomes, err = o.TriggerDueRules(ctx, searchRuleClockBase.Add(25*time.Hour))
	require.NoError(t, err)
	require.Empty(t, outcomes)

	require.Zero(t, countRows(t, o, &searchRuleRunRecord{}), "zero runs for disabled rules")
	require.Zero(t, countRows(t, o, &searchRecord{}), "zero searches for disabled rules")
	require.Zero(t, countRows(t, o, &searchDiscoveryTodoRecord{}), "zero discovery todos for disabled rules")
	require.Zero(t, gate.calls, "a disabled rule never consults quota admission")
	require.Empty(t, transport.calls, "a disabled rule never reaches a source")
}

// ---- 3. paused semantics: cancel the pending occurrence ----------------------
//
// Frozen choice: pausing cancels the pending (missed) occurrence — nothing is
// enqueued while paused and the rule holds no due plan; resuming schedules the
// next trigger from the resume moment.

func TestPausedRuleCancelsOrDefersNextTrigger(t *testing.T) {
	o, transport, _, ctx := newSearchRuleOffice(t)
	created, err := o.SetRule(ctx, ruleInput("rule-pause-1", 60, RuleStatusEnabled))
	require.NoError(t, err)
	require.NotNil(t, created.NextDueAt)
	require.Equal(t, searchRuleClockBase.Add(time.Hour), *created.NextDueAt)

	pause := ruleInput("rule-pause-2", 60, RuleStatusPaused)
	pause.RuleID = created.RuleID
	paused, err := o.SetRule(ctx, pause)
	require.NoError(t, err)
	require.Nil(t, paused.NextDueAt)

	// The due moment passes while paused: the occurrence is cancelled, not run.
	outcomes, err := o.TriggerDueRules(ctx, searchRuleClockBase.Add(90*time.Minute))
	require.NoError(t, err)
	require.Empty(t, outcomes)
	require.Zero(t, countRows(t, o, &searchRuleRunRecord{}))
	require.Empty(t, transport.calls)

	// Resume schedules the next trigger from the resume moment.
	resumeAt := searchRuleClockBase.Add(2 * time.Hour)
	o.searchRuleNow = func() time.Time { return resumeAt }
	resume := ruleInput("rule-pause-3", 60, RuleStatusEnabled)
	resume.RuleID = created.RuleID
	resumed, err := o.SetRule(ctx, resume)
	require.NoError(t, err)
	require.NotNil(t, resumed.NextDueAt)
	require.Equal(t, resumeAt.Add(time.Hour), *resumed.NextDueAt)

	early, err := o.TriggerDueRules(ctx, resumeAt.Add(30*time.Minute))
	require.NoError(t, err)
	require.Empty(t, early, "the deferred trigger must not fire before its new due time")

	due := resumeAt.Add(time.Hour)
	outcomes, err = o.TriggerDueRules(ctx, due)
	require.NoError(t, err)
	require.Len(t, outcomes, 1)
	require.Equal(t, RuleRunStatusCompleted, outcomes[0].Status)
	require.Equal(t, uint64(1), outcomes[0].Period)

	view, err := o.Rule(ctx, created.RuleID)
	require.NoError(t, err)
	require.Len(t, view.Runs, 1)
	require.Equal(t, uint64(1), view.Runs[0].Period)
	require.NotNil(t, view.NextDueAt)
	require.Equal(t, due.Add(time.Hour), *view.NextDueAt)
}

// ---- 4. enable-time preview: conditions, frequency, estimated cost ----------

func TestRuleEnableSurfacesConditionsFrequencyAndEstimatedCost(t *testing.T) {
	o, _, _, ctx := newSearchRuleOffice(t)
	receipt, err := o.SetRule(ctx, ruleInput("rule-est-1", 30, RuleStatusEnabled))
	require.NoError(t, err)
	require.Equal(t, searchFixtureQuery, receipt.Query)
	require.Equal(t, uint64(30), receipt.IntervalMinutes)
	require.Equal(t, 48.0, receipt.Estimate.TriggersPerDay)
	require.Equal(t, 1, receipt.Estimate.SourcesPerTrigger)
	require.Equal(t, 48.0, receipt.Estimate.EstimatedSearchesPerDay)
	require.NotEmpty(t, receipt.Estimate.Basis)
	require.Contains(t, receipt.Estimate.Basis, "deterministic")
	require.Contains(t, receipt.Estimate.Basis, "not a quota balance")

	// The rule view carries the same honest preview before any run happens.
	view, err := o.Rule(ctx, receipt.RuleID)
	require.NoError(t, err)
	require.Equal(t, receipt.Estimate, view.Estimate)
	require.Equal(t, RuleStatusEnabled, view.Status)
	require.Empty(t, view.Runs)
	require.Zero(t, countRows(t, o, &searchRecord{}))
	require.Zero(t, countRows(t, o, &searchRuleRunRecord{}))
	require.Zero(t, countRows(t, o, &searchDiscoveryTodoRecord{}))

	// An empty registry keeps the estimate honest: zero vetted sources means
	// zero projected source fetches even though the trigger cadence stays.
	o.searchRegistry = emptySearchSourceRegistry{}
	emptyView, err := o.Rule(ctx, receipt.RuleID)
	require.NoError(t, err)
	require.Zero(t, emptyView.Estimate.SourcesPerTrigger)
	require.Zero(t, emptyView.Estimate.EstimatedSearchesPerDay)
	require.Equal(t, 48.0, emptyView.Estimate.TriggersPerDay)
}

// ---- 5. one discovery todo per new job ---------------------------------------

func TestRuleTriggerSameNewJobProducesSingleDiscoveryTodo(t *testing.T) {
	o, _, _, ctx := newSearchRuleOffice(t)
	created, err := o.SetRule(ctx, ruleInput("rule-todo-1", 60, RuleStatusEnabled))
	require.NoError(t, err)

	first, err := o.TriggerDueRules(ctx, searchRuleClockBase.Add(time.Hour))
	require.NoError(t, err)
	require.Len(t, first, 1)
	require.Equal(t, RuleRunStatusCompleted, first[0].Status)
	require.EqualValues(t, 2, countRows(t, o, &searchDiscoveryTodoRecord{}), "one discovery todo per discovered job link")

	// The next period surfaces the same links again: still exactly one todo
	// per job (durable idempotency key), never a duplicate.
	second, err := o.TriggerDueRules(ctx, searchRuleClockBase.Add(2*time.Hour))
	require.NoError(t, err)
	require.Len(t, second, 1)
	require.EqualValues(t, 2, countRows(t, o, &searchDiscoveryTodoRecord{}))

	view, err := o.Rule(ctx, created.RuleID)
	require.NoError(t, err)
	require.Len(t, view.Runs, 2)
	require.Len(t, view.Todos, 2)
	links := map[string]bool{}
	for _, todo := range view.Todos {
		require.Equal(t, RuleTodoStatusOpen, todo.Status)
		links[todo.Link] = true
	}
	require.True(t, links["https://jobs.example.test/j/1001"])
	require.True(t, links["https://jobs.example.test/j/1002"])
}

// ---- 6. duplicate trigger reconciles by the same request ID ------------------

func TestRuleTriggerDuplicateReconcilesBySameRequestId(t *testing.T) {
	o, transport, _, ctx := newSearchRuleOffice(t)
	created, err := o.SetRule(ctx, ruleInput("rule-dup-1", 60, RuleStatusEnabled))
	require.NoError(t, err)

	// Simulate a crashed prior attempt: the deterministic period request ID
	// already holds a terminal search.
	deterministic := "rule:" + created.RuleID + ":1"
	seeded, err := o.SearchOnce(ctx, SearchOnceInput{RequestID: deterministic, Query: searchFixtureQuery, ExpectedRevision: 0})
	require.NoError(t, err)
	require.Equal(t, SearchStatusCompleted, seeded.Status)
	require.Len(t, transport.calls, 1)

	outcomes, err := o.TriggerDueRules(ctx, searchRuleClockBase.Add(time.Hour))
	require.NoError(t, err)
	require.Len(t, outcomes, 1)
	require.Equal(t, deterministic, outcomes[0].RequestID)
	require.Equal(t, RuleRunStatusCompleted, outcomes[0].Status)
	require.Len(t, transport.calls, 1, "the duplicate trigger reconciles by request ID without refetching")

	require.EqualValues(t, 1, countRows(t, o, &searchRecord{}), "no duplicated search row")
	require.EqualValues(t, 1, countRows(t, o, &searchRuleRunRecord{}), "one run row per rule period")
	view, err := o.Rule(ctx, created.RuleID)
	require.NoError(t, err)
	require.Len(t, view.Runs, 1)
	require.Equal(t, seeded.SearchID, view.Runs[0].SearchID)
}

// TestRuleTriggerTakesOverStrandedClaimAfterRevisionMoved pins ocr3-017: a
// crashed rule attempt leaves the deterministic period request ID claiming;
// once the profile revision moves, the plain fingerprint no longer matches
// and every later trigger would answer ErrIdempotencyConflict forever,
// stranding the rule period and failing the whole trigger. The trigger
// takes the stranded row over under its stored fingerprint instead.
func TestRuleTriggerTakesOverStrandedClaimAfterRevisionMoved(t *testing.T) {
	o, _, _, ctx := newSearchRuleOffice(t)
	created, err := o.SetRule(ctx, ruleInput("rule-strand-1", 60, RuleStatusEnabled))
	require.NoError(t, err)
	scope, err := getScope(ctx)
	require.NoError(t, err)

	// A crashed first attempt: the period request ID holds an expired claim
	// fingerprinted against the revision observed back then (0).
	deterministic := "rule:" + created.RuleID + ":1"
	strandedFingerprint, err := searchFingerprint(SearchOnceInput{RequestID: deterministic, Query: searchFixtureQuery, ExpectedRevision: 0})
	require.NoError(t, err)
	leaseUntil := searchRuleClockBase.Add(-time.Minute)
	claimBody, err := json.Marshal(searchClaimBody{Kind: searchOnceClaimKind, ClaimToken: "stranded-token", LeaseUntil: leaseUntil})
	require.NoError(t, err)
	require.NoError(t, o.db.Create(&searchRecord{
		ID: uuid.NewString(), TenantID: scope.TenantID, UserID: scope.UserID,
		RequestID: deterministic, Fingerprint: strandedFingerprint, Status: searchStatusClaiming,
		ClaimToken: "stranded-token", LeaseUntil: &leaseUntil, Query: searchFixtureQuery,
		ReceiptBody: string(claimBody), CreatedAt: searchRuleClockBase.Add(-2 * time.Minute),
	}).Error)

	// The revision moves before the retry (every write path bumps it).
	_, err = o.Confirm(ctx, "skill.go", "Go", "rule-strand-rev", 0, Source{Kind: "manual"})
	require.NoError(t, err)

	outcomes, err := o.TriggerDueRules(ctx, searchRuleClockBase.Add(time.Hour))
	require.NoError(t, err, "a stranded claim under a moved revision must not fail the trigger")
	require.Len(t, outcomes, 1)
	require.Equal(t, deterministic, outcomes[0].RequestID)
	require.Equal(t, RuleRunStatusCompleted, outcomes[0].Status)

	require.EqualValues(t, 1, countRows(t, o, &searchRecord{}), "the stranded row is taken over, not duplicated")
	require.EqualValues(t, 1, countRows(t, o, &searchRuleRunRecord{}))
	view, err := o.Rule(ctx, created.RuleID)
	require.NoError(t, err)
	require.Len(t, view.Runs, 1)
	require.Equal(t, RuleRunStatusCompleted, view.Runs[0].Status)
}

// TestRuleTriggerReplaysStrandedTerminalAfterRevisionMoved pins the replay
// half of ocr3-017: a terminal receipt stored under an older revision still
// replays for the rule trigger after the revision moved — no refetch, no
// duplicate search row.
func TestRuleTriggerReplaysStrandedTerminalAfterRevisionMoved(t *testing.T) {
	o, transport, _, ctx := newSearchRuleOffice(t)
	created, err := o.SetRule(ctx, ruleInput("rule-strand-2", 60, RuleStatusEnabled))
	require.NoError(t, err)

	deterministic := "rule:" + created.RuleID + ":1"
	seeded, err := o.SearchOnce(ctx, SearchOnceInput{RequestID: deterministic, Query: searchFixtureQuery, ExpectedRevision: 0})
	require.NoError(t, err)
	require.Equal(t, SearchStatusCompleted, seeded.Status)

	// The revision moves before the trigger runs.
	_, err = o.Confirm(ctx, "skill.go", "Go", "rule-strand-2-rev", 0, Source{Kind: "manual"})
	require.NoError(t, err)

	outcomes, err := o.TriggerDueRules(ctx, searchRuleClockBase.Add(time.Hour))
	require.NoError(t, err)
	require.Len(t, outcomes, 1)
	require.Equal(t, RuleRunStatusCompleted, outcomes[0].Status)
	require.Len(t, transport.calls, 1, "the stored terminal receipt replays without refetching")
	require.EqualValues(t, 1, countRows(t, o, &searchRecord{}))
	view, err := o.Rule(ctx, created.RuleID)
	require.NoError(t, err)
	require.Len(t, view.Runs, 1)
	require.Equal(t, seeded.SearchID, view.Runs[0].SearchID)
}

// ---- 7. insufficient budget is a visible status, never a silent skip ---------

func TestRuleBudgetInsufficientFormsVisibleStatusNotSilentSkip(t *testing.T) {
	o, _, gate, ctx := newSearchRuleOffice(t)
	gate.refused = true
	created, err := o.SetRule(ctx, ruleInput("rule-budget-1", 60, RuleStatusEnabled))
	require.NoError(t, err)

	outcomes, err := o.TriggerDueRules(ctx, searchRuleClockBase.Add(time.Hour))
	require.NoError(t, err)
	require.Len(t, outcomes, 1)
	require.Equal(t, RuleRunStatusBlockedNoQuota, outcomes[0].Status)
	require.Zero(t, countRows(t, o, &searchRecord{}), "a blocked trigger consumes no search")

	view, err := o.Rule(ctx, created.RuleID)
	require.NoError(t, err)
	require.Len(t, view.Runs, 1)
	require.Equal(t, RuleRunStatusBlockedNoQuota, view.Runs[0].Status)
	require.Equal(t, "rule:"+created.RuleID+":1", view.Runs[0].RequestID)
	require.Empty(t, view.Runs[0].SearchID)
	require.NotEmpty(t, view.Runs[0].Note, "the blocked status must carry a human-readable reason")

	// Quota recovery: once admission passes, the next due trigger runs.
	gate.refused = false
	recovered, err := o.TriggerDueRules(ctx, searchRuleClockBase.Add(2*time.Hour))
	require.NoError(t, err)
	require.Len(t, recovered, 1)
	require.Equal(t, RuleRunStatusCompleted, recovered[0].Status)
}

// ---- 8. incomplete sources are a visible status ------------------------------

func TestRuleIncompleteSourcesFormVisibleStatus(t *testing.T) {
	o, transport, _, ctx := newSearchRuleOffice(t)
	o.searchRegistry = emptySearchSourceRegistry{}
	created, err := o.SetRule(ctx, ruleInput("rule-src-1", 60, RuleStatusEnabled))
	require.NoError(t, err)

	outcomes, err := o.TriggerDueRules(ctx, searchRuleClockBase.Add(time.Hour))
	require.NoError(t, err)
	require.Len(t, outcomes, 1)
	require.Equal(t, RuleRunStatusNoVettedSources, outcomes[0].Status)
	require.Zero(t, countRows(t, o, &searchRecord{}))
	require.Empty(t, transport.calls, "no source may be reached when none is vetted")

	view, err := o.Rule(ctx, created.RuleID)
	require.NoError(t, err)
	require.Len(t, view.Runs, 1)
	require.Equal(t, RuleRunStatusNoVettedSources, view.Runs[0].Status)
	require.NotEmpty(t, view.Runs[0].Note)
}

// ---- 9. scope isolation -------------------------------------------------------

func TestRuleScopeRejectsOtherTenantAndOwner(t *testing.T) {
	o, _, _, ctx := newSearchRuleOffice(t)
	created, err := o.SetRule(ctx, ruleInput("rule-scope-1", 60, RuleStatusEnabled))
	require.NoError(t, err)

	intruder := WithScope(context.Background(), Scope{UserID: "intruder", TenantID: 96})
	_, err = o.SetRule(intruder, ruleInput("rule-scope-intruder", 60, RuleStatusEnabled))
	require.ErrorIs(t, err, ErrUnauthorized)
	_, err = o.Rule(intruder, created.RuleID)
	require.ErrorIs(t, err, ErrUnauthorized)
	_, err = o.TriggerDueRules(intruder, searchRuleClockBase.Add(time.Hour))
	require.ErrorIs(t, err, ErrUnauthorized)

	otherTenant := WithScope(context.Background(), Scope{UserID: "rule-owner", TenantID: 97})
	_, err = o.SetRule(otherTenant, ruleInput("rule-scope-other", 60, RuleStatusEnabled))
	require.ErrorIs(t, err, ErrUnauthorized)

	// A properly claimed neighbor space must not learn the rule exists.
	neighborCtx := WithScope(context.Background(), Scope{UserID: "neighbor", TenantID: 97})
	require.NoError(t, o.ClaimSpace(neighborCtx))
	_, err = o.Rule(neighborCtx, created.RuleID)
	require.ErrorIs(t, err, ErrRuleNotFound)
	_, err = o.FindRuleReceipt(neighborCtx, "rule-scope-1")
	require.ErrorIs(t, err, ErrReceiptNotFound)
	outcomes, err := o.TriggerDueRules(neighborCtx, searchRuleClockBase.Add(time.Hour))
	require.NoError(t, err)
	require.Empty(t, outcomes, "the sweep only touches the authenticated scope's rules")
}

func TestDueRulePauseCommittedAfterScanPreventsClaimAndSearch(t *testing.T) {
	o, transport, gate, ctx := newSearchRuleOffice(t)
	created, err := o.SetRule(ctx, ruleInput("rule-race-pause-create", 60, RuleStatusEnabled))
	require.NoError(t, err)
	var scanned searchRuleRecord
	require.NoError(t, o.db.Where("id=?", created.RuleID).First(&scanned).Error)

	// A separately constructed Office represents another handler/process using
	// the same database. The stale candidate is dispatched after its pause wins.
	o2, err := NewOffice(o.db)
	require.NoError(t, err)
	pause := ruleInput("rule-race-pause-update", 60, RuleStatusPaused)
	pause.RuleID = created.RuleID
	pause.ExpectedRevision = 0
	_, err = o2.SetRule(ctx, pause)
	require.NoError(t, err)

	scope, err := getScope(ctx)
	require.NoError(t, err)
	_, err = o.triggerRulePeriod(ctx, scope, scanned, searchRuleClockBase.Add(time.Hour))
	require.ErrorIs(t, err, errRuleCandidateStale)
	require.Zero(t, gate.calls, "a candidate invalidated by pause cannot reach quota admission")
	require.Empty(t, transport.calls, "a pause committed before claim prevents external search")
	require.Zero(t, countRows(t, o, &searchRecord{}))
	require.Zero(t, countRows(t, o, &searchRuleRunRecord{}), "no claim/run is persisted for stale candidate")
}

func TestDueRuleClaimBeforePauseFinishesSameRequest(t *testing.T) {
	o, transport, _, ctx := newSearchRuleOffice(t)
	created, err := o.SetRule(ctx, ruleInput("rule-race-claim-create", 60, RuleStatusEnabled))
	require.NoError(t, err)
	gate := &blockingRuleQuotaGate{entered: make(chan struct{}, 1), release: make(chan struct{})}
	o.searchQuotaGate = gate
	o2, err := NewOffice(o.db)
	require.NoError(t, err)
	var outcomes []RuleRunSummary
	finished := make(chan error, 1)
	go func() {
		var triggerErr error
		outcomes, triggerErr = o.TriggerDueRules(ctx, searchRuleClockBase.Add(time.Hour))
		finished <- triggerErr
	}()
	select {
	case <-gate.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("trigger did not reach quota after claiming the period")
	}
	var claim searchRuleRunRecord
	claimErr := o.db.Where("rule_id=? AND period=1", created.RuleID).First(&claim).Error

	pause := ruleInput("rule-race-claim-pause", 60, RuleStatusPaused)
	pause.RuleID = created.RuleID
	pause.ExpectedRevision = 0
	_, err = o2.SetRule(ctx, pause)
	require.NoError(t, err, "pause serializes after the committed claim")
	close(gate.release)
	require.NoError(t, <-finished)
	require.NoError(t, claimErr, "claim must commit before quota admission")
	require.Equal(t, "started", claim.Status)
	require.Equal(t, "rule:"+created.RuleID+":1", claim.RequestID)
	require.Len(t, outcomes, 1)
	require.Equal(t, claim.RequestID, outcomes[0].RequestID)
	require.Equal(t, RuleRunStatusCompleted, outcomes[0].Status)
	require.Len(t, transport.calls, 1)
	var completed searchRuleRunRecord
	require.NoError(t, o.db.Where("rule_id=? AND period=1", created.RuleID).First(&completed).Error)
	require.Equal(t, claim.RequestID, completed.RequestID)
	require.Equal(t, RuleRunStatusCompleted, completed.Status)
}

func TestListRulesUsesStableBoundedOwnerScopedPages(t *testing.T) {
	o, _, _, ctx := newSearchRuleOffice(t)
	// Same timestamp forces the ID tie-breaker to determine stable page order.
	for i := 0; i < 52; i++ {
		id := fmt.Sprintf("rule-list-%03d", i)
		require.NoError(t, o.db.Create(&searchRuleRecord{
			TenantID: 96, UserID: "rule-owner", ID: id, Query: "query " + id,
			IntervalMinutes: 60, Status: RuleStatusPaused, Revision: 2,
			CreatedAt: searchRuleClockBase, UpdatedAt: searchRuleClockBase,
		}).Error)
	}
	page, err := o.ListRules(ctx, "")
	require.NoError(t, err)
	require.Len(t, page.Rules, 50)
	require.NotNil(t, page.NextCursor)
	require.Equal(t, "rule-list-000", page.Rules[0].RuleID)
	require.Nil(t, page.Rules[0].NextDueAt)
	second, err := o.ListRules(ctx, *page.NextCursor)
	require.NoError(t, err)
	require.Len(t, second.Rules, 2)
	require.Nil(t, second.NextCursor)
	require.Equal(t, []string{"rule-list-050", "rule-list-051"}, []string{second.Rules[0].RuleID, second.Rules[1].RuleID})
	_, err = o.ListRules(ctx, "not-a-cursor")
	require.ErrorIs(t, err, ErrInvalidRequest)
	unknownVersion, err := json.Marshal(rulePageCursor{Version: 99, TenantID: 96, UserID: "rule-owner", Updated: searchRuleClockBase, RuleID: "rule-list-000"})
	require.NoError(t, err)
	_, err = o.ListRules(ctx, base64.RawURLEncoding.EncodeToString(unknownVersion))
	require.ErrorIs(t, err, ErrInvalidRequest, "unknown cursor versions must fail closed")
	other := WithScope(context.Background(), Scope{UserID: "neighbor", TenantID: 97})
	require.NoError(t, o.ClaimSpace(other))
	otherPage, err := o.ListRules(other, "")
	require.NoError(t, err)
	require.Empty(t, otherPage.Rules)
	require.Nil(t, otherPage.NextCursor)
	_, err = o.ListRules(other, *page.NextCursor)
	require.ErrorIs(t, err, ErrInvalidRequest, "cursor from another owner scope must not be reusable")

	pausedJSON, err := json.Marshal(page.Rules[0])
	require.NoError(t, err)
	require.Contains(t, string(pausedJSON), `"nextDueAt":null`)
	var fields map[string]any
	require.NoError(t, json.Unmarshal(pausedJSON, &fields))
	require.Len(t, fields, 9)
	for _, key := range []string{"ruleId", "query", "intervalMinutes", "status", "revision", "nextDueAt", "estimate", "createdAt", "updatedAt"} {
		require.Contains(t, fields, key)
	}
}

func TestRuleEditDuringStartedRunPreservesEditedSchedule(t *testing.T) {
	o, _, _, ctx := newSearchRuleOffice(t)
	o.searchRuleNow = func() time.Time { return searchRuleClockBase.Add(2 * time.Minute) }
	created, err := o.SetRule(ctx, ruleInput("rule-edit-active-create", 60, RuleStatusEnabled))
	require.NoError(t, err)
	require.NoError(t, o.db.Model(&searchRuleRecord{}).Where("id=?", created.RuleID).Update("next_due_at", searchRuleClockBase.Add(-time.Minute)).Error)
	var scanned searchRuleRecord
	require.NoError(t, o.db.Where("id=?", created.RuleID).First(&scanned).Error)
	scope, err := getScope(ctx)
	require.NoError(t, err)
	claim, existing, err := o.claimRulePeriod(ctx, scope, scanned, searchRuleClockBase)
	require.NoError(t, err)
	require.NotNil(t, existing)
	require.Equal(t, RuleRunStatusStarted, existing.Status)
	var claimRow lifecycleClaim
	require.NoError(t, o.db.Where("tenant_id=? AND user_id=? AND operation=? AND request_id=?", 96, "rule-owner", "rule_run", claim.Run.RequestID).First(&claimRow).Error)

	edit := ruleInput("rule-edit-active-edit", 90, RuleStatusEnabled)
	edit.RuleID = created.RuleID
	edit.ExpectedRevision = 0
	updated, err := o.SetRule(ctx, edit)
	require.NoError(t, err)
	var afterEdit searchRuleRecord
	require.NoError(t, o.db.Where("id=?", created.RuleID).First(&afterEdit).Error)
	finished, err := o.executeClaimedRuleRun(ctx, scope, claim, searchRuleClockBase.Add(5*time.Minute))
	require.NoError(t, err)
	require.Equal(t, claim.Run.RequestID, finished.RequestID)
	var stored searchRuleRecord
	require.NoError(t, o.db.Where("id=?", created.RuleID).First(&stored).Error)
	require.Equal(t, updated.Revision, stored.Revision)
	require.Equal(t, updated.NextDueAt, stored.NextDueAt)
	require.Equal(t, afterEdit.Revision, stored.Revision)
	require.Equal(t, afterEdit.UpdatedAt, stored.UpdatedAt) // Terminalization cannot rewrite the edit timestamp.
	var claims int64
	require.NoError(t, o.db.Model(&lifecycleClaim{}).Where("operation=? AND request_id=?", "rule_run", claim.Run.RequestID).Count(&claims).Error)
	require.Zero(t, claims, "terminalization resolves the lifecycle claim")
}

func TestSecondStaleDueCandidateCannotClaimNextPeriodEarly(t *testing.T) {
	o, _, gate, ctx := newSearchRuleOffice(t)
	created, err := o.SetRule(ctx, ruleInput("rule-stale-generation-create", 60, RuleStatusEnabled))
	require.NoError(t, err)
	var scanned searchRuleRecord
	require.NoError(t, o.db.Where("id=?", created.RuleID).First(&scanned).Error)
	first, err := o.TriggerDueRules(ctx, searchRuleClockBase.Add(time.Hour))
	require.NoError(t, err)
	require.Len(t, first, 1)
	firstQuotaCalls := gate.calls
	scope, err := getScope(ctx)
	require.NoError(t, err)
	_, err = o.triggerRulePeriod(ctx, scope, scanned, searchRuleClockBase.Add(time.Hour))
	require.ErrorIs(t, err, errRuleCandidateStale)
	require.Equal(t, firstQuotaCalls, gate.calls, "stale generation cannot charge for a following period")
	require.EqualValues(t, 1, countRows(t, o, &searchRuleRunRecord{}))
}

func TestStartedRuleRunRecoversAfterRestartEvenWhenPaused(t *testing.T) {
	o, transport, gate, ctx := newSearchRuleOffice(t)
	created, err := o.SetRule(ctx, ruleInput("rule-recovery-create", 60, RuleStatusEnabled))
	require.NoError(t, err)
	require.NoError(t, o.db.Model(&searchRuleRecord{}).Where("id=?", created.RuleID).
		Updates(map[string]any{"next_due_at": searchRuleClockBase.Add(-time.Minute)}).Error)
	var rule searchRuleRecord
	require.NoError(t, o.db.Where("id=?", created.RuleID).First(&rule).Error)
	run := RuleRunView{Kind: RuleKindRun, RuleID: rule.ID, Period: 1, RequestID: "rule:" + rule.ID + ":1", Status: RuleRunStatusStarted, TriggeredAt: searchRuleClockBase}
	body, err := json.Marshal(searchRuleRunBody{RuleRunView: run, Query: rule.Query, ExpectedProfileRevision: 0})
	require.NoError(t, err)
	require.NoError(t, o.db.Create(&searchRuleRunRecord{ID: uuid.NewString(), TenantID: 96, UserID: "rule-owner", RuleID: rule.ID,
		Period: 1, RequestID: run.RequestID, Status: RuleRunStatusStarted, Body: string(body), CreatedAt: searchRuleClockBase}).Error)
	err = o.admitLifecycleClaim(ctx, Scope{TenantID: 96, UserID: "rule-owner"}, "rule_run", run.RequestID, rule.Query)
	require.NoError(t, err, "restart re-admits the original request before its external recovery")
	err = o.beginLifecycleDeletion(ctx, Scope{TenantID: 96, UserID: "rule-owner"}, "delete-in-progress", "delete-fingerprint")
	require.ErrorIs(t, err, ErrCareerOperationsBusy, "recovery claim must retain deletion barrier until the run settles")

	o2, err := NewOffice(o.db)
	require.NoError(t, err)
	o2.searchRegistry = o.searchRegistry
	o2.sourcePolicy = o.sourcePolicy
	o2.sourceTransport = transport
	o2.searchQuotaGate = gate
	pause := ruleInput("rule-recovery-pause", 60, RuleStatusPaused)
	pause.RuleID = rule.ID
	_, err = o2.SetRule(ctx, pause)
	require.NoError(t, err)

	outcomes, err := o2.TriggerDueRules(ctx, searchRuleClockBase.Add(2*time.Hour))
	require.NoError(t, err)
	require.Len(t, outcomes, 1)
	require.Equal(t, run.RequestID, outcomes[0].RequestID)
	require.Equal(t, RuleRunStatusCompleted, outcomes[0].Status)
	require.Len(t, transport.calls, 1)
	require.Equal(t, 1, gate.calls)
	var recovered searchRuleRunRecord
	require.NoError(t, o.db.Where("rule_id=? AND period=1", rule.ID).First(&recovered).Error)
	require.Equal(t, RuleRunStatusCompleted, recovered.Status)
	var storedRule searchRuleRecord
	require.NoError(t, o.db.Where("id=?", rule.ID).First(&storedRule).Error)
	require.Equal(t, uint64(0), storedRule.LastPeriod, "legacy started recovery must not fabricate a claim-time schedule advancement")
	require.Nil(t, storedRule.NextDueAt)
}

// CAREER-OCR H8: one failing due rule must not abort the whole sweep nor
// discard the outcomes already earned. The failing rule stays first in the
// due order (earlier next_due_at), so the old `return nil, triggerErr`
// starved every other rule in the scope.
type failingAdmissionForRule struct{ ruleID string }

func (g failingAdmissionForRule) AdmitSearch(_ context.Context, _ Scope, requestID, _ string) error {
	if strings.HasPrefix(requestID, "rule:"+g.ruleID+":") {
		return ErrAdmissionUnavailable
	}
	return nil
}

func TestTriggerDueRulesContinuesPastFailingRule(t *testing.T) {
	o, _, _, ctx := newSearchRuleOffice(t)
	first, err := o.SetRule(ctx, ruleInput("rule-fail-first", 30, RuleStatusEnabled))
	require.NoError(t, err)
	second, err := o.SetRule(ctx, ruleInput("rule-fail-second", 60, RuleStatusEnabled))
	require.NoError(t, err)
	o.searchQuotaGate = failingAdmissionForRule{ruleID: first.RuleID}

	outcomes, err := o.TriggerDueRules(ctx, searchRuleClockBase.Add(61*time.Minute))
	require.ErrorIs(t, err, ErrAdmissionUnavailable)
	require.Len(t, outcomes, 1, "the healthy later-due rule must still run when an earlier one fails")
	require.Equal(t, second.RuleID, outcomes[0].RuleID)
	require.Equal(t, RuleRunStatusCompleted, outcomes[0].Status)
}
