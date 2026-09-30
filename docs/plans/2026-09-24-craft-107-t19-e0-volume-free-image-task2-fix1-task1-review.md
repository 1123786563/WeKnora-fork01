# Craft #107 T19 E0 volume-free runner Fix1 Task 1 — independent review

Date: 2026-09-24. Scope: VI-1 exact derivative image pin in the full verifier and its focused synthetic tests. Read the Fix1 plan, original VI-1/VI-2 review, approved Craft web-artifact Spec, `CONTEXT.md`, relevant ADRs, implementation report, incremental checkpoint, verifier and focused tests. No Docker, OCR, source/test edit, or remote issue operation.

## Findings

No blocking finding in the Task 1 scope.

## Evidence

The incremental patch SHA-256 is `de5b1eedbdcdcf5c090482f25536fe24e6869e2f0be8c563a934cc23cef10822`, matching the checkpoint. Both captured preimages and postimages match their checkpoint hashes and current files. Applying the patch to the preimages in a temporary directory returned 0 and reproduced both postimages byte for byte: `assert_v2.py` `8e410d4021e06a6e024d51b66124fb2482d017e836d83f7ffd07a1d8f2650869`, `test_assert_v2.py` `d3d7fa30a683474ab82f557abf86827cd65e69ae753582d190e0ca5af33293a7`.

The verifier production change is exactly one literal at `assert_v2.py:966`: its exact immutable pin now equals the reviewed derivative `sha256:87d2f57937114373d64c804a4a7fc44c0f0c1e70e340b88b14da83215e4f4615`. It does not accept an arbitrary manifest value. Raw participant image-to-manifest comparison at `:552`, fixed UID/capability/network/mount/source-isolation topology checks at `:548-646`, and platform/version, binary and source-evidence checks remain unchanged by this patch. This is an identity alignment, not a relaxed topology policy.

The new test clones a complete synthetic artifact, sets its manifest image, image-case result and pre/post raw C/D/A/helper inspect to the derivative ID, refreshes raw hashes, and requires full verifier success with an explicit synthetic label. It then mutates the manifest ID to the old base and to an arbitrary SHA-256 value, requiring the `wrong or missing pinned image ID` diagnostic. The recorded RED run failed the derivative fixture under the old pin. I independently ran `python3 test_assert_v2.py` from the fixture directory: exit 0; derivative fixture accepted; old base and arbitrary IDs rejected at exit 1; all existing source, topology, host-command, cleanup and attempt-provenance mutations rejected; both quiescence checks completed. This is synthetic evidence only and establishes no measured E0 PASS.

## Verdict

**Spec compliance: PASS for VI-1's exact derivative pin. Code quality: PASS for the narrow one-line verifier change and its positive/negative synthetic coverage.** VI-2 (runner manifest C volume names) is a separate Task 2 correction. Full E0 remains **BLOCKED** pending that integration and the physical provider/network-attempt matrix.
