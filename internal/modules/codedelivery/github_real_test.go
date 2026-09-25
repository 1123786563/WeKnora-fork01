package codedelivery

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestGitHubClientAgainstRealGitHub 需要 WEKNORA_GITHUB_TEST_TOKEN（经典
// PAT，repo 只读作用域）与 WEKNORA_GITHUB_TEST_REPO（owner/name，公开仓库）。
// 缺 env 即 skip——真实 GitHub 属 true external（blocked-env），不伪造。
func TestGitHubClientAgainstRealGitHub(t *testing.T) {
	token := os.Getenv("WEKNORA_GITHUB_TEST_TOKEN")
	repoRaw := os.Getenv("WEKNORA_GITHUB_TEST_REPO")
	if token == "" || repoRaw == "" {
		t.Skip("WEKNORA_GITHUB_TEST_TOKEN/WEKNORA_GITHUB_TEST_REPO not set (blocked-env)")
	}
	repo, err := ParseRepoRef(repoRaw)
	require.NoError(t, err)
	client := NewGitHubClientFactory(httpClientDefault(), GitHubAPIBaseURL)(token, repo)
	ctx := context.Background()

	info, err := client.Repository(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, info.DefaultBranch)
	login, err := client.CurrentLogin(ctx)
	require.NoError(t, err)
	t.Logf("real github: repo=%s default=%s login=%s", repo, info.DefaultBranch, login)
}
