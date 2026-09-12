/**
 * Send-run preparation for the chat composer.
 *
 * Ordering invariant: when a send has to create the session inline (the
 * creatChat first-message flow), the session-create await and the caller's
 * selectSession() side effects (history push, prior-stream teardown via
 * streamAbortRef) MUST complete before the stream AbortController is created.
 * selectSession aborts whatever controller is currently registered in
 * streamAbortRef, so a controller installed before that point would hand the
 * new stream an already-aborted signal: the fetch rejects without ever
 * issuing the HTTP request, and send()'s catch swallows the rejection as an
 * intentional abort — a silent hang with no error surfaced.
 */
export interface SendRun<TSession> {
  sessionId: string;
  controller: AbortController;
  /** Set only when this run created the session inline. */
  createdSession: TSession | null;
}

export async function prepareSendRun<TSession>(input: {
  selectedSessionId: string | null;
  content: string;
  createSession: (title: string) => Promise<TSession & { id: string }>;
  /** Runs the selectSession side effects for the freshly created session. */
  onSessionSelected: (session: TSession & { id: string }) => void;
}): Promise<SendRun<TSession>> {
  let sessionId = input.selectedSessionId;
  let createdSession: (TSession & { id: string }) | null = null;
  if (!sessionId) {
    createdSession = await input.createSession(input.content.slice(0, 80));
    input.onSessionSelected(createdSession);
    sessionId = createdSession.id;
  }
  // Created (and registered) strictly after the selectSession teardown so the
  // signal is never aborted by the run that owns it.
  return { sessionId, controller: new AbortController(), createdSession };
}
