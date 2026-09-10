# T24 local UI rollback rehearsal — 2026-09-11

## Scope

- Worktree: `codex/react-multiclient` at `a62ad2d`.
- Runtime: local `weknora-ui:t24-nginx-fix` image, with an isolated static
  artifact mount. `APP_HOST=127.0.0.1` and an unused backend port were supplied
  so Nginx could start without a backend process.
- No Go backend, database, migration, or production data was started or
  modified.

## Rehearsal

1. Started the image with `dist/react-web/web` mounted as the Nginx document
   root. `GET /` returned `200` and the body contained the React fingerprint
   `WeKnora React`; `BUILD_INFO.json` reported `renderer: react`.
2. Stopped that container without touching the mounted candidate or any DB.
3. Started the same image without the mount, exposing the old Vue artifact
   shipped inside the image. `GET /` returned `200`, contained the Vue
   `<div id="app"></div>` root and `/assets/main-*.js`, and contained no React
   fingerprint.

Observed output:

```text
rollback fingerprints passed
react=HTTP/1.1 200 OK
vue=HTTP/1.1 200 OK
rollback rehearsal: old Vue artifact restored after React candidate stop; no backend/DB process was started
```

## Boundary

This is a local artifact/container rollback rehearsal. It proves that the old
Vue static input remains recoverable without a database change. It does not
prove registry promotion, ingress control-plane rollback, a real authenticated
request matrix, or production traffic drain. T24 remains `review`.
