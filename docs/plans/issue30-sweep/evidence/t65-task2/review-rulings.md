# T65 Task 2 independent review record

Reviewed implementation range: `93706830b78205de0c7d433097e89f33d9726513..e7c42c51bc31f0215f3f4f27850246ca55e773ba`.

The initial independent review found (1) duplicated eligibility SQL across detail/catalog/transaction paths despite the plan requiring one shared predicate, (2) incomplete active Adoption and provenance-preservation assertions, and (3) a possible public Listing unlist race. R1 extracted `publicListingEligibilityQuery(*gorm.DB)` for all three paths and added persisted Adoption/provenance/bundle preservation assertions. Independent scoped re-review confirmed the first two findings resolved.

Ruling T2-R1-1: no new public unlist API was introduced. Code search found no supported public Listing-state writer: the production state transition operates on the publisher's tenant source; public repository writes only advance current Release; direct public `state=unlisted` was test seeding. Future public state writers must use the publisher guard. Residual risk if this inventory is incomplete: a future writer outside the guarded path could race admission.

At the fix HEAD, the focused predicate/custody tests, ten-repeat custody/introduction/revocation group, public marketplace service tests, `git diff --check`, and `go build ./...` passed. The build emitted duplicate `-lc++` linker warnings. The first fix-validator report had an implementation-report path limitation; the controller reran the exact full fix-plan command set and the implementation report now records raw outputs. Re-review confirmed evidence closure. PostgreSQL runtime migration testing is not in this Task 2 scope.
