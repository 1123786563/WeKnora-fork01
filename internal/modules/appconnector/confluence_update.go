package appconnector

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// ConfluenceUpdateSnapshot is the A03-approved argument snapshot for
// updating ONE existing external page: exactly page_id, expected_version
// (the version.number the plan read before approval — an immutable
// approval anchor), title and storage. Every dispatched request is built
// FROM these fields; nothing is added, rewritten or re-derived after
// approval.
type ConfluenceUpdateSnapshot struct {
	PageID          string
	ExpectedVersion string
	Title           string
	Storage         string
}

// ParseConfluenceUpdateSnapshot validates that args are EXACTLY the
// approved four-field update snapshot.
func ParseConfluenceUpdateSnapshot(args json.RawMessage) (ConfluenceUpdateSnapshot, error) {
	var s ConfluenceUpdateSnapshot
	dec := json.NewDecoder(bytes.NewReader(args))
	dec.UseNumber()
	var raw map[string]json.RawMessage
	if err := dec.Decode(&raw); err != nil {
		return s, fmt.Errorf("%w: %v", ErrConfluenceSnapshotInvalid, err)
	}
	if len(raw) != 4 {
		return s, fmt.Errorf("%w: snapshot must be exactly page_id, expected_version, title, storage", ErrConfluenceSnapshotInvalid)
	}
	for _, f := range []struct {
		name string
		dst  *string
	}{{"page_id", &s.PageID}, {"expected_version", &s.ExpectedVersion}, {"title", &s.Title}, {"storage", &s.Storage}} {
		if err := json.Unmarshal(raw[f.name], f.dst); err != nil {
			return s, fmt.Errorf("%w: %s: %v", ErrConfluenceSnapshotInvalid, f.name, err)
		}
	}
	if s.PageID == "" || s.ExpectedVersion == "" || s.Title == "" || s.Storage == "" {
		return s, fmt.Errorf("%w: page_id, expected_version, title and storage must be non-empty", ErrConfluenceSnapshotInvalid)
	}
	return s, nil
}

// IsConfluenceUpdateArgs reports whether args carry the update snapshot's
// distinguishing keys: page_id AND expected_version AND storage. The
// storage key is what separates a Confluence update from a Notion update
// (page_id + expected_version) if bytes ever cross bridges; full
// validation still happens in ParseConfluenceUpdateSnapshot.
func IsConfluenceUpdateArgs(args json.RawMessage) bool {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(args, &raw); err != nil {
		return false
	}
	_, hasPage := raw["page_id"]
	_, hasVersion := raw["expected_version"]
	_, hasStorage := raw["storage"]
	return hasPage && hasVersion && hasStorage
}

type confluenceVersionRef struct {
	Number  int    `json:"number"`
	Message string `json:"message,omitempty"`
}

type confluenceCloudUpdateRequest struct {
	ID      string                `json:"id"`
	Status  string                `json:"status"`
	Title   string                `json:"title"`
	Body    confluenceStorageBody `json:"body"`
	Version confluenceVersionRef  `json:"version"`
}

type confluenceServerUpdateRequest struct {
	Type  string `json:"type"`
	ID    string `json:"id"`
	Title string `json:"title"`
	// Body carries the Server/DC content in the documented nested
	// body.storage.{value,representation} shape (confluenceServerBodyRef) —
	// the same wire the create adapter and the fake double encode.
	Body    confluenceServerBodyRef `json:"body"`
	Version confluenceVersionRef    `json:"version"`
}

// confluenceNextVersion parses the approved expected version and returns
// the NEXT number a real instance demands on update (Confluence rejects a
// PUT whose version.number is not current+1).
func confluenceNextVersion(expected string) (int, error) {
	n, err := strconv.Atoi(expected)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("%w: expected_version %q is not a real page version", ErrConfluenceSnapshotInvalid, expected)
	}
	return n + 1, nil
}

// ConfluenceUpdateAdapter executes ONE approved page update against the
// reviewed contract family: the update is a TWO-request sequence — read
// the current version (the conflict gate), then PUT the storage body with
// version.number+1. The version pre-read is the publish conflict gate:
// the remote version.number must still equal the snapshot's approved
// expected_version, otherwise the update is refused with
// ErrConfluenceVersionConflict and ZERO write requests leave the process.
//
// Outcome semantics (identical to the create adapter):
//   - pre-read failures are definitive FAILED (a GET cannot have produced
//     the write; nothing left the process);
//   - the PUT's transport failure / 5xx / unparseable reply is
//     ErrConfluenceOutcomeUnknown — the effect may exist remotely;
//   - Query reconciles ONLY via the reliable page read: version moved by
//     exactly one AND title AND storage all still ours → success.
type ConfluenceUpdateAdapter struct {
	Policy                 HTTPPolicy
	Credential             func(ctx context.Context) (ConfluenceCredential, error)
	Edition                string
	APIBasePath            string
	ConnectionCapabilities func(ctx context.Context, a Action) ([]string, error)
	Recheck                func(ctx context.Context, a Action) error
}

var _ Adapter = (*ConfluenceUpdateAdapter)(nil)

func (m *ConfluenceUpdateAdapter) configError() error {
	if m.Policy.Host == "" || m.Policy.Scheme == "" {
		return fmt.Errorf("%w: no reviewed outbound policy", ErrConfluenceNotConfigured)
	}
	if m.Credential == nil {
		return fmt.Errorf("%w: no credential source", ErrConfluenceNotConfigured)
	}
	return nil
}

func (m *ConfluenceUpdateAdapter) editionName() (string, error) {
	switch m.Edition {
	case "", EditionCloud:
		return EditionCloud, nil
	case EditionServer:
		return EditionServer, nil
	default:
		return "", fmt.Errorf("%w: edition %q", ErrConfluenceNotConfigured, m.Edition)
	}
}

func (m *ConfluenceUpdateAdapter) requireWriteCapability(ctx context.Context, a Action) error {
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

// putPage performs the single update step with version.number+1.
func (m *ConfluenceUpdateAdapter) putPage(ctx context.Context, edition string, snap ConfluenceUpdateSnapshot) ([]byte, error) {
	next, nerr := confluenceNextVersion(snap.ExpectedVersion)
	if nerr != nil {
		return nil, nerr
	}
	var body []byte
	var u *url.URL
	var err error
	if edition == EditionServer {
		body, err = json.Marshal(confluenceServerUpdateRequest{
			Type: "page", ID: snap.PageID, Title: snap.Title,
			Body:    confluenceServerBodyRef{Storage: confluenceServerStorageBody{Value: snap.Storage, Representation: "storage"}},
			Version: confluenceVersionRef{Number: next},
		})
		u = confluenceTargetURL(m.Policy, m.APIBasePath, ConfluenceServerContentPath+"/"+url.PathEscape(snap.PageID), "")
	} else {
		body, err = json.Marshal(confluenceCloudUpdateRequest{
			ID: snap.PageID, Status: "current", Title: snap.Title,
			Body:    confluenceStorageBody{Representation: "storage", Value: snap.Storage},
			Version: confluenceVersionRef{Number: next},
		})
		u = confluenceTargetURL(m.Policy, m.APIBasePath, ConfluenceCloudPagesPath+"/"+url.PathEscape(snap.PageID), "")
	}
	if err != nil {
		return nil, err
	}
	status, raw, derr := confluenceDo(ctx, m.Policy, m.Credential, http.MethodPut, u, body)
	if derr != nil {
		return nil, derr
	}
	if status >= 400 {
		if status >= 500 {
			return nil, fmt.Errorf("%w: status=%d", ErrConfluenceOutcomeUnknown, status)
		}
		return nil, fmt.Errorf("confluence_provider_error: status=%d", status)
	}
	return raw, nil
}

// Execute performs the approved update. Order: A03 recheck, snapshot
// validation, capability check, version PRE-READ + conflict detection —
// all BEFORE any network write — then the single PUT whose reply payload
// is the action's output evidence (the receipt basis).
func (m *ConfluenceUpdateAdapter) Execute(ctx context.Context, a Action) (ActionResult, error) {
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
	snap, err := ParseConfluenceUpdateSnapshot(a.Args)
	if err != nil {
		return ActionResult{State: ActionFailed}, err
	}
	if err := m.requireWriteCapability(ctx, a); err != nil {
		return ActionResult{State: ActionFailed}, err
	}
	// AC1: read the external current version FIRST. A read can never have
	// produced the write, so an unreadable pre-read is a definitive
	// failure — zero write requests leave the process.
	cur, rerr := confluenceReadPage(ctx, m.Policy, m.Credential, edition, m.APIBasePath, snap.PageID)
	if rerr != nil {
		return ActionResult{State: ActionFailed}, rerr
	}
	if cerr := DetectConfluenceVersionConflict(snap.ExpectedVersion, cur.VersionNumber); cerr != nil {
		return ActionResult{State: ActionFailed}, cerr
	}
	raw, uerr := m.putPage(ctx, edition, snap)
	if uerr != nil {
		state := ActionFailed
		if errors.Is(uerr, ErrConfluenceOutcomeUnknown) {
			state = ActionUnknown
		}
		return ActionResult{State: state, ExternalID: snap.PageID}, uerr
	}
	rcpt, rerr := ParseConfluencePageReceipt(raw)
	if rerr != nil {
		// The PUT may have applied while the reply proves nothing: the
		// honest state is unknown.
		return ActionResult{State: ActionUnknown, ExternalID: snap.PageID}, rerr
	}
	return ActionResult{State: ActionSucceeded, ExternalID: rcpt.ExternalID, Output: json.RawMessage(raw)}, nil
}

// normalizeStorageXHTML collapses the KNOWN re-serialization noise between
// the locally-escaped storage a plan writes (ConfluenceStorageBody emits
// &#39;/&#34; entities, publish/confluence_blocks.go) and the server's own
// re-serialized read-back (bare characters, LF-only whitespace, trimmed
// edges). It is applied to BOTH sides before comparison, so it can only
// converge equivalent bodies — genuinely different content (an extra
// paragraph) still differs after normalization and stays unverifiable.
func normalizeStorageXHTML(s string) string {
	s = html.UnescapeString(s)
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.TrimSpace(s)
}

// Query is the reconciliation entry point for an update parked in
// unknown: it reconciles ONLY via the reliable page read. Success is
// claimed only when the version advanced by EXACTLY one from the approved
// baseline AND the title AND the storage body all still match the
// approved snapshot (storage compared in normalized form — the server
// re-serializes XHTML); anything less stays the honest unknown.
func (m *ConfluenceUpdateAdapter) Query(ctx context.Context, a Action) (ActionResult, error) {
	if err := m.configError(); err != nil {
		return ActionResult{State: ActionFailed}, err
	}
	snap, err := ParseConfluenceUpdateSnapshot(a.Args)
	if err != nil {
		return ActionResult{State: ActionFailed}, err
	}
	edition, ederr := m.editionName()
	if ederr != nil {
		return ActionResult{State: ActionUnknown}, ederr
	}
	next, nerr := confluenceNextVersion(snap.ExpectedVersion)
	if nerr != nil {
		return ActionResult{State: ActionUnknown}, nerr
	}
	cur, rerr := confluenceReadPage(ctx, m.Policy, m.Credential, edition, m.APIBasePath, snap.PageID)
	if rerr != nil {
		return ActionResult{State: ActionUnknown}, rerr
	}
	if cur.VersionNumber != strconv.Itoa(next) {
		return ActionResult{State: ActionUnknown}, fmt.Errorf("confluence_query_unverifiable: remote version %q, expected ours at %q", cur.VersionNumber, strconv.Itoa(next))
	}
	if strings.TrimSpace(cur.Title) != strings.TrimSpace(snap.Title) ||
		normalizeStorageXHTML(cur.BodyStorage) != normalizeStorageXHTML(snap.Storage) {
		return ActionResult{State: ActionUnknown}, fmt.Errorf("confluence_query_unverifiable: remote content drifted from the approved snapshot")
	}
	// The success output is a faithful local projection
	// {"id","version":{"number"}} assembled from the remote page read this
	// Query just validated (version == expected+1 above; title/storage
	// equal to the approved snapshot) — not the raw reply of a PUT this
	// query never issued. The settle path consumes exactly these two
	// fields through ParseConfluencePageReceipt (publish/confluence.go),
	// so the receipt wording in confluence_common.go ("read from the
	// provider's own reply") holds field-for-field; only the carrier
	// differs: a validated read replaces the lost write reply.
	raw, _ := json.Marshal(map[string]any{"id": cur.PageID, "version": map[string]int{"number": next}})
	return ActionResult{State: ActionSucceeded, ExternalID: cur.PageID, Output: json.RawMessage(raw)}, nil
}

// ReadConfluencePageVersion performs a one-off version read through the
// given reviewed policy and credential source — the plan-formation
// pre-read shared by the publish seam. It performs no write of any kind.
func ReadConfluencePageVersion(ctx context.Context, pol HTTPPolicy, cred func(context.Context) (ConfluenceCredential, error), edition, apiBasePath, pageID string) (string, error) {
	ver, err := confluenceReadPage(ctx, pol, cred, edition, apiBasePath, pageID)
	if err != nil {
		return "", err
	}
	return ver.VersionNumber, nil
}
