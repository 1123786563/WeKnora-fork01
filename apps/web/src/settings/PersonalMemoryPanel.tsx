// Memory workspace settings — TDesign 同构迁移（T12b）平移自
// frontend/src/views/settings/MemoryWorkspaceSettings.vue（Settings.vue 挂载于
// "memory" 分区，长期记忆空间级开关）。DOM/类名/文案逐节点对照 Vue SFC；
// 样式在 settings.td.css §9（scoped 块以 .memory-workspace-settings 根类限定）。
// 保存走 Vue debouncedSave(500ms) + MessagePlugin toast（pushSettingsToast）。
import { useEffect, useRef, useState } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import { Icon as TIcon } from 'tdesign-icons-react';
import { InputNumber, Radio, RadioGroup, Switch as TSwitch, Textarea as TTextarea } from 'tdesign-react';
import { readInitialLocale, settingsT } from './PortedSectionsPanel.tsx';
import { navigate } from '../platform/navigation.ts';
import { memoryWorkspacePatch } from './surface.ts';
import { pushSettingsToast } from './settings-toast.tsx';
import { ModelSelector, type ModelSelectorModel } from './ModelSelector.tsx';

type MemoryRow = Record<string, unknown>;

interface MemoryDraft {
  enabled: boolean;
  write_mode: 'explicit_only' | 'auto';
  extract_model_id: string;
  max_items: number;
  extract_delay_seconds: number;
  extract_min_interval_seconds: number;
  extract_instructions: string;
  interest_threshold: number;
  retrieval_conditioning: boolean;
  embedding_model_id: string;
  vector_recall: boolean;
}

const DEFAULT_DRAFT: MemoryDraft = {
  enabled: false,
  write_mode: 'explicit_only',
  extract_model_id: '',
  max_items: 200,
  extract_delay_seconds: 90,
  extract_min_interval_seconds: 300,
  extract_instructions: '',
  interest_threshold: 3,
  retrieval_conditioning: true,
  embedding_model_id: '',
  vector_recall: true,
};

// Vue loadConfig 的字段回退（cfg.x || default / !== false）。
function readDraft(value: unknown): MemoryDraft {
  const row = value !== null && typeof value === 'object' && !Array.isArray(value) ? value as MemoryRow : {};
  const num = (key: string, fallback: number): number => (typeof row[key] === 'number' && row[key] !== 0 ? row[key] as number : fallback);
  return {
    enabled: row.enabled === true,
    write_mode: row.write_mode === 'auto' ? 'auto' : 'explicit_only',
    extract_model_id: typeof row.extract_model_id === 'string' && row.extract_model_id ? row.extract_model_id : '',
    max_items: num('max_items', 200),
    extract_delay_seconds: num('extract_delay_seconds', 90),
    extract_min_interval_seconds: num('extract_min_interval_seconds', 300),
    extract_instructions: typeof row.extract_instructions === 'string' && row.extract_instructions ? row.extract_instructions : '',
    interest_threshold: num('interest_threshold', 3),
    retrieval_conditioning: row.retrieval_conditioning !== false,
    embedding_model_id: typeof row.embedding_model_id === 'string' && row.embedding_model_id ? row.embedding_model_id : '',
    vector_recall: row.vector_recall !== false,
  };
}

export function MemoryWorkspacePanel({ client, initialConfig, canEdit = true }: { client: WeKnoraClient; initialConfig: unknown; canEdit?: boolean }) {
  const t = settingsT(readInitialLocale());
  const [draft, setDraft] = useState<MemoryDraft>(() => readDraft(initialConfig));
  const [models, setModels] = useState<ModelSelectorModel[]>([]);
  const saveTimerRef = useRef<number | null>(null);
  const draftRef = useRef(draft);
  draftRef.current = draft;

  useEffect(() => { setDraft(readDraft(initialConfig)); }, [initialConfig]);

  useEffect(() => () => {
    if (saveTimerRef.current !== null) window.clearTimeout(saveTimerRef.current);
  }, []);

  // Vue ModelSelector 未收到 allModels 时自拉（chatResources.ensureModels →
  // configuration.models.list）。
  useEffect(() => {
    void client.configuration.models.list()
      .then((items) => setModels(items.map((item) => ({
        id: item.id,
        name: item.name,
        display_name: typeof item.display_name === 'string' ? item.display_name : undefined,
        is_builtin: item.is_builtin === true,
        is_default: item.is_default === true,
        type: typeof item.type === 'string' ? item.type : undefined,
        parameters: (item.parameters ?? null) as Record<string, unknown> | null,
      }))))
      .catch(() => setModels([]));
  }, [client]);

  function update(patch: Partial<MemoryDraft>) {
    setDraft((current) => ({ ...current, ...patch }));
  }

  // Vue saveConfig（debouncedSave 500ms，isInitializing/canEdit 抑制）。
  function debouncedSave() {
    if (!canEdit) return;
    if (saveTimerRef.current !== null) window.clearTimeout(saveTimerRef.current);
    saveTimerRef.current = window.setTimeout(() => {
      saveTimerRef.current = null;
      void (async () => {
        const next = draftRef.current;
        try {
          await client.settings.memory.workspace.update(memoryWorkspacePatch(
            next.enabled, next.write_mode, next.max_items, next.vector_recall, next.retrieval_conditioning,
            {
              extractModelId: next.extract_model_id,
              extractDelaySeconds: next.extract_delay_seconds,
              extractMinIntervalSeconds: next.extract_min_interval_seconds,
              extractInstructions: next.extract_instructions,
              interestThreshold: next.interest_threshold,
              embeddingModelId: next.embedding_model_id,
            },
          ) as never);
          pushSettingsToast(t('memoryWorkspaceSettings.toasts.saveSuccess'), 'success');
        } catch (reason) {
          const message = reason instanceof Error ? reason.message : '';
          pushSettingsToast(t('memoryWorkspaceSettings.toasts.saveFailed', { message }));
        }
      })();
    }, 500);
  }

  return (
    <div className="memory-workspace-settings">
      <div className="section-header">
        <h2>{t('memoryWorkspaceSettings.title')}</h2>
        <p className="section-description">{t('memoryWorkspaceSettings.description')}</p>
      </div>

      {/* The switch defaults to off because memory retains what users say
          across sessions. That makes the feature easy to miss, so the intro
          states plainly what turning it on does. */}
      <div className="intro">
        <TIcon name="info-circle" className="intro-icon" />
        <div>
          <p className="intro-title">{t('memoryWorkspaceSettings.introTitle')}</p>
          <p className="intro-desc">{t('memoryWorkspaceSettings.introDescription')}</p>
        </div>
      </div>

      <div className="settings-group">
        <div className="setting-row">
          <div className="setting-info">
            <label>{t('memoryWorkspaceSettings.enableLabel')}</label>
            <p className="desc">{t('memoryWorkspaceSettings.enableDescription')}</p>
          </div>
          <div className="setting-control">
            <TSwitch value={draft.enabled} disabled={!canEdit} onChange={(value) => { update({ enabled: value === true }); debouncedSave(); }} />
          </div>
        </div>

        {draft.enabled ? <div className="setting-row">
          <div className="setting-info">
            <label>{t('memoryWorkspaceSettings.writeModeLabel')}</label>
            <p className="desc">{t('memoryWorkspaceSettings.writeModeDescription')}</p>
            <p className="desc hint">
              {draft.write_mode === 'auto' ? t('memoryWorkspaceSettings.writeModeAutoHint') : t('memoryWorkspaceSettings.writeModeExplicitHint')}
            </p>
          </div>
          <div className="setting-control">
            <RadioGroup value={draft.write_mode} disabled={!canEdit} onChange={(value) => { update({ write_mode: value === 'auto' ? 'auto' : 'explicit_only' }); debouncedSave(); }}>
              <Radio.Button value="explicit_only">{t('memoryWorkspaceSettings.writeModeExplicit')}</Radio.Button>
              <Radio.Button value="auto">{t('memoryWorkspaceSettings.writeModeAuto')}</Radio.Button>
            </RadioGroup>
          </div>
        </div> : null}

        {draft.enabled && draft.write_mode === 'auto' ? <div className="setting-row">
          <div className="setting-info">
            <label>{t('memoryWorkspaceSettings.extractModelLabel')}</label>
            <p className="desc">{t('memoryWorkspaceSettings.extractModelDescription')}</p>
          </div>
          <div className="setting-control" style={{ minWidth: '280px' }}>
            <ModelSelector
              modelType="KnowledgeQA"
              selectedModelId={draft.extract_model_id}
              disabled={!canEdit}
              allModels={models}
              onChange={(modelId) => { update({ extract_model_id: modelId }); debouncedSave(); }}
              onAddModel={() => navigate('/platform/settings?section=models&subsection=chat')}
            />
          </div>
        </div> : null}

        {draft.enabled && draft.write_mode === 'auto' ? <div className="setting-row">
          <div className="setting-info">
            <label>{t('memoryWorkspaceSettings.extractDelayLabel')}</label>
            <p className="desc">{t('memoryWorkspaceSettings.extractDelayDescription')}</p>
          </div>
          <div className="setting-control">
            <InputNumber value={draft.extract_delay_seconds} min={5} max={3600} step={15} suffix="s" disabled={!canEdit} onChange={(value) => { update({ extract_delay_seconds: Number(value) }); debouncedSave(); }} />
          </div>
        </div> : null}

        {draft.enabled && draft.write_mode === 'auto' ? <div className="setting-row">
          <div className="setting-info">
            <label>{t('memoryWorkspaceSettings.extractMinIntervalLabel')}</label>
            <p className="desc">{t('memoryWorkspaceSettings.extractMinIntervalDescription')}</p>
          </div>
          <div className="setting-control">
            <InputNumber value={draft.extract_min_interval_seconds} min={0} max={86400} step={60} suffix="s" disabled={!canEdit} onChange={(value) => { update({ extract_min_interval_seconds: Number(value) }); debouncedSave(); }} />
          </div>
        </div> : null}

        {draft.enabled ? <div className="setting-row">
          <div className="setting-info">
            <label>{t('memoryWorkspaceSettings.vectorRecallLabel')}</label>
            <p className="desc">{t('memoryWorkspaceSettings.vectorRecallDescription')}</p>
          </div>
          <div className="setting-control">
            <TSwitch value={draft.vector_recall} disabled={!canEdit} onChange={(value) => { update({ vector_recall: value === true }); debouncedSave(); }} />
          </div>
        </div> : null}

        {draft.enabled && draft.vector_recall ? <div className="setting-row">
          <div className="setting-info">
            <label>{t('memoryWorkspaceSettings.embeddingModelLabel')}</label>
            <p className="desc">{t('memoryWorkspaceSettings.embeddingModelDescription')}</p>
          </div>
          <div className="setting-control" style={{ minWidth: '280px' }}>
            <ModelSelector
              modelType="Embedding"
              selectedModelId={draft.embedding_model_id}
              disabled={!canEdit}
              clearable
              allModels={models}
              onChange={(modelId) => { update({ embedding_model_id: modelId }); debouncedSave(); }}
              onAddModel={() => navigate('/platform/settings?section=models&subsection=embedding')}
            />
          </div>
        </div> : null}

        {draft.enabled ? <div className="setting-row">
          <div className="setting-info">
            <label>{t('memoryWorkspaceSettings.conditioningLabel')}</label>
            <p className="desc">{t('memoryWorkspaceSettings.conditioningDescription')}</p>
          </div>
          <div className="setting-control">
            <TSwitch value={draft.retrieval_conditioning} disabled={!canEdit} onChange={(value) => { update({ retrieval_conditioning: value === true }); debouncedSave(); }} />
          </div>
        </div> : null}

        {draft.enabled && draft.write_mode === 'auto' ? <div className="setting-row">
          <div className="setting-info">
            <label>{t('memoryWorkspaceSettings.interestThresholdLabel')}</label>
            <p className="desc">{t('memoryWorkspaceSettings.interestThresholdDescription')}</p>
          </div>
          <div className="setting-control">
            <InputNumber value={draft.interest_threshold} min={1} max={20} step={1} disabled={!canEdit} onChange={(value) => { update({ interest_threshold: Number(value) }); debouncedSave(); }} />
          </div>
        </div> : null}

        {draft.enabled && draft.write_mode === 'auto' ? <div className="setting-row instructions-row">
          <div className="setting-info">
            <label>{t('memoryWorkspaceSettings.instructionsLabel')}</label>
            <p className="desc">{t('memoryWorkspaceSettings.instructionsDescription')}</p>
          </div>
          <div className="setting-control instructions-control">
            <TTextarea
              value={draft.extract_instructions}
              autosize={{ minRows: 3, maxRows: 8 }}
              maxlength={1000}
              disabled={!canEdit}
              placeholder={t('memoryWorkspaceSettings.instructionsPlaceholder')}
              count={({ count, maxLength }) => <span className="t-textarea__limit">{`${count}/${maxLength}`}</span>}
              onChange={(value) => update({ extract_instructions: String(value ?? '') })}
              onBlur={debouncedSave}
            />
          </div>
        </div> : null}

        {draft.enabled ? <div className="setting-row">
          <div className="setting-info">
            <label>{t('memoryWorkspaceSettings.maxItemsLabel')}</label>
            <p className="desc">{t('memoryWorkspaceSettings.maxItemsDescription')}</p>
          </div>
          <div className="setting-control">
            <InputNumber value={draft.max_items} min={10} max={2000} step={10} disabled={!canEdit} onChange={(value) => { update({ max_items: Number(value) }); debouncedSave(); }} />
          </div>
        </div> : null}
      </div>
    </div>
  );
}
