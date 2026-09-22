package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/Tencent/WeKnora/internal/modules/craft"
)

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
