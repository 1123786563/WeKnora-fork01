package service

import (
	"context"
	"fmt"

	"github.com/Tencent/WeKnora/internal/metrics"
	"github.com/Tencent/WeKnora/internal/modules/commercial"
	"github.com/Tencent/WeKnora/internal/modules/craft"
)

// CraftUsageStore is the persistence port of the usage ledger: the durable
// craft.UsageSink plus the read-back queries reconciliation needs. The
// concrete implementation is repository.CraftUsageStore.
type CraftUsageStore interface {
	craft.UsageSink
	Facts(ctx context.Context, tenant uint64, runID string) ([]craft.UsageFact, error)
	FactsBySession(ctx context.Context, tenant uint64, sessionID string) ([]craft.UsageFact, error)
	FactsByDelegation(ctx context.Context, tenant uint64, delegationID string) ([]craft.UsageFact, error)
	Revisions(ctx context.Context, tenant uint64, callID, attemptID string) ([]craft.UsageFact, error)
}

// PhysicalCall is one real outbound model attempt observed at the
// server-side model gateway. Identity is issued BEFORE forwarding (see
// IssueCallID/IssueAttemptID) and binds the server-resolved tenant, run,
// delegation, model and funding; nothing here is trusted from model
// arguments. Usage is the provider-observed token block; nil records an
// unknown observation (stream break, cancellation) with no numbers.
type PhysicalCall struct {
	TenantID     uint64
	RunID        string
	DelegationID string
	CallID       string
	AttemptID    string
	Runtime      string // craft.RuntimeMain or craft.RuntimeOC
	ModelID      string
	Funding      string
	Usage        *craft.UsageTotals
}

// OCDelegateUsage is one aggregate usage event emitted by the OpenCode
// sub-execution for one delegation. Aggregate events are cross-check input
// ONLY — they never enter the ledger and are never a second billing source.
type OCDelegateUsage struct {
	TenantID     uint64
	DelegationID string
	Totals       craft.UsageTotals
}

// OCVerification is the cross-check verdict of one OC aggregate event
// against the recorded physical child facts of the same delegation.
type OCVerification struct {
	DelegationID string
	// Recorded folds the CURRENT revisions of the delegation's physical
	// child facts (unknown children stay on their own line and make an
	// exact aggregate unverifiable).
	Recorded craft.UsageTotals
	// Reported is what the aggregate event claimed.
	Reported      craft.UsageTotals
	RecordedFacts int
	Matched       bool
}

// CraftUsageService records the physical model attempts of a Craft run:
// the main agent runtime and the OC child runtime each append their own
// facts at their own real model-call boundaries, a retry is a new attempt,
// tool-result handoff from child to main adds NOTHING (the main agent's
// next real model call is its own new fact), and late or broken
// observations travel as explicit revisions (unknown/corrected) so history
// is never overwritten. The service computes no prices and keeps no Credits
// ledger: billability stays with the commercial specialty — platform-funded
// model calls are billable there (commercial.BillableModel), BYOK model
// calls are recorded as observations only, and OpenMeter delivery is the G4
// worker consuming the outbox.
type CraftUsageService struct {
	store CraftUsageStore
}

// NewCraftUsageService assembles the usage service over the durable store.
func NewCraftUsageService(store CraftUsageStore) *CraftUsageService {
	return &CraftUsageService{store: store}
}

// IssueCallID derives the logical call identity the server-side model
// gateway assigns BEFORE forwarding one outbound call, binding tenant, run,
// delegation, model and funding. See craft.DeriveCallID.
func (s *CraftUsageService) IssueCallID(tenant uint64, runID, delegationID, modelID, funding string, callSeq int64) string {
	return craft.DeriveCallID(tenant, runID, delegationID, modelID, funding, callSeq)
}

// IssueAttemptID derives the physical attempt identity of one logical call;
// a real retry forwards again with attemptSeq+1 and records as a NEW
// physical fact. See craft.DeriveAttemptID.
func (s *CraftUsageService) IssueAttemptID(callID string, attemptSeq int64) string {
	return craft.DeriveAttemptID(callID, attemptSeq)
}

// buildFact validates a PhysicalCall against the commercial funding
// vocabulary (untrusted funding never enters the ledger) and assembles the
// UsageFact. nil usage maps to the unknown observation: no numbers, ever.
func buildFact(in PhysicalCall) (craft.UsageFact, error) {
	if err := commercial.ValidateFunding(in.Funding); err != nil {
		return craft.UsageFact{}, fmt.Errorf("%w: funding %q is not a server-recognized funding source",
			craft.ErrInvalidInput, in.Funding)
	}
	if in.Runtime == craft.RuntimeOC && in.DelegationID == "" {
		return craft.UsageFact{}, fmt.Errorf("%w: oc physical call without delegation", craft.ErrInvalidInput)
	}
	status := craft.UsageStatusReported
	var totals craft.UsageTotals
	if in.Usage != nil {
		totals = *in.Usage
	} else {
		status = craft.UsageStatusUnknown
	}
	f := craft.UsageFact{
		RunID:        in.RunID,
		DelegationID: in.DelegationID,
		CallID:       in.CallID,
		AttemptID:    in.AttemptID,
		Runtime:      in.Runtime,
		ModelID:      in.ModelID,
		Funding:      in.Funding,
		Status:       status,
		TenantID:     in.TenantID,
		Input:        totals.Input,
		Output:       totals.Output,
		Cached:       totals.Cached,
	}
	f.ID = craft.UsageKey(f.TenantID, f.CallID, f.AttemptID)
	return f, nil
}

// RecordPhysicalCall appends ONE physical attempt fact — from the main
// runtime or from an OC child runtime — exactly as observed at the model
// gateway. Appending is idempotent per (tenant, call, attempt): redelivered
// observations (including OC summary replays) never count twice, and a
// changed observation for the same attempt is a conflict that must travel
// through CorrectLateUsage.
func (s *CraftUsageService) RecordPhysicalCall(ctx context.Context, in PhysicalCall) (craft.UsageFact, error) {
	f, err := buildFact(in)
	if err != nil {
		return craft.UsageFact{}, err
	}
	if err := s.store.Append(ctx, f); err != nil {
		return craft.UsageFact{}, err
	}
	return f, nil
}

// CorrectLateUsage files a late observation for an ALREADY recorded
// physical attempt as an explicit corrected revision: a stream that broke
// and later delivered usage, or usage arriving after a cancellation. The
// correction never creates a second physical fact and never overwrites the
// recorded unknown — the full revision trail stays queryable as the
// reconciliation record (see Reconciliation).
func (s *CraftUsageService) CorrectLateUsage(ctx context.Context, in PhysicalCall) error {
	f, err := buildFact(in)
	if err != nil {
		return err
	}
	f.Status = craft.UsageStatusCorrected
	if err := s.store.Correct(ctx, f); err != nil {
		return err
	}
	metrics.CraftReconciled("corrected")
	return nil
}

// ObserveOCAggregate cross-checks one OC aggregate usage event against the
// recorded physical child facts of the same delegation. It writes NOTHING:
// the aggregate can verify the child ledger, never extend or re-bill it,
// so duplicate aggregate events are free and a parent rollup (which folds
// the main agent's tokens) honestly reports a mismatch. An aggregate can
// only match when every child fact is observed: an unobserved (unknown)
// child makes exact equality unverifiable.
func (s *CraftUsageService) ObserveOCAggregate(ctx context.Context, agg OCDelegateUsage) (OCVerification, error) {
	if agg.TenantID == 0 || agg.DelegationID == "" {
		return OCVerification{}, fmt.Errorf("%w: aggregate event without tenant or delegation", craft.ErrInvalidInput)
	}
	facts, err := s.store.FactsByDelegation(ctx, agg.TenantID, agg.DelegationID)
	if err != nil {
		return OCVerification{}, err
	}
	verdict := OCVerification{
		DelegationID:  agg.DelegationID,
		Reported:      agg.Totals,
		RecordedFacts: len(facts),
	}
	for _, f := range facts {
		verdict.Recorded.Add(f)
	}
	// Recorded.Unknown counts the delegation's unobserved children: they
	// are folded onto their own line above and block an exact match.
	verdict.Matched = len(facts) > 0 &&
		verdict.Recorded.Unknown == 0 &&
		verdict.Recorded.Input == agg.Totals.Input &&
		verdict.Recorded.Output == agg.Totals.Output &&
		verdict.Recorded.Cached == agg.Totals.Cached &&
		verdict.Recorded.Facts == agg.Totals.Facts
	if verdict.Matched {
		metrics.CraftReconciled("matched")
	} else {
		metrics.CraftReconciled("mismatched")
	}
	return verdict, nil
}

// Facts returns the CURRENT revision of every physical attempt of one
// tenant's run — the traceable ground truth of "which space/run did this
// call come from": each fact carries its run, delegation, runtime, model
// and funding.
func (s *CraftUsageService) Facts(ctx context.Context, tenant uint64, runID string) ([]craft.UsageFact, error) {
	return s.store.Facts(ctx, tenant, runID)
}

// SessionFacts returns the CURRENT revision of every physical attempt
// recorded across one session's runs — the O04 usage view's read side.
func (s *CraftUsageService) SessionFacts(ctx context.Context, tenant uint64, sessionID string) ([]craft.UsageFact, error) {
	return s.store.FactsBySession(ctx, tenant, sessionID)
}

// Totals folds the run's current facts into observed totals. Unknown facts
// are NOT summed into the token counts — they are reported on their own
// line (see UnknownConsumption) so unobserved consumption is never
// estimated from the final model response or fabricated as zero.
func (s *CraftUsageService) Totals(ctx context.Context, tenant uint64, runID string) (craft.UsageTotals, error) {
	facts, err := s.store.Facts(ctx, tenant, runID)
	if err != nil {
		return craft.UsageTotals{}, err
	}
	var totals craft.UsageTotals
	for _, f := range facts {
		totals.Add(f)
	}
	return totals, nil
}

// UnknownConsumption lists the run's physical attempts whose usage could
// not be observed — a separate line item, never zeros.
func (s *CraftUsageService) UnknownConsumption(ctx context.Context, tenant uint64, runID string) ([]craft.UsageFact, error) {
	facts, err := s.store.Facts(ctx, tenant, runID)
	if err != nil {
		return nil, err
	}
	unknown := make([]craft.UsageFact, 0)
	for _, f := range facts {
		if f.Status == craft.UsageStatusUnknown {
			unknown = append(unknown, f)
		}
	}
	return unknown, nil
}

// Reconciliation returns the full revision trail of one physical attempt:
// the audit record for stream breaks (revision 1 unknown) and late arrivals
// (corrected revisions appended afterwards).
func (s *CraftUsageService) Reconciliation(ctx context.Context, tenant uint64, callID, attemptID string) ([]craft.UsageFact, error) {
	return s.store.Revisions(ctx, tenant, callID, attemptID)
}
