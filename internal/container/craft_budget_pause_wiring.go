package container

// T20 (#139) budget-pause assembly wiring: the durable pause view and the
// owner/billing-admin extension surface over the real commercial budget
// tables. The service is the SAME CraftBudgetService family the controlled
// model gateway admits against (one policy family per deployment); the
// handler registers through the session package's registry so the craft
// session route table mounts it with the same guards as the usage view.

import (
	"context"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/handler/session"
	"github.com/Tencent/WeKnora/internal/logger"
	"gorm.io/gorm"
)

// newCraftBudgetPauseService assembles the budget-pause surface service with
// the deployment's default admission policy (the same defaults the
// controlled model gateway admits against).
func newCraftBudgetPauseService(db *gorm.DB) (*service.CraftBudgetService, error) {
	budget, err := service.NewCraftBudgetService(db, nil, defaultCraftBudgetPolicy())
	if err != nil {
		return nil, err
	}
	logger.Infof(context.Background(), "[CraftBudgetPause] service assembled (T20 pause/extend surface)")
	return budget, nil
}

// registerCraftBudgetPauseHTTPHandlers installs the pause API together with
// the persistent T08 Task read gate; the craft session route table mounts
// GET/POST budget routes only when this registration ran (fail-closed, no
// 503 shims — the route simply does not exist).
func registerCraftBudgetPauseHTTPHandlers(budget *service.CraftBudgetService, access *service.CraftAccessService) {
	if budget == nil {
		return
	}
	session.RegisterCraftBudgetPauseHandler(budget, access)
	logger.Infof(context.Background(), "[CraftBudgetPause] pause/extend routes registered with the craft session table")
}
