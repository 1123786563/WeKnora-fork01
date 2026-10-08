package commercial

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	domain "github.com/Tencent/WeKnora/internal/commercial"
	repocommercial "github.com/Tencent/WeKnora/internal/commercial/repository/commercial"
	"github.com/Tencent/WeKnora/internal/logger"

	"gorm.io/gorm"
)

// ErrWebhookSignatureRejected: the delivered signature did not verify.
var ErrWebhookSignatureRejected = errors.New("webhook_signature_rejected")

// ErrWebhookMalformed: the payload did not carry the contract fields.
var ErrWebhookMalformed = errors.New("webhook_malformed")

// ProjectionReReader re-reads ONE authoritative object and updates the
// local projection (#98): the webhook is only a change notification, the
// authoritative value comes from the re-read — which is also why duplicate
// and out-of-order deliveries cannot regress a projection (last
// authoritative write wins, idempotent on a converged projection).
type ProjectionReReader func(ctx context.Context, tenantID uint64, externalID string) (changed bool, err error)

// webhookPayload is the provider-neutral notification contract the
// consumer accepts: the provider correlation identity never crosses into
// the Billing API.
type webhookPayload struct {
	EventID            string `json:"event_id"`
	WebhookType        string `json:"webhook_type"`
	ObjectID           string `json:"object_id"`
	ExternalCustomerID string `json:"external_customer_id"`
}

// WebhookService is the #98 commercial webhook consumer: signature
// verification over the raw body, unique-key dedupe, and — only after both
// — one authoritative re-read per notified object with an auditable trail.
type WebhookService struct {
	db      *gorm.DB
	secrets map[string][]byte
	readers map[string]ProjectionReReader
	store   *repocommercial.ProjectionStore
}

func NewWebhookService(db *gorm.DB, secrets map[string][]byte, readers map[string]ProjectionReReader) *WebhookService {
	return &WebhookService{db: db, secrets: secrets, readers: readers,
		store: repocommercial.NewProjectionStore(db)}
}

// verifyWebhook checks the HMAC-SHA256 signature over the raw body
// (sha256=<hex>, the GitHub/Stripe-style scheme). The Lago JWT face stays
// behind the real-stack adapter (env-gated) — the seam contract is
// provider-neutral.
func verifyWebhook(secret, body []byte, signature string) bool {
	if !strings.HasPrefix(signature, "sha256=") {
		return false
	}
	given, err := hex.DecodeString(strings.TrimPrefix(signature, "sha256="))
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	return hmac.Equal(given, mac.Sum(nil))
}

// HandleWebhook consumes one provider notification end to end: verify →
// dedupe on (provider, event_id) → re-read the authoritative object →
// audit. A duplicate delivery returns nil (receipt no-op).
func (s *WebhookService) HandleWebhook(ctx context.Context, provider, signature string, body []byte) error {
	secret := s.secrets[provider]
	if len(secret) == 0 {
		return domain.ErrPlatformUnconfigured
	}
	if !verifyWebhook(secret, body, signature) {
		return ErrWebhookSignatureRejected
	}
	var payload webhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return ErrWebhookMalformed
	}
	if payload.EventID == "" || payload.WebhookType == "" || payload.ObjectID == "" {
		return ErrWebhookMalformed
	}
	tenantID := domain.TenantIDFromExternalCustomerID(payload.ExternalCustomerID)
	fresh, err := s.store.RecordWebhook(ctx, &repocommercial.WebhookInboxRow{
		Provider: provider, EventID: payload.EventID, Kind: payload.WebhookType,
		ExternalID: payload.ObjectID, TenantID: tenantID, ReceivedAt: time.Now().UTC(),
	})
	if err != nil {
		return err
	}
	if !fresh {
		return nil
	}
	action := "received"
	if reader := s.readers[payload.WebhookType]; reader != nil {
		changed, rerr := reader(ctx, tenantID, payload.ObjectID)
		if rerr != nil {
			return rerr
		}
		if changed {
			action = "repaired"
		}
	}
	return s.store.Audit(ctx, &repocommercial.ProjectionAuditRow{
		Source: "webhook", Action: action, Kind: payload.WebhookType,
		ExternalID: payload.ObjectID, TenantID: tenantID, CreatedAt: time.Now().UTC(),
	})
}

// ReconciliationStream is the single durable cursor stream the local
// convergence loop consumes: one commercial stream, one watermark.
const ReconciliationStream = "commercial"

// ReconciliationService is the periodic convergence loop (#98): it streams
// authoritative changes from the durable cursor and drives the same
// authoritative re-read as the webhook path — a lost notification is
// recovered here. The cursor only advances on a fully successful pass.
type ReconciliationService struct {
	db       *gorm.DB
	platform domain.CommercialPlatform
	readers  map[string]ProjectionReReader
	store    *repocommercial.ProjectionStore

	stopMu sync.Mutex
	stop   chan struct{}
	interval time.Duration
}

func NewReconciliationService(db *gorm.DB, platform domain.CommercialPlatform, readers map[string]ProjectionReReader) *ReconciliationService {
	return &ReconciliationService{db: db, platform: platform, readers: readers,
		store: repocommercial.NewProjectionStore(db), interval: time.Minute}
}

// ReconcileOnce runs one convergence pass over the platform's change
// stream: every change triggers an authoritative re-read (audited as
// repaired when the projection actually moved), then the watermark
// advances. An unreachable authority leaves the cursor untouched.
func (s *ReconciliationService) ReconcileOnce(ctx context.Context) error {
	if s.platform == nil {
		return domain.ErrPlatformUnconfigured
	}
	stream := ReconciliationStream
	cursorValue, err := s.store.LoadCursor(ctx, stream)
	if err != nil {
		return err
	}
	page, err := s.platform.Reconcile(ctx, domain.ReconciliationCursor{Stream: stream, Value: cursorValue})
	if err != nil {
		return err
	}
	for _, ch := range page.Changes {
		action := "received"
		if reader := s.readers[ch.Kind]; reader != nil {
			changed, rerr := reader(ctx, ch.TenantID, ch.ExternalID)
			if rerr != nil {
				return rerr
			}
			if changed {
				action = "repaired"
			}
		}
		if err := s.store.Audit(ctx, &repocommercial.ProjectionAuditRow{
			Source: "reconcile", Action: action, Kind: ch.Kind,
			ExternalID: ch.ExternalID, TenantID: ch.TenantID,
			Detail:     page.Next.Value, CreatedAt: time.Now().UTC(),
		}); err != nil {
			return err
		}
	}
	return s.store.SaveCursor(ctx, stream, page.Next.Value)
}

// StartBackground registers the convergence loop (#98): one immediate
// pass, then one per interval — the FulfillmentService StartBackground
// precedent. Safe to call more than once.
func (s *ReconciliationService) StartBackground(ctx context.Context) {
	s.stopMu.Lock()
	defer s.stopMu.Unlock()
	if s.stop != nil {
		return
	}
	stop := make(chan struct{})
	s.stop = stop
	go func() {
		run := func() {
			if err := s.ReconcileOnce(ctx); err != nil && !errors.Is(err, domain.ErrPlatformUnsupported) {
				// Unsupported means the seam family is disabled for this
				// deployment (drain-only wiring): a quiet no-op.
				logger.Warnf(ctx, "[CommercialReconciliation] pass failed: %v", err)
			}
		}
		run()
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				run()
			case <-stop:
				return
			}
		}
	}()
}

// Stop terminates the loop; no-op when never started.
func (s *ReconciliationService) Stop() {
	s.stopMu.Lock()
	defer s.stopMu.Unlock()
	if s.stop != nil {
		close(s.stop)
		s.stop = nil
	}
}
