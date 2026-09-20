package session

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/agent/nativecontract"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type nativeArchiveHandlerScopeFake struct {
	scope nativecontract.Scope
	err   error
}

func (f nativeArchiveHandlerScopeFake) Resolve(context.Context) (nativecontract.Scope, error) {
	return f.scope, f.err
}
func (f nativeArchiveHandlerScopeFake) Recheck(context.Context, nativecontract.Scope, []nativecontract.ResourceGrant) (nativecontract.Scope, error) {
	return f.scope, f.err
}

type nativeArchiveHandlerReaderFake struct {
	page     nativecontract.ArchivePage
	record   nativecontract.ArchiveRecord
	artifact nativecontract.ArtifactRef
	err      error
	query    nativecontract.ArchiveQuery
}

func (f *nativeArchiveHandlerReaderFake) List(_ context.Context, _ nativecontract.Scope, query nativecontract.ArchiveQuery) (nativecontract.ArchivePage, error) {
	f.query = query
	return f.page, f.err
}
func (f *nativeArchiveHandlerReaderFake) Read(context.Context, nativecontract.Scope, string) (nativecontract.ArchiveRecord, error) {
	return f.record, f.err
}
func (f *nativeArchiveHandlerReaderFake) ReadArtifact(context.Context, nativecontract.Scope, string) (nativecontract.ArtifactRef, error) {
	return f.artifact, f.err
}

func nativeArchiveRequest(method, target string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(7))
	c.Request = httptest.NewRequest(method, target, nil).WithContext(ctx)
	return c, w
}

func TestNativeArchiveHandlerListsStablePageForViewer(t *testing.T) {
	reader := &nativeArchiveHandlerReaderFake{page: nativecontract.ArchivePage{Records: []nativecontract.ArchiveRecord{{ID: "record-1", SessionID: "session-1", Kind: "message", Artifacts: []nativecontract.PublicArtifactRef{{ID: "artifact-1", MediaType: "text/plain", SHA256: "sha256:abc", SizeBytes: "42"}}}}, NextCursor: "cursor-2"}}
	h := NewNativeArchiveHandler(reader, nativeArchiveHandlerScopeFake{scope: nativecontract.Scope{TenantID: 7, SessionOwnerID: "viewer-1"}})
	c, w := nativeArchiveRequest(http.MethodGet, "/api/v1/agent-archive/sessions?session_id=session-1&kind=message&limit=200")
	h.List(c)

	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, `{"success":true,"data":{"records":[{"id":"record-1","session_id":"session-1","kind":"message","data":null,"artifacts":[{"id":"artifact-1","media_type":"text/plain","sha256":"sha256:abc","size_bytes":"42"}]}],"next_cursor":"cursor-2"}}`, w.Body.String())
	require.Equal(t, 100, reader.query.Limit)
}

func TestNativeArchiveHandlerRedactsStorageReferenceFromArtifactResponse(t *testing.T) {
	reader := &nativeArchiveHandlerReaderFake{artifact: nativecontract.ArtifactRef{ID: "artifact-1", MediaType: "text/plain", SHA256: "sha256:abc", SizeBytes: 42}}
	h := NewNativeArchiveHandler(reader, nativeArchiveHandlerScopeFake{scope: nativecontract.Scope{TenantID: 7, SessionOwnerID: "viewer-1"}})
	c, w := nativeArchiveRequest(http.MethodGet, "/api/v1/agent-archive/artifacts/artifact-1")
	c.Params = gin.Params{{Key: "id", Value: "artifact-1"}}
	h.GetArtifact(c)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"size_bytes":"42"`)
	require.Contains(t, w.Body.String(), `"download_reference":{"artifact_id":"artifact-1"}`)
	require.NotContains(t, w.Body.String(), "storage")
	require.NotContains(t, w.Body.String(), "provider")
}

func TestNativeArchiveHandlerDistinguishesRevocationAndMissingRecord(t *testing.T) {
	for _, tc := range []struct {
		name  string
		scope nativeArchiveHandlerScopeFake
		err   error
		want  int
	}{
		{name: "revoked viewer", scope: nativeArchiveHandlerScopeFake{err: errors.New("revoked")}, want: http.StatusForbidden},
		{name: "missing record", scope: nativeArchiveHandlerScopeFake{scope: nativecontract.Scope{TenantID: 7, SessionOwnerID: "viewer-1"}}, err: &nativecontract.Failure{Code: nativecontract.ErrNotFound}, want: http.StatusNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := NewNativeArchiveHandler(&nativeArchiveHandlerReaderFake{err: tc.err}, tc.scope)
			c, w := nativeArchiveRequest(http.MethodGet, "/api/v1/agent-archive/records/record-1")
			c.Params = gin.Params{{Key: "id", Value: "record-1"}}
			h.GetRecord(c)
			require.Equal(t, tc.want, w.Code)
		})
	}
}
