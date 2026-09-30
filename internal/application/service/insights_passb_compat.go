package service

// Pass B 过渡 shim（37-insights）：dataset.go / evaluation.go / metric_hook.go
// 已物理迁移至 internal/modules/insights/evaluation。本文件为留守宿主消费方
// （container.go:388/:389 dig Provide）提供构造包装器，并把模块侧 cleanup
// seam 接到宿主现行 deleteReferencedKnowledge（knowledge_delete_plan.go:36，
// K4 推迟批留守未导出）——与迁移前同包直引同一目标函数，行为零变化。
// 删除点：ib3 集成屏障直连后（Brief 指令；K4 补迁窗导出后的接线源切换见
// Brief (b)）。
// Ruling 2026-09-25-TRANSITION-SHIM-ROW-REGISTRATION：manifest/matrix 成对
// 补行，ib3 同 commit 随文件删行。

import (
	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/modules/insights/evaluation"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// NewDatasetService forwards to the module constructor (signature
// identical to the pre-migration host declaration).
func NewDatasetService() interfaces.DatasetService {
	return evaluation.NewDatasetService()
}

// NewEvaluationService keeps the pre-migration 6-argument signature
// (container.go:389 dig compatible) and wires the cleanup seam to the host's
// current deleteReferencedKnowledge — the same target function the migrated
// code called directly before the move, so behavior is unchanged.
func NewEvaluationService(
	config *config.Config,
	dataset interfaces.DatasetService,
	knowledgeBaseService interfaces.KnowledgeBaseService,
	knowledgeService interfaces.KnowledgeService,
	sessionService interfaces.SessionService,
	modelService interfaces.ModelService,
) interfaces.EvaluationService {
	return evaluation.NewEvaluationService(config, dataset, knowledgeBaseService,
		knowledgeService, sessionService, modelService, deleteReferencedKnowledge)
}
