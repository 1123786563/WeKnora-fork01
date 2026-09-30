package craft

import (
	"context"
	"fmt"
)

type DraftHeadState string

const (
	DraftHeadEmpty    DraftHeadState = "empty"
	DraftHeadSelected DraftHeadState = "selected"
	// MaxDraftHeadBytes bounds the complete selected manifest to the existing
	// artifact object ceiling. This also prevents integer overflow while
	// summing file sizes before a durable revision is inserted.
	MaxDraftHeadBytes int64 = 8 << 30
)

// ErrDraftHeadUnresolved marks an existing Workspace whose draft history is
// absent. It must never be interpreted as a known-empty Workspace.
var ErrDraftHeadUnresolved = fmt.Errorf("craft workspace draft head unresolved")

// DraftHead is the current immutable Workspace draft manifest.
type DraftHead struct {
	WorkspaceID    string
	Revision       int64
	State          DraftHeadState
	SourceRunID    string
	ManifestDigest string
	Files          []File
}

func (h DraftHead) Clone() DraftHead {
	h.Files = append([]File(nil), h.Files...)
	return h
}

// Validate enforces explicit empty and sealed selected states. The SQL layer
// separately binds this value to the authorized Workspace and Run.
func (h DraftHead) Validate() error {
	if h.WorkspaceID == "" || h.Revision < 0 {
		return fmt.Errorf("%w: invalid draft head identity or revision", ErrInvalidInput)
	}
	switch h.State {
	case DraftHeadEmpty:
		if h.Revision != 0 || h.SourceRunID != "" || h.ManifestDigest != "" || len(h.Files) != 0 {
			return fmt.Errorf("%w: empty draft head must be revision zero without manifest", ErrInvalidInput)
		}
	case DraftHeadSelected:
		if h.Revision < 1 || h.SourceRunID == "" || len(h.Files) == 0 {
			return fmt.Errorf("%w: selected draft head requires revision, source Run and files", ErrInvalidInput)
		}
		var total int64
		for _, file := range h.Files {
			if file.Ref == "" {
				return fmt.Errorf("%w: artifact %q has no immutable object ref", ErrInvalidInput, file.Path)
			}
			if file.Bytes < 0 || file.Bytes > MaxDraftHeadBytes || total > MaxDraftHeadBytes-file.Bytes {
				return fmt.Errorf("%w: draft manifest exceeds %d bytes", ErrInvalidInput, MaxDraftHeadBytes)
			}
			total += file.Bytes
		}
		digest, err := ManifestDigest(h.Files)
		if err != nil {
			return err
		}
		if digest != h.ManifestDigest {
			return fmt.Errorf("%w: draft manifest digest mismatch", ErrInvalidInput)
		}
	default:
		return fmt.Errorf("%w: unknown draft head state %q", ErrInvalidInput, h.State)
	}
	return nil
}

type DraftHeadStore interface {
	Read(context.Context, Scope, string) (DraftHead, error)
	ReadRevision(context.Context, Scope, string, int64) (DraftHead, error)
	Advance(context.Context, Scope, string, int64, string, []File) (DraftHead, error)
}
