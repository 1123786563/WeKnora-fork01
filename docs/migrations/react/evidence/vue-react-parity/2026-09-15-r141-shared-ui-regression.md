# R141 shared UI and production regression

- Shared package regression: `pnpm test:shared` passed 469/469, including the Checkbox primitive and settings contracts.
- Web production build: `pnpm --dir apps/web run build` passed; emitted `index-Dnr-iyRV.js` (2,538.64 kB, gzip 642.89 kB).
- Existing >500 kB chunk advisory remains open as a performance follow-up.
