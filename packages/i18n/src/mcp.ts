import type { Locale } from './index.ts';

/** Shared MCP surface copy; Web and native clients consume the same keys. */
export const mcpMessages: Record<Locale, Record<string, string>> = {
  'zh-CN': {
    'mcpServiceDialog.authTypeOAuth': 'OAuth 2.0（首次连接授权）',
    'mcpServiceDialog.oauthAuthorize': '去授权',
    'mcpServiceDialog.oauthReauthorize': '重新授权',
    'mcpServiceDialog.oauthRevoke': '撤销授权',
    'mcpServiceDialog.testConnection': '测试连接',
    'mcpSettings.toasts.updateFailed': '更新 MCP 服务失败',
  },
  'en-US': {
    'mcpServiceDialog.authTypeOAuth': 'OAuth 2.0',
    'mcpServiceDialog.oauthAuthorize': 'Authorize',
    'mcpServiceDialog.oauthReauthorize': 'Re-authorize',
    'mcpServiceDialog.oauthRevoke': 'Revoke',
    'mcpServiceDialog.testConnection': 'Test connection',
    'mcpSettings.toasts.updateFailed': 'Unable to update MCP service',
  },
  'ja-JP': {
    'mcpServiceDialog.authTypeOAuth': 'OAuth 2.0',
    'mcpServiceDialog.oauthAuthorize': '認証する',
    'mcpServiceDialog.oauthReauthorize': '再認証',
    'mcpServiceDialog.oauthRevoke': '認証を取り消す',
    'mcpServiceDialog.testConnection': '接続をテスト',
    'mcpSettings.toasts.updateFailed': 'MCP サービスの更新に失敗しました',
  },
  'ko-KR': {
    'mcpServiceDialog.authTypeOAuth': 'OAuth 2.0',
    'mcpServiceDialog.oauthAuthorize': '인증',
    'mcpServiceDialog.oauthReauthorize': '재인증',
    'mcpServiceDialog.oauthRevoke': '인증 취소',
    'mcpServiceDialog.testConnection': '연결 테스트',
    'mcpSettings.toasts.updateFailed': 'MCP 서비스 업데이트 실패',
  },
  'ru-RU': {
    'mcpServiceDialog.authTypeOAuth': 'OAuth 2.0',
    'mcpServiceDialog.oauthAuthorize': 'Авторизовать',
    'mcpServiceDialog.oauthReauthorize': 'Авторизовать снова',
    'mcpServiceDialog.oauthRevoke': 'Отозвать',
    'mcpServiceDialog.testConnection': 'Проверить соединение',
    'mcpSettings.toasts.updateFailed': 'Не удалось обновить MCP-сервис',
  },
};
