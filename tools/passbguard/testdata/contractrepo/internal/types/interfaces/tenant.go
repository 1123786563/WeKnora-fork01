package interfaces

import "context"

// TenantService 是 fixture 形态的 Tenant 能力端口（与真实仓库同名接口无关）。
type TenantService interface {
	CreateTenant(ctx context.Context, name string) error
	GetTenantByID(ctx context.Context, id uint64) (string, error)
}
