# Vue ↔ React parity evidence — mobile knowledge upload progress

- The shared client now forwards `ClientRequest.onProgress` to `sendMultipartFile` without changing the existing request shape when no callback is supplied.
- `uploadKnowledgeFiles` converts `{loaded,total}` to clamped progress events, preserving FIFO, cancellation, and completion semantics.
- `KnowledgeDocumentsScreen` passes the callback into the knowledge upload endpoint.
- Validation: upload queue 6/6, shared suite 465/465, web typecheck, mobile typecheck, and `git diff --check` passed.
- Runtime limitation: this does not prove a device upload against a live backend; authenticated iOS/Android knowledge interaction remains open.
