# OpenMeter contract cases

Each case is a JSON object with `id`, `operation_id`, `request`, `expected`,
`captures`, and `cleanup`. `expected` contains business assertions and is
checked recursively; an HTTP 2xx response alone never constitutes evidence.

Run a case with an explicit namespace. `POST`, `PUT`, `PATCH`, and `DELETE`
require `--allow-test-writes`, and writes are limited to that namespace. Capture
values use `${capture:name}` and capture selectors are JSON Pointers only.

Artifacts are written under `artifacts/saas-contract/<case-id>/` with secrets,
tokens, payment credentials, and payer details redacted. A runner success is
runner evidence only; it does not establish provider capability or production
business evidence.
