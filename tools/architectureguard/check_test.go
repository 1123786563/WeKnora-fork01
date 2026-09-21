package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// violationsManifest pairs with writeViolationsRepo: every discovered asset
// is owned except the route router.RegisterFooRoutes; task.type.foo breaks
// Redis/Lite parity; module a imports module b's adapters package; module
// b's domain ROOT package imports module a's public root; one temporary
// exception is expired at wave 2.
const violationsManifest = `schema_version: 1
scope:
  include: [internal]
  exclude: []
modules:
  - {id: a, target_prefix: internal/modules/a}
  - {id: b, target_prefix: internal/modules/b}
platform:
  id: platform
  target_prefixes: [internal/platform]
assets:
  - kind: route
    id: platform.owned-routes
    owner: platform
    source: internal/router/router.go
    symbol: RegisterOwnedRoutes
    target: internal/platform/transport/http
  - kind: worker
    id: a.bar
    owner: a
    source: internal/router/task.go
    task_type: task.type.bar
    target: internal/modules/a/application
  - kind: worker
    id: a.foo
    owner: a
    source: internal/router/task.go
    task_type: task.type.foo
    target: internal/modules/a/application
  - kind: lifecycle
    id: platform.boot
    owner: platform
    source: internal/container/container.go
    symbol: registerBoot
    target: internal/platform
  - kind: migration
    id: migration.versioned.000001_init
    owner: platform
    source: migrations/versioned/000001_init.up.sql
    target: internal/platform/migrations
  - kind: package
    id: pkg.internal.router
    owner: platform
    source: internal/router
    target: internal/platform/router
  - kind: package
    id: pkg.internal.container
    owner: platform
    source: internal/container
    target: internal/platform/container
  - kind: package
    id: pkg.internal.legacy
    owner: b
    source: internal/legacy
    target: internal/modules/b
  - kind: package
    id: pkg.internal.modules.a
    owner: a
    source: internal/modules/a
    target: internal/modules/a
  - kind: package
    id: pkg.internal.modules.b
    owner: b
    source: internal/modules/b
    target: internal/modules/b
  - kind: package
    id: pkg.internal.modules.b.adapters
    owner: b
    source: internal/modules/b/adapters
    target: internal/modules/b/adapters
  - kind: package
    id: pkg.internal.modules.b.domain
    owner: b
    source: internal/modules/b/domain
    target: internal/modules/b/domain
temporary_exceptions:
  - from: internal/modules/a
    to: internal/types
    reason: expired legacy dependency
    remove_in_wave: 1
  - from: internal/modules/b/legacy
    to: internal/types
    reason: still active at wave 2
    remove_in_wave: 5
`

// cleanManifest pairs with writeCleanRepo: complete ownership, balanced
// workers, no cross-module imports, only an active exception.
const cleanManifest = `schema_version: 1
scope:
  include: [internal]
  exclude: []
modules:
  - {id: a, target_prefix: internal/modules/a}
platform:
  id: platform
  target_prefixes: [internal/platform]
assets:
  - kind: route
    id: platform.owned-routes
    owner: platform
    source: internal/router/router.go
    symbol: RegisterOwnedRoutes
    target: internal/platform/transport/http
  - kind: worker
    id: a.bar
    owner: a
    source: internal/router/task.go
    task_type: task.type.bar
    target: internal/modules/a/application
  - kind: lifecycle
    id: platform.boot
    owner: platform
    source: internal/container/container.go
    symbol: registerBoot
    target: internal/platform
  - kind: package
    id: pkg.internal.router
    owner: platform
    source: internal/router
    target: internal/platform/router
  - kind: package
    id: pkg.internal.container
    owner: platform
    source: internal/container
    target: internal/platform/container
  - kind: package
    id: pkg.internal.legacy
    owner: a
    source: internal/legacy
    target: internal/modules/a
  - kind: package
    id: pkg.internal.modules.a
    owner: a
    source: internal/modules/a
    target: internal/modules/a
temporary_exceptions:
  - from: internal/modules/a/legacy
    to: internal/types
    reason: active during the migration
    remove_in_wave: 5
`

// writeViolationsRepo materializes a miniature repository containing a route
// registration, a Redis worker set, a Lite worker set (missing one task), a
// lifecycle hook, a migration pair, and an illegal cross-module import.
func writeViolationsRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write := func(rel, content string) {
		abs := filepath.Join(root, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(abs), 0o755))
		require.NoError(t, os.WriteFile(abs, []byte(content), 0o644))
	}
	write("go.mod", "module example.com/fixture\n\ngo 1.26\n")
	write("internal/router/router.go", `package router

func registerRoutes() {
	RegisterOwnedRoutes()
	router.RegisterFooRoutes()
}
`)
	write("internal/router/task.go", `package router

func registerRedisWorkers() {
	mux.HandleFunc("task.type.bar", handleBar)
	mux.HandleFunc("task.type.foo", handleFoo)
}
`)
	write("internal/router/sync_task.go", `package router

func registerLiteWorkers() {
	executor.RegisterHandler("task.type.bar", handleBar)
}
`)
	write("internal/container/container.go", `package container

func wire() {
	must(container.Invoke(registerBoot))
}
`)
	write("migrations/versioned/000001_init.up.sql", "CREATE TABLE t();\n")
	write("migrations/versioned/000001_init.down.sql", "DROP TABLE t;\n")
	write("internal/legacy/legacy.go", "package legacy\n")
	write("internal/modules/a/a.go", `package a

import _ "example.com/fixture/internal/modules/b/adapters"
`)
	write("internal/modules/b/b.go", "package b\n")
	write("internal/modules/b/adapters/adapters.go", "package adapters\n")
	// A domain ROOT package (path ends in /domain, no trailing segment):
	// even importing another module's public root is forbidden from here.
	write("internal/modules/b/domain/domain.go", `package domain

import _ "example.com/fixture/internal/modules/a"
`)
	return root
}

// writeCleanRepo materializes a miniature repository with balanced workers
// and no cross-module imports.
func writeCleanRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write := func(rel, content string) {
		abs := filepath.Join(root, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(abs), 0o755))
		require.NoError(t, os.WriteFile(abs, []byte(content), 0o644))
	}
	write("go.mod", "module example.com/fixture\n\ngo 1.26\n")
	write("internal/router/router.go", `package router

func registerRoutes() {
	RegisterOwnedRoutes()
}
`)
	write("internal/router/task.go", `package router

func registerRedisWorkers() {
	mux.HandleFunc("task.type.bar", handleBar)
}
`)
	write("internal/router/sync_task.go", `package router

func registerLiteWorkers() {
	executor.RegisterHandler("task.type.bar", handleBar)
}
`)
	write("internal/container/container.go", `package container

func wire() {
	must(container.Invoke(registerBoot))
}
`)
	write("internal/legacy/legacy.go", "package legacy\n")
	write("internal/modules/a/a.go", "package a\n")
	return root
}

func writeManifest(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "manifest.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func violationLines(vs []Violation) []string {
	lines := make([]string, 0, len(vs))
	for _, v := range vs {
		lines = append(lines, v.Message)
	}
	return lines
}

func TestDiscoverReturnsStableIDs(t *testing.T) {
	root := writeViolationsRepo(t)
	first, err := Discover(root)
	require.NoError(t, err)

	var routeIDs []string
	for _, r := range first.Routes {
		routeIDs = append(routeIDs, r.ID)
	}
	require.Equal(t, []string{"router.RegisterFooRoutes", "router.RegisterOwnedRoutes"}, routeIDs)
	require.Equal(t, []string{"task.type.bar", "task.type.foo"}, first.RedisTaskTypes)
	require.Equal(t, []string{"task.type.bar"}, first.LiteTaskTypes)
	var lifecycleIDs []string
	for _, l := range first.Lifecycles {
		lifecycleIDs = append(lifecycleIDs, l.ID)
	}
	require.Equal(t, []string{"registerBoot"}, lifecycleIDs)
	require.Equal(t, []string{"migration.versioned.000001_init"}, first.Migrations)
	require.Equal(t, []string{
		"internal/container",
		"internal/legacy",
		"internal/modules/a",
		"internal/modules/b",
		"internal/modules/b/adapters",
		"internal/modules/b/domain",
		"internal/router",
	}, first.Packages)

	second, err := Discover(root)
	require.NoError(t, err)
	require.Equal(t, first, second)
}

func TestCheckReportsUnownedRouteParityForbiddenImportAndExpiredException(t *testing.T) {
	root := writeViolationsRepo(t)
	manifestPath := writeManifest(t, root, violationsManifest)
	m, err := LoadManifest(manifestPath)
	require.NoError(t, err)
	m.CurrentWave = 2

	d, err := Discover(root)
	require.NoError(t, err)

	require.Equal(t, []string{
		"expired exception: internal/modules/a -> internal/types remove_in_wave=1 current_wave=2",
		"forbidden import: internal/modules/a -> internal/modules/b/adapters",
		"forbidden import: internal/modules/b/domain -> internal/modules/a",
		"unowned route: router.RegisterFooRoutes",
		"worker parity mismatch: task.type.foo missing from lite",
	}, violationLines(Check(m, d)))
}

func TestCheckPassesCleanRepository(t *testing.T) {
	root := writeCleanRepo(t)
	manifestPath := writeManifest(t, root, cleanManifest)
	m, err := LoadManifest(manifestPath)
	require.NoError(t, err)
	m.CurrentWave = 2

	d, err := Discover(root)
	require.NoError(t, err)
	require.Empty(t, violationLines(Check(m, d)))
}

// TestCheckFlagsDomainRootImportingOtherModule proves the domain isolation
// rule also covers a domain ROOT package (path ending in "/domain"): such a
// package may not import even another module's public root package.
func TestCheckFlagsDomainRootImportingOtherModule(t *testing.T) {
	root := writeViolationsRepo(t)
	manifestPath := writeManifest(t, root, violationsManifest)
	m, err := LoadManifest(manifestPath)
	require.NoError(t, err)
	m.CurrentWave = 2

	d, err := Discover(root)
	require.NoError(t, err)

	var domainEdges []ImportEdge
	for _, e := range d.Imports {
		if isDomainPackage(e.Package) {
			domainEdges = append(domainEdges, e)
		}
	}
	require.Len(t, domainEdges, 1)
	require.Equal(t, "internal/modules/b/domain", domainEdges[0].Package)
	require.Equal(t, "internal/modules/a", domainEdges[0].Path)

	require.Contains(t, violationLines(Check(m, d)),
		"forbidden import: internal/modules/b/domain -> internal/modules/a")
}

// TestCheckFlagsTargetPinnedToSourceOutsideOwnerPrefixes proves that
// target == source is no longer an escape hatch: an asset whose target sits
// outside its owner's prefixes is flagged even when the target equals the
// source.
func TestCheckFlagsTargetPinnedToSourceOutsideOwnerPrefixes(t *testing.T) {
	root := writeCleanRepo(t)
	manifestPath := writeManifest(t, root, cleanManifest)
	m, err := LoadManifest(manifestPath)
	require.NoError(t, err)
	m.CurrentWave = 2

	pinned := 0
	for i := range m.Assets {
		if m.Assets[i].ID == "pkg.internal.router" || m.Assets[i].ID == "pkg.internal.container" {
			m.Assets[i].Target = m.Assets[i].Source
			pinned++
		}
	}
	require.Equal(t, 2, pinned)

	d, err := Discover(root)
	require.NoError(t, err)

	lines := violationLines(Check(m, d))
	require.Contains(t, lines, "invalid target: asset pkg.internal.router target internal/router outside owner platform prefixes")
	require.Contains(t, lines, "invalid target: asset pkg.internal.container target internal/container outside owner platform prefixes")
}

func TestCheckReportsMissingRouteFromTestdataManifest(t *testing.T) {
	root := t.TempDir()
	write := func(rel, content string) {
		abs := filepath.Join(root, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(abs), 0o755))
		require.NoError(t, os.WriteFile(abs, []byte(content), 0o644))
	}
	write("go.mod", "module example.com/fixture\n\ngo 1.26\n")
	write("internal/router/routes.go", `package router

func registerRoutes() {
	RegisterOwnedRoutes()
	RegisterGhostRoutes()
}
`)

	m, err := LoadManifest("testdata/missing-route.yaml")
	require.NoError(t, err)
	m.CurrentWave = 1

	d, err := Discover(root)
	require.NoError(t, err)
	lines := violationLines(Check(m, d))
	require.Contains(t, lines, "unowned route: router.RegisterGhostRoutes")
}
