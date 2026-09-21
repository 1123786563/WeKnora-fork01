// Pass A compatibility alias: internal/sandbox forwards to
// internal/modules/execution/sandbox. Zero logic. Deleted by Pass B task B-execution.
package sandbox

import "github.com/Tencent/WeKnora/internal/modules/execution/sandbox"

// ConfigureDockerResourceProtection forwards to the moved sandbox package.
var ConfigureDockerResourceProtection = sandbox.ConfigureDockerResourceProtection
