package types

import (
	"errors"
	"time"
)

// ErrTaskComplianceNotFound marks a compliance-lane task miss (unknown or
// cross-tenant). One uniform miss so probes learn nothing (T12 convention).
// Defined here so both repository and service reference it without a
// service→repository dependency edge.
var ErrTaskComplianceNotFound = errors.New("task compliance record not found")

// TenantTaskPolicy is the tenant-level Task retention policy (T13, #43).
// retention_days bounds how long a soft-deleted task must stay restorable
// before the permanent-delete (purge) lane admits it; legal_hold freezes
// every internal deletion lane (single delete, batch delete, message clear)
// until an authorized administrator lifts it. Archiving is NOT blocked:
// it is a non-destructive organizational action (CONTEXT.md 任务保留策略).
//
// A tenant with no row is ungated — the default keeps today's behavior.
type TenantTaskPolicy struct {
	TenantID      uint64    `json:"tenant_id" gorm:"primaryKey;column:tenant_id"`
	RetentionDays int       `json:"retention_days" gorm:"column:retention_days;not null"`
	LegalHold     bool      `json:"legal_hold" gorm:"column:legal_hold;not null"`
	UpdatedBy     string    `json:"updated_by" gorm:"column:updated_by;type:varchar(512);not null;default:''"`
	CreatedAt     time.Time `json:"created_at" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt     time.Time `json:"updated_at" gorm:"column:updated_at;autoUpdateTime"`
}

// TableName pins the table name (same convention as TaskGrant).
func (TenantTaskPolicy) TableName() string { return "tenant_task_policies" }

// TaskComplianceAccess is one administrator's reasoned, time-limited,
// fully-audited compliance window on one task's private content. It is
// deliberately NOT a task_grants row: compliance access never joins the
// task collaboration list (CONTEXT.md 合规访问 calls it an 独立流程).
type TaskComplianceAccess struct {
	ID        uint64    `json:"id" gorm:"primaryKey;autoIncrement;column:id"`
	TenantID  uint64    `json:"tenant_id" gorm:"column:tenant_id;not null;index:idx_task_compliance_access_task,priority:1"`
	TaskID    string    `json:"task_id" gorm:"column:task_id;type:varchar(36);not null;index:idx_task_compliance_access_task,priority:2"`
	AdminID   string    `json:"admin_id" gorm:"column:admin_id;type:varchar(512);not null;index:idx_task_compliance_access_admin,priority:2"`
	Reason    string    `json:"reason" gorm:"column:reason;type:text;not null"`
	ExpiresAt time.Time `json:"expires_at" gorm:"column:expires_at;not null"`
	CreatedAt time.Time `json:"created_at" gorm:"column:created_at;not null"`
}

// TableName pins the table name.
func (TaskComplianceAccess) TableName() string { return "task_compliance_access" }

// ExpiredAt reports whether the window is over at the given instant.
func (a TaskComplianceAccess) ExpiredAt(now time.Time) bool { return !a.ExpiresAt.After(now) }

// Covers reports whether the window still authorizes access at now.
func (a TaskComplianceAccess) Covers(now time.Time) bool { return !a.ExpiredAt(now) }
