package v7

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/elastic/go-elasticsearch/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	typesLocal "github.com/Tencent/WeKnora/internal/types"
)

// newBatchSaveRepository serves bulkBody for every _bulk request.
// Regression guard for #3832: Elasticsearch answers HTTP 200 with
// errors=true and items[].index.error when individual documents are
// rejected. Those documents never enter the index and are not retried,
// so BatchSave must not report success.
func newBatchSaveRepository(t *testing.T, bulkBody string) *elasticsearchRepository {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		switch r.URL.Path {
		case "/":
			_, _ = w.Write([]byte(`{"version":{"number":"7.17.0","build_flavor":"default"},` +
				`"tagline":"You Know, for Search"}`))
		case "/vectors/_bulk":
			_, _ = w.Write([]byte(bulkBody))
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)
	client, err := elasticsearch.NewClient(
		elasticsearch.Config{Addresses: []string{server.URL}, DisableRetry: true},
	)
	require.NoError(t, err)
	return &elasticsearchRepository{client: client, index: "vectors"}
}

func batchSaveDocs() []*typesLocal.IndexInfo {
	return []*typesLocal.IndexInfo{
		{Content: "first chunk", SourceID: "s1", ChunkID: "c1", KnowledgeID: "k1", KnowledgeBaseID: "kb1"},
		{Content: "second chunk", SourceID: "s2", ChunkID: "c2", KnowledgeID: "k1", KnowledgeBaseID: "kb1"},
	}
}

func TestBatchSaveSurfacesPerItemErrors(t *testing.T) {
	repo := newBatchSaveRepository(t, `{"took":5,"errors":true,"items":[`+
		`{"index":{"_index":"vectors","_id":"doc-1","status":201,"result":"created"}},`+
		`{"index":{"_index":"vectors","_id":"doc-2","status":400,`+
		`"error":{"type":"mapper_parsing_exception","reason":"failed to parse field [embedding] of type [dense_vector]"}}}`+
		`]}`)

	err := repo.BatchSave(context.Background(), batchSaveDocs(), nil)

	require.Error(t, err, "a 200 bulk response with errors=true must not report success")
	assert.Contains(t, err.Error(), "1/2")
	assert.Contains(t, err.Error(), "mapper_parsing_exception")
	// Upstream #3842 excludes error.reason from surfaced messages because it
	// can embed document content; only the bounded error.type travels.
	assert.Contains(t, err.Error(), "doc-2")
	assert.NotContains(t, err.Error(), "failed to parse field [embedding]")
}

func TestBatchSaveSucceedsWithoutItemErrors(t *testing.T) {
	repo := newBatchSaveRepository(t, `{"took":5,"errors":false,"items":[`+
		`{"index":{"_index":"vectors","_id":"doc-1","status":201,"result":"created"}},`+
		`{"index":{"_index":"vectors","_id":"doc-2","status":201,"result":"created"}}`+
		`]}`)

	require.NoError(t, repo.BatchSave(context.Background(), batchSaveDocs(), nil))
}

// errors=true without a concrete failed item leaves the outcome unknown, so
// upstream #3842 reports it as an error instead of tolerating it silently.
func TestBatchSaveToleratesErrorsFlagWithoutFailedItems(t *testing.T) {
	repo := newBatchSaveRepository(t, `{"took":5,"errors":true,"items":[]}`)

	err := repo.BatchSave(context.Background(), batchSaveDocs(), nil)

	require.Error(t, err, "errors=true with no item detail is an unknown outcome, not success")
	assert.Contains(t, err.Error(), "without per-item failure detail")
}
