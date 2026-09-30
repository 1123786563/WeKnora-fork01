package codedelivery

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// githubEmulator 是一个内存 GitHub：真实 HTTP 字节进出生产客户端，
// blobs 以真实 git blob sha 入库（与 GitBlobSHA 同构），并记录每类调用次数
// 与违规（merge 尝试、越权 ref 写）。blackout 模拟传输不可观测。
type githubEmulator struct {
	t                 *testing.T
	srv               *httptest.Server
	mu                sync.Mutex
	token             string
	calls             map[string]int
	bad               []string
	repo              map[string][]byte            // path → content（默认分支树）
	blobs             map[string][]byte            // sha → content
	trees             map[string]map[string]string // tree sha → path→blob sha
	commits           map[string]string            // commit sha → tree sha
	commitParents     map[string][]string          // commit sha → parents（fast-forward 判定）
	refs              map[string]string            // refs/heads/<branch> → commit sha
	prs               []emulatorPR
	nextPR            int64
	protectedBranches []string
	failPR            bool // 下一条 POST /pulls 确定性 422 一次
	blackout          bool // 所有请求 hijack 断连（传输不可观测）
	blackoutAfterRef  bool // 下一次成功的 POST /git/refs 之后进入 blackout
}

type emulatorPR struct {
	Number int64
	Title  string
	Head   string
	Base   string
	Draft  bool
	State  string
}

func newGitHubEmulator(t *testing.T) *githubEmulator {
	e := &githubEmulator{
		t: t, token: "gho_testtoken", calls: map[string]int{},
		blobs: map[string][]byte{}, trees: map[string]map[string]string{},
		commits: map[string]string{}, commitParents: map[string][]string{},
		refs: map[string]string{}, prs: []emulatorPR{},
		protectedBranches: []string{"prod"},
		repo: map[string][]byte{
			"README.md": []byte("# hello\n"),
			"main.go":   []byte("package main\n"),
		},
	}
	tree := map[string]string{}
	for p, c := range e.repo {
		sha := GitBlobSHA(c)
		e.blobs[sha] = c
		tree[p] = sha
	}
	e.trees["tree-baseline"] = tree
	e.commits["b"+strings.Repeat("0", 39)] = "tree-baseline"
	e.refs["refs/heads/main"] = "b" + strings.Repeat("0", 39)
	mux := http.NewServeMux()
	mux.HandleFunc("/", e.serve)
	e.srv = httptest.NewServer(mux)
	t.Cleanup(e.srv.Close)
	return e
}

func (e *githubEmulator) note(call string) { e.mu.Lock(); e.calls[call]++; e.mu.Unlock() }

func (e *githubEmulator) violation(v string) {
	e.mu.Lock()
	e.bad = append(e.bad, v)
	e.mu.Unlock()
}

func (e *githubEmulator) Calls() map[string]int { return e.calls }
func (e *githubEmulator) Violations() []string  { return e.bad }

func (e *githubEmulator) BranchCommit(branch string) (string, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	sha, ok := e.refs["refs/heads/"+branch]
	return sha, ok
}

// ParentOf returns the recorded parents of a commit (fast-forward assertions).
func (e *githubEmulator) ParentOf(sha string) []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]string(nil), e.commitParents[sha]...)
}

func (e *githubEmulator) protectBranch(branch string) {
	e.mu.Lock()
	e.protectedBranches = append(e.protectedBranches, branch)
	e.mu.Unlock()
}

func (e *githubEmulator) failNextPRCreation() { e.mu.Lock(); e.failPR = true; e.mu.Unlock() }

// blackoutAfterRefCreate：下一次成功的 POST /git/refs 之后，所有后续请求
// hijack 断连（模拟推送已完成、PR 创建中途网络不可观测）。
func (e *githubEmulator) blackoutAfterRefCreate() {
	e.mu.Lock()
	e.blackoutAfterRef = true
	e.mu.Unlock()
}

func (e *githubEmulator) liftBlackout() { e.mu.Lock(); e.blackout = false; e.mu.Unlock() }

func (e *githubEmulator) isProtected(branch string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, b := range e.protectedBranches {
		if b == branch {
			return true
		}
	}
	return false
}

func (e *githubEmulator) branchExists(branch string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, exists := e.refs["refs/heads/"+branch]; exists {
		return true
	}
	for _, protected := range e.protectedBranches {
		if protected == branch {
			return true
		}
	}
	return false
}

func (e *githubEmulator) serve(w http.ResponseWriter, r *http.Request) {
	e.mu.Lock()
	blackout := e.blackout
	failPR := e.failPR
	e.mu.Unlock()
	if blackout {
		// 连接直接断开：客户端拿到 transport 错误（不可观测）。
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		panic(http.ErrAbortHandler)
	}
	if got := r.Header.Get("Authorization"); got != "Bearer "+e.token {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{"message": "bad credentials"})
		return
	}
	// 结构性护栏：任何 merge 尝试都是违规并被拒绝。
	if strings.Contains(r.URL.Path, "/merge") && r.Method == http.MethodPut {
		e.violation("merge attempted: " + r.Method + " " + r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	path := r.URL.Path
	switch {
	case r.Method == http.MethodGet && path == "/user":
		e.note("GET /user")
		writeJSON(w, map[string]any{"login": "octocat"})
	case r.Method == http.MethodGet && path == "/repos/octocat/hello":
		e.note("GET /repos")
		writeJSON(w, map[string]any{"default_branch": "main", "private": true})
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/repos/octocat/hello/branches/"):
		e.note("GET /branches")
		branch := strings.TrimPrefix(path, "/repos/octocat/hello/branches/")
		if !e.branchExists(branch) {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{"message": "branch not found"})
			return
		}
		writeJSON(w, map[string]any{"name": branch, "protected": e.isProtected(branch)})
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/repos/octocat/hello/git/commits/"):
		e.note("GET /git/commits")
		sha := strings.TrimPrefix(path, "/repos/octocat/hello/git/commits/")
		e.mu.Lock()
		treeSHA, ok := e.commits[sha]
		e.mu.Unlock()
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		writeJSON(w, map[string]any{"sha": sha, "tree": treeSHA, "parents": []string{}})
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/repos/octocat/hello/git/ref/heads/"):
		e.note("GET /git/ref")
		branch := strings.TrimPrefix(path, "/repos/octocat/hello/git/ref/heads/")
		e.mu.Lock()
		sha, ok := e.refs["refs/heads/"+branch]
		e.mu.Unlock()
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		writeJSON(w, map[string]any{"ref": "refs/heads/" + branch, "object": map[string]any{"sha": sha}})
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/repos/octocat/hello/git/trees/"):
		e.note("GET /git/trees")
		sha := strings.TrimPrefix(path, "/repos/octocat/hello/git/trees/")
		e.mu.Lock()
		tree, ok := e.trees[sha]
		e.mu.Unlock()
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		entries := make([]map[string]any, 0, len(tree))
		for p, b := range tree {
			entries = append(entries, map[string]any{"path": p, "type": "blob", "sha": b})
		}
		writeJSON(w, map[string]any{"sha": sha, "tree": entries, "truncated": false})
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/repos/octocat/hello/git/blobs/"):
		e.note("GET /git/blobs")
		sha := strings.TrimPrefix(path, "/repos/octocat/hello/git/blobs/")
		e.mu.Lock()
		content, ok := e.blobs[sha]
		e.mu.Unlock()
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		writeJSON(w, map[string]any{"sha": sha, "encoding": "base64", "content": base64.StdEncoding.EncodeToString(content)})
	case r.Method == http.MethodPost && path == "/repos/octocat/hello/git/blobs":
		e.note("POST /git/blobs")
		var body struct {
			Content string `json:"content"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		content, err := base64.StdEncoding.DecodeString(body.Content)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		sha := GitBlobSHA(content)
		e.mu.Lock()
		e.blobs[sha] = content
		e.mu.Unlock()
		writeJSON(w, map[string]any{"sha": sha})
	case r.Method == http.MethodPost && path == "/repos/octocat/hello/git/trees":
		e.note("POST /git/trees")
		var body struct {
			BaseTree string `json:"base_tree"`
			Tree     []struct {
				Path string `json:"path"`
				Mode string `json:"mode"`
				Type string `json:"type"`
				SHA  string `json:"sha"`
			} `json:"tree"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		e.mu.Lock()
		_, ok := e.trees[body.BaseTree]
		e.mu.Unlock()
		if !ok {
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}
		next := map[string]string{}
		e.mu.Lock()
		for k, v := range e.trees[body.BaseTree] {
			next[k] = v
		}
		for _, entry := range body.Tree {
			if entry.SHA == "" { // 删除项（sha 为 null）
				delete(next, entry.Path)
				continue
			}
			next[entry.Path] = entry.SHA
		}
		sha := fmt.Sprintf("tree-%d", len(e.trees)+1)
		e.trees[sha] = next
		e.mu.Unlock()
		writeJSON(w, map[string]any{"sha": sha})
	case r.Method == http.MethodPost && path == "/repos/octocat/hello/git/commits":
		e.note("POST /git/commits")
		var body struct {
			Message string   `json:"message"`
			Tree    string   `json:"tree"`
			Parents []string `json:"parents"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		sha := fmt.Sprintf("c%d", len(e.commits)+1)
		e.mu.Lock()
		e.commits[sha] = body.Tree
		e.commitParents[sha] = body.Parents
		e.mu.Unlock()
		writeJSON(w, map[string]any{"sha": sha})
	case r.Method == http.MethodPost && path == "/repos/octocat/hello/git/refs":
		e.note("POST /git/refs")
		var body struct {
			Ref string `json:"ref"`
			SHA string `json:"sha"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if !strings.HasPrefix(body.Ref, "refs/heads/") || strings.HasSuffix(body.Ref, "refs/heads/main") || strings.HasSuffix(body.Ref, "refs/heads/prod") {
			e.violation("illegal ref write: " + body.Ref)
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}
		e.mu.Lock()
		_, shaExists := e.commits[body.SHA]
		_, refExists := e.refs[body.Ref]
		e.mu.Unlock()
		if !shaExists { // 真实 GitHub 同语义：目标对象不存在 → 422
			w.WriteHeader(http.StatusUnprocessableEntity)
			_ = json.NewEncoder(w).Encode(map[string]any{"message": "Object does not exist"})
			return
		}
		if refExists {
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}
		e.mu.Lock()
		e.refs[body.Ref] = body.SHA
		if e.blackoutAfterRef {
			e.blackout = true
		}
		e.mu.Unlock()
		writeJSON(w, map[string]any{"ref": body.Ref, "object": map[string]any{"sha": body.SHA}})
	case r.Method == http.MethodPatch && strings.HasPrefix(path, "/repos/octocat/hello/git/refs/heads/"):
		e.note("PATCH /git/refs")
		branch := strings.TrimPrefix(path, "/repos/octocat/hello/git/refs/heads/")
		if branch == "main" || branch == "prod" {
			e.violation("protected ref update: " + branch)
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}
		var body struct {
			SHA   string `json:"sha"`
			Force bool   `json:"force"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		e.mu.Lock()
		current, refOK := e.refs["refs/heads/"+branch]
		_, shaOK := e.commits[body.SHA]
		e.mu.Unlock()
		if !refOK { // 真实 GitHub 同语义：ref 不存在 → 422（不得以 PATCH 掩盖 create 根因）
			w.WriteHeader(http.StatusUnprocessableEntity)
			_ = json.NewEncoder(w).Encode(map[string]any{"message": "Reference does not exist"})
			return
		}
		if !shaOK {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_ = json.NewEncoder(w).Encode(map[string]any{"message": "Object does not exist"})
			return
		}
		// 真实 GitHub 同语义：force:false 的 ref 更新必须 fast-forward——
		// 分支现 head 必须在新 sha 的祖先链上，否则 422。该校验是「同任务
		// 分支二次交付必须以现 head 为 parent」的契约钉子（此前模拟器无
		// 条件接受任何 PATCH，掩盖了非 ff 迭代在真实 GitHub 必败）。
		if !body.Force && body.SHA != current && !e.isAncestor(current, body.SHA) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_ = json.NewEncoder(w).Encode(map[string]any{"message": "Update is not a fast forward"})
			return
		}
		e.mu.Lock()
		e.refs["refs/heads/"+branch] = body.SHA
		e.mu.Unlock()
		writeJSON(w, map[string]any{"ref": "refs/heads/" + branch, "object": map[string]any{"sha": body.SHA}})
	case r.Method == http.MethodGet && path == "/repos/octocat/hello/pulls":
		e.note("GET /pulls")
		head := r.URL.Query().Get("head")
		e.mu.Lock()
		for _, pr := range e.prs {
			if pr.Head == head {
				e.mu.Unlock()
				writeJSON(w, []map[string]any{{"number": pr.Number, "html_url": prURL(pr.Number), "draft": pr.Draft, "state": pr.State}})
				return
			}
		}
		e.mu.Unlock()
		writeJSON(w, []map[string]any{})
	case r.Method == http.MethodPost && path == "/repos/octocat/hello/pulls":
		e.note("POST /pulls")
		if failPR {
			e.mu.Lock()
			e.failPR = false
			e.mu.Unlock()
			w.WriteHeader(http.StatusUnprocessableEntity)
			_ = json.NewEncoder(w).Encode(map[string]any{"message": "validation failed"})
			return
		}
		var body struct {
			Title string `json:"title"`
			Head  string `json:"head"`
			Base  string `json:"base"`
			Draft *bool  `json:"draft"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Draft == nil || !*body.Draft {
			e.violation("non-draft pull request created")
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}
		e.mu.Lock()
		e.nextPR++
		pr := emulatorPR{Number: e.nextPR, Title: body.Title, Head: body.Head, Base: body.Base, Draft: true, State: "open"}
		e.prs = append(e.prs, pr)
		e.mu.Unlock()
		writeJSON(w, map[string]any{"number": pr.Number, "html_url": prURL(pr.Number), "draft": true})
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

// isAncestor 判定 ancestor 是否在 descendant 的提交祖先链上（模拟器内的
// fast-forward 判定；种子提交无 parents 记录时视为根）。
func (e *githubEmulator) isAncestor(ancestor, descendant string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	seen := map[string]bool{}
	stack := []string{descendant}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if cur == ancestor {
			return true
		}
		if seen[cur] {
			continue
		}
		seen[cur] = true
		stack = append(stack, e.commitParents[cur]...)
	}
	return false
}

func prURL(n int64) string { return fmt.Sprintf("https://github.com/octocat/hello/pull/%d", n) }

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func TestGitHubClientWireChainCreatesBranchAndDraftPR(t *testing.T) {
	e := newGitHubEmulator(t)
	factory := NewGitHubClientFactory(http.DefaultClient, e.srv.URL)
	ctx := context.Background()

	repo := RepoRef{Owner: "octocat", Name: "hello"}
	client := factory(e.token, repo) // 每次调用钉定 token + 仓库
	info, err := client.Repository(ctx)
	require.NoError(t, err)
	require.Equal(t, "main", info.DefaultBranch)

	baseTree, err := client.CommitTree(ctx, "b"+strings.Repeat("0", 39))
	require.NoError(t, err)
	require.Equal(t, "tree-baseline", baseTree)

	protected, err := client.BranchProtected(ctx, "prod")
	require.NoError(t, err)
	require.True(t, protected)

	tree, err := client.Tree(ctx, baseTree) // git/trees 端点只接受 tree sha（CommitTree 的返回值）
	require.NoError(t, err)
	require.Len(t, tree, 2)

	blob, err := client.Blob(ctx, tree["main.go"])
	require.NoError(t, err)
	require.Equal(t, "package main\n", string(blob))

	newSHA, err := client.CreateBlob(ctx, []byte("package main\n\nfunc main() {}\n"))
	require.NoError(t, err)
	treeSHA, err := client.CreateTree(ctx, "tree-baseline", []TreeEntry{{Path: "main.go", SHA: newSHA}})
	require.NoError(t, err)
	commitSHA, err := client.CreateCommit(ctx, "b"+strings.Repeat("0", 39), treeSHA, "fix: greeting")
	require.NoError(t, err)

	branch := TaskBranchOf("s-1")
	require.NoError(t, client.EnsureBranch(ctx, branch, commitSHA))
	got, ok := e.BranchCommit(branch)
	require.True(t, ok)
	require.Equal(t, commitSHA, got)

	receipt, err := client.DraftPullRequest(ctx, PullRequestInput{Title: "WeKnora task s-1", Head: "octocat:" + branch, Base: "main"})
	require.NoError(t, err)
	require.True(t, receipt.Draft)
	require.True(t, receipt.Created)
	require.EqualValues(t, 1, receipt.Number)

	login, err := client.CurrentLogin(ctx)
	require.NoError(t, err)
	require.Equal(t, "octocat", login)

	// 已存在同 head 的 PR：复用（Created=false），不重复建。
	again, err := client.DraftPullRequest(ctx, PullRequestInput{Title: "WeKnora task s-1", Head: "octocat:" + branch, Base: "main"})
	require.NoError(t, err)
	require.False(t, again.Created)
	require.EqualValues(t, 1, again.Number)

	require.Empty(t, e.Violations())
	require.Zero(t, e.Calls()["PUT /pulls/merge"])
}

func TestGitHubClientBranchHeadTreatsOnlyNotFoundAsMissing(t *testing.T) {
	e := newGitHubEmulator(t)
	client := NewGitHubClientFactory(http.DefaultClient, e.srv.URL)(e.token, RepoRef{Owner: "octocat", Name: "hello"})

	sha, exists, err := client.BranchHead(context.Background(), "missing")
	require.NoError(t, err)
	require.False(t, exists)
	require.Empty(t, sha)

	unauthorized := NewGitHubClientFactory(http.DefaultClient, e.srv.URL)("invalid", RepoRef{Owner: "octocat", Name: "hello"})
	sha, exists, err = unauthorized.BranchHead(context.Background(), "missing")
	require.Error(t, err)
	require.Empty(t, sha)
	require.False(t, exists)
	var apiErr *GitHubAPIError
	require.ErrorAs(t, err, &apiErr)
	require.Equal(t, http.StatusUnauthorized, apiErr.Status)
}

func TestGitHubClientBranchProtectedTreatsOnlyNotFoundAsUnprotected(t *testing.T) {
	e := newGitHubEmulator(t)
	client := NewGitHubClientFactory(http.DefaultClient, e.srv.URL)(e.token, RepoRef{Owner: "octocat", Name: "hello"})

	protected, err := client.BranchProtected(context.Background(), "missing")
	require.NoError(t, err)
	require.False(t, protected)

	unauthorized := NewGitHubClientFactory(http.DefaultClient, e.srv.URL)("invalid", RepoRef{Owner: "octocat", Name: "hello"})
	protected, err = unauthorized.BranchProtected(context.Background(), "missing")
	require.Error(t, err)
	require.False(t, protected)
	var apiErr *GitHubAPIError
	require.ErrorAs(t, err, &apiErr)
	require.Equal(t, http.StatusUnauthorized, apiErr.Status)
}

// TestGitHubClientCreateTreeDeleteEntryUsesEmptySHA 把 TreeEntry 的删除语义
// 钉死为契约证据：SHA=="" 在 wire 层编码为 null sha，目标 path 从结果树消失、
// 其余 entry 保留。这是 Task 2 权威形态（TreeEntry{Path,SHA,Mode}，SHA==""=
// 删除）的回归守卫——brief Interfaces 段的 TreeEntry{Path,SHA,Deleted bool}
// 是内部矛盾笔误，若 Task 5/6 按其编码将编译失败而非静默漂移（控制器裁决材料
// 见 .superpowers/sdd/plan-t52/task-2-report.md）。
func TestGitHubClientCreateTreeDeleteEntryUsesEmptySHA(t *testing.T) {
	e := newGitHubEmulator(t)
	factory := NewGitHubClientFactory(http.DefaultClient, e.srv.URL)
	client := factory(e.token, RepoRef{Owner: "octocat", Name: "hello"})
	ctx := context.Background()

	treeSHA, err := client.CreateTree(ctx, "tree-baseline", []TreeEntry{{Path: "README.md"}})
	require.NoError(t, err)
	tree, err := client.Tree(ctx, treeSHA)
	require.NoError(t, err)
	require.NotContains(t, tree, "README.md")
	require.Contains(t, tree, "main.go")
}

func TestGitHubClientClassifiesDefiniteVsUnobservable(t *testing.T) {
	e := newGitHubEmulator(t)
	factory := NewGitHubClientFactory(http.DefaultClient, e.srv.URL)
	ctx := context.Background()

	// 确定性 404：GitHubAPIError（status 携带），不是 transport。
	_, err := factory(e.token, RepoRef{Owner: "octocat", Name: "hello"}).Tree(ctx, "0123456789012345678901234567890123456789")
	var apiErr *GitHubAPIError
	require.ErrorAs(t, err, &apiErr)
	require.Equal(t, http.StatusNotFound, apiErr.Status)

	// 坏令牌：确定性 401。
	_, err = factory("gho_wrong", RepoRef{Owner: "octocat", Name: "hello"}).Repository(ctx)
	require.ErrorAs(t, err, &apiErr)
	require.Equal(t, http.StatusUnauthorized, apiErr.Status)

	// 网络不可达：ErrGitHubTransport（不可观测）。
	closed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	closed.Close()
	_, err = NewGitHubClientFactory(http.DefaultClient, closed.URL)(e.token, RepoRef{Owner: "octocat", Name: "hello"}).Repository(ctx)
	require.ErrorIs(t, err, ErrGitHubTransport)
}

// 同任务分支迭代（真实 GitHub 语义回归，最终修复轮发现 1/5）：PATCH 现在
// 按 fast-forward 校验——以分支现 head 为 parent 的提交能顶上去；以
// BaselineSHA 为 parent 的「平行」提交被 422 拒绝且 head 不动；create 422
// 但分支不存在时根因原样浮出（不再被注定失败的 PATCH 遮蔽）。
func TestGitHubClientEnsureBranchFastForwardAndRootCause(t *testing.T) {
	e := newGitHubEmulator(t)
	factory := NewGitHubClientFactory(http.DefaultClient, e.srv.URL)
	ctx := context.Background()
	client := factory(e.token, RepoRef{Owner: "octocat", Name: "hello"})
	baseline := "b" + strings.Repeat("0", 39)
	branch := TaskBranchOf("s-ff")

	c1, err := client.CreateCommit(ctx, baseline, "tree-baseline", "one")
	require.NoError(t, err)
	require.NoError(t, client.EnsureBranch(ctx, branch, c1), "首次交付：分支不存在 → create")
	got, ok := e.BranchCommit(branch)
	require.True(t, ok)
	require.Equal(t, c1, got)

	// 平行提交（parent=baseline，非分支现 head）：create 422（已存在）→
	// PATCH 被模拟器 fast-forward 校验拒绝——真实 GitHub 同样 422。旧实现
	// 的 PATCH 无条件接受该更新，此断言在其上必失败（契约钉子）。
	parallel, err := client.CreateCommit(ctx, baseline, "tree-baseline", "parallel")
	require.NoError(t, err)
	err = client.EnsureBranch(ctx, branch, parallel)
	var apiErr *GitHubAPIError
	require.ErrorAs(t, err, &apiErr, "非 ff 的 ref 更新必须被拒")
	require.Equal(t, http.StatusUnprocessableEntity, apiErr.Status)
	got, _ = e.BranchCommit(branch)
	require.Equal(t, c1, got, "被拒后分支 head 不动")

	// ff 迭代：parent=现 head 的第二颗提交成功顶上去（二次交付的正确形态）。
	c2, err := client.CreateCommit(ctx, c1, "tree-baseline", "two")
	require.NoError(t, err)
	require.NoError(t, client.EnsureBranch(ctx, branch, c2))
	got, _ = e.BranchCommit(branch)
	require.Equal(t, c2, got)

	// create 422 但分支并不存在（sha 非法 → "Object does not exist"）：
	// 根因原样浮出，不被 PATCH 的 "Reference does not exist" 遮蔽。
	err = client.EnsureBranch(ctx, TaskBranchOf("s-fresh"), "dead"+strings.Repeat("0", 36))
	require.ErrorAs(t, err, &apiErr)
	require.Equal(t, "Object does not exist", apiErr.Message)
	_, ok = e.BranchCommit(TaskBranchOf("s-fresh"))
	require.False(t, ok)
}
