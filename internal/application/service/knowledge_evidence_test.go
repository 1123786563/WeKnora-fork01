package service

// T15（Issue #45）AC2 三个场景的服务端集成证据：多知识源、撤权、无证据。
// 全部跑在真实 sqlite 行 + 真实 KBShareService + 真实事件总线上；只有「检索
// 已返回什么」以夹具 MergeResult 代入（检索引擎是 remote-owned 依赖）。

import (
	"context"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type evidenceCapture struct {
	evidence []types.AnswerEvidence
	fallback []string
	other    []string
}

func newKnowledgeEvidenceEnv(t *testing.T) (*sessionService, *gorm.DB, *event.EventBus, *evidenceCapture) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&types.KnowledgeBase{}, &types.Knowledge{}, &types.Chunk{},
		&types.Organization{}, &types.OrganizationTenantMember{}, &types.KnowledgeBaseShare{},
	))
	shares := NewKBShareService(
		repository.NewKBShareRepository(db),
		repository.NewOrganizationRepository(db),
		repository.NewKnowledgeBaseRepository(db),
		repository.NewKnowledgeRepository(db),
		repository.NewChunkRepository(db),
		nil,
	)
	svc := &sessionService{
		cfg:                  &config.Config{},
		knowledgeBaseService: &knowledgeBaseService{repo: repository.NewKnowledgeBaseRepository(db)},
		kbShareService:       shares,
	}
	bus := event.NewEventBus()
	capture := &evidenceCapture{}
	bus.On(event.EventAgentEvidence, func(_ context.Context, evt event.Event) error {
		data, ok := evt.Data.(event.AgentEvidenceData)
		require.True(t, ok)
		capture.evidence = append(capture.evidence, data.Evidence.(types.AnswerEvidence))
		return nil
	})
	bus.On(event.EventAgentFinalAnswer, func(_ context.Context, evt event.Event) error {
		if data, ok := evt.Data.(event.AgentFinalAnswerData); ok && data.IsFallback {
			capture.fallback = append(capture.fallback, data.Content)
			return nil
		}
		capture.other = append(capture.other, "final_answer")
		return nil
	})
	bus.On(event.EventAgentReferences, func(_ context.Context, evt event.Event) error {
		capture.other = append(capture.other, "references")
		return nil
	})
	return svc, db, bus, capture
}

// caller 是租户 2 的 Viewer：kb-own 同租户；kb-shared 属租户 3、经 org-1 共享给租户 2；
// kb-denied 属租户 3、无共享。
func seedKnowledgeEvidenceRows(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, kb := range []*types.KnowledgeBase{
		{ID: "kb-own", TenantID: 2},
		{ID: "kb-shared", TenantID: 3},
		{ID: "kb-denied", TenantID: 3},
	} {
		require.NoError(t, db.Create(kb).Error)
	}
	for _, knowledge := range []*types.Knowledge{
		{ID: "doc-own", TenantID: 2, KnowledgeBaseID: "kb-own", Type: "file"},
		{ID: "doc-shared", TenantID: 3, KnowledgeBaseID: "kb-shared", Type: "file"},
	} {
		require.NoError(t, db.Create(knowledge).Error)
	}
	require.NoError(t, db.Create(&types.Chunk{ID: "chunk-own", TenantID: 2, KnowledgeBaseID: "kb-own", KnowledgeID: "doc-own", Content: "同租户手册命中段", ContentRevision: 3, StartAt: 10, EndAt: 40}).Error)
	require.NoError(t, db.Create(&types.Chunk{ID: "chunk-shared", TenantID: 3, KnowledgeBaseID: "kb-shared", KnowledgeID: "doc-shared", Content: "跨租户共享指南命中段", ContentRevision: 5, StartAt: 0, EndAt: 30}).Error)
	require.NoError(t, db.Create(&types.Organization{ID: "org-1", Name: "org", OwnerID: "user-3", OwnerTenantID: 3}).Error)
	// 共享创建约束（ShareKnowledgeBase）要求源租户是 org 成员，这里补上，
	// 否则该共享按「源租户已退出」的失效规则被忽略。
	require.NoError(t, db.Create(&types.OrganizationTenantMember{ID: "member-3", OrganizationID: "org-1", TenantID: 3, Role: types.OrgRoleAdmin}).Error)
	require.NoError(t, db.Create(&types.OrganizationTenantMember{ID: "member-2", OrganizationID: "org-1", TenantID: 2, Role: types.OrgRoleViewer}).Error)
	require.NoError(t, db.Create(&types.KnowledgeBaseShare{ID: "share-1", KnowledgeBaseID: "kb-shared", OrganizationID: "org-1", SharedByUserID: "user-3", SourceTenantID: 3, Permission: types.OrgRoleViewer}).Error)
}

func knowledgeEvidenceCallerContext() context.Context {
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(2))
	ctx = context.WithValue(ctx, types.UserIDContextKey, "user-2")
	return context.WithValue(ctx, types.TenantRoleContextKey, types.TenantRoleViewer)
}

func evidenceChatManage(bus *event.EventBus, results []*types.SearchResult) *types.ChatManage {
	return &types.ChatManage{
		PipelineRequest: types.PipelineRequest{Query: "依赖关系是什么", SessionID: "sess-1", ChatModelID: "chat-model-1"},
		PipelineState:   types.PipelineState{MergeResult: results},
		PipelineContext: types.PipelineContext{EventBus: bus.AsEventBusInterface()},
	}
}

func searchRow(chunkID, knowledgeID, kbID, title string, revision int) *types.SearchResult {
	return &types.SearchResult{ID: chunkID, KnowledgeID: knowledgeID, KnowledgeBaseID: kbID, KnowledgeTitle: title, ContentRevision: revision, Content: "命中内容", StartAt: 10, EndAt: 40}
}

// knowledgeRetrievedStamp 模拟「检索管线完成时刻」的盖戳：取一个明显早于测试
// 运行的固定时刻，断言信封精确携带它（而非交付组装时的 time.Now()）。
var knowledgeRetrievedStamp = time.Date(2026, 9, 24, 8, 0, 0, 0, time.UTC)

// 场景 3（多知识源）+ AC1（三类区分）：同租户与跨租户共享两个知识源的引用并存，
// 逐引用携带 revision 与 retrieved_at；结论是模型推断（带模型标识），引用是原文事实。
func TestKnowledgeQAEvidenceMultiKnowledgeSourceAndKinds(t *testing.T) {
	svc, db, bus, capture := newKnowledgeEvidenceEnv(t)
	seedKnowledgeEvidenceRows(t, db)
	cm := evidenceChatManage(bus, []*types.SearchResult{
		searchRow("chunk-own", "doc-own", "kb-own", "手册", 3),
		searchRow("chunk-shared", "doc-shared", "kb-shared", "指南", 5),
	})

	state := svc.deliverKnowledgeEvidence(knowledgeEvidenceCallerContext(), cm, knowledgeRetrievedStamp)
	require.Equal(t, types.EvidenceStateCited, state)
	require.Len(t, capture.evidence, 1)
	envelope := capture.evidence[0]
	require.NoError(t, types.ValidateAnswerEvidence(envelope))
	require.Equal(t, knowledgeRetrievedStamp.Format(time.RFC3339), envelope.RetrievedAt,
		"信封 retrieved_at 必须取检索管线完成时刻（调用方盖戳），而非交付前信封组装时刻")
	kbIDs := []string{envelope.Citations[0].KnowledgeBaseID, envelope.Citations[1].KnowledgeBaseID}
	require.ElementsMatch(t, []string{"kb-own", "kb-shared"}, kbIDs, "两个知识源的引用都必须在场")
	revisions := map[string]int{}
	for _, citation := range envelope.Citations {
		revisions[citation.KnowledgeID] = citation.Revision
		require.Equal(t, types.EvidenceKindFact, citation.Kind)
		_, err := time.Parse(time.RFC3339, citation.RetrievedAt)
		require.NoError(t, err, "每条引用必须带可解析的检索时间")
	}
	require.Equal(t, 3, revisions["doc-own"])
	require.Equal(t, 5, revisions["doc-shared"])
	require.Len(t, envelope.Conclusions, 1)
	require.Equal(t, types.EvidenceKindModelInferred, envelope.Conclusions[0].Kind)
	require.Equal(t, "chat-model-1", envelope.Conclusions[0].ModelID)
	require.False(t, envelope.SemanticGraphUsed, "本地检索必须显式注明未使用语义图谱（ADR-0002）")
	require.Len(t, capture.other, 1, "引用帧恰好一帧")
	require.Equal(t, "references", capture.other[0])
	require.Empty(t, capture.fallback)
}

// 场景 1（撤权）：真实 RemoveShare 翻转权限后，跨租户源在交付前被丢弃；
// 剩余同租户源继续支撑 cited 态。
func TestKnowledgeQAEvidenceRevocationBeforeDeliveryDropsSource(t *testing.T) {
	svc, db, bus, capture := newKnowledgeEvidenceEnv(t)
	seedKnowledgeEvidenceRows(t, db)
	ctx := knowledgeEvidenceCallerContext()

	before := evidenceChatManage(bus, []*types.SearchResult{
		searchRow("chunk-own", "doc-own", "kb-own", "手册", 3),
		searchRow("chunk-shared", "doc-shared", "kb-shared", "指南", 5),
	})
	require.Equal(t, types.EvidenceStateCited, svc.deliverKnowledgeEvidence(ctx, before, knowledgeRetrievedStamp))

	// 真实撤权：share-1 的原始分享者移除共享（真实 service 方法 + 真实行删除）。
	sharerCtx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(3))
	sharerCtx = context.WithValue(sharerCtx, types.UserIDContextKey, "user-3")
	require.NoError(t, svc.kbShareService.RemoveShare(sharerCtx, "share-1", "user-3", 3))

	capture.evidence = nil
	capture.other = nil
	after := evidenceChatManage(bus, []*types.SearchResult{
		searchRow("chunk-own", "doc-own", "kb-own", "手册", 3),
		searchRow("chunk-shared", "doc-shared", "kb-shared", "指南", 5),
		searchRow("chunk-denied", "doc-denied", "kb-denied", "无权文档", 1),
	})
	state := svc.deliverKnowledgeEvidence(ctx, after, knowledgeRetrievedStamp)
	require.Equal(t, types.EvidenceStateCited, state)
	require.Len(t, capture.evidence, 1)
	for _, citation := range capture.evidence[0].Citations {
		require.Equal(t, "kb-own", citation.KnowledgeBaseID, "已撤权与从未获准的源都不得出现在证据里")
	}
	require.Len(t, after.MergeResult, 1, "被撤销/无权的行必须同时从 prompt 上下文移除（不得继续喂养答案）")
	require.Equal(t, "chunk-own", after.MergeResult[0].ID)
}

// 场景 1（撤权·全失权）：唯一证据源被撤销 → 本回答作废（固定文案 fallback），
// 证据信封 state=revoked 且零引用零结论（不得降级为无证据继续作答）。
func TestKnowledgeQAEvidenceAllSourcesRevokedVoidsAnswer(t *testing.T) {
	svc, db, bus, capture := newKnowledgeEvidenceEnv(t)
	seedKnowledgeEvidenceRows(t, db)
	ctx := knowledgeEvidenceCallerContext()

	sharerCtx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(3))
	sharerCtx = context.WithValue(sharerCtx, types.UserIDContextKey, "user-3")
	require.NoError(t, svc.kbShareService.RemoveShare(sharerCtx, "share-1", "user-3", 3))

	cm := evidenceChatManage(bus, []*types.SearchResult{
		searchRow("chunk-shared", "doc-shared", "kb-shared", "指南", 5),
	})
	state := svc.deliverKnowledgeEvidence(ctx, cm, knowledgeRetrievedStamp)
	require.Equal(t, types.EvidenceStateRevoked, state)
	require.Len(t, capture.evidence, 1)
	require.Equal(t, types.EvidenceStateRevoked, capture.evidence[0].State)
	require.Empty(t, capture.evidence[0].Citations)
	require.Empty(t, capture.evidence[0].Conclusions)
	require.NoError(t, types.ValidateAnswerEvidence(capture.evidence[0]))
	require.Len(t, capture.fallback, 1)
	require.Contains(t, capture.fallback[0], "作废", "作废回答必须以固定文案显式收尾（emitFallbackAnswer 可能按引用开关重写标记，断言语义而非整串）")
}

// 场景 1（撤权·不可归属）：KnowledgeBaseID 为空（无法证明归属）的检索行按撤权同形丢弃。
func TestKnowledgeQAEvidenceDropsUnattributableRows(t *testing.T) {
	svc, db, bus, capture := newKnowledgeEvidenceEnv(t)
	seedKnowledgeEvidenceRows(t, db)
	orphan := searchRow("chunk-anon", "doc-anon", "", "无归属", 1)
	kept := searchRow("chunk-own", "doc-own", "kb-own", "手册", 3)
	cm := evidenceChatManage(bus, []*types.SearchResult{orphan, kept})

	state := svc.deliverKnowledgeEvidence(knowledgeEvidenceCallerContext(), cm, knowledgeRetrievedStamp)
	require.Equal(t, types.EvidenceStateCited, state)
	require.Len(t, capture.evidence[0].Citations, 1)
	require.Equal(t, "chunk-own", capture.evidence[0].Citations[0].CitationID)
}

// 场景 2（无证据）：检索为空 → 显式 no_evidence 信封（零引用零结论），随后既有 fallback。
func TestKnowledgeQAEvidenceNoEvidenceIsExplicit(t *testing.T) {
	svc, _, bus, capture := newKnowledgeEvidenceEnv(t)
	cm := evidenceChatManage(bus, nil)
	// 检索为空时管线不会进入 deliverKnowledgeEvidence 的 cited 路径；直接断言
	// no_evidence 信封的组装与发射（KnowledgeQAByEvent 的 ErrSearchNothing 分支调用同一方法）。
	svc.emitKnowledgeEvidenceEvent(knowledgeEvidenceCallerContext(), cm, types.EvidenceStateNoEvidence, types.EvidenceReasoning{}, time.Time{})
	require.Len(t, capture.evidence, 1)
	envelope := capture.evidence[0]
	require.Equal(t, types.EvidenceStateNoEvidence, envelope.State)
	require.Empty(t, envelope.Citations)
	require.Empty(t, envelope.Conclusions)
	require.NoError(t, types.ValidateAnswerEvidence(envelope))
}

// ADR-0002：明确请求推理而部署未接入语义推理服务 → 推理未完成 + 重试入口，
// 零结论零引用，不用普通检索冒充推理成功。
func TestKnowledgeQAReasoningRequestShortCircuitsIncomplete(t *testing.T) {
	svc, _, bus, capture := newKnowledgeEvidenceEnv(t)
	err := svc.KnowledgeQA(knowledgeEvidenceCallerContext(), &types.QARequest{
		Session:       &types.Session{ID: "sess-1"},
		Query:         "A 是否间接依赖 C？",
		ReasoningMode: "rules",
	}, bus)
	require.NoError(t, err)
	require.Len(t, capture.evidence, 1)
	envelope := capture.evidence[0]
	require.Equal(t, types.EvidenceStateNoEvidence, envelope.State)
	require.Empty(t, envelope.Citations, "不得以检索结果冒充推理证据")
	require.Empty(t, envelope.Conclusions, "不得产出任何被标类结论")
	require.True(t, envelope.Reasoning.Requested)
	require.Equal(t, "rules", envelope.Reasoning.Mode)
	require.Equal(t, types.EvidenceReasoningIncomplete, envelope.Reasoning.State)
	require.True(t, envelope.Reasoning.Retryable)
	require.Len(t, capture.fallback, 1)
	require.Contains(t, capture.fallback[0], "推理未完成", "收尾文案必须显式声明推理未完成（emitFallbackAnswer 可能按引用开关重写标记，断言语义而非整串）")
	require.Empty(t, capture.other, "短路后不得再发 references/answer 等业务帧")
}
