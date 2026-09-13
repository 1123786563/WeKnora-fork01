# N008 media preview parity slice

Date: 2026-09-14

## Vue baseline

`frontend/src/components/document-preview.vue` resolves audio and video files to native media controls. Audio uses a full-width bounded player; video uses `controls` and `playsinline`, with the preview blob managed alongside PDF/image resources.

## React implementation

The shared preview kind resolver now recognizes common audio (`mp3`, `wav`, `ogg`, `oga`, `m4a`, `aac`, `flac`) and video (`mp4`, `webm`, `ogv`, `mov`, `m4v`) extensions. The React preview renders native `<audio controls>` and `<video controls playsInline>` elements using the existing authenticated blob lifecycle. DOCX/PPTX/Excel/Mermaid remain separate renderer-dependent gaps.

The detail surface also now exposes a Vue-equivalent retry action after a preview request fails. Download failures remain in the download status channel and no longer replace an already loaded preview.

## Verification

- Domain preview tests: 2/2 passed.
- Web preview tests: 4/4 passed.
- Web typecheck: passed after the retry/error-channel change.
- `git diff --check` passed for this slice.

## Evidence boundary

This is source and focused-test evidence. It does not prove same-condition browser media playback, real-backend bytes, or desktop/mobile platform acceptance.
