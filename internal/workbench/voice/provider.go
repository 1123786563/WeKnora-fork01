package voice

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var (
	// ErrProviderUnavailable is the fail-closed answer of a provider whose
	// server-side wiring (long-lived key / endpoints) is not configured:
	// no session is minted and nothing is charged. Real-provider evidence
	// stays blocked-env until a live service is configured.
	ErrProviderUnavailable = errors.New("voice provider unconfigured")
	// ErrProviderOpenUnknown reports a token-endpoint outcome that could
	// not be observed (timeout, transport break). The provider may have
	// opened a billable session, so the caller must record the session as
	// unknown and reconcile it — never blindly open another.
	ErrProviderOpenUnknown = errors.New("voice provider open outcome unknown")
	// ErrProviderConfig rejects an internally inconsistent configuration
	// (for example a signing key below the minimum length).
	ErrProviderConfig = errors.New("voice provider config invalid")
	// ErrProxyTokenInvalid rejects a media-proxy ticket whose signature,
	// session binding, or expiry does not verify.
	ErrProxyTokenInvalid = errors.New("voice proxy token invalid")
	// ErrTranscriptionFailed reports a provider transcription error; the
	// returned TranscriptionResult may still carry the partially consumed
	// AudioSeconds that must be settled.
	ErrTranscriptionFailed = errors.New("voice transcription failed")
)

// providerGrantMaxSeconds caps one grant's requested max duration so the
// ticket TTL itself stays short-lived even when a caller asks for more.
const providerGrantMaxSeconds = 600

// proxySigningKeyMin is the smallest accepted media-proxy signing key.
const proxySigningKeyMin = 16

// Config is the server-held provider wiring. LongLivedAPIKey never leaves
// the server process: token minting and transcription proxying both happen
// here, behind the product's authenticated endpoints.
type Config struct {
	// LongLivedAPIKey is the provider key held only by the server.
	LongLivedAPIKey string
	// Model names the provider voice model.
	Model string
	// TokenEndpoint, when set, is the provider endpoint that mints
	// short-lived session tokens (ephemeral-token providers). Unset means
	// the provider has no short-lived token: sessions then ride the
	// server-side media proxy with an HMAC ticket (see Open).
	TokenEndpoint string
	// TranscribeEndpoint is the provider transcription endpoint proxied by
	// the product's own /mobile/voice/transcriptions route.
	TranscribeEndpoint string
	// SigningKey signs the server-side media-proxy tickets (>=16 bytes).
	SigningKey string
	// Client is the outbound HTTP client (a default with a short timeout
	// is used when nil — the token mint timeout is what makes open
	// outcomes "unknown" rather than slow).
	Client *http.Client
	// Now overrides the clock for tests.
	Now func() time.Time
}

// VoiceProvider is the session lifecycle boundary of one voice provider.
type VoiceProvider interface {
	// Open mints the short-lived grant of one session reference. A
	// transport-level uncertainty MUST surface as ErrProviderOpenUnknown.
	Open(ctx context.Context, sessionRef string, maxSeconds int) (Grant, error)
	// Close ends the provider-side session of one session reference; it is
	// idempotent per reference.
	Close(ctx context.Context, sessionRef string) error
}

// TranscriptionRequest is one proxied transcription: the phone's captured
// audio plus its content type.
type TranscriptionRequest struct {
	SessionRef string
	Audio      []byte
	MimeType   string
	Locale     string
}

// TranscriptionResult carries the provider answer: the recognized text and
// the provider-reported audio seconds used for settlement. AudioSeconds is
// the trusted metering input — the client never self-reports it.
type TranscriptionResult struct {
	Text         string
	AudioSeconds int64
}

// Transcriber proxies one audio transcription through the server-held
// provider credential. A failed call may still return a non-nil result
// carrying partially consumed seconds.
type Transcriber interface {
	Transcribe(ctx context.Context, req TranscriptionRequest) (TranscriptionResult, error)
}

// ManagedProvider is the production VoiceProvider + Transcriber over a
// real (HTTP) voice provider. Providers split into two modes:
//
//   - ephemeral-token mode (TokenEndpoint set): Open asks the provider to
//     mint the short-lived token; the long-lived key is used only for that
//     mint call, server-side.
//   - media-proxy mode (TokenEndpoint unset): the provider offers no
//     short-lived token, so Open signs an HMAC ticket the product's own
//     media proxy verifies. The long-lived key still never leaves the
//     server; the phone only ever holds the ticket.
//
// An unconfigured provider (missing key or model) fails closed with
// ErrProviderUnavailable on every operation — blocked-env deployments
// refuse charged voice instead of fabricating a free one.
type ManagedProvider struct {
	cfg    Config
	client *http.Client
	now    func() time.Time
}

// NewManagedProvider validates the configuration shape and returns the
// provider. Construction always succeeds for an EMPTY config (fail-closed
// behavior answers on call), so boot never depends on the provider being
// configured; only internally inconsistent partial configs are rejected.
func NewManagedProvider(cfg Config) (*ManagedProvider, error) {
	if cfg.TokenEndpoint == "" && cfg.TranscribeEndpoint == "" && cfg.LongLivedAPIKey == "" && cfg.Model == "" && cfg.SigningKey == "" {
		return &ManagedProvider{cfg: cfg, client: cfg.Client, now: cfg.Now}, nil
	}
	if cfg.SigningKey != "" && len(cfg.SigningKey) < proxySigningKeyMin {
		return nil, ErrProviderConfig
	}
	if cfg.TokenEndpoint != "" || cfg.TranscribeEndpoint != "" {
		if strings.TrimSpace(cfg.LongLivedAPIKey) == "" || strings.TrimSpace(cfg.Model) == "" {
			return nil, ErrProviderConfig
		}
	}
	if cfg.TokenEndpoint == "" && cfg.SigningKey != "" && strings.TrimSpace(cfg.LongLivedAPIKey) == "" {
		return nil, ErrProviderConfig
	}
	client := cfg.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	return &ManagedProvider{cfg: cfg, client: client, now: now}, nil
}

func (p *ManagedProvider) configured() bool {
	return p != nil &&
		strings.TrimSpace(p.cfg.LongLivedAPIKey) != "" &&
		strings.TrimSpace(p.cfg.Model) != ""
}

// Open mints the grant of one session reference.
func (p *ManagedProvider) Open(ctx context.Context, sessionRef string, maxSeconds int) (Grant, error) {
	if !p.configured() {
		return Grant{}, ErrProviderUnavailable
	}
	if maxSeconds <= 0 || maxSeconds > providerGrantMaxSeconds {
		return Grant{}, fmt.Errorf("%w: max_seconds %d", ErrProviderConfig, maxSeconds)
	}
	now := p.now().UTC()
	expiry := now.Add(time.Duration(maxSeconds) * time.Second)
	if p.cfg.TokenEndpoint != "" {
		return p.openEphemeral(ctx, sessionRef, expiry)
	}
	return p.openProxyTicket(sessionRef, expiry)
}

// openEphemeral asks the provider to mint a short-lived token. Any
// transport-level uncertainty (timeout, connection break) is UNKNOWN: the
// provider may have opened a billable session.
func (p *ManagedProvider) openEphemeral(ctx context.Context, sessionRef string, expiry time.Time) (Grant, error) {
	body, err := json.Marshal(map[string]any{
		"model":       p.cfg.Model,
		"session":     sessionRef,
		"max_seconds": int(expiry.Sub(p.now()).Seconds()),
	})
	if err != nil {
		return Grant{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.cfg.TokenEndpoint, bytes.NewReader(body))
	if err != nil {
		return Grant{}, err
	}
	req.Header.Set("Authorization", "Bearer "+p.cfg.LongLivedAPIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return Grant{}, fmt.Errorf("%w: %v", ErrProviderOpenUnknown, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		// A definitive non-200 answer is a refusal, not an unknown.
		return Grant{}, fmt.Errorf("voice provider token endpoint status %d", resp.StatusCode)
	}
	var payload struct {
		Token     string `json:"token"`
		ExpiresAt int64  `json:"expires_at"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payload); err != nil {
		return Grant{}, fmt.Errorf("%w: %v", ErrProviderOpenUnknown, err)
	}
	if strings.TrimSpace(payload.Token) == "" {
		return Grant{}, fmt.Errorf("%w: empty token", ErrProviderOpenUnknown)
	}
	grant := Grant{Token: payload.Token, ExpiresAt: expiry, MaxSeconds: int(expiry.Sub(p.now()).Seconds())}
	if payload.ExpiresAt > 0 {
		providerExpiry := time.Unix(payload.ExpiresAt, 0).UTC()
		if providerExpiry.Before(grant.ExpiresAt) {
			grant.ExpiresAt = providerExpiry
		}
	}
	return grant, nil
}

// openProxyTicket signs the media-proxy ticket for providers without
// short-lived token support. The ticket binds the session reference and
// expiry with an HMAC over a non-secret payload — the long-lived key is
// never embedded.
func (p *ManagedProvider) openProxyTicket(sessionRef string, expiry time.Time) (Grant, error) {
	if len(p.cfg.SigningKey) < proxySigningKeyMin {
		return Grant{}, ErrProviderUnavailable
	}
	maxSeconds := int(expiry.Sub(p.now()).Seconds())
	payload := proxyTicketPayload(sessionRef, expiry)
	mac := hmac.New(sha256.New, []byte(p.cfg.SigningKey))
	mac.Write([]byte(payload))
	token := "wvp1." + base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return Grant{Token: token, ExpiresAt: expiry, MaxSeconds: maxSeconds}, nil
}

func proxyTicketPayload(sessionRef string, expiry time.Time) string {
	return sessionRef + "|" + strconv.FormatInt(expiry.Unix(), 10)
}

// VerifyProxyToken checks a media-proxy ticket against its session
// reference at a point in time. It is the check the server-side media
// proxy performs on every proxied frame.
func (p *ManagedProvider) VerifyProxyToken(token, sessionRef string, now time.Time) (time.Time, error) {
	if p == nil || len(p.cfg.SigningKey) < proxySigningKeyMin || !strings.HasPrefix(token, "wvp1.") {
		return time.Time{}, ErrProxyTokenInvalid
	}
	rest := strings.TrimPrefix(token, "wvp1.")
	parts := strings.Split(rest, ".")
	if len(parts) != 2 {
		return time.Time{}, ErrProxyTokenInvalid
	}
	rawPayload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return time.Time{}, ErrProxyTokenInvalid
	}
	rawMAC, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return time.Time{}, ErrProxyTokenInvalid
	}
	mac := hmac.New(sha256.New, []byte(p.cfg.SigningKey))
	mac.Write(rawPayload)
	if !hmac.Equal(mac.Sum(nil), rawMAC) {
		return time.Time{}, ErrProxyTokenInvalid
	}
	payload := string(rawPayload)
	idx := strings.LastIndex(payload, "|")
	if idx < 0 {
		return time.Time{}, ErrProxyTokenInvalid
	}
	unix, err := strconv.ParseInt(payload[idx+1:], 10, 64)
	if err != nil {
		return time.Time{}, ErrProxyTokenInvalid
	}
	if payload[:idx] != sessionRef {
		return time.Time{}, ErrProxyTokenInvalid
	}
	expiry := time.Unix(unix, 0).UTC()
	if !now.Before(expiry) {
		return time.Time{}, ErrProxyTokenInvalid
	}
	return expiry, nil
}

// Close ends the provider-side session. Ephemeral mode asks the provider;
// proxy mode has nothing remote to close (the ticket expires on its own).
// Failures are reported but Close stays idempotent — settlement runs
// regardless of the close outcome.
func (p *ManagedProvider) Close(ctx context.Context, sessionRef string) error {
	if !p.configured() {
		return ErrProviderUnavailable
	}
	if p.cfg.TokenEndpoint == "" {
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, strings.TrimRight(p.cfg.TokenEndpoint, "/")+"/"+base64.RawURLEncoding.EncodeToString([]byte(sessionRef)), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+p.cfg.LongLivedAPIKey)
	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("voice provider close unknown: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("voice provider close status %d", resp.StatusCode)
	}
	return nil
}

// Transcribe proxies one audio capture to the provider with the
// server-held long-lived key. On a provider failure the returned result
// still carries any provider-reported duration so the caller settles the
// real consumed cost.
func (p *ManagedProvider) Transcribe(ctx context.Context, req TranscriptionRequest) (TranscriptionResult, error) {
	if !p.configured() || strings.TrimSpace(p.cfg.TranscribeEndpoint) == "" {
		return TranscriptionResult{}, ErrProviderUnavailable
	}
	if len(req.Audio) == 0 {
		return TranscriptionResult{}, fmt.Errorf("%w: empty audio", ErrTranscriptionFailed)
	}
	form := &bytes.Buffer{}
	writer := multipart.NewWriter(form)
	mimeType := strings.TrimSpace(req.MimeType)
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	if err := writer.WriteField("model", p.cfg.Model); err != nil {
		return TranscriptionResult{}, err
	}
	if req.Locale != "" {
		if err := writer.WriteField("locale", req.Locale); err != nil {
			return TranscriptionResult{}, err
		}
	}
	part, err := writer.CreateFormFile("audio", "capture."+mediaExtension(mimeType))
	if err != nil {
		return TranscriptionResult{}, err
	}
	if _, err := part.Write(req.Audio); err != nil {
		return TranscriptionResult{}, err
	}
	if err := writer.Close(); err != nil {
		return TranscriptionResult{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.cfg.TranscribeEndpoint, bytes.NewReader(form.Bytes()))
	if err != nil {
		return TranscriptionResult{}, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+p.cfg.LongLivedAPIKey)
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return TranscriptionResult{}, fmt.Errorf("%w: %v", ErrTranscriptionFailed, err)
	}
	defer func() { _ = resp.Body.Close() }()
	var payload struct {
		Text     string `json:"text"`
		Duration int64  `json:"duration"`
		Error    string `json:"error"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&payload); err != nil {
		return TranscriptionResult{}, fmt.Errorf("%w: %v", ErrTranscriptionFailed, err)
	}
	result := TranscriptionResult{Text: payload.Text, AudioSeconds: payload.Duration}
	if resp.StatusCode != http.StatusOK {
		return result, fmt.Errorf("%w: %s", ErrTranscriptionFailed, payload.Error)
	}
	if result.AudioSeconds < 0 {
		result.AudioSeconds = 0
	}
	return result, nil
}

func mediaExtension(mimeType string) string {
	switch {
	case strings.Contains(mimeType, "wav"):
		return "wav"
	case strings.Contains(mimeType, "webm"):
		return "webm"
	case strings.Contains(mimeType, "mpeg"), strings.Contains(mimeType, "mp3"):
		return "mp3"
	case strings.Contains(mimeType, "ogg"):
		return "ogg"
	default:
		return "m4a"
	}
}
