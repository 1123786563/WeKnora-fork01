package service

// Wiki（23-knowledge-wikifaq K3.1/W0）消费侧 seam 定义：随上游对齐 round 2
// 自 internal/modules/knowledge/wiki/seams.go 归位至 service 包。原 Span 装箱
// 句柄（Span/NewSpan/box SpanTracker/noopSpanTracker）随包合并删除——wiki
// 侧代码现与宿主共享 knowledge_span_tracker.go 的具名 *Span/SpanTracker
// （句柄始终按不透明指针传递，装箱/解箱仅在迁移期适配器存在）。
//
// Seams 汇集 wiki 域服务对宿主纯函数符号的依赖端口。由构造兼容层
//（wiki_k3_ctor_compat.go 的 wikiK3Seams）接线；零值仅允许出现在不触达
// seam 调用点的测试构造中。

import (
	"context"

	apprepo "github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// Wiki 域哨兵别名：真源在 repository 包（wiki_k3_repo_compat.go），
// errors.Is 链身份不变。原 wiki 包内同名别名随包合并删除。
var (
	ErrWikiPageNotFound   = apprepo.ErrWikiPageNotFound
	ErrWikiPageConflict   = apprepo.ErrWikiPageConflict
	ErrWikiFolderNotFound = apprepo.ErrWikiFolderNotFound
	ErrWikiFolderConflict = apprepo.ErrWikiFolderConflict
	ErrWikiFolderNotEmpty = apprepo.ErrWikiFolderNotEmpty
)

// Seams 汇集 wiki 域服务依赖端口（签名照录宿主符号）。
type Seams struct {
	// ResolveDeadSlug 照录宿主 K2 符号 resolveDeadSlug（slug_fuzzy.go:91）
	// 的签名：死链模糊复活（显示名反查 / 连字符归一 / bigram 相似度）。
	ResolveDeadSlug ResolveDeadSlugFn

	// FinalizeSubtaskDetached 照录宿主 K4 符号 finalizeSubtaskDetached
	//（knowledge.go:235）：脱离上下文递减 knowledge 的 finalizing 子任务
	// 计数。repo 为 nil 时实现自身为 no-op（宿主实现的语义）。
	FinalizeSubtaskDetached FinalizeSubtaskDetachedFn

	// IsLikelyRateLimitError 照录宿主 K4 符号 isLikelyRateLimitError
	//（knowledge_process.go:3902）：错误文本启发式判定上游限流。
	IsLikelyRateLimitError IsLikelyRateLimitErrorFn

	// RemoveSourceRef 照录宿主 K4 符号 removeSourceRef
	//（knowledge_delete.go:346）：从 source_refs 剔除指定 knowledge ID。
	RemoveSourceRef RemoveSourceRefFn

	// RecordWikiContentActivity 照录宿主 K2 导出符号
	// RecordWikiContentActivity（kb_activity.go:181）：把 Wiki 变更摘要投影
	// 进知识库活动流。
	RecordWikiContentActivity RecordWikiContentActivityFn

	// IsKnowledgeBaseNotFound 判定错误是否为 repository 包哨兵
	// ErrKnowledgeBaseNotFound。
	IsKnowledgeBaseNotFound func(err error) bool
}

// isKnowledgeBaseNotFound 是 IsKnowledgeBaseNotFound 的 nil 安全形态：
// 未接线（零值构造的测试 service）时返回 false——生产路径恒经装配层接线，
// 该回落仅在测试构造不触达 KB 判定时出现（与迁移前 errors.Is 对非哨兵
// 错误返回 false 的行为一致）。
func (s Seams) isKnowledgeBaseNotFound(err error) bool {
	if s.IsKnowledgeBaseNotFound == nil {
		return false
	}
	return s.IsKnowledgeBaseNotFound(err)
}

// ResolveDeadSlugFn mirrors the host resolveDeadSlug signature (K2,
// slug_fuzzy.go:91). Both maps are consulted only — never mutated.
type ResolveDeadSlugFn = func(deadSlug string, displayText string, liveSlugs map[string]struct{}, titleToSlug map[string]string) (string, bool)

// FinalizeSubtaskDetachedFn mirrors the host finalizeSubtaskDetached
// signature (K4, knowledge.go:235): detached finalizing-counter decrement
// for one knowledge's named subtask slot.
type FinalizeSubtaskDetachedFn = func(ctx context.Context, repo interfaces.KnowledgeRepository, knowledgeID, source string, retErr error, superseded, final bool)

// IsLikelyRateLimitErrorFn mirrors the host isLikelyRateLimitError
// signature (K4, knowledge_process.go:3902).
type IsLikelyRateLimitErrorFn = func(err error) bool

// RemoveSourceRefFn mirrors the host removeSourceRef signature
// (K4, knowledge_delete.go:346).
type RemoveSourceRefFn = func(refs types.StringArray, knowledgeID string) types.StringArray

// RecordWikiContentActivityFn mirrors the host RecordWikiContentActivity
// signature (K2, kb_activity.go:181).
type RecordWikiContentActivityFn = func(ctx context.Context, audit interfaces.AuditLogService, tenantID uint64, kbID string, actions map[string]int)
