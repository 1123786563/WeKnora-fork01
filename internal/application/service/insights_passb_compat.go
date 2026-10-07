package service

// Pass B 过渡 shim（37-insights）——ib3 后收口：dataset.go / evaluation.go /
// metric_hook.go 已随上游对齐 round 2 自 internal/modules/insights/evaluation
// 归位本包（真源现为 service/dataset.go、service/evaluation.go、
// service/metric_hook.go）。NewDatasetService 真源签名与宿主旧装配面完全一致
//（本文件历史转发已删除）；NewEvaluationService 保留 6 参旧装配签名作为
// *DI 适配器，cleanup seam（deleteReferencedKnowledge，K4 属主留守）由本
// 函数就地供给，行为零变化。

import (
	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// NewEvaluationServiceDI 是 dig 装配面（container.go）的适配构造器：
// 保持迁移前 6 参签名，cleanup seam 接到宿主现行
// deleteReferencedKnowledge（knowledge_delete_plan.go:36）——与迁移前
// 同包直引同一目标函数。
func NewEvaluationServiceDI(
	config *config.Config,
	dataset interfaces.DatasetService,
	knowledgeBaseService interfaces.KnowledgeBaseService,
	knowledgeService interfaces.KnowledgeService,
	sessionService interfaces.SessionService,
	modelService interfaces.ModelService,
) interfaces.EvaluationService {
	return NewEvaluationService(config, dataset, knowledgeBaseService,
		knowledgeService, sessionService, modelService, deleteReferencedKnowledge)
}
