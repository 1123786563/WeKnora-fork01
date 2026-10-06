package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Tencent/WeKnora/internal/modules/policy/access"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/require"
)

type reparseFailureKnowledgeRepo struct {
	interfaces.KnowledgeRepository
	knowledge   *types.Knowledge
	updateCalls int
}

func (r *reparseFailureKnowledgeRepo) GetKnowledgeByID(
	_ context.Context,
	_ uint64,
	_ string,
) (*types.Knowledge, error) {
	return r.knowledge, nil
}

func (r *reparseFailureKnowledgeRepo) UpdateKnowledge(
	_ context.Context,
	_ *types.Knowledge,
) error {
	r.updateCalls++
	return nil
}

func (r *reparseFailureKnowledgeRepo) UpdateKnowledgeColumn(
	_ context.Context,
	_ string,
	_ string,
	_ interface{},
) error {
	return nil
}

type reparseFailureKBService struct {
	interfaces.KnowledgeBaseService
	kb *types.KnowledgeBase
}

func (s *reparseFailureKBService) GetKnowledgeBaseByID(
	_ context.Context,
	_ string,
) (*types.KnowledgeBase, error) {
	return s.kb, nil
}

type failingReparseTaskEnqueuer struct {
	err error
}

func (e failingReparseTaskEnqueuer) Enqueue(
	_ *asynq.Task,
	_ ...asynq.Option,
) (*asynq.TaskInfo, error) {
	return nil, e.err
}

func TestReparseKnowledgeManualEnqueueFailureIsVisible(t *testing.T) {
	enqueueErr := errors.New("queue unavailable")
	knowledge := &types.Knowledge{
		ID:              "knowledge-1",
		TenantID:        7,
		KnowledgeBaseID: "kb-1",
		Type:            types.KnowledgeTypeManual,
		ParseStatus:     types.ParseStatusCompleted,
		EnableStatus:    "enabled",
	}
	require.NoError(t, knowledge.SetManualMetadata(
		types.NewManualKnowledgeMetadata("# content", types.ManualKnowledgeStatusPublish, 1),
	))
	repo := &reparseFailureKnowledgeRepo{knowledge: knowledge}
	svc := &knowledgeService{
		repo:      repo,
		kbService: &reparseFailureKBService{kb: &types.KnowledgeBase{ID: "kb-1", TenantID: 7}},
		task:      failingReparseTaskEnqueuer{err: enqueueErr},
	}
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(7))

	ctx, grantErr := access.WithKBTaskWrite(ctx, &types.KnowledgeBase{ID: "kb-1", TenantID: 7}, 7)
	require.NoError(t, grantErr)

	got, err := svc.ReparseKnowledge(ctx, knowledge.ID, nil)

	require.Error(t, err)
	require.NotNil(t, got)
	require.Equal(t, types.ParseStatusFailed, got.ParseStatus)
	require.Equal(t, "disabled", got.EnableStatus)
	require.Equal(t, "Failed to enqueue processing task", got.ErrorMessage)
	require.GreaterOrEqual(t, repo.updateCalls, 2, "pending and failed states must both be persisted")
}

func TestRunKnowledgeListReparseSubmissionsReportsPartialFailure(t *testing.T) {
	firstErr := errors.New("first failed")
	secondErr := errors.New("second failed")
	var attempted []string

	outcome, err := runKnowledgeListReparseSubmissions(
		[]string{"ok-1", "bad-1", "ok-2", "bad-2"},
		func(id string) error {
			attempted = append(attempted, id)
			switch id {
			case "bad-1":
				return firstErr
			case "bad-2":
				return secondErr
			default:
				return nil
			}
		},
	)

	require.Equal(t, []string{"ok-1", "bad-1", "ok-2", "bad-2"}, attempted)
	require.Equal(t, knowledgeListReparseOutcome{Submitted: 2, Failed: 2}, outcome)
	require.ErrorIs(t, err, asynq.SkipRetry)
	require.ErrorIs(t, err, firstErr)
	require.ErrorIs(t, err, secondErr)
	require.ErrorContains(t, err, "knowledge bad-1")
	require.ErrorContains(t, err, "knowledge bad-2")
}

func TestRunKnowledgeListReparseSubmissionsSucceeds(t *testing.T) {
	outcome, err := runKnowledgeListReparseSubmissions(
		[]string{"knowledge-1", "knowledge-2"},
		func(string) error { return nil },
	)

	require.NoError(t, err)
	require.Equal(t, knowledgeListReparseOutcome{Submitted: 2}, outcome)
}

// reparseConfigScenario builds the manual-knowledge + queue stubs shared by
// the #3851 reparse-config scenarios: the upload confirm step runs against
// kbUpload, then the KB object itself is reconfigured in place to simulate the
// settings change that happens between upload and reparse.
func reparseConfigScenario(t *testing.T, kb *types.KnowledgeBase) (*knowledgeService, *types.Knowledge, context.Context) {
	t.Helper()
	knowledge := &types.Knowledge{
		ID: "knowledge-1", TenantID: 7, KnowledgeBaseID: "kb-1",
		Type: types.KnowledgeTypeManual, ParseStatus: types.ParseStatusCompleted,
	}
	require.NoError(t, knowledge.SetManualMetadata(
		types.NewManualKnowledgeMetadata("# content", types.ManualKnowledgeStatusPublish, 1)))
	svc := &knowledgeService{
		repo:      &reparseFailureKnowledgeRepo{knowledge: knowledge},
		kbService: &reparseFailureKBService{kb: kb},
		task:      &wikiEnqueueFailureTaskQueue{},
	}
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(7))
	ctx, err := access.WithKBTaskWrite(ctx, kb, 7)
	require.NoError(t, err)
	return svc, knowledge, ctx
}

// Scenario (a) of #3851: upload with question generation on (KB default,
// nothing overridden), KB disables it, reparse with an empty body — the new
// snapshot must resolve question generation off instead of replaying the
// upload-time dialog snapshot.
func TestReparseKnowledgeFollowsLatestKBConfigWhenNotOverridden(t *testing.T) {
	kb := &types.KnowledgeBase{
		ID: "kb-1", TenantID: 7,
		ChunkingConfig:           types.ChunkingConfig{ChunkSize: 512, ChunkOverlap: 50},
		QuestionGenerationConfig: &types.QuestionGenerationConfig{Enabled: true, QuestionCount: 3},
	}
	svc, knowledge, ctx := reparseConfigScenario(t, kb)

	// Upload confirm: the dialog posts its full prefilled state; the service
	// persists only deviations, so nothing is stored.
	_, err := ApplyKnowledgeProcessOverrides(ctx, kb, knowledge, dialogStyleOverrides(kb, nil), nil, nil)
	require.NoError(t, err)

	// The KB is reconfigured between upload and reparse.
	kb.QuestionGenerationConfig = &types.QuestionGenerationConfig{Enabled: false}
	kb.ChunkingConfig.ChunkSize = 1024

	got, err := svc.ReparseKnowledge(ctx, knowledge.ID, nil)
	require.NoError(t, err)
	stored, err := got.ProcessOverrides()
	require.NoError(t, err)
	eff := ResolveProcessConfig(kb, stored)
	require.False(t, eff.QuestionGenerationConfig.Enabled,
		"reparse must resolve the KB's latest question generation setting")
	require.Equal(t, 1024, eff.ChunkingConfig.ChunkSize,
		"reparse must resolve the KB's latest chunking config")
}

// Scenario (b) of #3851: a chunk size the user explicitly set at upload stays
// pinned across a later KB change, while the fields the user never overrode
// follow the KB's new values.
func TestReparseKnowledgeKeepsUploadExplicitOverrideOverKBChange(t *testing.T) {
	kb := &types.KnowledgeBase{
		ID: "kb-1", TenantID: 7,
		ChunkingConfig:           types.ChunkingConfig{ChunkSize: 512, ChunkOverlap: 50},
		QuestionGenerationConfig: &types.QuestionGenerationConfig{Enabled: true, QuestionCount: 3},
	}
	svc, knowledge, ctx := reparseConfigScenario(t, kb)

	_, err := ApplyKnowledgeProcessOverrides(ctx, kb, knowledge, dialogStyleOverrides(kb,
		&types.KnowledgeProcessOverrides{
			ChunkingConfig: &types.ChunkingConfig{ChunkSize: 2048, ChunkOverlap: 50},
		}), nil, nil)
	require.NoError(t, err)

	kb.ChunkingConfig.ChunkSize = 1024
	kb.QuestionGenerationConfig = &types.QuestionGenerationConfig{Enabled: false}

	got, err := svc.ReparseKnowledge(ctx, knowledge.ID, nil)
	require.NoError(t, err)
	stored, err := got.ProcessOverrides()
	require.NoError(t, err)
	require.NotNil(t, stored.ChunkingConfig,
		"the upload-time explicit chunking override must survive the reparse")
	eff := ResolveProcessConfig(kb, stored)
	require.Equal(t, 2048, eff.ChunkingConfig.ChunkSize,
		"an explicit upload override wins over the KB's later change")
	require.False(t, eff.QuestionGenerationConfig.Enabled,
		"fields the user never overrode follow the KB's latest config")
}

// Scenario (c) of #3851: overrides the reparse request supplies take
// priority; fields the request omits keep the upload-time explicit override.
func TestReparseKnowledgeRequestOverridesTakePriority(t *testing.T) {
	kb := &types.KnowledgeBase{
		ID: "kb-1", TenantID: 7,
		ChunkingConfig:           types.ChunkingConfig{ChunkSize: 512, ChunkOverlap: 50},
		QuestionGenerationConfig: &types.QuestionGenerationConfig{Enabled: true, QuestionCount: 3},
	}
	svc, knowledge, ctx := reparseConfigScenario(t, kb)

	_, err := ApplyKnowledgeProcessOverrides(ctx, kb, knowledge, dialogStyleOverrides(kb,
		&types.KnowledgeProcessOverrides{
			ChunkingConfig: &types.ChunkingConfig{ChunkSize: 2048, ChunkOverlap: 50},
		}), nil, nil)
	require.NoError(t, err)

	kb.ChunkingConfig.ChunkSize = 1024
	kb.QuestionGenerationConfig = &types.QuestionGenerationConfig{Enabled: false}

	// The request re-enables question generation (a deviation from the KB's
	// current setting) while saying nothing about chunking.
	got, err := svc.ReparseKnowledge(ctx, knowledge.ID, &types.KnowledgeProcessOverrides{
		QuestionGenerationConfig: &types.QuestionGenerationConfig{Enabled: true, QuestionCount: 5},
	})
	require.NoError(t, err)
	stored, err := got.ProcessOverrides()
	require.NoError(t, err)
	require.NotNil(t, stored.QuestionGenerationConfig, "the new request override must be persisted")
	require.Equal(t, 5, stored.QuestionGenerationConfig.QuestionCount)
	require.NotNil(t, stored.ChunkingConfig, "the omitted field keeps the upload-time override")
	eff := ResolveProcessConfig(kb, stored)
	require.True(t, eff.QuestionGenerationConfig.Enabled, "the request override wins")
	require.Equal(t, 2048, eff.ChunkingConfig.ChunkSize, "the omitted override survives")

	// A request that overrides the same field replaces the stored choice.
	got, err = svc.ReparseKnowledge(ctx, knowledge.ID, &types.KnowledgeProcessOverrides{
		ChunkingConfig: &types.ChunkingConfig{ChunkSize: 4096, ChunkOverlap: 50},
	})
	require.NoError(t, err)
	stored, err = got.ProcessOverrides()
	require.NoError(t, err)
	require.Equal(t, 4096, ResolveProcessConfig(kb, stored).ChunkingConfig.ChunkSize,
		"the newest explicit chunking choice wins")
}

func TestReparseKnowledgePreservesOrChangesSummaryChoice(t *testing.T) {
	for _, tc := range []struct {
		name      string
		overrides *types.KnowledgeProcessOverrides
		want      bool
	}{
		{name: "reuse upload choice"},
		{
			name:      "explicitly keep disabled",
			overrides: &types.KnowledgeProcessOverrides{SummaryEnabled: processConfigBoolPtr(false)},
		},
		{
			name:      "enable on reparse",
			overrides: &types.KnowledgeProcessOverrides{SummaryEnabled: processConfigBoolPtr(true)},
			want:      true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			knowledge := &types.Knowledge{
				ID: "knowledge-1", TenantID: 7, KnowledgeBaseID: "kb-1",
				Type: types.KnowledgeTypeManual, ParseStatus: types.ParseStatusCompleted,
			}
			metadata := types.NewManualKnowledgeMetadata("# content", types.ManualKnowledgeStatusPublish, 1)
			require.NoError(t, knowledge.SetManualMetadata(metadata))
			require.NoError(t, knowledge.SetProcessOverrides(&types.KnowledgeProcessOverrides{
				SummaryEnabled: processConfigBoolPtr(false),
			}))
			kb := &types.KnowledgeBase{ID: "kb-1", TenantID: 7}
			queue := &wikiEnqueueFailureTaskQueue{}
			svc := &knowledgeService{
				repo:      &reparseFailureKnowledgeRepo{knowledge: knowledge},
				kbService: &reparseFailureKBService{kb: kb}, task: queue,
			}
			ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(7))
			ctx, err := access.WithKBTaskWrite(ctx, kb, 7)
			require.NoError(t, err)
			got, err := svc.ReparseKnowledge(ctx, knowledge.ID, tc.overrides)
			require.NoError(t, err)
			require.Equal(t, []string{types.TypeManualProcess}, queue.taskTypes)
			overrides, err := got.ProcessOverrides()
			require.NoError(t, err)
			// Since #3851 reparse persists only overrides that deviate from
			// the KB: asking to re-enable the KB default stores nothing, so
			// the stored pointer is optional — the effective choice is what
			// must hold in every case.
			if overrides != nil && overrides.SummaryEnabled != nil {
				require.Equal(t, tc.want, *overrides.SummaryEnabled)
			}
			require.Equal(t, tc.want, ResolveProcessConfig(kb, overrides).SummaryEnabled)
		})
	}
}
