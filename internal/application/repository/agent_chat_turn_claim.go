package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ReplayState string

const (
	ReplayNew      ReplayState = "new"
	ReplayActive   ReplayState = "active"
	ReplayTerminal ReplayState = "terminal"
)

var (
	ErrAgentChatTurnClaimConflict           = errors.New("agent chat turn request conflicts with existing claim")
	ErrAgentChatTurnClaimFenced             = errors.New("agent chat turn claim lease is fenced")
	ErrAgentChatTurnClaimPlaceholderMissing = errors.New("agent chat turn claim assistant placeholder is missing or mismatched")
)

const agentChatTurnLease = 30 * time.Second

type AgentChatTurnClaimInput struct {
	SourceTenantID       uint64
	SessionTenantID      uint64
	SessionID            string
	OwnerID              string
	RequestID            string
	RequestHash          string
	LeaseOwner           string
	AgentID              string
	AssistantPlaceholder *types.Message
}

type AgentChatTurnClaim = types.AgentChatTurnClaimEntity

type AgentChatTurnClaimStore interface {
	Admit(context.Context, AgentChatTurnClaimInput) (AgentChatTurnClaim, ReplayState, error)
	Renew(context.Context, uint64, string, uint64, string) error
	GetForHandler(context.Context, uint64, string, uint64, string) (AgentChatTurnClaim, bool, error)
	Finish(context.Context, uint64, string, uint64, string, *types.Message, string, string) (bool, error)
	CreateUserMessage(context.Context, uint64, string, uint64, string, *types.Message) (string, error)
	UpdateMessage(context.Context, uint64, string, uint64, string, *types.Message) error
	CancelByOwner(context.Context, uint64, string, string, string, string) (AgentChatTurnClaim, bool, error)
}

type AgentChatTurnClaimRepository struct{ db *gorm.DB }

func NewAgentChatTurnClaimRepository(db *gorm.DB) *AgentChatTurnClaimRepository {
	return &AgentChatTurnClaimRepository{db: db}
}

func (r *AgentChatTurnClaimRepository) Admit(ctx context.Context, in AgentChatTurnClaimInput) (AgentChatTurnClaim, ReplayState, error) {
	if in.SourceTenantID == 0 || in.SessionTenantID == 0 || in.SessionID == "" || in.OwnerID == "" || in.RequestID == "" || len(in.RequestID) > 128 || in.RequestHash == "" || in.LeaseOwner == "" || in.AgentID == "" || in.AssistantPlaceholder == nil || in.AssistantPlaceholder.ID != "" {
		return AgentChatTurnClaim{}, "", errors.New("invalid agent chat turn claim input")
	}
	var result AgentChatTurnClaim
	var replay ReplayState
	err := withTenantSecurityGuards(ctx, r.db, []uint64{in.SourceTenantID, in.SessionTenantID}, func(tx *gorm.DB) error {
		var existing AgentChatTurnClaim
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("session_tenant_id = ? AND session_id = ? AND owner_id = ? AND request_id = ?", in.SessionTenantID, in.SessionID, in.OwnerID, in.RequestID).Take(&existing).Error
		if err == nil {
			if existing.SourceTenantID != in.SourceTenantID {
				return gorm.ErrRecordNotFound
			}
			if existing.RequestHash != in.RequestHash {
				return ErrAgentChatTurnClaimConflict
			}
			result = existing
			if existing.State == "active" {
				replay = ReplayActive
			} else {
				replay = ReplayTerminal
			}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var session types.Session
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id = ? AND user_id = ?", in.SessionTenantID, in.SessionID, in.OwnerID).Take(&session).Error; err != nil {
			return err
		}
		if in.SourceTenantID != in.SessionTenantID {
			var grantIDs []string
			grantQuery := "SELECT s.id FROM agent_shares s JOIN organization_tenant_members m ON m.organization_id = s.organization_id WHERE s.source_tenant_id = ? AND s.agent_id = ? AND s.deleted_at IS NULL AND m.tenant_id = ? AND m.joined_at IS NOT NULL ORDER BY s.id"
			if tx.Dialector.Name() == "postgres" {
				grantQuery += " FOR UPDATE OF s, m"
			}
			err := tx.Raw(grantQuery, in.SourceTenantID, in.AgentID, in.SessionTenantID).Scan(&grantIDs).Error
			if err != nil {
				return err
			}
			if len(grantIDs) == 0 {
				return gorm.ErrRecordNotFound
			}
		}
		// Expired work is never resumed. Terminalize a bounded batch and its
		// orphan assistant placeholders before admitting an explicit new turn.
		var expired []AgentChatTurnClaim
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("session_tenant_id = ? AND session_id = ? AND state = 'active' AND lease_expires_at <= ?", in.SessionTenantID, in.SessionID, time.Now().UTC()).Order("lease_expires_at ASC").Limit(100).Find(&expired).Error; err != nil {
			return err
		}
		for _, old := range expired {
			now := time.Now().UTC()
			transition := tx.Model(&AgentChatTurnClaim{}).Where("id = ? AND source_tenant_id = ? AND state = 'active' AND lease_expires_at <= ?", old.ID, old.SourceTenantID, now).Updates(map[string]any{"state": "failed", "reason": "claim lease expired", "generation": gorm.Expr("generation + 1"), "updated_at": now})
			if transition.Error != nil {
				return transition.Error
			}
			if transition.RowsAffected != 1 {
				continue
			}
			if err := terminalizeClaimAssistantPlaceholderTx(tx, old, now); err != nil {
				return err
			}
		}
		var variant types.AgentAdoptionVariantEntity
		var versionID string
		err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND local_agent_id = ? AND state = 'published'", in.SourceTenantID, in.AgentID).Take(&variant).Error
		if err == nil {
			versionID = variant.LocalAgentVersionID
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		releaseID, adopted, err := checkLocalAgentReleaseAdmissionTx(tx, in.SourceTenantID, in.AgentID, versionID)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		claim := AgentChatTurnClaim{ID: uuid.NewString(), SourceTenantID: in.SourceTenantID, SessionTenantID: in.SessionTenantID, SessionID: in.SessionID, OwnerID: in.OwnerID, RequestID: in.RequestID, RequestHash: in.RequestHash, AgentID: in.AgentID, State: "active", LeaseOwner: in.LeaseOwner, LeaseExpiresAt: now.Add(agentChatTurnLease), Generation: 1, CreatedAt: now, UpdatedAt: now}
		if adopted {
			claim.LocalAgentVersionID, claim.ReleaseID = &versionID, &releaseID
		}
		placeholder := *in.AssistantPlaceholder
		placeholder.ID, placeholder.SessionID, placeholder.RequestID, placeholder.Role = "", in.SessionID, in.RequestID, "assistant"
		placeholder.CreatedAt, placeholder.UpdatedAt = now, now
		if err := tx.Create(&placeholder).Error; err != nil {
			return err
		}
		claim.AssistantMessageID = placeholder.ID
		if err := tx.Create(&claim).Error; err != nil {
			return err
		}
		result, replay = claim, ReplayNew
		return nil
	})
	return result, replay, err
}

func (r *AgentChatTurnClaimRepository) lockedClaim(tx *gorm.DB, sourceTenantID uint64, id string, generation uint64, owner string) (AgentChatTurnClaim, error) {
	var row AgentChatTurnClaim
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("source_tenant_id = ? AND id = ?", sourceTenantID, id).Take(&row).Error
	if err != nil {
		return row, err
	}
	if row.Generation != generation || row.LeaseOwner != owner || row.State != "active" || !row.LeaseExpiresAt.After(time.Now().UTC()) {
		return row, ErrAgentChatTurnClaimFenced
	}
	return row, nil
}

func (r *AgentChatTurnClaimRepository) Renew(ctx context.Context, sourceTenantID uint64, id string, generation uint64, owner string) error {
	return withTenantSecurityGuard(ctx, r.db, sourceTenantID, func(tx *gorm.DB) error {
		row, err := r.lockedClaim(tx, sourceTenantID, id, generation, owner)
		if err != nil {
			return err
		}
		res := tx.Model(&AgentChatTurnClaim{}).Where("id = ? AND source_tenant_id = ? AND generation = ? AND lease_owner = ? AND state = 'active' AND lease_expires_at > ?", row.ID, sourceTenantID, generation, owner, time.Now().UTC()).Updates(map[string]any{"lease_expires_at": time.Now().UTC().Add(agentChatTurnLease), "updated_at": time.Now().UTC()})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return ErrAgentChatTurnClaimFenced
		}
		return nil
	})
}

func (r *AgentChatTurnClaimRepository) GetForHandler(ctx context.Context, sourceTenantID uint64, id string, generation uint64, owner string) (AgentChatTurnClaim, bool, error) {
	var row AgentChatTurnClaim
	err := r.db.WithContext(ctx).Where("source_tenant_id = ? AND id = ?", sourceTenantID, id).Take(&row).Error
	if err != nil {
		return row, false, err
	}
	return row, row.Generation == generation && row.LeaseOwner == owner && row.State == "active" && row.LeaseExpiresAt.After(time.Now().UTC()), nil
}

func (r *AgentChatTurnClaimRepository) fencedSession(tx *gorm.DB, row AgentChatTurnClaim) error {
	var session types.Session
	return tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id = ? AND user_id = ?", row.SessionTenantID, row.SessionID, row.OwnerID).Take(&session).Error
}

func updateClaimMessage(tx *gorm.DB, target *types.Message, source *types.Message) error {
	target.Content, target.KnowledgeReferences, target.AgentSteps = source.Content, source.KnowledgeReferences, source.AgentSteps
	target.Images, target.Attachments, target.Artifacts = source.Images, source.Attachments, source.Artifacts
	target.IsCompleted, target.IsFallback, target.AgentDurationMs = source.IsCompleted, source.IsFallback, source.AgentDurationMs
	target.Usage, target.RenderedContent, target.UpdatedAt = source.Usage, source.RenderedContent, time.Now().UTC()
	return tx.Save(target).Error
}

func terminalizeClaimAssistantPlaceholderTx(tx *gorm.DB, claim AgentChatTurnClaim, now time.Time) error {
	result := tx.Model(&types.Message{}).
		Where("id = ? AND session_id = ? AND request_id = ? AND role = 'assistant' AND EXISTS (SELECT 1 FROM sessions s WHERE s.id = messages.session_id AND s.tenant_id = ? AND s.user_id = ?)",
			claim.AssistantMessageID, claim.SessionID, claim.RequestID, claim.SessionTenantID, claim.OwnerID).
		Updates(map[string]any{"is_completed": true, "updated_at": now})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrAgentChatTurnClaimPlaceholderMissing
	}
	return nil
}

func (r *AgentChatTurnClaimRepository) Finish(ctx context.Context, sourceTenantID uint64, id string, generation uint64, owner string, assistantMessage *types.Message, terminalState, reason string) (bool, error) {
	if assistantMessage == nil || (terminalState != "completed" && terminalState != "failed") {
		return false, errors.New("invalid terminal assistant message")
	}
	changed := false
	err := withTenantSecurityGuard(ctx, r.db, sourceTenantID, func(tx *gorm.DB) error {
		row, err := r.lockedClaim(tx, sourceTenantID, id, generation, owner)
		if err != nil {
			return fmt.Errorf("lock active claim: %w", err)
		}
		if err = r.fencedSession(tx, row); err != nil {
			return fmt.Errorf("lock claim session owner: %w", err)
		}
		var msg types.Message
		if err = tx.Where("id = ? AND session_id = ? AND request_id = ? AND role = 'assistant'", row.AssistantMessageID, row.SessionID, row.RequestID).Take(&msg).Error; err != nil {
			return fmt.Errorf("load assistant placeholder: %w", err)
		}
		if err = updateClaimMessage(tx, &msg, assistantMessage); err != nil {
			return err
		}
		res := tx.Model(&AgentChatTurnClaim{}).Where("id = ? AND source_tenant_id = ? AND generation = ? AND lease_owner = ? AND state = 'active' AND lease_expires_at > ?", row.ID, sourceTenantID, generation, owner, time.Now().UTC()).Updates(map[string]any{"state": terminalState, "reason": reason, "updated_at": time.Now().UTC()})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return ErrAgentChatTurnClaimFenced
		}
		changed = true
		return nil
	})
	return changed, err
}

func (r *AgentChatTurnClaimRepository) CreateUserMessage(ctx context.Context, sourceTenantID uint64, id string, generation uint64, owner string, message *types.Message) (string, error) {
	if message == nil || message.ID != "" || message.Role != "user" {
		return "", errors.New("invalid user message")
	}
	var userMessageID string
	err := withTenantSecurityGuard(ctx, r.db, sourceTenantID, func(tx *gorm.DB) error {
		row, err := r.lockedClaim(tx, sourceTenantID, id, generation, owner)
		if err != nil {
			return err
		}
		if err = r.fencedSession(tx, row); err != nil {
			return err
		}
		if message.SessionID != row.SessionID || message.RequestID != row.RequestID {
			return errors.New("user message identity does not match claim")
		}
		if row.UserMessageID != nil {
			userMessageID = *row.UserMessageID
			return nil
		}
		copy := *message
		copy.ID = uuid.NewString()
		copy.CreatedAt = time.Now().UTC()
		copy.UpdatedAt = copy.CreatedAt
		if err = tx.Create(&copy).Error; err != nil {
			return err
		}
		res := tx.Model(&AgentChatTurnClaim{}).Where("id = ? AND source_tenant_id = ? AND generation = ? AND lease_owner = ? AND state = 'active' AND lease_expires_at > ? AND user_message_id IS NULL", row.ID, sourceTenantID, generation, owner, time.Now().UTC()).Update("user_message_id", copy.ID)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return ErrAgentChatTurnClaimFenced
		}
		userMessageID = copy.ID
		return nil
	})
	return userMessageID, err
}

func (r *AgentChatTurnClaimRepository) UpdateMessage(ctx context.Context, sourceTenantID uint64, id string, generation uint64, owner string, message *types.Message) error {
	if message == nil {
		return errors.New("message is required")
	}
	return withTenantSecurityGuard(ctx, r.db, sourceTenantID, func(tx *gorm.DB) error {
		row, err := r.lockedClaim(tx, sourceTenantID, id, generation, owner)
		if err != nil {
			return err
		}
		if err = r.fencedSession(tx, row); err != nil {
			return err
		}
		role := "assistant"
		if row.UserMessageID != nil && message.ID == *row.UserMessageID {
			role = "user"
		} else if message.ID != row.AssistantMessageID {
			return gorm.ErrRecordNotFound
		}
		var target types.Message
		if err = tx.Where("id = ? AND session_id = ? AND request_id = ? AND role = ?", message.ID, row.SessionID, row.RequestID, role).Take(&target).Error; err != nil {
			return err
		}
		return updateClaimMessage(tx, &target, message)
	})
}

func (r *AgentChatTurnClaimRepository) CancelByOwner(ctx context.Context, sessionTenantID uint64, sessionID, ownerID, assistantMessageID, reason string) (AgentChatTurnClaim, bool, error) {
	var hint AgentChatTurnClaim
	if err := r.db.WithContext(ctx).Where("session_tenant_id = ? AND session_id = ? AND assistant_message_id = ?", sessionTenantID, sessionID, assistantMessageID).Take(&hint).Error; err != nil {
		return hint, false, err
	}
	var result AgentChatTurnClaim
	changed := false
	err := withTenantSecurityGuards(ctx, r.db, []uint64{sessionTenantID, hint.SourceTenantID}, func(tx *gorm.DB) error {
		var row AgentChatTurnClaim
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("source_tenant_id = ? AND id = ? AND session_tenant_id = ? AND session_id = ? AND assistant_message_id = ?", hint.SourceTenantID, hint.ID, sessionTenantID, sessionID, assistantMessageID).Take(&row).Error; err != nil {
			return err
		}
		if row.OwnerID != ownerID {
			return gorm.ErrRecordNotFound
		}
		if err := r.fencedSession(tx, row); err != nil {
			return err
		}
		res := tx.Model(&AgentChatTurnClaim{}).Where("id = ? AND source_tenant_id = ? AND state = 'active'", row.ID, row.SourceTenantID).Updates(map[string]any{"state": "cancelled", "reason": reason, "generation": gorm.Expr("generation + 1"), "updated_at": time.Now().UTC()})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			result = row
			return nil
		}
		if res.RowsAffected != 1 {
			return errors.New("agent chat turn claim owner cancellation changed an unexpected row count")
		}
		if err := terminalizeClaimAssistantPlaceholderTx(tx, row, time.Now().UTC()); err != nil {
			return err
		}
		row.State, row.Reason, row.Generation = "cancelled", reason, row.Generation+1
		result, changed = row, true
		return nil
	})
	return result, changed, err
}
