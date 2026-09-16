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
];

function dataSourceKeys(locale: string): string[] {
  return Object.keys(messages[locale as keyof typeof messages]).filter((key) => key.startsWith('dataSource.'));
}

test('data-source log messages exist in every supported locale', () => {
  const keys = dataSourceKeys('en-US').sort();
  assert.equal(keys.length, 169);
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
