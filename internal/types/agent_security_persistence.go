package types

import "time"

// Agent security revocation persistence entities (T34, Ticket #64).
//
// 两张表都是 append-only 台账：同一 target 的重复撤回追加新行（读取侧以
// 最新行为准），旧行保留为历史（CONTEXT.md「安全撤回」避免「静默删除
// 历史」）。匹配键是 AgentReleaseDependency 的完整四元组
// (type, id, version, digest)——与 DependencyLock 的锁定身份同口径，
// 不是名称（spec §11「运行时不得自动拉取最新依赖或按同名替换」）。

type AgentReleaseRevocationEntity struct {
	ID                   string    `gorm:"type:varchar(36);primaryKey"`
	TenantID             uint64    `gorm:"primaryKey"`
	ListingID            string    `gorm:"type:varchar(36);not null;default:''"`
	ReleaseID            string    `gorm:"type:varchar(36);not null"`
	Reason               string    `gorm:"type:text;not null"`
	ReplacementReleaseID string    `gorm:"type:varchar(36);not null;default:''"`
	InFlightDisposition  string    `gorm:"type:varchar(16);not null;default:'cancel'"`
	CanceledRunCount     int64     `gorm:"not null;default:0"`
	RunCancellationState string    `gorm:"type:varchar(16);not null;default:'complete'"`
	RevokedBy            string    `gorm:"type:varchar(255);not null;default:''"`
	RevokedAt            time.Time `gorm:"not null"`
	CreatedAt            time.Time `gorm:"not null"`
}

func (AgentReleaseRevocationEntity) TableName() string { return "agent_release_revocations" }

type AgentDependencyRevocationEntity struct {
	ID                   string    `gorm:"type:varchar(36);primaryKey"`
	TenantID             uint64    `gorm:"primaryKey"`
	DepType              string    `gorm:"type:varchar(32);not null"`
	DepID                string    `gorm:"type:varchar(255);not null"`
	DepVersion           string    `gorm:"type:varchar(64);not null"`
	DepDigest            string    `gorm:"type:varchar(64);not null"`
	Reason               string    `gorm:"type:text;not null"`
	ReplacementVersion   string    `gorm:"type:varchar(64);not null;default:''"`
	InFlightDisposition  string    `gorm:"type:varchar(16);not null;default:'cancel'"`
	CanceledRunCount     int64     `gorm:"not null;default:0"`
	RunCancellationState string    `gorm:"type:varchar(16);not null;default:'complete'"`
	RevokedBy            string    `gorm:"type:varchar(255);not null;default:''"`
	RevokedAt            time.Time `gorm:"not null"`
	CreatedAt            time.Time `gorm:"not null"`
}

func (AgentDependencyRevocationEntity) TableName() string { return "agent_dependency_revocations" }
