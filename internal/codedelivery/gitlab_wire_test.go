package codedelivery

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"

	appconnectorsvc "github.com/Tencent/WeKnora/internal/appconnector/service/appconnector"
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
	// lastStartBranch/lastActions 捕获最近一次 commits POST 的锚定与动作
	// 面（R5-F11 的断言用）。
	lastStartBranch string
	lastActions     []glCommitAction
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

// addMR 注入一条开放 MR（绕过 createMR 的同 source 冲突闸——真实 GitLab
// 允许同 source 对不同 target 的多条开放 MR，这正是 R5-F8 的平台语义）。
func (e *gitLabEmulator) addMR(source, target string) int64 {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.nextMR++
	e.mrs = append(e.mrs, gitLabMR{IID: e.nextMR, Title: "Draft: injected", Source: source, Target: target, State: "opened"})
	return e.nextMR
}

// deleteBranch 删除分支（MR 保持开放——真实 GitLab 在 source 分支被删后
// MR 仍开放，只是不可合并；对账面这是「分支缺席、MR 残留」的形态）。
func (e *gitLabEmulator) deleteBranch(name string) {
	e.mu.Lock()
	delete(e.branches, name)
	e.mu.Unlock()
}

// advanceDefaultTip 在默认分支 tip 之上直接落一颗「外部协作者」提交
// （模拟基线锚定之后 main 上的推进：新增/修改文件——R5-F11 的现场）。
func (e *gitLabEmulator) advanceDefaultTip(changes map[string][]byte) {
	e.mu.Lock()
	defer e.mu.Unlock()
	old := e.branches[e.defaultBranch]
	tree := map[string]string{}
	for p, b := range e.trees[old] {
		tree[p] = b
	}
	for p, c := range changes {
		sha := GitBlobSHA(c)
		e.blobs[sha] = c
		tree[p] = sha
	}
	sha := fmt.Sprintf("ext-%d", len(e.commits)+1)
	e.commits[sha] = []string{old}
	e.trees[sha] = tree
	e.branches[e.defaultBranch] = sha
}

// LastStartBranch 返回最近一次 commits POST 携带的 start_branch（空 = 未
// 携带/分支已存在）。
func (e *gitLabEmulator) LastStartBranch() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.lastStartBranch
}

// LastActions 返回最近一次 commits POST 的 actions 列表副本。
func (e *gitLabEmulator) LastActions() []glCommitAction {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]glCommitAction(nil), e.lastActions...)
}

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
		target := r.URL.Query().Get("target_branch")
		perPage := 20 // real GitLab default
		if v, err := strconv.Atoi(r.URL.Query().Get("per_page")); err == nil && v > 0 && v <= 100 {
			perPage = v
		}
		page := 1
		if v, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && v > 0 {
			page = v
		}
		e.mu.Lock()
		filtered := []gitLabMR{}
		for _, mr := range e.mrs {
			if (source == "" || mr.Source == source) && (state == "" || mr.State == state) && (target == "" || mr.Target == target) {
				filtered = append(filtered, mr)
			}
		}
		start := (page - 1) * perPage
		if start > len(filtered) {
			start = len(filtered)
		}
		end := start + perPage
		if end > len(filtered) {
			end = len(filtered)
		}
		out := []map[string]any{}
		for _, mr := range filtered[start:end] {
			out = append(out, map[string]any{"iid": mr.IID, "web_url": mrURL(mr.IID), "title": mr.Title, "state": mr.State, "target_branch": mr.Target})
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
		// 真实 GitLab 的 start_branch 接受 branch/tag/commit SHA（ref 解析）；
		// 此处已持锁，按 resolveRef 同语义内联解析。
		s, ok := "", false
		if sha, found := e.branches[body.StartBranch]; found {
			s, ok = sha, true
		} else if _, found := e.trees[body.StartBranch]; found {
			s, ok = body.StartBranch, true
		}
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			writeJSON(w, map[string]any{"message": "start_branch not found"})
			return
		}
		start = s
	}
	e.lastStartBranch = body.StartBranch
	e.lastActions = append([]glCommitAction(nil), body.Actions...)
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

// TestGitLabRawRejectsBodyOverCap pins R5-F9: a 2xx body larger than the raw
// cap must surface as a transport-family ERROR naming the cap — never as
// silently truncated bytes that the baseline materializer would write into
// the session workspace.
func TestGitLabRawRejectsBodyOverCap(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		chunk := make([]byte, 64*1024)
		remaining := maxRawBodyBytes + 1
		for remaining > 0 {
			n := len(chunk)
			if remaining < n {
				n = remaining
			}
			if _, err := w.Write(chunk[:n]); err != nil {
				return
			}
			remaining -= n
		}
	}))
	t.Cleanup(srv.Close)
	client := NewGitLabClientFactory(http.DefaultClient, srv.URL)("glpat-testtoken", RepoRef{Owner: "octocat", Name: "hello"})

	var raw []byte
	raw, err := client.Blob(context.Background(), strings.Repeat("0", 40))
	require.Error(t, err, "an over-cap body must be refused, not returned truncated")
	require.ErrorIs(t, err, ErrCodeTransport)
	require.Contains(t, err.Error(), "exceeds", "the error must name the cap")
	require.Contains(t, err.Error(), fmt.Sprintf("%d", maxRawBodyBytes))
	require.Nil(t, raw)
}

// TestGitLabBlobVerifiesContentAddressedSHA pins R5-F9's second layer: the
// raw blob endpoint is content-addressed, so the body must hash to the
// requested sha before it is trusted. Mismatched bytes (a truncated or
// misrouted read) are a transport-family error; matching bytes pass.
func TestGitLabBlobVerifiesContentAddressedSHA(t *testing.T) {
	content := []byte("package main\n\nfunc main() {}\n")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(content)
	}))
	t.Cleanup(srv.Close)
	client := NewGitLabClientFactory(http.DefaultClient, srv.URL)("glpat-testtoken", RepoRef{Owner: "octocat", Name: "hello"})

	got, err := client.Blob(context.Background(), GitBlobSHA(content))
	require.NoError(t, err, "a body matching the requested sha must pass")
	require.Equal(t, content, got)

	_, err = client.Blob(context.Background(), strings.Repeat("f", 40))
	require.Error(t, err, "a body that does not hash to the requested sha must be refused")
	require.ErrorIs(t, err, ErrCodeTransport)
	require.Contains(t, err.Error(), "content-addressed")
}

// TestPullRequestForHeadMatchesTargetBranch pins R5-F8: GitLab allows
// several open MRs from the same source branch to DIFFERENT targets, so
// head alone is not an identity. The lookup must resolve by source+target
// — a foreign-target MR must never be hit, and a base with no MR must
// return (nil, nil) rather than an unrelated reuse.
func TestPullRequestForHeadMatchesTargetBranch(t *testing.T) {
	e := newGitLabEmulator(t)
	head := "octocat:weknora/task/s-1"
	e.addMR("weknora/task/s-1", "release-1") // foreign target, listed FIRST
	mainIID := e.addMR("weknora/task/s-1", "main")
	client := NewGitLabClientFactory(http.DefaultClient, e.srv.URL)("glpat-testtoken", RepoRef{Owner: "octocat", Name: "hello"})

	got, err := client.PullRequestForHead(context.Background(), head, "main")
	require.NoError(t, err)
	require.NotNil(t, got, "the MR targeting main must be hit")
	require.EqualValues(t, mainIID, got.Number, "the foreign-target MR must never be returned")

	got, err = client.PullRequestForHead(context.Background(), head, "feature-x")
	require.NoError(t, err)
	require.Nil(t, got, "no open MR targets feature-x — (nil, nil), not an unrelated reuse")
}

// TestPullRequestForHeadPaginates pins R5-F8's pagination half: the MR
// listing pages per_page=100 until a short page (cap 50). The fake here
// IGNORES the target_branch query filter (modeling a proxy that drops
// unknown params), so the client must page through a full first page of
// foreign-target MRs and find the match on the short second page by the
// response's own target_branch field — the double-insurance compare.
func TestPullRequestForHeadPaginates(t *testing.T) {
	type mrItem struct {
		IID    int64  `json:"iid"`
		Title  string `json:"title"`
		State  string `json:"state"`
		Target string `json:"target_branch"`
	}
	head := "weknora/task/s-1"
	// 100 foreign-target MRs (full page 1) + the main-target MR first on
	// page 2 (short page).
	page1 := make([]mrItem, 100)
	for i := range page1 {
		page1[i] = mrItem{IID: int64(i + 1), Title: "Draft: x", State: "opened", Target: fmt.Sprintf("release-%d", i)}
	}
	page2 := []mrItem{{IID: 101, Title: "Draft: ours", State: "opened", Target: "main"}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("per_page") != "100" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		switch r.URL.Query().Get("page") {
		case "1", "":
			writeJSON(w, page1)
		case "2":
			writeJSON(w, page2)
		default:
			writeJSON(w, []mrItem{})
		}
	}))
	t.Cleanup(srv.Close)
	client := NewGitLabClientFactory(http.DefaultClient, srv.URL)("glpat-testtoken", RepoRef{Owner: "octocat", Name: "hello"})

	got, err := client.PullRequestForHead(context.Background(), "octocat:"+head, "main")
	require.NoError(t, err)
	require.NotNil(t, got, "the match on the second (short) page must be found")
	require.EqualValues(t, 101, got.Number)
}

// TestCallClassifies2xxDecodeFailureAsServerFault pins R5-F10: a 2xx reply
// carrying undecodable JSON is a server/protocol fault on a response that
// DID arrive — never ErrCodeRequestInvalid, which is reserved for requests
// that never left the process.
func TestCallClassifies2xxDecodeFailureAsServerFault(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`this is { not json`))
	}))
	t.Cleanup(srv.Close)
	client := NewGitLabClientFactory(http.DefaultClient, srv.URL)("glpat-testtoken", RepoRef{Owner: "octocat", Name: "hello"})

	_, err := client.CurrentLogin(context.Background())
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrCodeRequestInvalid, "a 2xx decode failure is not a request-invalid (never-sent) outcome")
	require.ErrorIs(t, err, ErrCodeTransport, "nearest existing family: an untrustworthy exchange, not a provable pre-send refusal")
}

// TestEnsureBranchAnchorsNewBranchAtBaseline pins R5-F11: a NEW task branch
// must grow from the APPROVED BASELINE ref (the GitHub chain's
// parent := material.BaselineSHA semantics), not the default branch's
// current tip. Anchoring at the default tip would converge the branch to
// the default↔baseline increment — deleting a collaborator's brand-new
// file X and rolling Y back — and misattribute the blown action cap to
// ErrBaselineTooLarge.
func TestEnsureBranchAnchorsNewBranchAtBaseline(t *testing.T) {
	e := newGitLabEmulator(t)
	baseline := "b" + strings.Repeat("0", 39)
	// The default branch moved past the baseline AFTER the plan anchored:
	// collaborator added X.txt and edited main.go on main.
	e.advanceDefaultTip(map[string][]byte{
		"X.txt":   []byte("external addition\n"),
		"main.go": []byte("package main // external edit\n"),
	})
	client := NewGitLabClientFactory(http.DefaultClient, e.srv.URL)("glpat-testtoken", RepoRef{Owner: "octocat", Name: "hello"})
	ctx := context.Background()

	ours := []byte("package main\n\nfunc main() {}\n")
	blobSHA, err := client.CreateBlob(ctx, ours)
	require.NoError(t, err)
	staged, err := client.CreateTree(ctx, baseline, []TreeEntry{{Path: "main.go", SHA: blobSHA}})
	require.NoError(t, err)
	placeholder, err := client.CreateCommit(ctx, baseline, staged, "fix: greeting")
	require.NoError(t, err)

	branch := "weknora/task/s-anchor"
	require.NoError(t, client.EnsureBranch(ctx, branch, placeholder))

	// The new branch is anchored at the BASELINE ref, not the default branch.
	require.Equal(t, baseline, e.LastStartBranch(), "start_branch must be the baseline ref, not the default branch name")
	commit, ok := e.BranchCommit(branch)
	require.True(t, ok)
	require.Equal(t, []string{baseline}, e.ParentOf(commit), "the new branch must fork from the baseline commit")
	// Actions cover ONLY the staged change: no delete of X, no rollback of Y.
	actions := e.LastActions()
	require.Len(t, actions, 1, "actions must be exactly the staged change, not the default↔baseline increment")
	require.Equal(t, "main.go", actions[0].FilePath)
	// Converged tree = baseline + staged only.
	tip := e.BranchTree(branch)
	require.Len(t, tip, 2)
	require.Equal(t, GitBlobSHA(ours), tip["main.go"])
	require.Equal(t, GitBlobSHA([]byte("# hello\n")), tip["README.md"])
	require.Empty(t, e.Violations())
}

// TestEnsureBranchEmptyConvergenceIsDispatchNotStarted pins R5-F13: an
// empty convergence on a missing branch is rejected BEFORE the commits API
// leaves the process (no branch, no MR — provably nothing started), so it
// must carry ErrDispatchNotStarted and settle FAILED, not park unknown
// where QueryProvider could never find a remote fact.
func TestEnsureBranchEmptyConvergenceIsDispatchNotStarted(t *testing.T) {
	e := newGitLabEmulator(t)
	baseline := "b" + strings.Repeat("0", 39)
	client := NewGitLabClientFactory(http.DefaultClient, e.srv.URL)("glpat-testtoken", RepoRef{Owner: "octocat", Name: "hello"})
	ctx := context.Background()

	staged, err := client.CreateTree(ctx, baseline, nil) // no mutations: intended == baseline
	require.NoError(t, err)
	placeholder, err := client.CreateCommit(ctx, baseline, staged, "empty")
	require.NoError(t, err)

	err = client.EnsureBranch(ctx, "weknora/task/s-empty", placeholder)
	require.ErrorIs(t, err, appconnectorsvc.ErrDispatchNotStarted, "a proven pre-send local rejection must carry the not-started contract")
	require.Zero(t, e.Calls()["POST /repository/commits"], "nothing may leave")
}
