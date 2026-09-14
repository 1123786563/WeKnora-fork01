import type { Locale } from '../index.ts';

/** Native organization management fallbacks that are not present in a server share payload. */
export const mobileOrganizationMessages: Record<Locale, Record<string, string>> = {
  'zh-CN': {
    'mobileOrganization.unnamedKnowledgeBase': '未命名知识库',
    'mobileOrganization.unnamedAgent': '未命名智能体',
  },
  'en-US': {
    'mobileOrganization.unnamedKnowledgeBase': 'Unnamed knowledge base',
    'mobileOrganization.unnamedAgent': 'Unnamed agent',
  },
  'ja-JP': {
    'mobileOrganization.unnamedKnowledgeBase': '名前のないナレッジベース',
    'mobileOrganization.unnamedAgent': '名前のないエージェント',
  },
  'ko-KR': {
    'mobileOrganization.unnamedKnowledgeBase': '이름 없는 지식 베이스',
    'mobileOrganization.unnamedAgent': '이름 없는 에이전트',
  },
  'ru-RU': {
    'mobileOrganization.unnamedKnowledgeBase': 'База знаний без названия',
    'mobileOrganization.unnamedAgent': 'Агент без названия',
  },
};
