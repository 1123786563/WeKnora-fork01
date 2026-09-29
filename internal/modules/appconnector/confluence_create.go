package appconnector

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
)

// NO-CF fixed contract, validated against the official Confluence REST
// documents (developer.atlassian.com): Cloud v2 pages under
// {base}/api/v2, Server/DC content under {base}/rest/api. These are the
// reviewed paths the adapter family is allowed to use; nothing here is
// derived from model output.
const (
	ConfluenceCloudPagesPath    = "/api/v2/pages"
	ConfluenceServerContentPath = "/rest/api/content"
)

// ConfluenceCreateSnapshot is the A03-approved argument snapshot for ONE
// page creation: exactly parent (the reviewed parent page id), title and
// storage (the deterministic Confluence storage-format body derived from
// the artifact by the publish seam). Every dispatched request is built
// FROM these fields; nothing is added, rewritten or re-derived after
// approval.
type ConfluenceCreateSnapshot struct {
	Parent  string
	Title   string
	Storage string
}

// ParseConfluenceCreateSnapshot validates that args are EXACTLY the
// approved three-field create snapshot.
func ParseConfluenceCreateSnapshot(args json.RawMessage) (ConfluenceCreateSnapshot, error) {
	var s ConfluenceCreateSnapshot
	dec := json.NewDecoder(bytes.NewReader(args))
	dec.UseNumber()
	var raw map[string]json.RawMessage
	if err := dec.Decode(&raw); err != nil {
		return s, fmt.Errorf("%w: %v", ErrConfluenceSnapshotInvalid, err)
	}
	if len(raw) != 3 {
		return s, fmt.Errorf("%w: snapshot must be exactly parent, title, storage", ErrConfluenceSnapshotInvalid)
	}
	for _, f := range []struct {
		name string
		dst  *string
	}{{"parent", &s.Parent}, {"title", &s.Title}, {"storage", &s.Storage}} {
		if err := json.Unmarshal(raw[f.name], f.dst); err != nil {
			return s, fmt.Errorf("%w: %s: %v", ErrConfluenceSnapshotInvalid, f.name, err)
		}
	}
	if s.Parent == "" || s.Title == "" || s.Storage == "" {
		return s, fmt.Errorf("%w: parent, title and storage must be non-empty", ErrConfluenceSnapshotInvalid)
	}
	return s, nil
}

// IsConfluenceCreateArgs reports whether args carry the create snapshot's
// distinguishing key pair (parent AND storage, with no page_id). Routing
// only — full validation happens in ParseConfluenceCreateSnapshot.
func IsConfluenceCreateArgs(args json.RawMessage) bool {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(args, &raw); err != nil {
		return false
	}
	_, hasParent := raw["parent"]
	_, hasStorage := raw["storage"]
	_, hasPage := raw["page_id"]
	return hasParent && hasStorage && !hasPage
}

// Wire bodies. Cloud v2 create carries space_id + optional parent_id; the
// Server/DC create carries the space key + ancestor list. Content travels
// as Confluence storage format (XHTML) in both.
type confluenceStorageBody struct {
	Representation string `json:"representation"`
	Value          string `json:"value"`
}

type confluenceCloudCreateRequest struct {
	SpaceID  string                `json:"spaceId"`
	ParentID string                `json:"parentId,omitempty"`
	Status   string                `json:"status"`
	Title    string                `json:"title"`
	Body     confluenceStorageBody `json:"body"`
}

type confluenceSpaceKeyRef struct {
	Key string `json:"key"`
}

type confluenceIDRef struct {
	ID string `json:"id"`
}

type confluenceServerStorageBody struct {
	Value          string `json:"value"`
	Representation string `json:"representation"`
}

// confluenceServerBodyRef carries the Server/DC create body in the
// documented nested shape body.storage.{value,representation} (the wire
// contract the fake double and developer.atlassian.com both encode).
type confluenceServerBodyRef struct {
	Storage confluenceServerStorageBody `json:"storage"`
}

type confluenceServerCreateRequest struct {
	Type      string                  `json:"type"`
	Space     confluenceSpaceKeyRef   `json:"space"`
	Ancestors []confluenceIDRef       `json:"ancestors"`
	Title     string                  `json:"title"`
	Body      confluenceServerBodyRef `json:"body"`
}

// confluenceTargetURL builds one request URL under the reviewed policy's
// origin and the configured context path.
func confluenceTargetURL(pol HTTPPolicy, apiBasePath, path, query string) *url.URL {
	host := pol.Host
	if pol.Port != "" {
		host = net.JoinHostPort(host, pol.Port)
	}
	return &url.URL{Scheme: pol.Scheme, Host: host, Path: apiBasePath + path, RawQuery: query}
}

// maxConfluenceBodyBytes caps ONE response body. It is a runaway guard,
// NOT a truncation point: a body over the cap is an ERROR (a truncated
// write reply proves nothing; a truncated read breaks reconciliation
// forever). The cap must cover the largest legal payload: publish artifact
// bodies are bounded by MaxPublishArtifactBytes (1MiB, publish/plan.go) but
// escape to ~5x their raw size through ConfluenceStorageBody
// (html.EscapeString emits &#39;/&#34; entities), so 8MiB covers the worst
// legal storage echo with headroom.
const maxConfluenceBodyBytes = 8 << 20

// confluenceDo performs ONE policy-validated request with HTTP Basic auth
// — the same request/redirect re-validation as the Notion family (A04).
// Transport failures wrap ErrConfluenceOutcomeUnknown: the effect may
// already exist remotely, so it is unknown, never failed.
func confluenceDo(ctx context.Context, pol HTTPPolicy, cred func(context.Context) (ConfluenceCredential, error), method string, u *url.URL, body []byte) (int, []byte, error) {
	if err := pol.ValidateRequest(method, u); err != nil {
		return 0, nil, err // policy denial: the request never leaves
	}
	c, err := cred(ctx)
	if err != nil {
		return 0, nil, err
	}
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), rd)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(c.Username+":"+c.Secret)))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := pol.NewClient().Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("%w: %v", ErrConfluenceOutcomeUnknown, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxConfluenceBodyBytes+1))
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("%w: %v", ErrConfluenceOutcomeUnknown, err)
	}
	if len(raw) > maxConfluenceBodyBytes {
		// Never hand back truncated bytes: the over-cap reply proves
		// nothing about the effect (and a truncated read would poison
		// every later reconciliation).
		return resp.StatusCode, nil, fmt.Errorf("%w: %s %s: response body exceeds cap %d bytes",
			ErrConfluenceOutcomeUnknown, method, u.Path, maxConfluenceBodyBytes)
	}
	return resp.StatusCode, raw, nil
}

// confluenceReadPage reads ONE page across both editions, always asking
// for the storage body (the update reconciliation compares it). Read
// failures are returned verbatim: a GET can never have produced a write,
// so callers may treat them as definitive.
func confluenceReadPage(ctx context.Context, pol HTTPPolicy, cred func(context.Context) (ConfluenceCredential, error), edition, apiBasePath, pageID string) (ConfluencePageVersion, error) {
	var u *url.URL
	if edition == EditionServer {
		u = confluenceTargetURL(pol, apiBasePath, ConfluenceServerContentPath+"/"+url.PathEscape(pageID), "expand=body.storage,version,space")
	} else {
		u = confluenceTargetURL(pol, apiBasePath, ConfluenceCloudPagesPath+"/"+url.PathEscape(pageID), "body-format=storage")
	}
	status, raw, err := confluenceDo(ctx, pol, cred, http.MethodGet, u, nil)
	if err != nil {
		return ConfluencePageVersion{}, err
	}
	if status != http.StatusOK {
		return ConfluencePageVersion{}, fmt.Errorf("confluence_provider_error: status=%d", status)
	}
	return ParseConfluencePageVersion(raw)
}

// ConfluenceCreateAdapter is the Confluence member of the A04 Adapter
// family. It executes ONE approved page creation against the reviewed
// contract as a SINGLE write: read the parent page (existence proof +
// destination space resolution), then POST the storage body. There is no
// multi-step progress to persist — a lost create reply parks unknown, and
// no confirmed created-page id exists anywhere to resume from.
type ConfluenceCreateAdapter struct {
	// Policy is the admin-reviewed outbound contract (A04).
	Policy HTTPPolicy
	// Credential returns the connection's Basic credential after the
	// permission guard has passed.
	Credential func(ctx context.Context) (ConfluenceCredential, error)
	// Edition selects the wire: "" / "cloud" → {base}/api/v2, "server" →
	// {base}/rest/api. Anything else fails closed pre-send.
	Edition string
	// APIBasePath is "" or a "/context" path (e.g. "/wiki").
	APIBasePath string
	// ApprovedParents is the approval / pre-authorization scope: the page
	// ids a creation may be parented under. Empty fails closed for every
	// parent.
	ApprovedParents []string
	// ConnectionCapabilities reports the connection's granted capabilities;
	// the reviewed write capability is required before any request. Nil
	// fails closed.
	ConnectionCapabilities func(ctx context.Context, a Action) ([]string, error)
	// Recheck re-validates the A03 approval right before any request; a
	// revocation between approval and execute blocks the creation.
	Recheck func(ctx context.Context, a Action) error
}

var _ Adapter = (*ConfluenceCreateAdapter)(nil)

func (m *ConfluenceCreateAdapter) configError() error {
	if m.Policy.Host == "" || m.Policy.Scheme == "" {
		return fmt.Errorf("%w: no reviewed outbound policy", ErrConfluenceNotConfigured)
	}
	if m.Credential == nil {
		return fmt.Errorf("%w: no credential source", ErrConfluenceNotConfigured)
	}
	return nil
}

func (m *ConfluenceCreateAdapter) editionName() (string, error) {
	switch m.Edition {
	case "", EditionCloud:
		return EditionCloud, nil
	case EditionServer:
		return EditionServer, nil
	default:
		return "", fmt.Errorf("%w: edition %q", ErrConfluenceNotConfigured, m.Edition)
	}
}

// parentApproved fails closed: only a parent explicitly listed in the
// approved scope may receive the new page.
func (m *ConfluenceCreateAdapter) parentApproved(parent string) bool {
	for _, p := range m.ApprovedParents {
		if p == parent {
			return true
		}
	}
	return false
}

func (m *ConfluenceCreateAdapter) requireWriteCapability(ctx context.Context, a Action) error {
	if m.ConnectionCapabilities == nil {
		return fmt.Errorf("%w: no capability source", ErrConfluenceMissingCapability)
	}
	caps, err := m.ConnectionCapabilities(ctx, a)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrConfluenceMissingCapability, err)
	}
	for _, c := range caps {
		if c == ConfluenceCapabilityWrite {
			return nil
		}
	}
	return fmt.Errorf("%w: connection lacks %s", ErrConfluenceMissingCapability, ConfluenceCapabilityWrite)
}

// createPage performs the single create step. The body is built FROM the
// approved snapshot field-by-field plus the parent's resolved space —
// nothing else. A reply without a provable receipt (real id + version)
// is an unknown, never a success.
func (m *ConfluenceCreateAdapter) createPage(ctx context.Context, edition string, snap ConfluenceCreateSnapshot, parent ConfluencePageVersion) ([]byte, error) {
	var body []byte
	var path string
	var err error
	if edition == EditionServer {
		body, err = json.Marshal(confluenceServerCreateRequest{
			Type:      "page",
			Space:     confluenceSpaceKeyRef{Key: parent.SpaceKey},
			Ancestors: []confluenceIDRef{{ID: parent.PageID}},
			Title:     snap.Title,
			Body:      confluenceServerBodyRef{Storage: confluenceServerStorageBody{Value: snap.Storage, Representation: "storage"}},
		})
		path = ConfluenceServerContentPath
	} else {
		body, err = json.Marshal(confluenceCloudCreateRequest{
			SpaceID:  parent.SpaceID,
			ParentID: parent.PageID,
			Status:   "current",
			Title:    snap.Title,
			Body:     confluenceStorageBody{Representation: "storage", Value: snap.Storage},
		})
		path = ConfluenceCloudPagesPath
	}
	if err != nil {
		return nil, err
	}
	u := confluenceTargetURL(m.Policy, m.APIBasePath, path, "")
	status, raw, derr := confluenceDo(ctx, m.Policy, m.Credential, http.MethodPost, u, body)
	if derr != nil {
		return nil, derr
	}
	if status >= 400 {
		if status >= 500 {
			// A 5xx leaves the create ambiguous: the page may exist.
			return nil, fmt.Errorf("%w: status=%d", ErrConfluenceOutcomeUnknown, status)
		}
		return nil, fmt.Errorf("confluence_provider_error: status=%d", status)
	}
	return raw, nil
}

// Execute performs the approved creation. Order: A03 recheck, snapshot
// validation, parent-scope check, capability check, parent read (existence
// + space resolution — a GET that can never write) — all BEFORE the single
// network write.
func (m *ConfluenceCreateAdapter) Execute(ctx context.Context, a Action) (ActionResult, error) {
	if err := m.configError(); err != nil {
		return ActionResult{State: ActionFailed}, err
	}
	edition, ederr := m.editionName()
	if ederr != nil {
		return ActionResult{State: ActionFailed}, ederr
	}
	if m.Recheck != nil {
		if err := m.Recheck(ctx, a); err != nil {
			return ActionResult{State: ActionAwaitingApproval}, fmt.Errorf("%w: %v", ErrConfluenceApprovalRevoked, err)
		}
	}
	snap, err := ParseConfluenceCreateSnapshot(a.Args)
	if err != nil {
		return ActionResult{State: ActionFailed}, err
	}
	if !m.parentApproved(snap.Parent) {
		return ActionResult{State: ActionFailed}, fmt.Errorf("%w: parent %q outside the approved page scope", ErrConfluenceParentOutOfScope, snap.Parent)
	}
	if err := m.requireWriteCapability(ctx, a); err != nil {
		return ActionResult{State: ActionFailed}, err
	}
	parent, rerr := confluenceReadPage(ctx, m.Policy, m.Credential, edition, m.APIBasePath, snap.Parent)
	if rerr != nil {
		// A read can never have produced the write: definitive failure.
		return ActionResult{State: ActionFailed}, rerr
	}
	raw, cerr := m.createPage(ctx, edition, snap, parent)
	if cerr != nil {
		state := ActionFailed
		if errors.Is(cerr, ErrConfluenceOutcomeUnknown) {
			state = ActionUnknown
		}
		return ActionResult{State: state}, cerr
	}
	rcpt, rerr := ParseConfluencePageReceipt(raw)
	if rerr != nil {
		// The POST may have applied while the reply proves nothing: the
		// honest state is unknown.
		return ActionResult{State: ActionUnknown}, rerr
	}
	return ActionResult{State: ActionSucceeded, ExternalID: rcpt.ExternalID, Output: json.RawMessage(raw)}, nil
}

// Query is the reconciliation entry point for a creation parked in
// unknown. A single-step create that lost its reply leaves NO confirmed
// created-page id anywhere in the durable record (Confluence create has
// no idempotency key), so no read can prove WHICH page — if any — this
// action produced. The honest outcome is unknown; resolution is human
// reconciliation (the Notion family's advisory title search is equally
// non-proving and is therefore not even attempted here).
func (m *ConfluenceCreateAdapter) Query(ctx context.Context, a Action) (ActionResult, error) {
	if err := m.configError(); err != nil {
		return ActionResult{State: ActionFailed}, err
	}
	if _, err := ParseConfluenceCreateSnapshot(a.Args); err != nil {
		return ActionResult{State: ActionFailed}, err
	}
	return ActionResult{State: ActionUnknown}, nil
}
