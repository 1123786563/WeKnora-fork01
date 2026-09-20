package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/agent/experts"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

type marketplaceVersionsFake struct{ snapshot types.AgentVersionSnapshot }

func (f marketplaceVersionsFake) FreezeAgentVersion(context.Context, uint64, string, string) (types.AgentVersionView, error) {
	return f.snapshot.AgentVersionView, nil
}
func (f marketplaceVersionsFake) GetAgentVersion(context.Context, uint64, string) (types.AgentVersionSnapshot, error) {
	return f.snapshot, nil
}
func (f marketplaceVersionsFake) ListAgentVersions(context.Context, uint64, string) ([]types.AgentVersionView, error) {
	return []types.AgentVersionView{f.snapshot.AgentVersionView}, nil
}

type marketplaceResolverFake struct{ lock types.DependencyLock }

func (f marketplaceResolverFake) Resolve(context.Context, uint64, types.AgentVersionSnapshot) (types.DependencyLock, error) {
	return f.lock, nil
}

type marketplaceRepoFake struct {
	queue      []types.AgentReleaseSubmissionEntity
	listing    types.AgentMarketplaceListingEntity
	submission *types.AgentReleaseSubmissionEntity
	review     *types.AgentReleaseReviewEntity
	release    *types.AgentReleaseEntity
	reviewErr  error
}

func (f *marketplaceRepoFake) CreateSubmission(_ context.Context, listing *types.AgentMarketplaceListingEntity, submission *types.AgentReleaseSubmissionEntity) (*types.AgentReleaseSubmissionEntity, error) {
	f.listing = *listing
	copy := *submission
	copy.ID = "submission-1"
	f.submission = &copy
	return &copy, nil
}
func (f *marketplaceRepoFake) ListReviewQueue(context.Context, uint64) ([]types.AgentReleaseSubmissionEntity, error) {
	return f.queue, nil
}
func (f *marketplaceRepoFake) GetSubmission(_ context.Context, _ uint64, id string) (*types.AgentReleaseSubmissionEntity, error) {
	if f.submission != nil && f.submission.ID == id {
		return f.submission, nil
	}
	for i := range f.queue {
		if f.queue[i].ID == id {
			row := f.queue[i]
			f.submission = &row
			return &row, nil
		}
	}
	return nil, nil
}
func (f *marketplaceRepoFake) ReviewAndPublishTx(_ context.Context, _ uint64, _, _, digest string, decision types.AgentReleaseReviewDecision) (*types.AgentReleaseReviewEntity, *types.AgentReleaseEntity, error) {
	if f.reviewErr != nil {
		return nil, nil, f.reviewErr
	}
	if f.review != nil {
		return f.review, f.release, nil
	}
	f.review = &types.AgentReleaseReviewEntity{ID: "review-1", ReviewerID: decision.ReviewerID, ReviewedDigest: digest, Decision: decision.Decision, Reason: decision.Reason}
	if decision.Decision == "approved" {
		f.release = &types.AgentReleaseEntity{ID: "release-1", BundleDigest: digest, Bundle: append([]byte(nil), f.submission.Bundle...)}
	}
	return f.review, f.release, nil
}
func (f *marketplaceRepoFake) ListTenantCatalog(context.Context, uint64) ([]types.AgentMarketplaceListingEntity, error) {
	return []types.AgentMarketplaceListingEntity{f.listing}, nil
}
func (f *marketplaceRepoFake) GetRelease(context.Context, uint64, string) (*types.AgentReleaseEntity, error) {
	return f.release, nil
}

func marketplaceTestSnapshot() types.AgentVersionSnapshot {
	return types.AgentVersionSnapshot{AgentVersionView: types.AgentVersionView{ID: "version-1", AgentID: "agent-1", VersionNumber: 1, SourceSHA256: "source-digest"}, Agent: &types.CustomAgent{ID: "agent-1", Config: types.CustomAgentConfig{SystemPrompt: "help", SelectedSkills: []string{"skill-a"}, Subagents: []string{"agent-a"}}}}
}
func marketplaceTestLock() types.DependencyLock {
	return types.DependencyLock{Dependencies: []types.AgentReleaseDependency{
		{Type: "skill", ID: "skill-a", Version: "1.0.0", Digest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", LicenseID: "MIT"},
		{Type: "subagent", ID: "agent-a", Version: "2.0.0", Digest: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", LicenseID: "Apache-2.0"},
	}}
}
func marketplaceMetadata() types.ReleaseMetadata {
	return types.ReleaseMetadata{SemanticVersion: "1.0.0", DisplayName: "Helper", Summary: "help", SupportedLanguages: []string{"en"}, UseCases: []string{"support"}, MinimumWeKnoraCapability: "1", LicenseID: "MIT"}
}

func TestAgentMarketplaceSubmitBindsFrozenSnapshotAndLocksEveryPayloadReference(t *testing.T) {
	repo := &marketplaceRepoFake{}
	svc := NewAgentMarketplaceService(marketplaceVersionsFake{marketplaceTestSnapshot()}, marketplaceResolverFake{marketplaceTestLock()}, repo, t.TempDir())
	got, err := svc.SubmitRelease(context.Background(), 7, "author", "version-1", interfaces.SubmitReleaseInput{Metadata: marketplaceMetadata()})
	require.NoError(t, err)
	require.Equal(t, "version-1", got.AgentVersionID)
	require.NotEmpty(t, got.BundleDigest)
	require.Equal(t, "author", repo.submission.AuthorID)
	require.Contains(t, repo.submission.ManifestJSON, `"license_id":"MIT"`)
	require.Contains(t, repo.submission.DependencyLockJSON, `"id":"skill-a"`)
	require.Contains(t, repo.submission.DependencyLockJSON, `"id":"agent-a"`)
	for _, ref := range append(append([]string{}, marketplaceTestSnapshot().Agent.Config.SelectedSkills...), marketplaceTestSnapshot().Agent.Config.Subagents...) {
		require.Contains(t, string(repo.submission.Bundle), ref)
	}
}

func TestAgentMarketplaceSubmitRejectsAnyUnlockedPayloadReference(t *testing.T) {
	lock := marketplaceTestLock()
	lock.Dependencies = lock.Dependencies[:1]
	svc := NewAgentMarketplaceService(marketplaceVersionsFake{marketplaceTestSnapshot()}, marketplaceResolverFake{lock}, &marketplaceRepoFake{}, t.TempDir())
	_, err := svc.SubmitRelease(context.Background(), 7, "author", "version-1", interfaces.SubmitReleaseInput{Metadata: marketplaceMetadata()})
	require.Error(t, err)
	require.Contains(t, err.Error(), "agent-a")
}

func TestAgentMarketplaceReviewRejectRequiresReasonAndStaleDigestDoesNotPublish(t *testing.T) {
	bundle, err := buildServiceTestBundle()
	require.NoError(t, err)
	repo := &marketplaceRepoFake{queue: []types.AgentReleaseSubmissionEntity{{ID: "submission-1", BundleDigest: bundle.SHA256, Bundle: bundle.Bytes}}}
	svc := NewAgentMarketplaceService(nil, nil, repo, t.TempDir())
	_, err = svc.ReviewSubmission(context.Background(), 7, "reviewer", "submission-1", bundle.SHA256, types.AgentReleaseReviewDecision{Decision: "rejected"})
	require.Error(t, err)
	_, err = svc.ReviewSubmission(context.Background(), 7, "reviewer", "submission-1", strings.Repeat("0", 64), types.AgentReleaseReviewDecision{Decision: "approved"})
	require.ErrorIs(t, err, ErrAgentMarketplaceStaleDigest)
	require.Nil(t, repo.review)
}

func TestAgentMarketplaceApprovalStagesVerifiedReleaseBytesAndRetryIsIdempotent(t *testing.T) {
	bundle, err := buildServiceTestBundle()
	require.NoError(t, err)
	repo := &marketplaceRepoFake{queue: []types.AgentReleaseSubmissionEntity{{ID: "submission-1", BundleDigest: bundle.SHA256, Bundle: bundle.Bytes}}}
	svc := NewAgentMarketplaceService(nil, nil, repo, t.TempDir())
	first, err := svc.ReviewSubmission(context.Background(), 7, "reviewer", "submission-1", bundle.SHA256, types.AgentReleaseReviewDecision{Decision: "approved"})
	require.NoError(t, err)
	require.NotNil(t, first.Release)
	path := filepath.Join(svc.bundleRoot, "tenant-7", "releases", "submission-1", bundle.SHA256)
	stored, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, bundle.Bytes, stored)
	second, err := svc.ReviewSubmission(context.Background(), 7, "reviewer", "submission-1", bundle.SHA256, types.AgentReleaseReviewDecision{Decision: "approved"})
	require.NoError(t, err)
	require.Equal(t, first.Review.ID, second.Review.ID)
	require.Equal(t, stored, second.Release.Bundle)
}

func TestAgentMarketplaceApprovalCompensatesFinalObjectWhenTransactionFails(t *testing.T) {
	bundle, err := buildServiceTestBundle()
	require.NoError(t, err)
	repo := &marketplaceRepoFake{queue: []types.AgentReleaseSubmissionEntity{{ID: "submission-1", BundleDigest: bundle.SHA256, Bundle: bundle.Bytes}}, reviewErr: errors.New("db unavailable")}
	svc := NewAgentMarketplaceService(nil, nil, repo, t.TempDir())
	_, err = svc.ReviewSubmission(context.Background(), 7, "reviewer", "submission-1", bundle.SHA256, types.AgentReleaseReviewDecision{Decision: "approved"})
	require.Error(t, err)
	_, err = os.Stat(filepath.Join(svc.bundleRoot, "tenant-7", "releases", "submission-1", bundle.SHA256))
	require.True(t, os.IsNotExist(err))
}

func buildServiceTestBundle() (types.AgentReleaseBundle, error) {
	return experts.BuildAgentReleaseBundle(marketplaceTestSnapshot(), marketplaceMetadata(), marketplaceTestLock())
}
