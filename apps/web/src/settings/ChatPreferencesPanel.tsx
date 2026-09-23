// SP14 Task 4 — 会话偏好分区：默认对话模型（Ruling P-3 方案 A 纯前端）。
//
// The panel is the settings surface for the user's server-side default chat
// model (backend Task 3: PUT /api/v1/auth/me/preferences merges
// {default_model: string|''}; GET /auth/me echoes it back through
// data.user.preferences). The model dropdown reuses the standard
// ModelOptionSelect with the chat KnowledgeQA filter (PersonalMemoryPanel
// extract/embedding precedent: self-pulled client.configuration.models.list()
// + addModel jumps to the models section) and a clear entry that persists the
// empty string. The chip-side resolution chain that consumes this preference
// lives in ../chat/model-chip.ts (pick > agent binding > user default >
// first model).
import { useEffect, useState } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
// S6：Status 无 TDesign 对应（playbook §1 附行），走 shared/wk-legacy。
import { WkStatus as Status } from '../shared/wk-legacy.tsx';
import { ModelOptionSelect, type ModelOption } from './ModelOptionSelect.tsx';
import { listChatModels, resolveChatModelOptions, type ChatModelLike } from '../chat/model-chip.ts';
import { navigate } from '../platform/navigation.ts';
import { readInitialLocale, settingsT } from './PortedSectionsPanel.tsx';

/**
 * preferences payload → default_model echo. The backend stores '' for "no
 * default"; any non-string (legacy payloads, test doubles) reads as unset so
 * the panel starts from the cleared state instead of crashing.
 */
export function readDefaultModelPreference(payload: unknown): string {
  if (payload === null || typeof payload !== 'object' || Array.isArray(payload)) return '';
  const value = (payload as Record<string, unknown>).default_model;
  return typeof value === 'string' ? value.trim() : '';
}

/**
 * Save/clear patch for PUT /auth/me/preferences. Task 3 merge semantics: the
 * empty string clears the default (a missing key would be a no-op).
 */
export function defaultModelPreferencePatch(modelId: string): { default_model: string } {
  return { default_model: modelId.trim() };
}

/**
 * Dropdown options for the default chat model: the Vue chat-dropdown filter
 * (KnowledgeQA only, chatResources.ts:59) with the chip's display_name ||
 * name || id label fallback (R483 D13 empty-display_name trap).
 */
export function chatDefaultModelOptions<T extends ChatModelLike>(models: readonly T[]): ModelOption[] {
  return resolveChatModelOptions(listChatModels(models)).map((option) => ({ value: option.id, label: option.name }));
}

export function ChatPreferencesPanel({ client, initialPreferences = null }: { client: WeKnoraClient; initialPreferences?: unknown }) {
  const t = settingsT(readInitialLocale());
  const [defaultModel, setDefaultModel] = useState(() => readDefaultModelPreference(initialPreferences));
  const [models, setModels] = useState<readonly ChatModelLike[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  // The shell hands the freshly read preferences payload in (silent refresh
  // on revisit); re-echo whenever a new payload lands.
  useEffect(() => { setDefaultModel(readDefaultModelPreference(initialPreferences)); }, [initialPreferences]);

  // PersonalMemoryPanel precedent: the dropdown pulls the tenant model list
  // itself; a failure degrades to an empty list (the add-model jump stays).
  useEffect(() => {
    void client.configuration.models.list().then((items) => setModels(items)).catch(() => setModels([]));
  }, [client]);

  async function saveDefaultModel(modelId: string): Promise<void> {
    setBusy(true); setError(null); setNotice(null);
    try {
      await client.settings.preferences.update(defaultModelPreferencePatch(modelId));
      setDefaultModel(modelId.trim());
      setNotice(t('chatPreferences.saveSuccess'));
    } catch (reason) {
      // Vue toast pattern (chatHistorySettings.toasts.saveFailed): localized
      // prefix + the backend message.
      const message = reason instanceof Error ? reason.message.trim() : '';
      setError(t('chatPreferences.saveFailed') + (message ? `: ${message}` : ''));
    } finally {
      setBusy(false);
    }
  }

  const options = chatDefaultModelOptions(models);

  return <div data-testid="chat-preferences-panel" className="wk-chat-prefs">
    {error ? <Status tone="error">{error}</Status> : null}
    {notice ? <Status tone="success">{notice}</Status> : null}
    <div className="wk-chat-prefs-body">
      <div className="wk-chat-prefs-row">
        <div className="wk-chat-prefs-row-copy">
          <label className="wk-chat-prefs-label">{t('chatPreferences.defaultModelLabel')}</label>
          <p className="wk-chat-prefs-desc">{t('chatPreferences.defaultModelDescription')}</p>
          <p className="wk-chat-prefs-hint">{t('chatPreferences.priorityHint')}</p>
        </div>
        <div className="wk-chat-prefs-control">
          <ModelOptionSelect
            value={defaultModel}
            options={options}
            disabled={busy}
            clearable
            clearLabel={t('common.remove')}
            addModelLabel={t('model.addModelInSettings')}
            onAddModel={() => navigate('/platform/settings?section=models&subsection=chat')}
            onChange={(value) => { void saveDefaultModel(value); }}
          />
        </div>
      </div>
    </div>
  </div>;
}
