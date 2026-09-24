#!/usr/bin/env node
/**
 * Gate regression fix: run an arbitrary command through node >= 26.
 *
 * `pnpm run test:shared` executes tsx via the node resolved from PATH. When
 * the ambient PATH only offers node v22 (the shell default on this host),
 * react-dom's CJS interop breaks and three unrelated .tsx tests false-red
 * with `createRoot is not a function` (known v22 tsx/CJS issue recorded in
 * .superpowers/sdd/2026-09-23-issue-106/environment.md). Prefixing this
 * wrapper makes the script self-healing instead of PATH-sensitive.
 *
 * Behavior:
 *   node scripts/run-with-node-gte26.mjs -- <command> [args...]
 *   - current node >= 26          -> shim to the current binary (so children
 *                                    resolve the same major even if PATH is odd)
 *   - current node <  26          -> discover a >=26 binary ($WEKNORA_NODE_BIN,
 *                                    homebrew, ~/.nvm) and shim to it
 *   - no >=26 binary discoverable -> fail fast with an actionable message
 *
 * Shim strategy mirrors scripts/run-gates.mjs: only a `node` symlink is
 * prepended to PATH so `pnpm` keeps resolving to the pinned version while
 * child processes (tsx, node --import tsx) resolve node >= 26.
 */
import { spawnSync, execFileSync } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";

const MIN_MAJOR = 26;

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
          "no >=26 binary found via WEKNORA_NODE_BIN, homebrew, or ~/.nvm — install node 26 (e.g. brew install node@26 / nvm install 26)",
      );
      process.exit(1);
    }
    console.error(
      `[with-node] current node ${process.version} < ${MIN_MAJOR}; running ${command} via ${target.bin} (${target.version})`,
    );
  }

  // Per-pid shim dir avoids racing the fixed dir scripts/run-gates.mjs uses
  // when both run concurrently.
  const shimDir = path.join(os.tmpdir(), `weknora-node26-shim-${process.pid}`);
  const shimNode = path.join(shimDir, "node");
  fs.mkdirSync(shimDir, { recursive: true });
  fs.rmSync(shimNode, { force: true });
  fs.symlinkSync(target.bin, shimNode);
  const env = { ...process.env, PATH: `${shimDir}:${process.env.PATH || ""}` };

  const result = spawnSync(command, args, { stdio: "inherit", env });
  try {
    fs.rmSync(shimDir, { recursive: true, force: true });
  } catch {
    /* best-effort cleanup */
  }
  process.exit(result.status ?? (result.error ? 1 : 0));
}

main();
