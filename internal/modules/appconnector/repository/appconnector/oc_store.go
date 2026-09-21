package appconnector

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	appconnector "github.com/Tencent/WeKnora/internal/appconnector"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	// ErrOCBindingInvalid covers malformed input rejected before any
	// database access: missing identity fields, non-positive versions,
	// unknown states, or a creation that does not start at version 1.
	ErrOCBindingInvalid = errors.New("oc_binding_invalid")
	// ErrOCBindingConflict covers rejected writes on an existing row:
	// stale or skipped binding generations, authorization version
	// regressions, and in-place identity rewrites.
	ErrOCBindingConflict = errors.New("oc_binding_conflict")
	// ErrOCOutboxInvalid covers malformed outbox claim/complete/extend input
	// (blank owner/id, non-positive fence or lease) rejected before any
	// database access.
	ErrOCOutboxInvalid = errors.New("oc_outbox_invalid")
)

// OCRuntimeRow is the registry row of one shared open-connector runtime.
// secret_refs holds a JSON array of secret REFERENCES (never credential
// material) for the control process that manages this runtime's tokens.
type OCRuntimeRow struct {
	ID          string `gorm:"primaryKey;column:id"`
	Address     string `gorm:"column:address;not null;default:''"`
	Enabled     bool   `gorm:"column:enabled;not null;default:false"`
	Version     string `gorm:"column:version;not null;default:''"`
	ImageDigest string `gorm:"column:image_digest;not null;default:''"`
	SecretRefs  string `gorm:"column:secret_refs;not null;default:''"`
}

func (OCRuntimeRow) TableName() string { return "connector_runtimes" }

// OCBindingRow persists the tenant-scoped wiring between one local
// connection and one external connection on a runtime. The composite primary
// key makes every binding permanently tenant-scoped; the two unique keys pin
// the physical external connection and the server-generated alias to one
// binding each — a revoked row keeps both reserved forever. auth_version
// mirrors the local connection's authorization counter and may only move
// forward; binding_version is the row's optimistic-concurrency counter.
type OCBindingRow struct {
	TenantID       uint64 `gorm:"primaryKey;column:tenant_id"`
	ConnectionID   string `gorm:"primaryKey;column:connection_id"`
	RuntimeID      string `gorm:"column:runtime_id;not null;uniqueIndex:uq_oc_bindings_runtime_external"`
	Provider       string `gorm:"column:provider;not null;uniqueIndex:uq_oc_bindings_runtime_provider_alias"`
	ExternalID     string `gorm:"column:external_id;not null;uniqueIndex:uq_oc_bindings_runtime_external"`
	Alias          string `gorm:"column:alias;not null;uniqueIndex:uq_oc_bindings_runtime_provider_alias"`
	AuthVersion    int64  `gorm:"column:auth_version;not null;check:auth_version > 0"`
	BindingVersion int64  `gorm:"column:binding_version;not null;check:binding_version > 0"`
	State          string `gorm:"column:state;not null;check:state IN ('pending','active','revoked','error')"`
}

func (OCBindingRow) TableName() string { return "connector_connection_bindings" }

// OCDefinitionRow is one reviewed action definition pinned to an app
// version. input_schema and required_scopes store the frozen JSON the review
// approved; schema_digest lets executors detect drift without re-parsing.
type OCDefinitionRow struct {
	AppID          string `gorm:"primaryKey;column:app_id"`
	AppVersion     string `gorm:"primaryKey;column:app_version"`
	ActionID       string `gorm:"primaryKey;column:action_id"`
	Provider       string `gorm:"column:provider;not null;default:''"`
	SchemaDigest   string `gorm:"column:schema_digest;not null;default:''"`
	InputSchema    string `gorm:"column:input_schema;not null;default:''"`
	RequiredScopes string `gorm:"column:required_scopes;not null;default:''"`
	Risk           string `gorm:"column:risk;not null;default:''"`
	Published      bool   `gorm:"column:published;not null;default:false"`
}

func (OCDefinitionRow) TableName() string { return "connector_action_definitions" }

// OCAuthorizationAttemptRow records one in-flight external authorization
// flow: the tenant/actor/connection that started it, the alias reserved for
// it, its state and its expiry. Attempts are single-use correlation records;
// they never grant anything by themselves.
type OCAuthorizationAttemptRow struct {
	ID           string    `gorm:"primaryKey;column:id"`
	TenantID     uint64    `gorm:"column:tenant_id;not null"`
	ActorID      string    `gorm:"column:actor_id;not null;default:''"`
	ConnectionID string    `gorm:"column:connection_id;not null"`
	RuntimeID    string    `gorm:"column:runtime_id;not null;default:''"`
	Provider     string    `gorm:"column:provider;not null;default:''"`
	Alias        string    `gorm:"column:alias;not null;default:''"`
	State        string    `gorm:"column:state;not null"`
	AuthVersion  int64     `gorm:"column:auth_version;not null;default:1"`
	ExpiresAt    time.Time `gorm:"column:expires_at;not null"`
}

func (OCAuthorizationAttemptRow) TableName() string { return "connector_authorization_attempts" }

// OCOperationsOutboxRow is one deferred cross-system operation (token
// refresh, remote revocation, orphan cleanup) with its retry bookkeeping:
// attempt count, next eligibility time, lease owner and the fence counter
// guarding lease takeovers.
type OCOperationsOutboxRow struct {
	ID              string    `gorm:"primaryKey;column:id"`
	TenantID        uint64    `gorm:"column:tenant_id;not null"`
	ResourceID      string    `gorm:"column:resource_id;not null;default:''"`
	ResourceVersion int64     `gorm:"column:resource_version;not null;default:0"`
	Kind            string    `gorm:"column:kind;not null"`
	Attempts        int64     `gorm:"column:attempts;not null;default:0"`
	NextAt          time.Time `gorm:"column:next_at;not null"`
	LeaseOwner      string    `gorm:"column:lease_owner;not null;default:''"`
	Fence           int64     `gorm:"column:fence;not null;default:0"`
	CreatedAt       time.Time `gorm:"column:created_at;not null"`
	UpdatedAt       time.Time `gorm:"column:updated_at;not null"`
}

func (OCOperationsOutboxRow) TableName() string { return "connector_operations_outbox" }

// OCStore persists the open-connector runtime registry, tenant-scoped
// connection bindings, reviewed action definitions, authorization attempts
// and the operations outbox. All queries are tenant-scoped; this store never
// materializes credential secrets, only references.
type OCStore struct{ db *gorm.DB }

// NewOCStore returns an OCStore bound to db.
func NewOCStore(db *gorm.DB) *OCStore { return &OCStore{db: db} }

// compile-time conformance: the store satisfies the shared tenant-scoped
// binding read contract.
var _ appconnector.OCBindingStore = (*OCStore)(nil)

func validOCBindingState(s string) bool {
	return s == appconnector.OCBindingPending || s == appconnector.OCBindingActive ||
		s == appconnector.OCBindingRevoked || s == appconnector.OCBindingError
}

// SaveBinding upserts one tenant binding.
//
// Creation always lands at binding version 1. Updates are compare-and-swap:
// the new generation must be exactly current+1, and the authorization
// version may never regress (the optimistic lock only moves forward). The
// external identity of a binding — alias and external id — is immutable for
// the row's whole life, including after revocation; replacing an account
// means revoking this binding and creating a new one under a fresh alias.
// Violations of the physical uniqueness keys (one external connection or one
// alias per runtime) surface as the driver's constraint errors.
func (s *OCStore) SaveBinding(ctx context.Context, b appconnector.OCBinding) error {
	if b.TenantID == 0 || b.ConnectionID == "" || b.RuntimeID == "" || b.Provider == "" ||
		b.ExternalID == "" || b.Alias == "" || b.AuthVersion <= 0 || b.BindingVersion <= 0 ||
		!validOCBindingState(b.State) {
		return ErrOCBindingInvalid
	}
	row := OCBindingRow{
		TenantID: b.TenantID, ConnectionID: b.ConnectionID, RuntimeID: b.RuntimeID,
		Provider: b.Provider, ExternalID: b.ExternalID, Alias: b.Alias,
		AuthVersion: b.AuthVersion, BindingVersion: b.BindingVersion, State: b.State,
	}
	var cur OCBindingRow
	err := s.db.WithContext(ctx).Where("tenant_id = ? AND connection_id = ?", b.TenantID, b.ConnectionID).First(&cur).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if b.BindingVersion != 1 {
			return ErrOCBindingInvalid
		}
		return s.db.WithContext(ctx).Create(&row).Error
	}
	if err != nil {
		return err
	}
	if b.BindingVersion != cur.BindingVersion+1 {
		return ErrOCBindingConflict
	}
	if b.AuthVersion < cur.AuthVersion {
		return ErrOCBindingConflict
	}
	if b.Alias != cur.Alias || b.ExternalID != cur.ExternalID {
		return ErrOCBindingConflict
	}
	res := s.db.WithContext(ctx).Model(&OCBindingRow{}).
		Where("tenant_id = ? AND connection_id = ? AND binding_version = ?", b.TenantID, b.ConnectionID, cur.BindingVersion).
		Updates(map[string]interface{}{
			"runtime_id": b.RuntimeID, "provider": b.Provider, "state": b.State,
			"auth_version": b.AuthVersion, "binding_version": b.BindingVersion,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrOCBindingConflict
	}
	return nil
}

// ---------------------------------------------------------------------------
// operations outbox claim/complete/extend (T05 control worker)
//
// SCHEMA ADAPTATION (coordinator ruling 1): the plan sketch modeled the lease
// with a dedicated lease_until column, but the FROZEN T03 schema
// (migrations/versioned/000121 + migrations/sqlite/000041) has NO lease_until:
// the outbox carries (next_at, lease_owner, fence) only. The claim protocol
// therefore doubles next_at as the lease deadline (visibility timeout):
//
//   - a row is claimable exactly when next_at <= now (never attempted, retry
//     backoff elapsed, or a previous worker's lease expired);
//   - claiming pushes next_at to now+lease, bumping fence and attempts, so
//     the row is invisible to every other worker until the lease lapses;
//   - renewal and retry scheduling are the same UPDATE, only the new deadline
//     differs (renewal: now+lease; backoff: now+exponential delay);
//   - Complete must match id AND lease_owner AND fence, so a worker whose
//     lease expired and was taken over at fence+1 updates ZERO rows.
//
// CLAIM/OPERATION FIELD MAPPING (ruling 1): the frozen schema names are
// resource_id / resource_version; the control-plane ControlOperation calls
// them ConnectionID / AuthVersion. OperationFromRow in
// internal/connectorcontrol performs the explicit mapping.
// ---------------------------------------------------------------------------

// outboxClaimSQL is one atomic claim. The UPDATE and its subselect run as a
// single statement, so exactly one concurrent caller can win a row. On
// PostgreSQL the subselect carries FOR UPDATE SKIP LOCKED (true queue
// semantics under concurrency — sqlite cannot honor SKIP LOCKED, which is why
// the double-claim acceptance runs on real PostgreSQL); on sqlite the
// single-statement UPDATE is atomic under the database's serialized writer.
// RETURNING (sqlite >= 3.35, always on PostgreSQL) yields the post-claim row
// so no second read is needed.
func outboxClaimSQL(dialect string) string {
	lock := ""
	if dialect == "postgres" {
		lock = " FOR UPDATE SKIP LOCKED"
	}
	return "UPDATE connector_operations_outbox " +
		"SET lease_owner = @owner, fence = fence + 1, attempts = attempts + 1, " +
		"next_at = @until, updated_at = @now " +
		"WHERE id IN (SELECT id FROM connector_operations_outbox " +
		"WHERE next_at <= @now ORDER BY next_at, id LIMIT 1" + lock + ") " +
		"RETURNING id, tenant_id, resource_id, resource_version, kind, attempts, " +
		"next_at, lease_owner, fence, created_at, updated_at"
}

// ClaimNextOperation atomically leases the earliest due outbox operation to
// owner: lease_owner/fence/attempts are bumped and next_at becomes the lease
// deadline now+lease. It returns nil (nil error) when nothing is due.
func (s *OCStore) ClaimNextOperation(ctx context.Context, owner string, now time.Time, lease time.Duration) (*OCOperationsOutboxRow, error) {
	if owner == "" || lease <= 0 || now.IsZero() {
		return nil, ErrOCOutboxInvalid
	}
	var row OCOperationsOutboxRow
	err := s.db.WithContext(ctx).Raw(outboxClaimSQL(s.db.Dialector.Name()), map[string]interface{}{
		"owner": owner,
		"until": now.Add(lease),
		"now":   now,
	}).Scan(&row).Error
	if err != nil {
		return nil, err
	}
	if row.ID == "" {
		return nil, nil
	}
	return &row, nil
}

// CompleteOperation removes a finished operation, but only if the caller still
// holds the lease: the predicate is id AND lease_owner AND fence. A worker
// whose lease was taken over at a higher fence updates ZERO rows (reported as
// false, not an error).
func (s *OCStore) CompleteOperation(ctx context.Context, id, owner string, fence int64) (bool, error) {
	if id == "" || owner == "" || fence < 1 {
		return false, ErrOCOutboxInvalid
	}
	res := s.db.WithContext(ctx).Exec(
		"DELETE FROM connector_operations_outbox WHERE id = ? AND lease_owner = ? AND fence = ?",
		id, owner, fence)
	return res.RowsAffected > 0, res.Error
}

// ExtendOperation moves an operation's next_at deadline (lease renewal or
// retry backoff) under the same lease_owner+fence predicate. false means the
// caller no longer holds the lease; the row itself is left untouched.
func (s *OCStore) ExtendOperation(ctx context.Context, id, owner string, fence int64, until, now time.Time) (bool, error) {
	if id == "" || owner == "" || fence < 1 || until.IsZero() || now.IsZero() {
		return false, ErrOCOutboxInvalid
	}
	res := s.db.WithContext(ctx).Exec(
		"UPDATE connector_operations_outbox SET next_at = ?, updated_at = ? WHERE id = ? AND lease_owner = ? AND fence = ?",
		until, now, id, owner, fence)
	return res.RowsAffected > 0, res.Error
}

// GetBinding loads one tenant's binding for a local connection. Unscoped
// input (zero tenant, empty connection) is rejected before any database
// access, so a misconfigured store can never leak another tenant's row.
func (s *OCStore) GetBinding(ctx context.Context, tenant uint64, connection string) (appconnector.OCBinding, error) {
	if tenant == 0 || connection == "" {
		return appconnector.OCBinding{}, ErrOCBindingInvalid
	}
	var row OCBindingRow
	if err := s.db.WithContext(ctx).Where("tenant_id = ? AND connection_id = ?", tenant, connection).First(&row).Error; err != nil {
		return appconnector.OCBinding{}, err
	}
	return appconnector.OCBinding{
		TenantID: row.TenantID, ConnectionID: row.ConnectionID, RuntimeID: row.RuntimeID,
		Provider: row.Provider, ExternalID: row.ExternalID, Alias: row.Alias,
		AuthVersion: row.AuthVersion, BindingVersion: row.BindingVersion, State: row.State,
	}, nil
}

// ---------------------------------------------------------------------------
// reviewed action definitions (T06 catalog)
//
// DOMAIN↔ROW MAPPING (coordinator ruling 3): appconnector.OCDefinition ↔
// OCDefinitionRow — RequiredScopes []string ↔ required_scopes JSON array,
// InputSchema json.RawMessage ↔ input_schema TEXT, everything else 1:1
// against the frozen connector_action_definitions PK(app_id, app_version,
// action_id). Risk values are stored EXACTLY as reviewed: the row layer
// never widens, narrows or invents vocabulary — the closed
// read/write/send/delete set is enforced by the reviewing service before
// publication (ValidateOCDefinition).
// ---------------------------------------------------------------------------

var (
	// ErrOCDefinitionInvalid covers malformed definitions rejected before
	// any database access: blank identity fields — including the degenerate
	// empty action set (a definition with no action id).
	ErrOCDefinitionInvalid = errors.New("oc_definition_invalid")
	// ErrOCDefinitionConflict covers immutability violations: re-publishing
	// an existing (app_id, app_version, action_id) with any reviewed-content
	// change (schema, digest, scopes, risk, provider).
	ErrOCDefinitionConflict = errors.New("oc_definition_conflict")
)

// ocScopesToRow marshals the required-scope list into its stored JSON
// array form; nil and empty both store "[]" so a missing column value can
// never masquerade as a different scope set.
func ocScopesToRow(scopes []string) (string, error) {
	if len(scopes) == 0 {
		return "[]", nil
	}
	raw, err := json.Marshal(scopes)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// ocScopesFromRow is the inverse mapping. Rows are only ever written by
// ocScopesToRow; a value that fails to parse maps to nil (no scopes),
// never to a widened set.
func ocScopesFromRow(raw string) []string {
	if raw == "" {
		return nil
	}
	var scopes []string
	if err := json.Unmarshal([]byte(raw), &scopes); err != nil {
		return nil
	}
	return scopes
}

// SaveDefinition persists one reviewed action definition under the
// immutability contract (ruling 5): the (app_id, app_version, action_id)
// row is created once and NEVER overwritten. A re-publish of the same
// triple is idempotent only when every reviewed field is byte-identical;
// any change to schema, digest, scopes, risk or provider is a conflict —
// scope/schema/risk changes require a NEW app_version. The published flag
// is the one workflow field the platform may still flip on a frozen row
// (staging a review and later publishing it, or taking a compromised
// action down).
func (s *OCStore) SaveDefinition(ctx context.Context, d appconnector.OCDefinition) error {
	if d.AppID == "" || d.AppVersion == "" || d.ActionID == "" || d.Provider == "" {
		return ErrOCDefinitionInvalid
	}
	scopes, err := ocScopesToRow(d.RequiredScopes)
	if err != nil {
		return err
	}
	row := OCDefinitionRow{
		AppID: d.AppID, AppVersion: d.AppVersion, ActionID: d.ActionID,
		Provider: d.Provider, SchemaDigest: d.SchemaDigest, InputSchema: string(d.InputSchema),
		RequiredScopes: scopes, Risk: d.Risk, Published: d.Published,
	}
	var cur OCDefinitionRow
	err = s.db.WithContext(ctx).Where("app_id = ? AND app_version = ? AND action_id = ?", d.AppID, d.AppVersion, d.ActionID).First(&cur).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return s.db.WithContext(ctx).Create(&row).Error
	}
	if err != nil {
		return err
	}
	if cur.Provider == row.Provider && cur.SchemaDigest == row.SchemaDigest &&
		cur.InputSchema == row.InputSchema && cur.RequiredScopes == row.RequiredScopes &&
		cur.Risk == row.Risk {
		if cur.Published == row.Published {
			return nil // byte-identical re-publish stays idempotent
		}
		res := s.db.WithContext(ctx).Model(&OCDefinitionRow{}).
			Where("app_id = ? AND app_version = ? AND action_id = ?", d.AppID, d.AppVersion, d.ActionID).
			Update("published", row.Published)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrOCDefinitionConflict
		}
		return nil
	}
	return ErrOCDefinitionConflict
}

// GetDefinition loads one definition by its exact frozen identity. The
// caller (the catalog service) owns the published/no_auth/tenant gates;
// this read carries no tenant dimension because (app, version, action)
// has none — tenant scope is enforced by the resolution chain that
// produces the key.
func (s *OCStore) GetDefinition(ctx context.Context, app, version, action string) (appconnector.OCDefinition, error) {
	if app == "" || version == "" || action == "" {
		return appconnector.OCDefinition{}, ErrOCDefinitionInvalid
	}
	var row OCDefinitionRow
	if err := s.db.WithContext(ctx).Where("app_id = ? AND app_version = ? AND action_id = ?", app, version, action).First(&row).Error; err != nil {
		return appconnector.OCDefinition{}, err
	}
	return appconnector.OCDefinition{
		AppID: row.AppID, AppVersion: row.AppVersion, ActionID: row.ActionID,
		Provider: row.Provider, SchemaDigest: row.SchemaDigest,
		InputSchema: json.RawMessage(row.InputSchema), RequiredScopes: ocScopesFromRow(row.RequiredScopes),
		Risk: row.Risk, Published: row.Published,
	}, nil
}

// ---------------------------------------------------------------------------
// authorization attempts + enqueue (T07 correlate-able OAuth / API key)
//
// PAYLOAD ADAPTATION (coordinator ruling 3, option a): the frozen outbox
// schema has NO payload column, so authorize/confirm rows encode their
// minimal payload into resource_id:
//
//	authorize (OAuth)     resource_id = <attempt id>
//	authorize (API key)   resource_id = <attempt id>|<sealed transient cipher>
//	confirm               resource_id = <attempt id>
//	delete_connection
//	  (cleanup enqueue)   resource_id = <attempt id>|<external id>
//
// The API-key transient ciphertext therefore lives exactly as long as its
// outbox row: success completes (deletes) the row, and the attempt expiry
// (15 minutes) turns a failing handoff into a permanent rejection that also
// deletes the row - no migration, no second store.
// ---------------------------------------------------------------------------

var (
	// ErrOCAttemptInvalid covers malformed attempt input rejected before any
	// database access: missing identity fields, unknown states, or an
	// illegal state-transition request.
	ErrOCAttemptInvalid = errors.New("oc_attempt_invalid")
	// ErrOCAttemptConflict covers rejected attempt writes: lost one-consume
	// races (the conditional UPDATE matched zero rows), activation against a
	// drifted connection version, or a disabled installation.
	ErrOCAttemptConflict = errors.New("oc_attempt_conflict")
	// ErrOCOperationInvalid covers malformed enqueue input rejected before
	// any database access.
	ErrOCOperationInvalid = errors.New("oc_operation_invalid")
	// ErrOCRuntimeUnavailable: no enabled open-connector runtime is
	// registered, so no authorization can start (fail closed).
	ErrOCRuntimeUnavailable = errors.New("oc_runtime_unavailable")
)

// Attempt lifecycle states (plan T07): pending -> authorizing -> verifying
// -> active; failed/expired/revoked are terminal and never revive. The
// service layer owns the exported spellings; these row-layer constants keep
// the SQL predicates independent of it.
const (
	ocAttemptPending     = "pending"
	ocAttemptAuthorizing = "authorizing"
	ocAttemptVerifying   = "verifying"
	ocAttemptActive      = "active"
	ocAttemptFailed      = "failed"
	ocAttemptExpired     = "expired"
	ocAttemptRevoked     = "revoked"
)

func validOCAttemptState(s string) bool {
	switch s {
	case ocAttemptPending, ocAttemptAuthorizing, ocAttemptVerifying, ocAttemptActive,
		ocAttemptFailed, ocAttemptExpired, ocAttemptRevoked:
		return true
	}
	return false
}

func ocAttemptRowFromDomain(row OCAuthorizationAttemptRow) appconnector.OCAuthorizationAttempt {
	return appconnector.OCAuthorizationAttempt{
		ID: row.ID,
		Subject: appconnector.OCSubject{
			TenantID: row.TenantID,
			ActorID:  row.ActorID,
		},
		ConnectionID: row.ConnectionID,
		RuntimeID:    row.RuntimeID,
		Provider:     row.Provider,
		Alias:        row.Alias,
		State:        row.State,
		AuthVersion:  row.AuthVersion,
		ExpiresAt:    row.ExpiresAt,
	}
}

// SaveOCAttempt inserts one new attempt row. Attempts are correlation
// records, not grants: the row is created once by Begin and every later
// change goes through the conditional-transition methods.
func (s *OCStore) SaveOCAttempt(ctx context.Context, a appconnector.OCAuthorizationAttempt) error {
	if a.ID == "" || a.Subject.TenantID == 0 || a.Subject.ActorID == "" ||
		a.ConnectionID == "" || a.RuntimeID == "" || a.Provider == "" || a.Alias == "" ||
		a.AuthVersion <= 0 || !validOCAttemptState(a.State) || a.ExpiresAt.IsZero() {
		return ErrOCAttemptInvalid
	}
	row := OCAuthorizationAttemptRow{
		ID: a.ID, TenantID: a.Subject.TenantID, ActorID: a.Subject.ActorID,
		ConnectionID: a.ConnectionID, RuntimeID: a.RuntimeID, Provider: a.Provider,
		Alias: a.Alias, State: a.State, AuthVersion: a.AuthVersion, ExpiresAt: a.ExpiresAt,
	}
	return s.db.WithContext(ctx).Create(&row).Error
}

// GetOCAttempt loads one attempt, tenant-scoped: another tenant's attempt id
// is indistinguishable from a missing one.
func (s *OCStore) GetOCAttempt(ctx context.Context, tenant uint64, id string) (appconnector.OCAuthorizationAttempt, error) {
	if tenant == 0 || id == "" {
		return appconnector.OCAuthorizationAttempt{}, ErrOCAttemptInvalid
	}
	var row OCAuthorizationAttemptRow
	if err := s.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenant, id).First(&row).Error; err != nil {
		return appconnector.OCAuthorizationAttempt{}, err
	}
	return ocAttemptRowFromDomain(row), nil
}

// GetOCAttemptByID loads one attempt by its unguessable id alone. Only the
// Confirm/Cancel paths (which hold the id the Begin call issued) may use it;
// every authorization decision still re-derives the tenant from the row.
func (s *OCStore) GetOCAttemptByID(ctx context.Context, id string) (appconnector.OCAuthorizationAttempt, error) {
	if id == "" {
		return appconnector.OCAuthorizationAttempt{}, ErrOCAttemptInvalid
	}
	var row OCAuthorizationAttemptRow
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&row).Error; err != nil {
		return appconnector.OCAuthorizationAttempt{}, err
	}
	return ocAttemptRowFromDomain(row), nil
}

// AdvanceOCAttempt performs one FORWARD lifecycle transition
// (pending->authorizing, authorizing->verifying) as a conditional UPDATE:
// the row must still sit in the from-state inside its expiry window. false
// means the transition lost a race or the attempt moved on - the caller
// treats that as a rejected replay, never an error.
func (s *OCStore) AdvanceOCAttempt(ctx context.Context, tenant uint64, id, from, to string, now time.Time) (bool, error) {
	if tenant == 0 || id == "" || !validOCAttemptState(from) || !validOCAttemptState(to) {
		return false, ErrOCAttemptInvalid
	}
	allowed := (from == ocAttemptPending && to == ocAttemptAuthorizing) ||
		(from == ocAttemptAuthorizing && to == ocAttemptVerifying)
	if !allowed {
		return false, ErrOCAttemptInvalid
	}
	// expires_at is stored UTC-only: normalize the caller's instant before
	// it becomes a SQL predicate, or a non-UTC host clock skews the window
	// (T07 quality Q-1).
	res := s.db.WithContext(ctx).Model(&OCAuthorizationAttemptRow{}).
		Where("id = ? AND tenant_id = ? AND state = ? AND expires_at > ?", id, tenant, from, now.UTC()).
		Update("state", to)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

// AdoptOCAttemptAlias rewrites the attempt's alias to the runtime-minted
// one (API-key path, coordinator R14(iv)): the runtime's api-key connect
// endpoint accepts no caller alias, so the worker persists the alias it got
// BEFORE verification, keeping the frozen CanCompleteOCAttempt alias
// comparison authoritative. Conditional on the current alias so a replayed
// handoff can never overwrite a different generation.
func (s *OCStore) AdoptOCAttemptAlias(ctx context.Context, tenant uint64, id, fromAlias, toAlias string, now time.Time) (bool, error) {
	if tenant == 0 || id == "" || fromAlias == "" || toAlias == "" || fromAlias == toAlias {
		return false, ErrOCAttemptInvalid
	}
	res := s.db.WithContext(ctx).Model(&OCAuthorizationAttemptRow{}).
		Where("id = ? AND tenant_id = ? AND alias = ? AND expires_at > ?", id, tenant, fromAlias, now.UTC()).
		Update("alias", toAlias)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

// TerminateOCAttempt moves a live attempt (pending/authorizing/verifying)
// into a terminal state (failed/expired/revoked). Terminal rows never change
// again, so a replayed termination reports false without error.
func (s *OCStore) TerminateOCAttempt(ctx context.Context, tenant uint64, id, to string, now time.Time) (bool, error) {
	if tenant == 0 || id == "" {
		return false, ErrOCAttemptInvalid
	}
	switch to {
	case ocAttemptFailed, ocAttemptExpired, ocAttemptRevoked:
	default:
		return false, ErrOCAttemptInvalid
	}
	res := s.db.WithContext(ctx).Model(&OCAuthorizationAttemptRow{}).
		Where("id = ? AND tenant_id = ? AND state IN (?, ?, ?)", id, tenant,
			ocAttemptPending, ocAttemptAuthorizing, ocAttemptVerifying).
		Update("state", to)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

// ActivateOCAttempt is the ONE-CONSUME activation (rulings 5/6): inside a
// single transaction it re-checks the local connection (same authorization
// generation) and its installation (still active), consumes the verifying
// attempt with a conditional UPDATE, and inserts the active binding carrying
// the persisted external id. A duplicate/replayed activation matches ZERO
// rows and writes nothing.
func (s *OCStore) ActivateOCAttempt(ctx context.Context, tenant uint64, id, externalID string, now time.Time) error {
	if tenant == 0 || id == "" || externalID == "" || now.IsZero() {
		return ErrOCAttemptInvalid
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var att OCAuthorizationAttemptRow
		if err := tx.Where("tenant_id = ? AND id = ?", tenant, id).First(&att).Error; err != nil {
			return err
		}
		// Local freshness re-check: the connection must still sit at the
		// attempt's authorization generation...
		var conn ConnectionRow
		if err := tx.Where("tenant_id = ? AND id = ?", tenant, att.ConnectionID).First(&conn).Error; err != nil {
			return err
		}
		if conn.AuthVersion != att.AuthVersion {
			return ErrOCAttemptConflict
		}
		// ...and the installation behind it must still be active.
		var inst InstallationRow
		if err := tx.Where("id = ? AND tenant_id = ?", conn.InstallationID, tenant).First(&inst).Error; err != nil {
			return err
		}
		if inst.State != appconnector.InstallationActive {
			return ErrOCAttemptConflict
		}
		// One-consume: WHERE state = 'verifying' AND expires_at > now (UTC
		// predicate - stored expiry is UTC-only, T07 quality Q-1).
		res := tx.Model(&OCAuthorizationAttemptRow{}).
			Where("id = ? AND tenant_id = ? AND state = ? AND expires_at > ?", id, tenant, ocAttemptVerifying, now.UTC()).
			Update("state", ocAttemptActive)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrOCAttemptConflict
		}
		binding := OCBindingRow{
			TenantID: tenant, ConnectionID: att.ConnectionID, RuntimeID: att.RuntimeID,
			Provider: att.Provider, ExternalID: externalID, Alias: att.Alias,
			AuthVersion: att.AuthVersion, BindingVersion: 1, State: appconnector.OCBindingActive,
		}
		if err := tx.Create(&binding).Error; err != nil {
			return err
		}
		return nil
	})
}

// CleanupFailedOCAttempt reconciles a remote success with a local failure:
// the attempt is marked failed, and a remote delete_connection operation is
// enqueued ONLY when the cleanup triple matches - this attempt, this
// attempt's exact alias, and the external id resolved for it - AND no newer
// activation has since bound that external connection (the newest connection
// is never deleted).
func (s *OCStore) CleanupFailedOCAttempt(ctx context.Context, tenant uint64, attemptID, alias, externalID string, now time.Time) error {
	if tenant == 0 || attemptID == "" || alias == "" || externalID == "" {
		return ErrOCAttemptInvalid
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var att OCAuthorizationAttemptRow
		if err := tx.Where("tenant_id = ? AND id = ?", tenant, attemptID).First(&att).Error; err != nil {
			return err
		}
		res := tx.Model(&OCAuthorizationAttemptRow{}).
			Where("id = ? AND tenant_id = ? AND state IN (?, ?, ?)", attemptID, tenant,
				ocAttemptPending, ocAttemptAuthorizing, ocAttemptVerifying).
			Update("state", ocAttemptFailed)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			// Already terminal or active: nothing to reconcile for it.
			return nil
		}
		if att.Alias != alias {
			// The external connection cannot be attributed to this attempt:
			// never delete something this attempt cannot prove it owns.
			return nil
		}
		var bound int64
		if err := tx.Model(&OCBindingRow{}).
			Where("runtime_id = ? AND external_id = ? AND state = ?", att.RuntimeID, externalID, appconnector.OCBindingActive).
			Count(&bound).Error; err != nil {
			return err
		}
		if bound > 0 {
			// A newer activation owns the external connection now.
			return nil
		}
		row := OCOperationsOutboxRow{
			ID: uuid.NewString(), TenantID: tenant,
			ResourceID: attemptID + "|" + externalID, ResourceVersion: att.AuthVersion,
			Kind: "delete_connection", NextAt: now.UTC(),
			CreatedAt: now.UTC(), UpdatedAt: now.UTC(),
		}
		return tx.Create(&row).Error
	})
}

// EnqueueOCOperation inserts one deferred control operation. nextAt is
// stored normalized to UTC (T05-Q-03): the outbox clock is UTC-only.
func (s *OCStore) EnqueueOCOperation(ctx context.Context, id string, tenant uint64, resourceID string, resourceVersion int64, kind string, nextAt time.Time) error {
	if id == "" || tenant == 0 || resourceID == "" || resourceVersion < 1 || kind == "" || nextAt.IsZero() {
		return ErrOCOperationInvalid
	}
	row := OCOperationsOutboxRow{
		ID: id, TenantID: tenant, ResourceID: resourceID, ResourceVersion: resourceVersion,
		Kind: kind, NextAt: nextAt.UTC(), CreatedAt: nextAt.UTC(), UpdatedAt: nextAt.UTC(),
	}
	return s.db.WithContext(ctx).Create(&row).Error
}

// EnabledOCRuntime returns the id of the enabled runtime authorizations are
// placed against (phase one: one shared runtime per deployment).
func (s *OCStore) EnabledOCRuntime(ctx context.Context) (string, error) {
	var row OCRuntimeRow
	err := s.db.WithContext(ctx).Where("enabled = ?", true).Order("id").First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", ErrOCRuntimeUnavailable
	}
	if err != nil {
		return "", err
	}
	return row.ID, nil
}
