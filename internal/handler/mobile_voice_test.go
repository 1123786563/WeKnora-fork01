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
	domain "github.com/Tencent/WeKnora/internal/modules/commercial"
	repocommercial "github.com/Tencent/WeKnora/internal/modules/commercial/repository/commercial"
	commercialsvc "github.com/Tencent/WeKnora/internal/modules/commercial/service/commercial"
	"github.com/Tencent/WeKnora/internal/modules/workbench/voice"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// ---- harness doubles -------------------------------------------------------

// fakeVoiceGate records the admission/settlement traffic. Begin mirrors the
// REAL commercial semantics the review round exposed (I-2): a reservation
// key replays idempotently only while its state is still `held`, and the
// execution gate marks Begin's outcome `dispatched` immediately — so any
// Begin replay of an already-used key conflicts and must surface as
// ErrInsufficientBudgetGate, never a silent second hold. Finish mirrors the
// commercial revision idempotency: replaying the same (call, attempt,
// revision) is a no-op.
type fakeVoiceGate struct {
	mu        sync.Mutex
	denyBegin bool
	finishErr error
	begins    int
	finishes  int
	beginKeys map[string]int
	held      map[string]bool // reservation keys already dispatched
	settled   map[string]domain.UsageFact
}

func newFakeVoiceGate() *fakeVoiceGate {
	return &fakeVoiceGate{beginKeys: map[string]int{}, held: map[string]bool{}, settled: map[string]domain.UsageFact{}}
}

func (g *fakeVoiceGate) Begin(_ context.Context, req domain.BudgetRequest) (domain.Reservation, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.denyBegin {
		return domain.Reservation{}, fmt.Errorf("%w: denied", domain.ErrInsufficientBudgetGate)
	}
	// Real replay semantics: an existing key whose state is no longer held
	// (our Begin dispatches immediately) answers a conflict.
	if _, exists := g.held[req.Key]; exists {
		return domain.Reservation{}, fmt.Errorf("%w: reservation_key_conflict", domain.ErrInsufficientBudgetGate)
	}
	g.begins++
	g.beginKeys[req.Key]++
	g.held[req.Key] = true
	return domain.Reservation{ID: req.Key, RunID: req.RunID, Upper: req.Upper, Deadline: req.Deadline}, nil
}

func (g *fakeVoiceGate) Finish(_ context.Context, reservationID string, fact domain.UsageFact) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.finishErr != nil {
		return g.finishErr
	}
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

// totalSettledAudioSeconds sums the settled facts: the exactly-once money
// invariant the review round demanded (I-1).
func (g *fakeVoiceGate) totalSettledAudioSeconds() int64 {
	g.mu.Lock()
	defer g.mu.Unlock()
	var total int64
	for _, f := range g.settled {
		total += f.Dimensions[domain.DimensionAudioSeconds]
	}
	return total
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

// fakeVoiceReleaser spies the hold-release seam (I-3).
type fakeVoiceReleaser struct {
	mu     sync.Mutex
	calls  int
	keys   []string
	ignore bool
}

func (r *fakeVoiceReleaser) ReleaseReservation(_ context.Context, _ uint64, key string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.ignore {
		return nil
	}
	r.calls++
	r.keys = append(r.keys, key)
	return nil
}

func (r *fakeVoiceReleaser) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls
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
	store          *repository.VoiceSessionStore
	transcriptions *repository.VoiceTranscriptionStore
	provider       *fakeVoiceProvider
	transcriber    *fakeVoiceTranscriber
	gate           *fakeVoiceGate
	releaser       *fakeVoiceReleaser
	handler        *MobileVoiceHandler
	router         *gin.Engine
	clock          time.Time
	logs           *bytes.Buffer
}

func newVoiceHarness(t *testing.T) *voiceHarness {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := db.AutoMigrate(&repository.VoiceSessionRow{}, &repository.VoiceTranscriptionRow{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	h := &voiceHarness{
		store:          repository.NewVoiceSessionStore(db),
		transcriptions: repository.NewVoiceTranscriptionStore(db),
		provider:       &fakeVoiceProvider{},
		transcriber:    &fakeVoiceTranscriber{},
		gate:           newFakeVoiceGate(),
		releaser:       &fakeVoiceReleaser{},
		clock:          time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC),
		logs:           &bytes.Buffer{},
	}
	handler, err := NewMobileVoiceHandler(h.store, h.transcriptions, h.provider, h.transcriber, h.gate, fakeVoiceRates{version: "voice-test-v1", upper: 1000}, h.releaser)
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
	handler, err := NewMobileVoiceHandler(h.store, h.transcriptions, h.provider, h.transcriber, h.gate, fakeVoiceRates{err: ErrVoiceChargingUnconfigured})
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

// Session-own usage (realtime/callback plane, recorded via the trusted
// callback path) settles at stop — a disconnect (provider close failing)
// still settles the real consumed cost exactly once.
func TestVoiceSessionStopSettlesAccumulatedUsage(t *testing.T) {
	h := newVoiceHarness(t)
	w := h.postSession(t, `{"session_id":"sess-1","max_seconds":30}`)
	id := voiceSessionIDOf(t, w)
	// Provider usage callback of the realtime plane: 5 seconds of session
	// audio (NOT transcription seconds — those settle under their own
	// voice_tx reservation per I-1).
	if err := h.store.RecordVoiceSessionUsage(context.Background(), 7, id, 5); err != nil {
		t.Fatalf("callback usage: %v", err)
	}
	h.provider.closeErr = errors.New("already gone") // disconnect: close fails
	w = h.stopSession(t, id, "owner-a")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "\"settled\":true") {
		t.Fatalf("stop with usage: %d %s", w.Code, w.Body.String())
	}
	if total := h.gate.totalSettledAudioSeconds(); total != 5 {
		t.Fatalf("total settled seconds = %d, want exactly 5", total)
	}
	// The whole plane settles exactly once overall: a bound transcription
	// of another 3 seconds adds its own single settlement.
	h.transcriber.result = voice.TranscriptionResult{Text: "a", AudioSeconds: 3}
	w2 := h.postTranscription(t, "owner-a", "req-1", "", []byte("audio"))
	if w2.Code != http.StatusOK {
		t.Fatalf("independent transcription: %d %s", w2.Code, w2.Body.String())
	}
	if total := h.gate.totalSettledAudioSeconds(); total != 8 {
		t.Fatalf("total settled seconds = %d, want exactly 8 (5 session + 3 transcription, each once)", total)
	}
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
			domain.DimensionModel:        {RateMicro: 1500, Units: 1000},
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

// ---- review round 1 (I-1..I-4) ------------------------------------------------

// I-1: a bound transcription's seconds settle EXACTLY ONCE overall. The
// transcription settles under its own voice_tx reservation at request time;
// the session stop settles only the session's own (realtime/callback)
// usage. Total settled audio across ALL facts must equal what the provider
// actually reported — never twice.
func TestBoundTranscriptionSettlesExactlyOnce(t *testing.T) {
	h := newVoiceHarness(t)
	w := h.postSession(t, `{"session_id":"sess-1","max_seconds":30}`)
	id := voiceSessionIDOf(t, w)
	h.transcriber.result = voice.TranscriptionResult{Text: "a", AudioSeconds: 5}
	if w = h.postTranscription(t, "owner-a", "req-1", id, []byte("audio")); w.Code != http.StatusOK {
		t.Fatalf("bound transcription: %d %s", w.Code, w.Body.String())
	}
	if w = h.stopSession(t, id, "owner-a"); w.Code != http.StatusOK {
		t.Fatalf("stop: %d %s", w.Code, w.Body.String())
	}
	if total := h.gate.totalSettledAudioSeconds(); total != 5 {
		t.Fatalf("total settled audio seconds = %d, want exactly 5 (the provider-reported amount, once)", total)
	}
}

// I-3: a DEFINITE provider open failure (not the unknown timeout) must
// release the budget hold it just took — repeated failures may not
// accumulate dangling holds (budget-consuming DoS).
func TestVoiceOpenDefiniteFailureReleasesHold(t *testing.T) {
	h := newVoiceHarness(t)
	h.provider.openErr = errors.New("provider refused")
	w := h.postSession(t, `{"session_id":"sess-1","max_seconds":30}`)
	if w.Code != http.StatusBadGateway {
		t.Fatalf("definite open failure: %d %s", w.Code, w.Body.String())
	}
	if h.gate.beginCount() != 1 {
		t.Fatalf("begins = %d, want 1", h.gate.beginCount())
	}
	if h.releaser.count() != 1 {
		t.Fatalf("hold releases = %d, want exactly 1 (the unusable hold must be released)", h.releaser.count())
	}
	if opens, _ := h.provider.counts(); opens != 1 {
		t.Fatalf("provider opens = %d, want 1", opens)
	}
	// A follow-up create for another product session is unaffected.
	h.provider.openErr = nil
	w = h.postSession(t, `{"session_id":"sess-2","max_seconds":30}`)
	if w.Code != http.StatusOK {
		t.Fatalf("recovery create: %d %s", w.Code, w.Body.String())
	}
}

// I-4: a settlement failure at stop must NOT close the session row — the
// row stays settleable, the response reports settled:false, and a retried
// stop completes the settlement and then closes.
func TestVoiceStopSettlementFailureKeepsRowSettleable(t *testing.T) {
	h := newVoiceHarness(t)
	w := h.postSession(t, `{"session_id":"sess-1","max_seconds":30}`)
	id := voiceSessionIDOf(t, w)
	h.gate.finishErr = errors.New("settlement db hiccup")
	w = h.stopSession(t, id, "owner-a")
	if w.Code != http.StatusAccepted {
		t.Fatalf("stop with settle failure: %d %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "\"settled\":true") {
		t.Fatalf("response must not claim settled:true on a failed settlement: %s", w.Body.String())
	}
	row, err := h.store.GetOwnedVoiceSession(context.Background(), 7, "owner-a", id)
	if err != nil {
		t.Fatalf("row read: %v", err)
	}
	if row.State == repository.VoiceSessionStateClosed {
		t.Fatal("row was closed despite failed settlement — the settlement is now unreachable")
	}
	// Retry the stop once the settlement layer recovers.
	h.gate.finishErr = nil
	w = h.stopSession(t, id, "owner-a")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "\"settled\":true") {
		t.Fatalf("retry stop: %d %s", w.Code, w.Body.String())
	}
	row, err = h.store.GetOwnedVoiceSession(context.Background(), 7, "owner-a", id)
	if err != nil {
		t.Fatalf("row read: %v", err)
	}
	if row.State != repository.VoiceSessionStateClosed {
		t.Fatalf("row state = %s, want closed after successful settlement", row.State)
	}
	if total := h.gate.totalSettledAudioSeconds(); total != 0 {
		t.Fatalf("total settled = %d, want 0 (no usage reported)", total)
	}
}

// I-2: replaying the same transcription request id is idempotent under the
// REAL Begin semantics — a dispatched reservation key conflicts, so the
// replay must be answered from the persisted result row (W04 request
// reconciliation shape: same id returns the same result) BEFORE any new
// Begin/provider call. The harness fake now mirrors the real conflict, so
// this test fails against any implementation that re-Begins on replay.
func TestTranscriptionReplayServedFromResultRow(t *testing.T) {
	h := newVoiceHarness(t)
	h.transcriber.result = voice.TranscriptionResult{Text: "hello", AudioSeconds: 12}
	w := h.postTranscription(t, "owner-a", "req-1", "", []byte("audio"))
	if w.Code != http.StatusOK {
		t.Fatalf("first transcription: %d %s", w.Code, w.Body.String())
	}
	callsAfterFirst := h.transcriber.calls
	w = h.postTranscription(t, "owner-a", "req-1", "", []byte("audio"))
	if w.Code != http.StatusOK {
		t.Fatalf("replay: %d %s (the real Begin conflicts on a dispatched key; the replay must be served from the result row)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "hello") {
		t.Fatalf("replay must return the ORIGINAL text: %s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "\"replay\":true") {
		t.Fatalf("replay response must be marked as one: %s", w.Body.String())
	}
	if h.transcriber.calls != callsAfterFirst {
		t.Fatalf("transcriber calls = %d, the replay must not reach the provider", h.transcriber.calls)
	}
	if h.gate.beginCount() != 1 {
		t.Fatalf("begins = %d, the replay must not take a second hold", h.gate.beginCount())
	}
	if total := h.gate.totalSettledAudioSeconds(); total != 12 {
		t.Fatalf("total settled audio seconds = %d, want exactly 12", total)
	}
	// A failed first attempt is equally replayable with its failure.
	h.transcriber.err = voice.ErrTranscriptionFailed
	h.transcriber.result = voice.TranscriptionResult{AudioSeconds: 3}
	w = h.postTranscription(t, "owner-a", "req-fail", "", []byte("audio"))
	if w.Code != http.StatusBadGateway {
		t.Fatalf("failing first attempt: %d %s", w.Code, w.Body.String())
	}
	w = h.postTranscription(t, "owner-a", "req-fail", "", []byte("audio"))
	if w.Code != http.StatusBadGateway || !strings.Contains(w.Body.String(), "\"replay\":true") {
		t.Fatalf("failing replay: %d %s", w.Code, w.Body.String())
	}
	if total := h.gate.totalSettledAudioSeconds(); total != 15 {
		t.Fatalf("total settled = %d, want 15 (12 + 3, each exactly once)", total)
	}
}

// ---- I-2 integration: the REAL ExecutionGateService --------------------------

// integrationVoiceGateway satisfies domain.CommercialGateway minimally:
// Finalize only persists local rows (the settlement record and outbox
// event); the remote dispatch loop these tests never run owns the gateway.
type integrationVoiceGateway struct{}

func (integrationVoiceGateway) ApplyBenefit(context.Context, domain.BenefitRequest) (domain.BenefitReceipt, error) {
	return domain.BenefitReceipt{}, nil
}
func (integrationVoiceGateway) FindBenefit(context.Context, string) (domain.BenefitReceipt, error) {
	return domain.BenefitReceipt{}, nil
}
func (integrationVoiceGateway) RevokeBenefit(context.Context, string, domain.Credits) error {
	return nil
}
func (integrationVoiceGateway) Settle(context.Context, domain.Settlement) (domain.SettlementReceipt, error) {
	return domain.SettlementReceipt{ExternalID: "ext-1", Watermark: "wm-1"}, nil
}
func (integrationVoiceGateway) ConfirmSettlement(context.Context, string) (domain.SettlementReceipt, error) {
	return domain.SettlementReceipt{ExternalID: "ext-1", Watermark: "wm-1"}, nil
}

// TestTranscriptionReplayAgainstRealExecutionGate runs the replay case
// against the REAL commercial ExecutionGateService over sqlite: a dispatched
// reservation key conflicts on Begin (the exact production semantics the
// harness fake now mirrors), so the replay MUST be served from the durable
// result row. The second POST answers 200 with the original text, exactly
// one usage fact exists for the call, and exactly one reservation was ever
// taken.
func TestTranscriptionReplayAgainstRealExecutionGate(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&repocommercial.BudgetAccountRow{},
		&repocommercial.TaskBudgetRow{},
		&repocommercial.ReservationRow{},
		&repocommercial.BudgetLotRow{},
		&repocommercial.BudgetLotAllocationRow{},
		&repocommercial.UsageRow{},
		&repocommercial.UsageCurrentRow{},
		&repocommercial.OutboxEvent{},
		&commercialsvc.SettlementRecord{},
		&repository.VoiceSessionRow{},
		&repository.VoiceTranscriptionRow{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	now := time.Now().UTC()
	if err := db.Create(&repocommercial.BudgetAccountRow{TenantID: 7, VerifiedMicro: 10_000_000, VerifiedUntil: now.Add(time.Hour), Version: 1}).Error; err != nil {
		t.Fatalf("account: %v", err)
	}
	if err := db.Create(&repocommercial.TaskBudgetRow{TenantID: 7, RunID: "voice:", LimitMicro: 10_000_000, Deadline: now.Add(time.Hour), Version: 1}).Error; err != nil {
		t.Fatalf("task budget: %v", err)
	}
	if err := db.Create(&repocommercial.BudgetLotRow{TenantID: 7, LotID: "lot-1", RemainingMicro: 10_000_000, IssuedAt: now}).Error; err != nil {
		t.Fatalf("lot: %v", err)
	}
	gate, err := commercialsvc.NewExecutionGateService(db, integrationVoiceGateway{})
	if err != nil {
		t.Fatalf("gate: %v", err)
	}
	if _, err := gate.WithRates(func(string) (domain.PriceVersionRates, error) {
		return domain.PriceVersionRates{Version: "voice-it-v1", Rates: map[string]domain.DimensionRate{
			domain.DimensionAudioSeconds: {RateMicro: 100, Units: 1},
		}}, nil
	}); err != nil {
		t.Fatalf("rates: %v", err)
	}
	transcriber := &fakeVoiceTranscriber{result: voice.TranscriptionResult{Text: "real-gate", AudioSeconds: 12}}
	voiceHandler, err := NewMobileVoiceHandler(
		repository.NewVoiceSessionStore(db),
		repository.NewVoiceTranscriptionStore(db),
		&fakeVoiceProvider{},
		transcriber,
		gate,
		fakeVoiceRates{version: "voice-it-v1", upper: 1200},
	)
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	identity := func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, uint64(7))
		ctx = context.WithValue(ctx, types.UserIDContextKey, "owner-a")
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
	group := r.Group("/mobile/voice", identity)
	group.POST("/transcriptions", voiceHandler.TranscribeAudio)

	post := func(requestID string) *httptest.ResponseRecorder {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		_ = writer.WriteField("request_id", requestID)
		part, _ := writer.CreateFormFile("audio", "capture.m4a")
		_, _ = part.Write([]byte("audio"))
		_ = writer.Close()
		req := httptest.NewRequest(http.MethodPost, "/mobile/voice/transcriptions", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	w := post("req-9")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "real-gate") {
		t.Fatalf("first: %d %s", w.Code, w.Body.String())
	}
	w = post("req-9")
	if w.Code != http.StatusOK {
		t.Fatalf("replay under the real gate: %d %s (a dispatched key conflicts on Begin; the result row must answer)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "real-gate") || !strings.Contains(w.Body.String(), "\"replay\":true") {
		t.Fatalf("replay body: %s", w.Body.String())
	}
	var facts, reservations int64
	callID := voice.TranscriptionCallID(7, "req-9")
	if err := db.Model(&repocommercial.UsageRow{}).Where("tenant_id = ? AND call_id = ?", uint64(7), callID).Count(&facts).Error; err != nil {
		t.Fatalf("facts: %v", err)
	}
	if err := db.Model(&repocommercial.ReservationRow{}).Where("tenant_id = ? AND key = ?", uint64(7), "voice_tx:"+callID).Count(&reservations).Error; err != nil {
		t.Fatalf("reservations: %v", err)
	}
	if facts != 1 {
		t.Fatalf("usage facts = %d, want exactly 1", facts)
	}
	if reservations != 1 {
		t.Fatalf("reservations = %d, want exactly 1", reservations)
	}
	// The settle-charged delta equals the provider's 12 seconds at 100
	// micro/second — charged once, from the one durable settlement record.
	var record commercialsvc.SettlementRecord
	if err := db.Where("tenant_id = ? AND call_id = ?", uint64(7), callID).First(&record).Error; err != nil {
		t.Fatalf("settlement record: %v", err)
	}
	if record.AmountMicro != 1200 {
		t.Fatalf("settled micro = %d, want 1200 (12s x 100, once)", record.AmountMicro)
	}
}

// ---- R1-N1 (review round 2): settlement-failed transcription replays -----

// R1-N1: when the transcription's gate.Finish fails, the result row must
// not act as a settled short-circuit. A replay while the settlement never
// landed must answer settled:false (never the hardcoded lie), must not call
// the provider again, and once the settlement layer recovers the replay
// COMPLETES the settlement exactly once — the I-4 keep-settleable semantics
// extended to the transcription plane.
func TestTranscriptionSettlementFailureReplayRetriesSettlement(t *testing.T) {
	h := newVoiceHarness(t)
	h.gate.finishErr = errors.New("settlement hiccup")
	h.transcriber.result = voice.TranscriptionResult{Text: "probe", AudioSeconds: 4}
	w := h.postTranscription(t, "owner-a", "req-1", "", []byte("audio"))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "\"settled\":false") {
		t.Fatalf("first: %d %s", w.Code, w.Body.String())
	}
	// Replay while the settlement is still failing: honest settled:false,
	// no second provider call, no second hold.
	w = h.postTranscription(t, "owner-a", "req-1", "", []byte("audio"))
	if w.Code != http.StatusOK {
		t.Fatalf("replay while down: %d %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "\"settled\":false") {
		t.Fatalf("replay must not claim settled while the settlement never landed: %s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "probe") {
		t.Fatalf("replay still returns the original text: %s", w.Body.String())
	}
	if h.transcriber.calls != 1 {
		t.Fatalf("transcriber calls = %d, the replay must not reach the provider", h.transcriber.calls)
	}
	if total := h.gate.totalSettledAudioSeconds(); total != 0 {
		t.Fatalf("total settled = %d, want 0 while settlement is down", total)
	}
	// Settlement layer recovers: the replay completes the settlement.
	h.gate.finishErr = nil
	w = h.postTranscription(t, "owner-a", "req-1", "", []byte("audio"))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "\"settled\":true") {
		t.Fatalf("recovery replay: %d %s", w.Code, w.Body.String())
	}
	if total := h.gate.totalSettledAudioSeconds(); total != 4 {
		t.Fatalf("total settled = %d, want exactly 4 after the recovered replay", total)
	}
	// And stays exactly 4 on further replays.
	w = h.postTranscription(t, "owner-a", "req-1", "", []byte("audio"))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "\"settled\":true") {
		t.Fatalf("settled replay: %d %s", w.Code, w.Body.String())
	}
	if total := h.gate.totalSettledAudioSeconds(); total != 4 {
		t.Fatalf("total settled = %d, want exactly 4 (stable)", total)
	}
	if h.transcriber.calls != 1 {
		t.Fatalf("transcriber calls = %d, no replay may reach the provider", h.transcriber.calls)
	}
}

// R1-N1 against the REAL ExecutionGateService: admission passes while the
// gate's own rate resolver is unavailable (the production default until
// pricing administration is wired) — Finish fails, the row persists
// unsettled, and wiring rates later lets the replay complete the charge.
func TestTranscriptionSettlementRecoveryAgainstRealGate(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&repocommercial.BudgetAccountRow{},
		&repocommercial.TaskBudgetRow{},
		&repocommercial.ReservationRow{},
		&repocommercial.BudgetLotRow{},
		&repocommercial.BudgetLotAllocationRow{},
		&repocommercial.UsageRow{},
		&repocommercial.UsageCurrentRow{},
		&repocommercial.OutboxEvent{},
		&commercialsvc.SettlementRecord{},
		&repository.VoiceSessionRow{},
		&repository.VoiceTranscriptionRow{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	now := time.Now().UTC()
	if err := db.Create(&repocommercial.BudgetAccountRow{TenantID: 7, VerifiedMicro: 10_000_000, VerifiedUntil: now.Add(time.Hour), Version: 1}).Error; err != nil {
		t.Fatalf("account: %v", err)
	}
	if err := db.Create(&repocommercial.TaskBudgetRow{TenantID: 7, RunID: "voice:", LimitMicro: 10_000_000, Deadline: now.Add(time.Hour), Version: 1}).Error; err != nil {
		t.Fatalf("task budget: %v", err)
	}
	if err := db.Create(&repocommercial.BudgetLotRow{TenantID: 7, LotID: "lot-1", RemainingMicro: 10_000_000, IssuedAt: now}).Error; err != nil {
		t.Fatalf("lot: %v", err)
	}
	// No WithRates: the gate's default unavailableRates makes Finish fail —
	// exactly the review's "admission passed, settlement cannot price" gap.
	gate, err := commercialsvc.NewExecutionGateService(db, integrationVoiceGateway{})
	if err != nil {
		t.Fatalf("gate: %v", err)
	}
	transcriber := &fakeVoiceTranscriber{result: voice.TranscriptionResult{Text: "rec", AudioSeconds: 4}}
	voiceHandler, err := NewMobileVoiceHandler(
		repository.NewVoiceSessionStore(db),
		repository.NewVoiceTranscriptionStore(db),
		&fakeVoiceProvider{},
		transcriber,
		gate,
		fakeVoiceRates{version: "voice-it-v1", upper: 1200},
	)
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	identity := func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, uint64(7))
		ctx = context.WithValue(ctx, types.UserIDContextKey, "owner-a")
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
	group := r.Group("/mobile/voice", identity)
	group.POST("/transcriptions", voiceHandler.TranscribeAudio)
	post := func(requestID string) *httptest.ResponseRecorder {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		_ = writer.WriteField("request_id", requestID)
		part, _ := writer.CreateFormFile("audio", "capture.m4a")
		_, _ = part.Write([]byte("audio"))
		_ = writer.Close()
		req := httptest.NewRequest(http.MethodPost, "/mobile/voice/transcriptions", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	w := post("req-rec")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "\"settled\":false") {
		t.Fatalf("unsettled first: %d %s", w.Code, w.Body.String())
	}
	w = post("req-rec")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "\"settled\":false") {
		t.Fatalf("replay must stay honest while unpriced: %d %s", w.Code, w.Body.String())
	}
	// Pricing administration arrives: the replay completes the settlement.
	if _, err := gate.WithRates(func(string) (domain.PriceVersionRates, error) {
		return domain.PriceVersionRates{Version: "voice-it-v1", Rates: map[string]domain.DimensionRate{
			domain.DimensionAudioSeconds: {RateMicro: 100, Units: 1},
		}}, nil
	}); err != nil {
		t.Fatalf("rates: %v", err)
	}
	w = post("req-rec")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "\"settled\":true") {
		t.Fatalf("recovered replay: %d %s", w.Code, w.Body.String())
	}
	callID := voice.TranscriptionCallID(7, "req-rec")
	var facts int64
	if err := db.Model(&repocommercial.UsageRow{}).Where("tenant_id = ? AND call_id = ?", uint64(7), callID).Count(&facts).Error; err != nil {
		t.Fatalf("facts: %v", err)
	}
	if facts != 1 {
		t.Fatalf("usage facts = %d, want exactly 1", facts)
	}
	var record commercialsvc.SettlementRecord
	if err := db.Where("tenant_id = ? AND call_id = ?", uint64(7), callID).First(&record).Error; err != nil {
		t.Fatalf("settlement record: %v", err)
	}
	if record.AmountMicro != 400 {
		t.Fatalf("settled micro = %d, want 400 (4s x 100, exactly once)", record.AmountMicro)
	}
}
