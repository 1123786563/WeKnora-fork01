# V02 ownership fix

Cleanup ownership now stores each write-created capture's immutable value and
refuses deletion when a later read overwrites that capture. Regression coverage
proves an ID changing from A to B cannot delete B.

`python3 -m unittest scripts.saas.probe_case_test -v` passes 11 tests; live
commercial experiments remain blocked-env.
