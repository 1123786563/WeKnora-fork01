import assert from 'node:assert/strict';
import test from 'node:test';

import {
  createSandboxSkillInstallApi,
  parseSkillInstallEvent,
  parseSkillInstallGuidanceState,
  skillGuidancePath,
  skillInstallEventsPath,
  skillTranscriptPath,
  type ParsedSkillSseFrame,
} from './skill-install.ts';
import type { ClientRequest } from '../client.ts';
import type { HttpStreamResult } from '../ports.ts';

function sseStream(status: number, frames: string[]): Promise<HttpStreamResult> {
  async function* chunks(): AsyncIterable<string> {
    for (const frame of frames) yield frame;
  }
  return Promise.resolve({ status, headers: {}, chunks: chunks() });
}

// --- path builders (routes: internal/router/routes_infra.go:70-77) ------------------------

test('skill paths encode config and skill ids under /sandbox-configs', () => {
  assert.equal(skillInstallEventsPath('cfg 1', 'sk/2'), '/api/v1/sandbox-configs/cfg%201/skills/sk%2F2/install-events');
  assert.equal(skillTranscriptPath('cfg', 'sk'), '/api/v1/sandbox-configs/cfg/skills/sk/transcript');
  assert.equal(skillGuidancePath('cfg', 'sk'), '/api/v1/sandbox-configs/cfg/skills/sk/guidance');
  assert.throws(() => skillInstallEventsPath(' ', 'sk'), /configId/);
  assert.throws(() => skillInstallEventsPath('cfg', ''), /skillId/);
});

// --- frame parsing ------------------------------------------------------------------------

test('progress events require percent/stage/done and keep optional strings', () => {
  assert.deepEqual(parseSkillInstallEvent({ percent: 7, stage: 'pulling', log: 'l', status: 'installing', done: false }),
    { percent: 7, stage: 'pulling', log: 'l', status: 'installing', done: false });
  assert.equal(parseSkillInstallEvent({ percent: 7, stage: 'x' }), null);
  assert.equal(parseSkillInstallEvent('x'), null);
});

test('guidance state parses accepting + message statuses', () => {
  assert.deepEqual(parseSkillInstallGuidanceState({ accepting: true, messages: [{ id: 'a', content: 'hi', status: 'pending' }] }),
    { accepting: true, messages: [{ id: 'a', content: 'hi', status: 'pending' }] });
  assert.throws(() => parseSkillInstallGuidanceState({ accepting: 'yes', messages: [] }), /accepting/);
});

// --- install-events SSE consumption -------------------------------------------------------

test('followInstallEvents streams GET install-events through sendStream and stops on done', async () => {
  const requests: ClientRequest[] = [];
  const seen: string[] = [];
  const api = createSandboxSkillInstallApi({
    request: async (input) => { requests.push(input); return undefined; },
    sendStream: (input) => {
      requests.push(input);
      return sseStream(200, [
        'event: message\ndata: {"percent":10,"stage":"queued","done":false}\n\n',
        ': keep-alive\n\n',
        'event: message\ndata: {"percent":100,"stage":"done","status":"ready","done":true}\n\n',
      ]);
    },
  });
  let terminal: ParsedSkillSseFrame | null = null;
  await api.followInstallEvents('cfg', 'sk', (frame) => {
    seen.push(frame.event.stage);
    if (frame.terminal) terminal = frame;
  });
  assert.deepEqual(seen, ['queued', 'done']);
  assert.equal(terminal !== null && (terminal as ParsedSkillSseFrame).event.stage, 'done');
  assert.equal(requests.length, 1);
  assert.equal(requests[0]!.method, 'GET');
  assert.equal(requests[0]!.path, '/api/v1/sandbox-configs/cfg/skills/sk/install-events');
  assert.equal(requests[0]!.headers['accept'], 'text/event-stream');
});

test('followInstallEvents falls back to the buffered request path without sendStream', async () => {
  const requests: ClientRequest[] = [];
  const api = createSandboxSkillInstallApi({
    request: async (input) => {
      requests.push(input);
      return 'event: message\ndata: {"percent":5,"stage":"s","done":false}\n\n';
    },
  });
  const stages: string[] = [];
  await api.followInstallEvents('cfg', 'sk', (frame) => { stages.push(frame.event.stage); });
  assert.deepEqual(stages, ['s']);
  assert.equal(requests[0]!.headers['accept'], 'text/event-stream');
});

// --- transcript SSE -----------------------------------------------------------------------

test('followTranscript reports served for 2xx and status 204 as not served without frames', async () => {
  const frames: unknown[] = [];
  const servedApi = createSandboxSkillInstallApi({
    request: async () => undefined,
    sendStream: (input) => {
      assert.equal(input.path, '/api/v1/sandbox-configs/cfg/skills/sk/transcript');
      return sseStream(200, [
        'event: message\ndata: {"id":"m1","response_type":"install_prompt","content":"go"}\n\n',
        'event: message\ndata: {"id":"m1","response_type":"complete","done":true}\n\n',
      ]);
    },
  });
  const served = await servedApi.followTranscript('cfg', 'sk', (frame) => { frames.push(frame); });
  assert.equal(served, true);
  assert.equal(frames.length, 2);

  const pendingApi = createSandboxSkillInstallApi({ request: async () => undefined, sendStream: () => sseStream(204, ['']) });
  assert.equal(await pendingApi.followTranscript('cfg', 'sk', () => {}), false);

  const missingApi = createSandboxSkillInstallApi({ request: async () => undefined, sendStream: () => sseStream(404, ['']) });
  await assert.rejects(missingApi.followTranscript('cfg', 'sk', () => {}), /404/);
});

// --- guidance read + steer POST -----------------------------------------------------------

test('guidance reads the Admin-only GET and steer posts the snake_case payload', async () => {
  const requests: ClientRequest[] = [];
  const respond = (body: unknown) => async (input: ClientRequest) => { requests.push(input); return body; };
  const api = createSandboxSkillInstallApi({ request: respond({ success: true, data: { accepting: true, messages: [] } }) });
  const state = await api.guidance('cfg', 'sk');
  assert.equal(state.accepting, true);
  assert.equal(requests[0]!.method, 'GET');
  assert.equal(requests[0]!.path, '/api/v1/sandbox-configs/cfg/skills/sk/guidance');

  await api.steer('cfg', 'sk', { expectedMessageId: 'm1', steerId: 's-1', content: 'use apt' });
  assert.equal(requests[1]!.method, 'POST');
  assert.equal(requests[1]!.path, '/api/v1/sandbox-configs/cfg/skills/sk/guidance');
  assert.deepEqual(requests[1]!.body, { expected_message_id: 'm1', steer_id: 's-1', content: 'use apt' });

  await assert.rejects(api.steer('cfg', 'sk', { expectedMessageId: 'm1', steerId: ' ', content: 'x' }), /steerId/);
  await assert.rejects(api.steer('cfg', 'sk', { expectedMessageId: 'm1', steerId: 's-2', content: ' '.repeat(10001) }), /content/);
});
