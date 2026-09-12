package appconnector

import (
	"context"
	"errors"

	appconnector "github.com/Tencent/WeKnora/internal/appconnector"

	"gorm.io/gorm"
)

var (
	// ErrInstallationConflict covers invalid transitions and stale
	// compare-and-swap versions.
	ErrInstallationConflict = errors.New("installation_conflict")
	// ErrReauthorizationRequired is returned when a caller tries to keep an
	// installation active across an upgrade that expands its scope.
	ErrReauthorizationRequired = errors.New("reauthorization_required")
)

// AppVersion is the published catalog entry for one version of an app.
// schema_json describes the requested permission scope; risk_json carries
// the vendor's risk self-assessment.
type AppVersion struct {
	AppID      string `gorm:"primaryKey;column:app_id"`
	Version    string `gorm:"primaryKey;column:version"`
	SchemaJSON string `gorm:"column:schema_json;not null;default:''"`
	RiskJSON   string `gorm:"column:risk_json;not null;default:''"`
}

func (AppVersion) TableName() string { return "app_versions" }

// InstallationRow persists one tenant's installation of an app. config_json
// holds non-secret settings only — credentials are never stored here; they
// live behind connections.credential_ref in the credential store. Version is
// the optimistic-concurrency counter consumed by ApplyInstallation's CAS.
type InstallationRow struct {
	ID         string `gorm:"primaryKey;column:id"`
	TenantID   uint64 `gorm:"column:tenant_id;not null;uniqueIndex:uq_installations_tenant_app"`
	AppID      string `gorm:"column:app_id;not null;uniqueIndex:uq_installations_tenant_app"`
	AppVersion string `gorm:"column:app_version;not null"`
	State      string `gorm:"column:state;not null"`
	ConfigJSON string `gorm:"column:config_json;not null;default:''"`
	Version    int64  `gorm:"column:version;not null;default:1"`
}

func (InstallationRow) TableName() string { return "installations" }

// ConnectionRow persists a connection bound to an installation. The
// (tenant_id, id) primary key enforces the tenant+id uniqueness.
type ConnectionRow struct {
	TenantID       uint64 `gorm:"primaryKey;column:tenant_id"`
	ID             string `gorm:"primaryKey;column:id"`
	InstallationID string `gorm:"column:installation_id;not null"`
	Kind           string `gorm:"column:kind;not null"`
	OwnerID        string `gorm:"column:owner_id;not null;default:''"`
	CredentialRef  string `gorm:"column:credential_ref;not null;default:''"`
	State          string `gorm:"column:state;not null"`
	AuthVersion    int64  `gorm:"column:auth_version;not null;default:1"`
}

func (ConnectionRow) TableName() string { return "connections" }

type InstallationStore struct{ db *gorm.DB }

func NewInstallationStore(db *gorm.DB) *InstallationStore { return &InstallationStore{db: db} }

func validInstallationState(s string) bool {
	return s == appconnector.InstallationActive ||
		s == appconnector.InstallationDisabled ||
		s == appconnector.InstallationReauthorizationRequired
}

// ApplyInstallation creates or transitions a tenant's installation of an app
// under compare-and-swap semantics. expected is the row version the caller
// believes is current; expected 0 means "no installation exists yet". After
// a successful write the target row is read back and must show the requested
// version and state.
//
// Two transitions are refused outright:
//   - an upgrade whose app_versions.schema_json differs from the installed
//     one (a scope expansion) may only land in reauthorization_required,
//     never active;
//   - an existing non-active installation may not be flipped back to active
//     here — reauthorization completes per-connection, not via this method.
func (s *InstallationStore) ApplyInstallation(ctx context.Context, i appconnector.Installation, expected int64) error {
	if i.TenantID == 0 || i.ID == "" || i.AppID == "" || i.Version == "" || !validInstallationState(i.State) {
		return ErrInstallationConflict
	}
	var cur InstallationRow
	err := s.db.WithContext(ctx).Where("tenant_id = ? AND app_id = ?", i.TenantID, i.AppID).First(&cur).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if expected != 0 {
			return ErrInstallationConflict
		}
		row := InstallationRow{ID: i.ID, TenantID: i.TenantID, AppID: i.AppID, AppVersion: i.Version, State: i.State, Version: 1}
		if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
			return err
		}
		return s.readBack(ctx, i, 1)
	}
	if err != nil {
		return err
	}
	if cur.Version != expected {
		return ErrInstallationConflict
	}
	target := i.State
	if target == appconnector.InstallationActive && cur.State != appconnector.InstallationActive {
		return ErrInstallationConflict
	}
	if i.Version != cur.AppVersion {
		expanded, err := s.scopeExpanded(ctx, cur.AppID, cur.AppVersion, i.Version)
		if err != nil {
			return err
		}
		if expanded && target != appconnector.InstallationReauthorizationRequired {
			return ErrReauthorizationRequired
		}
	}
	res := s.db.WithContext(ctx).Model(&InstallationRow{}).
		Where("id = ? AND tenant_id = ? AND version = ?", cur.ID, cur.TenantID, cur.Version).
		Updates(map[string]interface{}{"app_version": i.Version, "state": target, "version": cur.Version + 1})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrInstallationConflict
	}
	return s.readBack(ctx, i, cur.Version+1)
}

// scopeExpanded compares the published schema (requested scope) of the
// installed version with the target version. A missing catalog entry is
// treated as expanded: an unknown scope can never silently inherit the old
// authorization.
func (s *InstallationStore) scopeExpanded(ctx context.Context, app, from, to string) (bool, error) {
	if from == to {
		return false, nil
	}
	var fromRow, toRow AppVersion
	fromErr := s.db.WithContext(ctx).Where("app_id = ? AND version = ?", app, from).First(&fromRow).Error
	toErr := s.db.WithContext(ctx).Where("app_id = ? AND version = ?", app, to).First(&toRow).Error
	if fromErr != nil {
		if errors.Is(fromErr, gorm.ErrRecordNotFound) {
			return true, nil
		}
		return false, fromErr
	}
	if toErr != nil {
		if errors.Is(toErr, gorm.ErrRecordNotFound) {
			return true, nil
		}
		return false, toErr
	}
	return fromRow.SchemaJSON != toRow.SchemaJSON, nil
}

// readBack re-reads the row after a write and verifies it landed on the
// requested app version, matching state and the expected CAS generation.
func (s *InstallationStore) readBack(ctx context.Context, i appconnector.Installation, wantVersion int64) error {
	var got InstallationRow
	if err := s.db.WithContext(ctx).Where("tenant_id = ? AND app_id = ?", i.TenantID, i.AppID).First(&got).Error; err != nil {
		return err
	}
	if got.AppVersion != i.Version || got.State != i.State || got.Version != wantVersion {
		return ErrInstallationConflict
	}
	return nil
}

// GetInstallation returns the domain projection of a tenant's installation
// together with its current CAS version.
func (s *InstallationStore) GetInstallation(ctx context.Context, tenant uint64, app string) (appconnector.Installation, int64, error) {
	var row InstallationRow
	if err := s.db.WithContext(ctx).Where("tenant_id = ? AND app_id = ?", tenant, app).First(&row).Error; err != nil {
		return appconnector.Installation{}, 0, err
	}
	return appconnector.Installation{ID: row.ID, AppID: row.AppID, Version: row.AppVersion, State: row.State, TenantID: row.TenantID}, row.Version, nil
}

// SaveConnection upserts a connection for a tenant. The credential itself is
// never an argument — only its reference.
func (s *InstallationStore) SaveConnection(ctx context.Context, c appconnector.Connection) error {
	if c.TenantID == 0 || c.ID == "" || c.InstallationID == "" || c.State == "" {
		return ErrInstallationConflict
	}
	row := ConnectionRow{TenantID: c.TenantID, ID: c.ID, InstallationID: c.InstallationID, Kind: c.Kind, OwnerID: c.OwnerID, CredentialRef: c.CredentialRef, State: c.State, AuthVersion: c.AuthVersion}
	return s.db.WithContext(ctx).Save(&row).Error
}

// GetConnection loads a connection by tenant and id.
func (s *InstallationStore) GetConnection(ctx context.Context, tenant uint64, id string) (appconnector.Connection, error) {
	var row ConnectionRow
	if err := s.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenant, id).First(&row).Error; err != nil {
		return appconnector.Connection{}, err
	}
	return appconnector.Connection{ID: row.ID, InstallationID: row.InstallationID, Kind: row.Kind, OwnerID: row.OwnerID, CredentialRef: row.CredentialRef, State: row.State, TenantID: row.TenantID, AuthVersion: row.AuthVersion}, nil
}

// ConnectionUsable answers the runtime question "may this actor make a new
// call through this connection": the owning installation must be active, and
// then appconnector.CanUseConnection decides on tenant scope, kind, state
// and grant. A disabled (or reauthorization-parked) installation therefore
// rejects every new call even for otherwise-authorized actors.
func (s *InstallationStore) ConnectionUsable(ctx context.Context, c appconnector.Connection, actor string, spaceGrant bool) (bool, error) {
	if c.TenantID == 0 || c.InstallationID == "" {
		return false, ErrInstallationConflict
	}
	var inst InstallationRow
	if err := s.db.WithContext(ctx).Where("id = ? AND tenant_id = ?", c.InstallationID, c.TenantID).First(&inst).Error; err != nil {
		return false, err
	}
	if inst.State != appconnector.InstallationActive {
		return false, nil
	}
	return appconnector.CanUseConnection(c, c.TenantID, actor, spaceGrant), nil
}
