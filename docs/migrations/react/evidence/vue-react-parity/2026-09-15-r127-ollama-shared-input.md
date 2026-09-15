# R127 Ollama shared input control

- Scope: `apps/web/src/settings/OllamaSettingsPanel.tsx`
- Change: replaced the native model-download `<input>` with the shared `@weknora/ui` `Input`, preserving localized label, placeholder, controlled value, and download behavior.
- Validation: `pnpm typecheck:web` passed; `pnpm test:web -- --runInBand` passed 895/895.
- Boundary: browser visual comparison and live Ollama success/download progress remain open; local Ollama was unavailable in the prior runtime probe.
