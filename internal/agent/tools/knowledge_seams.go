package tools

// R2 消费侧 seam（owner: 32-agentruntime-tools，remove_at: ib3）。
// 本文件按 32 计划 §0.4/§0.5 收敛 tools 包对 knowledge 模块 searchutil
// 子包的深 import：仅允许类型别名 / 常量别名 / 薄委托三种无逻辑形态
// （spec §4.4 过渡薄别名），单一真源仍在 knowledge 模块内；ib3 门面
// 合法化契约任务落地后翻转本文件 import 至模块根并随之删除。
// 签名中的 internal/types 与 internal/types/interfaces 为宿主共享包
// （tools 包既有 import，非模块跨边），不产生新的模块深 import 边。
// 例外登记：guard importExceptions 1 行 + exception-ledger exc-0148
// （Ruling 2026-09-24-IMPORT-EXCEPTION-REGISTRY）。

import (
	"context"

	"github.com/Tencent/WeKnora/internal/searchutil"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// ---- 薄委托函数（单一真源，签名逐字取自源）----

// BuildContentSignature R2 seam（remove_at: ib3）— 门面合法化后随本文件消除
func BuildContentSignature(content string) string { // searchutil/textutil.go:14
	return searchutil.BuildContentSignature(content)
}

// TokenizeSimple R2 seam（remove_at: ib3）— 门面合法化后随本文件消除
func TokenizeSimple(text string) map[string]struct{} { // searchutil/textutil.go:40
	return searchutil.TokenizeSimple(text)
}

// Jaccard R2 seam（remove_at: ib3）— 门面合法化后随本文件消除
func Jaccard(a, b map[string]struct{}) float64 { // searchutil/textutil.go:77
	return searchutil.Jaccard(a, b)
}

// ClampFloat R2 seam（remove_at: ib3）— 门面合法化后随本文件消除
func ClampFloat(v, minV, maxV float64) float64 { // searchutil/textutil.go:154
	return searchutil.ClampFloat(v, minV, maxV)
}

// CollectImageInfoByChunkIDs R2 seam（remove_at: ib3）— 门面合法化后随本文件消除
func CollectImageInfoByChunkIDs( // searchutil/imageinfo.go:55
	ctx context.Context,
	chunkRepo interfaces.ChunkRepository,
	tenantID uint64,
	chunkIDs []string,
) map[string]string {
	return searchutil.CollectImageInfoByChunkIDs(ctx, chunkRepo, tenantID, chunkIDs)
}

// EnrichSearchResultsImageInfo R2 seam（remove_at: ib3）— 门面合法化后随本文件消除
func EnrichSearchResultsImageInfo( // searchutil/imageinfo.go:164
	ctx context.Context,
	chunkRepo interfaces.ChunkRepository,
	tenantID uint64,
	results []*types.SearchResult,
) {
	searchutil.EnrichSearchResultsImageInfo(ctx, chunkRepo, tenantID, results)
}

// BuildImageInfoMarkdownWithURL R2 seam（remove_at: ib3）— 门面合法化后随本文件消除
func BuildImageInfoMarkdownWithURL(url string, img *types.ImageInfo) string { // searchutil/imageinfo.go:434
	return searchutil.BuildImageInfoMarkdownWithURL(url, img)
}
