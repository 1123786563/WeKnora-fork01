package plugins

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/utils"
)

// ErrOAuthProtectedEndpoint is the sentinel for "the plugin's MCP endpoint
// cannot be verified because it demands OAuth authorization" — the endpoint
// answered the handshake with a 401 advertising RFC 9728 protected-resource
// metadata. The production adapter (internal/container's
// NewPluginMCPEndpointLister) wraps the MCP layer's OAuthRequiredError into
// this sentinel while keeping the underlying cause, so both identities
// survive. A preview must fail loudly here — there is no such thing as a
// half-verified preview.
var ErrOAuthProtectedEndpoint = errors.New("plugin endpoint requires OAuth authorization")

// IsOAuthProtected reports whether err (or anything it wraps) carries the
// ErrOAuthProtectedEndpoint sentinel, i.e. the production adapter recognized
// the MCP layer's OAuthRequiredError during verification.
func IsOAuthProtected(err error) bool {
	return errors.Is(err, ErrOAuthProtectedEndpoint)
}

// EndpointLister performs a LIVE ListTools against a plugin's MCP endpoint.
// It is a seam: production uses internal/container's
// NewPluginMCPEndpointLister over the airesource MCPManager (modules must not
// import each other — the composition root is the only legal glue point);
// tests substitute fakes to assert zero-call guarantees. Adapters that hit an
// OAuth-protected endpoint MUST wrap the error with ErrOAuthProtectedEndpoint.
type EndpointLister func(ctx context.Context, transportType string, endpointURL string) ([]*types.MCPTool, error)

// maxManifestBytes bounds the manifest download (1 MiB): an untrusted URL
// must not be able to make WeKnora buffer arbitrary amounts of data.
var maxManifestBytes = 1 << 20

// FetchResult is the verified outcome of FetchAndVerify: the parsed manifest,
// the live tool directory it was checked against, and the authoritative
// snapshot derived from the live data.
type FetchResult struct {
	Manifest            *types.PluginManifest
	LiveTools           []*types.MCPTool
	Snapshot            []types.PluginToolSnapshot
	ToolsDigest         string
	IdentityFingerprint string
}

// FetchAndVerify downloads a weknora.plugin/1 manifest and verifies that its
// declared tool directory matches the LIVE endpoint. Order of operations is
// security-relevant: both the manifest URL and the manifest-declared endpoint
// must pass SSRF validation BEFORE any request is sent to them.
func FetchAndVerify(ctx context.Context, manifestURL string, lister EndpointLister) (*FetchResult, error) {
	// 1) SSRF gate on the manifest URL: http/https only, and reject
	// localhost/loopback/private/reserved targets before any dial.
	if err := utils.ValidateURLForSSRF(manifestURL); err != nil {
		return nil, fmt.Errorf("manifest URL rejected: %w", err)
	}
	// 2) Fetch with the SSRF-safe client, size- and time-bounded.
	body, err := fetchLimited(ctx, manifestURL)
	if err != nil {
		return nil, err
	}
	// 3) Parse and validate the protocol document.
	var manifest types.PluginManifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		return nil, fmt.Errorf("invalid manifest JSON: %w", err)
	}
	if err := ValidateManifest(&manifest); err != nil {
		return nil, err
	}
	if manifest.ContentDigest != "" && manifest.ContentDigest != ManifestContentDigest(&manifest) {
		return nil, fmt.Errorf("manifest content_digest mismatch")
	}
	// 4) SSRF gate on the manifest-declared endpoint, then live verification.
	if err := utils.ValidateURLForSSRF(manifest.Transport.Endpoint); err != nil {
		return nil, fmt.Errorf("plugin endpoint rejected: %w", err)
	}
	if lister == nil {
		return nil, fmt.Errorf("endpoint lister is required")
	}
	live, err := lister(ctx, manifest.Transport.Type, manifest.Transport.Endpoint)
	if err != nil {
		if IsOAuthProtected(err) {
			// The adapter already wrapped the MCP layer's OAuthRequiredError
			// with the sentinel via double %w (both identities preserved) —
			// pass it through unchanged.
			return nil, err
		}
		return nil, fmt.Errorf("plugin endpoint verification failed: %w", err)
	}
	snapshot, toolsDigest, err := BuildVerifiedSnapshot(&manifest, live)
	if err != nil {
		return nil, err
	}
	return &FetchResult{
		Manifest:            &manifest,
		LiveTools:           live,
		Snapshot:            snapshot,
		ToolsDigest:         toolsDigest,
		IdentityFingerprint: IdentityFingerprint(manifest.PluginID, manifest.Version, manifest.Transport.Endpoint, toolsDigest),
	}, nil
}

// fetchLimited downloads at most maxManifestBytes over the SSRF-safe client.
// Only 2xx is accepted; redirects are re-validated by the client itself.
func fetchLimited(ctx context.Context, manifestURL string) ([]byte, error) {
	client := utils.NewSSRFSafeHTTPClient(utils.SSRFSafeHTTPClientConfig{
		Timeout:           15 * time.Second,
		MaxRedirects:      5,
		DisableKeepAlives: true,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, manifestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("invalid manifest URL: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("manifest fetch failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("manifest fetch failed: unexpected status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, int64(maxManifestBytes)+1))
	if err != nil {
		return nil, fmt.Errorf("manifest fetch failed: %w", err)
	}
	if len(body) > maxManifestBytes {
		return nil, fmt.Errorf("manifest exceeds %d bytes", maxManifestBytes)
	}
	return body, nil
}
