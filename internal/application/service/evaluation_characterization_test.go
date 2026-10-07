package service
// B3-IN.2 特征化测试（characterization，37-insights §4）：锚定 evaluation 面
// （evaluation.go / metric_hook.go）迁移前的现行为，供 B3-IN.4 迁移后同用例
// 在模块位复跑做差分等价比对（conventions §1.4「搬迁类任务先写特征化测试」/§6）。

import (
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

// TestGetPassageListGolden 钉住 getPassageList 的稀疏 PID 映射行为：
// PIDs 与 Passages 按下标配对，结果按 maxPID 定长、未出现的槽为零值。
func TestGetPassageListGolden(t *testing.T) {
	dataset := []*types.QAPair{{
		PIDs:     []int{2, 0},
		Passages: []string{"p2", "p0"},
	}}

	got := getPassageList(dataset)

	// maxPID=2 → 长度 3；槽 1 无 PID 命中，保持零值 ""。
	require.Equal(t, []string{"p0", "", "p2"}, got)
}

// TestMetricListAppendAvg 钉住 MetricList.Append/Avg 的聚合语义：
// 受控输入（GT={1,2}、检索=[1]、生成文本与 GT 相同）下 Precision=1.0、Recall=0.5；
// 同输入两次 Append 后 Avg 与单次结果一致；空表 Avg 返回非 nil 零值。
func TestMetricListAppendAvg(t *testing.T) {
	input := &types.MetricInput{
		RetrievalGT:    [][]int{{1, 2}},
		RetrievalIDs:   []int{1},
		GeneratedGT:    "answer",
		GeneratedTexts: "answer",
	}

	single := &MetricList{}
	single.Append(input)
	singleAvg := single.Avg()
	require.Equal(t, 1.0, singleAvg.RetrievalMetrics.Precision)
	require.Equal(t, 0.5, singleAvg.RetrievalMetrics.Recall)

	doubled := &MetricList{}
	doubled.Append(input)
	doubled.Append(input)
	doubledAvg := doubled.Avg()
	require.Equal(t, singleAvg.RetrievalMetrics.Precision, doubledAvg.RetrievalMetrics.Precision)
	require.Equal(t, singleAvg.RetrievalMetrics.Recall, doubledAvg.RetrievalMetrics.Recall)

	empty := (&MetricList{}).Avg()
	require.NotNil(t, empty)
	require.Equal(t, &types.MetricResult{}, empty)
}

// TestHookMetricRecordFinishMapsContentToPID 钉住 recordFinish 的内容回映射
// （metric_hook.go:147-167）：检索 chunk 的 Content 与 ground truth passage
// 做子串互含匹配，命中则映射回该 passage 的 PID（而非 ChunkIndex）。
// GT={7,9}、检索 chunk 内容 "alpha" 仅命中 passage[0]（PID 7）时，
// retrievalIDs 恰为 [7] 当且仅当 Precision==1.0 且 Recall==0.5。
func TestHookMetricRecordFinishMapsContentToPID(t *testing.T) {
	hook := NewHookMetric(1)
	hook.recordInit(0)
	hook.recordQaPair(0, &types.QAPair{
		PIDs:     []int{7, 9},
		Passages: []string{"alpha passage", "beta passage"},
		Answer:   "ans",
	})
	hook.recordSearchResult(0, []*types.SearchResult{{Content: "alpha"}})
	hook.recordFinish(0)

	got := hook.MetricResult()
	require.Equal(t, 1.0, got.RetrievalMetrics.Precision)
	require.Equal(t, 0.5, got.RetrievalMetrics.Recall)
}
