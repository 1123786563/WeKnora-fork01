# R013 Web Ollama settings runtime evidence

Date: 2026-09-15

- Authenticated React Chrome reached `http://localhost:5181/platform/settings?section=ollama`.
- The page exposed localized Chinese title and description (`Ollama 配置`, `管理本地 Ollama 服务，查看和下载模型`) and rendered the server-reported unavailable state.
- The backend reported `ollama service unavailable` because `localhost:11434` was not listening. Model inventory and download success paths could not be exercised.
- The available Vue tab did not have an authenticated session, so same-session Vue comparison remains unproven.

This is partial protected AX/runtime evidence only; it does not certify backend success behavior or pixel parity.
