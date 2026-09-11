# T07 React Web upload live evidence — 2026-09-12

## Scope

This follow-up verifies the React Web document route against an isolated Lite
backend. It covers authenticated knowledge-base navigation, the file-upload
contract, document processing state, and the rendered list state. It does not
claim preview/download or search-reference acceptance.

## Environment

- Worktree: `codex/react-multiclient` at `e74850859bc4b1d89cbc661db2048f0c6dc31ac9`
- React Web: `http://127.0.0.1:5175/`
- Lite backend: `http://127.0.0.1:18082/`, SQLite data isolated under `/tmp/weknora-react-web-chat-20260912`
- Embedding stub: local OpenAI-compatible server at `http://127.0.0.1:19998/v1`
- Browser account: newly-created isolated account `react-upload-20260912@example.com`; no user data was used
- Fixture: `react-t07-upload.txt`, 132 bytes, SHA-256 `7835e738ff41496f02c24b752b8b4a3b40d9897e7f1d057b5ecaecda228ec281`

## Observed steps and evidence

1. The visible React Web sign-in form accepted the isolated account and
   navigated to `/platform/knowledge-bases`. The rendered scope contained the
   new user ID and tenant `3`; the authenticated list request returned a
   tenant-local empty state.
2. The React Web list created `React T07 Upload 20260912` and rendered its
   `Open documents` action. The documents route was reachable at
   `/knowledgeBase/83c30b46-3d4c-4b51-8bff-5ea5cb696638` and rendered File/URL/Manual
   source controls, folder root, status filter, tag filter, and the upload
   action.
3. The in-app browser exposed the file input and the upload button, but its
   native file chooser did not accept a path through the connected browser
   control surface. The UI therefore displayed the authoritative validation
   message `Choose a file before uploading.`. To keep the backend contract
   evidence reproducible, the same isolated fixture was submitted through the
   documented multipart endpoint rather than pretending the chooser completed.
4. A first upload to an intentionally unbound test KB returned HTTP 200 but
   remained `parse_status=processing`; Lite logged `model ID cannot be empty`.
   This exposed the required embedding binding rather than being counted as a
   successful processing result.
5. An isolated embedding model (`react-t07-embedding`, tenant `3`) was created
   against the local stub, and a second KB was created with that binding:
   `e20fd1ca-be02-4e85-978e-9ab614cd7c22`. Multipart
   `POST /api/v1/knowledge-bases/e20fd1ca-be02-4e85-978e-9ab614cd7c22/knowledge/file`
   returned HTTP 200 and document ID
   `571fb485-a2b3-4f88-8e0c-c45c16a1eec9`.
6. The authenticated document list returned the fixture as `txt`, with
   `parse_status=completed`, `summary_status=failed`,
   `pending_subtasks_count=0`, and no error message. The failed summary is
   expected for this fixture because no summary model was configured; it is not
   being reported as successful summarization.
7. After navigating the visible React Web page to the bound KB, the rendered
   list showed `react-t07-upload.txt` and `Root · txt Completed`. This confirms
   that the React route consumes the backend's authoritative parse state rather
   than inferring completion from upload acknowledgement. The local embedding
   stub recorded `POST /v1/embeddings` with HTTP 200.

## Result and remaining gates

Pass: authenticated React Web KB/document navigation, multipart backend
contract, embedding-backed document parsing, status polling/readback, and
rendered `Completed` state.

Still open: native chooser submission through the browser harness, actual
browser progress telemetry, protected binary preview/download headers and
filename behavior, search/reference rendering, and per-format renderer
acceptance. T07 remains `review`.
