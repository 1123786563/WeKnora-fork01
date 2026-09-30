package types

import "time"

// TaskGrantRole is the per-task collaboration role a Task Owner assigns to a
// tenant member. It is strictly distinct from the tenant-level TenantRole:
// a grant never carries tenant authority, and tenant authority never implies
// a grant (CONTEXT.md: 任务协作者/任务查看者).
type TaskGrantRole string

const (
	// TaskGrantRoleViewer: read-only access to the task (任务查看者).
	TaskGrantRoleViewer TaskGrantRole = "viewer"
	// TaskGrantRoleCollaborator: may comment, add instructions and request
	// new runs (任务协作者) — never budget, personal connections or approval
	// of the owner's side effects.
	TaskGrantRoleCollaborator TaskGrantRole = "collaborator"
)

// IsValid reports whether r is one of the two defined grant roles. "owner" is
// deliberately NOT a grantable value: ownership is inherited from creating
// the task and cannot be assigned here.
func (r TaskGrantRole) IsValid() bool {
	return r == TaskGrantRoleViewer || r == TaskGrantRoleCollaborator
}

// TaskAccessRole is the resolved per-task role of one caller, including the
// two states that never live in task_grants rows (owner, none).
type TaskAccessRole string

const (
	TaskAccessNone         TaskAccessRole = "none"
	TaskAccessViewer       TaskAccessRole = "viewer"
	TaskAccessCollaborator TaskAccessRole = "collaborator"
	TaskAccessOwner        TaskAccessRole = "owner"
)

// TaskRoleCanRun reports whether the role may request a new run. Only the
// owner and collaborators can (Issue #42 AC1: Viewer 不能运行).
func TaskRoleCanRun(r TaskAccessRole) bool {
	return r == TaskAccessOwner || r == TaskAccessCollaborator
}

// TaskRoleCanManage reports whether the role may perform ownership actions:
// managing grants, archiving, raising the task budget, approving the task's
// external side effects (Issue #42 AC1).
func TaskRoleCanManage(r TaskAccessRole) bool {
	return r == TaskAccessOwner
}

// TaskGrant persists one explicit per-task role assignment. Task = Session
// (ADR-0004), so TaskID references sessions.id; the owner authority itself
// is sessions.user_id and NEVER lives in this table.
type TaskGrant struct {
	TenantID  uint64        `json:"tenant_id" gorm:"primaryKey;column:tenant_id"`
	TaskID    string        `json:"task_id" gorm:"primaryKey;column:task_id;type:varchar(36)"`
	GranteeID string        `json:"grantee_id" gorm:"primaryKey;column:grantee_id;type:varchar(512)"`
	Role      TaskGrantRole `json:"role" gorm:"column:role;type:varchar(16);not null"`
	GrantedBy string        `json:"granted_by" gorm:"column:granted_by;type:varchar(512);not null"`
	CreatedAt time.Time     `json:"created_at" gorm:"column:created_at"`
	UpdatedAt time.Time     `json:"updated_at" gorm:"column:updated_at"`
}

// TableName binds TaskGrant to the task_grants table.
func (TaskGrant) TableName() string { return "task_grants" }

// TaskAccess is the resolved access of one caller to one task.
type TaskAccess struct {
	TaskID    string
	OwnerID   string
	Role      TaskAccessRole
	GrantRole TaskGrantRole
}
