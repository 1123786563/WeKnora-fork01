// Pass A compatibility alias for internal/modules/policy/ratelimit — zero logic. Deleted by Pass B task B-policy.
package ratelimit

import "github.com/Tencent/WeKnora/internal/modules/policy/ratelimit"

// Type aliases to the moved package.
type Limiter = ratelimit.Limiter

// Constants forwarded to the moved package.

// Variables and functions forwarded to the moved package.
var New = ratelimit.New
