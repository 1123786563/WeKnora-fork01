package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// craftGatewayBudget is a TEST budget port with real admission semantics
// (grants, call counting, denial injection). It is NOT an AllowAll stub and
// it lives only in _test.go: production charging assemblies must wire the
// durable service.CraftBudgetService, never a fake.
type craftGatewayBudget struct {
	mu      sync.Mutex
	grants  map[string]craft.BudgetGrant
	byRun   map[string]string
	calls   map[string]int // grantID -> distinct authorized callIDs
	callIDs map[string]bool
	denyErr error
	nextSeq int
	revoked map[string]bool
}

func newCraftGatewayBudget() *craftGatewayBudget {
	return &craftGatewayBudget{
		grants:  map[string]craft.BudgetGrant{},
		byRun:   map[string]string{},
		calls:   map[string]int{},
		callIDs: map[string]bool{},
		revoked: map[string]bool{},
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

func (b *craftGatewayBudget) AuthorizeCall(_ context.Context, grantID, callID string) error {
	if err := b.authorize(grantID); err != nil {
		return err
	}
	b.count(grantID, callID)
	return nil
}

func (b *craftGatewayBudget) AuthorizeBinding(_ context.Context, grantID string, binding service.CraftCallBinding) (string, error) {
	if err := b.authorize(grantID); err != nil {
		return "", err
	}
	b.mu.Lock()
	b.nextSeq++
	seq := b.nextSeq
	b.mu.Unlock()
	callID := craft.DeriveCallID(7, "run-1", binding.DelegationID, binding.ModelID, binding.Funding, int64(seq))
	b.count(grantID, callID)
	return callID, nil
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
	mu sync.Mutex
	v  string
}

func (a *craftUpstreamAuth) Store(s string) { a.mu.Lock(); a.v = s; a.mu.Unlock() }
func (a *craftUpstreamAuth) Load() string   { a.mu.Lock(); defer a.mu.Unlock(); return a.v }

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
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/craft/model-gateway/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+credential)
	w := httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	return w
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
	recorded := env.recorder.recorded()
	require.Len(t, recorded, 1)
	fact := recorded[0]
	require.Equal(t, uint64(7), fact.TenantID)
	require.Equal(t, "run-1", fact.RunID)
	require.Equal(t, "del-1", fact.DelegationID)
	require.Equal(t, craft.RuntimeOC, fact.Runtime)
	require.Equal(t, "m1", fact.ModelID)
	require.Equal(t, "platform", fact.Funding)
	require.True(t, strings.HasPrefix(fact.CallID, "call_"))
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
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusServiceUnavailable, w.Code)
	require.Contains(t, w.Body.String(), "UPSTREAM_UNAVAILABLE")
	require.NotContains(t, w.Body.String(), "sk-env", "environment keys must never surface")
	require.Equal(t, "", env.upstreamAuth(), "no env-derived key may reach an upstream")
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
	// A budget port without the durable call-identity capability is refused
	// too: without it call identities could restart at zero after recovery.
	_, err = NewCraftModelGateway(CraftModelGatewayConfig{
		Secret: []byte("test-signing-secret-16b"),
		Budget: portOnlyBudget{}, Recorder: env.recorder, Upstream: upstream,
	})
	require.Error(t, err)
}

// portOnlyBudget implements ONLY the three port methods, proving the gateway
// demands the durable identity capability on top of the port.
type portOnlyBudget struct{}

func (portOnlyBudget) Admit(context.Context, craft.Scope, string) (craft.BudgetGrant, error) {
	return craft.BudgetGrant{}, nil
}
func (portOnlyBudget) AuthorizeCall(context.Context, string, string) error { return nil }
func (portOnlyBudget) Reconcile(context.Context, string) error             { return nil }
