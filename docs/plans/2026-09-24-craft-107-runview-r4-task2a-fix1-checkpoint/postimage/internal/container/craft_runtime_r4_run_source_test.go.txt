package container

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/stretchr/testify/require"
)

func TestCraftRunViewR4RunBoundArtifactSourceListsAndReadsOnlyVerifiedOutput(t *testing.T) {
	f := newR4SeedExecuteFixture(t, nil, 3, nil, false)
	assetDir := filepath.Join(f.material.output, "assets")
	require.NoError(t, os.Mkdir(assetDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(f.material.output, "index.html"), []byte("run B"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(assetDir, "app.js"), []byte("B asset"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(f.material.inputs, "input-canary"), []byte("not output"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(f.material.knowledge, "knowledge-canary"), []byte("not output"), 0o644))
	sibling := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(sibling, "sibling-secret.html"), []byte("foreign"), 0o644))
	require.NoError(t, os.Symlink(sibling, filepath.Join(f.runtime.workDir, craftLocalOutputDir)))
	source, err := newRunBoundCraftArtifactSource(context.Background(), f.runtime, f.task, f.material, craftLocalOutputDir)
	require.NoError(t, err)

	entries, err := source.ListSessionFiles(context.Background(), f.task.Scope.SessionID, craftLocalOutputDir)
	require.NoError(t, err)
	require.Len(t, entries, 3)
	require.Equal(t, "output/assets", entries[0].Path)
	require.Equal(t, "output/assets/app.js", entries[1].Path)
	require.Equal(t, "output/index.html", entries[2].Path)
	data, err := source.ReadSessionFile(context.Background(), f.task.Scope.SessionID, "output/assets/app.js")
	require.NoError(t, err)
	require.Equal(t, []byte("B asset"), data)

	_, err = source.ReadSessionFile(context.Background(), "foreign-session", "output/index.html")
	require.ErrorIs(t, err, craft.ErrForbidden)
	_, err = source.ListSessionFiles(context.Background(), f.task.Scope.SessionID, "other-output")
	require.ErrorIs(t, err, craft.ErrForbidden)
	_, err = source.ReadSessionFile(context.Background(), f.task.Scope.SessionID, "output/../output/index.html")
	require.ErrorIs(t, err, craft.ErrInvalidInput)
}

func TestCraftRunViewR4RunBoundArtifactSourceRejectsForeignRunHandle(t *testing.T) {
	f := newR4SeedExecuteFixture(t, nil, 3, nil, false)
	foreign := f.task
	foreign.Fence.RunID = "run-a"
	_, err := newRunBoundCraftArtifactSource(context.Background(), f.runtime, foreign, f.material, craftLocalOutputDir)
	require.ErrorIs(t, err, craft.ErrForbidden)
}

func TestCraftRunViewR4RunBoundArtifactSourceRejectsSymlinkAndHardlink(t *testing.T) {
	t.Run("symlink", func(t *testing.T) {
		f := newR4SeedExecuteFixture(t, nil, 3, nil, false)
		target := filepath.Join(t.TempDir(), "outside")
		require.NoError(t, os.WriteFile(target, []byte("outside"), 0o644))
		require.NoError(t, os.Symlink(target, filepath.Join(f.material.output, "link.html")))
		source, err := newRunBoundCraftArtifactSource(context.Background(), f.runtime, f.task, f.material, craftLocalOutputDir)
		require.NoError(t, err)
		_, err = source.ListSessionFiles(context.Background(), f.task.Scope.SessionID, craftLocalOutputDir)
		require.ErrorIs(t, err, craft.ErrInvalidInput)
	})

	t.Run("hard link", func(t *testing.T) {
		f := newR4SeedExecuteFixture(t, nil, 3, nil, false)
		original := filepath.Join(f.material.output, "index.html")
		require.NoError(t, os.WriteFile(original, []byte("private"), 0o644))
		require.NoError(t, os.Link(original, filepath.Join(f.material.output, "linked.html")))
		source, err := newRunBoundCraftArtifactSource(context.Background(), f.runtime, f.task, f.material, craftLocalOutputDir)
		require.NoError(t, err)
		_, err = source.ListSessionFiles(context.Background(), f.task.Scope.SessionID, craftLocalOutputDir)
		require.ErrorIs(t, err, craft.ErrInvalidInput)
	})
}

func TestCraftRunViewR4RunBoundArtifactSourceRejectsListReadReplacement(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*testing.T, string)
		replace func(*testing.T, string)
		listed  string
	}{
		{
			name: "file",
			setup: func(t *testing.T, root string) {
				require.NoError(t, os.WriteFile(filepath.Join(root, "index.html"), []byte("same bytes"), 0o644))
			},
			replace: func(t *testing.T, root string) {
				old := filepath.Join(root, "old.html")
				require.NoError(t, os.Rename(filepath.Join(root, "index.html"), old))
				require.NoError(t, os.WriteFile(filepath.Join(root, "index.html"), []byte("same bytes"), 0o644))
				require.NoError(t, os.Remove(old))
			},
			listed: "output/index.html",
		},
		{
			name: "same inode bytes",
			setup: func(t *testing.T, root string) {
				require.NoError(t, os.WriteFile(filepath.Join(root, "index.html"), []byte("same bytes"), 0o644))
			},
			replace: func(t *testing.T, root string) {
				file, err := os.OpenFile(filepath.Join(root, "index.html"), os.O_WRONLY|os.O_TRUNC, 0)
				require.NoError(t, err)
				_, err = file.Write([]byte("diff bytes"))
				require.NoError(t, err)
				require.NoError(t, file.Close())
			},
			listed: "output/index.html",
		},
		{
			name: "intermediate directory",
			setup: func(t *testing.T, root string) {
				dir := filepath.Join(root, "assets")
				require.NoError(t, os.Mkdir(dir, 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(dir, "app.js"), []byte("same bytes"), 0o644))
			},
			replace: func(t *testing.T, root string) {
				require.NoError(t, os.Rename(filepath.Join(root, "assets"), filepath.Join(root, "old-assets")))
				dir := filepath.Join(root, "assets")
				require.NoError(t, os.Mkdir(dir, 0o755))
				require.NoError(t, os.WriteFile(filepath.Join(dir, "app.js"), []byte("same bytes"), 0o644))
			},
			listed: "output/assets/app.js",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newR4SeedExecuteFixture(t, nil, 3, nil, false)
			tc.setup(t, f.material.output)
			source, err := newRunBoundCraftArtifactSource(context.Background(), f.runtime, f.task, f.material, craftLocalOutputDir)
			require.NoError(t, err)
			_, err = source.ListSessionFiles(context.Background(), f.task.Scope.SessionID, craftLocalOutputDir)
			require.NoError(t, err)
			tc.replace(t, f.material.output)
			_, err = source.ReadSessionFile(context.Background(), f.task.Scope.SessionID, tc.listed)
			require.ErrorIs(t, err, craft.ErrConflict)
		})
	}
}

func TestCraftRunViewR4RunBoundArtifactSourceRejectsChangedGenerationLayout(t *testing.T) {
	f := newR4SeedExecuteFixture(t, nil, 3, nil, false)
	require.NoError(t, os.WriteFile(filepath.Join(f.material.output, "index.html"), []byte("B"), 0o644))
	source, err := newRunBoundCraftArtifactSource(context.Background(), f.runtime, f.task, f.material, craftLocalOutputDir)
	require.NoError(t, err)
	_, err = source.ListSessionFiles(context.Background(), f.task.Scope.SessionID, craftLocalOutputDir)
	require.NoError(t, err)
	require.NoError(t, os.Rename(f.material.output, f.material.output+".old"))
	require.NoError(t, os.Mkdir(f.material.output, 0o755))
	_, err = source.ReadSessionFile(context.Background(), f.task.Scope.SessionID, "output/index.html")
	require.Error(t, err)
}

func TestCraftRunViewR4RunBoundArtifactSourceRechecksRunEpochBetweenListAndRead(t *testing.T) {
	f := newR4SeedExecuteFixture(t, nil, 3, nil, false)
	require.NoError(t, os.WriteFile(filepath.Join(f.material.output, "index.html"), []byte("B"), 0o644))
	source, err := newRunBoundCraftArtifactSource(context.Background(), f.runtime, f.task, f.material, craftLocalOutputDir)
	require.NoError(t, err)
	_, err = source.ListSessionFiles(context.Background(), f.task.Scope.SessionID, craftLocalOutputDir)
	require.NoError(t, err)
	require.NoError(t, f.db.Exec("UPDATE agent_runs SET epoch = 4 WHERE tenant_id = 1 AND run_id = 'run-b'").Error)
	_, err = source.ReadSessionFile(context.Background(), f.task.Scope.SessionID, "output/index.html")
	require.ErrorIs(t, err, craft.ErrConflict)
}

func TestCraftRunViewR4RunBoundArtifactSourceClearsSnapshotAfterFailedRelist(t *testing.T) {
	f := newR4SeedExecuteFixture(t, nil, 3, nil, false)
	require.NoError(t, os.WriteFile(filepath.Join(f.material.output, "index.html"), []byte("B"), 0o644))
	source, err := newRunBoundCraftArtifactSource(context.Background(), f.runtime, f.task, f.material, craftLocalOutputDir)
	require.NoError(t, err)
	_, err = source.ListSessionFiles(context.Background(), f.task.Scope.SessionID, craftLocalOutputDir)
	require.NoError(t, err)
	bad := filepath.Join(f.material.output, "bad-link")
	require.NoError(t, os.Symlink(filepath.Join(t.TempDir(), "outside"), bad))
	_, err = source.ListSessionFiles(context.Background(), f.task.Scope.SessionID, craftLocalOutputDir)
	require.ErrorIs(t, err, craft.ErrInvalidInput)
	require.NoError(t, os.Remove(bad))
	_, err = source.ReadSessionFile(context.Background(), f.task.Scope.SessionID, "output/index.html")
	require.ErrorIs(t, err, craft.ErrConflict, "a failed list cannot leave an older file identity usable")
}

func TestCraftRunViewR4RunBoundArtifactSourceBoundsZeroByteAndDirectoryEntries(t *testing.T) {
	const maxEntriesPerDirectory = 1024
	const maxEntries = 4096
	t.Run("per directory zero-byte files", func(t *testing.T) {
		f := newR4SeedExecuteFixture(t, nil, 3, nil, false)
		for i := 0; i <= maxEntriesPerDirectory; i++ {
			name := filepath.Join(f.material.output, fmt.Sprintf("empty-%04d.txt", i))
			require.NoError(t, os.WriteFile(name, nil, 0o644))
		}
		source, err := newRunBoundCraftArtifactSource(context.Background(), f.runtime, f.task, f.material, craftLocalOutputDir)
		require.NoError(t, err)
		_, err = source.ListSessionFiles(context.Background(), f.task.Scope.SessionID, craftLocalOutputDir)
		require.ErrorIs(t, err, craft.ErrInvalidInput)
	})

	t.Run("global nested empty directories", func(t *testing.T) {
		f := newR4SeedExecuteFixture(t, nil, 3, nil, false)
		for i := 0; i < maxEntriesPerDirectory/maxEntries+6; i++ {
			parent := filepath.Join(f.material.output, fmt.Sprintf("dir-%02d", i))
			require.NoError(t, os.Mkdir(parent, 0o755))
			for j := 0; j < maxEntriesPerDirectory-1; j++ {
				require.NoError(t, os.Mkdir(filepath.Join(parent, fmt.Sprintf("empty-%03d", j)), 0o755))
			}
		}
		source, err := newRunBoundCraftArtifactSource(context.Background(), f.runtime, f.task, f.material, craftLocalOutputDir)
		require.NoError(t, err)
		_, err = source.ListSessionFiles(context.Background(), f.task.Scope.SessionID, craftLocalOutputDir)
		require.ErrorIs(t, err, craft.ErrInvalidInput)
	})
}

func TestCraftRunViewR4RunBoundArtifactSourceRejectsDirectorySetChanges(t *testing.T) {
	t.Run("insertion during root enumeration", func(t *testing.T) {
		f := newR4SeedExecuteFixture(t, nil, 3, nil, false)
		require.NoError(t, os.WriteFile(filepath.Join(f.material.output, "index.html"), []byte("listed"), 0o644))
		source, err := newRunBoundCraftArtifactSource(context.Background(), f.runtime, f.task, f.material, craftLocalOutputDir)
		require.NoError(t, err)
		var inserted atomic.Bool
		source.afterDirectoryRead = func(rel string) {
			if rel == "" && inserted.CompareAndSwap(false, true) {
				require.NoError(t, os.WriteFile(filepath.Join(f.material.output, "inserted.html"), []byte("late"), 0o644))
			}
		}
		_, err = source.ListSessionFiles(context.Background(), f.task.Scope.SessionID, craftLocalOutputDir)
		require.ErrorIs(t, err, craft.ErrConflict)
	})
	t.Run("nested insertion after list", func(t *testing.T) {
		f := newR4SeedExecuteFixture(t, nil, 3, nil, false)
		assetDir := filepath.Join(f.material.output, "assets")
		require.NoError(t, os.Mkdir(assetDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(assetDir, "app.js"), []byte("listed"), 0o644))
		source, err := newRunBoundCraftArtifactSource(context.Background(), f.runtime, f.task, f.material, craftLocalOutputDir)
		require.NoError(t, err)
		_, err = source.ListSessionFiles(context.Background(), f.task.Scope.SessionID, craftLocalOutputDir)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(assetDir, "inserted.js"), []byte("late"), 0o644))
		_, err = source.ReadSessionFile(context.Background(), f.task.Scope.SessionID, "output/assets/app.js")
		require.ErrorIs(t, err, craft.ErrConflict)
	})

	for _, mutation := range []string{"insert", "remove", "rename"} {
		t.Run("after list "+mutation, func(t *testing.T) {
			f := newR4SeedExecuteFixture(t, nil, 3, nil, false)
			require.NoError(t, os.WriteFile(filepath.Join(f.material.output, "index.html"), []byte("listed"), 0o644))
			source, err := newRunBoundCraftArtifactSource(context.Background(), f.runtime, f.task, f.material, craftLocalOutputDir)
			require.NoError(t, err)
			_, err = source.ListSessionFiles(context.Background(), f.task.Scope.SessionID, craftLocalOutputDir)
			require.NoError(t, err)
			switch mutation {
			case "insert":
				require.NoError(t, os.WriteFile(filepath.Join(f.material.output, "inserted.html"), []byte("late"), 0o644))
			case "remove":
				require.NoError(t, os.Remove(filepath.Join(f.material.output, "index.html")))
			case "rename":
				require.NoError(t, os.Rename(filepath.Join(f.material.output, "index.html"), filepath.Join(f.material.output, "renamed.html")))
			}
			_, err = source.ReadSessionFile(context.Background(), f.task.Scope.SessionID, "output/index.html")
			require.ErrorIs(t, err, craft.ErrConflict)
		})
	}
}

func TestCraftRunViewR4RunBoundArtifactSourceRejectsChangedOutputMount(t *testing.T) {
	f := newR4SeedExecuteFixture(t, nil, 3, nil, false)
	engine, ok := f.material.provider.engine.(*fakeCraftRunViewContainerEngine)
	require.True(t, ok)
	spec := craftRunViewSpecForGeneration(f.material.generation)
	engine.mu.Lock()
	container := engine.containers[spec.ContainerID]
	for i := range container.Mounts {
		if container.Mounts[i].Destination == filepath.ToSlash(filepath.Join(spec.Directory, "output")) {
			container.Mounts[i].Source = "/tmp/other-run-output"
		}
	}
	engine.containers[spec.ContainerID] = container
	engine.mu.Unlock()
	source, err := newRunBoundCraftArtifactSource(context.Background(), f.runtime, f.task, f.material, craftLocalOutputDir)
	require.Error(t, err)
	require.Nil(t, source)
}

type r4Task2aSucceededExecutor struct {
	calls      int
	outputRoot string
}

func (e *r4Task2aSucceededExecutor) Execute(context.Context, craft.Task) (craft.Result, error) {
	e.calls++
	if err := os.WriteFile(filepath.Join(e.outputRoot, "index.html"), []byte("candidate"), 0o644); err != nil {
		return craft.Result{}, err
	}
	return craft.Result{Status: "succeeded"}, nil
}
func (*r4Task2aSucceededExecutor) Observe(context.Context, craft.Task) (craft.Observation, error) {
	return craft.Observation{}, nil
}
func (*r4Task2aSucceededExecutor) Abort(context.Context, craft.Task) error { return nil }

func TestCraftRunViewR4ExecuteDoesNotPublishThroughLegacySessionCollector(t *testing.T) {
	f := newR4SeedExecuteFixture(t, nil, 3, nil, false)
	require.NoError(t, f.db.Exec(`CREATE TABLE craft_versions (id TEXT PRIMARY KEY)`).Error)
	inner := &r4Task2aSucceededExecutor{outputRoot: f.material.output}
	f.runtime.inner = inner
	f.runtime.artifacts = service.NewCraftArtifactService(
		&localCraftArtifactSource{workDir: f.runtime.workDir, outputDir: craftLocalOutputDir},
		f.files, repository.NewCraftVersionStore(f.db), nil,
		service.CraftArtifactConfig{Kind: craft.KindWeb, OutputDir: craftLocalOutputDir},
	)
	_, err := f.runtime.Execute(context.Background(), f.task)
	require.ErrorIs(t, err, ErrCraftRunViewRuntimeUnresolved)
	require.Contains(t, err.Error(), "candidate collection is not assembled")
	require.Equal(t, 1, inner.calls)
	var versions int64
	require.NoError(t, f.db.Table("craft_versions").Count(&versions).Error)
	require.Zero(t, versions, "Task2a must not publish a Version through the legacy session-wide collector")
}
