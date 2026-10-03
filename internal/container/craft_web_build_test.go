package container

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/stretchr/testify/require"
)

// T04 (#123): the fixed offline web toolchain. These tests drive the REAL
// shipped build program (docker/craft/web/build.py) through the pinned
// toolchain directory, then judge the produced build log, entry and assets
// at the W01 BuildChecks evidence seam — the same seam the collector uses.
// No test here fabricates a passing fact: a build that did not run stays
// not_run, a non-zero exit is a failed build check, and a log that does not
// match the pinned toolchain is ignored entirely.

// craftWebToolchainDir is the shipped pinned toolchain (template, provisioned
// dependencies, build program and its lock), resolved to an absolute path
// like the build program's directory checks expect.
func craftWebToolchainAbsDir(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("..", "..", "docker", "craft", "web"))
	require.NoError(t, err)
	return abs
}

// runCraftWebBuild executes the REAL offline build program with a scrubbed
// environment: an empty HOME and TMPDIR (no host caches or home directories)
// and no proxy variables, proving the build needs nothing but the toolchain
// and the staged content.
func runCraftWebBuild(t *testing.T, toolchainDir, inputDir, outputDir, runtimeDigest string, extraEnv ...string) (int, string) {
	t.Helper()
	absToolchain, err := filepath.Abs(toolchainDir)
	require.NoError(t, err)
	absInput, err := filepath.Abs(inputDir)
	require.NoError(t, err)
	absOutput, err := filepath.Abs(outputDir)
	require.NoError(t, err)
	cmd := exec.Command("python3", filepath.Join(absToolchain, "build.py"),
		"--toolchain", absToolchain, "--input", absInput, "--output", absOutput,
		"--runtime-digest", runtimeDigest)
	home := filepath.Join(t.TempDir(), "empty-home")
	tmp := filepath.Join(t.TempDir(), "empty-tmp")
	require.NoError(t, os.MkdirAll(home, 0o755))
	require.NoError(t, os.MkdirAll(tmp, 0o755))
	cmd.Env = append([]string{
		"HOME=" + home,
		"TMPDIR=" + tmp,
		"PATH=/usr/bin:/bin:/usr/sbin:/sbin:/usr/local/bin",
		"PYTHONDONTWRITEBYTECODE=1",
		"no_proxy=*", "NO_PROXY=*",
	}, extraEnv...)
	out, err := cmd.CombinedOutput()
	code := 0
	if exit, ok := err.(*exec.ExitError); ok {
		code = exit.ExitCode()
	} else if err != nil {
		t.Fatalf("run real build program: %v\n%s", err, out)
	}
	return code, string(out)
}

// craftWebTestContent stages one content bundle: a title, one table section
// and one html section — the shape the craft-web-build skill authors.
func craftWebTestContent(t *testing.T, dir string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0o755))
	content := map[string]any{
		"title":    "区域销售网页",
		"subtitle": "华东区 2026 Q3",
		"lang":     "zh-CN",
		"sections": []map[string]any{
			{"heading": "按省销售额", "table": map[string]any{"columns": []string{"省份", "销售额（万元）"}, "rows": [][]string{{"浙江", "96"}, {"江苏", "12"}}}},
			{"heading": "要点", "html": "<ul><li>96% 增长来自华东</li></ul>"},
		},
	}
	raw, err := json.Marshal(content)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "content.json"), raw, 0o644))
}

// TestCraftT04Journey walks the T04 member journey at the highest available
// seam: the pinned offline build runs against staged material, its log feeds
// the collector's evidence, and the resulting version checks tell the truth
// on success, missing dependencies and absent entries.
func TestCraftT04Journey(t *testing.T) {
	pin, err := LoadCraftWebToolchainPin(craftWebToolchainAbsDir(t))
	require.NoError(t, err, "shipped toolchain must load with verified pins")
	require.NotEmpty(t, pin.ToolchainDigest)
	require.NotEmpty(t, pin.TemplateVersion)

	runtimeDigest := "sha256:" + strings.Repeat("ab", 32)

	// 1. Success path: a clean offline build (empty HOME/TMPDIR, no proxy)
	//    produces the entry, local-only assets and an honest log.
	workspace := t.TempDir()
	inputDir := filepath.Join(workspace, "input")
	outputDir := filepath.Join(workspace, "output")
	craftWebTestContent(t, inputDir)
	code, log := runCraftWebBuild(t, craftWebToolchainAbsDir(t), inputDir, outputDir, runtimeDigest)
	require.Zero(t, code, "offline build must succeed without network or host caches:\n%s", log)

	entry, err := os.ReadFile(filepath.Join(outputDir, "index.html"))
	require.NoError(t, err, "successful build must produce the web entry")
	require.Contains(t, string(entry), pin.TemplateVersion, "entry carries the fixed template identity")
	for _, asset := range []string{"assets/craft-web.css", "assets/craft-web.js"} {
		raw, err := os.ReadFile(filepath.Join(outputDir, filepath.FromSlash(asset)))
		require.NoError(t, err, "provisioned dependency must ship as a local asset: %s", asset)
		require.NotEmpty(t, raw)
	}

	rawLog, err := os.ReadFile(filepath.Join(outputDir, "build-log.json"))
	require.NoError(t, err, "successful build must leave a build log")
	buildLog, err := ParseCraftWebBuildLog(rawLog)
	require.NoError(t, err, "real build log must decode under the strict contract")
	require.Equal(t, runtimeDigest, buildLog.RuntimeDigest, "log identifies the runtime digest")
	require.Equal(t, pin.ToolchainDigest, buildLog.ToolchainDigest, "log identifies the pinned toolchain digest")
	require.Equal(t, pin.TemplateVersion, buildLog.TemplateVersion)
	require.Equal(t, pin.TemplateSHA256, buildLog.TemplateSHA256)
	require.NotNil(t, buildLog.ExitCode)
	require.Zero(t, *buildLog.ExitCode)
	require.Equal(t, "index.html", buildLog.Entry)

	// The log feeds the W01 evidence seam: build ran and exited 0, so the
	// collected version's build check passes and the entry check passes on
	// the really produced files.
	exitCode := 0
	evidence, err := CraftWebBuildEvidence(buildLog, pin, &exitCode)
	require.NoError(t, err)
	require.True(t, evidence.BuildRan)
	require.Zero(t, evidence.BuildExitCode)
	files := []craft.File{
		{Path: "index.html", Ref: "resource://index.html", SHA256: sha256Sum(string(entry))},
		{Path: "assets/craft-web.css", Ref: "resource://css", SHA256: strings.Repeat("c", 64)},
		{Path: "assets/craft-web.js", Ref: "resource://js", SHA256: strings.Repeat("j", 64)},
	}
	checks := craft.BuildChecks(craft.KindWeb, files, evidence)
	require.Equal(t, craft.CheckPassed, checks[0].Status, "build check: %s", checks[0].Detail)
	require.Equal(t, craft.CheckPassed, checks[1].Status, "entry check: %s", checks[1].Detail)

	// The evidence source composition keeps the preview verdicts and adds
	// the real build facts from the delegated run's output.
	innerRan := false
	inner := func(context.Context, craft.Task) craft.ArtifactEvidence {
		innerRan = true
		return craft.ArtifactEvidence{PreviewRan: true, PreviewPassed: true}
	}
	task := craft.Task{WorkspaceID: "ws-1"}
	task.Fence.RunID = "run-1"
	task.Scope = craft.Scope{TenantID: 1, UserID: "u1", SessionID: "t04-run"}
	merged := craftWebBuildEvidenceSource(inner, func(context.Context, craft.Task) ([]byte, error) {
		return os.ReadFile(filepath.Join(outputDir, "build-log.json"))
	}, pin, nil)(context.Background(), task)
	require.True(t, innerRan, "preview evidence source is still consulted")
	require.False(t, merged.BuildRan, "sandbox-writable logs cannot establish execution evidence without a server receipt")
	require.True(t, merged.PreviewRan && merged.PreviewPassed, "preview verdicts survive the merge")

	// 2. Missing provisioned dependency: the build fails loudly, the log
	//    carries the REAL non-zero exit status and the collector's build
	//    check is failed — never fabricated as passed.
	broken := t.TempDir()
	require.NoError(t, exec.Command("cp", "-R", craftWebToolchainAbsDir(t)+"/.", broken).Run())
	require.NoError(t, os.Remove(filepath.Join(broken, "deps", "craft-web.css")))
	brokenOut := filepath.Join(t.TempDir(), "output")
	code, failureLog := runCraftWebBuild(t, broken, inputDir, brokenOut, runtimeDigest)
	require.NotZero(t, code, "missing provisioned dependency must fail the build")
	rawFailLog, err := os.ReadFile(filepath.Join(brokenOut, "build-log.json"))
	require.NoError(t, err, "failed build still leaves its log")
	failLog, err := ParseCraftWebBuildLog(rawFailLog)
	require.NoError(t, err)
	require.NotNil(t, failLog.ExitCode)
	require.Equal(t, code, *failLog.ExitCode, "log records the real exit status")
	require.NotEmpty(t, failLog.Error, "failure names its cause")
	require.Contains(t, failureLog, "craft-web.css", "stderr names the missing dependency")
	failEvidence, err := CraftWebBuildEvidence(failLog, pin, &code)
	require.NoError(t, err)
	require.True(t, failEvidence.BuildRan)
	require.Equal(t, code, failEvidence.BuildExitCode)
	_, _ = os.Stat(filepath.Join(brokenOut, "index.html"))
	failedChecks := craft.BuildChecks(craft.KindWeb, nil, failEvidence)
	require.Equal(t, craft.CheckFailed, failedChecks[0].Status, "failed build check must stay failed")

	// 3. Absent entry fails honestly: a log that claims exit 0 without the
	//    entry file cannot turn the entry check into a pass.
	entryChecks := craft.BuildChecks(craft.KindWeb, nil, evidence)
	require.Equal(t, craft.CheckFailed, entryChecks[1].Status, "entry check without index.html must fail")

	// 4. A log from a DIFFERENT toolchain is ignored whole — the build fact
	//    stays not_run instead of being imported on trust.
	foreignLog := buildLog
	foreignLog.ToolchainDigest = strings.Repeat("0f", 32)
	_, err = CraftWebBuildEvidence(foreignLog, pin, &exitCode)
	require.Error(t, err, "log pinned to a foreign toolchain must be refused")
	ignored := craftWebBuildEvidenceSource(inner, func(context.Context, craft.Task) ([]byte, error) {
		raw, _ := json.Marshal(foreignLog)
		return raw, nil
	}, pin, nil)(context.Background(), task)
	require.False(t, ignored.BuildRan, "foreign-toolchain log must not fabricate a build fact")

	// 5. Malformed logs are rejected by the strict contract: smuggled
	//    fields, remote assets, impossible exit codes, missing digests.
	var tampered map[string]any
	require.NoError(t, json.Unmarshal(rawLog, &tampered))
	tampered["network"] = "allowed"
	rawBad, _ := json.Marshal(tampered)
	_, err = ParseCraftWebBuildLog(rawBad)
	require.Error(t, err, "unknown field must reject")

	external := buildLog
	external.Assets = []string{"https://cdn.example.com/app.js"}
	_, err = ParseCraftWebBuildLog(mustJSON(t, external))
	require.Error(t, err, "remote asset must reject")

	escape := buildLog
	escape.Assets = []string{"../outside.txt"}
	_, err = ParseCraftWebBuildLog(mustJSON(t, escape))
	require.Error(t, err, "escaping asset path must reject")

	bigExit := buildLog
	tooBig := 256
	bigExit.ExitCode = &tooBig
	_, err = ParseCraftWebBuildLog(mustJSON(t, bigExit))
	require.Error(t, err, "impossible exit code must reject")

	noDigest := buildLog
	noDigest.RuntimeDigest = ""
	_, err = ParseCraftWebBuildLog(mustJSON(t, noDigest))
	require.Error(t, err, "log without runtime digest must reject")

	// A missing log is not an error: the build fact simply stays unobserved.
	missing := craftWebBuildEvidenceSource(inner, func(context.Context, craft.Task) ([]byte, error) {
		return nil, os.ErrNotExist
	}, pin, nil)(context.Background(), task)
	require.False(t, missing.BuildRan)
	require.True(t, missing.PreviewRan)

	// 6. Deferred item (ledger :533): the offline build COMMAND entry consumes
	//    the T03 uploaded-material policy. The pinned build command passes the
	//    same gate the exec services enforce; a command that smuggles admitted
	//    material is refused member-visibly; a foreign command shape never
	//    reaches the gate at all.
	{
		fixture := newCraftWebBuildReviewFixture(t)
		reviewer, rerr := NewCraftWebBuildCommandGate(fixture.gate, pin, craftWebToolchainAbsDir(t))
		require.NoError(t, rerr, "the command gate assembles over the shipped toolchain")

		require.NoError(t, reviewer.Review(context.Background(), craftWebBuildRequest(craftWebPinnedBuildCommand(), nil)),
			"the pinned offline build command passes the T03 gate")

		derr := reviewer.Review(context.Background(), craftWebBuildRequest(craftWebPinnedBuildCommand(), map[string]string{
			"PYTHONSTARTUP": "/workspace/inputs/" + fixture.uploaded.SHA256 + "/analyze.py",
		}))
		require.ErrorIs(t, derr, craft.ErrForbidden)
		require.Contains(t, derr.Error(), "Allowed alternative", "the refusal must be member-visible")

		serr := reviewer.Review(context.Background(), craftWebBuildRequest(
			[]string{"python3", "/tmp/evil.py"}, nil))
		require.ErrorIs(t, serr, craft.ErrForbidden)
		require.Contains(t, serr.Error(), "craft web build command", "the fixed shape is named for the member")
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := json.Marshal(v)
	require.NoError(t, err)
	return raw
}

// TestCraftWebToolchainPinsMatchShippedFiles machine-checks acceptance #1:
// the fixed template and dependency versions are pinned by digest in the
// toolchain lock AND represented in the runtime config the sandbox probe
// hashes (docker/craft/runtime-config.json), and the derived toolchain
// digest equals both records.
func TestCraftWebToolchainPinsMatchShippedFiles(t *testing.T) {
	dir := craftWebToolchainAbsDir(t)
	template := filepath.Join(dir, "template.html")
	css := filepath.Join(dir, "deps", "craft-web.css")
	js := filepath.Join(dir, "deps", "craft-web.js")
	buildProgram := filepath.Join(dir, "build.py")

	templateSum := fileSHA256(t, template)
	cssSum := fileSHA256(t, css)
	jsSum := fileSHA256(t, js)
	buildSum := fileSHA256(t, buildProgram)

	derived := CraftWebToolchainDigest(templateSum, map[string]string{
		"craft-web.css": cssSum,
		"craft-web.js":  jsSum,
	}, buildSum)

	// The shipped lock pins every file digest and the derived digest.
	lockBytes := readFile(t, filepath.Join(dir, "toolchain.lock.json"))
	var lock struct {
		Schema          int    `json:"schema"`
		Name            string `json:"name"`
		ToolchainDigest string `json:"toolchain_digest"`
		Template        struct {
			Name    string `json:"name"`
			Version string `json:"version"`
			SHA256  string `json:"sha256"`
		} `json:"template"`
		Dependencies []struct {
			Name    string `json:"name"`
			Version string `json:"version"`
			SHA256  string `json:"sha256"`
		} `json:"dependencies"`
		BuildProgram struct {
			Name   string `json:"name"`
			SHA256 string `json:"sha256"`
		} `json:"build_program"`
	}
	require.NoError(t, json.Unmarshal(lockBytes, &lock))
	require.Equal(t, "craft-web-toolchain", lock.Name)
	require.Equal(t, templateSum, lock.Template.SHA256, "lock pins the shipped template bytes")
	require.NotEmpty(t, lock.Template.Version)
	deps := map[string]string{}
	for _, d := range lock.Dependencies {
		deps[d.Name] = d.SHA256
	}
	require.Equal(t, cssSum, deps["craft-web.css"], "lock pins the shipped css bytes")
	require.Equal(t, jsSum, deps["craft-web.js"], "lock pins the shipped js bytes")
	require.Equal(t, buildSum, lock.BuildProgram.SHA256, "lock pins the build program bytes")
	require.Equal(t, derived, lock.ToolchainDigest, "lock carries the derived toolchain digest")

	// The runtime config — the file the sandbox probe hashes as the runtime
	// identity — carries the same pins, so template and dependency versions
	// are represented in the runtime digest.
	configBytes := readFile(t, filepath.Join("..", "..", "docker", "craft", "runtime-config.json"))
	var config struct {
		WebToolchain *struct {
			ToolchainDigest string `json:"toolchain_digest"`
			TemplateVersion string `json:"template_version"`
			TemplateSHA256  string `json:"template_sha256"`
			Dependencies    []struct {
				Name    string `json:"name"`
				Version string `json:"version"`
				SHA256  string `json:"sha256"`
			} `json:"dependencies"`
			Root string `json:"root"`
		} `json:"web_toolchain"`
	}
	require.NoError(t, json.Unmarshal(configBytes, &config))
	require.NotNil(t, config.WebToolchain, "runtime config must represent the web toolchain")
	require.Equal(t, derived, config.WebToolchain.ToolchainDigest)
	require.Equal(t, lock.Template.Version, config.WebToolchain.TemplateVersion)
	require.Equal(t, templateSum, config.WebToolchain.TemplateSHA256)
	require.Len(t, config.WebToolchain.Dependencies, len(lock.Dependencies))
	for i, d := range config.WebToolchain.Dependencies {
		require.Equal(t, lock.Dependencies[i].Name, d.Name)
		require.Equal(t, lock.Dependencies[i].Version, d.Version)
		require.Equal(t, lock.Dependencies[i].SHA256, d.SHA256)
	}

	// LoadCraftWebToolchainPin re-verifies the bytes on load: a tampered
	// template must fail the pin, not load silently.
	tampered := t.TempDir()
	require.NoError(t, exec.Command("cp", "-R", craftWebToolchainAbsDir(t)+"/.", tampered).Run())
	require.NoError(t, os.WriteFile(filepath.Join(tampered, "template.html"), []byte("<html>tampered</html>"), 0o644))
	_, err := LoadCraftWebToolchainPin(tampered)
	require.Error(t, err, "tampered toolchain must fail pin verification")
}

// TestCraftWebBuildSkillManifestSelfConsistent pins the shipped OpenCode
// craft-web-build skill: the manifest decodes under the strict authority-free
// contract and its digest pins the actual SKILL.md bytes shipped beside it.
func TestCraftWebBuildSkillManifestSelfConsistent(t *testing.T) {
	rawManifest := readFile(t, filepath.Join("..", "..", "skills", "craft-web-build", "manifest.json"))
	manifest, err := craft.DecodeSkillManifest(rawManifest)
	require.NoError(t, err, "shipped skill manifest must satisfy its own contract")
	require.Equal(t, "craft-web-build", manifest.Name)
	require.Equal(t, craft.KindWeb, manifest.ArtifactKind)

	skillMD := readFile(t, filepath.Join("..", "..", "skills", "craft-web-build", "SKILL.md"))
	require.Equal(t, sha256Sum(string(skillMD)), manifest.Digest, "manifest digest pins SKILL.md")
	require.Equal(t, "SKILL.md", manifest.DigestOf)
	require.NotEmpty(t, manifest.ToolRequirements)

	// The skill directs the model at the pinned offline build program, never
	// at a package manager or a CDN.
	require.Contains(t, string(skillMD), "/opt/craft/web/build.py")
	require.NotContains(t, string(skillMD), "npm install")
	require.NotContains(t, string(skillMD), "cdn")
}

// fileSHA256 hashes one file's bytes.
func fileSHA256(t *testing.T, path string) string {
	t.Helper()
	raw := readFile(t, path)
	return sha256Sum(string(raw))
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	return raw
}
