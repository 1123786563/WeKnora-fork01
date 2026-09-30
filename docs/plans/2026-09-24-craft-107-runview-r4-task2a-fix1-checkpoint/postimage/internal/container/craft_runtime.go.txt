// Package container — env-driven Craft runtime assembly (W06, coordinator
// authorized extension of the R05 fail-closed boundary).
//
// CRAFT_OPENCODE_BASE_URL names one pinned OpenCode serve endpoint (R01
// client, protocol-locked binary). When it is set, this file assembles the
// REAL execution chain the craft_delegate tool drives:
//
//	R01 Client -> R04 Executor -> CraftDelegateService   (R05 real chain)
//
// plus the two deployment-level halves the R03 sandbox resolver owns in the
// containerised deployment, mapped onto the local single-serve runtime:
//
//   - workspace provisioning: the first delegation for a craft session
//     creates the OpenCode session and CAS-binds it into the craft workspace
//     row (the R03 resolver does this per sandbox binding in production);
//   - per-session working directories: every session gets ws/<session-id>/
//     under the serve working directory — inputs stage into
//     ws/<sid>/inputs/<sha256>/<name>, artifacts are collected from
//     ws/<sid>/output/ — so one shared serve still isolates sessions;
//   - artifact publication: a finished delegation's output is collected
//     through W01's CraftArtifactService into an immutable craft.Version and
//     announced with an artifact.published run event, which is what makes the
//     workbench preview/download/version UI real;
//   - craft events: the executor's delegation.* events are appended to the
//     durable run event stream (same fence, same seq counter) so the browser
//     SSE subscription projects them.
//
// When CRAFT_OPENCODE_BASE_URL is unset the provider keeps R05's
// NewUnavailableCraftExecutor: craft stays fail-closed and the default-off
// semantics of the deployment are unchanged.
package container

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/opencode"
	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/Tencent/WeKnora/internal/modules/execution/sandbox"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"golang.org/x/sys/unix"
	"gorm.io/gorm"
)

// Env switches of the local craft runtime assembly.
const (
	// craftOpenCodeBaseURLEnv points the real executor chain at the pinned
	// OpenCode serve endpoint. Empty keeps the fail-closed executor.
	craftOpenCodeBaseURLEnv = "CRAFT_OPENCODE_BASE_URL"
	// craftOpenCodeWorkDirEnv is the serve process working directory; the
	// per-session workspace directories are created under it.
	craftOpenCodeWorkDirEnv = "CRAFT_OPENCODE_WORK_DIR"
	// craftOpenCodeOutputDirEnv selects the workspace-relative artifact
	// output directory the W01 collector scans (default "output").
	craftOpenCodeOutputDirEnv = "CRAFT_OPENCODE_OUTPUT_DIR"
	// craftOpenCodeRuntimeDigestEnv overrides the runtime digest stamped
	// into provisioned workspaces (default marks the local serve runtime).
	craftOpenCodeRuntimeDigestEnv = "CRAFT_OPENCODE_RUNTIME_DIGEST"
	// craftLocalRuntimeDigest is the honest default identity of the local
	// single-serve runtime: it is not a reproducible image digest.
	craftLocalRuntimeDigest = "local-opencode-serve"
	// craftLocalOutputDir is the workspace-relative artifact directory.
	craftLocalOutputDir = "output"
	// craftLocalStagedReadBudget bounds one staged input read.
	craftLocalStagedReadBudget = craft.MaxInputBytes + 1
	// craftLocalMaxReadBytes guards one artifact read (the W01 collector
	// enforces its own per-file cap on top of this).
	craftLocalMaxReadBytes                 = int64(50 << 20)
	runBoundArtifactMaxEntries             = 4096
	runBoundArtifactMaxEntriesPerDirectory = 1024
)

// newCraftRuntimeExecutor assembles the craft.Executor for the container:
// the env-driven real chain when CRAFT_OPENCODE_BASE_URL is configured, the
// unchanged R05 fail-closed executor otherwise.
func newCraftRuntimeExecutor(
	db *gorm.DB,
	store craft.Store,
	runs *repository.AgentRunStore,
	files interfaces.FileService,
	versions craft.VersionStore,
	previews *service.CraftPreviewService,
) (craft.Executor, error) {
	baseURL := strings.TrimSpace(os.Getenv(craftOpenCodeBaseURLEnv))
	if baseURL == "" {
		// Default-off: exactly the R05 assembly boundary.
		return service.NewUnavailableCraftExecutor(
			"the craft opencode runtime dial is not assembled (set CRAFT_OPENCODE_BASE_URL to enable the local real runtime)"), nil
	}
	workDir := strings.TrimSpace(os.Getenv(craftOpenCodeWorkDirEnv))
	if workDir == "" {
		return nil, fmt.Errorf("%s requires %s to name the opencode serve working directory",
			craftOpenCodeBaseURLEnv, craftOpenCodeWorkDirEnv)
	}
	info, err := os.Stat(workDir)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("craft local runtime work dir %q is not a directory: %v", workDir, err)
	}
	outputDir := strings.TrimSpace(os.Getenv(craftOpenCodeOutputDirEnv))
	if outputDir == "" {
		outputDir = craftLocalOutputDir
	}
	runtimeDigest := craftRuntimeDigestFromEnv()
	client, err := opencode.NewClient(baseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("craft local runtime client %s: %w", baseURL, err)
	}
	var evidence service.ArtifactEvidenceSource
	if previews != nil {
		evidence = previews.EvidenceSource()
	}
	source := &localCraftArtifactSource{workDir: workDir, outputDir: outputDir}
	artifacts := service.NewCraftArtifactService(source, files, versions, evidence,
		service.CraftArtifactConfig{Kind: craft.KindWeb, OutputDir: outputDir})
	// C02: an interaction.pending event first lands durably (interaction row
	// + waiting_user park) before it is projected to the run stream, so the
	// pending decision is decidable through the HTTP surface. The registrar
	// is installed AFTER construction (wireCraftInteractionRegistrar breaks
	// the executor → interaction assembly → agent runtime provider cycle),
	// so the inner executor's emission path must read the runtime's CURRENT
	// emitter: the closure below indirections every sub-execution event
	// through runtime.emit instead of capturing the plain emitter at
	// construction. Once the registrar is wired, interaction.pending flows
	// through craftInteractionRegistrar — the same production emission point
	// BASE wrapped at construction — and every other kind passes straight
	// through to the durable run event stream.
	emit := craftRunEventEmitter(runs)
	runtime := &localCraftRuntime{
		db:            db,
		client:        client,
		store:         store,
		files:         files,
		artifacts:     artifacts,
		emit:          emit,
		outputDir:     outputDir,
		runtimeDigest: runtimeDigest,
		sessionsRoot:  filepath.Join(workDir, "ws"),
		workDir:       workDir,
	}
	runtime.inner = opencode.NewExecutor(client, store,
		func(ctx context.Context, task craft.Task, kind string, data json.RawMessage) error {
			return runtime.emit(ctx, task, kind, data)
		})
	logger.Infof(context.Background(),
		"[CraftRuntime] local real runtime assembled: serve=%s work_dir=%s output=%s", baseURL, workDir, outputDir)
	return runtime, nil
}

// localCraftRuntime implements craft.Executor for the local single-serve
// deployment: provision the OpenCode binding, stage authorized inputs, frame
// the sub-prompt with the session workspace, execute through the R04
// executor, then publish the finished output as an immutable version.
type localCraftRuntime struct {
	db               *gorm.DB
	client           *opencode.Client
	store            craft.Store
	files            interfaces.FileService
	inner            craft.Executor
	artifacts        *service.CraftArtifactService
	emit             func(context.Context, craft.Task, string, json.RawMessage) error
	workDir          string
	outputDir        string
	runtimeDigest    string
	sessionsRoot     string
	materialResolver func(context.Context, craft.Task) (CraftRunViewMaterialHandle, error)
	// knowledgeResolver must load the admitted Run and use its durable
	// selection plus exact accepted record/package for this material handle.
	knowledgeResolver       func(context.Context, craft.Task, CraftRunViewMaterialHandle) (CraftKnowledgeRunViewAcceptance, error)
	knowledgeVerifier       func(context.Context, craft.Task, CraftRunViewMaterialHandle, CraftKnowledgeRunViewAcceptance) error
	inputWriteBeforePublish func(int, string, string) error
	inputWriteAfterPublish  func(int, string, string) error
	inputPublishNoReplace   func(int, string, string) error
	snapshotCapture         func(context.Context, craft.Task, string)
}

// Execute runs one delegation through the real chain.
func (e *localCraftRuntime) Execute(ctx context.Context, task craft.Task) (craft.Result, error) {
	if e.materialResolver == nil {
		return craft.Result{}, unresolvedCraftRunView("verified RunView material resolver is not assembled", nil)
	}
	material, err := e.materialResolver(ctx, task)
	if err != nil {
		return craft.Result{}, err
	}
	if err := e.stageWorkspaceInputs(ctx, task, material); err != nil {
		return craft.Result{}, err
	}
	if e.knowledgeResolver == nil {
		return craft.Result{}, unresolvedCraftRunView("accepted Run knowledge package resolver is not assembled", nil)
	}
	accepted, err := e.knowledgeResolver(ctx, task, material)
	if err != nil {
		return craft.Result{}, err
	}
	if accepted.RunID != task.Fence.RunID || !validCraftKnowledgeRunViewDigest(accepted.PackageDigest) {
		return craft.Result{}, craft.ErrConflict
	}
	if err := e.ensureWorkspace(ctx, task); err != nil {
		return craft.Result{}, err
	}
	if err := e.seedRunViewWorkspaceOutput(ctx, task, material); err != nil {
		return craft.Result{}, err
	}
	adopted, aerr := e.adoptExecutorMessageID(ctx, task)
	if aerr != nil {
		return craft.Result{}, aerr
	}
	task = adopted
	if material.provider == nil {
		return craft.Result{}, unresolvedCraftRunView("verified material provider is missing before dispatch", nil)
	}
	if e.knowledgeVerifier == nil {
		return craft.Result{}, unresolvedCraftRunView("accepted Run knowledge verifier is not assembled", nil)
	}
	if err := verifyRunViewKnowledgeRoot(material, accepted.RunID); err != nil {
		return craft.Result{}, err
	}
	if err := e.knowledgeVerifier(ctx, task, material, accepted); err != nil {
		return craft.Result{}, err
	}
	if err := verifyRunViewKnowledgeRoot(material, accepted.RunID); err != nil {
		return craft.Result{}, err
	}
	if err := material.provider.RevalidateMaterialHandle(ctx, material); err != nil {
		return craft.Result{}, err
	}
	if _, err := e.admittedRunForCraftFence(ctx, task); err != nil {
		return craft.Result{}, err
	}
	result, err := e.inner.Execute(ctx, task)
	if err != nil {
		return result, err
	}
	if result.Status != "succeeded" {
		return result, nil
	}
	// R4 Task2a has a Run-bound output source but the later candidate and
	// post-terminal draft-capture seam is not yet assembled. Do not publish via
	// the legacy session-wide collector: that source follows a shared pointer
	// and would make a newly collected version current before T15's promotion
	// gate. Keep the execution result fail-closed until Task2b wires the
	// candidate path and terminal/quiescence writer.
	return craft.Result{}, unresolvedCraftRunView("RunView artifact candidate collection is not assembled", nil)
}

// verifyRunViewKnowledgeRoot constrains the complete read-only bind source to
// the one expected Run package. It opens each directory without following
// symlinks and rejects siblings at both mounted levels.
func verifyRunViewKnowledgeRoot(material CraftRunViewMaterialHandle, runID string) error {
	if craft.KnowledgeRunDir(runID) == "" || material.provider == nil {
		return craft.ErrConflict
	}
	if err := material.provider.verifyMaterialHandle(material); err != nil {
		return err
	}
	_, rootFD, inputsFD, err := openVerifiedRunViewInputs(material)
	if err != nil {
		return unresolvedCraftRunView("open exact RunView knowledge root", err)
	}
	defer unix.Close(inputsFD)
	defer unix.Close(rootFD)
	knowledgeFD, err := unix.Openat(rootFD, craft.KnowledgeDir, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return fmt.Errorf("%w: open RunView knowledge root: %v", craft.ErrConflict, err)
	}
	defer unix.Close(knowledgeFD)
	entries, err := readRunViewDirEntries(knowledgeFD)
	if err != nil || len(entries) != 1 || entries[0].Name() != path.Base(craft.KnowledgeRunsDir) {
		return fmt.Errorf("%w: mounted RunView knowledge root contains unexpected entries", craft.ErrConflict)
	}
	runsFD, err := unix.Openat(knowledgeFD, path.Base(craft.KnowledgeRunsDir), unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return fmt.Errorf("%w: open RunView knowledge runs directory: %v", craft.ErrConflict, err)
	}
	defer unix.Close(runsFD)
	runEntries, err := readRunViewDirEntries(runsFD)
	if err != nil || len(runEntries) != 1 || runEntries[0].Name() != runID {
		return fmt.Errorf("%w: mounted RunView knowledge root contains a sibling or missing Run", craft.ErrConflict)
	}
	runFD, err := unix.Openat(runsFD, runID, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return fmt.Errorf("%w: open accepted RunView knowledge package: %v", craft.ErrConflict, err)
	}
	defer unix.Close(runFD)
	var info unix.Stat_t
	if err := unix.Fstat(runFD, &info); err != nil || info.Mode&unix.S_IFMT != unix.S_IFDIR || info.Mode&07777 != 0555 {
		return fmt.Errorf("%w: accepted RunView knowledge package is not a sealed directory", craft.ErrConflict)
	}
	return nil
}

// sessionKind loads the craft session's artwork kind for one delegation.
func (e *localCraftRuntime) sessionKind(ctx context.Context, task craft.Task) string {
	if e.db == nil {
		return craft.KindWeb
	}
	var kind string
	if err := e.db.WithContext(ctx).Raw(
		"SELECT kind FROM craft_sessions WHERE tenant_id = ? AND session_id = ?",
		task.Fence.TenantID, task.Scope.SessionID).Scan(&kind).Error; err != nil || kind == "" {
		return craft.KindWeb
	}
	return kind
}

// Observe settles a dispatching-unknown delegation. The workspace is ensured
// best-effort so an interrupted first delegation can still be observed.
func (e *localCraftRuntime) Observe(ctx context.Context, task craft.Task) (craft.Observation, error) {
	if err := e.ensureWorkspace(ctx, task); err != nil {
		logger.Warnf(ctx, "[CraftRuntime] observe could not ensure the workspace: %v", err)
	}
	return e.inner.Observe(ctx, task)
}

// Abort cancels the sub-execution through the R04 executor.
func (e *localCraftRuntime) Abort(ctx context.Context, task craft.Task) error {
	return e.inner.Abort(ctx, task)
}

// ensureWorkspace provisions the OpenCode session binding for the task's
// session on first use (the local mapping of the R03 resolver's dial +
// CreateSession), reusing the recorded binding on every later round.
func (e *localCraftRuntime) ensureWorkspace(ctx context.Context, task craft.Task) error {
	workspace, err := e.store.GetWorkspace(ctx, task.Scope)
	if err != nil {
		return err
	}
	if workspace.OpenCodeSessionID != "" {
		return nil
	}
	sessionID, cerr := e.client.CreateSession(ctx)
	if cerr != nil {
		return fmt.Errorf("craft local runtime create opencode session for %s: %w", task.Scope.SessionID, cerr)
	}
	if workspace.SandboxID == "" {
		workspace.SandboxID = "local-opencode-serve"
	}
	if workspace.Generation == "" {
		workspace.Generation = "1"
	}
	workspace.OpenCodeSessionID = sessionID
	workspace.RuntimeDigest = e.runtimeDigest
	if _, perr := e.store.PutWorkspace(ctx, workspace, workspace.Revision); perr != nil {
		if errors.Is(perr, craft.ErrConflict) {
			// Another worker provisioned first: the recorded binding wins.
			if winner, gerr := e.store.GetWorkspace(ctx, task.Scope); gerr == nil && winner.OpenCodeSessionID != "" {
				return nil
			}
		}
		return perr
	}
	logger.Infof(ctx, "[CraftRuntime] provisioned opencode session %s for craft session %s",
		sessionID, task.Scope.SessionID)
	return nil
}

// adoptExecutorMessageID rewrites the delegation row's prompt message id
// into the pinned OpenCode id space before dispatch. R02's store
// default-fills a UUID when the delegation service first prepares the task,
// while the R04 executor validates every persisted id against its message-id
// wrap window — left as-is, the first Delegate→Execute hand-off parks every
// delegation as outcome-unknown. The adoption happens once: rows already
// carrying an executor-space id ("msg_") keep it, so retries never mint a
// second id for the same delegation.
func (e *localCraftRuntime) adoptExecutorMessageID(ctx context.Context, task craft.Task) (craft.Task, error) {
	if _, err := e.admittedRunForCraftFence(ctx, task); err != nil {
		return craft.Task{}, err
	}
	if e.db == nil {
		return task, nil
	}
	var stored struct {
		TaskJSON        string `gorm:"column:task_json"`
		PromptMessageID string `gorm:"column:prompt_message_id"`
	}
	query := func() error {
		return e.db.WithContext(ctx).Table("craft_delegations").Select("task_json, prompt_message_id").Where(
			"tenant_id = ? AND run_id = ? AND tool_call_id = ? AND workspace_id = ?",
			task.Fence.TenantID, task.Fence.RunID, task.ToolCallID, task.WorkspaceID).Take(&stored).Error
	}
	if err := query(); err != nil {
		return craft.Task{}, fmt.Errorf("craft: load durable delegation before message adoption: %w", err)
	}
	storedTask, err := decodeCraftRuntimeDelegationTask(stored.TaskJSON)
	if err != nil || !sameCraftRuntimeDelegationRequest(storedTask, task) ||
		storedTask.PromptMessageID != stored.PromptMessageID {
		return craft.Task{}, fmt.Errorf("%w: durable delegation request changed before message adoption", craft.ErrConflict)
	}
	expectedTaskJSON := stored.TaskJSON
	expectedPromptID := stored.PromptMessageID
	for attempt := 0; attempt < 8; attempt++ {
		if strings.HasPrefix(stored.PromptMessageID, "msg_") {
			task.PromptMessageID = stored.PromptMessageID
			return task, nil
		}
		if task.PromptMessageID != stored.PromptMessageID {
			return craft.Task{}, fmt.Errorf("%w: durable delegation message id differs from request", craft.ErrConflict)
		}
		fresh, err := opencode.NewMessageID()
		if err != nil {
			return craft.Task{}, err
		}
		updatedTask := task
		updatedTask.PromptMessageID = fresh
		encoded, err := json.Marshal(updatedTask)
		if err != nil {
			return craft.Task{}, err
		}
		result := e.db.WithContext(ctx).Exec(
			`UPDATE craft_delegations SET task_json = ?, prompt_message_id = ?
			 WHERE tenant_id = ? AND run_id = ? AND tool_call_id = ? AND workspace_id = ?
			   AND task_json = ? AND prompt_message_id = ?
			   AND EXISTS (SELECT 1 FROM agent_runs WHERE tenant_id = ? AND run_id = ?
			       AND session_id = ? AND owner_id = ? AND epoch = ?)`,
			string(encoded), fresh, task.Fence.TenantID, task.Fence.RunID, task.ToolCallID, task.WorkspaceID,
			expectedTaskJSON, expectedPromptID,
			task.Fence.TenantID, task.Fence.RunID, task.Scope.SessionID, task.Scope.UserID, task.Fence.Epoch)
		if result.Error == nil && result.RowsAffected == 1 {
			logger.Infof(ctx, "[CraftRuntime] delegation %s adopted executor message id %s", task.ID, fresh)
			return updatedTask, nil
		}
		if result.Error != nil && !strings.Contains(strings.ToLower(result.Error.Error()), "locked") {
			// A transport error can happen after a database committed. Adopt only
			// when a scoped reload proves the matching durable winner.
			if _, fenceErr := e.admittedRunForCraftFence(ctx, task); fenceErr != nil {
				return craft.Task{}, fenceErr
			}
			if reloadErr := query(); reloadErr != nil {
				return craft.Task{}, result.Error
			}
			storedTask, err = decodeCraftRuntimeDelegationTask(stored.TaskJSON)
			if err != nil || !sameCraftRuntimeDelegationRequest(storedTask, task) ||
				storedTask.PromptMessageID != stored.PromptMessageID || !strings.HasPrefix(stored.PromptMessageID, "msg_") {
				return craft.Task{}, result.Error
			}
			task.PromptMessageID = stored.PromptMessageID
			return task, nil
		}
		// A zero-row CAS means another writer changed the row. SQLite shared
		// cache can also report SQLITE_LOCKED while two readers upgrade; reload
		// and retry only while the exact same old request is still present.
		if _, fenceErr := e.admittedRunForCraftFence(ctx, task); fenceErr != nil {
			return craft.Task{}, fenceErr
		}
		if result.Error != nil {
			timer := time.NewTimer(time.Duration(attempt+1) * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return craft.Task{}, ctx.Err()
			case <-timer.C:
			}
		}
		if err := query(); err != nil {
			if result.Error != nil {
				continue
			}
			return craft.Task{}, err
		}
		storedTask, err = decodeCraftRuntimeDelegationTask(stored.TaskJSON)
		if err != nil || !sameCraftRuntimeDelegationRequest(storedTask, task) ||
			storedTask.PromptMessageID != stored.PromptMessageID {
			return craft.Task{}, fmt.Errorf("%w: delegation message adoption CAS lost to a different request", craft.ErrConflict)
		}
		if strings.HasPrefix(stored.PromptMessageID, "msg_") {
			task.PromptMessageID = stored.PromptMessageID
			return task, nil
		}
		if stored.PromptMessageID != expectedPromptID || stored.TaskJSON != expectedTaskJSON {
			return craft.Task{}, fmt.Errorf("%w: delegation message adoption CAS lost its expected old value", craft.ErrConflict)
		}
	}
	return craft.Task{}, fmt.Errorf("%w: delegation message adoption remained contended", craft.ErrConflict)
}

func decodeCraftRuntimeDelegationTask(raw string) (craft.Task, error) {
	var task craft.Task
	if raw == "" {
		return task, fmt.Errorf("empty task_json")
	}
	if err := json.Unmarshal([]byte(raw), &task); err != nil {
		return craft.Task{}, err
	}
	return task, nil
}

// sameCraftRuntimeDelegationRequest mirrors the persisted PrepareTask request
// identity. A worker owner/epoch and the adopted executor message ID may
// change on recovery; caller-controlled request fields may not.
func sameCraftRuntimeDelegationRequest(a, b craft.Task) bool {
	if a.ToolCallID != b.ToolCallID || a.WorkspaceID != b.WorkspaceID || a.Prompt != b.Prompt ||
		a.RequestHash != b.RequestHash || !craft.SameScope(a.Scope, b.Scope) ||
		a.Fence.TenantID != b.Fence.TenantID || a.Fence.RunID != b.Fence.RunID ||
		len(a.Inputs) != len(b.Inputs) || len(a.SkillDigests) != len(b.SkillDigests) || !a.Deadline.Equal(b.Deadline) {
		return false
	}
	for i := range a.Inputs {
		if a.Inputs[i] != b.Inputs[i] {
			return false
		}
	}
	for i := range a.SkillDigests {
		if a.SkillDigests[i] != b.SkillDigests[i] {
			return false
		}
	}
	return true
}

// pointWorkspaceOutput binds this delegation's output directory: every
// delegation gets its own directory ws/<key>/ (key = sha256 of the exact
// delegation prompt, stable across retries) under the serve working
// directory, and the shared serve-relative "<outputDir>" path is repointed
// at it as a symlink. The sub-executor writes ordinary relative paths
// ("<outputDir>/index.html") and lands in this delegation's directory; the
// collector resolves the same symlink. The task itself is NEVER modified —
// the R04 executor re-prepares it durably and any prompt rewrite would
// collide with the delegation's stored request. Concurrent delegations on
// one shared serve race on the symlink (the containerised deployment gives
// each session its own sandbox instead); craft sessions run serially here.
func (e *localCraftRuntime) pointWorkspaceOutput(ctx context.Context, task craft.Task) error {
	sum := sha256.Sum256([]byte(task.Prompt))
	key := hex.EncodeToString(sum[:8])
	workspaceDir := filepath.Join(e.sessionsRoot, key)
	outputDir := filepath.Join(workspaceDir, e.outputDir)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return err
	}
	link := filepath.Join(e.workDir, e.outputDir)
	if existing, err := os.Readlink(link); err == nil {
		if existing == outputDir {
			return nil
		}
		if err := os.Remove(link); err != nil {
			return err
		}
	} else if _, serr := os.Stat(link); serr == nil {
		// A real directory blocks the pointer; refuse rather than delete user data.
		return fmt.Errorf("craft local runtime: %s exists and is not the runtime's pointer", link)
	}
	if err := os.Symlink(outputDir, link); err != nil {
		return err
	}
	logger.Infof(ctx, "[CraftRuntime] delegation %s output pointer -> %s", task.ID, outputDir)
	return nil
}

type craftRunViewWorkspaceSeed struct {
	WorkspaceID    string               `json:"workspace_id"`
	State          craft.DraftHeadState `json:"state"`
	DraftRevision  int64                `json:"draft_revision"`
	SourceRunID    string               `json:"source_run_id,omitempty"`
	ManifestDigest string               `json:"manifest_digest,omitempty"`
}

func validateCraftRunViewWorkspaceSeed(seed craftRunViewWorkspaceSeed, head craft.DraftHead) error {
	if seed.WorkspaceID == "" || seed.WorkspaceID != head.WorkspaceID || seed.DraftRevision != head.Revision || seed.State != head.State {
		return fmt.Errorf("%w: admitted Workspace predecessor identity changed or does not match", craft.ErrConflict)
	}
	switch seed.State {
	case craft.DraftHeadEmpty:
		if seed.DraftRevision != 0 || seed.SourceRunID != "" || seed.ManifestDigest != "" || head.SourceRunID != "" || head.ManifestDigest != "" || len(head.Files) != 0 {
			return fmt.Errorf("%w: malformed explicit empty Workspace predecessor", craft.ErrInvalidInput)
		}
	case craft.DraftHeadSelected:
		if seed.DraftRevision < 1 || seed.SourceRunID == "" || seed.ManifestDigest == "" ||
			seed.SourceRunID != head.SourceRunID || seed.ManifestDigest != head.ManifestDigest {
			return fmt.Errorf("%w: admitted Workspace predecessor manifest changed", craft.ErrConflict)
		}
	default:
		return fmt.Errorf("%w: unknown admitted Workspace predecessor state %q", craft.ErrInvalidInput, seed.State)
	}
	if err := head.Validate(); err != nil {
		return err
	}
	return nil
}

func (e *localCraftRuntime) seedRunViewWorkspaceOutput(ctx context.Context, task craft.Task, material CraftRunViewMaterialHandle) error {
	if material.provider == nil || material.generation == "" || material.key.TenantID != task.Fence.TenantID ||
		material.key.TenantID != task.Scope.TenantID || material.key.OwnerID != task.Scope.UserID ||
		material.key.SessionID != task.Scope.SessionID || material.key.RunID != task.Fence.RunID ||
		task.WorkspaceID == "" || e.db == nil {
		return fmt.Errorf("%w: material or Run is not bound to admitted Craft scope", craft.ErrForbidden)
	}
	if err := material.provider.verifyMaterialHandle(material); err != nil {
		return unresolvedCraftRunView("verify RunView material before draft seed", err)
	}
	row, err := e.admittedRunForCraftFence(ctx, task)
	if err != nil {
		return err
	}
	snapshot, err := service.ParseDurableRunSnapshot(json.RawMessage(row.Snapshot))
	if err != nil || snapshot.CraftWorkspaceSeed == nil {
		return fmt.Errorf("%w: admitted Run snapshot has no valid Workspace predecessor", craft.ErrConflict)
	}
	seed := craftRunViewWorkspaceSeed(*snapshot.CraftWorkspaceSeed)
	if seed.WorkspaceID != task.WorkspaceID {
		return fmt.Errorf("%w: admitted Run Workspace differs from delegation", craft.ErrForbidden)
	}
	head, err := repository.NewCraftDraftHeadStore(e.db).ReadRevision(ctx, task.Scope, task.WorkspaceID, seed.DraftRevision)
	if err != nil {
		return fmt.Errorf("craft RunView read frozen Workspace revision: %w", err)
	}
	if err := validateCraftRunViewWorkspaceSeed(seed, head); err != nil {
		return err
	}
	expected := make(map[string]craft.File, len(head.Files))
	var total int64
	for _, file := range head.Files {
		if err := craft.ValidateArtifactPath(file.Path); err != nil {
			return err
		}
		if file.Bytes < 0 || file.Bytes > craft.MaxDraftHeadBytes || total > craft.MaxDraftHeadBytes-file.Bytes {
			return fmt.Errorf("%w: Workspace predecessor exceeds seed quota", craft.ErrInvalidInput)
		}
		if _, exists := expected[file.Path]; exists {
			return fmt.Errorf("%w: duplicate Workspace predecessor path", craft.ErrInvalidInput)
		}
		if file.Bytes > craftLocalMaxReadBytes {
			return fmt.Errorf("%w: Workspace predecessor file exceeds runtime artifact read cap", craft.ErrInvalidInput)
		}
		expected[file.Path] = file
		total += file.Bytes
	}
	if err := validateRunViewDraftOutputPaths(expected); err != nil {
		return err
	}
	layout, rootFD, inputsFD, err := openVerifiedRunViewInputs(material)
	if err != nil {
		return unresolvedCraftRunView("open verified RunView output root", err)
	}
	defer unix.Close(inputsFD)
	defer unix.Close(rootFD)
	outputFD, err := unix.Openat(rootFD, "output", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return fmt.Errorf("%w: open RunView private output: %v", craft.ErrConflict, err)
	}
	defer unix.Close(outputFD)
	var outputStat unix.Stat_t
	var identity craftRunViewLayoutIdentity
	if json.Unmarshal(material.layoutID, &identity) != nil {
		return fmt.Errorf("%w: RunView layout identity is malformed", craft.ErrConflict)
	}
	if err := unix.Fstat(outputFD, &outputStat); err != nil || !runViewDirectoryStatMatches(identity, layout.output, outputStat) {
		return fmt.Errorf("%w: RunView output directory identity changed while opening", craft.ErrConflict)
	}
	present, err := inspectRunViewDraftOutput(outputFD, "", expected)
	if err != nil {
		return err
	}
	paths := make([]string, 0, len(expected))
	for rel := range expected {
		paths = append(paths, rel)
	}
	sort.Strings(paths)
	for _, rel := range paths {
		if err := ctx.Err(); err != nil {
			return err
		}
		if present[rel] {
			continue
		}
		name := path.Base(rel)
		dirFD, err := openOrCreateRunViewDraftParent(outputFD, path.Dir(rel))
		if err != nil {
			return err
		}
		file := expected[rel]
		content, readErr := readRunViewDraftObject(ctx, e.files, file)
		if readErr != nil {
			_ = unix.Close(dirFD)
			return readErr
		}
		_, writeErr := writeRunViewInputAtomic(dirFD, name, content)
		_ = unix.Close(dirFD)
		if writeErr != nil {
			return writeErr
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	present, err = inspectRunViewDraftOutput(outputFD, "", expected)
	if err != nil {
		return err
	}
	if len(present) != len(expected) {
		return fmt.Errorf("%w: RunView output seed is incomplete", craft.ErrConflict)
	}
	if err := material.provider.verifyMaterialHandle(material); err != nil {
		return unresolvedCraftRunView("RunView material changed during Workspace seed", err)
	}
	if _, err := e.admittedRunForCraftFence(ctx, task); err != nil {
		return err
	}
	return unix.Fsync(outputFD)
}

type admittedCraftRunFenceRow struct {
	Snapshot string
	Epoch    int64
}

func (e *localCraftRuntime) admittedRunForCraftFence(ctx context.Context, task craft.Task) (admittedCraftRunFenceRow, error) {
	if e.db == nil || task.Fence.TenantID == 0 || task.Fence.TenantID != task.Scope.TenantID ||
		task.Fence.RunID == "" || task.Scope.SessionID == "" || task.Scope.UserID == "" || task.Fence.Epoch < 1 {
		return admittedCraftRunFenceRow{}, fmt.Errorf("%w: incomplete durable Run writer fence", craft.ErrForbidden)
	}
	var row admittedCraftRunFenceRow
	if err := e.db.WithContext(ctx).Table("agent_runs").Select("snapshot, epoch").Where(
		"tenant_id = ? AND run_id = ? AND session_id = ? AND owner_id = ?",
		task.Fence.TenantID, task.Fence.RunID, task.Scope.SessionID, task.Scope.UserID,
	).Take(&row).Error; err != nil {
		return admittedCraftRunFenceRow{}, fmt.Errorf("craft RunView read admitted Run fence: %w", err)
	}
	if row.Epoch != task.Fence.Epoch {
		return admittedCraftRunFenceRow{}, fmt.Errorf("%w: admitted Run writer fence changed", craft.ErrConflict)
	}
	return row, nil
}

func readRunViewDraftObject(ctx context.Context, files interfaces.FileService, file craft.File) ([]byte, error) {
	if files == nil || file.Ref == "" || file.Bytes < 0 || file.Bytes > craftLocalMaxReadBytes {
		return nil, fmt.Errorf("%w: Workspace draft file exceeds the runtime read cap", craft.ErrInvalidInput)
	}
	reader, err := files.GetFile(ctx, file.Ref)
	if err != nil {
		return nil, fmt.Errorf("craft RunView read draft object %s: %w", file.Path, err)
	}
	if reader == nil {
		return nil, fmt.Errorf("craft RunView read draft object %s: FileService returned no reader", file.Path)
	}
	content, readErr := io.ReadAll(io.LimitReader(reader, file.Bytes+1))
	closeErr := reader.Close()
	if readErr != nil || closeErr != nil {
		return nil, errors.Join(readErr, closeErr)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if int64(len(content)) != file.Bytes {
		return nil, fmt.Errorf("%w: Workspace draft file %s declares %d bytes but stored %d", craft.ErrInvalidInput, file.Path, file.Bytes, len(content))
	}
	sum := sha256.Sum256(content)
	if hex.EncodeToString(sum[:]) != file.SHA256 {
		return nil, fmt.Errorf("%w: Workspace draft file %s digest mismatch", craft.ErrInvalidInput, file.Path)
	}
	return content, nil
}

func openOrCreateRunViewDraftParent(rootFD int, rel string) (int, error) {
	fd, err := unix.Dup(rootFD)
	if err != nil {
		return -1, err
	}
	if rel == "." {
		return fd, nil
	}
	for _, component := range strings.Split(rel, "/") {
		if component == "" || component == "." || component == ".." {
			_ = unix.Close(fd)
			return -1, craft.ErrInvalidInput
		}
		created := false
		if err := unix.Mkdirat(fd, component, 0755); err == nil {
			created = true
		} else if !errors.Is(err, unix.EEXIST) {
			_ = unix.Close(fd)
			return -1, err
		}
		next, err := unix.Openat(fd, component, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
		if err == nil && created {
			err = unix.Fsync(fd)
		}
		_ = unix.Close(fd)
		if err != nil {
			return -1, err
		}
		var stat unix.Stat_t
		if err := unix.Fstat(next, &stat); err != nil || stat.Mode&unix.S_IFMT != unix.S_IFDIR || stat.Mode&07777 != 0755 {
			_ = unix.Close(next)
			return -1, craft.ErrConflict
		}
		fd = next
	}
	return fd, nil
}

func inspectRunViewDraftOutput(fd int, prefix string, expected map[string]craft.File) (map[string]bool, error) {
	return inspectRunViewDraftOutputWithOps(fd, prefix, expected, runViewDraftOutputInspectOps{})
}

type runViewDraftOutputInspectOps struct {
	beforeFileOpen func(parentFD int, name string) error
	beforeDirOpen  func(parentFD int, name string) error
}

func inspectRunViewDraftOutputWithOps(fd int, prefix string, expected map[string]craft.File, ops runViewDraftOutputInspectOps) (map[string]bool, error) {
	present := map[string]bool{}
	entries, err := readRunViewDirEntries(fd)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		name := entry.Name()
		rel := name
		if prefix != "" {
			rel = prefix + "/" + name
		}
		if file, ok := expected[rel]; ok {
			var before unix.Stat_t
			if err := unix.Fstatat(fd, name, &before, unix.AT_SYMLINK_NOFOLLOW); err != nil {
				return nil, err
			}
			if !runViewDraftFileStatIsCanonical(before, file.Bytes) {
				return nil, fmt.Errorf("%w: seeded output file is unsafe", craft.ErrConflict)
			}
			if ops.beforeFileOpen != nil {
				if err := ops.beforeFileOpen(fd, name); err != nil {
					return nil, err
				}
			}
			fileFD, err := unix.Openat(fd, name, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
			if err != nil {
				return nil, err
			}
			f := os.NewFile(uintptr(fileFD), name)
			var opened unix.Stat_t
			if err := unix.Fstat(fileFD, &opened); err != nil || !runViewDraftFileStatsMatch(before, opened, file.Bytes) {
				_ = f.Close()
				return nil, fmt.Errorf("%w: seeded output file changed while opening", craft.ErrConflict)
			}
			raw, readErr := io.ReadAll(io.LimitReader(f, file.Bytes+1))
			var after unix.Stat_t
			statErr := unix.Fstat(fileFD, &after)
			closeErr := f.Close()
			sum := sha256.Sum256(raw)
			var pathAfter unix.Stat_t
			pathErr := unix.Fstatat(fd, name, &pathAfter, unix.AT_SYMLINK_NOFOLLOW)
			if readErr != nil || statErr != nil || closeErr != nil || pathErr != nil {
				return nil, errors.Join(readErr, statErr, closeErr, pathErr)
			}
			if !runViewDraftFileStatsMatch(opened, after, file.Bytes) || !runViewDraftFileStatsMatch(opened, pathAfter, file.Bytes) {
				return nil, fmt.Errorf("%w: seeded output file identity changed while reading", craft.ErrConflict)
			}
			if int64(len(raw)) != file.Bytes || hex.EncodeToString(sum[:]) != file.SHA256 {
				return nil, fmt.Errorf("%w: existing output differs from frozen Workspace predecessor", craft.ErrConflict)
			}
			present[rel] = true
			continue
		}
		needed := false
		for wanted := range expected {
			if strings.HasPrefix(wanted, rel+"/") {
				needed = true
				break
			}
		}
		if !needed {
			return nil, fmt.Errorf("%w: extra object in RunView output: %s", craft.ErrConflict, rel)
		}
		var before unix.Stat_t
		if err := unix.Fstatat(fd, name, &before, unix.AT_SYMLINK_NOFOLLOW); err != nil {
			return nil, err
		}
		if before.Mode&unix.S_IFMT != unix.S_IFDIR || before.Mode&07777 != 0755 {
			return nil, fmt.Errorf("%w: unsafe RunView output directory", craft.ErrConflict)
		}
		if ops.beforeDirOpen != nil {
			if err := ops.beforeDirOpen(fd, name); err != nil {
				return nil, err
			}
		}
		child, err := unix.Openat(fd, name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
		if err != nil {
			return nil, err
		}
		var opened unix.Stat_t
		if err := unix.Fstat(child, &opened); err != nil || !runViewDraftDirStatsMatch(before, opened) {
			_ = unix.Close(child)
			return nil, fmt.Errorf("%w: output directory changed while opening", craft.ErrConflict)
		}
		nested, nestedErr := inspectRunViewDraftOutputWithOps(child, rel, expected, ops)
		var after unix.Stat_t
		statErr := unix.Fstat(child, &after)
		closeErr := unix.Close(child)
		var pathAfter unix.Stat_t
		pathErr := unix.Fstatat(fd, name, &pathAfter, unix.AT_SYMLINK_NOFOLLOW)
		if nestedErr != nil || statErr != nil || closeErr != nil || pathErr != nil {
			return nil, errors.Join(nestedErr, statErr, closeErr, pathErr)
		}
		if !runViewDraftDirStatsMatch(opened, after) || !runViewDraftDirStatsMatch(opened, pathAfter) {
			return nil, fmt.Errorf("%w: output directory identity changed while reading", craft.ErrConflict)
		}
		for k := range nested {
			present[k] = true
		}
	}
	return present, nil
}

func runViewDraftFileStatIsCanonical(stat unix.Stat_t, size int64) bool {
	return stat.Mode&unix.S_IFMT == unix.S_IFREG && stat.Mode&07777 == 0644 && stat.Nlink == 1 && stat.Size == size
}

func runViewDraftFileStatsMatch(a, b unix.Stat_t, size int64) bool {
	return runViewDraftFileStatIsCanonical(a, size) && runViewDraftFileStatIsCanonical(b, size) && a.Dev == b.Dev && a.Ino == b.Ino
}

func runViewDraftDirStatsMatch(a, b unix.Stat_t) bool {
	return a.Mode&unix.S_IFMT == unix.S_IFDIR && b.Mode&unix.S_IFMT == unix.S_IFDIR &&
		a.Mode&07777 == 0755 && b.Mode&07777 == 0755 && a.Dev == b.Dev && a.Ino == b.Ino
}

func validateRunViewDraftOutputPaths(expected map[string]craft.File) error {
	paths := make([]string, 0, len(expected))
	for rel, file := range expected {
		if rel != file.Path {
			return fmt.Errorf("%w: Workspace predecessor path key mismatch", craft.ErrInvalidInput)
		}
		if err := craft.ValidateArtifactPath(rel); err != nil {
			return err
		}
		paths = append(paths, rel)
	}
	sort.Strings(paths)
	for i, rel := range paths {
		if i+1 < len(paths) && strings.HasPrefix(paths[i+1], rel+"/") {
			return fmt.Errorf("%w: Workspace predecessor file is also a directory", craft.ErrInvalidInput)
		}
	}
	return nil
}

// stageWorkspaceInputs materializes only the immutable input selection carried
// by the admitted delegation task into the verified generation's private
// inputs directory. It never stages into localCraftRuntime.workDir.
func (e *localCraftRuntime) stageWorkspaceInputs(
	ctx context.Context,
	task craft.Task,
	material CraftRunViewMaterialHandle,
) error {
	if material.generation == "" || material.key.TenantID != task.Fence.TenantID ||
		material.key.TenantID != task.Scope.TenantID || material.key.OwnerID != task.Scope.UserID ||
		material.key.SessionID != task.Scope.SessionID || material.key.RunID != task.Fence.RunID {
		return fmt.Errorf("%w: material handle is not bound to the admitted Run", craft.ErrForbidden)
	}
	inputs, err := e.selectedWorkspaceInputs(ctx, task)
	if err != nil {
		return err
	}
	if err := craft.ValidateInputManifest(inputs); err != nil {
		return err
	}
	layout, rootFD, inputsFD, err := openVerifiedRunViewInputs(material)
	if err != nil {
		return unresolvedCraftRunView("open verified RunView inputs directory", err)
	}
	defer unix.Close(inputsFD)
	defer unix.Close(rootFD)
	_ = layout

	expected, byDirectory, err := runViewInputManifest(inputs)
	if err != nil {
		return err
	}
	present, err := inspectRunViewInputTree(inputsFD, expected, byDirectory)
	if err != nil {
		return err
	}
	contents := make(map[string][]byte, len(expected))
	paths := make([]string, 0, len(expected))
	for rel := range expected {
		paths = append(paths, rel)
	}
	sort.Strings(paths)
	for _, rel := range paths {
		if err := ctx.Err(); err != nil {
			return err
		}
		input := expected[rel]
		if present[rel] {
			continue
		}
		content, err := readRunViewInputObject(ctx, e.files, input)
		if err != nil {
			return err
		}
		contents[rel] = content
	}

	for _, rel := range paths {
		content, missing := contents[rel]
		if !missing {
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		digestDirectory, fileName := path.Split(rel)
		digestDirectory = strings.TrimSuffix(digestDirectory, "/")
		dirFD, _, err := openOrCreateRunViewInputDigestDir(inputsFD, digestDirectory)
		if err != nil {
			return err
		}
		var writeErr error
		_, writeErr = writeRunViewInputAtomicWithHooks(dirFD, fileName, content, runViewInputPublishHooks{
			beforePublish: e.inputWriteBeforePublish,
			publish:       e.inputPublishNoReplace,
			afterPublish:  e.inputWriteAfterPublish,
		})
		_ = unix.Close(dirFD)
		if writeErr != nil {
			return writeErr
		}
	}
	if err := unix.Fsync(inputsFD); err != nil {
		return err
	}
	present, err = inspectRunViewInputTree(inputsFD, expected, byDirectory)
	if err != nil {
		return err
	}
	if len(present) != len(expected) {
		return fmt.Errorf("%w: staged RunView input tree is incomplete", craft.ErrConflict)
	}
	return nil
}

func runViewInputManifest(inputs []craft.Input) (map[string]craft.Input, map[string]map[string]craft.Input, error) {
	expected := make(map[string]craft.Input, len(inputs))
	byDirectory := make(map[string]map[string]craft.Input, len(inputs))
	for _, input := range inputs {
		rel, err := craft.InputPath(input.SHA256, input.Name)
		if err != nil {
			return nil, nil, err
		}
		rel = strings.TrimPrefix(rel, craft.InputDir+"/")
		directory, name, ok := strings.Cut(rel, "/")
		if !ok || directory == "" || name == "" {
			return nil, nil, fmt.Errorf("%w: invalid canonical RunView input path", craft.ErrInvalidInput)
		}
		if previous, exists := expected[rel]; exists {
			if previous.Ref != input.Ref || previous.SHA256 != input.SHA256 || previous.Bytes != input.Bytes {
				return nil, nil, fmt.Errorf("%w: duplicate RunView input path is ambiguous", craft.ErrConflict)
			}
			continue
		}
		expected[rel] = input
		if byDirectory[directory] == nil {
			byDirectory[directory] = make(map[string]craft.Input)
		}
		byDirectory[directory][name] = input
	}
	return expected, byDirectory, nil
}

func openVerifiedRunViewInputs(material CraftRunViewMaterialHandle) (craftRunViewHostLayout, int, int, error) {
	spec := craftRunViewSpecForGeneration(material.generation)
	layout := generationLayoutPaths(filepath.Dir(material.root), spec)
	if layout.root != material.root || layout.inputs != material.inputs ||
		layout.knowledge != material.knowledge || layout.output != material.output {
		return craftRunViewHostLayout{}, -1, -1, errors.New("material handle paths do not derive from its generation")
	}
	if err := verifyGenerationLayout(layout, spec); err != nil {
		return craftRunViewHostLayout{}, -1, -1, err
	}
	rootFD, err := unix.Open(layout.root, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return craftRunViewHostLayout{}, -1, -1, err
	}
	inputsFD, err := unix.Openat(rootFD, "inputs", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		_ = unix.Close(rootFD)
		return craftRunViewHostLayout{}, -1, -1, err
	}
	identityFileFD, err := unix.Openat(rootFD, craftRunViewLayoutIdentityFile, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		_ = unix.Close(inputsFD)
		_ = unix.Close(rootFD)
		return craftRunViewHostLayout{}, -1, -1, err
	}
	identityFile := os.NewFile(uintptr(identityFileFD), craftRunViewLayoutIdentityFile)
	info, err := identityFile.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 || info.Size() > 4096 {
		_ = identityFile.Close()
		_ = unix.Close(inputsFD)
		_ = unix.Close(rootFD)
		return craftRunViewHostLayout{}, -1, -1, errors.New("RunView layout identity is not a private regular file")
	}
	actualIdentity, readErr := io.ReadAll(io.LimitReader(identityFile, 4097))
	closeErr := identityFile.Close()
	if readErr != nil || closeErr != nil {
		_ = unix.Close(inputsFD)
		_ = unix.Close(rootFD)
		return craftRunViewHostLayout{}, -1, -1, errors.Join(readErr, closeErr)
	}
	expectedIdentity, err := layoutIdentityBytes(layout, spec)
	if err != nil || !bytes.Equal(actualIdentity, expectedIdentity) {
		_ = unix.Close(inputsFD)
		_ = unix.Close(rootFD)
		return craftRunViewHostLayout{}, -1, -1, errors.New("RunView layout identity changed while opening input directory")
	}
	var identity craftRunViewLayoutIdentity
	if err := json.Unmarshal(actualIdentity, &identity); err != nil || identity.Generation != material.generation {
		_ = unix.Close(inputsFD)
		_ = unix.Close(rootFD)
		return craftRunViewHostLayout{}, -1, -1, errors.New("RunView layout identity manifest is invalid")
	}
	var rootStat, inputsStat unix.Stat_t
	if err := unix.Fstat(rootFD, &rootStat); err != nil {
		_ = unix.Close(inputsFD)
		_ = unix.Close(rootFD)
		return craftRunViewHostLayout{}, -1, -1, err
	}
	if err := unix.Fstat(inputsFD, &inputsStat); err != nil {
		_ = unix.Close(inputsFD)
		_ = unix.Close(rootFD)
		return craftRunViewHostLayout{}, -1, -1, err
	}
	if !runViewDirectoryStatMatches(identity, layout.root, rootStat) ||
		!runViewDirectoryStatMatches(identity, layout.inputs, inputsStat) {
		_ = unix.Close(inputsFD)
		_ = unix.Close(rootFD)
		return craftRunViewHostLayout{}, -1, -1, errors.New("opened RunView input directory differs from durable identity")
	}
	return layout, rootFD, inputsFD, nil
}

func runViewDirectoryStatMatches(identity craftRunViewLayoutIdentity, path string, stat unix.Stat_t) bool {
	for _, dir := range identity.Directories {
		if dir.Path == path {
			return dir.Device == uint64(stat.Dev) && dir.Inode == uint64(stat.Ino)
		}
	}
	return false
}

func inspectRunViewInputTree(
	inputsFD int,
	expected map[string]craft.Input,
	byDirectory map[string]map[string]craft.Input,
) (map[string]bool, error) {
	present := make(map[string]bool, len(expected))
	entries, err := readRunViewDirEntries(inputsFD)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		directory := entry.Name()
		files, allowed := byDirectory[directory]
		if !allowed {
			return nil, fmt.Errorf("%w: extra object in RunView inputs: %s", craft.ErrConflict, directory)
		}
		var stat unix.Stat_t
		if err := unix.Fstatat(inputsFD, directory, &stat, unix.AT_SYMLINK_NOFOLLOW); err != nil {
			return nil, err
		}
		if stat.Mode&unix.S_IFMT != unix.S_IFDIR || stat.Mode&0777 != 0755 {
			return nil, fmt.Errorf("%w: RunView input digest parent is not a canonical directory", craft.ErrConflict)
		}
		dirFD, err := unix.Openat(inputsFD, directory, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
		if err != nil {
			return nil, fmt.Errorf("%w: open RunView input digest parent: %v", craft.ErrConflict, err)
		}
		fileEntries, readErr := readRunViewDirEntries(dirFD)
		if readErr != nil {
			_ = unix.Close(dirFD)
			return nil, readErr
		}
		for _, fileEntry := range fileEntries {
			name := fileEntry.Name()
			input, ok := files[name]
			if !ok {
				_ = unix.Close(dirFD)
				return nil, fmt.Errorf("%w: extra object in RunView inputs: %s/%s", craft.ErrConflict, directory, name)
			}
			var targetStat unix.Stat_t
			if err := unix.Fstatat(dirFD, name, &targetStat, unix.AT_SYMLINK_NOFOLLOW); err != nil {
				_ = unix.Close(dirFD)
				return nil, err
			}
			if targetStat.Mode&unix.S_IFMT != unix.S_IFREG || targetStat.Mode&07777 != 0644 || targetStat.Nlink != 1 || targetStat.Size != input.Bytes {
				_ = unix.Close(dirFD)
				return nil, fmt.Errorf("%w: existing RunView input is not the admitted regular file", craft.ErrConflict)
			}
			fileFD, err := unix.Openat(dirFD, name, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
			if err != nil {
				_ = unix.Close(dirFD)
				return nil, fmt.Errorf("%w: open existing RunView input: %v", craft.ErrConflict, err)
			}
			var openedStat unix.Stat_t
			if err := unix.Fstat(fileFD, &openedStat); err != nil || openedStat.Dev != targetStat.Dev || openedStat.Ino != targetStat.Ino || openedStat.Mode&07777 != 0644 {
				_ = unix.Close(fileFD)
				_ = unix.Close(dirFD)
				return nil, fmt.Errorf("%w: RunView input changed while opening", craft.ErrConflict)
			}
			file := os.NewFile(uintptr(fileFD), name)
			content, readErr := io.ReadAll(io.LimitReader(file, craftLocalStagedReadBudget))
			closeErr := file.Close()
			if readErr != nil || closeErr != nil {
				_ = unix.Close(dirFD)
				return nil, errors.Join(readErr, closeErr)
			}
			sum := sha256.Sum256(content)
			if int64(len(content)) != input.Bytes || hex.EncodeToString(sum[:]) != input.SHA256 {
				_ = unix.Close(dirFD)
				return nil, fmt.Errorf("%w: existing RunView input bytes differ from admitted digest", craft.ErrConflict)
			}
			present[directory+"/"+name] = true
		}
		_ = unix.Close(dirFD)
	}
	return present, nil
}

func readRunViewDirEntries(fd int) ([]os.DirEntry, error) {
	dirFD, err := unix.Openat(fd, ".", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(dirFD), "runview-input-dir")
	entries, readErr := file.ReadDir(-1)
	closeErr := file.Close()
	return entries, errors.Join(readErr, closeErr)
}

func readRunViewInputObject(ctx context.Context, files interfaces.FileService, input craft.Input) ([]byte, error) {
	if files == nil {
		return nil, fmt.Errorf("craft RunView runtime: FileService is unavailable")
	}
	reader, err := files.GetFile(ctx, input.Ref)
	if err != nil {
		return nil, fmt.Errorf("craft RunView runtime read input %s: %w", input.Name, err)
	}
	if reader == nil {
		return nil, fmt.Errorf("craft RunView runtime read input %s: FileService returned no reader", input.Name)
	}
	content, readErr := io.ReadAll(io.LimitReader(reader, craftLocalStagedReadBudget))
	closeErr := reader.Close()
	if readErr != nil || closeErr != nil {
		return nil, errors.Join(readErr, closeErr)
	}
	if int64(len(content)) != input.Bytes {
		return nil, fmt.Errorf("%w: RunView input %s declares %d bytes but stored %d", craft.ErrInvalidInput, input.Name, input.Bytes, len(content))
	}
	sum := sha256.Sum256(content)
	if hex.EncodeToString(sum[:]) != input.SHA256 {
		return nil, fmt.Errorf("%w: RunView input %s digest mismatch", craft.ErrInvalidInput, input.Name)
	}
	return content, nil
}

func openOrCreateRunViewInputDigestDir(inputsFD int, directory string) (int, bool, error) {
	return openOrCreateRunViewInputDigestDirWithOps(inputsFD, directory, runViewInputDigestDirOps{})
}

type runViewInputDigestDirOps struct {
	openat func(int, string, int, uint32) (int, error)
	fchmod func(int, uint32) error
	fsync  func(int) error
	fstat  func(int, *unix.Stat_t) error
}

func openOrCreateRunViewInputDigestDirWithOps(inputsFD int, directory string, ops runViewInputDigestDirOps) (int, bool, error) {
	created := false
	if err := unix.Mkdirat(inputsFD, directory, 0755); err == nil {
		created = true
	} else if !errors.Is(err, unix.EEXIST) {
		return -1, false, err
	}
	openat := ops.openat
	if openat == nil {
		openat = unix.Openat
	}
	fd, err := openat(inputsFD, directory, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return -1, created, err
	}
	if created {
		fchmod := ops.fchmod
		if fchmod == nil {
			fchmod = unix.Fchmod
		}
		if err := fchmod(fd, 0755); err != nil {
			return -1, created, errors.Join(err, unix.Close(fd))
		}
		fsync := ops.fsync
		if fsync == nil {
			fsync = unix.Fsync
		}
		if err := fsync(inputsFD); err != nil {
			return -1, created, errors.Join(err, unix.Close(fd))
		}
	}
	fstat := ops.fstat
	if fstat == nil {
		fstat = unix.Fstat
	}
	var stat unix.Stat_t
	if err := fstat(fd, &stat); err != nil {
		return -1, created, errors.Join(err, unix.Close(fd))
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFDIR || stat.Mode&07777 != 0755 {
		return -1, created, errors.Join(fmt.Errorf("%w: RunView input digest parent changed during staging", craft.ErrConflict), unix.Close(fd))
	}
	return fd, created, nil
}

func writeRunViewInputAtomic(dirFD int, name string, content []byte) (bool, error) {
	return writeRunViewInputAtomicWithHooks(dirFD, name, content, runViewInputPublishHooks{})
}

func writeRunViewInputAtomicWithBeforePublish(dirFD int, name string, content []byte, beforePublish func(int, string) error) (bool, error) {
	var before func(int, string, string) error
	if beforePublish != nil {
		before = func(fd int, _ string, target string) error { return beforePublish(fd, target) }
	}
	return writeRunViewInputAtomicWithHooks(dirFD, name, content, runViewInputPublishHooks{beforePublish: before})
}

type runViewInputPublishHooks struct {
	beforePublish func(int, string, string) error
	publish       func(int, string, string) error
	afterPublish  func(int, string, string) error
}

func writeRunViewInputAtomicWithHooks(dirFD int, name string, content []byte, hooks runViewInputPublishHooks) (bool, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return false, err
	}
	tempName := ".craft-input-" + hex.EncodeToString(random[:])
	fd, err := unix.Openat(dirFD, tempName, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0600)
	if err != nil {
		return false, err
	}
	file := os.NewFile(uintptr(fd), tempName)
	if _, err := file.Write(content); err != nil {
		return false, errors.Join(fmt.Errorf("write temporary RunView input: %w", err), file.Close())
	}
	if err := file.Chmod(0644); err != nil {
		return false, errors.Join(fmt.Errorf("chmod temporary RunView input: %w", err), file.Close())
	}
	if err := file.Sync(); err != nil {
		return false, errors.Join(fmt.Errorf("sync temporary RunView input: %w", err), file.Close())
	}
	var tempStat unix.Stat_t
	if err := unix.Fstat(fd, &tempStat); err != nil {
		return false, errors.Join(fmt.Errorf("stat temporary RunView input: %w", err), file.Close())
	}
	if tempStat.Mode&unix.S_IFMT != unix.S_IFREG || tempStat.Mode&07777 != 0644 || tempStat.Nlink != 1 || tempStat.Size != int64(len(content)) {
		return false, errors.Join(fmt.Errorf("%w: temporary RunView input changed before publication", craft.ErrConflict), file.Close())
	}
	if hooks.beforePublish != nil {
		if err := hooks.beforePublish(dirFD, tempName, name); err != nil {
			return false, errors.Join(err, file.Close())
		}
	}
	publish := hooks.publish
	if publish == nil {
		publish = renameRunViewInputNoReplace
	}
	if err := publish(dirFD, tempName, name); err != nil {
		wrapped := fmt.Errorf("publish RunView input without replacement: %w", err)
		if errors.Is(err, unix.EEXIST) {
			wrapped = errors.Join(fmt.Errorf("%w: RunView input target appeared during staging", craft.ErrConflict), wrapped)
		}
		return false, errors.Join(wrapped, file.Close())
	}
	published := true
	var targetStat unix.Stat_t
	if err := unix.Fstatat(dirFD, name, &targetStat, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return published, errors.Join(fmt.Errorf("verify published RunView input: %w", err), file.Close())
	}
	if targetStat.Dev != tempStat.Dev || targetStat.Ino != tempStat.Ino || targetStat.Mode&unix.S_IFMT != unix.S_IFREG ||
		targetStat.Mode&07777 != 0644 || targetStat.Nlink != 1 || targetStat.Size != int64(len(content)) {
		return published, errors.Join(fmt.Errorf("%w: published RunView input differs from open source identity", craft.ErrConflict), file.Close())
	}
	if hooks.afterPublish != nil {
		if err := hooks.afterPublish(dirFD, tempName, name); err != nil {
			return published, errors.Join(err, file.Close())
		}
	}
	if err := unix.Fsync(dirFD); err != nil {
		return published, errors.Join(fmt.Errorf("sync published RunView input directory: %w", err), file.Close())
	}
	if err := file.Close(); err != nil {
		return published, fmt.Errorf("close published RunView input: %w", err)
	}
	return published, nil
}

// selectedWorkspaceInputs materializes only the typed manifest in the
// immutable admitted Run snapshot, then verifies each ref against the scoped
// workspace association before reading bytes. Delegate arguments may narrow
// that set, but cannot authorize additional material.
func (e *localCraftRuntime) selectedWorkspaceInputs(ctx context.Context, task craft.Task) ([]craft.Input, error) {
	if e.db == nil {
		return nil, fmt.Errorf("craft local runtime: cannot read the admitted Run snapshot")
	}
	if task.Fence.TenantID == 0 || task.Fence.TenantID != task.Scope.TenantID {
		return nil, fmt.Errorf("%w: Run and delegation tenant scopes differ", craft.ErrForbidden)
	}
	inputs, err := e.admittedRunInputManifest(ctx, task)
	if err != nil {
		return nil, err
	}
	if err := craft.ValidateInputManifest(inputs); err != nil {
		return nil, err
	}
	if err := craft.ValidateInputManifest(task.Inputs); err != nil {
		return nil, err
	}
	admittedByRef := make(map[string]craft.Input, len(inputs))
	for _, in := range inputs {
		admittedByRef[in.Ref] = in
	}
	for _, delegated := range task.Inputs {
		admitted, ok := admittedByRef[delegated.Ref]
		if !ok || admitted.Name != delegated.Name || admitted.SHA256 != delegated.SHA256 || admitted.Bytes != delegated.Bytes {
			return nil, fmt.Errorf("%w: delegated input %q is outside the admitted Run snapshot", craft.ErrForbidden, delegated.Name)
		}
	}
	var workspaceCount int64
	if err := e.db.WithContext(ctx).Table("craft_workspaces").Where(
		"id = ? AND tenant_id = ? AND owner_id = ? AND session_id = ?",
		task.WorkspaceID, task.Scope.TenantID, task.Scope.UserID, task.Scope.SessionID,
	).Count(&workspaceCount).Error; err != nil {
		return nil, fmt.Errorf("craft local runtime verify workspace scope: %w", err)
	}
	if workspaceCount != 1 {
		return nil, fmt.Errorf("%w: workspace %s is not bound to the delegation scope", craft.ErrForbidden, task.WorkspaceID)
	}
	if len(inputs) == 0 {
		return nil, nil
	}
	var rows []struct {
		Ref    string
		Name   string
		SHA256 string
		Bytes  int64
	}
	refs := make([]string, 0, len(inputs))
	for _, in := range inputs {
		refs = append(refs, in.Ref)
	}
	if err := e.db.WithContext(ctx).Table("craft_workspace_inputs").
		Select("ref, name, sha256, bytes").
		Where("workspace_id = ? AND tenant_id = ? AND ref IN ?", task.WorkspaceID, task.Scope.TenantID, refs).
		Order("created_at, ref").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("craft local runtime verify selected workspace inputs: %w", err)
	}
	byRef := make(map[string]craft.Input, len(rows))
	for _, row := range rows {
		byRef[row.Ref] = craft.Input{Ref: row.Ref, Name: row.Name, SHA256: row.SHA256, Bytes: row.Bytes}
	}
	verified := make([]craft.Input, 0, len(inputs))
	for _, selected := range inputs {
		stored, ok := byRef[selected.Ref]
		if !ok || stored.Name != selected.Name || stored.SHA256 != selected.SHA256 || stored.Bytes != selected.Bytes {
			return nil, fmt.Errorf("%w: selected input %q is missing or changed in workspace %s",
				craft.ErrConflict, selected.Name, task.WorkspaceID)
		}
		verified = append(verified, selected)
	}
	if err := craft.ValidateInputManifest(verified); err != nil {
		return nil, err
	}
	return verified, nil
}

// admittedRunInputManifest strictly loads the typed input selection frozen at
// Run admission. A missing optional field means this is an old/non-Craft Run;
// the local Craft runtime fails closed rather than inferring selection from
// accumulated workspace rows or model-visible Query text.
func (e *localCraftRuntime) admittedRunInputManifest(ctx context.Context, task craft.Task) ([]craft.Input, error) {
	var row struct {
		Snapshot string
	}
	if err := e.db.WithContext(ctx).Table("agent_runs").Select("snapshot").Where(
		"tenant_id = ? AND run_id = ? AND session_id = ? AND owner_id = ?",
		task.Fence.TenantID, task.Fence.RunID, task.Scope.SessionID, task.Scope.UserID,
	).Take(&row).Error; err != nil {
		return nil, fmt.Errorf("craft local runtime read admitted Run snapshot: %w", err)
	}
	snapshot, err := service.ParseDurableRunSnapshot(json.RawMessage(row.Snapshot))
	if err != nil {
		return nil, fmt.Errorf("craft local runtime decode admitted Run snapshot: %w", err)
	}
	if snapshot.CraftInputManifest == nil {
		return nil, fmt.Errorf("%w: admitted Run snapshot has no typed Craft input manifest", craft.ErrConflict)
	}
	return append([]craft.Input{}, (*snapshot.CraftInputManifest)...), nil
}

func pathCleanForward(rel string) string {
	rel = strings.ReplaceAll(rel, "\\", "/")
	parts := strings.Split(rel, "/")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			continue
		}
		out = append(out, part)
	}
	return strings.Join(out, "/")
}

// runBoundCraftArtifactSource adapts one admitted RunView generation's
// private output directory to the collector interface. It carries the exact
// task and opaque material handle for its lifetime; the collector's session
// argument never selects a filesystem root.
type runBoundCraftArtifactSource struct {
	runtime   *localCraftRuntime
	task      craft.Task
	material  CraftRunViewMaterialHandle
	outputDir string

	mu     sync.Mutex
	listed map[string]runBoundArtifactIdentity
	// afterDirectoryRead is an inert-by-default deterministic filesystem race
	// seam. Production construction leaves it nil; tests use it to mutate a
	// directory exactly between bounded enumeration and its stability checks.
	afterDirectoryRead func(relative string)
}

type runBoundArtifactIdentity struct {
	device uint64
	inode  uint64
	mode   uint32
	size   int64
	digest string
	dir    bool
	nlink  uint64
	mtime  int64
	ctime  int64
}

func newRunBoundCraftArtifactSource(ctx context.Context, runtime *localCraftRuntime, task craft.Task, material CraftRunViewMaterialHandle, outputDir string) (*runBoundCraftArtifactSource, error) {
	if outputDir != craftLocalOutputDir {
		return nil, fmt.Errorf("%w: RunView artifact output path is fixed by the verified generation", craft.ErrInvalidInput)
	}
	source := &runBoundCraftArtifactSource{
		runtime: runtime, task: task, material: material,
		outputDir: outputDir, listed: make(map[string]runBoundArtifactIdentity),
	}
	if err := source.verifyBinding(ctx); err != nil {
		return nil, err
	}
	return source, nil
}

func (s *runBoundCraftArtifactSource) verifyBinding(ctx context.Context) error {
	if s == nil || s.runtime == nil || s.runtime.db == nil || s.material.provider == nil ||
		s.task.WorkspaceID == "" || s.task.Fence.TenantID == 0 || s.task.Fence.TenantID != s.task.Scope.TenantID ||
		s.task.Fence.RunID == "" || s.task.Scope.SessionID == "" || s.task.Scope.UserID == "" ||
		s.material.key.TenantID != s.task.Fence.TenantID || s.material.key.OwnerID != s.task.Scope.UserID ||
		s.material.key.SessionID != s.task.Scope.SessionID || s.material.key.RunID != s.task.Fence.RunID {
		return fmt.Errorf("%w: artifact source is not bound to the admitted RunView", craft.ErrForbidden)
	}
	row, err := s.runtime.admittedRunForCraftFence(ctx, s.task)
	if err != nil {
		return err
	}
	snapshot, err := service.ParseDurableRunSnapshot(json.RawMessage(row.Snapshot))
	if err != nil || snapshot.CraftWorkspaceSeed == nil || snapshot.CraftWorkspaceSeed.WorkspaceID != s.task.WorkspaceID {
		return fmt.Errorf("%w: artifact source Run snapshot differs from its Workspace", craft.ErrConflict)
	}
	if err := s.material.provider.RevalidateMaterialHandle(ctx, s.material); err != nil {
		return unresolvedCraftRunView("Run-bound artifact material changed", err)
	}
	return ctx.Err()
}

func (s *runBoundCraftArtifactSource) openOutputRoot(ctx context.Context) (int, error) {
	if err := s.verifyBinding(ctx); err != nil {
		return -1, err
	}
	layout, rootFD, inputsFD, err := openVerifiedRunViewInputs(s.material)
	if err != nil {
		return -1, unresolvedCraftRunView("open verified Run-bound artifact output", err)
	}
	_ = unix.Close(inputsFD)
	outputFD, err := unix.Openat(rootFD, "output", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		_ = unix.Close(rootFD)
		return -1, fmt.Errorf("%w: open private RunView output: %v", craft.ErrConflict, err)
	}
	var identity craftRunViewLayoutIdentity
	if json.Unmarshal(s.material.layoutID, &identity) != nil {
		_ = unix.Close(outputFD)
		_ = unix.Close(rootFD)
		return -1, fmt.Errorf("%w: malformed RunView layout identity", craft.ErrConflict)
	}
	var outputStat unix.Stat_t
	if err := unix.Fstat(outputFD, &outputStat); err != nil || !runViewDirectoryStatMatches(identity, layout.output, outputStat) {
		_ = unix.Close(outputFD)
		_ = unix.Close(rootFD)
		return -1, fmt.Errorf("%w: opened output differs from verified RunView generation", craft.ErrConflict)
	}
	_ = unix.Close(rootFD)
	return outputFD, nil
}

func (s *runBoundCraftArtifactSource) ListSessionFiles(ctx context.Context, sessionID, dir string) ([]sandbox.RemoteDirEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.listed = nil
	if sessionID != s.task.Scope.SessionID || dir != s.outputDir || dir != craftLocalOutputDir {
		return nil, fmt.Errorf("%w: artifact list request differs from the bound RunView", craft.ErrForbidden)
	}
	outputFD, err := s.openOutputRoot(ctx)
	if err != nil {
		return nil, err
	}
	defer unix.Close(outputFD)
	identities := make(map[string]runBoundArtifactIdentity)
	entries := make([]sandbox.RemoteDirEntry, 0)
	var total int64
	var outputStat unix.Stat_t
	if err := unix.Fstat(outputFD, &outputStat); err != nil {
		return nil, err
	}
	identities[""] = runBoundArtifactIdentityForDirectory(outputStat)
	entryCount := 0
	if err := walkRunBoundArtifactTree(ctx, outputFD, "", dir, identities, &entries, &total, &entryCount, s.afterDirectoryRead); err != nil {
		return nil, err
	}
	if err := s.verifyBinding(ctx); err != nil {
		return nil, err
	}
	s.listed = identities
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return entries, nil
}

func walkRunBoundArtifactTree(ctx context.Context, dirFD int, prefix, outputDir string, identities map[string]runBoundArtifactIdentity, entries *[]sandbox.RemoteDirEntry, total *int64, entryCount *int, afterDirectoryRead func(string)) error {
	var beforeDir unix.Stat_t
	if err := unix.Fstat(dirFD, &beforeDir); err != nil {
		return err
	}
	dirIdentity := runBoundArtifactIdentityForDirectory(beforeDir)
	if listed, ok := identities[prefix]; !ok || !runBoundDirIdentityMatches(listed, beforeDir) {
		return fmt.Errorf("%w: RunView output directory identity changed before enumeration", craft.ErrConflict)
	}
	remaining := runBoundArtifactMaxEntries - *entryCount
	limit := runBoundArtifactMaxEntriesPerDirectory
	if remaining < limit {
		limit = remaining
	}
	children, overflow, err := readRunBoundArtifactDirEntries(dirFD, limit)
	if err != nil {
		return err
	}
	if overflow {
		return fmt.Errorf("%w: RunView output entry count exceeds limit", craft.ErrInvalidInput)
	}
	*entryCount += len(children)
	if afterDirectoryRead != nil {
		afterDirectoryRead(prefix)
	}
	initialNames := runBoundArtifactEntryNames(children)
	for _, child := range children {
		if err := ctx.Err(); err != nil {
			return err
		}
		name := child.Name()
		if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\\x00") {
			return fmt.Errorf("%w: invalid RunView output entry name", craft.ErrInvalidInput)
		}
		rel := name
		if prefix != "" {
			rel = prefix + "/" + name
		}
		if err := craft.ValidateArtifactPath(rel); err != nil {
			return err
		}
		var before unix.Stat_t
		if err := unix.Fstatat(dirFD, name, &before, unix.AT_SYMLINK_NOFOLLOW); err != nil {
			return err
		}
		identity := runBoundArtifactIdentity{device: uint64(before.Dev), inode: before.Ino, mode: uint32(before.Mode),
			nlink: uint64(before.Nlink)}
		switch before.Mode & unix.S_IFMT {
		case unix.S_IFDIR:
			identity = runBoundArtifactIdentityForDirectory(before)
			if before.Mode&07777 != 0755 {
				return fmt.Errorf("%w: unsafe RunView output directory", craft.ErrConflict)
			}
			openedFD, err := unix.Openat(dirFD, name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
			if err != nil {
				return fmt.Errorf("%w: open RunView output directory: %v", craft.ErrConflict, err)
			}
			var opened unix.Stat_t
			if err := unix.Fstat(openedFD, &opened); err != nil || !runViewDraftDirStatsMatch(before, opened) {
				_ = unix.Close(openedFD)
				return fmt.Errorf("%w: RunView output directory changed while opening", craft.ErrConflict)
			}
			identities[rel] = identity
			entryPath := outputDir + "/" + rel
			*entries = append(*entries, sandbox.RemoteDirEntry{Name: name, Path: entryPath, Type: sandbox.RemoteEntryDir})
			nestedErr := walkRunBoundArtifactTree(ctx, openedFD, rel, outputDir, identities, entries, total, entryCount, afterDirectoryRead)
			var after unix.Stat_t
			statErr := unix.Fstat(openedFD, &after)
			var pathAfter unix.Stat_t
			pathErr := unix.Fstatat(dirFD, name, &pathAfter, unix.AT_SYMLINK_NOFOLLOW)
			closeErr := unix.Close(openedFD)
			if nestedErr != nil || statErr != nil || pathErr != nil || closeErr != nil {
				return errors.Join(nestedErr, statErr, pathErr, closeErr)
			}
			if !runViewDraftDirStatsMatch(opened, after) || !runViewDraftDirStatsMatch(opened, pathAfter) ||
				!runBoundDirIdentityMatches(identities[rel], after) || !runBoundDirIdentityMatches(identities[rel], pathAfter) {
				return fmt.Errorf("%w: RunView output directory changed during list", craft.ErrConflict)
			}
		case unix.S_IFREG:
			if !runViewDraftFileStatIsCanonical(before, before.Size) || before.Size > craftLocalMaxReadBytes || *total > craft.MaxDraftHeadBytes-before.Size {
				return fmt.Errorf("%w: unsafe or oversized RunView output file", craft.ErrInvalidInput)
			}
			fileFD, err := unix.Openat(dirFD, name, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
			if err != nil {
				return fmt.Errorf("%w: open RunView output file: %v", craft.ErrConflict, err)
			}
			var opened unix.Stat_t
			if err := unix.Fstat(fileFD, &opened); err != nil || !runViewDraftFileStatsMatch(before, opened, before.Size) {
				_ = unix.Close(fileFD)
				return fmt.Errorf("%w: RunView output file changed while opening", craft.ErrConflict)
			}
			file := os.NewFile(uintptr(fileFD), name)
			listedBytes, readErr := io.ReadAll(io.LimitReader(file, before.Size+1))
			var readStat unix.Stat_t
			statErr := unix.Fstat(fileFD, &readStat)
			closeErr := file.Close()
			if readErr != nil || statErr != nil || closeErr != nil {
				return errors.Join(readErr, statErr, closeErr)
			}
			if int64(len(listedBytes)) != before.Size || !runViewDraftFileStatsMatch(opened, readStat, before.Size) {
				return fmt.Errorf("%w: RunView output file changed while listing", craft.ErrConflict)
			}
			var pathAfter unix.Stat_t
			pathErr := unix.Fstatat(dirFD, name, &pathAfter, unix.AT_SYMLINK_NOFOLLOW)
			if pathErr != nil {
				return pathErr
			}
			if !runViewDraftFileStatsMatch(opened, pathAfter, before.Size) {
				return fmt.Errorf("%w: RunView output file changed during list", craft.ErrConflict)
			}
			identity.size = before.Size
			sum := sha256.Sum256(listedBytes)
			identity.digest = hex.EncodeToString(sum[:])
			identities[rel] = identity
			*total += before.Size
			*entries = append(*entries, sandbox.RemoteDirEntry{Name: name, Path: outputDir + "/" + rel,
				Type: sandbox.RemoteEntryFile, Size: before.Size})
		default:
			return fmt.Errorf("%w: RunView output contains a symlink or non-regular entry", craft.ErrInvalidInput)
		}
	}
	finalChildren, overflow, err := readRunBoundArtifactDirEntries(dirFD, limit)
	if err != nil {
		return err
	}
	var afterDir unix.Stat_t
	if err := unix.Fstat(dirFD, &afterDir); err != nil {
		return err
	}
	if overflow || !runBoundSameNames(initialNames, runBoundArtifactEntryNames(finalChildren)) ||
		!runBoundDirIdentityMatches(dirIdentity, afterDir) {
		return fmt.Errorf("%w: RunView output directory entry set changed during list", craft.ErrConflict)
	}
	return nil
}

func readRunBoundArtifactDirEntries(fd, limit int) ([]os.DirEntry, bool, error) {
	if limit < 0 {
		limit = 0
	}
	dirFD, err := unix.Openat(fd, ".", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, false, err
	}
	file := os.NewFile(uintptr(dirFD), "runview-output-dir")
	children, readErr := file.ReadDir(limit + 1)
	closeErr := file.Close()
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return nil, false, errors.Join(readErr, closeErr)
	}
	if closeErr != nil {
		return nil, false, closeErr
	}
	return children, len(children) > limit, nil
}

func runBoundArtifactEntryNames(entries []os.DirEntry) []string {
	names := make([]string, len(entries))
	for i := range entries {
		names[i] = entries[i].Name()
	}
	sort.Strings(names)
	return names
}

func runBoundSameNames(a, b []string) bool {
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

func runBoundArtifactIdentityForDirectory(stat unix.Stat_t) runBoundArtifactIdentity {
	return runBoundArtifactIdentity{device: uint64(stat.Dev), inode: stat.Ino, mode: uint32(stat.Mode), dir: true,
		size: stat.Size, nlink: uint64(stat.Nlink), mtime: runBoundStatTimestamp(stat, "Mtim", "Mtimespec"),
		ctime: runBoundStatTimestamp(stat, "Ctim", "Ctimespec")}
}

// unix.Stat_t uses Mtim/Ctim on Linux and Mtimespec/Ctimespec on Darwin. Read
// those fields reflectively so the fence remains buildable on both supported
// developer and production platforms without reducing timestamp precision.
func runBoundStatTimestamp(stat unix.Stat_t, names ...string) int64 {
	value := reflect.ValueOf(stat)
	for _, name := range names {
		field := value.FieldByName(name)
		if !field.IsValid() || field.Kind() != reflect.Struct {
			continue
		}
		seconds := field.FieldByName("Sec")
		nanos := field.FieldByName("Nsec")
		if seconds.IsValid() && nanos.IsValid() && seconds.CanInt() && nanos.CanInt() {
			return seconds.Int()*int64(time.Second) + nanos.Int()
		}
	}
	return 0
}

func (s *runBoundCraftArtifactSource) ReadSessionFile(ctx context.Context, sessionID, listedPath string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sessionID != s.task.Scope.SessionID {
		return nil, fmt.Errorf("%w: artifact read session differs from the bound RunView", craft.ErrForbidden)
	}
	rel, err := craft.ArtifactRelativePath(s.outputDir, listedPath)
	if err != nil {
		return nil, err
	}
	expected, ok := s.listed[rel]
	if !ok || expected.dir || expected.size < 0 || expected.size > craftLocalMaxReadBytes {
		return nil, fmt.Errorf("%w: artifact was not listed from the bound RunView", craft.ErrConflict)
	}
	outputFD, err := s.openOutputRoot(ctx)
	if err != nil {
		return nil, err
	}
	defer unix.Close(outputFD)
	content, err := readRunBoundArtifactAt(outputFD, rel, expected, s.listed)
	if err != nil {
		return nil, err
	}
	if err := s.verifyBinding(ctx); err != nil {
		return nil, err
	}
	return content, nil
}

func readRunBoundArtifactAt(rootFD int, rel string, expected runBoundArtifactIdentity, listed map[string]runBoundArtifactIdentity) ([]byte, error) {
	components := strings.Split(rel, "/")
	parentFD, err := unix.Dup(rootFD)
	if err != nil {
		return nil, err
	}
	rootDup := parentFD
	type openedDir struct {
		parent int
		fd     int
		name   string
		prefix string
		id     runBoundArtifactIdentity
		stat   unix.Stat_t
	}
	dirs := make([]openedDir, 0, len(components)-1)
	defer func() {
		for i := len(dirs) - 1; i >= 0; i-- {
			_ = unix.Close(dirs[i].fd)
		}
		_ = unix.Close(rootDup)
	}()
	rootID, ok := listed[""]
	var rootStat unix.Stat_t
	if !ok || !rootID.dir || unix.Fstat(rootDup, &rootStat) != nil || !runBoundDirIdentityMatches(rootID, rootStat) {
		return nil, fmt.Errorf("%w: RunView output root changed after list", craft.ErrConflict)
	}
	if err := verifyRunBoundArtifactDirectoryEntries(rootDup, "", listed); err != nil {
		return nil, err
	}
	for i, component := range components[:len(components)-1] {
		prefix := strings.Join(components[:i+1], "/")
		id, ok := listed[prefix]
		if !ok || !id.dir {
			return nil, fmt.Errorf("%w: listed output directory identity is missing", craft.ErrConflict)
		}
		var before unix.Stat_t
		if err := unix.Fstatat(parentFD, component, &before, unix.AT_SYMLINK_NOFOLLOW); err != nil || !runBoundDirIdentityMatches(id, before) {
			return nil, fmt.Errorf("%w: RunView output directory changed after list", craft.ErrConflict)
		}
		fd, err := unix.Openat(parentFD, component, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
		if err != nil {
			return nil, fmt.Errorf("%w: open listed RunView directory: %v", craft.ErrConflict, err)
		}
		var opened unix.Stat_t
		if err := unix.Fstat(fd, &opened); err != nil || !runViewDraftDirStatsMatch(before, opened) || !runBoundDirIdentityMatches(id, opened) {
			_ = unix.Close(fd)
			return nil, fmt.Errorf("%w: RunView output directory changed while opening", craft.ErrConflict)
		}
		dirs = append(dirs, openedDir{parent: parentFD, fd: fd, name: component, prefix: prefix, id: id, stat: opened})
		parentFD = fd
	}
	name := components[len(components)-1]
	var before unix.Stat_t
	if err := unix.Fstatat(parentFD, name, &before, unix.AT_SYMLINK_NOFOLLOW); err != nil || !runBoundFileIdentityMatches(expected, before) {
		return nil, fmt.Errorf("%w: RunView output file changed after list", craft.ErrConflict)
	}
	fd, err := unix.Openat(parentFD, name, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, fmt.Errorf("%w: open listed RunView output file: %v", craft.ErrConflict, err)
	}
	file := os.NewFile(uintptr(fd), name)
	var opened unix.Stat_t
	if err := unix.Fstat(fd, &opened); err != nil || !runViewDraftFileStatsMatch(before, opened, expected.size) || !runBoundFileIdentityMatches(expected, opened) {
		_ = file.Close()
		return nil, fmt.Errorf("%w: RunView output file changed while opening", craft.ErrConflict)
	}
	content, readErr := io.ReadAll(io.LimitReader(file, expected.size+1))
	var after unix.Stat_t
	statErr := unix.Fstat(fd, &after)
	closeErr := file.Close()
	var pathAfter unix.Stat_t
	pathErr := unix.Fstatat(parentFD, name, &pathAfter, unix.AT_SYMLINK_NOFOLLOW)
	if readErr != nil || statErr != nil || closeErr != nil || pathErr != nil {
		return nil, errors.Join(readErr, statErr, closeErr, pathErr)
	}
	if int64(len(content)) != expected.size || !runViewDraftFileStatsMatch(opened, after, expected.size) ||
		!runViewDraftFileStatsMatch(opened, pathAfter, expected.size) || !runBoundFileIdentityMatches(expected, after) || !runBoundFileIdentityMatches(expected, pathAfter) {
		return nil, fmt.Errorf("%w: RunView output file changed while reading", craft.ErrConflict)
	}
	sum := sha256.Sum256(content)
	if hex.EncodeToString(sum[:]) != expected.digest {
		return nil, fmt.Errorf("%w: RunView output file bytes changed after list", craft.ErrConflict)
	}
	for i := len(dirs) - 1; i >= 0; i-- {
		if err := verifyRunBoundArtifactDirectoryEntries(dirs[i].fd, dirs[i].prefix, listed); err != nil {
			return nil, err
		}
		var after unix.Stat_t
		statErr := unix.Fstat(dirs[i].fd, &after)
		if statErr != nil || !runBoundDirIdentityMatches(dirs[i].id, after) {
			return nil, fmt.Errorf("%w: RunView output directory changed while reading", craft.ErrConflict)
		}
		if dirs[i].parent >= 0 {
			var pathAfter unix.Stat_t
			if pathErr := unix.Fstatat(dirs[i].parent, dirs[i].name, &pathAfter, unix.AT_SYMLINK_NOFOLLOW); pathErr != nil ||
				!runBoundDirIdentityMatches(dirs[i].id, pathAfter) {
				return nil, fmt.Errorf("%w: RunView output directory path changed while reading", craft.ErrConflict)
			}
		}
	}
	if err := verifyRunBoundArtifactDirectoryEntries(rootDup, "", listed); err != nil {
		return nil, err
	}
	var rootAfter unix.Stat_t
	if err := unix.Fstat(rootDup, &rootAfter); err != nil || !runBoundDirIdentityMatches(rootID, rootAfter) {
		return nil, fmt.Errorf("%w: RunView output root changed while reading", craft.ErrConflict)
	}
	return content, nil
}

func runBoundFileIdentityMatches(identity runBoundArtifactIdentity, stat unix.Stat_t) bool {
	return !identity.dir && stat.Mode&unix.S_IFMT == unix.S_IFREG && uint32(stat.Mode) == identity.mode &&
		stat.Nlink == 1 && stat.Size == identity.size && uint64(stat.Dev) == identity.device && stat.Ino == identity.inode
}

func runBoundDirIdentityMatches(identity runBoundArtifactIdentity, stat unix.Stat_t) bool {
	return identity.dir && stat.Mode&unix.S_IFMT == unix.S_IFDIR && uint32(stat.Mode) == identity.mode &&
		uint64(stat.Dev) == identity.device && stat.Ino == identity.inode && uint64(stat.Nlink) == identity.nlink &&
		stat.Size == identity.size && runBoundStatTimestamp(stat, "Mtim", "Mtimespec") == identity.mtime &&
		runBoundStatTimestamp(stat, "Ctim", "Ctimespec") == identity.ctime
}

func verifyRunBoundArtifactDirectoryEntries(dirFD int, prefix string, listed map[string]runBoundArtifactIdentity) error {
	expected := make(map[string]struct{})
	for rel := range listed {
		if rel == "" {
			continue
		}
		remaining := rel
		if prefix != "" {
			p := prefix + "/"
			if !strings.HasPrefix(rel, p) {
				continue
			}
			remaining = strings.TrimPrefix(rel, p)
		}
		if remaining != "" && !strings.Contains(remaining, "/") {
			expected[remaining] = struct{}{}
		}
	}
	if len(expected) > runBoundArtifactMaxEntriesPerDirectory {
		return fmt.Errorf("%w: listed RunView output directory exceeds entry limit", craft.ErrConflict)
	}
	children, overflow, err := readRunBoundArtifactDirEntries(dirFD, runBoundArtifactMaxEntriesPerDirectory)
	if err != nil {
		return err
	}
	if overflow || len(children) != len(expected) {
		return fmt.Errorf("%w: RunView output directory entry set changed after list", craft.ErrConflict)
	}
	for _, child := range children {
		if _, ok := expected[child.Name()]; !ok {
			return fmt.Errorf("%w: RunView output directory entry set changed after list", craft.ErrConflict)
		}
	}
	return nil
}

// localCraftArtifactSource is the legacy shared-serve adapter retained for
// non-RunView wiring only. RunView collection must use runBoundCraftArtifactSource.
type localCraftArtifactSource struct {
	workDir   string
	outputDir string
}

var _ service.SandboxArtifactSource = (*localCraftArtifactSource)(nil)

func (s *localCraftArtifactSource) ListSessionFiles(ctx context.Context, sessionID, dir string) ([]sandbox.RemoteDirEntry, error) {
	root := filepath.Join(s.workDir, filepath.FromSlash(pathCleanForward(dir)))
	// The serve-relative output path is the pointer symlink: resolve it so
	// the walk sees the delegation's real directory.
	if resolved, rerr := filepath.EvalSymlinks(root); rerr == nil {
		root = resolved
	}
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // nothing produced yet
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, nil
	}
	prefix := pathCleanForward(dir)
	var entries []sandbox.RemoteDirEntry
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return rerr
		}
		entry := sandbox.RemoteDirEntry{
			Name: d.Name(),
			Path: prefix + "/" + pathCleanForward(rel),
		}
		switch {
		case d.Type()&os.ModeSymlink != 0:
			entry.Type = sandbox.RemoteEntryOther
		case d.IsDir():
			entry.Type = sandbox.RemoteEntryDir
		default:
			entry.Type = sandbox.RemoteEntryFile
			if meta, merr := d.Info(); merr == nil {
				entry.Size = meta.Size()
				entry.ModTime = meta.ModTime()
			}
		}
		entries = append(entries, entry)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return entries, nil
}

func (s *localCraftArtifactSource) ReadSessionFile(ctx context.Context, sessionID, path string) ([]byte, error) {
	target := filepath.Join(s.workDir, filepath.FromSlash(pathCleanForward(path)))
	cap := craftLocalMaxReadBytes + 1
	raw, err := os.ReadFile(target)
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) >= cap {
		return nil, fmt.Errorf("%w: artifact %q exceeds the local read cap", craft.ErrInvalidInput, path)
	}
	return raw, nil
}

// craftRunEventSink is the durable event append surface the emitter needs;
// *repository.AgentRunStore satisfies it in the container assembly.
type craftRunEventSink interface {
	AppendEvent(context.Context, agentruntime.Fence, agentruntime.RunEvent) (agentruntime.RunEvent, error)
}

// craftRunEventEmitter projects the executor's craft events into the durable
// run event stream under the delegation's fence: the browser subscription
// (GET /sessions/:id/runs/:run/events) replays exactly these frames and the
// W04 controller projects them with the shared seq counter.
func craftRunEventEmitter(runs craftRunEventSink) func(context.Context, craft.Task, string, json.RawMessage) error {
	return func(ctx context.Context, task craft.Task, kind string, data json.RawMessage) error {
		payload, err := json.Marshal(struct {
			Kind         string          `json:"kind"`
			WorkspaceID  string          `json:"workspace_id"`
			DelegationID string          `json:"delegation_id,omitempty"`
			ToolCallID   string          `json:"tool_call_id,omitempty"`
			Data         json.RawMessage `json:"data"`
		}{Kind: kind, WorkspaceID: task.WorkspaceID, DelegationID: task.ID, ToolCallID: task.ToolCallID, Data: data})
		if err != nil {
			return err
		}
		if _, err := runs.AppendEvent(ctx, task.Fence, agentruntime.RunEvent{Type: "craft", Payload: payload}); err != nil {
			return err
		}
		return nil
	}
}

// -----------------------------------------------------------------------------
// C05 assembly: recovery snapshots + the C04 recovery program hook.
// -----------------------------------------------------------------------------

// craftRuntimeDigestFromEnv is the W06 runtime identity semantic shared by
// the executor, the C04 recovery config and the C05 snapshot service:
// CRAFT_OPENCODE_RUNTIME_DIGEST overrides, the honest local-serve default
// otherwise.
func craftRuntimeDigestFromEnv() string {
	if digest := strings.TrimSpace(os.Getenv(craftOpenCodeRuntimeDigestEnv)); digest != "" {
		return digest
	}
	return craftLocalRuntimeDigest
}

// setSnapshotCapture installs the best-effort post-publish capture hook (the
// snapshot service registers itself once assembled; without it versions
// still publish, they are just not restorable).
func (e *localCraftRuntime) setSnapshotCapture(capture func(context.Context, craft.Task, string)) {
	e.snapshotCapture = capture
}

// setInteractionEmitter installs the C02 interaction.pending registrar
// around the current emitter. It exists to break the construction-time
// provider cycle (executor → interaction assembly → agent runtime →
// executor): the assembly is built after the runtime, and the container
// wires the registrar in once both sides exist, before any traffic. Because
// the inner executor reads e.emit INDIRECTLY (see newCraftRuntimeExecutor),
// replacing the field here is visible to the sub-execution's event path —
// the production emission point for interaction.pending.
func (e *localCraftRuntime) setInteractionEmitter(emit func(context.Context, craft.Task, string, json.RawMessage) error) {
	if e != nil && emit != nil {
		e.emit = emit
	}
}

// localCraftSnapshotSource is the local single-serve implementation of the
// C05 isolated-state source: the OpenCode persistent data is read through
// the serve process that owns it (a consistent point-in-time read — it never
// copies a SQLite file mid-write or a half-written WAL), files materialize
// into a fresh generation directory under the serve workspace root, and the
// provider's isolated-data capability is exactly "the serve still carries
// the session".
type localCraftSnapshotSource struct {
	runtime *localCraftRuntime
	files   interfaces.FileService
}

var _ service.CraftSnapshotSource = (*localCraftSnapshotSource)(nil)

func (s *localCraftSnapshotSource) Quiescent(ctx context.Context, ws craft.Workspace) (bool, string, error) {
	status, err := s.runtime.client.Status(ctx, ws.OpenCodeSessionID)
	if err != nil {
		return false, "", err
	}
	if status != "idle" {
		return false, "opencode session status is " + status, nil
	}
	return true, "", nil
}

func (s *localCraftSnapshotSource) ExportSessionData(ctx context.Context, ws craft.Workspace) (craft.SessionExport, error) {
	messages, err := s.runtime.client.Messages(ctx, ws.OpenCodeSessionID)
	if err != nil {
		return craft.SessionExport{}, fmt.Errorf("read opencode session %s: %w", ws.OpenCodeSessionID, err)
	}
	records := make([]craft.SessionRecord, 0, len(messages))
	for _, m := range messages {
		parts := make([]string, 0, len(m.Parts))
		for _, p := range m.Parts {
			parts = append(parts, string(p))
		}
		records = append(records, craft.SessionRecord{
			ID: m.ID, ParentID: m.ParentID, Role: m.Role,
			Finish: m.Finish, CompletedAt: m.CompletedAt, Parts: parts,
		})
	}
	return craft.SessionExport{
		OpenCodeSessionID: ws.OpenCodeSessionID,
		SchemaVersion:     "opencode-1.18.4/messages-v1",
		Records:           records,
	}, nil
}

// RestoreGeneration materializes the version files into a fresh generation
// directory under the serve workspace root (every object re-read through
// controlled storage and verified against its manifest digest) and verifies
// the OpenCode session still exists with the snapshot's message chain as a
// verifiable prefix. The candidate binding it returns is not live until the
// service's CAS wins.
func (s *localCraftSnapshotSource) RestoreGeneration(
	ctx context.Context, ws craft.Workspace, files []craft.File, export craft.SessionExport,
) (craft.Workspace, error) {
	live, err := s.runtime.client.Messages(ctx, ws.OpenCodeSessionID)
	if err != nil {
		return craft.Workspace{}, fmt.Errorf("%w: opencode session %s no longer answers; its persistent data is not restorable through this provider: %v",
			craft.ErrUnsupported, ws.OpenCodeSessionID, err)
	}
	if err := craft.SessionChainPrefix(export.Records, s.recordsWithParts(live)); err != nil {
		return craft.Workspace{}, fmt.Errorf("%w: the opencode session no longer carries the snapshot chain: %v", craft.ErrConflict, err)
	}

	generation := fmt.Sprintf("restore-%d", time.Now().UTC().UnixNano())
	root := filepath.Join(s.runtime.sessionsRoot, "restored", ws.ID, generation)
	for _, f := range files {
		if err := craft.ValidateArtifactPath(f.Path); err != nil {
			return craft.Workspace{}, err
		}
		reader, rerr := s.files.GetFile(ctx, f.Ref)
		if rerr != nil {
			return craft.Workspace{}, fmt.Errorf("craft: read snapshot object %s: %w", f.Path, rerr)
		}
		content, ierr := io.ReadAll(io.LimitReader(reader, craftLocalMaxReadBytes+1))
		_ = reader.Close()
		if ierr != nil {
			return craft.Workspace{}, ierr
		}
		if int64(len(content)) != f.Bytes {
			return craft.Workspace{}, fmt.Errorf("%w: restored file %s stores %d bytes, manifest pins %d",
				craft.ErrConflict, f.Path, len(content), f.Bytes)
		}
		sum := sha256.Sum256(content)
		if hex.EncodeToString(sum[:]) != f.SHA256 {
			return craft.Workspace{}, fmt.Errorf("%w: restored file %s fails its manifest digest",
				craft.ErrConflict, f.Path)
		}
		target := filepath.Join(root, filepath.FromSlash(f.Path))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return craft.Workspace{}, err
		}
		if err := os.WriteFile(target, content, 0o644); err != nil {
			return craft.Workspace{}, err
		}
	}
	logger.Infof(ctx, "[CraftRuntime] restored generation %s from %d verified files", generation, len(files))
	return craft.Workspace{
		Scope: ws.Scope, ID: ws.ID, SandboxID: ws.SandboxID,
		Generation: generation, OpenCodeSessionID: ws.OpenCodeSessionID,
		RuntimeDigest: ws.RuntimeDigest,
	}, nil
}

// recordsWithParts converts live messages including their verbatim parts.
func (s *localCraftSnapshotSource) recordsWithParts(messages []opencode.Message) []craft.SessionRecord {
	records := make([]craft.SessionRecord, 0, len(messages))
	for _, m := range messages {
		parts := make([]string, 0, len(m.Parts))
		for _, p := range m.Parts {
			parts = append(parts, string(p))
		}
		records = append(records, craft.SessionRecord{
			ID: m.ID, ParentID: m.ParentID, Role: m.Role,
			Finish: m.Finish, CompletedAt: m.CompletedAt, Parts: parts,
		})
	}
	return records
}

func (s *localCraftSnapshotSource) ReleaseGeneration(ctx context.Context, candidate craft.Workspace) error {
	if candidate.Generation == "" {
		return nil
	}
	root := filepath.Join(s.runtime.sessionsRoot, "restored", candidate.ID, candidate.Generation)
	if err := os.RemoveAll(root); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s *localCraftSnapshotSource) IsolatedDataRestore() bool { return true }

// newCraftSnapshotService assembles the C05 snapshot service onto the local
// real runtime. Without CRAFT_OPENCODE_BASE_URL the executor is the R05
// fail-closed one and this provider answers a nil service: the snapshot
// routes then never mount, and captures simply do not happen.
func newCraftSnapshotService(
	db *gorm.DB,
	sessions interfaces.SessionService,
	store craft.Store,
	versions craft.VersionStore,
	files interfaces.FileService,
	executor craft.Executor,
) (*service.CraftSnapshotService, error) {
	runtime, ok := executor.(*localCraftRuntime)
	if !ok {
		return nil, nil
	}
	source := &localCraftSnapshotSource{runtime: runtime, files: files}
	svc, err := service.NewCraftSnapshotService(service.CraftSnapshotConfig{
		DB: db, Sessions: sessions, Store: store, Versions: versions,
		Snapshots: repository.NewCraftSnapshotStore(db), Files: files,
		Source: source, ActiveRuns: service.CraftActiveRunsQuery(db),
		RuntimeDigest: runtime.runtimeDigest,
	})
	if err != nil {
		return nil, err
	}
	// Best-effort post-publish capture: the delegation just completed, the
	// runtime is idle and no delegation of this workspace is pending, which
	// is exactly the C05 quiescence gate. A failed capture only leaves that
	// version without a restorable snapshot (the workbench then shows the
	// download-only reason) — the published version itself is untouched.
	runtime.setSnapshotCapture(func(ctx context.Context, task craft.Task, versionID string) {
		if _, err := svc.Capture(ctx, task.Scope, versionID); err != nil {
			logger.Warnf(ctx, "[CraftRuntime] post-publish snapshot capture failed for version %s: %v", versionID, err)
		}
	})
	return svc, nil
}

// craftRecoveryReconcileBudget bounds one craft reconciliation inside the
// worker recovery hook even when the delegation carries no persisted
// deadline (C04 review nit-2, assembly half).
const craftRecoveryReconcileBudget = 15 * time.Minute

// newCraftRecoveryHook chains C04's post-failure reconciliation program into
// the worker recovery path: every unfinished craft delegation of the claimed
// run is reconciled (reuse / collect / observe / durable wait) before the
// graph executes. An unknown outcome or a sandbox-class problem parks the
// run durably inside Reconcile; the returned error then skips execution.
func newCraftRecoveryHook(
	db *gorm.DB,
	recovery *service.CraftRecovery,
) func(context.Context, agentruntime.Fence) error {
	return func(ctx context.Context, fence agentruntime.Fence) error {
		if db == nil || recovery == nil {
			return nil
		}
		var ids []string
		if err := db.WithContext(ctx).Table("craft_delegations").
			Where("tenant_id = ? AND run_id = ? AND result_json IS NULL", fence.TenantID, fence.RunID).
			Order("created_at, id").Pluck("id", &ids).Error; err != nil {
			// Transient read: leave the run non-terminal so the expiring
			// lease triggers a bounded reclaim.
			return err
		}
		for _, id := range ids {
			taskCtx, cancel := context.WithTimeout(ctx, craftRecoveryReconcileBudget)
			_, err := recovery.Reconcile(taskCtx, fence, id)
			cancel()
			if err != nil {
				// Reconcile already persisted its durable wait classes; the
				// error skips graph execution for this claim.
				return err
			}
		}
		return nil
	}
}
