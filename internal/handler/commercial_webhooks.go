package handler

import (
	"errors"
	"io"
	"net/http"

	domain "github.com/Tencent/WeKnora/internal/modules/commercial"
	commercialsvc "github.com/Tencent/WeKnora/internal/modules/commercial/service/commercial"

	"github.com/gin-gonic/gin"
)

// CommercialWebhookHandler receives asynchronous billing-authority
// webhooks (#98 / Lago 26). The face is anonymously reachable: authenticity
// comes from the HMAC signature over the raw body, never from a session.
// The payload NEVER carries a trusted tenant — the identity is rebuilt
// from the deterministic external customer id by the service.
type CommercialWebhookHandler struct {
	webhooks *commercialsvc.WebhookService
}

func NewCommercialWebhookHandler(webhooks *commercialsvc.WebhookService) *CommercialWebhookHandler {
	return &CommercialWebhookHandler{webhooks: webhooks}
}

// HandleWebhook serves POST /api/v1/commercial/webhooks/:provider.
// It reads the raw body exactly once, hands the RAW body plus signature
// header to the service (verify → dedupe → authoritative re-read → audit)
// and answers the provider's retry contract: non-2xx keeps the retries
// coming, 200 acknowledges — including the duplicate no-op.
func (h *CommercialWebhookHandler) HandleWebhook(c *gin.Context) {
	if h == nil || h.webhooks == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": "FAIL", "message": "webhook consumer unconfigured"})
		return
	}
	// Same anonymous-surface cap as the payment callbacks: the body is
	// buffered before the signature check can fail it.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "FAIL", "message": "unreadable webhook body"})
		return
	}
	provider := c.Param("provider")
	signature := c.GetHeader("X-WeKnora-Signature")
	err = h.webhooks.HandleWebhook(c.Request.Context(), provider, signature, body)
	switch {
	case err == nil:
		c.JSON(http.StatusOK, gin.H{"code": "SUCCESS", "message": "OK"})
	case errors.Is(err, commercialsvc.ErrWebhookSignatureRejected):
		c.JSON(http.StatusUnauthorized, gin.H{"code": "FAIL", "message": "signature verification failed"})
	case errors.Is(err, domain.ErrPlatformUnconfigured):
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": "FAIL", "message": "webhook provider unconfigured"})
	case errors.Is(err, commercialsvc.ErrWebhookMalformed):
		c.JSON(http.StatusBadRequest, gin.H{"code": "FAIL", "message": "malformed webhook payload"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"code": "FAIL", "message": "webhook processing failed"})
	}
}
