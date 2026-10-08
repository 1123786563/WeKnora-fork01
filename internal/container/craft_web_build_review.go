package container

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/craft"
)

// T04 (#123) deferred item (ledger 2026-09-23-craft-107-ledger.md:533): the
// offline build COMMAND entry consumes the T03 uploaded-material execution
// policy (#122). CraftWebBuildCommandGate is that entry: every command
// dispatched in the name of the fixed offline web build must pass this review
// BEFORE it reaches any executor, and it enforces three independent layers:
//
//  1. the fixed command shape — only the pinned build program invocation the
//     craft-web-build skill stages (python3 /opt/craft/web/build.py with its
//     known flag set) may ride this entry, so the entry cannot be borrowed to
//     dispatch arbitrary commands;
//  2. the host-side symlink contract (ledger :530 — server-resolvable
//     surfaces evaluate links BEFORE screening): the toolchain directory is
//     re-verified against the pin and the build program's symlink-resolved
//     location must stay inside the pinned toolchain's own resolved
//     directory, away from the staged read-only inputs tree;
//  3. the SAME T03 gate the exec services enforce through WithExecutionPolicy
//     (service.CraftExecutionPolicyGate.ReviewNormalExec): the staged command
//     is screened against the admitted Run's frozen input manifest — lexically
//     and by byte identity — with the member-visible refusal on denial.
//
// The gate instance handed here is the same one attached to
// CraftDockerNormalExecService/CraftDockerRestrictedExec at assembly; its
// durable audit sink (WithAuditLog) is attached by the central assembly.
//
// ResolvedTargetPath note (round-2 update): the shared DTO NOW carries the
// optional additive fields ResolvedTargetPath/TargetSHA256 (omitempty), and
// CraftDelegateExecutionPolicy.ReviewNormalExec forwards both into the
// module policy's InputExecutionRequest. This entry STILL enforces the host
// facts here (layer 2) because its symlink layer keys on the CONTAINER
// workspace root while this gate walks HOST paths — the namespace difference
// is real. The dispatch face (T20 lane) must fill the two fields with
// HOST-side facts only; container-semantics callers leave them empty.

// craftWebBuildProgramPath is the pinned build program's container path baked
// into the runtime image (docker/craft/Dockerfile W1 layer).
const craftWebBuildProgramPath = "/opt/craft/web/build.py"

// craftWebBuildToolchainPath is the pinned toolchain directory inside the
// image; --toolchain may not name anywhere else.
const craftWebBuildToolchainPath = "/opt/craft/web"

// CraftWebBuildCommandGate reviews build-command dispatches for the fixed
// offline web toolchain. Assemble it once per deployment next to the toolchain
// pin; Review is per dispatch.
type CraftWebBuildCommandGate struct {
	warnInputsRootOnce sync.Once
	policy             service.CraftExecutionPolicyGate
	pin                CraftWebToolchainPin
	toolchainDir       string
	hostInputsRel      string // symlink-resolved staged inputs root, "" when unknown
}

// CraftWebBuildCommandOption configures one assembled command gate.
type CraftWebBuildCommandOption func(*CraftWebBuildCommandGate) error

// WithCraftWebHostInputsRoot names the HOST-side root of the staged read-only
// inputs tree (the RunView material layout). When set, the resolved toolchain
// root must never overlap it — a toolchain directory that is itself a link
// into the material tree is a deployment attack shape and refuses dispatch.
func WithCraftWebHostInputsRoot(root string) CraftWebBuildCommandOption {
	return func(g *CraftWebBuildCommandGate) error {
		resolved, err := filepath.EvalSymlinks(root)
		if err != nil {
			return fmt.Errorf("%w: craft web command gate host inputs root %q: %v", craft.ErrInvalidInput, root, err)
		}
		g.hostInputsRel = resolved
		return nil
	}
}

// NewCraftWebBuildCommandGate validates the toolchain directory against the
// deployment pin (bytes are re-verified by LoadCraftWebToolchainPin) and
// stores the resolved toolchain root. A drifted or unreachable toolchain
// fails assembly with ErrConflict — before any command could be reviewed.
func NewCraftWebBuildCommandGate(
	policy service.CraftExecutionPolicyGate,
	pin CraftWebToolchainPin,
	toolchainDir string,
	options ...CraftWebBuildCommandOption,
) (*CraftWebBuildCommandGate, error) {
	if strings.TrimSpace(pin.ToolchainDigest) == "" || strings.TrimSpace(pin.TemplateSHA256) == "" {
		return nil, fmt.Errorf("%w: craft web command gate requires the loaded toolchain pin", craft.ErrInvalidInput)
	}
	if strings.TrimSpace(toolchainDir) == "" {
		return nil, fmt.Errorf("%w: craft web command gate requires the toolchain directory", craft.ErrInvalidInput)
	}
	if _, err := filepath.EvalSymlinks(toolchainDir); err != nil {
		return nil, fmt.Errorf("%w: craft web command gate toolchain directory %q is unreachable: %v", craft.ErrConflict, toolchainDir, err)
	}
	loaded, err := LoadCraftWebToolchainPin(toolchainDir)
	if err != nil {
		return nil, fmt.Errorf("%w: craft web command gate toolchain verification: %v", craft.ErrConflict, err)
	}
	if loaded.ToolchainDigest != pin.ToolchainDigest || loaded.TemplateSHA256 != pin.TemplateSHA256 || loaded.TemplateVersion != pin.TemplateVersion {
		return nil, fmt.Errorf("%w: craft web command gate toolchain %s does not match the deployment pin", craft.ErrConflict, toolchainDir)
	}
	g := &CraftWebBuildCommandGate{policy: policy, pin: pin, toolchainDir: toolchainDir}
	for _, option := range options {
		if option == nil {
			continue
		}
		if err := option(g); err != nil {
			return nil, err
		}
	}
	return g, nil
}

// craftWebBuildValueFlags is the fixed value-taking flag vocabulary of the
// pinned build program (docker/craft/web/build.py argparse); --selftest is
// the one boolean flag and is handled inline below.
var craftWebBuildValueFlags = map[string]bool{
	"toolchain":      true,
	"input":          true,
	"output":         true,
	"runtime-digest": true,
}

// craftWebBuildCommandShapeOk reports whether the command is exactly the
// pinned build program invocation: python3 + the image build program + a
// known flag set, with --toolchain pinned to the image directory.
func craftWebBuildCommandShapeOk(command []string, expectedRuntimeDigest string) bool {
	// argv[0] is pinned to the EXACT interpreter name (never a path):
	// a same-named wrapper or symlink at any writable location
	// (/tmp/evil/python3, ./python3) must not borrow this entry, and
	// path.Base would happily accept all of them.
	if len(command) < 2 || command[0] != "python3" || command[1] != craftWebBuildProgramPath {
		return false
	}
	seen := make(map[string]bool, len(craftWebBuildValueFlags))
	for i := 2; i < len(command); {
		arg := command[i]
		if arg == "--selftest" {
			// Self-test writes to a temporary directory and is not the build
			// operation admitted through this server dispatch entry.
			return false
		}
		if !strings.HasPrefix(arg, "--") {
			return false
		}
		name, value, hasValue := strings.Cut(strings.TrimPrefix(arg, "--"), "=")
		if !craftWebBuildValueFlags[name] {
			return false
		}
		if seen[name] {
			return false
		}
		if !hasValue {
			i++
			if i >= len(command) {
				return false
			}
			value = command[i]
		}
		i++
		seen[name] = true
		if name == "toolchain" {
			if value != craftWebBuildToolchainPath {
				return false
			}
		}
		if name == "input" && !craftWebWorkspacePath(value) {
			return false
		}
		if name == "output" && value != "/workspace/output" {
			return false
		}
		if name == "runtime-digest" && (strings.TrimSpace(value) == "" || strings.ContainsAny(value, "\x00\r\n")) {
			return false
		}
		if name == "runtime-digest" && expectedRuntimeDigest != "" && value != expectedRuntimeDigest {
			return false
		}
	}
	// The fixed build requires each value exactly once. `argparse` otherwise
	// uses the last duplicate value, which could redirect input/output or
	// replace the runtime identity after this gate has screened the request.
	for name := range craftWebBuildValueFlags {
		if !seen[name] {
			return false
		}
	}
	return true
}

// craftWebWorkspacePath allows absolute data paths inside the container's
// workspace while refusing traversal, normalization aliases, and paths
// outside the mounted workspace. Output has a stricter fixed destination.
func craftWebWorkspacePath(value string) bool {
	if !filepath.IsAbs(value) || filepath.Clean(value) != value || strings.Contains(value, "\\") || value == "/workspace" || !strings.HasPrefix(value, "/workspace/") {
		return false
	}
	for _, component := range strings.Split(value, string(filepath.Separator)) {
		if component == ".." {
			return false
		}
	}
	return true
}

// craftWebShapeRefusal is the member-visible refusal for a foreign command
// shape: what the entry accepts and what to do instead.
func craftWebShapeRefusal(command []string) error {
	return fmt.Errorf("%w: craft web build command entry only dispatches the pinned offline build (python3 %s with its known flags); refusing: %s",
		craft.ErrForbidden, craftWebBuildProgramPath, strings.Join(command, " "))
}

// Review screens one build-command dispatch. A refusal is never silent: every
// layer logs the durable Run identity alongside its reason, and the T03
// gate's own refusal (with the "Allowed alternative" text) is passed through
// unchanged so the caller can project it into the member-facing response.
func (g *CraftWebBuildCommandGate) Review(ctx context.Context, request repository.CraftDockerNormalInputRequest) error {
	if g == nil {
		// Nil-receiver check FIRST: the warnInputsRootOnce field access below
		// would panic on a nil gate (an assembly failure returning nil is
		// exactly the case this refusal exists for).
		return fmt.Errorf("%w: craft web build command gate is not assembled", craft.ErrForbidden)
	}
	if g.hostInputsRel == "" {
		// The material-overlap layer is unwired (no WithCraftWebHostInputsRoot
		// at assembly): say so once per process instead of failing open in
		// silence — the operator can then wire the host root.
		g.warnInputsRootOnce.Do(func() {
			logger.Warnf(ctx, "[CraftWebBuildGate] host inputs root NOT configured: the staged-material overlap layer is inactive (pass WithCraftWebHostInputsRoot at assembly)")
		})
	}
	// PATH hijack: argv[0] is pinned to the bare name "python3", which the
	// EXECUTOR resolves through the container PATH — a request carrying a
	// PATH override plus a writable-dir wrapper of the same name would
	// borrow this entry. Refuse PATH overrides outright.
	for key := range request.Environment {
		if key == "PATH" {
			return fmt.Errorf("%w: the fixed web build entry does not accept a PATH override (argv[0] resolution hijack)", craft.ErrForbidden)
		}
	}
	if g.policy == nil {
		logger.Warnf(ctx, "[CraftWebBuild] command gate has NO T03 execution policy attached; refusing dispatch for run %s", request.RunID)
		return fmt.Errorf("%w: craft web build command entry is not wired to the uploaded-material execution policy; the dispatch is refused", craft.ErrForbidden)
	}
	if !craftWebBuildCommandShapeOk(request.Command, g.pin.RuntimeDigest) {
		logger.Warnf(ctx, "[CraftWebBuild] refusing foreign command shape for run %s: %s", request.RunID, strings.Join(request.Command, " "))
		return craftWebShapeRefusal(request.Command)
	}
	if err := g.verifyHostToolchain(ctx, request); err != nil {
		return err
	}
	if err := g.policy.ReviewNormalExec(ctx, request); err != nil {
		// The gate's refusal is already member-visible (Refusal text) and
		// audited by its own sink; this line keeps the denial non-silent on
		// the command-entry surface too.
		logger.Warnf(ctx, "[CraftWebBuild] T03 execution policy refused the build command for run %s: %v", request.RunID, err)
		return err
	}
	logger.Infof(ctx, "[CraftWebBuild] build command for run %s passed the T03 gate (toolchain %s)", request.RunID, g.pin.ToolchainDigest)
	return nil
}

// verifyHostToolchain enforces the host-side symlink contract BEFORE the
// policy screening: the pinned bytes are re-verified (assembly-time
// verification cannot prove review-time bytes), the build program's resolved
// location must stay inside the resolved toolchain directory, and the
// resolved toolchain must stay clear of the staged read-only inputs root.
func (g *CraftWebBuildCommandGate) verifyHostToolchain(ctx context.Context, request repository.CraftDockerNormalInputRequest) error {
	loaded, err := LoadCraftWebToolchainPin(g.toolchainDir)
	if err != nil || loaded.ToolchainDigest != g.pin.ToolchainDigest {
		logger.Warnf(ctx, "[CraftWebBuild] toolchain drifted from the deployment pin before screening run %s: %v", request.RunID, err)
		return fmt.Errorf("%w: craft web toolchain no longer matches the pinned bytes for run %s", craft.ErrConflict, request.RunID)
	}
	resolvedDir, err := filepath.EvalSymlinks(g.toolchainDir)
	if err != nil {
		logger.Warnf(ctx, "[CraftWebBuild] toolchain directory unresolvable before screening run %s: %v", request.RunID, err)
		return fmt.Errorf("%w: craft web toolchain directory is unresolvable for run %s", craft.ErrConflict, request.RunID)
	}
	resolvedBuild, err := filepath.EvalSymlinks(filepath.Join(g.toolchainDir, "build.py"))
	if err != nil {
		logger.Warnf(ctx, "[CraftWebBuild] build program unresolvable before screening run %s: %v", request.RunID, err)
		return fmt.Errorf("%w: craft web build program is unresolvable for run %s", craft.ErrConflict, request.RunID)
	}
	if rel, relErr := filepath.Rel(resolvedDir, resolvedBuild); relErr != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		logger.Warnf(ctx, "[CraftWebBuild] build program resolves outside the pinned toolchain for run %s: %s", request.RunID, resolvedBuild)
		return fmt.Errorf("%w: craft web build program resolves through a symlink outside the pinned toolchain for run %s", craft.ErrConflict, request.RunID)
	}
	if g.hostInputsRel != "" && (pathWithinDir(g.hostInputsRel, resolvedDir) || pathWithinDir(g.hostInputsRel, resolvedBuild)) {
		logger.Warnf(ctx, "[CraftWebBuild] resolved toolchain overlaps the staged read-only inputs tree for run %s: %s", request.RunID, resolvedDir)
		return fmt.Errorf("%w: craft web toolchain resolves inside the staged inputs tree for run %s", craft.ErrConflict, request.RunID)
	}
	return nil
}

// pathWithinDir reports whether target equals or lies inside directory after
// lexical cleaning (both sides are already symlink-resolved).
func pathWithinDir(directory, target string) bool {
	if directory == "" || target == "" {
		return false
	}
	rel, err := filepath.Rel(directory, target)
	if err != nil {
		// A Rel failure means the roots cannot be related (relative vs
		// absolute mix): the check cannot PROVE separation, so it reports
		// overlap (the caller refuses dispatch) instead of silently passing.
		return true
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
