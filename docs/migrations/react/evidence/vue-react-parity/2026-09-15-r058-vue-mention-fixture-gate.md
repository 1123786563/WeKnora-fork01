# R058 Vue mention fixture gate

- Runtime: authenticated Vue Web `/platform/creatChat` on the shared parity tenant.
- Observed: the Vue page renders the composer and disabled send control, but no mention control is mounted in this tenant state; the input toolbar exposes only the send button.
- Consequence: a Vue/React computed-style comparison for the mention selector cannot be completed from this fixture. The React baseline remains recorded in R057, and a tenant with the Vue mention capability enabled is required for a valid cross-end comparison.
- No configuration or backend data was changed.
