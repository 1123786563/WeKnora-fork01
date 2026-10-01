package career

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/google/uuid"
	sqlite3 "github.com/mattn/go-sqlite3"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrUnauthorized        = errors.New("career scope missing or not authorized")
	ErrRevisionConflict    = errors.New("career revision conflict")
	ErrReceiptNotFound     = errors.New("career receipt not found")
	ErrIdempotencyConflict = errors.New("request id was already used with different content")
	ErrInvalidRequest      = errors.New("invalid career request")
	ErrProposalNotFound    = errors.New("career proposal not found")
	ErrProposalResolved    = errors.New("career proposal is no longer pending")
	ErrOutcomeUnknown      = errors.New("career request outcome unknown")
)

type Scope struct {
	UserID   string
	TenantID uint64
}
type scopeKey struct{}

func WithScope(ctx context.Context, s Scope) context.Context {
	return context.WithValue(ctx, scopeKey{}, s)
}
func getScope(ctx context.Context) (Scope, error) {
	s, ok := ctx.Value(scopeKey{}).(Scope)
	if !ok || strings.TrimSpace(s.UserID) == "" || s.TenantID == 0 {
		return Scope{}, ErrUnauthorized
	}
	return s, nil
}

type Source struct {
	Kind        string `json:"kind"`
	Label       string `json:"label,omitempty"`
	ReferenceID string `json:"referenceId,omitempty"`
}
type Confirmation struct {
	UserID      string    `json:"userId"`
	ConfirmedAt time.Time `json:"confirmedAt"`
}
type profile struct {
	TenantID uint64 `gorm:"primaryKey"`
	UserID   string `gorm:"primaryKey;size:512"`
	Revision uint64 `gorm:"not null"`
}
type space struct {
	TenantID uint64 `gorm:"primaryKey"`
	// `unique` (not `uniqueIndex`): migrations create the column inline-UNIQUE
	// and gorm's MigrateColumnUnique drops a generated uni_<table>_<col>
	// constraint when the model lacks the `unique` tag — that drop 42704s
	// and kills boot.
	OwnerUserID string `gorm:"unique;size:512"`
	CreatedAt   time.Time
}

// Lifecycle rows are durable across independently constructed Offices.
// Claims intentionally have no time-based expiry; unknown effects must be
// reconciled by the original request ID before deletion can proceed.
type lifecycleGate struct {
	TenantID            uint64 `gorm:"primaryKey"`
	UserID              string `gorm:"primaryKey;size:512"`
	Phase               string `gorm:"size:16;not null"`
	DeletionRequestID   string `gorm:"size:128"`
	DeletionFingerprint string `gorm:"size:64;not null;default:''"`
}

func (lifecycleGate) TableName() string { return "career_lifecycle_gates" }

type lifecycleClaim struct {
	TenantID    uint64    `gorm:"primaryKey"`
	UserID      string    `gorm:"primaryKey;size:512"`
	Operation   string    `gorm:"primaryKey;size:32"`
	RequestID   string    `gorm:"primaryKey;size:128"`
	Fingerprint string    `gorm:"size:64;not null;default:''"`
	OwnerToken  string    `gorm:"size:36;not null;default:''"`
	CreatedAt   time.Time `gorm:"not null"`
}

func (lifecycleClaim) TableName() string { return "career_lifecycle_claims" }

type fact struct {
	ID           uint   `gorm:"primaryKey"`
	TenantID     uint64 `gorm:"uniqueIndex:career_fact_scope_key"`
	UserID       string `gorm:"uniqueIndex:career_fact_scope_key;size:512"`
	Key          string `gorm:"uniqueIndex:career_fact_scope_key;size:128"`
	Value        string `gorm:"type:text;not null"`
	Revision     uint64
	Source       string `gorm:"type:text"`
	Confirmation string `gorm:"type:text"`
	RequestID    string `gorm:"size:128"`
	CreatedAt    time.Time
}
type factVersion struct {
	ID           uint   `gorm:"primaryKey"`
	TenantID     uint64 `gorm:"index:idx_career_fact_version_scope"`
	UserID       string `gorm:"index:idx_career_fact_version_scope;size:512"`
	Key          string `gorm:"index:idx_career_fact_version_scope;size:128"`
	Value        string `gorm:"type:text"`
	Revision     uint64 `gorm:"index:idx_career_fact_version_scope"`
	Source       string `gorm:"type:text"`
	Confirmation string `gorm:"type:text"`
	ProposalID   string `gorm:"size:36"`
	RequestID    string `gorm:"size:128"`
	CreatedAt    time.Time
}
type proposal struct {
	ID uint `gorm:"primaryKey"`
	// Same inline-UNIQUE migration/gorm tag reconciliation as space.OwnerUserID.
	PublicID         string `gorm:"unique;size:36"`
	TenantID         uint64 `gorm:"index:idx_career_proposal_scope"`
	UserID           string `gorm:"index:idx_career_proposal_scope;size:512"`
	Key              string `gorm:"size:128"`
	Value            string `gorm:"type:text"`
	Evidence         string `gorm:"type:text"`
	Source           string `gorm:"type:text"`
	Status           string `gorm:"size:16;index:idx_career_proposal_scope"`
	ResolvedAt       *time.Time
	ResolvedRevision *uint64
	Confirmation     string    `gorm:"type:text"`
	ResolutionSource string    `gorm:"type:text"`
	CreatedAt        time.Time `gorm:"index:idx_career_proposal_scope"`
}
type change struct {
	ID        uint   `gorm:"primaryKey"`
	TenantID  uint64 `gorm:"uniqueIndex:career_change_revision"`
	UserID    string `gorm:"uniqueIndex:career_change_revision;size:512"`
	Revision  uint64 `gorm:"uniqueIndex:career_change_revision"`
	Kind      string `gorm:"size:32"`
	Body      string `gorm:"type:text"`
	CreatedAt time.Time
}
type receipt struct {
	TenantID    uint64 `gorm:"uniqueIndex:career_receipt_scope_id"`
	UserID      string `gorm:"uniqueIndex:career_receipt_scope_id;size:512"`
	RequestID   string `gorm:"uniqueIndex:career_receipt_scope_id;size:128"`
	Fingerprint string `gorm:"size:64"`
	Body        string `gorm:"type:text"`
	CreatedAt   time.Time
}
type sourceRevision struct {
	ID                string `gorm:"primaryKey;size:36"`
	TenantID          uint64 `gorm:"uniqueIndex:career_source_revision_scope"`
	UserID            string `gorm:"uniqueIndex:career_source_revision_scope;size:512"`
	Revision          uint64 `gorm:"uniqueIndex:career_source_revision_scope"`
	FileName          string `gorm:"size:255"`
	MIMEType          string `gorm:"size:128"`
	Size              int64
	Digest            string `gorm:"size:64"`
	RequestID         string `gorm:"size:128"`
	IntentHash        string `gorm:"size:64"`
	ExpectedRevision  uint64
	ClaimToken        string `gorm:"size:36"`
	LeaseUntil        *time.Time
	ResourceRef       string `gorm:"type:text"`
	Status            string `gorm:"size:16;index"`
	ErrorCategory     string `gorm:"size:64"`
	ErrorMessage      string `gorm:"type:text"`
	ExtractedText     string `gorm:"type:text"`
	MissingCategories string `gorm:"type:text"`
	ReviewFlags       string `gorm:"type:text"`
	CreatedAt         time.Time
	CompletedAt       *time.Time
}

type evaluationRecord struct {
	ID              string    `gorm:"primaryKey;size:36"`
	TenantID        uint64    `gorm:"uniqueIndex:career_evaluation_scope_request;index:idx_career_evaluation_scope"`
	UserID          string    `gorm:"uniqueIndex:career_evaluation_scope_request;index:idx_career_evaluation_scope;size:512"`
	RequestID       string    `gorm:"uniqueIndex:career_evaluation_scope_request;size:128"`
	Fingerprint     string    `gorm:"size:64;not null"`
	Intent          string    `gorm:"type:text;not null"`
	OpportunityID   string    `gorm:"size:36;not null;index"`
	SnapshotID      string    `gorm:"size:36;not null;index"`
	ProfileRevision uint64    `gorm:"not null"`
	ReceiptBody     string    `gorm:"type:text;not null"`
	EvaluationBody  string    `gorm:"type:text;not null"`
	CreatedAt       time.Time `gorm:"not null"`
}

func (evaluationRecord) TableName() string { return "career_evaluations" }

type Fact struct {
	Key          string       `json:"key"`
	Value        string       `json:"value"`
	Revision     uint64       `json:"revision"`
	Source       Source       `json:"source"`
	Confirmation Confirmation `json:"confirmation"`
	ConfirmedAt  time.Time    `json:"confirmedAt"`
}
type Proposal struct {
	ID               string        `json:"id"`
	Key              string        `json:"key"`
	Value            string        `json:"value"`
	Evidence         string        `json:"evidence,omitempty"`
	Source           Source        `json:"source"`
	Status           string        `json:"status"`
	Revision         *uint64       `json:"revision,omitempty"`
	Confirmation     *Confirmation `json:"confirmation,omitempty"`
	ResolutionSource *Source       `json:"resolutionSource,omitempty"`
	CreatedAt        time.Time     `json:"createdAt"`
}
type View struct {
	Revision  uint64     `json:"revision"`
	Facts     []Fact     `json:"facts"`
	Proposals []Proposal `json:"proposals"`
}
type Receipt struct {
	Kind      string     `json:"kind"`
	RequestID string     `json:"requestId"`
	Revision  uint64     `json:"revision"`
	Proposal  *Proposal  `json:"proposal,omitempty"`
	Fact      *Fact      `json:"fact,omitempty"`
	Proposals []Proposal `json:"proposals,omitempty"`
}
type Change struct {
	Revision  uint64     `json:"revision"`
	Kind      string     `json:"kind"`
	Proposal  *Proposal  `json:"proposal,omitempty"`
	Fact      *Fact      `json:"fact,omitempty"`
	Proposals []Proposal `json:"proposals,omitempty"`
}
type ChangeSet struct {
	Revision uint64   `json:"revision"`
	Changes  []Change `json:"changes"`
}
type RevisionConflictError struct{ CurrentRevision uint64 }
type OutcomeUnknownError struct{ RequestID string }

func (e *OutcomeUnknownError) Error() string        { return ErrOutcomeUnknown.Error() }
func (e *OutcomeUnknownError) Is(target error) bool { return target == ErrOutcomeUnknown }

func (e *RevisionConflictError) Error() string        { return ErrRevisionConflict.Error() }
func (e *RevisionConflictError) Is(target error) bool { return target == ErrRevisionConflict }
func (profile) TableName() string                     { return "career_profiles" }
func (space) TableName() string                       { return "career_spaces" }
func (fact) TableName() string                        { return "career_facts" }
func (factVersion) TableName() string                 { return "career_fact_versions" }
func (proposal) TableName() string                    { return "career_proposals" }
func (change) TableName() string                      { return "career_changes" }
func (receipt) TableName() string                     { return "career_receipts" }
func (sourceRevision) TableName() string              { return "career_source_revisions" }

type Office struct {
	// lifecycleMu serializes deletion against operations whose durable side
	// effects cross the Career transaction boundary (export storage and the
	// Workbench task linker). The database revision checks remain the durable
	// guard for ordinary Career writes.
	lifecycleMu          sync.RWMutex
	db                   *gorm.DB
	opportunityExtractor opportunityExtractor
	// linker is the only channel to durable Workbench application tasks.
	// Career never imports Workbench repositories or writes their tables.
	linker interfaces.CareerApplicationTaskLinker
	// Source policy owns URL source trust; production starts with an empty
	// allowlist. The transport is a narrow HTTP-only adapter and must never be
	// invoked for unapproved sources.
	sourcePolicy    SourcePolicy
	sourceTransport SourceTransport
	// One-shot search seams (T11, upgraded by T21): the registry enumerates
	// vetted searchable sources (production starts empty) and the quota gate
	// is the only quota authority. Since T21 the production gate is the real
	// usage ledger: reserve/settle/release by request ID with a frozen cost.
	searchRegistry  SearchSourceRegistry
	searchQuotaGate SearchQuotaGate
	// searchQuotaLimit pins the monthly allowance (tests shrink it; the
	// deployment overrides it through CAREER_SEARCH_QUOTA_LIMIT).
	searchQuotaLimit int64
	// Recurring search rule clock seam (T13): set_rule scheduling decisions
	// read this injected clock; tests pin it, production reads the wall clock.
	// Rule triggering itself is the explicit TriggerDueRules(ctx, now) seam —
	// no resident scheduler exists on the production path.
	searchRuleNow func() time.Time
	// These hooks only synchronize transaction-boundary and error-path tests.
	beforeFirstWrite               func()
	afterReceiptPersist            func()
	beforeReplayReceipt            func(context.Context)
	beforeOpportunityTransaction   func()
	afterOpportunityCommit         func() error
	afterEvaluationCommit          func() error
	afterEvaluationReceiptMiss     func()
	failApplicationReadyUpdate     func() error
	failSearchTerminalCommit       func() error
	afterProgressEventPersist      func() error
	afterSubmissionPersist         func() error
	afterSubmissionProgressPersist func() error
	// failUsageLedgerRead (T21) injects an unreadable quota ledger: admission
	// and estimates must then fail closed instead of executing first.
	failUsageLedgerRead func() error
	// Export rendering seams (T16): storage holds the rendered bytes under
	// local:// object keys, the signing key mints short-lived download grants,
	// and exportNow only makes expiry testable. failExportVerify injects a
	// verification failure for one format in error-path tests.
	exportStorage    materialExportStorage
	exportSigningKey []byte
	exportNow        func() time.Time
	failExportVerify func(format string) error
	// Complete-deletion seam (T22): the remover is the only channel to
	// Workbench application-task projections during delete_career.
	// failDeletionStep injects a sub-deletion failure for recovery tests.
	applicationTaskRemover interfaces.CareerApplicationTaskProjectionRemover
	failDeletionStep       func(step string) error
	// sourceUploadReleaser releases uploaded source originals (catalog
	// binding + physical object) before the purge step clears their rows.
	sourceUploadReleaser careerSourceUploadReleaser
	// Preparation generation seam (T19): the generator composes the cover
	// letter / interview draft from durable evidence. Production wires the
	// deterministic local composer — no external LLM dependency exists on
	// the production path; tests inject fakes through the setter.
	preparationGenerator PreparationGenerator
	// Push reminder seam (T20): the notifier is the only push channel for the
	// in-station todos. Production wires nil — the todo row alone is the
	// authoritative reminder fact; tests inject fakes through the setter.
	reminderNotifier ReminderNotifier
	// failReconcileCommit (T12) injects a post-decision failure inside the
	// reconciliation transaction so unknown-outcome recovery is testable.
	failReconcileCommit func() error
}

func NewOffice(db *gorm.DB) (*Office, error) {
	if db == nil {
		return nil, errors.New("career database required")
	}
	models := []any{&profile{}, &space{}, &lifecycleGate{}, &lifecycleClaim{}, &fact{}, &factVersion{}, &proposal{}, &change{}, &receipt{}, &sourceRevision{}, &opportunity{}, &opportunityObservation{}, &opportunitySnapshot{}, &opportunityReceipt{}, &evaluationRecord{}, &applicationRecord{}, &searchRecord{}, &searchResultRecord{}, &materialRecord{}, &materialVersionRecord{}, &materialReceiptRecord{}, &materialExportRecord{}, &progressEventRecord{}, &searchRuleRecord{}, &searchRuleReceiptRecord{}, &searchRuleRunRecord{}, &searchDiscoveryTodoRecord{}, &submissionRecord{}, &careerDataExportRecord{}, &careerDataDeletionRecord{}, &preparationRecord{}, &reminderRecord{}, &reminderReceiptRecord{}, &usageReservationRecord{}, &reconciliationRecord{}}
	if db.Dialector.Name() == "sqlite" {
		present := 0
		for _, model := range models {
			if db.Migrator().HasTable(model) {
				present++
			}
		}
		switch {
		case present == 0:
			// Keep AutoMigrate as a compatibility path for a truly new
			// database. The application migration runner still owns upgrades.
		case present != len(models):
			return nil, fmt.Errorf("incomplete Career SQLite schema: found %d of %d tables; apply database migrations before startup", present, len(models))
		default:
			if err := validateSQLiteCareerSchema(db); err != nil {
				return nil, err
			}
		}
		if present == 0 {
			if err := db.AutoMigrate(models...); err != nil {
				return nil, err
			}
		}
	} else {
		if e := db.AutoMigrate(models...); e != nil {
			return nil, e
		}
	}
	if err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS career_source_request_scope ON career_source_revisions (tenant_id, user_id, request_id) WHERE request_id <> ''").Error; err != nil {
		return nil, err
	}
	policy := SourcePolicy(emptySourcePolicy{})
	limit, err := resolveSearchQuotaLimitFromEnv()
	if err != nil {
		return nil, err
	}
	office := &Office{
		db:                   db,
		sourcePolicy:         policy,
		sourceTransport:      newCareerSourceTransport(policy, transportDialOptions{}),
		searchRegistry:       emptySearchSourceRegistry{},
		searchQuotaGate:      passThroughSearchQuotaGate{},
		preparationGenerator: deterministicPreparationGenerator{},
		searchQuotaLimit:     limit,
	}
	// T21 upgrades the T11 pass-through into the real usage ledger: the
	// production gate reserves, settles, and releases quota by request ID.
	office.searchQuotaGate = searchUsageGate{office: office}
	return office, nil
}

func validateSQLiteCareerSchema(db *gorm.DB) error {
	requiredColumns := map[string][]string{
		"career_profiles":                 {"tenant_id", "user_id", "revision"},
		"career_spaces":                   {"tenant_id", "owner_user_id", "created_at"},
		"career_lifecycle_gates":          {"tenant_id", "user_id", "phase", "deletion_request_id", "deletion_fingerprint"},
		"career_lifecycle_claims":         {"tenant_id", "user_id", "operation", "request_id", "fingerprint", "created_at"},
		"career_facts":                    {"id", "tenant_id", "user_id", "key", "value", "revision", "source", "confirmation", "request_id", "created_at"},
		"career_fact_versions":            {"id", "tenant_id", "user_id", "key", "value", "revision", "source", "confirmation", "proposal_id", "request_id", "created_at"},
		"career_proposals":                {"id", "public_id", "tenant_id", "user_id", "key", "value", "evidence", "source", "status", "resolved_at", "resolved_revision", "confirmation", "resolution_source", "created_at"},
		"career_changes":                  {"id", "tenant_id", "user_id", "revision", "kind", "body", "created_at"},
		"career_receipts":                 {"tenant_id", "user_id", "request_id", "fingerprint", "body", "created_at"},
		"career_source_revisions":         {"id", "tenant_id", "user_id", "revision", "file_name", "mime_type", "size", "digest", "request_id", "intent_hash", "expected_revision", "claim_token", "lease_until", "resource_ref", "status", "error_category", "error_message", "extracted_text", "missing_categories", "review_flags", "created_at", "completed_at"},
		"career_opportunities":            {"id", "tenant_id", "user_id", "created_at"},
		"career_opportunity_observations": {"id", "tenant_id", "user_id", "opportunity_id", "snapshot_id", "source_kind", "source_label", "source_ref", "acquired_at", "created_at", "source_status", "completeness", "failure_code", "submitted_url", "final_url", "adapter_id", "adapter_version", "observed_http_status"},
		"career_opportunity_snapshots":    {"id", "tenant_id", "user_id", "opportunity_id", "observation_id", "raw_text", "raw_sha256", "extracted", "status", "acquired_at", "created_at"},
		"career_opportunity_receipts":     {"tenant_id", "user_id", "request_id", "fingerprint", "body", "created_at"},
		"career_evaluations":              {"id", "tenant_id", "user_id", "request_id", "fingerprint", "intent", "opportunity_id", "snapshot_id", "profile_revision", "receipt_body", "evaluation_body", "created_at"},
		"career_applications":             {"id", "tenant_id", "user_id", "request_id", "fingerprint", "opportunity_id", "snapshot_id", "evaluation_id", "profile_revision", "evidence_body", "batch_identity", "continue_despite_hard_failure", "evaluation_status", "qualified", "warning_body", "link_state", "task_id", "run_id", "receipt_body", "created_at", "updated_at"},
		"career_searches":                 {"id", "tenant_id", "user_id", "request_id", "fingerprint", "status", "claim_token", "lease_until", "query", "receipt_body", "created_at", "completed_at"},
		"career_search_results":           {"id", "tenant_id", "user_id", "search_id", "source_id", "link", "checked_at", "qualification", "uncertainty", "created_at"},
		"career_materials":                {"id", "tenant_id", "user_id", "request_id", "fingerprint", "opportunity_id", "snapshot_id", "profile_revision", "evidence_body", "status", "draft_body", "failure_code", "failure_message", "version_count", "receipt_body", "created_at", "updated_at"},
		"career_material_versions":        {"id", "tenant_id", "user_id", "material_id", "version", "request_id", "fingerprint", "evidence_body", "version_body", "receipt_body", "created_at"},
		"career_material_receipts":        {"tenant_id", "user_id", "request_id", "fingerprint", "body", "created_at"},
		"career_material_exports":         {"id", "tenant_id", "user_id", "material_id", "version", "request_id", "fingerprint", "content_digest", "status", "pdf_object_key", "pdf_digest", "pdf_size", "pdf_error", "docx_object_key", "docx_digest", "docx_size", "docx_error", "revoked_at", "receipt_body", "created_at", "updated_at"},
		"career_progress_events":          {"id", "tenant_id", "user_id", "application_id", "seq", "kind", "event_type", "note", "occurred_at", "corrects_event_id", "source", "confirmer", "request_id", "fingerprint", "receipt_body", "created_at"},
		"career_search_rules":             {"id", "tenant_id", "user_id", "query", "interval_minutes", "status", "revision", "last_period", "next_due_at", "created_at", "updated_at"},
		"career_search_rule_receipts":     {"tenant_id", "user_id", "request_id", "fingerprint", "body", "created_at"},
		"career_search_rule_runs":         {"id", "tenant_id", "user_id", "rule_id", "period", "request_id", "status", "body", "created_at"},
		"career_search_discovery_todos":   {"id", "tenant_id", "user_id", "rule_id", "run_id", "search_id", "source_id", "link", "status", "created_at"},
		"career_submissions":              {"id", "tenant_id", "user_id", "application_id", "request_id", "fingerprint", "channel", "occurred_at", "version_confirmed", "material_id", "export_id", "version", "content_digest", "note", "confirmer", "receipt_body", "created_at", "updated_at"},
		"career_data_exports":             {"id", "tenant_id", "user_id", "request_id", "fingerprint", "revision", "digest", "archive_body", "receipt_body", "created_at"},
		"career_data_deletions":           {"id", "tenant_id", "user_id", "request_id", "fingerprint", "expected_revision", "status", "state_body", "receipt_body", "created_at", "updated_at"},
		"career_preparations":             {"id", "tenant_id", "user_id", "application_id", "request_id", "fingerprint", "focus", "status", "submission_id", "submitted_material_id", "submitted_export_id", "submitted_version", "submitted_digest", "snapshot_id", "snapshot_sha256", "profile_revision", "material_id", "failure_code", "failure_message", "receipt_body", "created_at", "updated_at"},
		"career_reminders":                {"id", "tenant_id", "user_id", "source_kind", "source_id", "application_id", "opportunity_id", "notice_key", "status", "request_id", "created_at", "updated_at"},
		"career_reminder_receipts":        {"tenant_id", "user_id", "request_id", "fingerprint", "body", "created_at"},
		"career_usage_reservations":       {"id", "tenant_id", "user_id", "operation", "request_id", "cost_units", "status", "period_start", "period_end", "lease_until", "created_at", "settled_at"},
		"career_reconciliations":          {"id", "tenant_id", "user_id", "request_id", "fingerprint", "decision", "target_id", "candidate_id", "evidence_body", "receipt_body", "created_at"},
	}
	for table, columns := range requiredColumns {
		for _, column := range columns {
			if !db.Migrator().HasColumn(table, column) {
				return fmt.Errorf("incomplete Career SQLite schema: %s.%s is missing; apply database migrations before startup", table, column)
			}
		}
	}
	for table, columns := range map[string][]string{
		"career_facts":                  {"tenant_id", "user_id", "key"},
		"career_changes":                {"tenant_id", "user_id", "revision"},
		"career_receipts":               {"tenant_id", "user_id", "request_id"},
		"career_source_revisions":       {"tenant_id", "user_id", "revision"},
		"career_opportunity_receipts":   {"tenant_id", "user_id", "request_id"},
		"career_evaluations":            {"tenant_id", "user_id", "request_id"},
		"career_applications":           {"tenant_id", "user_id", "request_id"},
		"career_searches":               {"tenant_id", "user_id", "request_id"},
		"career_search_results":         {"tenant_id", "user_id", "search_id", "link"},
		"career_materials":              {"tenant_id", "user_id", "request_id"},
		"career_material_versions":      {"tenant_id", "user_id", "material_id", "version"},
		"career_material_receipts":      {"tenant_id", "user_id", "request_id"},
		"career_material_exports":       {"tenant_id", "user_id", "request_id"},
		"career_progress_events":        {"tenant_id", "user_id", "request_id"},
		"career_search_rules":           {"tenant_id", "user_id", "id"},
		"career_search_rule_receipts":   {"tenant_id", "user_id", "request_id"},
		"career_search_rule_runs":       {"tenant_id", "user_id", "rule_id", "period"},
		"career_search_discovery_todos": {"tenant_id", "user_id", "link"},
		"career_submissions":            {"tenant_id", "user_id", "request_id"},
		"career_data_exports":           {"tenant_id", "user_id", "request_id"},
		"career_data_deletions":         {"tenant_id", "user_id", "request_id"},
		"career_preparations":           {"tenant_id", "user_id", "request_id"},
		"career_reminders":              {"tenant_id", "user_id", "source_kind", "source_id"},
		"career_reminder_receipts":      {"tenant_id", "user_id", "request_id"},
		"career_usage_reservations":     {"tenant_id", "user_id", "request_id"},
		"career_reconciliations":        {"tenant_id", "user_id", "request_id"},
	} {
		if err := requireSQLiteUniqueConstraint(db, table, columns); err != nil {
			return fmt.Errorf("incomplete Career SQLite schema: %w; apply database migrations before startup", err)
		}
	}
	// One job and batch admits exactly one application; this second uniqueness
	// needs its own check because the map above allows one entry per table.
	if err := requireSQLiteUniqueConstraint(db, "career_applications", []string{"tenant_id", "user_id", "opportunity_id", "batch_identity"}); err != nil {
		return fmt.Errorf("incomplete Career SQLite schema: %w; apply database migrations before startup", err)
	}
	// Progress events are append-only: one application holds each sequence
	// number exactly once and each request ID writes exactly one event.
	if err := requireSQLiteUniqueConstraint(db, "career_progress_events", []string{"tenant_id", "user_id", "application_id", "seq"}); err != nil {
		return fmt.Errorf("incomplete Career SQLite schema: %w; apply database migrations before startup", err)
	}
	// One application holds at most one user-confirmed submission record.
	if err := requireSQLiteUniqueConstraint(db, "career_submissions", []string{"tenant_id", "user_id", "application_id"}); err != nil {
		return fmt.Errorf("incomplete Career SQLite schema: %w; apply database migrations before startup", err)
	}
	// One source event holds exactly one reminder todo; this second uniqueness
	// (the creating request ID) needs its own check because the map above
	// allows one entry per table.
	if err := requireSQLiteUniqueConstraint(db, "career_reminders", []string{"tenant_id", "user_id", "request_id"}); err != nil {
		return fmt.Errorf("incomplete Career SQLite schema: %w; apply database migrations before startup", err)
	}
	return nil
}

func requireSQLiteUniqueConstraint(db *gorm.DB, table string, columns []string) error {
	var indexes []struct {
		Name    string
		Partial int
	}
	if err := db.Raw(`SELECT name, partial FROM pragma_index_list(?) WHERE "unique" = 1`, table).Scan(&indexes).Error; err != nil {
		return fmt.Errorf("inspect %s unique constraints: %w", table, err)
	}
	for _, index := range indexes {
		if index.Partial != 0 {
			continue
		}
		var indexedColumns []string
		if err := db.Raw("SELECT name FROM pragma_index_info(?) ORDER BY seqno", index.Name).Scan(&indexedColumns).Error; err != nil {
			return fmt.Errorf("inspect %s unique constraint %s: %w", table, index.Name, err)
		}
		if len(indexedColumns) != len(columns) {
			continue
		}
		matches := true
		for i := range columns {
			if indexedColumns[i] != columns[i] {
				matches = false
				break
			}
		}
		if matches {
			return nil
		}
	}
	return fmt.Errorf("%s uniqueness on (%s) is missing", table, strings.Join(columns, ", "))
}

func (o *Office) ClaimSpace(ctx context.Context) error {
	s, e := getScope(ctx)
	if e != nil {
		return e
	}
	row := space{TenantID: s.TenantID, OwnerUserID: s.UserID}
	res := o.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
	if res.Error != nil {
		return res.Error
	}
	var actual space
	e = o.db.WithContext(ctx).Where("tenant_id=?", s.TenantID).First(&actual).Error
	if e != nil {
		return ErrUnauthorized
	}
	if actual.OwnerUserID != s.UserID {
		return ErrUnauthorized
	}
	return nil
}
func (o *Office) requireSpace(ctx context.Context, s Scope) error {
	var row space
	e := o.db.WithContext(ctx).Where("tenant_id=? AND owner_user_id=?", s.TenantID, s.UserID).First(&row).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return ErrUnauthorized
	}
	return e
}
func decodeSource(v string) Source { var s Source; _ = json.Unmarshal([]byte(v), &s); return s }
func decodeConfirmation(v string) Confirmation {
	var c Confirmation
	_ = json.Unmarshal([]byte(v), &c)
	return c
}
func (o *Office) Open(ctx context.Context) (View, error) {
	s, e := getScope(ctx)
	if e != nil {
		return View{}, e
	}
	if e = o.requireSpace(ctx, s); e != nil {
		return View{}, e
	}
	v := View{Facts: []Fact{}, Proposals: []Proposal{}}
	var p profile
	if e = o.db.WithContext(ctx).Where("tenant_id=? AND user_id=?", s.TenantID, s.UserID).First(&p).Error; e != nil && !errors.Is(e, gorm.ErrRecordNotFound) {
		return v, e
	}
	v.Revision = p.Revision
	var fs []fact
	if e = o.db.WithContext(ctx).Where("tenant_id=? AND user_id=?", s.TenantID, s.UserID).Order("key").Find(&fs).Error; e != nil {
		return v, e
	}
	for _, f := range fs {
		c := decodeConfirmation(f.Confirmation)
		v.Facts = append(v.Facts, Fact{f.Key, f.Value, f.Revision, decodeSource(f.Source), c, c.ConfirmedAt})
	}
	var ps []proposal
	if e = o.db.WithContext(ctx).Where("tenant_id=? AND user_id=? AND status='pending'", s.TenantID, s.UserID).Order("created_at,id").Find(&ps).Error; e != nil {
		return v, e
	}
	for _, p := range ps {
		v.Proposals = append(v.Proposals, Proposal{ID: p.PublicID, Key: p.Key, Value: p.Value, Evidence: p.Evidence, Source: decodeSource(p.Source), Status: p.Status, Revision: p.ResolvedRevision, CreatedAt: p.CreatedAt})
	}
	return v, nil
}
func (o *Office) Act(ctx context.Context, action, proposalID, k, v, r string, rev uint64, source Source) (Receipt, error) {
	switch action {
	case "propose":
		return o.propose(ctx, k, v, r, rev, source)
	case "confirm":
		return o.confirm(ctx, "", k, v, r, rev, source, source)
	case "confirm_proposal":
		return o.confirm(ctx, proposalID, "", "", r, rev, Source{}, source)
	case "dismiss":
		return o.dismiss(ctx, proposalID, r, rev, source)
	default:
		return Receipt{}, ErrInvalidRequest
	}
}
func (o *Office) Propose(ctx context.Context, k, v, r string, rev uint64, src Source) (Receipt, error) {
	return o.propose(ctx, k, v, r, rev, src)
}
func (o *Office) propose(ctx context.Context, k, v, r string, rev uint64, src Source) (out Receipt, err error) {
	s, e := getScope(ctx)
	if e != nil {
		return out, e
	}
	if e = o.requireSpace(ctx, s); e != nil {
		return out, e
	}
	k = strings.TrimSpace(k)
	// career_proposals.key and career_facts.key are VARCHAR(128): a longer
	// key would fail on PostgreSQL with an untyped "value too long" instead
	// of a 400 invalid request (SQLite silently accepts it).
	if k == "" || len(k) > 128 || r == "" || src.Kind == "" {
		return out, ErrInvalidRequest
	}
	pid := uuid.NewString()
	p := Proposal{ID: pid, Key: k, Value: v, Source: src, Status: "pending"}
	return o.mutate(ctx, s, "proposed", r, rev, []any{"propose", k, v, rev, src}, func(tx *gorm.DB, next uint64) (Receipt, error) {
		sb, _ := json.Marshal(src)
		row := proposal{PublicID: pid, TenantID: s.TenantID, UserID: s.UserID, Key: k, Value: v, Source: string(sb), Status: "pending"}
		if e := tx.Create(&row).Error; e != nil {
			return Receipt{}, e
		}
		p.CreatedAt = row.CreatedAt
		return Receipt{Kind: "proposed", RequestID: r, Revision: next, Proposal: &p}, nil
	}, &Change{Kind: "proposed", Proposal: &p})
}
func (o *Office) Confirm(ctx context.Context, k, v, r string, rev uint64, source Source) (Receipt, error) {
	return o.confirm(ctx, "", k, v, r, rev, source, source)
}
func (o *Office) confirm(ctx context.Context, pid, k, v, r string, rev uint64, src, confirmationSrc Source) (out Receipt, err error) {
	s, e := getScope(ctx)
	if e != nil {
		return out, e
	}
	if e = o.requireSpace(ctx, s); e != nil {
		return out, e
	}
	if r == "" || confirmationSrc.Kind == "" || (pid == "" && (strings.TrimSpace(k) == "" || len(k) > 128 || src.Kind == "")) {
		return out, ErrInvalidRequest
	}
	return o.mutate(ctx, s, "confirmed", r, rev, []any{"confirm", pid, k, v, rev, src, confirmationSrc}, func(tx *gorm.DB, next uint64) (Receipt, error) {
		var prop *Proposal
		if pid != "" {
			var row proposal
			if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id=? AND user_id=? AND public_id=?", s.TenantID, s.UserID, pid).First(&row).Error; errors.Is(e, gorm.ErrRecordNotFound) {
				return Receipt{}, ErrProposalNotFound
			} else if e != nil {
				return Receipt{}, e
			}
			if row.Status != "pending" {
				return Receipt{}, ErrProposalResolved
			}
			k, v = row.Key, row.Value
			src = decodeSource(row.Source)
			now := time.Now().UTC()
			row.Status = "confirmed"
			row.ResolvedAt = &now
			row.ResolvedRevision = &next
			cb, _ := json.Marshal(Confirmation{UserID: s.UserID, ConfirmedAt: now})
			row.Confirmation = string(cb)
			rsb, _ := json.Marshal(confirmationSrc)
			row.ResolutionSource = string(rsb)
			if e := tx.Save(&row).Error; e != nil {
				return Receipt{}, e
			}
			conf := decodeConfirmation(row.Confirmation)
			resolutionSource := decodeSource(row.ResolutionSource)
			q := Proposal{ID: row.PublicID, Key: row.Key, Value: row.Value, Evidence: row.Evidence, Source: decodeSource(row.Source), Status: row.Status, Revision: row.ResolvedRevision, CreatedAt: row.CreatedAt, Confirmation: &conf, ResolutionSource: &resolutionSource}
			prop = &q
		}
		now := time.Now().UTC()
		confirmation := Confirmation{UserID: s.UserID, ConfirmedAt: now}
		sb, _ := json.Marshal(src)
		cb, _ := json.Marshal(confirmation)
		f := fact{TenantID: s.TenantID, UserID: s.UserID, Key: k, Value: v, Revision: next, Source: string(sb), Confirmation: string(cb), RequestID: r}
		if e := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "tenant_id"}, {Name: "user_id"}, {Name: "key"}}, DoUpdates: clause.AssignmentColumns([]string{"value", "revision", "source", "confirmation", "request_id", "created_at"})}).Create(&f).Error; e != nil {
			return Receipt{}, e
		}
		ver := factVersion{TenantID: s.TenantID, UserID: s.UserID, Key: k, Value: v, Revision: next, Source: string(sb), Confirmation: string(cb), ProposalID: pid, RequestID: r}
		if e := tx.Create(&ver).Error; e != nil {
			return Receipt{}, e
		}
		factOut := Fact{k, v, next, src, confirmation, now}
		return Receipt{Kind: "confirmed", RequestID: r, Revision: next, Proposal: prop, Fact: &factOut}, nil
	}, &Change{Kind: "confirmed"})
}
func (o *Office) dismiss(ctx context.Context, pid, r string, rev uint64, src Source) (out Receipt, err error) {
	s, e := getScope(ctx)
	if e != nil {
		return out, e
	}
	if e = o.requireSpace(ctx, s); e != nil {
		return out, e
	}
	if pid == "" || r == "" || src.Kind == "" {
		return out, ErrInvalidRequest
	}
	return o.mutate(ctx, s, "dismissed", r, rev, []any{"dismiss", pid, rev, src}, func(tx *gorm.DB, next uint64) (Receipt, error) {
		var p proposal
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id=? AND user_id=? AND public_id=? AND status='pending'", s.TenantID, s.UserID, pid).First(&p).Error; e != nil {
			return Receipt{}, ErrProposalResolved
		}
		now := time.Now().UTC()
		p.Status = "dismissed"
		p.ResolvedAt = &now
		p.ResolvedRevision = &next
		resolutionSourceBytes, _ := json.Marshal(src)
		p.ResolutionSource = string(resolutionSourceBytes)
		confirmation, _ := json.Marshal(Confirmation{UserID: s.UserID, ConfirmedAt: now})
		p.Confirmation = string(confirmation)
		if e := tx.Save(&p).Error; e != nil {
			return Receipt{}, e
		}
		conf := decodeConfirmation(p.Confirmation)
		resolutionSource := decodeSource(p.ResolutionSource)
		q := Proposal{ID: p.PublicID, Key: p.Key, Value: p.Value, Evidence: p.Evidence, Source: decodeSource(p.Source), Status: p.Status, Revision: p.ResolvedRevision, CreatedAt: p.CreatedAt, Confirmation: &conf, ResolutionSource: &resolutionSource}
		return Receipt{Kind: "dismissed", RequestID: r, Revision: next, Proposal: &q}, nil
	}, &Change{Kind: "dismissed"})
}
func (o *Office) mutate(ctx context.Context, s Scope, kind, r string, rev uint64, fpInput any, apply func(*gorm.DB, uint64) (Receipt, error), event *Change) (Receipt, error) {
	// Common throat for every profile write: career_receipts.request_id is
	// VARCHAR(128), so an oversized ID must be refused as invalid_request
	// here instead of surfacing a dialect-dependent 500 on PostgreSQL.
	// Trimming normalizes replay: one client-supplied request ID maps to one
	// durable receipt whatever padding it carried.
	r = strings.TrimSpace(r)
	if r == "" || len(r) > 128 {
		return Receipt{}, ErrInvalidRequest
	}
	fb, _ := json.Marshal(fpInput)
	hash := sha256.Sum256(fb)
	fp := hex.EncodeToString(hash[:])
	// Keep lock acquisition finite even when the HTTP client supplied no deadline.
	operationCtx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	for {
		var out Receipt
		firstWriteBusy := false
		transaction := func(tx *gorm.DB) error {
			if o.beforeFirstWrite != nil {
				o.beforeFirstWrite()
			}
			// This must be the first SQL statement: SQLite obtains its writer
			// reservation before any receipt read snapshot. It also ensures the
			// scoped profile exists before the revision CAS on PostgreSQL.
			p := profile{TenantID: s.TenantID, UserID: s.UserID}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&p).Error; err != nil {
				firstWriteBusy = isSQLiteBusy(err)
				return err
			}
			var old receipt
			e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, r).First(&old).Error
			if e == nil {
				if old.Fingerprint != fp {
					return ErrIdempotencyConflict
				}
				return json.Unmarshal([]byte(old.Body), &out)
			}
			if !errors.Is(e, gorm.ErrRecordNotFound) {
				return e
			}
			next := rev + 1
			updated := tx.Model(&profile{}).Where("tenant_id=? AND user_id=? AND revision=?", s.TenantID, s.UserID, rev).Update("revision", next)
			if updated.Error != nil {
				return updated.Error
			}
			if updated.RowsAffected == 0 {
				if e = tx.Where("tenant_id=? AND user_id=?", s.TenantID, s.UserID).First(&p).Error; e != nil {
					return e
				}
				return &RevisionConflictError{p.Revision}
			}
			out, e = apply(tx, next)
			if e != nil {
				return e
			}
			out.RequestID = r
			out.Revision = next
			if event != nil {
				event.Revision = next
				if out.Proposal != nil {
					event.Proposal = out.Proposal
				}
				if len(out.Proposals) > 0 {
					event.Proposals = out.Proposals
				}
				if out.Fact != nil {
					event.Fact = out.Fact
				}
				body, _ := json.Marshal(event)
				if e = tx.Create(&change{TenantID: s.TenantID, UserID: s.UserID, Revision: next, Kind: kind, Body: string(body)}).Error; e != nil {
					return e
				}
			} else if out.Proposal != nil {
				event = &Change{Revision: next, Kind: kind, Proposal: out.Proposal}
				body, _ := json.Marshal(event)
				if e = tx.Create(&change{TenantID: s.TenantID, UserID: s.UserID, Revision: next, Kind: kind, Body: string(body)}).Error; e != nil {
					return e
				}
			}
			body, e := json.Marshal(out)
			if e != nil {
				return e
			}
			if err := tx.Create(&receipt{TenantID: s.TenantID, UserID: s.UserID, RequestID: r, Fingerprint: fp, Body: string(body)}).Error; err != nil {
				return err
			}
			if o.afterReceiptPersist != nil {
				o.afterReceiptPersist()
			}
			return nil
		}
		var e error
		if o.db.Dialector.Name() == "sqlite" {
			// SQLite's DSN busy handler can outlive a cancelled Go context.
			// Limit it on this borrowed connection, then restore its setting.
			e = o.db.WithContext(operationCtx).Connection(func(conn *gorm.DB) error {
				var prior int
				if err := conn.Raw("PRAGMA busy_timeout").Scan(&prior).Error; err != nil {
					return fmt.Errorf("read sqlite busy timeout: %w", err)
				}
				if err := conn.Exec("PRAGMA busy_timeout=25").Error; err != nil {
					return fmt.Errorf("set sqlite busy timeout: %w", err)
				}
				defer conn.WithContext(context.Background()).Exec("PRAGMA busy_timeout=" + strconv.Itoa(prior))
				return conn.Session(&gorm.Session{NewDB: true}).Transaction(transaction)
			})
		} else {
			e = o.db.WithContext(operationCtx).Transaction(transaction)
		}
		if e == nil {
			return out, nil
		}
		if operationCtx.Err() != nil {
			return Receipt{}, &OutcomeUnknownError{RequestID: r}
		}
		if firstWriteBusy {
			// The failed transaction is rolled back. Retry only this known
			// pre-read lock failure; a later failure may have different semantics.
			timer := time.NewTimer(10 * time.Millisecond)
			select {
			case <-operationCtx.Done():
				timer.Stop()
				return Receipt{}, &OutcomeUnknownError{RequestID: r}
			case <-timer.C:
			}
			continue
		}
		if isReceiptRaceError(e) {
			if replay, found, receiptErr := o.replayReceipt(operationCtx, s, r, fp); receiptErr != nil {
				if operationCtx.Err() != nil || errors.Is(receiptErr, context.Canceled) || errors.Is(receiptErr, context.DeadlineExceeded) {
					return Receipt{}, &OutcomeUnknownError{RequestID: r}
				}
				return Receipt{}, receiptErr
			} else if found {
				return replay, nil
			}
		}
		return Receipt{}, e
	}
}

func isSQLiteBusy(err error) bool {
	var sqliteErr sqlite3.Error
	return errors.As(err, &sqliteErr) && (sqliteErr.Code == sqlite3.ErrBusy || sqliteErr.Code == sqlite3.ErrLocked)
}

func isReceiptRaceError(err error) bool {
	var revisionConflict *RevisionConflictError
	if errors.As(err, &revisionConflict) || errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "database is locked") || strings.Contains(message, "unique constraint") || strings.Contains(message, "duplicate key")
}

func (o *Office) replayReceipt(ctx context.Context, s Scope, requestID, fingerprint string) (Receipt, bool, error) {
	if o.beforeReplayReceipt != nil {
		o.beforeReplayReceipt(ctx)
	}
	var stored receipt
	err := o.db.WithContext(ctx).Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, requestID).First(&stored).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Receipt{}, false, nil
	}
	if err != nil {
		return Receipt{}, false, err
	}
	if stored.Fingerprint != fingerprint {
		return Receipt{}, true, ErrIdempotencyConflict
	}
	var replay Receipt
	if err := json.Unmarshal([]byte(stored.Body), &replay); err != nil {
		return Receipt{}, true, err
	}
	return replay, true, nil
}
func (o *Office) Changes(ctx context.Context, since uint64) (ChangeSet, error) {
	s, e := getScope(ctx)
	if e != nil {
		return ChangeSet{}, e
	}
	if e = o.requireSpace(ctx, s); e != nil {
		return ChangeSet{}, e
	}
	type changeRow struct {
		Head          uint64  `gorm:"column:head"`
		EventRevision *uint64 `gorm:"column:event_revision"`
		Body          *string `gorm:"column:body"`
	}
	var rows []changeRow
	e = o.db.WithContext(ctx).Table("career_profiles AS cp").
		Select("cp.revision AS head, cc.revision AS event_revision, cc.body AS body").
		Joins("LEFT JOIN career_changes AS cc ON cc.tenant_id = cp.tenant_id AND cc.user_id = cp.user_id AND cc.revision > ?", since).
		Where("cp.tenant_id = ? AND cp.user_id = ?", s.TenantID, s.UserID).
		Order("cc.revision ASC").Scan(&rows).Error
	if e != nil {
		return ChangeSet{}, e
	}
	head := uint64(0)
	if len(rows) > 0 {
		head = rows[0].Head
	}
	if since > head {
		return ChangeSet{}, &RevisionConflictError{CurrentRevision: head}
	}
	out := ChangeSet{Revision: head, Changes: []Change{}}
	for _, row := range rows {
		if row.Body == nil || row.EventRevision == nil {
			continue
		}
		var c Change
		if e = json.Unmarshal([]byte(*row.Body), &c); e != nil {
			return out, e
		}
		out.Changes = append(out.Changes, c)
	}
	return out, nil
}
func (o *Office) Receipt(ctx context.Context, r string) (Receipt, error) {
	s, e := getScope(ctx)
	if e != nil {
		return Receipt{}, e
	}
	if e = o.requireSpace(ctx, s); e != nil {
		return Receipt{}, e
	}
	var row receipt
	e = o.db.WithContext(ctx).Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, r).First(&row).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return Receipt{}, ErrReceiptNotFound
	}
	if e != nil {
		return Receipt{}, e
	}
	var out Receipt
	e = json.Unmarshal([]byte(row.Body), &out)
	return out, e
}
func (o *Office) History(ctx context.Context, key string) ([]Fact, error) {
	s, e := getScope(ctx)
	if e != nil {
		return nil, e
	}
	if e = o.requireSpace(ctx, s); e != nil {
		return nil, e
	}
	var rows []factVersion
	e = o.db.WithContext(ctx).Where("tenant_id=? AND user_id=? AND key=?", s.TenantID, s.UserID, key).Order("revision ASC").Find(&rows).Error
	if e != nil {
		return nil, e
	}
	out := make([]Fact, 0, len(rows))
	for _, r := range rows {
		c := decodeConfirmation(r.Confirmation)
		out = append(out, Fact{r.Key, r.Value, r.Revision, decodeSource(r.Source), c, c.ConfirmedAt})
	}
	return out, nil
}
