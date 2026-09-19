import assert from 'node:assert/strict';
import test from 'node:test';
import { formatMessage, messages, supportedLocales } from '../src/index.ts';

// Ported from frontend/src/i18n/locales/*.ts -> datasource block. The mobile
// data-source screen consumes these keys; every locale must carry the same
// key set so a missing translation falls through to the key name instead of
// a mixed-language page.
const EDITOR_KEYS = [
  'dataSource.title',
  'dataSource.createTitle',
  'dataSource.editTitle',
  'dataSource.nameLabel',
  'dataSource.connectorTypeLabel',
  'dataSource.connectorSettingsLabel',
  'dataSource.namePlaceholder',
  'dataSource.credentialsLabel',
  'dataSource.testConnection',
  'dataSource.isRequired',
  'dataSource.save',
  'dataSource.field.appId',
  'dataSource.field.appSecret',
  'dataSource.field.integrationToken',
  'dataSource.field.apiToken',
  'dataSource.field.imaClientId',
  'dataSource.field.imaApiKey',
  'dataSource.field.baseUrl',
  'dataSource.field.feedUrls',
  'dataSource.field.authHeaders',
  'dataSource.connector.feishu',
  'dataSource.connector.feishu_drive',
  'dataSource.connector.notion',
  'dataSource.connector.yuque',
  'dataSource.connector.ima',
  'dataSource.connector.rss',
  'dataSource.connector.gitlab',
  'dataSource.connectorDesc.rss',
  'dataSource.connectorDesc.gitlab',
  'dataSource.gitlab.projects',
  'dataSource.gitlab.projectIdPlaceholder',
  'dataSource.gitlab.addProject',
  'dataSource.gitlab.projectRequired',
  'dataSource.drive.folderTokenLabel',
  'dataSource.drive.folderTokenPlaceholder',
  'dataSource.drive.folderTokenRequired',
  'dataSource.drive.load',
  'dataSource.resourceHint',
  'dataSource.noResources',
  'dataSource.retryLoadResources',
  'dataSource.untitled',
  'dataSource.step.resources',
  'dataSource.syncModeLabel',
  'dataSource.syncScheduleLabel',
  'dataSource.conflictLabel',
  'dataSource.syncDeletions',
  'dataSource.conflict.overwrite',
  'dataSource.conflict.skip',
  'dataSource.status.active',
  'dataSource.status.paused',
  'dataSource.status.error',
  'dataSource.syncMode.incremental',
  'dataSource.syncMode.full',
  'dataSource.logMetric.failed',
  'dataSource.neverSynced',
  'dataSource.testing',
  'dataSource.loadFailed',
  'dataSource.connectionTestFailed',
  'dataSource.knowledgeBaseRequired',
  'dataSource.permissionRequired',
  'dataSource.discardTemporaryFailed',
  'dataSource.resumeFailed',
  'dataSource.syncLogsLoadFailed',
  'dataSource.syncLogsLoadMoreFailed',
  'dataSource.noDeclaredCapabilities',
  'dataSource.connectorTypePlaceholder',
  'dataSource.cronSchedulePlaceholder',
  'dataSource.credentialsPlaceholder',
  'dataSource.settingsPlaceholder',
  // Vue credential.* block (frontend/src/i18n/locales/*.ts), flattened under
  // the dataSource. prefix for the edit-mode Replace/Remove step.
  'dataSource.credential.configured',
  'dataSource.credential.unconfigured',
  'dataSource.credential.configure',
  'dataSource.credential.update',
  'dataSource.credential.remove',
  'dataSource.credential.inputPlaceholder',
  'dataSource.credential.saveFailed',
  'dataSource.credential.removedToast',
  'dataSource.credential.removeFailed',
  'dataSource.credential.confirmRemovePrompt',
  'dataSource.credential.confirmRemove',
  // R451 A1: rss custom header rows editor (Vue model.editor.customHeaders*
  // block, relocated under the dataSource.credential prefix).
  'dataSource.credential.headerAdd',
  'dataSource.credential.headerKeyPlaceholder',
  'dataSource.credential.headerValuePlaceholder',
  // R453 A1: Vue DataSourceEditorDialog prereq setup-guide + doc-hint block
  // (frontend/src/i18n/locales/*.ts datasource block, byte-exact). Covers the
  // feishu/lark/feishu_drive/lark_drive/yuque per-type copy plus the shared
  // docHint/openDoc and fallback keys the Vue template references. The ima
  // per-type keys stay unported because Vue gates the guide on
  // requiredPermissions.length > 0 and ima declares none.
  'dataSource.docHint',
  'dataSource.openDoc',
  'dataSource.prereqBarText',
  'dataSource.prereqOpenConsole',
  'dataSource.prereqBotBrief',
  'dataSource.prereqBotDesc',
  'dataSource.prereqPermBrief',
  'dataSource.prereqMemberBrief',
  'dataSource.prereqMemberDesc',
  'dataSource.prereqStep1Brief_feishu', 'dataSource.prereqStep1Desc_feishu',
  'dataSource.prereqStep2Brief_feishu', 'dataSource.prereqStep2Desc_feishu',
  'dataSource.prereqStep3Brief_feishu', 'dataSource.prereqStep3Desc_feishu',
  'dataSource.prereqStep1Brief_lark', 'dataSource.prereqStep1Desc_lark',
  'dataSource.prereqStep2Brief_lark', 'dataSource.prereqStep2Desc_lark',
  'dataSource.prereqStep3Brief_lark', 'dataSource.prereqStep3Desc_lark',
  'dataSource.prereqStep1Brief_feishu_drive', 'dataSource.prereqStep1Desc_feishu_drive',
  'dataSource.prereqStep2Brief_feishu_drive', 'dataSource.prereqStep2Desc_feishu_drive',
  'dataSource.prereqStep3Brief_feishu_drive', 'dataSource.prereqStep3Desc_feishu_drive',
  'dataSource.prereqStep1Brief_lark_drive', 'dataSource.prereqStep1Desc_lark_drive',
  'dataSource.prereqStep2Brief_lark_drive', 'dataSource.prereqStep2Desc_lark_drive',
  'dataSource.prereqStep3Brief_lark_drive', 'dataSource.prereqStep3Desc_lark_drive',
  'dataSource.prereqBarText_yuque',
  'dataSource.prereqStep1Brief_yuque', 'dataSource.prereqStep1Desc_yuque',
  'dataSource.prereqStep2Brief_yuque', 'dataSource.prereqStep2Desc_yuque',
  'dataSource.prereqStep3Brief_yuque', 'dataSource.prereqStep3Desc_yuque',
  'dataSource.prereqOpenConsole_yuque',
];

function dataSourceKeys(locale: string): string[] {
  return Object.keys(messages[locale as keyof typeof messages]).filter((key) => key.startsWith('dataSource.'));
}

test('data-source log messages exist in every supported locale', () => {
  const keys = dataSourceKeys('en-US').sort();
  // 224 Vue-ported keys + 13 confluence/dingtalk connector/desc/field keys.
  assert.equal(keys.length, 237);
  for (const locale of supportedLocales) {
    assert.deepEqual(dataSourceKeys(locale).sort(), keys, `${locale} data-source messages diverge`);
    assert.notEqual(formatMessage(locale, 'dataSource.syncHistory'), 'dataSource.syncHistory');
    assert.notEqual(formatMessage(locale, 'dataSource.status.running'), 'dataSource.status.running');
  }
});

test('every locale carries the data-source editor keys', () => {
  for (const locale of supportedLocales) {
    for (const key of EDITOR_KEYS) {
      assert.ok(dataSourceKeys(locale).includes(key), `${locale} missing ${key}`);
      assert.notEqual(formatMessage(locale, key), key, `${locale} ${key} has no value`);
    }
  }
});

test('data-source editor copy is byte-exact against the Vue zh-CN baseline', () => {
  // frontend/src/i18n/locales/zh-CN.ts datasource block (lines 574-814).
  assert.equal(formatMessage('zh-CN', 'dataSource.title'), '数据源管理');
  assert.equal(formatMessage('zh-CN', 'dataSource.description'), '配置外部数据源，自动同步内容到知识库');
  assert.equal(formatMessage('zh-CN', 'dataSource.namePlaceholder'), '输入数据源名称');
  assert.equal(formatMessage('zh-CN', 'dataSource.connectorTypeLabel'), '连接器类型');
  assert.equal(formatMessage('zh-CN', 'dataSource.connectorSettingsLabel'), '连接器设置');
  assert.equal(formatMessage('zh-CN', 'dataSource.testConnection'), '测试连接');
  assert.equal(formatMessage('zh-CN', 'dataSource.isRequired'), '为必填项');
  assert.equal(formatMessage('zh-CN', 'dataSource.field.feedUrlsHint'), '每行一个 RSS / Atom 订阅源地址，支持同时填写多个');
  assert.equal(formatMessage('zh-CN', 'dataSource.connector.feishu'), '飞书');
  assert.equal(formatMessage('zh-CN', 'dataSource.connectorDesc.rss'), '同步 RSS / Atom 订阅源中的文章');
  assert.equal(formatMessage('zh-CN', 'dataSource.gitlab.projectRequired'), '请至少添加一个 GitLab 项目');
  assert.equal(formatMessage('zh-CN', 'dataSource.drive.folderTokenRequired'), '请输入具体文件夹的 folder_token，不支持云空间根目录');
  assert.equal(formatMessage('zh-CN', 'dataSource.syncDeletions'), '同步删除（源端删除时同步删除知识库中的条目）');
  assert.equal(formatMessage('zh-CN', 'dataSource.testing'), '测试中...');
});

// Vue credential.* block, byte-exact (frontend/src/i18n/locales/zh-CN.ts
// lines 375-388): the edit-mode Replace/Remove step copy.
test('credential step copy is byte-exact against the Vue credential block', () => {
  assert.equal(formatMessage('zh-CN', 'dataSource.credential.configured'), '已配置');
  assert.equal(formatMessage('zh-CN', 'dataSource.credential.unconfigured'), '未配置');
  assert.equal(formatMessage('zh-CN', 'dataSource.credential.update'), '更换');
  assert.equal(formatMessage('zh-CN', 'dataSource.credential.remove'), '移除');
  assert.equal(formatMessage('zh-CN', 'dataSource.credential.confirmRemovePrompt'), '确认移除？此操作不可撤销');
  assert.equal(formatMessage('en-US', 'dataSource.credential.update'), 'Replace');
  assert.equal(formatMessage('en-US', 'dataSource.credential.confirmRemovePrompt'), 'Remove this credential? This cannot be undone.');
  assert.equal(formatMessage('ja-JP', 'dataSource.credential.update'), '差し替え');
  assert.equal(formatMessage('ko-KR', 'dataSource.credential.update'), '교체');
  assert.equal(formatMessage('ru-RU', 'dataSource.credential.update'), 'Заменить');
});

// R451 A1: rss custom header rows copy, byte-exact against the Vue
// model.editor.customHeaders* block across all five locales.
test('rss custom header rows copy is byte-exact against the Vue editor block', () => {
  assert.equal(formatMessage('zh-CN', 'dataSource.credential.headerAdd'), '添加请求头');
  assert.equal(formatMessage('zh-CN', 'dataSource.credential.headerKeyPlaceholder'), 'Header 名称');
  assert.equal(formatMessage('zh-CN', 'dataSource.credential.headerValuePlaceholder'), 'Header 值');
  assert.equal(formatMessage('en-US', 'dataSource.credential.headerAdd'), 'Add Header');
  assert.equal(formatMessage('ja-JP', 'dataSource.credential.headerKeyPlaceholder'), 'ヘッダー名');
  assert.equal(formatMessage('ko-KR', 'dataSource.credential.headerValuePlaceholder'), '헤더 값');
  assert.equal(formatMessage('ru-RU', 'dataSource.credential.headerAdd'), 'Добавить заголовок');
});

test('data-source editor copy is byte-exact against the Vue en-US baseline', () => {
  // frontend/src/i18n/locales/en-US.ts datasource block.
  assert.equal(formatMessage('en-US', 'dataSource.title'), 'Data Sources');
  assert.equal(formatMessage('en-US', 'dataSource.testConnection'), 'Test Connection');
  assert.equal(formatMessage('en-US', 'dataSource.syncMode.incremental'), 'Incremental');
  assert.equal(formatMessage('en-US', 'dataSource.connector.gitlab'), 'GitLab');
  assert.equal(formatMessage('en-US', 'dataSource.testing'), 'Testing...');
});

test('relative-time templates interpolate the datasource counters', () => {
  assert.equal(formatMessage('zh-CN', 'dataSource.minutesAgo', { n: 5 }), '5 分钟前');
  assert.equal(formatMessage('en-US', 'dataSource.hoursAgo', { n: 2 }), '2h ago');
});

// R453 A1: Vue DataSourceEditorDialog prereq setup-guide + doc-hint copy,
// byte-exact against the frontend/src/i18n/locales/*.ts datasource block.
test('prereq setup-guide copy is byte-exact against the Vue zh-CN baseline', () => {
  assert.equal(formatMessage('zh-CN', 'dataSource.docHint'), '在以下地址获取凭证：');
  assert.equal(formatMessage('zh-CN', 'dataSource.openDoc'), '打开文档');
  assert.equal(formatMessage('zh-CN', 'dataSource.prereqBarText'), '首次使用？点击查看飞书应用配置指引');
  assert.equal(formatMessage('zh-CN', 'dataSource.prereqStep1Brief_feishu'), '创建飞书自建应用');
  assert.equal(formatMessage('zh-CN', 'dataSource.prereqStep2Desc_feishu'), '开放平台 → 你的应用 → 添加应用能力 → 机器人');
  assert.equal(formatMessage('zh-CN', 'dataSource.prereqStep3Desc_feishu'), '为应用开通 wiki:wiki:readonly, drive:drive:readonly, drive:export:readonly, docx:document:readonly 权限');
  assert.equal(formatMessage('zh-CN', 'dataSource.prereqStep3Desc_feishu_drive'), '为应用开通 drive:drive:readonly, drive:export:readonly, docx:document:readonly 权限');
  assert.equal(formatMessage('zh-CN', 'dataSource.prereqBarText_yuque'), '首次使用？点击查看语雀 Token 配置指引');
  assert.equal(formatMessage('zh-CN', 'dataSource.prereqOpenConsole_yuque'), '前往语雀 Token 设置');
  assert.equal(formatMessage('zh-CN', 'dataSource.prereqOpenConsole'), '前往飞书开放平台配置');
  assert.equal(formatMessage('zh-CN', 'dataSource.prereqBotBrief'), '为应用添加「机器人」能力');
  assert.equal(formatMessage('zh-CN', 'dataSource.prereqMemberDesc'), '创建群聊 → 添加应用为群机器人 → 将群聊添加为知识库成员');
});

test('prereq setup-guide copy is byte-exact against the Vue en-US baseline', () => {
  assert.equal(formatMessage('en-US', 'dataSource.docHint'), 'Get credentials at:');
  assert.equal(formatMessage('en-US', 'dataSource.openDoc'), 'Open documentation');
  assert.equal(formatMessage('en-US', 'dataSource.prereqStep1Brief_lark'), 'Create Lark custom app');
  assert.equal(formatMessage('en-US', 'dataSource.prereqStep3Desc_lark_drive'), 'Enable drive:drive:readonly, drive:export:readonly, docx:document:readonly permissions');
  assert.equal(formatMessage('en-US', 'dataSource.prereqOpenConsole'), 'Open Feishu Developer Console');
  assert.equal(formatMessage('en-US', 'dataSource.prereqPermBrief'), 'Grant API permissions');
});

test('prereq setup-guide copy is byte-exact against the Vue ja/ko/ru baselines', () => {
  assert.equal(formatMessage('ja-JP', 'dataSource.prereqBarText'), '初めてですか？クリックしてFeishuアプリの設定ガイドを表示');
  assert.equal(formatMessage('ja-JP', 'dataSource.openDoc'), 'ドキュメントを開く');
  assert.equal(formatMessage('ja-JP', 'dataSource.prereqOpenConsole'), 'Feishu開発者コンソールを開く');
  assert.equal(formatMessage('ko-KR', 'dataSource.prereqBotBrief'), "앱에 '봇' 기능 추가");
  assert.equal(formatMessage('ko-KR', 'dataSource.prereqOpenConsole'), 'Feishu 오픈 플랫폼 설정으로 이동');
  assert.equal(formatMessage('ru-RU', 'dataSource.docHint'), 'Получить учётные данные можно здесь:');
  assert.equal(formatMessage('ru-RU', 'dataSource.prereqOpenConsole'), 'Открыть настройки Feishu Open Platform');
  assert.equal(formatMessage('ru-RU', 'dataSource.prereqMemberBrief'), 'Добавьте через групповой чат как участника базы знаний');
});
