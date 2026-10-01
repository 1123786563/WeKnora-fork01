package handler

import (
	"encoding/json"
	stderrors "errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	marketrepo "github.com/Tencent/WeKnora/internal/application/repository"
	marketservice "github.com/Tencent/WeKnora/internal/application/service"
	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// PublicMarketplaceHandler is the HTTP boundary for the Public Marketplace
// (T30 #60): Verified Publisher registry (platform), public submissions,
// platform review, the public catalog and cross-tenant adoption. Tenant
// and actor identity always come from the authenticated request context;
// request bodies never carry principals (strict decoding rejects spoof
// attempts). No response of this handler carries adopter-derived data.
type PublicMarketplaceHandler struct {
	public interfaces.PublicMarketplaceService
}

func NewPublicMarketplaceHandler(public interfaces.PublicMarketplaceService) *PublicMarketplaceHandler {
	return &PublicMarketplaceHandler{public: public}
}

type verifyPublisherBody struct {
	TenantID uint64 `json:"tenant_id"`
	Note     string `json:"note,omitempty"`
}

type submitPublicReleaseBody struct {
	SourceListingID string `json:"source_listing_id"`
	ReleaseID       string `json:"release_id,omitempty"`
}

type adoptPublicListingBody struct {
	ReleaseID string `json:"release_id,omitempty"`
}

// recordPublicEvaluationBody is the closed request shape for platform
// Evaluation authoring: results must be the structured
// {status,checks:[{code,status}]} object, never freeform text, and the
// reviewer identity never arrives from the body.
type recordPublicEvaluationBody struct {
	ReleaseID        string `json:"release_id"`
	TestSetID        string `json:"test_set_id"`
	TestSetVersion   string `json:"test_set_version"`
	EnvironmentClass string `json:"environment_class"`
	Results          struct {
		Status string `json:"status"`
		Checks []struct {
			Code   string `json:"code"`
			Status string `json:"status"`
		} `json:"checks"`
	} `json:"results"`
}

type verifiedPublisherResponse struct {
	TenantID   uint64    `json:"tenant_id"`
	State      string    `json:"state"`
	VerifiedBy string    `json:"verified_by"`
	Note       string    `json:"note"`
	VerifiedAt time.Time `json:"verified_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type publicCatalogReleaseResponse struct {
	ID                       string          `json:"id"`
	SemanticVersion          string          `json:"semantic_version"`
	BundleDigest             string          `json:"bundle_digest"`
	Manifest                 json.RawMessage `json:"manifest"`
	DependencyLock           json.RawMessage `json:"dependency_lock"`
	MinimumWeKnoraCapability string          `json:"minimum_weknora_capability"`
	CapabilityRequirements   []string        `json:"capability_requirements"`
	LicenseID                string          `json:"license_id"`
	CreatedAt                time.Time       `json:"created_at"`
}

// publicCatalogListingResponse is the public catalog row wire. Its field
// set is pinned by the router tests with strict decoding: adding any
// adopter-derived field (adoption counts, adopter ids, raw metrics) breaks
// the privacy contract (spec §12) and the test. The T35 #65 trust fields
// (safe review summary, manifest projection, release-pinned Evaluation
// summaries, bucketed metrics view) are the reviewed allowlist.
type publicCatalogListingResponse struct {
	ID                   string                        `json:"id"`
	DisplayName          string                        `json:"display_name"`
	Summary              string                        `json:"summary"`
	State                string                        `json:"state"`
	PublisherTenantID    uint64                        `json:"publisher_tenant_id"`
	PublisherVerified    bool                          `json:"publisher_verified"`
	CurrentRelease       *publicCatalogReleaseResponse `json:"current_release,omitempty"`
	CurrentReleaseReview *publicReviewSummaryResponse  `json:"current_release_review,omitempty"`
	Evaluations          []agentEvaluationResponse     `json:"evaluations,omitempty"`
	Metrics              *marketplaceMetricsResponse   `json:"metrics,omitempty"`
	CreatedAt            time.Time                     `json:"created_at"`
	UpdatedAt            time.Time                     `json:"updated_at"`
}

type publicSubmissionResponse struct {
	ID                string          `json:"id"`
	PublisherTenantID uint64          `json:"publisher_tenant_id"`
	PublicListingID   string          `json:"public_listing_id"`
	SourceListingID   string          `json:"source_listing_id"`
	SourceReleaseID   string          `json:"source_release_id"`
	PublisherActorID  string          `json:"publisher_actor_id"`
	SemanticVersion   string          `json:"semantic_version"`
	BundleDigest      string          `json:"bundle_digest"`
	Manifest          json.RawMessage `json:"manifest"`
	DependencyLock    json.RawMessage `json:"dependency_lock"`
	Status            string          `json:"status"`
	CreatedAt         time.Time       `json:"created_at"`
}

type publicReviewResponse struct {
	ID             string    `json:"id"`
	SubmissionID   string    `json:"submission_id"`
	ReviewerID     string    `json:"reviewer_id"`
	ReviewedDigest string    `json:"reviewed_digest"`
	Decision       string    `json:"decision"`
	Reason         string    `json:"reason"`
	CreatedAt      time.Time `json:"created_at"`
}

type publicReleaseResponse struct {
	ID                string          `json:"id"`
	ListingID         string          `json:"listing_id"`
	SubmissionID      string          `json:"submission_id"`
	PublisherTenantID uint64          `json:"publisher_tenant_id"`
	ReleaseNumber     int             `json:"release_number"`
	SemanticVersion   string          `json:"semantic_version"`
	BundleDigest      string          `json:"bundle_digest"`
	Manifest          json.RawMessage `json:"manifest"`
	DependencyLock    json.RawMessage `json:"dependency_lock"`
	PublishedBy       string          `json:"published_by"`
	CreatedAt         time.Time       `json:"created_at"`
}

type publicIntroductionResponse struct {
	ID              string    `json:"id"`
	PublicListingID string    `json:"public_listing_id"`
	PublicReleaseID string    `json:"public_release_id"`
	DisplayName     string    `json:"display_name"`
	Summary         string    `json:"summary"`
	SemanticVersion string    `json:"semantic_version"`
	BundleDigest    string    `json:"bundle_digest"`
	IntroducedBy    string    `json:"introduced_by"`
	IntroducedAt    time.Time `json:"introduced_at"`
}

type publicAdoptionSummaryResponse struct {
	ID                string    `json:"id"`
	ListingID         string    `json:"listing_id"`
	AcceptedReleaseID string    `json:"accepted_release_id"`
	State             string    `json:"state"`
	CreatedBy         string    `json:"created_by"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// publicReviewSummaryResponse carries the platform review decision with
// the current release: decision/reviewer/time ONLY — never the Reason.
type publicReviewSummaryResponse struct {
	SubmissionID string    `json:"submission_id"`
	ReviewerID   string    `json:"reviewer_id"`
	Decision     string    `json:"decision"`
	ReviewedAt   time.Time `json:"reviewed_at"`
}

// agentEvaluationCheckResponse / agentEvaluationResultsResponse are the
// typed structured Evaluation results on the wire (never double-encoded
// JSON strings, never freeform text).
type agentEvaluationCheckResponse struct {
	Code   string `json:"code"`
	Status string `json:"status"`
}

type agentEvaluationResultsResponse struct {
	Status string                         `json:"status"`
	Checks []agentEvaluationCheckResponse `json:"checks"`
}

type agentEvaluationResponse struct {
	ID               string                         `json:"id"`
	ReleaseID        string                         `json:"release_id"`
	TestSetID        string                         `json:"test_set_id"`
	TestSetVersion   string                         `json:"test_set_version"`
	EnvironmentClass string                         `json:"environment_class"`
	EvaluatorID      string                         `json:"evaluator_id"`
	EvaluatedAt      time.Time                      `json:"evaluated_at"`
	Results          agentEvaluationResultsResponse `json:"results"`
}

// marketplaceMetricsResponse is the closed five-field metrics wire: coarse
// buckets plus the explicit not_collected error-category availability.
type marketplaceMetricsResponse struct {
	IntroductionsBucket       string `json:"introductions_bucket"`
	ActiveAdoptersBucket      string `json:"active_adopters_bucket"`
	UpgradeProposalsBucket    string `json:"upgrade_proposals_bucket"`
	AcceptedUpgradesBucket    string `json:"accepted_upgrades_bucket"`
	ErrorCategoryAvailability string `json:"error_category_availability"`
}

type adoptPublicListingResponse struct {
	Introduction publicIntroductionResponse    `json:"introduction"`
	Adoption     publicAdoptionSummaryResponse `json:"adoption"`
}

func verifiedPublisherDTO(row interfaces.VerifiedPublisherView) verifiedPublisherResponse {
	return verifiedPublisherResponse{
		TenantID: row.TenantID, State: row.State, VerifiedBy: row.VerifiedBy, Note: row.Note,
		VerifiedAt: row.VerifiedAt, UpdatedAt: row.UpdatedAt,
	}
}

func publicCatalogReleaseDTO(release *interfaces.PublicReleaseSummary) *publicCatalogReleaseResponse {
	if release == nil {
		return nil
	}
	return &publicCatalogReleaseResponse{
		ID: release.ID, SemanticVersion: release.SemanticVersion, BundleDigest: release.BundleDigest,
		Manifest: json.RawMessage(release.ManifestJSON), DependencyLock: json.RawMessage(release.DependencyLockJSON),
		MinimumWeKnoraCapability: release.MinimumWeKnoraCapability,
		CapabilityRequirements:   release.CapabilityRequirements,
		LicenseID:                release.LicenseID,
		CreatedAt:                release.CreatedAt,
	}
}

func publicReviewSummaryDTO(summary *interfaces.PublicReleaseReviewSummary) *publicReviewSummaryResponse {
	if summary == nil {
		return nil
	}
	return &publicReviewSummaryResponse{
		SubmissionID: summary.SubmissionID, ReviewerID: summary.ReviewerID,
		Decision: summary.Decision, ReviewedAt: summary.ReviewedAt,
	}
}

// publicEvaluationDTO renders one Evaluation with its ResultsJSON parsed
// into the closed typed shape. ResultsJSON is repo-validated at write time
// (T35 #65 Task 1); a decode failure here means storage corruption and is
// surfaced as an error instead of silently fabricated evidence.
func publicEvaluationDTO(view interfaces.AgentEvaluationView) (agentEvaluationResponse, error) {
	response := agentEvaluationResponse{
		ID: view.ID, ReleaseID: view.ReleaseID, TestSetID: view.TestSetID, TestSetVersion: view.TestSetVersion,
		EnvironmentClass: view.EnvironmentClass, EvaluatorID: view.EvaluatorID, EvaluatedAt: view.EvaluatedAt,
		Results: agentEvaluationResultsResponse{Checks: []agentEvaluationCheckResponse{}},
	}
	decoder := json.NewDecoder(strings.NewReader(view.ResultsJSON))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&response.Results); err != nil {
		return agentEvaluationResponse{}, err
	}
	return response, nil
}

func publicMetricsDTO(view *interfaces.MarketplaceMetricsView) *marketplaceMetricsResponse {
	if view == nil {
		return nil
	}
	return &marketplaceMetricsResponse{
		IntroductionsBucket:       view.IntroductionsBucket,
		ActiveAdoptersBucket:      view.ActiveAdoptersBucket,
		UpgradeProposalsBucket:    view.UpgradeProposalsBucket,
		AcceptedUpgradesBucket:    view.AcceptedUpgradesBucket,
		ErrorCategoryAvailability: view.ErrorCategoryAvailability,
	}
}

func publicCatalogListingDTO(entry interfaces.PublicCatalogEntryView) (publicCatalogListingResponse, error) {
	response := publicCatalogListingResponse{
		ID: entry.ListingID, DisplayName: entry.DisplayName, Summary: entry.Summary, State: entry.State,
		PublisherTenantID: entry.PublisherTenantID, PublisherVerified: entry.PublisherVerified,
		CurrentRelease:       publicCatalogReleaseDTO(entry.CurrentRelease),
		CurrentReleaseReview: publicReviewSummaryDTO(entry.CurrentReleaseReview),
		Metrics:              publicMetricsDTO(entry.Metrics),
		CreatedAt:            entry.CreatedAt, UpdatedAt: entry.UpdatedAt,
	}
	for _, evaluation := range entry.Evaluations {
		dto, err := publicEvaluationDTO(evaluation)
		if err != nil {
			return publicCatalogListingResponse{}, err
		}
		response.Evaluations = append(response.Evaluations, dto)
	}
	return response, nil
}

func publicSubmissionDTO(row interfaces.PublicSubmissionView) publicSubmissionResponse {
	return publicSubmissionResponse{
		ID: row.ID, PublisherTenantID: row.PublisherTenantID, PublicListingID: row.PublicListingID,
		SourceListingID: row.SourceListingID, SourceReleaseID: row.SourceReleaseID,
		PublisherActorID: row.PublisherActorID, SemanticVersion: row.SemanticVersion,
		BundleDigest: row.BundleDigest, Manifest: json.RawMessage(row.ManifestJSON),
		DependencyLock: json.RawMessage(row.DependencyLockJSON), Status: row.Status, CreatedAt: row.CreatedAt,
	}
}

func publicMarketplaceClientError(err error) error {
	switch {
	case stderrors.Is(err, marketservice.ErrPublicMarketplaceNotFound), stderrors.Is(err, marketrepo.ErrPublicMarketplaceNotFound):
		return apperrors.NewNotFoundError("public marketplace resource not found")
	case stderrors.Is(err, marketservice.ErrPublicMarketplaceNotVerifiedPublisher):
		return apperrors.NewForbiddenError("tenant is not a verified publisher")
	case stderrors.Is(err, marketservice.ErrReleaseRedistributionForbidden):
		// T32 #62 AC2: the refusal message names the source lineage license
		// and why it refuses (409, reviewable).
		return apperrors.NewConflictError(err.Error())
	case stderrors.Is(err, marketrepo.ErrAgentAdoptionTransition):
		return apperrors.NewConflictError(err.Error())
	case stderrors.Is(err, marketrepo.ErrAgentEvaluationConflict):
		return apperrors.NewConflictError("agent evaluation identity already exists")
	case stderrors.Is(err, marketrepo.ErrAgentEvaluationInvalid),
		stderrors.Is(err, marketservice.ErrAgentEvaluationInvalidInput):
		return apperrors.NewValidationError("invalid agent evaluation")
	case stderrors.Is(err, marketrepo.ErrAgentSecurityReleaseBlocked):
		return apperrors.NewConflictError(err.Error())
	case stderrors.Is(err, marketservice.ErrPublicMarketplaceStaleDigest),
		stderrors.Is(err, marketrepo.ErrPublicMarketplaceDigestMismatch),
		stderrors.Is(err, marketrepo.ErrPublicMarketplaceReviewConflict),
		stderrors.Is(err, marketrepo.ErrPublicMarketplacePointerConflict),
		stderrors.Is(err, marketrepo.ErrPublicMarketplaceReleaseConflict):
		return apperrors.NewConflictError("public marketplace submission changed; reload and review again")
	case stderrors.Is(err, marketservice.ErrPublicMarketplaceInvalidInput),
		stderrors.Is(err, marketrepo.ErrPublicMarketplaceInvalidDecision):
		return apperrors.NewValidationError("invalid public marketplace request")
	default:
		return err
	}
}

func (h *PublicMarketplaceHandler) VerifyPublisher(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, agentMarketplaceMaxRequestBytes)
	var body *verifyPublisherBody
	if err := decodeAgentMarketplaceBody(c.Request.Body, &body); err != nil {
		invalidMarketplaceBody(c, err)
		return
	}
	if body == nil || body.TenantID == 0 {
		invalidMarketplaceBody(c, stderrors.New("tenant_id is required"))
		return
	}
	actorID, _ := types.UserIDFromContext(c.Request.Context())
	view, created, err := h.public.VerifyPublisher(c.Request.Context(), actorID, body.TenantID, body.Note)
	if err != nil {
		_ = c.Error(publicMarketplaceClientError(err))
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	c.JSON(status, gin.H{"success": true, "data": verifiedPublisherDTO(view)})
}

func (h *PublicMarketplaceHandler) RevokePublisher(c *gin.Context) {
	targetTenantID, err := parseTenantIDParam(c.Param("tenant_id"))
	if err != nil {
		invalidMarketplaceBody(c, err)
		return
	}
	actorID, _ := types.UserIDFromContext(c.Request.Context())
	if err := h.public.RevokePublisher(c.Request.Context(), actorID, targetTenantID); err != nil {
		_ = c.Error(publicMarketplaceClientError(err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *PublicMarketplaceHandler) ListVerifiedPublishers(c *gin.Context) {
	views, err := h.public.ListVerifiedPublishers(c.Request.Context())
	if err != nil {
		_ = c.Error(publicMarketplaceClientError(err))
		return
	}
	data := make([]verifiedPublisherResponse, 0, len(views))
	for _, view := range views {
		data = append(data, verifiedPublisherDTO(view))
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

func (h *PublicMarketplaceHandler) SubmitPublicRelease(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, agentMarketplaceMaxRequestBytes)
	var body *submitPublicReleaseBody
	if err := decodeAgentMarketplaceBody(c.Request.Body, &body); err != nil {
		invalidMarketplaceBody(c, err)
		return
	}
	if body == nil || strings.TrimSpace(body.SourceListingID) == "" {
		invalidMarketplaceBody(c, stderrors.New("source_listing_id is required"))
		return
	}
	actorID, _ := types.UserIDFromContext(c.Request.Context())
	view, err := h.public.SubmitPublicRelease(c.Request.Context(), sandboxConfigTenantID(c), actorID, body.SourceListingID, body.ReleaseID)
	if err != nil {
		_ = c.Error(publicMarketplaceClientError(err))
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": publicSubmissionDTO(view)})
}

func (h *PublicMarketplaceHandler) ListPublicSubmissions(c *gin.Context) {
	views, err := h.public.ListPublicSubmissions(c.Request.Context(), sandboxConfigTenantID(c))
	if err != nil {
		_ = c.Error(publicMarketplaceClientError(err))
		return
	}
	data := make([]publicSubmissionResponse, 0, len(views))
	for _, view := range views {
		data = append(data, publicSubmissionDTO(view))
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

func (h *PublicMarketplaceHandler) ListPublicReviewQueue(c *gin.Context) {
	views, err := h.public.ListPublicReviewQueue(c.Request.Context())
	if err != nil {
		_ = c.Error(publicMarketplaceClientError(err))
		return
	}
	data := make([]publicSubmissionResponse, 0, len(views))
	for _, view := range views {
		data = append(data, publicSubmissionDTO(view))
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

func (h *PublicMarketplaceHandler) ReviewPublicSubmission(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, agentMarketplaceMaxRequestBytes)
	var body *reviewAgentReleaseBody
	if err := decodeAgentMarketplaceBody(c.Request.Body, &body); err != nil {
		invalidMarketplaceBody(c, err)
		return
	}
	if body == nil || strings.TrimSpace(body.ExpectedDigest) == "" || strings.TrimSpace(body.Decision) == "" {
		invalidMarketplaceBody(c, stderrors.New("expected_digest and decision are required"))
		return
	}
	actorID, _ := types.UserIDFromContext(c.Request.Context())
	result, err := h.public.ReviewPublicSubmission(c.Request.Context(), actorID, c.Param("id"), body.ExpectedDigest, types.AgentReleaseReviewDecision{Decision: body.Decision, Reason: body.Reason})
	if err != nil {
		_ = c.Error(publicMarketplaceClientError(err))
		return
	}
	var review *publicReviewResponse
	if result.Review != nil {
		review = &publicReviewResponse{
			ID: result.Review.ID, SubmissionID: result.Review.SubmissionID, ReviewerID: result.Review.ReviewerID,
			ReviewedDigest: result.Review.ReviewedDigest, Decision: result.Review.Decision, Reason: result.Review.Reason,
			CreatedAt: result.Review.CreatedAt,
		}
	}
	var release *publicReleaseResponse
	if result.Release != nil {
		release = &publicReleaseResponse{
			ID: result.Release.ID, ListingID: result.Release.ListingID, SubmissionID: result.Release.SubmissionID,
			PublisherTenantID: result.Release.PublisherTenantID, ReleaseNumber: result.Release.ReleaseNumber,
			SemanticVersion: result.Release.SemanticVersion, BundleDigest: result.Release.BundleDigest,
			Manifest: json.RawMessage(result.Release.ManifestJSON), DependencyLock: json.RawMessage(result.Release.DependencyLockJSON),
			PublishedBy: result.Release.PublishedBy, CreatedAt: result.Release.CreatedAt,
		}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"review": review, "release": release}})
}

// parseTenantIDParam parses the numeric :tenant_id route param; a malformed
// value is a plain 400.
func parseTenantIDParam(raw string) (uint64, error) {
	value, err := strconv.ParseUint(strings.TrimSpace(raw), 10, 64)
	if err != nil || value == 0 {
		return 0, stderrors.New("tenant_id must be a positive integer")
	}
	return value, nil
}

func (h *PublicMarketplaceHandler) ListPublicCatalog(c *gin.Context) {
	entries, err := h.public.ListPublicCatalog(c.Request.Context())
	if err != nil {
		_ = c.Error(publicMarketplaceClientError(err))
		return
	}
	data := make([]publicCatalogListingResponse, 0, len(entries))
	for _, entry := range entries {
		dto, err := publicCatalogListingDTO(entry)
		if err != nil {
			_ = c.Error(err)
			return
		}
		data = append(data, dto)
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

func (h *PublicMarketplaceHandler) GetPublicListing(c *gin.Context) {
	entry, err := h.public.GetPublicListing(c.Request.Context(), c.Param("id"))
	if err != nil {
		_ = c.Error(publicMarketplaceClientError(err))
		return
	}
	dto, err := publicCatalogListingDTO(entry.PublicCatalogEntryView)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": dto})
}

// RecordPublicEvaluation is the SystemAdmin-only platform Evaluation
// authoring entry (T35 #65 Task 4): the reviewer identity comes from the
// authenticated context, the structured results are re-serialized into the
// repo-validated closed shape, and invalid/conflict map to 400/409.
func (h *PublicMarketplaceHandler) RecordPublicEvaluation(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, agentMarketplaceMaxRequestBytes)
	var body *recordPublicEvaluationBody
	if err := decodeAgentMarketplaceBody(c.Request.Body, &body); err != nil {
		invalidMarketplaceBody(c, err)
		return
	}
	if body == nil || strings.TrimSpace(body.ReleaseID) == "" || strings.TrimSpace(body.TestSetID) == "" ||
		strings.TrimSpace(body.TestSetVersion) == "" || strings.TrimSpace(body.EnvironmentClass) == "" ||
		strings.TrimSpace(body.Results.Status) == "" || len(body.Results.Checks) == 0 {
		invalidMarketplaceBody(c, stderrors.New("release_id, test_set_id, test_set_version, environment_class and results (status+checks) are required"))
		return
	}
	resultsJSON, err := json.Marshal(body.Results)
	if err != nil {
		invalidMarketplaceBody(c, err)
		return
	}
	reviewerID, _ := types.UserIDFromContext(c.Request.Context())
	view, err := h.public.RecordEvaluation(c.Request.Context(), reviewerID, types.AgentEvaluationEntity{
		ReleaseID: body.ReleaseID, TestSetID: body.TestSetID, TestSetVersion: body.TestSetVersion,
		EnvironmentClass: body.EnvironmentClass, EvaluatedAt: time.Now().UTC(), ResultsJSON: string(resultsJSON),
	})
	if err != nil {
		_ = c.Error(publicMarketplaceClientError(err))
		return
	}
	response, err := publicEvaluationDTO(view)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": response})
}

func (h *PublicMarketplaceHandler) AdoptPublicListing(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, agentMarketplaceMaxRequestBytes)
	var body *adoptPublicListingBody
	if err := decodeAgentMarketplaceBody(c.Request.Body, &body); err != nil {
		invalidMarketplaceBody(c, err)
		return
	}
	releaseID := ""
	if body != nil {
		releaseID = body.ReleaseID
	}
	actorID, _ := types.UserIDFromContext(c.Request.Context())
	result, created, err := h.public.AdoptPublicListing(c.Request.Context(), sandboxConfigTenantID(c), actorID, c.Param("id"), releaseID)
	if err != nil {
		_ = c.Error(publicMarketplaceClientError(err))
		return
	}
	response := adoptPublicListingResponse{
		Introduction: publicIntroductionResponse{
			ID: result.Introduction.ID, PublicListingID: result.Introduction.PublicListingID,
			PublicReleaseID: result.Introduction.PublicReleaseID, DisplayName: result.Introduction.DisplayName,
			Summary: result.Introduction.Summary, SemanticVersion: result.Introduction.SemanticVersion,
			BundleDigest: result.Introduction.BundleDigest, IntroducedBy: result.Introduction.IntroducedBy,
			IntroducedAt: result.Introduction.IntroducedAt,
		},
		Adoption: publicAdoptionSummaryResponse{
			ID: result.Adoption.ID, ListingID: result.Adoption.ListingID,
			AcceptedReleaseID: result.Adoption.AcceptedReleaseID, State: result.Adoption.State,
			CreatedBy: result.Adoption.CreatedBy, CreatedAt: result.Adoption.CreatedAt, UpdatedAt: result.Adoption.UpdatedAt,
		},
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	c.JSON(status, gin.H{"success": true, "data": response})
}
