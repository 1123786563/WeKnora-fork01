package milvus

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/milvus-io/milvus-proto/go-api/v2/commonpb"
	"github.com/milvus-io/milvus-proto/go-api/v2/milvuspb"
	"github.com/milvus-io/milvus-proto/go-api/v2/schemapb"
	client "github.com/milvus-io/milvus/client/v2/milvusclient"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestBuildMilvusIndexResultsPreservesSearchScores(t *testing.T) {
	documents := []*MilvusVectorEmbeddingWithScore{
		{MilvusVectorEmbedding: MilvusVectorEmbedding{ID: "id-1", ChunkID: "chunk-1"}},
		{MilvusVectorEmbedding: MilvusVectorEmbedding{ID: "id-2", ChunkID: "chunk-2"}},
	}

	results, err := buildMilvusIndexResults(
		documents,
		[]float64{3.25, 0.75},
		types.MatchTypeKeywords,
	)

	require.NoError(t, err)
	require.Len(t, results, 2)
	require.Equal(t, types.MatchTypeKeywords, results[0].MatchType)
	require.Equal(t, 3.25, results[0].Score)
	require.Equal(t, 0.75, results[1].Score)
}

func TestBuildMilvusIndexResultsRejectsScoreCountMismatch(t *testing.T) {
	_, err := buildMilvusIndexResults(
		[]*MilvusVectorEmbeddingWithScore{
			{MilvusVectorEmbedding: MilvusVectorEmbedding{ID: "id-1"}},
		},
		nil,
		types.MatchTypeKeywords,
	)

	require.Error(t, err)
}

func TestUpdateChunkEnabledStatusInCollectionSkipsEmptyChunkIDs(t *testing.T) {
	repo := &milvusRepository{}

	require.NoError(t, repo.updateChunkEnabledStatusInCollection(
		context.Background(),
		"weknora_embeddings_1024",
		nil,
		false,
	))
	require.NoError(t, repo.updateChunkEnabledStatusInCollection(
		context.Background(),
		"weknora_embeddings_1024",
		[]string{},
		true,
	))
}

func TestUpdateChunkEnabledStatusInCollectionsPropagatesFailure(t *testing.T) {
	wantErr := errors.New("upsert failed")
	err := updateChunkEnabledStatusInCollections(
		context.Background(),
		[]string{"other_collection", "weknora_embeddings_1024"},
		"weknora_embeddings",
		nil,
		[]string{"chunk-1"},
		func(_ context.Context, collection string, _ []string, enabled bool) error {
			if collection == "weknora_embeddings_1024" && !enabled {
				return wantErr
			}
			return nil
		},
	)
	require.ErrorIs(t, err, wantErr)
}

func TestUpdateChunkEnabledStatusInCollectionsIgnoresExtendedPrefix(t *testing.T) {
	var seen []string
	seenSet := map[string]bool{}
	err := updateChunkEnabledStatusInCollections(
		context.Background(),
		[]string{
			"other_collection",
			"weknora_embeddings_1024",
			"weknora_embeddings_multilingual_1024",
			"weknora_embeddings_1024_backup",
		},
		"weknora_embeddings",
		[]string{"chunk-1"},
		nil,
		func(_ context.Context, collection string, _ []string, _ bool) error {
			if !seenSet[collection] {
				seenSet[collection] = true
				seen = append(seen, collection)
			}
			return nil
		},
	)
	require.NoError(t, err)
	require.Equal(t, []string{"weknora_embeddings_1024"}, seen)
}

// newInterceptedMilvusClient builds a Milvus client whose unary RPCs are
// answered by the given handler, so repository behaviour can be pinned
// without a live Milvus server.
//
// The Milvus client dials with grpc.WithBlock, so an empty local gRPC server
// is served just to let the dial reach Ready; every RPC is then answered by
// the interceptor and never reaches the server.
func newInterceptedMilvusClient(t *testing.T, handler func(method string, req, reply any) error) *client.Client {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := grpc.NewServer()
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)

	c, err := client.New(context.Background(), &client.ClientConfig{
		Address:     listener.Addr().String(),
		DisableConn: true,
		DialOptions: []grpc.DialOption{grpc.WithChainUnaryInterceptor(func(
			_ context.Context, method string, req, reply any, _ *grpc.ClientConn,
			_ grpc.UnaryInvoker, _ ...grpc.CallOption,
		) error {
			return handler(method, req, reply)
		})},
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, c.Close(context.Background()))
	})
	return c
}

// milvusTestSchema describes a legacy (pre-multilingual) collection with just
// the fields the keyword search result mapping consumes.
func milvusTestSchema(name string) *schemapb.CollectionSchema {
	return &schemapb.CollectionSchema{
		Name: name,
		Fields: []*schemapb.FieldSchema{
			{Name: fieldID, DataType: schemapb.DataType_VarChar, IsPrimaryKey: true},
			{Name: fieldContent, DataType: schemapb.DataType_VarChar},
			{Name: fieldChunkID, DataType: schemapb.DataType_VarChar},
		},
	}
}

type milvusKeywordHit struct {
	id      string
	chunkID string
	content string
	score   float32
}

func milvusTestSearchResults(hits []milvusKeywordHit) *schemapb.SearchResultData {
	ids := make([]string, 0, len(hits))
	contents := make([]string, 0, len(hits))
	chunkIDs := make([]string, 0, len(hits))
	scores := make([]float32, 0, len(hits))
	for _, hit := range hits {
		ids = append(ids, hit.id)
		contents = append(contents, hit.content)
		chunkIDs = append(chunkIDs, hit.chunkID)
		scores = append(scores, hit.score)
	}
	return &schemapb.SearchResultData{
		NumQueries: 1,
		TopK:       int64(len(hits)),
		Topks:      []int64{int64(len(hits))},
		Scores:     scores,
		Ids: &schemapb.IDs{
			IdField: &schemapb.IDs_StrId{StrId: &schemapb.StringArray{Data: ids}},
		},
		FieldsData: []*schemapb.FieldData{
			milvusVarCharFieldData(fieldID, ids),
			milvusVarCharFieldData(fieldContent, contents),
			milvusVarCharFieldData(fieldChunkID, chunkIDs),
		},
	}
}

func milvusVarCharFieldData(name string, values []string) *schemapb.FieldData {
	return &schemapb.FieldData{
		Type:      schemapb.DataType_VarChar,
		FieldName: name,
		Field: &schemapb.FieldData_Scalars{
			Scalars: &schemapb.ScalarField{
				Data: &schemapb.ScalarField_StringData{
					StringData: &schemapb.StringArray{Data: values},
				},
			},
		},
	}
}

// newKeywordsTestRepository fakes the RPCs KeywordsRetrieve issues:
// ShowCollections lists `collections`, DescribeCollection serves a legacy
// schema, and every BM25 Search goes through `search`.
func newKeywordsTestRepository(t *testing.T, collections []string,
	search func(*milvuspb.SearchRequest) ([]milvusKeywordHit, error),
) *milvusRepository {
	t.Helper()
	c := newInterceptedMilvusClient(t, func(method string, req, reply any) error {
		switch request := req.(type) {
		case *milvuspb.ShowCollectionsRequest:
			reply.(*milvuspb.ShowCollectionsResponse).Status = &commonpb.Status{ErrorCode: commonpb.ErrorCode_Success}
			reply.(*milvuspb.ShowCollectionsResponse).CollectionNames = collections
			return nil
		case *milvuspb.DescribeCollectionRequest:
			reply.(*milvuspb.DescribeCollectionResponse).Status = &commonpb.Status{ErrorCode: commonpb.ErrorCode_Success}
			reply.(*milvuspb.DescribeCollectionResponse).Schema = milvusTestSchema(request.GetCollectionName())
			return nil
		case *milvuspb.SearchRequest:
			hits, err := search(request)
			if err != nil {
				return err
			}
			reply.(*milvuspb.SearchResults).Status = &commonpb.Status{ErrorCode: commonpb.ErrorCode_Success}
			reply.(*milvuspb.SearchResults).Results = milvusTestSearchResults(hits)
			return nil
		default:
			return fmt.Errorf("unexpected RPC %s", method)
		}
	})
	return &milvusRepository{client: c, collectionBaseName: "vectors"}
}

// When every attempted collection fails to answer the keyword query, the
// retrieval must return an error instead of empty results that read as "the
// library has no such content" (#3835).
func TestKeywordsRetrieveAllCollectionsFailed(t *testing.T) {
	var searched []string
	repo := newKeywordsTestRepository(t, []string{"vectors_768", "vectors_1536", "unrelated_768"},
		func(request *milvuspb.SearchRequest) ([]milvusKeywordHit, error) {
			searched = append(searched, request.GetCollectionName())
			return nil, fmt.Errorf("milvus unavailable in %s", request.GetCollectionName())
		})

	results, err := repo.KeywordsRetrieve(context.Background(), types.RetrieveParams{
		Query: "hello world",
		TopK:  10,
	})

	require.Error(t, err)
	require.Nil(t, results)
	require.Contains(t, err.Error(), "all 2 milvus collections")
	require.Contains(t, err.Error(), "vectors_768")
	require.Contains(t, err.Error(), "milvus unavailable")
	require.Equal(t, []string{"vectors_768", "vectors_1536"}, searched)
}

// One collection failing must not negate matches found in the healthy ones.
func TestKeywordsRetrievePartialFailureKeepsResults(t *testing.T) {
	repo := newKeywordsTestRepository(t, []string{"vectors_768", "vectors_1536"},
		func(request *milvuspb.SearchRequest) ([]milvusKeywordHit, error) {
			if request.GetCollectionName() == "vectors_768" {
				return nil, errors.New("search timeout")
			}
			return []milvusKeywordHit{{
				id:      "point-1",
				chunkID: "chunk-1",
				content: "hello world",
				score:   3.25,
			}}, nil
		})

	results, err := repo.KeywordsRetrieve(context.Background(), types.RetrieveParams{
		Query: "hello world",
		TopK:  10,
	})

	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, types.KeywordsRetrieverType, results[0].RetrieverType)
	require.Equal(t, types.MilvusRetrieverEngineType, results[0].RetrieverEngineType)
	require.Len(t, results[0].Results, 1)
	require.Equal(t, "chunk-1", results[0].Results[0].ChunkID)
	require.Equal(t, 3.25, results[0].Results[0].Score)
}

func TestKeywordsRetrieveAllCollectionsSucceed(t *testing.T) {
	repo := newKeywordsTestRepository(t, []string{"vectors_768", "vectors_1536"},
		func(request *milvuspb.SearchRequest) ([]milvusKeywordHit, error) {
			return []milvusKeywordHit{{
				id:      "point-" + request.GetCollectionName(),
				chunkID: "chunk-" + request.GetCollectionName(),
				content: "hello world",
				score:   1.5,
			}}, nil
		})

	results, err := repo.KeywordsRetrieve(context.Background(), types.RetrieveParams{
		Query: "hello world",
		TopK:  10,
	})

	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Len(t, results[0].Results, 2)
}

// A store that never created a dimension collection has nothing to search:
// that is a legitimate "no matches", not a failure.
func TestKeywordsRetrieveNoMatchingCollectionsIsEmpty(t *testing.T) {
	repo := newKeywordsTestRepository(t, []string{"unrelated_768"},
		func(*milvuspb.SearchRequest) ([]milvusKeywordHit, error) {
			t.Error("no matching collections must not be searched")
			return nil, nil
		})

	results, err := repo.KeywordsRetrieve(context.Background(), types.RetrieveParams{
		Query: "hello world",
		TopK:  10,
	})

	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Empty(t, results[0].Results)
}
