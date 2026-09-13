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

	"github.com/Tencent/WeKnora/internal/agent/opencode"
	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/sandbox"
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
	interactions *CraftInteractionAssembly,
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
	runtimeDigest := strings.TrimSpace(os.Getenv(craftOpenCodeRuntimeDigestEnv))
	if runtimeDigest == "" {
		runtimeDigest = craftLocalRuntimeDigest
	}
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
	// pending decision is decidable through the HTTP surface.
	emit := craftRunEventEmitter(runs)
	if interactions != nil {
		emit = craftInteractionRegistrar(client, store, interactions.Store, interactions.Runs, emit)
	}
	runtime := &localCraftRuntime{
		db:            db,
		client:        client,
		store:         store,
		files:         files,
		inner:         opencode.NewExecutor(client, store, emit),
		artifacts:     artifacts,
		emit:          craftRunEventEmitter(runs),
		outputDir:     outputDir,
		runtimeDigest: runtimeDigest,
		sessionsRoot:  filepath.Join(workDir, "ws"),
		workDir:       workDir,
	}
	logger.Infof(context.Background(),
		"[CraftRuntime] local real runtime assembled: serve=%s work_dir=%s output=%s", baseURL, workDir, outputDir)
	return runtime, nil
}

// localCraftRuntime implements craft.Executor for the local single-serve
// deployment: provision the OpenCode binding, stage authorized inputs, frame
// the sub-prompt with the session workspace, execute through the R04
// executor, then publish the finished output as an immutable version.
type localCraftRuntime struct {
	db            *gorm.DB
	client        *opencode.Client
	store         craft.Store
	files         interfaces.FileService
	inner         craft.Executor
	artifacts     *service.CraftArtifactService
	emit          func(context.Context, craft.Task, string, json.RawMessage) error
	workDir       string
	outputDir     string
	runtimeDigest string
	sessionsRoot  string
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
	version, verr := e.artifacts.Collect(ctx, task)
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
	logger.Infof(ctx, "[CraftRuntime] delegation %s published version %s (%d files)",
		task.ID, version.ID, len(version.Files))
	return published, nil
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
