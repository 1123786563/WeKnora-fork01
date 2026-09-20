package service

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
)

// dmStrPtr is a local test helper for building *string literals.
func dmStrPtr(s string) *string { return &s }

// TestUpdateUserPreferencesDefaultModelMerge pins the PATCH merge matrix
// for the default_model preference: only a key present in the patch is
// applied, an empty string is the "clear" sentinel (stored as nil), and
// a nil patch field leaves the stored value untouched.
func TestUpdateUserPreferencesDefaultModelMerge(t *testing.T) {
	cases := []struct {
		name     string
		existing *string
		patch    *string
		want     *string
	}{
		{
			name:     "set value when no preference stored",
			existing: nil,
			patch:    dmStrPtr("glm-4.7"),
			want:     dmStrPtr("glm-4.7"),
		},
		{
			name:     "replace existing value",
			existing: dmStrPtr("glm-4.7"),
			patch:    dmStrPtr("deepseek-v3"),
			want:     dmStrPtr("deepseek-v3"),
		},
		{
			name:     "empty string clears the preference",
			existing: dmStrPtr("glm-4.7"),
			patch:    dmStrPtr(""),
			want:     nil,
		},
		{
			name:     "empty string on empty preference stays nil",
			existing: nil,
			patch:    dmStrPtr(""),
			want:     nil,
		},
		{
			name:     "nil patch field preserves existing value",
			existing: dmStrPtr("glm-4.7"),
			patch:    nil,
			want:     dmStrPtr("glm-4.7"),
		},
		{
			name:     "nil patch field preserves absent preference",
			existing: nil,
			patch:    nil,
			want:     nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			repo := &switchTenantUserRepo{users: map[string]types.User{
				"alice": {ID: "alice", Preferences: types.UserPreferences{
					DefaultModel: tc.existing,
				}},
			}}
			svc := &userService{userRepo: repo}

			merged, err := svc.UpdateUserPreferences(ctx, "alice", types.UserPreferences{
				DefaultModel: tc.patch,
			})
			if err != nil {
				t.Fatalf("UpdateUserPreferences: %v", err)
			}

			stored, _ := repo.GetUserByID(ctx, "alice")
			for label, got := range map[string]*string{
				"stored": stored.Preferences.DefaultModel,
				"merged": merged.DefaultModel,
			} {
				switch {
				case tc.want == nil && got != nil:
					t.Fatalf("%s DefaultModel = %q, want nil", label, *got)
				case tc.want != nil && got == nil:
					t.Fatalf("%s DefaultModel = nil, want %q", label, *tc.want)
				case tc.want != nil && *got != *tc.want:
					t.Fatalf("%s DefaultModel = %q, want %q", label, *got, *tc.want)
				}
			}
		})
	}
}

// TestUpdateUserPreferencesDefaultModelLeavesOtherKeysAlone pins the
// isolation contract: applying a default_model patch must not disturb
// other preference keys, and vice versa.
func TestUpdateUserPreferencesDefaultModelLeavesOtherKeysAlone(t *testing.T) {
	ctx := context.Background()
	oidcOnly := true
	tenantID := uint64(42)
	instructions := "prefer official docs"
	repo := &switchTenantUserRepo{users: map[string]types.User{
		"alice": {ID: "alice", Preferences: types.UserPreferences{
			LastActiveTenantID:        &tenantID,
			OidcOnlyLogin:             &oidcOnly,
			BrowserSearchInstructions: &instructions,
		}},
	}}
	svc := &userService{userRepo: repo}

	if _, err := svc.UpdateUserPreferences(ctx, "alice", types.UserPreferences{
		DefaultModel: dmStrPtr("glm-4.7"),
	}); err != nil {
		t.Fatalf("UpdateUserPreferences: %v", err)
	}

	stored, _ := repo.GetUserByID(ctx, "alice")
	prefs := stored.Preferences
	if prefs.LastActiveTenantID == nil || *prefs.LastActiveTenantID != 42 {
		t.Fatalf("LastActiveTenantID = %v, want 42 (must survive default_model patch)", prefs.LastActiveTenantID)
	}
	if prefs.OidcOnlyLogin == nil || !*prefs.OidcOnlyLogin {
		t.Fatal("OidcOnlyLogin was dropped by the default_model patch")
	}
	if prefs.BrowserSearchInstructions == nil || *prefs.BrowserSearchInstructions != instructions {
		t.Fatalf("BrowserSearchInstructions = %v, want %q (must survive default_model patch)",
			prefs.BrowserSearchInstructions, instructions)
	}
	if prefs.DefaultModel == nil || *prefs.DefaultModel != "glm-4.7" {
		t.Fatalf("DefaultModel = %v, want glm-4.7", prefs.DefaultModel)
	}

	// And the reverse direction: patching another key must not touch an
	// existing default_model.
	if _, err := svc.UpdateUserPreferences(ctx, "alice", types.UserPreferences{
		LastActiveTenantID: func() *uint64 { v := uint64(7); return &v }(),
	}); err != nil {
		t.Fatalf("UpdateUserPreferences (tenant patch): %v", err)
	}
	stored, _ = repo.GetUserByID(ctx, "alice")
	if stored.Preferences.DefaultModel == nil || *stored.Preferences.DefaultModel != "glm-4.7" {
		t.Fatalf("DefaultModel = %v after tenant-only patch, want glm-4.7 (must be preserved)",
			stored.Preferences.DefaultModel)
	}
}
