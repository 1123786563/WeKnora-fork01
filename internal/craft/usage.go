package craft

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
)

// Runtime identities of the two physical model-calling planes of a Craft
// execution. The main agent runtime and the OpenCode child runtime each make
// their own real physical model calls; each attempt records its own usage
// fact under its own runtime so a parent rollup can never be billed as if
// the child's tokens were the parent's.
const (
	// RuntimeMain is the primary agent runtime of the owning run.
	RuntimeMain = "main"
	// RuntimeOC is the OpenCode sub-execution runtime of a delegation.
	RuntimeOC = "oc"
)

// Usage observation statuses. reported is a provider-observed usage block;
// unknown marks an attempt whose usage could NOT be observed (stream break,
// cancellation) and never fabricates zero token counts; corrected marks a
// later observation superseding a previous revision of the SAME physical
// attempt (late usage, post-cancel arrival) — a correction is a new
// revision, never an in-place overwrite of the recorded history.
const (
	UsageStatusReported  = "reported"
	UsageStatusUnknown   = "unknown"
	UsageStatusCorrected = "corrected"
)

// UsageKey is the dedup identity of ONE physical model attempt per tenant:
// the same (tenant, call, attempt) redelivered — an at-least-once usage
// observation, a worker replay, an OC summary replay — keys identically and
// is counted once, while a real retry of the same logical call carries a new
// attempt id and therefore a new key. A second tenant reusing the same call
// id never collides.
func UsageKey(tenant uint64, callID, attemptID string) string {
	raw, _ := json.Marshal([]string{strconv.FormatUint(tenant, 10), callID, attemptID})
	hash := sha256.Sum256(raw)
	return hex.EncodeToString(hash[:])
}

// DeriveCallID derives the logical call identity the server-side model
// gateway assigns BEFORE forwarding one outbound call. The identity binds
// the server-resolved tenant, run, delegation, model and funding source, so
// a different funding binding (platform vs BYOK) or model can never adopt
// another binding's ledger row; callSeq distinguishes logical calls of the
// same binding inside one run.
func DeriveCallID(tenant uint64, runID, delegationID, modelID, funding string, callSeq int64) string {
	raw, _ := json.Marshal([]string{
		strconv.FormatUint(tenant, 10), runID, delegationID, modelID, funding,
		strconv.FormatInt(callSeq, 10),
	})
	hash := sha256.Sum256(raw)
	return "call_" + hex.EncodeToString(hash[:16])
}

// DeriveAttemptID derives one physical attempt identity of a logical call.
// A real retry of the call forwards again with attemptSeq+1 and therefore
// records as a NEW physical fact instead of overwriting the failed attempt.
func DeriveAttemptID(callID string, attemptSeq int64) string {
	raw, _ := json.Marshal([]string{callID, strconv.FormatInt(attemptSeq, 10)})
	hash := sha256.Sum256(raw)
	return "att_" + hex.EncodeToString(hash[:16])
}

// UsageFact is the raw observation of ONE physical model attempt: which
// runtime made it (main agent or OC child), under which run and delegation,
// with which server-bound model and funding source, and the token counts as
// reported by the provider. Funding is ALWAYS the server-side credential
// binding's verdict (commercial.FundingPlatform / commercial.FundingBYOK
// vocabulary); a fact never trusts tenant or cost from model arguments.
//
// Token semantics follow the fixed provider contract: Input and Output are
// the provider-reported prompt and completion tokens and Cached is the
// cache-hit portion OF Input — a subset, never an additive line, so mapping
// either the OpenAI-style (prompt_tokens_details.cached_tokens) or the
// Anthropic-style (cache_read_input_tokens) contract lands on the same
// shape without double counting. An unknown fact carries no numbers at all:
// it stays its own line item instead of being recorded as zero usage.
type UsageFact struct {
	ID           string
	RunID        string
	DelegationID string
	CallID       string
	AttemptID    string
	Runtime      string
	ModelID      string
	Funding      string
	Status       string
	TenantID     uint64
	Input        int64
	Output       int64
	Cached       int64
}

// Validate enforces the fact invariants: complete identity, a known runtime
// plane (an OC child attempt always names its delegation), a known status,
// non-negative counts, the cached-within-input provider contract, and the
// no-fabricated-numbers rule for unknown facts.
func (f UsageFact) Validate() error {
	if f.TenantID == 0 || f.CallID == "" || f.AttemptID == "" ||
		f.ModelID == "" || f.Funding == "" || f.Status == "" {
		return fmt.Errorf("%w: incomplete usage fact identity", ErrInvalidInput)
	}
	switch f.Runtime {
	case RuntimeMain, RuntimeOC:
	default:
		return fmt.Errorf("%w: usage runtime %q", ErrInvalidInput, f.Runtime)
	}
	if f.Runtime == RuntimeOC && f.DelegationID == "" {
		return fmt.Errorf("%w: oc usage attempt without delegation", ErrInvalidInput)
	}
	switch f.Status {
	case UsageStatusReported, UsageStatusUnknown, UsageStatusCorrected:
	default:
		return fmt.Errorf("%w: usage status %q", ErrInvalidInput, f.Status)
	}
	if f.Input < 0 || f.Output < 0 || f.Cached < 0 {
		return fmt.Errorf("%w: negative usage counts", ErrInvalidInput)
	}
	if f.Cached > f.Input {
		return fmt.Errorf("%w: cached %d exceeds input %d (cached is a subset of input)", ErrInvalidInput, f.Cached, f.Input)
	}
	if f.Status == UsageStatusUnknown && (f.Input != 0 || f.Output != 0 || f.Cached != 0) {
		return fmt.Errorf("%w: unknown usage fact must not carry token counts", ErrInvalidInput)
	}
	return nil
}

// ResolvedID returns the fact's ledger identity: an empty ID is derived from
// the fact's own (tenant, call, attempt); a present ID must equal that key,
// so a spoofed ID can never alias another tenant's or another attempt's row.
func (f UsageFact) ResolvedID() (string, error) {
	if err := f.Validate(); err != nil {
		return "", err
	}
	key := UsageKey(f.TenantID, f.CallID, f.AttemptID)
	if f.ID != "" && f.ID != key {
		return "", fmt.Errorf("%w: usage fact id %q does not match its identity", ErrInvalidInput, f.ID)
	}
	return key, nil
}

// UsageTotals aggregates observed usage across physical facts. Unknown facts
// are never summed — they are counted on their own line so unobserved
// consumption can be reconciled instead of being guessed as zero.
type UsageTotals struct {
	Input   int64
	Output  int64
	Cached  int64
	Facts   int
	Unknown int
}

// Add folds one fact into the totals. Reported and corrected facts
// contribute their tokens; unknown facts only bump the unknown counter.
func (t *UsageTotals) Add(f UsageFact) {
	if f.Status == UsageStatusUnknown {
		t.Unknown++
		return
	}
	t.Input += f.Input
	t.Output += f.Output
	t.Cached += f.Cached
	t.Facts++
}

// UsageSink receives raw observations of physical model attempts at the
// persistence boundary of the usage ledger. The durable implementation is
// the craft usage repository; the delegation/usage service is the only
// writer. Delivery of the facts to the official OpenMeter pipeline is NOT
// this sink's job: the outbox rows it writes carry stable event identities
// so the G4 gateway adapter can map and redeliver them.
type UsageSink interface {
	// Append records the first observation of one physical attempt keyed by
	// its UsageKey. Appending the same fact again (same ID, same content) is
	// an idempotent no-op — redelivery is never counted twice. Appending a
	// DIFFERENT fact under the same ID fails with ErrConflict: a changed
	// observation must travel through Correct as an explicit revision and
	// can never overwrite the recorded history.
	Append(ctx context.Context, f UsageFact) error
	// Correct files a later observation of an already-recorded physical
	// attempt as a new revision with status corrected. It requires the
	// attempt to exist, keeps every earlier revision readable (the
	// revision trail IS the reconciliation record for stream breaks and
	// post-cancellation late arrivals), and never creates a second physical
	// fact for the same attempt.
	Correct(ctx context.Context, f UsageFact) error
}

// ErrUsageNotRecorded reports a correction for an attempt that was never
// appended. A late observation for an unknown attempt is a correction; a
// late observation for an attempt nobody recorded is a caller bug.
var ErrUsageNotRecorded = errors.New("craft usage attempt not recorded")
