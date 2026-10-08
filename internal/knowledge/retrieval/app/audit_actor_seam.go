package app

import (
	"context"

	"github.com/Tencent/WeKnora/internal/types"
)

// Pass B 过渡 seam（R2）：auditActor / auditActorRole 原定义于宿主
// internal/application/service/tenant_member.go:156-165（identity 属主 b1 推迟件，
// K0 冻结 20-knowledge-program.md §6.2 组 E 裁定「K2 按 R2 建 seam，
// K5/ib2 接 identity 导出」）。本文件体=对冻结公共 API 的同一委托
// （types.UserIDFromContext / types.TenantRoleFromContext），与 tenant_member.go
// 逐字等价；kb_activity.go 调用点零改动。属「多属主重复 helper 族」，
// ib2 与 identity 导出的 AuditActor/AuditActorRole 收口为单一实现
// （conventions §7.1）。
func auditActorRole(ctx context.Context) string {
	return string(types.TenantRoleFromContext(ctx))
}

func auditActor(ctx context.Context) string {
	uid, _ := types.UserIDFromContext(ctx)
	return uid
}
