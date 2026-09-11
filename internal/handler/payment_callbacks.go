package handler

import (
	"errors"
	"io"
	"net/http"

	repository "github.com/Tencent/WeKnora/internal/application/repository/commercial"
	domain "github.com/Tencent/WeKnora/internal/commercial"
	"github.com/Tencent/WeKnora/internal/payment"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// PaymentCallbacksHandler receives asynchronous provider payment
// notifications. Callbacks are publicly reachable: authenticity comes from
// the provider's signature over the raw body, never from a session. The
// payload NEVER carries tenant identity — the trusted tenant and order are
// rebuilt server-side from the registered payment attempt.
type PaymentCallbacksHandler struct {
	db        *gorm.DB
	orders    *repository.OrderStore
	providers map[string]payment.Provider
}

// NewPaymentCallbacksHandler wires the C01 order store with the configured
// providers. A nil db or nil map is legal and fails closed: every
// notification is rejected and nothing is persisted.
func NewPaymentCallbacksHandler(db *gorm.DB, providers map[string]payment.Provider) *PaymentCallbacksHandler {
	return &PaymentCallbacksHandler{db: db, orders: repository.NewOrderStore(db), providers: providers}
}

// registeredAttempt is the server-side projection used to rebuild the
// trusted (tenant, order) pair from a verified merchant_order_id.
type registeredAttempt struct {
	TenantID uint64
	OrderID  string
}

// resolveByMerchantOrderID rebuilds the trusted tenant and order for a
// verified callback fact. The lookup uses ONLY locally registered attempts
// (C01 RegisterAttempt rows) — callback payload identity is never trusted.
func (h *PaymentCallbacksHandler) resolveByMerchantOrderID(provider, merchant, merchantOrderID string) (registeredAttempt, error) {
	if h == nil || h.db == nil {
		return registeredAttempt{}, errors.New("callback_store_unavailable")
	}
	var row registeredAttempt
	err := h.db.Raw(`SELECT tenant_id, order_id FROM commercial_payment_attempts
		WHERE provider = ? AND merchant = ? AND merchant_order_id = ?
		LIMIT 1`,
		provider, merchant, merchantOrderID).Scan(&row).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return registeredAttempt{}, err
	}
	if row.OrderID == "" {
		return registeredAttempt{}, repository.ErrPaymentAttemptNotFound
	}
	return row, nil
}

// HandleProviderCallback serves POST /api/v1/commercial/callbacks/:provider.
// It reads the raw body EXACTLY ONCE, authenticates it via Provider.Verify,
// rebuilds the trusted tenant/order from the local registry, and only after
// ConfirmPayment has durably recorded the fact answers success. A duplicate
// notification is idempotent: ConfirmPayment replays the original result
// (C01 semantics) and the success response is repeated.
func (h *PaymentCallbacksHandler) HandleProviderCallback(c *gin.Context) {
	providerName := c.Param("provider")
	if h == nil || h.providers[providerName] == nil {
		callbackFail(c, http.StatusServiceUnavailable, "unknown payment provider")
		return
	}
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		callbackFail(c, http.StatusBadRequest, "unreadable callback body")
		return
	}
	fact, err := h.providers[providerName].Verify(c.Request.Context(), c.Request.Header, body)
	if err != nil {
		// Verification failure must be answered with a non-2xx FAIL so the
		// provider keeps retrying; nothing is persisted.
		callbackFail(c, http.StatusUnauthorized, "signature verification failed")
		return
	}
	attempt, err := h.resolveByMerchantOrderID(fact.Provider, fact.Merchant, fact.AttemptID)
	if err != nil {
		callbackFail(c, http.StatusNotFound, "no registered payment attempt")
		return
	}
	fact.TenantID = attempt.TenantID
	fact.OrderID = attempt.OrderID
	// ConfirmPayment (C01) validates merchant, attempt, amount and currency
	// inside one transaction; a wrong amount/merchant/state rolls back with
	// ErrPaymentMismatch, and a replayed channel transaction returns the
	// original success without a second event.
	if err := h.orders.ConfirmPayment(c.Request.Context(), fact); err != nil {
		if errors.Is(err, domain.ErrPaymentMismatch) {
			callbackFail(c, http.StatusConflict, "payment does not match its order")
			return
		}
		callbackFail(c, http.StatusInternalServerError, "payment confirmation failed")
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "SUCCESS", "message": "OK"})
}

// callbackFail answers with the WeChat-required FAIL body and a non-2xx
// status so the provider retries the notification.
func callbackFail(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{"code": "FAIL", "message": message})
}
