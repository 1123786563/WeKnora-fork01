package appconnector

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func cfUpdateArgs(pageID, expectedVersion, title, storage string) json.RawMessage {
	raw, _ := json.Marshal(map[string]any{
		"page_id": pageID, "expected_version": expectedVersion,
		"title": title, "storage": storage,
	})
	return raw
}

func cfUpdateAction(args json.RawMessage) Action {
	return Action{ID: "act_cf_u1", TenantID: 7, ActorID: "u1", ConnectionID: "conn-cf",
		Version: "confluence/v1", Target: "page-9", Risk: RiskWrite, Args: args}
}

func TestParseConfluenceUpdateSnapshot(t *testing.T) {
	snap, err := ParseConfluenceUpdateSnapshot(cfUpdateArgs("page-9", "3", "T", "<p>x</p>"))
	if err != nil || snap.PageID != "page-9" || snap.ExpectedVersion != "3" || snap.Title != "T" || snap.Storage != "<p>x</p>" {
		t.Fatalf("snapshot fields drift: %+v %v", snap, err)
	}
	raw := append([]byte{}, cfUpdateArgs("p", "3", "T", "s")...)
	raw = raw[:len(raw)-1]
	raw = append(raw, []byte(`,"evil":"x"}`)...)
	if _, err := ParseConfluenceUpdateSnapshot(raw); !errors.Is(err, ErrConfluenceSnapshotInvalid) {
		t.Fatalf("approve-then-rewrite must be refused, got %v", err)
	}
	for name, raw := range map[string]json.RawMessage{
		"missing page":    []byte(`{"expected_version":"3","title":"T","storage":"s"}`),
		"missing version": []byte(`{"page_id":"p","title":"T","storage":"s"}`),
		"empty version":   cfUpdateArgs("p", "", "T", "s"),
		"empty page":      cfUpdateArgs("", "3", "T", "s"),
		"empty storage":   cfUpdateArgs("p", "3", "T", ""),
		"three fields":    []byte(`{"page_id":"p","title":"T","storage":"s"}`),
		"not an object":   []byte(`["page_id"]`),
	} {
		if _, err := ParseConfluenceUpdateSnapshot(raw); !errors.Is(err, ErrConfluenceSnapshotInvalid) {
			t.Fatalf("%s: want ErrConfluenceSnapshotInvalid, got %v", name, err)
		}
	}
}

func TestIsConfluenceUpdateArgs(t *testing.T) {
	if !IsConfluenceUpdateArgs(cfUpdateArgs("p", "3", "T", "s")) {
		t.Fatal("update-shaped args must be detected")
	}
	// The Notion update shape (page_id + expected_version but NO storage)
	// must never be treated as a Confluence update — the storage key is the
	// edition-bridging discriminator.
	notion, _ := json.Marshal(map[string]any{"page_id": "p", "expected_version": "v", "title": "T", "blocks": []any{}})
	if IsConfluenceUpdateArgs(notion) {
		t.Fatal("notion-shaped args must not be confluence updates")
	}
	if IsConfluenceUpdateArgs(cfCreateArgs("p", "T", "s")) {
		t.Fatal("create-shaped args must not be updates")
	}
	if IsConfluenceUpdateArgs([]byte(`garbage`)) {
		t.Fatal("garbage must not be update")
	}
}

func TestConfluenceUpdateCloudHappyPath(t *testing.T) {
	fake := newFakeConfluence("cf-user@example.test", "secret_cf_token")
	fake.addPage("page-9", "sp-1", "ENG", "old title", 3)
	srv := fake.server(t)
	ad := &ConfluenceUpdateAdapter{
		Policy: cfPolicyOf(srv, ""), Credential: cfCredential,
		Edition: EditionCloud, ConnectionCapabilities: cfCaps(ConfluenceCapabilityWrite),
	}

	out, err := ad.Execute(context.Background(), cfUpdateAction(cfUpdateArgs("page-9", "3", "new title", "<p>next</p>")))
	if err != nil || out.State != ActionSucceeded {
		t.Fatalf("update: state=%s err=%v", out.State, err)
	}
	if out.ExternalID != "page-9" {
		t.Fatalf("external id: %+v", out)
	}
	rcpt, rerr := ParseConfluencePageReceipt(out.Output)
	if rerr != nil || rcpt.ExternalID != "page-9" || rcpt.ExternalVersion != "4" {
		t.Fatalf("receipt must carry the version THIS publish produced: %+v %v", rcpt, rerr)
	}
	posts, puts, gets := fake.stats()
	if puts != 1 || gets < 1 || posts != 0 {
		t.Fatalf("wire counts: posts=%d puts=%d gets=%d (pre-read GET must run)", posts, puts, gets)
	}
	fake.mu.Lock()
	p := fake.pages["page-9"]
	fake.mu.Unlock()
	if p.title != "new title" || p.storage != "<p>next</p>" || p.version != 4 {
		t.Fatalf("remote page drift: %+v", p)
	}
}

func TestConfluenceUpdateServerHappyPath(t *testing.T) {
	fake := newFakeConfluence("cf-user@example.test", "secret_cf_token")
	fake.addPage("page-9", "42", "ENG", "old title", 5)
	srv := fake.server(t)
	ad := &ConfluenceUpdateAdapter{
		Policy: cfPolicyOf(srv, ""), Credential: cfCredential,
		Edition: EditionServer, ConnectionCapabilities: cfCaps(ConfluenceCapabilityWrite),
	}

	out, err := ad.Execute(context.Background(), cfUpdateAction(cfUpdateArgs("page-9", "5", "v2", "<p>srv</p>")))
	if err != nil || out.State != ActionSucceeded {
		t.Fatalf("server update: state=%s err=%v", out.State, err)
	}
	rcpt, rerr := ParseConfluencePageReceipt(out.Output)
	if rerr != nil || rcpt.ExternalVersion != "6" {
		t.Fatalf("server receipt drift: %+v %v", rcpt, rerr)
	}
	_, puts, _ := fake.stats()
	if puts != 1 {
		t.Fatalf("puts=%d", puts)
	}
	// The Server/DC PUT must carry the documented nested body.storage
	// shape (the same wire the create adapter and the real instance use):
	// the approved storage must actually LAND on the remote page.
	fake.mu.Lock()
	p := fake.pages["page-9"]
	fake.mu.Unlock()
	if p.title != "v2" || p.storage != "<p>srv</p>" || p.version != 6 {
		t.Fatalf("remote page drift: %+v", p)
	}
}

func TestConfluenceUpdateServerQueryResolvesAfterDroppedReply(t *testing.T) {
	fake := newFakeConfluence("cf-user@example.test", "secret_cf_token")
	fake.addPage("page-9", "42", "ENG", "old title", 5)
	srv := fake.server(t)
	ad := &ConfluenceUpdateAdapter{
		Policy: cfPolicyOf(srv, ""), Credential: cfCredential,
		Edition: EditionServer, ConnectionCapabilities: cfCaps(ConfluenceCapabilityWrite),
	}
	fake.mu.Lock()
	fake.dropNextWrite = true
	fake.mu.Unlock()
	act := cfUpdateAction(cfUpdateArgs("page-9", "5", "v2", "<p>srv</p>"))
	if out, _ := ad.Execute(context.Background(), act); out.State != ActionUnknown {
		t.Fatalf("setup: unknown expected, got %s", out.State)
	}
	// The Server/DC write-read loop must agree on the nested body.storage
	// shape: the reconciliation read sees exactly the approved storage the
	// PUT applied, so the unknown resolves WITHOUT any new write.
	_, putsBefore, _ := fake.stats()
	q, qerr := ad.Query(context.Background(), act)
	if qerr != nil || q.State != ActionSucceeded || q.ExternalID != "page-9" {
		t.Fatalf("server query must resolve from the remote state: %+v %v", q, qerr)
	}
	rcpt, rerr := ParseConfluencePageReceipt(q.Output)
	if rerr != nil || rcpt.ExternalVersion != "6" {
		t.Fatalf("server query receipt drift: %+v %v", rcpt, rerr)
	}
	_, putsAfter, _ := fake.stats()
	if putsAfter != putsBefore {
		t.Fatalf("query must not re-send: puts %d -> %d", putsBefore, putsAfter)
	}
}

func TestConfluenceUpdateConflictZeroWrites(t *testing.T) {
	fake := newFakeConfluence("cf-user@example.test", "secret_cf_token")
	fake.addPage("page-9", "sp-1", "ENG", "old title", 3)
	srv := fake.server(t)
	ad := &ConfluenceUpdateAdapter{
		Policy: cfPolicyOf(srv, ""), Credential: cfCredential,
		Edition: EditionCloud, ConnectionCapabilities: cfCaps(ConfluenceCapabilityWrite),
	}
	// External collaborator edited AFTER the plan was formed: version 3 → 4.
	fake.bump("page-9", "<p>external edit</p>")

	out, err := ad.Execute(context.Background(), cfUpdateAction(cfUpdateArgs("page-9", "3", "new title", "<p>mine</p>")))
	if !errors.Is(err, ErrConfluenceVersionConflict) || out.State != ActionFailed {
		t.Fatalf("version drift must fail definitively: state=%s err=%v", out.State, err)
	}
	posts, puts, gets := fake.stats()
	if posts != 0 || puts != 0 {
		t.Fatalf("AC1: conflict must leave ZERO write requests, got posts=%d puts=%d", posts, puts)
	}
	if gets == 0 {
		t.Fatal("the pre-read version GET must have run")
	}
	fake.mu.Lock()
	p := fake.pages["page-9"]
	fake.mu.Unlock()
	if p.storage != "<p>external edit</p>" || p.version != 4 {
		t.Fatalf("the external edit must be untouched: %+v", p)
	}
}

func TestConfluenceUpdatePreReadFailureIsFailedNotUnknown(t *testing.T) {
	fake := newFakeConfluence("cf-user@example.test", "secret_cf_token")
	srv := fake.server(t)
	ad := &ConfluenceUpdateAdapter{
		Policy: cfPolicyOf(srv, ""), Credential: cfCredential,
		Edition: EditionCloud, ConnectionCapabilities: cfCaps(ConfluenceCapabilityWrite),
	}

	out, err := ad.Execute(context.Background(), cfUpdateAction(cfUpdateArgs("page-missing", "3", "T", "s")))
	if out.State != ActionFailed || err == nil {
		t.Fatalf("missing page (a GET that never wrote) must fail definitively: state=%s err=%v", out.State, err)
	}
	_, puts, _ := fake.stats()
	if puts != 0 {
		t.Fatalf("no write may follow an unreadable pre-read: %d", puts)
	}
}

func TestConfluenceUpdateUnknownOnLostWriteReply(t *testing.T) {
	fake := newFakeConfluence("cf-user@example.test", "secret_cf_token")
	fake.addPage("page-9", "sp-1", "ENG", "old title", 3)
	srv := fake.server(t)
	ad := &ConfluenceUpdateAdapter{
		Policy: cfPolicyOf(srv, ""), Credential: cfCredential,
		Edition: EditionCloud, ConnectionCapabilities: cfCaps(ConfluenceCapabilityWrite),
	}
	fake.mu.Lock()
	fake.dropNextWrite = true
	fake.mu.Unlock()

	out, err := ad.Execute(context.Background(), cfUpdateAction(cfUpdateArgs("page-9", "3", "new title", "<p>next</p>")))
	if out.State != ActionUnknown || !errors.Is(err, ErrConfluenceOutcomeUnknown) {
		t.Fatalf("lost write reply must park unknown, got state=%s err=%v", out.State, err)
	}
	fake.mu.Lock()
	p := fake.pages["page-9"]
	fake.mu.Unlock()
	if p.version != 4 || p.storage != "<p>next</p>" {
		t.Fatalf("the effect DID apply (the fake drops the reply after): %+v", p)
	}
}

func TestConfluenceUpdateQueryResolvesAfterDroppedReply(t *testing.T) {
	fake := newFakeConfluence("cf-user@example.test", "secret_cf_token")
	fake.addPage("page-9", "sp-1", "ENG", "old title", 3)
	srv := fake.server(t)
	ad := &ConfluenceUpdateAdapter{
		Policy: cfPolicyOf(srv, ""), Credential: cfCredential,
		Edition: EditionCloud, ConnectionCapabilities: cfCaps(ConfluenceCapabilityWrite),
	}
	fake.mu.Lock()
	fake.dropNextWrite = true
	fake.mu.Unlock()
	act := cfUpdateAction(cfUpdateArgs("page-9", "3", "new title", "<p>next</p>"))
	out, err := ad.Execute(context.Background(), act)
	if out.State != ActionUnknown {
		t.Fatalf("dropped reply must park unknown, got %s (%v)", out.State, err)
	}
	_, putsBefore, _ := fake.stats()
	// AC2: reconcile by READING the remote first — version advanced by
	// exactly one AND the content is ours, so the query must confirm
	// success without any new write.
	q, qerr := ad.Query(context.Background(), act)
	if qerr != nil || q.State != ActionSucceeded || q.ExternalID != "page-9" {
		t.Fatalf("query must resolve the unknown from the remote state: %+v %v", q, qerr)
	}
	rcpt, rerr := ParseConfluencePageReceipt(q.Output)
	if rerr != nil || rcpt.ExternalVersion != "4" {
		t.Fatalf("query receipt drift: %+v %v", rcpt, rerr)
	}
	_, putsAfter, _ := fake.stats()
	if putsAfter != putsBefore {
		t.Fatalf("query must not re-send: puts %d -> %d", putsBefore, putsAfter)
	}
}

func TestConfluenceUpdateQueryStaysUnknownWhenContentDrifted(t *testing.T) {
	fake := newFakeConfluence("cf-user@example.test", "secret_cf_token")
	fake.addPage("page-9", "sp-1", "ENG", "old title", 3)
	srv := fake.server(t)
	ad := &ConfluenceUpdateAdapter{
		Policy: cfPolicyOf(srv, ""), Credential: cfCredential,
		Edition: EditionCloud, ConnectionCapabilities: cfCaps(ConfluenceCapabilityWrite),
	}
	fake.mu.Lock()
	fake.dropNextWrite = true
	fake.mu.Unlock()
	act := cfUpdateAction(cfUpdateArgs("page-9", "3", "new title", "<p>next</p>"))
	if out, _ := ad.Execute(context.Background(), act); out.State != ActionUnknown {
		t.Fatalf("setup: unknown expected, got %s", out.State)
	}
	// A SECOND external edit lands on top of ours: version moved past
	// expected+1 AND content no longer matches — not provably ours anymore.
	fake.bump("page-9", "<p>someone else</p>")
	q, _ := ad.Query(context.Background(), act)
	if q.State != ActionUnknown {
		t.Fatalf("drifted content must stay unknown, got %s", q.State)
	}
}

func TestConfluenceUpdateQueryStaysUnknownWhenVersionUnmoved(t *testing.T) {
	fake := newFakeConfluence("cf-user@example.test", "secret_cf_token")
	fake.addPage("page-9", "sp-1", "ENG", "old title", 3)
	srv := fake.server(t)
	ad := &ConfluenceUpdateAdapter{
		Policy: cfPolicyOf(srv, ""), Credential: cfCredential,
		Edition: EditionCloud, ConnectionCapabilities: cfCaps(ConfluenceCapabilityWrite),
	}
	// No write ever happened: version still 3 — the lost PUT provably did
	// not apply, but the ADAPTER's Query still reports unknown (definitive
	// failed settlement is the pipeline's business, not the query's).
	q, _ := ad.Query(context.Background(), cfUpdateAction(cfUpdateArgs("page-9", "3", "new title", "<p>next</p>")))
	if q.State != ActionUnknown {
		t.Fatalf("unmoved version must stay unknown at adapter level, got %s", q.State)
	}
}

// TestConfluenceUpdateQueryOutputIsReceiptProjection pins the exact
// success-output shape the settle path consumes (T50 final-review
// finding): a faithful local projection {"id","version":{"number":next}}
// assembled from the remote read the query just validated — never the raw
// reply of a PUT this query did not issue — and parseable by
// ParseConfluencePageReceipt with no other fields.
func TestConfluenceUpdateQueryOutputIsReceiptProjection(t *testing.T) {
	fake := newFakeConfluence("cf-user@example.test", "secret_cf_token")
	fake.addPage("page-9", "sp-1", "ENG", "old title", 3)
	srv := fake.server(t)
	ad := &ConfluenceUpdateAdapter{
		Policy: cfPolicyOf(srv, ""), Credential: cfCredential,
		Edition: EditionCloud, ConnectionCapabilities: cfCaps(ConfluenceCapabilityWrite),
	}
	fake.mu.Lock()
	fake.dropNextWrite = true
	fake.mu.Unlock()
	act := cfUpdateAction(cfUpdateArgs("page-9", "3", "new title", "<p>next</p>"))
	if out, _ := ad.Execute(context.Background(), act); out.State != ActionUnknown {
		t.Fatalf("setup: unknown expected, got %s", out.State)
	}
	q, qerr := ad.Query(context.Background(), act)
	if qerr != nil || q.State != ActionSucceeded {
		t.Fatalf("setup: query must resolve, got %+v %v", q, qerr)
	}
	var proj map[string]any
	if err := json.Unmarshal(q.Output, &proj); err != nil {
		t.Fatalf("query output must be JSON: %v", err)
	}
	if len(proj) != 2 {
		t.Fatalf("projection must carry exactly id+version, got %v", proj)
	}
	if proj["id"] != "page-9" {
		t.Fatalf("projection id drift: %v", proj["id"])
	}
	ver, ok := proj["version"].(map[string]any)
	if !ok || ver["number"] != float64(4) {
		t.Fatalf("projection version drift: %v", proj["version"])
	}
	// The settle path (publish/confluence.go) consumes the projection
	// through the receipt parser — it must keep parsing field-for-field.
	rcpt, rerr := ParseConfluencePageReceipt(q.Output)
	if rerr != nil || rcpt.ExternalID != "page-9" || rcpt.ExternalVersion != "4" {
		t.Fatalf("settle receipt consumption drift: %+v %v", rcpt, rerr)
	}
}

func TestReadConfluencePageVersion(t *testing.T) {
	fake := newFakeConfluence("cf-user@example.test", "secret_cf_token")
	fake.addPage("page-9", "sp-1", "ENG", "P", 4)
	srv := fake.server(t)
	v, err := ReadConfluencePageVersion(context.Background(), cfPolicyOf(srv, ""), cfCredential, EditionCloud, "", "page-9")
	if err != nil || v != "4" {
		t.Fatalf("pre-read drift: %q %v", v, err)
	}
	if _, err := ReadConfluencePageVersion(context.Background(), cfPolicyOf(srv, ""), cfCredential, EditionCloud, "", "missing"); err == nil {
		t.Fatal("missing page must error, never fabricate a version")
	}
}

// TestConfluenceQueryConvergesAfterServerEntityNormalization pins R5-F7:
// the publish side writes the locally-escaped storage form (&#39;/&#34;
// entities, CRLF), while the real Confluence serializer writes back bare
// quotes/apostrophes on LF-only whitespace. A byte-exact comparison can
// never converge for quoted bodies — the query must compare NORMALIZED
// forms (entity-unescape + CRLF→LF + trim on both sides).
func TestConfluenceQueryConvergesAfterServerEntityNormalization(t *testing.T) {
	fake := newFakeConfluence("cf-user@example.test", "secret_cf_token")
	fake.addPage("page-9", "sp-1", "ENG", "old title", 3)
	srv := fake.server(t)
	ad := &ConfluenceUpdateAdapter{
		Policy: cfPolicyOf(srv, ""), Credential: cfCredential,
		Edition: EditionCloud, ConnectionCapabilities: cfCaps(ConfluenceCapabilityWrite),
	}
	fake.mu.Lock()
	fake.dropNextWrite = true
	fake.mu.Unlock()
	storage := "<p>it&#39;s &#34;quoted&#34;\r\nsecond line</p>"
	act := cfUpdateAction(cfUpdateArgs("page-9", "3", "Report", storage))
	if out, _ := ad.Execute(context.Background(), act); out.State != ActionUnknown {
		t.Fatalf("setup: unknown expected, got %s", out.State)
	}
	// The effect applied in the escaped form; every later read is served in
	// the server's own re-serialized form (entities restored, LF-only).
	fake.mu.Lock()
	fake.reserialize = true
	fake.mu.Unlock()

	q, qerr := ad.Query(context.Background(), act)
	if qerr != nil || q.State != ActionSucceeded {
		t.Fatalf("entity/whitespace-only reserialization must converge, got state=%s err=%v", q.State, qerr)
	}
	_, putsAfter, _ := fake.stats()
	if putsAfter != 1 {
		t.Fatalf("query must not re-send: puts=%d", putsAfter)
	}
}

// TestConfluenceQueryStillRejectsRealContentDrift pins R5-F7's honest side:
// normalization only collapses KNOWN re-serialization noise — a remote body
// with genuinely different content (an extra paragraph) stays unverifiable
// even when the version advanced by exactly one.
func TestConfluenceQueryStillRejectsRealContentDrift(t *testing.T) {
	fake := newFakeConfluence("cf-user@example.test", "secret_cf_token")
	fake.addPage("page-9", "sp-1", "ENG", "old title", 3)
	srv := fake.server(t)
	ad := &ConfluenceUpdateAdapter{
		Policy: cfPolicyOf(srv, ""), Credential: cfCredential,
		Edition: EditionCloud, ConnectionCapabilities: cfCaps(ConfluenceCapabilityWrite),
	}
	fake.mu.Lock()
	fake.dropNextWrite = true
	fake.mu.Unlock()
	act := cfUpdateAction(cfUpdateArgs("page-9", "3", "Report", "<p>ours</p>"))
	if out, _ := ad.Execute(context.Background(), act); out.State != ActionUnknown {
		t.Fatalf("setup: unknown expected, got %s", out.State)
	}
	// Version stays ours (4) but the remote body carries real extra content.
	fake.mu.Lock()
	fake.pages["page-9"].storage = "<p>ours</p><p>someone appended real text</p>"
	fake.mu.Unlock()

	q, qerr := ad.Query(context.Background(), act)
	if q.State != ActionUnknown || qerr == nil {
		t.Fatalf("real content drift must stay unverifiable, got state=%s err=%v", q.State, qerr)
	}
	if !strings.Contains(qerr.Error(), "confluence_query_unverifiable") {
		t.Fatalf("the honest refusal must stay classified unverifiable, got: %v", qerr)
	}
}
