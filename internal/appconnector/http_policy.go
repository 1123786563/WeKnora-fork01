package appconnector

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// PublicAddress reports whether ip is a globally routable address. Loopback,
// RFC1918/ULA private, link-local (unicast and multicast), multicast and
// unspecified addresses are never public, so none of them may be dialed by a
// connector's outbound HTTP call without an explicit admin authorization.
func PublicAddress(ip net.IP) bool {
	return ip != nil && !ip.IsLoopback() && !ip.IsPrivate() &&
		!ip.IsLinkLocalUnicast() && !ip.IsLinkLocalMulticast() &&
		!ip.IsUnspecified() && !ip.IsMulticast()
}

// Outbound-policy rejections. Every denial names which reviewed rule was
// violated; none of them leak credential material.
var (
	// ErrUnlistedDestination: scheme/host/port or the target resource was
	// never admin-reviewed for this connection.
	ErrUnlistedDestination = errors.New("http_unlisted_destination")
	// ErrAddressNotAllowed: a resolved address (or every address of a mixed
	// DNS answer) is not a public address and not inside an admin-authorized
	// network range.
	ErrAddressNotAllowed = errors.New("http_address_not_allowed")
	// ErrTooManyRedirects: the hop limit was reached.
	ErrTooManyRedirects = errors.New("http_too_many_redirects")
)

// IPResolver resolves a hostname to its addresses. It exists so callers and
// tests can pin resolution behavior; the zero HTTPPolicy uses the default
// resolver.
type IPResolver interface {
	LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
}

// IPResolverFunc adapts a function to IPResolver (test/fixture hook).
type IPResolverFunc func(ctx context.Context, host string) ([]net.IPAddr, error)

// LookupIPAddr implements IPResolver.
func (f IPResolverFunc) LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error) {
	return f(ctx, host)
}

// HTTPPolicy is the admin-reviewed outbound contract of one connection:
// exactly one scheme, host, port, method set and target-resource (path
// prefix) rule, plus optional explicitly authorized network ranges.
//
// AuthorizedNetworks is the ONLY way a non-public address may be dialed —
// an enterprise that genuinely needs an internal endpoint grants the CIDR
// here, on the admin side. It is structurally out of reach of the model:
// the policy is built from reviewed installation configuration, never from
// Action.Args or any model-generated parameter, so model output can never
// auto-allow a network range. (Tests use the same field to authorize
// 127.0.0.0/8 for httptest servers — a documented test hook, not a
// weakening of PublicAddress.)
type HTTPPolicy struct {
	Scheme     string
	Host       string // exact reviewed hostname
	Port       string // exact reviewed port ("" = scheme default)
	Methods    []string
	PathPrefix string // target-resource rule; "" = "/"
	// AuthorizedNetworks are admin-authorized non-public ranges. Never
	// populated from model parameters.
	AuthorizedNetworks []*net.IPNet
	// Resolver overrides DNS resolution (admin/test hook).
	Resolver IPResolver
	// Timeout bounds each outbound request.
	Timeout time.Duration
}

// NewHTTPPolicy builds the reviewed policy for one method and URL.
func NewHTTPPolicy(method string, target *url.URL) HTTPPolicy {
	host := target.Hostname()
	if host == "" {
		host = target.Host
	}
	return HTTPPolicy{
		Scheme:     target.Scheme,
		Host:       host,
		Port:       target.Port(),
		Methods:    []string{strings.ToUpper(method)},
		PathPrefix: target.Path,
	}
}

// AddressAllowed reports whether one resolved address may be dialed: either
// it is a public address, or it falls inside an admin-authorized range.
func (p HTTPPolicy) AddressAllowed(ip net.IP) bool {
	if PublicAddress(ip) {
		return true
	}
	for _, n := range p.AuthorizedNetworks {
		if n != nil && n.Contains(ip) {
			return true
		}
	}
	return false
}

// ValidateURL checks one hop against the reviewed scheme/host/port and the
// target-resource rule. A redirect to any other destination fails here, so
// "public but unlisted" is just as rejected as "internal".
func (p HTTPPolicy) ValidateURL(u *url.URL) error {
	if u == nil || u.Scheme != p.Scheme || u.Hostname() != p.Host || u.Port() != p.Port {
		return fmt.Errorf("%w: %s", ErrUnlistedDestination, u)
	}
	path := u.Path
	if path == "" {
		path = "/"
	}
	prefix := p.PathPrefix
	if prefix == "" {
		prefix = "/"
	}
	if !strings.HasPrefix(path, prefix) {
		return fmt.Errorf("%w: path %q outside target resource %q", ErrUnlistedDestination, path, prefix)
	}
	return nil
}

// ValidateRequest checks the method as well as the destination.
func (p HTTPPolicy) ValidateRequest(method string, u *url.URL) error {
	m := strings.ToUpper(method)
	for _, allowed := range p.Methods {
		if strings.ToUpper(allowed) == m {
			return p.ValidateURL(u)
		}
	}
	return fmt.Errorf("%w: method %q", ErrUnlistedDestination, m)
}

// resolve returns every address of host and requires that ALL of them be
// allowed. A DNS answer mixing public and private records (classic DNS
// rebinding) is rejected because ANY disallowed address fails the policy.
func (p HTTPPolicy) resolve(ctx context.Context, host string) ([]net.IPAddr, error) {
	resolver := p.Resolver
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	addrs, err := resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	if len(addrs) == 0 {
		return nil, fmt.Errorf("%w: %s resolved to no address", ErrAddressNotAllowed, host)
	}
	for _, a := range addrs {
		if !p.AddressAllowed(a.IP) {
			return nil, fmt.Errorf("%w: %s resolves to non-allowed %s", ErrAddressNotAllowed, host, a.IP)
		}
	}
	return addrs, nil
}

// dial connects to a VALIDATED resolved IP, not to the hostname: DNS is
// resolved and every address is checked by resolve before any connection is
// opened, so a rebinding answer can never reach the socket. The transport
// still derives TLS ServerName from the request URL's hostname (Go uses the
// URL host, not the dialed address), so certificate validation is preserved.
func (p HTTPPolicy) dial(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	var addrs []net.IPAddr
	if ip := net.ParseIP(host); ip != nil {
		addrs = []net.IPAddr{{IP: ip}}
	} else {
		addrs, err = p.resolve(ctx, host)
		if err != nil {
			return nil, err
		}
	}
	for _, a := range addrs {
		if !p.AddressAllowed(a.IP) {
			return nil, fmt.Errorf("%w: %s", ErrAddressNotAllowed, a.IP)
		}
	}
	d := net.Dialer{}
	return d.DialContext(ctx, network, net.JoinHostPort(addrs[0].IP.String(), port))
}

// checkRedirect re-validates EVERY redirect hop and never lets
// Authorization/Cookie (or proxy auth) headers cross a host. The strip
// happens before validation so even a policy-violating hop observably
// carries no credentials.
func (p HTTPPolicy) checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) > 0 && req.URL.Host != via[0].URL.Host {
		req.Header.Del("Authorization")
		req.Header.Del("Cookie")
		req.Header.Del("Proxy-Authorization")
	}
	if err := p.ValidateURL(req.URL); err != nil {
		return err
	}
	if len(via) >= 10 {
		return ErrTooManyRedirects
	}
	return nil
}

type validatingTransport struct {
	rt  http.RoundTripper
	pol HTTPPolicy
}

// RoundTrip validates method and destination of every request (including
// redirect hops, which Go routes back through the transport) before any
// dial happens.
func (t *validatingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := t.pol.ValidateRequest(req.Method, req.URL); err != nil {
		return nil, err
	}
	return t.rt.RoundTrip(req)
}

// NewClient builds an http.Client bound to this policy: every request and
// every redirect hop is re-validated, DNS answers are fully checked, and
// dials go to validated IPs while TLS ServerName stays the reviewed host.
func (p HTTPPolicy) NewClient() *http.Client {
	inner := &http.Transport{
		DialContext:         p.dial,
		MaxIdleConns:        4,
		IdleConnTimeout:     30 * time.Second,
		DisableKeepAlives:   false,
		TLSHandshakeTimeout: 10 * time.Second,
	}
	return &http.Client{
		Transport:     &validatingTransport{rt: inner, pol: p},
		CheckRedirect: p.checkRedirect,
		Timeout:       p.Timeout,
	}
}

// PolicyDigest fingerprints the reviewed outbound contract. A stored policy
// digest lets installers detect drift between what an admin reviewed and
// what a later dispatch would use.
func (p HTTPPolicy) PolicyDigest() string {
	h := sha256.New()
	h.Write([]byte(p.Scheme))
	h.Write([]byte{0})
	h.Write([]byte(p.Host))
	h.Write([]byte{0})
	h.Write([]byte(p.Port))
	h.Write([]byte{0})
	h.Write([]byte(strings.ToUpper(strings.Join(p.Methods, ","))))
	h.Write([]byte{0})
	h.Write([]byte(p.PathPrefix))
	return hex.EncodeToString(h.Sum(nil))
}
