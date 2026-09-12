package connectorcontrol

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/appconnector/openconnector"
)

// Admin wire facts are FROZEN by the T01 runtime-evidence contract
// (docs/integrations/open-connector-contract.md §3.1/§3.2/§3.4/§3.6,
// fixtures admin_denied.json / empty_grant.json / cross_connection.json):
//
//   - admin auth: Authorization: Bearer <OOMOL_CONNECT_ADMIN_TOKEN>
//   - POST /api/runtime-tokens {name, allowedConnections, allowedActions,
//     allowedProxies, blockedActions} -> {token, record:{id,...}}
//     (token shown exactly once; grant fields are exactly those four
//     camelCase arrays, no blockedProxies)
//   - PUT /api/runtime-tokens/:id must carry ALL FOUR arrays — which is why
//     revocation never uses PUT: an empty allowedConnections list means
//     ALLOW-ALL upstream. Revocation is DELETE /api/runtime-tokens/:id
//     -> {id, revoked:true}; 404 = runtime_token_not_found.
//   - POST /api/oauth/authorizations {service, connectionName,...}
//     -> {authorizationUrl, state}
//   - GET /v1/connections/by-id/:appId -> connection with wire fields
//     alias / providerAccountId (admin scope).
//
// The admin API error shape is {error:{code,message}} (NOT the /v1 runtime
// envelope). No DELETE endpoint for connections is pinned by the T01
// evidence, so DeleteRuntimeConnection fails closed with
// ErrUnpinnedEndpoint instead of inventing a wire call.

const (
	// AdminDefaultTimeout bounds one admin call. It must stay below the
	// worker's default 30s lease so a slow-but-healthy call still completes
	// inside its own lease.
	AdminDefaultTimeout = 20 * time.Second
	// AdminMaxResponseBytes caps admin response bodies.
	AdminMaxResponseBytes = 2 << 20 // 2 MiB
)

// DefaultInternalAllowlist is the static set of internal runtime addresses
// the admin client may dial in a standard deployment. Public URLs are never
// allowed: the runtime lives on the internal network, and the admin client
// must not become an SSRF hop.
var DefaultInternalAllowlist = []string{
	"http://open-connector:8080",
	"http://127.0.0.1:8080",
	"http://localhost:8080",
}

// Admin client errors. Messages are static on purpose: the admin secret,
// token material, request bodies and response bodies must never appear in
// error text (see TestAdminClientErrorsCarryNoSecrets).
var (
	// ErrInvalidRuntimeAddress: base URL is not a bare http(s) origin.
	ErrInvalidRuntimeAddress = errors.New("connectorcontrol: invalid runtime address")
	// ErrRuntimeAddressNotAllowed: address is not in the static internal
	// allowlist.
	ErrRuntimeAddressNotAllowed = errors.New("connectorcontrol: runtime address not in internal allowlist")
	// ErrAdminSecretRequired: the client was built without a secret source.
	ErrAdminSecretRequired = errors.New("connectorcontrol: admin secret source required")
	// ErrAdminResponseTooLarge: response body exceeded the cap.
	ErrAdminResponseTooLarge = errors.New("connectorcontrol: admin response exceeds 2 MiB cap")
	// ErrMalformedAdminResponse: 2xx body did not match the frozen shape.
	ErrMalformedAdminResponse = errors.New("connectorcontrol: malformed admin response")
	// ErrUnpinnedEndpoint: the T01 contract does not pin this endpoint, so
	// the client refuses to invent the wire call (fail closed).
	ErrUnpinnedEndpoint = errors.New("connectorcontrol: endpoint not pinned by the runtime contract")
)

// AdminError is a non-2xx admin response. Only the status and the upstream
// error code are carried — response messages are deliberately dropped from
// Error() so nothing upstream echoes can leak into logs.
type AdminError struct {
	Status int
	Code   string
}

func (e *AdminError) Error() string {
	return fmt.Sprintf("connectorcontrol: admin api rejected: status %d code %q", e.Status, e.Code)
}

// IsNotFound reports whether err is an admin 404 (idempotent success for
// revocation).
func IsNotFound(err error) bool {
	var ae *AdminError
	return errors.As(err, &ae) && ae.Status == http.StatusNotFound
}

// CreateTokenRequest mints one per-connection scoped runtime token.
type CreateTokenRequest struct {
	Name       string
	ExternalID string
	Actions    []string
}

// RuntimeTokenCreated is the one-time result of a mint. Token is the raw
// oct_ material — it exists ONLY in this struct and must go straight into a
// SecretSink; it is never logged, never returned in API responses, never
// stored in the database. TokenRecordID is the safe external id used for
// later revocation.
type RuntimeTokenCreated struct {
	TokenRecordID string
	Token         string
}

// AuthorizationStart is an externally-started OAuth authorization.
type AuthorizationStart struct {
	URL   string
	State string
}

// RuntimeConnection is one external connection resolved by stable id.
type RuntimeConnection struct {
	ID                string
	Alias             string
	ProviderAccountID string
	Provider          string
}

// AdminClientConfig configures the runtime admin client.
type AdminClientConfig struct {
	// BaseURL is the runtime's internal address; it must equal one allowlist
	// entry (bare origin, no path/query/userinfo).
	BaseURL string
	// Allowlist is the static internal address set; nil means
	// DefaultInternalAllowlist.
	Allowlist []string
	// AdminSecret supplies the admin bearer credential per call.
	AdminSecret AdminSecret
	// HTTPClient is used for the single attempt (nil: default client).
	HTTPClient *http.Client
	// Timeout per attempt (0: AdminDefaultTimeout).
	Timeout time.Duration
}

// RuntimeAdminClient talks to one open-connector runtime's admin surface. It
// is safe for concurrent use. Every request: exactly one attempt, redirects
// never followed, secrets never in errors, response bodies capped.
type RuntimeAdminClient struct {
	base        string
	allowlist   []string
	adminSecret AdminSecret
	http        *http.Client
	timeout     time.Duration
}

// normalizeAdminBase validates a bare http(s) origin and returns it in
// scheme://host form. No userinfo, path, query or fragment is accepted.
func normalizeAdminBase(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", ErrInvalidRuntimeAddress
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" ||
		u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.RawFragment != "" ||
		(u.Path != "" && u.Path != "/") {
		return "", ErrInvalidRuntimeAddress
	}
	return u.Scheme + "://" + u.Host, nil
}

// NewRuntimeAdminClient validates the runtime address against the static
// internal allowlist and wires the fail-closed secret source.
func NewRuntimeAdminClient(cfg AdminClientConfig) (*RuntimeAdminClient, error) {
	if cfg.AdminSecret == nil {
		return nil, ErrAdminSecretRequired
	}
	base, err := normalizeAdminBase(cfg.BaseURL)
	if err != nil {
		return nil, err
	}
	allowlist := cfg.Allowlist
	if len(allowlist) == 0 {
		allowlist = DefaultInternalAllowlist
	}
	allowed := false
	for _, entry := range allowlist {
		normalized, err := normalizeAdminBase(entry)
		if err != nil {
			continue
		}
		if normalized == base {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil, ErrRuntimeAddressNotAllowed
	}
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{}
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = AdminDefaultTimeout
	}
	return &RuntimeAdminClient{base: base, allowlist: allowlist, adminSecret: cfg.AdminSecret, http: hc, timeout: timeout}, nil
}

// Address returns the (allowlisted) base address — safe to log: it is an
// internal static address by construction.
func (c *RuntimeAdminClient) Address() string { return c.base }

// do performs exactly ONE admin HTTP attempt. Redirects are never followed
// (a redirect would re-send the Authorization header to another origin).
// Fail-closed ordering: the secret is resolved before any network I/O, so a
// missing mount costs zero HTTP calls.
func (c *RuntimeAdminClient) do(ctx context.Context, method, path string, body []byte, out interface{}) error {
	secret, err := c.adminSecret(ctx)
	if err != nil || secret == "" {
		return ErrAdminSecretUnavailable
	}
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	reqCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, method, c.base+path, reader)
	if err != nil {
		return fmt.Errorf("connectorcontrol: build admin request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+secret)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	// Per-request shallow copy: never mutate the shared client, never follow
	// redirects.
	noRedirect := *c.http
	noRedirect.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	resp, err := noRedirect.Do(req)
	if err != nil {
		return fmt.Errorf("connectorcontrol: admin transport: %w", err)
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(resp.Body, AdminMaxResponseBytes+1))
	if err != nil {
		return fmt.Errorf("connectorcontrol: read admin response: %w", err)
	}
	if len(payload) > AdminMaxResponseBytes {
		return ErrAdminResponseTooLarge
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var env struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		_ = json.Unmarshal(payload, &env)
		return &AdminError{Status: resp.StatusCode, Code: env.Error.Code}
	}
	if out != nil {
		if err := json.Unmarshal(payload, out); err != nil {
			return ErrMalformedAdminResponse
		}
	}
	return nil
}

// CreateRuntimeToken mints a scoped runtime token. The grant is built with
// T02's NewGrant semantics: a blank connection id or an empty action list is
// rejected LOCALLY (before any HTTP) because upstream treats an empty list as
// ALLOW-ALL — an empty grant may never stand in for deny-all. The request
// body carries exactly the four frozen camelCase arrays.
func (c *RuntimeAdminClient) CreateRuntimeToken(ctx context.Context, req CreateTokenRequest) (RuntimeTokenCreated, error) {
	if strings.TrimSpace(req.Name) == "" {
		return RuntimeTokenCreated{}, permanentf("token name required")
	}
	grant, err := openconnector.NewGrant(req.ExternalID, req.Actions)
	if err != nil {
		return RuntimeTokenCreated{}, permanentf("token grant rejected: %v", err)
	}
	body, err := json.Marshal(struct {
		Name string `json:"name"`
		openconnector.Grant
	}{Name: req.Name, Grant: grant})
	if err != nil {
		return RuntimeTokenCreated{}, err
	}
	var resp struct {
		Token  string `json:"token"`
		Record struct {
			ID string `json:"id"`
		} `json:"record"`
	}
	if err := c.do(ctx, http.MethodPost, "/api/runtime-tokens", body, &resp); err != nil {
		return RuntimeTokenCreated{}, err
	}
	if resp.Token == "" || resp.Record.ID == "" {
		return RuntimeTokenCreated{}, ErrMalformedAdminResponse
	}
	return RuntimeTokenCreated{TokenRecordID: resp.Record.ID, Token: resp.Token}, nil
}

// RevokeRuntimeToken revokes a token by its record id using DELETE — never a
// PUT with allowedConnections=[] (an empty allowlist re-opens access
// upstream). A 404 is idempotent success: the token is already gone.
func (c *RuntimeAdminClient) RevokeRuntimeToken(ctx context.Context, tokenRecordID string) error {
	if strings.TrimSpace(tokenRecordID) == "" {
		return permanentf("token record id required")
	}
	err := c.do(ctx, http.MethodDelete, "/api/runtime-tokens/"+url.PathEscape(tokenRecordID), nil, nil)
	if IsNotFound(err) {
		return nil
	}
	return err
}

// StartAuthorization starts an external OAuth authorization for one service
// and connection name (alias).
func (c *RuntimeAdminClient) StartAuthorization(ctx context.Context, service, connectionName string) (AuthorizationStart, error) {
	if strings.TrimSpace(service) == "" || strings.TrimSpace(connectionName) == "" {
		return AuthorizationStart{}, permanentf("service and connection name required")
	}
	body, err := json.Marshal(struct {
		Service        string `json:"service"`
		ConnectionName string `json:"connectionName"`
	}{Service: service, ConnectionName: connectionName})
	if err != nil {
		return AuthorizationStart{}, err
	}
	var resp struct {
		URL   string `json:"authorizationUrl"`
		State string `json:"state"`
	}
	if err := c.do(ctx, http.MethodPost, "/api/oauth/authorizations", body, &resp); err != nil {
		return AuthorizationStart{}, err
	}
	if resp.URL == "" || resp.State == "" {
		return AuthorizationStart{}, ErrMalformedAdminResponse
	}
	return AuthorizationStart{URL: resp.URL, State: resp.State}, nil
}

// LookupRuntimeConnection resolves one external connection by its stable id
// (admin GET /v1/connections/by-id/:appId).
func (c *RuntimeAdminClient) LookupRuntimeConnection(ctx context.Context, appID string) (RuntimeConnection, error) {
	if strings.TrimSpace(appID) == "" {
		return RuntimeConnection{}, permanentf("external connection id required")
	}
	var resp struct {
		ID                string `json:"id"`
		Alias             string `json:"alias"`
		ProviderAccountID string `json:"providerAccountId"`
		Provider          string `json:"provider"`
	}
	if err := c.do(ctx, http.MethodGet, "/v1/connections/by-id/"+url.PathEscape(appID), nil, &resp); err != nil {
		return RuntimeConnection{}, err
	}
	if resp.ID == "" {
		return RuntimeConnection{}, ErrMalformedAdminResponse
	}
	return RuntimeConnection{ID: resp.ID, Alias: resp.Alias, ProviderAccountID: resp.ProviderAccountID, Provider: resp.Provider}, nil
}

// DeleteRuntimeConnection fails closed: the T01 contract evidence pins no
// connection-deletion endpoint, so the client refuses to invent the wire
// call (returning ErrUnpinnedEndpoint keeps the operation queued for retry
// instead of silently dropping a revocation).
func (c *RuntimeAdminClient) DeleteRuntimeConnection(ctx context.Context, appID string) error {
	if strings.TrimSpace(appID) == "" {
		return permanentf("external connection id required")
	}
	return ErrUnpinnedEndpoint
}

// ParseAllowlist splits a comma-separated address list, dropping blanks.
func ParseAllowlist(csv string) []string {
	if strings.TrimSpace(csv) == "" {
		return nil
	}
	parts := strings.Split(csv, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// defaultJitter is the production jitter source for backoff (fraction in
// [0,1)).
func defaultJitter() float64 { return rand.Float64() }
