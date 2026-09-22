package appconnector

import (
	"context"
	"encoding/json"
	"time"
)

// Open-connector binding lifecycle states. A binding starts pending while the
// external authorization attempt is in flight, becomes active once the runtime
// confirms the external connection, and ends revoked (owner request) or error
// (runtime rejected the binding). Terminal states keep their rows forever:
// the external id and alias of a revoked binding are never handed out again.
const (
	OCBindingPending = "pending"
	OCBindingActive  = "active"
	OCBindingRevoked = "revoked"
	OCBindingError   = "error"
)

// OCBinding is the tenant-scoped wiring between one local connection and one
// external connection on a shared open-connector runtime. AuthVersion mirrors
// the local connection's authorization counter; BindingVersion is the row's
// optimistic-concurrency counter and only ever moves forward.
type OCBinding struct {
	TenantID                                             uint64
	ConnectionID, RuntimeID, Provider, ExternalID, Alias string
	AuthVersion, BindingVersion                          int64
	State                                                string
}

// OCDefinition is a reviewed, published action definition pinned to one app
// version. InputSchema is the frozen upstream JSON schema the review approved;
// SchemaDigest lets executors reject any drift without re-parsing it.
type OCDefinition struct {
	AppID, AppVersion, ActionID, Provider, SchemaDigest, Risk string
	InputSchema                                               json.RawMessage
	RequiredScopes                                            []string
	Published                                                 bool
}

// OCExecutionBinding is the immutable execution snapshot a dispatcher carries:
// which runtime and external connection to target, under which alias, and the
// binding generation the approval was granted against.
type OCExecutionBinding struct {
	RuntimeID, Provider, ExternalID, Alias, ActionID, SchemaDigest string
	BindingVersion                                                 int64
}

// OCSubject identifies the authenticated caller behind an authorization or
// execution request. It is only constructed by authenticated handlers or
// controlled background jobs — model parameters can never supply it.
type OCSubject struct {
	TenantID uint64
	ActorID  string
}

// OCAuthorizationAttempt records one in-flight external authorization flow:
// which tenant/actor/connection started it, the alias reserved for it, and
// when the attempt expires. Attempts are single-use correlation records, not
// grants.
type OCAuthorizationAttempt struct {
	ID                                              string
	Subject                                         OCSubject
	ConnectionID, RuntimeID, Provider, Alias, State string
	AuthVersion                                     int64
	ExpiresAt                                       time.Time
}

// OCDispatchRecord is the local linearization point of one approved external
// dispatch: the idempotency key, the runtime it was sent to, the external
// execution id once known, and the fence counter protecting replay decisions.
type OCDispatchRecord struct {
	TenantID                                                    uint64
	ActionID, RuntimeID, Key, ExecutionID, State, ReservationID string
	FirstSentAt, ReplayUntil                                    time.Time
	Fence                                                       int64
}

// OCBindingStore is the minimal tenant-scoped read contract later services
// consume. Lookups are always keyed by tenant; no method accepts an unscoped
// identifier.
type OCBindingStore interface {
	GetBinding(context.Context, uint64, string) (OCBinding, error)
}

// OCAuthorizer decouples authorization checks from raw credentials: it
// validates the subject against the local connection and binding (including
// the expected AuthVersion) and returns the binding that authorized the call.
// Implementations must never load credential material.
type OCAuthorizer interface {
	Check(context.Context, OCSubject, string, int64) (OCBinding, error)
}
