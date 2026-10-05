package v8

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	typesLocal "github.com/Tencent/WeKnora/internal/types"
)

// newBatchSaveRepository serves bulkBody for every _bulk request.
// Regression guard for #3832: Elasticsearch answers HTTP 200 with
// errors=true and items[].<op>.error when individual documents are
// rejected. Those documents never enter the index and are not retried,
// so BatchSave must not report success.
func newBatchSaveRepository(t *testing.T, bulkBody string) *elasticsearchRepository {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		if r.URL.Path != "/vectors/_bulk" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			return
		}
		_, _ = w.Write([]byte(bulkBody))
	}))
	t.Cleanup(server.Close)
	client, err := elasticsearch.NewTypedClient(
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
		`{"create":{"_index":"vectors","_id":"doc-1","status":201,"result":"created"}},`+
		`{"create":{"_index":"vectors","_id":"doc-2","status":400,`+
		`"error":{"type":"mapper_parsing_exception","reason":"failed to parse field [embedding] of type [dense_vector]"}}}`+
		`]}`)

	err := repo.BatchSave(context.Background(), batchSaveDocs(), nil)

	require.Error(t, err, "a 200 bulk response with errors=true must not report success")
	assert.Contains(t, err.Error(), "1/2")
	assert.Contains(t, err.Error(), "mapper_parsing_exception")
	assert.Contains(t, err.Error(), "failed to parse field [embedding]")
}

func TestBatchSaveSucceedsWithoutItemErrors(t *testing.T) {
	repo := newBatchSaveRepository(t, `{"took":5,"errors":false,"items":[`+
		`{"create":{"_index":"vectors","_id":"doc-1","status":201,"result":"created"}},`+
		`{"create":{"_index":"vectors","_id":"doc-2","status":201,"result":"created"}}`+
		`]}`)

	require.NoError(t, repo.BatchSave(context.Background(), batchSaveDocs(), nil))
}

// errors=true without any per-item detail still fails the call: the bulk
// outcome is a failure, and returning nil would recreate the fake success.
func TestBatchSaveFailsOnErrorsWithoutItemDetail(t *testing.T) {
	repo := newBatchSaveRepository(t, `{"took":5,"errors":true,"items":[]}`)

	err := repo.BatchSave(context.Background(), batchSaveDocs(), nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "errors=true")
}
