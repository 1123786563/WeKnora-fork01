package appconnector

import (
	"errors"
	"testing"
)

func TestParseConfluenceCredential(t *testing.T) {
	c, err := ParseConfluenceCredential([]byte(`{"username":"u@example.test","secret":"s3cret"}`))
	if err != nil || c.Username != "u@example.test" || c.Secret != "s3cret" {
		t.Fatalf("credential parse drift: %+v %v", c, err)
	}
	for name, raw := range map[string][]byte{
		"not json":       []byte(`nope`),
		"empty username": []byte(`{"username":"","secret":"s"}`),
		"blank username": []byte(`{"username":"  ","secret":"s"}`),
		"empty secret":   []byte(`{"username":"u","secret":""}`),
		"missing secret": []byte(`{"username":"u"}`),
	} {
		if _, err := ParseConfluenceCredential(raw); !errors.Is(err, ErrConfluenceCredentialInvalid) {
			t.Fatalf("%s: want ErrConfluenceCredentialInvalid, got %v", name, err)
		}
	}
}

func TestParseConfluenceBaseURL(t *testing.T) {
	// Atlassian Cloud: /wiki context path is filled in when absent.
	ep, err := ParseConfluenceBaseURL("https://acme.atlassian.net", "")
	if err != nil || ep.Edition != EditionCloud || ep.APIBasePath != "/wiki" ||
		ep.Scheme != "https" || ep.Host != "acme.atlassian.net" || ep.Port != "" {
		t.Fatalf("cloud endpoint drift: %+v %v", ep, err)
	}
	// An explicit path is kept as-is.
	ep, err = ParseConfluenceBaseURL("https://acme.atlassian.net/wiki/", "")
	if err != nil || ep.APIBasePath != "/wiki" {
		t.Fatalf("cloud explicit context path drift: %+v %v", ep, err)
	}
	// Server/DC at the root: empty base path, server edition.
	ep, err = ParseConfluenceBaseURL("https://confluence.corp.example", "")
	if err != nil || ep.Edition != EditionServer || ep.APIBasePath != "" || ep.Host != "confluence.corp.example" {
		t.Fatalf("server root drift: %+v %v", ep, err)
	}
	// Server/DC behind a context path keeps it.
	ep, err = ParseConfluenceBaseURL("https://corp.example/confluence", "")
	if err != nil || ep.Edition != EditionServer || ep.APIBasePath != "/confluence" {
		t.Fatalf("server context path drift: %+v %v", ep, err)
	}
	// An explicit edition overrides host-based derivation (loopback test
	// doubles are cloud-shaped but not *.atlassian.net).
	ep, err = ParseConfluenceBaseURL("http://127.0.0.1:8989/wiki", "cloud")
	if err != nil || ep.Edition != EditionCloud || ep.APIBasePath != "/wiki" || ep.Port != "8989" {
		t.Fatalf("explicit edition override drift: %+v %v", ep, err)
	}
	for name, raw := range map[string]string{
		"empty":       "   ",
		"no scheme":   "confluence.corp.example",
		"bad scheme":  "ftp://confluence.corp.example",
		"no host":     "https:///wiki",
		"parse error": "https://exa mple.test",
	} {
		if _, err := ParseConfluenceBaseURL(raw, ""); err == nil {
			t.Fatalf("%s: must be refused", name)
		}
	}
	// Unknown explicit edition fails closed.
	if _, err := ParseConfluenceBaseURL("https://confluence.corp.example", "hyper"); err == nil {
		t.Fatal("unknown explicit edition must be refused")
	}
}

func TestParseConfluencePageVersion(t *testing.T) {
	// Cloud v2 page shape.
	v, err := ParseConfluencePageVersion([]byte(`{"id":"9","status":"current","title":"P","spaceId":"42","version":{"number":3,"createdAt":"2026-09-25T08:00:00Z"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if v.PageID != "9" || v.Title != "P" || v.SpaceID != "42" || v.VersionNumber != "3" || v.SpaceKey != "" {
		t.Fatalf("cloud page version drift: %+v", v)
	}
	// Server/DC content shape.
	v, err = ParseConfluencePageVersion([]byte(`{"id":"9","type":"page","title":"P","space":{"key":"ENG","id":"42"},"version":{"number":7},"body":{"storage":{"value":"<p>x</p>"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if v.SpaceKey != "ENG" || v.VersionNumber != "7" || v.BodyStorage != "<p>x</p>" {
		t.Fatalf("server page version drift: %+v", v)
	}
	for name, raw := range map[string][]byte{
		"no id":         []byte(`{"version":{"number":2}}`),
		"no version":    []byte(`{"id":"9"}`),
		"zero version":  []byte(`{"id":"9","version":{"number":0}}`),
		"negative":      []byte(`{"id":"9","version":{"number":-1}}`),
		"not an object": []byte(`["page"]`),
	} {
		if _, err := ParseConfluencePageVersion(raw); err == nil {
			t.Fatalf("%s: a fabricated version must never parse", name)
		}
	}
}

func TestParseConfluencePageReceipt(t *testing.T) {
	r, err := ParseConfluencePageReceipt([]byte(`{"id":"p1","version":{"number":9}}`))
	if err != nil {
		t.Fatal(err)
	}
	if r.ExternalID != "p1" || r.ExternalVersion != "9" {
		t.Fatalf("receipt fields drift: %+v", r)
	}
	if _, err := ParseConfluencePageReceipt([]byte(`{"id":"","version":{"number":1}}`)); err == nil {
		t.Fatal("no real page id must refuse a receipt")
	}
}

func TestDetectConfluenceVersionConflict(t *testing.T) {
	if err := DetectConfluenceVersionConflict("3", "3"); err != nil {
		t.Fatalf("matching version must pass: %v", err)
	}
	if err := DetectConfluenceVersionConflict("3", "4"); !errors.Is(err, ErrConfluenceVersionConflict) {
		t.Fatalf("drift must conflict, got %v", err)
	}
	if err := DetectConfluenceVersionConflict("", "4"); !errors.Is(err, ErrConfluenceVersionConflict) {
		t.Fatalf("empty expected must fail closed, got %v", err)
	}
	if err := DetectConfluenceVersionConflict("3", ""); !errors.Is(err, ErrConfluenceVersionConflict) {
		t.Fatalf("unreadable remote must fail closed, got %v", err)
	}
}
