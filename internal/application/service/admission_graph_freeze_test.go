package service

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/config"
	"github.com/stretchr/testify/require"
)

// WB-GRAPH review I-1: the short-code mapping had zero coverage — the five
// freezer tests all used stubs or pre-built bytes. These tables pin it end to
// end: the pure mapper (4 codes + fail-closed default) and the production
// sessionService freezer over seeded fixtures driving the real
// GetAgentByID → GetSessionByID → buildAgentConfig → resolveChatModelID chain.
func TestGraphResolutionFailureShortCodeMapping(t *testing.T) {
	for _, tc := range []struct {
		name    string
		message string
		want    string
	}{
		{"model_unresolved", "chat model is not configured: please set model_id on agent wb-1", "model_unresolved"},
		{"model_unavailable", "configured chat model m-1 is unavailable for agent wb-1", "model_unavailable"},
		{"rerank_unresolved", "rerank model is not configured: please set rerank_model_id on the agent", "rerank_unresolved"},
		{"unknown upstream error fails closed as config_unbuildable", "load agent wb-1: record not found", "config_unbuildable"},
		{"empty message still carries the default code", "", "config_unbuildable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := graphResolutionFailure(errors.New(tc.message))
			require.Error(t, err)
			require.True(t, strings.HasPrefix(err.Error(), tc.want+":"), "short code prefix missing: %v", err)
			require.True(t, strings.HasSuffix(err.Error(), tc.message), "wrapped message must stay verbatim: %v", err)
		})
	}
}

func newFreezerFixtureService(t *testing.T) *sessionService {
	t.Helper()
	db := openDurableRunTestDB(t) // tenant 1, user u1, session s1
	require.NoError(t, db.Exec(`INSERT INTO models (id, tenant_id, name, type, source, parameters, is_default)
		VALUES ('model-ok', 1, 'chat', 'KnowledgeQA', 'builtin', '{}', 1)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO models (id, tenant_id, name, type, source, parameters)
		VALUES ('model-rerank', 1, 'rerank', 'rerank', 'builtin', '{}')`).Error)
	seed := func(id, agentConfig string) {
		require.NoError(t, db.Exec(
			`INSERT INTO custom_agents (id, name, is_builtin, tenant_id, created_by, config)
			 VALUES (?, ?, 0, 1, 'u1', ?)`, id, id, agentConfig).Error)
	}
	seed("wb-no-model", `{"agent_mode":"quick-answer","allowed_tools":["thinking"]}`)
	seed("wb-stale-model", `{"agent_mode":"quick-answer","model_id":"model-rerank","allowed_tools":["thinking"]}`)
	seed("wb-rerank-missing", `{"agent_mode":"quick-answer","model_id":"model-ok","allowed_tools":["knowledge_search"]}`)
	seed("wb-happy", `{"agent_mode":"quick-answer","model_id":"model-ok","allowed_tools":["thinking"]}`)
	return &sessionService{
		cfg:                   &config.Config{},
		sessionRepo:           repository.NewSessionRepository(db),
		modelService:          NewModelService(repository.NewModelRepository(db), nil, nil, nil, nil, nil),
		customAgents:          NewCustomAgentService(repository.NewCustomAgentRepository(db), nil, nil, nil, nil, nil, nil),
		webSearchProviderRepo: repository.NewWebSearchProviderRepository(db),
	}
}

func TestFreezeAdmissionGraphCoreResolutionFailuresOverProductionChain(t *testing.T) {
	svc := newFreezerFixtureService(t)
	ctx := durableRunCtx()
	for _, tc := range []struct {
		name    string
		agentID string
		want    string
	}{
		{"missing agent fails closed as config_unbuildable", "wb-missing", "config_unbuildable"},
		{"agent without model_id is model_unresolved", "wb-no-model", "model_unresolved"},
		{"agent pointing at a non-chat model is model_unavailable", "wb-stale-model", "model_unavailable"},
		{"retrieval agent without rerank model is rerank_unresolved", "wb-rerank-missing", "rerank_unresolved"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := svc.freezeAdmissionGraphCore(ctx, 1, "u1", "s1", tc.agentID, "整理本周周报")
			require.Nil(t, raw)
			require.Error(t, err)
			require.True(t, strings.HasPrefix(err.Error(), tc.want+":"), "got %v", err)
		})
	}

	t.Run("fully resolved agent freezes a graph core snapshot", func(t *testing.T) {
		raw, err := svc.freezeAdmissionGraphCore(ctx, 1, "u1", "s1", "wb-happy", "整理本周周报")
		require.NoError(t, err)
		parsed, err := ParseDurableRunSnapshot(raw)
		require.NoError(t, err)
		require.Equal(t, "model-ok", parsed.ModelID)
		require.Equal(t, "整理本周周报", parsed.Query)
		var fields map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(raw, &fields))
		require.Contains(t, fields, "agent_config")
		require.Contains(t, fields, "runtime")
	})
}
