import test from 'node:test';
import assert from 'node:assert/strict';
import { sourceKey, deduplicateSourceEvents, projectSnapshot, type SourceEvent } from './events.ts';

const event = (overrides: Partial<SourceEvent> = {}): SourceEvent => ({bindingID:'b', generation:'g1', eventID:'1', attemptID:'a', type:'text.delta', payload:{text:'same'}, ...overrides});

test('daemon restart does not collide with old source sequence',()=>{
 assert.notEqual(sourceKey(event()), sourceKey(event({generation:'g2'})));
 assert.equal(sourceKey(event()), sourceKey(event({payload:{text:'different'}})));
});
test('dedupe keeps same source once and rejects payload conflict',()=>{
 assert.equal(deduplicateSourceEvents([event(), event()]).length, 1);
 assert.throws(()=>deduplicateSourceEvents([event(), event({payload:{text:'different'}})]), /conflict/);
});
test('unknown and out of order events remain visible and snapshot is terminally stable',()=>{
 const out = deduplicateSourceEvents([event({eventID:'2', type:'future.new', payload:{x:1}}), event({eventID:'1'}), event({eventID:'2', type:'future.new', payload:{x:1}})]);
 assert.equal(out[0].type, 'unknown');
 const snapshot = projectSnapshot(out);
 assert.equal(snapshot.events.length, 2);
 assert.equal(snapshot.events[0].seq, 1);
});
test('terminal status precedence is deterministic regardless of source order',()=>{
 const out = deduplicateSourceEvents([
   event({eventID:'s', type:'execution.succeeded'}),
   event({eventID:'f', type:'execution.failed'}),
   event({eventID:'c', type:'execution.canceled'}),
 ]);
 const snapshot = projectSnapshot(out);
 assert.equal(snapshot.executionStatus, 'canceled');
 assert.equal(snapshot.settlementStatus, 'settled');
});
