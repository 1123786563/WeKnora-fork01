package career

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

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
