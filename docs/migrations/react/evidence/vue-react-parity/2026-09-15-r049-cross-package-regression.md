# R049 Cross-package regression after chat mention expansion

- Shared contracts/domain/API/i18n/UI/views: 465/465 tests passed.
- Mobile: 189/189 tests passed.
- Embed: 7/7 tests passed.
- Desktop renderer: 2/2 tests passed.
- Scope: validates the widened chat mention resource contract does not break shared consumers or native/embed adapters.
- Limitation: these are package-level checks; they do not replace authenticated browser or native device acceptance.
