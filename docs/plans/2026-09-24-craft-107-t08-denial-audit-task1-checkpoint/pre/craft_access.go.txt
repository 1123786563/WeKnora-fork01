package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/Tencent/WeKnora/internal/types"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CraftAccessService is the persistent Task ACL. Each read checks the
// current session owner, tenant membership, and explicit Task grant.
type CraftAccessService struct{ db *gorm.DB }

var _ craft.TaskAccessChecker = (*CraftAccessService)(nil)

func NewCraftAccessService(db *gorm.DB) *CraftAccessService { return &CraftAccessService{db: db} }

type craftAccessSession struct {
	ID, UserID string
	TenantID   uint64
}

func (craftAccessSession) TableName() string { return "sessions" }

type craftTaskGrant struct {
	TenantID     uint64         `gorm:"primaryKey"`
	SessionID    string         `gorm:"primaryKey"`
	UserID       string         `gorm:"primaryKey"`
	MembershipID uint64         `gorm:"not null"`
	Role         craft.TaskRole `gorm:"type:varchar(16);not null"`
	GrantedBy    string         `gorm:"not null"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (craftTaskGrant) TableName() string { return "craft_task_grants" }

type craftAccessAudit struct {
	TenantID     uint64
	ActorUserID  string
	Action       string
	ScopeType    string
	ScopeID      string
	TargetType   string
	TargetID     string
	TargetUserID string
	Outcome      string
	Details      types.JSON `gorm:"type:jsonb"`
	CreatedAt    time.Time
}

func (craftAccessAudit) TableName() string { return "audit_logs" }

func (s *CraftAccessService) session(ctx context.Context, scope craft.Scope) (craftAccessSession, error) {
	if s == nil || s.db == nil || scope.TenantID == 0 || scope.UserID == "" || scope.SessionID == "" {
		return craftAccessSession{}, craft.ErrForbidden
	}
	var session craftAccessSession
	err := s.db.WithContext(ctx).Table("sessions AS s").
		Select("s.id, s.tenant_id, s.user_id").
		Joins("JOIN craft_sessions AS c ON c.session_id = s.id AND c.tenant_id = s.tenant_id").
		Where("s.tenant_id = ? AND s.id = ? AND s.deleted_at IS NULL", scope.TenantID, scope.SessionID).
		Take(&session).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return craftAccessSession{}, craft.ErrNotFound
	}
	return session, err
}

// activeMemberID returns the single active membership incarnation for a
// tenant/user pair. Ambiguous membership state fails closed.
func activeMemberID(ctx context.Context, db *gorm.DB, tenantID uint64, userID string) (uint64, error) {
	var ids []uint64
	err := db.WithContext(ctx).Table("tenant_members").
		Where("tenant_id = ? AND user_id = ? AND status = ? AND deleted_at IS NULL", tenantID, userID, "active").
		Order("id").Pluck("id", &ids).Error
	if err != nil {
		return 0, err
	}
	if len(ids) != 1 || ids[0] == 0 {
		return 0, craft.ErrForbidden
	}
	return ids[0], nil
}

// Role reports only explicit Task authority for an active tenant member.
func (s *CraftAccessService) Role(ctx context.Context, scope craft.Scope) (craft.TaskRole, error) {
	session, err := s.session(ctx, scope)
	if err != nil {
		return "", err
	}
	membershipID, err := activeMemberID(ctx, s.db, scope.TenantID, scope.UserID)
	if err != nil {
		return "", err
	}
	if session.UserID == scope.UserID {
		return craft.TaskRoleOwner, nil
	}
	var grant craftTaskGrant
	err = s.db.WithContext(ctx).Where("tenant_id = ? AND session_id = ? AND user_id = ? AND membership_id = ?", scope.TenantID, scope.SessionID, scope.UserID, membershipID).First(&grant).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", craft.ErrForbidden
	}
	return grant.Role, err
}

func (s *CraftAccessService) CheckTaskAccess(ctx context.Context, scope craft.Scope, action craft.TaskAction) error {
	role, err := s.Role(ctx, scope)
	if err != nil {
		return err
	}
	if !role.AllowsTaskAction(action) {
		return craft.ErrForbidden
	}
	return nil
}

// IsCraftTask classifies an existing active session from the durable
// tenant-scoped Craft registration, without using actor grants or snapshot
// metadata. A missing, deleted, or tenant-mismatched session is an error so
// callers cannot mistake inconsistent state for a generic session. Database
// errors propagate so callers can fail closed instead of treating an
// unavailable lookup as a generic session.
func (s *CraftAccessService) IsCraftTask(ctx context.Context, tenantID uint64, sessionID string) (bool, error) {
	if s == nil || s.db == nil || tenantID == 0 || strings.TrimSpace(sessionID) == "" {
		return false, craft.ErrForbidden
	}
	var sessionCount int64
	err := s.db.WithContext(ctx).Table("sessions").
		Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", tenantID, sessionID).
		Limit(1).Count(&sessionCount).Error
	if err != nil {
		return false, err
	}
	if sessionCount == 0 {
		return false, craft.ErrNotFound
	}
	var registrationCount int64
	err = s.db.WithContext(ctx).Table("craft_sessions").
		Where("tenant_id = ? AND session_id = ?", tenantID, sessionID).
		Limit(1).Count(&registrationCount).Error
	if err != nil {
		return false, err
	}
	return registrationCount > 0, nil
}

func (s *CraftAccessService) owner(ctx context.Context, scope craft.Scope) error {
	return s.CheckTaskAccess(ctx, scope, craft.TaskShare)
}

// Grant upserts one Collaborator/Viewer only after a fresh owner and active
// same-tenant member check. Audit and grant commit together.
func (s *CraftAccessService) Grant(ctx context.Context, scope craft.Scope, userID string, role craft.TaskRole) error {
	userID = strings.TrimSpace(userID)
	if userID == "" || !role.Grantable() {
		return craft.ErrInvalidInput
	}
	if err := s.owner(ctx, scope); err != nil {
		return err
	}
	if userID == scope.UserID {
		return craft.ErrInvalidInput
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		membershipID, err := activeMemberID(ctx, tx, scope.TenantID, userID)
		if err != nil {
			return err
		}
		grant := craftTaskGrant{TenantID: scope.TenantID, SessionID: scope.SessionID, UserID: userID, MembershipID: membershipID, Role: role, GrantedBy: scope.UserID}
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "tenant_id"}, {Name: "session_id"}, {Name: "user_id"}}, DoUpdates: clause.AssignmentColumns([]string{"membership_id", "role", "granted_by", "updated_at"})}).Create(&grant).Error; err != nil {
			return err
		}
		return tx.Create(&craftAccessAudit{TenantID: scope.TenantID, ActorUserID: scope.UserID, Action: "craft.member_added", ScopeType: "session", ScopeID: scope.SessionID, TargetType: "task_member", TargetID: scope.SessionID, TargetUserID: userID, Outcome: "success", Details: types.JSON(fmt.Sprintf(`{"role":%q}`, role)), CreatedAt: time.Now()}).Error
	})
}

// Revoke deletes the live grant; historical grant/revoke events stay in audit.
func (s *CraftAccessService) Revoke(ctx context.Context, scope craft.Scope, userID string) error {
	userID = strings.TrimSpace(userID)
	if userID == "" || userID == scope.UserID {
		return craft.ErrInvalidInput
	}
	if err := s.owner(ctx, scope); err != nil {
		return err
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Where("tenant_id = ? AND session_id = ? AND user_id = ?", scope.TenantID, scope.SessionID, userID).Delete(&craftTaskGrant{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return craft.ErrNotFound
		}
		return tx.Create(&craftAccessAudit{TenantID: scope.TenantID, ActorUserID: scope.UserID, Action: "craft.member_revoked", ScopeType: "session", ScopeID: scope.SessionID, TargetType: "task_member", TargetID: scope.SessionID, TargetUserID: userID, Outcome: "success", Details: types.JSON(`{}`), CreatedAt: time.Now()}).Error
	})
}

func (s *CraftAccessService) ListMembers(ctx context.Context, scope craft.Scope) ([]craft.TaskMember, error) {
	if err := s.CheckTaskAccess(ctx, scope, craft.TaskRead); err != nil {
		return nil, err
	}
	var grants []craftTaskGrant
	if err := s.db.WithContext(ctx).Table("craft_task_grants AS g").
		Select("g.tenant_id, g.session_id, g.user_id, g.membership_id, g.role, g.granted_by, g.created_at, g.updated_at").
		Joins("JOIN tenant_members AS m ON m.id = g.membership_id AND m.tenant_id = g.tenant_id AND m.user_id = g.user_id AND m.status = ? AND m.deleted_at IS NULL", "active").
		Where("g.tenant_id = ? AND g.session_id = ?", scope.TenantID, scope.SessionID).Order("g.user_id").Find(&grants).Error; err != nil {
		return nil, err
	}
	result := make([]craft.TaskMember, 0, len(grants)+1)
	session, err := s.session(ctx, scope)
	if err != nil {
		return nil, err
	}
	result = append(result, craft.TaskMember{UserID: session.UserID, Role: craft.TaskRoleOwner})
	for _, grant := range grants {
		result = append(result, craft.TaskMember{UserID: grant.UserID, Role: grant.Role})
	}
	return result, nil
}

// ListAccessibleTaskIDs is a server-side filter for the Craft task listing.
// Revocation removes a Task from the next result even when a caller has
// tenant admin authority. T20 applies this filter to the central list route.
func (s *CraftAccessService) ListAccessibleTaskIDs(ctx context.Context, tenantID uint64, userID string) ([]string, error) {
	if s == nil || s.db == nil || tenantID == 0 || userID == "" {
		return nil, craft.ErrForbidden
	}
	_, err := activeMemberID(ctx, s.db, tenantID, userID)
	if err != nil {
		return nil, err
	}
	var ids []string
	err = s.db.WithContext(ctx).Table("sessions AS s").Distinct("s.id").
		Joins("JOIN craft_sessions AS c ON c.session_id = s.id AND c.tenant_id = s.tenant_id").
		Joins("JOIN tenant_members AS m ON m.tenant_id = s.tenant_id AND m.user_id = ? AND m.status = ? AND m.deleted_at IS NULL", userID, "active").
		Joins("LEFT JOIN craft_task_grants AS g ON g.tenant_id = s.tenant_id AND g.session_id = s.id AND g.user_id = ? AND g.membership_id = m.id", userID).
		Where("s.tenant_id = ? AND s.deleted_at IS NULL AND (s.user_id = ? OR g.user_id IS NOT NULL)", tenantID, userID).
		Order("s.id").Pluck("s.id", &ids).Error
	if err != nil {
		return nil, fmt.Errorf("list accessible craft tasks: %w", err)
	}
	return ids, nil
}
