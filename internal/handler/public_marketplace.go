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

type verifiedPublisherResponse struct {
	TenantID   uint64    `json:"tenant_id"`
	State      string    `json:"state"`
	VerifiedBy string    `json:"verified_by"`
	Note       string    `json:"note"`
	VerifiedAt time.Time `json:"verified_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type publicCatalogReleaseResponse struct {
	ID              string          `json:"id"`
	SemanticVersion string          `json:"semantic_version"`
	BundleDigest    string          `json:"bundle_digest"`
	Manifest        json.RawMessage `json:"manifest"`
	DependencyLock  json.RawMessage `json:"dependency_lock"`
	CreatedAt       time.Time       `json:"created_at"`
}

// publicCatalogListingResponse is the public catalog row wire. Its field
// set is pinned by the router tests with strict decoding: adding any
// adopter-derived field (adoption counts, adopter ids, metrics) breaks the
// privacy contract (spec §12) and the test.
type publicCatalogListingResponse struct {
	ID                string                        `json:"id"`
	DisplayName       string                        `json:"display_name"`
	Summary           string                        `json:"summary"`
	State             string                        `json:"state"`
	PublisherTenantID uint64                        `json:"publisher_tenant_id"`
	PublisherVerified bool                          `json:"publisher_verified"`
	CurrentRelease    *publicCatalogReleaseResponse `json:"current_release,omitempty"`
	CreatedAt         time.Time                     `json:"created_at"`
	UpdatedAt         time.Time                     `json:"updated_at"`
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
		CreatedAt: release.CreatedAt,
	}
}

func publicCatalogListingDTO(entry interfaces.PublicCatalogEntryView) publicCatalogListingResponse {
	return publicCatalogListingResponse{
		ID: entry.ListingID, DisplayName: entry.DisplayName, Summary: entry.Summary, State: entry.State,
		PublisherTenantID: entry.PublisherTenantID, PublisherVerified: entry.PublisherVerified,
		CurrentRelease: publicCatalogReleaseDTO(entry.CurrentRelease),
		CreatedAt:      entry.CreatedAt, UpdatedAt: entry.UpdatedAt,
	}
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
