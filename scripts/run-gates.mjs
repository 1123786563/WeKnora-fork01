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
import { execFileSync, spawnSync } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";

const MIN_MAJOR = 26;
const GATES = [
  ["test:shared", "pnpm run test:shared"],
  ["typecheck:shared", "pnpm run typecheck:shared"],
  ["test:web", "pnpm run test:web"],
  ["typecheck:web", "pnpm run typecheck:web"],
  ["check:integrity", "pnpm run check:integrity"],
  ["build:web", "pnpm run build:web"],
];

function majorOf(version) {
  return Number.parseInt(String(version).trim().replace(/^v/, "").split(".")[0], 10) || 0;
}

function candidateBin(nodeBin) {
  try {
    const v = execFileSync(nodeBin, ["-v"], { encoding: "utf8" });
    return majorOf(v) >= MIN_MAJOR ? { bin: nodeBin, version: v.trim() } : null;
  } catch {
    return null;
  }
}

function findNodeGte26() {
  const candidates = [];
  if (process.env.WEKNORA_NODE_BIN) candidates.push(process.env.WEKNORA_NODE_BIN);
  if (process.platform === "darwin") {
    candidates.push("/opt/homebrew/bin/node", "/usr/local/bin/node");
  }
  const nvmDir = path.join(os.homedir(), ".nvm", "versions", "node");
  if (fs.existsSync(nvmDir)) {
    const nvmNodes = fs
      .readdirSync(nvmDir)
      .filter((d) => /^v\d+\./.test(d) && majorOf(d) >= MIN_MAJOR)
      .sort((a, b) => majorOf(b) - majorOf(a) || a.localeCompare(b, undefined, { numeric: true }))
      .map((d) => path.join(nvmDir, d, "bin", "node"));
    candidates.push(...nvmNodes);
  }
  for (const bin of candidates) {
    const hit = candidateBin(bin);
    if (hit) return hit;
  }
  return null;
}

function reexecWith(node) {
  // Prepend a shim dir containing ONLY a `node` symlink to the >=26 binary.
  // Prepending the whole bin dir (e.g. /opt/homebrew/bin) would also shadow
  // `pnpm` with a different major (v11 vs the pinned 10.28.2) and corepack
  // refuses the mismatch — observed in the R476 smoke test.
  const shimDir = path.join(os.tmpdir(), "weknora-gates-node-shim");
  const shimNode = path.join(shimDir, "node");
  fs.mkdirSync(shimDir, { recursive: true });
  try {
    fs.rmSync(shimNode, { force: true });
  } catch {
    /* ignore */
  }
  fs.symlinkSync(node.bin, shimNode);
  const env = {
    ...process.env,
    PATH: `${shimDir}:${process.env.PATH || ""}`,
  };
  console.error(`[gates] current node ${process.version} < ${MIN_MAJOR}; re-exec via ${node.bin} (${node.version})`);
  const result = spawnSync(shimNode, [process.argv[1], ...process.argv.slice(2)], {
    stdio: "inherit",
    env,
  });
  process.exit(result.status ?? (result.error ? 1 : 0));
}

function main() {
  if (majorOf(process.version) < MIN_MAJOR) {
    const node = findNodeGte26();
    if (!node) {
      console.error(
        `[gates] node >= ${MIN_MAJOR} required (current ${process.version}); ` +
          "no >=26 binary found via WEKNORA_NODE_BIN, homebrew, or ~/.nvm — install node 26 (e.g. brew install node@26 / nvm install 26)",
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
