#!/usr/bin/env node
/**
 * R476 A1: composite gate runner for the React parity worktree.
 *
 * Serial pipeline (one command for future rounds):
 *   1. node >= 26 check (the v22.22.3 default on PATH produces the known
 *      tsx/CJS createPortal false-reds; R475 A4 dual-node experiment proved
 *      v26 fully green) — auto re-execs through a >=26 binary when needed.
 *   2. four gates: test:shared -> typecheck:shared -> test:web -> typecheck:web
 *   3. integrity: check:integrity
 *   4. build: build:web (R477 A1: appended so a full gates run also proves the
 *      production web bundle builds under node >=26)
 *
 * Node >=26 discovery order: $WEKNORA_NODE_BIN, homebrew node, nvm versions.
 * On re-exec only a temp shim dir containing a `node` symlink is prepended to
 * PATH, so child processes (pnpm -> node --import tsx, tsc) resolve node >=26
 * while `pnpm` itself keeps resolving to the project's pinned version.
 *
 * Usage: pnpm gates
 */
import { spawnSync } from "node:child_process";
import {
  MIN_MAJOR,
  createNodeShim,
  exitWithSpawnResult,
  findNodeGte26,
  installShimSignalCleanup,
  joinShimPath,
  majorOf,
  removeNodeShim,
} from "./lib/node-gte26.mjs";

const GATES = [
  ["test:shared", "pnpm run test:shared"],
  ["typecheck:shared", "pnpm run typecheck:shared"],
  ["test:web", "pnpm run test:web"],
  ["typecheck:web", "pnpm run typecheck:web"],
  ["check:integrity", "pnpm run check:integrity"],
  ["build:web", "pnpm run build:web"],
];

// majorOf/candidateBin/findNodeGte26 moved to scripts/lib/node-gte26.mjs
// (OCR round-1 F11: the duplicated copies had drifted into a copy-paste
// contract — the CI Linux discovery gap, F09, lived in both).

function reexecWith(node) {
  // Prepend a shim dir containing ONLY a `node` symlink to the >=26 binary.
  // Prepending the whole bin dir (e.g. /opt/homebrew/bin) would also shadow
  // `pnpm` with a different major (v11 vs the pinned 10.28.2) and corepack
  // refuses the mismatch — observed in the R476 smoke test.
  //
  // The shim dir comes from the shared module (OCR round-1 F10/F11): the
  // previous FIXED name `weknora-gates-node-shim` under the shared tmpdir was
  // predictable and pre-creatable by another local user (CWE-377/426) — and
  // racing concurrent runs. mkdtempSync is private and unique per run.
  const { shimDir, shimNode } = createNodeShim(node.bin);
  installShimSignalCleanup(shimDir);
  const env = {
    ...process.env,
    // Platform-delimiter join (跨任务转交 T01-OCR1-F12): a hard-coded ':'
    // broke PATH resolution on win32 even when the shim itself had succeeded.
    PATH: joinShimPath(shimDir, process.env.PATH),
  };
  console.error(`[gates] current node ${process.version} < ${MIN_MAJOR}; re-exec via ${node.bin} (${node.version})`);
  const result = spawnSync(shimNode, [process.argv[1], ...process.argv.slice(2)], {
    stdio: "inherit",
    env,
  });
  removeNodeShim(shimDir);
  exitWithSpawnResult(result, "gates");
}

function main() {
  if (majorOf(process.version) < MIN_MAJOR) {
    const node = findNodeGte26();
    if (!node) {
      console.error(
        `[gates] node >= ${MIN_MAJOR} required (current ${process.version}); ` +
          "no >=26 binary found via WEKNORA_NODE_BIN, platform prefixes, ~/.nvm, or $PATH — install node 26 (e.g. brew install node@26 / nvm install 26)",
      );
      process.exit(1);
    }
    reexecWith(node);
  }
  console.log(`[gates] node ${process.version} OK (>= ${MIN_MAJOR})`);

  const failures = [];
  for (const [name, cmd] of GATES) {
    const started = Date.now();
    console.log(`\n[gates] >>> ${name}: ${cmd}`);
    const result = spawnSync(cmd, { stdio: "inherit", shell: true, env: process.env });
    const seconds = ((Date.now() - started) / 1000).toFixed(1);
    const ok = result.status === 0;
    console.log(`[gates] <<< ${name} ${ok ? "PASS" : "FAIL"} (${seconds}s)`);
    if (!ok) failures.push(name);
  }

  console.log("\n[gates] summary:");
  for (const [name] of GATES) {
    console.log(`  ${failures.includes(name) ? "FAIL" : "PASS"}  ${name}`);
  }
  if (failures.length > 0) {
    console.error(`[gates] ${failures.length} gate(s) failed: ${failures.join(", ")}`);
    process.exit(1);
  }
  console.log("[gates] all gates green");
}

main();
