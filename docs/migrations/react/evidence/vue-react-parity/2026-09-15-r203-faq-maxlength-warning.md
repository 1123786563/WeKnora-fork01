# R203 FAQ maxLength warning cleanup (2026-09-15)

The FAQ standard-question field previously passed lowercase `maxlength` through a type cast. React emitted `Invalid DOM property maxlength` during the Web suite. It now uses the native React `maxLength={200}` prop, preserving the Vue 200-character cap without the warning.

Verification:

- FAQ suite: 48/48
- Web full regression: 895/895
- `git diff --check`: pass
