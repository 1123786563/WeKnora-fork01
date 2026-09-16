# N031 mobile connector credential fields

## Scope

The native data-source editor now renders connector-specific credential inputs
for Feishu/Lark and Drive (`app_id`, `app_secret`, optional `base_url`), Notion
(`api_key`), Yuque (`api_token`, optional `base_url`), Tencent IMA
(`client_id`, `api_key`, optional `base_url`), and GitLab (`base_url`,
`access_token`). Unknown connector types retain the existing key/value fallback;
RSS retains its dedicated feed/header controls.

Known connector fields are write-only in the editor: edit mode starts with empty
credential values, and changing connector type clears any unsaved values. New
sources fail closed on missing required fields. Existing sources without newly
entered credentials continue to use the server-side validation endpoint by ID;
entered values use the non-persisting credential validation endpoint. Save uses
the existing update/create and credentials subresource transport.

## Contract evidence

- `packages/api-client/src/datasource.ts` exposes the existing `validate`,
  `validateCredentials`, `putCredentials`, create, and update transport.
- `internal/datasource/connector/feishu/core/types.go` and
  `internal/datasource/connector/feishu/drive` use `app_id`, `app_secret`, and
  `base_url`.
- `internal/datasource/connector/notion/types.go` requires `api_key`.
- `internal/datasource/connector/yuque/types.go` requires `api_token` and
  supports `base_url`.
- `internal/datasource/connector/ima/types.go` requires `client_id` and
  `api_key` and supports `base_url`.
- `internal/datasource/connector/gitlab/connector.go` reads `base_url` and
  `access_token`.
- `frontend/src/api/datasource/index.ts` is the Vue reference for the same
  validation and credentials subresource routes. The Vue baseline was not
  modified.

## Verification

- Focused native-host screen and policy tests: 20/20 passed.
- Full mobile test suite: 95/95 passed.
- Mobile TypeScript check: passed.
- `git diff --check`: passed.

## Acceptance status

N031 remains `implementing`. This slice has no Vue-vs-React browser screenshot,
real backend E2E, Wails, iOS, or Android native-runtime evidence yet. Exact
localized copy and full page visual parity are also still open.
