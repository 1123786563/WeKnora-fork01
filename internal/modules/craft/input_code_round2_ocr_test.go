package craft

// The OCR hardening rounds: every bypass shape reported against the argv
// and shell screening must be denied, while the legitimate shapes (reading
// uploaded data, generated-code execution) stay allowed.
//
// Round 2 covers wrappers carrying option operands (nice -n 5, timeout -k 5
// 10, env -iu) that used to break interpreter detection and let inline
// program text ride the weaker non-interpreter branch, plus =-attached flag
// values (--require=inputs/x.js, -finputs/x, make -finputs/Makefile) that
// bypassed every lexical layer.

import (
	"testing"
)

// round2Bypass lists every shape that must now be refused.
var round2Bypass = [][]string{
	// Wrapper-option shapes that hide the interpreter from detection.
	{"nice", "-n", "5", "python3", "-c", "exec(open('in'+'puts/x.py').read())"},
	{"timeout", "-k", "5", "10", "python3", "-c", "print(1)"},
	{"env", "-iu", "F", "python3", "-c", "print(1)"},
	{"setsid", "-w", "python3", "-c", "print(1)"},
	{"stdbuf", "-oL", "python3", "-c", "print(1)"},
	{"time", "-p", "python3", "-c", "print(1)"},
	{"xargs", "-I", "{}", "python3", "-c", "print(1)"},
}

func TestInputCodePolicyDeniesOptionBearingWrappers(t *testing.T) {
	policy := newT03Policy(t)
	for _, command := range round2Bypass {
		decision := policy.Review(InputExecutionRequest{Command: command, WorkingDir: "/workspace"})
		if decision.Allowed {
			t.Fatalf("option-bearing wrapper bypass must be denied: %v -> %+v", command, decision)
		}
	}
}

func TestInputCodePolicyDeniesAttachedFlagValues(t *testing.T) {
	policy := newT03Policy(t)
	script := t03ScriptPath()
	bypass := [][]string{
		{"node", "--require=" + script, "main.js"},
		{"bash", "--init-file=" + script},
		{"make", "-f" + script},
		{"node", "--eval=console.log(1)"},
		{"python3", "-m=" + script},
		{"gcc", "-f" + script},
	}
	for _, command := range bypass {
		decision := policy.Review(InputExecutionRequest{Command: command, WorkingDir: "/workspace"})
		if decision.Allowed {
			t.Fatalf("attached-value bypass must be denied: %v -> %+v", command, decision)
		}
	}
}

func TestInputCodePolicyKeepsLegitimateShapesAllowed(t *testing.T) {
	policy := newT03Policy(t)
	script := t03ScriptPath()

	// Read-only utilities naming uploaded DATA stay allowed, including a
	// =-attached value of their own flag.
	read := policy.Review(InputExecutionRequest{Command: []string{"grep", "--file=" + script, "x"}, WorkingDir: "/workspace"})
	if !read.Allowed {
		t.Fatalf("read-only utilities naming input material must stay allowed: %+v", read)
	}
	// Wrapped execution of GENERATED code outside the inputs tree stays
	// allowed, including wrappers with option operands.
	generated := policy.Review(InputExecutionRequest{Command: []string{"timeout", "10", "python3", "gen/main.py"}, WorkingDir: "/workspace"})
	if !generated.Allowed {
		t.Fatalf("wrapped execution of generated code must stay allowed: %+v", generated)
	}
	niced := policy.Review(InputExecutionRequest{Command: []string{"nice", "-n", "5", "python3", "gen/main.py"}, WorkingDir: "/workspace"})
	if !niced.Allowed {
		t.Fatalf("nice-wrapped generated code must stay allowed: %+v", niced)
	}
}
