// Pass A compatibility alias for internal/modules/channels/im/wecom — zero logic. Deleted by Pass B task B-channels.
package wecom

import "github.com/Tencent/WeKnora/internal/modules/channels/im/wecom"

// NewFactory forwards to the moved package (used by internal/container).
var NewFactory = wecom.NewFactory
