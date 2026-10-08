package workbench

import (
	"context"
	"time"

	"gorm.io/gorm"
)

/**
 * 工作台聚合读模型（MX-013，B 类 /workbench/overview）。
 * 授权后固定少量聚合查询给出：计数、进行中执行、待处理交互、最近完成与 as_of
 * ——客户端不做逐会话 N+1。unread_notifications 自本计划补建的
 * workbench_notifications（迁移 000190/000111，列集对齐 InboxNotificationRow）
 * 真实计数；recent_artifacts 在 MX-024 接入前如实为空。
 * 三段任务视图均排除已归档任务（Task lifecycle archived，T04）。
 */

// OverviewRunRow 映射 agent_runs 物理列 + sessions.title 投影（单查询 JOIN；
// taskId = sessionId，ADR-0004）。
type OverviewRunRow struct {
	TenantID  uint64
	RunID     string
	SessionID string
	OwnerID   string
	Status    string
	Title     string
	UpdatedAt time.Time
}

func (OverviewRunRow) TableName() string { return "agent_runs" }

// OverviewInteractionRow 映射既有 workbench_interactions 表（只读投影列）。
type OverviewInteractionRow struct {
	TenantID         uint64
	ID               string
	RunID            string
	OwnerID          string
	Kind             string
	ArgsHash         string
	Status           string
	ExpectedRevision int64
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (OverviewInteractionRow) TableName() string { return "workbench_interactions" }

// OverviewTaskRow projects the task identity columns the overview joins on
// (sessions). Test fixtures AutoMigrate it; production uses the real table
// after the task-archive migration (versioned 000189 / sqlite 000110).
type OverviewTaskRow struct {
	TenantID   uint64
	ID         string
	Title      string
	ArchivedAt *time.Time
}

func (OverviewTaskRow) TableName() string { return "sessions" }

// OverviewNotificationRow projects workbench_notifications (created by
// migration 000190/000111; column-aligned with InboxNotificationRow) for the
// unread count. Test fixtures AutoMigrate it.
type OverviewNotificationRow struct {
	TenantID uint64
	ID       string
	OwnerID  string
	Read     bool
}

func (OverviewNotificationRow) TableName() string { return "workbench_notifications" }

type OverviewCounts struct {
	ActiveRuns          int64 `json:"active_runs"`
	PendingInteractions int64 `json:"pending_interactions"`
	UnreadNotifications int64 `json:"unread_notifications"`
}

type OverviewRunSummary struct {
	RunID            string    `json:"run_id"`
	SessionID        string    `json:"session_id"`
	Title            string    `json:"title,omitempty"`
	RunStatus        string    `json:"run_status"`
	ExecutionStatus  string    `json:"execution_status"`
	SettlementStatus string    `json:"settlement_status"`
	Attention        string    `json:"attention"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type OverviewInteractionSummary struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	CreatedAt string `json:"created_at"`
}

type OverviewArtifactSummary struct {
	ArtifactID string `json:"artifact_id"`
	Title      string `json:"title"`
	Kind       string `json:"kind"`
}

type Overview struct {
	Counts              OverviewCounts               `json:"counts"`
	InProgress          []OverviewRunSummary         `json:"in_progress"`
	PendingInteractions []OverviewInteractionSummary `json:"pending_interactions"`
	RecentlyCompleted   []OverviewRunSummary         `json:"recently_completed"`
	RecentArtifacts     []OverviewArtifactSummary    `json:"recent_artifacts"`
	AsOf                string                       `json:"as_of"`
}

const (
	overviewInProgressLimit      = 20
	overviewRecentCompletedLimit = 10
)

var (
	overviewActiveStatuses   = []string{"queued", "running", "waiting_user", "reconciling"}
	overviewTerminalStatuses = []string{"succeeded", "failed", "canceled"}
)

func overviewAttention(status string) string {
	if status == "waiting_user" {
		return "required"
	}
	return "none"
}

type OverviewService struct {
	db    *gorm.DB
	clock func() time.Time
}

func NewWorkbenchOverviewService(db *gorm.DB, clock func() time.Time) *OverviewService {
	if clock == nil {
		clock = time.Now
	}
	return &OverviewService{db: db, clock: clock}
}

// runSegmentQuery issues one joined query for a status set: tenant+owner
// predicate, archived tasks excluded, task title projected. No per-session
// follow-up reads ever happen (the N+1 rule).
func (s *OverviewService) runSegmentQuery(ctx context.Context, tenantID uint64, ownerID string, statuses []string, limit int) ([]OverviewRunRow, error) {
	var runs []OverviewRunRow
	err := s.db.WithContext(ctx).
		Select("agent_runs.tenant_id, agent_runs.run_id, agent_runs.session_id, agent_runs.owner_id, agent_runs.status, agent_runs.updated_at, sessions.title AS title").
		Joins("JOIN sessions ON sessions.tenant_id = agent_runs.tenant_id AND sessions.id = agent_runs.session_id").
		Where("agent_runs.tenant_id = ? AND agent_runs.owner_id = ? AND agent_runs.status IN ? AND sessions.archived_at IS NULL", tenantID, ownerID, statuses).
		Order("agent_runs.updated_at DESC").Limit(limit).Find(&runs).Error
	return runs, err
}

func runSummaryOf(run OverviewRunRow, settlement string) OverviewRunSummary {
	return OverviewRunSummary{
		RunID:            run.RunID,
		SessionID:        run.SessionID,
		Title:            run.Title,
		RunStatus:        run.Status,
		ExecutionStatus:  run.Status, // 活动态执行观察与 run 状态同源（快照先例）
		SettlementStatus: settlement,
		Attention:        overviewAttention(run.Status),
		UpdatedAt:        run.UpdatedAt,
	}
}

// Overview 以 tenant+owner 谓词聚合；上限限制防大租户拖垮（进行中 20 / 最近完成 10）。
func (s *OverviewService) Overview(ctx context.Context, tenantID uint64, ownerID string) (Overview, error) {
	result := Overview{
		InProgress:          []OverviewRunSummary{},
		PendingInteractions: []OverviewInteractionSummary{},
		RecentlyCompleted:   []OverviewRunSummary{},
		RecentArtifacts:     []OverviewArtifactSummary{},
		AsOf:                s.clock().UTC().Format(time.RFC3339),
	}
	if tenantID == 0 || ownerID == "" {
		return result, gorm.ErrRecordNotFound
	}
	runs, err := s.runSegmentQuery(ctx, tenantID, ownerID, overviewActiveStatuses, overviewInProgressLimit)
	if err != nil {
		return result, err
	}
	for _, run := range runs {
		result.InProgress = append(result.InProgress, runSummaryOf(run, "pending"))
	}
	// 计数为截断查询长度（上限 20）——首页计数语义按“进行中卡片数”展示（D-025）
	result.Counts.ActiveRuns = int64(len(runs))

	completed, err := s.runSegmentQuery(ctx, tenantID, ownerID, overviewTerminalStatuses, overviewRecentCompletedLimit)
	if err != nil {
		return result, err
	}
	for _, run := range completed {
		// 终态结算投影沿用 agent_run_snapshot.go 的先例：终态即已结算出证。
		result.RecentlyCompleted = append(result.RecentlyCompleted, runSummaryOf(run, "settled"))
	}

	var interactions []OverviewInteractionRow
	if err := s.db.WithContext(ctx).
		Table("workbench_interactions").
		Select("workbench_interactions.id, workbench_interactions.kind, workbench_interactions.created_at").
		Joins("LEFT JOIN agent_runs ar ON ar.tenant_id = workbench_interactions.tenant_id AND ar.run_id = workbench_interactions.run_id").
		Joins("LEFT JOIN sessions ON sessions.tenant_id = ar.tenant_id AND sessions.id = ar.session_id").
		Where("workbench_interactions.tenant_id = ? AND workbench_interactions.owner_id = ? AND workbench_interactions.status = ? AND sessions.archived_at IS NULL", tenantID, ownerID, "pending").
		Order("workbench_interactions.created_at ASC").Limit(overviewInProgressLimit).Find(&interactions).Error; err != nil {
		return result, err
	}
	for _, row := range interactions {
		result.PendingInteractions = append(result.PendingInteractions, OverviewInteractionSummary{
			ID:        row.ID,
			Kind:      row.Kind,
			CreatedAt: row.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	result.Counts.PendingInteractions = int64(len(interactions))

	var unread int64
	if err := s.db.WithContext(ctx).Model(&OverviewNotificationRow{}).
		Where("tenant_id = ? AND owner_id = ? AND read = ?", tenantID, ownerID, false).
		Count(&unread).Error; err != nil {
		return result, err
	}
	result.Counts.UnreadNotifications = unread
	return result, nil
}
