// Pass A compatibility alias for internal/modules/channels/im/slack — zero logic. Deleted by Pass B task B-channels.
package slack

import "github.com/Tencent/WeKnora/internal/modules/channels/im/slack"

// NewFactory forwards to the moved package (used by internal/container).
var NewFactory = slack.NewFactory
