import assert from 'node:assert/strict';
import { chmodSync, mkdirSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { spawnSync } from 'node:child_process';
import { test } from 'node:test';

const script = join(import.meta.dirname, '../scripts/verify-ios-framework-closure.py');
function fixture(loads: Record<string, string>, failBinary?: string) {
  const root = mkdtempSync(join(tmpdir(), 'ios-framework-closure-'));
  const app = join(root, 'WeKnora.app');
  const frameworks = join(app, 'Frameworks');
  mkdirSync(frameworks, { recursive: true });
  const appBinary = join(app, 'WeKnora');
  writeFileSync(appBinary, 'fixture');
  writeFileSync(join(app, 'Info.plist'), plist('WeKnora'));
  for (const name of ['Alpha', 'Beta']) {
    const dir = join(frameworks, `${name}.framework`);
    mkdirSync(dir);
    writeFileSync(join(dir, name), 'fixture');
    writeFileSync(join(dir, 'Info.plist'), plist(name));
  }
  const bin = join(root, 'bin');
  mkdirSync(bin);
  const fakeOtool = join(bin, 'otool');
  writeFileSync(fakeOtool, `#!/bin/sh\nname=$(basename "$2")\nif [ "$name" = "${failBinary ?? '__none__'}" ]; then echo fake-failure >&2; exit 9; fi\necho "$2:"\ncase "$name" in WeKnora) deps='${loads.app ?? ''}' ;; Alpha) deps='${loads.Alpha ?? ''}' ;; Beta) deps='${loads.Beta ?? ''}' ;; esac\n[ -z "$deps" ] || printf '    %s\\n' "$deps"\n`);
  chmodSync(fakeOtool, 0o755);
  return { root, app, bin, appBinary };
}
function plist(executable: string) {
  // XML plist accepted by Python's standard plistlib.
  return `<?xml version="1.0"?><!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd"><plist version="1.0"><dict><key>CFBundleExecutable</key><string>${executable}</string></dict></plist>`;
}
function run(f: ReturnType<typeof fixture>) {
  const result = spawnSync('python3', [script, f.app], { encoding: 'utf8', env: { ...process.env, PATH: `${f.bin}:${process.env.PATH}` } });
  return { ...result, output: `${result.stdout}${result.stderr}` };
}

test('production checker accepts complete app and framework dependency closure', () => {
  const f = fixture({ app: '@rpath/Alpha.framework/Alpha', Alpha: '@rpath/Beta.framework/Beta' });
  try {
    const result = run(f);
    assert.equal(result.status, 0, result.output);
    assert.match(result.output, /FRAMEWORK_CLOSURE_OK/);
  } finally { rmSync(f.root, { recursive: true, force: true }); }
});

test('production checker rejects a missing direct app executable dependency', () => {
  const f = fixture({ app: '@rpath/Missing.framework/Missing' });
  try {
    const result = run(f);
    assert.notEqual(result.status, 0);
    assert.match(result.output, /WeKnora\.app requires Missing\.framework/);
  } finally { rmSync(f.root, { recursive: true, force: true }); }
});

test('production checker rejects missing framework executable despite framework directory presence', () => {
  const f = fixture({ app: '@rpath/Alpha.framework/Alpha' });
  try {
    rmSync(join(f.app, 'Frameworks/Alpha.framework/Alpha'));
    const result = run(f);
    assert.notEqual(result.status, 0);
    assert.match(result.output, /FRAMEWORK_BINARY_MISSING: Alpha\.framework/);
  } finally { rmSync(f.root, { recursive: true, force: true }); }
});

test('production checker rejects framework executable symlinks escaping the app', () => {
  const f = fixture({ app: '@rpath/Alpha.framework/Alpha' });
  try {
    rmSync(join(f.app, 'Frameworks/Alpha.framework/Alpha'));
    writeFileSync(join(f.root, 'outside'), 'not inside app');
    require('node:fs').symlinkSync(join(f.root, 'outside'), join(f.app, 'Frameworks/Alpha.framework/Alpha'));
    const result = run(f);
    assert.notEqual(result.status, 0);
    assert.match(result.output, /FRAMEWORK_BINARY_OUTSIDE_APP/);
  } finally { rmSync(f.root, { recursive: true, force: true }); }
});

test('production checker fails closed when otool exits unsuccessfully', () => {
  const f = fixture({ app: '', Alpha: '' }, 'WeKnora');
  try {
    const result = run(f);
    assert.notEqual(result.status, 0);
    assert.match(result.output, /FRAMEWORK_INSPECTION_FAILED: .*WeKnora/);
  } finally { rmSync(f.root, { recursive: true, force: true }); }
});
