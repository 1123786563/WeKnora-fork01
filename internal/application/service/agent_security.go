package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

var (
	ErrAgentSecurityInvalidInput        = errors.New("invalid agent security revocation request")
	ErrAgentSecurityNotFound            = errors.New("agent security revocation not found")
	ErrAgentSecurityReleaseUnresolvable = errors.New("release is not resolvable in this tenant")
	ErrAgentSecurityReleaseBlocked      = errors.New("agent security policy blocked the release")
)

// ReleaseSecurityGate is the governance seam consumed by adoption and upgrade
// flows. A nil gate keeps those flows unchanged.
type ReleaseSecurityGate interface {
	ReleaseAdmission(ctx context.Context, tenantID uint64, releaseID string) error
}

var _ interfaces.AgentSecurityService = (*AgentSecurityService)(nil)

type AgentSecurityService struct {
	store *repository.AgentSecurityStore
	runs  *repository.AgentRunStore
	now   func() time.Time
}

func NewAgentSecurityService(store *repository.AgentSecurityStore, runs *repository.AgentRunStore) *AgentSecurityService {
	return &AgentSecurityService{store: store, runs: runs, now: func() time.Time { return time.Now().UTC() }}
}

func (s *AgentSecurityService) RevokeRelease(context.Context, uint64, string, interfaces.ReleaseRevocationInput) (interfaces.AgentSecurityRevocationView, error) {
	return interfaces.AgentSecurityRevocationView{}, ErrAgentSecurityInvalidInput
}

func (s *AgentSecurityService) RevokeDependency(context.Context, uint64, string, interfaces.DependencyRevocationInput) (interfaces.AgentSecurityRevocationView, error) {
	return interfaces.AgentSecurityRevocationView{}, ErrAgentSecurityInvalidInput
}

func (s *AgentSecurityService) ListRevocations(context.Context, uint64) ([]interfaces.AgentSecurityRevocationView, error) {
	return nil, ErrAgentSecurityInvalidInput
}

func (s *AgentSecurityService) GetRevocation(context.Context, uint64, string) (interfaces.AgentSecurityRevocationView, error) {
	return interfaces.AgentSecurityRevocationView{}, ErrAgentSecurityInvalidInput
}

func (s *AgentSecurityService) VerdictForAgent(ctx context.Context, tenantID uint64, agentID string) (interfaces.AgentSecurityVerdict, error) {
	if agentID == "" {
		return okAgentSecurityVerdict(), nil
	}
	variants, err := s.store.VariantsByLocalAgent(ctx, tenantID, agentID)
	if err != nil {
		return interfaces.AgentSecurityVerdict{}, err
	}
	if len(variants) == 0 {
		return okAgentSecurityVerdict(), nil
	}
	return s.verdictForRelease(ctx, tenantID, variants[0].ReleaseID)
}

func (s *AgentSecurityService) ReleaseAdmission(ctx context.Context, tenantID uint64, releaseID string) error {
	verdict, err := s.verdictForRelease(ctx, tenantID, releaseID)
	if err != nil {
		return err
	}
	if verdict.Blocked() {
		return fmt.Errorf("%w: %s", ErrAgentSecurityReleaseBlocked, verdict.Reason)
	}
	return nil
}

func (s *AgentSecurityService) verdictForRelease(ctx context.Context, tenantID uint64, releaseID string) (interfaces.AgentSecurityVerdict, error) {
	releaseRevocations, err := s.store.ListReleaseRevocations(ctx, tenantID)
	if err != nil {
		return interfaces.AgentSecurityVerdict{}, err
	}
	var latest *types.AgentReleaseRevocationEntity
	for i := range releaseRevocations {
		row := &releaseRevocations[i]
		if row.ReleaseID != releaseID {
			continue
		}
		if latest == nil || !row.CreatedAt.Before(latest.CreatedAt) {
			latest = row
		}
	}
	if latest != nil {
		return interfaces.AgentSecurityVerdict{
			State:                interfaces.AgentSecurityVerdictReleaseRevoked,
			Reason:               fmt.Sprintf("release %s security-revoked: %s", releaseID, latest.Reason),
			RevocationID:         latest.ID,
			ReleaseID:            releaseID,
			ReplacementReleaseID: latest.ReplacementReleaseID,
		}, nil
	}

	_, lockJSON, found, err := s.store.ReleaseFacts(ctx, tenantID, releaseID)
	if err != nil {
		return interfaces.AgentSecurityVerdict{}, err
	}
	if !found {
		return interfaces.AgentSecurityVerdict{}, ErrAgentSecurityReleaseUnresolvable
	}
	var lock types.DependencyLock
	if err := json.Unmarshal([]byte(lockJSON), &lock); err != nil {
		return interfaces.AgentSecurityVerdict{}, fmt.Errorf("decode dependency lock for release %s: %w", releaseID, err)
	}
	dependencyRevocations, err := s.store.ListDependencyRevocations(ctx, tenantID)
	if err != nil {
		return interfaces.AgentSecurityVerdict{}, err
	}
	for _, dependency := range lock.Dependencies {
		for i := range dependencyRevocations {
			revocation := &dependencyRevocations[i]
			if !dependencyIdentityMatches(dependency, *revocation) {
				continue
			}
			lockedDependency := dependency
			return interfaces.AgentSecurityVerdict{
				State:              interfaces.AgentSecurityVerdictDependencyBlocked,
				Reason:             fmt.Sprintf("locked dependency %s/%s@%s sha256:%s is security-revoked: %s", dependency.Type, dependency.ID, dependency.Version, dependency.Digest, revocation.Reason),
				RevocationID:       revocation.ID,
				ReleaseID:          releaseID,
				Dependency:         &lockedDependency,
				ReplacementVersion: revocation.ReplacementVersion,
			}, nil
		}
	}
	return okAgentSecurityVerdict(), nil
}

func dependencyIdentityMatches(dependency types.AgentReleaseDependency, revocation types.AgentDependencyRevocationEntity) bool {
	return dependency.Type == revocation.DepType &&
		dependency.ID == revocation.DepID &&
		dependency.Version == revocation.DepVersion &&
		dependency.Digest == revocation.DepDigest
}

func okAgentSecurityVerdict() interfaces.AgentSecurityVerdict {
	return interfaces.AgentSecurityVerdict{State: interfaces.AgentSecurityVerdictOK}
}
