import assert from 'node:assert/strict';
import test from 'node:test';

import {
  MCP_OAUTH_CALLBACK_PATH,
  MCP_OAUTH_POLL_ATTEMPTS,
  MCP_OAUTH_POLL_INTERVAL_MS,
  MCP_OAUTH_POPUP_FEATURES,
  MCP_OAUTH_POPUP_NAME,
} from './oauth.ts';

// OCR 终局第 2 轮 f01："/api/v1/mcp-oauth/callback" 此前在 apps/web 内四处
// 逐字副本（PluginsPanel/McpSettingsPanel/ConfigurationEditor/ChatRoutePage），
// 后端路由调整无编译期保护。常量值在此逐字锁定（与四处现行字面量一致），
// 四处统一引用后调整只动一处。
test('MCP OAuth callback path constant matches the backend route verbatim', () => {
  assert.equal(MCP_OAUTH_CALLBACK_PATH, '/api/v1/mcp-oauth/callback');
});

// f02：授权弹窗参数与轮询节奏此前在两面板各自维护（PluginsPanel 常量化、
// McpSettingsPanel 内联魔数）且已分叉——收敛为单一来源，值与现行一致。
test('OAuth popup and polling constants lock the shared authorization skeleton values', () => {
  assert.equal(MCP_OAUTH_POPUP_NAME, 'weknora_mcp_oauth');
  assert.equal(MCP_OAUTH_POPUP_FEATURES, 'width=600,height=720');
  assert.equal(MCP_OAUTH_POLL_INTERVAL_MS, 1500);
  assert.equal(MCP_OAUTH_POLL_ATTEMPTS, 40);
});
