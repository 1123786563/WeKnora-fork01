package commercial

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	domain "github.com/Tencent/WeKnora/internal/commercial"
	"gorm.io/gorm"
)

var (
	ErrInvalidOrderRow        = errors.New("invalid_order_row")
	ErrOrderNotFound          = errors.New("order_not_found")
	ErrInvalidOrderState      = errors.New("invalid_order_state")
	ErrInvalidPaymentAttempt  = errors.New("invalid_payment_attempt")
	ErrPaymentAttemptNotFound = errors.New("payment_attempt_not_found")
)

// Payment attempt lifecycle states: an attempt is registered pending and
// becomes succeeded once its provider transaction is confirmed.
const (
	PaymentAttemptStatePending   = "pending"
	PaymentAttemptStateSucceeded = "succeeded"
)

// Outbox event kinds emitted by payment confirmation.
const (
	OutboxKindFulfill   = "fulfill"
	OutboxKindOverPaid  = "over_payment"
	OutboxOverPaidState = "over_paid"
)

// OrderRow stores a commercial order. quote_id UNIQUE makes a quote
// consumable by exactly one order; version guards the pending→paid
// transition so exactly one confirmation wins.
type OrderRow struct {
	ID        string `gorm:"primaryKey;column:id"`
	TenantID  uint64 `gorm:"column:tenant_id;not null"`
	QuoteID   string `gorm:"column:quote_id;uniqueIndex;not null"`
	AmountFen int64  `gorm:"column:amount_fen;not null"`
	Currency  string `gorm:"column:currency;not null"`
	State     string `gorm:"column:state;not null"`
	Version   int64  `gorm:"column:version;not null;default:1"`
}

func (OrderRow) TableName() string { return "commercial_orders" }

// Domain projects the persisted row onto the commercial domain type.
func (r OrderRow) Domain() domain.Order {
	return domain.Order{
		ID:       r.ID,
		TenantID: r.TenantID,
		Amount:   domain.CNYFen(r.AmountFen),
		Currency: r.Currency,
		State:    r.State,
		Version:  r.Version,
	}
}

// PaymentAttemptRow is the registered merchant-side attempt for one order.
// (provider, merchant, merchant_order_id) is unique, and once a provider
// transaction lands, (provider, merchant, provider_transaction_id) is unique
// too, so the same channel transaction can never confirm twice as different
// attempts. provider_transaction_id stays NULL until confirmation, letting
// several pending attempts coexist.
type PaymentAttemptRow struct {
	ID                    string  `gorm:"primaryKey;column:id"`
	TenantID              uint64  `gorm:"column:tenant_id;not null"`
	OrderID               string  `gorm:"column:order_id;not null"`
	Provider              string  `gorm:"column:provider;not null"`
	Merchant              string  `gorm:"column:merchant;not null"`
	MerchantOrderID       string  `gorm:"column:merchant_order_id;not null"`
	ProviderTransactionID *string `gorm:"column:provider_transaction_id"`
	AmountFen             int64   `gorm:"column:amount_fen;not null"`
	Currency              string  `gorm:"column:currency;not null"`
	State                 string  `gorm:"column:state;not null"`
}

func (PaymentAttemptRow) TableName() string { return "commercial_payment_attempts" }

type OrderStore struct{ db *gorm.DB }

func NewOrderStore(db *gorm.DB) *OrderStore { return &OrderStore{db: db} }

// CreateOrder registers a pending order. The quote_id unique index guarantees
// a quote is consumable by exactly one order, concurrent creators included.
func (s *OrderStore) CreateOrder(ctx context.Context, row OrderRow) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return createOrderTx(tx, row)
	})
}

func createOrderTx(tx *gorm.DB, row OrderRow) error {
	if row.ID == "" || row.TenantID == 0 || row.QuoteID == "" || row.AmountFen <= 0 || row.Currency == "" {
		return ErrInvalidOrderRow
	}
	if row.State != "" && row.State != domain.OrderStatePending {
		return ErrInvalidOrderRow
	}
	row.State = domain.OrderStatePending
	if row.Version == 0 {
		row.Version = 1
	}
	var existing OrderRow
	err := tx.Where("quote_id = ?", row.QuoteID).First(&existing).Error
	if err == nil {
		return ErrQuoteAlreadyUsed
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return tx.Create(&row).Error
}

// OpenOrder is the atomic unit of work behind a checkout (and an upgrade):
// it creates the pending order, CONSUMES the quote and registers the pending
// attempt in ONE transaction. Any failure — stale quote, expiry, version
// conflict, unique violation — rolls the whole unit back: no orphan order
// row survives a refused checkout and no compensation delete is needed.
func (s *OrderStore) OpenOrder(ctx context.Context, order OrderRow, attempt PaymentAttemptRow, quoteID string, subscriptionVersion int64, now time.Time) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := createOrderTx(tx, order); err != nil {
			return err
		}
		if _, err := consumeQuoteTx(tx, quoteID, subscriptionVersion, order.ID, now); err != nil {
			return err
		}
		return registerAttemptTx(tx, attempt)
	})
}

// ListOrdersByTenant returns a space's orders, newest first. The tenant
// comes exclusively from the authenticated context; the query never
// accepts a tenant from the caller's payload.
func (s *OrderStore) ListOrdersByTenant(ctx context.Context, tenantID uint64) ([]OrderRow, error) {
	if tenantID == 0 {
		return nil, ErrOrderNotFound
	}
	var rows []OrderRow
	if err := s.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).Order("id DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// FirstPendingAttempt returns the oldest still-pending attempt of an order —
// the identifier payment recovery re-queries the channel with. An order with
// no pending attempt reports ErrPaymentAttemptNotFound.
func (s *OrderStore) FirstPendingAttempt(ctx context.Context, orderID string) (PaymentAttemptRow, error) {
	var att PaymentAttemptRow
	err := s.db.WithContext(ctx).
		Where("order_id = ? AND state = ?", orderID, PaymentAttemptStatePending).
		Order("id").First(&att).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return PaymentAttemptRow{}, ErrPaymentAttemptNotFound
	}
	return att, err
}

// GetOrder returns an order by ID; readers distinguish paid from fulfilled.
func (s *OrderStore) GetOrder(ctx context.Context, id string) (OrderRow, error) {
	var row OrderRow
	err := s.db.WithContext(ctx).Where("id = ?", id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return OrderRow{}, ErrOrderNotFound
	}
	return row, err
}

// RegisterAttempt records the merchant-side attempt before any payment
// result arrives; ConfirmPayment only accepts facts whose merchant and
// attempt were registered this way.
func (s *OrderStore) RegisterAttempt(ctx context.Context, row PaymentAttemptRow) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return registerAttemptTx(tx, row)
	})
}

func registerAttemptTx(tx *gorm.DB, row PaymentAttemptRow) error {
	if row.ID == "" || row.TenantID == 0 || row.OrderID == "" || row.Provider == "" ||
		row.Merchant == "" || row.MerchantOrderID == "" || row.AmountFen <= 0 || row.Currency == "" {
		return ErrInvalidPaymentAttempt
	}
	if row.State != "" && row.State != PaymentAttemptStatePending {
		return ErrInvalidPaymentAttempt
	}
	if row.ProviderTransactionID != nil {
		return ErrInvalidPaymentAttempt
	}
	row.State = PaymentAttemptStatePending
	return tx.Create(&row).Error
}

type paymentEventPayload struct {
	OrderID     string `json:"order_id"`
	QuoteID     string `json:"quote_id"`
	TenantID    uint64 `json:"tenant_id"`
	AttemptID   string `json:"attempt_id"`
	Provider    string `json:"provider"`
	Merchant    string `json:"merchant"`
	Transaction string `json:"transaction"`
	AmountFen   int64  `json:"amount_fen"`
	Currency    string `json:"currency"`
	Reason      string `json:"reason,omitempty"`
}

func (p paymentEventPayload) toJSON() (string, error) {
	b, err := json.Marshal(p)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidOutboxEvent, err)
	}
	return string(b), nil
}

// ConfirmPayment records a provider payment result in ONE transaction: it
// validates the registered merchant and attempt, checks the order version,
// saves the payment fact, marks the order paid, and inserts the unique
// fulfillment outbox event. A replayed same-channel transaction returns the
// original result with no second event. A different attempt succeeding after
// the order is already paid only records an over-payment audit event; the
// fulfillment event is never duplicated. A failure at any point — including
// between the payment confirmation and the outbox write — rolls back the
// whole transaction.
func (s *OrderStore) ConfirmPayment(ctx context.Context, fact domain.PaymentFact) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var attempt PaymentAttemptRow
		err := tx.Where("provider = ? AND merchant = ? AND merchant_order_id = ?",
			fact.Provider, fact.Merchant, fact.AttemptID).First(&attempt).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrPaymentAttemptNotFound
		}
		if err != nil {
			return err
		}
		if attempt.OrderID != fact.OrderID || attempt.TenantID != fact.TenantID ||
			attempt.AmountFen != int64(fact.Amount) || attempt.Currency != fact.Currency {
			return domain.ErrPaymentMismatch
		}

		var row OrderRow
		if err := tx.Where("id = ?", fact.OrderID).First(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrOrderNotFound
			}
			return err
		}
		if err := domain.ValidatePayment(row.Domain(), fact); err != nil {
			return err
		}

		// Save the payment fact; a replay of the same provider transaction is
		// detected instead of being written a second time.
		sameTxn := attempt.State == PaymentAttemptStateSucceeded &&
			attempt.ProviderTransactionID != nil && *attempt.ProviderTransactionID == fact.Transaction
		if !sameTxn {
			txn := fact.Transaction
			if err := tx.Model(&PaymentAttemptRow{}).Where("id = ?", attempt.ID).
				Updates(map[string]interface{}{
					"state":                   PaymentAttemptStateSucceeded,
					"provider_transaction_id": txn,
				}).Error; err != nil {
				return err
			}
		}

		// Guarded pending→paid transition: exactly one confirmation wins the
		// order version and therefore the single fulfillment right.
		res := tx.Model(&OrderRow{}).
			Where("id = ? AND state = ? AND version = ?", row.ID, domain.OrderStatePending, row.Version).
			Updates(map[string]interface{}{
				"state":   domain.OrderStatePaid,
				"version": row.Version + 1,
			})
		if res.Error != nil {
			return res.Error
		}
		payload := paymentEventPayload{
			OrderID:     row.ID,
			QuoteID:     row.QuoteID,
			TenantID:    row.TenantID,
			AttemptID:   attempt.ID,
			Provider:    fact.Provider,
			Merchant:    fact.Merchant,
			Transaction: fact.Transaction,
			AmountFen:   int64(fact.Amount),
			Currency:    fact.Currency,
		}
		if res.RowsAffected == 1 {
			payloadJSON, err := payload.toJSON()
			if err != nil {
				return err
			}
			return insertOutboxEvent(tx, OutboxEvent{
				EventKey:    OutboxKindFulfill + ":" + row.ID,
				TenantID:    row.TenantID,
				Kind:        OutboxKindFulfill,
				PayloadJSON: payloadJSON,
			})
		}

		// The order is already paid or fulfilled by another channel. A replay
		// of the winning transaction returns the original success silently;
		// any other late success only appends an over-payment audit event —
		// a duplicate audit key means this replay was already recorded.
		if sameTxn {
			return nil
		}
		payload.Reason = OutboxOverPaidState
		payloadJSON, err := payload.toJSON()
		if err != nil {
			return err
		}
		err = insertOutboxEvent(tx, OutboxEvent{
			EventKey:    fmt.Sprintf("%s:%s:%s:%s:%s", OutboxKindOverPaid, row.ID, fact.Provider, fact.Merchant, fact.Transaction),
			TenantID:    row.TenantID,
			Kind:        OutboxKindOverPaid,
			PayloadJSON: payloadJSON,
		})
		if errors.Is(err, ErrOutboxEventDuplicate) {
			return nil
		}
		return err
	})
}

// MarkFulfilled transitions a paid order to fulfilled once its benefit event
// has been delivered; it is idempotent for an already-fulfilled order.
func (s *OrderStore) MarkFulfilled(ctx context.Context, id string) error {
	res := s.db.WithContext(ctx).Model(&OrderRow{}).
		Where("id = ? AND state = ?", id, domain.OrderStatePaid).
		Updates(map[string]interface{}{
			"state":   domain.OrderStateFulfilled,
			"version": gorm.Expr("version + 1"),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 1 {
		return nil
	}
	var row OrderRow
	err := s.db.WithContext(ctx).Where("id = ?", id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrOrderNotFound
	}
	if err != nil {
		return err
	}
	if row.State == domain.OrderStateFulfilled {
		return nil
	}
	return ErrInvalidOrderState
}
