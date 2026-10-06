// Pass B (23-knowledge-wikifaq) 过渡 shim — 删除点 ib2（Integration Brief 登记）。
//
// dig 装配（container.go:431/432/433/723）仍 Provide 本包的三个 wiki 构造器；
// K3.1 迁移后它们转发到 internal/modules/knowledge/wiki 的实现，并把 wiki 侧
// W0 seam（§5.2 K2/K4 属主符号）以闭包接到宿主现行符号。行为零变化：
// seam 全部指向迁移前同包直引的同一实现；span 适配器仅做宿主 *Span 与
// wiki 不透明句柄的装卸（wiki 代码从不读 span 字段）。
package service

import (
	"context"
	"errors"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/modules/knowledge/wiki"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/redis/go-redis/v9"
)

// wikiK3Seams 把 wiki 包对宿主 service 包（K2/K4 属主、ib2 前未迁出）纯函数
// 符号的依赖端口接到宿主现行实现——与迁移前同包直引完全相同的目标函数。
func wikiK3Seams() wiki.Seams {
	return wiki.Seams{
		ResolveDeadSlug:         resolveDeadSlug,
		FinalizeSubtaskDetached: finalizeSubtaskDetached,
		IsLikelyRateLimitError:  isLikelyRateLimitError,
		RemoveSourceRef:         removeSourceRef,
		RecordWikiContentActivity: func(
			ctx context.Context,
			audit interfaces.AuditLogService,
			tenantID uint64,
			kbID string,
			actions map[string]int,
		) {
			RecordWikiContentActivity(ctx, audit, tenantID, kbID, actions)
		},
		IsKnowledgeBaseNotFound: func(err error) bool {
			return errors.Is(err, repository.ErrKnowledgeBaseNotFound)
		},
	}
}

// NewWikiPageService creates a new wiki page service.
func NewWikiPageService(
	repo interfaces.WikiPageRepository,
	chunkRepo interfaces.ChunkRepository,
	kbService interfaces.KnowledgeBaseService,
	taskPendingRepo interfaces.TaskPendingOpsRepository,
	redisClient *redis.Client,
) interfaces.WikiPageService {
	return wiki.NewWikiPageService(repo, chunkRepo, kbService, taskPendingRepo, redisClient, wikiK3Seams(), nil)
}

// NewWikiIngestService creates a new wiki ingest service.
func NewWikiIngestService(
	wikiService interfaces.WikiPageService,
	kbService interfaces.KnowledgeBaseService,
	knowledgeSvc interfaces.KnowledgeService,
	knowledgeRepo interfaces.KnowledgeRepository,
	chunkRepo interfaces.ChunkRepository,
	modelService interfaces.ModelService,
	task interfaces.TaskEnqueuer,
	audit interfaces.AuditLogService,
	pendingRepo interfaces.TaskPendingOpsRepository,
	deadLetterRepo interfaces.TaskDeadLetterRepository,
	redisClient *redis.Client,
	spanTracker SpanTracker,
) interfaces.TaskHandler {
	return wiki.NewWikiIngestService(
		wikiService, kbService, knowledgeSvc, knowledgeRepo, chunkRepo,
		modelService, task, audit, pendingRepo, deadLetterRepo, redisClient,
		wikiK3SpanAdapter{inner: spanTracker}, wikiK3Seams(),
	)
}

// NewWikiLintService creates a new wiki lint service.
func NewWikiLintService(
	wikiService interfaces.WikiPageService,
	kbService interfaces.KnowledgeBaseService,
	knowledgeService interfaces.KnowledgeService,
) *wiki.WikiLintService {
	return wiki.NewWikiLintService(wikiService, kbService, knowledgeService, wikiK3Seams())
}

// wikiK3SpanAdapter 把宿主 SpanTracker（含宿主具名 *Span）适配到 wiki 包
// 的窄端口。wiki 侧 Span 是不透明句柄（仅原样回传 tracker），装卸保持
// nil 语义与 span 身份，行为等价。
type wikiK3SpanAdapter struct {
	inner SpanTracker
}

func (a wikiK3SpanAdapter) LatestAttempt(ctx context.Context, knowledgeID string) int {
	return a.inner.LatestAttempt(ctx, knowledgeID)
}

func (a wikiK3SpanAdapter) LookupStage(ctx context.Context, knowledgeID string, attempt int, stage string) *wiki.Span {
	return wiki.NewSpan(a.inner.LookupStage(ctx, knowledgeID, attempt, stage))
}

func (a wikiK3SpanAdapter) BeginSubSpan(ctx context.Context, parent *wiki.Span, name, kind string, input types.JSONMap) *wiki.Span {
	return wiki.NewSpan(a.inner.BeginSubSpan(ctx, a.hostSpan(parent), name, kind, input))
}

func (a wikiK3SpanAdapter) EndSpan(ctx context.Context, span *wiki.Span, output types.JSONMap) {
	a.inner.EndSpan(ctx, a.hostSpan(span), output)
}

func (a wikiK3SpanAdapter) FailSpan(ctx context.Context, span *wiki.Span, errorCode, errorMessage string, errorDetail error) {
	a.inner.FailSpan(ctx, a.hostSpan(span), errorCode, errorMessage, errorDetail)
}

func (a wikiK3SpanAdapter) SkipSpan(ctx context.Context, span *wiki.Span, reason string) {
	a.inner.SkipSpan(ctx, a.hostSpan(span), reason)
}

// hostSpan 解包 wiki 句柄回宿主 *Span；适配器自身是唯一装箱者。
func (a wikiK3SpanAdapter) hostSpan(s *wiki.Span) *Span {
	if s == nil {
		return nil
	}
	if raw := s.Raw(); raw != nil {
		if host, ok := raw.(*Span); ok {
			return host
		}
	}
	return nil
}
