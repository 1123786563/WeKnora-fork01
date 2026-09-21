// Pass A compatibility alias for internal/modules/channels/im/wechat — zero logic. Deleted by Pass B task B-channels.
package wechat

import "github.com/Tencent/WeKnora/internal/modules/channels/im/wechat"

// NewFactory forwards to the moved package (used by internal/container).
var NewFactory = wechat.NewFactory
