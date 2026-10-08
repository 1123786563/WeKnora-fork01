// Package codedelivery owns the developer delivery chain (T22 #52):
// anchoring a run's workspace diff against a fixed GitHub baseline into an
// A03-approved action, and delivering the approved material as a task
// branch push + draft PR — never a protected-branch write, never a merge.
package codedelivery

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// DeliveryState is the delivery lifecycle owned by this module. The A03
// action row keeps the approval lifecycle; this state records the remote
// effect, including the PARTIAL completion push-succeeded-PR-failed.
type DeliveryState string

const (
	DeliveryPrepared   DeliveryState = "prepared"   // 材料已锚定为 A03 action，等待审批
	DeliveryDispatched DeliveryState = "dispatched" // 推送链路进行中
	DeliveryPushed     DeliveryState = "pushed"     // 部分完成：任务分支已推、PR 未成（恢复只补 PR）
	DeliveryDelivered  DeliveryState = "delivered"  // 草稿 PR 已创建/更新
	DeliveryFailed     DeliveryState = "failed"
	DeliveryUnknown    DeliveryState = "unknown" // 远端结果不可观测，只以远端事实收敛
)

var (
	ErrProtectedBranch    = errors.New("code_delivery_protected_branch")
	ErrInvalidBranch      = errors.New("code_delivery_invalid_branch")
	ErrInvalidMaterial    = errors.New("code_delivery_invalid_material")
	ErrBaselineTooLarge   = errors.New("code_delivery_baseline_too_large")
	ErrInvalidBaselineSHA = errors.New("code_delivery_invalid_baseline_sha")
	ErrRepoRefInvalid     = errors.New("code_delivery_repo_ref_invalid")
)

// TaskBranchPrefix is the whitelist prefix of every branch this module may
// create or update. refs outside heads/<prefix>… are structurally unreachable.
const TaskBranchPrefix = "weknora/task/"

// MaxDeliveryFiles bounds one delivery material (baseline + workspace union).
const MaxDeliveryFiles = 500

var taskBranchSuffix = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,120}$`)

// TaskBranchOf derives the deterministic task branch for a task (session) id.
func TaskBranchOf(taskID string) string { return TaskBranchPrefix + taskID }

// ValidateTaskBranch enforces the prefix whitelist and a conservative refname
// charset (no spaces, no "..", no leading "-", bounded length). ".." is
// rejected explicitly because RE2 has no negative lookahead and the whitelist
// charset alone admits a literal dot.
func ValidateTaskBranch(branch string) error {
	if !strings.HasPrefix(branch, TaskBranchPrefix) {
		return fmt.Errorf("%w: %q lacks prefix %q", ErrInvalidBranch, branch, TaskBranchPrefix)
	}
	suffix := strings.TrimPrefix(branch, TaskBranchPrefix)
	if suffix == "" || !taskBranchSuffix.MatchString(suffix) || strings.Contains(suffix, "..") {
		return fmt.Errorf("%w: illegal task branch suffix", ErrInvalidBranch)
	}
	return nil
}

// branchShapeLegal is the pure-local refname shape gate: non-empty, bounded
// length (git's own loose-ref ceiling), no "..", printable refname charset
// only. It runs BEFORE any provider round-trip (final-fix round: a branch
// string that is not even a legal refname must not spend a remote read nor
// reach URL construction). The deliberate AC1 order is untouched — any
// well-formed branch (including "main") still hits RefuseProtectedTarget
// before the prefix whitelist; only genuinely malformed strings are refused
// locally.
func branchShapeLegal(branch string) bool {
	if branch == "" || len(branch) > 255 {
		return false
	}
	if strings.Contains(branch, "..") {
		return false
	}
	if branch[0] == '/' || branch[len(branch)-1] == '/' {
		return false
	}
	for i := 0; i < len(branch); i++ {
		c := branch[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case c == '.' || c == '_' || c == '-' || c == '/':
		default:
			return false
		}
	}
	return true
}

// RefuseProtectedTarget is the AC1 guardrail: the delivery target must differ
// from the repository default branch and must not be marked protected by the
// code platform.
func RefuseProtectedTarget(branch, defaultBranch string, protected bool) error {
	if defaultBranch != "" && branch == defaultBranch {
		return fmt.Errorf("%w: target branch is the repository default branch", ErrProtectedBranch)
	}
	if protected {
		return fmt.Errorf("%w: target branch is protected on the code platform", ErrProtectedBranch)
	}
	return nil
}

// RepoRef names one GitHub repository.
type RepoRef struct{ Owner, Name string }

func (r RepoRef) String() string { return r.Owner + "/" + r.Name }

var repoRefPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

// ParseRepoRef accepts exactly "owner/name" (single slash, no scheme).
// Any segment carrying ".." is refused so WorkspaceRepoRoot can never
// concatenate its way out of the fixed /workspace root — the same guard
// task branch suffixes and delivery file paths enforce.
func ParseRepoRef(v string) (RepoRef, error) {
	v = strings.TrimSpace(v)
	if !repoRefPattern.MatchString(v) || strings.Contains(v, "..") {
		return RepoRef{}, fmt.Errorf("%w: %q", ErrRepoRefInvalid, v)
	}
	parts := strings.SplitN(v, "/", 2)
	return RepoRef{Owner: parts[0], Name: parts[1]}, nil
}

// WorkspaceRepoRoot is the fixed workspace root a baseline is materialized
// into and diffs are read from: /workspace/<owner>/<name>.
func WorkspaceRepoRoot(repo RepoRef) string { return "/workspace/" + repo.Owner + "/" + repo.Name }

// GitBlobSHA computes the git blob object id (sha1 of "blob <len>\0"+content).
// It equals what GitHub returns for CreateBlob of the same bytes, so local
// diffing against a baseline tree needs no git binary and no credential.
func GitBlobSHA(content []byte) string {
	h := sha1.New()
	fmt.Fprintf(h, "blob %d\x00", len(content))
	h.Write(content)
	return hex.EncodeToString(h.Sum(nil))
}

// FileChange is one entry of a delivery: a modified/added file, or a deletion.
type FileChange struct {
	Path    string `json:"path"`
	Deleted bool   `json:"deleted"`
}

// DiffAgainstBaseline returns the sorted changes between a baseline tree
// (path→blob sha) and a workspace projection (path→blob sha).
func DiffAgainstBaseline(baseline, workspace map[string]string) []FileChange {
	paths := make(map[string]bool, len(baseline)+len(workspace))
	for p := range baseline {
		paths[p] = true
	}
	for p := range workspace {
		paths[p] = true
	}
	ordered := make([]string, 0, len(paths))
	for p := range paths {
		ordered = append(ordered, p)
	}
	sort.Strings(ordered)
	out := make([]FileChange, 0, len(ordered))
	for _, p := range ordered {
		baseSHA, inBase := baseline[p]
		workSHA, inWork := workspace[p]
		switch {
		case inBase && !inWork:
			out = append(out, FileChange{Path: p, Deleted: true})
		case !inBase && inWork:
			out = append(out, FileChange{Path: p})
		case inBase && inWork && baseSHA != workSHA:
			out = append(out, FileChange{Path: p})
		}
	}
	return out
}

// DeliveryMaterial is the EXACT approved snapshot of one delivery — the A03
// action args. ParseDeliveryMaterial is the dispatch-side guard: the bytes
// approved are the bytes delivered, and a tampered or drifted snapshot
// (missing field, extra field, non-object) is refused, never subset-parsed.
type DeliveryMaterial struct {
	Repo          RepoRef      `json:"repo"`
	BaselineSHA   string       `json:"baseline_sha"`
	Branch        string       `json:"branch"`
	Files         []FileChange `json:"files"`
	CommitMessage string       `json:"commit_message"`
	PRTitle       string       `json:"pr_title"`
}

// CanonicalJSON marshals the material deterministically (struct field order,
// sorted file list) so the A03 digest always covers the same logical bytes.
func (m DeliveryMaterial) CanonicalJSON() (json.RawMessage, error) {
	sorted := make([]FileChange, len(m.Files))
	copy(sorted, m.Files)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Path < sorted[j].Path })
	m.Files = sorted
	raw, err := json.Marshal(m)
	return raw, err
}

var baselineSHAPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

// ParseDeliveryMaterial validates the approved snapshot exactly: all six
// fields present, no extras, legal repo/baseline/branch, bounded file list.
func ParseDeliveryMaterial(raw json.RawMessage) (DeliveryMaterial, error) {
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil || probe == nil {
		return DeliveryMaterial{}, fmt.Errorf("%w: not a JSON object", ErrInvalidMaterial)
	}
	if len(probe) != 6 {
		return DeliveryMaterial{}, fmt.Errorf("%w: snapshot must be exactly repo, baseline_sha, branch, files, commit_message, pr_title", ErrInvalidMaterial)
	}
	var m DeliveryMaterial
	if err := json.Unmarshal(raw, &m); err != nil {
		return DeliveryMaterial{}, fmt.Errorf("%w: %v", ErrInvalidMaterial, err)
	}
	if _, err := ParseRepoRef(m.Repo.String()); err != nil {
		return m, err
	}
	if !baselineSHAPattern.MatchString(m.BaselineSHA) {
		return m, fmt.Errorf("%w: baseline must be a 40-hex commit sha", ErrInvalidBaselineSHA)
	}
	if err := ValidateTaskBranch(m.Branch); err != nil {
		return m, err
	}
	if m.CommitMessage == "" || m.PRTitle == "" {
		return m, fmt.Errorf("%w: commit_message and pr_title are required", ErrInvalidMaterial)
	}
	if len(m.Files) > MaxDeliveryFiles {
		return m, fmt.Errorf("%w: %d files exceed cap %d", ErrBaselineTooLarge, len(m.Files), MaxDeliveryFiles)
	}
	for _, f := range m.Files {
		if f.Path == "" || strings.HasPrefix(f.Path, "/") || strings.Contains(f.Path, "..") {
			return m, fmt.Errorf("%w: illegal file path %q", ErrInvalidMaterial, f.Path)
		}
	}
	return m, nil
}
