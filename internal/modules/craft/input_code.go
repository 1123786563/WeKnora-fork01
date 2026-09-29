package craft

import (
	"bytes"
	"fmt"
	"path"
	"regexp"
	"strconv"
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
	Reason    string // input_target | interpreter_input | shell_input | input_symlink | input_identity | wrapper_shape | exec_forward
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
	if target == "" {
		// Evidence-free denials (unreviewable shell or program-text forms)
		// carry no matched identity; the member still needs a readable,
		// non-degenerate refusal.
		target = "an unreviewable command form (shell expression or inline program)"
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
	// because runtimes source those files at startup. Path-list variables
	// (PYTHONPATH, NODE_PATH, PERL5LIB, RUBYLIB, PATH) carry colon-separated
	// entries, so every segment is screened independently — an inputs entry
	// hidden behind a leading /usr/lib must not slip through.
	for _, value := range req.Environment {
		for _, token := range shellTokens(value) {
			for _, segment := range strings.Split(token, ":") {
				candidates := append([]string{segment}, flagValueCandidates(segment)...)
				for _, candidate := range candidates {
					if stripped, ok := stripEnvAssignment(candidate); ok {
						candidate = stripped
					}
					if abs := p.canonical(req.WorkingDir, candidate); p.withinInputs(abs) {
						return p.deny("input_target", abs, "")
					}
				}
			}
		}
	}
	if req.Shell {
		// Java expands @argfiles before the launcher policy can inspect their
		// contents. For shell forms, use the same quote/escape-normalized
		// tokens as the other lexical checks and fail closed on any Java
		// command carrying an argfile token.
		shellArgv := shellTokens(req.CommandText)
		if hasJavaLauncherToken(shellArgv) {
			if value := strings.TrimSpace(req.Environment["JDK_JAVA_OPTIONS"]); value != "" {
				return p.deny("interpreter_input", "JDK_JAVA_OPTIONS", "")
			}
			for _, rawToken := range strings.Fields(req.CommandText) {
				if key, value, ok := javaEnvAssignment(trimQuotes(rawToken)); ok && key == "JDK_JAVA_OPTIONS" && strings.Trim(value, "'\"") != "" {
					return p.deny("interpreter_input", key, "")
				}
			}
			for _, token := range shellArgv {
				if target, ok := javaArgFileReference(token, req.WorkingDir, p); ok {
					return p.deny("interpreter_input", target, "")
				}
			}
		}
		// Fail-closed: a shell expression cannot be reviewed lexically with
		// sufficient confidence (indirection, quoting and escapes can hide
		// the real target). Only adapter-supplied normalized evidence — a
		// resolved target outside the tree plus a digest that is not
		// uploaded material, both screened above — may pass a shell form.
		if req.ResolvedTargetPath == "" || req.TargetSHA256 == "" {
			return p.deny("shell_input", p.canonical(req.WorkingDir, req.TargetPath), "")
		}
		for _, token := range shellTokens(req.CommandText) {
			for _, segment := range strings.Split(token, ":") {
				candidates := append([]string{segment}, flagValueCandidates(segment)...)
				for _, candidate := range candidates {
					if stripped, ok := stripEnvAssignment(candidate); ok {
						candidate = stripped
					}
					if abs := p.canonical(req.WorkingDir, candidate); p.withinInputs(abs) {
						return p.deny("shell_input", abs, "")
					}
				}
			}
		}
	} else if hasExecForwardFlag(req.Command) || hasStdinPlaceholder(req.Command) {
		// find -exec / -execdir forwards arbitrary files to an interpreter
		// at RUNTIME ({} is filled with paths from the whole workspace,
		// including the read-only inputs tree): the argv itself is lexically
		// clean, so no path containment can catch it. Fail closed.
		return p.deny("exec_forward", strings.Join(req.Command, " "), "")
	} else if status, offset := InterpreterPrefixStatus(req.Command); status != InterpreterScanNone {
		if status == InterpreterScanInconclusive {
			// Fail closed: the wrapper's option structure could not be
			// parsed conclusively (unknown flag arity, combined short
			// options, an operand shape we do not model). Guessing here
			// would silently drop the rest of the line into the weaker
			// non-interpreter lexical branch, where inline program-text
			// flags are no longer refused.
			return p.deny("wrapper_shape", strings.Join(req.Command, " "), "")
		}
		// The wrapper prefix consumed by InterpreterPrefix is itself
		// screened: env assignments (env BASH_ENV=<inputs>/x.sh bash gen.sh)
		// are startup hooks exactly like Environment entries and must not
		// ride along unscreened just because the interpreter was found.
		for _, prefix := range req.Command[:offset] {
			for _, token := range shellTokens(prefix) {
				for _, segment := range strings.Split(token, ":") {
					if abs := p.canonical(req.WorkingDir, segment); p.withinInputs(abs) {
						return p.deny("input_target", abs, "")
					}
				}
			}
			// Wrapper long options with attached values (env
			// --split-string=BASH_ENV=<inputs>/x.sh) are startup hooks exactly
			// like assignments: the =-attached value is screened too.
			for _, value := range flagValueCandidates(prefix) {
				if abs := p.canonical(req.WorkingDir, value); p.withinInputs(abs) {
					return p.deny("input_target", abs, "")
				}
			}
		}
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
		pendingRun := false
		// programFileNext marks that the previous option token was a
		// separated-form program-file flag whose VALUE operand follows.
		programFileNext := false
		javaPathValueNext := ""
		// phpSawServer marks that php's -S already appeared: only THEN is a
		// positional the router script php executes per request.
		phpSawServer := false
		interpreter := path.Base(req.Command[offset-1])
		if interpreter == "java" {
			for _, arg := range rest {
				if target, ok := javaArgFileReference(arg, req.WorkingDir, p); ok {
					return p.deny("interpreter_input", target, "")
				}
			}
			for _, prefixArg := range req.Command[:offset-1] {
				if key, value, ok := javaEnvAssignment(prefixArg); ok && key == "JDK_JAVA_OPTIONS" && value != "" {
					return p.deny("interpreter_input", key, "")
				}
			}
			for key, value := range req.Environment {
				if key == "JDK_JAVA_OPTIONS" && strings.TrimSpace(value) != "" {
					return p.deny("interpreter_input", key, "")
				}
			}
		}
		for _, arg := range rest {
			if !scriptSeen {
				// run subcommands (deno run x.ts, bun run x.js) shift the
				// executed file one operand later; remember and keep
				// treating the remainder as the option/operand region.
				if arg == "run" && !pendingRun {
					// "run" itself can BE the uploaded script when the
					// working dir points inside the tree (python3 run with
					// an uploaded extension-less file named run): screen it
					// before shifting.
					if abs := p.canonical(req.WorkingDir, arg); p.withinInputs(abs) {
						return p.deny("interpreter_input", abs, "")
					}
					pendingRun = true
					continue
				}
				if javaPathValueNext != "" {
					option := javaPathValueNext
					javaPathValueNext = ""
					if option == "--patch-module" {
						target, valid := javaPatchModuleInputEntry(arg, req.WorkingDir, p)
						if !valid {
							return p.deny("interpreter_input", arg, "")
						}
						if target != "" {
							return p.deny("interpreter_input", target, "")
						}
					} else if target := javaClasspathInputEntry(req.WorkingDir, arg, p); target != "" {
						return p.deny("interpreter_input", target, "")
					}
					continue
				}
				if strings.HasPrefix(arg, "-") {
					// Program-text options are unreviewable by construction
					// (their payload is code, not a screenable path):
					// -c/-e/-r inline programs (python3 -c, node -e, php
					// -r, perl -e, ruby -e/-r, awk -e), including combined
					// short groups (-cexec(...), -lc "...") and python -m
					// module indirection. The long forms (--eval=,
					// --print=, --execute=, --require=, --init-file=, ...)
					// are program text or startup hooks exactly the same
					// way, whatever value syntax they use.
					if arg != "-" && !strings.HasPrefix(arg, "--") && carriesProgramTextFlag(arg) && !benignInterpreterLauncherFlag(interpreter, arg) {
						return p.deny("interpreter_input", arg, "")
					}
					if strings.HasPrefix(arg, "--") {
						name := strings.TrimPrefix(arg, "--")
						if eq := strings.IndexByte(name, '='); eq >= 0 {
							name = name[:eq]
						}
						if longProgramTextOptions[name] {
							return p.deny("interpreter_input", arg, "")
						}
					}
					if interpreter == "java" {
						if arg == "-cp" || arg == "-classpath" || arg == "--class-path" || arg == "--module-path" || arg == "-p" || arg == "--upgrade-module-path" || arg == "--patch-module" {
							javaPathValueNext = arg
							continue
						}
						attachedPathHandled := false
						for _, option := range []string{"--class-path=", "--module-path=", "--upgrade-module-path=", "-p="} {
							if strings.HasPrefix(arg, option) {
								attachedPathHandled = true
								if target := javaClasspathInputEntry(req.WorkingDir, strings.TrimPrefix(arg, option), p); target != "" {
									return p.deny("interpreter_input", target, "")
								}
								break
							}
						}
						if attachedPathHandled {
							continue
						}
						if strings.HasPrefix(arg, "--patch-module=") {
							target, valid := javaPatchModuleInputEntry(strings.TrimPrefix(arg, "--patch-module="), req.WorkingDir, p)
							if !valid {
								return p.deny("interpreter_input", arg, "")
							}
							if target != "" {
								return p.deny("interpreter_input", target, "")
							}
							continue
						}
					}
					// An attached value is still a value: --flag=inputs/x,
					// -finputs/x and every short-flag suffix must be
					// screened against the tree, or a startup hook
					// (node --require=inputs/x.js) rides the flag token
					// itself past every layer.
					for _, value := range flagValueCandidates(arg) {
						if abs := p.canonical(req.WorkingDir, value); p.withinInputs(abs) {
							return p.deny("interpreter_input", abs, "")
						}
					}
					if arg == "-S" {
						// php's router script sits in the post-script operand
						// region after the -S addr pair — track it from the
						// prefix region too.
						phpSawServer = true
					}
					continue
				}
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
			// The post-script region is NOT safe to skip wholesale:
			//  - OPTION values: awk -f executes EVERY -f program in order,
			//    php -B/-F/-R/-E run their file arguments — an option naming
			//    tree material in ANY position is a program/hook, so every
			//    flag-attached value is screened.
			//  - POSITIONAL operands stay data for most interpreters
			//    (python3 gen.py <input> is a read), EXCEPT the php family
			//    where a positional after -S is a router script executed
			//    per request.
			if strings.HasPrefix(arg, "-") {
				for _, value := range flagValueCandidates(arg) {
					if stripped, ok := stripEnvAssignment(value); ok {
						value = stripped
					}
					if abs := p.canonical(req.WorkingDir, value); p.withinInputs(abs) {
						return p.deny("interpreter_input", abs, "")
					}
				}
				if arg == "-S" {
					phpSawServer = true
				}
				programFileNext = programFileFlag(arg)
				continue
			}
			if programFileNext {
				// Separated-form program-file flag (awk -f PROG, php -F/-B/-R/-E
				// PROG): the NEXT operand is a program file, not data.
				programFileNext = false
				if abs := p.canonical(req.WorkingDir, arg); p.withinInputs(abs) {
					return p.deny("interpreter_input", abs, "")
				}
				continue
			}
			if phpSawServer && isPHPFamily(path.Base(req.Command[interpreterOffset(req.Command)])) {
				// Only a php positional AFTER -S is the router script; a
				// plain php data read (php gen.php inputs/data.csv) is the
				// same data read python3 performs.
				if abs := p.canonical(req.WorkingDir, arg); p.withinInputs(abs) {
					return p.deny("interpreter_input", abs, "")
				}
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
				// Flag-attached values are operands too: make -finputs/x
				// and --file=inputs/x must not slip past the containment
				// rule on token-shape grounds.
				if strings.HasPrefix(arg, "-") && arg != "-" {
					for _, value := range flagValueCandidates(arg) {
						if abs := p.canonical(req.WorkingDir, value); p.withinInputs(abs) {
							return p.deny("input_target", abs, "")
						}
					}
				}
			}
		}
		if abs := p.canonical(req.WorkingDir, req.Command[0]); p.withinInputs(abs) {
			// Direct execution of the uploaded path itself.
			return p.deny("input_target", abs, "")
		}
		// Byte-copying commands (cp, mv, dd, tee, install, rsync) are NOT in
		// readOnlyCommands: their operands go through the full containment
		// screening above, so naming inputs-tree paths is refused here
		// outright. Only the narrow reader/inspector vocabulary may name
		// input material as data.
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
// copying command (cp, mv, dd, tee, install, rsync) are NOT here, and
// neither are rg (--pre runs a subprocess per file), less/more (the +!cmd
// initial command executes) — their operand set is screened by the
// containment rule above instead.
var readOnlyCommands = map[string]bool{
	"cat": true, "head": true, "tail": true,
	"grep": true, "egrep": true, "fgrep": true, "wc": true,
	"stat": true, "file": true, "ls": true, "du": true, "diff": true,
	"md5sum": true, "sha1sum": true, "sha256sum": true, "sha512sum": true,
	// sort is deliberately NOT here: GNU coreutils sort --compress-program=PROG
	// executes PROG through sh -c, so its operands stay under the full
	// flag-value and containment screening.
	"uniq": true, "cut": true, "tr": true,
}

// InterpreterPrefixStatus classifies the outcome of the wrapper-aware
// interpreter scan: a plain non-interpreter command, an interpreter with the
// operand offset, or an inconclusive wrapper whose option structure the
// scanner cannot conclusively parse (the policy then fails closed instead of
// degrading to the weaker non-interpreter screening).
type InterpreterScanStatus int

const (
	InterpreterScanNone InterpreterScanStatus = iota
	InterpreterScanInterpreter
	InterpreterScanInconclusive
)

// InterpreterPrefix reports whether the argv launches an interpreter that
// executes a following path argument, and the offset where its operands
// start. See InterpreterPrefixStatus for the tri-state form the policy uses.
func InterpreterPrefix(command []string) (bool, int) {
	status, offset := InterpreterPrefixStatus(command)
	return status == InterpreterScanInterpreter, offset
}

// longProgramTextOption lists long options whose value is program text or a
// startup hook: unreviewable by construction, exactly like -c/-e/-r/-m.
var longProgramTextOptions = map[string]bool{
	"eval": true, "print": true, "execute": true, "exec": true,
	"require": true, "include": true, "import": true,
	"init-file": true, "rcfile": true, "exrc": true, "lua": true,
}

// flagValueCandidates returns every substring of a flag token that could be
// an attached value: the part after the first '=', plus each short-flag
// suffix (-finputs/x → inputs/x). Only candidates that resolve inside the
// screened tree matter, so over-generation is safe (it can only add denies).
func flagValueCandidates(arg string) []string {
	var out []string
	if arg == "" || arg == "-" {
		return nil
	}
	if eq := strings.IndexByte(arg, '='); eq >= 0 && eq+1 < len(arg) {
		value := arg[eq+1:]
		out = append(out, value)
		// The value may itself carry a VAR= assignment prefix
		// (env --split-string=BASH_ENV=inputs/x): strip it so the ASSIGNED
		// path is what gets screened.
		if stripped, ok := stripEnvAssignment(value); ok {
			out = append(out, stripped)
		}
	}
	if !strings.HasPrefix(arg, "--") {
		// Every short-flag suffix participates: the flag's letter count is
		// unknowable from the token alone, and only a suffix that resolves
		// inside the screened tree produces a denial anyway.
		for i := 1; i < len(arg); i++ {
			if arg[i] == '=' || arg[i] == '-' {
				continue
			}
			out = append(out, arg[i:])
		}
	}
	return out
}

// wrapperValueFlags lists each wrapper's short flags that consume one value
// operand; every other short flag is valueless. Anything outside these tables
// (unknown short flags, long options other than "--") makes the scan
// inconclusive: guessing the arity would silently misplace the real program.
var wrapperValueFlags = map[string]map[string]bool{
	"timeout": {"-k": true, "-s": true},
	"env":     {"-u": true, "-S": true},
	"nice":    {"-n": true},
	"time":    {"-o": true, "-f": true},
	"xargs":   {"-I": true, "-D": true, "-E": true, "-n": true, "-P": true, "-s": true},
	"stdbuf":  {"-i": true, "-o": true, "-e": true},
	"setsid":  {},
	"nohup":   {},
}

// InterpreterPrefixStatus scans past wrapper launchers with their option
// grammar modeled per wrapper (timeout/env/nohup/xargs/nice/stdbuf/setsid/
// time), tolerant of version-suffixed interpreter names (python3.11, php8.2,
// lua5.4, tclsh8.6, mawk ...). It is exported so server-side adapters reuse
// the same detection for their own evidence assembly.
func InterpreterPrefixStatus(command []string) (InterpreterScanStatus, int) {
	for offset := 0; offset < len(command); {
		base := path.Base(command[offset])
		if isInterpreterName(base) {
			return InterpreterScanInterpreter, offset + 1
		}
		if !wrapperCommands[base] {
			return InterpreterScanNone, 0
		}
		offset++
		valueFlags := wrapperValueFlags[base]
		switch base {
		case "env":
			// Skip VAR=value assignments and env's own flags; a flag-bearing
			// env used to hide the interpreter must not defeat detection.
			for offset < len(command) {
				next := command[offset]
				if strings.Contains(next, "=") && !strings.HasPrefix(next, "-") {
					offset++
					continue
				}
				if strings.HasPrefix(next, "-") && next != "-" {
					if next == "--" {
						offset++
						break
					}
					if valueFlags[next] {
						offset++
						if offset < len(command) {
							offset++
						}
						continue
					}
					if strings.HasPrefix(next, "--") {
						if strings.Contains(next, "=") {
							offset++
							continue
						}
						return InterpreterScanInconclusive, 0
					}
					// Short group: every letter before the last must be the
					// known valueless 'i'; a value-taking letter ('u', 'S')
					// is only conclusive as the LAST letter, where it
					// consumes one following operand (-iu F, -S "x"). Any
					// other letter makes the arity unknowable — refuse the
					// guess (the policy fails closed).
					group := next[1:]
					if group == "" {
						offset++
						continue
					}
					for i := 0; i < len(group)-1; i++ {
						if group[i] != 'i' {
							return InterpreterScanInconclusive, 0
						}
					}
					switch group[len(group)-1] {
					case 'i':
						offset++
						continue
					case 'u', 'S':
						offset++
						if offset < len(command) {
							offset++
						}
						continue
					default:
						return InterpreterScanInconclusive, 0
					}
				}
				break
			}
		case "timeout":
			for offset < len(command) {
				next := command[offset]
				if next == "--" {
					offset++
					break
				}
				if !strings.HasPrefix(next, "-") {
					// The mandatory duration operand.
					offset++
					break
				}
				if valueFlags[next] {
					offset++
					if offset < len(command) {
						offset++
					}
					continue
				}
				if strings.Contains(next, "=") {
					offset++
					continue
				}
				if len(next) > 2 && valueFlags[next[:2]] {
					offset++
					continue
				}
				if len(next) == 2 || strings.HasPrefix(next, "--") {
					return InterpreterScanInconclusive, 0
				}
				offset++
			}
		case "nice":
			for offset < len(command) {
				next := command[offset]
				if next == "--" {
					offset++
					break
				}
				if !strings.HasPrefix(next, "-") {
					// Optional numeric priority operand.
					if _, err := strconv.Atoi(strings.TrimPrefix(next, "+")); err == nil {
						offset++
					}
					break
				}
				if valueFlags[next] {
					offset++
					if offset < len(command) {
						offset++
					}
					continue
				}
				if strings.Contains(next, "=") {
					offset++
					continue
				}
				if len(next) > 2 && valueFlags[next[:2]] {
					offset++
					continue
				}
				if len(next) == 2 || strings.HasPrefix(next, "--") {
					return InterpreterScanInconclusive, 0
				}
				offset++
			}
		default:
			// xargs / stdbuf / setsid / time / nohup: modeled value flags,
			// everything else inconclusive.
			for offset < len(command) {
				next := command[offset]
				if next == "--" {
					offset++
					break
				}
				if !strings.HasPrefix(next, "-") {
					break
				}
				if valueFlags[next] {
					offset++
					if offset < len(command) {
						offset++
					}
					continue
				}
				if strings.Contains(next, "=") {
					offset++
					continue
				}
				if len(next) > 2 && valueFlags[next[:2]] {
					offset++
					continue
				}
				if len(next) == 2 || strings.HasPrefix(next, "--") {
					return InterpreterScanInconclusive, 0
				}
				offset++
			}
		}
	}
	return InterpreterScanNone, 0
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

// hasExecForwardFlag reports whether any argv token is find's execution
// forwarding flag (separate or =-attached forms).
// programFileFlag reports whether an option token selects a PROGRAM FILE in
// a multi-program interpreter family: awk/mawk/gawk -f/--file, php -B/-F/-R/-E
// (both attached and bare separated forms).
func programFileFlag(arg string) bool {
	switch arg {
	case "-f", "-F", "-B", "-R", "-E":
		return true
	}
	return strings.HasPrefix(arg, "--file=") || arg == "--file"
}

// isPHPFamily reports whether the interpreter is php (any version suffix):
// a POSITIONAL operand after php -S is a router script executed per request,
// unlike the plain data operands of most interpreters.
func isPHPFamily(base string) bool {
	return base == "php" || strings.HasPrefix(base, "php")
}

// interpreterOffset returns the index of the interpreter itself (0 when no
// wrapper preceded it).
func interpreterOffset(command []string) int {
	for offset := 0; offset < len(command); offset++ {
		base := path.Base(command[offset])
		if isInterpreterName(base) {
			return offset
		}
		if !wrapperCommands[base] {
			return 0
		}
	}
	return 0
}

// hasStdinPlaceholder reports whether any argv token carries the xargs
// replace-str placeholder {}: at runtime each stdin line is substituted into
// it and executed as the wrapped command's operand — a stdin-content
// forwarder exactly like find -exec.
func hasStdinPlaceholder(command []string) bool {
	for _, arg := range command {
		if strings.Contains(arg, "{}") {
			return true
		}
	}
	return false
}

func hasExecForwardFlag(command []string) bool {
	for _, arg := range command {
		if arg == "-exec" || arg == "-execdir" || strings.HasPrefix(arg, "-exec=") || strings.HasPrefix(arg, "-execdir=") {
			return true
		}
	}
	return false
}

// carriesProgramTextFlag reports whether a short-option group (already
// known to start with a single '-' and not be the stdin marker) selects a
// program-text or module-loading flag: c (inline program), e (inline
// program for node/php/perl/ruby/awk), r (php program / ruby require), m
// (python module-by-name indirection). Combined groups (-lc, -cexec(...))
// match too; long options are excluded by the caller.
func carriesProgramTextFlag(arg string) bool {
	for _, r := range arg[1:] {
		switch r {
		case 'c', 'e', 'r', 'm':
			return true
		}
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') {
			return false // payload text after combined flags ends the group
		}
	}
	return false
}

// benignInterpreterLauncherFlag exempts common options whose letters overlap
// the program-text heuristic but whose meaning is a normal launcher setting.
// Keep these exact and interpreter-specific so code-bearing flags such as
// python -c, node -e, ruby -r, and python -m remain fail-closed.
func benignInterpreterLauncherFlag(interpreter, arg string) bool {
	switch interpreter {
	case "java":
		switch arg {
		case "-jar", "-cp", "-classpath", "-ea", "-m":
			return true
		}
		return strings.HasPrefix(arg, "-ea:")
	case "pwsh", "powershell":
		return strings.EqualFold(arg, "-File")
	case "bash":
		return arg == "-e"
	}
	return false
}

// javaClasspathInputEntry checks each path element separately because a
// classpath is a colon-delimited list, not one filesystem path.
func javaClasspathInputEntry(workingDir, classpath string, policy *InputExecutionPolicy) string {
	for _, entry := range strings.Split(classpath, ":") {
		if abs := policy.canonical(workingDir, entry); policy.withinInputs(abs) {
			return abs
		}
	}
	return ""
}

// javaArgFileReference recognizes the launcher's @argfile token. The
// policy rejects these tokens even outside the inputs tree because it cannot
// inspect the referenced file's contents at this seam.
func javaArgFileReference(token, workingDir string, policy *InputExecutionPolicy) (string, bool) {
	token = trimQuotes(strings.TrimSpace(token))
	if token == "" {
		return "", false
	}
	if value, ok := stripEnvAssignment(token); ok {
		return javaArgFileReference(value, workingDir, policy)
	}
	if !strings.HasPrefix(token, "@") || len(token) == 1 {
		return "", false
	}
	target := policy.canonical(workingDir, strings.TrimPrefix(token, "@"))
	return target, true
}

func javaEnvAssignment(token string) (key, value string, ok bool) {
	prefix := envAssignment(token)
	if prefix == "" {
		return "", "", false
	}
	return strings.TrimSuffix(prefix, "="), token[len(prefix):], true
}

func hasJavaLauncherToken(tokens []string) bool {
	for _, token := range tokens {
		if path.Base(token) == "java" {
			return true
		}
	}
	return false
}

// javaPatchModuleInputEntry parses module=path[:path]. The selector must be
// present and every path entry is checked independently.
func javaPatchModuleInputEntry(value, workingDir string, policy *InputExecutionPolicy) (string, bool) {
	module, paths, ok := strings.Cut(value, "=")
	if !ok || strings.TrimSpace(module) == "" || paths == "" {
		return "", false
	}
	for _, entry := range strings.Split(paths, ":") {
		if entry == "" {
			return "", false
		}
		if abs := policy.canonical(workingDir, entry); policy.withinInputs(abs) {
			return abs, true
		}
	}
	return "", true
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

// stripEnvAssignment removes a leading VAR= prefix from a flag VALUE
// candidate: "BASH_ENV=inputs/x" screens the assigned path, not the
// concatenation with WorkingDir that never lands inside the tree.
func stripEnvAssignment(value string) (string, bool) {
	prefix := envAssignment(value)
	if prefix == "" {
		return value, false
	}
	return value[len(prefix):], true
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
