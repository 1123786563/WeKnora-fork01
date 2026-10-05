package service

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

func TestValidateWorkerConcurrencyMinimums(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		value   any
		wantErr bool
	}{
		{name: "core zero", key: "asynq.core_concurrency", value: 0, wantErr: true},
		{name: "core minimum", key: "asynq.core_concurrency", value: 1},
		{name: "postprocess minimum", key: "asynq.postprocess_concurrency", value: 1},
		{name: "enrichment minimum", key: "asynq.enrichment_concurrency", value: 1},
		{name: "maintenance minimum", key: "asynq.maintenance_concurrency", value: 1},
		{name: "shared minimum", key: "asynq.shared_concurrency", value: 1},
		{name: "wiki zero", key: "asynq.wiki_concurrency", value: 0, wantErr: true},
		{name: "wiki minimum", key: "asynq.wiki_concurrency", value: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRegistryEntry(tt.key, tt.value)
			if tt.wantErr && err == nil {
				t.Fatal("expected validation error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected validation error: %v", err)
			}
		})
	}
}

type registrationSettingRepo struct {
	interfaces.SystemSettingRepository
	row *types.SystemSetting
}

func (r *registrationSettingRepo) Get(context.Context, string) (*types.SystemSetting, error) {
	return r.row, nil
}

func (r *registrationSettingRepo) Upsert(_ context.Context, row *types.SystemSetting) error {
	r.row = row
	return nil
}

func TestValidateRegistrationModes(t *testing.T) {
	svc := &systemSettingService{repo: &registrationSettingRepo{}, cache: make(map[string]*types.SystemSetting)}
	ctx := context.WithValue(context.Background(), types.UserIDContextKey, "system-admin")
	for _, mode := range []string{"self_serve", "invite_register", "invite_only"} {
		row, err := svc.Update(ctx, "auth.registration_mode", mode)
		if err != nil {
			t.Fatalf("mode %q rejected: %v", mode, err)
		}
		if len(row.Enum) != 3 || svc.GetString(ctx, "auth.registration_mode", "", "") != mode {
			t.Fatalf("saved mode %q must be effective and expose all three choices", mode)
		}
	}
	if _, err := svc.Update(ctx, "auth.registration_mode", "unknown"); err == nil {
		t.Fatal("unknown registration mode accepted")
	}
}

type listMaskingRepo struct {
	interfaces.SystemSettingRepository
	rows []*types.SystemSetting
}

func (r *listMaskingRepo) List(context.Context) ([]*types.SystemSetting, error) {
	return r.rows, nil
}

// TestListMasksUnregisteredSecretRows guards the leak path for the
// bootstrap-provisioned signing key (cmd/server/embed_signing_key.go):
// out-of-band rows stay visible for diagnostics, but values of rows flagged
// IsSecret must never render via the management API.
func TestListMasksUnregisteredSecretRows(t *testing.T) {
	repo := &listMaskingRepo{rows: []*types.SystemSetting{
		{Key: "security.embed_signing_key", Value: types.JSON(`"deadbeef-secret"`), ValueType: "string", IsSecret: true},
		{Key: "ops.diagnostics_row", Value: types.JSON(`"visible"`), ValueType: "string"},
	}}
	svc := &systemSettingService{repo: repo, cache: make(map[string]*types.SystemSetting)}

	out, err := svc.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	byKey := map[string]*types.SystemSetting{}
	for _, row := range out {
		byKey[row.Key] = row
	}
	secret, ok := byKey["security.embed_signing_key"]
	if !ok {
		t.Fatal("secret row must stay listed (masked, not hidden)")
	}
	if string(secret.Value) != `"***"` {
		t.Fatalf("secret value must be masked, got %s", string(secret.Value))
	}
	plain, ok := byKey["ops.diagnostics_row"]
	if !ok || string(plain.Value) != `"visible"` {
		t.Fatalf("non-secret out-of-band row must stay visible, got %#v", plain)
	}
}
