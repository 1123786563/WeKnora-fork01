package service

import (
	"context"
	"fmt"
	"math"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

func TestFuseOrDeduplicate_KeywordOnlyRescalesUnboundedBM25(t *testing.T) {
	t.Parallel()

	got := fuseOrDeduplicate(context.Background(), nil, [][]*types.IndexWithScore{[]*types.IndexWithScore{
		{ChunkID: "strong", Score: 16.1239},
		{ChunkID: "mid", Score: 8.06195},
		{ChunkID: "weak", Score: 4.030975},
	}}, nil)

	require.Len(t, got, 3)
	require.Equal(t, "strong", got[0].ChunkID)
	require.InDelta(t, 1.0, got[0].Score, 1e-9)
	require.InDelta(t, 0.5, got[1].Score, 1e-9)
	require.InDelta(t, 0.25, got[2].Score, 1e-9)

	// 0.3*base stays below the composite clamp, so model score can still discriminate.
	require.Less(t, 0.3*got[0].Score, 1.0)
	require.Less(t, 0.3*got[1].Score, 0.3*got[0].Score)
}

func TestFuseOrDeduplicate_KeywordOnlyLeavesUnitIntervalScores(t *testing.T) {
	t.Parallel()

	flat := fuseOrDeduplicate(context.Background(), nil, [][]*types.IndexWithScore{[]*types.IndexWithScore{
		{ChunkID: "a", Score: 1.0},
		{ChunkID: "b", Score: 1.0},
	}}, nil)
	require.Len(t, flat, 2)
	require.InDelta(t, 1.0, flat[0].Score, 1e-9)
	require.InDelta(t, 1.0, flat[1].Score, 1e-9)

	bounded := fuseOrDeduplicate(context.Background(), nil, [][]*types.IndexWithScore{[]*types.IndexWithScore{
		{ChunkID: "high", Score: 0.8},
		{ChunkID: "low", Score: 0.4},
	}}, nil)
	require.Equal(t, "high", bounded[0].ChunkID)
	require.InDelta(t, 0.8, bounded[0].Score, 1e-9)
	require.InDelta(t, 0.4, bounded[1].Score, 1e-9)
}

func TestFuseOrDeduplicate_VectorOnlyKeepsEmbeddingScores(t *testing.T) {
	t.Parallel()

	got := fuseOrDeduplicate(context.Background(), [][]*types.IndexWithScore{[]*types.IndexWithScore{
		{ChunkID: "near", Score: 0.91},
		{ChunkID: "far", Score: 0.22},
	}}, nil, nil)

	require.Equal(t, "near", got[0].ChunkID)
	require.InDelta(t, 0.91, got[0].Score, 1e-9)
	require.InDelta(t, 0.22, got[1].Score, 1e-9)
}

func TestFuseOrDeduplicate_HybridUsesRRFNotRawBM25(t *testing.T) {
	t.Parallel()

	got := fuseOrDeduplicate(context.Background(),
		[][]*types.IndexWithScore{{{ChunkID: "vec", Score: 0.9}}},
		[][]*types.IndexWithScore{{{ChunkID: "kw", Score: 16.1239}}},
		nil,
	)

	require.Len(t, got, 2)
	for _, hit := range got {
		require.Greater(t, hit.Score, 0.0)
		require.Less(t, hit.Score, 1.0)
		require.NotEqual(t, 16.1239, hit.Score)
	}
}

func TestRescaleUnboundedScores_IgnoresNonFiniteWhenFindingMax(t *testing.T) {
	t.Parallel()

	hits := []*types.IndexWithScore{
		{ChunkID: "nan", Score: math.NaN()},
		{ChunkID: "top", Score: 10},
		{ChunkID: "low", Score: 5},
		nil,
	}
	rescaleUnboundedScores(hits)
	require.Equal(t, 0.0, hits[0].Score)
	require.InDelta(t, 1.0, hits[1].Score, 1e-9)
	require.InDelta(t, 0.5, hits[2].Score, 1e-9)
}

// TestFuseOrDeduplicate_HybridRanksPerListNotListPosition reproduces the
// issue #3796 shape: a FAQ index list returns first (goroutine completion
// order) with many mediocre hits, the document index list returns second
// carrying the globally best chunk. Ranks must come from each list's own
// score order — never from the position in the concatenation, where the FAQ
// hits would occupy vector rank 1..N and bury the best document chunk deep
// enough to fall off the MatchCount cut.
func TestFuseOrDeduplicate_HybridRanksPerListNotListPosition(t *testing.T) {
	t.Parallel()

	faqList := make([]*types.IndexWithScore, 0, 40)
	for i := 0; i < 40; i++ {
		faqList = append(faqList, &types.IndexWithScore{
			ChunkID: fmt.Sprintf("faq-%02d", i),
			Score:   0.55 - float64(i)*0.001,
		})
	}
	// Document list delivered with its best chunk last, to also pin that
	// each list is sorted by score before ranks are read off.
	docList := []*types.IndexWithScore{
		{ChunkID: "doc-2", Score: 0.90},
		{ChunkID: "doc-3", Score: 0.85},
		{ChunkID: "doc-1", Score: 0.96},
	}
	keywordList := []*types.IndexWithScore{
		{ChunkID: "doc-1", Score: 16.1239},
		{ChunkID: "doc-2", Score: 8.06195},
	}

	got := fuseOrDeduplicate(context.Background(),
		[][]*types.IndexWithScore{faqList, docList},
		[][]*types.IndexWithScore{keywordList},
		nil,
	)

	require.Len(t, got, 43)
	// Best document chunk leads: vector rank 1 of its own list + keyword
	// rank 1 — regardless of arriving after 40 FAQ hits.
	require.Equal(t, "doc-1", got[0].ChunkID)
	require.InDelta(t, 1.0, got[0].Score, 1e-9)
	// doc-2: vector rank 2 (own list) + keyword rank 2.
	require.Equal(t, "doc-2", got[1].ChunkID)
	require.InDelta(t, 0.7*61.0/62.0+0.3*61.0/62.0, got[1].Score, 1e-9)
	// faq-00 keeps vector rank 1 of its own list instead of sharing the
	// 1..43 positional range the concatenation would hand out.
	require.Equal(t, "faq-00", got[2].ChunkID)
	require.InDelta(t, 0.7, got[2].Score, 1e-9)
}

// TestFuseOrDeduplicate_SameInputFusesIdentically pins determinism: lists
// delivered out of score order and with equal scores must fuse to the exact
// same ranking on every call. Assembly from randomized map iteration plus an
// unstable sort used to let tied chunks trade places between calls, so the
// downstream MatchCount cut could drop a different tied chunk each time.
func TestFuseOrDeduplicate_SameInputFusesIdentically(t *testing.T) {
	t.Parallel()

	// fusion overwrites each hit's Score with its fused score, so every
	// iteration fuses a fresh allocation of the same logical input —
	// exactly what concurrent retrieval produces per request.
	newVectorLists := func() [][]*types.IndexWithScore {
		return [][]*types.IndexWithScore{{
			{ChunkID: "v-a", Score: 0.9},
			{ChunkID: "v-b", Score: 0.9},
			{ChunkID: "v-c", Score: 0.4},
			{ChunkID: "v-d", Score: 0.4},
			{ChunkID: "v-e", Score: 0.4},
			{ChunkID: "v-f", Score: 0.4},
		}}
	}
	newKeywordLists := func() [][]*types.IndexWithScore {
		return [][]*types.IndexWithScore{{
			{ChunkID: "v-b", Score: 8.0},
			{ChunkID: "k-a", Score: 6.0},
			{ChunkID: "k-b", Score: 6.0},
		}}
	}

	type fused struct {
		id    string
		score float64
	}
	snapshot := func(out []*types.IndexWithScore) []fused {
		s := make([]fused, len(out))
		for i, r := range out {
			s[i] = fused{r.ChunkID, r.Score}
		}
		return s
	}

	got := fuseOrDeduplicate(context.Background(), newVectorLists(), newKeywordLists(), nil)
	first := snapshot(got)
	for i := 0; i < 32; i++ {
		require.Equal(t, first,
			snapshot(fuseOrDeduplicate(context.Background(), newVectorLists(), newKeywordLists(), nil)))
	}

	// Ranks follow the stable score order of the input: v-a precedes v-b
	// (tie kept in supplied order), so v-b carries vector rank 2 plus its
	// keyword rank 1, and the equal-score v-c..v-f keep their input order.
	require.Equal(t, "v-b", got[0].ChunkID)
	require.InDelta(t, 0.7*61.0/62.0+0.3, got[0].Score, 1e-9)
	require.Equal(t, "v-a", got[1].ChunkID)
	require.InDelta(t, 0.7, got[1].Score, 1e-9)
	require.Equal(t, []string{"v-c", "v-d", "v-e", "v-f"},
		[]string{got[2].ChunkID, got[3].ChunkID, got[4].ChunkID, got[5].ChunkID})
	require.Equal(t, []string{"k-a", "k-b"}, []string{got[6].ChunkID, got[7].ChunkID})

	// The vector-only dedup path is equally deterministic.
	newSingle := func() [][]*types.IndexWithScore {
		return [][]*types.IndexWithScore{{
			{ChunkID: "s-b", Score: 0.5},
			{ChunkID: "s-a", Score: 0.5},
			{ChunkID: "s-c", Score: 0.8},
		}}
	}
	firstSingle := snapshot(fuseOrDeduplicate(context.Background(), newSingle(), nil, nil))
	for i := 0; i < 32; i++ {
		require.Equal(t, firstSingle,
			snapshot(fuseOrDeduplicate(context.Background(), newSingle(), nil, nil)))
	}
	require.Equal(t, []string{"s-c", "s-b", "s-a"},
		[]string{firstSingle[0].id, firstSingle[1].id, firstSingle[2].id})
}

// TestFuseOrDeduplicate_SingleListHybridExpectations pins the single-KB
// arithmetic (one vector list + one keyword list, k=60, weights 0.7/0.3,
// maxRRF = 1/61) so the multi-list ranking fixes cannot drift it.
func TestFuseOrDeduplicate_SingleListHybridExpectations(t *testing.T) {
	t.Parallel()

	got := fuseOrDeduplicate(context.Background(),
		[][]*types.IndexWithScore{
			{{ChunkID: "a", Score: 0.9}, {ChunkID: "b", Score: 0.5}},
		},
		[][]*types.IndexWithScore{
			{{ChunkID: "a", Score: 16.0}, {ChunkID: "c", Score: 8.0}},
		},
		nil,
	)

	require.Len(t, got, 3)
	require.Equal(t, "a", got[0].ChunkID) // vector rank 1 + keyword rank 1
	require.InDelta(t, 1.0, got[0].Score, 1e-9)
	require.Equal(t, "b", got[1].ChunkID) // vector rank 2 only
	require.InDelta(t, 0.7*61.0/62.0, got[1].Score, 1e-9)
	require.Equal(t, "c", got[2].ChunkID) // keyword rank 2 only
	require.InDelta(t, 0.3*61.0/62.0, got[2].Score, 1e-9)
}

// TestFuseOrDeduplicate_KeywordListsRankedPerList is the keyword-side
// counterpart of the multi-list vector case: two keyword lists (multi-store
// fan-out, each with its own BM25 scale) arrive in completion order, and a
// hit topping its own list must get keyword rank 1 — not rank 3 behind the
// first list's hits.
func TestFuseOrDeduplicate_KeywordListsRankedPerList(t *testing.T) {
	t.Parallel()

	got := fuseOrDeduplicate(context.Background(),
		[][]*types.IndexWithScore{
			{{ChunkID: "a", Score: 0.9}, {ChunkID: "z", Score: 0.8}},
		},
		[][]*types.IndexWithScore{
			{{ChunkID: "x", Score: 12.0}, {ChunkID: "y", Score: 11.0}},
			{{ChunkID: "z", Score: 2.0}},
		},
		nil,
	)

	require.Len(t, got, 4)
	// z: vector rank 2 + keyword rank 1 of its own second list. It must
	// not inherit the concatenated keyword position (rank 3), which would
	// bury it behind the first list's hits.
	require.Equal(t, "z", got[0].ChunkID)
	require.InDelta(t, 0.7*61.0/62.0+0.3, got[0].Score, 1e-9)
	require.Greater(t, got[0].Score, 0.7*61.0/62.0+0.3*61.0/63.0)
	require.Equal(t, "a", got[1].ChunkID) // vector rank 1 only
	require.InDelta(t, 0.7, got[1].Score, 1e-9)
	require.Equal(t, "x", got[2].ChunkID) // keyword rank 1 (first list) only
	require.InDelta(t, 0.3, got[2].Score, 1e-9)
	require.Equal(t, "y", got[3].ChunkID) // keyword rank 2 (first list) only
	require.InDelta(t, 0.3*61.0/62.0, got[3].Score, 1e-9)
}
