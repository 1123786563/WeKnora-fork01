package craft

import (
	"path"
	"strings"
	"testing"
)

// The OCR hardening round: every bypass shape reported against the argv and
// shell screening must now be denied, while the legitimate shapes (reading
// uploaded data, generated-code execution) stay allowed.

func TestInputCodePolicyDeniesWrapperAndVersionedInterpreterBypasses(t *testing.T) {
	policy := newT03Policy(t)
	script := t03ScriptPath()
	cases := [][]string{
		{"timeout", "10", "python3", script},
		{"env", "VAR=1", "python3", script},
		{"nohup", "python3", script},
		{"xargs", "python3", script},
		{"python3.11", script},
		{"php8.2", script},
		{"lua5.4", script},
		{"tclsh8.6", script},
		{"mawk", "-f", script},
	}
	for _, command := range cases {
		decision := policy.Review(InputExecutionRequest{Command: command, WorkingDir: "/workspace"})
		if decision.Allowed {
			t.Fatalf("wrapped/versioned interpreter bypass must be denied: %v -> %+v", command, decision)
		}
	}
}

func TestInputCodePolicyDeniesArgvEmbeddedShellCommand(t *testing.T) {
	policy := newT03Policy(t)
	script := t03ScriptPath()
	cases := [][]string{
		{"bash", "-c", "python3 " + script},
		{"env", "sh", "-c", "python3 " + script},
		{"timeout", "5", "bash", "-c", "python3 " + script},
	}
	for _, command := range cases {
		decision := policy.Review(InputExecutionRequest{Command: command, WorkingDir: "/workspace"})
		if decision.Allowed {
			t.Fatalf("argv-embedded shell command must be denied: %v -> %+v", command, decision)
		}
	}
}

func TestInputCodePolicyDeniesShellWithoutAdapterEvidence(t *testing.T) {
	policy := newT03Policy(t)

	// A shell expression without adapter-normalized evidence is unreviewable
	// and fails closed — even when it looks benign.
	benign := policy.Review(InputExecutionRequest{
		Shell: true, CommandText: "echo hi", WorkingDir: "/workspace",
	})
	if benign.Allowed {
		t.Fatalf("evidence-free shell expression must fail closed: %+v", benign)
	}

	// With adapter evidence (resolved target outside the tree and a digest
	// that is not uploaded material) the screened layers above decide.
	evidence := policy.Review(InputExecutionRequest{
		Shell: true, CommandText: "run-task.sh",
		WorkingDir:         "/workspace/rv-abc/output",
		ResolvedTargetPath: "/workspace/rv-abc/output/run-task.sh",
		TargetSHA256:       strings.Repeat("f", 64),
	})
	if !evidence.Allowed {
		t.Fatalf("evidence-carrying generated shell script must execute: %+v", evidence)
	}
}

func TestInputCodePolicyDeniesEnvironmentStartupHooks(t *testing.T) {
	policy := newT03Policy(t)
	for key := range map[string]string{
		"BASH_ENV": "", "ENV": "", "PYTHONSTARTUP": "", "NODE_OPTIONS": "", "RUBYOPT": "", "PERL5OPT": "",
	} {
		decision := policy.Review(InputExecutionRequest{
			Command:     []string{"bash"},
			WorkingDir:  "/workspace",
			Environment: map[string]string{key: t03ScriptPath()},
		})
		if decision.Allowed {
			t.Fatalf("environment hook %s pointing at uploaded material must be denied: %+v", key, decision)
		}
	}
}

func TestInputCodePolicyDeniesInterpreterStdinProgramForms(t *testing.T) {
	policy := newT03Policy(t)
	for _, arg := range []string{"-", "/dev/stdin"} {
		decision := policy.Review(InputExecutionRequest{
			Command: []string{"python3", arg}, WorkingDir: "/workspace",
		})
		if decision.Allowed {
			t.Fatalf("interpreter reading its program from %q must be denied: %+v", arg, decision)
		}
	}
}

func TestInputCodePolicyAllowsDataOperandsAndReadsButScreensCopies(t *testing.T) {
	policy := newT03Policy(t)
	script := t03ScriptPath()

	// Data operands after the script are reads, not execution.
	data := policy.Review(InputExecutionRequest{
		Command: []string{"python3", "/workspace/app/main.py", script}, WorkingDir: "/workspace",
	})
	if !data.Allowed {
		t.Fatalf("passing uploaded data to a generated script is a read, not execution: %+v", data)
	}

	// Read-only utilities may name uploaded material.
	read := policy.Review(InputExecutionRequest{
		Command: []string{"cat", script}, WorkingDir: "/workspace",
	})
	if !read.Allowed {
		t.Fatalf("reading uploaded material with a read-only utility must stay allowed: %+v", read)
	}

	// Copying commands are not read-only utilities: their operands are
	// screened so uploaded bytes cannot be staged for a later execution.
	copy := policy.Review(InputExecutionRequest{
		Command: []string{"cp", script, "/tmp/run.py"}, WorkingDir: "/workspace",
	})
	if copy.Allowed {
		t.Fatalf("copying uploaded material out of the input tree must be screened: %+v", copy)
	}
}

func TestInputCodeShellTokenNormalizationCatchesIndirection(t *testing.T) {
	policy := newT03Policy(t)
	script := t03ScriptPath()

	// The path appears only inside an assignment; the stripped assignment
	// value must still be screened (and the evidence-free shell form fails
	// closed regardless).
	indirect := policy.Review(InputExecutionRequest{
		Shell: true, CommandText: "f=" + script + "; python3 \"$f\"", WorkingDir: "/workspace",
	})
	if indirect.Allowed {
		t.Fatalf("assignment indirection must not hide the inputs path: %+v", indirect)
	}

	// Token normalization used by the embedded-command screening: adjacent
	// quote splices expose the real path after quote removal.
	spliced := policy.Review(InputExecutionRequest{
		Command:    []string{"bash", "-c", "python3 " + strings.ReplaceAll(script, "inputs", `in"pu"ts`)},
		WorkingDir: "/workspace",
	})
	if spliced.Allowed {
		t.Fatalf("adjacent-quote splices must not hide the inputs path: %+v", spliced)
	}

	// Backslash-escaped path segments expose the real path after escape
	// flattening.
	escaped := policy.Review(InputExecutionRequest{
		Command:    []string{"bash", "-c", "python3 " + strings.Replace(script, "inputs", `in\puts`, 1)},
		WorkingDir: "/workspace",
	})
	if escaped.Allowed {
		t.Fatalf("backslash escapes must not hide the inputs path: %+v", escaped)
	}
}

func TestInputCodeClassifySkipsBOMBeforeShebang(t *testing.T) {
	label := ClassifyInputCode("boot.py", append([]byte("\xef\xbb\xbf"), []byte("#!/usr/bin/env python3\nprint(1)\n")...))
	if !label.IsCode || label.Reason != "shebang" {
		t.Fatalf("BOM-prefixed shebang must still be classified as code, got %+v", label)
	}
}

// Round 4: execution-capable tools must not sit in the read-only whitelist
// (rg --pre runs a subprocess per file; less/more +!cmd run commands), and
// every program-text or indirection form must fail closed.
func TestInputCodePolicyScreensExecCapableReadOnlyCandidates(t *testing.T) {
	policy := newT03Policy(t)
	script := t03ScriptPath()
	cases := [][]string{
		{"rg", "--pre", script, "--", "/workspace/notes.txt"},
		{"rg", script},
		{"less", script},
		{"more", script},
	}
	for _, command := range cases {
		decision := policy.Review(InputExecutionRequest{Command: command, WorkingDir: "/workspace"})
		if decision.Allowed {
			t.Fatalf("execution-capable reader/wrapper form must be screened: %v -> %+v", command, decision)
		}
	}
	// The whitelist itself stays usable for pure reads.
	if read := policy.Review(InputExecutionRequest{Command: []string{"cat", script}, WorkingDir: "/workspace"}); !read.Allowed {
		t.Fatalf("cat must remain a whitelisted read: %+v", read)
	}
}

func TestInputCodePolicyDeniesInlineProgramAndRunSubcommandForms(t *testing.T) {
	policy := newT03Policy(t)
	script := t03ScriptPath()
	cases := [][]string{
		{"python3", "-c", "exec(open('" + script + "').read())"},
		{"python3", "-cexec(open('" + script + "').read())"},
		{"node", "-e", "require('" + script + "')"},
		{"php", "-r", "include '" + script + "';"},
		{"perl", "-e", "do '" + script + "'"},
		{"ruby", "-e", "load '" + script + "'"},
		{"awk", "-e", "'" + script + "'"},
		{"bash", "-lc", "python3 " + script},
		{"sh", "-ic", "python3 " + script},
		{"deno", "run", script},
		{"bun", "run", script},
		{"env", "-i", "python3", script},
		{"python3", "-m", "analyze"},
	}
	for _, command := range cases {
		decision := policy.Review(InputExecutionRequest{Command: command, WorkingDir: "/workspace"})
		if decision.Allowed {
			t.Fatalf("inline-program / run-subcommand / env-flag form must fail closed: %v -> %+v", command, decision)
		}
	}
}

func TestInputCodePolicyScreensWrapperPrefixAssignmentsAndPathLists(t *testing.T) {
	policy := newT03Policy(t)
	script := t03ScriptPath()

	// argv-form startup-hook assignment rides in the wrapper prefix.
	argvHook := policy.Review(InputExecutionRequest{
		Command: []string{"env", "BASH_ENV=" + script, "bash", "/workspace/gen.sh"}, WorkingDir: "/workspace",
	})
	if argvHook.Allowed {
		t.Fatalf("wrapper-prefix assignment pointing at uploaded material must be denied: %+v", argvHook)
	}

	// Path-list environment entries are screened per colon segment, in both
	// the argv prefix and the environment map.
	argvPathList := policy.Review(InputExecutionRequest{
		Command:    []string{"env", "PYTHONPATH=/usr/lib/python3:" + path.Dir(script), "python3", "/workspace/gen.py"},
		WorkingDir: "/workspace",
	})
	if argvPathList.Allowed {
		t.Fatalf("argv path-list containing the inputs tree must be denied: %+v", argvPathList)
	}
	envPathList := policy.Review(InputExecutionRequest{
		Command:     []string{"python3", "/workspace/gen.py"},
		WorkingDir:  "/workspace",
		Environment: map[string]string{"PYTHONPATH": "/usr/lib/python3:" + path.Dir(script)},
	})
	if envPathList.Allowed {
		t.Fatalf("environment path-list containing the inputs tree must be denied: %+v", envPathList)
	}
}

func TestInputCodePolicyAllowsScriptOwnFlagsAfterScriptOperand(t *testing.T) {
	policy := newT03Policy(t)
	decision := policy.Review(InputExecutionRequest{
		Command: []string{"python3", "/workspace/rv-x/train.py", "-c", "config.yaml"}, WorkingDir: "/workspace",
	})
	if !decision.Allowed {
		t.Fatalf("the script's own -c argument is data, not an interpreter shell flag: %+v", decision)
	}
}

func TestInputCodeRefusalNeverRendersDegenerateTarget(t *testing.T) {
	policy := newT03Policy(t)
	decision := policy.Review(InputExecutionRequest{
		Shell: true, CommandText: "echo hi", WorkingDir: "/workspace",
	})
	if decision.Allowed {
		t.Fatalf("evidence-free shell must deny")
	}
	refusal := decision.Refusal()
	if strings.Contains(refusal, "(matched )") {
		t.Fatalf("refusal must not render a degenerate empty target: %q", refusal)
	}
	if !strings.Contains(refusal, "unreviewable command form") {
		t.Fatalf("refusal must carry the stable fallback identity: %q", refusal)
	}
}
