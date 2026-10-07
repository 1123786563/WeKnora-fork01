// Pass B (23-knowledge-wikifaq) 过渡 shim — 删除点 ib2（Integration Brief 登记）。
//
// dig 装配（container.go）仍 Provide 本包的三个 wiki 构造器；wiki 域随上游
// 对齐 round 2 归位本包后，构造器真源已在本包（wiki_page.go / wiki_ingest.go /
// wiki_lint.go，seam 化新签名）。本文件保留旧装配签名作为 *DI 适配器：
// 把 W0 seam（K2/K4 属主符号）与宿主 SpanTracker 以闭包/直引接到现行实现，
// 行为零变化。span 装箱适配器（wikiK3SpanAdapter）已随包合并删除——wiki
// 侧代码现直接消费宿主具名 *Span/SpanTracker。
package service

import (
	"context"
	"errors"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/redis/go-redis/v9"
)

// wikiK3Seams 把 wiki 域对宿主（K2/K4 属主）纯函数符号的依赖端口接到
// 现行实现——与迁移前同包直引完全相同的目标函数。
func wikiK3Seams() Seams {
	return Seams{
		ResolveDeadSlug:         ResolveDeadSlug,
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

// NewWikiPageServiceDI 是 dig 装配面（container.go）的适配构造器：
// 保持搬迁前 5 参旧签名，seam 与 kbShareService 由本函数就地供给。
func NewWikiPageServiceDI(
	repo interfaces.WikiPageRepository,
	chunkRepo interfaces.ChunkRepository,
	kbService interfaces.KnowledgeBaseService,
	taskPendingRepo interfaces.TaskPendingOpsRepository,
	redisClient *redis.Client,
) interfaces.WikiPageService {
	return NewWikiPageService(repo, chunkRepo, kbService, taskPendingRepo, redisClient, wikiK3Seams(), nil)
}

// NewWikiIngestServiceDI 是 dig 装配面（dig.Name("wikiIngest")）的适配
// 构造器：保持搬迁前旧签名（末参 spanTracker SpanTracker），seam 与
// tracker 由本函数就地供给；宿主 SpanTracker 现被 wiki 侧直接消费。
func NewWikiIngestServiceDI(
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
	return NewWikiIngestService(
		wikiService, kbService, knowledgeSvc, knowledgeRepo, chunkRepo,
		modelService, task, audit, pendingRepo, deadLetterRepo, redisClient,
		spanTracker, wikiK3Seams(),
	)
}

// NewWikiLintServiceDI 是 dig 装配面（container.go）的适配构造器：
// 保持搬迁前 3 参旧签名，seam 由本函数就地供给。
func NewWikiLintServiceDI(
	wikiService interfaces.WikiPageService,
	kbService interfaces.KnowledgeBaseService,
	knowledgeService interfaces.KnowledgeService,
) *WikiLintService {
	return NewWikiLintService(wikiService, kbService, knowledgeService, wikiK3Seams())
}
