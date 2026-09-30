# OCR Build Receipt Finding Ruling

**Date:** 2026-09-29
**Source:** `docs/plans/craft-107-ocr-source-round3-partial.md`, SHA-256 `8f36d582901e4af6a28f3c70dac7a83fdd2acacd071b3f82b188d053e452efcc`.

## Finding

The partial source OCR reports a High finding: production build evidence calls `CraftWebBuildEvidence` with no trusted observed-exit receipt, so writable `build-log.json` cannot establish `BuildRan`; the durable receipt repository has no production dispatch/read wiring.

## Ruling

**Valid production gap; blocked by unresolved F08/T03/T19 interfaces. Do not patch T04 by manufacturing a receipt or inferring build billing identity.**

## Evidence

- Approved F08 design states receipt persistence is integrated while production dispatch remains blocked on upstream identity and billing interfaces; F08 remains open (`docs/plans/2026-09-28-craft-107-f08-execution-receipt-design.md`, lines 1–3).
- `CraftWebBuildEvidence` rejects a nil observed exit code and requires the trusted receipt to agree with the diagnostic log (`internal/container/craft_web_build.go`, lines 291–328).
- `craftWebBuildEvidenceSource` currently passes nil for the trusted receipt (`internal/container/craft_web_build.go`, lines 373–385).
- Runtime registers the T04 command gate but documents its dispatch consumer as a future T20 face (`internal/container/craft_runtime.go`, lines 187–220).
- The receipt repository provides durable `RecordTerminal`/`Read`, but the F08 report explicitly leaves dispatch, evidence lookup, and promotion binding outside that slice.
- Normal-exec requires an explicit `CraftCallBinding`, fully staged request, and live provider-issued `sandbox.RemoteSandboxHandle`; it fails closed without the T03 policy (`internal/application/service/craft_docker_normal_exec.go`, lines 19–39 and 137–170).
- Persisted `CraftRunViewMaterialHandle` intentionally contains no Docker handle (`internal/container/craft_runview_material.go`, lines 16–29). Runtime provider interface does not establish a currently wired provider implementation (`internal/container/craft_runview_runtime.go`, lines 68–95).

## Safe closure seam

1. T03/T19 provide an explicit server-owned budget binding for the fixed build activity.
2. A provider-issued handle resolver verifies tenant, Task/session, Workspace, Run, generation, and exact live container identity.
3. T20 stages and executes the T04 request, persists the returned observation, and makes the trusted receipt available to evidence.
4. Receipt is bound to candidate/output generation and revalidated during promotion.

These are upstream contract changes, not a local logging change. Keep T04/T19/T20 and Spec #107 acceptance blocked until the contracts are implemented and verified. This ruling does not waive the High finding or count the acceptance as complete.

## Source limitations

The source OCR round was partial (17 of 26 selected files completed; 9 timed out/failed), so this ruling is limited to the adjudicated build-receipt finding and does not represent a full OCR pass.
