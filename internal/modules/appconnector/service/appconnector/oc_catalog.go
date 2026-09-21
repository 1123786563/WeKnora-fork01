package appconnector

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	appconn "github.com/Tencent/WeKnora/internal/appconnector"
	"github.com/santhosh-tekuri/jsonschema/v6"

	"gorm.io/gorm"
)

// Reviewed action-definition catalog for open connectors (T06).
//
// Publishing is a PLATFORM-CONTROL operation — management CLI / control
// plane only; no tenant-facing route reaches it (T13 owns routes, ruling
// 7). Every review record is frozen per (app, app_version, action_id): the
// exact schema bytes, their pinned SHA-256 digest, the required scopes and
// the execution risk. Any content change — including narrowing scopes —
// needs a NEW app version; the same triple may only be re-published
// byte-identically (or with only the published workflow flag flipped).
//
// RULING R2 (plan deviation, recorded): the plan text cites
// github.com/google/jsonschema-go, but the repository precedent (and
// already-present direct dependency) is github.com/santhosh-tekuri/
// jsonschema/v6, used exactly like internal/agent/tools/mcp_schema.go: the
// input schema is COMPILED — JSON well-formedness alone is not sufficient —
// with no URL loader, so file/URL-shaped references fail at review time.

const (
	// The closed risk vocabulary (ruling 6): execution risk comes from the
	// publish record, and any value outside this set means the definition
	// was never reviewed. Case-sensitive on purpose.
	OCRiskRead   = "read"
	OCRiskWrite  = "write"
	OCRiskSend   = "send"
	OCRiskDelete = "delete"
)

// ocSchemaResource is the in-memory resource name the frozen schema is
// compiled under; it never reaches a filesystem or network.
const ocSchemaResource = "urn:weknora:oc:input-schema"

var (
	// ErrOCDefinitionInvalid: the definition failed review validation —
	// blank identity fields (including the empty action set), a risk
	// outside read/write/send/delete, a schema that is not a JSON object,
	// does not compile, is file/URL-shaped, or a supplied digest that
	// disagrees with the schema bytes.
	ErrOCDefinitionInvalid = errors.New("oc_definition_invalid")
	// ErrOCDefinitionUnpublished: the definition exists for the resolved
	// app version but its published flag is false — staged reviews are
	// never tenant-visible.
	ErrOCDefinitionUnpublished = errors.New("oc_definition_unpublished")
	// ErrOCDefinitionNoAuthExcluded: a no_auth-shaped action (no required
	// scopes) — default-excluded from the first-phase tenant catalog
	// (ruling 8); the platform may still stage it.
	ErrOCDefinitionNoAuthExcluded = errors.New("oc_definition_noauth_excluded")
	// ErrOCDefinitionUnreachable: no tenant path to the definition — no
	// live binding for the connection in THIS tenant, no tenant connection
	// or installation, an install version whose actions were never
	// published, or a foreign provider.
	ErrOCDefinitionUnreachable = errors.New("oc_definition_unreachable")
)

// validOCRisk reports whether r is inside the closed review vocabulary.
func validOCRisk(r string) bool {
	return r == OCRiskRead || r == OCRiskWrite || r == OCRiskSend || r == OCRiskDelete
}

// OCSchemaDigest returns the pinned digest of a definition's schema: the
// lowercase-hex SHA-256 of the EXACT schema bytes. Executors compare this
// value to detect drift without re-parsing the schema.
func OCSchemaDigest(schema []byte) string {
	sum := sha256.Sum256(schema)
	return hex.EncodeToString(sum[:])
}

// ValidateOCDefinition runs the review gates on one definition:
//
//   - AppID, AppVersion, ActionID and Provider must all be present (a
//     definition with no action id — the empty action set — is refused);
//   - Risk must come from the closed read/write/send/delete vocabulary;
//   - InputSchema must be present, a JSON OBJECT at the root, and must
//     COMPILE as a JSON Schema (ruling R2) — and because the compiler
//     runs without a loader, any file:// or http(s):// reference fails
//     here, so file/URL-shaped action schemas never enter the catalog;
//   - a supplied SchemaDigest must match the pinned SHA-256 of the schema
//     bytes (the digest is never taken on trust; PublishOCDefinition pins
//     it when absent).
//
// Validation never touches the database or the network.
func ValidateOCDefinition(d appconn.OCDefinition) error {
	if d.AppID == "" || d.AppVersion == "" || d.ActionID == "" || d.Provider == "" {
		return fmt.Errorf("%w: app, version, action and provider are required", ErrOCDefinitionInvalid)
	}
	if !validOCRisk(d.Risk) {
		return fmt.Errorf("%w: risk %q is outside the reviewed vocabulary read/write/send/delete", ErrOCDefinitionInvalid, d.Risk)
	}
	if len(d.InputSchema) == 0 {
		return fmt.Errorf("%w: empty input schema", ErrOCDefinitionInvalid)
	}
	if err := compileOCInputSchema(d.InputSchema); err != nil {
		return fmt.Errorf("%w: %v", ErrOCDefinitionInvalid, err)
	}
	if d.SchemaDigest != "" && !strings.EqualFold(d.SchemaDigest, OCSchemaDigest(d.InputSchema)) {
		return fmt.Errorf("%w: schema digest %q does not match the schema bytes", ErrOCDefinitionInvalid, d.SchemaDigest)
	}
	return nil
}

// compileOCInputSchema compiles the frozen input schema with the same
// untrusted-schema posture as the MCP tools
// (internal/agent/tools/mcp_schema.go): the root must be a JSON object,
// embedded $defs/$refs resolve in-memory, and the compiler carries NO
// loader — any reference that would leave the document (file://,
// http(s)://) fails compilation instead of reaching the filesystem or
// network at review or dispatch time.
func compileOCInputSchema(schema []byte) error {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(schema))
	if err != nil {
		return fmt.Errorf("input schema is not valid JSON: %w", err)
	}
	if _, ok := doc.(map[string]any); !ok {
		return errors.New("input schema root must be a JSON object")
	}
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	compiler.UseLoader(nil)
	if err := compiler.AddResource(ocSchemaResource, doc); err != nil {
		return fmt.Errorf("input schema rejected: %w", err)
	}
	if _, err := compiler.Compile(ocSchemaResource); err != nil {
		return fmt.Errorf("input schema does not compile: %w", err)
	}
	return nil
}

// OCDefinitionStore is the persistence surface for reviewed definitions;
// implemented by repository/appconnector.OCStore (T06 additions).
type OCDefinitionStore interface {
	SaveDefinition(ctx context.Context, d appconn.OCDefinition) error
	GetDefinition(ctx context.Context, app, version, action string) (appconn.OCDefinition, error)
}

// OCTenantConnectionSource resolves a connection INSIDE one tenant;
// implemented by repository/appconnector.InstallationStore.GetConnection.
type OCTenantConnectionSource interface {
	GetConnection(ctx context.Context, tenant uint64, connectionID string) (appconn.Connection, error)
}

// OCTenantInstallationSource resolves an installation by row id inside one
// tenant; implemented by repository/appconnector.InstallationStore.
// GetInstallationByID (T06 addition).
type OCTenantInstallationSource interface {
	GetInstallationByID(ctx context.Context, tenant uint64, installationID string) (appconn.Installation, error)
}

// OCCatalog publishes reviewed definitions (platform control plane only)
// and resolves them for tenants through the binding → connection →
// installation chain.
type OCCatalog struct {
	definitions   OCDefinitionStore
	bindings      appconn.OCBindingStore
	connections   OCTenantConnectionSource
	installations OCTenantInstallationSource
}

// NewOCCatalog builds the catalog service. All four dependencies are
// required; a nil dependency is a wiring bug and fails on first use.
func NewOCCatalog(
	definitions OCDefinitionStore,
	bindings appconn.OCBindingStore,
	connections OCTenantConnectionSource,
	installations OCTenantInstallationSource,
) *OCCatalog {
	return &OCCatalog{
		definitions:   definitions,
		bindings:      bindings,
		connections:   connections,
		installations: installations,
	}
}

// PublishOCDefinition validates and persists one reviewed definition. The
// digest is pinned from the exact schema bytes: an absent SchemaDigest is
// filled in, a disagreeing one is rejected. Immutability is enforced by
// the store — the same (app, app_version, action_id) may only be
// re-published byte-identically (or with only the published workflow flag
// flipped); scope/schema/risk changes need a NEW app version (ruling 5).
//
// This method is the platform control entry; it must never sit behind a
// tenant-facing route (T13 owns routes; ruling 7).
func (c *OCCatalog) PublishOCDefinition(ctx context.Context, d appconn.OCDefinition) error {
	pinned := OCSchemaDigest(d.InputSchema)
	if d.SchemaDigest != "" && !strings.EqualFold(d.SchemaDigest, pinned) {
		return fmt.Errorf("%w: schema digest %q does not match the schema bytes", ErrOCDefinitionInvalid, d.SchemaDigest)
	}
	d.SchemaDigest = pinned
	if err := ValidateOCDefinition(d); err != nil {
		return err
	}
	return c.definitions.SaveDefinition(ctx, d)
}

// GetOCDefinition resolves one action definition for a tenant through its
// OWN wiring (ruling 4; the plan's lookup shape adapted to the frozen T03
// types, which carry no app/version on the binding — recorded as a
// deviation):
//
//	binding(tenant, connectionID) — the tenant's LIVE open-connector
//	binding gates reachability;
//	connection(tenant, connectionID) → installation(tenant, id) — the
//	tenant-scoped chain that yields app_id + the INSTALLED app_version;
//	definition(app, version, action) — the frozen review pinned to that
//	exact version.
//
// Tenant scope is enforced at EVERY step: another tenant re-using the
// same connection id has no binding there and sees nothing. The resolved
// definition must additionally be published, target the binding's
// provider, and — first-phase rule, ruling 8 — require at least one scope
// (no_auth actions are default-excluded; nothing here mints credentials
// or special-cases virtual connection ids). A disabled or
// reauthorization-parked installation blocks resolution entirely.
func (c *OCCatalog) GetOCDefinition(ctx context.Context, tenant uint64, connectionID, actionID string) (appconn.OCDefinition, error) {
	if tenant == 0 || connectionID == "" || actionID == "" {
		return appconn.OCDefinition{}, ErrOCDefinitionInvalid
	}
	binding, err := c.bindings.GetBinding(ctx, tenant, connectionID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return appconn.OCDefinition{}, fmt.Errorf("%w: no open-connector binding for connection %q in tenant %d", ErrOCDefinitionUnreachable, connectionID, tenant)
	}
	if err != nil {
		return appconn.OCDefinition{}, err
	}
	if binding.State != appconn.OCBindingActive {
		return appconn.OCDefinition{}, fmt.Errorf("%w: binding for connection %q is %s", ErrOCDefinitionUnreachable, connectionID, binding.State)
	}
	conn, err := c.connections.GetConnection(ctx, tenant, connectionID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return appconn.OCDefinition{}, fmt.Errorf("%w: no connection %q in tenant %d", ErrOCDefinitionUnreachable, connectionID, tenant)
	}
	if err != nil {
		return appconn.OCDefinition{}, err
	}
	inst, err := c.installations.GetInstallationByID(ctx, tenant, conn.InstallationID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return appconn.OCDefinition{}, fmt.Errorf("%w: no installation %q in tenant %d", ErrOCDefinitionUnreachable, conn.InstallationID, tenant)
	}
	if err != nil {
		return appconn.OCDefinition{}, err
	}
	if inst.State != appconn.InstallationActive {
		return appconn.OCDefinition{}, ErrInstallationNotActive
	}
	def, err := c.definitions.GetDefinition(ctx, inst.AppID, inst.Version, actionID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return appconn.OCDefinition{}, fmt.Errorf("%w: no definition for %s@%s/%s (install version mismatch)", ErrOCDefinitionUnreachable, inst.AppID, inst.Version, actionID)
	}
	if err != nil {
		return appconn.OCDefinition{}, err
	}
	if !def.Published {
		return appconn.OCDefinition{}, ErrOCDefinitionUnpublished
	}
	if len(def.RequiredScopes) == 0 {
		return appconn.OCDefinition{}, ErrOCDefinitionNoAuthExcluded
	}
	if def.Provider != binding.Provider {
		return appconn.OCDefinition{}, fmt.Errorf("%w: definition provider %q is not reachable through this tenant's %q binding", ErrOCDefinitionUnreachable, def.Provider, binding.Provider)
	}
	return def, nil
}
