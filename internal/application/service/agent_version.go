// Agent-domain immutable Tenant agent version service (T28 Wave 1, Task 1).
// The semantic this file implements: freezing an agent COPIES it — the
// tenant-scoped CustomAgent is serialized once, canonically, and appended as
// an immutable agent_versions row with a content digest. Later edits or
// deletion of the live agent never rewrite history; the Marketplace built in
// later tasks references the frozen ID + digest and never re-reads the
// source agent.
package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// AgentVersionAgentSource is the agent-read slice the freeze flow needs:
// one tenant-scoped agent lookup. interfaces.CustomAgentService satisfies
// it; tests fake it. The service consumes the existing custom-agent loading
// logic instead of duplicating it.
type AgentVersionAgentSource interface {
	GetAgentByIDAndTenant(ctx context.Context, id string, tenantID uint64) (*types.CustomAgent, error)
}

// AgentVersionService implements interfaces.AgentVersionService.
type AgentVersionService struct {
	agents   AgentVersionAgentSource
	versions repository.AgentVersionRepository
	now      func() time.Time
}

var _ interfaces.AgentVersionService = (*AgentVersionService)(nil)

// NewAgentVersionService wires the seams. agents is the custom agent
// service, versions the append-only agent version repository.
func NewAgentVersionService(
	agents AgentVersionAgentSource,
	versions repository.AgentVersionRepository,
) *AgentVersionService {
	return &AgentVersionService{agents: agents, versions: versions, now: time.Now}
}

// agentVersionSnapshot serializes the frozen agent canonically and returns
// the JSON bytes with their SHA-256 digest. encoding/json orders map keys
// deterministically, so the same agent state always hashes to the same
// digest — and the digest is computed over the exact stored bytes, so a
// later read verifies immutability byte-for-byte.
func agentVersionSnapshot(agent *types.CustomAgent) (string, string, error) {
	clone := *agent
	clone.CreatorName = "" // list-handler decoration, never part of the source row
	raw, err := json.Marshal(clone)
	if err != nil {
		return "", "", err
	}
	sum := sha256.Sum256(raw)
	return string(raw), hex.EncodeToString(sum[:]), nil
}

func agentVersionViewOf(row *types.AgentVersionEntity) interfaces.AgentVersionView {
	return interfaces.AgentVersionView{
		ID:            row.ID,
		AgentID:       row.AgentID,
		VersionNumber: row.VersionNumber,
		SourceSHA256:  row.SourceSHA256,
		FrozenBy:      row.FrozenBy,
		CreatedAt:     row.CreatedAt,
	}
}

// decodeAgentVersionSnapshot verifies the stored bytes against the recorded
// digest (the immutability contract: tampered content refuses to serve) and
// decodes the frozen CustomAgent.
func decodeAgentVersionSnapshot(row *types.AgentVersionEntity) (*types.CustomAgent, error) {
	sum := sha256.Sum256([]byte(row.Snapshot))
	if hex.EncodeToString(sum[:]) != row.SourceSHA256 {
		return nil, apperrors.NewInternalServerError(
			"the frozen agent version does not match its recorded digest")
	}
	var agent types.CustomAgent
	if err := json.Unmarshal([]byte(row.Snapshot), &agent); err != nil {
		return nil, apperrors.NewInternalServerError("decode the frozen agent snapshot: " + err.Error())
	}
	return &agent, nil
}

// FreezeAgentVersion implements interfaces.AgentVersionService. The source
// agent must live in THIS tenant: another tenant's agent reads as absent
// (the wrong-tenant 404), exactly like the expert-market publish flow.
func (s *AgentVersionService) FreezeAgentVersion(
	ctx context.Context, tenantID uint64, actorID, agentID string,
) (interfaces.AgentVersionView, error) {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return interfaces.AgentVersionView{}, apperrors.NewNotFoundError("agent not found")
	}
	agent, err := s.agents.GetAgentByIDAndTenant(ctx, agentID, tenantID)
	if err != nil {
		if errors.Is(err, ErrAgentNotFound) {
			return interfaces.AgentVersionView{}, apperrors.NewNotFoundError("agent not found")
		}
		return interfaces.AgentVersionView{}, apperrors.NewInternalServerError("load the agent to freeze: " + err.Error())
	}
	if agent == nil {
		return interfaces.AgentVersionView{}, apperrors.NewNotFoundError("agent not found")
	}

	snapshot, digest, err := agentVersionSnapshot(agent)
	if err != nil {
		return interfaces.AgentVersionView{}, apperrors.NewInternalServerError("serialize the agent snapshot: " + err.Error())
	}

	row, err := s.versions.Freeze(ctx, &types.AgentVersionEntity{
		TenantID: tenantID, AgentID: agentID,
		Snapshot: snapshot, SourceSHA256: digest,
		FrozenBy:  strings.TrimSpace(actorID),
		CreatedAt: s.now(),
	})
	if err != nil {
		return interfaces.AgentVersionView{}, apperrors.NewInternalServerError("freeze the agent version: " + err.Error())
	}
	logger.Infof(ctx, "Froze agent %s of tenant %d as immutable version %d (source sha %s)",
		agentID, tenantID, row.VersionNumber, row.SourceSHA256)
	return agentVersionViewOf(row), nil
}

// GetAgentVersion implements interfaces.AgentVersionService. Unknown,
// malformed and other-tenant version ids all read the same way here — the
// wrong-tenant 404.
func (s *AgentVersionService) GetAgentVersion(
	ctx context.Context, tenantID uint64, versionID string,
) (interfaces.AgentVersionSnapshot, error) {
	row, err := s.versions.GetByTenantAndID(ctx, tenantID, versionID)
	if err != nil {
		return interfaces.AgentVersionSnapshot{}, err
	}
	if row == nil {
		return interfaces.AgentVersionSnapshot{}, apperrors.NewNotFoundError("agent version not found")
	}
	agent, err := decodeAgentVersionSnapshot(row)
	if err != nil {
		return interfaces.AgentVersionSnapshot{}, err
	}
	return interfaces.AgentVersionSnapshot{AgentVersionView: agentVersionViewOf(row), Agent: agent}, nil
}

// ListAgentVersions implements interfaces.AgentVersionService. Another
// tenant's agent simply has no versions under this tenant's scope.
func (s *AgentVersionService) ListAgentVersions(
	ctx context.Context, tenantID uint64, agentID string,
) ([]interfaces.AgentVersionView, error) {
	rows, err := s.versions.ListByTenantAndAgent(ctx, tenantID, agentID)
	if err != nil {
		return nil, err
	}
	views := make([]interfaces.AgentVersionView, 0, len(rows))
	for i := range rows {
		views = append(views, agentVersionViewOf(&rows[i]))
	}
	return views, nil
}
