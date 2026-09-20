# P1 native Memory SQL evidence

Status: **incompatible for P1 governance; SQL behavior characterized.**

The probe is [memoryprobe](../../../../tools/trpc-native-p1/memoryprobe/). It
uses the exact standalone module requirements `trpc-agent-go@v1.11.0`,
`memory/sqlite@v1.11.0`, and `memory/postgres@v1.11.0`, plus the exact
`storage/postgres@v1.11.0` selected to avoid the backend module's development
pseudo-version. No root module, application code, migration, or SDK cache was
modified.

| Capability | SQLite | PostgreSQL | Classification |
| --- | --- | --- | --- |
| AppName/UserID isolation, reopen, and clear | pass | pass | PASS for tested SQL behavior |
| Same content add, metadata, update, delete, clear | pass | pass | PASS for tested SQL behavior |
| 20 concurrent same-content adds | pass | pass | PASS for tested SQL behavior |
| `Close` owns supplied SQLite `*sql.DB` | pass | n/a: backend creates its own client | PASS for documented ownership |
| stale extraction after delete or clear cannot resurrect content | raw service resurrects | raw service resurrects | **incompatible / extension-required** |

Commands run from `tools/trpc-native-p1/memoryprobe`:

```sh
GOWORK=off GOPROXY=https://proxy.golang.org,direct go mod tidy
GOWORK=off go list -m all
GOWORK=off GOPROXY=https://proxy.golang.org,direct go list -m -json trpc.group/trpc-go/trpc-agent-go trpc.group/trpc-go/trpc-agent-go/memory/sqlite trpc.group/trpc-go/trpc-agent-go/memory/postgres trpc.group/trpc-go/trpc-agent-go/storage/postgres
GOWORK=off GOPROXY=https://proxy.golang.org,direct go mod download -json trpc.group/trpc-go/trpc-agent-go@v1.11.0 trpc.group/trpc-go/trpc-agent-go/memory/sqlite@v1.11.0 trpc.group/trpc-go/trpc-agent-go/memory/postgres@v1.11.0 trpc.group/trpc-go/trpc-agent-go/storage/postgres@v1.11.0
GOWORK=off GOPROXY=https://proxy.golang.org,direct go test -run TestMemory -count=1 -v ./...
P1_MEMORY_PG_DSN='postgres://postgres@127.0.0.1:32768/postgres?sslmode=disable' GOWORK=off GOPROXY=https://proxy.golang.org,direct go test -run TestMemory -count=1 -v ./...
P1_MEMORY_PG_DSN='postgres://postgres@127.0.0.1:32768/postgres?sslmode=disable' GOWORK=off GOPROXY=https://proxy.golang.org,direct go test -race -count=20 -timeout=5m ./...
P1_MEMORY_PG_DSN='postgres://postgres@127.0.0.1:32768/postgres?sslmode=disable' P1_REQUIRE_CONTRACT=1 GOWORK=off GOPROXY=https://proxy.golang.org,direct go test -run TestMemoryContractRejectsStaleExtractionAfterClear -count=1 -v ./...
```

The exact module download sums were root
`h1:LwMxQwT2l6hqWUVARfVA/ef2tq8gJCzbImSHurpaPIo=`, SQLite
`h1:Xb5/BLdIhS0FPEdqDdtq5Mm3I3wQaytFYiGq+F24ln8=`, PostgreSQL
`h1:l5osN13fRX8oj3luKydVbxd/s+VKXwDd5RN1DkT/w2g=`, and required storage
`h1:t7sovgCKIVbxNETujKGvJ4hWZlQY+hoFmE2dNeT9piY=`. The non-contract commands
exited `0`; the final race run took 12.016s. PostgreSQL used a fresh `p1m_*`
schema per subtest and cleanup issued `DROP SCHEMA ... CASCADE`.
The normal suite skips `TestMemoryContractRejectsStaleExtractionAfterClear`;
that skip prevents a characterization result from being misreported as a
governance pass. The final `P1_REQUIRE_CONTRACT=1` command exited **1**: both
the SQLite and PostgreSQL subtests expected a stale post-clear write to fail,
but `AddMemory` returned nil. This intentional red result records that raw
`AddMemory` has no generation/tombstone/CAS parameter and old extraction output
can reappear.

Environment: `go version go1.26.3 darwin/arm64`; the real PostgreSQL target was
Docker container `weknora-trpc-p1-probe`, image `postgres:15.2-alpine`, with
`127.0.0.1:32768 -> 5432/tcp`. The client shell had no `psql`; probe setup and
cleanup used `database/sql` with `pgx` and completed successfully.

`GOWORK=off go list -m all` exited `0`. Its complete module graph was:

```text
github.com/Tencent/WeKnora/tools/trpc-native-p1/memoryprobe
github.com/DATA-DOG/go-sqlmock v1.5.2
github.com/bmatcuk/doublestar/v4 v4.9.1
github.com/bufbuild/protocompile v0.14.1
github.com/cenkalti/backoff/v4 v4.3.0
github.com/clbanning/mxj v1.8.4
github.com/creack/pty v1.1.24
github.com/davecgh/go-spew v1.1.1
github.com/decred/dcrd/dcrec/secp256k1/v4 v4.4.0
github.com/getkin/kin-openapi v0.124.0
github.com/go-ego/gse v1.0.0
github.com/go-logr/logr v1.4.3
github.com/go-logr/stdr v1.2.2
github.com/go-openapi/jsonpointer v0.20.2
github.com/go-openapi/swag v0.22.8
github.com/goccy/go-json v0.10.3
github.com/golang-jwt/jwt/v5 v5.2.3
github.com/golang/glog v1.2.4
github.com/golang/protobuf v1.5.3
github.com/gomutex/godocx v0.1.5
github.com/gonfva/docxlib v0.0.0-20210517191039-d8f39cecf1ad
github.com/google/go-cmp v0.6.0
github.com/google/go-querystring v1.0.0
github.com/google/uuid v1.6.0
github.com/grpc-ecosystem/grpc-gateway/v2 v2.22.0
github.com/hashicorp/errwrap v1.1.0
github.com/hashicorp/go-multierror v1.1.1
github.com/invopop/yaml v0.2.0
github.com/jackc/pgpassfile v1.0.0
github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761
github.com/jackc/pgx/v5 v5.7.2
github.com/jackc/puddle/v2 v2.2.2
github.com/josharian/intern v1.0.0
github.com/kr/pretty v0.3.0
github.com/kr/text v0.2.0
github.com/lestrrat-go/blackmagic v1.0.2
github.com/lestrrat-go/httpcc v1.0.1
github.com/lestrrat-go/httprc v1.0.6
github.com/lestrrat-go/iter v1.0.2
github.com/lestrrat-go/jwx/v2 v2.1.4
github.com/lestrrat-go/option v1.0.1
github.com/mailru/easyjson v0.9.0
github.com/mattn/go-sqlite3 v1.14.32
github.com/mitchellh/mapstructure v1.5.0
github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826
github.com/mozillazg/go-httpheader v0.2.1
github.com/openai/openai-go v1.12.0
github.com/panjf2000/ants/v2 v2.10.0
github.com/perimeterx/marshmallow v1.1.5
github.com/pmezard/go-difflib v1.0.0
github.com/rogpeppe/go-internal v1.12.0
github.com/santhosh-tekuri/jsonschema/v6 v6.0.2
github.com/segmentio/asm v1.2.0
github.com/stretchr/objx v0.5.2
github.com/stretchr/testify v1.11.1
github.com/tencentyun/cos-go-sdk-v5 v0.7.69
github.com/tidwall/gjson v1.14.4
github.com/tidwall/match v1.1.1
github.com/tidwall/pretty v1.2.1
github.com/tidwall/sjson v1.2.5
github.com/vcaesar/cedar v0.20.2
github.com/vcaesar/tt v0.20.1
github.com/yosida95/uritemplate/v3 v3.0.2
github.com/yuin/goldmark v1.4.13
go.opentelemetry.io/otel v1.29.0
go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc v1.29.0
go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp v1.29.0
go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.29.0
go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.29.0
go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.29.0
go.opentelemetry.io/otel/metric v1.29.0
go.opentelemetry.io/otel/sdk v1.29.0
go.opentelemetry.io/otel/sdk/metric v1.29.0
go.opentelemetry.io/otel/trace v1.29.0
go.opentelemetry.io/proto/otlp v1.3.1
go.uber.org/goleak v1.3.0
go.uber.org/multierr v1.10.0
go.uber.org/zap v1.27.0
golang.org/x/crypto v0.32.0
golang.org/x/mod v0.17.0
golang.org/x/net v0.34.0
golang.org/x/oauth2 v0.26.0
golang.org/x/sync v0.10.0
golang.org/x/sys v0.30.0
golang.org/x/term v0.28.0
golang.org/x/text v0.21.0
golang.org/x/tools v0.21.1-0.20240508182429-e35e4ccd0d2d
google.golang.org/appengine v1.6.8
google.golang.org/genproto/googleapis/api v0.0.0-20240822170219-fc7c04adadcd
google.golang.org/genproto/googleapis/rpc v0.0.0-20240822170219-fc7c04adadcd
google.golang.org/grpc v1.65.0
google.golang.org/protobuf v1.34.2
gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c
gopkg.in/yaml.v3 v3.0.1
trpc.group/trpc-go/trpc-a2a-go v0.2.6-0.20260721084546-18c8244d0acb
trpc.group/trpc-go/trpc-a2a-go/v2 v2.0.0-alpha.3.0.20260728070620-f4f0b6dd56ad
trpc.group/trpc-go/trpc-agent-go v1.11.0
trpc.group/trpc-go/trpc-agent-go/memory/postgres v1.11.0
trpc.group/trpc-go/trpc-agent-go/memory/sqlite v1.11.0
trpc.group/trpc-go/trpc-agent-go/storage/postgres v1.11.0
trpc.group/trpc-go/trpc-mcp-go v0.0.10
```

The native `EnqueueAutoMemoryJob` only enqueues into an in-process worker when
an extractor is configured. Its eventual writes call the same `AddMemory`
method, and neither the enqueue nor write interface accepts an authorization
revision, setting generation, tombstone, or conditional expected value. A
product adapter must recheck current authorization and settings and make the
generation/tombstone comparison and write atomic at commit time.

Existing WeKnora memory functionality is broader than the SDK metadata. Its
`MemoryItem` retains topic, normalized contradiction key, importance, origin,
status, source session/message provenance, validity/expiry, replacement and
usage fields. `MemorySettings` contains effective workspace/user enablement,
write mode, item count, and capacity. The separate `MemoryConfig` contains
extraction scheduling/instructions and vector/retrieval settings. The existing
repository also records deletion tombstones and extraction lease state. These
are required extension inputs for P1 and must not be discarded in an adapter;
most importantly, the persisted invalidation generation/tombstone and
authorization recheck must guard delayed extraction commits.
