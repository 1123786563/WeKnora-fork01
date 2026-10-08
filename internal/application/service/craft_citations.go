// Package service - Craft web citations (T06, #125).
//
// The citation service is the evidence-backed citation authority of the web
// artifact: it admits a staged Run output's citation manifest only when every
// fact binds to that Run's RECORDED actual sources (T05's immutable record),
// keeps model inference explicitly distinct, verifies the rendered page
// agrees with its manifest, and resolves citation opens through the existing
// fresh-authorization seam — with a non-leaking placeholder whenever the
// source is missing, revoked or inaccessible.
package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/types"
)

// CraftCitationRecordReader reads one Run's immutable actual-source record.
// The production adapter is the same store T05's Sources projection reads,
// so web citations and Workbench evidence observe identical facts.
type CraftCitationRecordReader interface {
	Load(context.Context, craft.Scope, string) (craft.KnowledgeRecord, error)
}

// CraftCitationSourceAuthorizer is the fresh source-open seam. The
// production adapter is *CraftKnowledgeService, whose AuthorizeSourceOpen
// re-checks current Task authority and the original resource's current
// visibility; a citation open never follows a stored URL.
type CraftCitationSourceAuthorizer interface {
	AuthorizeSourceOpen(context.Context, craft.Scope, string, string) (string, error)
}

// CraftCitationConfig assembles the citation service. Records is required
// and the service fails closed without it; a missing Authorizer only
// disables ResolveCitation (fail closed), never admission.
type CraftCitationConfig struct {
	Records    CraftCitationRecordReader
	Authorizer CraftCitationSourceAuthorizer
	TaskAccess craft.TaskAccessChecker
}

// CraftCitationService owns the web kind's citation admission and citation
// open resolution.
type CraftCitationService struct {
	records    CraftCitationRecordReader
	authorizer CraftCitationSourceAuthorizer
	taskAccess craft.TaskAccessChecker
}

// NewCraftCitationService validates the assembly.
func NewCraftCitationService(cfg CraftCitationConfig) (*CraftCitationService, error) {
	if cfg.Records == nil {
		return nil, errors.New("craft: citation service requires the Run source records")
	}
	return &CraftCitationService{
		records:    cfg.Records,
		authorizer: cfg.Authorizer,
		taskAccess: cfg.TaskAccess,
	}, nil
}

// CraftCitationAgreement projects how one delivered citation manifest and
// the Workbench's recorded evidence relate. FactCitationIDs are the
// manifest's fact ids in first-appearance order; RecordedCitationIDs are the
// Run's recorded source ids; UnboundCitationIDs is empty for an admitted
// manifest (admission rejects unbound facts) and is surfaced here so a
// later consumer can detect drift explicitly instead of guessing.
type CraftCitationAgreement struct {
	FactCitationIDs     []string `json:"fact_citation_ids"`
	RecordedCitationIDs []string `json:"recorded_citation_ids"`
	UnboundCitationIDs  []string `json:"unbound_citation_ids"`
}

// citationRecord loads the Run's record and enforces the server-side
// binding: the record belongs to this scope's task and this exact Run, and
// only published records are delivered evidence. These checks are the
// authority of this gate; callers never widen them with request input.
func (s *CraftCitationService) citationRecord(ctx context.Context, scope craft.Scope, runID string) (craft.KnowledgeRecord, error) {
	record, err := s.records.Load(ctx, scope, runID)
	if err != nil {
		return craft.KnowledgeRecord{}, err
	}
	if record.Scope.TenantID != scope.TenantID || record.Scope.SessionID != scope.SessionID || record.RunID != runID {
		return craft.KnowledgeRecord{}, craft.ErrForbidden
	}
	if record.PublicationState != craft.KnowledgePublicationPublished {
		return craft.KnowledgeRecord{}, craft.ErrConflict
	}
	return record, nil
}

// AdmitStagedWebCitations is the web kind's citation admission gate, called
// server-side before a staged Run output is packaged as an immutable
// candidate or version. staged carries the output-relative files exactly as
// collected. Rules, all fail-closed:
//
//   - the staged output must carry the web entry (index.html) so the
//     rendered view is verifiable;
//   - citations.json, when staged, must strictly decode and bind every fact
//     to a source this Run actually recorded (a fabricated but well-formed
//     kc_ id is refused);
//   - an inference entry never carries a citation id;
//   - the entry's rendered markers must agree with the manifest: undeclared
//     citation markers are refused, declared facts must be rendered, and
//     every inference must carry its explicit marker;
//   - without citations.json the entry must not display any citation
//     marker — an unbound fact can never pass as cited.
//
// The admitted manifest is returned for evidence correlation (digest,
// logging, version-pinned evidence).
func (s *CraftCitationService) AdmitStagedWebCitations(
	ctx context.Context,
	scope craft.Scope,
	runID string,
	staged map[string][]byte,
) (craft.WebCitationManifest, error) {
	if s == nil || s.records == nil {
		return craft.WebCitationManifest{}, craft.ErrForbidden
	}
	if scope.TenantID == 0 || scope.UserID == "" || scope.SessionID == "" || runID == "" {
		return craft.WebCitationManifest{}, fmt.Errorf("%w: incomplete citation admission request", craft.ErrInvalidInput)
	}
	record, err := s.citationRecord(ctx, scope, runID)
	if err != nil {
		return craft.WebCitationManifest{}, err
	}
	entryPath, defined := craft.EntryPath(craft.KindWeb)
	if !defined {
		return craft.WebCitationManifest{}, fmt.Errorf("%w: web kind has no entry path", craft.ErrInvalidInput)
	}
	entry, ok := staged[entryPath]
	if !ok {
		return craft.WebCitationManifest{}, fmt.Errorf("%w: staged web output is missing the entry %q", craft.ErrInvalidInput, entryPath)
	}
	recorded := make(map[string]struct{}, len(record.Sources))
	for _, source := range record.Sources {
		recorded[source.ID] = struct{}{}
	}
	manifest := craft.WebCitationManifest{Schema: craft.WebCitationSchema, Entries: []craft.WebCitationEntry{}}
	if raw, stagedManifest := staged[craft.WebCitationsPath]; stagedManifest {
		manifest, err = craft.DecodeWebCitationManifest(raw)
		if err != nil {
			return craft.WebCitationManifest{}, err
		}
	}
	if err := craft.ValidateWebCitationManifest(manifest, recorded); err != nil {
		return craft.WebCitationManifest{}, err
	}
	if err := craft.ValidateWebCitationView(string(entry), manifest); err != nil {
		return craft.WebCitationManifest{}, err
	}
	return manifest, nil
}

// WorkbenchAgreement projects one delivered citation manifest against the
// same immutable record the Workbench sources panel serves, after a fresh
// TaskRead check. The web artifact's facts and the Workbench evidence table
// therefore agree by construction: an unbound fact is reported explicitly,
// never silently dropped.
func (s *CraftCitationService) WorkbenchAgreement(
	ctx context.Context,
	scope craft.Scope,
	runID string,
	manifest craft.WebCitationManifest,
) (CraftCitationAgreement, error) {
	if s == nil || s.records == nil {
		return CraftCitationAgreement{}, craft.ErrForbidden
	}
	if err := craft.RequireTaskAccess(ctx, s.taskAccess, scope, craft.TaskRead); err != nil {
		return CraftCitationAgreement{}, err
	}
	caller := types.CallerFromContext(ctx)
	if caller.TenantID != scope.TenantID || caller.UserID != scope.UserID {
		return CraftCitationAgreement{}, craft.ErrForbidden
	}
	record, err := s.citationRecord(ctx, scope, runID)
	if err != nil {
		return CraftCitationAgreement{}, err
	}
	recorded := make(map[string]struct{}, len(record.Sources))
	agreement := CraftCitationAgreement{FactCitationIDs: []string{}, RecordedCitationIDs: []string{}, UnboundCitationIDs: []string{}}
	for _, source := range record.Sources {
		recorded[source.ID] = struct{}{}
		agreement.RecordedCitationIDs = append(agreement.RecordedCitationIDs, source.ID)
	}
	seen := make(map[string]struct{}, len(manifest.Entries))
	for _, entry := range manifest.Entries {
		if entry.Kind != craft.WebCitationFact {
			continue
		}
		if _, dup := seen[entry.CitationID]; dup {
			continue
		}
		seen[entry.CitationID] = struct{}{}
		agreement.FactCitationIDs = append(agreement.FactCitationIDs, entry.CitationID)
		if _, ok := recorded[entry.CitationID]; !ok {
			agreement.UnboundCitationIDs = append(agreement.UnboundCitationIDs, entry.CitationID)
		}
	}
	return agreement, nil
}

// ResolveCitation opens one citation id for a current viewer. Task-level
// authority is checked first (a viewer without open_source gets a
// permission error, not a placeholder); the citation must be one of the
// Run's recorded sources — a fabricated id never resolves — and the source
// open then goes through the existing fresh-authorization seam. A source
// that is missing, revoked or inaccessible keeps a NON-LEAKING placeholder:
// only the stable citation id and the unavailable status, never the ref,
// digest or excerpt.
func (s *CraftCitationService) ResolveCitation(
	ctx context.Context,
	scope craft.Scope,
	runID, citationID string,
) (craft.WebCitationOpen, error) {
	if s == nil || s.records == nil {
		return craft.WebCitationOpen{}, craft.ErrForbidden
	}
	if err := craft.RequireTaskAccess(ctx, s.taskAccess, scope, craft.TaskOpenSource); err != nil {
		return craft.WebCitationOpen{}, err
	}
	caller := types.CallerFromContext(ctx)
	if caller.TenantID != scope.TenantID || caller.UserID != scope.UserID {
		return craft.WebCitationOpen{}, craft.ErrForbidden
	}
	record, err := s.citationRecord(ctx, scope, runID)
	if err != nil {
		return craft.WebCitationOpen{}, err
	}
	recorded := false
	for _, source := range record.Sources {
		if source.ID == citationID {
			recorded = true
			break
		}
	}
	if !recorded {
		return craft.WebCitationOpen{}, fmt.Errorf("%w: citation %q is not a recorded source of this Run", craft.ErrNotFound, citationID)
	}
	if s.authorizer == nil {
		return craft.WebCitationOpen{}, craft.ErrForbidden
	}
	ref, err := s.authorizer.AuthorizeSourceOpen(ctx, scope, runID, citationID)
	switch {
	case err == nil:
		open := craft.WebCitationOpen{Ref: ref}
		return open, open.Validate()
	case errors.Is(err, craft.ErrForbidden), errors.Is(err, craft.ErrNotFound):
		// Source-level revocation or disappearance: keep the citable
		// placeholder, leak nothing about the original.
		open := craft.WebCitationOpen{Placeholder: &craft.WebCitationPlaceholder{
			CitationID: citationID,
			Kind:       craft.WebCitationFact,
			Status:     craft.WebCitationStatusUnavailable,
		}}
		return open, open.Validate()
	default:
		return craft.WebCitationOpen{}, err
	}
}
