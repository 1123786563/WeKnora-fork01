// Pass A compatibility alias: internal/browserskill forwards to
// internal/modules/execution/browserskill. Zero logic. Deleted by Pass B task B-execution.
package browserskill

import "github.com/Tencent/WeKnora/internal/modules/execution/browserskill"

// Manager is the moved browserskill manager type.
type Manager = browserskill.Manager

// NewManager forwards to the moved browserskill package.
var NewManager = browserskill.NewManager

// NewStore forwards to the moved browserskill package.
var NewStore = browserskill.NewStore
