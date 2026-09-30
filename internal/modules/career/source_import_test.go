package career

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// ---- test doubles -------------------------------------------------------

type stubSourcePolicy struct {
	approvedHosts map[string]bool
	verifyCalls   []string
	redirectCalls [][2]string
}

func (p *stubSourcePolicy) Verify(rawURL string) (ApprovedSource, error) {
	p.verifyCalls = append(p.verifyCalls, rawURL)
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Hostname() == "" || !p.approvedHosts[parsed.Hostname()] {
		return ApprovedSource{}, errors.New("source host is not vetted")
	}
	return ApprovedSource{AdapterID: "stub-adapter", AdapterVersion: "1"}, nil
}

func (p *stubSourcePolicy) VerifyRedirect(current, next string) error {
	p.redirectCalls = append(p.redirectCalls, [2]string{current, next})
	parsed, err := url.Parse(next)
	if err != nil || parsed.Hostname() == "" || !p.approvedHosts[parsed.Hostname()] {
		return errors.New("redirect target is not an approved source")
	}
	return nil
}

type scriptedFetch struct {
	redirectTo string // when set the transport consults the policy before "following"
	result     SourceFetchResult
	err        error
}

// scriptedTransport records every fetch and consults the policy when a script
// emulates a redirect, mirroring the production adapter contract.
type scriptedTransport struct {
	policy  SourcePolicy
	calls   []string
	scripts map[string]scriptedFetch
}

func (t *scriptedTransport) Fetch(_ context.Context, _ ApprovedSource, rawURL string) (SourceFetchResult, error) {
	t.calls = append(t.calls, rawURL)
	script := t.scripts[rawURL]
	if script.redirectTo != "" {
		if err := t.policy.VerifyRedirect(rawURL, script.redirectTo); err != nil {
			return SourceFetchResult{}, &SourceFetchError{Code: FailureRedirectDisallowed}
		}
	}
	if script.err != nil {
		return SourceFetchResult{}, script.err
	}
	return script.result, nil
}

func newSourceImportOffice(t *testing.T, user string, tenant uint64) (*Office, context.Context) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	o, err := NewOffice(db)
	require.NoError(t, err)
	ctx := WithScope(context.Background(), Scope{UserID: user, TenantID: tenant})
	require.NoError(t, o.ClaimSpace(ctx))
	return o, ctx
}

const completeJDText = "高级后端工程师\n工作职责：\n负责核心服务的架构设计与实现\n负责在线系统的稳定性与性能优化\n任职要求：\n本科及以上学历，计算机相关专业\n三年以上后端开发经验\n熟悉 Go 或 Java 生态与分布式系统"

func approvedSourceOffice(t *testing.T, scripts map[string]scriptedFetch) (*Office, *scriptedTransport, context.Context) {
	t.Helper()
	o, ctx := newSourceImportOffice(t, "owner", 81)
	policy := &stubSourcePolicy{approvedHosts: map[string]bool{"jobs.example.com": true}}
	transport := &scriptedTransport{policy: policy, scripts: scripts}
	o.sourcePolicy = policy
	o.sourceTransport = transport
	return o, transport, ctx
}

// ---- 1. complete import -------------------------------------------------

func TestImportURLCompleteCreatesImmutableOpportunityEvidence(t *testing.T) {
	rawURL := "https://jobs.example.com/posting/123?from=feed#jd"
	o, transport, ctx := approvedSourceOffice(t, map[string]scriptedFetch{
		rawURL: {result: SourceFetchResult{StatusCode: 200, ContentType: "text/html; charset=utf-8", Text: completeJDText, FinalURL: rawURL, Complete: true}},
	})

	first, err := o.ImportURL(ctx, ImportURLInput{RequestID: "url-1", URL: rawURL})
	require.NoError(t, err)
	require.Equal(t, SourceStatusComplete, first.SourceStatus)
	require.Equal(t, CompletenessComplete, first.Completeness)
	require.Empty(t, first.FailureCode)
	require.False(t, first.NeedsUserJD)
	require.Equal(t, rawURL, first.SubmittedURL, "the exact submitted URL must be preserved")
	require.NotEmpty(t, first.OpportunityID)
	require.NotEmpty(t, first.ObservationID)
	require.NotEmpty(t, first.SnapshotID)
	require.False(t, first.AcquiredAt.IsZero())
	require.Len(t, transport.calls, 1)

	second, err := o.ImportURL(ctx, ImportURLInput{RequestID: "url-1", URL: rawURL})
	require.NoError(t, err)
	firstJSON, _ := json.Marshal(first)
	secondJSON, _ := json.Marshal(second)
	require.JSONEq(t, string(firstJSON), string(secondJSON))
	require.Len(t, transport.calls, 1, "replay must be served from the durable receipt without refetching")

	evidence, err := o.OpportunityEvidence(ctx, first.OpportunityID, first.SnapshotID)
	require.NoError(t, err)
	require.Equal(t, completeJDText, evidence.RawText)
	digest := sha256.Sum256([]byte(completeJDText))
	require.Equal(t, hex.EncodeToString(digest[:]), evidence.RawSHA256)
	require.Equal(t, "url", evidence.Source.Kind)
	require.Equal(t, rawURL, evidence.Source.ReferenceID)
	require.Equal(t, OpportunityNeedsReview, evidence.Status)
	for _, field := range []ExtractedValue{evidence.Extracted.Title, evidence.Extracted.Company, evidence.Extracted.Location, evidence.Extracted.Batch, evidence.Extracted.Requirements} {
		require.Equal(t, "unknown", field.State)
		require.Empty(t, field.Value)
	}

	for _, model := range []any{&opportunity{}, &opportunityObservation{}, &opportunitySnapshot{}, &opportunityReceipt{}} {
		var count int64
		require.NoError(t, o.db.Model(model).Count(&count).Error)
		require.EqualValues(t, 1, count)
	}
	view, err := o.Open(ctx)
	require.NoError(t, err)
	require.Zero(t, view.Revision, "URL imports must not advance profile revision")
}

// ---- 2. policy_unverified ------------------------------------------------

func TestImportURLPolicyUnverifiedDoesNotFetchAndRequestsJD(t *testing.T) {
	o, ctx := newSourceImportOffice(t, "owner", 82)
	transport := &scriptedTransport{scripts: map[string]scriptedFetch{}}
	o.sourceTransport = transport
	// Production default: the source allowlist starts empty, so every host is
	// unverified and no network attempt may happen.
	_, err := o.sourcePolicy.Verify("https://any.example/job/1")
	require.Error(t, err)

	rawURL := "https://any.example/job/1"
	result, err := o.ImportURL(ctx, ImportURLInput{RequestID: "url-unverified", URL: rawURL})
	require.NoError(t, err)
	require.Equal(t, SourceStatusPolicyUnverified, result.SourceStatus)
	require.Equal(t, CompletenessUnknown, result.Completeness)
	require.Equal(t, FailureSourceUnverified, result.FailureCode)
	require.True(t, result.NeedsUserJD)
	require.Equal(t, rawURL, result.SubmittedURL)
	require.Empty(t, transport.calls, "unapproved source must be recorded without any network attempt")

	observations, err := o.OpportunityObservations(ctx, result.OpportunityID)
	require.NoError(t, err)
	require.Len(t, observations, 1)
	require.Equal(t, SourceStatusPolicyUnverified, observations[0].SourceStatus)
	require.True(t, observations[0].NeedsUserJD)
	require.Equal(t, rawURL, observations[0].SubmittedURL)
	evidence, err := o.OpportunityEvidence(ctx, result.OpportunityID, result.SnapshotID)
	require.NoError(t, err)
	require.Empty(t, evidence.RawText)
	emptyDigest := sha256.Sum256(nil)
	require.Equal(t, hex.EncodeToString(emptyDigest[:]), evidence.RawSHA256)
	require.Equal(t, OpportunityNeedsReview, evidence.Status)

	replay, err := o.ImportURL(ctx, ImportURLInput{RequestID: "url-unverified", URL: rawURL})
	require.NoError(t, err)
	require.Equal(t, result, replay)
	require.Empty(t, transport.calls)
}

// ---- 3. login / summary / timeout classifications ------------------------

func TestImportURLLoginSummaryMissingAndTimeoutClassifications(t *testing.T) {
	loginURL := "https://jobs.example.com/posting/login-wall"
	summaryURL := "https://jobs.example.com/posting/summary"
	timeoutURL := "https://jobs.example.com/posting/slow"
	o, _, ctx := approvedSourceOffice(t, map[string]scriptedFetch{
		loginURL: {result: SourceFetchResult{StatusCode: 200, ContentType: "text/html", Text: "想要查看完整职位详情，请先登录", LoginWall: true}},
		summaryURL: {result: SourceFetchResult{
			StatusCode: 200, ContentType: "text/html",
			Text:     "2026 届校园招聘（摘要）：本科及以上学历，计算机相关专业，详见官网完整职位页。",
			FinalURL: summaryURL,
		}},
		timeoutURL: {err: &SourceFetchError{Code: FailureTimeout, Err: errors.New("context deadline exceeded")}},
	})

	login, err := o.ImportURL(ctx, ImportURLInput{RequestID: "url-login", URL: loginURL})
	require.NoError(t, err)
	require.Equal(t, SourceStatusLoginRequired, login.SourceStatus)
	require.Equal(t, FailureLoginRequired, login.FailureCode)
	require.Equal(t, CompletenessUnknown, login.Completeness)
	require.True(t, login.NeedsUserJD)

	summary, err := o.ImportURL(ctx, ImportURLInput{RequestID: "url-summary", URL: summaryURL})
	require.NoError(t, err)
	require.Equal(t, SourceStatusPartial, summary.SourceStatus)
	require.Equal(t, CompletenessIncomplete, summary.Completeness)
	require.Empty(t, summary.FailureCode, "an incomplete summary is not a transport failure")
	require.True(t, summary.NeedsUserJD)
	summaryEvidence, err := o.OpportunityEvidence(ctx, summary.OpportunityID, summary.SnapshotID)
	require.NoError(t, err)
	require.Contains(t, summaryEvidence.RawText, "2026 届校园招聘", "partial text stays traceable as evidence")

	timedOut, err := o.ImportURL(ctx, ImportURLInput{RequestID: "url-timeout", URL: timeoutURL})
	require.NoError(t, err)
	require.Equal(t, SourceStatusTimedOut, timedOut.SourceStatus)
	require.Equal(t, FailureTimeout, timedOut.FailureCode)
	require.True(t, timedOut.NeedsUserJD)

	for _, result := range []ImportURLResult{login, timedOut} {
		evidence, err := o.OpportunityEvidence(ctx, result.OpportunityID, result.SnapshotID)
		require.NoError(t, err)
		require.Empty(t, evidence.RawText, "failed observations persist an explicit empty snapshot")
		emptyDigest := sha256.Sum256(nil)
		require.Equal(t, hex.EncodeToString(emptyDigest[:]), evidence.RawSHA256)
		require.Equal(t, OpportunityNeedsReview, evidence.Status)
	}
	var count int64
	require.NoError(t, o.db.Model(&opportunitySnapshot{}).Count(&count).Error)
	require.EqualValues(t, 3, count)
}

// ---- 4. redirect rejection ------------------------------------------------

func TestImportURLRejectsUnapprovedOrPrivateRedirect(t *testing.T) {
	t.Run("service maps a policy-rejected redirect to a bounded durable failure", func(t *testing.T) {
		originURL := "https://jobs.example.com/posting/hop"
		privateRedirect := "https://192.168.1.20/job/1"
		o, _, ctx := approvedSourceOffice(t, map[string]scriptedFetch{
			originURL: {redirectTo: privateRedirect},
		})
		result, err := o.ImportURL(ctx, ImportURLInput{RequestID: "url-redirect", URL: originURL})
		require.NoError(t, err)
		require.Equal(t, SourceStatusFetchFailed, result.SourceStatus)
		require.Equal(t, FailureRedirectDisallowed, result.FailureCode)
		require.True(t, result.NeedsUserJD)
		require.Equal(t, originURL, result.SubmittedURL)
		encoded, err := json.Marshal(result)
		require.NoError(t, err)
		require.NotContains(t, string(encoded), "192.168.1.20", "redirect details must not leak")
	})

	t.Run("production transport rejects at the redirect and never dials the target", func(t *testing.T) {
		longBody := "<html><body>职位描述 职责与要求 高级后端工程师岗位，负责核心服务的架构设计与实现，负责在线系统的稳定性与性能优化，参与技术方案评审。任职要求：本科及以上学历，计算机相关专业，三年以上后端开发经验，熟悉分布式系统与常用中间件。</body></html>"
		var originHits, approvedHits, deniedHits int32
		denied := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&deniedHits, 1)
			_, _ = io.WriteString(w, "must never be fetched")
		}))
		approved := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&approvedHits, 1)
			_, _ = io.WriteString(w, longBody)
		}))
		// Explicit ports keep the production transport's dial on the mapped
		// local listeners; hostnames stay policy-owned.
		approvedURL := "http://approved.example.test:" + portOf(t, approved)
		origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/approved-hop":
				http.Redirect(w, r, approvedURL+"/final", http.StatusFound)
			case "/denied-hop":
				// Redirect to a host that the policy never approved.
				w.Header().Set("Location", "http://denied.example.test/job")
				w.WriteHeader(http.StatusFound)
			case "/private-hop":
				// Redirect to a literal private IP host; the policy is
				// deliberately permissive so only the SSRF guard can stop it.
				w.Header().Set("Location", "http://10.99.0.5:9/private")
				w.WriteHeader(http.StatusFound)
			default:
				atomic.AddInt32(&originHits, 1)
				_, _ = io.WriteString(w, longBody)
			}
		}))
		t.Cleanup(func() {
			origin.Close()
			denied.Close()
			approved.Close()
		})
		originURL := "http://origin.example.test:" + portOf(t, origin)

		policy := &stubSourcePolicy{approvedHosts: map[string]bool{
			"origin.example.test":   true,
			"approved.example.test": true,
		}}
		transport := newCareerSourceTransport(policy, transportTestDialer(t, map[string]string{
			"origin.example.test":   origin.Listener.Addr().String(),
			"approved.example.test": approved.Listener.Addr().String(),
		}))
		ctx := context.Background()

		direct, err := transport.Fetch(ctx, mustApprovedSource(t, policy, originURL+"/direct"), originURL+"/direct")
		require.NoError(t, err)
		require.True(t, direct.Complete, "text=%q", direct.Text)
		require.EqualValues(t, 1, atomic.LoadInt32(&originHits))

		// Approved redirect is followed and fetched.
		_, err = transport.Fetch(ctx, mustApprovedSource(t, policy, originURL+"/approved-hop"), originURL+"/approved-hop")
		require.NoError(t, err)
		require.EqualValues(t, 1, atomic.LoadInt32(&approvedHits))

		// Unapproved redirect is rejected at the redirect, never fetched.
		_, err = transport.Fetch(ctx, mustApprovedSource(t, policy, originURL+"/denied-hop"), originURL+"/denied-hop")
		requireFetchFailureCode(t, err, FailureRedirectDisallowed)
		require.EqualValues(t, 0, atomic.LoadInt32(&deniedHits), "the unapproved redirect target must not be dialed")

		// Private-IP redirect is rejected by the transport even though the
		// permissive policy approved every host it was asked about.
		_, err = transport.Fetch(ctx, mustApprovedSource(t, policy, originURL+"/private-hop"), originURL+"/private-hop")
		requireFetchFailureCode(t, err, FailureRedirectDisallowed)
		require.EqualValues(t, 1, atomic.LoadInt32(&originHits))
	})
}

// ---- 5. partial text never infers hard fields -----------------------------

func TestImportURLDoesNotInferHardFieldsFromPartialText(t *testing.T) {
	summaryURL := "https://jobs.example.com/posting/summary-only"
	o, _, ctx := approvedSourceOffice(t, map[string]scriptedFetch{
		summaryURL: {result: SourceFetchResult{StatusCode: 200, ContentType: "text/html", Text: "摘要：仅限 2026 届毕业生，本科及以上学历，中共党员优先。完整职责要求请见原网站。"}},
	})
	// Even with a fully configured extractor, URL text must never infer
	// graduation, degree, or other hard conditions.
	o.opportunityExtractor = func(string) (OpportunityFields, error) {
		fields := unknownOpportunityFields()
		fields.Batch = ExtractedValue{State: "known", Value: "2026"}
		fields.Requirements = ExtractedValue{State: "known", Value: "本科"}
		return fields, nil
	}

	result, err := o.ImportURL(ctx, ImportURLInput{RequestID: "url-partial", URL: summaryURL})
	require.NoError(t, err)
	require.Equal(t, SourceStatusPartial, result.SourceStatus)
	require.Equal(t, CompletenessIncomplete, result.Completeness)
	require.True(t, result.NeedsUserJD)

	evidence, err := o.OpportunityEvidence(ctx, result.OpportunityID, result.SnapshotID)
	require.NoError(t, err)
	require.Equal(t, OpportunityNeedsReview, evidence.Status)
	for _, field := range []ExtractedValue{evidence.Extracted.Title, evidence.Extracted.Company, evidence.Extracted.Location, evidence.Extracted.Batch, evidence.Extracted.Requirements} {
		require.Equal(t, "unknown", field.State, "URL text must never infer hard conditions")
		require.Empty(t, field.Value)
	}
}

// ---- 6. replay conflict + concurrency --------------------------------------

func TestImportURLReplayConflictAndConcurrentSingleObservation(t *testing.T) {
	rawURL := "https://jobs.example.com/posting/777"
	changedURL := "https://jobs.example.com/posting/778"
	o, _, ctx := approvedSourceOffice(t, map[string]scriptedFetch{
		rawURL:     {result: SourceFetchResult{StatusCode: 200, ContentType: "text/html", Text: completeJDText, Complete: true}},
		changedURL: {result: SourceFetchResult{StatusCode: 200, ContentType: "text/html", Text: completeJDText, Complete: true}},
	})

	stored, err := o.ImportURL(ctx, ImportURLInput{RequestID: "url-replay", URL: rawURL})
	require.NoError(t, err)
	require.NotEmpty(t, stored.SnapshotID)
	_, err = o.ImportURL(ctx, ImportURLInput{RequestID: "url-replay", URL: changedURL})
	require.ErrorIs(t, err, ErrIdempotencyConflict)
	var count int64
	require.NoError(t, o.db.Model(&opportunityObservation{}).Count(&count).Error)
	require.EqualValues(t, 1, count)

	dsn := fmt.Sprintf("file:career-source-import-concurrent-%s?mode=memory&cache=shared&_busy_timeout=1000", strings.ReplaceAll(uuid.NewString(), "-", ""))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(8)
	concurrent, err := NewOffice(db)
	require.NoError(t, err)
	concurrentCtx := WithScope(context.Background(), Scope{UserID: "owner", TenantID: 83})
	require.NoError(t, concurrent.ClaimSpace(concurrentCtx))
	concurrent.sourcePolicy = &stubSourcePolicy{approvedHosts: map[string]bool{"jobs.example.com": true}}
	concurrent.sourceTransport = &scriptedTransport{
		policy: concurrent.sourcePolicy,
		scripts: map[string]scriptedFetch{
			rawURL: {result: SourceFetchResult{StatusCode: 200, ContentType: "text/html", Text: completeJDText, Complete: true}},
		},
	}

	const workers = 8
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := concurrent.ImportURL(concurrentCtx, ImportURLInput{RequestID: "url-concurrent", URL: rawURL})
			errs <- e
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			require.ErrorIs(t, err, ErrOutcomeUnknown, "concurrent callers may only fail with a typed unknown outcome")
		}
	}
	// Concurrent identical requests append only one observation; retries never
	// rewrite history, they replay the single committed snapshot.
	require.NoError(t, db.Model(&opportunityObservation{}).Count(&count).Error)
	require.EqualValues(t, 1, count)
	require.NoError(t, db.Model(&opportunitySnapshot{}).Count(&count).Error)
	require.EqualValues(t, 1, count)
	require.NoError(t, db.Model(&opportunityReceipt{}).Count(&count).Error)
	require.EqualValues(t, 1, count)
	final, err := concurrent.ImportURL(concurrentCtx, ImportURLInput{RequestID: "url-concurrent", URL: rawURL})
	require.NoError(t, err)
	require.Equal(t, SourceStatusComplete, final.SourceStatus)
}

// ---- 7. manual JD append after URL failure ----------------------------------

func TestManualJDAfterURLCreatesNewSnapshotAndPreservesTrace(t *testing.T) {
	loginURL := "https://jobs.example.com/posting/locked"
	o, _, ctx := approvedSourceOffice(t, map[string]scriptedFetch{
		loginURL: {result: SourceFetchResult{StatusCode: 200, ContentType: "text/html", Text: "请登录后查看职位", LoginWall: true}},
	})

	urlResult, err := o.ImportURL(ctx, ImportURLInput{RequestID: "url-then-jd", URL: loginURL})
	require.NoError(t, err)
	require.Equal(t, SourceStatusLoginRequired, urlResult.SourceStatus)

	fullJD := "高级后端工程师\n职责：负责核心服务\n要求：本科及以上学历"
	manual, err := o.ImportJD(ctx, ImportJDInput{
		RequestID:          "url-then-jd-manual",
		RawText:            fullJD,
		OpportunityID:      urlResult.OpportunityID,
		PriorObservationID: urlResult.ObservationID,
	})
	require.NoError(t, err)
	require.Equal(t, urlResult.OpportunityID, manual.OpportunityID)
	require.NotEqual(t, urlResult.ObservationID, manual.ObservationID)
	require.NotEqual(t, urlResult.SnapshotID, manual.SnapshotID)

	observations, err := o.OpportunityObservations(ctx, urlResult.OpportunityID)
	require.NoError(t, err)
	require.Len(t, observations, 2, "both observations stay open and traceable")
	require.Equal(t, urlResult.ObservationID, observations[0].ObservationID)
	require.Equal(t, "url", observations[0].Source.Kind)
	require.Equal(t, SourceStatusLoginRequired, observations[0].SourceStatus)
	require.Equal(t, loginURL, observations[0].SubmittedURL)
	require.Equal(t, manual.ObservationID, observations[1].ObservationID)
	require.Equal(t, "manual_paste", observations[1].Source.Kind)
	require.Empty(t, observations[1].SourceStatus)

	original, err := o.OpportunityEvidence(ctx, urlResult.OpportunityID, urlResult.SnapshotID)
	require.NoError(t, err)
	require.Empty(t, original.RawText, "the original URL observation is never rewritten")
	appended, err := o.OpportunityEvidence(ctx, manual.OpportunityID, manual.SnapshotID)
	require.NoError(t, err)
	require.Equal(t, fullJD, appended.RawText)

	// Linkage must be verified against the owner's own URL observation.
	_, err = o.ImportJD(ctx, ImportJDInput{RequestID: "url-then-jd-bad", RawText: fullJD, OpportunityID: urlResult.OpportunityID, PriorObservationID: "missing-observation"})
	require.Error(t, err)
	_, err = o.ImportJD(ctx, ImportJDInput{RequestID: "url-then-jd-bad-2", RawText: fullJD, OpportunityID: urlResult.OpportunityID, PriorObservationID: manual.ObservationID})
	require.ErrorIs(t, err, ErrInvalidRequest)
}

// ---- 8. scope + error sanitization -------------------------------------------

func TestImportURLScopeAndErrorSanitization(t *testing.T) {
	rawURL := "https://jobs.example.com/posting/leaky"
	upstreamDetail := "dial tcp 10.42.0.7:443: connect: connection refused (credential=seekrit)"
	o, _, ctx := approvedSourceOffice(t, map[string]scriptedFetch{
		rawURL: {err: &SourceFetchError{Code: FailureNetworkError, Err: errors.New(upstreamDetail)}},
	})

	result, err := o.ImportURL(ctx, ImportURLInput{RequestID: "url-sanitize", URL: rawURL})
	require.NoError(t, err)
	require.Equal(t, SourceStatusFetchFailed, result.SourceStatus)
	require.Equal(t, FailureNetworkError, result.FailureCode)
	encoded, err := json.Marshal(result)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "10.42.0.7")
	require.NotContains(t, string(encoded), "seekrit")
	require.NotContains(t, string(encoded), "connection refused")

	var storedReceipt opportunityReceipt
	require.NoError(t, o.db.Where("request_id=?", "url-sanitize").First(&storedReceipt).Error)
	require.NotContains(t, storedReceipt.Body, upstreamDetail)
	require.NotContains(t, storedReceipt.Body, "10.42.0.7")

	// Handler surfaces the bounded classification without upstream detail.
	h := &Handler{office: o, members: &memberListStub{members: []*types.TenantMember{{UserID: "owner", TenantID: 81, Role: types.TenantRoleOwner}}}}
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	base := context.WithValue(context.Background(), types.UserIDContextKey, "owner")
	base = context.WithValue(base, types.TenantIDContextKey, uint64(81))
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest("POST", "/api/v1/career/opportunities/import-url", strings.NewReader(`{"requestId":"url-http","url":"https://jobs.example.com/posting/leaky"}`)).WithContext(base)
	h.ImportURL(c)
	require.Equal(t, 200, rec.Code, rec.Body.String())
	require.NotContains(t, rec.Body.String(), upstreamDetail)
	require.Contains(t, rec.Body.String(), `"sourceStatus":"`+SourceStatusFetchFailed+`"`)
	require.Contains(t, rec.Body.String(), `"failureCode":"`+FailureNetworkError+`"`)

	var httpResult ImportURLResult
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &httpResult))
	listRec := httptest.NewRecorder()
	listCtx, _ := gin.CreateTestContext(listRec)
	listCtx.Request = httptest.NewRequest("GET", "/api/v1/career/opportunities/"+httpResult.OpportunityID+"/observations", nil).WithContext(base)
	listCtx.Params = gin.Params{{Key: "opportunityId", Value: httpResult.OpportunityID}}
	h.OpportunityObservations(listCtx)
	require.Equal(t, 200, listRec.Code, listRec.Body.String())
	require.Contains(t, listRec.Body.String(), `"sourceStatus":"`+SourceStatusFetchFailed+`"`)

	// Scope isolation: another user or tenant can neither import nor list.
	otherUser := WithScope(context.Background(), Scope{UserID: "intruder", TenantID: 81})
	_, err = o.ImportURL(otherUser, ImportURLInput{RequestID: "url-intruder", URL: rawURL})
	require.ErrorIs(t, err, ErrUnauthorized)
	otherTenant := WithScope(context.Background(), Scope{UserID: "owner", TenantID: 85})
	_, err = o.ImportURL(otherTenant, ImportURLInput{RequestID: "url-intruder", URL: rawURL})
	require.ErrorIs(t, err, ErrUnauthorized)
	_, err = o.OpportunityObservations(otherUser, httpResult.OpportunityID)
	require.ErrorIs(t, err, ErrUnauthorized)
	_, err = o.OpportunityObservations(ctx, "not-an-opportunity")
	require.ErrorIs(t, err, ErrOpportunityNotFound)

	intruderRec := httptest.NewRecorder()
	intruderCtx, _ := gin.CreateTestContext(intruderRec)
	intruderBase := context.WithValue(context.Background(), types.UserIDContextKey, "intruder")
	intruderBase = context.WithValue(intruderBase, types.TenantIDContextKey, uint64(81))
	intruderCtx.Request = httptest.NewRequest("GET", "/api/v1/career/opportunities/"+httpResult.OpportunityID+"/observations", nil).WithContext(intruderBase)
	intruderCtx.Params = gin.Params{{Key: "opportunityId", Value: httpResult.OpportunityID}}
	h.OpportunityObservations(intruderCtx)
	require.Equal(t, 403, intruderRec.Code)
}

// ---- helpers ------------------------------------------------------------------

func requireFetchFailureCode(t *testing.T, err error, code string) {
	t.Helper()
	require.Error(t, err)
	var fetchErr *SourceFetchError
	require.ErrorAs(t, err, &fetchErr)
	require.Equal(t, code, fetchErr.Code)
	require.NotContains(t, err.Error(), "10.99.0.5", "SSRF rejection must not leak private redirect details")
}

func mustApprovedSource(t *testing.T, policy SourcePolicy, rawURL string) ApprovedSource {
	t.Helper()
	source, err := policy.Verify(rawURL)
	require.NoError(t, err)
	return source
}

func portOf(t *testing.T, server *httptest.Server) string {
	t.Helper()
	addr, ok := server.Listener.Addr().(*net.TCPAddr)
	require.True(t, ok)
	return fmt.Sprintf("%d", addr.Port)
}

// ---- 9. frozen failure classifications (T09 Wave1 debt repayment) -----------

// TestImportURLFrozenFailureClassifications covers the five frozen failure
// classifications that stayed unasserted after T09: not_found, blocked,
// unsupported_content, empty_content, and response_too_large. Each case runs
// through the public ImportURL seam so the durable observation, snapshot, and
// receipt all carry the bounded classification.
func TestImportURLFrozenFailureClassifications(t *testing.T) {
	cases := []struct {
		name         string
		code         string
		wantStatus   string
		wantComplete string
	}{
		{name: "not_found", code: FailureNotFound, wantStatus: SourceStatusNotFound, wantComplete: CompletenessUnknown},
		{name: "blocked", code: FailureAccessBlocked, wantStatus: SourceStatusBlocked, wantComplete: CompletenessUnknown},
		{name: "unsupported_content", code: FailureUnsupportedContent, wantStatus: SourceStatusFetchFailed, wantComplete: CompletenessUnknown},
		{name: "empty_content", code: FailureEmptyContent, wantStatus: SourceStatusFetchFailed, wantComplete: CompletenessUnknown},
		{name: "response_too_large", code: FailureResponseTooLarge, wantStatus: SourceStatusFetchFailed, wantComplete: CompletenessUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rawURL := "https://jobs.example.com/posting/frozen-" + tc.name
			o, _, ctx := approvedSourceOffice(t, map[string]scriptedFetch{
				rawURL: {err: &SourceFetchError{Code: tc.code, Err: errors.New("scripted " + tc.name)}},
			})
			result, err := o.ImportURL(ctx, ImportURLInput{RequestID: "url-frozen-" + tc.name, URL: rawURL})
			require.NoError(t, err)
			require.Equal(t, tc.wantStatus, result.SourceStatus)
			require.Equal(t, tc.code, result.FailureCode)
			require.Equal(t, tc.wantComplete, result.Completeness)
			require.True(t, result.NeedsUserJD)
			// The classification must also be durable: the observation row and
			// the explicitly empty snapshot keep the bounded code as evidence.
			observations, err := o.OpportunityObservations(ctx, result.OpportunityID)
			require.NoError(t, err)
			require.Len(t, observations, 1)
			require.Equal(t, tc.wantStatus, observations[0].SourceStatus)
			require.Equal(t, tc.code, observations[0].FailureCode)
			evidence, err := o.OpportunityEvidence(ctx, result.OpportunityID, result.SnapshotID)
			require.NoError(t, err)
			require.Empty(t, evidence.RawText)
			require.Equal(t, OpportunityNeedsReview, evidence.Status)
			// Replay returns the identical durable classification.
			replay, err := o.ImportURL(ctx, ImportURLInput{RequestID: "url-frozen-" + tc.name, URL: rawURL})
			require.NoError(t, err)
			require.Equal(t, result, replay)
		})
	}
}

// transportTestDialer builds the injectable dial/resolver pair used to exercise
// the production transport against local httptest servers without weakening
// the SSRF guard: DNS always "resolves" to a public IP and every dial is routed
// to the mapped local listener by port.
func transportTestDialer(t *testing.T, hostTargets map[string]string) transportDialOptions {
	t.Helper()
	publicIP := net.ParseIP("93.184.216.34")
	lookupIPs := func(context.Context, string) ([]net.IP, error) { return []net.IP{publicIP}, nil }
	dialContext := func(ctx context.Context, network, address string) (net.Conn, error) {
		_, port, err := net.SplitHostPort(address)
		require.NoError(t, err)
		for _, target := range hostTargets {
			if strings.HasSuffix(target, ":"+port) {
				var d net.Dialer
				return d.DialContext(ctx, network, target)
			}
		}
		return nil, fmt.Errorf("no test listener for %s", port)
	}
	return transportDialOptions{LookupIPs: lookupIPs, DialContext: dialContext}
}

// TestImportURLExpiredClaimTakeoverGuardsTerminalReceipt pins the takeover
// contract for an expired claim: the new owner wins through the body-CAS
// UPDATE (RowsAffected==1 with a fresh token and lease), and the previous
// owner's commit under the old token is rejected as a lost claim — the
// terminal receipt can never be overwritten by a takeover.
func TestImportURLExpiredClaimTakeoverGuardsTerminalReceipt(t *testing.T) {
	o, ctx := newSourceImportOffice(t, "owner", 81)
	s := Scope{UserID: "owner", TenantID: 81}
	requestID := "url-cas-1"
	rawURL := "https://jobs.example.com/posting/cas"

	fingerprintInput, err := json.Marshal([]any{"import_url", requestID, rawURL})
	require.NoError(t, err)
	fingerprintSum := sha256.Sum256(fingerprintInput)
	fingerprint := hex.EncodeToString(fingerprintSum[:])

	expired := time.Now().UTC().Add(-2 * time.Minute)
	expiredBody, err := json.Marshal(importURLClaimBody{Kind: sourceImportClaimKind, ClaimToken: "old-token", LeaseUntil: expired})
	require.NoError(t, err)
	require.NoError(t, o.db.Create(&opportunityReceipt{
		TenantID: s.TenantID, UserID: s.UserID, RequestID: requestID,
		Fingerprint: fingerprint, Body: string(expiredBody), CreatedAt: expired,
	}).Error)

	// The takeover must go through the CAS UPDATE: it only lands when the
	// read body is still the one being replaced.
	outcome, err := o.claimImportURLRequest(ctx, s, requestID, fingerprint)
	require.NoError(t, err)
	require.Equal(t, importClaimProceed, outcome.state)
	require.NotEmpty(t, outcome.token)
	require.NotEqual(t, "old-token", outcome.token)

	var row opportunityReceipt
	require.NoError(t, o.db.Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, requestID).First(&row).Error)
	var claim importURLClaimBody
	require.NoError(t, json.Unmarshal([]byte(row.Body), &claim))
	require.Equal(t, outcome.token, claim.ClaimToken)
	require.True(t, claim.LeaseUntil.After(time.Now().UTC()), "the takeover installs a fresh lease")

	// The expired previous owner can no longer commit: its token is gone, so
	// the observation commit refuses (reconciling to an unknown outcome) and
	// never touches the live claim.
	_, err = o.commitImportURLObservation(ctx, s, requestID, fingerprint, "old-token",
		urlSourceEvidence{sourceStatus: SourceStatusComplete, completeness: CompletenessComplete, text: "text"}, rawURL)
	var unknown *OutcomeUnknownError
	require.ErrorAs(t, err, &unknown, "a lost claim reconciles to an unknown outcome, never a second observation")
	require.NoError(t, json.Unmarshal([]byte(row.Body), &claim))
	require.Equal(t, outcome.token, claim.ClaimToken, "the failed commit leaves the live claim untouched")

	// Nothing was duplicated while the claim was lost.
	var opportunities int64
	require.NoError(t, o.db.Model(&opportunity{}).Where("tenant_id=? AND user_id=?", s.TenantID, s.UserID).Count(&opportunities).Error)
	require.Zerof(t, opportunities, "a lost claim must not create a second opportunity")
}
