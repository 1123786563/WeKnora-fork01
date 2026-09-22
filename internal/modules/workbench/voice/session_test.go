package voice

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestVoiceCannotOutliveBudget is the W30 booklet acceptance: a grant never
// outlives the task deadline and an unfunded request is refused outright.
func TestVoiceCannotOutliveBudget(t *testing.T) {
	now := time.Now()
	deadline := now.Add(30 * time.Second)
	expiry, err := AuthorizeVoice(now, deadline, 1)
	if err != nil || expiry.After(deadline) {
		t.Fatal("voice exceeds task deadline")
	}
	if _, err = AuthorizeVoice(now, deadline, 0); err == nil {
		t.Fatal("unfunded voice granted")
	}
	if _, err = AuthorizeVoice(now, deadline, -5); err == nil {
		t.Fatal("negative-budget voice granted")
	}
}

// TestVoiceDeadlineAlreadyPassedIsRefused: an expired (or zero) deadline
// never grants, regardless of the budget presented.
func TestVoiceDeadlineAlreadyPassedIsRefused(t *testing.T) {
	now := time.Now()
	if _, err := AuthorizeVoice(now, now.Add(-time.Second), 10); err == nil {
		t.Fatal("expired deadline granted voice")
	}
	if _, err := AuthorizeVoice(now, now, 10); err == nil {
		t.Fatal("zero deadline granted voice")
	}
}

// TestVoiceGrantNeverExceedsWindow: even a far deadline caps the grant at
// the one-minute window; a nearer deadline wins over the cap.
func TestVoiceGrantNeverExceedsWindow(t *testing.T) {
	now := time.Now()
	expiry, err := AuthorizeVoice(now, now.Add(time.Hour), 1000)
	if err != nil {
		t.Fatalf("valid voice refused: %v", err)
	}
	if expiry.Sub(now) > GrantWindow {
		t.Fatal("grant outlives the grant window")
	}
	if !expiry.Equal(now.Add(GrantWindow)) {
		t.Fatal("far deadline must yield exactly the window cap")
	}
}

// TestUnconfiguredProviderFailsClosed: without the server-side long-lived
// provider key no session may be minted — a missing real provider is a hard
// stop (blocked-env), never a fabricated grant.
func TestUnconfiguredProviderFailsClosed(t *testing.T) {
	p, err := NewManagedProvider(Config{})
	if err != nil {
		t.Fatalf("constructing unconfigured provider failed: %v", err)
	}
	if _, err := p.Open(context.Background(), "ref", 30); !errors.Is(err, ErrProviderUnavailable) {
		t.Fatalf("unconfigured provider opened a session: %v", err)
	}
	if err := p.Close(context.Background(), "ref"); !errors.Is(err, ErrProviderUnavailable) {
		t.Fatalf("unconfigured provider closed a session: %v", err)
	}
	if _, err := p.Transcribe(context.Background(), TranscriptionRequest{Audio: []byte("x")}); !errors.Is(err, ErrProviderUnavailable) {
		t.Fatalf("unconfigured provider transcribed: %v", err)
	}
}

// TestProxyGrantNeverCarriesLongLivedKey: a provider without short-lived
// token support gets the server-side media-proxy ticket — an HMAC token the
// product signs itself. The provider's long-lived key must never appear in
// the grant (nor anywhere else handed to the phone).
func TestProxyGrantNeverCarriesLongLivedKey(t *testing.T) {
	const longLived = "sk-provider-long-lived-secret"
	p, err := NewManagedProvider(Config{
		LongLivedAPIKey: longLived,
		Model:           "voice-1",
		SigningKey:      "0123456789abcdef",
		Now:             func() time.Time { return time.Unix(1758000000, 0) },
	})
	if err != nil {
		t.Fatalf("provider config rejected: %v", err)
	}
	grant, err := p.Open(context.Background(), "tenant/owner/sess", 30)
	if err != nil {
		t.Fatalf("proxy open failed: %v", err)
	}
	if grant.Token == "" || strings.Contains(grant.Token, longLived) {
		t.Fatalf("proxy grant leaked or dropped the token boundary: %q", grant.Token)
	}
	if grant.MaxSeconds != 30 {
		t.Fatalf("grant max seconds = %d, want 30", grant.MaxSeconds)
	}
	if !grant.ExpiresAt.Equal(time.Unix(1758000000, 0).Add(30 * time.Second)) {
		t.Fatalf("proxy grant expiry %v exceeds its own max window", grant.ExpiresAt)
	}
}

// TestProxyGrantIsBoundToItsSession: a proxy ticket minted for one session
// must not verify for another session or after expiry.
func TestProxyGrantIsBoundToItsSession(t *testing.T) {
	p, err := NewManagedProvider(Config{
		LongLivedAPIKey: "sk-x",
		Model:           "voice-1",
		SigningKey:      "0123456789abcdef",
		Now:             func() time.Time { return time.Unix(1758000000, 0) },
	})
	if err != nil {
		t.Fatalf("provider config rejected: %v", err)
	}
	grant, err := p.Open(context.Background(), "tenant/owner/a", 10)
	if err != nil {
		t.Fatalf("proxy open failed: %v", err)
	}
	if _, err := p.VerifyProxyToken(grant.Token, "tenant/owner/b", time.Unix(1758000000, 0).Add(time.Second)); err == nil {
		t.Fatal("proxy token verified for a different session")
	}
	if _, err := p.VerifyProxyToken(grant.Token, "tenant/owner/a", time.Unix(1758000000, 0).Add(11*time.Second)); err == nil {
		t.Fatal("expired proxy token verified")
	}
}

// TestTokenProviderOpenUnknownOnTimeout: when the provider's token endpoint
// times out the outcome is UNKNOWN — the provider may have opened a
// billable session — so Open must surface ErrProviderOpenUnknown instead of
// a plain failure the caller could retry into a second charged session.
func TestTokenProviderOpenUnknownOnTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		_, _ = w.Write([]byte(`{"token":"ephemeral"}`))
	}))
	defer srv.Close()
	p, err := NewManagedProvider(Config{
		LongLivedAPIKey: "sk-x",
		Model:           "voice-1",
		TokenEndpoint:   srv.URL,
		Client:          &http.Client{Timeout: 20 * time.Millisecond},
	})
	if err != nil {
		t.Fatalf("provider config rejected: %v", err)
	}
	if _, err := p.Open(context.Background(), "tenant/owner/sess", 30); !errors.Is(err, ErrProviderOpenUnknown) {
		t.Fatalf("timed-out open classified %v, want ErrProviderOpenUnknown", err)
	}
}

// TestTranscribeProxiesThroughServerKey: the transcription path is the
// server-side media proxy — the phone's audio goes out through the
// server-held long-lived key and only the text comes back.
func TestTranscribeProxiesThroughServerKey(t *testing.T) {
	const longLived = "sk-provider-long-lived-secret"
	var gotAuth, gotModel string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_ = r.ParseMultipartForm(1 << 20)
		gotModel = r.FormValue("model")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"text":"hello","duration":12}`))
	}))
	defer srv.Close()
	p, err := NewManagedProvider(Config{
		LongLivedAPIKey:   longLived,
		Model:             "voice-1",
		TranscribeEndpoint: srv.URL,
	})
	if err != nil {
		t.Fatalf("provider config rejected: %v", err)
	}
	result, err := p.Transcribe(context.Background(), TranscriptionRequest{Audio: []byte("audio-bytes"), MimeType: "audio/mp4"})
	if err != nil {
		t.Fatalf("transcription failed: %v", err)
	}
	if result.Text != "hello" || result.AudioSeconds != 12 {
		t.Fatalf("transcription result = %+v", result)
	}
	if gotAuth != "Bearer "+longLived || gotModel != "voice-1" {
		t.Fatalf("upstream call missed the server credential/model: auth=%q model=%q", gotAuth, gotModel)
	}
}

// TestTranscribeFailureKeepsPartialUsage: a provider error that still
// reports consumed seconds must hand those seconds back — failed
// transcription still settles the real cost.
func TestTranscribeFailureKeepsPartialUsage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"partial","duration":3}`))
	}))
	defer srv.Close()
	p, err := NewManagedProvider(Config{
		LongLivedAPIKey:   "sk-x",
		Model:             "voice-1",
		TranscribeEndpoint: srv.URL,
	})
	if err != nil {
		t.Fatalf("provider config rejected: %v", err)
	}
	result, err := p.Transcribe(context.Background(), TranscriptionRequest{Audio: []byte("audio")})
	if err == nil {
		t.Fatal("failed transcription reported success")
	}
	if result.AudioSeconds != 3 {
		t.Fatalf("partial usage lost: %+v", result)
	}
}
