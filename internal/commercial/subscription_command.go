package commercial

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// This file grows the frozen Commercial Platform seam (internal/commercial/
// platform.go) ADDITIVELY per ADR-0014 and the T05 Task 2 freeze — the exact
// plan_command.go precedent: two new CommandKind constants
// (ensure_subscription, grant_included_credits) plus their typed
// provider-neutral payloads and one new SnapshotKind (benefits) live here.
// The frozen file itself gains ONLY the additive Snapshot.Benefits field (the
// #78 Snapshot.Account precedent); the three method signatures never change
// and no per-object wrapper methods exist anywhere.
//
// Spec: docs/specs/2026-09-20-lago-billing-migration-design.md —
// "Catalog, subscriptions, and entitlements" (subscription continuity on ONE
// external identity across upgrade/downgrade/Base-Plan transition) and
// "Credits and Task admission" (included credits expire at monthly period
// end, no rollover). Binding lab verdicts: docs/migrations/lago/
// t03-wallet-semantics/verdict.md (a1 monthly issuance is
// PASS-WITH-COORDINATION with three coordinator obligations; E3 — Lago
// wallet/wallet-transaction APIs carry NO external idempotency, a replayed
// grant POST doubles the balance) and t02-payment-activation/DECISION.md (a
// free Base Plan must be a STANDARD subscription with no activation_rules —
// payment gating needs a supported provider, Community has none wired).

// CommandKindEnsureSubscription idempotently places the tenant on a plan
// version as a STANDARD subscription under the deterministic subscription
// identity (#80, Lago T08). No activation_rules ever ride the command — the
// Base Plan must not gate on payment (T02 decision).
const CommandKindEnsureSubscription CommandKind = "ensure_subscription"

// BasePlanKey is the built-in free tier's plan key — the bottom rung of the
// tier ladder (base@0, added in T08) that every new space enters through the
// ensure chain.
const BasePlanKey = "base"

// ExternalSubscriptionID is THE deterministic WeKnora→authority subscription
// identity (subscription continuity: one identity per tenant across upgrade,
// downgrade and Base-Plan transition — spec "Catalog, subscriptions, and
// entitlements"). A pure function of the tenant ID only; defined once here
// so the service, both adapters and tests share it, never a local copy.
func ExternalSubscriptionID(tenantID uint64) string {
	return ExternalCustomerID(tenantID) + "-sub"
}

// EnsureSubscriptionPayload is the typed payload of ensure_subscription.
// Both external identities are DERIVED — the adapter refuses any mismatch
// with the derivations before a request leaves the seam, so no input can
// address another tenant's objects. PlanCode is the Base Plan version's
// deterministic external code (seam-internal).
type EnsureSubscriptionPayload struct {
	TenantID               uint64
	ExternalCustomerID     string // must equal ExternalCustomerID(TenantID)
	ExternalSubscriptionID string // must equal ExternalSubscriptionID(TenantID)
	PlanCode               string // the plan version's deterministic code (seam-internal)
}

// Validate enforces the derived-identity equality and a plan code to attach.
func (p EnsureSubscriptionPayload) Validate() error {
	if p.TenantID == 0 {
		return errors.New("invalid ensure_subscription payload: tenant is required")
	}
	if p.ExternalCustomerID != ExternalCustomerID(p.TenantID) {
		return errors.New("invalid ensure_subscription payload: external_customer_id must equal ExternalCustomerID(tenant)")
	}
	if p.ExternalSubscriptionID != ExternalSubscriptionID(p.TenantID) {
		return errors.New("invalid ensure_subscription payload: external_subscription_id must equal ExternalSubscriptionID(tenant)")
	}
	if p.PlanCode == "" {
		return errors.New("invalid ensure_subscription payload: plan_code is required")
	}
	return nil
}

// EnsureSubscriptionCommandKey derives the seam command idempotency identity
// "ensure_subscription:<ext-sub-id>" — the coordinator (not the provider)
// owns subscription identity.
func EnsureSubscriptionCommandKey(externalSubscriptionID string) string {
	return string(CommandKindEnsureSubscription) + ":" + externalSubscriptionID
}

// CommandKindGrantIncludedCredits issues one month's included credits as a
// short-TTL wallet (coordinator-owned monthly issuance — the T03 verdict a1
// design: Lago Community has NO native recurring wallet issuance, interval
// rules are Premium-gated and 500).
const CommandKindGrantIncludedCredits CommandKind = "grant_included_credits"

// MonthlyWalletPriority is the wallet priority class of monthly included
// credits: 1 = earliest-expiry class consumed first (T03 verdict fact b:
// consumption order is `priority ASC, created_at ASC`).
const MonthlyWalletPriority = 1

// TopUpWalletPriority is the single creation-time priority class of top-up
// batches (#86 Task 2). Under the uniform twelve-month TTL the family's
// internal expiry order ≡ grant order, so Lago's same-priority
// `created_at ASC` tie-break already IS the expiry order — one class
// suffices at creation; the authority rebalance (WalletRank) is the
// invariant's real guarantee.
const TopUpWalletPriority = 2

// MaxWalletPriority is the UPPER bound of the wallet priority domain
// [1, MaxWalletPriority] — the same field GrantIncludedCreditsPayload.Validate
// enforces at grant time (OCR84-R1-09: one shared constant, never two
// copies of the domain). WalletRank answers ranks 1..n over the ACTIVE
// batches; n beyond the bound means the rebalance would WRITE a priority
// outside the documented domain, so the rank answer fails closed instead.
const MaxWalletPriority = 50

// MonthlyWalletPriorityFor computes a monthly wallet's CREATION-TIME
// priority: MonthlyWalletPriority (1) unless some top-up batch expires
// strictly before this period's end ("aging" — it must be consumed before
// the monthly batch), in which case the monthly batch yields to
// TopUpWalletPriority+1 (3). This initial value only reduces rebalance
// writes; correctness is the refresh-time authority rebalance — a static
// encoding cannot express mixed aging+fresh coexistence (the r1-review
// counterexample: rebalance converges A=1, M=2, B=3).
func MonthlyWalletPriorityFor(topUpExpiries []time.Time, periodEnd time.Time) int {
	for _, exp := range topUpExpiries {
		if exp.After(time.Time{}) && exp.Before(periodEnd) {
			return TopUpWalletPriority + 1
		}
	}
	return MonthlyWalletPriority
}

// WalletRankInput is one active batch participating in the authority
// consumption-order ranking (seam-internal shape).
type WalletRankInput struct {
	WalletRef string // the wallet's deterministic name (adapters map to lago_id)
	ExpiresAt time.Time
	GrantedAt time.Time // the authority's created_at (grant time)
}

// WalletRank answers the correct ranks 1..n over (ExpiresAt ASC, GrantedAt
// ASC, WalletRef ASC) — the "earliest expiry, then earliest grant" order
// spec L132 mandates. WalletRef is the deterministic final tie-break. The
// authority rebalance aligns each wallet's priority to its rank; Lago then
// consumes `priority ASC, created_at ASC` = the intended total order.
//
// (OCR84-R1-09) Fail-closed guards: the 1..n answer must stay INSIDE the
// documented priority domain [1, MaxWalletPriority] — the same bound
// GrantIncludedCreditsPayload.Validate enforces on the creation-time value;
// #86's 12-month top-up TTL lets active wallets accumulate, and a rebalance
// that PUT rank 51+ out of the domain either bounces forever (authority
// rejects → attention) or silently falsifies the local contract. A
// DUPLICATE WalletRef (the map key) would silently overwrite one batch's
// rank — the E3 recovery-anomaly shape — and is refused loudly instead.
func WalletRank(batches []WalletRankInput) (map[string]int, error) {
	if len(batches) > MaxWalletPriority {
		return nil, fmt.Errorf("wallet rank overflow: %d active batches exceed the priority domain [1,%d]",
			len(batches), MaxWalletPriority)
	}
	seen := make(map[string]bool, len(batches))
	for _, b := range batches {
		if b.WalletRef == "" {
			return nil, errors.New("wallet rank input carries an empty wallet ref")
		}
		if seen[b.WalletRef] {
			return nil, fmt.Errorf("wallet rank input carries duplicate wallet ref %q", b.WalletRef)
		}
		seen[b.WalletRef] = true
	}
	ordered := append([]WalletRankInput(nil), batches...)
	sort.Slice(ordered, func(i, j int) bool {
		if !ordered[i].ExpiresAt.Equal(ordered[j].ExpiresAt) {
			return ordered[i].ExpiresAt.Before(ordered[j].ExpiresAt)
		}
		if !ordered[i].GrantedAt.Equal(ordered[j].GrantedAt) {
			return ordered[i].GrantedAt.Before(ordered[j].GrantedAt)
		}
		return ordered[i].WalletRef < ordered[j].WalletRef
	})
	ranks := make(map[string]int, len(ordered))
	for i, b := range ordered {
		ranks[b.WalletRef] = i + 1
	}
	return ranks, nil
}

// GrantIncludedCreditsPayload is the typed payload of grant_included_credits.
// One calendar month per grant: Period is the UTC "YYYY-MM", ExpiresAt the
// EXCLUSIVE period end (short-TTL wallet — included credits never roll
// over), CreditsMicro an integer micro-credit amount that must stay
// cent-aligned (% 10_000 == 0) so the adapter converts to the provider's
// decimal string exactly, never through binary float.
//
// WalletName (additive, #82 D4) optionally overrides the wallet identity:
// empty keeps the Base-plan MonthlyWalletName batch; the purchase first-
// period grant sets PurchaseWalletName so purchase credits never collide
// with the Base monthly batch (F11: grant idempotency matches by name AND
// metadata — a shared name would trigger grant content conflicts).
//
// TopUp (additive, #85 G-B) selects the one-shot top-up family: no period
// identity (a period-less wallet of this tenant is exactly the top-up
// batch the #86 snapshot classification reads back), a REQUIRED
// deterministic WalletName as the read-before-create idempotency anchor of
// a lost response's recovery, and its own TTL (the 12-month accumulation
// class) instead of the exclusive period end.
type GrantIncludedCreditsPayload struct {
	TenantID           uint64
	ExternalCustomerID string    // derived-equality enforced
	Period             string    // "YYYY-MM", UTC; empty allowed only for TopUp
	CreditsMicro       int64     // > 0 and cent-aligned (CreditsMicro % 10_000 == 0)
	ExpiresAt          time.Time // exclusive period end, > grant time
	WalletName         string    // optional; empty = MonthlyWalletName (Base batch)
	// Priority is the consumption-order class encoded into the wallet at
	// creation (#86 Task 2; MonthlyWalletPriorityFor / TopUpWalletPriority).
	// Required [1,50] — the consumption order must be explicit, never the
	// provider default. The invariant's authority is the refresh-time
	// rebalance; this initial value only reduces rebalance writes.
	Priority int
	// TopUp marks the #85 one-shot purchase grant family.
	TopUp bool
}

// Validate enforces the grant contract per family: derived identity,
// cent-aligned positive integer credits, the priority class, and — per
// family — the monthly form (strict period, exclusive period end) or the
// top-up form (named wallet, own expiry still in the future).
func (p GrantIncludedCreditsPayload) Validate() error {
	if p.TenantID == 0 {
		return errors.New("invalid grant payload: tenant is required")
	}
	if p.ExternalCustomerID != ExternalCustomerID(p.TenantID) {
		return errors.New("invalid grant payload: external_customer_id must equal ExternalCustomerID(tenant)")
	}
	if p.TopUp {
		if p.WalletName == "" {
			return errors.New("invalid grant payload: a top-up grant requires its deterministic wallet name")
		}
	} else {
		end, err := PeriodEnd(p.Period)
		if err != nil {
			return fmt.Errorf("invalid grant payload: %v", err)
		}
		if !p.ExpiresAt.Equal(end) {
			return fmt.Errorf("invalid grant payload: expires_at must be the exclusive period end %s", end.Format(time.RFC3339))
		}
	}
	if p.ExpiresAt.Compare(time.Now().UTC()) <= 0 {
		return errors.New("invalid grant payload: expires_at must be after the grant time")
	}
	if p.CreditsMicro <= 0 {
		return errors.New("invalid grant payload: credits_micro must be positive")
	}
	if p.CreditsMicro%10_000 != 0 {
		return errors.New("invalid grant payload: credits_micro must be cent-aligned (% 10_000 == 0)")
	}
	if p.Priority < 1 || p.Priority > MaxWalletPriority {
		return fmt.Errorf("invalid grant payload: priority must be within [1,%d] (the consumption-order class)", MaxWalletPriority)
	}
	return nil
}

// GrantCreditsCommandKey derives the seam command idempotency identity
// "grant_included_credits:<ext-customer>:<period>" — deterministic per
// (tenant, period): a replayed grant addresses the same month, never a
// second wallet (E3: the provider API has no idempotency of its own).
func GrantCreditsCommandKey(externalCustomerID, period string) string {
	return string(CommandKindGrantIncludedCredits) + ":" + externalCustomerID + ":" + period
}

// MonthlyWalletName derives the deterministic wallet name
// "weknora-tenant-<id>-<YYYY-MM>" for one monthly batch — the second-layer
// grant idempotency anchor (adapter read-before-create matches by name,
// metadata as fallback).
func MonthlyWalletName(tenantID uint64, period string) string {
	return ExternalCustomerID(tenantID) + "-" + period
}

// PurchaseWalletName derives the deterministic wallet identity for a
// purchase first-period credits batch (#82 D4):
// "<ext-purchase-subscription-id>-<YYYY-MM>". Deliberately distinct from
// MonthlyWalletName (the Base-plan monthly batch) so the two grant families
// never collide on the adapter's by-name idempotency match (F11: a shared
// name would make the purchase grant re-use — or content-conflict with —
// the Base monthly wallet).
func PurchaseWalletName(tenantID uint64, period string) string {
	return ExternalPurchaseSubscriptionID(tenantID) + "-" + period
}

// TopUpWalletName derives the deterministic wallet identity of one paid
// top-up order (#85): "<ext-customer>-topup-<hash12(orderID)>". The
// "-topup-" suffix never parses as a period, so both adapters classify the
// batch source=topup (the period-less family of the #86 snapshot
// classification); the same name on every replay is the read-before-create
// idempotency anchor of a lost response's recovery (E3).
func TopUpWalletName(tenantID uint64, orderID string) string {
	sum := sha256.Sum256([]byte("topup:" + orderID))
	return ExternalCustomerID(tenantID) + "-topup-" + hex.EncodeToString(sum[:6])
}

// TopUpGrantCommandKey derives the seam command idempotency identity of one
// order's top-up grant — deterministic per order, never colliding with the
// monthly (customer, period) key space.
func TopUpGrantCommandKey(tenantID uint64, orderID string) string {
	return string(CommandKindGrantIncludedCredits) + ":topup:" + TopUpWalletName(tenantID, orderID)
}

// Wallet metadata keys — the E3 recovery-by-metadata obligation: a grant
// whose create response was lost is recovered by querying wallets carrying
// these markers, never by a blind re-create.
const (
	WalletMetaTenant = "weknora_tenant"
	WalletMetaPeriod = "weknora_period"
	// WalletMetaPurchasePeriod marks a PURCHASE first-period batch (#82 D4):
	// a distinct key so purchase wallets never match a Base monthly batch's
	// metadata (and vice versa) even if a name ever collided.
	WalletMetaPurchasePeriod = "weknora_purchase_period"
)

// MonthlyPeriod formats a time as the UTC calendar period "YYYY-MM".
func MonthlyPeriod(now time.Time) string {
	return now.UTC().Format("2006-01")
}

// PeriodEnd returns the EXCLUSIVE UTC end instant of a "YYYY-MM" period
// (the first nanosecond of the next month). Malformed periods error; month
// rolls December into January of the next year.
func PeriodEnd(period string) (time.Time, error) {
	start, err := time.Parse("2006-01", period)
	if err != nil {
		return time.Time{}, fmt.Errorf("period %q must be YYYY-MM (UTC)", period)
	}
	// time.Parse("2006-01") rejects month 13 and single-digit months; assert
	// the canonical form too so "2026-9"-style inputs can never pass.
	if start.Format("2006-01") != period {
		return time.Time{}, fmt.Errorf("period %q must be YYYY-MM (UTC)", period)
	}
	year, month, _ := start.Date()
	return time.Date(year, month+1, 1, 0, 0, 0, 0, time.UTC), nil
}

// MicroToDecimalString converts cent-aligned micro-credits to the provider's
// decimal-string credit form with exact integer mapping ("credits.cents",
// e.g. 9_900_000 → "9.9", 100_000 → "0.1", 1_000_000 → "1"). Non-cent-aligned
// input is REJECTED — the conversion never rounds and never touches binary
// float (spec: integer minor units end-to-end).
func MicroToDecimalString(micro int64) (string, error) {
	if micro < 0 || micro%10_000 != 0 {
		return "", fmt.Errorf("micro amount %d is not a positive cent-aligned value", micro)
	}
	cents := micro / 10_000
	return trimFixed(cents, 2), nil
}

// CentsToMicro scales integer credit cents up to micro-credits (× 10^4) —
// the exact reverse of MicroToDecimalString's scaling.
func CentsToMicro(cents int64) int64 {
	return cents * 10_000
}

// trimFixed renders value/10^places as a decimal string with trailing zeros
// and a trailing dot trimmed (formatFixed's exact-integer sibling).
func trimFixed(value int64, places int) string {
	s := formatFixed(value, places)
	if i := strings.IndexByte(s, '.'); i >= 0 {
		s = strings.TrimRight(s, "0")
		s = strings.TrimSuffix(s, ".")
	}
	return s
}

// SnapshotKindBenefits reads the billing authority's benefits truth for one
// tenant: the subscription state, the plan version's entitlements, and the
// wallet balances (raw authority truth — the coordinator overlays registry
// expiry). W4 additive kind (#80, ADR-0014 additive-kind rule).
const SnapshotKindBenefits SnapshotKind = "benefits"

// SubscriptionState is the closed authority-truth enum for the benefits
// snapshot: the subscription is active or it is not yet (absent, incomplete,
// unconfirmed all map to pending). This is NOT the API envelope — the
// Billing API projects its own closed tokens. SubscriptionStateTerminated
// is the #102 additive terminal state: the workspace was closed and the
// authority subscription terminated under the retention policy.
const (
	SubscriptionStateActive     = "active"
	SubscriptionStatePending    = "pending"
	SubscriptionStateTerminated = "terminated"
)

// CreditBatchSnapshot is one wallet batch inside a benefits snapshot: the
// calendar period, the RAW authority balance micro (expiry overlay is the
// coordinator's job), and the batch's expiry instant. Source and GrantedAt
// are additive (#86): the closed batch family (monthly vs top-up — top-up
// batches are the #85 payment-confirmed credits shape) and the authority's
// grant instant (created_at), the consumption-order tie-break input
// ("earliest expiry, then earliest grant").
type CreditBatchSnapshot struct {
	Period       string
	BalanceMicro int64
	ExpiresAt    time.Time
	Source       string    // closed set: BatchSourceMonthly | BatchSourceTopUp
	GrantedAt    time.Time // the wallet's created_at (grant time, tie-break)
	// WalletRef is the batch's deterministic wallet name — the batch
	// identity the local lot projection keys on (#86 Task 3). The Base
	// monthly and purchase first-period batches of one period carry
	// DISTINCT refs (their own wallet names), so both stay addressable.
	WalletRef string
}

// Batch sources — the closed two-family set. A top-up batch carries no
// calendar period (Period stays ""); a monthly-family batch (Base monthly
// OR purchase first-period — both expire at period end and never roll over,
// #82 D4) answers BatchSourceMonthly.
const (
	BatchSourceMonthly = "monthly"
	BatchSourceTopUp   = "topup"
)

// BenefitsSnapshot is the authority-side benefits section of a Snapshot: the
// subscription truth, the attached plan code (seam-internal — the service
// maps it through commercial_plan_publications), the entitlement feature
// map, the summed raw wallet balance and the per-batch breakdown.
type BenefitsSnapshot struct {
	TenantID          uint64
	SubscriptionState string // closed set SubscriptionState*
	PlanCode          string // seam-internal; the service maps it via commercial_plan_publications
	Features          map[string]bool
	BalanceMicro      int64 // raw authority balance; the service overlays registry expiry
	Batches           []CreditBatchSnapshot
	CheckedAt         time.Time
}

// CommandKindRebalanceCreditsOrder converges the customer's active wallets'
// priorities onto the correct consumption order (#86 Task 2): the adapter
// lists the active wallets, ranks them by WalletRank (expires_at ASC,
// created_at ASC) and PUTs each wallet whose current priority diverges from
// its rank — aligned wallets answer ZERO writes (idempotent no-op). A replay
// re-computes from the authoritative list and converges again: the command
// is a calibrating reconciliation, never a mutation with its own state.
const CommandKindRebalanceCreditsOrder CommandKind = "rebalance_credits_order"

// RebalanceCreditsOrderPayload is the typed payload of
// rebalance_credits_order — one customer's wallet set, derived identity only.
type RebalanceCreditsOrderPayload struct {
	TenantID           uint64
	ExternalCustomerID string // must equal ExternalCustomerID(TenantID)
}

// Validate enforces the derived-identity equality.
func (p RebalanceCreditsOrderPayload) Validate() error {
	if p.TenantID == 0 {
		return errors.New("invalid rebalance_credits_order payload: tenant is required")
	}
	if p.ExternalCustomerID != ExternalCustomerID(p.TenantID) {
		return errors.New("invalid rebalance_credits_order payload: external_customer_id must equal ExternalCustomerID(tenant)")
	}
	return nil
}

// RebalanceCreditsOrderCommandKey derives the seam command identity
// "rebalance_credits_order:<ext-customer>" — a convergent calibration: a
// replay re-ranks from the authoritative list and is harmless by design.
func RebalanceCreditsOrderCommandKey(externalCustomerID string) string {
	return string(CommandKindRebalanceCreditsOrder) + ":" + externalCustomerID
}
