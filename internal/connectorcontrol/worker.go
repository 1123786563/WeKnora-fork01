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
	"os"
	"path/filepath"
	"strings"
	"time"

	appconn "github.com/Tencent/WeKnora/internal/appconnector"
	repoapp "github.com/Tencent/WeKnora/internal/application/repository/appconnector"
	"github.com/Tencent/WeKnora/internal/logger"
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
	// ErrKindNotWiredYet: kind is valid but its execution path lands with a
	// later task (authorize/confirm correlation is T07). Operations stay
	// queued; they are never silently dropped or half-executed.
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

// ControlWorker drains the operations outbox exactly one operation per tick.
type ControlWorker struct {
	cfg      WorkerConfig
	outbox   OutboxStore
	bindings appconn.OCBindingStore
	admin    RuntimeAdmin
	secrets  SecretSink
	secret   AdminSecret
	now      func() time.Time
	jitter   func() float64 // [0,1); injected for deterministic tests
}

// NewControlWorker validates its dependencies up front: every collaborator
// must be present (fail-closed wiring, no silent no-ops).
func NewControlWorker(outbox OutboxStore, bindings appconn.OCBindingStore, admin RuntimeAdmin, sink SecretSink, secret AdminSecret, cfg WorkerConfig) (*ControlWorker, error) {
	if outbox == nil || bindings == nil || admin == nil || sink == nil || secret == nil {
		return nil, errors.New("connectorcontrol: worker dependencies must all be non-nil")
	}
	if cfg.OwnerID == "" || cfg.Lease <= 0 || cfg.RenewInterval <= 0 || cfg.BackoffBase <= 0 || cfg.BackoffMax < cfg.BackoffBase {
		return nil, errors.New("connectorcontrol: invalid worker config")
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 500 * time.Millisecond
	}
	return &ControlWorker{
		cfg:      cfg,
		outbox:   outbox,
		bindings: bindings,
		admin:    admin,
		secrets:  sink,
		secret:   secret,
		now:      time.Now,
		jitter:   defaultJitter,
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

	// 2. Binding freshness gate for kinds that could (re)activate a
	// connection: the binding must exist, not be revoked/errored, and sit at
	// exactly the operation's auth generation. A deactivated connection can
	// never be reactivated by a stale create/authorize operation.
	var binding *appconn.OCBinding
	switch op.Kind {
	case KindAuthorize, KindCreateToken, KindConfirm:
		b, err := w.bindings.GetBinding(ctx, op.TenantID, op.ConnectionID)
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
	}

	// 3. Admin credential gate: fail CLOSED. Without the dedicated mount the
	// worker refuses admin operations entirely — the operation stays queued
	// and is retried once the mount appears.
	if _, err := w.secret(ctx); err != nil {
		w.retryOperation(ctx, row, op)
		return fmt.Errorf("%w: operation %s", ErrAdminSecretUnavailable, op.ID)
	}

	// 4. Execute (single attempt per admin call, no redirects).
	execErr := w.execute(ctx, op, binding)
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
func (w *ControlWorker) execute(ctx context.Context, op ControlOperation, binding *appconn.OCBinding) error {
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
		created, err := w.admin.CreateRuntimeToken(ctx, CreateTokenRequest{Name: name, ExternalID: binding.ExternalID, Actions: p.Actions})
		if err != nil {
			return err
		}
		// The token material is shown exactly once by upstream; persist it
		// under a reference immediately. Only the reference is ever logged,
		// returned or stored in the database.
		ref := fmt.Sprintf("oc/runtime-token/%d/%s/%d", op.TenantID, op.ConnectionID, op.AuthVersion)
		if err := w.secrets.PutSecret(ctx, ref, created.Token); err != nil {
			return fmt.Errorf("persist token material: %w", err)
		}
		return nil
	case KindDeleteToken:
		var p deleteTokenPayload
		if err := decodePayload(op.Payload, &p); err != nil {
			return err
		}
		if p.TokenID == "" {
			return permanentf("delete_token without token id")
		}
		return w.admin.RevokeRuntimeToken(ctx, p.TokenID)
	case KindDeleteConnection:
		var p deleteConnectionPayload
		if err := decodePayload(op.Payload, &p); err != nil {
			return err
		}
		external := p.ExternalID
		if external == "" {
			b, err := w.bindings.GetBinding(ctx, op.TenantID, op.ConnectionID)
			if err == nil {
				external = b.ExternalID
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}
		if external == "" {
			return permanentf("delete_connection without a resolvable external id")
		}
		return w.admin.DeleteRuntimeConnection(ctx, external)
	case KindAuthorize, KindConfirm:
		return fmt.Errorf("%w: %s", ErrKindNotWiredYet, op.Kind)
	default:
		return ErrUnsupportedControlKind
	}
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
	logger.Warnf(ctx, "[connector-control] dropped control operation %s (kind %s, tenant %d, connection %s): %v",
		op.ID, op.Kind, op.TenantID, op.ConnectionID, cause)
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
// 0700 directory under a filename derived from sha256(ref) — the reference
// itself (which contains tenant ids) never becomes a path, so a hostile ref
// cannot traverse. Later tasks swap this for the platform secret store; the
// database only ever stores the reference.
type FileSecretSink struct{ dir string }

// NewFileSecretSink creates (and tightens) the secrets directory.
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
	tmp := final + ".tmp"
	if err := os.WriteFile(tmp, []byte(secret), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, final)
}
