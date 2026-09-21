// Command architectureguard enforces the backend module manifest
// (docs/architecture/backend-modules.yaml) against the real repository.
//
// It exits 1 and prints one violation per line to stderr when rules are
// broken, prints "backend architecture guard: ok" and exits 0 when the
// repository is compliant, and exits 2 on tool errors (unreadable
// manifest, unparseable Go source, ...).
//
// The loader (manifest.go), discovery (discovery.go), and rule checks
// (check.go) live in this package so the guard stays a single self-
// contained tool that runs with `go run ./tools/architectureguard`.
package main

import (
	"flag"
	"fmt"
	"os"
)

const (
	exitOK          = 0
	exitViolations  = 1
	exitToolFailure = 2
)

func main() {
	root := flag.String("root", ".", "repository root")
	manifestPath := flag.String("manifest", "docs/architecture/backend-modules.yaml", "path to the backend module manifest")
	wave := flag.Int("wave", 1, "current migration wave used for temporary-exception expiry")
	flag.Parse()

	m, err := LoadManifest(*manifestPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "backend architecture guard: load manifest: %v\n", err)
		os.Exit(exitToolFailure)
	}
	m.CurrentWave = *wave

	discovery, err := Discover(*root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "backend architecture guard: discover: %v\n", err)
		os.Exit(exitToolFailure)
	}

	violations := Check(m, discovery)
	if len(violations) > 0 {
		for _, v := range violations {
			fmt.Fprintln(os.Stderr, v.Message)
		}
		os.Exit(exitViolations)
	}
	fmt.Println("backend architecture guard: ok")
	os.Exit(exitOK)
}
