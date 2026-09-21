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
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/agent/opencode"
	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/modules/execution/sandbox"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
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
	craftLocalMaxReadBytes = int64(50 << 20)
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
	db              *gorm.DB
	client          *opencode.Client
	store           craft.Store
	files           interfaces.FileService
	inner           craft.Executor
	artifacts       *service.CraftArtifactService
	emit            func(context.Context, craft.Task, string, json.RawMessage) error
	workDir         string
	outputDir       string
	runtimeDigest   string
	sessionsRoot    string
	snapshotCapture func(context.Context, craft.Task, string)
}

// Execute runs one delegation through the real chain.
func (e *localCraftRuntime) Execute(ctx context.Context, task craft.Task) (craft.Result, error) {
	if err := e.ensureWorkspace(ctx, task); err != nil {
		return craft.Result{}, err
	}
	if err := e.stageWorkspaceInputs(ctx, task); err != nil {
		return craft.Result{}, err
	}
	if err := e.pointWorkspaceOutput(ctx, task); err != nil {
		return craft.Result{}, err
	}
	adopted, aerr := e.adoptExecutorMessageID(ctx, task)
	if aerr != nil {
		return craft.Result{}, aerr
	}
	task = adopted
	result, err := e.inner.Execute(ctx, task)
	if err != nil {
		return result, err
	}
	if result.Status != "succeeded" {
		return result, nil
	}
	// D01 wiring: the version carries the SESSION's kind (craft_sessions),
	// so the entry check judges the kind's own deliverable and the manifest
	// admission gate fires for document/spreadsheet/slides rounds. An
	// unreadable kind falls back to web — the fail-closed default the W01
	// collector shipped with.
	version, verr := e.artifacts.CollectForKind(ctx, task, e.sessionKind(ctx, task))
	if verr != nil {
		// A finished execution whose output cannot be collected is a failed
		// craft round: settle the durable result as failed so neither the
		// main model nor the workbench is shown a success without a version
		// (W01: a failed collection leaves no version visible).
		failed := result
		failed.Status = "failed"
		failed.Summary = fmt.Sprintf("sub-execution finished but the output was not publishable: %v", verr)
		if serr := e.store.SaveResult(ctx, task.Fence, failed); serr != nil {
			logger.Warnf(ctx, "[CraftRuntime] persisting the collection failure failed: %v", serr)
		}
		logger.Warnf(ctx, "[CraftRuntime] artifact collection failed for run %s: %v", task.Fence.RunID, verr)
		return failed, nil
	}
	published := result
	published.Files = version.Files
	if raw, merr := json.Marshal(map[string]any{"version_id": version.ID}); merr == nil {
		_ = e.emit(ctx, task, "artifact.published", raw)
	}
	if e.snapshotCapture != nil {
		// C05: the round just completed verifiably, so this is the natural
		// quiescent point to capture the files+session snapshot of the
		// published version (best effort — see newCraftSnapshotService).
		e.snapshotCapture(ctx, task, version.ID)
	}
	logger.Infof(ctx, "[CraftRuntime] delegation %s published version %s (%d files)",
		task.ID, version.ID, len(version.Files))
	return published, nil
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
	if strings.HasPrefix(task.PromptMessageID, "msg_") {
		return task, nil
	}
	if e.db == nil {
		return task, nil
	}
	fresh, err := opencode.NewMessageID()
	if err != nil {
		return task, err
	}
	task.PromptMessageID = fresh
	encoded, err := json.Marshal(task)
	if err != nil {
		return task, err
	}
	if err := e.db.WithContext(ctx).Exec(
		"UPDATE craft_delegations SET task_json = ?, prompt_message_id = ? WHERE tenant_id = ? AND run_id = ? AND tool_call_id = ?",
		string(encoded), fresh, task.Fence.TenantID, task.Fence.RunID, task.ToolCallID).Error; err != nil {
		return craft.Task{}, err
	}
	logger.Infof(ctx, "[CraftRuntime] delegation %s adopted executor message id %s", task.ID, fresh)
	return task, nil
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

// stageWorkspaceInputs materializes the session's associated (W03)
// workspace input manifest into the serve working directory's inputs/ tree
// at the content-addressed paths the delegation prompt already names
// (inputs/<sha256>/<name>), verifying every byte stream against its
// recorded digest. Content addressing makes the shared tree safe across
// sessions: identical bytes land on identical paths.
func (e *localCraftRuntime) stageWorkspaceInputs(ctx context.Context, task craft.Task) error {
	inputs, err := e.workspaceInputManifest(ctx, task)
	if err != nil {
		return err
	}
	task.Inputs = inputs
	if len(task.Inputs) == 0 {
		return nil
	}
	if err := craft.ValidateInputManifest(task.Inputs); err != nil {
		return err
	}
	sort.Slice(task.Inputs, func(i, j int) bool { return task.Inputs[i].SHA256 < task.Inputs[j].SHA256 })
	for _, in := range task.Inputs {
		rel, err := craft.InputPath(in.SHA256, in.Name)
		if err != nil {
			return err
		}
		target := filepath.Join(e.workDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if info, serr := os.Stat(target); serr == nil && info.Size() == in.Bytes {
			continue // already staged with the declared size
		}
		reader, err := e.files.GetFile(ctx, in.Ref)
		if err != nil {
			return fmt.Errorf("craft local runtime stage input %s: %w", in.Name, err)
		}
		content, err := io.ReadAll(io.LimitReader(reader, craftLocalStagedReadBudget))
		_ = reader.Close()
		if err != nil {
			return err
		}
		if int64(len(content)) != in.Bytes {
			return fmt.Errorf("%w: staged input %s declares %d bytes but stored %d",
				craft.ErrInvalidInput, in.Name, in.Bytes, len(content))
		}
		sum := sha256.Sum256(content)
		if hex.EncodeToString(sum[:]) != in.SHA256 {
			return fmt.Errorf("%w: staged input %s digest mismatch", craft.ErrInvalidInput, in.Name)
		}
		if err := os.WriteFile(target, content, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// workspaceInputManifest loads the W03-associated inputs of the task's
// workspace (craft_workspace_inputs) in creation order. An unreadable
// manifest aborts the delegation rather than staging a partial set.
func (e *localCraftRuntime) workspaceInputManifest(ctx context.Context, task craft.Task) ([]craft.Input, error) {
	if e.db == nil {
		return nil, nil
	}
	var rows []struct {
		Ref    string
		Name   string
		SHA256 string
		Bytes  int64
	}
	if err := e.db.WithContext(ctx).Table("craft_workspace_inputs").
		Select("ref, name, sha256, bytes").
		Where("workspace_id = ?", task.WorkspaceID).
		Order("created_at, ref").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("craft local runtime read workspace inputs: %w", err)
	}
	inputs := make([]craft.Input, 0, len(rows))
	for _, row := range rows {
		inputs = append(inputs, craft.Input{Ref: row.Ref, Name: row.Name, SHA256: row.SHA256, Bytes: row.Bytes})
	}
	if err := craft.ValidateInputManifest(inputs); err != nil {
		return nil, err
	}
	return inputs, nil
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

// localCraftArtifactSource adapts the delegation output directories to the
// W01 collector's SandboxArtifactSource. The serve-relative output path
// (<workDir>/<outputDir>) is the runtime's pointer symlink into the active
// delegation's ws/<key>/<outputDir> directory, so resolving dir relative to
// the serve working directory follows exactly this delegation's output.
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
