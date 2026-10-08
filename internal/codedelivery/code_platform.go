package codedelivery

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	appconnector "github.com/Tencent/WeKnora/internal/appconnector"
)

// —— 统一 Delivery seam 的代码平台中立词汇（T24 #54）——
// 交付链（基线、diff、审批、任务分支、草稿 PR/MR）只认本文件的中立形状；
// 平台差异被各适配器隐藏。GitHub 先定义了端口形状，GitLab 经类型别名适配
// 同一端口：唯一的平台 switch 是 clientForPlatform，其余文件零平台分支。

// CodePlatformClient is the provider-neutral outbound port of one delivery.
type CodePlatformClient = GitHubClient

// CodePlatformClientFactory binds one token + one repo to a platform client.
type CodePlatformClientFactory = GitHubClientFactory

// CodePlatformAPIError is a definite provider refusal with an HTTP status.
type CodePlatformAPIError = GitHubAPIError

// The classification (definite vs unobservable) is provider-neutral; only the
// historical names carry "GitHub". Same values keep every existing
// errors.Is mapping intact.
var (
	ErrCodeTransport      = ErrGitHubTransport
	ErrCodeRequestInvalid = ErrGitHubRequestInvalid
)

// Provider vocabulary. The provider of a connection is authoritative on its
// installation row (installations.app_id) — never client input.
const (
	ProviderGitHub = "github"
	ProviderGitLab = "gitlab"
)

// ErrUnsupportedProvider: the connection's app is not a code platform (or the
// adapter is not wired). Every consumer fails closed before any remote call.
var ErrUnsupportedProvider = errors.New("code_delivery_unsupported_provider")

// DeliveryTargetOf maps a provider to its A03 action target.
func DeliveryTargetOf(provider string) string { return provider + ".deliver" }

// ProviderOfTarget parses an A03 action target back to its provider.
func ProviderOfTarget(target string) (string, error) {
	rest, ok := strings.CutSuffix(target, ".deliver")
	if !ok || rest == "" {
		return "", fmt.Errorf("%w: %q is not a delivery target", ErrUnsupportedProvider, target)
	}
	return rest, nil
}

// ProviderSource resolves the code platform behind a connection: the app id
// of the installation the connection belongs to. Production:
// *appconnectorrepo.InstallationStore (GetInstallationByID, tenant-scoped).
type ProviderSource interface {
	GetInstallationByID(ctx context.Context, tenantID uint64, installationID string) (appconnector.Installation, error)
}

// clientForPlatform picks the platform adapter behind the unified seam. Both
// the service (prepare face) and the dispatcher (dispatch face) route through
// this one switch — no other file may branch on a provider name.
func clientForPlatform(gitHub, gitLab CodePlatformClientFactory, provider, token string, repo RepoRef) (CodePlatformClient, error) {
	switch provider {
	case ProviderGitHub:
		if gitHub == nil {
			return nil, fmt.Errorf("%w: github adapter not wired", ErrUnsupportedProvider)
		}
		return gitHub(token, repo), nil
	case ProviderGitLab:
		if gitLab == nil {
			return nil, fmt.Errorf("%w: gitlab adapter not wired", ErrUnsupportedProvider)
		}
		return gitLab(token, repo), nil
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedProvider, provider)
	}
}

// draftMRPrefix is GitLab's draft marker (title prefix; GitHub uses a draft
// boolean — the adapter maps, the chain never sees the difference). "WIP: "
// is GitLab's legacy draft marker and counts as already-draft.
const draftMRPrefix = "Draft: "

// DraftMRTitle marks a title as draft MR, idempotently.
func DraftMRTitle(title string) string {
	if strings.HasPrefix(title, draftMRPrefix) || strings.HasPrefix(title, "WIP: ") {
		return title
	}
	return draftMRPrefix + title
}

// ProtectedBranchGlobMatch evaluates a GitLab protected-branch pattern
// (wildcards * and ?) against a branch name. GitLab protects by exact name or
// wildcard; GitHub reports a boolean per branch — the adapter hides that by
// matching the platform's declared patterns itself. "*" spans "/" (GitLab's
// wildcard is broad); over-matching only ever REFUSES more, the safe
// direction for a guardrail.
func ProtectedBranchGlobMatch(pattern, branch string) bool {
	if pattern == "" {
		return false
	}
	var b strings.Builder
	b.WriteString(`^`)
	for _, c := range pattern {
		switch c {
		case '*':
			b.WriteString(`.*`)
		case '?':
			b.WriteString(`.`)
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	b.WriteString(`$`)
	matched, err := regexp.MatchString(b.String(), branch)
	return err == nil && matched
}
