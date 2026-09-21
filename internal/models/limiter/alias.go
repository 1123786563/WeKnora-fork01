// Pass A compatibility alias for github.com/Tencent/WeKnora/internal/modules/airesource/models/limiter — zero logic. Deleted by Pass B task B-airesource.
package limiter

import "github.com/Tencent/WeKnora/internal/modules/airesource/models/limiter"

// Type aliases to the moved package.
type ModelConcurrencyLimiter = limiter.ModelConcurrencyLimiter
type RuntimeStat = limiter.RuntimeStat

// Functions forwarded to the moved package (function values; zero logic).
var Gate = limiter.Gate
var GateN = limiter.GateN
var GateNamedN = limiter.GateNamedN
var NewLocalLimiter = limiter.NewLocalLimiter
var NewRedisLimiter = limiter.NewRedisLimiter
var RuntimeStats = limiter.RuntimeStats
var SetGlobalLimit = limiter.SetGlobalLimit
var SetGovernor = limiter.SetGovernor
