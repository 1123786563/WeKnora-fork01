import test from 'node:test';
import assert from 'node:assert/strict';
import {
  createDictationController,
  defaultDictationLimits,
  DictationError,
  type DictationPort,
  type DictationScope,
} from './dictation.ts';

test('transcription remains a draft until explicit confirmation',async()=>{
 let draft='';const sent:string[]=[];
 const c=createDictationController(async()=>'删除这个文件',value=>{draft=value;},()=>draft,async text=>{sent.push(text);});
 await c.finish();assert.equal(draft,'删除这个文件');assert.deepEqual(sent,[]);
 draft='保留这个文件';await c.confirm();assert.deepEqual(sent,['保留这个文件']);
});

test('confirm clears the draft only after the send succeeds',async()=>{
 let draft='转写文本';const sent:string[]=[];
 const c=createDictationController(async()=>'',value=>{draft=value;},()=>draft,async text=>{sent.push(text);});
 await c.confirm();
 assert.deepEqual(sent,['转写文本']);
 assert.equal(draft,'');
});

test('confirm on an empty draft rejects with TRANSCRIPT_REQUIRED and never sends',async()=>{
 let draft='   ';const sent:string[]=[];
 const c=createDictationController(async()=>'',value=>{draft=value;},()=>draft,async text=>{sent.push(text);});
 await assert.rejects(()=>c.confirm(),/TRANSCRIPT_REQUIRED/);
 assert.deepEqual(sent,[]);
 assert.equal(draft,'   ');
});

test('a failed confirm (single-flight busy sender) preserves the draft',async()=>{
 let draft='保留我';const sent:string[]=[];
 const c=createDictationController(async()=>'',value=>{draft=value;},()=>draft,async()=>{throw new Error('SEND_IN_PROGRESS');});
 await assert.rejects(()=>c.confirm(),/SEND_IN_PROGRESS/);
 assert.deepEqual(sent,[]);
 assert.equal(draft,'保留我');
});

function fakePort(overrides: Partial<DictationPort> & { stopResult?: { uri: string; durationMs: number } } = {}) {
  const { stopResult = { uri: 'file:///cache/dictation-1.m4a', durationMs: 1500 }, ...portOverrides } = overrides;
  const calls: string[] = [];
  const impl: DictationPort = {
    async start() { return undefined; },
    async stop() { return stopResult; },
    async cancel() { return undefined; },
    async transcribe() { return '删除这个文件'; },
    ...portOverrides,
  };
  const port: DictationPort = {
    async start() { calls.push('start'); return impl.start(); },
    async stop() { calls.push('stop'); return impl.stop(); },
    async cancel() { calls.push('cancel'); return impl.cancel(); },
    async transcribe(uri) { calls.push(`transcribe:${uri}`); return impl.transcribe(uri); },
  };
  return { port, calls };
}

function harness(port: DictationPort, extra: { limits?: Record<string, number>; scope?: DictationScope } = {}) {
  let draft = '';
  const sent: string[] = [];
  const controller = createDictationController(
    async () => { throw new Error('PORTLESS_TRANSCRIBE_UNUSED'); },
    (value) => { draft = value; },
    () => draft,
    async (text) => { sent.push(text); },
    { port, limits: extra.limits, scope: extra.scope },
  );
  return { controller, readDraft: () => draft, sent };
}

test('hold-to-talk state machine: begin records, release transcribes into the draft only',async()=>{
 const { port, calls } = fakePort();
 const { controller, readDraft, sent } = harness(port);
 assert.equal(controller.state(),'idle');
 await controller.begin();
 assert.equal(controller.state(),'recording');
 await controller.finish();
 assert.equal(controller.state(),'ready');
 assert.equal(readDraft(),'删除这个文件');
 assert.deepEqual(sent,[]);
 assert.deepEqual(calls,['start','stop','transcribe:file:///cache/dictation-1.m4a','cancel']);
});

test('a stray release without a hold is ignored in port mode',async()=>{
 const { port, calls } = fakePort();
 const { controller } = harness(port);
 await controller.finish();
 assert.equal(controller.state(),'idle');
 assert.deepEqual(calls,[]);
});

test('denied microphone permission lands in the error state and falls back to text input',async()=>{
 let deny=true;
 const { port, calls } = fakePort({ start: async () => { if (deny) { deny=false; throw new DictationError('MIC_PERMISSION_DENIED'); } } });
 const { controller, readDraft } = harness(port);
 await controller.begin();
 assert.equal(controller.state(),'error');
 assert.equal(controller.isPermissionDenied(),true);
 assert.equal(controller.failure()?.code,'MIC_PERMISSION_DENIED');
 // The text-input fallback: the draft is untouched and no stop/transcribe ran.
 assert.equal(readDraft(),'');
 assert.deepEqual(calls,['start']);
 // A later hold retries the permission instead of being stuck in the error.
 await controller.begin();
 assert.equal(controller.state(),'recording');
 await controller.cancel();
});

test('interrupted recording keeps the draft and never transcribes',async()=>{
 const { port, calls } = fakePort({ stop: async () => { throw new Error('recorder interrupted by phone call'); } });
 const { controller, readDraft } = harness(port);
 await controller.begin();
 await controller.finish();
 assert.equal(controller.state(),'error');
 assert.equal(controller.failure()?.code,'RECORD_INTERRUPTED');
 assert.equal(readDraft(),'');
 assert.ok(!calls.some((call) => call.startsWith('transcribe:')));
});

test('transcription timeout keeps the draft',async()=>{
 const { port } = fakePort({ transcribe: () => new Promise<string>(() => undefined) });
 const { controller, readDraft } = harness(port, { limits: { transcribeTimeoutMs: 30 } });
 await controller.begin();
 await controller.finish();
 assert.equal(controller.state(),'error');
 assert.equal(controller.failure()?.code,'TRANSCRIBE_TIMEOUT');
 assert.equal(readDraft(),'');
});

test('transcription failure keeps the draft',async()=>{
 const { port } = fakePort({ transcribe: async () => { throw new Error('upstream 503'); } });
 const { controller, readDraft } = harness(port);
 await controller.begin();
 await controller.finish();
 assert.equal(controller.failure()?.code,'TRANSCRIBE_FAILED');
 assert.equal(readDraft(),'');
});

test('empty audio (zero duration) is rejected before transcription',async()=>{
 const { port, calls } = fakePort({ stopResult: { uri: 'file:///cache/empty.m4a', durationMs: 0 } });
 const { controller, readDraft } = harness(port);
 await controller.begin();
 await controller.finish();
 assert.equal(controller.failure()?.code,'EMPTY_AUDIO');
 assert.equal(readDraft(),'');
 assert.ok(!calls.some((call) => call.startsWith('transcribe:')));
});

test('an over-long capture is rejected as AUDIO_TOO_LONG',async()=>{
 const { port, calls } = fakePort({ stopResult: { uri: 'file:///cache/long.m4a', durationMs: 90_000 } });
 const { controller, readDraft } = harness(port);
 await controller.begin();
 await controller.finish();
 assert.equal(controller.failure()?.code,'AUDIO_TOO_LONG');
 assert.equal(readDraft(),'');
 assert.ok(!calls.some((call) => call.startsWith('transcribe:')));
});

test('the default duration cap is 60s and capability config may tighten it',async()=>{
 assert.equal(defaultDictationLimits.maxDurationMs,60_000);
 const { port, calls } = fakePort();
 const { controller, readDraft } = harness(port,{limits:{maxDurationMs:40}});
 await controller.begin();
 // No explicit finish: the cap auto-finishes the hold and still transcribes.
 await new Promise((resolve)=>setTimeout(resolve,150));
 assert.equal(controller.state(),'ready');
 assert.equal(readDraft(),'删除这个文件');
 assert.ok(calls.includes('stop'));
});

test('a capped capture within the timer tolerance is still transcribed',async()=>{
 const { port } = fakePort({ stopResult: { uri: 'file:///cache/capped.m4a', durationMs: 60_800 } });
 const { controller } = harness(port);
 await controller.begin();
 await controller.finish();
 assert.equal(controller.state(),'ready');
});

test('chinese punctuation survives into the draft verbatim',async()=>{
 const { port } = fakePort({ transcribe: async () => '删除，这个「重要」文件。' });
 const { controller, readDraft } = harness(port);
 await controller.begin();
 await controller.finish();
 assert.equal(readDraft(),'删除，这个「重要」文件。');
});

test('a successful transcription replaces the previous draft content',async()=>{
 const { port } = fakePort();
 const { controller, readDraft } = harness(port);
 await controller.begin();
 await controller.finish();
 assert.equal(readDraft(),'删除这个文件');
});

test('cancel during recording deletes the temp audio, keeps the draft, and ignores the stray release',async()=>{
 const { port, calls } = fakePort();
 const { controller, readDraft } = harness(port);
 await controller.begin();
 await controller.cancel();
 assert.equal(controller.state(),'idle');
 assert.equal(readDraft(),'');
 assert.ok(calls.includes('cancel'));
 // The release event after a cancelled hold must not transcribe anything.
 await controller.finish();
 assert.ok(!calls.some((call) => call.startsWith('transcribe:')));
});

test('cancel during transcription drops the late transcript',async()=>{
 let releaseTranscribe!: (value: string) => void;
 const { port } = fakePort({ transcribe: () => new Promise<string>((resolve) => { releaseTranscribe = resolve; }) });
 const { controller, readDraft } = harness(port);
 await controller.begin();
 const finishing = controller.finish();
 await new Promise((resolve)=>setTimeout(resolve,10));
 assert.equal(controller.state(),'transcribing');
 await controller.cancel();
 assert.equal(controller.state(),'idle');
 releaseTranscribe('迟到的转写');
 await finishing;
 assert.equal(readDraft(),'');
 assert.equal(controller.state(),'idle');
});

test('a space switch (scope change) discards the transcript without touching the draft',async()=>{
 let accept = true;
 const { port } = fakePort();
 const scope: DictationScope = { capture: () => ({ generation: 7, signal: new AbortController().signal }), accept: () => accept };
 const { controller, readDraft } = harness(port,{scope});
 await controller.begin();
 accept = false;
 await controller.finish();
 assert.equal(controller.failure()?.code,'SCOPE_CHANGED');
 assert.equal(readDraft(),'');
});

test('double confirm while the first send is in flight cannot double-send',async()=>{
 let release!: () => void;
 const sent: string[] = [];
 let draft = '转写文本';
 let inFlight = false;
 const c = createDictationController(
  async()=>'' ,
  value=>{draft=value;},
  ()=>draft,
  async text=>{
    if (inFlight) throw new Error('SEND_IN_PROGRESS');
    inFlight = true;
    sent.push(text);
    await new Promise<void>((resolve)=>{release=resolve;});
    inFlight = false;
  },
 );
 const first = c.confirm();
 await assert.rejects(()=>c.confirm(),/SEND_IN_PROGRESS/);
 release();
 await first;
 assert.deepEqual(sent,['转写文本']);
});
