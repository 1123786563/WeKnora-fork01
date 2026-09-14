# Wiki graph runtime gating evidence

- Runtime: React Web `http://localhost:5181`, authenticated `parity-test@local.dev`, tenant-scoped KB `14ea2229-67bf-4f27-b759-a08b0dc23563`.
- Route: `/platform/knowledge-bases/14ea2229-67bf-4f27-b759-a08b0dc23563?tab=graph`.
- Observed DOM: heading `知识图谱`, localized explanatory copy, `适应屏幕` and `刷新` controls, and alert `error code: 1000, error message: Wiki feature is not enabled for this knowledge base` with localized `重试` action.
- This proves the real backend disabled-feature path is surfaced instead of presenting a false empty graph. It does not prove a graph-success payload or native-device graph rendering.
