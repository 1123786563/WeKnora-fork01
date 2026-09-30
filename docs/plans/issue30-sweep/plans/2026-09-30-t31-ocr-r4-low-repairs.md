# T31 OCR R4 Low Diagnostic Repair Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the generated iOS scene verifier parse valid plist values without losing the scene manifest and report project-structure failures at their actual source.

**Architecture:** Keep the verifier's dependency-free parser, but consume complete plist scalar tokens and valid empty string/data/container forms. Treat each missing generated file as an independent diagnostic and skip only checks that depend on that file; distinguish an unlocatable app target configuration list from a located target whose deployment value differs from 16.4.

**Tech Stack:** TypeScript, Node.js `node:test`, `tsx`, Expo SDK 57 project validation.

**Spec:** `docs/plans/issue30-sweep/plans/plan-t31-ios27.md`; approved `docs/specs/2026-09-20-mobile-ai-office-design.md`; `docs/adr/0005-weknora-native-mobile-client.md`; `CONTEXT.md`; Issue #31 snapshot `docs/plans/issue30-sweep/issues/issue-31.md`; OCR R4 low adjudication `docs/plans/issue30-sweep/plans/2026-09-30-t31-ocr-r4-low-adjudication.md`.

## Global Constraints

- Preserve the SDK 57 generated-project contract: application scene role maps to `EXExpoAppSceneDelegate`, AppDelegate provides both URL callbacks, and all app target deployment settings are `16.4`.
- Preserve the dependency-free verifier and its current public signature `verifyIosSceneProject(iosDirectory: string): string[]`.
- A missing generated file is reported by its path; do not add downstream diagnostics that require parsing that absent file.
- A valid plist value unrelated to the scene manifest must not make the verifier reject an otherwise valid scene contract.
- Do not change mobile runtime behavior or the accepted SafeAreaProvider → SafeAreaView → Stack structure.

## Review Focus

- An empty `<string></string>` or self-closing `<string/>` before the scene manifest does not shift parser tokens or hide the valid delegate; `generated SDK57 scene checker parses valid empty and unrelated plist scalar values`.
- Legal plist scalar/container values (`integer`, `real`, `date`, `data`, empty `dict`/`array`) before the scene manifest are consumed as one value; malformed values still fail closed; same parser regression test.
- An unreadable or structurally unparseable `project.pbxproj` is not reported as an incorrect deployment value; `generated SDK57 scene checker distinguishes an unreadable PBX target block from a deployment mismatch`.
- Missing Info.plist, AppDelegate, project.pbxproj, or Podfile properties reports the missing path without dependent scene, callback, deployment, or JSON cascades; `generated SDK57 scene checker suppresses downstream errors for missing generated files`.
- A located app target with an absent/wrong deployment setting continues to report the explicit `16.4` contract; covered by the structure/deployment regression test.

---

### Task 1: Correct plist token consumption and generated-file diagnostics

**Dependencies:** T31 R4 source checkpoint `840f64cd8ed162bc3b216ca2a4a28b9d59bfec1a`; adjudicated OCR findings R4-L1 and R4-L2. No task-level dependency on another Issue #30 node.

**Owner role:** `frontend_implementer`. **Validator role:** `frontend_validator`. **Independent reviewer:** `reviewer`.

**Files:**
- Modify: `apps/mobile/scripts/verify-ios-scene-project.ts`
- Test: `apps/mobile/src/scripts/verify-ios-scene-project.test.ts`

**Interfaces:**
- Consume: `verifyIosSceneProject(iosDirectory: string): string[]` and its existing temporary generated-project fixture.
- Produce: the same function signature and stable valid-contract behavior; malformed PBX structure reports `Unable to locate WeKnora build configuration list in project.pbxproj`; a resolved app target with any deployment value other than `16.4` reports `All app target deployment settings must be 16.4`; absent files report only their missing-file diagnostic and checks for their absent contents are skipped.

- [ ] **Step 1: Add the failing valid-plist regression.** In `apps/mobile/src/scripts/verify-ios-scene-project.test.ts`, add `generated SDK57 scene checker parses valid empty and unrelated plist scalar values` with this fixture mutation and assertion:

```ts
const plistPath = join(root, 'WeKnora', 'Info.plist');
const source = readFileSync(plistPath, 'utf8');
const unrelated = '<key>Empty</key><string></string><key>EmptySelfClosing</key><string/><key>Count</key><integer>7</integer><key>Ratio</key><real>1.5</real><key>Created</key><date>2026-09-30T00:00:00Z</date><key>Blob</key><data>AA==</data><key>EmptyDictionary</key><dict/><key>EmptyArray</key><array/>';
writeFileSync(plistPath, source.replace('<key>UIApplicationSceneManifest</key>', `${unrelated}<key>UIApplicationSceneManifest</key>`));
assert.deepEqual(verifyIosSceneProject(root), []);
```

Run `pnpm --filter @weknora/mobile exec tsx --test src/scripts/verify-ios-scene-project.test.ts`; expected RED is the current false `application scene role must map to EXExpoAppSceneDelegate` diagnostic.
- [ ] **Step 2: Add the failing structure and missing-file regressions.** Add `generated SDK57 scene checker distinguishes an unreadable PBX target block from a deployment mismatch` with these cases:

```ts
const projectPath = join(root, 'WeKnora.xcodeproj', 'project.pbxproj');
fixture(root);
writeFileSync(projectPath, '/* readable project file with no target sections */');
let messages = verifyIosSceneProject(root).join('\n');
assert.ok(messages.includes('Unable to locate WeKnora build configuration list in project.pbxproj'));
assert.doesNotMatch(messages, /All app target deployment settings must be 16\.4/);

fixture(root);
writeFileSync(projectPath, readFileSync(projectPath, 'utf8').replace(
  'IPHONEOS_DEPLOYMENT_TARGET = 16.4;',
  'IPHONEOS_DEPLOYMENT_TARGET = 16.0;',
));
messages = verifyIosSceneProject(root).join('\n');
assert.ok(messages.includes('All app target deployment settings must be 16.4'));
```

Also add `generated SDK57 scene checker suppresses downstream errors for missing generated files` using this table-driven behavior:

```ts
const missingCases = [
  ['WeKnora/Info.plist', /application scene role/],
  ['WeKnora/AppDelegate.swift', /must conform|callback must forward/],
  ['WeKnora.xcodeproj/project.pbxproj', /build configuration list|deployment settings/],
  ['Podfile.properties.json', /must be valid JSON with iOS deployment target/],
] as const;
for (const [path, dependentMessage] of missingCases) {
  const root = mkdtempSync(join(tmpdir(), 'ios-scene-contract-'));
  try {
    fixture(root);
    unlinkSync(join(root, path));
    const messages = verifyIosSceneProject(root).join('\n');
    assert.ok(messages.includes(`missing generated file: ${path}`));
    assert.doesNotMatch(messages, dependentMessage);
  } finally { rmSync(root, { recursive: true, force: true }); }
}
```

Import `unlinkSync` from `node:fs`. Each case creates and removes a fresh temporary root. Run the focused test command; expected RED includes the current downstream cascade and mislabeled PBX error.
- [ ] **Step 3: Implement complete value consumption for the supported plist subset.** Extend the tokenizer to recognize `real`, `date`, and `data` tokens. In `parseValue`, accept empty/self-closing string and data values plus self-closing empty dict/array values; consume exactly one matching closing token for paired scalar elements, including empty strings; throw on an invalid or mismatched closing element. Continue parsing Boolean and non-empty container forms as before.
- [ ] **Step 4: Gate diagnostics on file availability and parser structure.** Change the internal `read` helper to return `string | undefined` while adding the existing missing-path diagnostic. Run scene validation only when Info.plist was read; AppDelegate checks only when AppDelegate.swift was read; PBX validation only when project.pbxproj was read; Podfile JSON validation only when its file was read. When a readable PBX file has no app target configuration IDs, emit the unable-to-locate diagnostic; only compare deployment values to 16.4 after configuration IDs were found.
- [ ] **Step 5: Verify RED → GREEN → REFACTOR.** Re-run `pnpm --filter @weknora/mobile exec tsx --test src/scripts/verify-ios-scene-project.test.ts` after each minimal implementation slice and confirm all verifier tests pass. Refactor only after GREEN; keep separate missing-file paths and stable runtime acceptance assertions.
- [ ] **Step 6: Run affected package checks.** Run `pnpm --filter @weknora/mobile test`, `pnpm --filter @weknora/mobile typecheck`, and `git diff --check`. Expected: mobile tests and TypeScript typecheck pass with no whitespace errors. Native generated-project verification is not required for a parser diagnostic-only change; record that the generated `apps/mobile/ios` directory remains unavailable if still absent.
- [ ] **Step 7: Commit and report.** Commit only the two owned files. Write the task report at `.superpowers/sdd/2026-09-30-t31-ocr-r4-low-repairs/task-1-report.md`, including plan and brief paths, BASE/HEAD, commands and actual outcomes, implementation SHA, diff/payload hash, and the disposition of R4-L1/R4-L2.

**Failure handling:** If a valid plist encoding still cannot be parsed without changing the public contract, retain fail-closed validation and report the exact token sequence and minimal fixture; do not silence scene validation or infer that a missing scene manifest is valid. If project structure is unparseable, report that fact without asserting a deployment mismatch.

## Plan self-review

- **Spec coverage:** The T31 plan's scene mapping, callback, and 16.4 deployment checks remain unchanged; this plan repairs only diagnostics and parsing inputs that precede those checks.
- **Task graph:** One independently reviewable Task owns both source and fixtures because both findings are in the same verifier and share its file-read/diagnostic flow; no second task consumes an unstable interface.
- **Type and message consistency:** `read` returns `string | undefined` internally only. The exported verifier signature is unchanged. The unable-to-locate message is distinct from the exact 16.4 drift message.
- **Review Focus coverage:** Every listed input has a named test and its owning file is explicit.
- **Scope:** Four style-only OCR suggestions (brace-scanner deduplication, constant extraction, callback helper extraction, and static RootLayout props) remain deferred per the separate adjudication; this Task does not alter RootLayout or unrelated parser behavior.
