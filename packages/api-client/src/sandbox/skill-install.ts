import { createServerSentEventParser, type ParsedServerSentEvent } from '../chat/stream.ts';
import type { ClientRequest } from '../client.ts';
import type { HttpStreamResult } from '../ports.ts';

/**
 * Skill install streams (Admin-gated; internal/router/routes_infra.go:70-77):
 * - GET  .../sandbox-configs/:id/skills/:skillId/install-events — SSE progress
 *   frames that always terminate (internal/handler/sandbox_skill.go:614-734).
 * - GET  .../transcript — SSE replay+tail of the installer agent's transcript;
 *   204 while the run has no locators yet, 404 once the event log expired
 *   (sandbox_skill.go:736-835). Frames are chat-shaped StreamResponse records.
 * - GET/POST .../guidance — live guidance state and steering (202) of the
 *   displayed run (sandbox_skill.go:903-937).
 *
 * SSE consumption reuses the chat stream parser (../chat/stream.ts) and the
 * client's transport chain — no separate request layer, so auth headers,
 * tenant scope and refresh behaviour match every other call.
 */

export interface SkillInstallEvent {
  percent: number;
  stage: string;
  log?: string;
  status?: string;
  done: boolean;
}

export interface SkillInstallGuidanceMessage {
  id: string;
  content: string;
  status: 'pending' | 'injected' | 'unprocessed' | string;
}

export interface SkillInstallGuidanceState {
  accepting: boolean;
  messages: SkillInstallGuidanceMessage[];
}

/** A validated install-events frame plus its terminal flag. */
export interface ParsedSkillSseFrame {
  event: SkillInstallEvent;
  terminal: boolean;
}

export interface SkillSteerInput {
  expectedMessageId: string;
  steerId: string;
  content: string;
}

function idPart(value: string, name: string): string {
  if (typeof value !== 'string' || value.trim() === '') throw new Error(`${name} must not be empty`);
  return encodeURIComponent(value);
}

function skillBasePath(configId: string, skillId: string): string {
  return `/api/v1/sandbox-configs/${idPart(configId, 'configId')}/skills/${idPart(skillId, 'skillId')}`;
}

export function skillInstallEventsPath(configId: string, skillId: string): string {
  return `${skillBasePath(configId, skillId)}/install-events`;
}

export function skillTranscriptPath(configId: string, skillId: string): string {
  return `${skillBasePath(configId, skillId)}/transcript`;
}

export function skillGuidancePath(configId: string, skillId: string): string {
  return `${skillBasePath(configId, skillId)}/guidance`;
}

/** Validates one install-events payload; junk frames return null, never throw. */
export function parseSkillInstallEvent(value: unknown): SkillInstallEvent | null {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) return null;
  const row = value as Record<string, unknown>;
  if (typeof row.percent !== 'number' || !Number.isFinite(row.percent)) return null;
  if (typeof row.stage !== 'string') return null;
  if (typeof row.done !== 'boolean') return null;
  const event: SkillInstallEvent = { percent: row.percent, stage: row.stage, done: row.done };
  if (typeof row.log === 'string' && row.log) event.log = row.log;
  if (typeof row.status === 'string' && row.status) event.status = row.status;
  return event;
}

export function parseSkillInstallGuidanceState(value: unknown): SkillInstallGuidanceState {
  if (typeof value !== 'object' || value === null) throw new Error('guidance state must be an object');
  const row = value as Record<string, unknown>;
  if (typeof row.accepting !== 'boolean') throw new Error('guidance.accepting must be a boolean');
  if (!Array.isArray(row.messages)) throw new Error('guidance.messages must be an array');
  const messages = row.messages.map((item, index) => {
    if (typeof item !== 'object' || item === null) throw new Error(`guidance.messages[${index}] must be an object`);
    const entry = item as Record<string, unknown>;
    if (typeof entry.id !== 'string' || !entry.id) throw new Error(`guidance.messages[${index}].id must be a string`);
    if (typeof entry.content !== 'string') throw new Error(`guidance.messages[${index}].content must be a string`);
    if (typeof entry.status !== 'string') throw new Error(`guidance.messages[${index}].status must be a string`);
    return { id: entry.id, content: entry.content, status: entry.status };
  });
  return { accepting: row.accepting, messages };
}

function envelopeData(value: unknown, path: string): unknown {
  if (typeof value !== 'object' || value === null) throw new Error(`${path} response must be an object`);
  const envelope = value as Record<string, unknown>;
  if (envelope.success !== true) throw new Error(`${path} request failed`);
  return envelope.data;
}

export interface SandboxSkillInstallDeps {
  request: (input: ClientRequest) => Promise<unknown>;
  /** Streaming transport; when absent the buffered request path is used. */
  sendStream?: (input: ClientRequest) => Promise<HttpStreamResult>;
}

export function createSandboxSkillInstallApi(deps: SandboxSkillInstallDeps) {
  async function consumeSse(
    input: ClientRequest,
    onEvent: (event: ParsedServerSentEvent) => void,
  ): Promise<void> {
    const request: ClientRequest = { headers: { accept: 'text/event-stream' }, ...input };
    if (deps.sendStream) {
      const result = await deps.sendStream(request);
      if (result.status < 200 || result.status >= 300) {
        throw new Error(`Skill stream failed with HTTP ${result.status}`);
      }
      const parser = createServerSentEventParser(onEvent);
      for await (const chunk of result.chunks) parser.push(chunk);
      parser.finish();
      return;
    }
    const body = await deps.request(request);
    if (typeof body !== 'string') throw new Error('Skill stream returned a non-text body');
    const parser = createServerSentEventParser(onEvent);
    parser.push(body);
    parser.finish();
  }

  function dispatchProgress(onEvent: (frame: ParsedSkillSseFrame) => void) {
    return (parsed: ParsedServerSentEvent) => {
      if (!parsed.data) return;
      let value: unknown;
      try {
        value = JSON.parse(parsed.data);
      } catch {
        return;
      }
      const event = parseSkillInstallEvent(value);
      if (!event) return;
      onEvent({ event, terminal: event.done === true });
    };
  }

  function dispatchTranscript(onFrame: (frame: unknown) => void) {
    return (parsed: ParsedServerSentEvent) => {
      if (!parsed.data) return;
      let value: unknown;
      try {
        value = JSON.parse(parsed.data);
      } catch {
        return;
      }
      onFrame(value);
    };
  }

  return {
    /**
     * Follows one install/removal. Resolves when the server closes the stream
     * (it always terminates itself); callers only need to handle aborts.
     */
    async followInstallEvents(
      configId: string,
      skillId: string,
      onEvent: (frame: ParsedSkillSseFrame) => void,
      signal?: AbortSignal,
    ): Promise<void> {
      await consumeSse({ method: 'GET', path: skillInstallEventsPath(configId, skillId), ...(signal === undefined ? {} : { signal }) },
        dispatchProgress(onEvent));
    },

    /**
     * Follows the installer transcript. Resolves to whether content was (or
     * can be) served: 204 means "not yet" — the run has no locators — while
     * 404 means the event log is gone and the durable history is the fallback
     * (sandbox_skill.go:771-781, 794-799).
     */
    async followTranscript(
      configId: string,
      skillId: string,
      onFrame: (frame: unknown) => void,
      signal?: AbortSignal,
    ): Promise<boolean> {
      const request: ClientRequest = {
        method: 'GET',
        path: skillTranscriptPath(configId, skillId),
        ...(signal === undefined ? {} : { signal }),
      };
      if (deps.sendStream) {
        const result = await deps.sendStream({ headers: { accept: 'text/event-stream' }, ...request });
        if (result.status === 204) return false;
        if (result.status < 200 || result.status >= 300) {
          throw new Error(`skill transcript stream failed with HTTP ${result.status}`);
        }
        const parser = createServerSentEventParser(dispatchTranscript(onFrame));
        for await (const chunk of result.chunks) parser.push(chunk);
        parser.finish();
        return true;
      }
      const body = await deps.request({ headers: { accept: 'text/event-stream' }, ...request });
      if (body === undefined) return false;
      if (typeof body !== 'string') throw new Error('skill transcript stream returned a non-text body');
      const parser = createServerSentEventParser(dispatchTranscript(onFrame));
      parser.push(body);
      parser.finish();
      return true;
    },

    /** GET guidance state for the current run (Admin+; routes_infra.go:70). */
    async guidance(configId: string, skillId: string, signal?: AbortSignal): Promise<SkillInstallGuidanceState> {
      return parseSkillInstallGuidanceState(envelopeData(await deps.request({
        method: 'GET',
        path: skillGuidancePath(configId, skillId),
        ...(signal === undefined ? {} : { signal }),
      }), 'guidance'));
    },

    /**
     * POST guidance for the displayed run (Admin+; routes_infra.go:71). The
     * backend binds expected_message_id/steer_id/content and caps content at
     * 10000 (sandbox_skill.go:920-924).
     */
    async steer(configId: string, skillId: string, input: SkillSteerInput, signal?: AbortSignal): Promise<void> {
      const expectedMessageId = idPart(input.expectedMessageId, 'expectedMessageId');
      const steerId = idPart(input.steerId, 'steerId');
      const content = typeof input.content === 'string' ? input.content : '';
      if (!content.trim()) throw new Error('content must not be empty');
      if (content.length > 10_000) throw new Error('content must not exceed 10000 characters');
      const envelope = await deps.request({
        method: 'POST',
        path: skillGuidancePath(configId, skillId),
        body: { expected_message_id: expectedMessageId, steer_id: steerId, content },
        ...(signal === undefined ? {} : { signal }),
      });
      if (typeof envelope !== 'object' || envelope === null || (envelope as Record<string, unknown>).success !== true) {
        throw new Error('steer request failed');
      }
    },
  };
}

export type SandboxSkillInstallApi = ReturnType<typeof createSandboxSkillInstallApi>;
