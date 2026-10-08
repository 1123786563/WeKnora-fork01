package container

// T20 (#139) T03 deferred-item assembly: the production construction point
// of the two Docker command faces with the SAME T03 uploaded-material
// execution policy (and the production audit sink) attached BEFORE they are
// exposed. The sandbox ROUTE that serves these faces is the T19 S3
// identity-propagation item and stays default-off — until it lands nothing
// in-process calls these faces; this factory is the exact seam that route
// consumes, and the assembly test re-runs the gate journey through THIS
// construction path (member-visible refusal, zero engine touches, audit row
// through the real audit service).

import (
	"context"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/sandbox"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"gorm.io/gorm"
)

// registeredCraftWebBuildCommandGate holds the T04 (#123) fixed offline
// web-build command gate assembled at the runtime site: the future command
// dispatch face (the T19 S3 route / T04 entry) consumes it through
// RegisteredCraftWebBuildCommandGate before any server-initiated build
// command is sent. nil (the default) leaves dispatch fail-closed per the
// gate's own contract.
var registeredCraftWebBuildCommandGate *CraftWebBuildCommandGate

// RegisterCraftWebBuildCommandGate installs the deployment's build-command
// gate (assembled once, next to the toolchain pin).
func RegisterCraftWebBuildCommandGate(gate *CraftWebBuildCommandGate) {
	registeredCraftWebBuildCommandGate = gate
}

// RegisteredCraftWebBuildCommandGate returns the registered gate (nil when
// the deployment pins no toolchain).
func RegisteredCraftWebBuildCommandGate() *CraftWebBuildCommandGate {
	return registeredCraftWebBuildCommandGate
}

// craftDockerWorkspaceRoot is the container-side workspace root the T03
// lexical layers resolve the read-only inputs tree against (the same root
// the T04 web-build gate uses at the runtime assembly site).
const craftDockerWorkspaceRoot = "/workspace"

// CraftDockerExecCommandFaces bundles the two T03-gated Docker command
// faces sharing ONE policy instance (exposed for seams like the T04
// web-build command gate that mount the SAME gate at their dispatch point).
type CraftDockerExecCommandFaces struct {
	Normal     *service.CraftDockerNormalExecService
	Restricted *service.CraftDockerRestrictedExec
	Policy     *service.CraftDelegateExecutionPolicy
}

// craftDockerNormalOutputQuota is the per-request durable output quota the
// normal face enforces on top of any provider-side limit — derived from the
// SAME validation ceiling the request path accepts
// (repository.MaxCraftDockerOutputBytes): a quota below the ceiling would
// silently truncate requests the validation contract called legal.
const craftDockerNormalOutputQuota = repository.MaxCraftDockerOutputBytes

// The two provider seams below mirror the service constructors' unexported
// parameter interfaces with exported structural types: the production
// Docker clients and the assembly-test fakes both satisfy them (identical,
// fully-exported method sets), and interface-to-interface assignment into
// the unexported constructor parameters works exactly because the method
// sets agree.
type craftDockerNormalEngine interface {
	CreateAttachedExec(context.Context, sandbox.RemoteSandboxHandle, sandbox.DockerNormalExecRequest) (sandbox.DockerNormalExecReceipt, error)
	StartAttachedExecOnce(context.Context, sandbox.DockerNormalExecReceipt, bool, []byte, sandbox.DockerNormalExecChunkSink) (sandbox.DockerNormalExecOutcome, error)
	ObserveAttachedExec(context.Context, sandbox.DockerNormalExecReceipt, bool) (sandbox.DockerNormalExecObservation, error)
}

type craftDockerOutputlessEngine = sandbox.DockerOutputlessExecClient

// newCraftDockerExecCommandFaces is the T03 production assembly: the send
// coordinator over the REAL budget service and claim repository, the output
// projection over the durable output store, and BOTH faces screened by the
// SAME delegate execution policy — the delegate service carries the
// production audit sink (material-policy rows) and the policy adapter
// carries it too (denial rows). WithExecutionPolicy logs the injection, and
// the assembly logs its own record so a deployment that failed to wire the
// security gate is observable in its logs.
func newCraftDockerExecCommandFaces(
	db *gorm.DB,
	store craft.Store,
	budget *service.CraftBudgetService,
	audit interfaces.AuditLogService,
	normalProvider craftDockerNormalEngine,
	outputless craftDockerOutputlessEngine,
	rpcTimeout time.Duration,
) (*CraftDockerExecCommandFaces, error) {
	coordinator, err := service.NewCraftDockerSendCoordinator(budget, repository.NewCraftDockerSendClaimRepository(db))
	if err != nil {
		return nil, err
	}
	output, err := service.NewCraftDockerOutputService(repository.NewCraftDockerOutputRepository(db, craftDockerNormalOutputQuota))
	if err != nil {
		return nil, err
	}
	delegate := service.NewCraftDelegateService(store, nil).WithAuditLog(audit)
	policy, err := service.NewCraftDelegateExecutionPolicy(delegate, db, craftDockerWorkspaceRoot)
	if err != nil {
		return nil, err
	}
	policy.WithAuditLog(audit)
	normal, err := service.NewCraftDockerNormalExecService(coordinator,
		repository.NewCraftDockerNormalInputRepository(db), normalProvider, output)
	if err != nil {
		return nil, err
	}
	restricted, err := service.NewCraftDockerRestrictedExec(coordinator, outputless, rpcTimeout)
	if err != nil {
		return nil, err
	}
	normal.WithExecutionPolicy(policy)
	restricted.WithExecutionPolicy(policy)
	logger.Infof(context.Background(),
		"[CraftDockerExec] T03 production assembly: normal+restricted command faces constructed with the uploaded-material execution gate (workspace root %s)", craftDockerWorkspaceRoot)
	return &CraftDockerExecCommandFaces{Normal: normal, Restricted: restricted, Policy: policy}, nil
}
