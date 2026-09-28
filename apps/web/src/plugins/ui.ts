/**
 * 插件面板共享 UI 常量与错误口径（OCR 终局 F02）。
 *
 * 此前四组 badge class 串在 PluginsPanel.tsx / PluginsSettingsPanel.tsx /
 * McpSettingsPanel.tsx（mcpBadge*）三处逐字重复，且「ApiError 判定 + 中文
 * 兜底」在两个插件面板各自维护（工具函数 vs 内联同款判定）——视觉或错误
 * 口径调整需同步三处、极易漂移。收敛为单一来源：面板层只 import，不再
 * 保留副本（值由 ui.test.ts 逐字锁定）。
 */

/* Tailwind utilities（三个面板统一的徽标视觉）。 */
export const pluginBadgeOk = "rounded-full px-2 py-[0.1rem] text-[.72rem] bg-[#ecfdf3] text-[#137333]";
export const pluginBadgeInfo = "rounded-full px-2 py-[0.1rem] text-[.72rem] bg-[#e8f1ff] text-[#2e6de6]";
export const pluginBadgeWarn = "rounded-full px-2 py-[0.1rem] text-[.72rem] bg-[#fffaeb] text-[#b54708]";
export const pluginBadgeMuted = "rounded-full px-2 py-[0.1rem] text-[.72rem] bg-[#f2f4f8] text-[#66758b]";

/** ApiError（后端/网络拒绝）原文透传；其余错误返回 null，由调用方兜底中文并 console.warn 留痕。 */
export function apiErrorMessage(cause: unknown): string | null {
  return cause instanceof Error && cause.name === "ApiError" ? cause.message : null;
}
