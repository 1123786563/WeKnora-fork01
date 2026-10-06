package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubGraphRepo struct {
	interfaces.RetrieveGraphRepository
	graph *types.GraphData
	terms []string
}

func (s *stubGraphRepo) SearchNode(_ context.Context, _ types.NameSpace, nodes []string) (*types.GraphData, error) {
	s.terms = nodes
	return s.graph, nil
}

type stubGraphChunkRepo struct {
	interfaces.ChunkRepository
	chunks map[string]*types.Chunk
}

func (s *stubGraphChunkRepo) ListChunksByIDOnly(_ context.Context, ids []string) ([]*types.Chunk, error) {
	var out []*types.Chunk
	for _, id := range ids {
		if c := s.chunks[id]; c != nil {
			out = append(out, c)
		}
	}
	return out, nil
}

// graphKnowledgeService answers only the document-existence lookup the graph
// evidence checks use (GetKnowledgeBatchWithSharedAccess): a document that
// was soft-deleted is simply absent.
type graphKnowledgeService struct {
	interfaces.KnowledgeService
	documents []*types.Knowledge
}

func (s *graphKnowledgeService) GetKnowledgeBatchWithSharedAccess(
	_ context.Context, _ uint64, ids []string,
) ([]*types.Knowledge, error) {
	byID := make(map[string]*types.Knowledge, len(s.documents))
	for _, k := range s.documents {
		if k != nil {
			byID[k.ID] = k
		}
	}
	out := make([]*types.Knowledge, 0, len(ids))
	for _, id := range ids {
		if k := byID[id]; k != nil {
			out = append(out, k)
		}
	}
	return out, nil
}

// The tool used to run plain text search only. It now looks the entities up
// in the graph, returns their relations, and puts the chunks they came from
// first — only chunks of the queried, authorized knowledge base.
func TestQueryKnowledgeGraph_QueriesTheGraph(t *testing.T) {
	graphRepo := &stubGraphRepo{graph: &types.GraphData{
		Node: []*types.GraphNode{
			{Name: "Docker", Chunks: []string{"c-docker", "c-foreign", "c-disabled"}},
			{Name: "Kubernetes", Chunks: []string{"c-k8s"}},
		},
		Relation: []*types.GraphRelation{{Node1: "Kubernetes", Node2: "Docker", Type: "orchestrates"}},
	}}
	chunkRepo := &stubGraphChunkRepo{chunks: map[string]*types.Chunk{
		"c-docker": {
			ID: "c-docker", KnowledgeBaseID: "kb-1", KnowledgeID: "doc",
			Content: "Docker runs containers", IsEnabled: true,
		},
		"c-k8s": {
			ID: "c-k8s", KnowledgeBaseID: "kb-1", KnowledgeID: "doc",
			Content: "Kubernetes schedules pods", IsEnabled: true,
		},
		"c-foreign":  {ID: "c-foreign", KnowledgeBaseID: "kb-other", Content: "foreign", IsEnabled: true},
		"c-disabled": {ID: "c-disabled", KnowledgeBaseID: "kb-1", Content: "disabled", IsEnabled: false},
	}}
	tool := NewQueryKnowledgeGraphTool(&stubKnowledgeBaseService{
		kb: &types.KnowledgeBase{ID: "kb-1", ExtractConfig: &types.ExtractConfig{
			Enabled: true, Nodes: []*types.GraphNode{{Name: "技术"}},
		}},
		results: []*types.SearchResult{{ID: "c-text", KnowledgeID: "doc", Content: "text hit", Score: 0.9}},
	}).WithGraph(graphRepo, chunkRepo)

	args, err := json.Marshal(QueryKnowledgeGraphInput{KnowledgeBaseIDs: []string{"kb-1"}, Query: "Docker Kubernetes"})
	require.NoError(t, err)
	result, err := tool.Execute(context.Background(), args)
	require.NoError(t, err)

	assert.Equal(t, []string{"Docker Kubernetes", "Kubernetes", "Docker"}, graphRepo.terms,
		"terms keep case: Neo4j CONTAINS is case-sensitive")
	rows, ok := result.Data["results"].([]map[string]interface{})
	require.True(t, ok)
	ids := make([]interface{}, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row["chunk_id"])
	}
	assert.Equal(t, []interface{}{"c-docker", "c-k8s", "c-text"}, ids,
		"graph evidence first, foreign and disabled chunks dropped")
	relations, ok := result.Data["relations"].([]map[string]interface{})
	require.True(t, ok)
	require.Len(t, relations, 1)
	assert.Equal(t, "orchestrates", relations[0]["type"])
	assert.Contains(t, result.Output, "Kubernetes --[orchestrates]--> Docker")
}

// The graph namespace is the whole knowledge base. Under a document scope,
// relations between entities from out-of-scope documents must not reach the
// model, and a failed text search is reported next to the graph evidence.
func TestQueryKnowledgeGraph_ScopesRelationsToDocuments(t *testing.T) {
	graphRepo := &stubGraphRepo{graph: &types.GraphData{
		Node: []*types.GraphNode{
			{Name: "Docker", Chunks: []string{"c-a"}},
			{Name: "Kubernetes", Chunks: []string{"c-a"}},
			{Name: "Secret", Chunks: []string{"c-b"}},
		},
		Relation: []*types.GraphRelation{
			{Node1: "Kubernetes", Node2: "Docker", Type: "orchestrates"},
			{Node1: "Secret", Node2: "Docker", Type: "leaks"},
		},
	}}
	chunkRepo := &stubGraphChunkRepo{chunks: map[string]*types.Chunk{
		"c-a": {ID: "c-a", KnowledgeBaseID: "kb-1", KnowledgeID: "doc-a", Content: "in scope", IsEnabled: true},
		"c-b": {ID: "c-b", KnowledgeBaseID: "kb-1", KnowledgeID: "doc-b", Content: "out of scope", IsEnabled: true},
	}}
	tool := NewQueryKnowledgeGraphTool(&stubKnowledgeBaseService{
		kb: &types.KnowledgeBase{ID: "kb-1", ExtractConfig: &types.ExtractConfig{
			Enabled: true, Nodes: []*types.GraphNode{{Name: "技术"}},
		}},
		err: assert.AnError,
	}, types.SearchTargets{{
		Type: types.SearchTargetTypeKnowledge, KnowledgeBaseID: "kb-1", KnowledgeIDs: []string{"doc-a"},
	}}).WithGraph(graphRepo, chunkRepo)

	args, err := json.Marshal(QueryKnowledgeGraphInput{KnowledgeBaseIDs: []string{"kb-1"}, Query: "Docker"})
	require.NoError(t, err)
	result, err := tool.Execute(context.Background(), args)
	require.NoError(t, err)

	relations, ok := result.Data["relations"].([]map[string]interface{})
	require.True(t, ok)
	require.Len(t, relations, 1)
	assert.Equal(t, "orchestrates", relations[0]["type"])
	assert.NotContains(t, result.Output, "Secret")
	errs, _ := result.Data["errors"].([]string)
	require.Len(t, errs, 1, "the failed text search is reported beside the graph hits")
	assert.Contains(t, errs[0], "text search failed")
}

// The relation cap used to drop the surplus silently: the model read "30
// relations" as the complete graph. It must now learn how many were left out.
func TestQueryKnowledgeGraph_ReportsRelationTruncation(t *testing.T) {
	relations := make([]*types.GraphRelation, 0, 42)
	for i := 0; i < 42; i++ {
		relations = append(relations, &types.GraphRelation{
			Node1: "Docker", Node2: "Kubernetes", Type: fmt.Sprintf("rel-%02d", i),
		})
	}
	graphRepo := &stubGraphRepo{graph: &types.GraphData{
		Node: []*types.GraphNode{
			{Name: "Docker", Chunks: []string{"c-1"}},
			{Name: "Kubernetes", Chunks: []string{"c-1"}},
		},
		Relation: relations,
	}}
	chunkRepo := &stubGraphChunkRepo{chunks: map[string]*types.Chunk{
		"c-1": {ID: "c-1", KnowledgeBaseID: "kb-1", KnowledgeID: "doc", Content: "evidence", IsEnabled: true},
	}}
	tool := NewQueryKnowledgeGraphTool(&stubKnowledgeBaseService{
		kb: &types.KnowledgeBase{ID: "kb-1", ExtractConfig: &types.ExtractConfig{
			Enabled: true, Nodes: []*types.GraphNode{{Name: "技术"}},
		}},
	}).WithGraph(graphRepo, chunkRepo)

	args, err := json.Marshal(QueryKnowledgeGraphInput{KnowledgeBaseIDs: []string{"kb-1"}, Query: "Docker Kubernetes"})
	require.NoError(t, err)
	result, err := tool.Execute(context.Background(), args)
	require.NoError(t, err)

	rows, ok := result.Data["relations"].([]map[string]interface{})
	require.True(t, ok)
	assert.Len(t, rows, graphQueryMaxRelations, "the display cap is unchanged")
	assert.Equal(t, 42, result.Data["relations_total"], "every verified relation is counted")
	assert.Equal(t, 12, result.Data["relations_omitted"])
	assert.Contains(t, result.Output, fmt.Sprintf("✓ Found %d of 42 relations", graphQueryMaxRelations))
	assert.Contains(t, result.Output, fmt.Sprintf("limit %d", graphQueryMaxRelations))
	assert.Contains(t, result.Output, "Narrow the query to a single entity")
	assert.Contains(t, result.Output, "rel-00", "kept relations are still listed")
	assert.NotContains(t, result.Output, "rel-41", "relations past the cap are not listed")
}

// A whole-KB scope used to skip relation evidence validation entirely, so
// relations whose supporting chunks were disabled or whose document was
// deleted still reached the model. Text search results survive.
func TestQueryKnowledgeGraph_WholeKBScopeDropsRelationsWithDeadEvidence(t *testing.T) {
	graphRepo := &stubGraphRepo{graph: &types.GraphData{
		Node: []*types.GraphNode{
			{Name: "Docker", Chunks: []string{"c-disabled"}},
			{Name: "Kubernetes", Chunks: []string{"c-removed-doc"}},
		},
		Relation: []*types.GraphRelation{
			{Node1: "Docker", Node2: "Kubernetes", Type: "orchestrates"},
			{Node1: "Kubernetes", Node2: "Docker", Type: "schedules"},
		},
	}}
	chunkRepo := &stubGraphChunkRepo{chunks: map[string]*types.Chunk{
		"c-disabled":    {ID: "c-disabled", KnowledgeBaseID: "kb-1", KnowledgeID: "doc", Content: "off", IsEnabled: false},
		"c-removed-doc": {ID: "c-removed-doc", KnowledgeBaseID: "kb-1", KnowledgeID: "doc-gone", Content: "orphan", IsEnabled: true},
	}}
	tool := NewQueryKnowledgeGraphTool(&stubKnowledgeBaseService{
		kb: &types.KnowledgeBase{ID: "kb-1", ExtractConfig: &types.ExtractConfig{
			Enabled: true, Nodes: []*types.GraphNode{{Name: "技术"}},
		}},
		results: []*types.SearchResult{{ID: "c-text", KnowledgeID: "doc", KnowledgeBaseID: "kb-1", Content: "text hit", Score: 0.9}},
	}, types.SearchTargets{{
		Type: types.SearchTargetTypeKnowledgeBase, KnowledgeBaseID: "kb-1",
	}}).WithGraph(graphRepo, chunkRepo).
		WithKnowledgeScope(&graphKnowledgeService{documents: []*types.Knowledge{
			{ID: "doc", KnowledgeBaseID: "kb-1"}, // doc-gone stays absent: deleted
		}})

	args, err := json.Marshal(QueryKnowledgeGraphInput{KnowledgeBaseIDs: []string{"kb-1"}, Query: "Docker Kubernetes"})
	require.NoError(t, err)
	result, err := tool.Execute(context.Background(), args)
	require.NoError(t, err)

	relations, ok := result.Data["relations"].([]map[string]interface{})
	require.True(t, ok)
	assert.Empty(t, relations, "unverified relations must not reach the model")
	assert.NotContains(t, result.Output, "orchestrates")
	assert.NotContains(t, result.Output, "schedules")
	rows, ok := result.Data["results"].([]map[string]interface{})
	require.True(t, ok)
	require.Len(t, rows, 1, "text search results are kept")
	assert.Equal(t, "c-text", rows[0]["chunk_id"])
}

// The evidence display budget (10 chunks) must not kill a relation whose only
// support is the 11th candidate chunk: validation has no budget and checks
// every candidate in one batch.
func TestQueryKnowledgeGraph_RelationEvidenceBeyondDisplayBudget(t *testing.T) {
	dockerChunks := make([]string, 0, 11)
	chunks := make(map[string]*types.Chunk, 11)
	for i := 1; i <= 11; i++ {
		id := fmt.Sprintf("c-%02d", i)
		dockerChunks = append(dockerChunks, id)
		chunks[id] = &types.Chunk{ID: id, KnowledgeBaseID: "kb-1", KnowledgeID: "doc", Content: "evidence", IsEnabled: true}
	}
	graphRepo := &stubGraphRepo{graph: &types.GraphData{
		Node: []*types.GraphNode{
			{Name: "Docker", Chunks: dockerChunks},
			{Name: "Kubernetes", Chunks: []string{"c-01"}},
		},
		Relation: []*types.GraphRelation{{
			Node1: "Docker", Node2: "Kubernetes", Type: "orchestrates",
			Node1Chunks: []string{"c-11"}, // beyond the display budget
		}},
	}}
	tool := NewQueryKnowledgeGraphTool(&stubKnowledgeBaseService{
		kb: &types.KnowledgeBase{ID: "kb-1", ExtractConfig: &types.ExtractConfig{
			Enabled: true, Nodes: []*types.GraphNode{{Name: "技术"}},
		}},
	}).WithGraph(graphRepo, &stubGraphChunkRepo{chunks: chunks})

	args, err := json.Marshal(QueryKnowledgeGraphInput{KnowledgeBaseIDs: []string{"kb-1"}, Query: "Docker Kubernetes"})
	require.NoError(t, err)
	result, err := tool.Execute(context.Background(), args)
	require.NoError(t, err)

	relations, ok := result.Data["relations"].([]map[string]interface{})
	require.True(t, ok)
	require.Len(t, relations, 1, "the budget does not bound evidence validation")
	assert.Equal(t, "orchestrates", relations[0]["type"])
	assert.Equal(t, 11, result.Data["evidence_total"], "the display cap on chunks is reported")
	assert.Equal(t, 1, result.Data["evidence_omitted"])
	assert.NotContains(t, result.Data, "relations_total", "no relation was truncated: silence means complete")
	rows, ok := result.Data["results"].([]map[string]interface{})
	require.True(t, ok)
	require.Len(t, rows, graphQueryMaxChunks, "the display budget itself is unchanged")
	assert.Equal(t, "c-10", rows[len(rows)-1]["chunk_id"])
}

// Under a document scope, a relation's own endpoint chunks — not same-name
// nodes that happen to live in an in-scope document — decide whether its
// evidence is in scope. Endpoints without endpoint chunks keep the
// name-aggregated fallback.
func TestQueryKnowledgeGraph_DocumentScopeUsesEndpointChunks(t *testing.T) {
	graphRepo := &stubGraphRepo{graph: &types.GraphData{
		Node: []*types.GraphNode{
			// One name, two documents: name-aggregated evidence spans both.
			{Name: "Docker", Chunks: []string{"c-doc-a", "c-doc-b"}},
			{Name: "Kubernetes", Chunks: []string{"c-k8s-b"}},
		},
		Relation: []*types.GraphRelation{
			// Node1 evidence inside the scope: kept.
			{Node1: "Docker", Node2: "Kubernetes", Type: "in-scope",
				Node1Chunks: []string{"c-doc-b"}, Node2Chunks: []string{"c-k8s-b"}},
			// Node1 evidence belongs to another document: name aggregation
			// would admit it through the shared "Docker" node; the endpoint
			// chunks must not.
			{Node1: "Docker", Node2: "Kubernetes", Type: "out-of-scope",
				Node1Chunks: []string{"c-doc-a"}, Node2Chunks: []string{"c-k8s-b"}},
			// No endpoint chunks: falls back to the name-aggregated chunks,
			// which do include an in-scope one.
			{Node1: "Docker", Node2: "Kubernetes", Type: "fallback"},
		},
	}}
	chunkRepo := &stubGraphChunkRepo{chunks: map[string]*types.Chunk{
		"c-doc-a": {ID: "c-doc-a", KnowledgeBaseID: "kb-1", KnowledgeID: "doc-a", Content: "doc a", IsEnabled: true},
		"c-doc-b": {ID: "c-doc-b", KnowledgeBaseID: "kb-1", KnowledgeID: "doc-b", Content: "doc b", IsEnabled: true},
		"c-k8s-b": {ID: "c-k8s-b", KnowledgeBaseID: "kb-1", KnowledgeID: "doc-b", Content: "doc b k8s", IsEnabled: true},
	}}
	tool := NewQueryKnowledgeGraphTool(&stubKnowledgeBaseService{
		kb: &types.KnowledgeBase{ID: "kb-1", ExtractConfig: &types.ExtractConfig{
			Enabled: true, Nodes: []*types.GraphNode{{Name: "技术"}},
		}},
		results: []*types.SearchResult{{ID: "c-text-b", KnowledgeID: "doc-b", KnowledgeBaseID: "kb-1", Content: "text hit", Score: 0.9}},
	}, types.SearchTargets{{
		Type: types.SearchTargetTypeKnowledge, KnowledgeBaseID: "kb-1", KnowledgeIDs: []string{"doc-b"},
	}}).WithGraph(graphRepo, chunkRepo)

	args, err := json.Marshal(QueryKnowledgeGraphInput{KnowledgeBaseIDs: []string{"kb-1"}, Query: "Docker Kubernetes"})
	require.NoError(t, err)
	result, err := tool.Execute(context.Background(), args)
	require.NoError(t, err)

	relations, ok := result.Data["relations"].([]map[string]interface{})
	require.True(t, ok)
	require.Len(t, relations, 2)
	relTypes := make([]interface{}, 0, len(relations))
	for _, rel := range relations {
		relTypes = append(relTypes, rel["type"])
	}
	assert.ElementsMatch(t, []interface{}{"in-scope", "fallback"}, relTypes,
		"endpoint-evidence relations in scope and the name-fallback one survive")
	assert.NotContains(t, result.Output, "out-of-scope")
}
