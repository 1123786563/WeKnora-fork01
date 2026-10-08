package craft

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDraftHeadValidationAndDefensiveFiles(t *testing.T) {
	valid := DraftHead{WorkspaceID: "ws", Revision: 0, State: DraftHeadEmpty}
	require.NoError(t, valid.Validate())
	for _, bad := range []DraftHead{
		{WorkspaceID: "ws", Revision: -1, State: DraftHeadEmpty},
		{WorkspaceID: "ws", Revision: 0, State: DraftHeadSelected},
		{WorkspaceID: "ws", Revision: 0, State: DraftHeadEmpty, SourceRunID: "run"},
		{WorkspaceID: "ws", Revision: 0, State: DraftHeadEmpty, ManifestDigest: "x"},
		{WorkspaceID: "ws", Revision: 1, State: DraftHeadSelected, SourceRunID: "run"},
	} {
		require.ErrorIs(t, bad.Validate(), ErrInvalidInput)
	}

	files := []File{{Path: "index.html", Ref: "object://abc", SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Bytes: 10}}
	digest, err := ManifestDigest(files)
	require.NoError(t, err)
	head := DraftHead{WorkspaceID: "ws", Revision: 1, State: DraftHeadSelected, SourceRunID: "run", ManifestDigest: digest, Files: append([]File(nil), files...)}
	require.NoError(t, head.Validate())
	files[0].Ref = "mutated"
	require.Equal(t, "object://abc", head.Files[0].Ref)
	got := head.Clone()
	got.Files[0].Ref = "mutated-copy"
	require.Equal(t, "object://abc", head.Files[0].Ref)
}

func TestDraftHeadValidationRejectsIncompleteOrUnsafeManifest(t *testing.T) {
	valid := func(file File) DraftHead {
		digest, _ := ManifestDigest([]File{file})
		return DraftHead{WorkspaceID: "ws", Revision: 1, State: DraftHeadSelected, SourceRunID: "run", ManifestDigest: digest, Files: []File{file}}
	}
	base := File{Path: "index.html", Ref: "object://abc", SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Bytes: 10}
	for _, file := range []File{
		{Path: "index.html", SHA256: base.SHA256, Bytes: 1},
		{Path: "../index.html", Ref: base.Ref, SHA256: base.SHA256, Bytes: 1},
		{Path: "index.html", Ref: base.Ref, SHA256: "bad", Bytes: 1},
		{Path: "index.html", Ref: base.Ref, SHA256: base.SHA256, Bytes: -1},
	} {
		require.ErrorIs(t, valid(file).Validate(), ErrInvalidInput)
	}
	selectedWithoutFiles := DraftHead{WorkspaceID: "ws", Revision: 1, State: DraftHeadSelected, SourceRunID: "run", ManifestDigest: "x"}
	require.ErrorIs(t, selectedWithoutFiles.Validate(), ErrInvalidInput)
	require.False(t, errors.Is(ErrDraftHeadUnresolved, ErrNotFound))
	require.False(t, errors.Is(ErrDraftHeadUnresolved, nil))
	large := []File{
		{Path: "a.bin", Ref: "object://a", SHA256: base.SHA256, Bytes: MaxDraftHeadBytes},
		{Path: "b.bin", Ref: "object://b", SHA256: base.SHA256, Bytes: 1},
	}
	digest, err := ManifestDigest(large)
	require.NoError(t, err)
	heterogeneous := DraftHead{WorkspaceID: "ws", Revision: 1, State: DraftHeadSelected, SourceRunID: "run", ManifestDigest: digest, Files: large}
	require.ErrorIs(t, heterogeneous.Validate(), ErrInvalidInput, "total manifest bytes are bounded without integer overflow")
}
