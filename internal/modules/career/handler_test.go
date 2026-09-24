package career

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCareerTenantRequiresSingleActiveOwner(t *testing.T) {
	owner := &types.TenantMember{UserID: "u1", TenantID: 7, Role: types.TenantRoleOwner}
	require.NoError(t, validateOwnerOnlyCareerTenant("u1", 7, []*types.TenantMember{owner}))
	require.ErrorIs(t, validateOwnerOnlyCareerTenant("u1", 7, []*types.TenantMember{owner, {UserID: "u2", TenantID: 7, Role: types.TenantRoleViewer}}), ErrUnauthorized)
	require.ErrorIs(t, validateOwnerOnlyCareerTenant("u2", 7, []*types.TenantMember{owner}), ErrUnauthorized)
	require.ErrorIs(t, validateOwnerOnlyCareerTenant("u1", 7, []*types.TenantMember{{UserID: "u1", TenantID: 7, Role: types.TenantRoleAdmin}}), ErrUnauthorized)
	require.ErrorIs(t, validateOwnerOnlyCareerTenant("u1", 7, nil), ErrUnauthorized)
}

func TestCareerActRejectsClientClaimedResumeExtractionSource(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	office, err := NewOffice(db)
	require.NoError(t, err)
	scope := Scope{UserID: "u1", TenantID: 7}
	baseCtx := context.WithValue(context.Background(), types.UserIDContextKey, scope.UserID)
	baseCtx = context.WithValue(baseCtx, types.TenantIDContextKey, scope.TenantID)
	require.NoError(t, office.ClaimSpace(WithScope(baseCtx, scope)))
	h := &Handler{office: office, members: &memberListStub{members: []*types.TenantMember{{UserID: "u1", TenantID: 7, Role: types.TenantRoleOwner}}}}
	requestBody := `{"action":"propose","key":"education.school","value":"Forged School","requestId":"forged-1","expectedRevision":0,"source":{"kind":"resume_extraction","referenceId":"client-controlled"}}`
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("POST", "/api/v1/career/act", strings.NewReader(requestBody)).WithContext(baseCtx)
	h.Act(c)
	require.Equal(t, 400, recorder.Code)
	view, err := office.Open(WithScope(baseCtx, scope))
	require.NoError(t, err)
	require.Empty(t, view.Proposals)
	confirmBody := `{"action":"confirm","key":"education.school","value":"Forged School","requestId":"forged-confirm-1","expectedRevision":0,"source":{"kind":"resume_extraction","referenceId":"client-controlled"}}`
	recorder = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("POST", "/api/v1/career/act", strings.NewReader(confirmBody)).WithContext(baseCtx)
	h.Act(c)
	require.Equal(t, 400, recorder.Code)
	view, err = office.Open(WithScope(baseCtx, scope))
	require.NoError(t, err)
	require.Empty(t, view.Facts)
}

func TestCareerActAcceptsT03UserSourceAliasForEveryAction(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	office, err := NewOffice(db)
	require.NoError(t, err)
	scope := Scope{UserID: "u1", TenantID: 7}
	base := context.WithValue(context.Background(), types.UserIDContextKey, scope.UserID)
	base = context.WithValue(base, types.TenantIDContextKey, scope.TenantID)
	ctx := WithScope(base, scope)
	require.NoError(t, office.ClaimSpace(ctx))
	h := &Handler{office: office, members: &memberListStub{members: []*types.TenantMember{{UserID: "u1", TenantID: 7, Role: types.TenantRoleOwner}}}}
	act := func(body string) (int, []byte) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest("POST", "/api/v1/career/act", strings.NewReader(body)).WithContext(base)
		h.Act(c)
		return rec.Code, rec.Body.Bytes()
	}
	status, body := act(`{"action":"propose","key":"education.school","value":"Example U","requestId":"alias-propose","expectedRevision":0,"source":{"kind":"user"}}`)
	require.Equal(t, 200, status)
	var proposed Receipt
	require.NoError(t, json.Unmarshal(body, &proposed))
	require.NotNil(t, proposed.Proposal)
	status, _ = act(`{"action":"confirm","key":"skill.go","value":"Go","requestId":"alias-confirm","expectedRevision":1,"source":{"kind":"user"}}`)
	require.Equal(t, 200, status)
	status, body = act(`{"action":"propose","key":"project.name","value":"Compiler","requestId":"alias-propose-2","expectedRevision":2,"source":{"kind":"user"}}`)
	require.Equal(t, 200, status)
	var second Receipt
	require.NoError(t, json.Unmarshal(body, &second))
	status, _ = act(fmt.Sprintf(`{"action":"confirm_proposal","proposalId":%q,"requestId":"alias-resolve","expectedRevision":3,"source":{"kind":"user"}}`, second.Proposal.ID))
	require.Equal(t, 200, status)
	status, body = act(`{"action":"propose","key":"certificate.name","value":"Cloud","requestId":"alias-propose-3","expectedRevision":4,"source":{"kind":"user"}}`)
	require.Equal(t, 200, status)
	var third Receipt
	require.NoError(t, json.Unmarshal(body, &third))
	status, _ = act(fmt.Sprintf(`{"action":"dismiss","proposalId":%q,"requestId":"alias-dismiss","expectedRevision":5,"source":{"kind":"user"}}`, third.Proposal.ID))
	require.Equal(t, 200, status)
}

type memberListStub struct{ members []*types.TenantMember }

func (s *memberListStub) ListByTenant(context.Context, uint64) ([]*types.TenantMember, error) {
	return s.members, nil
}
func TestCareerHTTPRechecksTenantMembershipOnEveryRequest(t *testing.T) {
	db, e := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, e)
	o, e := NewOffice(db)
	require.NoError(t, e)
	members := &memberListStub{members: []*types.TenantMember{{UserID: "u1", TenantID: 7, Role: types.TenantRoleOwner}}}
	h := &Handler{office: o, members: members}
	request := func() *httptest.ResponseRecorder {
		gin.SetMode(gin.TestMode)
		rec := httptest.NewRecorder()
		ctx := context.WithValue(context.Background(), types.UserIDContextKey, "u1")
		ctx = context.WithValue(ctx, types.TenantIDContextKey, uint64(7))
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest("GET", "/api/v1/career/open", nil).WithContext(ctx)
		h.Open(c)
		return rec
	}
	require.Equal(t, 200, request().Code)
	members.members = append(members.members, &types.TenantMember{UserID: "u2", TenantID: 7, Role: types.TenantRoleViewer})
	require.Equal(t, 403, request().Code)
	members.members = nil
	require.Equal(t, 403, request().Code)
}

func TestCareerApplicationHandlerMapsLinkOutcomes(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	office, err := NewOffice(db)
	require.NoError(t, err)
	scope := Scope{UserID: "u1", TenantID: 7}
	base := context.WithValue(context.Background(), types.UserIDContextKey, scope.UserID)
	base = context.WithValue(base, types.TenantIDContextKey, scope.TenantID)
	ctx := WithScope(base, scope)
	require.NoError(t, office.ClaimSpace(ctx))

	seed := seedApplicationEvaluation(t, office, ctx, "仅限2027届。", "2026", "handler")
	linker := &fakeCareerApplicationLinker{}
	office.SetApplicationTaskLinker(linker)
	h := &Handler{office: office, members: &memberListStub{members: []*types.TenantMember{{UserID: "u1", TenantID: 7, Role: types.TenantRoleOwner}}}}

	postApplication := func(body string) (int, string) {
		gin.SetMode(gin.TestMode)
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/career/applications", strings.NewReader(body)).WithContext(base)
		h.CreateApplication(c)
		return rec.Code, rec.Body.String()
	}
	receiptRequest := func(requestID string) (int, string) {
		gin.SetMode(gin.TestMode)
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/career/applications/receipt?requestId="+url.QueryEscape(requestID), nil).WithContext(base)
		h.ApplicationReceipt(c)
		return rec.Code, rec.Body.String()
	}

	// A hard-ineligible evaluation without explicit continuation maps to 409.
	body, _ := json.Marshal(applicationInput(seed, "handler-1", "batch-a"))
	status, payload := postApplication(string(body))
	require.Equal(t, 409, status)
	require.Contains(t, payload, "hard_ineligible_requires_continue")

	// An unknown linker outcome maps to 504 with the original request id.
	unknownSeed := seedApplicationEvaluation(t, office, ctx, "仅限2029届。", "2029", "handler-unknown")
	linker.ensureErr = context.Canceled
	unknownBody, _ := json.Marshal(applicationInput(unknownSeed, "handler-2", "batch-b"))
	status, payload = postApplication(string(unknownBody))
	require.Equal(t, 504, status)
	require.Contains(t, payload, "outcome_unknown")
	require.Contains(t, payload, `"requestId":"handler-2"`)

	// A receipt for an unknown request maps to 404.
	status, payload = receiptRequest("handler-missing")
	require.Equal(t, 404, status)
	require.Contains(t, payload, "not_found")
}
