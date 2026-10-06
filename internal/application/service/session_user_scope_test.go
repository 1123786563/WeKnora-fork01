package service

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func testSessionScopeContext(tenantID uint64, userID string) context.Context {
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, tenantID)
	if userID != "" {
		ctx = context.WithValue(ctx, types.UserIDContextKey, userID)
	}
	return ctx
}

func testAPISessionScopeContext(tenantID uint64, externalUserID string) context.Context {
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, tenantID)
	ctx = context.WithValue(ctx, types.UserIDContextKey, "system-7")
	return types.WithPrincipal(ctx, types.Principal{
		Type: types.PrincipalAPIExternalUser,
		ID:   externalUserID,
	})
}

func testAPITenantKeyScopeContext(tenantID uint64, keyID uint64) context.Context {
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, tenantID)
	ctx = context.WithValue(ctx, types.UserIDContextKey, "system-1")
	ctx = context.WithValue(ctx, types.TenantRoleContextKey, types.TenantRoleViewer)
	ctx = types.WithPrincipal(ctx, types.Principal{
		Type: types.PrincipalAPITenant,
		ID:   "1",
	})
	return types.WithTenantAPIKeyScope(ctx, types.TenantAPIKeyScope{KeyID: keyID})
}

func newTestSessionService(t *testing.T) (*sessionService, *gorm.DB) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&types.Session{}))

	return &sessionService{
		sessionRepo: repository.NewSessionRepository(db),
	}, db
}

func TestGetSessionIsScopedToCurrentUser(t *testing.T) {
	svc, db := newTestSessionService(t)
	aliceSession := &types.Session{
		TenantID: 1,
		UserID:   "alice",
		Title:    "alice private session",
	}
	require.NoError(t, db.Create(aliceSession).Error)
	bobSession := &types.Session{
		TenantID: 1,
		UserID:   "bob",
		Title:    "bob private session",
	}
	require.NoError(t, db.Create(bobSession).Error)
	legacySession := &types.Session{
		TenantID: 1,
		Title:    "legacy tenant session",
	}
	require.NoError(t, db.Create(legacySession).Error)

	_, err := svc.GetSession(testSessionScopeContext(1, "bob"), aliceSession.ID)
	require.ErrorIs(t, err, apperrors.ErrSessionNotFound)

	got, err := svc.GetSession(testSessionScopeContext(1, "bob"), bobSession.ID)
	require.NoError(t, err)
	require.Equal(t, bobSession.ID, got.ID)

	got, err = svc.GetSession(testSessionScopeContext(1, "bob"), legacySession.ID)
	require.NoError(t, err)
	require.Equal(t, legacySession.ID, got.ID)
}

func TestUpdateSessionIsScopedToCurrentUserAndAllowsNoOp(t *testing.T) {
	svc, db := newTestSessionService(t)
	aliceSession := &types.Session{
		TenantID:    1,
		UserID:      "alice",
		Title:       "alice private session",
		Description: "original description",
	}
	require.NoError(t, db.Create(aliceSession).Error)

	err := svc.UpdateSession(testSessionScopeContext(1, "bob"), &types.Session{
		ID:          aliceSession.ID,
		TenantID:    1,
		Title:       "bob update attempt",
		Description: "should not be saved",
	})
	require.ErrorIs(t, err, apperrors.ErrSessionNotFound)

	var unchanged types.Session
	require.NoError(t, db.First(&unchanged, "id = ?", aliceSession.ID).Error)
	require.Equal(t, aliceSession.Title, unchanged.Title)
	require.Equal(t, aliceSession.Description, unchanged.Description)

	err = svc.UpdateSession(testSessionScopeContext(1, "alice"), &types.Session{
		ID:          aliceSession.ID,
		TenantID:    1,
		Title:       aliceSession.Title,
		Description: aliceSession.Description,
	})
	require.NoError(t, err)
}

func TestUpdateSessionRejectsPlantedMaintenanceDescription(t *testing.T) {
	svc, db := newTestSessionService(t)
	aliceSession := &types.Session{
		TenantID:    1,
		UserID:      "alice",
		Title:       "alice private session",
		Description: "ordinary",
	}
	require.NoError(t, db.Create(aliceSession).Error)

	err := svc.UpdateSession(testSessionScopeContext(1, "alice"), &types.Session{
		ID:          aliceSession.ID,
		TenantID:    1,
		Title:       "still mine",
		Description: types.SkillMaintenanceSessionMarker + "install",
	})
	require.NoError(t, err)

	var got types.Session
	require.NoError(t, db.First(&got, "id = ?", aliceSession.ID).Error)
	require.Equal(t, "still mine", got.Title)
	require.Empty(t, got.Description,
		"a client must not be able to hide a session behind the maintenance marker")
}

func TestUpdateSessionPreservesMaintenanceDescription(t *testing.T) {
	svc, db := newTestSessionService(t)
	marker := types.SkillMaintenanceSessionMarker + "install"
	row := &types.Session{
		TenantID:    1,
		UserID:      "alice",
		Title:       "Skill install",
		Description: marker,
	}
	require.NoError(t, db.Create(row).Error)

	err := svc.UpdateSession(testSessionScopeContext(1, "alice"), &types.Session{
		ID:          row.ID,
		TenantID:    1,
		Title:       "unhide me",
		Description: "plain chat",
	})
	require.NoError(t, err)

	var got types.Session
	require.NoError(t, db.First(&got, "id = ?", row.ID).Error)
	require.Equal(t, marker, got.Description,
		"a PUT must not strip the marker off a real maintenance session")
}

func TestGetSessionIsScopedToAPIExternalUser(t *testing.T) {
	svc, db := newTestSessionService(t)
	aliceSession := &types.Session{
		TenantID: 1,
		UserID:   "api_external_user:7:alice",
		Title:    "alice api session",
	}
	require.NoError(t, db.Create(aliceSession).Error)
	bobSession := &types.Session{
		TenantID: 1,
		UserID:   "api_external_user:7:bob",
		Title:    "bob api session",
	}
	require.NoError(t, db.Create(bobSession).Error)
	tenantSession := &types.Session{
		TenantID: 1,
		UserID:   "system-7",
		Title:    "tenant api session",
	}
	require.NoError(t, db.Create(tenantSession).Error)

	_, err := svc.GetSession(testAPISessionScopeContext(1, "7:alice"), bobSession.ID)
	require.ErrorIs(t, err, apperrors.ErrSessionNotFound)

	got, err := svc.GetSession(testAPISessionScopeContext(1, "7:alice"), aliceSession.ID)
	require.NoError(t, err)
	require.Equal(t, aliceSession.ID, got.ID)

	_, err = svc.GetSession(testAPISessionScopeContext(1, "7:alice"), tenantSession.ID)
	require.ErrorIs(t, err, apperrors.ErrSessionNotFound)

	got, err = svc.GetSession(testSessionScopeContext(1, "system-7"), tenantSession.ID)
	require.NoError(t, err)
	require.Equal(t, tenantSession.ID, got.ID)
}

func TestGetSessionAllowsAdminToOpenAPIKeySessions(t *testing.T) {
	svc, db := newTestSessionService(t)
	apiSession := &types.Session{
		TenantID: 1,
		UserID:   types.SessionOwnerAPITenantKeyPrefix + "1:10",
		Title:    "api key session",
	}
	require.NoError(t, db.Create(apiSession).Error)
	otherUserSession := &types.Session{
		TenantID: 1,
		UserID:   "bob",
		Title:    "bob private session",
	}
	require.NoError(t, db.Create(otherUserSession).Error)

	// A non-admin web user cannot open the API-key session.
	viewerCtx := testSessionScopeContext(1, "alice")
	_, err := svc.GetSession(viewerCtx, apiSession.ID)
	require.ErrorIs(t, err, apperrors.ErrSessionNotFound)

	// An admin can open the API-key session.
	adminCtx := context.WithValue(testSessionScopeContext(1, "alice"), types.TenantRoleContextKey, types.TenantRoleAdmin)
	got, err := svc.GetSession(adminCtx, apiSession.ID)
	require.NoError(t, err)
	require.Equal(t, apiSession.ID, got.ID)

	// The admin fallback is limited to API-key sessions; another user's
	// personal session stays hidden.
	_, err = svc.GetSession(adminCtx, otherUserSession.ID)
	require.ErrorIs(t, err, apperrors.ErrSessionNotFound)
}

func TestGetOwnedSessionDeniesAdminOnAPIKeySessions(t *testing.T) {
	svc, db := newTestSessionService(t)
	apiSession := &types.Session{
		TenantID: 1,
		UserID:   types.SessionOwnerAPITenantKeyPrefix + "1:10",
		Title:    "api key session",
	}
	require.NoError(t, db.Create(apiSession).Error)

	adminCtx := context.WithValue(
		testSessionScopeContext(1, "alice"), types.TenantRoleContextKey, types.TenantRoleAdmin,
	)

	// The read path lets an admin open the API-key session (folder navigation)...
	got, err := svc.GetSession(adminCtx, apiSession.ID)
	require.NoError(t, err)
	require.Equal(t, apiSession.ID, got.ID)

	// ...but the strict owner scope used by write/mutation endpoints (title
	// generation, attachments, stop, QA) denies it, so admins stay read-only.
	_, err = svc.GetOwnedSession(adminCtx, apiSession.ID)
	require.ErrorIs(t, err, apperrors.ErrSessionNotFound)
}

func TestListSessionsAPISourceRequiresAdminAndReturnsAllKeys(t *testing.T) {
	svc, db := newTestSessionService(t)
	require.NoError(t, db.AutoMigrate(&testListSessionsIMChannelSession{}))

	key1 := &types.Session{TenantID: 1, UserID: types.SessionOwnerAPITenantKeyPrefix + "1:10", Title: "key1"}
	key2 := &types.Session{TenantID: 1, UserID: types.SessionOwnerAPITenantKeyPrefix + "1:20", Title: "key2"}
	externalUser := &types.Session{
		TenantID: 1,
		UserID:   types.SessionOwnerAPIExternalUserPrefix + "1:external-u1",
		Title:    "external user",
	}
	web := &types.Session{TenantID: 1, UserID: "alice", Title: "alice web"}
	require.NoError(t, db.Create(key1).Error)
	require.NoError(t, db.Create(key2).Error)
	require.NoError(t, db.Create(externalUser).Error)
	require.NoError(t, db.Create(web).Error)

	// A non-admin (viewer) web user is rejected.
	viewerCtx := testSessionScopeContext(1, "alice")
	_, err := svc.ListSessions(viewerCtx, &types.SessionListQuery{Source: types.SessionSourceAPI})
	require.Error(t, err)
	var appErr *apperrors.AppError
	require.ErrorAs(t, err, &appErr)
	require.Equal(t, apperrors.ErrForbidden, appErr.Code)

	// An admin sees every API-key session in the tenant, not just their own.
	adminCtx := context.WithValue(testSessionScopeContext(1, "alice"), types.TenantRoleContextKey, types.TenantRoleAdmin)
	result, err := svc.ListSessions(adminCtx, &types.SessionListQuery{Source: types.SessionSourceAPI})
	require.NoError(t, err)
	require.EqualValues(t, 3, result.Total)
}

func TestGetSessionAllowsAdminToReadAPIExternalUserSession(t *testing.T) {
	svc, db := newTestSessionService(t)
	require.NoError(t, db.AutoMigrate(&testListSessionsIMChannelSession{}))

	apiSession := &types.Session{
		TenantID: 1,
		UserID:   types.SessionOwnerAPIExternalUserPrefix + "1:external-u1",
		Title:    "external user",
	}
	require.NoError(t, db.Create(apiSession).Error)

	viewerCtx := testSessionScopeContext(1, "alice")
	_, err := svc.GetSession(viewerCtx, apiSession.ID)
	require.ErrorIs(t, err, apperrors.ErrSessionNotFound)

	adminCtx := context.WithValue(viewerCtx, types.TenantRoleContextKey, types.TenantRoleAdmin)
	got, err := svc.GetSession(adminCtx, apiSession.ID)
	require.NoError(t, err)
	require.Equal(t, apiSession.ID, got.ID)
}

func TestListSessionsIMSourceRequiresAdmin(t *testing.T) {
	svc, db := newTestSessionService(t)
	require.NoError(t, db.AutoMigrate(&testListSessionsIMChannelSession{}))

	imSession := &types.Session{TenantID: 1, Title: "feishu chat"}
	require.NoError(t, db.Create(imSession).Error)
	require.NoError(t, db.Create(&testListSessionsIMChannelSession{
		SessionID: imSession.ID, Platform: "feishu",
	}).Error)

	viewerCtx := testSessionScopeContext(1, "alice")
	_, err := svc.ListSessions(viewerCtx, &types.SessionListQuery{Source: "feishu"})
	require.Error(t, err)
	var appErr *apperrors.AppError
	require.ErrorAs(t, err, &appErr)
	require.Equal(t, apperrors.ErrForbidden, appErr.Code)

	adminCtx := context.WithValue(testSessionScopeContext(1, "alice"), types.TenantRoleContextKey, types.TenantRoleAdmin)
	result, err := svc.ListSessions(adminCtx, &types.SessionListQuery{Source: "feishu"})
	require.NoError(t, err)
	require.EqualValues(t, 1, result.Total)
}

func TestListSessionsEmbedSourceRequiresAdmin(t *testing.T) {
	svc, db := newTestSessionService(t)
	require.NoError(t, db.AutoMigrate(&testListSessionsIMChannelSession{}))

	embed := &types.Session{
		TenantID:    1,
		Title:       "embed chat",
		Description: types.EmbedSessionMarkerPrefix + "ch-1",
		UserID:      types.PrincipalEmbedSession + ":1:ch-1:sess-1",
	}
	require.NoError(t, db.Create(embed).Error)

	viewerCtx := testSessionScopeContext(1, "alice")
	_, err := svc.ListSessions(viewerCtx, &types.SessionListQuery{Source: "embed:ch-1"})
	require.Error(t, err)
	var appErr *apperrors.AppError
	require.ErrorAs(t, err, &appErr)
	require.Equal(t, apperrors.ErrForbidden, appErr.Code)

	adminCtx := context.WithValue(testSessionScopeContext(1, "alice"), types.TenantRoleContextKey, types.TenantRoleAdmin)
	result, err := svc.ListSessions(adminCtx, &types.SessionListQuery{Source: "embed:ch-1"})
	require.NoError(t, err)
	require.EqualValues(t, 1, result.Total)
}

func TestGetSessionDeniesViewerOnIMSession(t *testing.T) {
	svc, db := newTestSessionService(t)
	require.NoError(t, db.AutoMigrate(&testListSessionsIMChannelSession{}))

	imSession := &types.Session{TenantID: 1, Title: "feishu chat"}
	require.NoError(t, db.Create(imSession).Error)
	require.NoError(t, db.Create(&testListSessionsIMChannelSession{
		SessionID: imSession.ID, Platform: "feishu",
	}).Error)

	viewerCtx := testSessionScopeContext(1, "alice")
	_, err := svc.GetSession(viewerCtx, imSession.ID)
	require.ErrorIs(t, err, apperrors.ErrSessionNotFound)

	adminCtx := context.WithValue(testSessionScopeContext(1, "alice"), types.TenantRoleContextKey, types.TenantRoleAdmin)
	got, err := svc.GetSession(adminCtx, imSession.ID)
	require.NoError(t, err)
	require.Equal(t, imSession.ID, got.ID)
	require.Equal(t, "feishu", got.IMPlatform)
}

func TestGetSessionAllowsAPITenantRuntimeToReadOwnAPIKeySession(t *testing.T) {
	svc, db := newTestSessionService(t)

	apiSession := &types.Session{
		TenantID: 1,
		UserID:   types.SessionOwnerAPITenantKeyPrefix + "1:10",
		Title:    "api key session",
	}
	require.NoError(t, db.Create(apiSession).Error)

	got, err := svc.GetSession(testAPITenantKeyScopeContext(1, 10), apiSession.ID)
	require.NoError(t, err)
	require.Equal(t, apiSession.ID, got.ID)
}

func TestGetSessionDeniesAPITenantRuntimeFromReadingIMSession(t *testing.T) {
	svc, db := newTestSessionService(t)
	require.NoError(t, db.AutoMigrate(&testListSessionsIMChannelSession{}))

	imSession := &types.Session{TenantID: 1, Title: "feishu chat"}
	require.NoError(t, db.Create(imSession).Error)
	require.NoError(t, db.Create(&testListSessionsIMChannelSession{
		SessionID: imSession.ID, Platform: "feishu",
	}).Error)

	_, err := svc.GetSession(testAPITenantKeyScopeContext(1, 10), imSession.ID)
	require.ErrorIs(t, err, apperrors.ErrSessionNotFound)
}

func TestGetSessionDeniesAPIExternalUserRuntimeFromReadingIMSession(t *testing.T) {
	svc, db := newTestSessionService(t)
	require.NoError(t, db.AutoMigrate(&testListSessionsIMChannelSession{}))

	imSession := &types.Session{TenantID: 1, Title: "feishu chat"}
	require.NoError(t, db.Create(imSession).Error)
	require.NoError(t, db.Create(&testListSessionsIMChannelSession{
		SessionID: imSession.ID, Platform: "feishu",
	}).Error)

	_, err := svc.GetSession(testAPISessionScopeContext(1, "1:alice"), imSession.ID)
	require.ErrorIs(t, err, apperrors.ErrSessionNotFound)
}

func TestGetSessionAllowsIMRuntimeToReadIMSession(t *testing.T) {
	svc, db := newTestSessionService(t)
	require.NoError(t, db.AutoMigrate(&testListSessionsIMChannelSession{}))

	imSession := &types.Session{TenantID: 1, Title: "feishu chat"}
	require.NoError(t, db.Create(imSession).Error)
	require.NoError(t, db.Create(&testListSessionsIMChannelSession{
		SessionID: imSession.ID,
		Platform:  "feishu",
	}).Error)

	ctx := context.WithValue(
		testSessionScopeContext(1, "system-1"),
		types.TenantRoleContextKey,
		types.TenantRoleViewer,
	)
	ctx = types.WithPrincipal(ctx, types.Principal{
		Type: types.PrincipalIMUser,
		ID:   "1:channel-1:feishu:open-id-1",
	})

	got, err := svc.GetSession(ctx, imSession.ID)
	require.NoError(t, err)
	require.Equal(t, imSession.ID, got.ID)
	require.Equal(t, "feishu", got.IMPlatform)
}

func TestGetSessionAllowsEmbedRuntimeToReadOwnEmbedSession(t *testing.T) {
	svc, db := newTestSessionService(t)
	require.NoError(t, db.AutoMigrate(&testListSessionsIMChannelSession{}))

	// An embed-widget session: description marks the channel, user_id is the
	// per-session principal's storage id (see CreateEmbedSession).
	principal := types.EmbedSessionPrincipal(1, "ch-1", "sess-1")
	ownSession := &types.Session{
		TenantID:    1,
		Title:       "embed chat",
		Description: types.EmbedSessionMarkerPrefix + "ch-1",
		UserID:      principal.StorageID(),
	}
	require.NoError(t, db.Create(ownSession).Error)

	// The embed widget authenticates as a Viewer (embed_auth.go) but is the
	// legitimate owner of its own channel session — symmetric to the IM and
	// API-tenant runtimes above.
	ctx := context.WithValue(
		testSessionScopeContext(1, "system-1"),
		types.TenantRoleContextKey,
		types.TenantRoleViewer,
	)
	ctx = types.WithPrincipal(ctx, principal)

	got, err := svc.GetSession(ctx, ownSession.ID)
	require.NoError(t, err)
	require.Equal(t, ownSession.ID, got.ID)
}

func TestGetSessionDeniesEmbedRuntimeFromReadingForeignEmbedSession(t *testing.T) {
	svc, db := newTestSessionService(t)
	require.NoError(t, db.AutoMigrate(&testListSessionsIMChannelSession{}))

	// A session owned by a different embed session principal.
	owner := types.EmbedSessionPrincipal(1, "ch-1", "sess-other")
	foreignSession := &types.Session{
		TenantID:    1,
		Title:       "foreign embed chat",
		Description: types.EmbedSessionMarkerPrefix + "ch-1",
		UserID:      owner.StorageID(),
	}
	require.NoError(t, db.Create(foreignSession).Error)

	// A different embed session principal must not read it — the owner scope
	// keeps each visitor's conversation isolated even after the bypass is granted.
	ctx := context.WithValue(
		testSessionScopeContext(1, "system-1"),
		types.TenantRoleContextKey,
		types.TenantRoleViewer,
	)
	ctx = types.WithPrincipal(ctx, types.EmbedSessionPrincipal(1, "ch-1", "sess-attacker"))

	_, err := svc.GetSession(ctx, foreignSession.ID)
	require.ErrorIs(t, err, apperrors.ErrSessionNotFound)
}

// testListSessionsIMChannelSession lets QueryPaged's LEFT JOIN resolve against a
// real table in the in-memory SQLite database.
type testListSessionsIMChannelSession struct {
	ID          uint64 `gorm:"primaryKey;autoIncrement"`
	SessionID   string `gorm:"column:session_id"`
	Platform    string `gorm:"column:platform"`
	ChatID      string `gorm:"column:chat_id"`
	ThreadID    string `gorm:"column:thread_id"`
	UserID      string `gorm:"column:user_id"`
	AgentID     string `gorm:"column:agent_id"`
	IMChannelID string `gorm:"column:im_channel_id"`
}

func (testListSessionsIMChannelSession) TableName() string { return "im_channel_sessions" }

// newAuditSessionService wires a sessionService with an in-memory session store
// plus the given tenant row behind a stub tenant repository, so ListSessions
// tests can exercise the query-history policy on the "all" audit source.
func newAuditSessionService(t *testing.T, tenant *types.Tenant) (*sessionService, *gorm.DB) {
	t.Helper()
	svc, db := newTestSessionService(t)
	require.NoError(t, db.AutoMigrate(&testListSessionsIMChannelSession{}))
	svc.tenantRepo = &stubTenantRepoForHistory{tenant: tenant}
	return svc, db
}

func adminAuditContext() context.Context {
	return context.WithValue(
		testSessionScopeContext(1, "admin-user"),
		types.TenantRoleContextKey,
		types.TenantRoleAdmin,
	)
}

// seedAuditSessions returns one web, one API-key, and one cross-user session
// so the audit listing has every origin to observe.
func seedAuditSessions(t *testing.T, db *gorm.DB) (web, apiKey, bob *types.Session) {
	t.Helper()
	web = &types.Session{TenantID: 1, UserID: "alice", Title: "alice web"}
	apiKey = &types.Session{TenantID: 1, UserID: types.SessionOwnerAPITenantKeyPrefix + "1:10", Title: "api key"}
	bob = &types.Session{TenantID: 1, UserID: "bob", Title: "bob web"}
	require.NoError(t, db.Create(web).Error)
	require.NoError(t, db.Create(apiKey).Error)
	require.NoError(t, db.Create(bob).Error)
	return web, apiKey, bob
}

// The "all" audit source is Admin+ gated exactly like the channel sources.
func TestListSessionsAllSourceRequiresAdmin(t *testing.T) {
	svc, db := newAuditSessionService(t, &types.Tenant{ID: 1})
	seedAuditSessions(t, db)

	_, err := svc.ListSessions(testSessionScopeContext(1, "alice"), &types.SessionListQuery{
		Source: types.SessionListSourceAll,
	})
	require.Error(t, err)
	var appErr *apperrors.AppError
	require.ErrorAs(t, err, &appErr)
	require.Equal(t, apperrors.ErrForbidden, appErr.Code)
}

// Normal mode (and no config): the audit listing returns every tenant session
// across origins with owner ids intact, and an admin's user_id filter narrows
// it to one principal.
func TestListSessionsAllSourceAuditModes(t *testing.T) {
	normalTenant := &types.Tenant{ID: 1}
	anonymizedTenant := tenantWithQueryHistoryMode(t, types.QueryHistoryModeAnonymized)
	disabledTenant := tenantWithQueryHistoryMode(t, types.QueryHistoryModeDisabled)

	t.Run("normal returns all origins with owner ids", func(t *testing.T) {
		svc, db := newAuditSessionService(t, normalTenant)
		web, apiKey, bob := seedAuditSessions(t, db)

		result, err := svc.ListSessions(adminAuditContext(), &types.SessionListQuery{
			Source: types.SessionListSourceAll,
		})
		require.NoError(t, err)
		require.EqualValues(t, 3, result.Total)
		rows := result.Data.([]*types.SessionListItem)
		ids := make([]string, 0, len(rows))
		owners := make(map[string]string, len(rows))
		for _, row := range rows {
			ids = append(ids, row.ID)
			owners[row.ID] = row.UserID
		}
		require.ElementsMatch(t, []string{web.ID, apiKey.ID, bob.ID}, ids)
		require.Equal(t, "alice", owners[web.ID],
			"normal mode must keep owner ids visible to the admin")
	})

	t.Run("anonymized masks owner ids", func(t *testing.T) {
		svc, db := newAuditSessionService(t, anonymizedTenant)
		seedAuditSessions(t, db)

		result, err := svc.ListSessions(adminAuditContext(), &types.SessionListQuery{
			Source: types.SessionListSourceAll,
		})
		require.NoError(t, err)
		rows := result.Data.([]*types.SessionListItem)
		require.NotEmpty(t, rows)
		for _, row := range rows {
			require.Equal(t, "anonymous", row.UserID,
				"anonymized mode must mask owner ids on audit rows")
		}
	})

	t.Run("disabled is forbidden", func(t *testing.T) {
		svc, db := newAuditSessionService(t, disabledTenant)
		seedAuditSessions(t, db)

		_, err := svc.ListSessions(adminAuditContext(), &types.SessionListQuery{
			Source: types.SessionListSourceAll,
		})
		require.Error(t, err)
		var appErr *apperrors.AppError
		require.ErrorAs(t, err, &appErr)
		require.Equal(t, apperrors.ErrForbidden, appErr.Code)
		require.Contains(t, appErr.Message, "query history is disabled for this tenant")
	})

	t.Run("admin user_id filter narrows the audit view", func(t *testing.T) {
		svc, db := newAuditSessionService(t, normalTenant)
		web, _, _ := seedAuditSessions(t, db)

		result, err := svc.ListSessions(adminAuditContext(), &types.SessionListQuery{
			Source: types.SessionListSourceAll,
			UserID: "alice",
		})
		require.NoError(t, err)
		require.EqualValues(t, 1, result.Total)
		rows := result.Data.([]*types.SessionListItem)
		require.Equal(t, web.ID, rows[0].ID)
	})
}

// A non-admin's own listing never applies the caller-supplied user_id: the
// non-admin branch overwrites the scope from the authenticated principal.
func TestListSessionsNonAdminUserIDParamCannotEscapeScope(t *testing.T) {
	svc, db := newAuditSessionService(t, &types.Tenant{ID: 1})
	_, _, bob := seedAuditSessions(t, db)

	// bob asks for alice's rows via the user_id filter on his own listing.
	result, err := svc.ListSessions(testSessionScopeContext(1, "bob"), &types.SessionListQuery{
		UserID: "alice",
	})
	require.NoError(t, err)
	require.EqualValues(t, 1, result.Total)
	rows := result.Data.([]*types.SessionListItem)
	require.Equal(t, bob.ID, rows[0].ID,
		"a non-admin's user_id param must not override the owner scope")
}

// --- Session management scope (#3926) ---
//
// Embed-channel sessions store the embed principal's storage id
// ("embed_session:<tenant>:<channel>:<session>") in sessions.user_id, so the
// strict owner scope of the management mutations can never match them: admins
// saw the sessions in the (Admin+ scoped) listing but every delete failed
// with "session not found". The management scope gives delete/update/batch
// delete the same tenant-wide view ListSessions grants Admin+ callers, while
// non-admin semantics stay strictly unchanged.

func testAdminScopeContext(tenantID uint64, userID string) context.Context {
	return context.WithValue(
		testSessionScopeContext(tenantID, userID),
		types.TenantRoleContextKey, types.TenantRoleAdmin,
	)
}

// seedManagementScopeSessions returns alice's own web session, bob's foreign
// web session, and one embed-channel session in tenant 1.
func seedManagementScopeSessions(t *testing.T, db *gorm.DB) (own, foreign, embed *types.Session) {
	t.Helper()
	own = &types.Session{TenantID: 1, UserID: "alice", Title: "alice web"}
	foreign = &types.Session{TenantID: 1, UserID: "bob", Title: "bob web"}
	embed = &types.Session{
		TenantID:    1,
		Title:       "embed chat",
		Description: types.EmbedSessionMarkerPrefix + "ch-1",
		UserID:      types.EmbedSessionPrincipal(1, "ch-1", "sess-1").StorageID(),
	}
	require.NoError(t, db.Create(own).Error)
	require.NoError(t, db.Create(foreign).Error)
	require.NoError(t, db.Create(embed).Error)
	return own, foreign, embed
}

func countLiveSessions(t *testing.T, db *gorm.DB, id string) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Model(&types.Session{}).Where("id = ?", id).Count(&n).Error)
	return n
}

func TestDeleteSessionManagementScope(t *testing.T) {
	svc, db := newSessionServiceForHostDeleteTest(t, stubHostManager{}, nil)
	own, foreign, embed := seedManagementScopeSessions(t, db)

	// A non-admin user cannot delete the embed channel session, nor another
	// user's session — strict owner scope, unchanged.
	require.ErrorIs(t,
		svc.DeleteSession(testSessionScopeContext(1, "alice"), embed.ID),
		apperrors.ErrSessionNotFound)
	require.ErrorIs(t,
		svc.DeleteSession(testSessionScopeContext(1, "alice"), foreign.ID),
		apperrors.ErrSessionNotFound)
	require.EqualValues(t, 1, countLiveSessions(t, db, embed.ID))
	require.EqualValues(t, 1, countLiveSessions(t, db, foreign.ID))

	// Admin+ deletes the embed channel session (#3926) — and remains able to
	// delete their own session.
	require.NoError(t, svc.DeleteSession(testAdminScopeContext(1, "admin-user"), embed.ID))
	require.EqualValues(t, 0, countLiveSessions(t, db, embed.ID))
	require.NoError(t, svc.DeleteSession(testAdminScopeContext(1, "admin-user"), own.ID))
	require.EqualValues(t, 0, countLiveSessions(t, db, own.ID))
}

// The Admin+ widening never crosses tenants: the repository's tenant filter
// is part of the scope, not bypassed by it.
func TestDeleteSessionAdminScopeStaysTenantScoped(t *testing.T) {
	svc, db := newSessionServiceForHostDeleteTest(t, stubHostManager{}, nil)
	otherTenant := &types.Session{
		TenantID:    2,
		Title:       "other tenant embed chat",
		Description: types.EmbedSessionMarkerPrefix + "ch-9",
		UserID:      types.EmbedSessionPrincipal(2, "ch-9", "sess-9").StorageID(),
	}
	require.NoError(t, db.Create(otherTenant).Error)

	require.ErrorIs(t,
		svc.DeleteSession(testAdminScopeContext(1, "admin-user"), otherTenant.ID),
		apperrors.ErrSessionNotFound)
	require.EqualValues(t, 1, countLiveSessions(t, db, otherTenant.ID))
}

func TestBatchDeleteSessionsAdminDeletesEmbedAndForeignSessions(t *testing.T) {
	svc, db := newSessionServiceForHostDeleteTest(t, stubHostManager{}, nil)
	own, foreign, embed := seedManagementScopeSessions(t, db)

	require.NoError(t, svc.BatchDeleteSessions(
		testAdminScopeContext(1, "admin-user"),
		[]string{embed.ID, foreign.ID, own.ID},
	))
	for _, s := range []*types.Session{embed, foreign, own} {
		require.EqualValues(t, 0, countLiveSessions(t, db, s.ID))
	}
}

func TestBatchDeleteSessionsNonAdminOnlyDeletesOwnSessions(t *testing.T) {
	svc, db := newSessionServiceForHostDeleteTest(t, stubHostManager{}, nil)
	own, foreign, embed := seedManagementScopeSessions(t, db)

	// bob asks to delete his session together with alice's and the embed
	// channel's: only his own row goes away, the others are skipped.
	require.NoError(t, svc.BatchDeleteSessions(
		testSessionScopeContext(1, "bob"),
		[]string{foreign.ID, own.ID, embed.ID},
	))
	require.EqualValues(t, 0, countLiveSessions(t, db, foreign.ID))
	require.EqualValues(t, 1, countLiveSessions(t, db, own.ID))
	require.EqualValues(t, 1, countLiveSessions(t, db, embed.ID))
}

func TestUpdateSessionManagementScope(t *testing.T) {
	svc, db := newTestSessionService(t)
	_, _, embed := seedManagementScopeSessions(t, db)

	// A non-admin user cannot rename the embed channel session.
	err := svc.UpdateSession(testSessionScopeContext(1, "alice"), &types.Session{
		ID:          embed.ID,
		TenantID:    1,
		Title:       "hijacked",
		Description: types.EmbedSessionMarkerPrefix + "ch-1",
	})
	require.ErrorIs(t, err, apperrors.ErrSessionNotFound)

	// Admin+ renames it through the same tenant-wide management scope.
	err = svc.UpdateSession(testAdminScopeContext(1, "admin-user"), &types.Session{
		ID:          embed.ID,
		TenantID:    1,
		Title:       "admin renamed",
		Description: types.EmbedSessionMarkerPrefix + "ch-1",
	})
	require.NoError(t, err)

	var got types.Session
	require.NoError(t, db.First(&got, "id = ?", embed.ID).Error)
	require.Equal(t, "admin renamed", got.Title)
}
