package service
// B3-IN.4 seam 钉住测试（37-insights §2.3(a)/§2.4）：钉住 Evaluation 运行
// 结束时对临时知识资源的清理语义（Spec §14.3 Knowledge 删除高风险面）。
// 迁移前该调用直指宿主同包 deleteReferencedKnowledge
// （internal/application/service/knowledge_delete_plan.go:36，K4 推迟批留守
// 未导出）；迁移后经 DeleteReferencedKnowledgeFunc seam 注入，本测试断言
// seam 收到与迁移前完全相同的实参（expectedKB + knowledge IDs），
// 且 KnowledgeBaseService.DeleteKnowledgeBase 同步被调——接线源不同、
// 行为等价。

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

// stubEvalDatasetService 返回受控 QA 对。
type stubEvalDatasetService struct {
	interfaces.DatasetService

	qaPairs []*types.QAPair
}

func (s *stubEvalDatasetService) GetDatasetByID(
	_ context.Context, _ string,
) ([]*types.QAPair, error) {
	return s.qaPairs, nil
}

// stubEvalKnowledgeService 固定返回评估临时知识。
type stubEvalKnowledgeService struct {
	interfaces.KnowledgeService
}

func (s *stubEvalKnowledgeService) CreateKnowledgeFromPassageSync(
	_ context.Context, _ string, _ []string, _ string,
) (*types.Knowledge, error) {
	return &types.Knowledge{ID: "k-eval-1"}, nil
}

// stubEvalKnowledgeBaseService 捕获 DeleteKnowledgeBase 实参。
type stubEvalKnowledgeBaseService struct {
	interfaces.KnowledgeBaseService

	deletedKBs []string
}

func (s *stubEvalKnowledgeBaseService) DeleteKnowledgeBase(_ context.Context, id string) error {
	s.deletedKBs = append(s.deletedKBs, id)
	return nil
}

// stubEvalSessionService 让 QA 管线静默通过。
type stubEvalSessionService struct {
	interfaces.SessionService
}

func (s *stubEvalSessionService) KnowledgeQAByEvent(
	_ context.Context, _ *types.ChatManage, _ []types.EventType,
) error {
	return nil
}

// cleanupCall 记录 seam 收到的实参。
type cleanupCall struct {
	expectedKB string
	ids        []string
}

// TestEvalDatasetInvokesKnowledgeCleanupSeam 断言 EvalDataset 的清理路径：
// seam 收到 ("kb-eval", ["k-eval-1"]) 且 DeleteKnowledgeBase("kb-eval") 被调。
func TestEvalDatasetInvokesKnowledgeCleanupSeam(t *testing.T) {
	dataset := &stubEvalDatasetService{qaPairs: []*types.QAPair{{
		Question: "q1",
		PIDs:     []int{1},
		Passages: []string{"p1"},
		Answer:   "a1",
	}}}
	knowledge := &stubEvalKnowledgeService{}
	kbService := &stubEvalKnowledgeBaseService{}
	session := &stubEvalSessionService{}

	var calls []cleanupCall
	cleanup := func(
		_ context.Context,
		_ interfaces.KnowledgeService,
		expectedKB string,
		ids []string,
	) error {
		calls = append(calls, cleanupCall{expectedKB: expectedKB, ids: ids})
		return nil
	}

	svc := NewEvaluationService(
		&config.Config{},
		dataset,
		kbService,
		knowledge,
		session,
		nil, // modelService：EvalDataset 路径不消费
		cleanup,
	)
	evalSvc, ok := svc.(*EvaluationService)
	require.True(t, ok, "NewEvaluationService 应返回具体 *EvaluationService")

	detail := &types.EvaluationDetail{
		Task:   &types.EvaluationTask{ID: "t1", DatasetID: "ds1"},
		Params: &types.ChatManage{},
	}

	err := evalSvc.EvalDataset(context.Background(), detail, "kb-eval")

	require.NoError(t, err)
	require.Len(t, calls, 1, "清理 seam 恰被调用一次")
	require.Equal(t, "kb-eval", calls[0].expectedKB)
	require.Equal(t, []string{"k-eval-1"}, calls[0].ids)
	require.Equal(t, []string{"kb-eval"}, kbService.deletedKBs,
		"评估临时知识库应同步删除")
}
