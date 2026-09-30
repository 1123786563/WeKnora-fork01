package career

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
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

func TestBusyUploadDuplicateDoesNotCreateClaimBeforeGuardOwner(t *testing.T) {
	o, _ := testOffice(t)
	scope := Scope{TenantID: 1, UserID: "u1"}
	ctx := WithScope(context.Background(), scope)
	requestID := "busy-before-source"
	intent, err := json.Marshal([]any{"resume.txt", "text/plain"})
	require.NoError(t, err)
	hash := sha256.Sum256(intent)
	_, unlock, err := o.acquireLifecycleClaim(ctx, scope, "source_upload", requestID, hex.EncodeToString(hash[:]))
	require.NoError(t, err)
	defer unlock()
	files := &careerUploadFiles{}
	h := &Handler{office: o, members: &memberListStub{members: []*types.TenantMember{{UserID: "u1", TenantID: 1, Role: types.TenantRoleOwner}}}, upload: NewUploadAdapter(files, &careerUploadCatalog{}, careerUploadReader{})}
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	partHeader := make(textproto.MIMEHeader)
	partHeader.Set("Content-Disposition", `form-data; name="file"; filename="resume.txt"`)
	partHeader.Set("Content-Type", "text/plain")
	part, err := mw.CreatePart(partHeader)
	require.NoError(t, err)
	_, err = part.Write([]byte("Education: Example University"))
	require.NoError(t, err)
	require.NoError(t, mw.WriteField("requestId", requestID))
	require.NoError(t, mw.WriteField("expectedRevision", "0"))
	require.NoError(t, mw.Close())
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	base := context.WithValue(context.Background(), types.UserIDContextKey, "u1")
	base = context.WithValue(base, types.TenantIDContextKey, uint64(1))
	req := httptest.NewRequest("POST", "/api/v1/career/sources/upload", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	c.Request = req.WithContext(base)
	h.Upload(c)
	require.Equal(t, 409, rec.Code, rec.Body.String())
	require.Zero(t, files.saves)
	var sourceCount int64
	require.NoError(t, o.db.Model(&sourceRevision{}).Where("request_id=?", requestID).Count(&sourceCount).Error)
	require.Zero(t, sourceCount, "busy duplicate must not insert a processing source")
}

func TestFinishClaimFailureTreatsCleanupClaimLossAsSuperseded(t *testing.T) {
	o, db, ctx := newProgressOffice(t, "claim-owner", 923)
	source, err := o.CreateProcessingSource(ctx, SourceUpload{ID: "cleanup-claim-loss", FileName: "resume.pdf", MIMEType: "application/pdf", Size: 10, Digest: "sha256:claim", ResourceRef: "private://resume"})
	require.NoError(t, err)
	require.NoError(t, db.Model(&sourceRevision{}).Where("id = ?", source.ID).Update("claim_token", "original-claim-token").Error)
	require.NoError(t, db.Exec(`CREATE TRIGGER replace_cleanup_claim AFTER UPDATE OF status ON career_source_revisions
		WHEN NEW.id = 'cleanup-claim-loss'
		BEGIN UPDATE career_source_revisions SET claim_token = 'new-owner-token' WHERE id = NEW.id; END;`).Error)
	h := &Handler{office: o, upload: &UploadAdapter{}}
	latest, superseded, err := h.finishClaimFailure(ctx, source.ID, "original-claim-token", errors.New("parse failed"))
	require.NoError(t, err)
	require.True(t, superseded)
	require.Equal(t, "new-owner-token", latest.ClaimToken)
	require.Equal(t, "failed", latest.Status)
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

func TestCareerProgressHTTPContract(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	office, err := NewOffice(db)
	require.NoError(t, err)
	scope := Scope{UserID: "progress-owner", TenantID: 98}
	base := context.WithValue(context.Background(), types.UserIDContextKey, scope.UserID)
	base = context.WithValue(base, types.TenantIDContextKey, scope.TenantID)
	ctx := WithScope(base, scope)
	require.NoError(t, office.ClaimSpace(ctx))

	applicationID := seedProgressApplication(t, office, ctx, "仅限2027届。", "2027", "prog-http", "batch-a")
	h := &Handler{office: office, members: &memberListStub{members: []*types.TenantMember{{UserID: scope.UserID, TenantID: scope.TenantID, Role: types.TenantRoleOwner}}}}
	call := func(method, target, body string, params gin.Params) *httptest.ResponseRecorder {
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
		c.Params = params
		switch {
		case method == http.MethodPost && strings.HasSuffix(target, "/correct"):
			h.CorrectProgress(c)
		case method == http.MethodPost:
			h.AppendProgress(c)
		case strings.Contains(target, "/receipt"):
			h.ProgressReceiptHandler(c)
		default:
			h.ApplicationProgress(c)
		}
		return rec
	}

	appendBody := func(requestID, eventType string, revision uint64) string {
		return fmt.Sprintf(`{"requestId":%q,"eventType":%q,"note":"官网投递","occurredAt":"2026-09-20T10:00:00Z","source":{"kind":"manual"},"expectedRevision":%d}`,
			requestID, eventType, revision)
	}

	// The append seam returns the frozen receipt with the authenticated
	// confirmer; clients cannot inject provenance outside the whitelist.
	rec := call(http.MethodPost, "/api/v1/career/applications/"+applicationID+"/progress",
		fmt.Sprintf(`{"requestId":"prog-http-1","eventType":"submitted","source":{"kind":"resume_extraction"},"expectedRevision":0}`),
		gin.Params{{Key: "applicationId", Value: applicationID}})
	require.Equal(t, 400, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "invalid_request")

	rec = call(http.MethodPost, "/api/v1/career/applications/"+applicationID+"/progress", appendBody("prog-http-1", ProgressEventSubmitted, 0),
		gin.Params{{Key: "applicationId", Value: applicationID}})
	require.Equal(t, 400, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "invalid_request")

	rec = call(http.MethodPost, "/api/v1/career/applications/"+applicationID+"/progress", appendBody("prog-http-1", ProgressEventPendingSubmission, 0),
		gin.Params{{Key: "applicationId", Value: applicationID}})
	require.Equal(t, 200, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), `"kind":"progress_appended"`)
	require.Contains(t, rec.Body.String(), `"confirmer":"progress-owner"`)
	var appended ProgressReceipt
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &appended))
	require.Equal(t, ProgressStagePendingSubmission, appended.Stage)

	// A stale expected revision maps to the typed 409 with the current value.
	rec = call(http.MethodPost, "/api/v1/career/applications/"+applicationID+"/progress", appendBody("prog-http-2", ProgressEventAssessment, 0),
		gin.Params{{Key: "applicationId", Value: applicationID}})
	require.Equal(t, 409, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "revision_conflict")
	require.Contains(t, rec.Body.String(), `"currentRevision":1`)

	// The correction seam appends without overwriting.
	correctBody := fmt.Sprintf(`{"requestId":"prog-http-c1","correctsEventId":%q,"eventType":"assessment","source":{"kind":"manual"},"expectedRevision":1}`, appended.EventID)
	rec = call(http.MethodPost, "/api/v1/career/applications/"+applicationID+"/progress/correct", correctBody,
		gin.Params{{Key: "applicationId", Value: applicationID}})
	require.Equal(t, 200, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), `"kind":"progress_corrected"`)
	require.Contains(t, rec.Body.String(), fmt.Sprintf(`"correctsEventId":%q`, appended.EventID))

	// The view serves the full history plus the deterministic projection.
	rec = call(http.MethodGet, "/api/v1/career/applications/"+applicationID+"/progress", "", gin.Params{{Key: "applicationId", Value: applicationID}})
	require.Equal(t, 200, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), `"stage":"assessment"`)
	require.Contains(t, rec.Body.String(), `"revision":2`)

	// The receipt endpoint replays by request ID and 404s unknown requests.
	rec = call(http.MethodGet, "/api/v1/career/progress/receipt?requestId=prog-http-1", "", nil)
	require.Equal(t, 200, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), `"requestId":"prog-http-1"`)
	rec = call(http.MethodGet, "/api/v1/career/progress/receipt?requestId=missing", "", nil)
	require.Equal(t, 404, rec.Code, rec.Body.String())

	// Another application of the same owner stays unchained.
	rec = call(http.MethodGet, "/api/v1/career/applications/00000000-0000-0000-0000-000000000000/progress", "",
		gin.Params{{Key: "applicationId", Value: "00000000-0000-0000-0000-000000000000"}})
	require.Equal(t, 404, rec.Code)
	require.Contains(t, rec.Body.String(), "not_found")
}

func TestCareerMaterialExportHTTPContract(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	office, err := NewOffice(db)
	require.NoError(t, err)
	store := newMapExportStorage()
	office.SetExportStorage(store)
	office.SetExportSigningKey([]byte("0123456789abcdef0123456789abcdef"))
	scope := Scope{UserID: "export-http-owner", TenantID: 99}
	base := context.WithValue(context.Background(), types.UserIDContextKey, scope.UserID)
	base = context.WithValue(base, types.TenantIDContextKey, scope.TenantID)
	ctx := WithScope(base, scope)
	require.NoError(t, office.ClaimSpace(ctx))

	seed := seedMaterialEvidence(t, office, ctx, "2027", "export-http")
	created, err := office.EditMaterial(ctx, editMaterialInput(seed, "export-http-edit", exportFixtureBody()))
	require.NoError(t, err)
	_, err = office.ConfirmMaterial(ctx, ConfirmMaterialInput{RequestID: "export-http-confirm", MaterialID: created.MaterialID, ExpectedRevision: seed.Revision})
	require.NoError(t, err)

	h := &Handler{office: office, members: &memberListStub{members: []*types.TenantMember{{UserID: scope.UserID, TenantID: scope.TenantID, Role: types.TenantRoleOwner}}}}
	call := func(method, target, body string, params gin.Params) *httptest.ResponseRecorder {
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
		c.Params = params
		switch {
		case method == http.MethodPost && strings.HasSuffix(target, "/exports"):
			h.PublishMaterialHandler(c)
		case method == http.MethodGet && strings.HasSuffix(target, "/exports"):
			h.ListMaterialExports(c)
		case method == http.MethodPost && strings.HasSuffix(target, "/signed-url"):
			h.MaterialExportSignedURL(c)
		case method == http.MethodGet && strings.Contains(target, "/download?"):
			h.DownloadMaterialExport(c)
		case method == http.MethodDelete:
			h.RevokeMaterialExport(c)
		default:
			t.Fatalf("unexpected call %s %s", method, target)
		}
		return rec
	}
	exportParams := func(exportID string) gin.Params {
		return gin.Params{{Key: "materialId", Value: created.MaterialID}, {Key: "exportId", Value: exportID}}
	}

	// Publish renders and verifies both formats of the confirmed version.
	rec := call(http.MethodPost, "/api/v1/career/materials/"+created.MaterialID+"/exports",
		fmt.Sprintf(`{"requestId":"export-http-publish","version":1,"expectedRevision":%d}`, seed.Revision),
		gin.Params{{Key: "materialId", Value: created.MaterialID}})
	require.Equal(t, 200, rec.Code, rec.Body.String())
	var receipt ExportReceipt
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &receipt))
	require.Equal(t, ExportStatusSubmittable, receipt.Status)
	require.True(t, receipt.Submittable)
	require.Len(t, receipt.Files, 2)

	// The list shows the export with its immutable binding.
	rec = call(http.MethodGet, "/api/v1/career/materials/"+created.MaterialID+"/exports", "", gin.Params{{Key: "materialId", Value: created.MaterialID}})
	require.Equal(t, 200, rec.Code)
	var listed struct {
		Exports []ExportReceipt `json:"exports"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &listed))
	require.Len(t, listed.Exports, 1)

	// The owner mints a short-lived grant and redeems the PDF bytes.
	rec = call(http.MethodPost, "/api/v1/career/materials/"+created.MaterialID+"/exports/"+receipt.ExportID+"/signed-url",
		`{"format":"pdf","ttlSeconds":300}`, exportParams(receipt.ExportID))
	require.Equal(t, 200, rec.Code, rec.Body.String())
	var grant ExportDownload
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &grant))
	require.Equal(t, receipt.ExportID, grant.ExportID)
	require.NotEmpty(t, grant.URL)

	rec = call(http.MethodGet, grant.URL, "", exportParams(receipt.ExportID))
	require.Equal(t, 200, rec.Code)
	require.Equal(t, "application/pdf", rec.Header().Get("Content-Type"))
	require.Equal(t, store.files[exportFile(receipt, ExportFormatPDF).ObjectKey], rec.Body.Bytes())

	// Revocation makes the already-issued grant fail immediately.
	rec = call(http.MethodDelete, "/api/v1/career/materials/"+created.MaterialID+"/exports/"+receipt.ExportID,
		fmt.Sprintf(`{"requestId":"export-http-revoke","expectedRevision":%d}`, seed.Revision), exportParams(receipt.ExportID))
	require.Equal(t, 200, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), ExportStatusRevoked)

	rec = call(http.MethodGet, grant.URL, "", exportParams(receipt.ExportID))
	require.Equal(t, 404, rec.Code)
	require.Contains(t, rec.Body.String(), "export_grant_invalid")

	// A revoked export no longer mints grants.
	rec = call(http.MethodPost, "/api/v1/career/materials/"+created.MaterialID+"/exports/"+receipt.ExportID+"/signed-url",
		`{"format":"pdf","ttlSeconds":300}`, exportParams(receipt.ExportID))
	require.Equal(t, 409, rec.Code)
	require.Contains(t, rec.Body.String(), "export_not_submittable")
}

func TestCareerRuleHTTPContract(t *testing.T) {
	o, _, _, ctx := newSearchRuleOffice(t)
	require.NoError(t, o.ClaimSpace(ctx))
	base := context.WithValue(context.Background(), types.UserIDContextKey, "rule-owner")
	base = context.WithValue(base, types.TenantIDContextKey, uint64(96))
	h := &Handler{office: o, members: &memberListStub{members: []*types.TenantMember{{UserID: "rule-owner", TenantID: 96, Role: types.TenantRoleOwner}}}}

	post := func(body string) *httptest.ResponseRecorder {
		gin.SetMode(gin.TestMode)
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/career/rules", strings.NewReader(body)).WithContext(base)
		c.Request.Header.Set("Content-Type", "application/json")
		h.SetRule(c)
		return rec
	}

	// set_rule stores the rule and surfaces the frozen receipt with the
	// deterministic estimate.
	rec := post(`{"requestId":"rule-http-1","query":"go engineer","intervalMinutes":60,"status":"enabled","expectedRevision":0}`)
	require.Equal(t, 200, rec.Code, rec.Body.String())
	var receipt SetRuleReceipt
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &receipt))
	require.Equal(t, RuleKindSet, receipt.Kind)
	require.Equal(t, RuleStatusEnabled, receipt.Status)
	require.NotNil(t, receipt.NextDueAt)
	require.NotEmpty(t, receipt.Estimate.Basis)

	listRec := httptest.NewRecorder()
	listCtx, _ := gin.CreateTestContext(listRec)
	listCtx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/career/rules", nil).WithContext(base)
	h.ListRules(listCtx)
	require.Equal(t, 200, listRec.Code, listRec.Body.String())
	var page struct {
		Rules      []map[string]json.RawMessage `json:"rules"`
		NextCursor *string                      `json:"nextCursor"`
	}
	require.NoError(t, json.Unmarshal(listRec.Body.Bytes(), &page))
	require.Len(t, page.Rules, 1)
	require.Nil(t, page.NextCursor)
	require.Contains(t, page.Rules[0], "nextDueAt")
	require.Contains(t, page.Rules[0], "estimate")
	require.NotContains(t, page.Rules[0], "runs")
	require.NotContains(t, page.Rules[0], "todos")

	unauthenticatedListRec := httptest.NewRecorder()
	unauthenticatedListCtx, _ := gin.CreateTestContext(unauthenticatedListRec)
	unauthenticatedListCtx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/career/rules", nil)
	h.ListRules(unauthenticatedListCtx)
	require.Equal(t, 403, unauthenticatedListRec.Code)

	// Exact replay replays the stored receipt by request ID.
	rec = post(`{"requestId":"rule-http-1","query":"go engineer","intervalMinutes":60,"status":"enabled","expectedRevision":0}`)
	require.Equal(t, 200, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), receipt.RuleID)

	// Content change under the same request ID is a typed conflict.
	rec = post(`{"requestId":"rule-http-1","query":"go engineer","intervalMinutes":30,"status":"enabled","expectedRevision":0}`)
	require.Equal(t, 409, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "idempotency_conflict")

	// Unknown fields, invalid status, and out-of-range intervals are invalid.
	rec = post(`{"requestId":"rule-http-bad","query":"go engineer","intervalMinutes":60,"status":"enabled","expectedRevision":0,"extra":1}`)
	require.Equal(t, 400, rec.Code, rec.Body.String())
	rec = post(`{"requestId":"rule-http-bad","query":"go engineer","intervalMinutes":60,"status":"sometimes","expectedRevision":0}`)
	require.Equal(t, 400, rec.Code, rec.Body.String())
	rec = post(`{"requestId":"rule-http-bad","query":"go engineer","intervalMinutes":0,"status":"enabled","expectedRevision":0}`)
	require.Equal(t, 400, rec.Code, rec.Body.String())

	// The receipt endpoint replays by request ID and 404s unknown requests.
	gin.SetMode(gin.TestMode)
	receiptRec := httptest.NewRecorder()
	receiptCtx, _ := gin.CreateTestContext(receiptRec)
	receiptCtx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/career/rules/receipt?requestId=rule-http-1", nil).WithContext(base)
	h.RuleReceipt(receiptCtx)
	require.Equal(t, 200, receiptRec.Code, receiptRec.Body.String())
	require.Contains(t, receiptRec.Body.String(), RuleKindSet)

	gin.SetMode(gin.TestMode)
	missingRec := httptest.NewRecorder()
	missingCtx, _ := gin.CreateTestContext(missingRec)
	missingCtx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/career/rules/receipt?requestId=missing", nil).WithContext(base)
	h.RuleReceipt(missingCtx)
	require.Equal(t, 404, missingRec.Code)

	// The rule view serves the live contract under the authenticated scope.
	gin.SetMode(gin.TestMode)
	viewRec := httptest.NewRecorder()
	viewCtx, _ := gin.CreateTestContext(viewRec)
	viewCtx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/career/rules/"+receipt.RuleID, nil).WithContext(base)
	viewCtx.Params = gin.Params{{Key: "ruleId", Value: receipt.RuleID}}
	h.GetRule(viewCtx)
	require.Equal(t, 200, viewRec.Code, viewRec.Body.String())
	require.Contains(t, viewRec.Body.String(), `"estimate":`)

	listRec = httptest.NewRecorder()
	listCtx, _ = gin.CreateTestContext(listRec)
	listCtx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/career/rules", nil).WithContext(base)
	h.ListRules(listCtx)
	require.Equal(t, 200, listRec.Code, listRec.Body.String())
	var listed struct {
		Rules []RuleSummary `json:"rules"`
	}
	require.NoError(t, json.Unmarshal(listRec.Body.Bytes(), &listed))
	require.Len(t, listed.Rules, 1)
	require.Equal(t, receipt.RuleID, listed.Rules[0].RuleID)
	require.NotEmpty(t, listed.Rules[0].CreatedAt)
	require.NotEmpty(t, listed.Rules[0].UpdatedAt)
	require.Contains(t, listRec.Body.String(), `"rules":[`)

	unauthRec := httptest.NewRecorder()
	unauthCtx, _ := gin.CreateTestContext(unauthRec)
	unauthCtx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/career/rules", nil)
	h.ListRules(unauthCtx)
	require.Equal(t, 403, unauthRec.Code)

	// An intruder from the same tenant is rejected at the scope gate.
	gin.SetMode(gin.TestMode)
	intruderRec := httptest.NewRecorder()
	intruderCtx, _ := gin.CreateTestContext(intruderRec)
	intruderBase := context.WithValue(context.Background(), types.UserIDContextKey, "intruder")
	intruderBase = context.WithValue(intruderBase, types.TenantIDContextKey, uint64(96))
	intruderCtx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/career/rules", strings.NewReader(`{"requestId":"rule-http-intruder","query":"go engineer","intervalMinutes":60,"status":"enabled","expectedRevision":0}`)).WithContext(intruderBase)
	intruderCtx.Request.Header.Set("Content-Type", "application/json")
	h.SetRule(intruderCtx)
	require.Equal(t, 403, intruderRec.Code)
}

func TestCareerSubmissionHTTPContract(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	office, err := NewOffice(db)
	require.NoError(t, err)
	store := newMapExportStorage()
	office.SetExportStorage(store)
	office.SetExportSigningKey([]byte("0123456789abcdef0123456789abcdef"))
	scope := Scope{UserID: "submission-owner", TenantID: 99}
	base := context.WithValue(context.Background(), types.UserIDContextKey, scope.UserID)
	base = context.WithValue(base, types.TenantIDContextKey, scope.TenantID)
	ctx := WithScope(base, scope)
	require.NoError(t, office.ClaimSpace(ctx))

	fx := seedSubmissionFixture(t, office, ctx, "sub-http")
	h := &Handler{office: office, members: &memberListStub{members: []*types.TenantMember{{UserID: scope.UserID, TenantID: scope.TenantID, Role: types.TenantRoleOwner}}}}
	call := func(method, target, body string, params gin.Params) *httptest.ResponseRecorder {
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
		c.Params = params
		switch {
		case method == http.MethodPost:
			h.RecordSubmission(c)
		case strings.Contains(target, "/receipt"):
			h.SubmissionReceiptHandler(c)
		default:
			h.ApplicationSubmissions(c)
		}
		return rec
	}
	applicationParams := gin.Params{{Key: "applicationId", Value: fx.ApplicationID}}

	// A stale expected revision maps to the typed 409 with the current value
	// (checked while the application is still unconfirmed).
	staleBody := fmt.Sprintf(`{"requestId":"sub-http-3","channel":"web","versionUnknown":true,"expectedRevision":%d}`, fx.Revision+9)
	rec := call(http.MethodPost, "/api/v1/career/applications/"+fx.ApplicationID+"/submissions", staleBody, applicationParams)
	require.Equal(t, 409, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "revision_conflict")
	require.Contains(t, rec.Body.String(), fmt.Sprintf(`"currentRevision":%d`, fx.Revision))

	// An unknown field is rejected: the contract stays closed.
	rec = call(http.MethodPost, "/api/v1/career/applications/"+fx.ApplicationID+"/submissions",
		fmt.Sprintf(`{"requestId":"sub-http-x","channel":"web","autoSubmit":true,"expectedRevision":%d}`, fx.Revision), applicationParams)
	require.Equal(t, 400, rec.Code, rec.Body.String())

	// The record seam returns the frozen receipt: channel, claimed time, and
	// the exact bound version of the submittable export.
	boundBody := fmt.Sprintf(`{"requestId":"sub-http-1","channel":"email","occurredAt":"2026-09-22T08:30:00Z","materialId":%q,"exportId":%q,"expectedRevision":%d}`,
		fx.MaterialID, fx.ExportID, fx.Revision)
	rec = call(http.MethodPost, "/api/v1/career/applications/"+fx.ApplicationID+"/submissions", boundBody, applicationParams)
	require.Equal(t, 200, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), `"kind":"submission_recorded"`)
	require.Contains(t, rec.Body.String(), `"channel":"email"`)
	require.Contains(t, rec.Body.String(), `"versionConfirmed":true`)
	require.Contains(t, rec.Body.String(), `"confirmer":"submission-owner"`)
	var recorded SubmissionReceipt
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &recorded))
	require.NotNil(t, recorded.BoundVersion)
	require.EqualValues(t, fx.Version, recorded.BoundVersion.Version)

	// A repeat confirmation under a new request ID is the typed 409.
	unknownBody := fmt.Sprintf(`{"requestId":"sub-http-2","channel":"web","versionUnknown":true,"expectedRevision":%d}`, fx.Revision)
	rec = call(http.MethodPost, "/api/v1/career/applications/"+fx.ApplicationID+"/submissions", unknownBody, applicationParams)
	require.Equal(t, 409, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "submission_already_confirmed")

	// The list seam serves the single durable record.
	rec = call(http.MethodGet, "/api/v1/career/applications/"+fx.ApplicationID+"/submissions", "", applicationParams)
	require.Equal(t, 200, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), recorded.SubmissionID)

	// The receipt endpoint replays by request ID and 404s unknown requests.
	rec = call(http.MethodGet, "/api/v1/career/submissions/receipt?requestId=sub-http-1", "", nil)
	require.Equal(t, 200, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), `"requestId":"sub-http-1"`)
	rec = call(http.MethodGet, "/api/v1/career/submissions/receipt?requestId=missing", "", nil)
	require.Equal(t, 404, rec.Code, rec.Body.String())

	// A body application ID diverging from the path is rejected.
	mismatchBody := fmt.Sprintf(`{"requestId":"sub-http-4","applicationId":"00000000-0000-0000-0000-00000000000f","channel":"web","versionUnknown":true,"expectedRevision":%d}`, fx.Revision)
	rec = call(http.MethodPost, "/api/v1/career/applications/"+fx.ApplicationID+"/submissions", mismatchBody, applicationParams)
	require.Equal(t, 400, rec.Code, rec.Body.String())
}

// TestCareerExportDeletionHTTPContract drives the closed export_career and
// delete_career intents through the HTTP surface: boundary explanation,
// export with digest, receipt replay, deletion, and typed error mapping.
func TestCareerExportDeletionHTTPContract(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	office, err := NewOffice(db)
	require.NoError(t, err)
	office.SetExportStorage(newMapExportStorage())
	office.SetExportSigningKey([]byte("0123456789abcdef0123456789abcdef"))
	scope := Scope{UserID: "lifecycle-owner", TenantID: 101}
	base := context.WithValue(context.Background(), types.UserIDContextKey, scope.UserID)
	base = context.WithValue(base, types.TenantIDContextKey, scope.TenantID)
	ctx := WithScope(base, scope)
	require.NoError(t, office.ClaimSpace(ctx))

	view, err := office.Open(ctx)
	require.NoError(t, err)
	_, err = office.Confirm(ctx, "education.graduation_year", "2027", "lifecycle-year", view.Revision, Source{Kind: "manual"})
	require.NoError(t, err)
	view, err = office.Open(ctx)
	require.NoError(t, err)

	h := &Handler{office: office, members: &memberListStub{members: []*types.TenantMember{{UserID: scope.UserID, TenantID: scope.TenantID, Role: types.TenantRoleOwner}}}}
	call := func(method, target, body string, dispatch func(*gin.Context)) *httptest.ResponseRecorder {
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
		dispatch(c)
		return rec
	}

	// Boundary first: the structured in-space vs external explanation.
	rec := call(http.MethodGet, "/api/v1/career/deletions/boundary", "", h.CareerDeletionBoundaryHandler)
	require.Equal(t, 200, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "external_platform_submissions")
	require.Contains(t, rec.Body.String(), "career_data_deletions")

	// Export: one complete package with a verifiable digest.
	exportBody := fmt.Sprintf(`{"requestId":"life-export-1","expectedRevision":%d}`, view.Revision)
	rec = call(http.MethodPost, "/api/v1/career/exports", exportBody, h.ExportCareerHandler)
	require.Equal(t, 200, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), `"kind":"career_exported"`)
	require.Contains(t, rec.Body.String(), `"digest":"`)
	require.Contains(t, rec.Body.String(), "education.graduation_year")

	// Receipt replay by request ID; unknown request IDs 404.
	rec = call(http.MethodGet, "/api/v1/career/exports/receipt?requestId=life-export-1", "", h.ExportCareerReceiptHandler)
	require.Equal(t, 200, rec.Code, rec.Body.String())
	rec = call(http.MethodGet, "/api/v1/career/exports/receipt?requestId=missing", "", h.ExportCareerReceiptHandler)
	require.Equal(t, 404, rec.Code, rec.Body.String())

	// A changed intent under the same request ID is the typed 409.
	conflictBody := fmt.Sprintf(`{"requestId":"life-export-1","expectedRevision":%d}`, view.Revision+1)
	rec = call(http.MethodPost, "/api/v1/career/exports", conflictBody, h.ExportCareerHandler)
	require.Equal(t, 409, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "idempotency_conflict")

	// Empty request IDs stay 400.
	rec = call(http.MethodPost, "/api/v1/career/exports", `{"requestId":"","expectedRevision":1}`, h.ExportCareerHandler)
	require.Equal(t, 400, rec.Code, rec.Body.String())

	// Deletion: only after every step succeeds does it answer "deleted".
	deleteBody := fmt.Sprintf(`{"requestId":"life-delete-1","expectedRevision":%d}`, view.Revision)
	rec = call(http.MethodPost, "/api/v1/career/deletions", deleteBody, h.DeleteCareerHandler)
	require.Equal(t, 200, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), `"status":"partial"`)
	// Without a configured Workbench remover the removal step fails honestly;
	// the receipt keeps the recoverable state instead of claiming success.

	rec = call(http.MethodGet, "/api/v1/career/deletions/receipt?requestId=life-delete-1", "", h.CareerDeletionReceiptHandler)
	require.Equal(t, 200, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), `"status":"partial"`)
	rec = call(http.MethodGet, "/api/v1/career/deletions/receipt?requestId=missing", "", h.CareerDeletionReceiptHandler)
	require.Equal(t, 404, rec.Code, rec.Body.String())
}

func TestCareerPreparationHandlersPromptReplayAndFailure(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	office, err := NewOffice(db)
	require.NoError(t, err)
	store := newMapExportStorage()
	office.SetExportStorage(store)
	office.SetExportSigningKey([]byte("0123456789abcdef0123456789abcdef"))
	scope := Scope{UserID: "prep-owner", TenantID: 4711}
	base := context.WithValue(context.Background(), types.UserIDContextKey, scope.UserID)
	base = context.WithValue(base, types.TenantIDContextKey, scope.TenantID)
	ctx := WithScope(base, scope)
	require.NoError(t, office.ClaimSpace(ctx))
	h := &Handler{office: office, members: &memberListStub{members: []*types.TenantMember{{UserID: scope.UserID, TenantID: scope.TenantID, Role: types.TenantRoleOwner}}}}
	invoke := func(method, target string, body string, params gin.Params, dispatch func(*gin.Context)) *httptest.ResponseRecorder {
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
		c.Params = params
		dispatch(c)
		return rec
	}

	// Seed one application with a confirmed submitted version and one with
	// the explicit unknown marker.
	fx := seedSubmissionFixture(t, office, ctx, "prep")
	_, err = office.RecordSubmission(ctx, RecordSubmissionInput{
		RequestID: "prep-sub", ApplicationID: fx.ApplicationID, Channel: SubmissionChannelEmail,
		MaterialID: fx.MaterialID, ExportID: fx.ExportID, ExpectedRevision: fx.Revision,
	})
	require.NoError(t, err)
	unknownFx := seedSubmissionFixture(t, office, ctx, "prepunk")
	_, err = office.RecordSubmission(ctx, RecordSubmissionInput{
		RequestID: "prepunk-sub", ApplicationID: unknownFx.ApplicationID, Channel: SubmissionChannelWeb,
		VersionUnknown: true, ExpectedRevision: unknownFx.Revision,
	})
	require.NoError(t, err)

	// Unknown submitted version: the typed prompt state is visible, never a
	// silent guess.
	head, err := office.Open(ctx)
	require.NoError(t, err)
	body := fmt.Sprintf(`{"requestId":"prep-unk-1","focus":"cover_letter","expectedRevision":%d}`, head.Revision)
	rec := invoke(http.MethodPost, "/api/v1/career/applications/"+unknownFx.ApplicationID+"/preparations", body,
		gin.Params{{Key: "applicationId", Value: unknownFx.ApplicationID}}, h.GeneratePreparationHandler)
	require.Equal(t, 409, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "preparation_version_unknown")
	require.Contains(t, rec.Body.String(), unknownFx.ApplicationID)

	// Confirmed submitted version: generation answers the draft receipt.
	body = fmt.Sprintf(`{"requestId":"prep-ok-1","focus":"interview_prep","expectedRevision":%d}`, head.Revision)
	rec = invoke(http.MethodPost, "/api/v1/career/applications/"+fx.ApplicationID+"/preparations", body,
		gin.Params{{Key: "applicationId", Value: fx.ApplicationID}}, h.GeneratePreparationHandler)
	require.Equal(t, 200, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), `"kind":"preparation_generated"`)
	require.Contains(t, rec.Body.String(), `"status":"draft"`)

	// The receipt replays by request ID; a missing request ID is 404.
	rec = invoke(http.MethodGet, "/api/v1/career/preparations/receipt?requestId=prep-ok-1", "", nil, h.PreparationReceiptHandler)
	require.Equal(t, 200, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), `"requestId":"prep-ok-1"`)
	rec = invoke(http.MethodGet, "/api/v1/career/preparations/receipt?requestId=missing", "", nil, h.PreparationReceiptHandler)
	require.Equal(t, 404, rec.Code, rec.Body.String())

	// The application listing includes the generated preparation.
	rec = invoke(http.MethodGet, "/api/v1/career/applications/"+fx.ApplicationID+"/preparations", "",
		gin.Params{{Key: "applicationId", Value: fx.ApplicationID}}, h.ApplicationPreparations)
	require.Equal(t, 200, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), `"preparations"`)

	// A changed intent under the same request ID is the typed 409.
	changed := fmt.Sprintf(`{"requestId":"prep-ok-1","focus":"cover_letter","expectedRevision":%d}`, head.Revision)
	rec = invoke(http.MethodPost, "/api/v1/career/applications/"+fx.ApplicationID+"/preparations", changed,
		gin.Params{{Key: "applicationId", Value: fx.ApplicationID}}, h.GeneratePreparationHandler)
	require.Equal(t, 409, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "idempotency_conflict")
}

// TestCareerReminderHandlerWritesTodoWithFrozenNotice pins the T20 HTTP
// surface: set_reminder takes a closed payload (request ID + expected
// revision), answers the frozen privacy notice, replays by request ID, and
// maps the typed source/reminder misses onto 404.
func TestCareerReminderHandlerWritesTodoWithFrozenNotice(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	office, err := NewOffice(db)
	require.NoError(t, err)
	scope := Scope{UserID: "reminder-owner", TenantID: 4713}
	base := context.WithValue(context.Background(), types.UserIDContextKey, scope.UserID)
	base = context.WithValue(base, types.TenantIDContextKey, scope.TenantID)
	ctx := WithScope(base, scope)
	require.NoError(t, office.ClaimSpace(ctx))
	h := &Handler{office: office, members: &memberListStub{members: []*types.TenantMember{{UserID: scope.UserID, TenantID: scope.TenantID, Role: types.TenantRoleOwner}}}}
	applicationID := seedProgressApplication(t, office, ctx, completeJDText, "2027", "http-rem", "http-rem-batch")
	view, err := office.Open(ctx)
	require.NoError(t, err)
	event, err := office.AppendProgress(ctx, appendProgressInput(applicationID, "http-rem-evt", ProgressEventInterview, "Acme 面试 10月1日", 0))
	require.NoError(t, err)

	invoke := func(method, target, body string, dispatch func(*gin.Context)) *httptest.ResponseRecorder {
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
		dispatch(c)
		return rec
	}

	// A closed payload: unknown fields are rejected outright.
	rec := invoke(http.MethodPost, "/api/v1/career/reminders", `{"requestId":"http-rem-1","sourceKind":"progress_event","sourceId":"`+event.EventID+`","expectedRevision":`+fmt.Sprint(view.Revision)+`,"note":"leak"}`, h.SetReminderHandler)
	require.Equal(t, 400, rec.Code, rec.Body.String())

	rec = invoke(http.MethodPost, "/api/v1/career/reminders", `{"requestId":"http-rem-1","sourceKind":"progress_event","sourceId":"`+event.EventID+`","expectedRevision":`+fmt.Sprint(view.Revision)+`}`, h.SetReminderHandler)
	require.Equal(t, 200, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), `"kind":"reminder_set"`)
	require.Contains(t, rec.Body.String(), ReminderNoticeBodies[ReminderNoticeProgressUpdated])
	require.NotContains(t, rec.Body.String(), "Acme")

	// The inbox listing is the authoritative reading surface.
	rec = invoke(http.MethodGet, "/api/v1/career/reminders", "", h.ListRemindersHandler)
	require.Equal(t, 200, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), `"reminders"`)

	// The receipt replays by request ID; foreign sources stay 404.
	rec = invoke(http.MethodGet, "/api/v1/career/reminders/receipt?requestId=http-rem-1", "", h.ReminderReceiptHandler)
	require.Equal(t, 200, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), `"requestId":"http-rem-1"`)
	rec = invoke(http.MethodGet, "/api/v1/career/reminders/receipt?requestId=missing", "", h.ReminderReceiptHandler)
	require.Equal(t, 404, rec.Code, rec.Body.String())
	rec = invoke(http.MethodPost, "/api/v1/career/reminders", `{"requestId":"http-rem-2","sourceKind":"progress_event","sourceId":"no-such-event","expectedRevision":`+fmt.Sprint(view.Revision)+`}`, h.SetReminderHandler)
	require.Equal(t, 404, rec.Code, rec.Body.String())

	// A changed intent under the same request ID is the typed 409.
	rec = invoke(http.MethodPost, "/api/v1/career/reminders", `{"requestId":"http-rem-1","sourceKind":"progress_event","sourceId":"no-such-event","expectedRevision":`+fmt.Sprint(view.Revision)+`}`, h.SetReminderHandler)
	require.Equal(t, 409, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "idempotency_conflict")
}

func TestCareerUsageEstimateHandlerContract(t *testing.T) {
	o, _, ctx := newUsageSearchOffice(t, "owner", 105, 1)
	require.NoError(t, o.ClaimSpace(ctx))
	base := context.WithValue(context.Background(), types.UserIDContextKey, "owner")
	base = context.WithValue(base, types.TenantIDContextKey, uint64(105))
	h := &Handler{office: o, members: &memberListStub{members: []*types.TenantMember{{UserID: "owner", TenantID: 105, Role: types.TenantRoleOwner}}}}
	request := func(target string) *httptest.ResponseRecorder {
		gin.SetMode(gin.TestMode)
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodGet, target, nil).WithContext(base)
		h.UsageEstimate(c)
		return rec
	}

	// The estimate is a free read: cost, conditions, and live balance before
	// any execution.
	rec := request("/api/v1/career/usage/estimate?operation=search_once")
	require.Equal(t, 200, rec.Code, rec.Body.String())
	var view UsageEstimateView
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &view))
	require.Equal(t, "usage_estimate", view.Kind)
	require.Equal(t, UsageOperationSearchOnce, view.Operation)
	require.EqualValues(t, 1, view.CostUnits)
	require.NotEmpty(t, view.Conditions)
	require.EqualValues(t, 1, view.LimitUnits)
	require.True(t, view.WouldAdmit)

	// An unknown operation is a typed refusal, never a fabricated estimate.
	rec = request("/api/v1/career/usage/estimate?operation=bulk_evaluate")
	require.Equal(t, 400, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "invalid_request")

	// An unavailable ledger maps to the typed 503, never a silent pass.
	o.failUsageLedgerRead = func() error { return errors.New("ledger read failed") }
	rec = request("/api/v1/career/usage/estimate?operation=search_once")
	require.Equal(t, 503, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "admission_unavailable")
}

func TestCareerReconcileHandlerContract(t *testing.T) {
	o, ctx := newSourceImportOffice(t, "owner", 131)
	base := context.WithValue(context.Background(), types.UserIDContextKey, "owner")
	base = context.WithValue(base, types.TenantIDContextKey, uint64(131))
	h := &Handler{office: o, members: &memberListStub{members: []*types.TenantMember{{UserID: "owner", TenantID: 131, Role: types.TenantRoleOwner}}}}

	o.opportunityExtractor = func(string) (OpportunityFields, error) {
		return knownFields("平台后端工程师", "示例科技", "杭州", "2027届秋招", "本科及以上学历"), nil
	}
	first, err := o.ImportJD(ctx, ImportJDInput{RequestID: "handler-rec-1", RawText: "岗位 A\n工作职责：\n负责服务\n任职要求：\n本科", SourceReference: "https://jobs.example.com/postings/1001"})
	require.NoError(t, err)
	second, err := o.ImportJD(ctx, ImportJDInput{RequestID: "handler-rec-2", RawText: "岗位 B\n工作职责：\n负责服务\n任职要求：\n本科", SourceReference: "https://other.example.net/jobs/1001"})
	require.NoError(t, err)

	post := func(body string) *httptest.ResponseRecorder {
		gin.SetMode(gin.TestMode)
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/career/opportunities/reconcile", strings.NewReader(body)).WithContext(base)
		h.ReconcileOpportunities(c)
		return rec
	}

	body := fmt.Sprintf(`{"requestId":"handler-rec-req","targetId":%q,"candidateId":%q}`, first.OpportunityID, second.OpportunityID)
	rec := post(body)
	require.Equal(t, 200, rec.Code, rec.Body.String())
	var receipt ReconcileReceipt
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &receipt))
	require.Equal(t, ReconcileDecisionMerged, receipt.Decision)

	// Unknown fields and malformed bodies are refused.
	rec = post(`{"requestId":"handler-rec-bad","targetId":"x","candidateId":"y","unexpected":true}`)
	require.Equal(t, 400, rec.Code, rec.Body.String())

	// Status reads carry the merged projection.
	gin.SetMode(gin.TestMode)
	rec = httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/career/opportunities/"+second.OpportunityID+"/status", nil).WithContext(base)
	c.Params = gin.Params{{Key: "opportunityId", Value: second.OpportunityID}}
	h.OpportunityStatus(c)
	require.Equal(t, 200, rec.Code, rec.Body.String())
	var status OpportunityStatusView
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &status))
	require.Equal(t, first.OpportunityID, status.MergedInto)

	// An unknown opportunity maps to the typed 404.
	gin.SetMode(gin.TestMode)
	rec = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/career/opportunities/missing/status", nil).WithContext(base)
	c.Params = gin.Params{{Key: "opportunityId", Value: "missing"}}
	h.OpportunityStatus(c)
	require.Equal(t, 404, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "not_found")

	// The coverage read is available on the authenticated scope.
	gin.SetMode(gin.TestMode)
	rec = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/career/coverage", nil).WithContext(base)
	h.SourceCoverage(c)
	require.Equal(t, 200, rec.Code, rec.Body.String())
	var coverage CareerCoverageView
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &coverage))
	require.Empty(t, coverage.ConfiguredSources, "production starts with no vetted source")
	require.Len(t, coverage.ObservedSources, 1)

	// The reconciliation receipt replays by request ID.
	gin.SetMode(gin.TestMode)
	rec = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/career/reconciliations/receipt?requestId=handler-rec-req", nil).WithContext(base)
	h.ReconciliationReceipt(c)
	require.Equal(t, 200, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), `"merged"`)
}

// OCR r1 fix: the Act endpoint is the only small JSON write without a body
// limit; an oversized body must answer 413 instead of being fully buffered.
func TestCareerActRejectsOversizedBodyWith413(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	office, err := NewOffice(db)
	require.NoError(t, err)
	scope := Scope{UserID: "u1", TenantID: 7}
	baseCtx := context.WithValue(context.Background(), types.UserIDContextKey, scope.UserID)
	baseCtx = context.WithValue(baseCtx, types.TenantIDContextKey, scope.TenantID)
	require.NoError(t, office.ClaimSpace(WithScope(baseCtx, scope)))
	h := &Handler{office: office, members: &memberListStub{members: []*types.TenantMember{{UserID: "u1", TenantID: 7, Role: types.TenantRoleOwner}}}}

	huge := `{"action":"propose","key":"education.school","value":"` + strings.Repeat("x", 32*1024) + `","requestId":"too-big-1"}`
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("POST", "/api/v1/career/act", strings.NewReader(huge)).WithContext(baseCtx)
	h.Act(c)
	require.Equal(t, http.StatusRequestEntityTooLarge, recorder.Code, recorder.Body.String())

	view, err := office.Open(WithScope(baseCtx, scope))
	require.NoError(t, err)
	require.Empty(t, view.Proposals, "an oversized act must never reach the office")
}
