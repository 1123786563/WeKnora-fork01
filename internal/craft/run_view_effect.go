package craft

import "context"

// RunViewEffectKind names one durable permission to mutate a RunView runtime.
// Allocation is recorded with the admitted Run transaction; provider effects
// are claimed separately immediately before dispatch.
type RunViewEffectKind string

const (
	RunViewEffectAllocate            RunViewEffectKind = "allocate"
	RunViewEffectDockerNetworkCreate RunViewEffectKind = "docker_network_create"
	RunViewEffectDockerCreate        RunViewEffectKind = "docker_create"
	RunViewEffectDockerStart         RunViewEffectKind = "docker_start"
	RunViewEffectDockerProbe         RunViewEffectKind = "docker_probe"
	RunViewEffectOpenCodeCreate      RunViewEffectKind = "opencode_create"
	// RunViewEffectWebBuild (F08 T-1) claims one fixed offline web build
	// dispatch; it always binds the canonical staged request digest.
	RunViewEffectWebBuild RunViewEffectKind = "web_build"
)

// RunViewEffectClaim is an opaque capability minted by the server after the
// current Run fence and immutable admission identity have been checked.
// Callers must never construct or substitute Token values.
type RunViewEffectClaim struct {
	Key        RunViewKey
	Generation string
	Kind       RunViewEffectKind
	// RequestDigest binds a claim to the complete canonical physical request.
	// The caller owns canonicalization and domain separation; the authority
	// persists and compares the lowercase SHA-256 digest before granting a send.
	RequestDigest string
	Token         string
}

// RunViewEffectOutcome records the observed result of one claimed effect.
// Unknown is durable and unresolved; it can never grant a replacement send.
type RunViewEffectOutcome struct {
	State   RunViewEffectState
	Receipt string
}

type RunViewEffectState string

const (
	RunViewEffectStatePending   RunViewEffectState = "pending"
	RunViewEffectStateSucceeded RunViewEffectState = "succeeded"
	RunViewEffectStateFailed    RunViewEffectState = "failed"
	RunViewEffectStateUnknown   RunViewEffectState = "unknown"
)

// RunViewEffectAuthority is the server-owned writer authority for RunView
// allocation and mutating runtime effects. It consumes the original admitted
// Task, including its independent snapshot digest and Run Fence. The legacy
// RunViewStore.Allocate key-only method is not an effect authorization.
type RunViewEffectAuthority interface {
	AllocateAdmitted(context.Context, Task) (RunView, error)
	BeginEffect(context.Context, Task, string, RunViewEffectKind) (RunViewEffectClaim, bool, error)
	FinishEffect(context.Context, RunViewEffectClaim, RunViewEffectOutcome) error
}

// RunViewEffectRequestAuthority is the additive seam for effect kinds that
// must bind a durable claim to the exact canonical provider request. The
// request digest is a lowercase SHA-256 hex value supplied by the caller.
type RunViewEffectRequestAuthority interface {
	BeginEffectWithDigest(context.Context, Task, string, RunViewEffectKind, string) (RunViewEffectClaim, bool, error)
}
