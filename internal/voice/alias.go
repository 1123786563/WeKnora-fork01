// Package voice is a Pass A forwarding alias for the moved voice session
// package.
//
// Deleted by Pass B task B-workbench (switch internal/container/container.go:113
// to internal/modules/workbench/voice, then remove this directory).
package voice

import (
	voicemodule "github.com/Tencent/WeKnora/internal/modules/workbench/voice"
)

type (
	// Config forwards the moved provider configuration (composite literals in
	// the forbidden shared file resolve fields through the alias).
	Config = voicemodule.Config
)

// NewManagedProvider forwards the moved constructor (consumed by the forbidden
// shared file internal/container/container.go).
var NewManagedProvider = voicemodule.NewManagedProvider
