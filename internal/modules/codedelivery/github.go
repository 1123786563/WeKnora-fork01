package codedelivery

import (
	"context"
	"errors"
	"net/http"
	"time"
)

// GitHubAPIBaseURL is the reviewed production endpoint. Tests inject an
// httptest URL; the factory never derives hosts from model output.
const GitHubAPIBaseURL = "https://api.github.com"

// ErrGitHubTransport marks an UNOBSERVABLE remote outcome (dial/timeout/EOF):
// the request may or may not have reached GitHub. Callers must park unknown,
// never blind-retry. A GitHub RESPONSE — any status — is a definite outcome
// and surfaces as *GitHubAPIError instead.
var ErrGitHubTransport = errors.New("github_transport_unobservable")

// ErrGitHubRequestInvalid marks a request that could not even be CONSTRUCTED
// (unusable method/URL): nothing ever left the process, so the outcome is
// provably not-sent. It must not masquerade as ErrGitHubTransport — a build
// failure is our own malformed request, not an unobservable provider outage
// (final-fix round: error classification).
var ErrGitHubRequestInvalid = errors.New("github_request_invalid")

// GitHubAPIError is a definite provider refusal with an HTTP status.
type GitHubAPIError struct {
	Status   int
	Endpoint string
	Message  string
}

func (e *GitHubAPIError) Error() string {
	return "github api " + e.Endpoint + ": status " + itoa(e.Status) + ": " + e.Message
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	digits := []byte{}
	for v > 0 {
		digits = append([]byte{byte('0' + v%10)}, digits...)
		v /= 10
	}
	return string(digits)
}

// GitHubRepoInfo is the repository metadata the guardrail needs.
type GitHubRepoInfo struct{ DefaultBranch string }

// TreeEntry is one tree mutation: set a path to a blob sha, or delete it.
// Authoritative shape is {Path, SHA, Mode} with SHA=="" meaning delete — the
// plan's Interfaces line ("Deleted bool") is an internal brief contradiction
// (T22 #52 task 2 review round 1); deletion is signaled by the empty SHA,
// encoded as a null wire sha. Downstream tasks must use this shape.
type TreeEntry struct {
	Path string
	SHA  string // "" = delete (wire: null sha)
	Mode string // "100644" default
}

// PullRequestInput names the draft PR to create/reuse.
type PullRequestInput struct {
	Title, Head, Base string
}

// PullRequestReceipt is the PR traceability receipt.
type PullRequestReceipt struct {
	Number  int64
	URL     string
	Draft   bool
	Created bool
}

// GitHubClient is the outbound code-platform port of one delivery token.
// There is deliberately NO merge method: auto-merge is structurally absent
// (spec: "It does not merge automatically").
type GitHubClient interface {
	Repository(ctx context.Context) (GitHubRepoInfo, error)
	BranchProtected(ctx context.Context, branch string) (bool, error)
	Tree(ctx context.Context, sha string) (map[string]string, error)
	CommitTree(ctx context.Context, commitSHA string) (string, error)
	Blob(ctx context.Context, sha string) ([]byte, error)
	CreateBlob(ctx context.Context, content []byte) (string, error)
	CreateTree(ctx context.Context, baseTree string, entries []TreeEntry) (string, error)
	CreateCommit(ctx context.Context, parent, tree, message string) (string, error)
	BranchHead(ctx context.Context, branch string) (string, bool, error)
	EnsureBranch(ctx context.Context, branch, commit string) error
	DraftPullRequest(ctx context.Context, input PullRequestInput) (PullRequestReceipt, error)
	// PullRequestForHead resolves the open PR/MR for one head AND one target
	// base. Head alone is not an identity on every platform: GitLab allows
	// several open MRs from the same source branch to different targets, so
	// the base dimension is part of the port (R5-F8).
	PullRequestForHead(ctx context.Context, head, base string) (*PullRequestReceipt, error)
	CurrentLogin(ctx context.Context) (string, error)
}

// GitHubClientFactory binds one token + one repository to a client: the
// caller pins the repo per call from the approved material/input, so REST
// paths are always derived from a reviewed RepoRef. The service resolves the
// token through CredentialResolver and immediately scopes it here — the
// token never escapes this port.
type GitHubClientFactory func(token string, repo RepoRef) GitHubClient

func httpClientDefault() *http.Client {
	return &http.Client{Timeout: 30 * time.Second}
}
