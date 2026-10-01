package codedelivery

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTaskBranchOfAndValidation(t *testing.T) {
	require.Equal(t, "weknora/task/s-1", TaskBranchOf("s-1"))
	require.NoError(t, ValidateTaskBranch("weknora/task/s-1"))
	// 非法：无前缀、空后缀、非法字符、过长（git refname 规则的子集白名单）
	require.ErrorIs(t, ValidateTaskBranch("main"), ErrInvalidBranch)
	require.ErrorIs(t, ValidateTaskBranch("weknora/task/"), ErrInvalidBranch)
	require.ErrorIs(t, ValidateTaskBranch("weknora/task/a b"), ErrInvalidBranch)
	require.ErrorIs(t, ValidateTaskBranch("weknora/task/a..b"), ErrInvalidBranch)
	require.ErrorIs(t, ValidateTaskBranch("weknora/task/"+string(make([]byte, 200))), ErrInvalidBranch)
}

func TestRefuseProtectedTarget(t *testing.T) {
	require.ErrorIs(t, RefuseProtectedTarget("main", "main", false), ErrProtectedBranch)
	require.ErrorIs(t, RefuseProtectedTarget("weknora/task/s-1", "main", true), ErrProtectedBranch)
	require.NoError(t, RefuseProtectedTarget("weknora/task/s-1", "main", false))
}

func TestParseRepoRefAndWorkspaceRoot(t *testing.T) {
	repo, err := ParseRepoRef("octocat/hello-world")
	require.NoError(t, err)
	require.Equal(t, RepoRef{Owner: "octocat", Name: "hello-world"}, repo)
	require.Equal(t, "octocat/hello-world", repo.String())
	_, err = ParseRepoRef("nope")
	require.ErrorIs(t, err, ErrRepoRefInvalid)
	_, err = ParseRepoRef("a/b/c")
	require.ErrorIs(t, err, ErrRepoRefInvalid)
	require.Equal(t, "/workspace/octocat/hello-world", WorkspaceRepoRoot(repo))
}

// 路径穿越回归：repo 段不得携带 ".."，否则 WorkspaceRepoRoot 拼接可突破
// 固定 /workspace 根（与任务分支后缀、交付文件路径同一防线）。
func TestParseRepoRefRefusesDotDotSegments(t *testing.T) {
	for _, bad := range []string{"../hello", "octocat/..", "octocat/a..b", "..", "octocat/../hello"} {
		_, err := ParseRepoRef(bad)
		require.ErrorIs(t, err, ErrRepoRefInvalid, "input %q must be refused", bad)
	}
	// 传导面：材料快照守卫复用同一 repo 校验，.. 段材料同被拒。
	mat := DeliveryMaterial{
		Repo:          RepoRef{Owner: "..", Name: "hello"},
		BaselineSHA:   "b" + strings.Repeat("0", 39),
		Branch:        TaskBranchOf("s-1"),
		Files:         []FileChange{{Path: "main.go"}},
		CommitMessage: "m",
		PRTitle:       "t",
	}
	_, err := ParseDeliveryMaterial(mustJSON(t, mat))
	require.ErrorIs(t, err, ErrRepoRefInvalid)
}

// git hash-object 与本实现同构：真实 git 生成的 blob sha 必须一致。
func TestGitBlobSHAMatchesRealGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available")
	}
	content := []byte("package main\n\nfunc main() {}\n")
	cmd := exec.Command("git", "hash-object", "--stdin")
	cmd.Stdin = bytes.NewReader(content)
	out, err := cmd.Output()
	require.NoError(t, err)
	require.Equal(t, strings.TrimSpace(string(out)), GitBlobSHA(content))
}

func TestDiffAgainstBaseline(t *testing.T) {
	baseline := map[string]string{"a.txt": "s-a", "b.txt": "s-b", "gone.txt": "s-g"}
	workspace := map[string]string{"a.txt": "s-a", "b.txt": "s-b2", "new.txt": "s-n"}
	changes := DiffAgainstBaseline(baseline, workspace)
	require.Equal(t, []FileChange{
		{Path: "b.txt", Deleted: false},
		{Path: "gone.txt", Deleted: true},
		{Path: "new.txt", Deleted: false},
	}, changes)
}

func TestParseDeliveryMaterialExactFields(t *testing.T) {
	mat := DeliveryMaterial{
		Repo: RepoRef{Owner: "octocat", Name: "hello"},
		BaselineSHA: "b" + strings.Repeat("0", 39), Branch: TaskBranchOf("s-1"),
		Files: []FileChange{{Path: "main.go", Deleted: false}},
		CommitMessage: "fix: greeting", PRTitle: "WeKnora task s-1",
	}
	raw, err := mat.CanonicalJSON()
	require.NoError(t, err)
	parsed, err := ParseDeliveryMaterial(raw)
	require.NoError(t, err)
	require.Equal(t, mat, parsed)

	// 缺字段
	_, err = ParseDeliveryMaterial(json.RawMessage(`{"repo":"octocat/hello"}`))
	require.ErrorIs(t, err, ErrInvalidMaterial)
	// 多字段（approve-then-rewrite 面）
	_, err = ParseDeliveryMaterial(json.RawMessage(`{"repo":"o/n","baseline_sha":"` + "b" + strings.Repeat("0", 39) + `","branch":"weknora/task/s-1","files":[],"commit_message":"m","pr_title":"t","extra":1}`))
	require.ErrorIs(t, err, ErrInvalidMaterial)
	// 非对象
	_, err = ParseDeliveryMaterial(json.RawMessage(`[]`))
	require.ErrorIs(t, err, ErrInvalidMaterial)
	// 非法 repo / 非法基线 / 非法分支 / 超量文件
	bad := mat
	bad.Repo = RepoRef{}
	_, err = ParseDeliveryMaterial(mustJSON(t, bad))
	require.ErrorIs(t, err, ErrRepoRefInvalid)
	bad = mat
	bad.BaselineSHA = "zz"
	_, err = ParseDeliveryMaterial(mustJSON(t, bad))
	require.ErrorIs(t, err, ErrInvalidBaselineSHA)
	bad = mat
	bad.Branch = "main"
	_, err = ParseDeliveryMaterial(mustJSON(t, bad))
	require.ErrorIs(t, err, ErrInvalidBranch)
	bad = mat
	bad.Files = make([]FileChange, MaxDeliveryFiles+1)
	_, err = ParseDeliveryMaterial(mustJSON(t, bad))
	require.ErrorIs(t, err, ErrBaselineTooLarge)
}

func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(v)
	require.NoError(t, err)
	return raw
}
