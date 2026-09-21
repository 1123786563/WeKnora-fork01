package commercial

import (
	"testing"
	"time"
)

// TestMigrationDoesNotInventPaidCreditOrConnectionOwnership is the brief
// Step-1 acceptance assertion (kept verbatim).
func TestMigrationDoesNotInventPaidCreditOrConnectionOwnership(t *testing.T) {
	got := InitialCommercialState(false)
	if got.Plan != "basic" || got.IssueCredits || got.ConnectionState != "requires_reauthorization" {
		t.Fatal(got)
	}
}

// A proven connection owner reaches ready_to_bind, but migration still grants
// only the free basic tier and issues no paid credits.
func TestMigrationProvenOwnerIsReadyToBindButStillBasicWithoutCredits(t *testing.T) {
	got := InitialCommercialState(true)
	if got.Plan != "basic" || got.IssueCredits || got.ConnectionState != "ready_to_bind" {
		t.Fatal(got)
	}
}

// An existing F02 Customer mapping is reused, never duplicated.
func TestMigrationReusesExistingCustomerAndNeverDuplicates(t *testing.T) {
	plan := PlanTenantMigration(TenantMigrationInput{TenantID: 7, ExistingCustomerID: "cus_existing"})
	if !plan.ReuseExistingCustomer || plan.CustomerID != "cus_existing" || !plan.BindRequired {
		t.Fatal(plan)
	}
}

// Unknown credentials are never shared or invented: a tenant without an
// existing Customer mapping and without operator-provided evidence is
// skipped, and nothing is bound for it.
func TestMigrationSkipsTenantWithoutCustomerEvidence(t *testing.T) {
	plan := PlanTenantMigration(TenantMigrationInput{TenantID: 8})
	if plan.BindRequired || plan.CustomerID != "" || plan.SkipReason == "" {
		t.Fatal(plan)
	}
}

// Legacy balances are reported but NEVER converted into money grants, and
// data sources are preserved until the new binding is verified.
func TestMigrationLegacyBalanceNeverBecomesMoneyGrant(t *testing.T) {
	plan := PlanTenantMigration(TenantMigrationInput{
		TenantID:             9,
		ExistingCustomerID:   "cus_9",
		LegacyBalanceCredits: 500000,
		DataSourceCount:      3,
	})
	if plan.LegacyBalanceConverted || !plan.DataSourcesPreserved || plan.Decision.IssueCredits {
		t.Fatal(plan)
	}
}

// Shadow-cutover semantics: billability is pinned to the trusted call START
// time and the cutover config version. Old shadow events replayed after the
// cutover must NOT become billable, and calls started before the cutover stay
// shadow no matter when they are reported.
func TestMigrationShadowCutoverPinsBillabilityToTrustedCallStart(t *testing.T) {
	cutover := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	if ShadowBillable(cutover.Add(-time.Second), cutover, 3, 3) {
		t.Fatal("pre-cutover call must stay shadow even when reported later")
	}
	if !ShadowBillable(cutover, cutover, 3, 3) {
		t.Fatal("call starting exactly at the cutover must be billable")
	}
	if !ShadowBillable(cutover.Add(time.Hour), cutover, 3, 3) {
		t.Fatal("post-cutover call must be billable")
	}
	if ShadowBillable(cutover.Add(time.Hour), cutover, 2, 3) {
		t.Fatal("event recorded under an older config version must stay shadow")
	}
}

// Any write requires an explicit --apply; the default is dry-run.
func TestMigrationApplyRequiresExplicitFlag(t *testing.T) {
	if ApplyAllowed(false) {
		t.Fatal("default must be dry-run with zero writes")
	}
	if !ApplyAllowed(true) {
		t.Fatal("explicit apply must allow writes")
	}
}

// Workspace deletion is refused while in-flight commercial records exist.
func TestMigrationDeletionRefusesPendingCommercialWork(t *testing.T) {
	d := CheckDeletionReadiness(DeletionReadinessInput{Pending: []string{"settlement:41", "payment:7"}})
	if d.Allow || d.AutoDelete || len(d.BlockedBy) != 2 {
		t.Fatal(d)
	}
}

// Without a configured retention policy nothing is auto-deleted, even when
// deletion is allowed; a configured policy version is recorded on the
// decision.
func TestMigrationDeletionNeverAutoDeletesWithoutRetentionPolicy(t *testing.T) {
	d := CheckDeletionReadiness(DeletionReadinessInput{})
	if !d.Allow || d.AutoDelete {
		t.Fatal(d)
	}
	withPolicy := CheckDeletionReadiness(DeletionReadinessInput{RetentionPolicyVersion: "ret-2026-09"})
	if !withPolicy.Allow || !withPolicy.AutoDelete || withPolicy.RetentionPolicyVersion != "ret-2026-09" {
		t.Fatal(withPolicy)
	}
}
