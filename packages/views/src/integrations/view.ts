import type { Locale } from '../../../i18n/src/index.ts';
import { EMBED_WIZARD_COPY_KEYS } from './embedWizardMessages.ts';
import { IM_WIZARD_COPY_KEYS } from './imWizardMessages.ts';
import { IM_DOC_URL, integrationsT, isUnresolvedMessage } from './messages.ts';
import type { IntegrationKey } from './registry.ts';

// View-model copy for the integrations surface, mirroring the Vue settings
// drawer sections in frontend/src/views/integrations/IntegrationSettingsSection.vue:
// each tab renders an h2 heading plus a section description (IM adds the
// 查看接入文档 doc link), then the channel panel
// (frontend/src/components/IMChannelPanel.vue, AgentEmbedChannelPanel.vue)
// with a "IM 渠道" count header and a dashed 添加渠道 tile.

export interface IntegrationSectionCopy {
  heading: string;
  description: string;
  /** IM only: label + url of the external integration guide link. */
  docLinkLabel?: string;
  docUrl?: string;
  channelsTitle: string;
  addTileLabel: string;
  emptyText: string;
  disabledLabel: string;
  unnamedLabel: string;
  /** Vue embedPublish.defaultChannelName, used when an embed channel has no name. */
  defaultChannelName: string;
  deleteConfirm: string;
  /** Which create form the tab panel uses. */
  createForm: 'im' | 'embed' | 'none';
}

const HEADING_KEYS: Record<IntegrationKey, string> = {
  im: 'integrations.tabs.im',
  embed: 'integrations.tabs.embed',
  api: 'integrations.api.title',
  cli: 'integrations.cli.title',
  chrome: 'integrations.chrome.title',
  claw: 'integrations.claw.title',
  // T08 (issue #110): member plugin discovery copy. CONSUMED IMMEDIATELY by
  // the integrations page tab strip (page.tsx renders t('integrations.tabs.'
  // + key) for every INTEGRATION_SECTIONS entry) and the section heading —
  // resolved via integrationsT's FALLBACK_STRINGS (messages.ts, all five
  // locales, T08-OCR1-F1). The SettingsPage sidebar label reads
  // @weknora/i18n's formatMessage DIRECTLY, so it stays a raw-key handoff
  // until that package owns integrations.tabs.plugins (same T03 precedent as
  // the settings 'plugins' section label).
  plugins: 'integrations.plugins.title',
};

const DESCRIPTION_KEYS: Record<IntegrationKey, string> = {
  im: 'agentEditor.im.description',
  embed: 'agentEditor.embed.description',
  api: 'integrations.api.subtitle',
  cli: 'integrations.cli.subtitle',
  chrome: 'integrations.chrome.subtitle',
  claw: 'integrations.claw.subtitle',
  plugins: 'integrations.plugins.subtitle',
};

interface ChannelListKeys {
  channelsTitle: string;
  addTileLabel: string;
  emptyText: string;
  disabledLabel: string;
  unnamedLabel: string;
  deleteConfirm: string;
}

const IM_LIST_KEYS: ChannelListKeys = {
  channelsTitle: 'agentEditor.im.channelsTitle',
  addTileLabel: 'agentEditor.im.addChannel',
  emptyText: 'agentEditor.im.empty',
  disabledLabel: 'agentEditor.im.disabled',
  unnamedLabel: 'agentEditor.im.unnamed',
  deleteConfirm: 'agentEditor.im.deleteConfirm',
};

const EMBED_LIST_KEYS: ChannelListKeys = {
  channelsTitle: 'embedPublish.channelsTitle',
  addTileLabel: 'embedPublish.create',
  emptyText: 'embedPublish.empty',
  disabledLabel: 'embedPublish.disabled',
  unnamedLabel: 'agentEditor.im.unnamed',
  deleteConfirm: 'embedPublish.deleteConfirm',
};

export function integrationSectionCopy(tab: IntegrationKey, locale: Locale): IntegrationSectionCopy {
  const heading = integrationsT(locale, HEADING_KEYS[tab]);
  const description = integrationsT(locale, DESCRIPTION_KEYS[tab]);
  if (tab === 'im') {
    return {
      heading,
      description,
      docLinkLabel: integrationsT(locale, 'agentEditor.im.docLink'),
      docUrl: IM_DOC_URL,
      channelsTitle: integrationsT(locale, IM_LIST_KEYS.channelsTitle),
      addTileLabel: integrationsT(locale, IM_LIST_KEYS.addTileLabel),
      emptyText: integrationsT(locale, IM_LIST_KEYS.emptyText),
      disabledLabel: integrationsT(locale, IM_LIST_KEYS.disabledLabel),
      unnamedLabel: integrationsT(locale, IM_LIST_KEYS.unnamedLabel),
      defaultChannelName: integrationsT(locale, IM_LIST_KEYS.unnamedLabel),
      deleteConfirm: integrationsT(locale, IM_LIST_KEYS.deleteConfirm),
      createForm: 'im',
    };
  }
  if (tab === 'embed') {
    return {
      heading,
      description,
      channelsTitle: integrationsT(locale, EMBED_LIST_KEYS.channelsTitle),
      addTileLabel: integrationsT(locale, EMBED_LIST_KEYS.addTileLabel),
      emptyText: integrationsT(locale, EMBED_LIST_KEYS.emptyText),
      disabledLabel: integrationsT(locale, EMBED_LIST_KEYS.disabledLabel),
      unnamedLabel: integrationsT(locale, EMBED_LIST_KEYS.unnamedLabel),
      defaultChannelName: integrationsT(locale, 'embedPublish.defaultChannelName'),
      deleteConfirm: integrationsT(locale, EMBED_LIST_KEYS.deleteConfirm),
      createForm: 'embed',
    };
  }
  return {
    heading,
    description,
    channelsTitle: '',
    addTileLabel: '',
    emptyText: '',
    disabledLabel: integrationsT(locale, 'agentEditor.im.disabled'),
    unnamedLabel: integrationsT(locale, 'agentEditor.im.unnamed'),
    defaultChannelName: integrationsT(locale, 'embedPublish.defaultChannelName'),
    deleteConfirm: integrationsT(locale, 'integrations.api.deleteApiKeyConfirm'),
    createForm: 'none',
  };
}

/** Vue platform list order: IMChannelPanel.vue platformOptions. */
export function imPlatformOrder(): string[] {
  return ['feishu', 'lark', 'wecom', 'slack', 'telegram', 'dingtalk', 'mattermost', 'wechat', 'qqbot', 'yunzhijia'];
}

export function imPlatformLabels(locale: Locale): Record<string, string> {
  const labels: Record<string, string> = {};
  for (const platform of imPlatformOrder()) {
    labels[platform] = integrationsT(locale, 'agentEditor.im.' + platform);
  }
  return labels;
}

export function imPlatformLabel(platform: string, locale: Locale): string {
  return integrationsT(locale, 'agentEditor.im.' + platform);
}

/** Flags every copy key the channel tabs depend on that fails to resolve. */
export function unresolvedCopyKeys(tabs: readonly ('im' | 'embed')[], locale: Locale): string[] {
  const keys = [
    'agentEditor.im.channelsTitle', 'agentEditor.im.addChannel', 'agentEditor.im.empty',
    'agentEditor.im.disabled', 'agentEditor.im.unnamed', 'agentEditor.im.deleteConfirm',
    'agentEditor.im.docLink', 'agentEditor.im.description', 'agentEditor.embed.description',
    'embedPublish.channelsTitle', 'embedPublish.create', 'embedPublish.empty',
    'embedPublish.disabled', 'embedPublish.deleteConfirm',
    ...imPlatformOrder().map((platform) => 'agentEditor.im.' + platform),
    ...IM_WIZARD_COPY_KEYS,
    ...EMBED_WIZARD_COPY_KEYS,
  ];
  return keys.filter((key) => isUnresolvedMessage(locale, key));
}
