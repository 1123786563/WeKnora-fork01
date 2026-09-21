package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/Tencent/WeKnora/internal/modules/craft"
)

func TestDelegateRejectsAuthorityArguments(t *testing.T) {
	if _, err := ParseDelegateArgs([]byte(`{"goal":"report","tenant_id":2,"sandbox_url":"http://evil"}`)); err == nil {
		t.Fatal("authority injection")
	}
	if _, err := ParseDelegateArgs([]byte(`{"goal":"report","input_refs":[]}`)); err != nil {
		t.Fatal(err)
	}
}

func TestParseDelegateArgsStrictness(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		ok   bool
	}{
		{name: "goal only", raw: `{"goal":"ship it"}`, ok: true},
		{name: "goal and refs", raw: `{"goal":"ship","input_refs":["res://tenant/1/a"]}`, ok: true},
		{name: "blank goal", raw: `{"goal":"   "}`, ok: false},
		{name: "missing goal", raw: `{"input_refs":[]}`, ok: false},
		{name: "session injection", raw: `{"goal":"x","session_id":"evil"}`, ok: false},
		{name: "workspace injection", raw: `{"goal":"x","workspace_id":"evil"}`, ok: false},
		{name: "deadline injection", raw: `{"goal":"x","deadline":"2020-01-01"}`, ok: false},
		{name: "trailing json", raw: `{"goal":"x"} {"goal":"y"}`, ok: false},
		{name: "trailing garbage", raw: `{"goal":"x"}xx`, ok: false},
		{name: "not an object", raw: `[]`, ok: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseDelegateArgs([]byte(tc.raw))
			if tc.ok && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tc.ok && err == nil {
				t.Fatal("expected rejection")
			}
		})
	}
}

func newCraftDelegateTestTool(t *testing.T, delegate CraftDelegateCall) *CraftDelegateTool {
	t.Helper()
	tool, err := NewCraftDelegateTool(CraftDelegateToolConfig{
		Scope:       craft.Scope{TenantID: 7, UserID: "user-7", SessionID: "sess-7"},
		WorkspaceID: "ws-7",
		Inputs: []craft.Input{
			{Ref: "res://tenant/7/brief", Name: "brief.md", SHA256: strings.Repeat("a", 64), Bytes: 12},
		},
		SkillDigests: []string{"skill:abc"},
		Delegate:     delegate,
	})
	if err != nil {
		t.Fatal(err)
	}
	return tool
}

func craftDelegateExecCtx() context.Context {
	fence := agentruntime.Fence{
		RunKey: agentruntime.RunKey{TenantID: 7, RunID: "run-7"},
		Owner:  "worker-7", Epoch: 3,
	}
	ctx := agentruntime.WithRunFence(context.Background(), fence)
	return WithToolExecContext(ctx, &ToolExecContext{ToolCallID: "call-7"})
}

func TestCraftDelegateToolAssemblesTaskFromRunContext(t *testing.T) {
	var got craft.Task
	tool := newCraftDelegateTestTool(t, func(_ context.Context, task craft.Task) (craft.Result, error) {
		got = task
		return craft.Result{TaskID: task.ID, Status: "succeeded", Summary: "done"}, nil
	})
	ctx := craftDelegateExecCtx()
	result, err := tool.Execute(ctx, []byte(`{"goal":"build the site","input_refs":["res://tenant/7/brief"]}`))
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success {
		t.Fatalf("result = %#v", result)
	}
	if got.ToolCallID != "call-7" {
		t.Fatalf("tool call id = %q, want the durable dispatch identity", got.ToolCallID)
	}
	if got.Fence.RunID != "run-7" || got.Fence.Owner != "worker-7" || got.Fence.Epoch != 3 || got.Fence.TenantID != 7 {
		t.Fatalf("fence = %#v, want the durable run fence from context", got.Fence)
	}
	if got.Scope.TenantID != 7 || got.Scope.UserID != "user-7" || got.Scope.SessionID != "sess-7" {
		t.Fatalf("scope = %#v, want the assembled session scope", got.Scope)
	}
	if got.WorkspaceID != "ws-7" {
		t.Fatalf("workspace = %q", got.WorkspaceID)
	}
	if len(got.Inputs) != 1 || got.Inputs[0].Ref != "res://tenant/7/brief" {
		t.Fatalf("inputs = %#v", got.Inputs)
	}
	if !strings.Contains(got.Prompt, "build the site") || !strings.Contains(got.Prompt, "brief.md") {
		t.Fatalf("prompt = %q", got.Prompt)
	}
	wantHash := CraftDelegateRequestHash("build the site", got.Inputs, []string{"skill:abc"}, "ws-7")
	if got.RequestHash != wantHash {
		t.Fatalf("request hash = %q, want %q", got.RequestHash, wantHash)
	}
	if !got.Deadline.IsZero() {
		t.Fatalf("deadline = %v, want zero when the context carries none (executor default window)", got.Deadline)
	}
}

func TestCraftDelegateToolRequestHashCoversGoalInputsSkills(t *testing.T) {
	inputs := []craft.Input{{Ref: "r", SHA256: strings.Repeat("a", 64)}}
	base := CraftDelegateRequestHash("goal", inputs, []string{"skill:1"}, "ws")
	if CraftDelegateRequestHash("goal-2", inputs, []string{"skill:1"}, "ws") == base {
		t.Fatal("goal must be covered")
	}
	other := []craft.Input{{Ref: "r", SHA256: strings.Repeat("b", 64)}}
	if CraftDelegateRequestHash("goal", other, []string{"skill:1"}, "ws") == base {
		t.Fatal("input digest must be covered")
	}
	if CraftDelegateRequestHash("goal", inputs, []string{"skill:2"}, "ws") == base {
		t.Fatal("skill version must be covered")
	}
}

func TestCraftDelegateToolUsesRunDeadline(t *testing.T) {
	deadline := time.Now().Add(3 * time.Minute).Round(0).UTC()
	var got craft.Task
	tool := newCraftDelegateTestTool(t, func(_ context.Context, task craft.Task) (craft.Result, error) {
		got = task
		return craft.Result{Status: "succeeded"}, nil
	})
	ctx, cancel := context.WithDeadline(craftDelegateExecCtx(), deadline)
	defer cancel()
	if _, err := tool.Execute(ctx, []byte(`{"goal":"x"}`)); err != nil {
		t.Fatal(err)
	}
	if !got.Deadline.Equal(deadline) {
		t.Fatalf("deadline = %v, want %v", got.Deadline, deadline)
	}
}

func TestCraftDelegateToolRejectsUnauthorizedInputs(t *testing.T) {
	tool := newCraftDelegateTestTool(t, func(context.Context, craft.Task) (craft.Result, error) {
		t.Fatal("delegate must not run for an unauthorized reference")
		return craft.Result{}, nil
	})
	result, err := tool.Execute(craftDelegateExecCtx(), []byte(`{"goal":"x","input_refs":["res://other/1/file"]}`))
	if err != nil {
		t.Fatal(err)
	}
	if result.Success || result.Error == "" {
		t.Fatalf("result = %#v", result)
	}
}

func TestCraftDelegateToolRequiresDurableFence(t *testing.T) {
	tool := newCraftDelegateTestTool(t, func(context.Context, craft.Task) (craft.Result, error) {
		t.Fatal("delegate must not run without a durable fence")
		return craft.Result{}, nil
	})
	result, err := tool.Execute(WithToolExecContext(context.Background(), &ToolExecContext{ToolCallID: "call-7"}), []byte(`{"goal":"x"}`))
	if err != nil {
		t.Fatal(err)
	}
	if result.Success {
		t.Fatalf("result = %#v", result)
	}
}

func TestCraftDelegateToolErrorMapping(t *testing.T) {
	unknown := fmt.Errorf("wrapped: %w", craft.ErrUnknown)
	tool := newCraftDelegateTestTool(t, func(context.Context, craft.Task) (craft.Result, error) {
		return craft.Result{}, unknown
	})
	if _, err := tool.Execute(craftDelegateExecCtx(), []byte(`{"goal":"x"}`)); !errors.Is(err, craft.ErrUnknown) {
		t.Fatalf("err = %v, want craft.ErrUnknown propagated for pending outcomes", err)
	}

	forbidden := fmt.Errorf("nope: %w", craft.ErrForbidden)
	tool2 := newCraftDelegateTestTool(t, func(context.Context, craft.Task) (craft.Result, error) {
		return craft.Result{}, forbidden
	})
	result, err := tool2.Execute(craftDelegateExecCtx(), []byte(`{"goal":"x"}`))
	if err != nil {
		t.Fatalf("definitive failures must not propagate as errors: %v", err)
	}
	if result.Success || !strings.Contains(result.Error, craft.ErrForbidden.Error()) {
		t.Fatalf("result = %#v", result)
	}
}

func TestCraftDelegateToolRendersBoundedSummary(t *testing.T) {
	big := craft.Result{
		TaskID:  "dlg_big",
		Status:  "failed",
		Summary: strings.Repeat("细节", 8<<9),
		Checks:  []craft.Check{{Name: "build", Status: "failed", Detail: strings.Repeat("d", 512)}},
		Files:   []craft.File{{Path: "index.html", Ref: "resource://artifact/1", Bytes: 42}},
	}
	result := renderCraftDelegateResult(big)
	if result.Success {
		t.Fatal("failed status must render unsuccessful")
	}
	if len(result.Output) > craftDelegateSummaryLimit {
		t.Fatalf("summary length = %d, want <= %d", len(result.Output), craftDelegateSummaryLimit)
	}
	if !strings.HasPrefix(result.Output, "status: failed") || !strings.Contains(result.Output, "dlg_big") {
		t.Fatalf("output = %q", result.Output[:200])
	}

	ok := renderCraftDelegateResult(craft.Result{
		Status: "succeeded", Summary: "ok",
		Checks: []craft.Check{{Name: "build", Status: "passed"}},
	})
	if !ok.Success || !strings.Contains(ok.Output, "build: passed") {
		t.Fatalf("result = %#v", ok)
	}
}
