// Pass A compatibility alias for internal/modules/craft — zero logic. Deleted by Pass B task B-craft.
package craft

import "github.com/Tencent/WeKnora/internal/modules/craft"

// Type aliases to the moved package.
type Executor = craft.Executor
type PreviewCheckStore = craft.PreviewCheckStore
type Store = craft.Store
type VersionStore = craft.VersionStore
type Workspace = craft.Workspace

// Constants forwarded to the moved package.
const KindWeb = craft.KindWeb

// Variables and functions forwarded to the moved package.
var KnownKind = craft.KnownKind
