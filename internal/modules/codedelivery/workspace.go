package codedelivery

import (
	"context"
	"errors"
)

// WorkspaceDirEntry is the provider-neutral workspace listing entry.
type WorkspaceDirEntry struct {
	Path  string
	IsDir bool
	Size  int64
}

// WorkspaceFileWrite is one workspace file mutation.
type WorkspaceFileWrite struct {
	Path    string
	Content []byte
}

// WorkspaceFileSource is the workspace port of the delivery module: the
// session-owned cloud workspace the run modified (CONTEXT.md 工作区). In
// production it is satisfied by *sandbox.SessionBoundManager (the container
// adapter lives in internal/container, mirroring the artifact collector's
// SandboxArtifactSource precedent); tests use NewLocalWorkspaceSource.
// This module never imports the sandbox package.
//
// Path convention (both adapters): ListSessionFiles returns entries prefixed
// with the CLEANED REQUEST DIR itself ("​/workspace/<owner>/<name>/…"), the
// same convention as the session sandbox — workspaceTree strips that prefix
// to repo-relative paths. Write/Read take absolute-in-workspace paths
// ("/workspace/<owner>/<name>/<rel>").
type WorkspaceFileSource interface {
	ListSessionFiles(ctx context.Context, sessionID, dir string) ([]WorkspaceDirEntry, error)
	ReadSessionFile(ctx context.Context, sessionID, path string) ([]byte, error)
	WriteSessionWorkspaceFiles(ctx context.Context, sessionID string, files []WorkspaceFileWrite) error
}

// ErrWorkspaceUnavailable marks a session without a usable live workspace.
var ErrWorkspaceUnavailable = errors.New("code_delivery_workspace_unavailable")
