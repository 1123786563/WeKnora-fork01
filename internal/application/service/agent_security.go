package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

var (
	ErrAgentSecurityInvalidInput        = errors.New("invalid agent security revocation request")
	ErrAgentSecurityNotFound            = errors.New("agent security revocation not found")
	ErrAgentSecurityReleaseUnresolvable = repository.ErrAgentSecurityReleaseUnresolvable
	ErrAgentSecurityReleaseBlocked      = repository.ErrAgentSecurityReleaseBlocked
)

// ReleaseSecurityGate is the governance seam consumed by adoption and upgrade
// flows. A nil gate keeps those flows unchanged.
type ReleaseSecurityGate interface {
	ReleaseAdmission(ctx context.Context, tenantID uint64, releaseID string) error
}

var _ interfaces.AgentSecurityService = (*AgentSecurityService)(nil)

const invalidInFlightDisposition = "invalid"

func normalizeInFlightDisposition(disposition string) string {
	switch strings.TrimSpace(disposition) {
	case "":
		return interfaces.AgentSecurityInFlightCancel
	case interfaces.AgentSecurityInFlightCancel:
		return interfaces.AgentSecurityInFlightCancel
	case interfaces.AgentSecurityInFlightAllow:
		return interfaces.AgentSecurityInFlightAllow
	default:
		return invalidInFlightDisposition
	}
}

func revocationAudit(action types.AuditAction, tenantID uint64, actorID, targetType, targetID string, details map[string]string, now time.Time) (*types.AuditLog, error) {
	encoded, err := json.Marshal(details)
	if err != nil {
		return nil, err
	}
	return &types.AuditLog{TenantID: tenantID, ActorUserID: actorID, Action: action,
		ScopeType: "marketplace", TargetType: targetType, TargetID: targetID,
		Details: types.JSON(encoded), CreatedAt: now}, nil
}

func releaseRevocationView(row *types.AgentReleaseRevocationEntity) interfaces.AgentSecurityRevocationView {
	return interfaces.AgentSecurityRevocationView{ID: row.ID, Kind: interfaces.AgentSecurityRevocationKindRelease,
		Reason: row.Reason, RevokedBy: row.RevokedBy, RevokedAt: row.RevokedAt,
		InFlightDisposition: row.InFlightDisposition, CanceledRunCount: row.CanceledRunCount,
		ListingID: row.ListingID, ReleaseID: row.ReleaseID, ReplacementReleaseID: row.ReplacementReleaseID}
}

func dependencyRevocationView(row *types.AgentDependencyRevocationEntity) interfaces.AgentSecurityRevocationView {
	dependency := &types.AgentReleaseDependency{Type: row.DepType, ID: row.DepID, Version: row.DepVersion, Digest: row.DepDigest}
	return interfaces.AgentSecurityRevocationView{ID: row.ID, Kind: interfaces.AgentSecurityRevocationKindDependency,
		Reason: row.Reason, RevokedBy: row.RevokedBy, RevokedAt: row.RevokedAt,
		InFlightDisposition: row.InFlightDisposition, CanceledRunCount: row.CanceledRunCount,
		Dependency: dependency, ReplacementVersion: row.ReplacementVersion}
}

func (s *AgentSecurityService) blockedReleasesForDependency(ctx context.Context, tenantID uint64, dependency types.AgentReleaseDependency) (map[string]string, error) {
	locks, err := s.store.ListTenantReleaseLocks(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	blocked := make(map[string]string)
	for _, release := range locks {
		lock, err := decodeDependencyLock(release.LockJSON)
		if err != nil {
			return nil, fmt.Errorf("decode dependency lock for release %s: %w", release.ReleaseID, err)
		}
		for _, locked := range lock.Dependencies {
			if locked.Type == dependency.Type && locked.ID == dependency.ID && locked.Version == dependency.Version && locked.Digest == dependency.Digest {
				blocked[release.ReleaseID] = release.ListingID
				break
			}
		}
	}
	return blocked, nil
}

func (s *AgentSecurityService) publishedAgentIDsForReleases(ctx context.Context, tenantID uint64, releases map[string]string) ([]string, error) {
	variants, err := s.store.ListVariants(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{})
	ids := make([]string, 0)
	for _, variant := range variants {
		if variant.State != "published" || variant.LocalAgentID == "" {
			continue
		}
		if _, blocked := releases[variant.ReleaseID]; !blocked {
			continue
		}
		if _, exists := seen[variant.LocalAgentID]; exists {
			continue
		}
		seen[variant.LocalAgentID] = struct{}{}
		ids = append(ids, variant.LocalAgentID)
	}
	sort.Strings(ids)
	return ids, nil
}

func (s *AgentSecurityService) revocationScope(ctx context.Context, tenantID uint64, blocked map[string]string, blockedBy string) (interfaces.AgentRevocationScope, error) {
	scope := interfaces.AgentRevocationScope{BlockedReleases: make([]interfaces.AgentBlockedRelease, 0, len(blocked)), AffectedAdoptionIDs: []string{}, AffectedVariants: []interfaces.AgentBlockedVariant{}}
	for releaseID, listingID := range blocked {
		scope.BlockedReleases = append(scope.BlockedReleases, interfaces.AgentBlockedRelease{ReleaseID: releaseID, ListingID: listingID, BlockedBy: blockedBy})
	}
	sort.Slice(scope.BlockedReleases, func(i, j int) bool { return scope.BlockedReleases[i].ReleaseID < scope.BlockedReleases[j].ReleaseID })
	variants, err := s.store.ListVariants(ctx, tenantID)
	if err != nil {
		return interfaces.AgentRevocationScope{}, err
	}
	adoptions := make(map[string]struct{})
	for _, variant := range variants {
		if _, match := blocked[variant.ReleaseID]; !match {
			continue
		}
		scope.AffectedVariants = append(scope.AffectedVariants, interfaces.AgentBlockedVariant{VariantID: variant.ID, AdoptionID: variant.AdoptionID,
			ReleaseID: variant.ReleaseID, State: variant.State, LocalAgentID: variant.LocalAgentID})
		if _, exists := adoptions[variant.AdoptionID]; !exists {
			adoptions[variant.AdoptionID] = struct{}{}
			scope.AffectedAdoptionIDs = append(scope.AffectedAdoptionIDs, variant.AdoptionID)
		}
	}
	sort.Strings(scope.AffectedAdoptionIDs)
	return scope, nil
}

type AgentSecurityService struct {
	store    *repository.AgentSecurityStore
	runs     *repository.AgentRunStore
	versions interfaces.AgentVersionService
	now      func() time.Time
}

func (s *AgentSecurityService) SetAgentVersionService(versions interfaces.AgentVersionService) {
	s.versions = versions
}

// ResolvePublishedAgentVersion returns the immutable snapshot pinned by the
// currently published Variant. The client never supplies a Version ID.
func (s *AgentSecurityService) ResolvePublishedAgentVersion(ctx context.Context, sourceTenantID uint64, localAgentID string) (interfaces.AgentVersionSnapshot, string, bool, error) {
	if sourceTenantID == 0 || strings.TrimSpace(localAgentID) == "" {
		return interfaces.AgentVersionSnapshot{}, "", false, ErrAgentSecurityInvalidInput
	}
	versionID, releaseID, adopted, err := s.store.ResolvePublishedVariant(ctx, sourceTenantID, localAgentID)
	if err != nil {
		return interfaces.AgentVersionSnapshot{}, "", false, err
	}
	if !adopted {
		return interfaces.AgentVersionSnapshot{}, "", false, nil
	}
	if s.versions == nil {
		return interfaces.AgentVersionSnapshot{}, "", false, errors.New("agent version service is unavailable")
	}
	snapshot, err := s.versions.GetAgentVersion(ctx, sourceTenantID, versionID)
	if err != nil {
		return interfaces.AgentVersionSnapshot{}, "", false, err
	}
	if snapshot.AgentVersionView.ID != versionID || snapshot.AgentVersionView.AgentID != localAgentID || snapshot.Agent == nil || snapshot.Agent.ID != localAgentID || snapshot.Agent.TenantID != sourceTenantID {
		return interfaces.AgentVersionSnapshot{}, "", false, repository.ErrAgentSecurityReleaseUnresolvable
	}
	return snapshot, releaseID, true, nil
}

func NewAgentSecurityService(store *repository.AgentSecurityStore, runs *repository.AgentRunStore) *AgentSecurityService {
	return &AgentSecurityService{store: store, runs: runs, now: func() time.Time { return time.Now().UTC() }}
}

func (s *AgentSecurityService) RevokeRelease(ctx context.Context, tenantID uint64, actorID string, input interfaces.ReleaseRevocationInput) (interfaces.AgentSecurityRevocationView, error) {
	input.ReleaseID, input.Reason = strings.TrimSpace(input.ReleaseID), strings.TrimSpace(input.Reason)
	input.ReplacementReleaseID = strings.TrimSpace(input.ReplacementReleaseID)
	input.InFlightDisposition = normalizeInFlightDisposition(input.InFlightDisposition)
	if tenantID == 0 || strings.TrimSpace(actorID) == "" || input.ReleaseID == "" || input.Reason == "" || input.InFlightDisposition == "invalid" {
		return interfaces.AgentSecurityRevocationView{}, ErrAgentSecurityInvalidInput
	}
	listingID, _, found, err := s.store.ReleaseFacts(ctx, tenantID, input.ReleaseID)
	if err != nil {
		return interfaces.AgentSecurityRevocationView{}, err
	}
	if !found {
		return interfaces.AgentSecurityRevocationView{}, ErrAgentSecurityReleaseUnresolvable
	}
	if input.ReplacementReleaseID != "" {
		replacementListing, _, replacementFound, err := s.store.ReleaseFacts(ctx, tenantID, input.ReplacementReleaseID)
		if err != nil {
			return interfaces.AgentSecurityRevocationView{}, err
		}
		if !replacementFound || replacementListing != listingID {
			return interfaces.AgentSecurityRevocationView{}, ErrAgentSecurityInvalidInput
		}
	}

	now := s.now().UTC()
	row := &types.AgentReleaseRevocationEntity{
		TenantID: tenantID, ListingID: listingID, ReleaseID: input.ReleaseID, Reason: input.Reason,
		ReplacementReleaseID: input.ReplacementReleaseID, InFlightDisposition: input.InFlightDisposition,
		RevokedBy: strings.TrimSpace(actorID), RevokedAt: now, CreatedAt: now,
	}
	audit, err := revocationAudit(types.AuditActionAgentReleaseRevoked, tenantID, strings.TrimSpace(actorID), "agent_release", input.ReleaseID,
		map[string]string{"reason": input.Reason, "replacement": input.ReplacementReleaseID, "in_flight_disposition": input.InFlightDisposition}, now)
	if err != nil {
		return interfaces.AgentSecurityRevocationView{}, err
	}
	appendErr := error(nil)
	if input.InFlightDisposition == interfaces.AgentSecurityInFlightCancel {
		appendErr = s.store.AppendReleaseRevocationWithAuditAndCancelClaims(ctx, row, audit)
	} else {
		appendErr = s.store.AppendReleaseRevocationWithAudit(ctx, row, audit)
	}
	if appendErr != nil {
		return interfaces.AgentSecurityRevocationView{}, appendErr
	}
	if input.InFlightDisposition == interfaces.AgentSecurityInFlightCancel {
		count, err := s.runs.ReconcileRunCancellation(ctx, tenantID, row.ID)
		if err != nil {
			return interfaces.AgentSecurityRevocationView{}, err
		}
		row.CanceledRunCount = count
	}
	return releaseRevocationView(row), nil
}

func (s *AgentSecurityService) RevokeDependency(ctx context.Context, tenantID uint64, actorID string, input interfaces.DependencyRevocationInput) (interfaces.AgentSecurityRevocationView, error) {
	input.Dependency.Type, input.Dependency.ID = strings.TrimSpace(input.Dependency.Type), strings.TrimSpace(input.Dependency.ID)
	input.Dependency.Version, input.Dependency.Digest = strings.TrimSpace(input.Dependency.Version), strings.TrimSpace(input.Dependency.Digest)
	input.Reason, input.ReplacementVersion = strings.TrimSpace(input.Reason), strings.TrimSpace(input.ReplacementVersion)
	input.InFlightDisposition = normalizeInFlightDisposition(input.InFlightDisposition)
	if tenantID == 0 || strings.TrimSpace(actorID) == "" || input.Reason == "" || input.InFlightDisposition == "invalid" ||
		input.Dependency.Type == "" || input.Dependency.ID == "" || input.Dependency.Version == "" || input.Dependency.Digest == "" {
		return interfaces.AgentSecurityRevocationView{}, ErrAgentSecurityInvalidInput
	}
	now := s.now().UTC()
	row := &types.AgentDependencyRevocationEntity{
		TenantID: tenantID, DepType: input.Dependency.Type, DepID: input.Dependency.ID,
		DepVersion: input.Dependency.Version, DepDigest: input.Dependency.Digest, Reason: input.Reason,
		ReplacementVersion: input.ReplacementVersion, InFlightDisposition: input.InFlightDisposition,
		RevokedBy: strings.TrimSpace(actorID), RevokedAt: now, CreatedAt: now,
	}
	targetID := input.Dependency.Type + "/" + input.Dependency.ID + "@" + input.Dependency.Version
	audit, err := revocationAudit(types.AuditActionAgentDependencyRevoked, tenantID, strings.TrimSpace(actorID), "agent_dependency", targetID,
		map[string]string{"reason": input.Reason, "replacement": input.ReplacementVersion, "in_flight_disposition": input.InFlightDisposition}, now)
	if err != nil {
		return interfaces.AgentSecurityRevocationView{}, err
	}
	appendErr := error(nil)
	if input.InFlightDisposition == interfaces.AgentSecurityInFlightCancel {
		appendErr = s.store.AppendDependencyRevocationWithAuditAndCancelClaims(ctx, row, audit)
	} else {
		appendErr = s.store.AppendDependencyRevocationWithAudit(ctx, row, audit)
	}
	if appendErr != nil {
		return interfaces.AgentSecurityRevocationView{}, appendErr
	}
	if input.InFlightDisposition == interfaces.AgentSecurityInFlightCancel {
		count, err := s.runs.ReconcileRunCancellation(ctx, tenantID, row.ID)
		if err != nil {
			return interfaces.AgentSecurityRevocationView{}, err
		}
		row.CanceledRunCount = count
	}
	return dependencyRevocationView(row), nil
}

func (s *AgentSecurityService) ListRevocations(ctx context.Context, tenantID uint64) ([]interfaces.AgentSecurityRevocationView, error) {
	if tenantID == 0 {
		return nil, ErrAgentSecurityInvalidInput
	}
	releases, err := s.store.ListReleaseRevocations(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	dependencies, err := s.store.ListDependencyRevocations(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	views := make([]interfaces.AgentSecurityRevocationView, 0, len(releases)+len(dependencies))
	for i := range releases {
		views = append(views, releaseRevocationView(&releases[i]))
	}
	for i := range dependencies {
		views = append(views, dependencyRevocationView(&dependencies[i]))
	}
	sort.SliceStable(views, func(i, j int) bool {
		if views[i].RevokedAt.Equal(views[j].RevokedAt) {
			return views[i].ID < views[j].ID
		}
		return views[i].RevokedAt.Before(views[j].RevokedAt)
	})
	return views, nil
}

func (s *AgentSecurityService) GetRevocation(ctx context.Context, tenantID uint64, id string) (interfaces.AgentSecurityRevocationView, error) {
	id = strings.TrimSpace(id)
	if tenantID == 0 || id == "" {
		return interfaces.AgentSecurityRevocationView{}, ErrAgentSecurityInvalidInput
	}
	if row, err := s.store.GetReleaseRevocation(ctx, tenantID, id); err != nil {
		return interfaces.AgentSecurityRevocationView{}, err
	} else if row != nil {
		view := releaseRevocationView(row)
		blocked := map[string]string{row.ReleaseID: row.ListingID}
		scope, err := s.revocationScope(ctx, tenantID, blocked, interfaces.AgentSecurityRevocationKindRelease)
		if err != nil {
			return interfaces.AgentSecurityRevocationView{}, err
		}
		view.Scope = &scope
		return view, nil
	}
	row, err := s.store.GetDependencyRevocation(ctx, tenantID, id)
	if err != nil {
		return interfaces.AgentSecurityRevocationView{}, err
	}
	if row == nil {
		return interfaces.AgentSecurityRevocationView{}, ErrAgentSecurityNotFound
	}
	view := dependencyRevocationView(row)
	blocked, err := s.blockedReleasesForDependency(ctx, tenantID, types.AgentReleaseDependency{Type: row.DepType, ID: row.DepID, Version: row.DepVersion, Digest: row.DepDigest})
	if err != nil {
		return interfaces.AgentSecurityRevocationView{}, err
	}
	scope, err := s.revocationScope(ctx, tenantID, blocked, interfaces.AgentSecurityRevocationKindDependency)
	if err != nil {
		return interfaces.AgentSecurityRevocationView{}, err
	}
	view.Scope = &scope
	return view, nil
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
	lock, err := decodeDependencyLock(lockJSON)
	if err != nil {
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

func decodeDependencyLock(lockJSON string) (types.DependencyLock, error) {
	trimmed := bytes.TrimSpace([]byte(lockJSON))
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return types.DependencyLock{}, errors.New("dependency lock must be a JSON object")
	}
	var envelope struct {
		Dependencies *[]types.AgentReleaseDependency `json:"dependencies"`
	}
	if err := json.Unmarshal(trimmed, &envelope); err != nil {
		return types.DependencyLock{}, err
	}
	if envelope.Dependencies == nil {
		return types.DependencyLock{}, errors.New("dependency lock must contain a non-null dependencies array")
	}
	for i, dependency := range *envelope.Dependencies {
		if strings.TrimSpace(dependency.Type) == "" || strings.TrimSpace(dependency.ID) == "" ||
			strings.TrimSpace(dependency.Version) == "" || strings.TrimSpace(dependency.Digest) == "" {
			return types.DependencyLock{}, fmt.Errorf("dependency lock entry %d is missing locked identity fields", i)
		}
	}
	return types.DependencyLock{Dependencies: *envelope.Dependencies}, nil
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
