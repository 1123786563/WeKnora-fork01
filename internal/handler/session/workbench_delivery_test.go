package session

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/Tencent/WeKnora/internal/modules/codedelivery"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type deliveryRunsStub struct{}

func (deliveryRunsStub) GetOwnedRun(ctx context.Context, tenantID uint64, ownerID, runID string) (runtime.Run, error) {
	if tenantID == 1 && ownerID == "u1" && runID == "run-1" {
		return runtime.Run{Key: runtime.RunKey{TenantID: tenantID, RunID: runID}, SessionID: "s-1", Owner: ownerID}, nil
	}
	// 包级 resolveOwnedRun（strictOwnedRun）只把 agentruntime.ErrNotFound
	// 归类为 scope miss（→404），其余 error 记存储故障（→500）。
	return runtime.Run{}, runtime.ErrNotFound
}

type deliveryGrantedStub struct{}

func (deliveryGrantedStub) GetRunForGrantedReader(ctx context.Context, tenantID uint64, readerID, runID string) (runtime.Run, error) {
	if tenantID == 1 && readerID == "u2" && runID == "run-1" {
		return runtime.Run{Key: runtime.RunKey{TenantID: tenantID, RunID: runID}, SessionID: "s-1", Owner: "u1"}, nil
	}
	return runtime.Run{}, runtime.ErrNotFound
}

type deliveryServiceStub struct {
	prepared    int
	readRuns    int
	prepErr     error
	dispatchErr error
	lastView    codedelivery.DeliveryView
	lastResolve codedelivery.DispatchInput
}

func (s *deliveryServiceStub) MaterializeBaseline(ctx context.Context, in codedelivery.BaselineInput) (codedelivery.BaselineReceipt, error) {
	return codedelivery.BaselineReceipt{Files: 2, Root: "/workspace/octocat/hello"}, nil
}

func (s *deliveryServiceStub) PrepareDelivery(ctx context.Context, in codedelivery.PrepareInput) (codedelivery.DeliveryView, error) {
	s.prepared++
	if s.prepErr != nil {
		return codedelivery.DeliveryView{}, s.prepErr
	}
	s.lastView = codedelivery.DeliveryView{
		ID: "dlv-1", TaskID: "s-1", RunID: in.RunID, State: "prepared",
		Repo: in.Repo.String(), Branch: "weknora/task/s-1",
		ActionID: "act-1", ActionState: "awaiting_approval", Digest: "d1", Files: 2,
	}
	return s.lastView, nil
}

func (s *deliveryServiceStub) DispatchDelivery(ctx context.Context, in codedelivery.DispatchInput) (codedelivery.DeliveryView, error) {
	if s.dispatchErr != nil {
		return codedelivery.DeliveryView{}, s.dispatchErr
	}
	return codedelivery.DeliveryView{ID: in.DeliveryID, State: "delivered", CommitSHA: "c1", PRNumber: 1, Approver: "u1"}, nil
}

func (s *deliveryServiceStub) ResolveDeliveryUnknown(ctx context.Context, in codedelivery.DispatchInput) (codedelivery.DeliveryView, error) {
	s.lastResolve = in
	return codedelivery.DeliveryView{ID: in.DeliveryID, State: "pushed"}, nil
}

func TestResolveDeliveryRequiresStrictOwnerConfirmationBody(t *testing.T) {
	svc := &deliveryServiceStub{}
	h := NewWorkbenchDeliveryHandler(deliveryRunsStub{}, deliveryGrantedStub{}, svc)
	for _, tc := range []struct {
		body      string
		status    int
		confirmed bool
	}{
		{"", http.StatusOK, false},
		{`{"confirm_no_matching_pr":true}`, http.StatusOK, true},
		{`{"confirm_no_matching_pr":false}`, http.StatusOK, false},
		{`{"confirm_no_matching_pr":true,"extra":1}`, http.StatusBadRequest, false},
		{`{`, http.StatusBadRequest, false},
		{`null`, http.StatusBadRequest, false},
	} {
		c, _ := deliveryContext(http.MethodPost, "/api/v1/workbench/executions/run-1/delivery/dlv-1/resolve", tc.body, "u1")
		c.Params = gin.Params{{Key: "run_id", Value: "run-1"}, {Key: "delivery_id", Value: "dlv-1"}}
		h.ResolveDeliveryUnknown(c)
		require.Equal(t, tc.status, c.Writer.Status())
		if tc.status == http.StatusOK {
			require.Equal(t, tc.confirmed, svc.lastResolve.ConfirmNoMatchingPR)
		}
	}
}

func (s *deliveryServiceStub) GetDeliveryForRun(ctx context.Context, tenantID uint64, runID string) (codedelivery.DeliveryView, error) {
	s.readRuns++
	return codedelivery.DeliveryView{
		ID: "dlv-1", TaskID: "s-1", RunID: runID, State: "delivered",
		Repo: "octocat/hello", Branch: "weknora/task/s-1", CommitSHA: "c1234567890abcdef",
		PRNumber: 7, PRURL: "https://github.com/octocat/hello/pull/7",
		RemoteLogin: "octocat", ActionID: "act-1", ActionState: "succeeded",
		Digest: "d1", Approver: "u1", Files: 2,
	}, nil
}

func deliveryContext(method, target, body, userID string) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(method, target, strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	ctx := c.Request.Context()
	ctx = context.WithValue(ctx, types.TenantIDContextKey, uint64(1))
	ctx = context.WithValue(ctx, types.UserIDContextKey, userID)
	c.Request = c.Request.WithContext(ctx)
	return c, recorder
}

func TestDeliveryPrepareOwnerOnly(t *testing.T) {
	svc := &deliveryServiceStub{}
	h := NewWorkbenchDeliveryHandler(deliveryRunsStub{}, deliveryGrantedStub{}, svc)

	// owner：201 + 交付视图（审批锚点字段可见）。
	c, rec := deliveryContext(http.MethodPost, "/api/v1/workbench/executions/run-1/delivery",
		`{"connection_id":"conn-gh","repo":"octocat/hello","baseline_sha":"`+"b0000000000000000000000000000000000000000"+`","commit_message":"m","pr_title":"t"}`, "u1")
	c.Params = gin.Params{{Key: "run_id", Value: "run-1"}}
	h.PrepareDelivery(c)
	require.Equal(t, http.StatusCreated, c.Writer.Status())
	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Delivery struct {
				ID          string `json:"id"`
				State       string `json:"state"`
				Branch      string `json:"branch"`
				ActionState string `json:"action_state"`
				Digest      string `json:"digest"`
			} `json:"delivery"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.True(t, body.Success)
	require.Equal(t, "prepared", body.Data.Delivery.State)
	require.Equal(t, "awaiting_approval", body.Data.Delivery.ActionState)

	// 非 owner（u2 有 grant 也只是读权限）：404，不产生交付。
	c, _ = deliveryContext(http.MethodPost, "/api/v1/workbench/executions/run-1/delivery",
		`{"connection_id":"conn-gh","repo":"octocat/hello","baseline_sha":"`+"b0000000000000000000000000000000000000000"+`","commit_message":"m","pr_title":"t"}`, "u2")
	c.Params = gin.Params{{Key: "run_id", Value: "run-1"}}
	h.PrepareDelivery(c)
	require.Equal(t, http.StatusNotFound, c.Writer.Status())
	require.Equal(t, 1, svc.prepared)
}

func TestDeliveryReadOpenToGrantedViewer(t *testing.T) {
	svc := &deliveryServiceStub{}
	h := NewWorkbenchDeliveryHandler(deliveryRunsStub{}, deliveryGrantedStub{}, svc)

	c, rec := deliveryContext(http.MethodGet, "/api/v1/workbench/executions/run-1/delivery", "", "u2")
	c.Params = gin.Params{{Key: "run_id", Value: "run-1"}}
	h.GetDelivery(c)
	require.Equal(t, http.StatusOK, c.Writer.Status())
	var body struct {
		Data struct {
			Delivery struct {
				CommitSHA   string `json:"commit_sha"`
				PRURL       string `json:"pr_url"`
				RemoteLogin string `json:"remote_login"`
				Approver    string `json:"approver"`
			} `json:"delivery"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, "c1234567890abcdef", body.Data.Delivery.CommitSHA)
	require.Equal(t, "u1", body.Data.Delivery.Approver)

	// 无 grant 的第三者：404。
	c, _ = deliveryContext(http.MethodGet, "/api/v1/workbench/executions/run-1/delivery", "", "u3")
	c.Params = gin.Params{{Key: "run_id", Value: "run-1"}}
	h.GetDelivery(c)
	require.Equal(t, http.StatusNotFound, c.Writer.Status())
}

func TestDeliveryDispatchOwnerOnlyAndBaselineMaterializes(t *testing.T) {
	svc := &deliveryServiceStub{}
	h := NewWorkbenchDeliveryHandler(deliveryRunsStub{}, deliveryGrantedStub{}, svc)

	c, rec := deliveryContext(http.MethodPost, "/api/v1/workbench/executions/run-1/baseline",
		`{"connection_id":"conn-gh","repo":"octocat/hello","baseline_sha":"`+"b0000000000000000000000000000000000000000"+`"}`, "u1")
	c.Params = gin.Params{{Key: "run_id", Value: "run-1"}}
	h.MaterializeBaseline(c)
	require.Equal(t, http.StatusCreated, c.Writer.Status())

	c, rec = deliveryContext(http.MethodPost, "/api/v1/workbench/executions/run-1/delivery/dlv-1/dispatch", "", "u2")
	c.Params = gin.Params{{Key: "run_id", Value: "run-1"}, {Key: "delivery_id", Value: "dlv-1"}}
	h.DispatchDelivery(c)
	require.Equal(t, http.StatusNotFound, c.Writer.Status())

	c, rec = deliveryContext(http.MethodPost, "/api/v1/workbench/executions/run-1/delivery/dlv-1/dispatch", "", "u1")
	c.Params = gin.Params{{Key: "run_id", Value: "run-1"}, {Key: "delivery_id", Value: "dlv-1"}}
	h.DispatchDelivery(c)
	require.Equal(t, http.StatusOK, c.Writer.Status())
	require.Contains(t, rec.Body.String(), "\"state\":\"delivered\"")
}

// 错误分类表（最终修复轮发现 2/3）：输入类错误 400、可证未出网的前置门
// 拒绝 409、请求构建失败 500（非 502 provider_unreachable）——各自独立成
// 码，不再统统掉进 500 code_delivery_failed。
func TestDeliveryErrorClassificationTable(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"invalid material", codedelivery.ErrInvalidMaterial, http.StatusBadRequest, "code_delivery_invalid_material"},
		{"invalid branch", codedelivery.ErrInvalidBranch, http.StatusBadRequest, "code_delivery_invalid_material"},
		{"invalid baseline", codedelivery.ErrInvalidBaselineSHA, http.StatusBadRequest, "code_delivery_invalid_material"},
		{"invalid repo ref", codedelivery.ErrRepoRefInvalid, http.StatusBadRequest, "code_delivery_invalid_material"},
		{"baseline too large", codedelivery.ErrBaselineTooLarge, http.StatusBadRequest, "code_delivery_invalid_material"},
		{"dispatch rejected pre-send", codedelivery.ErrDeliveryDispatchRejected, http.StatusConflict, "code_delivery_dispatch_rejected"},
		{"unsupported provider", codedelivery.ErrUnsupportedProvider, http.StatusBadRequest, "code_delivery_unsupported_provider"},
		{"request build failure", codedelivery.ErrGitHubRequestInvalid, http.StatusInternalServerError, "code_delivery_request_invalid"},
		{"provider refused", &codedelivery.GitHubAPIError{Status: 422, Endpoint: "POST /pulls"}, http.StatusBadGateway, "code_delivery_provider_refused"},
		{"provider unobservable", codedelivery.ErrGitHubTransport, http.StatusBadGateway, "code_delivery_provider_unreachable"},
		{"unknown failure", context.DeadlineExceeded, http.StatusInternalServerError, "code_delivery_failed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := &deliveryServiceStub{prepErr: tc.err, dispatchErr: tc.err}
			h := NewWorkbenchDeliveryHandler(deliveryRunsStub{}, deliveryGrantedStub{}, svc)

			c, rec := deliveryContext(http.MethodPost, "/api/v1/workbench/executions/run-1/delivery",
				`{"connection_id":"conn-gh","repo":"octocat/hello","baseline_sha":"b0000000000000000000000000000000000000000","commit_message":"m","pr_title":"t"}`, "u1")
			c.Params = gin.Params{{Key: "run_id", Value: "run-1"}}
			h.PrepareDelivery(c)
			require.Equal(t, tc.status, c.Writer.Status(), "%s: prepare status", tc.name)
			require.Contains(t, rec.Body.String(), "\"code\":\""+tc.code+"\"", "%s: prepare code", tc.name)

			c, rec = deliveryContext(http.MethodPost, "/api/v1/workbench/executions/run-1/delivery/dlv-1/dispatch", "", "u1")
			c.Params = gin.Params{{Key: "run_id", Value: "run-1"}, {Key: "delivery_id", Value: "dlv-1"}}
			h.DispatchDelivery(c)
			require.Equal(t, tc.status, c.Writer.Status(), "%s: dispatch status", tc.name)
			require.Contains(t, rec.Body.String(), "\"code\":\""+tc.code+"\"", "%s: dispatch code", tc.name)
		})
	}
}
