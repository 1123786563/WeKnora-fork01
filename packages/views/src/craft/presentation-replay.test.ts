// CFT-S01-T007: replay stability of the message projection — the composition
// of the W05 log (dedupe by seq), projectAssistant (main-run-only
// completeness) and craftThreadMessages (stable ids, T006). Reconnects
// replay the same frames; nothing below may mint new ids, duplicate text or
// let a finished CHILD delegation complete the MAIN message.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import React from 'react';
import type { CraftEventFrame } from '@weknora/core/craft/controller';
import {
  createCraftMessageLog,
  projectAssistant,
} from './presentation.ts';
import { craftThreadMessages, type CraftThreadRow } from './assistant-runtime.tsx';

/** One SSE wire frame exactly as the transport delivers it (JSON data). */
function wire(seq: number, kind: string, data: Record<string, unknown> = {}): CraftEventFrame {
  return {
    event: String(seq),
    data: JSON.stringify({
      seq,
      type: 'craft',
      payload: { workspace_id: 'w1', delegation_id: null, tool_call_id: 'call_1', kind, data },
    }),
  };
}

const RUN_FRAMES: CraftEventFrame[] = [
  wire(1, 'delegation.started'),
  wire(2, 'delegation.text', { text: '正在生成' }),
  wire(3, 'delegation.text', { text: '销售报告…' }),
];

function feed(log: ReturnType<typeof createCraftMessageLog>, frames: CraftEventFrame[], times = 1): void {
  for (let i = 0; i < times; i += 1) for (const f of frames) log.ingest(f);
}

function projectedRows(events: { seq: number }[], mainStatus: string): CraftThreadRow[] {
  const projection = projectAssistant(events as never, mainStatus);
  return [{ prompt: '按月分析', assistantText: projection.text, assistantComplete: projection.complete }];
}

test('replaying the same seq twice adds no message and no text', () => {
  const log = createCraftMessageLog();
  feed(log, RUN_FRAMES, 2);
  const snapshotAfterFirst = log.getSnapshot();
  const once = craftThreadMessages({
    runId: 'run-r1',
    prompt: '按月分析',
    assistantText: projectAssistant(snapshotAfterFirst.events, 'running').text,
    assistantComplete: false,
    archivedTurns: [],
  });
  // a FULL replay of the same frames after a reconnect must be idempotent
  feed(log, RUN_FRAMES, 2);
  const twice = craftThreadMessages({
    runId: 'run-r1',
    prompt: '按月分析',
    assistantText: projectAssistant(log.getSnapshot().events, 'running').text,
    assistantComplete: false,
    archivedTurns: [],
  });
  assert.deepEqual(twice, once, 'double replay changes nothing: ids, count, text');
  assert.equal(twice.filter((m) => m.role === 'assistant').length, 1);
  const text = String(twice[twice.length - 1].content);
  assert.equal((text.match(/正在生成/g) ?? []).length, 1, 'incremental text is not duplicated');
  assert.equal((text.match(/销售报告/g) ?? []).length, 1, 'later chunks append exactly once');
});

test('a finished child delegation never completes the main message', () => {
  const log = createCraftMessageLog();
  feed(log, [...RUN_FRAMES, wire(4, 'delegation.finished', { status: 'succeeded' })], 1);
  const events = log.getSnapshot().events;
  const projection = projectAssistant(events, 'running');
  assert.equal(projection.childStatus, 'succeeded', 'the child status is tracked separately');
  assert.equal(projection.complete, false, 'the MAIN projection stays incomplete while the main run runs');
  const rows = projectedRows(events, 'running');
  const msgs = craftThreadMessages({ runId: 'run-a', prompt: 'p', assistantText: rows[0].assistantText, assistantComplete: rows[0].assistantComplete, archivedTurns: [] });
  const assistant = msgs[msgs.length - 1];
  assert.deepEqual(assistant.status, { type: 'running' });
  assert.equal(assistant.id, 'run-a-assistant', 'ids stay run-derived, never random');
  // ...and when the MAIN run reaches its terminal status, only then complete:
  const doneRows = projectedRows(events, 'succeeded');
  assert.equal(doneRows[0].assistantComplete, true);
});

test('tool facts keep stable call ids across replays', () => {
  const frames = [
    wire(1, 'delegation.started'),
    wire(2, 'delegation.tool', { tool: 'craft_delegate', status: 'running', call_id: 'call_1' }),
  ];
  const log = createCraftMessageLog();
  feed(log, frames, 1);
  const a = projectAssistant(log.getSnapshot().events, 'running');
  feed(log, frames, 3); // reconnect replays the same tool frame
  const b = projectAssistant(log.getSnapshot().events, 'running');
  assert.deepEqual(b.tools, a.tools, 'replays do not duplicate tool cards');
  assert.equal(a.tools[0]?.callId, 'call_1', 'tool cards key on the stable server call id');
});

test('thread tool cards render stable keys and readable summaries', async () => {
  const React = await import('react');
  const { renderToStaticMarkup } = await import('../../../../apps/web/node_modules/react-dom/server.js');
  const { CraftToolFactList, CraftInteractionSummary, toolStatusLabel } = await import('./thread.tsx');
  const facts = [
    { seq: 2, tool: 'craft_delegate', status: 'succeeded', callId: 'call_1' },
    { seq: 5, tool: 'craft_delegate', status: 'unknown', callId: 'call_2' },
  ];
  const markup = renderToStaticMarkup(React.createElement(CraftToolFactList, { facts }));
  assert.match(markup, /data-call-id="call_1"/);
  assert.match(markup, /craft_delegate · 已完成/);
  assert.match(markup, /craft_delegate · 结果待核对/, 'unknown stays unknown — never repainted as success');
  assert.equal(toolStatusLabel('running'), '执行中');
  const interaction = renderToStaticMarkup(React.createElement(CraftInteractionSummary, {
    card: { id: 'i-9', seq: 7, kind: 'permission', prompt: '允许执行构建？', scope: 'build', resolved: false, allowedActions: ['approve', 'reject'] },
  }));
  assert.match(interaction, /i-9 · 待批准/);
  assert.match(interaction, /aria-live="polite"/, 'interaction state is announced, not color-only');
});
