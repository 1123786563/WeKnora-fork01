// Pass A compatibility alias for internal/modules/datasource — zero logic. Deleted by Pass B task B-datasource.
package datasource

import "github.com/Tencent/WeKnora/internal/modules/datasource"

// ConnectorRegistry aliases the moved type.
type ConnectorRegistry = datasource.ConnectorRegistry

// Scheduler aliases the moved type.
type Scheduler = datasource.Scheduler

// NewConnectorRegistry forwards the moved constructor.
var NewConnectorRegistry = datasource.NewConnectorRegistry

// NewScheduler forwards the moved constructor.
var NewScheduler = datasource.NewScheduler
