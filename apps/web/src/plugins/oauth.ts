/**
 * MCP OAuth 授权的共享前端常量（OCR 终局第 2 轮 f01/f02）。
 *
 * 回调路径此前在 apps/web 内四处逐字副本（PluginsPanel / McpSettingsPanel /
 * ConfigurationEditor / ChatRoutePage 的 authorizeUrl redirectURI），后端路由
 * 调整无任何编译期保护，遗漏任一处该处 OAuth 授权即静默失效；授权弹窗参数
 * 与轮询节奏在两个面板各自维护且已分叉（一处常量化、一处内联魔数）。收敛
 * 为单一来源：消费方只 import，不再保留字面量副本（值由 oauth.test.ts 逐字
 * 锁定）。ChatRoutePage 的页面级弹窗名（'mcp_oauth'）与面板骨架语义不同，
 * 不在本模块收敛面。
 */

/** OAuth 授权码回传端点（后端路由 GET /api/v1/mcp-oauth/callback）。 */
export const MCP_OAUTH_CALLBACK_PATH = "/api/v1/mcp-oauth/callback";

/** 授权弹窗的窗口名与特性串（两面板同款骨架）。 */
export const MCP_OAUTH_POPUP_NAME = "weknora_mcp_oauth";
export const MCP_OAUTH_POPUP_FEATURES = "width=600,height=720";

/** 弹窗授权后的状态轮询节奏：间隔 × 次数（先例 McpSettingsPanel.startAuthorize）。 */
export const MCP_OAUTH_POLL_INTERVAL_MS = 1500;
export const MCP_OAUTH_POLL_ATTEMPTS = 40;
