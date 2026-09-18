#!/usr/bin/env node
/**
 * WeKnora commit-integrity self-check (R460 audit batch 2).
 *
 * Background: three missed-commit incidents (R442 chunkingSamples.ts, R444
 * embed/markdown.ts, R451 i18n two files) all had the same shape -- local
 * tests read the working tree and passed, while a fresh HEAD checkout broke
 * because a dependency file was never committed. This script guards against
 * the three accident classes surfaced by the R453 audit:
 *
 *   [1/3] (P0)   Relative-import resolution: every relative import in tracked
 *                apps/<app>/src and packages/<pkg>/src .ts/.tsx files (vendored
 *                packages/happy-wire excluded) must resolve against the git
 *                index. A target that exists only in the working tree is
 *                exactly the "forgot to git add" incident; a target that
 *                resolves nowhere is a plain broken import.
 *   [2/3] (WARN) Test-glob coverage: every *.test.{ts,tsx} under packages/<pkg>/src,
 *                packages/<pkg>/test and apps/web/src must be picked up by the root
 *                test:shared glob set or by its own package's test script,
 *                otherwise local green does not imply CI green.
 *   [3/3] (INFO) Dirty shared files: modified-but-uncommitted packages/<...>
 *                entries are listed so the committer does not push a partial
 *                shared-package change.
 *
 * Safety constraints (hook environment): local filesystem + git only.
 * No network requests. No credentials. No writes; output goes to stdout/stderr.
 *
 * Usage:
 *   node scripts/check-commit-integrity.mjs [--root <dir>]
 * Exit code: 0 = no P0 findings (warnings/info allowed); 1 = P0 findings.
 */

import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import pathPosix from 'node:path/posix';
import { pathToFileURL } from 'node:url';
import { globSync } from 'node:fs';

const VENDORED = 'packages/happy-wire';

const SOURCE_FILE_RE = /^apps\/[^/]+\/src\/.+\.(ts|tsx)$|^packages\/[^/]+\/src\/.+\.(ts|tsx)$/;
const TEST_FILE_SCOPE_RE =
  /^(?:packages\/(?!happy-wire)[^/]+\/(?:src|test)\/|apps\/web\/src\/).+\.test\.(ts|tsx)$/;

// Extensions we try when a specifier has no recognizable file extension.
const RESOLVE_EXTENSIONS = ['.ts', '.tsx', '.d.ts', '.js', '.jsx', '.mjs', '.cjs', '.json'];
const KNOWN_ASSET_EXT_RE =
  /\.(ts|tsx|d\.ts|js|jsx|mjs|cjs|json|css|scss|sass|less|svg|png|jpe?g|webp|gif|avif|ico|woff2?|ttf|otf|eot|mp3|mp4|webm|wasm|ya?ml|txt|md|html)$/i;

/* ------------------------------------------------------------------ */
/* Pure helpers (exported for node:test)                               */
/* ------------------------------------------------------------------ */

/** Extract relative imports (./ or ../) from TS source text, with line numbers. */
export function collectRelativeImports(content) {
  const found = [];
  const lines = content.split(/\r?\n/);
  const importRe =
    /(?:\bfrom\s*|\brequire\s*\(\s*|\bimport\s*\(\s*|\bimport\s*)(['"])(\.[^'"]*)\1/g;
  lines.forEach((line, idx) => {
    const trimmed = line.trimStart();
    if (trimmed.startsWith('//') || trimmed.startsWith('*') || trimmed.startsWith('/*')) {
      return;
    }
    importRe.lastIndex = 0;
    let m;
    while ((m = importRe.exec(line)) !== null) {
      found.push({ specifier: m[2], line: idx + 1 });
    }
  });
  return found;
}

/** Candidate repo-relative paths a relative specifier could resolve to. */
export function resolveImportCandidates(fromFile, specifier) {
  const dir = pathPosix.dirname(fromFile);
  const base = pathPosix.normalize(pathPosix.join(dir, specifier));
  const candidates = [base];
  if (!KNOWN_ASSET_EXT_RE.test(base)) {
    for (const ext of RESOLVE_EXTENSIONS) candidates.push(base + ext);
    for (const ext of ['.ts', '.tsx', '.js', '.jsx']) {
      candidates.push(`${base}/index${ext}`);
    }
  }
  return candidates;
}

/**
 * Check 1: every relative import of every source file must resolve against
 * the tracked file set (the git index).
 *
 * @param {object} opts
 * @param {Array<{path: string, content: string}>} opts.sourceFiles
 * @param {Set<string>} opts.trackedSet repo-relative paths in the git index
 * @param {string} opts.root absolute repo root (for on-disk existence probes)
 * @param {(p: string) => boolean} [opts.isIgnored] gitignore predicate; an
 *   ignored on-disk target is a warning (likely generated), not a P0.
 * @returns {{ findings: Array<object>, importCount: number, fileCount: number }}
 */
export function checkImports({ sourceFiles, trackedSet, root, isIgnored = () => false }) {
  const findings = [];
  let importCount = 0;
  for (const file of sourceFiles) {
    if (!trackedSet.has(file.path)) continue; // only tracked files ship in HEAD
    for (const { specifier, line } of collectRelativeImports(file.content)) {
      importCount += 1;
      const candidates = resolveImportCandidates(file.path, specifier);
      if (candidates.some((c) => trackedSet.has(c))) continue;
      const onDisk = candidates.filter((c) => fs.existsSync(path.join(root, c)));
      if (onDisk.length > 0) {
        if (onDisk.every((c) => c.includes('/node_modules/'))) {
          findings.push({
            severity: 'WARN',
            kind: 'import-target-node-modules',
            file: file.path,
            line,
            specifier,
            detail: `resolves into ${onDisk[0]} (dependency install path, not committed; fragile cross-package import)`,
          });
        } else {
          const ignored = onDisk.every((c) => isIgnored(c));
          findings.push({
            severity: ignored ? 'WARN' : 'P0',
            kind: ignored ? 'import-target-ignored' : 'import-target-not-in-index',
            file: file.path,
            line,
            specifier,
            detail: `resolves on disk to ${onDisk[0]} which is ${
              ignored ? 'gitignored (generated?)' : 'NOT in the git index (git add it!)'
            }`,
          });
        }
      } else {
        findings.push({
          severity: 'P0',
          kind: 'import-unresolved',
          file: file.path,
          line,
          specifier,
          detail: 'no candidate target found in index or on disk',
        });
      }
    }
  }
  return { findings, importCount, fileCount: sourceFiles.filter((f) => trackedSet.has(f.path)).length };
}

/** Pull every glob-looking token (quoted or bare) out of a package.json script. */
export function extractGlobsFromScript(script) {
  if (!script) return [];
  const out = new Set();
  for (const m of script.matchAll(/'([^']*\*[^']*)'|"([^"]*\*[^"]*)"/g)) {
    out.add(m[1] ?? m[2]);
  }
  const withoutQuoted = script.replace(/'[^']*'|"[^"]*"/g, ' ');
  for (const tok of withoutQuoted.split(/\s+/)) {
    if (tok.includes('*')) out.add(tok);
  }
  return [...out];
}

/**
 * Check 2: every in-scope test file must be matched by the root test:shared
 * glob set or by its owning package's test script (expanded via fs.globSync).
 *
 * @param {object} opts
 * @param {string[]} opts.testFiles repo-relative *.test.{ts,tsx} paths
 * @param {string} opts.rootScript root package.json "test:shared" script
 * @param {Array<{dir: string, script: string}>} opts.packageScripts
 *   e.g. { dir: 'apps/web', script: "node --import tsx --test 'src/...'" }
 *   (quoted or bare glob tokens inside the script are expanded relative to dir)
 * @param {string} opts.root absolute repo root
 * @returns {{ uncovered: string[], covered: string[], expansionErrors: string[] }}
 */
export function checkTestCoverage({ testFiles, rootScript, packageScripts, root }) {
  const covered = new Set();
  const expansionErrors = [];
  const addAll = (pattern, baseDir) => {
    try {
      for (const hit of globSync(pattern, { cwd: path.join(root, baseDir) })) {
        covered.add(pathPosix.join(baseDir, hit));
      }
    } catch (err) {
      expansionErrors.push(`${baseDir}: ${pattern} (${err.message})`);
    }
  };
  for (const pattern of extractGlobsFromScript(rootScript)) addAll(pattern, '.');
  for (const { dir, script } of packageScripts ?? []) {
    for (const pattern of extractGlobsFromScript(script)) addAll(pattern, dir);
  }
  const uncovered = testFiles.filter((f) => !covered.has(f));
  return { uncovered, covered: testFiles.filter((f) => covered.has(f)), expansionErrors };
}

/**
 * Check 3: list modified (M in either column) packages/** entries from
 * `git status --porcelain` output text.
 */
export function parseDirtySharedFiles(statusPorcelain) {
  const out = [];
  for (const line of String(statusPorcelain).split('\n')) {
    if (line.length < 4) continue;
    const xy = line.slice(0, 2);
    let p = line.slice(3);
    if (p.includes(' -> ')) p = p.split(' -> ').pop();
    p = p.replace(/^"(.*)"$/, '$1');
    if (xy.includes('M') && p.startsWith('packages/')) {
      out.push({ status: xy.trim(), path: p });
    }
  }
  return out;
}

/* ------------------------------------------------------------------ */
/* Runner                                                              */
/* ------------------------------------------------------------------ */

function execGit(root, args) {
  return execFileSync('git', args, { cwd: root, encoding: 'utf8', maxBuffer: 256 * 1024 * 1024 });
}

function readPackageJson(root, repoRelative) {
  const abs = path.join(root, repoRelative, 'package.json');
  try {
    return JSON.parse(fs.readFileSync(abs, 'utf8'));
  } catch {
    return null;
  }
}

export function parseArgs(argv) {
  const out = { root: process.cwd() };
  for (let i = 0; i < argv.length; i += 1) {
    if (argv[i] === '--root' && argv[i + 1]) {
      out.root = path.resolve(argv[i + 1]);
      i += 1;
    }
  }
  return out;
}

export function main({ root = process.cwd(), out = process.stdout, err = process.stderr } = {}) {
  const write = (s) => out.write(`${s}\n`);
  const writeErr = (s) => err.write(`${s}\n`);
  write('== check-commit-integrity ==');

  // ---- collect repository facts -------------------------------------
  const tracked = execGit(root, ['ls-files']).split('\n').filter(Boolean);
  const trackedSet = new Set(tracked);
  const statusText = (() => {
    try {
      return execGit(root, ['status', '--porcelain']);
    } catch {
      return '';
    }
  })();
  const ignoredSet = (() => {
    // Single batched check-ignore call; empty input returns exit 1, so guard.
    const paths = tracked.slice();
    const untracked = statusText
      .split('\n')
      .filter((l) => l.startsWith('??'))
      .map((l) => l.slice(3).split(' -> ').pop().replace(/^"(.*)"$/, '$1'));
    const probe = [...new Set([...paths, ...untracked])].filter((p) =>
      fs.existsSync(path.join(root, p)),
    );
    if (probe.length === 0) return new Set();
    try {
      const res = execFileSync(
        'git',
        ['check-ignore', '--stdin'],
        { cwd: root, input: `${probe.join('\n')}\n`, encoding: 'utf8', maxBuffer: 256 * 1024 * 1024 },
      );
      return new Set(res.split('\n').filter(Boolean));
    } catch {
      return new Set();
    }
  })();
  const isIgnored = (p) => {
    if (ignoredSet.has(p)) return true;
    // Findings may reference arbitrary paths; ask git directly for those.
    try {
      execFileSync('git', ['check-ignore', '-q', '--no-index', p], { cwd: root, stdio: 'ignore' });
      return true; // exit 0 => ignored
    } catch (err) {
      return err.status === 1 ? false : false; // 1 = not ignored; 128 etc. = unanswerable
    }
  };

  const sourceFiles = tracked
    .filter((p) => SOURCE_FILE_RE.test(p) && !p.startsWith(`${VENDORED}/`))
    .map((p) => {
      try {
        return { path: p, content: fs.readFileSync(path.join(root, p), 'utf8') };
      } catch {
        return { path: p, content: '', unreadable: true };
      }
    });

  const p0 = [];
  const warnings = [];
  const infos = [];

  // ---- check 1: relative imports vs git index -----------------------
  write(`[1/3] relative imports vs git index (${sourceFiles.length} files; ${VENDORED} excluded)`);
  const importResult = checkImports({ sourceFiles, trackedSet, root, isIgnored });
  for (const file of sourceFiles) {
    if (file.unreadable) {
      warnings.push(`tracked file missing in working tree: ${file.path}`);
    }
  }
  if (importResult.findings.length === 0) {
    write(`  OK   all ${importResult.importCount} relative imports resolve inside the git index`);
  } else {
    for (const f of importResult.findings) {
      const line = `  ${f.severity}  ${f.file}:${f.line} import '${f.specifier}' -> ${f.detail}`;
      writeErr(line);
      if (f.severity === 'P0') p0.push(line.trim());
      else warnings.push(line.trim());
    }
  }

  // ---- check 2: test glob coverage ----------------------------------
  write('[2/3] test glob coverage (packages/*/src, packages/*/test, apps/web/src)');
  const rootPkg = readPackageJson(root, '.');
  const rootScript = rootPkg?.scripts?.['test:shared'] ?? '';
  const testFiles = tracked.filter((p) => TEST_FILE_SCOPE_RE.test(p));
  const packageDirs = new Set(testFiles.map((p) => p.split('/').slice(0, 2).join('/')));
  const packageScripts = [...packageDirs]
    .sort()
    .map((dir) => ({ dir, script: readPackageJson(root, dir)?.scripts?.test ?? '' }));
  const cov = checkTestCoverage({ testFiles, rootScript, packageScripts, root });
  if (cov.expansionErrors.length > 0) {
    for (const e of cov.expansionErrors) warnings.push(`glob expansion failed: ${e}`);
  }
  if (cov.uncovered.length === 0) {
    write(`  OK   all ${cov.covered.length} test files matched by test:shared or package test scripts`);
  } else {
    for (const f of cov.uncovered) {
      const line = `  WARN ${f} is not matched by test:shared nor its package test script (local green != CI green)`;
      writeErr(line);
      warnings.push(line.trim());
    }
  }

  // ---- check 3: dirty shared files ----------------------------------
  write('[3/3] dirty shared files (git status, packages/**)');
  const dirty = parseDirtySharedFiles(statusText);
  if (dirty.length === 0) {
    write('  OK   no modified packages/** files pending commit');
  } else {
    for (const d of dirty) {
      const line = `  INFO ${d.path} (${d.status}) -- modified but not committed, do not forget it`;
      write(line);
      infos.push(line.trim());
    }
  }

  // ---- summary -------------------------------------------------------
  write(
    `Summary: ${p0.length} P0, ${warnings.length} WARN, ${infos.length} INFO -- ${
      p0.length > 0 ? 'FAIL (fix P0 before push)' : 'PASS (P0 clean)'
    }`,
  );
  if (p0.length > 0) {
    writeErr('P0 findings present: imports would break on a fresh HEAD checkout.');
    return 1;
  }
  return 0;
}

const isDirectRun =
  process.argv[1] && import.meta.url === pathToFileURL(path.resolve(process.argv[1])).href;
if (isDirectRun) {
  const { root } = parseArgs(process.argv.slice(2));
  let code = 0;
  try {
    code = main({ root });
  } catch (e) {
    process.stderr.write(`check-commit-integrity: fatal: ${e.message}\n`);
    code = 1;
  }
  process.exit(code);
}
