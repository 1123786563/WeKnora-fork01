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
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/craft"
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
	craftMaxForwardBody     = 8 << 20
	craftUsageRecordTimeout = 10 * time.Second // bounded loss window for O01 usage facts; ledger reconcile remains the backstop
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

// CraftChargeStarter coordinates the committed intent and G4 hold before
// beginning a physical model request. Existing or unresolved activity keys
// must be rejected without invoking start again.
type CraftChargeStarter interface {
	BeginBinding(ctx context.Context, grantID, activityID string, binding service.CraftCallBinding) (service.CraftChargeStartAttempt, error)
}

// CraftGrantRevoker durably cancels a grant (cancellation path). Optional at
// wiring: without it the revoke endpoint can only revoke tokens.
type CraftGrantRevoker interface {
	RevokeGrant(ctx context.Context, grantID string) error
}

// CraftModelGatewayConfig wires the gateway. Budget, Starter, Recorder and
// Upstream are mandatory: no model forward may exist without a durable
// committed-start coordinator.
type CraftModelGatewayConfig struct {
	Secret         []byte
	Budget         craft.BudgetPort
	Starter        CraftChargeStarter
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
	secret             []byte
	budget             craft.BudgetPort
	starter            CraftChargeStarter
	revoker            CraftGrantRevoker
	recorder           CraftPhysicalCallRecorder
	upstream           CraftUpstreamResolver
	baseURL            string
	now                func() time.Time
	forward            func(req *http.Request) (*http.Response, error)
	maxTTL             time.Duration
	timeout            time.Duration
	usageRecordTimeout time.Duration

	mu      sync.Mutex
	revoked map[string]struct{} // revoked credential JTIs (short TTL bound)
}

// NewCraftModelGateway validates the wiring fail-closed: a signing secret, a
// budget port, a durable charge-start coordinator, a usage recorder and a
// server-configured upstream resolver are all mandatory.
func NewCraftModelGateway(cfg CraftModelGatewayConfig) (*CraftModelGateway, error) {
	if len(cfg.Secret) < craftCredentialMinSecret {
		return nil, fmt.Errorf("%w: signing secret of at least %d bytes is required", ErrCraftGatewayConfig, craftCredentialMinSecret)
	}
	if cfg.Budget == nil {
		return nil, fmt.Errorf("%w: a craft.BudgetPort is required", ErrCraftGatewayConfig)
	}
	if cfg.Starter == nil {
		return nil, fmt.Errorf("%w: a durable charge-start coordinator is required", ErrCraftGatewayConfig)
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
		secret:             append([]byte(nil), cfg.Secret...),
		budget:             cfg.Budget,
		starter:            cfg.Starter,
		recorder:           cfg.Recorder,
		upstream:           cfg.Upstream,
		baseURL:            cfg.GatewayBaseURL,
		now:                now,
		forward:            forward,
		maxTTL:             maxTTL,
		timeout:            timeout,
		usageRecordTimeout: craftUsageRecordTimeout,
		revoked:            map[string]struct{}{},
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
		g.failBudget(c, err, true, "")
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

const craftModelActivityHeader = "X-Craft-Activity-ID"

// Forward POST /craft/model-gateway/v1/chat/completions — the OC runtime's
// model entry. Order of gates: credential -> binding/model/upstream fields ->
// stable activity identity and server upstream resolution -> committed charge
// start -> bounded Do initiation -> response/body and usage outside SQL. A
// budget denial is a hard, visible stop; recording happens even for broken
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

	activityID, err := craftModelActivityKey(payload, c.GetHeader(craftModelActivityHeader))
	if err != nil {
		appFail(c, http.StatusBadRequest, "ACTIVITY_ID_REQUIRED", "a stable X-Craft-Activity-ID is required for each model activity")
		return
	}
	callID := "activity/" + activityID
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
	// StartBinding bounds transport initiation. A separate bounded context
	// derived from the inbound request owns the response body through close.
	responseCtx, cancelResponse := context.WithTimeout(c.Request.Context(), g.timeout)
	defer cancelResponse()
	req, err := http.NewRequestWithContext(responseCtx, http.MethodPost, endpoint, bytes.NewReader(forwardBody))
	if err != nil {
		appFail(c, http.StatusInternalServerError, "FORWARD_FAILED", err.Error())
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+target.APIKey)
	attempt, err := g.starter.BeginBinding(c.Request.Context(), payload.GrantID, activityID,
		service.CraftCallBinding{DelegationID: payload.DelegationID, ModelID: model, Funding: payload.Funding})
	if err != nil {
		if errors.Is(err, craft.ErrConflict) {
			appFail(c, http.StatusConflict, "ACTIVITY_UNRESOLVED", "this model activity was already attempted; reconcile before retry")
			return
		}
		g.failBudget(c, err, false, payload.GrantID)
		return
	}
	defer attempt.CancelInitiation()
	resp, forwardErr, attempted, initiationExpiredRaw, cancelRequest := g.forwardWithinInitiation(attempt, req)
	// err==nil proves the cancellation never took effect: a response that
	// raced the deadline callback must not be rewritten into a phantom
	// DeadlineExceeded (which would Resolve Unknown and park the Run).
	initiationExpired := initiationExpiredRaw && forwardErr != nil
	if cancelRequest != nil {
		defer cancelRequest()
	}
	if !attempted {
		resolveErr := attempt.Resolve(c.Request.Context(), service.CraftChargeStartDefinitelyNotStarted)
		if resolveErr != nil {
			// The client sees only the opaque code; the detail (which may
			// quote SQL or table names) stays in the server log, exactly
			// like recordCall's redaction discipline.
			logger.ErrorWithFields(c.Request.Context(), resolveErr, map[string]any{
				"craft_run_id": payload.RunID, "craft_call_id": callID, "craft_attempt_id": attemptID,
			})
			appFail(c, http.StatusBadGateway, "ACTIVITY_UNRESOLVED", "the activity outcome could not be recorded")
			return
		}
		// DefinitelyNotStarted resolved above — a definitive clean failure.
		// UPSTREAM_ERROR (not ACTIVITY_UNRESOLVED) keeps the adapter from
		// parking an id whose retry is provably safe.
		appFail(c, http.StatusBadGateway, "UPSTREAM_ERROR", "the activity never started before the initiation deadline")
		return
	}
	if initiationExpired {
		closeGatewayResponse(resp)
		g.recordCall(c, payload, callID, attemptID, model, nil)
		resolveErr := attempt.Resolve(c.Request.Context(), service.CraftChargeStartUnknown)
		if resolveErr != nil {
			logger.ErrorWithFields(c.Request.Context(), resolveErr, map[string]any{
				"craft_run_id": payload.RunID, "craft_call_id": callID, "craft_attempt_id": attemptID,
			})
		}
		appFail(c, http.StatusBadGateway, "ACTIVITY_UNRESOLVED", "the activity start outcome is unknown and requires reconciliation")
		return
	}
	if forwardErr != nil || resp == nil {
		closeGatewayResponse(resp)
		g.recordCall(c, payload, callID, attemptID, model, nil)
		if forwardErr == nil {
			forwardErr = errors.New("model transport returned no response")
		}
		resolveErr := attempt.Resolve(c.Request.Context(), service.CraftChargeStartUnknown)
		if resolveErr != nil {
			logger.ErrorWithFields(c.Request.Context(), resolveErr, map[string]any{
				"craft_run_id": payload.RunID, "craft_call_id": callID, "craft_attempt_id": attemptID,
			})
		}
		// The charge-start journal resolved Unknown: the send MAY have left.
		// The machine-readable code must say so — the egress adapter keys
		// its parked/resolved decision off this code.
		appFail(c, http.StatusBadGateway, "ACTIVITY_UNRESOLVED", "the upstream model request failed with an unknown activity outcome")
		return
	}
	if resp.Body == nil {
		// Headers returned => the physical call started: resolve Started
		// (deterministic) instead of parking the Run as unknown.
		if resolveErr := attempt.Resolve(c.Request.Context(), service.CraftChargeStartStarted); resolveErr != nil {
			logger.ErrorWithFields(c.Request.Context(), resolveErr, map[string]any{
				"craft_run_id": payload.RunID, "craft_call_id": callID, "craft_attempt_id": attemptID,
			})
		}
		g.recordCall(c, payload, callID, attemptID, model, nil)
		appFail(c, http.StatusBadGateway, "UPSTREAM_ERROR", "the upstream response body is missing; the activity started and failed")
		return
	}
	respBody, readErr := io.ReadAll(io.LimitReader(resp.Body, craftMaxForwardBody+1))
	closeErr := resp.Body.Close()
	if readErr != nil || len(respBody) == 0 || len(respBody) > craftMaxForwardBody || closeErr != nil {
		// resp != nil here means the response HEADERS returned: the physical
		// call factually started (and is recorded in the O01 ledger below),
		// so the durable outcome is Started, not Unknown — an Unknown here
		// would park the adapter id and exclude the Run from lease recovery.
		if resolveErr := attempt.Resolve(c.Request.Context(), service.CraftChargeStartStarted); resolveErr != nil {
			logger.ErrorWithFields(c.Request.Context(), resolveErr, map[string]any{
				"craft_run_id": payload.RunID, "craft_call_id": callID, "craft_attempt_id": attemptID,
			})
		}
		g.recordCall(c, payload, callID, attemptID, model, craftParseUsage(respBody))
		failure := errors.Join(readErr, closeErr)
		if len(respBody) == 0 && readErr == nil {
			failure = errors.Join(failure, errors.New("upstream response body is empty"))
		}
		if len(respBody) > craftMaxForwardBody {
			failure = errors.Join(failure, errors.New("upstream response body exceeds maximum size"))
		}
		// (The Started resolution above already fixed the durable outcome;
		// a second, contradictory Unknown resolve here is dead residue.)
		// Upstream transport errors can quote internal IP:port topology and
		// resolve errors can quote SQL — both stay in the server log; the
		// client gets the opaque code and a fixed sentence.
		logger.ErrorWithFields(c.Request.Context(), failure, map[string]any{
			"craft_run_id": payload.RunID, "craft_call_id": callID, "craft_attempt_id": attemptID,
		})
		appFail(c, http.StatusBadGateway, "UPSTREAM_ERROR", "the upstream response could not be read; the activity started and failed")
		return
	}
	if err := attempt.Resolve(c.Request.Context(), service.CraftChargeStartStarted); err != nil {
		// The response body was fully observed above: its usage fact is
		// idempotently recorded even when the Started resolution failed, so
		// manual reconciliation keeps a fixed billing basis.
		g.recordCall(c, payload, callID, attemptID, model, craftParseUsage(respBody))
		logger.ErrorWithFields(c.Request.Context(), err, map[string]any{
			"craft_run_id": payload.RunID, "craft_call_id": callID, "craft_attempt_id": attemptID,
		})
		appFail(c, http.StatusBadGateway, "ACTIVITY_UNRESOLVED", "the activity started but its record could not be persisted; reconciliation required")
		return
	}
	g.recordCall(c, payload, callID, attemptID, model, craftParseUsage(respBody))
	if ct := resp.Header.Get("Content-Type"); ct != "" {
		c.Header("Content-Type", ct)
	}
	c.Data(resp.StatusCode, "application/json", respBody)
}

// forwardWithinInitiation cancels a blocked Do when the coordinator's start
// context expires. Once Do returns headers, that deadline no longer cancels
// the request context; the whole-response context owns the body through Close.
func (g *CraftModelGateway) forwardWithinInitiation(attempt service.CraftChargeStartAttempt, req *http.Request) (*http.Response, error, bool, bool, context.CancelFunc) {
	startCtx := attempt.InitiationContext()
	if err := startCtx.Err(); err != nil {
		return nil, err, false, false, nil
	}
	requestCtx, cancelRequest := context.WithCancel(req.Context())
	var mu sync.Mutex
	responseHeadersReturned := false
	stopDeadline := context.AfterFunc(startCtx, func() {
		mu.Lock()
		defer mu.Unlock()
		if !responseHeadersReturned {
			cancelRequest()
		}
	})
	resp, err := g.forward(req.WithContext(requestCtx))
	mu.Lock()
	responseHeadersReturned = true
	initiationExpired := startCtx.Err() != nil
	mu.Unlock()
	stopDeadline()
	attempt.CancelInitiation()
	if initiationExpired {
		// err == nil proves the deadline callback never cancelled the
		// request (responseHeadersReturned=true skips it): a fully
		// successful response must NOT be rewritten into a phantom
		// DeadlineExceeded — that would Resolve Unknown and park an
		// activity whose physical call demonstrably completed.
		if err != nil || resp == nil {
			cancelRequest()
			if err == nil {
				err = context.DeadlineExceeded
			}
			return resp, err, true, true, cancelRequest
		}
		return resp, err, true, false, cancelRequest
	}
	return resp, err, true, false, cancelRequest
}

func closeGatewayResponse(resp *http.Response) {
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
}

// craftModelActivityKey binds the runtime-supplied stable request identity to
// the authenticated run/grant scope. Binding facets stay out of the activity
// key so reusing one ID with a different model/delegation/funding conflicts
// in the durable coordinator. The raw key is not persisted in the journal.
func craftModelActivityKey(payload craftCredentialPayload, requestActivityID string) (string, error) {
	if requestActivityID == "" || len(requestActivityID) > 128 || strings.TrimSpace(requestActivityID) != requestActivityID {
		return "", craft.ErrInvalidInput
	}
	for _, char := range requestActivityID {
		if !(char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || strings.ContainsRune("._:-", char)) {
			return "", craft.ErrInvalidInput
		}
	}
	identity := fmt.Sprintf("%d\x00%s\x00%s\x00%s", payload.TenantID,
		payload.RunID, payload.GrantID, requestActivityID)
	digest := sha256.Sum256([]byte(identity))
	return "model-" + hex.EncodeToString(digest[:]), nil
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
// records an unknown observation (no fabricated numbers). Persistence uses a
// bounded context detached from request cancellation so a client disconnect
// cannot suppress the durable fact after provider traffic has started.
func (g *CraftModelGateway) recordCall(c *gin.Context, payload craftCredentialPayload, callID, attemptID, model string, usage *craft.UsageTotals) {
	timeout := g.usageRecordTimeout
	if timeout <= 0 {
		timeout = craftUsageRecordTimeout
	}
	recordCtx, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), timeout)
	defer cancel()
	_, err := g.recorder.RecordPhysicalCall(recordCtx, service.PhysicalCall{
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
		// The response header is supplemental because a canceled caller may
		// never observe it. Emit durable identities for operator reconciliation;
		// the raw error stays server-side only — clients get an opaque marker.
		logger.ErrorWithFields(c.Request.Context(), err, logger.Fields{
			"event":      "craft_model_gateway_usage_record_failed",
			"run_id":     payload.RunID,
			"call_id":    callID,
			"attempt_id": attemptID,
		})
		c.Header("X-Craft-Usage-Record-Error", "usage_record_failed")
		c.Header("X-Craft-Usage-Record-Run-ID", payload.RunID)
		c.Header("X-Craft-Usage-Record-Call-ID", callID)
		c.Header("X-Craft-Usage-Record-Attempt-ID", attemptID)
	}
}

// failBudget maps a budget-port refusal to a visible stop. admission marks
// whether the failure happened at run admission (403) or at call
// authorization (402 payment-required): both carry BUDGET_STOPPED so the
// main agent can read the stop and its reason verbatim.
func (g *CraftModelGateway) failBudget(c *gin.Context, err error, admission bool, grantID string) {
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
		// Infrastructure failures (GORM/driver errors may quote SQL or
		// internal DB topology) log server-side; the client sees only the
		// opaque code and a fixed sentence.
		logger.ErrorWithFields(c.Request.Context(), err, map[string]any{
			"craft_grant_id": grantID,
		})
		appFail(c, http.StatusInternalServerError, "BUDGET_GATE_FAILED", "the budget gate could not be consulted; retry or contact the operator")
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
