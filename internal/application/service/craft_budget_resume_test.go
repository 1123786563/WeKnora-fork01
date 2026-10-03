package service

// T09 (#39) AC2 权威场景（达限→授权扩额→恢复同一 Run 的不重复计费不变量）：
//  1. 历史 callID 重放幂等：恢复后重发 call A 的 AuthorizeCall 不产生第二笔
//     预占/消耗（reservations 总数不变）；
//  2. 扩额 exactly-once：同 idempotency key 重放不再加额；
//  3. 恢复只对「新调用」放行：call C 在扩额前被拒、扩额后成功，历史 call A/B
//     的预占原样保留（未被释放、未被重复）。
// 自建库（AutoMigrate），不读 migrations/sqlite（当前 HEAD 序号冲突，见计划差异记录）。

import (
	"context"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/modules/commercial"
	repocommercial "github.com/Tencent/WeKnora/internal/modules/commercial/repository/commercial"
	commercialsvc "github.com/Tencent/WeKnora/internal/modules/commercial/service/commercial"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func openResumeCraftDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := openCraftUsageServiceDB(t)
	if pool, err := db.DB(); err == nil {
		pool.SetMaxOpenConns(1)
	}
	require.NoError(t, db.AutoMigrate(
		&repocommercial.BudgetAccountRow{}, &repocommercial.TaskBudgetRow{},
		&repocommercial.ReservationRow{}, &repocommercial.BudgetLotRow{},
		&repocommercial.BudgetLotAllocationRow{}, &repocommercial.TaskBudgetExtensionRow{},
		&commercialsvc.SettlementRecord{},
		&CraftBudgetGrantRow{}, &CraftBudgetCallRow{},
	))
	return db
}

func TestCraftBudgetResumeAfterExtensionDoesNotDoubleCharge(t *testing.T) {
	db := openResumeCraftDB(t)
	// TaskLimit 2000、CallUpper 1000：两次调用即耗尽任务预算（limit-spent-held=0）。
	svc, err := NewCraftBudgetService(db, nil, CraftBudgetPolicy{
		GrantWindow: time.Hour, MaxCalls: 5,
		CallUpper: commercial.Credits(1000), TaskLimit: commercial.Credits(2000),
	})
	require.NoError(t, err)
	ctx := context.Background()
	seedCraftFundedTenant(t, db, 7, 100000) // 既有 helper（craft_budget_test.go:72-81）
	seedCraftBudgetRun(t, db, 7, "resume-run-1", "s1")

	g, err := svc.Admit(ctx, craft.Scope{TenantID: 7, UserID: "u1", SessionID: "s1"}, "resume-run-1")
	require.NoError(t, err)
	binding := CraftCallBinding{ModelID: "m1", Funding: "platform"}

	callA, err := svc.AuthorizeBinding(ctx, g.ID, binding)
	require.NoError(t, err)
	_, err = svc.AuthorizeBinding(ctx, g.ID, binding) // callB：仅制造第二笔预占，后续断言只引用 callA
	require.NoError(t, err)
	require.Len(t, craftReservations(t, db, 7), 2)

	// 达限：第三次调用被拒（task headroom=0），此刻 run 持久暂停（Task 1）。
	_, err = svc.AuthorizeBinding(ctx, g.ID, binding)
	require.ErrorIs(t, err, craft.ErrBudgetDenied)

	// 授权扩额（+2000）。
	require.NoError(t, svc.Extend(ctx, g.ID, "k-resume-1", 5, commercial.Credits(2000)))
	// 达限停靠是持久状态：恢复前先复位 Run 行（同 TestCraftBudgetExtendRaisesCallCapAndCommercialLimit）。
	require.NoError(t, db.Table("agent_runs").Where("tenant_id = ? AND run_id = ?", 7, "resume-run-1").
		Updates(map[string]any{"status": "running", "wait_reason": ""}).Error)

	// 恢复后：新调用成功；历史 callID 重放幂等（不产生第二笔预占）。
	callC, err := svc.AuthorizeBinding(ctx, g.ID, binding)
	require.NoError(t, err)
	require.NotEmpty(t, callC)
	require.NoError(t, svc.AuthorizeCall(ctx, g.ID, callA))
	require.NoError(t, svc.AuthorizeCall(ctx, g.ID, callA)) // 再重放仍幂等
	require.Len(t, craftReservations(t, db, 7), 3, "resume replays never add reservations")

	// 任务行不变量：limit 落在根行（Task=Session），恰好 +2000 一次；held=3×1000；spent 仍 0（未结算）。
	var task repocommercial.TaskBudgetRow
	require.NoError(t, db.Where("tenant_id = ? AND run_id = ?", 7, "s1").First(&task).Error)
	require.EqualValues(t, 4000, task.LimitMicro)
	require.EqualValues(t, 3000, task.HeldMicro)
	require.EqualValues(t, 0, task.SpentMicro)
	var child repocommercial.TaskBudgetRow
	require.NoError(t, db.Where("tenant_id = ? AND run_id = ?", 7, "resume-run-1").First(&child).Error)
	require.Equal(t, "s1", child.RootRunID)
	require.Zero(t, child.LimitMicro)

	// 同 key 扩额重放：整体幂等，双 fence 均不再动。
	require.NoError(t, svc.Extend(ctx, g.ID, "k-resume-1", 5, commercial.Credits(2000)))
	require.NoError(t, db.Where("tenant_id = ? AND run_id = ?", 7, "s1").First(&task).Error)
	require.EqualValues(t, 4000, task.LimitMicro, "same-key replay never raises twice")
	var grants int64
	require.NoError(t, db.Model(&CraftBudgetGrantRow{}).Where("grant_id = ?", g.ID).Count(&grants).Error)
	require.EqualValues(t, 1, grants)
	var calls int64
	require.NoError(t, db.Model(&CraftBudgetCallRow{}).Where("grant_id = ?", g.ID).Count(&calls).Error)
	require.EqualValues(t, 4, calls, "A/B/被拒尝试/C 各记一行；重放不加行，无重复计费由 reservations==3 保证")
}
