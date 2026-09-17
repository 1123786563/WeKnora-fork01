package handler

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	domain "github.com/Tencent/WeKnora/internal/commercial"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/voice"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// ---- harness doubles -------------------------------------------------------

// fakeVoiceGate records the admission/settlement traffic. Finish mirrors the
// commercial revision idempotency: replaying the same (call, attempt,
// revision) is a no-op, so "settled exactly once" is decidable from the
// recorded facts.
type fakeVoiceGate struct {
	mu        sync.Mutex
	denyBegin bool
	begins    int
	finishes  int
	beginKeys map[string]int
	settled   map[string]domain.UsageFact
}

func newFakeVoiceGate() *fakeVoiceGate {
	return &fakeVoiceGate{beginKeys: map[string]int{}, settled: map[string]domain.UsageFact{}}
}

func (g *fakeVoiceGate) Begin(_ context.Context, req domain.BudgetRequest) (domain.Reservation, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.denyBegin {
		return domain.Reservation{}, fmt.Errorf("%w: denied", domain.ErrInsufficientBudgetGate)
	}
	g.begins++
	g.beginKeys[req.Key]++
	return domain.Reservation{ID: req.Key, RunID: req.RunID, Upper: req.Upper, Deadline: req.Deadline}, nil
}

func (g *fakeVoiceGate) Finish(_ context.Context, reservationID string, fact domain.UsageFact) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.finishes++
	key := fmt.Sprintf("%d|%s|%s|%d", fact.TenantID, fact.CallID, fact.AttemptID, fact.Revision)
	if _, replay := g.settled[key]; replay {
		return nil // revision replay: idempotent no-op, like the store
	}
	g.settled[key] = fact
	return nil
}

func (g *fakeVoiceGate) settledFacts() []domain.UsageFact {
	g.mu.Lock()
	defer g.mu.Unlock()
	out := make([]domain.UsageFact, 0, len(g.settled))
	for _, f := range g.settled {
		out = append(out, f)
	}
	return out
}

func (g *fakeVoiceGate) beginCount() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.begins
}

func (g *fakeVoiceGate) finishCount() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.finishes
}

// fakeVoiceProvider counts provider calls — the acceptance matrix asserts
// rejections leave it at zero.
type fakeVoiceProvider struct {
	mu        sync.Mutex
	opens     int
	closes    int
	openErr   error
	closeErr  error
	openedRef []string
}

func (p *fakeVoiceProvider) Open(_ context.Context, sessionRef string, maxSeconds int) (voice.Grant, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.opens++
	p.openedRef = append(p.openedRef, sessionRef)
	if p.openErr != nil {
		return voice.Grant{}, p.openErr
	}
	return voice.Grant{Token: "ephemeral-token-" + sessionRef, ExpiresAt: time.Now().UTC().Add(time.Duration(maxSeconds) * time.Second), MaxSeconds: maxSeconds}, nil
}

func (p *fakeVoiceProvider) Close(_ context.Context, _ string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closes++
	return p.closeErr
}

func (p *fakeVoiceProvider) counts() (int, int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.opens, p.closes
}

type fakeVoiceTranscriber struct {
	result voice.TranscriptionResult
	err    error
	calls  int
}

func (t *fakeVoiceTranscriber) Transcribe(_ context.Context, _ voice.TranscriptionRequest) (voice.TranscriptionResult, error) {
	t.calls++
	return t.result, t.err
}

type fakeVoiceRates struct {
	version string
	upper   domain.Credits
	err     error
}

func (r fakeVoiceRates) VoiceAdmission(context.Context, int64) (string, domain.Credits, error) {
	return r.version, r.upper, r.err
}

// ---- harness assembly ------------------------------------------------------

type voiceHarness struct {
	store       *repository.VoiceSessionStore
	provider    *fakeVoiceProvider
	transcriber *fakeVoiceTranscriber
	gate        *fakeVoiceGate
	handler     *MobileVoiceHandler
	router      *gin.Engine
	clock       time.Time
	logs        *bytes.Buffer
}

func newVoiceHarness(t *testing.T) *voiceHarness {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := db.AutoMigrate(&repository.VoiceSessionRow{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	h := &voiceHarness{
		store:       repository.NewVoiceSessionStore(db),
		provider:    &fakeVoiceProvider{},
		transcriber: &fakeVoiceTranscriber{},
		gate:        newFakeVoiceGate(),
		clock:       time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC),
		logs:        &bytes.Buffer{},
	}
	handler, err := NewMobileVoiceHandler(h.store, h.provider, h.transcriber, h.gate, fakeVoiceRates{version: "voice-test-v1", upper: 1000})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	handler.SetClock(func() time.Time { return h.clock })
	h.handler = handler
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(gin.LoggerWithWriter(h.logs))
	identity := func(c *gin.Context) {
		owner := "owner-a"
		if c.GetHeader("X-Test-Owner") == "owner-b" {
			owner = "owner-b"
		}
		ctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, uint64(7))
		ctx = context.WithValue(ctx, types.UserIDContextKey, owner)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
	voiceGroup := r.Group("/mobile/voice", identity)
	voiceGroup.POST("/sessions", handler.CreateVoiceSession)
	voiceGroup.DELETE("/sessions/:id", handler.StopVoiceSession)
	voiceGroup.POST("/transcriptions", handler.TranscribeAudio)
	h.router = r
	return h
}

func (h *voiceHarness) postSession(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/mobile/voice/sessions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.router.ServeHTTP(w, req)
	return w
}

func (h *voiceHarness) stopSession(t *testing.T, id, owner string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodDelete, "/mobile/voice/sessions/"+id, nil)
	if owner == "owner-b" {
		req.Header.Set("X-Test-Owner", "owner-b")
	}
	w := httptest.NewRecorder()
	h.router.ServeHTTP(w, req)
	return w
}

func (h *voiceHarness) postTranscription(t *testing.T, owner, requestID, voiceSessionID string, audio []byte) *httptest.ResponseRecorder {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("request_id", requestID)
	if voiceSessionID != "" {
		_ = writer.WriteField("voice_session_id", voiceSessionID)
	}
	part, _ := writer.CreateFormFile("audio", "capture.m4a")
	_, _ = part.Write(audio)
	_ = writer.Close()
	req := httptest.NewRequest(http.MethodPost, "/mobile/voice/transcriptions", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if owner == "owner-b" {
		req.Header.Set("X-Test-Owner", "owner-b")
	}
	w := httptest.NewRecorder()
	h.router.ServeHTTP(w, req)
	return w
}

func voiceSessionIDOf(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	if w.Code != http.StatusOK {
		t.Fatalf("session create failed: %d %s", w.Code, w.Body.String())
	}
	return strings.Trim(strings.Split(strings.Split(w.Body.String(), "\"id\":\"")[1], "\"")[0], "\"")
}

// ---- acceptance matrix -----------------------------------------------------

// Positive: an authorized session hands out a short-lived token that never
// contains the provider's long-lived key, and the token never reaches a log.
func TestVoiceSessionHappyPathTokenNeverLogged(t *testing.T) {
	h := newVoiceHarness(t)
	w := h.postSession(t, `{"session_id":"sess-1","run_id":"run-9","max_seconds":30}`)
	if w.Code != http.StatusOK {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "\"token\"") {
		t.Fatalf("create response missing token: %s", w.Body.String())
	}
	token := strings.Split(strings.Split(w.Body.String(), "\"token\":\"")[1], "\"")[0]
	if token == "" || strings.Contains(token, "sk-") {
		t.Fatalf("token boundary violated: %q", token)
	}
	if strings.Contains(h.logs.String(), token) {
		t.Fatal("grant token leaked into the request log")
	}
	if h.gate.beginCount() != 1 {
		t.Fatalf("begins = %d, want exactly one durable admission", h.gate.beginCount())
	}
	// Stop settles and closes.
	id := voiceSessionIDOf(t, w)
	w = h.stopSession(t, id, "owner-a")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "\"settled\":true") {
		t.Fatalf("stop: %d %s", w.Code, w.Body.String())
	}
	// Repeated stop stays idempotent: no second settlement fact.
	w = h.stopSession(t, id, "owner-a")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "\"replay\":true") {
		t.Fatalf("stop replay: %d %s", w.Code, w.Body.String())
	}
	if facts := h.gate.settledFacts(); len(facts) != 1 {
		t.Fatalf("settled facts = %d, want exactly one", len(facts))
	}
}

// Cross-owner: another owner sees 404 and provokes zero provider calls.
func TestVoiceSessionCrossOwnerInvisible(t *testing.T) {
	h := newVoiceHarness(t)
	w := h.postSession(t, `{"session_id":"sess-1","max_seconds":30}`)
	id := voiceSessionIDOf(t, w)
	w = h.stopSession(t, id, "owner-b")
	if w.Code != http.StatusNotFound {
		t.Fatalf("cross-owner stop: %d %s", w.Code, w.Body.String())
	}
	opens, closes := h.provider.counts()
	if opens != 1 || closes != 0 {
		t.Fatalf("provider calls = (%d opens, %d closes), close must not run for a foreign owner", opens, closes)
	}
	// Binding a foreign voice session to a transcription is equally invisible.
	w = h.postTranscription(t, "owner-b", "req-1", id, []byte("audio"))
	if w.Code != http.StatusNotFound {
		t.Fatalf("cross-owner bind: %d %s", w.Code, w.Body.String())
	}
	if h.transcriber.calls != 0 {
		t.Fatalf("transcriber calls = %d, want 0", h.transcriber.calls)
	}
}

// Budget denial: the durable admission refuses, the provider is never called.
func TestVoiceSessionBudgetDeniedProviderUntouched(t *testing.T) {
	h := newVoiceHarness(t)
	h.gate.denyBegin = true
	w := h.postSession(t, `{"session_id":"sess-1","max_seconds":30}`)
	if w.Code != http.StatusPaymentRequired {
		t.Fatalf("budget denied: %d %s", w.Code, w.Body.String())
	}
	opens, _ := h.provider.counts()
	if opens != 0 {
		t.Fatalf("provider opens = %d, want 0", opens)
	}
}

// Expired deadline: refused before any budget or provider call.
func TestVoiceSessionDeadlineExpiredRefused(t *testing.T) {
	h := newVoiceHarness(t)
	w := h.postSession(t, `{"session_id":"sess-1","max_seconds":30,"deadline":"2026-09-17T11:00:00Z"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expired deadline: %d %s", w.Code, w.Body.String())
	}
	if h.gate.beginCount() != 0 {
		t.Fatalf("begins = %d, want 0", h.gate.beginCount())
	}
	opens, _ := h.provider.counts()
	if opens != 0 {
		t.Fatalf("provider opens = %d, want 0", opens)
	}
}

// Unconfigured charging: no voice admission pricing means no charged voice
// session — never a text-dimension priced one.
func TestVoiceChargingUnconfiguredRefused(t *testing.T) {
	h := newVoiceHarness(t)
	w := h.postSession(t, `{"session_id":"sess-1","max_seconds":30}`)
	if w.Code != http.StatusOK {
		t.Fatalf("setup create failed: %d %s", w.Code, w.Body.String())
	}
	handler, err := NewMobileVoiceHandler(h.store, h.provider, h.transcriber, h.gate, fakeVoiceRates{err: ErrVoiceChargingUnconfigured})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/mobile/voice/sessions", func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, uint64(7))
		ctx = context.WithValue(ctx, types.UserIDContextKey, "owner-a")
		c.Request = c.Request.WithContext(ctx)
		handler.CreateVoiceSession(c)
	})
	w = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mobile/voice/sessions", strings.NewReader(`{"session_id":"sess-2","max_seconds":30}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("unconfigured: %d %s", w.Code, w.Body.String())
	}
	opens, _ := h.provider.counts()
	if opens != 1 { // only the setup call
		t.Fatalf("provider opens = %d, want only the setup call", opens)
	}
}

// Open-unknown: the timed-out open parks the session as unknown, and a new
// charged session of the same product session is refused BEFORE any new
// provider call (never repeatedly open billed sessions).
func TestVoiceOpenUnknownParksAndBlocksReopen(t *testing.T) {
	h := newVoiceHarness(t)
	h.provider.openErr = voice.ErrProviderOpenUnknown
	w := h.postSession(t, `{"session_id":"sess-1","max_seconds":30}`)
	if w.Code != http.StatusServiceUnavailable || !strings.Contains(w.Body.String(), "unknown") {
		t.Fatalf("unknown open: %d %s", w.Code, w.Body.String())
	}
	opens, _ := h.provider.counts()
	if opens != 1 {
		t.Fatalf("provider opens = %d, want 1", opens)
	}
	h.clock = h.clock.Add(2 * time.Second)
	w = h.postSession(t, `{"session_id":"sess-1","max_seconds":30}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("reopen over unknown: %d %s", w.Code, w.Body.String())
	}
	opens, _ = h.provider.counts()
	if opens != 1 {
		t.Fatalf("provider opens = %d, the reopen must not reach the provider", opens)
	}
	if h.gate.beginCount() != 1 {
		t.Fatalf("begins = %d, the reopen must not take a second hold", h.gate.beginCount())
	}
}

// A closed session keeps accepting no transcription binding.
func TestTranscriptionOnClosedSessionRefused(t *testing.T) {
	h := newVoiceHarness(t)
	w := h.postSession(t, `{"session_id":"sess-1","max_seconds":30}`)
	id := voiceSessionIDOf(t, w)
	h.transcriber.result = voice.TranscriptionResult{Text: "hi", AudioSeconds: 4}
	w = h.postTranscription(t, "owner-a", "req-1", id, []byte("audio"))
	if w.Code != http.StatusOK {
		t.Fatalf("bound transcription: %d %s", w.Code, w.Body.String())
	}
	_ = h.stopSession(t, id, "owner-a")
	w = h.postTranscription(t, "owner-a", "req-2", id, []byte("audio"))
	if w.Code != http.StatusConflict {
		t.Fatalf("closed binding: %d %s", w.Code, w.Body.String())
	}
}

// Transcription: settlement rides the trusted provider-reported duration;
// replaying the same request id settles exactly once (idempotent), and a
// FAILED transcription with partial usage still settles the real cost.
func TestTranscriptionSettlesIdempotentlyAndOnFailure(t *testing.T) {
	h := newVoiceHarness(t)
	h.transcriber.result = voice.TranscriptionResult{Text: "hello", AudioSeconds: 12}
	w := h.postTranscription(t, "owner-a", "req-1", "", []byte("audio"))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "hello") {
		t.Fatalf("transcription: %d %s", w.Code, w.Body.String())
	}
	// Replay of the same request id: a second provider call may happen, the
	// CHARGE does not — one settled fact for the identity.
	w = h.postTranscription(t, "owner-a", "req-1", "", []byte("audio"))
	if w.Code != http.StatusOK {
		t.Fatalf("transcription replay: %d %s", w.Code, w.Body.String())
	}
	facts := h.gate.settledFacts()
	if len(facts) != 1 {
		t.Fatalf("settled facts = %d, want exactly one", len(facts))
	}
	if facts[0].Dimensions[domain.DimensionAudioSeconds] != 12 {
		t.Fatalf("settled audio seconds = %v, want 12", facts[0].Dimensions)
	}
	if facts[0].Service != domain.ServiceVoice {
		t.Fatalf("settled service = %q, voice must bill under its own service", facts[0].Service)
	}
	// Failure with partial provider-reported usage still settles it.
	h.transcriber.err = voice.ErrTranscriptionFailed
	h.transcriber.result = voice.TranscriptionResult{Text: "", AudioSeconds: 3}
	w = h.postTranscription(t, "owner-a", "req-2", "", []byte("audio"))
	if w.Code != http.StatusBadGateway {
		t.Fatalf("failed transcription status: %d %s", w.Code, w.Body.String())
	}
	facts = h.gate.settledFacts()
	var partial int64
	for _, f := range facts {
		if f.CallID == voice.TranscriptionCallID(7, "req-2") {
			partial = f.Dimensions[domain.DimensionAudioSeconds]
		}
	}
	if partial != 3 {
		t.Fatalf("partial usage settled = %d, want 3", partial)
	}
}

// Budget denial on transcription: refused before the provider call.
func TestTranscriptionBudgetDeniedProviderUntouched(t *testing.T) {
	h := newVoiceHarness(t)
	h.gate.denyBegin = true
	w := h.postTranscription(t, "owner-a", "req-1", "", []byte("audio"))
	if w.Code != http.StatusPaymentRequired {
		t.Fatalf("denied transcription: %d %s", w.Code, w.Body.String())
	}
	if h.transcriber.calls != 0 {
		t.Fatalf("transcriber calls = %d, want 0", h.transcriber.calls)
	}
}

// Session-bound usage accumulates and settles on stop (disconnect still
// settles the real consumed cost).
func TestVoiceSessionStopSettlesAccumulatedUsage(t *testing.T) {
	h := newVoiceHarness(t)
	w := h.postSession(t, `{"session_id":"sess-1","max_seconds":30}`)
	id := voiceSessionIDOf(t, w)
	h.transcriber.result = voice.TranscriptionResult{Text: "a", AudioSeconds: 5}
	if w = h.postTranscription(t, "owner-a", "req-1", id, []byte("audio")); w.Code != http.StatusOK {
		t.Fatalf("bound transcription: %d %s", w.Code, w.Body.String())
	}
	h.provider.closeErr = errors.New("already gone") // disconnect: close fails
	w = h.stopSession(t, id, "owner-a")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "\"settled\":true") {
		t.Fatalf("stop with usage: %d %s", w.Code, w.Body.String())
	}
	for _, f := range h.gate.settledFacts() {
		if f.CallID == voice.SessionCallID(7, id) {
			if f.Dimensions[domain.DimensionAudioSeconds] != 5 {
				t.Fatalf("session settled seconds = %v, want 5", f.Dimensions)
			}
			return
		}
	}
	t.Fatal("session settlement fact missing")
}

// Voice bills under its own gating dimension: the commercial whitelist maps
// the voice service to audio_seconds (never a text dimension).
func TestVoiceServiceGatesOnAudioDimension(t *testing.T) {
	dimension, err := domain.ServiceDimension(domain.ServiceVoice)
	if err != nil {
		t.Fatalf("voice service unknown to the commercial gate: %v", err)
	}
	if dimension != domain.DimensionAudioSeconds {
		t.Fatalf("voice gated by %q, want %q", dimension, domain.DimensionAudioSeconds)
	}
	if domain.DimensionAudioSeconds == domain.DimensionModel || domain.DimensionAudioSeconds == domain.DimensionParsing {
		t.Fatal("audio dimension aliases a text dimension")
	}
}

// The production VoiceRateSource: unconfigured or text-only price versions
// refuse charged voice; a real audio rate rounds the hold UP.
func TestPriceVersionVoiceAdmission(t *testing.T) {
	if _, _, err := (&PriceVersionVoiceAdmission{}).VoiceAdmission(context.Background(), 30); !errors.Is(err, ErrVoiceChargingUnconfigured) {
		t.Fatalf("empty admission = %v", err)
	}
	textOnly := func(string) (domain.PriceVersionRates, error) {
		return domain.PriceVersionRates{Version: "v1", Rates: map[string]domain.DimensionRate{
			domain.DimensionModel: {RateMicro: 1500, Units: 1000},
		}}, nil
	}
	if _, _, err := (&PriceVersionVoiceAdmission{Version: "v1", Resolve: textOnly}).VoiceAdmission(context.Background(), 30); !errors.Is(err, ErrVoiceChargingUnconfigured) {
		t.Fatalf("text-only pricing must refuse voice: %v", err)
	}
	withAudio := func(string) (domain.PriceVersionRates, error) {
		return domain.PriceVersionRates{Version: "v1", Rates: map[string]domain.DimensionRate{
			domain.DimensionModel:         {RateMicro: 1500, Units: 1000},
			domain.DimensionAudioSeconds: {RateMicro: 1001, Units: 60},
		}}, nil
	}
	version, upper, err := (&PriceVersionVoiceAdmission{Version: "v1", Resolve: withAudio}).VoiceAdmission(context.Background(), 61)
	if err != nil || version != "v1" {
		t.Fatalf("audio admission failed: %v", err)
	}
	// 61 * 1001 / 60 = 1017.68... -> hold 1018 (round up).
	if upper != 1018 {
		t.Fatalf("upper = %d, want 1018 (rounded up)", upper)
	}
}
