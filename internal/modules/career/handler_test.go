package career

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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

func TestCareerSearchHandlerMapsQuotaRefusalUnknownAndReceipt(t *testing.T) {
	o, _, gate, ctx := newSearchFixtureOffice(t)
	require.NoError(t, o.ClaimSpace(ctx))
	base := context.WithValue(context.Background(), types.UserIDContextKey, "owner")
	base = context.WithValue(base, types.TenantIDContextKey, uint64(91))
	h := &Handler{office: o, members: &memberListStub{members: []*types.TenantMember{{UserID: "owner", TenantID: 91, Role: types.TenantRoleOwner}}}}
	request := func(method, target, body string) *httptest.ResponseRecorder {
		gin.SetMode(gin.TestMode)
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		var reader io.Reader
		if body != "" {
			reader = strings.NewReader(body)
		}
		c.Request = httptest.NewRequest(method, target, reader).WithContext(base)
		if body != "" {
			c.Request.Header.Set("Content-Type", "application/json")
		}
		h.SearchOnce(c)
		return rec
	}

	// Quota refusal maps to a typed 429 that is recoverable by replay.
	gate.refused = true
	rec := request(http.MethodPost, "/api/v1/career/searches", `{"requestId":"search-http-quota","query":"go engineer","expectedRevision":0}`)
	require.Equal(t, 429, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "search_quota_refused")

	// A successful one-shot search returns the frozen receipt shape.
	gate.refused = false
	input, _ := json.Marshal(SearchOnceInput{RequestID: "search-http", Query: "go engineer", ExpectedRevision: 0})
	rec = request(http.MethodPost, "/api/v1/career/searches", string(input))
	require.Equal(t, 200, rec.Code, rec.Body.String())
	var receipt SearchOnceReceipt
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &receipt))
	require.Equal(t, "search_once", receipt.Kind)
	require.Equal(t, SearchStatusCompleted, receipt.Status)
	require.Len(t, receipt.Results, 2)

	// Malformed or trailing JSON is rejected.
	rec = request(http.MethodPost, "/api/v1/career/searches", `{"requestId":"search-http-bad","query":"go","expectedRevision":0,"extra":1}`)
	require.Equal(t, 400, rec.Code, rec.Body.String())

	// The receipt endpoint replays by request ID and 404s unknown requests.
	gin.SetMode(gin.TestMode)
	receiptRec := httptest.NewRecorder()
	receiptCtx, _ := gin.CreateTestContext(receiptRec)
	receiptCtx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/career/searches/receipt?requestId=search-http", nil).WithContext(base)
	h.SearchReceipt(receiptCtx)
	require.Equal(t, 200, receiptRec.Code, receiptRec.Body.String())
	require.Contains(t, receiptRec.Body.String(), `"kind":"search_once"`)

	gin.SetMode(gin.TestMode)
	missingRec := httptest.NewRecorder()
	missingCtx, _ := gin.CreateTestContext(missingRec)
	missingCtx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/career/searches/receipt?requestId=missing", nil).WithContext(base)
	h.SearchReceipt(missingCtx)
	require.Equal(t, 404, missingRec.Code)

	// The result read path serves the same stored receipt by search ID.
	gin.SetMode(gin.TestMode)
	getRec := httptest.NewRecorder()
	getCtx, _ := gin.CreateTestContext(getRec)
	getCtx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/career/searches/"+receipt.SearchID, nil).WithContext(base)
	getCtx.Params = gin.Params{{Key: "searchId", Value: receipt.SearchID}}
	h.GetSearch(getCtx)
	require.Equal(t, 200, getRec.Code, getRec.Body.String())
	require.Contains(t, getRec.Body.String(), `"results":`)

	// An intruder from another tenant is rejected at the scope gate.
	gin.SetMode(gin.TestMode)
	intruderRec := httptest.NewRecorder()
	intruderCtx, _ := gin.CreateTestContext(intruderRec)
	intruderBase := context.WithValue(context.Background(), types.UserIDContextKey, "intruder")
	intruderBase = context.WithValue(intruderBase, types.TenantIDContextKey, uint64(91))
	intruderCtx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/career/searches/"+receipt.SearchID, nil).WithContext(intruderBase)
	intruderCtx.Params = gin.Params{{Key: "searchId", Value: receipt.SearchID}}
	h.GetSearch(intruderCtx)
	require.Equal(t, 403, intruderRec.Code)
}

func TestCareerMaterialHTTPContract(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	office, err := NewOffice(db)
	require.NoError(t, err)
	scope := Scope{UserID: "material-owner", TenantID: 97}
	base := context.WithValue(context.Background(), types.UserIDContextKey, scope.UserID)
	base = context.WithValue(base, types.TenantIDContextKey, scope.TenantID)
	ctx := WithScope(base, scope)
	require.NoError(t, office.ClaimSpace(ctx))

	view, err := office.Open(ctx)
	require.NoError(t, err)
	_, err = office.Confirm(ctx, "education.graduation_year", "2027", "mat-http-year", view.Revision, Source{Kind: "manual"})
	require.NoError(t, err)
	view, err = office.Open(ctx)
	require.NoError(t, err)
	job, err := office.ImportJD(ctx, ImportJDInput{RequestID: "mat-http-job", RawText: "仅限2027届。"})
	require.NoError(t, err)

	h := &Handler{office: office, members: &memberListStub{members: []*types.TenantMember{{UserID: scope.UserID, TenantID: scope.TenantID, Role: types.TenantRoleOwner}}}}
	materialJSON := func(requestID string, claims string) string {
		return fmt.Sprintf(`{"requestId":%q,"opportunityId":%q,"snapshotId":%q,"expectedRevision":%d,"body":{"sections":[{"heading":"summary","content":"正文","claims":[%s]}]}}`,
			requestID, job.OpportunityID, job.SnapshotID, view.Revision, claims)
	}
	post := func(body string) *httptest.ResponseRecorder {
		gin.SetMode(gin.TestMode)
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/career/materials", strings.NewReader(body)).WithContext(base)
		h.EditMaterial(c)
		return rec
	}

	// A claim on an unconfirmed fact maps to the typed 409 contract.
	rec := post(materialJSON("mat-http-1", `{"claimId":"c1","text":"曾在某公司实习","factKey":"experience.internship"}`))
	require.Equal(t, 409, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "material_claim_unconfirmed")

	// A claim linked to a confirmed fact returns the frozen receipt shape.
	rec = post(materialJSON("mat-http-2", `{"claimId":"c1","text":"2027 届毕业生","factKey":"education.graduation_year"}`))
	require.Equal(t, 200, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), `"kind":"material_edited"`)
	require.Contains(t, rec.Body.String(), `"pinnedEvidence":`)
	var created MaterialReceipt
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	require.Equal(t, view.Revision, created.PinnedEvidence.ProfileRevision)

	// Confirm forms an immutable version through the HTTP seam.
	gin.SetMode(gin.TestMode)
	confirmRec := httptest.NewRecorder()
	confirmCtx, _ := gin.CreateTestContext(confirmRec)
	confirmCtx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/career/materials/confirm", strings.NewReader(fmt.Sprintf(`{"requestId":"mat-http-confirm","materialId":%q,"expectedRevision":%d}`, created.MaterialID, view.Revision))).WithContext(base)
	h.ConfirmMaterialBody(confirmCtx)
	require.Equal(t, 200, confirmRec.Code, confirmRec.Body.String())
	require.Contains(t, confirmRec.Body.String(), `"kind":"material_confirmed"`)
	require.Contains(t, confirmRec.Body.String(), `"version":1`)

	// Version reads and compares serve the immutable history.
	gin.SetMode(gin.TestMode)
	getRec := httptest.NewRecorder()
	getCtx, _ := gin.CreateTestContext(getRec)
	getCtx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/career/materials/"+created.MaterialID, nil).WithContext(base)
	getCtx.Params = gin.Params{{Key: "materialId", Value: created.MaterialID}}
	h.GetMaterial(getCtx)
	require.Equal(t, 200, getRec.Code, getRec.Body.String())
	require.Contains(t, getRec.Body.String(), `"versionCount":1`)

	gin.SetMode(gin.TestMode)
	verRec := httptest.NewRecorder()
	verCtx, _ := gin.CreateTestContext(verRec)
	verCtx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/career/materials/"+created.MaterialID+"/versions", nil).WithContext(base)
	verCtx.Params = gin.Params{{Key: "materialId", Value: created.MaterialID}}
	h.ListMaterialVersions(verCtx)
	require.Equal(t, 200, verRec.Code, verRec.Body.String())
	require.Contains(t, verRec.Body.String(), `"version":1`)

	gin.SetMode(gin.TestMode)
	cmpRec := httptest.NewRecorder()
	cmpCtx, _ := gin.CreateTestContext(cmpRec)
	cmpCtx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/career/materials/"+created.MaterialID+"/versions/1/compare?baseline=1", nil).WithContext(base)
	cmpCtx.Params = gin.Params{{Key: "materialId", Value: created.MaterialID}, {Key: "versionId", Value: "1"}}
	h.CompareMaterialVersions(cmpCtx)
	require.Equal(t, 200, cmpRec.Code, cmpRec.Body.String())

	// An unknown material is a plain 404 without leaking existence.
	gin.SetMode(gin.TestMode)
	missingRec := httptest.NewRecorder()
	missingCtx, _ := gin.CreateTestContext(missingRec)
	missingCtx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/career/materials/00000000-0000-0000-0000-000000000000", nil).WithContext(base)
	missingCtx.Params = gin.Params{{Key: "materialId", Value: "00000000-0000-0000-0000-000000000000"}}
	h.GetMaterial(missingCtx)
	require.Equal(t, 404, missingRec.Code)
	require.Contains(t, missingRec.Body.String(), "not_found")
}
