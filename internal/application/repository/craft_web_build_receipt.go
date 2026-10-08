package repository

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrCraftWebBuildReceiptInvalid     = fmt.Errorf("invalid web build receipt: %w", craft.ErrInvalidInput)
	ErrCraftWebBuildReceiptConflict    = fmt.Errorf("web build receipt conflicts with durable attempt: %w", craft.ErrConflict)
	ErrCraftWebBuildReceiptNotFound    = errors.New("web build receipt not found")
	ErrCraftWebBuildReceiptUnavailable = errors.New("web build receipt store unavailable")
	craftWebBuildReceiptSHA256Pattern  = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// CraftWebBuildReceiptKey is the complete scope used to look up one immutable
// build attempt. SessionID is part of the read scope; request identity remains
// stable across retries and is included in the database uniqueness key.
type CraftWebBuildReceiptKey struct {
	TenantID      uint64
	TaskID        string
	SessionID     string
	WorkspaceID   string
	RunID         string
	ActivityKey   string
	RequestSHA256 string
}

// CraftWebBuildReceipt records the first terminal observation for one fixed
// web build attempt. The writable build log is intentionally absent: none of
// these process facts may be sourced from that delegated output file.
type CraftWebBuildReceipt struct {
	TenantID      uint64
	TaskID        string
	SessionID     string
	WorkspaceID   string
	RunID         string
	ActivityKey   string
	RequestSHA256 string

	CommandSHA256   string
	RuntimeDigest   string
	ToolchainDigest string
	TemplateVersion string
	TemplateSHA256  string
	TimeoutMillis   int64
	OutputLimit     int64

	Provider    string
	ContainerID string
	ExecID      string

	ProcessState            string
	ExitCode                *int
	Started                 bool
	TransportComplete       bool
	OutputComplete          bool
	OutputGeneration        string
	CandidateManifestSHA256 string
	ObservedAt              time.Time
}

func (receipt CraftWebBuildReceipt) Key() CraftWebBuildReceiptKey {
	return CraftWebBuildReceiptKey{
		TenantID: receipt.TenantID, TaskID: receipt.TaskID, SessionID: receipt.SessionID,
		WorkspaceID: receipt.WorkspaceID, RunID: receipt.RunID, ActivityKey: receipt.ActivityKey,
		RequestSHA256: receipt.RequestSHA256,
	}
}

type craftWebBuildReceiptRow struct {
	ID                      string    `gorm:"column:id;primaryKey"`
	TenantID                uint64    `gorm:"column:tenant_id"`
	TaskID                  string    `gorm:"column:task_id"`
	SessionID               string    `gorm:"column:session_id"`
	WorkspaceID             string    `gorm:"column:workspace_id"`
	RunID                   string    `gorm:"column:run_id"`
	ActivityKey             string    `gorm:"column:activity_key"`
	RequestSHA256           string    `gorm:"column:request_sha256"`
	CommandSHA256           string    `gorm:"column:command_sha256"`
	RuntimeDigest           string    `gorm:"column:runtime_digest"`
	ToolchainDigest         string    `gorm:"column:toolchain_digest"`
	TemplateVersion         string    `gorm:"column:template_version"`
	TemplateSHA256          string    `gorm:"column:template_sha256"`
	TimeoutMillis           int64     `gorm:"column:timeout_ms"`
	OutputLimit             int64     `gorm:"column:output_limit"`
	Provider                string    `gorm:"column:provider"`
	ContainerID             string    `gorm:"column:container_id"`
	ExecID                  string    `gorm:"column:exec_id"`
	ProcessState            string    `gorm:"column:process_state"`
	ExitCode                *int      `gorm:"column:exit_code"`
	Started                 bool      `gorm:"column:started"`
	TransportComplete       bool      `gorm:"column:transport_complete"`
	OutputComplete          bool      `gorm:"column:output_complete"`
	OutputGeneration        string    `gorm:"column:output_generation"`
	CandidateManifestSHA256 string    `gorm:"column:candidate_manifest_sha256"`
	ObservedAt              time.Time `gorm:"column:observed_at"`
}

func (craftWebBuildReceiptRow) TableName() string { return "craft_web_build_receipts" }

type CraftWebBuildReceiptRepository struct{ db *gorm.DB }

func NewCraftWebBuildReceiptRepository(db *gorm.DB) *CraftWebBuildReceiptRepository {
	return &CraftWebBuildReceiptRepository{db: db}
}

// RecordTerminal stores the first terminal observation for an attempt. An
// exact replay adopts the original row; a replay that changes any identity,
// pin, provider, process, completeness, or output fact conflicts and cannot
// overwrite it.
func (r *CraftWebBuildReceiptRepository) RecordTerminal(ctx context.Context, receipt CraftWebBuildReceipt) (CraftWebBuildReceipt, error) {
	if r == nil || r.db == nil {
		return CraftWebBuildReceipt{}, ErrCraftWebBuildReceiptUnavailable
	}
	if err := validateCraftWebBuildReceipt(receipt); err != nil {
		return CraftWebBuildReceipt{}, err
	}
	receipt.ObservedAt = receipt.ObservedAt.UTC().Round(time.Microsecond)
	row := craftWebBuildReceiptRowFromReceipt(receipt)
	row.ID = uuid.NewString()
	created := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
	if created.Error != nil {
		if err := ctx.Err(); err != nil {
			return CraftWebBuildReceipt{}, err
		}
		return CraftWebBuildReceipt{}, fmt.Errorf("%w: %v", ErrCraftWebBuildReceiptUnavailable, created.Error)
	}
	if created.RowsAffected == 1 {
		return receipt, nil
	}
	prior, err := r.readRow(ctx, receipt.Key())
	if err != nil {
		if errors.Is(err, ErrCraftWebBuildReceiptNotFound) {
			return CraftWebBuildReceipt{}, ErrCraftWebBuildReceiptConflict
		}
		return CraftWebBuildReceipt{}, err
	}
	if !sameCraftWebBuildReceipt(prior, receipt) {
		return CraftWebBuildReceipt{}, ErrCraftWebBuildReceiptConflict
	}
	return prior, nil
}

// SealCandidateManifest latches the collector's candidate manifest digest
// into the attempt's receipt. It is the only mutation the schema permits:
// the empty seal fills exactly once (a different digest is a conflict — the
// output mutated after the build) and OutputComplete flips to its truthful
// sealed value (complete only when the exec transport was complete); every
// process fact stays frozen. A missing receipt reports NotFound.
func (r *CraftWebBuildReceiptRepository) SealCandidateManifest(ctx context.Context, key CraftWebBuildReceiptKey, manifestDigest string) (CraftWebBuildReceipt, error) {
	if r == nil || r.db == nil {
		return CraftWebBuildReceipt{}, ErrCraftWebBuildReceiptUnavailable
	}
	if err := validateCraftWebBuildReceiptKey(key); err != nil {
		return CraftWebBuildReceipt{}, err
	}
	if !craftWebBuildReceiptSHA256Pattern.MatchString(manifestDigest) {
		return CraftWebBuildReceipt{}, ErrCraftWebBuildReceiptInvalid
	}
	prior, err := r.readRow(ctx, key)
	if err != nil {
		return CraftWebBuildReceipt{}, err
	}
	if prior.CandidateManifestSHA256 == manifestDigest {
		return prior, nil
	}
	if prior.CandidateManifestSHA256 != "" {
		return CraftWebBuildReceipt{}, ErrCraftWebBuildReceiptConflict
	}
	updated := r.db.WithContext(ctx).Model(&craftWebBuildReceiptRow{}).
		Where("tenant_id = ? AND task_id = ? AND session_id = ? AND workspace_id = ? AND run_id = ? AND activity_key = ? AND request_sha256 = ? AND candidate_manifest_sha256 = ''",
			key.TenantID, key.TaskID, key.SessionID, key.WorkspaceID, key.RunID, key.ActivityKey, key.RequestSHA256).
		Updates(map[string]any{"candidate_manifest_sha256": manifestDigest, "output_complete": prior.TransportComplete})
	if updated.Error != nil {
		if err := ctx.Err(); err != nil {
			return CraftWebBuildReceipt{}, err
		}
		return CraftWebBuildReceipt{}, fmt.Errorf("%w: %v", ErrCraftWebBuildReceiptUnavailable, updated.Error)
	}
	if updated.RowsAffected == 0 {
		sealed, err := r.readRow(ctx, key)
		if err != nil {
			return CraftWebBuildReceipt{}, err
		}
		if sealed.CandidateManifestSHA256 != manifestDigest {
			return CraftWebBuildReceipt{}, ErrCraftWebBuildReceiptConflict
		}
		return sealed, nil
	}
	return r.readRow(ctx, key)
}

func validateCraftWebBuildReceiptKey(key CraftWebBuildReceiptKey) error {
	if key.TenantID == 0 || strings.TrimSpace(key.TaskID) == "" || strings.TrimSpace(key.SessionID) == "" ||
		strings.TrimSpace(key.WorkspaceID) == "" || strings.TrimSpace(key.RunID) == "" || strings.TrimSpace(key.ActivityKey) == "" ||
		!craftWebBuildReceiptSHA256Pattern.MatchString(key.RequestSHA256) {
		return ErrCraftWebBuildReceiptInvalid
	}
	return nil
}

// Read returns a receipt only for the complete tenant/task/session/workspace/
// Run/activity/request identity. A different request digest cannot inherit
// the attempt's server-observed process outcome.
func (r *CraftWebBuildReceiptRepository) Read(ctx context.Context, key CraftWebBuildReceiptKey) (CraftWebBuildReceipt, error) {
	if r == nil || r.db == nil {
		return CraftWebBuildReceipt{}, ErrCraftWebBuildReceiptUnavailable
	}
	if err := validateCraftWebBuildReceiptKey(key); err != nil {
		return CraftWebBuildReceipt{}, err
	}
	return r.readRow(ctx, key)
}

func (r *CraftWebBuildReceiptRepository) readRow(ctx context.Context, key CraftWebBuildReceiptKey) (CraftWebBuildReceipt, error) {
	var row craftWebBuildReceiptRow
	err := r.db.WithContext(ctx).Where(
		"tenant_id = ? AND task_id = ? AND session_id = ? AND workspace_id = ? AND run_id = ? AND activity_key = ? AND request_sha256 = ?",
		key.TenantID, key.TaskID, key.SessionID, key.WorkspaceID, key.RunID, key.ActivityKey, key.RequestSHA256,
	).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return CraftWebBuildReceipt{}, ErrCraftWebBuildReceiptNotFound
	}
	if err != nil {
		if cancelErr := ctx.Err(); cancelErr != nil {
			return CraftWebBuildReceipt{}, cancelErr
		}
		return CraftWebBuildReceipt{}, fmt.Errorf("%w: %v", ErrCraftWebBuildReceiptUnavailable, err)
	}
	return row.receipt(), nil
}

func (row craftWebBuildReceiptRow) receipt() CraftWebBuildReceipt {
	return CraftWebBuildReceipt{
		TenantID: row.TenantID, TaskID: row.TaskID, SessionID: row.SessionID, WorkspaceID: row.WorkspaceID,
		RunID: row.RunID, ActivityKey: row.ActivityKey, RequestSHA256: row.RequestSHA256,
		CommandSHA256: row.CommandSHA256, RuntimeDigest: row.RuntimeDigest, ToolchainDigest: row.ToolchainDigest,
		TemplateVersion: row.TemplateVersion, TemplateSHA256: row.TemplateSHA256, TimeoutMillis: row.TimeoutMillis,
		OutputLimit: row.OutputLimit, Provider: row.Provider, ContainerID: row.ContainerID, ExecID: row.ExecID,
		ProcessState: row.ProcessState, ExitCode: row.ExitCode, Started: row.Started,
		TransportComplete: row.TransportComplete, OutputComplete: row.OutputComplete,
		OutputGeneration: row.OutputGeneration, CandidateManifestSHA256: row.CandidateManifestSHA256,
		ObservedAt: row.ObservedAt.UTC().Round(time.Microsecond),
	}
}

func craftWebBuildReceiptRowFromReceipt(receipt CraftWebBuildReceipt) craftWebBuildReceiptRow {
	return craftWebBuildReceiptRow{
		TenantID: receipt.TenantID, TaskID: receipt.TaskID, SessionID: receipt.SessionID,
		WorkspaceID: receipt.WorkspaceID, RunID: receipt.RunID, ActivityKey: receipt.ActivityKey,
		RequestSHA256: receipt.RequestSHA256, CommandSHA256: receipt.CommandSHA256, RuntimeDigest: receipt.RuntimeDigest,
		ToolchainDigest: receipt.ToolchainDigest, TemplateVersion: receipt.TemplateVersion, TemplateSHA256: receipt.TemplateSHA256,
		TimeoutMillis: receipt.TimeoutMillis, OutputLimit: receipt.OutputLimit,
		Provider: receipt.Provider, ContainerID: receipt.ContainerID, ExecID: receipt.ExecID,
		ProcessState: receipt.ProcessState, ExitCode: receipt.ExitCode, Started: receipt.Started,
		TransportComplete: receipt.TransportComplete, OutputComplete: receipt.OutputComplete,
		OutputGeneration: receipt.OutputGeneration, CandidateManifestSHA256: receipt.CandidateManifestSHA256,
		ObservedAt: receipt.ObservedAt,
	}
}

func validateCraftWebBuildReceipt(receipt CraftWebBuildReceipt) error {
	if receipt.TenantID == 0 || !craftWebReceiptText(receipt.TaskID, 128) || !craftWebReceiptText(receipt.SessionID, 64) ||
		!craftWebReceiptText(receipt.WorkspaceID, 64) || !craftWebReceiptText(receipt.RunID, 64) ||
		!craftWebReceiptText(receipt.ActivityKey, 128) || !craftWebBuildReceiptSHA256Pattern.MatchString(receipt.RequestSHA256) ||
		!craftWebBuildReceiptSHA256Pattern.MatchString(receipt.CommandSHA256) || !craftWebReceiptText(receipt.RuntimeDigest, 256) ||
		!craftWebBuildReceiptSHA256Pattern.MatchString(receipt.ToolchainDigest) || !craftWebReceiptText(receipt.TemplateVersion, 128) ||
		!craftWebBuildReceiptSHA256Pattern.MatchString(receipt.TemplateSHA256) || receipt.TimeoutMillis <= 0 || receipt.OutputLimit <= 0 ||
		receipt.Provider != "docker" || !craftWebReceiptText(receipt.ContainerID, 256) || !craftWebReceiptText(receipt.ExecID, 256) ||
		(receipt.ProcessState != "succeeded" && receipt.ProcessState != "failed" && receipt.ProcessState != "unknown") ||
		receipt.ObservedAt.IsZero() || !craftWebReceiptText(receipt.OutputGeneration, 128) {
		return ErrCraftWebBuildReceiptInvalid
	}
	if receipt.ExitCode != nil && (*receipt.ExitCode < 0 || *receipt.ExitCode > 255) {
		return ErrCraftWebBuildReceiptInvalid
	}
	if receipt.ProcessState == "succeeded" && (!receipt.Started || receipt.ExitCode == nil || *receipt.ExitCode != 0) {
		return ErrCraftWebBuildReceiptInvalid
	}
	if receipt.ProcessState == "failed" && (!receipt.Started || receipt.ExitCode == nil || *receipt.ExitCode == 0) {
		return ErrCraftWebBuildReceiptInvalid
	}
	if receipt.CandidateManifestSHA256 != "" && !craftWebBuildReceiptSHA256Pattern.MatchString(receipt.CandidateManifestSHA256) {
		return ErrCraftWebBuildReceiptInvalid
	}
	if receipt.OutputComplete && (!receipt.TransportComplete || receipt.CandidateManifestSHA256 == "" || receipt.OutputGeneration == "") {
		return ErrCraftWebBuildReceiptInvalid
	}
	return nil
}

func craftWebReceiptText(value string, max int) bool {
	return value != "" && value == strings.TrimSpace(value) && len(value) <= max && !strings.ContainsRune(value, '\x00')
}

func sameCraftWebBuildReceipt(left, right CraftWebBuildReceipt) bool {
	left.ObservedAt = left.ObservedAt.UTC().Round(time.Microsecond)
	right.ObservedAt = right.ObservedAt.UTC().Round(time.Microsecond)
	if left.ExitCode == nil || right.ExitCode == nil {
		if left.ExitCode != right.ExitCode {
			return false
		}
	} else if *left.ExitCode != *right.ExitCode {
		return false
	}
	left.ExitCode, right.ExitCode = nil, nil
	return left == right
}
