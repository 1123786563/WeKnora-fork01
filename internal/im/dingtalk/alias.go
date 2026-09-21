// Pass A compatibility alias for internal/modules/channels/im/dingtalk — zero logic. Deleted by Pass B task B-channels.
package dingtalk

import "github.com/Tencent/WeKnora/internal/modules/channels/im/dingtalk"

// NewFactory forwards to the moved package (used by internal/container).
var NewFactory = dingtalk.NewFactory
