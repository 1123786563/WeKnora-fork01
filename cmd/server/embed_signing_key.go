// Auto-provisioning of the deployment-wide HMAC signing key consumed by
// embed session handles (internal/application/service/embed_session.go) and
// presigned file URLs (internal/utils/presign.go).
//
// Background (issue #3898): fresh docker-compose deployments ship without
// SYSTEM_SIGNING_KEY, so the embed UI failed out of the box with
// "embed session signing key is not configured" and only source-diving
// revealed the fix. Following the desktop precedent (cmd/desktop/signing_key.go
// generates a random key, persists it, and exports it via the environment),
// the server provisions a random key on first boot and persists it in the
// system_settings table so restarts keep it stable and every replica shares
// it (they all read the same row at boot while SYSTEM_SIGNING_KEY is unset).
//
// Explicit configuration always wins: when SYSTEM_SIGNING_KEY is set, or the
// SYSTEM_AES_KEY fallback is usable (see utils.SystemHMACKey), this hook is a
// no-op and the persisted row, if any, stays inert.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"

	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/Tencent/WeKnora/internal/utils"
)

// embedSigningKeySetting is the system_settings row that persists the
// auto-provisioned key. It is intentionally NOT registered in the
// systemSettingService registry: Update/Get/Reset all reject unregistered
// keys, so the management API can neither read nor rotate the secret, and
// List masks out-of-band rows flagged IsSecret (see systemSettingService.List).
const embedSigningKeySetting = "security.embed_signing_key"

// embedSigningKeyBytes is the entropy of a generated key: 32 random bytes,
// hex-encoded to 64 chars — comfortably above the 16-char floor that
// utils.SystemHMACKey enforces.
const embedSigningKeyBytes = 32

// embedSigningKeyMinLen mirrors the shortest key SystemHMACKey accepts, used
// to validate rows restored from the DB (a hand-truncated row must not
// silently disable signing with a misleading "configured" log line).
const embedSigningKeyMinLen = 16

// ensureEmbedSigningKey provisions the embed/presign HMAC key when the
// deployment has none configured. Best-effort, like every bootstrap step:
// on any failure it only warns and leaves the request-time 503 in
// embed_channel.go as the loud signal. Idempotent: a second call with the
// row already present restores the stored value instead of regenerating.
func ensureEmbedSigningKey(ctx context.Context, repo interfaces.SystemSettingRepository) {
	// Explicit config always wins. A non-empty SYSTEM_SIGNING_KEY that
	// SystemHMACKey still rejects (too short / known example value) stays
	// loud: silently substituting a generated key would mask a
	// misconfiguration the operator explicitly attempted.
	if os.Getenv("SYSTEM_SIGNING_KEY") != "" || len(utils.SystemHMACKey()) != 0 {
		return
	}
	if repo == nil {
		return
	}

	// Existing row: restore it. This is what keeps the key stable across
	// restarts and identical across replicas.
	row, err := repo.Get(ctx, embedSigningKeySetting)
	if err != nil {
		logger.Warnf(ctx,
			"[bootstrap] cannot read %s row; embed signing stays unconfigured (set SYSTEM_SIGNING_KEY to override): %v",
			embedSigningKeySetting, err)
		return
	}
	if row != nil {
		key, err := row.AsString()
		if err != nil || len(key) < embedSigningKeyMinLen {
			// Hand-edited or corrupt row: never rotate silently — keep the
			// loud "not configured" path so the operator notices and fixes
			// the row (or sets SYSTEM_SIGNING_KEY) deliberately.
			logger.Warnf(ctx,
				"[bootstrap] system_settings row %s is unreadable or too short; fix or delete the row, or set SYSTEM_SIGNING_KEY",
				embedSigningKeySetting)
			return
		}
		applyEmbedSigningKey(ctx, key, false)
		return
	}

	// First boot: generate, persist, then adopt whatever the row holds.
	raw := make([]byte, embedSigningKeyBytes)
	if _, err := rand.Read(raw); err != nil {
		logger.Warnf(ctx, "[bootstrap] cannot generate embed signing key: %v", err)
		return
	}
	generated := hex.EncodeToString(raw)
	value, err := json.Marshal(generated)
	if err != nil {
		logger.Warnf(ctx, "[bootstrap] cannot encode embed signing key: %v", err)
		return
	}
	if err := repo.Upsert(ctx, &types.SystemSetting{
		Key:            embedSigningKeySetting,
		Value:          value,
		ValueType:      "string",
		Category:       "security",
		Description:    "Auto-provisioned embed session / presigned-URL HMAC key. Set SYSTEM_SIGNING_KEY to override.",
		IsSecret:       true,
		LastModifiedBy: "system",
	}); err != nil {
		logger.Warnf(ctx,
			"[bootstrap] cannot persist embed signing key; embed stays unconfigured until SYSTEM_SIGNING_KEY is set: %v", err)
		return
	}
	// Read back so simultaneous first boots converge on one key: Upsert is
	// last-writer-wins, so the stored value — not the generated one — is
	// authoritative whenever a replica raced us between write and read.
	stored, err := repo.Get(ctx, embedSigningKeySetting)
	if err != nil || stored == nil {
		logger.Warnf(ctx, "[bootstrap] cannot re-read %s row after write: %v", embedSigningKeySetting, err)
		return
	}
	key, err := stored.AsString()
	if err != nil || len(key) < embedSigningKeyMinLen {
		logger.Warnf(ctx, "[bootstrap] system_settings row %s unreadable after write", embedSigningKeySetting)
		return
	}
	applyEmbedSigningKey(ctx, key, true)
}

// applyEmbedSigningKey exports the key via the environment (the same
// mechanism the desktop build uses) so every utils.SystemHMACKey caller —
// embed handles, presign, IM content rewrite — picks it up without a
// signature change. Never logs the value itself.
func applyEmbedSigningKey(ctx context.Context, key string, fresh bool) {
	if err := os.Setenv("SYSTEM_SIGNING_KEY", key); err != nil {
		logger.Warnf(ctx, "[bootstrap] cannot apply embed signing key: %v", err)
		return
	}
	if fresh {
		logger.Infof(ctx,
			"[bootstrap] auto-provisioned embed signing key (persisted as system_settings.%s); set SYSTEM_SIGNING_KEY explicitly to pin or rotate it",
			embedSigningKeySetting)
	} else {
		logger.Infof(ctx,
			"[bootstrap] restored auto-provisioned embed signing key from system_settings.%s",
			embedSigningKeySetting)
	}
}
