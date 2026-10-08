// Package service - Craft controlled preview (W02: 独立 origin 受控预览).
//
// CraftPreviewService serves one immutable artifact version's files on an
// isolated https origin through short, bearer-capability URLs:
//
//   - Issue: one authenticated main-origin request validates the caller's
//     scope against the version and mints a 256-bit one-time redemption
//     ticket. Only the ticket's digest is kept — never the ticket itself, and
//     neither tickets nor capabilities ever enter persistent events or logs.
//   - Redeem: the first request on the isolated origin consumes the ticket
//     and mints a distinct read-only capability for the same version. Both
//     grants expire after PreviewTicketTTL (5 minutes); refresh always goes
//     back through the main origin's authenticated issuance.
//   - Resolve: every single file read re-validates the capability, re-loads
//     the version in the capability's scope, and serves only files listed in
//     that version's manifest. There is no third cookie, no session, and no
//     way to address any other version or tenant through the same URL.
//
// The preview serves immutable static web artifacts only. Kinds whose
// deliverables need a backing application answer ErrUnsupported — the preview
// never proxies a model-supplied host, port or URL.
package service

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/sandbox"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/moby/moby/client"
)

// maxCraftPreviewGrants bounds the in-memory grant tables. Live grants are
// ephemeral by design (5 minute TTL, lazy purge); the cap keeps a runaway
// issuer from growing process memory unbounded.
const maxCraftPreviewGrants = 1 << 16

// CraftPreviewConfig configures the controlled preview. AppOrigin and
// PreviewOrigin are bare origins; the preview must remain disabled until
// PreviewOrigin is configured (production) — a local HTTPS test origin is
// configured separately.
type CraftPreviewConfig struct {
	// AppOrigin is the main application origin, e.g. https://app.example.com.
	AppOrigin string
	// PreviewOrigin is the isolated preview origin, e.g.
	// https://preview.example.com. Empty disables the feature.
	PreviewOrigin string
	// TTL bounds both the redemption ticket and the read capability.
	// Defaults to craft.PreviewTicketTTL.
	TTL time.Duration
	// Now overrides the clock in tests.
	Now func() time.Time
	// AccessChecker supplies current Task membership at issuance and each read.
	// An unwired deployment refuses preview grants.
	AccessChecker craft.TaskAccessChecker
	// NetworkChecker verifies the bound Task sandbox's effective network policy.
	// Nil fails closed; the default Docker bridge is never accepted.
	NetworkChecker CraftPreviewNetworkChecker
	// BrowserNavigationProtected is an explicit deployment gate. A page served
	// to an end-user browser can navigate that browser to arbitrary hosts; CSP
	// fetch directives and iframe sandbox flags do not prevent that request.
	// Keep false until preview runs behind a browser/network boundary that does.
	BrowserNavigationProtected bool
}

type CraftPreviewNetworkChecker interface {
	CheckPreviewNoEgress(context.Context, craft.Scope) error
}

// CraftPreviewDockerNetworkChecker reads the *bound* sandbox config, not a
// caller-selected config. It accepts only Docker's network_mode=none. A stale
// binding, missing config, provider switch, or store error refuses preview.
type CraftPreviewDockerNetworkChecker struct {
	Bindings sandbox.SessionSandboxBindingStore
	Loader   sandbox.TenantSandboxConfigLoader
	Global   *sandbox.Config
	// Inspector must query the bound container's live HostConfig.NetworkMode on
	// the resolved Docker daemon. A stored config can change after creation.
	Inspector CraftPreviewContainerNetworkInspector
}

type CraftPreviewContainerNetworkInspector interface {
	NetworkMode(context.Context, *sandbox.Config, string) (string, error)
}

// CraftPreviewLocalDockerInspector verifies the live container on a local
// Docker daemon. Remote daemon endpoints are refused until a guarded TLS
// inspector is wired; a stored policy value alone cannot attest an old
// container that was created while the config still used bridge.
type CraftPreviewLocalDockerInspector struct{}

// craftPreviewInspectTimeout bounds one inspect RPC, mirroring the sandbox
// module's DefaultDockerHTTPTimeout convention for short Engine API calls:
// a hung local daemon must not pin a preview worker until the browser
// drops the connection.
const craftPreviewInspectTimeout = 30 * time.Second

// craftPreviewDockerClients caches one shared client per daemon host. The
// no-egress check runs per preview resource request; building (and
// closing) a fresh client per call would re-dial the unix socket and redo
// the lazy API-version negotiation every time (the sandbox module keeps
// the same per-endpoint pool discipline in dockerEngineClientPool).
var craftPreviewDockerClients struct {
	mu      sync.Mutex
	clients map[string]*client.Client
}

func craftPreviewDockerClient(host string) (*client.Client, error) {
	craftPreviewDockerClients.mu.Lock()
	defer craftPreviewDockerClients.mu.Unlock()
	if craftPreviewDockerClients.clients == nil {
		craftPreviewDockerClients.clients = make(map[string]*client.Client)
	}
	if existing, ok := craftPreviewDockerClients.clients[host]; ok {
		return existing, nil
	}
	built, err := client.New(client.WithHost(host))
	if err != nil {
		return nil, err
	}
	craftPreviewDockerClients.clients[host] = built
	return built, nil
}

func (CraftPreviewLocalDockerInspector) NetworkMode(ctx context.Context, config *sandbox.Config, sandboxID string) (string, error) {
	if config == nil || sandboxID == "" {
		return "", fmt.Errorf("%w: Docker preview inspect needs config and container", craft.ErrUnsupported)
	}
	host := strings.TrimSpace(config.DockerHost)
	if host == "" {
		host = sandbox.DetectLocalDockerHost()
	}
	if !strings.HasPrefix(host, "unix://") {
		return "", fmt.Errorf("%w: remote Docker preview inspection is not configured", craft.ErrUnsupported)
	}
	api, err := craftPreviewDockerClient(host)
	if err != nil {
		return "", err
	}
	inspectCtx, cancel := context.WithTimeout(ctx, craftPreviewInspectTimeout)
	defer cancel()
	inspected, err := api.ContainerInspect(inspectCtx, sandboxID, client.ContainerInspectOptions{})
	if err != nil {
		return "", err
	}
	if inspected.Container.HostConfig == nil || inspected.Container.State == nil || !inspected.Container.State.Running {
		return "", fmt.Errorf("%w: Docker preview container is not running", craft.ErrUnsupported)
	}
	return string(inspected.Container.HostConfig.NetworkMode), nil
}

// optionalErr renders an underlying error for a refusal message without
// leaking a "<nil>" tail when the refusal came from a condition (nil
// binding, missing config, wrong mode) rather than a failed call. These
// messages surface to authenticated callers and must stay decipherable.
func optionalErr(err error) string {
	if err == nil {
		return ""
	}
	return ": " + err.Error()
}

func (n CraftPreviewDockerNetworkChecker) CheckPreviewNoEgress(ctx context.Context, scope craft.Scope) error {
	if n.Bindings == nil || n.Loader == nil || n.Global == nil || n.Inspector == nil || scope.TenantID == 0 || scope.SessionID == "" {
		return fmt.Errorf("%w: preview network policy is unavailable", craft.ErrUnsupported)
	}
	binding, err := n.Bindings.Get(ctx, sandbox.SessionSandboxKey{TenantID: scope.TenantID, SessionID: scope.SessionID})
	if err != nil || binding == nil || binding.StaleAt != nil || binding.Provider != sandbox.SandboxTypeDocker || binding.ConfigID == "" {
		return fmt.Errorf("%w: no current Docker sandbox binding%s", craft.ErrUnsupported, optionalErr(err))
	}
	config := n.Global
	if binding.ConfigID != types.SandboxConfigIDGlobalDefault {
		selected, loadErr := n.Loader.Load(ctx, scope.TenantID, binding.ConfigID)
		if loadErr != nil || !selected.Found || selected.Cordoned || selected.Config == nil {
			return fmt.Errorf("%w: bound sandbox config is unavailable%s", craft.ErrUnsupported, optionalErr(loadErr))
		}
		config, err = sandbox.ResolveEffectiveConfig(selected.Config, n.Global)
		if err != nil {
			return fmt.Errorf("%w: resolve bound sandbox config: %v", craft.ErrUnsupported, err)
		}
	}
	if config.Type != sandbox.SandboxTypeDocker || !strings.EqualFold(strings.TrimSpace(config.DockerNetworkMode), "none") {
		return fmt.Errorf("%w: bound preview sandbox must use Docker network_mode=none", craft.ErrUnsupported)
	}
	actual, err := n.Inspector.NetworkMode(ctx, config, binding.SandboxID)
	if err != nil || !strings.EqualFold(strings.TrimSpace(actual), "none") {
		return fmt.Errorf("%w: bound Docker container is not running with network_mode=none%s", craft.ErrUnsupported, optionalErr(err))
	}
	return nil
}

func (c CraftPreviewConfig) withDefaults() CraftPreviewConfig {
	if c.TTL <= 0 {
		c.TTL = craft.PreviewTicketTTL
	}
	if c.Now == nil {
		c.Now = time.Now
	}
	return c
}

// craftPreviewGrant is one live grant (ticket or capability) tracked by the
// digest of its opaque token. The token itself is never stored.
type craftPreviewGrant struct {
	scope     craft.Scope
	versionID string
	expiresAt time.Time
}

// CraftPreviewService implements the W02 controlled preview.
type CraftPreviewService struct {
	versions craft.VersionStore
	files    interfaces.FileService
	checks   craft.PreviewCheckStore
	config   CraftPreviewConfig

	mu      sync.Mutex
	tickets map[string]craftPreviewGrant // digest → grant
	caps    map[string]craftPreviewGrant // digest → grant
	// doorCache dedups the per-resource access/no-egress doors inside one
	// capability burst (short TTL, successes only).
	doorCache map[craft.Scope]craftPreviewDoorResult
	// versionFiles caches ONE shared allowlist per immutable version id.
	// Grants reference their version instead of freezing a private copy of
	// the manifest: the grant tables stay bounded by maxCraftPreviewGrants
	// times an O(1) reference, while the allowlists are bounded by the
	// number of DISTINCT published versions actually previewed.
	versionFiles map[string]map[string]struct{}
}

// NewCraftPreviewService assembles the preview service. versions and files
// are required; checks may be nil (verdict recording is then unavailable and
// the evidence source reports not_run, but serving still works). Configuring
// a preview origin that is not a distinct https origin from the app origin is
// an assembly bug and panics here, mirroring the artifact service.
func NewCraftPreviewService(
	versions craft.VersionStore,
	files interfaces.FileService,
	checks craft.PreviewCheckStore,
	config CraftPreviewConfig,
) *CraftPreviewService {
	if versions == nil || files == nil {
		panic("craft: NewCraftPreviewService requires a version store and file service")
	}
	config = config.withDefaults()
	if config.AppOrigin != "" && config.PreviewOrigin != "" &&
		!craft.PreviewOriginAllowed(config.AppOrigin, config.PreviewOrigin) {
		panic(fmt.Sprintf("craft: preview origin %q must be a distinct https origin from app origin %q",
			config.PreviewOrigin, config.AppOrigin))
	}
	if config.AppOrigin != "" && config.PreviewOrigin != "" &&
		previewBareHostname(config.AppOrigin) != "" &&
		strings.EqualFold(previewBareHostname(config.AppOrigin), previewBareHostname(config.PreviewOrigin)) {
		// Host-only cookies are shared across PORTS: a preview origin that
		// differs from the app origin only by port spelling (or by port at
		// all) is not an isolation boundary for the unauthenticated preview
		// routes.
		panic(fmt.Sprintf("craft: preview origin %q shares host %q with app origin %q; the preview origin must live on a distinct hostname",
			config.PreviewOrigin, previewBareHostname(config.PreviewOrigin), config.AppOrigin))
	}
	return &CraftPreviewService{
		versions:     versions,
		files:        files,
		checks:       checks,
		config:       config,
		tickets:      map[string]craftPreviewGrant{},
		caps:         map[string]craftPreviewGrant{},
		doorCache:    map[craft.Scope]craftPreviewDoorResult{},
		versionFiles: map[string]map[string]struct{}{},
	}
}

// Enabled reports whether the preview origin is configured. Unconfigured
// deployments keep the feature off: issuance answers ErrUnsupported and the
// isolated-origin endpoint 404s.
func (s *CraftPreviewService) Enabled() bool {
	return s != nil && craft.PreviewOriginAllowed(s.config.AppOrigin, s.config.PreviewOrigin)
}

// AcceptsPreviewHost prevents the auth-free /p route mounted on the shared
// router from redeeming a ticket on the main product host.
// trimDefaultPort strips a port that is the scheme's default (https=443,
// http=80). RFC 7230 allows clients and some reverse proxies to send the
// default port explicitly; the origin config usually omits it, and a bare
// EqualFold would silently 404 the shared /p route for those deployments.
// The result keeps the bracketed form for IPv6 literals ("[::1]"), because
// that is the only legal spelling inside a URL authority or Host header —
// comparing a de-bracketed "::1" against a configured "[::1]" would never
// match in either direction.
func trimDefaultPort(host, scheme string) string {
	if h, p, err := net.SplitHostPort(host); err == nil {
		if (scheme == "https" && p == "443") || (scheme == "http" && p == "80") {
			if strings.Contains(h, ":") {
				return "[" + h + "]"
			}
			return h
		}
	}
	return host
}

func (s *CraftPreviewService) AcceptsPreviewHost(host string) bool {
	if !s.Enabled() {
		return false
	}
	u, err := url.Parse(s.config.PreviewOrigin)
	if err != nil {
		return false
	}
	// Normalize both sides: the config side may omit the default port while
	// the request carries it (or vice versa).
	if !strings.EqualFold(trimDefaultPort(host, u.Scheme), trimDefaultPort(u.Host, u.Scheme)) {
		return false
	}
	// Bare-hostname overlap guard: a request on the APP origin's hostname
	// (whatever port spelling) must never reach the unauthenticated preview
	// routes — Host-only session cookies follow the hostname, not the port.
	if app, err := url.Parse(s.config.AppOrigin); err == nil && app.Host != "" {
		requestBare := previewBareHostname("https://" + host)
		if strings.EqualFold(requestBare, previewBareHostname(s.config.AppOrigin)) &&
			!strings.EqualFold(previewBareHostname(s.config.PreviewOrigin), previewBareHostname(s.config.AppOrigin)) {
			return false
		}
	}
	return true
}

// previewBareHostname reduces an origin (or host:port) to its bare hostname:
// default port trimmed, any remaining port split off, IPv6 brackets removed.
func previewBareHostname(origin string) string {
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return ""
	}
	host := trimDefaultPort(u.Host, u.Scheme)
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	return strings.Trim(host, "[]")
}

// Issue mints one redemption ticket for a published version after the full
// version ACL ran in the requesting scope. The ticket URL points at the
// isolated preview origin. Issuing is not verification: the preview check
// only turns passed when real page verification records its verdict (W06),
// never here.
func (s *CraftPreviewService) Issue(ctx context.Context, scope craft.Scope, versionID string) (craft.PreviewTicket, error) {
	if !s.Enabled() {
		return craft.PreviewTicket{}, fmt.Errorf("%w: preview origin is not configured", craft.ErrUnsupported)
	}
	if !s.config.BrowserNavigationProtected {
		return craft.PreviewTicket{}, fmt.Errorf("%w: preview disabled until browser navigation egress is isolated", craft.ErrUnsupported)
	}
	if scope.TenantID == 0 || scope.UserID == "" || scope.SessionID == "" {
		return craft.PreviewTicket{}, fmt.Errorf("%w: preview issuance requires a complete scope", craft.ErrInvalidInput)
	}
	if !craft.IsVersionID(versionID) {
		return craft.PreviewTicket{}, fmt.Errorf("%w: version id %q", craft.ErrInvalidInput, versionID)
	}
	if err := craft.RequireTaskAccess(ctx, s.config.AccessChecker, scope, craft.TaskPreview); err != nil {
		return craft.PreviewTicket{}, err
	}
	if err := s.requireNoEgress(ctx, scope); err != nil {
		return craft.PreviewTicket{}, err
	}
	version, err := s.versions.Get(ctx, scope, versionID)
	if err != nil {
		return craft.PreviewTicket{}, err
	}
	if !craft.PreviewableKind(version.Kind) {
		return craft.PreviewTicket{}, fmt.Errorf("%w: 不支持此预览类型: kind %q 的产物需要后端应用，受控预览仅服务不可变静态网页产物，不代理任意端口或 URL",
			craft.ErrUnsupported, version.Kind)
	}
	entry, ok := craft.EntryPath(version.Kind)
	if !ok {
		return craft.PreviewTicket{}, fmt.Errorf("%w: kind %q has no entry file", craft.ErrUnsupported, version.Kind)
	}
	allowedFiles := s.sharedVersionFiles(version.ID, version.Files)
	if _, ok := allowedFiles[entry]; !ok {
		return craft.PreviewTicket{}, fmt.Errorf("%w: entry is absent from version manifest", craft.ErrNotFound)
	}

	token, digest, err := craft.NewPreviewToken()
	if err != nil {
		return craft.PreviewTicket{}, err
	}
	now := s.config.Now()
	expiresAt := now.Add(s.config.TTL)
	if err := s.putTicket(digest, craftPreviewGrant{scope: scope, versionID: version.ID, expiresAt: expiresAt}); err != nil {
		return craft.PreviewTicket{}, err
	}
	base := strings.TrimSuffix(s.config.PreviewOrigin, "/")
	url := base + "/" + craft.PreviewPathSegment + "/" + token + "/" + entry
	// No events, no persistent logs: the ticket never appears anywhere but
	// the authorized response body.
	return craft.PreviewTicket{URL: url, ExpiresAt: expiresAt, VersionID: version.ID}, nil
}

// PreviewOpen is one answered request on the isolated origin: either a
// redirect that exchanges a live ticket into its fresh capability, or the
// resolved file with an open reader.
type PreviewOpen struct {
	// Redirect is the capability URL to follow (ticket redemption). The
	// redirected path keeps the requested file so relative resources inherit
	// the capability path.
	Redirect string
	// File is the resolved manifest entry (set when Redirect is empty).
	File craft.File
	// Reader streams the file's pinned bytes (set when Redirect is empty).
	Reader io.ReadCloser
}

// Open answers one /p/<token>/<file> request: a still-live ticket is redeemed
// exactly once — the second hit on the same ticket no longer exists — and
// answers a redirect to the same path under the fresh capability; a
// capability serves the requested file of its bound version.
func (s *CraftPreviewService) Open(ctx context.Context, token, requestPath string) (PreviewOpen, error) {
	if !s.Enabled() {
		return PreviewOpen{}, fmt.Errorf("%w: preview origin is not configured", craft.ErrUnsupported)
	}
	if !s.config.BrowserNavigationProtected {
		return PreviewOpen{}, fmt.Errorf("%w: browser navigation egress is not isolated", craft.ErrUnsupported)
	}
	rel, err := craft.ValidatePreviewRequestPath(requestPath)
	if err != nil {
		return PreviewOpen{}, err
	}
	digest := craft.PreviewTokenDigest(token)

	// Ticket redemption first: unconsumed, unexpired tickets only.
	s.mu.Lock()
	grant, ok := s.tickets[digest]
	if ok {
		delete(s.tickets, digest) // one time, always: consumed is indistinguishable from unknown
	}
	s.mu.Unlock()
	if ok {
		if s.config.Now().After(grant.expiresAt) {
			return PreviewOpen{}, fmt.Errorf("%w: preview ticket expired, re-authorize on the main origin", craft.ErrForbidden)
		}
		if err := craft.RequireTaskAccess(ctx, s.config.AccessChecker, grant.scope, craft.TaskPreview); err != nil {
			return PreviewOpen{}, err
		}
		if err := s.requireNoEgress(ctx, grant.scope); err != nil {
			return PreviewOpen{}, err
		}
		capToken, capDigest, err := craft.NewPreviewToken()
		if err != nil {
			return PreviewOpen{}, err
		}
		expiresAt := s.config.Now().Add(s.config.TTL)
		if err := s.putCapability(capDigest, craftPreviewGrant{scope: grant.scope, versionID: grant.versionID, expiresAt: expiresAt}); err != nil {
			return PreviewOpen{}, err
		}
		return PreviewOpen{Redirect: "/" + craft.PreviewPathSegment + "/" + capToken + "/" + rel}, nil
	}

	file, _, grant, err := s.lookup(ctx, digest, rel)
	if err != nil {
		return PreviewOpen{}, err
	}
	reader, err := s.openReader(ctx, grant, file)
	if err != nil {
		return PreviewOpen{}, err
	}
	return PreviewOpen{File: file, Reader: reader}, nil
}

// Resolve validates one capability against one version-relative path and
// returns the manifest entry — the brief's serving contract. It performs the
// same per-read validation as Open without opening bytes.
func (s *CraftPreviewService) Resolve(ctx context.Context, token, path string) (craft.File, error) {
	if !s.Enabled() {
		return craft.File{}, fmt.Errorf("%w: preview origin is not configured", craft.ErrUnsupported)
	}
	rel, err := craft.ValidatePreviewRequestPath(path)
	if err != nil {
		return craft.File{}, err
	}
	file, _, _, err := s.lookup(ctx, craft.PreviewTokenDigest(token), rel)
	return file, err
}

// lookup resolves a capability digest and a canonical file path to the
// version's manifest entry. Every read re-validates: the grant must be live,
// the version must still exist in the grant's scope, and the file must be
// part of that version's manifest — an entry that only exists in a NEWER
// version of the workspace stays a 404.
func (s *CraftPreviewService) lookup(ctx context.Context, digest, rel string) (craft.File, craft.Scope, craftPreviewGrant, error) {
	s.mu.Lock()
	grant, ok := s.caps[digest]
	if ok && s.config.Now().After(grant.expiresAt) {
		delete(s.caps, digest)
		ok = false
	}
	s.mu.Unlock()
	if !ok {
		return craft.File{}, craft.Scope{}, craftPreviewGrant{}, fmt.Errorf("%w: no preview capability", craft.ErrNotFound)
	}
	// The per-capability allowlist is O(1) local state frozen from the
	// immutable version at issuance, so it gates FIRST: favicon probes and
	// stale relative references answer 404 without paying the two
	// I/O-heavy per-resource doors (membership store, then binding store +
	// config load + live container inspect). Authorized manifest paths
	// still run both doors before any byte is served — the order change
	// only short-circuits guaranteed-404 requests.
	if allowlist, cached := s.sharedVersionAllowlist(grant.versionID); cached {
		if _, ok := allowlist[rel]; !ok {
			return craft.File{}, craft.Scope{}, craftPreviewGrant{}, fmt.Errorf("%w: file is outside preview capability", craft.ErrNotFound)
		}
	}
	if err := s.requireFreshPreviewDoors(ctx, grant.scope); err != nil {
		return craft.File{}, craft.Scope{}, craftPreviewGrant{}, err
	}
	version, err := s.versions.Get(ctx, grant.scope, grant.versionID)
	if err != nil {
		return craft.File{}, craft.Scope{}, craftPreviewGrant{}, err
	}
	for _, f := range version.Files {
		if f.Path == rel {
			return f, grant.scope, grant, nil
		}
	}
	return craft.File{}, craft.Scope{}, craftPreviewGrant{},
		fmt.Errorf("%w: %q is not part of version %s", craft.ErrNotFound, rel, grant.versionID)
}

// craftPreviewDoorCacheTTL bounds how long a successful (access, no-egress)
// door pair is reused for the same scope: one preview page fetches dozens of
// static assets in a burst, and re-running membership + binding + live
// Docker inspect per asset pins workers when the daemon is slow. Failures
// are NEVER cached — a revocation or a flapping daemon takes effect on the
// very next request; the TTL only bounds how long a fresh success may be
// reused, so revocation windows stay in the same order as the TTL.
const craftPreviewDoorCacheTTL = 2 * time.Second

type craftPreviewDoorResult struct {
	ok bool
	at time.Time
}

// requireFreshPreviewDoors runs the two I/O-heavy doors with a per-scope
// short-TTL success cache (per-resource dedup inside a capability burst).
func (s *CraftPreviewService) requireFreshPreviewDoors(ctx context.Context, scope craft.Scope) error {
	// Membership/revocation stays LIVE on every read (a revoked grant must
	// end a live capability immediately — the cheap indexed query); only the
	// expensive no-egress door (binding load + live Docker inspect, up to
	// 30s) is deduped within the short TTL window.
	if err := craft.RequireTaskAccess(ctx, s.config.AccessChecker, scope, craft.TaskPreview); err != nil {
		return err
	}
	now := s.config.Now()
	s.mu.Lock()
	if cached, ok := s.doorCache[scope]; ok {
		if cached.ok && now.Sub(cached.at) < craftPreviewDoorCacheTTL {
			s.mu.Unlock()
			return nil
		}
		delete(s.doorCache, scope)
	}
	s.mu.Unlock()
	if err := s.requireNoEgress(ctx, scope); err != nil {
		return err
	}
	// Timestamp AFTER the expensive door: the door itself can take up to
	// 30s (live Docker inspect) while the TTL is 2s — stamping before it
	// would expire the entry at insert time exactly in the slow-daemon
	// scenario the cache exists for.
	s.mu.Lock()
	if len(s.doorCache) > 1024 {
		s.doorCache = make(map[craft.Scope]craftPreviewDoorResult)
	}
	s.doorCache[scope] = craftPreviewDoorResult{ok: true, at: s.config.Now()}
	s.mu.Unlock()
	return nil
}

func (s *CraftPreviewService) requireNoEgress(ctx context.Context, scope craft.Scope) error {
	if s.config.NetworkChecker == nil {
		return fmt.Errorf("%w: preview network checker is not assembled", craft.ErrUnsupported)
	}
	return s.config.NetworkChecker.CheckPreviewNoEgress(ctx, scope)
}

// openReader streams one resolved file through the tenant's file service,
// reading as the file's owning tenant. The read is additionally bounded by
// the manifest's recorded size so a swapped blob cannot stream past the
// pinned entry.
func (s *CraftPreviewService) openReader(ctx context.Context, grant craftPreviewGrant, file craft.File) (io.ReadCloser, error) {
	readCtx := types.WithExecutionTenant(ctx, grant.scope.TenantID)
	reader, err := s.files.GetFile(readCtx, file.Ref)
	if err != nil {
		return nil, fmt.Errorf("craft: open preview object: %w", err)
	}
	return newBoundedReadCloser(reader, file.Bytes), nil
}

// maxCraftPreviewVersionCache bounds the shared per-version allowlist cache;
// beyond it new versions simply skip the O(1) 404 short-circuit and pay the
// full per-read validation path.
const maxCraftPreviewVersionCache = 1 << 12

// sharedVersionFiles returns the shared allowlist for one immutable version,
// populating the cache on first use. Versions are immutable, so one map is
// safely shared by every grant, ticket and capability of that version.
func (s *CraftPreviewService) sharedVersionFiles(versionID string, files []craft.File) map[string]struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	if cached, ok := s.versionFiles[versionID]; ok {
		return cached
	}
	allowlist := make(map[string]struct{}, len(files))
	for _, file := range files {
		allowlist[file.Path] = struct{}{}
	}
	if len(s.versionFiles) < maxCraftPreviewVersionCache {
		s.versionFiles[versionID] = allowlist
	}
	return allowlist
}

// sharedVersionAllowlist reports the cached allowlist for a version, if any.
func (s *CraftPreviewService) sharedVersionAllowlist(versionID string) (map[string]struct{}, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cached, ok := s.versionFiles[versionID]
	return cached, ok
}

// putTicket stores one ticket grant, purging expired entries first.
func (s *CraftPreviewService) putTicket(digest string, grant craftPreviewGrant) error {
	return s.putGrant(s.tickets, digest, grant)
}

// putCapability stores one capability grant, purging expired entries first.
func (s *CraftPreviewService) putCapability(digest string, grant craftPreviewGrant) error {
	return s.putGrant(s.caps, digest, grant)
}

func (s *CraftPreviewService) putGrant(table map[string]craftPreviewGrant, digest string, grant craftPreviewGrant) error {
	now := s.config.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	for d, g := range table {
		if now.After(g.expiresAt) {
			delete(table, d)
		}
	}
	if len(table) >= maxCraftPreviewGrants {
		return fmt.Errorf("%w: too many live preview grants", craft.ErrBusy)
	}
	table[digest] = grant
	return nil
}

// RecordPreviewVerdict records the externally observed preview verdict for
// one version through the W02 update channel — the only way a published
// version's preview check moves off not_run. A verdict of passed without a
// run is rejected: facts that never happened are never recorded. Issuing a
// ticket does NOT call this; the verdict belongs to real page verification
// (W06 loads the page; only then does the check pass).
func (s *CraftPreviewService) RecordPreviewVerdict(ctx context.Context, scope craft.Scope, versionID string, ran, passed bool) error {
	if s == nil || s.checks == nil {
		return fmt.Errorf("%w: preview check update channel is not assembled", craft.ErrUnsupported)
	}
	if !ran && passed {
		return fmt.Errorf("%w: a preview verdict cannot pass without running", craft.ErrInvalidInput)
	}
	version, err := s.versions.Get(ctx, scope, versionID)
	if err != nil {
		return err
	}
	if !craft.PreviewableKind(version.Kind) {
		return fmt.Errorf("%w: kind %q has no controlled preview", craft.ErrUnsupported, version.Kind)
	}
	check := craft.PreviewCheckFromEvidence(ran, passed)
	for _, existing := range version.Checks {
		if existing == check {
			return nil // idempotent replay
		}
	}
	_, err = s.checks.UpdatePreviewCheck(ctx, scope, versionID, check)
	if err != nil {
		logger.Warnf(ctx, "[CraftPreview] preview check update failed for version %s: %v", versionID, err)
	}
	return err
}

// EvidenceSource adapts the recorded preview verdicts into W01's
// ArtifactEvidenceSource injection point. Truthfulness rules: a verdict is
// carried into a collection round only when the workspace has exactly one
// published version for that run — a re-collection retry of the same run
// re-derives the same version identity, so the recorded verdict is a fact
// about exactly the bytes being re-published. Multiple versions under one
// run mean content changed mid-run; the source then reports nothing and the
// fresh version's preview check stays not_run until it is really verified.
func (s *CraftPreviewService) EvidenceSource() ArtifactEvidenceSource {
	return func(ctx context.Context, task craft.Task) craft.ArtifactEvidence {
		versions, err := s.versions.List(ctx, task.Scope)
		if err != nil {
			return craft.ArtifactEvidence{}
		}
		var match *craft.Version
		for i := range versions {
			v := versions[i]
			if v.WorkspaceID != task.WorkspaceID || v.RunID != task.Fence.RunID {
				continue
			}
			if match != nil {
				return craft.ArtifactEvidence{} // ambiguous: content changed within the run
			}
			match = &versions[i]
		}
		if match == nil {
			return craft.ArtifactEvidence{}
		}
		evidence := craft.ArtifactEvidence{}
		for _, c := range match.Checks {
			if c.Name != craft.CheckPreview {
				continue
			}
			switch c.Status {
			case craft.CheckPassed:
				evidence.PreviewRan, evidence.PreviewPassed = true, true
			case craft.CheckFailed:
				evidence.PreviewRan = true
			}
		}
		return evidence
	}
}

// PreviewCSP renders the Content-Security-Policy served on every preview
// response. Resources come from the controlled preview origin only, network
// and worker registration are denied (connect-src/worker-src none), forms
// are inert, and only the main app origin may embed the preview.
func (s *CraftPreviewService) PreviewCSP() string {
	ancestors := "'none'"
	if s != nil && strings.TrimSpace(s.config.AppOrigin) != "" {
		ancestors = strings.TrimSpace(s.config.AppOrigin)
	}
	return "default-src 'none'; " +
		"script-src 'self' 'unsafe-inline'; " +
		"style-src 'self' 'unsafe-inline'; " +
		"img-src 'self' data: blob:; " +
		"font-src 'self'; " +
		"media-src 'self'; " +
		"connect-src 'none'; " +
		"object-src 'none'; " +
		"base-uri 'none'; " +
		"form-action 'none'; " +
		"frame-ancestors " + ancestors + "; " +
		"worker-src 'none'; " +
		"child-src 'none'"
}

// boundedReadCloser caps one stream at n bytes; an overrun is a hard error
// because the pinned manifest entry promised this exact size.
type boundedReadCloser struct {
	inner     io.ReadCloser
	remaining int64
}

func newBoundedReadCloser(inner io.ReadCloser, n int64) *boundedReadCloser {
	if n < 0 {
		n = 0
	}
	return &boundedReadCloser{inner: inner, remaining: n}
}

func (b *boundedReadCloser) Read(p []byte) (int, error) {
	if b.remaining <= 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > b.remaining {
		p = p[:b.remaining]
	}
	n, err := b.inner.Read(p)
	b.remaining -= int64(n)
	if err == nil && b.remaining <= 0 {
		// Emit EOF together with the final bytes; a longer underlying stream
		// is masked, never served.
		return n, io.EOF
	}
	return n, err
}

func (b *boundedReadCloser) Close() error { return b.inner.Close() }
