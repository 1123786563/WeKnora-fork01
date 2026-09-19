# SP11 反馈+分析（Onyx 平台运营面对齐）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 新建 `message_feedback` 表与反馈 API、四组分析聚合端点、`/platform/analytics` recharts 图表页、chat 消息气泡 like/dislike 按钮（GAP-MATRIX P-4/P-5）。

**Architecture:** 后端按仓库既有分层（types → interfaces → repository → container → handler → routes），分析聚合走读时实时 SQL（PG `to_char`/sqlite `date()` 双方言分支），无独立分析表；反馈按钮经回调注入（packages/views 不依赖 api-client，宿主接线）。前端 api-client 新增 analytics 域与 chat.feedback，页面挂 `/platform/analytics`。

**Tech Stack:** Go（gin v1.12.0 + GORM + golang-migrate + dig 容器）、React 19 + TanStack Router + Tailwind v4 + recharts（新依赖，仅 apps/web）、node:test + tsx（前端测试，**不是 vitest**）。

**Spec:** `docs/superpowers/specs/2026-09-19-onyx-platform-parity-design.md`（第 4 节 SP11 部分；GAP-MATRIX P-4/P-5）

## Global Constraints

- Go 模块路径 `github.com/Tencent/WeKnora`；成功响应一律 `gin.H{"success": true, "data": ...}`；错误一律 `c.Error(errors.NewBadRequestError/NewForbiddenError/NewNotFoundError(...))` 交 ErrorHandler，不在 handler 里直接写错误响应。
- 迁移成对 up/down：PostgreSQL 加 `migrations/versioned/`（下一个编号 **000158**）；SQLite 方言加 `migrations/sqlite/`（下一个编号 **000079**），文件头注释 `-- SQLite dialect of PG 000158`。
- 新路由必须经 `g.apiKeyGroup(...)` 声明 API-key 策略，否则 X-API-Key 调用方默认 403（`/api/v1` 组级 fail-closed）。
- 时间范围 query 参数命名 `start_time`/`end_time`（仓库惯例，见 `internal/handler/knowledge.go:913`），解析支持 RFC3339Nano/RFC3339/`2006-01-02 15:04:05`/`2006-01-02`；缺省回看 30 天（UTC）。
- repository 测试用包内既有 helper `openRunTestDB`（见 `internal/application/repository/agent_run_test.go:29`，SQLite t.TempDir + 真实迁移；Postgres 子测试名带 `/postgres` 且需 `TRPC_TEST_POSTGRES_DSN`，未设置自动 Skip）。
- 前端测试一律 node:test：`node --import tsx --test`，运行 `pnpm test:web`（apps/web）或照根 package.json `test:shared` 模式。
- recharts 只加到 apps/web（`pnpm --filter @weknora/web add recharts`）；packages/views 不得引入。
- embed 渠道不展示反馈按钮（spec 明确）。
- 每个 commit 前核对 `git status --porcelain` 只暂存本任务文件（仓库纪律）。

---

### Task 1: message_feedback 迁移 + 模型 + FeedbackRepository

**Files:**
- Create: `migrations/versioned/000158_message_feedback.up.sql` / `.down.sql`
- Create: `migrations/sqlite/000079_message_feedback.up.sql` / `.down.sql`
- Create: `internal/types/message_feedback.go`
- Create: `internal/types/interfaces/feedback.go`
- Create: `internal/application/repository/feedback.go`
- Test: `internal/application/repository/feedback_test.go`
- Modify: `internal/container/container.go`（在 repository Provide 区，照 `NewMessageRepository` 附近加一行）

**Interfaces:**
- Produces: `interfaces.FeedbackRepository`（UpsertFeedback / RemoveFeedback / ListBySessionAndUser）与 `types.MessageFeedback`，Task 2 的 service 依赖它。

- [ ] **Step 1: 写 PG 迁移 up/down**

`migrations/versioned/000158_message_feedback.up.sql`：

```sql
DO $$ BEGIN RAISE NOTICE '[Migration 000158] message_feedback'; END $$;
CREATE TABLE IF NOT EXISTS message_feedback (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    user_id VARCHAR(512) NOT NULL,
    message_id VARCHAR(36) NOT NULL,
    session_id VARCHAR(36) NOT NULL DEFAULT '',
    rating VARCHAR(16) NOT NULL,
    comment TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_message_feedback UNIQUE (message_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_message_feedback_session ON message_feedback (session_id);
CREATE INDEX IF NOT EXISTS idx_message_feedback_tenant_time ON message_feedback (tenant_id, created_at);
```

`.down.sql`：

```sql
DROP TABLE IF EXISTS message_feedback;
```

- [ ] **Step 2: 写 SQLite 方言迁移**

`migrations/sqlite/000079_message_feedback.up.sql`：

```sql
-- SQLite dialect of PG 000158
CREATE TABLE IF NOT EXISTS message_feedback (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    user_id TEXT NOT NULL,
    message_id TEXT NOT NULL,
    session_id TEXT NOT NULL DEFAULT '',
    rating TEXT NOT NULL,
    comment TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_message_feedback UNIQUE (message_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_message_feedback_session ON message_feedback (session_id);
CREATE INDEX IF NOT EXISTS idx_message_feedback_tenant_time ON message_feedback (tenant_id, created_at);
```

`.down.sql` 同 PG 版（`DROP TABLE IF EXISTS message_feedback;`）。

- [ ] **Step 3: 写模型与常量** `internal/types/message_feedback.go`

```go
package types

import "time"

const (
	FeedbackRatingLike    = "like"
	FeedbackRatingDislike = "dislike"
)

type MessageFeedback struct {
	ID        uint64    `json:"id" gorm:"primaryKey;autoIncrement"`
	TenantID  uint64    `json:"tenant_id" gorm:"index"`
	UserID    string    `json:"user_id" gorm:"type:varchar(512);not null"`
	MessageID string    `json:"message_id" gorm:"type:varchar(36);not null"`
	SessionID string    `json:"session_id" gorm:"type:varchar(36);not null;default:''"`
	Rating    string    `json:"rating" gorm:"type:varchar(16);not null"`
	Comment   string    `json:"comment" gorm:"type:text;not null;default:''"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (MessageFeedback) TableName() string { return "message_feedback" }

func IsValidFeedbackRating(rating string) bool {
	return rating == FeedbackRatingLike || rating == FeedbackRatingDislike
}
```

- [ ] **Step 4: 写接口** `internal/types/interfaces/feedback.go`

```go
package interfaces

import (
	"context"

	"github.com/Tencent/WeKnora/internal/types"
)

type FeedbackRepository interface {
	UpsertFeedback(ctx context.Context, fb *types.MessageFeedback) error
	RemoveFeedback(ctx context.Context, tenantID uint64, messageID, userID string) error
	ListBySessionAndUser(ctx context.Context, tenantID uint64, sessionID, userID string) ([]types.MessageFeedback, error)
}
```

- [ ] **Step 5: 写失败测试** `internal/application/repository/feedback_test.go`

```go
package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/Tencent/WeKnora/internal/types"
)

func TestFeedbackUpsertIsIdempotentPerUser(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			db := openRunTestDB(t, dialect)
			repo := NewFeedbackRepository(db)
			ctx := context.Background()
			fb1 := &types.MessageFeedback{TenantID: 1, UserID: "u1", MessageID: "m1", SessionID: "s1", Rating: types.FeedbackRatingLike}
			require.NoError(t, repo.UpsertFeedback(ctx, fb1))
			updated := &types.MessageFeedback{TenantID: 1, UserID: "u1", MessageID: "m1", SessionID: "s1", Rating: types.FeedbackRatingDislike, Comment: "bad"}
			require.NoError(t, repo.UpsertFeedback(ctx, updated)) // 同 (message,user) 应更新而非新增
			list, err := repo.ListBySessionAndUser(ctx, 1, "s1", "u1")
			require.NoError(t, err)
			require.Len(t, list, 1)
			require.Equal(t, types.FeedbackRatingDislike, list[0].Rating)
			// 另一用户互不影响
			require.NoError(t, repo.UpsertFeedback(ctx, &types.MessageFeedback{TenantID: 1, UserID: "u2", MessageID: "m1", SessionID: "s1", Rating: types.FeedbackRatingLike}))
			all, err := repo.ListBySessionAndUser(ctx, 1, "s1", "u2")
			require.NoError(t, err)
			require.Len(t, all, 1)
			// 移除
			require.NoError(t, repo.RemoveFeedback(ctx, 1, "m1", "u1"))
			after, err := repo.ListBySessionAndUser(ctx, 1, "s1", "u1")
			require.NoError(t, err)
			require.Len(t, after, 0)
		})
	}
}
```

注意：若 `openRunTestDB` 的签名是 `openRunTestDB(t, dialect)` 之外的形式（例如只接受 `t`），以 `agent_run_test.go:29` 实际签名为准调整调用。

- [ ] **Step 6: 运行确认失败**

Run: `go test ./internal/application/repository/ -run TestFeedbackUpsertIsIdempotentPerUser -v`
Expected: FAIL（`NewFeedbackRepository` 未定义）

- [ ] **Step 7: 写实现** `internal/application/repository/feedback.go`

```go
package repository

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

type feedbackRepository struct {
	db *gorm.DB
}

func NewFeedbackRepository(db *gorm.DB) interfaces.FeedbackRepository {
	return &feedbackRepository{db: db}
}

func (r *feedbackRepository) UpsertFeedback(ctx context.Context, fb *types.MessageFeedback) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "message_id"}, {Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"rating", "comment", "updated_at"}),
	}).Create(fb).Error
}

func (r *feedbackRepository) RemoveFeedback(ctx context.Context, tenantID uint64, messageID, userID string) error {
	return r.db.WithContext(ctx).
		Where("tenant_id = ? AND message_id = ? AND user_id = ?", tenantID, messageID, userID).
		Delete(&types.MessageFeedback{}).Error
}

func (r *feedbackRepository) ListBySessionAndUser(ctx context.Context, tenantID uint64, sessionID, userID string) ([]types.MessageFeedback, error) {
	var list []types.MessageFeedback
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND session_id = ? AND user_id = ?", tenantID, sessionID, userID).
		Find(&list).Error
	return list, err
}
```

- [ ] **Step 8: 注册 DI** — `internal/container/container.go` repository Provide 区（grep `NewMessageRepository` 的 must(container.Provide(...)) 行，紧邻添加）：

```go
must(container.Provide(repository.NewFeedbackRepository))
```

- [ ] **Step 9: 跑测试通过**

Run: `go test ./internal/application/repository/ -run TestFeedbackUpsertIsIdempotentPerUser -v`
Expected: PASS（sqlite 跑过；postgres 无 DSN 自动 Skip）

- [ ] **Step 10: Commit**

```bash
git add migrations/versioned/000158_message_feedback.* migrations/sqlite/000079_message_feedback.* internal/types/message_feedback.go internal/types/interfaces/feedback.go internal/application/repository/feedback.go internal/application/repository/feedback_test.go internal/container/container.go
git commit -m "feat(analytics): message_feedback 表与 repository（SP11 P-4）"
```

---

### Task 2: FeedbackService + 反馈 handler + 路由

**Files:**
- Modify: `internal/types/interfaces/feedback.go`（追加 service 接口）
- Create: `internal/application/service/feedback.go`
- Test: `internal/application/service/feedback_test.go`
- Create: `internal/handler/feedback.go`
- Test: `internal/handler/feedback_test.go`
- Modify: `internal/router/routes_chat.go`（RegisterMessageRoutes 内加三行路由）
- Modify: `internal/container/container.go`（Provide service + handler，照 MessageService/MessageHandler 注册链）

**Interfaces:**
- Consumes: Task 1 的 `interfaces.FeedbackRepository`；既有 `interfaces.SessionRepository`、`interfaces.MessageRepository`（按 id 取记录的方法名以 `internal/types/interfaces/session.go`、`message.go` 实际为准——执行时 grep `GetSession`/`GetMessageByID` 类方法；service 只需要"按 id+tenant 取单条"语义）。
- Produces: `interfaces.FeedbackService{SubmitFeedback, RemoveFeedback, ListMyFeedback}`；HTTP `POST/DELETE /api/v1/messages/:session_id/:message_id/feedback`、`GET /api/v1/messages/:session_id/feedback/mine`（Task 8 前端回显用）。

- [ ] **Step 1: 追加 service 接口**（`internal/types/interfaces/feedback.go`）

```go
type FeedbackService interface {
	SubmitFeedback(ctx context.Context, caller types.Caller, sessionID, messageID, rating, comment string) error
	RemoveFeedback(ctx context.Context, caller types.Caller, sessionID, messageID string) error
	ListMyFeedback(ctx context.Context, caller types.Caller, sessionID string) ([]types.MessageFeedback, error)
}
```

- [ ] **Step 2: 写失败 service 测试** `internal/application/service/feedback_test.go`

stub 三个 repository 接口（手写 struct 嵌接口，未用到的方法可嵌 nil），覆盖四条路径：

```go
package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/Tencent/WeKnora/internal/types"
)

func TestFeedbackServiceSubmitEnforcesOwnership(t *testing.T) {
	ctx := context.Background()
	owner := types.Caller{TenantID: 1, UserID: "owner-1", Role: types.TenantRoleMember}
	other := types.Caller{TenantID: 1, UserID: "other-1", Role: types.TenantRoleMember}
	admin := types.Caller{TenantID: 1, UserID: "other-1", Role: types.TenantRoleAdmin}
	svc := NewFeedbackService(&stubFeedbackRepo{}, &stubSessionRepo{session: &types.Session{ID: "s1", TenantID: 1, UserID: "owner-1"}}, &stubMessageRepo{message: &types.Message{ID: "m1", SessionID: "s1"}})

	// 非 owner 非 admin 拒绝
	require.Error(t, svc.SubmitFeedback(ctx, other, "s1", "m1", types.FeedbackRatingLike, ""))
	// owner 放行
	require.NoError(t, svc.SubmitFeedback(ctx, owner, "s1", "m1", types.FeedbackRatingLike, ""))
	// admin 放行（跨用户审计场景）
	require.NoError(t, svc.SubmitFeedback(ctx, admin, "s1", "m1", types.FeedbackRatingDislike, "x"))
	// 非法 rating 拒绝
	require.Error(t, svc.SubmitFeedback(ctx, owner, "s1", "m1", "meh", ""))
	// 消息不属于该 session 拒绝
	msgRepo := &stubMessageRepo{message: &types.Message{ID: "m2", SessionID: "other-session"}}
	svc2 := NewFeedbackService(&stubFeedbackRepo{}, &stubSessionRepo{session: &types.Session{ID: "s1", TenantID: 1, UserID: "owner-1"}}, msgRepo)
	require.Error(t, svc2.SubmitFeedback(ctx, owner, "s1", "m2", types.FeedbackRatingLike, ""))
}
```

stub 的具体字段/构造按 `types.SessionRepository`/`types.MessageRepository` 接口手写最小实现（只实现被调方法，其余 panic("not implemented")）。`types.TenantRoleMember` 等角色常量名以 `internal/types` 实际定义为准（grep `TenantRole`）。

- [ ] **Step 3: 运行确认失败**

Run: `go test ./internal/application/service/ -run TestFeedbackServiceSubmitEnforcesOwnership -v`
Expected: FAIL（NewFeedbackService 未定义）

- [ ] **Step 4: 写 service 实现** `internal/application/service/feedback.go`

```go
package service

import (
	"context"
	"errors"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	apperrors "github.com/Tencent/WeKnora/internal/errors"
)

type feedbackService struct {
	feedbackRepo interfaces.FeedbackRepository
	sessionRepo  interfaces.SessionRepository
	messageRepo  interfaces.MessageRepository
}

func NewFeedbackService(fr interfaces.FeedbackRepository, sr interfaces.SessionRepository, mr interfaces.MessageRepository) interfaces.FeedbackService {
	return &feedbackService{feedbackRepo: fr, sessionRepo: sr, messageRepo: mr}
}

// canFeedback 判定：session owner 本人，或租户 admin 及以上
func canFeedback(caller types.Caller, session *types.Session) bool {
	if caller.UserID != "" && session.UserID == caller.UserID {
		return true
	}
	return caller.Role.HasPermission(types.TenantRoleAdmin)
}

func (s *feedbackService) SubmitFeedback(ctx context.Context, caller types.Caller, sessionID, messageID, rating, comment string) error {
	if !types.IsValidFeedbackRating(rating) {
		return apperrors.NewBadRequestError("invalid rating, must be like or dislike")
	}
	session, err := s.loadSession(ctx, caller, sessionID)
	if err != nil {
		return err
	}
	if !canFeedback(caller, session) {
		return apperrors.NewForbiddenError("not allowed to feedback on this session")
	}
	msg, err := s.loadMessage(ctx, messageID)
	if err != nil {
		return err
	}
	if msg.SessionID != sessionID {
		return apperrors.NewBadRequestError("message does not belong to session")
	}
	return s.feedbackRepo.UpsertFeedback(ctx, &types.MessageFeedback{
		TenantID: caller.TenantID, UserID: caller.UserID,
		MessageID: messageID, SessionID: sessionID,
		Rating: rating, Comment: comment,
	})
}

func (s *feedbackService) RemoveFeedback(ctx context.Context, caller types.Caller, sessionID, messageID string) error {
	session, err := s.loadSession(ctx, caller, sessionID)
	if err != nil {
		return err
	}
	if !canFeedback(caller, session) {
		return apperrors.NewForbiddenError("not allowed to feedback on this session")
	}
	return s.feedbackRepo.RemoveFeedback(ctx, caller.TenantID, messageID, caller.UserID)
}

func (s *feedbackService) ListMyFeedback(ctx context.Context, caller types.Caller, sessionID string) ([]types.MessageFeedback, error) {
	return s.feedbackRepo.ListBySessionAndUser(ctx, caller.TenantID, sessionID, caller.UserID)
}
```

`loadSession`/`loadMessage` 用对应 repository 的按 id 查询方法（执行时以接口实际方法名实现；session 查询必须带 `tenant_id` 过滤，查不到返回 `apperrors.NewNotFoundError("session not found")`；message 同理）。

- [ ] **Step 5: 跑 service 测试通过**

Run: `go test ./internal/application/service/ -run TestFeedbackServiceSubmitEnforcesOwnership -v`
Expected: PASS

- [ ] **Step 6: 写 handler** `internal/handler/feedback.go`

```go
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

type FeedbackHandler struct {
	FeedbackService interfaces.FeedbackService
}

type feedbackRequest struct {
	Rating  string `json:"rating"`
	Comment string `json:"comment"`
}

func (h *FeedbackHandler) SubmitFeedback(c *gin.Context) {
	ctx := c.Request.Context()
	var req feedbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(errors.NewBadRequestError("invalid request body"))
		return
	}
	err := h.FeedbackService.SubmitFeedback(ctx, types.CallerFromContext(ctx),
		c.Param("session_id"), c.Param("message_id"), req.Rating, req.Comment)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{}})
}

func (h *FeedbackHandler) RemoveFeedback(c *gin.Context) {
	ctx := c.Request.Context()
	if err := h.FeedbackService.RemoveFeedback(ctx, types.CallerFromContext(ctx), c.Param("session_id"), c.Param("message_id")); err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{}})
}

func (h *FeedbackHandler) ListMyFeedback(c *gin.Context) {
	ctx := c.Request.Context()
	list, err := h.FeedbackService.ListMyFeedback(ctx, types.CallerFromContext(ctx), c.Param("session_id"))
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": list})
}
```

- [ ] **Step 7: 写 handler httptest 测试** `internal/handler/feedback_test.go`

照 `internal/handler/message_resource_urls_test.go:43` 的 `newResourceURLTestRouter` 模式：gin.TestMode + `middleware.ErrorHandler()` + 手写 stub `interfaces.FeedbackService`，断言 envelope：

```go
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

// 测试 1：合法 like 提交 → 200 且 stub 收到 rating=like
// 测试 2：service 返回 ForbiddenError → 403 且 body.success=false、error.code 存在
// 测试 3：非法 JSON body → 400
```

（stub 内用 channel/字段记录收到的参数，断言 session_id/message_id 透传正确。）

- [ ] **Step 8: 注册路由** — `internal/router/routes_chat.go` 的 `RegisterMessageRoutes` 内（`chatMessages` 组，紧邻现有 `DELETE /:session_id/:id`）：

```go
chatMessages.POST("/:session_id/:message_id/feedback", g.Viewer(), handler.SubmitFeedback)
chatMessages.DELETE("/:session_id/:message_id/feedback", g.Viewer(), handler.RemoveFeedback)
chatMessages.GET("/:session_id/feedback/mine", g.Viewer(), handler.ListMyFeedback)
```

注意 `RegisterMessageRoutes` 的第二参数当前是 `handler *handler.MessageHandler`——反馈路由挂同一函数需改签名为接收两个 handler（或独立 `RegisterFeedbackRoutes`）。**选独立函数**（改动面小）：新建注册函数并在 `internal/router/router.go` 的 v1 块调用（grep `RegisterMessageRoutes(v1` 处旁加一行），handler 经 router params 结构体传入（照 MessageHandler 的 params 字段+container Provide 链）。

- [ ] **Step 9: container 注册** — `internal/container/container.go`：

```go
must(container.Provide(service.NewFeedbackService))
must(container.Provide(handler.NewFeedbackHandler))
```

（`NewFeedbackHandler` 返回 `*handler.FeedbackHandler`；dig 自动解析三个 repository 依赖。）

- [ ] **Step 10: 全量编译+测试**

Run: `go build ./... && go test ./internal/handler/ ./internal/application/service/ -v`
Expected: BUILD OK，新测试 PASS

- [ ] **Step 11: Commit**

```bash
git add internal/types/interfaces/feedback.go internal/application/service/feedback.go internal/application/service/feedback_test.go internal/handler/feedback.go internal/handler/feedback_test.go internal/router/routes_chat.go internal/router/router.go internal/container/container.go
git commit -m "feat(analytics): 反馈提交/撤销/回显 API（SP11 P-4）"
```

---

### Task 3: AnalyticsRepository（四组聚合，双方言 SQL）

**Files:**
- Create: `internal/types/interfaces/analytics.go`
- Create: `internal/application/repository/analytics.go`
- Test: `internal/application/repository/analytics_test.go`

**Interfaces:**
- Produces: `interfaces.AnalyticsRepository{QueryTrend, ActiveUsers, ChannelSessions, AgentMessages}` 与四个点结构体，Task 4 handler 依赖。

- [ ] **Step 1: 写接口与点类型** `internal/types/interfaces/analytics.go`

```go
package interfaces

import (
	"context"
	"time"
)

type QueryTrendPoint struct {
	Date             string `json:"date"`
	Queries          int64  `json:"queries"`
	Likes            int64  `json:"likes"`
	Dislikes         int64  `json:"dislikes"`
}

type ActiveUsersPoint struct {
	Date        string `json:"date"`
	ActiveUsers int64  `json:"active_users"`
}

type ChannelSessionsPoint struct {
	Date     string `json:"date"`
	Source   string `json:"source"`
	Sessions int64  `json:"sessions"`
}

type AgentUsagePoint struct {
	Date        string `json:"date"`
	Messages    int64  `json:"messages"`
	UniqueUsers int64  `json:"unique_users"`
}

type AnalyticsRepository interface {
	QueryTrend(ctx context.Context, tenantID uint64, from, to time.Time) ([]QueryTrendPoint, error)
	ActiveUsers(ctx context.Context, tenantID uint64, from, to time.Time) ([]ActiveUsersPoint, error)
	ChannelSessions(ctx context.Context, tenantID uint64, from, to time.Time) ([]ChannelSessionsPoint, error)
	AgentMessages(ctx context.Context, tenantID uint64, agentID string, from, to time.Time) ([]AgentUsagePoint, error)
}
```

- [ ] **Step 2: 写失败测试** `internal/application/repository/analytics_test.go`

种子数据用原生 SQL 插入（照 `craft_usage_test.go` 模式）：2 个 session（s1 source=web owner=u1、s2 source=api owner=u2，同 tenant 1，跨日 created_at）、3 条 messages（s1 两条 user/assistant 各一跨日、s2 一条 user）、1 条 assistant 带 agent_id=a1、feedback 两条（like/dislike 各一）。断言：

```go
func TestAnalyticsAggregations(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			db := openRunTestDB(t, dialect)
			repo := NewAnalyticsRepository(db)
			ctx := context.Background()
			seedAnalyticsData(t, db) // 原生 INSERT sessions/messages/message_feedback（时间跨两天，UTC）
			from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
			to := time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)

			trend, err := repo.QueryTrend(ctx, 1, from, to)
			require.NoError(t, err)
			require.GreaterOrEqual(t, len(trend), 2) // 两天各一个点
			var totalQueries, totalLikes int64
			for _, p := range trend { totalQueries += p.Queries; totalLikes += p.Likes }
			require.Equal(t, int64(2), totalQueries) // 2 条 user 消息
			require.Equal(t, int64(1), totalLikes)

			users, err := repo.ActiveUsers(ctx, 1, from, to)
			require.NoError(t, err)
			// 断言两个日期点的 active_users 合计按日去重正确（u1、u2 各活跃）

			channels, err := repo.ChannelSessions(ctx, 1, from, to)
			require.NoError(t, err)
			// 断言出现 web 与 api 两个 source 各 1

			agentStats, err := repo.AgentMessages(ctx, 1, "a1", from, to)
			require.NoError(t, err)
			// 断言 messages=1 且 unique_users=1
		})
	}
}
```

`seedAnalyticsData` 内 sessions 必须填全 NOT NULL 列（执行时对照 `migrations/versioned` 中 sessions/messages 建表列；最简方式是 INSERT 指定列清单）。

- [ ] **Step 3: 运行确认失败**

Run: `go test ./internal/application/repository/ -run TestAnalyticsAggregations -v`
Expected: FAIL（NewAnalyticsRepository 未定义）

- [ ] **Step 4: 写实现** `internal/application/repository/analytics.go`

按 `r.db.Dialector.Name()` 分支（`"sqlite"` vs `"postgres"`），日期列 PG 用 `to_char(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD')`、sqlite 用 `date(created_at)`；PG 的 FILTER 子句在 sqlite 侧换成 `SUM(CASE WHEN ... THEN 1 ELSE 0 END)`。查询语义（join sessions 拿 tenant 归属，**软删除过滤 `s.deleted_at IS NULL`、`m.deleted_at IS NULL`**）：

```go
package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

type analyticsRepository struct {
	db *gorm.DB
}

func NewAnalyticsRepository(db *gorm.DB) interfaces.AnalyticsRepository {
	return &analyticsRepository{db: db}
}

func dayExpr(dialect, column string) string {
	if dialect == "sqlite" {
		return "date(" + column + ")"
	}
	return "to_char(" + column + " AT TIME ZONE 'UTC', 'YYYY-MM-DD')"
}

func (r *analyticsRepository) QueryTrend(ctx context.Context, tenantID uint64, from, to time.Time) ([]interfaces.QueryTrendPoint, error) {
	dialect := r.db.Dialector.Name()
	day := dayExpr(dialect, "m.created_at")
	likeExpr := countIf(dialect, "f.rating = 'like'")
	dislikeExpr := countIf(dialect, "f.rating = 'dislike'")
	queryExpr := countIf(dialect, "m.role = 'user'")
	// PG: COUNT(*) FILTER (WHERE cond)；sqlite: SUM(CASE WHEN cond THEN 1 ELSE 0 END)
	sql := `
SELECT ` + day + ` AS date,
       ` + queryExpr + ` AS queries,
       ` + likeExpr + ` AS likes,
       ` + dislikeExpr + ` AS dislikes
FROM messages m
JOIN sessions s ON s.id = m.session_id AND s.tenant_id = @tenant
LEFT JOIN message_feedback f ON f.message_id = m.id
WHERE m.created_at >= @from AND m.created_at < @to
  AND m.deleted_at IS NULL AND s.deleted_at IS NULL
GROUP BY 1 ORDER BY 1`
	var out []interfaces.QueryTrendPoint
	err := r.db.WithContext(ctx).Raw(sql, map[string]any{"tenant": tenantID, "from": from, "to": to}).Scan(&out).Error
	return out, err
}
```

`countIf`：

```go
func countIf(dialect, cond string) string {
	if dialect == "sqlite" {
		return "COALESCE(SUM(CASE WHEN " + cond + " THEN 1 ELSE 0 END), 0)"
	}
	return "COUNT(*) FILTER (WHERE " + cond + ")"
}
```

注意 PG 侧 join 放大问题：一条消息多用户反馈产生多行，`queries` 计数需去重——`queryExpr` 在 PG 侧改用 `COUNT(DISTINCT m.id) FILTER (WHERE m.role = 'user')`、sqlite 侧 `COUNT(DISTINCT CASE WHEN m.role='user' THEN m.id END)`。likes/dislikes 按 f 行计数天然正确。**`countIf` 的这两个变体直接内联写进 QueryTrend 的 SQL，不复用通用 helper**（其余三个查询无 join 放大）。

其余三查询（同样双言）：

```go
// ActiveUsers
SELECT <day(s 側不适用——用 m.created_at)> AS date, COUNT(DISTINCT s.user_id) AS active_users
FROM messages m JOIN sessions s ON s.id = m.session_id AND s.tenant_id = @tenant
WHERE m.role = 'user' AND m.created_at >= @from AND m.created_at < @to
  AND m.deleted_at IS NULL AND s.deleted_at IS NULL
GROUP BY 1 ORDER BY 1

// ChannelSessions
SELECT <day(s.created_at)> AS date, s.source AS source, COUNT(DISTINCT s.id) AS sessions
FROM sessions s
WHERE s.tenant_id = @tenant AND s.created_at >= @from AND s.created_at < @to AND s.deleted_at IS NULL
GROUP BY 1, 2 ORDER BY 1, 2

// AgentMessages
SELECT <day(m.created_at)> AS date, COUNT(DISTINCT m.id) AS messages, COUNT(DISTINCT s.user_id) AS unique_users
FROM messages m JOIN sessions s ON s.id = m.session_id AND s.tenant_id = @tenant
WHERE m.agent_id = @agent AND m.role = 'assistant' AND m.created_at >= @from AND m.created_at < @to
  AND m.deleted_at IS NULL AND s.deleted_at IS NULL
GROUP BY 1 ORDER BY 1
```

（`source` 为空的旧行返回 `source: ""`，前端归入 "web" 展示。）

- [ ] **Step 5: 跑测试通过**

Run: `go test ./internal/application/repository/ -run TestAnalyticsAggregations -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/types/interfaces/analytics.go internal/application/repository/analytics.go internal/application/repository/analytics_test.go
git commit -m "feat(analytics): 四组读时聚合查询（SP11 P-5）"
```

---

### Task 4: AnalyticsHandler + 路由 + container/router 接线

**Files:**
- Create: `internal/handler/analytics.go`
- Test: `internal/handler/analytics_test.go`
- Create: `internal/router/routes_analytics.go`
- Modify: `internal/router/router.go`（v1 块加调用 + params 结构体加 `AnalyticsHandler *handler.AnalyticsHandler`，照 MessageHandler 字段位置）
- Modify: `internal/container/container.go`（`must(container.Provide(repository.NewAnalyticsRepository))` + `must(container.Provide(handler.NewAnalyticsHandler))`）

**Interfaces:**
- Consumes: Task 3 的 `interfaces.AnalyticsRepository`。
- Produces: `GET /api/v1/analytics/queries|users|channels?start_time&end_time`、`GET /api/v1/analytics/agents/:agent_id?start_time&end_time`（响应 data 为点数组）。

- [ ] **Step 1: 写 handler** `internal/handler/analytics.go`

```go
package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

const analyticsDefaultLookbackDays = 30

type AnalyticsHandler struct {
	AnalyticsRepo interfaces.AnalyticsRepository
}

// parseAnalyticsRange 解析 start_time/end_time；缺省回看 30 天（UTC 日界）
func parseAnalyticsRange(c *gin.Context) (from, to time.Time, ok bool) {
	now := time.Now().UTC()
	from = now.AddDate(0, 0, -analyticsDefaultLookbackDays)
	to = now
	if raw := c.Query("start_time"); raw != "" {
		t, err := parseFilterTime(raw)
		if err != nil {
			c.Error(errors.NewBadRequestError("invalid start_time: " + err.Error()))
			return time.Time{}, time.Time{}, false
		}
		from = t.UTC()
	}
	if raw := c.Query("end_time"); raw != "" {
		t, err := parseFilterTime(raw)
		if err != nil {
			c.Error(errors.NewBadRequestError("invalid end_time: " + err.Error()))
			return time.Time{}, time.Time{}, false
		}
		to = t.UTC()
	}
	return from, to, true
}

func (h *AnalyticsHandler) tenantID(c *gin.Context) (uint64, bool) {
	tid := types.TenantIDFromContext(c.Request.Context())
	if tid == 0 {
		c.Error(errors.NewForbiddenError("tenant context required"))
		return 0, false
	}
	return tid, true
}

func (h *AnalyticsHandler) QueryTrend(c *gin.Context) {
	from, to, ok := parseAnalyticsRange(c)
	if !ok { return }
	tid, ok := h.tenantID(c)
	if !ok { return }
	data, err := h.AnalyticsRepo.QueryTrend(c.Request.Context(), tid, from, to)
	if err != nil { c.Error(errors.NewInternalServerError("analytics query failed")); return }
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}
// ActiveUsers / ChannelSessions 同构（替换 repo 方法）；
// AgentMessages 额外取 c.Param("agent_id")，空串报 400。
```

`parseFilterTime` 若为 knowledge.go 包内私有函数，不能跨包复用——在 analytics.go 内复制一份（多 layout 依次尝试，照 `internal/handler/knowledge.go:913` 的实现逐行复制并保持本地时区回退语义）。

- [ ] **Step 2: 写 httptest 测试** `internal/handler/analytics_test.go`

stub `interfaces.AnalyticsRepository`（记录收到的 tenantID/from/to），覆盖：默认 30 天范围（未传参时 to-from ≈ 30*24h）、`start_time=2026-09-01`（本地日期 layout 也接受）、非法 `start_time=abc` → 400。无 tenant context 的 403 分支通过手动注入 context 值测试（照仓库内 `types.TenantIDFromContext` 的 WithContext 写法，grep 现有测试用法）。

- [ ] **Step 3: 写路由** `internal/router/routes_analytics.go`

```go
package router

import (
	"github.com/gin-gonic/gin"
	"github.com/Tencent/WeKnora/internal/handler"
)

func RegisterAnalyticsRoutes(r *gin.RouterGroup, handler *handler.AnalyticsHandler, g *rbacGuards) {
	grp := g.apiKeyGroup(r.Group("/analytics", g.Admin()), apiKeyFullAccess())
	{
		grp.GET("/queries", handler.QueryTrend)
		grp.GET("/users", handler.ActiveUsers)
		grp.GET("/channels", handler.ChannelSessions)
		grp.GET("/agents/:agent_id", handler.AgentMessages)
	}
}
```

（`g.Admin()` 与 `apiKeyFullAccess()` 的签名以 `internal/router/rbac.go:195` 与 `routes_chat.go:22` 现有调用为准；若 apiKeyGroup 返回类型需 `.With(...)` 链式声明才能挂路由，照 routes_chat 的两步写法调整。）

- [ ] **Step 4: router.go 接线** — v1 块（grep `RegisterMessageRoutes(v1`）旁加 `RegisterAnalyticsRoutes(v1, params.AnalyticsHandler, rbacGuards)`；params 结构体加字段；container.go 加两个 Provide。

- [ ] **Step 5: 编译+全量 handler 测试**

Run: `go build ./... && go test ./internal/handler/ -run TestAnalytics -v`
Expected: PASS

- [ ] **Step 6: 手动冒烟（可选但推荐）** — 本地起后端（端口 8082，见仓库 dev 惯例），curl：

```bash
curl -s 'http://127.0.0.1:8082/api/v1/analytics/queries' -H "Authorization: Bearer <token>" | head -c 400
```
Expected: `{"success":true,"data":[]}`（空库空数组）

- [ ] **Step 7: Commit**

```bash
git add internal/handler/analytics.go internal/handler/analytics_test.go internal/router/routes_analytics.go internal/router/router.go internal/container/container.go
git commit -m "feat(analytics): admin 分析聚合端点与路由（SP11 P-5）"
```

---

### Task 5: contracts 类型与 parse 函数

**Files:**
- Create: `packages/contracts/src/analytics.ts`
- Create: `packages/contracts/src/analytics.test.ts`
- Modify: `packages/contracts/src/index.ts`（末尾 `export * from './analytics.ts';`）

**Interfaces:**
- Produces: `FeedbackRating`、`MessageFeedbackEntry`、`QueryTrendPoint`、`ActiveUsersPoint`、`ChannelSessionsPoint`、`AgentUsagePoint` 与 `parseQueryTrendResponse` 等四个 parse + `parseMessageFeedbackListResponse`，Task 6 依赖。

- [ ] **Step 1: 写类型与 parse** `packages/contracts/src/analytics.ts`

照 `packages/contracts/src/index.ts` 的既有写法（`ContractError` + 手写窄化，不用 zod）：

```ts
import { ContractError, envelope, type Envelope } from './index.ts'; // 若 envelope 未导出则复制 index.ts 内同名 helper 的用法

export type FeedbackRating = 'like' | 'dislike';

export interface MessageFeedbackEntry {
  id: number;
  message_id: string;
  session_id: string;
  rating: FeedbackRating;
  comment?: string;
  [key: string]: unknown;
}

export interface QueryTrendPoint { date: string; queries: number; likes: number; dislikes: number; }
export interface ActiveUsersPoint { date: string; active_users: number; }
export interface ChannelSessionsPoint { date: string; source: string; sessions: number; }
export interface AgentUsagePoint { date: string; messages: number; unique_users: number; }

function parsePointList<T>(value: unknown, parseItem: (item: unknown, path: string) => T, path: string): { items: T[] } {
  // 解 { success, data: [...] } envelope；data 非数组抛 ContractError
}

export function parseQueryTrendResponse(value: unknown): { items: QueryTrendPoint[] } { /* parsePointList + 逐字段 number/string 校验 */ }
export function parseActiveUsersResponse(value: unknown): { items: ActiveUsersPoint[] } { /* 同构 */ }
export function parseChannelSessionsResponse(value: unknown): { items: ChannelSessionsPoint[] } { /* 同构 */ }
export function parseAgentUsageResponse(value: unknown): { items: AgentUsagePoint[] } { /* 同构 */ }
export function parseMessageFeedbackListResponse(value: unknown): { items: MessageFeedbackEntry[] } { /* rating 必须是 like|dislike */ }
```

（`envelope` helper 若未从 index.ts 导出，则在本文件内实现一个 `successEnvelope(value)` 私有函数：校验 `value` 为对象且 `success === true` 后返回 `data`。**以 index.ts 实际导出为准**，执行时先读 index.ts 头部 helper 区。）

- [ ] **Step 2: 写测试** `packages/contracts/src/analytics.test.ts`（node:test）：

```ts
import assert from 'node:assert/strict';
import test from 'node:test';
import { parseQueryTrendResponse, parseMessageFeedbackListResponse } from './analytics.ts';

test('parseQueryTrendResponse unwraps envelope and validates fields', () => {
  const result = parseQueryTrendResponse({ success: true, data: [{ date: '2026-09-19', queries: 3, likes: 1, dislikes: 0 }] });
  assert.deepEqual(result.items, [{ date: '2026-09-19', queries: 3, likes: 1, dislikes: 0 }]);
});
test('parseMessageFeedbackListResponse rejects unknown rating', () => {
  assert.throws(() => parseMessageFeedbackListResponse({ success: true, data: [{ id: 1, message_id: 'm', session_id: 's', rating: 'meh' }] }));
});
test('parse rejects non-envelope', () => {
  assert.throws(() => parseQueryTrendResponse({ data: [] }));
});
```

- [ ] **Step 3: 运行**

Run: `cd /Users/wuyongjun/trea/WeKnora-fork01 && npx tsx --test packages/contracts/src/analytics.test.ts`
Expected: 3 PASS（若 contracts 有专属 test script，以根 package.json 为准）

- [ ] **Step 4: index.ts re-export + Commit**

```bash
git add packages/contracts/src/analytics.ts packages/contracts/src/analytics.test.ts packages/contracts/src/index.ts
git commit -m "feat(contracts): analytics 与 feedback 契约类型（SP11）"
```

---

### Task 6: api-client — chat.feedback 与 analytics 域

**Files:**
- Create: `packages/api-client/src/chat/feedback.ts`
- Create: `packages/api-client/src/analytics/index.ts`
- Test: `packages/api-client/src/analytics/index.test.ts`（照 `packages/api-client/src/wiki/pages.test.ts` 的既有模式）
- Modify: `packages/api-client/src/client.ts`（import + 实例化 + 挂载：`chat` 对象加 `feedback`；顶层加 `analytics`）
- Modify: `packages/api-client/src/index.ts`（barrel re-export，若该文件按域列导出）

**Interfaces:**
- Consumes: Task 5 的 parse 函数与类型。
- Produces: `client.chat.feedback.submit(sessionId, messageId, rating, comment?)` / `.remove(sessionId, messageId)` / `.mine(sessionId)`；`client.analytics.queryTrend(params)` / `.activeUsers(params)` / `.channelSessions(params)` / `.agentUsage(agentId, params)`。参数 `{ startTime?: string; endTime?: string; signal?: AbortSignal }`。

- [ ] **Step 1: 写 feedback api** `packages/api-client/src/chat/feedback.ts`

```ts
import { parseActionSuccessResponse, parseMessageFeedbackListResponse, type FeedbackRating } from '@weknora/contracts';
import type { ClientRequest } from '../client.ts';

function feedbackPath(sessionId: string, messageId?: string): string {
  const session = encodeURIComponent(sessionId);
  if (sessionId.trim() === '') throw new Error('sessionId must not be empty');
  if (messageId === undefined) return `/api/v1/messages/${session}/feedback/mine`;
  if (messageId.trim() === '') throw new Error('messageId must not be empty');
  return `/api/v1/messages/${session}/${encodeURIComponent(messageId)}/feedback`;
}

export function createFeedbackApi(request: (input: ClientRequest) => Promise<unknown>) {
  return {
    async submit(sessionId: string, messageId: string, rating: FeedbackRating, comment = '', signal?: AbortSignal): Promise<void> {
      parseActionSuccessResponse(await request({
        method: 'POST',
        path: feedbackPath(sessionId, messageId),
        body: { rating, comment },
        ...(signal === undefined ? {} : { signal }),
      }));
    },
    async remove(sessionId: string, messageId: string, signal?: AbortSignal): Promise<void> {
      parseActionSuccessResponse(await request({ method: 'DELETE', path: feedbackPath(sessionId, messageId), ...(signal === undefined ? {} : { signal }) }));
    },
    async mine(sessionId: string, signal?: AbortSignal) {
      return parseMessageFeedbackListResponse(await request({ method: 'GET', path: feedbackPath(sessionId), ...(signal === undefined ? {} : { signal }) }));
    },
  };
}
export type FeedbackApi = ReturnType<typeof createFeedbackApi>;
```

- [ ] **Step 2: 写 analytics 域** `packages/api-client/src/analytics/index.ts`

```ts
import {
  parseActiveUsersResponse, parseAgentUsageResponse, parseChannelSessionsResponse, parseQueryTrendResponse,
} from '@weknora/contracts';
import type { ClientRequest } from '../client.ts';

export interface AnalyticsRangeParams { startTime?: string; endTime?: string; signal?: AbortSignal; }

function rangeQuery(params: AnalyticsRangeParams): URLSearchParams {
  const query = new URLSearchParams();
  if (params.startTime !== undefined) query.set('start_time', params.startTime);
  if (params.endTime !== undefined) query.set('end_time', params.endTime);
  return query;
}

export function createAnalyticsApi(request: (input: ClientRequest) => Promise<unknown>) {
  return {
    async queryTrend(params: AnalyticsRangeParams = {}) {
      const suffix = rangeQuery(params).toString();
      return parseQueryTrendResponse(await request({ method: 'GET', path: `/api/v1/analytics/queries${suffix ? `?${suffix}` : ''}`, signal: params.signal }));
    },
    async activeUsers(params: AnalyticsRangeParams = {}) { /* 同构 /api/v1/analytics/users */ },
    async channelSessions(params: AnalyticsRangeParams = {}) { /* 同构 /api/v1/analytics/channels */ },
    async agentUsage(agentId: string, params: AnalyticsRangeParams = {}) {
      if (agentId.trim() === '') throw new Error('agentId must not be empty');
      const suffix = rangeQuery(params).toString();
      return parseAgentUsageResponse(await request({ method: 'GET', path: `/api/v1/analytics/agents/${encodeURIComponent(agentId)}${suffix ? `?${suffix}` : ''}`, signal: params.signal }));
    },
  };
}
export type AnalyticsApi = ReturnType<typeof createAnalyticsApi>;
```

（注释处为完全同构代码，执行时照 queryTrend 逐字展开，不留省略。）

- [ ] **Step 3: 写测试** `packages/api-client/src/analytics/index.test.ts`

stub request 断言 method/path/query（照 `packages/api-client/src/wiki/pages.test.ts` 模式）：

```ts
import assert from 'node:assert/strict';
import test from 'node:test';
import { createAnalyticsApi } from './index.ts';

test('analytics queryTrend maps params to start_time/end_time query', async () => {
  const calls: Array<{ method: string; path: string }> = [];
  const api = createAnalyticsApi(async (input) => {
    calls.push({ method: input.method, path: input.path });
    return { success: true, data: [] };
  });
  await api.queryTrend({ startTime: '2026-09-01', endTime: '2026-09-19' });
  assert.deepEqual(calls, [{ method: 'GET', path: '/api/v1/analytics/queries?start_time=2026-09-01&end_time=2026-09-19' }]);
});
test('agentUsage rejects empty agentId', async () => { /* assert.rejects */ });
```

- [ ] **Step 4: client.ts 装配** — 照 sessions 的三处：import 区、`const analytics = createAnalyticsApi(request);`、返回对象 `chat: { ..., feedback }` 与顶层 `analytics,`。

- [ ] **Step 5: 运行测试 + Commit**

Run: `npx tsx --test packages/api-client/src/analytics/index.test.ts`
```bash
git add packages/api-client/src/chat/feedback.ts packages/api-client/src/analytics/ packages/api-client/src/client.ts packages/api-client/src/index.ts
git commit -m "feat(api-client): feedback 与 analytics 域（SP11）"
```

---

### Task 7: 消息气泡反馈按钮（packages/views 层）

**Files:**
- Modify: `packages/views/src/chat/message-list.tsx`（props + FeedbackButtons + toolbar 渲染）
- Modify: `packages/views/src/chat/chat-copy.ts`（文案键）
- Modify: `packages/views/src/chat/page.tsx`（props 透传）
- Test: `packages/views/src/chat/message-list.test.tsx`（追加纯函数测试）

**Interfaces:**
- Produces: `MessageListProps` 新增三个可选回调/取值函数：`onRateMessage?: (messageId: string, rating: FeedbackRating) => void`、`onRemoveRating?: (messageId: string) => void`、`ratingOf?: (messageId: string) => FeedbackRating | undefined`。宿主不传即整组隐藏（与 `onBookmark` 同款能力开关惯例）。Task 8 的 apps/web 宿主消费。

- [ ] **Step 1: chat-copy.ts 加文案**（照现有 ChatCopy 键的中英文结构追加，执行时先看 `resolveChatCopy` 的表结构；键与文案）：

```
feedbackLikeTooltip: '赞' / 'Like'
feedbackDislikeTooltip: '踩' / 'Dislike'
feedbackRemoveTooltip: '撤销评价' / 'Remove rating'
```

- [ ] **Step 2: message-list.tsx 实现**

在 assistant toolbar（`wk-chat-answer-toolbar` 区，415-426 行）追加（`isAssistant && onRateMessage` 时渲染）：

```tsx
function FeedbackButtons({ copy, message, ratingOf, onRateMessage, onRemoveRating }: {
  copy: ChatCopy; message: ChatMessage;
  ratingOf?: (messageId: string) => FeedbackRating | undefined;
  onRateMessage?: (messageId: string, rating: FeedbackRating) => void;
  onRemoveRating?: (messageId: string) => void;
}) {
  const current = ratingOf?.(message.id);
  const toggle = (rating: FeedbackRating) => {
    if (current === rating) { onRemoveRating?.(message.id); return; }
    onRateMessage?.(message.id, rating);
  };
  return (<>
    <button type="button" className={`${ANSWER_TOOL_BUTTON} wk-chat-feedback-like`}
      aria-label={copy.feedbackLikeTooltip} title={copy.feedbackLikeTooltip}
      aria-pressed={current === 'like'} onClick={() => toggle('like')}>
      {/* 内联 SVG 大拇指向上，16x16 viewBox 0 0 16 16，风格照 CopyAnswerButton 的图标 */}
    </button>
    <button type="button" className={`${ANSWER_TOOL_BUTTON} wk-chat-feedback-dislike`}
      aria-label={copy.feedbackDislikeTooltip} title={copy.feedbackDislikeTooltip}
      aria-pressed={current === 'dislike'} onClick={() => toggle('dislike')}>
      {/* 内联 SVG 大拇指向下 */}
    </button>
  </>);
}
```

toolbar 内渲染（`FallbackInfoButton` 之后）：`{isAssistant && onRateMessage ? <FeedbackButtons copy={t} message={message} ratingOf={ratingOf} onRateMessage={onRateMessage} onRemoveRating={onRemoveRating} /> : null}`。`MessageListProps` 加三个可选字段并在 `MessageList` 解构透传。

- [ ] **Step 3: 纯函数 + 测试** — 把"是否展示"提为可测纯函数（照 `isBookmarkActionAvailable` 模式）：

```ts
export function isFeedbackAvailable(onRateMessage?: (id: string, rating: FeedbackRating) => void): boolean {
  return typeof onRateMessage === 'function';
}
```

`message-list.test.tsx` 追加：

```ts
test('feedback buttons only render when host provides onRateMessage', () => {
  assert.equal(isFeedbackAvailable(), false);
  assert.equal(isFeedbackAvailable(() => undefined), true);
});
```

- [ ] **Step 4: chat/page.tsx 透传** — `MessageList` 调用处（723 行 `onBookmark` 旁）把三个 props 从 ChatPageProps 透传下去；ChatPageProps 加同名可选字段。

- [ ] **Step 5: 运行 + Commit**

Run: `npx tsx --test packages/views/src/chat/message-list.test.tsx`
```bash
git add packages/views/src/chat/message-list.tsx packages/views/src/chat/message-list.test.tsx packages/views/src/chat/chat-copy.ts packages/views/src/chat/page.tsx
git commit -m "feat(views): assistant 消息 like/dislike 反馈按钮（SP11 P-4）"
```

---

### Task 8: apps/web 宿主接线反馈回调

**Files:**
- Modify: apps/web 的聊天宿主页（`packages/views/src/chat/page.tsx` 的消费者；用 `grep -rn "ChatPage\|onBookmark=" apps/web/src --include="*.tsx" | grep -v test` 定位实际接线的页面文件）

**Interfaces:**
- Consumes: Task 7 的三个 props、Task 6 的 `client.chat.feedback`。

- [ ] **Step 1: 宿主实现状态与回调**

在宿主组件（ChatPage 的渲染处）加：

```tsx
const [ratings, setRatings] = useState<Record<string, FeedbackRating>>({});

const ratingOf = useCallback((messageId: string) => ratings[messageId], [ratings]);
const onRateMessage = useCallback(async (messageId: string, rating: FeedbackRating) => {
  setRatings((prev) => ({ ...prev, [messageId]: rating }));
  try { await client.chat.feedback.submit(sessionId, messageId, rating); }
  catch (reason) { setRatings((prev) => { const next = { ...prev }; delete next[messageId]; return next; }); /* toast：复用页面现有错误提示 */ }
}, [client, sessionId]);
const onRemoveRating = useCallback(async (messageId: string) => {
  const previous = ratings[messageId];
  setRatings((prev) => { const next = { ...prev }; delete next[messageId]; return next; });
  try { await client.chat.feedback.remove(sessionId, messageId); }
  catch (reason) { if (previous) setRatings((prev) => ({ ...prev, [messageId]: previous })); }
}, [client, sessionId, ratings]);
```

并把三者传入 ChatPage。`sessionId` 取自宿主现有会话状态（执行时按该页面的 session 上下文变量名）。

- [ ] **Step 2: 会话加载时回显** — 在宿主加载会话消息的既有流程后追加：

```tsx
try {
  const mine = await client.chat.feedback.mine(sessionId);
  setRatings(Object.fromEntries(mine.items.filter((e) => e.rating === 'like' || e.rating === 'dislike').map((e) => [e.message_id, e.rating])));
} catch { /* 回显失败不阻塞聊天 */ }
```

- [ ] **Step 3: 类型检查 + 手动冒烟** — `pnpm --filter @weknora/web exec tsc --noEmit`（或该包现有 typecheck script）；起 React dev（5175）+ 后端（8082）实测：点赞→刷新页面→高亮保持；再点同键→撤销。

- [ ] **Step 4: Commit**

```bash
git add apps/web/src/chat/   # 仅实际改动的宿主文件
git commit -m "feat(web): 聊天页接线消息反馈回调（SP11 P-4）"
```

---

### Task 9: recharts + AnalyticsPage + 路由 + 导航入口

**Files:**
- Modify: `apps/web/package.json`（recharts 依赖）
- Create: `apps/web/src/analytics/AnalyticsPage.tsx`
- Test: `apps/web/src/analytics/analytics-range.test.ts`（日期范围纯函数）
- Modify: `apps/web/src/router.tsx`（lazy import + analyticsRoute + addChildren）
- Modify: `apps/web/src/platform/PlatformShell.tsx`（导航项 + labels）

**Interfaces:**
- Consumes: Task 6 `client.analytics.*`；`scopeRuntime.role()`（admin/owner 门禁按 OrganizationsPage 的 role prop 模式）。

- [ ] **Step 1: 加依赖**

Run: `pnpm --filter @weknora/web add recharts`

- [ ] **Step 2: 日期范围纯函数**（抽出来便于测试）：

```ts
// analytics-range.ts
export interface AnalyticsDateRange { startTime: string; endTime: string; }
export function defaultAnalyticsRange(now = new Date()): AnalyticsDateRange {
  const end = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate()));
  const start = new Date(end); start.setUTCDate(start.getUTCDate() - 30);
  return { startTime: toISODate(start), endTime: toISODate(end) };
}
export function toISODate(d: Date): string { return d.toISOString().slice(0, 10); }
export function clampAnalyticsRange(startTime: string, endTime: string): AnalyticsDateRange {
  // 起止非法/倒置时回退默认；范围最长 366 天
}
```

测试断言：默认差 30 天、倒置回退、超 366 天截断。

- [ ] **Step 3: AnalyticsPage 组件** `apps/web/src/analytics/AnalyticsPage.tsx`

照 `OrganizationsPage.tsx` 骨架（props `{ client, role }`、`useState`+`load()`+`useEffect`、`formatMessage(locale, key)`、Tailwind class 常量）：

- 顶部：标题 + 两个 `<input type="date">`（from/to，初值 `defaultAnalyticsRange()`）+ 应用按钮
- `role !== 'owner' && role !== 'admin'` 时渲染无权限占位（"需要管理员权限"）
- 四个 section（grid 布局）：
  1. **查询趋势**：recharts `LineChart`，双 Y 轴或双线（queries 主线；likes/dislikes 双线）——数据 `client.analytics.queryTrend({ startTime, endTime })`
  2. **活跃用户**：`BarChart`（active_users 按日）
  3. **渠道分布**：`BarChart`（把 channels 平铺点透视成 `date[] × source` 堆叠条；source 空串归 `web`）
  4. **Agent 维度**：agentId 输入框 + 查询按钮（`client.analytics.agentUsage(agentId, {...})`，双线 messages/unique_users）
- loading/error/empty 三态（照 OrganizationsPage 的 listError/loading 写法）；recharts 组件用 `ResponsiveContainer` 包裹，高度 240px
- 全部数据在 `load()` 里并发拉取（`Promise.all`），日期变更时重载

- [ ] **Step 4: 路由挂载** `apps/web/src/router.tsx`：

```tsx
const AnalyticsPage = lazy(() => import('./analytics/AnalyticsPage.tsx').then((m) => ({ default: m.AnalyticsPage })));
const analyticsRoute = createRoute({
  getParentRoute: () => platformRoute,
  path: 'analytics',
  component: (): ReactNode => (
    <Suspense fallback={<RoutePending loadingText={deps.loadingText} />}>
      <AnalyticsPage client={client} role={scopeRuntime.role()} />
    </Suspense>
  ),
});
```

`platformRoute.addChildren([...])` 数组加 `analyticsRoute`；`routes.tsx` 的 resolveRoute 平台路径表（76 行附近）加 `'/platform/analytics'` 映射（若表内需枚举）。

- [ ] **Step 5: 导航入口** `apps/web/src/platform/PlatformShell.tsx`：导航项数组（159 行模式）加：

```tsx
{ key: 'analytics', href: '/platform/analytics', label: labels.analytics, icon: <Icon path={ICONS.chart} />, match: (p: string) => p.startsWith('/platform/analytics') },
```

（`ICONS.chart` 若不存在，从现有 ICONS 集合选一个图表类 path 或新增一条 SVG path 常量，与相邻 icon 同规格。）labels 区（188 行模式）加 `analytics: formatMessage(locale, 'menu.analytics')`；i18n 文案键 `menu.analytics`（中"数据分析"/英"Analytics"）加进 `@weknora/i18n` 的 zh-CN/en-US 词条文件（grep `menu.agents` 定位词条文件）。入口按 `canViewChannelSessions()` 同款布尔（systemAdmin || owner || admin）条件渲染。

- [ ] **Step 6: 测试 + 冒烟**

Run: `pnpm test:web`（含新 analytics-range.test.ts）+ 起 React dev 手动访问 `/platform/analytics`：默认 30 天空数据不报错、造几条消息+反馈后四图出数。

- [ ] **Step 7: Commit**

```bash
git add apps/web/package.json pnpm-lock.yaml apps/web/src/analytics/ apps/web/src/router.tsx apps/web/src/routes.tsx apps/web/src/platform/PlatformShell.tsx packages/i18n/
git commit -m "feat(web): /platform/analytics 分析仪表盘（SP11 P-5）"
```

---

### Task 10: 回归、证据与登记

**Files:**
- Create: `docs/migrations/react/evidence/onyx-parity/2026-09-19-sp11-feedback-analytics.md`（截图+冒烟记录；沿用 parity 线 evidence 惯例）
- Modify: `docs/onyx-parity/ROADMAP.md`（SP11 状态 🔄 → ✅，附验证证据链接）
- Modify: `docs/onyx-parity/GAP-MATRIX.md`（P-4/P-5 行的"定性"列改 ✅ 并注明交付物）

- [ ] **Step 1: 全量后端回归**

Run: `go build ./... && go test ./internal/... 2>&1 | tail -20`
Expected: 全 PASS（如有既有红项，对照记忆"main 门禁红项"仅确认非本次引入）

- [ ] **Step 2: 全量前端回归**

Run: `pnpm test:web && pnpm test:shared`
Expected: 全 PASS

- [ ] **Step 3: 端到端冒烟并截图**（后端 8082 + React 5175）：消息点赞/撤销/回显、分析页四图与日期过滤、非 admin 角色看不到入口。截图存 evidence 目录并写结论。

- [ ] **Step 4: 更新 ROADMAP/GAP-MATRIX 状态 + Commit**

```bash
git add docs/migrations/react/evidence/onyx-parity/2026-09-19-sp11-feedback-analytics.md docs/onyx-parity/ROADMAP.md docs/onyx-parity/GAP-MATRIX.md
git commit -m "docs(parity): SP11 反馈+分析验收与登记（P-4/P-5 ✅）"
```

---

## Self-Review 记录

- **Spec 覆盖**：P-4（feedback 表/API/气泡按钮）→ Task 1/2/7/8；P-5（四组聚合端点+图表页）→ Task 3/4/9；api-client/contracts → Task 5/6；embed 不做=按钮仅在 web 宿主接线 ✓。
- **类型一致性**：`FeedbackRating`（contracts）与 `types.FeedbackRatingLike/Dislike`（Go）字符串值 'like'/'dislike' 对齐；repo/service/handler 方法名在 Task 1/2/4 间一致（UpsertFeedback/QueryTrend 等）✓。
- **已知执行期确认点**（已在任务内写明 grep 指引，非占位）：SessionRepository/MessageRepository 按 id 取单条的方法名、`envelope` helper 导出与否、ICONS.chart 存在性、ChatPage 宿主文件定位。
