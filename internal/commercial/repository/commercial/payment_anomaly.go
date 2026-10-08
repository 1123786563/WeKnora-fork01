package commercial

// #84: abnormal payment fact retention (spec L127). A refused amount /
// currency mismatch is an EXTERNAL FUND FACT: it stays recorded here as an
// awaiting-disposition anomaly row while the confirmation transaction keeps
// rolling back — the Invoice and the delivered benefits are never touched.
// The public surface is WeKnora's closed vocabulary only (spec L170): kinds
// and states below, never a provider raw status.

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strconv"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Payment anomaly kinds — the closed classification of a retained abnormal
// fact. over_payment is written atomically by ConfirmPayment for a later
// successful collection, never by ClassifyPaymentAnomaly.
const (
	PaymentAnomalyKindPartial  = "partial_payment"   // actual < expected
	PaymentAnomalyKindAmount   = "amount_mismatch"   // actual ≠ expected (single-payment wrong amount)
	PaymentAnomalyKindCurrency = "currency_mismatch" // actual currency ≠ expected
	PaymentAnomalyKindOverPaid = "over_payment"      // multiple-success second payment
)

// Payment anomaly states — awaiting operator disposition until resolved.
const (
	PaymentAnomalyStateAwaiting = "awaiting_disposition"
	PaymentAnomalyStateResolved = "resolved"
)

var (
	// ErrPaymentAnomalyNotFound answers a resolve on an anomaly that does
	// not exist (anymore).
	ErrPaymentAnomalyNotFound = errors.New("payment_anomaly_not_found")
	// ErrPaymentAnomalyVersionConflict answers a resolve whose
	// expected_version no longer matches the stored row (the anomaly
	// changed since the operator read it).
	ErrPaymentAnomalyVersionConflict = errors.New("payment_anomaly_version_conflict")
	// ErrPaymentAnomalyConflict means an idempotency-key collision names a
	// different immutable payment fact; callers must not treat it as replay.
	ErrPaymentAnomalyConflict = errors.New("payment_anomaly_fact_conflict")
)

// PaymentAnomalyRow retains one abnormal payment fact. The unique index
// (provider, merchant, transaction) makes retention idempotent: a channel
// redelivery or an outbox replay never mints a second row.
type PaymentAnomalyRow struct {
	ID                string     `gorm:"primaryKey;column:id"` // "anom_"+random hex (generated on insert when empty)
	TenantID          uint64     `gorm:"column:tenant_id;not null"`
	OrderID           string     `gorm:"column:order_id;not null;index"`
	AttemptID         string     `gorm:"column:attempt_id;not null"` // merchant_order_id
	Provider          string     `gorm:"column:provider;not null;uniqueIndex:uq_payment_anomaly_txn,priority:1"`
	Merchant          string     `gorm:"column:merchant;not null;uniqueIndex:uq_payment_anomaly_txn,priority:2"`
	Transaction       string     `gorm:"column:transaction;not null;uniqueIndex:uq_payment_anomaly_txn,priority:3"`
	Kind              string     `gorm:"column:kind;not null"`
	ExpectedAmountFen int64      `gorm:"column:expected_amount_fen;not null"`
	ActualAmountFen   int64      `gorm:"column:actual_amount_fen;not null"`
	ExpectedCurrency  string     `gorm:"column:expected_currency;not null"`
	ActualCurrency    string     `gorm:"column:actual_currency;not null"`
	State             string     `gorm:"column:state;not null;default:'awaiting_disposition'"`
	Version           int64      `gorm:"column:version;not null;default:1"`
	CreatedAt         time.Time  `gorm:"column:created_at"`
	ResolvedAt        *time.Time `gorm:"column:resolved_at"`
}

func (PaymentAnomalyRow) TableName() string { return "commercial_payment_anomalies" }

// ClassifyPaymentAnomaly is the closed classification of a single-payment
// mismatch: currency first (a wrong currency is never an amount judgement),
// then partial (actual < expected), everything else is a wrong amount
// (including a single over-payment of the expected face). over_payment —
// the later successful collection — is written atomically with its outbox event
// by ConfirmPayment; its consumer validates the saved immutable fact.
func ClassifyPaymentAnomaly(expectedFen, actualFen int64, expectedCurrency, actualCurrency string) string {
	if expectedCurrency != actualCurrency {
		return PaymentAnomalyKindCurrency
	}
	if actualFen < expectedFen {
		return PaymentAnomalyKindPartial
	}
	return PaymentAnomalyKindAmount
}

// newAnomalyToken mints the random hex suffix of an anomaly id when a row is
// first inserted. A rolled-back transaction also rolls back this private ID;
// a later retry gets a fresh one.
func newAnomalyToken() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	return hex.EncodeToString(b[:])
}

// RecordPaymentAnomaly idempotently retains one abnormal fact in its own
// transaction, requiring an exact immutable-fact match on replay. The row's
// ID and defaults are filled here so callers pass the bare snapshot.
func (s *OrderStore) RecordPaymentAnomaly(ctx context.Context, row PaymentAnomalyRow) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return recordPaymentAnomalyTx(tx, row)
	})
}

// recordPaymentAnomalyTx idempotently inserts a payment fact using its
// provider/merchant/transaction identity. A uniqueness collision is only a
// replay when every immutable fact field matches the already stored row.
// Keep this helper transaction-local so ConfirmPayment can commit the
// anomaly and its outbox event atomically.
func recordPaymentAnomalyTx(tx *gorm.DB, row PaymentAnomalyRow) error {
	if row.ID == "" {
		row.ID = "anom_" + newAnomalyToken()
	}
	if row.State == "" {
		row.State = PaymentAnomalyStateAwaiting
	}
	if row.CreatedAt.IsZero() {
		row.CreatedAt = time.Now().UTC()
	}
	res := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 1 {
		return nil
	}
	var existing PaymentAnomalyRow
	if err := tx.Where("provider = ? AND merchant = ? AND `transaction` = ?", row.Provider, row.Merchant, row.Transaction).
		First(&existing).Error; err != nil {
		return err
	}
	if !samePaymentAnomalyFact(existing, row) {
		return ErrPaymentAnomalyConflict
	}
	return nil
}

func samePaymentAnomalyFact(a, b PaymentAnomalyRow) bool {
	return a.TenantID == b.TenantID && a.OrderID == b.OrderID && a.AttemptID == b.AttemptID &&
		a.Provider == b.Provider && a.Merchant == b.Merchant && a.Transaction == b.Transaction && a.Kind == b.Kind &&
		a.ExpectedAmountFen == b.ExpectedAmountFen && a.ActualAmountFen == b.ActualAmountFen &&
		a.ExpectedCurrency == b.ExpectedCurrency && a.ActualCurrency == b.ActualCurrency
}

// HasUnresolvedPaymentAnomaly reports whether an order still carries an
// awaiting-disposition anomaly — the attention signal of the order read
// paths. Parameter-bound.
func (s *OrderStore) HasUnresolvedPaymentAnomaly(ctx context.Context, orderID string) (bool, error) {
	if orderID == "" {
		return false, nil
	}
	var n int64
	if err := s.db.WithContext(ctx).Model(&PaymentAnomalyRow{}).
		Where("order_id = ? AND state = ?", orderID, PaymentAnomalyStateAwaiting).
		Count(&n).Error; err != nil {
		return false, err
	}
	return n > 0, nil
}

// ListAwaitingPaymentAnomalyOrderIDs returns the distinct order IDs with an unresolved anomaly for one tenant.
// It deliberately accepts no order ID slice, keeping the lookup to one bounded tenant/state query.
func (s *OrderStore) ListAwaitingPaymentAnomalyOrderIDs(ctx context.Context, tenantID uint64) (map[string]struct{}, error) {
	ids := make(map[string]struct{})
	if tenantID == 0 {
		return ids, nil
	}
	var orderIDs []string
	err := s.db.WithContext(ctx).Model(&PaymentAnomalyRow{}).Distinct("order_id").
		Where("tenant_id = ? AND state = ?", tenantID, PaymentAnomalyStateAwaiting).Pluck("order_id", &orderIDs).Error
	if err != nil {
		return nil, err
	}
	for _, id := range orderIDs {
		ids[id] = struct{}{}
	}
	return ids, nil
}

// ListPaymentAnomalies answers the admin disposition surface: every
// retained fact, newest first. Parameter-bound; no caller input.
func (s *OrderStore) ListPaymentAnomalies(ctx context.Context) ([]PaymentAnomalyRow, error) {
	rows := make([]PaymentAnomalyRow, 0)
	if err := s.db.WithContext(ctx).Order("created_at DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ResolvePaymentAnomaly flips one awaiting anomaly to resolved under the
// state+version guard: expectedVersion must match the stored row, so two
// operators cannot resolve past each other. A missing row answers
// ErrPaymentAnomalyNotFound; a stale version answers
// ErrPaymentAnomalyVersionConflict. The retained fund fact itself is never
// rewritten. Parameter-bound throughout.
func (s *OrderStore) ResolvePaymentAnomaly(ctx context.Context, id string, expectedVersion int64) (PaymentAnomalyRow, error) {
	if id == "" {
		return PaymentAnomalyRow{}, ErrPaymentAnomalyNotFound
	}
	now := time.Now().UTC()
	res := s.db.WithContext(ctx).Model(&PaymentAnomalyRow{}).
		Where("id = ? AND state = ? AND version = ?", id, PaymentAnomalyStateAwaiting, expectedVersion).
		Updates(map[string]interface{}{
			"state":       PaymentAnomalyStateResolved,
			"version":     gorm.Expr("version + 1"),
			"resolved_at": now,
		})
	if res.Error != nil {
		return PaymentAnomalyRow{}, res.Error
	}
	if res.RowsAffected == 0 {
		var row PaymentAnomalyRow
		err := s.db.WithContext(ctx).Where("id = ?", id).First(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return PaymentAnomalyRow{}, ErrPaymentAnomalyNotFound
		}
		if err != nil {
			return PaymentAnomalyRow{}, err
		}
		return PaymentAnomalyRow{}, ErrPaymentAnomalyVersionConflict
	}
	var row PaymentAnomalyRow
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&row).Error; err != nil {
		return PaymentAnomalyRow{}, err
	}
	return row, nil
}
