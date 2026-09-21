package commercial

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"
)

// This file freezes the Commercial Platform seam (ADR-0012; the "Commercial
// authority and module seam" section of the Lago billing migration spec,
// ADR-0014). The interface exposes exactly three operation families —
// SubmitCommand, ReadSnapshot and Reconcile — and grows ONLY additively (new
// CommandKind/SnapshotKind constants, new typed payloads, new optional struct
// fields). Per-object shallow wrappers (GetCustomer/GetWallet/GetInvoice
// style) are forbidden by design: a new provider object is a new kind, never
// a new method. After ticket #77 lands, these signatures are FROZEN for the
// W3 tickets; changing one requires an ADR amendment, not a silent edit.
//
// The port is deliberately provider-neutral: no provider identifier, URL,
// HTTP type, or raw provider vocabulary may appear here — the callers see
// only WeKnora product vocabulary and closed token sets.

// CommandKind selects the typed commercial command family. T05 defines no
// kinds (both adapters fail closed); W3 adds constants additively (e.g.
// ensure_customer, publish_plan_version).
type CommandKind string

// CommandKindEnsureCustomer is the W3 additive first enabled command kind
// (#78, ADR-0014): idempotently ensure the tenant's customer exists on the
// authority under the deterministic identity. An ensure/upsert command —
// replay under the same Key legitimately refreshes advisory metadata and
// never changes identity.
const CommandKindEnsureCustomer CommandKind = "ensure_customer"

// EnsureCustomerPayload is the typed payload of ensure_customer. TenantID is
// the WeKnora space; ExternalCustomerID is the deterministic immutable
// identity (ExternalCustomerID(t) — a mismatched identity must be refused by
// the adapter, never silently forwarded); DisplayName is ADVISORY metadata
// (a rename updates it, never identity).
type EnsureCustomerPayload struct {
	TenantID           uint64
	ExternalCustomerID string
	DisplayName        string
}

// Command is one typed commercial command. Key is the idempotency identity:
// a replay of the same Key can never apply the command twice. Actor and
// Reason carry the audit trail; Payload receives its typed shape with the
// W3 kinds.
type Command struct {
	Kind    CommandKind
	Key     string
	Actor   string
	Reason  string
	Payload any
}

// Validate rejects empty Kind/Key; payload typing lands with W3 kinds.
func (c Command) Validate() error {
	if c.Kind == "" {
		return fmt.Errorf("invalid commercial command: kind is required")
	}
	if c.Key == "" {
		return fmt.Errorf("invalid commercial command: key (idempotency identity) is required")
	}
	return nil
}

// SnapshotKind selects the authoritative commercial snapshot family.
// Readiness (platform operational state) is the first and, in T05, the only
// kind; later kinds add sections to Snapshot additively.
type SnapshotKind string

// SnapshotKindReadiness reads the billing authority's readiness/version
// snapshot: platform operational state, not a commercial domain object.
const SnapshotKindReadiness SnapshotKind = "readiness"

// SnapshotKindAccount reads the billing authority's account truth for one
// tenant: whether the authority already holds this tenant's customer
// (linked) or definitively holds none (absent). W3 additive kind (#78,
// ADR-0014 additive-kind rule).
const SnapshotKindAccount SnapshotKind = "account"

// SnapshotQuery addresses one snapshot read. TenantID 0 means platform-wide
// (readiness is always platform-wide).
type SnapshotQuery struct {
	Kind     SnapshotKind
	TenantID uint64
}

// ReadinessState is the closed product enum of platform operational states —
// the only readiness tokens that may ever cross the Billing API.
type ReadinessState string

const (
	ReadinessReady       ReadinessState = "ready"
	ReadinessDegraded    ReadinessState = "degraded"
	ReadinessUnavailable ReadinessState = "unavailable"
)

// ReadinessSnapshot is the billing authority's operational state. Reason is
// a closed token (unconfigured|unreachable|invalid_response|unsupported)
// explaining a non-ready state, empty when ready. Release is
// deployment-pinned configuration (the operator-locked image identity),
// NEVER text parsed from a provider response.
type ReadinessSnapshot struct {
	State     ReadinessState
	Release   string
	CheckedAt time.Time
	Reason    string
}

// AccountState is the closed authority-truth enum for the account snapshot:
// the authority holds this tenant's customer (linked) or definitively holds
// none (absent). This is NOT the Billing API state — the API projects its
// own closed linked|pending envelope from this truth plus local recovery
// state (ADR-0014 additive-kind rule, #78).
type AccountState string

const (
	// AccountStateLinked: the authority holds this tenant's customer.
	AccountStateLinked AccountState = "linked"
	// AccountStateAbsent: the authority definitively holds none.
	AccountStateAbsent AccountState = "absent"
)

// AccountSnapshot is the authority-side account section of a Snapshot: the
// customer truth for one tenant at one check time.
type AccountSnapshot struct {
	TenantID  uint64
	State     AccountState
	CheckedAt time.Time
}

// Snapshot is one authoritative commercial snapshot read. Later kinds add
// sections additively; readiness remains the first section.
type Snapshot struct {
	Kind      SnapshotKind
	Readiness *ReadinessSnapshot
	// Account is the additive W3 account section (#78); nil unless the
	// snapshot kind is account. Readiness-only consumers are unaffected —
	// a zero-value Snapshot keeps Account nil.
	Account *AccountSnapshot
	// Benefits is the additive W4 benefits section (#80, Lago T08); nil
	// unless the snapshot kind is benefits. Earlier consumers are
	// unaffected — a zero-value Snapshot keeps Benefits nil.
	Benefits *BenefitsSnapshot
}

// ExternalCustomerID is THE deterministic WeKnora→authority customer
// identity: a pure function of the tenant ID only (no name, owner, or time),
// so a space rename (UpdateTenant) or owner transfer can never move it.
// Defined once here (ADR-0014 additive rule); the service (Key/payload),
// both adapters (REST addressing), and tests all call it — never a local
// copy. Format: "weknora-tenant-<decimal tenantID>" (the #73 probe-proven
// Lago external-id charset).
func ExternalCustomerID(tenantID uint64) string {
	return "weknora-tenant-" + strconv.FormatUint(tenantID, 10)
}

// ReconciliationCursor is an opaque durable token into a reconciliation
// stream; the adapter defines its interpretation and the caller only stores
// and replays it.
type ReconciliationCursor struct {
	Stream string
	Value  string
}

// CommercialChange is one authoritative change observed on a reconciliation
// stream. ExternalID is the provider correlation identity — it lives inside
// the seam and never crosses the Billing API.
type CommercialChange struct {
	Kind       string
	TenantID   uint64
	ExternalID string
	At         time.Time
}

// ReconciliationPage is one batch of changes plus the cursor to continue
// from. HasMore=false means the stream is drained for now.
type ReconciliationPage struct {
	Changes []CommercialChange
	Next    ReconciliationCursor
	HasMore bool
}

// CommandReceipt is the durable record of one accepted command. ExternalID
// is the provider correlation identity — seam-internal, never crossed into a
// Billing API response.
type CommandReceipt struct {
	Key        string
	ExternalID string
	RecordedAt time.Time
}

// Provider-neutral fail-closed sentinel errors. Adapters wrap them with
// short provider-neutral context; no provider URL, status text, or response
// body may ride along into a caller-visible error.
var (
	// ErrPlatformUnsupported: the command kind, snapshot kind, or
	// reconciliation stream is not enabled on this seam (T05 enables none of
	// the command/reconcile families).
	ErrPlatformUnsupported = errors.New("platform_operation_unsupported")
	// ErrPlatformUnconfigured: server-side configuration is missing, so the
	// adapter refuses to fabricate an answer.
	ErrPlatformUnconfigured = errors.New("platform_unconfigured")
	// ErrPlatformUnreachable: the authority could not be reached (transport,
	// timeout, non-2xx) — the outcome is unknown, never ready.
	ErrPlatformUnreachable = errors.New("platform_unreachable")
	// ErrPlatformInvalidResponse: the authority answered definitively wrong
	// (e.g. a 4xx), a wrong answer rather than a retryable miss.
	ErrPlatformInvalidResponse = errors.New("platform_invalid_response")
)

// CommercialPlatform is the one deep seam to the commercial billing
// authority (ADR-0012): typed commercial commands, authoritative commercial
// snapshots, and cursor-based reconciliation. No per-object wrapper methods
// exist or may be added — see the package comment.
type CommercialPlatform interface {
	// SubmitCommand applies one typed commercial command idempotently on
	// Command.Key and returns its durable receipt.
	SubmitCommand(ctx context.Context, cmd Command) (CommandReceipt, error)
	// ReadSnapshot reads one authoritative commercial snapshot addressed by
	// the query kind.
	ReadSnapshot(ctx context.Context, query SnapshotQuery) (Snapshot, error)
	// Reconcile streams authoritative changes from a durable cursor.
	Reconcile(ctx context.Context, from ReconciliationCursor) (ReconciliationPage, error)
}
