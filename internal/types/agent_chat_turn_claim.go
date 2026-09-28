package types

import "time"

// AgentChatTurnClaimEntity is the durable fencing record for one AgentQA turn.
// Its unique request scope preserves replay identity across handler restarts.
type AgentChatTurnClaimEntity struct {
	ID                  string    `gorm:"type:varchar(36);primaryKey"`
	SourceTenantID      uint64    `gorm:"not null;index"`
	SessionTenantID     uint64    `gorm:"not null;uniqueIndex:uq_agent_chat_turn_claim_request,priority:1;index:idx_agent_chat_turn_claim_active,priority:1"`
	SessionID           string    `gorm:"type:varchar(36);not null;uniqueIndex:uq_agent_chat_turn_claim_request,priority:2;index:idx_agent_chat_turn_claim_active,priority:2"`
	OwnerID             string    `gorm:"type:varchar(512);not null;uniqueIndex:uq_agent_chat_turn_claim_request,priority:3"`
	RequestID           string    `gorm:"type:varchar(128);not null;uniqueIndex:uq_agent_chat_turn_claim_request,priority:4"`
	RequestHash         string    `gorm:"type:varchar(64);not null"`
	AssistantMessageID  string    `gorm:"type:varchar(36);not null;uniqueIndex"`
	UserMessageID       *string   `gorm:"type:varchar(36)"`
	AgentID             string    `gorm:"type:varchar(36);not null"`
	LocalAgentVersionID *string   `gorm:"type:varchar(36)"`
	ReleaseID           *string   `gorm:"type:varchar(36)"`
	State               string    `gorm:"type:varchar(16);not null"`
	Reason              string    `gorm:"type:text;not null;default:''"`
	LeaseOwner          string    `gorm:"type:varchar(128);not null"`
	LeaseExpiresAt      time.Time `gorm:"not null;index:idx_agent_chat_turn_claim_active,priority:4"`
	Generation          uint64    `gorm:"not null"`
	CreatedAt           time.Time `gorm:"not null"`
	UpdatedAt           time.Time `gorm:"not null"`
}

func (AgentChatTurnClaimEntity) TableName() string { return "agent_chat_turn_claims" }
