import type { ResearchAnnotationRow, ResearchBackendPort, ResearchDelegationRow } from './types.ts';

/** 场景 Adapter（module-seams §4.4/§7.3 同款）：脚本化委派与批注，供 Interface 级
 *  测试与 composition 的离线替身使用；绝不冒充真实集成证据。 */

export interface ScenarioResearchRemoteScript {
  delegations: Array<Partial<ResearchDelegationRow> & { objective: string; sources: string[] }>;
  annotations?: Array<{ materialId: string; baseVersion: string; body: string; authorId?: string }>;
  /** 置真后 annotate 恒以 RESEARCH_BASE_VERSION_CONFLICT 失败（冲突分支脚本）。 */
  conflictOnAnnotate?: boolean;
}

export function createScenarioResearchRemote(script: ScenarioResearchRemoteScript): ResearchBackendPort {
  let delegationSeq = 0;
  let annotationSeq = 0;
  const recorded: ResearchAnnotationRow[] = (script.annotations ?? []).map((row, index) => ({
    annotationId: `an-scenario-${index + 1}`, runId: 'r-scenario', materialId: row.materialId,
    baseVersion: row.baseVersion, body: row.body, authorId: row.authorId ?? 'u-scenario',
    createdAt: '2026-09-26T00:00:00Z',
  }));
  const delegationRows: ResearchDelegationRow[] = script.delegations.map((row, index) => ({
    delegationId: row.delegationId ?? `d-scenario-${index + 1}`,
    runId: row.runId ?? 'r-scenario',
    sessionId: row.sessionId ?? 's-scenario',
    objective: row.objective,
    sources: row.sources,
    status: row.status ?? 'assigned',
    summary: row.summary,
    createdAt: row.createdAt ?? '2026-09-26T00:00:00Z',
  }));
  return {
    async delegate(input) {
      delegationSeq += 1;
      const row: ResearchDelegationRow = {
        delegationId: `d-live-${delegationSeq}`, runId: input.runId, sessionId: 's-live',
        objective: input.objective, sources: input.sources, status: 'assigned', createdAt: new Date().toISOString(),
      };
      delegationRows.push(row);
      return row;
    },
    async list(runId) {
      return { runId, delegations: delegationRows.filter((row) => row.runId === runId) };
    },
    async complete(input) {
      const row = delegationRows.find((candidate) => candidate.delegationId === input.delegationId);
      if (row === undefined) throw new Error('RESEARCH_NOT_FOUND');
      row.status = 'completed';
      row.summary = input.summary;
      return row;
    },
    async annotate(input) {
      if (script.conflictOnAnnotate === true || input.baseVersion === 'stale') throw new Error('RESEARCH_BASE_VERSION_CONFLICT');
      annotationSeq += 1;
      const row: ResearchAnnotationRow = {
        annotationId: `an-live-${annotationSeq}`, runId: input.runId, materialId: input.materialId,
        baseVersion: input.baseVersion, body: input.body, authorId: 'u-live', createdAt: new Date().toISOString(),
      };
      recorded.push(row);
      return row;
    },
    async annotations(runId) {
      return { runId, annotations: recorded.filter((row) => row.runId === runId) };
    },
  };
}
