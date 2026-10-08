package career

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"html"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/utils"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Frozen source-status, completeness, and bounded failure-code enums (T09).
const (
	SourceStatusComplete         = "complete"
	SourceStatusPartial          = "partial"
	SourceStatusLoginRequired    = "login_required"
	SourceStatusBlocked          = "blocked"
	SourceStatusNotFound         = "not_found"
	SourceStatusTimedOut         = "timed_out"
	SourceStatusFetchFailed      = "fetch_failed"
	SourceStatusPolicyUnverified = "policy_unverified"

	CompletenessComplete   = "complete"
	CompletenessIncomplete = "incomplete"
	CompletenessUnknown    = "unknown"
)

const (
	FailureLoginRequired      = "login_required"
	FailureAccessBlocked      = "access_blocked"
	FailureNotFound           = "not_found"
	FailureTimeout            = "timeout"
	FailureSourceUnverified   = "source_unverified"
	FailureUnsupportedContent = "unsupported_content"
	FailureEmptyContent       = "empty_content"
	FailureResponseTooLarge   = "response_too_large"
	FailureNetworkError       = "network_error"
	FailureRedirectDisallowed = "redirect_disallowed"
)

const (
	sourceImportURLKind   = "opportunity_url_imported"
	sourceImportClaimKind = "import_url_claim"
	maxSourceURLBytes     = 2048
	sourceImportLease     = 60 * time.Second
	sourceImportWaitLimit = 3 * time.Second
	sourceImportInterval  = 25 * time.Millisecond
	sourceFetchTimeout    = 20 * time.Second
	sourceDialTimeout     = 15 * time.Second
	sourceMaxBodyBytes    = 2 * 1024 * 1024
	sourceMaxRedirects    = 5
	// A page only counts as complete through this adapter-specific positive
	// check: substantial text plus at least two job-description markers.
	sourceMinCompleteRunes = 60
)

var (
	errSourceNotVetted        = errors.New("career source is not vetted")
	errImportClaimLost        = errors.New("career import url claim lost")
	errImportURLReceipt       = errors.New("unreadable career import url receipt")
	sourceBoundedFailureCodes = map[string]bool{
		FailureLoginRequired:      true,
		FailureAccessBlocked:      true,
		FailureNotFound:           true,
		FailureTimeout:            true,
		FailureSourceUnverified:   true,
		FailureUnsupportedContent: true,
		FailureEmptyContent:       true,
		FailureResponseTooLarge:   true,
		FailureNetworkError:       true,
		FailureRedirectDisallowed: true,
	}
)

// ApprovedSource identifies the adapter that vetted a source host.
type ApprovedSource struct {
	AdapterID      string
	AdapterVersion string
}

// SourcePolicy is the server-owned trust authority for URL sources. The
// production allowlist starts empty; no client-declared trust is accepted.
type SourcePolicy interface {
	Verify(rawURL string) (ApprovedSource, error)
	VerifyRedirect(current, next string) error
}

// SourceFetchResult carries a successful transport fetch. Complete is only
// ever set by an adapter-specific positive completeness check.
type SourceFetchResult struct {
	StatusCode  int
	ContentType string
	Text        string
	FinalURL    string
	Complete    bool
	LoginWall   bool
}

// SourceFetchError bounds every transport failure to a frozen code. Raw
// upstream errors, private addresses, and credentials must never escape.
type SourceFetchError struct {
	Code string
	Err  error
}

func (e *SourceFetchError) Error() string {
	if e == nil {
		return ""
	}
	return "career source fetch failed: " + boundedSourceFailureCode(e.Code)
}

func (e *SourceFetchError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func boundedSourceFailureCode(code string) string {
	if sourceBoundedFailureCodes[code] {
		return code
	}
	return FailureNetworkError
}

// SourceTransport performs the network fetch for an already-approved source.
type SourceTransport interface {
	Fetch(ctx context.Context, source ApprovedSource, rawURL string) (SourceFetchResult, error)
}

type ImportURLInput struct {
	RequestID string `json:"requestId"`
	URL       string `json:"url"`
}

type ImportURLResult struct {
	OpportunityID string    `json:"opportunityId"`
	ObservationID string    `json:"observationId"`
	SnapshotID    string    `json:"snapshotId"`
	SourceStatus  string    `json:"sourceStatus"`
	Completeness  string    `json:"completeness"`
	FailureCode   string    `json:"failureCode,omitempty"`
	SubmittedURL  string    `json:"submittedUrl"`
	AcquiredAt    time.Time `json:"acquiredAt"`
	NeedsUserJD   bool      `json:"needsUserJD"`
}

// ImportURLReceipt is the durable terminal receipt body for a URL import. It
// is a flat superset of ImportURLResult so the generic opportunity receipt
// endpoint stays readable for both manual and URL imports.
type ImportURLReceipt struct {
	Kind               string    `json:"kind"`
	RequestID          string    `json:"requestId"`
	OpportunityID      string    `json:"opportunityId"`
	ObservationID      string    `json:"observationId"`
	SnapshotID         string    `json:"snapshotId"`
	SnapshotStatus     string    `json:"status"`
	SourceStatus       string    `json:"sourceStatus"`
	Completeness       string    `json:"completeness"`
	FailureCode        string    `json:"failureCode,omitempty"`
	SubmittedURL       string    `json:"submittedUrl"`
	FinalURL           string    `json:"finalUrl,omitempty"`
	AdapterID          string    `json:"adapterId,omitempty"`
	AdapterVersion     string    `json:"adapterVersion,omitempty"`
	ObservedHTTPStatus int       `json:"observedHttpStatus,omitempty"`
	AcquiredAt         time.Time `json:"acquiredAt"`
	NeedsUserJD        bool      `json:"needsUserJD"`
}

// Result projects the frozen contract view of the receipt.
func (r ImportURLReceipt) Result() ImportURLResult {
	return ImportURLResult{
		OpportunityID: r.OpportunityID,
		ObservationID: r.ObservationID,
		SnapshotID:    r.SnapshotID,
		SourceStatus:  r.SourceStatus,
		Completeness:  r.Completeness,
		FailureCode:   r.FailureCode,
		SubmittedURL:  r.SubmittedURL,
		AcquiredAt:    r.AcquiredAt,
		NeedsUserJD:   r.NeedsUserJD,
	}
}

// OpportunityObservationView is one immutable observation row in the
// owner-scoped observation list.
type OpportunityObservationView struct {
	ObservationID      string            `json:"observationId"`
	SnapshotID         string            `json:"snapshotId"`
	Source             OpportunitySource `json:"source"`
	SourceStatus       string            `json:"sourceStatus,omitempty"`
	Completeness       string            `json:"completeness,omitempty"`
	FailureCode        string            `json:"failureCode,omitempty"`
	SubmittedURL       string            `json:"submittedUrl,omitempty"`
	FinalURL           string            `json:"finalUrl,omitempty"`
	AdapterID          string            `json:"adapterId,omitempty"`
	AdapterVersion     string            `json:"adapterVersion,omitempty"`
	ObservedHTTPStatus int               `json:"observedHttpStatus,omitempty"`
	NeedsUserJD        bool              `json:"needsUserJD"`
	AcquiredAt         time.Time         `json:"acquiredAt"`
}

// ---- production policy ----------------------------------------------------

// emptySourcePolicy is the production policy: the source allowlist starts
// empty. No recruiting domain may be marked vetted without an authorized
// source-review record, so every URL is policy_unverified today.
type emptySourcePolicy struct{}

func (emptySourcePolicy) Verify(string) (ApprovedSource, error) {
	return ApprovedSource{}, errSourceNotVetted
}

func (emptySourcePolicy) VerifyRedirect(current, next string) error {
	return errSourceNotVetted
}

// ---- narrow Career-owned transport -----------------------------------------

type transportDialOptions struct {
	LookupIPs   func(context.Context, string) ([]net.IP, error)
	DialContext func(context.Context, string, string) (net.Conn, error)
}

// careerSourceTransport is the only network path for URL imports: HTTP-only,
// cookie-less, no browser rendering, with DNS-pinned SSRF guards and a
// policy-checked redirect chain. It consumes the SourcePolicy interface and
// deliberately does not depend on web_fetch.NewFetcher.
type careerSourceTransport struct {
	policy SourcePolicy
	client *http.Client
}

func newCareerSourceTransport(policy SourcePolicy, dial transportDialOptions) *careerSourceTransport {
	if dial.LookupIPs == nil {
		dial.LookupIPs = func(ctx context.Context, host string) ([]net.IP, error) {
			return net.DefaultResolver.LookupIP(ctx, "ip", host)
		}
	}
	if dial.DialContext == nil {
		dialer := &net.Dialer{Timeout: sourceDialTimeout, KeepAlive: 30 * time.Second}
		dial.DialContext = dialer.DialContext
	}
	base := &http.Transport{
		// Never route source fetches through ambient proxy configuration and
		// never attach cookies or login sessions.
		Proxy: nil,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, &SourceFetchError{Code: FailureNetworkError}
			}
			ips := []net.IP{}
			if literal := net.ParseIP(host); literal != nil {
				ips = []net.IP{literal}
			} else {
				resolved, resolveErr := dial.LookupIPs(ctx, host)
				if resolveErr != nil || len(resolved) == 0 {
					return nil, &SourceFetchError{Code: FailureNetworkError}
				}
				ips = resolved
			}
			for _, ip := range ips {
				if !utils.IsPublicIP(ip) {
					// Localhost, loopback, private, and reserved ranges are
					// rejected before any connection is opened.
					return nil, &SourceFetchError{Code: FailureNetworkError}
				}
			}
			return dial.DialContext(ctx, network, net.JoinHostPort(ips[0].String(), port))
		},
	}
	transport := &careerSourceTransport{policy: policy}
	transport.client = &http.Client{Transport: base, CheckRedirect: transport.checkRedirect}
	return transport
}

func (t *careerSourceTransport) checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= sourceMaxRedirects {
		return &SourceFetchError{Code: FailureRedirectDisallowed}
	}
	if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
		return &SourceFetchError{Code: FailureRedirectDisallowed}
	}
	if host := req.URL.Hostname(); host != "" {
		if ip := net.ParseIP(host); ip != nil && !utils.IsPublicIP(ip) {
			// Private or loopback redirect targets are rejected at the
			// redirect, before any trust is extended to them.
			return &SourceFetchError{Code: FailureRedirectDisallowed}
		}
	}
	current := ""
	if len(via) > 0 && via[len(via)-1].URL != nil {
		current = via[len(via)-1].URL.String()
	}
	if err := t.policy.VerifyRedirect(current, req.URL.String()); err != nil {
		return &SourceFetchError{Code: FailureRedirectDisallowed}
	}
	return nil
}

func (t *careerSourceTransport) Fetch(ctx context.Context, _ ApprovedSource, rawURL string) (SourceFetchResult, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return SourceFetchResult{}, &SourceFetchError{Code: FailureNetworkError}
	}
	fetchCtx, cancel := context.WithTimeout(ctx, sourceFetchTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(fetchCtx, http.MethodGet, rawURL, nil)
	if err != nil {
		return SourceFetchResult{}, &SourceFetchError{Code: FailureNetworkError}
	}
	// Honest client identity: no browser spoofing, cookies, or crawler bypass.
	req.Header.Set("User-Agent", "WeKnora-Career-Source-Import/1.0")
	req.Header.Set("Accept", "text/html,text/plain;q=0.9,*/*;q=0.1")
	resp, err := t.client.Do(req)
	if err != nil {
		return SourceFetchResult{}, classifyTransportError(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return SourceFetchResult{}, classifyTransportHTTPStatus(resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, sourceMaxBodyBytes+1))
	if err != nil {
		return SourceFetchResult{}, &SourceFetchError{Code: FailureNetworkError}
	}
	if len(body) > sourceMaxBodyBytes {
		return SourceFetchResult{}, &SourceFetchError{Code: FailureResponseTooLarge}
	}
	contentType, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = http.DetectContentType(body)
	}
	if !sourceReadableContent(contentType) {
		return SourceFetchResult{}, &SourceFetchError{Code: FailureUnsupportedContent}
	}
	text := sourceReadableText(contentType, body)
	if strings.TrimSpace(text) == "" {
		return SourceFetchResult{}, &SourceFetchError{Code: FailureEmptyContent}
	}
	complete, loginWall := assessSourceCompleteness(text)
	finalURL := rawURL
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}
	return SourceFetchResult{StatusCode: resp.StatusCode, ContentType: contentType, Text: text, FinalURL: finalURL, Complete: complete, LoginWall: loginWall}, nil
}

func classifyTransportError(err error) error {
	var fetchFailure *SourceFetchError
	if errors.As(err, &fetchFailure) {
		return fetchFailure
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return &SourceFetchError{Code: FailureTimeout}
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return &SourceFetchError{Code: FailureTimeout}
	}
	return &SourceFetchError{Code: FailureNetworkError}
}

func classifyTransportHTTPStatus(status int) error {
	switch {
	case status == http.StatusUnauthorized || status == http.StatusPaymentRequired || status == http.StatusProxyAuthRequired:
		return &SourceFetchError{Code: FailureLoginRequired}
	case status == http.StatusNotFound || status == http.StatusGone:
		return &SourceFetchError{Code: FailureNotFound}
	case status >= http.StatusInternalServerError:
		return &SourceFetchError{Code: FailureNetworkError}
	default:
		return &SourceFetchError{Code: FailureAccessBlocked}
	}
}

func sourceReadableContent(contentType string) bool {
	switch contentType {
	case "text/html", "text/plain", "application/xhtml+xml":
		return true
	default:
		return strings.HasPrefix(contentType, "text/")
	}
}

var (
	sourceScriptPattern  = regexp.MustCompile(`(?is)<(script|style)\b[^>]*>.*?</(script|style)\s*>`)
	sourceCommentPattern = regexp.MustCompile(`(?s)<!--.*?-->`)
	sourceTagPattern     = regexp.MustCompile(`(?s)<[^>]*>`)
)

func sourceReadableText(contentType string, body []byte) string {
	text := string(body)
	if strings.Contains(strings.ToLower(contentType), "html") {
		text = sourceScriptPattern.ReplaceAllString(text, " ")
		text = sourceCommentPattern.ReplaceAllString(text, " ")
		text = sourceTagPattern.ReplaceAllString(text, " ")
		text = html.UnescapeString(text)
	}
	return strings.Join(strings.Fields(text), " ")
}

var (
	sourceLoginMarkers = []string{"请登录", "请先登录", "立即登录", "登录后查看", "登录查看", "登录继续", "sign in", "log in", "please log in", "login required"}
	sourceJDMarkers    = []string{"职责", "任职", "要求", "职位", "岗位", "描述", "responsibilit", "qualification", "requirement", "job description", "about the role"}
)

// assessSourceCompleteness is the adapter-specific positive completeness
// check. Complete requires substantial text plus job-description markers; a
// short page dominated by a login call is a login wall. Everything else stays
// partial and asks the user for the JD.
func assessSourceCompleteness(text string) (complete bool, loginWall bool) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false, false
	}
	runes := len([]rune(trimmed))
	lower := strings.ToLower(trimmed)
	if runes < 300 {
		for _, marker := range sourceLoginMarkers {
			if strings.Contains(lower, marker) {
				return false, true
			}
		}
	}
	matched := 0
	for _, marker := range sourceJDMarkers {
		if strings.Contains(lower, marker) {
			matched++
		}
	}
	return runes >= sourceMinCompleteRunes && matched >= 2, false
}

// ---- URL import service ----------------------------------------------------

type urlSourceEvidence struct {
	sourceStatus   string
	completeness   string
	failureCode    string
	text           string
	finalURL       string
	adapterID      string
	adapterVersion string
	httpStatus     int
	needsUserJD    bool
}

type importURLClaimBody struct {
	Kind       string    `json:"kind"`
	ClaimToken string    `json:"claimToken"`
	LeaseUntil time.Time `json:"leaseUntil"`
}

type importClaimState int

const (
	importClaimProceed importClaimState = iota
	importClaimTerminal
	importClaimInFlight
)

type importClaimOutcome struct {
	state   importClaimState
	receipt ImportURLReceipt
	token   string
}

// ImportURL records one URL import attempt. All network I/O happens outside
// any database transaction; the durable claim/receipt pair reconciles
// concurrent and unknown outcomes under the original request ID.
func (o *Office) ImportURL(ctx context.Context, input ImportURLInput) (ImportURLResult, error) {
	receipt, err := o.ImportURLReceipt(ctx, input)
	if err != nil {
		return ImportURLResult{}, err
	}
	return receipt.Result(), nil
}

func (o *Office) ImportURLReceipt(ctx context.Context, input ImportURLInput) (ImportURLReceipt, error) {
	s, err := getScope(ctx)
	if err != nil {
		return ImportURLReceipt{}, err
	}
	if err = o.requireSpace(ctx, s); err != nil {
		if isSQLiteBusy(err) {
			// Space validation hit a transient SQLite lock before any claim
			// or network I/O started; the same request ID may be retried, so
			// report a typed unknown outcome instead of a raw lock error.
			return ImportURLReceipt{}, &OutcomeUnknownError{RequestID: input.RequestID}
		}
		return ImportURLReceipt{}, err
	}
	input.RequestID = strings.TrimSpace(input.RequestID)
	if input.RequestID == "" || len(input.RequestID) > 128 || strings.TrimSpace(input.URL) == "" || len(input.URL) > maxSourceURLBytes {
		return ImportURLReceipt{}, ErrInvalidRequest
	}
	// The exact submitted URL is preserved verbatim; parsing only validates.
	parsed, parseErr := url.Parse(input.URL)
	if parseErr != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" {
		return ImportURLReceipt{}, ErrInvalidRequest
	}
	fingerprintInput, err := json.Marshal([]any{"import_url", input.RequestID, input.URL})
	if err != nil {
		return ImportURLReceipt{}, err
	}
	fingerprintSum := sha256.Sum256(fingerprintInput)
	fingerprint := hex.EncodeToString(fingerprintSum[:])

	claim, err := o.claimImportURLRequest(ctx, s, input.RequestID, fingerprint)
	if err != nil {
		return ImportURLReceipt{}, err
	}
	switch claim.state {
	case importClaimTerminal:
		return claim.receipt, nil
	case importClaimInFlight:
		receipt, found, awaitErr := o.awaitImportURLTerminal(ctx, s, input.RequestID, fingerprint, sourceImportWaitLimit)
		if awaitErr != nil {
			return ImportURLReceipt{}, awaitErr
		}
		if found {
			return receipt, nil
		}
		return ImportURLReceipt{}, &OutcomeUnknownError{RequestID: input.RequestID}
	}

	// Network I/O starts here, strictly outside any database transaction, and
	// only after the server-owned policy approved the source.
	evidence := urlSourceEvidence{
		sourceStatus: SourceStatusPolicyUnverified,
		completeness: CompletenessUnknown,
		failureCode:  FailureSourceUnverified,
		needsUserJD:  true,
	}
	if approved, verifyErr := o.sourcePolicy.Verify(input.URL); verifyErr == nil {
		evidence.adapterID = approved.AdapterID
		evidence.adapterVersion = approved.AdapterVersion
		result, fetchErr := o.sourceTransport.Fetch(ctx, approved, input.URL)
		evidence = classifySourceFetch(result, fetchErr, evidence.adapterID, evidence.adapterVersion)
	}
	return o.commitImportURLObservation(ctx, s, input.RequestID, fingerprint, claim.token, evidence, input.URL)
}

func classifySourceFetch(result SourceFetchResult, fetchErr error, adapterID, adapterVersion string) urlSourceEvidence {
	evidence := urlSourceEvidence{adapterID: adapterID, adapterVersion: adapterVersion, needsUserJD: true}
	if fetchErr != nil {
		code := FailureNetworkError
		var fetchFailure *SourceFetchError
		if errors.As(fetchErr, &fetchFailure) {
			code = boundedSourceFailureCode(fetchFailure.Code)
		}
		evidence.failureCode = code
		// Nothing readable was acquired, so completeness is explicitly unknown
		// rather than empty — consistent with the frozen enum used by the
		// policy_unverified and login-wall paths.
		evidence.completeness = CompletenessUnknown
		switch code {
		case FailureLoginRequired:
			evidence.sourceStatus = SourceStatusLoginRequired
		case FailureAccessBlocked:
			evidence.sourceStatus = SourceStatusBlocked
		case FailureNotFound:
			evidence.sourceStatus = SourceStatusNotFound
		case FailureTimeout:
			evidence.sourceStatus = SourceStatusTimedOut
		default:
			evidence.sourceStatus = SourceStatusFetchFailed
		}
		return evidence
	}
	evidence.httpStatus = result.StatusCode
	evidence.finalURL = result.FinalURL
	if result.LoginWall {
		// A login wall is a failed observation: nothing readable was
		// acquired, so the snapshot persists explicitly empty text.
		evidence.sourceStatus = SourceStatusLoginRequired
		evidence.failureCode = FailureLoginRequired
		evidence.completeness = CompletenessUnknown
		return evidence
	}
	evidence.text = result.Text
	if result.Complete {
		evidence.sourceStatus = SourceStatusComplete
		evidence.completeness = CompletenessComplete
		evidence.needsUserJD = false
		return evidence
	}
	evidence.sourceStatus = SourceStatusPartial
	evidence.completeness = CompletenessIncomplete
	return evidence
}

func decodeImportURLBody(body string) (*ImportURLReceipt, importURLClaimBody, error) {
	var claim importURLClaimBody
	if err := json.Unmarshal([]byte(body), &claim); err != nil {
		return nil, importURLClaimBody{}, errImportURLReceipt
	}
	switch claim.Kind {
	case sourceImportURLKind:
		var receipt ImportURLReceipt
		if err := json.Unmarshal([]byte(body), &receipt); err != nil {
			return nil, importURLClaimBody{}, errImportURLReceipt
		}
		return &receipt, importURLClaimBody{}, nil
	case sourceImportClaimKind:
		return nil, claim, nil
	default:
		return nil, importURLClaimBody{}, errImportURLReceipt
	}
}

// claimImportURLRequest durably claims the scoped request before any network
// I/O. Concurrent identical claims resolve to one owner; the losers wait for
// the terminal receipt instead of fetching twice.
func (o *Office) claimImportURLRequest(ctx context.Context, s Scope, requestID, fingerprint string) (importClaimOutcome, error) {
	outcome := importClaimOutcome{}
	token := uuid.NewString()
	claimJSON := func() (string, error) {
		body, err := json.Marshal(importURLClaimBody{Kind: sourceImportClaimKind, ClaimToken: token, LeaseUntil: time.Now().UTC().Add(sourceImportLease)})
		return string(body), err
	}
	register := func(tx *gorm.DB) error {
		var existing opportunityReceipt
		err := tx.Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, requestID).First(&existing).Error
		if err == nil {
			if existing.Fingerprint != fingerprint {
				return ErrIdempotencyConflict
			}
			receipt, claim, decodeErr := decodeImportURLBody(existing.Body)
			if decodeErr != nil {
				return decodeErr
			}
			if receipt != nil {
				outcome.state, outcome.receipt = importClaimTerminal, *receipt
				return nil
			}
			if claim.ClaimToken != "" && claim.LeaseUntil.After(time.Now().UTC()) {
				outcome.state = importClaimInFlight
				return nil
			}
			// An expired claim is taken over so a crashed fetch cannot strand
			// the request ID forever. The UPDATE carries a compare-and-swap on
			// the exact body that was read: under READ COMMITTED the previous
			// owner may commit its terminal receipt between this transaction's
			// read and write, and an unconditional UPDATE would destroy that
			// receipt (and duplicate the opportunity). RowsAffected != 1 means
			// the body moved — degrade to the in-flight waiter path instead.
			takeover, err := claimJSON()
			if err != nil {
				return err
			}
			res := tx.Model(&opportunityReceipt{}).
				Where("tenant_id=? AND user_id=? AND request_id=? AND body=?", s.TenantID, s.UserID, requestID, existing.Body).
				Update("body", takeover)
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected != 1 {
				outcome.state = importClaimInFlight
				return nil
			}
			outcome.state, outcome.token = importClaimProceed, token
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := requireGateActiveTx(tx, s); err != nil {
			return err
		}
		claimBody, err := claimJSON()
		if err != nil {
			return err
		}
		err = tx.Create(&opportunityReceipt{TenantID: s.TenantID, UserID: s.UserID, RequestID: requestID, Fingerprint: fingerprint, Body: claimBody, CreatedAt: time.Now().UTC()}).Error
		if err != nil {
			if isSQLiteBusy(err) {
				return err
			}
			if isReceiptRaceError(err) {
				outcome.state = importClaimInFlight
				return nil
			}
			return err
		}
		outcome.state, outcome.token = importClaimProceed, token
		return nil
	}
	err := o.runImportTransaction(ctx, register)
	if err != nil {
		if isSQLiteBusy(err) {
			// The claim never committed; the caller may retry the same
			// request ID, so report a typed unknown outcome instead of a raw
			// lock error.
			return importClaimOutcome{}, &OutcomeUnknownError{RequestID: requestID}
		}
		return importClaimOutcome{}, err
	}
	return outcome, nil
}

// runImportTransaction runs op in a short transaction, retrying whole
// transactions while SQLite reports writer-lock contention. A busy failure
// always rolls the transaction back, so retries stay idempotent.
func (o *Office) runImportTransaction(ctx context.Context, op func(tx *gorm.DB) error) error {
	operationCtx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	var err error
	for attempt := 0; ; attempt++ {
		err = o.db.WithContext(operationCtx).Transaction(op)
		if err == nil || !isSQLiteBusy(err) {
			return err
		}
		delay := time.Duration(attempt+1) * 25 * time.Millisecond
		if delay > 250*time.Millisecond {
			delay = 250 * time.Millisecond
		}
		timer := time.NewTimer(delay)
		select {
		case <-operationCtx.Done():
			timer.Stop()
			return err
		case <-timer.C:
		}
	}
}

// awaitImportURLTerminal polls the durable receipt until it turns terminal.
// A maxWait of zero performs exactly one bounded read.
func (o *Office) awaitImportURLTerminal(ctx context.Context, s Scope, requestID, fingerprint string, maxWait time.Duration) (ImportURLReceipt, bool, error) {
	deadline := time.Now().Add(maxWait)
	for {
		var row opportunityReceipt
		err := o.db.WithContext(ctx).Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, requestID).First(&row).Error
		if err == nil {
			if row.Fingerprint != fingerprint {
				return ImportURLReceipt{}, false, ErrIdempotencyConflict
			}
			receipt, _, decodeErr := decodeImportURLBody(row.Body)
			if decodeErr != nil {
				return ImportURLReceipt{}, false, decodeErr
			}
			if receipt != nil {
				return *receipt, true, nil
			}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) && !isSQLiteBusy(err) {
			return ImportURLReceipt{}, false, err
		}
		if time.Now().After(deadline) {
			return ImportURLReceipt{}, false, nil
		}
		select {
		case <-ctx.Done():
			return ImportURLReceipt{}, false, &OutcomeUnknownError{RequestID: requestID}
		case <-time.After(sourceImportInterval):
		}
	}
}

// commitImportURLObservation atomically inserts the opportunity, observation,
// snapshot, and terminal receipt. Failed observations still receive a
// snapshot row with empty text, the SHA-256 of empty bytes, and needs_review.
func (o *Office) commitImportURLObservation(ctx context.Context, s Scope, requestID, fingerprint, claimToken string, evidence urlSourceEvidence, rawURL string) (ImportURLReceipt, error) {
	var stored ImportURLReceipt
	err := o.runImportTransaction(ctx, func(tx *gorm.DB) error {
		var row opportunityReceipt
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, requestID).First(&row).Error; err != nil {
			return err
		}
		if row.Fingerprint != fingerprint {
			return ErrIdempotencyConflict
		}
		receipt, claim, decodeErr := decodeImportURLBody(row.Body)
		if decodeErr != nil {
			return decodeErr
		}
		if receipt != nil {
			// A concurrent reconcile already committed the terminal receipt.
			stored = *receipt
			return nil
		}
		if claim.ClaimToken == "" || claim.ClaimToken != claimToken {
			return errImportClaimLost
		}
		acquiredAt := time.Now().UTC()
		oppID, observationID, snapshotID := uuid.NewString(), uuid.NewString(), uuid.NewString()
		digest := sha256.Sum256([]byte(evidence.text))
		// URL text never infers graduation, degree, or other hard conditions:
		// extracted fields stay unknown and the snapshot stays needs_review.
		extracted, err := json.Marshal(unknownOpportunityFields())
		if err != nil {
			return err
		}
		sourceLabel := ""
		if parsed, parseErr := url.Parse(rawURL); parseErr == nil {
			sourceLabel = parsed.Hostname()
		}
		stored = ImportURLReceipt{
			Kind:               sourceImportURLKind,
			RequestID:          requestID,
			OpportunityID:      oppID,
			ObservationID:      observationID,
			SnapshotID:         snapshotID,
			SnapshotStatus:     OpportunityNeedsReview,
			SourceStatus:       evidence.sourceStatus,
			Completeness:       evidence.completeness,
			FailureCode:        evidence.failureCode,
			SubmittedURL:       rawURL,
			FinalURL:           evidence.finalURL,
			AdapterID:          evidence.adapterID,
			AdapterVersion:     evidence.adapterVersion,
			ObservedHTTPStatus: evidence.httpStatus,
			AcquiredAt:         acquiredAt,
			NeedsUserJD:        evidence.needsUserJD,
		}
		if err = tx.Create(&opportunity{ID: oppID, TenantID: s.TenantID, UserID: s.UserID, CreatedAt: acquiredAt}).Error; err != nil {
			return err
		}
		if err = tx.Create(&opportunityObservation{
			ID: observationID, TenantID: s.TenantID, UserID: s.UserID, OpportunityID: oppID, SnapshotID: snapshotID,
			SourceKind: "url", SourceLabel: sourceLabel, SourceRef: rawURL,
			SourceStatus: evidence.sourceStatus, Completeness: evidence.completeness, FailureCode: evidence.failureCode,
			SubmittedURL: rawURL, FinalURL: evidence.finalURL,
			AdapterID: evidence.adapterID, AdapterVersion: evidence.adapterVersion, ObservedHTTPStatus: evidence.httpStatus,
			AcquiredAt: acquiredAt, CreatedAt: acquiredAt,
		}).Error; err != nil {
			return err
		}
		if err = tx.Create(&opportunitySnapshot{
			ID: snapshotID, TenantID: s.TenantID, UserID: s.UserID, OpportunityID: oppID, ObservationID: observationID,
			RawText: evidence.text, RawSHA256: hex.EncodeToString(digest[:]),
			Extracted: string(extracted), Status: OpportunityNeedsReview,
			AcquiredAt: acquiredAt, CreatedAt: acquiredAt,
		}).Error; err != nil {
			return err
		}
		body, err := json.Marshal(stored)
		if err != nil {
			return err
		}
		return tx.Model(&opportunityReceipt{}).Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, requestID).Update("body", string(body)).Error
	})
	if err == nil {
		return stored, nil
	}
	if errors.Is(err, errImportClaimLost) || isSQLiteBusy(err) {
		// Our claim was taken over after a lease expiry, or writer-lock
		// contention exhausted the retry budget. Reconcile once with the
		// original request ID instead of guessing the outcome.
		reconcileCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), opportunityLookupWindow)
		defer cancel()
		receipt, found, lookupErr := o.awaitImportURLTerminal(reconcileCtx, s, requestID, fingerprint, 0)
		if lookupErr != nil {
			return ImportURLReceipt{}, lookupErr
		}
		if found {
			return receipt, nil
		}
		return ImportURLReceipt{}, &OutcomeUnknownError{RequestID: requestID}
	}
	return ImportURLReceipt{}, err
}

// OpportunityObservations lists the immutable observation history of one
// opportunity under the current owner's authenticated scope.
func (o *Office) OpportunityObservations(ctx context.Context, opportunityID string) ([]OpportunityObservationView, error) {
	s, err := getScope(ctx)
	if err != nil {
		return nil, err
	}
	if err = o.requireSpace(ctx, s); err != nil {
		return nil, err
	}
	opportunityID = strings.TrimSpace(opportunityID)
	if opportunityID == "" || len(opportunityID) > 36 {
		return nil, ErrInvalidRequest
	}
	var owner opportunity
	err = o.db.WithContext(ctx).Where("tenant_id=? AND user_id=? AND id=?", s.TenantID, s.UserID, opportunityID).First(&owner).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrOpportunityNotFound
	}
	if err != nil {
		return nil, err
	}
	var rows []opportunityObservation
	err = o.db.WithContext(ctx).Where("tenant_id=? AND user_id=? AND opportunity_id=?", s.TenantID, s.UserID, opportunityID).Order("acquired_at, id").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	views := make([]OpportunityObservationView, 0, len(rows))
	for _, row := range rows {
		views = append(views, OpportunityObservationView{
			ObservationID:      row.ID,
			SnapshotID:         row.SnapshotID,
			Source:             OpportunitySource{Kind: row.SourceKind, Label: row.SourceLabel, ReferenceID: row.SourceRef},
			SourceStatus:       row.SourceStatus,
			Completeness:       row.Completeness,
			FailureCode:        row.FailureCode,
			SubmittedURL:       row.SubmittedURL,
			FinalURL:           row.FinalURL,
			AdapterID:          row.AdapterID,
			AdapterVersion:     row.AdapterVersion,
			ObservedHTTPStatus: row.ObservedHTTPStatus,
			NeedsUserJD:        row.SourceKind == "url" && row.SourceStatus != SourceStatusComplete,
			AcquiredAt:         row.AcquiredAt,
		})
	}
	return views, nil
}
