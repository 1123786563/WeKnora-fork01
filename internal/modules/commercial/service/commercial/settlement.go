package commercial

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	repocommercial "github.com/Tencent/WeKnora/internal/application/repository/commercial"
	domain "github.com/Tencent/WeKnora/internal/commercial"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	// ErrSettlementDatabaseMissing rejects wiring without durable storage:
	// settlement state must survive process restarts by construction.
	ErrSettlementDatabaseMissing = errors.New("settlement_database_missing")
	// ErrSettlementGatewayMissing rejects wiring without a commercial gateway.
	ErrSettlementGatewayMissing = errors.New("settlement_gateway_missing")
	// ErrSettlementNotFound rejects confirming an unknown settlement key.
	ErrSettlementNotFound = errors.New("settlement_not_found")
	// ErrSettlementNotFinal rejects Finalize of a fact that is not final:
	// partials, unknowns and parent aggregates are observations, never
	// settlements, and unknown never silently becomes a zero settlement.
	ErrSettlementNotFinal = errors.New("settlement_not_final")
	// ErrSettlementNetReversalUnsupported rejects a correction revision whose
	// net amount is negative: a negative reversal needs the reversal
	// capability of the V03-selected provider model (blocked-env), and
	// silently zero-settling it would delete history. The fact stays
	// unrecorded and the reservation hold stays intact.
	ErrSettlementNetReversalUnsupported = errors.New("settlement_net_reversal_unsupported")
)

// errSettlementReplay is an internal sentinel: the persist closure detected
// that this exact revision was already durably settled (fact row present).
// Returning it rolls the WHOLE Finalize transaction back — accounting
// included — and Finalize maps it to idempotent success, so a replay after a
// crash or a lost response can never double-protect, double-dispatch, or
// double-count the same revision. It never crosses the API surface.
var errSettlementReplay = errors.New("settlement_replay")

// SettlementRecord is the durable hand-off state of one usage revision. The
// primary key IS the domain SettlementKey, so retries, crashes, and concurrent
// workers converge on exactly one settlement per revision; corrections arrive
// as new revisions and therefore new keys. State reuses the domain vocabulary
// (dispatched/unknown/sent/accepted/confirmed) and history is never deleted:
// unknown outcomes are retained for reconciliation, not erased.
type SettlementRecord struct {
	Key            string    `gorm:"primaryKey;column:key"`
	TenantID       uint64    `gorm:"column:tenant_id;not null;index"`
	CallID         string    `gorm:"column:call_id;not null"`
	AttemptID      string    `gorm:"column:attempt_id;not null"`
	ReservationKey string    `gorm:"column:reservation_key;not null"`
	RunID          string    `gorm:"column:run_id;not null;default:''"`
	AmountMicro    int64     `gorm:"column:amount_micro;not null"`
	Revision       int64     `gorm:"column:revision;not null"`
	State          string    `gorm:"column:state;not null;default:dispatched"`
	ExternalID     string    `gorm:"column:external_id;not null;default:''"`
	Watermark      string    `gorm:"column:watermark;not null;default:''"`
	OccurredAt     time.Time `gorm:"column:occurred_at;not null"`
	UpdatedAt      time.Time `gorm:"column:updated_at;not null"`
}

func (SettlementRecord) TableName() string { return "commercial_settlement_records" }

// settlementEventPayload is the outbox payload of one final revision's
// settlement. The record row remains the source of truth: Dispatch re-reads
// it from storage instead of trusting the payload.
type settlementEventPayload struct {
	SettlementKey  string `json:"settlement_key"`
	TenantID       uint64 `json:"tenant_id"`
	RunID          string `json:"run_id,omitempty"`
	CallID         string `json:"call_id"`
	AttemptID      string `json:"attempt_id"`
	Revision       int64  `json:"revision"`
	ChargeMicro    int64  `json:"charge_micro"`
	ReservationKey string `json:"reservation_key"`
}

// SettlementService hands reserved usage to the commercial provider only
// after a CONFIRMED settlement. Finalize commits ONE local transaction: the
// usage fact, the final net amount, the settlement record, and the settlement
// outbox event, together with the reservation conversion (consumed part
// held → unreflected, actually-unused part released). The unreflected part
// stays protected until ConfirmSettlement carries real correlation evidence
// (external id plus the watermark it advanced); a balance drop is never
// evidence, and ingest acceptance is never confirmation.
type SettlementService struct {
	db      *gorm.DB
	budget  *repocommercial.BudgetStore
	outbox  *repocommercial.OutboxStore
	gateway domain.CommercialGateway
	rates   repocommercial.RateResolver
	now     func() time.Time
}

// NewSettlementService validates its wiring and migrates the settlement
// record table. The gateway arriving unconfigured (blocked-env) is legal:
// Dispatch then classifies every call as unknown and the spend stays
// protected until the connector is configured.
func NewSettlementService(db *gorm.DB, budget *repocommercial.BudgetStore, gateway domain.CommercialGateway, rates repocommercial.RateResolver) (*SettlementService, error) {
	if db == nil {
		return nil, ErrSettlementDatabaseMissing
	}
	if gateway == nil {
		return nil, ErrSettlementGatewayMissing
	}
	if rates == nil {
		return nil, repocommercial.ErrUsageRatesUnavailable
	}
	if budget == nil {
		budget = repocommercial.NewBudgetStore(db)
	}
	if err := db.AutoMigrate(&SettlementRecord{}); err != nil {
		return nil, err
	}
	return &SettlementService{
		db:      db,
		budget:  budget,
		outbox:  repocommercial.NewOutboxStore(db),
		gateway: gateway,
		rates:   rates,
		now:     func() time.Time { return time.Now().UTC() },
	}, nil
}

// Finalize records one final usage revision and converts its reservation in
// ONE local transaction. Semantics:
//   - only final facts settle (partials/unknown/aggregates are observations);
//   - the settled amount is the revision's NET delta against the attempt's
//     current version (a late correction revision settles only its delta),
//     so per-revision settlements at the provider sum to the net consumption;
//   - the reservation's consumed part moves held → unreflected, the unused
//     part is released, and the unreflected part REMAINS PROTECTED until a
//     confirmed settlement releases it;
//   - replaying the identical revision is an idempotent success (the sentinel
//     rolls the whole transaction back, so nothing is counted twice); the
//     same revision with a different charge is a conflict;
//   - revisions of one attempt must finalize sequentially — the caller
//     contract of U01's current-version pointer.
func (s *SettlementService) Finalize(ctx context.Context, fact domain.UsageFact, reservationKey string) (domain.Settlement, error) {
	if fact.Status != domain.UsageStatusFinal {
		return domain.Settlement{}, ErrSettlementNotFinal
	}
	if err := fact.Validate(); err != nil {
		return domain.Settlement{}, err
	}
	if reservationKey == "" {
		return domain.Settlement{}, repocommercial.ErrInvalidBudgetRequest
	}
	charge, err := s.chargeFor(fact)
	if err != nil {
		return domain.Settlement{}, err
	}
	previous, err := s.previousCharge(ctx, fact)
	if err != nil {
		return domain.Settlement{}, err
	}
	delta := charge - previous
	if delta < 0 {
		return domain.Settlement{}, fmt.Errorf("%w: revision %d nets below the current version", ErrSettlementNetReversalUnsupported, fact.Revision)
	}
	settlement := domain.Settlement{
		ID:            domain.SettlementKey(fact.TenantID, fact.CallID, fact.AttemptID, fact.Revision),
		CallID:        fact.CallID,
		ReservationID: reservationKey,
		TenantID:      fact.TenantID,
		Amount:        delta,
		Revision:      fact.Revision,
		OccurredAt:    fact.OccurredAt,
	}
	if err := settlement.Validate(); err != nil {
		return domain.Settlement{}, err
	}
	persist := func(tx *gorm.DB) error {
		return s.persistFinalize(tx, fact, settlement, charge)
	}
	if err := s.budget.SettleReservationHold(ctx, fact.TenantID, reservationKey, delta, persist); err != nil {
		if errors.Is(err, errSettlementReplay) {
			return settlement, nil // already durably settled: idempotent replay
		}
		return domain.Settlement{}, err
	}
	return settlement, nil
}

// persistFinalize runs INSIDE Finalize's single transaction: the fact row,
// the current-version pointer, the settlement record (state dispatched), and
// the settlement outbox event — all atomic with the reservation accounting,
// or all rolled back together.
func (s *SettlementService) persistFinalize(tx *gorm.DB, fact domain.UsageFact, st domain.Settlement, charge domain.Credits) error {
	row := repocommercial.UsageRow{
		ID:             fmt.Sprintf("%d:%s:%s:%d", fact.TenantID, fact.CallID, fact.AttemptID, fact.Revision),
		TenantID:       fact.TenantID,
		CallID:         fact.CallID,
		AttemptID:      fact.AttemptID,
		Revision:       fact.Revision,
		RunID:          fact.RunID,
		DelegationID:   fact.DelegationID,
		Funding:        fact.Funding,
		Service:        fact.Service,
		PriceVersion:   fact.PriceVersion,
		OccurredAt:     fact.OccurredAt,
		DimensionsJSON: "null",
		Status:         fact.Status,
		ChargeMicro:    int64(charge), // the revision's ABSOLUTE charge (U01 semantics); the record carries the delta
	}
	if dims, err := json.Marshal(fact.Dimensions); err == nil {
		row.DimensionsJSON = string(dims)
	}
	var existing repocommercial.UsageRow
	err := tx.Where("tenant_id = ? AND call_id = ? AND attempt_id = ? AND revision = ?",
		row.TenantID, row.CallID, row.AttemptID, row.Revision).First(&existing).Error
	if err == nil {
		if existing.Status != row.Status || existing.ChargeMicro != row.ChargeMicro {
			return repocommercial.ErrUsageRevisionConflict
		}
		// Durably settled before: roll the whole transaction back so the
		// reservation accounting never runs a second time.
		return errSettlementReplay
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if err := tx.Create(&row).Error; err != nil {
		return err
	}
	if err := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "call_id"}, {Name: "attempt_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"revision", "updated_at"}),
	}).Create(&repocommercial.UsageCurrentRow{
		TenantID:  row.TenantID,
		CallID:    row.CallID,
		AttemptID: row.AttemptID,
		Revision:  row.Revision,
		UpdatedAt: s.now(),
	}).Error; err != nil {
		return err
	}
	rec := SettlementRecord{
		Key:            st.ID,
		TenantID:       st.TenantID,
		CallID:         st.CallID,
		AttemptID:      fact.AttemptID,
		ReservationKey: st.ReservationID,
		RunID:          fact.RunID,
		AmountMicro:    int64(st.Amount),
		Revision:       st.Revision,
		State:          domain.SettlementStateDispatched,
		OccurredAt:     st.OccurredAt,
		UpdatedAt:      s.now(),
	}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&rec).Error; err != nil {
		return err
	}
	payload, err := json.Marshal(settlementEventPayload{
		SettlementKey:  st.ID,
		TenantID:       st.TenantID,
		RunID:          fact.RunID,
		CallID:         st.CallID,
		AttemptID:      fact.AttemptID,
		Revision:       st.Revision,
		ChargeMicro:    int64(charge),
		ReservationKey: st.ReservationID,
	})
	if err != nil {
		return fmt.Errorf("%w: %v", repocommercial.ErrInvalidOutboxEvent, err)
	}
	if err := s.outbox.EnqueueTx(tx, repocommercial.OutboxEvent{
		EventKey:    fmt.Sprintf("%s:%d:%s:%s:%d", repocommercial.OutboxKindUsageSettlement, st.TenantID, st.CallID, fact.AttemptID, st.Revision),
		TenantID:    st.TenantID,
		Kind:        repocommercial.OutboxKindUsageSettlement,
		PayloadJSON: string(payload),
	}); err != nil {
		if errors.Is(err, repocommercial.ErrOutboxEventDuplicate) {
			return errSettlementReplay // the revision already settled once
		}
		return err
	}
	return nil
}

// chargeFor prices the fact with its immutable price version's fixed-point
// rates — the single end-of-call rounding point U01 established. Unresolvable
// rates are an error: an unpriced final call is never silently zero-settled.
func (s *SettlementService) chargeFor(fact domain.UsageFact) (domain.Credits, error) {
	rates, err := s.rates(fact.PriceVersion)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", repocommercial.ErrUsageRatesUnavailable, err)
	}
	charge, err := rates.ChargeForCall(fact)
	if err != nil {
		return 0, err
	}
	if charge < 0 {
		return 0, repocommercial.ErrInvalidUsageRow
	}
	return charge, nil
}

// previousCharge reads the charge of the attempt's current version — the
// amount the previous revision already settled and protects.
func (s *SettlementService) previousCharge(ctx context.Context, fact domain.UsageFact) (domain.Credits, error) {
	var cur repocommercial.UsageCurrentRow
	err := s.db.WithContext(ctx).Where("tenant_id = ? AND call_id = ? AND attempt_id = ?",
		fact.TenantID, fact.CallID, fact.AttemptID).First(&cur).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	var prev repocommercial.UsageRow
	err = s.db.WithContext(ctx).Where("tenant_id = ? AND call_id = ? AND attempt_id = ? AND revision = ?",
		fact.TenantID, fact.CallID, fact.AttemptID, cur.Revision).First(&prev).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return domain.Credits(prev.ChargeMicro), nil
}

// Dispatch drains pending settlement outbox events to the gateway. Ingest
// acceptance alone is NOT confirmation: a receipt with only a transaction
// identity moves the record to accepted and keeps the protection; a receipt
// that already carries full confirmation evidence (external id AND watermark)
// goes straight to the confirmed release. Indeterminate outcomes (timeout,
// dropped response) mark the record unknown and LEAVE THE EVENT PENDING, so
// the next pass replays the same idempotency key instead of creating a second
// remote transaction.
func (s *SettlementService) Dispatch(ctx context.Context) error {
	var events []repocommercial.OutboxEvent
	if err := s.db.WithContext(ctx).
		Where("kind = ? AND state = ?", repocommercial.OutboxKindUsageSettlement, repocommercial.OutboxStatePending).
		Find(&events).Error; err != nil {
		return err
	}
	var lastErr error
	for _, ev := range events {
		if err := s.dispatchEvent(ctx, ev); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

func (s *SettlementService) dispatchEvent(ctx context.Context, ev repocommercial.OutboxEvent) error {
	var payload settlementEventPayload
	if err := json.Unmarshal([]byte(ev.PayloadJSON), &payload); err != nil {
		return err
	}
	var rec SettlementRecord
	err := s.db.WithContext(ctx).Where("key = ?", payload.SettlementKey).First(&rec).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil // nothing durable to dispatch for this event
	}
	if err != nil {
		return err
	}
	if rec.State == domain.SettlementStateConfirmed {
		return s.markEventSent(ev.EventKey) // already released; the event's work is done
	}
	st := domain.Settlement{
		ID:            rec.Key,
		CallID:        rec.CallID,
		ReservationID: rec.ReservationKey,
		TenantID:      rec.TenantID,
		Amount:        domain.Credits(rec.AmountMicro),
		Revision:      rec.Revision,
		OccurredAt:    rec.OccurredAt,
	}
	gctx := domain.WithBenefitCustomer(ctx, OrderCustomerID(rec.TenantID))
	receipt, err := s.gateway.Settle(gctx, st)
	if err != nil {
		if errors.Is(err, domain.ErrGatewayIndeterminate) {
			// The remote outcome is unknown: retain for reconciliation —
			// never flip to a terminal state without evidence.
			if uerr := s.setRecordState(rec.Key, domain.SettlementStateUnknown); uerr != nil {
				return uerr
			}
		}
		return err // the event stays pending; the next pass replays the same key
	}
	if receipt.ExternalID == "" {
		// A 2xx without a transaction identity proves nothing about the
		// remote state; treat it as indeterminate, never as success.
		if uerr := s.setRecordState(rec.Key, domain.SettlementStateUnknown); uerr != nil {
			return uerr
		}
		return fmt.Errorf("%w: settlement carried no transaction identity", domain.ErrGatewayIndeterminate)
	}
	if receipt.ConfirmedEvidence() {
		return s.applyConfirmed(ctx, &rec, receipt, ev.EventKey)
	}
	if err := s.db.WithContext(ctx).Model(&SettlementRecord{}).Where("key = ?", rec.Key).Updates(map[string]any{
		"state":       domain.SettlementStateAccepted,
		"external_id": receipt.ExternalID,
		"updated_at":  s.now(),
	}).Error; err != nil {
		return err
	}
	return s.markEventSent(ev.EventKey)
}

// ConfirmSettlement resolves the explicit confirmation of a settled
// transaction. Success requires correlation evidence — external id AND the
// watermark it advanced; seeing the balance drop is never evidence. On
// success the SAME transaction advances the watermark, removes the protection
// covered by it, flips the record to confirmed, and marks its event sent. An
// already-confirmed record replays as success without calling the gateway or
// touching the account, so a lost confirmation response retried later
// advances the watermark exactly once. Stale or failed confirmations retain
// the record (history is never deleted) and return the error.
func (s *SettlementService) ConfirmSettlement(ctx context.Context, settlementID string) (domain.SettlementReceipt, error) {
	var rec SettlementRecord
	err := s.db.WithContext(ctx).Where("key = ?", settlementID).First(&rec).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.SettlementReceipt{}, ErrSettlementNotFound
	}
	if err != nil {
		return domain.SettlementReceipt{}, err
	}
	if rec.State == domain.SettlementStateConfirmed {
		return domain.SettlementReceipt{ExternalID: rec.ExternalID, Watermark: rec.Watermark}, nil
	}
	gctx := domain.WithBenefitCustomer(ctx, OrderCustomerID(rec.TenantID))
	receipt, err := s.gateway.ConfirmSettlement(gctx, settlementID)
	if err != nil {
		return domain.SettlementReceipt{}, err // outcome unknown/failed: retained for reconciliation
	}
	if !receipt.ConfirmedEvidence() {
		return domain.SettlementReceipt{}, fmt.Errorf("%w: confirmation carried no correlation evidence", domain.ErrGatewayIndeterminate)
	}
	if err := s.applyConfirmed(ctx, &rec, receipt, ""); err != nil {
		return domain.SettlementReceipt{}, err
	}
	return receipt, nil
}

// applyConfirmed releases the protection covered by an explicitly confirmed
// settlement in ONE transaction: the store advances the watermark
// monotonically and removes the protected (unreflected) amount together with
// an equal verified drop (releasing protection without dropping verified
// would transiently re-admit spent credits), while the persist closure flips
// the record to confirmed and marks its outbox event sent.
func (s *SettlementService) applyConfirmed(ctx context.Context, rec *SettlementRecord, receipt domain.SettlementReceipt, eventKey string) error {
	persist := func(tx *gorm.DB) error {
		if err := tx.Model(&SettlementRecord{}).Where("key = ?", rec.Key).Updates(map[string]any{
			"state":       domain.SettlementStateConfirmed,
			"external_id": receipt.ExternalID,
			"watermark":   receipt.Watermark,
			"updated_at":  s.now(),
		}).Error; err != nil {
			return err
		}
		if eventKey == "" {
			return nil
		}
		return tx.Model(&repocommercial.OutboxEvent{}).
			Where("event_key = ? AND state = ?", eventKey, repocommercial.OutboxStatePending).
			Update("state", repocommercial.OutboxStateSent).Error
	}
	return s.budget.ApplyConfirmedSettlement(ctx, rec.TenantID, domain.Credits(rec.AmountMicro), receipt.Watermark, persist)
}

func (s *SettlementService) setRecordState(key, state string) error {
	return s.db.Model(&SettlementRecord{}).Where("key = ?", key).Updates(map[string]any{
		"state":      state,
		"updated_at": s.now(),
	}).Error
}

func (s *SettlementService) markEventSent(eventKey string) error {
	return s.db.Model(&repocommercial.OutboxEvent{}).
		Where("event_key = ? AND state = ?", eventKey, repocommercial.OutboxStatePending).
		Update("state", repocommercial.OutboxStateSent).Error
}
