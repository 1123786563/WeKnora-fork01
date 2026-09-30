import assert from 'node:assert/strict';
import { chmodSync, mkdirSync, mkdtempSync, rmSync, symlinkSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { spawnSync } from 'node:child_process';
import { test } from 'node:test';

const script = join(import.meta.dirname, '../scripts/verify-ios-framework-closure.py');
function fixture(
  loads: Record<string, string>,
  failBinary?: string,
  mode: 'source' | 'precompiled' = 'source',
  options: { architectureHeaders?: boolean; pathPrefixAppLoad?: boolean } = {},
) {
  const root = mkdtempSync(join(tmpdir(), 'ios-framework-closure-'));
  const app = join(root, 'WeKnora.app');
  const frameworks = join(app, 'Frameworks');
  mkdirSync(frameworks, { recursive: true });
  const appBinary = join(app, 'WeKnora');
  writeFileSync(appBinary, 'fixture');
  writeFileSync(join(app, 'Info.plist'), plist('WeKnora'));
  const propertiesPath = join(root, 'Podfile.properties.json');
  writeFileSync(propertiesPath, JSON.stringify({
    'ios.buildReactNativeFromSource': mode === 'source' ? 'true' : 'false',
    EXPO_USE_PRECOMPILED_MODULES: mode === 'source' ? 'false' : 'true',
  }));
  for (const name of ['Alpha', 'Beta']) {
    const dir = join(frameworks, `${name}.framework`);
    mkdirSync(dir);
    writeFileSync(join(dir, name), 'fixture');
    writeFileSync(join(dir, 'Info.plist'), plist(name));
  }
  const bin = join(root, 'bin');
  mkdirSync(bin);
  const fakeOtool = join(bin, 'otool');
  const dependencyRows = `case "$name" in WeKnora) deps='${loads.app ?? ''}' ;; Alpha) deps='${loads.Alpha ?? ''}' ;; Beta) deps='${loads.Beta ?? ''}' ;; esac; [ -z "$deps" ] || printf '    %s\\n' "$deps"${options.pathPrefixAppLoad ? `; if [ "$name" = WeKnora ]; then printf '    %sExtra.framework/Missing\\n' "$2"; fi` : ''}`;
  const headers = options.architectureHeaders
    ? `for arch in arm64 x86_64; do printf '%s (architecture %s):\\n' "$2" "$arch"; ${dependencyRows}; done`
    : `echo "$2:"; ${dependencyRows}`;
  writeFileSync(fakeOtool, `#!/bin/sh\nname=$(basename "$2")\nif [ "$name" = "${failBinary ?? '__none__'}" ]; then echo fake-failure >&2; exit 9; fi\n${headers}\n`);
  chmodSync(fakeOtool, 0o755);
  return { root, app, bin, appBinary, propertiesPath };
}
function plist(executable: string) {
  // XML plist accepted by Python's standard plistlib.
  return `<?xml version="1.0"?><!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd"><plist version="1.0"><dict><key>CFBundleExecutable</key><string>${executable}</string></dict></plist>`;
}
function run(f: ReturnType<typeof fixture>) {
  const result = spawnSync('python3', [script, f.app, f.propertiesPath], { encoding: 'utf8', env: { ...process.env, PATH: `${f.bin}:${process.env.PATH}` } });
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

test('production checker rejects unresolved loader-relative framework dependencies', () => {
  for (const load of [
    '@loader_path/Frameworks/Missing.framework/Missing',
    '@executable_path/Frameworks/Missing.framework/Missing',
  ]) {
    const f = fixture({ app: load });
    try {
      const result = run(f);
      assert.notEqual(result.status, 0, `${load} unexpectedly passed: ${result.output}`);
      assert.match(result.output, /WeKnora\.app requires Missing\.framework/);
    } finally { rmSync(f.root, { recursive: true, force: true }); }
  }
});

test('production checker rejects an executable-relative framework load outside the app', () => {
  const f = fixture({ app: '@executable_path/../Missing.framework/Missing' });
  try {
    const result = run(f);
    assert.notEqual(result.status, 0);
    assert.match(result.output, /FRAMEWORK_LOAD_OUTSIDE_APP/);
  } finally { rmSync(f.root, { recursive: true, force: true }); }
});

test('production checker rejects framework install names without an executable path', () => {
  for (const load of ['@rpath/Missing.framework', '@rpath/Missing.framework/']) {
    const f = fixture({ app: load });
    try {
      const result = run(f);
      assert.notEqual(result.status, 0, `${load} unexpectedly passed: ${result.output}`);
      assert.match(result.output, /MALFORMED_FRAMEWORK_LOAD_PATH: WeKnora\.app.*Missing\.framework/);
    } finally { rmSync(f.root, { recursive: true, force: true }); }
  }
});

test('production checker does not discard dependency paths that share an image path prefix', () => {
  const f = fixture(
    { app: '' },
    undefined,
    'source',
    { architectureHeaders: true, pathPrefixAppLoad: true },
  );
  try {
    const result = run(f);
    assert.notEqual(result.status, 0);
    assert.match(result.output, /UNSUPPORTED_FRAMEWORK_LOAD_PATH: WeKnora\.app/);
  } finally { rmSync(f.root, { recursive: true, force: true }); }
});

test('production checker resolves loader-relative dependencies to embedded executables', () => {
  for (const load of [
    '@loader_path/Frameworks/Alpha.framework/Alpha',
    '@executable_path/Frameworks/Alpha.framework/Alpha',
  ]) {
    const f = fixture({ app: load });
    try {
      const result = run(f);
      assert.equal(result.status, 0, result.output);
      assert.match(result.output, /FRAMEWORK_CLOSURE_OK/);
    } finally { rmSync(f.root, { recursive: true, force: true }); }
  }
});

test('production checker resolves framework-owner loader-relative sibling dependency', () => {
  const f = fixture({ Alpha: '@loader_path/../Beta.framework/Beta' });
  try {
    const result = run(f);
    assert.equal(result.status, 0, result.output);
    assert.match(result.output, /FRAMEWORK_CLOSURE_OK/);
  } finally { rmSync(f.root, { recursive: true, force: true }); }
});

test('production checker fails closed on unsupported non-system framework path tokens', () => {
  for (const load of ['@unknown_path/Foo.framework/Foo', '/private/other/Foo.framework/Foo']) {
    const f = fixture({ app: load });
    try {
      const result = run(f);
      assert.notEqual(result.status, 0);
      assert.match(result.output, /UNSUPPORTED_FRAMEWORK_LOAD_PATH: WeKnora\.app/);
    } finally { rmSync(f.root, { recursive: true, force: true }); }
  }
});

test('production checker ignores system framework paths', () => {
  const f = fixture({ app: '/System/Library/Frameworks/Foundation.framework/Foundation' });
  try {
    const result = run(f);
    assert.equal(result.status, 0, result.output);
    assert.match(result.output, /FRAMEWORK_CLOSURE_OK/);
  } finally { rmSync(f.root, { recursive: true, force: true }); }
});

test('production checker permits only explicit OS roots and rejects unresolved dynamic libraries', () => {
  for (const load of ['@rpath/libMissing.dylib', '@loader_path/libMissing.dylib', '@unknown_path/libMissing.dylib', 'libMissing.dylib', '/opt/vendor/libMissing.dylib']) {
    const f = fixture({ app: load });
    try {
      const result = run(f);
      assert.notEqual(result.status, 0, `${load} unexpectedly passed: ${result.output}`);
      assert.match(result.output, /UNSUPPORTED_DYNAMIC_LIBRARY_LOAD/);
      assert.doesNotMatch(result.output, /Traceback/);
    } finally { rmSync(f.root, { recursive: true, force: true }); }
  }
  const system = fixture({ app: '/usr/lib/libobjc.A.dylib' });
  try {
    const result = run(system);
    assert.equal(result.status, 0, result.output);
    assert.match(result.output, /FRAMEWORK_CLOSURE_OK/);
  } finally { rmSync(system.root, { recursive: true, force: true }); }
});

test('production checker rejects unresolved non-framework dylib loads', () => {
  for (const load of ['@rpath/Foo.framework.dylib', '@rpath/libFoo.dylib']) {
    const f = fixture({ app: load });
    try {
      const result = run(f);
      assert.notEqual(result.status, 0, result.output);
      assert.match(result.output, /UNSUPPORTED_DYNAMIC_LIBRARY_LOAD/);
    } finally { rmSync(f.root, { recursive: true, force: true }); }
  }
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
    symlinkSync(join(f.root, 'outside'), join(f.app, 'Frameworks/Alpha.framework/Alpha'));
    const result = run(f);
    assert.notEqual(result.status, 0);
    assert.match(result.output, /FRAMEWORK_BINARY_OUTSIDE_BUNDLE/);
  } finally { rmSync(f.root, { recursive: true, force: true }); }
});

test('production checker rejects framework executable declared in a sibling bundle', () => {
  const f = fixture({ app: '@rpath/Alpha.framework/Alpha' });
  try {
    writeFileSync(join(f.app, 'Frameworks/Alpha.framework/Info.plist'), plist('../Beta.framework/Alpha'));
    writeFileSync(join(f.app, 'Frameworks/Beta.framework/Alpha'), 'sibling executable');
    const result = run(f);
    assert.notEqual(result.status, 0);
    assert.match(result.output, /FRAMEWORK_BINARY_OUTSIDE_BUNDLE/);
  } finally { rmSync(f.root, { recursive: true, force: true }); }
});

test('production checker rejects a framework-local symlink to a sibling framework executable', () => {
  const f = fixture({ app: '@rpath/Alpha.framework/Alpha' });
  try {
    writeFileSync(join(f.app, 'Frameworks/Beta.framework/Alpha'), 'sibling executable');
    rmSync(join(f.app, 'Frameworks/Alpha.framework/Alpha'));
    symlinkSync('../Beta.framework/Alpha', join(f.app, 'Frameworks/Alpha.framework/Alpha'));
    const result = run(f);
    assert.notEqual(result.status, 0);
    assert.match(result.output, /FRAMEWORK_BINARY_OUTSIDE_BUNDLE/);
  } finally { rmSync(f.root, { recursive: true, force: true }); }
});

test('production checker accepts a versioned framework load path resolving to CFBundleExecutable', () => {
  const f = fixture({ app: '@rpath/Alpha.framework/Versions/A/Alpha' });
  try {
    const versioned = join(f.app, 'Frameworks/Alpha.framework/Versions/A');
    mkdirSync(versioned, { recursive: true });
    writeFileSync(join(versioned, 'Alpha'), 'versioned executable');
    rmSync(join(f.app, 'Frameworks/Alpha.framework/Alpha'));
    symlinkSync('Versions/A/Alpha', join(f.app, 'Frameworks/Alpha.framework/Alpha'));
    const result = run(f);
    assert.equal(result.status, 0, result.output);
    assert.match(result.output, /FRAMEWORK_CLOSURE_OK/);
  } finally { rmSync(f.root, { recursive: true, force: true }); }
});

test('production checker rejects a mismatched requested framework binary path', () => {
  const f = fixture({ app: '@rpath/Alpha.framework/OtherBinary' });
  try {
    const result = run(f);
    assert.notEqual(result.status, 0);
    assert.match(result.output, /MISSING_FRAMEWORK_DEPENDENCY/);
  } finally { rmSync(f.root, { recursive: true, force: true }); }
});

test('production checker reports mode from generated properties and rejects missing or contradictory values', () => {
  const source = fixture({ app: '' });
  try { assert.match(run(source).output, /FRAMEWORK_MODE=source-expo-modules/); }
  finally { rmSync(source.root, { recursive: true, force: true }); }
  const precompiled = fixture({ app: '' }, undefined, 'precompiled');
  try { assert.match(run(precompiled).output, /FRAMEWORK_MODE=precompiled-expo-modules/); }
  finally { rmSync(precompiled.root, { recursive: true, force: true }); }
  const invalid = fixture({ app: '' });
  try {
    writeFileSync(invalid.propertiesPath, JSON.stringify([]));
    const nonObject = run(invalid);
    assert.notEqual(nonObject.status, 0);
    assert.match(nonObject.output, /FRAMEWORK_MODE_PROPERTIES_INVALID/);
    assert.doesNotMatch(nonObject.output, /Traceback/);
    const plistPropertiesPath = invalid.propertiesPath.replace('.json', '.plist');
    writeFileSync(plistPropertiesPath, '<?xml version="1.0"?><plist version="1.0"><array><string>invalid root</string></array></plist>');
    const nonObjectPlist = run({ ...invalid, propertiesPath: plistPropertiesPath });
    assert.notEqual(nonObjectPlist.status, 0);
    assert.match(nonObjectPlist.output, /FRAMEWORK_MODE_PROPERTIES_INVALID/);
    assert.doesNotMatch(nonObjectPlist.output, /Traceback/);
    writeFileSync(invalid.propertiesPath, JSON.stringify({ 'ios.buildReactNativeFromSource': 'false', EXPO_USE_PRECOMPILED_MODULES: 'false' }));
    assert.match(run(invalid).output, /FRAMEWORK_MODE_PROPERTIES_CONTRADICTORY/);
    writeFileSync(invalid.propertiesPath, JSON.stringify({}));
    assert.match(run(invalid).output, /FRAMEWORK_MODE_PROPERTIES_INVALID/);
    const worklets = join(invalid.app, 'Frameworks/ExpoModulesWorklets.framework');
    mkdirSync(worklets);
    writeFileSync(join(worklets, 'ExpoModulesWorklets'), 'binary');
    writeFileSync(join(worklets, 'Info.plist'), plist('ExpoModulesWorklets'));
    writeFileSync(invalid.propertiesPath, JSON.stringify({ 'ios.buildReactNativeFromSource': 'true', EXPO_USE_PRECOMPILED_MODULES: 'false' }));
    assert.match(run(invalid).output, /FRAMEWORK_MODE_MISMATCH/);
  } finally { rmSync(invalid.root, { recursive: true, force: true }); }
});

test('production checker fails closed when otool exits unsuccessfully', () => {
  const f = fixture({ app: '', Alpha: '' }, 'WeKnora');
  try {
    const result = run(f);
    assert.notEqual(result.status, 0);
    assert.match(result.output, /FRAMEWORK_INSPECTION_FAILED: .*WeKnora/);
  } finally { rmSync(f.root, { recursive: true, force: true }); }
});
