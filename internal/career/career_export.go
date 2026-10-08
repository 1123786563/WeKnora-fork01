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

	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// T22 closed intent set for whole-space lifecycle operations.
const (
	CareerKindExported = "career_exported"
	CareerKindDeleted  = "career_deleted"

	ChangeKindCareerDeleted = "career_deleted"

	CareerExportStatusComplete = "complete"

	DeletionStatusDeleting = "deleting"
	DeletionStatusPartial  = "partial"
	DeletionStatusDeleted  = "deleted"

	DeletionStepStatusPending = "pending"
	DeletionStepStatusDone    = "done"
	DeletionStepStatusFailed  = "failed"

	DeletionStepRevokeExports     = "revoke_material_exports"
	DeletionStepPurgeCareerData   = "purge_career_data"
	DeletionStepRemoveProjections = "remove_workbench_tasks"
	DeletionStepFinalize          = "finalize"

	CareerRetentionStatusRetained = "retained"
)

var ErrDeletionNotFound = errors.New("career deletion receipt not found")

// careerDataExportRecord is one complete export package: the frozen archive
// payload, its verifiable digest, and the request-ID ledger row that makes
// the export replayable.
type careerDataExportRecord struct {
	ID          string    `gorm:"primaryKey;size:36"`
	TenantID    uint64    `gorm:"uniqueIndex:career_data_export_scope_request;index:idx_career_data_export_scope"`
	UserID      string    `gorm:"uniqueIndex:career_data_export_scope_request;index:idx_career_data_export_scope;size:512"`
	RequestID   string    `gorm:"size:128;uniqueIndex:career_data_export_scope_request"`
	Fingerprint string    `gorm:"size:64;not null"`
	Revision    uint64    `gorm:"not null"`
	Digest      string    `gorm:"size:64;not null"`
	ArchiveBody string    `gorm:"type:text;not null"`
	ReceiptBody string    `gorm:"type:text;not null"`
	CreatedAt   time.Time `gorm:"not null"`
}

func (careerDataExportRecord) TableName() string { return "career_data_exports" }

// careerDataDeletionRecord is the deletion state machine: the durable,
// recoverable audit row. It survives the deletion it describes (disclosed as
// a retained row) so partial failures can be resumed and replays stay
// truthful about what was and was not removed.
type careerDataDeletionRecord struct {
	ID               string    `gorm:"primaryKey;size:36"`
	TenantID         uint64    `gorm:"uniqueIndex:career_data_deletion_scope_request;index:idx_career_data_deletion_scope"`
	UserID           string    `gorm:"uniqueIndex:career_data_deletion_scope_request;index:idx_career_data_deletion_scope;size:512"`
	RequestID        string    `gorm:"size:128;uniqueIndex:career_data_deletion_scope_request"`
	Fingerprint      string    `gorm:"size:64;not null"`
	ExpectedRevision uint64    `gorm:"not null"`
	Status           string    `gorm:"size:16;not null"`
	StateBody        string    `gorm:"type:text;not null"`
	ReceiptBody      string    `gorm:"type:text;not null"`
	CreatedAt        time.Time `gorm:"not null"`
	UpdatedAt        time.Time `gorm:"not null"`
}

func (careerDataDeletionRecord) TableName() string { return "career_data_deletions" }

// SetApplicationTaskRemover binds the Workbench complete-deletion seam. Like
// the linker, it is the only channel through which Career may remove durable
// Workbench projections.
func (o *Office) SetApplicationTaskRemover(remover interfaces.CareerApplicationTaskProjectionRemover) {
	o.applicationTaskRemover = remover
}

// careerSourceUploadReleaser releases one uploaded source's catalog binding
// and physical object. The UploadAdapter implements it; keeping it narrow
// lets deletion tests run without the file backend.
type careerSourceUploadReleaser interface {
	Release(ctx context.Context, reference, sourceID string) error
}

// SetSourceUploadReleaser binds the uploaded-source release seam used by the
// purge step so uploaded originals do not outlive their locator rows.
func (o *Office) SetSourceUploadReleaser(releaser careerSourceUploadReleaser) {
	o.sourceUploadReleaser = releaser
}

// ---- Export (export_career) ----

// CareerExportInput is the closed export intent.
type CareerExportInput struct {
	RequestID        string `json:"requestId"`
	ExpectedRevision uint64 `json:"expectedRevision"`
}

// CareerExportSnapshot is one frozen job posting snapshot.
type CareerExportSnapshot struct {
	SnapshotID string    `json:"snapshotId"`
	Status     string    `json:"status"`
	RawText    string    `json:"rawText"`
	AcquiredAt time.Time `json:"acquiredAt"`
}

// CareerExportOpportunity groups one opportunity with its snapshots.
type CareerExportOpportunity struct {
	OpportunityID string                 `json:"opportunityId"`
	Snapshots     []CareerExportSnapshot `json:"snapshots"`
}

// CareerExportProgressEvent is one immutable application progress event.
type CareerExportProgressEvent struct {
	EventID         string    `json:"eventId"`
	ApplicationID   string    `json:"applicationId"`
	Seq             uint64    `json:"seq"`
	EventType       string    `json:"eventType"`
	Note            string    `json:"note,omitempty"`
	OccurredAt      time.Time `json:"occurredAt"`
	CorrectsEventID string    `json:"correctsEventId,omitempty"`
	Source          Source    `json:"source"`
	Confirmer       string    `json:"confirmer"`
}

// CareerExportApplication groups one application with its progress events.
type CareerExportApplication struct {
	ApplicationID  string                      `json:"applicationId"`
	OpportunityID  string                      `json:"opportunityId"`
	SnapshotID     string                      `json:"snapshotId"`
	BatchIdentity  string                      `json:"batchIdentity"`
	TaskID         string                      `json:"taskId,omitempty"`
	ProgressEvents []CareerExportProgressEvent `json:"progressEvents"`
}

// CareerExportMaterialVersion is one immutable confirmed material version.
type CareerExportMaterialVersion struct {
	Version     uint64    `json:"version"`
	RequestID   string    `json:"requestId,omitempty"`
	VersionBody string    `json:"versionBody"`
	CreatedAt   time.Time `json:"createdAt"`
}

// CareerExportMaterial groups one material with its version history.
type CareerExportMaterial struct {
	MaterialID    string                        `json:"materialId"`
	OpportunityID string                        `json:"opportunityId"`
	Status        string                        `json:"status"`
	Versions      []CareerExportMaterialVersion `json:"versions"`
}

// CareerExportSubmission is one user-confirmed submission record.
type CareerExportSubmission struct {
	SubmissionID     string    `json:"submissionId"`
	ApplicationID    string    `json:"applicationId"`
	Channel          string    `json:"channel"`
	OccurredAt       time.Time `json:"occurredAt"`
	VersionConfirmed bool      `json:"versionConfirmed"`
	MaterialID       string    `json:"materialId,omitempty"`
	ExportID         string    `json:"exportId,omitempty"`
	Version          uint64    `json:"version,omitempty"`
	ContentDigest    string    `json:"contentDigest,omitempty"`
	Note             string    `json:"note,omitempty"`
	Confirmer        string    `json:"confirmer"`
	CreatedAt        time.Time `json:"createdAt"`
}

// CareerExportPreparation is one preparation draft receipt record.
type CareerExportPreparation struct {
	PreparationID string             `json:"preparationId"`
	ApplicationID string             `json:"applicationId"`
	RequestID     string             `json:"requestId"`
	Focus         string             `json:"focus"`
	Status        string             `json:"status"`
	FailureCode   string             `json:"failureCode,omitempty"`
	CreatedAt     time.Time          `json:"createdAt"`
	Receipt       PreparationReceipt `json:"receipt"`
}

// CareerExportSearchRule is one periodic search rule.
type CareerExportSearchRule struct {
	RuleID          string     `json:"ruleId"`
	Query           string     `json:"query"`
	IntervalMinutes uint64     `json:"intervalMinutes"`
	Status          string     `json:"status"`
	Revision        uint64     `json:"revision"`
	LastPeriod      uint64     `json:"lastPeriod"`
	NextDueAt       *time.Time `json:"nextDueAt,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
}

// CareerExportReminder is one in-space reminder row.
type CareerExportReminder struct {
	ReminderID    string    `json:"reminderId"`
	SourceKind    string    `json:"sourceKind"`
	SourceID      string    `json:"sourceId"`
	ApplicationID string    `json:"applicationId,omitempty"`
	OpportunityID string    `json:"opportunityId,omitempty"`
	NoticeKey     string    `json:"noticeKey"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"createdAt"`
}

// CareerExportArchive is the frozen structure of one complete export
// package: profile, original job snapshots, application events, material
// versions, submission records, preparation drafts, periodic search rules,
// and reminders. Deletion purges every one of these sections, so the export
// must carry all of them or the data would be destroyed unrecoverably.
type CareerExportArchive struct {
	Profile       View                      `json:"profile"`
	FactHistory   []Fact                    `json:"factHistory"`
	Opportunities []CareerExportOpportunity `json:"opportunities"`
	Applications  []CareerExportApplication `json:"applications"`
	Materials     []CareerExportMaterial    `json:"materials"`
	Submissions   []CareerExportSubmission  `json:"submissions"`
	Preparations  []CareerExportPreparation `json:"preparations"`
	SearchRules   []CareerExportSearchRule  `json:"searchRules"`
	Reminders     []CareerExportReminder    `json:"reminders"`
}

// CareerExportReceipt is the export receipt: the archive travels inline
// (synchronous export — house decision frozen here) with a sha256 digest the
// client can verify.
type CareerExportReceipt struct {
	Kind      string              `json:"kind"`
	RequestID string              `json:"requestId"`
	ExportID  string              `json:"exportId"`
	Revision  uint64              `json:"revision"`
	Status    string              `json:"status"`
	Digest    string              `json:"digest"`
	Archive   CareerExportArchive `json:"archive"`
	CreatedAt time.Time           `json:"createdAt"`
}

func careerExportFingerprint(requestID string, expectedRevision uint64) (string, error) {
	body, err := json.Marshal([]any{"export_career", requestID, expectedRevision})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}

// ExportCareer produces one complete, owner-scoped export package of the
// whole Career space. The export is synchronous and read-only: it never
// bumps the profile revision, but it does require the expected revision so a
// client can pin the snapshot it meant to export. One request ID replays the
// stored receipt; a changed intent under the same request ID is rejected.
func (o *Office) ExportCareer(ctx context.Context, input CareerExportInput) (CareerExportReceipt, error) {
	s, err := getScope(ctx)
	if err != nil {
		return CareerExportReceipt{}, err
	}
	if err = o.requireSpace(ctx, s); err != nil {
		return CareerExportReceipt{}, err
	}
	input.RequestID = trimRequestID(input.RequestID)
	if input.RequestID == "" {
		return CareerExportReceipt{}, ErrInvalidRequest
	}
	fingerprint, err := careerExportFingerprint(input.RequestID, input.ExpectedRevision)
	if err != nil {
		return CareerExportReceipt{}, err
	}
	var stored careerDataExportRecord
	err = o.db.WithContext(ctx).
		Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, input.RequestID).
		First(&stored).Error
	if err == nil {
		if stored.Fingerprint != fingerprint {
			return CareerExportReceipt{}, ErrIdempotencyConflict
		}
		return decodeCareerExportReceipt(stored.ReceiptBody)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return CareerExportReceipt{}, err
	}

	var receipt CareerExportReceipt
	err = o.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if e := requireGateActiveTx(tx, s); e != nil {
			return e
		}
		var head profile
		e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tenant_id=? AND user_id=?", s.TenantID, s.UserID).First(&head).Error
		if errors.Is(e, gorm.ErrRecordNotFound) {
			// A space that was only opened (or only imported sources) may
			// carry no profile row yet: the epoch reads as revision 0, the
			// same tolerant read evaluation and material already apply.
			head.Revision = 0
		} else if e != nil {
			return e
		}
		if head.Revision != input.ExpectedRevision {
			return &RevisionConflictError{CurrentRevision: head.Revision}
		}
		archive, e := buildCareerExportArchive(tx, s)
		if e != nil {
			return e
		}
		payload, e := json.Marshal(archive)
		if e != nil {
			return e
		}
		sum := sha256.Sum256(payload)
		receipt = CareerExportReceipt{
			Kind:      CareerKindExported,
			RequestID: input.RequestID,
			ExportID:  uuid.NewString(),
			Revision:  head.Revision,
			Status:    CareerExportStatusComplete,
			Digest:    hex.EncodeToString(sum[:]),
			Archive:   archive,
			CreatedAt: time.Now().UTC(),
		}
		return tx.Create(&careerDataExportRecord{
			ID: receipt.ExportID, TenantID: s.TenantID, UserID: s.UserID,
			RequestID: input.RequestID, Fingerprint: fingerprint,
			Revision: head.Revision, Digest: receipt.Digest,
			ArchiveBody: string(payload), ReceiptBody: string(mustJSON(receipt)),
			CreatedAt: receipt.CreatedAt,
		}).Error
	})
	if err != nil {
		if isReceiptRaceError(err) {
			var raced careerDataExportRecord
			if e := o.db.WithContext(ctx).
				Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, input.RequestID).
				First(&raced).Error; e == nil {
				if raced.Fingerprint != fingerprint {
					return CareerExportReceipt{}, ErrIdempotencyConflict
				}
				return decodeCareerExportReceipt(raced.ReceiptBody)
			}
		}
		return CareerExportReceipt{}, err
	}
	return receipt, nil
}

// FindCareerExport replays one export receipt by its original request ID.
func (o *Office) FindCareerExport(ctx context.Context, requestID string) (CareerExportReceipt, error) {
	s, err := getScope(ctx)
	if err != nil {
		return CareerExportReceipt{}, err
	}
	if err = o.requireSpace(ctx, s); err != nil {
		return CareerExportReceipt{}, err
	}
	requestID = trimRequestID(requestID)
	if requestID == "" {
		return CareerExportReceipt{}, ErrInvalidRequest
	}
	var stored careerDataExportRecord
	err = o.db.WithContext(ctx).
		Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, requestID).
		First(&stored).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return CareerExportReceipt{}, ErrReceiptNotFound
	}
	if err != nil {
		return CareerExportReceipt{}, err
	}
	return decodeCareerExportReceipt(stored.ReceiptBody)
}

func decodeCareerExportReceipt(body string) (CareerExportReceipt, error) {
	var receipt CareerExportReceipt
	if err := json.Unmarshal([]byte(body), &receipt); err != nil {
		return CareerExportReceipt{}, fmt.Errorf("decode career export receipt: %w", err)
	}
	return receipt, nil
}

func buildCareerExportArchive(tx *gorm.DB, s Scope) (CareerExportArchive, error) {
	archive := CareerExportArchive{
		Profile:       View{Facts: []Fact{}, Proposals: []Proposal{}},
		FactHistory:   []Fact{},
		Opportunities: []CareerExportOpportunity{},
		Applications:  []CareerExportApplication{},
		Materials:     []CareerExportMaterial{},
		Submissions:   []CareerExportSubmission{},
		Preparations:  []CareerExportPreparation{},
		SearchRules:   []CareerExportSearchRule{},
		Reminders:     []CareerExportReminder{},
	}
	var head profile
	if err := tx.Where("tenant_id=? AND user_id=?", s.TenantID, s.UserID).First(&head).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return archive, err
		}
		head.Revision = 0
	}
	archive.Profile.Revision = head.Revision

	var facts []fact
	if err := tx.Where("tenant_id=? AND user_id=?", s.TenantID, s.UserID).Order("key").Find(&facts).Error; err != nil {
		return archive, err
	}
	for _, f := range facts {
		c := decodeConfirmation(f.Confirmation)
		archive.Profile.Facts = append(archive.Profile.Facts, Fact{f.Key, f.Value, f.Revision, decodeSource(f.Source), c, c.ConfirmedAt})
	}
	var proposals []proposal
	if err := tx.Where("tenant_id=? AND user_id=? AND status='pending'", s.TenantID, s.UserID).Order("created_at,id").Find(&proposals).Error; err != nil {
		return archive, err
	}
	for _, p := range proposals {
		archive.Profile.Proposals = append(archive.Profile.Proposals, Proposal{
			ID: p.PublicID, Key: p.Key, Value: p.Value, Evidence: p.Evidence,
			Source: decodeSource(p.Source), Status: p.Status, CreatedAt: p.CreatedAt,
		})
	}
	var versions []factVersion
	if err := tx.Where("tenant_id=? AND user_id=?", s.TenantID, s.UserID).Order("key,revision").Find(&versions).Error; err != nil {
		return archive, err
	}
	for _, r := range versions {
		c := decodeConfirmation(r.Confirmation)
		archive.FactHistory = append(archive.FactHistory, Fact{r.Key, r.Value, r.Revision, decodeSource(r.Source), c, c.ConfirmedAt})
	}

	var opportunities []opportunity
	if err := tx.Where("tenant_id=? AND user_id=?", s.TenantID, s.UserID).Order("created_at,id").Find(&opportunities).Error; err != nil {
		return archive, err
	}
	for _, opp := range opportunities {
		var snapshots []opportunitySnapshot
		if err := tx.Where("tenant_id=? AND user_id=? AND opportunity_id=?", s.TenantID, s.UserID, opp.ID).
			Order("created_at,id").Find(&snapshots).Error; err != nil {
			return archive, err
		}
		exported := CareerExportOpportunity{OpportunityID: opp.ID, Snapshots: []CareerExportSnapshot{}}
		for _, snap := range snapshots {
			exported.Snapshots = append(exported.Snapshots, CareerExportSnapshot{
				SnapshotID: snap.ID, Status: snap.Status, RawText: snap.RawText, AcquiredAt: snap.AcquiredAt,
			})
		}
		archive.Opportunities = append(archive.Opportunities, exported)
	}

	var applications []applicationRecord
	if err := tx.Where("tenant_id=? AND user_id=?", s.TenantID, s.UserID).Order("created_at,id").Find(&applications).Error; err != nil {
		return archive, err
	}
	for _, app := range applications {
		var events []progressEventRecord
		if err := tx.Where("tenant_id=? AND user_id=? AND application_id=?", s.TenantID, s.UserID, app.ID).
			Order("seq").Find(&events).Error; err != nil {
			return archive, err
		}
		exported := CareerExportApplication{
			ApplicationID: app.ID, OpportunityID: app.OpportunityID, SnapshotID: app.SnapshotID,
			BatchIdentity: app.BatchIdentity, TaskID: app.TaskID, ProgressEvents: []CareerExportProgressEvent{},
		}
		for _, event := range events {
			exported.ProgressEvents = append(exported.ProgressEvents, CareerExportProgressEvent{
				EventID: event.ID, ApplicationID: event.ApplicationID, Seq: event.Seq,
				EventType: event.EventType, Note: event.Note, OccurredAt: event.OccurredAt,
				CorrectsEventID: event.CorrectsEventID, Source: decodeSource(event.Source),
				Confirmer: event.Confirmer,
			})
		}
		archive.Applications = append(archive.Applications, exported)
	}

	var materials []materialRecord
	if err := tx.Where("tenant_id=? AND user_id=?", s.TenantID, s.UserID).Order("created_at,id").Find(&materials).Error; err != nil {
		return archive, err
	}
	for _, mat := range materials {
		var materialVersions []materialVersionRecord
		if err := tx.Where("tenant_id=? AND user_id=? AND material_id=?", s.TenantID, s.UserID, mat.ID).
			Order("version").Find(&materialVersions).Error; err != nil {
			return archive, err
		}
		exported := CareerExportMaterial{
			MaterialID: mat.ID, OpportunityID: mat.OpportunityID, Status: mat.Status, Versions: []CareerExportMaterialVersion{},
		}
		for _, version := range materialVersions {
			exported.Versions = append(exported.Versions, CareerExportMaterialVersion{
				Version: version.Version, RequestID: version.RequestID,
				VersionBody: version.VersionBody, CreatedAt: version.CreatedAt,
			})
		}
		archive.Materials = append(archive.Materials, exported)
	}

	var submissions []submissionRecord
	if err := tx.Where("tenant_id=? AND user_id=?", s.TenantID, s.UserID).Order("created_at,id").Find(&submissions).Error; err != nil {
		return archive, err
	}
	for _, sub := range submissions {
		archive.Submissions = append(archive.Submissions, CareerExportSubmission{
			SubmissionID: sub.ID, ApplicationID: sub.ApplicationID, Channel: sub.Channel,
			OccurredAt: sub.OccurredAt, VersionConfirmed: sub.VersionConfirmed,
			MaterialID: sub.MaterialID, ExportID: sub.ExportID, Version: sub.Version,
			ContentDigest: sub.ContentDigest, Note: sub.Note, Confirmer: sub.Confirmer,
			CreatedAt: sub.CreatedAt,
		})
	}

	var preparations []preparationRecord
	if err := tx.Where("tenant_id=? AND user_id=?", s.TenantID, s.UserID).Order("created_at,id").Find(&preparations).Error; err != nil {
		return archive, err
	}
	for _, prep := range preparations {
		var preparationReceipt PreparationReceipt
		if err := decodePreparationReceipt(prep.ReceiptBody, &preparationReceipt); err != nil {
			return archive, err
		}
		archive.Preparations = append(archive.Preparations, CareerExportPreparation{
			PreparationID: prep.ID, ApplicationID: prep.ApplicationID, RequestID: prep.RequestID,
			Focus: prep.Focus, Status: prep.Status, FailureCode: prep.FailureCode,
			CreatedAt: prep.CreatedAt, Receipt: preparationReceipt,
		})
	}

	var searchRules []searchRuleRecord
	if err := tx.Where("tenant_id=? AND user_id=?", s.TenantID, s.UserID).Order("created_at,id").Find(&searchRules).Error; err != nil {
		return archive, err
	}
	for _, rule := range searchRules {
		archive.SearchRules = append(archive.SearchRules, CareerExportSearchRule{
			RuleID: rule.ID, Query: rule.Query, IntervalMinutes: rule.IntervalMinutes,
			Status: rule.Status, Revision: rule.Revision, LastPeriod: rule.LastPeriod,
			NextDueAt: rule.NextDueAt, CreatedAt: rule.CreatedAt,
		})
	}

	var reminders []reminderRecord
	if err := tx.Where("tenant_id=? AND user_id=?", s.TenantID, s.UserID).Order("created_at,id").Find(&reminders).Error; err != nil {
		return archive, err
	}
	for _, rem := range reminders {
		archive.Reminders = append(archive.Reminders, CareerExportReminder{
			ReminderID: rem.ID, SourceKind: rem.SourceKind, SourceID: rem.SourceID,
			ApplicationID: rem.ApplicationID, OpportunityID: rem.OpportunityID,
			NoticeKey: rem.NoticeKey, Status: rem.Status, CreatedAt: rem.CreatedAt,
		})
	}
	return archive, nil
}

// ---- Deletion boundary (pre-deletion explanation) ----

// CareerDeletionSection is one in-space data section the system can delete.
type CareerDeletionSection struct {
	Section     string `json:"section"`
	Description string `json:"description"`
	Count       int    `json:"count"`
}

// CareerExternalBoundaryItem describes data outside this space's control.
type CareerExternalBoundaryItem struct {
	Item        string `json:"item"`
	Description string `json:"description"`
	Revocable   bool   `json:"revocable"`
}

// CareerRetentionItem discloses one retained row class with its reason.
type CareerRetentionItem struct {
	Holder string `json:"holder"`
	Reason string `json:"reason"`
	Status string `json:"status"`
}

// CareerDeletionBoundaryView is the structured pre-deletion explanation:
// what the system will delete in-space, what it can never touch externally,
// and which rows are retained with reasons. The API returns this before any
// deletion executes; the frontend renders it as the confirmation surface.
type CareerDeletionBoundaryView struct {
	InSpace   []CareerDeletionSection      `json:"inSpace"`
	External  []CareerExternalBoundaryItem `json:"external"`
	Retention []CareerRetentionItem        `json:"retention"`
}

func careerDeletionRetention() []CareerRetentionItem {
	return []CareerRetentionItem{
		{Holder: "career_data_deletions", Reason: "删除审计与可恢复状态（法定/技术保留）", Status: CareerRetentionStatusRetained},
		{Holder: "career_changes", Reason: "仅保留 deletion 事件以驱动客户端缓存失效", Status: CareerRetentionStatusRetained},
		{Holder: "career_profiles", Reason: "仅保留 revision 计数器（空间 epoch 语义，不含个人数据）", Status: CareerRetentionStatusRetained},
		{Holder: "career_spaces", Reason: "空间归属与授权校验（不含个人数据）", Status: CareerRetentionStatusRetained},
	}
}

func careerDeletionExternalBoundary() []CareerExternalBoundaryItem {
	return []CareerExternalBoundaryItem{
		{
			Item:        "external_platform_submissions",
			Description: "你在外部招聘平台完成的投递、沟通与账号操作不在本空间控制范围内，本系统无法撤回或修改。",
			Revocable:   false,
		},
		{
			Item:        "external_email_copies",
			Description: "已通过邮件或其他渠道发往外部的简历与材料副本无法由本系统收回。",
			Revocable:   false,
		},
	}
}

// CareerDeletionBoundary explains, before any deletion, exactly what will be
// deleted in-space, what cannot be reached externally, and what is retained.
// It is a read-only projection: it never mutates data.
func (o *Office) CareerDeletionBoundary(ctx context.Context) (CareerDeletionBoundaryView, error) {
	s, err := getScope(ctx)
	if err != nil {
		return CareerDeletionBoundaryView{}, err
	}
	if err = o.requireSpace(ctx, s); err != nil {
		return CareerDeletionBoundaryView{}, err
	}
	var countErr error
	sectionCount := func(table, extra string, args ...any) int {
		var total int64
		query := o.db.WithContext(ctx).Table(table).Where("tenant_id=? AND user_id=?", s.TenantID, s.UserID)
		if extra != "" {
			query = query.Where(extra, args...)
		}
		if err := query.Count(&total).Error; err != nil {
			if countErr == nil {
				countErr = fmt.Errorf("count career deletion boundary %s: %w", table, err)
			}
			return 0
		}
		return int(total)
	}
	view := CareerDeletionBoundaryView{
		InSpace: []CareerDeletionSection{
			{Section: "profile", Description: "已确认的档案事实与待处理提案", Count: sectionCount("career_facts", "")},
			{Section: "fact_history", Description: "档案事实的版本历史", Count: sectionCount("career_fact_versions", "")},
			{Section: "opportunities", Description: "岗位记录及其原始快照", Count: sectionCount("career_opportunities", "")},
			{Section: "applications", Description: "申请意向与证据", Count: sectionCount("career_applications", "")},
			{Section: "progress_events", Description: "申请进展事件", Count: sectionCount("career_progress_events", "")},
			{Section: "materials", Description: "材料草稿", Count: sectionCount("career_materials", "")},
			{Section: "material_versions", Description: "材料的不可变版本", Count: sectionCount("career_material_versions", "")},
			{Section: "material_exports", Description: "材料导出与下载授权", Count: sectionCount("career_material_exports", "")},
			{Section: "submissions", Description: "你确认的投递记录", Count: sectionCount("career_submissions", "")},
			{Section: "preparations", Description: "投递准备稿（随完整导出携带后删除）", Count: sectionCount("career_preparations", "")},
			{Section: "searches", Description: "一次性搜索记录及其搜索结果", Count: sectionCount("career_searches", "")},
			{Section: "search_rules", Description: "周期搜索规则（运行与发现待办不随导出携带；删除后不可恢复）", Count: sectionCount("career_search_rules", "")},
			{Section: "reminders", Description: "站内待办与提醒回执（推送仅为提醒渠道，不含公司、岗位或面试细节；随完整导出携带后删除）", Count: sectionCount("career_reminders", "")},
			{Section: "usage_reservations", Description: "搜索额度预占与结算记录（额度账本，删除后随空间一并清空）", Count: sectionCount("career_usage_reservations", "")},
			{Section: "reconciliations", Description: "岗位去重与合并的决策记录（含合并证据，随删除一并清除）", Count: sectionCount("career_reconciliations", "")},
			{Section: "evaluations", Description: "岗位资格评估结果与固定证据（资格判断历史，随删除一并清除）", Count: sectionCount("career_evaluations", "")},
			{Section: "source_revisions", Description: "导入的简历/JD 原件记录（上传原件随删除一并物理释放）", Count: sectionCount("career_source_revisions", "")},
			{Section: "data_exports", Description: "整空间导出归档（冻结的导出载荷与回执，随删除一并清除）", Count: sectionCount("career_data_exports", "")},
			{Section: "receipts", Description: "各操作幂等回执账本（回执正文随删除一并清除）", Count: sectionCount("career_receipts", "") + sectionCount("career_opportunity_receipts", "") + sectionCount("career_material_receipts", "") + sectionCount("career_search_rule_receipts", "") + sectionCount("career_reminder_receipts", "")},
			{Section: "workbench_tasks", Description: "Workbench 侧申请任务投影（经删除端口移除）", Count: sectionCount("career_applications", "task_id <> ''")},
		},
		External:  careerDeletionExternalBoundary(),
		Retention: careerDeletionRetention(),
	}
	if countErr != nil {
		return CareerDeletionBoundaryView{}, countErr
	}
	return view, nil
}

// ---- Deletion (delete_career) ----

// CareerDeletionInput is the closed deletion intent.
type CareerDeletionInput struct {
	RequestID        string `json:"requestId"`
	ExpectedRevision uint64 `json:"expectedRevision"`
}

// CareerDeletionStep reports one sub-deletion's durable status.
type CareerDeletionStep struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

// CareerDeletionReceipt is the truthful deletion outcome: it only reports
// status "deleted" after every step succeeded. Partial failures keep a
// recoverable state that the same request ID can resume.
type CareerDeletionReceipt struct {
	Kind        string                `json:"kind"`
	RequestID   string                `json:"requestId"`
	Status      string                `json:"status"`
	Steps       []CareerDeletionStep  `json:"steps"`
	Retention   []CareerRetentionItem `json:"retention"`
	Revision    uint64                `json:"revision"`
	StartedAt   time.Time             `json:"startedAt"`
	CompletedAt *time.Time            `json:"completedAt,omitempty"`
}

type deletionExecution struct {
	Steps        []CareerDeletionStep
	StepIndex    map[string]int
	NextRevision uint64
}

func careerDeletionFingerprint(requestID string, expectedRevision uint64) (string, error) {
	body, err := json.Marshal([]any{"delete_career", requestID, expectedRevision})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}

var careerPurgeTables = []string{
	"career_facts",
	"career_fact_versions",
	"career_proposals",
	"career_receipts",
	"career_source_revisions",
	"career_opportunities",
	"career_opportunity_observations",
	"career_opportunity_snapshots",
	"career_opportunity_receipts",
	"career_evaluations",
	"career_applications",
	"career_searches",
	"career_search_results",
	"career_materials",
	"career_material_versions",
	"career_material_receipts",
	"career_material_exports",
	"career_progress_events",
	"career_search_rules",
	"career_search_rule_receipts",
	"career_search_rule_runs",
	"career_search_discovery_todos",
	"career_submissions",
	"career_preparations",
	"career_reminders",
	"career_reminder_receipts",
	"career_usage_reservations",
	"career_reconciliations",
	"career_data_exports",
}

// DeleteCareer completely deletes the owner's Career space: Career domain
// data, the Workbench application-task projections (through the remover
// port), and every previously issued artifact/export grant. The changes
// stream receives one career_deleted event so client caches invalidate. Any
// failing step leaves a partial, recoverable state under the same request
// ID — the receipt never claims full deletion until every step is done.
func (o *Office) DeleteCareer(ctx context.Context, input CareerDeletionInput) (CareerDeletionReceipt, error) {
	o.lifecycleMu.Lock()
	defer o.lifecycleMu.Unlock()
	s, err := getScope(ctx)
	if err != nil {
		return CareerDeletionReceipt{}, err
	}
	if err = o.requireSpace(ctx, s); err != nil {
		return CareerDeletionReceipt{}, err
	}
	input.RequestID = trimRequestID(input.RequestID)
	if input.RequestID == "" {
		return CareerDeletionReceipt{}, ErrInvalidRequest
	}
	fingerprint, err := careerDeletionFingerprint(input.RequestID, input.ExpectedRevision)
	if err != nil {
		return CareerDeletionReceipt{}, err
	}
	// Close admission before creating or resuming the deletion state machine.
	// An unresolved claim remains durable and retryable under its original ID.
	if err = o.beginLifecycleDeletion(ctx, s, input.RequestID, fingerprint); err != nil {
		return CareerDeletionReceipt{}, err
	}

	now := time.Now().UTC()
	var record careerDataDeletionRecord
	err = o.db.WithContext(ctx).
		Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, input.RequestID).
		First(&record).Error
	if err == nil {
		if record.Fingerprint != fingerprint {
			return CareerDeletionReceipt{}, ErrIdempotencyConflict
		}
		if record.Status == DeletionStatusDeleted {
			return decodeCareerDeletionReceipt(record.ReceiptBody)
		}
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		var head profile
		if e := o.db.WithContext(ctx).
			Where("tenant_id=? AND user_id=?", s.TenantID, s.UserID).First(&head).Error; e != nil {
			if !errors.Is(e, gorm.ErrRecordNotFound) {
				return CareerDeletionReceipt{}, e
			}
			// No profile row yet (the space was only opened or only imported
			// sources): the epoch reads as revision 0. Deletion must still
			// proceed — data sovereignty cannot require a written profile.
			head.Revision = 0
		}
		if head.Revision != input.ExpectedRevision {
			return CareerDeletionReceipt{}, &RevisionConflictError{CurrentRevision: head.Revision}
		}
		execution := newDeletionExecution()
		record = careerDataDeletionRecord{
			ID: uuid.NewString(), TenantID: s.TenantID, UserID: s.UserID,
			RequestID: input.RequestID, Fingerprint: fingerprint,
			ExpectedRevision: input.ExpectedRevision, Status: DeletionStatusDeleting,
			StateBody: string(mustJSON(execution)), ReceiptBody: "{}",
			CreatedAt: now, UpdatedAt: now,
		}
		if err = o.db.WithContext(ctx).Create(&record).Error; err != nil {
			if isReceiptRaceError(err) {
				replay, resolved, raceErr := o.resolveDeletionCreateRace(ctx, s, input.RequestID, fingerprint)
				if raceErr != nil {
					return CareerDeletionReceipt{}, raceErr
				}
				if resolved {
					return replay, nil
				}
			}
			return CareerDeletionReceipt{}, err
		}
	} else {
		return CareerDeletionReceipt{}, err
	}

	execution := newDeletionExecution()
	if err := json.Unmarshal([]byte(record.StateBody), &execution); err != nil {
		return CareerDeletionReceipt{}, fmt.Errorf("decode career deletion state: %w", err)
	}

	failed := o.runDeletionSteps(ctx, s, input.RequestID, &execution)

	// The finalize phase runs inside one transaction that locks the deletion
	// audit row first and the profile row second (ocr3-128). The record lock
	// makes finalize exactly-once per request: a concurrent retry of the same
	// request ID that reaches this point after another runner finalized
	// replays the stored receipt instead of re-running finalize and bumping
	// the revision twice. The profile lock — the one every Career write path
	// takes first — plus the in-transaction re-purge close the window in
	// which a racing write could commit new career rows after the purge step
	// and before the receipt claims "deleted". Step failures still commit
	// their partial state afterwards, exactly as before.
	var finalized CareerDeletionReceipt
	if failed == "" {
		finalizeErr := o.runImportTransaction(ctx, func(tx *gorm.DB) error {
			var current careerDataDeletionRecord
			e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, input.RequestID).
				First(&current).Error
			if e != nil {
				return e
			}
			if current.Status == DeletionStatusDeleted {
				// A concurrent runner finalized first: its receipt replays
				// and nothing here re-runs.
				storedReceipt, decodeErr := decodeCareerDeletionReceipt(current.ReceiptBody)
				if decodeErr != nil {
					return decodeErr
				}
				finalized = storedReceipt
				return nil
			}
			var head profile
			e = tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("tenant_id=? AND user_id=?", s.TenantID, s.UserID).First(&head).Error
			if errors.Is(e, gorm.ErrRecordNotFound) {
				// A space without a profile row still finalizes: create the
				// epoch counter (the same first-write semantics profile
				// writes use) so the lock has a row to hold.
				head = profile{TenantID: s.TenantID, UserID: s.UserID, Revision: 0}
				if e = tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&head).Error; e != nil {
					return e
				}
			} else if e != nil {
				return e
			}
			// Defensive sweep: the purge step ran unlocked above; anything a
			// racing write committed in between is removed here under the
			// profile lock, so the "deleted" receipt stays truthful.
			if e = purgeCareerRows(tx, s); e != nil {
				return e
			}
			if e = finalizeDeletionRows(tx, s, &execution); e != nil {
				return e
			}
			finalized = CareerDeletionReceipt{
				Kind:      CareerKindDeleted,
				RequestID: input.RequestID,
				Status:    DeletionStatusDeleted,
				Steps:     execution.Steps,
				Retention: careerDeletionRetention(),
				Revision:  execution.NextRevision,
				StartedAt: record.CreatedAt,
			}
			completed := time.Now().UTC()
			finalized.CompletedAt = &completed
			return persistDeletionOutcomeRow(tx, s, input.RequestID, DeletionStatusDeleted, &execution, finalized, &finalized)
		})
		if finalizeErr != nil {
			failed = DeletionStepFinalize
			execution.Steps[execution.StepIndex[DeletionStepFinalize]].Status = DeletionStepStatusFailed
			execution.Steps[execution.StepIndex[DeletionStepFinalize]].Detail = finalizeErr.Error()
		} else if finalized.Kind != "" {
			return finalized, nil
		}
	}

	status := DeletionStatusPartial
	receipt := CareerDeletionReceipt{
		Kind:      CareerKindDeleted,
		RequestID: input.RequestID,
		Status:    status,
		Steps:     execution.Steps,
		Retention: careerDeletionRetention(),
		Revision:  execution.NextRevision,
		StartedAt: record.CreatedAt,
	}
	return o.persistDeletionOutcome(ctx, s, input.RequestID, status, &execution, receipt)
}

// resolveDeletionCreateRace settles a lost Create race for a deletion
// request ID: the winner's durable row decides. A fingerprint mismatch
// (same request ID, different expected revision) is a definite conflict —
// the winner's receipt must never be handed to the loser — while an exact
// match replays the winner's current receipt. resolved=false means no
// durable row was found and the original Create error still stands.
func (o *Office) resolveDeletionCreateRace(
	ctx context.Context, s Scope, requestID, fingerprint string,
) (CareerDeletionReceipt, bool, error) {
	var raced careerDataDeletionRecord
	e := o.db.WithContext(ctx).
		Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, requestID).
		First(&raced).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return CareerDeletionReceipt{}, false, nil
	}
	if e != nil {
		return CareerDeletionReceipt{}, false, e
	}
	if raced.Fingerprint != fingerprint {
		return CareerDeletionReceipt{}, false, ErrIdempotencyConflict
	}
	replay, lookupErr := o.FindCareerDeletion(ctx, requestID)
	if lookupErr != nil {
		return CareerDeletionReceipt{}, false, lookupErr
	}
	return replay, true, nil
}

// persistDeletionOutcome records this run's outcome — unless the same
// request ID already finalized its terminal "deleted" state: a late partial
// from a concurrent runner must never regress a finalized receipt, so the
// write is conditional on the row not being deleted yet, and a no-op write
// replays the stored terminal receipt instead.
func (o *Office) persistDeletionOutcome(
	ctx context.Context, s Scope, requestID, status string, execution *deletionExecution, receipt CareerDeletionReceipt,
) (CareerDeletionReceipt, error) {
	result := o.db.WithContext(ctx).Model(&careerDataDeletionRecord{}).
		Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, requestID).
		Where("status<>?", DeletionStatusDeleted).
		Updates(map[string]any{
			"status":       status,
			"state_body":   string(mustJSON(execution)),
			"receipt_body": string(mustJSON(receipt)),
			"updated_at":   time.Now().UTC(),
		})
	if result.Error != nil {
		return CareerDeletionReceipt{}, result.Error
	}
	if result.RowsAffected == 0 {
		if stored, err := o.FindCareerDeletion(ctx, requestID); err == nil {
			return stored, nil
		}
		return CareerDeletionReceipt{}, ErrDeletionNotFound
	}
	return receipt, nil
}

// FindCareerDeletion replays the durable deletion receipt by request ID.
func (o *Office) FindCareerDeletion(ctx context.Context, requestID string) (CareerDeletionReceipt, error) {
	s, err := getScope(ctx)
	if err != nil {
		return CareerDeletionReceipt{}, err
	}
	if err = o.requireSpace(ctx, s); err != nil {
		return CareerDeletionReceipt{}, err
	}
	requestID = trimRequestID(requestID)
	if requestID == "" {
		return CareerDeletionReceipt{}, ErrInvalidRequest
	}
	var record careerDataDeletionRecord
	err = o.db.WithContext(ctx).
		Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, requestID).
		First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return CareerDeletionReceipt{}, ErrDeletionNotFound
	}
	if err != nil {
		return CareerDeletionReceipt{}, err
	}
	// A record still in the executing window carries the initial "{}" body.
	// Decoding it would present an empty (status-less) receipt as success;
	// instead the durable deletion state is synthesized into a truthful
	// in-progress receipt. Terminal records (deleted/partial) always carry
	// their real receipt and replay directly.
	if record.Status == DeletionStatusDeleting && record.ReceiptBody == "{}" {
		receipt := CareerDeletionReceipt{
			Kind:      CareerKindDeleted,
			RequestID: record.RequestID,
			Status:    DeletionStatusDeleting,
			Steps:     newDeletionExecution().Steps,
			Retention: careerDeletionRetention(),
			Revision:  record.ExpectedRevision,
			StartedAt: record.CreatedAt,
		}
		var execution deletionExecution
		if json.Unmarshal([]byte(record.StateBody), &execution) == nil && execution.Steps != nil {
			receipt.Steps = execution.Steps
		}
		return receipt, nil
	}
	return decodeCareerDeletionReceipt(record.ReceiptBody)
}

func decodeCareerDeletionReceipt(body string) (CareerDeletionReceipt, error) {
	var receipt CareerDeletionReceipt
	if err := json.Unmarshal([]byte(body), &receipt); err != nil {
		return CareerDeletionReceipt{}, fmt.Errorf("decode career deletion receipt: %w", err)
	}
	return receipt, nil
}

func newDeletionExecution() deletionExecution {
	steps := []CareerDeletionStep{
		{Name: DeletionStepRevokeExports, Status: DeletionStepStatusPending},
		{Name: DeletionStepPurgeCareerData, Status: DeletionStepStatusPending},
		{Name: DeletionStepRemoveProjections, Status: DeletionStepStatusPending},
		{Name: DeletionStepFinalize, Status: DeletionStepStatusPending},
	}
	index := map[string]int{}
	for i, step := range steps {
		index[step.Name] = i
	}
	return deletionExecution{Steps: steps, StepIndex: index}
}

// runDeletionSteps executes every pending step in order. It returns the name
// of the first failing step ("" when all succeeded). Completed steps are
// never re-run, which is what makes a retry under the same request ID a
// resume instead of a replay from zero.
func (o *Office) runDeletionSteps(ctx context.Context, s Scope, requestID string, execution *deletionExecution) string {
	for i, step := range execution.Steps {
		if step.Status == DeletionStepStatusDone {
			continue
		}
		var err error
		switch step.Name {
		case DeletionStepRevokeExports:
			err = o.deletionRevokeExports(ctx, s)
		case DeletionStepPurgeCareerData:
			err = o.deletionPurgeCareerData(ctx, s)
		case DeletionStepRemoveProjections:
			err = o.deletionRemoveProjections(ctx, s)
		default:
			continue
		}
		if o.failDeletionStep != nil {
			if injected := o.failDeletionStep(step.Name); injected != nil && err == nil {
				err = injected
			}
		}
		if err != nil {
			execution.Steps[i].Status = DeletionStepStatusFailed
			execution.Steps[i].Detail = err.Error()
			return step.Name
		}
		execution.Steps[i].Status = DeletionStepStatusDone
		execution.Steps[i].Detail = ""
	}
	return ""
}

// deletionRevokeExports revokes every material export under the scope AND
// removes its physical objects. The objects are deleted first: the purge
// step afterwards clears the export rows, so any file left behind would be
// unreachable and remain on disk forever. DeleteExport is idempotent, so a
// retry after a partial object failure never wedges on earlier progress.
func (o *Office) deletionRevokeExports(ctx context.Context, s Scope) error {
	var rows []materialExportRecord
	if err := o.db.WithContext(ctx).
		Where("tenant_id=? AND user_id=?", s.TenantID, s.UserID).
		Find(&rows).Error; err != nil {
		return err
	}
	if len(rows) > 0 && o.exportStorage == nil {
		for _, row := range rows {
			if row.PDFObjectKey != "" || row.DOCXObjectKey != "" {
				return ErrExportStorageUnavailable
			}
		}
	}
	if o.exportStorage != nil {
		for _, row := range rows {
			for _, key := range []string{row.PDFObjectKey, row.DOCXObjectKey} {
				if key == "" {
					continue
				}
				if err := o.exportStorage.DeleteExport(ctx, key); err != nil {
					return fmt.Errorf("delete career material export object %s: %w", key, err)
				}
			}
		}
	}
	return o.db.WithContext(ctx).Model(&materialExportRecord{}).
		Where("tenant_id=? AND user_id=?", s.TenantID, s.UserID).
		Updates(map[string]any{"status": ExportStatusRevoked, "revoked_at": time.Now().UTC(), "updated_at": time.Now().UTC()}).Error
}

// deletionPurgeCareerData first releases every uploaded source through the
// catalog seam (binding + physical object) and then clears the Career rows.
// Each source's ref is blanked right after a successful release so a retry
// after a mid-step failure never releases the same resource twice.
func (o *Office) deletionPurgeCareerData(ctx context.Context, s Scope) error {
	var sources []sourceRevision
	if err := o.db.WithContext(ctx).
		Where("tenant_id=? AND user_id=? AND resource_ref<>''", s.TenantID, s.UserID).
		Find(&sources).Error; err != nil {
		return err
	}
	if len(sources) > 0 && o.sourceUploadReleaser == nil {
		return errors.New("career source upload releaser is not configured")
	}
	if o.sourceUploadReleaser != nil {
		for _, src := range sources {
			if err := o.sourceUploadReleaser.Release(ctx, src.ResourceRef, src.ID); err != nil {
				return fmt.Errorf("release career source %s: %w", src.ID, err)
			}
			if err := o.db.WithContext(ctx).Model(&sourceRevision{}).
				Where("tenant_id=? AND user_id=? AND id=? AND resource_ref=?", s.TenantID, s.UserID, src.ID, src.ResourceRef).
				Update("resource_ref", "").Error; err != nil {
				return err
			}
		}
	}
	return o.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return purgeCareerRows(tx, s)
	})
}

// purgeCareerRows clears every Career table for the scope inside the given
// transaction. It backs both the purge step and the finalize-phase sweep.
func purgeCareerRows(tx *gorm.DB, s Scope) error {
	for _, table := range careerPurgeTables {
		if err := tx.Table(table).
			Where("tenant_id=? AND user_id=?", s.TenantID, s.UserID).
			Delete(nil).Error; err != nil {
			return fmt.Errorf("purge %s: %w", table, err)
		}
	}
	return nil
}

func (o *Office) deletionRemoveProjections(ctx context.Context, s Scope) error {
	if o.applicationTaskRemover == nil {
		return errors.New("workbench application task remover is not configured")
	}
	_, err := o.applicationTaskRemover.RemoveCareerApplicationTaskProjections(ctx, s.TenantID, s.UserID)
	return err
}

// finalizeDeletionRows bumps the space revision, replaces the changes stream
// with the single career_deleted event, and marks the finalize step done. It
// runs inside the caller's locked transaction (see DeleteCareer): the
// profile row is already held FOR UPDATE there, and the terminal audit write
// lands atomically with the revision bump.
func finalizeDeletionRows(tx *gorm.DB, s Scope, execution *deletionExecution) error {
	var head profile
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("tenant_id=? AND user_id=?", s.TenantID, s.UserID).First(&head).Error; err != nil {
		return err
	}
	next := head.Revision + 1
	if err := tx.Model(&profile{}).
		Where("tenant_id=? AND user_id=? AND revision=?", s.TenantID, s.UserID, head.Revision).
		Update("revision", next).Error; err != nil {
		return err
	}
	if err := tx.Where("tenant_id=? AND user_id=?", s.TenantID, s.UserID).Delete(&change{}).Error; err != nil {
		return err
	}
	event := Change{Revision: next, Kind: ChangeKindCareerDeleted}
	body, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if err := tx.Create(&change{
		TenantID: s.TenantID, UserID: s.UserID, Revision: next,
		Kind: ChangeKindCareerDeleted, Body: string(body),
	}).Error; err != nil {
		return err
	}
	execution.NextRevision = next
	step := execution.Steps[execution.StepIndex[DeletionStepFinalize]]
	step.Status = DeletionStepStatusDone
	execution.Steps[execution.StepIndex[DeletionStepFinalize]] = step
	return nil
}

// persistDeletionOutcomeRow is the in-transaction form of
// persistDeletionOutcome: the terminal write lands atomically with the
// finalize revision bump. Under the deletion-record lock the guarded update
// always applies; the replay hook only keeps the no-regress invariant.
func persistDeletionOutcomeRow(tx *gorm.DB, s Scope, requestID, status string,
	execution *deletionExecution, receipt CareerDeletionReceipt, replay *CareerDeletionReceipt,
) error {
	result := tx.Model(&careerDataDeletionRecord{}).
		Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, requestID).
		Where("status<>?", DeletionStatusDeleted).
		Updates(map[string]any{
			"status":       status,
			"state_body":   string(mustJSON(execution)),
			"receipt_body": string(mustJSON(receipt)),
			"updated_at":   time.Now().UTC(),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		var stored careerDataDeletionRecord
		if err := tx.Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, requestID).
			First(&stored).Error; err != nil {
			return ErrDeletionNotFound
		}
		storedReceipt, decodeErr := decodeCareerDeletionReceipt(stored.ReceiptBody)
		if decodeErr != nil {
			return decodeErr
		}
		*replay = storedReceipt
		return nil
	}
	return nil
}

func trimRequestID(requestID string) string {
	if len(requestID) > 128 {
		return ""
	}
	return strings.TrimSpace(requestID)
}
