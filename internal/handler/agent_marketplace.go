package handler

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	marketrepo "github.com/Tencent/WeKnora/internal/application/repository"
	marketservice "github.com/Tencent/WeKnora/internal/application/service"
	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

const agentMarketplaceMaxRequestBytes = 1 << 20

// AgentMarketplaceHandler is the HTTP boundary for tenant catalog release
// submissions, review and listing reads. Tenant and actor identity are
// always taken from the authenticated request context.
type AgentMarketplaceHandler struct {
	market   interfaces.AgentMarketplaceService
	versions interfaces.AgentVersionService
}

func NewAgentMarketplaceHandler(market interfaces.AgentMarketplaceService, versions interfaces.AgentVersionService) *AgentMarketplaceHandler {
	return &AgentMarketplaceHandler{market: market, versions: versions}
}

// AgentIDForVersion resolves the tenant-scoped source Agent used by the
// submission ownership guard. It never accepts an Agent ID from the request.
func (h *AgentMarketplaceHandler) AgentIDForVersion(ctx context.Context, tenantID uint64, versionID string) (string, error) {
	snapshot, err := h.versions.GetAgentVersion(ctx, tenantID, versionID)
	if err != nil {
		return "", err
	}
	return snapshot.AgentID, nil
}

type submitAgentReleaseBody struct {
	AgentVersionID string                `json:"agent_version_id"`
	Metadata       types.ReleaseMetadata `json:"metadata"`
}

type reviewAgentReleaseBody struct {
	ExpectedDigest string `json:"expected_digest"`
	Decision       string `json:"decision"`
	Reason         string `json:"reason,omitempty"`
}

type marketplaceSubmissionResponse struct {
	ID                  string          `json:"id"`
	TenantID            uint64          `json:"tenant_id"`
	ListingID           string          `json:"listing_id"`
	AgentVersionID      string          `json:"agent_version_id"`
	SourceAgentID       string          `json:"source_agent_id"`
	AuthorID            string          `json:"author_id"`
	SemanticVersion     string          `json:"semantic_version"`
	BundleDigest        string          `json:"bundle_digest"`
	Manifest            json.RawMessage `json:"manifest"`
	DependencyLock      json.RawMessage `json:"dependency_lock"`
	Payload             json.RawMessage `json:"payload"`
	Status              string          `json:"status"`
	IsFork              bool            `json:"is_fork"`
	ForkSourceListingID string          `json:"fork_source_listing_id,omitempty"`
	ForkSourceReleaseID string          `json:"fork_source_release_id,omitempty"`
	ForkNotes           string          `json:"fork_notes,omitempty"`
	LineageLicenseID    string          `json:"lineage_license_id,omitempty"`
	CreatedAt           time.Time       `json:"created_at"`
}

type marketplaceReviewResponse struct {
	ID             string    `json:"id"`
	SubmissionID   string    `json:"submission_id"`
	ReviewerID     string    `json:"reviewer_id"`
	ReviewedDigest string    `json:"reviewed_digest"`
	Decision       string    `json:"decision"`
	Reason         string    `json:"reason"`
	CreatedAt      time.Time `json:"created_at"`
}

type marketplaceReleaseResponse struct {
	ID                  string          `json:"id"`
	ListingID           string          `json:"listing_id"`
	SubmissionID        string          `json:"submission_id"`
	AgentVersionID      string          `json:"agent_version_id"`
	SourceAgentID       string          `json:"source_agent_id"`
	ReleaseNumber       int             `json:"release_number"`
	SemanticVersion     string          `json:"semantic_version"`
	BundleDigest        string          `json:"bundle_digest"`
	Manifest            json.RawMessage `json:"manifest"`
	DependencyLock      json.RawMessage `json:"dependency_lock"`
	IsFork              bool            `json:"is_fork"`
	ForkSourceListingID string          `json:"fork_source_listing_id,omitempty"`
	ForkSourceReleaseID string          `json:"fork_source_release_id,omitempty"`
	ForkNotes           string          `json:"fork_notes,omitempty"`
	LineageLicenseID    string          `json:"lineage_license_id,omitempty"`
	PublishedBy         string          `json:"published_by"`
	CreatedAt           time.Time       `json:"created_at"`
}

type marketplaceListingResponse struct {
	ID               string    `json:"id"`
	TenantID         uint64    `json:"tenant_id"`
	SourceAgentID    string    `json:"source_agent_id"`
	DisplayName      string    `json:"display_name"`
	Summary          string    `json:"summary"`
	State            string    `json:"state"`
	CurrentReleaseID *string   `json:"current_release_id,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

func marketplaceSubmissionDTO(row interfaces.ReleaseSubmissionView) marketplaceSubmissionResponse {
	var envelope struct {
		Payload json.RawMessage `json:"payload"`
	}
	_ = json.Unmarshal(row.Bundle, &envelope)
	return marketplaceSubmissionResponse{ID: row.ID, TenantID: row.TenantID, ListingID: row.ListingID, AgentVersionID: row.AgentVersionID, SourceAgentID: row.SourceAgentID, AuthorID: row.AuthorID, SemanticVersion: row.SemanticVersion, BundleDigest: row.BundleDigest, Manifest: json.RawMessage(row.ManifestJSON), DependencyLock: json.RawMessage(row.DependencyLockJSON), Payload: envelope.Payload, Status: row.Status, IsFork: row.IsFork, ForkSourceListingID: row.ForkSourceListingID, ForkSourceReleaseID: row.ForkSourceReleaseID, ForkNotes: row.ForkNotes, LineageLicenseID: row.LineageLicenseID, CreatedAt: row.CreatedAt}
}

func marketplaceClientError(err error) error {
	switch {
	case stderrors.Is(err, marketrepo.ErrAgentMarketplaceNotFound):
		return apperrors.NewNotFoundError("release submission not found")
	case stderrors.Is(err, marketrepo.ErrAgentMarketplaceDigestMismatch), stderrors.Is(err, marketservice.ErrAgentMarketplaceStaleDigest), stderrors.Is(err, marketrepo.ErrAgentMarketplacePointerConflict), stderrors.Is(err, marketrepo.ErrAgentMarketplaceReviewConflict):
		return apperrors.NewConflictError("release submission changed; reload and review again")
	case stderrors.Is(err, marketservice.ErrReleaseRedistributionForbidden):
		// The refusal message IS the governance reason: it names the
		// source license and why it refuses (409, reviewable).
		return apperrors.NewConflictError(err.Error())
	case stderrors.Is(err, marketservice.ErrAgentMarketplaceMissingDependency):
		return apperrors.NewValidationError(err.Error())
	case stderrors.Is(err, marketservice.ErrAgentMarketplaceInvalidInput), stderrors.Is(err, marketrepo.ErrAgentMarketplaceInvalidDecision), stderrors.Is(err, marketrepo.ErrAgentMarketplaceVersionAgentMismatch):
		return apperrors.NewValidationError("invalid marketplace release request")
	default:
		return err
	}
}

func marketplaceReleaseDTO(row *types.AgentReleaseEntity) *marketplaceReleaseResponse {
	if row == nil {
		return nil
	}
	return &marketplaceReleaseResponse{ID: row.ID, ListingID: row.ListingID, SubmissionID: row.SubmissionID, AgentVersionID: row.AgentVersionID, SourceAgentID: row.SourceAgentID, ReleaseNumber: row.ReleaseNumber, SemanticVersion: row.SemanticVersion, BundleDigest: row.BundleDigest, Manifest: json.RawMessage(row.ManifestJSON), DependencyLock: json.RawMessage(row.DependencyLockJSON), IsFork: row.IsFork, ForkSourceListingID: row.ForkSourceListingID, ForkSourceReleaseID: row.ForkSourceReleaseID, ForkNotes: row.ForkNotes, LineageLicenseID: row.LineageLicenseID, PublishedBy: row.PublishedBy, CreatedAt: row.CreatedAt}
}

func decodeAgentMarketplaceBody(r io.Reader, dst any) error {
	dec := json.NewDecoder(io.LimitReader(r, agentMarketplaceMaxRequestBytes+1))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return stderrors.New("trailing JSON value")
		}
		return err
	}
	return nil
}

func invalidMarketplaceBody(c *gin.Context, err error) {
	_ = c.Error(apperrors.NewBadRequestError("invalid marketplace request: " + err.Error()))
}

func (h *AgentMarketplaceHandler) SubmitRelease(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, agentMarketplaceMaxRequestBytes)
	var body *submitAgentReleaseBody
	if err := decodeAgentMarketplaceBody(c.Request.Body, &body); err != nil {
		invalidMarketplaceBody(c, err)
		return
	}
	if body == nil || strings.TrimSpace(body.AgentVersionID) == "" {
		invalidMarketplaceBody(c, stderrors.New("agent_version_id is required"))
		return
	}
	actorID, _ := types.UserIDFromContext(c.Request.Context())
	view, err := h.market.SubmitRelease(c.Request.Context(), sandboxConfigTenantID(c), actorID, body.AgentVersionID, interfaces.SubmitReleaseInput{Metadata: body.Metadata})
	if err != nil {
		_ = c.Error(marketplaceClientError(err))
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": marketplaceSubmissionDTO(view)})
}

func (h *AgentMarketplaceHandler) ListReviewQueue(c *gin.Context) {
	views, err := h.market.ListReviewQueue(c.Request.Context(), sandboxConfigTenantID(c))
	if err != nil {
		_ = c.Error(marketplaceClientError(err))
		return
	}
	data := make([]marketplaceSubmissionResponse, 0, len(views))
	for _, view := range views {
		data = append(data, marketplaceSubmissionDTO(view))
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

func (h *AgentMarketplaceHandler) ReviewSubmission(c *gin.Context) {
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
	result, err := h.market.ReviewSubmission(c.Request.Context(), sandboxConfigTenantID(c), actorID, c.Param("id"), body.ExpectedDigest, types.AgentReleaseReviewDecision{Decision: body.Decision, Reason: body.Reason})
	if err != nil {
		_ = c.Error(marketplaceClientError(err))
		return
	}
	var review *marketplaceReviewResponse
	if result.Review != nil {
		review = &marketplaceReviewResponse{ID: result.Review.ID, SubmissionID: result.Review.SubmissionID, ReviewerID: result.Review.ReviewerID, ReviewedDigest: result.Review.ReviewedDigest, Decision: result.Review.Decision, Reason: result.Review.Reason, CreatedAt: result.Review.CreatedAt}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"review": review, "release": marketplaceReleaseDTO(result.Release)}})
}

func (h *AgentMarketplaceHandler) ListTenantCatalog(c *gin.Context) {
	views, err := h.market.ListTenantCatalog(c.Request.Context(), sandboxConfigTenantID(c))
	if err != nil {
		_ = c.Error(err)
		return
	}
	data := make([]marketplaceListingResponse, 0, len(views))
	for _, view := range views {
		row := view.AgentMarketplaceListingEntity
		data = append(data, marketplaceListingResponse{ID: row.ID, TenantID: row.TenantID, SourceAgentID: row.SourceAgentID, DisplayName: row.DisplayName, Summary: row.Summary, State: row.State, CurrentReleaseID: row.CurrentReleaseID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt})
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

type registerLicenseBody struct {
	ID                   string `json:"id"`
	Name                 string `json:"name"`
	AllowsRedistribution bool   `json:"allows_redistribution"`
}

type licenseResponse struct {
	ID                   string    `json:"id"`
	Name                 string    `json:"name"`
	AllowsRedistribution bool      `json:"allows_redistribution"`
	CreatedBy            string    `json:"created_by"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

func licenseDTO(row types.AgentLicenseEntity) licenseResponse {
	return licenseResponse{ID: row.ID, Name: row.Name, AllowsRedistribution: row.AllowsRedistribution, CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
}

// RegisterLicense records (or re-records) one deployment license term
// (T32 #62). The redistribution gate reads this registry live.
func (h *AgentMarketplaceHandler) RegisterLicense(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, agentMarketplaceMaxRequestBytes)
	var body *registerLicenseBody
	if err := decodeAgentMarketplaceBody(c.Request.Body, &body); err != nil {
		invalidMarketplaceBody(c, err)
		return
	}
	if body == nil || strings.TrimSpace(body.ID) == "" {
		invalidMarketplaceBody(c, stderrors.New("id is required"))
		return
	}
	actorID, _ := types.UserIDFromContext(c.Request.Context())
	row, err := h.market.RegisterLicense(c.Request.Context(), actorID, interfaces.LicenseInput{
		ID: body.ID, Name: body.Name, AllowsRedistribution: body.AllowsRedistribution,
	})
	if err != nil {
		if stderrors.Is(err, marketservice.ErrAgentLicenseInvalid) {
			_ = c.Error(apperrors.NewValidationError("invalid agent license request"))
			return
		}
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": licenseDTO(row)})
}

// ListLicenses returns the deployment license registry (Admin+).
func (h *AgentMarketplaceHandler) ListLicenses(c *gin.Context) {
	rows, err := h.market.ListLicenses(c.Request.Context())
	if err != nil {
		_ = c.Error(err)
		return
	}
	data := make([]licenseResponse, 0, len(rows))
	for _, row := range rows {
		data = append(data, licenseDTO(row))
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}
