// Package connectorcontrol implements the open-connector control worker: the
// ONLY process that holds the runtime admin credential and performs admin
// surface operations (runtime token mint/revoke, external authorization,
// external connection lookup). Ordinary API processes enqueue ControlOperation
// rows into the local connector_operations_outbox and never import this
// package's admin client.
//
// Concurrency model (plan T05, adapted to the FROZEN T03 schema — see
// oc_store.go): an operation is executed by at most one worker at a time.
// Claiming bumps fence and holds the lease by pushing next_at; a worker whose
// lease expired may be taken over at fence+1, after which the stale worker's
// Complete matches ZERO rows. Default lease is 30s, renewed every 10s while an
// operation executes; failures retry with exponential backoff capped at 5
// minutes plus jitter.
package connectorcontrol

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/logger"
	appconn "github.com/Tencent/WeKnora/internal/modules/appconnector"
	repoapp "github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Control operation kinds. The closed set is deliberate: generic HTTP verbs
// are NOT control operations — ValidateControlOperation rejects anything
// outside this list before the worker ever builds an admin request.
const (
	KindAuthorize        = "authorize"         // start an external OAuth authorization (wired by T07)
	KindCreateToken      = "create_token"      // mint a per-connection runtime token
	KindDeleteToken      = "delete_token"      // revoke a runtime token by record id
	KindDeleteConnection = "delete_connection" // remove the external connection
	KindConfirm          = "confirm"           // confirm/correlate an external connection (wired by T07)
)

// ControlOperation is one deferred admin-surface operation. The frozen
// outbox columns resource_id / resource_version map EXPLICITLY to
// ConnectionID / AuthVersion here (coordinator ruling 1): the "resource" a
// control operation acts on is always a tenant connection, and the version is
// that connection's authorization generation.
type ControlOperation struct {
	ID           string
	TenantID     uint64
	ConnectionID string
	Kind         string
	AuthVersion  int64
	Payload      json.RawMessage
}

// Validation errors (plan T05 sketch, verbatim semantics).
var (
	// ErrInvalidControlOperation: missing ID/TenantID/ConnectionID or
	// AuthVersion < 1.
	ErrInvalidControlOperation = errors.New("invalid control operation")
	// ErrUnsupportedControlKind: kind outside the closed set (in particular
	// any generic HTTP kind).
	ErrUnsupportedControlKind = errors.New("unsupported control operation")
	// ErrAdminSecretUnavailable: the dedicated admin secret mount is absent
	// or unreadable — admin operations fail CLOSED.
	ErrAdminSecretUnavailable = errors.New("connectorcontrol: admin secret unavailable")
	// ErrKindNotWiredYet: historical placeholder error kept for import
	// compatibility - authorize/confirm correlation is wired since T07
	// (payload-less rows of payload-requiring kinds are permanent
	// rejections instead; unavailable correlation endpoints surface
	// ErrCorrelationUnavailable).
	ErrKindNotWiredYet = errors.New("connectorcontrol: control kind not wired yet")
)

// errPermanent marks operation errors that retrying can never fix (malformed
// payload, empty grant). Permanent errors DROP the outbox row; everything
// else retries with capped backoff.
var errPermanent = errors.New("connectorcontrol: permanent operation rejection")

func permanentf(format string, args ...interface{}) error {
	return fmt.Errorf("%w: %s", errPermanent, fmt.Sprintf(format, args...))
}

func isPermanentError(err error) bool {
	return err != nil && errors.Is(err, errPermanent)
}

// ValidateControlOperation enforces the closed control-operation contract
// BEFORE any admin HTTP is attempted: an operation must be tenant-scoped with
// identity fields present, target a positive authorization version, and use a
// kind from the frozen list. Everything else — in particular generic HTTP
// operation kinds — is rejected.
func ValidateControlOperation(op ControlOperation) error {
	if op.ID == "" || op.TenantID == 0 || op.ConnectionID == "" || op.AuthVersion < 1 {
		return ErrInvalidControlOperation
	}
	switch op.Kind {
	case KindAuthorize, KindCreateToken, KindDeleteToken, KindDeleteConnection, KindConfirm:
		return nil
	default:
		return ErrUnsupportedControlKind
	}
}

// OperationFromRow maps a claimed outbox row to a ControlOperation. Field
// mapping (frozen schema -> control plane): resource_id -> ConnectionID,
// resource_version -> AuthVersion. Payload is nil: the frozen outbox schema
// has NO payload column — payload-carrying enqueue paths are introduced by
// later tasks; until then payload-bearing executions are driven explicitly
// and a payload-less operation of a payload-requiring kind is a permanent
// rejection (never a mint with an empty grant).
func OperationFromRow(row *repoapp.OCOperationsOutboxRow) ControlOperation {
	return ControlOperation{
		ID:           row.ID,
		TenantID:     row.TenantID,
		ConnectionID: row.ResourceID,
		Kind:         row.Kind,
		AuthVersion:  row.ResourceVersion,
	}
}

// AdminSecret loads the runtime admin credential. The only production
// implementation is FileAdminSecretSource (dedicated mount path); it returns
// an error — never an empty token — when the mount is absent, so an
// API-process-style environment without the mount can never perform admin
// operations. The secret value must never be logged or embedded in errors.
type AdminSecret func(ctx context.Context) (string, error)

// SecretSink persists credential material under an opaque reference. The
// database only ever stores the reference (T03 schema rule: credentials exist
// as secret REFERENCES only).
type SecretSink interface {
	PutSecret(ctx context.Context, ref, secret string) error
}

// OutboxStore is the claim/complete/extend contract the worker needs from the
// T03 outbox (implemented by appconnector.OCStore with real transactions and
// fence predicates).
type OutboxStore interface {
	// ClaimNextOperation atomically leases the earliest due operation.
	ClaimNextOperation(ctx context.Context, owner string, now time.Time, lease time.Duration) (*repoapp.OCOperationsOutboxRow, error)
	// CompleteOperation removes a finished operation; false = the caller no
	// longer holds the lease (ZERO rows affected).
	CompleteOperation(ctx context.Context, id, owner string, fence int64) (bool, error)
	// ExtendOperation moves the lease deadline (renewal) or schedules the
	// next retry (backoff) under the lease_owner+fence predicate.
	ExtendOperation(ctx context.Context, id, owner string, fence int64, until, now time.Time) (bool, error)
}

// RuntimeAdmin is the admin surface of one open-connector runtime. Every
// method performs AT MOST ONE HTTP attempt, never follows redirects, and is
// authenticated with the isolated admin secret.
type RuntimeAdmin interface {
	// CreateRuntimeToken mints a per-connection scoped token (empty
	// connection/action grants are rejected before any HTTP — upstream
	// treats [] as ALLOW-ALL, so an empty grant may never stand for deny).
	CreateRuntimeToken(ctx context.Context, req CreateTokenRequest) (RuntimeTokenCreated, error)
	// RevokeRuntimeToken revokes a token by record id via DELETE. Revoking
	// the last authorization DELETES the token — never a PUT with
	// allowedConnections=[] (writing an empty list RE-OPENS access).
	RevokeRuntimeToken(ctx context.Context, tokenRecordID string) error
	// StartAuthorization starts an external OAuth authorization.
	StartAuthorization(ctx context.Context, service, connectionName string) (AuthorizationStart, error)
	// LookupRuntimeConnection resolves an external connection by stable id.
	LookupRuntimeConnection(ctx context.Context, appID string) (RuntimeConnection, error)
	// DeleteRuntimeConnection removes the external connection.
	DeleteRuntimeConnection(ctx context.Context, appID string) error
}

// WorkerConfig bounds the claim lease, renewal cadence and retry backoff.
// Lease must exceed the admin client timeout so a healthy execution
// completes inside its own lease.
type WorkerConfig struct {
	OwnerID       string
	Lease         time.Duration // default 30s
	RenewInterval time.Duration // default 10s
	BackoffBase   time.Duration // default 2s
	BackoffMax    time.Duration // default 5m
	PollInterval  time.Duration // default 500ms (used by RunForever)
}

// DefaultWorkerConfig returns the plan-mandated timing: 30s lease, 10s
// renewal, exponential backoff capped at 5 minutes (jitter is added per
// retry).
func DefaultWorkerConfig(owner string) WorkerConfig {
	return WorkerConfig{
		OwnerID:       owner,
		Lease:         30 * time.Second,
		RenewInterval: 10 * time.Second,
		BackoffBase:   2 * time.Second,
		BackoffMax:    5 * time.Minute,
		PollInterval:  500 * time.Millisecond,
	}
}

// ControlStore is the persistence surface the worker needs beyond the
// outbox claim/complete/extend contract: tenant bindings plus the T07
// authorization-attempt correlation rows. *repoapp.OCStore satisfies it; the
// widened parameter keeps NewControlWorker's arity unchanged (coordinator
// R13.3) - every existing caller passes the concrete store.
type ControlStore interface {
	appconn.OCBindingStore
	GetOCAttempt(ctx context.Context, tenant uint64, id string) (appconn.OCAuthorizationAttempt, error)
	AdvanceOCAttempt(ctx context.Context, tenant uint64, id, from, to string, now time.Time) (bool, error)
	TerminateOCAttempt(ctx context.Context, tenant uint64, id, to string, now time.Time) (bool, error)
	AdoptOCAttemptAlias(ctx context.Context, tenant uint64, id, from, to string, now time.Time) (bool, error)
	ActivateOCAttempt(ctx context.Context, tenant uint64, id, externalID string, now time.Time) error
	CleanupFailedOCAttempt(ctx context.Context, tenant uint64, attemptID, alias, externalID string, now time.Time) error
	EnqueueOCOperation(ctx context.Context, id string, tenant uint64, resourceID string, resourceVersion int64, kind string, nextAt time.Time) error
}

// ControlWorker drains the operations outbox exactly one operation per tick.
type ControlWorker struct {
	cfg           WorkerConfig
	outbox        OutboxStore
	store         ControlStore
	admin         RuntimeAdmin
	secrets       SecretSink
	secret        AdminSecret
	correlator    RuntimeCorrelator // optional override; default derives from admin
	correlatorSet bool
	transient     appconnectorsvc.TransientCipher // optional override; default derives from env
	transientSet  bool
	now           func() time.Time
	jitter        func() float64 // [0,1); injected for deterministic tests
}

// NewControlWorker validates its dependencies up front: every collaborator
// must be present (fail-closed wiring, no silent no-ops).
func NewControlWorker(outbox OutboxStore, store ControlStore, admin RuntimeAdmin, sink SecretSink, secret AdminSecret, cfg WorkerConfig) (*ControlWorker, error) {
	if outbox == nil || store == nil || admin == nil || sink == nil || secret == nil {
		return nil, errors.New("connectorcontrol: worker dependencies must all be non-nil")
	}
	if cfg.OwnerID == "" || cfg.Lease <= 0 || cfg.RenewInterval <= 0 || cfg.BackoffBase <= 0 || cfg.BackoffMax < cfg.BackoffBase {
		return nil, errors.New("connectorcontrol: invalid worker config")
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 500 * time.Millisecond
	}
	return &ControlWorker{
		cfg:     cfg,
		outbox:  outbox,
		store:   store,
		admin:   admin,
		secrets: sink,
		secret:  secret,
		now:     time.Now,
		jitter:  defaultJitter,
	}, nil
}

// RunOnce claims at most one due operation and drives it to a terminal local
// state: completed, dropped (permanent rejection), or scheduled for retry.
// It returns nil when the queue is idle.
func (w *ControlWorker) RunOnce(ctx context.Context) error {
	row, err := w.outbox.ClaimNextOperation(ctx, w.cfg.OwnerID, w.now().UTC(), w.cfg.Lease)
	if err != nil {
		return fmt.Errorf("connectorcontrol: claim: %w", err)
	}
	if row == nil {
		return nil
	}
	return w.runClaimed(ctx, row, OperationFromRow(row))
}

// runClaimed executes one already-leased row. Tests may drive it directly
// with a synthetic ControlOperation (the frozen outbox carries no payload
// column; payload-bearing enqueue lands with T07).
func (w *ControlWorker) runClaimed(ctx context.Context, row *repoapp.OCOperationsOutboxRow, op ControlOperation) error {
	done := make(chan struct{})
	go w.renewLoop(ctx, row, done)
	defer close(done)

	// 1. Closed-set validation BEFORE any admin API touch.
	if err := ValidateControlOperation(op); err != nil {
		w.dropOperation(ctx, row, op, err)
		return nil
	}

	// 2. Freshness gates. create_token re-checks the tenant binding (it must
	// exist, be live, and sit at exactly the operation's auth generation - a
	// deactivated connection can never be reactivated by a stale operation).
	// authorize/confirm gate on the ATTEMPT instead: the binding does not
	// exist yet, because activation is what creates it. The attempt must
	// belong to the operation's tenant (forgery reads as not-found), sit at
	// the operation's authorization generation, be inside its 15-minute
	// window, and be in a state the kind may still act on; failed/expired/
	// revoked/active attempts never revive.
	var binding *appconn.OCBinding
	var attempt *appconn.OCAuthorizationAttempt
	switch op.Kind {
	case KindCreateToken:
		b, err := w.store.GetBinding(ctx, op.TenantID, op.ConnectionID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				w.dropOperation(ctx, row, op, errors.New("binding not found"))
				return nil
			}
			return fmt.Errorf("connectorcontrol: load binding for %s: %w", op.ID, err)
		}
		if b.State != appconn.OCBindingPending && b.State != appconn.OCBindingActive {
			w.dropOperation(ctx, row, op, fmt.Errorf("binding state %s", b.State))
			return nil
		}
		if b.AuthVersion != op.AuthVersion {
			w.dropOperation(ctx, row, op, fmt.Errorf("stale auth version %d (binding at %d)", op.AuthVersion, b.AuthVersion))
			return nil
		}
		binding = &b
	case KindAuthorize, KindConfirm:
		a, err := w.store.GetOCAttempt(ctx, op.TenantID, attemptIDFromOperation(op))
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				w.dropOperation(ctx, row, op, errors.New("authorization attempt not found"))
				return nil
			}
			return fmt.Errorf("connectorcontrol: load attempt for %s: %w", op.ID, err)
		}
		gateNow := w.now().UTC()
		switch {
		case a.AuthVersion != op.AuthVersion:
			w.dropOperation(ctx, row, op, fmt.Errorf("stale attempt version %d (attempt at %d)", op.AuthVersion, a.AuthVersion))
			return nil
		case !gateNow.Before(a.ExpiresAt):
			// The 15-minute window closed: mark the attempt expired, then
			// drop. Dropping deletes the outbox row, which for an API-key
			// handoff also deletes the transient ciphertext (ruling 7: the
			// failed blob survives at most 15 minutes).
			_, _ = w.store.TerminateOCAttempt(ctx, op.TenantID, a.ID, appconnectorsvc.OCAttemptExpired, gateNow)
			w.dropOperation(ctx, row, op, errors.New("authorization attempt expired"))
			return nil
		case op.Kind == KindAuthorize && a.State == appconnectorsvc.OCAttemptPending:
			// fresh start
		case op.Kind == KindAuthorize && a.State == appconnectorsvc.OCAttemptAuthorizing:
			// idempotent re-run after a partially-failed start
		case op.Kind == KindConfirm && a.State == appconnectorsvc.OCAttemptVerifying:
			// exactly the consumable state
		case op.Kind == KindAuthorize && a.State == appconnectorsvc.OCAttemptVerifying && sealedSegment(op) != "":
			// API-key handoff that already reached verification (T08 ruling
			// 1a / T07 Q-3): the external connection exists under the ADOPTED
			// alias, so the retry must re-drive the completion tail instead of
			// permanently dropping the row and orphaning the attempt plus the
			// external connection. OAuth rows keep the pinned drop semantics —
			// their verifying-state driver is the confirm row, not this one.
		default:
			w.dropOperation(ctx, row, op, fmt.Errorf("attempt state %s cannot process %s", a.State, op.Kind))
			return nil
		}
		attempt = &a
	}

	// 3. Admin credential gate: fail CLOSED. Without the dedicated mount the
	// worker refuses admin operations entirely — the operation stays queued
	// and is retried once the mount appears.
	if _, err := w.secret(ctx); err != nil {
		w.retryOperation(ctx, row, op)
		return fmt.Errorf("%w: operation %s", ErrAdminSecretUnavailable, op.ID)
	}

	// 4. Execute (single attempt per admin call, no redirects).
	execErr := w.execute(ctx, op, binding, attempt)
	if execErr != nil {
		if isPermanentError(execErr) {
			w.dropOperation(ctx, row, op, execErr)
			return nil
		}
		w.retryOperation(ctx, row, op)
		return fmt.Errorf("connectorcontrol: operation %s (%s): %w", op.ID, op.Kind, execErr)
	}

	// 5. Complete under the fence. false means a takeover happened after our
	// lease expired: the operation is now owned by a higher fence and will
	// re-execute there; we must not touch the row again.
	ok, err := w.outbox.CompleteOperation(ctx, row.ID, row.LeaseOwner, row.Fence)
	if err != nil {
		return fmt.Errorf("connectorcontrol: complete %s: %w", op.ID, err)
	}
	if !ok {
		logger.Warnf(ctx, "[connector-control] operation %s lease lost before completion; superseded owner takes over", op.ID)
	}
	return nil
}

// execute dispatches one validated operation to the admin client. The
// authorize/confirm kinds deliberately hold the operation (transient error)
// until T07 wires attempt/alias correlation — starting an external OAuth flow
// whose state nobody can complete, or confirming without correlation, would
// be worse than waiting.
func (w *ControlWorker) execute(ctx context.Context, op ControlOperation, binding *appconn.OCBinding, attempt *appconn.OCAuthorizationAttempt) error {
	switch op.Kind {
	case KindCreateToken:
		var p createTokenPayload
		if err := decodePayload(op.Payload, &p); err != nil {
			return err
		}
		if len(p.Actions) == 0 {
			// "无连接或无允许动作时不签发 Token，绝不以空清单模拟拒绝全部"
			return permanentf("create_token without actions never mints (empty grant)")
		}
		name := p.TokenName
		if name == "" {
			name = fmt.Sprintf("weknora-%d-%s-v%d", op.TenantID, op.ConnectionID, op.AuthVersion)
		}
		// mintRuntimeToken persists the material under the deterministic
		// reference and reconciles a prior orphaned mint before re-casting
		// (T08 ruling 1d). Only the reference is ever logged, returned or
		// stored in the database.
		return w.mintRuntimeToken(ctx, op.TenantID, op.ConnectionID, op.AuthVersion, name, binding.ExternalID, p.Actions)
	case KindDeleteToken:
		var p deleteTokenPayload
		if err := decodePayload(op.Payload, &p); err != nil {
			return err
		}
		if p.TokenID == "" {
			return permanentf("delete_token without token id")
		}
		// A 404 is idempotent success (the token is already gone — e.g. a
		// delete whose success response was lost); anything else retries.
		if err := w.admin.RevokeRuntimeToken(ctx, p.TokenID); err != nil && !IsNotFound(err) {
			return err
		}
		return nil
	case KindDeleteConnection:
		var p deleteConnectionPayload
		if len(op.Payload) > 0 {
			if err := decodePayload(op.Payload, &p); err != nil {
				return err
			}
		}
		external := p.ExternalID
		// Cleanup rows (ruling 3a) carry the external id after "|" in
		// resource_id; ordinary revocation rows keep the connection id there
		// and resolve through the binding.
		if external == "" && strings.ContainsRune(op.ConnectionID, '|') {
			external = op.ConnectionID[strings.IndexByte(op.ConnectionID, '|')+1:]
		}
		if external == "" {
			b, err := w.store.GetBinding(ctx, op.TenantID, op.ConnectionID)
			if err == nil {
				external = b.ExternalID
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}
		if external == "" {
			return permanentf("delete_connection without a resolvable external id")
		}
		// A 404 is idempotent success (the external connection is already
		// gone — e.g. a delete whose success response was lost); anything
		// else retries so a revocation is never silently dropped.
		if err := w.admin.DeleteRuntimeConnection(ctx, external); err != nil && !IsNotFound(err) {
			return err
		}
		return nil
	case KindAuthorize:
		p, err := decodeAttemptPayload(op)
		if err != nil {
			return err
		}
		if sealed := sealedSegment(op); sealed != "" {
			// API-key path: the transient ciphertext rides resource_id
			// (ruling 3a); this worker is the only decryptor.
			return w.executeAPIKeyHandoff(ctx, op, attempt, sealed)
		}
		_ = p
		// OAuth path: start the external authorization under the attempt's
		// UUID alias via the pinned admin authorizations route (the alias is
		// frozen at initiation - upstream freezes the target there). The
		// upstream authorizationUrl/stateHandle are consumed by the T13/T14
		// HTTP surface; the worker persists only the state transition.
		if _, err := w.admin.StartAuthorization(ctx, attempt.Provider, attempt.Alias); err != nil {
			return err
		}
		if attempt.State == appconnectorsvc.OCAttemptPending {
			ok, err := w.store.AdvanceOCAttempt(ctx, op.TenantID, attempt.ID,
				appconnectorsvc.OCAttemptPending, appconnectorsvc.OCAttemptAuthorizing, w.now().UTC())
			if err != nil {
				// Remote start already succeeded under our alias; a retry
				// re-runs it idempotently, so a transient local failure is
				// retried, not dropped.
				return err
			}
			if !ok {
				return permanentf("attempt %s no longer awaiting authorization", attempt.ID)
			}
		}
		return nil
	case KindConfirm:
		p, err := decodeAttemptPayload(op)
		if err != nil {
			return err
		}
		return w.executeConfirm(ctx, op, attempt, p)
	default:
		return ErrUnsupportedControlKind
	}
}

// ---------------------------------------------------------------------------
// authorize/confirm execution (T07 correlation wiring)
// ---------------------------------------------------------------------------

// attemptPayload is the optional explicit payload for authorize/confirm
// operations driven directly (tests, later HTTP surfaces); enqueued rows
// carry the attempt id inside resource_id instead (ruling 3a).
type attemptPayload struct {
	AttemptID string   `json:"attemptID,omitempty"`
	Actions   []string `json:"actions,omitempty"`
}

// decodeAttemptPayload decodes the optional payload; absent payloads are
// legal (resource_id carries the correlation), malformed ones are permanent
// rejections (never a silently degraded execution).
func decodeAttemptPayload(op ControlOperation) (attemptPayload, error) {
	if len(op.Payload) == 0 {
		return attemptPayload{}, nil
	}
	if !json.Valid(op.Payload) {
		return attemptPayload{}, permanentf("malformed operation payload")
	}
	var p attemptPayload
	if err := json.Unmarshal(op.Payload, &p); err != nil {
		return attemptPayload{}, permanentf("malformed operation payload")
	}
	return p, nil
}

// resourceIDSegment returns the resource_id payload before the first "|":
// the attempt id for authorize/confirm rows and the attempt id of a cleanup
// delete row.
func resourceIDSegment(s string) string {
	if i := strings.IndexByte(s, '|'); i >= 0 {
		return s[:i]
	}
	return s
}

// attemptIDFromOperation resolves which attempt an operation targets: the
// explicit payload id when present, else the resource_id segment. A
// malformed payload yields a blank id, which the gate then treats as a
// not-found attempt (permanent drop).
func attemptIDFromOperation(op ControlOperation) string {
	p, err := decodeAttemptPayload(op)
	if err != nil || p.AttemptID == "" {
		return resourceIDSegment(op.ConnectionID)
	}
	return p.AttemptID
}

// sealedSegment returns the transient ciphertext carried by an API-key
// authorize row (after the "|"), or "" for OAuth rows.
func sealedSegment(op ControlOperation) string {
	if op.Kind != KindAuthorize {
		return ""
	}
	if i := strings.IndexByte(op.ConnectionID, '|'); i >= 0 {
		return op.ConnectionID[i+1:]
	}
	return ""
}

// ErrCorrelationUnavailable: the alias-to-external-connection correlation
// cannot run - the runtime's correlation endpoints are not available to this
// worker (unpinned/admin client without correlation support, or missing
// transient key material). Operations stay queued (fail-closed, mirroring
// ErrUnpinnedEndpoint); this is never a silent no-op.
var ErrCorrelationUnavailable = errors.New("connectorcontrol: correlation endpoints unavailable")

// correlationAdmin is the correlation surface the pinned RuntimeAdminClient
// grew in T07 (coordinator R13.5/R14 grant).
type correlationAdmin interface {
	ListRuntimeConnections(ctx context.Context) ([]RuntimeConnection, error)
	ConnectRuntimeAPIKey(ctx context.Context, service, apiKey string) (RuntimeConnection, error)
}

// RuntimeCorrelator bridges attempt aliases to external connections through
// the T01-verified correlation chain (ruling 4): the exact attempt alias
// resolves through the admin list endpoint (exact-match filter, no fallback
// to a "default" or newest connection), the stable id projects through the
// by-id lookup, and the API-key handoff submits the decrypted key to the
// runtime's api-key connect endpoint.
type RuntimeCorrelator interface {
	// ResolveExternalConnection returns the external connection the runtime
	// holds under EXACTLY this attempt alias.
	ResolveExternalConnection(ctx context.Context, service, alias string) (RuntimeConnection, error)
	// SubmitExternalCredential hands one transient provider API key to the
	// runtime and returns the immediately-created external connection.
	SubmitExternalCredential(ctx context.Context, service, alias, apiKey string) (RuntimeConnection, error)
}

// SetRuntimeCorrelator overrides the correlation implementation (tests, or a
// deployment with a bespoke bridge). The default derives from the wired
// admin client whenever it implements the pinned correlation endpoints.
func (w *ControlWorker) SetRuntimeCorrelator(rc RuntimeCorrelator) {
	w.correlator, w.correlatorSet = rc, true
}

// SetTransientCipher overrides the transient API-key cipher (tests).
func (w *ControlWorker) SetTransientCipher(c appconnectorsvc.TransientCipher) {
	w.transient, w.transientSet = c, true
}

func (w *ControlWorker) runtimeCorrelator() (RuntimeCorrelator, error) {
	if w.correlatorSet {
		if w.correlator == nil {
			return nil, ErrCorrelationUnavailable
		}
		return w.correlator, nil
	}
	if ca, ok := w.admin.(correlationAdmin); ok {
		return adminCorrelator{admin: ca}, nil
	}
	return nil, ErrCorrelationUnavailable
}

func (w *ControlWorker) transientCipher() (appconnectorsvc.TransientCipher, error) {
	if w.transientSet {
		if w.transient == nil {
			return nil, ErrCorrelationUnavailable
		}
		return w.transient, nil
	}
	c, err := appconnectorsvc.NewTransientCipherFromEnv()
	if err != nil {
		return nil, ErrCorrelationUnavailable
	}
	return c, nil
}

// adminCorrelator is the DEFAULT correlation implementation over the pinned
// admin client endpoints (R14: the real implementation is the default;
// fail-closed applies only when the endpoints are unavailable).
type adminCorrelator struct{ admin correlationAdmin }

// ResolveExternalConnection lists the runtime's managed connections and
// returns the one whose alias equals the attempt alias EXACTLY. No match is
// a not-found AdminError (the authorization has not completed upstream);
// multiple matches are an integrity violation and refuse to guess.
func (a adminCorrelator) ResolveExternalConnection(ctx context.Context, service, alias string) (RuntimeConnection, error) {
	conns, err := a.admin.ListRuntimeConnections(ctx)
	if err != nil {
		return RuntimeConnection{}, err
	}
	var match *RuntimeConnection
	for i := range conns {
		if conns[i].Alias == alias {
			if match != nil {
				return RuntimeConnection{}, &AdminError{Status: http.StatusConflict, Code: "ambiguous_alias"}
			}
			match = &conns[i]
		}
	}
	if match == nil {
		return RuntimeConnection{}, &AdminError{Status: http.StatusNotFound, Code: "connection_not_found"}
	}
	return *match, nil
}

// SubmitExternalCredential hands the decrypted API key to the runtime. The
// runtime mints the new connection's alias itself (the pinned api-key
// connect endpoint accepts no caller alias); the worker adopts that alias
// into the attempt before verification (R14 iv).
func (a adminCorrelator) SubmitExternalCredential(ctx context.Context, service, alias, apiKey string) (RuntimeConnection, error) {
	_ = alias // the runtime mints its own alias; see call site
	return a.admin.ConnectRuntimeAPIKey(ctx, service, apiKey)
}

// executeAPIKeyHandoff is the DEDICATED API-key path (plan T07): decrypt the
// transient ciphertext, hand the key to the runtime once, adopt the
// runtime-minted alias, then verify and activate exactly like an OAuth
// confirm. The key material exists only inside this function; audit lines
// and errors never carry it.
func (w *ControlWorker) executeAPIKeyHandoff(ctx context.Context, op ControlOperation, attempt *appconn.OCAuthorizationAttempt, sealed string) error {
	now := w.now().UTC()
	cipher, err := w.transientCipher()
	if err != nil {
		return err
	}
	apiKey, err := cipher.Open(sealed, attempt.ID)
	if err != nil {
		// Undecryptable blob: poison. Dropping deletes the row - and with it
		// the ciphertext (15-minute bound, ruling 7).
		return permanentf("transient credential for attempt %s cannot be opened", attempt.ID)
	}
	correlator, err := w.runtimeCorrelator()
	if err != nil {
		return err
	}
	// Retry idempotency (T08 ruling 1a): once the attempt left pending, the
	// external connection may already exist under the STORED alias — the
	// runtime-minted one after adoption — so resolve FIRST and submit only
	// when nothing resolves. A not-found is the only lookup outcome that may
	// fall through to a submit; any other failure is transient, because a
	// blind second submit could create a second live external connection
	// (T07 Q-2 double-submit window).
	var resolved RuntimeConnection
	if attempt.State == appconnectorsvc.OCAttemptAuthorizing || attempt.State == appconnectorsvc.OCAttemptVerifying {
		prior, rerr := correlator.ResolveExternalConnection(ctx, attempt.Provider, attempt.Alias)
		switch {
		case rerr == nil:
			resolved = prior
		case IsNotFound(rerr):
			// not created yet (or already gone): submit below
		default:
			return rerr
		}
	}
	submitted := false
	if resolved.ID == "" {
		resolved, err = correlator.SubmitExternalCredential(ctx, attempt.Provider, attempt.Alias, apiKey)
		if err != nil {
			return err // transient: retried while the attempt window is open
		}
		submitted = true
	}
	// Walk the attempt to verifying: the external state exists now. The
	// advance stays AFTER the submit (pinned semantics: a failed first
	// submit leaves a pending attempt — the handoff never started).
	if attempt.State == appconnectorsvc.OCAttemptPending {
		ok, err := w.store.AdvanceOCAttempt(ctx, op.TenantID, attempt.ID,
			appconnectorsvc.OCAttemptPending, appconnectorsvc.OCAttemptAuthorizing, now)
		if err != nil {
			w.cleanupOrphanExternal(ctx, op, attempt, resolved, submitted, now)
			return err
		}
		if !ok {
			w.cleanupOrphanExternal(ctx, op, attempt, resolved, submitted, now)
			return permanentf("attempt %s no longer awaiting handoff", attempt.ID)
		}
		attempt.State = appconnectorsvc.OCAttemptAuthorizing
	}
	// Adopt the runtime-minted alias BEFORE verification (R14 iv) so the
	// frozen CanCompleteOCAttempt comparison stays authoritative. When the
	// adoption write itself fails after a fresh submit, the connection is
	// attributable to NO stored alias — the extended cleanup triple
	// (ruling 1a) schedules its deletion before the error surfaces.
	if resolved.Alias != "" && resolved.Alias != attempt.Alias {
		ok, err := w.store.AdoptOCAttemptAlias(ctx, op.TenantID, attempt.ID, attempt.Alias, resolved.Alias, now)
		if err != nil {
			w.cleanupOrphanExternal(ctx, op, attempt, resolved, submitted, now)
			return err
		}
		if !ok {
			w.cleanupOrphanExternal(ctx, op, attempt, resolved, submitted, now)
			return permanentf("attempt %s alias adoption lost the race", attempt.ID)
		}
		attempt.Alias = resolved.Alias
	}
	if attempt.State == appconnectorsvc.OCAttemptAuthorizing {
		ok, err := w.store.AdvanceOCAttempt(ctx, op.TenantID, attempt.ID,
			appconnectorsvc.OCAttemptAuthorizing, appconnectorsvc.OCAttemptVerifying, now)
		if err != nil {
			return err
		}
		if !ok {
			return permanentf("attempt %s cannot enter verification", attempt.ID)
		}
		attempt.State = appconnectorsvc.OCAttemptVerifying
	}
	return w.verifyAndActivate(ctx, op, attempt, resolved, nil)
}

// cleanupOrphanExternal reconciles a JUST-SUBMITTED external connection this
// execution can no longer adopt (T08 ruling 1a, extended cleanup triple for
// T07 Q-2 adopted-alias orphans): when the stored attempt alias still
// differs from the runtime-minted one, no retry can ever resolve the
// connection again, so its remote deletion is scheduled immediately. A
// resolved (pre-existing) connection is never scheduled here — its owner is
// whichever execution created it. Best-effort by design: the enqueue runs
// in its own transaction and failures only log.
func (w *ControlWorker) cleanupOrphanExternal(ctx context.Context, op ControlOperation, attempt *appconn.OCAuthorizationAttempt, resolved RuntimeConnection, submitted bool, now time.Time) {
	if !submitted || resolved.ID == "" || resolved.Alias == attempt.Alias {
		// Nothing provably orphaned: either this execution did not create the
		// connection, or it is already attributable to the stored alias.
		return
	}
	if oc, ok := w.store.(orphanCleaner); ok {
		if err := oc.EnqueueOCOrphanCleanup(ctx, op.TenantID, attempt.ID, resolved.ID, now); err != nil {
			logger.Warnf(ctx, "[connector-control] orphan cleanup scheduling for attempt %s failed: %v", attempt.ID, err)
		}
	}
}

// executeConfirm runs the worker-only activation path: resolve the external
// connection through the exact attempt/alias correlation, gate on the frozen
// CanCompleteOCAttempt check, optionally mint the connection token (failure
// => NO activation), then atomically activate. Browser success flags never
// reach this code.
func (w *ControlWorker) executeConfirm(ctx context.Context, op ControlOperation, attempt *appconn.OCAuthorizationAttempt, p attemptPayload) error {
	correlator, err := w.runtimeCorrelator()
	if err != nil {
		return err
	}
	resolved, err := correlator.ResolveExternalConnection(ctx, attempt.Provider, attempt.Alias)
	if err != nil {
		return err // transient (not connected yet / transport) => retried
	}
	return w.verifyAndActivate(ctx, op, attempt, resolved, p.Actions)
}

// verifyAndActivate performs the shared verification + activation tail:
// provider match, account id presence, live connection status, the frozen
// completion gate, optional token mint (never an empty grant), and the
// one-consume atomic activation with reconciliation cleanup on local
// failure.
func (w *ControlWorker) verifyAndActivate(ctx context.Context, op ControlOperation, attempt *appconn.OCAuthorizationAttempt, resolved RuntimeConnection, actions []string) error {
	now := w.now().UTC()
	if !appconnectorsvc.CanCompleteOCAttempt(*attempt, attempt.Subject, resolved.Alias, op.AuthVersion, now) {
		// Alias substitution or expiry: fail the attempt; the cleanup triple
		// (attempt, alias, external id) protects anything not provably ours.
		_ = w.store.CleanupFailedOCAttempt(ctx, op.TenantID, attempt.ID, resolved.Alias, resolved.ID, now)
		return permanentf("completion gate rejected attempt %s (alias/expiry)", attempt.ID)
	}
	if resolved.Provider != attempt.Provider {
		_ = w.store.CleanupFailedOCAttempt(ctx, op.TenantID, attempt.ID, resolved.Alias, resolved.ID, now)
		return permanentf("provider mismatch for attempt %s", attempt.ID)
	}
	if resolved.ProviderAccountID == "" || (resolved.Status != "" && resolved.Status != "active") {
		_ = w.store.CleanupFailedOCAttempt(ctx, op.TenantID, attempt.ID, resolved.Alias, resolved.ID, now)
		return permanentf("external connection for attempt %s is not a live account connection", attempt.ID)
	}
	// Token issuance (optional, explicit grant only): whitespace-only
	// entries collapse away (T06-Q-F5) and an empty grant NEVER mints -
	// upstream treats [] as allow-all.
	var granted []string
	for _, a := range actions {
		if t := strings.TrimSpace(a); t != "" {
			granted = append(granted, t)
		}
	}
	if len(granted) > 0 {
		name := fmt.Sprintf("weknora-%d-%s-v%d", op.TenantID, attempt.ConnectionID, op.AuthVersion)
		if err := w.mintRuntimeToken(ctx, op.TenantID, attempt.ConnectionID, op.AuthVersion, name, resolved.ID, granted); err != nil {
			return err // token failure => NO activation
		}
	}
	// Re-bind-aware activation when the store carries it (T08 ruling 1b):
	// idempotent on the same external id + generation, a CLEAR conflict
	// otherwise. Stores without the additive method keep the frozen
	// ActivateOCAttempt behavior.
	activate := w.store.ActivateOCAttempt
	if ra, ok := w.store.(rebindActivator); ok {
		activate = ra.ActivateOCAttemptRebind
	}
	if err := activate(ctx, op.TenantID, attempt.ID, resolved.ID, now); err != nil {
		if errors.Is(err, repoapp.ErrOCAttemptConflict) {
			// Remote success / local failure: reconcile by THIS attempt's
			// alias; the store guard never deletes the newest connection.
			_ = w.store.CleanupFailedOCAttempt(ctx, op.TenantID, attempt.ID, resolved.Alias, resolved.ID, now)
			return permanentf("activation rejected for attempt %s", attempt.ID)
		}
		return err
	}
	return nil
}

// mintRuntimeToken mints one per-connection runtime token and persists its
// material under the deterministic reference. Recast reconciliation (T08
// ruling 1d / T05 risk-5): when the sink can READ BACK the record id of a
// previous mint for this generation (a mint whose material persist failed
// and whose row is retrying), that orphaned remote token is revoked BEFORE
// the replacement is cast — retries never accumulate live orphans. The
// record id is persisted first (while the sink can read) so the reconcile
// reference outlives a failed material write; sinks without read-back keep
// the legacy single-write behavior.
func (w *ControlWorker) mintRuntimeToken(ctx context.Context, tenant uint64, connectionID string, authVersion int64, name, externalID string, actions []string) error {
	ref := fmt.Sprintf("oc/runtime-token/%d/%s/%d", tenant, connectionID, authVersion)
	src, canRead := w.secrets.(secretReader)
	if canRead {
		if prior, err := src.GetSecret(ctx, ref+"/record-id"); err == nil && prior != "" {
			// T08 dual review Q-1/F-1 (T09 hard prerequisite): an admin 404
			// here is idempotent success — the orphan was already revoked
			// (earlier partial reconcile, remote GC) — same guard as the two
			// delete paths above; without it a retried mint whose orphan is
			// gone could never succeed. Any other error still aborts BEFORE
			// a second token is cast.
			if err := w.admin.RevokeRuntimeToken(ctx, prior); err != nil && !IsNotFound(err) {
				return fmt.Errorf("reconcile prior token before re-cast: %w", err)
			}
		}
	}
	created, err := w.admin.CreateRuntimeToken(ctx, CreateTokenRequest{Name: name, ExternalID: externalID, Actions: actions})
	if err != nil {
		return err
	}
	if canRead {
		if err := w.secrets.PutSecret(ctx, ref+"/record-id", created.TokenRecordID); err != nil {
			return fmt.Errorf("persist token record id: %w", err)
		}
	}
	if err := w.secrets.PutSecret(ctx, ref, created.Token); err != nil {
		return fmt.Errorf("persist token material: %w", err)
	}
	return nil
}

type createTokenPayload struct {
	Actions   []string `json:"actions"`
	TokenName string   `json:"tokenName"`
}

type deleteTokenPayload struct {
	TokenID string `json:"tokenID"`
}

type deleteConnectionPayload struct {
	ExternalID string `json:"externalID"`
}

func decodePayload(raw json.RawMessage, v interface{}) error {
	if len(raw) == 0 {
		return permanentf("missing operation payload")
	}
	if !json.Valid(raw) {
		return permanentf("malformed operation payload")
	}
	if err := json.Unmarshal(raw, v); err != nil {
		return permanentf("malformed operation payload")
	}
	return nil
}

// dropOperation removes an operation that can never succeed (poison row).
func (w *ControlWorker) dropOperation(ctx context.Context, row *repoapp.OCOperationsOutboxRow, op ControlOperation, cause error) {
	ok, err := w.outbox.CompleteOperation(ctx, row.ID, row.LeaseOwner, row.Fence)
	if err != nil {
		logger.Warnf(ctx, "[connector-control] drop %s failed: %v", op.ID, err)
		return
	}
	if !ok {
		logger.Warnf(ctx, "[connector-control] drop %s lost lease; superseded owner takes over", op.ID)
		return
	}
	// resource_id may carry a transient ciphertext after "|" (API-key
	// authorize rows): only the attempt-id segment is ever logged.
	logger.Warnf(ctx, "[connector-control] dropped control operation %s (kind %s, tenant %d, connection %s): %v",
		op.ID, op.Kind, op.TenantID, resourceIDSegment(op.ConnectionID), cause)
}

// retryOperation schedules the next attempt with exponential backoff capped
// at BackoffMax plus jitter in (cap/2, cap].
func (w *ControlWorker) retryOperation(ctx context.Context, row *repoapp.OCOperationsOutboxRow, op ControlOperation) {
	delay := w.backoff(row.Attempts)
	now := w.now().UTC()
	ok, err := w.outbox.ExtendOperation(ctx, row.ID, row.LeaseOwner, row.Fence, now.Add(delay), now)
	if err != nil {
		logger.Warnf(ctx, "[connector-control] retry scheduling for %s failed: %v", op.ID, err)
		return
	}
	if !ok {
		logger.Warnf(ctx, "[connector-control] retry scheduling for %s lost lease", op.ID)
	}
}

func (w *ControlWorker) backoff(attempts int64) time.Duration {
	shift := uint(0)
	if attempts > 1 {
		shift = uint(attempts - 1)
	}
	if shift > 20 {
		shift = 20
	}
	exp := w.cfg.BackoffBase << shift
	if exp <= 0 || exp > w.cfg.BackoffMax {
		exp = w.cfg.BackoffMax
	}
	half := time.Duration(float64(exp) / 2)
	return exp - time.Duration(w.jitter()*float64(half))
}

// renewLoop keeps a long-running execution's lease alive by extending
// next_at every RenewInterval until the execution finishes. It stops at the
// first failed/lost extension.
func (w *ControlWorker) renewLoop(ctx context.Context, row *repoapp.OCOperationsOutboxRow, done <-chan struct{}) {
	ticker := time.NewTicker(w.cfg.RenewInterval)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := w.now().UTC()
			ok, err := w.outbox.ExtendOperation(ctx, row.ID, row.LeaseOwner, row.Fence, now.Add(w.cfg.Lease), now)
			if err != nil || !ok {
				return
			}
		}
	}
}

// RunForever drains the outbox until ctx is cancelled. Per-tick errors are
// logged, never fatal: a poisoned environment must not kill the worker.
func (w *ControlWorker) RunForever(ctx context.Context, poll time.Duration) error {
	if poll <= 0 {
		poll = w.cfg.PollInterval
	}
	ticker := time.NewTicker(poll)
	defer ticker.Stop()
	for {
		if err := w.RunOnce(ctx); err != nil {
			logger.Warnf(ctx, "[connector-control] tick: %v", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

// FileAdminSecretSource reads the admin secret from a dedicated mount path on
// EVERY call (the mount may appear after startup). It fails closed — missing
// file, empty content, or group/world-readable permissions all refuse to
// yield a secret. Error text carries the path, never the secret.
func FileAdminSecretSource(path string) AdminSecret {
	return func(ctx context.Context) (string, error) {
		st, err := os.Stat(path)
		if err != nil {
			return "", fmt.Errorf("%w: mount %s", ErrAdminSecretUnavailable, path)
		}
		if st.Mode().Perm()&0o077 != 0 {
			return "", fmt.Errorf("%w: mount %s permissions too open", ErrAdminSecretUnavailable, path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("%w: mount %s", ErrAdminSecretUnavailable, path)
		}
		secret := strings.TrimSpace(string(data))
		if secret == "" {
			return "", fmt.Errorf("%w: mount %s empty", ErrAdminSecretUnavailable, path)
		}
		return secret, nil
	}
}

// FileSecretSink is the phase-one SecretSink: material lands in a dedicated
// directory under a filename derived from sha256(ref) — the reference itself
// (which contains tenant ids) never becomes a path, so a hostile ref cannot
// traverse. PRODUCTION USES EncryptedFileSecretSink (T16): static
// encryption at rest plus enforced 0700 directory permissions; this
// plaintext sink remains for dev/test read compatibility because the
// control worker binary refuses to start without an encryption key. The
// database only ever stores the reference.
type FileSecretSink struct{ dir string }

// NewFileSecretSink creates the secrets directory with 0700 (an existing
// directory is left as-is: this sink does not enforce permissions — the
// enforced, encrypted production constructor is NewEncryptedFileSecretSink).
func NewFileSecretSink(dir string) (*FileSecretSink, error) {
	if dir == "" {
		return nil, errors.New("connectorcontrol: secret dir required")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return &FileSecretSink{dir: dir}, nil
}

// PutSecret atomically writes secret under ref (write-to-temp + rename,
// mode 0600). Re-putting the same ref overwrites — the newest token for a
// connection generation wins.
func (s *FileSecretSink) PutSecret(ctx context.Context, ref, secret string) error {
	if ref == "" || secret == "" {
		return errors.New("connectorcontrol: secret ref and material required")
	}
	sum := sha256.Sum256([]byte(ref))
	final := filepath.Join(s.dir, hex.EncodeToString(sum[:])+".secret")
	// Unique temp name per call: concurrent PutSecret calls for one ref must
	// not clobber each other's temp file (T05-Q-02).
	tmp := final + "." + uuid.NewString() + ".tmp"
	if err := os.WriteFile(tmp, []byte(secret), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, final)
}

// GetSecret reads one reference's material back (T08 ruling 1d: the token
// recast reconciliation resolves the previous mint's record id before a
// replacement is cast). A missing file fails closed — never an empty
// success that would masquerade as a known reference.
func (s *FileSecretSink) GetSecret(ctx context.Context, ref string) (string, error) {
	if ref == "" {
		return "", errors.New("connectorcontrol: secret ref required")
	}
	sum := sha256.Sum256([]byte(ref))
	data, err := os.ReadFile(filepath.Join(s.dir, hex.EncodeToString(sum[:])+".secret"))
	if err != nil {
		return "", err
	}
	secret := strings.TrimSpace(string(data))
	if secret == "" {
		return "", errors.New("connectorcontrol: secret material empty")
	}
	return secret, nil
}

// ---------------------------------------------------------------------------
// optional store/sink capabilities (T08): additive interfaces asserted at
// runtime so existing fakes and the frozen T03/T05/T07 faces keep compiling
// and behaving exactly as before when they do not carry the new methods.
// ---------------------------------------------------------------------------

// rebindActivator is the OPTIONAL re-bind-aware activation (ruling 1b):
// idempotent on the same external id + generation, a clear conflict
// otherwise. Falls back to the frozen ActivateOCAttempt when absent.
type rebindActivator interface {
	ActivateOCAttemptRebind(ctx context.Context, tenant uint64, id, externalID string, now time.Time) error
}

// orphanCleaner is the OPTIONAL extended reconciliation enqueue (ruling 1a):
// schedules the delete of an external connection that can no longer be
// attributed to any stored attempt alias.
type orphanCleaner interface {
	EnqueueOCOrphanCleanup(ctx context.Context, tenant uint64, attemptID, externalID string, now time.Time) error
}

// secretReader is the OPTIONAL read side of a SecretSink (ruling 1d): sinks
// that can read back enable the token recast reconciliation and its
// record-id persistence.
type secretReader interface {
	GetSecret(ctx context.Context, ref string) (string, error)
}
