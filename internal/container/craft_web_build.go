package container

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/craft"
)

// T04 (#123): the fixed offline web toolchain. The template, the provisioned
// dependency set and the build program are pinned by digest in
// docker/craft/web/toolchain.lock.json and represented in the runtime
// identity (docker/craft/runtime-config.json's web_toolchain section — the
// same file the sandbox probe hashes). The build program writes
// output/build-log.json for diagnostics, but that file is sandbox-writable
// and cannot prove that the pinned process ran. W01 build evidence therefore
// requires a server-observed execution receipt; without one it remains
// unobserved (not_run).

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
	// Mirror the Python derivation exactly: sort the complete "name:sha"
	// byte strings (not just the names) so a future dependency whose name is
	// a prefix of another cannot diverge between the two implementations.
	entries := make([]string, 0, len(dependencies))
	for name, sha := range dependencies {
		entries = append(entries, name+":"+sha)
	}
	sort.Strings(entries)
	payload := &bytes.Buffer{}
	payload.WriteString(craftWebToolchainName)
	payload.WriteByte(0)
	payload.WriteString(templateSHA)
	payload.WriteByte(0)
	for i, entry := range entries {
		if i > 0 {
			payload.WriteByte(0)
		}
		payload.WriteString(entry)
	}
	payload.WriteByte(0)
	payload.WriteString(buildSHA)
	sum := sha256.Sum256(payload.Bytes())
	return hex.EncodeToString(sum[:])
}

// CraftWebToolchainPin is the pinned identity a build log must match before
// its exit status may become the collector's build evidence. RuntimeDigest
// optionally carries the deployment's own runtime identity (from
// craftRuntimeDigestFromEnv): when set, a log naming any other runtime is
// refused whole.
type CraftWebToolchainPin struct {
	ToolchainDigest string
	TemplateVersion string
	TemplateSHA256  string
	RuntimeDigest   string
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
	// The stream must be exactly exhausted: anything other than io.EOF after
	// the primary decode — valid trailing JSON or garbage alike — means the
	// lock carries extra content and is refused.
	if err := dec.Decode(new(any)); !errors.Is(err, io.EOF) {
		return CraftWebToolchainPin{}, fmt.Errorf("%w: craft web toolchain lock carries trailing content", craft.ErrInvalidInput)
	}
	if lock.Schema != 1 {
		return CraftWebToolchainPin{}, fmt.Errorf("%w: craft web toolchain lock must be schema 1, got %d", craft.ErrInvalidInput, lock.Schema)
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
	if !sameStrings(listed, fixed) {
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
	// Read failures surface as deployment errors (ErrInvalidInput chain), so
	// ErrConflict stays reserved for "bytes were read but differ from the
	// pin" — a mis-deployed directory is not tampering.
	if got, err := fileDigest(filepath.Join(dir, "template.html")); err != nil {
		return CraftWebToolchainPin{}, err
	} else if got != lock.Template.SHA256 {
		return CraftWebToolchainPin{}, fmt.Errorf("%w: craft web template bytes differ from the pin", craft.ErrConflict)
	}
	deps := map[string]string{}
	for _, name := range craftWebFixedDependencies {
		got, err := fileDigest(filepath.Join(dir, "deps", name))
		if err != nil {
			return CraftWebToolchainPin{}, err
		}
		if got != byName[name] {
			return CraftWebToolchainPin{}, fmt.Errorf("%w: craft web dependency %s bytes differ from the pin", craft.ErrConflict, name)
		}
		deps[name] = got
	}
	if got, err := fileDigest(filepath.Join(dir, "build.py")); err != nil {
		return CraftWebToolchainPin{}, err
	} else if got != lock.BuildProgram.SHA256 {
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
	Schema          int    `json:"schema"`
	Kind            string `json:"kind"`
	RuntimeDigest   string `json:"runtime_digest"`
	ToolchainDigest string `json:"toolchain_digest"`
	TemplateVersion string `json:"template_version"`
	TemplateSHA256  string `json:"template_sha256"`
	// ExitCode is a pointer so a missing exit_code field (or an explicit
	// null) is distinguishable from a genuine 0: the log is agent-writable
	// untrusted input and a missing field must never decode as "build
	// succeeded".
	ExitCode *int     `json:"exit_code"`
	Entry    string   `json:"entry"`
	Assets   []string `json:"assets"`
	Egress   string   `json:"egress"`
	Error    string   `json:"error"`
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
	// The stream must be exactly exhausted (io.EOF): both a valid trailing
	// JSON document and trailing garbage mean the log carries extra content.
	if err := dec.Decode(new(any)); !errors.Is(err, io.EOF) {
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
	if log.ExitCode == nil {
		return CraftWebBuildLog{}, fmt.Errorf("%w: craft web build log is missing its exit code", craft.ErrInvalidInput)
	}
	if *log.ExitCode < 0 || *log.ExitCode > 255 {
		return CraftWebBuildLog{}, fmt.Errorf("%w: craft web build log exit code %d is impossible", craft.ErrInvalidInput, *log.ExitCode)
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
	if strings.HasPrefix(asset, "/") {
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

// CraftWebBuildEvidence cross-checks a build log against a server-observed
// execution result before the result can become W01 evidence. The build log
// lives in a sandbox-writable directory, so its own exit_code is never an
// authority. A nil observedExitCode means no trusted execution receipt is
// available and fails closed.
func CraftWebBuildEvidence(log CraftWebBuildLog, pin CraftWebToolchainPin, observedExitCode *int) (craft.ArtifactEvidence, error) {
	if observedExitCode == nil {
		return craft.ArtifactEvidence{}, fmt.Errorf("%w: server-observed web build execution receipt is unavailable", craft.ErrConflict)
	}
	if log.ToolchainDigest != pin.ToolchainDigest {
		return craft.ArtifactEvidence{}, fmt.Errorf("%w: build log names toolchain %s, deployment pins %s", craft.ErrConflict, log.ToolchainDigest, pin.ToolchainDigest)
	}
	if log.TemplateSHA256 != pin.TemplateSHA256 {
		return craft.ArtifactEvidence{}, fmt.Errorf("%w: build log names template %s, deployment pins %s", craft.ErrConflict, log.TemplateSHA256, pin.TemplateSHA256)
	}
	// The toolchain digest derivation does not include the version string,
	// so the log's declared version is compared directly: a log claiming a
	// different template version than the pinned one is a foreign log even
	// when every digest matches.
	if log.TemplateVersion != pin.TemplateVersion {
		return craft.ArtifactEvidence{}, fmt.Errorf("%w: build log names template version %s, deployment pins %s", craft.ErrConflict, log.TemplateVersion, pin.TemplateVersion)
	}
	// The runtime identity participates in the same agreement: when the
	// deployment pinned its own runtime digest, a log naming any other
	// runtime (a stale tree, another serve instance) is foreign even when
	// the toolchain pins match.
	if pin.RuntimeDigest != "" && log.RuntimeDigest != pin.RuntimeDigest {
		return craft.ArtifactEvidence{}, fmt.Errorf("%w: build log names runtime %s, deployment runs %s", craft.ErrConflict, log.RuntimeDigest, pin.RuntimeDigest)
	}
	if log.ExitCode == nil {
		// Parse guarantees a non-nil pointer for canonical logs; a caller
		// that constructed the struct directly must not panic here.
		return craft.ArtifactEvidence{}, fmt.Errorf("%w: build log carries no exit code", craft.ErrConflict)
	}
	if *observedExitCode < 0 || *observedExitCode > 255 || *log.ExitCode != *observedExitCode {
		return craft.ArtifactEvidence{}, fmt.Errorf("%w: build log exit code does not match the server-observed execution receipt", craft.ErrConflict)
	}
	return craft.ArtifactEvidence{BuildRan: true, BuildExitCode: *observedExitCode}, nil
}

// craftWebBuildEvidenceSource wraps one preview evidence source with the
// delegated run's build log: preview verdicts keep flowing, and the build
// fact is added only when the log exists, decodes under the strict contract,
// matches the pinned toolchain AND the F08 T-1 dispatch receipt supplies the
// trusted exit status (readReceipt). Anything else — missing file, malformed
// JSON, foreign toolchain, missing receipt — leaves the build fact exactly as
// the inner source reported it (unobserved), never fabricated. A nil
// readReceipt keeps the historical fail-closed not_run behavior.
func craftWebBuildEvidenceSource(
	inner service.ArtifactEvidenceSource,
	readLog func(context.Context, craft.Task) ([]byte, error),
	pin CraftWebToolchainPin,
	readReceipt func(context.Context, craft.Task) (*int, error),
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
			// Missing file is the common "no build ran" case and stays
			// silent; a read failure beyond not-exist is worth a trace.
			if err != nil && !errors.Is(err, fs.ErrNotExist) {
				logger.Warnf(ctx, "[CraftWebBuild] build log read failed for run %s: %v", task.Fence.RunID, err)
			}
			if err == nil && len(raw) == 0 {
				// A present-but-EMPTY log is the classic truncation tamper
				// shape: the decision stays unobserved (fail-closed) but the
				// visibility gap between "absent" and "zeroed" must be
				// logged like any other tamper/drift signal.
				logger.Warnf(ctx, "[CraftWebBuild] build log for run %s is present but empty (possible truncation)", task.Fence.RunID)
			}
			return evidence
		}
		log, err := ParseCraftWebBuildLog(raw)
		if err != nil {
			// A present-but-malformed log is a tamper/drift signal the
			// operator must be able to see; the decision stays unobserved.
			logger.Warnf(ctx, "[CraftWebBuild] rejecting malformed build log for run %s: %v", task.Fence.RunID, err)
			return evidence
		}
		// The session artifact source exposes only sandbox-writable files and
		// carries no trusted process outcome: the log alone can never
		// establish the build fact. The trusted exit status comes exclusively
		// from the server-owned dispatch receipt re-read here; without a
		// terminal started receipt the fact stays unobserved (fail-closed).
		observedExitCode := (*int)(nil)
		if readReceipt != nil {
			code, receiptErr := readReceipt(ctx, task)
			observedExitCode = code
			if receiptErr != nil && !errors.Is(receiptErr, repository.ErrCraftWebBuildReceiptNotFound) {
				logger.Warnf(ctx, "[CraftWebBuild] receipt read failed for run %s: %v", task.Fence.RunID, receiptErr)
			}
		}
		build, err := CraftWebBuildEvidence(log, pin, observedExitCode)
		if err != nil {
			// A log naming a foreign toolchain is refused whole — and is
			// precisely the alert-worthy case, so it is never silent.
			logger.Warnf(ctx, "[CraftWebBuild] refusing build log for run %s: %v", task.Fence.RunID, err)
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

// fileDigest hashes one file. A read failure is distinguished from a
// content mismatch so a mis-deployed toolchain directory (missing files)
// reports a deployment error instead of being mislabeled as tampering.
func fileDigest(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read toolchain file %s: %w", path, err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}
