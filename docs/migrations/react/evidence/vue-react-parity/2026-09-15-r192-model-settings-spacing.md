# R192 model settings spacing parity

- Conditions: Chrome, same authenticated seeded account, zh-CN, 1355x720 viewport, React `:5181` and Vue `:5173` model settings.
- Finding: React inserted a divider plus stacked margins between the panel heading and builtin-model notice, pushing the notice, tabs and cards about 45–50px below the Vue baseline.
- Change: removed the redundant heading divider/padding and duplicate notice top margin; the existing grid gap now controls the section rhythm.
- Browser evidence: refreshed React screenshot places the builtin notice, model tabs and first card at the same vertical rhythm as Vue while retaining all labels and controls.
- Validation: ModelSettings focused tests 23/23, Web typecheck and `git diff --check` pass.
- Boundary: model mutation/debug flows, other locales, responsive widths and native/Wails rendering remain open.
