// Pass A compatibility alias for internal/modules/channels/im/telegram — zero logic. Deleted by Pass B task B-channels.
package telegram

import "github.com/Tencent/WeKnora/internal/modules/channels/im/telegram"

// NewFactory forwards to the moved package (used by internal/container).
var NewFactory = telegram.NewFactory
