# Semantica 0.6.8 bounded capability probe

The frozen public wheel is `semantica-0.6.8-py3-none-any.whl`, SHA-256
`0af4d9dd9b01503e0d72c0ae6b0703364d01dd1443e42bc7e9a6f415835917d7`.
Declared provenance, captured from the GitHub tag API rather than verified by a
runtime probe, is tag `v0.6.8` /
`29f3c3230cb72e6b84201a0f5f4e310929fefb47`, resolving to commit
`f73f599a22c320676a45f247247038ca1fcf40f0`.

## Evidence separation

- [import-396.json](import-396.json) is the CPython 3.9.6 public-import
  diagnostic. It exited 1 with `TypeError: unsupported operand type(s) for |:
  'type' and 'EnumMeta'`. This is a compatibility finding for that interpreter,
  not an overall Semantica capability failure.
- [capability-evidence.json](capability-evidence.json) is the CPython 3.12
  public-import and exact-wheel source-only inspection result. Its public import
  succeeded. It deliberately does not run the rule, Context, or provider
  algorithms; its source-only findings are not runtime acceptance.
- [runtime-312.json](runtime-312.json) is the CPython 3.12 actual runtime
  probe. It records the positive and missing-premise Reasoner cases, in-memory
  ContextGraph node/edge calls, and a controlled factory-patch GraphReasoner
  seam check. The factory patch is classified `mock`, not a real provider or Go
  gateway validation.

The 3.12 environment used bounded dependencies only: numpy, scipy, networkx,
python-dateutil, PyYAML, rdflib, and pytest. It did not install torch,
transformers, model weights, provider SDKs, or connect to external services.

The runtime result shows `IndirectDependsOn(A, C)` with both source facts in
`premises`; removing `B DependsOn C` produces no result. ContextGraph accepts a
node and edge in memory. The controlled factory patch observes entity and
relationship properties in the prompt and returns a controlled string. These
observations do not validate persistent storage, Go authorization prefiltering,
ACL/deletion blocking, budget controls, real providers, gRPC, performance, or
quality thresholds.

This bounded probe does not pass V01, V02, or V03 as a whole.
