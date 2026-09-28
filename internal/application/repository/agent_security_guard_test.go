package repository

import (
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestCheckLocalAgentReleaseAdmissionTx(t *testing.T) {
	tests := []struct {
		name      string
		version   string
		variant   bool
		revoked   bool
		deps      string
		depRevoke bool
		duplicate bool
		wantID    string
		wantAdopt bool
		wantErr   bool
	}{
		{name: "active published exact version", version: "version-1", variant: true, deps: `{"dependencies":[]}`, wantID: "release-1", wantAdopt: true},
		{name: "confirmed non marketplace", version: "", wantErr: false},
		{name: "missing version", version: "missing", variant: true, deps: `{"dependencies":[]}`, wantErr: true},
		{name: "mismatched version", version: "version-2", variant: true, deps: `{"dependencies":[]}`, wantErr: true},
		{name: "retired variant", version: "version-1", variant: true, deps: `{"dependencies":[]}`, wantErr: true, wantID: "retired"},
		{name: "draft variant", version: "version-1", variant: true, deps: `{"dependencies":[]}`, wantErr: true, wantID: "draft"},
		{name: "revoked release", version: "version-1", variant: true, revoked: true, deps: `{"dependencies":[]}`, wantErr: true},
		{name: "exact dependency tuple revoked", version: "version-1", variant: true, deps: `{"dependencies":[{"type":"skill","id":"dep","version":"1","digest":"sha-a"}]}`, depRevoke: true, wantErr: true},
		{name: "different dependency version allowed", version: "version-1", variant: true, deps: `{"dependencies":[{"type":"skill","id":"dep","version":"2","digest":"sha-a"}]}`, depRevoke: true, wantID: "release-1", wantAdopt: true},
		{name: "same version different digest allowed", version: "version-1", variant: true, deps: `{"dependencies":[{"type":"skill","id":"dep","version":"1","digest":"sha-b"}]}`, depRevoke: true, wantID: "release-1", wantAdopt: true},
		{name: "duplicate eligible mappings fail closed", version: "version-1", variant: true, duplicate: true, deps: `{"dependencies":[]}`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := openRunTestDB(t)
			localAgent := "local-agent"
			if !tt.variant {
				require.NoError(t, db.Create(&types.CustomAgent{ID: localAgent, TenantID: 1, Name: "Ordinary"}).Error)
			} else {
				seedAdmissionRelease(t, db, tt.deps)
				seedAdmissionVariant(t, db, "variant-1", localAgent, "version-1", "release-1", tt.deps, "published")
				if tt.duplicate {
					require.NoError(t, db.Create(&types.AgentAdoptionVariantEntity{ID: "variant-2", TenantID: 1, AdoptionID: "adoption-1", ReleaseID: "release-1", Name: "duplicate", State: "published", LocalAgentID: localAgent, LocalAgentVersionID: "version-1"}).Error)
				}
				if tt.wantID == "retired" || tt.wantID == "draft" {
					state := tt.wantID
					result := db.Table("agent_adoption_variants").Where("id = ?", "variant-1").Update("state", state)
					require.NoError(t, result.Error)
				}
				if tt.revoked {
					now := time.Now()
					require.NoError(t, db.Create(&types.AgentReleaseRevocationEntity{ID: "rev-release", TenantID: 1, ReleaseID: "release-1", Reason: "blocked", RevokedBy: "admin", RevokedAt: now, CreatedAt: now}).Error)
				}
				if tt.depRevoke {
					now := time.Now()
					require.NoError(t, db.Create(&types.AgentDependencyRevocationEntity{ID: "rev-dep", TenantID: 1, DepType: "skill", DepID: "dep", DepVersion: "1", DepDigest: "sha-a", Reason: "blocked", RevokedBy: "admin", RevokedAt: now, CreatedAt: now}).Error)
				}
			}
			gotID, adopted, err := checkLocalAgentReleaseAdmissionTx(db, 1, localAgent, tt.version)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantID, gotID)
			require.Equal(t, tt.wantAdopt, adopted)
		})
	}
}

func seedAdmissionRelease(t *testing.T, db *gorm.DB, lock string) {
	t.Helper()
	require.NoError(t, db.Create(&types.AgentVersionEntity{ID: "release-source-version", TenantID: 1, AgentID: "source-agent", VersionNumber: 1, Snapshot: "{}", SourceSHA256: "sha"}).Error)
	require.NoError(t, db.Create(&types.AgentMarketplaceListingEntity{ID: "listing-1", TenantID: 1, SourceAgentID: "source-agent", DisplayName: "Source", State: "listed"}).Error)
	require.NoError(t, db.Create(&types.AgentReleaseSubmissionEntity{ID: "submission-1", TenantID: 1, ListingID: "listing-1", AgentVersionID: "release-source-version", SourceAgentID: "source-agent", SemanticVersion: "1.0.0", BundleDigest: "bundle-digest", ManifestJSON: "{}", DependencyLockJSON: lock, Bundle: []byte("bundle"), Status: "released"}).Error)
	require.NoError(t, db.Create(&types.AgentReleaseEntity{
		ID: "release-1", TenantID: 1, ListingID: "listing-1", SubmissionID: "submission-1", AgentVersionID: "release-source-version", SourceAgentID: "source-agent",
		ReleaseNumber: 1, SemanticVersion: "1.0.0", BundleDigest: "bundle-digest", ManifestJSON: "{}", DependencyLockJSON: lock, Bundle: []byte("bundle"),
	}).Error)
	require.NoError(t, db.Create(&types.AgentAdoptionEntity{ID: "adoption-1", TenantID: 1, ListingID: "listing-1", AcceptedReleaseID: "release-1", State: "active"}).Error)
}

func seedAdmissionVariant(t *testing.T, db *gorm.DB, id, agentID, versionID, releaseID, lock, state string) {
	t.Helper()
	require.NoError(t, db.Create(&types.AgentVersionEntity{ID: versionID, TenantID: 1, AgentID: agentID, VersionNumber: 1, Snapshot: "{}", SourceSHA256: "sha"}).Error)
	require.NoError(t, db.Create(&types.AgentAdoptionVariantEntity{
		ID: id, TenantID: 1, AdoptionID: "adoption-1", ReleaseID: releaseID, Name: "display name is irrelevant", State: state,
		LocalAgentID: agentID, LocalAgentVersionID: versionID,
	}).Error)
}
