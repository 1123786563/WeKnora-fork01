package service

// T09 (#39) 达限持久暂停：预算耗尽的执行错误必须把 run 持久停靠在
// waiting_user/budget_exhausted（可经授权扩额恢复同一 Run），而不是终态
// failed。与 durableRunFailureEvent（agent_run_graph.go:496-501）同一错误集；
// 过期类错误（ErrGrantExpired/ErrTaskBudgetExpired）不进 wait 集——否则扩额
// 重排后会形成 park/失败循环。本文件 schema 由 AutoMigrate 按镜像 struct 自建
// （列集对齐 agentRunRow，internal/application/repository/agent_run.go:40-58），
// 不读 migrations/sqlite 目录（当前 HEAD 的 000112 序号冲突是已升级的独立
// 决策，见 ocr-fix-increment-b3.md）；全部数据访问走 ORM 参数绑定。

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	repocommercial "github.com/Tencent/WeKnora/internal/modules/commercial/repository/commercial"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// budgetParkAgentRun 是 agentRunRow 的建表镜像：列名/可空性/默认值与
// agent_runs 生产 schema 一致，使真实 AgentRunStore 的
// Scan/Claim/Get/Renew/SetStatus 全链路可跑而无需迁移目录。
type budgetParkAgentRun struct {
	TenantID           uint64     `gorm:"column:tenant_id;primaryKey"`
	RunID              string     `gorm:"column:run_id;primaryKey"`
	SessionID          string     `gorm:"column:session_id;not null;default:''"`
	OwnerID            string     `gorm:"column:owner_id;not null;default:''"`
	RequestID          string     `gorm:"column:request_id;not null;default:''"`
	AssistantMessageID string     `gorm:"column:assistant_message_id;not null;default:''"`
	RequestHash        string     `gorm:"column:request_hash;not null;default:''"`
	EngineType         string     `gorm:"column:engine_type;not null;default:''"`
	Driver             string     `gorm:"column:driver;not null;default:'platform'"`
	TargetID           string     `gorm:"column:target_id;not null;default:''"`
	BudgetRef          string     `gorm:"column:budget_ref;not null;default:''"`
	Status             string     `gorm:"column:status;not null;default:'queued'"`
	WaitReason         string     `gorm:"column:wait_reason;not null;default:''"`
	Snapshot           string     `gorm:"column:snapshot;not null;default:'{}'"`
	GraphVersion       string     `gorm:"column:graph_version;not null;default:''"`
	SDKVersion         string     `gorm:"column:sdk_version;not null;default:''"`
	SchemaVersion      int        `gorm:"column:schema_version;not null;default:0"`
	LeaseOwner         string     `gorm:"column:lease_owner;not null;default:''"`
	LeaseUntil         *time.Time `gorm:"column:lease_until"`
	Epoch              int64      `gorm:"column:epoch;not null;default:0"`
	Revision           int64      `gorm:"column:revision;not null;default:0"`
	MaxRounds          int        `gorm:"column:max_rounds;not null;default:0"`
	MaxToolCalls       int        `gorm:"column:max_tool_calls;not null;default:0"`
	TokenBudget        int64      `gorm:"column:token_budget;not null;default:0"`
	Deadline           *time.Time `gorm:"column:deadline"`
	CreatedAt          time.Time  `gorm:"column:created_at;not null;default:current_timestamp"`
	UpdatedAt          time.Time  `gorm:"column:updated_at;not null;default:current_timestamp"`
}

func (budgetParkAgentRun) TableName() string { return "agent_runs" }

type budgetParkSession struct {
	ID               string  `gorm:"column:id;primaryKey"`
	TenantID         uint64  `gorm:"column:tenant_id;not null"`
	UserID           *string `gorm:"column:user_id"`
	ActiveAgentRunID *string `gorm:"column:active_agent_run_id"`
}

func (budgetParkSession) TableName() string { return "sessions" }

// openBudgetParkDB 打开每测试独立的 SQLite 文件库并建 agent_runs/sessions。
func openBudgetParkDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "budget-park.db")),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&budgetParkAgentRun{}, &budgetParkSession{}))
	userID := "u1"
	require.NoError(t, db.Create(&budgetParkSession{ID: "s1", TenantID: 7, UserID: &userID}).Error)
	t.Cleanup(func() { conn, _ := db.DB(); _ = conn.Close() })
	return db
}

func seedClaimableBudgetRun(t *testing.T, db *gorm.DB, runID string) {
	t.Helper()
	deadline := time.Now().UTC().Add(time.Hour)
	require.NoError(t, db.Create(&budgetParkAgentRun{
		TenantID: 7, RunID: runID, SessionID: "s1", OwnerID: "u1",
		Driver: "platform", Status: "queued", Deadline: &deadline,
	}).Error)
}

func runStatusReason(t *testing.T, db *gorm.DB, runID string) (string, string) {
	t.Helper()
	var row struct {
		Status     string
		WaitReason string
	}
	require.NoError(t, db.Model(&budgetParkAgentRun{}).
		Select("status, wait_reason").
		Where("tenant_id = ? AND run_id = ?", 7, runID).
		Take(&row).Error)
	return row.Status, row.WaitReason
}

// TestDurableWaitReasonClassifiesBudgetExhaustionAsDurablePark 是纯分类器表测：
// 三个预算哨兵 park；过期/吊销/普通错误不 park（防达限-过期循环，Review Focus #4）。
func TestDurableWaitReasonClassifiesBudgetExhaustionAsDurablePark(t *testing.T) {
	for _, err := range []error{
		repocommercial.ErrTaskBudgetExhausted,
		craft.ErrBudgetDenied,
		craft.ErrGrantExhausted,
	} {
		reason, wait := durableWaitReason(err)
		require.True(t, wait, "budget sentinel %v must park durably", err)
		require.Equal(t, "budget_exhausted", reason)
	}
	for _, err := range []error{
		craft.ErrGrantExpired,                // 预算/grant 过期：终态，扩额不延长 deadline
		craft.ErrGrantRevoked,                // 吊销：终态
		errors.New("ordinary model failure"), // 普通失败：终态 failed
	} {
		_, wait := durableWaitReason(err)
		require.False(t, wait, "non-budget error %v must stay terminal", err)
	}
}

// TestWorkerParksBudgetExhaustedRunDurable 走真实 worker 全链路（Scan→Claim→
// execute→SetStatus）：预算耗尽错误使 run 停靠 waiting_user/budget_exhausted；
// 对照组普通错误保持终态 failed。
func TestWorkerParksBudgetExhaustedRunDurable(t *testing.T) {
	db := openBudgetParkDB(t)
	seedClaimableBudgetRun(t, db, "park-r1")
	store := repository.NewAgentRunStore(db)
	worker, err := NewAgentRunWorker(store, func(context.Context, agentruntime.Fence) error {
		return repocommercial.ErrTaskBudgetExhausted
	}, WorkerConfig{Enabled: true, Lease: time.Minute, Heartbeat: 15 * time.Second,
		ScanInterval: 5 * time.Second, MaxWorkers: 4})
	require.NoError(t, err)
	require.NoError(t, worker.Tick(context.Background()))
	require.Eventually(t, func() bool {
		status, reason := runStatusReason(t, db, "park-r1")
		return status == "waiting_user" && reason == "budget_exhausted"
	}, 5*time.Second, 20*time.Millisecond, "budget exhaustion must durably park the run")

	// 对照组：普通错误 → 终态 failed（不可经扩额恢复）。
	seedClaimableBudgetRun(t, db, "fail-r2")
	worker2, err := NewAgentRunWorker(store, func(context.Context, agentruntime.Fence) error {
		return errors.New("provider exploded")
	}, WorkerConfig{Enabled: true, Lease: time.Minute, Heartbeat: 15 * time.Second,
		ScanInterval: 5 * time.Second, MaxWorkers: 4})
	require.NoError(t, err)
	require.NoError(t, worker2.Tick(context.Background()))
	require.Eventually(t, func() bool {
		status, _ := runStatusReason(t, db, "fail-r2")
		return status == "failed"
	}, 5*time.Second, 20*time.Millisecond, "ordinary failures stay terminal")
}
