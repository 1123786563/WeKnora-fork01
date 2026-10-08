package career

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// ---- T21 fixtures -----------------------------------------------------------
//
// The usage fixtures keep the production admission gate installed: NewOffice
// wires the real ledger-backed gate, and only the quota limit is pinned per
// test. The search source is scripted exactly like the T11 fixtures so a
// charged run performs one real fetch.

const usageFixtureQuery = "go engineer"

func usageFixtureURL() string {
	return "https://jobs.example.test/listings?q=" + url.QueryEscape(usageFixtureQuery)
}

func newUsageSearchOffice(t *testing.T, user string, tenant uint64, limit int64) (*Office, *scriptedTransport, context.Context) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	o, err := NewOffice(db)
	require.NoError(t, err)
	o.SetSearchQuotaLimit(limit)
	policy := &stubSourcePolicy{approvedHosts: map[string]bool{"jobs.example.test": true}}
	transport := &scriptedTransport{policy: policy, scripts: map[string]scriptedFetch{
		usageFixtureURL(): {result: SourceFetchResult{StatusCode: 200, ContentType: "text/html", Text: "高级后端工程师 https://jobs.example.test/j/2001 全职", FinalURL: usageFixtureURL(), Complete: true}},
	}}
	o.sourcePolicy = policy
	o.sourceTransport = transport
	o.searchRegistry = &stubSearchRegistry{sources: []VettedSearchSource{{
		ID: "src-usage", Label: "Usage Fixture Jobs",
		SearchURLTemplate: "https://jobs.example.test/listings?q={query}",
		AccessMethods:     []string{"public https listing page"},
		Cities:            []string{"Hangzhou"},
	}}}
	ctx := WithScope(context.Background(), Scope{UserID: user, TenantID: tenant})
	require.NoError(t, o.ClaimSpace(ctx))
	return o, transport, ctx
}

func usageSearchInput(requestID string) SearchOnceInput {
	return SearchOnceInput{RequestID: requestID, Query: usageFixtureQuery, ExpectedRevision: 0}
}

func usageReservationCount(t *testing.T, o *Office) int64 {
	t.Helper()
	var total int64
	require.NoError(t, o.db.Model(&usageReservationRecord{}).Count(&total).Error)
	return total
}

// ---- 1. estimate before execution -------------------------------------------

func TestAdmissionShowsCostAndConditionsBeforeExecution(t *testing.T) {
	o, _, ctx := newUsageSearchOffice(t, "owner", 95, 2)

	// The estimate is answerable before anything executes: it names the
	// frozen cost, the admission conditions, and the live balance.
	before, err := o.UsageEstimate(ctx, UsageOperationSearchOnce)
	require.NoError(t, err)
	require.Equal(t, "usage_estimate", before.Kind)
	require.Equal(t, UsageOperationSearchOnce, before.Operation)
	require.EqualValues(t, 1, before.CostUnits, "one charged run costs exactly one frozen unit")
	require.NotEmpty(t, before.Conditions, "the estimate must state when the quota is charged")
	conditions := ""
	for _, condition := range before.Conditions {
		conditions += condition + "\n"
	}
	require.Contains(t, conditions, "requestId", "conditions must explain the request-ID idempotency")
	require.Contains(t, conditions, "执行前", "conditions must state the charge happens before execution")
	require.EqualValues(t, 2, before.LimitUnits)
	require.EqualValues(t, 0, before.ReservedUnits)
	require.EqualValues(t, 0, before.SettledUnits)
	require.EqualValues(t, 2, before.RemainingUnits)
	require.True(t, before.WouldAdmit)
	require.False(t, before.PeriodStart.IsZero())
	require.False(t, before.PeriodEnd.IsZero())
	require.True(t, before.PeriodEnd.After(before.PeriodStart))

	// One charged run settles exactly the promised cost, no more.
	receipt, err := o.SearchOnce(ctx, usageSearchInput("usage-estimate-1"))
	require.NoError(t, err)
	require.Equal(t, SearchStatusCompleted, receipt.Status)
	after, err := o.UsageEstimate(ctx, UsageOperationSearchOnce)
	require.NoError(t, err)
	require.EqualValues(t, 1, after.SettledUnits, "the executed run settles exactly one unit")
	require.EqualValues(t, 0, after.ReservedUnits)
	require.EqualValues(t, 1, after.RemainingUnits)
	require.True(t, after.WouldAdmit)

	// An unknown operation is a typed refusal, never a fabricated estimate.
	_, err = o.UsageEstimate(ctx, "bulk_evaluate")
	require.ErrorIs(t, err, ErrInvalidRequest)
}

// ---- 2. estimate unavailable never executes first ---------------------------

func TestAdmissionEstimateUnavailableNeverExecutesFirst(t *testing.T) {
	o, transport, ctx := newUsageSearchOffice(t, "owner", 96, 5)
	o.failUsageLedgerRead = func() error { return errors.New("ledger read failed") }

	// The estimate endpoint reports a typed unavailable reason.
	_, err := o.UsageEstimate(ctx, UsageOperationSearchOnce)
	require.ErrorIs(t, err, ErrAdmissionUnavailable)

	// The charged run is refused before any execution: no fetch, no search
	// row, no reservation. Never execute first and report afterwards.
	_, err = o.SearchOnce(ctx, usageSearchInput("usage-unavailable-1"))
	require.ErrorIs(t, err, ErrAdmissionUnavailable)
	require.Empty(t, transport.calls, "an unavailable estimate must block execution, not defer the cost")
	var searches int64
	require.NoError(t, o.db.Model(&searchRecord{}).Count(&searches).Error)
	require.Zero(t, searches)
	require.Zero(t, usageReservationCount(t, o))

	// Once the ledger is readable again the same request ID recovers.
	o.failUsageLedgerRead = nil
	receipt, err := o.SearchOnce(ctx, usageSearchInput("usage-unavailable-1"))
	require.NoError(t, err)
	require.Equal(t, SearchStatusCompleted, receipt.Status)
}

// ---- 3. over quota blocks only new charged runs -----------------------------

func TestOverQuotaBlocksNewChargedRunsOnly(t *testing.T) {
	o, transport, ctx := newUsageSearchOffice(t, "owner", 97, 1)

	first, err := o.SearchOnce(ctx, usageSearchInput("usage-over-1"))
	require.NoError(t, err)
	require.Equal(t, SearchStatusCompleted, first.Status)

	// A new charged run is refused with the typed, recoverable error.
	_, err = o.SearchOnce(ctx, usageSearchInput("usage-over-2"))
	require.ErrorIs(t, err, ErrSearchQuotaRefused)
	require.Len(t, transport.calls, 1, "the refused run must not reach any source")
	var searches int64
	require.NoError(t, o.db.Model(&searchRecord{}).Count(&searches).Error)
	require.EqualValues(t, 1, searches, "the refused run must not create a search row")
	require.EqualValues(t, 1, usageReservationCount(t, o), "the refused run must not pre-reserve a unit")

	// The refusal is recoverable: the same blocked request ID runs once the
	// window admits again (a wider limit models the same recovery).
	o.SetSearchQuotaLimit(2)
	recovered, err := o.SearchOnce(ctx, usageSearchInput("usage-over-2"))
	require.NoError(t, err)
	require.Equal(t, SearchStatusCompleted, recovered.Status)
}

// ---- 4. existing records stay readable without quota ------------------------

func TestOverQuotaKeepsExistingRecordsAndApplicationsReadable(t *testing.T) {
	o, _, ctx := newUsageSearchOffice(t, "owner", 98, 1)

	search, err := o.SearchOnce(ctx, usageSearchInput("usage-readable-1"))
	require.NoError(t, err)
	o.SetApplicationTaskLinker(&fakeCareerApplicationLinker{})
	_, err = o.Confirm(ctx, "skill.go", "Go", "usage-readable-confirm", 0, Source{Kind: "manual"})
	require.NoError(t, err)
	job, err := o.ImportJD(ctx, ImportJDInput{RequestID: "jd-1", RawText: completeJDText})
	require.NoError(t, err)
	evaluation, err := o.EvaluateOpportunity(ctx, EvaluateInput{RequestID: "eval-1", OpportunityID: job.OpportunityID, SnapshotID: job.SnapshotID})
	require.NoError(t, err)
	application, err := o.CreateApplication(ctx, CreateApplicationInput{
		RequestID: "app-1", OpportunityID: job.OpportunityID, SnapshotID: job.SnapshotID,
		EvaluationID: evaluation.EvaluationID, BatchIdentity: "batch-1", ExpectedRevision: 1,
	})
	require.NoError(t, err)

	// The quota is exhausted now: new charged runs are refused...
	_, err = o.SearchOnce(ctx, usageSearchInput("usage-readable-2"))
	require.ErrorIs(t, err, ErrSearchQuotaRefused)

	// ...and every existing record stays readable without any quota.
	replayed, err := o.FindSearchReceipt(ctx, "usage-readable-1")
	require.NoError(t, err)
	require.Equal(t, search.SearchID, replayed.SearchID)
	stored, err := o.Search(ctx, search.SearchID)
	require.NoError(t, err)
	require.Equal(t, search.SearchID, stored.SearchID)
	_, err = o.Evaluation(ctx, evaluation.EvaluationID)
	require.NoError(t, err)
	_, err = o.Application(ctx, application.ApplicationID)
	require.NoError(t, err)
	view, err := o.Open(ctx)
	require.NoError(t, err)
	require.NotNil(t, view)

	// Reads never consumed another unit.
	estimate, err := o.UsageEstimate(ctx, UsageOperationSearchOnce)
	require.NoError(t, err)
	require.EqualValues(t, 1, estimate.SettledUnits)
	require.EqualValues(t, 0, estimate.ReservedUnits)
	require.False(t, estimate.WouldAdmit)
}

// ---- 5. duplicate requests never double reserve or charge -------------------

func TestDuplicateRequestDoesNotDoubleReserveOrCharge(t *testing.T) {
	o, _, ctx := newUsageSearchOffice(t, "owner", 99, 3)

	// Repeating the same request ID replays the receipt...
	first, err := o.SearchOnce(ctx, usageSearchInput("usage-dup-1"))
	require.NoError(t, err)
	replay, err := o.SearchOnce(ctx, usageSearchInput("usage-dup-1"))
	require.NoError(t, err)
	firstJSON, _ := json.Marshal(first)
	replayJSON, _ := json.Marshal(replay)
	require.JSONEq(t, string(firstJSON), string(replayJSON))

	// ...and the ledger holds exactly one settled unit for it.
	require.EqualValues(t, 1, usageReservationCount(t, o), "one request ID reserves exactly one row")
	estimate, err := o.UsageEstimate(ctx, UsageOperationSearchOnce)
	require.NoError(t, err)
	require.EqualValues(t, 1, estimate.SettledUnits, "a replayed request settles once, never twice")

	// Re-entering admission for the same request ID is idempotent: the rule
	// trigger path calls the gate before and inside SearchOnce.
	s, err := getScope(ctx)
	require.NoError(t, err)
	require.NoError(t, o.searchQuotaGate.AdmitSearch(ctx, s, "usage-dup-1", usageFixtureQuery))
	require.NoError(t, o.searchQuotaGate.AdmitSearch(ctx, s, "usage-dup-1", usageFixtureQuery))
	require.EqualValues(t, 1, usageReservationCount(t, o), "admission replays never add reservations")
	estimate, err = o.UsageEstimate(ctx, UsageOperationSearchOnce)
	require.NoError(t, err)
	require.EqualValues(t, 1, estimate.SettledUnits)

	// A different request ID is a separate charged run.
	_, err = o.SearchOnce(ctx, usageSearchInput("usage-dup-2"))
	require.NoError(t, err)
	require.EqualValues(t, 2, usageReservationCount(t, o))
}

// ---- 6. payment status never alters ranking or qualification ----------------

func TestPaymentStatusDoesNotAlterRankingOrQualification(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	newOffice := func(user string, tenant uint64, limit int64, drain bool) (*Office, context.Context) {
		o, err := NewOffice(db)
		require.NoError(t, err)
		o.SetSearchQuotaLimit(limit)
		policy := &stubSourcePolicy{approvedHosts: map[string]bool{"jobs.example.test": true}}
		o.sourcePolicy = policy
		o.sourceTransport = &scriptedTransport{policy: policy, scripts: map[string]scriptedFetch{
			usageFixtureURL(): {result: SourceFetchResult{StatusCode: 200, ContentType: "text/html", Text: "https://jobs.example.test/j/3001", FinalURL: usageFixtureURL(), Complete: true}},
		}}
		o.searchRegistry = &stubSearchRegistry{sources: []VettedSearchSource{{
			ID: "src-usage", Label: "Usage Fixture Jobs",
			SearchURLTemplate: "https://jobs.example.test/listings?q={query}",
			AccessMethods:     []string{"public https listing page"}, Cities: []string{"Hangzhou"},
		}}}
		ctx := WithScope(context.Background(), Scope{UserID: user, TenantID: tenant})
		require.NoError(t, o.ClaimSpace(ctx))
		if drain {
			_, err := o.SearchOnce(ctx, usageSearchInput("drain-"+user))
			require.NoError(t, err)
		}
		return o, ctx
	}

	rich, richCtx := newOffice("rich-owner", 301, 10, false)
	broke, brokeCtx := newOffice("broke-owner", 302, 1, true)

	// The broke tenant is genuinely out of quota for new charged runs...
	_, err = broke.SearchOnce(brokeCtx, usageSearchInput("broke-blocked"))
	require.ErrorIs(t, err, ErrSearchQuotaRefused)

	// ...yet identical profile and JD inputs yield identical qualification
	// decisions and identical search-result ordering.
	for _, setup := range []struct {
		o   *Office
		ctx context.Context
	}{{rich, richCtx}, {broke, brokeCtx}} {
		_, err := setup.o.Confirm(setup.ctx, "education.graduation_year", "2027", "year-confirm", 0, Source{Kind: "manual"})
		require.NoError(t, err)
	}
	richJob, err := rich.ImportJD(richCtx, ImportJDInput{RequestID: "jd", RawText: "仅限2027届"})
	require.NoError(t, err)
	brokeJob, err := broke.ImportJD(brokeCtx, ImportJDInput{RequestID: "jd", RawText: "仅限2027届"})
	require.NoError(t, err)
	richEval, err := rich.EvaluateOpportunity(richCtx, EvaluateInput{RequestID: "eval", OpportunityID: richJob.OpportunityID, SnapshotID: richJob.SnapshotID})
	require.NoError(t, err)
	brokeEval, err := broke.EvaluateOpportunity(brokeCtx, EvaluateInput{RequestID: "eval", OpportunityID: brokeJob.OpportunityID, SnapshotID: brokeJob.SnapshotID})
	require.NoError(t, err)
	require.Equal(t, richEval.Status, brokeEval.Status, "qualification never depends on payment status")

	richFull, err := rich.Evaluation(richCtx, richEval.EvaluationID)
	require.NoError(t, err)
	brokeFull, err := broke.Evaluation(brokeCtx, brokeEval.EvaluationID)
	require.NoError(t, err)
	// The decision inputs are everything the rules actually consume: the
	// overall status, the profile fact value, and every rule outcome with its
	// reason. Identifiers, timestamps, and digests are per-space noise.
	decisionInputs := func(view Evaluation) map[string]any {
		encoded, err := json.Marshal(view)
		require.NoError(t, err)
		var decoded struct {
			Status         string `json:"status"`
			RulesetVersion string `json:"rulesetVersion"`
			Hard           struct {
				Overall string `json:"overall"`
				Rules   []struct {
					RuleID     string `json:"ruleId"`
					Criterion  string `json:"criterion"`
					Outcome    string `json:"outcome"`
					ReasonCode string `json:"reasonCode"`
					Profile    struct {
						FactKey string `json:"factKey"`
						Value   string `json:"value"`
					} `json:"profileEvidence"`
				} `json:"rules"`
			} `json:"hard"`
			Soft struct {
				Matches []string `json:"matches"`
			} `json:"soft"`
		}
		require.NoError(t, json.Unmarshal(encoded, &decoded))
		return map[string]any{
			"status": decoded.Status, "rulesetVersion": decoded.RulesetVersion,
			"hardOverall": decoded.Hard.Overall, "hardRules": decoded.Hard.Rules,
			"softMatches": decoded.Soft.Matches,
		}
	}
	require.Equal(t, decisionInputs(richFull), decisionInputs(brokeFull), "hard and soft matching inputs carry zero payment dimension")

	// Structural honesty: the evaluation body has no quota, balance, plan, or
	// payment field anywhere.
	encoded, err := json.Marshal(richFull)
	require.NoError(t, err)
	lower := string(encoded)
	for _, forbidden := range []string{"quota", "usage", "balance", "plan", "paid", "payment", "billing"} {
		require.NotContains(t, lower, forbidden, "evaluation output must not carry a payment dimension")
	}

	// The one search the broke tenant already paid for keeps the same
	// result order as the rich tenant's identical search. The rich tenant's
	// profile moved to revision 1 when its fact was confirmed.
	richInput := usageSearchInput("rich-search")
	richInput.ExpectedRevision = 1
	richSearch, err := rich.SearchOnce(richCtx, richInput)
	require.NoError(t, err)
	brokeSearch, err := broke.FindSearchReceipt(brokeCtx, "drain-broke-owner")
	require.NoError(t, err)
	require.Equal(t, richSearch.Results[0].Link, brokeSearch.Results[0].Link, "result ordering never depends on payment status")
}

// ---- 7. read-only access consumes no quota ----------------------------------

func TestReadOnlyAccessConsumesNoQuota(t *testing.T) {
	o, _, ctx := newUsageSearchOffice(t, "owner", 100, 5)

	_, err := o.SearchOnce(ctx, usageSearchInput("usage-readonly-1"))
	require.NoError(t, err)
	baseline, err := o.UsageEstimate(ctx, UsageOperationSearchOnce)
	require.NoError(t, err)
	require.EqualValues(t, 1, baseline.SettledUnits)

	// Free operations: profile intake and evaluation writes stay free, and
	// every read path stays free.
	job, err := o.ImportJD(ctx, ImportJDInput{RequestID: "jd-free", RawText: completeJDText})
	require.NoError(t, err)
	evaluation, err := o.EvaluateOpportunity(ctx, EvaluateInput{RequestID: "eval-free", OpportunityID: job.OpportunityID, SnapshotID: job.SnapshotID})
	require.NoError(t, err)
	_, err = o.ImportJD(ctx, ImportJDInput{RequestID: "jd-free", RawText: completeJDText})
	require.NoError(t, err, "replays of free writes stay free")
	_, err = o.FindSearchReceipt(ctx, "usage-readonly-1")
	require.NoError(t, err)
	_, err = o.Evaluation(ctx, evaluation.EvaluationID)
	require.NoError(t, err)
	_, err = o.Open(ctx)
	require.NoError(t, err)
	_, err = o.Changes(ctx, 0)
	require.NoError(t, err)
	_, err = o.ListSources(ctx)
	require.NoError(t, err)
	_, err = o.UsageEstimate(ctx, UsageOperationSearchOnce)
	require.NoError(t, err)

	after, err := o.UsageEstimate(ctx, UsageOperationSearchOnce)
	require.NoError(t, err)
	require.EqualValues(t, baseline.SettledUnits, after.SettledUnits, "reads and free writes never consume quota")
	require.EqualValues(t, 1, usageReservationCount(t, o))
}

// ---- 8. scope isolation -----------------------------------------------------

func TestAdmissionScopeRejectsOtherTenantAndOwner(t *testing.T) {
	o, _, ownerCtx := newUsageSearchOffice(t, "owner", 101, 2)

	_, err := o.SearchOnce(ownerCtx, usageSearchInput("usage-scope-1"))
	require.NoError(t, err)

	intruderCtx := WithScope(context.Background(), Scope{UserID: "intruder", TenantID: 102})
	require.NoError(t, o.ClaimSpace(intruderCtx))

	// The intruder's estimate never counts the owner's usage: the ledger is
	// scoped from the authenticated context, never from client input.
	intruderEstimate, err := o.UsageEstimate(intruderCtx, UsageOperationSearchOnce)
	require.NoError(t, err)
	require.EqualValues(t, 0, intruderEstimate.SettledUnits)
	require.EqualValues(t, 0, intruderEstimate.ReservedUnits)
	require.EqualValues(t, intruderEstimate.LimitUnits, intruderEstimate.RemainingUnits)
	require.True(t, intruderEstimate.WouldAdmit)

	// The intruder can spend their own quota without touching the owner...
	intruderSearch, err := o.SearchOnce(intruderCtx, usageSearchInput("usage-scope-intruder"))
	require.NoError(t, err)
	require.NotEmpty(t, intruderSearch.SearchID)
	ownerEstimate, err := o.UsageEstimate(ownerCtx, UsageOperationSearchOnce)
	require.NoError(t, err)
	require.EqualValues(t, 1, ownerEstimate.SettledUnits, "cross-tenant usage never leaks into the owner's balance")

	// ...and reservation rows carry the authenticated scope columns only.
	var rows []usageReservationRecord
	require.NoError(t, o.db.Find(&rows).Error)
	scopes := map[string]bool{}
	for _, row := range rows {
		if row.TenantID == 101 && row.UserID == "owner" {
			scopes["101:owner"] = true
		}
		if row.TenantID == 102 && row.UserID == "intruder" {
			scopes["102:intruder"] = true
		}
	}
	require.True(t, scopes["101:owner"], "the owner's reservation is stored under the authenticated scope")
	require.True(t, scopes["102:intruder"], "the intruder's reservation is stored under their own scope")

	// An unauthenticated context is refused before any ledger work.
	_, err = o.UsageEstimate(context.Background(), UsageOperationSearchOnce)
	require.Error(t, err)
}

// ---- 9. deletion purge and boundary -----------------------------------------

func TestUsageTableIncludedInDeletionPurgeAndBoundary(t *testing.T) {
	o, _, ctx := newUsageSearchOffice(t, "owner", 103, 2)

	_, err := o.SearchOnce(ctx, usageSearchInput("usage-purge-1"))
	require.NoError(t, err)
	estimate, err := o.UsageEstimate(ctx, UsageOperationSearchOnce)
	require.NoError(t, err)
	require.EqualValues(t, 1, estimate.SettledUnits)

	// The boundary explains the reservation rows before any deletion.
	boundary, err := o.CareerDeletionBoundary(ctx)
	require.NoError(t, err)
	var section *CareerDeletionSection
	for i := range boundary.InSpace {
		if boundary.InSpace[i].Section == "usage_reservations" {
			section = &boundary.InSpace[i]
		}
	}
	require.NotNil(t, section, "the deletion boundary must disclose the usage ledger")
	require.Equal(t, 1, section.Count)

	// The revision the caller observed pins the deletion intent; Confirm
	// created the profile at revision 1. The remover is the Workbench
	// projection port (empty here: this space links no external task).
	o.SetApplicationTaskRemover(&fakeCareerTaskRemover{})
	_, err = o.Confirm(ctx, "skill.go", "Go", "usage-purge-confirm", 0, Source{Kind: "manual"})
	require.NoError(t, err)
	receipt, err := o.DeleteCareer(ctx, CareerDeletionInput{RequestID: "delete-usage-1", ExpectedRevision: 1})
	require.NoError(t, err)
	require.Equal(t, DeletionStatusDeleted, receipt.Status)
	require.Zero(t, usageReservationCount(t, o), "delete_career must purge the usage ledger")
}

// ---- 10. reservation lifecycle honesty --------------------------------------

func TestAdmissionReservationIsReleasableWhenNeverClaimed(t *testing.T) {
	o, _, ctx := newUsageSearchOffice(t, "owner", 104, 1)

	// Admission succeeded but the run never claimed its search row (for
	// example an immediate revision conflict): the reserved unit is leased,
	// and once the lease expires the reconcile path releases it so a failed
	// attempt never strands the quota.
	s, err := getScope(ctx)
	require.NoError(t, err)
	require.NoError(t, o.searchQuotaGate.AdmitSearch(ctx, s, "usage-release-1", usageFixtureQuery))
	estimate, err := o.UsageEstimate(ctx, UsageOperationSearchOnce)
	require.NoError(t, err)
	require.EqualValues(t, 1, estimate.ReservedUnits)
	require.False(t, estimate.WouldAdmit, "a live reservation holds the unit")

	expired := time.Now().UTC().Add(-2 * usageReservationLease)
	require.NoError(t, o.db.Model(&usageReservationRecord{}).
		Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, "usage-release-1").
		Update("lease_until", expired).Error)
	estimate, err = o.UsageEstimate(ctx, UsageOperationSearchOnce)
	require.NoError(t, err)
	require.EqualValues(t, 0, estimate.ReservedUnits, "an expired, never-claimed reservation is released")
	require.True(t, estimate.WouldAdmit, "the released unit is spendable again")
}

// ---- 11. admission atomicity under concurrency (review F1) ------------------

func TestAdmissionIsAtomicUnderConcurrentRequests(t *testing.T) {
	// The check-then-insert of admission must be atomic: with a one-unit
	// limit, concurrent admissions under different request IDs may admit
	// exactly one — never more, whatever the interleaving. The shared-cache
	// in-memory database gives every goroutine its own connection, so the
	// statements genuinely interleave instead of being serialized by a
	// single pooled connection.
	for round := 0; round < 10; round++ {
		db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:usage_race_%d_%d?mode=memory&cache=shared", os.Getpid(), round)), &gorm.Config{})
		require.NoError(t, err)
		o, err := NewOffice(db)
		require.NoError(t, err)
		o.SetSearchQuotaLimit(1)
		policy := &stubSourcePolicy{approvedHosts: map[string]bool{"jobs.example.test": true}}
		o.sourcePolicy = policy
		o.sourceTransport = &scriptedTransport{policy: policy, scripts: map[string]scriptedFetch{}}
		ctx := WithScope(context.Background(), Scope{UserID: "owner", TenantID: 106 + uint64(round)})
		require.NoError(t, o.ClaimSpace(ctx))
		s, err := getScope(ctx)
		require.NoError(t, err)

		const racers = 8
		results := make([]error, racers)
		var start sync.WaitGroup
		start.Add(1)
		var done sync.WaitGroup
		for i := 0; i < racers; i++ {
			done.Add(1)
			go func(i int) {
				defer done.Done()
				start.Wait() // maximize the check-then-insert overlap
				results[i] = o.admitSearchUsage(ctx, s, fmt.Sprintf("usage-race-%d", i))
			}(i)
		}
		start.Done()
		done.Wait()

		admitted := 0
		for _, result := range results {
			switch {
			case result == nil:
				admitted++
			case errors.Is(result, ErrSearchQuotaRefused):
			default:
				t.Fatalf("round %d: unexpected admission error: %v", round, result)
			}
		}
		require.Equalf(t, 1, admitted, "round %d: a one-unit limit admits exactly one concurrent request", round)
		require.EqualValuesf(t, 1, usageReservationCount(t, o), "round %d: exactly one reservation row exists", round)

		// The estimate agrees with the ledger after the race.
		estimate, err := o.UsageEstimate(ctx, UsageOperationSearchOnce)
		require.NoError(t, err)
		require.EqualValuesf(t, 1, estimate.ReservedUnits+estimate.SettledUnits, "round %d: the ledger holds exactly one held unit", round)
	}
}

// TestAdmissionIsAtomicOnFileBackedSQLite reproduces review F4: a file-backed
// SQLite database (the supported deployment shape) reports writer contention
// as "database is locked", which the admission insert must NOT mistake for a
// same-request uniqueness race. A swallowed BUSY would report admission
// success with no ledger row — an uncounted charged run.
func TestAdmissionIsAtomicOnFileBackedSQLite(t *testing.T) {
	for round := 0; round < 20; round++ {
		path := filepath.Join(t.TempDir(), fmt.Sprintf("usage-file-race-%d.db", round))
		db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
		require.NoError(t, err)
		o, err := NewOffice(db)
		require.NoError(t, err)
		o.SetSearchQuotaLimit(1)
		policy := &stubSourcePolicy{approvedHosts: map[string]bool{"jobs.example.test": true}}
		o.sourcePolicy = policy
		o.sourceTransport = &scriptedTransport{policy: policy, scripts: map[string]scriptedFetch{}}
		ctx := WithScope(context.Background(), Scope{UserID: "owner", TenantID: 206 + uint64(round)})
		require.NoError(t, o.ClaimSpace(ctx))
		s, err := getScope(ctx)
		require.NoError(t, err)

		const racers = 8
		results := make([]error, racers)
		var start sync.WaitGroup
		start.Add(1)
		var done sync.WaitGroup
		for i := 0; i < racers; i++ {
			done.Add(1)
			go func(i int) {
				defer done.Done()
				start.Wait()
				results[i] = o.admitSearchUsage(ctx, s, fmt.Sprintf("usage-file-race-%d", i))
			}(i)
		}
		start.Done()
		done.Wait()

		admitted := 0
		for _, result := range results {
			switch {
			case result == nil:
				admitted++
			case errors.Is(result, ErrSearchQuotaRefused):
			default:
				t.Fatalf("round %d: unexpected admission error: %v", round, result)
			}
		}
		require.Equalf(t, 1, admitted, "round %d: every reported success must hold a ledger row — a one-unit limit admits exactly one", round)
		require.EqualValuesf(t, 1, usageReservationCount(t, o), "round %d: exactly one reservation row exists", round)
		estimate, err := o.UsageEstimate(ctx, UsageOperationSearchOnce)
		require.NoError(t, err)
		require.EqualValuesf(t, 1, estimate.ReservedUnits+estimate.SettledUnits, "round %d: the ledger holds exactly one held unit", round)
	}
}
