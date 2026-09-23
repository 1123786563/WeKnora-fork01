package career

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	secutils "github.com/Tencent/WeKnora/internal/utils"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Handler struct {
	office  *Office
	upload  *UploadAdapter
	members interface {
		ListByTenant(context.Context, uint64) ([]*types.TenantMember, error)
	}
}

func NewHandler(db *gorm.DB, members interfaces.TenantMemberService, files interfaces.FileService, catalog interfaces.ResourceCatalog, reader interfaces.DocumentReader) (*Handler, error) {
	o, e := NewOffice(db)
	if e != nil {
		return nil, e
	}
	return &Handler{office: o, members: members, upload: NewUploadAdapter(files, catalog, reader)}, nil
}
func validateOwnerOnlyCareerTenant(userID string, tenantID uint64, members []*types.TenantMember) error {
	if userID == "" || tenantID == 0 || len(members) != 1 || members[0] == nil || members[0].UserID != userID || members[0].TenantID != tenantID || members[0].Role != types.TenantRoleOwner {
		return ErrUnauthorized
	}
	return nil
}
func (h *Handler) scope(c *gin.Context, claim bool) (context.Context, bool) {
	ctx := c.Request.Context()
	uid, uok := types.UserIDFromContext(ctx)
	tid, tok := types.TenantIDFromContext(ctx)
	if !uok || !tok || h.members == nil {
		writeError(c, ErrUnauthorized)
		return nil, false
	}
	members, e := h.members.ListByTenant(ctx, tid)
	if e != nil {
		c.JSON(500, gin.H{"error": gin.H{"code": "internal", "message": "failed to verify personal career workspace"}})
		return nil, false
	}
	if e = validateOwnerOnlyCareerTenant(uid, tid, members); e != nil {
		writeError(c, e)
		return nil, false
	}
	ctx = WithScope(ctx, Scope{UserID: uid, TenantID: tid})
	if claim {
		if e = h.office.ClaimSpace(ctx); e != nil {
			writeError(c, e)
			return nil, false
		}
	}
	return ctx, true
}
func writeError(c *gin.Context, e error) {
	status, code := 500, "internal"
	switch {
	case errors.Is(e, ErrUnauthorized):
		status = 403
		code = "forbidden"
	case errors.Is(e, ErrRevisionConflict):
		status = 409
		code = "revision_conflict"
	case errors.Is(e, ErrIdempotencyConflict):
		status = 409
		code = "idempotency_conflict"
	case errors.Is(e, ErrInvalidRequest):
		status = 400
		code = "invalid_request"
	case errors.Is(e, ErrReceiptNotFound), errors.Is(e, ErrProposalNotFound), errors.Is(e, ErrSourceNotFound):
		status = 404
		code = "not_found"
	case errors.Is(e, ErrProposalResolved):
		status = 409
		code = "proposal_resolved"
	case errors.Is(e, ErrOutcomeUnknown):
		status = 504
		code = "outcome_unknown"
	}
	body := gin.H{"code": code, "message": e.Error()}
	if errors.Is(e, ErrRevisionConflict) {
		var ce *RevisionConflictError
		if errors.As(e, &ce) {
			body["currentRevision"] = ce.CurrentRevision
		}
	}
	if errors.Is(e, ErrOutcomeUnknown) {
		var unknown *OutcomeUnknownError
		if errors.As(e, &unknown) {
			body["requestId"] = unknown.RequestID
		}
	}
	c.JSON(status, gin.H{"error": body})
}
func (h *Handler) Open(c *gin.Context) {
	ctx, ok := h.scope(c, true)
	if !ok {
		return
	}
	v, e := h.office.Open(ctx)
	if e != nil {
		writeError(c, e)
		return
	}
	c.JSON(200, v)
}
func (h *Handler) List(c *gin.Context) {
	ctx, ok := h.scope(c, false)
	if !ok {
		return
	}
	v, e := h.office.Open(ctx)
	if e != nil {
		writeError(c, e)
		return
	}
	c.JSON(200, v)
}
func (h *Handler) Act(c *gin.Context) {
	ctx, ok := h.scope(c, false)
	if !ok {
		return
	}
	var req struct {
		Action           string `json:"action"`
		ProposalID       string `json:"proposalId"`
		Key              string `json:"key"`
		Value            string `json:"value"`
		RequestID        string `json:"requestId"`
		ExpectedRevision uint64 `json:"expectedRevision"`
		Source           Source `json:"source"`
	}
	if e := c.ShouldBindJSON(&req); e != nil {
		writeError(c, ErrInvalidRequest)
		return
	}
	// Resume extraction provenance is server-issued by CompleteIntake. A client
	// may add manual facts but cannot label its own values as parser output.
	if req.Action == "propose" && (req.Source.Kind != "manual" || req.Source.ReferenceID != "") {
		writeError(c, ErrInvalidRequest)
		return
	}
	r, e := h.office.Act(ctx, req.Action, req.ProposalID, req.Key, req.Value, req.RequestID, req.ExpectedRevision, req.Source)
	if e != nil {
		writeError(c, e)
		return
	}
	c.JSON(200, r)
}
func (h *Handler) Receipt(c *gin.Context) {
	ctx, ok := h.scope(c, false)
	if !ok {
		return
	}
	r, e := h.office.Receipt(ctx, c.Query("requestId"))
	if e != nil {
		writeError(c, e)
		return
	}
	c.JSON(200, r)
}
func (h *Handler) Changes(c *gin.Context) {
	ctx, ok := h.scope(c, false)
	if !ok {
		return
	}
	since, e := strconv.ParseUint(c.Query("since"), 10, 64)
	if e != nil && c.Query("since") != "" {
		writeError(c, ErrInvalidRequest)
		return
	}
	v, e := h.office.Changes(ctx, since)
	if e != nil {
		writeError(c, e)
		return
	}
	c.JSON(200, v)
}

func (h *Handler) Sources(c *gin.Context) {
	ctx, ok := h.scope(c, false)
	if !ok {
		return
	}
	v, err := h.office.ListSources(ctx)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(200, gin.H{"sources": v})
}

func (h *Handler) Upload(c *gin.Context) {
	ctx, ok := h.scope(c, true)
	if !ok {
		return
	}
	currentView, err := h.office.Open(ctx)
	if err != nil {
		writeError(c, err)
		return
	}
	maxSize := int64(secutils.GetMaxFileSizeMB()) * 1024 * 1024
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSize+1024*1024)
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		writeError(c, ErrInvalidRequest)
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxSize+1))
	if err != nil {
		c.JSON(400, gin.H{"error": gin.H{"code": "invalid_request", "message": "unable to read uploaded file"}})
		return
	}
	if int64(len(data)) > maxSize || header.Size > maxSize {
		c.JSON(400, gin.H{"error": gin.H{"code": "invalid_request", "message": "uploaded file exceeds size limit"}})
		return
	}
	tenantID, tenantOK := types.TenantIDFromContext(c.Request.Context())
	if !tenantOK {
		writeError(c, ErrUnauthorized)
		return
	}
	expectedRevision := currentView.Revision
	if rawRevision := c.Request.PostFormValue("expectedRevision"); rawRevision != "" {
		expectedRevision, err = strconv.ParseUint(rawRevision, 10, 64)
		if err != nil {
			writeError(c, ErrInvalidRequest)
			return
		}
	}
	requestID := strings.TrimSpace(c.Request.PostFormValue("requestId"))
	processingCreated := false
	result, err := h.upload.StoreAndParse(ctx, tenantID, header.Filename, header.Header.Get("Content-Type"), data, func(upload UploadResult) error {
		_, createErr := h.office.CreateProcessingSource(ctx, upload.Upload)
		processingCreated = createErr == nil
		return createErr
	})
	if err != nil {
		if result.SourceID != "" {
			failureCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			var failed CareerSource
			var createErr error
			if processingCreated {
				failed, createErr = h.office.FinishSource(failureCtx, result.SourceID, "", nil, nil, err)
			} else {
				failed, createErr = h.office.CreateFailedSource(failureCtx, result.Upload, err.Error())
			}
			if createErr != nil {
				writeError(c, createErr)
				return
			}
			c.JSON(200, UploadResponse{Source: failed})
			return
		}
		writeError(c, ErrInvalidRequest)
		return
	}
	var receipt *Receipt
	if len(result.Fields) > 0 {
		if requestID == "" {
			requestID = result.SourceID + ":batch"
		}
		batch, batchErr := h.office.CompleteIntake(ctx, result.SourceID, result.Upload.Text, result.Fields, result.MissingCategories, result.ReviewFlags, requestID, expectedRevision)
		if batchErr != nil {
			if !errors.Is(batchErr, ErrOutcomeUnknown) {
				failureCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
				_, _ = h.office.FinishSource(failureCtx, result.SourceID, "", nil, nil, batchErr)
				_ = h.upload.Release(failureCtx, result.Upload.ResourceRef, result.SourceID)
				cancel()
			}
			writeError(c, batchErr)
			return
		}
		receipt = &batch
	}
	var source CareerSource
	if len(result.Fields) == 0 {
		source, err = h.office.FinishSource(ctx, result.SourceID, result.Upload.Text, result.MissingCategories, result.ReviewFlags, nil)
	} else {
		source, err = h.office.GetSource(ctx, result.SourceID)
	}
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(201, UploadResponse{Source: source, Receipt: receipt})
}
