package service

// T16 (#134) run-admission wiring: after a Run Submit commits, StartRun
// acquires the workspace writer lease and projects the T00 frozen
// acquisition outcome (acquired/conflict/unknown). The lease is the second
// serialization layer on top of admission — an outcome of conflict (another
// writing Run holds the workspace) or an indeterminate acquisition can
// never fail or un-admit the committed run.

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/stretchr/testify/require"
)

// t16LeaseCall records exactly what the admission wiring asked of the
// durable lease store.
type t16LeaseCall struct{ workspaceID, runID string }

// t16LeaseProbe fakes the lease store: every acquisition answers the canned
// status echoing the requested workspace identity.
type t16LeaseProbe struct {
	status craft.WriterAcquireStatus
	err    error
	calls  []t16LeaseCall
}

func (p *t16LeaseProbe) AcquireWriterLease(_ context.Context, _ craft.Scope, workspaceID, runID string) (craft.WriterAcquisition, error) {
	p.calls = append(p.calls, t16LeaseCall{workspaceID: workspaceID, runID: runID})
	if p.err != nil {
		return craft.WriterAcquisition{}, p.err
	}
	return craft.WriterAcquisition{Outcome: craft.WriterAcquireOutcome{WorkspaceID: workspaceID, Status: p.status}}, nil
}

func (p *t16LeaseProbe) ReleaseWriterLease(context.Context, craft.Scope, string, string, string) error {
	return nil
}

func (p *t16LeaseProbe) GetWriterLease(context.Context, craft.Scope, string) (*craft.WriterLease, error) {
	return nil, nil
}

// newT16WiredEnv builds the session env plus a second service instance that
// carries the T16 lease seam, mirroring the central assembly's wiring.
func newT16WiredEnv(t *testing.T, leases CraftWriterLeaseStore) (*craftSessionEnv, *CraftSessionService) {
	t.Helper()
	env := newCraftSessionEnv(t, openGate)
	svc, err := NewCraftSessionService(CraftSessionConfig{
		DB: env.db, Sessions: env.sessions, Store: env.store, Versions: env.versions,
		Runs: env.runs, ActiveRuns: CraftActiveRunsQuery(env.db),
		TemporaryDocs: env.docs, Files: &fakeCraftFiles{blobs: map[string][]byte{}}, Models: env.models, Gate: openGate,
		WriterLeases: leases,
	})
	require.NoError(t, err)
	return env, svc
}

// TestCraftT16RunAdmissionAcquiresAndProjectsWriterLease pins the wiring:
// a committed Submit acquires the lease exactly once for (workspace, run)
// and the T00 frozen outcome projects straight out of StartRun.
func TestCraftT16RunAdmissionAcquiresAndProjectsWriterLease(t *testing.T) {
	probe := &t16LeaseProbe{status: craft.WriterAcquired}
	env, svc := newT16WiredEnv(t, probe)
	ws := createCraftSession(t, env, "u1", "k-1", "站点", "web")
	ctx := craftCtx(1, "u1", ws.SessionID)

	run, acquisition, err := svc.StartRun(ctx, ownerScope(1, "u1", ws.SessionID), CraftRunRequest{
		RequestID: "r-1", Prompt: "改一下首页",
	})
	require.NoError(t, err)
	require.NotEmpty(t, run.Key.RunID)
	require.Equal(t, craft.WriterAcquired, acquisition.Outcome.Status)
	require.Equal(t, ws.ID, acquisition.Outcome.WorkspaceID)
	require.Len(t, probe.calls, 1, "one acquisition per admitted run")
	require.Equal(t, ws.ID, probe.calls[0].workspaceID)
	require.Equal(t, run.Key.RunID, probe.calls[0].runID, "the lease binds the admitted run")
}

// TestCraftT16LeaseConflictAndIndeterminateNeverUnadmit pin the degradation
// contract: a conflict outcome (another writing Run holds the workspace)
// and an indeterminate acquisition both keep the committed run — admission
// stays the first serialization layer and the fence keeps whatever row
// exists while the caller sees conflict/unknown.
func TestCraftT16LeaseConflictAndIndeterminateNeverUnadmit(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status craft.WriterAcquireStatus
		fails  bool
	}{
		{name: "conflict", status: craft.WriterConflict},
		{name: "indeterminate store error", status: craft.WriterAcquired, fails: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			probe := &t16LeaseProbe{status: tc.status, err: nil}
			if tc.fails {
				probe.err = context.DeadlineExceeded
			}
			env, svc := newT16WiredEnv(t, probe)
			ws := createCraftSession(t, env, "u1", "k-"+tc.name, "站点", "web")
			ctx := craftCtx(1, "u1", ws.SessionID)

			run, acquisition, err := svc.StartRun(ctx, ownerScope(1, "u1", ws.SessionID), CraftRunRequest{
				RequestID: "r-1", Prompt: "改一下首页",
			})
			require.NoError(t, err, "the lease outcome never fails or un-admits the committed run")
			require.NotEmpty(t, run.Key.RunID)
			require.Len(t, probe.calls, 1)
			if tc.fails {
				require.Equal(t, craft.WriterUnknown, acquisition.Outcome.Status, "an indeterminate acquisition reports unknown")
			} else {
				require.Equal(t, craft.WriterConflict, acquisition.Outcome.Status)
			}
			require.Equal(t, ws.ID, acquisition.Outcome.WorkspaceID)
		})
	}
}

// TestCraftT16UnwiredLeaseSeamKeepsPreT16Behavior pins the nil-seam
// degradation: without a lease store, StartRun keeps the pre-T16 behavior
// and reports the honest unknown outcome for the workspace.
func TestCraftT16UnwiredLeaseSeamKeepsPreT16Behavior(t *testing.T) {
	env, svc := newT16WiredEnv(t, nil)
	ws := createCraftSession(t, env, "u1", "k-unwired", "站点", "web")
	ctx := craftCtx(1, "u1", ws.SessionID)

	run, acquisition, err := svc.StartRun(ctx, ownerScope(1, "u1", ws.SessionID), CraftRunRequest{
		RequestID: "r-1", Prompt: "改一下首页",
	})
	require.NoError(t, err)
	require.NotEmpty(t, run.Key.RunID)
	require.Equal(t, craft.WriterUnknown, acquisition.Outcome.Status, "an unwired seam reports unknown, never a fake acquired")
	require.Equal(t, ws.ID, acquisition.Outcome.WorkspaceID)
}
