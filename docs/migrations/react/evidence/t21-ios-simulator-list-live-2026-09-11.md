# T21 iOS Simulator knowledge list evidence — 2026-09-11

## Scope and environment

- Same isolated iPhone 17 Pro iOS 26.5 simulator, native host
  `com.weknora.mobile`, worktree Metro, and Lite backend described in
  `t20-ios-simulator-auth-live-2026-09-11.md`.
- Authenticated KB: `iOS Live KB` (`115bd1bc-072b-42c7-802a-da15ef845fbb`).
- No production data or provider-backed document was used.

## Live API and native route

The authenticated backend responses were collected directly from the isolated
server:

- `GET /api/v1/knowledge-bases/{id}/knowledge?page=1&page_size=20&folder_path=&folder_recursive=true`
  returned `200`, `success: true`, `data: []`, `total: 0`, `page: 1`, and
  `page_size: 20`.
- `GET /api/v1/knowledge-bases/{id}/knowledge/folders` returned `200` with
  an empty folder tree.
- `GET /api/v1/knowledge-bases/{id}/tags?page=1&page_size=100` returned
  `200` with the backend's actual nested page shape:
  `data: { total: 0, page: 1, page_size: 100, data: [] }`.

Opening the real `iOS Live KB` row on the simulator navigated to the native
`knowledge/[id]` route. The Files screen showed Wiki, FAQ, Upload, search and
folder controls, then rendered `0 files` and `No files match these filters.`
without the previous `data: expected an array` error.

For a non-empty read fixture, the isolated backend accepted
`POST /api/v1/knowledge-bases/{id}/knowledge/manual` with a draft manual
document (`Native list fixture`, `200`). After a Metro reload, the native
screen rendered `1 files` and the row `Native list fixture.md, draft`. Tapping
the row opened the native `knowledge/document/[id]` route and rendered the
server-owned title, parse status, document id, type (`manual`), size, and the
`Download and share` action.

## Contract correction

The nested tag response first failed the new contract test with
`ContractError: data: expected an array`. The parser was changed only after
that RED result to accept both the existing flat fixture form and the actual
paginated backend form, while validating `total`, `page`, `page_size`, tag
identity, and tag field types. The GREEN test passed 5/5.

This is authenticated list/detail and empty-state evidence, not native picker
upload-processing, search-result, preview/download/share execution, 403/413,
cancellation, or large-file acceptance. Those gates remain `review`.
