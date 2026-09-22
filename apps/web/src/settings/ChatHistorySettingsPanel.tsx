// Chat history settings — TDesign 同构迁移（T12b）平移自
// frontend/src/views/settings/ChatHistorySettings.vue（Settings.vue 挂载于
// "chathistory" 分区）。DOM/类名/文案逐节点对照 Vue SFC；样式在
// settings.td.css §8（scoped 块以 .chat-history-settings 根类限定）。
// ModelSelector（modelType='Embedding'）见 ModelSelector.tsx。
import { useEffect, useMemo, useRef, useState } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import { Switch as TSwitch } from 'tdesign-react';
import { readInitialLocale, settingsT } from './PortedSectionsPanel.tsx';
import { pushSettingsToast } from './settings-toast.tsx';
import { settingsConfigPatch, tenantModelIds } from './surface.ts';
import { ModelSelector, type ModelSelectorModel } from './ModelSelector.tsx';

interface ChatHistoryConfig { enabled: boolean; embedding_model_id: string }

function readConfig(value: unknown): ChatHistoryConfig {
  const row = value !== null && typeof value === 'object' && !Array.isArray(value) ? value as Record<string, unknown> : {};
  return { enabled: row.enabled === true, embedding_model_id: typeof row.embedding_model_id === 'string' ? row.embedding_model_id : '' };
}

export function ChatHistorySettingsPanel({ client, initialValue, models, embeddingLocked, stats, onSaved }: {
  client: WeKnoraClient;
  /** getTenantChatHistoryConfig 载荷（壳层 readSettingsSection 预取）。 */
  initialValue: unknown;
  models?: readonly ModelSelectorModel[];
  /** stats.has_indexed_messages → Vue modelLocked。 */
  embeddingLocked?: boolean;
  /** getChatHistoryKBStats 载荷（Vue stats ref）。 */
  stats?: unknown;
  onSaved?: () => void;
}) {
  const locale = readInitialLocale();
  const t = settingsT(locale);
  const savedConfig = useMemo(() => readConfig(initialValue), [initialValue]);
  const [enabled, setEnabled] = useState(savedConfig.enabled);
  const [embeddingModelId, setEmbeddingModelId] = useState(savedConfig.embedding_model_id);
  const saveTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const savingRef = useRef(false);
  const latestRef = useRef({ enabled: savedConfig.enabled, embedding_model_id: savedConfig.embedding_model_id, saved: savedConfig });

  useEffect(() => {
    const next = readConfig(initialValue);
    setEnabled(next.enabled);
    setEmbeddingModelId(next.embedding_model_id);
    latestRef.current = { enabled: next.enabled, embedding_model_id: next.embedding_model_id, saved: next };
  }, [initialValue]);

  const allowedModelIds = tenantModelIds(models ?? []);

  // Vue saveConfig（debouncedSave 500ms）：保存成功 toast + onSaved 刷新 stats。
  async function save(next: ChatHistoryConfig) {
    if (savingRef.current) return;
    savingRef.current = true;
    try {
      const patch = settingsConfigPatch('chathistory', { enabled: next.enabled, embedding_model_id: next.embedding_model_id }, allowedModelIds.length > 0 ? { allowedModelIds } : {});
      const saved = await client.settings.chatHistory.config.update(patch as never);
      const confirmed = readConfig(saved);
      latestRef.current = { ...latestRef.current, saved: confirmed };
      pushSettingsToast(t('chatHistorySettings.toasts.saveSuccess'), 'success');
      onSaved?.();
    } catch (reason) {
      const message = reason instanceof Error ? reason.message : 'Unknown error';
      pushSettingsToast(t('chatHistorySettings.toasts.saveFailed', { message }));
    } finally { savingRef.current = false; }
  }

  function scheduleSave() {
    if (saveTimer.current) clearTimeout(saveTimer.current);
    const snapshot = latestRef.current;
    saveTimer.current = setTimeout(() => {
      saveTimer.current = null;
      const { saved } = latestRef.current;
      if (snapshot.enabled !== saved.enabled || snapshot.embedding_model_id !== saved.embedding_model_id) {
        void save({ enabled: snapshot.enabled, embedding_model_id: snapshot.embedding_model_id });
      }
    }, 500);
  }
  useEffect(() => () => { if (saveTimer.current) clearTimeout(saveTimer.current); }, []);

  const modelLocked = embeddingLocked === true;
  const statsRow = stats !== null && stats !== undefined && typeof stats === 'object' ? stats as Record<string, unknown> : null;

  return (
    <div className="chat-history-settings">
      <div className="section-header">
        <h2>{t('chatHistorySettings.title')}</h2>
        <p className="section-description">{t('chatHistorySettings.description')}</p>
      </div>

      <div className="settings-group">
        {/* 启用开关 */}
        <div className="setting-row">
          <div className="setting-info">
            <label>{t('chatHistorySettings.enableLabel')}</label>
            <p className="desc">{t('chatHistorySettings.enableDescription')}</p>
          </div>
          <div className="setting-control">
            <TSwitch value={enabled} onChange={(value) => {
              setEnabled(value === true);
              latestRef.current = { ...latestRef.current, enabled: value === true };
              scheduleSave();
            }} />
          </div>
        </div>

        {/* Embedding 模型选择 */}
        {enabled ? <div className="setting-row">
          <div className="setting-info">
            <label>{t('chatHistorySettings.embeddingModelLabel')}</label>
            <p className="desc">{t('chatHistorySettings.embeddingModelDescription')}</p>
            {modelLocked ? <p className="desc warning-text">{t('chatHistorySettings.embeddingModelLocked')}</p> : null}
          </div>
          <div className="setting-control" style={{ minWidth: '280px' }}>
            <ModelSelector
              modelType="Embedding"
              selectedModelId={embeddingModelId}
              disabled={modelLocked}
              allModels={models ?? []}
              onChange={(modelId) => {
                setEmbeddingModelId(modelId);
                latestRef.current = { ...latestRef.current, embedding_model_id: modelId };
                scheduleSave();
              }}
            />
          </div>
        </div> : null}
      </div>

      {/* 统计信息 */}
      <div className="stats-section">
        <h3 className="stats-title">{t('chatHistorySettings.statsTitle')}</h3>
        {statsRow && statsRow.enabled === true && statsRow.knowledge_base_id ? <div className="stats-grid">
          <div className="stat-card">
            <div className="stat-value">{String(statsRow.indexed_message_count ?? 0)}</div>
            <div className="stat-label">{t('chatHistorySettings.statsIndexedMessages')}</div>
          </div>
        </div> : <div className="stats-empty">
          <p className="stats-empty-title">{t('chatHistorySettings.statsNotConfigured')}</p>
          <p className="stats-empty-desc">{t('chatHistorySettings.statsNotConfiguredDesc')}</p>
        </div>}
      </div>
    </div>
  );
}
