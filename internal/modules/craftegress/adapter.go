package craftegress

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// craftModelActivityHeader mirrors the gateway contract
// (internal/handler/craft_model_gateway.go): an opaque stable per-activity
// identity with charset [a-zA-Z0-9._:-], at most 128 bytes.
const craftModelActivityHeader = "X-Craft-Activity-ID"

const (
	craftEgressDefaultMaxBodyBytes = 16 << 20
	// craftEgressDefaultTimeout must stay AT OR ABOVE the Craft model
	// gateway's own forward budget (ForwardTimeout default 5 minutes in
	// internal/handler/craft_model_gateway.go): a shorter adapter timeout
	// would abort slow-but-legitimate generations mid-flight, park the
	// attempt as unknown-outcome, and deadlock same-fingerprint retries on
	// the gateway's ACTIVITY_UNRESOLVED 409 until manual reconciliation.
	// The one-minute headroom absorbs buffering between the two hops.
	craftEgressDefaultTimeout = 6 * time.Minute
)

// DefaultForwardTimeout exposes the adapter's default forward budget so
// operators tooling around cmd/craft-egress-adapter share one source of
// truth with the gateway-budget linkage documented above.
func DefaultForwardTimeout() time.Duration { return craftEgressDefaultTimeout }

// CraftEgressAdapterConfig assembles the per-Run egress adapter. The gateway
// base URL is operator configuration (never client input) and must be http or
// https; the credential is supplied by the environment at assembly time and is
// never journaled.
type CraftEgressAdapterConfig struct {
	GatewayBaseURL string
	Credential     string
	JournalPath    string
	MaxBodyBytes   int64
	ForwardTimeout time.Duration
	Now            func() time.Time
	// Transport is optional (tests inject one); production uses a hardened
	// default transport bound to ForwardTimeout.
	Transport http.RoundTripper
	// AllowPrivateTarget mirrors ValidateGatewayTarget's flag for the
	// RUNTIME dial check: when false, every actually-dialed gateway IP is
	// re-validated against the private/loopback/reserved ranges at connect
	// time, closing the startup-DNS vs runtime-DNS rebinding window.
	AllowPrivateTarget bool
}

// CraftEgressAdapter is the runtime-side producer of per-physical-attempt
// model activity identities. OpenCode (or any in-sandbox provider client)
// targets this adapter as its only provider endpoint; the adapter owns the
// durable attempt journal and injects X-Craft-Activity-ID toward the Craft
// model gateway, which stays fail-closed without it.
type CraftEgressAdapter struct {
	gateway    *url.URL
	credential string
	journal    *CraftEgressAttemptJournal
	client     *http.Client
	maxBody    int64
}

func NewCraftEgressAdapter(config CraftEgressAdapterConfig) (*CraftEgressAdapter, error) {
	if strings.TrimSpace(config.GatewayBaseURL) == "" {
		return nil, fmt.Errorf("craftegress: gateway base URL is required")
	}
	target, err := url.Parse(config.GatewayBaseURL)
	if err != nil {
		return nil, fmt.Errorf("craftegress: gateway base URL: %w", err)
	}
	if target.Scheme != "http" && target.Scheme != "https" {
		return nil, fmt.Errorf("craftegress: gateway base URL must be http or https, got %q", target.Scheme)
	}
	if target.Host == "" {
		return nil, fmt.Errorf("craftegress: gateway base URL requires a host")
	}
	if strings.TrimSpace(config.Credential) == "" {
		return nil, fmt.Errorf("craftegress: execution credential is required")
	}
	journal, err := OpenCraftEgressAttemptJournal(config.JournalPath)
	if err != nil {
		return nil, err
	}
	if config.Now != nil {
		journal.now = config.Now
	}
	maxBody := config.MaxBodyBytes
	if maxBody <= 0 {
		maxBody = craftEgressDefaultMaxBodyBytes
	}
	timeout := config.ForwardTimeout
	if timeout <= 0 {
		timeout = craftEgressDefaultTimeout
	}
	transport := config.Transport
	if transport == nil {
		dialer := &net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}
		// Connect-time host re-validation: the startup
		// ValidateGatewayTarget DNS answer is stale the moment a short TTL
		// expires, and every request dials fresh. Rejecting the
		// actually-connected IP keeps Bearer-bearing forwards off
		// loopback/private/cloud-metadata targets under rebinding.
		if !config.AllowPrivateTarget {
			dialer.Control = func(network, address string, _ syscall.RawConn) error {
				host, _, err := net.SplitHostPort(address)
				if err != nil {
					return fmt.Errorf("craftegress: dial address %q: %w", address, err)
				}
				ip := net.ParseIP(host)
				if ip == nil {
					return fmt.Errorf("craftegress: dial address %q is not an IP", address)
				}
				return rejectPrivateIP(ip)
			}
		}
		transport = &http.Transport{
			Proxy:                 nil,
			DialContext:           dialer.DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		}
	}
	return &CraftEgressAdapter{
		gateway:    target,
		credential: config.Credential,
		journal:    journal,
		client: &http.Client{
			Transport: transport,
			Timeout:   timeout,
			// The gateway is the only configured endpoint; a redirect would
			// move a credentialed forward elsewhere by definition.
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return fmt.Errorf("craftegress: gateway redirect refused")
			},
		},
		maxBody: maxBody,
	}, nil
}

// Close releases the journal and idle connections.
func (a *CraftEgressAdapter) Close() error {
	if a == nil {
		return nil
	}
	a.client.CloseIdleConnections()
	return a.journal.Close()
}

// ServeHTTP forwards one provider request through the attempt authority.
// Mint-before-send is fail-closed: without a durable journal commit no
// forward leaves this process.
func (a *CraftEgressAdapter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if a == nil || a.journal == nil {
		http.Error(w, "egress adapter unavailable", http.StatusServiceUnavailable)
		return
	}
	body, ok := a.readBounded(w, r)
	if !ok {
		return
	}
	forwardURL, ok := a.targetURL(w, r)
	if !ok {
		return
	}

	attemptID := ""
	if r.Method == http.MethodPost && len(body) > 0 {
		digest := craftEgressRequestDigest(r.Method, r.URL.Path, r.URL.RawQuery, body)
		// Single-lock check-and-mint: two concurrent same-fingerprint
		// requests must never each mint an identity.
		record, _, err := a.journal.AllocateIfNotParked(digest)
		if err != nil {
			// No durable identity, no physical send. Ever.
			http.Error(w, "egress attempt journal unavailable", http.StatusServiceUnavailable)
			return
		}
		attemptID = record.AttemptID
	}

	req, err := http.NewRequestWithContext(r.Context(), r.Method, forwardURL, strings.NewReader(string(body)))
	if err != nil {
		http.Error(w, "egress forward unavailable", http.StatusBadGateway)
		return
	}
	if contentType := r.Header.Get("Content-Type"); contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("Authorization", "Bearer "+a.credential)
	if attemptID != "" {
		req.Header.Set(craftModelActivityHeader, attemptID)
	}
	resp, err := a.client.Do(req)
	if err != nil {
		// Unknown outcome: the identity stays parked and must be reused. The
		// parked id still travels back so the runtime can correlate.
		if attemptID != "" {
			w.Header().Set(craftModelActivityHeader, attemptID)
		}
		http.Error(w, "egress forward outcome unknown", http.StatusBadGateway)
		return
	}
	defer func() { _ = resp.Body.Close() }()
	responseBody, readErr := io.ReadAll(io.LimitReader(resp.Body, a.maxBody+1))
	if readErr != nil || int64(len(responseBody)) > a.maxBody {
		if attemptID != "" {
			digest := craftEgressRequestDigest(r.Method, r.URL.Path, r.URL.RawQuery, body)
			_ = a.journal.Resolve(attemptID, digest, resp.StatusCode, false)
			w.Header().Set(craftModelActivityHeader, attemptID)
		}
		http.Error(w, "egress response incomplete", http.StatusBadGateway)
		return
	}
	if attemptID != "" {
		digest := craftEgressRequestDigest(r.Method, r.URL.Path, r.URL.RawQuery, body)
		// The gateway marks an unknown-outcome send with the machine-readable
		// appFail code ACTIVITY_UNRESOLVED — on 409 (its own binding
		// conflict) and on 502 (initiation timeout, failed forward, lost
		// response). Only that code parks; a bare status (an upstream 409
		// passed through verbatim) is definitive, and a definitive resolve
		// of an actually-unknown send would mint a fresh identity on retry
		// and bill the same logical request twice.
		definitive := !gatewayReportsActivityUnresolved(resp.StatusCode, responseBody)
		if err := a.journal.Resolve(attemptID, digest, resp.StatusCode, definitive); err != nil {
			// The response was observed; a failed resolution record leaves the
			// attempt reusable, which reconciles safely on the next pass.
			_ = err
		}
	}
	if contentType := resp.Header.Get("Content-Type"); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	if attemptID != "" {
		w.Header().Set(craftModelActivityHeader, attemptID)
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(responseBody)
}

func (a *CraftEgressAdapter) readBounded(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	if r.Body == nil {
		return nil, true
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, a.maxBody+1))
	if err != nil {
		http.Error(w, "egress request body unreadable", http.StatusBadRequest)
		return nil, false
	}
	if int64(len(body)) > a.maxBody {
		http.Error(w, "egress request body exceeds maximum size", http.StatusRequestEntityTooLarge)
		return nil, false
	}
	return body, true
}

// targetURL joins the configured gateway base with the inbound request path.
// The path is operator-surface constrained: absolute, no traversal, and the
// resulting host is always the configured gateway host.
func (a *CraftEgressAdapter) targetURL(w http.ResponseWriter, r *http.Request) (string, bool) {
	path := r.URL.Path
	if !strings.HasPrefix(path, "/") || strings.Contains(path, "..") {
		http.Error(w, "egress path refused", http.StatusBadRequest)
		return "", false
	}
	joined := *a.gateway
	joined.Path = strings.TrimSuffix(joined.Path, "/") + path
	joined.RawQuery = r.URL.RawQuery
	return joined.String(), true
}

// craftEgressRequestDigest fingerprints one logical provider request so an
// unknown-outcome retry can be correlated back to its parked attempt. The
// query string participates: two requests that differ only by query are
// different logical requests and must never share a parked identity. This
// correlation is bookkeeping for reuse; identity itself is always the
// journal-minted opaque id.
func craftEgressRequestDigest(method, path, rawQuery string, body []byte) string {
	digest := sha256.Sum256(append([]byte(strconv.Itoa(len(path))+"\x00"+method+"\x00"+path+"\x00"+strconv.Itoa(len(rawQuery))+"\x00"+rawQuery+"\x00"), body...))
	return hex.EncodeToString(digest[:])
}

// gatewayReportsActivityUnresolved parses the gateway's application error
// envelope and reports whether this response is an explicit
// unknown-outcome signal, regardless of HTTP status.
func gatewayReportsActivityUnresolved(status int, body []byte) bool {
	if status != http.StatusConflict && status != http.StatusBadGateway {
		return false
	}
	var envelope struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return false
	}
	// The gateway marks an unknown-outcome send with ACTIVITY_UNRESOLVED —
	// and ONLY that code: the remaining UPSTREAM_ERROR emitter is the
	// initiation-expired path whose charge-start journal already resolved
	// DefinitelyNotStarted (nothing was physically sent), a definitive
	// outcome. Parking a definitive failure makes the same-fingerprint retry
	// reuse the parked id and deadlock on the gateway's 409 forever.
	return envelope.Error.Code == "ACTIVITY_UNRESOLVED"
}
