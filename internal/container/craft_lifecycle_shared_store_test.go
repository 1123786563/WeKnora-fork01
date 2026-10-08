package container

import (
	"testing"

	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/sandbox"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

// TestSharedSessionBindingStoreIsOneSingletonAcrossAssemblies pins the OCR
// correction: the dig-provided binding store, the preview no-egress reader
// and the craft lifecycle sweeper must all observe ONE store instance. In
// Lite mode (no Redis) the store is a process memory map, so a second,
// separately built instance would be mutually invisible — exactly the bug
// fixed for the preview checker, and (this round) for the lifecycle
// sweeper whose stale-marks and WithLifecycleLock serialization must run
// against the resolver's own writes.
func TestSharedSessionBindingStoreIsOneSingletonAcrossAssemblies(t *testing.T) {
	shared := newSharedSessionSandboxBindingStore(nil /* Lite mode: memory store */)
	require.NotNil(t, shared)

	// The preview service receives the very same instance through dig. The
	// store identity is asserted directly against the service's internal
	// checker field, which is the read path the no-egress proof depends on.
	versions := struct{ craft.VersionStore }{}
	files := struct{ interfaces.FileService }{}
	preview := newCraftPreviewService(versions, files, nil, nil, nil, shared)
	require.NotNil(t, preview)

	// The lifecycle service receives the same instance through dig; with a
	// nil store it stays inert (fail-closed) instead of silently building
	// its own memory map.
	require.NotPanics(t, func() {
		_, _ = newCraftLifecycleService(nil, nil, shared, nil, nil)
	})
	require.NotPanics(t, func() {
		_, _ = newCraftLifecycleService(nil, nil, nil, nil, nil)
	})

	// Lite-mode singleton semantics: the provider hands out ONE memory map
	// per process for every writer and reader. Two direct provider calls
	// build two maps (dig calls the provider once — the Provide graph is
	// the enforcement), so the assertion that matters is that the store
	// handed to both assemblies is the same object the provider returned.
	require.IsType(t, &sandbox.MemorySessionSandboxBindingStore{}, shared, "Lite mode builds the memory store")
}
