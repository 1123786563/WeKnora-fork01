package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestCraftT01Journey(t *testing.T) {
	env := newCraftSessionEnv(t, openGate)
	env.svc.files = &craftT01Files{blobs: map[string][]byte{}}
	ws := createCraftSession(t, env, "u1", "t01-journey", "Opaque input", "web")
	scope := ownerScope(1, "u1", ws.SessionID)
	ctx := craftCtx(1, "u1", ws.SessionID)
	input, err := env.svc.AcceptInputRound(ctx, scope, []CraftInputUpload{{Name: "ledger.mystery", Content: []byte("opaque payload"), SHA256: sha256Sum("opaque payload")}})
	require.NoError(t, err)
	require.Len(t, input, 1)
	require.Equal(t, &craft.InputRecognition{Accepted: true, Understood: false, Reason: "unrecognized_format"}, input[0].Recognition)
	persisted, err := env.svc.WorkspaceInputs(ctx, scope)
	require.NoError(t, err)
	require.Equal(t, input, persisted)
	var stagedPath string
	stager := NewCraftInputService(
		func(ctx context.Context, _ craft.Scope, ref string) ([]byte, error) {
			reader, err := env.svc.files.GetFile(ctx, ref)
			if err != nil {
				return nil, err
			}
			defer reader.Close()
			return io.ReadAll(reader)
		},
		func(_ context.Context, _ craft.Workspace, path string, content []byte) error {
			stagedPath = path
			require.Equal(t, []byte("opaque payload"), content)
			return nil
		},
	)
	staged, err := stager.Stage(ctx, scope, ws, input)
	require.NoError(t, err)
	require.Equal(t, input, staged)
	require.Equal(t, "inputs/"+input[0].SHA256+"/ledger.mystery", stagedPath)
	_, _, err = env.svc.StartRun(ctx, scope, CraftRunRequest{RequestID: "r1", Prompt: "Make a page", InputRefs: []string{input[0].Ref}})
	require.ErrorIs(t, err, craft.ErrConflict)
	require.NoError(t, env.svc.DecideInput(ctx, scope, input[0].Ref, "cancel"))
	_, _, err = env.svc.StartRun(ctx, scope, CraftRunRequest{RequestID: "r1", Prompt: "Make a page", InputRefs: []string{input[0].Ref}})
	require.ErrorIs(t, err, craft.ErrConflict)
	var count int64
	require.NoError(t, env.db.Table("agent_runs").Where("session_id = ?", ws.SessionID).Count(&count).Error)
	require.Zero(t, count, "cancel must submit no Run")
	require.NoError(t, env.svc.DecideInput(ctx, scope, input[0].Ref, "continue"))
	run, _, err := env.svc.StartRun(ctx, scope, CraftRunRequest{RequestID: "r1", Prompt: "Make a page", InputRefs: []string{input[0].Ref}})
	require.NoError(t, err)
	replay, _, err := env.svc.StartRun(ctx, scope, CraftRunRequest{RequestID: "r1", Prompt: "Make a page", InputRefs: []string{input[0].Ref}})
	require.NoError(t, err)
	require.Equal(t, run.Key.RunID, replay.Key.RunID)
	require.NoError(t, env.db.Table("agent_runs").Where("session_id = ?", ws.SessionID).Count(&count).Error)
	require.EqualValues(t, 1, count)
	_, err = env.svc.AcceptInputRound(ctx, scope, []CraftInputUpload{{Name: "../escape", Content: []byte("x"), SHA256: sha256Sum("x")}})
	require.ErrorIs(t, err, craft.ErrInvalidInput)
	_, err = env.svc.AcceptInputRound(ctx, scope, []CraftInputUpload{{Name: "empty.bin", SHA256: sha256Sum("")}})
	require.ErrorIs(t, err, craft.ErrInvalidInput)
	_, err = env.svc.AcceptInputRound(ctx, scope, []CraftInputUpload{{Name: "wrong.bin", Content: []byte("x"), SHA256: sha256Sum("wrong")}})
	require.ErrorIs(t, err, craft.ErrInvalidInput)
	_, err = env.svc.AcceptInputRound(context.Background(), scope, []CraftInputUpload{{Name: "unauthorized.bin", Content: []byte("x"), SHA256: sha256Sum("x")}})
	require.True(t, errors.Is(err, craft.ErrNotFound) || errors.Is(err, craft.ErrForbidden))
}

func TestCraftT01RestartRecoversAdmissionClaimWithNoDurableRun(t *testing.T) {
	env := newCraftSessionEnv(t, openGate)
	env.svc.files = &craftT01Files{blobs: map[string][]byte{}}
	ws := createCraftSession(t, env, "u1", "t01-abandoned-admission", "Opaque input", "web")
	scope := ownerScope(1, "u1", ws.SessionID)
	ctx := craftCtx(1, "u1", ws.SessionID)
	inputs, err := env.svc.AcceptInputRound(ctx, scope, []CraftInputUpload{{Name: "ledger.mystery", Content: []byte("opaque payload"), SHA256: sha256Sum("opaque payload")}})
	require.NoError(t, err)
	require.NoError(t, env.svc.DecideInput(ctx, scope, inputs[0].Ref, "continue"))
	session, err := env.svc.writeSession(ctx, scope, ws.SessionID)
	require.NoError(t, err)
	selected, err := env.svc.resolveInputRefs(ctx, ws.ID, []string{inputs[0].Ref})
	require.NoError(t, err)
	// Model a process interruption after the durable claim but before Submit.
	_, _, err = env.svc.claimInputDecisions(ctx, session, selected, "craft-abandoned", "abandoned-run", "u1")
	require.NoError(t, err)
	var claim struct {
		RunID        string    `gorm:"column:admission_run_id"`
		Token        string    `gorm:"column:admission_token"`
		State        string    `gorm:"column:admission_state"`
		LeaseExpires time.Time `gorm:"column:lease_expires_at"`
	}
	require.NoError(t, env.db.Table("craft_session_requests").Select(
		"admission_run_id, admission_token, admission_state, lease_expires_at",
	).Where("tenant_id = ? AND user_id = ? AND session_id = ? AND purpose = ? AND request_id = ?",
		1, "u1", ws.SessionID, "input_admission", craftInputDecisionKey(inputs[0].Ref)).Take(&claim).Error)
	require.NotEmpty(t, claim.RunID, "the preallocated server Run ID must be durable before Submit")
	require.NotEmpty(t, claim.Token, "the admission claim must carry a server-generated fencing token")
	require.Equal(t, "claimed", claim.State)
	require.True(t, claim.LeaseExpires.After(time.Now()))
	// Expiry only permits transactional recovery; it must not itself clear the
	// claim or allow the old token to cross AgentRunStore.Admit.
	require.NoError(t, env.db.Table("craft_session_requests").Where(
		"tenant_id = ? AND user_id = ? AND session_id = ? AND purpose = ? AND request_id = ?",
		1, "u1", ws.SessionID, "input_admission", craftInputDecisionKey(inputs[0].Ref),
	).Update("lease_expires_at", time.Now().Add(-time.Minute)).Error)
	var admitted int64
	require.NoError(t, env.db.Table("agent_runs").Where("tenant_id = ? AND owner_id = ? AND request_id = ?", 1, "u1", "craft-abandoned").Count(&admitted).Error)
	require.Zero(t, admitted, "the interrupted request has no authoritative Run")

	// A freshly assembled service has no in-memory knowledge of the interrupted
	// StartRun and must reconcile from durable admission state.
	restarted, err := NewCraftSessionService(CraftSessionConfig{
		DB: env.db, Sessions: env.sessions, Store: env.store, Versions: env.versions,
		Runs: env.runs, ActiveRuns: CraftActiveRunsQuery(env.db), TemporaryDocs: env.docs,
		Files: env.svc.files, Models: env.models, Gate: openGate,
	})
	require.NoError(t, err)
	require.NoError(t, restarted.DecideInput(ctx, scope, inputs[0].Ref, "cancel"),
		"a restart must recover a claim whose request has no durable Run")
}

func TestCraftT01RestartReplaysCommittedAdmissionClaim(t *testing.T) {
	env := newCraftSessionEnv(t, openGate)
	env.svc.files = &craftT01Files{blobs: map[string][]byte{}}
	ws := createCraftSession(t, env, "u1", "t01-committed-admission", "Opaque input", "web")
	scope := ownerScope(1, "u1", ws.SessionID)
	ctx := craftCtx(1, "u1", ws.SessionID)
	inputs, err := env.svc.AcceptInputRound(ctx, scope, []CraftInputUpload{{Name: "ledger.mystery", Content: []byte("opaque payload"), SHA256: sha256Sum("opaque payload")}})
	require.NoError(t, err)
	require.NoError(t, env.svc.DecideInput(ctx, scope, inputs[0].Ref, "continue"))
	first, _, err := env.svc.StartRun(ctx, scope, CraftRunRequest{RequestID: "committed", Prompt: "build", InputRefs: []string{inputs[0].Ref}})
	require.NoError(t, err)
	// Simulate an old/partial marker left as claimed even though the exact Run
	// is durable. Recovery must reconcile it, while refusing to replace the
	// user's decision with cancel.
	require.NoError(t, env.db.Table("craft_session_requests").Where(
		"tenant_id = ? AND user_id = ? AND session_id = ? AND purpose = ? AND request_id = ?",
		1, "u1", ws.SessionID, "input_admission", craftInputDecisionKey(inputs[0].Ref),
	).Updates(map[string]any{"admission_state": "claimed", "lease_expires_at": time.Now().Add(-time.Minute)}).Error)
	require.ErrorIs(t, env.svc.DecideInput(ctx, scope, inputs[0].Ref, "cancel"), craft.ErrConflict)
	var reconciledState string
	require.NoError(t, env.db.Table("craft_session_requests").Select("admission_state").Where(
		"tenant_id = ? AND user_id = ? AND session_id = ? AND purpose = ? AND request_id = ?",
		1, "u1", ws.SessionID, "input_admission", craftInputDecisionKey(inputs[0].Ref),
	).Scan(&reconciledState).Error)
	require.Equal(t, "admitted", reconciledState)
	session, err := env.svc.writeSession(ctx, scope, ws.SessionID)
	require.NoError(t, err)
	selected, err := env.svc.resolveInputRefs(ctx, ws.ID, []string{inputs[0].Ref})
	require.NoError(t, err)
	// Simulate termination after Submit committed, before the normal fence
	// release. Retrying the same key must replay, not create a second Run.
	_, _, err = env.svc.claimInputDecisions(ctx, session, selected, "craft-committed", first.Key.RunID, "u1")
	require.NoError(t, err)
	restarted, err := NewCraftSessionService(CraftSessionConfig{
		DB: env.db, Sessions: env.sessions, Store: env.store, Versions: env.versions,
		Runs: env.runs, ActiveRuns: CraftActiveRunsQuery(env.db), TemporaryDocs: env.docs,
		Files: env.svc.files, Models: env.models, Gate: openGate,
	})
	require.NoError(t, err)
	replay, _, err := restarted.StartRun(ctx, scope, CraftRunRequest{RequestID: "committed", Prompt: "build", InputRefs: []string{inputs[0].Ref}})
	require.NoError(t, err)
	require.Equal(t, first.Key.RunID, replay.Key.RunID)
	var admitted int64
	require.NoError(t, env.db.Table("agent_runs").Where("tenant_id = ? AND owner_id = ? AND request_id = ?", 1, "u1", "craft-committed").Count(&admitted).Error)
	require.EqualValues(t, 1, admitted)
}

func TestCraftT01RecoveryRotatesExpiredClaimAndFencesPausedSubmit(t *testing.T) {
	env := newCraftSessionEnv(t, openGate)
	env.svc.files = &craftT01Files{blobs: map[string][]byte{}}
	ws := createCraftSession(t, env, "u1", "t01-token-rotation", "Opaque input", "web")
	scope := ownerScope(1, "u1", ws.SessionID)
	ctx := craftCtx(1, "u1", ws.SessionID)
	inputs, err := env.svc.AcceptInputRound(ctx, scope, []CraftInputUpload{{Name: "ledger.mystery", Content: []byte("opaque payload"), SHA256: sha256Sum("opaque payload")}})
	require.NoError(t, err)
	require.NoError(t, env.svc.DecideInput(ctx, scope, inputs[0].Ref, "continue"))
	session, err := env.svc.writeSession(ctx, scope, ws.SessionID)
	require.NoError(t, err)
	selected, err := env.svc.resolveInputRefs(ctx, ws.ID, []string{inputs[0].Ref})
	require.NoError(t, err)
	const requestID = "craft-token-recovery"
	const runID = "paused-original-run"
	oldClaims, _, err := env.svc.claimInputDecisions(ctx, session, selected, requestID, runID, "u1")
	require.NoError(t, err)
	require.Len(t, oldClaims, 1)
	oldToken := oldClaims[0].Token
	key := craftInputDecisionKey(inputs[0].Ref)
	require.NoError(t, env.db.Table("craft_session_requests").Where(
		"tenant_id = ? AND user_id = ? AND session_id = ? AND purpose = ? AND request_id = ?",
		1, "u1", ws.SessionID, "input_admission", key,
	).Update("lease_expires_at", time.Now().Add(-time.Minute)).Error)

	// A fresh service instance finds the expired claim, retains the immutable
	// Run identity and rotates only the fencing token on a separate connection.
	dsn := env.db.Dialector.(*sqlite.Dialector).DSN
	recoveryDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	recoveryConn, err := recoveryDB.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = recoveryConn.Close() })
	recoveryRuns := NewAgentRunService(repository.NewAgentRunStore(recoveryDB))
	restarted, err := NewCraftSessionService(CraftSessionConfig{
		DB: recoveryDB, Sessions: env.sessions, Store: env.store, Versions: env.versions,
		Runs: recoveryRuns, ActiveRuns: CraftActiveRunsQuery(recoveryDB), TemporaryDocs: env.docs,
		Files: env.svc.files, Models: env.models, Gate: openGate,
	})
	require.NoError(t, err)
	newClaims, recoveredRunID, err := restarted.claimInputDecisions(ctx, session, selected, requestID, "new-random-proposal", "u1")
	require.NoError(t, err)
	require.Equal(t, runID, recoveredRunID)
	require.Len(t, newClaims, 1)
	require.NotEqual(t, oldToken, newClaims[0].Token)
	require.Equal(t, runID, newClaims[0].RunID)

	// Claimant A was paused before Submit and still holds its old token. Its
	// late Submit must fail before creating any durable Run. The snapshot must
	// mirror the production Craft admission contract: Submit rejects a
	// registered Craft Task admission whose snapshot carries no input manifest.
	manifest, err := json.Marshal([]craft.Input{inputs[0]})
	require.NoError(t, err)
	pausedSnapshot, err := json.Marshal(map[string]json.RawMessage{
		"version":              json.RawMessage(`1`),
		"craft_input_manifest": manifest,
	})
	require.NoError(t, err)
	pausedAdmission := agentruntime.Admission{
		Key:                agentruntime.RunKey{TenantID: 1, RunID: runID},
		SessionID:          ws.SessionID,
		UserID:             "u1",
		ActorUserID:        "u1",
		RequestID:          requestID,
		AssistantMessageID: "paused-assistant",
		RequestHash:        "paused-request-hash",
		Snapshot:           pausedSnapshot,
		UserMessage:        json.RawMessage(`{"role":"user","content":"build"}`),
		AssistantMessage:   json.RawMessage(`{"role":"assistant","content":""}`),
		Deadline:           time.Now().Add(time.Hour),
		InputClaims: []agentruntime.InputAdmissionClaim{{
			DecisionKey: key, RunID: runID, Token: oldToken,
		}},
	}
	_, err = env.runs.Submit(ctx, pausedAdmission)
	require.ErrorIs(t, err, agentruntime.ErrConflict)
	var runs int64
	require.NoError(t, env.db.Table("agent_runs").Where("tenant_id = ? AND request_id = ?", 1, requestID).Count(&runs).Error)
	require.Zero(t, runs)

	// Claimant B's current token admits the same immutable Run identity.
	pausedAdmission.InputClaims[0].Token = newClaims[0].Token
	accepted, err := recoveryRuns.Submit(ctx, pausedAdmission)
	require.NoError(t, err)
	require.Equal(t, runID, accepted.Key.RunID)
	require.NoError(t, env.db.Table("agent_runs").Where("tenant_id = ? AND request_id = ?", 1, requestID).Count(&runs).Error)
	require.EqualValues(t, 1, runs)
	var state string
	require.NoError(t, env.db.Table("craft_session_requests").Select("admission_state").Where(
		"tenant_id = ? AND user_id = ? AND session_id = ? AND purpose = ? AND request_id = ?",
		1, "u1", ws.SessionID, "input_admission", key,
	).Scan(&state).Error)
	require.Equal(t, "admitted", state)
}

func TestCraftT01QuotaAndBinary(t *testing.T) {
	env := newCraftSessionEnv(t, openGate)
	files := &craftT01Files{blobs: map[string][]byte{}}
	env.svc.files = files
	ws := createCraftSession(t, env, "u1", "t01-quotas", "Quota", "web")
	scope := ownerScope(1, "u1", ws.SessionID)
	ctx := craftCtx(1, "u1", ws.SessionID)
	makeUpload := func(name string, content []byte) CraftInputUpload {
		sum := sha256.Sum256(content)
		return CraftInputUpload{Name: name, Content: content, SHA256: hex.EncodeToString(sum[:])}
	}
	_, err := env.svc.AcceptInputRound(ctx, scope, nil)
	require.ErrorIs(t, err, craft.ErrInvalidInput)
	_, err = env.svc.AcceptInputRound(ctx, scope, []CraftInputUpload{makeUpload("oversize.bin", make([]byte, craft.MaxInputBytes+1))})
	require.ErrorIs(t, err, craft.ErrInvalidInput)
	many := make([]CraftInputUpload, craft.MaxInputsPerRound+1)
	for i := range many {
		many[i] = makeUpload("a.bin", []byte("x"))
	}
	_, err = env.svc.AcceptInputRound(ctx, scope, many)
	require.ErrorIs(t, err, craft.ErrInvalidInput)
	chunk := make([]byte, 18<<20)
	total := make([]CraftInputUpload, 6)
	for i := range total {
		total[i] = makeUpload("total.bin", chunk)
	}
	_, err = env.svc.AcceptInputRound(ctx, scope, total)
	require.ErrorIs(t, err, craft.ErrInvalidInput)
	for _, name := range []string{"", ".", "..", "../escape", "/absolute", `a\b`, "a/b"} {
		_, err = env.svc.AcceptInputRound(ctx, scope, []CraftInputUpload{makeUpload(name, []byte("x"))})
		require.ErrorIs(t, err, craft.ErrInvalidInput, name)
	}
	require.Empty(t, files.blobs, "invalid rounds must not write stored objects")
	inputs, err := env.svc.AcceptInputRound(ctx, scope, []CraftInputUpload{makeUpload("binary.txt", append([]byte("MZ"), []byte(strings.Repeat("x", 10))...))})
	require.NoError(t, err)
	require.Equal(t, "executable_binary", inputs[0].Recognition.Reason)
	require.False(t, inputs[0].Recognition.Understood)
}

func TestCraftT01RecognitionRequiresParsing(t *testing.T) {
	malformed := recognizeCraftInput("broken.json", []byte(`{"ok":`))
	require.True(t, malformed.Accepted)
	require.False(t, malformed.Understood, "a known extension must not stand in for successful parsing")
	require.NotEmpty(t, malformed.Reason)

	valid := recognizeCraftInput("valid.json", []byte(`{"ok":true}`))
	require.True(t, valid.Accepted)
	require.True(t, valid.Understood, "valid bounded JSON is parsed by the supported consumer")
}

func TestCraftT01InputRoundRollsBackRowsAndPreservesExistingBlob(t *testing.T) {
	env := newCraftSessionEnv(t, openGate)
	files := &craftT01Files{blobs: map[string][]byte{}}
	env.svc.files = files
	ws := createCraftSession(t, env, "u1", "t01-rollback", "Rollback", "web")
	scope := ownerScope(1, "u1", ws.SessionID)
	ctx := craftCtx(1, "u1", ws.SessionID)
	upload := func(name, body string) CraftInputUpload {
		return CraftInputUpload{Name: name, Content: []byte(body), SHA256: sha256Sum(body)}
	}
	first := upload("first.txt", "already-associated")
	accepted, err := env.svc.AcceptInputRound(ctx, scope, []CraftInputUpload{first})
	require.NoError(t, err)
	firstRef := accepted[0].Ref

	creates := 0
	callbackName := "test:fail-second-craft-input-create"
	require.NoError(t, env.db.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Table == "craft_workspace_inputs" {
			creates++
			if creates == 2 {
				tx.AddError(errors.New("injected second-row failure"))
			}
		}
	}))
	callbackActive := true
	t.Cleanup(func() {
		if callbackActive {
			_ = env.db.Callback().Create().Remove(callbackName)
		}
	})
	second := upload("second.bin", "new-unassociated")
	_, err = env.svc.AcceptInputRound(ctx, scope, []CraftInputUpload{first, second})
	require.ErrorContains(t, err, "injected second-row failure")

	rows, err := env.svc.WorkspaceInputs(ctx, scope)
	require.NoError(t, err)
	require.Len(t, rows, 1, "the failed transaction must not leave a partial input row")
	require.Equal(t, firstRef, rows[0].Ref)
	require.Contains(t, files.blobs, firstRef, "a pre-existing association must protect its object from cleanup")
	require.NotContains(t, files.blobs, "craft-test://craft_input_"+second.SHA256+".bin", "an unreferenced object from the failed round should be cleaned up")
	require.NoError(t, env.db.Callback().Create().Remove(callbackName))
	callbackActive = false
	retried, err := env.svc.AcceptInputRound(ctx, scope, []CraftInputUpload{first, second})
	require.NoError(t, err, "retrying the complete round after an insert failure must succeed")
	require.Len(t, retried, 2)
	rows, err = env.svc.WorkspaceInputs(ctx, scope)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.Contains(t, files.blobs, "craft-test://craft_input_"+second.SHA256+".bin")
}

func TestCraftT01CancelCannotReplaceDecisionDuringRunAdmission(t *testing.T) {
	env := newCraftSessionEnv(t, openGate)
	env.svc.files = &craftT01Files{blobs: map[string][]byte{}}
	ws := createCraftSession(t, env, "u1", "t01-admission-fence", "Fence", "web")
	scope := ownerScope(1, "u1", ws.SessionID)
	ctx := craftCtx(1, "u1", ws.SessionID)
	input, err := env.svc.AcceptInputRound(ctx, scope, []CraftInputUpload{{Name: "unknown.bin", Content: []byte("opaque"), SHA256: sha256Sum("opaque")}})
	require.NoError(t, err)
	require.NoError(t, env.svc.DecideInput(ctx, scope, input[0].Ref, "continue"))
	session, err := env.svc.writeSession(ctx, scope, ws.SessionID)
	require.NoError(t, err)
	requestID := "craft-run-fence"
	_, _, err = env.svc.claimInputDecisions(ctx, session, input, requestID, "uncommitted-run", "u1")
	require.NoError(t, err)
	require.ErrorIs(t, env.svc.DecideInput(ctx, scope, input[0].Ref, "cancel"), craft.ErrConflict,
		"cancel must not be acknowledged while Submit could still admit the claimed continue")
	require.NoError(t, env.db.Table("craft_session_requests").Where(
		"tenant_id = ? AND user_id = ? AND session_id = ? AND purpose = ? AND request_id = ?",
		1, "u1", ws.SessionID, "input_admission", craftInputDecisionKey(input[0].Ref),
	).Update("lease_expires_at", time.Now().Add(-time.Minute)).Error)
	require.NoError(t, env.svc.DecideInput(ctx, scope, input[0].Ref, "cancel"))
	_, _, err = env.svc.StartRun(ctx, scope, CraftRunRequest{RequestID: "cancelled", Prompt: "build", InputRefs: []string{input[0].Ref}})
	require.ErrorIs(t, err, craft.ErrConflict)
	var count int64
	require.NoError(t, env.db.Table("agent_runs").Where("session_id = ?", ws.SessionID).Count(&count).Error)
	require.Zero(t, count)
}

type craftT01Files struct {
	*fakeCraftFiles
	blobs map[string][]byte
}

func (f *craftT01Files) SaveBytes(_ context.Context, data []byte, _ uint64, name string, _ bool) (string, error) {
	ref := "craft-test://" + name
	f.blobs[ref] = append([]byte(nil), data...)
	return ref, nil
}

func (f *craftT01Files) GetFile(_ context.Context, ref string) (io.ReadCloser, error) {
	return io.NopCloser(&craftT01Reader{data: f.blobs[ref]}), nil
}

func (f *craftT01Files) DeleteFile(_ context.Context, ref string) error {
	delete(f.blobs, ref)
	return nil
}

type craftT01Reader struct{ data []byte }

func (r *craftT01Reader) Read(dst []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	n := copy(dst, r.data)
	r.data = r.data[n:]
	return n, nil
}
