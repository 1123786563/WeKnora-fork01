package types

// T17 (#47): Lead Agent read-only research delegations and version-pinned
// material annotations (CONTEXT.md 主理 Agent / 委派授权 / 任务产物). A
// delegation is a durable READ-ONLY assignment: it never touches
// sessions.active_agent_run_id and it never widens task_grants. An annotation
// is an append-only review record bound to the exact artifact version identity
// it reviewed; the annotated version is never rewritten.

import (
	"encoding/json"
	"errors"
	"time"
)

const (
	// TaskResearchAssigned is the only state a delegation is created in.
	TaskResearchAssigned = "assigned"
	// TaskResearchCompleted is terminal: the recorded summary is immutable.
	TaskResearchCompleted = "completed"
)

var (
	// ErrTaskResearchNotFound reports a missing or cross-tenant delegation.
	// Both are deliberately indistinguishable (probe learns nothing).
	ErrTaskResearchNotFound = errors.New("task research delegation not found")
	// ErrTaskResearchState reports a completion attempt on a delegation that
	// is not in the assigned state (already completed).
	ErrTaskResearchState = errors.New("task research delegation state conflict")
	// ErrTaskAnnotationInvalid reports an annotation that fails validation
	// before any durable write.
	ErrTaskAnnotationInvalid = errors.New("invalid task artifact annotation")
)

// TaskResearchDelegation persists one read-only research assignment made by
// the task owner's active run. Sources are the delegated knowledge subset;
// the handler proves each source is inside the task's tenant knowledge scope
// before this row is ever written.
type TaskResearchDelegation struct {
	TenantID    uint64    `json:"tenant_id" gorm:"primaryKey;column:tenant_id"`
	ID          string    `json:"id" gorm:"primaryKey;column:id;type:varchar(36)"`
	SessionID   string    `json:"session_id" gorm:"column:session_id;type:varchar(36)"`
	ParentRunID string    `json:"parent_run_id" gorm:"column:parent_run_id;type:varchar(64)"`
	Objective   string    `json:"objective" gorm:"column:objective"`
	SourcesJSON string    `json:"-" gorm:"column:sources_json"`
	Status      string    `json:"status" gorm:"column:status;type:varchar(16);not null;default:assigned"`
	Summary     string    `json:"summary" gorm:"column:summary"`
	CreatedBy   string    `json:"created_by" gorm:"column:created_by;type:varchar(512);not null"`
	CreatedAt   time.Time `json:"created_at" gorm:"column:created_at"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"column:updated_at"`
}

// TableName binds TaskResearchDelegation to task_research_delegations.
func (TaskResearchDelegation) TableName() string { return "task_research_delegations" }

// Sources decodes the delegated knowledge subset. A malformed payload reads
// as empty — sources are server-validated at creation, so corruption here
// must degrade to "no delegated sources", never to invented grants.
func (d TaskResearchDelegation) Sources() []string {
	out := []string{}
	_ = json.Unmarshal([]byte(d.SourcesJSON), &out)
	return out
}

// TaskArtifactAnnotation is one append-only review record pinned to the
// artifact version identity (workbench artifactVersionOf) that the reviewer
// actually saw. material_id is the (message_id, index) binding; base_version
// is the version identity at annotation time.
type TaskArtifactAnnotation struct {
	TenantID    uint64    `json:"tenant_id" gorm:"primaryKey;column:tenant_id"`
	ID          string    `json:"id" gorm:"primaryKey;column:id;type:varchar(36)"`
	SessionID   string    `json:"session_id" gorm:"column:session_id;type:varchar(36)"`
	RunID       string    `json:"run_id" gorm:"column:run_id;type:varchar(64)"`
	MaterialID  string    `json:"material_id" gorm:"column:material_id"`
	BaseVersion string    `json:"base_version" gorm:"column:base_version"`
	Body        string    `json:"body" gorm:"column:body"`
	AuthorID    string    `json:"author_id" gorm:"column:author_id;type:varchar(512);not null"`
	CreatedAt   time.Time `json:"created_at" gorm:"column:created_at"`
}

// TableName binds TaskArtifactAnnotation to task_artifact_annotations.
func (TaskArtifactAnnotation) TableName() string { return "task_artifact_annotations" }
