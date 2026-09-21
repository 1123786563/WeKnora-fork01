package appconnector

import (
	"context"
	"errors"
	"testing"

	appconnector "github.com/Tencent/WeKnora/internal/appconnector"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func testInstallationStore(t *testing.T) *InstallationStore {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&AppVersion{}, &InstallationRow{}, &ConnectionRow{}); err != nil {
		t.Fatal(err)
	}
	return NewInstallationStore(db)
}

// TestSameAppTwoSpacesIndependent pins the core multi-tenant boundary: the
// same app installed in two tenants is two independent installations, and a
// lifecycle change on one never touches the other.
func TestSameAppTwoSpacesIndependent(t *testing.T) {
	s := testInstallationStore(t)
	ctx := context.Background()
	for _, tenant := range []uint64{7, 8} {
		if err := s.ApplyInstallation(ctx, appconnector.Installation{ID: "inst-" + itoa(tenant), AppID: "app-x", Version: "1.0.0", State: appconnector.InstallationActive, TenantID: tenant}, 0); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.ApplyInstallation(ctx, appconnector.Installation{ID: "inst-7", AppID: "app-x", Version: "1.0.0", State: appconnector.InstallationDisabled, TenantID: 7}, 1); err != nil {
		t.Fatal(err)
	}
	seven, v7, err := s.GetInstallation(ctx, 7, "app-x")
	if err != nil {
		t.Fatal(err)
	}
	if seven.State != appconnector.InstallationDisabled || v7 != 2 {
		t.Fatalf("tenant 7: state=%s version=%d", seven.State, v7)
	}
	eight, v8, err := s.GetInstallation(ctx, 8, "app-x")
	if err != nil {
		t.Fatal(err)
	}
	if eight.State != appconnector.InstallationActive || v8 != 1 || eight.ID != "inst-8" {
		t.Fatalf("tenant 8 affected: %+v v=%d", eight, v8)
	}
}

// TestOneInstallationMultipleConnections covers many connections sharing one
// installation with different kinds and owners.
func TestOneInstallationMultipleConnections(t *testing.T) {
	s := testInstallationStore(t)
	ctx := context.Background()
	if err := s.ApplyInstallation(ctx, appconnector.Installation{ID: "inst-1", AppID: "app-x", Version: "1.0.0", State: appconnector.InstallationActive, TenantID: 7}, 0); err != nil {
		t.Fatal(err)
	}
	conns := []appconnector.Connection{
		{ID: "c1", InstallationID: "inst-1", Kind: appconnector.ConnectionKindPersonal, OwnerID: "u1", CredentialRef: "cred/u1", State: appconnector.ConnectionActive, TenantID: 7, AuthVersion: 1},
		{ID: "c2", InstallationID: "inst-1", Kind: appconnector.ConnectionKindSpace, OwnerID: "u1", CredentialRef: "cred/space", State: appconnector.ConnectionActive, TenantID: 7, AuthVersion: 1},
		{ID: "c3", InstallationID: "inst-1", Kind: appconnector.ConnectionKindPersonal, OwnerID: "u2", CredentialRef: "cred/u2", State: appconnector.ConnectionActive, TenantID: 7, AuthVersion: 1},
	}
	for _, c := range conns {
		if err := s.SaveConnection(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range conns {
		got, err := s.GetConnection(ctx, 7, c.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.CredentialRef != c.CredentialRef || got.AuthVersion != 1 {
			t.Fatalf("connection %s not persisted faithfully: %+v", c.ID, got)
		}
	}
	// Each connection answers authorization independently.
	ok, err := s.ConnectionUsable(ctx, conns[0], "u1", false)
	if err != nil || !ok {
		t.Fatalf("personal owner: ok=%v err=%v", ok, err)
	}
	ok, err = s.ConnectionUsable(ctx, conns[1], "u2", true)
	if err != nil || !ok {
		t.Fatalf("space granted member: ok=%v err=%v", ok, err)
	}
	ok, err = s.ConnectionUsable(ctx, conns[2], "u1", true)
	if err != nil || ok {
		t.Fatalf("foreign personal connection leaked: ok=%v err=%v", ok, err)
	}
}

// TestPersonalConnectionIsolated pins owner-only access through the store.
func TestPersonalConnectionIsolated(t *testing.T) {
	s := testInstallationStore(t)
	ctx := context.Background()
	if err := s.ApplyInstallation(ctx, appconnector.Installation{ID: "inst-1", AppID: "app-x", Version: "1.0.0", State: appconnector.InstallationActive, TenantID: 7}, 0); err != nil {
		t.Fatal(err)
	}
	c := appconnector.Connection{ID: "c1", InstallationID: "inst-1", Kind: appconnector.ConnectionKindPersonal, OwnerID: "u1", CredentialRef: "cred/u1", State: appconnector.ConnectionActive, TenantID: 7, AuthVersion: 1}
	if err := s.SaveConnection(ctx, c); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		actor      string
		spaceGrant bool
		want       bool
	}{
		{"u1", false, true},
		{"u1", true, true},
		{"u2", true, false},
		{"u2", false, false},
	} {
		ok, err := s.ConnectionUsable(ctx, c, tc.actor, tc.spaceGrant)
		if err != nil {
			t.Fatal(err)
		}
		if ok != tc.want {
			t.Fatalf("actor=%s grant=%v: got %v want %v", tc.actor, tc.spaceGrant, ok, tc.want)
		}
	}
}

// TestSpaceConnectionRequiresExplicitGrant pins that a space connection is
// unusable without a grant even for the owner who created it.
func TestSpaceConnectionRequiresExplicitGrant(t *testing.T) {
	s := testInstallationStore(t)
	ctx := context.Background()
	if err := s.ApplyInstallation(ctx, appconnector.Installation{ID: "inst-1", AppID: "app-x", Version: "1.0.0", State: appconnector.InstallationActive, TenantID: 7}, 0); err != nil {
		t.Fatal(err)
	}
	c := appconnector.Connection{ID: "c-space", InstallationID: "inst-1", Kind: appconnector.ConnectionKindSpace, OwnerID: "u1", CredentialRef: "cred/space", State: appconnector.ConnectionActive, TenantID: 7, AuthVersion: 1}
	if err := s.SaveConnection(ctx, c); err != nil {
		t.Fatal(err)
	}
	ok, err := s.ConnectionUsable(ctx, c, "u2", false)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("space connection used without explicit grant")
	}
	ok, err = s.ConnectionUsable(ctx, c, "u2", true)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("explicitly granted member rejected")
	}
}

// TestUpgradeWithExpandedScopeRequiresReauthorization pins that an upgrade
// whose published schema differs from the installed one parks the
// installation in reauthorization_required and can never auto-activate.
func TestUpgradeWithExpandedScopeRequiresReauthorization(t *testing.T) {
	s := testInstallationStore(t)
	ctx := context.Background()
	for _, av := range []AppVersion{
		{AppID: "app-x", Version: "1.0.0", SchemaJSON: `{"scopes":["read"]}`, RiskJSON: `{"level":"low"}`},
		{AppID: "app-x", Version: "2.0.0", SchemaJSON: `{"scopes":["read","write"]}`, RiskJSON: `{"level":"medium"}`},
	} {
		if err := s.db.WithContext(ctx).Create(&av).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := s.ApplyInstallation(ctx, appconnector.Installation{ID: "inst-1", AppID: "app-x", Version: "1.0.0", State: appconnector.InstallationActive, TenantID: 7}, 0); err != nil {
		t.Fatal(err)
	}
	// Trying to keep it active across the scope expansion is refused.
	err := s.ApplyInstallation(ctx, appconnector.Installation{ID: "inst-1", AppID: "app-x", Version: "2.0.0", State: appconnector.InstallationActive, TenantID: 7}, 1)
	if !errors.Is(err, ErrReauthorizationRequired) {
		t.Fatalf("auto-active upgrade: got %v", err)
	}
	// The state-changing attempt also left the installation untouched.
	inst, v, err := s.GetInstallation(ctx, 7, "app-x")
	if err != nil {
		t.Fatal(err)
	}
	if inst.Version != "1.0.0" || inst.State != appconnector.InstallationActive || v != 1 {
		t.Fatalf("refused upgrade leaked state: %+v v=%d", inst, v)
	}
	// The only accepted landing state is reauthorization_required.
	if err := s.ApplyInstallation(ctx, appconnector.Installation{ID: "inst-1", AppID: "app-x", Version: "2.0.0", State: appconnector.InstallationReauthorizationRequired, TenantID: 7}, 1); err != nil {
		t.Fatal(err)
	}
	inst, v, err = s.GetInstallation(ctx, 7, "app-x")
	if err != nil {
		t.Fatal(err)
	}
	if inst.Version != "2.0.0" || inst.State != appconnector.InstallationReauthorizationRequired || v != 2 {
		t.Fatalf("scope upgrade: %+v v=%d", inst, v)
	}
	// While parked, new calls through its connections are rejected.
	c := appconnector.Connection{ID: "c1", InstallationID: "inst-1", Kind: appconnector.ConnectionKindSpace, OwnerID: "u1", CredentialRef: "cred/space", State: appconnector.ConnectionActive, TenantID: 7, AuthVersion: 1}
	if err := s.SaveConnection(ctx, c); err != nil {
		t.Fatal(err)
	}
	ok, err := s.ConnectionUsable(ctx, c, "u2", true)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("connection usable while installation awaits reauthorization")
	}
}

// TestDisabledInstallationRejectsConnectionUse pins that disabling an
// installation rejects new calls through all of its connections, whatever
// the kind, owner, or grant.
func TestDisabledInstallationRejectsConnectionUse(t *testing.T) {
	s := testInstallationStore(t)
	ctx := context.Background()
	if err := s.ApplyInstallation(ctx, appconnector.Installation{ID: "inst-1", AppID: "app-x", Version: "1.0.0", State: appconnector.InstallationActive, TenantID: 7}, 0); err != nil {
		t.Fatal(err)
	}
	personal := appconnector.Connection{ID: "c1", InstallationID: "inst-1", Kind: appconnector.ConnectionKindPersonal, OwnerID: "u1", CredentialRef: "cred/u1", State: appconnector.ConnectionActive, TenantID: 7, AuthVersion: 1}
	space := appconnector.Connection{ID: "c2", InstallationID: "inst-1", Kind: appconnector.ConnectionKindSpace, OwnerID: "u1", CredentialRef: "cred/space", State: appconnector.ConnectionActive, TenantID: 7, AuthVersion: 1}
	for _, c := range []appconnector.Connection{personal, space} {
		if err := s.SaveConnection(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.ApplyInstallation(ctx, appconnector.Installation{ID: "inst-1", AppID: "app-x", Version: "1.0.0", State: appconnector.InstallationDisabled, TenantID: 7}, 1); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		c          appconnector.Connection
		actor      string
		spaceGrant bool
	}{{personal, "u1", false}, {space, "u2", true}} {
		ok, err := s.ConnectionUsable(ctx, tc.c, tc.actor, tc.spaceGrant)
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			t.Fatalf("connection %s usable on disabled installation", tc.c.ID)
		}
	}
}

// TestApplyInstallationCASConflict pins that a stale expected version is
// rejected without touching the row.
func TestApplyInstallationCASConflict(t *testing.T) {
	s := testInstallationStore(t)
	ctx := context.Background()
	if err := s.ApplyInstallation(ctx, appconnector.Installation{ID: "inst-1", AppID: "app-x", Version: "1.0.0", State: appconnector.InstallationActive, TenantID: 7}, 0); err != nil {
		t.Fatal(err)
	}
	// Create with a non-zero expectation on an absent row is also a conflict.
	if err := s.ApplyInstallation(ctx, appconnector.Installation{ID: "inst-9", AppID: "app-y", Version: "1.0.0", State: appconnector.InstallationActive, TenantID: 7}, 1); !errors.Is(err, ErrInstallationConflict) {
		t.Fatalf("create with stale expected: got %v", err)
	}
	if err := s.ApplyInstallation(ctx, appconnector.Installation{ID: "inst-1", AppID: "app-x", Version: "1.0.0", State: appconnector.InstallationDisabled, TenantID: 7}, 1); err != nil {
		t.Fatal(err)
	}
	// Version is now 2; replaying expected=1 must conflict and leave v2 intact.
	err := s.ApplyInstallation(ctx, appconnector.Installation{ID: "inst-1", AppID: "app-x", Version: "1.0.0", State: appconnector.InstallationDisabled, TenantID: 7}, 1)
	if !errors.Is(err, ErrInstallationConflict) {
		t.Fatalf("stale expected: got %v", err)
	}
	_, v, err := s.GetInstallation(ctx, 7, "app-x")
	if err != nil {
		t.Fatal(err)
	}
	if v != 2 {
		t.Fatalf("stale CAS bumped version to %d", v)
	}
}

func itoa(n uint64) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
