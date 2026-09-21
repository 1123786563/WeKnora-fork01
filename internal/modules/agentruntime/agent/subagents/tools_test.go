package subagents

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/tools"
	"github.com/Tencent/WeKnora/internal/modules/airesource/models/chat"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- MapRoleTools ----------------------------------------------------------

func TestMapRoleToolsEnglishNames(t *testing.T) {
	cases := map[string]string{
		"WebSearch": tools.ToolWebSearch,
		"WebFetch":  tools.ToolWebFetch,
		"Read":      tools.ToolReadFile,
		"Write":     tools.ToolWriteSandboxFile,
		"Edit":      tools.ToolEditSandboxFile,
		"Bash":      tools.ToolShellExec,
	}
	for raw, want := range cases {
		assert.Equal(t, []string{want}, MapRoleTools(raw), raw)
	}
}

func TestMapRoleToolsChineseNames(t *testing.T) {
	cases := map[string]string{
		"搜索": tools.ToolWebSearch,
		"网页": tools.ToolWebFetch,
		"阅读": tools.ToolReadFile,
		"写作": tools.ToolWriteSandboxFile,
		"编辑": tools.ToolEditSandboxFile,
		"终端": tools.ToolShellExec,
	}
	for raw, want := range cases {
		assert.Equal(t, []string{want}, MapRoleTools(raw), raw)
	}
}

func TestMapRoleToolsSplitsCommaAndCJKLists(t *testing.T) {
	// The two raw shapes that actually ship in the library.
	assert.Equal(t, []string{
		tools.ToolWebFetch, tools.ToolWebSearch,
		tools.ToolReadFile, tools.ToolWriteSandboxFile, tools.ToolEditSandboxFile,
	}, MapRoleTools("WebFetch, WebSearch, Read, Write, Edit"))
	assert.Equal(t, []string{
		tools.ToolWebFetch, tools.ToolWebSearch,
		tools.ToolReadFile, tools.ToolWriteSandboxFile, tools.ToolEditSandboxFile,
		tools.ToolShellExec,
	}, MapRoleTools("WebFetch, WebSearch, Read, Write, Edit, Bash"))
	assert.Equal(t, []string{
		tools.ToolReadFile, tools.ToolWriteSandboxFile, tools.ToolEditSandboxFile,
	}, MapRoleTools("阅读、写作、编辑"))
}

func TestMapRoleToolsDropsUnknownNames(t *testing.T) {
	assert.Equal(t, []string{tools.ToolReadFile, tools.ToolWriteSandboxFile},
		MapRoleTools("Read, MysteryTool, Write"))
	assert.Empty(t, MapRoleTools("MysteryTool"))
	assert.Empty(t, MapRoleTools(""))
}

func TestMapRoleToolsDeduplicatesTargets(t *testing.T) {
	// Two source names may name the same WeKnora tool; the sub-run SDK
	// rejects duplicate declarations.
	assert.Equal(t, []string{tools.ToolReadFile}, MapRoleTools("Read, 阅读"))
}

// TestRoleToolNameMapTargetsAreRealTools locks every mapping target against
// the tool names the engine can actually register. A target that is not a
// WeKnora tool would silently grant nothing — the mapping table must name
// real identifiers only.
func TestRoleToolNameMapTargetsAreRealTools(t *testing.T) {
	real := map[string]bool{
		tools.ToolWebSearch:        true,
		tools.ToolWebFetch:         true,
		tools.ToolReadFile:         true,
		tools.ToolWriteSandboxFile: true,
		tools.ToolEditSandboxFile:  true,
		tools.ToolShellExec:        true,
	}
	for source, target := range roleToolNameMap {
		assert.True(t, real[target], "mapping %q targets unknown tool %q", source, target)
	}
}

// ---- ParseRoleMarkdown ------------------------------------------------------

func TestParseRoleMarkdown(t *testing.T) {
	content := "---\nname: 审计员\ndescription: 审计\ntools: Read, Write\n---\n\n# 审计员\n\n正文"
	fm, body, err := ParseRoleMarkdown(content)
	require.NoError(t, err)
	assert.Equal(t, "审计员", fm.Name)
	assert.Equal(t, "Read, Write", fm.ToolsRaw)
	// The body is verbatim below the closing fence line — the blank line
	// before the heading included (the scanner's contract).
	assert.Equal(t, "\n# 审计员\n\n正文", body)

	_, _, err = ParseRoleMarkdown("# no frontmatter\n")
	assert.Error(t, err)

	_, _, err = ParseRoleMarkdown("---\ndescription: no name\n---\nbody")
	assert.Error(t, err, "a role without a name must not resolve")
}

// ---- ParseDelegateArgs ------------------------------------------------------

func TestParseDelegateArgs(t *testing.T) {
	args, err := ParseDelegateArgs([]byte(`{"slug":"code-reviewer","goal":"review the diff","input_refs":["doc://1"]}`))
	require.NoError(t, err)
	assert.Equal(t, "code-reviewer", args.Slug)
	assert.Equal(t, "review the diff", args.Goal)
	assert.Equal(t, []string{"doc://1"}, args.InputRefs)

	// Strict surface: unknown fields (injected authority) are rejected.
	_, err = ParseDelegateArgs([]byte(`{"slug":"s","goal":"g","tenant_id":1}`))
	assert.Error(t, err)

	// Trailing input after the first JSON value is rejected.
	_, err = ParseDelegateArgs([]byte(`{"slug":"s","goal":"g"} {"slug":"s","goal":"evil"}`))
	assert.Error(t, err)

	// Blank goal is rejected.
	_, err = ParseDelegateArgs([]byte(`{"slug":"s","goal":"   "}`))
	assert.Error(t, err)

	// Blank slug can never resolve to an installed specialist.
	_, err = ParseDelegateArgs([]byte(`{"slug":"","goal":"g"}`))
	assert.Error(t, err)
}

// ---- tool -------------------------------------------------------------------

// fakeDelegateChat satisfies chat.Chat for the Model slot; delegation tests
// never call it (Exec is faked).
type fakeDelegateChat struct{}

func (fakeDelegateChat) Chat(context.Context, []chat.Message, *chat.ChatOptions) (*types.ChatResponse, error) {
	return nil, errors.New("unused")
}

func (fakeDelegateChat) ChatStream(context.Context, []chat.Message, *chat.ChatOptions) (<-chan types.StreamResponse, error) {
	return nil, errors.New("unused")
}

func (fakeDelegateChat) GetModelName() string { return "fake-delegate-model" }

func (fakeDelegateChat) GetModelID() string { return "fake-delegate-model" }

// fakeRegistryTool is a registry-resident tool the intersection can pull.
type fakeRegistryTool struct{ name string }

func (f *fakeRegistryTool) Name() string        { return f.name }
func (f *fakeRegistryTool) Description() string { return "fake " + f.name }
func (f *fakeRegistryTool) Parameters() json.RawMessage {
	return json.RawMessage(`{"type":"object"}`)
}

func (f *fakeRegistryTool) Execute(context.Context, json.RawMessage) (*types.ToolResult, error) {
	return &types.ToolResult{Success: true, Output: "ok"}, nil
}

func newDelegateToolForTest(t *testing.T, cfg SubagentDelegateToolConfig) *SubagentDelegateTool {
	t.Helper()
	if cfg.SessionID == "" {
		cfg.SessionID = "session-1"
	}
	if cfg.UserID == "" {
		cfg.UserID = "u1"
	}
	if cfg.Slugs == nil {
		cfg.Slugs = []string{"code-reviewer"}
	}
	if cfg.Model == nil {
		cfg.Model = fakeDelegateChat{}
	}
	if cfg.Lookup == nil {
		cfg.Lookup = func(string) (ResolvedRole, error) {
			return ResolvedRole{}, errors.New("no lookup wired")
		}
	}
	if cfg.Exec == nil {
		cfg.Exec = func(context.Context, ExecuteRequest) (ExecuteResult, error) {
			return ExecuteResult{}, errors.New("no exec wired")
		}
	}
	tool, err := NewSubagentDelegateTool(cfg)
	require.NoError(t, err)
	return tool
}

func TestNewSubagentDelegateToolValidation(t *testing.T) {
	base := SubagentDelegateToolConfig{
		SessionID: "s1",
		Slugs:     []string{"code-reviewer"},
		Model:     fakeDelegateChat{},
		Lookup:    func(string) (ResolvedRole, error) { return ResolvedRole{}, nil },
		Exec:      func(context.Context, ExecuteRequest) (ExecuteResult, error) { return ExecuteResult{}, nil },
	}

	_, err := NewSubagentDelegateTool(SubagentDelegateToolConfig{})
	require.Error(t, err, "an empty config must not build a tool")

	noSession := base
	noSession.SessionID = ""
	_, err = NewSubagentDelegateTool(noSession)
	assert.Error(t, err)

	noSlugs := base
	noSlugs.Slugs = nil
	_, err = NewSubagentDelegateTool(noSlugs)
	assert.Error(t, err)

	noLookup := base
	noLookup.Lookup = nil
	_, err = NewSubagentDelegateTool(noLookup)
	assert.Error(t, err)

	noExec := base
	noExec.Exec = nil
	_, err = NewSubagentDelegateTool(noExec)
	assert.Error(t, err)

	noModel := base
	noModel.Model = nil
	_, err = NewSubagentDelegateTool(noModel)
	assert.Error(t, err)
}

func TestSubagentDelegateToolDescriptionListsSlugs(t *testing.T) {
	tool := newDelegateToolForTest(t, SubagentDelegateToolConfig{Slugs: []string{"code-reviewer", "web-researcher"}})
	assert.Equal(t, ToolSubagentDelegate, tool.Name())
	assert.Contains(t, tool.Description(), "code-reviewer")
	assert.Contains(t, tool.Description(), "web-researcher")
	// The model must never see specialists that are not installed.
	assert.NotContains(t, tool.Description(), "ghost-role")
}

func TestSubagentDelegateToolHappyPath(t *testing.T) {
	registry := tools.NewToolRegistry()
	registry.RegisterTool(&fakeRegistryTool{name: tools.ToolWebSearch})

	var captured ExecuteRequest
	tool := newDelegateToolForTest(t, SubagentDelegateToolConfig{
		AllowedTools: []string{tools.ToolWebSearch, tools.ToolWebFetch},
		Registry:     registry,
		Emit:         func(context.Context, event.Event) {},
		Lookup: func(slug string) (ResolvedRole, error) {
			require.Equal(t, "code-reviewer", slug)
			return ResolvedRole{
				Slug:         slug,
				Label:        "代码评审",
				SystemPrompt: "# 代码评审\n\n审查代码。",
				ToolsRaw:     "WebSearch, WebFetch",
			}, nil
		},
		Exec: func(ctx context.Context, req ExecuteRequest) (ExecuteResult, error) {
			captured = req
			return ExecuteResult{Summary: "审查完成", Rounds: 3, Overrun: true}, nil
		},
	})

	result, err := tool.Execute(t.Context(), json.RawMessage(
		`{"slug":"code-reviewer","goal":"review the diff","input_refs":["doc://1"]}`))
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)
	assert.Equal(t, "审查完成", result.Output)
	assert.Equal(t, map[string]interface{}{
		"slug":    "code-reviewer",
		"rounds":  3,
		"overrun": true,
	}, result.Data)

	assert.Equal(t, "session-1", captured.SessionID)
	assert.Equal(t, "subagent:code-reviewer", captured.RunLabel)
	assert.Equal(t, "# 代码评审\n\n审查代码。", captured.SystemPrompt)
	assert.Equal(t, "review the diff", captured.Goal)
	assert.Equal(t, []string{"doc://1"}, captured.InputRefs)
	assert.Equal(t, "u1", captured.UserID)
	assert.NotNil(t, captured.Model)
	assert.NotNil(t, captured.Emit)

	// The intersection pulls only the role tools that are BOTH on the main
	// allowlist and constructed in the registry (web_fetch is allowed but not
	// registered, so only web_search survives).
	require.Len(t, captured.Tools, 1)
	assert.Equal(t, tools.ToolWebSearch, captured.Tools[0].Name())
}

func TestSubagentDelegateToolRejectsUninstalledSlug(t *testing.T) {
	execCalled := false
	tool := newDelegateToolForTest(t, SubagentDelegateToolConfig{
		Lookup: func(slug string) (ResolvedRole, error) {
			return ResolvedRole{}, ErrSubagentNotInstalled
		},
		Exec: func(context.Context, ExecuteRequest) (ExecuteResult, error) {
			execCalled = true
			return ExecuteResult{}, nil
		},
	})

	result, err := tool.Execute(t.Context(), json.RawMessage(`{"slug":"ghost","goal":"do it"}`))
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "subagent not installed: ghost")
	assert.False(t, execCalled, "an uninstalled slug must never reach the executor")
}

func TestSubagentDelegateToolLookupFailure(t *testing.T) {
	tool := newDelegateToolForTest(t, SubagentDelegateToolConfig{
		Lookup: func(string) (ResolvedRole, error) {
			return ResolvedRole{}, errors.New("row unreadable")
		},
	})

	result, err := tool.Execute(t.Context(), json.RawMessage(`{"slug":"broken","goal":"do it"}`))
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "broken")
	assert.Contains(t, result.Error, "row unreadable")
}

func TestSubagentDelegateToolEmptyIntersectionRunsToolless(t *testing.T) {
	registry := tools.NewToolRegistry()
	registry.RegisterTool(&fakeRegistryTool{name: tools.ToolWebSearch})

	toolCount := -1
	tool := newDelegateToolForTest(t, SubagentDelegateToolConfig{
		AllowedTools: []string{tools.ToolWebSearch}, // role wants none of these
		Registry:     registry,
		Lookup: func(slug string) (ResolvedRole, error) {
			return ResolvedRole{Slug: slug, SystemPrompt: "prompt", ToolsRaw: "Read, Write, Edit"}, nil
		},
		Exec: func(ctx context.Context, req ExecuteRequest) (ExecuteResult, error) {
			toolCount = len(req.Tools)
			return ExecuteResult{Summary: "结论"}, nil
		},
	})

	result, err := tool.Execute(t.Context(), json.RawMessage(`{"slug":"code-reviewer","goal":"g"}`))
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)
	assert.Equal(t, 0, toolCount, "the sub-run must run first-class with no tools")
	assert.Equal(t, subagentNoToolsNote+"\n\n结论", result.Output)

	// Empty summary still carries the note alone.
	tool2 := newDelegateToolForTest(t, SubagentDelegateToolConfig{
		Lookup: func(slug string) (ResolvedRole, error) {
			return ResolvedRole{Slug: slug, SystemPrompt: "prompt", ToolsRaw: "Read"}, nil
		},
		Exec: func(context.Context, ExecuteRequest) (ExecuteResult, error) {
			return ExecuteResult{}, nil
		},
	})
	result2, err := tool2.Execute(t.Context(), json.RawMessage(`{"slug":"code-reviewer","goal":"g"}`))
	require.NoError(t, err)
	assert.Equal(t, subagentNoToolsNote, result2.Output)
}

func TestSubagentDelegateToolExecErrorIsToolResult(t *testing.T) {
	tool := newDelegateToolForTest(t, SubagentDelegateToolConfig{
		Lookup: func(slug string) (ResolvedRole, error) {
			return ResolvedRole{Slug: slug, SystemPrompt: "p"}, nil
		},
		Exec: func(context.Context, ExecuteRequest) (ExecuteResult, error) {
			return ExecuteResult{}, errors.New("model dial failed")
		},
	})

	result, err := tool.Execute(t.Context(), json.RawMessage(`{"slug":"code-reviewer","goal":"g"}`))
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "model dial failed")
}

func TestSubagentDelegateToolRejectsArguments(t *testing.T) {
	tool := newDelegateToolForTest(t, SubagentDelegateToolConfig{})
	result, err := tool.Execute(t.Context(), json.RawMessage(`{"goal":"no slug"}`))
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "subagent_delegate arguments rejected")
}

// TestSubagentDelegateToolUserIDFallsBackToToolExecContext pins the durable
// path: the worker's registration ctx carries no UserIDContextKey, but the
// durable graph wraps every tool call in a ToolExecContext carrying the run
// row's user id (agent_run_graph.go) — the same identity the main run's own
// tools get. Delegation must reuse it instead of running userless.
func TestSubagentDelegateToolUserIDFallsBackToToolExecContext(t *testing.T) {
	var gotUserID string
	newTool := func(registrationUserID string) *SubagentDelegateTool {
		tool, err := NewSubagentDelegateTool(SubagentDelegateToolConfig{
			SessionID: "session-1",
			UserID:    registrationUserID,
			Slugs:     []string{"code-reviewer"},
			Model:     fakeDelegateChat{},
			Lookup: func(slug string) (ResolvedRole, error) {
				return ResolvedRole{Slug: slug, SystemPrompt: "p"}, nil
			},
			Exec: func(ctx context.Context, req ExecuteRequest) (ExecuteResult, error) {
				gotUserID = req.UserID
				return ExecuteResult{Summary: "done"}, nil
			},
		})
		require.NoError(t, err)
		return tool
	}

	// Durable run: registration saw no user id; the graph's ToolExecContext
	// (run.UserID) resolves it at delegation time.
	durableCtx := tools.WithToolExecContext(t.Context(), &tools.ToolExecContext{
		UserID: "durable-run-user",
	})
	result, err := newTool("").Execute(durableCtx, json.RawMessage(`{"slug":"code-reviewer","goal":"g"}`))
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "durable-run-user", gotUserID)

	// Live run: the registration-time user id wins over the execution
	// context (whose UserID the builtin engine formats differently).
	liveCtx := tools.WithToolExecContext(t.Context(), &tools.ToolExecContext{
		UserID: "web_user:u1",
	})
	_, err = newTool("u1").Execute(liveCtx, json.RawMessage(`{"slug":"code-reviewer","goal":"g"}`))
	require.NoError(t, err)
	assert.Equal(t, "u1", gotUserID)

	// No identity anywhere: delegation still runs (the sub-run falls back to
	// its own default user), it never fails the tool call.
	_, err = newTool("").Execute(t.Context(), json.RawMessage(`{"slug":"code-reviewer","goal":"g"}`))
	require.NoError(t, err)
	assert.Equal(t, "", gotUserID)
}
