import { formatMessage, type Locale } from '@weknora/i18n';

export interface ChatSessionSourceOption {
  value: string;
  label: string;
}

export function resolveChatSessionSourceOptions(locale: Locale, canViewChannelSessions: boolean): ChatSessionSourceOption[] {
  const label = (key: string, fallback: string): string => {
    const translated = formatMessage(locale, key);
    return translated === key ? fallback : translated;
  };
  const options = [
    { value: '', key: 'knowledgeBase.allSources' },
    { value: 'web', key: 'knowledgeBase.channelWeb' },
    { value: 'embed', key: 'knowledgeBase.channelEmbed' },
    { value: 'api', key: 'knowledgeBase.channelApi' },
    { value: 'feishu', key: 'knowledgeBase.channelFeishu' },
    { value: 'wechat', key: 'knowledgeBase.channelWechat' },
    { value: 'slack', key: 'knowledgeBase.channelSlack' },
  ];
  return (canViewChannelSessions ? options : options.slice(1, 2)).map(({ value, key }) => ({
    value,
    label: label(key, value === 'embed' ? 'Embed' : value === '' ? 'All Sources' : value),
  }));
}
