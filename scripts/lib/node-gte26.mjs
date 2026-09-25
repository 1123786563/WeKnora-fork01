/**
 * Shared node >= 26 discovery / shim plumbing for the gate scripts.
 *
 * Extracted from the duplicated copies in scripts/run-with-node-gte26.mjs and
 * scripts/run-gates.mjs (OCR round-1 F11): MIN_MAJOR, candidate paths and the
 * shim discipline must move in lockstep — the CI Linux discovery gap (F09)
 * existed in BOTH copies precisely because they had drifted into a
 * copy-paste contract.
 *
 * consumers:
 *   scripts/run-with-node-gte26.mjs  — wrapper entry (`-- <command>`)
 *   scripts/run-gates.mjs            — composite gate runner (re-exec path)
 */
import { execFileSync } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";

export const MIN_MAJOR = 26;

export function majorOf(version) {
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

/**
 * Find a node >= MIN_MAJOR binary. Discovery order (first hit wins):
 *   1. $WEKNORA_NODE_BIN (explicit override)
 *   2. platform prefixes — homebrew on darwin; /usr/local/bin + /usr/bin on
 *      Linux (F09: the previous darwin-only branch left GitHub's ubuntu
 *      runners with no candidates at all)
 *   3. ~/.nvm/versions/node (highest major first)
 *   4. every $PATH entry (F09: actions/setup-node exports the toolchain via
 *      PATH — none of the fixed locations above contain it)
 * Each candidate is version-checked, so a <26 node found on PATH is skipped.
 */
export function findNodeGte26() {
  const candidates = [];
  if (process.env.WEKNORA_NODE_BIN) candidates.push(process.env.WEKNORA_NODE_BIN);
  if (process.platform === "darwin") {
    candidates.push("/opt/homebrew/bin/node", "/usr/local/bin/node");
  } else {
    candidates.push("/usr/local/bin/node", "/usr/bin/node");
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
  for (const dir of (process.env.PATH || "").split(path.delimiter)) {
    if (dir) candidates.push(path.join(dir, "node"));
  }
  const seen = new Set();
  for (const bin of candidates) {
    if (seen.has(bin)) continue;
    seen.add(bin);
    const hit = candidateBin(bin);
    if (hit) return hit;
  }
  return null;
}

/**
 * Create the PATH shim directory holding ONLY a `node` symlink to the >=26
 * binary. mkdtempSync (F10) gives an unpredictable name AND creates the
 * directory with 0700 in one step — the previous predictable
 * `weknora-node26-shim-<pid>` name under the shared tmpdir could be
 * pre-created by another local user and primed with a malicious `tsx`/`sh`
 * that this script would then resolve first from PATH (CWE-377/426).
 */
export function createNodeShim(targetBin, { linker = fs.symlinkSync } = {}) {
  const shimDir = fs.mkdtempSync(path.join(os.tmpdir(), "weknora-node26-shim-"));
  const shimNode = path.join(shimDir, "node");
  fs.rmSync(shimNode, { force: true });
  try {
    linker(targetBin, shimNode);
  } catch (cause) {
    // win32 without developer mode / admin rights denies symlinks with EPERM
    // (跨任务转交 T01-OCR1-F12): crashing there made `pnpm test:shared`
    // unusable for local Windows developers. Fall back to a byte-identical
    // copy of the binary — heavier, but correct, and Windows has no CI gate
    // riding on this path. Any copy failure still fails fast.
    console.warn(
      `[node-gte26] symlink denied (${cause && cause.code ? cause.code : cause}) — falling back to copying ${targetBin}`,
    );
    fs.copyFileSync(targetBin, shimNode);
  }
  return { shimDir, shimNode };
}

/**
 * Prepend the shim dir to PATH with the PLATFORM delimiter (跨任务转交
 * T01-OCR1-F12): the hard-coded ':' join produced a single garbage entry on
 * win32 (';' is the delimiter there), silently breaking PATH resolution even
 * when the shim itself had succeeded.
 */
export function joinShimPath(shimDir, existingPath) {
  return [shimDir, existingPath || ""].filter(Boolean).join(path.delimiter);
}

/** Best-effort shim removal; never throws (callers run it on every exit path). */
export function removeNodeShim(shimDir) {
  try {
    fs.rmSync(shimDir, { recursive: true, force: true });
  } catch {
    /* best-effort cleanup */
  }
}

/**
 * Signal-time cleanup (F12): SIGINT/SIGTERM skip the post-spawn cleanup
 * block, so without these handlers every interrupted run leaks a shim dir
 * into the shared tmpdir. Exit codes are the conventional 130/143.
 *
 * Measured limitation: while the parent is blocked inside a synchronous
 * spawnSync, Node cannot run JS signal handlers — a signal delivered to the
 * PARENT ONLY in that window breaks the wait without dispatching the handler
 * here (the post-spawn cleanup still removes the shim dir). The realistic
 * interrupt paths (terminal Ctrl+C, CI cancel) signal the whole process
 * GROUP, so the child dies by signal and exitWithSpawnResult reports it.
 */
export function installShimSignalCleanup(shimDir) {
  const onSignal = (code) => {
    removeNodeShim(shimDir);
    process.exit(code);
  };
  process.on("SIGINT", () => onSignal(130));
  process.on("SIGTERM", () => onSignal(143));
}

/**
 * Exit-code discipline (F08 / 跨任务转交 T01-OCR1-F2): a child killed by a
 * signal (OOM SIGKILL, CI cancel SIGTERM) makes spawnSync return status:null
 * WITHOUT setting error — `status ?? (error ? 1 : 0)` then exits 0 and the
 * gate records a pass for a murdered test run. Both failure shapes now exit
 * 1 with a diagnostic line.
 *
 * Returns the exit code; the caller performs process.exit. `opts.log`
 * (defaults to console.error) is the diagnostic sink so tests can capture it.
 */
export function spawnExitCode(result, { log = console.error } = {}) {
  if (result.error) {
    log(`spawn failed to start: ${result.error.message}`);
    return 1;
  }
  if (result.signal) {
    log(`spawn terminated by signal ${result.signal}`);
    return 1;
  }
  return result.status ?? 0;
}

/** Labeled wrapper exit: same discipline, with the script tag prefix. */
export function exitWithSpawnResult(result, label) {
  process.exit(spawnExitCode(result, { log: (message) => console.error(`[${label}] ${message}`) }));
}
