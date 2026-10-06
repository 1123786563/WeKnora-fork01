package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"path"
	"sort"

	"github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/Tencent/WeKnora/internal/sandbox"
)

// QuiescentRunArtifactSource is server constructed for a single bound Run and
// generation. VerifyCraftCaptureQuiescent must fail closed unless the runtime
// can prove that no sandbox writer remains. Task3 wires the real provider.
type QuiescentRunArtifactSource interface {
	RunBoundSandboxArtifactSource
	VerifyCraftCaptureQuiescent(context.Context) error
}

type craftRunCaptureStore interface {
	EnsurePending(context.Context, craft.Scope, string, string, string) (repository.CraftRunCapture, error)
	BeginCapture(context.Context, repository.CraftRunCapture, string) (repository.CraftRunCapture, error)
	ClaimForDrain(context.Context, repository.CraftRunCapture) (repository.CraftRunCapture, error)
	RecoverPending(context.Context, int) ([]repository.CraftRunCapture, error)
	RecoverPendingTick(context.Context, int) ([]repository.CraftRunCapture, error)
	RecoverPendingForRun(context.Context, uint64, string) ([]repository.CraftRunCapture, error)
	Seal(context.Context, repository.CraftRunCapture, []craft.File, string) (repository.CraftRunCapture, error)
	VerifySealedRefs(context.Context, repository.CraftRunCapture) error
	MarkAdvanced(context.Context, repository.CraftRunCapture, int64) error
	MarkPendingError(context.Context, repository.CraftRunCapture, error) error
}

// CraftRunCaptureService coordinates terminal output sealing and draft CAS.
// It has no VersionStore dependency; capture cannot change the published V1.
type CraftRunCaptureService struct {
	artifacts *CraftArtifactService
	captures  craftRunCaptureStore
	drafts    craft.DraftHeadStore
	resolve   func(context.Context, repository.CraftRunCapture) (QuiescentRunArtifactSource, error)
}

func NewCraftRunCaptureService(artifacts *CraftArtifactService, captures craftRunCaptureStore, drafts craft.DraftHeadStore,
	resolve func(context.Context, repository.CraftRunCapture) (QuiescentRunArtifactSource, error)) *CraftRunCaptureService {
	if artifacts == nil || captures == nil || drafts == nil {
		panic("craft: capture service requires artifact, receipt and draft stores")
	}
	return &CraftRunCaptureService{artifacts: artifacts, captures: captures, drafts: drafts, resolve: resolve}
}

// CaptureTerminal captures a failed, canceled, or successful terminal Run.
// Identity comes from the server-created source, and SQL independently binds
// it to the durable RunView before any output read or object upload.
func (s *CraftRunCaptureService) CaptureTerminal(ctx context.Context, scope craft.Scope, workspaceID, runID, kind string, source QuiescentRunArtifactSource) (craft.DraftHead, error) {
	if s == nil || s.captures == nil || s.drafts == nil || s.artifacts == nil || source == nil {
		return craft.DraftHead{}, fmt.Errorf("%w: capture service is not assembled", craft.ErrInvalidInput)
	}
	if !craft.KnownKind(kind) {
		return craft.DraftHead{}, fmt.Errorf("%w: unknown capture artifact kind %q", craft.ErrInvalidInput, kind)
	}
	if source.CraftArtifactRunID() != runID || source.CraftArtifactGeneration() == "" {
		return craft.DraftHead{}, fmt.Errorf("%w: source is bound to another Run or no generation", craft.ErrConflict)
	}
	receipt, err := s.captures.EnsurePending(ctx, scope, workspaceID, runID, source.CraftArtifactGeneration())
	if err != nil {
		return craft.DraftHead{}, err
	}
	return s.capture(ctx, receipt, kind, source)
}

// RecoverPending replays durable receipts. Sealed receipts always advance
// from stored refs; the source is not consulted for file bytes again.
func (s *CraftRunCaptureService) RecoverPending(ctx context.Context, limit int) error {
	if s == nil || s.resolve == nil {
		return fmt.Errorf("%w: capture recovery source resolver unavailable", craft.ErrInvalidInput)
	}
	receipts, err := s.captures.RecoverPending(ctx, limit)
	if err != nil {
		return err
	}
	return s.recoverReceipts(ctx, receipts)
}

// RecoverPendingTick runs one periodic-scan round through the store's tick
// variant (synthesis only every Nth pass, freshness gate honored).
func (s *CraftRunCaptureService) RecoverPendingTick(ctx context.Context, limit int) error {
	if s == nil || s.resolve == nil {
		return fmt.Errorf("%w: capture recovery source resolver unavailable", craft.ErrInvalidInput)
	}
	receipts, err := s.captures.RecoverPendingTick(ctx, limit)
	if err != nil {
		return err
	}
	return s.recoverReceipts(ctx, receipts)
}

// RecoverRun replays only the receipts of one Run. The post-terminal drain
// uses it so an unrelated Run's completion never pays for a global recovery
// pass; the periodic scan keeps the global pass, including missing-receipt
// synthesis for runs whose enqueue trigger was lost.
func (s *CraftRunCaptureService) RecoverRun(ctx context.Context, tenantID uint64, runID string) error {
	if s == nil || s.resolve == nil {
		return fmt.Errorf("%w: capture recovery source resolver unavailable", craft.ErrInvalidInput)
	}
	receipts, err := s.captures.RecoverPendingForRun(ctx, tenantID, runID)
	if err != nil {
		return err
	}
	return s.recoverReceipts(ctx, receipts)
}

func (s *CraftRunCaptureService) recoverReceipts(ctx context.Context, receipts []repository.CraftRunCapture) error {
	var first error
	for _, receipt := range receipts {
		if receipt.State == "sealed" {
			adopted, adoptErr := s.adoptAdvancedRevision(ctx, receipt)
			if adoptErr == nil && adopted {
				continue
			}
			if adoptErr != nil {
				_ = s.captures.MarkPendingError(ctx, receipt, adoptErr)
				if first == nil {
					first = adoptErr
				}
				continue
			}
		}
		source, resolveErr := s.resolve(ctx, receipt)
		if resolveErr != nil {
			_ = s.captures.MarkPendingError(ctx, receipt, resolveErr)
			if first == nil {
				first = resolveErr
			}
			continue
		}
		if _, captureErr := s.capture(ctx, receipt, craft.KindWeb, source); captureErr != nil {
			_ = s.captures.MarkPendingError(ctx, receipt, captureErr)
			if first == nil {
				first = captureErr
			}
		}
	}
	return first
}

func (s *CraftRunCaptureService) capture(ctx context.Context, receipt repository.CraftRunCapture, kind string, source QuiescentRunArtifactSource) (craft.DraftHead, error) {
	if receipt.State == "advanced" && receipt.DraftRevision != nil {
		return s.drafts.ReadRevision(ctx, receipt.Scope, receipt.WorkspaceID, *receipt.DraftRevision)
	}
	if receipt.State == "sealed" {
		adopted, err := s.adoptAdvancedRevision(ctx, receipt)
		if err != nil {
			return craft.DraftHead{}, err
		}
		if adopted {
			head, err := s.drafts.ReadRevision(ctx, receipt.Scope, receipt.WorkspaceID, receipt.PredecessorRevision+1)
			return head, err
		}
	}
	if source == nil || source.CraftArtifactRunID() != receipt.RunID || source.CraftArtifactGeneration() != receipt.Generation {
		return craft.DraftHead{}, craft.ErrConflict
	}
	if receipt.State != "sealed" && receipt.State != "advanced" {
		// Claim the receipt BEFORE the expensive quiescence/staging work: the
		// freshness gate excludes CAPTURING receipts touched within the drain
		// window, so the periodic ticker cannot concurrently redo this
		// receipt while the drain walks the tree (previously the gate only
		// fired after BeginCapture, leaving the whole pre-staging phase —
		// two full sha256 walks plus a full read — unprotected).
		claimed, claimErr := s.captures.ClaimForDrain(ctx, receipt)
		if claimErr != nil {
			// Losing the claim means another path owns this receipt's
			// capture: proceeding anyway would walk the tree and upload a
			// SECOND physical object (the loser leaks as an unreferenced
			// resource row). Surface a retryable conflict — the periodic
			// ticker reconciles the winner's outcome.
			return craft.DraftHead{}, claimErr
		}
		receipt = claimed
		if err := source.VerifyCraftCaptureQuiescent(ctx); err != nil {
			_ = s.captures.MarkPendingError(ctx, receipt, err)
			return craft.DraftHead{}, fmt.Errorf("%w: sandbox quiescence is unproven: %v", craft.ErrBusy, err)
		}
		task := craft.Task{Scope: receipt.Scope, WorkspaceID: receipt.WorkspaceID, Fence: runtime.Fence{RunKey: runtime.RunKey{TenantID: receipt.Scope.TenantID, RunID: receipt.RunID}}}
		staged, err := s.stageCapture(ctx, task, kind, source)
		if err != nil {
			_ = s.captures.MarkPendingError(ctx, receipt, err)
			return craft.DraftHead{}, err
		}
		if err := verifyCaptureSourceBinding(source, receipt); err != nil {
			return craft.DraftHead{}, err
		}
		if err := source.VerifyCraftCaptureQuiescent(ctx); err != nil {
			_ = s.captures.MarkPendingError(ctx, receipt, err)
			return craft.DraftHead{}, fmt.Errorf("%w: sandbox changed during capture: %v", craft.ErrBusy, err)
		}
		attemptFiles, err := captureAttemptFiles(staged)
		if err != nil {
			return craft.DraftHead{}, err
		}
		digest, err := craft.ManifestDigest(attemptFiles)
		if err != nil {
			return craft.DraftHead{}, err
		}
		receipt, err = s.captures.BeginCapture(ctx, receipt, digest)
		if err != nil {
			return craft.DraftHead{}, err
		}
		if err := verifyCaptureSourceBinding(source, receipt); err != nil {
			return craft.DraftHead{}, err
		}
		if err := source.VerifyCraftCaptureQuiescent(ctx); err != nil {
			_ = s.captures.MarkPendingError(ctx, receipt, err)
			return craft.DraftHead{}, fmt.Errorf("%w: quiescence lost before object upload: %v", craft.ErrBusy, err)
		}
		files, err := s.uploadCapture(ctx, task, staged)
		if err != nil {
			_ = s.captures.MarkPendingError(ctx, receipt, err)
			return craft.DraftHead{}, err
		}
		if err := verifyCaptureSourceBinding(source, receipt); err != nil {
			return craft.DraftHead{}, err
		}
		sealedDigest, err := craft.ManifestDigest(files)
		if err != nil {
			return craft.DraftHead{}, err
		}
		if sealedDigest != digest {
			return craft.DraftHead{}, craft.ErrConflict
		}
		if err := source.VerifyCraftCaptureQuiescent(ctx); err != nil {
			_ = s.captures.MarkPendingError(ctx, receipt, err)
			return craft.DraftHead{}, fmt.Errorf("%w: sandbox changed during object upload: %v", craft.ErrBusy, err)
		}
		receipt.ManifestDigest = sealedDigest
		receipt, err = s.captures.Seal(ctx, receipt, files, sealedDigest)
		if err != nil {
			return craft.DraftHead{}, err
		}
	}
	if err := source.VerifyCraftCaptureQuiescent(ctx); err != nil {
		_ = s.captures.MarkPendingError(ctx, receipt, err)
		return craft.DraftHead{}, fmt.Errorf("%w: quiescence lost before draft advance: %v", craft.ErrBusy, err)
	}
	if err := verifyCaptureSourceBinding(source, receipt); err != nil {
		return craft.DraftHead{}, err
	}
	predecessor, err := s.drafts.ReadRevision(ctx, receipt.Scope, receipt.WorkspaceID, receipt.PredecessorRevision)
	if err != nil {
		return craft.DraftHead{}, err
	}
	if predecessor.State != receipt.PredecessorState || predecessor.SourceRunID != receipt.PredecessorRunID || predecessor.ManifestDigest != receipt.PredecessorDigest {
		return craft.DraftHead{}, craft.ErrConflict
	}
	if receipt.State != "sealed" {
		return craft.DraftHead{}, fmt.Errorf("%w: capture must be sealed before draft advance", craft.ErrConflict)
	}
	if err := s.verifySealedCaptureObjects(ctx, receipt); err != nil {
		_ = s.captures.MarkPendingError(ctx, receipt, err)
		return craft.DraftHead{}, err
	}
	advanced, advanceErr := s.drafts.Advance(ctx, receipt.Scope, receipt.WorkspaceID, receipt.PredecessorRevision, receipt.RunID, receipt.Files)
	if advanceErr != nil {
		// Advance may have committed before the process lost its receipt write.
		// Adopt only the exact next immutable revision from this Run.
		advanced, err = s.drafts.ReadRevision(ctx, receipt.Scope, receipt.WorkspaceID, receipt.PredecessorRevision+1)
		if err != nil || advanced.SourceRunID != receipt.RunID || advanced.ManifestDigest != receipt.ManifestDigest {
			return craft.DraftHead{}, advanceErr
		}
	}
	if advanced.SourceRunID != receipt.RunID || advanced.ManifestDigest != receipt.ManifestDigest {
		return craft.DraftHead{}, craft.ErrConflict
	}
	receipt.ManifestDigest = advanced.ManifestDigest
	if err := s.captures.MarkAdvanced(ctx, receipt, advanced.Revision); err != nil {
		return craft.DraftHead{}, err
	}
	return advanced, nil
}

func (s *CraftRunCaptureService) verifySealedCaptureObjects(ctx context.Context, receipt repository.CraftRunCapture) error {
	if receipt.State != "sealed" || len(receipt.Files) == 0 || !craft.ValidSHA256(receipt.ManifestDigest) {
		return fmt.Errorf("%w: incomplete sealed capture", craft.ErrConflict)
	}
	digest, err := craft.ManifestDigest(receipt.Files)
	if err != nil {
		return err
	}
	if digest != receipt.ManifestDigest {
		return fmt.Errorf("%w: sealed refs do not match their manifest digest", craft.ErrConflict)
	}
	if err := s.captures.VerifySealedRefs(ctx, receipt); err != nil {
		return err
	}
	var total int64
	for _, file := range receipt.Files {
		if file.Bytes < 0 || file.Bytes > s.artifacts.config.MaxFileBytes {
			return fmt.Errorf("%w: sealed object size exceeds capture bound", craft.ErrConflict)
		}
		total += file.Bytes
		if total > s.artifacts.config.MaxTotalBytes {
			return fmt.Errorf("%w: sealed manifest exceeds capture bound", craft.ErrConflict)
		}
		reader, err := s.artifacts.files.GetFile(ctx, file.Ref)
		if err != nil {
			return fmt.Errorf("%w: sealed capture object is unavailable: %v", craft.ErrNotFound, err)
		}
		data, readErr := io.ReadAll(io.LimitReader(reader, s.artifacts.config.MaxFileBytes+1))
		closeErr := reader.Close()
		if readErr != nil {
			return fmt.Errorf("%w: read sealed capture object: %v", craft.ErrConflict, readErr)
		}
		if closeErr != nil {
			return fmt.Errorf("%w: close sealed capture object: %v", craft.ErrConflict, closeErr)
		}
		if int64(len(data)) != file.Bytes {
			return fmt.Errorf("%w: sealed capture object byte count changed", craft.ErrConflict)
		}
		hash := sha256.Sum256(data)
		if hex.EncodeToString(hash[:]) != file.SHA256 {
			return fmt.Errorf("%w: sealed capture object hash changed", craft.ErrConflict)
		}
	}
	return nil
}

func verifyCaptureSourceBinding(source QuiescentRunArtifactSource, receipt repository.CraftRunCapture) error {
	if source == nil || source.CraftArtifactRunID() != receipt.RunID || source.CraftArtifactGeneration() != receipt.Generation {
		return craft.ErrConflict
	}
	return nil
}

// adoptAdvancedRevision repairs the Advance-committed/receipt-not-updated
// crash without depending on a sandbox source that may no longer be available.
func (s *CraftRunCaptureService) adoptAdvancedRevision(ctx context.Context, receipt repository.CraftRunCapture) (bool, error) {
	head, err := s.drafts.ReadRevision(ctx, receipt.Scope, receipt.WorkspaceID, receipt.PredecessorRevision+1)
	if errors.Is(err, craft.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if head.SourceRunID != receipt.RunID || head.ManifestDigest != receipt.ManifestDigest {
		return false, craft.ErrConflict
	}
	if err := s.captures.MarkAdvanced(ctx, receipt, head.Revision); err != nil {
		return false, err
	}
	return true, nil
}

// stageCapture shares the collector's path, quota and manifest rules but
// separates reads from uploads so the attempt digest is durable first.
func (s *CraftRunCaptureService) stageCapture(ctx context.Context, task craft.Task, kind string, source SandboxArtifactSource) ([]stagedArtifact, error) {
	entries, err := source.ListSessionFiles(ctx, task.Scope.SessionID, s.artifacts.config.OutputDir)
	if err != nil {
		return nil, fmt.Errorf("craft: list Run output: %w", err)
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
		rel, err := craft.ArtifactRelativePath(s.artifacts.config.OutputDir, entry.Path)
		if err != nil {
			return nil, err
		}
		if entry.Size > s.artifacts.config.MaxFileBytes {
			return nil, fmt.Errorf("%w: artifact %q exceeds per-file bound", craft.ErrInvalidInput, rel)
		}
		data, err := source.ReadSessionFile(ctx, task.Scope.SessionID, entry.Path)
		if err != nil {
			return nil, fmt.Errorf("craft: read Run output %q: %w", rel, err)
		}
		if int64(len(data)) > s.artifacts.config.MaxFileBytes {
			return nil, fmt.Errorf("%w: artifact %q exceeds per-file bound", craft.ErrInvalidInput, rel)
		}
		total += int64(len(data))
		if total > s.artifacts.config.MaxTotalBytes {
			return nil, fmt.Errorf("%w: Run output exceeds total bound", craft.ErrInvalidInput)
		}
		staged = append(staged, stagedArtifact{rel: rel, data: data})
	}
	sort.Slice(staged, func(i, j int) bool { return staged[i].rel < staged[j].rel })
	if len(staged) == 0 {
		return nil, fmt.Errorf("%w: Run output is empty", craft.ErrInvalidInput)
	}
	if err := craftValidateStagedManifest(kind, staged); err != nil {
		return nil, err
	}
	return staged, nil
}

func captureAttemptFiles(staged []stagedArtifact) ([]craft.File, error) {
	files := make([]craft.File, 0, len(staged))
	for _, item := range staged {
		sum := sha256.Sum256(item.data)
		files = append(files, craft.File{Path: item.rel, SHA256: hex.EncodeToString(sum[:]), Bytes: int64(len(item.data))})
	}
	return files, nil
}

func (s *CraftRunCaptureService) uploadCapture(ctx context.Context, task craft.Task, staged []stagedArtifact) ([]craft.File, error) {
	files := make([]craft.File, 0, len(staged))
	for _, item := range staged {
		sum := sha256.Sum256(item.data)
		hash := hex.EncodeToString(sum[:])
		name := "craft_" + hash + "_" + safeFileName(path.Base(item.rel))
		ref, err := s.artifacts.files.SaveBytes(ctx, item.data, task.Scope.TenantID, name, false)
		if err != nil {
			return nil, fmt.Errorf("craft: upload Run output %q: %w", item.rel, err)
		}
		files = append(files, craft.File{Path: item.rel, Ref: ref, SHA256: hash, Bytes: int64(len(item.data)), MIME: craftArtifactMIME(item.rel)})
	}
	return files, nil
}
