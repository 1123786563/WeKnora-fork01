package repository

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openMobileDeviceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	require.NoError(t, err)
	up, err := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "sqlite", "000058_mobile_devices.up.sql"))
	require.NoError(t, err)
	require.NoError(t, db.Exec(string(up)).Error)
	return db
}

func mobileRegistration(tenant uint64, owner, device, token string) DeviceRegistration {
	return DeviceRegistration{TenantID: tenant, OwnerID: owner, DeviceID: device,
		Environment: "dev", Platform: "ios", TokenCiphertext: "enc:" + token,
		TokenHash: DeviceTokenHash(token), ScopeGeneration: 1}
}

func TestMobileDeviceRevocationIsOwnerScoped(t *testing.T) {
	s := NewMobileDeviceStore(openMobileDeviceTestDB(t), "dev")
	ctx := context.Background()
	require.NoError(t, s.Bind(ctx, mobileRegistration(1, "u1", "d", "token-a")))
	require.ErrorIs(t, s.RevokeForTenant(ctx, 1, "other", "d", 1), ErrMobileDeviceNotFound)
	require.NoError(t, s.RevokeForTenant(ctx, 1, "u1", "d", 1))
	rows, err := s.ListActiveForTenant(ctx, 1, "u1", "dev")
	require.NoError(t, err)
	require.Empty(t, rows)
}

func TestMobileDeviceTenantAndEnvironmentIsolation(t *testing.T) {
	s := NewMobileDeviceStore(openMobileDeviceTestDB(t), "dev")
	ctx := context.Background()
	require.NoError(t, s.Bind(ctx, mobileRegistration(1, "same-user", "d1", "token-a")))
	require.NoError(t, s.Bind(ctx, mobileRegistration(2, "same-user", "d2", "token-b")))
	one, err := s.ListActiveForTenant(ctx, 1, "same-user", "dev")
	require.NoError(t, err)
	require.Len(t, one, 1)
	two, err := s.ListActiveForTenant(ctx, 2, "same-user", "dev")
	require.NoError(t, err)
	require.Len(t, two, 1)
	_, listErr := s.ListActive(ctx, "same-user", "prod")
	require.ErrorIs(t, listErr, ErrMobileDeviceInvalid)
}

func TestMobileDeviceTokenTakeoverRevokesOldOwner(t *testing.T) {
	s := NewMobileDeviceStore(openMobileDeviceTestDB(t), "dev")
	ctx := context.Background()
	require.NoError(t, s.Bind(ctx, mobileRegistration(1, "u1", "d1", "same-token")))
	require.NoError(t, s.Bind(ctx, mobileRegistration(2, "u2", "d2", "same-token")))
	old, err := s.ListActiveForTenant(ctx, 1, "u1", "dev")
	require.NoError(t, err)
	require.Empty(t, old)
	current, err := s.ListActiveForTenant(ctx, 2, "u2", "dev")
	require.NoError(t, err)
	require.Len(t, current, 1)
}

func TestMobileDeviceStaleRevocationCannotResurrect(t *testing.T) {
	s := NewMobileDeviceStore(openMobileDeviceTestDB(t), "dev")
	ctx := context.Background()
	in := mobileRegistration(1, "u1", "d", "token-a")
	require.NoError(t, s.Bind(ctx, in))
	require.NoError(t, s.RevokeForTenant(ctx, 1, "u1", "d", 1))
	// A delayed registration carrying the old revision cannot reopen the row.
	in.TokenHash = DeviceTokenHash("token-b")
	in.TokenCiphertext = "enc:token-b"
	in.Revision = 1
	require.ErrorIs(t, s.Bind(ctx, in), ErrMobileDeviceRevision)
	in.Revision = 0
	in.ScopeGeneration = 3
	require.NoError(t, s.Bind(ctx, DeviceRegistration{TenantID: 1, OwnerID: "u1", DeviceID: "d", Environment: "dev", Platform: "ios", TokenCiphertext: "enc:token-b", TokenHash: DeviceTokenHash("token-b"), ScopeGeneration: 3}))
}

func TestMobileDeviceScopeGenerationRevokesOlderBindings(t *testing.T) {
	s := NewMobileDeviceStore(openMobileDeviceTestDB(t), "dev")
	ctx := context.Background()
	require.NoError(t, s.Bind(ctx, mobileRegistration(1, "u1", "d1", "token-a")))
	require.NoError(t, s.Bind(ctx, DeviceRegistration{TenantID: 1, OwnerID: "u1", DeviceID: "d2", Environment: "dev", Platform: "android", TokenCiphertext: "enc:token-b", TokenHash: DeviceTokenHash("token-b"), ScopeGeneration: 3}))
	require.NoError(t, s.RevokeBeforeScopeGeneration(ctx, 1, "u1", 3))
	// A delayed refresh from the retired epoch cannot resurrect the row, even
	// when it guesses a future revision.
	late := mobileRegistration(1, "u1", "d1", "token-c")
	late.Revision, late.ScopeGeneration = 99, 1
	require.ErrorIs(t, s.Bind(ctx, late), ErrMobileDeviceRevision)
	rows, err := s.ListActiveForTenant(ctx, 1, "u1", "dev")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "d2", rows[0].DeviceID)
}

func TestMobileDeviceRebindAfterLogoutAllocatesServerRevision(t *testing.T) {
	s := NewMobileDeviceStore(openMobileDeviceTestDB(t), "dev")
	ctx := context.Background()
	require.NoError(t, s.Bind(ctx, mobileRegistration(1, "u1", "d", "token-a")))
	require.NoError(t, s.RevokeForTenant(ctx, 1, "u1", "d", 1))
	// Logout advances the durable scope epoch to 2. A new login receives the
	// next signed transition (3) before a revision-less registration rebinds.
	rebind := mobileRegistration(1, "u1", "d", "token-b")
	rebind.ScopeGeneration = 3
	require.NoError(t, s.Bind(ctx, rebind))
	row, err := s.GetActiveForTenant(ctx, 1, "u1", "d")
	require.NoError(t, err)
	require.EqualValues(t, 3, row.Revision)
}

func TestMobileDeviceRevisionlessReplayAfterLogoutCannotReopen(t *testing.T) {
	s := NewMobileDeviceStore(openMobileDeviceTestDB(t), "dev")
	ctx := context.Background()
	first := mobileRegistration(1, "u1", "d", "token-a")
	require.NoError(t, s.Bind(ctx, first))
	require.NoError(t, s.RevokeForTenant(ctx, 1, "u1", "d", 1))

	// This is the original request arriving after logout: no CAS revision and
	// the pre-logout client generation. It must remain a conflict forever.
	late := first
	late.Revision = 0
	require.ErrorIs(t, s.Bind(ctx, late), ErrMobileDeviceRevision)
	active, err := s.ListActiveForTenant(ctx, 1, "u1", "dev")
	require.NoError(t, err)
	require.Empty(t, active)

	newLogin := first
	newLogin.TokenCiphertext = "enc:token-b"
	newLogin.TokenHash = DeviceTokenHash("token-b")
	newLogin.ScopeGeneration = 3
	require.NoError(t, s.Bind(ctx, newLogin))
	active, err = s.ListActiveForTenant(ctx, 1, "u1", "dev")
	require.NoError(t, err)
	require.Len(t, active, 1)
}

func TestMobileDeviceRegistrationIntentIssuedBeforeConcurrentLogoutCannotReopen(t *testing.T) {
	s := NewMobileDeviceStore(openMobileDeviceTestDB(t), "dev")
	ctx := context.Background()
	first := mobileRegistration(1, "u1", "d", "token-a")
	require.NoError(t, s.Bind(ctx, first))
	// The handler may already have read and signed epoch 2 when logout commits
	// the same epoch.  Equality is therefore stale, even without a revision.
	require.NoError(t, s.RevokeForTenant(ctx, 1, "u1", "d", 1))
	late := first
	late.Revision = 0
	late.ScopeGeneration = 2
	late.TokenCiphertext = "enc:token-b"
	late.TokenHash = DeviceTokenHash("token-b")
	require.ErrorIs(t, s.Bind(ctx, late), ErrMobileDeviceRevision)
	active, err := s.ListActiveForTenant(ctx, 1, "u1", "dev")
	require.NoError(t, err)
	require.Empty(t, active)
	// A new login's strictly newer server epoch remains valid.
	late.ScopeGeneration = 3
	require.NoError(t, s.Bind(ctx, late))
}

func TestMobileDeviceConcurrentBindAndRevokePreserveLifecycleFence(t *testing.T) {
	for _, first := range []string{"bind", "revoke"} {
		t.Run(first+"-first", func(t *testing.T) {
			db := openMobileDeviceTestDB(t)
			s := NewMobileDeviceStore(db, "dev")
			ctx := context.Background()
			initial := mobileRegistration(1, "u1", "d", "token-a")
			require.NoError(t, s.Bind(ctx, initial))

			bind := initial
			bind.TokenCiphertext, bind.TokenHash, bind.ScopeGeneration = "enc:token-b", DeviceTokenHash("token-b"), 2
			start := make(chan struct{})
			results := make(chan error, 2)
			runBind := func() { <-start; results <- s.Bind(context.Background(), bind) }
			runRevoke := func() { <-start; results <- s.RevokeForTenant(context.Background(), 1, "u1", "d", 1) }
			if first == "bind" {
				go runBind()
				go runRevoke()
			} else {
				go runRevoke()
				go runBind()
			}
			close(start)
			for range 2 {
				err := <-results
				if err != nil {
					require.True(t, errors.Is(err, ErrMobileDeviceRevision) || stringsContains(err.Error(), "locked") || stringsContains(err.Error(), "busy"), err)
				}
			}

			var row mobileDeviceRow
			require.NoError(t, db.Where("tenant_id = ? AND owner_id = ? AND device_id = ? AND environment = ?", 1, "u1", "d", "dev").Take(&row).Error)
			// Whichever transaction wins, the row remains fenced at epoch 2:
			// Bind-first leaves a newer active revision, while revoke-first leaves
			// the same revision revoked. A stale registration cannot resurrect it.
			require.EqualValues(t, 2, row.Revision)
			require.EqualValues(t, 2, row.ScopeGeneration)
		})
	}
}

func TestMobileDeviceRejectsFutureRevisionAndLowerEpoch(t *testing.T) {
	s := NewMobileDeviceStore(openMobileDeviceTestDB(t), "dev")
	ctx := context.Background()
	require.NoError(t, s.Bind(ctx, mobileRegistration(1, "u1", "d", "token-a")))
	future := mobileRegistration(1, "u1", "d", "token-b")
	future.Revision, future.ScopeGeneration = 99, 1
	require.ErrorIs(t, s.Bind(ctx, future), ErrMobileDeviceRevision)
	lower := mobileRegistration(1, "u1", "d", "token-c")
	lower.Revision, lower.ScopeGeneration = 0, 0
	require.ErrorIs(t, s.Bind(ctx, lower), ErrMobileDeviceRevision)
}

func TestMobileDeviceMigrationRoundTrip(t *testing.T) {
	db := openMobileDeviceTestDB(t)
	down, err := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "sqlite", "000058_mobile_devices.down.sql"))
	require.NoError(t, err)
	require.NoError(t, db.Exec(string(down)).Error)
	up, err := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "sqlite", "000058_mobile_devices.up.sql"))
	require.NoError(t, err)
	require.NoError(t, db.Exec(string(up)).Error)
	var count int64
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='mobile_devices'").Scan(&count).Error)
	require.EqualValues(t, 1, count)
}

func TestMobileDevicePresenceIsTenantOwnerRevisionScoped(t *testing.T) {
	s := NewMobileDeviceStore(openMobileDeviceTestDB(t), "dev")
	ctx := context.Background()
	require.NoError(t, s.Bind(ctx, mobileRegistration(1, "u1", "d", "token-a")))
	row, err := s.SetPresence(ctx, 1, "u1", "d", 1)
	require.NoError(t, err)
	require.NotNil(t, row.LastSeenAt)
	_, err = s.GetPresence(ctx, 2, "u1", "d", 1)
	require.ErrorIs(t, err, ErrMobileDeviceNotFound)
	require.ErrorIs(t, s.DeletePresence(ctx, 1, "u1", "d", 99), ErrMobileDeviceRevision)
	require.NoError(t, s.DeletePresence(ctx, 1, "u1", "d", 1))
	row, err = s.GetPresence(ctx, 1, "u1", "d", 1)
	require.NoError(t, err)
	require.Nil(t, row.LastSeenAt)
}

func TestMobileDeviceConcurrentTokenTakeoverLeavesOneActiveBinding(t *testing.T) {
	db := openMobileDeviceTestDB(t)
	s := NewMobileDeviceStore(db, "dev")
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := uint64(1); i <= 2; i++ {
		wg.Add(1)
		go func(tenant uint64) {
			defer wg.Done()
			err := s.Bind(context.Background(), mobileRegistration(tenant, "u", "d", "same"))
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)
	var successful int
	for err := range errs {
		if err == nil {
			successful++
		} else {
			// SQLite can reject the losing writer with a transient lock; it
			// must never produce two active rows.
			require.True(t, errors.Is(err, gorm.ErrInvalidData) || stringsContains(err.Error(), "locked") || stringsContains(err.Error(), "constraint"), err)
		}
	}
	require.GreaterOrEqual(t, successful, 1)
	var active int64
	require.NoError(t, db.Model(&mobileDeviceRow{}).Where("environment = ? AND token_hash = ? AND revoked_at IS NULL", "dev", DeviceTokenHash("same")).Count(&active).Error)
	require.EqualValues(t, 1, active)
}

func stringsContains(value, needle string) bool {
	return len(value) >= len(needle) && (value == needle || strings.Contains(value, needle))
}
