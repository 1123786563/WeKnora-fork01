// Pass A compatibility alias: internal/execution forwards to
// internal/modules/execution. Zero logic. Deleted by Pass B task B-execution.
package execution

import "github.com/Tencent/WeKnora/internal/modules/execution"

// NewRegistrationService forwards to the moved execution package.
var NewRegistrationService = execution.NewRegistrationService
