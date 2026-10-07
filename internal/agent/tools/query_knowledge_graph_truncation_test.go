package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// graphToolOverEveryCap builds a tool whose graph answer overflows both caps:
// graphQueryMaxRelations+12 distinct relations on one hub entity, and
// graphQueryMaxChunks+4 chunks referenced by it. Upstream #3885 asserted this
// visibility with its own schema; this fork reports the same cut through
// relations_total/evidence_total/dropped_terms, so the scenarios below keep
// upstream's intent against the fork's keys.
func graphToolOverEveryCap() *QueryKnowledgeGraphTool {
	relations := make([]*types.GraphRelation, 0, graphQueryMaxRelations+12)
	for i := 0; i < graphQueryMaxRelations+12; i++ {
		relations = append(relations, &types.GraphRelation{
			Node1: "Docker", Node2: "Kubernetes", Type: fmt.Sprintf("rel-%02d", i),
		})
	}
	// A repeat of a relation already in the list must not inflate the total.
	relations = append(relations, &types.GraphRelation{Node1: "Docker", Node2: "Kubernetes", Type: "rel-00"})

	chunks := make(map[string]*types.Chunk, graphQueryMaxChunks+4)
	chunkIDs := make([]string, 0, graphQueryMaxChunks+4)
	for i := 0; i < graphQueryMaxChunks+4; i++ {
		id := fmt.Sprintf("c-%02d", i)
		chunkIDs = append(chunkIDs, id)
		chunks[id] = &types.Chunk{
			ID: id, KnowledgeBaseID: "kb-1", KnowledgeID: "doc", Content: "Docker evidence", IsEnabled: true,
		}
	}
	graphRepo := &stubGraphRepo{graph: &types.GraphData{
		Node: []*types.GraphNode{
			{Name: "Docker", Chunks: chunkIDs},
			{Name: "Kubernetes", Chunks: []string{"c-00"}},
		},
		Relation: relations,
	}}
	return NewQueryKnowledgeGraphTool(&stubKnowledgeBaseService{
		kb: &types.KnowledgeBase{ID: "kb-1", ExtractConfig: &types.ExtractConfig{
			Enabled: true, Nodes: []*types.GraphNode{{Name: "技术"}},
		}},
	}).WithGraph(graphRepo, &stubGraphChunkRepo{chunks: chunks})
}

// A hub entity has far more relations than the tool returns. The cut has to be
// stated, in the tool output and in Data: read without a marker, the returned
// subgraph is the entity's whole neighbourhood to the model, which then answers
// "X is not related to Y" from a list that never held Y.
func TestQueryKnowledgeGraph_MarksCappedRelationsAndChunks(t *testing.T) {
	tool := graphToolOverEveryCap()

	args, err := json.Marshal(QueryKnowledgeGraphInput{KnowledgeBaseIDs: []string{"kb-1"}, Query: "Docker Kubernetes"})
	require.NoError(t, err)
	result, err := tool.Execute(context.Background(), args)
	require.NoError(t, err)
	t.Logf("tool output:\n%s", result.Output)

	assert.Contains(t, result.Output, fmt.Sprintf("✓ Found %d of %d relations", graphQueryMaxRelations,
		graphQueryMaxRelations+12), "the headline count must not read as the total")
	assert.Contains(t, result.Output, fmt.Sprintf("⚠️ Showing %d of %d relations (limit %d)",
		graphQueryMaxRelations, graphQueryMaxRelations+12, graphQueryMaxRelations))
	assert.Contains(t, result.Output, fmt.Sprintf("⚠️ Showing %d of %d evidence chunks (limit %d per knowledge base)",
		graphQueryMaxChunks, graphQueryMaxChunks+4, graphQueryMaxChunks))

	relations, ok := result.Data["relations"].([]map[string]interface{})
	require.True(t, ok)
	assert.Len(t, relations, graphQueryMaxRelations)
	assert.Equal(t, graphQueryMaxRelations+12, result.Data["relations_total"])
	assert.Equal(t, 12, result.Data["relations_omitted"])
	assert.Equal(t, graphQueryMaxChunks+4, result.Data["evidence_total"])
	assert.Equal(t, 4, result.Data["evidence_omitted"])
	rows, ok := result.Data["results"].([]map[string]interface{})
	require.True(t, ok)
	assert.Len(t, rows, graphQueryMaxChunks, "the display budget itself is unchanged")
}

// A long query names more entity candidates than the tool looks up. The words
// that were left out have to be stated: an entity named by one of them is never
// matched, and "no relevant graph information" would otherwise read as "this
// entity has no relations".
func TestQueryKnowledgeGraph_MarksCappedQueryTerms(t *testing.T) {
	graphRepo := &stubGraphRepo{graph: &types.GraphData{}}
	tool := NewQueryKnowledgeGraphTool(&stubKnowledgeBaseService{
		kb: &types.KnowledgeBase{ID: "kb-1", ExtractConfig: &types.ExtractConfig{
			Enabled: true, Nodes: []*types.GraphNode{{Name: "技术"}},
		}},
	}).WithGraph(graphRepo, &stubGraphChunkRepo{chunks: map[string]*types.Chunk{}})

	query := "docker kubernetes scheduler container image registry cluster pod node volume secret ingress"
	args, err := json.Marshal(QueryKnowledgeGraphInput{KnowledgeBaseIDs: []string{"kb-1"}, Query: query})
	require.NoError(t, err)
	result, err := tool.Execute(context.Background(), args)
	require.NoError(t, err)
	t.Logf("tool output:\n%s", result.Output)

	// The 12 words plus the query itself are candidates; the cap keeps 8 and
	// the names of the rest travel with the result.
	dropped, ok := result.Data["dropped_terms"].([]string)
	require.True(t, ok, "dropped terms must be visible in Data")
	assert.Len(t, dropped, 5)
	assert.Contains(t, result.Output, "No relevant graph information found.")
}

// Silence has to mean "nothing was dropped": a result under every cap carries
// no truncation marker and no total, so a marker always means a subset.
func TestQueryKnowledgeGraph_NoTruncationMarkerWhenNothingDropped(t *testing.T) {
	graphRepo := &stubGraphRepo{graph: &types.GraphData{
		Node:     []*types.GraphNode{{Name: "Docker", Chunks: []string{"c-1"}}},
		Relation: []*types.GraphRelation{{Node1: "Kubernetes", Node2: "Docker", Type: "orchestrates"}},
	}}
	// The relation's Kubernetes endpoint needs live evidence to survive the
	// fork's all-scope evidence validation.
	graphRepo.graph.Node = append(graphRepo.graph.Node, &types.GraphNode{Name: "Kubernetes", Chunks: []string{"c-1"}})
	chunkRepo := &stubGraphChunkRepo{chunks: map[string]*types.Chunk{
		"c-1": {ID: "c-1", KnowledgeBaseID: "kb-1", KnowledgeID: "doc", Content: "Docker", IsEnabled: true},
	}}
	tool := NewQueryKnowledgeGraphTool(&stubKnowledgeBaseService{
		kb: &types.KnowledgeBase{ID: "kb-1", ExtractConfig: &types.ExtractConfig{
			Enabled: true, Nodes: []*types.GraphNode{{Name: "技术"}},
		}},
	}).WithGraph(graphRepo, chunkRepo)

	args, err := json.Marshal(QueryKnowledgeGraphInput{KnowledgeBaseIDs: []string{"kb-1"}, Query: "Docker"})
	require.NoError(t, err)
	result, err := tool.Execute(context.Background(), args)
	require.NoError(t, err)

	assert.NotContains(t, result.Output, "Showing")
	assert.NotContains(t, result.Output, " of 1 relations")
	_, hasRelationsTotal := result.Data["relations_total"]
	assert.False(t, hasRelationsTotal, "an uncapped result must not claim a relation total")
	_, hasEvidenceTotal := result.Data["evidence_total"]
	assert.False(t, hasEvidenceTotal, "an uncapped result must not claim a chunk total")
	_, hasDroppedTerms := result.Data["dropped_terms"]
	assert.False(t, hasDroppedTerms, "an uncapped result must not claim dropped terms")
}
