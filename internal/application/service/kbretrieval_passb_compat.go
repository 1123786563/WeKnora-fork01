package service

// Pass B 宿主兼容层（b2-k-retrieval / K2.3）：semantic 能力/策略/作用域 3 文件已物理迁移至
// internal/modules/knowledge/retrieval/app（docs/plans/passb/22-knowledge-retrieval.md §5.5 行 2）。
// 本文件为留守宿主消费方（container.go:196-197/:712/:1059-1069、推迟件 semantic_model.go、
// handler/semantic_internal.go、handler/semantic_model_policy.go、宿主 semantic 面测试）提供
// type/var 别名与 semanticScopeGuard 宿主侧同形定义（§5.4 双轨），调用点零改动。
// 删除点：ib2 集成屏障直连后（Integration Brief 指令），随 manifest compat 登记行一并删除。

import (
	"context"

	kbretrieval "github.com/Tencent/WeKnora/internal/modules/knowledge/retrieval/app"
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
