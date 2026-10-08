package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"path"
	"path/filepath"

	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/craft"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ExpandArchive validates one associated archive input within the hard
// resource ceilings and publishes its regular-file members as
// content-addressed read-only inputs of the same workspace. The operation is
// all-or-nothing: either every member becomes an associated input backed by
// a stored immutable object, or the workspace keeps exactly its previous
// state and nothing partial is usable by a Run.
//
// The extraction runs in three gated phases:
//
//  1. authorize and load — the archive must already be an input of the
//     caller's workspace, the read is bounded by the input cap, and the
//     stored bytes must still hash to the content-addressed digest recorded
//     at acceptance;
//  2. extract — craft.ExtractArchive enforces canonical paths, duplicate
//     normalized paths, regular-file-only members, no nested archives and
//     the byte/count/depth/ratio ceilings, buffering members so a failure
//     here leaves the writer and the store untouched;
//  3. publish — member objects are stored, then the manifest rows are
//     inserted in one transaction; a failure rolls the objects back unless a
//     workspace association already protects them.
//
// The whole expansion is bounded by craft.MaxArchiveExtractDuration; CPU is
// bounded because each expanded byte costs O(1) work under the byte ceilings,
// and memory is bounded because members are buffered only up to the
// cumulative expanded-byte ceiling.
func (s *CraftSessionService) ExpandArchive(
	ctx context.Context,
	scope craft.Scope,
	ref string,
) ([]craft.Input, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, craft.MaxArchiveExtractDuration)
	defer cancel()

	session, err := s.writeSession(ctx, scope, scope.SessionID)
	if err != nil {
		return nil, err
	}
	if _, err := s.craftRow(ctx, session.TenantID, session.ID); err != nil {
		return nil, err
	}
	workspace, err := s.store.GetWorkspace(ctx, ownerScopeOf(session))
	if err != nil {
		return nil, err
	}

	// The archive must already be an associated input of this workspace;
	// expansion never reaches into another workspace's material.
	var archiveRow craftWorkspaceInputRow
	err = s.db.WithContext(ctx).Where("workspace_id = ? AND ref = ?", workspace.ID, ref).Take(&archiveRow).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("%w: input ref %q is not associated with this workspace", craft.ErrNotFound, ref)
	}
	if err != nil {
		return nil, err
	}
	if archiveRow.TenantID != session.TenantID {
		return nil, fmt.Errorf("%w: input ref %q belongs to another tenant", craft.ErrForbidden, ref)
	}

	reader, err := s.files.GetFile(ctx, ref)
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(io.LimitReader(reader, craft.MaxInputBytes+1))
	closeErr := reader.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if int64(len(data)) > craft.MaxInputBytes {
		return nil, fmt.Errorf("%w: archive %q exceeds the %d input byte cap",
			craft.ErrInvalidInput, archiveRow.Name, craft.MaxInputBytes)
	}
	sum := sha256.Sum256(data)
	digest := hex.EncodeToString(sum[:])
	if digest != archiveRow.SHA256 {
		return nil, fmt.Errorf("%w: archive %q digest mismatch: recorded %s, stored %s",
			craft.ErrInvalidInput, archiveRow.Name, archiveRow.SHA256, digest)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	members, err := craft.ExtractArchive(data)
	if err != nil {
		return nil, err
	}

	manifest := make([]craft.Input, len(members))
	for i, member := range members {
		memberSum := sha256.Sum256(member.Content)
		memberDigest := hex.EncodeToString(memberSum[:])
		// The member's archive-relative path was validated and deduplicated
		// inside ExtractArchive; the input manifest then names the member by
		// its base name, because the frozen T01 input contract names exactly
		// one canonical path element and every downstream consumer (Run
		// admission, staging, sandbox layout) validates that shape.
		name := path.Base(member.Path)
		if len(member.Content) == 0 {
			// Fail with the root cause instead of letting the shared T01
			// manifest gate report an opaque "no declared size" per name:
			// an archive holding empty members (for example a bare
			// __init__.py) cannot be expanded at all under the frozen
			// contract, and the caller must learn that up front.
			return nil, fmt.Errorf("%w: archive %q holds empty member %q; every input must carry bytes",
				craft.ErrInvalidInput, archiveRow.Name, name)
		}
		manifest[i] = craft.Input{
			Ref:         "pending",
			Name:        name,
			SHA256:      memberDigest,
			Bytes:       int64(len(member.Content)),
			CitationID:  memberDigest,
			Recognition: recognizeCraftInput(name, member.Content),
		}
	}
	// Second gate on the actually-extracted bytes, reusing the exact T01
	// round rules: name shape, digest shape and every quota cap.
	if err := craft.ValidateInputManifest(manifest); err != nil {
		return nil, err
	}
	// Fold same-identity members — the same base name and digest under
	// different archive paths, for example a/LICENSE and b/LICENSE — onto
	// one manifest entry before publishing. The identity reuse below only
	// sees committed rows, so without the fold both members of one expansion
	// would upload as separate objects and rows (production stores mint a
	// unique ref per save and the table keys rows by ref), duplicating the
	// input in the response and the workspace.
	folded := make([]craft.Input, 0, len(manifest))
	foldedMembers := make([]craft.ArchiveMember, 0, len(members))
	seenIdentity := make(map[string]struct{}, len(manifest))
	for i, entry := range manifest {
		identity := entry.Name + "\x00" + entry.SHA256
		if _, dup := seenIdentity[identity]; dup {
			continue
		}
		seenIdentity[identity] = struct{}{}
		folded = append(folded, entry)
		foldedMembers = append(foldedMembers, members[i])
	}
	manifest, members = folded, foldedMembers
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Save only after every check passed. Replay idempotency keys on the
	// member identity, not on the ref: production object stores mint a fresh
	// key on every SaveBytes (timestamp or uuid names), so a ref-keyed
	// conflict would never fire in production and every replay would
	// duplicate rows and objects. An existing row for the same
	// (workspace, name, digest) is reused verbatim — including its original
	// ref — and only genuinely new members are uploaded.
	created := make([]string, 0, len(members))
	// The failure that reaches rollback is often the very cancellation or
	// timeout of ctx (the hard extract budget); cleanup must run on a
	// detached context or the association Count and the DeleteFile calls
	// would fail immediately and leak every stored object. The detachment
	// also drops the deadline, so a finite cleanup budget is layered on top:
	// a wedged database or object backend must not pin the request forever
	// either.
	rollback := func() {
		// The budget starts HERE, when the failure is already known: a
		// budget that started before the (up to 100 MiB) member upload loop
		// would already be spent exactly when cleanup matters most.
		cleanupCtx, cleanupDone := context.WithTimeout(context.WithoutCancel(ctx), craftInputCleanupBudget)
		defer cleanupDone()
		seen := make(map[string]struct{}, len(created))
		for _, createdRef := range created {
			if _, ok := seen[createdRef]; ok {
				continue
			}
			seen[createdRef] = struct{}{}
			var associations int64
			if err := s.db.WithContext(cleanupCtx).Model(&craftWorkspaceInputRow{}).
				Where("ref = ?", createdRef).Count(&associations).Error; err != nil || associations != 0 {
				continue
			}
			if err := s.files.DeleteFile(cleanupCtx, createdRef); err != nil {
				logger.Warnf(cleanupCtx, "[CraftArchive] rollback delete failed for ref %s (object may leak until reclamation): %v", createdRef, err)
			}
		}
	}
	for i, member := range members {
		var existing craftWorkspaceInputRow
		err := s.db.WithContext(ctx).Where("workspace_id = ? AND name = ? AND sha256 = ?",
			workspace.ID, manifest[i].Name, manifest[i].SHA256).Take(&existing).Error
		if err == nil {
			// Legacy rows written while the recognition columns were being
			// added (ALTER TABLE ADD COLUMN, all three NULL) project a nil
			// Recognition; reusing one would nil-dereference in the publish
			// transaction below. They are conflicts, exactly like the
			// transaction's own fallback branch treats them.
			if existing.TenantID != session.TenantID || existing.Bytes != manifest[i].Bytes ||
				existing.RecognitionAccepted == nil || existing.RecognitionUnderstood == nil || existing.RecognitionReason == nil {
				rollback()
				return nil, fmt.Errorf("%w: member %s collides with different or legacy content",
					craft.ErrConflict, manifest[i].Name)
			}
			manifest[i] = existing.input()
			continue
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			rollback()
			return nil, err
		}
		storageName := "craft_input_" + manifest[i].SHA256 + filepath.Ext(member.Path)
		memberRef, err := s.files.SaveBytes(ctx, member.Content, session.TenantID, storageName, false)
		if err != nil {
			rollback()
			return nil, err
		}
		created = append(created, memberRef)
		manifest[i].Ref = memberRef
	}
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i := range manifest {
			row := craftWorkspaceInputRow{
				WorkspaceID: workspace.ID, TenantID: session.TenantID, Ref: manifest[i].Ref,
				Name: manifest[i].Name, SHA256: manifest[i].SHA256, Bytes: manifest[i].Bytes,
				CitationID:            manifest[i].CitationID,
				RecognitionAccepted:   &manifest[i].Recognition.Accepted,
				RecognitionUnderstood: &manifest[i].Recognition.Understood,
				RecognitionReason:     &manifest[i].Recognition.Reason,
				CreatedAt:             s.now(),
			}
			result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				// Idempotent collapse: production stores mint a fresh key
				// per SaveBytes, so a conflicting row means either a
				// deterministic-ref test backend replaying byte-identical
				// content or two same-identity archive members collapsing
				// onto one row; anything else is a conflict.
				var existing craftWorkspaceInputRow
				if err := tx.Where("workspace_id = ? AND ref = ?", workspace.ID, manifest[i].Ref).
					Take(&existing).Error; err != nil {
					return err
				}
				if existing.TenantID != row.TenantID || existing.Name != row.Name || existing.SHA256 != row.SHA256 ||
					existing.Bytes != row.Bytes || existing.CitationID != row.CitationID ||
					existing.RecognitionAccepted == nil || existing.RecognitionUnderstood == nil ||
					existing.RecognitionReason == nil ||
					*existing.RecognitionAccepted != *row.RecognitionAccepted ||
					*existing.RecognitionUnderstood != *row.RecognitionUnderstood ||
					*existing.RecognitionReason != *row.RecognitionReason {
					return fmt.Errorf("%w: member %s already names different or legacy content",
						craft.ErrConflict, manifest[i].Name)
				}
				manifest[i] = existing.input()
			}
		}
		return nil
	}); err != nil {
		rollback()
		return nil, err
	}
	return manifest, nil
}
