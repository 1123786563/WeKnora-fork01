/**
 * Regression tests for scripts/lib/node-gte26.mjs (跨任务转交修复轮).
 *
 * Covers the handover findings:
 *   - T01-OCR1-F2 (F08): a child killed by a signal (status null, no error)
 *     must exit non-zero — the old `status ?? (error ? 1 : 0)` produced a
 *     false-green 0.
 *   - T01-OCR1-F12: on win32, symlinkSync can throw EPERM (no developer
 *     mode / non-admin) — the shim must fall back to copying the binary
 *     instead of crashing; PATH must be joined with path.delimiter (';' on
 *     win32), never a hard-coded ':'.
 *
 * Run: node --test scripts/lib/node-gte26.test.mjs
 */
import test from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import {
  MIN_MAJOR,
  createNodeShim,
  findNodeGte26,
  joinShimPath,
  majorOf,
  shimEnv,
  shimEntryName,
  spawnExitCode,
} from "./node-gte26.mjs";

test("majorOf parses plain, v-prefixed and dirty version strings", () => {
  assert.equal(majorOf("v26.1.0"), 26);
  assert.equal(majorOf("26.1.0"), 26);
  assert.equal(majorOf("  v22.11.0\n"), 22);
  assert.equal(majorOf(""), 0);
  assert.equal(majorOf("not-a-version"), 0);
});

test("joinShimPath puts the shim dir first and uses the platform delimiter", () => {
  // Platform-neutral inputs (R1-F29): literal POSIX strings made this test a
  // deterministic false-red on win32, where ':' is an ordinary character.
  const first = path.join(os.tmpdir(), "bin-a");
  const second = path.join(os.tmpdir(), "bin-b");
  const third = path.join(os.tmpdir(), "bin-c");
  const existing = [second, third].join(path.delimiter);
  const joined = joinShimPath(first, existing);
  const parts = joined.split(path.delimiter);
  assert.equal(parts.length, 3);
  assert.equal(parts[0], first);
  assert.equal(parts[1], second);
  assert.equal(parts[2], third);
});

test("joinShimPath tolerates an empty existing PATH without a trailing delimiter", () => {
  const shimDir = path.join(os.tmpdir(), "shim-dir");
  const joined = joinShimPath(shimDir, "");
  assert.equal(joined, shimDir);
});

test("shimEnv overwrites the existing PATH case variant instead of adding a second key (R1-F30/F33)", (t) => {
  const previousPath = process.env.PATH;
  const previousVariant = process.env.Path;
  // Simulate the win32 shape where the inherited key is cased `Path`: on a
  // case-sensitive platform both keys can coexist in process.env, which is
  // exactly the duplicate shape the naive spread would ship to the child.
  process.env.Path = "/legacy/value";
  delete process.env.PATH;
  t.after(() => {
    process.env.PATH = previousPath;
    if (previousVariant === undefined) delete process.env.Path;
    else process.env.Path = previousVariant;
  });
  const shimDir = path.join(os.tmpdir(), "weknora-shim-x");
  const env = shimEnv(shimDir);
  const pathKeys = Object.keys(env).filter((key) => key.toUpperCase() === "PATH");
  assert.equal(pathKeys.length, 1, `exactly one PATH variant key must survive: ${pathKeys.join(",")}`);
  const value = env[pathKeys[0]];
  assert.ok(value.startsWith(shimDir + path.delimiter), `shim dir must lead: ${value}`);
  assert.ok(value.includes("/legacy/value"), "the old PATH content must be preserved after the shim dir");
});

test("shimEnv prepends to a normally-cased PATH", (t) => {
  const previous = process.env.PATH;
  t.after(() => { process.env.PATH = previous; });
  process.env.PATH = ["/a", "/b"].join(path.delimiter);
  const shimDir = path.join(os.tmpdir(), "weknora-shim-y");
  const env = shimEnv(shimDir);
  assert.equal(env.PATH, [shimDir, "/a", "/b"].join(path.delimiter));
});

test("shimEntryName carries .exe on win32 only (R1-F27)", () => {
  const expected = process.platform === "win32" ? "node.exe" : "node";
  assert.equal(shimEntryName(), expected);
});

test("findNodeGte26 honors $WEKNORA_NODE_BIN pointing at a >=26 binary", { skip: process.platform === "win32" }, (t) => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "node-gte26-test-"));
  t.after(() => fs.rmSync(dir, { recursive: true, force: true }));
  const fakeNode = path.join(dir, "fake-node-26");
  fs.writeFileSync(fakeNode, `#!/bin/sh\necho "v${MIN_MAJOR}.1.0"\n`, { mode: 0o755 });
  const previous = process.env.WEKNORA_NODE_BIN;
  process.env.WEKNORA_NODE_BIN = fakeNode;
  t.after(() => {
    if (previous === undefined) delete process.env.WEKNORA_NODE_BIN;
    else process.env.WEKNORA_NODE_BIN = previous;
  });
  const hit = findNodeGte26();
  assert.ok(hit, "override candidate must be discovered");
  assert.equal(hit.bin, fakeNode);
  assert.equal(hit.version, `v${MIN_MAJOR}.1.0`);
});

test("findNodeGte26 skips an override candidate below the floor", { skip: process.platform === "win32" }, (t) => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "node-gte26-test-"));
  t.after(() => fs.rmSync(dir, { recursive: true, force: true }));
  const fakeNode = path.join(dir, "fake-node-22");
  fs.writeFileSync(fakeNode, "#!/bin/sh\necho 'v22.11.0'\n", { mode: 0o755 });
  const previous = process.env.WEKNORA_NODE_BIN;
  process.env.WEKNORA_NODE_BIN = fakeNode;
  t.after(() => {
    if (previous === undefined) delete process.env.WEKNORA_NODE_BIN;
    else process.env.WEKNORA_NODE_BIN = previous;
  });
  const hit = findNodeGte26();
  // The v22 override must never be returned: either another >=26 candidate
  // wins (its version reports >= 26) or nothing is found.
  if (hit) assert.ok(majorOf(hit.version) >= MIN_MAJOR, `discovered binary must be >= ${MIN_MAJOR}: ${hit.version}`);
});

test("createNodeShim symlinks the target by default and cleans up", {
  // R1-F29: without developer mode win32 denies real symlinks with EPERM, the
  // copy fallback fires and the realpath assertion below would fail on the
  // copy's (different) path — the symlinked shape is only guaranteed where
  // symlinks actually work.
  skip: process.platform === "win32",
}, (t) => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "node-gte26-shim-"));
  t.after(() => fs.rmSync(dir, { recursive: true, force: true }));
  const target = path.join(dir, "real-node");
  fs.writeFileSync(target, "binary-payload", { mode: 0o755 });
  const { shimDir, shimNode } = createNodeShim(target);
  t.after(() => fs.rmSync(shimDir, { recursive: true, force: true }));
  assert.equal(fs.realpathSync(shimNode), fs.realpathSync(target));
});

test("createNodeShim falls back to copying when symlink is denied (win32 EPERM, T01-OCR1-F12)", (t) => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "node-gte26-shim-"));
  t.after(() => fs.rmSync(dir, { recursive: true, force: true }));
  const target = path.join(dir, "real-node.exe");
  fs.writeFileSync(target, "binary-payload", { mode: 0o755 });
  const eperm = Object.assign(new Error("operation not permitted, symlink 'node' -> '" + target + "'"), {
    code: "EPERM",
  });
  const { shimDir, shimNode } = createNodeShim(target, {
    linker: () => {
      throw eperm;
    },
  });
  t.after(() => fs.rmSync(shimDir, { recursive: true, force: true }));
  const stat = fs.lstatSync(shimNode);
  assert.equal(stat.isFile(), true, "fallback must produce a regular file, not a dangling symlink");
  assert.equal(fs.readFileSync(shimNode, "utf8"), "binary-payload", "copy must be byte-identical to the target");
});

test("spawnExitCode: clean exit passes the child's status through", () => {
  const logs = [];
  assert.equal(spawnExitCode({ status: 0, signal: null, error: undefined }, { log: (m) => logs.push(m) }), 0);
  assert.equal(spawnExitCode({ status: 2, signal: null, error: undefined }, { log: (m) => logs.push(m) }), 2);
  assert.equal(logs.length, 0, "no diagnostics for clean exits");
});

test("spawnExitCode: signal kill exits 1 with a diagnostic (false-green regression, T01-OCR1-F2)", () => {
  const logs = [];
  const code = spawnExitCode({ status: null, signal: "SIGTERM", error: undefined }, { log: (m) => logs.push(m) });
  assert.equal(code, 1, "a child murdered by a signal must NOT exit 0");
  assert.ok(logs.join("\n").includes("SIGTERM"), "the diagnostic must name the signal");
});

test("spawnExitCode: spawn failure exits 1 and surfaces the cause", () => {
  const logs = [];
  const err = new Error("spawn tsx ENOENT");
  const code = spawnExitCode({ status: null, signal: null, error: err }, { log: (m) => logs.push(m) });
  assert.equal(code, 1);
  assert.ok(logs.join("\n").includes("ENOENT"), "the spawn error message must be surfaced");
});
