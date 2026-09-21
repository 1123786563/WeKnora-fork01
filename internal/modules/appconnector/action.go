package appconnector

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"
)

// Action risk categories. "read" is safe, "write" may be covered by a
// bounded general write grant, everything else (send, delete, ...) always
// needs an explicit approval — a write grant never satisfies them.
const (
	RiskRead   = "read"
	RiskWrite  = "write"
	RiskSend   = "send"
	RiskDelete = "delete"
)

// Action lifecycle states. Prepare parks a new action in awaiting_approval
// (authorized only via a matching scope pre-authorization); Approve moves it
// to authorized; Execute consumes the approval and writes the dispatch
// intent in one transaction (queued→dispatched inside it), and the provider
// outcome settles it into succeeded/failed/unknown. unknown is NEVER
// re-queued into the normal path — it resolves only via a provider query.
const (
	ActionAwaitingApproval = "awaiting_approval"
	ActionAuthorized       = "authorized"
	ActionQueued           = "queued"
	ActionDispatched       = "dispatched"
	ActionSucceeded        = "succeeded"
	ActionFailed           = "failed"
	ActionUnknown          = "unknown"
)

// Action is one pending connector call whose exact bytes are bound to an
// approval digest before anything is dispatched. AuthVersion is the
// connection's permission version at preparation time; a version bump
// between approval and execute invalidates the dispatch.
// OC carries the open-connector execution binding for OC actions. It is
// filled ONLY server-side (PrepareOC, from the tenant's live binding and the
// reviewed definition) — a client can never supply runtime/alias/external
// identity through this field, and the native Prepare path rejects an Action
// that arrives with it set. DigestVersion records which digest generation
// the row's stored digest was computed under (1 = legacy text layout,
// CurrentDigestVersion = structured JSON material); old v1 rows in
// preparable states must be re-Prepared instead of silently continuing.
type Action struct {
	ID            string
	TenantID      uint64
	ActorID       string
	ConnectionID  string
	Version       string
	Target        string
	Risk          string
	Args          json.RawMessage
	AuthVersion   int64
	OC            *OCExecutionBinding
	DigestVersion int
}

// CurrentDigestVersion is the digest generation this code writes: the
// structured JSON approval material of ActionDigest. Rows written before it
// carry 1 (set by migration 000122/000042) and must re-Prepare.
const CurrentDigestVersion = 2

// NeedsExplicitApproval reports whether a call of the given risk needs an
// explicit per-action human approval even when the actor holds a bounded
// general write grant. Reads never do; writes are covered by the grant;
// every other category (send, delete, ...) is always explicit — a general
// write grant must never authorize spending or deletion side effects.
func NeedsExplicitApproval(risk string, generalWriteGrant bool) bool {
	switch risk {
	case RiskRead:
		return false
	case RiskWrite:
		return !generalWriteGrant
	default:
		return true
	}
}

// ErrInvalidArgs rejects arguments that are not valid JSON.
var ErrInvalidArgs = errors.New("invalid_action_args")

// NormalizeArgs canonicalizes raw JSON arguments: a json.Decoder with
// UseNumber preserves exact number literals, then Marshal re-encodes
// deterministically (object keys sorted, stable escaping). The returned
// bytes are THE snapshot: the same normalized bytes are persisted with the
// action, covered by the digest, and handed to the provider — approving one
// payload and sending another ("approve-then-rewrite") is structurally
// impossible because the dispatch path only ever sees these bytes.
func NormalizeArgs(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 {
		return []byte("null"), nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v interface{}
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidArgs, err)
	}
	// Exactly ONE JSON value: the second Decode must report io.EOF. A
	// concatenated second document (or trailing garbage) is rejected — a
	// payload like {"a":1}{"b":2} must never normalize to its first half
	// and silently drop the rest. json.Decoder with UseNumber keeps number
	// literals byte-exact (no float64 re-encoding), so precision survives.
	if err := dec.Decode(&v); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%w: trailing data after the first JSON value", ErrInvalidArgs)
	}
	out, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidArgs, err)
	}
	return out, nil
}

// ActionDigest hashes every input an approval is bound to, as ONE
// structured JSON document (never newline-bearing text concatenation, which
// ambiguous fields could collide with): the digest layout version, the
// logical action identity (ID), the tenant and actor identity, the
// connection and its auth version, the reviewed risk, the target, the app
// version, the FULL open-connector execution binding when present, and the
// NORMALIZED argument bytes. Changing any one of them changes the digest,
// so an approval for one exact call can never authorize another identity,
// another action, another risk, another permission version, another target,
// another binding generation, or other arguments. Material layout is
// generation 2 for every digest this code computes; generation-1 rows are
// legacy and must re-Prepare (see CurrentDigestVersion).
func ActionDigest(a Action) (string, error) {
	norm, err := NormalizeArgs(a.Args)
	if err != nil {
		return "", err
	}
	material := struct {
		Version                  int
		ID                       string
		Tenant                   uint64
		Actor, Connection        string
		AuthVersion              int64
		AppVersion, Target, Risk string
		OC                       *OCExecutionBinding
		Args                     json.RawMessage
	}{CurrentDigestVersion, a.ID, a.TenantID, a.ActorID, a.ConnectionID, a.AuthVersion,
		a.Version, a.Target, a.Risk, a.OC, norm}
	b, err := json.Marshal(material)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

// PreAuthorization is a persisted scope pre-authorization: it pre-approves
// a category of actions (e.g. "send") on one connection / target scope for
// a validity window under a budget cap. A send pre-authorization is its OWN
// scope — it is matched only through its allowed risk categories, never via
// a general write grant, and a write pre-authorization never covers send.
type PreAuthorization struct {
	ID             string
	TenantID       uint64
	AllowedRisks   []string
	ConnectionID   string // exact connection id, or "*"
	TargetScope    string // exact target, or "*"
	ValidFrom      time.Time
	ValidUntil     time.Time
	BudgetCapMicro int64
}

// PreAuthorizationCovers reports whether p pre-authorizes exactly a at time
// now. The risk category must be explicitly listed in p.AllowedRisks (own
// scope matching), the connection and target must match (or be wildcarded),
// now must be strictly inside the validity window, and the budget cap must
// not be negative (cap enforcement happens at the budget gate on execute).
func PreAuthorizationCovers(p PreAuthorization, a Action, now time.Time) bool {
	if p.ID == "" || a.TenantID == 0 || p.TenantID != a.TenantID {
		return false
	}
	if !now.Before(p.ValidUntil) || now.Before(p.ValidFrom) {
		return false
	}
	if p.ConnectionID != "*" && p.ConnectionID != a.ConnectionID {
		return false
	}
	if p.TargetScope != "*" && p.TargetScope != a.Target {
		return false
	}
	if p.BudgetCapMicro < 0 {
		return false
	}
	for _, r := range p.AllowedRisks {
		if r == a.Risk {
			return true
		}
	}
	return false
}
