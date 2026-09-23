package handler

import (
	ifaces "github.com/Tencent/WeKnora/internal/types/interfaces"
)

// UseTenant 是 fixture 生产消费方：以限定名引用 interfaces.TenantService。
func UseTenant(s ifaces.TenantService) {}

var _ = ifaces.TenantService(nil)
