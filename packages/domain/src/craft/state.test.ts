import { test } from 'node:test';
import assert from 'node:assert/strict';
import { applyCraftEvent } from './state.ts';

test('child completion never finishes main run', () => {
  const s={generation:1,runId:'r',seq:0,mainStatus:'running',delegationStatus:'running',versionId:null};
  const n=applyCraftEvent(s,{generation:1,runId:'r',seq:1,kind:'delegation.finished'});
  assert.equal(n.mainStatus,'running');
  assert.equal(applyCraftEvent(n,{generation:1,runId:'r',seq:1,kind:'delegation.finished'}),n);
  assert.equal(applyCraftEvent(n,{generation:0,runId:'r',seq:2,kind:'delegation.finished'}),n);
});

test('stale run events never cross streams', () => {
  const s={generation:1,runId:'r1',seq:5,mainStatus:'running',delegationStatus:'running',versionId:null};
  // Another run's event (refresh replay of the previous run) must not apply.
  assert.equal(applyCraftEvent(s,{generation:1,runId:'r2',seq:6,kind:'delegation.finished'}),s);
  // Older seq is dropped even for the current run.
  assert.equal(applyCraftEvent(s,{generation:1,runId:'r1',seq:5,kind:'delegation.finished'}),s);
  assert.equal(applyCraftEvent(s,{generation:1,runId:'r1',seq:0,kind:'artifact.published',versionId:'v9'}),s);
});

test('artifact.published moves the selected version', () => {
  const s={generation:1,runId:'r',seq:1,mainStatus:'running',delegationStatus:'running',versionId:null};
  const n=applyCraftEvent(s,{generation:1,runId:'r',seq:2,kind:'artifact.published',versionId:'v1'});
  assert.equal(n.versionId,'v1');
  // A publish without a version id keeps the previous selection.
  const kept=applyCraftEvent(n,{generation:1,runId:'r',seq:3,kind:'artifact.published'});
  assert.equal(kept.versionId,'v1');
  // Non-publish kinds never touch the selection.
  const after=applyCraftEvent(kept,{generation:1,runId:'r',seq:4,kind:'delegation.text'});
  assert.equal(after.versionId,'v1');
  assert.equal(after.seq,4);
});

test('delegation status only tracks delegation kinds', () => {
  const s={generation:1,runId:'r',seq:1,mainStatus:'running',delegationStatus:'running',versionId:null};
  const started=applyCraftEvent(s,{generation:1,runId:'r',seq:2,kind:'delegation.started'});
  assert.equal(started.delegationStatus,'running');
  const finished=applyCraftEvent(started,{generation:1,runId:'r',seq:3,kind:'delegation.finished'});
  assert.equal(finished.delegationStatus,'finished');
  // A later delegation.started on the same run does not resurrect 'running'.
  const replay=applyCraftEvent(finished,{generation:1,runId:'r',seq:4,kind:'delegation.started'});
  assert.equal(replay.delegationStatus,'finished');
});
