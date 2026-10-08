package appconnector

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// Confluence write rejections. Every denial names which rule was violated;
// none of them ever carries credential material.
var (
	// ErrConfluenceVersionConflict: the external page's current
	// version.number no longer equals the version the approved update plan
	// was formed against. The update is refused BEFORE any write request —
	// per CONTEXT.md「外部发布」: 再次更新前必须读取外部当前版本并形成新的
	// 候选变更.
	ErrConfluenceVersionConflict = errors.New("confluence_version_conflict")
	// ErrConfluenceSnapshotInvalid: the action arguments are not exactly the
	// approved snapshot shape — an extra field (approve-then-rewrite), a
	// missing field, or an empty required field is refused rather than
	// silently forwarded.
	ErrConfluenceSnapshotInvalid = errors.New("confluence_snapshot_invalid")
	// ErrConfluenceParentOutOfScope: the snapshot's parent page is not
	// inside the approval scope for this connection. The check fails
	// closed: an empty scope list rejects every parent.
	ErrConfluenceParentOutOfScope = errors.New("confluence_parent_out_of_scope")
	// ErrConfluenceMissingCapability: the connection does not carry the
	// reviewed write capability.
	ErrConfluenceMissingCapability = errors.New("confluence_missing_capability")
	// ErrConfluenceOutcomeUnknown: the request may or may not have produced
	// its remote effect (response lost, 5xx, unparseable reply, or a
	// provider reply without a real page id/version). The outcome stays
	// unknown and resolves ONLY via Query's reliable page read.
	ErrConfluenceOutcomeUnknown = errors.New("confluence_outcome_unknown")
	// ErrConfluenceNotConfigured: the adapter is missing a reviewed outbound
	// policy or a credential source — fail closed, never dial by invention.
	ErrConfluenceNotConfigured = errors.New("confluence_adapter_not_configured")
	// ErrConfluenceApprovalRevoked: the A03 approval was revoked between
	// approval and execute; the action stays parked awaiting approval.
	ErrConfluenceApprovalRevoked = errors.New("confluence_approval_revoked")
	// ErrConfluenceCredentialInvalid: the resolved credential bytes are not
	// the reviewed JSON shape.
	ErrConfluenceCredentialInvalid = errors.New("confluence_credential_invalid")
)

// Confluence capability + editions. ConfluenceCapabilityWrite is the
// reviewed schema_json scope this adapter family requires
// ({"scopes":["write_content"],...}); EditionCloud selects the
// {base}/api/v2 wire, EditionServer the {base}/rest/api wire.
const (
	ConfluenceCapabilityWrite = "write_content"
	EditionCloud              = "cloud"
	EditionServer             = "server"
)

// ConfluenceCredential is the decrypted connection credential: Confluence
// authenticates with HTTP Basic over (username, secret) on BOTH editions
// (Cloud: email + API token; Server/DC: username + password). The
// connection credential store holds it as JSON
// {"username": "...", "secret": "..."} — never as a header string.
type ConfluenceCredential struct {
	Username string
	Secret   string
}

// ParseConfluenceCredential validates the resolved credential bytes.
func ParseConfluenceCredential(raw []byte) (ConfluenceCredential, error) {
	var c struct {
		Username string `json:"username"`
		Secret   string `json:"secret"`
	}
	if err := json.Unmarshal(raw, &c); err != nil {
		return ConfluenceCredential{}, fmt.Errorf("%w: %v", ErrConfluenceCredentialInvalid, err)
	}
	if strings.TrimSpace(c.Username) == "" || c.Secret == "" {
		return ConfluenceCredential{}, fmt.Errorf("%w: username and secret are required", ErrConfluenceCredentialInvalid)
	}
	return ConfluenceCredential{Username: c.Username, Secret: c.Secret}, nil
}

// ConfluenceEndpoint is the reviewed outbound anchor derived from the
// installation's config_json.base_url: exactly one scheme/host/port plus
// the API base path (context path) and the resolved edition.
type ConfluenceEndpoint struct {
	Scheme string
	Host   string
	Port   string
	// APIBasePath is "" or a "/context" path (e.g. "/wiki").
	APIBasePath string
	Edition     string
}

// ParseConfluenceBaseURL derives the endpoint from the reviewed base URL.
// Edition defaults from the host (*.atlassian.net → cloud) and may be
// overridden explicitly (an enterprise test double on a loopback host is
// cloud-shaped but not an atlassian host). Only http/https is accepted —
// dial-time public-address enforcement stays with HTTPPolicy.
func ParseConfluenceBaseURL(raw, explicitEdition string) (ConfluenceEndpoint, error) {
	raw = strings.TrimSpace(strings.TrimRight(strings.TrimSpace(raw), "/"))
	if raw == "" || !strings.Contains(raw, "://") {
		return ConfluenceEndpoint{}, fmt.Errorf("confluence_base_url_invalid: %q", raw)
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return ConfluenceEndpoint{}, fmt.Errorf("confluence_base_url_invalid: %q", raw)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return ConfluenceEndpoint{}, fmt.Errorf("confluence_base_url_invalid: scheme %q", parsed.Scheme)
	}
	host := parsed.Hostname()
	ep := ConfluenceEndpoint{Scheme: parsed.Scheme, Host: host, Port: parsed.Port(), Edition: EditionServer}
	if host == "atlassian.net" || strings.HasSuffix(host, ".atlassian.net") {
		ep.Edition = EditionCloud
	}
	if explicitEdition != "" {
		if explicitEdition != EditionCloud && explicitEdition != EditionServer {
			return ConfluenceEndpoint{}, fmt.Errorf("confluence_base_url_invalid: edition %q", explicitEdition)
		}
		ep.Edition = explicitEdition
	}
	if path := strings.TrimRight(parsed.Path, "/"); path != "" {
		ep.APIBasePath = path
	} else if ep.Edition == EditionCloud {
		ep.APIBasePath = "/wiki"
	}
	return ep, nil
}

// ConfluencePageVersion is the reliable read shape of one page across both
// editions. VersionNumber is the external collaboration authority's version
// token — Confluence's monotonic integer version.number, carried as its
// decimal string. BodyStorage is present when the read asked for the
// storage body.
type ConfluencePageVersion struct {
	PageID        string
	Title         string
	SpaceID       string
	SpaceKey      string
	VersionNumber string
	BodyStorage   string
}

type confluencePageWire struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	SpaceID string `json:"spaceId"`
	Space   struct {
		Key string `json:"key"`
		ID  string `json:"id"`
	} `json:"space"`
	Version struct {
		Number    int    `json:"number"`
		When      string `json:"when"`
		CreatedAt string `json:"createdAt"`
	} `json:"version"`
	Body struct {
		Storage struct {
			Value string `json:"value"`
		} `json:"storage"`
	} `json:"body"`
}

// ParseConfluencePageVersion accepts the Cloud v2 page object (spaceId) and
// the Server/DC content object (space.key) in one shape. A reply without a
// real id or a real version.number (>= 1) is an error — a fabricated
// version is never a basis for conflict detection or a receipt.
func ParseConfluencePageVersion(raw []byte) (ConfluencePageVersion, error) {
	var w confluencePageWire
	if err := json.Unmarshal(raw, &w); err != nil {
		return ConfluencePageVersion{}, fmt.Errorf("confluence_page_version_unparseable: %v", err)
	}
	if w.ID == "" || w.Version.Number < 1 {
		return ConfluencePageVersion{}, fmt.Errorf("confluence_page_version_unparseable: no id or version.number")
	}
	return ConfluencePageVersion{
		PageID: w.ID, Title: w.Title, SpaceID: w.SpaceID, SpaceKey: w.Space.Key,
		VersionNumber: strconv.Itoa(w.Version.Number), BodyStorage: w.Body.Storage.Value,
	}, nil
}

// ConfluencePageReceipt is the persisted external receipt of one publish:
// the provider page id and the version the publish itself produced (read
// from the provider's own reply — never fabricated locally).
type ConfluencePageReceipt struct {
	ExternalID      string
	ExternalVersion string
}

// ParseConfluencePageReceipt extracts the receipt fields from a provider
// page payload (the create/update reply the adapter recorded as the
// action's output evidence).
func ParseConfluencePageReceipt(raw []byte) (ConfluencePageReceipt, error) {
	v, err := ParseConfluencePageVersion(raw)
	if err != nil {
		return ConfluencePageReceipt{}, err
	}
	return ConfluencePageReceipt{ExternalID: v.PageID, ExternalVersion: v.VersionNumber}, nil
}

// DetectConfluenceVersionConflict compares the approved expected version
// with the version just read from the provider. Anything but an exact
// match — including an unreadable empty side — is a conflict; an
// unobservable remote state must never authorize an overwrite.
func DetectConfluenceVersionConflict(expected, actual string) error {
	if expected == "" || actual == "" || expected != actual {
		return fmt.Errorf("%w: approved %q but remote has %q", ErrConfluenceVersionConflict, expected, actual)
	}
	return nil
}
