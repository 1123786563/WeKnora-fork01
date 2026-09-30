// T09 (#135) workbench edit panel: requesting a serialized edit is a server
// authority this panel only PROJECTS. The server derives the initiating
// member's CURRENT Task role on every request — a Viewer never sees the
// composer, and a Collaborator's request runs under their own grant in the
// same Workspace behind the T16 writer lease. The panel surfaces the
// server's serialization answers verbatim: the writer-acquisition conflict
// (another writing Run holds the Workspace), an unknown lease outcome (never
// painted as acquired) and the ACTUAL initiating member the server recorded
// on the admitted run (initiated_by — absent on wires that predate it, and
// then simply not shown). It never derives authority, identity or conflict
// state client-side, and a refused or conflicted request keeps the draft
// for an explicit retry.
import React, { useState } from 'react';
import {
  parseCraftWriterAcquireOutcome,
  type CraftWriterAcquireOutcome,
} from '@weknora/contracts';

/** One admitted serialized-edit request, projected from the run-response wire. */
export interface CraftEditRequestOutcome {
  /** The stable Run ID of the admitted serialized edit (wire: data.run_id). */
  runId: string;
  /** The ACTUAL initiating member the server recorded (wire: initiated_by);
   * null when the wire does not carry one — never a client guess. */
  actorUserId: string | null;
  /** The T00 frozen writer-acquisition outcome (wire: writer_acquisition,
   * the { workspace_id, status } object); null when the server did not
   * answer one. */
  writerAcquisition: CraftWriterAcquireOutcome | null;
}

/** Parses one writer-acquisition wire object with the frozen central parser;
 * null (never a default) when the payload is malformed. */
function parseWriterAcquisition(value: unknown): CraftWriterAcquireOutcome | null {
  try {
    return parseCraftWriterAcquireOutcome(value);
  } catch {
    return null;
  }
}

/**
 * Projects one raw run-response wire payload ({ data: { run_id, ... },
 * writer_acquisition, initiated_by } — the PostCraftRun envelope, snake_case
 * like every frozen craft wire). Malformed payloads (a missing envelope, an
 * empty run id, an unknown writer status) are dropped instead of guessed —
 * the panel never defaults a conflict into an acquired lease or invents an
 * initiator.
 */
export function projectEditOutcome(value: unknown): CraftEditRequestOutcome | null {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) return null;
  const raw = value as Record<string, unknown>;
  if (typeof raw.data !== 'object' || raw.data === null || Array.isArray(raw.data)) return null;
  const data = raw.data as Record<string, unknown>;
  if (typeof data.run_id !== 'string' || data.run_id.trim() === '') return null;
  let writerAcquisition: CraftWriterAcquireOutcome | null = null;
  if (raw.writer_acquisition !== undefined && raw.writer_acquisition !== null) {
    writerAcquisition = parseWriterAcquisition(raw.writer_acquisition);
    if (writerAcquisition === null) return null;
  }
  let actorUserId: string | null = null;
  if (raw.initiated_by !== undefined && raw.initiated_by !== null) {
    if (typeof raw.initiated_by !== 'string' || raw.initiated_by.trim() === '') return null;
    actorUserId = raw.initiated_by;
  }
  return { runId: data.run_id, actorUserId, writerAcquisition };
}

export interface CraftEditRequestPanelProps {
  locale: 'zh' | 'en';
  /** Server-derived: true only for a member whose CURRENT role may write (owner or collaborator). */
  canWrite: boolean;
  /** True while a writing Run already holds the Task's single run slot. */
  runActive: boolean;
  /** Sends one serialized-edit request and resolves with the RAW run-response
   * wire payload (unknown): the assembly owns the API call and the request
   * id, this panel holds the single projection-and-rejection point. */
  onRequestEdit(prompt: string): Promise<unknown>;
}

const EDIT_STRINGS = {
  zh: {
    title: '请求修改',
    readOnly: '当前身份为只读成员，不能发起修改',
    placeholder: '描述这次要做的修改…',
    submit: '发起修改',
    busy: '提交中…',
    runActive: '已有一个修改任务正在执行，新的修改请求会被拒绝；等待其完成后再发起',
    initiatedBy: '发起成员',
    runLabel: '运行',
    conflict: '另一个写入任务正在占用工作区，本次修改不会写入；请等待其完成后再试',
    unknownLease: '写入租约结果不明，本次修改是否会写入待服务端确认',
    failed: '修改请求失败',
    invalidResponse: '服务器响应无效',
  },
  en: {
    title: 'Request an edit',
    readOnly: 'You are a read-only member of this task and cannot request edits',
    placeholder: 'Describe the change you want…',
    submit: 'Request edit',
    busy: 'Submitting…',
    runActive: 'An edit run is already in progress; a new request would be refused — wait for it to finish',
    initiatedBy: 'Initiated by',
    runLabel: 'Run',
    conflict: 'Another writing run holds the workspace; this edit will not write. Retry after it finishes',
    unknownLease: 'The writer-lease outcome is unknown; whether this edit will write awaits server confirmation',
    failed: 'Edit request failed',
    invalidResponse: 'Invalid server response',
  },
} as const;

/** The serialized-edit request panel (T09 #135). Server-authoritative. */
export function CraftEditRequestPanel(props: CraftEditRequestPanelProps) {
  const strings = props.locale === 'zh' ? EDIT_STRINGS.zh : EDIT_STRINGS.en;
  const [draft, setDraft] = useState('');
  const [sending, setSending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [outcome, setOutcome] = useState<CraftEditRequestOutcome | null>(null);

  if (!props.canWrite) {
    // A Viewer (or any member without a current write grant) never sees the
    // composer; the request callback is unreachable in this branch.
    return (
      <section className="wk-craft-card" aria-label={strings.title}>
        <h4>{strings.title}</h4>
        <p className="wk-craft-hint" data-testid="craft-edit-readonly">{strings.readOnly}</p>
      </section>
    );
  }

  const conflict = outcome?.writerAcquisition?.status === 'conflict';
  const unknownLease = outcome?.writerAcquisition?.status === 'unknown';

  const submit = async (event: React.FormEvent<HTMLFormElement>): Promise<void> => {
    event.preventDefault();
    const prompt = draft.trim();
    if (prompt === '' || sending || props.runActive) return;
    setSending(true);
    setError(null);
    try {
      const raw = await props.onRequestEdit(prompt);
      const projected = projectEditOutcome(raw);
      if (projected === null) {
        throw new Error(strings.invalidResponse);
      }
      setOutcome(projected);
      // Only a lease-conflicted request keeps the draft: the conflict notice
      // guides an explicit retry, so the prompt must survive it. Acquired and
      // unknown answers mean the request went through — the composer resets.
      if (projected.writerAcquisition?.status !== 'conflict') {
        setDraft('');
      }
    } catch (requestError) {
      // The refused prompt returns to the composer for an explicit retry.
      setOutcome(null);
      setError(requestError instanceof Error ? requestError.message : String(requestError));
    } finally {
      setSending(false);
    }
  };

  return (
    <section className="wk-craft-card" aria-label={strings.title}>
      <h4>{strings.title}</h4>
      <form onSubmit={(event) => { void submit(event); }}>
        <label className="wk-craft-hint" htmlFor="craft-edit-prompt">{strings.placeholder}</label>
        <textarea
          id="craft-edit-prompt"
          data-testid="craft-edit-prompt"
          value={draft}
          placeholder={strings.placeholder}
          disabled={sending}
          onChange={(event) => setDraft(event.target.value)}
        />
        <button type="submit" data-testid="craft-edit-submit" disabled={sending || props.runActive || draft.trim() === ''}>
          {sending ? strings.busy : strings.submit}
        </button>
      </form>
      {props.runActive ? <p className="wk-craft-hint" role="status">{strings.runActive}</p> : null}
      {error !== null ? <p className="wk-craft-error" role="alert">{strings.failed}: {error}</p> : null}
      {conflict ? <p className="wk-craft-error" role="alert">{strings.conflict}</p> : null}
      {unknownLease && error === null ? <p className="wk-craft-hint" role="status" data-testid="craft-edit-unknown-lease">{strings.unknownLease}</p> : null}
      {outcome !== null && error === null ? (
        <p className="wk-craft-hint" role="status" data-testid="craft-edit-outcome">
          {strings.runLabel}: <code>{outcome.runId}</code>
          {outcome.actorUserId !== null ? <> · {strings.initiatedBy}: <code>{outcome.actorUserId}</code></> : null}
        </p>
      ) : null}
    </section>
  );
}
