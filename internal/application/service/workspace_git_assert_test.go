package service

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/Tencent/WeKnora/internal/modules/execution/sandbox"
)

// assertWorkspaceGitLayout pins the fork/checkpoint git plumbing shape
// (moved out of the dropped workspace_git_layout_test.go).
func assertWorkspaceGitLayout(t *testing.T, script string) {
	t.Helper()
	require.Contains(t, script, "--git-dir=")
	require.Contains(t, script, sandbox.SessionGitDir)
	require.Contains(t, script, "--work-tree=")
	require.Contains(t, script, sandbox.SessionWorkspaceRoot)
	require.NotContains(t, script, sandbox.SessionWorkspaceRoot+"/.git")
}
