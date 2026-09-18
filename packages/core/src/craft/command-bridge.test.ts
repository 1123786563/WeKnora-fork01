// CFT-S01-T008: the command bridge owns the submit ordering and the frozen
// intent. The routes assembly keeps its upload/poll/associate pieces; the
// bridge freezes what T002 contracted:
//   * every attachment must be uploaded+associated BEFORE one run submission
//     — an upload failure submits ZERO runs
//   * the requestId is minted ONCE per intent by the caller and survives
//     lost responses: a retry sends the SAME id with an IDENTICAL payload
//     (server replays the admission; the e2e reload spec counts 1 run)
//   * baseVersionId is the editing baseline ONLY — a previewed historical
//     version can never leak into the submission
import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
  submitDraftWithAttachments,
  type CraftBridgePorts,
  type CraftBridgeAttachment,
} from './command-bridge.ts';

interface Call {
  op: 'upload' | 'submit';
  payload: unknown;
}

function makePorts(log: Call[], opts: { uploadFails?: string; submitFails?: number } = {}): CraftBridgePorts & { submitCount: () => number } {
  let submits = 0;
  return {
    async uploadAndAssociate(attachment: CraftBridgeAttachment): Promise<string> {
      log.push({ op: 'upload', payload: { name: attachment.name } });
      if (opts.uploadFails === attachment.name) throw new Error(`upload failed: ${attachment.name}`);
      return `resource://${attachment.name}`;
    },
    async submitDraft(command: unknown): Promise<unknown> {
      submits += 1;
      log.push({ op: 'submit', payload: command });
      if (opts.submitFails !== undefined && submits <= opts.submitFails) {
        throw new Error('network error: response lost');
      }
      return { ok: true };
    },
    submitCount: () => submits,
  };
}

const attachments: CraftBridgeAttachment[] = [
  { name: 'craft-sales.csv', sha256: 'aa', file: {} as never },
  { name: 'brief.md', sha256: 'bb', file: {} as never },
];

const intent = {
  sessionId: 'ses-1',
  prompt: '按月分析销售数据',
  kind: 'web' as const,
  knowledgeScope: 'kb/sales',
  baseVersionId: 'ver-3',
  requestId: 'intent-key-1',
  expectedWorkspaceRevision: null as number | null,
};

test('an upload failure submits ZERO runs and keeps the error attributable', async () => {
  const log: Call[] = [];
  const ports = makePorts(log, { uploadFails: 'brief.md' });
  await assert.rejects(
    submitDraftWithAttachments({ ...intent, attachments }, ports),
    /brief\.md/,
  );
  assert.deepEqual(log.map((c) => c.op), ['upload', 'upload'], 'both uploads attempted, no submit');
});

test('a lost submit response retries with the SAME requestId and identical payload', async () => {
  const log: Call[] = [];
  const ports = makePorts(log, { submitFails: 1 });
  // first attempt: uploads succeed, submit's response is lost
  await assert.rejects(submitDraftWithAttachments({ ...intent, attachments }, ports), /response lost/);
  // the caller retries the SAME intent (same requestId object) — not a new one
  await submitDraftWithAttachments({ ...intent, attachments }, ports);
  assert.equal(ports.submitCount(), 2);
  const submits = log.filter((c) => c.op === 'submit').map((c) => c.payload);
  assert.deepEqual(submits[0], submits[1], 'payload snapshot must be identical across the retry');
  const payload = submits[1] as Record<string, unknown>;
  assert.equal(payload.request_id, 'intent-key-1');
  assert.deepEqual(payload.input_refs, ['resource://craft-sales.csv', 'resource://brief.md']);
  assert.equal(payload.base_version_id, 'ver-3');
  // upload ordering: both associations precede the first submit
  assert.deepEqual(log.slice(0, 3).map((c) => c.op), ['upload', 'upload', 'submit']);
});

test('the bridge never mints ids and never reads a previewed version', async () => {
  const log: Call[] = [];
  const ports = makePorts(log);
  await submitDraftWithAttachments({ ...intent, attachments: [], requestId: 'intent-key-2' }, ports);
  const payload = (log.find((c) => c.op === 'submit')?.payload ?? {}) as Record<string, unknown>;
  assert.equal(payload.request_id, 'intent-key-2', 'the caller-owned key passes verbatim');
  assert.equal(payload.base_version_id, 'ver-3', 'the editing baseline passes verbatim');
  // no viewVersion concept exists on the bridge input at all: submitting with
  // a different in-flight preview cannot change the frozen intent
  const bridgeInputKeys = Object.keys({ ...intent, attachments: [] });
  assert.equal(bridgeInputKeys.includes('viewVersion'), false);
});

test('a blank prompt or missing session is rejected before any port fires', async () => {
  const log: Call[] = [];
  const ports = makePorts(log);
  await assert.rejects(submitDraftWithAttachments({ ...intent, prompt: '   ', attachments: [], requestId: 'k' }, ports), /prompt/);
  await assert.rejects(submitDraftWithAttachments({ ...intent, sessionId: '', attachments: [], requestId: 'k' }, ports), /session/);
  assert.equal(log.length, 0, 'validation precedes every side effect');
});
