package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const craftInputAdmissionLease = 2 * time.Minute

// craftInputCleanupBudget bounds the object-store rollback that follows a
// failed publish: the cleanup context is detached from the (usually already
// canceled or timed-out) request context, so without its own deadline a
// wedged database or object backend could pin the request indefinitely.
const craftInputCleanupBudget = 30 * time.Second

func craftInputLeaseExpiryPredicate(db *gorm.DB) string {
	if db.Name() == "sqlite" {
		return "julianday(lease_expires_at) <= julianday(?)"
	}
	return "lease_expires_at <= ?"
}

// AuthorizedInputLoader reads one referenced resource for a scope. It is the
// assembly-injected authorized read: it must resolve the ref against the
// existing upload/permission services and answer craft.ErrForbidden for any
// resource outside the caller's tenant — Craft never re-implements the
// resource ACL, it only refuses to stage what the loader refuses.
type AuthorizedInputLoader func(ctx context.Context, scope craft.Scope, ref string) ([]byte, error)

// WorkspaceFileWriter writes one file at a workspace-relative path inside the
// sandbox bound to the workspace. The workspace root is /workspace inside the
// container, so staged material lands under /workspace/inputs, which the
// runtime image mounts read-only for the OpenCode process.
type WorkspaceFileWriter func(ctx context.Context, workspace craft.Workspace, path string, content []byte) error

// CraftInputService stages authorized upload material into a Craft workspace
// as controlled read-only inputs. Account credentials never enter this path:
// only resource bytes the loader already authorized for the scope.
type CraftInputService struct {
	load  AuthorizedInputLoader
	write WorkspaceFileWriter
}

// NewCraftInputService assembles the input service from the authorized
// resource loader and the workspace-bound file writer.
func NewCraftInputService(
	load func(context.Context, craft.Scope, string) ([]byte, error),
	write func(context.Context, craft.Workspace, string, []byte) error,
) *CraftInputService {
	if load == nil || write == nil {
		panic("craft: NewCraftInputService requires a loader and a writer")
	}
	return &CraftInputService{load: load, write: write}
}

// Stage materializes inputs into the workspace and returns the citation
// manifest preserving each source ref. Every rule fails before delegation:
//
//   - the workspace must belong to the exact resolving scope;
//   - the declared manifest is validated (name, ref, digest shape,
//     per-file / per-round / total caps) before a single byte is read;
//   - every file is loaded and verified before any write happens, so a
//     cross-tenant refusal or a lying declaration leaves the writer at
//     zero calls;
//   - after the bounded read the actual size and SHA-256 must equal the
//     declaration and the caps are re-checked against real bytes;
//   - staged paths are content-addressed: inputs/<sha256>/<name>.
func (s *CraftInputService) Stage(
	ctx context.Context,
	scope craft.Scope,
	workspace craft.Workspace,
	inputs []craft.Input,
) ([]craft.Input, error) {
	if !craft.SameScope(scope, workspace.Scope) {
		return nil, fmt.Errorf("%w: workspace %s is bound to another scope", craft.ErrForbidden, workspace.ID)
	}
	if err := craft.ValidateInputManifest(inputs); err != nil {
		return nil, err
	}

	type stagedFile struct {
		path    string
		content []byte
	}
	staged := make([]stagedFile, 0, len(inputs))
	var total int64
	for _, in := range inputs {
		content, err := s.load(ctx, scope, in.Ref)
		if err != nil {
			return nil, err
		}
		// Second, post-read validation pass: the loader's bounded read is not
		// trusted to match the declaration.
		if int64(len(content)) > craft.MaxInputBytes {
			return nil, fmt.Errorf("%w: input %q read %d bytes over the %d cap",
				craft.ErrInvalidInput, in.Name, len(content), craft.MaxInputBytes)
		}
		total += int64(len(content))
		if total > craft.MaxTotalInputBytes {
			return nil, fmt.Errorf("%w: inputs read %d bytes over the %d total cap",
				craft.ErrInvalidInput, total, craft.MaxTotalInputBytes)
		}
		if int64(len(content)) != in.Bytes {
			return nil, fmt.Errorf("%w: input %q size mismatch: declared %d bytes, read %d",
				craft.ErrInvalidInput, in.Name, in.Bytes, len(content))
		}
		sum := sha256.Sum256(content)
		digest := hex.EncodeToString(sum[:])
		if digest != in.SHA256 {
			return nil, fmt.Errorf("%w: input %q digest mismatch: declared %s, read %s",
				craft.ErrInvalidInput, in.Name, in.SHA256, digest)
		}
		path, err := craft.InputPath(digest, in.Name)
		if err != nil {
			return nil, err
		}
		staged = append(staged, stagedFile{path: path, content: content})
	}

	for _, file := range staged {
		if err := s.write(ctx, workspace, file.path, file.content); err != nil {
			return nil, fmt.Errorf("craft: stage input at %s: %w", file.path, err)
		}
	}
	// The returned manifest preserves the source refs for citation.
	manifest := make([]craft.Input, len(inputs))
	copy(manifest, inputs)
	return manifest, nil
}

// CraftInputUpload is one file of a single acceptance round. The digest is
// supplied by the caller but always checked against the bytes before storage.
type CraftInputUpload struct {
	Name    string
	Content []byte
	SHA256  string
}

// AcceptInputRound persists a bounded round of opaque, immutable source
// objects. It never invokes a document parser or treats an extension as proof
// that the bytes were understood.
func (s *CraftSessionService) AcceptInputRound(ctx context.Context, scope craft.Scope, uploads []CraftInputUpload) ([]craft.Input, error) {
	if len(uploads) == 0 {
		return nil, fmt.Errorf("%w: empty input round", craft.ErrInvalidInput)
	}
	manifest := make([]craft.Input, 0, len(uploads))
	for _, upload := range uploads {
		manifest = append(manifest, craft.Input{Ref: "pending", Name: upload.Name, SHA256: upload.SHA256, Bytes: int64(len(upload.Content))})
	}
	if err := craft.ValidateInputManifest(manifest); err != nil {
		return nil, err
	}
	for i, upload := range uploads {
		sum := sha256.Sum256(upload.Content)
		if hex.EncodeToString(sum[:]) != upload.SHA256 {
			return nil, fmt.Errorf("%w: input %q digest mismatch", craft.ErrInvalidInput, upload.Name)
		}
		manifest[i].Recognition = recognizeCraftInput(upload.Name, upload.Content)
	}
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
	// Save only after all shape, quota and digest checks. Database rows are
	// inserted as one unit; on failure, only refs with no workspace association
	// are eligible for cleanup. A lookup error preserves the object fail-safe.
	// Cleanup runs detached from ctx: the failure that reached the rollback
	// is often the very cancellation or deadline of the request context, and
	// a canceled context would make the Count and DeleteFile calls fail
	// immediately, leaking every stored object. Detachment also drops the
	// deadline, so a finite cleanup budget bounds a wedged backend instead
	// of pinning the upload request forever.
	created := make([]string, 0, len(uploads))
	rollback := func() {
		// The budget starts HERE, not when the upload loop began: an upload
		// phase that consumed most of a pre-started budget would expire the
		// Count/DeleteFile calls below immediately and silently leak every
		// stored object (the errors were previously discarded outright).
		cleanupCtx, cleanupDone := context.WithTimeout(context.WithoutCancel(ctx), craftInputCleanupBudget)
		defer cleanupDone()
		seen := make(map[string]struct{}, len(created))
		for _, ref := range created {
			if _, ok := seen[ref]; ok {
				continue
			}
			seen[ref] = struct{}{}
			var associations int64
			if err := s.db.WithContext(cleanupCtx).Model(&craftWorkspaceInputRow{}).Where("ref = ?", ref).Count(&associations).Error; err != nil || associations != 0 {
				continue
			}
			if err := s.files.DeleteFile(cleanupCtx, ref); err != nil {
				logger.Warnf(cleanupCtx, "[CraftInput] rollback delete failed for ref %s (object may leak until reclamation): %v", ref, err)
			}
		}
	}
	for i, upload := range uploads {
		storageName := "craft_input_" + upload.SHA256 + filepath.Ext(upload.Name)
		ref, err := s.files.SaveBytes(ctx, upload.Content, session.TenantID, storageName, false)
		if err != nil {
			rollback()
			return nil, err
		}
		created = append(created, ref)
		manifest[i].Ref = ref
		manifest[i].CitationID = upload.SHA256
	}
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, in := range manifest {
			row := craftWorkspaceInputRow{
				WorkspaceID: workspace.ID, TenantID: session.TenantID, Ref: in.Ref,
				Name: in.Name, SHA256: in.SHA256, Bytes: in.Bytes, CitationID: in.CitationID,
				RecognitionAccepted: &in.Recognition.Accepted, RecognitionUnderstood: &in.Recognition.Understood,
				RecognitionReason: &in.Recognition.Reason, CreatedAt: s.now(),
			}
			result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				var existing craftWorkspaceInputRow
				if err := tx.Where("workspace_id = ? AND ref = ?", workspace.ID, in.Ref).Take(&existing).Error; err != nil {
					return err
				}
				if existing.TenantID != row.TenantID || existing.Name != row.Name || existing.SHA256 != row.SHA256 ||
					existing.Bytes != row.Bytes || existing.CitationID != row.CitationID || existing.RecognitionAccepted == nil ||
					existing.RecognitionUnderstood == nil || existing.RecognitionReason == nil ||
					*existing.RecognitionAccepted != *row.RecognitionAccepted ||
					*existing.RecognitionUnderstood != *row.RecognitionUnderstood || *existing.RecognitionReason != *row.RecognitionReason {
					return fmt.Errorf("%w: input ref %s already names different or legacy content", craft.ErrConflict, in.Ref)
				}
			}
		}
		return nil
	}); err != nil {
		rollback()
		return nil, err
	}
	return manifest, nil
}

func recognizeCraftInput(name string, content []byte) *craft.InputRecognition {
	reason := "unrecognized_format"
	if len(content) >= 2 && string(content[:2]) == "MZ" || len(content) >= 4 &&
		(string(content[:4]) == "\x7fELF" || string(content[:4]) == "\xca\xfe\xba\xbe" || string(content[:4]) == "\xfe\xed\xfa\xce" || string(content[:4]) == "\xfe\xed\xfa\xcf") {
		reason = "executable_binary"
	} else if !utf8.Valid(content) || strings.ContainsRune(string(content), '\x00') {
		reason = "binary_content"
	} else {
		// Recognition means a bounded consumer has actually parsed the bytes;
		// a filename or valid UTF-8 alone is not evidence of understanding.
		switch strings.ToLower(filepath.Ext(name)) {
		case ".json":
			if json.Valid(content) {
				return &craft.InputRecognition{Accepted: true, Understood: true}
			}
			reason = "parse_failed"
		case ".txt", ".md", ".markdown", ".csv", ".xml", ".yaml", ".yml", ".log":
			reason = "no_supported_parser"
		}
	}
	return &craft.InputRecognition{Accepted: true, Understood: false, Reason: reason}
}

func craftInputDecisionKey(ref string) string {
	sum := sha256.Sum256([]byte(ref))
	return hex.EncodeToString(sum[:])
}

func craftInputDecisionHash(action string, in craft.Input) string {
	sum := sha256.Sum256([]byte(action + "\x00" + in.Ref + "\x00" + in.SHA256))
	return hex.EncodeToString(sum[:])
}

func craftInputAdmissionHash(requestID string, in craft.Input) string {
	sum := sha256.Sum256([]byte("craft-input-admission-v1\x00" + requestID + "\x00" + craftInputDecisionHash("continue", in)))
	return hex.EncodeToString(sum[:])
}

// DecideInput persists the owner's explicit continue/cancel decision for one
// currently associated unrecognized input. A later change of mind replaces
// the decision, but no Run is admitted by this command itself.
func (s *CraftSessionService) DecideInput(ctx context.Context, scope craft.Scope, ref, action string) error {
	if action != "continue" && action != "cancel" {
		return fmt.Errorf("%w: invalid input decision", craft.ErrInvalidInput)
	}
	session, err := s.writeSession(ctx, scope, scope.SessionID)
	if err != nil {
		return err
	}
	workspace, err := s.store.GetWorkspace(ctx, ownerScopeOf(session))
	if err != nil {
		return err
	}
	inputs, err := s.resolveInputRefs(ctx, workspace.ID, []string{ref})
	if err != nil {
		return err
	}
	in := inputs[0]
	if in.Recognition == nil || in.Recognition.Understood {
		return fmt.Errorf("%w: input does not need a decision", craft.ErrInvalidInput)
	}
	row := craftSessionRequestRow{TenantID: session.TenantID, UserID: session.UserID,
		Purpose: "input_decision", RequestID: craftInputDecisionKey(ref),
		RequestHash: craftInputDecisionHash(action, in), SessionID: session.ID, CreatedAt: s.now()}
	claimedRunConflict := false
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Lock the decision row itself before checking the admission marker. The
		// Run claimant updates this same row with a compare-and-swap, so a
		// concurrent cancel either wins first or observes the committed claim.
		lock := tx.Model(&craftSessionRequestRow{}).
			Where("tenant_id = ? AND user_id = ? AND session_id = ? AND purpose = ? AND request_id = ?",
				session.TenantID, session.UserID, session.ID, "input_decision", row.RequestID).
			UpdateColumn("request_hash", gorm.Expr("request_hash"))
		if lock.Error != nil {
			return lock.Error
		}
		admissionLock := tx.Model(&craftSessionRequestRow{}).
			Where("tenant_id = ? AND user_id = ? AND session_id = ? AND purpose = ? AND request_id = ?",
				session.TenantID, session.UserID, session.ID, "input_admission", row.RequestID).
			UpdateColumn("admission_token", gorm.Expr("admission_token"))
		if admissionLock.Error != nil {
			return admissionLock.Error
		}
		var admission craftSessionRequestRow
		admissionErr := tx.Where("tenant_id = ? AND user_id = ? AND session_id = ? AND purpose = ? AND request_id = ?",
			session.TenantID, session.UserID, session.ID, "input_admission", row.RequestID).Take(&admission).Error
		if admissionErr == nil {
			if admission.AdmissionState != "claimed" || admission.AdmissionRunID == "" || admission.AdmissionToken == "" || admission.LeaseExpiresAt == nil {
				return fmt.Errorf("%w: input decision has a legacy or invalid admission claim", craft.ErrConflict)
			}
			if admission.LeaseExpiresAt.After(s.now()) {
				return fmt.Errorf("%w: input decision is currently being admitted by a Run", craft.ErrConflict)
			}
			var committed int64
			if err := tx.Table("agent_runs").Where(
				"tenant_id = ? AND owner_id = ? AND session_id = ? AND run_id = ?",
				session.TenantID, session.UserID, session.ID, admission.AdmissionRunID,
			).Count(&committed).Error; err != nil {
				return err
			}
			if committed != 0 {
				if err := tx.Model(&craftSessionRequestRow{}).
					Where("tenant_id = ? AND user_id = ? AND session_id = ? AND purpose = ? AND request_id = ? AND admission_token = ? AND admission_state = ?",
						session.TenantID, session.UserID, session.ID, "input_admission", row.RequestID, admission.AdmissionToken, "claimed").
					Update("admission_state", "admitted").Error; err != nil {
					return err
				}
				claimedRunConflict = true
				return nil
			}
			removed := tx.Where("tenant_id = ? AND user_id = ? AND session_id = ? AND purpose = ? AND request_id = ? AND admission_token = ? AND admission_state = ? AND "+craftInputLeaseExpiryPredicate(tx),
				session.TenantID, session.UserID, session.ID, "input_admission", row.RequestID, admission.AdmissionToken, "claimed", s.now()).
				Delete(&craftSessionRequestRow{})
			if removed.Error != nil {
				return removed.Error
			}
			if removed.RowsAffected != 1 {
				return fmt.Errorf("%w: input admission claim changed during cancellation", craft.ErrConflict)
			}
		} else if !errors.Is(admissionErr, gorm.ErrRecordNotFound) {
			return admissionErr
		}
		return tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "tenant_id"}, {Name: "user_id"}, {Name: "purpose"}, {Name: "request_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"request_hash", "session_id", "created_at"})}).Create(&row).Error
	})
	if err != nil {
		return err
	}
	if claimedRunConflict {
		return fmt.Errorf("%w: input decision already belongs to a durable Run", craft.ErrConflict)
	}
	return nil
}

// claimInputDecisions fences every selected, unrecognized input before the
// Run is submitted. Expiry only permits this transaction to rotate a token;
// it never grants an old Submit authority. The returned Run ID is the durable
// identity already attached to an abandoned claim, when recovering one.
func (s *CraftSessionService) claimInputDecisions(
	ctx context.Context,
	session *types.Session,
	inputs []craft.Input,
	requestID, proposedRunID, actorUserID string,
) ([]agentruntime.InputAdmissionClaim, string, error) {
	if proposedRunID == "" || requestID == "" || actorUserID == "" {
		return nil, "", craft.ErrConflict
	}
	claims := make([]agentruntime.InputAdmissionClaim, 0, len(inputs))
	runID := proposedRunID
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, in := range inputs {
			if in.Recognition == nil || in.Recognition.Understood {
				continue
			}
			key := craftInputDecisionKey(in.Ref)
			continueHash := craftInputDecisionHash("continue", in)
			// Keep the owner/session/request key stable for existing T01 claim
			// lookup and recovery, while making the decision digest actor-bound.
			claimHash := craftInputAdmissionHash(requestID+"\x00actor:"+actorUserID, in)

			decisionLock := tx.Model(&craftSessionRequestRow{}).
				Where("tenant_id = ? AND user_id = ? AND session_id = ? AND purpose = ? AND request_id = ?",
					session.TenantID, session.UserID, session.ID, "input_decision", key).
				UpdateColumn("request_hash", gorm.Expr("request_hash"))
			if decisionLock.Error != nil {
				return decisionLock.Error
			}
			var decision craftSessionRequestRow
			if err := tx.Where("tenant_id = ? AND user_id = ? AND session_id = ? AND purpose = ? AND request_id = ?",
				session.TenantID, session.UserID, session.ID, "input_decision", key).Take(&decision).Error; err != nil {
				return fmt.Errorf("%w: input %q requires a continue decision for this Run", craft.ErrConflict, in.Name)
			}

			var claim craftSessionRequestRow
			claimLock := tx.Model(&craftSessionRequestRow{}).
				Where("tenant_id = ? AND user_id = ? AND session_id = ? AND purpose = ? AND request_id = ?",
					session.TenantID, session.UserID, session.ID, "input_admission", key).
				UpdateColumn("admission_token", gorm.Expr("admission_token"))
			if claimLock.Error != nil {
				return claimLock.Error
			}
			claimErr := tx.Where("tenant_id = ? AND user_id = ? AND session_id = ? AND purpose = ? AND request_id = ?",
				session.TenantID, session.UserID, session.ID, "input_admission", key).Take(&claim).Error

			if decision.RequestHash == continueHash {
				if claimErr == nil {
					return fmt.Errorf("%w: input %q has a stale or inconsistent admission claim", craft.ErrConflict, in.Name)
				}
				if !errors.Is(claimErr, gorm.ErrRecordNotFound) {
					return claimErr
				}
				token := uuid.NewString()
				lease := s.now().Add(craftInputAdmissionLease)
				updated := tx.Model(&craftSessionRequestRow{}).
					Where("tenant_id = ? AND user_id = ? AND session_id = ? AND purpose = ? AND request_id = ? AND request_hash = ?",
						session.TenantID, session.UserID, session.ID, "input_decision", key, continueHash).
					Updates(map[string]any{"request_hash": claimHash, "created_at": s.now()})
				if updated.Error != nil {
					return updated.Error
				}
				if updated.RowsAffected != 1 {
					return fmt.Errorf("%w: input %q decision changed during admission", craft.ErrConflict, in.Name)
				}
				claim = craftSessionRequestRow{TenantID: session.TenantID, UserID: session.UserID,
					Purpose: "input_admission", RequestID: key, RequestHash: claimHash, SessionID: session.ID,
					AdmissionRunID: proposedRunID, AdmissionToken: token, AdmissionState: "claimed",
					LeaseExpiresAt: &lease, CreatedAt: s.now()}
				if err := tx.Create(&claim).Error; err != nil {
					return err
				}
			} else if decision.RequestHash != claimHash {
				return fmt.Errorf("%w: input %q requires a continue decision for this Run", craft.ErrConflict, in.Name)
			} else {
				if claimErr != nil {
					return fmt.Errorf("%w: input %q has an incomplete admission fence", craft.ErrConflict, in.Name)
				}
				if claim.RequestHash != claimHash || claim.AdmissionRunID == "" || claim.AdmissionToken == "" ||
					claim.SessionID != session.ID || (claim.AdmissionState != "claimed" && claim.AdmissionState != "admitted") {
					return fmt.Errorf("%w: input %q has an invalid admission fence", craft.ErrConflict, in.Name)
				}
				existingRun := tx.Table("agent_runs").Where(
					"tenant_id = ? AND owner_id = ? AND session_id = ? AND run_id = ? AND request_id = ?",
					session.TenantID, session.UserID, session.ID, claim.AdmissionRunID, requestID,
				).Select("1").Limit(1)
				var found int
				runErr := existingRun.Scan(&found).Error
				if runErr != nil {
					return runErr
				}
				hasRun := found == 1
				if claim.AdmissionState == "admitted" || hasRun {
					if !hasRun {
						return fmt.Errorf("%w: admitted input claim has no matching Run", craft.ErrConflict)
					}
					if claim.AdmissionState != "admitted" {
						if err := tx.Model(&craftSessionRequestRow{}).
							Where("tenant_id = ? AND user_id = ? AND session_id = ? AND purpose = ? AND request_id = ? AND admission_token = ? AND admission_state = ?",
								session.TenantID, session.UserID, session.ID, "input_admission", key, claim.AdmissionToken, "claimed").
							Update("admission_state", "admitted").Error; err != nil {
							return err
						}
					}
				} else {
					if claim.AdmissionState != "claimed" || claim.LeaseExpiresAt == nil || claim.LeaseExpiresAt.After(s.now()) {
						return fmt.Errorf("%w: input %q is already being admitted", craft.ErrConflict, in.Name)
					}
					if len(claims) == 0 {
						runID = claim.AdmissionRunID
					} else if runID != claim.AdmissionRunID {
						return fmt.Errorf("%w: selected inputs have different admission Runs", craft.ErrConflict)
					}
					token := uuid.NewString()
					lease := s.now().Add(craftInputAdmissionLease)
					rotated := tx.Model(&craftSessionRequestRow{}).
						Where("tenant_id = ? AND user_id = ? AND session_id = ? AND purpose = ? AND request_id = ? AND request_hash = ? AND admission_run_id = ? AND admission_token = ? AND admission_state = ? AND "+craftInputLeaseExpiryPredicate(tx),
							session.TenantID, session.UserID, session.ID, "input_admission", key, claimHash, claim.AdmissionRunID, claim.AdmissionToken, "claimed", s.now()).
						Updates(map[string]any{"admission_token": token, "lease_expires_at": lease, "created_at": s.now()})
					if rotated.Error != nil {
						return rotated.Error
					}
					if rotated.RowsAffected != 1 {
						return fmt.Errorf("%w: input %q admission claim changed during recovery", craft.ErrConflict, in.Name)
					}
					claim.AdmissionToken = token
					claim.LeaseExpiresAt = &lease
				}
			}
			if len(claims) == 0 {
				if claim.AdmissionRunID != proposedRunID && runID == proposedRunID {
					runID = claim.AdmissionRunID
				} else if claim.AdmissionRunID != runID {
					return fmt.Errorf("%w: selected inputs have different admission Runs", craft.ErrConflict)
				}
			} else if claim.AdmissionRunID != runID {
				return fmt.Errorf("%w: selected inputs have different admission Runs", craft.ErrConflict)
			}
			claims = append(claims, agentruntime.InputAdmissionClaim{
				DecisionKey: key, RunID: claim.AdmissionRunID, Token: claim.AdmissionToken,
			})
		}
		return nil
	})
	if err != nil {
		return nil, "", err
	}
	return claims, runID, nil
}
