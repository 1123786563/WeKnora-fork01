package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	appconnector "github.com/Tencent/WeKnora/internal/appconnector"
	appconnectorrepo "github.com/Tencent/WeKnora/internal/application/repository/appconnector"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// mcpOAuthRepository implements interfaces.MCPOAuthRepository.
type mcpOAuthRepository struct {
	db *gorm.DB
}

// NewMCPOAuthRepository creates a new MCP OAuth repository.
func NewMCPOAuthRepository(db *gorm.DB) interfaces.MCPOAuthRepository {
	return &mcpOAuthRepository{db: db}
}

func (r *mcpOAuthRepository) GetClient(
	ctx context.Context, tenantID uint64, serviceID string,
) (*types.MCPOAuthClient, error) {
	var client types.MCPOAuthClient
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND service_id = ?", tenantID, serviceID).
		First(&client).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &client, nil
}

func (r *mcpOAuthRepository) SaveClient(ctx context.Context, client *types.MCPOAuthClient) error {
	client.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "service_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"client_id", "client_secret", "redirect_uri", "updated_at"}),
		}).
		Create(client).Error
}

func (r *mcpOAuthRepository) DeleteClient(ctx context.Context, tenantID uint64, serviceID string) error {
	return r.db.WithContext(ctx).
		Where("tenant_id = ? AND service_id = ?", tenantID, serviceID).
		Delete(&types.MCPOAuthClient{}).Error
}

func (r *mcpOAuthRepository) GetToken(
	ctx context.Context, tenantID uint64, userID, serviceID string,
) (*types.MCPOAuthToken, error) {
	return r.GetTokenForPrincipal(ctx, tenantID, types.Principal{
		Type: types.PrincipalWebUser,
		ID:   userID,
	}, serviceID)
}

func (r *mcpOAuthRepository) GetTokenForPrincipal(
	ctx context.Context, tenantID uint64, principal types.Principal, serviceID string,
) (*types.MCPOAuthToken, error) {
	principal = principal.Normalize()
	if !principal.Valid() {
		return nil, nil
	}
	var token types.MCPOAuthToken
	err := r.db.WithContext(ctx).
		Where(
			"tenant_id = ? AND principal_type = ? AND principal_id = ? AND service_id = ?",
			tenantID, principal.Type, principal.ID, serviceID,
		).
		First(&token).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &token, nil
}

func (r *mcpOAuthRepository) SaveToken(ctx context.Context, token *types.MCPOAuthToken) error {
	if token.PrincipalType == "" || token.PrincipalID == "" {
		token.PrincipalType = types.PrincipalWebUser
		token.PrincipalID = token.UserID
	}
	return r.SaveTokenForPrincipal(ctx, token)
}

func (r *mcpOAuthRepository) SaveTokenForPrincipal(ctx context.Context, token *types.MCPOAuthToken) error {
	if token.PrincipalType == "" || token.PrincipalID == "" {
		return fmt.Errorf("mcp oauth token requires principal_type and principal_id")
	}
	if token.UserID == "" {
		token.UserID = (types.Principal{Type: token.PrincipalType, ID: token.PrincipalID}).StorageID()
	}
	token.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "tenant_id"},
				{Name: "principal_type"},
				{Name: "principal_id"},
				{Name: "service_id"},
			},
			DoUpdates: clause.AssignmentColumns([]string{
				"user_id", "access_token", "refresh_token", "token_type", "expires_at", "updated_at",
			}),
		}).
		Create(token).Error
}

func (r *mcpOAuthRepository) DeleteToken(
	ctx context.Context, tenantID uint64, userID, serviceID string,
) error {
	return r.DeleteTokenForPrincipal(ctx, tenantID, types.Principal{
		Type: types.PrincipalWebUser,
		ID:   userID,
	}, serviceID)
}

func (r *mcpOAuthRepository) DeleteTokenForPrincipal(
	ctx context.Context, tenantID uint64, principal types.Principal, serviceID string,
) error {
	principal = principal.Normalize()
	if !principal.Valid() {
		return nil
	}
	return r.db.WithContext(ctx).
		Where(
			"tenant_id = ? AND principal_type = ? AND principal_id = ? AND service_id = ?",
			tenantID, principal.Type, principal.ID, serviceID,
		).
		Delete(&types.MCPOAuthToken{}).Error
}

func (r *mcpOAuthRepository) TryAcquireTokenRefreshLease(
	ctx context.Context,
	tenantID uint64,
	principal types.Principal,
	serviceID, leaseID string,
	leaseUntil time.Time,
) (bool, error) {
	principal = principal.Normalize()
	if !principal.Valid() || leaseID == "" {
		return false, nil
	}
	now := time.Now()
	result := r.db.WithContext(ctx).
		Model(&types.MCPOAuthToken{}).
		Where(
			"tenant_id = ? AND principal_type = ? AND principal_id = ? AND service_id = ?",
			tenantID, principal.Type, principal.ID, serviceID,
		).
		Where("refresh_lease_until IS NULL OR refresh_lease_until < ?", now).
		Updates(map[string]interface{}{
			"refresh_lease_id":    leaseID,
			"refresh_lease_until": leaseUntil,
		})
	return result.RowsAffected == 1, result.Error
}

func (r *mcpOAuthRepository) ReleaseTokenRefreshLease(
	ctx context.Context,
	tenantID uint64,
	principal types.Principal,
	serviceID, leaseID string,
) error {
	principal = principal.Normalize()
	if !principal.Valid() || leaseID == "" {
		return nil
	}
	return r.db.WithContext(ctx).
		Model(&types.MCPOAuthToken{}).
		Where(
			"tenant_id = ? AND principal_type = ? AND principal_id = ? AND service_id = ? AND refresh_lease_id = ?",
			tenantID, principal.Type, principal.ID, serviceID, leaseID,
		).
		Updates(map[string]interface{}{
			"refresh_lease_id":    "",
			"refresh_lease_until": nil,
		}).Error
}

// ---------------------------------------------------------------------------
// OAuth binding states (A02). Everything below is an additive surface for
// one-time state consumption and credential-reference binding; the
// pre-existing client/token methods above are untouched.
// ---------------------------------------------------------------------------

// CredentialRefPrefix marks a Connection.CredentialRef that points at an
// MCPOAuthToken row ("<prefix><service_id>"). The connection never stores
// credential material — only this reference; the token row itself keeps the
// encrypted-at-rest secrets.
const CredentialRefPrefix = "mcp_oauth_token:"

// Binding-lifecycle errors. Handlers map these to 400/403/404 without
// leaking credential material.
var (
	// ErrOAuthBindingInvalid covers unknown, expired, replayed, wrong-tenant,
	// wrong-actor states and bindings whose installation is unusable.
	ErrOAuthBindingInvalid = errors.New("oauth_binding_invalid")
	// ErrOAuthBindingActorNotMember is returned when the callback initiator
	// no longer holds an active membership in the tenant.
	ErrOAuthBindingActorNotMember = errors.New("oauth_binding_actor_not_member")
	// ErrCredentialRefInvalid is returned when a connection's credential
	// reference does not point at a known credential store shape.
	ErrCredentialRefInvalid = errors.New("credential_ref_invalid")
	// ErrConnectionNotFound is returned when a revoke targets no connection
	// the caller may touch.
	ErrConnectionNotFound = errors.New("connection_not_found")
)

// MCPOAuthBindingRow persists one pending OAuth binding state. Used is the
// one-time consumption flag: a single conditional UPDATE flips it, so two
// concurrent callbacks presenting the same state cannot both win.
type MCPOAuthBindingRow struct {
	State          string    `gorm:"type:varchar(128);primaryKey"`
	TenantID       uint64    `gorm:"not null;index"`
	ActorID        string    `gorm:"type:varchar(512);not null"`
	InstallationID string    `gorm:"type:varchar(36);not null"`
	ServiceID      string    `gorm:"type:varchar(36);not null"`
	ExpiresAt      time.Time `gorm:"not null"`
	Used           bool      `gorm:"not null;default:false"`
	UsedAt         *time.Time
	CreatedAt      time.Time
}

// TableName pins the plural table name used by migration 000118.
func (MCPOAuthBindingRow) TableName() string { return "mcp_oauth_binding_states" }

// MCPOAuthBindingStore consumes one-time OAuth binding states and binds
// connections to installations. It is deliberately separate from the
// MCPOAuthRepository interface so the existing constructor and callers are
// unchanged.
type MCPOAuthBindingStore struct{ db *gorm.DB }

// NewMCPOAuthBindingStore creates the binding store over the same gorm DB.
func NewMCPOAuthBindingStore(db *gorm.DB) *MCPOAuthBindingStore {
	return &MCPOAuthBindingStore{db: db}
}

// IssueBindingState records a freshly minted one-time state for an actor
// starting an OAuth flow. The state value itself is minted by the HTTP layer
// from crypto/rand — it is never accepted from the client.
func (s *MCPOAuthBindingStore) IssueBindingState(
	ctx context.Context, b appconnector.OAuthBinding, serviceID string,
) error {
	if b.State == "" || b.TenantID == 0 || b.ActorID == "" || b.InstallationID == "" ||
		serviceID == "" || !time.Now().Before(b.ExpiresAt) {
		return ErrOAuthBindingInvalid
	}
	return s.db.WithContext(ctx).Create(&MCPOAuthBindingRow{
		State:          b.State,
		TenantID:       b.TenantID,
		ActorID:        b.ActorID,
		InstallationID: b.InstallationID,
		ServiceID:      serviceID,
		ExpiresAt:      b.ExpiresAt,
	}).Error
}

// GetBindingState loads one pending binding row by state WITHOUT consuming
// it. The public OAuth callback uses it to recover the tenant and the
// initiating actor (the state itself is the only credential a provider
// redirect carries) before calling CompleteBinding, which performs the real
// one-time consumption. Unknown states return ErrOAuthBindingInvalid.
func (s *MCPOAuthBindingStore) GetBindingState(ctx context.Context, state string) (MCPOAuthBindingRow, error) {
	var row MCPOAuthBindingRow
	err := s.db.WithContext(ctx).Where("state = ?", state).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return MCPOAuthBindingRow{}, ErrOAuthBindingInvalid
	}
	return row, err
}

// serviceIDFromRef extracts the service id carried inside a credential ref.
func serviceIDFromRef(ref string) (string, bool) {
	if !strings.HasPrefix(ref, CredentialRefPrefix) {
		return "", false
	}
	svc := strings.TrimPrefix(ref, CredentialRefPrefix)
	return svc, svc != ""
}

// connectionRowToDomain projects the persisted row onto the domain struct.
func connectionRowToDomain(row appconnectorrepo.ConnectionRow) appconnector.Connection {
	return appconnector.Connection{
		ID:             row.ID,
		InstallationID: row.InstallationID,
		Kind:           row.Kind,
		OwnerID:        row.OwnerID,
		CredentialRef:  row.CredentialRef,
		State:          row.State,
		TenantID:       row.TenantID,
		AuthVersion:    row.AuthVersion,
	}
}

// CompleteBinding consumes the one-time state and binds the installation to
// the initiator's personal connection in a SINGLE transaction:
//
//  1. load and validate the state (exists, unused, unexpired, same tenant,
//     same actor) via appconnector.ValidateOAuthBinding;
//  2. verify the initiator still holds an ACTIVE membership in the tenant;
//  3. verify the installation exists in this tenant and is active;
//  4. flip Used with a conditional UPDATE — a concurrent replay loses here;
//  5. persist the provider-exchanged token (encrypted at rest by the
//     existing MCPOAuthToken hooks — the existing AES-256-GCM helpers);
//  6. create the personal connection storing ONLY the credential reference.
//
// Any failure rolls the whole transaction back, so a half-bound state can
// never survive. The token must have been obtained by the caller strictly
// per provider-verified callback/PKCE rules; this method never performs the
// HTTP exchange itself.
func (s *MCPOAuthBindingStore) CompleteBinding(
	ctx context.Context,
	tenant uint64,
	state, actor string,
	token *types.MCPOAuthToken,
) (appconnector.Connection, error) {
	var conn appconnector.Connection
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row MCPOAuthBindingRow
		err := tx.Where("state = ?", state).First(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrOAuthBindingInvalid
		}
		if err != nil {
			return err
		}
		binding := appconnector.OAuthBinding{
			State:          row.State,
			InstallationID: row.InstallationID,
			ActorID:        row.ActorID,
			TenantID:       row.TenantID,
			ExpiresAt:      row.ExpiresAt,
			Used:           row.Used,
		}
		if err := appconnector.ValidateOAuthBinding(binding, tenant, actor, time.Now()); err != nil {
			return ErrOAuthBindingInvalid
		}

		// The initiator must still be an active member at callback time.
		var members int64
		if err := tx.Model(&types.TenantMember{}).
			Where("tenant_id = ? AND user_id = ? AND status = ?",
				tenant, actor, types.TenantMemberStatusActive).
			Count(&members).Error; err != nil {
			return err
		}
		if members == 0 {
			return ErrOAuthBindingActorNotMember
		}

		var inst appconnectorrepo.InstallationRow
		err = tx.Where("id = ? AND tenant_id = ?", row.InstallationID, tenant).
			First(&inst).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrOAuthBindingInvalid
		}
		if err != nil {
			return err
		}
		if inst.State != appconnector.InstallationActive {
			return ErrOAuthBindingInvalid
		}

		// One-time consumption inside the same transaction: the conditional
		// UPDATE guarantees exactly one winner even under concurrency.
		res := tx.Model(&MCPOAuthBindingRow{}).
			Where("state = ? AND used = ?", state, false).
			Updates(map[string]interface{}{"used": true, "used_at": time.Now()})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrOAuthBindingInvalid
		}

		// Persist the exchanged token through the existing encryption hooks,
		// mirroring SaveTokenForPrincipal's upsert semantics.
		token.TenantID = tenant
		token.PrincipalType = types.PrincipalWebUser
		token.PrincipalID = actor
		if token.UserID == "" {
			token.UserID = (types.Principal{Type: token.PrincipalType, ID: token.PrincipalID}).StorageID()
		}
		token.ServiceID = row.ServiceID
		token.UpdatedAt = time.Now()
		if err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "tenant_id"},
				{Name: "principal_type"},
				{Name: "principal_id"},
				{Name: "service_id"},
			},
			DoUpdates: clause.AssignmentColumns([]string{
				"user_id", "access_token", "refresh_token", "token_type", "expires_at", "updated_at",
			}),
		}).Create(token).Error; err != nil {
			return err
		}

		conn = appconnector.Connection{
			ID:             uuid.New().String(),
			InstallationID: row.InstallationID,
			Kind:           appconnector.ConnectionKindPersonal,
			OwnerID:        actor,
			CredentialRef:  CredentialRefPrefix + row.ServiceID,
			State:          appconnector.ConnectionActive,
			TenantID:       tenant,
			AuthVersion:    1,
		}
		return tx.Create(&appconnectorrepo.ConnectionRow{
			TenantID:       conn.TenantID,
			ID:             conn.ID,
			InstallationID: conn.InstallationID,
			Kind:           conn.Kind,
			OwnerID:        conn.OwnerID,
			CredentialRef:  conn.CredentialRef,
			State:          conn.State,
			AuthVersion:    conn.AuthVersion,
		}).Error
	})
	if err != nil {
		return appconnector.Connection{}, err
	}
	return conn, nil
}

// RevokePersonalConnections revokes every personal connection owned by
// userID inside tenant and increments its auth_version, so outstanding
// references fail the version check immediately. Space connections are
// deliberately untouched: a space connection never silently becomes anyone's
// personal connection because one member (even an admin) left.
func (s *MCPOAuthBindingStore) RevokePersonalConnections(
	ctx context.Context, tenantID uint64, userID string,
) (int64, error) {
	res := s.db.WithContext(ctx).
		Model(&appconnectorrepo.ConnectionRow{}).
		Where(
			"tenant_id = ? AND owner_id = ? AND kind = ? AND state <> ?",
			tenantID, userID, appconnector.ConnectionKindPersonal, appconnector.ConnectionRevoked,
		).
		Updates(map[string]interface{}{
			"state":        appconnector.ConnectionRevoked,
			"auth_version": gorm.Expr("auth_version + 1"),
		})
	return res.RowsAffected, res.Error
}

// RevokeConnection revokes one connection after an explicit owner request.
// Personal connections may only be revoked by their owner; space connections
// are governed by route-level RBAC and accepted here for any authenticated
// caller that reached this code. The auth_version bump invalidates every
// outstanding credential reference immediately.
func (s *MCPOAuthBindingStore) RevokeConnection(
	ctx context.Context, tenantID uint64, actor, connectionID string,
) error {
	res := s.db.WithContext(ctx).
		Model(&appconnectorrepo.ConnectionRow{}).
		Where(
			"tenant_id = ? AND id = ? AND state <> ? AND (kind = ? OR owner_id = ?)",
			tenantID, connectionID, appconnector.ConnectionRevoked,
			appconnector.ConnectionKindSpace, actor,
		).
		Updates(map[string]interface{}{
			"state":        appconnector.ConnectionRevoked,
			"auth_version": gorm.Expr("auth_version + 1"),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrConnectionNotFound
	}
	return nil
}

// FindConnectionByID loads a connection by id alone; the tenant is taken
// from the row, which is what the dispatch-time resolver needs.
func (s *MCPOAuthBindingStore) FindConnectionByID(
	ctx context.Context, connectionID string,
) (appconnector.Connection, error) {
	var row appconnectorrepo.ConnectionRow
	if err := s.db.WithContext(ctx).Where("id = ?", connectionID).
		First(&row).Error; err != nil {
		return appconnector.Connection{}, err
	}
	return connectionRowToDomain(row), nil
}

// LoadCredential returns the decrypted credential bytes behind a
// connection's credential reference. The reference is resolved to the
// (tenant, owner, service) token row whose secrets are encrypted at rest by
// the existing helpers. The result is for the calling adapter ONLY and must
// never be serialized into a response or model context.
func (s *MCPOAuthBindingStore) LoadCredential(
	ctx context.Context, c appconnector.Connection,
) ([]byte, error) {
	serviceID, ok := serviceIDFromRef(c.CredentialRef)
	if !ok {
		return nil, ErrCredentialRefInvalid
	}
	var token types.MCPOAuthToken
	err := s.db.WithContext(ctx).
		Where(
			"tenant_id = ? AND principal_type = ? AND principal_id = ? AND service_id = ?",
			c.TenantID, types.PrincipalWebUser, c.OwnerID, serviceID,
		).
		First(&token).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrCredentialRefInvalid
	}
	if err != nil {
		return nil, err
	}
	return []byte(token.AccessToken), nil
}

// MemberActive reports whether userID currently holds an active membership
// in tenant. Soft-deleted rows are excluded by gorm's soft-delete scope.
func (s *MCPOAuthBindingStore) MemberActive(
	ctx context.Context, tenantID uint64, userID string,
) (bool, error) {
	var members int64
	if err := s.db.WithContext(ctx).
		Model(&types.TenantMember{}).
		Where("tenant_id = ? AND user_id = ? AND status = ?",
			tenantID, userID, types.TenantMemberStatusActive).
		Count(&members).Error; err != nil {
		return false, err
	}
	return members > 0, nil
}

// TryAcquireRefreshLease reuses the EXISTING per-token refresh lease so that
// exactly one refresher presents a rotating refresh token to the provider.
func (s *MCPOAuthBindingStore) TryAcquireRefreshLease(
	ctx context.Context, c appconnector.Connection, leaseID string, until time.Time,
) (bool, error) {
	serviceID, ok := serviceIDFromRef(c.CredentialRef)
	if !ok {
		return false, ErrCredentialRefInvalid
	}
	return NewMCPOAuthRepository(s.db).TryAcquireTokenRefreshLease(
		ctx, c.TenantID,
		types.Principal{Type: types.PrincipalWebUser, ID: c.OwnerID},
		serviceID, leaseID, until,
	)
}

// ReleaseRefreshLease releases the refresh lease only when leaseID owns it.
func (s *MCPOAuthBindingStore) ReleaseRefreshLease(
	ctx context.Context, c appconnector.Connection, leaseID string,
) error {
	serviceID, ok := serviceIDFromRef(c.CredentialRef)
	if !ok {
		return ErrCredentialRefInvalid
	}
	return NewMCPOAuthRepository(s.db).ReleaseTokenRefreshLease(
		ctx, c.TenantID,
		types.Principal{Type: types.PrincipalWebUser, ID: c.OwnerID},
		serviceID, leaseID,
	)
}
