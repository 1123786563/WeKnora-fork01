// Pure contract for the R439 editor-section migration (Vue
// KnowledgeBaseEditorModal.vue → KBModelConfig.vue / KBChunkingSettings.vue /
// KBAdvancedSettings.vue). Kept framework-free so the Vue vocabulary can be
// asserted directly by editorSections.test.ts.

export interface KnowledgeSettingsModelOption {
  id: string;
  name: string;
  displayName: string;
  type: string;
  source: string;
  status?: string;
}

// Vue modelDefaults: a model is usable when it carries no status or an
// 'active' status — anything else (unavailable, downloading…) is excluded
// from the selectors, mirroring the parser engines' Available !== false rule.
export function isKnowledgeSettingsModelAvailable(model: { status?: string }): boolean {
  return !model.status || model.status === 'active';
}

// Vue ModelSelector filterModelsByType: same-type models only, plus the
// availability exclusion above.
export function filterKnowledgeSettingsModels(models: KnowledgeSettingsModelOption[], type: string): KnowledgeSettingsModelOption[] {
  return models
    .filter((model) => model.type === type && isKnowledgeSettingsModelAvailable(model))
    .map((model) => ({ ...model, displayName: model.displayName || model.name }));
}

// Slider bounds mirror the validated backend splitter bounds documented in
// KBChunkingSettings.vue.
export const CHUNK_SIZE_RANGE = { min: 100, max: 4000, step: 50 } as const;
export const CHUNK_OVERLAP_RANGE = { min: 0, max: 500, step: 20 } as const;
export const PARENT_CHUNK_SIZE_RANGE = { min: 512, max: 8192, step: 64 } as const;
export const CHILD_CHUNK_SIZE_RANGE = { min: 64, max: 2048, step: 32 } as const;
export const TOKEN_LIMIT_RANGE = { min: 0, max: 8192, step: 64 } as const;

// KBAdvancedSettings question-count input-number bounds.
export const QUESTION_COUNT_RANGE = { min: 1, max: 10, step: 1 } as const;

// Vue strategyOptions values; '' = not set (backend clears via pointer DTO).
export const CHUNKING_STRATEGY_VALUES = ['auto', 'heading', 'heuristic', 'legacy'] as const;
// Vue separatorOptions values in Vue order.
export const CHUNKING_SEPARATOR_VALUES = ['\n\n', '\n', '。', '！', '？', '；', ';', ' '] as const;
// Vue languageOptions values.
export const CHUNKING_LANGUAGE_VALUES = ['de', 'en', 'zh'] as const;

// Existing knowledgeEditor.* keys (generated i18n covers all five locales).
export const CHUNKING_STRATEGY_LABEL_KEY_PREFIX = 'knowledgeEditor.chunking.strategies';
export const CHUNKING_STRATEGY_TOOLTIP_KEY_PREFIX = 'knowledgeEditor.chunking.strategies';

export const CHUNKING_SEPARATOR_LABEL_KEYS: Record<string, string> = {
  '\n\n': 'knowledgeEditor.chunking.separators.doubleNewline',
  '\n': 'knowledgeEditor.chunking.separators.singleNewline',
  '。': 'knowledgeEditor.chunking.separators.periodCn',
  '！': 'knowledgeEditor.chunking.separators.exclamationCn',
  '？': 'knowledgeEditor.chunking.separators.questionCn',
  '；': 'knowledgeEditor.chunking.separators.semicolonCn',
  ';': 'knowledgeEditor.chunking.separators.semicolonEn',
  ' ': 'knowledgeEditor.chunking.separators.space',
};

export const CHUNKING_LANGUAGE_LABEL_KEYS: Record<string, string> = {
  de: 'knowledgeEditor.chunking.languageOptions.de',
  en: 'knowledgeEditor.chunking.languageOptions.en',
  zh: 'knowledgeEditor.chunking.languageOptions.zh',
};

// Vue overlapTooHigh computed.
export function isChunkOverlapTooHigh(chunkSize: number, chunkOverlap: number): boolean {
  return chunkOverlap > 0 && chunkOverlap >= chunkSize / 2;
}

// Vue t-select tag label for a separator value (KBChunkingSettings.vue
// separatorOptions): preset values carry the localized label, values the user
// created through the creatable input display the raw characters.
export function formatKnowledgeSettingsSeparatorLabel(value: string, translate: (key: string) => string): string {
  const key = CHUNKING_SEPARATOR_LABEL_KEYS[value];
  return key ? translate(key) : value;
}

// Vue advancedDisabled computed: legacy chunking cannot use the adaptive
// token-budget/language-hint controls.
export function isChunkingAdvancedDisabled(strategy: string): boolean {
  return strategy === 'legacy';
}

export function clampQuestionCount(value: number): number {
  if (!Number.isFinite(value)) return QUESTION_COUNT_RANGE.min;
  return Math.min(QUESTION_COUNT_RANGE.max, Math.max(QUESTION_COUNT_RANGE.min, Math.round(value)));
}
