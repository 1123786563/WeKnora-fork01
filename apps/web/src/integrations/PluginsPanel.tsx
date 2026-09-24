import * as React from "react";
import { useEffect, useState } from "react";
import type { WeKnoraClient } from "@weknora/api-client";
import { Status } from "@weknora/ui";
// 单一来源解析器（T08）：与 PluginsSettingsPanel 同款深路径复用（生产先例
// SandboxSettingsPanel.tsx），面板层不维护 wire-format 镜像副本。
import {
  createPluginsApi,
  type PluginInstallationSummary,
} from "../../../../packages/api-client/src/plugins.ts";

/**
 * 成员插件发现面板（Issue #110 / 计划 T08）。
 *
 * registry：packages/views/src/integrations/registry.ts 的 `plugins` section
 * （viewId PluginDiscoverPanel，minRole viewer——本空间全员可见）。
 *
 * 面板口径（计划 Task 8 Step 1 测试伪代码）：
 * - 列出本空间已安装插件的名称/版本/状态/漂移/需个人授权徽标；
 * - **只读发现面**——不渲染任何管理操作（不停用/不启用/不确认安装），
 *   全文不出现治理动词（状态徽标用「可用/不可用」，与管理侧
 *   「启用中/已停用」文案区分）；
 * - 卸载范围明示：不出现任何移除插件入口或文案（停用即治理终点，
 *   rulings.md R4——运维 DELETE 端点仅 API 层自愈通道，不进用户界面）。
 *
 * i18n 说明：直书中文字面量，迁移随 T12/T20 统一处理（先例
 * PluginsSettingsPanel）。
 */

/* Tailwind utilities（PluginsSettingsPanel 同款视觉）。 */
const pluginBadgeOk = "rounded-full px-2 py-[0.1rem] text-[.72rem] bg-[#ecfdf3] text-[#137333]";
const pluginBadgeInfo = "rounded-full px-2 py-[0.1rem] text-[.72rem] bg-[#e8f1ff] text-[#2e6de6]";
const pluginBadgeWarn = "rounded-full px-2 py-[0.1rem] text-[.72rem] bg-[#fffaeb] text-[#b54708]";
const pluginBadgeMuted = "rounded-full px-2 py-[0.1rem] text-[.72rem] bg-[#f2f4f8] text-[#66758b]";

type Props = {
  client: WeKnoraClient;
  /** SSR/首屏直出数据；挂载后仍会经 client 刷新（GET /plugins/installations，Viewer+）。 */
  initialInstallations?: readonly PluginInstallationSummary[];
};

export function PluginsPanel({ client, initialInstallations = [] }: Props) {
  const [installations, setInstallations] = useState<readonly PluginInstallationSummary[]>(initialInstallations);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    createPluginsApi((input) => client.request(input))
      .listInstallations()
      .then((rows) => {
        if (cancelled) return;
        setInstallations(rows);
        setError(null);
      })
      .catch((cause: unknown) => {
        if (cancelled) return;
        if (cause instanceof Error && cause.name === "ApiError") {
          setError(cause.message);
        } else {
          console.warn("plugin discoveries load failed:", cause);
          setError("空间插件目录加载失败");
        }
      });
    return () => {
      cancelled = true;
    };
  }, [client]);

  return (
    <section className="grid gap-3" data-testid="plugins-discover" aria-label="空间插件">
      <div>
        <h2 className="m-0 mb-1 text-[18px] font-semibold leading-[normal] text-[rgb(0_0_0_/_90%)]">空间插件</h2>
        <p className="wk-muted m-0 text-[13px] leading-[1.6] text-[rgb(0_0_0_/_60%)]">
          本空间已安装插件的目录：Agent 仅可调用空间已接受版本中开放的工具，写入类工具默认关闭，需个人授权的工具在会话中按成员本人的授权调用。
        </p>
      </div>
      {error ? <Status tone="error">{error}</Status> : null}
      {installations.length === 0 && error === null ? (
        <Status>暂无已安装插件</Status>
      ) : (
        <ul className="m-0 grid list-none gap-2 p-0">
          {installations.map((item) => (
            <li
              key={item.installationId}
              className="flex flex-wrap items-center gap-2 rounded-[10px] border border-[#dce3ed] bg-white p-3 text-[13px] leading-[20px]"
            >
              <span className="min-w-0 font-medium [overflow-wrap:anywhere]">{item.name}</span>
              <code className="text-[12px] text-[#66758b] [overflow-wrap:anywhere]">{item.pluginId}</code>
              <span className={pluginBadgeInfo}>{item.version}</span>
              {item.state === "active" ? (
                <span className={pluginBadgeOk}>可用</span>
              ) : (
                <span className={pluginBadgeMuted}>不可用</span>
              )}
              {item.driftState === "detected" ? <span className={pluginBadgeWarn}>检测到漂移</span> : null}
              {item.requiresPersonalAuth ? <span className={pluginBadgeInfo}>需个人授权</span> : null}
              <span className="text-[#66758b]">{item.toolCount} 个工具</span>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
