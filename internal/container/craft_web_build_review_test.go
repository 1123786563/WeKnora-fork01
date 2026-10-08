package container

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// T04 (#123) deferred item (ledger 2026-09-23-craft-107-ledger.md:533): the
// offline build command entry consumes the T03 uploaded-material execution
// policy. These tests drive the command-review entry at its public seam — a
// staged normal-exec request carrying the durable Run identity — through the
// REAL T03 gate adapter (in-memory durable run snapshot, parameter-bound
// reload) and the REAL shipped toolchain pin. Every refusal must carry the
// member-visible text and must not be silent.

// craftWebBuildReviewFixture is one seeded durable run plus the real T03 gate
// adapter, mirroring the wiring-test seed shape from the service package.
type craftWebBuildReviewFixture struct {
	db       *gorm.DB
	gate     *service.CraftDelegateExecutionPolicy
	uploaded craft.Input
}

const (
	craftWebReviewTenant = uint64(9404)
	craftWebReviewRun    = "run-t04-build"
	craftWebReviewTask   = "task-t04-build"
)

func newCraftWebBuildReviewFixture(t *testing.T) *craftWebBuildReviewFixture {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "t04-build-cmd.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE agent_runs (tenant_id INTEGER, run_id TEXT, session_id TEXT, owner_id TEXT, snapshot TEXT)`).Error)

	content := []byte("print('data')\n")
	sum := sha256.Sum256(content)
	digest := hex.EncodeToString(sum[:])
	uploaded := craft.Input{
		Ref: "resource://t04-build-input-1", Name: "analyze.py",
		SHA256: digest, Bytes: int64(len(content)), CitationID: digest,
	}
	snapshot, err := json.Marshal(map[string]any{
		"version": 1, "query": "build the region page", "model_id": "model-1",
		"agent_config":         json.RawMessage(`{}`),
		"craft_input_manifest": []craft.Input{uploaded},
		"craft_workspace_seed": service.CraftWorkspaceSeedSnapshot{WorkspaceID: "ws-t04-build", State: craft.DraftHeadEmpty},
	})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`INSERT INTO agent_runs (tenant_id, run_id, session_id, owner_id, snapshot) VALUES (?, ?, ?, ?, ?)`,
		craftWebReviewTenant, craftWebReviewRun, craftWebReviewTask, "u-t04", string(snapshot)).Error)

	delegate := service.NewCraftDelegateService(repository.NewCraftStore(db), nil)
	gate, err := service.NewCraftDelegateExecutionPolicy(delegate, db, "/workspace")
	require.NoError(t, err)
	return &craftWebBuildReviewFixture{db: db, gate: gate, uploaded: uploaded}
}

// craftWebPinnedBuildCommand is the exact command shape the craft-web-build
// skill stages inside the sandbox.
func craftWebPinnedBuildCommand() []string {
	return []string{
		"python3", "/opt/craft/web/build.py",
		"--toolchain", "/opt/craft/web",
		"--input", "/workspace/material",
		"--output", "/workspace/output",
		"--runtime-digest", "sha256:" + strings.Repeat("ab", 32),
	}
}

func craftWebBuildRequest(command []string, environment map[string]string) repository.CraftDockerNormalInputRequest {
	return repository.CraftDockerNormalInputRequest{
		TenantID: craftWebReviewTenant, RunID: craftWebReviewRun, TaskID: craftWebReviewTask,
		WorkingDir: "/workspace", Command: command, Environment: environment,
	}
}

// craftWebShippedToolchainClone copies the shipped pinned toolchain into a
// temp directory so drift and symlink scenarios cannot touch the repo files.
func craftWebShippedToolchainClone(t *testing.T) string {
	t.Helper()
	clone := t.TempDir()
	require.NoError(t, exec.Command("cp", "-R", craftWebToolchainAbsDir(t)+"/.", clone).Run())
	return clone
}

// TestCraftWebBuildCommandGateReviewsPinnedCommandThroughT03Gate is the T04
// command-entry journey: the pinned build command passes the same gate the
// exec services enforce, admitted material smuggled through the environment
// is refused with the member-visible refusal, and the refusal names the
// uploaded identity.
func TestCraftWebBuildCommandGateReviewsPinnedCommandThroughT03Gate(t *testing.T) {
	fixture := newCraftWebBuildReviewFixture(t)
	pin, err := LoadCraftWebToolchainPin(craftWebToolchainAbsDir(t))
	require.NoError(t, err)

	reviewer, err := NewCraftWebBuildCommandGate(fixture.gate, pin, craftWebToolchainAbsDir(t))
	require.NoError(t, err, "shipped toolchain must assemble the command gate")

	// Success path: the exact pinned command shape passes the same gate.
	require.NoError(t, reviewer.Review(context.Background(), craftWebBuildRequest(craftWebPinnedBuildCommand(), nil)),
		"the pinned offline build command must pass the T03 gate")

	// Reading staged material as data is the build's own job and stays
	// allowed: the --input value naming the admitted upload passes.
	dataCommand := craftWebPinnedBuildCommand()
	dataCommand[5] = "/workspace/inputs/" + fixture.uploaded.SHA256 + "/analyze.py"
	require.NoError(t, reviewer.Review(context.Background(), craftWebBuildRequest(dataCommand, nil)),
		"naming admitted material as the build's data input must stay allowed")

	// Highest-risk refusal: smuggle the uploaded program through a python
	// startup hook — executing uploaded material must be refused by the SAME
	// gate with the member-visible refusal text.
	err = reviewer.Review(context.Background(), craftWebBuildRequest(craftWebPinnedBuildCommand(), map[string]string{
		"PYTHONSTARTUP": "/workspace/inputs/" + fixture.uploaded.SHA256 + "/analyze.py",
	}))
	require.ErrorIs(t, err, craft.ErrForbidden)
	require.Contains(t, err.Error(), "Allowed alternative", "the refusal must be member-visible")
}

// TestCraftWebBuildCommandGateFailsClosedWithoutPolicy proves the entry never
// dispatches an unreviewed command: without the T03 gate the review itself
// refuses, loudly.
func TestCraftWebBuildCommandGateFailsClosedWithoutPolicy(t *testing.T) {
	pin, err := LoadCraftWebToolchainPin(craftWebToolchainAbsDir(t))
	require.NoError(t, err)
	reviewer, err := NewCraftWebBuildCommandGate(nil, pin, craftWebToolchainAbsDir(t))
	require.NoError(t, err, "assembling without a policy is allowed; reviewing is not")
	err = reviewer.Review(context.Background(), craftWebBuildRequest(craftWebPinnedBuildCommand(), nil))
	require.ErrorIs(t, err, craft.ErrForbidden)
	require.Contains(t, err.Error(), "craft web build command")
}

// TestCraftWebBuildCommandGateRejectsForeignCommandShapes pins the fixed
// command shape: nothing but the pinned build program reaches the gate, so
// the entry cannot be borrowed to dispatch arbitrary commands.
func TestCraftWebBuildCommandGateRejectsForeignCommandShapes(t *testing.T) {
	fixture := newCraftWebBuildReviewFixture(t)
	pin, err := LoadCraftWebToolchainPin(craftWebToolchainAbsDir(t))
	require.NoError(t, err)
	reviewer, err := NewCraftWebBuildCommandGate(fixture.gate, pin, craftWebToolchainAbsDir(t))
	require.NoError(t, err)

	shapes := [][]string{
		{"python3", "/tmp/evil.py"},                                         // not the pinned build program
		{"python3", "/opt/craft/web/build.py", "--evil", "1"},               // unknown flag
		{"python3", "/opt/craft/web/build.py", "--toolchain", "/elsewhere"}, // toolchain redirected
		{"timeout", "30", "python3", "/opt/craft/web/build.py"},             // wrapper shape
		{"bash", "/opt/craft/web/build.py"},                                 // foreign interpreter
		{"python3", "-c", "print('boom')"},                                  // inline program text
	}
	for _, command := range shapes {
		err := reviewer.Review(context.Background(), craftWebBuildRequest(command, nil))
		require.ErrorIs(t, err, craft.ErrForbidden, "shape %v must be refused", command)
		require.Contains(t, err.Error(), "craft web build command", "refusal names the fixed shape for the member: %v", command)
	}
}

// TestCraftWebBuildCommandGateRejectsToolchainDrift proves the host-side
// review re-verifies the pinned bytes before screening: a build program whose
// bytes no longer match the pin refuses the dispatch (deployment drift is not
// member-visible permission, so the error carries the conflict identity).
func TestCraftWebBuildCommandGateRejectsToolchainDrift(t *testing.T) {
	fixture := newCraftWebBuildReviewFixture(t)
	shipped, err := LoadCraftWebToolchainPin(craftWebToolchainAbsDir(t))
	require.NoError(t, err)

	drifted := craftWebShippedToolchainClone(t)
	require.NoError(t, os.WriteFile(filepath.Join(drifted, "build.py"), []byte("# drifted\n"), 0o644))

	_, err = NewCraftWebBuildCommandGate(fixture.gate, shipped, drifted)
	require.Error(t, err, "a drifted toolchain must fail assembly, not review")
	require.ErrorIs(t, err, craft.ErrConflict)
}

// TestCraftWebBuildCommandGateRejectsSymlinkEscape proves the entry evaluates
// symlinks BEFORE screening (the ledger :530 contract for server-resolvable
// surfaces): a build program that resolves outside the pinned toolchain's own
// directory refuses even when the bytes still match the pin.
func TestCraftWebBuildCommandGateRejectsSymlinkEscape(t *testing.T) {
	fixture := newCraftWebBuildReviewFixture(t)
	shipped, err := LoadCraftWebToolchainPin(craftWebToolchainAbsDir(t))
	require.NoError(t, err)

	root := t.TempDir()
	toolchain := filepath.Join(root, "toolchain")
	require.NoError(t, exec.Command("cp", "-R", craftWebToolchainAbsDir(t)+"/.", toolchain).Run())
	// Move the build program out of the directory, then link it back: the
	// bytes are unchanged but the resolved target escapes the pinned tree.
	outer := filepath.Join(root, "outer-build.py")
	require.NoError(t, os.Rename(filepath.Join(toolchain, "build.py"), outer))
	require.NoError(t, os.Symlink(outer, filepath.Join(toolchain, "build.py")))

	reviewer, err := NewCraftWebBuildCommandGate(fixture.gate, shipped, toolchain)
	require.NoError(t, err, "bytes still match the pin at assembly time")
	err = reviewer.Review(context.Background(), craftWebBuildRequest(craftWebPinnedBuildCommand(), nil))
	require.ErrorIs(t, err, craft.ErrConflict)
	require.Contains(t, err.Error(), "symlink")
}

// TestCraftWebBuildCommandGateRejectsHostInputsRootOverlap proves the
// host-side resolved toolchain root must never overlap the staged read-only
// inputs tree: a toolchain directory that is itself a link into the material
// tree refuses dispatch.
func TestCraftWebBuildCommandGateRejectsHostInputsRootOverlap(t *testing.T) {
	fixture := newCraftWebBuildReviewFixture(t)
	shipped, err := LoadCraftWebToolchainPin(craftWebToolchainAbsDir(t))
	require.NoError(t, err)

	root := t.TempDir()
	hostInputsRoot := filepath.Join(root, "material", "inputs")
	require.NoError(t, os.MkdirAll(hostInputsRoot, 0o755))
	clone := filepath.Join(hostInputsRoot, "toolchain-clone")
	require.NoError(t, exec.Command("cp", "-R", craftWebToolchainAbsDir(t)+"/.", clone).Run())
	link := filepath.Join(root, "toolchain-link")
	require.NoError(t, os.Symlink(clone, link))

	reviewer, err := NewCraftWebBuildCommandGate(fixture.gate, shipped, link, WithCraftWebHostInputsRoot(hostInputsRoot))
	require.NoError(t, err, "bytes match; only the resolved location is hostile")
	err = reviewer.Review(context.Background(), craftWebBuildRequest(craftWebPinnedBuildCommand(), nil))
	require.ErrorIs(t, err, craft.ErrConflict)
	require.Contains(t, err.Error(), "inputs")
}

// TestCraftWebBuildCommandGateRejectsPathOverride is the round-2 medium
// finding regression: argv[0] is a bare name resolved by the executor's
// PATH, so a request carrying a PATH override (plus a writable-dir wrapper)
// must be refused at this gate.
func TestCraftWebBuildCommandGateRejectsPathOverride(t *testing.T) {
	fixture := newCraftWebBuildReviewFixture(t)
	pin, err := LoadCraftWebToolchainPin(craftWebToolchainAbsDir(t))
	require.NoError(t, err)
	reviewer, err := NewCraftWebBuildCommandGate(fixture.gate, pin, craftWebToolchainAbsDir(t))
	require.NoError(t, err)

	overridden := craftWebBuildRequest(craftWebPinnedBuildCommand(), map[string]string{
		"PATH": "/tmp/evil:/usr/bin",
	})
	require.ErrorIs(t, reviewer.Review(context.Background(), overridden), craft.ErrForbidden,
		"a PATH override must not borrow the bare-name argv[0] resolution")

	clean := craftWebBuildRequest(craftWebPinnedBuildCommand(), map[string]string{"LC_ALL": "C"})
	require.NoError(t, reviewer.Review(context.Background(), clean))
}

func TestCraftWebBuildCommandShapeRejectsUnconstrainedFlags(t *testing.T) {
	base := craftWebPinnedBuildCommand()
	valid := func(command []string) bool { return craftWebBuildCommandShapeOk(command, "") }
	require.True(t, valid(base))

	for name, mutate := range map[string]func([]string) []string{
		"repeated input": func(command []string) []string {
			return append(command, "--input", "/workspace/other")
		},
		"repeated output": func(command []string) []string {
			return append(command, "--output", "/workspace/other")
		},
		"repeated runtime digest": func(command []string) []string {
			return append(command, "--runtime-digest", "forged")
		},
		"input escape": func(command []string) []string {
			changed := append([]string(nil), command...)
			changed[5] = "/workspace/material/../outside"
			return changed
		},
		"input backslash alias": func(command []string) []string {
			changed := append([]string(nil), command...)
			changed[5] = "/workspace/material\\..\\outside"
			return changed
		},
		"input outside workspace": func(command []string) []string {
			changed := append([]string(nil), command...)
			changed[5] = "/tmp/content"
			return changed
		},
		"output escape": func(command []string) []string {
			changed := append([]string(nil), command...)
			changed[7] = "/workspace/output/../tmp"
			return changed
		},
		"output outside fixed destination": func(command []string) []string {
			changed := append([]string(nil), command...)
			changed[7] = "/tmp/output"
			return changed
		},
	} {
		t.Run(name, func(t *testing.T) {
			require.False(t, valid(mutate(append([]string(nil), base...))))
		})
	}
}

func TestCraftWebBuildCommandShapeRequiresPinnedRuntimeDigest(t *testing.T) {
	command := craftWebPinnedBuildCommand()
	require.True(t, craftWebBuildCommandShapeOk(command, "sha256:"+strings.Repeat("ab", 32)))
	require.False(t, craftWebBuildCommandShapeOk(command, "sha256:"+strings.Repeat("cd", 32)))
}

// TestCraftWebBuildCommandGateNilReceiverRefusesNotPanics is the round-2
// high-finding regression: a nil gate (an assembly failure returning nil is
// the designed defensive case) must REFUSE, not panic on the field access
// that used to precede the nil check.
func TestCraftWebBuildCommandGateNilReceiverRefusesNotPanics(t *testing.T) {
	var gate *CraftWebBuildCommandGate
	request := repository.CraftDockerNormalInputRequest{RunID: "run-nil"}
	require.NotPanics(t, func() {
		err := gate.Review(context.Background(), request)
		require.ErrorIs(t, err, craft.ErrForbidden)
	})
}
