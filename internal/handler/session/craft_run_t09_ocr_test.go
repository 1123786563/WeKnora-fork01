package session

// T09 (#135) OCR round-1 regression: the run-response envelope carries the
// wire the edit panel projects — the snake_case data.run_id run view, the
// T00 frozen writer_acquisition outcome as a bare string, and the ADDITIVE
// initiated_by envelope field echoing the admitted run's durable actor (the
// ACTUAL initiating member). runView itself stays the frozen DTO projection
// without actor identity.
import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Tencent/WeKnora/internal/application/service"
)

func TestCraftHTTPRunEnvelopeProjectsInitiatorAndWireNaming(t *testing.T) {
	env := newCraftHTTPEnv(t, service.CraftFeatureGate{Enabled: true, Kinds: []string{"web"}})
	created := env.createSession(t, "key-t09ocr", "修改请求面板对接", "web")
	sessionID := created["session_id"].(string)

	w := env.do(t, http.MethodPost, "/api/v1/sessions/"+sessionID+"/craft/runs", "",
		`{"request_id":"run-t09ocr","prompt":"把标题改成蓝色"}`)
	require.Equal(t, http.StatusAccepted, w.Code, "run body: %s", w.Body.String())

	var body struct {
		Data              map[string]any `json:"data"`
		WriterAcquisition struct {
			WorkspaceID string `json:"workspace_id"`
			Status      string `json:"status"`
		} `json:"writer_acquisition"`
		InitiatedBy string `json:"initiated_by"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.NotEmpty(t, body.Data["run_id"], "the run id projects under the frozen snake_case wire name")
	// The shared env wires no writer-lease store: the nil seam fails closed
	// to the frozen "unknown" outcome — still the {workspace_id, status}
	// OBJECT shape, never a bare string (craft_session.go:932-942).
	require.Equal(t, "unknown", body.WriterAcquisition.Status,
		"the T00 frozen writer-acquisition outcome projects as the frozen {workspace_id, status} object")
	require.NotEmpty(t, body.WriterAcquisition.WorkspaceID,
		"the acquisition object names the workspace it serialized")
	require.Equal(t, "u1", body.InitiatedBy,
		"the envelope echoes the admitted run's durable actor — the actual initiating member")
	// runView stays the frozen craft DTO projection: actor identity lives in
	// the envelope only, never inside the data payload.
	require.NotContains(t, body.Data, "actor_user_id")
	require.NotContains(t, body.Data, "initiated_by")
}
