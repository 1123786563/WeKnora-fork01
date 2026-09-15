# R183 Web preview smoke

- Scope: built Web artifact served by Vite preview.
- Validation: `vite preview --host 127.0.0.1 --port 4179` started successfully; `curl` received HTTP 200 with the WeKnora title and hashed JavaScript entry, then the preview process was stopped cleanly.
- Boundary: this is a static serving smoke check, not protected backend or browser interaction acceptance.
