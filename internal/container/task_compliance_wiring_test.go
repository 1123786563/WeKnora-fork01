package container

// T13 (#43) Task 4: wiring-guard. The compliance handler is an OPTIONAL
// RouterParams field — a missing dig Provide leaves it nil and the router
// silently mounts nothing, which no handler-level test can observe (the e2e
// constructs the handler directly). These source-level assertions pin the
// Provide/Invoke registrations into CI (same defense style as an
// architecture guard, scoped to this plan's wiring).

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile(rel)
	require.NoError(t, err)
	return string(data)
}

func TestTaskComplianceWiringRegistered(t *testing.T) {
	containerSrc := readRepoFile(t, "container.go")
	for _, want := range []string{
		"must(container.Provide(NewTaskComplianceStore))",
		"must(container.Provide(NewTaskComplianceService))",
		"must(container.Provide(NewWorkbenchTaskComplianceHandler))",
	} {
		require.True(t, strings.Contains(containerSrc, want),
			"container.go must register %q — without it the optional handler stays nil and the compliance routes silently disappear", want)
	}
	// T13 (#43) Task 5: the legal-hold gate must be Invoke-wired at the
	// session deletion entrances, or the refusal never fires in production.
	require.True(t, strings.Contains(containerSrc, "must(container.Invoke(wireTaskDeletionGuard))"),
		"container.go must Invoke wireTaskDeletionGuard — without it the legal-hold gate is never installed at the deletion entrances")
	workbenchSrc := readRepoFile(t, "workbench.go")
	for _, want := range []string{
		"func NewTaskComplianceStore(",
		"func NewTaskComplianceService(",
		"func NewWorkbenchTaskComplianceHandler(",
	} {
		require.True(t, strings.Contains(workbenchSrc, want),
			"internal/container/workbench.go must define provider %q", want)
	}
}
