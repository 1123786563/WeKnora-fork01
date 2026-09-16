import * as React from 'react';
// @ts-expect-error react-test-renderer has no declarations in this workspace.
import { act, create } from 'react-test-renderer';
import { describe, expect, it, vi } from 'vitest';
import { ConversationScreen, ProductConversationMessages } from './ConversationScreen';
import type { ConversationViewModel } from './view-model';

vi.mock('react-native', async () => {
  const ReactModule = await import('react');
  const host = (name: string) => (props: any) => ReactModule.createElement(name, props, props.children);
  return { Pressable: host('Pressable'), Text: host('Text'), TextInput: host('TextInput'), View: host('View'), ScrollView: host('ScrollView') };
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
});
