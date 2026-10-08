package appconnector

// FE-PUB-01 contract-layer tests for the Feishu docx publish family
// (#49). The contract facts are pinned against the official oapi-sdk-go
// v3.9.7 docx/v1 service source; the fakes in Task 2 reproduce the wire
// shapes from that source.

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParseFeishuDocCreateSnapshotExactFields(t *testing.T) {
	args := json.RawMessage(`{"parent_folder":"fld-1","title":"Report","blocks":[{"block_type":2}]}`)
	snap, err := ParseFeishuDocCreateSnapshot(args)
	require.NoError(t, err)
	require.Equal(t, "fld-1", snap.ParentFolder)
	require.Equal(t, "Report", snap.Title)
	require.Len(t, snap.Blocks, 1)
}

func TestParseFeishuDocCreateSnapshotRejectsShapeDrift(t *testing.T) {
	cases := map[string]json.RawMessage{
		"extra field":    json.RawMessage(`{"parent_folder":"f","title":"T","blocks":[],"x":1}`),
		"missing title":  json.RawMessage(`{"parent_folder":"f","blocks":[]}`),
		"missing folder": json.RawMessage(`{"title":"T","blocks":[]}`),
		"missing blocks": json.RawMessage(`{"parent_folder":"f","title":"T"}`),
		"empty folder":   json.RawMessage(`{"parent_folder":"","title":"T","blocks":[]}`),
		"empty title":    json.RawMessage(`{"parent_folder":"f","title":"","blocks":[]}`),
		"invalid block":  json.RawMessage(`{"parent_folder":"f","title":"T","blocks":["not-json-object"]}`),
		"not an object":  json.RawMessage(`["parent_folder"]`),
	}
	for name, args := range cases {
		_, err := ParseFeishuDocCreateSnapshot(args)
		require.ErrorIs(t, err, ErrFeishuPublishSnapshotInvalid, name)
	}
}

func TestParseFeishuDocUpdateSnapshotExactFields(t *testing.T) {
	args := json.RawMessage(`{"document_id":"doc-1","expected_revision":"3","title":"Report v2","blocks":[]}`)
	snap, err := ParseFeishuDocUpdateSnapshot(args)
	require.NoError(t, err)
	require.Equal(t, "doc-1", snap.DocumentID)
	require.Equal(t, "3", snap.ExpectedRevision)
	require.Equal(t, "Report v2", snap.Title)
	require.Empty(t, snap.Blocks)
}

func TestParseFeishuDocUpdateSnapshotRejectsShapeDrift(t *testing.T) {
	cases := map[string]json.RawMessage{
		"extra field":      json.RawMessage(`{"document_id":"d","expected_revision":"1","title":"T","blocks":[],"x":1}`),
		"missing revision": json.RawMessage(`{"document_id":"d","title":"T","blocks":[]}`),
		"empty revision":   json.RawMessage(`{"document_id":"d","expected_revision":"","title":"T","blocks":[]}`),
		"missing document": json.RawMessage(`{"expected_revision":"1","title":"T","blocks":[]}`),
		"missing title":    json.RawMessage(`{"document_id":"d","expected_revision":"1","blocks":[]}`),
		"missing blocks":   json.RawMessage(`{"document_id":"d","expected_revision":"1","title":"T"}`),
		"blocks not json":  json.RawMessage(`{"document_id":"d","expected_revision":"1","title":"T","blocks":[12]}`),
	}
	for name, args := range cases {
		_, err := ParseFeishuDocUpdateSnapshot(args)
		require.ErrorIs(t, err, ErrFeishuPublishSnapshotInvalid, name)
	}
}

func TestIsFeishuDocUpdateArgs(t *testing.T) {
	require.True(t, IsFeishuDocUpdateArgs(json.RawMessage(`{"document_id":"d","expected_revision":"1","title":"T","blocks":[]}`)))
	require.False(t, IsFeishuDocUpdateArgs(json.RawMessage(`{"parent_folder":"f","title":"T","blocks":[]}`)))
	require.False(t, IsFeishuDocUpdateArgs(json.RawMessage(`{"document_id":"d"}`)))
	require.False(t, IsFeishuDocUpdateArgs(json.RawMessage(`not json`)))
}

func TestDetectFeishuRevisionConflict(t *testing.T) {
	require.NoError(t, DetectFeishuRevisionConflict("3", "3"))
	require.ErrorIs(t, DetectFeishuRevisionConflict("3", "4"), ErrFeishuPublishRevisionConflict)
	require.ErrorIs(t, DetectFeishuRevisionConflict("", "3"), ErrFeishuPublishRevisionConflict)
	require.ErrorIs(t, DetectFeishuRevisionConflict("3", ""), ErrFeishuPublishRevisionConflict)
	// The numeric wire shape must not leak into the comparison: "03" vs
	// "3" is a mismatch — the adapter always stores strconv.Itoa output.
	require.ErrorIs(t, DetectFeishuRevisionConflict("03", "3"), ErrFeishuPublishRevisionConflict)
}

func TestFeishuTextBlocksDerivesTextParagraphs(t *testing.T) {
	blocks, err := FeishuTextBlocks("第一段。\n\n第二段。")
	require.NoError(t, err)
	require.Len(t, blocks, 2)
	var first struct {
		BlockType int `json:"block_type"`
		Text      struct {
			Elements []struct {
				TextRun struct {
					Content string `json:"content"`
				} `json:"text_run"`
			} `json:"elements"`
			Style struct{} `json:"style"`
		} `json:"text"`
	}
	require.NoError(t, json.Unmarshal(blocks[0], &first))
	require.Equal(t, 2, first.BlockType, "block_type 2 = text block")
	require.Len(t, first.Text.Elements, 1)
	require.Equal(t, "第一段。", first.Text.Elements[0].TextRun.Content)

	// Deterministic: the same bytes always derive the same blocks.
	again, err := FeishuTextBlocks("第一段。\n\n第二段。")
	require.NoError(t, err)
	require.Equal(t, blocks, again)
}

func TestFeishuTextBlocksEmptyAndOversize(t *testing.T) {
	_, err := FeishuTextBlocks("  \n\n  ")
	require.ErrorIs(t, err, ErrFeishuPublishEmptyContent)

	long := strings.Repeat("段\n\n", feishuDocMaxParagraphs) + "段" // 501 paragraphs
	_, err = FeishuTextBlocks(long)
	require.ErrorIs(t, err, ErrFeishuPublishContentTooLarge)
}

func TestFeishuTextBlocksChunksLongParagraph(t *testing.T) {
	long := strings.Repeat("x", feishuTextRunChunk+10)
	blocks, err := FeishuTextBlocks(long)
	require.NoError(t, err)
	require.Len(t, blocks, 1)
	joined := ""
	var parsed struct {
		Text struct {
			Elements []struct {
				TextRun struct {
					Content string `json:"content"`
				} `json:"text_run"`
			} `json:"elements"`
		} `json:"text"`
	}
	require.NoError(t, json.Unmarshal(blocks[0], &parsed))
	for _, e := range parsed.Text.Elements {
		joined += e.TextRun.Content
	}
	require.Equal(t, long, joined, "chunking must preserve the full content")
}

func TestParseFeishuDocumentVersionReadsNumericRevision(t *testing.T) {
	// data.document.revision_id is a JSON NUMBER on the wire (SDK
	// Document.RevisionId *int); the parser stringifies it.
	raw := []byte(`{"code":0,"data":{"document":{"document_id":"doc-1","revision_id":7,"title":""}}}`)
	v, err := ParseFeishuDocumentVersion(raw)
	require.NoError(t, err)
	require.Equal(t, "doc-1", v.DocumentID)
	require.Equal(t, "7", v.RevisionID)

	_, err = ParseFeishuDocumentVersion([]byte(`{"code":0,"data":{"document":{"document_id":"doc-1"}}}`))
	require.Error(t, err, "a reply without a revision is never a version")

	_, err = ParseFeishuDocumentVersion([]byte(`{"code":99991663,"msg":"denied"}`))
	require.Error(t, err)

	// The official not-exist code is TYPED: the plan pre-read tells
	// "folder has no revision" apart from an unreadable destination.
	_, err = ParseFeishuDocumentVersion([]byte(`{"code":99991661,"msg":"not exist"}`))
	require.ErrorIs(t, err, ErrFeishuPublishNotFound)
}

func TestParseFeishuDocReceipt(t *testing.T) {
	rcpt, err := ParseFeishuDocReceipt([]byte(`{"document":{"document_id":"doc-9","revision_id":12}}`))
	require.NoError(t, err)
	require.Equal(t, "doc-9", rcpt.ExternalID)
	require.Equal(t, "12", rcpt.ExternalVersion)
}

func TestFeishuDocBlockContentsExtractsTextRuns(t *testing.T) {
	raw := []byte(`[{"block_id":"b1","parent_id":"doc-1","block_type":2,"text":{"elements":[{"text_run":{"content":"a"}}],"style":{}}},
		{"block_id":"b2","block_type":2,"text":{"elements":[{"text_run":{"content":"b"}}]}}]`)
	got, err := feishuDocBlockContents(raw)
	require.NoError(t, err)
	require.Equal(t, []string{"a", "b"}, got)
}

// ---- the docx contract double: reproduces the official docx/v1 wire
// shapes (envelope {code,msg,data}; numeric revision_id; children carry
// server-side fields) from the SDK source. Same self-containment
// discipline as the handler package's e2eNotion. ----

type fakeFeishuDocx struct {
	mu             sync.Mutex
	token          string
	docs           map[string]*fakeFeishuDoc
	nextID         int
	revisionBase   int
	createCalls    int
	appendCalls    int
	appendSizes    []int
	dropNextCreate bool
	// dropAppendAt is the 1-based index of the append call whose REPLY is
	// lost mid-flight (0 = none). The remote effect still lands — only the
	// response disappears (the partial-success shape these tests pin).
	// (The plan's draft used a single boolean, which could only ever drop
	// the FIRST append; the tests' own comments require the drop on the
	// FINAL batch, so the index form is the faithful shape.)
	dropAppendAt int
}

type fakeFeishuDoc struct {
	id       string
	revision int
	children []json.RawMessage
}

func newFakeFeishuDocx(token string) *fakeFeishuDocx {
	return &fakeFeishuDocx{token: token, docs: map[string]*fakeFeishuDoc{}, revisionBase: 1}
}

func (f *fakeFeishuDocx) server(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	write := func(w http.ResponseWriter, body string) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write([]byte(body))
	}
	auth := func(r *http.Request) bool { return r.Header.Get("Authorization") == "Bearer "+f.token }
	mux.HandleFunc("/open-apis/docx/v1/documents", func(w http.ResponseWriter, r *http.Request) {
		if !auth(r) {
			write(w, `{"code":99991663,"msg":"invalid token"}`)
			return
		}
		if r.Method != http.MethodPost {
			write(w, `{"code":99991661,"msg":"method"}`)
			return
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.dropNextCreate {
			f.dropNextCreate = false
			panic(http.ErrAbortHandler)
		}
		f.createCalls++
		var req struct {
			FolderToken string `json:"folder_token"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		f.nextID++
		id := fmt.Sprintf("doc-%d", f.nextID)
		f.docs[id] = &fakeFeishuDoc{id: id, revision: f.revisionBase}
		write(w, fmt.Sprintf(`{"code":0,"data":{"document":{"document_id":%q,"revision_id":%d,"title":""}}}`, id, f.revisionBase))
	})
	mux.HandleFunc("/open-apis/docx/v1/documents/", func(w http.ResponseWriter, r *http.Request) {
		if !auth(r) {
			write(w, `{"code":99991663,"msg":"invalid token"}`)
			return
		}
		rest := strings.TrimPrefix(r.URL.Path, "/open-apis/docx/v1/documents/")
		switch {
		case !strings.Contains(rest, "/blocks/"):
			// GET /open-apis/docx/v1/documents/{id} — the version read.
			id := rest
			f.mu.Lock()
			defer f.mu.Unlock()
			d, ok := f.docs[id]
			if !ok {
				write(w, `{"code":99991661,"msg":"not found"}`)
				return
			}
			write(w, fmt.Sprintf(`{"code":0,"data":{"document":{"document_id":%q,"revision_id":%d,"title":""}}}`, d.id, d.revision))
		case strings.HasSuffix(rest, "/children"):
			// {id}/blocks/{block_id}/children — POST append / GET list.
			segs := strings.Split(rest, "/")
			id := segs[0]
			f.mu.Lock()
			defer f.mu.Unlock()
			d, ok := f.docs[id]
			if !ok {
				write(w, `{"code":99991661,"msg":"not found"}`)
				return
			}
			switch r.Method {
			case http.MethodPost:
				var req struct {
					Children []json.RawMessage `json:"children"`
					Index    *int              `json:"index"`
				}
				_ = json.NewDecoder(r.Body).Decode(&req)
				// Record the effect FIRST, then optionally drop the reply —
				// the exact partial-success shape the notion double pins
				// (effect applied remotely, response lost mid-flight).
				f.appendCalls++
				f.appendSizes = append(f.appendSizes, len(req.Children))
				for _, ch := range req.Children {
					// Provider blocks carry server-side fields the approved
					// snapshot bytes never have (Review Focus 5, plan line:
					// "fake 与 E2E 的 children 响应携带 block_id/parent_id 等
					// 服务端字段") — the double annotates every stored child
					// so ONLY the semantic content-sequence comparison can
					// ever succeed; a byte comparison would always fail.
					var obj map[string]json.RawMessage
					_ = json.Unmarshal(ch, &obj)
					obj["block_id"] = json.RawMessage(fmt.Sprintf("%q", fmt.Sprintf("%s-child-%d", id, len(d.children))))
					obj["parent_id"] = json.RawMessage(fmt.Sprintf("%q", id))
					merged, merr := json.Marshal(obj)
					if merr != nil {
						write(w, `{"code":99991661,"msg":"block"}`)
						return
					}
					d.children = append(d.children, merged)
				}
				d.revision++
				if f.dropAppendAt > 0 && f.appendCalls == f.dropAppendAt {
					f.dropAppendAt = 0
					panic(http.ErrAbortHandler)
				}
				write(w, fmt.Sprintf(`{"code":0,"data":{"children":[],"document_revision_id":%d}}`, d.revision))
			case http.MethodGet:
				// Paginated list; the fake honors page_size=1 so the
				// pagination-follow test can prove the adapter walks it.
				q := r.URL.Query()
				pageToken := q.Get("page_token")
				start := 0
				if pageToken != "" {
					_, _ = fmt.Sscanf(pageToken, "%d", &start)
				}
				end := start + 1
				hasMore := end < len(d.children)
				items := d.children
				if start < len(d.children) {
					if end > len(d.children) {
						end = len(d.children)
					}
					items = d.children[start:end]
				} else {
					items = nil
				}
				next := ""
				if hasMore {
					next = fmt.Sprintf("%d", end)
				}
				write(w, fmt.Sprintf(`{"code":0,"data":{"items":[%s],"page_token":%q,"has_more":%t}}`,
					strings.Join(rawList(items), ","), next, hasMore))
			default:
				write(w, `{"code":99991661,"msg":"method"}`)
			}
		default:
			write(w, `{"code":99991661,"msg":"unknown path"}`)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func rawList(items []json.RawMessage) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = string(it)
	}
	return out
}

// docAdapter builds a FeishuDocxAdapter over the fake with an in-memory
// progress map, mirroring the publicationProgress wiring the bridge does
// in production.
func docAdapter(t *testing.T, fake *fakeFeishuDocx, caps []string, batch int) (*FeishuDocxAdapter, map[string]FeishuDocProgress) {
	t.Helper()
	srv := fake.server(t)
	_, network, _ := net.ParseCIDR("127.0.0.0/8")
	host, port, _ := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	pol := HTTPPolicy{
		Scheme: "http", Host: host, Port: port,
		Methods: []string{"GET", "POST"}, PathPrefix: "/open-apis/docx/",
		AuthorizedNetworks: []*net.IPNet{network}, Timeout: 10 * time.Second,
	}
	progress := map[string]FeishuDocProgress{}
	return &FeishuDocxAdapter{
		Policy: pol,
		Token:  func(ctx context.Context) (string, error) { return fake.token, nil },
		ConnectionCapabilities: func(ctx context.Context, a Action) ([]string, error) {
			return caps, nil
		},
		LoadProgress: func(a Action) FeishuDocProgress { return progress[a.ID] },
		SaveProgress: func(a Action, p FeishuDocProgress) error { progress[a.ID] = p; return nil },
		MaxBatch:     batch,
	}, progress
}

func feishuCreateAction(id string, folder string, n int) (Action, []string) {
	// The blocks and the expected content sequence derive from the SAME
	// numbered paragraphs, so the want slice is exactly what the derived
	// blocks carry (per-paragraph distinct content proves per-block
	// fidelity end to end).
	paras := make([]string, 0, n)
	want := make([]string, 0, n)
	for i := 0; i < n; i++ {
		p := fmt.Sprintf("段落%d。", i+1)
		paras = append(paras, p)
		want = append(want, p)
	}
	blocks, _ := FeishuTextBlocks(strings.Join(paras, "\n\n"))
	args, _ := json.Marshal(map[string]any{"parent_folder": folder, "title": "Report", "blocks": blocks})
	return Action{ID: id, TenantID: 7, ActorID: "u1", ConnectionID: "c1", Version: "feishu/v1",
		Target: folder, Risk: RiskWrite, Args: args}, want
}

func TestFeishuDocxAdapterCreateFullLoop(t *testing.T) {
	fake := newFakeFeishuDocx("secret_test_token")
	ad, progress := docAdapter(t, fake, []string{FeishuCapabilityWriteDocx}, 2)
	a, want := feishuCreateAction("act-1", "fld-1", 3)

	out, err := ad.Execute(context.Background(), a)
	require.NoError(t, err)
	require.Equal(t, ActionSucceeded, out.State)
	require.NotEmpty(t, out.ExternalID)

	// One create + two batch appends (2+1): the documented cap is never
	// exceeded, and the progress record ends complete.
	fake.mu.Lock()
	require.Equal(t, 1, fake.createCalls)
	require.Equal(t, []int{2, 1}, fake.appendSizes)
	doc := fake.docs[out.ExternalID]
	fake.mu.Unlock()
	require.Len(t, doc.children, 3)

	p := progress[a.ID]
	require.Equal(t, out.ExternalID, p.DocumentID)
	require.Equal(t, 3, p.BlocksDone)

	// The committed children carry the approved content.
	got, err := feishuDocBlockContents([]byte("[" + strings.Join(rawList(doc.children), ",") + "]"))
	require.NoError(t, err)
	require.Equal(t, want, got)
}

func TestFeishuDocxAdapterCreateResumesFromCheckpoint(t *testing.T) {
	fake := newFakeFeishuDocx("secret_test_token")
	ad, progress := docAdapter(t, fake, []string{FeishuCapabilityWriteDocx}, 2)
	a, _ := feishuCreateAction("act-2", "fld-1", 5)

	// First attempt: create + batch1(2) + batch2(2) succeed with
	// checkpoints persisted; batch3(1) lands REMOTELY but loses its
	// reply → the honest state is unknown and the checkpoint stops at 4.
	fake.mu.Lock()
	fake.dropAppendAt = 3 // batches are [2,2,1]: the 3rd append loses its reply
	fake.mu.Unlock()
	out, err := ad.Execute(context.Background(), a)
	require.Error(t, err)
	require.Equal(t, ActionUnknown, out.State, "a lost append reply must park unknown")
	require.Equal(t, 4, progress[a.ID].BlocksDone, "checkpoint holds the confirmed batches")
	require.Equal(t, out.ExternalID, progress[a.ID].DocumentID)

	// Recovery on the SAME action: the persisted document id forces the
	// SAME document; confirmed batches (blocks 1-4) are NEVER re-sent;
	// only the unconfirmed tail re-dispatches (its remote effect already
	// landed once — the duplicated tail is the honest #48-aligned
	// semantics that Query's contiguous-run check still settles).
	out2, err2 := ad.Execute(context.Background(), a)
	require.NoError(t, err2)
	require.Equal(t, ActionSucceeded, out2.State)
	require.Equal(t, out.ExternalID, out2.ExternalID, "resume lands on the SAME document")

	fake.mu.Lock()
	require.Equal(t, 1, fake.createCalls, "NO second create after a persisted document id")
	require.Equal(t, []int{2, 2, 1, 1}, fake.appendSizes, "confirmed batches re-dispatch ZERO; only the tail re-sends")
	doc := fake.docs[out.ExternalID]
	fake.mu.Unlock()
	require.Len(t, doc.children, 6, "5 approved + 1 duplicated tail (recorded honestly)")
}

func TestFeishuDocxAdapterUpdateRevisionConflictRefusesBeforeWrite(t *testing.T) {
	fake := newFakeFeishuDocx("secret_test_token")
	ad, _ := docAdapter(t, fake, []string{FeishuCapabilityWriteDocx}, 0)
	// A remote document at revision 1.
	fake.mu.Lock()
	fake.nextID++
	fake.docs["doc-x"] = &fakeFeishuDoc{id: "doc-x", revision: 1}
	fake.mu.Unlock()
	blocks, _ := FeishuTextBlocks("追加内容")
	args, _ := json.Marshal(map[string]any{"document_id": "doc-x", "expected_revision": "1", "title": "T", "blocks": blocks})
	a := Action{ID: "act-3", TenantID: 7, ActorID: "u1", ConnectionID: "c1", Version: "feishu/v1",
		Target: "doc-x", Risk: RiskWrite, Args: args}

	// A collaborator moves the revision between approval and execute.
	fake.mu.Lock()
	fake.docs["doc-x"].revision = 5
	fake.mu.Unlock()

	out, err := ad.Execute(context.Background(), a)
	require.ErrorIs(t, err, ErrFeishuPublishRevisionConflict)
	require.Equal(t, ActionFailed, out.State)

	fake.mu.Lock()
	require.Equal(t, 0, fake.appendCalls, "AC1: conflict leaves ZERO writes")
	fake.mu.Unlock()
}

func TestFeishuDocxAdapterUpdateHappyPathAppends(t *testing.T) {
	fake := newFakeFeishuDocx("secret_test_token")
	ad, _ := docAdapter(t, fake, []string{FeishuCapabilityWriteDocx}, 0)
	fake.mu.Lock()
	fake.nextID++
	fake.docs["doc-y"] = &fakeFeishuDoc{id: "doc-y", revision: 1}
	fake.mu.Unlock()
	blocks, _ := FeishuTextBlocks("追加内容")
	args, _ := json.Marshal(map[string]any{"document_id": "doc-y", "expected_revision": "1", "title": "T", "blocks": blocks})
	a := Action{ID: "act-4", TenantID: 7, ActorID: "u1", ConnectionID: "c1", Version: "feishu/v1",
		Target: "doc-y", Risk: RiskWrite, Args: args}

	out, err := ad.Execute(context.Background(), a)
	require.NoError(t, err)
	require.Equal(t, ActionSucceeded, out.State)
	require.Equal(t, "doc-y", out.ExternalID)

	// The output evidence carries the NEW revision the publish produced.
	rcpt, rerr := ParseFeishuDocReceipt(out.Output)
	require.NoError(t, rerr)
	require.Equal(t, "doc-y", rcpt.ExternalID)
	require.NotEqual(t, "1", rcpt.ExternalVersion, "the receipt version must be the post-write revision")
}

func TestFeishuDocxAdapterReadOnlyCapabilityRefused(t *testing.T) {
	fake := newFakeFeishuDocx("secret_test_token")
	// AC2: the connection's reviewed scopes carry only a read capability.
	ad, _ := docAdapter(t, fake, []string{"read_docx"}, 0)
	a, _ := feishuCreateAction("act-5", "fld-1", 1)

	out, err := ad.Execute(context.Background(), a)
	require.ErrorIs(t, err, ErrFeishuPublishMissingCapability)
	require.Equal(t, ActionFailed, out.State)

	fake.mu.Lock()
	require.Equal(t, 0, fake.createCalls, "refusal happens BEFORE any network write")
	require.Equal(t, 0, fake.appendCalls)
	fake.mu.Unlock()
}

// TestFeishuDocxAdapterQueryUpdateReconcilesWithoutProgress pins the
// update-branch reconciliation anchor: when the FIRST append batch's
// reply is lost, executeUpdate never persists progress — the approved
// snapshot's document_id is the only reconciliation target available,
// and it IS the authority (FormPlan's update gate binds it to a
// document this connection previously published). Query must use it
// instead of parking unknown.
func TestFeishuDocxAdapterQueryUpdateReconcilesWithoutProgress(t *testing.T) {
	fake := newFakeFeishuDocx("secret_test_token")
	ad, progress := docAdapter(t, fake, []string{FeishuCapabilityWriteDocx}, 0)
	fake.mu.Lock()
	fake.nextID++
	fake.docs["doc-q"] = &fakeFeishuDoc{id: "doc-q", revision: 1}
	fake.dropAppendAt = 1
	fake.mu.Unlock()
	blocks, _ := FeishuTextBlocks("追加内容")
	args, _ := json.Marshal(map[string]any{"document_id": "doc-q", "expected_revision": "1", "title": "T", "blocks": blocks})
	a := Action{ID: "act-q", TenantID: 7, ActorID: "u1", ConnectionID: "c1", Version: "feishu/v1",
		Target: "doc-q", Risk: RiskWrite, Args: args}

	// The append lands REMOTELY but its reply is lost → unknown, and
	// being the FIRST batch, no checkpoint was ever persisted.
	out, _ := ad.Execute(context.Background(), a)
	require.Equal(t, ActionUnknown, out.State)
	require.Empty(t, progress[a.ID].DocumentID, "the lost first batch leaves NO persisted checkpoint")

	// Reconcile reads the remote through the approved snapshot's
	// document id — the content IS there (recorded before the drop) —
	// and settles success WITHOUT re-sending.
	fake.mu.Lock()
	appendsBefore := fake.appendCalls
	fake.mu.Unlock()
	q, err := ad.Query(context.Background(), a)
	require.NoError(t, err)
	require.Equal(t, ActionSucceeded, q.State, "the approved snapshot's document id reconciles a checkpoint-less update")
	require.Equal(t, "doc-q", q.ExternalID)
	fake.mu.Lock()
	require.Equal(t, appendsBefore, fake.appendCalls, "reconcile must not re-send")
	fake.mu.Unlock()
}

func TestFeishuDocxAdapterQueryReconcilesByContent(t *testing.T) {
	fake := newFakeFeishuDocx("secret_test_token")
	ad, _ := docAdapter(t, fake, []string{FeishuCapabilityWriteDocx}, 2)
	a, _ := feishuCreateAction("act-6", "fld-1", 3)

	// The create and both batches land REMOTELY; the final batch's reply
	// is lost → unknown, checkpoint=2.
	fake.mu.Lock()
	fake.dropAppendAt = 2 // batches are [2,1]: the 2nd (final) append loses its reply
	fake.mu.Unlock()
	out, _ := ad.Execute(context.Background(), a)
	require.Equal(t, ActionUnknown, out.State)

	// The full approved content IS remote (all 3 batches recorded).
	// Query's content check settles success WITHOUT any re-send — even
	// though the provider children carry server-side fields the snapshot
	// bytes never had (semantic comparison, Review Focus 5).
	fake.mu.Lock()
	appendsBefore := fake.appendCalls
	fake.mu.Unlock()

	q2, err2 := ad.Query(context.Background(), a)
	require.NoError(t, err2)
	require.Equal(t, ActionSucceeded, q2.State)
	require.Equal(t, out.ExternalID, q2.ExternalID)
	rcpt, rerr := ParseFeishuDocReceipt(q2.Output)
	require.NoError(t, rerr)
	require.Equal(t, out.ExternalID, rcpt.ExternalID)

	fake.mu.Lock()
	require.Equal(t, appendsBefore, fake.appendCalls, "reconcile must not re-send")
	fake.mu.Unlock()
}

func TestFeishuDocxAdapterQueryWithoutDocumentIDStaysUnknown(t *testing.T) {
	fake := newFakeFeishuDocx("secret_test_token")
	ad, _ := docAdapter(t, fake, []string{FeishuCapabilityWriteDocx}, 0)
	a, _ := feishuCreateAction("act-7", "fld-1", 1)
	// No progress: the create itself is unobservable. Feishu offers no
	// reliable title search — the honest outcome is unknown, never a
	// fabricated id from a title match.
	q, err := ad.Query(context.Background(), a)
	require.Error(t, err)
	require.Equal(t, ActionUnknown, q.State)
	fake.mu.Lock()
	require.Equal(t, 0, fake.createCalls, "Query never re-creates")
	fake.mu.Unlock()
}

func TestFeishuDocxAdapterQueryFollowsPagination(t *testing.T) {
	fake := newFakeFeishuDocx("secret_test_token")
	ad, _ := docAdapter(t, fake, []string{FeishuCapabilityWriteDocx}, 0)
	a, _ := feishuCreateAction("act-8", "fld-1", 3)
	out, err := ad.Execute(context.Background(), a)
	require.NoError(t, err)
	require.Equal(t, ActionSucceeded, out.State)

	// The fake lists ONE child per page; the adapter must walk page_token
	// to see all three before claiming success.
	q, err := ad.Query(context.Background(), a)
	require.NoError(t, err)
	require.Equal(t, ActionSucceeded, q.State)
}

func TestReadFeishuDocumentVersionPreRead(t *testing.T) {
	fake := newFakeFeishuDocx("secret_test_token")
	srv := fake.server(t)
	_, network, _ := net.ParseCIDR("127.0.0.0/8")
	host, port, _ := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	pol := HTTPPolicy{Scheme: "http", Host: host, Port: port, Methods: []string{"GET"},
		PathPrefix: "/open-apis/docx/", AuthorizedNetworks: []*net.IPNet{network}, Timeout: 10 * time.Second}
	tok := func(ctx context.Context) (string, error) { return fake.token, nil }

	fake.mu.Lock()
	fake.nextID++
	fake.docs["doc-v"] = &fakeFeishuDoc{id: "doc-v", revision: 9}
	fake.mu.Unlock()

	v, err := ReadFeishuDocumentVersion(context.Background(), pol, tok, "doc-v")
	require.NoError(t, err)
	require.Equal(t, "doc-v", v.DocumentID)
	require.Equal(t, "9", v.RevisionID)
}
