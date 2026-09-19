package types

import "time"

// Usage flow vocabulary for UserUsage.Flow. The bucket writers currently
// emit chat and craft; "search" is a reserved future flow value.
const (
	UsageFlowChat  = "chat"
	UsageFlowCraft = "craft"
)

// UserUsage is one daily usage bucket per (tenant, user, UTC day, model,
// flow). Rows are accumulated in place by UsageRepository.AddUsage: writing
// the same dimension tuple again adds the token/cost deltas onto the stored
// bucket instead of opening a new row.
type UserUsage struct {
	ID               uint64    `json:"id" gorm:"primaryKey;autoIncrement"`
	TenantID         uint64    `json:"tenant_id" gorm:"index"`
	UserID           string    `json:"user_id" gorm:"type:varchar(512);not null;default:''"`
	WindowStart      time.Time `json:"window_start" gorm:"not null"` // UTC day boundary
	Model            string    `json:"model" gorm:"type:varchar(128);not null;default:''"`
	Flow             string    `json:"flow" gorm:"type:varchar(32);not null;default:''"` // chat|craft|search
	InputTokens      int64     `json:"input_tokens" gorm:"not null;default:0"`
	OutputTokens     int64     `json:"output_tokens" gorm:"not null;default:0"`
	CacheReadTokens  int64     `json:"cache_read_tokens" gorm:"not null;default:0"`
	CacheWriteTokens int64     `json:"cache_write_tokens" gorm:"not null;default:0"`
	CostMicrocredits int64     `json:"cost_microcredits" gorm:"not null;default:0"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

func (UserUsage) TableName() string { return "user_usage" }
