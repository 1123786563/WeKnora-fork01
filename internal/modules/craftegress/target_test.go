package craftegress

import (
	"errors"
	"net"
	"strings"
	"testing"
)

func TestValidateGatewayTargetEnforcesSchemeAndHostPolicy(t *testing.T) {
	publicLookup := func(string) ([]net.IP, error) { return []net.IP{net.ParseIP("203.0.113.7")}, nil }
	cases := []struct {
		name         string
		rawURL       string
		allowPrivate bool
		lookup       func(string) ([]net.IP, error)
		wantErr      string
	}{
		{name: "public https", rawURL: "https://craft-gateway.example.com/craft/model-gateway", lookup: publicLookup},
		{name: "ftp scheme refused", rawURL: "ftp://craft-gateway.example.com/x", wantErr: "http or https"},
		{name: "missing host", rawURL: "https:///path", wantErr: "requires a host"},
		{name: "localhost name refused", rawURL: "http://localhost:8080/x", wantErr: "local name"},
		{name: "internal suffix refused", rawURL: "http://gateway.internal/x", wantErr: "local name"},
		{name: "loopback ip refused", rawURL: "http://127.0.0.1:8080/x", wantErr: "loopback/private/reserved"},
		{name: "private range refused", rawURL: "http://10.1.2.3/x", wantErr: "loopback/private/reserved"},
		{name: "link local refused", rawURL: "http://169.254.1.1/x", wantErr: "loopback/private/reserved"},
		{name: "name resolving private refused", rawURL: "http://craft-gateway.example.com/x", lookup: func(string) ([]net.IP, error) { return []net.IP{net.ParseIP("10.0.0.9")}, nil }, wantErr: "loopback/private/reserved"},
		{name: "name resolving to nothing refused", rawURL: "http://craft-gateway.example.com/x", lookup: func(string) ([]net.IP, error) { return nil, errors.New("nxdomain") }, wantErr: "does not resolve"},
		{name: "loopback allowed with opt-in", rawURL: "http://127.0.0.1:8080/x", allowPrivate: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lookup := tc.lookup
			if lookup == nil {
				lookup = func(string) ([]net.IP, error) { return nil, errors.New("unexpected dns use") }
			}
			target, err := validateGatewayTarget(tc.rawURL, tc.allowPrivate, lookup)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("expected acceptance, got %v", err)
				}
				if target.Host == "" {
					t.Fatal("accepted target must carry a host")
				}
				return
			}
			if err == nil {
				t.Fatalf("expected rejection containing %q, got acceptance", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}
