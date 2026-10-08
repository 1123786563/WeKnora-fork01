package career

import (
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestGoWireMatchesSharedFixtures(t *testing.T) {
	data, e := os.ReadFile("../../packages/career-core/testdata/wire-fixtures.json")
	require.NoError(t, e)
	var fixtures map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &fixtures))
	created := time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC)
	confirmed := time.Date(2026, 9, 24, 0, 0, 1, 0, time.UTC)
	source := Source{Kind: "resume_extraction", ReferenceID: "file-1"}
	pending := Proposal{ID: "proposal-1", Key: "graduation_year", Value: "2027", Source: source, Status: "pending", CreatedAt: created}
	proposed, _ := json.Marshal(Receipt{Kind: "proposed", RequestID: "p-1", Revision: 1, Proposal: &pending})
	require.JSONEq(t, string(fixtures["proposed"]), string(proposed))
	done := pending
	done.Status = "confirmed"
	rev := uint64(2)
	done.Revision = &rev
	confirmation := Confirmation{UserID: "u1", ConfirmedAt: confirmed}
	resolutionSource := Source{Kind: "user_confirmation"}
	done.Confirmation = &confirmation
	done.ResolutionSource = &resolutionSource
	fact := Fact{Key: "graduation_year", Value: "2027", Revision: 2, Source: source, Confirmation: Confirmation{UserID: "u1", ConfirmedAt: confirmed}, ConfirmedAt: confirmed}
	confirmedJSON, _ := json.Marshal(Receipt{Kind: "confirmed", RequestID: "c-1", Revision: 2, Proposal: &done, Fact: &fact})
	require.JSONEq(t, string(fixtures["confirmed"]), string(confirmedJSON))
}
func TestHTTPErrorBodiesMatchSharedFixtures(t *testing.T) {
	data, e := os.ReadFile("../../packages/career-core/testdata/wire-fixtures.json")
	require.NoError(t, e)
	var fixtures map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &fixtures))
	cases := []struct {
		name    string
		err     error
		fixture string
		status  int
	}{{"missing", ErrReceiptNotFound, "not_found", 404}, {"conflict", &RevisionConflictError{CurrentRevision: 2}, "revision_conflict", 409}, {"unknown", &OutcomeUnknownError{RequestID: "same"}, "outcome_unknown", 504}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			writeError(c, tc.err)
			require.JSONEq(t, string(fixtures[tc.fixture]), w.Body.String())
			require.Equal(t, tc.status, w.Code)
		})
	}
}

func TestHTTPUnknownErrorDoesNotExposeInternalDetails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	writeError(c, errors.New("storage bucket private-prod at internal.example:9000 failed"))
	require.Equal(t, 500, w.Code)
	require.Contains(t, w.Body.String(), `"code":"internal"`)
	require.Contains(t, w.Body.String(), `"message":"internal career office error"`)
	require.NotContains(t, w.Body.String(), "private-prod")
	require.NotContains(t, w.Body.String(), "internal.example")
}
