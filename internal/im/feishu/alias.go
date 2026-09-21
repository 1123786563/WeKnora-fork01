// Pass A compatibility alias for internal/modules/channels/im/feishu — zero logic. Deleted by Pass B task B-channels.
package feishu

import "github.com/Tencent/WeKnora/internal/modules/channels/im/feishu"

// NewFactory forwards to the moved package (used by internal/container).
var NewFactory = feishu.NewFactory

// RegionFeishu forwards the moved package value (used by internal/container).
var RegionFeishu = feishu.RegionFeishu

// RegionLark forwards the moved package value (used by internal/container).
var RegionLark = feishu.RegionLark
