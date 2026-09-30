package codedelivery

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestGitLabClientAgainstRealGitLab 需要 WEKNORA_GITLAB_TEST_TOKEN（OAuth 或
// PAT 令牌，api 作用域）与 WEKNORA_GITLAB_TEST_PROJECT（owner/name，可达仓库）。
// 缺 env 即 skip——真实 GitLab 属 true external（blocked-env），不伪造。
func TestGitLabClientAgainstRealGitLab(t *testing.T) {
	token := os.Getenv("WEKNORA_GITLAB_TEST_TOKEN")
	project := os.Getenv("WEKNORA_GITLAB_TEST_PROJECT")
	if token == "" || project == "" {
		t.Skip("WEKNORA_GITLAB_TEST_TOKEN/WEKNORA_GITLAB_TEST_PROJECT not set (blocked-env)")
	}
	repo, err := ParseRepoRef(project)
	require.NoError(t, err)
	client := NewGitLabClientFactory(httpClientDefault(), GitLabAPIBaseURL)(token, repo)
	ctx := context.Background()
	info, err := client.Repository(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, info.DefaultBranch)
	login, err := client.CurrentLogin(ctx)
	require.NoError(t, err)
	t.Logf("real gitlab: project=%s default=%s login=%s", repo, info.DefaultBranch, login)
}
