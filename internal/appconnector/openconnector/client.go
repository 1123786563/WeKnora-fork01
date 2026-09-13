package openconnector

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	// DefaultTimeout bounds one Execute call. It is applied per request via
	// context.WithTimeout, so it is context-aware: an earlier deadline on the
	// caller's context always wins.
	DefaultTimeout = 30 * time.Second

	// MaxResponseBytes caps how much of a response body is read. One byte
	// beyond the cap aborts the call as a protocol failure.
	MaxResponseBytes = 2 << 20 // 2 MiB
)

// actionIDPattern is the closed charset allowed in POST /v1/actions/{actionId}
// path segments (no spaces, separators or glob syntax).
var actionIDPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

// Validation and protocol errors. Messages are static on purpose: tokens,
// idempotency keys, raw input and response bodies must never appear in error
// text (see TestClientErrorsCarryNoSecrets).
var (
	// ErrInvalidBaseURL: the base URL is not a bare http(s) origin (userinfo,
	// query or fragment present, or another scheme).
	ErrInvalidBaseURL = errors.New("openconnector: invalid base URL")
	// ErrAliasRequired: the connection alias is omitted or blank.
	ErrAliasRequired = errors.New("openconnector: connection alias required")
	// ErrTokenRequired: the scoped token is empty.
	ErrTokenRequired = errors.New("openconnector: token required")
	// ErrInvalidActionID: the action id does not match ^[A-Za-z0-9_.-]+$.
	ErrInvalidActionID = errors.New("openconnector: invalid action id")
	// ErrInvalidKey: the idempotency key is blank or longer than 255 bytes.
	ErrInvalidKey = errors.New("openconnector: invalid idempotency key")
	// ErrInvalidInput: the call input is not valid JSON.
	ErrInvalidInput = errors.New("openconnector: input must be valid JSON")
	// ErrResponseTooLarge: the response body exceeded MaxResponseBytes.
	ErrResponseTooLarge = errors.New("openconnector: response exceeds 2 MiB cap")
	// ErrMalformedEnvelope: the response body is not the frozen /v1 JSON
	// envelope.
	ErrMalformedEnvelope = errors.New("openconnector: malformed response envelope")
)

// envelope mirrors the FROZEN /v1 wire envelopes (contract doc §3.3). The
// error code field is errorCode (NOT code); executionId / actionId /
// auditPersisted are nested inside meta (NOT at the top level).
type envelope struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
	Code    string          `json:"errorCode"`
	Meta    envelopeMeta    `json:"meta"`
}

type envelopeMeta struct {
	ExecutionID    string `json:"executionId"`
	ActionID       string `json:"actionId"`
	AuditPersisted *bool  `json:"auditPersisted"`
}

// Client is a scoped HTTP executor for one open-connector deployment root. It
// is safe for concurrent use; it holds no credentials (every Execute receives
// the token for exactly one connection).
type Client struct {
	base    string
	http    *http.Client
	timeout time.Duration
}

// NewClient validates baseURL — it must be a bare http(s) origin (optionally
// with a path prefix) carrying no userinfo, query or fragment — and returns a
// client that will dial through hc (a nil hc gets a default *http.Client).
func NewClient(baseURL string, hc *http.Client) (*Client, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("%w: unparsable", ErrInvalidBaseURL)
	}
	// u.ForceQuery models a bare trailing "?" (url.Parse keeps it and
	// u.String() preserves it): accepting such a base would turn every action
	// path into a rawQuery and POST to the root, so it is rejected as "has
	// query" per the frozen pre-network validation rules.
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" ||
		u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.RawFragment != "" {
		return nil, ErrInvalidBaseURL
	}
	if hc == nil {
		hc = &http.Client{}
	}
	return &Client{base: u.String(), http: hc, timeout: DefaultTimeout}, nil
}

// validateCall enforces every input rule BEFORE any network I/O; a violation
// must cost zero HTTP calls.
func validateCall(token string, call Call) error {
	if strings.TrimSpace(call.Alias) == "" {
		return ErrAliasRequired
	}
	if token == "" {
		return ErrTokenRequired
	}
	if !actionIDPattern.MatchString(call.ActionID) {
		return ErrInvalidActionID
	}
	if key := call.Key; strings.TrimSpace(key) == "" || len(key) > 255 {
		return ErrInvalidKey
	}
	if !json.Valid(call.Input) {
		return ErrInvalidInput
	}
	return nil
}

func (c *Client) actionURL(actionID string) string {
	return strings.TrimSuffix(c.base, "/") + "/v1/actions/" + actionID
}

// Execute performs exactly one POST /v1/actions/{ActionID}. It never retries
// and never follows redirects (a redirect would re-send the Authorization
// header and body to another origin); a non-2xx response is still decoded and
// returned raw in Result with a nil error — only transport/protocol failures
// (dial, timeout, context cancel, oversize or malformed body) return errors.
//
// The request body is json.Marshal of {"input": <raw>} exactly; the three
// contract headers are Authorization, Idempotency-Key and x-oo-connector-alias
// (plus Content-Type, which is protocol, not user input).
func (c *Client) Execute(ctx context.Context, token string, call Call) (Result, error) {
	if err := validateCall(token, call); err != nil {
		return Result{}, err
	}
	payload, err := json.Marshal(struct {
		Input json.RawMessage `json:"input"`
	}{call.Input})
	if err != nil {
		// Unreachable after json.Valid: kept as a guard, message stays static.
		return Result{}, ErrInvalidInput
	}

	reqCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, c.actionURL(call.ActionID), bytes.NewReader(payload))
	if err != nil {
		return Result{}, fmt.Errorf("openconnector: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Idempotency-Key", call.Key)
	req.Header.Set("x-oo-connector-alias", call.Alias)

	// Per-request shallow copy: the redirect policy must never follow, and the
	// caller's *http.Client must not be mutated.
	noRedirect := *c.http
	noRedirect.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	resp, err := noRedirect.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("openconnector: transport: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, MaxResponseBytes+1))
	if err != nil {
		return Result{HTTPStatus: resp.StatusCode}, fmt.Errorf("openconnector: read response: %w", err)
	}
	if len(body) > MaxResponseBytes {
		return Result{HTTPStatus: resp.StatusCode}, ErrResponseTooLarge
	}
	var env envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return Result{HTTPStatus: resp.StatusCode}, ErrMalformedEnvelope
	}
	return Result{
		HTTPStatus:     resp.StatusCode,
		Success:        env.Success,
		Code:           env.Code,
		Message:        env.Message,
		Data:           env.Data,
		ActionID:       env.Meta.ActionID,
		ExecutionID:    env.Meta.ExecutionID,
		AuditPersisted: env.Meta.AuditPersisted,
	}, nil
}
