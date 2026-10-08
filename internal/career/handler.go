package career

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
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

// NewHandler wires the Career HTTP surface. The linker is the Workbench
// boundary for durable application tasks; Career never imports Workbench
// repositories directly.
func NewHandler(db *gorm.DB, members interfaces.TenantMemberService, files interfaces.FileService, catalog interfaces.ResourceCatalog, reader interfaces.DocumentReader, linker interfaces.CareerApplicationTaskLinker, remover interfaces.CareerApplicationTaskProjectionRemover) (*Handler, error) {
	o, e := NewOffice(db)
	if e != nil {
		return nil, e
	}
	o.SetApplicationTaskLinker(linker)
	o.SetApplicationTaskRemover(remover)
	if files != nil {
		o.SetExportStorage(newFileExportStorage(files))
	}
	if key, keyErr := ExportSigningKeyFromEnv(); keyErr == nil {
		o.SetExportSigningKey(key)
	}
	upload := NewUploadAdapter(files, catalog, reader)
	// Uploaded originals must be released through the catalog seam when the
	// whole space is deleted; the purge step calls this before clearing rows.
	o.SetSourceUploadReleaser(upload)
	return &Handler{office: o, members: members, upload: upload}, nil
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
	case errors.Is(e, ErrCareerOperationsBusy):
		status = http.StatusConflict
		code = "processing"
	case errors.Is(e, ErrInvalidRequest):
		status = 400
		code = "invalid_request"
	case errors.Is(e, ErrReceiptNotFound), errors.Is(e, ErrProposalNotFound), errors.Is(e, ErrSourceNotFound), errors.Is(e, ErrOpportunityNotFound), errors.Is(e, ErrEvaluationNotFound), errors.Is(e, ErrApplicationNotFound):
		status = 404
		code = "not_found"
	case errors.Is(e, ErrProposalResolved):
		status = 409
		code = "proposal_resolved"
	case errors.Is(e, ErrApplicationConflict):
		status = 409
		code = "application_conflict"
	case errors.Is(e, ErrApplicationHardIneligible):
		status = 409
		code = "hard_ineligible_requires_continue"
	case errors.Is(e, ErrOutcomeUnknown):
		status = 504
		code = "outcome_unknown"
	case errors.Is(e, ErrSearchNotFound):
		status = 404
		code = "not_found"
	case errors.Is(e, ErrRuleNotFound):
		status = 404
		code = "not_found"
	case errors.Is(e, ErrSearchQuotaRefused):
		status = http.StatusTooManyRequests
		code = "search_quota_refused"
	case errors.Is(e, ErrAdmissionUnavailable):
		status = http.StatusServiceUnavailable
		code = "admission_unavailable"
	case errors.Is(e, ErrMaterialNotFound), errors.Is(e, ErrMaterialVersionNotFound):
		status = 404
		code = "not_found"
	case errors.Is(e, ErrMaterialClaimUnconfirmed):
		status = 409
		code = "material_claim_unconfirmed"
	case errors.Is(e, ErrProgressEventNotFound):
		status = 404
		code = "not_found"
	case errors.Is(e, ErrSubmissionNotFound):
		status = 404
		code = "not_found"
	case errors.Is(e, ErrSubmissionAlreadyConfirmed):
		status = 409
		code = "submission_already_confirmed"
	case errors.Is(e, ErrExportNotFound):
		status = 404
		code = "not_found"
	case errors.Is(e, ErrDeletionNotFound):
		status = 404
		code = "not_found"
	case errors.Is(e, ErrExportNotSubmittable):
		status = 409
		code = "export_not_submittable"
	case errors.Is(e, ErrPreparationVersionUnknown):
		status = 409
		code = "preparation_version_unknown"
	case errors.Is(e, ErrPreparationGenerationFailed):
		status = 500
		code = "preparation_generation_failed"
	case errors.Is(e, ErrReminderNotFound), errors.Is(e, ErrReminderSourceNotFound):
		status = 404
		code = "not_found"
	case errors.Is(e, ErrExportGrantInvalid):
		status = 404
		code = "export_grant_invalid"
	case errors.Is(e, ErrExportStorageUnavailable):
		status = 500
		code = "export_storage_unavailable"
	case errors.Is(e, ErrExportSigningKeyMissing):
		status = http.StatusNotImplemented
		code = "export_signing_key_missing"
	}
	message := e.Error()
	if status == http.StatusInternalServerError && code == "internal" {
		slog.Error("unmapped career office error", "error", e)
		message = "internal career office error"
	}
	body := gin.H{"code": code, "message": message}
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
	if errors.Is(e, ErrPreparationVersionUnknown) {
		var prompt *PreparationVersionUnknownError
		if errors.As(e, &prompt) {
			body["applicationId"] = prompt.ApplicationID
			body["submissionRecorded"] = prompt.SubmissionRecorded
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
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxProfileActionBodyBytes)
	if e := c.ShouldBindJSON(&req); e != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(e, &maxBytesError) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": gin.H{"code": "request_too_large", "message": "profile action request is too large"}})
			return
		}
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

const maxImportURLBodyBytes = 16 * 1024

// ImportURL records one URL import attempt with truthful source integrity.
// The office layer performs all network I/O outside any database transaction
// and reconciles through a durable receipt; failures surface as bounded
// classifications, never raw upstream errors.
func (h *Handler) ImportURL(c *gin.Context) {
	ctx, ok := h.scope(c, true)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxImportURLBodyBytes)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	var input ImportURLInput
	if err := decoder.Decode(&input); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": gin.H{"code": "request_too_large", "message": "URL import request is too large"}})
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
	receipt, err := h.office.ImportURLReceipt(ctx, input)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, receipt)
}

// OpportunityObservations lists the immutable observation history of one
// opportunity under the current owner's authenticated scope.
func (h *Handler) OpportunityObservations(c *gin.Context) {
	ctx, ok := h.scope(c, false)
	if !ok {
		return
	}
	observations, err := h.office.OpportunityObservations(ctx, c.Param("opportunityId"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"observations": observations})
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

const maxReconcileBodyBytes = 4 * 1024

// ReconcileOpportunities decides one duplicate pair under the frozen identity
// evidence rules: a sufficient posting-code/company/location/batch match
// merges the candidate into the target; an uncertain pair stays side by side
// with an explicit suspected-duplicate flag. The decision is durable per
// request ID and never deletes an original observation.
func (h *Handler) ReconcileOpportunities(c *gin.Context) {
	ctx, ok := h.scope(c, false)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxReconcileBodyBytes)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	var input ReconcileInput
	if err := decoder.Decode(&input); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": gin.H{"code": "request_too_large", "message": "reconciliation request is too large"}})
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
	receipt, err := h.office.ReconcileOpportunities(ctx, input)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, receipt)
}

// OpportunityStatus projects one opportunity's explicit status: expiry,
// delisting, and requirement-change annotations plus the stale window of the
// last successful source check and the full immutable observation history.
func (h *Handler) OpportunityStatus(c *gin.Context) {
	ctx, ok := h.scope(c, false)
	if !ok {
		return
	}
	status, err := h.office.OpportunityStatus(ctx, c.Param("opportunityId"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, status)
}

// OpportunityReconciliationsHandler lists the durable reconciliation
// decisions that involved one opportunity.
func (h *Handler) OpportunityReconciliationsHandler(c *gin.Context) {
	ctx, ok := h.scope(c, false)
	if !ok {
		return
	}
	decisions, err := h.office.OpportunityReconciliations(ctx, c.Param("opportunityId"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"reconciliations": decisions})
}

// ReconciliationReceipt replays a stored reconciliation decision by request ID.
func (h *Handler) ReconciliationReceipt(c *gin.Context) {
	ctx, ok := h.scope(c, false)
	if !ok {
		return
	}
	receipt, err := h.office.FindReconciliationReceipt(ctx, c.Query("requestId"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, receipt)
}

// SourceCoverage discloses the honest coverage facts of this space: the
// vetted source registry (production starts empty) plus the sources and
// cities actually observed. Nothing is invented here.
func (h *Handler) SourceCoverage(c *gin.Context) {
	ctx, ok := h.scope(c, false)
	if !ok {
		return
	}
	coverage, err := h.office.SourceCoverage(ctx)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, coverage)
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

// CreateApplication pins application evidence and links one durable Workbench
// task per job and batch. The Career intent row is durable before any
// external call, so an unknown outcome stays recoverable through reconcile.
func (h *Handler) CreateApplication(c *gin.Context) {
	ctx, ok := h.scope(c, false)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16*1024)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	var input CreateApplicationInput
	if err := decoder.Decode(&input); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": gin.H{"code": "request_too_large", "message": "application request is too large"}})
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
	receipt, err := h.office.CreateApplication(ctx, input)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, receipt)
}

func (h *Handler) ApplicationReceipt(c *gin.Context) {
	ctx, ok := h.scope(c, false)
	if !ok {
		return
	}
	receipt, err := h.office.FindApplicationReceipt(ctx, c.Query("requestId"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, receipt)
}

func (h *Handler) GetApplication(c *gin.Context) {
	ctx, ok := h.scope(c, false)
	if !ok {
		return
	}
	receipt, err := h.office.Application(ctx, c.Param("applicationId"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, receipt)
}

// ReconcileApplicationLink resolves an undecided application link with the
// original request ID; it never changes the pinned intent.
func (h *Handler) ReconcileApplicationLink(c *gin.Context) {
	ctx, ok := h.scope(c, false)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16*1024)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	var req struct {
		RequestID string `json:"requestId"`
	}
	if err := decoder.Decode(&req); err != nil {
		writeError(c, ErrInvalidRequest)
		return
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		writeError(c, ErrInvalidRequest)
		return
	}
	receipt, err := h.office.ReconcileApplicationLink(ctx, req.RequestID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, receipt)
}

const maxSearchBodyBytes = 16 * 1024

// SearchOnce runs exactly one durable job search per request ID. It never
// creates a continuous rule; sources are only reached through the vetted
// policy/transport seams and the receipt reports the truthful coverage.
func (h *Handler) SearchOnce(c *gin.Context) {
	ctx, ok := h.scope(c, true)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSearchBodyBytes)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	var input SearchOnceInput
	if err := decoder.Decode(&input); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": gin.H{"code": "request_too_large", "message": "search request is too large"}})
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
	receipt, err := h.office.SearchOnce(ctx, input)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, receipt)
}

// SearchReceipt replays the terminal receipt of a one-shot search by its
// original request ID.
func (h *Handler) SearchReceipt(c *gin.Context) {
	ctx, ok := h.scope(c, false)
	if !ok {
		return
	}
	receipt, err := h.office.FindSearchReceipt(ctx, c.Query("requestId"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, receipt)
}

// GetSearch serves the stored result set of one durable search by its ID.
func (h *Handler) GetSearch(c *gin.Context) {
	ctx, ok := h.scope(c, false)
	if !ok {
		return
	}
	receipt, err := h.office.Search(ctx, c.Param("searchId"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, receipt)
}

// UsageEstimate answers, before anything executes, what the next charged run
// of one operation would consume and under which conditions, together with
// the live balance of the current window. It is a free read-only projection:
// an unreadable ledger is the typed 503 admission_unavailable (never
// execute-first), and an unknown operation is the typed 400 invalid_request.
func (h *Handler) UsageEstimate(c *gin.Context) {
	ctx, ok := h.scope(c, false)
	if !ok {
		return
	}
	view, err := h.office.UsageEstimate(ctx, c.Query("operation"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, view)
}

const maxRuleBodyBytes = 16 * 1024

// SetRule creates or updates one user-controlled recurring search rule. The
// rule never runs on its own here: triggering is the explicit office-level
// TriggerDueRules seam, so nothing in this HTTP path schedules background
// work. The receipt surfaces the conditions, the frequency, and the
// deterministic estimated consumption before any run happens.
func (h *Handler) SetRule(c *gin.Context) {
	ctx, ok := h.scope(c, true)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxRuleBodyBytes)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	var input SetRuleInput
	if err := decoder.Decode(&input); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": gin.H{"code": "request_too_large", "message": "rule request is too large"}})
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
	receipt, err := h.office.SetRule(ctx, input)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, receipt)
}

// RuleReceipt replays a stored set_rule receipt by its original request ID.
func (h *Handler) RuleReceipt(c *gin.Context) {
	ctx, ok := h.scope(c, false)
	if !ok {
		return
	}
	receipt, err := h.office.FindRuleReceipt(ctx, c.Query("requestId"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, receipt)
}

// ListRules serves a bounded owner-scoped page of rule summaries.
func (h *Handler) ListRules(c *gin.Context) {
	ctx, ok := h.scope(c, false)
	if !ok {
		return
	}
	page, err := h.office.ListRules(ctx, c.Query("cursor"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, page)
}

// GetRule serves the live rule contract: configuration, deterministic
// estimate, full run history (including blocked statuses), and discovery
// todos.
func (h *Handler) GetRule(c *gin.Context) {
	ctx, ok := h.scope(c, false)
	if !ok {
		return
	}
	view, err := h.office.Rule(ctx, c.Param("ruleId"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, view)
}

const maxMaterialBodyBytes = 256 * 1024

// EditMaterial creates or edits one structured material draft. Creation
// freezes the opportunity snapshot and profile revision; claims may only
// reference confirmed facts, and a refused edit keeps the prior draft.
func (h *Handler) EditMaterial(c *gin.Context) {
	ctx, ok := h.scope(c, false)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxMaterialBodyBytes)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	var input EditMaterialInput
	if err := decoder.Decode(&input); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": gin.H{"code": "request_too_large", "message": "material request is too large"}})
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
	receipt, err := h.office.EditMaterial(ctx, input)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, receipt)
}

// ConfirmMaterialBody confirms the current draft as the next immutable
// version; no earlier version is ever rewritten.
func (h *Handler) ConfirmMaterialBody(c *gin.Context) {
	ctx, ok := h.scope(c, false)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16*1024)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	var input ConfirmMaterialInput
	if err := decoder.Decode(&input); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": gin.H{"code": "request_too_large", "message": "material confirm request is too large"}})
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
	receipt, err := h.office.ConfirmMaterial(ctx, input)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, receipt)
}

// MaterialReceiptHandler replays a stored material receipt by request ID.
func (h *Handler) MaterialReceiptHandler(c *gin.Context) {
	ctx, ok := h.scope(c, false)
	if !ok {
		return
	}
	receipt, err := h.office.FindMaterialReceipt(ctx, c.Query("requestId"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, receipt)
}

// GetMaterial serves one material with its draft, review risks, and version
// history under the authenticated scope.
func (h *Handler) GetMaterial(c *gin.Context) {
	ctx, ok := h.scope(c, false)
	if !ok {
		return
	}
	view, err := h.office.Material(ctx, c.Param("materialId"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, view)
}

// ListMaterialVersions serves the immutable version history of one material.
func (h *Handler) ListMaterialVersions(c *gin.Context) {
	ctx, ok := h.scope(c, false)
	if !ok {
		return
	}
	versions, err := h.office.MaterialVersions(ctx, c.Param("materialId"))
	if err != nil {
		writeError(c, err)
		return
	}
	if versions == nil {
		versions = []MaterialVersionSummary{}
	}
	c.JSON(http.StatusOK, gin.H{"materialId": c.Param("materialId"), "versions": versions})
}

// GetMaterialVersion serves one immutable version by its number.
func (h *Handler) GetMaterialVersion(c *gin.Context) {
	ctx, ok := h.scope(c, false)
	if !ok {
		return
	}
	version, err := strconv.ParseUint(c.Param("versionId"), 10, 64)
	if err != nil || version == 0 {
		writeError(c, ErrInvalidRequest)
		return
	}
	view, err := h.office.MaterialVersion(ctx, c.Param("materialId"), version)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, view)
}

// CompareMaterialVersions serves the honest diff between an old (baseline)
// version and the target version.
func (h *Handler) CompareMaterialVersions(c *gin.Context) {
	ctx, ok := h.scope(c, false)
	if !ok {
		return
	}
	target, err := strconv.ParseUint(c.Param("versionId"), 10, 64)
	if err != nil || target == 0 {
		writeError(c, ErrInvalidRequest)
		return
	}
	baseline, err := strconv.ParseUint(c.Query("baseline"), 10, 64)
	if err != nil || baseline == 0 {
		writeError(c, ErrInvalidRequest)
		return
	}
	comparison, err := h.office.CompareMaterialVersions(ctx, c.Param("materialId"), baseline, target)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, comparison)
}

const maxExportBodyBytes = 16 * 1024

func decodeStrictJSON(c *gin.Context, target any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxExportBodyBytes)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": gin.H{"code": "request_too_large", "message": "material export request is too large"}})
			return false
		}
		writeError(c, ErrInvalidRequest)
		return false
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		writeError(c, ErrInvalidRequest)
		return false
	}
	return true
}

// PublishMaterialHandler renders and verifies the PDF/DOCX pair of one
// immutable material version. The material comes from the authenticated path;
// only both-verified exports become submittable.
func (h *Handler) PublishMaterialHandler(c *gin.Context) {
	ctx, ok := h.scope(c, false)
	if !ok {
		return
	}
	var input PublishMaterialInput
	if !decodeStrictJSON(c, &input) {
		return
	}
	input.MaterialID = c.Param("materialId")
	receipt, err := h.office.PublishMaterial(ctx, input)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, receipt)
}

// ListMaterialExports serves every export of one material under the
// authenticated scope.
func (h *Handler) ListMaterialExports(c *gin.Context) {
	ctx, ok := h.scope(c, false)
	if !ok {
		return
	}
	exports, err := h.office.MaterialExports(ctx, c.Param("materialId"))
	if err != nil {
		writeError(c, err)
		return
	}
	if exports == nil {
		exports = []ExportReceipt{}
	}
	c.JSON(http.StatusOK, gin.H{"materialId": c.Param("materialId"), "exports": exports})
}

// MaterialExportSignedURL issues a short-lived download grant for one format
// of a submittable export.
func (h *Handler) MaterialExportSignedURL(c *gin.Context) {
	ctx, ok := h.scope(c, false)
	if !ok {
		return
	}
	var req struct {
		Format     string `json:"format"`
		TTLSeconds int64  `json:"ttlSeconds"`
	}
	if !decodeStrictJSON(c, &req) {
		return
	}
	if req.TTLSeconds <= 0 {
		writeError(c, ErrInvalidRequest)
		return
	}
	download, err := h.office.MaterialExportGrant(ctx, c.Param("materialId"), c.Param("exportId"), req.Format, time.Duration(req.TTLSeconds)*time.Second)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, download)
}

var exportContentTypes = map[string]string{
	ExportFormatPDF:  "application/pdf",
	ExportFormatDOCX: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
}

// DownloadMaterialExport redeems a download grant and streams the stored
// bytes; the durable export state is re-checked at redemption time.
func (h *Handler) DownloadMaterialExport(c *gin.Context) {
	ctx, ok := h.scope(c, false)
	if !ok {
		return
	}
	format := c.Query("format")
	data, err := h.office.DownloadMaterialExport(ctx, c.Param("materialId"), c.Param("exportId"), format, c.Query("expires"), c.Query("signature"))
	if err != nil {
		writeError(c, err)
		return
	}
	contentType := exportContentTypes[format]
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="career-material.%s"`, format))
	c.Data(http.StatusOK, contentType, data)
}

// RevokeMaterialExport revokes one export; already-issued grants fail
// immediately afterwards.
func (h *Handler) RevokeMaterialExport(c *gin.Context) {
	ctx, ok := h.scope(c, false)
	if !ok {
		return
	}
	var input RevokeMaterialExportInput
	if !decodeStrictJSON(c, &input) {
		return
	}
	input.MaterialID = c.Param("materialId")
	input.ExportID = c.Param("exportId")
	receipt, err := h.office.RevokeMaterialExport(ctx, input)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, receipt)
}

const maxProgressBodyBytes = 16 * 1024

// progressClientSource whitelists the provenance an HTTP caller may claim:
// only user entry (manual). Server-side imports claim their own enum value
// inside the office, never through this seam.
func progressClientSource(source Source) bool {
	return (source.Kind == "manual" || source.Kind == "user") && source.ReferenceID == ""
}

func bindProgressApplication(c *gin.Context, bodyApplicationID string) (string, bool) {
	applicationID := c.Param("applicationId")
	if bodyApplicationID != "" && bodyApplicationID != applicationID {
		writeError(c, ErrInvalidRequest)
		return "", false
	}
	return applicationID, true
}

// AppendProgress appends one immutable progress event to the application named
// in the path. The confirmer is the authenticated user; provenance outside the
// manual whitelist is rejected.
func (h *Handler) AppendProgress(c *gin.Context) {
	ctx, ok := h.scope(c, false)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxProgressBodyBytes)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	var input AppendProgressInput
	if err := decoder.Decode(&input); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": gin.H{"code": "request_too_large", "message": "progress request is too large"}})
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
	if !progressClientSource(input.Source) {
		writeError(c, ErrInvalidRequest)
		return
	}
	input.Source = Source{Kind: "manual"}
	applicationID, ok := bindProgressApplication(c, input.ApplicationID)
	if !ok {
		return
	}
	input.ApplicationID = applicationID
	receipt, err := h.office.AppendProgress(ctx, input)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, receipt)
}

// CorrectProgress appends a correction event referencing the corrected event;
// the original history row is never rewritten.
func (h *Handler) CorrectProgress(c *gin.Context) {
	ctx, ok := h.scope(c, false)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxProgressBodyBytes)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	var input CorrectProgressInput
	if err := decoder.Decode(&input); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": gin.H{"code": "request_too_large", "message": "progress correction request is too large"}})
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
	if !progressClientSource(input.Source) {
		writeError(c, ErrInvalidRequest)
		return
	}
	input.Source = Source{Kind: "manual"}
	applicationID, ok := bindProgressApplication(c, input.ApplicationID)
	if !ok {
		return
	}
	input.ApplicationID = applicationID
	receipt, err := h.office.CorrectProgress(ctx, input)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, receipt)
}

// ProgressReceiptHandler replays a stored progress receipt by request ID.
func (h *Handler) ProgressReceiptHandler(c *gin.Context) {
	ctx, ok := h.scope(c, false)
	if !ok {
		return
	}
	receipt, err := h.office.FindProgressReceipt(ctx, c.Query("requestId"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, receipt)
}

// ApplicationProgress serves the immutable event history of one application
// plus the deterministic stage projection.
func (h *Handler) ApplicationProgress(c *gin.Context) {
	ctx, ok := h.scope(c, false)
	if !ok {
		return
	}
	view, err := h.office.ApplicationProgress(ctx, c.Param("applicationId"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, view)
}

const maxSubmissionBodyBytes = 16 * 1024

// RecordSubmission persists the user-confirmed submission fact of the
// application named in the path: the channel they picked, the time they
// claim, and either a submittable material export or the explicit unknown
// marker. It never performs or infers any external action.
func (h *Handler) RecordSubmission(c *gin.Context) {
	ctx, ok := h.scope(c, false)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSubmissionBodyBytes)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	var input RecordSubmissionInput
	if err := decoder.Decode(&input); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": gin.H{"code": "request_too_large", "message": "submission request is too large"}})
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
	applicationID, ok := bindProgressApplication(c, input.ApplicationID)
	if !ok {
		return
	}
	input.ApplicationID = applicationID
	receipt, err := h.office.RecordSubmission(ctx, input)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, receipt)
}

// ApplicationSubmissions serves the submissions of one application under the
// authenticated scope.
func (h *Handler) ApplicationSubmissions(c *gin.Context) {
	ctx, ok := h.scope(c, false)
	if !ok {
		return
	}
	submissions, err := h.office.ApplicationSubmissions(ctx, c.Param("applicationId"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"submissions": submissions})
}

// SubmissionReceiptHandler replays a stored submission receipt by request ID.
func (h *Handler) SubmissionReceiptHandler(c *gin.Context) {
	ctx, ok := h.scope(c, false)
	if !ok {
		return
	}
	receipt, err := h.office.FindSubmissionReceipt(ctx, c.Query("requestId"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, receipt)
}

const maxPreparationBodyBytes = 4 * 1024

// GeneratePreparationHandler anchors to the application's actually
// submitted version and composes the cover letter / interview draft. An
// unconfirmed or missing submitted version answers the typed prompt state;
// the composed draft is reviewable through the material seams.
func (h *Handler) GeneratePreparationHandler(c *gin.Context) {
	ctx, ok := h.scope(c, false)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxPreparationBodyBytes)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	var input GeneratePreparationInput
	if err := decoder.Decode(&input); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": gin.H{"code": "request_too_large", "message": "preparation request is too large"}})
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
	applicationID, ok := bindProgressApplication(c, input.ApplicationID)
	if !ok {
		return
	}
	input.ApplicationID = applicationID
	receipt, err := h.office.GeneratePreparation(ctx, input)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, receipt)
}

// ApplicationPreparations serves the preparations of one application under
// the authenticated scope, failed requests included.
func (h *Handler) ApplicationPreparations(c *gin.Context) {
	ctx, ok := h.scope(c, false)
	if !ok {
		return
	}
	preparations, err := h.office.ApplicationPreparations(ctx, c.Param("applicationId"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"preparations": preparations})
}

// PreparationReceiptHandler replays a stored preparation receipt by request
// ID; a failed or interrupted generation answers its typed failure state.
func (h *Handler) PreparationReceiptHandler(c *gin.Context) {
	ctx, ok := h.scope(c, false)
	if !ok {
		return
	}
	receipt, err := h.office.FindPreparationReceipt(ctx, c.Query("requestId"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, receipt)
}

const maxReminderBodyBytes = 4 * 1024

// SetReminderHandler produces the in-station todo for one source event. The
// payload is closed (request ID + source + expected revision); the response
// carries the frozen privacy notice and, when a push channel exists, its
// best-effort report — a delivery failure never fails this request.
func (h *Handler) SetReminderHandler(c *gin.Context) {
	ctx, ok := h.scope(c, false)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxReminderBodyBytes)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	var input SetReminderInput
	if err := decoder.Decode(&input); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": gin.H{"code": "request_too_large", "message": "reminder request is too large"}})
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
	receipt, err := h.office.SetReminder(ctx, input)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, receipt)
}

// ListRemindersHandler serves the authoritative in-station todo list.
func (h *Handler) ListRemindersHandler(c *gin.Context) {
	ctx, ok := h.scope(c, false)
	if !ok {
		return
	}
	reminders, err := h.office.ListReminders(ctx)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"reminders": reminders})
}

// ReminderReceiptHandler replays a stored reminder receipt by request ID.
func (h *Handler) ReminderReceiptHandler(c *gin.Context) {
	ctx, ok := h.scope(c, false)
	if !ok {
		return
	}
	receipt, err := h.office.FindReminderReceipt(ctx, c.Query("requestId"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, receipt)
}

const maxExportDeletionBodyBytes = 16 * 1024

func decodeCareerLifecycleJSON(c *gin.Context, target any, label string) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxExportDeletionBodyBytes)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": gin.H{"code": "request_too_large", "message": label + " request is too large"}})
			return false
		}
		writeError(c, ErrInvalidRequest)
		return false
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		writeError(c, ErrInvalidRequest)
		return false
	}
	return true
}

// ExportCareerHandler runs the closed export_career intent: one complete,
// owner-scoped export package per request ID, synchronous, digest-verifiable.
func (h *Handler) ExportCareerHandler(c *gin.Context) {
	ctx, ok := h.scope(c, false)
	if !ok {
		return
	}
	var input CareerExportInput
	if !decodeCareerLifecycleJSON(c, &input, "career export") {
		return
	}
	receipt, err := h.office.ExportCareer(ctx, input)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, receipt)
}

// ExportCareerReceiptHandler replays one stored export receipt by request ID.
func (h *Handler) ExportCareerReceiptHandler(c *gin.Context) {
	ctx, ok := h.scope(c, false)
	if !ok {
		return
	}
	receipt, err := h.office.FindCareerExport(ctx, c.Query("requestId"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, receipt)
}

// CareerDeletionBoundaryHandler explains, before any deletion, the in-space
// versus external-platform boundary and the disclosed retention rows.
func (h *Handler) CareerDeletionBoundaryHandler(c *gin.Context) {
	ctx, ok := h.scope(c, false)
	if !ok {
		return
	}
	boundary, err := h.office.CareerDeletionBoundary(ctx)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, boundary)
}

// DeleteCareerHandler runs the closed delete_career intent. Partial failures
// answer with the truthful partial receipt, never a success claim.
func (h *Handler) DeleteCareerHandler(c *gin.Context) {
	ctx, ok := h.scope(c, false)
	if !ok {
		return
	}
	var input CareerDeletionInput
	if !decodeCareerLifecycleJSON(c, &input, "career deletion") {
		return
	}
	receipt, err := h.office.DeleteCareer(ctx, input)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, receipt)
}

// CareerDeletionReceiptHandler replays the durable deletion receipt (status,
// steps, retention) by its original request ID.
func (h *Handler) CareerDeletionReceiptHandler(c *gin.Context) {
	ctx, ok := h.scope(c, false)
	if !ok {
		return
	}
	receipt, err := h.office.FindCareerDeletion(ctx, c.Query("requestId"))
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
	h.cleanupSourcesBounded(ctx, v)
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
		h.cleanupSourcesBounded(ctx, sources)
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
	scope, scopeErr := getScope(ctx)
	if scopeErr != nil {
		writeError(c, scopeErr)
		return
	}
	ownerToken, unlockAttempt, guardErr := h.office.acquireLifecycleClaim(ctx, scope, "source_upload", requestID, hex.EncodeToString(intentSum[:]))
	if guardErr != nil {
		if errors.Is(guardErr, ErrCareerOperationsBusy) {
			prior, found, pendingErr := h.office.FindUploadClaim(ctx, requestID)
			if pendingErr == nil && found {
				if prior.Digest != digest || prior.FileName != baseName || prior.MIMEType != declaredMIME || prior.ExpectedRevision != expectedRevision {
					writeError(c, ErrIdempotencyConflict)
					return
				}
				if pending, exists, lookupErr := h.office.FindUploadSource(ctx, requestID); lookupErr == nil && exists && pending.Status == "processing" {
					c.JSON(http.StatusAccepted, UploadResponse{Source: pending})
					return
				}
			}
		}
		writeError(c, guardErr)
		return
	}
	defer unlockAttempt()
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
		if releaseErr := h.office.resolveLifecycleClaimOwned(context.Background(), scope, "source_upload", requestID, ownerToken); releaseErr != nil {
			writeError(c, &OutcomeUnknownError{RequestID: requestID})
			return
		}
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
		recovered, recoverErr := h.recoverCatalogRef(ctx, claim.ID, claim.ClaimToken, requestID, ownerToken)
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
			persistErr := h.office.PersistUploadResourceOwned(ctx, upload.SourceID, claim.ClaimToken, ownerToken, upload.Upload.ResourceRef)
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
		if releaseErr := h.office.resolveLifecycleClaimOwned(context.Background(), scope, "source_upload", requestID, ownerToken); releaseErr != nil {
			writeError(c, &OutcomeUnknownError{RequestID: requestID})
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
						if releaseErr := h.office.resolveLifecycleClaimOwned(context.Background(), scope, "source_upload", requestID, ownerToken); releaseErr != nil {
							writeError(c, &OutcomeUnknownError{RequestID: requestID})
							return
						}
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
				if finishErr == nil {
					if releaseErr := h.office.resolveLifecycleClaimOwned(context.Background(), scope, "source_upload", requestID, ownerToken); releaseErr != nil {
						writeError(c, &OutcomeUnknownError{RequestID: requestID})
						return
					}
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
	if releaseErr := h.office.resolveLifecycleClaimOwned(context.Background(), scope, "source_upload", requestID, ownerToken); releaseErr != nil {
		writeError(c, &OutcomeUnknownError{RequestID: requestID})
		return
	}
	c.JSON(201, UploadResponse{Source: source, Receipt: receipt})
}

func (h *Handler) reconcileStaleSources(ctx context.Context) error {
	return h.reconcileStaleSourcesExcept(ctx, "")
}

// cleanupSourcesBounded runs catalog cleanup for every source under ONE
// detached context with a small shared budget, mirroring
// reconcileStaleSourcesExcept: a stalled storage backend can never hold the
// caller's request hostage, and the N sources share one bounded window
// instead of one unbounded request context each.
func (h *Handler) cleanupSourcesBounded(ctx context.Context, sources []CareerSource) {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	for _, source := range sources {
		if cleanupErr := h.cleanupCatalogCandidates(cleanupCtx, source.ID); cleanupErr != nil {
			slog.Warn("career catalog cleanup pending", "source_id", source.ID)
		}
	}
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
		scope, scopeErr := getScope(cleanupCtx)
		if scopeErr != nil {
			continue
		}
		ownerToken, unlock, guardErr := h.office.acquireLifecycleClaim(cleanupCtx, scope, "source_upload", resource.RequestID, resource.IntentHash)
		if guardErr != nil {
			continue
		}
		if err = h.upload.Release(cleanupCtx, resource.ResourceRef, resource.ID); err != nil {
			slog.Warn("career source cleanup pending", "source_id", resource.ID, "stage", "release")
			unlock()
			continue
		}
		if err = h.office.ClearSourceResource(cleanupCtx, resource.ID, resource.ClaimToken, resource.ResourceRef, resource.FinalErrorCategory); err != nil {
			slog.Warn("career source cleanup state pending", "source_id", resource.ID, "stage", "state_update")
			unlock()
			continue
		}
		if err = h.office.resolveLifecycleClaimOwned(cleanupCtx, scope, "source_upload", resource.RequestID, ownerToken); err != nil {
			slog.Warn("career source lifecycle cleanup pending", "source_id", resource.ID)
		}
		unlock()
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
		if e := h.upload.Release(detached, source.ResourceRef, source.ID); e != nil {
			return source, false, e
		} else {
			if e = h.office.ClearSourceResource(detached, source.ID, token, source.ResourceRef, final); e != nil {
				if errors.Is(e, ErrUploadClaimLost) {
					latest, readErr := h.office.GetSource(detached, id)
					return latest, true, readErr
				}
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
