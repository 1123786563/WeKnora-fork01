package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

// kbServiceStub embeds the service interface so only the methods the retrieval
// fan-out actually calls need real bodies.
type kbServiceStub struct {
	interfaces.KnowledgeBaseService
	kbs    []*types.KnowledgeBase
	search func(ctx context.Context, id string, params types.SearchParams) ([]*types.SearchResult, error)
}

func (s *kbServiceStub) GetKnowledgeBasesByIDsOnly(_ context.Context, _ []string) ([]*types.KnowledgeBase, error) {
	return s.kbs, nil
}

func (s *kbServiceStub) ResolveEmbeddingModelKeys(context.Context, []*types.KnowledgeBase) map[string]string {
	return nil
}

func (s *kbServiceStub) HybridSearch(ctx context.Context, id string, params types.SearchParams) ([]*types.SearchResult, error) {
	return s.search(ctx, id, params)
}

func keywordKB(id string) *types.KnowledgeBase {
	return &types.KnowledgeBase{
		ID: id,
		IndexingStrategy: types.IndexingStrategy{KeywordEnabled: true},
	}
}

// TestSearchKnowledgeWorkerPanicFailsTheSearchNotTheProcess pins #3837: the
// retrieval fan-out spawns goroutines inside the tool, beyond the reach of the
// registry's executeRecovered (recover is per goroutine), so a panicking
// HybridSearch stack used to kill the whole process. Executing the tool
// through the registry must instead report a failed search and return.
func TestSearchKnowledgeWorkerPanicFailsTheSearchNotTheProcess(t *testing.T) {
	svc := &kbServiceStub{kbs: []*types.KnowledgeBase{keywordKB("kb-1")}}
	svc.search = func(context.Context, string, types.SearchParams) ([]*types.SearchResult, error) {
		var empty []int
		_ = empty[len(empty)] // runtime panic a broken retrieval stack would raise
		return nil, nil
	}
	tool := NewSearchKnowledgeTool(svc, nil, nil, types.SearchTargets{
		{Type: types.SearchTargetTypeKnowledgeBase, KnowledgeBaseID: "kb-1"},
		{Type: types.SearchTargetTypeKnowledge, KnowledgeBaseID: "kb-1", KnowledgeIDs: []string{"k1"}},
	}, nil, nil)
	registry := NewToolRegistry()
	registry.RegisterTool(tool)
	result, err := registry.ExecuteTool(
		context.Background(), tool.Name(), json.RawMessage(`{"query":"order status","mode":"keyword"}`))
	require.NoError(t, err, "the tool must return instead of crashing the test binary")
	require.NotNil(t, result)
	require.False(t, result.Success, "both retrieval workers panicked, so this is a failed search, not an empty one")
	require.Contains(t, result.Output, "Knowledge search failed")
	require.Contains(t, result.Output, "internal error")
}

// TestSearchKnowledgeWorkerNormalSearchUnchanged keeps the non-panicking path
// honest: a healthy HybridSearch still collects its results through the same
// worker goroutines.
func TestSearchKnowledgeWorkerNormalSearchUnchanged(t *testing.T) {
	svc := &kbServiceStub{kbs: []*types.KnowledgeBase{keywordKB("kb-1")}}
	svc.search = func(context.Context, string, types.SearchParams) ([]*types.SearchResult, error) {
		return []*types.SearchResult{{
			KnowledgeID:     "k1",
			KnowledgeBaseID: "kb-1",
			KnowledgeTitle:  "Order handling",
			Content:         "Orders ship within two days.",
			Score:           0.9,
		}}, nil
	}
	tool := NewSearchKnowledgeTool(svc, nil, nil, types.SearchTargets{
		{Type: types.SearchTargetTypeKnowledgeBase, KnowledgeBaseID: "kb-1"},
	}, nil, nil)
	registry := NewToolRegistry()
	registry.RegisterTool(tool)
	result, err := registry.ExecuteTool(
		context.Background(), tool.Name(), json.RawMessage(`{"query":"order status","mode":"keyword"}`))
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.Success, result.Error)
	require.Contains(t, result.Output, "Orders ship within two days.")
}
