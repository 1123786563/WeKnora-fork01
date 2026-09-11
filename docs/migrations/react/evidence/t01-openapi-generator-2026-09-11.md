# T01 OpenAPI Generator trial — 2026-09-11

## Toolchain

- CLI: `@openapitools/openapi-generator-cli@2.20.0`
- Selected generator: OpenAPI Generator `7.14.0`
- Generator: `typescript-fetch`
- Java: Temurin OpenJDK `17.0.19`
- Input: `docs/swagger.yaml`

## Results

Strict validation was run first:

```text
npx --yes @openapitools/openapi-generator-cli@2.20.0 generate \
  -i docs/swagger.yaml -g typescript-fetch -o <temporary-directory> \
  --additional-properties=supportsES6=true,useSingleRequestParameter=true
exit 1
```

The generator reported 9 errors and 1 warning. The errors are missing
`responses` and undeclared path parameters in
`/organizations/{id}/search-users` and the session artifact/artifact-download
routes.

The diagnostic skip-validation run completed:

```text
npx --yes @openapitools/openapi-generator-cli@2.20.0 generate \
  -i docs/swagger.yaml -g typescript-fetch -o <temporary-directory> \
  --skip-validate-spec \
  --additional-properties=supportsES6=true,useSingleRequestParameter=true
exit 0
```

It generated the TypeScript Fetch files, but emitted the known specification
warnings, auto-generated operation IDs for unnamed operations, and a duplicate
case-sensitive API file path. The generated client was not copied into the
workspace: the migration uses the reviewed shared client boundary so generated
code cannot bypass common authentication, error normalization, tenant scope, or
platform transport.

## Boundary

This proves the pinned generator/runtime can parse the document and identifies
the exact Swagger defects that prevent strict generation. It does not prove the
Swagger document is a complete runtime contract, and it does not replace the
required handler DTO, middleware/permission, and live endpoint review.
