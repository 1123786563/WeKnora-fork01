// oc_revoke.go implements T08's durable revocation and the carried
// orphan/re-bind remediations (coordinator rulings 1a/1b/2):
//
//   - RevokeOCConnection: one transaction flips the tenant connection and its
//     binding to revoked, bumps the authorization version exactly once, and
//     enqueues exactly ONE durable remote-cleanup outbox row keyed by
//     (connection, new version, operation kind).
//   - ActivateOCAttemptRebind: the re-bind-aware activation (ruling 1b) --
//     idempotent when the existing binding carries the same external id at
//     the same generation, a CLEAR conflict otherwise. Additive next to the
//     frozen ActivateOCAttempt; the control worker adopts it when present.
//   - EnqueueOCOrphanCleanup: the extended cleanup triple (ruling 1a) for
//     external connections that can no longer be attributed to an attempt's
//     stored alias (the runtime minted its own alias and the adoption write
//     was lost).
//
// LOCKING CONTRACT (documented for T10's claim, ruling 2): every writer that
// mutates a connection's authorization generation or its binding/outbox rows
// MUST first take the connections row lock -- SELECT ... FROM connections
// WHERE tenant_id = ? AND id = ? FOR UPDATE on PostgreSQL (SQLite's
// serialized single-writer provides the same mutual exclusion) -- and then
// perform binding and outbox writes INSIDE the same transaction, never the
// reverse order. Revocation never takes any other row lock first, so T10's
// claim (which per plan locks action -> connection -> installation/grants)
// cannot deadlock against it: the two paths serialize on the connection row.
package appconnector

import (
	"context"
	"errors"
	"fmt"
	"time"

	appconnector "github.com/Tencent/WeKnora/internal/modules/appconnector"

	"gorm.io/gorm"
)

var (
	// ErrOCRevokeInvalid covers malformed revocation input rejected before
	// any database access: missing subject identity, a blank connection id,
	// or a non-positive expected version.
	ErrOCRevokeInvalid = errors.New("oc_revoke_invalid")
	// ErrOCRevokeConflict covers rejected revocations on a LIVE connection:
	// the expected version no longer matches, so the caller must re-read and
	// retry (the optimistic guard refuses to clobber a newer generation).
	ErrOCRevokeConflict = errors.New("oc_revoke_conflict")
)

// ocConnectionRevoked is the connections.state value a revocation writes
// (plan T08 SQL sketch; CanUseConnection already fails closed on any state
// other than "active").
const ocConnectionRevoked = "revoked"

// ocRevokeCleanupKind is the durable remote-cleanup operation kind. The
// frozen T05 kind set plus the frozen payload-less outbox make
// delete_connection the only executable cleanup: it resolves the external id
// through the tenant binding at execution time, while a delete_token row
// could never carry its required token-record id. The delete removes the
// external connection -- and with it every per-connection token minted
// against it -- without ever writing an empty allowlist (spec: revoking the
// last authorization deletes the token; allowedConnections=[] re-opens
// access upstream and is never written).
const ocRevokeCleanupKind = "delete_connection"

// ocRevokeOutboxID derives the outbox row's PRIMARY KEY from
// (tenant, connection, new version, kind): the frozen T03 schema has no
// unique key on those columns, so the deterministic id turns the PK into the
// plan's "unique key = connection + new version + operation kind". A
// duplicate insert of the same revoke is impossible in practice (the row
// lock serializes concurrent revokes and the idempotent path never inserts),
// and would surface as a constraint error rolling the transaction back.
func ocRevokeOutboxID(tenant uint64, connectionID string, version int64) string {
	return fmt.Sprintf("revoke|%d|%s|%d|%s", tenant, connectionID, version, ocRevokeCleanupKind)
}

// RevokeOCConnection durably revokes one tenant connection (plan T08):
//
//  1. Lock and read the connection row (FOR UPDATE on PostgreSQL).
//  2. Already revoked -> idempotent success: NO version bump, NO second
//     cleanup row (duplicate revocations never inflate the version).
//  3. Version mismatch on a live connection -> ErrOCRevokeConflict; nothing
//     is written (the plan's RowsAffected guard, see TestOCRevokeVersionGuard).
//  4. Conditional UPDATE connections SET state='revoked',
//     auth_version=expectedVersion+1 WHERE ... AND state <> 'revoked' -- the
//     stale guard is IN the SQL predicate, and RowsAffected must be exactly 1.
//  5. Mirror the binding (state='revoked', new auth_version,
//     binding_version+1 -- its optimistic counter moves with the row).
//  6. Insert the durable cleanup outbox row in the SAME transaction with a
//     deterministic id.
//
// The local transaction is the authorization authority: the moment it
// commits, Check rejects new requests on every path regardless of the
// control worker's availability; the outbox row only carries the REMOTE
// cleanup (which retries with backoff and never silently disappears).
func (s *OCStore) RevokeOCConnection(ctx context.Context, subject appconnector.OCSubject, connectionID string, expectedVersion int64) error {
	if subject.TenantID == 0 || subject.ActorID == "" || connectionID == "" || expectedVersion < 1 {
		return ErrOCRevokeInvalid
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. Connection row lock (see the locking contract above). Raw SQL so
		// the FOR UPDATE clause is dialect-gated exactly like outboxClaimSQL.
		lock := ""
		if tx.Dialector.Name() == "postgres" {
			lock = " FOR UPDATE"
		}
		var conn ConnectionRow
		if err := tx.Raw(
			"SELECT tenant_id, id, installation_id, kind, owner_id, credential_ref, state, auth_version FROM connections WHERE tenant_id = ? AND id = ?"+lock,
			subject.TenantID, connectionID).Scan(&conn).Error; err != nil {
			return err
		}
		if conn.ID == "" {
			return gorm.ErrRecordNotFound
		}
		// 2. Idempotent replay: the connection is already revoked, so this
		// request asks for something that already holds. No bump, no row.
		if conn.State == ocConnectionRevoked {
			return nil
		}
		// 3. Stale view of a live connection: refuse; the caller re-reads.
		if conn.AuthVersion != expectedVersion {
			return ErrOCRevokeConflict
		}
		// 4. Version-guarded conditional update; the guard lives in the SQL.
		res := tx.Model(&ConnectionRow{}).
			Where("tenant_id = ? AND id = ? AND auth_version = ? AND state <> ?", subject.TenantID, connectionID, expectedVersion, ocConnectionRevoked).
			Updates(map[string]interface{}{
				"state":        ocConnectionRevoked,
				"auth_version": expectedVersion + 1,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			// Impossible under the row lock; treat as a lost race regardless.
			return ErrOCRevokeConflict
		}
		newVersion := expectedVersion + 1
		// 5. Mirror the binding. A native connection has no binding row --
		// zero affected rows is legal; the connection row is the authority.
		if err := tx.Exec(
			"UPDATE connector_connection_bindings SET state = ?, auth_version = ?, binding_version = binding_version + 1 WHERE tenant_id = ? AND connection_id = ?",
			ocConnectionRevoked, newVersion, subject.TenantID, connectionID).Error; err != nil {
			return err
		}
		// 6. Durable remote cleanup, same transaction, deterministic id.
		now := time.Now().UTC()
		row := OCOperationsOutboxRow{
			ID:       ocRevokeOutboxID(subject.TenantID, connectionID, newVersion),
			TenantID: subject.TenantID, ResourceID: connectionID, ResourceVersion: newVersion,
			Kind: ocRevokeCleanupKind, NextAt: now, CreatedAt: now, UpdatedAt: now,
		}
		return tx.Create(&row).Error
	})
}

// ActivateOCAttemptRebind is the re-bind-aware activation (ruling 1b). It is
// ADDITIVE to the frozen ActivateOCAttempt -- same shape, same one-consume
// conditional transition -- except the binding step handles an existing row:
//
//   - no existing binding: insert the fresh active binding exactly like the
//     frozen method (BindingVersion 1);
//   - existing binding with the SAME external id at the SAME authorization
//     generation and still active: idempotent success -- the attempt is
//     consumed, the binding is NOT rewritten (a replayed activation must not
//     collide on the (tenant, connection) primary key);
//   - anything else (different external id, different generation, revoked or
//     pending row): ErrOCAttemptConflict -- a CLEAR conflict the worker turns
//     into reconciliation of the new external connection, never a blind PK
//     insert the outbox would retry against forever.
//
// Replacing the account behind a connection means revoking this connection
// and creating a NEW one: the binding's external identity (alias, external
// id) is immutable for the row's whole life (T03 SaveBinding contract).
func (s *OCStore) ActivateOCAttemptRebind(ctx context.Context, tenant uint64, id, externalID string, now time.Time) error {
	if tenant == 0 || id == "" || externalID == "" || now.IsZero() {
		return ErrOCAttemptInvalid
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var att OCAuthorizationAttemptRow
		if err := tx.Where("tenant_id = ? AND id = ?", tenant, id).First(&att).Error; err != nil {
			return err
		}
		var conn ConnectionRow
		if err := tx.Where("tenant_id = ? AND id = ?", tenant, att.ConnectionID).First(&conn).Error; err != nil {
			return err
		}
		if conn.AuthVersion != att.AuthVersion {
			return ErrOCAttemptConflict
		}
		var inst InstallationRow
		if err := tx.Where("id = ? AND tenant_id = ?", conn.InstallationID, tenant).First(&inst).Error; err != nil {
			return err
		}
		if inst.State != appconnector.InstallationActive {
			return ErrOCAttemptConflict
		}
		// Re-bind decision BEFORE consuming the attempt: a conflict must not
		// burn the one-consume transition.
		var cur OCBindingRow
		err := tx.Where("tenant_id = ? AND connection_id = ?", tenant, att.ConnectionID).First(&cur).Error
		hasBinding := true
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			hasBinding = false
		case err != nil:
			return err
		}
		if hasBinding {
			idempotent := cur.ExternalID == externalID &&
				cur.AuthVersion == att.AuthVersion &&
				cur.State == appconnector.OCBindingActive
			if !idempotent {
				return ErrOCAttemptConflict
			}
		}
		// One-consume (same predicate as the frozen activation).
		res := tx.Model(&OCAuthorizationAttemptRow{}).
			Where("id = ? AND tenant_id = ? AND state = ? AND expires_at > ?", id, tenant, ocAttemptVerifying, now.UTC()).
			Update("state", ocAttemptActive)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrOCAttemptConflict
		}
		if hasBinding {
			// Idempotent replay: attempt consumed above, binding untouched.
			return nil
		}
		binding := OCBindingRow{
			TenantID: tenant, ConnectionID: att.ConnectionID, RuntimeID: att.RuntimeID,
			Provider: att.Provider, ExternalID: externalID, Alias: att.Alias,
			AuthVersion: att.AuthVersion, BindingVersion: 1, State: appconnector.OCBindingActive,
		}
		return tx.Create(&binding).Error
	})
}

// ocOrphanCleanupID deduplicates orphan-cleanup rows for one
// (attempt, external connection) pair.
func ocOrphanCleanupID(attemptID, externalID string) string {
	return "cleanup-orphan|" + attemptID + "|" + externalID
}

// EnqueueOCOrphanCleanup extends the reconciliation triple (ruling 1a) to
// external connections the attempt can no longer prove ownership of through
// its STORED alias: the runtime minted its own alias for the connection, the
// adoption write was lost, and no retry can ever resolve it again. Such a
// connection is an orphan the moment the submitting execution fails, so this
// schedules its delete_connection immediately.
//
// Safety mirrors CleanupFailedOCAttempt: a newer ACTIVATED binding owning the
// external connection (matched by runtime + external id across tenants) wins
// and suppresses the cleanup -- the newest activation is never deleted. The
// attempt itself is NOT terminated here; whatever driver row still exists
// keeps retrying it through its own lifecycle.
func (s *OCStore) EnqueueOCOrphanCleanup(ctx context.Context, tenant uint64, attemptID, externalID string, now time.Time) error {
	if tenant == 0 || attemptID == "" || externalID == "" || now.IsZero() {
		return ErrOCAttemptInvalid
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var att OCAuthorizationAttemptRow
		if err := tx.Where("tenant_id = ? AND id = ?", tenant, attemptID).First(&att).Error; err != nil {
			return err
		}
		var bound int64
		if err := tx.Model(&OCBindingRow{}).
			Where("runtime_id = ? AND external_id = ? AND state = ?", att.RuntimeID, externalID, appconnector.OCBindingActive).
			Count(&bound).Error; err != nil {
			return err
		}
		if bound > 0 {
			// A newer activation owns the external connection now.
			return nil
		}
		// Deduplicate on the deterministic id: a repeated reconciliation for
		// the same (attempt, external) pair is a no-op, never an error.
		var exists int64
		if err := tx.Model(&OCOperationsOutboxRow{}).Where("id = ?", ocOrphanCleanupID(attemptID, externalID)).Count(&exists).Error; err != nil {
			return err
		}
		if exists > 0 {
			return nil
		}
		row := OCOperationsOutboxRow{
			ID:              ocOrphanCleanupID(attemptID, externalID),
			TenantID:        tenant,
			ResourceID:      attemptID + "|" + externalID,
			ResourceVersion: att.AuthVersion,
			Kind:            ocRevokeCleanupKind,
			NextAt:          now.UTC(), CreatedAt: now.UTC(), UpdatedAt: now.UTC(),
		}
		return tx.Create(&row).Error
	})
}
