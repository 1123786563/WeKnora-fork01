package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadManifestAcceptsValidManifest(t *testing.T) {
	m, err := LoadManifest("testdata/valid.yaml")
	require.NoError(t, err)
	require.Equal(t, 1, m.SchemaVersion)
	require.Equal(t, []string{"internal"}, m.Scope.Include)
	require.Equal(t, []string{"internal/debug"}, m.Scope.Exclude)
	require.Len(t, m.Modules, 2)
	require.Equal(t, "alpha", m.Modules[0].ID)
	require.Equal(t, "internal/modules/alpha", m.Modules[0].TargetPrefix)
	require.Equal(t, "platform", m.Platform.ID)
	require.Equal(t, []string{"internal/platform"}, m.Platform.TargetPrefixes)
	require.Len(t, m.Assets, 5)
	require.Equal(t, "route", m.Assets[0].Kind)
	require.Equal(t, "RegisterHelloRoutes", m.Assets[0].Symbol)
	require.Equal(t, "worker", m.Assets[1].Kind)
	require.Equal(t, "TypeGreet", m.Assets[1].TaskType)
	require.Equal(t, "lifecycle", m.Assets[2].Kind)
	require.Equal(t, "migration", m.Assets[3].Kind)
	require.Equal(t, "package", m.Assets[4].Kind)
	require.Len(t, m.TemporaryExceptions, 1)
	require.Equal(t, "internal/modules/beta/legacy", m.TemporaryExceptions[0].From)
	require.Equal(t, "internal/application/repository", m.TemporaryExceptions[0].To)
	require.Equal(t, 5, m.TemporaryExceptions[0].RemoveInWave)
}

func TestLoadManifestRejectsUnknownAndDuplicateOwners(t *testing.T) {
	_, err := LoadManifest("testdata/duplicate-owner.yaml")
	require.ErrorContains(t, err, "duplicate asset id")
}

func TestLoadManifestRejectsWildcardException(t *testing.T) {
	_, err := LoadManifest("testdata/forbidden-import.yaml")
	require.ErrorContains(t, err, "exception paths must be exact prefixes")
}

func TestLoadManifestRejectsUnknownOwner(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.yaml")
	content := `schema_version: 1
scope:
  include: [internal]
  exclude: []
modules:
  - {id: alpha, target_prefix: internal/modules/alpha}
platform:
  id: platform
  target_prefixes: [internal/platform]
assets:
  - kind: package
    id: pkg.ghost
    owner: ghost
    source: internal/ghost
    target: internal/modules/alpha
temporary_exceptions: []
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	_, err := LoadManifest(path)
	require.ErrorContains(t, err, "unknown owner")
}

func TestLoadManifestRejectsUnknownFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.yaml")
	content := `schema_version: 1
scope:
  include: [internal]
  exclude: []
bogus_root: true
modules:
  - {id: alpha, target_prefix: internal/modules/alpha}
platform:
  id: platform
  target_prefixes: [internal/platform]
assets: []
temporary_exceptions: []
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	_, err := LoadManifest(path)
	require.Error(t, err)
}

func TestLoadManifestRejectsInvalidPathsAndWaves(t *testing.T) {
	cases := []struct {
		name    string
		asset   string
		except  string
		wantErr string
	}{
		{
			name:    "absolute source",
			asset:   "  - {kind: package, id: p1, owner: alpha, source: /abs/path, target: internal/modules/alpha}",
			except:  "",
			wantErr: "absolute path",
		},
		{
			name:    "escaping target",
			asset:   "  - {kind: package, id: p1, owner: alpha, source: internal/x, target: ../../etc}",
			except:  "",
			wantErr: `".."`,
		},
		{
			name:    "empty source",
			asset:   "  - {kind: package, id: p1, owner: alpha, source: '', target: internal/modules/alpha}",
			except:  "",
			wantErr: "empty source",
		},
		{
			name:    "route missing symbol",
			asset:   "  - {kind: route, id: r1, owner: alpha, source: internal/router/routes.go, target: internal/modules/alpha/transport/http}",
			except:  "",
			wantErr: "requires symbol",
		},
		{
			name:    "worker missing task_type",
			asset:   "  - {kind: worker, id: w1, owner: alpha, source: internal/router/task.go, target: internal/modules/alpha/application}",
			except:  "",
			wantErr: "requires task_type",
		},
		{
			name:    "remove_in_wave below one",
			asset:   "  - {kind: package, id: p1, owner: alpha, source: internal/x, target: internal/modules/alpha}",
			except:  "  - {from: internal/modules/alpha, to: internal/types, reason: r, remove_in_wave: 0}",
			wantErr: "remove_in_wave must be >= 1",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "manifest.yaml")
			content := `schema_version: 1
scope:
  include: [internal]
  exclude: []
modules:
  - {id: alpha, target_prefix: internal/modules/alpha}
platform:
  id: platform
  target_prefixes: [internal/platform]
assets:
` + tc.asset + `
temporary_exceptions:
` + exceptList(tc.except)
			require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
			_, err := LoadManifest(path)
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func exceptList(exc string) string {
	if exc == "" {
		return "  []\n"
	}
	return exc + "\n"
}
