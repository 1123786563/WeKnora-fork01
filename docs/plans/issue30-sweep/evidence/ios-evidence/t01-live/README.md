# T01 (#31) iOS live round — evidence bundle

Status: PARTIAL / BLOCKED (two blockers, see t01-live-run.txt §D-§E).

| Goal | Status | Evidence |
| --- | --- | --- |
| Authorized HTTPS deployment reachable + runtime integration gate exercised | partial (Node harness; real login 200 + capabilities 200 + gate classification over https://192-168-3-33.nip.io:8443; authorized surface NOT reached) | t01-live-run.txt §A-§B |
| App launches against authorized origin | yes (deployment-login renders with prefilled authorized HTTPS origin) | app-deployment-login-authorized-origin.png, app-deployment-login-lvhme-prefill.png |
| Real OIDC redirect login at Casdoor | partial (real authorize-page login form + real t01live authentication success rendered in system Safari; no code issuance — app-side begin blocked) | casdoor-login-form-username-only.png, casdoor-authentication-success.png |
| Post-login capability surface | not reached | — |

Blockers: (1) HEAD backend boot panic (container wiring regression, 9/30 merge window); (2) machine-wide
CoreTunnel proxy swallows simulator-app hostname traffic while the app gate forbids IP-literal origins.

sha256: sha256sums.txt. Secrets live only in ~/.t01-live-creds.env (mode 600) and were never committed.
