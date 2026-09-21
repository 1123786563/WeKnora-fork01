// Pass A compatibility alias for internal/modules/channels/im/qqbot — zero logic. Deleted by Pass B task B-channels.
package qqbot

import "github.com/Tencent/WeKnora/internal/modules/channels/im/qqbot"

// NewFactory forwards to the moved package (used by internal/container).
var NewFactory = qqbot.NewFactory
