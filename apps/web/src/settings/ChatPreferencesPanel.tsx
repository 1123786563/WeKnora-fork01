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
import { Status } from '@weknora/ui';
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

  return <div data-testid="chat-preferences-panel" className="w-full text-[rgba(0,0,0,0.9)]">
    {error ? <Status tone="error">{error}</Status> : null}
    {notice ? <Status tone="success">{notice}</Status> : null}
    <div className="flex flex-col">
      <div className="flex items-start justify-between border-b border-[#e7e7e7] py-5 last:border-b-0 max-[720px]:flex-col max-[720px]:gap-2">
        <div className="max-w-[65%] flex-1 pr-6 max-[720px]:max-w-full max-[720px]:pr-0">
          <label className="mb-1 block text-[15px] font-medium text-[rgba(0,0,0,0.9)]">{t('chatPreferences.defaultModelLabel')}</label>
          <p className="m-0 text-[13px] leading-[1.5] text-[rgba(0,0,0,0.6)]">{t('chatPreferences.defaultModelDescription')}</p>
          <p className="m-0 mt-1 text-[13px] leading-[1.5] text-[rgba(0,0,0,0.4)]">{t('chatPreferences.priorityHint')}</p>
        </div>
        <div className="w-[280px] max-w-[280px] shrink-0 max-[720px]:w-full">
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
