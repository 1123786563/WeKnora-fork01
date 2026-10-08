package insights

import (
	ifaces "github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/Tencent/WeKnora/internal/modules/airesource/models/chat"
)

// RogueCross 是 fixture 模块代码消费方：引用能力端口，同时导入另一模块
// （airesource）非公开子包（fixture：在册例外内）。
func RogueCross(s ifaces.TenantService) {
	_ = chat.Model{}
}
