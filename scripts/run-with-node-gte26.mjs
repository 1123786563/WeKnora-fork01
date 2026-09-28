#!/usr/bin/env node
/**
 * Gate regression fix: run an arbitrary command through node >= 26.
 *
 * `pnpm run test:shared` (and `pnpm run test:craft:shared`, OCR round-1 F7 —
 * the craft entry runs the same react-dom createRoot dynamic imports) executes
 * tsx via the node resolved from PATH. When the ambient PATH only offers
 * node v22 (the shell default on this host), react-dom's CJS interop breaks
 * and unrelated .tsx tests false-red with `createRoot is not a function`
 * (known v22 tsx/CJS issue; the reproduction notes live in the local
 * workspace file .superpowers/sdd/2026-09-23-issue-106/environment.md, which
 * is NOT committed to the repository — OCR round-1 F13). Prefixing this
 * wrapper makes the entry self-healing instead of PATH-sensitive.
 *
 * Behavior:
 *   node scripts/run-with-node-gte26.mjs -- <command> [args...]
 *   - current node >= 26          -> shim to the current binary (so children
 *                                    resolve the same major even if PATH is odd)
 *   - current node <  26          -> discover a >=26 binary (override env,
 *                                    platform prefixes, ~/.nvm, $PATH scan —
 *                                    see scripts/lib/node-gte26.mjs, F09/F11)
 *                                    and shim to it
 *   - no >=26 binary discoverable -> fail fast with an actionable message
 *   - child exits non-zero / is killed by a signal / fails to spawn
 *                                -> exit non-zero with a diagnostic (F08)
 *
 * Shim strategy (scripts/lib/node-gte26.mjs): only a `node` symlink inside a
 * mkdtempSync private directory (F10) is prepended to PATH so `pnpm` keeps
 * resolving to the pinned version while child processes (tsx, node --import
 * tsx) resolve node >= 26; SIGINT/SIGTERM clean the shim dir up (F12).
 */
import { spawnSync } from "node:child_process";
import {
  MIN_MAJOR,
  createNodeShim,
  exitWithSpawnResult,
  findNodeGte26,
  installShimSignalCleanup,
  majorOf,
  removeNodeShim,
  shimEnv,
} from "./lib/node-gte26.mjs";

function main() {
  const dashDash = process.argv.indexOf("--");
  if (dashDash === -1 || process.argv.length <= dashDash + 1) {
    console.error("usage: node scripts/run-with-node-gte26.mjs -- <command> [args...]");
    process.exit(2);
  }
  const command = process.argv[dashDash + 1];
  const args = process.argv.slice(dashDash + 2);

  let target;
  if (majorOf(process.version) >= MIN_MAJOR) {
    target = { bin: process.execPath, version: process.version };
  } else {
    target = findNodeGte26();
    if (!target) {
      console.error(
        `[with-node] node >= ${MIN_MAJOR} required (current ${process.version}); ` +
          "no >=26 binary found via WEKNORA_NODE_BIN, platform prefixes, ~/.nvm, or $PATH — install node 26 (e.g. brew install node@26 / nvm install 26)",
      );
      process.exit(1);
    }
    console.error(
      `[with-node] current node ${process.version} < ${MIN_MAJOR}; running ${command} via ${target.bin} (${target.version})`,
    );
  }

  const { shimDir } = createNodeShim(target.bin);
  installShimSignalCleanup(shimDir);
  // shimEnv (R1-F30/F33): case-insensitive PATH overwrite — a naive spread
  // would materialize both `Path` (old) and `PATH` (new) on win32 and the
  // child would honor the old value, silently dropping the shim.
  const env = shimEnv(shimDir);

  // win32 (R1-F31): pnpm dispatches tsx as tsx.cmd, and spawnSync without a
  // shell cannot execute .cmd files (libuv does no PATHEXT resolution; Node
  // explicitly errors EINVAL/ENOENT). The command/args come from our own
  // package.json scripts, so the shell surface is fixed, not user input.
  const result = spawnSync(command, args, {
    stdio: "inherit",
    env,
    shell: process.platform === "win32",
  });
  removeNodeShim(shimDir);
  exitWithSpawnResult(result, "with-node");
}

main();
