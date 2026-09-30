// Package career owns private Career facts and workflow state.
package career

import "github.com/Tencent/WeKnora/internal/modules/career/repository"

// Module is the Career module's application boundary. Process assembly and
// HTTP route registration are owned by the integration layer.
type Module struct{ Repository repository.Store }
