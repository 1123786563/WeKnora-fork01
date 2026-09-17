import * as React from 'react';
// @ts-expect-error react-test-renderer has no declarations in this workspace.
import { act, create } from 'react-test-renderer';
import { AppState } from 'react-native';
import { describe, expect, it, vi } from 'vitest';
import { ConversationScreen, ProductConversationMessages } from './ConversationScreen';
import type { ConversationViewModel } from './view-model';
import type { ExecutionRecovery } from '../executions/recovery';
import { DictationError, type DictationPort } from '../voice/dictation';

vi.mock('react-native', async () => {
  const ReactModule = await import('react');
  const host = (name: string) => (props: any) => ReactModule.createElement(name, props, props.children);
  const listeners = new Set<(state: string) => void>();
  return {
    Pressable: host('Pressable'), Text: host('Text'), TextInput: host('TextInput'), View: host('View'), ScrollView: host('ScrollView'),
    AppState: {
      currentState: 'active',
      addEventListener: (type: string, listener: (state: string) => void) => {
        if (type !== 'change') return { remove: () => undefined };
        listeners.add(listener);
        return { remove: () => { listeners.delete(listener); } };
      },
      // Test-only emission seam for the mocked native AppState.
      emitForTest: (state: string) => { listeners.forEach((listener) => listener(state)); },
    },
  };
});
vi.mock('@/-session/SessionView', async () => {
  const ReactModule = await import('react');
  return { SessionView: (props: any) => ReactModule.createElement('SessionView', props) };
});

function model(overrides: Partial<ConversationViewModel> = {}): ConversationViewModel {
  return {
    scope: { origin: 'https://api.example', userId: 'u1', tenantId: 't1', spaceId: 's1' },
    messages: [{ id: 'm1', role: 'assistant', text: 'ready' }],
    pendingInteractions: [{ id: 'p1', kind: 'approval', status: 'pending', label: 'Run tool', revision: 7 }],
    capabilities: { canCancel: false, canSteer: false, canAttach: false, canVoice: false },
    execution: { runID: 'run-1', requestID: 'q-1', status: 'pending' },
    commands: { cancel: async () => undefined, steer: async () => undefined },
    ...overrides,
  };
}

describe('ConversationScreen', () => {
it('mounts the native seam, projects product execution, and sends typed approval revision', async () => {
  const decisions: Array<[string, number]> = [];
  const viewModel = model({ commands: {
    cancel: async () => undefined,
    steer: async () => undefined,
    approve: async (id, revision) => { decisions.push([id, revision ?? -1]); },
    reject: async () => undefined,
  } });
  const SessionRenderer = ({ viewModel: current }: { id: string; viewModel: ConversationViewModel }) => React.createElement(ProductConversationMessages, { viewModel: current });
  let renderer: ReturnType<typeof create>;
  await act(async () => { renderer = create(React.createElement(ConversationScreen, { sessionId: 's1', viewModel, sessionRenderer: SessionRenderer })); });
  expect(renderer!.root.findByProps({ accessibilityLabel: 'execution-state' })).toBeDefined();
  expect(renderer!.root.findByProps({ accessibilityLabel: 'message-m1' })).toBeDefined();
  await act(async () => { renderer!.root.findByProps({ accessibilityLabel: '批准 Run tool' }).props.onPress(); });
  expect(decisions).toEqual([['p1', 7]]);
  await act(async () => renderer!.unmount());
});

it('preserves a failed draft and prevents a second native submit while busy', async () => {
  let release!: () => void;
  const calls: string[] = [];
  const viewModel = model({ pendingInteractions: [], send: {
    draft: () => 'hello', busy: () => true,
    submit: async (text) => { calls.push(text); await new Promise<void>((resolve) => { release = resolve; }); },
  } });
  const SessionRenderer = ({ viewModel: current }: { id: string; viewModel: ConversationViewModel }) => React.createElement(ProductConversationMessages, { viewModel: current });
  let renderer: ReturnType<typeof create>;
  await act(async () => { renderer = create(React.createElement(ConversationScreen, { sessionId: 's1', viewModel, sessionRenderer: SessionRenderer })); });
  const input = renderer!.root.findByProps({ accessibilityLabel: 'conversation-draft' });
  await act(async () => input.props.onChangeText('hello'));
  const submit = renderer!.root.findByProps({ accessibilityLabel: '发送' });
  let first!: Promise<void>;
  await act(async () => { first = submit.props.onPress(); });
  expect(calls).toEqual(['hello']);
  await act(async () => release());
  await first;
  await act(async () => renderer!.unmount());
});

it('wires attachment picker, camera and share entries with cancel and failed-record removal', async () => {
  const entries = { chooseFromLibrary: vi.fn(async () => undefined), takePhoto: vi.fn(async () => undefined), acceptSharedFile: vi.fn(async () => undefined) };
  const cancelled: string[] = [];
  const removed: string[] = [];
  let records = [
    { id: 'u1', input: { sessionID: 's1', uri: 'content://a', name: 'a.pdf', mime: 'application/pdf', size: 2 }, status: 'uploading' as const },
    { id: 'u2', input: { sessionID: 's1', uri: 'content://b', name: 'b.png', mime: 'image/png', size: 3 }, status: 'failed' as const, error: 'UPLOAD_HTTP_500' },
  ];
  const attachments = {
    entries,
    records: () => records,
    cancel: (id: string) => { cancelled.push(id); records = records.filter((item) => item.id !== id); },
    remove: (id: string) => { removed.push(id); records = records.filter((item) => item.id !== id); },
  };
  const viewModel = model({ pendingInteractions: [], capabilities: { canCancel: false, canSteer: false, canAttach: true, canVoice: false } });
  let renderer: ReturnType<typeof create>;
  await act(async () => { renderer = create(React.createElement(ConversationScreen, { sessionId: 's1', viewModel, attachments })); });
  await act(async () => renderer!.root.findByProps({ accessibilityLabel: '附件' }).props.onPress());
  expect(entries.chooseFromLibrary).toHaveBeenCalledTimes(1);
  await act(async () => renderer!.root.findByProps({ accessibilityLabel: '拍照' }).props.onPress());
  expect(entries.takePhoto).toHaveBeenCalledTimes(1);
  await act(async () => renderer!.root.findByProps({ accessibilityLabel: '取消上传 a.pdf' }).props.onPress());
  expect(cancelled).toEqual(['u1']);
  await act(async () => renderer!.root.findByProps({ accessibilityLabel: '移除 b.png' }).props.onPress());
  expect(removed).toEqual(['u2']);
  // The system-share entry is exposed on the same prop for the native intent layer.
  await act(async () => attachments.entries.acceptSharedFile());
  expect(entries.acceptSharedFile).toHaveBeenCalledTimes(1);
  await act(async () => renderer!.unmount());
});

it('hides attachment entries when the capability is off', async () => {
  const entries = { chooseFromLibrary: vi.fn(async () => undefined), takePhoto: vi.fn(async () => undefined), acceptSharedFile: vi.fn(async () => undefined) };
  const attachments = { entries, records: () => [], cancel: () => undefined, remove: () => undefined };
  const viewModel = model({ pendingInteractions: [] });
  let renderer: ReturnType<typeof create>;
  await act(async () => { renderer = create(React.createElement(ConversationScreen, { sessionId: 's1', viewModel, attachments })); });
  expect(renderer!.root.findAllByProps({ accessibilityLabel: '附件' })).toHaveLength(0);
  await act(async () => renderer!.unmount());
});

function recoveryHandle(overrides: Partial<ExecutionRecovery> = {}): ExecutionRecovery & { transitions: string[]; retries: number } {
  const transitions: string[] = [];
  const handle = {
    transitions,
    retries: 0,
    recover: async () => { handle.retries += 1; },
    appStateChange: (next: string) => { transitions.push(next); },
    getState: () => ({ state: 'idle' as const }),
    subscribe: () => () => undefined,
    dispose: () => { transitions.push('dispose'); },
    ...overrides,
  };
  return handle as ExecutionRecovery & { transitions: string[]; retries: number };
}

it('forwards AppState transitions to the recovery handle and cleans up on unmount', async () => {
  const viewModel = model({ pendingInteractions: [] });
  const recovery = recoveryHandle();
  let renderer: ReturnType<typeof create>;
  await act(async () => { renderer = create(React.createElement(ConversationScreen, { sessionId: 's1', viewModel, recovery })); });
  const appState = AppState as unknown as { emitForTest(state: string): void };
  await act(async () => { appState.emitForTest('background'); });
  await act(async () => { appState.emitForTest('active'); });
  expect(recovery.transitions).toEqual(['background', 'active']);
  await act(async () => renderer!.unmount());
  // After unmount the native subscription is gone: further transitions stop.
  await act(async () => { appState.emitForTest('active'); });
  expect(recovery.transitions).toEqual(['background', 'active', 'dispose']);
});

it('shows a retry affordance while recovery is failed and clears it once recovered', async () => {
  const viewModel = model({ pendingInteractions: [] });
  const state: { value: { state: 'idle' | 'recovering' | 'failed'; error?: unknown } } = { value: { state: 'failed', error: new Error('execution stream HTTP 404') } };
  const retries = { count: 0 };
  const stateListeners = new Set<() => void>();
  const recovery: ExecutionRecovery = {
    recover: async () => { retries.count += 1; state.value = { state: 'idle' }; stateListeners.forEach((listener) => listener()); },
    appStateChange: () => undefined,
    getState: () => state.value,
    subscribe: (listener: () => void) => { stateListeners.add(listener); return () => stateListeners.delete(listener); },
    dispose: () => undefined,
  };
  let renderer: ReturnType<typeof create>;
  await act(async () => { renderer = create(React.createElement(ConversationScreen, { sessionId: 's1', viewModel, recovery })); });
  const notice = renderer!.root.findByProps({ accessibilityLabel: 'recovery-failure' });
  expect(notice).toBeDefined();
  expect(renderer!.root.findByProps({ accessibilityLabel: '重试恢复' })).toBeDefined();
  await act(async () => { renderer!.root.findByProps({ accessibilityLabel: '重试恢复' }).props.onPress(); });
  expect(retries.count).toBe(1);
  expect(renderer!.root.findAllByProps({ accessibilityLabel: 'recovery-failure' })).toHaveLength(0);
  await act(async () => renderer!.unmount());
});
});

// ---------------------------------------------------------------------------
// W29 — hold-to-talk dictation on the conversation control panel.
// ---------------------------------------------------------------------------

function fakeDictationPort(overrides: Partial<DictationPort> = {}) {
  const calls: string[] = [];
  const impl: DictationPort = {
    async start() { return undefined; },
    async stop() { return { uri: 'file:///cache/dictation-1.m4a', durationMs: 1500 }; },
    async cancel() { return undefined; },
    async transcribe() { return '删除这个文件'; },
    ...overrides,
  };
  const port: DictationPort = {
    async start() { calls.push('start'); return impl.start(); },
    async stop() { calls.push('stop'); return impl.stop(); },
    async cancel() { calls.push('cancel'); return impl.cancel(); },
    async transcribe(uri) { calls.push(`transcribe:${uri}`); return impl.transcribe(uri); },
  };
  return { port, calls };
}

function voiceModel(send: NonNullable<ConversationViewModel['send']>): ConversationViewModel {
  return model({
    pendingInteractions: [],
    capabilities: { canCancel: false, canSteer: false, canAttach: false, canVoice: true },
    send,
  });
}

describe('ConversationScreen dictation (W29)', () => {
it('fills the draft from the transcript without sending; the edited user send goes through exactly once', async () => {
  const submitted: Array<{ text: string; requestID: string }> = [];
  const viewModel = voiceModel({
    draft: () => '', busy: () => false,
    submit: async (text, requestID) => { submitted.push({ text, requestID }); },
  });
  const { port, calls } = fakeDictationPort();
  let renderer: ReturnType<typeof create>;
  await act(async () => { renderer = create(React.createElement(ConversationScreen, { sessionId: 's1', viewModel, dictation: { port } })); });
  const hold = renderer!.root.findByProps({ accessibilityLabel: '按住说话' });
  await act(async () => { hold.props.onPressIn(); });
  expect(calls).toEqual(['start']);
  await act(async () => { hold.props.onPressOut(); });
  // Transcription completion fills the draft only: no send/submit happened.
  expect(submitted).toEqual([]);
  const input = renderer!.root.findByProps({ accessibilityLabel: 'conversation-draft' });
  expect(input.props.value).toBe('删除这个文件');
  expect(calls).toContain('transcribe:file:///cache/dictation-1.m4a');
  // The user edits the transcript and sends their latest value, once.
  await act(async () => { input.props.onChangeText('保留这个文件'); });
  await act(async () => { renderer!.root.findByProps({ accessibilityLabel: '发送' }).props.onPress(); });
  expect(submitted).toEqual([{ text: '保留这个文件', requestID: expect.any(String) }]);
  await act(async () => renderer!.unmount());
});

it('prevents a second send while the first is still in flight', async () => {
  let release!: () => void;
  const submitted: string[] = [];
  const viewModel = voiceModel({
    draft: () => '', busy: () => false,
    submit: async (text) => { submitted.push(text); await new Promise<void>((resolve) => { release = resolve; }); },
  });
  const { port } = fakeDictationPort();
  let renderer: ReturnType<typeof create>;
  await act(async () => { renderer = create(React.createElement(ConversationScreen, { sessionId: 's1', viewModel, dictation: { port } })); });
  const input = renderer!.root.findByProps({ accessibilityLabel: 'conversation-draft' });
  await act(async () => { input.props.onChangeText('hello'); });
  await act(async () => { renderer!.root.findByProps({ accessibilityLabel: '发送' }).props.onPress(); });
  await act(async () => { renderer!.root.findByProps({ accessibilityLabel: '发送' }).props.onPress(); });
  expect(submitted).toEqual(['hello']);
  await act(async () => release());
  await act(async () => renderer!.unmount());
});

it('falls back to the text input when the microphone permission is denied', async () => {
  const submitted: string[] = [];
  const viewModel = voiceModel({
    draft: () => '', busy: () => false,
    submit: async (text) => { submitted.push(text); },
  });
  const { port, calls } = fakeDictationPort({ start: async () => { throw new DictationError('MIC_PERMISSION_DENIED'); } });
  let renderer: ReturnType<typeof create>;
  await act(async () => { renderer = create(React.createElement(ConversationScreen, { sessionId: 's1', viewModel, dictation: { port } })); });
  const hold = renderer!.root.findByProps({ accessibilityLabel: '按住说话' });
  await act(async () => { hold.props.onPressIn(); });
  await act(async () => { hold.props.onPressOut(); });
  expect(renderer!.root.findByProps({ accessibilityLabel: 'dictation-error' }).props.children).toBe('麦克风权限被拒绝，已切回文本输入');
  expect(calls).toEqual(['start']); // no stop, no transcription
  // The fallback: typing and sending still works.
  const input = renderer!.root.findByProps({ accessibilityLabel: 'conversation-draft' });
  await act(async () => { input.props.onChangeText('typed fallback'); });
  await act(async () => { renderer!.root.findByProps({ accessibilityLabel: '发送' }).props.onPress(); });
  expect(submitted).toEqual(['typed fallback']);
  await act(async () => renderer!.unmount());
});

it('cancel during a hold deletes the temporary audio and ignores the stray release', async () => {
  const viewModel = voiceModel({ draft: () => '', busy: () => false, submit: async () => undefined });
  const { port, calls } = fakeDictationPort();
  let renderer: ReturnType<typeof create>;
  await act(async () => { renderer = create(React.createElement(ConversationScreen, { sessionId: 's1', viewModel, dictation: { port } })); });
  const hold = renderer!.root.findByProps({ accessibilityLabel: '按住说话' });
  await act(async () => { hold.props.onPressIn(); });
  await act(async () => { renderer!.root.findByProps({ accessibilityLabel: '取消录音' }).props.onPress(); });
  expect(calls).toContain('cancel');
  await act(async () => { hold.props.onPressOut(); });
  expect(calls.filter((call) => call.startsWith('transcribe:'))).toHaveLength(0);
  expect(renderer!.root.findByProps({ accessibilityLabel: 'conversation-draft' }).props.value).toBe('');
  await act(async () => renderer!.unmount());
});

it('backgrounding cancels an in-flight hold', async () => {
  const viewModel = voiceModel({ draft: () => '', busy: () => false, submit: async () => undefined });
  const { port, calls } = fakeDictationPort();
  let renderer: ReturnType<typeof create>;
  await act(async () => { renderer = create(React.createElement(ConversationScreen, { sessionId: 's1', viewModel, dictation: { port } })); });
  await act(async () => { renderer!.root.findByProps({ accessibilityLabel: '按住说话' }).props.onPressIn(); });
  await act(async () => { (AppState as unknown as { emitForTest(state: string): void }).emitForTest('background'); });
  expect(calls).toContain('cancel');
  await act(async () => renderer!.unmount());
});

it('hides the voice entry when the capability is off or no dictation surface is provided', async () => {
  const entries = { chooseFromLibrary: vi.fn(async () => undefined), takePhoto: vi.fn(async () => undefined), acceptSharedFile: vi.fn(async () => undefined) };
  const attachments = { entries, records: () => [], cancel: () => undefined, remove: () => undefined };
  const offViewModel = model({ pendingInteractions: [], capabilities: { canCancel: false, canSteer: false, canAttach: false, canVoice: false } });
  const { port } = fakeDictationPort();
  let renderer: ReturnType<typeof create>;
  await act(async () => { renderer = create(React.createElement(ConversationScreen, { sessionId: 's1', viewModel: offViewModel, attachments, dictation: { port } })); });
  expect(renderer!.root.findAllByProps({ accessibilityLabel: '按住说话' })).toHaveLength(0);
  await act(async () => renderer!.unmount());
  const capableViewModel = voiceModel({ draft: () => '', busy: () => false, submit: async () => undefined });
  await act(async () => { renderer = create(React.createElement(ConversationScreen, { sessionId: 's1', viewModel: capableViewModel })); });
  expect(renderer!.root.findAllByProps({ accessibilityLabel: '按住说话' })).toHaveLength(0);
  await act(async () => renderer!.unmount());
});
});

// ---------------------------------------------------------------------------
// W28 — knowledge citations and specialist result registry on the screen.
// ---------------------------------------------------------------------------

describe('ConversationScreen result resources (W28)', () => {
it('provides the authorized resource seam to the mounted message surface', async () => {
  const openCitation = vi.fn(async () => undefined);
  const viewModel = model({ pendingInteractions: [], messages: [
    { id: 'm1', role: 'assistant', text: '', blocks: [
      { id: 'b1', kind: 'tool', text: JSON.stringify({ type: 'knowledge.citation', data: { document_id: 'doc-1', chunk_ids: ['c1'] } }) },
    ] },
  ] });
  const SessionRenderer = ({ viewModel: current }: { id: string; viewModel: ConversationViewModel }) => React.createElement(ProductConversationMessages, { viewModel: current });
  let renderer: ReturnType<typeof create>;
  await act(async () => { renderer = create(React.createElement(ConversationScreen, {
    sessionId: 's1', viewModel, sessionRenderer: SessionRenderer,
    resultResources: { openCitation, openFile: async () => undefined },
  })); });
  expect(renderer!.root.findByProps({ accessibilityLabel: 'knowledge-citation-doc-1' })).toBeDefined();
  await act(async () => { renderer!.root.findByProps({ accessibilityLabel: '打开引用 知识引用' }).props.onPress(); });
  expect(openCitation).toHaveBeenCalledTimes(1);
  await act(async () => renderer!.unmount());
});

it('keeps mounting safely without the resource seam (entries explain, no fake opens)', async () => {
  const viewModel = model({ pendingInteractions: [], messages: [
    { id: 'm1', role: 'assistant', text: '', blocks: [
      { id: 'b1', kind: 'tool', text: JSON.stringify({ type: 'knowledge.citation', data: { document_id: 'doc-2', chunk_ids: ['c1'] } }) },
    ] },
  ] });
  const SessionRenderer = ({ viewModel: current }: { id: string; viewModel: ConversationViewModel }) => React.createElement(ProductConversationMessages, { viewModel: current });
  let renderer: ReturnType<typeof create>;
  await act(async () => { renderer = create(React.createElement(ConversationScreen, { sessionId: 's1', viewModel, sessionRenderer: SessionRenderer })); });
  expect(renderer!.root.findByProps({ accessibilityLabel: 'knowledge-citation-doc-2' })).toBeDefined();
  expect(renderer!.root.findAllByProps({ accessibilityRole: 'button', accessibilityLabel: '打开引用 知识引用' })).toHaveLength(0);
  await act(async () => renderer!.unmount());
});
});
