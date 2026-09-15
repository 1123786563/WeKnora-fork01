# 2026-09-15 R330 — guide transition recheck

- Fresh authenticated Vue and React contexts used the same local account, 1440×900 viewport, and zh-CN locale.
- Both initially rendered the seven-step global guide. After clicking the visible `跳过引导` action, both rendered the one-step empty-knowledge-base contextual guide (`1 / 1`, `创建第一个知识库`).
- The earlier apparent mismatch was a transition timing/selection artifact, not a remaining implementation difference.

