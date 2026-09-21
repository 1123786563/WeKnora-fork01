// Package commercial — migration of legacy spaces into the commercial model
// (O02). The rules fixed here: migration never invents commercial state. It
// issues no paid credits, never fabricates connection ownership, reuses
// existing Customer mappings instead of duplicating them, keeps legacy
// balances out of money grants, preserves data sources until the new binding
// is verified, and refuses workspace deletion while commercial work is
// in flight.
package commercial

import "time"

// ConnectionState vocabulary reuses the appconnector binding states (A07):
// requires_reauthorization for legacy credentials that cannot prove space
// ownership, ready_to_bind once ownership is proven.
const (
	ConnectionStateRequiresReauthorization = "requires_reauthorization"
	ConnectionStateReadyToBind             = "ready_to_bind"
)

// MigrationDecision is the initial commercial state a migrated space starts
// from. It is deliberately minimal: the free basic tier, no issued credits,
// and a connection state that never claims ownership the evidence does not
// prove.
type MigrationDecision struct {
	Plan            string
	IssueCredits    bool
	ConnectionState string
}

// InitialCommercialState returns the starting commercial state for a migrated
// space. Only proven connection ownership upgrades the connection state to
// ready_to_bind; even then the plan stays basic and no credits are issued.
func InitialCommercialState(provenConnectionOwner bool) MigrationDecision {
	state := ConnectionStateRequiresReauthorization
	if provenConnectionOwner {
		state = ConnectionStateReadyToBind
	}
	return MigrationDecision{Plan: "basic", IssueCredits: false, ConnectionState: state}
}

// TenantMigrationInput is the per-tenant evidence the migration CLI collects
// before planning. Nothing here is guessed: every field comes from an
// existing store or from an explicit operator decision.
type TenantMigrationInput struct {
	TenantID uint64
	// ExistingCustomerID is the F02 Account mapping already bound to this
	// tenant; non-empty means it MUST be reused, never duplicated.
	ExistingCustomerID string
	// OperatorCustomerID is a Customer ID the operator explicitly provides
	// for this tenant (verified in the external billing system). Used only
	// when no existing mapping exists.
	OperatorCustomerID string
	// ProvenConnectionOwner reports whether the A07 binding evidence proves
	// this tenant owns its space connections. Defaults to false: ownership is
	// never invented.
	ProvenConnectionOwner bool
	// LegacyBalanceCredits is the pre-commercial balance, reported for the
	// record only. It is NEVER converted into a money grant.
	LegacyBalanceCredits int64
	// DataSourceCount is the number of existing data sources, preserved
	// until the new binding is verified.
	DataSourceCount int
}

// TenantMigrationPlan is the idempotent per-tenant decision produced by
// PlanTenantMigration. Planning twice from the same input yields the same
// plan; applying a plan twice binds the same mapping twice (a no-op).
type TenantMigrationPlan struct {
	TenantID              uint64
	Decision              MigrationDecision
	CustomerID            string
	ReuseExistingCustomer bool
	// BindRequired reports whether apply must call AccountStore.Bind for
	// this tenant.
	BindRequired bool
	// SkipReason is non-empty when the tenant is skipped (never shared or
	// invented credentials/customers).
	SkipReason             string
	LegacyBalanceConverted bool
	DataSourcesPreserved   bool
}

// PlanTenantMigration derives one tenant's migration plan. Existing Customer
// mappings are reused; without a mapping, only an explicitly operator-
// provided Customer ID may be bound; otherwise the tenant is skipped.
// Legacy balances are never converted and data sources are always preserved.
func PlanTenantMigration(in TenantMigrationInput) TenantMigrationPlan {
	plan := TenantMigrationPlan{
		TenantID:               in.TenantID,
		Decision:               InitialCommercialState(in.ProvenConnectionOwner),
		LegacyBalanceConverted: false,
		DataSourcesPreserved:   true,
	}
	switch {
	case in.ExistingCustomerID != "":
		plan.ReuseExistingCustomer = true
		plan.CustomerID = in.ExistingCustomerID
		plan.BindRequired = true
	case in.OperatorCustomerID != "":
		plan.CustomerID = in.OperatorCustomerID
		plan.BindRequired = true
	default:
		plan.SkipReason = "no_existing_customer_mapping_and_no_operator_provided_customer_id"
	}
	return plan
}

// ApplyAllowed reports whether the migration may write. The CLI defaults to
// dry-run (zero writes); only an explicit --apply allows any write.
func ApplyAllowed(explicit bool) bool { return explicit }

// ShadowBillable decides whether a metered call becomes billable at the
// commercial cutover. The decision is pinned to the TRUSTED call start time
// and the config version: a call that started before the cutover instant
// stays shadow-metered forever — no matter when it is reported or replayed —
// and events recorded under a config version older than the cutover version
// never graduate to billable. The reporting/replay time is deliberately not
// an input; replaying old shadow events can never change their outcome.
func ShadowBillable(callStartedAt, enabledAt time.Time, eventConfigVersion, cutoverConfigVersion int64) bool {
	if eventConfigVersion != cutoverConfigVersion {
		return false
	}
	return !callStartedAt.Before(enabledAt)
}

// DeletionReadinessInput aggregates the in-flight commercial records and the
// configured retention policy for a workspace about to be deleted.
type DeletionReadinessInput struct {
	// Pending lists unsettled commercial records that block deletion, e.g.
	// "settlement:41", "payment:7", "refund:3".
	Pending []string
	// RetentionPolicyVersion is the configured retention policy version;
	// empty means no retention policy is configured.
	RetentionPolicyVersion string
}

// DeletionDecision is the pre-deletion commercial check outcome.
type DeletionDecision struct {
	// Allow reports whether workspace deletion may proceed.
	Allow bool
	// BlockedBy echoes the pending records that refused the deletion.
	BlockedBy []string
	// AutoDelete reports whether commercial records may EVER be deleted
	// automatically. It requires deletion to be allowed AND an explicitly
	// configured retention policy; without one, records are preserved
	// indefinitely.
	AutoDelete             bool
	RetentionPolicyVersion string
}

// CheckDeletionReadiness refuses workspace deletion while any in-flight
// commercial record (pending settlement/payment/refund) exists — the safer
// default — and never enables automatic deletion of commercial records
// without an explicitly configured retention policy version. Pending
// commercial records are never cascade-deleted.
func CheckDeletionReadiness(in DeletionReadinessInput) DeletionDecision {
	decision := DeletionDecision{
		Allow:                  len(in.Pending) == 0,
		BlockedBy:              in.Pending,
		RetentionPolicyVersion: in.RetentionPolicyVersion,
	}
	decision.AutoDelete = decision.Allow && in.RetentionPolicyVersion != ""
	return decision
}
