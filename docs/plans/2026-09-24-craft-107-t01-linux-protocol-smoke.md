# T01 Linux/arm64 OpenCode protocol smoke

Date: 2026-09-24 (Asia/Shanghai)

## Scope and controls

Bounded disposable Docker smoke against `weknora-craft-runtime:1.18.4`, exact image ID `sha256:9678e13bd1c7dc9650aef26623ef8ab276d46565bbd6cb006b8e308229968f11`, on Linux/arm64. The container used temporary HOME/XDG directories and a temporary project bind mount; no provider credentials, model request, prompt, or external provider call was made. Two containers were stopped and auto-removed. No production source, lock, or main checkout file was changed.

## Verified facts

- `docker image inspect` returned `sha256:9678e13bd1c7dc9650aef26623ef8ab276d46565bbd6cb006b8e308229968f11 linux/arm64`.
- In-image `/usr/local/bin/opencode --version` returned `1.18.4`.
- In-image `/usr/local/bin/opencode` SHA-256 was `3557e87db8c7db70e8ebd42157df1246554120896b115c462b760ff248cf751e`. The lock's `9449af...` digest is explicitly the Darwin/arm64 local probe; `container_digest` remains null, so this Linux digest is evidence for follow-up pinning rather than a lock match.
- `POST /session?directory=/workspace/project` returned `200` and a session object with `id`, `projectID: "global"`, canonical `directory: "/workspace/project"`, `path`, `version: "1.18.4"`, timestamps, and token/cost fields.
- `GET /session?directory=/workspace/project` returned `200` with an array containing the created session.
- `GET /api/session?directory=/workspace/project&project=global&limit=1` returned `200` with `data` and opaque `cursor.previous`/`cursor.next`; the row contained matching ID, `projectID`, `location.directory`, and `subpath`.
- `GET /api/session/<id>?directory=/workspace/project&project=global` returned `200` with `data` and matching ID/project/location.
- `GET /api/session/<id>?directory=/workspace/other&project=global` also returned `200` with the same record and `/workspace/project` location. The server did not enforce the mismatched directory query on by-ID lookup.
- `GET /api/session?directory=/workspace/project&project=global&limit=1&cursor=bogus` returned `400` with `{"_tag":"InvalidCursorError","message":"Invalid cursor"}`.

## Exact raw command/response excerpts

```text
docker image inspect weknora-craft-runtime:1.18.4 --format '{{.Id}} {{.Os}}/{{.Architecture}}'
sha256:9678e13bd1c7dc9650aef26623ef8ab276d46565bbd6cb006b8e308229968f11 linux/arm64

docker run --rm --entrypoint /usr/local/bin/opencode weknora-craft-runtime:1.18.4 --version
1.18.4

POST /session?directory=/workspace/project  -> HTTP/1.1 200 OK
{"id":"ses_<redacted>","projectID":"global","directory":"/workspace/project","path":"workspace/project","version":"1.18.4",...}

GET /api/session/<id>?directory=/workspace/project&project=global -> HTTP/1.1 200 OK
{"data":{"id":"ses_<redacted>","projectID":"global","location":{"directory":"/workspace/project"},"subpath":"workspace/project",...}}

GET /api/session/<id>?directory=/workspace/other&project=global -> HTTP/1.1 200 OK
{"data":{"id":"ses_<redacted>","projectID":"global","location":{"directory":"/workspace/project"},"subpath":"workspace/project",...}}

GET /api/session?directory=/workspace/project&project=global&limit=1&cursor=bogus -> HTTP/1.1 400 Bad Request
{"_tag":"InvalidCursorError","message":"Invalid cursor"}
```

## Inference and recommendation

The Linux/arm64 image serves the required create and v2 inventory protocol at the pinned image ID. The v2 list cursor is opaque and malformed cursors are rejected. By-ID lookup ignores a mismatched directory query in this build, so callers must independently validate returned `projectID` and `location.directory` against the intended scope. The Linux binary digest must be recorded in the Linux lock/image verification path before release; it cannot be compared to the Darwin digest as if they were the same artifact.

## Source pointers

- `docker/craft/opencode.lock.json` — protocol contract, Darwin digest, and pending Linux container digest.
- `docs/plans/2026-09-23-craft-107-t01-host-protocol-smoke.md` — prior host smoke and scope-validation inference.
- `docs/plans/2026-09-23-craft-107-runview-provider-fix1-review.md` and `docs/plans/2026-09-24-craft-107-runview-r2-executor-binding-review.md` — provider/R2 review boundaries.
- `internal/modules/agentruntime/agent/opencode/client.go` and `inventory.go` — client and v2 inventory behavior.
