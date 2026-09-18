// CFT-S00-T006: the real @assistant-ui/react mount for craft (locked 0.15.20).
//
// This replaces the old G2 decision (plain React 19 mimicking ExternalStore
// semantics — see workbench.tsx history) with the SDK itself. The W05
// projection stays the single source of truth: the message log and the
// controller's main-run state already own every fact, so the adapter below
// only PROJECTS them:
//
//   * messages  — stable ids derived from the run/turn identity; replaying a
//                 reconnect never mints new ids (A9 invariant).
//   * isRunning — flows ONLY from the caller's main-run projection. A child
//                 delegation finishing can never flip it; the browser never
//                 decides the main run's success or failure.
//   * onNew     — routes the composer's text to the caller's send port (the
//                 T002/T008 command bridge), NOT to any chat API.
//   * onCancel  — the controller's cancel port when provided.
//
// The minimal Thread below composes the SDK primitives; T007/T010 build the
// full craft conversation UI on top of this provider.
import React, { useSyncExternalStore, type ReactNode } from 'react';
import {
  AssistantRuntimeProvider,
  ComposerPrimitive,
  MessagePrimitive,
  ThreadPrimitive,
  useExternalStoreRuntime,
  type AssistantRuntime,
  type ThreadMessageLike,
} from '@assistant-ui/react';

/** One archived or live conversation row, as the workbench already derives it. */
export interface CraftThreadRow {
  prompt: string;
  assistantText: string;
  /** True ONLY when the main-run projection is terminal (W05 invariant). */
  assistantComplete: boolean;
}

/** Stable id scheme: the run owns its turn; archived turns number by index. */
export function craftThreadMessages(input: {
  runId: string;
  prompt: string | null;
  assistantText: string;
  assistantComplete: boolean;
  archivedTurns: readonly CraftThreadRow[];
}): ThreadMessageLike[] {
  const messages: ThreadMessageLike[] = [];
  input.archivedTurns.forEach((row, index) => {
    const n = index + 1;
    if (row.prompt) messages.push({ id: `turn-${n}-user`, role: 'user', content: row.prompt });
    messages.push({
      id: `turn-${n}-assistant`,
      role: 'assistant',
      content: row.assistantText,
      status: row.assistantComplete
        ? { type: 'complete', reason: 'stop' }
        : { type: 'running' },
    });
  });
  if (input.prompt !== null) {
    messages.push({ id: `${input.runId}-user`, role: 'user', content: input.prompt });
  }
  if (input.assistantText !== '' || input.prompt !== null) {
    messages.push({
      id: `${input.runId}-assistant`,
      role: 'assistant',
      content: input.assistantText,
      status: input.assistantComplete
        ? { type: 'complete', reason: 'stop' }
        : { type: 'running' },
    });
  }
  return messages;
}

export interface CraftAssistantStoreLike {
  subscribe(listener: () => void): () => void;
  getSnapshot(): { text: string; complete: boolean };
}

export interface UseCraftAssistantRuntimeInput {
  /** The W05 external store (or a test fixture with the same contract). */
  store: CraftAssistantStoreLike;
  /** Main-run projection ONLY — never a child-delegation status. */
  isRunning: boolean;
  /** The command bridge's submitDraft port. */
  onNew(text: string): Promise<void> | void;
  /** The controller's cancel port, when the session supports it. */
  onCancel?(): Promise<void> | void;
  /** Read-only sessions disable the composer input entirely. */
  isDisabled?: boolean;
  prompt: string | null;
  runId: string;
  archivedTurns?: readonly CraftThreadRow[];
}

export function useCraftAssistantRuntime(input: UseCraftAssistantRuntimeInput): AssistantRuntime {
  const snapshot = useSyncExternalStore(input.store.subscribe, input.store.getSnapshot, input.store.getSnapshot);
  const messages = craftThreadMessages({
    runId: input.runId,
    prompt: input.prompt,
    assistantText: snapshot.text,
    assistantComplete: snapshot.complete,
    archivedTurns: input.archivedTurns ?? [],
  });
  return useExternalStoreRuntime({
    isRunning: input.isRunning,
    isDisabled: input.isDisabled ?? false,
    messages,
    // ThreadMessageLike inputs must go through the SDK's fromThreadMessageLike
    // completion (ids, metadata, timing); without a converter the runtime
    // adopts the rows verbatim and its state getter crashes on missing
    // metadata. Identity is enough — craftThreadMessages already builds the
    // full ThreadMessageLike shape.
    convertMessage: (m: ThreadMessageLike) => m,
    onNew: async (message) => {
      const text = typeof message.content === 'string' ? message.content : '';
      if (text.trim() === '') return;
      await input.onNew(text);
    },
    ...(input.onCancel ? { onCancel: async () => void (await input.onCancel?.()) } : {}),
  });
}

export interface CraftAssistantThreadProps extends UseCraftAssistantRuntimeInput {
  /** Rendered while the thread has no messages. */
  empty?: ReactNode;
}

/**
 * The minimal REAL mount: provider + thread viewport + messages + composer,
 * all composed from the SDK primitives. Styling hooks use the wk-craft-*
 * classes so T003's scoped tokens apply; no lookalike DOM is hand-rolled.
 */
export function CraftAssistantThread(props: CraftAssistantThreadProps) {
  const runtime = useCraftAssistantRuntime(props);
  return (
    <AssistantRuntimeProvider runtime={runtime}>
      <ThreadPrimitive.Root className="wk-craft-thread">
        <ThreadPrimitive.Viewport className="wk-craft-thread-viewport">
          <ThreadPrimitive.Empty>{props.empty}</ThreadPrimitive.Empty>
          <ThreadPrimitive.Messages
            components={{
              UserMessage: () => (
                <MessagePrimitive.Root className="wk-craft-msg wk-craft-msg-user">
                  <MessagePrimitive.Content components={{ Text: TextPart }} />
                </MessagePrimitive.Root>
              ),
              AssistantMessage: () => (
                <MessagePrimitive.Root className="wk-craft-msg wk-craft-msg-assistant">
                  <MessagePrimitive.Content components={{ Text: TextPart }} />
                </MessagePrimitive.Root>
              ),
            }}
          />
        </ThreadPrimitive.Viewport>
        <ComposerPrimitive.Root className="wk-craft-composer">
          <ComposerPrimitive.Input
            className="wk-craft-composer-input"
            aria-label="craft composer"
            placeholder="输入修改要求…"
          />
          <ComposerPrimitive.Send className="wk-craft-composer-send">发送</ComposerPrimitive.Send>
        </ComposerPrimitive.Root>
      </ThreadPrimitive.Root>
    </AssistantRuntimeProvider>
  );
}

function TextPart({ text }: { text: string }) {
  return <span>{text}</span>;
}
