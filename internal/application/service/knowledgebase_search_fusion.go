package service

import (
	"context"
	"math"
	"slices"

	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
)

// classifyRetrievalResults separates retrieval results by retriever type
// (vector vs keyword). Each RetrieveResult stays its own list: they come from
// different engines, store groups and parameter sets (document vs FAQ), are
// appended in goroutine completion order, and their positions are only
// meaningful within the list.
func classifyRetrievalResults(ctx context.Context, retrieveResults []*types.RetrieveResult) (
	vectorLists, keywordLists [][]*types.IndexWithScore,
) {
	for _, retrieveResult := range retrieveResults {
		logger.Infof(ctx, "Retrieval results, engine: %v, retriever: %v, count: %v",
			retrieveResult.RetrieverEngineType,
			retrieveResult.RetrieverType,
			len(retrieveResult.Results),
		)
		if len(retrieveResult.Results) == 0 {
			continue
		}
		if retrieveResult.RetrieverType == types.VectorRetrieverType {
			vectorLists = append(vectorLists, retrieveResult.Results)
		} else {
			keywordLists = append(keywordLists, retrieveResult.Results)
		}
	}
	return
}

// flattenLists concatenates retrieval lists.
func flattenLists(lists [][]*types.IndexWithScore) []*types.IndexWithScore {
	var out []*types.IndexWithScore
	for _, list := range lists {
		out = append(out, list...)
	}
	return out
}

// fuseOrDeduplicate either fuses vector+keyword results via RRF or deduplicates
// single-retriever results. Every path returns scores in [0, 1] so results of
// separate searches (per-document targets, embedding-model groups, FAQ vs
// document calls) can be ranked together downstream:
//   - vector only: the engines' cosine similarity, unchanged (FAQ thresholds
//     depend on it);
//   - keyword only: BM25 divided by the best score of its own list;
//   - hybrid: weighted RRF divided by its maximum (a chunk ranked first by
//     both retrievers scores 1).
//
// retrievalCfg may be nil — defaults are then used for RRF parameters.
func fuseOrDeduplicate(
	ctx context.Context, vectorLists, keywordLists [][]*types.IndexWithScore, retrievalCfg *types.RetrievalConfig,
) []*types.IndexWithScore {
	if len(keywordLists) == 0 {
		// Vector-only: keep original embedding scores (important for FAQ)
		result := deduplicateByScore(flattenLists(vectorLists))
		logger.Infof(ctx, "Result count after deduplication: %d", len(result))
		return result
	}
	if len(vectorLists) == 0 {
		// Keyword-only: keep relative BM25 order, but fold unbounded
		// scores into [0, 1] before they reach rerank/MMR. Raw BM25
		// (often >10) saturates compositeScore's 0.3*base term.
		result := deduplicateByScore(flattenLists(keywordLists))
		rescaleUnboundedScores(result)
		logger.Infof(ctx, "Result count after deduplication: %d", len(result))
		return result
	}
	// Hybrid: use RRF fusion to merge vector + keyword results
	result := fuseWithRRF(ctx, vectorLists, keywordLists, retrievalCfg)
	logger.Infof(ctx, "Result count after RRF fusion: %d", len(result))
	return result
}

// sortByScoreDesc is a reusable sort comparator for IndexWithScore slices (descending by Score).
func sortByScoreDesc(a, b *types.IndexWithScore) int {
	if a.Score > b.Score {
		return -1
	} else if a.Score < b.Score {
		return 1
	}
	return 0
}

// deduplicateByScore deduplicates retrieval results by chunk ID, keeping the highest score
// for each chunk. Returns the results sorted by score descending.
// Used when only a single retriever (e.g. vector-only for FAQ) is active.
//
// The output is assembled in input order and sorted with a stable sort:
// Go randomizes map iteration, so emitting straight from the chunk map made
// equal-score chunks trade places between calls and left the downstream
// MatchCount cut free to drop a different tied chunk each time.
func deduplicateByScore(results []*types.IndexWithScore) []*types.IndexWithScore {
	chunkInfoMap := make(map[string]*types.IndexWithScore, len(results))
	for _, r := range results {
		if existing, exists := chunkInfoMap[r.ChunkID]; !exists || r.Score > existing.Score {
			chunkInfoMap[r.ChunkID] = r
		}
	}
	deduped := make([]*types.IndexWithScore, 0, len(chunkInfoMap))
	emitted := make(map[string]struct{}, len(chunkInfoMap))
	for _, r := range results {
		if _, done := emitted[r.ChunkID]; done {
			continue
		}
		// Emit each chunk at the position of the entry that survived the
		// dedup map (its best-scoring occurrence). Pointer identity tells
		// the winning row from ties that lost to an earlier equal score.
		if chunkInfoMap[r.ChunkID] == r {
			emitted[r.ChunkID] = struct{}{}
			deduped = append(deduped, r)
		}
	}
	slices.SortStableFunc(deduped, sortByScoreDesc)
	return deduped
}

// rescaleUnboundedScores maps a single-retriever candidate set onto [0, 1]
// by dividing through the maximum finite score. Keyword (BM25) scores are
// unbounded; leaving them in place saturates compositeScore (0.3*base with
// a [0, 1] clamp) so every candidate ties at 1.0 and MMR degenerates to
// diversity-only ordering.
//
// Scores already in [0, 1] are left unchanged so engines that stamp
// keyword hits at 1.0 (Qdrant, Doris) and already-normalized vector
// scores keep their current magnitude. Relative order is preserved
// either way. Retrieve/Langfuse sampling happens before fusion, so
// raw BM25 remains visible in the retrieve span.
func rescaleUnboundedScores(results []*types.IndexWithScore) {
	maxScore := 0.0
	for _, r := range results {
		if r == nil {
			continue
		}
		if math.IsNaN(r.Score) || math.IsInf(r.Score, 0) {
			continue
		}
		if r.Score > maxScore {
			maxScore = r.Score
		}
	}
	if maxScore <= 1 {
		return
	}
	for _, r := range results {
		if r == nil {
			continue
		}
		switch {
		case math.IsNaN(r.Score), math.IsInf(r.Score, -1), r.Score <= 0:
			r.Score = 0
		case math.IsInf(r.Score, 1):
			r.Score = 1
		default:
			r.Score = r.Score / maxScore
		}
	}
}

// fuseWithRRF merges vector and keyword retrieval results using Reciprocal Rank Fusion.
// RRF score = vectorWeight/(k+vectorRank) + keywordWeight/(k+keywordRank),
// divided by its maximum (vectorWeight+keywordWeight)/(k+1) so it lands in
// [0, 1] like the single-retriever paths. Raw RRF tops out near 0.016, which
// made hybrid hits lose to any vector-only or keyword-only result they were
// ranked against and left rerank's base-score term and MMR's relevance term
// with nothing to work with.
//
// A chunk's rank for a retriever is its best position in any one list of
// that retriever, each list ordered by its own score (see bestRanks).
// Positions in the concatenation of several lists are arbitrary: the second
// list's best hit would otherwise rank behind every hit of the first.
// k, vectorWeight and keywordWeight are sourced from retrievalCfg (with defaults).
// The merged results are sorted by RRF score descending; equal RRF scores keep
// the deterministic first-seen assembly order below.
func fuseWithRRF(
	ctx context.Context, vectorLists, keywordLists [][]*types.IndexWithScore, retrievalCfg *types.RetrievalConfig,
) []*types.IndexWithScore {
	rrfK := retrievalCfg.GetEffectiveRRFK()
	vectorWeight, keywordWeight := retrievalCfg.GetEffectiveRRFWeights()
	maxRRF := (vectorWeight + keywordWeight) / float64(rrfK+1)

	vectorRanks := bestRanks(vectorLists)
	keywordRanks := bestRanks(keywordLists)

	// Collect all unique chunks — prefer vector result's metadata for each
	// chunk — assembling them in first-seen order over the flattened lists.
	// The final sort below only orders by RRF score, so equal scores would
	// otherwise fall back to Go's randomized map iteration order and the
	// downstream MatchCount cut could drop a different tied chunk per call.
	chunkInfoMap := make(map[string]*types.IndexWithScore)
	firstSeen := make([]*types.IndexWithScore, 0, len(flattenLists(vectorLists)))
	for _, r := range flattenLists(vectorLists) {
		if existing, exists := chunkInfoMap[r.ChunkID]; !exists {
			chunkInfoMap[r.ChunkID] = r
			firstSeen = append(firstSeen, r)
		} else if r.Score > existing.Score {
			chunkInfoMap[r.ChunkID] = r
		}
	}
	for _, r := range flattenLists(keywordLists) {
		if _, exists := chunkInfoMap[r.ChunkID]; !exists {
			chunkInfoMap[r.ChunkID] = r
			firstSeen = append(firstSeen, r)
		}
	}

	// Compute weighted RRF scores and assign to each chunk
	result := make([]*types.IndexWithScore, 0, len(firstSeen))
	for _, first := range firstSeen {
		info := chunkInfoMap[first.ChunkID]
		chunkID := info.ChunkID
		rrfScore := 0.0
		if rank, ok := vectorRanks[chunkID]; ok {
			rrfScore += vectorWeight / float64(rrfK+rank)
		}
		if rank, ok := keywordRanks[chunkID]; ok {
			rrfScore += keywordWeight / float64(rrfK+rank)
		}
		info.Score = rrfScore / maxRRF
		result = append(result, info)
	}
	slices.SortStableFunc(result, sortByScoreDesc)

	// Log top results for debugging
	for i, chunk := range result {
		if i >= 15 {
			break
		}
		vRank, vOk := vectorRanks[chunk.ChunkID]
		kRank, kOk := keywordRanks[chunk.ChunkID]
		logger.Debugf(ctx, "RRF rank %d: chunk_id=%s, rrf_score=%.6f, vector_rank=%v(%v), keyword_rank=%v(%v)",
			i, chunk.ChunkID, chunk.Score, vRank, vOk, kRank, kOk)
	}

	return result
}

// bestRanks returns each chunk's best 1-based rank across lists, ranking every
// list on its own by score (a chunk repeated in a list keeps its best score).
//
// Each list is stably sorted by its own retriever's score (cosine for vector
// lists, BM25 for keyword lists) before ranks are read off, so a chunk's rank
// is its score position — never its position in the slice, whose order comes
// from goroutine completion in the fan-out. The stable sort keeps equal-score
// chunks in their supplied order, so the same input always yields the same
// ranks for ties as well.
func bestRanks(lists [][]*types.IndexWithScore) map[string]int {
	ranks := make(map[string]int)
	for _, list := range lists {
		sorted := slices.Clone(list)
		slices.SortStableFunc(sorted, sortByScoreDesc)
		seen := make(map[string]struct{}, len(sorted))
		rank := 0
		for _, r := range sorted {
			if _, dup := seen[r.ChunkID]; dup {
				continue
			}
			seen[r.ChunkID] = struct{}{}
			rank++
			if current, ok := ranks[r.ChunkID]; !ok || rank < current {
				ranks[r.ChunkID] = rank
			}
		}
	}
	return ranks
}
