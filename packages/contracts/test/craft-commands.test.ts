// CFT-S00-T002 contract tests: the craft command ports freeze the mapping
// between the design's internal DraftIntent/RestoreIntent/Decision vocabulary
// (docs/design/weknora-craft-hifi/examples/command-bridge.types.ts) and the
// VERIFIED existing HTTP DTOs:
//
//   POST /sessions/:id/craft/runs     {request_id, prompt, input_refs,
//                                      knowledge_scope, base_version_id}
//   POST /sessions/:id/craft/restore  {request_id, snapshot_id, revision}
//   POST /sessions/:id/craft/interactions/:iid/decide
//                                    {decision_id, action, args_hash,
//                                     expected_revision, answers|answer, reason}
//
// These tests assert the frozen mapping itself: validation rejects a missing
// request id or an illegal revision, a validated intent survives into the wire
// body field-for-field, and client-supplied authority facts (sandbox URL,
// tenant/user identity) are rejected — identity travels only via the
// authenticated transport.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
  CRAFT_DECISION_ACTIONS,
  validateCraftDecisionCommand,
  validateCraftRestoreCommand,
  validateCraftSubmitDraftCommand,
  craftDraftWireBody,
  craftRestoreWireBody,
  craftDecisionWireBody,
  type CraftSubmitDraftCommand,
} from '../src/craft/command-ports.ts';

const draft = (over: Partial<CraftSubmitDraftCommand> = {}): CraftSubmitDraftCommand => ({
  request_id: 'req-1',
  session_id: 'ses-1',
  prompt: '  按月分析销售数据  ',
  kind: 'web',
  input_refs: ['resource://abc', 'resource://def'],
  knowledge_scope: 'kb/physics,kb/math',
  base_version_id: 'ver-9',
  expected_workspace_revision: 3,
  ...over,
});

test('a valid draft keeps inputs, knowledge and base version field-for-field', () => {
  const cmd = validateCraftSubmitDraftCommand(draft());
  assert.equal(cmd.prompt, '按月分析销售数据'); // trimmed, not silently re-frozen
  assert.deepEqual(cmd.input_refs, ['resource://abc', 'resource://def']);
  assert.equal(cmd.knowledge_scope, 'kb/physics,kb/math');
  assert.equal(cmd.base_version_id, 'ver-9');
  const body = craftDraftWireBody(cmd);
  // Frozen mapping: prompt->prompt (not text), inputs->input_refs,
  // knowledge->knowledge_scope, base version->base_version_id.
  assert.deepEqual(body, {
    request_id: 'req-1',
    prompt: '按月分析销售数据',
    input_refs: ['resource://abc', 'resource://def'],
    knowledge_scope: 'kb/physics,kb/math',
    base_version_id: 'ver-9',
  });
  // The wire body must never leak the target-only CAS field (D003) nor any
  // client authority fact.
  assert.deepEqual(Object.keys(body).sort(), ['base_version_id', 'input_refs', 'knowledge_scope', 'prompt', 'request_id']);
});

test('missing or blank request id is rejected for every command', () => {
  assert.throws(() => validateCraftSubmitDraftCommand(draft({ request_id: '' })), /request_id/);
  assert.throws(() => validateCraftSubmitDraftCommand(draft({ request_id: '  ' })), /request_id/);
  assert.throws(
    () => validateCraftRestoreCommand({ request_id: '', snapshot_id: 's-1', revision: 3 }),
    /request_id/,
  );
  assert.throws(
    () =>
      validateCraftDecisionCommand({
        request_id: '',
        session_id: 'ses-1',
        interaction_id: 'i-1',
        action: 'allow_once',
        expected_revision: 3,
      }),
    /request_id/,
  );
});

test('illegal revisions are rejected', () => {
  // draft: the CAS field is optional but must be a positive integer when present
  assert.throws(() => validateCraftSubmitDraftCommand(draft({ expected_workspace_revision: 0 })), /expected_workspace_revision/);
  assert.throws(() => validateCraftSubmitDraftCommand(draft({ expected_workspace_revision: -2 })), /expected_workspace_revision/);
  assert.throws(() => validateCraftSubmitDraftCommand(draft({ expected_workspace_revision: 1.5 })), /expected_workspace_revision/);
  assert.equal(validateCraftSubmitDraftCommand(draft({ expected_workspace_revision: undefined as unknown as number })).expected_workspace_revision, null);
  // restore: revision is mandatory positive CAS
  assert.throws(() => validateCraftRestoreCommand({ request_id: 'r', session_id: 'ses-1', snapshot_id: 's', revision: 0 }), /revision/);
  assert.throws(() => validateCraftRestoreCommand({ request_id: 'r', session_id: 'ses-1', snapshot_id: 's', revision: -1 }), /revision/);
  // decision: expected_revision is mandatory positive CAS
  assert.throws(
    () => validateCraftDecisionCommand({ request_id: 'r', session_id: 'ses-1', interaction_id: 'i', action: 'allow_once', expected_revision: 0 }),
    /expected_revision/,
  );
});

test('blank prompts and unknown kinds are rejected on the draft', () => {
  assert.throws(() => validateCraftSubmitDraftCommand(draft({ prompt: '   ' })), /prompt/);
  assert.throws(() => validateCraftSubmitDraftCommand(draft({ kind: 'website' as CraftSubmitDraftCommand['kind'] })), /kind/);
});

test('client-supplied authority facts are rejected on every command', () => {
  assert.throws(() => validateCraftSubmitDraftCommand(draft({ sandbox_url: 'http://evil' } as Partial<CraftSubmitDraftCommand>)), /sandbox_url/);
  assert.throws(() => validateCraftSubmitDraftCommand(draft({ tenant_id: '2' } as Partial<CraftSubmitDraftCommand>)), /tenant_id/);
  assert.throws(() => validateCraftSubmitDraftCommand(draft({ user_id: 'attacker' } as Partial<CraftSubmitDraftCommand>)), /user_id/);
  assert.throws(
    () => validateCraftRestoreCommand({ request_id: 'r', snapshot_id: 's', revision: 3, tenant_id: '2' } as never),
    /tenant_id/,
  );
  assert.throws(
    () =>
      validateCraftDecisionCommand({
        request_id: 'r',
        session_id: 'ses-1',
        interaction_id: 'i',
        action: 'allow_once',
        expected_revision: 3,
        sandbox_url: 'http://evil',
      } as never),
    /sandbox_url/,
  );
});

test('decision commands freeze action, answers and the wire mapping', () => {
  const permission = validateCraftDecisionCommand({
    request_id: 'r',
    session_id: 'ses-1',
    interaction_id: 'i-1',
    action: 'allow_once',
    args_hash: 'h-1',
    expected_revision: 4,
  });
  assert.deepEqual(craftDecisionWireBody(permission), {
    decision_id: '',
    action: 'allow_once',
    args_hash: 'h-1',
    expected_revision: 4,
    answers: [],
  });
  const answer = validateCraftDecisionCommand({
    request_id: 'r',
    session_id: 'ses-1',
    interaction_id: 'i-1',
    action: 'answer',
    expected_revision: 4,
    answers: [{ question_id: 'q1', choices: ['a'], text: '说明' }],
  });
  assert.deepEqual(craftDecisionWireBody(answer).answers, [{ question_id: 'q1', choices: ['a'], text: '说明' }]);
  assert.throws(
    () => validateCraftDecisionCommand({ request_id: 'r', session_id: 'ses-1', interaction_id: 'i', action: 'approve-all', expected_revision: 4 }),
    /action/,
  );
  // allow_once is the only granting action in the frozen vocabulary (CRAFT_DECISION_ACTIONS)
  assert.deepEqual(CRAFT_DECISION_ACTIONS, ['allow_once', 'deny', 'answer']);
});

test('restore wire body maps snapshot id and revision verbatim', () => {
  const cmd = validateCraftRestoreCommand({ request_id: 'r-1', session_id: 'ses-1', snapshot_id: 'snap-7', revision: 5 });
  assert.deepEqual(craftRestoreWireBody(cmd), { request_id: 'r-1', snapshot_id: 'snap-7', revision: 5 });
  assert.throws(() => validateCraftRestoreCommand({ request_id: 'r-1', session_id: 'ses-1', revision: 5 } as never), /snapshot_id/);
});
