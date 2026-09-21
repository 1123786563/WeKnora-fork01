// oc_dispatcher.go implements T11: the HTTP ActionDispatcher for frozen
// open-connector actions, plus the conservative result classification that
// preserves unknown outcomes.
//
// Wire facts consumed here are FROZEN by the T01 runtime-evidence contract
// (docs/integrations/open-connector-contract.md §3, upstream
// @oomol-lab/open-connector v1.5.0 @ 33dd4ad6ee22f9ce5158a1516a11d8b8566b5c8a):
//
//   - the error code field is errorCode (NOT code); executionId and
//     auditPersisted live inside meta (NOT at the top level);
//   - 400 invalid_input and 403 authorization_failed/connection_not_allowed
//     are the only T01-PROVEN pre-execution rejections (the cross-connection
//     denial happens BEFORE credential use — fixtures/cross_connection.json),
//     so they are the only non-2xx outcomes safely classifiable as failed;
//   - meta.auditPersisted=false does NOT negate the action result (§3.7,
//     fixtures/audit_failure.json) — success stays success and a local audit
//     warning is recorded on the action row;
//   - the idempotency fingerprint includes runtimeTokenId (undocumented
//     upstream behavior, §3.5): a replay under a different token 409s
//     instead of replaying — cross-token replay is never assumed, which is
//     one more reason this dispatcher sends exactly ONE POST per dispatch
//     and never re-sends a possibly-side-effected request.
package appconnector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	appconn "github.com/Tencent/WeKnora/internal/modules/appconnector"
	"github.com/Tencent/WeKnora/internal/modules/appconnector/openconnector"
	repoappconn "github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector"
)

// OCTokenSource reads the version-scoped restricted runtime token for one
// connection: the token handed to the runtime is scoped to exactly the
// connection's CURRENT authorization generation (tenant + connection +
// authVersion), so a revoked-and-rebound connection can never be reached
// with a stale token. The production implementation lands with the token
// lifecycle wiring (T13/T18); tests inject stubs.
type OCTokenSource interface {
	Token(ctx context.Context, tenant uint64, connectionID string, authVersion int64) (string, error)
}

// OCDispatchSource reads the durable open-connector dispatch record (T10):
// the claim that minted the operation-scoped idempotency key. Implemented by
// repository/appconnector.OCStore.GetOCDispatch.
type OCDispatchSource interface {
	GetOCDispatch(ctx context.Context, tenant uint64, actionID string) (appconn.OCDispatchRecord, error)
}

// OCRetryAfterSink durably records a provider throttle instant; implemented
// by repository/appconnector.OCStore.NoteOCProviderRetryAfter. Optional
// wiring (nil = 429s still classify as unknown, just without a recorded
// cooldown).
type OCRetryAfterSink interface {
	NoteOCProviderRetryAfter(ctx context.Context, provider string, retryAfter, now time.Time) error
}

// ocThrottleCooldown is the cooldown instant recorded on a 429. The frozen
// T02 executor surface (openconnector.Result) carries the HTTP status and
// the decoded envelope but NOT the Retry-After header value, so phase one
// records this fixed, deliberately short, fail-closed window instead of
// inventing a parse of data the client never surfaced; the store's
// forward-only rule keeps any longer observed instant authoritative. When a
// later task surfaces the raw header on the executor result, this constant
// becomes the fallback only.
const ocThrottleCooldown = 30 * time.Second

// ocResultValidators is the FROZEN result-validator registry (ruling 5):
// entries exist only for reviewed actions the T06 catalog published whose
// provider identity fields are confirmed. GitHub's reviewed read action
// get_current_user must return a numeric non-zero id and a non-empty login;
// write actions stay UNPUBLISHED here until T18 confirms their identity
// fields — a success envelope for an unregistered action stands as success
// (nothing to confirm), while a REGISTERED action whose result lacks its
// identity fields settles as unknown, never succeeded.
var ocResultValidators = map[string]func(json.RawMessage) bool{
	"github.get_current_user": ocValidGitHubIdentity,
}

// ocValidGitHubIdentity checks the reviewed identity fields of GitHub's
// get_current_user result: data.id must be a number > 0 and data.login a
// non-empty string. Anything else (null data, wrong types, zero id) fails —
// a success envelope that cannot prove WHICH provider identity answered is
// not a success we can record.
func ocValidGitHubIdentity(data json.RawMessage) bool {
	var id struct {
		ID    int64  `json:"id"`
		Login string `json:"login"`
	}
	if err := json.Unmarshal(data, &id); err != nil {
		return false
	}
	return id.ID > 0 && strings.TrimSpace(id.Login) != ""
}

// OCDispatcher is the ActionDispatcher for open-connector actions. It is the
// only outbound boundary of the OC pipeline: it re-verifies the persisted
// approval snapshot against the tenant's live binding, reads the
// version-scoped restricted token and the PERSISTED operation-scoped key
// (T10 claim record — never a connection id), sends exactly ONE POST with
// ONLY the approved normalized args, and classifies the raw result
// conservatively. It never retries and never re-sends: the T02 executor
// enforces single-POST/no-redirect, and this dispatcher adds no loop around
// it. The providerKey argument of Dispatch is deliberately ignored — for OC
// actions the wire key surface is the dispatch record's key alone.
type OCDispatcher struct {
	exec      openconnector.Executor
	bindings  appconn.OCBindingStore
	tokens    OCTokenSource
	records   OCDispatchSource
	retrySink OCRetryAfterSink
	now       func() time.Time
}

// NewOCDispatcher builds the open-connector dispatcher. All four
// dependencies are required for dispatch; a missing one is a wiring bug and
// fails closed as a pre-send rejection (never a panic, never a call).
func NewOCDispatcher(exec openconnector.Executor, bindings appconn.OCBindingStore, tokens OCTokenSource, records OCDispatchSource) *OCDispatcher {
	return &OCDispatcher{exec: exec, bindings: bindings, tokens: tokens, records: records, now: time.Now}
}

// UseOCRetryAfterSink wires the durable provider-throttle recorder. Without
// it a 429 still settles as unknown, but no cooldown is recorded.
func (d *OCDispatcher) UseOCRetryAfterSink(sink OCRetryAfterSink) { d.retrySink = sink }

// Dispatch performs the single outbound call for one claimed action. Every
// rejection BEFORE the executor call is a PROVEN pre-send refusal and wraps
// ErrDispatchNotStarted (settling as failed); everything after the call —
// transport failure, protocol failure, or an unprovable business result —
// settles as unknown, because the request may already have reached the
// provider.
func (d *OCDispatcher) Dispatch(ctx context.Context, snap ActionSnapshot, providerKey string) (DispatchOutcome, error) {
	if snap.OC == nil {
		return DispatchOutcome{}, fmt.Errorf("%w: action %s carries no open-connector execution binding", ErrDispatchNotStarted, snap.ID)
	}
	if d.exec == nil || d.bindings == nil || d.tokens == nil || d.records == nil {
		return DispatchOutcome{}, fmt.Errorf("%w: oc dispatcher wiring incomplete for %s", ErrDispatchNotStarted, snap.ID)
	}
	// 1. The PERSISTED dispatch record is the only source of the wire key:
	// the T10 claim minted the operation-scoped idempotency key exactly once
	// per (tenant, action). Never the connection id, never a re-mint.
	rec, err := d.records.GetOCDispatch(ctx, snap.TenantID, snap.ID)
	if err != nil {
		return DispatchOutcome{}, fmt.Errorf("%w: no durable dispatch record for %s: %v", ErrDispatchNotStarted, snap.ID, err)
	}
	if rec.Key == "" {
		return DispatchOutcome{}, fmt.Errorf("%w: dispatch record for %s carries no key", ErrDispatchNotStarted, snap.ID)
	}
	// 2. Snapshot vs binding: the approval's frozen execution identity must
	// still match the tenant's LIVE binding field for field — an approval
	// can never dispatch through a rebound or re-authorized connection.
	binding, err := d.bindings.GetBinding(ctx, snap.TenantID, snap.ConnectionID)
	if err != nil {
		return DispatchOutcome{}, fmt.Errorf("%w: binding for connection %q: %v", ErrDispatchNotStarted, snap.ConnectionID, err)
	}
	if drift := ocBindingDrift(binding, *snap.OC, rec, snap.AuthVersion); drift != "" {
		return DispatchOutcome{}, fmt.Errorf("%w: %s", ErrDispatchNotStarted, drift)
	}
	// 3. Version-scoped restricted token for exactly this connection and
	// authorization generation.
	token, err := d.tokens.Token(ctx, snap.TenantID, snap.ConnectionID, snap.AuthVersion)
	if err != nil {
		return DispatchOutcome{}, fmt.Errorf("%w: token for connection %q at version %d: %v", ErrDispatchNotStarted, snap.ConnectionID, snap.AuthVersion, err)
	}
	if token == "" || snap.OC.Alias == "" || !json.Valid(snap.Args) {
		return DispatchOutcome{}, fmt.Errorf("%w: empty token or alias, or invalid args snapshot for %s", ErrDispatchNotStarted, snap.ID)
	}
	// 4. Exactly ONE POST with ONLY the approved args (the persisted
	// normalized snapshot bytes). From here on the request may have reached
	// the provider: no error may settle as failed.
	res, xerr := d.exec.Execute(ctx, token, openconnector.Call{
		Alias:    snap.OC.Alias,
		ActionID: snap.OC.ActionID,
		Key:      rec.Key,
		Input:    snap.Args,
	})
	if xerr != nil {
		return DispatchOutcome{}, xerr
	}
	return d.classify(snap.OC, res), nil
}

// ocBindingDrift compares the persisted approval snapshot against the live
// binding and the durable claim record; it returns "" when they agree, or a
// short reason why the snapshot must not dispatch.
func ocBindingDrift(binding appconn.OCBinding, oc appconn.OCExecutionBinding, rec appconn.OCDispatchRecord, authVersion int64) string {
	if binding.State != appconn.OCBindingActive {
		return "binding for connection is " + binding.State
	}
	if binding.RuntimeID != oc.RuntimeID {
		return "runtime drifted since approval"
	}
	if binding.Provider != oc.Provider {
		return "provider drifted since approval"
	}
	if binding.ExternalID != oc.ExternalID {
		return "external identity drifted since approval"
	}
	if binding.Alias != oc.Alias {
		return "alias drifted since approval"
	}
	if binding.BindingVersion != oc.BindingVersion {
		return "binding generation drifted since approval"
	}
	if binding.AuthVersion != authVersion {
		return "authorization generation drifted since approval"
	}
	if rec.RuntimeID != oc.RuntimeID {
		return "claim record is pinned to another runtime"
	}
	return ""
}

// classify maps one raw executor result to a terminal outcome under the
// conservative T11 rules:
//
//   - 2xx + success=true → succeeded, after the action's frozen result
//     validator confirms the provider identity fields; an auditPersisted
//     =false warning is retained locally without negating the success;
//   - 400/403 + success=false + errorCode → failed: the only T01-proven
//     pre-execution rejections (invalid_input; authorization_failed /
//     connection_not_allowed denied before credential use);
//   - 429 → unknown, with the provider cooldown recorded durably;
//   - everything else (success=false with unprovable side effects, 402/404/
//     409, 5xx, odd combinations) → unknown: only a provider query may
//     resolve it, never a fabricated failure and never a re-send.
func (d *OCDispatcher) classify(oc *appconn.OCExecutionBinding, res openconnector.Result) DispatchOutcome {
	switch {
	case res.HTTPStatus >= 200 && res.HTTPStatus < 300 && res.Success:
		if validate, ok := ocResultValidators[oc.ActionID]; ok && !validate(res.Data) {
			return DispatchOutcome{
				Status:         appconn.ActionUnknown,
				ProviderResult: "provider identity fields missing for " + oc.ActionID,
			}
		}
		result := "provider succeeded"
		if res.ExecutionID != "" {
			result += "; execution " + res.ExecutionID
		}
		if res.AuditPersisted != nil && !*res.AuditPersisted {
			// Local audit warning (T01 §3.7): the runtime's audit write
			// failed while the action result itself is unchanged — the
			// success is retained and the warning is persisted on the
			// action row for the audit trail.
			result += "; audit warning: meta.auditPersisted=false"
		}
		// T13 R15 extension: surface meta.executionId as a STRUCTURED field
		// so the durable dispatch record can correlate the provider's
		// execution without parsing result text. Classification is
		// untouched.
		return DispatchOutcome{Status: appconn.ActionSucceeded, ProviderResult: result, ExecutionID: res.ExecutionID}
	case !res.Success && (res.HTTPStatus == http.StatusBadRequest || res.HTTPStatus == http.StatusForbidden) && res.Code != "":
		// Pre-execution rejection PROVEN by T01: 400 invalid_input and 403
		// authorization_failed/connection_not_allowed (the cross-connection
		// denial happens before any credential use — contract §3.3,
		// fixtures/cross_connection.json). The errorCode field (NOT code)
		// must be present: that is the proven envelope shape.
		return DispatchOutcome{
			Status: appconn.ActionFailed,
			ProviderResult: fmt.Sprintf("pre-execution rejection http %d errorCode %s exec %s",
				res.HTTPStatus, res.Code, res.ExecutionID),
			ExecutionID: res.ExecutionID,
		}
	case res.HTTPStatus == http.StatusTooManyRequests:
		return DispatchOutcome{Status: appconn.ActionUnknown, ProviderResult: d.noteThrottle(oc.Provider)}
	default:
		return DispatchOutcome{
			Status: appconn.ActionUnknown,
			ProviderResult: fmt.Sprintf("provider outcome unprovable http %d success %t",
				res.HTTPStatus, res.Success),
		}
	}
}

// noteThrottle records a provider 429 cooldown and returns the local note
// for the action row. T10-Q-5: the durable store reports a RETAINED update
// value (the stored instant is already at or after the new one) as
// repoappconn.ErrOCDispatchConflict — that sentinel is success-with-keep
// here, never a failure. A real recording failure is noted but never
// changes the outcome: a 429 settles as unknown either way.
func (d *OCDispatcher) noteThrottle(provider string) string {
	if d.retrySink == nil || provider == "" {
		return "rate limited; provider outcome unprovable"
	}
	now := d.now()
	err := d.retrySink.NoteOCProviderRetryAfter(context.Background(), provider, now.Add(ocThrottleCooldown), now)
	switch {
	case err == nil:
		return "rate limited; cooldown recorded"
	case errors.Is(err, repoappconn.ErrOCDispatchConflict):
		return "rate limited; cooldown retained (stored instant already at or after the observed one)"
	default:
		return "rate limited; cooldown record failed: " + err.Error()
	}
}
