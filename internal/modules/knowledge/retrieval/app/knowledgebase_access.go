package app

import (
	"context"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/modules/policy/access"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// Service reads accept exact upstream grants. Otherwise cross-tenant reads
// require a user and organization permission resolved for the original caller.
// Pass B K2.4 R1 导出（22-knowledge-retrieval.md §5.1）：原未导出名
// kbReadPermissions/resolveKBReadTenant/requireKBWrite/withKBWriteTenantInfo
// 改为首字母大写导出，宿主 compat 留同名一行委托（ib2 直连后删除）。
func KBReadPermissions(ctx context.Context, shares access.KBShareLookup) *access.KBPermissions {
	if types.CallerFromContext(ctx).UserID == "" {
		shares = nil
	}
	return access.NewKBPermissions(ctx, shares)
}

func ResolveKBReadTenant(ctx context.Context, kb *types.KnowledgeBase, shares access.KBShareLookup) (uint64, error) {
	if kb != nil {
		allowed, err := KBReadPermissions(ctx, shares).Check(kb.ID, kb.TenantID, types.OrgRoleViewer)
		if err == nil && allowed {
			return kb.TenantID, nil
		}
	}
	return 0, apperrors.NewForbiddenError("无权访问该知识库")
}

func RequireKBWrite(ctx context.Context, kb *types.KnowledgeBase) (context.Context, error) {
	if err := access.RequireKBWrite(ctx, kb); err != nil {
		return ctx, apperrors.NewForbiddenError("无权修改该知识库")
	}
	return types.WithExecutionTenant(ctx, kb.TenantID), nil
}

// A shared KB mutation must also resolve models/index backends against its
// owner. The authenticated caller remains unchanged when TenantInfo changes.
func WithKBWriteTenantInfo(
	ctx context.Context,
	kb *types.KnowledgeBase,
	tenants interfaces.TenantRepository,
) (context.Context, error) {
	if tenant, ok := types.TenantInfoFromContext(ctx); ok && tenant != nil && tenant.ID == kb.TenantID {
		return ctx, nil
	}
	if tenants == nil {
		return ctx, apperrors.NewServiceUnavailableError("无法获取知识库所属空间")
	}
	tenant, err := tenants.GetTenantByID(ctx, kb.TenantID)
	if err != nil {
		return ctx, err
	}
	if tenant == nil || tenant.ID != kb.TenantID {
		return ctx, apperrors.NewNotFoundError("知识库所属空间不存在")
	}
	return context.WithValue(ctx, types.TenantInfoContextKey, tenant), nil
}
