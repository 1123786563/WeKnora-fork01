package repository

import (
	"context"
	"errors"
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
	require.NoError(t, db.AutoMigrate(&mobileDeviceRow{}))
	// The production migration uses a partial unique index. SQLite supports
	// the same predicate and this makes the test exercise token takeover.
	require.NoError(t, db.Exec("CREATE UNIQUE INDEX uq_mobile_device_token ON mobile_devices(environment, token_hash) WHERE revoked_at IS NULL").Error)
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
	require.NoError(t, s.Bind(ctx, DeviceRegistration{TenantID: 1, OwnerID: "u1", DeviceID: "d", Environment: "dev", Platform: "ios", TokenCiphertext: "enc:token-b", TokenHash: DeviceTokenHash("token-b"), Revision: 3, ScopeGeneration: 2}))
}

func TestMobileDeviceScopeGenerationRevokesOlderBindings(t *testing.T) {
	s := NewMobileDeviceStore(openMobileDeviceTestDB(t), "dev")
	ctx := context.Background()
	require.NoError(t, s.Bind(ctx, mobileRegistration(1, "u1", "d1", "token-a")))
	require.NoError(t, s.Bind(ctx, DeviceRegistration{TenantID: 1, OwnerID: "u1", DeviceID: "d2", Environment: "dev", Platform: "android", TokenCiphertext: "enc:token-b", TokenHash: DeviceTokenHash("token-b"), ScopeGeneration: 3}))
	require.NoError(t, s.RevokeBeforeScopeGeneration(ctx, 1, "u1", 3))
	rows, err := s.ListActiveForTenant(ctx, 1, "u1", "dev")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "d2", rows[0].DeviceID)
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
