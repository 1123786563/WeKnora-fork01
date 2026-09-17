import * as React from 'react';
// @ts-expect-error react-test-renderer has no declarations in this workspace.
import { act, create } from 'react-test-renderer';
import { AppState } from 'react-native';
import { describe, expect, it, vi } from 'vitest';
import { ConversationScreen, ProductConversationMessages } from './ConversationScreen';
import type { ConversationViewModel } from './view-model';
import type { ExecutionRecovery } from '../executions/recovery';

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
