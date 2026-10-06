package service

import (
	"context"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/agent/tools"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/stretchr/testify/require"
)

// t03UploadedScript is the member's uploaded python script. Uploading it is
// legitimate: it must stay read-only input material, never executable.
var t03UploadedScript = []byte("import csv\nprint('summarizing')\n")

func t03UploadDigest() string { return sha256Sum(string(t03UploadedScript)) }

const t03ScriptDigest = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func t03ServiceInput() craft.Input {
	return craft.Input{Ref: "ref-analyze.py", Name: "analyze.py", SHA256: t03ScriptDigest, Bytes: int64(len(t03UploadedScript)),
		Recognition: &craft.InputRecognition{Accepted: true, Understood: false, Reason: "no_supported_parser"}}
}

// uploadT03Code drives the member-visible upload journey through the real
// service seam and returns the persisted input manifest.
func uploadT03Code(t *testing.T, env *craftSessionEnv, scope craft.Scope, ctx context.Context) craft.Input {
	t.Helper()
	input, err := env.svc.AcceptInputRound(ctx, scope, []CraftInputUpload{{
		Name: "analyze.py", Content: t03UploadedScript, SHA256: t03UploadDigest(),
	}})
	require.NoError(t, err)
	require.Len(t, input, 1)
	persisted, err := env.svc.WorkspaceInputs(ctx, scope)
	require.NoError(t, err)
	require.Equal(t, input, persisted, "uploaded code must persist as input material")
	return input[0]
}

func t03DelegationTask(input craft.Input) craft.Task {
	task := delegateTestTask("summarize the uploaded script")
	task.Inputs = []craft.Input{input}
	task.RequestHash = tools.CraftDelegateRequestHash(task.Prompt, task.Inputs, nil, task.WorkspaceID)
	return task
}

// TestCraftT03Journey walks the full T03 member journey: upload code, delegate
// with it as read-only material, then verify every execution path against the
// uploaded bytes is denied with a member-visible refusal and distinct audit,
// while generated Workspace code still executes.
func TestCraftT03Journey(t *testing.T) {
	env := newCraftSessionEnv(t, openGate)
	env.svc.files = &craftT01Files{blobs: map[string][]byte{}}
	ws := createCraftSession(t, env, "u1", "t03-journey", "Sales page", "web")
	scope := ownerScope(1, "u1", ws.SessionID)
	ctx := craftCtx(1, "u1", ws.SessionID)

	// 1. The member uploads code. It is accepted as data: recognition never
	//    claims understanding and the provenance label says code.
	input := uploadT03Code(t, env, scope, ctx)
	require.Equal(t, &craft.InputRecognition{Accepted: true, Understood: false, Reason: "unrecognized_format"},
		input.Recognition, "uploaded code stays accepted-but-unparsed data")
	label := craft.ClassifyInputCode(input.Name, t03UploadedScript)
	require.True(t, label.IsCode, "provenance must label the uploaded script as code")

	// 2. Delegating with the uploaded script as material stays legitimate.
	store := newFakeDelegationStore()
	exec := &fakeDelegationExecutor{}
	delegate := NewCraftDelegateService(store, exec)
	task := t03DelegationTask(input)
	result, err := delegate.Delegate(ctx, task)
	require.NoError(t, err)
	require.Equal(t, "succeeded", result.Status)
	require.Equal(t, 1, exec.executeCount(), "reading uploaded code must not block delegation")

	// 3. The delegated-material policy screens every execution attempt.
	policy, err := delegate.MaterialPolicy("/workspace", task)
	require.NoError(t, err)
	staged, err := craft.InputPath(input.SHA256, input.Name)
	require.NoError(t, err)
	script := "/workspace/" + staged

	denials := []struct {
		name string
		req  craft.InputExecutionRequest
	}{
		{"interpreter", craft.InputExecutionRequest{Command: []string{"python3", script}, WorkingDir: "/workspace"}},
		{"direct", craft.InputExecutionRequest{Command: []string{script}, WorkingDir: "/workspace"}},
		{"shell", craft.InputExecutionRequest{Shell: true, CommandText: "sh -c 'python3 " + script + "'", WorkingDir: "/workspace"}},
		{"copy-then-execute", craft.InputExecutionRequest{Command: []string{"python3", "/tmp/run.py"}, WorkingDir: "/workspace", TargetPath: "/tmp/run.py", TargetSHA256: input.SHA256}},
		{"symlink", craft.InputExecutionRequest{Command: []string{"python3", "/tmp/link.py"}, WorkingDir: "/workspace", TargetPath: "/tmp/link.py", ResolvedTargetPath: script}},
		{"alias", craft.InputExecutionRequest{Command: []string{"python3", "//workspace/./inputs/../inputs/" + input.SHA256 + "/analyze.py"}, WorkingDir: "/workspace"}},
		{"temp-dir", craft.InputExecutionRequest{Command: []string{"/tmp/.tmpAB/copy.sh"}, WorkingDir: "/workspace", TargetPath: "/tmp/.tmpAB/copy.sh", TargetSHA256: input.SHA256}},
	}
	for _, d := range denials {
		decision := policy.ReviewExecution(ctx, d.req)
		require.False(t, decision.Allowed, "%s bypass must be denied", d.name)
		require.ErrorIs(t, decision.Err, craft.ErrExecutionDenied, "%s denial must carry the sentinel", d.name)
		require.Equal(t, craft.AuditKindInputExecuteDenied, decision.AuditKind, "%s audit kind", d.name)
		refusal := decision.Refusal()
		require.NotEmpty(t, refusal, "%s refusal must be member-visible", d.name)
		require.Contains(t, refusal, "Allowed alternative", "%s refusal names the alternative", d.name)
	}

	// 4. Generated code in the writable Workspace boundary still executes.
	generated := policy.ReviewExecution(ctx, craft.InputExecutionRequest{
		Command: []string{"python3", "/workspace/app/main.py"}, WorkingDir: "/workspace",
		TargetPath: "/workspace/app/main.py", TargetSHA256: strings.Repeat("f", 64),
	})
	require.True(t, generated.Allowed)
	require.Equal(t, craft.AuditKindGeneratedExecute, generated.AuditKind)

	// 5. Audit evidence distinguishes reading uploaded code from executing
	//    generated code.
	read := policy.AuditInputRead(ctx, input)
	require.Equal(t, craft.AuditKindInputRead, read.Kind)
	require.Equal(t, input.SHA256, read.Digest)
	require.NotEqual(t, read.Kind, generated.AuditKind)
	require.NotEqual(t, read.Kind, craft.AuditKindInputExecuteDenied)
}

// TestCraftDelegateMaterialPolicyValidatesTask guards the delegated-material
// policy construction: only well-formed input manifests naming a workspace
// build a policy.
func TestCraftDelegateMaterialPolicyValidatesTask(t *testing.T) {
	delegate := NewCraftDelegateService(newFakeDelegationStore(), &fakeDelegationExecutor{})
	task := t03DelegationTask(t03ServiceInput())

	if _, err := delegate.MaterialPolicy("/workspace", task); err != nil {
		t.Fatalf("valid task must build a policy: %v", err)
	}
	noWorkspace := task
	noWorkspace.WorkspaceID = ""
	if _, err := delegate.MaterialPolicy("/workspace", noWorkspace); err == nil {
		t.Fatalf("task without workspace must be rejected")
	}
	broken := task
	broken.Inputs = []craft.Input{{Ref: "r", Name: "x.py", SHA256: "bad", Bytes: 3}}
	if _, err := delegate.MaterialPolicy("/workspace", broken); err == nil {
		t.Fatalf("malformed input manifest must be rejected")
	}
	if _, err := delegate.MaterialPolicy("", task); err == nil {
		t.Fatalf("empty workspace root must be rejected")
	}
}

// TestCraftInputExecutionRefusalProjection checks the member-visible refusal
// projection in isolation: it names the read-only reason and the workspace
// alternative.
func TestCraftInputExecutionRefusalProjection(t *testing.T) {
	policy, err := craft.NewInputExecutionPolicy("/workspace", []craft.Input{t03ServiceInput()})
	if err != nil {
		t.Fatalf("policy: %v", err)
	}
	decision := policy.Review(craft.InputExecutionRequest{
		Command:    []string{"python3", "/workspace/inputs/" + t03ScriptDigest + "/analyze.py"},
		WorkingDir: "/workspace",
	})
	if decision.Allowed {
		t.Fatalf("uploaded code execution must be denied")
	}
	refusal := decision.Refusal()
	if !strings.Contains(refusal, "Allowed alternative") || !strings.Contains(refusal, "read-only") {
		t.Fatalf("refusal %q must state the read-only reason and the alternative", refusal)
	}
}
