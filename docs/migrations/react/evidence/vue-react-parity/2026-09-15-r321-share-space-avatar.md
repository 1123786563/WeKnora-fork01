# R321 share dialog organization avatar parity

Vue `SpaceAvatar.vue` renders shared-space avatars with deterministic gradient selection, optional `emoji:` avatars, a 22px small variant, square corner radius, and no small-variant shadow. The share dialog previously used a 32px circular initials badge in its organization picker and shared-list rows.

React now has a Web project `SpaceAvatar` component porting those rules and uses it for the share dialog's selected organization, option rows, and shared rows. Data filtering, role labels, permission selection, confirmation, and mutation behavior are unchanged.

Verification:

- Share dialog focused tests: 13/13 passed.
- Web typecheck: passed.
- Web suite: 911/911 passed, with 0 failed/cancelled/skipped.
- `git diff --check`: passed.

This is source and jsdom evidence. Authenticated post-edit paired screenshots and real backend share/unshare evidence remain open; N005 stays `implementing`.
