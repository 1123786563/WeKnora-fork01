package session

// T17 (#47): the Lead Agent's read-only research delegation surface and the
// version-pinned material annotation surface. Delegations are structurally
// read-only: this handler holds no run-admission write path, so a delegation
// can never take (or release) the session's single write slot. Annotations
// are append-only and pin the artifact's CURRENT version identity at write
// time; a stale base version conflicts instead of silently re-labeling a
// newer version (spec: 已审批版本保持不变).

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ResearchStore is the delegation persistence seam (production:
// *repository.TaskResearchStore).
type ResearchStore interface {
	CreateDelegation(ctx context.Context, d types.TaskResearchDelegation) error
	GetDelegation(ctx context.Context, tenantID uint64, id string) (types.TaskResearchDelegation, error)
	ListDelegationsBySession(ctx context.Context, tenantID uint64, sessionID string) ([]types.TaskResearchDelegation, error)
	CompleteDelegation(ctx context.Context, tenantID uint64, id, summary string) (types.TaskResearchDelegation, error)
}

// AnnotationStore is the annotation persistence seam (production:
// *repository.TaskAnnotationStore).
type AnnotationStore interface {
	CreateAnnotation(ctx context.Context, a types.TaskArtifactAnnotation) error
	ListAnnotationsBySession(ctx context.Context, tenantID uint64, sessionID string) ([]types.TaskArtifactAnnotation, error)
}

// ResearchSourceAuthorizer decides whether one knowledge base may be
// delegated as a read-only research source. Production wires the tenant-bound
// knowledge base lookup: a source the task's tenant does not own is outside
// the Task Grant and must be rejected BEFORE the delegation row is written
// (spec: Delegated Agents receive only a minimum subset).
type ResearchSourceAuthorizer interface {
	AuthorizeResearchSource(ctx context.Context, tenantID uint64, knowledgeBaseID string) error
}

// TaskAccessResolver resolves the caller's per-task role for the annotation
// write gate (production: *service.TaskGrantService).
type TaskAccessResolver interface {
	ResolveTaskAccess(ctx context.Context, caller types.Caller, taskID string) (types.TaskAccess, error)
}

const (
	maxResearchSources        = 8
	maxResearchObjectiveRunes = 2000
)

// WorkbenchResearchHandler owns the research/annotation endpoints.
type WorkbenchResearchHandler struct {
	runs        OwnedRunReader
	granted     GrantedRunReader
	refs        ArtifactRefReader
	research    ResearchStore
	annotations AnnotationStore
	sources     ResearchSourceAuthorizer
	access      TaskAccessResolver
}

// NewWorkbenchResearchHandler assembles the handler. runs and granted may be
// the same store (the container passes *repository.AgentRunStore twice, same
// as the read/delivery handlers).
func NewWorkbenchResearchHandler(
	runs OwnedRunReader, granted GrantedRunReader, refs ArtifactRefReader,
	research ResearchStore, annotations AnnotationStore,
	sources ResearchSourceAuthorizer, access TaskAccessResolver,
) *WorkbenchResearchHandler {
	return &WorkbenchResearchHandler{
		runs: runs, granted: granted, refs: refs,
		research: research, annotations: annotations,
		sources: sources, access: access,
	}
}

// caller resolves the authenticated tenant and actor (same lockstep as the
// delivery handler: request context first, then the gin auth keys).
func (h *WorkbenchResearchHandler) caller(c *gin.Context) (uint64, string, bool) {
	tenantID, userID := workbenchCaller(c)
	if tenantID == 0 || userID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "code": "unauthorized", "error": "tenant and user identity required"})
		return 0, "", false
	}
	return tenantID, userID, true
}

// resolveReadable resolves the run owner-first, then through the task-grant
// fallback (#42 read face). Writes re-gate on the resolved task role.
func (h *WorkbenchResearchHandler) resolveReadable(c *gin.Context) (agentruntime.Run, bool) {
	tenantID, userID, ok := h.caller(c)
	if !ok {
		return agentruntime.Run{}, false
	}
	run, err := h.runs.GetOwnedRun(c.Request.Context(), tenantID, userID, c.Param("run_id"))
	if err == nil {
		return run, true
	}
	if h.granted != nil {
		if granted, gerr := h.granted.GetRunForGrantedReader(c.Request.Context(), tenantID, userID, c.Param("run_id")); gerr == nil {
			return granted, true
		}
	}
	c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"success": false, "code": "run_not_found", "error": "execution not found"})
	return agentruntime.Run{}, false
}

type researchDelegationView struct {
	DelegationID string   `json:"delegation_id"`
	RunID        string   `json:"run_id"`
	SessionID    string   `json:"session_id"`
	Objective    string   `json:"objective"`
	Sources      []string `json:"sources"`
	Status       string   `json:"status"`
	Summary      string   `json:"summary,omitempty"`
	CreatedAt    string   `json:"created_at"`
}

func delegationViewOf(d types.TaskResearchDelegation) researchDelegationView {
	return researchDelegationView{
		DelegationID: d.ID,
		RunID:        d.ParentRunID,
		SessionID:    d.SessionID,
		Objective:    d.Objective,
		Sources:      d.Sources(),
		Status:       d.Status,
		Summary:      d.Summary,
		CreatedAt:    d.CreatedAt.UTC().Format(time.RFC3339),
	}
}

type researchAnnotationView struct {
	AnnotationID string `json:"annotation_id"`
	RunID        string `json:"run_id"`
	MaterialID   string `json:"material_id"`
	BaseVersion  string `json:"base_version"`
	Body         string `json:"body"`
	AuthorID     string `json:"author_id"`
	CreatedAt    string `json:"created_at"`
}

func annotationViewOf(a types.TaskArtifactAnnotation) researchAnnotationView {
	return researchAnnotationView{
		AnnotationID: a.ID,
		RunID:        a.RunID,
		MaterialID:   a.MaterialID,
		BaseVersion:  a.BaseVersion,
		Body:         a.Body,
		AuthorID:     a.AuthorID,
		CreatedAt:    a.CreatedAt.UTC().Format(time.RFC3339),
	}
}

type researchDelegateInput struct {
	Objective string   `json:"objective"`
	Sources   []string `json:"sources"`
	// NOTE: the plan's original draft also carried an agent_id field, but a
	// delegation is consumed inside the owning write run and never binds to
	// a specific agent, so the field was dead wire — removed. Clients that
	// still send agent_id are unaffected: gin's JSON binding ignores
	// unknown keys.
}

// DelegateResearch POST /workbench/executions/:run_id/research — owner-only.
// Sources are authorized before any durable write; the persisted row is a
// read-only assignment and never touches the single write slot.
func (h *WorkbenchResearchHandler) DelegateResearch(c *gin.Context) {
	tenantID, userID, ok := h.caller(c)
	if !ok {
		return
	}
	run, ok := resolveOwnedRun(c, h.runs)
	if !ok {
		return
	}
	var input researchDelegateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "research_invalid_request", "error": "objective and sources are required"})
		return
	}
	objective := strings.TrimSpace(input.Objective)
	if objective == "" || len([]rune(objective)) > maxResearchObjectiveRunes || len(input.Sources) == 0 || len(input.Sources) > maxResearchSources {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "research_invalid_request", "error": "objective (1..2000 runes) and 1..8 sources are required"})
		return
	}
	cleaned := make([]string, 0, len(input.Sources))
	for _, source := range input.Sources {
		source = strings.TrimSpace(source)
		if source == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "research_invalid_request", "error": "sources must not contain empty ids"})
			return
		}
		cleaned = append(cleaned, source)
	}
	for _, source := range cleaned {
		if err := h.sources.AuthorizeResearchSource(c.Request.Context(), tenantID, source); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "research_source_out_of_task_grant", "error": "research source is outside the task's tenant knowledge scope"})
			return
		}
	}
	encoded, _ := json.Marshal(cleaned)
	delegation := types.TaskResearchDelegation{
		TenantID: tenantID, ID: uuid.NewString(), SessionID: run.SessionID, ParentRunID: run.Key.RunID,
		Objective: objective, SourcesJSON: string(encoded),
		Status: types.TaskResearchAssigned, CreatedBy: userID,
	}
	if err := h.research.CreateDelegation(c.Request.Context(), delegation); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "code": "research_backend", "error": "failed to persist delegation"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": gin.H{"delegation": delegationViewOf(delegation)}})
}

// ListResearch GET /workbench/executions/:run_id/research — owner + granted.
func (h *WorkbenchResearchHandler) ListResearch(c *gin.Context) {
	run, ok := h.resolveReadable(c)
	if !ok {
		return
	}
	rows, err := h.research.ListDelegationsBySession(c.Request.Context(), run.Key.TenantID, run.SessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "code": "research_backend", "error": "failed to list delegations"})
		return
	}
	items := make([]researchDelegationView, 0, len(rows))
	for _, row := range rows {
		items = append(items, delegationViewOf(row))
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"items": items}})
}

type researchSummaryInput struct {
	Summary string `json:"summary"`
}

// CompleteResearch POST /workbench/executions/:run_id/research/:delegation_id/summary
// — owner-only. The delegation must belong to this run's session; completion
// is one CAS and replays conflict (the recorded summary is immutable).
func (h *WorkbenchResearchHandler) CompleteResearch(c *gin.Context) {
	if _, _, ok := h.caller(c); !ok {
		return
	}
	run, ok := resolveOwnedRun(c, h.runs)
	if !ok {
		return
	}
	var input researchSummaryInput
	if err := c.ShouldBindJSON(&input); err != nil || strings.TrimSpace(input.Summary) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "research_invalid_request", "error": "summary is required"})
		return
	}
	delegation, err := h.research.GetDelegation(c.Request.Context(), run.Key.TenantID, c.Param("delegation_id"))
	if err != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"success": false, "code": "research_not_found", "error": "delegation not found"})
		return
	}
	if delegation.SessionID != run.SessionID {
		// Another task's delegation: same uniform miss, no existence leak.
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"success": false, "code": "research_not_found", "error": "delegation not found"})
		return
	}
	done, err := h.research.CompleteDelegation(c.Request.Context(), run.Key.TenantID, delegation.ID, strings.TrimSpace(input.Summary))
	if err != nil {
		writeResearchError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"delegation": delegationViewOf(done)}})
}

type annotateInput struct {
	MaterialID  string `json:"material_id"`
	BaseVersion string `json:"base_version"`
	Body        string `json:"body"`
}

// AnnotateMaterial POST /workbench/executions/:run_id/annotations — owner or
// collaborator (TaskRoleCanRun). The base_version must equal the material's
// CURRENT version identity; a stale base conflicts (409) so a review record
// can never be silently re-attached to a newer version.
func (h *WorkbenchResearchHandler) AnnotateMaterial(c *gin.Context) {
	tenantID, userID, ok := h.caller(c)
	if !ok {
		return
	}
	run, ok := h.resolveReadable(c)
	if !ok {
		return
	}
	access, err := h.access.ResolveTaskAccess(c.Request.Context(), types.Caller{TenantID: tenantID, UserID: userID}, run.SessionID)
	if err != nil {
		writeResearchError(c, err)
		return
	}
	if !types.TaskRoleCanRun(access.Role) {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"success": false, "code": "research_forbidden", "error": "annotations require the task owner or a collaborator"})
		return
	}
	var input annotateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "research_invalid_request", "error": "material_id, base_version and body are required"})
		return
	}
	input.MaterialID = strings.TrimSpace(input.MaterialID)
	input.BaseVersion = strings.TrimSpace(input.BaseVersion)
	refs, err := h.refs.GetSessionArtifactRefs(c.Request.Context(), run.SessionID)
	if err != nil {
		writeWorkbenchError(c, err)
		return
	}
	var matched *types.SessionArtifactRef
	for i := range refs {
		if refs[i].MessageID+":"+strconv.Itoa(refs[i].Index) == input.MaterialID {
			matched = &refs[i]
			break
		}
	}
	if matched == nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"success": false, "code": "material_not_found", "error": "material not found in this execution"})
		return
	}
	if artifactVersionOf(*matched) != input.BaseVersion {
		c.AbortWithStatusJSON(http.StatusConflict, gin.H{"success": false, "code": "annotation_base_version_conflict", "error": "base_version does not match the current artifact version; reload the material list"})
		return
	}
	if strings.TrimSpace(input.Body) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "research_invalid_request", "error": "body is required"})
		return
	}
	annotation := types.TaskArtifactAnnotation{
		TenantID: tenantID, ID: uuid.NewString(), SessionID: run.SessionID, RunID: run.Key.RunID,
		MaterialID: input.MaterialID, BaseVersion: input.BaseVersion,
		Body: strings.TrimSpace(input.Body), AuthorID: userID,
	}
	if err := h.annotations.CreateAnnotation(c.Request.Context(), annotation); err != nil {
		writeResearchError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": gin.H{"annotation": annotationViewOf(annotation)}})
}

// ListAnnotations GET /workbench/executions/:run_id/annotations — owner +
// granted read (annotations are review records, readable by viewers).
func (h *WorkbenchResearchHandler) ListAnnotations(c *gin.Context) {
	run, ok := h.resolveReadable(c)
	if !ok {
		return
	}
	rows, err := h.annotations.ListAnnotationsBySession(c.Request.Context(), run.Key.TenantID, run.SessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "code": "research_backend", "error": "failed to list annotations"})
		return
	}
	items := make([]researchAnnotationView, 0, len(rows))
	for _, row := range rows {
		items = append(items, annotationViewOf(row))
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"items": items}})
}

// writeResearchError maps store/service failures onto a fixed code table; no
// upstream text crosses the wire.
func writeResearchError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, types.ErrTaskResearchNotFound):
		c.JSON(http.StatusNotFound, gin.H{"success": false, "code": "research_not_found", "error": "delegation not found"})
	case errors.Is(err, types.ErrTaskResearchState):
		c.JSON(http.StatusConflict, gin.H{"success": false, "code": "research_delegation_state", "error": "delegation is not in the assigned state"})
	case errors.Is(err, types.ErrTaskAnnotationInvalid):
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "research_invalid_request", "error": "annotation payload is invalid"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "code": "research_backend", "error": "research surface temporarily unavailable"})
	}
}
