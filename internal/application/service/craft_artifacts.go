// Package service - Craft artifact collection and private Run candidates.
//
// Legacy callers may still publish a finished delegation's output as an
// immutable craft.Version. RunView callers instead pass a per-Run source and
// persist a private candidate, without touching VersionStore. Both routes
// share bounded path/list/read/manifest validation and upload content-addressed
// bytes before creating a durable row.
package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"mime"
	"path"
	"sort"
	"strings"

	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/skills"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/Tencent/WeKnora/internal/modules/execution/sandbox"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// Collection caps. Defaults mirror the artifact collector's per-file bound;
// the total bound keeps one runaway round from exhausting process memory.
const (
	defaultCraftMaxArtifactFileBytes  int64 = 50 << 20
	defaultCraftMaxTotalArtifactBytes int64 = 200 << 20
)

// CraftArtifactConfig bounds one collection round. Every field has a safe
// zero-value default applied by withDefaults.
type CraftArtifactConfig struct {
	// Kind is the artwork kind this service collects for; it selects the
	// entry-check rule. Defaults to craft.KindWeb until further kinds are
	// gated in (O05).
	Kind string

	// OutputDir is the sandbox directory scanned for artifact files. Empty
	// falls back to skills.ArtifactOutputDir() — the same directory the
	// runtime tells skill scripts to write to.
	OutputDir string

	// MaxFileBytes caps one artifact file, checked against both the listed
	// size and the actually-read bytes.
	MaxFileBytes int64

	// MaxTotalBytes caps the summed bytes of one round.
	MaxTotalBytes int64

	// WebCitationGate is the T06 (#125) evidence-backed citation admission
	// port. When present and the collected kind is web, every staged round
	// passes AdmitStagedWebCitations before any byte is uploaded: facts
	// must bind to sources this Run actually recorded, inferences must be
	// explicitly marked, and fabricated citation ids are refused. nil keeps
	// the unwired legacy behavior (fail-open for citation evidence, the
	// recorded pre-integration state).
	WebCitationGate WebCitationGate
}

// WebCitationGate admits one staged web round's citation manifest. The
// production implementation is CraftCitationService; the seam exists so the
// artifact collector (T07 ownership) can consume the citation service (T06
// ownership) without either owning the other's files.
type WebCitationGate interface {
	AdmitStagedWebCitations(ctx context.Context, scope craft.Scope, runID string, staged map[string][]byte) (craft.WebCitationManifest, error)
}

func (c CraftArtifactConfig) withDefaults() CraftArtifactConfig {
	if strings.TrimSpace(c.Kind) == "" {
		c.Kind = craft.KindWeb
	}
	if strings.TrimSpace(c.OutputDir) == "" {
		c.OutputDir = skills.ArtifactOutputDir()
	}
	if c.MaxFileBytes <= 0 {
		c.MaxFileBytes = defaultCraftMaxArtifactFileBytes
	}
	if c.MaxTotalBytes <= 0 {
		c.MaxTotalBytes = defaultCraftMaxTotalArtifactBytes
	}
	return c
}

// ArtifactEvidenceSource supplies the externally observed verification facts
// for one delegation: the build step's exit code and, later, W02's preview
// verdict. It may be nil — the collector then reports those checks as
// not_run instead of guessing. Whatever it returns is recorded verbatim;
// facts that never happened are never fabricated here.
type ArtifactEvidenceSource func(ctx context.Context, task craft.Task) craft.ArtifactEvidence

// WebPageLoadProbe supplies T14's externally observed preview facts for one
// private candidate: whether the controlled preview origin was reachable and
// whether an actual browser loaded the page. The two facts are independent —
// a probe that only answered the HTTP request reports reachability and
// leaves the page load not_run, and no implementation may infer one from the
// other. Facts that were not observed are reported not_run, never fabricated.
type WebPageLoadProbe interface {
	ProbeWebPage(ctx context.Context, scope craft.Scope, candidate craft.Candidate) (reachable, loaded craft.CheckOutcome)
}

// CraftArtifactService collects one delegation's workspace output into an
// immutable, run-linked version.
type CraftArtifactService struct {
	source     SandboxArtifactSource
	files      interfaces.FileService
	versions   craft.VersionStore
	candidates craft.CandidateStore
	evidence   ArtifactEvidenceSource
	config     CraftArtifactConfig
	// drafts owns the Workspace revision fence of the T15 promotion gate.
	drafts craft.DraftHeadStore
	// webProbe supplies the externally observed reachability/page-load facts.
	// Nil leaves both facts not_run and the promotion gate refuses.
	webProbe WebPageLoadProbe
}

// RunBoundSandboxArtifactSource identifies the verified generation it reads.
// Task and generation identity are constructed by the server-side source
// factory; callers must not supply values derived from client/model input.
type RunBoundSandboxArtifactSource interface {
	SandboxArtifactSource
	CraftArtifactRunID() string
	CraftArtifactGeneration() string
}

// NewCraftArtifactService assembles the artifact service from the
// session-bound sandbox source, the tenant resource storage and the version
// store. evidence may be nil.
func NewCraftArtifactService(
	source SandboxArtifactSource,
	files interfaces.FileService,
	versions craft.VersionStore,
	evidence ArtifactEvidenceSource,
	config CraftArtifactConfig,
) *CraftArtifactService {
	if source == nil || files == nil || versions == nil {
		panic("craft: NewCraftArtifactService requires a source, file service and version store")
	}
	return &CraftArtifactService{
		source:   source,
		files:    files,
		versions: versions,
		evidence: evidence,
		config:   config.withDefaults(),
	}
}

// NewCraftArtifactServiceWithCandidates adds the private candidate writer
// while retaining the legacy VersionStore route for CollectForKind callers.
func NewCraftArtifactServiceWithCandidates(
	source SandboxArtifactSource,
	files interfaces.FileService,
	versions craft.VersionStore,
	candidates craft.CandidateStore,
	evidence ArtifactEvidenceSource,
	config CraftArtifactConfig,
) *CraftArtifactService {
	if candidates == nil {
		panic("craft: NewCraftArtifactServiceWithCandidates requires a candidate store")
	}
	s := NewCraftArtifactService(source, files, versions, evidence, config)
	s.candidates = candidates
	return s
}

// stagedArtifact is one accepted output file with its bytes in memory,
// already validated against the collection rules.
type stagedArtifact struct {
	rel  string
	data []byte
}

// WithWebPromotion wires the T15 four-check promotion dependencies: the
// Workspace draft-head store that owns the revision fence and the externally
// observed page probe. Both may be nil — promotion then fails closed: a
// missing draft-head store refuses the whole promotion (the revision fence
// cannot be verified), and a missing probe leaves both page facts not_run so
// the gate refuses on the incomplete evidence.
func (s *CraftArtifactService) WithWebPromotion(drafts craft.DraftHeadStore, probe WebPageLoadProbe) *CraftArtifactService {
	if s == nil {
		return s
	}
	s.drafts, s.webProbe = drafts, probe
	return s
}

// PromoteWebVersion is T15's four-check release gate: promote one Run's
// private web candidate to an immutable published version — eligible for the
// default preview seat — only after build, entry, preview reachability and
// actual page load each independently passed.
//
// The candidate's own collected evidence supplies the build and entry facts;
// the probe supplies the two externally observed page facts. Any failed or
// not-run fact refuses the promotion with ErrConflict, publishes nothing and
// leaves the generated files a private Workspace draft — the prior default
// version keeps the seat (see SelectDefaultVersion).
//
// Binding: the request must name the candidate's own Workspace and Run, and
// carry the Workspace revision the candidate was captured at; the candidate
// Version identity re-derives from the candidate's manifest. A stale
// revision callback — the workspace advanced past it — is a conflict, and a
// replayed identical callback adopts the already published version: the
// identity derives from content, so duplicate callbacks cannot create
// duplicate versions.
func (s *CraftArtifactService) PromoteWebVersion(ctx context.Context, scope craft.Scope, req craft.WebPromotionRequest) (craft.Version, error) {
	if s == nil || s.files == nil || s.versions == nil || s.candidates == nil {
		return craft.Version{}, fmt.Errorf("%w: artifact service is not assembled for promotion", craft.ErrInvalidInput)
	}
	if scope.TenantID == 0 || scope.UserID == "" || scope.SessionID == "" {
		return craft.Version{}, fmt.Errorf("%w: promotion requires a complete scope", craft.ErrInvalidInput)
	}
	if err := req.Validate(); err != nil {
		return craft.Version{}, err
	}
	if s.drafts == nil {
		return craft.Version{}, fmt.Errorf("%w: promotion requires the Workspace draft-head store (revision fence)", craft.ErrUnsupported)
	}

	// The candidate store's own scope ACL runs here: a foreign session does
	// not see the candidate at all, a foreign owner is forbidden.
	candidate, err := s.candidates.GetCandidate(ctx, scope, req.CandidateID)
	if err != nil {
		return craft.Version{}, err
	}
	if candidate.WorkspaceID != req.WorkspaceID || candidate.RunID != req.RunID {
		return craft.Version{}, fmt.Errorf("%w: promotion request does not bind candidate %s", craft.ErrConflict, candidate.ID)
	}
	if candidate.Kind != craft.KindWeb {
		return craft.Version{}, fmt.Errorf("%w: the four-check gate promotes web versions only, got kind %q", craft.ErrInvalidInput, candidate.Kind)
	}

	// Revision fence: the callback's revision must be the Workspace's current
	// draft-head revision. A late callback for a superseded revision is a
	// conflict, never a silent promotion of stale files.
	head, err := s.drafts.Read(ctx, scope, candidate.WorkspaceID)
	if err != nil {
		return craft.Version{}, err
	}
	if head.Revision != req.Revision {
		return craft.Version{}, fmt.Errorf("%w: stale workspace revision %d (head is %d) for run %s", craft.ErrConflict, req.Revision, head.Revision, candidate.RunID)
	}

	// The four facts, each from its own observation source.
	evidence := craft.WebCheckEvidence{
		Build: craft.WebBuildOutcome(candidate.Evidence),
		Entry: craft.WebEntryOutcome(candidate.Kind, candidate.Files),
	}
	if s.webProbe != nil {
		evidence.PreviewReachable, evidence.PageLoaded = s.webProbe.ProbeWebPage(ctx, scope, candidate)
	} // a missing probe leaves both page facts not_run — the gate refuses below
	if err := evidence.Validate(); err != nil {
		return craft.Version{}, err
	}
	versionID := craft.VersionID(candidate.WorkspaceID, candidate.RunID, candidate.ManifestDigest)
	record := craft.WebPromotionRecord{
		RunID: candidate.RunID, Revision: req.Revision, VersionID: versionID,
		WebCheckEvidence: evidence,
	}
	if !record.Promotable() {
		return craft.Version{}, fmt.Errorf(
			"%w: web promotion refused: build=%s entry=%s preview_reachable=%s page_loaded=%s (run %s, workspace revision %d, version %s)",
			craft.ErrConflict, evidence.Build, evidence.Entry, evidence.PreviewReachable, evidence.PageLoaded,
			record.RunID, record.Revision, record.VersionID,
		)
	}

	published, err := s.versions.Publish(ctx, scope, craft.Version{
		ID: versionID, WorkspaceID: candidate.WorkspaceID, RunID: candidate.RunID,
		Kind: craft.KindWeb, Files: candidate.Files, Checks: craft.WebChecks(record),
	})
	if err != nil {
		logger.Warnf(ctx, "[CraftArtifact] web promotion publish failed for run %s: %v", candidate.RunID, err)
		return craft.Version{}, err
	}
	// The store persists the four checks; the returned projection carries the
	// same evidence derived from them so callers (and the DTO) never have to.
	out := published
	out.WebEvidence = &evidence
	logger.Infof(ctx, "[CraftArtifact] promoted web version %s for run %s at workspace revision %d",
		published.ID, candidate.RunID, req.Revision)
	return out, nil
}

// SelectDefaultVersion projects the workspace's default preview version
// under the T15 policy: the newest published version whose four web checks
// each independently passed. Versions with failed, not-run or missing facts
// never take the seat, so a refused or unverified round keeps the prior
// default exactly where it was.
func (s *CraftArtifactService) SelectDefaultVersion(ctx context.Context, scope craft.Scope) (craft.Version, bool, error) {
	if s == nil || s.versions == nil {
		return craft.Version{}, false, fmt.Errorf("%w: artifact service is not assembled", craft.ErrInvalidInput)
	}
	versions, err := s.versions.List(ctx, scope)
	if err != nil {
		return craft.Version{}, false, err
	}
	selected, ok := craft.SelectDefaultVersion(versions)
	if !ok {
		return craft.Version{}, false, nil
	}
	evidence := craft.WebEvidenceFromChecks(selected.Checks)
	selected.WebEvidence = &evidence
	return selected, true, nil
}

// Collect reads the delegation's workspace output and publishes it as one
// immutable version, using the service's configured kind. Delegations whose
// session kind is known at call time use CollectForKind instead.
//
// Ordering is the W01 contract:
//
//  1. every listed entry is validated (regular file only — symlinks abort,
//     no traversal, no credentials, per-file and total caps) and read;
//  2. every file is uploaded through the resource storage BEFORE any
//     version row exists, so a visible version always has all its objects;
//     objects stranded by a later failure are left for O03's deferred
//     reclamation;
//  3. the version — identity derived from the content manifest — publishes
//     through the store in one transaction. Only a published version is ever
//     associated with tool results, via its RunID.
//
// A failed collection returns an error and leaves no version visible.
func (s *CraftArtifactService) Collect(ctx context.Context, task craft.Task) (craft.Version, error) {
	return s.CollectForKind(ctx, task, s.config.Kind)
}

// CollectForKind is the kind-aware collection entrance (D01 wiring): the
// version is stamped with the SESSION's kind, the entry check judges that
// kind's own deliverable, and — for the kinds whose skill writes a
// manifest.json — the manifest is decoded and validated server-side BEFORE
// anything publishes. A skill cannot fake its way past the gate by writing
// checks: "passed" inside a structurally invalid manifest publishes nothing.
func (s *CraftArtifactService) CollectForKind(ctx context.Context, task craft.Task, kind string) (craft.Version, error) {
	if s == nil || s.source == nil || s.files == nil || s.versions == nil {
		return craft.Version{}, fmt.Errorf("%w: artifact service is not assembled", craft.ErrInvalidInput)
	}
	if !craft.KnownKind(kind) {
		return craft.Version{}, fmt.Errorf("%w: unknown artifact kind %q", craft.ErrInvalidInput, kind)
	}
	if task.Scope.TenantID == 0 || task.Scope.UserID == "" || task.Scope.SessionID == "" ||
		task.WorkspaceID == "" || task.Fence.RunID == "" {
		return craft.Version{}, fmt.Errorf("%w: incomplete delegation request", craft.ErrInvalidInput)
	}

	files, err := s.stageAndUpload(ctx, task, kind, s.source)
	if err != nil {
		return craft.Version{}, err
	}

	digest, err := craft.ManifestDigest(files)
	if err != nil {
		return craft.Version{}, err
	}
	evidence := craft.ArtifactEvidence{}
	if s.evidence != nil {
		evidence = s.evidence(ctx, task)
	}
	version := craft.Version{
		ID:          craft.VersionID(task.WorkspaceID, task.Fence.RunID, digest),
		WorkspaceID: task.WorkspaceID,
		RunID:       task.Fence.RunID,
		Kind:        kind,
		Files:       files,
		Checks:      craft.BuildChecks(kind, files, evidence),
	}
	published, err := s.versions.Publish(ctx, task.Scope, version)
	if err != nil {
		logger.Warnf(ctx, "[CraftArtifact] publish failed for run %s: %v (uploaded objects stay for O03 deferred reclamation)", task.Fence.RunID, err)
		return craft.Version{}, err
	}
	logger.Infof(ctx, "[CraftArtifact] published version %s workspace %s run %s files %d",
		published.ID, task.WorkspaceID, task.Fence.RunID, len(published.Files))
	return published, nil
}

// CollectCandidate stages one verified Run's output as a private immutable
// candidate. The per-call source is checked against both the task Run and the
// independently supplied server-verified generation before any list/read or
// upload. This path never calls VersionStore.Publish.
func (s *CraftArtifactService) CollectCandidate(
	ctx context.Context,
	task craft.Task,
	kind string,
	source RunBoundSandboxArtifactSource,
	verifiedGeneration string,
) (craft.Candidate, error) {
	if s == nil || s.files == nil || s.candidates == nil {
		return craft.Candidate{}, fmt.Errorf("%w: private candidate service is not assembled", craft.ErrInvalidInput)
	}
	if !craft.KnownKind(kind) {
		return craft.Candidate{}, fmt.Errorf("%w: unknown artifact kind %q", craft.ErrInvalidInput, kind)
	}
	if task.Scope.TenantID == 0 || task.Scope.UserID == "" || task.Scope.SessionID == "" ||
		task.Fence.TenantID != task.Scope.TenantID || task.Fence.RunID == "" || task.WorkspaceID == "" {
		return craft.Candidate{}, fmt.Errorf("%w: incomplete Run candidate request", craft.ErrInvalidInput)
	}
	if source == nil || strings.TrimSpace(verifiedGeneration) == "" || len(verifiedGeneration) > 128 ||
		strings.ContainsAny(verifiedGeneration, "\x00\r\n\t ") || source.CraftArtifactRunID() != task.Fence.RunID ||
		source.CraftArtifactGeneration() != verifiedGeneration {
		return craft.Candidate{}, fmt.Errorf("%w: artifact source differs from the verified Run generation", craft.ErrConflict)
	}
	files, err := s.stageAndUpload(ctx, task, kind, source)
	if err != nil {
		return craft.Candidate{}, err
	}
	digest, err := craft.ManifestDigest(files)
	if err != nil {
		return craft.Candidate{}, err
	}
	evidence := craft.ArtifactEvidence{}
	if s.evidence != nil {
		evidence = s.evidence(ctx, task)
	}
	candidate := craft.Candidate{
		ID:    craft.CandidateID(task.WorkspaceID, task.Fence.RunID, digest),
		Scope: task.Scope, WorkspaceID: task.WorkspaceID, RunID: task.Fence.RunID,
		Generation: verifiedGeneration, Kind: kind, ManifestDigest: digest,
		Files: files, Evidence: evidence, Checks: craft.BuildChecks(kind, files, evidence),
	}
	if err := candidate.Validate(task.Scope); err != nil {
		return craft.Candidate{}, err
	}
	stored, err := s.candidates.PutCandidate(ctx, task.Scope, candidate)
	if err != nil {
		logger.Warnf(ctx, "[CraftArtifact] candidate seal failed for run %s: %v (uploaded objects stay for O03 deferred reclamation)", task.Fence.RunID, err)
		return craft.Candidate{}, err
	}
	return stored, nil
}

// stageAndUpload centralizes the bounded path/list/read/manifest rules shared
// by legacy publication and private Run candidate staging.
func (s *CraftArtifactService) stageAndUpload(
	ctx context.Context,
	task craft.Task,
	kind string,
	source SandboxArtifactSource,
) ([]craft.File, error) {
	entries, err := source.ListSessionFiles(ctx, task.Scope.SessionID, s.config.OutputDir)
	if err != nil {
		return nil, fmt.Errorf("craft: list workspace output: %w", err)
	}
	staged := make([]stagedArtifact, 0, len(entries))
	var total int64
	for _, entry := range entries {
		switch entry.Type {
		case sandbox.RemoteEntryDir:
			continue
		case sandbox.RemoteEntryFile:
		default:
			return nil, fmt.Errorf("%w: output entry %q is not a regular file", craft.ErrInvalidInput, entry.Path)
		}
		rel, err := craft.ArtifactRelativePath(s.config.OutputDir, entry.Path)
		if err != nil {
			return nil, err
		}
		if entry.Size > s.config.MaxFileBytes {
			return nil, fmt.Errorf("%w: artifact %q lists %d bytes over the %d cap", craft.ErrInvalidInput, rel, entry.Size, s.config.MaxFileBytes)
		}
		data, err := source.ReadSessionFile(ctx, task.Scope.SessionID, entry.Path)
		if err != nil {
			return nil, fmt.Errorf("craft: read artifact %q: %w", rel, err)
		}
		if int64(len(data)) > s.config.MaxFileBytes {
			return nil, fmt.Errorf("%w: artifact %q read %d bytes over the %d cap", craft.ErrInvalidInput, rel, len(data), s.config.MaxFileBytes)
		}
		total += int64(len(data))
		if total > s.config.MaxTotalBytes {
			return nil, fmt.Errorf("%w: round read %d bytes over the %d total cap", craft.ErrInvalidInput, total, s.config.MaxTotalBytes)
		}
		staged = append(staged, stagedArtifact{rel: rel, data: data})
	}
	sort.Slice(staged, func(i, j int) bool { return staged[i].rel < staged[j].rel })
	if err := craftValidateStagedManifest(kind, staged); err != nil {
		logger.Warnf(ctx, "[CraftArtifact] manifest admission rejected the round: %v", err)
		return nil, err
	}
	// T06 (#125): the web kind's citation admission runs after the shape
	// gate and BEFORE any byte is uploaded, so a round with fabricated
	// citations never stages objects. The admitted manifest digest is
	// logged for evidence correlation; the gate is optional only because
	// non-web kinds and unwired assemblies keep their recorded behavior.
	if s.config.WebCitationGate != nil && kind == craft.KindWeb {
		stagedBytes := make(map[string][]byte, len(staged))
		for _, a := range staged {
			stagedBytes[a.rel] = a.data
		}
		manifest, err := s.config.WebCitationGate.AdmitStagedWebCitations(ctx, task.Scope, task.Fence.RunID, stagedBytes)
		if err != nil {
			logger.Warnf(ctx, "[CraftArtifact] citation admission rejected the round for run %s: %v", task.Fence.RunID, err)
			return nil, err
		}
		if digest, derr := craft.WebCitationsDigest(manifest); derr == nil {
			logger.Infof(ctx, "[CraftArtifact] web citations admitted for run %s: manifest digest %s", task.Fence.RunID, digest)
		}
	}
	files := make([]craft.File, 0, len(staged))
	for _, a := range staged {
		sum := sha256.Sum256(a.data)
		digest := hex.EncodeToString(sum[:])
		storageName := "craft_" + digest + "_" + safeFileName(path.Base(a.rel))
		ref, err := s.files.SaveBytes(ctx, a.data, task.Scope.TenantID, storageName, false)
		if err != nil {
			logger.Warnf(ctx, "[CraftArtifact] upload failed for %s: %v (already-uploaded objects stay for O03 deferred reclamation)", a.rel, err)
			return nil, fmt.Errorf("craft: upload artifact %q: %w", a.rel, err)
		}
		files = append(files, craft.File{Path: a.rel, Ref: ref, SHA256: digest, MIME: craftArtifactMIME(a.rel), Bytes: int64(len(a.data))})
	}
	return files, nil
}

// craftArtifactMIME reports the MIME type served for one artifact path; the
// fallback keeps unknown extensions downloadable as opaque bytes.
func craftArtifactMIME(rel string) string {
	if t := mime.TypeByExtension(path.Ext(rel)); t != "" {
		return t
	}
	return "application/octet-stream"
}

// craftManifestGateKinds lists the kinds whose skill contract is a
// manifest.json the server must admit before publishing. Web keeps the
// entry-check-only gate (its skill ships no manifest).
func craftManifestGateKind(kind string) bool {
	return kind == craft.KindDocument || kind == craft.KindSpreadsheet || kind == craft.KindSlides
}

// craftValidateStagedManifest decodes the round's output/manifest.json and
// runs the kind's own validator against it. A gated kind without a
// manifest, a manifest that is not JSON, or one that fails its kind's rules
// (wrong deliverable paths, unready artifacts, failed or missing checks,
// invented citations…) refuses the whole collection — fail closed.
func craftValidateStagedManifest(kind string, staged []stagedArtifact) error {
	if !craftManifestGateKind(kind) {
		return nil
	}
	for _, artifact := range staged {
		if artifact.rel != "manifest.json" {
			continue
		}
		switch kind {
		case craft.KindDocument:
			var m craft.DocumentManifest
			if err := json.Unmarshal(artifact.data, &m); err != nil {
				return fmt.Errorf("%w: document manifest decode: %v", craft.ErrInvalidInput, err)
			}
			if err := craft.ValidateDocumentManifest(m); err != nil {
				return err
			}
		case craft.KindSpreadsheet:
			var m craft.SpreadsheetManifest
			if err := json.Unmarshal(artifact.data, &m); err != nil {
				return fmt.Errorf("%w: spreadsheet manifest decode: %v", craft.ErrInvalidInput, err)
			}
			if err := craft.ValidateSpreadsheetManifest(m); err != nil {
				return err
			}
		case craft.KindSlides:
			var m craft.SlideManifest
			if err := json.Unmarshal(artifact.data, &m); err != nil {
				return fmt.Errorf("%w: slides manifest decode: %v", craft.ErrInvalidInput, err)
			}
			if err := craft.ValidateSlidesManifest(m); err != nil {
				return err
			}
		}
		return nil
	}
	return fmt.Errorf("%w: kind %q requires output/manifest.json (gate checks) for admission; none was collected", craft.ErrInvalidInput, kind)
}
