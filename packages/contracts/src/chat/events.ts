export type ChatResponseType =
  | 'answer'
  | 'thinking'
  | 'tool_call'
  | 'tool_result'
  | 'tool_approval_required'
  | 'tool_approval_resolved'
  | 'references'
  | 'complete'
  | 'stop'
  | 'error';

export interface ChatStreamEvent {
  response_type?: ChatResponseType | string;
  type?: ChatResponseType | string;
  event_id?: string;
  message_id?: string;
  content?: unknown;
  data?: unknown;
  [key: string]: unknown;
}

export function responseType(event: ChatStreamEvent): string | undefined {
  return typeof event.response_type === 'string' ? event.response_type : typeof event.type === 'string' ? event.type : undefined;
}
