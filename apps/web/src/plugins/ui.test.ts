import assert from 'node:assert/strict';
import test from 'node:test';

import { apiErrorMessage, pluginBadgeInfo, pluginBadgeMuted, pluginBadgeOk, pluginBadgeWarn } from './ui.ts';

// OCR 终局 F02：三面板（PluginsPanel/PluginsSettingsPanel/McpSettingsPanel）
// 共享的 badge 样式常量收敛为单一来源——常量值在这里逐字锁定，防止任一
// 面板私自漂移出不同视觉。
test('plugin badge class strings keep the shared visual tokens verbatim', () => {
  assert.equal(pluginBadgeOk, 'rounded-full px-2 py-[0.1rem] text-[.72rem] bg-[#ecfdf3] text-[#137333]');
  assert.equal(pluginBadgeInfo, 'rounded-full px-2 py-[0.1rem] text-[.72rem] bg-[#e8f1ff] text-[#2e6de6]');
  assert.equal(pluginBadgeWarn, 'rounded-full px-2 py-[0.1rem] text-[.72rem] bg-[#fffaeb] text-[#b54708]');
  assert.equal(pluginBadgeMuted, 'rounded-full px-2 py-[0.1rem] text-[.72rem] bg-[#f2f4f8] text-[#66758b]');
});

// ApiError（后端/网络拒绝）原文透传、其余错误返回 null 由调用方兜底中文
// ——口径与 PluginsSettingsPanel 既有工具函数一致，两插件面板共用。
test('apiErrorMessage passes ApiError text through and nulls everything else', () => {
  const apiError = new Error('服务端拒绝的原文');
  apiError.name = 'ApiError';
  assert.equal(apiErrorMessage(apiError), '服务端拒绝的原文');

  assert.equal(apiErrorMessage(new Error('plain error')), null);
  assert.equal(apiErrorMessage(undefined), null);
  assert.equal(apiErrorMessage('string failure'), null);
});
