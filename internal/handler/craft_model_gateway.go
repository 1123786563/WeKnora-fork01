package handler

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
)

// The Craft controlled model gateway (O02). Every chargeable model call of a
// Craft execution — main agent runtime and OpenCode child runtime alike —
// enters through here: the OC runtime's model baseURL points at this gateway,
// never at a provider.
//
// Non-negotiable invariants:
//   - Execution credentials are short-lived, HMAC-signed, revocable, and bind
//     tenant + run + delegation + grant + allowed models + deadline. Anything
//     the credential did not bind is refused, and a checkpoint may only carry
//     the secret-free view.
//   - The client may never choose the upstream address, smuggle an API key,
//     or call a model outside the credential's scope.
//   - Platform long-lived keys NEVER come from the environment or the
//     workspace: the only key source is the server-configured upstream
//     resolver; a missing resolver fails closed with 503.
//   - Before every real forward the gateway authorizes the call through the
//     craft.BudgetPort (atomic commercial reservation). A denial is a hard,
//     VISIBLE stop (BUDGET_STOPPED) the main agent can read — never a silent
//     fallback to an ungated path.
//   - Reads (model listing) stay open: they follow the commercial over-limit
//     semantics of their own surface instead of a blanket space lock.

const (
	// craftCredentialPrefix versions the execution credential format.
	craftCredentialPrefix = "cmg1"
	// craftCredentialMaxTTL caps how long one execution credential lives.
	// Short-lived by design; revocation closes the remaining window.
	craftCredentialMaxTTL = 15 * time.Minute
	// craftCredentialMinSecret is the smallest accepted signing key.
	craftCredentialMinSecret = 16
	// craftMaxForwardBody bounds one forwarded model request.
	craftMaxForwardBody = 8 << 20
)

var (
	// ErrCraftGatewayConfig rejects incomplete or unsafe wiring.
	ErrCraftGatewayConfig = errors.New("craft_model_gateway_config_invalid")
)

// CraftUpstreamTarget is one server-resolved upstream endpoint: where the
// call goes and which managed credential carries it. It is produced ONLY by
// the server-configured resolver.
type CraftUpstreamTarget struct {
	BaseURL string
	Path    string
	APIKey  string
}

// CraftUpstreamResolver resolves the managed upstream of one model. It is the
// ONLY source of platform credentials in this gateway — implementations must
// read from the server's credential store, never from the environment or the
// workspace, and must fail closed.
type CraftUpstreamResolver func(ctx context.Context, model string) (CraftUpstreamTarget, error)

// CraftPhysicalCallRecorder records one physical model attempt into the O01
// usage ledger (service.CraftUsageService satisfies it).
type CraftPhysicalCallRecorder interface {
	RecordPhysicalCall(ctx context.Context, in service.PhysicalCall) (craft.UsageFact, error)
}

// CraftCallAuthorizer is the durable call-identity capability the gateway
// requires on top of craft.BudgetPort: AuthorizeBinding allocates the NEXT
// per-binding call identity (monotonic across restarts, O01 DeriveCallID) and
// atomically reserves its budget. service.CraftBudgetService implements it.
type CraftCallAuthorizer interface {
	AuthorizeBinding(ctx context.Context, grantID string, binding service.CraftCallBinding) (string, error)
}

// CraftGrantRevoker durably cancels a grant (cancellation path). Optional at
// wiring: without it the revoke endpoint can only revoke tokens.
type CraftGrantRevoker interface {
	RevokeGrant(ctx context.Context, grantID string) error
}

// CraftModelGatewayConfig wires the gateway. Budget, Recorder and Upstream
// are mandatory: a gateway without any of them must not exist, and Budget
// must additionally carry the durable identity capability.
type CraftModelGatewayConfig struct {
	Secret         []byte
	Budget         craft.BudgetPort
	Recorder       CraftPhysicalCallRecorder
	Upstream       CraftUpstreamResolver
	GatewayBaseURL string
	Now            func() time.Time
	Forward        func(req *http.Request) (*http.Response, error)
	MaxTTL         time.Duration
	ForwardTimeout time.Duration
}

// CraftModelGateway is the controlled model entry of Craft executions.
type CraftModelGateway struct {
	secret   []byte
	budget   craft.BudgetPort
	author   CraftCallAuthorizer
	revoker  CraftGrantRevoker
	recorder CraftPhysicalCallRecorder
	upstream CraftUpstreamResolver
	baseURL  string
	now      func() time.Time
	forward  func(req *http.Request) (*http.Response, error)
	maxTTL   time.Duration
	timeout  time.Duration

	mu      sync.Mutex
	revoked map[string]struct{} // revoked credential JTIs (short TTL bound)
}

// NewCraftModelGateway validates the wiring fail-closed: a signing secret, a
// budget port WITH the durable call-identity capability, a usage recorder and
// a server-configured upstream resolver are all mandatory. A nil port or an
// AllowAll-shaped fake cannot pass construction — the real charging assembly
// passes the durable service.CraftBudgetService.
func NewCraftModelGateway(cfg CraftModelGatewayConfig) (*CraftModelGateway, error) {
	if len(cfg.Secret) < craftCredentialMinSecret {
		return nil, fmt.Errorf("%w: signing secret of at least %d bytes is required", ErrCraftGatewayConfig, craftCredentialMinSecret)
	}
	if cfg.Budget == nil {
		return nil, fmt.Errorf("%w: a craft.BudgetPort is required", ErrCraftGatewayConfig)
	}
	author, ok := cfg.Budget.(CraftCallAuthorizer)
	if !ok {
		return nil, fmt.Errorf("%w: the budget port must provide durable call identities (AuthorizeBinding)", ErrCraftGatewayConfig)
	}
	if cfg.Recorder == nil {
		return nil, fmt.Errorf("%w: a physical-call recorder is required", ErrCraftGatewayConfig)
	}
	if cfg.Upstream == nil {
		return nil, fmt.Errorf("%w: a server-configured upstream resolver is required (never the environment)", ErrCraftGatewayConfig)
	}
	now := cfg.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	forward := cfg.Forward
	if forward == nil {
		forward = http.DefaultClient.Do
	}
	maxTTL := cfg.MaxTTL
	if maxTTL <= 0 {
		maxTTL = craftCredentialMaxTTL
	}
	timeout := cfg.ForwardTimeout
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	gw := &CraftModelGateway{
		secret:   append([]byte(nil), cfg.Secret...),
		budget:   cfg.Budget,
		author:   author,
		recorder: cfg.Recorder,
		upstream: cfg.Upstream,
		baseURL:  cfg.GatewayBaseURL,
		now:      now,
		forward:  forward,
		maxTTL:   maxTTL,
		timeout:  timeout,
		revoked:  map[string]struct{}{},
	}
	if revoker, ok := cfg.Budget.(CraftGrantRevoker); ok {
		gw.revoker = revoker
	}
	return gw, nil
}

// craftCredentialPayload is the signed binding of one execution credential.
// Every field is server-derived at issuance; the Forward path trusts ONLY
// this signed payload, never request parameters.
type craftCredentialPayload struct {
	JTI          string    `json:"jti"`
	TenantID     uint64    `json:"tenant_id"`
	RunID        string    `json:"run_id"`
	DelegationID string    `json:"delegation_id"`
	GrantID      string    `json:"grant_id"`
	Runtime      string    `json:"runtime"`
	Funding      string    `json:"funding"`
	Models       []string  `json:"models"`
	IssuedAt     time.Time `json:"issued_at"`
	ExpiresAt    time.Time `json:"expires_at"`
}

func (g *CraftModelGateway) sign(payload []byte) string {
	mac := hmac.New(sha256.New, g.secret)
	mac.Write(payload)
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (g *CraftModelGateway) issue(payload craftCredentialPayload) (string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return craftCredentialPrefix + "." + base64.RawURLEncoding.EncodeToString(body) + "." + g.sign(body), nil
}

// verify parses and authenticates one execution credential: format, signature
// (constant-time), expiry and the in-memory revocation registry. The short
// TTL bounds the registry's lifetime.
func (g *CraftModelGateway) verify(token string) (craftCredentialPayload, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[0] != craftCredentialPrefix {
		return craftCredentialPayload{}, fmt.Errorf("malformed credential")
	}
	body, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return craftCredentialPayload{}, fmt.Errorf("credential payload: %w", err)
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return craftCredentialPayload{}, fmt.Errorf("credential signature: %w", err)
	}
	mac := hmac.New(sha256.New, g.secret)
	mac.Write(body)
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return craftCredentialPayload{}, fmt.Errorf("credential signature mismatch")
	}
	var payload craftCredentialPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return craftCredentialPayload{}, fmt.Errorf("credential payload: %w", err)
	}
	if payload.JTI == "" || payload.TenantID == 0 || payload.RunID == "" || payload.GrantID == "" {
		return craftCredentialPayload{}, fmt.Errorf("credential binding incomplete")
	}
	if !g.now().Before(payload.ExpiresAt) {
		return craftCredentialPayload{}, fmt.Errorf("credential expired")
	}
	g.mu.Lock()
	_, revoked := g.revoked[payload.JTI]
	g.mu.Unlock()
	if revoked {
		return craftCredentialPayload{}, fmt.Errorf("credential revoked")
	}
	return payload, nil
}

func (g *CraftModelGateway) revokeJTI(jti string) {
	g.mu.Lock()
	g.revoked[jti] = struct{}{}
	g.mu.Unlock()
}

// credentialFromRequest extracts and verifies the bearer credential.
func (g *CraftModelGateway) credentialFromRequest(c *gin.Context) (craftCredentialPayload, bool) {
	header := c.GetHeader("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		appFail(c, http.StatusUnauthorized, "CREDENTIAL_INVALID", "execution credential required")
		return craftCredentialPayload{}, false
	}
	payload, err := g.verify(strings.TrimSpace(strings.TrimPrefix(header, "Bearer ")))
	if err != nil {
		code := "CREDENTIAL_INVALID"
		switch {
		case strings.Contains(err.Error(), "expired"):
			code = "CREDENTIAL_EXPIRED"
		case strings.Contains(err.Error(), "revoked"):
			code = "CREDENTIAL_REVOKED"
		}
		appFail(c, http.StatusUnauthorized, code, err.Error())
		return craftCredentialPayload{}, false
	}
	return payload, true
}

// IssueCredential POST /craft/model-gateway/credentials.
//
// The admission path of a chargeable run: tenant scope ALWAYS comes from the
// authenticated context; the budget port admits the run (idempotent per run)
// and the gateway issues a short-lived execution credential bound to that
// grant. The response's checkpoint view is structurally secret-free — the
// token itself must never be persisted in a checkpoint.
func (g *CraftModelGateway) IssueCredential(c *gin.Context) {
	tenantID, _, _, ok := appTenantScope(c)
	if !ok {
		return
	}
	var input struct {
		RunID        string   `json:"run_id"`
		DelegationID string   `json:"delegation_id"`
		Runtime      string   `json:"runtime"`
		Models       []string `json:"models"`
		Funding      string   `json:"funding"`
		TTLSeconds   int      `json:"ttl_seconds"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || input.RunID == "" || len(input.Models) == 0 || input.Funding == "" {
		appFail(c, http.StatusBadRequest, "INVALID_REQUEST",
			"run_id, models[] and funding are required")
		return
	}
	runtime := input.Runtime
	if runtime == "" {
		runtime = craft.RuntimeOC
	}
	if runtime != craft.RuntimeMain && runtime != craft.RuntimeOC {
		appFail(c, http.StatusBadRequest, "INVALID_REQUEST", "runtime must be main or oc")
		return
	}
	userID, _ := types.UserIDFromContext(c.Request.Context())
	sessionID := ""
	scope := craft.Scope{TenantID: tenantID, UserID: userID, SessionID: sessionID}
	grant, err := g.budget.Admit(c.Request.Context(), scope, input.RunID)
	if err != nil {
		g.failBudget(c, err, true)
		return
	}
	if !grant.Allowed {
		appFail(c, http.StatusForbidden, "BUDGET_STOPPED", "run admission is not allowed: grant was cancelled")
		return
	}
	if !g.now().Before(grant.Deadline) {
		appFail(c, http.StatusForbidden, "BUDGET_STOPPED", "run admission is not allowed: budget deadline passed")
		return
	}
	ttl := g.maxTTL
	if input.TTLSeconds > 0 {
		ttl = time.Duration(input.TTLSeconds) * time.Second
		if ttl > g.maxTTL {
			ttl = g.maxTTL
		}
	}
	now := g.now()
	expires := now.Add(ttl)
	if grant.Deadline.Before(expires) {
		expires = grant.Deadline // a credential never outlives its grant
	}
	jti := newCraftJTI()
	token, err := g.issue(craftCredentialPayload{
		JTI: jti, TenantID: tenantID, RunID: input.RunID, DelegationID: input.DelegationID,
		GrantID: grant.ID, Runtime: runtime, Funding: input.Funding,
		Models: append([]string(nil), input.Models...), IssuedAt: now, ExpiresAt: expires,
	})
	if err != nil {
		appFail(c, http.StatusInternalServerError, "CREDENTIAL_ISSUE_FAILED", "failed to sign the execution credential")
		return
	}
	appOK(c, http.StatusOK, gin.H{
		"credential":       token,
		"expires_at":       expires.UTC(),
		"gateway_base_url": g.baseURL,
		"grant": gin.H{
			"id": grant.ID, "deadline": grant.Deadline.UTC(),
			"max_calls": grant.MaxCalls, "used_calls": grant.UsedCalls, "allowed": grant.Allowed,
		},
		// Checkpoint-safe view: everything an execution checkpoint may persist
		// EXCEPT the credential token. Structurally unable to leak the secret.
		"checkpoint": gin.H{
			"grant_id": grant.ID, "run_id": input.RunID, "delegation_id": input.DelegationID,
			"runtime": runtime, "models": input.Models, "funding": input.Funding,
			"expires_at": expires.UTC(),
		},
	})
}

// RevokeCredential POST /craft/model-gateway/credentials/revoke.
//
// Immediate revocation: a specific token (registry hit) and/or the whole
// durable grant (new calls of the run stop; already-admitted calls settle
// inside their deadline). Cancellation of a run funnels through here.
func (g *CraftModelGateway) RevokeCredential(c *gin.Context) {
	var input struct {
		Credential string `json:"credential"`
		GrantID    string `json:"grant_id"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || (input.Credential == "" && input.GrantID == "") {
		appFail(c, http.StatusBadRequest, "INVALID_REQUEST", "credential or grant_id is required")
		return
	}
	if input.Credential != "" {
		payload, err := g.verify(input.Credential)
		if err == nil {
			g.revokeJTI(payload.JTI)
		} else {
			// The token may be expired already — revoking its JTI anyway is a
			// harmless no-op that keeps cancellation idempotent.
			if parts := strings.Split(input.Credential, "."); len(parts) == 3 {
				if body, derr := base64.RawURLEncoding.DecodeString(parts[1]); derr == nil {
					var payload craftCredentialPayload
					if jerr := json.Unmarshal(body, &payload); jerr == nil && payload.JTI != "" {
						g.revokeJTI(payload.JTI)
					}
				}
			}
		}
	}
	if input.GrantID != "" {
		if g.revoker == nil {
			appFail(c, http.StatusNotImplemented, "GRANT_REVOKE_UNAVAILABLE",
				"the configured budget port cannot revoke grants")
			return
		}
		if err := g.revoker.RevokeGrant(c.Request.Context(), input.GrantID); err != nil {
			appFail(c, http.StatusInternalServerError, "GRANT_REVOKE_FAILED", err.Error())
			return
		}
	}
	appOK(c, http.StatusOK, gin.H{"revoked": true})
}

// craftForbiddenBodyFields are request parameters a client must never set:
// they would let model traffic escape the controlled upstream.
var craftForbiddenBodyFields = []string{
	"base_url", "baseUrl", "baseURL", "endpoint", "upstream", "url",
	"api_key", "apiKey", "api_key_alias", "credential", "authorization",
}

// Forward POST /craft/model-gateway/v1/chat/completions — the OC runtime's
// model entry. Order of gates: credential -> binding/model/upstream fields ->
// budget authorization -> server upstream resolution -> forward -> record.
// A budget denial is a hard, visible stop; recording happens even for broken
// responses (nil usage = unknown observation, per the O01 ledger contract).
func (g *CraftModelGateway) Forward(c *gin.Context) {
	payload, ok := g.credentialFromRequest(c)
	if !ok {
		return
	}
	raw, err := io.ReadAll(io.LimitReader(c.Request.Body, craftMaxForwardBody))
	if err != nil {
		appFail(c, http.StatusBadRequest, "INVALID_REQUEST", "unreadable request body")
		return
	}
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		appFail(c, http.StatusBadRequest, "INVALID_REQUEST", "request body must be a JSON object")
		return
	}
	for _, field := range craftForbiddenBodyFields {
		if _, present := body[field]; present {
			appFail(c, http.StatusForbidden, "CLIENT_UPSTREAM_FORBIDDEN",
				"clients cannot choose the upstream or supply credentials: "+field)
			return
		}
	}
	model, _ := body["model"].(string)
	if model == "" {
		appFail(c, http.StatusBadRequest, "INVALID_REQUEST", "model is required")
		return
	}
	if !craftCredentialAllowsModel(payload, model) {
		appFail(c, http.StatusForbidden, "MODEL_NOT_ALLOWED",
			"model is outside this execution credential's scope")
		return
	}
	if mismatch := craftBindingMismatch(payload, body); mismatch != "" {
		appFail(c, http.StatusForbidden, "BINDING_MISMATCH",
			"request "+mismatch+" does not match the execution credential binding")
		return
	}

	callID, err := g.author.AuthorizeBinding(c.Request.Context(), payload.GrantID, service.CraftCallBinding{
		DelegationID: payload.DelegationID, ModelID: model, Funding: payload.Funding,
	})
	if err != nil {
		g.failBudget(c, err, false)
		return
	}
	attemptID := craft.DeriveAttemptID(callID, 1)

	target, err := g.upstream(c.Request.Context(), model)
	if err != nil || target.BaseURL == "" || target.APIKey == "" {
		// Fail closed: there is NO environment fallback for platform keys.
		appFail(c, http.StatusServiceUnavailable, "UPSTREAM_UNAVAILABLE",
			"no managed upstream credential is configured for this model")
		return
	}

	forwardBody, _ := json.Marshal(body)
	endpoint := strings.TrimSuffix(target.BaseURL, "/") + target.Path
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, endpoint, bytes.NewReader(forwardBody))
	if err != nil {
		appFail(c, http.StatusInternalServerError, "FORWARD_FAILED", err.Error())
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+target.APIKey)
	ctx, cancel := context.WithTimeout(c.Request.Context(), g.timeout)
	defer cancel()
	resp, err := g.forward(req.WithContext(ctx))
	if err != nil {
		g.recordCall(c, payload, callID, attemptID, model, nil)
		appFail(c, http.StatusBadGateway, "UPSTREAM_ERROR", err.Error())
		return
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, craftMaxForwardBody))
	if err != nil {
		g.recordCall(c, payload, callID, attemptID, model, nil)
		appFail(c, http.StatusBadGateway, "UPSTREAM_ERROR", err.Error())
		return
	}
	g.recordCall(c, payload, callID, attemptID, model, craftParseUsage(respBody))
	if ct := resp.Header.Get("Content-Type"); ct != "" {
		c.Header("Content-Type", ct)
	}
	c.Data(resp.StatusCode, "application/json", respBody)
}

// ListModels GET /craft/model-gateway/v1/models — a credential-scoped READ.
// Reads are NOT budget-gated: they follow the commercial over-limit
// semantics of their own surface, so an out-of-budget run can still list,
// download and clean up — never a blanket space lock.
func (g *CraftModelGateway) ListModels(c *gin.Context) {
	payload, ok := g.credentialFromRequest(c)
	if !ok {
		return
	}
	models := append([]string(nil), payload.Models...)
	if models == nil {
		models = []string{}
	}
	appOK(c, http.StatusOK, gin.H{"models": models, "run_id": payload.RunID})
}

// recordCall appends the physical attempt to the O01 usage ledger. nil usage
// records an unknown observation (no fabricated numbers); a recording error
// is logged into the response but never swallowed silently.
func (g *CraftModelGateway) recordCall(c *gin.Context, payload craftCredentialPayload, callID, attemptID, model string, usage *craft.UsageTotals) {
	_, err := g.recorder.RecordPhysicalCall(c.Request.Context(), service.PhysicalCall{
		TenantID:     payload.TenantID,
		RunID:        payload.RunID,
		DelegationID: payload.DelegationID,
		CallID:       callID,
		AttemptID:    attemptID,
		Runtime:      payload.Runtime,
		ModelID:      model,
		Funding:      payload.Funding,
		Usage:        usage,
	})
	if err != nil {
		// The ledger is the billing ground truth: surface the failure rather
		// than letting an unrecorded charge pass silently.
		c.Header("X-Craft-Usage-Record-Error", err.Error())
	}
}

// failBudget maps a budget-port refusal to a visible stop. admission marks
// whether the failure happened at run admission (403) or at call
// authorization (402 payment-required): both carry BUDGET_STOPPED so the
// main agent can read the stop and its reason verbatim.
func (g *CraftModelGateway) failBudget(c *gin.Context, err error, admission bool) {
	status := http.StatusPaymentRequired
	if admission {
		status = http.StatusForbidden
	}
	switch {
	case errors.Is(err, craft.ErrGrantRevoked):
		appFail(c, http.StatusForbidden, "BUDGET_STOPPED", "model call blocked: run was cancelled ("+err.Error()+")")
	case errors.Is(err, craft.ErrGrantExpired):
		appFail(c, http.StatusForbidden, "BUDGET_STOPPED", "model call blocked: budget deadline passed ("+err.Error()+")")
	case errors.Is(err, craft.ErrGrantExhausted), errors.Is(err, craft.ErrBudgetDenied):
		appFail(c, status, "BUDGET_STOPPED", "model call blocked by budget: "+err.Error())
	default:
		appFail(c, http.StatusInternalServerError, "BUDGET_GATE_FAILED", err.Error())
	}
}

func craftCredentialAllowsModel(payload craftCredentialPayload, model string) bool {
	for _, m := range payload.Models {
		if m == model {
			return true
		}
	}
	return false
}

// craftBindingMismatch returns the request field that contradicts the signed
// credential binding, or "" when consistent. Identity comes from the signed
// credential only — a request may repeat it but never contradict it.
func craftBindingMismatch(payload craftCredentialPayload, body map[string]any) string {
	if v, ok := body["run_id"].(string); ok && v != payload.RunID {
		return "run_id"
	}
	if v, ok := body["delegation_id"].(string); ok && v != payload.DelegationID {
		return "delegation_id"
	}
	if v, ok := body["tenant_id"].(float64); ok && uint64(v) != payload.TenantID {
		return "tenant_id"
	}
	return ""
}

// craftParseUsage maps provider usage blocks onto the O01 totals contract:
// prompt_tokens/input_tokens, completion_tokens/output_tokens and the cached
// portion of input (prompt_tokens_details.cached_tokens or
// cache_read_input_tokens). Anything unparseable yields nil — unknown, never
// fabricated zeros.
func craftParseUsage(body []byte) *craft.UsageTotals {
	var parsed struct {
		Usage *struct {
			PromptTokens     float64 `json:"prompt_tokens"`
			InputTokens      float64 `json:"input_tokens"`
			CompletionTokens float64 `json:"completion_tokens"`
			OutputTokens     float64 `json:"output_tokens"`
			CacheRead        float64 `json:"cache_read_input_tokens"`
			Details          *struct {
				CachedTokens float64 `json:"cached_tokens"`
			} `json:"prompt_tokens_details"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil || parsed.Usage == nil {
		return nil
	}
	u := parsed.Usage
	input := u.PromptTokens
	if u.InputTokens > 0 {
		input = u.InputTokens
	}
	output := u.CompletionTokens
	if u.OutputTokens > 0 {
		output = u.OutputTokens
	}
	cached := u.CacheRead
	if u.Details != nil && u.Details.CachedTokens > 0 {
		cached = u.Details.CachedTokens
	}
	return &craft.UsageTotals{
		Input:  int64(input),
		Output: int64(output),
		Cached: int64(cached),
	}
}

// registeredCraftModelGateway holds the production gateway for route
// mounting (two auth planes: credential issuance behind the global Auth,
// Forward/ListModels pre-auth on the cmg1 HMAC credential).
var registeredCraftModelGateway *CraftModelGateway

// RegisterCraftModelGateway installs the gateway for route mounting.
func RegisterCraftModelGateway(g *CraftModelGateway) { registeredCraftModelGateway = g }

// RegisteredCraftModelGateway returns the registered gateway (nil keeps
// every gateway route unmounted — fail-closed, no 503 shims).
func RegisteredCraftModelGateway() *CraftModelGateway { return registeredCraftModelGateway }

func newCraftJTI() string {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("jti-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}
