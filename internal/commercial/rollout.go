package commercial

import (
	"errors"
	"strings"
	"time"
)

// Rollout-pause vocabulary (O01). AllowDuringRollback is the single
// authority deciding which commercial operations keep running while new
// consumption is switched off during a rollout pause: the four
// recovery-safe operations (payment callbacks, refund queries,
// fulfillment replays, settlement) continue so paid work is never lost,
// and every other operation — anything that accepts NEW money or
// dispatches NEW work — is refused until the pause is lifted.
const (
	OperationPaymentCallback   = "payment_callback"
	OperationRefundQuery       = "refund_query"
	OperationFulfillmentReplay = "fulfillment_replay"
	OperationSettlement        = "settlement"
)

// AllowDuringRollback reports whether an operation is safe to keep
// processing while commercial rollout is paused. Only query/recovery
// operations on ALREADY-PAID money pass; new consumption fails closed.
func AllowDuringRollback(operation string) bool {
	switch operation {
	case "payment_callback", "refund_query", "fulfillment_replay", "settlement":
		return true
	default:
		return false
	}
}

// RecoveryCategory classifies one recovery-queue entry. The queue is
// presented to operators grouped by these categories; each maps to a
// documented manual runbook path (docs/operations/saas-commercial-recovery.md).
type RecoveryCategory string

const (
	// RecoveryCategoryPaidUnfulfilled: an order was paid but its benefit
	// never landed (payment callback or fulfillment replay needed).
	RecoveryCategoryPaidUnfulfilled RecoveryCategory = "paid_unfulfilled"
	// RecoveryCategoryRefundUnknown: a channel refund whose outcome is
	// indeterminate — query the ORIGINAL refund key, never re-create.
	RecoveryCategoryRefundUnknown RecoveryCategory = "refund_unknown"
	// RecoveryCategoryRevocationPending: channel payout succeeded but the
	// precise-credits revocation is still pending — retry revocation only.
	RecoveryCategoryRevocationPending RecoveryCategory = "revocation_pending"
	// RecoveryCategoryUsageUnconfirmed: dispatched usage whose settlement
	// is not yet confirmed — query the provider before releasing anything.
	RecoveryCategoryUsageUnconfirmed RecoveryCategory = "usage_unconfirmed"
	// RecoveryCategoryBalanceDiscrepancy: recorded balance disagrees with
	// the provider/ledger view — reconcile from source records only.
	RecoveryCategoryBalanceDiscrepancy RecoveryCategory = "balance_discrepancy"
	// RecoveryCategoryUnknownAction: an operation kind this build does not
	// understand — verification only, NEVER re-dispatch.
	RecoveryCategoryUnknownAction RecoveryCategory = "unknown_action"
)

// RecoveryCategories is the closed category set, in display order.
var RecoveryCategories = []RecoveryCategory{
	RecoveryCategoryPaidUnfulfilled,
	RecoveryCategoryRefundUnknown,
	RecoveryCategoryRevocationPending,
	RecoveryCategoryUsageUnconfirmed,
	RecoveryCategoryBalanceDiscrepancy,
	RecoveryCategoryUnknownAction,
}

// ParseRecoveryCategory validates a category string (config/API input)
// back onto the closed set.
func ParseRecoveryCategory(s string) (RecoveryCategory, bool) {
	for _, c := range RecoveryCategories {
		if string(c) == s {
			return c, true
		}
	}
	return "", false
}

// ClassifyRecovery maps an operation kind plus its current object state
// onto one queue category. Unknown kinds or states land in
// unknown_action so the queue can never silently drop an entry — an
// unrecognized WRITE is verification-only by construction.
func ClassifyRecovery(kind, state string) RecoveryCategory {
	switch kind {
	case OperationPaymentCallback, OperationFulfillmentReplay:
		return RecoveryCategoryPaidUnfulfilled
	case OperationRefundQuery:
		if state == string(RecoveryCategoryRevocationPending) {
			return RecoveryCategoryRevocationPending
		}
		return RecoveryCategoryRefundUnknown
	case OperationSettlement:
		if strings.Contains(state, "discrepan") || state == "dead" {
			return RecoveryCategoryBalanceDiscrepancy
		}
		return RecoveryCategoryUsageUnconfirmed
	default:
		return RecoveryCategoryUnknownAction
	}
}

// CommercialSwitches mirrors the three config.yaml rollout switches
// (commercial_new_orders / commercial_new_dispatch /
// connector_new_actions). Default is ALL OPEN (safe-on): money keeps
// moving unless an operator explicitly closes a lane as a rollback
// action. Closing a switch never blocks recovery of already-paid work.
type CommercialSwitches struct {
	NewOrders           bool
	NewDispatch         bool
	ConnectorNewActions bool
}

// DefaultCommercialSwitches is the safe-on default: everything enabled.
func DefaultCommercialSwitches() CommercialSwitches {
	return CommercialSwitches{NewOrders: true, NewDispatch: true, ConnectorNewActions: true}
}

// AllowsNewConsumption reports whether a NEW consumption lane is open.
// Only the three new-consumption operations consult a switch; recovery
// operations (callbacks, queries, settlement) are always allowed and
// unknown operations are governed by AllowDuringRollback instead.
func (s CommercialSwitches) AllowsNewConsumption(operation string) bool {
	switch operation {
	case "new_order":
		return s.NewOrders
	case "new_dispatch":
		return s.NewDispatch
	case "connector_action":
		return s.ConnectorNewActions
	default:
		return true
	}
}

// RecoveryCategoryMetrics is one pure per-category snapshot: queue count
// plus age percentiles. Callers compute ages from queue entries; this
// structure only carries the numbers so alerting stays side-effect free.
type RecoveryCategoryMetrics struct {
	Category RecoveryCategory
	Count    int
	AgeP50   time.Duration
	AgeP90   time.Duration
	AgeP99   time.Duration
	MaxAge   time.Duration
}

// RecoveryAlertThresholds configures when one category's metrics breach
// alerting. At least one positive bound must be set — an all-zero config
// would never alert, which is exactly the misconfiguration to reject.
type RecoveryAlertThresholds struct {
	MaxCount  int
	MaxAgeP90 time.Duration
	MaxAge    time.Duration
}

// ErrInvalidAlertThresholds rejects a threshold config that could never
// alert or carries negative bounds.
var ErrInvalidAlertThresholds = errors.New("invalid_recovery_alert_thresholds")

// Validate checks the threshold configuration is usable.
func (t RecoveryAlertThresholds) Validate() error {
	if t.MaxCount < 0 || t.MaxAgeP90 < 0 || t.MaxAge < 0 {
		return ErrInvalidAlertThresholds
	}
	if t.MaxCount == 0 && t.MaxAgeP90 == 0 && t.MaxAge == 0 {
		return ErrInvalidAlertThresholds
	}
	return nil
}

// Breached reports whether a category's metrics cross any configured
// bound. Metrics with no configured bound never breach on that axis.
func (t RecoveryAlertThresholds) Breached(m RecoveryCategoryMetrics) bool {
	if t.MaxCount > 0 && m.Count >= t.MaxCount {
		return true
	}
	if t.MaxAgeP90 > 0 && m.AgeP90 >= t.MaxAgeP90 {
		return true
	}
	if t.MaxAge > 0 && m.MaxAge >= t.MaxAge {
		return true
	}
	return false
}

// credentialKeyFragments marks map keys whose values must never reach a
// recovery log line. Matched case-insensitively as substrings so
// access_token / api_key / private_key / authorization all hit.
var credentialKeyFragments = []string{
	"token", "secret", "password", "passwd", "credential", "authorization",
	"api_key", "apikey", "private", "signature", "cookie", "session",
}

// RedactedValue replaces every masked value.
const RedactedValue = "[REDACTED]"

// maxLogValueLen caps any single logged value; longer payloads are
// truncated with an explicit marker so dumps cannot flood recovery logs.
const maxLogValueLen = 512

func looksCredentialLike(key string) bool {
	k := strings.ToLower(key)
	for _, f := range credentialKeyFragments {
		if strings.Contains(k, f) {
			return true
		}
	}
	return false
}

// TruncateLogValue caps a string for logging.
func TruncateLogValue(s string) string {
	if len(s) <= maxLogValueLen {
		return s
	}
	return s[:maxLogValueLen] + "…[truncated]"
}

// RedactForLog makes one value safe for a recovery log line: maps are
// walked (nested maps included), credential-like keys are replaced with
// [REDACTED], long strings are truncated, and scalars pass through.
func RedactForLog(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			if looksCredentialLike(k) {
				out[k] = RedactedValue
				continue
			}
			out[k] = RedactForLog(val)
		}
		return out
	case string:
		return TruncateLogValue(t)
	default:
		return v
	}
}
