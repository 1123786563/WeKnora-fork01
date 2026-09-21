// Pass A compatibility alias for internal/modules/channels/im/mattermost — zero logic. Deleted by Pass B task B-channels.
package mattermost

import "github.com/Tencent/WeKnora/internal/modules/channels/im/mattermost"

// NewFactory forwards to the moved package (used by internal/container).
var NewFactory = mattermost.NewFactory
