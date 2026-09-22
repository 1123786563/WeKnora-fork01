// ModelSelector — frontend/src/components/ModelSelector.vue 的 TDesign 同构
// 平移（T12b）。t-select filterable + t-option 列表（含底部“前往全局设置添加
// 模型”项）；KnowledgeQA/VLLM 类型渲染上下文窗口 chip（showContextWindow）。
// 弹层类样式（.model-option/.model-ctx）在 settings.td.css §8b（portal 到
// body 的弹层 DOM 不挂组件根类，§2.5 同款不加前缀）。
import { useMemo } from 'react';
import { Icon as TIcon } from 'tdesign-icons-react';
import { Select as TSelect, Tag as TTag } from 'tdesign-react';
import { readInitialLocale, settingsT } from './PortedSectionsPanel.tsx';

export interface ModelSelectorModel {
  readonly id: string;
  readonly name?: string;
  readonly display_name?: string;
  readonly is_builtin?: boolean;
  readonly is_default?: boolean;
  readonly type?: string;
  readonly parameters?: Record<string, unknown> | null;
}

export type ModelSelectorType = 'KnowledgeQA' | 'Embedding' | 'Rerank' | 'VLLM' | 'ASR';

/* frontend/src/utils/contextWindow.ts 平移。 */
const DEFAULT_MODEL_CONTEXT_WINDOW = 200000;
function modelHasContextWindow(type: string): boolean {
  return type === 'KnowledgeQA' || type === 'VLLM' || type === 'chat' || type === 'vllm';
}
function effectiveContextWindow(tokens?: number | null): number {
  return typeof tokens === 'number' && tokens > 0 ? tokens : DEFAULT_MODEL_CONTEXT_WINDOW;
}
function isDefaultContextWindow(tokens?: number | null): boolean {
  return !(typeof tokens === 'number' && tokens > 0);
}
function formatTokenCount(tokens: number): string {
  if (!Number.isFinite(tokens) || tokens <= 0) return '';
  const n = Math.round(tokens);
  if (n >= 1_000_000 && n % 1_000_000 === 0) return `${n / 1_000_000}M`;
  if (n >= 1000 && n % 1000 === 0) return `${n / 1000}K`;
  if (n >= 1024 && n % 1024 === 0) {
    const k = n / 1024;
    if (k >= 1024 && k % 1024 === 0) return `${k / 1024}M`;
    return `${k}K`;
  }
  return String(n);
}
function formatContextWindow(tokens?: number | null): string {
  return formatTokenCount(effectiveContextWindow(tokens));
}

/* frontend/src/components/modelSelectorFilter.ts 平移。 */
export function filterModelsByType(allModels: readonly ModelSelectorModel[], modelType: ModelSelectorType): ModelSelectorModel[] {
  if (modelType === 'VLLM') {
    return allModels.filter((m) => m.type === 'VLLM' || (m.type === 'KnowledgeQA' && (m.parameters as { supports_vision?: boolean } | undefined)?.supports_vision === true));
  }
  return allModels.filter((m) => m.type === modelType);
}

function contextWindowTokens(model: ModelSelectorModel): number | undefined {
  const value = (model.parameters as { context_window?: unknown } | undefined)?.context_window;
  return typeof value === 'number' ? value : undefined;
}

function modelDisplayName(model: ModelSelectorModel): string {
  const displayName = typeof model.display_name === 'string' ? model.display_name.trim() : '';
  return displayName || (model.name ?? model.id);
}

export function ModelSelector({ modelType, selectedModelId = '', disabled = false, clearable = false, allModels, onChange, onAddModel }: {
  modelType: ModelSelectorType;
  selectedModelId?: string;
  disabled?: boolean;
  clearable?: boolean;
  /** 外部传入的全量模型列表（Vue allModels prop；未提供时父组件自拉）。 */
  allModels: readonly ModelSelectorModel[];
  /** Vue update:selectedModelId emit。 */
  onChange: (modelId: string) => void;
  /** Vue add-model emit（选择「前往全局设置添加模型」项）。 */
  onAddModel?: () => void;
}) {
  const t = settingsT(readInitialLocale());
  const models = useMemo(() => filterModelsByType(allModels, modelType), [allModels, modelType]);
  const selectedModel = selectedModelId ? models.find((m) => m.id === selectedModelId) : undefined;
  const showContextWindow = modelHasContextWindow(modelType);
  const contextWindowTitle = (tokens?: number) => isDefaultContextWindow(tokens)
    ? t('model.editor.contextWindowDefaultHint', { value: formatContextWindow(tokens) })
    : t('model.editor.contextWindowTokens', { count: effectiveContextWindow(tokens) });

  return (
    <div className="model-selector">
      <TSelect
        value={selectedModelId}
        onChange={(value) => {
          // Vue handleModelChange：选添加项时触发 add-model，不更新选中值。
          if (value === '__add_model__') { onAddModel?.(); return; }
          onChange(typeof value === 'string' ? value : '');
        }}
        placeholder={t('model.selectModelPlaceholder')}
        disabled={disabled}
        clearable={clearable}
        filterable
        style={{ width: '100%' }}
        {...(selectedModel && showContextWindow ? {
          valueDisplay: () => (
            <span className="selected-model">
              <span className="selected-model__name">{modelDisplayName(selectedModel)}</span>
              <span
                className={'model-ctx' + (isDefaultContextWindow(contextWindowTokens(selectedModel)) ? ' model-ctx--default' : '')}
                title={contextWindowTitle(contextWindowTokens(selectedModel))}
              >{formatContextWindow(contextWindowTokens(selectedModel))}</span>
            </span>
          ),
        } : {})}
      >
        {[
          ...models.map((model) => {
            const tokens = contextWindowTokens(model);
            return (
              <TSelect.Option key={model.id} value={model.id} label={modelDisplayName(model)}>
                <div className="model-option">
                  <TIcon name="check-circle-filled" className="model-icon" />
                  <span className="model-name">{modelDisplayName(model)}</span>
                  {model.display_name ? <span className="model-raw-name">{model.name}</span> : null}
                  {model.is_builtin ? <TTag size="small" theme="primary">{t('model.builtinTag')}</TTag> : null}
                  {model.is_default ? <TTag size="small" theme="success">{t('model.defaultTag')}</TTag> : null}
                  {showContextWindow ? <span
                    className={'model-ctx' + (isDefaultContextWindow(tokens) ? ' model-ctx--default' : '')}
                    title={contextWindowTitle(tokens)}
                  >{formatContextWindow(tokens)}</span> : null}
                </div>
              </TSelect.Option>
            );
          }),
          ...(!disabled ? [(
            <TSelect.Option key="__add_model__" value="__add_model__" className="add-model-option">
              <div className="model-option add">
                <TIcon name="add" className="add-icon" />
                <span className="model-name">{t('model.addModelInSettings')}</span>
              </div>
            </TSelect.Option>
          )] : []),
        ]}
      </TSelect>
    </div>
  );
}
