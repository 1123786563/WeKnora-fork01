# R196 model settings heading metrics

- Scope: model settings section header.
- Vue baseline: `ModelSettings.vue` defines a 20px, 600-weight title with 28px line-height, an 14px subtitle with 1.6 line-height, and an 8px title margin.
- Finding: live React inherited a 30px title line-height and 16px/24px subtitle, producing a measurable vertical mismatch despite the existing structural tests.
- Change: `ModelSettingsPanel` now applies the Vue title and subtitle metrics through Tailwind utilities.
- Browser evidence: React computed styles after the change are title `20px/28px` and subtitle `14px/22.4px`, at the same x/y origin as the Vue section header fixture.
- Validation: ModelSettings focused tests 23/23 and Web typecheck pass. Full Web regression was already re-run after the preceding organization changes at 895/895; this metric-only change requires the next cross-package regression gate.
