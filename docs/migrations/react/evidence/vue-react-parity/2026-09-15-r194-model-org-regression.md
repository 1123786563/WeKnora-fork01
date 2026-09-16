# R194 model and organization regression

- Validation after model-panel spacing and organization modal/permission/search refinements: Web 895/895, Web typecheck, Web production build and `git diff --check` pass.
- Production build transformed 2,448 modules and retained the existing large-chunk advisory (main chunk about 2.54 MB minified); no new build failure was introduced.
- Runtime screenshots cover authenticated model settings and organization create/permission dialogs at 1355x720 zh-CN; protected writes and native hosts remain open.
