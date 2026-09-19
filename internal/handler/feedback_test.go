package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// stubFeedbackService records what each handler forwarded to the service.
type stubFeedbackService struct {
	interfaces.FeedbackService
	submitErr error

	submitCaller   types.Caller
	submitSession  string
	submitMessage  string
	submitRating   string
	submitComment  string
	removeCalled   bool
	removeMessage  string
	listSession    string
	listMyFeedback []types.MessageFeedback
}

func (s *stubFeedbackService) SubmitFeedback(
	_ context.Context, caller types.Caller, sessionID, messageID, rating, comment string,
) error {
	s.submitCaller, s.submitSession, s.submitMessage = caller, sessionID, messageID
	s.submitRating, s.submitComment = rating, comment
	return s.submitErr
}

func (s *stubFeedbackService) RemoveFeedback(
	_ context.Context, _ types.Caller, _, messageID string,
) error {
	s.removeCalled, s.removeMessage = true, messageID
	return nil
}

func (s *stubFeedbackService) ListMyFeedback(
	_ context.Context, _ types.Caller, sessionID string,
) ([]types.MessageFeedback, error) {
	s.listSession = sessionID
	return s.listMyFeedback, nil
}

func newFeedbackTestRouter(t *testing.T, svc interfaces.FeedbackService) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.ErrorHandler())
	h := &FeedbackHandler{FeedbackService: svc}
	r.POST("/messages/:session_id/:message_id/feedback", h.SubmitFeedback)
	r.DELETE("/messages/:session_id/:message_id/feedback", h.RemoveFeedback)
	r.GET("/messages/:session_id/feedback/mine", h.ListMyFeedback)
	return r
}

// 合法 like 提交 → 200 envelope，且 stub 收到 rating=like 与正确的
// session/message 透传。
func TestFeedbackHandler_SubmitPassesParams(t *testing.T) {
	svc := &stubFeedbackService{}
	router := newFeedbackTestRouter(t, svc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/messages/s1/m1/feedback",
		strings.NewReader(`{"rating":"like","comment":"good"}`))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(types.WithCaller(req.Context(), types.Caller{TenantID: 1, UserID: "u1", Role: types.TenantRoleContributor}))
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	var body struct {
		Success bool `json:"success"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.True(t, body.Success)

	assert.Equal(t, "like", svc.submitRating)
	assert.Equal(t, "good", svc.submitComment)
	assert.Equal(t, "s1", svc.submitSession)
	assert.Equal(t, "m1", svc.submitMessage)
	assert.Equal(t, "u1", svc.submitCaller.UserID)
}

// service 返回 ForbiddenError → 403，envelope success=false 且 error.code 存在。
func TestFeedbackHandler_SubmitMapsServiceError(t *testing.T) {
	svc := &stubFeedbackService{submitErr: errors.NewForbiddenError("not allowed to feedback on this session")}
	router := newFeedbackTestRouter(t, svc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/messages/s1/m1/feedback",
		strings.NewReader(`{"rating":"like"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusForbidden, w.Code, "body=%s", w.Body.String())
	var body struct {
		Success bool `json:"success"`
		Error   struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.False(t, body.Success)
	assert.NotZero(t, body.Error.Code)
	assert.NotEmpty(t, body.Error.Message)
}

// 非法 JSON body → 400。
func TestFeedbackHandler_SubmitRejectsInvalidBody(t *testing.T) {
	svc := &stubFeedbackService{}
	router := newFeedbackTestRouter(t, svc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/messages/s1/m1/feedback",
		strings.NewReader(`{"rating":`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
	assert.Contains(t, w.Body.String(), "invalid request body")
}

// 回显端点透传 session_id 并以 data 数组返回调用者自己的反馈行。
func TestFeedbackHandler_ListMineReturnsData(t *testing.T) {
	svc := &stubFeedbackService{listMyFeedback: []types.MessageFeedback{
		{ID: 1, TenantID: 1, UserID: "u1", SessionID: "s1", MessageID: "m1", Rating: types.FeedbackRatingLike},
	}}
	router := newFeedbackTestRouter(t, svc)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/messages/s1/feedback/mine", nil))

	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	var body struct {
		Success bool                    `json:"success"`
		Data    []types.MessageFeedback `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.True(t, body.Success)
	require.Len(t, body.Data, 1)
	assert.Equal(t, types.FeedbackRatingLike, body.Data[0].Rating)
	assert.Equal(t, "s1", svc.listSession)
}
