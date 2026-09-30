# T07/#147 isolated HTTP upload and review validation

Date: 2026-09-24. Code checkpoint: backend integration `5d8c57fea` plus architectureguard/docs-only commits through `f69656ae2` on `codex/issue-140-integration`. A fresh disposable SQLite Lite API ran on `127.0.0.1:57344` with local file storage and memory stream. Two disposable registered users had different personal Tenant IDs. The document-reader HTTP protocol pointed to an isolated loopback test double on `127.0.0.1:57345` that decoded the submitted UTF-8 text into `markdown_content`; this tests the API/storage/parser boundary but **not** the production DocReader extraction service. The Career deterministic extractor and durable source/receipt path were real code. No real credentials, token, or private response body are retained here.

The input was a local `.txt` resume with six explicitly labeled categories, two conflicting work-experience lines, no graduation date, and an unrelated identity-number line. The test double returned that exact text. An authenticated owner request supplied multipart `file`, stable `requestId`, and `expectedRevision=0`.

| HTTP observation | Result |
| --- | --- |
| Unauthenticated `GET /api/v1/career/open` | 401 |
| Owner `POST /api/v1/career/sources/upload` | 201, `ready` source, `intake_completed` receipt with 7 pending proposals |
| Source review metadata | `education.graduation_year` missing; `multiple_experience_claims_require_review` flagged; no public resource ref or extracted raw text |
| Exact multipart replay with same request ID/bytes/revision | 200, same source ID and identical receipt |
| Same request ID with changed file-name intent | 409 `idempotency_conflict` |
| Owner A token plus owner B Tenant header on `/career/sources` | 403, no facts/proposals/sources in error body |
| Profile immediately after upload | 200, zero confirmed facts and seven pending proposals |
| Confirm one proposal through `/career/act`, reopen profile | 200, one confirmed fact and six pending proposals |
| Invalid PDF upload after confirmation, then reopen profile | visible `failed` source; the earlier confirmed fact remains |

The Python standard-library probe and temporary sample stayed under `/tmp`, outside the repository. Its first registration attempt used a password longer than the runtime policy and was rejected with 400; the corrected probe registered successfully. An early assertion expected seven proposals to remain in the profile after confirmation, but the read model removes the resolved proposal, so the assertion was corrected to one confirmed fact plus six pending proposals. The completed probe then passed all assertions. This is an isolated integration check; Web browser upload and a real DocReader service are separate gates.
