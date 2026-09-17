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
];

function dataSourceKeys(locale: string): string[] {
  return Object.keys(messages[locale as keyof typeof messages]).filter((key) => key.startsWith('dataSource.'));
}

test('data-source log messages exist in every supported locale', () => {
  const keys = dataSourceKeys('en-US').sort();
  assert.equal(keys.length, 183);
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
