# T01 #31 iOS live — round 2 evidence (2026-10-02, HEAD 54cdfa09c)

Verdict: **BLOCKED-proxy** — user-level CoreTunnel proxy (127.0.0.1:17890,
profile, no ExceptionsList) drops all simulator hostname traffic; the five app
defects from round 1 are fixed at HEAD and verified as far as the network layer.

| file | proves |
| --- | --- |
| r2-01-app-login-authorized-origin.png | Release app boots, login screen, authorized origin `https://192-168-3-33.nip.io:8443` prefilled (EXPO_PUBLIC at bundle time) — round-1 boot/PKCE panics gone |
| r2-02-after-sso-tap-unchanged.png | after tapping "Continue with single sign-on": no navigation, no error (beginOidc fetch silently dies in proxy; backend got 0 requests) |
| r2-03-safari-nipio-blocked.png | sim Safari cannot load the origin hostname (probe URL never reached backend) — sim-wide, not app-specific |
| r2-04-debug-devclient-noscripturl-redbox.png | debug/dev-client variant: bundle fetch via proxy dead → "No script URL provided (null)" — matches round-1 "zero bundling" symptom, now attributed to proxy, not Metro |
| t01-live-run-round2.txt | full sanitized run log: env checks, bundle-build proof (1425 modules), proxy attribution matrix, per-goal status |
| sha256sums-round2.txt | hashes |

Round-1 files (below) remain untouched. Android round passed end-to-end with the
same backend/Casdoor stack (evidence 67d591d5b) — the delta blocking iOS is the
macOS-side proxy the simulator inherits.
