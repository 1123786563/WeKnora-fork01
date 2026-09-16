/**
 * Empty-state suggested questions for the new-conversation (creatChat) view.
 *
 * Backend surface: GET /api/v1/agents/:id/suggested-questions (verified in
 * internal/router/routes_agent.go). The main chat API has NO session-level
 * suggested-questions route — only the embed channel does — so starters are
 * only available when a specific agent is selected. Any fetch error
 * (including a missing agent or 404) degrades to no suggestions.
 */

export interface AgentQuestionsApi {
  suggestedQuestions(agentId: string, options?: { signal?: AbortSignal }): Promise<string[]>;
}

export function starterQuestionTarget(selectedSessionId: string | null, agentId: string | undefined): string | null {
  if (selectedSessionId) return null;
  const id = agentId?.trim();
  return id ? id : null;
}

export async function loadStarterQuestions(
  agentsApi: AgentQuestionsApi,
  selectedSessionId: string | null,
  agentId: string | undefined,
  signal?: AbortSignal,
): Promise<string[]> {
  const target = starterQuestionTarget(selectedSessionId, agentId);
  if (!target) return [];
  try {
    const questions = await agentsApi.suggestedQuestions(target, { signal });
    return Array.isArray(questions) ? questions : [];
  } catch {
    return [];
  }
}
