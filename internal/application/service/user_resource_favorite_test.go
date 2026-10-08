package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

// Characterization tests for the user resource favorite service, anchored on
// the legacy host-package implementation before the Pass B (25a) move to
// internal/agentcatalog/service.

type fakeFavoriteRepo struct {
	listCalls   []string
	addCalls    []string
	removeCalls []string

	listResult []*types.UserResourceFavorite
	listErr    error
	addCreated bool
	addErr     error
	removeErr  error
}

func (f *fakeFavoriteRepo) List(_ context.Context, userID string, _ uint64, resourceType string) ([]*types.UserResourceFavorite, error) {
	f.listCalls = append(f.listCalls, userID+"|"+resourceType)
	return f.listResult, f.listErr
}

func (f *fakeFavoriteRepo) Add(_ context.Context, userID string, _ uint64, resourceType, resourceID string) (bool, error) {
	f.addCalls = append(f.addCalls, userID+"|"+resourceType+"|"+resourceID)
	return f.addCreated, f.addErr
}

func (f *fakeFavoriteRepo) Remove(_ context.Context, userID string, _ uint64, resourceType, resourceID string) (bool, error) {
	f.removeCalls = append(f.removeCalls, userID+"|"+resourceType+"|"+resourceID)
	return true, f.removeErr
}

func (f *fakeFavoriteRepo) IsFavorite(context.Context, string, uint64, string, string) (bool, error) {
	return false, nil
}

func TestFavoriteServiceListRejectsUnknownResourceType(t *testing.T) {
	repo := &fakeFavoriteRepo{}
	svc := NewUserResourceFavoriteService(repo)

	list, err := svc.List(context.Background(), "user-1", 7, "bogus")
	require.ErrorIs(t, err, ErrFavoriteInvalidType)
	require.Nil(t, list)
	require.Empty(t, repo.listCalls, "the type allowlist runs before the repository is touched")
}

func TestFavoriteServiceListPassesThroughToRepository(t *testing.T) {
	want := []*types.UserResourceFavorite{{ResourceID: "kb-1", ResourceType: types.ResourceTypeKB}}
	repo := &fakeFavoriteRepo{listResult: want}
	svc := NewUserResourceFavoriteService(repo)

	list, err := svc.List(context.Background(), "user-1", 7, types.ResourceTypeKB)
	require.NoError(t, err)
	require.Same(t, want[0], list[0], "the list result is the repository's slice, unmodified")
	require.Equal(t, []string{"user-1|kb"}, repo.listCalls, "validated args reach the repository unchanged")
}

func TestFavoriteServiceAddValidatesTypeAndID(t *testing.T) {
	repo := &fakeFavoriteRepo{}
	svc := NewUserResourceFavoriteService(repo)

	require.ErrorIs(t, svc.Add(context.Background(), "user-1", 7, "bogus", "kb-1"), ErrFavoriteInvalidType)
	require.ErrorIs(t, svc.Add(context.Background(), "user-1", 7, types.ResourceTypeKB, ""), ErrFavoriteEmptyID)
	require.ErrorIs(t, svc.Add(context.Background(), "user-1", 7, types.ResourceTypeKB, "   "), ErrFavoriteEmptyID,
		"a whitespace-only id is treated as empty")
	require.Empty(t, repo.addCalls, "invalid requests never reach the repository")
}

func TestFavoriteServiceAddPassesThroughValidRequest(t *testing.T) {
	repo := &fakeFavoriteRepo{}
	svc := NewUserResourceFavoriteService(repo)

	require.NoError(t, svc.Add(context.Background(), "user-1", 7, types.ResourceTypeAgent, "agent-1"))
	require.Equal(t, []string{"user-1|agent|agent-1"}, repo.addCalls)
}

func TestFavoriteServiceAddPropagatesRepositoryError(t *testing.T) {
	repo := &fakeFavoriteRepo{addErr: errors.New("db down")}
	svc := NewUserResourceFavoriteService(repo)

	require.EqualError(t, svc.Add(context.Background(), "user-1", 7, types.ResourceTypeAgent, "agent-1"), "db down",
		"repository errors propagate unwrapped once validation passes")
}

func TestFavoriteServiceRemoveValidatesTypeAndID(t *testing.T) {
	repo := &fakeFavoriteRepo{}
	svc := NewUserResourceFavoriteService(repo)

	require.ErrorIs(t, svc.Remove(context.Background(), "user-1", 7, "bogus", "kb-1"), ErrFavoriteInvalidType)
	require.ErrorIs(t, svc.Remove(context.Background(), "user-1", 7, types.ResourceTypeKB, ""), ErrFavoriteEmptyID)
	require.ErrorIs(t, svc.Remove(context.Background(), "user-1", 7, types.ResourceTypeKB, "  "), ErrFavoriteEmptyID,
		"a whitespace-only id is treated as empty")
	require.Empty(t, repo.removeCalls, "invalid requests never reach the repository")
}

func TestFavoriteServiceRemovePassesThroughValidRequest(t *testing.T) {
	repo := &fakeFavoriteRepo{}
	svc := NewUserResourceFavoriteService(repo)

	require.NoError(t, svc.Remove(context.Background(), "user-1", 7, types.ResourceTypeKB, "kb-1"))
	require.Equal(t, []string{"user-1|kb|kb-1"}, repo.removeCalls)
}
