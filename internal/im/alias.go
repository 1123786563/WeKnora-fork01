// Pass A compatibility alias for internal/modules/channels/im — zero logic. Deleted by Pass B task B-channels.
package im

import "github.com/Tencent/WeKnora/internal/modules/channels/im"

// Service is an alias kept for internal/container (registerIMService).
type Service = im.Service

// NewService forwards to the moved package.
var NewService = im.NewService
