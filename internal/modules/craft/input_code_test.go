package craft

import (
	"strings"
	"testing"
)

func t03Input(digest, name string) Input {
	return Input{Ref: "ref-" + name, Name: name, SHA256: digest, Bytes: 42,
		Recognition: &InputRecognition{Accepted: true, Understood: false, Reason: "no_supported_parser"}}
}

const t03PythonDigest = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func t03ScriptPath() string {
	path, err := InputPath(t03PythonDigest, "analyze.py")
	if err != nil {
		panic(err)
	}
	return "/workspace/" + path
}

func newT03Policy(t *testing.T) *InputExecutionPolicy {
	t.Helper()
	policy, err := NewInputExecutionPolicy("/workspace", []Input{t03Input(t03PythonDigest, "analyze.py")})
	if err != nil {
		t.Fatalf("NewInputExecutionPolicy: %v", err)
	}
	return policy
}

func TestInputCodeClassificationLabelsCodeProvenance(t *testing.T) {
	cases := []struct {
		name    string
		content string
		isCode  bool
		reason  string
	}{
		{"analyze.py", "print('hi')\n", true, "script_extension"},
		{"run.sh", "echo hi\n", true, "script_extension"},
		{"ledger.csv", "a,b\n1,2\n", false, "data"},
		{"notes.txt", "plain text without any shebang", false, "data"},
		{"noext", "#!/usr/bin/env python3\nprint('hi')\n", true, "shebang"},
	}
	for _, tc := range cases {
		label := ClassifyInputCode(tc.name, []byte(tc.content))
		if label.IsCode != tc.isCode || label.Reason != tc.reason {
			t.Fatalf("ClassifyInputCode(%q) = %+v, want isCode=%v reason=%q", tc.name, label, tc.isCode, tc.reason)
		}
	}
}

func TestInputCodePolicyDeniesDirectInterpreterAndShellExecution(t *testing.T) {
	policy := newT03Policy(t)
	script := t03ScriptPath()

	interpreter := policy.Review(InputExecutionRequest{
		Command:    []string{"python3", script},
		WorkingDir: "/workspace",
	})
	if interpreter.Allowed {
		t.Fatalf("interpreter execution of uploaded input must be denied")
	}
	if interpreter.Reason != "interpreter_input" {
		t.Fatalf("interpreter denial reason = %q, want interpreter_input", interpreter.Reason)
	}

	direct := policy.Review(InputExecutionRequest{
		Command:    []string{script},
		WorkingDir: "/workspace",
	})
	if direct.Allowed || direct.Reason != "input_target" {
		t.Fatalf("direct execution decision = %+v, want denied input_target", direct)
	}

	sh := policy.Review(InputExecutionRequest{
		Shell:       true,
		CommandText: "bash -c 'python3 " + script + "'",
		WorkingDir:  "/workspace",
	})
	if sh.Allowed || sh.Reason != "shell_input" {
		t.Fatalf("shell execution decision = %+v, want denied shell_input", sh)
	}

	relative := policy.Review(InputExecutionRequest{
		Command:    []string{"python3", "inputs/" + t03PythonDigest + "/analyze.py"},
		WorkingDir: "/workspace",
	})
	if relative.Allowed {
		t.Fatalf("relative interpreter path must still be denied: %+v", relative)
	}
}

func TestInputCodePolicyDeniesCopySymlinkAliasAndTempBypasses(t *testing.T) {
	policy := newT03Policy(t)
	script := t03ScriptPath()

	copied := policy.Review(InputExecutionRequest{
		Command:      []string{"python3", "/tmp/run.py"},
		WorkingDir:   "/workspace",
		TargetPath:   "/tmp/run.py",
		TargetSHA256: t03PythonDigest,
	})
	if copied.Allowed || copied.Reason != "input_identity" {
		t.Fatalf("copy-then-execute decision = %+v, want denied input_identity", copied)
	}

	linked := policy.Review(InputExecutionRequest{
		Command:            []string{"python3", "/tmp/link.py"},
		WorkingDir:         "/workspace",
		TargetPath:         "/tmp/link.py",
		ResolvedTargetPath: script,
	})
	if linked.Allowed || linked.Reason != "input_symlink" {
		t.Fatalf("symlink bypass decision = %+v, want denied input_symlink", linked)
	}

	alias := policy.Review(InputExecutionRequest{
		Command:    []string{"python3", "//workspace/./inputs/../inputs/" + t03PythonDigest + "/analyze.py"},
		WorkingDir: "/workspace",
	})
	if alias.Allowed {
		t.Fatalf("aliased path must canonicalize into the inputs tree and be denied: %+v", alias)
	}

	temp := policy.Review(InputExecutionRequest{
		Command:      []string{"/tmp/.tmpXYZ/copied.sh"},
		WorkingDir:   "/workspace",
		TargetPath:   "/tmp/.tmpXYZ/copied.sh",
		TargetSHA256: t03PythonDigest,
	})
	if temp.Allowed || temp.Reason != "input_identity" {
		t.Fatalf("temporary-directory bypass decision = %+v, want denied input_identity", temp)
	}
}

func TestInputCodePolicyAllowsGeneratedWorkspaceCode(t *testing.T) {
	policy := newT03Policy(t)
	generatedDigest := strings.Repeat("f", 64)

	generated := policy.Review(InputExecutionRequest{
		Command:      []string{"python3", "/workspace/app/main.py"},
		WorkingDir:   "/workspace",
		TargetPath:   "/workspace/app/main.py",
		TargetSHA256: generatedDigest,
	})
	if !generated.Allowed {
		t.Fatalf("generated workspace code must execute: %+v", generated)
	}
	if generated.AuditKind != AuditKindGeneratedExecute {
		t.Fatalf("allowed execution audit kind = %q, want %q", generated.AuditKind, AuditKindGeneratedExecute)
	}
	if generated.Refusal() != "" {
		t.Fatalf("allowed decision must carry no refusal, got %q", generated.Refusal())
	}

	output := policy.Review(InputExecutionRequest{
		Command:    []string{"/workspace/rv-abc/output/build.sh"},
		WorkingDir: "/workspace/rv-abc",
	})
	if !output.Allowed {
		t.Fatalf("code generated into the writable output boundary must execute: %+v", output)
	}
}

func TestInputCodePolicyRefusalNamesAllowedAlternative(t *testing.T) {
	policy := newT03Policy(t)
	denied := policy.Review(InputExecutionRequest{
		Command:    []string{"python3", t03ScriptPath()},
		WorkingDir: "/workspace",
	})
	refusal := denied.Refusal()
	if refusal == "" {
		t.Fatalf("denied execution must render a member-visible refusal")
	}
	if !strings.Contains(refusal, "Allowed alternative") {
		t.Fatalf("refusal %q must name the allowed alternative", refusal)
	}
	if !strings.Contains(refusal, t03PythonDigest) && !strings.Contains(refusal, t03ScriptPath()) {
		t.Fatalf("refusal %q must identify the denied uploaded material", refusal)
	}
}

func TestInputCodeAuditKindsStayDistinct(t *testing.T) {
	if AuditKindInputRead == AuditKindGeneratedExecute ||
		AuditKindInputRead == AuditKindInputExecuteDenied ||
		AuditKindGeneratedExecute == AuditKindInputExecuteDenied {
		t.Fatalf("audit kinds must distinguish reading uploaded code from executing generated code")
	}
	if AuditKindInputRead != "craft.input.read" || AuditKindGeneratedExecute != "craft.generated.execute" ||
		AuditKindInputExecuteDenied != "craft.input.execute_denied" {
		t.Fatalf("audit kind constants changed: %q %q %q", AuditKindInputRead, AuditKindGeneratedExecute, AuditKindInputExecuteDenied)
	}
	read := InputReadAuditEvent(t03Input(t03PythonDigest, "analyze.py"))
	if read.Kind != AuditKindInputRead || read.Digest != t03PythonDigest || read.Target == "" {
		t.Fatalf("input read audit event = %+v", read)
	}
}

func TestInputCodePolicyRejectsMalformedConstruction(t *testing.T) {
	if _, err := NewInputExecutionPolicy("", nil); err == nil {
		t.Fatalf("empty workspace root must be rejected")
	}
	bad := t03Input("not-a-digest", "analyze.py")
	if _, err := NewInputExecutionPolicy("/workspace", []Input{bad}); err == nil {
		t.Fatalf("malformed input digest must be rejected")
	}
}

func TestInputCodePolicyContainsAndInputsServeAdapters(t *testing.T) {
	policy := newT03Policy(t)
	if !policy.Contains("/workspace", "inputs/"+t03PythonDigest+"/analyze.py") {
		t.Fatalf("relative staged input path must resolve inside the tree")
	}
	if !policy.Contains("", "/workspace/inputs/"+t03PythonDigest+"/analyze.py") {
		t.Fatalf("absolute staged input path must resolve inside the tree")
	}
	if policy.Contains("/workspace", "app/main.py") {
		t.Fatalf("writable workspace path must not be classified as input material")
	}
	inputs := policy.Inputs()
	if len(inputs) != 1 || inputs[0].SHA256 != t03PythonDigest {
		t.Fatalf("Inputs() = %+v, want the admitted manifest", inputs)
	}
	inputs[0].SHA256 = "mutated"
	if policy.Inputs()[0].SHA256 != t03PythonDigest {
		t.Fatalf("Inputs() must return a defensive copy")
	}
}
