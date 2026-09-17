package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	repocommercial "github.com/Tencent/WeKnora/internal/application/repository/commercial"
	domain "github.com/Tencent/WeKnora/internal/commercial"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/voice"
	"github.com/gin-gonic/gin"
)

// The W30 mobile voice surface: authorized short-lived voice sessions and
// the server-proxied transcription endpoint that consumes W29's
// TRANSCRIBE_ENDPOINT_PENDING_W30 seam.
//
// Non-negotiable invariants:
//   - Ownership and budget resolve ONLY from the product context (the
//     authenticated tenant/user); no request field can widen them.
//   - Charged voice requires a configured voice admission (price version +
//     audio_seconds rate). Unconfigured pricing refuses the call — the
//     voice plane never borrows a text dimension to price audio, and never
//     grants an unpriced charged session.
//   - The durable budget hold (ExecutionGate.Begin) is taken BEFORE the
//     provider open; a timed-out open parks the session as `unknown` for
//     reconciliation instead of blindly opening a second charged session.
//   - The grant token exists exactly once: in the creation response. It is
//     stored only as a hash and is never written to a log.
//   - Settlement runs on trusted provider-reported usage, idempotently, in
//     every terminal path — disconnect, repeated stop, callback replay and
//     failed transcription settle the real consumed cost exactly once.

const (
	// voiceSessionMaxSeconds bounds one authorized session's requested
	// window (the ticket TTL stays short-lived).
	voiceSessionMaxSeconds = 600
	// voiceTranscribeMaxSeconds bounds one proxied transcription's budget
	// hold; the provider-reported duration settles the actual cost.
	voiceTranscribeMaxSeconds = 120
	// voiceMaxAudioBytes bounds one uploaded capture (16 MiB).
	voiceMaxAudioBytes = 16 << 20
)

var (
	// ErrVoiceChargingUnconfigured is the fail-closed answer when no voice
	// admission (price version + audio rate) is configured.
	ErrVoiceChargingUnconfigured = errors.New("voice charging unconfigured")
)

// VoiceSessionRepository is the persistence seam the handler codes against;
// repository.VoiceSessionStore satisfies it.
type VoiceSessionRepository interface {
	CreateVoiceSession(ctx context.Context, row repository.VoiceSessionRow) (repository.VoiceSessionRow, error)
	GetOwnedVoiceSession(ctx context.Context, tenantID uint64, ownerID, id string) (repository.VoiceSessionRow, error)
	MarkVoiceSessionUnknown(ctx context.Context, tenantID uint64, id string) error
	PendingUnknownVoiceSession(ctx context.Context, tenantID uint64, sessionID string) (bool, error)
	RecordVoiceSessionUsage(ctx context.Context, tenantID uint64, id string, audioSeconds int64) error
	CloseVoiceSessionWithUsage(ctx context.Context, tenantID uint64, id string, audioSeconds int64, settledAt time.Time) (bool, error)
}

// VoiceTranscriptionRepository persists request-id'd transcription results
// (the W04-style request reconciliation record of the replay path);
// repository.VoiceTranscriptionStore satisfies it.
type VoiceTranscriptionRepository interface {
	SaveVoiceTranscription(ctx context.Context, row repository.VoiceTranscriptionRow) error
	GetOwnedVoiceTranscription(ctx context.Context, tenantID uint64, ownerID, requestID string) (repository.VoiceTranscriptionRow, error)
	MarkVoiceTranscriptionSettled(ctx context.Context, tenantID uint64, requestID string) (bool, error)
}

// VoiceRateSource resolves the charged-voice admission numbers: the price
// version in force and the upper-bound credits for a requested audio
// window. An unconfigured source refuses charged voice.
type VoiceRateSource interface {
	VoiceAdmission(ctx context.Context, maxSeconds int64) (priceVersion string, upper domain.Credits, err error)
}

// VoiceReservationReleaser releases a hold the voice path took but could
// not use (a provider session that never mapped to a durable row). The
// commercial BudgetStore satisfies it; nil keeps the hold for
// reconciliation instead of releasing it.
type VoiceReservationReleaser interface {
	ReleaseReservation(ctx context.Context, tenantID uint64, reservationKey string) error
}

// MobileVoiceHandler serves POST /mobile/voice/sessions,
// DELETE /mobile/voice/sessions/:id and POST /mobile/voice/transcriptions.
type MobileVoiceHandler struct {
	sessions       VoiceSessionRepository
	transcriptions VoiceTranscriptionRepository
	provider       voice.VoiceProvider
	transcriber    voice.Transcriber
	gate           domain.ExecutionGate
	rates          VoiceRateSource
	releases       VoiceReservationReleaser
	now            func() time.Time
}

// NewMobileVoiceHandler validates the wiring. Every dependency is required:
// a nil provider or gate fails closed (the constructor refuses), matching
// the W26 craft assembly rule. The reservation releaser is optional.
func NewMobileVoiceHandler(
	sessions VoiceSessionRepository,
	transcriptions VoiceTranscriptionRepository,
	provider voice.VoiceProvider,
	transcriber voice.Transcriber,
	gate domain.ExecutionGate,
	rates VoiceRateSource,
	releasers ...VoiceReservationReleaser,
) (*MobileVoiceHandler, error) {
	if sessions == nil || transcriptions == nil || provider == nil || transcriber == nil || gate == nil || rates == nil {
		return nil, errors.New("mobile voice handler wiring incomplete")
	}
	var releases VoiceReservationReleaser
	if len(releasers) > 0 {
		releases = releasers[0]
	}
	return &MobileVoiceHandler{
		sessions:       sessions,
		transcriptions: transcriptions,
		provider:       provider,
		transcriber:    transcriber,
		gate:           gate,
		rates:          rates,
		releases:       releases,
		now:            time.Now,
	}, nil
}

// SetClock overrides the clock (tests).
func (h *MobileVoiceHandler) SetClock(now func() time.Time) {
	if now != nil {
		h.now = now
	}
}

// voiceIdentity resolves tenant and owner from the product context only.
func voiceIdentity(c *gin.Context) (uint64, string, bool) {
	tenantID, ok := types.TenantIDFromContext(c.Request.Context())
	if !ok || tenantID == 0 {
		if value, exists := c.Get(types.TenantIDContextKey.String()); exists {
			tenantID, ok = value.(uint64)
		}
	}
	ownerID, ownerOK := types.UserIDFromContext(c.Request.Context())
	if !ownerOK || ownerID == "" {
		if value, exists := c.Get(types.UserIDContextKey.String()); exists {
			ownerID, ownerOK = value.(string)
		}
	}
	if !ok || tenantID == 0 || !ownerOK || ownerID == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthenticated"})
		return 0, "", false
	}
	return tenantID, ownerID, true
}

type createVoiceSessionRequest struct {
	SessionID  string `json:"session_id"`
	RunID      string `json:"run_id"`
	MaxSeconds int    `json:"max_seconds"`
	Deadline   string `json:"deadline"`
}

// CreateVoiceSession POST /mobile/voice/sessions.
func (h *MobileVoiceHandler) CreateVoiceSession(c *gin.Context) {
	tenantID, ownerID, ok := voiceIdentity(c)
	if !ok {
		return
	}
	var request createVoiceSessionRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid_request"})
		return
	}
	request.SessionID = strings.TrimSpace(request.SessionID)
	if request.SessionID == "" || request.MaxSeconds <= 0 || request.MaxSeconds > voiceSessionMaxSeconds {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid_voice_session_request"})
		return
	}
	now := h.now().UTC()
	// An un-reconciled unknown session of this product session blocks NEW
	// charged sessions before any budget or provider call: never repeatedly
	// open billed sessions on top of an uncertain one.
	if pending, err := h.sessions.PendingUnknownVoiceSession(c.Request.Context(), tenantID, request.SessionID); err == nil && pending {
		c.AbortWithStatusJSON(http.StatusConflict, gin.H{"success": false, "error": "voice_session_unknown_pending"})
		return
	} else if err != nil && !errors.Is(err, repository.ErrVoiceSessionInvalid) {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"success": false, "error": "voice_session_store_unavailable"})
		return
	}
	// The task deadline: caller-supplied but clamped to the voice session
	// window; absent means now+window.
	deadline := now.Add(voiceSessionMaxSeconds * time.Second)
	if raw := strings.TrimSpace(request.Deadline); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid_deadline"})
			return
		}
		if parsed.Before(deadline) {
			deadline = parsed
		}
	}
	sessionID := "vs_" + strconv.FormatInt(now.UnixNano(), 36) + "_" + shortVoiceDigest(now, tenantID, ownerID, request.SessionID)

	// Idempotent replay: an existing row is answered from its recorded
	// state without re-opening (or re-charging) anything.
	if existing, err := h.sessions.GetOwnedVoiceSession(c.Request.Context(), tenantID, ownerID, sessionID); err == nil {
		h.writeExistingVoiceSession(c, existing)
		return
	} else if !errors.Is(err, repository.ErrVoiceSessionNotFound) && !errors.Is(err, repository.ErrVoiceSessionInvalid) {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"success": false, "error": "voice_session_store_unavailable"})
		return
	}

	// Charged-voice admission: price version + audio rate must be
	// configured, or the session is refused (never text-dimension priced).
	priceVersion, upper, err := h.rates.VoiceAdmission(c.Request.Context(), int64(request.MaxSeconds))
	if err != nil {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "voice_charging_unconfigured"})
		return
	}

	// The W30 admission rule: the grant never outlives the deadline and an
	// unfunded request is refused before any provider call.
	expiry, err := voice.AuthorizeVoice(now, deadline, int64(upper))
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, voice.ErrUnfundedVoice) {
			status = http.StatusPaymentRequired
		}
		c.AbortWithStatusJSON(status, gin.H{"success": false, "error": err.Error()})
		return
	}
	maxSeconds := int(expiry.Sub(now).Seconds())
	if maxSeconds <= 0 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"success": false, "error": "voice_window_too_short"})
		return
	}

	// Durable budget admission BEFORE the provider open (idempotent hold).
	reservationKey := "voice_open:" + sessionID
	if _, err := h.gate.Begin(c.Request.Context(), domain.BudgetRequest{
		TenantID: tenantID,
		RunID:    voiceSessionRunID(request.RunID, request.SessionID),
		Key:      reservationKey,
		Upper:    upper,
		Deadline: deadline,
	}); err != nil {
		c.AbortWithStatusJSON(http.StatusPaymentRequired, gin.H{"success": false, "error": "voice_budget_denied"})
		return
	}

	ref := voice.SessionRef(tenantID, ownerID, sessionID)
	grant, err := h.provider.Open(c.Request.Context(), ref, maxSeconds)
	if err != nil {
		if errors.Is(err, voice.ErrProviderOpenUnknown) {
			// The provider may have opened a billable session: park the
			// row as unknown (keeping its hold) for reconciliation — a new
			// charged session of the same product session is refused until
			// this one is closed and settled.
			row := h.newVoiceSessionRow(tenantID, ownerID, sessionID, request, repository.VoiceSessionStateUnknown, expiry, maxSeconds, reservationKey, "")
			if _, createErr := h.sessions.CreateVoiceSession(c.Request.Context(), row); createErr != nil {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"success": false, "error": "voice_open_unknown_persist_failed"})
				return
			}
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "voice_open_unknown", "data": gin.H{"id": sessionID, "state": repository.VoiceSessionStateUnknown}})
			return
		}
		// I-3 (review round 1): a DEFINITE provider refusal (as opposed to
		// the unknown timeout above) means no session exists — the hold it
		// took is unusable and must be released immediately, so repeated
		// failures can never accumulate dangling holds.
		h.releaseHold(c.Request.Context(), tenantID, reservationKey)
		c.AbortWithStatusJSON(http.StatusBadGateway, gin.H{"success": false, "error": "voice_provider_unavailable"})
		return
	}

	row := h.newVoiceSessionRow(tenantID, ownerID, sessionID, request, repository.VoiceSessionStateOpen, expiry, maxSeconds, reservationKey, repository.HashVoiceToken(grant.Token))
	if _, err := h.sessions.CreateVoiceSession(c.Request.Context(), row); err != nil {
		// The provider session exists but the mapping could not be
		// recorded: close the provider side so no unmapped charged session
		// survives, and release the unusable hold when a releaser is wired
		// (otherwise it stays for reconciliation).
		_ = h.provider.Close(context.WithoutCancel(c.Request.Context()), ref)
		h.releaseHold(c.Request.Context(), tenantID, reservationKey)
		if errors.Is(err, repository.ErrVoiceSessionUnknownOpen) {
			c.AbortWithStatusJSON(http.StatusConflict, gin.H{"success": false, "error": "voice_session_unknown_pending"})
			return
		}
		if errors.Is(err, repository.ErrVoiceSessionExists) {
			c.AbortWithStatusJSON(http.StatusConflict, gin.H{"success": false, "error": "voice_session_exists"})
			return
		}
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"success": false, "error": "voice_session_persist_failed"})
		return
	}
	// The plaintext token appears exactly once, here. Never logged.
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
		"id":           sessionID,
		"token":        grant.Token,
		"expires_at":   expiry,
		"max_seconds":  maxSeconds,
		"price_version": priceVersion,
	}})
}

func (h *MobileVoiceHandler) newVoiceSessionRow(tenantID uint64, ownerID, sessionID string, request createVoiceSessionRequest, state string, expiry time.Time, maxSeconds int, reservationKey, tokenHash string) repository.VoiceSessionRow {
	return repository.VoiceSessionRow{
		TenantID:       tenantID,
		ID:             sessionID,
		OwnerID:        ownerID,
		SessionID:      request.SessionID,
		RunID:          request.RunID,
		ProviderRef:    voice.SessionRef(tenantID, ownerID, sessionID),
		State:          state,
		Deadline:       deadlineOf(expiry, request),
		ExpiresAt:      expiry,
		MaxSeconds:     maxSeconds,
		ReservationKey: reservationKey,
		TokenHash:      tokenHash,
	}
}

// releaseHold drops an unusable hold when a releaser is wired.
func (h *MobileVoiceHandler) releaseHold(ctx context.Context, tenantID uint64, reservationKey string) {
	if h.releases != nil {
		_ = h.releases.ReleaseReservation(ctx, tenantID, reservationKey)
	}
}

// writeExistingVoiceSession answers an idempotent create replay from the
// recorded row WITHOUT re-issuing a token (only its hash is stored).
func (h *MobileVoiceHandler) writeExistingVoiceSession(c *gin.Context, existing repository.VoiceSessionRow) {
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
		"id":            existing.ID,
		"state":         existing.State,
		"expires_at":    existing.ExpiresAt,
		"max_seconds":   existing.MaxSeconds,
		"token_issued":  false,
		"replay":        true,
	}})
}

// StopVoiceSession DELETE /mobile/voice/sessions/:id — the stop/settle
// path. Cross-owner and missing rows are indistinguishable (404) and no
// provider call happens for them. Repeated stops are idempotent: the row's
// CAS + the settlement revision replay guarantee exactly-once charging.
func (h *MobileVoiceHandler) StopVoiceSession(c *gin.Context) {
	tenantID, ownerID, ok := voiceIdentity(c)
	if !ok {
		return
	}
	id := strings.TrimSpace(c.Param("id"))
	row, err := h.sessions.GetOwnedVoiceSession(c.Request.Context(), tenantID, ownerID, id)
	if errors.Is(err, repository.ErrVoiceSessionNotFound) || errors.Is(err, repository.ErrVoiceSessionInvalid) {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"success": false, "error": "voice_session_store_unavailable"})
		return
	}
	if row.State == repository.VoiceSessionStateClosed {
		// Repeated stop: already settled, nothing re-charged.
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"id": row.ID, "state": row.State, "settled": true, "replay": true}})
		return
	}
	// Best-effort provider close; settlement runs regardless of its outcome
	// (a disconnect already ended the media leg).
	closeErr := h.provider.Close(c.Request.Context(), row.ProviderRef)

	if row.State == repository.VoiceSessionStateUnknown && row.AudioSeconds == 0 {
		// Outcome-unknown and no trusted usage on record: keep the hold
		// for reconciliation instead of releasing spend protection.
		status := http.StatusOK
		if closeErr != nil {
			status = http.StatusAccepted
		}
		c.AbortWithStatusJSON(status, gin.H{"success": true, "data": gin.H{"id": row.ID, "state": repository.VoiceSessionStateUnknown, "settled": false, "reason": "pending_reconciliation"}})
		return
	}

	// I-4 (review round 1): a settlement failure must NOT close the row —
	// closing here would make the settlement permanently unreachable (the
	// closed replay branch answers before ever retrying Finish). The row
	// keeps its settleable state, the response reports settled:false, and a
	// retried stop runs the settlement again (the fact's revision identity
	// keeps the eventual settlement exactly-once).
	if settleErr := h.settleVoiceUsage(c.Request.Context(), tenantID, row); settleErr != nil {
		c.AbortWithStatusJSON(http.StatusAccepted, gin.H{"success": true, "data": gin.H{
			"id":      row.ID,
			"state":   row.State,
			"settled": false,
			"reason":  "settlement_retry_required",
		}})
		return
	}
	_, closeRowErr := h.sessions.CloseVoiceSessionWithUsage(c.Request.Context(), tenantID, row.ID, 0, h.now().UTC())
	if closeRowErr != nil && !errors.Is(closeRowErr, repository.ErrVoiceSessionStateConflict) {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"success": false, "error": "voice_session_close_failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
		"id":            row.ID,
		"state":         repository.VoiceSessionStateClosed,
		"settled":       true,
		"audio_seconds": row.AudioSeconds,
	}})
}

// settleVoiceUsage settles the session's trusted accumulated usage exactly
// once: the (tenant, call, attempt, revision) identity replays as a no-op.
func (h *MobileVoiceHandler) settleVoiceUsage(ctx context.Context, tenantID uint64, row repository.VoiceSessionRow) error {
	fact := domain.UsageFact{
		TenantID:     tenantID,
		RunID:        row.RunID,
		CallID:       voice.SessionCallID(tenantID, row.ID),
		AttemptID:    "sess",
		Funding:      domain.FundingPlatform,
		Service:      domain.ServiceVoice,
		PriceVersion: h.priceVersionFor(ctx),
		Revision:     1,
		OccurredAt:   h.now().UTC(),
		Dimensions:   map[string]int64{domain.DimensionAudioSeconds: row.AudioSeconds},
		Status:       domain.UsageStatusFinal,
	}
	return h.gate.Finish(ctx, row.ReservationKey, fact)
}

// TranscribeAudio POST /mobile/voice/transcriptions — the real backend of
// W29's dictation seam. The phone uploads its capture; the server proxies
// it through its own long-lived provider key, meters the provider-reported
// duration, and settles it even when the transcription itself failed.
func (h *MobileVoiceHandler) TranscribeAudio(c *gin.Context) {
	tenantID, ownerID, ok := voiceIdentity(c)
	if !ok {
		return
	}
	requestID := strings.TrimSpace(c.PostForm("request_id"))
	voiceSessionID := strings.TrimSpace(c.PostForm("voice_session_id"))
	locale := strings.TrimSpace(c.PostForm("locale"))
	if requestID == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"success": false, "error": "missing_request_id"})
		return
	}
	// I-2 (review round 1) + R1-N1 (review round 2): request-id replay
	// reconciliation, the W04 shape. The REAL ExecutionGate conflicts on any
	// Begin replay of an already dispatched reservation key, so a replay can
	// never be answered by re-running the charged pipeline. A SETTLED result
	// row answers with the ORIGINAL outcome before any new budget, provider
	// or settlement call; an UNSETTLED row (its Finish failed) does not act
	// as a settled short-circuit — the replay retries the settlement itself
	// (keep-settleable, like the stop path) and answers settled honestly.
	if existing, replayErr := h.transcriptions.GetOwnedVoiceTranscription(c.Request.Context(), tenantID, ownerID, requestID); replayErr == nil {
		settled := existing.Settled
		if !settled {
			// The reservation is already dispatched (Begin replay would
			// conflict) and the audio is gone, but its trusted usage is
			// durable in the row: re-issue the SAME settlement fact — the
			// revision identity keeps it exactly-once — and mark the row.
			settled = h.retryTranscriptionSettlement(c.Request.Context(), tenantID, existing)
		}
		if existing.Status == repository.VoiceTranscriptionSucceeded {
			c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
				"text": existing.Text, "audio_seconds": existing.AudioSeconds, "settled": settled, "replay": true,
			}})
			return
		}
		c.AbortWithStatusJSON(http.StatusBadGateway, gin.H{"success": false, "error": "transcribe_failed", "data": gin.H{
			"audio_seconds": existing.AudioSeconds, "settled": settled, "replay": true,
		}})
		return
	} else if !errors.Is(replayErr, repository.ErrVoiceTranscriptionNotFound) && !errors.Is(replayErr, repository.ErrVoiceTranscriptionInvalid) {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"success": false, "error": "voice_transcription_store_unavailable"})
		return
	}
	file, header, err := c.Request.FormFile("audio")
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"success": false, "error": "missing_audio"})
		return
	}
	defer func() { _ = file.Close() }()
	if header.Size <= 0 || header.Size > voiceMaxAudioBytes {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid_audio_size"})
		return
	}
	audioBytes := make([]byte, header.Size)
	if _, err := file.Read(audioBytes); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"success": false, "error": "unreadable_audio"})
		return
	}
	// Optional binding: usage of this transcription accumulates onto the
	// caller's own open voice session (never another owner's row).
	var bound repository.VoiceSessionRow
	if voiceSessionID != "" {
		bound, err = h.sessions.GetOwnedVoiceSession(c.Request.Context(), tenantID, ownerID, voiceSessionID)
		if errors.Is(err, repository.ErrVoiceSessionNotFound) || errors.Is(err, repository.ErrVoiceSessionInvalid) {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"success": false, "error": "voice_session_store_unavailable"})
			return
		}
		if bound.State == repository.VoiceSessionStateClosed {
			c.AbortWithStatusJSON(http.StatusConflict, gin.H{"success": false, "error": "voice_session_closed"})
			return
		}
	}

	// Charged transcription admission: the same voice pricing, an
	// upper-bounded hold keyed by the request's idempotency identity.
	_, upper, err := h.rates.VoiceAdmission(c.Request.Context(), voiceTranscribeMaxSeconds)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "voice_charging_unconfigured"})
		return
	}
	callID := voice.TranscriptionCallID(tenantID, requestID)
	reservationKey := voiceTranscriptionReservationKey(callID)
	if _, err := h.gate.Begin(c.Request.Context(), domain.BudgetRequest{
		TenantID: tenantID,
		RunID:    voiceSessionRunID(bound.RunID, bound.SessionID),
		Key:      reservationKey,
		Upper:    upper,
		Deadline: h.now().UTC().Add(voiceTranscribeMaxSeconds * time.Second),
	}); err != nil {
		c.AbortWithStatusJSON(http.StatusPaymentRequired, gin.H{"success": false, "error": "voice_budget_denied"})
		return
	}

	result, transcribeErr := h.transcriber.Transcribe(c.Request.Context(), voice.TranscriptionRequest{
		SessionRef: bound.ProviderRef,
		Audio:      audioBytes,
		MimeType:   header.Header.Get("Content-Type"),
		Locale:     locale,
	})
	usage := result.AudioSeconds
	if usage < 0 {
		usage = 0
	}
	// I-1 (review round 1): these seconds settle EXACTLY ONCE, here, under
	// the transcription's own voice_tx reservation — the settlement fact's
	// (call, attempt, revision) identity makes callback/retry replays
	// no-ops. They are deliberately NOT accumulated onto the bound session
	// row: the session's stop settlement bills only the session's own
	// (realtime/callback) usage, so one physical consumption can never be
	// billed under both identities.
	settleErr := h.gate.Finish(c.Request.Context(), reservationKey, domain.UsageFact{
		TenantID:     tenantID,
		RunID:        bound.RunID,
		CallID:       callID,
		AttemptID:    "tx",
		Funding:      domain.FundingPlatform,
		Service:      domain.ServiceVoice,
		PriceVersion: h.priceVersionFor(c.Request.Context()),
		Revision:     1,
		OccurredAt:   h.now().UTC(),
		Dimensions:   map[string]int64{domain.DimensionAudioSeconds: usage},
		Status:       domain.UsageStatusFinal,
	})
	// Persist the request id's outcome with its REAL settlement state
	// (R1-N1): a settled row short-circuits future replays; an unsettled
	// row keeps the request id settleable so a later replay (or a
	// reconciliation sweep) can complete the charge. A persist failure is
	// answered 500 — the outcome is lost to this request id (its
	// reservation is already dispatched); the client retries with a NEW
	// request id.
	resultRow := repository.VoiceTranscriptionRow{
		TenantID:     tenantID,
		RequestID:    requestID,
		CallID:       callID,
		OwnerID:      ownerID,
		RunID:        bound.RunID,
		Status:       repository.VoiceTranscriptionFailed,
		AudioSeconds: usage,
		Settled:      settleErr == nil,
	}
	if transcribeErr == nil {
		resultRow.Status = repository.VoiceTranscriptionSucceeded
		resultRow.Text = result.Text
	}
	if saveErr := h.transcriptions.SaveVoiceTranscription(c.Request.Context(), resultRow); saveErr != nil && !errors.Is(saveErr, repository.ErrVoiceTranscriptionExists) {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"success": false, "error": "voice_transcription_persist_failed"})
		return
	}
	if transcribeErr != nil {
		c.AbortWithStatusJSON(http.StatusBadGateway, gin.H{"success": false, "error": "transcribe_failed", "data": gin.H{"audio_seconds": usage, "settled": settleErr == nil}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"text": result.Text, "audio_seconds": usage, "settled": settleErr == nil}})
}

// retryTranscriptionSettlement completes a deferred settlement for one
// unsettled result row (R1-N1): the reservation is already dispatched and
// the audio is gone, but the trusted provider-reported usage is durable in
// the row. It re-issues the SAME settlement fact — the revision identity
// keeps it exactly-once, and an already-landed settlement replays as an
// idempotent no-op — then flips the row's settled flag.
func (h *MobileVoiceHandler) retryTranscriptionSettlement(ctx context.Context, tenantID uint64, row repository.VoiceTranscriptionRow) bool {
	if row.AudioSeconds < 0 || strings.TrimSpace(row.CallID) == "" {
		return false
	}
	err := h.gate.Finish(ctx, voiceTranscriptionReservationKey(row.CallID), domain.UsageFact{
		TenantID:     tenantID,
		RunID:        row.RunID,
		CallID:       row.CallID,
		AttemptID:    "tx",
		Funding:      domain.FundingPlatform,
		Service:      domain.ServiceVoice,
		PriceVersion: h.priceVersionFor(ctx),
		Revision:     1,
		OccurredAt:   h.now().UTC(),
		Dimensions:   map[string]int64{domain.DimensionAudioSeconds: row.AudioSeconds},
		Status:       domain.UsageStatusFinal,
	})
	if err != nil {
		return false
	}
	// The settlement landed. A failed/flipped flag only costs one more
	// idempotent replay — never a second charge.
	_, _ = h.transcriptions.MarkVoiceTranscriptionSettled(ctx, tenantID, row.RequestID)
	return true
}

func voiceTranscriptionReservationKey(callID string) string {
	return "voice_tx:" + callID
}

// priceVersionFor resolves the current voice price version for a settlement
// fact. An unconfigured version keeps the empty string, which the
// settlement layer rejects — an unpriced fact never settles as zero.
func (h *MobileVoiceHandler) priceVersionFor(ctx context.Context) string {
	version, _, err := h.rates.VoiceAdmission(ctx, 1)
	if err != nil {
		return ""
	}
	return version
}

func voiceSessionRunID(runID, sessionID string) string {
	if strings.TrimSpace(runID) != "" {
		return runID
	}
	return "voice:" + sessionID
}

// shortVoiceDigest derives the session id's owner-binding suffix so ids of
// one owner are not enumerable from another's.
func shortVoiceDigest(now time.Time, tenantID uint64, ownerID, sessionID string) string {
	sum := sha256.Sum256([]byte(strconv.FormatUint(tenantID, 10) + "|" + ownerID + "|" + sessionID + "|" + strconv.FormatInt(now.UnixNano(), 10)))
	return hex.EncodeToString(sum[:8])
}

func deadlineOf(expiry time.Time, request createVoiceSessionRequest) time.Time {
	if raw := strings.TrimSpace(request.Deadline); raw != "" {
		if parsed, err := time.Parse(time.RFC3339, raw); err == nil && parsed.After(expiry) {
			return parsed
		}
	}
	return expiry
}

// PriceVersionVoiceAdmission is the production VoiceRateSource over the
// commercial rate resolver: a configured price version plus its
// audio_seconds dimension rate yield the budget upper bound for a requested
// window. Missing version, unresolvable rates, or a version without an
// audio rate refuse charged voice (ErrVoiceChargingUnconfigured) — audio is
// never priced through the text dimensions.
type PriceVersionVoiceAdmission struct {
	Version string
	Resolve repocommercial.RateResolver
}

// VoiceAdmission implements VoiceRateSource.
func (p *PriceVersionVoiceAdmission) VoiceAdmission(_ context.Context, maxSeconds int64) (string, domain.Credits, error) {
	if p == nil || strings.TrimSpace(p.Version) == "" || p.Resolve == nil {
		return "", 0, ErrVoiceChargingUnconfigured
	}
	rates, err := p.Resolve(p.Version)
	if err != nil {
		return "", 0, fmt.Errorf("%w: %v", ErrVoiceChargingUnconfigured, err)
	}
	audio, ok := rates.Rates[domain.DimensionAudioSeconds]
	if !ok || audio.Units <= 0 || audio.RateMicro < 0 || maxSeconds <= 0 {
		return "", 0, ErrVoiceChargingUnconfigured
	}
	upper := new(big.Rat).SetFrac(
		new(big.Int).Mul(big.NewInt(audio.RateMicro), big.NewInt(maxSeconds)),
		big.NewInt(audio.Units),
	)
	// Round the reservation UP so the hold can never undercut the real
	// charge of the window it admits.
	num, den := upper.Num(), upper.Denom()
	q, rem := new(big.Int).QuoRem(num, den, new(big.Int))
	if rem.Sign() != 0 {
		q.Add(q, big.NewInt(1))
	}
	if !q.IsInt64() {
		return "", 0, ErrVoiceChargingUnconfigured
	}
	return p.Version, domain.Credits(q.Int64()), nil
}
