package workbench

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type versionGrantAuthorizer struct {
	revoked bool
	calls   int
}

func (a *versionGrantAuthorizer) AuthorizeVersionGrant(_ context.Context, _ VersionArtifactGrant) error {
	a.calls++
	if a.revoked {
		return errors.New("revoked")
	}
	return nil
}

func TestVersionArtifactGrantBindsOwnerResourceVersionAndDigest(t *testing.T) {
	key := []byte(strings.Repeat("k", 32))
	now := time.Now()
	base := VersionArtifactGrant{TenantID: 7, OwnerID: "u1", ResourceID: "resume", VersionID: "v2", Digest: strings.Repeat("a", 64), ExpiresAt: now.Add(time.Minute).Unix()}
	sig, err := SignVersionArtifactGrant(key, base)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyVersionArtifactGrantAt(key, base, sig, now); err != nil {
		t.Fatal(err)
	}
	mutations := []VersionArtifactGrant{
		func() VersionArtifactGrant { g := base; g.TenantID++; return g }(),
		func() VersionArtifactGrant { g := base; g.OwnerID = "u2"; return g }(),
		func() VersionArtifactGrant { g := base; g.ResourceID = "other"; return g }(),
		func() VersionArtifactGrant { g := base; g.VersionID = "v3"; return g }(),
		func() VersionArtifactGrant { g := base; g.Digest = strings.Repeat("b", 64); return g }(),
	}
	for _, changed := range mutations {
		if err := VerifyVersionArtifactGrantAt(key, changed, sig, now); err == nil {
			t.Fatalf("mutated grant accepted: %+v", changed)
		}
	}
}

func TestVersionArtifactGrantRejectsExpiredAndAmbiguousFields(t *testing.T) {
	key := []byte(strings.Repeat("k", 32))
	grant := VersionArtifactGrant{TenantID: 1, OwnerID: "u1", ResourceID: "r", VersionID: "v1", Digest: "abc", ExpiresAt: 100}
	if err := VerifyVersionArtifactGrantAt(key, grant, "", time.Unix(100, 0)); err == nil {
		t.Fatal("expired grant accepted")
	}
	grant.OwnerID = "u1|u2"
	if _, err := SignVersionArtifactGrant(key, grant); err == nil {
		t.Fatal("ambiguous owner accepted")
	}
	grant.OwnerID = "u1"
	grant.Digest = "not-a-sha256"
	if _, err := SignVersionArtifactGrant(key, grant); err == nil {
		t.Fatal("non-SHA256 digest accepted")
	}
}

func TestNewVersionArtifactGrantCapsExpiry(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	grant, err := NewVersionArtifactGrant(1, "u1", "r1", "v1", strings.Repeat("a", 64), now, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if grant.ExpiresAt != now.Add(MaxArtifactGrantTTL).Unix() {
		t.Fatalf("expiry = %d, want capped at %d", grant.ExpiresAt, now.Add(MaxArtifactGrantTTL).Unix())
	}
}

func TestVersionArtifactGrantAuthorityRechecksAuthorizationAtDownload(t *testing.T) {
	key := []byte(strings.Repeat("k", 32))
	check := &versionGrantAuthorizer{}
	authority := VersionArtifactGrantAuthority{Secret: key, Authorizer: check}
	now := time.Now()
	grant, sig, err := authority.Issue(context.Background(), 7, "u1", "resume", "v2", strings.Repeat("a", 64), now, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if err := authority.Authorize(context.Background(), grant, sig, now); err != nil {
		t.Fatal(err)
	}
	check.revoked = true
	if err := authority.Authorize(context.Background(), grant, sig, now); err == nil {
		t.Fatal("revoked version was authorized at download time")
	}
	if check.calls != 3 {
		t.Fatalf("live authorizer calls = %d, want issue + 2 download checks", check.calls)
	}
}
