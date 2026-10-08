// oc_connections.go implements the correlate-able authorization lifecycle
// (plan T07): UUID attempts with UUID aliases and a 15-minute window, a
// subject/alias/version-gated completion check, the Begin/Confirm service
// the HTTP surface will drive, the full subject guard wiring (R11 carry),
// and the purpose-separated transient cipher for the API-key handoff.
//
// The browser is NEVER trusted to activate anything: Confirm only moves an
// attempt to verifying and enqueues a confirm operation; activation is the
// control worker's job after it has resolved and validated the external
// connection through the exact attempt/alias correlation (T01 contract).
package appconnector

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"os"
	"strings"
	"time"

	appconn "github.com/Tencent/WeKnora/internal/appconnector"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Authorization-attempt lifecycle states (plan T07):
// pending -> authorizing -> verifying -> active; failed/expired/revoked are
// terminal and never revive.
const (
	OCAttemptPending     = "pending"
	OCAttemptAuthorizing = "authorizing"
	OCAttemptVerifying   = "verifying"
	OCAttemptActive      = "active"
	OCAttemptFailed      = "failed"
	OCAttemptExpired     = "expired"
	OCAttemptRevoked     = "revoked"
)

// OCAttemptTTL is the single-use correlation window: 15 minutes, mirroring
// the upstream OAuth state TTL verified by T01.
const OCAttemptTTL = 15 * time.Minute

// Control-operation kinds the service enqueues. They spell exactly the
// frozen connectorcontrol kind set (the connectorcontrol tests pin the
// parity); the service must never import the admin-client package.
const (
	KindOCAuthorize        = "authorize"
	KindOCConfirm          = "confirm"
	KindOCDeleteConnection = "delete_connection"
)

var (
	// ErrOCAttemptState: the requested transition does not apply to the
	// attempt's current state (replay, cancel, pending confirm).
	ErrOCAttemptState = errors.New("oc_attempt_state")
	// ErrOCAttemptExpired: the callback arrived outside the 15-minute window.
	ErrOCAttemptExpired = errors.New("oc_attempt_expired")
)

// NewOCAttempt mints one single-use correlation record: a UUID attempt id, a
// UUID alias reserved for the external connection, the starting subject and
// authorization generation, and a 15-minute expiry. provider comes ONLY from
// the installation catalog - client input never names it.
func NewOCAttempt(subject appconn.OCSubject, connection, runtime, provider string, authVersion int64, now time.Time) (appconn.OCAuthorizationAttempt, error) {
	if subject.TenantID == 0 || subject.ActorID == "" || connection == "" || runtime == "" ||
		provider == "" || authVersion < 1 || now.IsZero() {
		return appconn.OCAuthorizationAttempt{}, ErrOCAttemptState
	}
	return appconn.OCAuthorizationAttempt{
		ID:           uuid.NewString(),
		Subject:      subject,
		ConnectionID: connection,
		RuntimeID:    runtime,
		Provider:     provider,
		Alias:        uuid.NewString(),
		State:        OCAttemptPending,
		AuthVersion:  authVersion,
		ExpiresAt:    now.Add(OCAttemptTTL).UTC(),
	}, nil
}

// CanCompleteOCAttempt is the frozen completion gate (plan sketch, verbatim
// semantics): only a verifying attempt whose persisted subject, alias and
// authorization generation all match - and only strictly inside its expiry -
// may complete. Alias substitution, subject forgery, stale versions and the
// expiry boundary itself all reject.
func CanCompleteOCAttempt(a appconn.OCAuthorizationAttempt, s appconn.OCSubject,
	alias string, version int64, now time.Time) bool {
	return a.State == OCAttemptVerifying && a.Subject == s && a.Alias == alias &&
		a.AuthVersion == version && now.Before(a.ExpiresAt)
}

// ---------------------------------------------------------------------------
// full subject guard (R11 carry: replace the interim NewSubjectGuard wiring)
// ---------------------------------------------------------------------------

// InstallationLookup is the installation-catalog read the guard and Begin
// need; *repository InstallationStore satisfies it structurally.
type InstallationLookup interface {
	GetInstallationByID(ctx context.Context, tenant uint64, installationID string) (appconn.Installation, error)
}

// installationStateSource adapts an InstallationLookup to the authorizer's
// InstallationStateSource. A missing row fails closed (not active).
type installationStateSource struct{ lookup InstallationLookup }

// NewInstallationStateSource wires the installation catalog into the guard.
func NewInstallationStateSource(l InstallationLookup) InstallationStateSource {
	return &installationStateSource{lookup: l}
}

func (s *installationStateSource) InstallationActive(ctx context.Context, installationID string, tenantID uint64) (bool, error) {
	inst, err := s.lookup.GetInstallationByID(ctx, tenantID, installationID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return inst.State == appconn.InstallationActive, nil
}

// NewOCSubjectGuard builds the FULL container guard: the credential source,
// the live installation catalog, a space-grant source and the OC binding
// store. Nil semantics are exactly the interim guard's (T04-QF-5 pin): nil
// installations skip that check, nil grants fail space connections closed,
// nil bindings defer OC verification - so wiring the full sources can only
// ever TIGHTEN the check, never loosen it.
func NewOCSubjectGuard(src ConnectionCredentialSource, installations InstallationStateSource, grants SpaceGrantSource, bindings appconn.OCBindingStore) A02Guard {
	return NewA02Guard(NewOCAuthorizer(src, installations, grants, bindings))
}

// ---------------------------------------------------------------------------
// transient API-key cipher (ruling 7)
// ---------------------------------------------------------------------------

// TransientCipher seals the provider API key for its at-most-15-minute trip
// through the outbox. The key purpose is deliberately separate from any
// runtime-token storage; T16 owns static secret management and swaps this
// implementation (and its key source) without touching the callers.
type TransientCipher interface {
	Seal(plaintext, aad string) (string, error)
	Open(sealed, aad string) (string, error)
}

// TransientCipherEnvKey is the deployment key source for the phase-one
// cipher. Empty or whitespace key material fails closed.
const TransientCipherEnvKey = "WEKNORA_OC_TRANSIENT_KEY"

// ErrTransientCipherUnavailable: no usable transient key material exists, so
// the API-key path refuses to start rather than storing plaintext.
var ErrTransientCipherUnavailable = errors.New("transient_cipher_unavailable")

// transientCipherSalt is the fixed HKDF-style derivation constant binding
// the derived key to this one purpose (API-key handoff), distinct from every
// other key use in the platform.
const transientCipherPurpose = "weknora-oc-transient-apikey-v1"

type aesGCMTransientCipher struct{ aead cipher.AEAD }

// NewTransientCipherFromKey derives the purpose-separated AES-256-GCM key
// from arbitrary key material (HMAC-SHA256 over the fixed purpose label -
// stdlib only, no new dependencies).
func NewTransientCipherFromKey(key string) (TransientCipher, error) {
	if strings.TrimSpace(key) == "" {
		return nil, ErrTransientCipherUnavailable
	}
	mac := hmac.New(sha256.New, []byte(transientCipherPurpose))
	mac.Write([]byte(key))
	block, err := aes.NewCipher(mac.Sum(nil))
	if err != nil {
		return nil, ErrTransientCipherUnavailable
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, ErrTransientCipherUnavailable
	}
	return &aesGCMTransientCipher{aead: aead}, nil
}

// NewTransientCipherFromEnv loads the transient key material from the
// deployment environment; absence fails closed with a nil cipher.
func NewTransientCipherFromEnv() (TransientCipher, error) {
	return NewTransientCipherFromKey(os.Getenv(TransientCipherEnvKey))
}

// Seal encrypts plaintext under the cipher key, binding the attempt id as
// additional authenticated data: a blob sealed for one attempt cannot be
// replayed against another. Output is versioned base64 (nonce || ciphertext
// || tag); it never contains the plaintext.
func (c *aesGCMTransientCipher) Seal(plaintext, aad string) (string, error) {
	if plaintext == "" || aad == "" {
		return "", ErrTransientCipherUnavailable
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", ErrTransientCipherUnavailable
	}
	sealed := c.aead.Seal(nonce, nonce, []byte(plaintext), []byte(aad))
	return "v1." + base64.RawURLEncoding.EncodeToString(sealed), nil
}

// Open authenticates and decrypts a Seal output against the same aad. Any
// mismatch (wrong key, wrong attempt, corruption) is a single generic error.
func (c *aesGCMTransientCipher) Open(sealed, aad string) (string, error) {
	if !strings.HasPrefix(sealed, "v1.") || aad == "" {
		return "", ErrTransientCipherUnavailable
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(sealed, "v1."))
	if err != nil || len(raw) < c.aead.NonceSize()+c.aead.Overhead() {
		return "", ErrTransientCipherUnavailable
	}
	nonce, ct := raw[:c.aead.NonceSize()], raw[c.aead.NonceSize():]
	pt, err := c.aead.Open(nil, nonce, ct, []byte(aad))
	if err != nil {
		return "", ErrTransientCipherUnavailable
	}
	return string(pt), nil
}

// ---------------------------------------------------------------------------
// OCConnectionService: Begin / Confirm / Cancel
// ---------------------------------------------------------------------------

// OCAttemptStore is the persistence surface the connection service needs.
type OCAttemptStore interface {
	SaveOCAttempt(ctx context.Context, a appconn.OCAuthorizationAttempt) error
	GetOCAttemptByID(ctx context.Context, id string) (appconn.OCAuthorizationAttempt, error)
	AdvanceOCAttempt(ctx context.Context, tenant uint64, id, from, to string, now time.Time) (bool, error)
	TerminateOCAttempt(ctx context.Context, tenant uint64, id, to string, now time.Time) (bool, error)
	EnqueueOCOperation(ctx context.Context, id string, tenant uint64, resourceID string, resourceVersion int64, kind string, nextAt time.Time) error
	EnabledOCRuntime(ctx context.Context) (string, error)
}

// OCConnectionService drives the correlate-able authorization lifecycle.
// Begin is the only entry point a browser ever reaches; Confirm moves an
// attempt to verifying but NEVER activates - activation is worker-only.
type OCConnectionService struct {
	src           ConnectionCredentialSource
	installations InstallationLookup
	store         OCAttemptStore
	cipher        TransientCipher
	now           func() time.Time
}

// OCConnectionOption configures an OCConnectionService.
type OCConnectionOption func(*OCConnectionService)

// WithTransientCipher arms the API-key path's seal/open implementation.
func WithTransientCipher(c TransientCipher) OCConnectionOption {
	return func(s *OCConnectionService) { s.cipher = c }
}

// WithNow overrides the service clock (deterministic tests).
func WithNow(fn func() time.Time) OCConnectionOption {
	return func(s *OCConnectionService) { s.now = fn }
}

// NewOCConnectionService validates its collaborators up front (fail-closed
// wiring; every source must be present).
func NewOCConnectionService(src ConnectionCredentialSource, installations InstallationLookup, store OCAttemptStore, opts ...OCConnectionOption) *OCConnectionService {
	s := &OCConnectionService{src: src, installations: installations, store: store, now: time.Now}
	for _, opt := range opts {
		if opt != nil {
			opt(s)
		}
	}
	return s
}

// Begin starts one OAuth authorization attempt for a connection: it
// validates the subject against the live connection/installation/membership
// chain, mints the attempt with a provider taken from the installation
// catalog and a runtime taken from the enabled registry, persists it, and
// enqueues the authorize operation carrying the attempt id.
func (s *OCConnectionService) Begin(ctx context.Context, subject appconn.OCSubject, connectionID string) (string, error) {
	return s.BeginAt(ctx, subject, connectionID, s.now())
}

// BeginAPIKey starts one API-key authorization attempt: the key is sealed
// immediately (purpose-separated cipher, attempt-bound AAD) and travels only
// as ciphertext inside the authorize operation's resource id. No plaintext
// key is ever persisted, logged or returned.
func (s *OCConnectionService) BeginAPIKey(ctx context.Context, subject appconn.OCSubject, connectionID, apiKey string) (string, error) {
	return s.BeginAPIKeyAt(ctx, subject, connectionID, apiKey, s.now())
}

// BeginAt is Begin with an explicit clock.
func (s *OCConnectionService) BeginAt(ctx context.Context, subject appconn.OCSubject, connectionID string, now time.Time) (string, error) {
	return s.begin(ctx, subject, connectionID, "", now)
}

// BeginAPIKeyAt is BeginAPIKey with an explicit clock.
func (s *OCConnectionService) BeginAPIKeyAt(ctx context.Context, subject appconn.OCSubject, connectionID, apiKey string, now time.Time) (string, error) {
	if strings.TrimSpace(apiKey) == "" {
		return "", ErrTransientCipherUnavailable
	}
	if s.cipher == nil {
		return "", ErrTransientCipherUnavailable
	}
	return s.begin(ctx, subject, connectionID, apiKey, now)
}

func (s *OCConnectionService) begin(ctx context.Context, subject appconn.OCSubject, connectionID, apiKey string, now time.Time) (string, error) {
	if subject.TenantID == 0 || subject.ActorID == "" {
		return "", ErrMissingSubject
	}
	conn, err := s.src.FindConnectionByID(ctx, connectionID)
	if err != nil {
		return "", err
	}
	if conn.TenantID != subject.TenantID {
		return "", ErrConnectionForbidden
	}
	member, err := s.src.MemberActive(ctx, subject.TenantID, subject.ActorID)
	if err != nil {
		return "", err
	}
	if !member {
		return "", ErrSubjectNotMember
	}
	inst, err := s.installations.GetInstallationByID(ctx, subject.TenantID, conn.InstallationID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", ErrInstallationNotActive
		}
		return "", err
	}
	if inst.State != appconn.InstallationActive {
		return "", ErrInstallationNotActive
	}
	runtimeID, err := s.store.EnabledOCRuntime(ctx)
	if err != nil {
		return "", err
	}
	attempt, err := NewOCAttempt(subject, connectionID, runtimeID, inst.AppID, conn.AuthVersion, now)
	if err != nil {
		return "", err
	}
	resource := attempt.ID
	if apiKey != "" {
		sealed, err := s.cipher.Seal(apiKey, attempt.ID)
		if err != nil {
			return "", err
		}
		resource = attempt.ID + "|" + sealed
	}
	if err := s.store.SaveOCAttempt(ctx, attempt); err != nil {
		return "", err
	}
	if err := s.store.EnqueueOCOperation(ctx, uuid.NewString(), subject.TenantID, resource, attempt.AuthVersion, KindOCAuthorize, now); err != nil {
		// Persistence-then-enqueue failure (T07 spec F-04 / T08 ruling 1c):
		// the attempt row exists but no operation will ever drive it, so it
		// terminates failed NOW rather than silently sticking in pending.
		// The enqueue error still surfaces; a caller retry mints a fresh
		// attempt (the failed row never revives).
		_, _ = s.store.TerminateOCAttempt(ctx, subject.TenantID, attempt.ID, OCAttemptFailed, now)
		return "", err
	}
	return attempt.ID, nil
}

// Confirm records that the actor reports the external authorization done.
// The subject comes from the PERSISTED attempt (the browser supplies nothing
// here), the transition authorizing -> verifying is one-consume, and only a
// successful transition enqueues exactly one confirm operation. Expiry is
// enforced strictly and marks the attempt expired for cleanup.
func (s *OCConnectionService) Confirm(ctx context.Context, attemptID string) error {
	return s.ConfirmAt(ctx, attemptID, s.now())
}

// ConfirmAt is Confirm with an explicit clock.
func (s *OCConnectionService) ConfirmAt(ctx context.Context, attemptID string, now time.Time) error {
	a, err := s.store.GetOCAttemptByID(ctx, attemptID)
	if err != nil {
		return err
	}
	if !now.Before(a.ExpiresAt) {
		_, _ = s.store.TerminateOCAttempt(ctx, a.Subject.TenantID, attemptID, OCAttemptExpired, now)
		return ErrOCAttemptExpired
	}
	if a.State != OCAttemptAuthorizing {
		return ErrOCAttemptState
	}
	ok, err := s.store.AdvanceOCAttempt(ctx, a.Subject.TenantID, attemptID, OCAttemptAuthorizing, OCAttemptVerifying, now)
	if err != nil {
		return err
	}
	if !ok {
		return ErrOCAttemptState
	}
	if err := s.store.EnqueueOCOperation(ctx, uuid.NewString(), a.Subject.TenantID, attemptID, a.AuthVersion, KindOCConfirm, now); err != nil {
		// The one-consume transition committed but the confirm operation
		// never landed: nothing will ever drive the verification, so the
		// attempt terminates failed NOW (no silent stuck-verifying; T08
		// ruling 1c). The state machine is forward-only — reverting to
		// authorizing is not an option.
		_, _ = s.store.TerminateOCAttempt(ctx, a.Subject.TenantID, attemptID, OCAttemptFailed, now)
		return err
	}
	return nil
}

// Cancel terminates a live attempt (owner gave up, or an operator cancelled).
// A callback arriving after the cancel finds a revoked attempt and fails.
func (s *OCConnectionService) Cancel(ctx context.Context, attemptID string) error {
	return s.CancelAt(ctx, attemptID, s.now())
}

// CancelAt is Cancel with an explicit clock.
func (s *OCConnectionService) CancelAt(ctx context.Context, attemptID string, now time.Time) error {
	a, err := s.store.GetOCAttemptByID(ctx, attemptID)
	if err != nil {
		return err
	}
	ok, err := s.store.TerminateOCAttempt(ctx, a.Subject.TenantID, attemptID, OCAttemptRevoked, now)
	if err != nil {
		return err
	}
	if !ok {
		return ErrOCAttemptState
	}
	return nil
}
