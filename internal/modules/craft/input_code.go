package craft

import (
	"fmt"
	"path"
	"strings"
)

// Uploaded-code execution policy (T03): a member may upload code and ask to
// run it, but the uploaded file stays read-only input material. The Agent may
// read it and generate new code in the writable Workspace; no interpreter,
// shell, copy, link or alias path may execute the uploaded bytes themselves.

// Audit kinds distinguish reading uploaded code from executing generated
// code. They are stable identifiers: keep the exact values.
const (
	// AuditKindInputRead marks an observation of uploaded input material as
	// data (read, cited, parsed) — never executed.
	AuditKindInputRead = "craft.input.read"
	// AuditKindGeneratedExecute marks an allowed execution whose bytes were
	// generated inside the writable Workspace/output boundary.
	AuditKindGeneratedExecute = "craft.generated.execute"
	// AuditKindInputExecuteDenied marks a refused execution attempt whose
	// target traced back to uploaded input material.
	AuditKindInputExecuteDenied = "craft.input.execute_denied"
)

// ErrExecutionDenied is the sentinel for a refused execution of uploaded
// input material. It wraps ErrForbidden so scope/error mapping stays uniform.
var ErrExecutionDenied = fmt.Errorf("%w: uploaded input material must not be executed", ErrForbidden)

// scriptExtensions name uploads whose bytes are code by declaration. The
// classification only labels provenance; it never grants any capability.
var scriptExtensions = map[string]bool{
	".py": true, ".pyw": true, ".sh": true, ".bash": true, ".zsh": true, ".ksh": true,
	".js": true, ".mjs": true, ".cjs": true, ".ts": true, ".rb": true, ".pl": true,
	".pm": true, ".php": true, ".lua": true, ".r": true, ".jl": true, ".go": true,
	".java": true, ".hs": true, ".ex": true, ".exs": true, ".scala": true, ".groovy": true,
}

// InputCodeLabel is the provenance label of one uploaded file: code means the
// bytes look like a program (shebang or script extension), data means they do
// not. Either way the file keeps read-only input status.
type InputCodeLabel struct {
	IsCode bool
	Reason string // shebang | script_extension | data
}

// ClassifyInputCode labels uploaded bytes by provenance only. It never parses
// the program and never changes the input's acceptance or capability.
func ClassifyInputCode(name string, content []byte) InputCodeLabel {
	if hasShebang(content) {
		return InputCodeLabel{IsCode: true, Reason: "shebang"}
	}
	if scriptExtensions[strings.ToLower(path.Ext(name))] {
		return InputCodeLabel{IsCode: true, Reason: "script_extension"}
	}
	return InputCodeLabel{Reason: "data"}
}

func hasShebang(content []byte) bool {
	return len(content) >= 2 && content[0] == '#' && content[1] == '!'
}

// InputExecutionRequest is one normalized, server-observed execution attempt
// against material in a Craft workspace. The adapter that observes the
// attempt fills what it knows: the argv (or shell expression), the working
// directory, and — when it resolved the primary target — the target path and
// the digest of the bytes that would execute. Relative paths resolve against
// WorkingDir (then the workspace root); ResolvedTargetPath carries the
// symlink-resolved absolute path when the adapter evaluated links.
type InputExecutionRequest struct {
	Command     []string // argv when Shell is false
	CommandText string   // shell expression when Shell is true
	Shell       bool
	WorkingDir  string

	TargetPath         string
	ResolvedTargetPath string
	TargetSHA256       string
}

// InputExecutionDecision is the policy verdict for one execution attempt.
// Reason is a stable machine code, empty when allowed.
type InputExecutionDecision struct {
	Allowed   bool
	Reason    string // input_target | interpreter_input | shell_input | input_symlink | input_identity
	AuditKind string
	Target    string // canonical path or digest identity that matched
	Digest    string
	Err       error
}

// Refusal renders the member-visible refusal for a denied execution: what was
// refused, why, and the allowed alternative. Allowed decisions return "".
func (d InputExecutionDecision) Refusal() string {
	if d.Allowed {
		return ""
	}
	target := d.Target
	if target == "" {
		target = d.Digest
	}
	return fmt.Sprintf(
		"Execution denied: uploaded input material is read-only data and must not be executed (matched %s). "+
			"Allowed alternative: have the sub-executor generate the code in the writable Workspace and execute the generated file instead.",
		target)
}

// InputAuditEvent is one typed audit observation for Craft material use.
type InputAuditEvent struct {
	Kind   string
	Target string
	Digest string
}

// InputReadAuditEvent projects one uploaded input read (data use) as a typed
// audit event, distinct from any execution event.
func InputReadAuditEvent(in Input) InputAuditEvent {
	return InputAuditEvent{Kind: AuditKindInputRead, Target: in.Ref, Digest: in.SHA256}
}

// interpreterCommands are the launchers that execute a following path
// argument as code. Matching is on the basename of argv[0] (and argv[1] for
// env-style loaders).
var interpreterCommands = map[string]bool{
	"python": true, "python3": true, "python2": true, "node": true, "nodejs": true,
	"deno": true, "bun": true, "bash": true, "sh": true, "zsh": true, "dash": true,
	"ksh": true, "ash": true, "csh": true, "tcsh": true, "ruby": true, "perl": true,
	"php": true, "lua": true, "luajit": true, "awk": true, "gawk": true, "tclsh": true,
	"wish": true, "pwsh": true, "powershell": true, "rscript": true, "java": true,
}

// shellSeparators are replaced by spaces before tokenizing a shell
// expression, so a path smuggled behind && ; | or a subshell still appears as
// its own token.
const shellSeparators = "&|;<>()`\n"

// InputExecutionPolicy decides execution attempts for one delegation's
// admitted inputs inside one workspace root. The read-only input tree is
// <root>/inputs; everything outside it that is not byte-identical to an
// admitted input counts as generated Workspace/output material.
type InputExecutionPolicy struct {
	workspaceRoot string
	inputsTree    string
	inputs        []Input
	byDigest      map[string]Input
}

// NewInputExecutionPolicy validates the workspace root and the admitted input
// manifest, then returns the execution policy for that material set.
func NewInputExecutionPolicy(workspaceRoot string, inputs []Input) (*InputExecutionPolicy, error) {
	if strings.TrimSpace(workspaceRoot) == "" {
		return nil, fmt.Errorf("%w: execution policy requires a workspace root", ErrInvalidInput)
	}
	if err := ValidateInputManifest(inputs); err != nil {
		return nil, err
	}
	root := path.Clean("/" + strings.TrimSpace(workspaceRoot))
	byDigest := make(map[string]Input, len(inputs))
	for _, in := range inputs {
		byDigest[in.SHA256] = in
	}
	return &InputExecutionPolicy{
		workspaceRoot: root,
		inputsTree:    path.Join(root, InputDir),
		inputs:        append([]Input(nil), inputs...),
		byDigest:      byDigest,
	}, nil
}

// Inputs returns the admitted input manifest the policy guards.
func (p *InputExecutionPolicy) Inputs() []Input {
	return append([]Input(nil), p.inputs...)
}

// Contains reports whether one path, after lexical canonicalization against
// the working directory, lies inside the read-only input tree. Adapters use
// it to classify material before proposing any execution.
func (p *InputExecutionPolicy) Contains(workingDir, target string) bool {
	return p.withinInputs(p.canonical(workingDir, target))
}

func (p *InputExecutionPolicy) withinInputs(abs string) bool {
	if abs == "" {
		return false
	}
	return abs == p.inputsTree || strings.HasPrefix(abs, p.inputsTree+"/")
}

// canonical resolves one possibly relative, aliased path (., .., duplicated
// separators, trailing slash) to a cleaned absolute path. Symlinks are not
// resolved here: the adapter supplies ResolvedTargetPath for that.
func (p *InputExecutionPolicy) canonical(workingDir, target string) string {
	target = strings.TrimSpace(target)
	if target == "" {
		return ""
	}
	target = trimQuotes(target)
	if !strings.HasPrefix(target, "/") {
		base := workingDir
		if strings.TrimSpace(base) == "" {
			base = p.workspaceRoot
		}
		if !strings.HasPrefix(base, "/") {
			base = "/" + base
		}
		target = base + "/" + target
	}
	return path.Clean(target)
}

func trimQuotes(token string) string {
	return strings.Trim(token, "'\"")
}

// Review decides one execution attempt. Upload-material execution is denied
// by path containment, by symlink resolution and by canonical byte identity
// (a copy in /tmp or anywhere else keeps the input digest); everything else
// inside the writable Workspace boundary is allowed generated code.
func (p *InputExecutionPolicy) Review(req InputExecutionRequest) InputExecutionDecision {
	// 1. Symlink-resolved target inside the read-only tree.
	if resolved := p.canonical(req.WorkingDir, req.ResolvedTargetPath); p.withinInputs(resolved) {
		return p.deny("input_symlink", resolved, "")
	}
	// 2. Byte identity: executing input bytes anywhere (copy, temp dir,
	// renamed payload) is executing the uploaded material.
	if req.TargetSHA256 != "" {
		if _, ok := p.byDigest[req.TargetSHA256]; ok {
			target := p.canonical(req.WorkingDir, req.TargetPath)
			if target == "" {
				target = req.TargetSHA256
			}
			return p.deny("input_identity", target, req.TargetSHA256)
		}
	}
	if req.Shell {
		for _, token := range shellTokens(req.CommandText) {
			if abs := p.canonical(req.WorkingDir, token); p.withinInputs(abs) {
				return p.deny("shell_input", abs, "")
			}
		}
	} else if interpreter, offset := p.interpreterPrefix(req); interpreter {
		// Any input-tree argument feeds the interpreter as the script to run.
		for _, arg := range req.Command[offset:] {
			if abs := p.canonical(req.WorkingDir, arg); p.withinInputs(abs) {
				return p.deny("interpreter_input", abs, "")
			}
		}
	} else if len(req.Command) > 0 {
		if abs := p.canonical(req.WorkingDir, req.Command[0]); p.withinInputs(abs) {
			// Direct execution of the uploaded path itself.
			return p.deny("input_target", abs, "")
		}
		// A non-interpreter command naming input material (cp, mv, cat) is
		// not execution of it; the digest rule above catches the copied
		// bytes when they later execute.
	}
	// Explicit target path inside the tree even when the command shape did
	// not match (defensive: adapters may fill TargetPath only).
	if abs := p.canonical(req.WorkingDir, req.TargetPath); p.withinInputs(abs) && !req.Shell {
		return p.deny("input_target", abs, "")
	}
	return InputExecutionDecision{Allowed: true, AuditKind: AuditKindGeneratedExecute}
}

// interpreterPrefix reports whether the argv launches an interpreter that
// executes a following path argument, and the offset where those path
// arguments start (env-style loaders skip their own first operand).
func (p *InputExecutionPolicy) interpreterPrefix(req InputExecutionRequest) (bool, int) {
	if len(req.Command) == 0 {
		return false, 0
	}
	if interpreterCommands[path.Base(req.Command[0])] {
		return true, 1
	}
	if path.Base(req.Command[0]) == "env" && len(req.Command) > 1 && interpreterCommands[path.Base(req.Command[1])] {
		return true, 2
	}
	return false, 0
}

func (p *InputExecutionPolicy) deny(reason, target, digest string) InputExecutionDecision {
	return InputExecutionDecision{
		Allowed:   false,
		Reason:    reason,
		AuditKind: AuditKindInputExecuteDenied,
		Target:    target,
		Digest:    digest,
		Err:       fmt.Errorf("%w: %s matched %s", ErrExecutionDenied, reason, target),
	}
}

// shellTokens splits a shell expression into policy-checkable tokens:
// separators become spaces, quote characters are trimmed from each edge.
func shellTokens(commandText string) []string {
	replaced := strings.Map(func(r rune) rune {
		if strings.ContainsRune(shellSeparators, r) {
			return ' '
		}
		return r
	}, commandText)
	fields := strings.Fields(replaced)
	for i, f := range fields {
		fields[i] = trimQuotes(f)
	}
	return fields
}
