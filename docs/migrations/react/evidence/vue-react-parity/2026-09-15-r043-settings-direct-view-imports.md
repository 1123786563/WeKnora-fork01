# R043 Settings direct view imports

- Scope: `SkillSettingsPanel` imports markdown rendering directly and `SandboxSettingsPanel` imports the settings role helper directly from their owning modules.
- Intent: keep lazy settings chunks independent from the `@weknora/views` barrel and make module ownership explicit.
- Verification: `pnpm run typecheck:web`, Web tests 891/891, and `pnpm run build:web` all passed.
- Limitation: this slice does not materially change the aggregate entry chunk; it reduces barrel coupling for future chunk-level splitting.
