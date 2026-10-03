package career

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Material statuses and receipt kinds (T15). A material owns exactly one
// structured body that all three clients edit; user confirmation turns the
// current draft into the next immutable version and never rewrites history.
const (
	MaterialStatusDraft     = "draft"
	MaterialStatusFailed    = "failed"
	MaterialStatusConfirmed = "confirmed"

	MaterialKindEdited    = "material_edited"
	MaterialKindConfirmed = "material_confirmed"

	MaterialRiskMissingPlaceholder = "missing_placeholder"
	MaterialRiskNeedsReview        = "needs_review"

	MaterialChangeSectionAdded   = "section_added"
	MaterialChangeSectionRemoved = "section_removed"
	MaterialChangeSectionChanged = "section_changed"
	MaterialChangeClaimAdded     = "claim_added"
	MaterialChangeClaimRemoved   = "claim_removed"
	MaterialChangeClaimChanged   = "claim_changed"

	MaterialFailureClaimUnconfirmed = "claim_unconfirmed"

	materialFingerprintEdit    = "edit_material"
	materialFingerprintConfirm = "confirm_material"

	maxMaterialSections         = 64
	maxMaterialClaimsPerSection = 64
	maxMaterialHeadingBytes     = 256
	maxMaterialContentBytes     = 16384
	maxMaterialClaimIDBytes     = 128
	maxMaterialClaimTextBytes   = 4096
	maxMaterialReviewNoteBytes  = 512
	maxMaterialFactKeyBytes     = 128
)

var (
	ErrMaterialNotFound         = errors.New("career material not found")
	ErrMaterialVersionNotFound  = errors.New("career material version not found")
	ErrMaterialClaimUnconfirmed = errors.New("career material claim references an unconfirmed fact")
)

// EditMaterialInput is the frozen request body of the edit_material seam. An
// empty MaterialID creates a new material (freezing the opportunity snapshot
// and the current profile revision); a non-empty one edits that material's
// draft and can never redirect its frozen evidence.
type EditMaterialInput struct {
	RequestID        string       `json:"requestId"`
	MaterialID       string       `json:"materialId,omitempty"`
	OpportunityID    string       `json:"opportunityId,omitempty"`
	SnapshotID       string       `json:"snapshotId,omitempty"`
	Body             MaterialBody `json:"body"`
	ExpectedRevision uint64       `json:"expectedRevision"`
}

// ConfirmMaterialInput confirms the current draft as the next immutable
// version. The pinned evidence and every earlier version stay untouched.
type ConfirmMaterialInput struct {
	RequestID        string `json:"requestId"`
	MaterialID       string `json:"materialId"`
	ExpectedRevision uint64 `json:"expectedRevision"`
}

// MaterialClaim is one assertion inside the structured body. A claim with a
// FactKey must reference a confirmed career fact; a claim without one is only
// allowed as an explicit needs_review placeholder that never asserts facts.
type MaterialClaim struct {
	ClaimID     string `json:"claimId"`
	Text        string `json:"text"`
	FactKey     string `json:"factKey,omitempty"`
	NeedsReview bool   `json:"needsReview"`
	ReviewNote  string `json:"reviewNote,omitempty"`
}

type MaterialSection struct {
	Heading string          `json:"heading"`
	Content string          `json:"content"`
	Claims  []MaterialClaim `json:"claims"`
}

// MaterialBody is the single structured body shared by web, mobile, and the
// mini program. No client-specific forks of the body exist.
type MaterialBody struct {
	Sections []MaterialSection `json:"sections"`
}

// MaterialEvidencePin is the immutable generation evidence frozen at material
// creation: the opportunity snapshot (with its content digest) and the profile
// revision the material was generated from.
type MaterialEvidencePin struct {
	OpportunityID   string `json:"opportunityId"`
	SnapshotID      string `json:"snapshotId"`
	SnapshotSHA256  string `json:"snapshotSha256"`
	ProfileRevision uint64 `json:"profileRevision"`
}

// MaterialReviewRisk is one honest review annotation carried by the material.
// Placeholders for missing internships, certificates, or numbers surface here
// instead of being fabricated.
type MaterialReviewRisk struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	ClaimID string `json:"claimId,omitempty"`
}

// MaterialReceipt is the durable receipt of every material write and the
// frozen contract served by the receipt endpoint.
type MaterialReceipt struct {
	Kind           string               `json:"kind"`
	RequestID      string               `json:"requestId"`
	MaterialID     string               `json:"materialId"`
	Status         string               `json:"status"`
	Version        uint64               `json:"version,omitempty"`
	PinnedEvidence MaterialEvidencePin  `json:"pinnedEvidence"`
	Body           MaterialBody         `json:"body"`
	ReviewRisks    []MaterialReviewRisk `json:"reviewRisks"`
	FailureCode    string               `json:"failureCode,omitempty"`
	FailureMessage string               `json:"failureMessage,omitempty"`
}

type MaterialVersionSummary struct {
	Version   uint64    `json:"version"`
	CreatedAt time.Time `json:"createdAt"`
}

// MaterialVersionView is one immutable version: the whole structured body,
// its review risks, and the evidence basis frozen at confirmation.
type MaterialVersionView struct {
	Version           uint64               `json:"version"`
	PinnedEvidence    MaterialEvidencePin  `json:"pinnedEvidence"`
	FactBasisRevision uint64               `json:"factBasisRevision"`
	Body              MaterialBody         `json:"body"`
	ReviewRisks       []MaterialReviewRisk `json:"reviewRisks"`
	RequestID         string               `json:"requestId"`
	CreatedAt         time.Time            `json:"createdAt"`
}

type MaterialVersionChange struct {
	Kind     string `json:"kind"`
	Heading  string `json:"heading,omitempty"`
	ClaimID  string `json:"claimId,omitempty"`
	Baseline string `json:"baseline,omitempty"`
	Target   string `json:"target,omitempty"`
}

type MaterialVersionComparison struct {
	MaterialID string                  `json:"materialId"`
	Baseline   MaterialVersionView     `json:"baseline"`
	Target     MaterialVersionView     `json:"target"`
	Changes    []MaterialVersionChange `json:"changes"`
}

type MaterialView struct {
	MaterialID     string                   `json:"materialId"`
	Status         string                   `json:"status"`
	PinnedEvidence MaterialEvidencePin      `json:"pinnedEvidence"`
	Body           MaterialBody             `json:"body"`
	ReviewRisks    []MaterialReviewRisk     `json:"reviewRisks"`
	FailureCode    string                   `json:"failureCode,omitempty"`
	FailureMessage string                   `json:"failureMessage,omitempty"`
	VersionCount   uint64                   `json:"versionCount"`
	Versions       []MaterialVersionSummary `json:"versions"`
	CreatedAt      time.Time                `json:"createdAt"`
	UpdatedAt      time.Time                `json:"updatedAt"`
}

// materialRecord is the material aggregate: frozen evidence, the mutable
// draft body, the last failure reason, and the version counter.
type materialRecord struct {
	ID              string    `gorm:"primaryKey;size:36"`
	TenantID        uint64    `gorm:"uniqueIndex:career_material_scope_request;index:idx_career_material_scope"`
	UserID          string    `gorm:"uniqueIndex:career_material_scope_request;index:idx_career_material_scope;size:512"`
	RequestID       string    `gorm:"uniqueIndex:career_material_scope_request;size:128"`
	Fingerprint     string    `gorm:"size:64;not null"`
	OpportunityID   string    `gorm:"size:36;not null"`
	SnapshotID      string    `gorm:"size:36;not null"`
	ProfileRevision uint64    `gorm:"not null"`
	EvidenceBody    string    `gorm:"type:text;not null"`
	Status          string    `gorm:"size:16;not null;index:idx_career_material_status"`
	DraftBody       string    `gorm:"type:text;not null"`
	FailureCode     string    `gorm:"size:64;not null;default:''"`
	FailureMessage  string    `gorm:"type:text;not null;default:''"`
	VersionCount    uint64    `gorm:"not null"`
	ReceiptBody     string    `gorm:"type:text;not null"`
	CreatedAt       time.Time `gorm:"not null"`
	UpdatedAt       time.Time `gorm:"not null"`
}

func (materialRecord) TableName() string { return "career_materials" }

// materialVersionRecord is one immutable confirmed version. Versions are
// append-only: (scope, material, version) uniqueness makes overwrites
// impossible and every earlier body stays readable forever.
type materialVersionRecord struct {
	ID           string    `gorm:"primaryKey;size:36"`
	TenantID     uint64    `gorm:"uniqueIndex:career_material_version_scope_version;index:idx_career_material_version_scope"`
	UserID       string    `gorm:"uniqueIndex:career_material_version_scope_version;index:idx_career_material_version_scope;size:512"`
	MaterialID   string    `gorm:"size:36;uniqueIndex:career_material_version_scope_version"`
	Version      uint64    `gorm:"uniqueIndex:career_material_version_scope_version"`
	RequestID    string    `gorm:"size:128"`
	Fingerprint  string    `gorm:"size:64;not null"`
	EvidenceBody string    `gorm:"type:text;not null"`
	VersionBody  string    `gorm:"type:text;not null"`
	ReceiptBody  string    `gorm:"type:text;not null"`
	CreatedAt    time.Time `gorm:"not null"`
}

func (materialVersionRecord) TableName() string { return "career_material_versions" }

// materialReceiptRecord is the replay ledger for every material write
// (edit and confirm); one request ID admits exactly one intent.
type materialReceiptRecord struct {
	TenantID    uint64    `gorm:"uniqueIndex:career_material_receipt_scope_request"`
	UserID      string    `gorm:"uniqueIndex:career_material_receipt_scope_request;size:512"`
	RequestID   string    `gorm:"size:128;uniqueIndex:career_material_receipt_scope_request"`
	Fingerprint string    `gorm:"size:64;not null"`
	Body        string    `gorm:"type:text;not null"`
	CreatedAt   time.Time `gorm:"not null"`
}

func (materialReceiptRecord) TableName() string { return "career_material_receipts" }

// EditMaterial creates or edits the structured draft of one material. Creation
// freezes the opportunity snapshot digest and the current profile revision;
// edits never redirect that evidence. Claims may only reference confirmed
// facts, and a refused edit keeps the previous draft plus the typed reason.
func (o *Office) EditMaterial(ctx context.Context, input EditMaterialInput) (MaterialReceipt, error) {
	s, err := getScope(ctx)
	if err != nil {
		return MaterialReceipt{}, err
	}
	if err = o.requireSpace(ctx, s); err != nil {
		return MaterialReceipt{}, err
	}
	input.RequestID = strings.TrimSpace(input.RequestID)
	input.MaterialID = strings.TrimSpace(input.MaterialID)
	input.OpportunityID = strings.TrimSpace(input.OpportunityID)
	input.SnapshotID = strings.TrimSpace(input.SnapshotID)
	if input.RequestID == "" || len(input.RequestID) > 128 {
		return MaterialReceipt{}, ErrInvalidRequest
	}
	if input.MaterialID == "" {
		if input.OpportunityID == "" || input.SnapshotID == "" {
			return MaterialReceipt{}, ErrInvalidRequest
		}
	} else if len(input.MaterialID) > 36 {
		return MaterialReceipt{}, ErrInvalidRequest
	}
	if err = validateMaterialShape(input.Body); err != nil {
		return MaterialReceipt{}, err
	}
	fingerprint, err := materialFingerprint(materialFingerprintEdit, input.RequestID, input.MaterialID, input.OpportunityID, input.SnapshotID, input.Body, input.ExpectedRevision)
	if err != nil {
		return MaterialReceipt{}, err
	}

	// Exact replay precedes every side effect.
	if replay, found, lookupErr := o.replayMaterialReceipt(ctx, s, input.RequestID, fingerprint); lookupErr != nil {
		return MaterialReceipt{}, lookupErr
	} else if found {
		return replay, nil
	}

	// Editing an existing material first confirms the frozen evidence cannot
	// be redirected onto another opportunity snapshot.
	var editRow *materialRecord
	if input.MaterialID != "" {
		row, loadErr := o.loadMaterialRow(ctx, s, input.MaterialID)
		if loadErr != nil {
			return MaterialReceipt{}, loadErr
		}
		if input.OpportunityID != "" && (input.OpportunityID != row.OpportunityID || input.SnapshotID != row.SnapshotID) {
			return MaterialReceipt{}, ErrInvalidRequest
		}
		editRow = &row
	}
	// Claims are reviewed before the revision CAS: a claim on an unconfirmed
	// fact is refused first, and an existing draft keeps its last good body
	// plus the typed reason.
	headRevision, headErr := materialHeadRevision(o.db.WithContext(ctx), s)
	if headErr != nil {
		return MaterialReceipt{}, headErr
	}
	if claimErr := validateMaterialClaims(o.db.WithContext(ctx), s, headRevision, input.Body); claimErr != nil {
		if errors.Is(claimErr, ErrMaterialClaimUnconfirmed) && editRow != nil {
			if persistErr := o.persistMaterialFailure(ctx, s, editRow.ID, MaterialFailureClaimUnconfirmed, claimErr.Error()); persistErr != nil {
				return MaterialReceipt{}, persistErr
			}
		}
		return MaterialReceipt{}, claimErr
	}

	var receipt MaterialReceipt
	err = o.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var stored materialReceiptRecord
		e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, input.RequestID).
			First(&stored).Error
		if e == nil {
			if stored.Fingerprint != fingerprint {
				return ErrIdempotencyConflict
			}
			return decodeMaterialReceipt(stored.Body, &receipt)
		}
		if !errors.Is(e, gorm.ErrRecordNotFound) {
			return e
		}
		if e := requireGateActiveTx(tx, s); e != nil {
			return e
		}
		var head profile
		e = tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tenant_id=? AND user_id=?", s.TenantID, s.UserID).First(&head).Error
		if errors.Is(e, gorm.ErrRecordNotFound) {
			head.Revision = 0
		} else if e != nil {
			return e
		}
		if head.Revision != input.ExpectedRevision {
			return &RevisionConflictError{CurrentRevision: head.Revision}
		}
		now := time.Now().UTC()
		if input.MaterialID == "" {
			// The same request ID may not also own a material from another
			// seam (a finalized preparation writes career_materials directly,
			// without a career_material_receipts row): the unique index on
			// career_materials.request_id would otherwise surface as an
			// untyped 500. Refuse it as the typed 409 idempotency conflict.
			var occupied materialRecord
			e = tx.Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, input.RequestID).
				First(&occupied).Error
			if e == nil {
				return ErrIdempotencyConflict
			}
			if !errors.Is(e, gorm.ErrRecordNotFound) {
				return e
			}
			var snapshot opportunitySnapshot
			e = tx.Where("tenant_id=? AND user_id=? AND opportunity_id=? AND id=?",
				s.TenantID, s.UserID, input.OpportunityID, input.SnapshotID).First(&snapshot).Error
			if errors.Is(e, gorm.ErrRecordNotFound) {
				// Pre-merge references keep resolving: a merged candidate's
				// snapshots were re-parented onto the merge target, so retry
				// through the merge chain (T12 reconciliation).
				if canonical := canonicalOpportunityID(tx, s, input.OpportunityID); canonical != "" && canonical != input.OpportunityID {
					e = tx.Where("tenant_id=? AND user_id=? AND opportunity_id=? AND id=?",
						s.TenantID, s.UserID, canonical, input.SnapshotID).First(&snapshot).Error
				}
			}
			if errors.Is(e, gorm.ErrRecordNotFound) {
				return ErrOpportunityNotFound
			}
			if e != nil {
				return e
			}
			if claimErr := validateMaterialClaims(tx, s, head.Revision, input.Body); claimErr != nil {
				return claimErr
			}
			pin := MaterialEvidencePin{
				OpportunityID:   snapshot.OpportunityID,
				SnapshotID:      snapshot.ID,
				SnapshotSHA256:  snapshot.RawSHA256,
				ProfileRevision: head.Revision,
			}
			receipt, e = buildMaterialReceipt(MaterialKindEdited, input.RequestID, uuid.NewString(), MaterialStatusDraft, 0, pin, input.Body)
			if e != nil {
				return e
			}
			e = insertMaterialRow(tx, s, receipt, fingerprint, pin, input, now)
			if e != nil {
				return e
			}
		} else {
			var row materialRecord
			e = tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("tenant_id=? AND user_id=? AND id=?", s.TenantID, s.UserID, input.MaterialID).
				First(&row).Error
			if errors.Is(e, gorm.ErrRecordNotFound) {
				return ErrMaterialNotFound
			}
			if e != nil {
				return e
			}
			if input.OpportunityID != "" && (input.OpportunityID != row.OpportunityID || input.SnapshotID != row.SnapshotID) {
				return ErrInvalidRequest
			}
			if claimErr := validateMaterialClaims(tx, s, head.Revision, input.Body); claimErr != nil {
				return claimErr
			}
			var pin MaterialEvidencePin
			if e = json.Unmarshal([]byte(row.EvidenceBody), &pin); e != nil {
				return e
			}
			receipt, e = buildMaterialReceipt(MaterialKindEdited, input.RequestID, row.ID, MaterialStatusDraft, 0, pin, input.Body)
			if e != nil {
				return e
			}
			if e = tx.Model(&materialRecord{}).
				Where("tenant_id=? AND user_id=? AND id=?", s.TenantID, s.UserID, row.ID).
				Updates(map[string]any{
					"draft_body":      string(mustJSON(receipt.Body)),
					"status":          MaterialStatusDraft,
					"failure_code":    "",
					"failure_message": "",
					"receipt_body":    string(mustJSON(receipt)),
					"updated_at":      now,
				}).Error; e != nil {
				return e
			}
		}
		return tx.Create(&materialReceiptRecord{
			TenantID: s.TenantID, UserID: s.UserID, RequestID: input.RequestID,
			Fingerprint: fingerprint, Body: string(mustJSON(receipt)), CreatedAt: now,
		}).Error
	})
	if err != nil {
		if errors.Is(err, ErrIdempotencyConflict) || errors.Is(err, ErrInvalidRequest) ||
			errors.Is(err, ErrOpportunityNotFound) || errors.Is(err, ErrMaterialNotFound) ||
			errors.Is(err, ErrMaterialClaimUnconfirmed) {
			return MaterialReceipt{}, err
		}
		var revisionConflict *RevisionConflictError
		if errors.As(err, &revisionConflict) {
			return MaterialReceipt{}, err
		}
		if isReceiptRaceError(err) {
			replay, found, lookupErr := o.replayMaterialReceipt(ctx, s, input.RequestID, fingerprint)
			if lookupErr != nil {
				return MaterialReceipt{}, lookupErr
			}
			if found {
				return replay, nil
			}
		}
		if ctx.Err() != nil {
			return MaterialReceipt{}, &OutcomeUnknownError{RequestID: input.RequestID}
		}
		return MaterialReceipt{}, err
	}
	return receipt, nil
}

// ConfirmMaterial turns the current draft into the next immutable version.
// The whole body, its review risks, and the evidence basis are snapshotted;
// no earlier version row is ever rewritten.
func (o *Office) ConfirmMaterial(ctx context.Context, input ConfirmMaterialInput) (MaterialReceipt, error) {
	s, err := getScope(ctx)
	if err != nil {
		return MaterialReceipt{}, err
	}
	if err = o.requireSpace(ctx, s); err != nil {
		return MaterialReceipt{}, err
	}
	input.RequestID = strings.TrimSpace(input.RequestID)
	input.MaterialID = strings.TrimSpace(input.MaterialID)
	if input.RequestID == "" || len(input.RequestID) > 128 || input.MaterialID == "" || len(input.MaterialID) > 36 {
		return MaterialReceipt{}, ErrInvalidRequest
	}
	fingerprint, err := materialFingerprint(materialFingerprintConfirm, input.RequestID, input.MaterialID, input.ExpectedRevision)
	if err != nil {
		return MaterialReceipt{}, err
	}
	if replay, found, lookupErr := o.replayMaterialReceipt(ctx, s, input.RequestID, fingerprint); lookupErr != nil {
		return MaterialReceipt{}, lookupErr
	} else if found {
		return replay, nil
	}

	// Review the stored draft before anything else; a refusal persists the
	// reason and publishes nothing.
	row, loadErr := o.loadMaterialRow(ctx, s, input.MaterialID)
	if loadErr != nil {
		return MaterialReceipt{}, loadErr
	}
	var draft MaterialBody
	if err = json.Unmarshal([]byte(row.DraftBody), &draft); err != nil {
		return MaterialReceipt{}, fmt.Errorf("decode career material draft: %w", err)
	}
	head, headErr := materialHeadRevision(o.db.WithContext(ctx), s)
	if headErr != nil {
		return MaterialReceipt{}, headErr
	}
	if claimErr := validateMaterialClaims(o.db.WithContext(ctx), s, head, draft); claimErr != nil {
		if errors.Is(claimErr, ErrMaterialClaimUnconfirmed) {
			if persistErr := o.persistMaterialFailure(ctx, s, row.ID, MaterialFailureClaimUnconfirmed, claimErr.Error()); persistErr != nil {
				return MaterialReceipt{}, persistErr
			}
		}
		return MaterialReceipt{}, claimErr
	}

	var receipt MaterialReceipt
	err = o.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var stored materialReceiptRecord
		e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, input.RequestID).
			First(&stored).Error
		if e == nil {
			if stored.Fingerprint != fingerprint {
				return ErrIdempotencyConflict
			}
			return decodeMaterialReceipt(stored.Body, &receipt)
		}
		if !errors.Is(e, gorm.ErrRecordNotFound) {
			return e
		}
		if e := requireGateActiveTx(tx, s); e != nil {
			return e
		}
		var head profile
		e = tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tenant_id=? AND user_id=?", s.TenantID, s.UserID).First(&head).Error
		if errors.Is(e, gorm.ErrRecordNotFound) {
			head.Revision = 0
		} else if e != nil {
			return e
		}
		if head.Revision != input.ExpectedRevision {
			return &RevisionConflictError{CurrentRevision: head.Revision}
		}
		var locked materialRecord
		e = tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tenant_id=? AND user_id=? AND id=?", s.TenantID, s.UserID, input.MaterialID).
			First(&locked).Error
		if errors.Is(e, gorm.ErrRecordNotFound) {
			return ErrMaterialNotFound
		}
		if e != nil {
			return e
		}
		if e = json.Unmarshal([]byte(locked.DraftBody), &draft); e != nil {
			return fmt.Errorf("decode career material draft: %w", e)
		}
		if claimErr := validateMaterialClaims(tx, s, head.Revision, draft); claimErr != nil {
			return claimErr
		}
		var pin MaterialEvidencePin
		if e = json.Unmarshal([]byte(locked.EvidenceBody), &pin); e != nil {
			return e
		}
		now := time.Now().UTC()
		version := locked.VersionCount + 1
		versionView := MaterialVersionView{
			Version:           version,
			PinnedEvidence:    pin,
			FactBasisRevision: head.Revision,
			Body:              draft,
			ReviewRisks:       materialReviewRisks(draft),
			RequestID:         input.RequestID,
			CreatedAt:         now,
		}
		receipt, e = buildMaterialReceipt(MaterialKindConfirmed, input.RequestID, locked.ID, MaterialStatusConfirmed, version, pin, draft)
		if e != nil {
			return e
		}
		if e = tx.Create(&materialVersionRecord{
			ID: uuid.NewString(), TenantID: s.TenantID, UserID: s.UserID,
			MaterialID: locked.ID, Version: version, RequestID: input.RequestID,
			Fingerprint: fingerprint, EvidenceBody: locked.EvidenceBody,
			VersionBody: string(mustJSON(versionView)), ReceiptBody: string(mustJSON(receipt)),
			CreatedAt: now,
		}).Error; e != nil {
			return e
		}
		if e = tx.Model(&materialRecord{}).
			Where("tenant_id=? AND user_id=? AND id=?", s.TenantID, s.UserID, locked.ID).
			Updates(map[string]any{
				"version_count":   version,
				"status":          MaterialStatusDraft,
				"failure_code":    "",
				"failure_message": "",
				"receipt_body":    string(mustJSON(receipt)),
				"updated_at":      now,
			}).Error; e != nil {
			return e
		}
		return tx.Create(&materialReceiptRecord{
			TenantID: s.TenantID, UserID: s.UserID, RequestID: input.RequestID,
			Fingerprint: fingerprint, Body: string(mustJSON(receipt)), CreatedAt: now,
		}).Error
	})
	if err != nil {
		if errors.Is(err, ErrIdempotencyConflict) || errors.Is(err, ErrInvalidRequest) ||
			errors.Is(err, ErrMaterialNotFound) || errors.Is(err, ErrMaterialClaimUnconfirmed) {
			return MaterialReceipt{}, err
		}
		var revisionConflict *RevisionConflictError
		if errors.As(err, &revisionConflict) {
			return MaterialReceipt{}, err
		}
		if isReceiptRaceError(err) {
			replay, found, lookupErr := o.replayMaterialReceipt(ctx, s, input.RequestID, fingerprint)
			if lookupErr != nil {
				return MaterialReceipt{}, lookupErr
			}
			if found {
				return replay, nil
			}
		}
		if ctx.Err() != nil {
			return MaterialReceipt{}, &OutcomeUnknownError{RequestID: input.RequestID}
		}
		return MaterialReceipt{}, err
	}
	return receipt, nil
}

// FindMaterialReceipt replays the stored material receipt by request ID.
func (o *Office) FindMaterialReceipt(ctx context.Context, requestID string) (MaterialReceipt, error) {
	s, err := getScope(ctx)
	if err != nil {
		return MaterialReceipt{}, err
	}
	if err = o.requireSpace(ctx, s); err != nil {
		return MaterialReceipt{}, err
	}
	requestID = strings.TrimSpace(requestID)
	if requestID == "" || len(requestID) > 128 {
		return MaterialReceipt{}, ErrInvalidRequest
	}
	var row materialReceiptRecord
	err = o.db.WithContext(ctx).
		Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, requestID).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return MaterialReceipt{}, ErrReceiptNotFound
	}
	if err != nil {
		return MaterialReceipt{}, err
	}
	var receipt MaterialReceipt
	if err = decodeMaterialReceipt(row.Body, &receipt); err != nil {
		return MaterialReceipt{}, err
	}
	return receipt, nil
}

// Material returns one material with its draft, review risks, and version
// history under the authenticated scope.
func (o *Office) Material(ctx context.Context, materialID string) (MaterialView, error) {
	s, err := getScope(ctx)
	if err != nil {
		return MaterialView{}, err
	}
	if err = o.requireSpace(ctx, s); err != nil {
		return MaterialView{}, err
	}
	row, err := o.loadMaterialRow(ctx, s, strings.TrimSpace(materialID))
	if err != nil {
		return MaterialView{}, err
	}
	return o.materialView(ctx, s, row)
}

// MaterialVersions lists the immutable version history of one material.
func (o *Office) MaterialVersions(ctx context.Context, materialID string) ([]MaterialVersionSummary, error) {
	s, err := getScope(ctx)
	if err != nil {
		return nil, err
	}
	if err = o.requireSpace(ctx, s); err != nil {
		return nil, err
	}
	if _, err = o.loadMaterialRow(ctx, s, strings.TrimSpace(materialID)); err != nil {
		return nil, err
	}
	return o.materialVersionSummaries(ctx, s, strings.TrimSpace(materialID))
}

// MaterialVersion returns one immutable version by its number.
func (o *Office) MaterialVersion(ctx context.Context, materialID string, version uint64) (MaterialVersionView, error) {
	s, err := getScope(ctx)
	if err != nil {
		return MaterialVersionView{}, err
	}
	if err = o.requireSpace(ctx, s); err != nil {
		return MaterialVersionView{}, err
	}
	if _, err = o.loadMaterialRow(ctx, s, strings.TrimSpace(materialID)); err != nil {
		return MaterialVersionView{}, err
	}
	if version == 0 {
		return MaterialVersionView{}, ErrInvalidRequest
	}
	var row materialVersionRecord
	err = o.db.WithContext(ctx).
		Where("tenant_id=? AND user_id=? AND material_id=? AND version=?", s.TenantID, s.UserID, strings.TrimSpace(materialID), version).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return MaterialVersionView{}, ErrMaterialVersionNotFound
	}
	if err != nil {
		return MaterialVersionView{}, err
	}
	return decodeMaterialVersion(row.VersionBody)
}

// CompareMaterialVersions returns the honest section/claim diff between the
// baseline and target immutable versions. Old versions stay comparable
// forever because neither row can change.
func (o *Office) CompareMaterialVersions(ctx context.Context, materialID string, baseline, target uint64) (MaterialVersionComparison, error) {
	s, err := getScope(ctx)
	if err != nil {
		return MaterialVersionComparison{}, err
	}
	if err = o.requireSpace(ctx, s); err != nil {
		return MaterialVersionComparison{}, err
	}
	materialID = strings.TrimSpace(materialID)
	if _, err = o.loadMaterialRow(ctx, s, materialID); err != nil {
		return MaterialVersionComparison{}, err
	}
	if baseline == 0 || target == 0 {
		return MaterialVersionComparison{}, ErrInvalidRequest
	}
	baselineView, err := o.MaterialVersion(ctx, materialID, baseline)
	if err != nil {
		return MaterialVersionComparison{}, err
	}
	targetView, err := o.MaterialVersion(ctx, materialID, target)
	if err != nil {
		return MaterialVersionComparison{}, err
	}
	return MaterialVersionComparison{
		MaterialID: materialID,
		Baseline:   baselineView,
		Target:     targetView,
		Changes:    diffMaterialBodies(baselineView.Body, targetView.Body),
	}, nil
}

func (o *Office) materialView(ctx context.Context, s Scope, row materialRecord) (MaterialView, error) {
	var body MaterialBody
	if err := json.Unmarshal([]byte(row.DraftBody), &body); err != nil {
		return MaterialView{}, fmt.Errorf("decode career material draft: %w", err)
	}
	var pin MaterialEvidencePin
	if err := json.Unmarshal([]byte(row.EvidenceBody), &pin); err != nil {
		return MaterialView{}, fmt.Errorf("decode career material evidence: %w", err)
	}
	versions, err := o.materialVersionSummaries(ctx, s, row.ID)
	if err != nil {
		return MaterialView{}, err
	}
	if versions == nil {
		versions = []MaterialVersionSummary{}
	}
	return MaterialView{
		MaterialID:     row.ID,
		Status:         row.Status,
		PinnedEvidence: pin,
		Body:           body,
		ReviewRisks:    materialReviewRisks(body),
		FailureCode:    row.FailureCode,
		FailureMessage: row.FailureMessage,
		VersionCount:   row.VersionCount,
		Versions:       versions,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
	}, nil
}

func (o *Office) materialVersionSummaries(ctx context.Context, s Scope, materialID string) ([]MaterialVersionSummary, error) {
	var rows []materialVersionRecord
	err := o.db.WithContext(ctx).
		Where("tenant_id=? AND user_id=? AND material_id=?", s.TenantID, s.UserID, materialID).
		Order("version ASC").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]MaterialVersionSummary, 0, len(rows))
	for _, row := range rows {
		out = append(out, MaterialVersionSummary{Version: row.Version, CreatedAt: row.CreatedAt})
	}
	return out, nil
}

func (o *Office) loadMaterialRow(ctx context.Context, s Scope, materialID string) (materialRecord, error) {
	if materialID == "" || len(materialID) > 36 {
		return materialRecord{}, ErrInvalidRequest
	}
	var row materialRecord
	err := o.db.WithContext(ctx).
		Where("tenant_id=? AND user_id=? AND id=?", s.TenantID, s.UserID, materialID).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return materialRecord{}, ErrMaterialNotFound
	}
	if err != nil {
		return materialRecord{}, err
	}
	return row, nil
}

// persistMaterialFailure records the typed review failure on the material
// without touching the draft: failures preserve the draft and the reason.
func (o *Office) persistMaterialFailure(ctx context.Context, s Scope, materialID, code, message string) error {
	return o.db.WithContext(ctx).Model(&materialRecord{}).
		Where("tenant_id=? AND user_id=? AND id=?", s.TenantID, s.UserID, materialID).
		Updates(map[string]any{
			"status":          MaterialStatusFailed,
			"failure_code":    code,
			"failure_message": message,
			"updated_at":      time.Now().UTC(),
		}).Error
}

func (o *Office) replayMaterialReceipt(ctx context.Context, s Scope, requestID, fingerprint string) (MaterialReceipt, bool, error) {
	var row materialReceiptRecord
	err := o.db.WithContext(ctx).
		Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, requestID).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return MaterialReceipt{}, false, nil
	}
	if err != nil {
		return MaterialReceipt{}, false, err
	}
	if row.Fingerprint != fingerprint {
		return MaterialReceipt{}, true, ErrIdempotencyConflict
	}
	var receipt MaterialReceipt
	if err = decodeMaterialReceipt(row.Body, &receipt); err != nil {
		return MaterialReceipt{}, true, err
	}
	return receipt, true, nil
}

func materialHeadRevision(db *gorm.DB, s Scope) (uint64, error) {
	var head profile
	err := db.Where("tenant_id=? AND user_id=?", s.TenantID, s.UserID).First(&head).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return head.Revision, nil
}

// validateMaterialShape enforces the bounded structured-body shape.
func validateMaterialShape(body MaterialBody) error {
	if len(body.Sections) == 0 || len(body.Sections) > maxMaterialSections {
		return ErrInvalidRequest
	}
	seenClaims := map[string]bool{}
	for _, section := range body.Sections {
		if strings.TrimSpace(section.Heading) == "" || len(section.Heading) > maxMaterialHeadingBytes {
			return ErrInvalidRequest
		}
		if len(section.Content) > maxMaterialContentBytes || len(section.Claims) > maxMaterialClaimsPerSection {
			return ErrInvalidRequest
		}
		for _, claim := range section.Claims {
			if strings.TrimSpace(claim.ClaimID) == "" || len(claim.ClaimID) > maxMaterialClaimIDBytes {
				return ErrInvalidRequest
			}
			if seenClaims[claim.ClaimID] {
				return ErrInvalidRequest
			}
			seenClaims[claim.ClaimID] = true
			if len(claim.Text) > maxMaterialClaimTextBytes || len(claim.ReviewNote) > maxMaterialReviewNoteBytes || len(claim.FactKey) > maxMaterialFactKeyBytes {
				return ErrInvalidRequest
			}
			// An asserted claim without a fact reference is a fabricated
			// assertion; missing items must use explicit needs_review
			// placeholders instead.
			if claim.FactKey == "" && !claim.NeedsReview {
				return fmt.Errorf("%w: claim %s asserts content without a confirmed fact", ErrMaterialClaimUnconfirmed, claim.ClaimID)
			}
		}
	}
	return nil
}

// validateMaterialClaims verifies every fact-referencing claim against the
// facts confirmed at (or before) the given profile revision. Unconfirmed
// references never enter the durable body.
func validateMaterialClaims(db *gorm.DB, s Scope, revision uint64, body MaterialBody) error {
	needed := map[string]bool{}
	for _, section := range body.Sections {
		for _, claim := range section.Claims {
			if claim.FactKey != "" {
				needed[claim.FactKey] = true
			}
		}
	}
	if len(needed) == 0 {
		return nil
	}
	facts, err := confirmedFactsAtRevision(db, s, revision)
	if err != nil {
		return err
	}
	confirmed := make(map[string]bool, len(facts))
	for _, fact := range facts {
		confirmed[fact.Key] = true
	}
	for key := range needed {
		if !confirmed[key] {
			return fmt.Errorf("%w: %s", ErrMaterialClaimUnconfirmed, key)
		}
	}
	return nil
}

// materialReviewRisks derives the honest review annotations of a body:
// placeholders for missing items and claims flagged for human review.
func materialReviewRisks(body MaterialBody) []MaterialReviewRisk {
	risks := []MaterialReviewRisk{}
	for _, section := range body.Sections {
		for _, claim := range section.Claims {
			if !claim.NeedsReview {
				continue
			}
			risk := MaterialReviewRisk{ClaimID: claim.ClaimID}
			if claim.FactKey == "" {
				risk.Code = MaterialRiskMissingPlaceholder
				risk.Message = "缺失或待补充信息占位（" + claim.Text + "）：不得由系统补造"
			} else {
				risk.Code = MaterialRiskNeedsReview
				risk.Message = "该主张被标记需人工审阅"
			}
			if claim.ReviewNote != "" {
				risk.Message = risk.Message + "：" + claim.ReviewNote
			}
			risks = append(risks, risk)
		}
	}
	return risks
}

// diffMaterialBodies produces the honest baseline→target diff by heading and
// claim identity.
func diffMaterialBodies(baseline, target MaterialBody) []MaterialVersionChange {
	changes := []MaterialVersionChange{}
	baselineSections := map[string]MaterialSection{}
	for _, section := range baseline.Sections {
		baselineSections[section.Heading] = section
	}
	targetSections := map[string]MaterialSection{}
	for _, section := range target.Sections {
		targetSections[section.Heading] = section
	}
	for _, section := range baseline.Sections {
		if _, ok := targetSections[section.Heading]; !ok {
			changes = append(changes, MaterialVersionChange{Kind: MaterialChangeSectionRemoved, Heading: section.Heading, Baseline: section.Content})
		}
	}
	for _, section := range target.Sections {
		prior, ok := baselineSections[section.Heading]
		if !ok {
			changes = append(changes, MaterialVersionChange{Kind: MaterialChangeSectionAdded, Heading: section.Heading, Target: section.Content})
			continue
		}
		if prior.Content != section.Content {
			changes = append(changes, MaterialVersionChange{Kind: MaterialChangeSectionChanged, Heading: section.Heading, Baseline: prior.Content, Target: section.Content})
		}
		changes = append(changes, diffMaterialClaims(section.Heading, prior.Claims, section.Claims)...)
	}
	return changes
}

func diffMaterialClaims(heading string, baseline, target []MaterialClaim) []MaterialVersionChange {
	changes := []MaterialVersionChange{}
	index := func(claims []MaterialClaim) map[string]MaterialClaim {
		out := make(map[string]MaterialClaim, len(claims))
		for _, claim := range claims {
			out[claim.ClaimID] = claim
		}
		return out
	}
	baselineClaims, targetClaims := index(baseline), index(target)
	for _, claim := range baseline {
		if _, ok := targetClaims[claim.ClaimID]; !ok {
			changes = append(changes, MaterialVersionChange{Kind: MaterialChangeClaimRemoved, Heading: heading, ClaimID: claim.ClaimID, Baseline: claim.Text})
		}
	}
	for _, claim := range target {
		prior, ok := baselineClaims[claim.ClaimID]
		if !ok {
			changes = append(changes, MaterialVersionChange{Kind: MaterialChangeClaimAdded, Heading: heading, ClaimID: claim.ClaimID, Target: claim.Text})
			continue
		}
		if prior.Text != claim.Text || prior.FactKey != claim.FactKey || prior.NeedsReview != claim.NeedsReview {
			changes = append(changes, MaterialVersionChange{Kind: MaterialChangeClaimChanged, Heading: heading, ClaimID: claim.ClaimID, Baseline: prior.Text, Target: claim.Text})
		}
	}
	return changes
}

func buildMaterialReceipt(kind, requestID, materialID, status string, version uint64, pin MaterialEvidencePin, body MaterialBody) (MaterialReceipt, error) {
	receipt := MaterialReceipt{
		Kind:           kind,
		RequestID:      requestID,
		MaterialID:     materialID,
		Status:         status,
		Version:        version,
		PinnedEvidence: pin,
		Body:           body,
		ReviewRisks:    materialReviewRisks(body),
	}
	if _, err := json.Marshal(receipt); err != nil {
		return MaterialReceipt{}, err
	}
	return receipt, nil
}

func insertMaterialRow(tx *gorm.DB, s Scope, receipt MaterialReceipt, fingerprint string, pin MaterialEvidencePin, input EditMaterialInput, now time.Time) error {
	return tx.Create(&materialRecord{
		ID: receipt.MaterialID, TenantID: s.TenantID, UserID: s.UserID,
		RequestID: input.RequestID, Fingerprint: fingerprint,
		OpportunityID: pin.OpportunityID, SnapshotID: pin.SnapshotID,
		ProfileRevision: pin.ProfileRevision,
		EvidenceBody:    string(mustJSON(pin)),
		Status:          MaterialStatusDraft,
		DraftBody:       string(mustJSON(receipt.Body)),
		ReceiptBody:     string(mustJSON(receipt)),
		CreatedAt:       now, UpdatedAt: now,
	}).Error
}

func materialFingerprint(kind string, parts ...any) (string, error) {
	intent := append([]any{kind}, parts...)
	intentBytes, err := json.Marshal(intent)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(intentBytes)
	return hex.EncodeToString(sum[:]), nil
}

func decodeMaterialReceipt(body string, receipt *MaterialReceipt) error {
	if err := json.Unmarshal([]byte(body), receipt); err != nil {
		return fmt.Errorf("decode career material receipt: %w", err)
	}
	return nil
}

func decodeMaterialVersion(body string) (MaterialVersionView, error) {
	var view MaterialVersionView
	if err := json.Unmarshal([]byte(body), &view); err != nil {
		return MaterialVersionView{}, fmt.Errorf("decode career material version: %w", err)
	}
	return view, nil
}

func mustJSON(value any) []byte {
	encoded, err := json.Marshal(value)
	if err != nil {
		return []byte("{}")
	}
	return encoded
}
