import { parseAnswerEvidence, responseType, type AnswerEvidenceWire, type EvidenceCitationWire, type EvidenceConclusionWire } from '@weknora/contracts';
import { createServerSentEventParser, parseChatEvent } from '../chat/stream.ts';
import type { ClientRequest } from '../client.ts';
import { requireDeploymentOrigin } from './deployment-origin.ts';

/**
 * T15（Issue #45）：knowledge-chat SSE 的 Knowledge QA Remote。POST
 * /api/v1/knowledge-chat/:session_id，消费 evidence/answer/终止帧；evidence 帧缺失、
 * 流截断或信封不自洽一律 fail closed——不得把无证据回答冒充有据回答（module-seams §5.3）。
 * api-client 不依赖 mobile-core（与 legacy remote 同例）：下列 DTO 与 mobile-core
 * 同名类型结构逐字一致，结构可赋值由 apps/mobile typecheck 证明。
 */

export interface KnowledgeQARemoteOptions {
	/** 部署 Origin：构造即强校验（绝对 HTTPS、无 path/query/fragment、无内嵌凭据）。 */
	origin: string;
	/** 授权读通道（MobileRuntime.authorizedRequest 或测试替身）；本适配器不新建传输。 */
	request: (input: ClientRequest) => Promise<unknown>;
	/** 授权 SSE 通道（MobileRuntime.authorizedEventStream 或测试替身）。 */
	stream: (input: ClientRequest, onChunk: (chunk: string) => void) => Promise<void>;
}

export interface RemoteEvidenceCitation {
	citationId: string;
	knowledgeId: string;
	knowledgeBaseId: string;
	revision: number;
	kind: 'fact';
	retrievedAt: string;
	title?: string;
	quote?: string;
	startAt?: number;
	endAt?: number;
}

export interface RemoteEvidenceConclusion {
	kind: 'fact' | 'rule_derived' | 'model_inferred';
	modelId?: string;
	ruleIds?: string[];
	citationIds?: string[];
}

export interface RemoteEvidenceReasoning {
	requested: boolean;
	mode?: string;
	state: 'not_requested' | 'incomplete';
	reason?: string;
	retryable: boolean;
}

export interface RemoteAnswerEvidence {
	state: 'cited' | 'no_evidence' | 'revoked';
	semanticGraphUsed: boolean;
	retrievedAt: string;
	citations: RemoteEvidenceCitation[];
	conclusions: RemoteEvidenceConclusion[];
	reasoning: RemoteEvidenceReasoning;
}

export interface RemoteKnowledgeQATurn {
	answer: string;
	isFallback: boolean;
	evidence: RemoteAnswerEvidence;
}

function citationOf(citation: EvidenceCitationWire): RemoteEvidenceCitation {
	return {
		citationId: citation.citation_id,
		knowledgeId: citation.knowledge_id,
		knowledgeBaseId: citation.knowledge_base_id,
		revision: citation.revision,
		kind: 'fact',
		retrievedAt: citation.retrieved_at,
		...(citation.title === undefined ? {} : { title: citation.title }),
		...(citation.quote === undefined ? {} : { quote: citation.quote }),
		...(citation.start_at === undefined ? {} : { startAt: citation.start_at }),
		...(citation.end_at === undefined ? {} : { endAt: citation.end_at }),
	};
}

function conclusionOf(conclusion: EvidenceConclusionWire): RemoteEvidenceConclusion {
	return {
		kind: conclusion.kind,
		...(conclusion.model_id === undefined ? {} : { modelId: conclusion.model_id }),
		...(conclusion.rule_ids === undefined ? {} : { ruleIds: [...conclusion.rule_ids] }),
		...(conclusion.citation_ids === undefined ? {} : { citationIds: [...conclusion.citation_ids] }),
	};
}

function evidenceOf(evidence: AnswerEvidenceWire): RemoteAnswerEvidence {
	return {
		state: evidence.state,
		semanticGraphUsed: evidence.semantic_graph_used,
		retrievedAt: evidence.retrieved_at,
		citations: evidence.citations.map(citationOf),
		conclusions: evidence.conclusions.map(conclusionOf),
		reasoning: {
			requested: evidence.reasoning.requested,
			state: evidence.reasoning.state,
			retryable: evidence.reasoning.retryable,
			...(evidence.reasoning.mode === undefined ? {} : { mode: evidence.reasoning.mode }),
			...(evidence.reasoning.reason === undefined ? {} : { reason: evidence.reasoning.reason }),
		},
	};
}

export function createMobileKnowledgeQARemote(options: KnowledgeQARemoteOptions) {
	requireDeploymentOrigin(options.origin);
	const stream = options.stream;
	return {
		async ask(input: { sessionId: string; question: string; knowledgeBaseIds?: string[]; signal?: AbortSignal }): Promise<RemoteKnowledgeQATurn> {
			const sessionId = input.sessionId.trim();
			const question = input.question.trim();
			if (sessionId === '') throw new Error('knowledge QA requires sessionId');
			if (question === '') throw new Error('knowledge QA requires a question');
			const knowledgeBaseIds = Array.isArray(input.knowledgeBaseIds)
				? input.knowledgeBaseIds.map((id) => id.trim()).filter((id) => id !== '')
				: [];
			let answer = '';
			let isFallback = false;
			let evidence: RemoteAnswerEvidence | undefined;
			let failed = false;
			let terminated = false;
			let frames = 0;
			const parser = createServerSentEventParser((frame) => {
				frames += 1;
				let event;
				try {
					event = parseChatEvent(frame);
				} catch {
					throw new Error('KNOWLEDGE_QA_MALFORMED_FRAME');
				}
				const type = responseType(event);
				if (type === 'error') {
					failed = true;
					return;
				}
				if (type === 'evidence') {
					// 防御深度：服务端契约保证 evidence 先于 CHAT_COMPLETION_STREAM
					//（complete/stop）发射；晚于终止帧到达的 evidence 说明流序错乱，
					// 不得采信（fail closed，与缺帧同判）。
					if (terminated) throw new Error('KNOWLEDGE_QA_EVIDENCE_AFTER_TERMINAL');
					// evidence 帧畸形/不自洽时 parseAnswerEvidence 整体拒绝（不部分渲染）。
					const data = (event.data ?? {}) as { evidence?: unknown };
					evidence = evidenceOf(parseAnswerEvidence(data.evidence));
					return;
				}
				if (type === 'answer') {
					if (typeof event.content === 'string') answer += event.content;
					const data = (event.data ?? {}) as { is_fallback?: unknown };
					if (data.is_fallback === true) isFallback = true;
					return;
				}
				if (type === 'complete' || type === 'stop') terminated = true;
			});
			await stream({
				method: 'POST',
				path: `/api/v1/knowledge-chat/${encodeURIComponent(sessionId)}`,
				headers: { accept: 'text/event-stream', 'content-type': 'application/json' },
				body: { query: question, ...(knowledgeBaseIds.length === 0 ? {} : { knowledge_base_ids: knowledgeBaseIds }) },
				...(input.signal === undefined ? {} : { signal: input.signal }),
			}, (chunk) => parser.push(chunk));
			parser.finish();
			if (failed) throw new Error('KNOWLEDGE_QA_FAILED');
			// Fail-closed 终止帧判定（与 #44 legacy followUp 同规则）：服务端正常完成必发
			// complete/stop；流结束无终止帧＝结果未知，不得静默判成功。
			if (!terminated) throw new Error(`KNOWLEDGE_QA_TRUNCATED: knowledge QA for session ${sessionId} ended without a terminal frame after ${frames} frame(s)`);
			// 能力 fail closed：无 evidence 帧的服务端（T15 之前）不能提供版本/时间/三类
			// 证据，本回合不得被当作有证据回答交付。
			if (evidence === undefined) throw new Error(`KNOWLEDGE_QA_MISSING_EVIDENCE: session ${sessionId} completed without an evidence frame`);
			return { answer, isFallback, evidence };
		},
	};
}
