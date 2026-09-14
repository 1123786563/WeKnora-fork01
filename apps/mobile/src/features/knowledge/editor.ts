import type { FAQEntry, WikiPage } from '@weknora/api-client';

export interface WikiEditorDraft {
  title: string;
  summary: string;
  content: string;
  version: number;
}

export interface FaqEditorDraft {
  standardQuestion: string;
  answer: string;
  isEnabled?: boolean;
  isRecommended?: boolean;
}

export function createWikiDraft(page: Pick<WikiPage, 'title' | 'summary' | 'content' | 'version'>): WikiEditorDraft {
  return { title: page.title.trim(), summary: page.summary, content: page.content, version: page.version };
}

export function createFaqDraft(entry: Pick<FAQEntry, 'standard_question' | 'answers' | 'is_enabled' | 'is_recommended'>): FaqEditorDraft {
  return {
    standardQuestion: entry.standard_question.trim(),
    answer: entry.answers[0] || '',
    isEnabled: entry.is_enabled,
    isRecommended: entry.is_recommended,
  };
}

export type MobileEditorValidationKey =
  | 'knowledgeEditor.mobile.titleRequired'
  | 'knowledgeEditor.mobile.contentRequired'
  | 'knowledgeEditor.mobile.questionRequired'
  | 'knowledgeEditor.mobile.answerRequired';

export function validateWikiDraft(draft: Pick<WikiEditorDraft, 'title' | 'content'>): MobileEditorValidationKey[] {
  const errors: MobileEditorValidationKey[] = [];
  if (!draft.title.trim()) errors.push('knowledgeEditor.mobile.titleRequired');
  if (!draft.content.trim()) errors.push('knowledgeEditor.mobile.contentRequired');
  return errors;
}

export function validateFaqDraft(draft: Pick<FaqEditorDraft, 'standardQuestion' | 'answer'>): MobileEditorValidationKey[] {
  const errors: MobileEditorValidationKey[] = [];
  if (!draft.standardQuestion.trim()) errors.push('knowledgeEditor.mobile.questionRequired');
  if (!draft.answer.trim()) errors.push('knowledgeEditor.mobile.answerRequired');
  return errors;
}

export function classifyMobileEditorError(error: unknown): 'conflict' | 'error' {
  return typeof error === 'object' && error !== null && 'status' in error && (error as { status?: unknown }).status === 409
    ? 'conflict'
    : 'error';
}
