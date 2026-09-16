# Shared controls across settings surfaces

Date: 2026-09-15  
Scope: system settings, preferences, cloud settings, environment variables, configuration and resource settings.

The settings migration now uses the shared UI control layer across the remaining form surfaces: `Input`, `NumberInput`, `Select`, `Switch`, and `Textarea`. Existing labels, validation, role gates, save/reset behavior, and Vue-shaped layout remain intact. This removes per-panel native-control styling drift.

Validation after commits `9b6119f9`, `e464614e`, `e5447b45`, `cc404fb6`, and `a77e1678`: Web typecheck passed; Web tests passed 895/895; production build passed; `git diff --check` passed. The known large-chunk advisory remains informational.
