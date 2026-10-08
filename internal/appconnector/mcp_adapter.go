package appconnector

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
)

// MCP definition/execution rejections.
var (
	// ErrMCPDefinitionChanged: the advertised tool list, input schema or
	// risk no longer matches the admin-pinned version. The installation
	// must go to pending re-review, not execute.
	ErrMCPDefinitionChanged = errors.New("mcp_definition_changed")
	// ErrHostCommandRefused: a stdio command was derived from a remote
	// description, or is not on the admin allowlist.
	ErrHostCommandRefused = errors.New("mcp_host_command_refused")
	// ErrStdioIsolationRequired: stdio execution was attempted outside the
	// controlled isolation (no isolation dir, or inheriting host env).
	ErrStdioIsolationRequired = errors.New("mcp_stdio_isolation_required")
)

// MCPToolSpec is one tool as ADVERTISED by the remote MCP server. Everything
// in it — including ReadOnlyHint — is untrusted remote input.
type MCPToolSpec struct {
	Name         string
	InputSchema  json.RawMessage
	Risk         string
	ReadOnlyHint bool // advisory only; NEVER lowers the reviewed risk
}

// PinnedMCPTool is the admin-reviewed version pin of one tool: the digest of
// its canonical input schema plus its risk category. The advisory readOnly
// hint is deliberately NOT pinned — flipping it remotely changes nothing
// that security depends on.
type PinnedMCPTool struct {
	Name         string
	SchemaDigest string
	Risk         string
}

// mcpSchemaDigest hashes the normalized input schema plus the risk. Key and
// order changes in the raw JSON that normalize to the same document do not
// trip the pin; any material schema or risk change does.
func mcpSchemaDigest(schema json.RawMessage, risk string) string {
	norm, err := NormalizeArgs(schema)
	if err != nil {
		norm = []byte(fmt.Sprintf("invalid:%q", string(schema)))
	}
	h := sha256.New()
	h.Write(norm)
	h.Write([]byte{0})
	h.Write([]byte(risk))
	return hex.EncodeToString(h.Sum(nil))
}

// PinMCPTools records the version pin for an admin-reviewed tool list.
func PinMCPTools(specs []MCPToolSpec) []PinnedMCPTool {
	pins := make([]PinnedMCPTool, 0, len(specs))
	for _, s := range specs {
		pins = append(pins, PinnedMCPTool{Name: s.Name, SchemaDigest: mcpSchemaDigest(s.InputSchema, s.Risk), Risk: s.Risk})
	}
	return pins
}

// VerifyPinnedMCPTools compares the currently advertised tools against the
// pin: every pinned tool must be present with an identical schema digest
// and risk, and no unpinned tool may appear. Any drift wraps
// ErrMCPDefinitionChanged.
func VerifyPinnedMCPTools(pinned []PinnedMCPTool, observed []MCPToolSpec) error {
	byName := make(map[string]MCPToolSpec, len(observed))
	for _, o := range observed {
		byName[o.Name] = o
	}
	for _, pin := range pinned {
		o, ok := byName[pin.Name]
		if !ok {
			return fmt.Errorf("%w: pinned tool %q missing from advertised list", ErrMCPDefinitionChanged, pin.Name)
		}
		if got := mcpSchemaDigest(o.InputSchema, o.Risk); got != pin.SchemaDigest {
			return fmt.Errorf("%w: tool %q schema/risk digest changed", ErrMCPDefinitionChanged, pin.Name)
		}
	}
	pinnedNames := make(map[string]bool, len(pinned))
	for _, pin := range pinned {
		pinnedNames[pin.Name] = true
	}
	for _, o := range observed {
		if !pinnedNames[o.Name] {
			return fmt.Errorf("%w: unpinned tool %q advertised", ErrMCPDefinitionChanged, o.Name)
		}
	}
	return nil
}

// EffectiveMCPRisk returns the risk category that governs approval for one
// tool: the REVIEWED risk. The remote readOnly hint is advisory metadata
// only — a forged hint can neither downgrade the category nor bypass
// NeedsExplicitApproval.
func EffectiveMCPRisk(tool MCPToolSpec) string {
	return tool.Risk
}

// InstallationPendingReview is the installation state a definition change
// forces: the drift is recorded and a human re-reviews before execution
// resumes.
const InstallationPendingReview = "pending_review"

// MCPDefinitionChangeOutcome maps a verification error to the installation
// outcome: changed=true with the pending-review state, or changed=false
// with the active state for a nil error.
func MCPDefinitionChangeOutcome(err error) (state string, changed bool) {
	if err == nil {
		return "active", false
	}
	if errors.Is(err, ErrMCPDefinitionChanged) {
		return InstallationPendingReview, true
	}
	return "error", false
}

// Who may declare a stdio command. Only admin-reviewed declarations can
// ever launch; anything the remote server described is refused.
const (
	MCPCommandDeclaredByAdmin  = "admin"
	MCPCommandDeclaredByRemote = "remote_description"
)

// MCPStdioCommand is a requested stdio launch. DeclaredBy records where the
// binary came from: a remote server's description may NEVER produce a host
// command.
type MCPStdioCommand struct {
	Binary     string
	Args       []string
	DeclaredBy string
}

// StdioSandboxPolicy is the admin-pinned stdio execution policy: an exact
// binary allowlist and a mandatory controlled isolation directory with a
// clean environment (never the host env).
type StdioSandboxPolicy struct {
	AllowedBinaries []string
	IsolationDir    string
	InheritEnv      bool
}

// Authorize decides whether one stdio command may launch. Remote-derived
// commands and non-allowlisted binaries are refused outright; even an
// allowlisted admin command is refused unless it runs inside the controlled
// isolation with a clean environment.
func (p StdioSandboxPolicy) Authorize(cmd MCPStdioCommand) error {
	if cmd.DeclaredBy != MCPCommandDeclaredByAdmin {
		return fmt.Errorf("%w: command declared by %q", ErrHostCommandRefused, cmd.DeclaredBy)
	}
	allowed := false
	for _, b := range p.AllowedBinaries {
		if b == cmd.Binary {
			allowed = true
			break
		}
	}
	if !allowed {
		return fmt.Errorf("%w: binary %q not on the admin allowlist", ErrHostCommandRefused, cmd.Binary)
	}
	if p.IsolationDir == "" {
		return fmt.Errorf("%w: no controlled isolation directory", ErrStdioIsolationRequired)
	}
	if p.InheritEnv {
		return fmt.Errorf("%w: host environment inheritance is not controlled isolation", ErrStdioIsolationRequired)
	}
	return nil
}

// MCPAdapter is the MCP member of the Adapter family. The real transport is
// injected as Call so this type owns exactly the MCP-specific guarantees:
// version-pinned definitions, advisory-only remote hints, and stdio
// execution confined to the admin sandbox. Composition with GatedAdapter
// puts the transport between the A03 intent and the U05 budget gate.
type MCPAdapter struct {
	QueryUnsupported
	Pinned []PinnedMCPTool
	Stdio  StdioSandboxPolicy
	// Call is the real MCP transport (session-owned, injected at install).
	Call func(ctx context.Context, a Action) (ActionResult, error)
}

// Execute runs the real MCP call. Definition drift is checked first: a
// changed tool never executes, the installation goes to pending re-review.
func (m *MCPAdapter) Execute(ctx context.Context, a Action) (ActionResult, error) {
	if err := VerifyAgainstAdvertised(ctx, m.Pinned); err != nil {
		if _, changed := MCPDefinitionChangeOutcome(err); changed {
			return ActionResult{State: ActionQueued}, err
		}
	}
	if m.Call == nil {
		return ActionResult{State: ActionFailed}, errors.New("mcp_transport_unavailable")
	}
	return m.Call(ctx, a)
}

// AdvertisedTools is the current remote tool list source used for drift
// checks (injected; production binds it to the MCP session refresh).
type AdvertisedTools interface {
	Current(ctx context.Context) ([]MCPToolSpec, error)
}

// VerifyAgainstAdvertised re-checks the pin against the live tool list when
// a source is bound; with no source the stored pin stands as reviewed.
func VerifyAgainstAdvertised(ctx context.Context, pinned []PinnedMCPTool) error {
	src := advertisedToolsFromContext(ctx)
	if src == nil {
		return nil
	}
	current, err := src.Current(ctx)
	if err != nil {
		return err
	}
	return VerifyPinnedMCPTools(pinned, current)
}

type advertisedCtxKey struct{}

// WithAdvertisedTools binds the live tool-list source for one call.
func WithAdvertisedTools(ctx context.Context, src AdvertisedTools) context.Context {
	return context.WithValue(ctx, advertisedCtxKey{}, src)
}

func advertisedToolsFromContext(ctx context.Context) AdvertisedTools {
	src, _ := ctx.Value(advertisedCtxKey{}).(AdvertisedTools)
	return src
}
