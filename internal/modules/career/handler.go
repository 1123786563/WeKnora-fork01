package career

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
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
	case errors.Is(e, ErrReceiptNotFound), errors.Is(e, ErrProposalNotFound), errors.Is(e, ErrSourceNotFound), errors.Is(e, ErrOpportunityNotFound):
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
	// Client writes cannot claim parser provenance. Direct confirmation is
	// explicitly manual; proposal resolution keeps the stored proposal source.
	switch req.Action {
	case "propose":
		if (req.Source.Kind != "manual" && req.Source.Kind != "user") || req.Source.ReferenceID != "" {
			writeError(c, ErrInvalidRequest)
			return
		}
		req.Source = Source{Kind: "manual"}
	case "confirm":
		if (req.Source.Kind != "manual" && req.Source.Kind != "user") || req.Source.ReferenceID != "" {
			writeError(c, ErrInvalidRequest)
			return
		}
		req.Source = Source{Kind: "manual"}
	case "confirm_proposal", "dismiss":
		if (req.Source.Kind != "user_confirmation" && req.Source.Kind != "user") || req.Source.ReferenceID != "" {
			writeError(c, ErrInvalidRequest)
			return
		}
		req.Source = Source{Kind: "user_confirmation"}
	default:
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

// ImportJD stores user-pasted job text as inert evidence. It never dispatches
// the content to an Agent, tool, URL fetcher, or authorization decision.
func (h *Handler) ImportJD(c *gin.Context) {
	ctx, ok := h.scope(c, true)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxJDRequestBodyBytes)
	var req ImportJDInput
	if err := c.ShouldBindJSON(&req); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": gin.H{"code": "request_too_large", "message": "JD import request is too large"}})
			return
		}
		writeError(c, ErrInvalidRequest)
		return
	}
	receipt, err := h.office.ImportJD(ctx, req)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, receipt)
}

// OpportunityEvidence reopens one fixed snapshot under the current owner's
// authenticated Career scope.
func (h *Handler) OpportunityEvidence(c *gin.Context) {
	ctx, ok := h.scope(c, false)
	if !ok {
		return
	}
	evidence, err := h.office.OpportunityEvidence(ctx, c.Param("opportunityId"), c.Query("snapshotId"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, evidence)
}

func (h *Handler) OpportunityReceipt(c *gin.Context) {
	ctx, ok := h.scope(c, false)
	if !ok {
		return
	}
	receipt, err := h.office.FindOpportunityReceipt(ctx, c.Query("requestId"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, receipt)
}

// EvaluateOpportunity creates an immutable evaluation from a fixed JD snapshot
// and the current or explicitly pinned confirmed profile revision.
func (h *Handler) EvaluateOpportunity(c *gin.Context) {
	ctx, ok := h.scope(c, false)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16*1024)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	var input EvaluateInput
	if err := decoder.Decode(&input); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": gin.H{"code": "request_too_large", "message": "evaluation request is too large"}})
			return
		}
		writeError(c, ErrInvalidRequest)
		return
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		writeError(c, ErrInvalidRequest)
		return
	}
	receipt, err := h.office.EvaluateOpportunity(ctx, input)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, receipt)
}

func (h *Handler) Evaluation(c *gin.Context) {
	ctx, ok := h.scope(c, false)
	if !ok {
		return
	}
	evaluation, err := h.office.Evaluation(ctx, c.Param("evaluationId"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, evaluation)
}

func (h *Handler) EvaluationReceipt(c *gin.Context) {
	ctx, ok := h.scope(c, false)
	if !ok {
		return
	}
	receipt, err := h.office.FindEvaluationReceipt(ctx, c.Query("requestId"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, receipt)
}

func (h *Handler) Sources(c *gin.Context) {
	ctx, ok := h.scope(c, false)
	if !ok {
		return
	}
	if err := h.reconcileStaleSources(ctx); err != nil {
		writeError(c, err)
		return
	}
	v, err := h.office.ListSources(ctx)
	if err != nil {
		writeError(c, err)
		return
	}
	for _, source := range v {
		if cleanupErr := h.cleanupCatalogCandidates(ctx, source.ID); cleanupErr != nil {
			slog.Warn("career catalog cleanup pending", "source_id", source.ID)
		}
	}
	if refreshed, refreshErr := h.office.ListSources(ctx); refreshErr == nil {
		v = refreshed
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
	explicitExpectedRevision := false
	if rawRevision := c.Request.PostFormValue("expectedRevision"); rawRevision != "" {
		explicitExpectedRevision = true
		expectedRevision, err = strconv.ParseUint(rawRevision, 10, 64)
		if err != nil {
			writeError(c, ErrInvalidRequest)
			return
		}
	}
	requestID := strings.TrimSpace(c.Request.PostFormValue("requestId"))
	if requestID == "" || len(requestID) > 128 {
		writeError(c, ErrInvalidRequest)
		return
	}
	if err := h.reconcileStaleSourcesExcept(ctx, requestID); err != nil {
		writeError(c, err)
		return
	}
	if sources, listErr := h.office.ListSources(ctx); listErr == nil {
		for _, source := range sources {
			if cleanupErr := h.cleanupCatalogCandidates(ctx, source.ID); cleanupErr != nil {
				slog.Warn("career catalog cleanup pending", "source_id", source.ID)
			}
		}
	}
	safeName, nameValid := secutils.ValidateInput(strings.TrimSpace(header.Filename))
	if !nameValid {
		writeError(c, ErrInvalidRequest)
		return
	}
	baseName, nameErr := secutils.SafeFileName(safeName)
	if nameErr != nil {
		writeError(c, ErrInvalidRequest)
		return
	}
	digestBytes := sha256.Sum256(data)
	digest := hex.EncodeToString(digestBytes[:])
	declaredMIME := strings.TrimSpace(header.Header.Get("Content-Type"))
	if !explicitExpectedRevision {
		prior, found, e := h.office.FindUploadClaim(ctx, requestID)
		if e != nil {
			writeError(c, e)
			return
		}
		if found {
			if prior.Digest != digest || prior.FileName != baseName || prior.MIMEType != declaredMIME {
				writeError(c, ErrIdempotencyConflict)
				return
			}
			expectedRevision = prior.ExpectedRevision
		}
	}
	intentBytes, _ := json.Marshal([]any{baseName, declaredMIME})
	intentSum := sha256.Sum256(intentBytes)
	claim, terminal, claimErr := h.office.ClaimUpload(ctx, SourceUpload{FileName: baseName, MIMEType: declaredMIME, Size: int64(len(data)), Digest: digest, RequestID: requestID, IntentHash: hex.EncodeToString(intentSum[:]), ExpectedRevision: expectedRevision})
	if errors.Is(claimErr, ErrUploadInProgress) {
		c.JSON(202, UploadResponse{Source: claim})
		return
	}
	if claimErr != nil {
		writeError(c, claimErr)
		return
	}
	if terminal {
		var receipt *Receipt
		if claim.Status == "ready" {
			r, e := h.office.Receipt(ctx, claim.ID+":batch")
			if e == nil {
				receipt = &r
			}
		}
		c.JSON(200, UploadResponse{Source: claim, Receipt: receipt})
		return
	}
	var result UploadResult
	if claim.ResourceRef == "" {
		recovered, recoverErr := h.recoverCatalogRef(ctx, claim.ID, claim.ClaimToken, requestID)
		if errors.Is(recoverErr, ErrUploadClaimLost) {
			if latest, lookupErr := h.office.GetSource(ctx, claim.ID); lookupErr == nil {
				c.JSON(202, UploadResponse{Source: latest})
				return
			}
		}
		if recoverErr != nil {
			writeError(c, recoverErr)
			return
		}
		claim.ResourceRef = recovered
	}
	if claim.ResourceRef != "" {
		result, err = h.upload.ResumeAndParse(ctx, tenantID, claim.ID, baseName, declaredMIME, data, claim.ResourceRef)
	} else {
		result, err = h.upload.StoreAndParseWithID(ctx, tenantID, claim.ID, baseName, declaredMIME, data, func(upload UploadResult) error {
			persistErr := h.office.PersistUploadResource(ctx, upload.SourceID, claim.ClaimToken, upload.Upload.ResourceRef)
			if persistErr == nil {
				return nil
			}
			current, readErr := h.office.privateSource(context.WithoutCancel(ctx), claim.ID)
			if readErr == nil && current.ResourceRef == upload.Upload.ResourceRef {
				return nil
			}
			if errors.Is(persistErr, ErrUploadClaimLost) && readErr == nil {
				return ErrUploadClaimLost
			}
			return &OutcomeUnknownError{RequestID: requestID}
		})
	}
	if err != nil {
		if errors.Is(err, ErrOutcomeUnknown) {
			writeError(c, err)
			return
		}
		failed, superseded, finishErr := h.finishClaimFailure(ctx, claim.ID, claim.ClaimToken, err)
		if finishErr != nil {
			writeError(c, finishErr)
			return
		}
		if superseded {
			c.JSON(202, UploadResponse{Source: failed})
			return
		}
		c.JSON(200, UploadResponse{Source: failed})
		return
	}
	var receipt *Receipt
	if len(result.Fields) > 0 {
		batchID := result.SourceID + ":batch"
		batch, batchErr := h.office.CompleteIntakeClaim(ctx, result.SourceID, claim.ClaimToken, result.Upload.Text, result.Fields, result.MissingCategories, result.ReviewFlags, batchID, expectedRevision)
		if batchErr != nil {
			if errors.Is(batchErr, ErrOutcomeUnknown) {
				if source, sourceErr := h.office.GetSource(ctx, result.SourceID); sourceErr == nil && source.Status == "ready" {
					if receipt, receiptErr := h.office.Receipt(ctx, batchID); receiptErr == nil {
						c.JSON(201, UploadResponse{Source: source, Receipt: &receipt})
						return
					}
				}
			} else if errors.Is(batchErr, ErrUploadClaimLost) {
				if source, sourceErr := h.office.GetSource(ctx, result.SourceID); sourceErr == nil {
					c.JSON(202, UploadResponse{Source: source})
					return
				}
			} else {
				failed, superseded, finishErr := h.finishClaimFailure(ctx, claim.ID, claim.ClaimToken, batchErr)
				if finishErr == nil && superseded {
					c.JSON(202, UploadResponse{Source: failed})
					return
				}
			}
			writeError(c, batchErr)
			return
		}
		receipt = &batch
	}
	var source CareerSource
	if len(result.Fields) == 0 {
		source, err = h.office.FinishSourceClaim(ctx, result.SourceID, claim.ClaimToken, result.Upload.Text, result.MissingCategories, result.ReviewFlags, nil)
	} else {
		source, err = h.office.GetSource(ctx, result.SourceID)
	}
	if err != nil {
		if errors.Is(err, ErrUploadClaimLost) {
			if latest, e := h.office.GetSource(ctx, result.SourceID); e == nil {
				c.JSON(202, UploadResponse{Source: latest})
				return
			}
		}
		writeError(c, err)
		return
	}
	c.JSON(201, UploadResponse{Source: source, Receipt: receipt})
}

func (h *Handler) reconcileStaleSources(ctx context.Context) error {
	return h.reconcileStaleSourcesExcept(ctx, "")
}

func (h *Handler) reconcileStaleSourcesExcept(ctx context.Context, skipRequestID string) error {
	resources, err := h.office.FailStaleUploads(ctx, time.Now().UTC(), skipRequestID)
	if err != nil {
		return err
	}
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	for _, resource := range resources {
		if resource.ResourceRef == "" {
			continue
		}
		if err = h.upload.Release(cleanupCtx, resource.ResourceRef, resource.ID); err != nil {
			slog.Warn("career source cleanup pending", "source_id", resource.ID, "stage", "release")
			continue
		}
		if err = h.office.ClearSourceResource(cleanupCtx, resource.ID, resource.ClaimToken, resource.ResourceRef, resource.FinalErrorCategory); err != nil {
			slog.Warn("career source cleanup state pending", "source_id", resource.ID, "stage", "state_update")
		}
	}
	return nil
}

func (h *Handler) finishClaimFailure(ctx context.Context, id, token string, failure error) (CareerSource, bool, error) {
	detached, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	source, err := h.office.FinishSourceClaim(detached, id, token, "", nil, nil, failure)
	if errors.Is(err, ErrUploadClaimLost) {
		latest, e := h.office.GetSource(detached, id)
		return latest, true, e
	}
	if err != nil {
		return CareerSource{}, false, err
	}
	if source.ResourceRef != "" {
		final := strings.TrimPrefix(source.ErrorCategory, "cleanup_pending_")
		if e := h.upload.Release(detached, source.ResourceRef, source.ID); e == nil {
			if e = h.office.ClearSourceResource(detached, source.ID, token, source.ResourceRef, final); e != nil {
				return source, false, e
			}
			source, e = h.office.GetSource(detached, id)
			if e != nil {
				return CareerSource{}, false, e
			}
		}
	}
	return source, false, nil
}
