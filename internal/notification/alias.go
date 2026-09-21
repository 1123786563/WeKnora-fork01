// Package notification is a Pass A forwarding alias for the moved push
// notification provider package.
//
// Deleted by Pass B task B-workbench (switch internal/container/container.go:106
// to internal/modules/workbench/notification, then remove this directory).
package notification

import (
	notificationmodule "github.com/Tencent/WeKnora/internal/modules/workbench/notification"
)

// NewExpoProvider forwards the moved constructor (consumed by the forbidden
// shared file internal/container/container.go).
var NewExpoProvider = notificationmodule.NewExpoProvider
