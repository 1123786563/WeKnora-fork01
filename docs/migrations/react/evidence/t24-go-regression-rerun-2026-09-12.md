# T24 Go 全量回归复跑 — 2026-09-12

Command:

```text
GOWORK=off go test ./...
```

Result: exit `0`. All packages either passed or reported `[no test files]`,
including `internal/application/service`, the Notion connector, chat models,
embedding models, router, handlers, runtime and types.

The previous run had two categories of failures. The Python verifier tests were
using the host macOS `python3` (3.9), while the sandbox contract is Python 3.11;
the tests now select Python 3.11+ and therefore exercise `tomllib` and a clean
host distribution view. The Notion/Azure/OpenAI tests only construct requests
and already replace the network transport; they now explicitly whitelist their
example endpoints in the non-parallel test scope, so the result does not depend
on DNS resolving examples to the SSRF-reserved `198.18.0.0/15` range. The
production SSRF validator and connection-level guard were not relaxed.

The macOS linker emitted the existing non-fatal warning
`ignoring duplicate libraries: '-lc++'` for two test binaries. No service,
external provider, production database, registry or release was used.
