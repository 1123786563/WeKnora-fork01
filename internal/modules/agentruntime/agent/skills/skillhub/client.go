// Package skillhub is the WeKnora port of Octop's SkillHub marketplace
// client (octop/infra/skills/skillhub_market.py): search, rankings and
// package download over HTTP with strict response-size caps, plus an
// exhaustive ZIP validation of every downloaded package before any byte of
// it reaches tenant storage. It is a pure client package — the only WeKnora
// dependency is the SSRF-safe HTTP transport in internal/utils.
package skillhub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/utils"
)

// Defaults and wire constants. Mirrors of the Octop module constants; the
// User-Agent is rebranded for WeKnora per the M4 port plan.
const (
	// DefaultHost is the public SkillHub registry origin.
	DefaultHost = "https://api.skillhub.cn"
	// DefaultTimeout applies to every outbound request when the caller
	// does not configure one.
	DefaultTimeout = 30 * time.Second
	// UserAgent identifies WeKnora to the registry.
	UserAgent = "weknora-skillhub-market/1.0"

	searchEndpoint   = "/api/v1/search"
	downloadEndpoint = "/api/v1/download"

	// maxDownloadBytes caps the raw zip payload (32 MiB).
	maxDownloadBytes int64 = 32 << 20
	// maxJSONBytes caps every JSON response (4 MiB).
	maxJSONBytes int64 = 4 << 20

	readChunk = 64 * 1024
)

// rankingEndpoints maps the public ranking kinds to their showcase paths.
var rankingEndpoints = map[string]string{
	"hot":         "/api/v1/showcase/hot",
	"featured":    "/api/v1/showcase/featured",
	"newest":      "/api/v1/showcase/newest",
	"recommended": "/api/v1/showcase/recommended",
	"trending":    "/api/v1/showcase/trending",
	"paid":        "/api/v1/showcase/paid",
}

// RankingKinds returns the supported ranking kinds in a stable order.
func RankingKinds() []string {
	kinds := make([]string, 0, len(rankingEndpoints))
	for kind := range rankingEndpoints {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	return kinds
}

// Error taxonomy. The Python original uses an exception hierarchy
// (SkillHubMarketError > Timeout, SkillHubPackageError > TooLarge); the Go
// port expresses it as wrapped sentinel errors so errors.Is walks the same
// tree: ErrMarketTimeout IS ErrMarket, ErrPackageTooLarge IS ErrPackage.
var (
	// ErrMarket: a SkillHub marketplace HTTP request failed.
	ErrMarket = errors.New("skillhub: market request failed")
	// ErrMarketTimeout: a SkillHub marketplace HTTP request timed out.
	ErrMarketTimeout = fmt.Errorf("%w: request timed out", ErrMarket)
	// ErrPackage: a downloaded SkillHub package failed validation.
	ErrPackage = errors.New("skillhub: package failed validation")
	// ErrPackageTooLarge: a response or package exceeded a safety limit.
	ErrPackageTooLarge = fmt.Errorf("%w: exceeds a safety limit", ErrPackage)
	// ErrUnsupportedRanking: the requested ranking kind does not exist.
	ErrUnsupportedRanking = errors.New("skillhub: unsupported ranking kind")
	// ErrInvalidHost: the configured SkillHub host is not an http(s) origin.
	ErrInvalidHost = errors.New("skillhub: invalid api host")
)

// SkillSummary is one marketplace listing. The four typed fields are the
// normalized values consumers rely on; Raw carries the stringified original
// entry for display passthrough (Python keeps the whole dict).
type SkillSummary struct {
	Slug        string
	Name        string
	Description string
	Version     string
	Raw         map[string]string
}

// Client talks to the public SkillHub registry.
type Client interface {
	// Search queries the registry. An empty/blank query is coerced to "a"
	// (Octop behavior) and limit is clamped to [1, 100].
	Search(ctx context.Context, query string, limit int) ([]SkillSummary, error)
	// Rankings fetches one showcase list; kind must be one of RankingKinds.
	Rankings(ctx context.Context, kind string) ([]SkillSummary, error)
	// Download fetches the raw package zip bytes for a slug. Validation of
	// the archive is a separate step: ParsePackage.
	Download(ctx context.Context, slug string) ([]byte, error)
}

// HTTPClient is the direct Client implementation. Its transport is the
// SSRF-safe client from internal/utils (dial-time IP pinning, redirect
// validation) with an explicit per-request timeout.
type HTTPClient struct {
	host string
	http *http.Client
}

// ResolveHost validates and canonicalizes a configured host: blank falls
// back to DefaultHost, surrounding whitespace and trailing slashes are
// trimmed, and the result must be an absolute http/https origin.
func ResolveHost(host string) (string, error) {
	resolved := strings.TrimRight(strings.TrimSpace(host), "/")
	if resolved == "" {
		resolved = DefaultHost
	}
	parsed, err := url.Parse(resolved)
	if err != nil || parsed.Scheme == "" {
		return "", fmt.Errorf("%w: %q", ErrInvalidHost, host)
	}
	if (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "", fmt.Errorf("%w: %q", ErrInvalidHost, host)
	}
	return resolved, nil
}

// New builds an HTTPClient against host with the given per-request timeout.
// A non-positive timeout falls back to DefaultTimeout.
func New(host string, timeout time.Duration) (*HTTPClient, error) {
	resolved, err := ResolveHost(host)
	if err != nil {
		return nil, err
	}
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	// MaxRedirects matters: a zero value would make the SSRF redirect policy
	// reject the FIRST redirect (len(via) >= 0 always holds), while real
	// registries redirect listings/downloads to CDNs. 10 matches urllib's
	// default redirect budget and the utils defaultHTTPClient precedent;
	// every hop is still SSRF-validated by the same policy.
	return &HTTPClient{
		host: resolved,
		http: utils.NewSSRFSafeHTTPClient(utils.SSRFSafeHTTPClientConfig{
			Timeout:      timeout,
			MaxRedirects: 10,
		}),
	}, nil
}

// Search implements Client.
func (c *HTTPClient) Search(ctx context.Context, query string, limit int) ([]SkillSummary, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		q = "a"
	}
	if limit < 1 {
		limit = 1
	}
	if limit > 100 {
		limit = 100
	}
	payload, err := c.get(ctx, searchEndpoint,
		url.Values{"q": {q}, "limit": {strconv.Itoa(limit)}},
		"application/json", maxJSONBytes)
	if err != nil {
		return nil, err
	}
	return parseSummaries(payload, "search", "results")
}

// Rankings implements Client.
func (c *HTTPClient) Rankings(ctx context.Context, kind string) ([]SkillSummary, error) {
	path, ok := rankingEndpoints[kind]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedRanking, kind)
	}
	payload, err := c.get(ctx, path, nil, "application/json", maxJSONBytes)
	if err != nil {
		return nil, err
	}
	// The live showcase endpoints answer {"section", "skills", "total"} —
	// there is no "results" key — so rankings accept "skills" as the list
	// key when "results" is absent (verified against api.skillhub.cn's
	// /api/v1/showcase/hot; item fields slug/name/description/version match
	// the search entry shape).
	return parseSummaries(payload, kind, "results", "skills")
}

// Download implements Client.
func (c *HTTPClient) Download(ctx context.Context, slug string) ([]byte, error) {
	return c.get(ctx, downloadEndpoint,
		url.Values{"slug": {slug}},
		"application/zip,application/octet-stream,*/*",
		maxDownloadBytes)
}

// get performs one capped GET. Both the declared Content-Length and the
// streamed byte count are enforced against maxBytes, mirroring the Octop
// _http_request helper.
func (c *HTTPClient) get(ctx context.Context, endpoint string, query url.Values, accept string, maxBytes int64) ([]byte, error) {
	target := c.host + endpoint
	if encoded := query.Encode(); encoded != "" {
		target += "?" + encoded
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMarket, err)
	}
	req.Header.Set("Accept", accept)
	req.Header.Set("User-Agent", UserAgent)

	resp, err := c.http.Do(req)
	if err != nil {
		if isTimeoutError(err) {
			return nil, fmt.Errorf("%w: %v", ErrMarketTimeout, err)
		}
		return nil, fmt.Errorf("%w: %v", ErrMarket, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: HTTP %d", ErrMarket, resp.StatusCode)
	}
	if resp.ContentLength > maxBytes {
		return nil, tooLargeResponse(maxBytes)
	}

	var chunks [][]byte
	var total int64
	buf := make([]byte, readChunk)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			total += int64(n)
			if total > maxBytes {
				return nil, tooLargeResponse(maxBytes)
			}
			chunk := make([]byte, n)
			copy(chunk, buf[:n])
			chunks = append(chunks, chunk)
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				break
			}
			if isTimeoutError(readErr) {
				return nil, fmt.Errorf("%w: %v", ErrMarketTimeout, readErr)
			}
			return nil, fmt.Errorf("%w: %v", ErrMarket, readErr)
		}
	}

	out := make([]byte, 0, total)
	for _, chunk := range chunks {
		out = append(out, chunk...)
	}
	return out, nil
}

func tooLargeResponse(maxBytes int64) error {
	return fmt.Errorf("%w: response exceeds %d MB", ErrPackageTooLarge, maxBytes>>20)
}

// isTimeoutError classifies transport errors into the timeout bucket.
func isTimeoutError(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, os.ErrDeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout()
	}
	return false
}

// parseSummaries decodes a listing document into normalized SkillSummary
// values. The first key in listKeys whose value is an array supplies the
// entries: search responses carry "results", while the live showcase
// (ranking) endpoints return {"section", "skills", "total"}, so rankings
// also accept "skills". Mirroring _fetch_search_json's JSON walk:
//   - the document must be a JSON object with one of the list-key arrays,
//   - entries that are not objects, or lack a non-empty slug, are skipped,
//   - name falls back displayName -> name -> slug,
//   - description falls back summary -> description -> "",
//   - version carries through as a trimmed string.
func parseSummaries(payload []byte, source string, listKeys ...string) ([]SkillSummary, error) {
	var document any
	if err := json.Unmarshal(payload, &document); err != nil {
		return nil, fmt.Errorf("%w: invalid JSON from SkillHub %s: %v", ErrMarket, source, err)
	}
	object, ok := document.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%w: SkillHub %s response must contain a %s array", ErrMarket, source, strings.Join(listKeys, "/"))
	}
	var rawResults []any
	for _, key := range listKeys {
		if list, ok := object[key].([]any); ok {
			rawResults = list
			break
		}
	}
	if rawResults == nil {
		return nil, fmt.Errorf("%w: SkillHub %s response must contain a %s array", ErrMarket, source, strings.Join(listKeys, "/"))
	}

	results := make([]SkillSummary, 0, len(rawResults))
	for _, raw := range rawResults {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		slug := strings.TrimSpace(firstTruthyString(entry, "slug"))
		if slug == "" {
			continue
		}
		// Name chain mirrors Python exactly:
		// str(displayName or name or slug).strip() or slug — the `or` chain
		// picks its winner first (a whitespace displayName wins and blocks
		// the name fallback), then the strip can only fall back to slug.
		name := firstTruthyString(entry, "displayName")
		if name == "" {
			name = firstTruthyString(entry, "name")
		}
		if name == "" {
			name = slug
		}
		name = strings.TrimSpace(name)
		if name == "" {
			name = slug
		}
		summary := SkillSummary{
			Slug:        slug,
			Name:        name,
			Description: strings.TrimSpace(firstTruthyString(entry, "summary", "description")),
			Version:     strings.TrimSpace(firstTruthyString(entry, "version")),
			Raw:         stringifyEntry(entry),
		}
		summary.Raw["slug"] = slug
		summary.Raw["name"] = summary.Name
		summary.Raw["description"] = summary.Description
		summary.Raw["version"] = summary.Version
		results = append(results, summary)
	}
	return results, nil
}

// firstTruthyString returns the first key whose JSON value is truthy in the
// Python sense (not null / false / 0 / ""), rendered as a string. Python's
// `raw.get(k) or fallback` semantics, so JSON numbers use their short form
// and falsy values never win the chain.
func firstTruthyString(entry map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := entry[key]; ok && isTruthyJSON(value) {
			return jsonValueString(value)
		}
	}
	return ""
}

func isTruthyJSON(value any) bool {
	switch v := value.(type) {
	case nil:
		return false
	case string:
		return v != ""
	case bool:
		return v
	case float64:
		return v != 0
	default:
		return true
	}
}

// jsonValueString renders one JSON scalar for the Raw display map.
func jsonValueString(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case bool:
		return strconv.FormatBool(v)
	case float64:
		return strconv.FormatFloat(v, 'g', -1, 64)
	default:
		encoded, err := json.Marshal(v)
		if err != nil {
			return ""
		}
		return string(encoded)
	}
}

// stringifyEntry copies the original entry into the Raw display map with
// every value rendered as a string.
func stringifyEntry(entry map[string]any) map[string]string {
	raw := make(map[string]string, len(entry))
	for key, value := range entry {
		raw[key] = jsonValueString(value)
	}
	return raw
}
