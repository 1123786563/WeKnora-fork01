package craft

import (
	"context"
	"fmt"
)

// TaskAccessChecker is injected by the membership lane. The session ID in
// Scope is the Task ID; callers must not substitute a second aggregate ID.
// The checker owns current membership and permission policy, including fresh
// authorization on reads of restricted originals.
type TaskAccessChecker interface {
	CheckTaskAccess(context.Context, Scope, TaskAction) error
}

// CraftTaskLookup classifies a durable session from its tenant-scoped
// registration row. It is independent of actor grants and request snapshots:
// snapshots can predate the typed Craft manifest, while grant checks cannot
// distinguish a non-Craft session from an inaccessible Craft task.
type CraftTaskLookup interface {
	IsCraftTask(context.Context, uint64, string) (bool, error)
}

// TaskRunAccess is the worker's complete authority seam: authoritative task
// classification plus a live action check for registered Craft tasks.
type TaskRunAccess interface {
	TaskAccessChecker
	CraftTaskLookup
}

type TaskAction string

// Valid reports whether the action belongs to the frozen five-action
// vocabulary. It is the single authority both RequireTaskAccess and the
// denial-audit vocabulary switch consume, so a new action cannot be added
// to one and silently skipped by the other.
func (a TaskAction) Valid() bool {
	switch a {
	case TaskRead, TaskWrite, TaskShare, TaskOpenSource, TaskPreview:
		return true
	}
	return false
}

const (
	TaskRead       TaskAction = "read"
	TaskWrite      TaskAction = "write"
	TaskShare      TaskAction = "share"
	TaskOpenSource TaskAction = "open_source"
	TaskPreview    TaskAction = "preview"
)

// RequireTaskAccess has no permissive default when an assembly lacks T08.
func RequireTaskAccess(ctx context.Context, checker TaskAccessChecker, scope Scope, action TaskAction) error {
	if checker == nil {
		return ErrForbidden
	}
	if scope.TenantID == 0 || scope.UserID == "" || scope.SessionID == "" {
		return ErrForbidden
	}
	if !action.Valid() {
		return ErrInvalidInput
	}
	return checker.CheckTaskAccess(ctx, scope, action)
}

// These records are immutable-version and run facts. They do not grant access
// or perform a transition: the owning application service remains authoritative.
type InputRecognition struct {
	Accepted   bool   `json:"accepted"`
	Understood bool   `json:"understood"`
	Reason     string `json:"reason"`
}

func (r InputRecognition) Validate() error {
	if r.Understood && !r.Accepted {
		return fmt.Errorf("%w: understood input was not accepted", ErrInvalidInput)
	}
	if r.Accepted && !r.Understood && r.Reason == "" {
		return fmt.Errorf("%w: unrecognized input needs a reason", ErrInvalidInput)
	}
	return nil
}

type CheckOutcome string

const (
	WebCheckPassed CheckOutcome = "passed"
	WebCheckFailed CheckOutcome = "failed"
	WebCheckNotRun CheckOutcome = "not_run"
)

func (s CheckOutcome) valid() bool {
	return s == WebCheckPassed || s == WebCheckFailed || s == WebCheckNotRun
}

// Each gate is separate evidence; page load cannot be inferred from HTTP reachability.
type WebCheckEvidence struct {
	Build            CheckOutcome `json:"build"`
	Entry            CheckOutcome `json:"entry"`
	PreviewReachable CheckOutcome `json:"preview_reachable"`
	PageLoaded       CheckOutcome `json:"page_loaded"`
}

func (e WebCheckEvidence) Validate() error {
	if !e.Build.valid() || !e.Entry.valid() || !e.PreviewReachable.valid() || !e.PageLoaded.valid() {
		return fmt.Errorf("%w: invalid web check outcome", ErrInvalidInput)
	}
	return nil
}
func (e WebCheckEvidence) Ready() bool {
	return e.Validate() == nil && e.Build == WebCheckPassed && e.Entry == WebCheckPassed && e.PreviewReachable == WebCheckPassed && e.PageLoaded == WebCheckPassed
}

type StopOutcomeStatus string

const (
	StopRequested StopOutcomeStatus = "requested"
	StopConfirmed StopOutcomeStatus = "confirmed"
	StopUnknown   StopOutcomeStatus = "unknown"
)

type StopOutcome struct {
	RunID  string            `json:"run_id"`
	Status StopOutcomeStatus `json:"status"`
}

func (o StopOutcome) Validate() error {
	if o.RunID == "" || (o.Status != StopRequested && o.Status != StopConfirmed && o.Status != StopUnknown) {
		return fmt.Errorf("%w: invalid stop outcome", ErrInvalidInput)
	}
	return nil
}

type WriterAcquireStatus string

const (
	WriterAcquired WriterAcquireStatus = "acquired"
	WriterConflict WriterAcquireStatus = "conflict"
	WriterUnknown  WriterAcquireStatus = "unknown"
)

type WriterAcquireOutcome struct {
	WorkspaceID string              `json:"workspace_id"`
	Status      WriterAcquireStatus `json:"status"`
}

func (o WriterAcquireOutcome) Validate() error {
	if o.WorkspaceID == "" || (o.Status != WriterAcquired && o.Status != WriterConflict && o.Status != WriterUnknown) {
		return fmt.Errorf("%w: invalid writer acquisition", ErrInvalidInput)
	}
	return nil
}

type BudgetPause struct {
	RunID           string                 `json:"run_id"`
	Reason          string                 `json:"reason"`
	Limit           int64                  `json:"limit"`
	Used            int64                  `json:"used"`
	ExtensionAction *BudgetExtensionAction `json:"extension_action"`
}

// BudgetExtensionAction is the server-owned action identity and quantum for
// one paused Run. The client echoes these exact values when it requests an
// extension; it cannot choose a new key or amount.
type BudgetExtensionAction struct {
	Key          string `json:"key"`
	ExtraCalls   int    `json:"extra_calls"`
	ExtraCredits int64  `json:"extra_credits"`
}

func (p BudgetPause) Validate() error {
	if p.RunID == "" || p.Reason == "" || p.Limit < 0 || p.Used < 0 {
		return fmt.Errorf("%w: invalid budget pause", ErrInvalidInput)
	}
	if a := p.ExtensionAction; a != nil && (a.Key == "" || a.ExtraCalls <= 0 || a.ExtraCredits <= 0) {
		return fmt.Errorf("%w: invalid budget extension action", ErrInvalidInput)
	}
	return nil
}

type RestrictedContribution struct {
	VersionID      string `json:"version_id"`
	EvidenceDigest string `json:"evidence_digest"`
	Restricted     bool   `json:"restricted"`
}

func (c RestrictedContribution) Validate() error {
	if c.VersionID == "" || c.EvidenceDigest == "" {
		return fmt.Errorf("%w: invalid restricted contribution", ErrInvalidInput)
	}
	return nil
}

type DecisionStatus string

const (
	DecisionApproved DecisionStatus = "approved"
	DecisionRejected DecisionStatus = "rejected"
	DecisionUnknown  DecisionStatus = "unknown"
)

func (d DecisionStatus) valid() bool {
	return d == DecisionApproved || d == DecisionRejected || d == DecisionUnknown
}

type ShareDecision struct {
	VersionID      string         `json:"version_id"`
	EvidenceDigest string         `json:"evidence_digest"`
	OwnerID        string         `json:"owner_id"`
	Decision       DecisionStatus `json:"decision"`
}

func (d ShareDecision) Validate() error {
	if d.VersionID == "" || d.EvidenceDigest == "" || d.OwnerID == "" || !d.Decision.valid() {
		return fmt.Errorf("%w: invalid share decision", ErrInvalidInput)
	}
	return nil
}

type ExportFile struct {
	Path       string            `json:"path"`
	SHA256     string            `json:"sha256"`
	Restricted bool              `json:"restricted"`
	Origins    []ExportOriginRef `json:"origins"`
}

// ExportOriginRef identifies one immutable input or evidence item that
// contributed to a derived file. Ref is an opaque identity, never a reusable
// provider URL or a grant to open the original material.
type ExportOriginKind string

const (
	ExportOriginKnowledge ExportOriginKind = "knowledge"
	ExportOriginInput     ExportOriginKind = "input"
	ExportOriginArtifact  ExportOriginKind = "artifact"
)

type ExportOriginRef struct {
	Kind       ExportOriginKind `json:"kind"`
	Ref        string           `json:"ref"`
	SHA256     string           `json:"sha256"`
	Restricted bool             `json:"restricted"`
}

func (o ExportOriginRef) Validate() error {
	if o.Kind != ExportOriginKnowledge && o.Kind != ExportOriginInput && o.Kind != ExportOriginArtifact {
		return fmt.Errorf("%w: invalid export origin kind", ErrInvalidInput)
	}
	if o.Ref == "" || o.SHA256 == "" {
		return fmt.Errorf("%w: incomplete export origin", ErrInvalidInput)
	}
	return nil
}

type ExportManifest struct {
	VersionID      string       `json:"version_id"`
	ManifestDigest string       `json:"manifest_digest"`
	Files          []ExportFile `json:"files"`
}

func (m ExportManifest) Validate() error {
	if m.VersionID == "" || m.ManifestDigest == "" {
		return fmt.Errorf("%w: invalid export manifest", ErrInvalidInput)
	}
	for _, f := range m.Files {
		if f.Path == "" || f.SHA256 == "" || f.Origins == nil {
			return fmt.Errorf("%w: invalid export file", ErrInvalidInput)
		}
		for _, origin := range f.Origins {
			if err := origin.Validate(); err != nil {
				return err
			}
		}
	}
	return nil
}

// ExportConsentView keeps the exact manifest, including file origins,
// alongside an optional owner decision bound to that manifest identity.
type ExportConsentView struct {
	Manifest ExportManifest  `json:"manifest"`
	Decision *ExportDecision `json:"decision"`
}

func (v ExportConsentView) Validate() error {
	if err := v.Manifest.Validate(); err != nil {
		return err
	}
	if v.Decision == nil {
		return nil
	}
	if err := v.Decision.Validate(); err != nil {
		return err
	}
	if v.Decision.VersionID != v.Manifest.VersionID || v.Decision.ManifestDigest != v.Manifest.ManifestDigest {
		return fmt.Errorf("%w: export decision does not bind this manifest", ErrInvalidInput)
	}
	return nil
}

type ExportDecision struct {
	VersionID      string         `json:"version_id"`
	ManifestDigest string         `json:"manifest_digest"`
	OwnerID        string         `json:"owner_id"`
	Decision       DecisionStatus `json:"decision"`
}

func (d ExportDecision) Validate() error {
	if d.VersionID == "" || d.ManifestDigest == "" || d.OwnerID == "" || !d.Decision.valid() {
		return fmt.Errorf("%w: invalid export decision", ErrInvalidInput)
	}
	return nil
}
