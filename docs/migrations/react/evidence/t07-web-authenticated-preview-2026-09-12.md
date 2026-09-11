# T07 Web authenticated preview follow-up (2026-09-12)

- Commit `369ddb8` adds an optional binary response transport to the shared
  client. Document preview and download now use authenticated requests rather
  than public `<a>` URLs, so bearer credentials are not placed in query
  strings.
- Completed text and Markdown documents render escaped text; images and PDFs
  render from short-lived Blob URLs; unsupported formats are explicitly
  download-only. Preview and download object URLs are revoked during cleanup.
- Deterministic verification passed: shared tests 182/182, Web tests 96/96,
  shared/Web typechecks, and `git diff --check`. Tests cover content headers,
  protected bytes, encoded IDs, safe Markdown rendering, and Blob cleanup
  behavior.
- This is transport/UI evidence only. A live protected binary response,
  browser download bytes/filename, and the complete format renderer matrix
  remain unverified; T07 stays `review`.
