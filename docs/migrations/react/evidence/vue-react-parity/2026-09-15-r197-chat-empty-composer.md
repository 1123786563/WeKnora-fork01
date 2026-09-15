# R197 chat empty composer

- Scope: `/platform/creatChat` empty new-chat state.
- Vue baseline: `creatChat.vue` centers the welcome title, an empty suggested-question slot, and the `Input-field` composer; the composer uses a 72px textarea minimum, 16px/24px typography, 16px top padding, and two 24px vertical gaps.
- Finding: live React rendered a 60px textarea and a 110px shell, leaving the empty composer about 13px shorter than Vue.
- Change: `ChatComposer` now uses the Vue textarea metrics and the empty chat cluster uses the Vue-equivalent 48px inter-section spacing with no extra bottom padding.
- Browser evidence: at 1355x720, React composer shell is `x=327.5,w=962,h=123`, textarea `h=72`; React welcome/composer y coordinates are `254.9/342.1`, matching Vue `255/342` within sub-pixel rounding.
- Validation: chat focused tests 74/74 and Web typecheck pass. Existing full Web regression remains 895/895 before this metric-only change; next cross-package gate should re-run it.
- Boundary: the Vue and React sessions exposed different model availability (`未配置` vs `mock-stream-model`), so model-selection business parity is not claimed by this visual result.
