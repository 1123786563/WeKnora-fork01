# 2026-09-13 five-locale dual-end sweep

## What
- .parity-tools/locale-sweep.cjs: for each of zh-CN/en-US/ja-JP/ko-KR/ru-RU
  (localStorage['locale'] seeded before app scripts on both ends, same
  convention both impls), logs in as parity-test@local.dev and captures
  kb-list, settings-general, creatChat at 1440x900.
- 30/30 captures in screenshots/locale-sweep-20260913/.

## Findings
- React ja-JP creatChat renders the full chat surface in Japanese
  (welcome, nav, time groups, composer placeholder, quick-answer chip) —
  the chat 5-locale slice works live; session titles stay in the data
  language (correct).
- REAL layout divergence found and fixed same-day (commit 42a67935):
  React pinned the creatChat welcome to the top and the composer to the
  bottom while Vue centers the cluster (.dialogue-wrap). The diff was
  invisible in the round-3 pixel ranking (sparse content) — text
  fingerprints + side-by-side inspection caught it. Fixed via the
  --empty conversation modifier; live re-shoot matches the Vue
  composition (heading y~347 vs Vue ~365; minor ~40px cluster offset
  remains, tracked as polish).
- Model chip gap re-confirmed on both locales (React shows the localized
  placeholder vs Vue showing the real mock-stream-model 200K) —
  existing R016 open item.
- Shell nav active color (React blue vs Vue brand green) — registered
  for the shell slice follow-up.

## Limits
- One account, one tenant, light theme, 3 routes; chat-session streaming
  states are not covered by this sweep.
