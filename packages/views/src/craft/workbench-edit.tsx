// T09 (#135) workbench edit panel: requesting a serialized edit is a server
// authority this panel only PROJECTS. The server derives the initiating
// member's CURRENT Task role on every request — a Viewer never sees the
// composer, and a Collaborator's request runs under their own grant in the
// same Workspace behind the T16 writer lease. The panel surfaces the
// server's serialization answers verbatim: the writer-acquisition conflict
// (another writing Run holds the Workspace) and the ACTUAL initiating member
// the server recorded on the Task timeline. It never derives authority,
// identity or conflict state client-side, and a refused request keeps the
// draft for an explicit retry.
import React, { useState } from 'react';

// T00 frozen writer-acquisition facts (packages/contracts/src/craft/
// web-artifact.ts, CRAFT_WRITER_ACQUIRE_OUTCOMES). The central package
// top-level index does not re-export them yet, so this panel mirrors the
// frozen spellings locally and validates with the same closed vocabulary —
// an unknown status is rejected, never defaulted.
const CRAFT_WRITER_ACQUIRE_STATUSES = ['acquired', 'conflict', 'unknown'] as const;
export type CraftWriterAcquireStatus = (typeof CRAFT_WRITER_ACQUIRE_STATUSES)[number];
export interface CraftWriterAcquireOutcome { workspace_id: string; status: CraftWriterAcquireStatus }

function parseWriterAcquisition(value: unknown): CraftWriterAcquireOutcome | null {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) return null;
  const raw = value as Record<string, unknown>;
  if (typeof raw.workspace_id !== 'string' || raw.workspace_id.trim() === '') return null;
  if (typeof raw.status !== 'string' || !(CRAFT_WRITER_ACQUIRE_STATUSES as readonly string[]).includes(raw.status)) return null;
  return { workspace_id: raw.workspace_id, status: raw.status as CraftWriterAcquireStatus };
}

/** One admitted serialized-edit request, as answered by the server. */
export interface CraftEditRequestOutcome {
  /** The stable Run ID of the admitted serialized edit. */
  runId: string;
  /** The ACTUAL initiating member the server recorded (never a client guess). */
  actorUserId: string;
  /** The T00 frozen writer-acquisition outcome; null when the server did not answer one. */
  writerAcquisition: CraftWriterAcquireOutcome | null;
}

/**
 * Projects one raw edit-request outcome. Malformed payloads (empty ids, an
 * unknown writer status) are dropped instead of guessed — the panel never
 * defaults a conflict into an acquired lease or invents an initiator.
 */
export function projectEditOutcome(value: unknown): CraftEditRequestOutcome | null {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) return null;
  const raw = value as Record<string, unknown>;
  if (typeof raw.runId !== 'string' || raw.runId.trim() === '') return null;
  if (typeof raw.actorUserId !== 'string' || raw.actorUserId.trim() === '') return null;
  let writerAcquisition: CraftWriterAcquireOutcome | null = null;
  if (raw.writerAcquisition !== undefined && raw.writerAcquisition !== null) {
    writerAcquisition = parseWriterAcquisition(raw.writerAcquisition);
    if (writerAcquisition === null) return null;
  }
  return { runId: raw.runId, actorUserId: raw.actorUserId, writerAcquisition };
}

export interface CraftEditRequestPanelProps {
  locale: 'zh' | 'en';
  /** Server-derived: true only for a member whose CURRENT role may write (owner or collaborator). */
  canWrite: boolean;
  /** True while a writing Run already holds the Task's single run slot. */
  runActive: boolean;
  /** Sends one serialized-edit request; the assembly owns the API call and the request id. */
  onRequestEdit(prompt: string): Promise<CraftEditRequestOutcome>;
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
    failed: '修改请求失败',
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
    failed: 'Edit request failed',
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
        throw new Error(props.locale === 'zh' ? '服务器响应无效' : 'Invalid server response');
      }
      setOutcome(projected);
      setDraft('');
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
      {outcome !== null && error === null ? (
        <p className="wk-craft-hint" role="status" data-testid="craft-edit-outcome">
          {strings.runLabel}: <code>{outcome.runId}</code> · {strings.initiatedBy}: <code>{outcome.actorUserId}</code>
        </p>
      ) : null}
    </section>
  );
}
