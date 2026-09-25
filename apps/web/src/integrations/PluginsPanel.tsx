// OCR R2 F07：删除死导入 import * as React（全文件无 React. 命名空间引用，
// jsx: react-jsx 下 JSX 不依赖它）。
import { useEffect, useMemo, useRef, useState } from "react";
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

/** 'load-failed' 墓碑键：该行连接加载失败（T12-OCR1-F4——失败态可见，不静默缺失）。 */
type ConnectionEntry = PluginMyConnection | "load-failed";
type ConnectionMap = Partial<Record<string, ConnectionEntry>>;

export function PluginsPanel({ client, initialInstallations }: Props) {
  const [installations, setInstallations] = useState<readonly PluginInstallationSummary[]>(initialInstallations ?? []);
  const [error, setError] = useState<string | null>(null);
  const [connections, setConnections] = useState<ConnectionMap>({});
  const [pendingId, setPendingId] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  // T12-OCR1-F4：本挂载周期内已发起过 connections/me 的 installationId（含
  // 失败——失败落墓碑键后不再由 effect 重发，其他行的成功也不会放大成
  // 对失败行的重试；显式重试走 retryConnection 清墓碑）。
  const requestedRef = useRef<Set<string>>(new Set());
  // OCR R2 F02：连接状态代数——client 切换时自增，在途回包凭旧代数丢弃。
  const connectionEpochRef = useRef(0);
  // OCR R2 F08：授权轮询的卸载终止标志——面板挂于 plugins tab 内容插槽
  //（tab 切换即卸载），卸载后轮询不再发后台请求。
  const aliveRef = useRef(true);
  useEffect(() => () => { aliveRef.current = false; }, []);
  // OCR R2 F06：列表加载完成标记——生产路径（未传 initialInstallations，即
  // 无 SSR 直出）挂载首拉期间不闪现「暂无已安装插件」空态误报；直出路径
  //（含空数组——服务端已确认空列表）直接视为已加载。
  const [listLoaded, setListLoaded] = useState(initialInstallations !== undefined);

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
        setListLoaded(true); // OCR R2 F06：首拉完成，空态判断自此有效。
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
  // 无 requires_personal_auth 的插件不发请求、不渲染授权区。去重由
  // requestedRef 承担（T12-OCR1-F4）：effect 仅随列表变化跑，晚到的响应经
  // 函数式 setState 落键（组件已卸载则为 no-op），无请求放大、无在途丢弃。
  useEffect(() => {
    for (const row of installations) {
      if (!row.requiresPersonalAuth) continue;
      if (requestedRef.current.has(row.installationId)) continue;
      requestedRef.current.add(row.installationId);
      void loadConnection(row.installationId);
    }
  }, [pluginsApi, installations]);

  // OCR R1 F42：client 切换（跨账号/工作区，页面不重挂载）时连接缓存必须
  // 失效——pluginsApi 重建会刷新列表，但 requestedRef 去重集仍持有全部
  // installationId，上一 effect 对每行直接 continue，旧 principal 的
  // 「已授权/已过期」徽标与 serviceId 原样残留展示在新 client 视图下。
  // 重置去重集与连接表，让新列表到位后按新 principal 重新拉取
  //（对照 PluginsSettingsPanel 的显式失效先例 T19-OCR1-F4/F1）。
  // OCR R2 F03：跳过首跑——effects 按声明序运行，无条件重置会在挂载首帧
  // 清掉第二个 effect 刚基于 initialInstallations 做的标记与请求，SSR/首
  // 屏直出路径下每个需授权行的连接状态被请求两次。
  const seenPluginsApiRef = useRef(pluginsApi);
  useEffect(() => {
    if (seenPluginsApiRef.current === pluginsApi) return;
    seenPluginsApiRef.current = pluginsApi;
    requestedRef.current = new Set();
    setConnections({});
    // OCR R2 F02：代际自增——已在途的 getMyConnection 回包凭旧代数被丢弃，
    // 旧 principal 的徽标不再覆盖新视图（先例 toolPolicyEpoch T19-OCR2-F1）。
    connectionEpochRef.current += 1;
  }, [pluginsApi]);

  function loadConnection(installationId: string): Promise<PluginMyConnection | null> {
    // OCR R2 F02：捕获发起时代数——client 切换（上面 effect 已自增）后旧
    // principal 的迟到回包/迟到失败直接丢弃，不写进新视图。
    const epoch = connectionEpochRef.current;
    return pluginsApi
      .getMyConnection(installationId)
      .then((next) => {
        if (epoch !== connectionEpochRef.current) return null;
        setConnections((prev) => ({ ...prev, [next.installationId]: next }));
        return next;
      })
      .catch((cause: unknown) => {
        if (epoch !== connectionEpochRef.current) return null;
        // 单行连接状态失败不打断整个目录（列表/其他行照常呈现），但落
        // 墓碑键让行内呈现失败态与重试入口（T12-OCR1-F4）。
        console.warn("plugin connection load failed:", cause);
        setConnections((prev) => ({ ...prev, [installationId]: "load-failed" }));
        return null;
      });
  }

  function retryConnection(installationId: string) {
    if (pendingId !== null) return;
    setPendingId(installationId);
    // 墓碑清理不需要动 requestedRef（OCR R1 F35）：墓碑键落在 connections
    // state 而非去重集，重试的 loadConnection 成功/失败都会覆盖
    // connections[id]，去重集语义（本挂载周期已发起过）不受影响。
    void loadConnection(installationId).finally(() => {
      setPendingId(null);
    });
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
      // T12-OCR1-F5：对齐 McpSettingsPanel.startAuthorize 先例顺序——先刷新
      // 先判 authorized 再判 popup.closed（用户授权完成后随手关弹窗落在
      // 轮询间隔内时，最终刷新不得被吞掉）；循环因关窗或耗尽退出后兜底
      // 一次最终刷新。
      let authorized = false;
      for (let attempt = 0; attempt < AUTH_POLL_ATTEMPTS; attempt += 1) {
        await new Promise((resolve) => window.setTimeout(resolve, AUTH_POLL_INTERVAL_MS));
        // OCR R2 F08：面板随 tab 切换卸载后终止轮询——setState 虽 no-op，
        // 但每 1.5s×40 的后台 getMyConnection 请求照发。
        if (!aliveRef.current) break;
        const next = await loadConnection(connection.installationId);
        if (next?.authorized) {
          authorized = true;
          break;
        }
        if (popup.closed) break;
      }
      if (!authorized) await loadConnection(connection.installationId);
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
      await loadConnection(connection.installationId);
    } catch (cause) {
      setActionError(cause instanceof Error ? cause.message : "撤销授权失败");
    } finally {
      setPendingId(null);
    }
  }

  return (
    <section className="grid gap-3" data-testid="plugins-discover" aria-label="空间插件">
      {/* OCR R2 F04：标题/描述由共享页 section heading 渲染（page.tsx 的
          HEADING_KEYS/DESCRIPTION_KEYS，五 locale 词条）——面板此前自带同名
          h2+描述段造成双重渲染且文案分叉；「需个人授权」分句已并入
          integrations.plugins.subtitle 词条。 */}
      {error ? <Status tone="error">{error}</Status> : null}
      {actionError ? <Status tone="error">{actionError}</Status> : null}
      {!listLoaded && error === null ? (
        <Status>加载中…</Status>
      ) : installations.length === 0 && error === null ? (
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
                installationId={item.installationId}
                entry={connections[item.installationId]}
                pending={pendingId === item.installationId}
                // OCR R1 F43：pendingId 是全面板单锁（授权轮询最长约 60s），
                // 任何操作在途时冻结所有行入口——否则其他行按钮可点但点击
                // 被各函数首行守卫静默吞掉，用户零反馈（与 PluginsSettingsPanel
                // 的跨行 disabled 口径一致）。
                anyPending={pendingId !== null}
                onAuthorize={authorizeConnection}
                onRevoke={revokeConnection}
                onRetry={retryConnection}
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
  installationId,
  entry,
  pending,
  anyPending,
  onAuthorize,
  onRevoke,
  onRetry,
}: {
  installationId: string;
  entry: ConnectionEntry | undefined;
  pending: boolean;
  anyPending: boolean;
  onAuthorize: (connection: PluginMyConnection) => Promise<void>;
  onRevoke: (connection: PluginMyConnection) => Promise<void>;
  onRetry: (installationId: string) => void;
}) {
  if (entry === undefined) return null;
  if (entry === "load-failed") {
    // T12-OCR1-F4：连接加载失败不再静默缺失——行内可见失败态 + 重试入口。
    return (
      <span className="flex flex-wrap items-center gap-2" data-testid="plugin-my-connection">
        <span className={pluginBadgeMuted}>授权状态加载失败</span>
        <Button type="button" className="h-7 rounded-[6px] px-3 text-[12px]" loading={pending} disabled={anyPending} onClick={() => onRetry(installationId)}>
          重试
        </Button>
      </span>
    );
  }
  const connection = entry;
  const badge =
    connection.state === "authorized"
      ? { className: pluginBadgeOk, label: "已授权" }
      : connection.state === "expired"
        ? { className: pluginBadgeWarn, label: "已过期" }
        : { className: pluginBadgeMuted, label: "未授权" };
  // T12-OCR1-F3：service_id 空（confirm 中断窗口且无孤儿服务可自愈）时无
  // OAuth 端点可指——徽标照常呈现，入口禁用而非发起注定失败的请求。
  // anyPending（OCR R1 F43）：任一行操作在途即全行冻结。
  const noService = connection.serviceId === "";
  const frozen = anyPending || noService;
  return (
    <span className="flex flex-wrap items-center gap-2" data-testid="plugin-my-connection">
      <span className={badge.className}>{badge.label}</span>
      {connection.state === "authorized" ? (
        <Button
          type="button"
          className="h-7 rounded-[6px] px-3 text-[12px]"
          loading={pending}
          disabled={frozen}
          title={noService ? "插件服务尚未就绪，请稍后重试" : undefined}
          onClick={() => void onRevoke(connection)}
        >
          撤销
        </Button>
      ) : (
        <Button
          type="button"
          className="h-7 rounded-[6px] px-3 text-[12px]"
          loading={pending}
          disabled={frozen}
          title={noService ? "插件服务尚未就绪，请稍后重试" : undefined}
          onClick={() => void onAuthorize(connection)}
        >
          去授权
        </Button>
      )}
    </span>
  );
}
