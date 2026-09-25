package craft

import (
	"bytes"
	"fmt"
	"path"
	"regexp"
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
	// A UTF-8 BOM may precede the shebang on real uploaded scripts; the
	// provenance label must not be understated because of it.
	content = bytes.TrimPrefix(content, []byte("\xef\xbb\xbf"))
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
	// Environment carries the exec environment; every value is screened
	// against the read-only tree so startup hooks (BASH_ENV, PYTHONSTARTUP,
	// ENV, NODE_OPTIONS --require, RUBYOPT, PERL5OPT, ...) cannot point at
	// uploaded material.
	Environment map[string]string

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
	"php": true, "lua": true, "luajit": true, "awk": true, "gawk": true, "mawk": true,
	"tclsh": true, "wish": true, "pwsh": true, "powershell": true, "rscript": true,
	"java": true, "scala": true, "groovy": true, "elixir": true, "runghc": true,
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
//
// Evidence semantics (adapted from the OCR hardening round): the symlink
// and byte-identity layers only fire when the ADAPTER supplies
// ResolvedTargetPath / TargetSHA256 — a server-side adapter that cannot walk
// the container filesystem relies on the lexical layers below. Shell
// expressions are therefore fail-closed: without adapter-supplied
// normalized evidence they are treated as unreviewable and denied, because
// lexical tokenization alone can be bypassed (assignments, quoting,
// escapes). argv operands are screened exhaustively (wrapper- and
// version-aware interpreter detection, sub-token screening of
// whitespace-carrying operands, environment-value containment) so a
// wrapping launcher cannot smuggle an uploaded script past the gate.
func (p *InputExecutionPolicy) Review(req InputExecutionRequest) InputExecutionDecision {
	// 1. Symlink-resolved target inside the read-only tree.
	if resolved := p.canonical(req.WorkingDir, req.ResolvedTargetPath); p.withinInputs(resolved) {
		return p.deny("input_symlink", resolved, "")
	}
	// 2. Byte identity: executing input bytes anywhere (copy, temp dir,
	// renamed payload, or bytes fed to the process on stdin) is executing
	// the uploaded material.
	if req.TargetSHA256 != "" {
		if _, ok := p.byDigest[req.TargetSHA256]; ok {
			target := p.canonical(req.WorkingDir, req.TargetPath)
			if target == "" {
				target = req.TargetSHA256
			}
			return p.deny("input_identity", target, req.TargetSHA256)
		}
	}
	// 2b. Environment values that point into the read-only tree turn a
	// benign command into execution of uploaded material (BASH_ENV,
	// PYTHONSTARTUP, ENV, NODE_OPTIONS --require, RUBYOPT, PERL5OPT ...),
	// because runtimes source those files at startup.
	for _, value := range req.Environment {
		for _, token := range shellTokens(value) {
			if abs := p.canonical(req.WorkingDir, token); p.withinInputs(abs) {
				return p.deny("input_target", abs, "")
			}
		}
	}
	if req.Shell {
		// Fail-closed: a shell expression cannot be reviewed lexically with
		// sufficient confidence (indirection, quoting and escapes can hide
		// the real target). Only adapter-supplied normalized evidence — a
		// resolved target outside the tree plus a digest that is not
		// uploaded material, both screened above — may pass a shell form.
		if req.ResolvedTargetPath == "" || req.TargetSHA256 == "" {
			return p.deny("shell_input", p.canonical(req.WorkingDir, req.TargetPath), "")
		}
		for _, token := range shellTokens(req.CommandText) {
			if abs := p.canonical(req.WorkingDir, token); p.withinInputs(abs) {
				return p.deny("shell_input", abs, "")
			}
		}
	} else if interpreter, offset := InterpreterPrefix(req.Command); interpreter {
		rest := req.Command[offset:]
		// An interpreter reading its program from stdin executes whatever
		// bytes arrive on stdin — deny the marker forms outright; the
		// adapter separately screens the stdin bytes by digest.
		for _, arg := range rest {
			if arg == "-" || arg == "/dev/stdin" {
				return p.deny("interpreter_input", arg, "")
			}
		}
		// The interpreter executes exactly one script operand: the first
		// argument that is not an option flag. Later operands are the
		// script's own data (reading uploaded data is allowed), except that
		// any operand carrying embedded whitespace is screened as a nested
		// command (bash -c "python3 <path>" smuggling).
		scriptSeen := false
		for _, arg := range rest {
			if !scriptSeen && strings.HasPrefix(arg, "-") {
				continue
			}
			if !scriptSeen {
				scriptSeen = true
				if abs := p.canonical(req.WorkingDir, arg); p.withinInputs(abs) {
					return p.deny("interpreter_input", abs, "")
				}
				// An embedded command (bash -c "python3 <path>") arrives as
				// the script operand carrying whitespace; screen its
				// sub-tokens too.
				for _, token := range shellTokens(arg) {
					if abs := p.canonical(req.WorkingDir, token); p.withinInputs(abs) {
						return p.deny("interpreter_input", abs, "")
					}
				}
				continue
			}
			if !strings.ContainsAny(arg, " \t\n") {
				continue // pure data operand: the script reads it, no execution
			}
			for _, token := range shellTokens(arg) {
				if abs := p.canonical(req.WorkingDir, token); p.withinInputs(abs) {
					return p.deny("interpreter_input", abs, "")
				}
			}
		}
	} else if len(req.Command) > 0 {
		// Non-interpreter command face. Read-only utilities may name input
		// material (reading uploaded data is the intended use); every other
		// command denies ANY operand or embedded sub-token inside the
		// read-only tree, so wrapping launchers (timeout, nohup, xargs, ...)
		// cannot smuggle an uploaded script as an operand.
		if !readOnlyCommands[path.Base(req.Command[0])] {
			for _, arg := range req.Command[1:] {
				for _, token := range shellTokens(arg) {
					if abs := p.canonical(req.WorkingDir, token); p.withinInputs(abs) {
						return p.deny("input_target", abs, "")
					}
				}
			}
		}
		if abs := p.canonical(req.WorkingDir, req.Command[0]); p.withinInputs(abs) {
			// Direct execution of the uploaded path itself.
			return p.deny("input_target", abs, "")
		}
		// Read-only utilities and byte-copying commands (cp, mv) naming input
		// material do not execute it here; the digest layer catches the
		// copied bytes at their next execution WHEN the adapter supplies the
		// target digest — a server-side adapter without container filesystem
		// access documents that two-step copy/execute as a known residual
		// instead (see the adapter's evidence contract).
	}
	// Explicit target path inside the tree even when the command shape did
	// not match (defensive: adapters may fill TargetPath only).
	if abs := p.canonical(req.WorkingDir, req.TargetPath); p.withinInputs(abs) {
		return p.deny("input_target", abs, "")
	}
	return InputExecutionDecision{Allowed: true, AuditKind: AuditKindGeneratedExecute}
}

// wrapperCommands transparently wrap the real launcher; skipping them keeps
// interpreter detection correct for `timeout 10 python3 x.py`,
// `env VAR=1 python3 x.py`, `nohup python3 x.py`, `xargs python3 x.py`, ...
var wrapperCommands = map[string]bool{
	"timeout": true, "env": true, "nohup": true, "xargs": true,
	"nice": true, "stdbuf": true, "setsid": true, "time": true,
}

// readOnlyCommands may name input material as operands without being
// execution of it. Deliberately narrow: text/byte readers and inspectors
// only. awk/sed (they execute programs), find (it has -exec) and every
// copying command (cp, mv, dd, tee, install, rsync) are NOT here, so their
// operand set is screened by the containment rule above.
var readOnlyCommands = map[string]bool{
	"cat": true, "head": true, "tail": true, "less": true, "more": true,
	"grep": true, "egrep": true, "fgrep": true, "rg": true, "wc": true,
	"stat": true, "file": true, "ls": true, "du": true, "diff": true,
	"md5sum": true, "sha1sum": true, "sha256sum": true, "sha512sum": true,
	"sort": true, "uniq": true, "cut": true, "tr": true,
}

// InterpreterPrefix reports whether the argv launches an interpreter that
// executes a following path argument, and the offset where its operands
// start. It is wrapper-aware (timeout/env/nohup/xargs/nice/stdbuf/setsid/
// time, with env assignments and the timeout duration skipped) and tolerant
// of version-suffixed interpreter names (python3.11, php8.2, lua5.4,
// tclsh8.6, mawk ...). It is exported so server-side adapters can reuse the
// same detection for their own evidence assembly (shell -c detection,
// stdin-program forms).
func InterpreterPrefix(command []string) (bool, int) {
	for offset := 0; offset < len(command); {
		base := path.Base(command[offset])
		if isInterpreterName(base) {
			return true, offset + 1
		}
		if !wrapperCommands[base] {
			return false, 0
		}
		offset++
		switch base {
		case "env":
			// Skip VAR=value assignments and env's own flags before the
			// real program.
			for offset < len(command) {
				next := command[offset]
				if strings.Contains(next, "=") && !strings.HasPrefix(next, "-") {
					offset++
					continue
				}
				break
			}
		case "timeout":
			// timeout consumes its duration operand (and optional flags).
			for offset < len(command) && strings.HasPrefix(command[offset], "-") {
				offset++
			}
			if offset < len(command) {
				offset++
			}
		case "xargs":
			for offset < len(command) && strings.HasPrefix(command[offset], "-") {
				offset++
			}
		}
	}
	return false, 0
}

// isInterpreterName matches an interpreter basename exactly or with a
// version suffix (python3.11, php8.2, lua5.4, tclsh8.6, ...).
func isInterpreterName(base string) bool {
	if interpreterCommands[base] {
		return true
	}
	for stem := range interpreterCommands {
		if strings.HasPrefix(base, stem) && versionSuffix.MatchString(base[len(stem):]) {
			return true
		}
	}
	return false
}

// versionSuffix is the tolerated trailing version marker after a known
// interpreter stem: digits and dots only, starting with a digit.
var versionSuffix = regexp.MustCompile(`^[0-9][0-9.]*$`)

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

// shellTokens splits a shell expression into policy-checkable tokens.
// Separators become spaces; backslash escapes and quote characters are
// dropped entirely (so adjacent-quote splices like "in""puts/x.py" and
// escapes like in\puts/x.py expose the real path); a leading VAR=value
// assignment prefix is stripped so the assigned path is still screened.
// Every transformation can only ADD screened tokens, never remove them.
func shellTokens(commandText string) []string {
	replaced := strings.Map(func(r rune) rune {
		if strings.ContainsRune(shellSeparators, r) {
			return ' '
		}
		if r == '\\' || r == '"' || r == '\'' {
			return -1
		}
		return r
	}, commandText)
	fields := strings.Fields(replaced)
	for i, f := range fields {
		fields[i] = strings.TrimPrefix(f, envAssignment(f))
	}
	return fields
}

// envAssignment returns the leading VAR= prefix of a token, if any, so the
// assigned value remains visible to screening after the prefix is stripped.
func envAssignment(token string) string {
	for i := 0; i < len(token); i++ {
		c := token[i]
		if c == '=' {
			if i == 0 {
				return ""
			}
			first := token[0]
			if (first >= 'a' && first <= 'z') || (first >= 'A' && first <= 'Z') || first == '_' {
				return token[:i+1]
			}
			return ""
		}
		if !(c >= 'a' && c <= 'z') && !(c >= 'A' && c <= 'Z') && !(c >= '0' && c <= '9') && c != '_' {
			return ""
		}
	}
	return ""
}
