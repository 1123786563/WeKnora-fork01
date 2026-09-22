package appconnector

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	appconn "github.com/Tencent/WeKnora/internal/modules/appconnector"
	repoappconn "github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// T14: the agent tool-call binding. One logical model tool call — identified
// by the engine-attached (tenant, session, tool_call_id), never by
// model-supplied fields — binds to exactly ONE persisted open-connector app
// action. The binding is what makes tool-call replay safe: identical
// arguments read the same action back, changed arguments are a conflict, and
// the model can never approve or execute anything through this surface.

var (
	// ErrOCToolArgsConflict: the same (tenant, session, tool call) was
	// presented with DIFFERENT arguments. The persisted binding is immutable;
	// a changed call must start a NEW tool call (and therefore a NEW action).
	ErrOCToolArgsConflict = errors.New("oc_tool_args_conflict")
	// ErrOCToolBindingExists: the binding insert lost the uniqueness race.
	// The transaction rolled back atomically; the caller re-reads the single
	// winner and — for identical arguments — returns its action.
	ErrOCToolBindingExists = errors.New("oc_tool_binding_exists")
	// ErrOCToolBindingInvalid: malformed binding input rejected before any
	// database access (missing identity fields or digest).
	ErrOCToolBindingInvalid = errors.New("oc_tool_binding_invalid")
)

// OCToolBinding is the domain projection of one tool-call binding row.
type OCToolBinding struct {
	TenantID     uint64
	SessionID    string
	ToolCallID   string
	ConnectionID string
	// ActionName is the addressed open-connector action (e.g. "github.search");
	// ActionID is the persisted app_actions row id the call is bound to.
	ActionName string
	ActionID   string
	// ArgsDigest covers the FULL logical arguments: the bound identity
	// (tenant, session, tool call), the connection, the action and the
	// normalized input.
	ArgsDigest string
}

// OCToolBindingRow persists one immutable tool-call binding. The composite
// primary key IS the identity an approval binds; args_digest decides replay
// equality; action_id is 1:1 with the app_actions row (UNIQUE per tenant).
type OCToolBindingRow struct {
	TenantID     uint64 `gorm:"primaryKey;column:tenant_id"`
	SessionID    string `gorm:"primaryKey;column:session_id"`
	ToolCallID   string `gorm:"primaryKey;column:tool_call_id"`
	ConnectionID string `gorm:"column:connection_id;not null"`
	ActionName   string `gorm:"column:action_name;not null;default:''"`
	ActionID     string `gorm:"column:action_id;not null;uniqueIndex:uq_oc_tool_bindings_tenant_action"`
	ArgsDigest   string `gorm:"column:args_digest;not null"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (OCToolBindingRow) TableName() string { return "connector_tool_bindings" }

// OCToolBindingStore persists tool-call bindings over gorm.
type OCToolBindingStore struct{ db *gorm.DB }

// NewOCToolBindingStore builds the binding store.
func NewOCToolBindingStore(db *gorm.DB) *OCToolBindingStore {
	return &OCToolBindingStore{db: db}
}

// FindBinding loads the binding of one logical tool call. not-found is a
// normal answer (first prepare), not an error.
func (s *OCToolBindingStore) FindBinding(ctx context.Context, tenant uint64, sessionID, toolCallID string) (OCToolBinding, bool, error) {
	if s == nil || s.db == nil {
		return OCToolBinding{}, false, ErrOCToolBindingInvalid
	}
	var row OCToolBindingRow
	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND session_id = ? AND tool_call_id = ?", tenant, sessionID, toolCallID).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return OCToolBinding{}, false, nil
	}
	if err != nil {
		return OCToolBinding{}, false, err
	}
	return OCToolBinding{
		TenantID: row.TenantID, SessionID: row.SessionID, ToolCallID: row.ToolCallID,
		ConnectionID: row.ConnectionID, ActionName: row.ActionName,
		ActionID: row.ActionID, ArgsDigest: row.ArgsDigest,
	}, true, nil
}

// toolActionRowOf maps a domain action to its persistence row exactly like
// ActionStore.CreateAction does. The mapping lives here because the binding
// insert must share ONE transaction with the action insert, and the frozen
// ActionStore face offers no transaction-scoped create.
func toolActionRowOf(a appconn.Action, snapshot, digest, state string) (repoappconn.ActionRow, error) {
	if a.ID == "" || a.TenantID == 0 || snapshot == "" || digest == "" || state == "" {
		return repoappconn.ActionRow{}, ErrOCToolBindingInvalid
	}
	ocJSON := ""
	if a.OC != nil {
		b, err := json.Marshal(a.OC)
		if err != nil {
			return repoappconn.ActionRow{}, err
		}
		ocJSON = string(b)
	}
	digestVersion := int64(a.DigestVersion)
	if digestVersion == 0 {
		digestVersion = appconn.CurrentDigestVersion
	}
	return repoappconn.ActionRow{
		ID: a.ID, TenantID: a.TenantID, ActorID: a.ActorID, ConnectionID: a.ConnectionID,
		AppVersion: a.Version, Target: a.Target, Risk: a.Risk, AuthVersion: a.AuthVersion,
		ArgsSnapshot: snapshot, ArgsDigest: digest, State: state,
		OCBindingJSON: ocJSON, DigestVersion: digestVersion,
	}, nil
}

// CreateWithAction persists the action row AND the binding row in ONE
// transaction (T14 ruling: PrepareForTool creates Action+binding atomically).
// When the binding insert loses the uniqueness race — another racer with the
// same (tenant, session, tool call) committed first — the whole transaction
// rolls back (no orphan action row) and ErrOCToolBindingExists is returned so
// the caller can read back the single winner.
func (s *OCToolBindingStore) CreateWithAction(ctx context.Context, b OCToolBinding, a appconn.Action, snapshot, digest, state string) error {
	if s == nil || s.db == nil {
		return ErrOCToolBindingInvalid
	}
	if b.TenantID == 0 || b.SessionID == "" || b.ToolCallID == "" || b.ActionID == "" || b.ArgsDigest == "" {
		return fmt.Errorf("%w: binding identity and digest are required", ErrOCToolBindingInvalid)
	}
	actionRow, err := toolActionRowOf(a, snapshot, digest, state)
	if err != nil {
		return err
	}
	bindingRow := OCToolBindingRow{
		TenantID: b.TenantID, SessionID: b.SessionID, ToolCallID: b.ToolCallID,
		ConnectionID: b.ConnectionID, ActionName: b.ActionName,
		ActionID: b.ActionID, ArgsDigest: b.ArgsDigest,
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&actionRow).Error; err != nil {
			return err
		}
		res := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&bindingRow)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrOCToolBindingExists
		}
		return nil
	})
}

// OCToolBindingService is the agent-facing open-connector facade (it
// satisfies tools.OCActionFacade structurally). PrepareForTool binds one
// logical tool call to one persisted action through the TRUSTED T09
// preparation chain; StatusForTool reads the action's lifecycle state under
// the caller's tenant.
//
// The surface deliberately exposes NO approve and NO execute face: the model
// can only request preparation and query. Approval and dispatch stay on the
// human-driven actions surface, the ActionService budget entry is entered
// exactly once (never from the tool layer — no GatedAdapter, no second
// Begin), and the tool can therefore never widen the Begin→claim orphan
// window (T13-F-5): it allocates nothing.
type OCToolBindingService struct {
	actions      ActionStoreSource
	catalog      OCDefinitionResolver
	bindings     appconn.OCBindingStore
	toolBindings *OCToolBindingStore
}

// NewOCToolBindingService builds the facade. All four dependencies are
// required; a nil dependency is a wiring bug and fails on first use.
func NewOCToolBindingService(
	actions ActionStoreSource,
	catalog OCDefinitionResolver,
	bindings appconn.OCBindingStore,
	toolBindings *OCToolBindingStore,
) *OCToolBindingService {
	return &OCToolBindingService{actions: actions, catalog: catalog, bindings: bindings, toolBindings: toolBindings}
}

// PrepareForTool resolves or creates the action one logical tool call is
// bound to:
//
//   - an existing binding with the SAME args digest answers with the bound
//     action (identical replay — recovery, retry, re-ask — is idempotent);
//   - an existing binding with a DIFFERENT digest is ErrOCToolArgsConflict;
//   - otherwise the trusted T09 chain fills every bound input server-side
//     (reviewed definition through the tenant's own wiring, execution
//     binding from the LIVE binding row, schema-validated normalized args)
//     and the action row + binding row land in ONE transaction. Racing
//     identical calls read back the single winner.
//
// No idempotency beyond the tool call identity is claimed: a NEW tool call
// id always prepares a NEW action, and the dispatch replay window belongs to
// the durable dispatch record (T10), not to this binding.
func (s *OCToolBindingService) PrepareForTool(
	ctx context.Context, subject appconn.OCSubject, sessionID, toolCallID, connectionID, actionID string, input json.RawMessage,
) (string, error) {
	if subject.TenantID == 0 || subject.ActorID == "" || sessionID == "" || toolCallID == "" || connectionID == "" || actionID == "" {
		return "", fmt.Errorf("%w: subject, session, tool call, connection and action are required", ErrInvalidAction)
	}
	norm, err := appconn.NormalizeArgs(input)
	if err != nil {
		return "", err
	}
	digest := octoolArgsDigest(subject.TenantID, sessionID, toolCallID, connectionID, actionID, norm)
	if existing, found, ferr := s.toolBindings.FindBinding(ctx, subject.TenantID, sessionID, toolCallID); ferr != nil {
		return "", ferr
	} else if found {
		if existing.ArgsDigest != digest {
			return "", fmt.Errorf("%w: tool call %s is already bound with different arguments; start a new tool call", ErrOCToolArgsConflict, toolCallID)
		}
		return existing.ActionID, nil
	}
	// Trusted preparation, mirroring OCPreparer.PrepareOC: reviewed PUBLISHED
	// definition through the tenant's own chain, execution identity from the
	// LIVE binding, args validated against the frozen schema.
	def, err := s.catalog.GetOCDefinition(ctx, subject.TenantID, connectionID, actionID)
	if err != nil {
		return "", err
	}
	binding, err := s.bindings.GetBinding(ctx, subject.TenantID, connectionID)
	if err != nil {
		return "", err
	}
	if binding.State != appconn.OCBindingActive {
		return "", fmt.Errorf("%w: binding for connection %q is %s", ErrInvalidAction, connectionID, binding.State)
	}
	if err := validateOCArgsAgainstSchema(def.InputSchema, norm); err != nil {
		return "", fmt.Errorf("%w: %v", ErrOCArgsRejectedBySchema, err)
	}
	a := appconn.Action{
		ID:            "ocact_" + uuid.NewString(),
		TenantID:      subject.TenantID,
		ActorID:       subject.ActorID,
		ConnectionID:  connectionID,
		Version:       def.AppVersion,
		Target:        deriveOCTarget(def, norm),
		Risk:          def.Risk,
		Args:          norm,
		AuthVersion:   binding.AuthVersion,
		OC:            ocExecutionBindingOf(binding, def),
		DigestVersion: appconn.CurrentDigestVersion,
	}
	actionDigest, err := appconn.ActionDigest(a)
	if err != nil {
		return "", err
	}
	bindingRow := OCToolBinding{
		TenantID: subject.TenantID, SessionID: sessionID, ToolCallID: toolCallID,
		ConnectionID: connectionID, ActionName: actionID,
		ActionID: a.ID, ArgsDigest: digest,
	}
	if err := s.toolBindings.CreateWithAction(ctx, bindingRow, a, string(norm), actionDigest, appconn.ActionAwaitingApproval); err != nil {
		if errors.Is(err, ErrOCToolBindingExists) {
			winner, found, rerr := s.toolBindings.FindBinding(ctx, subject.TenantID, sessionID, toolCallID)
			if rerr != nil {
				return "", errors.Join(err, rerr)
			}
			if !found {
				return "", err
			}
			if winner.ArgsDigest != digest {
				return "", fmt.Errorf("%w: tool call %s is already bound with different arguments; start a new tool call", ErrOCToolArgsConflict, toolCallID)
			}
			return winner.ActionID, nil
		}
		return "", err
	}
	// Covering scope pre-authorization, with the same semantics and ordering
	// as ActionService.persistPrepared: the action+binding transaction is
	// committed first; a covering pre-auth then births the action authorized.
	if pre, ok, perr := s.matchPreAuthorization(ctx, a, time.Now()); perr != nil {
		return "", perr
	} else if ok {
		if aerr := s.actions.SaveApproval(ctx, a.ID, actionDigest, "preauth:"+pre.ID, pre.ValidUntil, 1); aerr != nil {
			return "", aerr
		}
		if serr := s.actions.SetActionState(ctx, a.ID, appconn.ActionAwaitingApproval, appconn.ActionAuthorized); serr != nil {
			return "", serr
		}
	}
	return a.ID, nil
}

// StatusForTool reports the action's lifecycle state to the calling tenant
// only. A row of another tenant is indistinguishable from a missing one —
// cross-space invisible by construction.
func (s *OCToolBindingService) StatusForTool(ctx context.Context, subject appconn.OCSubject, actionID string) (string, error) {
	if subject.TenantID == 0 || actionID == "" {
		return "", fmt.Errorf("%w: subject and action are required", ErrInvalidAction)
	}
	row, err := s.actions.FindAction(ctx, actionID)
	if err != nil {
		return "", err
	}
	if row.TenantID != subject.TenantID {
		return "", repoappconn.ErrActionNotFound
	}
	return row.State, nil
}

// matchPreAuthorization mirrors ActionService.matchPreAuthorization: the
// tenant pre-authorization covering the action under the domain's own-scope
// rules (a send pre-auth matches send only; a general write grant never
// satisfies send).
func (s *OCToolBindingService) matchPreAuthorization(ctx context.Context, a appconn.Action, now time.Time) (appconn.PreAuthorization, bool, error) {
	pres, err := s.actions.ListPreAuthorizations(ctx, a.TenantID)
	if err != nil {
		return appconn.PreAuthorization{}, false, err
	}
	for _, p := range pres {
		if appconn.PreAuthorizationCovers(p, a, now) {
			return p, true, nil
		}
	}
	return appconn.PreAuthorization{}, false, nil
}

// octoolArgsDigest hashes the FULL logical tool call arguments as ONE
// structured JSON document (digest material generation 2 — never
// newline-bearing text concatenation): the material version, the bound
// identity (tenant, session, tool call), the addressed connection and
// action, and the normalized input. Persisted in the binding row, it decides
// replay equality: identical replay reads the same action; ANY change —
// args, action, or connection — is a conflict, never a rebind.
func octoolArgsDigest(tenant uint64, sessionID, toolCallID, connectionID, actionID string, norm json.RawMessage) string {
	material := struct {
		Version    int
		Tenant     uint64
		Session    string
		ToolCall   string
		Connection string
		Action     string
		Args       json.RawMessage
	}{appconn.CurrentDigestVersion, tenant, sessionID, toolCallID, connectionID, actionID, norm}
	b, err := json.Marshal(material)
	if err != nil {
		// Marshaling this closed struct cannot fail; if it ever did, hash a
		// fixed marker so the answer stays deterministic (never empty).
		b = []byte("octool-args-digest-marshal-error")
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
