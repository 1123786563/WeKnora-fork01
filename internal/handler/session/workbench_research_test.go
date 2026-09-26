package session

// T17 (#47) handler evidence: the delegation surface is structurally
// read-only (the handler holds no AgentRunStore write path), sources are
// validated against the tenant knowledge scope BEFORE any durable write,
// annotations pin the CURRENT artifact version identity, and every read
// falls back to the granted reader exactly like the delivery read face.

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// ─── stubs ───────────────────────────────────────────────────────────────────

type researchStoreStub struct {
	created       []types.TaskResearchDelegation
	completed     map[string]string
	getByID       func(tenant uint64, id string) (types.TaskResearchDelegation, error)
	completeCalls int
}

func newResearchStoreStub() *researchStoreStub {
	return &researchStoreStub{completed: map[string]string{}}
}

func (s *researchStoreStub) CreateDelegation(_ context.Context, d types.TaskResearchDelegation) error {
	s.created = append(s.created, d)
	return nil
}

func (s *researchStoreStub) GetDelegation(_ context.Context, tenantID uint64, id string) (types.TaskResearchDelegation, error) {
	if s.getByID != nil {
		return s.getByID(tenantID, id)
	}
	for _, d := range s.created {
		if d.TenantID == tenantID && d.ID == id {
			return d, nil
		}
	}
	return types.TaskResearchDelegation{}, types.ErrTaskResearchNotFound
}

func (s *researchStoreStub) ListDelegationsBySession(_ context.Context, tenantID uint64, sessionID string) ([]types.TaskResearchDelegation, error) {
	var out []types.TaskResearchDelegation
	for _, d := range s.created {
		if d.TenantID == tenantID && d.SessionID == sessionID {
			out = append(out, d)
		}
	}
	return out, nil
}

func (s *researchStoreStub) CompleteDelegation(_ context.Context, tenantID uint64, id, summary string) (types.TaskResearchDelegation, error) {
	s.completeCalls++
	if s.getByID != nil {
		got, err := s.getByID(tenantID, id)
		if err != nil {
			return got, err
		}
		if got.Status != types.TaskResearchAssigned {
			return got, types.ErrTaskResearchState
		}
		got.Status = types.TaskResearchCompleted
		got.Summary = summary
		return got, nil
	}
	for i, d := range s.created {
		if d.TenantID == tenantID && d.ID == id {
			if d.Status != types.TaskResearchAssigned {
				return d, types.ErrTaskResearchState
			}
			s.created[i].Status = types.TaskResearchCompleted
			s.created[i].Summary = summary
			return s.created[i], nil
		}
	}
	return types.TaskResearchDelegation{}, types.ErrTaskResearchNotFound
}

type annotationStoreStub struct {
	created []types.TaskArtifactAnnotation
}

func (s *annotationStoreStub) CreateAnnotation(_ context.Context, a types.TaskArtifactAnnotation) error {
	s.created = append(s.created, a)
	return nil
}

func (s *annotationStoreStub) ListAnnotationsBySession(_ context.Context, tenantID uint64, sessionID string) ([]types.TaskArtifactAnnotation, error) {
	var out []types.TaskArtifactAnnotation
	for _, a := range s.created {
		if a.TenantID == tenantID && a.SessionID == sessionID {
			out = append(out, a)
		}
	}
	return out, nil
}

type sourceAuthorizerStub struct{ denied map[string]bool }

func (s sourceAuthorizerStub) AuthorizeResearchSource(_ context.Context, _ uint64, kbID string) error {
	if s.denied[kbID] {
		return errors.New("source outside tenant knowledge scope")
	}
	return nil
}

type accessResolverStub struct{ role types.TaskAccessRole }

func (a accessResolverStub) ResolveTaskAccess(context.Context, types.Caller, string) (types.TaskAccess, error) {
	return types.TaskAccess{TaskID: "sess-1", OwnerID: "u1", Role: a.role}, nil
}

// ownerRunStub satisfies OwnedRunReader only (owner = u1 @ tenant 1).
func researchRunStub() *workbenchRunReaderStub {
	return &workbenchRunReaderStub{run: agentruntime.Run{
		Key:       agentruntime.RunKey{TenantID: 1, RunID: "run-1"},
		SessionID: "sess-1",
	}}
}

// grantedRunStub satisfies GrantedRunReader: u2 (viewer) and u3
// (collaborator) hold grants on run-1; everyone else misses.
type grantedRunStub struct{}

func (grantedRunStub) GetRunForGrantedReader(_ context.Context, tenantID uint64, readerID, runID string) (agentruntime.Run, error) {
	if tenantID == 1 && (readerID == "u2" || readerID == "u3") && runID == "run-1" {
		return agentruntime.Run{Key: agentruntime.RunKey{TenantID: tenantID, RunID: runID}, SessionID: "sess-1"}, nil
	}
	return agentruntime.Run{}, agentruntime.ErrNotFound
}

func researchHandler(runs OwnedRunReader, granted GrantedRunReader, role types.TaskAccessRole) (*WorkbenchResearchHandler, *researchStoreStub, *annotationStoreStub, *artifactRefReaderStub) {
	delegations := newResearchStoreStub()
	annotations := &annotationStoreStub{}
	refs := &artifactRefReaderStub{refs: researchRefs()}
	h := NewWorkbenchResearchHandler(
		runs, granted, refs,
		delegations, annotations,
		sourceAuthorizerStub{denied: map[string]bool{"kb-secret": true, "": true}},
		accessResolverStub{role: role},
	)
	return h, delegations, annotations, refs
}

func researchHandlerEnv(role types.TaskAccessRole) (*WorkbenchResearchHandler, *researchStoreStub, *annotationStoreStub, *artifactRefReaderStub) {
	return researchHandler(researchRunStub(), grantedRunStub{}, role)
}

// researchRefs pins one artifact whose version identity derives from its
// ContentHash (same derivation as the workbench artifact list).
func researchRefs() []types.SessionArtifactRef {
	return []types.SessionArtifactRef{
		{MessageID: "m1", Index: 0, Artifact: types.MessageArtifact{
			URL: "local://t/1/report.md", FileName: "report.md", FileType: ".md", FileSize: 256,
			ContentHash: "9a2f1c3d4e5f6a7b0000000000000000000000000000000000000000000000abc",
			CreatedAt:   time.Unix(1_700_000_000, 0),
		}},
	}
}

func researchContext(method, path, body string) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	c.Request = httptest.NewRequest(method, path, reader)
	c.Request.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, uint64(1))
	ctx = context.WithValue(ctx, types.UserIDContextKey, "u1")
	c.Request = c.Request.WithContext(ctx)
	if strings.Contains(path, "/run-1/") || strings.HasSuffix(path, "/run-1") {
		c.Params = gin.Params{{Key: "run_id", Value: "run-1"}}
	}
	if strings.Contains(path, "delegation_id") {
		c.Params = append(c.Params, gin.Param{Key: "delegation_id", Value: "d1"})
	}
	return c, recorder
}

// ─── delegate ────────────────────────────────────────────────────────────────

func TestDelegateResearchPersistsReadOnlyDelegation(t *testing.T) {
	h, delegations, _, _ := researchHandlerEnv(types.TaskAccessOwner)
	c, rec := researchContext(http.MethodPost, "/api/v1/workbench/executions/run-1/research",
		`{"objective":"survey retrieval baselines","sources":["kb-1","kb-2"]}`)
	h.DelegateResearch(c)

	require.Equal(t, http.StatusCreated, c.Writer.Status())
	require.Len(t, delegations.created, 1)
	d := delegations.created[0]
	require.Equal(t, uint64(1), d.TenantID)
	require.Equal(t, "sess-1", d.SessionID)
	require.Equal(t, "run-1", d.ParentRunID)
	require.Equal(t, types.TaskResearchAssigned, d.Status)
	require.Equal(t, "u1", d.CreatedBy)

	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Delegation researchDelegationView `json:"delegation"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.True(t, body.Success)
	require.Equal(t, "run-1", body.Data.Delegation.RunID)
	require.Equal(t, []string{"kb-1", "kb-2"}, body.Data.Delegation.Sources)
	require.Equal(t, "assigned", body.Data.Delegation.Status)
}

func TestDelegateResearchRejectsSourceOutsideTenantScope(t *testing.T) {
	h, delegations, _, _ := researchHandlerEnv(types.TaskAccessOwner)
	// 越权源必须在任何落库之前被拒：先断言零行，再断言响应。
	c, rec := researchContext(http.MethodPost, "/api/v1/workbench/executions/run-1/research",
		`{"objective":"x","sources":["kb-1","kb-secret"]}`)
	h.DelegateResearch(c)

	require.Equal(t, http.StatusBadRequest, c.Writer.Status())
	require.Contains(t, rec.Body.String(), "research_source_out_of_task_grant")
	require.Empty(t, delegations.created, "越权委派绝不能落库（AC1）")
}

func TestDelegateResearchRequiresObjectiveAndSources(t *testing.T) {
	h, delegations, _, _ := researchHandlerEnv(types.TaskAccessOwner)
	c, _ := researchContext(http.MethodPost, "/api/v1/workbench/executions/run-1/research", `{"objective":"x","sources":[]}`)
	h.DelegateResearch(c)
	require.Equal(t, http.StatusBadRequest, c.Writer.Status())

	c, _ = researchContext(http.MethodPost, "/api/v1/workbench/executions/run-1/research", `{"objective":"","sources":["kb-1"]}`)
	h.DelegateResearch(c)
	require.Equal(t, http.StatusBadRequest, c.Writer.Status())
	require.Empty(t, delegations.created)
}

// ─── list / complete ─────────────────────────────────────────────────────────

func TestListResearchFallsBackToGrantedReader(t *testing.T) {
	h, delegations, _, _ := researchHandlerEnv(types.TaskAccessViewer)
	delegations.created = append(delegations.created, types.TaskResearchDelegation{
		TenantID: 1, ID: "d1", SessionID: "sess-1", ParentRunID: "run-1",
		Objective: "o", SourcesJSON: `["kb-1"]`, Status: types.TaskResearchAssigned, CreatedBy: "u1",
	})
	// owner 路径以 u2 miss（stub 只认 u1），granted 路径以 u2 命中 → 200。
	c, rec := researchContext(http.MethodGet, "/api/v1/workbench/executions/run-1/research", "")
	c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), types.UserIDContextKey, "u2"))
	h.ListResearch(c)

	require.Equal(t, http.StatusOK, c.Writer.Status())
	require.Contains(t, rec.Body.String(), `"delegation_id":"d1"`)

	// 无 grant 的读者：owner（u9≠u1）与 granted（u9 不持 grant）都 miss → 统一 404。
	h2, _, _, _ := researchHandlerEnv(types.TaskAccessViewer)
	c2, _ := researchContext(http.MethodGet, "/api/v1/workbench/executions/run-1/research", "")
	c2.Request = c2.Request.WithContext(context.WithValue(c2.Request.Context(), types.UserIDContextKey, "u9"))
	h2.ListResearch(c2)
	require.Equal(t, http.StatusNotFound, c2.Writer.Status())
}

func TestCompleteResearchCASConflictsOnCompleted(t *testing.T) {
	h, delegations, _, _ := researchHandlerEnv(types.TaskAccessOwner)
	delegations.created = append(delegations.created, types.TaskResearchDelegation{
		TenantID: 1, ID: "d1", SessionID: "sess-1", ParentRunID: "run-1",
		Objective: "o", SourcesJSON: `["kb-1"]`, Status: types.TaskResearchCompleted, Summary: "done once", CreatedBy: "u1",
	})
	c, rec := researchContext(http.MethodPost, "/api/v1/workbench/executions/run-1/research/delegation_id/summary", `{"summary":"replay"}`)
	c.Params = gin.Params{{Key: "run_id", Value: "run-1"}, {Key: "delegation_id", Value: "d1"}}
	h.CompleteResearch(c)

	require.Equal(t, http.StatusConflict, c.Writer.Status())
	require.Contains(t, rec.Body.String(), "research_delegation_state")
}

func TestCompleteResearchCrossTaskDelegationIsUniform404(t *testing.T) {
	h, delegations, _, _ := researchHandlerEnv(types.TaskAccessOwner)
	// d1 属于另一 session：即使 id 命中，session 绑定不匹配也必须 404。
	delegations.created = append(delegations.created, types.TaskResearchDelegation{
		TenantID: 1, ID: "d1", SessionID: "sess-other", ParentRunID: "run-9",
		Objective: "o", SourcesJSON: `["kb-1"]`, Status: types.TaskResearchAssigned, CreatedBy: "u1",
	})
	c, _ := researchContext(http.MethodPost, "/api/v1/workbench/executions/run-1/research/delegation_id/summary", `{"summary":"s"}`)
	c.Params = gin.Params{{Key: "run_id", Value: "run-1"}, {Key: "delegation_id", Value: "d1"}}
	h.CompleteResearch(c)
	require.Equal(t, http.StatusNotFound, c.Writer.Status())
}

// ─── annotate ────────────────────────────────────────────────────────────────

func TestAnnotateMaterialPinsCurrentArtifactVersion(t *testing.T) {
	h, _, annotations, _ := researchHandlerEnv(types.TaskAccessCollaborator)
	c, rec := researchContext(http.MethodPost, "/api/v1/workbench/executions/run-1/annotations",
		`{"material_id":"m1:0","base_version":"9a2f1c3d4e5f6a7b","body":"结论第三段缺引用"}`)
	h.AnnotateMaterial(c)

	require.Equal(t, http.StatusCreated, c.Writer.Status(), rec.Body.String())
	require.Len(t, annotations.created, 1)
	a := annotations.created[0]
	require.Equal(t, "m1:0", a.MaterialID)
	require.Equal(t, "9a2f1c3d4e5f6a7b", a.BaseVersion)
	require.Equal(t, "sess-1", a.SessionID)
	require.Equal(t, "u1", a.AuthorID)
	require.Contains(t, rec.Body.String(), `"annotation_id"`)
}

func TestAnnotateMaterialRejectsCollaboratorRoleForViewer(t *testing.T) {
	// viewer 只有只读角色：批注是评论性写入，TaskRoleCanRun(viewer)=false → 403。
	h, annotations, _, _ := researchHandlerEnv(types.TaskAccessViewer)
	c, _ := researchContext(http.MethodPost, "/api/v1/workbench/executions/run-1/annotations",
		`{"material_id":"m1:0","base_version":"9a2f1c3d4e5f6a7b","body":"x"}`)
	h.AnnotateMaterial(c)
	require.Equal(t, http.StatusForbidden, c.Writer.Status())
	require.Empty(t, annotations.created)
}

func TestAnnotateMaterialRejectsStaleBaseVersion(t *testing.T) {
	h, annotations, _, _ := researchHandlerEnv(types.TaskAccessOwner)
	// base_version 与当前版本身份不一致 → 409，防审计链断裂（Review Focus 2）。
	c, rec := researchContext(http.MethodPost, "/api/v1/workbench/executions/run-1/annotations",
		`{"material_id":"m1:0","base_version":"stale-version-0001","body":"x"}`)
	h.AnnotateMaterial(c)
	require.Equal(t, http.StatusConflict, c.Writer.Status())
	require.Contains(t, rec.Body.String(), "annotation_base_version_conflict")
	require.Empty(t, annotations.created)
}

func TestAnnotateMaterialUnknownMaterialIsUniform404(t *testing.T) {
	h, annotations, _, _ := researchHandlerEnv(types.TaskAccessOwner)
	c, _ := researchContext(http.MethodPost, "/api/v1/workbench/executions/run-1/annotations",
		`{"material_id":"m9:9","base_version":"9a2f1c3d4e5f6a7b","body":"x"}`)
	h.AnnotateMaterial(c)
	require.Equal(t, http.StatusNotFound, c.Writer.Status())
	require.Empty(t, annotations.created)
}

func TestListAnnotationsReadableByGrantedViewer(t *testing.T) {
	h, _, annotations, _ := researchHandlerEnv(types.TaskAccessViewer)
	annotations.created = append(annotations.created, types.TaskArtifactAnnotation{
		TenantID: 1, ID: "an1", SessionID: "sess-1", RunID: "run-1",
		MaterialID: "m1:0", BaseVersion: "9a2f1c3d4e5f6a7b", Body: "b", AuthorID: "u3",
	})
	c, rec := researchContext(http.MethodGet, "/api/v1/workbench/executions/run-1/annotations", "")
	h.ListAnnotations(c)
	require.Equal(t, http.StatusOK, c.Writer.Status())
	require.Contains(t, rec.Body.String(), `"base_version":"9a2f1c3d4e5f6a7b"`)
}
