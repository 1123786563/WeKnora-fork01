# Reproducible evidence

See [the experiment README](../../../../../../semantic/experiments/README.md) for the
hash-checked replay command.

- [runtime-312.json](runtime-312.json) is the current CPython 3.12 actual
  runtime result: rule cases, in-memory ContextGraph calls, and a mock provider
  factory seam check.
- [capability-evidence.json](capability-evidence.json) is CPython 3.12 public
  import plus source-only API inspection; it does not execute those algorithms.
- [import-396.json](import-396.json) is the retained CPython 3.9.6 public-import
  compatibility failure.

Read [REPORT.md](REPORT.md) for scope and limitations.
