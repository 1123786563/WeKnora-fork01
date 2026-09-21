// Package workbench is a Pass A forwarding alias for the moved workbench
// service package.
//
// Deleted by Pass B task B-workbench (switch the remaining forbidden-file
// importers below to internal/modules/workbench/service/workbench, then
// remove this directory).
//
// Remaining consumers (why this alias exists):
//   - internal/container/container.go:39 (forbidden shared file)
//   - internal/modules/agentruntime/agent/engine_test.go:11 (frozen batch-a3
//     module; symbol surface: NewGormInteractionStore,
//     NewInteractionServiceWithApproval)
package workbench

import (
	workbenchmodule "github.com/Tencent/WeKnora/internal/modules/workbench/service/workbench"
)

type (
	// NotificationProvider forwards the moved interface.
	NotificationProvider = workbenchmodule.NotificationProvider
	// NotificationDeliveryWorker forwards the moved worker type.
	NotificationDeliveryWorker = workbenchmodule.NotificationDeliveryWorker
)

var (
	// NewNotificationProjector forwards the moved constructor.
	NewNotificationProjector = workbenchmodule.NewNotificationProjector
	// NewNotificationWorker forwards the moved constructor.
	NewNotificationWorker = workbenchmodule.NewNotificationWorker
	// NewRemoteUsageServiceWithDB forwards the moved constructor.
	NewRemoteUsageServiceWithDB = workbenchmodule.NewRemoteUsageServiceWithDB
	// NewPushNotificationProvider forwards the moved constructor.
	NewPushNotificationProvider = workbenchmodule.NewPushNotificationProvider
	// NewHTTPNotificationProvider forwards the moved constructor.
	NewHTTPNotificationProvider = workbenchmodule.NewHTTPNotificationProvider
	// NewNotificationDeliveryWorkerWithHealth forwards the moved constructor.
	NewNotificationDeliveryWorkerWithHealth = workbenchmodule.NewNotificationDeliveryWorkerWithHealth
	// NewGormInteractionStore forwards the moved constructor (consumed by the
	// frozen agentruntime test).
	NewGormInteractionStore = workbenchmodule.NewGormInteractionStore
	// NewInteractionServiceWithApproval forwards the moved constructor
	// (consumed by the frozen agentruntime test).
	NewInteractionServiceWithApproval = workbenchmodule.NewInteractionServiceWithApproval
)
