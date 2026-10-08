package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/golang-migrate/migrate/v4"
	sqlite3migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// craftGatewayBudget is a TEST budget port with real admission semantics
// (grants, call counting, denial injection). It is NOT an AllowAll stub and
// it lives only in _test.go: production charging assemblies must wire the
// durable service.CraftBudgetService, never a fake.
type craftGatewayBudget struct {
	mu                  sync.Mutex
	grants              map[string]craft.BudgetGrant
	byRun               map[string]string
	calls               map[string]int // grantID -> distinct authorized callIDs
	callIDs             map[string]bool
	starts              map[string]bool // durable test intent by grant/activity key
	startOutcomes       map[string]service.CraftChargeStartOutcome
	callbackAfterIntent bool
	initiationTimeout   time.Duration
	resolveErr          error
	denyErr             error
	revoked             map[string]bool
}

func newCraftGatewayBudget() *craftGatewayBudget {
	return &craftGatewayBudget{
		grants:        map[string]craft.BudgetGrant{},
		byRun:         map[string]string{},
		calls:         map[string]int{},
		callIDs:       map[string]bool{},
		starts:        map[string]bool{},
		startOutcomes: map[string]service.CraftChargeStartOutcome{},
		revoked:       map[string]bool{},
	}
}

func (b *craftGatewayBudget) Admit(_ context.Context, scope craft.Scope, runID string) (craft.BudgetGrant, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if scope.TenantID == 0 || runID == "" {
		return craft.BudgetGrant{}, fmt.Errorf("%w: admission needs scope and run", craft.ErrInvalidInput)
	}
	key := fmt.Sprintf("%d|%s", scope.TenantID, runID)
	if id, ok := b.byRun[key]; ok {
		return b.grants[id], nil
	}
	id := fmt.Sprintf("grant_%d_%s", scope.TenantID, runID)
	grant := craft.BudgetGrant{
		ID: id, Deadline: time.Now().UTC().Add(time.Hour),
		MaxCalls: 5, Allowed: !b.revoked[id],
	}
	b.grants[id] = grant
	b.byRun[key] = id
	return grant, nil
}

func (b *craftGatewayBudget) authorize(grantID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	grant, ok := b.grants[grantID]
	if !ok {
		return fmt.Errorf("%w: unknown grant", craft.ErrNotFound)
	}
	if !grant.Allowed {
		return fmt.Errorf("%w: grant cancelled", craft.ErrGrantRevoked)
	}
	if !time.Now().UTC().Before(grant.Deadline) {
		return fmt.Errorf("%w: deadline passed", craft.ErrGrantExpired)
	}
	if b.calls[grantID] >= grant.MaxCalls {
		return fmt.Errorf("%w: cap reached", craft.ErrGrantExhausted)
	}
	if b.denyErr != nil {
		return b.denyErr
	}
	return nil
}

func (b *craftGatewayBudget) count(grantID, callID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.callIDs[grantID+"/"+callID] {
		b.callIDs[grantID+"/"+callID] = true
		b.calls[grantID]++
	}
}

func (b *craftGatewayBudget) authorizeCount(grantID string) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.calls[grantID]
}

func (b *craftGatewayBudget) outcome(key string) service.CraftChargeStartOutcome {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.startOutcomes[key]
}

func (b *craftGatewayBudget) AuthorizeCall(_ context.Context, grantID, callID string) error {
	if err := b.authorize(grantID); err != nil {
		return err
	}
	b.count(grantID, callID)
	return nil
}

func (b *craftGatewayBudget) BeginBinding(ctx context.Context, grantID, activityID string, _ service.CraftCallBinding) (service.CraftChargeStartAttempt, error) {
	key := grantID + "/" + activityID
	b.mu.Lock()
	if b.starts[key] {
		b.mu.Unlock()
		return nil, craft.ErrConflict
	}
	b.mu.Unlock()
	if err := b.authorize(grantID); err != nil {
		return nil, err
	}
	b.mu.Lock()
	if b.starts[key] {
		b.mu.Unlock()
		return nil, craft.ErrConflict
	}
	b.starts[key] = true // fake committed intent/hold before callback
	b.callIDs[key] = true
	b.calls[grantID]++
	b.callbackAfterIntent = b.starts[key]
	b.mu.Unlock()
	timeout := b.initiationTimeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	startCtx, cancel := context.WithTimeout(ctx, timeout)
	return &fakeGatewayChargeAttempt{owner: b, key: key, ctx: startCtx, cancel: cancel}, nil
}

func (b *craftGatewayBudget) StartBinding(ctx context.Context, grantID, activityID string, binding service.CraftCallBinding,
	start func(context.Context) (service.CraftChargeStartOutcome, error),
) (service.CraftChargeStartOutcome, error) {
	attempt, err := b.BeginBinding(ctx, grantID, activityID, binding)
	if err != nil {
		return service.CraftChargeStartDefinitelyNotStarted, err
	}
	defer attempt.CancelInitiation()
	outcome, startErr := start(attempt.InitiationContext())
	if attempt.InitiationContext().Err() != nil && outcome != service.CraftChargeStartDefinitelyNotStarted {
		outcome = service.CraftChargeStartUnknown
		if startErr == nil {
			startErr = attempt.InitiationContext().Err()
		}
	}
	if startErr != nil && outcome != service.CraftChargeStartDefinitelyNotStarted {
		outcome = service.CraftChargeStartUnknown
	}
	if err := attempt.Resolve(ctx, outcome); err != nil {
		return service.CraftChargeStartUnknown, err
	}
	return outcome, startErr
}

type fakeGatewayChargeAttempt struct {
	owner  *craftGatewayBudget
	key    string
	ctx    context.Context
	cancel context.CancelFunc
}

func (a *fakeGatewayChargeAttempt) InitiationContext() context.Context { return a.ctx }
func (a *fakeGatewayChargeAttempt) CancelInitiation()                  { a.cancel() }
func (a *fakeGatewayChargeAttempt) Resolve(_ context.Context, outcome service.CraftChargeStartOutcome) error {
	a.owner.mu.Lock()
	defer a.owner.mu.Unlock()
	if a.owner.resolveErr != nil {
		return a.owner.resolveErr
	}
	if a.owner.startOutcomes[a.key] != 0 {
		return craft.ErrConflict
	}
	a.owner.startOutcomes[a.key] = outcome
	return nil
}

func (b *craftGatewayBudget) Reconcile(_ context.Context, grantID string) error { return nil }

func (b *craftGatewayBudget) RevokeGrant(_ context.Context, grantID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if g, ok := b.grants[grantID]; ok {
		g.Allowed = false
		b.grants[grantID] = g
	}
	b.revoked[grantID] = true
	return nil
}

func (b *craftGatewayBudget) deny(err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.denyErr = err
}

// craftGatewayRecorder captures the physical calls the gateway records.
type craftGatewayRecorder struct {
	mu    sync.Mutex
	calls []service.PhysicalCall
}

func (r *craftGatewayRecorder) RecordPhysicalCall(_ context.Context, in service.PhysicalCall) (craft.UsageFact, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, in)
	return craft.UsageFact{ID: "fact", CallID: in.CallID, AttemptID: in.AttemptID}, nil
}

func (r *craftGatewayRecorder) recorded() []service.PhysicalCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]service.PhysicalCall{}, r.calls...)
}

// craftUpstreamAuth captures the Authorization header the real upstream saw.
type craftUpstreamAuth struct {
	mu    sync.Mutex
	v     string
	calls int
}

func (a *craftUpstreamAuth) Store(s string) { a.mu.Lock(); a.v = s; a.calls++; a.mu.Unlock() }
func (a *craftUpstreamAuth) Load() string   { a.mu.Lock(); defer a.mu.Unlock(); return a.v }
func (a *craftUpstreamAuth) Count() int     { a.mu.Lock(); defer a.mu.Unlock(); return a.calls }

// craftGatewayTestEnv assembles the gateway over fakes plus a REAL upstream
// httptest server that asserts the server-side credential was used.
type craftGatewayTestEnv struct {
	gw       *CraftModelGateway
	budget   *craftGatewayBudget
	recorder *craftGatewayRecorder
	upstream *httptest.Server
	auth     craftUpstreamAuth
	router   *gin.Engine
	now      time.Time
}

func newCraftGatewayTestEnv(t *testing.T) *craftGatewayTestEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	env := &craftGatewayTestEnv{budget: newCraftGatewayBudget(), recorder: &craftGatewayRecorder{}}
	env.upstream = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		env.auth.Store(r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl-1","choices":[{"message":{"role":"assistant","content":"ok"}}],"usage":{"prompt_tokens":100,"completion_tokens":40,"prompt_tokens_details":{"cached_tokens":10}}}`))
	}))
	t.Cleanup(env.upstream.Close)

	env.now = time.Now().UTC()
	gw, err := NewCraftModelGateway(CraftModelGatewayConfig{
		Secret:   []byte("test-signing-secret-16b"),
		Budget:   env.budget,
		Starter:  env.budget,
		Recorder: env.recorder,
		Upstream: func(_ context.Context, _ string) (CraftUpstreamTarget, error) {
			return CraftUpstreamTarget{BaseURL: env.upstream.URL, Path: "/v1/chat/completions", APIKey: "sk-platform-managed"}, nil
		},
		GatewayBaseURL: "http://gateway.internal",
		Now:            func() time.Time { return env.now },
	})
	require.NoError(t, err)
	env.gw = gw

	r := gin.New()
	r.POST("/craft/model-gateway/credentials", func(c *gin.Context) {
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), types.TenantIDContextKey, uint64(7)))
		gw.IssueCredential(c)
	})
	r.POST("/craft/model-gateway/credentials/revoke", gw.RevokeCredential)
	r.POST("/craft/model-gateway/v1/chat/completions", gw.Forward)
	r.GET("/craft/model-gateway/v1/models", gw.ListModels)
	env.router = r
	return env
}

func (e *craftGatewayTestEnv) upstreamAuth() string { return e.auth.Load() }

// issueCredentialOn issues a credential through the real endpoint.
func issueCredentialOn(t *testing.T, env *craftGatewayTestEnv, body string) (string, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/craft/model-gateway/credentials", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var out struct {
		Success bool           `json:"success"`
		Data    map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	require.True(t, out.Success)
	return out.Data["credential"].(string), out.Data
}

func forwardOn(t *testing.T, env *craftGatewayTestEnv, credential, body string) *httptest.ResponseRecorder {
	return forwardWithActivityOn(t, env, credential, body, "activity-test-1")
}

func forwardWithActivityOn(t *testing.T, env *craftGatewayTestEnv, credential, body, activityID string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/craft/model-gateway/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+credential)
	if activityID != "" {
		req.Header.Set("X-Craft-Activity-ID", activityID)
	}
	w := httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	return w
}

func TestCraftGatewayRequiresStableActivityIdentityBeforeAnyUpstreamWork(t *testing.T) {
	env := newCraftGatewayTestEnv(t)
	credential, _ := issueCredentialOn(t, env, craftGatewayIssueBody)

	w := forwardWithActivityOn(t, env, credential, `{"model":"m1","messages":[]}`, "")
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "ACTIVITY_ID_REQUIRED")
	require.Zero(t, env.auth.Count())
	require.Zero(t, env.budget.authorizeCount("grant_7_run-1"))
}

func TestCraftGatewaySameActivityKeyCannotStartTwice(t *testing.T) {
	env := newCraftGatewayTestEnv(t)
	credential, _ := issueCredentialOn(t, env, craftGatewayIssueBody)

	first := forwardWithActivityOn(t, env, credential, `{"model":"m1","messages":[{"role":"user","content":"hi"}]}`, "activity-stable-1")
	require.Equal(t, http.StatusOK, first.Code, first.Body.String())
	second := forwardWithActivityOn(t, env, credential, `{"model":"m2","messages":[{"role":"user","content":"hi"}]}`, "activity-stable-1")
	require.Equal(t, http.StatusConflict, second.Code, second.Body.String())
	require.Contains(t, second.Body.String(), "ACTIVITY_UNRESOLVED")
	require.Equal(t, 1, env.auth.Count(), "a replay of one activity cannot reach the upstream twice")
}

func TestCraftGatewayPostSendAndBodyReadFailuresStayNonReplayable(t *testing.T) {
	t.Run("transport outcome unknown", func(t *testing.T) {
		env := newCraftGatewayTestEnv(t)
		credential, _ := issueCredentialOn(t, env, craftGatewayIssueBody)
		forwardCalls := 0
		env.gw.forward = func(*http.Request) (*http.Response, error) {
			forwardCalls++
			return nil, errors.New("connection lost after possible send")
		}

		first := forwardWithActivityOn(t, env, credential, `{"model":"m1","messages":[]}`, "activity-unknown-1")
		require.Equal(t, http.StatusBadGateway, first.Code, first.Body.String())
		second := forwardWithActivityOn(t, env, credential, `{"model":"m1","messages":[]}`, "activity-unknown-1")
		require.Equal(t, http.StatusConflict, second.Code, second.Body.String())
		require.Equal(t, 1, forwardCalls, "an uncertain send must never be initiated twice")
		recorded := env.recorder.recorded()
		require.Len(t, recorded, 1)
		require.Nil(t, recorded[0].Usage, "an uncertain response records unknown usage")
	})

	t.Run("body read failure", func(t *testing.T) {
		env := newCraftGatewayTestEnv(t)
		credential, _ := issueCredentialOn(t, env, craftGatewayIssueBody)
		forwardCalls := 0
		env.gw.forward = func(*http.Request) (*http.Response, error) {
			forwardCalls++
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       failingGatewayBody{},
			}, nil
		}

		first := forwardWithActivityOn(t, env, credential, `{"model":"m1","messages":[]}`, "activity-body-1")
		require.Equal(t, http.StatusBadGateway, first.Code, first.Body.String())
		second := forwardWithActivityOn(t, env, credential, `{"model":"m1","messages":[]}`, "activity-body-1")
		require.Equal(t, http.StatusConflict, second.Code, second.Body.String())
		require.Equal(t, 1, forwardCalls, "body-read failure cannot reopen the committed activity")
		recorded := env.recorder.recorded()
		require.Len(t, recorded, 1)
		require.Nil(t, recorded[0].Usage, "an unreadable body records nil usage (unknown observation)")
		// HEAD semantics: the fake forward returned response HEADERS, so the
		// physical call factually started — the durable outcome is the
		// deterministic Started. The activity stays closed against replay
		// exactly as before (the 409 above proves it).
		require.Equal(t, service.CraftChargeStartStarted, env.budget.outcome(fakeGatewayActivityKey(t, "activity-body-1")), "headers-returned body failure is definitively started")
	})

	t.Run("response returned with transport error is closed", func(t *testing.T) {
		env := newCraftGatewayTestEnv(t)
		credential, _ := issueCredentialOn(t, env, craftGatewayIssueBody)
		body := &trackingGatewayBody{}
		env.gw.forward = func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: body}, errors.New("transport failed after response")
		}
		w := forwardWithActivityOn(t, env, credential, `{"model":"m1","messages":[]}`, "activity-response-error")
		require.Equal(t, http.StatusBadGateway, w.Code, w.Body.String())
		require.True(t, body.closed)
		recorded := env.recorder.recorded()
		require.Len(t, recorded, 1)
		require.Nil(t, recorded[0].Usage)
		require.Equal(t, service.CraftChargeStartUnknown, env.budget.outcome(fakeGatewayActivityKey(t, "activity-response-error")))
	})

	t.Run("headers-first delayed body survives initiation deadline", func(t *testing.T) {
		env := newCraftGatewayTestEnv(t)
		credential, _ := issueCredentialOn(t, env, craftGatewayIssueBody)
		releaseBody := make(chan struct{})
		headersFlushed := make(chan struct{})
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.(http.Flusher).Flush()
			close(headersFlushed)
			select {
			case <-releaseBody:
				_, _ = io.WriteString(w, `{"usage":{"prompt_tokens":2,"completion_tokens":3}}`)
			case <-r.Context().Done():
			}
		}))
		defer server.Close()
		env.gw.forward = http.DefaultClient.Do
		env.gw.timeout = time.Second
		env.budget.initiationTimeout = 60 * time.Millisecond
		env.gw.upstream = func(context.Context, string) (CraftUpstreamTarget, error) {
			return CraftUpstreamTarget{BaseURL: server.URL, Path: "/v1/chat/completions", APIKey: "sk-platform-managed"}, nil
		}
		result := make(chan *httptest.ResponseRecorder, 1)
		go func() {
			result <- forwardWithActivityOn(t, env, credential, `{"model":"m1","messages":[]}`, "activity-delayed-body")
		}()
		<-headersFlushed                   // Client.Do returned after response headers, body is pending.
		time.Sleep(100 * time.Millisecond) // exceed the coordinator's initiation deadline
		require.Equal(t, service.CraftChargeStartOutcome(0), env.budget.outcome(fakeGatewayActivityKey(t, "activity-delayed-body")), "headers alone cannot resolve the durable start")
		select {
		case w := <-result:
			require.FailNow(t, "gateway returned before delayed upstream body was released", "status %d: %s", w.Code, w.Body.String())
		default:
		}
		close(releaseBody)
		w := <-result
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		require.Contains(t, w.Body.String(), `"completion_tokens":3`)
		recorded := env.recorder.recorded()
		require.Len(t, recorded, 1)
		require.NotNil(t, recorded[0].Usage)
		require.Equal(t, service.CraftChargeStartStarted, env.budget.outcome(fakeGatewayActivityKey(t, "activity-delayed-body")))
	})

	t.Run("blocked Do is canceled by initiation deadline", func(t *testing.T) {
		env := newCraftGatewayTestEnv(t)
		credential, _ := issueCredentialOn(t, env, craftGatewayIssueBody)
		requestStarted := make(chan struct{})
		allowHandlerExit := make(chan struct{})
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			close(requestStarted)
			<-allowHandlerExit // do not send response headers until cleanup
		}))
		defer func() {
			close(allowHandlerExit)
			server.Close()
		}()
		env.gw.forward = http.DefaultClient.Do
		env.gw.timeout = time.Second
		env.budget.initiationTimeout = 60 * time.Millisecond
		env.gw.upstream = func(context.Context, string) (CraftUpstreamTarget, error) {
			return CraftUpstreamTarget{BaseURL: server.URL, Path: "/v1/chat/completions", APIKey: "sk-platform-managed"}, nil
		}
		started := time.Now()
		w := forwardWithActivityOn(t, env, credential, `{"model":"m1","messages":[]}`, "activity-blocked-do")
		require.Less(t, time.Since(started), 500*time.Millisecond, "blocked Do must observe the shorter start deadline, not whole-response timeout")
		require.Equal(t, http.StatusBadGateway, w.Code, w.Body.String())
		<-requestStarted
		require.Equal(t, service.CraftChargeStartUnknown, env.budget.outcome(fakeGatewayActivityKey(t, "activity-blocked-do")))
	})

	for _, tc := range []struct {
		name string
		body func() io.ReadCloser
	}{
		{name: "nil body", body: func() io.ReadCloser { return nil }},
		{name: "empty body", body: func() io.ReadCloser { return http.NoBody }},
		{name: "oversize body", body: func() io.ReadCloser {
			return io.NopCloser(strings.NewReader(strings.Repeat("x", craftMaxForwardBody+1)))
		}},
		{name: "close failure", body: func() io.ReadCloser {
			return &closeErrorGatewayBody{Reader: strings.NewReader(`{"usage":{}}`), err: errors.New("close failed")}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := newCraftGatewayTestEnv(t)
			credential, _ := issueCredentialOn(t, env, craftGatewayIssueBody)
			body := tc.body()
			env.gw.forward = func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: body}, nil
			}
			activity := "activity-" + strings.ReplaceAll(tc.name, " ", "-")
			w := forwardWithActivityOn(t, env, credential, `{"model":"m1","messages":[]}`, activity)
			require.Equal(t, http.StatusBadGateway, w.Code, w.Body.String())
			// HEAD semantics ([T14] 7e4be7a93/ba78f06c1): the response
			// HEADERS returned, so the physical call factually started — the
			// durable outcome is Started (deterministic), never Unknown; an
			// Unknown here would park the adapter id and exclude the Run from
			// lease recovery. Replay protection is unchanged: the fake
			// coordinator still refuses a second BeginBinding on the same
			// activity key.
			require.Equal(t, service.CraftChargeStartStarted, env.budget.outcome(fakeGatewayActivityKey(t, activity)))
			recorded := env.recorder.recorded()
			require.Len(t, recorded, 1)
			// Usage fidelity: whatever bytes survived the read failure still
			// feed the billing basis. A fully unreadable body records nil
			// usage (unknown observation); the close-failure fixture read the
			// whole `{"usage":{}}` body first, so its (empty) totals persist.
			if tc.name == "close failure" {
				require.NotNil(t, recorded[0].Usage)
			} else {
				require.Nil(t, recorded[0].Usage)
			}
		})
	}

	t.Run("resolver failure is not reported as success", func(t *testing.T) {
		env := newCraftGatewayTestEnv(t)
		credential, _ := issueCredentialOn(t, env, craftGatewayIssueBody)
		env.budget.resolveErr = errors.New("journal unavailable")
		env.gw.forward = func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"usage":{"prompt_tokens":1}}`))}, nil
		}
		w := forwardWithActivityOn(t, env, credential, `{"model":"m1","messages":[]}`, "activity-resolver-failure")
		require.Equal(t, http.StatusBadGateway, w.Code, w.Body.String())
		require.Equal(t, service.CraftChargeStartOutcome(0), env.budget.outcome(fakeGatewayActivityKey(t, "activity-resolver-failure")), "failed CAS leaves the fake intent unresolved")
		recorded := env.recorder.recorded()
		require.Len(t, recorded, 1)
		// Usage fidelity on a failed Started resolution: the fully observed
		// body still feeds the usage fact so manual reconciliation keeps a
		// fixed billing basis (HEAD [T14] semantics — recorded BEFORE the
		// ACTIVITY_UNRESOLVED answer below).
		require.NotNil(t, recorded[0].Usage)
		require.Equal(t, int64(1), recorded[0].Usage.Input)
	})

	t.Run("inbound cancellation interrupts response body", func(t *testing.T) {
		env := newCraftGatewayTestEnv(t)
		credential, _ := issueCredentialOn(t, env, craftGatewayIssueBody)
		usage := newHandlerCraftUsageService(t)
		env.gw.recorder = usage
		headersSent := make(chan struct{})
		allowHandlerExit := make(chan struct{})
		var requestMu sync.Mutex
		requestCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestMu.Lock()
			requestCount++
			requestMu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.(http.Flusher).Flush()
			close(headersSent)
			select {
			case <-allowHandlerExit:
			case <-r.Context().Done():
			}
		}))
		defer func() {
			close(allowHandlerExit)
			server.Close()
		}()
		env.gw.forward = http.DefaultClient.Do
		env.gw.upstream = func(context.Context, string) (CraftUpstreamTarget, error) {
			return CraftUpstreamTarget{BaseURL: server.URL, Path: "/v1/chat/completions", APIKey: "sk-platform-managed"}, nil
		}
		ctx, cancel := context.WithCancel(context.Background())
		result := make(chan *httptest.ResponseRecorder, 1)
		go func() {
			req := httptest.NewRequest(http.MethodPost, "/craft/model-gateway/v1/chat/completions", strings.NewReader(`{"model":"m1","messages":[]}`)).WithContext(ctx)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+credential)
			req.Header.Set(craftModelActivityHeader, "activity-inbound-cancel")
			w := httptest.NewRecorder()
			env.router.ServeHTTP(w, req)
			result <- w
		}()
		<-headersSent
		cancel()
		select {
		case w := <-result:
			require.Equal(t, http.StatusBadGateway, w.Code, w.Body.String())
		case <-time.After(time.Second):
			t.Fatal("inbound cancellation did not interrupt body read")
		}
		require.Equal(t, service.CraftChargeStartUnknown, env.budget.outcome(fakeGatewayActivityKey(t, "activity-inbound-cancel")))
		facts, err := usage.Facts(context.Background(), 7, "run-1")
		require.NoError(t, err)
		require.Len(t, facts, 1, "cancellation after provider acceptance is durably recorded")
		require.Equal(t, craft.UsageStatusUnknown, facts[0].Status)
		require.Zero(t, facts[0].Input)
		require.Zero(t, facts[0].Output)
		require.Zero(t, facts[0].Cached)
		retry := forwardWithActivityOn(t, env, credential, `{"model":"m1","messages":[]}`, "activity-inbound-cancel")
		require.Equal(t, http.StatusConflict, retry.Code, retry.Body.String())
		requestMu.Lock()
		require.Equal(t, 1, requestCount, "same activity cannot issue a second provider request")
		requestMu.Unlock()
	})
}

type closeErrorGatewayBody struct {
	io.Reader
	err error
}

func (b *closeErrorGatewayBody) Close() error { return b.err }

type failingGatewayRecorder struct{ err error }

func (r *failingGatewayRecorder) RecordPhysicalCall(context.Context, service.PhysicalCall) (craft.UsageFact, error) {
	return craft.UsageFact{}, r.err
}

// A usage-record failure must never leak the raw internal error string to the
// API client. The response header stays an opaque marker plus durable
// correlation identities; the detail itself goes only to the server log.
func TestCraftGatewayUsageRecordErrorHeaderIsOpaqueWithCorrelationIdentity(t *testing.T) {
	env := newCraftGatewayTestEnv(t)
	credential, _ := issueCredentialOn(t, env, craftGatewayIssueBody)
	env.gw.recorder = &failingGatewayRecorder{err: errors.New("sql: no such table craft_usage_facts in debian-prod-db-07")}
	w := forwardWithActivityOn(t, env, credential, `{"model":"m1","messages":[]}`, "activity-opaque-record-error")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.NotContains(t, w.Header().Get("X-Craft-Usage-Record-Error"), "no such table")
	require.NotContains(t, w.Header().Get("X-Craft-Usage-Record-Error"), "debian-prod-db-07")
	require.Equal(t, "usage_record_failed", w.Header().Get("X-Craft-Usage-Record-Error"))
	require.NotEmpty(t, w.Header().Get("X-Craft-Usage-Record-Run-ID"), "durable run identity travels with the marker")
	require.NotEmpty(t, w.Header().Get("X-Craft-Usage-Record-Attempt-ID"), "durable attempt identity travels with the marker")
	require.Contains(t, w.Header().Get("X-Craft-Usage-Record-Call-ID"), "activity/", "durable call identity travels with the marker")
}

func TestCraftGatewayUsageRecordFailureUsesBoundedDetachedContextAndLogsIdentity(t *testing.T) {
	env := newCraftGatewayTestEnv(t)
	credential, _ := issueCredentialOn(t, env, craftGatewayIssueBody)
	env.gw.usageRecordTimeout = 40 * time.Millisecond
	traceKey := struct{}{}
	logBuffer := &strings.Builder{}
	log := logrus.New()
	log.SetFormatter(&logrus.JSONFormatter{})
	log.SetOutput(logBuffer)
	ctx := context.WithValue(context.Background(), traceKey, "trace-preserved")
	ctx = context.WithValue(ctx, types.LoggerContextKey, logrus.NewEntry(log))
	ctx, cancelRequest := context.WithCancel(ctx)
	defer cancelRequest()
	recorder := &blockingFailureGatewayRecorder{traceKey: traceKey, expectedTrace: "trace-preserved"}
	env.gw.recorder = recorder
	forwardCalls := 0
	env.gw.forward = func(*http.Request) (*http.Response, error) {
		forwardCalls++
		return &http.Response{StatusCode: http.StatusOK, Body: cancelingGatewayBody{cancel: cancelRequest}}, nil
	}
	req := httptest.NewRequest(http.MethodPost, "/craft/model-gateway/v1/chat/completions", strings.NewReader(`{"model":"m1","messages":[]}`)).WithContext(ctx)
	req.Header.Set("Authorization", "Bearer "+credential)
	req.Header.Set(craftModelActivityHeader, "activity-record-failure")
	w := httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadGateway, w.Code, w.Body.String())
	// HEAD semantics: the fake forward returned response HEADERS, so the
	// physical call factually started — the durable outcome is the
	// deterministic Started, never Unknown (an Unknown would park the
	// adapter id and exclude the Run from lease recovery). The bounded
	// detached recordCall context is what this test actually proves below.
	require.Equal(t, service.CraftChargeStartStarted, env.budget.outcome(fakeGatewayActivityKey(t, "activity-record-failure")))
	require.True(t, recorder.sawDeadline, "append receives an explicit deadline")
	require.True(t, recorder.sawTrace, "WithoutCancel preserves trace values")
	require.True(t, recorder.sawExpiry, "bounded persistence ends when its deadline expires")
	// The read-failure branch and the usage-record failure each emit one
	// JSON log line; the assertions target the usage-record entry, which is
	// the one carrying the durable correlation identities.
	var entry map[string]any
	for _, line := range strings.Split(strings.TrimSpace(logBuffer.String()), "\n") {
		if line == "" {
			continue
		}
		var parsed map[string]any
		if err := json.Unmarshal([]byte(line), &parsed); err != nil {
			continue
		}
		if parsed["event"] == "craft_model_gateway_usage_record_failed" {
			entry = parsed
		}
	}
	require.NotNil(t, entry, "a usage-record failure must log its correlation identities")
	require.Equal(t, "run-1", entry["run_id"])
	require.NotEmpty(t, entry["call_id"])
	require.NotEmpty(t, entry["attempt_id"])
	require.Contains(t, entry["error"], "context deadline exceeded")
	require.NotContains(t, logBuffer.String(), "sk-platform-managed")
	require.NotContains(t, logBuffer.String(), `"messages"`)
	retry := forwardWithActivityOn(t, env, credential, `{"model":"m1","messages":[]}`, "activity-record-failure")
	require.Equal(t, http.StatusConflict, retry.Code, retry.Body.String())
	require.Equal(t, 1, forwardCalls, "unresolved physical activity is never sent twice")
}

type blockingFailureGatewayRecorder struct {
	traceKey      any
	expectedTrace any
	sawDeadline   bool
	sawTrace      bool
	sawExpiry     bool
}

type cancelingGatewayBody struct{ cancel context.CancelFunc }

func (b cancelingGatewayBody) Read([]byte) (int, error) {
	b.cancel()
	return 0, errors.New("response body lost after request cancellation")
}
func (cancelingGatewayBody) Close() error { return nil }

func (r *blockingFailureGatewayRecorder) RecordPhysicalCall(ctx context.Context, _ service.PhysicalCall) (craft.UsageFact, error) {
	_, r.sawDeadline = ctx.Deadline()
	r.sawTrace = ctx.Value(r.traceKey) == r.expectedTrace
	<-ctx.Done()
	r.sawExpiry = errors.Is(ctx.Err(), context.DeadlineExceeded)
	return craft.UsageFact{}, ctx.Err()
}

func newHandlerCraftUsageService(t *testing.T) *service.CraftUsageService {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "../.."))
	dsn := "file:" + filepath.Join(t.TempDir(), "craft-usage-handler.db") + "?_foreign_keys=on&_busy_timeout=5000"
	sqlDB, err := sql.Open("sqlite3", dsn)
	require.NoError(t, err)
	driver, err := sqlite3migrate.WithInstance(sqlDB, &sqlite3migrate.Config{NoTxWrap: true})
	require.NoError(t, err)
	migrator, err := migrate.NewWithDatabaseInstance("file:"+filepath.Join(repoRoot, "migrations/sqlite"), "sqlite3", driver)
	require.NoError(t, err)
	require.NoError(t, migrator.Up())
	_, _ = migrator.Close()
	_ = sqlDB.Close()
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	require.NoError(t, err)
	t.Cleanup(func() {
		conn, err := db.DB()
		if err == nil {
			_ = conn.Close()
		}
	})
	return service.NewCraftUsageService(repository.NewCraftUsageStore(db))
}

func fakeGatewayActivityKey(t *testing.T, requestID string) string {
	t.Helper()
	key, err := craftModelActivityKey(craftCredentialPayload{TenantID: 7, RunID: "run-1", GrantID: "grant_7_run-1"}, requestID)
	require.NoError(t, err)
	return "grant_7_run-1/" + key
}

type trackingGatewayBody struct{ closed bool }

func (*trackingGatewayBody) Read([]byte) (int, error) { return 0, io.EOF }
func (b *trackingGatewayBody) Close() error           { b.closed = true; return nil }

type failingGatewayBody struct{}

func (failingGatewayBody) Read([]byte) (int, error) { return 0, errors.New("response body lost") }
func (failingGatewayBody) Close() error             { return nil }

func TestCraftModelActivityKeyIsStableAndCredentialScoped(t *testing.T) {
	payload := craftCredentialPayload{JTI: "token-a", TenantID: 7, RunID: "run-1", GrantID: "grant-1", DelegationID: "del-1", Funding: "platform"}
	first, err := craftModelActivityKey(payload, "request-1")
	require.NoError(t, err)
	replay, err := craftModelActivityKey(payload, "request-1")
	require.NoError(t, err)
	require.Equal(t, first, replay)
	reissuedCredential := payload
	reissuedCredential.JTI = "token-b"
	reissued, err := craftModelActivityKey(reissuedCredential, "request-1")
	require.NoError(t, err)
	require.Equal(t, first, reissued, "credential reissue for the same durable grant preserves activity identity")
	otherRun := payload
	otherRun.RunID = "run-2"
	other, err := craftModelActivityKey(otherRun, "request-1")
	require.NoError(t, err)
	require.NotEqual(t, first, other)
	for _, invalid := range []string{"", " has-space", "bad/key", strings.Repeat("x", 129)} {
		if _, err := craftModelActivityKey(payload, invalid); err == nil {
			t.Fatalf("activity key accepted invalid request identity %q", invalid)
		}
	}
}

// issueCredentialOnBare issues a credential on a bare router with the tenant
// middleware attached (for gateways assembled outside the shared env).
func issueCredentialOnBare(t *testing.T, gw *CraftModelGateway) string {
	t.Helper()
	r := gin.New()
	r.POST("/craft/model-gateway/credentials", func(c *gin.Context) {
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), types.TenantIDContextKey, uint64(7)))
		gw.IssueCredential(c)
	})
	req := httptest.NewRequest(http.MethodPost, "/craft/model-gateway/credentials", strings.NewReader(craftGatewayIssueBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var out struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	return out.Data["credential"].(string)
}

const craftGatewayIssueBody = `{"run_id":"run-1","delegation_id":"del-1","runtime":"oc","models":["m1","m2"],"funding":"platform","ttl_seconds":600}`

func TestCraftGatewayIssuesCheckpointSafeCredentialBoundToGrant(t *testing.T) {
	env := newCraftGatewayTestEnv(t)
	credential, data := issueCredentialOn(t, env, craftGatewayIssueBody)

	require.NotEmpty(t, credential)
	require.True(t, strings.HasPrefix(credential, "cmg1."), "execution credential must carry its version prefix")
	grant := data["grant"].(map[string]any)
	require.Equal(t, true, grant["allowed"])
	require.Equal(t, float64(5), grant["max_calls"])

	// The checkpoint view is structurally secret-free: marshaling it must not
	// leak the credential token anywhere.
	checkpoint, err := json.Marshal(data["checkpoint"])
	require.NoError(t, err)
	require.NotContains(t, string(checkpoint), credential)
	require.Contains(t, string(checkpoint), "run-1")
	require.Contains(t, string(checkpoint), "del-1")
}

func TestCraftGatewayForwardAuthorizesRecordsAndUsesServerCredential(t *testing.T) {
	env := newCraftGatewayTestEnv(t)
	credential, _ := issueCredentialOn(t, env, craftGatewayIssueBody)

	w := forwardOn(t, env, credential, `{"model":"m1","messages":[{"role":"user","content":"hi"}]}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "chatcmpl-1")

	// The upstream saw ONLY the server-managed platform credential.
	require.Equal(t, "Bearer sk-platform-managed", env.upstreamAuth())
	// One authorized call, one recorded physical fact with server-derived identity.
	require.Equal(t, 1, env.budget.authorizeCount("grant_7_run-1"))
	require.True(t, env.budget.callbackAfterIntent, "transport callback follows committed activity intent and hold")
	recorded := env.recorder.recorded()
	require.Len(t, recorded, 1)
	fact := recorded[0]
	require.Equal(t, uint64(7), fact.TenantID)
	require.Equal(t, "run-1", fact.RunID)
	require.Equal(t, "del-1", fact.DelegationID)
	require.Equal(t, craft.RuntimeOC, fact.Runtime)
	require.Equal(t, "m1", fact.ModelID)
	require.Equal(t, "platform", fact.Funding)
	require.True(t, strings.HasPrefix(fact.CallID, "activity/"))
	require.True(t, strings.HasPrefix(fact.AttemptID, "att_"))
	require.NotNil(t, fact.Usage)
	require.Equal(t, int64(100), fact.Usage.Input)
	require.Equal(t, int64(40), fact.Usage.Output)
	require.Equal(t, int64(10), fact.Usage.Cached)
}

func TestCraftGatewayRejectsClientSuppliedUpstreamAndKeys(t *testing.T) {
	env := newCraftGatewayTestEnv(t)
	credential, _ := issueCredentialOn(t, env, craftGatewayIssueBody)

	for _, forbidden := range []string{
		`{"model":"m1","base_url":"https://evil.example/v1","messages":[]}`,
		`{"model":"m1","api_key":"sk-client-smuggled","messages":[]}`,
		`{"model":"m1","upstream":"https://evil.example","messages":[]}`,
	} {
		w := forwardOn(t, env, credential, forbidden)
		require.Equal(t, http.StatusForbidden, w.Code, forbidden)
		require.Contains(t, w.Body.String(), "CLIENT_UPSTREAM_FORBIDDEN")
	}
	require.Equal(t, "", env.upstreamAuth(), "no outbound call may happen for a smuggled upstream")
	require.Equal(t, 0, env.budget.authorizeCount("grant_7_run-1"))
}

func TestCraftGatewayRejectsModelOutsideCredentialScope(t *testing.T) {
	env := newCraftGatewayTestEnv(t)
	credential, _ := issueCredentialOn(t, env, craftGatewayIssueBody)

	w := forwardOn(t, env, credential, `{"model":"m-forbidden","messages":[]}`)
	require.Equal(t, http.StatusForbidden, w.Code)
	require.Contains(t, w.Body.String(), "MODEL_NOT_ALLOWED")
	require.Equal(t, "", env.upstreamAuth())
	require.Equal(t, 0, env.budget.authorizeCount("grant_7_run-1"))
}

func TestCraftGatewayRejectsExpiredRevokedAndTamperedCredentials(t *testing.T) {
	env := newCraftGatewayTestEnv(t)
	credential, _ := issueCredentialOn(t, env, craftGatewayIssueBody)

	// Expired: advance the gateway clock past the credential TTL.
	env.now = env.now.Add(30 * time.Minute)
	w := forwardOn(t, env, credential, `{"model":"m1","messages":[]}`)
	require.Equal(t, http.StatusUnauthorized, w.Code)
	require.Contains(t, w.Body.String(), "CREDENTIAL_EXPIRED")

	// Revoked: reissue, revoke via the endpoint, forward must refuse.
	env.now = env.now.Add(-29 * time.Minute)
	fresh, _ := issueCredentialOn(t, env, craftGatewayIssueBody)
	req := httptest.NewRequest(http.MethodPost, "/craft/model-gateway/credentials/revoke",
		strings.NewReader(fmt.Sprintf(`{"credential":%q}`, fresh)))
	w = httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	w = forwardOn(t, env, fresh, `{"model":"m1","messages":[]}`)
	require.Equal(t, http.StatusUnauthorized, w.Code)
	require.Contains(t, w.Body.String(), "CREDENTIAL_REVOKED")

	// Tampered: a signature that does not match the payload is refused.
	tampered := credential[:strings.LastIndex(credential, ".")] + "AAAA"
	w = forwardOn(t, env, tampered, `{"model":"m1"}`)
	require.Equal(t, http.StatusUnauthorized, w.Code)
	require.Contains(t, w.Body.String(), "CREDENTIAL_INVALID")
	require.Equal(t, "", env.upstreamAuth())
}

func TestCraftGatewayBudgetStopIsVisibleToMainAgent(t *testing.T) {
	env := newCraftGatewayTestEnv(t)
	credential, _ := issueCredentialOn(t, env, craftGatewayIssueBody)
	env.budget.deny(fmt.Errorf("%w: no funded headroom", craft.ErrBudgetDenied))

	w := forwardOn(t, env, credential, `{"model":"m1","messages":[]}`)
	require.Equal(t, http.StatusPaymentRequired, w.Code)
	require.Contains(t, w.Body.String(), "BUDGET_STOPPED")
	require.Contains(t, w.Body.String(), "no funded headroom", "the stop reason must be visible to the main agent")
	require.Equal(t, "", env.upstreamAuth(), "a denied call must never reach the upstream")
}

func TestCraftGatewayRevokedGrantStopsNewCalls(t *testing.T) {
	env := newCraftGatewayTestEnv(t)
	credential, _ := issueCredentialOn(t, env, craftGatewayIssueBody)

	req := httptest.NewRequest(http.MethodPost, "/craft/model-gateway/credentials/revoke",
		strings.NewReader(`{"grant_id":"grant_7_run-1"}`))
	w := httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	w = forwardOn(t, env, credential, `{"model":"m1","messages":[]}`)
	require.Equal(t, http.StatusForbidden, w.Code)
	require.Contains(t, w.Body.String(), "BUDGET_STOPPED")
	require.Contains(t, w.Body.String(), "cancelled")
	require.Equal(t, "", env.upstreamAuth())
}

func TestCraftGatewayListModelsStaysOpenWithoutBudgetCall(t *testing.T) {
	env := newCraftGatewayTestEnv(t)
	credential, _ := issueCredentialOn(t, env, craftGatewayIssueBody)

	req := httptest.NewRequest(http.MethodGet, "/craft/model-gateway/v1/models", nil)
	req.Header.Set("Authorization", "Bearer "+credential)
	w := httptest.NewRecorder()
	env.router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "m1")
	require.Equal(t, 0, env.budget.authorizeCount("grant_7_run-1"),
		"reads follow commercial over-limit semantics and never consume call budget")
}

func TestCraftGatewayNeverReadsPlatformKeysFromEnvironment(t *testing.T) {
	env := newCraftGatewayTestEnv(t)
	t.Setenv("OPENAI_API_KEY", "sk-env-longlived-platform-key")
	t.Setenv("CRAFT_MODEL_API_KEY", "sk-env-craft-key")

	gw, err := NewCraftModelGateway(CraftModelGatewayConfig{
		Secret:   []byte("test-signing-secret-16b"),
		Budget:   env.budget,
		Starter:  env.budget,
		Recorder: env.recorder,
		Upstream: func(context.Context, string) (CraftUpstreamTarget, error) {
			return CraftUpstreamTarget{}, fmt.Errorf("upstream credential provider unavailable")
		},
		GatewayBaseURL: "http://gateway.internal",
	})
	require.NoError(t, err)

	credential := issueCredentialOnBare(t, gw)
	r := gin.New()
	r.POST("/x", gw.Forward)
	req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(`{"model":"m1","messages":[]}`))
	req.Header.Set("Authorization", "Bearer "+credential)
	req.Header.Set(craftModelActivityHeader, "environment-test-1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusServiceUnavailable, w.Code)
	require.Contains(t, w.Body.String(), "UPSTREAM_UNAVAILABLE")
	require.NotContains(t, w.Body.String(), "sk-env", "environment keys must never surface")
	require.Equal(t, "", env.upstreamAuth(), "no env-derived key may reach an upstream")
	require.Zero(t, env.budget.authorizeCount("grant_7_run-1"), "upstream resolution failure occurs before durable start")
	require.Empty(t, env.budget.starts, "pre-send resolver failure creates no activity intent")
}

func TestCraftGatewayReissueAfterTokenRevocationKeepsGrant(t *testing.T) {
	env := newCraftGatewayTestEnv(t)
	first, firstData := issueCredentialOn(t, env, craftGatewayIssueBody)

	req := httptest.NewRequest(http.MethodPost, "/craft/model-gateway/credentials/revoke",
		strings.NewReader(fmt.Sprintf(`{"credential":%q}`, first)))
	w := httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// Recovery reissues on the SAME durable grant: new token, same identity.
	second, secondData := issueCredentialOn(t, env, craftGatewayIssueBody)
	require.NotEqual(t, first, second)
	require.Equal(t, firstData["grant"].(map[string]any)["id"], secondData["grant"].(map[string]any)["id"])

	w = forwardOn(t, env, second, `{"model":"m1","messages":[]}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
}

func TestCraftGatewayRejectsCrossBindingRequests(t *testing.T) {
	env := newCraftGatewayTestEnv(t)
	credential, _ := issueCredentialOn(t, env, craftGatewayIssueBody)

	// A body claiming another run/delegation/tenant never crosses bindings.
	for _, body := range []string{
		`{"model":"m1","run_id":"run-other","messages":[]}`,
		`{"model":"m1","delegation_id":"del-other","messages":[]}`,
		`{"model":"m1","tenant_id":99,"messages":[]}`,
	} {
		w := forwardOn(t, env, credential, body)
		require.Equal(t, http.StatusForbidden, w.Code, body)
		require.Contains(t, w.Body.String(), "BINDING_MISMATCH")
	}
	require.Equal(t, "", env.upstreamAuth())
}

func TestCraftGatewayConstructorRefusesIncompleteWiring(t *testing.T) {
	env := newCraftGatewayTestEnv(t)
	upstream := func(context.Context, string) (CraftUpstreamTarget, error) {
		return CraftUpstreamTarget{BaseURL: "http://u", Path: "/p", APIKey: "k"}, nil
	}
	_, err := NewCraftModelGateway(CraftModelGatewayConfig{Budget: env.budget, Recorder: env.recorder, Upstream: upstream})
	require.Error(t, err, "a gateway without a signing secret must not exist")
	_, err = NewCraftModelGateway(CraftModelGatewayConfig{Secret: []byte("test-signing-secret-16b"), Recorder: env.recorder, Upstream: upstream})
	require.Error(t, err, "a gateway without a budget port must not exist")
	_, err = NewCraftModelGateway(CraftModelGatewayConfig{Secret: []byte("test-signing-secret-16b"), Budget: env.budget, Upstream: upstream})
	require.Error(t, err, "a gateway without a usage recorder must not exist")
	_, err = NewCraftModelGateway(CraftModelGatewayConfig{Secret: []byte("test-signing-secret-16b"), Budget: env.budget, Recorder: env.recorder})
	require.Error(t, err, "a gateway without a server-configured upstream resolver must not exist")
	// A port without a durable starter cannot authorize model sends.
	_, err = NewCraftModelGateway(CraftModelGatewayConfig{
		Secret: []byte("test-signing-secret-16b"),
		Budget: portOnlyBudget{}, Recorder: env.recorder, Upstream: upstream,
	})
	require.Error(t, err)
	_, err = NewCraftModelGateway(CraftModelGatewayConfig{
		Secret: []byte("test-signing-secret-16b"),
		Budget: env.budget, Recorder: env.recorder, Upstream: upstream,
	})
	require.Error(t, err, "a gateway without the typed charge-start coordinator must fail construction")
}

// portOnlyBudget implements only the ordinary budget port and has no durable
// charge-start coordinator.
type portOnlyBudget struct{}

func (portOnlyBudget) Admit(context.Context, craft.Scope, string) (craft.BudgetGrant, error) {
	return craft.BudgetGrant{}, nil
}
func (portOnlyBudget) AuthorizeCall(context.Context, string, string) error { return nil }
func (portOnlyBudget) Reconcile(context.Context, string) error             { return nil }
