package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/Tencent/WeKnora/internal/modules/execution/sandbox"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// craftPolicySeed manifests one uploaded python input inside the durable
// snapshot of an already-seeded run row, so the execution-policy gate can
// re-load it by identity.
func craftPolicySeed(t *testing.T, db *gorm.DB, tenant uint64, runID string) craft.Input {
	t.Helper()
	content := []byte("print('data')\n")
	sum := sha256.Sum256(content)
	digest := hex.EncodeToString(sum[:])
	uploaded := craft.Input{
		Ref: "resource://policy-input-1", Name: "analyze.py",
		SHA256: digest, Bytes: int64(len(content)), CitationID: digest,
	}
	snapshot, err := json.Marshal(map[string]any{
		"version": 1, "query": "build", "model_id": "model-1",
		"agent_config":         json.RawMessage(`{}`),
		"craft_input_manifest": []craft.Input{uploaded},
		"craft_workspace_seed": CraftWorkspaceSeedSnapshot{WorkspaceID: "ws-policy-1", State: craft.DraftHeadEmpty},
	})
	require.NoError(t, err)
	require.NoError(t, db.Exec("UPDATE agent_runs SET snapshot = ? WHERE tenant_id = ? AND run_id = ?",
		string(snapshot), tenant, runID).Error)
	return uploaded
}

func craftPolicyGate(t *testing.T, db *gorm.DB) *CraftDelegateExecutionPolicy {
	t.Helper()
	delegate := NewCraftDelegateService(repository.NewCraftStore(db), nil)
	gate, err := NewCraftDelegateExecutionPolicy(delegate, db, "/workspace")
	require.NoError(t, err)
	return gate
}

// TestCraftExecutionPolicyGateDeniesUploadedInputExecution is the T03
// central wiring regression: the adapter re-loads the admitted manifest by
// durable identity and denies executing uploaded code with the
// member-visible refusal, while generated Workspace code passes and an
// unknown run fails closed.
func TestCraftExecutionPolicyGateDeniesUploadedInputExecution(t *testing.T) {
	const tenant = uint64(9301)
	db := openCraftBudgetTestDB(t)
	seedCraftFundedTenant(t, db, tenant, 10000)
	seedCraftBudgetRun(t, db, tenant, "run-policy-gate", "sess-policy-gate")
	craftPolicySeed(t, db, tenant, "run-policy-gate")
	gate := craftPolicyGate(t, db)

	err := gate.ReviewNormalExec(context.Background(), repository.CraftDockerNormalInputRequest{
		TenantID: tenant, RunID: "run-policy-gate", WorkingDir: "/workspace",
		Command: []string{"python3", "/workspace/inputs/analyze.py"},
	})
	require.ErrorIs(t, err, craft.ErrForbidden)
	require.Contains(t, err.Error(), "Allowed alternative", "the refusal must be member-visible")

	// A generated script inside the writable workspace output passes.
	require.NoError(t, gate.ReviewNormalExec(context.Background(), repository.CraftDockerNormalInputRequest{
		TenantID: tenant, RunID: "run-policy-gate", WorkingDir: "/workspace",
		Command: []string{"/workspace/rv-abc/output/build.sh"},
	}))

	// Unknown run identity fails closed.
	require.Error(t, gate.ReviewNormalExec(context.Background(), repository.CraftDockerNormalInputRequest{
		TenantID: tenant, RunID: "run-never-admitted", WorkingDir: "/workspace",
		Command: []string{"/bin/true"},
	}))
}

// TestCraftNormalExecServiceEnforcesPolicyGateBeforeSend proves the wired
// gate runs before any create, bind, claim or send: a denied review returns
// the refusal and the provider observes nothing.
func TestCraftNormalExecServiceEnforcesPolicyGateBeforeSend(t *testing.T) {
	t.Setenv("SYSTEM_AES_KEY", "01234567890123456789012345678901")
	db := openCraftBudgetTestDB(t)
	const tenant = uint64(9302)
	// The seeded run's session must equal the staged request's TaskID
	// binding: the gate mirrors the coordinator's consistency check.
	coordinator, _, grantID := seedCraftDockerNormalCoordinator(t, db, tenant, "run-gated-exec", "task-gated-exec")
	request := craftDockerNormalCoordinatorRequest(tenant, "task-gated-exec", "run-gated-exec", "activity-gated-exec")
	craftPolicySeed(t, db, tenant, request.RunID)

	provider := &normalExecTestProvider{}
	outputRepo := repository.NewCraftDockerOutputRepository(db, 4096)
	output, err := NewCraftDockerOutputService(outputRepo)
	require.NoError(t, err)
	inputs := repository.NewCraftDockerNormalInputRepository(db)
	svc, err := NewCraftDockerNormalExecService(coordinator, inputs, provider, output)
	require.NoError(t, err)
	svc.WithExecutionPolicy(craftPolicyGate(t, db))

	request.Command = []string{"python3", "/workspace/inputs/analyze.py"}
	_, err = svc.Execute(context.Background(), grantID, request.ActivityKey,
		CraftCallBinding{ModelID: "model-gated", Funding: "platform"}, normalExecTestHandle{id: "container-gated"}, request)
	require.ErrorIs(t, err, craft.ErrForbidden)
	require.Contains(t, err.Error(), "Allowed alternative")
	require.Zero(t, provider.created, "a denied review must never reach the engine")
	require.Zero(t, provider.started)
}

// TestCraftRestrictedExecServiceEnforcesPolicyGateBeforeSend proves the
// restricted face is screened first too: without a durable Run identity the
// adapter refuses and the durable send hold is never written.
func TestCraftRestrictedExecServiceEnforcesPolicyGateBeforeSend(t *testing.T) {
	coordinator, budget, _, tenant, grantID := newCraftDockerCoordinatorFixture(t)
	svc, err := NewCraftDockerRestrictedExec(coordinator, &sandbox.DockerRemoteClient{}, time.Second)
	require.NoError(t, err)
	svc.WithExecutionPolicy(craftPolicyGate(t, budget.db))

	_, err = svc.Start(context.Background(), grantID, "activity-gated-restricted",
		CraftCallBinding{}, fakeDockerHandle{},
		CraftDockerOutputlessRequest{DiscardOutput: true, Exec: sandbox.RemoteExecRequest{Command: "true"}})
	require.ErrorIs(t, err, craft.ErrForbidden)
	require.Contains(t, err.Error(), "no durable Run identity", "unreviewable restricted exec must fail closed")
	var rows int64
	require.NoError(t, budget.db.Table("craft_charge_start_journal").
		Where("tenant_id = ? AND activity_key = ?", tenant, "activity-gated-restricted").Count(&rows).Error)
	require.Zero(t, rows, "the denied review must precede the durable send hold")
}

// recordingAuditSink captures audit rows for the material-policy events.
type recordingAuditSink struct {
	interfaces.AuditLogService
	entries []*types.AuditLog
}

func (s *recordingAuditSink) Log(_ context.Context, entry *types.AuditLog) error {
	s.entries = append(s.entries, entry)
	return nil
}

// TestCraftMaterialPolicyPersistsAuditRows is the item-3 wiring regression:
// with an injected AuditLogService the material-policy events persist to
// audit_logs (best-effort, never changing a decision).
func TestCraftMaterialPolicyPersistsAuditRows(t *testing.T) {
	const tenant = uint64(9303)
	db := openCraftBudgetTestDB(t)
	seedCraftFundedTenant(t, db, tenant, 10000)
	seedCraftBudgetRun(t, db, tenant, "run-policy-audit", "sess-policy-audit")
	uploaded := craftPolicySeed(t, db, tenant, "run-policy-audit")

	audit := &recordingAuditSink{}
	delegate := NewCraftDelegateService(repository.NewCraftStore(db), nil).WithAuditLog(audit)
	policy, err := delegate.MaterialPolicy("/workspace", craft.Task{
		Scope:       craft.Scope{TenantID: tenant, UserID: "owner", SessionID: "sess-policy-audit"},
		WorkspaceID: "ws-policy-1",
		Fence:       agentruntime.Fence{RunKey: agentruntime.RunKey{TenantID: tenant, RunID: "run-policy-audit"}},
	})
	require.NoError(t, err)

	event := policy.AuditInputRead(context.Background(), uploaded)
	require.Equal(t, craft.AuditKindInputRead, event.Kind)
	decision := policy.ReviewExecution(context.Background(), craft.InputExecutionRequest{
		Command: []string{"python3", "/workspace/inputs/analyze.py"}, WorkingDir: "/workspace",
	})
	require.False(t, decision.Allowed)

	require.Len(t, audit.entries, 2, "read + denied execution each persist one audit row")
	require.Equal(t, types.AuditAction(craft.AuditKindInputRead), audit.entries[0].Action)
	require.Equal(t, types.AuditOutcomeSuccess, audit.entries[0].Outcome)
	require.Equal(t, types.AuditAction(craft.AuditKindInputExecuteDenied), audit.entries[1].Action)
	require.Equal(t, types.AuditOutcomeDenied, audit.entries[1].Outcome)
	require.Equal(t, "craft_run", audit.entries[1].ScopeType)
	require.Equal(t, "run-policy-audit", audit.entries[1].ScopeID)
}

// gateWithAuditedRun seeds one craft run and returns a gate wired to a
// recording audit sink alongside the uploaded input.
func gateWithAuditedRun(t *testing.T, tenant uint64, runID, sessionID string) (*CraftDelegateExecutionPolicy, *recordingAuditSink, craft.Input) {
	t.Helper()
	db := openCraftBudgetTestDB(t)
	seedCraftFundedTenant(t, db, tenant, 10000)
	seedCraftBudgetRun(t, db, tenant, runID, sessionID)
	uploaded := craftPolicySeed(t, db, tenant, runID)
	delegate := NewCraftDelegateService(repository.NewCraftStore(db), nil)
	gate, err := NewCraftDelegateExecutionPolicy(delegate, db, "/workspace")
	require.NoError(t, err)
	audit := &recordingAuditSink{}
	gate.WithAuditLog(audit)
	return gate, audit, uploaded
}

// TestCraftExecutionPolicyGateDeniesShellWrappedCommand is the OCR
// high-finding regression: `bash -c "python3 <uploaded>"` in argv form is
// recognized as a shell expression and denied fail-closed.
func TestCraftExecutionPolicyGateDeniesShellWrappedCommand(t *testing.T) {
	const tenant = uint64(9304)
	gate, _, _ := gateWithAuditedRun(t, tenant, "run-policy-shell", "sess-policy-shell")
	err := gate.ReviewNormalExec(context.Background(), repository.CraftDockerNormalInputRequest{
		TenantID: tenant, RunID: "run-policy-shell", TaskID: "sess-policy-shell",
		WorkingDir: "/workspace",
		Command:    []string{"bash", "-c", "python3 /workspace/inputs/analyze.py"},
	})
	require.ErrorIs(t, err, craft.ErrForbidden)
}

// TestCraftExecutionPolicyGateDeniesUploadedStdinBytes is the OCR
// high-finding regression: stdin bytes equal to an admitted input are
// matched by digest, and interpreter stdin-program forms are refused.
func TestCraftExecutionPolicyGateDeniesUploadedStdinBytes(t *testing.T) {
	const tenant = uint64(9305)
	gate, _, uploaded := gateWithAuditedRun(t, tenant, "run-policy-stdin", "sess-policy-stdin")

	// Uploaded bytes on stdin of an interpreter stdin-program form.
	err := gate.ReviewNormalExec(context.Background(), repository.CraftDockerNormalInputRequest{
		TenantID: tenant, RunID: "run-policy-stdin", TaskID: "sess-policy-stdin",
		WorkingDir: "/workspace", Command: []string{"python3"},
		StdinEnabled: true, Stdin: []byte("print('data')\n"),
	})
	require.ErrorIs(t, err, craft.ErrForbidden)

	// The explicit stdin marker form is refused regardless of the bytes.
	err = gate.ReviewNormalExec(context.Background(), repository.CraftDockerNormalInputRequest{
		TenantID: tenant, RunID: "run-policy-stdin", TaskID: "sess-policy-stdin",
		WorkingDir: "/workspace", Command: []string{"python3", "-"},
		StdinEnabled: true, Stdin: []byte("anything"),
	})
	require.ErrorIs(t, err, craft.ErrForbidden)
	require.NotEqual(t, uploaded.SHA256, "", "fixture sanity")
}

// TestCraftExecutionPolicyGateDeniesEnvironmentHook is the OCR medium
// finding regression: a startup-hook environment value pointing at the
// read-only tree is denied.
func TestCraftExecutionPolicyGateDeniesEnvironmentHook(t *testing.T) {
	const tenant = uint64(9306)
	gate, _, _ := gateWithAuditedRun(t, tenant, "run-policy-env", "sess-policy-env")
	err := gate.ReviewNormalExec(context.Background(), repository.CraftDockerNormalInputRequest{
		TenantID: tenant, RunID: "run-policy-env", TaskID: "sess-policy-env",
		WorkingDir: "/workspace", Command: []string{"bash"},
		Environment: map[string]string{"BASH_ENV": "/workspace/inputs/analyze.py"},
	})
	require.ErrorIs(t, err, craft.ErrForbidden)
}

// TestCraftExecutionPolicyGateCrossChecksTaskBinding is the OCR medium
// finding regression: the request's task binding must agree with the run's
// session, so a request cannot review against another run of the tenant.
func TestCraftExecutionPolicyGateCrossChecksTaskBinding(t *testing.T) {
	const tenant = uint64(9307)
	gate, audit, _ := gateWithAuditedRun(t, tenant, "run-policy-bind", "sess-policy-bind")
	err := gate.ReviewNormalExec(context.Background(), repository.CraftDockerNormalInputRequest{
		TenantID: tenant, RunID: "run-policy-bind", TaskID: "some-other-session",
		WorkingDir: "/workspace", Command: []string{"/bin/true"},
	})
	require.ErrorIs(t, err, craft.ErrConflict)
	require.NotEmpty(t, audit.entries, "the fail-closed refusal must persist a denied audit row")
	require.Equal(t, types.AuditOutcomeDenied, audit.entries[len(audit.entries)-1].Outcome)
}

// TestCraftExecutionPolicyGateAuditsRestrictedRefusal is the OCR low-finding
// regression: the restricted face's constant denial carries no durable
// identity, so it must NOT write an unattributable tenant-0 audit row —
// the log line carries the event instead.
func TestCraftExecutionPolicyGateAuditsRestrictedRefusal(t *testing.T) {
	const tenant = uint64(9308)
	gate, audit, _ := gateWithAuditedRun(t, tenant, "run-policy-restricted", "sess-policy-restricted")
	err := gate.ReviewOutputlessExec(context.Background(), CraftCallBinding{}, CraftDockerOutputlessRequest{
		DiscardOutput: true, Exec: sandbox.RemoteExecRequest{Command: "true"},
	})
	require.ErrorIs(t, err, craft.ErrForbidden)
	require.Empty(t, audit.entries, "identity-free denials must never write orphan audit rows")
}

// TestCraftExecutionPolicyGateRound4Forms covers the round-4 adapter
// posture: combined shell flags fail closed via the module's program-text
// rule, and the script's own later flags stay reviewable data.
func TestCraftExecutionPolicyGateRound4Forms(t *testing.T) {
	const tenant = uint64(9309)
	gate, _, _ := gateWithAuditedRun(t, tenant, "run-policy-r4", "sess-policy-r4")

	// bash -lc "..." (combined short group) is denied by the module's
	// option-region program-text rule — no weaker lexical path exists.
	err := gate.ReviewNormalExec(context.Background(), repository.CraftDockerNormalInputRequest{
		TenantID: tenant, RunID: "run-policy-r4", TaskID: "sess-policy-r4",
		WorkingDir: "/workspace",
		Command:    []string{"bash", "-lc", "eval $(echo eYWFsZSBpbg== | base64 -d)"},
	})
	require.ErrorIs(t, err, craft.ErrForbidden)

	// The script's own -c argument is data, not an interpreter flag: the
	// recommended "execute a generated file" alternative stays usable.
	require.NoError(t, gate.ReviewNormalExec(context.Background(), repository.CraftDockerNormalInputRequest{
		TenantID: tenant, RunID: "run-policy-r4", TaskID: "sess-policy-r4",
		WorkingDir: "/workspace",
		Command:    []string{"python3", "/workspace/rv-x/train.py", "-c", "config.yaml"},
	}))

	// Inline program text on a generated-code run still fails closed.
	err = gate.ReviewNormalExec(context.Background(), repository.CraftDockerNormalInputRequest{
		TenantID: tenant, RunID: "run-policy-r4", TaskID: "sess-policy-r4",
		WorkingDir: "/workspace",
		Command:    []string{"env", "-i", "python3", "-c", "print('hi')"},
	})
	require.ErrorIs(t, err, craft.ErrForbidden)
}

// TestCraftRestrictedExecResumeBoundIsGated is the OCR low-finding
// regression: the recovery send path screens through the same gate, so an
// operation bound before the gate existed can never be sent unreviewed.
func TestCraftRestrictedExecResumeBoundIsGated(t *testing.T) {
	coordinator, budget, _, _, _ := newCraftDockerCoordinatorFixture(t)
	svc, err := NewCraftDockerRestrictedExec(coordinator, &sandbox.DockerRemoteClient{}, time.Second)
	require.NoError(t, err)
	svc.WithExecutionPolicy(craftPolicyGate(t, budget.db))

	_, err = svc.ResumeBound(context.Background(), "grant-anything", "activity-anything")
	require.ErrorIs(t, err, craft.ErrForbidden, "the recovery send path must refuse unreviewable commands")
}

// TestCraftDelegateExecutionPolicyRefusesExplicitDigestPlusStdin is the
// round-2 medium finding regression: carrying BOTH an explicit
// TargetSHA256 and stdin bytes would silently disable the stdin channel's
// own byte-identity refusal (the stdin bytes could be uploaded material).
// The combination is refused fail-closed.
func TestCraftDelegateExecutionPolicyRefusesExplicitDigestPlusStdin(t *testing.T) {
	const tenant = uint64(9307)
	db := openCraftBudgetTestDB(t)
	seedCraftFundedTenant(t, db, tenant, 10000)
	seedCraftBudgetRun(t, db, tenant, "run-dual-evidence", "sess-dual-evidence")
	craftPolicySeed(t, db, tenant, "run-dual-evidence")
	gate := craftPolicyGate(t, db)

	err := gate.ReviewNormalExec(context.Background(), repository.CraftDockerNormalInputRequest{
		TenantID: tenant, RunID: "run-dual-evidence", WorkingDir: "/workspace",
		Command:      []string{"python3", "gen.py"},
		StdinEnabled: true, Stdin: []byte("uploaded-material-bytes"),
		TargetSHA256: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	})
	require.ErrorIs(t, err, craft.ErrForbidden, "the dual-evidence combination must be refused fail-closed")
}
