# R171 React bundle after control migrations

- Scope: Web and Embed production bundle after shared `Range` and resource-tree checkbox migrations.
- Validation: `pnpm build:react-bundle` passed; Web transformed 2,448 modules and the React Web/Embed bundle was copied to `dist/react-web/web`. Existing large-chunk advisory remains informational.
- Boundary: native runtime visual and protected backend acceptance remain open.
