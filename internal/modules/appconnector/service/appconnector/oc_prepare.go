package appconnector

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	appconn "github.com/Tencent/WeKnora/internal/modules/appconnector"

	"github.com/google/uuid"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// ErrOCArgsRejectedBySchema: the (normalized) arguments do not satisfy the
// frozen, reviewed input schema of the definition being prepared.
var ErrOCArgsRejectedBySchema = errors.New("oc_args_rejected_by_schema")

// OCDefinitionResolver resolves one reviewed action definition for a tenant
// through its own wiring; implemented by *OCCatalog (T06 GetOCDefinition
// chain: live tenant binding → tenant connection → ENABLED installation →
// published definition of the installed app version).
type OCDefinitionResolver interface {
	GetOCDefinition(ctx context.Context, tenant uint64, connectionID, actionID string) (appconn.OCDefinition, error)
}

// OCPreparer is the TRUSTED Prepare path for open-connector actions
// (plan T09): every bound input the approval digest covers is filled from
// server-side sources only —
//
//	risk + app version + input schema + schema digest → the reviewed
//	                                   definition (T06 catalog, published);
//	runtime + provider + external id + alias + binding generation + auth
//	version → the tenant's LIVE binding (T03 store);
//	action identity → a fresh server-generated id;
//	args → normalized, then validated against the compiled schema (R2:
//	       santhosh-tekuri v6, no loader).
//
// The interface carries NO channel for client-supplied risk, version,
// runtime or alias — a caller physically cannot smuggle them in, and the
// native Prepare path rejects any Action that arrives with OC set.
//
// PrepareOC intentionally performs no membership check of its own: the
// catalog chain already gates tenant reachability through the live binding
// and the enabled installation, the actor is persisted onto the row, and
// Execute's A02 guard re-checks the persisted subject on every dispatch.
type OCPreparer struct {
	actions  *ActionService
	catalog  OCDefinitionResolver
	bindings appconn.OCBindingStore
}

// NewOCPreparer builds the trusted OC prepare path. All three dependencies
// are required.
func NewOCPreparer(actions *ActionService, catalog OCDefinitionResolver, bindings appconn.OCBindingStore) *OCPreparer {
	return &OCPreparer{actions: actions, catalog: catalog, bindings: bindings}
}

// PrepareOC resolves the reviewed definition through the tenant's own
// wiring, validates the normalized args against its frozen input schema,
// fills the execution binding from the tenant's LIVE binding, and persists
// one awaiting_approval action under the current digest generation — the
// binding JSON, args snapshot and digest land in the row's single INSERT.
func (p *OCPreparer) PrepareOC(ctx context.Context, subject appconn.OCSubject, connectionID, actionID string, args json.RawMessage) (string, error) {
	if subject.TenantID == 0 || subject.ActorID == "" || connectionID == "" || actionID == "" {
		return "", fmt.Errorf("%w: subject, connection and action are required", ErrInvalidAction)
	}
	// 1. Reviewed, PUBLISHED definition through the tenant's OWN chain:
	// unreachable definitions, staged (unpublished) reviews, disabled
	// installations and foreign providers all refuse here.
	def, err := p.catalog.GetOCDefinition(ctx, subject.TenantID, connectionID, actionID)
	if err != nil {
		return "", err
	}
	// 2. The tenant's live binding: the ONLY source of runtime/external
	// identity. Read AFTER the catalog resolution so the freshest binding
	// generation is the one the digest binds.
	binding, err := p.bindings.GetBinding(ctx, subject.TenantID, connectionID)
	if err != nil {
		return "", err
	}
	if binding.State != appconn.OCBindingActive {
		return "", fmt.Errorf("%w: binding for connection %q is %s", ErrInvalidAction, connectionID, binding.State)
	}
	// 3. Normalize (exactly one JSON value, exact number literals) and
	// validate against the compiled frozen schema.
	norm, err := appconn.NormalizeArgs(args)
	if err != nil {
		return "", err
	}
	if err := validateOCArgsAgainstSchema(def.InputSchema, norm); err != nil {
		return "", fmt.Errorf("%w: %v", ErrOCArgsRejectedBySchema, err)
	}
	// 4. Build the action from server-side sources only.
	a := appconn.Action{
		ID:            "ocact_" + uuid.NewString(),
		TenantID:      subject.TenantID,
		ActorID:       subject.ActorID,
		ConnectionID:  connectionID,
		Version:       def.AppVersion, // app version from the reviewed definition
		Target:        deriveOCTarget(def, norm),
		Risk:          def.Risk, // risk from the reviewed publish record
		Args:          norm,
		AuthVersion:   binding.AuthVersion,
		OC:            ocExecutionBindingOf(binding, def),
		DigestVersion: appconn.CurrentDigestVersion,
	}
	return p.actions.persistPrepared(ctx, a)
}

// ocExecutionBindingOf snapshots the immutable execution identity from the
// tenant's live binding plus the reviewed definition.
func ocExecutionBindingOf(binding appconn.OCBinding, def appconn.OCDefinition) *appconn.OCExecutionBinding {
	return &appconn.OCExecutionBinding{
		RuntimeID:      binding.RuntimeID,
		Provider:       binding.Provider,
		ExternalID:     binding.ExternalID,
		Alias:          binding.Alias,
		ActionID:       def.ActionID,
		SchemaDigest:   def.SchemaDigest,
		BindingVersion: binding.BindingVersion,
	}
}

// deriveOCTarget derives the approval's target from the reviewed
// definition's own input field: when the frozen schema declares a "target"
// string property and the normalized args carry it, that concrete value is
// the target (the approval binds the exact object being acted on);
// otherwise — in particular for targetless reads — the fixed action id is
// the target. A non-string or empty value falls back to the fixed action
// id; the full args stay digest-bound either way.
func deriveOCTarget(def appconn.OCDefinition, norm json.RawMessage) string {
	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(def.InputSchema, &schema); err != nil || schema.Properties == nil {
		return def.ActionID
	}
	if _, declared := schema.Properties["target"]; !declared {
		return def.ActionID
	}
	var args map[string]json.RawMessage
	if err := json.Unmarshal(norm, &args); err != nil {
		return def.ActionID
	}
	raw, present := args["target"]
	if !present || len(raw) == 0 {
		return def.ActionID
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil || s == "" {
		return def.ActionID
	}
	return s
}

// validateOCArgsAgainstSchema compiles the frozen reviewed schema with the
// same untrusted-schema posture as the MCP tools (R2: santhosh-tekuri v6,
// Draft 2020, NO loader — file/URL references cannot resolve) and validates
// the normalized args against it. Args must be a JSON object at the root,
// matching the object-root rule every reviewed OC schema already carries.
func validateOCArgsAgainstSchema(schema, args []byte) error {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(schema))
	if err != nil {
		return fmt.Errorf("reviewed schema is not valid JSON: %w", err)
	}
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	compiler.UseLoader(nil)
	if err := compiler.AddResource(ocSchemaResource, doc); err != nil {
		return fmt.Errorf("reviewed schema rejected: %w", err)
	}
	compiled, err := compiler.Compile(ocSchemaResource)
	if err != nil {
		return fmt.Errorf("reviewed schema does not compile: %w", err)
	}
	value, err := jsonschema.UnmarshalJSON(bytes.NewReader(args))
	if err != nil {
		return fmt.Errorf("invalid arguments JSON: %w", err)
	}
	if _, ok := value.(map[string]any); !ok {
		return errors.New("arguments must be a JSON object")
	}
	return compiled.Validate(value)
}
