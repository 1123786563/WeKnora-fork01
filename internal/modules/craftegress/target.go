package craftegress

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// ValidateGatewayTarget enforces the production host policy for the
// operator-configured gateway endpoint: http/https only, and the host must
// not be localhost, loopback, private, link-local, unspecified or multicast
// unless the deployment explicitly opts in (local development, same-host
// smoke runs). The adapter never derives its target from client input.
func ValidateGatewayTarget(rawURL string, allowPrivate bool) (*url.URL, error) {
	return validateGatewayTarget(rawURL, allowPrivate, net.LookupIP)
}

func validateGatewayTarget(rawURL string, allowPrivate bool, lookup func(string) ([]net.IP, error)) (*url.URL, error) {
	target, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("craftegress: gateway URL: %w", err)
	}
	if target.Scheme != "http" && target.Scheme != "https" {
		return nil, fmt.Errorf("craftegress: gateway URL must be http or https, got %q", target.Scheme)
	}
	host := target.Hostname()
	if host == "" {
		return nil, fmt.Errorf("craftegress: gateway URL requires a host")
	}
	if allowPrivate {
		return target, nil
	}
	if strings.EqualFold(host, "localhost") || strings.HasSuffix(strings.ToLower(host), ".localhost") ||
		strings.HasSuffix(strings.ToLower(host), ".local") || strings.HasSuffix(strings.ToLower(host), ".internal") {
		return nil, fmt.Errorf("craftegress: gateway host %q is a local name; set the explicit private-target opt-in for local deployments", host)
	}
	if ip := net.ParseIP(host); ip != nil {
		if err := rejectPrivateIP(ip); err != nil {
			return nil, err
		}
		return target, nil
	}
	resolved, err := lookup(host)
	if err != nil || len(resolved) == 0 {
		return nil, fmt.Errorf("craftegress: gateway host %q does not resolve: %w", host, err)
	}
	for _, ip := range resolved {
		if err := rejectPrivateIP(ip); err != nil {
			return nil, err
		}
	}
	return target, nil
}

// craftCGNATBlock is the shared-address range 100.64.0.0/10: not in
// net.IP.IsPrivate, but a common cloud-internal/internal-LB range that a
// Bearer-bearing forward must never dial unintentionally.
var craftCGNATBlock = net.IPNet{IP: net.IP{100, 64, 0, 0}, Mask: net.CIDRMask(10, 32)}

func rejectPrivateIP(ip net.IP) error {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() ||
		!ip.IsGlobalUnicast() || craftCGNATBlock.Contains(ip) {
		// !IsGlobalUnicast additionally refuses the limited broadcast
		// 255.255.255.255 and every other non-routable form.
		return fmt.Errorf("craftegress: gateway host %s is loopback/private/reserved; set the explicit private-target opt-in for local deployments", ip)
	}
	return nil
}
