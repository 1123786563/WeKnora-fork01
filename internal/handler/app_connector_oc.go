package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	appconnector "github.com/Tencent/WeKnora/internal/modules/appconnector"
	appconnectorrepo "github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// app_connector_oc.go serves the T13 open-connector PRODUCT surface: the
// reviewed-action catalog, the correlate-able authorization attempts and
// the TRUSTED OC prepare path. Shared invariants with the rest of the
// /api/v1/apps surface: tenant scope always comes from the authenticated
// context; a cross-tenant id and a missing one are both 404; upstream or
// credential errors never leak (fixed codes + fixed messages only); and
// the handlers accept LOCAL services only — no admin client is ever
// injected here (the control worker owns the runtime admin secret).

// ---------------------------------------------------------------------------
// OCPrepare DTO (plan T13 interface, decode sketch verbatim in semantics)
// ---------------------------------------------------------------------------

// OCPrepareInput is the ONLY client-supplied shape of a trusted OC prepare:
// which connection, which reviewed action, and the action input. Every
// trusted field — risk, app version, runtime, alias, external identity,
// schema — is derived server-side from the reviewed definition and the
// tenant's live binding (oc_prepare.go), which is exactly why the decoder
// below rejects ANY additional field: a client cannot smuggle a channel
// for them because the struct has none, and DisallowUnknownFields turns
// the attempt into a 400.
type OCPrepareInput struct {
	ConnectionID string          `json:"connection_id"`
	ActionID     string          `json:"action_id"`
	Input        json.RawMessage `json:"input"`
}

// ocPrepareMaxBytes bounds one prepare body (1 MiB, plan ruling 1).
const ocPrepareMaxBytes = 1 << 20

// DecodeOCPrepare strictly decodes one prepare request: unknown fields are
// rejected (client-forged trusted fields such as risk/runtime_id never
// parse), exactly one JSON value is allowed (trailing input is an error),
// and the three bound fields must be present with valid JSON input.
func DecodeOCPrepare(r io.Reader) (OCPrepareInput, error) {
	var in OCPrepareInput
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		return in, err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return in, errors.New("trailing input")
	}
	if in.ConnectionID == "" || in.ActionID == "" || !json.Valid(in.Input) {
		return in, errors.New("invalid prepare")
	}
	return in, nil
}

// ---------------------------------------------------------------------------
// Catalog — GET /apps/catalog (AppInstallationHandler)
// ---------------------------------------------------------------------------

// ocCatalogEntry is ONE action definition the calling tenant may actually
// use. DTO conventions codified from the T06 reviews:
//
//   - target naming (T06-F-08): the approval's target is DERIVED
//     SERVER-SIDE at prepare time — when the frozen schema declares a
//     "target" string property, the approved input's value for it becomes
//     the target; targetless reads use the fixed action id as the target.
//     Clients never send a target, risk or runtime: DecodeOCPrepare
//     rejects those fields outright.
//   - no_auth discrimination (T06-F02): listing an action here means the
//     reviewed manifest's auth field required at least one scope —
//     scope-free (no_auth-shaped) actions are default-excluded from the
//     first-phase tenant catalog and are not prepareable either.
//   - multi-provider fail-closed (T06-F03): an entry appears only when
//     the definition's provider equals the provider of one of the
//     tenant's LIVE bindings; a same-named action of a foreign provider is
//     never listed, never resolvable.
//   - published-flip audit trail (T06-F04): published + schema_digest
//     expose the review state in DTO metadata — the digest is the pinned
//     SHA-256 of the exact frozen schema bytes, so any content change
//     (which requires a NEW app version) is visible, and the boolean
//     carries the staged/published workflow flip. (Full audit-log joins
//     land with the T16 observability pass.)
type ocCatalogEntry struct {
	ActionID       string          `json:"action_id"`
	AppID          string          `json:"app_id"`
	AppVersion     string          `json:"app_version"`
	Provider       string          `json:"provider"`
	Risk           string          `json:"risk"`
	ConnectionID   string          `json:"connection_id"`
	SchemaDigest   string          `json:"schema_digest"`
	InputSchema    json.RawMessage `json:"input_schema"`
	RequiredScopes []string        `json:"required_scopes"`
	Published      bool            `json:"published"`
}

// ListOCCatalog GET /apps/catalog — the published, tenant-reachable action
// definitions with their (frozen, safe) input schemas. Filtering happens
// per binding: only actions reachable through an ACTIVE binding → tenant
// connection → ACTIVE installation chain, published, scope-bearing and
// provider-matching are listed. Any other tenant's rows are invisible by
// construction (every lookup is tenant-scoped).
func (h *AppInstallationHandler) ListOCCatalog(c *gin.Context) {
	tenantID, _, _, ok := appTenantScope(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	var bindings []appconnectorrepo.OCBindingRow
	if err := h.db.WithContext(ctx).
		Where("tenant_id = ? AND state = ?", tenantID, appconnector.OCBindingActive).
		Find(&bindings).Error; err != nil {
		appFail(c, http.StatusInternalServerError, "OC_CATALOG_FAILED", "failed to list the action catalog")
		return
	}
	entries := make([]ocCatalogEntry, 0)
	for _, b := range bindings {
		var conn appconnectorrepo.ConnectionRow
		if err := h.db.WithContext(ctx).
			Where("tenant_id = ? AND id = ?", tenantID, b.ConnectionID).First(&conn).Error; err != nil {
			continue // binding without a live tenant connection: unreachable
		}
		var inst appconnectorrepo.InstallationRow
		if err := h.db.WithContext(ctx).
			Where("tenant_id = ? AND id = ?", tenantID, conn.InstallationID).First(&inst).Error; err != nil {
			continue
		}
		if inst.State != appconnector.InstallationActive {
			continue
		}
		var defs []appconnectorrepo.OCDefinitionRow
		if err := h.db.WithContext(ctx).
			Where("app_id = ? AND app_version = ? AND provider = ? AND published = ?",
				inst.AppID, inst.AppVersion, b.Provider, true).
			Where("required_scopes NOT IN ?", []string{"", "[]"}).
			Find(&defs).Error; err != nil {
			appFail(c, http.StatusInternalServerError, "OC_CATALOG_FAILED", "failed to list the action catalog")
			return
		}
		for _, d := range defs {
			entries = append(entries, ocCatalogEntry{
				ActionID: d.ActionID, AppID: d.AppID, AppVersion: d.AppVersion,
				Provider: d.Provider, Risk: d.Risk, ConnectionID: b.ConnectionID,
				SchemaDigest:   d.SchemaDigest,
				InputSchema:    json.RawMessage(d.InputSchema),
				RequiredScopes: parseOCScopes(d.RequiredScopes),
				Published:      d.Published,
			})
		}
	}
	appOK(c, http.StatusOK, entries)
}

// parseOCScopes decodes the frozen scope JSON; an unreadable list yields an
// empty one — the handler never invents scopes the review did not record.
func parseOCScopes(raw string) []string {
	var scopes []string
	if err := json.Unmarshal([]byte(raw), &scopes); err != nil || scopes == nil {
		return []string{}
	}
	return scopes
}

// ---------------------------------------------------------------------------
// Authorization attempts (AppConnectionHandler)
// ---------------------------------------------------------------------------

// ocAttemptInput is the body of POST /apps/connections/:id/authorization-
// attempts. credential is the provider API key for key-based apps; it
// exists ONLY to flow into the T07 encrypted handoff (sealed under the
// attempt id as AAD, then straight into the operations outbox as
// ciphertext) — it is never echoed, logged or persisted in the clear.
type ocAttemptInput struct {
	Credential string `json:"credential"`
}

// SetOCConnectionService wires the local connection lifecycle service
// (container injection point). Until it is called, attempt endpoints fail
// closed with 503 — nothing is minted without the correlate-able lifecycle.
//
// The OC attempt endpoints ride AppActionHandler (not AppConnectionHandler)
// because Go struct fields must live in the type's defining file, and this
// task's write set modifies only app_connector_action.go in this package.
// The ROUTES still sit under the connection-management write gate; the
// receiver is an accident of file layout, not of policy.
func (h *AppActionHandler) SetOCConnectionService(s *appconnectorsvc.OCConnectionService) {
	h.ocConnections = s
}

// BeginOCAuthorization POST /apps/connections/:id/authorization-attempts —
// starts one single-use authorization attempt for the connection. The
// route carries the existing connection-management write gate
// (owner/admin); a PERSONAL connection additionally admits only its owner.
// The response carries the attempt id and its status — never a state
// parameter, never a token, never the credential back.
func (h *AppActionHandler) BeginOCAuthorization(c *gin.Context) {
	tenantID, _, userID, ok := appTenantScope(c)
	if !ok {
		return
	}
	var conn appconnectorrepo.ConnectionRow
	if err := h.db.WithContext(c.Request.Context()).
		Where("tenant_id = ? AND id = ?", tenantID, c.Param("id")).First(&conn).Error; err != nil {
		appFail(c, http.StatusNotFound, "CONNECTION_NOT_FOUND", "connection not found")
		return
	}
	if conn.Kind == appconnector.ConnectionKindPersonal && conn.OwnerID != userID {
		appFail(c, http.StatusForbidden, "NOT_CONNECTION_OWNER",
			"a personal connection may only start its own authorizations")
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, ocPrepareMaxBytes)
	var input ocAttemptInput
	if err := c.ShouldBindJSON(&input); err != nil {
		appFail(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid authorization-attempt body")
		return
	}
	if h.ocConnections == nil {
		appFail(c, http.StatusServiceUnavailable, "OC_AUTHORIZATION_NOT_CONFIGURED",
			"the open-connector authorization lifecycle is not wired in this deployment")
		return
	}
	subject := appconnector.OCSubject{TenantID: tenantID, ActorID: userID}
	var (
		attemptID string
		err       error
	)
	if strings.TrimSpace(input.Credential) != "" {
		attemptID, err = h.ocConnections.BeginAPIKey(c.Request.Context(), subject, conn.ID, input.Credential)
	} else {
		attemptID, err = h.ocConnections.Begin(c.Request.Context(), subject, conn.ID)
	}
	if err != nil {
		appFailOCAttempt(c, err)
		return
	}
	appOK(c, http.StatusCreated, gin.H{"attempt_id": attemptID, "status": appconnectorsvc.OCAttemptPending})
}

// GetOCAuthorizationAttempt GET /apps/authorization-attempts/:id — the
// status of one attempt, visible only to the tenant+actor that started it
// (both mismatches are 404). The DTO is deliberately minimal: status,
// connection and expiry. No OAuth state, no token, no credential and no
// auth URL is stored on or echoed from this row (the restricted
// authorization URL is produced by the control worker's external flow and
// surfaced to the actor through that flow only).
func (h *AppActionHandler) GetOCAuthorizationAttempt(c *gin.Context) {
	tenantID, _, userID, ok := appTenantScope(c)
	if !ok {
		return
	}
	var row appconnectorrepo.OCAuthorizationAttemptRow
	if err := h.db.WithContext(c.Request.Context()).
		Where("id = ?", c.Param("id")).First(&row).Error; err != nil {
		appFail(c, http.StatusNotFound, "ATTEMPT_NOT_FOUND", "authorization attempt not found")
		return
	}
	if row.TenantID != tenantID || row.ActorID != userID {
		appFail(c, http.StatusNotFound, "ATTEMPT_NOT_FOUND", "authorization attempt not found")
		return
	}
	appOK(c, http.StatusOK, gin.H{
		"attempt_id":    row.ID,
		"status":        row.State,
		"connection_id": row.ConnectionID,
		"expires_at":    row.ExpiresAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
	})
}

// appFailOCAttempt maps the connection-lifecycle service errors onto the
// fixed code table (missing/cross-space 404, permission 403, params 400,
// version/state 409, unconfigured 503). Messages are static: no upstream
// error text crosses the wire.
func appFailOCAttempt(c *gin.Context, err error) {
	switch {
	case errors.Is(err, appconnectorsvc.ErrConnectionForbidden),
		errors.Is(err, appconnectorsvc.ErrSubjectNotMember),
		errors.Is(err, appconnectorsvc.ErrMissingSubject):
		appFail(c, http.StatusForbidden, "OC_ATTEMPT_FORBIDDEN", "the actor may not authorize this connection")
	case errors.Is(err, appconnectorsvc.ErrInstallationNotActive):
		appFail(c, http.StatusConflict, "INSTALLATION_NOT_ACTIVE", "the installation behind the connection is not active")
	case errors.Is(err, appconnectorsvc.ErrOCAttemptState),
		errors.Is(err, appconnectorsvc.ErrOCAttemptExpired):
		appFail(c, http.StatusConflict, "OC_ATTEMPT_STATE", "the authorization attempt is not in a state that accepts this operation")
	case errors.Is(err, appconnectorsvc.ErrTransientCipherUnavailable),
		errors.Is(err, appconnectorrepo.ErrOCRuntimeUnavailable):
		appFail(c, http.StatusServiceUnavailable, "OC_NOT_CONFIGURED",
			"the open-connector runtime is not configured in this deployment")
	case errors.Is(err, gorm.ErrRecordNotFound):
		appFail(c, http.StatusNotFound, "ATTEMPT_NOT_FOUND", "authorization attempt not found")
	default:
		appFail(c, http.StatusInternalServerError, "OC_ATTEMPT_FAILED", "failed to start the authorization attempt")
	}
}

// ---------------------------------------------------------------------------
// Trusted OC prepare (AppActionHandler)
// ---------------------------------------------------------------------------

// SetOCPreparer wires the TRUSTED OC prepare path (container injection
// point). Until it is called, OC prepare fails closed with 503 — no
// action row is ever minted from client-supplied trusted fields.
func (h *AppActionHandler) SetOCPreparer(p *appconnectorsvc.OCPreparer) {
	h.ocPreparer = p
}

// PrepareOCAction POST /apps/oc/actions/prepare — the plan's trusted OC
// entry point. The body is strictly decoded (1 MiB cap, unknown fields
// rejected), and EVERY trusted input — risk, app version, input schema,
// runtime, provider, external identity, alias, binding generation — is
// filled server-side by OCPreparer from the reviewed definition and the
// tenant's live binding. The response is the standard action detail view:
// the fresh action id, its digest, the awaiting_approval state, the
// server-derived target and the fence an approval must echo.
func (h *AppActionHandler) PrepareOCAction(c *gin.Context) {
	tenantID, _, userID, ok := appTenantScope(c)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, ocPrepareMaxBytes)
	input, err := DecodeOCPrepare(c.Request.Body)
	if err != nil {
		appFail(c, http.StatusBadRequest, "INVALID_OC_PREPARE",
			"connection_id, action_id and a single valid JSON input object are required; trusted fields are server-derived")
		return
	}
	if h.ocPreparer == nil {
		appFail(c, http.StatusServiceUnavailable, "OC_PREPARE_NOT_CONFIGURED",
			"the open-connector prepare path is not wired in this deployment")
		return
	}
	id, err := h.ocPreparer.PrepareOC(c.Request.Context(),
		appconnector.OCSubject{TenantID: tenantID, ActorID: userID},
		input.ConnectionID, input.ActionID, input.Input)
	if err != nil {
		appFailOCPrepare(c, err)
		return
	}
	row, ok := h.appActionByID(c, tenantID, id)
	if !ok {
		return
	}
	appOK(c, http.StatusCreated, h.actionDetailFor(c, tenantID, row))
}

// appFailOCPrepare maps the trusted-prepare errors onto the fixed code
// table. Unreachable definitions — no live binding in THIS tenant, a
// disabled installation, an unpublished/no_auth review, a foreign provider
// — are all 404: cross-space and missing are indistinguishable, and none
// of them reveal which one fired.
func appFailOCPrepare(c *gin.Context, err error) {
	switch {
	case errors.Is(err, appconnectorsvc.ErrOCDefinitionUnreachable),
		errors.Is(err, appconnectorsvc.ErrOCDefinitionUnpublished),
		errors.Is(err, appconnectorsvc.ErrOCDefinitionNoAuthExcluded),
		errors.Is(err, gorm.ErrRecordNotFound):
		appFail(c, http.StatusNotFound, "ACTION_NOT_REACHABLE",
			"the action is not available through this connection in this workspace")
	case errors.Is(err, appconnectorsvc.ErrOCDefinitionInvalid),
		errors.Is(err, appconnectorsvc.ErrInvalidAction),
		errors.Is(err, appconnectorsvc.ErrOCArgsRejectedBySchema),
		errors.Is(err, appconnector.ErrInvalidArgs):
		appFail(c, http.StatusBadRequest, "INVALID_OC_ACTION",
			"the input does not satisfy the reviewed action definition")
	default:
		appFail(c, http.StatusInternalServerError, "OC_PREPARE_FAILED", "failed to prepare the action")
	}
}
