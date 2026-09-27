package codedelivery

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// GitLabAPIBaseURL is the reviewed production endpoint (gitlab.com SaaS).
// Tests inject an httptest URL; the factory never derives hosts from data.
// Self-hosted GitLab base URLs are a deployment concern, not model output.
const GitLabAPIBaseURL = "https://gitlab.com"

// NewGitLabClientFactory builds the GitLab REST v4 adapter behind the unified
// CodePlatformClient seam (T24 #54). The adapter hides every GitLab shape
// difference from the delivery chain:
//   - project addressing is the URL-escaped "owner/name" path segment;
//   - auth is the OAuth2 Bearer header (connections hold app-OAuth tokens);
//   - the push half collapses GitHub's blobs→tree→commit→ref chain into ONE
//     commits-API call (EnsureBranch) that CONVERGES the task branch to the
//     intended tree (baseline tree + approved changes): GitLab has no
//     client-controlled parent and no standalone blob/tree creation, and its
//     commits API never force-updates history — the same no-force-push
//     invariant the GitHub chain enforces with force:false, different
//     mechanism, hidden here;
//   - draft PR is a draft-marked merge request ("Draft: " title prefix) and
//     the chain's GitHub-shaped "owner:branch" head is stripped to a bare
//     source_branch;
//   - protected branches are name/wildcard PATTERNS, matched locally;
//   - the commit sha is assigned server-side: CreateCommit returns a
//     deterministic LOCAL placeholder and the dispatcher re-reads the branch
//     head after the push, so the placeholder never reaches the ledger.
//
// One client instance serves ONE dispatch/recovery operation; calls are
// sequential (the dispatcher is the only caller).
func NewGitLabClientFactory(httpClient *http.Client, baseURL string) CodePlatformClientFactory {
	if httpClient == nil {
		httpClient = httpClientDefault()
	}
	return func(token string, repo RepoRef) CodePlatformClient {
		return &gitLabRestClient{http: httpClient, base: baseURL, token: token, repo: repo}
	}
}

type gitLabRestClient struct {
	http  *http.Client
	base  string
	token string
	repo  RepoRef

	defaultBranch string            // cached from Repository()
	stagedBase    string            // ref CreateTree anchored on
	stagedEntries []TreeEntry       // approved mutations staged by CreateTree
	stagedMessage string            // staged by CreateCommit
	stagedBlobs   map[string][]byte // blob sha → content (from CreateBlob)
}

func (c *gitLabRestClient) projectSegment() string { return url.PathEscape(c.repo.String()) }

// call is the shared REST helper: any response is a definite outcome
// (*CodePlatformAPIError), a dial/timeout/EOF is unobservable
// (ErrCodeTransport), a malformed request never left the process
// (ErrCodeRequestInvalid).
func (c *gitLabRestClient) call(ctx context.Context, method, path string, body any, out any) (*http.Header, error) {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("%w: encode %s %s: %v", ErrCodeRequestInvalid, method, path, err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, reader)
	if err != nil {
		return nil, fmt.Errorf("%w: build %s %s: %v", ErrCodeRequestInvalid, method, path, err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %s %s: %v", ErrCodeTransport, method, path, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("%w: %s %s: read body: %v", ErrCodeTransport, method, path, err)
	}
	header := resp.Header
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var env struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(raw, &env)
		return &header, &CodePlatformAPIError{Status: resp.StatusCode, Endpoint: method + " " + path, Message: env.Message}
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			return &header, fmt.Errorf("%w: decode %s %s: %v", ErrCodeRequestInvalid, method, path, err)
		}
	}
	return &header, nil
}

// maxRawBodyBytes caps ONE raw (non-JSON) response body. It is a runaway
// guard, NOT a truncation point: a body over the cap is a transport-family
// ERROR — silently truncated bytes would be written into the session
// workspace by the baseline materializer. The cap must stay >=
// maxBaselineBytes (service.go, the 16MiB single-blob ceiling the baseline
// materializer admits) so a legal large blob is never refused.
const maxRawBodyBytes = 16 << 20

// callRaw fetches non-JSON payloads (raw blobs).
func (c *gitLabRestClient) callRaw(ctx context.Context, method, path string, out *[]byte) error {
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, nil)
	if err != nil {
		return fmt.Errorf("%w: build %s %s: %v", ErrCodeRequestInvalid, method, path, err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %s %s: %v", ErrCodeTransport, method, path, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxRawBodyBytes+1))
	if err != nil {
		return fmt.Errorf("%w: %s %s: read body: %v", ErrCodeTransport, method, path, err)
	}
	if len(raw) > maxRawBodyBytes {
		// Never hand back truncated bytes.
		return fmt.Errorf("%w: %s %s: response body exceeds cap %d bytes",
			ErrCodeTransport, method, path, maxRawBodyBytes)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(raw))
		if len(msg) > 200 {
			msg = msg[:200]
		}
		return &CodePlatformAPIError{Status: resp.StatusCode, Endpoint: method + " " + path, Message: msg}
	}
	*out = raw
	return nil
}

func (c *gitLabRestClient) Repository(ctx context.Context) (GitHubRepoInfo, error) {
	var out struct {
		DefaultBranch string `json:"default_branch"`
	}
	_, err := c.call(ctx, http.MethodGet, "/api/v4/projects/"+c.projectSegment(), nil, &out)
	if err != nil {
		return GitHubRepoInfo{}, err
	}
	c.defaultBranch = out.DefaultBranch
	return GitHubRepoInfo{DefaultBranch: out.DefaultBranch}, nil
}

// BranchProtected lists the project's protected-branch PATTERNS and matches
// locally (exact or wildcard) — GitLab has no per-branch boolean.
func (c *gitLabRestClient) BranchProtected(ctx context.Context, branch string) (bool, error) {
	for page := 1; page <= 10; page++ {
		var out []struct {
			Name string `json:"name"`
		}
		header, err := c.call(ctx, http.MethodGet,
			fmt.Sprintf("/api/v4/projects/%s/protected_branches?per_page=100&page=%d", c.projectSegment(), page), nil, &out)
		if err != nil {
			return false, err
		}
		for _, p := range out {
			if p.Name == branch || ProtectedBranchGlobMatch(p.Name, branch) {
				return true, nil
			}
		}
		if header.Get("X-Next-Page") == "" {
			return false, nil
		}
	}
	return false, &CodePlatformAPIError{Status: 0, Endpoint: "protected_branches", Message: "pagination exceeded local cap"}
}

// Tree reads the full recursive tree at a ref (commit sha or branch name),
// paging per_page=100 until a short page.
func (c *gitLabRestClient) Tree(ctx context.Context, ref string) (map[string]string, error) {
	entries := map[string]string{}
	for page := 1; page <= 50; page++ {
		var out []struct {
			ID   string `json:"id"`
			Type string `json:"type"`
			Path string `json:"path"`
		}
		q := url.Values{}
		q.Set("ref", ref)
		q.Set("recursive", "true")
		q.Set("per_page", "100")
		q.Set("page", fmt.Sprint(page))
		_, err := c.call(ctx, http.MethodGet, "/api/v4/projects/"+c.projectSegment()+"/repository/tree?"+q.Encode(), nil, &out)
		if err != nil {
			return nil, err
		}
		for _, e := range out {
			if e.Type == "blob" {
				entries[e.Path] = e.ID
			}
		}
		if len(out) < 100 {
			return entries, nil
		}
	}
	return nil, &CodePlatformAPIError{Status: 0, Endpoint: "repository/tree", Message: "tree pagination exceeded local cap"}
}

// CommitTree: GitLab has no client-visible tree handle distinct from the ref
// — this adapter's Tree accepts a commit/branch ref directly, so the identity
// mapping IS the hiding (zero remote spend; the paired Tree call does the
// read).
func (c *gitLabRestClient) CommitTree(ctx context.Context, commitSHA string) (string, error) {
	if commitSHA == "" {
		return "", &CodePlatformAPIError{Status: 0, Endpoint: "repository/commits", Message: "empty ref"}
	}
	return commitSHA, nil
}

// Blob reads raw bytes by blob sha (GitLab's raw-blob endpoint takes the sha,
// no path needed). The body is content-addressed: it must hash to the
// requested sha (GitBlobSHA, the same object id CreateBlob stages) — a
// mismatch means a truncated or misrouted read and is refused.
func (c *gitLabRestClient) Blob(ctx context.Context, sha string) ([]byte, error) {
	var raw []byte
	err := c.callRaw(ctx, http.MethodGet,
		"/api/v4/projects/"+c.projectSegment()+"/repository/blobs/"+url.PathEscape(sha)+"/raw", &raw)
	if err != nil {
		return nil, err
	}
	if got := GitBlobSHA(raw); got != sha {
		return nil, fmt.Errorf("%w: blob %s is not content-addressed: body hashes to %s", ErrCodeTransport, sha, got)
	}
	return raw, nil
}

// CreateBlob stages content locally under its REAL git blob sha
// (content-addressed — GitLab stores the identical object); the commits API
// materializes it later. Zero remote call.
func (c *gitLabRestClient) CreateBlob(ctx context.Context, content []byte) (string, error) {
	sha := GitBlobSHA(content)
	if c.stagedBlobs == nil {
		c.stagedBlobs = map[string][]byte{}
	}
	c.stagedBlobs[sha] = content
	return sha, nil
}

// CreateTree stages the approved mutations; GitLab composes trees server-side
// at commit time. The returned id is a deterministic LOCAL handle (sha256 of
// the staged intent) that only travels between this adapter's own methods.
func (c *gitLabRestClient) CreateTree(ctx context.Context, baseTree string, entries []TreeEntry) (string, error) {
	c.stagedBase = baseTree
	c.stagedEntries = append([]TreeEntry(nil), entries...)
	intent, err := json.Marshal(struct {
		Base    string      `json:"base"`
		Entries []TreeEntry `json:"entries"`
	}{Base: baseTree, Entries: c.stagedEntries})
	if err != nil {
		return "", fmt.Errorf("%w: stage tree: %v", ErrCodeRequestInvalid, err)
	}
	sum := sha256.Sum256(intent)
	return "gl-stage-" + hex.EncodeToString(sum[:8]), nil
}

// CreateCommit stages the message only. GitLab assigns the real commit
// server-side when the push (EnsureBranch) lands; the dispatcher re-reads the
// branch head, so the placeholder returned here never reaches the ledger.
func (c *gitLabRestClient) CreateCommit(ctx context.Context, parent, tree, message string) (string, error) {
	c.stagedMessage = message
	return "gl-commit-placeholder", nil
}

func (c *gitLabRestClient) baseBranch(ctx context.Context) (string, error) {
	if c.defaultBranch != "" {
		return c.defaultBranch, nil
	}
	info, err := c.Repository(ctx)
	if err != nil {
		return "", err
	}
	return info.DefaultBranch, nil
}

// EnsureBranch materializes the staged delivery as ONE GitLab commits-API
// call that converges <branch> to the intended tree (tree at stagedBase plus
// the staged entries). A missing branch is created from the default branch
// (start_branch); an existing branch is committed onto its CURRENT tip —
// GitLab never rewrites history. When the branch already sits exactly on the
// intended tree the call is a converged no-op (GitLab rejects empty actions;
// the tip IS the delivery fact).
func (c *gitLabRestClient) EnsureBranch(ctx context.Context, branch, commit string) error {
	base, err := c.baseBranch(ctx)
	if err != nil {
		return err
	}
	intended, err := c.Tree(ctx, c.stagedBase)
	if err != nil {
		return err
	}
	for _, e := range c.stagedEntries {
		if e.SHA == "" {
			delete(intended, e.Path)
			continue
		}
		intended[e.Path] = e.SHA
	}
	head, exists, err := c.BranchHead(ctx, branch)
	if err != nil {
		return err
	}
	startRef := base
	if exists {
		startRef = head
	}
	current, err := c.Tree(ctx, startRef)
	if err != nil {
		return err
	}
	type commitAction struct {
		Action   string `json:"action"`
		FilePath string `json:"file_path"`
		Content  string `json:"content,omitempty"`
		Encoding string `json:"encoding,omitempty"`
	}
	var actions []commitAction
	for path, want := range intended {
		if got, ok := current[path]; ok && got == want {
			continue
		}
		content, ok := c.stagedBlobs[want]
		if !ok {
			// 未在本派发中上传的内容（基线里已有、但起点分支缺它）：
			// 按内容寻址的 blob sha 从平台取回真实字节。
			content, err = c.Blob(ctx, want)
			if err != nil {
				return err
			}
		}
		act := "create"
		if _, onBranch := current[path]; onBranch {
			act = "update"
		}
		actions = append(actions, commitAction{Action: act, FilePath: path, Content: base64.StdEncoding.EncodeToString(content), Encoding: "base64"})
	}
	for path := range current {
		if _, ok := intended[path]; !ok {
			actions = append(actions, commitAction{Action: "delete", FilePath: path})
		}
	}
	if len(actions) == 0 {
		if exists {
			return nil // 已收敛：GitLab 不写空提交，tip 即交付事实
		}
		return fmt.Errorf("%w: task branch %s would be empty; nothing to push", ErrInvalidMaterial, branch)
	}
	if len(actions) > 2*MaxDeliveryFiles {
		return fmt.Errorf("%w: %d commit actions exceed cap %d", ErrBaselineTooLarge, len(actions), 2*MaxDeliveryFiles)
	}
	body := map[string]any{
		"branch":         branch,
		"commit_message": c.stagedMessage,
		"actions":        actions,
	}
	if !exists {
		body["start_branch"] = base
	}
	var out struct {
		ID string `json:"id"`
	}
	_, err = c.call(ctx, http.MethodPost, "/api/v4/projects/"+c.projectSegment()+"/repository/commits", body, &out)
	return err
}

// BranchHead resolves the branch tip; a missing branch is (absent), a
// transport/provider failure is an error.
func (c *gitLabRestClient) BranchHead(ctx context.Context, branch string) (string, bool, error) {
	var out struct {
		Commit struct {
			ID string `json:"id"`
		} `json:"commit"`
	}
	_, err := c.call(ctx, http.MethodGet,
		"/api/v4/projects/"+c.projectSegment()+"/repository/branches/"+url.PathEscape(branch), nil, &out)
	if err != nil {
		var apiErr *CodePlatformAPIError
		if errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound {
			return "", false, nil
		}
		return "", false, err
	}
	if out.Commit.ID == "" {
		return "", false, nil
	}
	return out.Commit.ID, true, nil
}

// DraftPullRequest creates (or reuses) the draft merge request. GitLab marks
// drafts by title prefix; the chain's GitHub-shaped "owner:branch" head is
// stripped to a bare source_branch. GitLab's 409 on a duplicate source branch
// resolves to the existing MR (Created=false).
func (c *gitLabRestClient) DraftPullRequest(ctx context.Context, input PullRequestInput) (PullRequestReceipt, error) {
	if existing, err := c.PullRequestForHead(ctx, input.Head); err == nil && existing != nil {
		// 复用既有开放 MR：Created 只在本次真实创建时为 true（与 GitHub 客户端同语义）。
		reuse := *existing
		reuse.Created = false
		return reuse, nil
	}
	body := map[string]any{
		"source_branch": c.branchOf(input.Head),
		"target_branch": input.Base,
		"title":         DraftMRTitle(input.Title),
	}
	var out struct {
		IID    int64  `json:"iid"`
		WebURL string `json:"web_url"`
	}
	_, err := c.call(ctx, http.MethodPost, "/api/v4/projects/"+c.projectSegment()+"/merge_requests", body, &out)
	if err != nil {
		var apiErr *CodePlatformAPIError
		if errors.As(err, &apiErr) && apiErr.Status == http.StatusConflict {
			if existing, rerr := c.PullRequestForHead(ctx, input.Head); rerr == nil && existing != nil {
				reuse := *existing
				reuse.Created = false
				return reuse, nil
			}
		}
		return PullRequestReceipt{}, err
	}
	return PullRequestReceipt{Number: out.IID, URL: out.WebURL, Draft: true, Created: true}, nil
}

func (c *gitLabRestClient) branchOf(head string) string {
	return strings.TrimPrefix(head, c.repo.Owner+":")
}

func (c *gitLabRestClient) PullRequestForHead(ctx context.Context, head string) (*PullRequestReceipt, error) {
	q := url.Values{}
	q.Set("source_branch", c.branchOf(head))
	q.Set("state", "opened")
	q.Set("per_page", "20")
	var out []struct {
		IID    int64  `json:"iid"`
		WebURL string `json:"web_url"`
		Title  string `json:"title"`
		State  string `json:"state"`
	}
	_, err := c.call(ctx, http.MethodGet, "/api/v4/projects/"+c.projectSegment()+"/merge_requests?"+q.Encode(), nil, &out)
	if err != nil {
		return nil, err
	}
	for _, mr := range out {
		if mr.State != "opened" {
			continue
		}
		return &PullRequestReceipt{Number: mr.IID, URL: mr.WebURL, Draft: strings.HasPrefix(mr.Title, draftMRPrefix), Created: true}, nil
	}
	return nil, nil
}

func (c *gitLabRestClient) CurrentLogin(ctx context.Context) (string, error) {
	var out struct {
		Username string `json:"username"`
	}
	_, err := c.call(ctx, http.MethodGet, "/api/v4/user", nil, &out)
	return out.Username, err
}
