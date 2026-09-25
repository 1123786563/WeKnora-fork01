package container

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/modules/craft"
)

// T04 (#123): the fixed offline web toolchain. The template, the provisioned
// dependency set and the build program are pinned by digest in
// docker/craft/web/toolchain.lock.json and represented in the runtime
// identity (docker/craft/runtime-config.json's web_toolchain section — the
// same file the sandbox probe hashes). The build program itself writes
// output/build-log.json naming the runtime digest, the toolchain digest and
// the REAL exit status; this file turns that log into the collector's W01
// build evidence — and refuses to fabricate anything: a missing, malformed
// or foreign-toolchain log leaves the build fact unobserved (not_run).

// craftWebToolchainName mirrors build.py's TOOLCHAIN_NAME.
const craftWebToolchainName = "craft-web-toolchain"

// craftWebBuildLogName is the workspace-output-relative build log path.
const craftWebBuildLogName = "build-log.json"

// craftWebFixedDependencies mirrors build.py's FIXED_DEPENDENCIES: the lock
// must list exactly this set — a lock naming anything else is refused before
// any of its strings could become a path.
var craftWebFixedDependencies = []string{"craft-web.css", "craft-web.js"}

// CraftWebToolchainDigest derives the deterministic identity of the pinned
// toolchain: sha256("craft-web-toolchain\x00" + templateSHA + "\x00" +
// "\x00".join(sorted("name:sha") of deps) + "\x00" + buildSHA). The Python
// build program derives the identical value byte-for-byte, and
// toolchain.lock.json + runtime-config.json both carry it, so three
// independent derivations must agree before a build counts.
func CraftWebToolchainDigest(templateSHA string, dependencies map[string]string, buildSHA string) string {
	names := make([]string, 0, len(dependencies))
	for name := range dependencies {
		names = append(names, name)
	}
	sort.Strings(names)
	payload := &bytes.Buffer{}
	payload.WriteString(craftWebToolchainName)
	payload.WriteByte(0)
	payload.WriteString(templateSHA)
	payload.WriteByte(0)
	for i, name := range names {
		if i > 0 {
			payload.WriteByte(0)
		}
		payload.WriteString(name)
		payload.WriteString(":")
		payload.WriteString(dependencies[name])
	}
	payload.WriteByte(0)
	payload.WriteString(buildSHA)
	sum := sha256.Sum256(payload.Bytes())
	return hex.EncodeToString(sum[:])
}

// CraftWebToolchainPin is the pinned identity a build log must match before
// its exit status may become the collector's build evidence.
type CraftWebToolchainPin struct {
	ToolchainDigest string
	TemplateVersion string
	TemplateSHA256  string
}

// craftWebToolchainLock is the shipped toolchain.lock.json shape.
type craftWebToolchainLock struct {
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

// LoadCraftWebToolchainPin loads one toolchain directory's lock and
// re-verifies every pinned file's bytes on disk before trusting the pin: a
// tampered template, dependency or build program fails instead of loading.
func LoadCraftWebToolchainPin(dir string) (CraftWebToolchainPin, error) {
	if strings.TrimSpace(dir) == "" {
		return CraftWebToolchainPin{}, fmt.Errorf("%w: craft web toolchain directory is required", craft.ErrInvalidInput)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "toolchain.lock.json"))
	if err != nil {
		return CraftWebToolchainPin{}, fmt.Errorf("craft web toolchain lock: %w", err)
	}
	var lock craftWebToolchainLock
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&lock); err != nil {
		return CraftWebToolchainPin{}, fmt.Errorf("%w: craft web toolchain lock decode: %v", craft.ErrInvalidInput, err)
	}
	var trailing any
	if err := dec.Decode(&trailing); err == nil {
		return CraftWebToolchainPin{}, fmt.Errorf("%w: craft web toolchain lock carries trailing content", craft.ErrInvalidInput)
	}
	if lock.Name != craftWebToolchainName {
		return CraftWebToolchainPin{}, fmt.Errorf("%w: craft web toolchain lock names %q", craft.ErrInvalidInput, lock.Name)
	}
	if lock.Template.Name != "template.html" || lock.BuildProgram.Name != "build.py" {
		return CraftWebToolchainPin{}, fmt.Errorf("%w: craft web toolchain layout must stay template.html + build.py", craft.ErrInvalidInput)
	}
	listed := make([]string, 0, len(lock.Dependencies))
	byName := map[string]string{}
	for _, dep := range lock.Dependencies {
		listed = append(listed, dep.Name)
		byName[dep.Name] = dep.SHA256
	}
	sort.Strings(listed)
	fixed := append([]string(nil), craftWebFixedDependencies...)
	sort.Strings(fixed)
	if !equalStrings(listed, fixed) {
		return CraftWebToolchainPin{}, fmt.Errorf("%w: craft web toolchain dependencies must be exactly %v, got %v", craft.ErrInvalidInput, fixed, listed)
	}
	if !validSHA256Hex(lock.Template.SHA256) || !validSHA256Hex(lock.BuildProgram.SHA256) || !validSHA256Hex(lock.ToolchainDigest) {
		return CraftWebToolchainPin{}, fmt.Errorf("%w: craft web toolchain lock carries a malformed digest", craft.ErrInvalidInput)
	}
	for _, sha := range byName {
		if !validSHA256Hex(sha) {
			return CraftWebToolchainPin{}, fmt.Errorf("%w: craft web toolchain lock carries a malformed dependency digest", craft.ErrInvalidInput)
		}
	}

	// Re-verify the actual bytes: template, every dependency, build program.
	if got := fileDigestOrEmpty(filepath.Join(dir, "template.html")); got != lock.Template.SHA256 {
		return CraftWebToolchainPin{}, fmt.Errorf("%w: craft web template bytes differ from the pin", craft.ErrConflict)
	}
	deps := map[string]string{}
	for _, name := range craftWebFixedDependencies {
		got := fileDigestOrEmpty(filepath.Join(dir, "deps", name))
		if got != byName[name] {
			return CraftWebToolchainPin{}, fmt.Errorf("%w: craft web dependency %s bytes differ from the pin", craft.ErrConflict, name)
		}
		deps[name] = got
	}
	if got := fileDigestOrEmpty(filepath.Join(dir, "build.py")); got != lock.BuildProgram.SHA256 {
		return CraftWebToolchainPin{}, fmt.Errorf("%w: craft web build program bytes differ from the pin", craft.ErrConflict)
	}

	derived := CraftWebToolchainDigest(lock.Template.SHA256, deps, lock.BuildProgram.SHA256)
	if derived != lock.ToolchainDigest {
		return CraftWebToolchainPin{}, fmt.Errorf("%w: craft web toolchain digest does not match the pinned files", craft.ErrConflict)
	}
	if strings.TrimSpace(lock.Template.Version) == "" {
		return CraftWebToolchainPin{}, fmt.Errorf("%w: craft web template version is required", craft.ErrInvalidInput)
	}
	return CraftWebToolchainPin{ToolchainDigest: derived, TemplateVersion: lock.Template.Version, TemplateSHA256: lock.Template.SHA256}, nil
}

// CraftWebBuildLog is the strict shape of output/build-log.json written by
// the pinned build program. Unknown fields reject; every identity field must
// be present and well-formed.
type CraftWebBuildLog struct {
	Schema          int      `json:"schema"`
	Kind            string   `json:"kind"`
	RuntimeDigest   string   `json:"runtime_digest"`
	ToolchainDigest string   `json:"toolchain_digest"`
	TemplateVersion string   `json:"template_version"`
	TemplateSHA256  string   `json:"template_sha256"`
	ExitCode        int      `json:"exit_code"`
	Entry           string   `json:"entry"`
	Assets          []string `json:"assets"`
	Egress          string   `json:"egress"`
	Error           string   `json:"error"`
}

// ParseCraftWebBuildLog decodes one build log under the strict contract:
// schema 1, kind web, non-empty runtime digest, well-formed digests, exit
// code 0..255, entry index.html, egress "denied", and assets that are local
// relative paths only (no traversal, no absolute paths, no schemes, no
// host-relative // URLs).
func ParseCraftWebBuildLog(raw []byte) (CraftWebBuildLog, error) {
	var log CraftWebBuildLog
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&log); err != nil {
		return CraftWebBuildLog{}, fmt.Errorf("%w: craft web build log decode: %v", craft.ErrInvalidInput, err)
	}
	var trailing any
	if err := dec.Decode(&trailing); err == nil {
		return CraftWebBuildLog{}, fmt.Errorf("%w: craft web build log carries trailing content", craft.ErrInvalidInput)
	}
	if log.Schema != 1 || log.Kind != craft.KindWeb {
		return CraftWebBuildLog{}, fmt.Errorf("%w: craft web build log must be schema 1 kind web", craft.ErrInvalidInput)
	}
	if strings.TrimSpace(log.RuntimeDigest) == "" {
		return CraftWebBuildLog{}, fmt.Errorf("%w: craft web build log requires the runtime digest", craft.ErrInvalidInput)
	}
	if !validSHA256Hex(log.ToolchainDigest) || !validSHA256Hex(log.TemplateSHA256) {
		return CraftWebBuildLog{}, fmt.Errorf("%w: craft web build log digests are malformed", craft.ErrInvalidInput)
	}
	if strings.TrimSpace(log.TemplateVersion) == "" {
		return CraftWebBuildLog{}, fmt.Errorf("%w: craft web build log requires the template version", craft.ErrInvalidInput)
	}
	if log.ExitCode < 0 || log.ExitCode > 255 {
		return CraftWebBuildLog{}, fmt.Errorf("%w: craft web build log exit code %d is impossible", craft.ErrInvalidInput, log.ExitCode)
	}
	if log.Entry != "index.html" {
		return CraftWebBuildLog{}, fmt.Errorf("%w: craft web entry must be index.html, got %q", craft.ErrInvalidInput, log.Entry)
	}
	if log.Egress != "denied" {
		return CraftWebBuildLog{}, fmt.Errorf("%w: craft web build log must record denied egress", craft.ErrInvalidInput)
	}
	for _, asset := range log.Assets {
		if err := validCraftWebLocalAsset(asset); err != nil {
			return CraftWebBuildLog{}, err
		}
	}
	return log, nil
}

// validCraftWebLocalAsset accepts only version-relative local asset paths:
// relative, no traversal, no drive/scheme, no host-relative // prefix.
func validCraftWebLocalAsset(asset string) error {
	if asset == "" {
		return fmt.Errorf("%w: craft web asset path is empty", craft.ErrInvalidInput)
	}
	if strings.Contains(asset, "\\") || strings.Contains(asset, "\x00") {
		return fmt.Errorf("%w: craft web asset path %q is not a plain relative path", craft.ErrInvalidInput, asset)
	}
	if strings.HasPrefix(asset, "/") || strings.HasPrefix(asset, "//") {
		return fmt.Errorf("%w: craft web asset %q must be relative, not absolute", craft.ErrInvalidInput, asset)
	}
	if strings.Contains(asset, "://") {
		return fmt.Errorf("%w: craft web asset %q carries a scheme", craft.ErrInvalidInput, asset)
	}
	cleaned := path.Clean(asset)
	if cleaned != asset || cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return fmt.Errorf("%w: craft web asset %q escapes the artifact boundary", craft.ErrInvalidInput, asset)
	}
	return nil
}

// CraftWebBuildEvidence folds one verified build log into the collector's
// W01 evidence: the build ran and its REAL exit status is the fact. A log
// pinned to a different toolchain or template than the one this deployment
// loaded is refused whole — its exit status never becomes evidence.
func CraftWebBuildEvidence(log CraftWebBuildLog, pin CraftWebToolchainPin) (craft.ArtifactEvidence, error) {
	if log.ToolchainDigest != pin.ToolchainDigest {
		return craft.ArtifactEvidence{}, fmt.Errorf("%w: build log names toolchain %s, deployment pins %s", craft.ErrConflict, log.ToolchainDigest, pin.ToolchainDigest)
	}
	if log.TemplateSHA256 != pin.TemplateSHA256 {
		return craft.ArtifactEvidence{}, fmt.Errorf("%w: build log names template %s, deployment pins %s", craft.ErrConflict, log.TemplateSHA256, pin.TemplateSHA256)
	}
	return craft.ArtifactEvidence{BuildRan: true, BuildExitCode: log.ExitCode}, nil
}

// craftWebBuildEvidenceSource wraps one preview evidence source with the
// delegated run's build log: preview verdicts keep flowing, and the build
// fact is added only when the log exists, decodes under the strict contract
// and matches the pinned toolchain. Anything else — missing file, malformed
// JSON, foreign toolchain — leaves the build fact exactly as the inner
// source reported it (unobserved), never fabricated.
func craftWebBuildEvidenceSource(
	inner service.ArtifactEvidenceSource,
	readLog func(context.Context, craft.Task) ([]byte, error),
	pin CraftWebToolchainPin,
) service.ArtifactEvidenceSource {
	return func(ctx context.Context, task craft.Task) craft.ArtifactEvidence {
		evidence := craft.ArtifactEvidence{}
		if inner != nil {
			evidence = inner(ctx, task)
		}
		if readLog == nil {
			return evidence
		}
		raw, err := readLog(ctx, task)
		if err != nil || len(raw) == 0 {
			return evidence
		}
		log, err := ParseCraftWebBuildLog(raw)
		if err != nil {
			return evidence
		}
		build, err := CraftWebBuildEvidence(log, pin)
		if err != nil {
			return evidence
		}
		evidence.BuildRan = build.BuildRan
		evidence.BuildExitCode = build.BuildExitCode
		return evidence
	}
}

// craftSessionBuildLogReader reads output/build-log.json through the same
// session artifact source the collector lists: the serve workspace's output
// pointer resolves to the delegation's real directory, so the log read is
// the same bytes a collection would stage.
func craftSessionBuildLogReader(src interface {
	ReadSessionFile(context.Context, string, string) ([]byte, error)
}, outputDir string) func(context.Context, craft.Task) ([]byte, error) {
	logPath := path.Clean(outputDir + "/" + craftWebBuildLogName)
	return func(ctx context.Context, task craft.Task) ([]byte, error) {
		if src == nil || strings.TrimSpace(task.Scope.SessionID) == "" {
			return nil, errors.New("craft web build log reader is not assembled")
		}
		return src.ReadSessionFile(ctx, task.Scope.SessionID, logPath)
	}
}

// equalStrings reports element-wise equality of two slices.
func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// fileDigestOrEmpty hashes one file, returning "" when unreadable so callers
// compare against the pin and fail closed.
func fileDigestOrEmpty(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
