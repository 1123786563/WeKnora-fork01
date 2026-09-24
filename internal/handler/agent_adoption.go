package handler

import (
	stderrors "errors"
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

// AgentAdoptionHandler is the HTTP boundary for the Tenant Adoption and
// Variant governance workflow (spec §8). Tenant and actor identity always
// come from the authenticated request context; request bodies never carry
// principals (strict decoding rejects spoof attempts).
type AgentAdoptionHandler struct {
	adoptions interfaces.AgentAdoptionService
}

func NewAgentAdoptionHandler(adoptions interfaces.AgentAdoptionService) *AgentAdoptionHandler {
	return &AgentAdoptionHandler{adoptions: adoptions}
}

type adoptAgentBody struct {
	ListingID string `json:"listing_id"`
	ReleaseID string `json:"release_id,omitempty"`
}

type createAdoptionVariantBody struct {
	Name      string `json:"name"`
	ReleaseID string `json:"release_id,omitempty"`
}

type capabilityMappingEntryBody struct {
	Capability       string   `json:"capability"`
	ModelID          string   `json:"model_id,omitempty"`
	KnowledgeBaseIDs []string `json:"knowledge_base_ids,omitempty"`
	ConnectionIDs    []string `json:"connection_ids,omitempty"`
}

type updateCapabilityMappingBody struct {
	Mappings []capabilityMappingEntryBody `json:"mappings"`
}

type adoptionVariantResponse struct {
	ID                  string    `json:"id"`
	AdoptionID          string    `json:"adoption_id"`
	ReleaseID           string    `json:"release_id"`
	Name                string    `json:"name"`
	State               string    `json:"state"`
	LocalAgentID        string    `json:"local_agent_id,omitempty"`
	LocalAgentVersionID string    `json:"local_agent_version_id,omitempty"`
	MissingCapabilities []string  `json:"missing_capabilities"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type adoptionResponse struct {
	ID                string                    `json:"id"`
	ListingID         string                    `json:"listing_id"`
	AcceptedReleaseID string                    `json:"accepted_release_id"`
	State             string                    `json:"state"`
	CreatedBy         string                    `json:"created_by"`
	CreatedAt         time.Time                 `json:"created_at"`
	UpdatedAt         time.Time                 `json:"updated_at"`
	Variants          []adoptionVariantResponse `json:"variants"`
}

type availableAgentCapability struct {
	State  string `json:"state"`
	Reason string `json:"reason"`
}

type availableAgentResponse struct {
	AgentID     string                   `json:"agent_id"`
	VariantID   string                   `json:"variant_id"`
	AdoptionID  string                   `json:"adoption_id"`
	ReleaseID   string                   `json:"release_id"`
	Name        string                   `json:"name"`
	Description string                   `json:"description"`
	IsBuiltin   bool                     `json:"is_builtin"`
	Capability  availableAgentCapability `json:"capability"`
}

func adoptionVariantDTO(view interfaces.AdoptionVariantView) adoptionVariantResponse {
	return adoptionVariantResponse{
		ID: view.ID, AdoptionID: view.AdoptionID, ReleaseID: view.ReleaseID,
		Name: view.Name, State: view.State,
		LocalAgentID: view.LocalAgentID, LocalAgentVersionID: view.LocalAgentVersionID,
		MissingCapabilities: view.MissingCapabilities,
		CreatedAt:           view.CreatedAt, UpdatedAt: view.UpdatedAt,
	}
}

func adoptionDTO(view interfaces.AdoptionView) adoptionResponse {
	variants := make([]adoptionVariantResponse, 0, len(view.Variants))
	for _, variant := range view.Variants {
		variants = append(variants, adoptionVariantDTO(variant))
	}
	return adoptionResponse{
		ID: view.ID, ListingID: view.ListingID, AcceptedReleaseID: view.AcceptedReleaseID,
		State: view.State, CreatedBy: view.CreatedBy,
		CreatedAt: view.CreatedAt, UpdatedAt: view.UpdatedAt, Variants: variants,
	}
}

func adoptionClientError(err error) error {
	switch {
	case stderrors.Is(err, marketservice.ErrAgentAdoptionNotFound), stderrors.Is(err, marketrepo.ErrAgentAdoptionNotFound):
		return apperrors.NewNotFoundError("agent adoption resource not found")
	case stderrors.Is(err, marketservice.ErrAgentAdoptionVariantNotRunnable),
		stderrors.Is(err, marketservice.ErrAgentAdoptionStateConflict),
		stderrors.Is(err, marketrepo.ErrAgentAdoptionVariantTransition),
		stderrors.Is(err, marketrepo.ErrAgentAdoptionRemapStateConflict):
		// The refusal message IS the explicit reason (missing capability
		// names, conflicting state) — pass it through verbatim.
		return apperrors.NewConflictError(err.Error())
	case stderrors.Is(err, marketservice.ErrAgentAdoptionInvalidInput):
		return apperrors.NewValidationError("invalid agent adoption request")
	default:
		return err
	}
}

func (h *AgentAdoptionHandler) Adopt(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, agentMarketplaceMaxRequestBytes)
	var body *adoptAgentBody
	if err := decodeAgentMarketplaceBody(c.Request.Body, &body); err != nil {
		invalidMarketplaceBody(c, err)
		return
	}
	if body == nil || strings.TrimSpace(body.ListingID) == "" {
		invalidMarketplaceBody(c, stderrors.New("listing_id is required"))
		return
	}
	actorID, _ := types.UserIDFromContext(c.Request.Context())
	view, created, err := h.adoptions.Adopt(c.Request.Context(), sandboxConfigTenantID(c), actorID, interfaces.AdoptInput{ListingID: body.ListingID, ReleaseID: body.ReleaseID})
	if err != nil {
		_ = c.Error(adoptionClientError(err))
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	c.JSON(status, gin.H{"success": true, "data": adoptionDTO(view)})
}

func (h *AgentAdoptionHandler) ListAdoptions(c *gin.Context) {
	views, err := h.adoptions.ListAdoptions(c.Request.Context(), sandboxConfigTenantID(c))
	if err != nil {
		_ = c.Error(adoptionClientError(err))
		return
	}
	data := make([]adoptionResponse, 0, len(views))
	for _, view := range views {
		data = append(data, adoptionDTO(view))
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

func (h *AgentAdoptionHandler) CreateVariant(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, agentMarketplaceMaxRequestBytes)
	var body *createAdoptionVariantBody
	if err := decodeAgentMarketplaceBody(c.Request.Body, &body); err != nil {
		invalidMarketplaceBody(c, err)
		return
	}
	if body == nil || strings.TrimSpace(body.Name) == "" {
		invalidMarketplaceBody(c, stderrors.New("name is required"))
		return
	}
	actorID, _ := types.UserIDFromContext(c.Request.Context())
	view, err := h.adoptions.CreateVariant(c.Request.Context(), sandboxConfigTenantID(c), actorID, c.Param("id"), interfaces.VariantDraftInput{Name: body.Name, ReleaseID: body.ReleaseID})
	if err != nil {
		_ = c.Error(adoptionClientError(err))
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": adoptionVariantDTO(view)})
}

func (h *AgentAdoptionHandler) UpdateCapabilityMapping(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, agentMarketplaceMaxRequestBytes)
	var body *updateCapabilityMappingBody
	if err := decodeAgentMarketplaceBody(c.Request.Body, &body); err != nil {
		invalidMarketplaceBody(c, err)
		return
	}
	if body == nil || body.Mappings == nil {
		invalidMarketplaceBody(c, stderrors.New("mappings are required"))
		return
	}
	mappings := make([]interfaces.CapabilityMapping, 0, len(body.Mappings))
	for _, entry := range body.Mappings {
		mappings = append(mappings, interfaces.CapabilityMapping{
			Capability: entry.Capability, ModelID: entry.ModelID,
			KnowledgeBaseIDs: entry.KnowledgeBaseIDs, ConnectionIDs: entry.ConnectionIDs,
		})
	}
	actorID, _ := types.UserIDFromContext(c.Request.Context())
	view, err := h.adoptions.UpdateCapabilityMapping(c.Request.Context(), sandboxConfigTenantID(c), actorID, c.Param("id"), mappings)
	if err != nil {
		_ = c.Error(adoptionClientError(err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": adoptionVariantDTO(view)})
}

func (h *AgentAdoptionHandler) TestVariant(c *gin.Context) {
	actorID, _ := types.UserIDFromContext(c.Request.Context())
	view, err := h.adoptions.TestVariant(c.Request.Context(), sandboxConfigTenantID(c), actorID, c.Param("id"))
	if err != nil {
		_ = c.Error(adoptionClientError(err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": adoptionVariantDTO(view)})
}

func (h *AgentAdoptionHandler) PublishVariant(c *gin.Context) {
	actorID, _ := types.UserIDFromContext(c.Request.Context())
	result, err := h.adoptions.PublishVariant(c.Request.Context(), sandboxConfigTenantID(c), actorID, c.Param("id"))
	if err != nil {
		_ = c.Error(adoptionClientError(err))
		return
	}
	// Same envelope as Adopt/CreateVariant/TestVariant/UpdateCapabilityMapping:
	// data IS the variant body (B3-F86).
	c.JSON(http.StatusOK, gin.H{"success": true, "data": adoptionVariantDTO(result.Variant)})
}

func (h *AgentAdoptionHandler) ListAvailableAgents(c *gin.Context) {
	views, err := h.adoptions.ListAvailableAgents(c.Request.Context(), sandboxConfigTenantID(c))
	if err != nil {
		_ = c.Error(adoptionClientError(err))
		return
	}
	data := make([]availableAgentResponse, 0, len(views))
	for _, view := range views {
		data = append(data, availableAgentResponse{
			AgentID: view.AgentID, VariantID: view.VariantID, AdoptionID: view.AdoptionID,
			ReleaseID: view.ReleaseID, Name: view.Name, Description: view.Description,
			IsBuiltin:  view.IsBuiltin,
			Capability: availableAgentCapability{State: view.Capability.State, Reason: view.Capability.Reason},
		})
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}
