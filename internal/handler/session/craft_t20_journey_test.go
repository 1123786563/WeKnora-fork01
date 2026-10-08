package session

// TestCraftT20Journey is the integrated web-artifact journey at the highest
// available Craft API seam (the authenticated HTTP surface over the real
// service + migrated database): the owner creates a private web Task,
// associates a CSV material, expands a bounded archive into immutable
// members, submits a Run and reconnects without duplicate submission, makes
// a second edit request, and the budget boundary parks the Run durably
// until the owner's extension resumes it. Authorization legs prove the
// tenant fence and the Task gate on the same spine. Assertions target
// persisted state and externally visible responses only.

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/commercial"
	repocommercial "github.com/Tencent/WeKnora/internal/commercial/repository/commercial"
	commercialsvc "github.com/Tencent/WeKnora/internal/commercial/service/commercial"
	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

// t20JourneyZip builds one real bounded archive holding a region CSV.
func t20JourneyZip(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	file, err := writer.Create("region-sales.csv")
	require.NoError(t, err)
	_, err = file.Write([]byte("region,revenue\nnorth,1200\nsouth,800\n"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	return buf.Bytes()
}

func TestCraftT20Journey(t *testing.T) {
	// The budget-pause service registers before route assembly (container
	// ordering); the concrete service lands once the env database exists.
	lazy := &t20LazyPauseAPI{}
	RegisterCraftBudgetPauseHandler(lazy, t20BudgetChecker{})
	env := newCraftHTTPEnv(t, service.CraftFeatureGate{Enabled: true, Kinds: []string{"web"}})

	// --- Leg A: a private web Task with CSV material + a bounded archive ---
	created := env.createSession(t, "key-t20-journey", "区域销售网页", "web")
	sessionID := created["session_id"].(string)
	require.NotEmpty(t, created["workspace_id"], "the workspace exists at creation")

	// Private by default: a foreign tenant never sees the Task (story 28).
	require.Equal(t, http.StatusNotFound, env.do(t, http.MethodGet,
		"/api/v1/sessions/"+sessionID+"/craft", "foreigntenant", "").Code)

	// CSV material through the existing session entrance (story 3).
	csvContent := "region,revenue\nnorth,1200\nsouth,800\n"
	env.addUpload(t, sessionID, "doc-csv", types.TemporaryDocumentStatusReady, csvContent)
	w := env.do(t, http.MethodPost, "/api/v1/sessions/"+sessionID+"/craft/inputs", "",
		fmt.Sprintf("{\"resource_ref\":\"doc-csv\",\"expected_sha256\":%q}", craftDigest(csvContent)))
	require.Equal(t, http.StatusCreated, w.Code, "csv input body: %s", w.Body.String())

	// A bounded archive expands into immutable members through the guarded
	// T02 endpoint (story 8): all members publish at once.
	zipBytes := t20JourneyZip(t)
	env.addUpload(t, sessionID, "doc-zip", types.TemporaryDocumentStatusReady, string(zipBytes))
	// The expansion reads the archive bytes through the FILE service by the
	// associated input ref (the tempdocs fake only gates association).
	env.files.blobs["tempdocs://doc-zip"] = zipBytes
	w = env.do(t, http.MethodPost, "/api/v1/sessions/"+sessionID+"/craft/inputs", "",
		fmt.Sprintf("{\"resource_ref\":\"doc-zip\",\"expected_sha256\":%q}", craftDigest(string(zipBytes))))
	require.Equal(t, http.StatusCreated, w.Code, "archive input body: %s", w.Body.String())
	w = env.do(t, http.MethodPost, "/api/v1/sessions/"+sessionID+"/craft/inputs/expand", "",
		`{"resource_ref":"tempdocs://doc-zip"}`)
	require.Equal(t, http.StatusCreated, w.Code, "expand body: %s", w.Body.String())
	var expanded struct {
		Data []map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &expanded))
	require.Len(t, expanded.Data, 1, "the single archive member publishes")
	require.Equal(t, "region-sales.csv", expanded.Data[0]["name"])
	// Persisted state: the member is an immutable workspace input now.
	var members int64
	require.NoError(t, env.db.Table("craft_workspace_inputs").
		Where("name = ?", "region-sales.csv").Count(&members).Error)
	require.EqualValues(t, 1, members)

	// --- Leg B: Run admission + reconnect without duplicate submission (19) ---
	// The unknown-format disclosure gate (stories 5/6): the `.txt` material is
	// accepted but not understood, and the Run refuses to consume it until
	// the owner's explicit continue decision — the disclose-then-decide
	// contract, exercised through the same authenticated surface.
	decidePath := "/api/v1/sessions/" + sessionID + "/craft/inputs/decision"
	require.Equal(t, http.StatusConflict, env.do(t, http.MethodPost,
		"/api/v1/sessions/"+sessionID+"/craft/runs", "",
		`{"request_id":"run-t20-early","prompt":" premature","input_refs":["tempdocs://doc-csv"]}`).Code,
		"a Run cannot consume an undecided unknown input")
	w = env.do(t, http.MethodPost, decidePath, "", `{"ref":"tempdocs://doc-csv","action":"continue"}`)
	require.Equal(t, http.StatusOK, w.Code, "decision body: %s", w.Body.String())
	// Re-deciding continue is idempotent — the acknowledgement never
	// duplicates the submission side.
	require.Equal(t, http.StatusOK, env.do(t, http.MethodPost, decidePath, "",
		`{"ref":"tempdocs://doc-csv","action":"continue"}`).Code)

	runPath := "/api/v1/sessions/" + sessionID + "/craft/runs"
	submitBody := `{"request_id":"run-t20-a","prompt":"生成区域筛选销售网页","input_refs":["tempdocs://doc-csv"]}`
	w = env.do(t, http.MethodPost, runPath, "", submitBody)
	require.Equal(t, http.StatusAccepted, w.Code, "run body: %s", w.Body.String())
	var firstRun struct {
		Data              map[string]any `json:"data"`
		WriterAcquisition struct {
			WorkspaceID string `json:"workspace_id"`
			Status      string `json:"status"`
		} `json:"writer_acquisition"`
		InitiatedBy string `json:"initiated_by"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &firstRun))
	runA := firstRun.Data["run_id"].(string)
	require.NotEmpty(t, runA)
	require.Equal(t, "u1", firstRun.InitiatedBy, "the owner is the recorded initiating member")
	require.Contains(t, []string{"acquired", "unknown"}, firstRun.WriterAcquisition.Status,
		"the frozen writer-acquisition object stays a closed vocabulary")

	// The browser reconnects and replays the SAME immutable command: the
	// server replays admission — same Run, never a duplicate submission.
	w = env.do(t, http.MethodPost, runPath, "", submitBody)
	require.Equal(t, http.StatusAccepted, w.Code, "replay body: %s", w.Body.String())
	var replay struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &replay))
	require.Equal(t, runA, replay.Data["run_id"], "the same request_id replays the same Run")
	var runRows int64
	require.NoError(t, env.db.Table("agent_runs").
		Where("tenant_id = ? AND session_id = ?", uint64(1), sessionID).Count(&runRows).Error)
	require.EqualValues(t, 1, runRows, "exactly one Run exists for the replayed intent")

	// --- Leg C: the second edit request (15) on the same Task — the prompt
	// alone addresses the persistent Workspace (no re-upload, no re-decide).
	env.craftHTTPFinalize(t, runA)
	w = env.do(t, http.MethodPost, runPath, "",
		`{"request_id":"run-t20-b","prompt":"把华北区域高亮"}`)
	require.Equal(t, http.StatusAccepted, w.Code, "second edit body: %s", w.Body.String())
	var secondRun struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &secondRun))
	runB := secondRun.Data["run_id"].(string)
	require.NotEmpty(t, runB)
	require.NotEqual(t, runA, runB, "a NEW edit intent is a NEW Run in the same Task")

	// --- Leg D: the budget boundary parks durably and resumes on the owner's
	// extension (34/35) — no false delivery while paused. ---
	require.NoError(t, env.db.AutoMigrate(
		&repocommercial.BudgetAccountRow{}, &repocommercial.TaskBudgetRow{},
		&repocommercial.ReservationRow{}, &repocommercial.BudgetLotRow{},
		&repocommercial.BudgetLotAllocationRow{}, &repocommercial.TaskBudgetExtensionRow{},
		&commercialsvc.SettlementRecord{},
	))
	end := time.Now().UTC().Add(2 * time.Hour)
	require.NoError(t, env.db.Create(&repocommercial.BudgetAccountRow{
		TenantID: 1, VerifiedMicro: 100000, Watermark: "w0", Version: 1, VerifiedUntil: end,
	}).Error)
	require.NoError(t, env.db.Create(&repocommercial.BudgetLotRow{
		TenantID: 1, LotID: "lot-t20-journey", RemainingMicro: 100000, IssuedAt: time.Now().UTC(),
	}).Error)
	budget, err := service.NewCraftBudgetService(env.db, nil, service.CraftBudgetPolicy{
		GrantWindow: time.Hour, MaxCalls: 1, CallUpper: commercial.Credits(500), TaskLimit: commercial.Credits(5000),
	})
	require.NoError(t, err)
	lazy.inner = budget

	ctx := context.Background()
	ownerScope := craft.Scope{TenantID: 1, UserID: "u1", SessionID: sessionID}
	grant, err := budget.Admit(ctx, ownerScope, runB)
	require.NoError(t, err)
	_, err = budget.AuthorizeBinding(ctx, grant.ID, service.CraftCallBinding{ModelID: "m-chat", Funding: commercial.FundingPlatform})
	require.NoError(t, err)
	_, err = budget.AuthorizeBinding(ctx, grant.ID, service.CraftCallBinding{ModelID: "m-chat", Funding: commercial.FundingPlatform})
	require.ErrorIs(t, err, craft.ErrGrantExhausted, "the exhausted grant parks the Run durably")

	// Persisted pause: waiting_user / budget_exhausted, visible across a
	// refresh through the pause view (the owner's allowed action).
	var parked struct{ Status, WaitReason string }
	require.NoError(t, env.db.Table("agent_runs").Select("status, wait_reason").
		Where("tenant_id = ? AND run_id = ?", uint64(1), runB).Take(&parked).Error)
	require.Equal(t, "waiting_user", parked.Status)
	require.Equal(t, "budget_exhausted", parked.WaitReason)

	pausePath := "/api/v1/sessions/" + sessionID + "/craft/runs/" + runB + "/budget/pause"
	extendPath := "/api/v1/sessions/" + sessionID + "/craft/runs/" + runB + "/budget/extend"
	w = env.do(t, http.MethodGet, pausePath, "", "")
	require.Equal(t, http.StatusOK, w.Code, "pause body: %s", w.Body.String())
	var pauseView struct {
		Data struct {
			RunID           string `json:"run_id"`
			Reason          string `json:"reason"`
			Limit           int64  `json:"limit"`
			Used            int64  `json:"used"`
			ExtensionAction *struct {
				Key          string `json:"key"`
				ExtraCalls   int    `json:"extra_calls"`
				ExtraCredits int64  `json:"extra_credits"`
			} `json:"extension_action"`
		} `json:"data"`
		CanExtend bool `json:"can_extend"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &pauseView))
	require.Equal(t, runB, pauseView.Data.RunID)
	require.Equal(t, "exhausted", pauseView.Data.Reason)
	require.True(t, pauseView.CanExtend)
	require.NotNil(t, pauseView.Data.ExtensionAction)
	require.NotEmpty(t, pauseView.Data.ExtensionAction.Key)
	require.Equal(t, 10, pauseView.Data.ExtensionAction.ExtraCalls)
	actionBody, err := json.Marshal(pauseView.Data.ExtensionAction)
	require.NoError(t, err)

	// Authorization legs on the same spine: the Task gate refuses a
	// same-tenant non-member; the tenant fence stays 404 for the foreigner.
	require.Equal(t, http.StatusForbidden, env.do(t, http.MethodGet, pausePath, "nonmember", "").Code)
	require.Equal(t, http.StatusNotFound, env.do(t, http.MethodGet, pausePath, "foreigntenant", "").Code)

	// The unconfirmed dispatched effect is never replayed: extension refuses
	// with the reconcile-pending conflict while the reservation is open.
	require.Equal(t, http.StatusConflict, env.do(t, http.MethodPost, extendPath, "",
		string(actionBody)).Code)
	require.NoError(t, env.db.Model(&repocommercial.ReservationRow{}).
		Where("tenant_id = ?", uint64(1)).
		Update("state", commercial.ReservationStateSettled).Error)

	// The owner's extension resumes the Run durably; the pause view is gone.
	require.Equal(t, http.StatusOK, env.do(t, http.MethodPost, extendPath, "",
		string(actionBody)).Code)
	var resumed struct{ Status, WaitReason string }
	require.NoError(t, env.db.Table("agent_runs").Select("status, wait_reason").
		Where("tenant_id = ? AND run_id = ?", uint64(1), runB).Take(&resumed).Error)
	require.Equal(t, "recovering", resumed.Status)
	require.Empty(t, resumed.WaitReason)
	require.Equal(t, http.StatusNotFound, env.do(t, http.MethodGet, pausePath, "", "").Code)

	// A malformed extension never reaches the service.
	require.Equal(t, http.StatusBadRequest, env.do(t, http.MethodPost, extendPath, "", `{"key":""}`).Code)
}
