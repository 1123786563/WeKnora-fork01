package main

import (
	"fmt"
	"sort"
	"strings"
)

// Violation is one architecture-guard diagnostic line.
type Violation struct {
	Message string
}

func (v Violation) String() string { return v.Message }

// forbiddenTargetSegments are module-internal packages that another module
// must never import directly.
var forbiddenTargetSegments = []string{"/transport/", "/adapters/"}

// contractsSuffix names the explicit cross-module contract package that is
// importable besides a module's root public package.
const contractsSuffix = "/contracts"

// Check compares a manifest against the discovered repository state and
// returns every violation, sorted lexically for stable diagnostics. It
// enforces complete ownership of discovered assets, Redis/Lite worker
// parity, manifest target-prefix validity, duplicate asset ids, and the
// module dependency rules, including temporary-exception expiry against
// Manifest.CurrentWave.
func Check(m *Manifest, d *Discovery) []Violation {
	var vs []Violation
	add := func(format string, args ...any) {
		vs = append(vs, Violation{Message: fmt.Sprintf(format, args...)})
	}

	checkOwnership(m, d, add)
	checkWorkerParity(d, add)
	checkTargetPrefixes(m, add)
	checkDependencies(m, d, add)
	checkExpiredExceptions(m, add)

	sort.Slice(vs, func(i, j int) bool { return vs[i].Message < vs[j].Message })
	return vs
}

// checkOwnership reports every discovered asset that has no manifest entry
// and every duplicated discovered asset id.
func checkOwnership(m *Manifest, d *Discovery, add func(string, ...any)) {
	routes := indexAssets(m, KindRoute, func(a Asset) string { return a.Symbol })
	workers := indexAssets(m, KindWorker, func(a Asset) string { return a.TaskType })
	lifecycles := indexAssets(m, KindLifecycle, func(a Asset) string { return a.Symbol })
	migrations := indexAssets(m, KindMigration, func(a Asset) string { return a.ID })
	packages := indexAssets(m, KindPackage, func(a Asset) string { return a.Source })

	for _, r := range d.Routes {
		if !routes[r.Symbol] {
			add("unowned route: %s", r.ID)
		}
	}
	for _, w := range d.Workers {
		if !workers[w.ID] {
			add("unowned worker: %s", w.ID)
		}
	}
	for _, l := range d.Lifecycles {
		if !lifecycles[l.ID] {
			add("unowned lifecycle: %s", l.ID)
		}
	}
	for _, id := range d.Migrations {
		if !migrations[id] {
			add("unowned migration: %s", id)
		}
	}
	for _, dir := range d.Packages {
		if !packages[dir] {
			add("unowned package: %s", dir)
		}
	}

	dupes := duplicateIDs(collect(d.Routes, func(r RouteAsset) string { return r.ID }))
	dupes = append(dupes, duplicateIDs(collect(d.Workers, func(w WorkerAsset) string { return w.ID }))...)
	dupes = append(dupes, duplicateIDs(collect(d.Lifecycles, func(l LifecycleAsset) string { return l.ID }))...)
	dupes = append(dupes, duplicateIDs(d.Migrations)...)
	dupes = append(dupes, duplicateIDs(d.Packages)...)
	for _, id := range dupes {
		add("duplicate asset id: %s", id)
	}
}

// checkWorkerParity requires the Redis and Lite registration files to cover
// the same set of task types.
func checkWorkerParity(d *Discovery, add func(string, ...any)) {
	lite := map[string]bool{}
	for _, t := range d.LiteTaskTypes {
		lite[t] = true
	}
	redis := map[string]bool{}
	for _, t := range d.RedisTaskTypes {
		redis[t] = true
	}
	for _, t := range d.RedisTaskTypes {
		if !lite[t] {
			add("worker parity mismatch: %s missing from lite", t)
		}
	}
	for _, t := range d.LiteTaskTypes {
		if !redis[t] {
			add("worker parity mismatch: %s missing from redis", t)
		}
	}
}

// checkTargetPrefixes requires every manifest asset target to sit under its
// owner's target prefix (a package may alternatively pin target == source
// while awaiting its move).
func checkTargetPrefixes(m *Manifest, add func(string, ...any)) {
	for _, a := range m.Assets {
		prefixes := m.OwnerPrefixes(a.Owner)
		ok := a.Target == a.Source
		for _, p := range prefixes {
			if a.Target == p || strings.HasPrefix(a.Target, p+"/") {
				ok = true
				break
			}
		}
		if !ok {
			add("invalid target: asset %s target %s outside owner %s prefixes", a.ID, a.Target, a.Owner)
		}
	}
}

// checkDependencies enforces the module dependency rules over the
// discovered import edges:
//
//   - a module may import another module only through that module's root
//     public package or its explicit contracts package;
//   - /transport/ and /adapters/ subpackages are never importable from
//     another module;
//   - a domain package may not import any other module;
//   - active temporary exceptions suppress exactly one from-prefix ->
//     to-prefix dependency.
func checkDependencies(m *Manifest, d *Discovery, add func(string, ...any)) {
	prefixOf := map[string]string{}
	for _, mod := range m.Modules {
		prefixOf[mod.ID] = mod.TargetPrefix
	}
	moduleOf := func(dir string) string {
		best := ""
		bestLen := -1
		for id, prefix := range prefixOf {
			if dir == prefix || strings.HasPrefix(dir, prefix+"/") {
				if len(prefix) > bestLen {
					best, bestLen = id, len(prefix)
				}
			}
		}
		return best
	}

	for _, e := range d.Imports {
		from, to := moduleOf(e.Package), moduleOf(e.Path)
		if from == "" || to == "" || from == to {
			continue
		}
		if matchesException(m, e) {
			continue
		}
		forbidden := false
		switch {
		case strings.Contains(e.Package, "/domain/"):
			// Domain may import no other module at all.
			forbidden = true
		case containsAnySegment(e.Path, forbiddenTargetSegments):
			forbidden = true
		default:
			root := prefixOf[to]
			publicRoot := e.Path == root || e.Path == root+contractsSuffix
			forbidden = !publicRoot
		}
		if forbidden {
			add("forbidden import: %s -> %s", e.Package, e.Path)
		}
	}
}

// checkExpiredExceptions reports temporary exceptions whose removal wave
// has passed; they must be deleted together with the dependency.
func checkExpiredExceptions(m *Manifest, add func(string, ...any)) {
	for _, ex := range m.TemporaryExceptions {
		if m.CurrentWave > ex.RemoveInWave {
			add("expired exception: %s -> %s remove_in_wave=%d current_wave=%d",
				ex.From, ex.To, ex.RemoveInWave, m.CurrentWave)
		}
	}
}

// matchesException reports whether an import edge is covered by an active
// temporary exception. Expired exceptions are also matched here so the
// dependency is not double-reported; their expiry is signalled separately.
func matchesException(m *Manifest, e ImportEdge) bool {
	for _, ex := range m.TemporaryExceptions {
		if prefixMatches(e.Package, ex.From) && prefixMatches(e.Path, ex.To) {
			return true
		}
	}
	return false
}

func prefixMatches(dir, prefix string) bool {
	return dir == prefix || strings.HasPrefix(dir, prefix+"/")
}

func containsAnySegment(path string, segments []string) bool {
	for _, seg := range segments {
		if strings.Contains(path, seg) {
			return true
		}
	}
	return false
}

func indexAssets(m *Manifest, kind string, key func(Asset) string) map[string]bool {
	out := map[string]bool{}
	for _, a := range m.Assets {
		if a.Kind == kind {
			out[key(a)] = true
		}
	}
	return out
}

func collect[T any](in []T, key func(T) string) []string {
	out := make([]string, 0, len(in))
	for _, v := range in {
		out = append(out, key(v))
	}
	return out
}

// duplicateIDs returns the ids appearing more than once, sorted and
// deduplicated themselves.
func duplicateIDs(ids []string) []string {
	counts := map[string]int{}
	for _, id := range ids {
		counts[id]++
	}
	var dupes []string
	for id, n := range counts {
		if n > 1 {
			dupes = append(dupes, id)
		}
	}
	sort.Strings(dupes)
	return dupes
}
