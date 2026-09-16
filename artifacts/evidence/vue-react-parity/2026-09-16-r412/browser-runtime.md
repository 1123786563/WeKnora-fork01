# R412 browser runtime evidence

- Browser Use local CDP connection was available and returned real rendered pages.
- Vue: `http://127.0.0.1:5180/platform/knowledge-bases`, title `🐴 WeKnora`,
  viewport report `1470x745`; the existing browser session was authenticated
  and the requested `/login` URL redirected to this protected page.
- React: `http://127.0.0.1:5181/login`, title `🐴 WeKnora`, viewport report
  `1470x745`; the page rendered the login surface, while its content report was
  `1455x963` because the local browser layout differs for this tab.
- Screenshots: `vue-platform-knowledge-bases.png` and `react-login.png`.
- These are real browser-rendering observations only. They are not a paired
  same-state visual acceptance: the Vue tab has an authenticated session and
  the React tab does not. No credentials, MFA, or session transfer was used.
