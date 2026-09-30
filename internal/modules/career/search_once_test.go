package career

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
)

// ---- test doubles ----------------------------------------------------------

type stubSearchRegistry struct{ sources []VettedSearchSource }

func (r *stubSearchRegistry) SearchSources() []VettedSearchSource { return r.sources }

type fakeSearchQuotaGate struct {
	refused bool
	calls   int
	onAdmit func()
}

func (g *fakeSearchQuotaGate) AdmitSearch(context.Context, Scope, string, string) error {
	g.calls++
	if g.onAdmit != nil {
		g.onAdmit()
	}
	if g.refused {
		return ErrSearchQuotaRefused
	}
	return nil
}

const searchFixtureQuery = "go engineer"

// The fixture listing mixes real absolute links (which become honest rows)
// with noise, a duplicate, and non-link text that must never become a result.
const searchFixtureListing = "高级后端工程师 https://jobs.example.test/j/1001 全职 平台后端 https://jobs.example.test/j/1002 联系 a@b.c 重复 https://jobs.example.test/j/1001 普通文字 javascript:void(0) mailto:x@y.z"

func searchFixtureURL() string {
	return "https://jobs.example.test/listings?q=" + url.QueryEscape(searchFixtureQuery)
}

func newSearchFixtureOffice(t *testing.T) (*Office, *scriptedTransport, *fakeSearchQuotaGate, context.Context) {
	t.Helper()
	o, ctx := newSourceImportOffice(t, "owner", 91)
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
	return o, transport, gate, ctx
}

func searchFixtureInput(requestID string) SearchOnceInput {
	return SearchOnceInput{RequestID: requestID, Query: searchFixtureQuery, ExpectedRevision: 0}
}

// ---- 1. one durable search per request, no continuous rule ------------------

func TestSearchOnceCreatesSingleDurableSearchPerRequest(t *testing.T) {
	o, transport, _, ctx := newSearchFixtureOffice(t)

	first, err := o.SearchOnce(ctx, searchFixtureInput("search-1"))
	require.NoError(t, err)
	require.Equal(t, SearchStatusCompleted, first.Status)
	require.NotEmpty(t, first.SearchID)
	require.Len(t, first.Results, 2)
	require.Len(t, transport.calls, 1, "one search executes exactly one fetch per source")

	// Exact replay returns the original receipt without refetching.
	replay, err := o.SearchOnce(ctx, searchFixtureInput("search-1"))
	require.NoError(t, err)
	firstJSON, _ := json.Marshal(first)
	replayJSON, _ := json.Marshal(replay)
	require.JSONEq(t, string(firstJSON), string(replayJSON))
	require.Len(t, transport.calls, 1)

	// The same request ID with changed content is a typed conflict.
	changed := searchFixtureInput("search-1")
	changed.Query = "rust engineer"
	_, err = o.SearchOnce(ctx, changed)
	require.ErrorIs(t, err, ErrIdempotencyConflict)

	var searches, results int64
	require.NoError(t, o.db.Model(&searchRecord{}).Count(&searches).Error)
	require.EqualValues(t, 1, searches, "exactly one durable search per request ID")
	require.NoError(t, o.db.Model(&searchResultRecord{}).Count(&results).Error)
	require.EqualValues(t, 2, results)

	// A one-shot search must not create any continuous rule structure. The
	// T13 schema legitimately ships rule tables, so the honest assertion is
	// data-level: a one-shot search stores no rule, schedules nothing due,
	// and records no rule run or discovery todo.
	var rules, ruleRuns, ruleTodos int64
	require.NoError(t, o.db.Model(&searchRuleRecord{}).Count(&rules).Error)
	require.Zero(t, rules, "a one-shot search must not create a rule")
	var dueRules int64
	require.NoError(t, o.db.Model(&searchRuleRecord{}).Where("next_due_at IS NOT NULL").Count(&dueRules).Error)
	require.Zero(t, dueRules, "a one-shot search must not schedule anything")
	require.NoError(t, o.db.Model(&searchRuleRunRecord{}).Count(&ruleRuns).Error)
	require.Zero(t, ruleRuns, "a one-shot search must not record a rule run")
	require.NoError(t, o.db.Model(&searchDiscoveryTodoRecord{}).Count(&ruleTodos).Error)
	require.Zero(t, ruleTodos, "a one-shot search must not enqueue a discovery todo")
	var changes int64
	require.NoError(t, o.db.Model(&change{}).Count(&changes).Error)
	require.Zero(t, changes, "a search is not a profile mutation")
	view, err := o.Open(ctx)
	require.NoError(t, err)
	require.Zero(t, view.Revision, "searches must not advance profile revision")
}

// ---- 2. result rows carry check time, qualification, link, uncertainty ------

func TestSearchOnceResultsCarryCheckTimeQualificationLinkAndUncertainty(t *testing.T) {
	o, _, _, ctx := newSearchFixtureOffice(t)
	receipt, err := o.SearchOnce(ctx, searchFixtureInput("search-rows"))
	require.NoError(t, err)
	require.Len(t, receipt.Results, 2)

	links := map[string]bool{}
	for _, row := range receipt.Results {
		require.False(t, row.CheckedAt.IsZero(), "every row must record its check time")
		require.Equal(t, SearchQualificationNeedsReview, row.Qualification, "an unevaluated job must never be claimed qualified or not qualified")
		require.Equal(t, SearchUncertaintyLowConfidence, row.Uncertainty)
		require.True(t, strings.HasPrefix(row.Link, "https://jobs.example.test/j/"), "the link must be the literal URL actually fetched")
		links[row.Link] = true
	}
	require.True(t, links["https://jobs.example.test/j/1001"])
	require.True(t, links["https://jobs.example.test/j/1002"])

	// The qualification mapping consumes the frozen T10 evaluation statuses and
	// never fabricates a conclusion from missing or unknown input.
	require.Equal(t, SearchQualificationNeedsReview, searchQualificationFromEvaluationStatus(""))
	require.Equal(t, SearchQualificationNeedsReview, searchQualificationFromEvaluationStatus(EvaluationUnknown))
	require.Equal(t, SearchQualificationQualified, searchQualificationFromEvaluationStatus(EvaluationEligible))
	require.Equal(t, SearchQualificationNotQualified, searchQualificationFromEvaluationStatus(EvaluationIneligible))
}

// TestSearchOnceDedupesSameLinkAcrossSources keeps one immutable row per link
// when two vetted sources surface the same posting.
func TestSearchOnceDedupesSameLinkAcrossSources(t *testing.T) {
	o, ctx := newSourceImportOffice(t, "owner", 95)
	policy := &stubSourcePolicy{approvedHosts: map[string]bool{"a.example.test": true, "b.example.test": true}}
	fetchURL := "https://a.example.test/listings?q=" + url.QueryEscape(searchFixtureQuery)
	otherURL := "https://b.example.test/listings?q=" + url.QueryEscape(searchFixtureQuery)
	transport := &scriptedTransport{policy: policy, scripts: map[string]scriptedFetch{
		fetchURL: {result: SourceFetchResult{StatusCode: 200, ContentType: "text/plain", Text: "岗位一 https://jobs.example.test/j/1001", FinalURL: fetchURL}},
		otherURL: {result: SourceFetchResult{StatusCode: 200, ContentType: "text/plain", Text: "岗位二 https://jobs.example.test/j/1001 岗位三 https://jobs.example.test/j/1003", FinalURL: otherURL}},
	}}
	o.sourcePolicy = policy
	o.sourceTransport = transport
	o.searchRegistry = &stubSearchRegistry{sources: []VettedSearchSource{
		{ID: "src-a", Label: "A", SearchURLTemplate: "https://a.example.test/listings?q={query}", AccessMethods: []string{"public https listing page"}, Cities: []string{"Hangzhou"}},
		{ID: "src-b", Label: "B", SearchURLTemplate: "https://b.example.test/listings?q={query}", AccessMethods: []string{"public https listing page"}, Cities: []string{"Shanghai"}},
	}}
	receipt, err := o.SearchOnce(ctx, searchFixtureInput("search-dedupe"))
	require.NoError(t, err)
	require.Equal(t, SearchStatusCompleted, receipt.Status)
	links := []string{}
	for _, row := range receipt.Results {
		links = append(links, row.Link)
	}
	require.ElementsMatch(t, []string{"https://jobs.example.test/j/1001", "https://jobs.example.test/j/1003"}, links, "one row per link across sources")
	var stored int64
	require.NoError(t, o.db.Model(&searchResultRecord{}).Count(&stored).Error)
	require.EqualValues(t, 2, stored)
}

// ---- 3. source unavailable: explicit scope + failure code, zero fabricated --

func TestSearchOnceDoesNotFabricateWhenSourceUnavailable(t *testing.T) {
	o, _, _, ctx := newSearchFixtureOffice(t)
	downURL := "https://jobs.example.test/listings?q=" + url.QueryEscape(searchFixtureQuery)
	o.sourceTransport = &scriptedTransport{scripts: map[string]scriptedFetch{
		downURL: {err: &SourceFetchError{Code: FailureNetworkError, Err: errors.New("dial tcp: connection refused")}},
	}}

	receipt, err := o.SearchOnce(ctx, searchFixtureInput("search-down"))
	require.NoError(t, err, "an unavailable source is a durable failed search, not an error response")
	require.Equal(t, SearchStatusFailed, receipt.Status)
	require.Equal(t, SearchFailureAllSourcesUnavailable, receipt.FailureCode)
	require.Empty(t, receipt.Results)
	require.NotEmpty(t, receipt.ScopeNotes, "the response must state the actual scope of the failure")
	require.Len(t, receipt.Coverage.Sources, 1)
	require.False(t, receipt.Coverage.Sources[0].Available)
	require.Equal(t, FailureNetworkError, receipt.Coverage.Sources[0].FailureCode)

	var results int64
	require.NoError(t, o.db.Model(&searchResultRecord{}).Count(&results).Error)
	require.Zero(t, results, "zero fabricated rows when nothing could be fetched")
	encoded, _ := json.Marshal(receipt)
	require.NotContains(t, string(encoded), "connection refused", "upstream failure detail must not leak")

	replay, err := o.SearchOnce(ctx, searchFixtureInput("search-down"))
	require.NoError(t, err)
	require.Equal(t, receipt, replay)
}

// ---- 4. truthful coverage listing, no national claim -------------------------

func TestSearchOnceCoverageListingIsTruthfulWithoutNationalClaim(t *testing.T) {
	o, _, _, ctx := newSearchFixtureOffice(t)
	// Production defaults: empty source registry and the empty source policy
	// mean no vetted source exists at all.
	o.searchRegistry = emptySearchSourceRegistry{}
	o.sourcePolicy = emptySourcePolicy{}
	_, err := o.sourcePolicy.Verify("https://jobs.example.test/listings")
	require.Error(t, err, "the production allowlist starts empty")

	receipt, err := o.SearchOnce(ctx, searchFixtureInput("search-coverage"))
	require.NoError(t, err)
	require.Equal(t, SearchStatusFailed, receipt.Status)
	require.Equal(t, SearchFailureNoVettedSources, receipt.FailureCode)
	require.Empty(t, receipt.Coverage.Sources, "coverage must list exactly the actually vetted sources: none today")
	require.Empty(t, receipt.Results)
	require.NotEmpty(t, receipt.ScopeNotes)

	encoded, err := json.Marshal(receipt)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "全国", "no national coverage claim may appear anywhere in the response")
	require.NotContains(t, string(encoded), "national")
	// No transport may exist to fabricate from — assert via a fresh scripted
	// transport that stays silent.
	silent := &scriptedTransport{scripts: map[string]scriptedFetch{}}
	o.sourceTransport = silent
	replay, err := o.SearchOnce(ctx, searchFixtureInput("search-coverage"))
	require.NoError(t, err)
	require.Equal(t, receipt, replay)
	require.Empty(t, silent.calls, "with no vetted source nothing may be fetched")
}

// ---- 5. failure, unknown outcome, and quota refusal are recoverable ----------

func TestSearchOnceFailureUnknownAndQuotaRefusalAreRecoverable(t *testing.T) {
	o, _, gate, ctx := newSearchFixtureOffice(t)

	// Quota refusal is typed, leaves no durable state, and the same request ID
	// can be replayed once quota is available.
	gate.refused = true
	_, err := o.SearchOnce(ctx, searchFixtureInput("search-recover"))
	require.ErrorIs(t, err, ErrSearchQuotaRefused)
	var refused int64
	require.NoError(t, o.db.Model(&searchRecord{}).Count(&refused).Error)
	require.Zero(t, refused, "a refused search must not consume the request ID")
	gate.refused = false
	afterQuota, err := o.SearchOnce(ctx, searchFixtureInput("search-recover"))
	require.NoError(t, err)
	require.Equal(t, SearchStatusCompleted, afterQuota.Status)

	// An uncertain commit outcome is typed unknown with the original request
	// ID; the same request ID recovers after the claim lease expires.
	unknown := searchFixtureInput("search-unknown")
	o.failSearchTerminalCommit = func() error { return sqlite3.Error{Code: sqlite3.ErrBusy} }
	_, err = o.SearchOnce(ctx, unknown)
	require.ErrorIs(t, err, ErrOutcomeUnknown)
	var unknownErr *OutcomeUnknownError
	require.ErrorAs(t, err, &unknownErr)
	require.Equal(t, unknown.RequestID, unknownErr.RequestID)
	var claim searchRecord
	require.NoError(t, o.db.Where("request_id=?", unknown.RequestID).First(&claim).Error)
	require.Equal(t, searchStatusClaiming, claim.Status)
	o.failSearchTerminalCommit = nil
	// Expire the lease the way elapsed time would, then replay the exact
	// original request to take the claim over.
	require.NoError(t, o.db.Model(&searchRecord{}).Where("request_id=?", unknown.RequestID).
		Update("lease_until", claim.CreatedAt.Add(-time.Minute)).Error)
	recovered, err := o.SearchOnce(ctx, unknown)
	require.NoError(t, err)
	require.Equal(t, SearchStatusCompleted, recovered.Status)
	require.Len(t, recovered.Results, 2)
}

// ---- 6. scope isolation ------------------------------------------------------

func TestSearchOnceScopeRejectsOtherTenantAndOwner(t *testing.T) {
	o, _, _, ctx := newSearchFixtureOffice(t)
	receipt, err := o.SearchOnce(ctx, searchFixtureInput("search-scope"))
	require.NoError(t, err)

	otherUser := WithScope(context.Background(), Scope{UserID: "intruder", TenantID: 91})
	_, err = o.SearchOnce(otherUser, searchFixtureInput("search-scope-intruder"))
	require.ErrorIs(t, err, ErrUnauthorized)
	_, err = o.FindSearchReceipt(otherUser, "search-scope")
	require.ErrorIs(t, err, ErrUnauthorized)
	_, err = o.Search(otherUser, receipt.SearchID)
	require.ErrorIs(t, err, ErrUnauthorized)

	otherTenant := WithScope(context.Background(), Scope{UserID: "owner", TenantID: 92})
	_, err = o.SearchOnce(otherTenant, searchFixtureInput("search-scope-intruder"))
	require.ErrorIs(t, err, ErrUnauthorized)

	// A second properly-claimed space cannot read the first owner's search and
	// must not learn whether it exists.
	neighbor, neighborCtx := newSourceImportOffice(t, "neighbor", 93)
	_, err = neighbor.Search(neighborCtx, receipt.SearchID)
	require.ErrorIs(t, err, ErrSearchNotFound)
	_, err = neighbor.FindSearchReceipt(neighborCtx, "search-scope")
	require.ErrorIs(t, err, ErrSearchNotFound)
	_, err = o.Search(ctx, "not-a-search")
	require.ErrorIs(t, err, ErrSearchNotFound)
}

// ---- 7. revision conflict returns the current revision -----------------------

func TestSearchOnceRevisionConflictReturnsCurrentRevision(t *testing.T) {
	o, _, _, ctx := newSearchFixtureOffice(t)
	_, err := o.Act(ctx, "propose", "", "preference.city", "Hangzhou", "rev-seed", 0, Source{Kind: "manual"})
	require.NoError(t, err)

	stale := searchFixtureInput("search-rev")
	stale.ExpectedRevision = 0
	_, err = o.SearchOnce(ctx, stale)
	require.ErrorIs(t, err, ErrRevisionConflict)
	var conflict *RevisionConflictError
	require.ErrorAs(t, err, &conflict)
	require.Equal(t, uint64(1), conflict.CurrentRevision, "the conflict must return the current revision")

	current := searchFixtureInput("search-rev")
	current.ExpectedRevision = 1
	receipt, err := o.SearchOnce(ctx, current)
	require.NoError(t, err)
	require.Equal(t, SearchStatusCompleted, receipt.Status)
	view, err := o.Open(ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(1), view.Revision, "a search must not advance the profile revision")

	// HTTP mapping: 409 with the current revision in the body.
	h := &Handler{office: o, members: &memberListStub{members: []*types.TenantMember{{UserID: "owner", TenantID: 91, Role: types.TenantRoleOwner}}}}
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	body, _ := json.Marshal(SearchOnceInput{RequestID: "search-rev-http", Query: searchFixtureQuery, ExpectedRevision: 0})
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/career/searches", strings.NewReader(string(body))).
		WithContext(context.WithValue(context.WithValue(context.Background(), types.UserIDContextKey, "owner"), types.TenantIDContextKey, uint64(91)))
	h.SearchOnce(c)
	require.Equal(t, 409, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), `"code":"revision_conflict"`)
	require.Contains(t, rec.Body.String(), `"currentRevision":1`)
}

// ---- 8. contract evidence: real allowed source through the production transport

// TestSearchOnceAllowedSourceContractViaProductionTransport runs the core
// chain against a live "allowed source": the policy approves the host, the
// production transport performs the real HTTP fetch with its SSRF guards, and
// only links that literally appeared in the fetched text become rows.
func TestSearchOnceAllowedSourceContractViaProductionTransport(t *testing.T) {
	var hits int32
	var seenQuery atomic.Value
	listing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		seenQuery.Store(r.URL.Query().Get("q"))
		_, _ = io.WriteString(w, `<html><body>
			<a href="/j/2001">Go 后端工程师 https://search.example.test/j/2001</a>
			<a href="/j/2002">数据平台工程师 https://search.example.test/j/2002</a>
			</body></html>`)
	}))
	t.Cleanup(listing.Close)
	listingURL, err := url.Parse(listing.URL)
	require.NoError(t, err)
	host := "search.example.test:" + listingURL.Port()

	o, ctx := newSourceImportOffice(t, "owner", 94)
	policy := &stubSourcePolicy{approvedHosts: map[string]bool{"search.example.test": true}}
	o.sourcePolicy = policy
	o.sourceTransport = newCareerSourceTransport(policy, transportTestDialer(t, map[string]string{
		"search.example.test": listing.Listener.Addr().String(),
	}))
	o.searchRegistry = &stubSearchRegistry{sources: []VettedSearchSource{{
		ID:                "src-live",
		Label:             "Live Allowed Jobs",
		SearchURLTemplate: "http://" + host + "/listings?q={query}",
		AccessMethods:     []string{"public http listing page"},
		Cities:            []string{"Shanghai"},
	}}}

	receipt, err := o.SearchOnce(ctx, SearchOnceInput{RequestID: "search-live", Query: searchFixtureQuery, ExpectedRevision: 0})
	require.NoError(t, err)
	require.Equal(t, SearchStatusCompleted, receipt.Status)
	require.EqualValues(t, 1, atomic.LoadInt32(&hits), "the allowed source is fetched exactly once")
	require.Equal(t, searchFixtureQuery, seenQuery.Load(), "the user instruction is passed to the source verbatim")
	require.Len(t, receipt.Results, 2)
	links := map[string]bool{}
	for _, row := range receipt.Results {
		require.Equal(t, SearchQualificationNeedsReview, row.Qualification)
		require.Equal(t, SearchUncertaintyLowConfidence, row.Uncertainty)
		require.False(t, row.CheckedAt.IsZero())
		links[row.Link] = true
	}
	require.True(t, links["https://search.example.test/j/2001"])
	require.True(t, links["https://search.example.test/j/2002"])
	require.Len(t, receipt.Coverage.Sources, 1)
	require.True(t, receipt.Coverage.Sources[0].Available)
	require.Equal(t, "src-live", receipt.Coverage.Sources[0].SourceID)
	require.Equal(t, []string{"Shanghai"}, receipt.Coverage.Sources[0].Cities)

	// Read-back paths agree with the receipt.
	byRequest, err := o.FindSearchReceipt(ctx, "search-live")
	require.NoError(t, err)
	require.Equal(t, receipt, byRequest)
	byID, err := o.Search(ctx, receipt.SearchID)
	require.NoError(t, err)
	require.Equal(t, receipt, byID)
}
