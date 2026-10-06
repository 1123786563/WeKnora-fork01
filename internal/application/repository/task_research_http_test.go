package repository_test

// T17 (#47) end-to-end evidence over a fully migrated sqlite database and
// real stores/handlers — no mocked service. AC1: read-only delegations run
// in PARALLEL while the single write slot stays exclusive, out-of-scope
// sources are rejected before any durable write, and the grants surface is
// not widened by delegation. AC2: annotations pin the current artifact
// version identity; a revision (a NEW artifact message) yields a NEW version
// while the previously annotated version stays byte-identical.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/handler/session"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type researchE2E struct {
	db     *gorm.DB
	engine *gin.Engine
}

func newResearchE2E(t *testing.T) *researchE2E {
	t.Helper()
	db := openTaskGrantDB(t)
	runs := repository.NewAgentRunStore(db)
	_, err := runs.Admit(context.Background(), taskGrantAdmission())
	require.NoError(t, err)

	// Tenant-owned knowledge base: the delegation source gate's authority.
	require.NoError(t, db.Create(&types.KnowledgeBase{ID: "kb-1", TenantID: 1, Name: "research-kb"}).Error)

	// Real grant rows: the granted-read fallback (annotate by collaborator u3,
	// delegation list by viewer u2) must resolve through the live SQL JOIN.
	grantsStore := repository.NewTaskGrantStore(db)
	_, err = grantsStore.UpsertGrant(context.Background(), 1, "s1", "u2", types.TaskGrantRoleViewer, "u1")
	require.NoError(t, err)
	_, err = grantsStore.UpsertGrant(context.Background(), 1, "s1", "u3", types.TaskGrantRoleCollaborator, "u1")
	require.NoError(t, err)

	grantsSvc := service.NewTaskGrantService(
		repository.NewTaskGrantStore(db),
		repository.NewSessionRepository(db),
		repository.NewTenantMemberRepository(db),
	)
	messages := repository.NewMessageRepository(db)
	researchHandler := session.NewWorkbenchResearchHandler(
		runs, runs, messages,
		repository.NewTaskResearchStore(db),
		repository.NewTaskAnnotationStore(db),
		researchGateAdapter{kb: repository.NewKnowledgeBaseRepository(db)},
		grantsSvc,
	)
	grantsHandler := session.NewWorkbenchTaskGrantsHandler(grantsSvc)
	artifactHandler := session.NewWorkbenchArtifactHandler(runs, messages)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	v1 := r.Group("/api/v1")
	v1.POST("/workbench/executions/:run_id/research", researchHandler.DelegateResearch)
	v1.GET("/workbench/executions/:run_id/research", researchHandler.ListResearch)
	v1.POST("/workbench/executions/:run_id/research/:delegation_id/summary", researchHandler.CompleteResearch)
	v1.POST("/workbench/executions/:run_id/annotations", researchHandler.AnnotateMaterial)
	v1.GET("/workbench/executions/:run_id/annotations", researchHandler.ListAnnotations)
	v1.POST("/workbench/tasks/:task_id/grants", grantsHandler.Grant)
	v1.GET("/workbench/executions", session.NewWorkbenchListHandler(repository.NewWorkbenchListStore(db)).ListWorkbenchExecutions)
	v1.GET("/workbench/executions/:run_id/artifacts", artifactHandler.ListWorkbenchArtifacts)
	return &researchE2E{db: db, engine: r}
}

// researchGateAdapter mirrors the production container gate
// (container.researchSourceAuthorizer) against the real tenant-bound KB
// repository: a source the task's tenant does not own is rejected (sentinel
// → 400) before any durable write; infrastructure failures pass through so
// the handler can answer 500 (the production wiring is byte-equivalent in
// behavior, B5-F58).
type researchGateAdapter struct {
	kb interfaces.KnowledgeBaseRepository
}

func (a researchGateAdapter) AuthorizeResearchSource(ctx context.Context, tenantID uint64, kbID string) error {
	if _, err := a.kb.GetKnowledgeBaseByIDAndTenant(ctx, kbID, tenantID); err != nil {
		if errors.Is(err, repository.ErrKnowledgeBaseNotFound) {
			return fmt.Errorf("research source %q: %w", kbID, session.ErrResearchSourceOutOfScope)
		}
		return err
	}
	return nil
}

func (e *researchE2E) do(t *testing.T, method, path, body, userID string, tenant uint64) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), types.TenantIDContextKey, tenant)
	ctx = context.WithValue(ctx, types.UserIDContextKey, userID)
	ctx = context.WithValue(ctx, types.TenantRoleContextKey, types.TenantRoleContributor)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	e.engine.ServeHTTP(w, req)
	return w
}

// seedArtifactMessage inserts one assistant message carrying one artifact so
// the artifact list projects a version identity derived from its digest.
func (e *researchE2E) seedArtifactMessage(t *testing.T, messageID, digest, fileName string) {
	t.Helper()
	artifacts := types.MessageArtifacts{{
		URL: "local://tenant/1/" + fileName, FileName: fileName, FileType: ".md",
		FileSize: 512, ContentHash: digest, CreatedAt: time.Now().UTC(),
	}}
	// SkipHooks: types.Message BeforeCreate unconditionally regenerates the ID,
	// which would break the (message_id, index) material addressing under test.
	require.NoError(t, e.db.Session(&gorm.Session{SkipHooks: true}).Create(&types.Message{
		ID: messageID, SessionID: "s1", Role: "assistant",
		Content: "revision output", IsCompleted: true,
	}).Error)
	// Artifacts persist into message_artifacts (migration 000023 made that table
	// the only source of truth); mirror what messageRepository.Create writes so
	// the seeded material is visible to the artifact list.
	rows := types.NewMessageArtifactRecords("s1", messageID, artifacts)
	require.NoError(t, e.db.Create(&rows).Error)
}

const researchDigestV1 = "1111111111111111111111111111111111111111111111111111111111111111"
const researchDigestV2 = "2222222222222222222222222222222222222222222222222222222222222222"

// AC1: parallel read-only delegations coexist with the exclusive write slot.
func TestTaskResearchEndToEndParallelReadOnlyDelegations(t *testing.T) {
	env := newResearchE2E(t)

	// Contrast (existing semantics, unchanged): a SECOND write admission on
	// the same session must fail on the single-write slot ...
	runs := repository.NewAgentRunStore(env.db)
	second := taskGrantAdmission()
	second.Key = agentruntime.RunKey{TenantID: 1, RunID: "r2"}
	second.RequestID = "q2"
	_, err := runs.Admit(context.Background(), second)
	require.ErrorIs(t, err, agentruntime.ErrRunActive, "同一 Task 的第二个写 Run 必须被单写槽拒绝（既有语义，本计划不放松）")

	// ... while TWO read-only research delegations are admitted in parallel
	// on the still-active write run: they never touch the slot.
	for i, objective := range []string{"survey A", "survey B"} {
		w := env.do(t, http.MethodPost, "/api/v1/workbench/executions/r1/research",
			`{"objective":"`+objective+`","sources":["kb-1"]}`, "u1", 1)
		require.Equal(t, http.StatusCreated, w.Code, "只读委派 #%d 必须与活跃写 Run 并行（AC1）: %s", i+1, w.Body.String())
	}
	w := env.do(t, http.MethodGet, "/api/v1/workbench/executions/r1/research", "", "u2", 1)
	require.Equal(t, http.StatusOK, w.Code, "granted viewer 读取委派列表")
	var list struct {
		Success bool `json:"success"`
		Data    struct {
			Items []struct {
				DelegationID string   `json:"delegation_id"`
				Sources      []string `json:"sources"`
				Status       string   `json:"status"`
			} `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &list))
	require.Len(t, list.Data.Items, 2, "两个只读委派共存")

	// Findings summary lands via the owner CAS; a replay conflicts.
	d1 := list.Data.Items[0].DelegationID
	w = env.do(t, http.MethodPost, "/api/v1/workbench/executions/r1/research/"+d1+"/summary",
		`{"summary":"two cited findings"}`, "u1", 1)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	w = env.do(t, http.MethodPost, "/api/v1/workbench/executions/r1/research/"+d1+"/summary",
		`{"summary":"replay"}`, "u1", 1)
	require.Equal(t, http.StatusConflict, w.Code, "完成的委派重放必须冲突（摘要不可变）")

	// The collaborator cannot write grants: delegation did NOT widen the
	// grants surface (owner-only stays owner-only).
	w = env.do(t, http.MethodPost, "/api/v1/workbench/tasks/s1/grants",
		`{"grantee_id":"u4","role":"viewer"}`, "u3", 1)
	require.Equal(t, http.StatusForbidden, w.Code, "协作者委派研究绝不等于可管理 task_grants（AC1）")
}

// AC1: out-of-scope sources are rejected before any durable write.
func TestTaskResearchEndToEndSourceOutsideTenantScope(t *testing.T) {
	env := newResearchE2E(t)

	w := env.do(t, http.MethodPost, "/api/v1/workbench/executions/r1/research",
		`{"objective":"probe","sources":["kb-unknown"]}`, "u1", 1)
	require.Equal(t, http.StatusBadRequest, w.Code, "租户外知识库不得成为委派源")
	require.Contains(t, w.Body.String(), "research_source_out_of_task_grant")

	// Cross-tenant id that happens to exist in tenant 2 — same rejection.
	require.NoError(t, env.db.Create(&types.KnowledgeBase{ID: "kb-t2", TenantID: 2, Name: "other-tenant-kb"}).Error)
	w = env.do(t, http.MethodPost, "/api/v1/workbench/executions/r1/research",
		`{"objective":"probe","sources":["kb-t2"]}`, "u1", 1)
	require.Equal(t, http.StatusBadRequest, w.Code, "委派源不得跨租户（AC1：不能借委派扩大 Task Grant）")

	// The uniform cross-tenant probe: tenant 2 sees no delegations of s1.
	w = env.do(t, http.MethodGet, "/api/v1/workbench/executions/r1/research", "", "u1", 2)
	require.Equal(t, http.StatusNotFound, w.Code, "跨租户探测统一 404")
}

// AC2: annotations pin the current version; a revision produces a NEW
// version while the previously annotated version stays identical.
func TestTaskResearchEndToEndAnnotationVersionImmutability(t *testing.T) {
	env := newResearchE2E(t)
	env.seedArtifactMessage(t, "m1", researchDigestV1, "report-v1.md")

	// Current version identity of m1:0 (first 16 hex of the digest).
	w := env.do(t, http.MethodGet, "/api/v1/workbench/executions/r1/artifacts", "", "u1", 1)
	require.Equal(t, http.StatusOK, w.Code)
	var artifacts struct {
		Data struct {
			Items []struct {
				ID      string `json:"id"`
				Version string `json:"version"`
				Digest  string `json:"digest"`
			} `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &artifacts))
	require.Len(t, artifacts.Data.Items, 1)
	v1 := artifacts.Data.Items[0].Version
	require.Equal(t, researchDigestV1[:16], v1)
	require.Equal(t, "m1:0", artifacts.Data.Items[0].ID)

	// Collaborator annotates v1 (owner or collaborator may comment).
	w = env.do(t, http.MethodPost, "/api/v1/workbench/executions/r1/annotations",
		`{"material_id":"m1:0","base_version":"`+v1+`","body":"结论第三段缺引用"}`, "u3", 1)
	require.Equal(t, http.StatusCreated, w.Code, "协作者批注 v1: %s", w.Body.String())

	// A stale base version conflicts: after v2 exists, annotating m1:0 with
	// a stale identity can never re-label the newer version.
	env.seedArtifactMessage(t, "m2", researchDigestV2, "report-v2.md")
	w = env.do(t, http.MethodPost, "/api/v1/workbench/executions/r1/annotations",
		`{"material_id":"m1:0","base_version":"deadbeefdeadbeef","body":"stale"}`, "u1", 1)
	require.Equal(t, http.StatusConflict, w.Code, "过期 base_version 必须 409（AC2）")
	require.Contains(t, w.Body.String(), "annotation_base_version_conflict")

	// The revision is a NEW artifact message (m2) with a NEW version identity;
	// the annotated v1 item keeps its id, version and digest byte-identically.
	w = env.do(t, http.MethodGet, "/api/v1/workbench/executions/r1/artifacts", "", "u1", 1)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &artifacts))
	require.Len(t, artifacts.Data.Items, 2, "修订产生新版本条目")
	byID := map[string]struct {
		Version string
		Digest  string
	}{}
	for _, item := range artifacts.Data.Items {
		byID[item.ID] = struct {
			Version string
			Digest  string
		}{item.Version, item.Digest}
	}
	require.Equal(t, researchDigestV1[:16], byID["m1:0"].Version, "已批注 v1 的版本身份保持不变（AC2）")
	require.Equal(t, researchDigestV1, byID["m1:0"].Digest, "已批注 v1 的内容摘要保持不变（AC2）")
	require.Equal(t, researchDigestV2[:16], byID["m2:0"].Version, "修订产出新版本身份")

	// The annotation still resolves to v1 and v1 only.
	w = env.do(t, http.MethodGet, "/api/v1/workbench/executions/r1/annotations", "", "u1", 1)
	require.Contains(t, w.Body.String(), `"`+v1+`"`, "批注仍钉在 v1 的版本身份上")
	require.NotContains(t, w.Body.String(), researchDigestV2[:16])
}
