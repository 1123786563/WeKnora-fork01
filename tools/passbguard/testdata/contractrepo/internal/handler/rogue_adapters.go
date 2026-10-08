package handler

import (
	"github.com/Tencent/WeKnora/internal/modules/identity/adapters/thing"
)

// RogueAdapters 导入 identity 模块的 /adapters 非公开子包（fixture：未在册）。
// 注意：本文件不引用 interfaces.TenantService，仅用于 forbidden-import 用例。
var _ = thing.Adapter{}
