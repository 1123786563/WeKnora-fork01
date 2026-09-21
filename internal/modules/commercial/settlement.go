package commercial

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

// Settlement lifecycle states. A settlement is born dispatched when Finalize
// commits its outbox event; unknown marks an indeterminate remote outcome
// (timeout after the remote may have persisted) retained for reconciliation;
// sent means the request was handed to the provider; accepted means the
// provider returned an explicit transaction identity — external ingest
// acceptance is NOT consumption confirmation; only the V03-selected explicit
// confirmation strategy produces confirmed, and history is never deleted.
const (
	SettlementStateDispatched = "dispatched"
	SettlementStateUnknown    = "unknown"
	SettlementStateSent       = "sent"
	SettlementStateAccepted   = "accepted"
	SettlementStateConfirmed  = "confirmed"
)

// ReservationStateSettled marks a reservation whose hold Finalize already
// converted (consumed part to unreflected, unused part released). U02's
// Reserve only replays identical holds in state held, so a settled key can
// never be double-held.
const ReservationStateSettled = "settled"

// ErrInvalidSettlement rejects settlements without identity, tenant,
// reservation, a non-negative amount, a positive revision, or an occurrence
// time — the gateway contract Finalize must satisfy before dispatch.
var ErrInvalidSettlement = errors.New("invalid_settlement")

// KeepProtection reports whether a settlement in the given state still
// protects its spend against the local budget projection. Every state except
// an explicitly confirmed one keeps the protection: dispatched, unknown,
// sent, and accepted (external ingest acceptance) all retain it, and so does
// any state this vocabulary does not recognize — an unrecognized answer is
// never a licence to release spend.
func KeepProtection(state string) bool { return state != SettlementStateConfirmed }

// Settlement is one call's final consumption handed to the commercial
// provider. ID is the settlement idempotency key derived from the usage
// revision identity (SettlementKey), so replays, lost responses, and
// external corrections/negative reversals reuse exactly one remote
// transaction per revision and can never double-settle.
type Settlement struct {
	ID            string
	CallID        string
	ReservationID string
	TenantID      uint64
	Amount        Credits
	Revision      int64
	OccurredAt    time.Time
}

// Validate enforces the gateway contract: full identity, a real tenant, a
// non-negative amount, a positive revision, and a real occurrence time.
func (s Settlement) Validate() error {
	if s.ID == "" || s.CallID == "" || s.ReservationID == "" || s.TenantID == 0 ||
		s.Amount < 0 || s.Revision <= 0 || s.OccurredAt.IsZero() {
		return ErrInvalidSettlement
	}
	return nil
}

// SettlementReceipt is the external proof of one settlement. ExternalID is
// the provider transaction identity (event/transaction correlation);
// Watermark is the provider-side consumption watermark the confirmation
// advanced. An empty field proves nothing.
type SettlementReceipt struct {
	ExternalID string
	Watermark  string
}

// ConfirmedEvidence reports whether the receipt carries real confirmation
// evidence: BOTH the transaction correlation id and the watermark it
// advanced. Seeing a balance drop is never evidence — "my event is counted"
// can only be proven by correlation, never inferred.
func (r SettlementReceipt) ConfirmedEvidence() bool {
	return r.ExternalID != "" && r.Watermark != ""
}

// SettlementKey derives the stable, collision-free idempotency identity of
// one usage revision's settlement. The NUL separators keep the components
// unambiguous, the hash hides internals, and the settle: prefix keeps the
// key namespace separate from FulfillmentKey's. Corrections and negative
// reversals arrive as their own revision and therefore their own key, while
// any retry of the same revision reuses this exact key.
func SettlementKey(tenantID uint64, callID, attemptID string, revision int64) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%d\x00%s\x00%s\x00%d", tenantID, callID, attemptID, revision)))
	return "settle:" + hex.EncodeToString(sum[:])
}
