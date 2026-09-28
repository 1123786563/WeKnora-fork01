package interfaces

import (
	"context"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
)

const (
	AgentSecurityRevocationKindRelease    = "release"
	AgentSecurityRevocationKindDependency = "dependency"
	AgentSecurityVerdictOK                = "ok"
	AgentSecurityVerdictReleaseRevoked    = "release-revoked"
	AgentSecurityVerdictDependencyBlocked = "dependency-blocked"
	AgentSecurityInFlightCancel           = "cancel"
	AgentSecurityInFlightAllow            = "allow"
)

type AgentSecurityVerdict struct {
	State                string                        `json:"state"`
	Reason               string                        `json:"reason,omitempty"`
	RevocationID         string                        `json:"revocation_id,omitempty"`
	ReleaseID            string                        `json:"release_id,omitempty"`
	Dependency           *types.AgentReleaseDependency `json:"dependency,omitempty"`
	ReplacementReleaseID string                        `json:"replacement_release_id,omitempty"`
	ReplacementVersion   string                        `json:"replacement_version,omitempty"`
}

func (v AgentSecurityVerdict) Blocked() bool { return v.State != AgentSecurityVerdictOK }

type AgentBlockedRelease struct{ ReleaseID, ListingID, BlockedBy string }
type AgentBlockedVariant struct{ VariantID, AdoptionID, ReleaseID, State, LocalAgentID string }
type AgentRevocationScope struct {
	BlockedReleases     []AgentBlockedRelease `json:"blocked_releases"`
	AffectedAdoptionIDs []string              `json:"affected_adoption_ids"`
	AffectedVariants    []AgentBlockedVariant `json:"affected_variants"`
}
type ReleaseRevocationInput struct{ ReleaseID, Reason, ReplacementReleaseID, InFlightDisposition string }
type DependencyRevocationInput struct {
	Dependency                                      types.AgentReleaseDependency
	Reason, ReplacementVersion, InFlightDisposition string
}
type AgentSecurityRevocationView struct {
	ID                   string                        `json:"id"`
	Kind                 string                        `json:"kind"`
	Reason               string                        `json:"reason"`
	RevokedBy            string                        `json:"revoked_by"`
	RevokedAt            time.Time                     `json:"revoked_at"`
	InFlightDisposition  string                        `json:"in_flight_disposition"`
	CanceledRunCount     int64                         `json:"canceled_run_count"`
	ListingID            string                        `json:"listing_id,omitempty"`
	ReleaseID            string                        `json:"release_id,omitempty"`
	ReplacementReleaseID string                        `json:"replacement_release_id,omitempty"`
	Dependency           *types.AgentReleaseDependency `json:"dependency,omitempty"`
	ReplacementVersion   string                        `json:"replacement_version,omitempty"`
	Scope                *AgentRevocationScope         `json:"scope,omitempty"`
}
type AgentSecurityService interface {
	RevokeRelease(context.Context, uint64, string, ReleaseRevocationInput) (AgentSecurityRevocationView, error)
	RevokeDependency(context.Context, uint64, string, DependencyRevocationInput) (AgentSecurityRevocationView, error)
	ListRevocations(context.Context, uint64) ([]AgentSecurityRevocationView, error)
	GetRevocation(context.Context, uint64, string) (AgentSecurityRevocationView, error)
	VerdictForAgent(context.Context, uint64, string) (AgentSecurityVerdict, error)
	ReleaseAdmission(context.Context, uint64, string) error
}
