/**
 * Skill install progress + transcript timeline reduction.
 *
 * Progress events mirror the backend SSE frame skillInstallEvent
 * (internal/handler/sandbox_skill.go:554-563): the stream always terminates,
 * with the run's own terminal event (stage done|failed), one derived from the
 * durable status, or a "detached" frame when the server stops following a run
 * that is still going. Done is carried explicitly so clients terminate on the
 * flag rather than on a stage name (sandbox_skill.go:554-556).
 *
 * Timeline frames are the transcript endpoint's chat-shaped StreamResponse
 * frames (sandbox_skill.go:839-863 emitTranscript); install_prompt is the
 * installer's opening line and becomes the user turn (SkillInstallTimeline.vue
 * applyPrompt), everything else feeds the compact agent run view.
 */

/** One install-events SSE frame (backend skillInstallEvent). */
export interface SkillInstallProgressEvent {
  percent: number;
  stage: string;
  log?: string;
  status?: string;
  done: boolean;
}

/** A transcript frame reduced into the compact install timeline view. */
export interface SkillTimelineToolCall {
  id: string;
  name?: string;
  status: 'pending' | 'completed' | 'failed';
  result?: unknown;
}

export interface SkillTimelineState {
  /** The installer's opening line, rendered as the timeline's user turn. */
  prompt: string;
  thinking: string;
  answer: string;
  toolCalls: SkillTimelineToolCall[];
  /** A complete frame arrived — the run is over and was not an error. */
  complete: boolean;
  error: string;
  /** Number of recognized transcript frames; 0 means the empty state. */
  frames: number;
}

function text(value: unknown): string {
  return typeof value === 'string' ? value : '';
}

function framePayload(frame: Record<string, unknown>): Record<string, unknown> {
  return typeof frame.data === 'object' && frame.data !== null ? frame.data as Record<string, unknown> : frame;
}

/** Parses one install-events SSE data payload; junk frames are dropped (null), never thrown. */
export function parseSkillInstallProgressFrame(data: string): SkillInstallProgressEvent | null {
  let value: unknown;
  try {
    value = JSON.parse(data);
  } catch {
    return null;
  }
  if (typeof value !== 'object' || value === null || Array.isArray(value)) return null;
  const row = value as Record<string, unknown>;
  if (typeof row.percent !== 'number' || !Number.isFinite(row.percent)) return null;
  if (typeof row.stage !== 'string') return null;
  if (typeof row.done !== 'boolean') return null;
  const event: SkillInstallProgressEvent = { percent: row.percent, stage: row.stage, done: row.done };
  const log = text(row.log);
  const status = text(row.status);
  if (log) event.log = log;
  if (status) event.status = status;
  return event;
}

/** done=true covers the done/failed verdicts and the handler's detached frame. */
export function isTerminalInstallProgress(event: SkillInstallProgressEvent): boolean {
  return event.done === true;
}

export function clampInstallPercent(percent: number): number {
  if (!Number.isFinite(percent)) return 0;
  return Math.max(0, Math.min(100, percent));
}

/**
 * Vue progressOf (SandboxSkillsPanel.vue:1017-1024): the live event's clamped
 * percent wins; without one, a removal shows the fixed "accepted" 5%, a
 * finished run shows 100%, and everything else starts at 0.
 */
export function installProgressPercent(
  event: SkillInstallProgressEvent | undefined,
  skillStatus: string,
  deleting = false,
): number {
  if (event && Number.isFinite(event.percent)) return clampInstallPercent(event.percent);
  if (skillStatus === 'removing' || deleting) return 5;
  if (skillStatus === 'ready' || skillStatus === 'failed') return 100;
  return 0;
}

export function initialSkillTimelineState(): SkillTimelineState {
  return { prompt: '', thinking: '', answer: '', toolCalls: [], complete: false, error: '', frames: 0 };
}

/**
 * Folds one transcript frame into the timeline. install_prompt only sets the
 * prompt once (SkillInstallTimeline.vue applyPrompt keeps a single user turn);
 * thinking/answer append; tool_call/tool_result track the step lifecycle.
 */
export function reduceSkillTimelineFrame(state: SkillTimelineState, frame: unknown): SkillTimelineState {
  if (typeof frame !== 'object' || frame === null || Array.isArray(frame)) return state;
  const row = frame as Record<string, unknown>;
  const kind = text(row.response_type) || text(row.type);
  if (!kind) return state;
  const data = framePayload(row);
  const content = text(row.content);
  const next: SkillTimelineState = { ...state, toolCalls: state.toolCalls.slice() };
  switch (kind) {
    case 'install_prompt':
      if (next.prompt) return state;
      next.prompt = content;
      next.frames += 1;
      return next;
    case 'thinking':
      next.thinking += content;
      next.frames += 1;
      return next;
    case 'answer':
      next.answer += content;
      next.frames += 1;
      return next;
    case 'tool_call': {
      const id = text(data.tool_call_id) || text(row.tool_call_id);
      if (!id) return state;
      if (next.toolCalls.some((call) => call.id === id)) return state;
      next.toolCalls.push({ id, ...(text(data.tool_name) || text(row.tool_name) ? { name: text(data.tool_name) || text(row.tool_name) } : {}), status: 'pending' });
      next.frames += 1;
      return next;
    }
    case 'tool_result': {
      const id = text(data.tool_call_id) || text(row.tool_call_id);
      if (!id) return state;
      const index = next.toolCalls.findIndex((call) => call.id === id);
      if (index < 0) return state;
      const failed = data.is_error === true || row.is_error === true;
      next.toolCalls[index] = { ...next.toolCalls[index]!, status: failed ? 'failed' : 'completed', result: data.result ?? row.result };
      next.frames += 1;
      return next;
    }
    case 'complete':
      next.complete = true;
      next.frames += 1;
      return next;
    case 'error':
      next.error = content;
      next.frames += 1;
      return next;
    default:
      return state;
  }
}
