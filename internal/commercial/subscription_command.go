package commercial

import (
	"errors"
	"fmt"
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

// GrantIncludedCreditsPayload is the typed payload of grant_included_credits.
// One calendar month per grant: Period is the UTC "YYYY-MM", ExpiresAt the
// EXCLUSIVE period end (short-TTL wallet — included credits never roll
// over), CreditsMicro an integer micro-credit amount that must stay
// cent-aligned (% 10_000 == 0) so the adapter converts to the provider's
// decimal string exactly, never through binary float.
type GrantIncludedCreditsPayload struct {
	TenantID           uint64
	ExternalCustomerID string    // derived-equality enforced
	Period             string    // "YYYY-MM", UTC
	CreditsMicro       int64     // > 0 and cent-aligned (CreditsMicro % 10_000 == 0)
	ExpiresAt          time.Time // exclusive period end, > grant time
}

// Validate enforces the monthly grant contract: derived identity, strict
// period form, cent-aligned positive integer credits, and the exclusive
// period end still in the future.
func (p GrantIncludedCreditsPayload) Validate() error {
	if p.TenantID == 0 {
		return errors.New("invalid grant payload: tenant is required")
	}
	if p.ExternalCustomerID != ExternalCustomerID(p.TenantID) {
		return errors.New("invalid grant payload: external_customer_id must equal ExternalCustomerID(tenant)")
	}
	end, err := PeriodEnd(p.Period)
	if err != nil {
		return fmt.Errorf("invalid grant payload: %v", err)
	}
	if !p.ExpiresAt.Equal(end) {
		return fmt.Errorf("invalid grant payload: expires_at must be the exclusive period end %s", end.Format(time.RFC3339))
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

// Wallet metadata keys — the E3 recovery-by-metadata obligation: a grant
// whose create response was lost is recovered by querying wallets carrying
// these markers, never by a blind re-create.
const (
	WalletMetaTenant = "weknora_tenant"
	WalletMetaPeriod = "weknora_period"
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
// Billing API projects its own closed tokens.
const (
	SubscriptionStateActive  = "active"
	SubscriptionStatePending = "pending"
)

// CreditBatchSnapshot is one wallet batch inside a benefits snapshot: the
// calendar period, the RAW authority balance micro (expiry overlay is the
// coordinator's job), and the batch's expiry instant.
type CreditBatchSnapshot struct {
	Period       string
	BalanceMicro int64
	ExpiresAt    time.Time
}

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

