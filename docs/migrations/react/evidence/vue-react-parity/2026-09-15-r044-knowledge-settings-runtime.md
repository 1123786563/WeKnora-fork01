# R044 Knowledge settings protected runtime

- Runtime: React Web at `http://localhost:5181/knowledgeBase/14ea2229-67bf-4f27-b759-a08b0dc23563/settings` with authenticated tenant `parity-test`.
- Observed: knowledge-base settings shell loads after the lazy route boundary; sidebar navigation, basic information, parser engine rows, chunking controls, index strategy, storage strategy, activity log, refresh and save controls are present.
- Capability truthfulness: unavailable parser engines expose disabled controls with backend-provided reasons (DocReader/MinerU/Cloud credentials/anydoc build state), while available `simple` options remain selectable.
- This runtime evidence confirms lazy loading did not regress protected KB settings navigation or capability gating.
- Limitation: mutation/save success and cross-platform native evidence remain open; no settings were changed during capture.
