package weaviate

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdk "github.com/weaviate/weaviate-go-client/v5/weaviate"
)

// weaviateErrorBody is how Weaviate words a refused schema request.
func weaviateErrorBody(message string) string {
	body, _ := json.Marshal(map[string]any{"error": []any{map[string]any{"message": message}}})
	return string(body)
}

// schemaTestServer stands in for Weaviate's schema API. A class exists once a
// create has been answered with 200; probe runs before every existence check.
type schemaTestServer struct {
	mu      sync.Mutex
	exists  bool
	probes  int
	creates int
	probe   func()
	create  func(attempt int) (int, string)
}

func (s *schemaTestServer) repository(t *testing.T) *weaviateRepository {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/schema/"):
			// Answer from the state the worker found on arrival, before the
			// barrier releases the race. Sampling after it instead lets the
			// winner's create turn into an "already exists" answer for whoever
			// the scheduler runs last, so that worker skips the create
			// entirely and fewer than `workers` creates ever reach the server.
			s.mu.Lock()
			s.probes++
			exists := s.exists
			s.mu.Unlock()
			if s.probe != nil {
				s.probe()
			}
			if !exists {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_, _ = w.Write([]byte(`{"class":"Weknora_embeddings_1024"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/schema":
			s.mu.Lock()
			s.creates++
			code, body := s.create(s.creates)
			if code == http.StatusOK {
				s.exists = true
			}
			s.mu.Unlock()
			w.WriteHeader(code)
			_, _ = w.Write([]byte(body))
		case r.URL.Path == "/v1/meta":
			_, _ = w.Write([]byte(`{"version":"1.37.3"}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	client, err := sdk.NewClient(sdk.Config{Scheme: "http", Host: strings.TrimPrefix(server.URL, "http://")})
	require.NoError(t, err)
	return &weaviateRepository{client: client, collectionBaseName: "Weknora_embeddings"}
}

// The first batches written at a new dimension are saved by several workers at
// once, and each may find the class missing before any has created it. Only
// one create wins. Weaviate refuses the others with 422, in one of two wordings
// depending on whether they lose before or inside the Raft apply; neither may
// fail their writes. The barrier makes every worker see the class as missing,
// so the race happens every run.
func TestEnsureCollectionToleratesConcurrentCreation(t *testing.T) {
	const workers = 8
	var arrived atomic.Int32
	allProbed := make(chan struct{})
	server := &schemaTestServer{
		probe: func() {
			n := arrived.Add(1)
			if n == workers {
				close(allProbed)
			}
			if n > workers {
				return // a loser looking again after its 422
			}
			select {
			case <-allProbed:
			case <-time.After(5 * time.Second):
				t.Error("not every worker probed the class")
			}
		},
		create: func(attempt int) (int, string) {
			switch {
			case attempt == 1:
				return http.StatusOK, `{"class":"Weknora_embeddings_1024"}`
			case attempt%2 == 0:
				return http.StatusUnprocessableEntity,
					weaviateErrorBody("updating schema: TYPE_ADD_CLASS: class already exists")
			default:
				return http.StatusUnprocessableEntity,
					weaviateErrorBody("class name Weknora_embeddings_1024 already exists")
			}
		},
	}
	repo := server.repository(t)

	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- repo.ensureCollection(context.Background(), 1024)
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		assert.NoError(t, err, "a worker lost the creation race")
	}
	assert.Equal(t, workers, server.creates, "every worker should have attempted the create")
	_, ok := repo.initializedCollections.Load(1024)
	assert.True(t, ok, "dimension 1024 should be marked initialized")
}

// A 422 for any other reason leaves the class missing, so the write must fail
// rather than be recorded as initialized.
func TestEnsureCollectionFailsWhenTheCreateIsRejected(t *testing.T) {
	server := &schemaTestServer{create: func(int) (int, string) {
		return http.StatusUnprocessableEntity, weaviateErrorBody("invalid property: data type not supported")
	}}
	repo := server.repository(t)

	require.ErrorContains(t, repo.ensureCollection(context.Background(), 1024), "failed to create collection")
	assert.Equal(t, 2, server.probes, "the class should be looked up again after the 422")
	_, ok := repo.initializedCollections.Load(1024)
	assert.False(t, ok)
}

// Only a 422 means the create was refused; anything else is reported as it is.
func TestEnsureCollectionDoesNotRecheckOtherFailures(t *testing.T) {
	server := &schemaTestServer{create: func(int) (int, string) {
		return http.StatusInternalServerError, weaviateErrorBody("disk full")
	}}
	repo := server.repository(t)

	require.Error(t, repo.ensureCollection(context.Background(), 1024))
	assert.Equal(t, 1, server.probes)
}

// A misbehaving server can ignore the after cursor and replay the same page
// forever. The copy must detect the stuck cursor instead of hanging until the
// context times out, and the replayed page must not be written a second time.
func TestCopyIndicesStopsReplayingAStuckCursor(t *testing.T) {
	const objectsPerPage = 64
	page := make([]any, 0, objectsPerPage)
	sourceToTargetChunkIDMap := make(map[string]string, objectsPerPage)
	for i := range objectsPerPage {
		chunkID := fmt.Sprintf("chunk-%03d", i)
		page = append(page, map[string]any{
			fieldContent:         "payload",
			fieldSourceID:        chunkID,
			fieldSourceType:      float64(1),
			fieldChunkID:         chunkID,
			fieldKnowledgeID:     "doc-1",
			fieldKnowledgeBaseID: "source",
			fieldTagID:           "",
			"_additional": map[string]any{
				"id":     fmt.Sprintf("00000000-0000-0000-0000-%012d", i),
				"vector": []any{0.1, 0.2, 0.3},
			},
		})
		sourceToTargetChunkIDMap[chunkID] = fmt.Sprintf("target-%03d", i)
	}

	var mu sync.Mutex
	queries, afterSeen := 0, false
	var batches atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/graphql":
			mu.Lock()
			queries++
			mu.Unlock()
			var body struct {
				Query string `json:"query"`
			}
			if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&body)) {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if strings.Contains(body.Query, "after") {
				mu.Lock()
				afterSeen = true
				mu.Unlock()
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"Get": map[string]any{"Copy_3": page}},
			})
		case "/v1/batch/objects":
			batches.Add(1)
			_, _ = w.Write([]byte(`[]`))
		case "/v1/meta":
			_, _ = w.Write([]byte(`{"version":"1.30.0"}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	client, err := sdk.NewClient(sdk.Config{Scheme: "http", Host: strings.TrimPrefix(server.URL, "http://")})
	require.NoError(t, err)
	repo := &weaviateRepository{client: client, collectionBaseName: "Copy"}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.ErrorContains(t,
		repo.CopyIndices(ctx, "source", map[string]string{"doc-1": "doc-2"}, sourceToTargetChunkIDMap, "target", 3, ""),
		"no progress")
	assert.Equal(t, int32(1), batches.Load(), "the replayed page must not be written a second time")
	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, 2, queries, "the stuck page should be fetched once more and then rejected")
	assert.True(t, afterSeen, "the follow-up query should carry the after cursor")
}

// Weaviate answers a failed GraphQL query with HTTP 200, an errors array and no
// data; the SDK reports no Go error for that. The copy must surface it as an
// error instead of panicking on the nil Get data.
func TestCopyIndicesFailsOnGraphQLErrorsWithoutPanic(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/graphql":
			_, _ = w.Write([]byte(`{"errors":[{"message":"boom"}]}`))
		case "/v1/meta":
			_, _ = w.Write([]byte(`{"version":"1.30.0"}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	client, err := sdk.NewClient(sdk.Config{Scheme: "http", Host: strings.TrimPrefix(server.URL, "http://")})
	require.NoError(t, err)
	repo := &weaviateRepository{client: client, collectionBaseName: "Copy"}

	require.ErrorContains(t,
		repo.CopyIndices(context.Background(), "source",
			map[string]string{"doc-1": "doc-2"}, map[string]string{"chunk-000": "target-000"}, "target", 3, ""),
		"copy indices query failed")
}
