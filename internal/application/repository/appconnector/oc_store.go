package appconnector

import (
	"context"
	"errors"
	"time"

	appconnector "github.com/Tencent/WeKnora/internal/appconnector"

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
