import assert from 'node:assert/strict';
import test from 'node:test';

import {
  clampInstallPercent,
  initialSkillTimelineState,
  installProgressPercent,
  isTerminalInstallProgress,
  parseSkillInstallProgressFrame,
  reduceSkillTimelineFrame,
} from './skill-install.ts';

// --- install-events frames (backend internal/handler/sandbox_skill.go:554-563) ----------

test('progress frames parse into percent/stage/log/status/done events', () => {
  const parsed = parseSkillInstallProgressFrame(JSON.stringify({ percent: 42, stage: 'building', log: 'step 1', status: 'installing', done: false }));
  assert.deepEqual(parsed, { percent: 42, stage: 'building', log: 'step 1', status: 'installing', done: false });
  assert.deepEqual(parseSkillInstallProgressFrame(JSON.stringify({ percent: 100, stage: 'done', done: true })), { percent: 100, stage: 'done', done: true });
});

test('progress frames that are not objects or lack percent/stage/done are dropped', () => {
  assert.equal(parseSkillInstallProgressFrame('not json'), null);
  assert.equal(parseSkillInstallProgressFrame('[1,2]'), null);
  assert.equal(parseSkillInstallProgressFrame(JSON.stringify({ stage: 'x', done: false })), null);
  assert.equal(parseSkillInstallProgressFrame(JSON.stringify({ percent: '40', stage: 'x', done: false })), null);
  assert.equal(parseSkillInstallProgressFrame(JSON.stringify({ percent: 40, done: false })), null);
  assert.equal(parseSkillInstallProgressFrame(JSON.stringify({ percent: 40, stage: 'x' })), null);
});

test('done flags the terminal frames: done, failed and detached stages', () => {
  assert.equal(isTerminalInstallProgress({ percent: 100, stage: 'done', done: true }), true);
  assert.equal(isTerminalInstallProgress({ percent: 100, stage: 'failed', log: 'boom', done: true }), true);
  assert.equal(isTerminalInstallProgress({ percent: 55, stage: 'detached', done: true }), true);
  assert.equal(isTerminalInstallProgress({ percent: 55, stage: 'building', done: false }), false);
});

// --- progressOf mirror (SandboxSkillsPanel.vue:1017-1024) --------------------------------

test('install percent clamps to 0..100 and falls back to the skill status', () => {
  assert.equal(clampInstallPercent(130), 100);
  assert.equal(clampInstallPercent(-3), 0);
  assert.equal(installProgressPercent({ percent: 130, stage: 'x', done: false }, 'installing'), 100);
  assert.equal(installProgressPercent({ percent: -3, stage: 'x', done: false }, 'installing'), 0);
  assert.equal(installProgressPercent(undefined, 'removing'), 5);
  assert.equal(installProgressPercent(undefined, 'installing', true), 5);
  assert.equal(installProgressPercent(undefined, 'ready'), 100);
  assert.equal(installProgressPercent(undefined, 'failed'), 100);
  assert.equal(installProgressPercent(undefined, 'installing'), 0);
});

// --- transcript timeline reduction (SkillInstallTimeline.vue frames) ---------------------

test('timeline reducer keeps one prompt turn and accumulates the agent run', () => {
  let state = initialSkillTimelineState();
  state = reduceSkillTimelineFrame(state, { response_type: 'install_prompt', content: 'Install pdf-skill' });
  state = reduceSkillTimelineFrame(state, { response_type: 'install_prompt', content: 'again' });
  state = reduceSkillTimelineFrame(state, { response_type: 'thinking', content: 'plan ' });
  state = reduceSkillTimelineFrame(state, { response_type: 'thinking', content: 'more' });
  state = reduceSkillTimelineFrame(state, { response_type: 'tool_call', data: { tool_call_id: 't1', tool_name: 'shell' } });
  state = reduceSkillTimelineFrame(state, { response_type: 'tool_result', data: { tool_call_id: 't1', result: 'ok' } });
  state = reduceSkillTimelineFrame(state, { response_type: 'answer', content: 'Installed ' });
  state = reduceSkillTimelineFrame(state, { response_type: 'answer', content: 'successfully' });
  state = reduceSkillTimelineFrame(state, { response_type: 'complete' });
  assert.equal(state.prompt, 'Install pdf-skill');
  assert.equal(state.thinking, 'plan more');
  assert.equal(state.answer, 'Installed successfully');
  assert.deepEqual(state.toolCalls, [{ id: 't1', name: 'shell', status: 'completed', result: 'ok' }]);
  assert.equal(state.complete, true);
  assert.equal(state.error, '');
  assert.ok(state.frames >= 6);
});

test('timeline reducer records tool failures and error frames without completing the run', () => {
  let state = initialSkillTimelineState();
  state = reduceSkillTimelineFrame(state, { response_type: 'tool_call', data: { tool_call_id: 't2', tool_name: 'pip' } });
  state = reduceSkillTimelineFrame(state, { response_type: 'tool_result', data: { tool_call_id: 't2', result: 'missing package', is_error: true } });
  state = reduceSkillTimelineFrame(state, { response_type: 'error', content: 'install failed' });
  assert.equal(state.toolCalls[0]?.status, 'failed');
  assert.equal(state.toolCalls[0]?.result, 'missing package');
  assert.equal(state.error, 'install failed');
  assert.equal(state.complete, false);
});

test('timeline reducer ignores junk frames and tolerates flat chat fields', () => {
  let state = initialSkillTimelineState();
  state = reduceSkillTimelineFrame(state, 'nope');
  state = reduceSkillTimelineFrame(state, null);
  state = reduceSkillTimelineFrame(state, { response_type: 'future_frame', content: 'ignored' });
  state = reduceSkillTimelineFrame(state, { type: 'thinking', content: 'flat' });
  state = reduceSkillTimelineFrame(state, { type: 'tool_call', tool_call_id: 't9', tool_name: 'flat-tool' });
  assert.equal(state.prompt, '');
  assert.equal(state.thinking, 'flat');
  assert.deepEqual(state.toolCalls, [{ id: 't9', name: 'flat-tool', status: 'pending' }]);
  assert.equal(state.complete, false);
});
