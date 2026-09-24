package codedelivery

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// NewLocalWorkspaceSource maps the workspace port onto a local directory
// (tests, single-box dev). Paths are workspace-absolute ("/workspace/…" or
// the cleaned request dir); traversal outside the root is refused.
func NewLocalWorkspaceSource(root string) (WorkspaceFileSource, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	return &localWorkspaceSource{root: abs}, nil
}

type localWorkspaceSource struct {
	root string
}

// resolve maps a workspace-absolute path onto the local root. It refuses
// anything that could escape the root: cleaned "." / ".." segments anywhere
// in the path (same ".." whitelist posture as DeliveryMaterial file paths).
func (s *localWorkspaceSource) resolve(p string) (string, error) {
	clean := path.Clean(strings.TrimPrefix(strings.TrimSpace(p), "/workspace"))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains(clean, "..") {
		return "", fmt.Errorf("%w: illegal workspace path %q", ErrInvalidMaterial, p)
	}
	return filepath.Join(s.root, filepath.FromSlash(clean)), nil
}

// ListSessionFiles walks dir and returns entries whose Path carries the
// CLEANED REQUEST DIR as prefix — the session-sandbox convention
// ("/workspace/<owner>/<name>/…"), so workspaceTree's repo-root prefix strip
// behaves identically for both adapters. A missing dir lists empty
// (nothing materialized yet), never an error.
func (s *localWorkspaceSource) ListSessionFiles(ctx context.Context, sessionID, dir string) ([]WorkspaceDirEntry, error) {
	root, err := s.resolve(dir)
	if err != nil {
		return nil, err
	}
	// 前缀=清洗后的请求目录本身（保留 /workspace 前缀，与沙箱同约定；
	// 计划稿此处 TrimPrefix 掉 "/workspace" 会让 workspaceTree 的
	// rootPrefix 裁剪永不命中、diff 退化为全量删除——偏差 D2）。
	prefix := path.Clean(strings.TrimSpace(dir))
	var out []WorkspaceDirEntry
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil // nothing materialized yet
			}
			return err
		}
		if p == root {
			return nil
		}
		rel, rerr := filepath.Rel(root, p)
		if rerr != nil {
			return rerr
		}
		info, ierr := d.Info()
		if ierr != nil {
			return ierr
		}
		out = append(out, WorkspaceDirEntry{Path: prefix + "/" + path.Clean(filepath.ToSlash(rel)), IsDir: d.IsDir(), Size: info.Size()})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (s *localWorkspaceSource) ReadSessionFile(ctx context.Context, sessionID, p string) ([]byte, error) {
	full, err := s.resolve(p)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(full)
}

func (s *localWorkspaceSource) WriteSessionWorkspaceFiles(ctx context.Context, sessionID string, files []WorkspaceFileWrite) error {
	for _, f := range files {
		full, err := s.resolve(f.Path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(full, f.Content, 0o644); err != nil {
			return err
		}
	}
	return nil
}
