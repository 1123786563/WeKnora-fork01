import type { Locale } from './index.ts';

/** Shared MCP surface copy; Web and native clients consume the same keys. */
export const mcpMessages: Record<Locale, Record<string, string>> = {
  'zh-CN': {
    'mcpServiceDialog.authTypeOAuth': 'OAuth 2.0（首次连接授权）',
    // oauthAuthorization/oauthAuthorized/oauthUnauthorized exist in NO Vue locale
    // file — McpServiceDialog.vue:208/210/217 renders them through vue-i18n
    // inline defaults, so every locale (including en-US) shows these Chinese
    // strings. Mirrored byte-exact for parity; do not translate per-locale.
    'mcpServiceDialog.oauthAuthorization': '授权状态',
    'mcpServiceDialog.oauthAuthorized': '已授权',
    'mcpServiceDialog.oauthUnauthorized': '未授权',
    'mcpServiceDialog.oauthAuthorize': '去授权',
    'mcpServiceDialog.oauthReauthorize': '重新授权',
    'mcpServiceDialog.oauthRevoke': '撤销授权',
    'mcpServiceDialog.testConnection': '测试连接',
    'mcpServiceDialog.oauthScopes': 'Scopes（可选，空格分隔）',
    'mcpMetadata.saveNext': '保存并下一步',
    'mcpSettings.toasts.updateFailed': '更新 MCP 服务失败',
  },
  'en-US': {
    // Inline-default keys (see zh-CN note): Vue shows the Chinese defaults in every locale.
    'mcpServiceDialog.oauthAuthorization': '授权状态',
    'mcpServiceDialog.oauthAuthorized': '已授权',
    'mcpServiceDialog.oauthUnauthorized': '未授权',
    'mcpServiceDialog.authTypeOAuth': 'OAuth 2.0',
    'mcpServiceDialog.oauthAuthorize': 'Authorize',
    'mcpServiceDialog.oauthReauthorize': 'Re-authorize',
    'mcpServiceDialog.oauthRevoke': 'Revoke',
    'mcpServiceDialog.testConnection': 'Test connection',
    'mcpServiceDialog.oauthScopes': 'Scopes (optional, space-separated)',
    'mcpMetadata.saveNext': 'Save and continue',
    'mcpSettings.toasts.updateFailed': 'Unable to update MCP service',
  },
  'ja-JP': {
    // Inline-default keys (see zh-CN note): Vue shows the Chinese defaults in every locale.
    'mcpServiceDialog.oauthAuthorization': '授权状态',
    'mcpServiceDialog.oauthAuthorized': '已授权',
    'mcpServiceDialog.oauthUnauthorized': '未授权',
    'mcpServiceDialog.authTypeOAuth': 'OAuth 2.0',
    'mcpServiceDialog.oauthAuthorize': '認証する',
    'mcpServiceDialog.oauthReauthorize': '再認証',
    'mcpServiceDialog.oauthRevoke': '認証を取り消す',
    'mcpServiceDialog.testConnection': '接続をテスト',
    'mcpServiceDialog.oauthScopes': 'Scopes（任意、スペース区切り）',
    'mcpMetadata.saveNext': '保存して次へ',
    'mcpSettings.toasts.updateFailed': 'MCP サービスの更新に失敗しました',
  },
  'ko-KR': {
    // Inline-default keys (see zh-CN note): Vue shows the Chinese defaults in every locale.
    'mcpServiceDialog.oauthAuthorization': '授权状态',
    'mcpServiceDialog.oauthAuthorized': '已授权',
    'mcpServiceDialog.oauthUnauthorized': '未授权',
    'mcpServiceDialog.authTypeOAuth': 'OAuth 2.0',
    'mcpServiceDialog.oauthAuthorize': '인증',
    'mcpServiceDialog.oauthReauthorize': '재인증',
    'mcpServiceDialog.oauthRevoke': '인증 취소',
    'mcpServiceDialog.testConnection': '연결 테스트',
    'mcpServiceDialog.oauthScopes': 'Scopes (선택 사항, 공백으로 구분)',
    'mcpMetadata.saveNext': '저장 후 다음',
    'mcpSettings.toasts.updateFailed': 'MCP 서비스 업데이트 실패',
  },
  'ru-RU': {
    // Inline-default keys (see zh-CN note): Vue shows the Chinese defaults in every locale.
    'mcpServiceDialog.oauthAuthorization': '授权状态',
    'mcpServiceDialog.oauthAuthorized': '已授权',
    'mcpServiceDialog.oauthUnauthorized': '未授权',
    'mcpServiceDialog.authTypeOAuth': 'OAuth 2.0',
    'mcpServiceDialog.oauthAuthorize': 'Авторизовать',
    'mcpServiceDialog.oauthReauthorize': 'Авторизовать снова',
    'mcpServiceDialog.oauthRevoke': 'Отозвать',
    'mcpServiceDialog.testConnection': 'Проверить соединение',
    'mcpServiceDialog.oauthScopes': 'Области (необязательно, через пробел)',
    'mcpMetadata.saveNext': 'Сохранить и продолжить',
    'mcpSettings.toasts.updateFailed': 'Не удалось обновить MCP-сервис',
  },
};
