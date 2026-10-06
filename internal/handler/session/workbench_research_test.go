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
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	apperrors "github.com/Tencent/WeKnora/internal/errors"
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

func (s *researchStoreStub) CreateDelegation(_ context.Context, d *types.TaskResearchDelegation) error {
	// Same contract as the production store: GORM writes the persisted
	// timestamps back through the pointer target.
	now := time.Now().UTC()
	d.CreatedAt, d.UpdatedAt = now, now
	s.created = append(s.created, *d)
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

// pageByKeyset mirrors the production keyset semantics on a pre-sorted
// slice: resume strictly after the cursor row; an unknown cursor is the
// uniform empty page (the SQL subselect yields NULL → no rows).
func pageByKeyset[T any](sorted []T, cursor string, limit int, idOf func(T) string) []T {
	start := 0
	if cursor != "" {
		start = -1
		for i := range sorted {
			if idOf(sorted[i]) == cursor {
				start = i + 1
				break
			}
		}
		if start == -1 {
			return nil
		}
	}
	if start >= len(sorted) {
		return nil
	}
	end := start + limit
	if end > len(sorted) {
		end = len(sorted)
	}
	return sorted[start:end]
}

func (s *researchStoreStub) ListDelegationsBySession(_ context.Context, tenantID uint64, sessionID string, limit int, cursor string) ([]types.TaskResearchDelegation, error) {
	all := make([]types.TaskResearchDelegation, 0, len(s.created))
	for _, d := range s.created {
		if d.TenantID == tenantID && d.SessionID == sessionID {
			all = append(all, d)
		}
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].CreatedAt.Equal(all[j].CreatedAt) {
			return all[i].ID < all[j].ID
		}
		return all[i].CreatedAt.Before(all[j].CreatedAt)
	})
	return pageByKeyset(all, cursor, limit, func(d types.TaskResearchDelegation) string { return d.ID }), nil
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

func (s *annotationStoreStub) CreateAnnotation(_ context.Context, a *types.TaskArtifactAnnotation) error {
	// Same contract as the production store: GORM writes the persisted
	// timestamps back through the pointer target.
	a.CreatedAt = time.Now().UTC()
	s.created = append(s.created, *a)
	return nil
}

func (s *annotationStoreStub) ListAnnotationsBySession(_ context.Context, tenantID uint64, sessionID string, limit int, cursor string) ([]types.TaskArtifactAnnotation, error) {
	all := make([]types.TaskArtifactAnnotation, 0, len(s.created))
	for _, a := range s.created {
		if a.TenantID == tenantID && a.SessionID == sessionID {
			all = append(all, a)
		}
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].CreatedAt.Equal(all[j].CreatedAt) {
			return all[i].ID < all[j].ID
		}
		return all[i].CreatedAt.Before(all[j].CreatedAt)
	})
	return pageByKeyset(all, cursor, limit, func(a types.TaskArtifactAnnotation) string { return a.ID }), nil
}

type sourceAuthorizerStub struct {
	denied map[string]bool
	// err, when set, is returned verbatim for every source: it simulates an
	// infrastructure failure (DB down) as opposed to a scope rejection.
	err error
}

func (s sourceAuthorizerStub) AuthorizeResearchSource(_ context.Context, _ uint64, kbID string) error {
	if s.err != nil {
		return s.err
	}
	if s.denied[kbID] {
		return fmt.Errorf("%w", ErrResearchSourceOutOfScope)
	}
	return nil
}

type accessResolverStub struct {
	role types.TaskAccessRole
	// err, when set, replaces the static role resolution (B5-F68 cases).
	err error
}

func (a accessResolverStub) ResolveTaskAccess(context.Context, types.Caller, string) (types.TaskAccess, error) {
	if a.err != nil {
		return types.TaskAccess{}, a.err
	}
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

// 最终审查发现 1：agent_id 曾是 dead wire 字段（声明即弃）。删除字段后，
// 旧客户端仍可能发送该键——gin 绑定必须忽略未知 JSON 键，委派行为不变。
func TestDelegateResearchIgnoresLegacyAgentIDKey(t *testing.T) {
	h, delegations, _, _ := researchHandlerEnv(types.TaskAccessOwner)
	c, rec := researchContext(http.MethodPost, "/api/v1/workbench/executions/run-1/research",
		`{"objective":"survey retrieval baselines","sources":["kb-1"],"agent_id":"agent-x"}`)
	h.DelegateResearch(c)

	require.Equal(t, http.StatusCreated, c.Writer.Status(), rec.Body.String())
	require.Len(t, delegations.created, 1)
	require.Equal(t, "survey retrieval baselines", delegations.created[0].Objective)
	require.Equal(t, `["kb-1"]`, delegations.created[0].SourcesJSON)
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

// 最终审查发现 2：base_version 曾未 trim 即与 artifactVersionOf 比对，
// 带空白的当前版本号会误得 409。修复后 trim 先于比对，落库值亦为干净版本号。
func TestAnnotateMaterialTrimsBaseVersionBeforeCompare(t *testing.T) {
	h, _, annotations, _ := researchHandlerEnv(types.TaskAccessOwner)
	c, rec := researchContext(http.MethodPost, "/api/v1/workbench/executions/run-1/annotations",
		`{"material_id":"m1:0","base_version":"  9a2f1c3d4e5f6a7b\n","body":"结论第三段缺引用"}`)
	h.AnnotateMaterial(c)

	require.Equal(t, http.StatusCreated, c.Writer.Status(), rec.Body.String())
	require.Len(t, annotations.created, 1)
	require.Equal(t, "9a2f1c3d4e5f6a7b", annotations.created[0].BaseVersion, "落库的 base_version 必须是 trim 后的当前版本身份")
}

// 最终审查发现 3：body 空值校验曾发生在 annotation 构造之后。修复后校验
// 先于构造，空白 body 仍 400 且零落库（fail closed 方向不变）。
func TestAnnotateMaterialRejectsBlankBodyBeforePersist(t *testing.T) {
	h, _, annotations, _ := researchHandlerEnv(types.TaskAccessOwner)
	c, rec := researchContext(http.MethodPost, "/api/v1/workbench/executions/run-1/annotations",
		`{"material_id":"m1:0","base_version":"9a2f1c3d4e5f6a7b","body":"   "}`)
	h.AnnotateMaterial(c)

	require.Equal(t, http.StatusBadRequest, c.Writer.Status())
	require.Contains(t, rec.Body.String(), "research_invalid_request")
	require.Empty(t, annotations.created, "空白 body 绝不能落库")
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

// ─── persisted timestamps (B5-F74/F75) ──────────────────────────────────────

// The 201 responses must echo the store-persisted created_at, not the
// handler-local zero value. The stub store simulates GORM's write-back
// through the pointer target (production: Create(d) fills CreatedAt on the
// pointed-to entity).
func TestDelegateResearchReturnsPersistedCreatedAt(t *testing.T) {
	h, _, _, _ := researchHandlerEnv(types.TaskAccessOwner)
	c, rec := researchContext(http.MethodPost, "/api/v1/workbench/executions/run-1/research",
		`{"objective":"survey retrieval baselines","sources":["kb-1"]}`)
	h.DelegateResearch(c)
	require.Equal(t, http.StatusCreated, c.Writer.Status())

	var body struct {
		Data struct {
			Delegation researchDelegationView `json:"delegation"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	created, err := time.Parse(time.RFC3339, body.Data.Delegation.CreatedAt)
	require.NoError(t, err)
	require.WithinDuration(t, time.Now().UTC(), created, 5*time.Second,
		"created_at 必须是落库回写时刻，绝非零值 0001-01-01T00:00:00Z（B5-F74）")
}

func TestAnnotateMaterialReturnsPersistedCreatedAt(t *testing.T) {
	h, _, _, _ := researchHandlerEnv(types.TaskAccessOwner)
	c, rec := researchContext(http.MethodPost, "/api/v1/workbench/executions/run-1/annotations",
		`{"material_id":"m1:0","base_version":"9a2f1c3d4e5f6a7b","body":"结论第三段缺引用"}`)
	h.AnnotateMaterial(c)
	require.Equal(t, http.StatusCreated, c.Writer.Status())

	var body struct {
		Data struct {
			Annotation researchAnnotationView `json:"annotation"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	created, err := time.Parse(time.RFC3339, body.Data.Annotation.CreatedAt)
	require.NoError(t, err)
	require.WithinDuration(t, time.Now().UTC(), created, 5*time.Second,
		"created_at 必须是落库回写时刻，绝非零值 0001-01-01T00:00:00Z（B5-F75）")
}

// ─── store failure classification (B5-F63/F76; F58/F68) ──────────────────────

// grantedRunErrStub misses on the owner path then fails with a raw
// infrastructure error on the granted fallback.
type grantedRunErrStub struct{ err error }

func (g grantedRunErrStub) GetRunForGrantedReader(_ context.Context, _ uint64, _, _ string) (agentruntime.Run, error) {
	return agentruntime.Run{}, g.err
}

// A storage failure must surface as 500 (research_backend), never be
// mislabeled as a miss (404 run_not_found / research_not_found) or a scope
// rejection (400 research_source_out_of_task_grant) — DB downtime must not
// impersonate business states (B5-F63, B5-F76, B5-F58).
func TestResearchReadsReturn500OnStoreFailure(t *testing.T) {
	dbDown := errors.New("db down")

	// 1) resolveReadable: GetOwnedRun infrastructure failure → 500, not 404.
	ownerRuns := researchRunStub()
	ownerRuns.err = dbDown
	h, _, _, _ := researchHandler(ownerRuns, grantedRunStub{}, types.TaskAccessOwner)
	c, rec := researchContext(http.MethodGet, "/api/v1/workbench/executions/run-1/research", "")
	h.ListResearch(c)
	require.Equal(t, http.StatusInternalServerError, c.Writer.Status(), rec.Body.String())
	require.Contains(t, rec.Body.String(), "research_backend")
	require.NotContains(t, rec.Body.String(), "run_not_found")

	// 2) Granted fallback infrastructure failure → 500, not 404. The owner
	// predicate misses for u2 (stub only admits u1) so the granted reader is
	// engaged with a raw db error.
	h2, _, _, _ := researchHandler(researchRunStub(), grantedRunErrStub{err: dbDown}, types.TaskAccessOwner)
	c2, rec2 := researchContext(http.MethodGet, "/api/v1/workbench/executions/run-1/annotations", "")
	c2.Request = c2.Request.WithContext(context.WithValue(c2.Request.Context(), types.UserIDContextKey, "u2"))
	h2.ListAnnotations(c2)
	require.Equal(t, http.StatusInternalServerError, c2.Writer.Status(), rec2.Body.String())
	require.NotContains(t, rec2.Body.String(), "run_not_found")

	// 3) CompleteResearch: GetDelegation infrastructure failure → 500, not 404.
	h3, delegations, _, _ := researchHandlerEnv(types.TaskAccessOwner)
	delegations.getByID = func(uint64, string) (types.TaskResearchDelegation, error) {
		return types.TaskResearchDelegation{}, dbDown
	}
	c3, rec3 := researchContext(http.MethodPost, "/api/v1/workbench/executions/run-1/research/delegation_id/summary", `{"summary":"s"}`)
	h3.CompleteResearch(c3)
	require.Equal(t, http.StatusInternalServerError, c3.Writer.Status(), rec3.Body.String())
	require.NotContains(t, rec3.Body.String(), "research_not_found", "DB 故障不得伪装成 miss")

	// 4) F58: AuthorizeResearchSource infrastructure failure → 500, not a
	// scope rejection 400, and nothing is persisted.
	delegations4 := newResearchStoreStub()
	h4 := NewWorkbenchResearchHandler(
		researchRunStub(), grantedRunStub{}, &artifactRefReaderStub{refs: researchRefs()},
		delegations4, &annotationStoreStub{},
		sourceAuthorizerStub{err: dbDown}, accessResolverStub{role: types.TaskAccessOwner},
	)
	c4, rec4 := researchContext(http.MethodPost, "/api/v1/workbench/executions/run-1/research",
		`{"objective":"x","sources":["kb-1"]}`)
	h4.DelegateResearch(c4)
	require.Equal(t, http.StatusInternalServerError, c4.Writer.Status(), rec4.Body.String())
	require.NotContains(t, rec4.Body.String(), "research_source_out_of_task_grant")
	require.Empty(t, delegations4.created)
}

// writeResearchError maps the task-access resolver's typed AppErrors:
// NotFound→404, BadRequest→400, infrastructure→500 (B5-F68).
func TestAnnotateMaterialClassifiesTaskAccessErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		code int
		body string
	}{
		{"not-found", apperrors.NewNotFoundError("task not found"), http.StatusNotFound, ""},
		{"bad-request", apperrors.NewBadRequestError("task id is required"), http.StatusBadRequest, ""},
		{"infra", errors.New("db down"), http.StatusInternalServerError, "research_backend"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := NewWorkbenchResearchHandler(
				researchRunStub(), grantedRunStub{}, &artifactRefReaderStub{refs: researchRefs()},
				newResearchStoreStub(), &annotationStoreStub{},
				sourceAuthorizerStub{}, accessResolverStub{err: tc.err},
			)
			c, rec := researchContext(http.MethodPost, "/api/v1/workbench/executions/run-1/annotations",
				`{"material_id":"m1:0","base_version":"9a2f1c3d4e5f6a7b","body":"x"}`)
			h.AnnotateMaterial(c)
			require.Equal(t, tc.code, c.Writer.Status(), rec.Body.String())
			if tc.body != "" {
				require.Contains(t, rec.Body.String(), tc.body)
			}
		})
	}
}

// ─── input bounds & pagination (B5-F64; F65/F66/F67) ─────────────────────────

func TestResearchInputBounds(t *testing.T) {
	overBody := strings.Repeat("好", maxAnnotationBodyRunes+1)
	overSummary := strings.Repeat("s", maxResearchSummaryRunes+1)

	// 1) F65: an oversized body is a 400 input-format verdict and must be
	// judged BEFORE business states (404 material miss, 409 stale version).
	h, _, annotations, _ := researchHandlerEnv(types.TaskAccessOwner)
	c, rec := researchContext(http.MethodPost, "/api/v1/workbench/executions/run-1/annotations",
		`{"material_id":"m-missing:9","base_version":"stale","body":"`+overBody+`"}`)
	h.AnnotateMaterial(c)
	require.Equal(t, http.StatusBadRequest, c.Writer.Status(), rec.Body.String())
	require.Contains(t, rec.Body.String(), "research_invalid_request")
	require.Empty(t, annotations.created)

	// Empty body with an unknown material: still the input 400, not 404.
	c2, rec2 := researchContext(http.MethodPost, "/api/v1/workbench/executions/run-1/annotations",
		`{"material_id":"m-missing:9","base_version":"stale","body":"  "}`)
	h.AnnotateMaterial(c2)
	require.Equal(t, http.StatusBadRequest, c2.Writer.Status(), rec2.Body.String())
	require.Empty(t, annotations.created)

	// A body at exactly the cap is accepted (boundary).
	c3, _ := researchContext(http.MethodPost, "/api/v1/workbench/executions/run-1/annotations",
		`{"material_id":"m1:0","base_version":"9a2f1c3d4e5f6a7b","body":"`+strings.Repeat("好", maxAnnotationBodyRunes)+`"}`)
	h.AnnotateMaterial(c3)
	require.Equal(t, http.StatusCreated, c3.Writer.Status())

	// 2) F64: complete summary over the cap → 400.
	h2, delegations, _, _ := researchHandlerEnv(types.TaskAccessOwner)
	delegations.created = append(delegations.created, types.TaskResearchDelegation{
		TenantID: 1, ID: "d1", SessionID: "sess-1", ParentRunID: "run-1",
		Objective: "o", SourcesJSON: `["kb-1"]`, Status: types.TaskResearchAssigned, CreatedBy: "u1",
	})
	c4, rec4 := researchContext(http.MethodPost, "/api/v1/workbench/executions/run-1/research/delegation_id/summary",
		`{"summary":"`+overSummary+`"}`)
	h2.CompleteResearch(c4)
	require.Equal(t, http.StatusBadRequest, c4.Writer.Status(), rec4.Body.String())
	require.Contains(t, rec4.Body.String(), "research_invalid_request")
	require.Equal(t, types.TaskResearchAssigned, delegations.created[0].Status, "超限摘要绝不能落库")

	// 3) F66: an agent_id key in the request body is ignored (dead binding
	// removed from the contract); delegation still succeeds.
	h3, _, _, _ := researchHandlerEnv(types.TaskAccessOwner)
	c5, rec5 := researchContext(http.MethodPost, "/api/v1/workbench/executions/run-1/research",
		`{"objective":"x","sources":["kb-1"],"agent_id":"agent-9"}`)
	h3.DelegateResearch(c5)
	require.Equal(t, http.StatusCreated, c5.Writer.Status(), rec5.Body.String())
	require.NotContains(t, rec5.Body.String(), "agent_id")
}

func TestResearchListsPaginateByIDCursor(t *testing.T) {
	h, delegations, annotations, _ := researchHandlerEnv(types.TaskAccessOwner)
	for i := 0; i < 3; i++ {
		delegations.created = append(delegations.created, types.TaskResearchDelegation{
			TenantID: 1, ID: fmt.Sprintf("d%d", i), SessionID: "sess-1", ParentRunID: "run-1",
			Objective: "o", SourcesJSON: `["kb-1"]`, Status: types.TaskResearchAssigned, CreatedBy: "u1",
			CreatedAt: time.Unix(int64(i), 0).UTC(),
		})
		annotations.created = append(annotations.created, types.TaskArtifactAnnotation{
			TenantID: 1, ID: fmt.Sprintf("an%d", i), SessionID: "sess-1", RunID: "run-1",
			MaterialID: "m1:0", BaseVersion: "9a2f1c3d4e5f6a7b", Body: "b", AuthorID: "u3",
			CreatedAt: time.Unix(int64(i), 0).UTC(),
		})
	}

	// First page: 2 of 3 annotations plus a cursor.
	c, rec := researchContext(http.MethodGet, "/api/v1/workbench/executions/run-1/annotations?limit=2", "")
	h.ListAnnotations(c)
	require.Equal(t, http.StatusOK, c.Writer.Status(), rec.Body.String())
	var page struct {
		Data struct {
			Items      []researchAnnotationView `json:"items"`
			NextCursor string                   `json:"next_cursor"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &page))
	require.Len(t, page.Data.Items, 2)
	require.NotEmpty(t, page.Data.NextCursor, "还有余页时必须返回 next_cursor")

	// Second page follows the cursor and drains the remainder.
	c2, rec2 := researchContext(http.MethodGet, "/api/v1/workbench/executions/run-1/annotations?limit=2&cursor="+page.Data.NextCursor, "")
	h.ListAnnotations(c2)
	require.Equal(t, http.StatusOK, c2.Writer.Status(), rec2.Body.String())
	var page2 struct {
		Data struct {
			Items      []researchAnnotationView `json:"items"`
			NextCursor string                   `json:"next_cursor"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &page2))
	require.Len(t, page2.Data.Items, 1)
	require.Empty(t, page2.Data.NextCursor, "末页不再有 next_cursor")

	// The delegation list paginates the same way.
	c3, rec3 := researchContext(http.MethodGet, "/api/v1/workbench/executions/run-1/research?limit=2", "")
	h.ListResearch(c3)
	require.Equal(t, http.StatusOK, c3.Writer.Status(), rec3.Body.String())
	var dpage struct {
		Data struct {
			Items      []researchDelegationView `json:"items"`
			NextCursor string                   `json:"next_cursor"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec3.Body.Bytes(), &dpage))
	require.Len(t, dpage.Data.Items, 2)
	require.NotEmpty(t, dpage.Data.NextCursor)
}
