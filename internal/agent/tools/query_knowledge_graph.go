package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/Tencent/WeKnora/internal/utils"
)

type graphConfigSummary struct {
	Nodes     []string
	Relations []string
}

var queryKnowledgeGraphTool = BaseTool{
	name: ToolQueryKnowledgeGraph,
	description: "Query the knowledge graph of graph-enabled knowledge bases to explore how entities relate " +
		"(for example \"relationship between Docker and Kubernetes\"). Put the entity names in query. Returns " +
		"the matching entities' relations and the chunks they were extracted from, then chunks found by text " +
		"search, with cN handles.\nUse search_knowledge for ordinary text retrieval and " +
		"read_document(id=cN, context=k) to read around a chunk.",
	schema: utils.GenerateSchema[QueryKnowledgeGraphInput](),
}

// QueryKnowledgeGraphInput defines the input parameters for query knowledge graph tool
type QueryKnowledgeGraphInput struct {
	KnowledgeBaseIDs []string `json:"knowledge_base_ids" jsonschema:"Array of short bN knowledge base IDs to query"`
	Query            string   `json:"query" jsonschema:"Query content (entity name or query text)"`
}

// QueryKnowledgeGraphTool queries the knowledge graph for entities and relationships
type QueryKnowledgeGraphTool struct {
	BaseTool
	knowledgeService      interfaces.KnowledgeBaseService
	scopeKnowledgeService interfaces.KnowledgeService
	searchTargets         types.SearchTargets
	scopeEnforced         bool
	graphRepo             interfaces.RetrieveGraphRepository
	chunkRepo             interfaces.ChunkRepository
}

// WithGraph lets the tool query the graph store for entities and relations.
// Without it the tool only has text search, which is what it used to do in
// every case: the tool never touched the graph, so a graph-only knowledge
// base always answered "no relevant graph information".
func (t *QueryKnowledgeGraphTool) WithGraph(
	graphRepo interfaces.RetrieveGraphRepository, chunkRepo interfaces.ChunkRepository,
) *QueryKnowledgeGraphTool {
	t.graphRepo = graphRepo
	t.chunkRepo = chunkRepo
	return t
}

const (
	// graphQueryMaxTerms bounds the entity-name terms sent to the graph store.
	graphQueryMaxTerms = 8
	// graphQueryMaxChunks bounds the evidence chunks loaded per knowledge base.
	graphQueryMaxChunks = 10
	// graphQueryMaxRelations bounds the relations returned to the model.
	graphQueryMaxRelations = 30
)

// graphSearchTerms turns the query into entity-name terms and reports the
// terms dropped by the term cap. The graph store matches node names by
// case-sensitive substring, so the whole query only matches when it is itself
// an entity name; its words catch the entities a question mentions
// ("Docker 和 Kubernetes 的关系" → Docker, Kubernetes).
// Words keep their case: lowercased terms never matched "Docker".
func graphSearchTerms(query string) (terms []string, dropped []string) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	seen := map[string]bool{query: true}
	var tokens []string
	for _, word := range queryTerms(query) {
		if !seen[word] {
			seen[word] = true
			tokens = append(tokens, word)
		}
	}
	// Longer tokens are more specific entity candidates; sort for stability.
	sort.SliceStable(tokens, func(i, j int) bool {
		li, lj := len([]rune(tokens[i])), len([]rune(tokens[j]))
		if li != lj {
			return li > lj
		}
		return tokens[i] < tokens[j]
	})
	terms = []string{query}
	for _, token := range tokens {
		if len(terms) >= graphQueryMaxTerms {
			dropped = append(dropped, token)
			continue
		}
		terms = append(terms, token)
	}
	return terms, dropped
}

// WithKnowledgeScope enables document/tag-level result filtering for Agent
// calls. The graph backend queries by KB, so the tool must enforce narrower
// SearchTargets before returning any result to the model.
func (t *QueryKnowledgeGraphTool) WithKnowledgeScope(
	knowledgeService interfaces.KnowledgeService,
) *QueryKnowledgeGraphTool {
	t.scopeKnowledgeService = knowledgeService
	return t
}

// NewQueryKnowledgeGraphTool creates a new query knowledge graph tool
func NewQueryKnowledgeGraphTool(
	knowledgeService interfaces.KnowledgeBaseService,
	searchTargets ...types.SearchTargets,
) *QueryKnowledgeGraphTool {
	tool := &QueryKnowledgeGraphTool{
		BaseTool:         queryKnowledgeGraphTool,
		knowledgeService: knowledgeService,
	}
	// Presence of the variadic argument — not its length — enables the Agent
	// authorization boundary, so an empty scope fails closed.
	if len(searchTargets) > 0 {
		tool.searchTargets = searchTargets[0]
		tool.scopeEnforced = true
	}
	return tool
}

// Execute performs the knowledge graph query with concurrent KB processing
func (t *QueryKnowledgeGraphTool) Execute(ctx context.Context, args json.RawMessage) (*types.ToolResult, error) {
	// Parse args from json.RawMessage
	var input QueryKnowledgeGraphInput
	if err := json.Unmarshal(args, &input); err != nil {
		return &types.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("Failed to parse args: %v", err),
		}, err
	}

	// Extract knowledge_base_ids array
	if len(input.KnowledgeBaseIDs) == 0 {
		return &types.ToolResult{
			Success: false,
			Error:   "knowledge_base_ids is required and must be a non-empty array",
		}, fmt.Errorf("knowledge_base_ids is required")
	}

	// Validate max 10 KBs
	if len(input.KnowledgeBaseIDs) > 10 {
		return &types.ToolResult{
			Success: false,
			Error:   "knowledge_base_ids must contain at most 10 KB IDs",
		}, fmt.Errorf("too many KB IDs")
	}
	if t.scopeEnforced {
		if err := validateKnowledgeBaseIDsInSearchTargets(t.searchTargets, input.KnowledgeBaseIDs); err != nil {
			return &types.ToolResult{Success: false, Error: err.Error()}, err
		}
	}

	query := input.Query
	if query == "" {
		return &types.ToolResult{
			Success: false,
			Error:   "query is required",
		}, fmt.Errorf("invalid query")
	}

	// Concurrently query all knowledge bases
	type graphQueryResult struct {
		kbID         string
		kb           *types.KnowledgeBase
		graph        *types.GraphData      // the matched graph, kept for post-merge relation evidence validation
		graphResults []*types.SearchResult // chunks the matched entities come from
		textResults  []*types.SearchResult
		relations    []*types.GraphRelation
		warnings     []string // failures of one source while the other returned results
		err          error
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	kbResults := make(map[string]*graphQueryResult)
	terms, droppedTerms := graphSearchTerms(query)

	searchParams := types.SearchParams{
		QueryText:             query,
		MatchCount:            10,
		SkipContextEnrichment: true,
	}

	for _, kbID := range input.KnowledgeBaseIDs {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			res := &graphQueryResult{kbID: id}
			defer func() {
				mu.Lock()
				kbResults[id] = res
				mu.Unlock()
			}()

			// Get knowledge base to check graph configuration
			kb, err := t.knowledgeService.GetKnowledgeBaseByIDOnly(ctx, id)
			if err != nil {
				res.err = fmt.Errorf("failed to get knowledge base: %v", err)
				return
			}
			res.kb = kb

			// Check if graph extraction is enabled
			if kb.ExtractConfig == nil || (len(kb.ExtractConfig.Nodes) == 0 && len(kb.ExtractConfig.Relations) == 0) {
				res.err = fmt.Errorf("graph extraction not configured")
				return
			}

			var errs []string
			graphResults, graph, err := t.queryGraph(ctx, id, terms)
			if err != nil {
				errs = append(errs, fmt.Sprintf("graph query failed: %v", err))
			}
			var relations []*types.GraphRelation
			if graph != nil {
				relations = graph.Relation
			}
			// Text search complements the graph: relations say how entities
			// connect, text hits carry statements the extraction missed. A KB
			// with no text index returns nothing here rather than an error.
			textResults, err := t.knowledgeService.HybridSearch(ctx, id, searchParams)
			if err != nil {
				errs = append(errs, fmt.Sprintf("text search failed: %v", err))
			}
			if t.scopeEnforced {
				if graphResults, err = filterSearchResultsInSearchTargets(
					ctx, t.searchTargets, id, graphResults, t.scopeKnowledgeService,
				); err != nil {
					res.err = err
					return
				}
				if textResults, err = filterSearchResultsInSearchTargets(
					ctx, t.searchTargets, id, textResults, t.scopeKnowledgeService,
				); err != nil {
					res.err = err
					return
				}
				// The graph namespace is the whole knowledge base, so under a
				// document or tag scope only relations between entities backed
				// by an in-scope chunk may reach the model.
				if !searchTargetsCoverWholeKB(t.searchTargets, id) {
					relations = relationsBackedBy(graph, graphResults)
				}
			}
			res.graph, res.graphResults, res.textResults, res.relations = graph, graphResults, textResults, relations
			if len(errs) > 0 && len(graphResults) == 0 && len(textResults) == 0 {
				res.err = errors.New(strings.Join(errs, "; "))
			} else {
				res.warnings = errs
			}
		}(kbID)
	}

	wg.Wait()

	// Collect and deduplicate results: graph evidence first (in entity
	// order), then text hits by score.
	seenChunks := make(map[string]bool)
	var errs []string
	graphConfigs := make(map[string]graphConfigSummary)
	kbCounts := make(map[string]int)
	var graphHits, textHits []*types.SearchResult
	var relations []*types.GraphRelation
	seenRelations := make(map[string]bool)
	totalRelations := 0 // every verified unique relation, including the truncated ones
	evidenceTotal := 0  // chunk candidates before the display budget, summed over KBs
	evidenceShown := 0  // chunk IDs the display budget collected, summed over KBs

	for _, kbID := range input.KnowledgeBaseIDs {
		result := kbResults[kbID]
		if result.err != nil {
			errs = append(errs, fmt.Sprintf("KB %s: %v", kbID, result.err))
			continue
		}
		for _, warning := range result.warnings {
			errs = append(errs, fmt.Sprintf("KB %s: %s", kbID, warning))
		}

		if result.kb != nil && result.kb.ExtractConfig != nil {
			graphConfigs[kbID] = summarizeGraphConfig(result.kb.ExtractConfig)
		}

		// Relation evidence validation runs for every scope — a whole-KB
		// query must not return relations whose supporting chunks have been
		// deleted — and before the dedup/truncate pass, so the relation
		// counts below refer to verified relations only. Without a chunk
		// store to verify against, an unverified relation never reaches the
		// model.
		kbRelations := result.relations
		if len(kbRelations) > 0 {
			if t.chunkRepo == nil {
				errs = append(errs, fmt.Sprintf(
					"KB %s: %d graph relations omitted: no chunk store to verify their evidence", kbID, len(kbRelations)))
				kbRelations = nil
			} else {
				verified, err := t.validateRelationEvidence(ctx, kbID, result.graph, kbRelations)
				if err != nil {
					errs = append(errs, fmt.Sprintf("KB %s: graph relations omitted: %v", kbID, err))
					kbRelations = nil
				} else {
					kbRelations = verified
				}
			}
		}
		if t.chunkRepo != nil && result.graph != nil {
			if candidates := graphChunkCandidates(result.graph); candidates > graphQueryMaxChunks {
				evidenceTotal += candidates
				evidenceShown += graphQueryMaxChunks
			}
		}

		kbCounts[kbID] = len(result.graphResults) + len(result.textResults)
		for _, r := range result.graphResults {
			if !seenChunks[r.ID] {
				seenChunks[r.ID] = true
				graphHits = append(graphHits, r)
			}
		}
		for _, r := range result.textResults {
			if !seenChunks[r.ID] {
				seenChunks[r.ID] = true
				textHits = append(textHits, r)
			}
		}
		for _, rel := range kbRelations {
			if rel == nil {
				continue
			}
			key := rel.Node1 + "\x00" + rel.Type + "\x00" + rel.Node2
			if seenRelations[key] {
				continue
			}
			// Count every unique relation even past the cap: silently
			// dropping them made the cap read as the complete graph.
			seenRelations[key] = true
			totalRelations++
			if len(relations) < graphQueryMaxRelations {
				relations = append(relations, rel)
			}
		}
	}

	sort.SliceStable(textHits, func(i, j int) bool {
		return textHits[i].Score > textHits[j].Score
	})
	allResults := append(graphHits, textHits...)
	relationData := make([]map[string]interface{}, 0, len(relations))
	for _, rel := range relations {
		relationData = append(relationData, map[string]interface{}{
			"source": rel.Node1, "type": rel.Type, "target": rel.Node2,
		})
	}

	if len(allResults) == 0 && len(relations) == 0 {
		// The model reads an empty result from Output alone, so failures
		// must be stated here or they read as "the graph has no such entity".
		output := "No relevant graph information found."
		if len(errs) > 0 {
			output += " Some knowledge bases could not be queried: " + strings.Join(errs, "; ") + "."
		}
		data := map[string]interface{}{
			"knowledge_base_ids": input.KnowledgeBaseIDs,
			"query":              query,
			"results":            []interface{}{},
			"relations":          relationData,
			"graph_configs":      graphConfigsToData(graphConfigs),
			"graph_config":       aggregateGraphConfig(graphConfigs),
			"errors":             errs,
			"display_type":       "graph_query_results",
		}
		applyGraphBudgetNotes(data, len(relations), totalRelations, evidenceShown, evidenceTotal, droppedTerms)
		return &types.ToolResult{
			Success: true,
			Output:  output,
			Data:    data,
		}, nil
	}

	// Format output with enhanced graph information
	output := "=== Knowledge Graph Query ===\n\n"
	output += fmt.Sprintf("📊 Query: %s\n", query)
	output += fmt.Sprintf("🎯 Target Knowledge Bases: %v\n", input.KnowledgeBaseIDs)
	if totalRelations > len(relations) {
		output += fmt.Sprintf("✓ Found %d of %d relations and %d relevant chunks (deduplicated)\n\n",
			len(relations), totalRelations, len(allResults))
	} else {
		output += fmt.Sprintf("✓ Found %d relations and %d relevant chunks (deduplicated)\n\n",
			len(relations), len(allResults))
	}

	if len(errs) > 0 {
		output += "=== ⚠️ Partial Failures ===\n"
		for _, errMsg := range errs {
			output += fmt.Sprintf("  - %s\n", errMsg)
		}
		output += "\n"
	}

	if len(relations) > 0 {
		output += "=== 🔗 Relations ===\n"
		for _, rel := range relations {
			output += fmt.Sprintf("  - %s --[%s]--> %s\n", rel.Node1, rel.Type, rel.Node2)
		}
		output += "\n"
	}
	if totalRelations > len(relations) {
		output += fmt.Sprintf("⚠️ Showing %d of %d relations (limit %d). Narrow the query to a single entity to see the remaining relations.\n\n",
			len(relations), totalRelations, graphQueryMaxRelations)
	}
	if evidenceTotal > evidenceShown {
		output += fmt.Sprintf("⚠️ Showing %d of %d evidence chunks (limit %d per knowledge base).\n\n",
			evidenceShown, evidenceTotal, graphQueryMaxChunks)
	}

	// Display graph configuration status
	hasGraphConfig := false
	output += "=== 📈 Graph Configuration Status ===\n\n"
	for kbID, config := range graphConfigs {
		hasGraphConfig = true
		output += fmt.Sprintf("Knowledge Base [%s]:\n", kbID)

		if len(config.Nodes) > 0 {
			output += fmt.Sprintf("  ✓ Entity Types (%d): %v\n", len(config.Nodes), config.Nodes)
		} else {
			output += "  ⚠️ No entity types configured\n"
		}

		if len(config.Relations) > 0 {
			output += fmt.Sprintf("  ✓ Relationship Types (%d): %v\n", len(config.Relations), config.Relations)
		} else {
			output += "  ⚠️ No relationship types configured\n"
		}
		output += "\n"
	}

	if !hasGraphConfig {
		output += "⚠️ None of the queried knowledge bases have graph extraction configured\n\n"
	}

	// Display result counts by KB
	if len(kbCounts) > 0 {
		output += "=== 📚 Knowledge Base Coverage ===\n"
		for kbID, count := range kbCounts {
			output += fmt.Sprintf("  - %s: %d results\n", kbID, count)
		}
		output += "\n"
	}

	// Display search results
	output += "=== 🔍 Query Results ===\n\n"

	formattedResults := make([]map[string]interface{}, 0, len(allResults))
	currentKB := ""

	for i, result := range allResults {
		// Group by knowledge base
		if result.KnowledgeID != currentKB {
			currentKB = result.KnowledgeID
			if i > 0 {
				output += "\n"
			}
			output += fmt.Sprintf("[Source Document: %s]\n\n", result.KnowledgeTitle)
		}

		relevanceLevel := GetRelevanceLevel(result.Score)

		output += fmt.Sprintf("Result #%d:\n", i+1)
		output += fmt.Sprintf("  📍 Relevance: %.2f (%s)\n", result.Score, relevanceLevel)
		output += fmt.Sprintf("  🔗 Match Type: %s\n", FormatMatchType(result.MatchType))
		output += fmt.Sprintf("  📄 Content: %s\n", result.Content)
		output += fmt.Sprintf("  🆔 chunk_id: %s\n\n", result.ID)

		formattedResults = append(formattedResults, map[string]interface{}{
			"result_index":      i + 1,
			"chunk_id":          result.ID,
			"chunk_index":       result.ChunkIndex,
			"chunk_type":        result.ChunkType,
			"content":           result.Content,
			"score":             result.Score,
			"relevance_level":   relevanceLevel,
			"knowledge_id":      result.KnowledgeID,
			"knowledge_base_id": result.KnowledgeBaseID,
			"knowledge_title":   result.KnowledgeTitle,
			"match_type":        FormatMatchType(result.MatchType),
		})
	}

	// Build structured graph data for frontend visualization
	graphData := buildGraphVisualizationData(allResults, relations)

	data := map[string]interface{}{
		"knowledge_base_ids": input.KnowledgeBaseIDs,
		"query":              query,
		"results":            formattedResults,
		"relations":          relationData,
		"count":              len(allResults),
		"kb_counts":          kbCounts,
		"graph_configs":      graphConfigsToData(graphConfigs),
		"graph_config":       aggregateGraphConfig(graphConfigs),
		"graph_data":         graphData,
		"has_graph_config":   hasGraphConfig,
		"errors":             errs,
		"display_type":       "graph_query_results",
	}
	applyGraphBudgetNotes(data, len(relations), totalRelations, evidenceShown, evidenceTotal, droppedTerms)
	return &types.ToolResult{
		Success: true,
		Output:  output,
		Data:    data,
	}, nil
}

// applyGraphBudgetNotes adds the display-budget counters to a graph query's
// Data: how many relations and evidence chunks a cap left out, and which
// search terms never reached the graph store. The keys are absent when no
// budget applied — silence means the result is complete.
func applyGraphBudgetNotes(
	data map[string]interface{}, relationsShown, relationsTotal, evidenceShown, evidenceTotal int, droppedTerms []string,
) {
	if relationsTotal > relationsShown {
		data["relations_total"] = relationsTotal
		data["relations_omitted"] = relationsTotal - relationsShown
	}
	if evidenceTotal > evidenceShown {
		data["evidence_total"] = evidenceTotal
		data["evidence_omitted"] = evidenceTotal - evidenceShown
	}
	if len(droppedTerms) > 0 {
		data["dropped_terms"] = droppedTerms
	}
}

// queryGraph looks the query's entity terms up in kbID's graph and returns
// the chunks the matched entities were extracted from, plus the matched graph
// (entities and relations). With no graph store wired, or none configured
// (the store answers nil), it returns nothing.
func (t *QueryKnowledgeGraphTool) queryGraph(
	ctx context.Context, kbID string, terms []string,
) ([]*types.SearchResult, *types.GraphData, error) {
	if t.graphRepo == nil || len(terms) == 0 {
		return nil, nil, nil
	}
	graph, err := t.graphRepo.SearchNode(ctx, types.NameSpace{KnowledgeBase: kbID}, terms)
	if err != nil || graph == nil {
		return nil, nil, err
	}
	chunkIDs := make([]string, 0, graphQueryMaxChunks)
	seen := make(map[string]bool)
	for _, node := range graph.Node {
		for _, id := range node.Chunks {
			if len(chunkIDs) >= graphQueryMaxChunks {
				break
			}
			if id != "" && !seen[id] {
				seen[id] = true
				chunkIDs = append(chunkIDs, id)
			}
		}
	}
	if len(chunkIDs) == 0 || t.chunkRepo == nil {
		return nil, graph, nil
	}
	// The graph namespace is the knowledge base, which the caller already
	// authorized; a chunk ID is only trusted when its row belongs to it.
	chunks, err := t.chunkRepo.ListChunksByIDOnly(ctx, chunkIDs)
	if err != nil {
		return nil, graph, err
	}
	byID := make(map[string]*types.Chunk, len(chunks))
	knowledgeIDs := make([]string, 0, len(chunks))
	for _, c := range chunks {
		if c != nil && c.KnowledgeBaseID == kbID && c.IsEnabled {
			byID[c.ID] = c
			knowledgeIDs = append(knowledgeIDs, c.KnowledgeID)
		}
	}
	titles, err := t.knowledgeTitles(ctx, knowledgeIDs)
	if err != nil {
		return nil, graph, err
	}
	results := make([]*types.SearchResult, 0, len(byID))
	for _, id := range chunkIDs {
		c := byID[id]
		if c == nil {
			continue
		}
		// A chunk can outlive its soft-deleted document; titles holds only
		// documents that still exist (nil when there is no way to check).
		if _, exists := titles[c.KnowledgeID]; titles != nil && !exists {
			continue
		}
		results = append(results, &types.SearchResult{
			ID:              c.ID,
			Content:         c.Content,
			KnowledgeID:     c.KnowledgeID,
			KnowledgeBaseID: c.KnowledgeBaseID,
			KnowledgeTitle:  titles[c.KnowledgeID],
			ChunkIndex:      c.ChunkIndex,
			ChunkType:       string(c.ChunkType),
			ParentChunkID:   c.ParentChunkID,
			MatchType:       types.MatchTypeGraph,
		})
	}
	return results, graph, nil
}

// knowledgeTitles returns the titles of the knowledge IDs that still exist.
// It returns nil when no knowledge service is wired to look them up.
func (t *QueryKnowledgeGraphTool) knowledgeTitles(ctx context.Context, ids []string) (map[string]string, error) {
	if t.scopeKnowledgeService == nil {
		return nil, nil
	}
	titles := make(map[string]string, len(ids))
	if len(ids) == 0 {
		return titles, nil
	}
	tenantID, _ := types.TenantIDFromContext(ctx)
	knowledges, err := t.scopeKnowledgeService.GetKnowledgeBatchWithSharedAccess(ctx, tenantID, ids)
	if err != nil {
		return nil, fmt.Errorf("failed to load documents: %w", err)
	}
	for _, k := range knowledges {
		if k != nil {
			titles[k.ID] = k.Title
		}
	}
	return titles, nil
}

// searchTargetsCoverWholeKB reports whether targets grant the whole of kbID
// rather than some of its documents or tags.
func searchTargetsCoverWholeKB(targets types.SearchTargets, kbID string) bool {
	for _, target := range targets {
		if target != nil && target.KnowledgeBaseID == kbID && searchTargetIsWholeKB(target) {
			return true
		}
	}
	return false
}

// relationsBackedBy keeps the graph's relations whose two endpoints were both
// extracted from one of the evidence chunks. A relation's own endpoint chunks
// are the real evidence — entities of the same name in other documents are
// different node instances — so the name-aggregated chunks are only the
// fallback for endpoints that carry none. Endpoints whose chunks are not
// among the evidence cannot be shown to be in scope, so their relations are
// dropped.
func relationsBackedBy(graph *types.GraphData, evidence []*types.SearchResult) []*types.GraphRelation {
	if graph == nil || len(evidence) == 0 {
		return nil
	}
	allowedChunks := make(map[string]bool, len(evidence))
	for _, r := range evidence {
		allowedChunks[r.ID] = true
	}
	nodeChunks := relationNodeChunks(graph)
	var relations []*types.GraphRelation
	for _, rel := range graph.Relation {
		if rel == nil {
			continue
		}
		if endpointEvidenceIntersects(rel.Node1Chunks, nodeChunks[rel.Node1], allowedChunks) &&
			endpointEvidenceIntersects(rel.Node2Chunks, nodeChunks[rel.Node2], allowedChunks) {
			relations = append(relations, rel)
		}
	}
	return relations
}

// relationNodeChunks aggregates each node name's chunk IDs: the fallback
// endpoint evidence for relations whose endpoints carry none.
func relationNodeChunks(graph *types.GraphData) map[string][]string {
	nodeChunks := make(map[string][]string)
	if graph == nil {
		return nodeChunks
	}
	for _, node := range graph.Node {
		if node == nil {
			continue
		}
		nodeChunks[node.Name] = append(nodeChunks[node.Name], node.Chunks...)
	}
	return nodeChunks
}

// endpointEvidenceIntersects reports whether a relation endpoint has evidence
// among the allowed chunks. The relation's own endpoint chunks name the exact
// node instances it connects, so they take precedence; the name-aggregated
// chunks stand in only when the endpoint carries none.
func endpointEvidenceIntersects(endpointChunks, nameChunks []string, allowed map[string]bool) bool {
	chunks := endpointChunks
	if len(chunks) == 0 {
		chunks = nameChunks
	}
	for _, id := range chunks {
		if allowed[id] {
			return true
		}
	}
	return false
}

// graphChunkCandidates counts the distinct chunk IDs the graph's nodes
// reference — the evidence pool before the per-KB display budget applies.
func graphChunkCandidates(graph *types.GraphData) int {
	if graph == nil {
		return 0
	}
	seen := make(map[string]bool)
	for _, node := range graph.Node {
		for _, id := range node.Chunks {
			if id != "" {
				seen[id] = true
			}
		}
	}
	return len(seen)
}

// validateRelationEvidence keeps only relations whose two endpoints are each
// backed by a chunk that is alive: its row exists, belongs to this knowledge
// base, is enabled, and its document still exists. It runs for every scope —
// including whole-KB queries, which used to skip the check entirely — and its
// budget is independent of the display budget: every candidate chunk ID,
// however many, is verified in one batch.
func (t *QueryKnowledgeGraphTool) validateRelationEvidence(
	ctx context.Context, kbID string, graph *types.GraphData, relations []*types.GraphRelation,
) ([]*types.GraphRelation, error) {
	nodeChunks := relationNodeChunks(graph)
	candidates := make(map[string]bool)
	addAll := func(ids []string) {
		for _, id := range ids {
			if id != "" {
				candidates[id] = true
			}
		}
	}
	for _, ids := range nodeChunks {
		addAll(ids)
	}
	for _, rel := range relations {
		addAll(rel.Node1Chunks)
		addAll(rel.Node2Chunks)
	}
	if len(candidates) == 0 {
		// No chunk backs any endpoint, so no relation can be verified.
		return nil, nil
	}
	ids := make([]string, 0, len(candidates))
	for id := range candidates {
		ids = append(ids, id)
	}
	chunks, err := t.chunkRepo.ListChunksByIDOnly(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("failed to verify relation evidence: %w", err)
	}
	aliveDocs := make(map[string]string, len(chunks)) // chunk ID → its document
	knowledgeIDs := make([]string, 0, len(chunks))
	for _, c := range chunks {
		if c == nil || c.KnowledgeBaseID != kbID || !c.IsEnabled {
			continue
		}
		if _, seen := aliveDocs[c.ID]; !seen {
			knowledgeIDs = append(knowledgeIDs, c.KnowledgeID)
		}
		aliveDocs[c.ID] = c.KnowledgeID
	}
	titles, err := t.knowledgeTitles(ctx, knowledgeIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to verify relation evidence: %w", err)
	}
	alive := make(map[string]bool, len(aliveDocs))
	for chunkID, knowledgeID := range aliveDocs {
		// titles holds only documents that still exist (nil when there is no
		// way to check); a chunk can outlive its soft-deleted document.
		if _, exists := titles[knowledgeID]; titles != nil && !exists {
			continue
		}
		alive[chunkID] = true
	}
	var verified []*types.GraphRelation
	for _, rel := range relations {
		if rel == nil {
			continue
		}
		if endpointEvidenceIntersects(rel.Node1Chunks, nodeChunks[rel.Node1], alive) &&
			endpointEvidenceIntersects(rel.Node2Chunks, nodeChunks[rel.Node2], alive) {
			verified = append(verified, rel)
		}
	}
	return verified, nil
}

func summarizeGraphConfig(config *types.ExtractConfig) graphConfigSummary {
	if config == nil {
		return graphConfigSummary{}
	}

	return graphConfigSummary{
		Nodes:     uniqueSortedNodeNames(config.Nodes),
		Relations: uniqueSortedRelationNames(config.Relations),
	}
}

func uniqueSortedNodeNames(nodes []*types.GraphNode) []string {
	seen := make(map[string]struct{}, len(nodes))
	names := make([]string, 0, len(nodes))
	for _, node := range nodes {
		if node == nil || node.Name == "" {
			continue
		}
		if _, exists := seen[node.Name]; exists {
			continue
		}
		seen[node.Name] = struct{}{}
		names = append(names, node.Name)
	}
	sort.Strings(names)
	return names
}

func uniqueSortedRelationNames(relations []*types.GraphRelation) []string {
	seen := make(map[string]struct{}, len(relations))
	names := make([]string, 0, len(relations))
	for _, relation := range relations {
		if relation == nil || relation.Type == "" {
			continue
		}
		if _, exists := seen[relation.Type]; exists {
			continue
		}
		seen[relation.Type] = struct{}{}
		names = append(names, relation.Type)
	}
	sort.Strings(names)
	return names
}

func graphConfigsToData(graphConfigs map[string]graphConfigSummary) map[string]map[string]interface{} {
	if len(graphConfigs) == 0 {
		return nil
	}

	data := make(map[string]map[string]interface{}, len(graphConfigs))
	for kbID, config := range graphConfigs {
		data[kbID] = map[string]interface{}{
			"nodes":     config.Nodes,
			"relations": config.Relations,
		}
	}
	return data
}

func aggregateGraphConfig(graphConfigs map[string]graphConfigSummary) map[string]interface{} {
	if len(graphConfigs) == 0 {
		return nil
	}

	merged := graphConfigSummary{}
	for _, config := range graphConfigs {
		merged.Nodes = append(merged.Nodes, config.Nodes...)
		merged.Relations = append(merged.Relations, config.Relations...)
	}

	return map[string]interface{}{
		"nodes":     uniqueStrings(merged.Nodes),
		"relations": uniqueStrings(merged.Relations),
	}
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

// buildGraphVisualizationData builds structured data for graph visualization:
// the entities and relations the graph returned, and the chunks.
func buildGraphVisualizationData(
	results []*types.SearchResult, relations []*types.GraphRelation,
) map[string]interface{} {
	nodes := make([]map[string]interface{}, 0)
	edges := make([]map[string]interface{}, 0)

	seenEntities := make(map[string]bool)
	addEntity := func(name string) {
		if name == "" || seenEntities["entity:"+name] {
			return
		}
		seenEntities["entity:"+name] = true
		nodes = append(nodes, map[string]interface{}{"id": "entity:" + name, "label": name, "type": "entity"})
	}
	for _, rel := range relations {
		addEntity(rel.Node1)
		addEntity(rel.Node2)
		edges = append(edges, map[string]interface{}{
			"source": "entity:" + rel.Node1, "target": "entity:" + rel.Node2, "label": rel.Type,
		})
	}
	for i, result := range results {
		if !seenEntities[result.ID] {
			nodes = append(nodes, map[string]interface{}{
				"id":       result.ID,
				"label":    fmt.Sprintf("Chunk %d", i+1),
				"content":  result.Content,
				"kb_id":    result.KnowledgeID,
				"kb_title": result.KnowledgeTitle,
				"score":    result.Score,
				"type":     "chunk",
			})
			seenEntities[result.ID] = true
		}
	}

	return map[string]interface{}{
		"nodes":       nodes,
		"edges":       edges,
		"total_nodes": len(nodes),
		"total_edges": len(edges),
	}
}
