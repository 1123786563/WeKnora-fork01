package commercial

import (
	"context"
	"errors"
	"time"

	domain "github.com/Tencent/WeKnora/internal/modules/commercial"
	repocommercial "github.com/Tencent/WeKnora/internal/modules/commercial/repository/commercial"

	"gorm.io/gorm"
)

// Closed product states of the billing account (the Billing API envelope):
// linked (the authority holds this space's customer) or pending (an ensure
// was attempted but nothing confirmed it — including every indeterminate
// platform outcome). A pending account never blocks non-commercial space
// functions (spec "Public product states").
const (
	BillingAccountLinked  = "linked"
	BillingAccountPending = "pending"
)

// BillingAccountStatus is the provider-neutral account answer: the closed
// state, a closed reason token (unconfigured|unreachable|invalid_response|
// unsupported, empty iff linked), and the confirmation time (nil when never
// confirmed). No provider identifier, URL or error text ever appears.
type BillingAccountStatus struct {
	State     string
	Reason    string
	EnsuredAt *time.Time
}

// BillingAccountService owns the lazy ensure-on-first-billing-access flow
// (#78): idempotently establish the space's Billing Account (the immutable
// Tenant→Customer mapping) through the frozen CommercialPlatform seam. The
// three idempotency layers: the DB unique constraints (one row per space),
// the stable Command.Key, and the deterministic identity + adapter
// read-before-create (a replay after ANY lost response resolves by identity
// and never creates a second customer).
type BillingAccountService struct {
	store    *repocommercial.BillingAccountStore
	platform domain.CommercialPlatform
}

// NewBillingAccountService builds the service. A nil platform is legal
// (wiring gaps fail closed as pending/unconfigured, the readiness posture);
// a nil database is a construction error.
func NewBillingAccountService(db *gorm.DB, platform domain.CommercialPlatform) (*BillingAccountService, error) {
	if db == nil {
		return nil, errors.New("billing account service requires a database")
	}
	return &BillingAccountService{
		store:    repocommercial.NewBillingAccountStore(db),
		platform: platform,
	}, nil
}

// EnsureBillingAccount runs the lazy ensure: derive the deterministic
// identity; EnsurePending lets the DB decide the single row under
// concurrency; a linked row answers immediately; otherwise recover by
// identity FIRST (an authority snapshot that already holds the customer —
// the lost-response fast path, no submit at all) and only then submit the
// ensure command. Platform failures are a STATE, not a caller error: the
// row stays pending and the closed reason token explains why (the outage
// posture of the readiness endpoint). Only DB errors return an error.
// tenantID comes exclusively from the caller's authenticated scope; there
// is no input through which one tenant names another.
func (s *BillingAccountService) EnsureBillingAccount(ctx context.Context, tenantID uint64, displayName, actor string) (BillingAccountStatus, error) {
	if s == nil || s.store == nil {
		return BillingAccountStatus{State: BillingAccountPending, Reason: "unconfigured"}, nil
	}
	if tenantID == 0 {
		return BillingAccountStatus{}, errors.New("billing account ensure requires an authenticated tenant scope")
	}
	if s.platform == nil {
		// Fail closed honestly: no seam wired means no authority answer.
		if _, err := s.store.EnsurePending(ctx, repocommercial.BillingAccount{
			TenantID: tenantID, ExternalCustomerID: domain.ExternalCustomerID(tenantID),
		}); err != nil {
			return BillingAccountStatus{}, err
		}
		return BillingAccountStatus{State: BillingAccountPending, Reason: "unconfigured"}, nil
	}

	extID := domain.ExternalCustomerID(tenantID)
	row, err := s.store.EnsurePending(ctx, repocommercial.BillingAccount{
		TenantID: tenantID, ExternalCustomerID: extID,
	})
	if err != nil {
		return BillingAccountStatus{}, err
	}
	if row.State == repocommercial.BillingAccountStateLinked {
		return linkedStatus(row), nil
	}

	// Recovery by identity first: a lost create response leaves the customer
	// on the authority — the snapshot resolves that without any submit.
	snap, snapErr := s.platform.ReadSnapshot(ctx, domain.SnapshotQuery{
		Kind: domain.SnapshotKindAccount, TenantID: tenantID,
	})
	if snapErr == nil && snap.Account != nil && snap.Account.State == domain.AccountStateLinked {
		row, err = s.store.MarkLinked(ctx, tenantID, extID, snap.Account.CheckedAt)
		if err != nil {
			return BillingAccountStatus{}, err
		}
		return linkedStatus(row), nil
	}

	// Not linked (absent, or the snapshot itself could not answer — the
	// submit below reclassifies an unreachable authority): ensure by the
	// stable idempotency identity.
	receipt, submitErr := s.platform.SubmitCommand(ctx, domain.Command{
		Kind:   domain.CommandKindEnsureCustomer,
		Key:    "ensure_customer:" + extID,
		Actor:  actor,
		Reason: BillingAccountEnsureReason,
		Payload: domain.EnsureCustomerPayload{
			TenantID:           tenantID,
			ExternalCustomerID: extID,
			DisplayName:        displayName,
		},
	})
	if submitErr != nil {
		// Indeterminate or refused: nothing rolls back; the row stays
		// pending and the next ensure re-reads by identity before any create.
		return BillingAccountStatus{State: BillingAccountPending, Reason: platformReason(submitErr)}, nil
	}
	ref := receipt.ExternalID
	if ref == "" {
		ref = extID
	}
	row, err = s.store.MarkLinked(ctx, tenantID, ref, receipt.RecordedAt)
	if err != nil {
		return BillingAccountStatus{}, err
	}
	return linkedStatus(row), nil
}

func linkedStatus(row repocommercial.BillingAccount) BillingAccountStatus {
	ensured := row.EnsuredAt
	return BillingAccountStatus{State: BillingAccountLinked, Reason: "", EnsuredAt: ensured}
}

// platformReason maps a seam failure onto the closed reason token set —
// the only failure vocabulary the Billing API may ever carry.
func platformReason(err error) string {
	reason := "unsupported"
	switch {
	case err == nil:
		reason = ""
	case errors.Is(err, domain.ErrPlatformUnconfigured):
		reason = "unconfigured"
	case errors.Is(err, domain.ErrPlatformUnreachable):
		reason = "unreachable"
	case errors.Is(err, domain.ErrPlatformInvalidResponse):
		reason = "invalid_response"
	}
	return reason
}

// BillingAccountEnsureReason is the audit reason the lazy ensure command
// carries (T06): the documented trigger is the first billing access.
const BillingAccountEnsureReason = "first_billing_access"
