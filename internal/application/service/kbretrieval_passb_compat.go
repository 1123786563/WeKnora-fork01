package service

// Pass B 宿主兼容层（b2-k-retrieval / K2.3）：semantic 能力/策略/作用域 3 文件已物理迁移至
// internal/knowledge/retrieval/app（docs/plans/passb/22-knowledge-retrieval.md §5.5 行 2）。
// 本文件为留守宿主消费方（container.go:196-197/:712/:1059-1069、推迟件 semantic_model.go、
// handler/semantic_internal.go、handler/semantic_model_policy.go、宿主 semantic 面测试）提供
// type/var 别名与 semanticScopeGuard 宿主侧同形定义（§5.4 双轨），调用点零改动。
// 删除点：ib2 集成屏障直连后（Integration Brief 指令），随 manifest compat 登记行一并删除。

import (
	"context"

	"github.com/Tencent/WeKnora/internal/application/access"
	kbretrieval "github.com/Tencent/WeKnora/internal/knowledge/retrieval/app"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// --- semanticScopeGuard 宿主侧同形定义（§5.4.2：原定义自 semantic_scope.go 迁出落此，
// 留守嵌入方 tenant.go:57/user.go:100/organization.go:47/tenant_member.go:78/
// knowledgebase.go:34/kbshare.go:38/knowledge.go:50 继续编译；
// 容器装饰器 container.go:344-391 的 SetSemanticScopeInvalidator 断言面不变。
// 落位包 app/semantic_scope_guard.go 为逐字对齐的另一轨；ib2 与 identity 同形 seam
// 一并收口为单一实现（多属主重复 helper 族，conventions §7.1）。---

// semanticScopeGuard preserves lightweight service construction in tools/tests.
// Production container decorators install the mandatory durable invalidator
// before exposing any ACL writer, independently of semantic.enabled.
type semanticScopeGuard struct {
	semanticInvalidator interfaces.SemanticScopeInvalidator
}

func (s *semanticScopeGuard) SetSemanticScopeInvalidator(i interfaces.SemanticScopeInvalidator) {
	s.semanticInvalidator = i
}
func (s *semanticScopeGuard) invalidateSemanticKB(ctx context.Context, tenant uint64, kb string) error {
	if s.semanticInvalidator == nil {
		return nil
	}
	return s.semanticInvalidator.InvalidateKB(ctx, tenant, kb)
}

func (s *semanticScopeGuard) invalidateSemanticTenant(ctx context.Context, tenant uint64) error {
	if s.semanticInvalidator == nil {
		return nil
	}
	return s.semanticInvalidator.InvalidateTenant(ctx, tenant)
}

func (s *semanticScopeGuard) invalidateSemanticUser(ctx context.Context, user string) error {
	if s.semanticInvalidator == nil {
		return nil
	}
	return s.semanticInvalidator.InvalidateUser(ctx, user)
}
func (s *semanticScopeGuard) invalidateSemanticOrganization(ctx context.Context, org string) error {
	if s.semanticInvalidator == nil {
		return nil
	}
	return s.semanticInvalidator.InvalidateOrganization(ctx, org)
}
func (s *semanticScopeGuard) invalidateSemanticTransfer(ctx context.Context, source, target *types.KnowledgeBase) error {
	if s.semanticInvalidator == nil || source.ID == target.ID && source.TenantID == target.TenantID {
		return nil
	}
	return s.semanticInvalidator.InvalidateTransfer(ctx, types.SemanticScopeKey{TenantID: source.TenantID, KBID: source.ID}, types.SemanticScopeKey{TenantID: target.TenantID, KBID: target.ID})
}

// --- type 别名（container.go:197/:1062-1063、semantic_model.go:45/:50 推迟件、
// handler/semantic_internal.go:16-18、handler/semantic_model_policy.go:16/:36/:71-80、
// semantic_model_test.go:401-405、semantic_scope*_test.go 留守测试引用面）---

type SemanticScopeResolver = kbretrieval.SemanticScopeResolver

type SemanticScopeSnapshot = kbretrieval.SemanticScopeSnapshot

type SemanticScopeService = kbretrieval.SemanticScopeService

type SemanticModelPolicyService = kbretrieval.SemanticModelPolicyService

type SemanticModelPolicyScope = kbretrieval.SemanticModelPolicyScope

type SemanticModelPolicyInput = kbretrieval.SemanticModelPolicyInput

// --- var 别名（container.go:196/:712/:1068 构造面）---

var NewSemanticScopeService = kbretrieval.NewSemanticScopeService

var NewUnavailableSemanticModelPolicyService = kbretrieval.NewUnavailableSemanticModelPolicyService

var NewSemanticModelCapabilityIssuer = kbretrieval.NewSemanticModelCapabilityIssuer

// --- 哨兵别名（handler/semantic_internal.go:53、handler/semantic_model_policy.go:84-86、
// 留宿主 semantic_scope_test.go 断言面）---

var ErrSemanticScopeInvalid = kbretrieval.ErrSemanticScopeInvalid

var ErrSemanticScopeExpired = kbretrieval.ErrSemanticScopeExpired

var ErrSemanticScopeChanged = kbretrieval.ErrSemanticScopeChanged

var ErrSemanticScopeUnavailable = kbretrieval.ErrSemanticScopeUnavailable

var ErrSemanticModelPolicyDisabled = kbretrieval.ErrSemanticModelPolicyDisabled

var ErrSemanticModelPricingUnavailable = kbretrieval.ErrSemanticModelPricingUnavailable

// --- K2.4 追加（22-knowledge-retrieval.md §5.1 读权限族 + §5.2 方法再归置）：
// knowledgebase_access.go / slug_fuzzy.go / graph.go 已物理迁移至
// internal/knowledge/retrieval/app。以下一行委托保持留守宿主调用方
// （knowledgebase_search_shared.go:45/83、tag.go:85、knowledge.go:830、
// knowledge_faq.go:36、knowledge_create/delete_plan/write.go、
// wiki_ingest.go:1670、wiki_page.go:1191 等）零改动编译；
// ib2 集成屏障直连后随本文件一并删除。---

func kbReadPermissions(ctx context.Context, shares access.KBShareLookup) *access.KBPermissions {
	return KBReadPermissions(ctx, shares)
}

// resolveKBReadTenant 真源已随 knowledgebase_access.go 归位本包
//（本文件历史转发副本已删除，宿主调用面为同名符号）。

func requireKBWrite(ctx context.Context, kb *types.KnowledgeBase) (context.Context, error) {
	return RequireKBWrite(ctx, kb)
}

func withKBWriteTenantInfo(
	ctx context.Context,
	kb *types.KnowledgeBase,
	tenants interfaces.TenantRepository,
) (context.Context, error) {
	return WithKBWriteTenantInfo(ctx, kb, tenants)
}

// resolveDeadSlug 真源已随 slug_fuzzy.go 归位本包（本文件历史转发副本已删除）。

// --- K2.5 追加（22-knowledge-retrieval.md §5.1 活动族 + §7.3 KB 活动审计流差分锚点）：
// kb_activity.go 已物理迁移至 internal/knowledge/retrieval/app。以下一行委托
// 保持留守宿主调用方（datasource_service.go 17+4 处、knowledge_faq*.go、
// knowledge_create/clone_move/delete/process/replace.go、knowledgebase.go、kbshare.go、
// tag.go、wiki_ingest_batch.go、handler/wiki_page.go:401、kb_activity_test.go 等）
// 零改动编译；ib2 集成屏障直连后随本文件一并删除。---

func withKBActivityTask(ctx context.Context, taskID, trigger string) context.Context {
	return WithKBActivityTask(ctx, taskID, trigger)
}

func kbActivityTrigger(ctx context.Context) string {
	return KBActivityTrigger(ctx)
}

func kbActivityAppendSampleTitles(details map[string]any, titles ...string) {
	KBActivityAppendSampleTitles(details, titles...)
}

func withKBActivitySuppressed(ctx context.Context) context.Context {
	return WithKBActivitySuppressed(ctx)
}

func recordKBActivity(
	ctx context.Context,
	audit interfaces.AuditLogService,
	tenantID uint64,
	kbID string,
	action types.AuditAction,
	targetType string,
	targetID string,
	outcome types.AuditOutcome,
	details map[string]any,
) {
	RecordKBActivity(ctx, audit, tenantID, kbID, action, targetType, targetID, outcome, details)
}

// RecordWikiContentActivity 真源已随 kb_activity.go 归位本包（本文件历史
// var 别名已删除）。

// auditScopeKnowledgeBase 原未导出常量，宿主特征化测试 kb_activity_test.go:130
// 锚定 ScopeType；经落位包导出 AuditScopeKnowledgeBase 别名保单一事实源。
const auditScopeKnowledgeBase = AuditScopeKnowledgeBase

// writableFAQKnowledgeBase 方法再归置（§5.2 行 3 / Ruling 2026-09-25-DEFERRED-FILE-SPLIT）：
// 原定义于 knowledgebase_access.go:38-51，接收者类型 knowledgeService 定义于
// knowledge.go（K4 留守），方法文本随 K2.4 迁出后落此，方法体一行不改。
// K4 调用点（knowledge_faq.go / knowledge_faq_import.go）零改动。
func (s *knowledgeService) writableFAQKnowledgeBase(
	ctx context.Context,
	kbID string,
) (*types.KnowledgeBase, context.Context, error) {
	kb, err := s.validateFAQKnowledgeBase(ctx, kbID)
	if err != nil {
		return nil, ctx, err
	}
	ctx, err = requireKBWrite(ctx, kb)
	if err == nil {
		ctx, err = withKBWriteTenantInfo(ctx, kb, s.tenantRepo)
	}
	return kb, ctx, err
}
