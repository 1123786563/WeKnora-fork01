package milvus

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/milvus-io/milvus-proto/go-api/v2/commonpb"
	"github.com/milvus-io/milvus-proto/go-api/v2/milvuspb"
	"github.com/milvus-io/milvus-proto/go-api/v2/schemapb"
	"github.com/milvus-io/milvus/client/v2/entity"
	client "github.com/milvus-io/milvus/client/v2/milvusclient"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/Tencent/WeKnora/internal/types"
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

// copyIndicesFakeServer is an in-process MilvusServiceServer backing a real
// milvusclient over gRPC, so CopyIndices exercises its actual Query and Upsert
// request paths. It reproduces the server behaviour the copy depends on: Query
// honours the knowledge_base_id / chunk_id template values and the offset /
// limit query params, and result order is free to drift between calls because
// Milvus does not guarantee query order.
type copyIndicesFakeServer struct {
	milvuspb.UnimplementedMilvusServiceServer

	mu            sync.Mutex
	rows          []*MilvusVectorEmbedding
	dimension     int
	rotateResults bool
	queries       int
	queryParams   []string
	upserted      []*MilvusVectorEmbedding
}

func (s *copyIndicesFakeServer) DescribeCollection(
	_ context.Context, req *milvuspb.DescribeCollectionRequest,
) (*milvuspb.DescribeCollectionResponse, error) {
	return &milvuspb.DescribeCollectionResponse{
		Status: &commonpb.Status{},
		Schema: &schemapb.CollectionSchema{
			Name: req.GetCollectionName(),
			Fields: []*schemapb.FieldSchema{
				{Name: fieldID, DataType: schemapb.DataType_VarChar, IsPrimaryKey: true},
				{Name: fieldEmbedding, DataType: schemapb.DataType_FloatVector, TypeParams: []*commonpb.KeyValuePair{
					{Key: entity.TypeParamDim, Value: strconv.Itoa(s.dimension)},
				}},
				{Name: fieldContent, DataType: schemapb.DataType_VarChar},
				{Name: fieldSourceID, DataType: schemapb.DataType_VarChar},
				{Name: fieldSourceType, DataType: schemapb.DataType_Int64},
				{Name: fieldChunkID, DataType: schemapb.DataType_VarChar},
				{Name: fieldKnowledgeID, DataType: schemapb.DataType_VarChar},
				{Name: fieldKnowledgeBaseID, DataType: schemapb.DataType_VarChar},
				{Name: fieldTagID, DataType: schemapb.DataType_VarChar},
				{Name: fieldIsEnabled, DataType: schemapb.DataType_Bool},
			},
		},
	}, nil
}

func (s *copyIndicesFakeServer) LoadCollection(
	context.Context, *milvuspb.LoadCollectionRequest,
) (*commonpb.Status, error) {
	return &commonpb.Status{}, nil
}

func (s *copyIndicesFakeServer) GetLoadingProgress(
	context.Context, *milvuspb.GetLoadingProgressRequest,
) (*milvuspb.GetLoadingProgressResponse, error) {
	return &milvuspb.GetLoadingProgressResponse{Status: &commonpb.Status{}, Progress: 100}, nil
}

func (s *copyIndicesFakeServer) Query(
	_ context.Context, req *milvuspb.QueryRequest,
) (*milvuspb.QueryResults, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	knowledgeBaseID := ""
	chunkIDs := map[string]bool{}
	for name, value := range req.GetExprTemplateValues() {
		switch {
		case strings.HasPrefix(name, fieldKnowledgeBaseID+"_"):
			knowledgeBaseID = value.GetStringVal()
		case strings.HasPrefix(name, fieldChunkID+"_"):
			for _, chunkID := range value.GetArrayVal().GetStringData().GetData() {
				chunkIDs[chunkID] = true
			}
		}
	}

	selected := make([]*MilvusVectorEmbedding, 0, len(s.rows))
	for _, row := range s.rows {
		if row.KnowledgeBaseID != knowledgeBaseID {
			continue
		}
		if len(chunkIDs) > 0 && !chunkIDs[row.ChunkID] {
			continue
		}
		selected = append(selected, row)
	}

	s.queries++
	if s.rotateResults && len(selected) > 1 {
		// Same rows, different order: an order the API contract permits on
		// every single query.
		rotate := s.queries % len(selected)
		selected = append(selected[rotate:], selected[:rotate]...)
	}

	offset, limit := 0, 0
	for _, pair := range req.GetQueryParams() {
		s.queryParams = append(s.queryParams, pair.GetKey())
		switch pair.GetKey() {
		case "offset":
			offset, _ = strconv.Atoi(pair.GetValue())
		case "limit":
			limit, _ = strconv.Atoi(pair.GetValue())
		}
	}
	if offset >= len(selected) {
		selected = nil
	} else if offset > 0 {
		selected = selected[offset:]
	}
	if limit > 0 && limit < len(selected) {
		selected = selected[:limit]
	}

	return &milvuspb.QueryResults{
		Status:       &commonpb.Status{},
		OutputFields: req.GetOutputFields(),
		FieldsData:   copyIndicesRowsToFieldData(selected, s.dimension),
	}, nil
}

func (s *copyIndicesFakeServer) Upsert(
	_ context.Context, req *milvuspb.UpsertRequest,
) (*milvuspb.MutationResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rows := make([]*MilvusVectorEmbedding, req.GetNumRows())
	for i := range rows {
		rows[i] = &MilvusVectorEmbedding{}
	}
	for _, field := range req.GetFieldsData() {
		switch field.GetType() {
		case schemapb.DataType_VarChar:
			for i, value := range field.GetScalars().GetStringData().GetData() {
				switch field.GetFieldName() {
				case fieldID:
					rows[i].ID = value
				case fieldContent:
					rows[i].Content = value
				case fieldSourceID:
					rows[i].SourceID = value
				case fieldChunkID:
					rows[i].ChunkID = value
				case fieldKnowledgeID:
					rows[i].KnowledgeID = value
				case fieldKnowledgeBaseID:
					rows[i].KnowledgeBaseID = value
				case fieldTagID:
					rows[i].TagID = value
				}
			}
		case schemapb.DataType_Int64:
			if field.GetFieldName() != fieldSourceType {
				continue
			}
			for i, value := range field.GetScalars().GetLongData().GetData() {
				rows[i].SourceType = int(value)
			}
		case schemapb.DataType_Bool:
			if field.GetFieldName() != fieldIsEnabled {
				continue
			}
			for i, value := range field.GetScalars().GetBoolData().GetData() {
				rows[i].IsEnabled = value
			}
		case schemapb.DataType_FloatVector:
			vectors := field.GetVectors().GetFloatVector().GetData()
			dimension := int(field.GetVectors().GetDim())
			for i := range rows {
				rows[i].Embedding = vectors[i*dimension : (i+1)*dimension]
			}
		}
	}
	s.upserted = append(s.upserted, rows...)

	return &milvuspb.MutationResult{Status: &commonpb.Status{}, UpsertCnt: int64(len(rows))}, nil
}

func copyIndicesRowsToFieldData(rows []*MilvusVectorEmbedding, dimension int) []*schemapb.FieldData {
	ids := make([]string, len(rows))
	contents := make([]string, len(rows))
	sourceIDs := make([]string, len(rows))
	sourceTypes := make([]int64, len(rows))
	chunkIDs := make([]string, len(rows))
	knowledgeIDs := make([]string, len(rows))
	knowledgeBaseIDs := make([]string, len(rows))
	tagIDs := make([]string, len(rows))
	isEnableds := make([]bool, len(rows))
	vectors := make([]float32, 0, len(rows)*dimension)
	for i, row := range rows {
		ids[i] = row.ID
		contents[i] = row.Content
		sourceIDs[i] = row.SourceID
		sourceTypes[i] = int64(row.SourceType)
		chunkIDs[i] = row.ChunkID
		knowledgeIDs[i] = row.KnowledgeID
		knowledgeBaseIDs[i] = row.KnowledgeBaseID
		tagIDs[i] = row.TagID
		isEnableds[i] = row.IsEnabled
		vectors = append(vectors, row.Embedding...)
	}

	varCharField := func(name string, values []string) *schemapb.FieldData {
		return &schemapb.FieldData{
			Type:      schemapb.DataType_VarChar,
			FieldName: name,
			Field: &schemapb.FieldData_Scalars{Scalars: &schemapb.ScalarField{
				Data: &schemapb.ScalarField_StringData{StringData: &schemapb.StringArray{Data: values}},
			}},
		}
	}
	return []*schemapb.FieldData{
		varCharField(fieldID, ids),
		varCharField(fieldContent, contents),
		varCharField(fieldSourceID, sourceIDs),
		{
			Type:      schemapb.DataType_Int64,
			FieldName: fieldSourceType,
			Field: &schemapb.FieldData_Scalars{Scalars: &schemapb.ScalarField{
				Data: &schemapb.ScalarField_LongData{LongData: &schemapb.LongArray{Data: sourceTypes}},
			}},
		},
		varCharField(fieldChunkID, chunkIDs),
		varCharField(fieldKnowledgeID, knowledgeIDs),
		varCharField(fieldKnowledgeBaseID, knowledgeBaseIDs),
		varCharField(fieldTagID, tagIDs),
		{
			Type:      schemapb.DataType_Bool,
			FieldName: fieldIsEnabled,
			Field: &schemapb.FieldData_Scalars{Scalars: &schemapb.ScalarField{
				Data: &schemapb.ScalarField_BoolData{BoolData: &schemapb.BoolArray{Data: isEnableds}},
			}},
		},
		{
			Type:      schemapb.DataType_FloatVector,
			FieldName: fieldEmbedding,
			Field: &schemapb.FieldData_Vectors{Vectors: &schemapb.VectorField{
				Dim: int64(dimension),
				Data: &schemapb.VectorField_FloatVector{FloatVector: &schemapb.FloatArray{Data: vectors}},
			}},
		},
	}
}

func newCopyIndicesTestRepository(t *testing.T, fake *copyIndicesFakeServer) *milvusRepository {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := grpc.NewServer()
	milvuspb.RegisterMilvusServiceServer(server, fake)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)

	ctx := context.Background()
	milvusClient, err := client.New(ctx, &client.ClientConfig{
		Address:     listener.Addr().String(),
		DisableConn: true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = milvusClient.Close(ctx) })

	return &milvusRepository{
		filter:             filter{},
		client:             milvusClient,
		collectionBaseName: "weknora_embeddings_copyindices_test",
	}
}

func TestCopyIndicesCopiesEveryMappedRowDespiteResultOrderDrift(t *testing.T) {
	const chunks = 200
	const dimension = 4

	rows := make([]*MilvusVectorEmbedding, 0, chunks+chunks/10)
	sourceToTargetChunkID := make(map[string]string, chunks)
	sourceToTargetKnowledgeID := map[string]string{"doc-source": "doc-target"}
	for i := range chunks {
		sourceChunkID := fmt.Sprintf("chunk-%03d", i)
		targetChunkID := fmt.Sprintf("target-chunk-%03d", i)
		sourceToTargetChunkID[sourceChunkID] = targetChunkID
		rows = append(rows, &MilvusVectorEmbedding{
			ID:              fmt.Sprintf("row-%03d", i),
			Content:         fmt.Sprintf("content %d", i),
			SourceID:        sourceChunkID,
			SourceType:      1,
			ChunkID:         sourceChunkID,
			KnowledgeID:     "doc-source",
			KnowledgeBaseID: "kb-source",
			Embedding:       []float32{float32(i), 1, 2, 3},
			IsEnabled:       i%2 == 0,
		})
		if i%10 == 0 {
			// QA chunks share a chunk_id with extra question rows, so a batch
			// of chunk ids matches more rows than ids.
			rows = append(rows, &MilvusVectorEmbedding{
				ID:              fmt.Sprintf("row-%03d-q1", i),
				Content:         fmt.Sprintf("question %d", i),
				SourceID:        sourceChunkID + "-q1",
				SourceType:      1,
				ChunkID:         sourceChunkID,
				KnowledgeID:     "doc-source",
				KnowledgeBaseID: "kb-source",
				Embedding:       []float32{0, float32(i), 2, 3},
				IsEnabled:       true,
			})
		}
	}

	fake := &copyIndicesFakeServer{rows: rows, dimension: dimension, rotateResults: true}
	repo := newCopyIndicesTestRepository(t, fake)

	require.NoError(t, repo.CopyIndices(
		context.Background(),
		"kb-source",
		sourceToTargetKnowledgeID,
		sourceToTargetChunkID,
		"kb-target",
		dimension,
		"",
	))

	// 200 main rows plus 20 question rows: nothing may be lost to the order
	// drift the fake introduces on every query.
	require.Len(t, fake.upserted, 220)

	copied := map[string]*MilvusVectorEmbedding{}
	rowIDs := map[string]bool{}
	for _, row := range fake.upserted {
		identity := row.ChunkID + "|" + row.SourceID
		require.Nil(t, copied[identity], "row copied twice: %s", identity)
		copied[identity] = row
		require.False(t, rowIDs[row.ID], "duplicate target row id: %s", row.ID)
		rowIDs[row.ID] = true

		require.Equal(t, "kb-target", row.KnowledgeBaseID)
		require.Equal(t, "doc-target", row.KnowledgeID)
		require.Len(t, row.Embedding, dimension)
	}
	require.Len(t, rowIDs, 220)

	for i := range chunks {
		targetChunkID := fmt.Sprintf("target-chunk-%03d", i)
		main := copied[targetChunkID+"|"+targetChunkID]
		require.NotNil(t, main, "main row of chunk %d missing", i)
		require.Equal(t, fmt.Sprintf("content %d", i), main.Content)
		require.Equal(t, i%2 == 0, main.IsEnabled)

		if i%10 == 0 {
			question := copied[targetChunkID+"|"+targetChunkID+"-q1"]
			require.NotNil(t, question, "question row of chunk %d missing", i)
			require.Equal(t, fmt.Sprintf("question %d", i), question.Content)
		}
	}

	// One read per 64-chunk batch, and none of them pages with an offset
	// window: the mapping bounds the reads, not the result order.
	require.Equal(t, 4, fake.queries)
	require.NotContains(t, fake.queryParams, "offset")
}

func TestCopyIndicesSkipsMappingEntriesWithoutSourceRows(t *testing.T) {
	const dimension = 4

	rows := make([]*MilvusVectorEmbedding, 0, 5)
	sourceToTargetChunkID := make(map[string]string, 6)
	for i := range 6 {
		sourceChunkID := fmt.Sprintf("chunk-%02d", i)
		sourceToTargetChunkID[sourceChunkID] = fmt.Sprintf("target-chunk-%02d", i)
		if i == 5 {
			continue
		}
		rows = append(rows, &MilvusVectorEmbedding{
			ID:              fmt.Sprintf("row-%02d", i),
			Content:         fmt.Sprintf("content %d", i),
			SourceID:        sourceChunkID,
			ChunkID:         sourceChunkID,
			KnowledgeID:     "doc-source",
			KnowledgeBaseID: "kb-source",
			Embedding:       []float32{1, 2, 3, 4},
		})
	}

	fake := &copyIndicesFakeServer{rows: rows, dimension: dimension}
	repo := newCopyIndicesTestRepository(t, fake)

	require.NoError(t, repo.CopyIndices(
		context.Background(),
		"kb-source",
		map[string]string{"doc-source": "doc-target"},
		sourceToTargetChunkID,
		"kb-target",
		dimension,
		"",
	))

	require.Len(t, fake.upserted, 5)
	for _, row := range fake.upserted {
		require.NotEqual(t, "target-chunk-05", row.ChunkID)
		require.Equal(t, "kb-target", row.KnowledgeBaseID)
	}
}
