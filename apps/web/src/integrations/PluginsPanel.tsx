import * as React from "react";
import { useEffect, useMemo, useState } from "react";
import type { WeKnoraClient } from "@weknora/api-client";
import { Button, Status } from "@weknora/ui";
// 单一来源解析器（T08）：与 PluginsSettingsPanel 同款深路径复用（生产先例
// SandboxSettingsPanel.tsx），面板层不维护 wire-format 镜像副本。
import {
  createPluginsApi,
  type PluginInstallationSummary,
  type PluginMyConnection,
} from "../../../../packages/api-client/src/plugins.ts";

/**
 * 成员插件发现面板（Issue #110 / 计划 T08；T12 增补成员个人授权/撤销入口）。
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
 * T12 个人授权口径（Issue #112，GAP-4）：`requires_personal_auth` 插件行
 * 渲染本人连接三态徽标（authorized 已授权 / expired 已过期 /
 * unauthorized 未授权）；「去授权」= 复用物化 service_id 上的既有
 * mcp oauth authorize-url 客户端方法（configuration.ts mcpOAuth）取授权
 * 地址并弹窗；「撤销」= 既有 mcp oauth revoke（DELETE token）后刷新
 * connections/me。授权动作作用于**本人**凭据（per-principal），不属于
 * 管理治理动词，与上面的只读口径不冲突。
 *
 * i18n 说明：直书中文字面量（同 T08 口径），迁移随 T20 统一处理（先例
 * PluginsSettingsPanel；locale 词条文件不在 T12 文件所有权内）。
 */

/* Tailwind utilities（PluginsSettingsPanel 同款视觉）。 */
const pluginBadgeOk = "rounded-full px-2 py-[0.1rem] text-[.72rem] bg-[#ecfdf3] text-[#137333]";
const pluginBadgeInfo = "rounded-full px-2 py-[0.1rem] text-[.72rem] bg-[#e8f1ff] text-[#2e6de6]";
const pluginBadgeWarn = "rounded-full px-2 py-[0.1rem] text-[.72rem] bg-[#fffaeb] text-[#b54708]";
const pluginBadgeMuted = "rounded-full px-2 py-[0.1rem] text-[.72rem] bg-[#f2f4f8] text-[#66758b]";

/** 弹窗授权后的状态轮询节奏（McpSettingsPanel startAuthorize 同款常量）。 */
const AUTH_POLL_INTERVAL_MS = 1500;
const AUTH_POLL_ATTEMPTS = 40;

type Props = {
  client: WeKnoraClient;
  /** SSR/首屏直出数据；挂载后仍会经 client 刷新（GET /plugins/installations，Viewer+）。 */
  initialInstallations?: readonly PluginInstallationSummary[];
};

type ConnectionMap = Partial<Record<string, PluginMyConnection>>;

export function PluginsPanel({ client, initialInstallations = [] }: Props) {
  const [installations, setInstallations] = useState<readonly PluginInstallationSummary[]>(initialInstallations);
  const [error, setError] = useState<string | null>(null);
  const [connections, setConnections] = useState<ConnectionMap>({});
  const [pendingId, setPendingId] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  // pluginsApi 稳定化（T08-OCR 同款：useMemo 只随 client 变化，避免 effect 反复重建）。
  const pluginsApi = useMemo(
    () => createPluginsApi((input) => client.request(input)),
    [client],
  );

  useEffect(() => {
    let cancelled = false;
    pluginsApi
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
  }, [pluginsApi]);

  // 需个人授权的插件行拉取本人连接状态（GET connections/me，Viewer+）；
  // 无 requires_personal_auth 的插件不发请求、不渲染授权区。
  useEffect(() => {
    let cancelled = false;
    for (const row of installations) {
      if (!row.requiresPersonalAuth) continue;
      if (connections[row.installationId] !== undefined) continue;
      pluginsApi
        .getMyConnection(row.installationId)
        .then((connection) => {
          if (cancelled) return;
          setConnections((prev) => ({ ...prev, [connection.installationId]: connection }));
        })
        .catch((cause: unknown) => {
          // 单行连接状态失败不打断整个目录（列表/其他行照常呈现）。
          console.warn("plugin connection load failed:", cause);
        });
    }
    return () => {
      cancelled = true;
    };
  }, [pluginsApi, installations, connections]);

  function applyConnection(next: PluginMyConnection) {
    setConnections((prev) => ({ ...prev, [next.installationId]: next }));
    return next;
  }

  function refreshConnection(installationId: string): Promise<PluginMyConnection> {
    return pluginsApi.getMyConnection(installationId).then(applyConnection);
  }

  /** 去授权：复用既有 mcp oauth authorize-url（物化 service_id）→ 弹窗 → 轮询至已授权。 */
  async function authorizeConnection(connection: PluginMyConnection) {
    if (pendingId !== null) return;
    setPendingId(connection.installationId);
    setActionError(null);
    try {
      if (typeof window === "undefined") return;
      const result = await client.configuration.mcp.oauth.authorizeUrl(connection.serviceId, {
        redirectURI: window.location.origin + "/api/v1/mcp-oauth/callback",
        frontendRedirect: window.location.href,
      });
      const popup = window.open(result.authorizationUrl, "weknora_mcp_oauth", "width=600,height=720");
      if (!popup) throw new Error("未能打开授权窗口，请检查浏览器弹窗设置");
      for (let attempt = 0; attempt < AUTH_POLL_ATTEMPTS; attempt += 1) {
        if (popup.closed) break;
        await new Promise((resolve) => window.setTimeout(resolve, AUTH_POLL_INTERVAL_MS));
        const next = await refreshConnection(connection.installationId);
        if (next.authorized) break;
      }
    } catch (cause) {
      setActionError(cause instanceof Error ? cause.message : "发起授权失败");
    } finally {
      setPendingId(null);
    }
  }

  /** 撤销：复用既有 mcp oauth revoke（DELETE token，仅影响本人）→ 刷新回未授权。 */
  async function revokeConnection(connection: PluginMyConnection) {
    if (pendingId !== null) return;
    setPendingId(connection.installationId);
    setActionError(null);
    try {
      await client.configuration.mcp.oauth.revoke(connection.serviceId);
      await refreshConnection(connection.installationId);
    } catch (cause) {
      setActionError(cause instanceof Error ? cause.message : "撤销授权失败");
    } finally {
      setPendingId(null);
    }
  }

  return (
    <section className="grid gap-3" data-testid="plugins-discover" aria-label="空间插件">
      <div>
        <h2 className="m-0 mb-1 text-[18px] font-semibold leading-[normal] text-[rgb(0_0_0_/_90%)]">空间插件</h2>
        <p className="wk-muted m-0 text-[13px] leading-[1.6] text-[rgb(0_0_0_/_60%)]">
          本空间已安装插件的目录：Agent 仅可调用空间已接受版本中开放的工具，写入类工具默认关闭，需个人授权的工具在会话中按成员本人的授权调用。
        </p>
      </div>
      {error ? <Status tone="error">{error}</Status> : null}
      {actionError ? <Status tone="error">{actionError}</Status> : null}
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
              <MemberConnectionControl
                connection={connections[item.installationId]}
                pending={pendingId === item.installationId}
                onAuthorize={authorizeConnection}
                onRevoke={revokeConnection}
              />
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

/**
 * 行内本人连接区（T12）：三态徽标 + 去授权/撤销入口。仅在插件
 * requires_personal_auth 且 connections/me 已加载后渲染；徽标/按钮全部
 * 描述「本人凭据」状态，非管理治理操作。
 */
function MemberConnectionControl({
  connection,
  pending,
  onAuthorize,
  onRevoke,
}: {
  connection: PluginMyConnection | undefined;
  pending: boolean;
  onAuthorize: (connection: PluginMyConnection) => Promise<void>;
  onRevoke: (connection: PluginMyConnection) => Promise<void>;
}) {
  if (!connection) return null;
  const badge =
    connection.state === "authorized"
      ? { className: pluginBadgeOk, label: "已授权" }
      : connection.state === "expired"
        ? { className: pluginBadgeWarn, label: "已过期" }
        : { className: pluginBadgeMuted, label: "未授权" };
  return (
    <span className="flex flex-wrap items-center gap-2" data-testid="plugin-my-connection">
      <span className={badge.className}>{badge.label}</span>
      {connection.state === "authorized" ? (
        <Button
          type="button"
          className="h-7 rounded-[6px] px-3 text-[12px]"
          disabled={pending}
          onClick={() => void onRevoke(connection)}
        >
          撤销
        </Button>
      ) : (
        <Button
          type="button"
          className="h-7 rounded-[6px] px-3 text-[12px]"
          disabled={pending}
          onClick={() => void onAuthorize(connection)}
        >
          去授权
        </Button>
      )}
    </span>
  );
}
