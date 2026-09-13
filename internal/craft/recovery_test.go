package craft

import "testing"

// TestUnknownExecutionNeverResubmits is the C04 safety prime directive,
// verbatim from the brief: an execution whose outcome cannot be proven was
// possibly already performed, so the only safe route is a durable wait —
// never an automatic resubmission. A persisted result must never be lost.
func TestUnknownExecutionNeverResubmits(t *testing.T) {
	if RecoveryRoute(RecoveryFacts{VersionCompatible: true, WorkspaceAvailable: true}) != "wait" {
		t.Fatal("unsafe retry")
	}
	if RecoveryRoute(RecoveryFacts{ResultStored: true}) != "reuse" {
		t.Fatal("lost persisted result")
	}
}

// TestRecoveryRouteMatrix pins every route transition: a stored result is
// always reused; an incompatible runtime or unavailable workspace always
// waits before any observation is trusted; only the R04 full-snapshot
// Completed predicate collects; only a still-running remote is observed;
// everything else — including a healthy idle runtime that cannot prove the
// prompt was or was not accepted — waits.
func TestRecoveryRouteMatrix(t *testing.T) {
	healthy := RecoveryFacts{VersionCompatible: true, WorkspaceAvailable: true}
	cases := []struct {
		name string
		f    RecoveryFacts
		want string
	}{
		{"stored result wins over everything", RecoveryFacts{
			ResultStored: true, ExactCompleted: true, RemoteRunning: true,
			VersionCompatible: false, WorkspaceAvailable: false,
		}, "reuse"},
		{"incompatible image never collects", RecoveryFacts{
			ExactCompleted: true, RemoteRunning: true, VersionCompatible: false, WorkspaceAvailable: true,
		}, "wait"},
		{"missing workspace never observes", RecoveryFacts{
			RemoteRunning: true, VersionCompatible: true, WorkspaceAvailable: false,
		}, "wait"},
		{"exact completed collects", RecoveryFacts{
			ExactCompleted: true, VersionCompatible: true, WorkspaceAvailable: true,
		}, "collect"},
		{"still running observes", RecoveryFacts{
			RemoteRunning: true, VersionCompatible: true, WorkspaceAvailable: true,
		}, "observe"},
		{"healthy but unprovable waits", healthy, "wait"},
		{"idle runtime without completion proof waits", RecoveryFacts{
			VersionCompatible: true, WorkspaceAvailable: true, RemoteRunning: false,
		}, "wait"},
		{"all facts unset waits", RecoveryFacts{}, "wait"},
	}
	for _, tc := range cases {
		if got := RecoveryRoute(tc.f); got != tc.want {
			t.Fatalf("case %q: RecoveryRoute = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// TestRecoveryRouteHasNoResubmitRoute proves the route vocabulary carries no
// automatic-resubmission branch at all: unknown writes can only wait.
func TestRecoveryRouteHasNoResubmitRoute(t *testing.T) {
	routes := map[string]bool{RecoveryRouteReuse: true, RecoveryRouteCollect: true,
		RecoveryRouteObserve: true, RecoveryRouteWait: true}
	if len(routes) != 4 {
		t.Fatalf("route vocabulary changed: %#v", routes)
	}
	for _, banned := range []string{"execute", "retry", "resubmit", "redispatch"} {
		if routes[banned] {
			t.Fatalf("route vocabulary must never contain %q", banned)
		}
	}
	// Every possible facts combination must land inside the vocabulary.
	for mask := 0; mask < 32; mask++ {
		f := RecoveryFacts{
			ResultStored:       mask&1 != 0,
			ExactCompleted:     mask&2 != 0,
			RemoteRunning:      mask&4 != 0,
			VersionCompatible:  mask&8 != 0,
			WorkspaceAvailable: mask&16 != 0,
		}
		route := RecoveryRoute(f)
		if !routes[route] {
			t.Fatalf("facts %#v produced unknown route %q", f, route)
		}
	}
}
