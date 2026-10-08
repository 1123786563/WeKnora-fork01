package container

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/craft"
)

const (
	craftKnowledgePublisherMaxFiles = craft.MaxKnowledgeSources + 1
	craftKnowledgePublisherMaxBytes = craft.MaxKnowledgeBundleBytes + 32<<10
	craftKnowledgePublisherSealName = "seal.json"
	craftKnowledgePublisherPayload  = "payload"
)

var errCraftKnowledgePublisherFS = errors.New("craft publisher: filesystem operation failed")

type craftKnowledgePublisherManifest struct {
	Kind       string                                  `json:"kind"`
	DataNotice string                                  `json:"data_notice"`
	Scope      craftKnowledgePublisherManifestScope    `json:"scope"`
	Query      string                                  `json:"query"`
	Truncated  bool                                    `json:"truncated"`
	Empty      bool                                    `json:"empty"`
	Sources    []craftKnowledgePublisherManifestSource `json:"sources"`
}

type craftKnowledgePublisherManifestScope struct {
	TenantID  uint64 `json:"tenant_id"`
	UserID    string `json:"user_id"`
	SessionID string `json:"session_id"`
}

type craftKnowledgePublisherManifestSource struct {
	CitationID      string    `json:"citation_id"`
	Ref             string    `json:"ref"`
	KnowledgeID     string    `json:"knowledge_id"`
	KnowledgeBaseID string    `json:"knowledge_base_id"`
	TenantID        uint64    `json:"tenant_id"`
	Title           string    `json:"title,omitempty"`
	ExcerptBytes    int       `json:"excerpt_bytes"`
	Digest          string    `json:"digest"`
	AcquiredAt      time.Time `json:"acquired_at"`
	File            string    `json:"file"`
	Truncated       bool      `json:"truncated,omitempty"`
}

type craftKnowledgePublisherSeal struct {
	Version   int                                        `json:"version"`
	RunID     string                                     `json:"run_id"`
	Directory string                                     `json:"directory"`
	Digest    string                                     `json:"digest"`
	Files     map[string]craftKnowledgePublisherSealFile `json:"files"`
}

type craftKnowledgePublisherSealFile struct {
	Bytes  int    `json:"bytes"`
	SHA256 string `json:"sha256"`
}

// craftKnowledgeFilesystemPublisher stores candidates in a private sibling of
// the isolated RunView root. The sibling shares the filesystem for atomic
// rename without becoming part of the delegate's view.
type craftKnowledgeFilesystemPublisher struct {
	root          string
	runID         string
	candidateRoot string
	writeFile     func(string, []byte, fs.FileMode) error
	rename        func(string, string) error
	syncDir       func(string) error
}

// NewCraftKnowledgePackagePublisher binds a publisher to one canonical,
// server-owned RunView root and its admitted Run ID.
func NewCraftKnowledgePackagePublisher(root, runID string) (*craftKnowledgeFilesystemPublisher, error) {
	if root == "" || !filepath.IsAbs(root) || filepath.Clean(root) != root || craft.KnowledgeRunDir(runID) == "" || len(runID) > 64 {
		return nil, craft.ErrInvalidInput
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil || resolved != root {
		return nil, craft.ErrInvalidInput
	}
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, craft.ErrInvalidInput
	}
	return &craftKnowledgeFilesystemPublisher{
		root: root, runID: runID, candidateRoot: root + ".craft-knowledge-candidates",
		writeFile: os.WriteFile, rename: os.Rename, syncDir: syncCraftKnowledgeDir,
	}, nil
}

func (p *craftKnowledgeFilesystemPublisher) Prepare(ctx context.Context, workspace craft.Workspace, pkg service.CraftKnowledgeMaterialPackage) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := p.validatePackage(workspace, pkg); err != nil {
		return err
	}
	if _, err := p.checkRootAndTarget(pkg.Directory, false); err != nil {
		return err
	}
	if existing, exists, err := p.recoverPublished(ctx, workspace, pkg.RunID, pkg.Digest); err != nil {
		return err
	} else if exists {
		if sameCraftKnowledgePackage(existing, pkg) {
			return nil
		}
		return craft.ErrConflict
	}
	if existing, exists, err := p.readCandidate(pkg.RunID, pkg.Digest, workspace); err != nil {
		return err
	} else if exists {
		if sameCraftKnowledgePackage(existing, pkg) {
			return nil
		}
		return craft.ErrConflict
	}
	if err := p.ensureCandidateParent(pkg.RunID); err != nil {
		return err
	}
	candidatePath := p.candidatePath(pkg.RunID, pkg.Digest)
	stage, err := os.MkdirTemp(filepath.Dir(candidatePath), ".stage-")
	if err != nil {
		return errCraftKnowledgePublisherFS
	}
	defer func() {
		if stage != "" {
			_ = os.RemoveAll(stage)
		}
	}()
	if err := os.Chmod(stage, 0700); err != nil {
		return errCraftKnowledgePublisherFS
	}
	payloadDir := filepath.Join(stage, craftKnowledgePublisherPayload)
	if err := os.Mkdir(payloadDir, 0700); err != nil {
		return errCraftKnowledgePublisherFS
	}
	seal := craftKnowledgePublisherSeal{
		Version: 1, RunID: pkg.RunID, Directory: pkg.Directory, Digest: pkg.Digest,
		Files: make(map[string]craftKnowledgePublisherSealFile, len(pkg.Files)),
	}
	for _, relative := range sortedCraftKnowledgeFiles(pkg.Files) {
		if err := ctx.Err(); err != nil {
			return err
		}
		name := path.Base(relative)
		content := pkg.Files[relative]
		filePath := filepath.Join(payloadDir, name)
		if err := p.writeFile(filePath, content, 0600); err != nil {
			return errCraftKnowledgePublisherFS
		}
		if err := syncCraftKnowledgeFile(filePath); err != nil {
			return errCraftKnowledgePublisherFS
		}
		sum := sha256.Sum256(content)
		seal.Files[name] = craftKnowledgePublisherSealFile{Bytes: len(content), SHA256: hex.EncodeToString(sum[:])}
	}
	sealBytes, err := json.Marshal(seal)
	if err != nil {
		return errCraftKnowledgePublisherFS
	}
	sealPath := filepath.Join(stage, craftKnowledgePublisherSealName)
	if err := p.writeFile(sealPath, sealBytes, 0600); err != nil || syncCraftKnowledgeFile(sealPath) != nil || p.syncDir(payloadDir) != nil || p.syncDir(stage) != nil {
		return errCraftKnowledgePublisherFS
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := p.rename(stage, candidatePath); err != nil {
		if existing, exists, readErr := p.readCandidate(pkg.RunID, pkg.Digest, workspace); readErr == nil && exists && sameCraftKnowledgePackage(existing, pkg) {
			return nil
		}
		return errCraftKnowledgePublisherFS
	}
	stage = ""
	if err := p.syncDir(filepath.Dir(candidatePath)); err != nil {
		return errCraftKnowledgePublisherFS
	}
	return nil
}

func (p *craftKnowledgeFilesystemPublisher) Publish(ctx context.Context, workspace craft.Workspace, pkg service.CraftKnowledgeMaterialPackage) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := p.validatePackage(workspace, pkg); err != nil {
		return err
	}
	finalPath, err := p.checkRootAndTarget(pkg.Directory, false)
	if err != nil {
		return err
	}
	if existing, exists, err := p.recoverPublished(ctx, workspace, pkg.RunID, pkg.Digest); err != nil {
		return err
	} else if exists {
		if sameCraftKnowledgePackage(existing, pkg) {
			return nil
		}
		return craft.ErrConflict
	}
	candidate, exists, err := p.readCandidate(pkg.RunID, pkg.Digest, workspace)
	if err != nil {
		return err
	}
	if !exists {
		return craft.ErrNotFound
	}
	if !sameCraftKnowledgePackage(candidate, pkg) {
		return craft.ErrConflict
	}
	if err := p.ensureRunParent(pkg.Directory); err != nil {
		return err
	}
	payloadPath := filepath.Join(p.candidatePath(pkg.RunID, pkg.Digest), craftKnowledgePublisherPayload)
	if err := prepareCraftKnowledgePublishedModes(payloadPath, pkg.Files); err != nil {
		return errCraftKnowledgePublisherFS
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := p.rename(payloadPath, finalPath); err != nil {
		if existing, found, readErr := p.readPublished(pkg.RunID); readErr == nil && found {
			if sameCraftKnowledgePackage(existing, pkg) {
				return nil
			}
			return craft.ErrConflict
		}
		return errCraftKnowledgePublisherFS
	}
	// Darwin filesystems reject renaming a 0555 source directory. The atomic
	// rename still exposes only the fully written tree (all files are already
	// 0444); seal the package directory immediately after the linearization
	// point, then finish durability work.
	if err := os.Chmod(finalPath, 0555); err != nil {
		return errCraftKnowledgePublisherFS
	}
	if err := p.syncDir(finalPath); err != nil {
		return errCraftKnowledgePublisherFS
	}
	// Rename is the publication linearization point; finish durability work
	// even if cancellation arrives after the complete tree became visible.
	if err := p.syncDir(filepath.Dir(finalPath)); err != nil {
		return errCraftKnowledgePublisherFS
	}
	if err := p.syncDir(filepath.Dir(filepath.Dir(finalPath))); err != nil {
		return errCraftKnowledgePublisherFS
	}
	if err := p.removePublishedCandidate(workspace, pkg); err != nil {
		return err
	}
	return nil
}

func (p *craftKnowledgeFilesystemPublisher) Discard(ctx context.Context, workspace craft.Workspace, pkg service.CraftKnowledgeMaterialPackage) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := p.validatePackage(workspace, pkg); err != nil {
		return err
	}
	if _, exists, err := p.readPublished(pkg.RunID); err != nil {
		return err
	} else if exists {
		return craft.ErrConflict
	}
	candidate, exists, err := p.readCandidate(pkg.RunID, pkg.Digest, workspace)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	if !sameCraftKnowledgePackage(candidate, pkg) {
		return craft.ErrConflict
	}
	if err := os.RemoveAll(p.candidatePath(pkg.RunID, pkg.Digest)); err != nil {
		return errCraftKnowledgePublisherFS
	}
	if err := p.syncDir(filepath.Dir(p.candidatePath(pkg.RunID, pkg.Digest))); err != nil {
		return errCraftKnowledgePublisherFS
	}
	return nil
}

func (p *craftKnowledgeFilesystemPublisher) Resume(ctx context.Context, workspace craft.Workspace, runID, acceptedDigest string) (service.CraftKnowledgeMaterialPackage, error) {
	if err := ctx.Err(); err != nil {
		return service.CraftKnowledgeMaterialPackage{}, err
	}
	if p == nil || runID != p.runID || craft.KnowledgeRunDir(runID) == "" || !validCraftKnowledgeDigest(acceptedDigest) {
		return service.CraftKnowledgeMaterialPackage{}, craft.ErrInvalidInput
	}
	if final, exists, err := p.recoverPublished(ctx, workspace, runID, acceptedDigest); err != nil {
		return service.CraftKnowledgeMaterialPackage{}, err
	} else if exists {
		if final.Digest != acceptedDigest {
			return service.CraftKnowledgeMaterialPackage{}, craft.ErrNotFound
		}
		if err := validateCraftKnowledgeWorkspace(workspace, final); err != nil {
			return service.CraftKnowledgeMaterialPackage{}, err
		}
		return final, nil
	}
	resumed, exists, err := p.readCandidate(runID, acceptedDigest, workspace)
	if err != nil {
		return service.CraftKnowledgeMaterialPackage{}, err
	}
	if !exists {
		return service.CraftKnowledgeMaterialPackage{}, craft.ErrNotFound
	}
	return resumed, nil
}

func (p *craftKnowledgeFilesystemPublisher) recoverPublished(ctx context.Context, workspace craft.Workspace, runID, acceptedDigest string) (service.CraftKnowledgeMaterialPackage, bool, error) {
	if err := ctx.Err(); err != nil {
		return service.CraftKnowledgeMaterialPackage{}, false, err
	}
	if p == nil || runID != p.runID {
		return service.CraftKnowledgeMaterialPackage{}, false, craft.ErrInvalidInput
	}
	finalPath, err := p.checkRootAndTarget(craft.KnowledgeRunDir(runID), false)
	if err != nil {
		return service.CraftKnowledgeMaterialPackage{}, false, err
	}
	info, err := os.Lstat(finalPath)
	if errors.Is(err, os.ErrNotExist) {
		return service.CraftKnowledgeMaterialPackage{}, false, nil
	}
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return service.CraftKnowledgeMaterialPackage{}, false, craft.ErrConflict
	}

	switch info.Mode().Perm() {
	case 0555:
		pkg, exists, err := p.readPublished(runID)
		if err != nil || !exists {
			if err == nil {
				err = craft.ErrConflict
			}
			return service.CraftKnowledgeMaterialPackage{}, false, err
		}
		if err := validateCraftKnowledgeWorkspace(workspace, pkg); err != nil {
			return service.CraftKnowledgeMaterialPackage{}, false, err
		}
		if err := p.syncDir(finalPath); err != nil {
			return service.CraftKnowledgeMaterialPackage{}, false, errCraftKnowledgePublisherFS
		}
		if err := p.syncDir(filepath.Dir(finalPath)); err != nil {
			return service.CraftKnowledgeMaterialPackage{}, false, errCraftKnowledgePublisherFS
		}
		// Durability parity with the Publish success path: the crash this
		// recovery completes may also predate the fsync of the freshly created
		// runs/ entry inside knowledge/, so finish the grandparent sync too.
		if err := p.syncDir(filepath.Dir(filepath.Dir(finalPath))); err != nil {
			return service.CraftKnowledgeMaterialPackage{}, false, errCraftKnowledgePublisherFS
		}
		if acceptedDigest == pkg.Digest {
			if err := p.removePublishedCandidate(workspace, pkg); err != nil {
				return service.CraftKnowledgeMaterialPackage{}, false, err
			}
		}
		return pkg, true, nil
	case 0700:
		if !validCraftKnowledgeDigest(acceptedDigest) {
			return service.CraftKnowledgeMaterialPackage{}, false, craft.ErrInvalidInput
		}
		seal, exists, err := p.readCandidateSeal(runID, acceptedDigest)
		if err != nil {
			return service.CraftKnowledgeMaterialPackage{}, false, err
		}
		if !exists || seal.Digest != acceptedDigest {
			return service.CraftKnowledgeMaterialPackage{}, false, craft.ErrConflict
		}
		candidatePath := p.candidatePath(runID, acceptedDigest)
		entries, err := os.ReadDir(candidatePath)
		if err != nil || len(entries) != 1 || entries[0].Name() != craftKnowledgePublisherSealName {
			return service.CraftKnowledgeMaterialPackage{}, false, craft.ErrConflict
		}
		files, err := readCraftKnowledgePayload(finalPath, craft.KnowledgeRunDir(runID), false)
		if err != nil {
			return service.CraftKnowledgeMaterialPackage{}, false, err
		}
		pkg := service.CraftKnowledgeMaterialPackage{RunID: runID, Directory: craft.KnowledgeRunDir(runID), Digest: craftKnowledgeFilesystemDigest(files), Files: files}
		if pkg.Digest != acceptedDigest || !verifyCraftKnowledgeSeal(seal, pkg.Files) {
			return service.CraftKnowledgeMaterialPackage{}, false, craft.ErrConflict
		}
		if _, err := p.validatePackage(workspace, pkg); err != nil {
			return service.CraftKnowledgeMaterialPackage{}, false, err
		}
		if err := prepareCraftKnowledgePublishedModes(finalPath, pkg.Files); err != nil {
			return service.CraftKnowledgeMaterialPackage{}, false, errCraftKnowledgePublisherFS
		}
		if err := os.Chmod(finalPath, 0555); err != nil {
			return service.CraftKnowledgeMaterialPackage{}, false, errCraftKnowledgePublisherFS
		}
		if err := p.syncDir(finalPath); err != nil {
			return service.CraftKnowledgeMaterialPackage{}, false, errCraftKnowledgePublisherFS
		}
		if err := p.syncDir(filepath.Dir(finalPath)); err != nil {
			return service.CraftKnowledgeMaterialPackage{}, false, errCraftKnowledgePublisherFS
		}
		// Same durability parity as the sealed-package branch above and the
		// Publish success path: complete the knowledge/ grandparent fsync.
		if err := p.syncDir(filepath.Dir(filepath.Dir(finalPath))); err != nil {
			return service.CraftKnowledgeMaterialPackage{}, false, errCraftKnowledgePublisherFS
		}
		if err := p.removePublishedCandidate(workspace, pkg); err != nil {
			return service.CraftKnowledgeMaterialPackage{}, false, err
		}
		return pkg, true, nil
	default:
		return service.CraftKnowledgeMaterialPackage{}, false, craft.ErrConflict
	}
}

func (p *craftKnowledgeFilesystemPublisher) removePublishedCandidate(workspace craft.Workspace, pkg service.CraftKnowledgeMaterialPackage) error {
	candidatePath := p.candidatePath(pkg.RunID, pkg.Digest)
	seal, exists, err := p.readCandidateSeal(pkg.RunID, pkg.Digest)
	if err != nil {
		return err
	}
	if !exists {
		parent := filepath.Dir(candidatePath)
		if info, err := os.Lstat(parent); errors.Is(err, os.ErrNotExist) {
			return nil
		} else if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return craft.ErrConflict
		}
		if err := p.syncDir(parent); err != nil {
			return errCraftKnowledgePublisherFS
		}
		return nil
	}
	if seal.Directory != pkg.Directory || seal.Digest != pkg.Digest || !verifyCraftKnowledgeSeal(seal, pkg.Files) {
		return craft.ErrConflict
	}
	entries, err := os.ReadDir(candidatePath)
	if err != nil {
		return craft.ErrConflict
	}
	for _, entry := range entries {
		if entry.Name() == craftKnowledgePublisherSealName {
			continue
		}
		if entry.Name() != craftKnowledgePublisherPayload || entry.Type()&os.ModeSymlink != 0 || !entry.IsDir() {
			return craft.ErrConflict
		}
		candidate, found, err := p.readCandidate(pkg.RunID, pkg.Digest, workspace)
		if err != nil || !found || !sameCraftKnowledgePackage(candidate, pkg) {
			return craft.ErrConflict
		}
	}
	if err := os.RemoveAll(candidatePath); err != nil {
		return errCraftKnowledgePublisherFS
	}
	if err := p.syncDir(filepath.Dir(candidatePath)); err != nil {
		return errCraftKnowledgePublisherFS
	}
	return nil
}

func (p *craftKnowledgeFilesystemPublisher) validatePackage(workspace craft.Workspace, pkg service.CraftKnowledgeMaterialPackage) (craftKnowledgePublisherManifest, error) {
	if p == nil || pkg.RunID != p.runID || craft.KnowledgeRunDir(pkg.RunID) == "" || pkg.Directory != craft.KnowledgeRunDir(pkg.RunID) ||
		len(pkg.Files) == 0 || len(pkg.Files) > craftKnowledgePublisherMaxFiles || !validCraftKnowledgeDigest(pkg.Digest) {
		return craftKnowledgePublisherManifest{}, craft.ErrInvalidInput
	}
	var totalBytes int
	for filePath, content := range pkg.Files {
		if !validCraftKnowledgeFilePath(pkg.Directory, filePath) {
			return craftKnowledgePublisherManifest{}, craft.ErrInvalidInput
		}
		totalBytes += len(content)
		if totalBytes > craftKnowledgePublisherMaxBytes {
			return craftKnowledgePublisherManifest{}, craft.ErrInvalidInput
		}
	}
	if craftKnowledgeFilesystemDigest(pkg.Files) != pkg.Digest {
		return craftKnowledgePublisherManifest{}, craft.ErrConflict
	}
	manifestBytes, ok := pkg.Files[pkg.Directory+"/manifest.json"]
	if !ok || len(manifestBytes) == 0 {
		return craftKnowledgePublisherManifest{}, craft.ErrInvalidInput
	}
	var manifest craftKnowledgePublisherManifest
	decoder := json.NewDecoder(strings.NewReader(string(manifestBytes)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return craftKnowledgePublisherManifest{}, craft.ErrInvalidInput
	}
	if manifest.Kind != "craft.knowledge.manifest" || manifest.DataNotice != craft.KnowledgeDataNotice ||
		manifest.Scope.TenantID != workspace.TenantID || manifest.Scope.UserID != workspace.UserID || manifest.Scope.SessionID != workspace.SessionID ||
		strings.TrimSpace(manifest.Query) == "" || len(manifest.Sources) > craft.MaxKnowledgeSources || len(pkg.Files) != len(manifest.Sources)+1 ||
		manifest.Empty != (len(manifest.Sources) == 0) {
		return craftKnowledgePublisherManifest{}, craft.ErrInvalidInput
	}
	if err := validateCraftKnowledgeWorkspace(workspace, pkg); err != nil {
		return craftKnowledgePublisherManifest{}, err
	}
	seen := make(map[string]struct{}, len(manifest.Sources))
	var totalExcerptBytes int
	for _, source := range manifest.Sources {
		if !validCraftKnowledgePathToken(source.CitationID) || source.KnowledgeID == "" || source.KnowledgeBaseID == "" || source.TenantID == 0 ||
			source.ExcerptBytes < 0 || source.ExcerptBytes > craft.MaxKnowledgeExcerptBytes || !validCraftKnowledgeDigest(source.Digest) ||
			source.File != pkg.Directory+"/"+source.CitationID+".txt" {
			return craftKnowledgePublisherManifest{}, craft.ErrInvalidInput
		}
		if _, duplicate := seen[source.CitationID]; duplicate {
			return craftKnowledgePublisherManifest{}, craft.ErrInvalidInput
		}
		seen[source.CitationID] = struct{}{}
		kbID, knowledgeID, chunkID, ok := parseCraftKnowledgeRef(source.Ref)
		if !ok || kbID != source.KnowledgeBaseID || knowledgeID != source.KnowledgeID || chunkID == "" {
			return craftKnowledgePublisherManifest{}, craft.ErrInvalidInput
		}
		material, ok := pkg.Files[source.File]
		if !ok {
			return craftKnowledgePublisherManifest{}, craft.ErrInvalidInput
		}
		prefix := craft.KnowledgeDataNotice + "\ncitation: " + source.CitationID + "\nref: " + source.Ref + "\n\n"
		content := string(material)
		if !strings.HasPrefix(content, prefix) || !strings.HasSuffix(content, "\n") || len(content) < len(prefix)+1 {
			return craftKnowledgePublisherManifest{}, craft.ErrInvalidInput
		}
		excerpt := strings.TrimSuffix(strings.TrimPrefix(content, prefix), "\n")
		sum := sha256.Sum256([]byte(excerpt))
		if len(excerpt) != source.ExcerptBytes || hex.EncodeToString(sum[:]) != source.Digest {
			return craftKnowledgePublisherManifest{}, craft.ErrConflict
		}
		totalExcerptBytes += len(excerpt)
		if totalExcerptBytes > craft.MaxKnowledgeBundleBytes {
			return craftKnowledgePublisherManifest{}, craft.ErrInvalidInput
		}
	}
	return manifest, nil
}

func validateCraftKnowledgeWorkspace(workspace craft.Workspace, pkg service.CraftKnowledgeMaterialPackage) error {
	if workspace.ID == "" || workspace.TenantID == 0 || workspace.UserID == "" || workspace.SessionID == "" {
		return craft.ErrInvalidInput
	}
	manifestBytes, ok := pkg.Files[pkg.Directory+"/manifest.json"]
	if !ok {
		return craft.ErrInvalidInput
	}
	var manifest craftKnowledgePublisherManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return craft.ErrInvalidInput
	}
	if manifest.Scope.TenantID != workspace.TenantID || manifest.Scope.UserID != workspace.UserID || manifest.Scope.SessionID != workspace.SessionID {
		return craft.ErrForbidden
	}
	return nil
}

func (p *craftKnowledgeFilesystemPublisher) readPublished(runID string) (service.CraftKnowledgeMaterialPackage, bool, error) {
	if p == nil || runID != p.runID {
		return service.CraftKnowledgeMaterialPackage{}, false, craft.ErrInvalidInput
	}
	directory := craft.KnowledgeRunDir(runID)
	finalPath, err := p.checkRootAndTarget(directory, false)
	if err != nil {
		return service.CraftKnowledgeMaterialPackage{}, false, err
	}
	info, err := os.Lstat(finalPath)
	if errors.Is(err, os.ErrNotExist) {
		return service.CraftKnowledgeMaterialPackage{}, false, nil
	}
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return service.CraftKnowledgeMaterialPackage{}, false, craft.ErrConflict
	}
	files, err := readCraftKnowledgePayload(finalPath, directory, true)
	if err != nil {
		return service.CraftKnowledgeMaterialPackage{}, false, err
	}
	pkg := service.CraftKnowledgeMaterialPackage{RunID: runID, Directory: directory, Digest: craftKnowledgeFilesystemDigest(files), Files: files}
	if _, err := p.validatePackageForPublished(pkg); err != nil {
		return service.CraftKnowledgeMaterialPackage{}, false, err
	}
	return pkg, true, nil
}

func (p *craftKnowledgeFilesystemPublisher) validatePackageForPublished(pkg service.CraftKnowledgeMaterialPackage) (craftKnowledgePublisherManifest, error) {
	manifestBytes, ok := pkg.Files[pkg.Directory+"/manifest.json"]
	if !ok {
		return craftKnowledgePublisherManifest{}, craft.ErrConflict
	}
	var manifest craftKnowledgePublisherManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return craftKnowledgePublisherManifest{}, craft.ErrConflict
	}
	workspace := craft.Workspace{ID: "publisher-validation", Scope: craft.Scope{TenantID: manifest.Scope.TenantID, UserID: manifest.Scope.UserID, SessionID: manifest.Scope.SessionID}}
	return p.validatePackage(workspace, pkg)
}

func (p *craftKnowledgeFilesystemPublisher) readCandidateSeal(runID, digest string) (craftKnowledgePublisherSeal, bool, error) {
	if p == nil || runID != p.runID || !validCraftKnowledgeDigest(digest) {
		return craftKnowledgePublisherSeal{}, false, craft.ErrInvalidInput
	}
	candidatePath := p.candidatePath(runID, digest)
	if err := rejectCraftKnowledgeCandidateSymlinks(p.candidateRoot, runID, digest); err != nil {
		return craftKnowledgePublisherSeal{}, false, err
	}
	info, err := os.Lstat(candidatePath)
	if errors.Is(err, os.ErrNotExist) {
		return craftKnowledgePublisherSeal{}, false, nil
	}
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return craftKnowledgePublisherSeal{}, false, craft.ErrConflict
	}
	sealPath := filepath.Join(candidatePath, craftKnowledgePublisherSealName)
	sealInfo, err := os.Lstat(sealPath)
	if err != nil || sealInfo.Mode()&os.ModeSymlink != 0 || !sealInfo.Mode().IsRegular() || sealInfo.Mode().Perm() != 0600 || sealInfo.Size() < 0 || sealInfo.Size() > 64<<10 {
		return craftKnowledgePublisherSeal{}, false, craft.ErrConflict
	}
	sealBytes, err := os.ReadFile(sealPath)
	if err != nil || int64(len(sealBytes)) != sealInfo.Size() {
		return craftKnowledgePublisherSeal{}, false, craft.ErrConflict
	}
	var seal craftKnowledgePublisherSeal
	decoder := json.NewDecoder(strings.NewReader(string(sealBytes)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&seal); err != nil || seal.Version != 1 || seal.RunID != runID || seal.Directory != craft.KnowledgeRunDir(runID) || seal.Digest != digest {
		return craftKnowledgePublisherSeal{}, false, craft.ErrConflict
	}
	return seal, true, nil
}

func (p *craftKnowledgeFilesystemPublisher) readCandidate(runID, digest string, workspace craft.Workspace) (service.CraftKnowledgeMaterialPackage, bool, error) {
	if p == nil || runID != p.runID || !validCraftKnowledgeDigest(digest) {
		return service.CraftKnowledgeMaterialPackage{}, false, craft.ErrInvalidInput
	}
	candidatePath := p.candidatePath(runID, digest)
	seal, exists, err := p.readCandidateSeal(runID, digest)
	if err != nil || !exists {
		if err == nil {
			return service.CraftKnowledgeMaterialPackage{}, false, nil
		}
		return service.CraftKnowledgeMaterialPackage{}, false, err
	}
	payloadPath := filepath.Join(candidatePath, craftKnowledgePublisherPayload)
	files, err := readCraftKnowledgePayload(payloadPath, seal.Directory, false)
	if err != nil {
		return service.CraftKnowledgeMaterialPackage{}, false, err
	}
	pkg := service.CraftKnowledgeMaterialPackage{RunID: runID, Directory: seal.Directory, Digest: seal.Digest, Files: files}
	if _, err := p.validatePackage(workspace, pkg); err != nil {
		return service.CraftKnowledgeMaterialPackage{}, false, err
	}
	if !verifyCraftKnowledgeSeal(seal, pkg.Files) {
		return service.CraftKnowledgeMaterialPackage{}, false, craft.ErrConflict
	}
	return pkg, true, nil
}

func verifyCraftKnowledgeSeal(seal craftKnowledgePublisherSeal, files map[string][]byte) bool {
	if len(seal.Files) != len(files) {
		return false
	}
	for relative, content := range files {
		file, ok := seal.Files[path.Base(relative)]
		if !ok || file.Bytes != len(content) {
			return false
		}
		sum := sha256.Sum256(content)
		if file.SHA256 != hex.EncodeToString(sum[:]) {
			return false
		}
	}
	return true
}

func (p *craftKnowledgeFilesystemPublisher) candidatePath(runID, digest string) string {
	return filepath.Join(p.candidateRoot, runID, digest)
}

func (p *craftKnowledgeFilesystemPublisher) ensureCandidateParent(runID string) error {
	if err := ensureCraftKnowledgeDirectory(p.candidateRoot, 0700, true); err != nil {
		return errCraftKnowledgePublisherFS
	}
	if err := ensureCraftKnowledgeDirectory(filepath.Join(p.candidateRoot, runID), 0700, true); err != nil {
		return errCraftKnowledgePublisherFS
	}
	return nil
}

func (p *craftKnowledgeFilesystemPublisher) ensureRunParent(directory string) error {
	_, err := p.checkRootAndTarget(directory, true)
	return err
}

func (p *craftKnowledgeFilesystemPublisher) checkRootAndTarget(directory string, createParents bool) (string, error) {
	if p == nil || directory != craft.KnowledgeRunDir(p.runID) {
		return "", craft.ErrInvalidInput
	}
	resolved, err := filepath.EvalSymlinks(p.root)
	if err != nil || resolved != p.root {
		return "", craft.ErrInvalidInput
	}
	rootInfo, err := os.Lstat(p.root)
	if err != nil || !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 {
		return "", craft.ErrInvalidInput
	}
	current := p.root
	parts := strings.Split(directory, "/")
	for i, part := range parts {
		if !validCraftKnowledgePathToken(part) {
			return "", craft.ErrInvalidInput
		}
		current = filepath.Join(current, part)
		info, statErr := os.Lstat(current)
		if errors.Is(statErr, os.ErrNotExist) {
			if createParents && i < len(parts)-1 {
				if err := os.Mkdir(current, 0700); err != nil && !errors.Is(err, os.ErrExist) {
					return "", errCraftKnowledgePublisherFS
				}
				info, statErr = os.Lstat(current)
			}
			if errors.Is(statErr, os.ErrNotExist) {
				continue
			}
		}
		if statErr != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return "", craft.ErrInvalidInput
		}
	}
	return filepath.Join(p.root, filepath.FromSlash(directory)), nil
}

func rejectCraftKnowledgeCandidateSymlinks(candidateRoot, runID, digest string) error {
	parent := filepath.Dir(candidateRoot)
	resolved, err := filepath.EvalSymlinks(parent)
	if err != nil || resolved != parent {
		return craft.ErrInvalidInput
	}
	for _, current := range []string{candidateRoot, filepath.Join(candidateRoot, runID), filepath.Join(candidateRoot, runID, digest)} {
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() || info.Mode().Perm() != 0700 {
			return craft.ErrConflict
		}
	}
	return nil
}

func ensureCraftKnowledgeDirectory(directory string, mode fs.FileMode, chmodExisting bool) error {
	if !filepath.IsAbs(directory) || filepath.Clean(directory) != directory {
		return craft.ErrInvalidInput
	}
	volume := filepath.VolumeName(directory)
	current := volume + string(filepath.Separator)
	relative := strings.TrimPrefix(directory, current)
	for _, part := range strings.Split(relative, string(filepath.Separator)) {
		if part == "" {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			if err := os.Mkdir(current, mode); err != nil && !errors.Is(err, os.ErrExist) {
				return err
			}
			info, err = os.Lstat(current)
		}
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return craft.ErrInvalidInput
		}
	}
	if chmodExisting {
		return os.Chmod(directory, mode)
	}
	return nil
}

func readCraftKnowledgePayload(hostDirectory, workspaceDirectory string, requireReadOnly bool) (map[string][]byte, error) {
	info, err := os.Lstat(hostDirectory)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, craft.ErrConflict
	}
	if requireReadOnly && info.Mode().Perm()&0222 != 0 {
		return nil, craft.ErrConflict
	}
	entries, err := os.ReadDir(hostDirectory)
	if err != nil || len(entries) == 0 || len(entries) > craftKnowledgePublisherMaxFiles {
		return nil, craft.ErrConflict
	}
	files := make(map[string][]byte, len(entries))
	totalBytes := 0
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 || entry.IsDir() || !validCraftKnowledgePathToken(entry.Name()) {
			return nil, craft.ErrConflict
		}
		filePath := filepath.Join(hostDirectory, entry.Name())
		fileInfo, err := os.Lstat(filePath)
		if err != nil || !fileInfo.Mode().IsRegular() || fileInfo.Mode()&os.ModeSymlink != 0 || (requireReadOnly && fileInfo.Mode().Perm()&0222 != 0) {
			return nil, craft.ErrConflict
		}
		if fileInfo.Size() < 0 || fileInfo.Size() > craftKnowledgePublisherMaxBytes {
			return nil, craft.ErrConflict
		}
		content, err := os.ReadFile(filePath)
		if err != nil || int64(len(content)) != fileInfo.Size() {
			return nil, craft.ErrConflict
		}
		totalBytes += len(content)
		if totalBytes > craftKnowledgePublisherMaxBytes {
			return nil, craft.ErrConflict
		}
		files[workspaceDirectory+"/"+entry.Name()] = content
	}
	return files, nil
}

func prepareCraftKnowledgePublishedModes(payloadPath string, files map[string][]byte) error {
	// A previous failed publish may have sealed some modes but retained the
	// candidate. Restore only this private payload directory before resealing.
	if err := os.Chmod(payloadPath, 0700); err != nil {
		return err
	}
	for relative := range files {
		filePath := filepath.Join(payloadPath, path.Base(relative))
		if err := os.Chmod(filePath, 0444); err != nil {
			return err
		}
		if err := syncCraftKnowledgeFile(filePath); err != nil {
			return err
		}
	}
	// Keep the directory writable until its atomic rename. macOS denies
	// renaming a non-writable source directory; files themselves are already
	// immutable before publication.
	return syncCraftKnowledgeDir(payloadPath)
}

func syncCraftKnowledgeFile(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	return file.Sync()
}

func syncCraftKnowledgeDir(directory string) error {
	file, err := os.Open(directory)
	if err != nil {
		return err
	}
	defer file.Close()
	return file.Sync()
}

func validCraftKnowledgeFilePath(directory, filePath string) bool {
	if strings.Contains(filePath, "\\") || strings.ContainsRune(filePath, '\x00') || path.IsAbs(filePath) || path.Clean(filePath) != filePath || path.Dir(filePath) != directory {
		return false
	}
	return validCraftKnowledgePathToken(path.Base(filePath))
}

func validCraftKnowledgePathToken(value string) bool {
	if value == "" || value == "." || value == ".." || len(value) > 255 {
		return false
	}
	for _, r := range value {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.') {
			return false
		}
	}
	return true
}

func validCraftKnowledgeDigest(value string) bool {
	if len(value) != 64 || strings.ToLower(value) != value {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size
}

func craftKnowledgeFilesystemDigest(files map[string][]byte) string {
	paths := sortedCraftKnowledgeFiles(files)
	h := sha256.New()
	for _, filePath := range paths {
		_, _ = fmt.Fprintf(h, "%d:%s%d:", len(filePath), filePath, len(files[filePath]))
		_, _ = h.Write(files[filePath])
	}
	return hex.EncodeToString(h.Sum(nil))
}

func sortedCraftKnowledgeFiles(files map[string][]byte) []string {
	paths := make([]string, 0, len(files))
	for filePath := range files {
		paths = append(paths, filePath)
	}
	sort.Strings(paths)
	return paths
}

func parseCraftKnowledgeRef(ref string) (kbID, knowledgeID, chunkID string, ok bool) {
	const prefix = "craftkb://kb/"
	if !strings.HasPrefix(ref, prefix) {
		return "", "", "", false
	}
	parts := strings.Split(strings.TrimPrefix(ref, prefix), "/")
	if len(parts) != 5 || parts[1] != "knowledge" || parts[3] != "chunk" ||
		!validCraftKnowledgePathToken(parts[0]) || !validCraftKnowledgePathToken(parts[2]) || !validCraftKnowledgePathToken(parts[4]) {
		return "", "", "", false
	}
	return parts[0], parts[2], parts[4], true
}

func sameCraftKnowledgePackage(left, right service.CraftKnowledgeMaterialPackage) bool {
	if left.RunID != right.RunID || left.Directory != right.Directory || left.Digest != right.Digest || len(left.Files) != len(right.Files) {
		return false
	}
	for filePath, leftContent := range left.Files {
		rightContent, ok := right.Files[filePath]
		if !ok || string(leftContent) != string(rightContent) {
			return false
		}
	}
	return true
}
