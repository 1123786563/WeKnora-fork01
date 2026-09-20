package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Tencent/WeKnora/internal/agent/nativecontract"
	"github.com/stretchr/testify/require"
)

type nativeArchiveScopeFake struct {
	result nativecontract.Scope
	err    error
	calls  int
}

func (f *nativeArchiveScopeFake) Resolve(context.Context) (nativecontract.Scope, error) {
	return f.result, f.err
}
func (f *nativeArchiveScopeFake) Recheck(_ context.Context, _ nativecontract.Scope, _ []nativecontract.ResourceGrant) (nativecontract.Scope, error) {
	f.calls++
	if f.err != nil {
		return nativecontract.Scope{}, f.err
	}
	return f.result, nil
}

type nativeArchiveStoreFake struct {
	listQuery  nativecontract.ArchiveQuery
	listScope  nativecontract.Scope
	readID     string
	artifactID string
	listPage   nativecontract.ArchivePage
	record     nativecontract.ArchiveRecord
	artifact   nativecontract.ArtifactRef
	err        error
	calls      int
}

func (f *nativeArchiveStoreFake) List(_ context.Context, scope nativecontract.Scope, query nativecontract.ArchiveQuery) (nativecontract.ArchivePage, error) {
	f.calls++
	f.listScope, f.listQuery = scope, query
	return f.listPage, f.err
}
func (f *nativeArchiveStoreFake) Read(_ context.Context, _ nativecontract.Scope, id string) (nativecontract.ArchiveRecord, error) {
	f.calls++
	f.readID = id
	return f.record, f.err
}
func (f *nativeArchiveStoreFake) ReadArtifact(_ context.Context, _ nativecontract.Scope, id string) (nativecontract.ArtifactRef, error) {
	f.calls++
	f.artifactID = id
	return f.artifact, f.err
}

func nativeArchiveScope() nativecontract.Scope {
	return nativecontract.Scope{TenantID: 7, SessionOwnerID: "viewer-1"}
}

func TestNativeArchiveListRechecksCurrentScopeAndNormalizesPagination(t *testing.T) {
	scopes := &nativeArchiveScopeFake{result: nativeArchiveScope()}
	store := &nativeArchiveStoreFake{listPage: nativecontract.ArchivePage{Records: []nativecontract.ArchiveRecord{{ID: "record-1", SessionID: "session-1", Kind: "message"}}}}
	svc := NewNativeArchiveService(store, scopes)

	page, err := svc.List(context.Background(), nativeArchiveScope(), nativecontract.ArchiveQuery{SessionID: "session-1", Kind: "message", Limit: 999})

	require.NoError(t, err)
	require.Len(t, page.Records, 1)
	require.Equal(t, 1, scopes.calls)
	require.Equal(t, nativeArchiveScope(), store.listScope)
	require.Equal(t, 100, store.listQuery.Limit)
	require.Equal(t, "session-1", store.listQuery.SessionID)
}

// The public contract calls 100 the default pagination bound.  Supplying no
// limit must therefore be equivalent to asking for the maximum safe page,
// rather than silently selecting a smaller, undocumented page size.
func TestNativeArchiveListDefaultsPaginationToContractMaximum(t *testing.T) {
	scopes := &nativeArchiveScopeFake{result: nativeArchiveScope()}
	store := &nativeArchiveStoreFake{}
	svc := NewNativeArchiveService(store, scopes)

	_, err := svc.List(context.Background(), nativeArchiveScope(), nativecontract.ArchiveQuery{})

	require.NoError(t, err)
	require.Equal(t, 100, store.listQuery.Limit)
}

func TestNativeArchiveListHidesCrossSessionRecords(t *testing.T) {
	store := &nativeArchiveStoreFake{listPage: nativecontract.ArchivePage{Records: []nativecontract.ArchiveRecord{{ID: "record-1", SessionID: "other-session", Kind: "message"}}}}
	svc := NewNativeArchiveService(store, &nativeArchiveScopeFake{result: nativeArchiveScope()})

	_, err := svc.List(context.Background(), nativeArchiveScope(), nativecontract.ArchiveQuery{SessionID: "session-1", Limit: 10})

	var failure *nativecontract.Failure
	require.True(t, errors.As(err, &failure))
	require.Equal(t, nativecontract.ErrNotFound, failure.Code)
}

func TestNativeArchiveRejectsRevokedViewerBeforeAnyArchiveRead(t *testing.T) {
	store := &nativeArchiveStoreFake{}
	svc := NewNativeArchiveService(store, &nativeArchiveScopeFake{result: nativeArchiveScope(), err: errors.New("membership revoked")})

	_, err := svc.Read(context.Background(), nativeArchiveScope(), "record-1")

	var failure *nativecontract.Failure
	require.True(t, errors.As(err, &failure))
	require.Equal(t, nativecontract.ErrForbidden, failure.Code)
	require.Zero(t, store.calls)
}

func TestNativeArchiveReadArtifactReturnsOnlyContractMetadata(t *testing.T) {
	store := &nativeArchiveStoreFake{artifact: nativecontract.ArtifactRef{ID: "artifact-1", MediaType: "text/plain", SHA256: "sha256:abc", SizeBytes: 42}}
	svc := NewNativeArchiveService(store, &nativeArchiveScopeFake{result: nativeArchiveScope()})

	artifact, err := svc.ReadArtifact(context.Background(), nativeArchiveScope(), "artifact-1")

	require.NoError(t, err)
	require.Equal(t, "artifact-1", artifact.ID)
	require.Equal(t, "sha256:abc", artifact.SHA256)
	require.Equal(t, int64(42), artifact.SizeBytes)
}

func TestArchiveMutationErrorUsesFrozenWireCode(t *testing.T) {
	require.Equal(t, nativecontract.ErrArchiveReadOnly, ArchiveMutationError().Code)
}
