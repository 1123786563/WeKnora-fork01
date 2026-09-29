import assert from 'node:assert/strict';
import { test } from 'node:test';
import { findMissingFrameworkDependencies } from './ios-framework-closure.js';

test('framework closure rejects ExpoModulesWorklets when React.framework is missing', () => {
  assert.deepEqual(
    findMissingFrameworkDependencies(
      [{ framework: 'ExpoModulesWorklets.framework', loadCommands: ['@rpath/React.framework/React', '@rpath/hermesvm.framework/hermesvm'] }],
      new Set(['ExpoModulesWorklets.framework', 'hermesvm.framework']),
    ),
    ['React.framework'],
  );
});

test('framework closure accepts complete embedded dependencies and ignores system loads', () => {
  assert.deepEqual(
    findMissingFrameworkDependencies(
      [{ framework: 'ExpoModulesWorklets.framework', loadCommands: ['@rpath/React.framework/React', '/System/Library/Frameworks/Foundation.framework/Foundation'] }],
      new Set(['ExpoModulesWorklets.framework', 'React.framework']),
    ),
    [],
  );
});

test('source-built Expo mode rejects a precompiled Worklets framework', () => {
  const sourceModeFrameworks = new Set(['ExpoModulesJSI.framework', 'hermesvm.framework']);
  assert.equal(sourceModeFrameworks.has('ExpoModulesWorklets.framework'), false);
  assert.equal(sourceModeFrameworks.has('React.framework'), false);
});

test('precompiled Expo mode requires React.framework for ExpoModulesWorklets', () => {
  assert.deepEqual(
    findMissingFrameworkDependencies(
      [{ framework: 'ExpoModulesWorklets.framework', loadCommands: ['@rpath/React.framework/React'] }],
      new Set(['ExpoModulesWorklets.framework']),
    ),
    ['React.framework'],
  );
});
