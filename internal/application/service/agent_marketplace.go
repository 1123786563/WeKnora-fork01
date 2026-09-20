package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Tencent/WeKnora/internal/agent/experts"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

var (
	ErrAgentMarketplaceInvalidInput      = errors.New("invalid agent marketplace input")
	ErrAgentMarketplaceStaleDigest       = errors.New("agent marketplace submission digest is stale")
	ErrAgentMarketplaceMissingDependency = errors.New("agent marketplace payload dependency is not locked")
)

type AgentMarketplaceService struct {
	versions   interfaces.AgentVersionService
	resolver   interfaces.ReleaseDependencyResolver
	repo       interfaces.AgentMarketplaceRepository
	bundleRoot string
	// publishNoReplace makes the final path visible atomically only if it does
	// not already exist. Keeping this seam small lets tests control a real race.
	publishNoReplace func(tempPath, finalPath string) error
}

var _ interfaces.AgentMarketplaceService = (*AgentMarketplaceService)(nil)

func NewAgentMarketplaceService(versions interfaces.AgentVersionService, resolver interfaces.ReleaseDependencyResolver, repo interfaces.AgentMarketplaceRepository, bundleRoot string) *AgentMarketplaceService {
	return &AgentMarketplaceService{versions: versions, resolver: resolver, repo: repo, bundleRoot: bundleRoot, publishNoReplace: os.Link}
}

func (s *AgentMarketplaceService) SubmitRelease(ctx context.Context, tenantID uint64, actorID, versionID string, input interfaces.SubmitReleaseInput) (interfaces.ReleaseSubmissionView, error) {
	actorID, versionID = strings.TrimSpace(actorID), strings.TrimSpace(versionID)
	if tenantID == 0 || actorID == "" || versionID == "" {
		return interfaces.ReleaseSubmissionView{}, ErrAgentMarketplaceInvalidInput
	}
	version, err := s.versions.GetAgentVersion(ctx, tenantID, versionID)
	if err != nil {
		return interfaces.ReleaseSubmissionView{}, fmt.Errorf("load frozen agent version: %w", err)
	}
	if version.ID != versionID || version.Agent == nil || version.AgentID == "" || version.Agent.ID != version.AgentID {
		return interfaces.ReleaseSubmissionView{}, fmt.Errorf("%w: invalid frozen agent version snapshot", ErrAgentMarketplaceInvalidInput)
	}
	lock, err := s.resolver.Resolve(ctx, tenantID, version)
	if err != nil {
		return interfaces.ReleaseSubmissionView{}, fmt.Errorf("resolve release dependencies: %w", err)
	}
	bundle, err := experts.BuildAgentReleaseBundle(version, input.Metadata, lock)
	if err != nil {
		return interfaces.ReleaseSubmissionView{}, err
	}
	if err := ensurePayloadReferencesLocked(bundle.Payload, bundle.Lock); err != nil {
		return interfaces.ReleaseSubmissionView{}, err
	}
	manifestJSON, err := json.Marshal(bundle.Manifest)
	if err != nil {
		return interfaces.ReleaseSubmissionView{}, fmt.Errorf("encode release manifest: %w", err)
	}
	lockJSON, err := json.Marshal(bundle.Lock)
	if err != nil {
		return interfaces.ReleaseSubmissionView{}, fmt.Errorf("encode dependency lock: %w", err)
	}
	listing := &types.AgentMarketplaceListingEntity{TenantID: tenantID, SourceAgentID: version.AgentID, DisplayName: bundle.Manifest.DisplayName, Summary: bundle.Manifest.Summary, State: "listed"}
	submission := &types.AgentReleaseSubmissionEntity{TenantID: tenantID, AgentVersionID: version.ID, SourceAgentID: version.AgentID, AuthorID: actorID, SemanticVersion: bundle.Manifest.SemanticVersion, BundleDigest: bundle.SHA256, ManifestJSON: string(manifestJSON), DependencyLockJSON: string(lockJSON), Bundle: append([]byte(nil), bundle.Bytes...), Status: "submitted"}
	created, err := s.repo.CreateSubmission(ctx, listing, submission)
	if err != nil {
		return interfaces.ReleaseSubmissionView{}, fmt.Errorf("persist release submission: %w", err)
	}
	return interfaces.ReleaseSubmissionView{AgentReleaseSubmissionEntity: *created}, nil
}

func ensurePayloadReferencesLocked(payload types.AgentReleasePayload, lock types.DependencyLock) error {
	locked := make(map[string]bool, len(lock.Dependencies))
	for _, dep := range lock.Dependencies {
		locked[dep.Type+"\x00"+dep.ID] = true
	}
	for _, id := range payload.Skills {
		if !locked["skill\x00"+id] {
			return fmt.Errorf("%w: skill %q", ErrAgentMarketplaceMissingDependency, id)
		}
	}
	for _, id := range payload.Subagents {
		if !locked["subagent\x00"+id] {
			return fmt.Errorf("%w: subagent %q", ErrAgentMarketplaceMissingDependency, id)
		}
	}
	return nil
}

func (s *AgentMarketplaceService) ListReviewQueue(ctx context.Context, tenantID uint64) ([]interfaces.ReleaseSubmissionView, error) {
	rows, err := s.repo.ListReviewQueue(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	views := make([]interfaces.ReleaseSubmissionView, 0, len(rows))
	for _, row := range rows {
		views = append(views, interfaces.ReleaseSubmissionView{AgentReleaseSubmissionEntity: row})
	}
	return views, nil
}

func (s *AgentMarketplaceService) ReviewSubmission(ctx context.Context, tenantID uint64, actorID, submissionID, expectedDigest string, decision types.AgentReleaseReviewDecision) (interfaces.ReleaseReviewResult, error) {
	actorID, submissionID, expectedDigest = strings.TrimSpace(actorID), strings.TrimSpace(submissionID), strings.ToLower(strings.TrimSpace(expectedDigest))
	decision.Decision, decision.Reason = strings.TrimSpace(decision.Decision), strings.TrimSpace(decision.Reason)
	if tenantID == 0 || actorID == "" || submissionID == "" || !validMarketplaceDigest(expectedDigest) {
		return interfaces.ReleaseReviewResult{}, ErrAgentMarketplaceInvalidInput
	}
	if decision.Decision != "approved" && decision.Decision != "rejected" && decision.Decision != "changes_requested" {
		return interfaces.ReleaseReviewResult{}, fmt.Errorf("%w: invalid review decision", ErrAgentMarketplaceInvalidInput)
	}
	if decision.Decision != "approved" && decision.Reason == "" {
		return interfaces.ReleaseReviewResult{}, fmt.Errorf("%w: a reason is required", ErrAgentMarketplaceInvalidInput)
	}
	decision.ReviewerID = actorID
	submission, err := s.repo.GetSubmission(ctx, tenantID, submissionID)
	if err != nil {
		return interfaces.ReleaseReviewResult{}, err
	}
	if submission == nil {
		return interfaces.ReleaseReviewResult{}, fmt.Errorf("submission not found")
	}
	if submission.BundleDigest != expectedDigest {
		return interfaces.ReleaseReviewResult{}, ErrAgentMarketplaceStaleDigest
	}
	sum := sha256.Sum256(submission.Bundle)
	if hex.EncodeToString(sum[:]) != submission.BundleDigest {
		return interfaces.ReleaseReviewResult{}, fmt.Errorf("submission bundle does not match its recorded digest")
	}
	priorReleaseID := ""
	if decision.Decision == "approved" {
		listing, err := s.repo.GetListing(ctx, tenantID, submission.ListingID)
		if err != nil {
			return interfaces.ReleaseReviewResult{}, err
		}
		if listing == nil {
			return interfaces.ReleaseReviewResult{}, fmt.Errorf("submission listing not found")
		}
		if listing.CurrentReleaseID != nil {
			priorReleaseID = *listing.CurrentReleaseID
		}
	}
	var stagedPath string
	created := false
	if decision.Decision == "approved" {
		unlock, err := s.lockReleaseBundle(ctx, tenantID, submissionID, expectedDigest)
		if err != nil {
			return interfaces.ReleaseReviewResult{}, err
		}
		defer unlock()
		stagedPath, created, err = s.stageReleaseBundle(ctx, tenantID, submissionID, expectedDigest, submission.Bundle)
		if err != nil {
			return interfaces.ReleaseReviewResult{}, err
		}
	}
	review, release, err := s.repo.ReviewAndPublishTx(ctx, tenantID, priorReleaseID, submissionID, expectedDigest, decision)
	if err != nil {
		if created {
			persisted, lookupErr := s.repo.GetReleaseBySubmission(ctx, tenantID, submissionID)
			if lookupErr == nil && (persisted == nil || persisted.BundleDigest != expectedDigest) {
				_ = os.Remove(stagedPath)
			}
		}
		return interfaces.ReleaseReviewResult{}, err
	}
	return interfaces.ReleaseReviewResult{Review: review, Release: release}, nil
}

// lockReleaseBundle serializes the filesystem stage/transaction/compensation
// window for one immutable digest across service processes sharing the
// bundle filesystem. Database CAS remains the publication correctness
// boundary; this lock only prevents one failed attempt from removing bytes
// another in-flight approval is about to commit.
func (s *AgentMarketplaceService) lockReleaseBundle(ctx context.Context, tenantID uint64, submissionID, digest string) (func(), error) {
	if strings.TrimSpace(s.bundleRoot) == "" {
		return nil, fmt.Errorf("release bundle storage root is required")
	}
	dir := filepath.Join(s.bundleRoot, fmt.Sprintf("tenant-%d", tenantID), "releases", filepath.Base(submissionID))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create release bundle directory: %w", err)
	}
	lockFile, err := os.OpenFile(filepath.Join(dir, "."+digest+".lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open release bundle lock: %w", err)
	}
	for {
		err = syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return func() {
				_ = syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)
				_ = lockFile.Close()
			}, nil
		}
		if err != syscall.EWOULDBLOCK && err != syscall.EAGAIN {
			_ = lockFile.Close()
			return nil, fmt.Errorf("lock release bundle path: %w", err)
		}
		select {
		case <-ctx.Done():
			_ = lockFile.Close()
			return nil, ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func validMarketplaceDigest(digest string) bool {
	if len(digest) != 64 {
		return false
	}
	_, err := hex.DecodeString(digest)
	return err == nil
}

func (s *AgentMarketplaceService) stageReleaseBundle(ctx context.Context, tenantID uint64, submissionID, digest string, bundle []byte) (string, bool, error) {
	if err := ctx.Err(); err != nil {
		return "", false, err
	}
	if strings.TrimSpace(s.bundleRoot) == "" {
		return "", false, fmt.Errorf("release bundle storage root is required")
	}
	sum := sha256.Sum256(bundle)
	if hex.EncodeToString(sum[:]) != digest {
		return "", false, fmt.Errorf("staged bundle digest verification failed")
	}
	dir := filepath.Join(s.bundleRoot, fmt.Sprintf("tenant-%d", tenantID), "releases", filepath.Base(submissionID))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", false, fmt.Errorf("create release bundle directory: %w", err)
	}
	final := filepath.Join(dir, digest)
	if existing, err := os.ReadFile(final); err == nil {
		if string(existing) != string(bundle) {
			return "", false, fmt.Errorf("existing release bundle digest path contains different bytes")
		}
		return final, false, nil
	} else if !os.IsNotExist(err) {
		return "", false, err
	}
	f, err := os.CreateTemp(dir, ".release-*.tmp")
	if err != nil {
		return "", false, err
	}
	temp := f.Name()
	defer os.Remove(temp)
	if _, err = f.Write(bundle); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return "", false, fmt.Errorf("materialize release bundle: %w", err)
	}
	if closeErr != nil {
		return "", false, closeErr
	}
	staged, err := os.ReadFile(temp)
	if err != nil {
		return "", false, err
	}
	stagedSum := sha256.Sum256(staged)
	if hex.EncodeToString(stagedSum[:]) != digest {
		return "", false, fmt.Errorf("staged bundle digest verification failed")
	}
	publish := s.publishNoReplace
	if publish == nil {
		publish = os.Link
	}
	if err := publish(temp, final); err != nil {
		if !errors.Is(err, os.ErrExist) {
			return "", false, fmt.Errorf("commit staged release bundle: %w", err)
		}
		existing, readErr := os.ReadFile(final)
		if readErr != nil {
			return "", false, fmt.Errorf("read existing release bundle after exclusive publish conflict: %w", readErr)
		}
		existingSum := sha256.Sum256(existing)
		if hex.EncodeToString(existingSum[:]) != digest {
			return "", false, fmt.Errorf("existing release bundle digest path contains different bytes")
		}
		return final, false, nil
	}
	return final, true, nil
}

func (s *AgentMarketplaceService) ListTenantCatalog(ctx context.Context, tenantID uint64) ([]interfaces.TenantListingView, error) {
	rows, err := s.repo.ListTenantCatalog(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	views := make([]interfaces.TenantListingView, 0, len(rows))
	for _, row := range rows {
		views = append(views, interfaces.TenantListingView{AgentMarketplaceListingEntity: row})
	}
	return views, nil
}

func (s *AgentMarketplaceService) GetRelease(ctx context.Context, tenantID uint64, releaseID string) (*types.AgentReleaseEntity, error) {
	return s.repo.GetRelease(ctx, tenantID, releaseID)
}
