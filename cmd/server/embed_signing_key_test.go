package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/Tencent/WeKnora/internal/utils"
)

// fakeSigningKeyRepo is an in-memory SystemSettingRepository for the
// provisioner tests. upsertCount tracks writes; onUpsert, when set, runs
// after each Upsert so tests can simulate a racing replica overwriting the
// row between our write and our read-back.
type fakeSigningKeyRepo struct {
	interfaces.SystemSettingRepository
	rows       map[string]*types.SystemSetting
	getErr     error
	upsertErr  error
	upserts    int
	onUpsert   func(rows map[string]*types.SystemSetting)
}

func (r *fakeSigningKeyRepo) Get(_ context.Context, key string) (*types.SystemSetting, error) {
	if r.getErr != nil {
		return nil, r.getErr
	}
	return r.rows[key], nil
}

func (r *fakeSigningKeyRepo) Upsert(_ context.Context, s *types.SystemSetting) error {
	if r.upsertErr != nil {
		return r.upsertErr
	}
	if r.rows == nil {
		r.rows = map[string]*types.SystemSetting{}
	}
	cp := *s
	r.rows[s.Key] = &cp
	r.upserts++
	if r.onUpsert != nil {
		r.onUpsert(r.rows)
	}
	return nil
}

func unconfiguredSigningEnv(t *testing.T) {
	t.Helper()
	t.Setenv("SYSTEM_SIGNING_KEY", "")
	t.Setenv("SYSTEM_AES_KEY", "")
}

func TestEnsureEmbedSigningKeyAutoProvisionsAndIsStable(t *testing.T) {
	unconfiguredSigningEnv(t)
	repo := &fakeSigningKeyRepo{}
	ctx := context.Background()

	ensureEmbedSigningKey(ctx, repo)

	// (a) Auto-provisioned: a usable key is exported and persisted.
	key := utils.SystemHMACKey()
	if len(key) < embedSigningKeyMinLen {
		t.Fatalf("no usable key after provisioning (len=%d)", len(key))
	}
	row := repo.rows[embedSigningKeySetting]
	if row == nil || !row.IsSecret {
		t.Fatalf("expected persisted IsSecret row, got %#v", row)
	}
	if stored, err := row.AsString(); err != nil || stored != string(key) {
		t.Fatalf("stored row %q must match exported key (err=%v)", stored, err)
	}
	// Presigned URLs and embed session handles work with the provisioned key.
	if u, err := utils.SignFileURL("https://weknora.example.com", "local://1/abc/img.png", 7, 0); err != nil || !strings.Contains(u, "sig=") {
		t.Fatalf("SignFileURL after provisioning: url=%q err=%v", u, err)
	}
	ch := &types.EmbedChannel{ID: "ch1", PublishToken: "tok"}
	sig := service.SignEmbedSessionHandle(ch, "sess1")
	if sig == "" || !service.VerifyEmbedSessionHandle(ch, "sess1", sig) {
		t.Fatal("embed session handle must sign+verify with provisioned key")
	}

	// (c) Second run must not regenerate: restore the unconfigured env a
	// fresh replica would have, re-run, and expect the same key.
	t.Setenv("SYSTEM_SIGNING_KEY", "")
	t.Setenv("SYSTEM_AES_KEY", "")
	ensureEmbedSigningKey(ctx, repo)
	if repo.upserts != 1 {
		t.Fatalf("second run must not rewrite the row (upserts=%d)", repo.upserts)
	}
	if string(utils.SystemHMACKey()) != string(key) {
		t.Fatal("key rotated on second run — must be stable")
	}
}

func TestEnsureEmbedSigningKeyExplicitConfigWins(t *testing.T) {
	t.Run("explicit signing key", func(t *testing.T) {
		t.Setenv("SYSTEM_SIGNING_KEY", "explicit-signing-key-32-bytes-ok!!!")
		t.Setenv("SYSTEM_AES_KEY", "")
		repo := &fakeSigningKeyRepo{}
		ensureEmbedSigningKey(context.Background(), repo)
		if repo.upserts != 0 || len(repo.rows) != 0 {
			t.Fatalf("explicit config must leave storage untouched (upserts=%d)", repo.upserts)
		}
		if string(utils.SystemHMACKey()) != "explicit-signing-key-32-bytes-ok!!!" {
			t.Fatal("explicit key must keep winning")
		}
	})
	t.Run("usable AES fallback", func(t *testing.T) {
		t.Setenv("SYSTEM_SIGNING_KEY", "")
		t.Setenv("SYSTEM_AES_KEY", "legacy-aes-key-32-bytes-long!!!!!")
		repo := &fakeSigningKeyRepo{}
		ensureEmbedSigningKey(context.Background(), repo)
		if repo.upserts != 0 {
			t.Fatal("usable AES fallback must suppress provisioning")
		}
	})
}

func TestEnsureEmbedSigningKeyRestoresExistingRow(t *testing.T) {
	unconfiguredSigningEnv(t)
	stored := "preexisting-embed-key-0123456789abcdef"
	repo := &fakeSigningKeyRepo{rows: map[string]*types.SystemSetting{
		embedSigningKeySetting: {
			Key:       embedSigningKeySetting,
			Value:     types.JSON(`"` + stored + `"`),
			ValueType: "string",
			IsSecret:  true,
		},
	}}
	ensureEmbedSigningKey(context.Background(), repo)
	if repo.upserts != 0 {
		t.Fatal("existing row must be restored, not rewritten")
	}
	if got := string(utils.SystemHMACKey()); got != stored {
		t.Fatalf("restored key = %q, want stored %q", got, stored)
	}
}

func TestEnsureEmbedSigningKeyFailuresStayLoud(t *testing.T) {
	t.Run("repo read error", func(t *testing.T) {
		unconfiguredSigningEnv(t)
		repo := &fakeSigningKeyRepo{getErr: errors.New("db down")}
		ensureEmbedSigningKey(context.Background(), repo)
		if utils.SystemHMACKey() != nil {
			t.Fatal("repo failure must not invent a key")
		}
	})
	t.Run("upsert error", func(t *testing.T) {
		unconfiguredSigningEnv(t)
		repo := &fakeSigningKeyRepo{upsertErr: errors.New("db down")}
		ensureEmbedSigningKey(context.Background(), repo)
		if utils.SystemHMACKey() != nil {
			t.Fatal("persist failure must not use an unpersisted key")
		}
	})
	t.Run("corrupt row", func(t *testing.T) {
		unconfiguredSigningEnv(t)
		repo := &fakeSigningKeyRepo{rows: map[string]*types.SystemSetting{
			embedSigningKeySetting: {Key: embedSigningKeySetting, Value: types.JSON(`"short"`), ValueType: "string", IsSecret: true},
		}}
		ensureEmbedSigningKey(context.Background(), repo)
		if utils.SystemHMACKey() != nil {
			t.Fatal("corrupt row must keep the loud unconfigured path, not rotate")
		}
		if repo.upserts != 0 {
			t.Fatal("corrupt row must not be silently rewritten")
		}
	})
}

func TestEnsureEmbedSigningKeyAdoptsRacedRowValue(t *testing.T) {
	unconfiguredSigningEnv(t)
	winner := "racing-replica-key-0123456789abcdef"
	repo := &fakeSigningKeyRepo{}
	// Another replica wins the row between our Upsert and our read-back:
	// the provisioned env must converge on the stored (winner) value.
	repo.onUpsert = func(rows map[string]*types.SystemSetting) {
		rows[embedSigningKeySetting].Value = types.JSON(`"` + winner + `"`)
	}
	ensureEmbedSigningKey(context.Background(), repo)
	if got := string(utils.SystemHMACKey()); got != winner {
		t.Fatalf("raced key = %q, want winner value %q", got, winner)
	}
}
