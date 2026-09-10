import type { FormEvent } from 'react';

export interface ChatSubmission {
  content: string;
  status: 'pending';
}

export function createChatSubmission(draft: string): ChatSubmission {
  const content = draft.trim();
  if (!content) throw new Error('message must not be empty');
  return { content, status: 'pending' };
}

export interface ChatComposerProps {
  draft: string;
  disabled?: boolean;
  onDraftChange(value: string): void;
  onSubmit(submission: ChatSubmission): void;
}

export function ChatComposer({ draft, disabled = false, onDraftChange, onSubmit }: ChatComposerProps) {
  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    onSubmit(createChatSubmission(draft));
  }

  return <form className="wk-chat-composer" onSubmit={submit}>
    <label htmlFor="wk-chat-draft">Message</label>
    <textarea
      id="wk-chat-draft"
      value={draft}
      onChange={(event) => onDraftChange(event.target.value)}
      disabled={disabled}
      rows={4}
    />
    <button type="submit" disabled={disabled || !draft.trim()}>Send</button>
  </form>;
}
