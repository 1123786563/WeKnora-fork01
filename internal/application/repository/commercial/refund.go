package commercial

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	domain "github.com/Tencent/WeKnora/internal/commercial"
	"gorm.io/gorm"
)

var (
	ErrInvalidRefundRow          = errors.New("invalid_refund_row")
	ErrRefundNotFound            = errors.New("refund_not_found")
	ErrInvalidRefundState        = errors.New("invalid_refund_state")
	ErrRefundLotsMissing         = errors.New("refund_lots_missing")
	ErrRefundInsufficientCredits = errors.New("refund_insufficient_credits")
)

// Outbox event kind emitted when a refund is approved into the channel
// payout stage. The payout worker drains these events; the event moves to
// sent once the refund reaches a post-payout state.
const OutboxKindRefundPayout = "refund_payout"

// RefundRow stores one refund request against an original order line
// (order_line_id, the fulfillment lot identity), its exact amount and
// credit amount, the reviewer, the channel settlement identity
// (provider_refund_id UNIQUE — one channel refund can never be recorded
// twice), the state machine state, and an optimistic version. review_basis
// records the manual approval basis (period not started vs already
// effective); review_note records why a review stopped (e.g. P03's
// occupancy check unavailable).
type RefundRow struct {
	ID               string  `gorm:"primaryKey;column:id"`
	TenantID         uint64  `gorm:"column:tenant_id;not null"`
	OrderID          string  `gorm:"column:order_id;not null;index"`
	OrderLineID      string  `gorm:"column:order_line_id;not null"`
	AmountFen        int64   `gorm:"column:amount_fen;not null"`
	CreditsMicro     int64   `gorm:"column:credits_micro;not null"`
	Reviewer         string  `gorm:"column:reviewer;not null;default:''"`
	ProviderRefundID *string `gorm:"column:provider_refund_id;uniqueIndex"`
	State            domain.RefundState `gorm:"column:state;not null"`
	Version          int64   `gorm:"column:version;not null;default:1"`
	ChannelAttempts  int64   `gorm:"column:channel_attempts;not null;default:0"`
	ReviewBasis      string  `gorm:"column:review_basis;not null;default:''"`
	ReviewNote       string  `gorm:"column:review_note;not null;default:''"`
}

func (RefundRow) TableName() string { return "commercial_refunds" }

// Domain projects the persisted row onto the produced refund state.
func (r RefundRow) Domain() domain.RefundRequestState {
	return domain.RefundRequestState{
		ID:           r.ID,
		OrderID:      r.OrderID,
		State:        r.State,
		TenantID:     r.TenantID,
		Amount:       domain.CNYFen(r.AmountFen),
		CreditAmount: domain.Credits(r.CreditsMicro),
	}
}

// RefundAllocationRow is one lot lock held by one refund (or, from P03 on,
// by one usage admission through the same coordinator). Active locks live
// here; release deletes the row, so the table always reflects held locks.
type RefundAllocationRow struct {
	ID          string `gorm:"primaryKey;column:id"`
	RefundID    string `gorm:"column:refund_id;not null;index"`
	TenantID    uint64 `gorm:"column:tenant_id;not null"`
	LotID       string `gorm:"column:lot_id;not null;index"`
	LockedMicro int64  `gorm:"column:locked_micro;not null"`
}

func (RefundAllocationRow) TableName() string { return "commercial_refund_allocations" }

type RefundStore struct{ db *gorm.DB }

func NewRefundStore(db *gorm.DB) *RefundStore { return &RefundStore{db: db} }

type refundLotRow struct {
	Key     string
	Credits int64
}

type refundPayoutPayload struct {
	RefundID     string `json:"refund_id"`
	TenantID     uint64 `json:"tenant_id"`
	OrderID      string `json:"order_id"`
	OrderLineID  string `json:"order_line_id"`
	AmountFen    int64  `json:"amount_fen"`
	CreditsMicro int64  `json:"credits_micro"`
}

// CreateRefundRequest registers a refund request in the requested state.
// It writes NO locks and NO channel call: locking happens only inside the
// ApproveRefund admission transaction.
func (s *RefundStore) CreateRefundRequest(ctx context.Context, row RefundRow) error {
	if row.ID == "" || row.TenantID == 0 || row.OrderID == "" || row.OrderLineID == "" ||
		row.AmountFen <= 0 || row.CreditsMicro <= 0 {
		return ErrInvalidRefundRow
	}
	if row.State != "" && row.State != domain.RefundStateRequested {
		return ErrInvalidRefundRow
	}
	if row.ProviderRefundID != nil {
		return ErrInvalidRefundRow
	}
	row.State = domain.RefundStateRequested
	if row.Version == 0 {
		row.Version = 1
	}
	return s.db.WithContext(ctx).Create(&row).Error
}

// GetRefund returns a refund row by ID.
func (s *RefundStore) GetRefund(ctx context.Context, id string) (RefundRow, error) {
	var row RefundRow
	err := s.db.WithContext(ctx).Where("id = ?", id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return RefundRow{}, ErrRefundNotFound
	}
	return row, err
}

// ListRefundAllocations returns the locks currently held by one refund.
func (s *RefundStore) ListRefundAllocations(ctx context.Context, refundID string) ([]RefundAllocationRow, error) {
	var rows []RefundAllocationRow
	err := s.db.WithContext(ctx).Where("refund_id = ?", refundID).Find(&rows).Error
	return rows, err
}

// lockedMicro sums the locks currently held against one lot. Because
// release deletes allocation rows, this sum is exactly the locked amount.
func lockedMicro(tx *gorm.DB, lotID string) (int64, error) {
	var locked int64
	err := tx.Table("commercial_refund_allocations").
		Select("COALESCE(SUM(locked_micro), 0)").Where("lot_id = ?", lotID).Scan(&locked).Error
	return locked, err
}

// admitLotLocksTx is the SINGLE admission-transaction coordinator seam:
// every lock admission — refund approvals here, usage consumption from P03
// — must pass through this one transactional check-and-write, never a
// standalone read-balance-then-write. For each wanted lot it re-reads the
// lot total and the currently held locks INSIDE the transaction and
// refuses any admission that would over-lock the lot.
func admitLotLocksTx(tx *gorm.DB, prefix string, tenantID uint64, wants map[string]int64) error {
	for lotID, micro := range wants {
		if micro <= 0 {
			return ErrInvalidRefundRow
		}
		var lot refundLotRow
		if err := tx.Table("commercial_fulfillment_records").
			Select("key, credits").Where("key = ?", lotID).Scan(&lot).Error; err != nil {
			return err
		}
		if lot.Key == "" || lot.Credits <= 0 {
			return ErrRefundLotsMissing
		}
		locked, err := lockedMicro(tx, lotID)
		if err != nil {
			return err
		}
		if locked+micro > lot.Credits {
			// Refusing here — inside the same transaction that would have
			// written the lock — is what makes concurrent partial refunds
			// and usage races unable to double-spend locked credits.
			return ErrRefundInsufficientCredits
		}
		if err := tx.Create(&RefundAllocationRow{
			ID: prefix + ":" + lotID, RefundID: prefix, TenantID: tenantID,
			LotID: lotID, LockedMicro: micro,
		}).Error; err != nil {
			return err
		}
	}
	return nil
}

// AdmitLotLock is the P03-facing entry of the same admission coordinator:
// usage consumption reserves credits against lots through the identical
// transactional check-and-write as refund approvals.
func (s *RefundStore) AdmitLotLock(ctx context.Context, tenantID uint64, ref string, wants map[string]int64) error {
	if ref == "" || len(wants) == 0 {
		return ErrInvalidRefundRow
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return admitLotLocksTx(tx, "usage:"+ref, tenantID, wants)
	})
}

// ApproveRefund is the approval admission transaction. In ONE database
// transaction it: re-reads the refund, records the manual review basis,
// checks the eligibility seam (P03 occupancy-refundable — until P03 lands
// the default refuses and the refund stays requested/reviewing with a
// durably recorded reason, NEVER approved into a paid-out state), walks
// the order's lots, verifies used / in-flight / prior refunds through the
// per-lot lock sums, writes the locks, moves the refund to pending, and
// emits the payout outbox event. A failure at any point rolls back — no
// locks survive a refused approval.
func (s *RefundStore) ApproveRefund(ctx context.Context, refundID, reviewer string, eligibility domain.RefundEligibilityChecker, basis string) error {
	if eligibility == nil {
		eligibility = domain.DefaultRefundEligibility
	}
	var pre RefundRow
	if err := s.db.WithContext(ctx).Where("id = ?", refundID).First(&pre).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrRefundNotFound
		}
		return err
	}
	if pre.State != domain.RefundStateRequested && pre.State != domain.RefundStateReviewing {
		return ErrInvalidRefundState
	}
	if err := eligibility.RefundEligible(ctx, pre.TenantID, pre.OrderID, domain.Credits(pre.CreditsMicro)); err != nil {
		// No configured policy → no auto-approve. Keep the refund in
		// requested/reviewing and durably record WHY the review stopped.
		// The version-guarded UPDATE is atomic on its own and MUST run
		// outside the refused admission transaction — returning the
		// refusal inside it would roll the review record back.
		res := s.db.WithContext(ctx).Model(&RefundRow{}).
			Where("id = ? AND state IN ? AND version = ?", pre.ID,
				[]domain.RefundState{domain.RefundStateRequested, domain.RefundStateReviewing}, pre.Version).
			Updates(map[string]interface{}{
				"state":        domain.RefundStateReviewing,
				"reviewer":     reviewer,
				"review_basis": basis,
				"review_note":  err.Error(),
				"version":      pre.Version + 1,
			})
		if res.Error != nil {
			return res.Error
		}
		return fmt.Errorf("%w: %v", domain.ErrRefundNotReady, err)
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rf RefundRow
		if err := tx.Where("id = ?", refundID).First(&rf).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrRefundNotFound
			}
			return err
		}
		if rf.State != domain.RefundStateRequested && rf.State != domain.RefundStateReviewing {
			return ErrInvalidRefundState
		}
		if err := eligibility.RefundEligible(ctx, rf.TenantID, rf.OrderID, domain.Credits(rf.CreditsMicro)); err != nil {
			// No configured policy → no auto-approve. Keep the refund in
			// requested/reviewing and record WHY the review stopped.
			res := tx.Model(&RefundRow{}).
				Where("id = ? AND state IN ? AND version = ?", rf.ID,
					[]domain.RefundState{domain.RefundStateRequested, domain.RefundStateReviewing}, rf.Version).
				Updates(map[string]interface{}{
					"state":        domain.RefundStateReviewing,
					"reviewer":     reviewer,
					"review_basis": basis,
					"review_note":  err.Error(),
					"version":      rf.Version + 1,
				})
			if res.Error != nil {
				return res.Error
			}
			return fmt.Errorf("%w: %v", domain.ErrRefundNotReady, err)
		}

		var lots []refundLotRow
		if err := tx.Table("commercial_fulfillment_records").
			Select("key, credits").Where("order_id = ?", rf.OrderID).Order("key").Find(&lots).Error; err != nil {
			return err
		}
		if len(lots) == 0 {
			return ErrRefundLotsMissing
		}
		want := rf.CreditsMicro
		wants := map[string]int64{}
		for _, lot := range lots {
			if want <= 0 {
				break
			}
			locked, err := lockedMicro(tx, lot.Key)
			if err != nil {
				return err
			}
			avail := lot.Credits - locked
			take := want
			if take > avail {
				take = avail
			}
			if take > 0 {
				wants[lot.Key] = take
				want -= take
			}
		}
		if want > 0 {
			// Used, in-flight, and prior refunds already consume the lots:
			// refuse the whole approval without writing any lock.
			return ErrRefundInsufficientCredits
		}
		if err := admitLotLocksTx(tx, rf.ID, rf.TenantID, wants); err != nil {
			return err
		}
		res := tx.Model(&RefundRow{}).
			Where("id = ? AND state IN ? AND version = ?", rf.ID,
				[]domain.RefundState{domain.RefundStateRequested, domain.RefundStateReviewing}, rf.Version).
			Updates(map[string]interface{}{
				"state":        domain.RefundStatePending,
				"reviewer":     reviewer,
				"review_basis": basis,
				"review_note":  "",
				"version":      rf.Version + 1,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrInvalidRefundState
		}
		payload, err := json.Marshal(refundPayoutPayload{
			RefundID: rf.ID, TenantID: rf.TenantID, OrderID: rf.OrderID,
			OrderLineID: rf.OrderLineID, AmountFen: rf.AmountFen, CreditsMicro: rf.CreditsMicro,
		})
		if err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidOutboxEvent, err)
		}
		return insertOutboxEvent(tx, OutboxEvent{
			EventKey:    OutboxKindRefundPayout + ":" + rf.ID,
			TenantID:    rf.TenantID,
			Kind:        OutboxKindRefundPayout,
			PayloadJSON: string(payload),
		})
	})
}

// MarkRefundChannelResult applies one confirmed channel outcome to the
// refund under a guarded version-checked update, records the channel
// settlement identity (provider_refund_id UNIQUE), and releases the locks
// if — and only if — the outcome is confirmed failed or confirmed
// not-created. The payout outbox event moves to sent once the refund
// leaves the payout stage.
func (s *RefundStore) MarkRefundChannelResult(ctx context.Context, refundID string, providerRefundID *string, channelState domain.RefundChannelState) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rf RefundRow
		if err := tx.Where("id = ?", refundID).First(&rf).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrRefundNotFound
			}
			return err
		}
		next := domain.TransitionRefundOnChannel(rf.State, channelState)
		if next == rf.State && providerRefundID == nil {
			return nil // still in flight: nothing to record
		}
		updates := map[string]interface{}{
			"state":            next,
			"version":          rf.Version + 1,
			"channel_attempts": gorm.Expr("channel_attempts + 1"),
		}
		if providerRefundID != nil && *providerRefundID != "" {
			updates["provider_refund_id"] = *providerRefundID
		}
		res := tx.Model(&RefundRow{}).
			Where("id = ? AND state = ? AND version = ?", rf.ID, rf.State, rf.Version).
			Updates(updates)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrInvalidRefundState
		}
		// Locks release ONLY on a confirmed failed / never-created LIFECYCLE
		// state — this was previously passed through the channel-state
		// helper, conflating the two vocabularies; the typed states now
		// make that mixing a compile error.
		if next == domain.RefundStateFailedConfirmed || next == domain.RefundStateNotCreatedConfirmed {
			if err := tx.Where("refund_id = ?", rf.ID).Delete(&RefundAllocationRow{}).Error; err != nil {
				return err
			}
		}
		if next != domain.RefundStatePending {
			return tx.Table("commercial_outbox_events").
				Where("event_key = ?", OutboxKindRefundPayout+":"+rf.ID).
				Update("state", OutboxStateSent).Error
		}
		return nil
	})
}

// CompleteRevocation finishes a revocation_pending refund only on a
// confirmed precise-credits revocation; a failed confirmation leaves the
// state (and the locks) untouched so recovery retries the revocation.
func (s *RefundStore) CompleteRevocation(ctx context.Context, refundID string, confirmed bool) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rf RefundRow
		if err := tx.Where("id = ?", refundID).First(&rf).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrRefundNotFound
			}
			return err
		}
		next := domain.TransitionRefundOnRevocation(rf.State, confirmed)
		if next == rf.State {
			return nil
		}
		res := tx.Model(&RefundRow{}).
			Where("id = ? AND state = ? AND version = ?", rf.ID, rf.State, rf.Version).
			Updates(map[string]interface{}{"state": next, "version": rf.Version + 1})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrInvalidRefundState
		}
		if next == domain.RefundStateCompleted {
			// Revocation confirmed for the precise locked amounts: the locks
			// are settled, not merely released.
			return tx.Where("refund_id = ?", rf.ID).Delete(&RefundAllocationRow{}).Error
		}
		return nil
	})
}
