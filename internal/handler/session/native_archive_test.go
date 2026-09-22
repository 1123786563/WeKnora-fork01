package session

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/nativecontract"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type nativeArchiveHandlerScopeFake struct {
	scope        nativecontract.Scope
	err          error
	recheckScope nativecontract.Scope
	recheckErr   error
	recheckCalls int
}

func (f nativeArchiveHandlerScopeFake) Resolve(context.Context) (nativecontract.Scope, error) {
	return f.scope, f.err
}

func (f *nativeArchiveHandlerScopeFake) Recheck(context.Context, nativecontract.Scope, []nativecontract.ResourceGrant) (nativecontract.Scope, error) {
	f.recheckCalls++
	if f.recheckErr != nil {
		return nativecontract.Scope{}, f.recheckErr
	}
	if f.recheckScope.TenantID != 0 {
		return f.recheckScope, nil
	}
	return f.scope, f.err
}

type nativeArchiveHandlerReaderFake struct {
	page          nativecontract.ArchivePage
	record        nativecontract.ArchiveRecord
	artifact      nativecontract.ArtifactRef
	err           error
	query         nativecontract.ArchiveQuery
	listScope     nativecontract.Scope
	readScope     nativecontract.Scope
	artifactScope nativecontract.Scope
}

func (f *nativeArchiveHandlerReaderFake) List(_ context.Context, scope nativecontract.Scope, query nativecontract.ArchiveQuery) (nativecontract.ArchivePage, error) {
	f.listScope = scope
	f.query = query
	return f.page, f.err
}

func (f *nativeArchiveHandlerReaderFake) Read(_ context.Context, scope nativecontract.Scope, _ string) (nativecontract.ArchiveRecord, error) {
	f.readScope = scope
	return f.record, f.err
}

func (f *nativeArchiveHandlerReaderFake) ReadArtifact(_ context.Context, scope nativecontract.Scope, _ string) (nativecontract.ArtifactRef, error) {
	f.artifactScope = scope
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
	h := NewNativeArchiveHandler(reader, &nativeArchiveHandlerScopeFake{scope: nativecontract.Scope{TenantID: 7, SessionOwnerID: "viewer-1"}})
	c, w := nativeArchiveRequest(http.MethodGet, "/api/v1/agent-archive/sessions?session_id=session-1&kind=message&limit=200")
	h.List(c)

	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, `{"success":true,"data":{"records":[{"id":"record-1","session_id":"session-1","kind":"message","data":null,"artifacts":[{"id":"artifact-1","media_type":"text/plain","sha256":"sha256:abc","size_bytes":"42"}]}],"next_cursor":"cursor-2"}}`, w.Body.String())
	require.Equal(t, 100, reader.query.Limit)
}

func TestNativeArchiveHandlerRedactsStorageReferenceFromArtifactResponse(t *testing.T) {
	reader := &nativeArchiveHandlerReaderFake{artifact: nativecontract.ArtifactRef{ID: "artifact-1", MediaType: "text/plain", SHA256: "sha256:abc", SizeBytes: 42}}
	h := NewNativeArchiveHandler(reader, &nativeArchiveHandlerScopeFake{scope: nativecontract.Scope{TenantID: 7, SessionOwnerID: "viewer-1"}})
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
		scope *nativeArchiveHandlerScopeFake
		err   error
		want  int
	}{
		{name: "revoked viewer", scope: &nativeArchiveHandlerScopeFake{err: errors.New("revoked")}, want: http.StatusForbidden},
		{name: "missing record", scope: &nativeArchiveHandlerScopeFake{scope: nativecontract.Scope{TenantID: 7, SessionOwnerID: "viewer-1"}}, err: &nativecontract.Failure{Code: nativecontract.ErrNotFound}, want: http.StatusNotFound},
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

func TestNativeArchiveHandlerRechecksScopeBeforeEveryRead(t *testing.T) {
	viewer := nativecontract.Scope{TenantID: 7, SessionOwnerID: "viewer-1"}
	current := nativecontract.Scope{TenantID: 7, SessionOwnerID: "viewer-1", PolicyRevision: 11}
	scopes := &nativeArchiveHandlerScopeFake{scope: viewer, recheckScope: current}
	reader := &nativeArchiveHandlerReaderFake{}
	h := NewNativeArchiveHandler(reader, scopes)
	c, w := nativeArchiveRequest(http.MethodGet, "/api/v1/agent-archive/sessions")
	h.List(c)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, 1, scopes.recheckCalls)
	require.Equal(t, current, reader.listScope)
}

func TestNativeArchiveHandlerBindsRecheckedScopeToRecordAndArtifact(t *testing.T) {
	viewer := nativecontract.Scope{TenantID: 7, SessionOwnerID: "viewer-1"}
	current := nativecontract.Scope{TenantID: 7, SessionOwnerID: "viewer-1", PolicyRevision: 11}
	for _, tc := range []struct {
		name     string
		path     string
		invoke   func(*NativeArchiveHandler, *gin.Context)
		observed func(*nativeArchiveHandlerReaderFake) nativecontract.Scope
	}{
		{
			name: "record", path: "/api/v1/agent-archive/records/record-1",
			invoke: func(h *NativeArchiveHandler, c *gin.Context) {
				c.Params = gin.Params{{Key: "id", Value: "record-1"}}
				h.GetRecord(c)
			},
			observed: func(r *nativeArchiveHandlerReaderFake) nativecontract.Scope { return r.readScope },
		},
		{
			name: "artifact", path: "/api/v1/agent-archive/artifacts/artifact-1",
			invoke: func(h *NativeArchiveHandler, c *gin.Context) {
				c.Params = gin.Params{{Key: "id", Value: "artifact-1"}}
				h.GetArtifact(c)
			},
			observed: func(r *nativeArchiveHandlerReaderFake) nativecontract.Scope { return r.artifactScope },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			scopes := &nativeArchiveHandlerScopeFake{scope: viewer, recheckScope: current}
			reader := &nativeArchiveHandlerReaderFake{artifact: nativecontract.ArtifactRef{ID: "artifact-1", MediaType: "text/plain", SHA256: "sha256:abc"}}
			h := NewNativeArchiveHandler(reader, scopes)
			c, w := nativeArchiveRequest(http.MethodGet, tc.path)

			tc.invoke(h, c)

			require.Equal(t, http.StatusOK, w.Code)
			require.Equal(t, 1, scopes.recheckCalls)
			require.Equal(t, current, tc.observed(reader))
		})
	}
}

func TestNativeArchiveHandlerRejectsMutationWithFrozenWireCode(t *testing.T) {
	c, w := nativeArchiveRequest(http.MethodPost, "/api/v1/agent-archive/sessions/session-1/resume")
	writeNativeArchiveFailure(c, &nativecontract.Failure{Code: nativecontract.ErrArchiveReadOnly, Message: "历史记录仅供查询"})

	require.Equal(t, http.StatusConflict, w.Code)
	require.Contains(t, w.Body.String(), string(nativecontract.ErrArchiveReadOnly))
}
