# R174 platform shell test harness

- Scope: platform shell tests importing the logo asset.
- Change: Node test module hooks now stub PNG imports alongside CSS, keeping shell tests deterministic after real logo asset usage; added missing newline normalization in touched tests.
- Validation: platform shell/guide/org/session tests passed 32/32; Web typecheck and diff check passed.
