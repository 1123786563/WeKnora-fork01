package codedelivery

import (
	"context"
	"errors"
	"testing"

	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime"
	runtime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector"
	"github.com/stretchr/testify/require"
)

func TestAgentRuntimeRootSentinelsRetainIdentity(t *testing.T) {
	require.Same(t, agentruntime.ErrNotFound, runtime.ErrNotFound)
	require.Same(t, agentruntime.ErrConflict, runtime.ErrConflict)
	require.ErrorIs(t, runtime.ErrNotFound, agentruntime.ErrNotFound)
	require.ErrorIs(t, runtime.ErrConflict, agentruntime.ErrConflict)
}

func TestDeliverySnapshotMapsApprovedIdentityAndPayload(t *testing.T) {
	app := appconnectorsvc.ActionSnapshot{ID: "action-1", TenantID: 77, ActorID: "owner", ConnectionID: "conn-9", Target: "github.deliver", AuthVersion: 4, Args: []byte(`{"repo":"acme/project"}`)}
	got := localSnapshot(app)
	require.Equal(t, ActionSnapshot{ID: "action-1", TenantID: 77, ActorID: "owner", ConnectionID: "conn-9", Target: "github.deliver", AuthVersion: 4, Args: app.Args}, got)
	require.Equal(t, []byte(app.Args), []byte(got.Args))
}

func TestFixtureRunReaderReturnsOwnerScopedSession(t *testing.T) {
	got, err := (fixtureRun{sessionID: "session-owner"}).GetOwnedRun(context.Background(), 7, "u1", "run-1")
	require.NoError(t, err)
	require.Equal(t, RunIdentity{SessionID: "session-owner"}, got)
	_, err = (fixtureRun{sessionID: "session-owner"}).GetOwnedRun(context.Background(), 8, "u1", "run-1")
	require.Error(t, err)
}

func TestDispatchClassificationsAreDistinct(t *testing.T) {
	require.NotEqual(t, ErrDispatchNotStarted, ErrDispatchUnknown)
	require.ErrorIs(t, errors.Join(errors.New("local rejection"), ErrDispatchNotStarted), ErrDispatchNotStarted)
	require.ErrorIs(t, errors.Join(errors.New("outcome uncertain"), ErrDispatchUnknown), ErrDispatchUnknown)
	require.NotErrorIs(t, ErrDispatchNotStarted, ErrDispatchUnknown)
}
