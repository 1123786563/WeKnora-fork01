package codedelivery

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// gitLabEmulator 是一个内存 GitLab REST v4（T24 #54）：真实 HTTP 字节进出
// 生产适配器，blobs 以真实 git blob sha 入库（GitLab 同为内容寻址 git 对象
// 库），commits API 按「在分支现 tip（或 start_branch tip）上应用 actions、
// 服务端自定提交 sha 与祖先链」语义推进。记录每类调用次数与违规（merge
// 尝试、非草稿 MR、保护分支写、未知 action）。cutAfterCommit 在 commits
// POST 成功后立即断连，模拟推送落地后的传输不可观测。
type gitLabEmulator struct {
	t              *testing.T
	srv            *httptest.Server
	mu             sync.Mutex
	token          string
	calls          map[string]int
	bad            []string
	proj           string // escaped path segment："octocat%2Fhello"
	defaultBranch  string
	protected      []string // 精确名或通配模式（GitLab 语义）
	blobs          map[string][]byte
	trees          map[string]map[string]string // commit sha → path→blob sha
	commits        map[string][]string          // commit sha → parent shas
	branches       map[string]string            // branch → commit sha
	mrs            []gitLabMR
	nextMR         int64
	failMR         bool
	cutAfterCommit bool
	blackout       bool
}

type gitLabMR struct {
	IID    int64
	Title  string
	Source string
	Target string
	State  string
}

func newGitLabEmulator(t *testing.T) *gitLabEmulator {
	e := &gitLabEmulator{
		t: t, token: "glpat-testtoken", calls: map[string]int{},
		proj:          "octocat%2Fhello",
		defaultBranch: "main",
		protected:     []string{"stable-*"},
		blobs:         map[string][]byte{},
		trees:         map[string]map[string]string{},
		commits:       map[string][]string{},
		branches:      map[string]string{},
		mrs:           []gitLabMR{},
	}
	baseline := "b" + strings.Repeat("0", 39)
	seed := map[string][]byte{
		"README.md": []byte("# hello\n"),
		"main.go":   []byte("package main\n"),
	}
	tree := map[string]string{}
	for p, c := range seed {
		sha := GitBlobSHA(c)
		e.blobs[sha] = c
		tree[p] = sha
	}
	e.trees[baseline] = tree
	e.commits[baseline] = []string{}
	e.branches["main"] = baseline
	mux := http.NewServeMux()
	mux.HandleFunc("/", e.serve)
	e.srv = httptest.NewServer(mux)
	t.Cleanup(e.srv.Close)
	return e
}

func (e *gitLabEmulator) note(call string) { e.mu.Lock(); e.calls[call]++; e.mu.Unlock() }
func (e *gitLabEmulator) violation(v string) {
	e.mu.Lock()
	e.bad = append(e.bad, v)
	e.mu.Unlock()
}
func (e *gitLabEmulator) Calls() map[string]int { return e.calls }
func (e *gitLabEmulator) Violations() []string  { return e.bad }
func (e *gitLabEmulator) BranchCommit(branch string) (string, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	sha, ok := e.branches[branch]
	return sha, ok
}

// BranchTree 投影分支 tip 树（path→blob sha）——「收敛不变量」断言用。
func (e *gitLabEmulator) BranchTree(branch string) map[string]string {
	e.mu.Lock()
	defer e.mu.Unlock()
	sha, ok := e.branches[branch]
	if !ok {
		return nil
	}
	out := map[string]string{}
	for p, b := range e.trees[sha] {
		out[p] = b
	}
	return out
}

// ParentOf 返回提交的父链（GitLab commits API 由服务端决定祖先的断言用）。
func (e *gitLabEmulator) ParentOf(sha string) []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]string(nil), e.commits[sha]...)
}

func (e *gitLabEmulator) protectPattern(pattern string) {
	e.mu.Lock()
	e.protected = append(e.protected, pattern)
	e.mu.Unlock()
}
func (e *gitLabEmulator) failNextMRCreation() { e.mu.Lock(); e.failMR = true; e.mu.Unlock() }

// blackoutAfterCommitCreate：下一条 commits POST 成功落地后立即断连——
// 推送已发生、结果不可观测（与 githubEmulator 的 blackoutAfterRefCreate
// 同语义；「after」由 refs/commits 路由内的置位实现）。
func (e *gitLabEmulator) blackoutAfterCommitCreate() {
	e.mu.Lock()
	e.cutAfterCommit = true
	e.mu.Unlock()
}
func (e *gitLabEmulator) liftBlackout() { e.mu.Lock(); e.blackout = false; e.mu.Unlock() }

// resolveRef 解析 ref（分支名优先，其次提交 sha）。
func (e *gitLabEmulator) resolveRef(ref string) (string, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if sha, ok := e.branches[ref]; ok {
		return sha, true
	}
	if _, ok := e.trees[ref]; ok {
		return ref, true
	}
	return "", false
}

func (e *gitLabEmulator) branchHead(branch string) (string, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	sha, ok := e.branches[branch]
	return sha, ok
}

func (e *gitLabEmulator) serve(w http.ResponseWriter, r *http.Request) {
	e.mu.Lock()
	blackout := e.blackout
	failMR := e.failMR
	e.mu.Unlock()
	if blackout {
		// 连接直接断开：客户端拿到 transport 错误（不可观测）。
		panic(http.ErrAbortHandler)
	}
	if got := r.Header.Get("Authorization"); got != "Bearer "+e.token {
		w.WriteHeader(http.StatusUnauthorized)
		writeJSON(w, map[string]any{"message": "invalid_token"})
		return
	}
	// 结构性护栏：merge 尝试是违规并被拒绝（真实端点 PUT …/merge_requests/:iid/merge
	// 以 "/merge" 结尾；"/merge_requests" 不误伤）。
	if r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/merge") {
		e.violation("merge attempted: " + r.Method + " " + r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	escaped := r.URL.EscapedPath()
	projPrefix := "/api/v4/projects/" + e.proj
	switch {
	case r.Method == http.MethodGet && escaped == "/api/v4/user":
		e.note("GET /user")
		writeJSON(w, map[string]any{"username": "gl-user"})
	case r.Method == http.MethodGet && escaped == projPrefix:
		e.note("GET /project")
		writeJSON(w, map[string]any{"default_branch": e.defaultBranch, "path_with_namespace": "octocat/hello"})
	case r.Method == http.MethodGet && escaped == projPrefix+"/protected_branches":
		e.note("GET /protected_branches")
		out := []map[string]any{}
		for _, p := range e.protected {
			out = append(out, map[string]any{"name": p})
		}
		w.Header().Set("X-Next-Page", "")
		writeJSON(w, out)
	case r.Method == http.MethodGet && strings.HasPrefix(escaped, projPrefix+"/repository/tree"):
		e.note("GET /repository/tree")
		ref := r.URL.Query().Get("ref")
		commit, ok := e.resolveRef(ref)
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			writeJSON(w, map[string]any{"message": "404 ref " + ref + " not found"})
			return
		}
		e.mu.Lock()
		entries := []map[string]any{}
		for p, sha := range e.trees[commit] {
			entries = append(entries, map[string]any{"id": sha, "type": "blob", "path": p, "mode": "100644"})
		}
		e.mu.Unlock()
		w.Header().Set("X-Next-Page", "")
		writeJSON(w, entries)
	case r.Method == http.MethodGet && strings.HasPrefix(escaped, projPrefix+"/repository/blobs/"):
		e.note("GET /repository/blobs raw")
		rest := strings.TrimPrefix(escaped, projPrefix+"/repository/blobs/")
		if !strings.HasSuffix(rest, "/raw") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		sha := strings.TrimSuffix(rest, "/raw")
		e.mu.Lock()
		content, ok := e.blobs[sha]
		e.mu.Unlock()
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			writeJSON(w, map[string]any{"message": "blob not found"})
			return
		}
		_, _ = w.Write(content)
	case r.Method == http.MethodGet && strings.HasPrefix(escaped, projPrefix+"/repository/branches/"):
		e.note("GET /repository/branches")
		branch, err := url.PathUnescape(strings.TrimPrefix(escaped, projPrefix+"/repository/branches/"))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		sha, ok := e.branchHead(branch)
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			writeJSON(w, map[string]any{"message": "404 branch not found"})
			return
		}
		writeJSON(w, map[string]any{"name": branch, "commit": map[string]any{"id": sha}})
	case r.Method == http.MethodPost && escaped == projPrefix+"/repository/commits":
		e.note("POST /repository/commits")
		e.commitOnBranch(w, r)
	case r.Method == http.MethodGet && escaped == projPrefix+"/merge_requests":
		e.note("GET /merge_requests")
		source := r.URL.Query().Get("source_branch")
		state := r.URL.Query().Get("state")
		e.mu.Lock()
		out := []map[string]any{}
		for _, mr := range e.mrs {
			if (source == "" || mr.Source == source) && (state == "" || mr.State == state) {
				out = append(out, map[string]any{"iid": mr.IID, "web_url": mrURL(mr.IID), "title": mr.Title, "state": mr.State})
			}
		}
		e.mu.Unlock()
		writeJSON(w, out)
	case r.Method == http.MethodPost && escaped == projPrefix+"/merge_requests":
		e.note("POST /merge_requests")
		e.createMR(w, r, failMR)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

type glCommitAction struct {
	Action   string `json:"action"`
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
}

func (e *gitLabEmulator) commitOnBranch(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Branch        string           `json:"branch"`
		StartBranch   string           `json:"start_branch"`
		CommitMessage string           `json:"commit_message"`
		Actions       []glCommitAction `json:"actions"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	e.mu.Lock()
	defer e.mu.Unlock()
	if body.Branch == e.defaultBranch {
		e.bad = append(e.bad, "protected ref write: "+body.Branch)
		w.WriteHeader(http.StatusForbidden)
		return
	}
	for _, pattern := range e.protected {
		if ProtectedBranchGlobMatch(pattern, body.Branch) {
			e.bad = append(e.bad, "protected ref write: "+body.Branch)
			w.WriteHeader(http.StatusForbidden)
			return
		}
	}
	tip, exists := e.branches[body.Branch]
	start := tip
	if !exists {
		if body.StartBranch == "" {
			w.WriteHeader(http.StatusBadRequest)
			writeJSON(w, map[string]any{"message": "branch does not exist; start_branch required"})
			return
		}
		s, ok := e.branches[body.StartBranch]
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			writeJSON(w, map[string]any{"message": "start_branch not found"})
			return
		}
		start = s
	}
	if len(body.Actions) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(w, map[string]any{"message": "ensure at least one action"})
		return
	}
	tree := map[string]string{}
	for p, b := range e.trees[start] {
		tree[p] = b
	}
	for _, act := range body.Actions {
		switch act.Action {
		case "create", "update":
			var content []byte
			var err error
			if act.Encoding == "base64" {
				content, err = base64.StdEncoding.DecodeString(act.Content)
				if err != nil {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
			} else {
				content = []byte(act.Content)
			}
			sha := GitBlobSHA(content)
			e.blobs[sha] = content
			tree[act.FilePath] = sha
		case "delete":
			delete(tree, act.FilePath)
		default:
			e.bad = append(e.bad, "unknown commit action: "+act.Action)
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}
	}
	sha := fmt.Sprintf("glc-%d", len(e.commits)+1)
	e.commits[sha] = []string{start}
	e.trees[sha] = tree
	e.branches[body.Branch] = sha
	if e.cutAfterCommit {
		e.blackout = true
	}
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, map[string]any{"id": sha})
}

func (e *gitLabEmulator) createMR(w http.ResponseWriter, r *http.Request, failMR bool) {
	var body struct {
		SourceBranch string `json:"source_branch"`
		TargetBranch string `json:"target_branch"`
		Title        string `json:"title"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	e.mu.Lock()
	defer e.mu.Unlock()
	if failMR {
		e.failMR = false
		w.WriteHeader(http.StatusConflict)
		writeJSON(w, map[string]any{"message": "merge request validation failed"})
		return
	}
	// GitLab 以标题前缀标记草稿 MR：非草稿标题是违规（差异必须由适配器隐藏）。
	if !strings.HasPrefix(body.Title, "Draft: ") {
		e.bad = append(e.bad, "non-draft merge request: "+body.Title)
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}
	for _, mr := range e.mrs {
		if mr.Source == body.SourceBranch && mr.State == "opened" {
			w.WriteHeader(http.StatusConflict)
			writeJSON(w, map[string]any{"message": "an open merge request already exists for this source branch"})
			return
		}
	}
	e.nextMR++
	mr := gitLabMR{IID: e.nextMR, Title: body.Title, Source: body.SourceBranch, Target: body.TargetBranch, State: "opened"}
	e.mrs = append(e.mrs, mr)
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, map[string]any{"iid": mr.IID, "web_url": mrURL(mr.IID), "title": mr.Title, "state": mr.State})
}

func mrURL(iid int64) string {
	return fmt.Sprintf("https://gitlab.com/octocat/hello/-/merge_requests/%d", iid)
}

func TestGitLabClientWireChainConvergesBranchAndDraftMR(t *testing.T) {
	e := newGitLabEmulator(t)
	factory := NewGitLabClientFactory(http.DefaultClient, e.srv.URL)
	ctx := context.Background()

	repo := RepoRef{Owner: "octocat", Name: "hello"}
	client := factory("glpat-testtoken", repo) // 每次调用钉定 token + 仓库
	info, err := client.Repository(ctx)
	require.NoError(t, err)
	require.Equal(t, "main", info.DefaultBranch)

	// GitLab 保护分支是「精确名或通配模式」：通配匹配由适配器本地完成。
	protected, err := client.BranchProtected(ctx, "stable-x")
	require.NoError(t, err)
	require.True(t, protected, "GitLab 通配保护模式必须由适配器匹配")
	protected, err = client.BranchProtected(ctx, "weknora/task/s-1")
	require.NoError(t, err)
	require.False(t, protected)

	// CommitTree→Tree 折叠为一次树读（GitLab 的 Tree 直接接受 ref）。
	baseline := "b" + strings.Repeat("0", 39)
	baseRef, err := client.CommitTree(ctx, baseline)
	require.NoError(t, err)
	require.Equal(t, baseline, baseRef)
	tree, err := client.Tree(ctx, baseRef)
	require.NoError(t, err)
	require.Len(t, tree, 2)
	blob, err := client.Blob(ctx, tree["main.go"])
	require.NoError(t, err)
	require.Equal(t, "package main\n", string(blob))

	newContent := []byte("package main\n\nfunc main() {}\n")
	newSHA, err := client.CreateBlob(ctx, newContent)
	require.NoError(t, err)
	require.Equal(t, GitBlobSHA(newContent), newSHA, "blob sha 内容寻址，与 GitLab 存储同构")
	staged, err := client.CreateTree(ctx, baseline, []TreeEntry{{Path: "main.go", SHA: newSHA}})
	require.NoError(t, err)
	require.NotEmpty(t, staged)
	placeholder, err := client.CreateCommit(ctx, baseline, staged, "fix: greeting")
	require.NoError(t, err)
	require.NotEmpty(t, placeholder)

	branch := TaskBranchOf("s-1")
	require.NoError(t, client.EnsureBranch(ctx, branch, placeholder))
	sha, ok := e.BranchCommit(branch)
	require.True(t, ok)
	require.NotEqual(t, placeholder, sha, "GitLab 服务端自定提交 sha，占位值不得外泄")
	tip := e.BranchTree(branch)
	require.Equal(t, GitBlobSHA(newContent), tip["main.go"], "任务分支收敛到基线树+变更")
	require.Equal(t, GitBlobSHA([]byte("# hello\n")), tip["README.md"], "基线未变文件保留")
	require.Equal(t, []string{baseline}, e.ParentOf(sha), "新分支从 start_branch tip 分叉")

	// 草稿 MR：owner:branch 前缀被适配器剥离为裸 source_branch。
	receipt, err := client.DraftPullRequest(ctx, PullRequestInput{Title: "WeKnora task s-1", Head: "octocat:" + branch, Base: "main"})
	require.NoError(t, err)
	require.True(t, receipt.Draft)
	require.True(t, receipt.Created)
	require.EqualValues(t, 1, receipt.Number)
	require.Contains(t, receipt.URL, "/-/merge_requests/1")

	login, err := client.CurrentLogin(ctx)
	require.NoError(t, err)
	require.Equal(t, "gl-user", login)

	// 已有同 source 的开放 MR：复用（Created=false），不重复开。
	again, err := client.DraftPullRequest(ctx, PullRequestInput{Title: "WeKnora task s-1", Head: "octocat:" + branch, Base: "main"})
	require.NoError(t, err)
	require.False(t, again.Created)
	require.EqualValues(t, 1, again.Number)

	require.Empty(t, e.Violations())
}

func TestGitLabClientClassifiesDefiniteVsUnobservable(t *testing.T) {
	e := newGitLabEmulator(t)
	factory := NewGitLabClientFactory(http.DefaultClient, e.srv.URL)
	ctx := context.Background()

	// 确定性 404：CodePlatformAPIError（status 携带）。
	_, err := factory("glpat-testtoken", RepoRef{Owner: "octocat", Name: "hello"}).Tree(ctx, "missing-ref")
	var apiErr *CodePlatformAPIError
	require.ErrorAs(t, err, &apiErr)
	require.Equal(t, http.StatusNotFound, apiErr.Status)

	// 坏令牌：确定性 401。
	_, err = factory("glpat-wrong", RepoRef{Owner: "octocat", Name: "hello"}).Repository(ctx)
	require.ErrorAs(t, err, &apiErr)
	require.Equal(t, http.StatusUnauthorized, apiErr.Status)

	// 网络不可达：ErrCodeTransport（不可观测）。
	closed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	closed.Close()
	_, err = NewGitLabClientFactory(http.DefaultClient, closed.URL)("glpat-testtoken", RepoRef{Owner: "octocat", Name: "hello"}).Repository(ctx)
	require.ErrorIs(t, err, ErrCodeTransport)
}

func TestDraftMRTitleAndProtectedGlob(t *testing.T) {
	require.Equal(t, "Draft: fix", DraftMRTitle("fix"))
	require.Equal(t, "Draft: fix", DraftMRTitle("Draft: fix"), "前缀必须幂等")
	require.True(t, ProtectedBranchGlobMatch("stable-*", "stable-x"))
	require.True(t, ProtectedBranchGlobMatch("release/*", "release/1.0/x"))
	require.False(t, ProtectedBranchGlobMatch("stable-*", "main"))
	require.True(t, ProtectedBranchGlobMatch("ma?n", "main"))
	require.False(t, ProtectedBranchGlobMatch("", "main"))
	provider, err := ProviderOfTarget("gitlab.deliver")
	require.NoError(t, err)
	require.Equal(t, "gitlab", provider)
	_, err = ProviderOfTarget("feishu.send")
	require.ErrorIs(t, err, ErrUnsupportedProvider)
	require.Equal(t, "gitlab.deliver", DeliveryTargetOf("gitlab"))
}
