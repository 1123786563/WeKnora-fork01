# T07 real DocReader integration check

Date: 2026-09-24. Integration code checkpoint: `01f4143ef` (T07 Web integrated; no later Career code changes). The preinstalled `wechatopenai/weknora-docreader:latest` image ran as a disposable local gRPC container on loopback `127.0.0.1:57345`; a fresh SQLite Lite API ran on `127.0.0.1:57344`, with local storage and disposable accounts. No production service or personal resume was used.

The same synthetic eight-line resume was uploaded first as valid UTF-8 `.txt`. The API returned a visible `failed` source with `parse_failed`; the DocReader log explicitly reported `Unsupported file type: txt`. This is a verified compatibility failure because the Web chooser and UploadAdapter both advertise/allow `.txt`.

The content was then packaged into a valid simple `.docx` file using Python's standard-library ZIP/XML writer. Against the same real DocReader container, an authenticated multipart upload returned **201 ready**, `intake_completed`, seven pending proposals, a missing-graduation-year marker, and a multiple-experience review flag. Exact replay returned **200** with the same source/receipt. Reusing the request ID with a changed filename returned **409 idempotency_conflict**. Cross-Tenant source read returned **403** with no private payload. After one proposal confirmation, one fact and six pending proposals remained. A subsequent invalid PDF upload returned a failed source while that confirmed fact persisted.

This confirms the production DocReader path for the `.docx` sample. It also opens a concrete `.txt` format defect; see `docs/plans/2026-09-24-issue-140-t07-text-resume-compatibility.md`. No live PostgreSQL run occurred. The disposable API and container are stopped after validation.
