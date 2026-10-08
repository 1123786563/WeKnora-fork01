package container

import (
	"context"
	"errors"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

type craftProviderSessions struct{ interfaces.SessionService }

func (craftProviderSessions) GetOwnedSession(context.Context, string) (*types.Session, error) {
	return nil, errors.New("owner-only fallback must not authorize shared Task access")
}

type craftProviderDocuments struct {
	interfaces.TemporaryDocumentService
}
type craftProviderFiles struct{ interfaces.FileService }
type craftProviderModels struct{ interfaces.ModelService }

func TestNewCraftSessionServiceReceivesSharedTaskACLPorts(t *testing.T) {
	db := wiringTestDB(t)
	require.NoError(t, db.Exec(`INSERT INTO tenant_members (tenant_id,user_id,role,status,joined_at,created_at,updated_at) VALUES
		(1,'u-wiring','contributor','active',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),
		(1,'u-collab','contributor','active',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO craft_sessions (session_id,tenant_id,kind) VALUES ('s-wiring',1,'web')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO craft_workspaces (id,tenant_id,session_id,owner_id,sandbox_id,generation) VALUES ('ws-wiring',1,'s-wiring','u-wiring','sandbox-wiring','1')`).Error)
	access := service.NewCraftAccessService(db)
	owner := craft.Scope{TenantID: 1, UserID: "u-wiring", SessionID: "s-wiring"}
	require.NoError(t, access.Grant(context.Background(), owner, "u-collab", craft.TaskRoleCollaborator))

	runs := service.NewAgentRunService(repository.NewAgentRunStore(db))
	svc, err := newCraftSessionService(
		db,
		craftProviderSessions{},
		repository.NewCraftStore(db),
		repository.NewCraftVersionStore(db),
		craftProviderDocuments{},
		craftProviderFiles{},
		craftProviderModels{},
		access,
		&AgentRuntime{Runs: runs},
		nil,
	)
	require.NoError(t, err)

	collaborator := craft.Scope{TenantID: 1, UserID: "u-collab", SessionID: "s-wiring"}
	view, err := svc.View(context.Background(), collaborator)
	require.NoError(t, err, "TaskRead must use the access checker before resolving owner storage scope")
	require.Equal(t, "ws-wiring", view.WorkspaceID)
	rows, next, err := svc.List(context.Background(), craft.Scope{TenantID: 1, UserID: "u-collab"}, "", 10)
	require.NoError(t, err, "dedicated list must use the grant-aware paginated port")
	require.Empty(t, next)
	require.Len(t, rows, 1)
	require.Equal(t, "s-wiring", rows[0].SessionID)
}
