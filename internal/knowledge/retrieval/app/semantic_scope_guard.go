package app

import (
	"context"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// Pass B 过渡 seam（R2/§5.4 双轨，b2-k-retrieval / K2.3）：semanticScopeGuard 定义
// 逐字复制自 internal/application/service/semantic_scope.go:28-69（原文本已随 K2.3 迁出
// 至宿主 compat internal/application/service/kbretrieval_passb_compat.go，留守嵌入方
// tenant/user/organization/tenant_member/knowledgebase/kbshare/knowledge.go 经宿主侧同形
// 定义继续编译）。多属主重复 helper 族，ib2 与 identity 导出的同形 seam 一并收口为单一定义
// （docs/plans/passb/22-knowledge-retrieval.md §5.4；conventions §7.1）。

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
