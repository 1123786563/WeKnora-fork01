# T01 host protocol smoke (Darwin/arm64, OpenCode 1.18.4)

Date: 2026-09-23 (Asia/Shanghai)

## Scope

Bounded host-only smoke against the pinned local binary. No model call was made. The server used a temporary `HOME`, XDG config/data/state/cache directories, a temporary project directory, and one dynamically selected localhost port. The process and temporary files were terminated/removed by the probe. No production or test source was changed.

## Verified facts

- Lock source: `docker/craft/opencode.lock.json`.
- Binary: `/Users/wuyongjun/.opencode/bin/opencode`.
- `opencode --version` returned `1.18.4`.
- SHA-256 returned `9449af91f517eacc2b0742fa93ae0da64fa6e5db7b714e30c62edea2a8de3f98`, matching both `binary_sha256` and `local_probe.binary_sha256` in the lock.
- Server command: `opencode serve --pure --hostname 127.0.0.1 --port <free-port> --print-logs --log-level ERROR`, launched from the temporary project directory.
- Legacy `POST /session` returned `200` and a session object with `id`, `projectID`, `directory`, `path`, `version: "1.18.4"`, and timestamps. The response was sanitized; the actual ID and temporary path are omitted here.
- `GET /session?directory=<canonical-temp-project>` returned `200` with a JSON array containing the created session.
- `GET /api/session?directory=<canonical-temp-project>&project=global&limit=1` returned `200` with envelope `{"data":[...],"cursor":{"previous":"<token>","next":"<token>"}}`. The record contained `id`, `projectID`, `location.directory`, and `subpath`.
- `GET /api/session/<id>?directory=<canonical-temp-project>&project=global` returned `200` with envelope `{"data":{...}}` and matching ID/project/location directory.
- `GET /api/session?directory=<canonical-temp-project>&project=global&limit=1&cursor=bogus` returned `400` with `{"_tag":"InvalidCursorError","message":"Invalid cursor"}`.
- A first empty `GET /api/session` returned `200` with `{"data":[],"cursor":{"previous":null,"next":null}}`.

## Inference and limits

- The v2 inventory endpoint exposes an opaque cursor envelope and rejects malformed cursors; callers should preserve the cursor as an opaque value and treat a non-null continuation cursor as pagination work.
- In this host build, by-ID lookup returned the record even when supplied a deliberately mismatched `directory` query. Therefore the caller must enforce its own post-response directory/project scope checks; query parameters alone did not prove by-ID scoping.
- The legacy create response selected `projectID: "global"` while recording the canonical server-selected directory. This smoke does not establish Linux image behavior, container mount isolation, or cross-process recovery guarantees.
- No prompt/model request, event stream, abort, or provider credential was exercised.

## Recommendation

Use the v2 list endpoint for recovery inventory with explicit `directory` and `project` scope, follow opaque cursors until `next` is null, and independently reject any by-ID record whose returned `projectID` or `location.directory` does not match the intended scope. Keep runtime version verification on `opencode --version` plus the binary digest; do not use `/doc`'s static API info version.

## Source pointers

- `docker/craft/opencode.lock.json` (digest, version, pinned protocol, host probe note).
- `internal/modules/agentruntime/agent/opencode/client.go` (directory header/binding and legacy protocol client).
- `internal/modules/agentruntime/agent/opencode/inventory.go` (v2 `/api/session` list/by-ID, cursor and scope validation expectations).
