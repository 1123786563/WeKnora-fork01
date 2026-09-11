# T25 Vue retirement preflight — 2026-09-11

## Scope

This is a read-only retirement preflight. It does not delete `frontend/`, alter
the production Lite input, or remove any fallback artifact. T25 requires T24
to be accepted and the complete role/deployment/native matrix to be accepted
first.

## Current legacy product inputs

- `frontend/src/` still contains the Vue product surface, Pinia stores,
  `vue-i18n` resources, the legacy route table, Embed bridge entry, and the
  existing chat/document/settings implementations.
- `frontend/package.json` still owns the Vue 3, Pinia, TDesign, Vue Router and
  Vue I18n build dependencies, as well as the existing Markdown, preview,
  terminal and spreadsheet packages. These dependencies cannot be removed
  while the legacy artifact remains a supported fallback input.
- `frontend/src/wailsjs/` remains a legacy generated runtime path, but
  `cmd/desktop/wails.json` already targets `apps/desktop`; this generated path
  must be rechecked after an installed Wails upgrade rehearsal before removal.

## Active build and release references

The following current files still consume or validate the legacy input:

- `Makefile`: `docker-build-frontend`, `build-lite`, `run-lite` and packaging
  fallback branches still build/copy `frontend/dist`.
- `frontend/Dockerfile`, `frontend/nginx.conf`, `.github/workflows/frontend.yml`,
  `.github/workflows/docker-image.yml` and the legacy part of
  `.github/workflows/release-lite.yml` still define the shipped Vue image
  path.
- `scripts/build_frontend_dist.sh`, `scripts/package-lite.sh`,
  `scripts/package-mac-app.sh`, `scripts/build_images.sh` and
  `scripts/verify_frontend_pr.sh` still have legacy build or recovery behavior.
- `scripts/check-react-boundaries.mjs` intentionally requires `frontend/` and
  checks its Docker/Nginx compatibility. Removing the directory before the
  release cutover would invalidate the current boundary gate and rollback
  rehearsal.
- `cmd/desktop/wails.json` is React-oriented, but the Lite/static fallback and
  installed-package evidence still depend on a retained old artifact rather
  than deletion of the source directory.

## Decision

T25 remains `pending`. The scan proves that deletion is not currently a
behavior-preserving cleanup: it would remove the only source used by the
legacy release and rollback paths. Required next evidence before T25 starts:

1. T24 changes are accepted rather than only review-level evidence.
2. The React Web/Embed artifact is promoted and the old Vue artifact is stored
   independently by the release system.
3. Installed Wails upgrade/rollback, supported-client compatibility, browser
   role/tenant matrix, and iOS/Android runtime gates are accepted.
4. A fresh reference scan proves no product or test path still needs
   `frontend/src`, Vue/Pinia/TDesign runtime, or the legacy build scripts.

The existing `frontend/` tree and old release artifact are therefore retained
as the explicitly documented rollback object.
