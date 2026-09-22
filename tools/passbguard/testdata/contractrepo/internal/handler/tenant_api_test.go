package handler

import (
	ifaces "github.com/Tencent/WeKnora/internal/types/interfaces"
)

// TestTenantServiceCharacterization 是 fixture 特征化测试引用。
func TestTenantServiceCharacterization() {
	var _ ifaces.TenantService = nil
}
