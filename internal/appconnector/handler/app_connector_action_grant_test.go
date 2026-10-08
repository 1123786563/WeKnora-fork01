package handler

// Personal-connection and approval gates (T12 #42): a collaborator (or any
// other member) may neither USE the owner's personal connection to prepare
// an action nor APPROVE an action prepared by the owner. Space connections
// keep their existing shape; tenant owner/admin keep approval authority
// (authorized actor). Runs on the real OC engine fixture: sqlite + real
// routes + real action service.

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrepareActionRejectsAnotherMembersPersonalConnection(t *testing.T) {
	// One engine for the whole scenario: the fixture seeds each row exactly
	// once, so the env must not be rebuilt between requests.
	env := newOCProductEngine(t)
	prepare := func(user, connectionID string) (*httptest.ResponseRecorder, string) {
		body := fmt.Sprintf(`{"connection_id":%q,"target":"t1","risk":"write","content":"{\"target\":\"t1\"}"}`, connectionID)
		w := ocDo(t, env, http.MethodPost, "/api/v1/apps/actions/prepare", body, "X-Test-User", user)
		return w, w.Body.String()
	}

	// conn-gh is user-a's PERSONAL connection. user-b (an admin in the test
	// fixture — the strongest non-owner case) is refused.
	w, body := prepare("user-b", "conn-gh")
	require.Equal(t, http.StatusForbidden, w.Code, body)
	require.Contains(t, body, "NOT_CONNECTION_OWNER")

	// The connection owner still prepares fine.
	w, body = prepare("user-a", "conn-gh")
	require.Equal(t, http.StatusCreated, w.Code, body)

	// A SPACE connection (conn-gl, owner_id='') is NOT affected: any
	// action-write-capable role keeps the existing shape.
	w, body = prepare("user-b", "conn-gl")
	require.Equal(t, http.StatusCreated, w.Code, body)
}

func TestApproveActionRejectsCollaboratorAndAdmitsOwner(t *testing.T) {
	env := newOCProductEngine(t)
	// user-a prepares on their personal connection.
	w := ocDo(t, env, http.MethodPost, "/api/v1/apps/actions/prepare",
		`{"connection_id":"conn-gh","target":"t1","risk":"write","content":"{\"target\":\"t1\"}"}`,
		"X-Test-User", "user-a")
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	id, digest, _, _, version := ocActionDetail(t, w.Body.String())

	approveBody := fmt.Sprintf(`{"digest":%q,"expected_version":%d}`, digest, int(version))

	// A different member in the collaborator shape (contributor role) cannot
	// approve the owner's action — sharing never delegates side-effect
	// approval (AC1).
	w = ocDo(t, env, http.MethodPost, "/api/v1/apps/actions/"+id+"/approve", approveBody,
		"X-Test-User", "user-b", "X-Test-Role", "contributor")
	require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "ACTION_APPROVAL_FORBIDDEN")

	// The actor (owner) approves their own action.
	w = ocDo(t, env, http.MethodPost, "/api/v1/apps/actions/"+id+"/approve", approveBody,
		"X-Test-User", "user-a", "X-Test-Role", "contributor")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	// A tenant admin keeps the authorized-actor approval arm (existing
	// capability preserved).
	w = ocDo(t, env, http.MethodPost, "/api/v1/apps/actions/prepare",
		`{"connection_id":"conn-gh","target":"t2","risk":"send","content":"{\"target\":\"t2\"}"}`,
		"X-Test-User", "user-a")
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	id2, digest2, _, _, version2 := ocActionDetail(t, w.Body.String())
	w = ocDo(t, env, http.MethodPost, "/api/v1/apps/actions/"+id2+"/approve",
		fmt.Sprintf(`{"digest":%q,"expected_version":%d}`, digest2, int(version2)),
		"X-Test-User", "user-b", "X-Test-Role", "admin")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
}

// TestApproveActionRejectsViewerOnOthersAction pins the default-deny path of
// the approval predicate (ruling via escalation, condition 3): a viewer
// satisfies none of the three arms — not the initiator, not the personal
// connection's owner, not CanDriveActionWrites — and must land on
// ACTION_APPROVAL_FORBIDDEN, never on a silent allow.
func TestApproveActionRejectsViewerOnOthersAction(t *testing.T) {
	env := newOCProductEngine(t)
	w := ocDo(t, env, http.MethodPost, "/api/v1/apps/actions/prepare",
		`{"connection_id":"conn-gh","target":"t1","risk":"write","content":"{\"target\":\"t1\"}"}`,
		"X-Test-User", "user-a")
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	id, digest, _, _, version := ocActionDetail(t, w.Body.String())

	w = ocDo(t, env, http.MethodPost, "/api/v1/apps/actions/"+id+"/approve",
		fmt.Sprintf(`{"digest":%q,"expected_version":%d}`, digest, int(version)),
		"X-Test-User", "user-b", "X-Test-Role", "viewer")
	require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "ACTION_APPROVAL_FORBIDDEN")
}
