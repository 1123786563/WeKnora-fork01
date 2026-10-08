package codedelivery

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// NewGitHubClientFactory builds the production REST adapter over the git-data
// API chain (blobs → tree(base_tree) → commit(parent) → refs/heads/<branch> →
// draft PR). baseURL is a reviewed constant in production and the injected
// emulator URL in tests. Every call pins BOTH the resolved token and the
// delivery's own RepoRef — the client is stateless between calls.
func NewGitHubClientFactory(httpClient *http.Client, baseURL string) GitHubClientFactory {
	if httpClient == nil {
		httpClient = httpClientDefault()
	}
	return func(token string, repo RepoRef) GitHubClient {
		return &gitHubRestClient{http: httpClient, base: baseURL, token: token, repo: repo}
	}
}

type gitHubRestClient struct {
	http  *http.Client
	base  string
	token string
	repo  RepoRef
}

func (c *gitHubRestClient) call(ctx context.Context, method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, reader)
	if err != nil {
		// 请求从未被构建成功 = 可证未出网；不得伪装成传输不可观测。
		return fmt.Errorf("%w: build %s %s: %v", ErrGitHubRequestInvalid, method, path, err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %s %s: %v", ErrGitHubTransport, method, path, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return fmt.Errorf("%w: %s %s: read body: %v", ErrGitHubTransport, method, path, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var env struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(raw, &env)
		return &GitHubAPIError{Status: resp.StatusCode, Endpoint: method + " " + path, Message: env.Message}
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(raw, out)
}

func (c *gitHubRestClient) Repository(ctx context.Context) (GitHubRepoInfo, error) {
	var out struct {
		DefaultBranch string `json:"default_branch"`
	}
	err := c.call(ctx, http.MethodGet, "/repos/"+c.repoString(), nil, &out)
	return GitHubRepoInfo{DefaultBranch: out.DefaultBranch}, err
}

func (c *gitHubRestClient) BranchProtected(ctx context.Context, branch string) (bool, error) {
	var out struct {
		Protected bool `json:"protected"`
	}
	err := c.call(ctx, http.MethodGet, "/repos/"+c.repoString()+"/branches/"+branch, nil, &out)
	if err != nil {
		var apiErr *GitHubAPIError
		if asGitHubAPIError(err, &apiErr) && apiErr.Status == http.StatusNotFound {
			// GitHub returns 404 when the task branch has not been created yet;
			// that is an unprotected branch, while every other response fails closed.
			return false, nil
		}
		return false, err
	}
	return out.Protected, nil
}

func (c *gitHubRestClient) Tree(ctx context.Context, sha string) (map[string]string, error) {
	var out struct {
		Truncated bool `json:"truncated"`
		Tree      []struct {
			Path string `json:"path"`
			Type string `json:"type"`
			SHA  string `json:"sha"`
		} `json:"tree"`
	}
	if err := c.call(ctx, http.MethodGet, "/repos/"+c.repoString()+"/git/trees/"+sha+"?recursive=1", nil, &out); err != nil {
		return nil, err
	}
	if out.Truncated {
		return nil, &GitHubAPIError{Status: 0, Endpoint: "git/trees", Message: "tree truncated beyond recursive limit"}
	}
	entries := make(map[string]string, len(out.Tree))
	for _, e := range out.Tree {
		if e.Type == "blob" {
			entries[e.Path] = e.SHA
		}
	}
	return entries, nil
}

func (c *gitHubRestClient) Blob(ctx context.Context, sha string) ([]byte, error) {
	var out struct {
		Encoding string `json:"encoding"`
		Content  string `json:"content"`
	}
	if err := c.call(ctx, http.MethodGet, "/repos/"+c.repoString()+"/git/blobs/"+sha, nil, &out); err != nil {
		return nil, err
	}
	if out.Encoding != "base64" {
		return nil, &GitHubAPIError{Status: 0, Endpoint: "git/blobs", Message: "unsupported encoding " + out.Encoding}
	}
	return base64.StdEncoding.DecodeString(out.Content)
}

// CommitTree resolves the tree sha of a commit (CreateTree's base_tree).
func (c *gitHubRestClient) CommitTree(ctx context.Context, commitSHA string) (string, error) {
	var out struct {
		Tree string `json:"tree"`
	}
	err := c.call(ctx, http.MethodGet, "/repos/"+c.repoString()+"/git/commits/"+commitSHA, nil, &out)
	return out.Tree, err
}

func (c *gitHubRestClient) CreateBlob(ctx context.Context, content []byte) (string, error) {
	var out struct {
		SHA string `json:"sha"`
	}
	err := c.call(ctx, http.MethodPost, "/repos/"+c.repoString()+"/git/blobs",
		map[string]string{"content": base64.StdEncoding.EncodeToString(content), "encoding": "base64"}, &out)
	return out.SHA, err
}

func (c *gitHubRestClient) CreateTree(ctx context.Context, baseTree string, entries []TreeEntry) (string, error) {
	type wireEntry struct {
		Path string  `json:"path"`
		Mode string  `json:"mode"`
		Type string  `json:"type"`
		SHA  *string `json:"sha"`
	}
	tree := make([]wireEntry, 0, len(entries))
	for _, e := range entries {
		mode := e.Mode
		if mode == "" {
			mode = "100644"
		}
		var sha *string
		if e.SHA != "" {
			sha = &e.SHA
		}
		tree = append(tree, wireEntry{Path: e.Path, Mode: mode, Type: "blob", SHA: sha})
	}
	var out struct {
		SHA string `json:"sha"`
	}
	err := c.call(ctx, http.MethodPost, "/repos/"+c.repoString()+"/git/trees",
		map[string]any{"base_tree": baseTree, "tree": tree}, &out)
	return out.SHA, err
}

func (c *gitHubRestClient) CreateCommit(ctx context.Context, parent, tree, message string) (string, error) {
	var out struct {
		SHA string `json:"sha"`
	}
	err := c.call(ctx, http.MethodPost, "/repos/"+c.repoString()+"/git/commits",
		map[string]any{"message": message, "tree": tree, "parents": []string{parent}}, &out)
	return out.SHA, err
}

// BranchHead reads refs/heads/<branch>; (sha,false,nil) when the branch
// does not exist. Used by unknown-resolution to read remote facts only.
func (c *gitHubRestClient) BranchHead(ctx context.Context, branch string) (string, bool, error) {
	var out struct {
		Object struct {
			SHA string `json:"sha"`
		} `json:"object"`
	}
	if err := c.call(ctx, http.MethodGet, "/repos/"+c.repoString()+"/git/ref/heads/"+branch, nil, &out); err != nil {
		var apiErr *GitHubAPIError
		if asGitHubAPIError(err, &apiErr) && apiErr.Status == http.StatusNotFound {
			return "", false, nil
		}
		return "", false, err
	}
	return out.Object.SHA, out.Object.SHA != "", nil
}

// EnsureBranch creates refs/heads/<branch>; a create 422 falls through to a
// force:false update ONLY when the ref genuinely already exists (verified by
// a ref read — a nonexistent ref would also 422 the PATCH, masking the create
// root cause). The update must be a fast-forward: callers chain each
// iteration's commit onto the branch's current head (dispatcher.deliver), so
// real GitHub accepts it; a non-fast-forward refusal surfaces verbatim.
func (c *gitHubRestClient) EnsureBranch(ctx context.Context, branch, commit string) error {
	create := map[string]any{"ref": "refs/heads/" + branch, "sha": commit}
	err := c.call(ctx, http.MethodPost, "/repos/"+c.repoString()+"/git/refs", create, nil)
	if err == nil {
		return nil
	}
	var apiErr *GitHubAPIError
	if !asGitHubAPIError(err, &apiErr) || apiErr.Status != http.StatusUnprocessableEntity {
		return err
	}
	// 仅当 ref 确实已存在（远端事实）才走更新路径；否则原样浮出 create 的
	// 根因（非法 sha、ref 名被拒等不再被注定失败的 PATCH 遮蔽）。
	_, exists, gerr := c.BranchHead(ctx, branch)
	if gerr != nil {
		return gerr
	}
	if !exists {
		return err
	}
	return c.call(ctx, http.MethodPatch, "/repos/"+c.repoString()+"/git/refs/heads/"+branch,
		map[string]any{"sha": commit, "force": false}, nil)
}

func (c *gitHubRestClient) DraftPullRequest(ctx context.Context, input PullRequestInput) (PullRequestReceipt, error) {
	if existing, err := c.PullRequestForHead(ctx, input.Head, input.Base); err == nil && existing != nil {
		return PullRequestReceipt{Number: existing.Number, URL: existing.URL, Draft: true, Created: false}, nil
	}
	body := map[string]any{"title": input.Title, "head": input.Head, "base": input.Base, "draft": true}
	var out struct {
		Number  int64  `json:"number"`
		HTMLURL string `json:"html_url"`
		Draft   bool   `json:"draft"`
	}
	if err := c.call(ctx, http.MethodPost, "/repos/"+c.repoString()+"/pulls", body, &out); err != nil {
		return PullRequestReceipt{}, err
	}
	return PullRequestReceipt{Number: out.Number, URL: out.HTMLURL, Draft: out.Draft, Created: true}, nil
}

func (c *gitHubRestClient) PullRequestForHead(ctx context.Context, head, base string) (*PullRequestReceipt, error) {
	// GitHub head ("owner:branch") is unique per repository, so the target
	// base is not an identity dimension on this platform — the parameter
	// exists to keep the shared port shape (R5-F8 is GitLab's same-source-
	// multiple-targets semantics; it cannot occur here).
	_ = base
	var out []struct {
		Number  int64  `json:"number"`
		HTMLURL string `json:"html_url"`
		Draft   bool   `json:"draft"`
		State   string `json:"state"`
	}
	if err := c.call(ctx, http.MethodGet, "/repos/"+c.repoString()+"/pulls?head="+head+"&state=open", nil, &out); err != nil {
		return nil, err
	}
	for _, pr := range out {
		if pr.State != "open" {
			continue
		}
		return &PullRequestReceipt{Number: pr.Number, URL: pr.HTMLURL, Draft: pr.Draft, Created: true}, nil
	}
	return nil, nil
}

func (c *gitHubRestClient) CurrentLogin(ctx context.Context) (string, error) {
	var out struct {
		Login string `json:"login"`
	}
	err := c.call(ctx, http.MethodGet, "/user", nil, &out)
	return out.Login, err
}

// repoString derives the REST path segment from the pinned RepoRef; the
// factory guarantees it was set at construction (ParseRepoRef-validated).
func (c *gitHubRestClient) repoString() string { return c.repo.String() }

func asGitHubAPIError(err error, target **GitHubAPIError) bool {
	apiErr, ok := err.(*GitHubAPIError)
	if ok {
		*target = apiErr
	}
	return ok
}
