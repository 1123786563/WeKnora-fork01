// CFT-S00-T005: view capabilities are a PROJECTION of server-owned facts
// (WEKNORA_CRAFT_ENABLED zero-off, WEKNORA_CRAFT_KINDS per-kind opening,
// session write permission, upload ceiling). The pure rules below decide the
// full matrix; hiding a button is never the security boundary (the server
// re-validates — CraftFeatureGate.Allows).
import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
  craftViewCapabilities,
  craftKindDisabledReason,
  CRAFT_CAPABILITY_REASONS,
  type CraftCapabilityInput,
} from './capabilities.ts';

const base: CraftCapabilityInput = {
  gateEnabled: true,
  allowedKinds: ['web'],
  canWrite: true,
  maxInputBytes: 20 * 1024 * 1024,
};

test('gate off (zero value) closes every submit entrance with one reason', () => {
  const caps = craftViewCapabilities({ ...base, gateEnabled: false });
  assert.equal(caps.canWrite, false);
  assert.equal(caps.canRead, true); // history stays viewable — see next test
  assert.deepEqual(caps.allowedKinds, []);
  assert.equal(caps.disabledReason, CRAFT_CAPABILITY_REASONS.gateOff);
});

test('read survives without write: history is viewable while submits close', () => {
  const caps = craftViewCapabilities({ ...base, canWrite: false });
  assert.equal(caps.canRead, true);
  assert.equal(caps.canWrite, false);
  assert.equal(caps.disabledReason, CRAFT_CAPABILITY_REASONS.readOnly);
});

test('a kind outside the server list is disabled with a readable reason, never silently', () => {
  const caps = craftViewCapabilities(base);
  assert.equal(craftKindDisabledReason('web', caps), null);
  const reason = craftKindDisabledReason('document', caps);
  assert.ok(reason && reason.length > 0, 'closed kind must carry an explicit reason');
  // and the draft survives: disabling projects a reason, it never clears the goal
  assert.equal(caps.allowedKinds.includes('document'), false);
});

test('the full gate x permission x kind matrix stays consistent', () => {
  for (const gateEnabled of [false, true]) {
    for (const canWrite of [false, true]) {
      for (const allowedKinds of [[], ['web'], ['web', 'document', 'spreadsheet', 'slides']] as string[][]) {
        const caps = craftViewCapabilities({ ...base, gateEnabled, canWrite, allowedKinds });
        // invariant 1: write requires BOTH an open gate and write permission
        assert.equal(caps.canWrite, gateEnabled && canWrite, `gate=${gateEnabled} write=${canWrite}`);
        // invariant 2: reading history never depends on the gate or write
        assert.equal(caps.canRead, true);
        // invariant 3: kinds mirror the server list exactly when open, empty when closed
        assert.deepEqual(caps.allowedKinds, gateEnabled ? allowedKinds : []);
        // invariant 4: a disabled surface always carries exactly one reason
        const disabled = !caps.canWrite;
        assert.equal(disabled, caps.disabledReason !== null);
      }
    }
  }
});

test('upload ceiling passes through verbatim; unknown stays unknown (never zero)', () => {
  assert.equal(craftViewCapabilities(base).maxInputBytes, 20 * 1024 * 1024);
  const unknown = craftViewCapabilities({ ...base, maxInputBytes: null });
  assert.equal(unknown.maxInputBytes, null, 'an unknown limit must not be fabricated as 0');
});
