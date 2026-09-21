package workbench

import (
	"context"
	"time"

	"gorm.io/gorm"
)

/**
 * 工作台聚合读模型（MX-013，B 类 /workbench/overview）。
 * 授权后一次查询给出：计数、进行中执行、待处理交互与 as_of——客户端不做逐会话 N+1。
 * unread_notifications 与 recent_artifacts 在对应能力（MX-021/024）接入前如实为零/空。
 */

// OverviewRunRow 只映射 agent_runs 的物理列（000093/000134 建表与 ALTER 的实际列集子集）。
// execution/settlement 是运行时投影（snapshotRunRow 先例）：活动态执行观察=Status、结算=pending。
type OverviewRunRow struct {
	TenantID  uint64
	RunID     string
	SessionID string
	OwnerID   string
	Status    string
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

type OverviewCounts struct {
	ActiveRuns          int64 `json:"active_runs"`
	PendingInteractions int64 `json:"pending_interactions"`
	UnreadNotifications int64 `json:"unread_notifications"`
}

type OverviewRunSummary struct {
	RunID            string    `json:"run_id"`
	SessionID        string    `json:"session_id"`
	RunStatus        string    `json:"run_status"`
	ExecutionStatus  string    `json:"execution_status"`
	SettlementStatus string    `json:"settlement_status"`
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
	Counts              OverviewCounts              `json:"counts"`
	InProgress          []OverviewRunSummary        `json:"in_progress"`
	PendingInteractions []OverviewInteractionSummary `json:"pending_interactions"`
	RecentArtifacts     []OverviewArtifactSummary   `json:"recent_artifacts"`
	AsOf                string                      `json:"as_of"`
}

const overviewInProgressLimit = 20

var overviewActiveStatuses = []string{"queued", "running", "waiting_user", "reconciling"}

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

// Overview 以 tenant+owner 谓词聚合；上限限制防大租户拖垮（进行中 20 条）。
func (s *OverviewService) Overview(ctx context.Context, tenantID uint64, ownerID string) (Overview, error) {
	result := Overview{
		InProgress:          []OverviewRunSummary{},
		PendingInteractions: []OverviewInteractionSummary{},
		RecentArtifacts:     []OverviewArtifactSummary{},
		AsOf:                s.clock().UTC().Format(time.RFC3339),
	}
	if tenantID == 0 || ownerID == "" {
		return result, gorm.ErrRecordNotFound
	}
	var runs []OverviewRunRow
	if err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND owner_id = ? AND status IN ?", tenantID, ownerID, overviewActiveStatuses).
		Order("updated_at DESC").Limit(overviewInProgressLimit).Find(&runs).Error; err != nil {
		return result, err
	}
	for _, run := range runs {
		result.InProgress = append(result.InProgress, OverviewRunSummary{
			RunID:            run.RunID,
			SessionID:        run.SessionID,
			RunStatus:        run.Status,
			ExecutionStatus:  run.Status,          // 活动态执行观察与 run 状态同源（快照先例）
			SettlementStatus: "pending",           // 活动中结算未最终（终态时由结算域出证）
			UpdatedAt:        run.UpdatedAt,
		})
	}
	// 计数为截断查询长度（上限 20）——首页计数语义按“进行中卡片数”展示（D-025）
	result.Counts.ActiveRuns = int64(len(runs))

	var interactions []OverviewInteractionRow
	if err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND owner_id = ? AND status = ?", tenantID, ownerID, "pending").
		Order("created_at ASC").Limit(overviewInProgressLimit).Find(&interactions).Error; err != nil {
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
	return result, nil
}
