package qdrant

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/qdrant/go-client/qdrant"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestNewQdrantValueMapSanitizesInvalidUTF8AndNUL(t *testing.T) {
	malformed := "prefix" + string([]byte{0xff}) + "\x00suffix"
	payload := newQdrantValueMap(map[string]any{
		fieldContent:    malformed,
		fieldSourceType: int64(1),
		fieldIsEnabled:  true,
	})

	got := payload[fieldContent].GetStringValue()
	if got != "prefixsuffix" {
		t.Fatalf("unexpected sanitized content: %q", got)
	}
	if !utf8.ValidString(got) {
		t.Fatalf("sanitized content is not valid UTF-8: % x", []byte(got))
	}
	if gotSourceType := payload[fieldSourceType].GetIntegerValue(); gotSourceType != 1 {
		t.Fatalf("source type changed: got %d, want 1", gotSourceType)
	}
	if gotEnabled := payload[fieldIsEnabled].GetBoolValue(); !gotEnabled {
		t.Fatal("is_enabled changed during payload sanitization")
	}
}

func TestNewQdrantValueMapPreservesValidUTF8(t *testing.T) {
	valid := "valid UTF-8 中文内容"
	payload := newQdrantValueMap(map[string]any{
		fieldContent: valid,
	})

	got := payload[fieldContent].GetStringValue()
	if got != valid {
		t.Fatalf("valid content changed: got %q, want %q", got, valid)
	}
}

func TestCreatePayloadSanitizesAllStringFields(t *testing.T) {
	malformed := "a" + string([]byte{0xff}) + "\x00b"
	embedding := &QdrantVectorEmbedding{
		Content:         malformed,
		SourceID:        malformed,
		SourceType:      2,
		ChunkID:         malformed,
		KnowledgeID:     malformed,
		KnowledgeBaseID: malformed,
		TagID:           malformed,
		IsEnabled:       true,
	}

	payload := createPayload(embedding)
	stringFields := []string{
		fieldContent,
		fieldSourceID,
		fieldChunkID,
		fieldKnowledgeID,
		fieldKnowledgeBaseID,
		fieldTagID,
	}
	for _, field := range stringFields {
		got := payload[field].GetStringValue()
		if got != "ab" {
			t.Errorf("%s was not sanitized correctly: got %q", field, got)
		}
		if !utf8.ValidString(got) {
			t.Errorf("%s remains invalid UTF-8: % x", field, []byte(got))
		}
	}
	if got := payload[fieldSourceType].GetIntegerValue(); got != 2 {
		t.Errorf("source type changed: got %d, want 2", got)
	}
	if got := payload[fieldIsEnabled].GetBoolValue(); !got {
		t.Error("is_enabled changed during payload creation")
	}
}

// newInterceptedQdrantClient builds a client whose unary RPCs are answered by
// the given interceptor, so repository behaviour can be pinned without a live
// Qdrant server.
func newInterceptedQdrantClient(t *testing.T, intercept grpc.UnaryClientInterceptor) *qdrant.Client {
	t.Helper()
	client, err := qdrant.NewClient(&qdrant.Config{
		Host:                   "localhost",
		PoolSize:               1,
		SkipCompatibilityCheck: true,
		GrpcOptions:            []grpc.DialOption{grpc.WithUnaryInterceptor(intercept)},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Error(err)
		}
	})
	return client
}

// Intercept the SDK's RPCs so failures and cancellation need no live server.
func newPayloadUpdateTestRepository(t *testing.T, collections []string, listErr error,
	setPayload func(context.Context, *qdrant.SetPayloadPoints) error,
) *qdrantRepository {
	t.Helper()
	client := newInterceptedQdrantClient(t, func(
		ctx context.Context, method string, req, reply any, _ *grpc.ClientConn,
		_ grpc.UnaryInvoker, _ ...grpc.CallOption,
	) error {
		switch request := req.(type) {
		case *qdrant.ListCollectionsRequest:
			if listErr != nil {
				return listErr
			}
			response := reply.(*qdrant.ListCollectionsResponse)
			for _, name := range collections {
				response.Collections = append(response.Collections, &qdrant.CollectionDescription{Name: name})
			}
			return nil
		case *qdrant.SetPayloadPoints:
			return setPayload(ctx, request)
		default:
			return fmt.Errorf("unexpected RPC %s", method)
		}
	})
	return &qdrantRepository{client: client, collectionBaseName: "vectors"}
}

func TestBatchPayloadUpdates(t *testing.T) {
	operations := []struct {
		name   string
		groups int
		run    func(context.Context, *qdrantRepository) error
	}{
		{"enable", 1, func(ctx context.Context, repo *qdrantRepository) error {
			return repo.BatchUpdateChunkEnabledStatus(ctx, map[string]bool{"chunk": true})
		}},
		{"disable", 1, func(ctx context.Context, repo *qdrantRepository) error {
			return repo.BatchUpdateChunkEnabledStatus(ctx, map[string]bool{"chunk": false})
		}},
		{"mixed status", 2, func(ctx context.Context, repo *qdrantRepository) error {
			return repo.BatchUpdateChunkEnabledStatus(ctx, map[string]bool{"on": true, "off": false})
		}},
		{"tags", 2, func(ctx context.Context, repo *qdrantRepository) error {
			return repo.BatchUpdateChunkTagID(ctx, map[string]string{"one": "tag-a", "two": "tag-b"})
		}},
	}
	for _, operation := range operations {
		t.Run(operation.name, func(t *testing.T) {
			modes := []string{"success", "partial failure", "all failures", "list failure", "no collections"}
			for _, mode := range modes {
				t.Run(mode, func(t *testing.T) {
					collections := []string{"vectors_768", "unrelated_768", "vectors_1536"}
					if mode == "no collections" {
						collections = []string{"unrelated_768"}
					}
					var listErr error
					if mode == "list failure" {
						listErr = errors.New("list unavailable")
					}
					calls := 0
					var failures []error
					var descriptions []string
					repo := newPayloadUpdateTestRepository(t, collections, listErr,
						func(_ context.Context, request *qdrant.SetPayloadPoints) error {
							calls++
							if request.CollectionName == "unrelated_768" {
								t.Error("updated an unrelated collection")
							}
							fail := mode == "all failures" ||
								(mode == "partial failure" && request.CollectionName == "vectors_768")
							if !fail {
								return nil
							}
							failure := fmt.Errorf("failure %d", calls)
							failures = append(failures, failure)
							description := fmt.Sprintf("set chunk tag_id %q",
								request.Payload[fieldTagID].GetStringValue())
							if enabled, ok := request.Payload[fieldIsEnabled]; ok {
								description = "disable chunks"
								if enabled.GetBoolValue() {
									description = "enable chunks"
								}
							}
							descriptions = append(descriptions, description+" in collection "+request.CollectionName)
							return failure
						})
					err := operation.run(context.Background(), repo)
					wantCalls := 2 * operation.groups
					if mode == "list failure" || mode == "no collections" {
						wantCalls = 0
					}
					if calls != wantCalls {
						t.Fatalf("got %d updates, want %d", calls, wantCalls)
					}
					if listErr != nil && !errors.Is(err, listErr) {
						t.Fatalf("lost list failure: %v", err)
					}
					if listErr == nil && len(failures) == 0 && err != nil {
						t.Fatalf("unexpected error: %v", err)
					}
					for i, failure := range failures {
						if !errors.Is(err, failure) {
							t.Errorf("lost failure %v in %v", failure, err)
						}
						if err == nil || !strings.Contains(err.Error(), descriptions[i]) {
							t.Errorf("missing operation/collection %q in %v", descriptions[i], err)
						}
					}
				})
			}

			t.Run("cancellation retains earlier failures", func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				failure := errors.New("first update failed")
				calls := 0
				repo := newPayloadUpdateTestRepository(t, []string{"vectors_768", "vectors_1536", "vectors_384"}, nil,
					func(context.Context, *qdrant.SetPayloadPoints) error {
						calls++
						if calls == 1 {
							return failure
						}
						cancel()
						return ctx.Err()
					})
				err := operation.run(ctx, repo)
				if calls != 2 || !errors.Is(err, failure) || !errors.Is(err, context.Canceled) {
					t.Fatalf("calls=%d, err=%v; want 2 calls and both errors", calls, err)
				}
			})

			t.Run("canceled before starting", func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				if err := operation.run(ctx, &qdrantRepository{}); !errors.Is(err, context.Canceled) {
					t.Fatalf("expected cancellation before any RPC, got %v", err)
				}
			})
			t.Run("canceled during final update", func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				calls := 0
				repo := newPayloadUpdateTestRepository(t, []string{"vectors_768"}, nil,
					func(context.Context, *qdrant.SetPayloadPoints) error {
						calls++
						if calls == operation.groups {
							cancel()
						}
						return nil
					})
				if err := operation.run(ctx, repo); !errors.Is(err, context.Canceled) {
					t.Fatalf("expected cancellation instead of success, got %v", err)
				}
			})
			t.Run("expired deadline", func(t *testing.T) {
				ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
				defer cancel()
				if err := operation.run(ctx, &qdrantRepository{}); !errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("expected deadline error before any RPC, got %v", err)
				}
			})
		})
	}
}

func TestBatchPayloadUpdatesEmptyInput(t *testing.T) {
	repo := &qdrantRepository{}
	if err := repo.BatchUpdateChunkEnabledStatus(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if err := repo.BatchUpdateChunkTagID(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
}

// deleteTestRepository is a stand-in Qdrant that only holds `collections` and
// answers like the real server: a delete against a collection it does not have
// fails with the same "Collection ... doesn't exist!" error the issue reports.
// `probe` optionally overrides the CollectionExists answer, e.g. to inject an
// infrastructure failure; a nil probe answers from the inventory.
const qdrantMissingCollectionErr = "Collection %s doesn't exist!"

type deleteTestRepository struct {
	repo        *qdrantRepository
	collections map[string]bool
	probe       func(collectionName string) (bool, error)
	probed      []string
	deletes     []string
}

func newDeleteTestRepository(t *testing.T, collections []string,
	probe func(collectionName string) (bool, error),
) *deleteTestRepository {
	t.Helper()
	harness := &deleteTestRepository{
		collections: make(map[string]bool, len(collections)),
		probe:       probe,
	}
	for _, name := range collections {
		harness.collections[name] = true
	}
	client := newInterceptedQdrantClient(t, func(
		_ context.Context, method string, req, reply any, _ *grpc.ClientConn,
		_ grpc.UnaryInvoker, _ ...grpc.CallOption,
	) error {
		switch request := req.(type) {
		case *qdrant.CollectionExistsRequest:
			name := request.GetCollectionName()
			harness.probed = append(harness.probed, name)
			if harness.probe == nil {
				response := reply.(*qdrant.CollectionExistsResponse)
				response.Result = &qdrant.CollectionExists{Exists: harness.collections[name]}
				return nil
			}
			found, err := harness.probe(name)
			if err != nil {
				return err
			}
			reply.(*qdrant.CollectionExistsResponse).Result = &qdrant.CollectionExists{Exists: found}
			return nil
		case *qdrant.DeletePoints:
			name := request.GetCollectionName()
			harness.deletes = append(harness.deletes, name)
			if !harness.collections[name] {
				// Same gRPC status and wording the real server uses, so the
				// NotFound no-op path is exercised rather than a paraphrase.
				return status.Error(codes.NotFound, fmt.Sprintf(qdrantMissingCollectionErr, name))
			}
			return nil
		default:
			return fmt.Errorf("unexpected RPC %s", method)
		}
	})
	harness.repo = &qdrantRepository{client: client, collectionBaseName: "vectors"}
	return harness
}

// deleteOperations covers every repository method that deletes by a dimension,
// so the missing-collection rule is pinned for all three of them.
func deleteOperations() []struct {
	name string
	run  func(context.Context, *qdrantRepository) error
} {
	return []struct {
		name string
		run  func(context.Context, *qdrantRepository) error
	}{
		{"chunk ids", func(ctx context.Context, repo *qdrantRepository) error {
			return repo.DeleteByChunkIDList(ctx, []string{"chunk-1"}, 1024, types.KnowledgeBaseTypeDocument)
		}},
		{"knowledge ids", func(ctx context.Context, repo *qdrantRepository) error {
			return repo.DeleteByKnowledgeIDList(ctx, []string{"knowledge-1"}, 1024, types.KnowledgeBaseTypeDocument)
		}},
		{"source ids", func(ctx context.Context, repo *qdrantRepository) error {
			return repo.DeleteBySourceIDList(ctx, []string{"source-1"}, 1024, types.KnowledgeBaseTypeDocument)
		}},
	}
}

// A dimension collection is created lazily on the first write, so deleting from
// one that was never created must be a no-op rather than an error: ingest and
// re-index run "delete then write", and failing here aborts the write that would
// have created the collection (#3337).
func TestDeletesSkipMissingCollection(t *testing.T) {
	for _, operation := range deleteOperations() {
		t.Run(operation.name, func(t *testing.T) {
			// The store holds no dimension collection yet: the delete must not
			// reach Qdrant, otherwise it reports the collection as missing.
			harness := newDeleteTestRepository(t, nil, nil)

			if err := operation.run(context.Background(), harness.repo); err != nil {
				t.Fatalf("delete against a missing collection must be a no-op, got %v", err)
			}
			if len(harness.probed) != 1 || harness.probed[0] != "vectors_1024" {
				t.Fatalf("expected one existence probe for vectors_1024, got %v", harness.probed)
			}
			if len(harness.deletes) != 0 {
				t.Fatalf("expected no delete RPC for a missing collection, got %v", harness.deletes)
			}
		})
	}
}

func TestDeletesIssueWhenCollectionExists(t *testing.T) {
	for _, operation := range deleteOperations() {
		t.Run(operation.name, func(t *testing.T) {
			harness := newDeleteTestRepository(t, []string{"vectors_1024"}, nil)

			if err := operation.run(context.Background(), harness.repo); err != nil {
				t.Fatal(err)
			}
			if len(harness.deletes) != 1 || harness.deletes[0] != "vectors_1024" {
				t.Fatalf("expected one delete against vectors_1024, got %v", harness.deletes)
			}
		})
	}
}

// Infrastructure failures must not be mistaken for an absent collection: a
// store that cannot answer must fail the delete loudly.
func TestDeletePropagatesCollectionCheckFailure(t *testing.T) {
	for _, operation := range deleteOperations() {
		t.Run(operation.name, func(t *testing.T) {
			harness := newDeleteTestRepository(t, []string{"vectors_1024"}, func(string) (bool, error) {
				return false, errors.New("qdrant unavailable")
			})

			err := operation.run(context.Background(), harness.repo)
			if err == nil || !strings.Contains(err.Error(), "failed to check collection existence") {
				t.Fatalf("expected the existence probe failure to surface, got %v", err)
			}
			if len(harness.deletes) != 0 {
				t.Fatalf("expected no delete RPC after a failed probe, got %v", harness.deletes)
			}
		})
	}
}

// A dimension this process already created needs no probe, which keeps the
// steady-state delete at one round-trip.
func TestDeleteSkipsProbeForInitializedDimension(t *testing.T) {
	harness := newDeleteTestRepository(t, []string{"vectors_1024"}, func(string) (bool, error) {
		t.Error("initialized dimensions must not be probed")
		return false, nil
	})
	harness.repo.initializedCollections.Store(1024, true)

	if err := harness.repo.DeleteByKnowledgeIDList(
		context.Background(), []string{"knowledge-1"}, 1024, types.KnowledgeBaseTypeDocument,
	); err != nil {
		t.Fatal(err)
	}
	if len(harness.deletes) != 1 || harness.deletes[0] != "vectors_1024" {
		t.Fatalf("expected one delete against vectors_1024, got %v", harness.deletes)
	}
}

// An empty ID list short-circuits before any RPC, including the new probe.
func TestDeleteWithEmptyIDListSkipsAllRPCs(t *testing.T) {
	harness := newDeleteTestRepository(t, []string{"vectors_1024"}, func(string) (bool, error) {
		t.Error("empty deletes must not probe the collection")
		return false, nil
	})

	ctx := context.Background()
	if err := harness.repo.DeleteByChunkIDList(ctx, nil, 1024, types.KnowledgeBaseTypeDocument); err != nil {
		t.Fatal(err)
	}
	if err := harness.repo.DeleteByKnowledgeIDList(ctx, nil, 1024, types.KnowledgeBaseTypeDocument); err != nil {
		t.Fatal(err)
	}
	if err := harness.repo.DeleteBySourceIDList(ctx, nil, 1024, types.KnowledgeBaseTypeDocument); err != nil {
		t.Fatal(err)
	}
	if len(harness.deletes) != 0 {
		t.Fatalf("expected no delete RPC for an empty ID list, got %v", harness.deletes)
	}
}

func TestIsMissingCollectionErr(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"generic", errors.New("qdrant unavailable"), false},
		{"issue wording", fmt.Errorf(qdrantMissingCollectionErr, "vectors_1024"), true},
		{"grpc not found", status.Error(codes.NotFound, "Not found: Collection vectors_1024 doesn't exist!"), true},
		{"wrapped not found", fmt.Errorf("Delete() failed: vectors_1024: %w", status.Error(codes.NotFound, "Not found")), true},
		{"other grpc", status.Error(codes.Unavailable, "qdrant down"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isMissingCollectionErr(tc.err); got != tc.want {
				t.Fatalf("isMissingCollectionErr(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

// A dimension this process already created is not probed, so a Qdrant wipe
// that leaves the cache stale still reaches DeletePoints. That RPC's missing
// collection must be a no-op and must drop the cache so the following write
// can recreate the collection (#3337).
func TestDeleteTreatsStaleCacheMissingCollectionAsNoOp(t *testing.T) {
	for _, operation := range deleteOperations() {
		t.Run(operation.name, func(t *testing.T) {
			harness := newDeleteTestRepository(t, nil, func(string) (bool, error) {
				t.Error("stale cache must skip the existence probe")
				return false, nil
			})
			harness.repo.initializedCollections.Store(1024, true)

			if err := operation.run(context.Background(), harness.repo); err != nil {
				t.Fatalf("stale cache + missing collection must be a no-op, got %v", err)
			}
			if len(harness.deletes) != 1 || harness.deletes[0] != "vectors_1024" {
				t.Fatalf("expected one delete against vectors_1024, got %v", harness.deletes)
			}
			if _, ok := harness.repo.initializedCollections.Load(1024); ok {
				t.Fatal("missing collection must drop the initialized cache so the following write can recreate it")
			}
		})
	}
}

// scrollFake is a stand-in Qdrant that answers CopyIndices' RPCs. Scroll
// pagination follows the real server's contract: the response's
// next_page_offset is the cursor to resume from while matching points remain
// and is nil on the last page. Upserted points are recorded so tests can
// assert exactly what reached the target collection.
type scrollFake struct {
	repo     *qdrantRepository
	points   []*qdrant.RetrievedPoint // ordered by point ID, like the server's scan
	limits   []int                    // optional per-scroll page limits
	stuck    bool                     // always answer with the first page and its cursor
	scrolls  int
	upserted []*qdrant.PointStruct
}

func newScrollFake(t *testing.T, points []*qdrant.RetrievedPoint) *scrollFake {
	t.Helper()
	harness := &scrollFake{points: points}
	client := newInterceptedQdrantClient(t, func(
		_ context.Context, method string, req, reply any, _ *grpc.ClientConn,
		_ grpc.UnaryInvoker, _ ...grpc.CallOption,
	) error {
		switch request := req.(type) {
		case *qdrant.CollectionExistsRequest:
			reply.(*qdrant.CollectionExistsResponse).Result = &qdrant.CollectionExists{Exists: true}
			return nil
		case *qdrant.ScrollPoints:
			response := reply.(*qdrant.ScrollResponse)
			response.Result, response.NextPageOffset = harness.scroll(request)
			return nil
		case *qdrant.UpsertPoints:
			harness.upserted = append(harness.upserted, request.Points...)
			return nil
		default:
			return fmt.Errorf("unexpected RPC %s", method)
		}
	})
	harness.repo = &qdrantRepository{client: client, collectionBaseName: "vectors"}
	return harness
}

// scroll answers one Scroll request: the points after the request cursor, at
// most limit of them, plus the cursor to continue from while matches remain.
func (f *scrollFake) scroll(request *qdrant.ScrollPoints) ([]*qdrant.RetrievedPoint, *qdrant.PointId) {
	f.scrolls++

	limit := int(request.GetLimit())
	if f.stuck {
		page := f.points[:limit]
		return page, page[len(page)-1].Id
	}
	if f.scrolls <= len(f.limits) {
		limit = f.limits[f.scrolls-1]
	}

	start := 0
	if cursor := request.GetOffset().GetNum(); cursor != 0 {
		start = sort.Search(len(f.points), func(i int) bool {
			return f.points[i].Id.GetNum() > cursor
		})
	}

	end := start + limit
	if end > len(f.points) {
		end = len(f.points)
	}
	page := f.points[start:end]
	if end >= len(f.points) {
		return page, nil
	}
	return page, page[len(page)-1].Id
}

// newScrollSourcePoints builds a source collection of numbered points whose
// payloads CopyIndices can map (chunk IDs unique, SourceID == ChunkID).
func newScrollSourcePoints(count int) []*qdrant.RetrievedPoint {
	points := make([]*qdrant.RetrievedPoint, 0, count)
	for i := 1; i <= count; i++ {
		chunkID := fmt.Sprintf("chunk-%03d", i)
		points = append(points, &qdrant.RetrievedPoint{
			Id: qdrant.NewIDNum(uint64(i)),
			Payload: newQdrantValueMap(map[string]any{
				fieldContent:         "content " + chunkID,
				fieldSourceID:        chunkID,
				fieldSourceType:      int64(0),
				fieldChunkID:         chunkID,
				fieldKnowledgeID:     "knowledge-1",
				fieldKnowledgeBaseID: "kb-source",
				fieldTagID:           "",
				fieldIsEnabled:       true,
			}),
			Vectors: &qdrant.VectorsOutput{
				VectorsOptions: &qdrant.VectorsOutput_Vector{
					Vector: &qdrant.VectorOutput{
						Vector: &qdrant.VectorOutput_Dense{
							Dense: &qdrant.DenseVector{Data: []float32{0.1, 0.2, 0.3}},
						},
					},
				},
			},
		})
	}
	return points
}

// newCopyIndicesMaps builds one-to-one knowledge and chunk mappings.
func newCopyIndicesMaps(count int) (map[string]string, map[string]string) {
	sourceToTargetKBID := map[string]string{"knowledge-1": "knowledge-2"}
	sourceToTargetChunkID := make(map[string]string, count)
	for i := 1; i <= count; i++ {
		sourceToTargetChunkID[fmt.Sprintf("chunk-%03d", i)] = fmt.Sprintf("target-chunk-%03d", i)
	}
	return sourceToTargetKBID, sourceToTargetChunkID
}

func runCopyIndices(t *testing.T, repo *qdrantRepository, count int) error {
	t.Helper()
	sourceToTargetKBID, sourceToTargetChunkID := newCopyIndicesMaps(count)
	return repo.CopyIndices(context.Background(),
		"kb-source", sourceToTargetKBID, sourceToTargetChunkID, "kb-target",
		3, types.KnowledgeBaseTypeDocument)
}

// #3870: the scan must follow the server's next_page_offset cursor and stop
// on its nil, so 150 points paged at 64 upsert exactly 150 unique points with
// no page-boundary duplicates.
func TestCopyIndicesFollowsServerPageOffsets(t *testing.T) {
	const sourceCount = 150
	harness := newScrollFake(t, newScrollSourcePoints(sourceCount))

	if err := runCopyIndices(t, harness.repo, sourceCount); err != nil {
		t.Fatal(err)
	}

	if harness.scrolls != 3 {
		t.Fatalf("got %d scroll requests, want 3 (64+64+22 pages)", harness.scrolls)
	}
	if len(harness.upserted) != sourceCount {
		t.Fatalf("got %d upserted points, want exactly %d", len(harness.upserted), sourceCount)
	}

	seenChunks := make(map[string]bool, sourceCount)
	for _, point := range harness.upserted {
		chunkID := point.Payload[fieldChunkID].GetStringValue()
		if !strings.HasPrefix(chunkID, "target-chunk-") {
			t.Fatalf("chunk %q was not mapped to its target ID", chunkID)
		}
		if seenChunks[chunkID] {
			t.Fatalf("chunk %q was upserted more than once", chunkID)
		}
		seenChunks[chunkID] = true
		if got := point.Payload[fieldSourceID].GetStringValue(); got != chunkID {
			t.Fatalf("source ID %q does not follow the mapped chunk ID %q", got, chunkID)
		}
		if got := point.Payload[fieldKnowledgeBaseID].GetStringValue(); got != "kb-target" {
			t.Fatalf("chunk %q landed in knowledge base %q, want kb-target", chunkID, got)
		}
	}
	if len(seenChunks) != sourceCount {
		t.Fatalf("got %d unique target chunks, want %d", len(seenChunks), sourceCount)
	}
}

// #3870: a server that keeps answering with the same page and the same
// next_page_offset must fail with a no-progress error instead of copying the
// page forever.
func TestCopyIndicesStuckPageOffsetFailsLoudly(t *testing.T) {
	const sourceCount = 150
	harness := newScrollFake(t, newScrollSourcePoints(sourceCount))
	harness.stuck = true

	err := runCopyIndices(t, harness.repo, sourceCount)
	if err == nil || !strings.Contains(err.Error(), "no progress") {
		t.Fatalf("expected a no-progress error, got %v", err)
	}
	if harness.scrolls > 3 {
		t.Fatalf("pagination did not stop promptly: %d scroll requests", harness.scrolls)
	}
}

// #3870: a page shorter than the batch size is not the end of the scan; only
// the server's nil cursor is. The removed count-based termination silently
// truncated such copies.
func TestCopyIndicesContinuesPastShortPage(t *testing.T) {
	const sourceCount = 150
	harness := newScrollFake(t, newScrollSourcePoints(sourceCount))
	harness.limits = []int{64, 40} // short second page, more points remain

	if err := runCopyIndices(t, harness.repo, sourceCount); err != nil {
		t.Fatal(err)
	}
	if len(harness.upserted) != sourceCount {
		t.Fatalf("got %d upserted points, want all %d after a short page", len(harness.upserted), sourceCount)
	}
}
