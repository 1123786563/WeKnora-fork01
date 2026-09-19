# Native P1 identity probe

This standalone module is an executable specification for the P0 identity-key,
checkpoint namespace, and replay-cursor encodings. It imports the exact
`trpc.group/trpc-go/trpc-agent-go v1.11.0` root module and has no `replace`
directives.

Run the probe from this directory:

```sh
GOWORK=off go test -race -count=20 ./...
```

`SessionKey` requires a nonzero tenant plus opaque, nonblank valid UTF-8 owner
and session IDs. `MemoryKey` requires a nonzero tenant plus an opaque, nonblank
valid UTF-8 subject ID. Each function intentionally ignores the other key
domain's fields: a Session key does not require a memory subject, and a Memory
key does not require a session owner or ID. It preserves valid whitespace in an
ID before raw base64url encoding, so validation cannot normalize distinct
identifiers into the same SDK key. Checkpoint namespaces and cursors use raw
base64url without padding and decimal integers.

This module is not a `ScopeResolver`, does not authenticate or authorize a
caller, does not model revocation, and does not establish database isolation.
Decoding a cursor only verifies its syntax and expected run ID; a cursor grants
no permission.
