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

func TestInputCodePolicyDeniesFindExecForwarding(t *testing.T) {
	policy := newT03Policy(t)
	cases := [][]string{
		{"find", ".", "-type", "f", "-name", "*.py", "-exec", "python3", "{}", ";"},
		{"find", ".", "-execdir", "python3", "{}", "+"},
	}
	for _, command := range cases {
		decision := policy.Review(InputExecutionRequest{Command: command, WorkingDir: "/workspace"})
		if decision.Allowed || decision.Reason != "exec_forward" {
			t.Fatalf("find -exec forwarding must be denied: %v -> %+v", command, decision)
		}
	}
	// Plain find (no execution forwarding) stays allowed.
	plain := policy.Review(InputExecutionRequest{Command: []string{"find", ".", "-name", "x"}, WorkingDir: "/workspace"})
	if !plain.Allowed {
		t.Fatalf("plain find must stay allowed: %+v", plain)
	}
}

func TestInputCodePolicyDeniesRound5SmugglingShapes(t *testing.T) {
	policy := newT03Policy(t)
	script := t03ScriptPath()
	cases := [][]string{
		// awk executes EVERY -f program in order.
		{"awk", "-f", "gen.awk", "-f", script},
		{"awk", "--file=" + script, "data"},
		// php router / per-request program files.
		{"php", "-S", "127.0.0.1:8080", script},
		{"php", "-B", "gen.php", "-F", script},
		// env --split-string attaches a VAR= startup hook value.
		{"env", "--split-string=BASH_ENV=" + script, "bash", "gen.sh"},
		// xargs stdin-content forwarder.
		{"xargs", "-I{}", "bash", "{}"},
		// "run" itself is the uploaded script when cwd sits in the tree.
		// (covered structurally: run shifts only after its own screening)
	}
	for _, command := range cases {
		decision := policy.Review(InputExecutionRequest{Command: command, WorkingDir: "/workspace"})
		if decision.Allowed {
			t.Fatalf("round-5 smuggling shape must be denied: %v -> %+v", command, decision)
		}
	}
	// Wrap-up OCR F36: a php positional that is NOT after -S is the same
	// data read python3 performs — only the -S router script is denied
	// (covered above).
	if decision := policy.Review(InputExecutionRequest{Command: []string{"php", "gen.php", script}, WorkingDir: "/workspace"}); !decision.Allowed {
		t.Fatalf("php gen.php <input> is a data read, not a router script: %+v", decision)
	}

	// Data operand after a generated script stays a READ (python semantics).
	data := policy.Review(InputExecutionRequest{Command: []string{"python3", "/workspace/app/main.py", script}, WorkingDir: "/workspace"})
	if !data.Allowed {
		t.Fatalf("positional data operand after a clean script is a read: %+v", data)
	}
}

func TestInputCodePolicyDeniesEnvironmentAttachedHooks(t *testing.T) {
	policy := newT03Policy(t)
	script := t03ScriptPath()
	env := map[string]string{
		"NODE_OPTIONS": "--require=" + script,
		"RUBYOPT":      "-r" + script,
		"PERL5OPT":     "-I" + t03PythonDigest[:8],
	}
	decision := policy.Review(InputExecutionRequest{
		Command: []string{"node", "gen.js"}, WorkingDir: "/workspace", Environment: env,
	})
	if decision.Allowed {
		t.Fatalf("environment-attached startup hooks must be denied: %+v", decision)
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

func TestInputCodePolicyAllowsInterpreterLauncherFlags(t *testing.T) {
	policy := newT03Policy(t)
	cases := [][]string{
		{"java", "-jar", "app.jar"},
		{"java", "-cp", "lib", "Main"},
		{"java", "-classpath", "lib", "Main"},
		{"java", "-ea", "Main"},
		{"pwsh", "-File", "gen.ps1"},
		{"powershell", "-File", "gen.ps1"},
		{"bash", "-e", "gen.sh"},
	}
	for _, command := range cases {
		decision := policy.Review(InputExecutionRequest{Command: command, WorkingDir: "/workspace"})
		if !decision.Allowed {
			t.Errorf("standard interpreter launcher flag must be allowed: %v -> %+v", command, decision)
		}
	}
}

func TestInputCodePolicyScreensEveryJavaClasspathEntry(t *testing.T) {
	policy := newT03Policy(t)
	for _, flag := range []string{"-cp", "-classpath"} {
		unsafe := policy.Review(InputExecutionRequest{
			Command:    []string{"java", flag, "lib:/workspace/inputs/payload.jar", "Main"},
			WorkingDir: "/workspace",
		})
		if unsafe.Allowed || unsafe.Reason != "interpreter_input" {
			t.Errorf("classpath entry under inputs must be denied for %s: %+v", flag, unsafe)
		}
	}

	for _, flag := range []string{"-cp", "-classpath"} {
		decision := policy.Review(InputExecutionRequest{
			Command:    []string{"java", flag, "lib:/workspace/generated", "Main"},
			WorkingDir: "/workspace",
		})
		if !decision.Allowed {
			t.Errorf("generated classpath entries must stay allowed for %s: %+v", flag, decision)
		}
	}
}

func TestInputCodePolicyStillDeniesProgramTextFlags(t *testing.T) {
	policy := newT03Policy(t)
	cases := [][]string{
		{"python3", "-c", "print(1)"},
		{"node", "-e", "console.log(1)"},
		{"php", "-r", "echo 1"},
		{"ruby", "-e", "puts 1"},
		{"python3", "-m", "some.module"},
	}
	for _, command := range cases {
		decision := policy.Review(InputExecutionRequest{Command: command, WorkingDir: "/workspace"})
		if decision.Allowed || decision.Reason != "interpreter_input" {
			t.Errorf("program text flag must remain denied: %v -> %+v", command, decision)
		}
	}
}

func TestInputCodePolicyScreensLongJavaClasspathEntries(t *testing.T) {
	policy := newT03Policy(t)
	unsafe := []struct {
		name string
		argv []string
	}{
		{"separated value", []string{"java", "--class-path", "lib:/workspace/inputs/payload.jar", "Main"}},
		{"attached value", []string{"java", "--class-path=lib:/workspace/inputs/payload.jar", "Main"}},
	}
	for _, tc := range unsafe {
		t.Run(tc.name, func(t *testing.T) {
			decision := policy.Review(InputExecutionRequest{Command: tc.argv, WorkingDir: "/workspace"})
			if decision.Allowed || decision.Reason != "interpreter_input" {
				t.Fatalf("long Java classpath input entry must be denied: %v -> %+v", tc.argv, decision)
			}
		})
	}
	for _, argv := range [][]string{
		{"java", "--class-path", "lib:/workspace/generated", "Main"},
		{"java", "--class-path=lib:/workspace/generated", "Main"},
	} {
		decision := policy.Review(InputExecutionRequest{Command: argv, WorkingDir: "/workspace"})
		if !decision.Allowed {
			t.Errorf("generated long Java classpath must stay allowed: %v -> %+v", argv, decision)
		}
	}
}

func TestInputCodePolicyUsesDetectedJavaInterpreterAfterWrappers(t *testing.T) {
	policy := newT03Policy(t)
	unsafe := [][]string{
		{"timeout", "10", "java", "--class-path", "lib:/workspace/inputs/payload.jar", "Main"},
		{"env", "MODE=test", "java", "--class-path=lib:/workspace/inputs/payload.jar", "Main"},
	}
	for _, argv := range unsafe {
		decision := policy.Review(InputExecutionRequest{Command: argv, WorkingDir: "/workspace"})
		if decision.Allowed || decision.Reason != "interpreter_input" {
			t.Errorf("wrapped Java classpath input entry must be denied: %v -> %+v", argv, decision)
		}
	}

	benign := [][]string{
		{"timeout", "10", "java", "-ea:com.acme...", "-cp", "lib", "Main"},
		{"env", "MODE=test", "java", "-ea:com.acme...", "-classpath", "lib", "Main"},
	}
	for _, argv := range benign {
		decision := policy.Review(InputExecutionRequest{Command: argv, WorkingDir: "/workspace"})
		if !decision.Allowed {
			t.Errorf("documented Java assertion flag must be allowed after wrapper: %v -> %+v", argv, decision)
		}
	}
}

func TestInputCodePolicyScreensJavaModulePathEntries(t *testing.T) {
	policy := newT03Policy(t)
	unsafe := [][]string{
		{"java", "--module-path", "lib:/workspace/inputs/mod.jar", "-m", "app/main"},
		{"java", "--module-path=lib:/workspace/inputs/mod.jar", "-m", "app/main"},
		{"java", "-p", "lib:/workspace/inputs/mod.jar", "-m", "app/main"},
		{"java", "-p=lib:/workspace/inputs/mod.jar", "-m", "app/main"},
		{"java", "--upgrade-module-path", "lib:/workspace/inputs/mod.jar", "-m", "app/main"},
		{"java", "--upgrade-module-path=lib:/workspace/inputs/mod.jar", "-m", "app/main"},
	}
	for _, argv := range unsafe {
		decision := policy.Review(InputExecutionRequest{Command: argv, WorkingDir: "/workspace"})
		if decision.Allowed || decision.Reason != "interpreter_input" {
			t.Errorf("Java module path input entry must be denied: %v -> %+v", argv, decision)
		}
	}

	generated := [][]string{
		{"java", "--module-path", "lib:/workspace/generated", "-m", "app/main"},
		{"java", "--module-path=lib:/workspace/generated", "-m", "app/main"},
		{"java", "-p", "lib:/workspace/generated", "-m", "app/main"},
		{"java", "-p=lib:/workspace/generated", "-m", "app/main"},
		{"java", "--upgrade-module-path", "lib:/workspace/generated", "-m", "app/main"},
		{"java", "--upgrade-module-path=lib:/workspace/generated", "-m", "app/main"},
	}
	for _, argv := range generated {
		decision := policy.Review(InputExecutionRequest{Command: argv, WorkingDir: "/workspace"})
		if !decision.Allowed {
			t.Errorf("generated Java module path must stay allowed: %v -> %+v", argv, decision)
		}
	}
}

func TestInputCodePolicyScreensJavaPatchModulePathEntries(t *testing.T) {
	policy := newT03Policy(t)
	unsafe := [][]string{
		{"java", "--patch-module", "java.base=lib:/workspace/inputs/patch.jar", "-m", "app/main"},
		{"java", "--patch-module=java.base=lib:/workspace/inputs/patch.jar", "-m", "app/main"},
	}
	for _, argv := range unsafe {
		decision := policy.Review(InputExecutionRequest{Command: argv, WorkingDir: "/workspace"})
		if decision.Allowed || decision.Reason != "interpreter_input" {
			t.Errorf("Java patch-module input entry must be denied: %v -> %+v", argv, decision)
		}
	}

	generated := [][]string{
		{"java", "--patch-module", "java.base=lib:/workspace/generated", "-m", "app/main"},
		{"java", "--patch-module=java.base=lib:/workspace/generated", "-m", "app/main"},
	}
	for _, argv := range generated {
		decision := policy.Review(InputExecutionRequest{Command: argv, WorkingDir: "/workspace"})
		if !decision.Allowed {
			t.Errorf("generated Java patch-module path must stay allowed: %v -> %+v", argv, decision)
		}
	}
}

func TestInputCodePolicyScreensWrappedJavaModulePaths(t *testing.T) {
	policy := newT03Policy(t)
	for _, argv := range [][]string{
		{"timeout", "10", "java", "--module-path", "/workspace/inputs/mod.jar", "-m", "app/main"},
		{"env", "MODE=test", "java", "--patch-module=java.base=/workspace/inputs/patch.jar", "-m", "app/main"},
	} {
		decision := policy.Review(InputExecutionRequest{Command: argv, WorkingDir: "/workspace"})
		if decision.Allowed || decision.Reason != "interpreter_input" {
			t.Errorf("wrapped Java module input must be denied: %v -> %+v", argv, decision)
		}
	}
}

func TestInputCodePolicyDeniesJavaArgFilesAndJDKOptions(t *testing.T) {
	policy := newT03Policy(t)
	unsafe := []struct {
		name string
		req  InputExecutionRequest
	}{
		{"absolute argv", InputExecutionRequest{Command: []string{"java", "@/workspace/inputs/launch.args"}, WorkingDir: "/workspace"}},
		{"relative argv", InputExecutionRequest{Command: []string{"java", "@inputs/launch.args"}, WorkingDir: "/workspace"}},
		{"timeout wrapper", InputExecutionRequest{Command: []string{"timeout", "10", "java", "@/workspace/inputs/launch.args"}, WorkingDir: "/workspace"}},
		{"env wrapper assignment", InputExecutionRequest{Command: []string{"env", "JDK_JAVA_OPTIONS=@/workspace/inputs/launch.args", "java", "-jar", "app.jar"}, WorkingDir: "/workspace"}},
		{"java environment", InputExecutionRequest{Command: []string{"java", "-jar", "app.jar"}, WorkingDir: "/workspace", Environment: map[string]string{"JDK_JAVA_OPTIONS": "@/workspace/inputs/launch.args"}}},
		{"quoted shell token", InputExecutionRequest{Shell: true, CommandText: `env MODE=test java "@/workspace/inputs/launch.args"`, WorkingDir: "/workspace", ResolvedTargetPath: "/workspace/generated/main.class", TargetSHA256: "generated-digest"}},
		{"shell JDK option assignment", InputExecutionRequest{Shell: true, CommandText: `env JDK_JAVA_OPTIONS='-Xmx1g' java -jar app.jar`, WorkingDir: "/workspace", ResolvedTargetPath: "/workspace/generated/main.class", TargetSHA256: "generated-digest"}},
		{"shell generated JDK argfile environment", InputExecutionRequest{Shell: true, CommandText: `java -jar app.jar`, WorkingDir: "/workspace", Environment: map[string]string{"JDK_JAVA_OPTIONS": "@/workspace/generated/launch.args"}, ResolvedTargetPath: "/workspace/generated/main.class", TargetSHA256: "generated-digest"}},
	}
	for _, tc := range unsafe {
		t.Run(tc.name, func(t *testing.T) {
			decision := policy.Review(tc.req)
			if decision.Allowed || (decision.Reason != "interpreter_input" && decision.Reason != "input_target" && decision.Reason != "shell_input") {
				t.Fatalf("Java argfile in inputs must be denied: %+v -> %+v", tc.req, decision)
			}
		})
	}
}

func TestInputCodePolicyAllowsBenignJavaArgFilesAndLaunchers(t *testing.T) {
	policy := newT03Policy(t)
	forbiddenArgFile := policy.Review(InputExecutionRequest{Command: []string{"java", "@/workspace/generated/launch.args"}, WorkingDir: "/workspace"})
	if forbiddenArgFile.Allowed {
		t.Fatalf("uninspectable generated Java argfile must fail closed: %+v", forbiddenArgFile)
	}
	allowed := []InputExecutionRequest{
		{Command: []string{"java", "-jar", "app.jar"}, WorkingDir: "/workspace"},
		{Command: []string{"timeout", "10", "java", "--module-path", "/workspace/generated", "-m", "app/main"}, WorkingDir: "/workspace"},
		{Shell: true, CommandText: `java -jar app.jar`, WorkingDir: "/workspace", ResolvedTargetPath: "/workspace/generated/main.class", TargetSHA256: "generated-digest"},
	}
	for _, req := range allowed {
		if decision := policy.Review(req); !decision.Allowed {
			t.Errorf("benign Java launcher input must remain allowed: %+v -> %+v", req, decision)
		}
	}
}
