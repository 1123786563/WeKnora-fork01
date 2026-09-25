import * as React from "react";
import { useEffect, useMemo, useRef, useState } from "react";
import type { WeKnoraClient } from "@weknora/api-client";
import { Button, Input, Status } from "@weknora/ui";
import { roleAtLeast } from "@weknora/views/settings/registry";
// 单一来源解析器/请求构造（T08 收敛）：packages/api-client/src/plugins.ts 的
// 严格 envelope 契约直接深路径复用（生产先例 SandboxSettingsPanel.tsx 对
// packages/views/src/settings/registry.ts），不再维护面板层镜像副本——转交
// 发现 T06-OCR3-F1/T03-OCR1-F1/T07-OCR1-F1/T09-OCR1-F3 的统一解法。
import {
  createPluginsApi,
  type PluginInstallationSummary,
  type PluginPreviewResult,
  type PluginToolPolicyRow,
  type PluginUpgradePreview,
  type PluginUpgradeToolChange,
  type PluginUpgradeToolSnapshot,
} from "../../../../packages/api-client/src/plugins.ts";

/**
 * 管理端插件面板（Issue #108/#110 / 计划 T03 + T08）。
 *
 * 空间管理员粘贴插件清单 URL → 核验预览 → 在预览卡上「确认安装」（消费
 * preview_id，POST /plugins/installations）→ 已安装列表出现该插件（版本/
 * 状态/漂移徽标），管理侧可停用/启用。卸载范围明示（计划 Task 8 Step 3
 * 与 rulings.md R4）：本版不提供插件删除入口，用户可见的治理终点是
 * 「停用」——停用后 Agent 不再调用，成员连接与审计记录保留。
 *
 * schema 展示取舍（Spec US9）：平台核验、界面不渲染原文——后端仅在清单
 * 声明的 input_schema_digest 与远端实际目录 digest 一致时才产生预览。
 *
 * i18n 说明：本面板文案为直书中文字面量——packages/i18n 不在本任务文件
 * 所有权内，i18n key 迁移由后续任务统一（先例：McpSettingsPanel 走
 * createTranslator，此处所有权受限暂不跟随）。
 */

/* Tailwind utilities（McpSettingsPanel mcpBadge* 同款视觉）。 */
const pluginBadgeOk = "rounded-full px-2 py-[0.1rem] text-[.72rem] bg-[#ecfdf3] text-[#137333]";
const pluginBadgeInfo = "rounded-full px-2 py-[0.1rem] text-[.72rem] bg-[#e8f1ff] text-[#2e6de6]";
const pluginBadgeWarn = "rounded-full px-2 py-[0.1rem] text-[.72rem] bg-[#fffaeb] text-[#b54708]";
const pluginBadgeMuted = "rounded-full px-2 py-[0.1rem] text-[.72rem] bg-[#f2f4f8] text-[#66758b]";

/** 预览有效期本地化；非法时间串原样回显（不猜测远端数据语义）。 */
export function formatPreviewExpiry(expiresAt: string, locale = "zh-CN"): string {
  const date = new Date(expiresAt);
  return Number.isNaN(date.getTime()) ? expiresAt : date.toLocaleString(locale);
}

/** ApiError（后端/网络拒绝）原文透传；其余错误统一中文并 console.warn 留痕。 */
function apiErrorMessage(cause: unknown): string | null {
  return cause instanceof Error && cause.name === "ApiError" ? cause.message : null;
}

/** 升级差异面板的挂载状态：记录来源插件名，重开/切换互斥渲染。 */
type UpgradePreviewState = {
  pluginName: string;
  result: PluginUpgradePreview;
};

/** 变更理由徽标：四维独立（schema/scope/读写分类/授权面），仅渲染命中的维度。 */
function changeReasonBadges(change: PluginUpgradeToolChange): React.ReactNode {
  const badges: string[] = [];
  if (change.schemaChanged) badges.push("schema 变更");
  if (change.scopeChanged) badges.push("scope 变更");
  if (change.readWriteClassChanged) badges.push("读写分类变更");
  if (change.personalAuthChanged) badges.push("授权面变更");
  return badges.map((label) => (
    <span key={label} className={pluginBadgeWarn}>
      {label}
    </span>
  ));
}

/** 快照列紧凑摘要：scopes · 只读/写入 · 授权面（供变更行现版/候选版对照）。 */
function snapshotSummary(snapshot: PluginUpgradeToolSnapshot): string {
  const parts = [
    snapshot.scopes.length > 0 ? snapshot.scopes.join(" ") : "无 scope",
    snapshot.readOnly ? "只读" : "写入",
    snapshot.requiresPersonalAuth ? "需个人授权" : "无需授权",
  ];
  return parts.join(" · ");
}

/** 新增/移除工具表（同一行结构，标题与数据不同）。 */
function DiffToolTable({ caption, rows }: { caption: string; rows: readonly PluginUpgradeToolSnapshot[] }) {
  return (
    <div className="overflow-x-auto">
      <h4 className="m-0 mb-1 text-[13px] font-medium">{caption}（{rows.length}）</h4>
      <table className="w-full border-collapse text-left text-[13px] leading-[20px]">
        <thead>
          <tr className="border-b border-[#eef1f5] text-[12px] text-[#66758b]">
            <th className="py-1 pr-3 font-medium">工具</th>
            <th className="py-1 pr-3 font-medium">说明</th>
            <th className="py-1 pr-3 font-medium">分类</th>
            <th className="py-1 pr-3 font-medium">授权</th>
            <th className="py-1 font-medium">Scopes</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((tool) => (
            <tr key={tool.name} className="border-b border-[#f3f5f8] align-top last:border-b-0">
              <td className="py-2 pr-3 font-medium [overflow-wrap:anywhere]">{tool.name}</td>
              <td className="py-2 pr-3 text-[#66758b] [overflow-wrap:anywhere]">{tool.description}</td>
              <td className="py-2 pr-3">
                <span className={tool.readOnly ? pluginBadgeOk : pluginBadgeWarn}>{tool.readOnly ? "只读" : "写入"}</span>
              </td>
              <td className="py-2 pr-3">
                {tool.requiresPersonalAuth ? (
                  <span className={pluginBadgeInfo}>需个人授权</span>
                ) : (
                  <span className={pluginBadgeMuted}>无需授权</span>
                )}
              </td>
              <td className="py-2 text-[#66758b] [overflow-wrap:anywhere]">
                {tool.scopes.length > 0 ? tool.scopes.join(" ") : "—"}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

type Props = {
  client: WeKnoraClient;
  role: "viewer" | "admin" | "owner" | "system-admin";
};

export function PluginsSettingsPanel({ client, role }: Props) {
  // registry 条目 minRole=admin：viewer 不给提交面（rank 比较，owner 同样可预览）。
  const canEdit = roleAtLeast(role, "admin");
  const [manifestUrl, setManifestUrl] = useState("");
  const [preview, setPreview] = useState<PluginPreviewResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [confirming, setConfirming] = useState(false);
  const [confirmError, setConfirmError] = useState<string | null>(null);
  const [installations, setInstallations] = useState<readonly PluginInstallationSummary[]>([]);
  const [listError, setListError] = useState<string | null>(null);
  const [actionBusyId, setActionBusyId] = useState<string | null>(null);
  // T15（Issue #114）：升级差异预览是只读操作——状态独立于停用/启用，失败只
  // 落错误条，安装行与已接受版本零变化（服务端 PreviewUpgrade 不写任何安装）。
  const [upgradePreview, setUpgradePreview] = useState<UpgradePreviewState | null>(null);
  const [upgradeBusyId, setUpgradeBusyId] = useState<string | null>(null);
  const [upgradeError, setUpgradeError] = useState<{ pluginName: string; message: string } | null>(null);
  // T19（Issue #118）：工具策略治理面——一次展开一个安装（互斥），行内
  // enabled 与成员审批（require_approval）两开关，PUT 成功以服务端返回的
  // 刷新列表回填（服务端是策略权威，客户端不做乐观翻转）。
  const [toolPolicy, setToolPolicy] = useState<{
    installationId: string;
    pluginName: string;
    rows: readonly PluginToolPolicyRow[];
    error: string | null;
  } | null>(null);
  const [toolPolicyBusyId, setToolPolicyBusyId] = useState<string | null>(null);
  const [policyToggleBusy, setPolicyToggleBusy] = useState<string | null>(null);
  // 治理面失效代数（T19-OCR2-F1）：effect 失效（client 变化）时自增；在途
  // GET 的迟到回包携带发起时的代数，落地前比对——陈旧代数直接丢弃，旧
  // client 的策略行不再写回新视图（effect 的 cancelled 布尔同款陈旧性范式）。
  const toolPolicyEpoch = useRef(0);

  // useMemo 稳定 pluginsApi（T08-OCR1-F4）：client.request 是纯传输包装，
  // 稳定引用让 refreshInstallations 的 useCallback 依赖完整（exhaustive-deps）。
  const pluginsApi = useMemo(() => createPluginsApi((input) => client.request(input)), [client]);

  // isStale 供 effect 的 cancelled 清理模式使用：client 变化触发重跑时，
  // 旧请求的迟到响应/迟到失败不再覆盖新 client 的列表状态（对齐 PluginsPanel）。
  const refreshInstallations = React.useCallback(
    async (isStale?: () => boolean) => {
      try {
        const rows = await pluginsApi.listInstallations();
        if (isStale?.()) return;
        setInstallations(rows);
        setListError(null);
      } catch (cause) {
        if (isStale?.()) return;
        const message = apiErrorMessage(cause);
        if (message === null) {
          console.warn("plugin installations load failed:", cause);
          setListError("已安装插件加载失败，请稍后重试");
        } else {
          setListError(message);
        }
      }
    },
    [pluginsApi],
  );

  useEffect(() => {
    let cancelled = false;
    // client 变化（跨账号/工作区）触发的重载同步失效治理面（T19-OCR1-F4）：
    // 旧 client 抓取的策略行不可再展示，行内开关也不得用新 client 对旧
    // installationId 发 PUT。挂载首跑时 toolPolicy 本就是 null，此清空为
    // 无操作；toggleState 等函数直调 refreshInstallations 不经本 effect，
    // 不会误伤正常重载（安装停用/启用不改变策略行）。
    setToolPolicy(null);
    // 代数自增（T19-OCR2-F1）：已在途的 GET 的迟到回包凭旧代数被丢弃。
    toolPolicyEpoch.current += 1;
    void refreshInstallations(() => cancelled);
    return () => {
      cancelled = true;
    };
  }, [refreshInstallations]);

  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!canEdit || busy) return;
    const url = manifestUrl.trim();
    if (url === "") {
      setPreview(null);
      setError("请输入插件清单 URL");
      return;
    }
    setBusy(true);
    setError(null);
    setConfirmError(null);
    try {
      setPreview(await pluginsApi.previewInstallation(url));
    } catch (cause) {
      setPreview(null);
      // 错误分类（OCR low）：后端/网络经 request 抛 ApiError，message 是后端
      // 可读文案（SSRF 拒绝等）——沿用 McpSettingsPanel 原文透传先例；解析器
      // 自产的英文诊断串（含内部 API 路径）属客户端自身校验失败，不进中文
      // 管理界面——统一中文文案，原始错误 console.warn 留痕。
      const message = apiErrorMessage(cause);
      if (message === null) {
        console.warn("plugin preview failed:", cause);
        setError("插件清单预览失败：核验未通过");
      } else {
        setError(message);
      }
    } finally {
      setBusy(false);
    }
  }

  async function confirmInstall() {
    if (!preview || confirming || !canEdit) return;
    setConfirming(true);
    setConfirmError(null);
    try {
      await pluginsApi.confirmInstallation(preview.previewId);
      // 预览已消费：撤卡并刷新已安装列表（列表将出现该插件与已接受版本）。
      setPreview(null);
      await refreshInstallations();
    } catch (cause) {
      const message = apiErrorMessage(cause);
      if (message === null) {
        console.warn("plugin confirm failed:", cause);
        setConfirmError("确认安装失败，请重试");
      } else {
        setConfirmError(message);
      }
    } finally {
      setConfirming(false);
    }
  }

  async function toggleState(item: PluginInstallationSummary) {
    if (!canEdit || actionBusyId !== null) return;
    setActionBusyId(item.installationId);
    setListError(null);
    // T15-OCR1-F4(low)：成功的状态变更同时清掉升级预览错误条——两类横幅的
    // 清理行为保持一致，陈旧的升级失败提示不在数次无关操作后悬挂。
    setUpgradeError(null);
    try {
      await pluginsApi.setInstallationState(
        item.installationId,
        item.state === "active" ? "disabled" : "active",
      );
      await refreshInstallations();
    } catch (cause) {
      const message = apiErrorMessage(cause);
      if (message === null) {
        console.warn("plugin state change failed:", cause);
        setListError("插件状态变更失败，请重试");
      } else {
        setListError(message);
      }
    } finally {
      setActionBusyId(null);
    }  }

  async function checkUpgrade(item: PluginInstallationSummary) {
    if (!canEdit || upgradeBusyId !== null) return;
    setUpgradeBusyId(item.installationId);
    setUpgradeError(null);
    try {
      const result = await pluginsApi.previewUpgrade(item.installationId);
      // 只读预览成功即替换面板（重复点击 = 幂等重读，结果不累积）。
      setUpgradePreview({ pluginName: item.name, result });
    } catch (cause) {
      // 候选不可达/核验失败：错误条定位来源安装行，面板不残留，安装列表不动。
      // message 存纯文案（渲染层统一「升级预览失败（插件名）：」前缀，不再双拼）。
      setUpgradePreview(null);
      const message = apiErrorMessage(cause);
      if (message === null) {
        console.warn("plugin upgrade preview failed:", cause);
        setUpgradeError({ pluginName: item.name, message: "候选不可达或核验未通过，请稍后重试" });
      } else {
        setUpgradeError({ pluginName: item.name, message });
      }
    } finally {
      setUpgradeBusyId(null);
    }
  }

  // T19：展开/收起一个安装的工具治理面（GET .../tools——require_approval
  // 的权威确定值视图）。再次点击同一安装 = 收起；点其他安装 = 互斥切换。
  // PUT 在途（policyToggleBusy）期间整体冻结——杜绝「旧安装的迟到回包
  // （成功或失败）写进新展开面板」的错位窗口（T19-OCR1-F3）。
  async function toggleToolPolicyView(item: PluginInstallationSummary) {
    if (!canEdit || toolPolicyBusyId !== null || policyToggleBusy !== null) return;
    if (toolPolicy?.installationId === item.installationId) {
      setToolPolicy(null);
      return;
    }
    setToolPolicyBusyId(item.installationId);
    const epoch = toolPolicyEpoch.current;
    try {
      const rows = await pluginsApi.listInstallationTools(item.installationId);
      // 陈旧性校验（T19-OCR2-F1）：client 已切换（effect 失效过治理面）的
      // 迟到回包直接丢弃——旧 client 的策略行不写回新视图。
      if (epoch !== toolPolicyEpoch.current) return;
      setToolPolicy({ installationId: item.installationId, pluginName: item.name, rows, error: null });
    } catch (cause) {
      if (epoch !== toolPolicyEpoch.current) return;
      const message = apiErrorMessage(cause);
      if (message === null) {
        console.warn("plugin tool policy load failed:", cause);
        setToolPolicy({ installationId: item.installationId, pluginName: item.name, rows: [], error: "工具策略加载失败，请稍后重试" });
      } else {
        setToolPolicy({ installationId: item.installationId, pluginName: item.name, rows: [], error: message });
      }
    } finally {
      setToolPolicyBusyId(null);
    }
  }

  // T19：行内开关——每次 PUT 只带被切换的字段（省略键 = 服务端保持原值），
  // 成功后以返回的刷新列表整体回填。守卫同时冻结 toolPolicyBusyId
  // （T19-OCR2-F2）：另一安装的治理 GET 在途时面板即将翻转，此时发起的
  // PUT 迟到回包会被 installationId 守卫丢弃——失败被吞、成功被丢，用户
  // 对写操作零反馈；对称冻结直接闭合该窗口。
  async function toggleToolPolicyFlag(
    installationId: string,
    row: PluginToolPolicyRow,
    field: "enabled" | "requireApproval",
  ) {
    if (!canEdit || policyToggleBusy !== null || toolPolicyBusyId !== null) return;
    setPolicyToggleBusy(`${row.name}:${field}`);
    try {
      const rows = await pluginsApi.setInstallationToolPolicy(
        installationId,
        row.name,
        field === "enabled" ? { enabled: !row.enabled } : { requireApproval: !row.requireApproval },
      );
      setToolPolicy((prev) =>
        prev && prev.installationId === installationId ? { ...prev, rows, error: null } : prev);
    } catch (cause) {
      const message = apiErrorMessage(cause);
      // 与成功分支同款 installationId 匹配（T19-OCR1-F3）：迟到失败只落在
      // 发起安装的面板上——面板已被切换/失效时保持原状，不错位污染。
      if (message === null) {
        console.warn("plugin tool policy update failed:", cause);
        setToolPolicy((prev) =>
          prev && prev.installationId === installationId ? { ...prev, error: "工具策略更新失败，请重试" } : prev);
      } else {
        setToolPolicy((prev) =>
          prev && prev.installationId === installationId ? { ...prev, error: message } : prev);
      }
    } finally {
      setPolicyToggleBusy(null);
    }
  }

  return (
    <section className="grid gap-4" data-testid="plugins-settings">
      <div className="wk-mcp-page-header flex items-start justify-between gap-4 mb-3 max-[720px]:flex-col">
        <div>
          <h2 className="m-0 mb-2 text-[20px] font-semibold leading-[normal] text-[rgb(0_0_0_/_90%)]">插件管理</h2>
          <p className="wk-muted m-0 text-[14px] leading-[1.6] text-[rgb(0_0_0_/_60%)]">
            粘贴插件清单地址，核验并预览远端声明的插件版本与工具目录；确认后安装到本空间
          </p>
        </div>
      </div>
      {canEdit ? (
        <form className="grid gap-2" onSubmit={(event) => void submit(event)}>
          <label className="grid gap-1">
            <span className="text-[13px] font-medium">插件清单 URL</span>
            <Input
              type="url"
              value={manifestUrl}
              placeholder="https://plugins.example.com/jira-todo/manifest.json"
              onChange={(event) => setManifestUrl(event.target.value)}
            />
          </label>
          <div>
            <Button type="submit" variant="primary" loading={busy}>
              核验预览
            </Button>
          </div>
        </form>
      ) : (
        <Status>仅空间管理员可预览插件清单</Status>
      )}
      {error ? (
        <div data-testid="plugin-preview-error">
          <Status tone="error">预览失败：{error}</Status>
        </div>
      ) : null}
      {preview ? (
        <article
          data-testid="plugin-preview-card"
          className="min-w-0 overflow-hidden rounded-[10px] border border-[#dce3ed] bg-white"
        >
          <div className="flex flex-wrap items-center gap-2 border-b border-[#eef1f5] p-3">
            <h3 className="m-0 text-[15px] font-semibold leading-[21px]">{preview.name}</h3>
            <span className={pluginBadgeInfo}>{preview.version}</span>
            <span className={pluginBadgeMuted}>{preview.transportType === "http-streamable" ? "HTTP Streamable" : "SSE"}</span>
            <span className="ml-auto text-[12px] text-[#66758b]">预览有效期至 {formatPreviewExpiry(preview.expiresAt)}</span>
          </div>
          <div className="grid gap-1 p-3 text-[13px] leading-[20px]">
            <div className="flex flex-wrap gap-x-4 gap-y-1">
              <span className="text-[#66758b]">
                插件标识 <code className="text-[rgb(0_0_0_/_90%)]">{preview.pluginId}</code>
              </span>
              <span className="min-w-0 text-[#66758b] [overflow-wrap:anywhere]">
                版本端点 <code className="text-[rgb(0_0_0_/_90%)]">{preview.endpointUrl}</code>
              </span>
            </div>
            {preview.description ? <p className="m-0 text-[#66758b]">{preview.description}</p> : null}
            <p className="wk-muted m-0 text-[12px] leading-[18px] text-[#66758b]">
              工具 schema 由平台按清单声明与远端实际目录的 digest 比对核验通过后才生成此预览；界面不展示 schema 原文。
            </p>
          </div>
          <div className="overflow-x-auto p-3 pt-0">
            <table className="w-full border-collapse text-left text-[13px] leading-[20px]">
              <thead>
                <tr className="border-b border-[#eef1f5] text-[12px] text-[#66758b]">
                  <th className="py-1 pr-3 font-medium">工具</th>
                  <th className="py-1 pr-3 font-medium">说明</th>
                  <th className="py-1 pr-3 font-medium">分类</th>
                  <th className="py-1 pr-3 font-medium">授权</th>
                  <th className="py-1 pr-3 font-medium">Scopes</th>
                  <th className="py-1 font-medium">Schema</th>
                </tr>
              </thead>
              <tbody>
                {preview.tools.map((tool) => (
                  <tr key={tool.name} className="border-b border-[#f3f5f8] align-top last:border-b-0">
                    <td className="py-2 pr-3 font-medium [overflow-wrap:anywhere]">{tool.name}</td>
                    <td className="py-2 pr-3 text-[#66758b] [overflow-wrap:anywhere]">{tool.description}</td>
                    <td className="py-2 pr-3">
                      <span className={tool.readOnly ? pluginBadgeOk : pluginBadgeWarn}>{tool.readOnly ? "只读" : "写入"}</span>
                    </td>
                    <td className="py-2 pr-3">
                      {tool.requiresPersonalAuth ? (
                        <span className={pluginBadgeInfo}>需个人授权</span>
                      ) : (
                        <span className={pluginBadgeMuted}>无需授权</span>
                      )}
                    </td>
                    <td className="py-2 pr-3 text-[#66758b] [overflow-wrap:anywhere]">
                      {tool.scopes.length > 0 ? tool.scopes.join(" ") : "—"}
                    </td>
                    <td className="py-2">
                      <span className={pluginBadgeOk}>schema 指纹已核验</span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          {canEdit ? (
            <div className="flex flex-wrap items-center gap-3 border-t border-[#eef1f5] p-3">
              <Button type="button" variant="primary" loading={confirming} onClick={() => void confirmInstall()}>
                确认安装
              </Button>
              <span className="text-[12px] leading-[18px] text-[#66758b]">
                确认后为本空间固定安装该版本（写入工具默认停用）；预览随即失效
              </span>
              {confirmError ? (
                <span data-testid="plugin-confirm-error">
                  <Status tone="error">确认失败：{confirmError}</Status>
                </span>
              ) : null}
            </div>
          ) : null}
        </article>
      ) : null}
      <section aria-label="已安装插件" data-testid="plugin-installations" className="grid gap-2">
        <h3 className="m-0 text-[15px] font-semibold leading-[21px]">已安装插件</h3>
        <p className="wk-muted m-0 text-[12px] leading-[18px] text-[#66758b]">
          插件治理以停用为终点：停用后 Agent 不再调用该插件，成员连接与审计记录保留。
        </p>
        {/* 完整文案由各 catch 分支写入（T08-OCR1-F3：加载失败与状态变更失败
            各自表述，不再固定拼接「加载失败」前缀）；失败 ≠ 空——空态仅在
            无错误且无数据时呈现，失败时只留错误横幅（T08-OCR1-F7）。 */}
        {listError ? <Status tone="error">{listError}</Status> : null}
        {/* 升级预览错误条（T15）：定位来源安装行的只读失败——候选不可达/核验
            不符只报错，不触碰安装行与已接受版本（列表不变形）。 */}
        {upgradeError ? (
          <div data-testid="plugin-upgrade-error">
            <Status tone="error">升级预览失败（{upgradeError.pluginName}）：{upgradeError.message}</Status>
          </div>
        ) : null}
        {installations.length === 0 ? (
          listError === null ? <Status>暂无已安装插件</Status> : null
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
                  <span className={pluginBadgeOk}>启用中</span>
                ) : (
                  <span className={pluginBadgeMuted}>已停用</span>
                )}
                {item.driftState === "detected" ? (
                  <span className={pluginBadgeWarn}>检测到漂移</span>
                ) : (
                  <span className={pluginBadgeMuted}>无漂移</span>
                )}
                {item.requiresPersonalAuth ? <span className={pluginBadgeInfo}>需个人授权</span> : null}
                <span className="text-[#66758b]">{item.toolCount} 个工具</span>
                {canEdit ? (
                  <span className="ml-auto flex flex-wrap gap-2">
                    <Button
                      type="button"
                      loading={toolPolicyBusyId === item.installationId}
                      disabled={(toolPolicyBusyId !== null && toolPolicyBusyId !== item.installationId)
                        || policyToggleBusy !== null}
                      onClick={() => void toggleToolPolicyView(item)}
                    >
                      工具治理
                    </Button>
                    <Button
                      type="button"
                      loading={upgradeBusyId === item.installationId}
                      disabled={upgradeBusyId !== null && upgradeBusyId !== item.installationId}
                      onClick={() => void checkUpgrade(item)}
                    >
                      检查升级
                    </Button>
                    <Button
                      type="button"
                      loading={actionBusyId === item.installationId}
                      onClick={() => void toggleState(item)}
                    >
                      {item.state === "active" ? "停用" : "启用"}
                    </Button>
                  </span>
                ) : null}
                {/* T19：工具策略治理面——enabled 与成员审批（require_approval）
                    两开关同行；关闭原因随行呈现（写工具默认关闭的解释面）。 */}
                {toolPolicy?.installationId === item.installationId ? (
                  <div className="w-full" data-testid="plugin-tool-policy">
                    <h4 className="m-0 mb-1 text-[13px] font-medium">工具策略（{toolPolicy.pluginName}）</h4>
                    <p className="wk-muted m-0 mb-2 text-[12px] leading-[18px] text-[#66758b]">
                      成员审批开启后，每次调用前成员审阅确定的操作目标与参数原文，批准才派发且仅派发一次。
                    </p>
                    {toolPolicy.error ? <Status tone="error">{toolPolicy.error}</Status> : null}
                    {toolPolicy.rows.length === 0 && toolPolicy.error === null ? (
                      <Status>该安装的已接受快照内没有工具</Status>
                    ) : (
                      <div className="overflow-x-auto">
                        <table className="w-full border-collapse text-[12px] leading-[18px]">
                          <thead>
                            <tr className="text-left text-[#66758b]">
                              <th className="py-1 pr-3 font-medium">工具</th>
                              <th className="py-1 pr-3 font-medium">分类</th>
                              <th className="py-1 pr-3 font-medium">启用</th>
                              <th className="py-1 pr-3 font-medium">成员审批</th>
                              <th className="py-1 pr-3 font-medium">说明</th>
                            </tr>
                          </thead>
                          <tbody>
                            {toolPolicy.rows.map((row) => (
                              <tr key={row.name} className="border-t border-[#eef1f6] align-top">
                                <td className="py-1 pr-3 [overflow-wrap:anywhere]">
                                  <span className="font-medium">{row.name}</span>
                                  {row.description ? (
                                    <span className="block text-[#66758b]">{row.description}</span>
                                  ) : null}
                                </td>
                                <td className="py-1 pr-3">
                                  {row.readOnly ? (
                                    <span className={pluginBadgeInfo}>只读</span>
                                  ) : (
                                    <span className={pluginBadgeWarn}>写</span>
                                  )}
                                </td>
                                <td className="py-1 pr-3">
                                  <button
                                    type="button"
                                    role="switch"
                                    aria-checked={row.enabled}
                                    aria-label={`${row.name} 启用`}
                                    disabled={policyToggleBusy !== null || toolPolicyBusyId !== null}
                                    className={row.enabled ? "text-[#137333]" : "text-[#98a2b8]"}
                                    onClick={() => void toggleToolPolicyFlag(item.installationId, row, "enabled")}
                                  >
                                    {row.enabled ? "已启用" : "已停用"}
                                  </button>
                                </td>
                                <td className="py-1 pr-3">
                                  <button
                                    type="button"
                                    role="switch"
                                    aria-checked={row.requireApproval}
                                    aria-label={`${row.name} 成员审批`}
                                    disabled={policyToggleBusy !== null || toolPolicyBusyId !== null}
                                    className={row.requireApproval ? "text-[#137333]" : "text-[#98a2b8]"}
                                    onClick={() => void toggleToolPolicyFlag(item.installationId, row, "requireApproval")}
                                  >
                                    {row.requireApproval ? "需审批" : "免审批"}
                                  </button>
                                </td>
                                <td className="py-1 pr-3 text-[#66758b] [overflow-wrap:anywhere]">
                                  {row.disabledReason}
                                </td>
                              </tr>
                            ))}
                          </tbody>
                        </table>
                      </div>
                    )}
                  </div>
                ) : null}
              </li>
            ))}
          </ul>
        )}
      </section>
      {/* T15（Issue #114）：五维差异预览面板——只读，不提供接受入口（接受升级
          属后续切片）；渲染版本对（含降级标注）、端点变化行、新增/移除/变更
          工具表与四个独立变更理由徽标（schema/scope/读写分类/授权面）。 */}
      {upgradePreview ? (
        <article
          data-testid="plugin-upgrade-preview"
          className="min-w-0 overflow-hidden rounded-[10px] border border-[#dce3ed] bg-white"
        >
          <div className="flex flex-wrap items-center gap-2 border-b border-[#eef1f5] p-3">
            <h3 className="m-0 text-[15px] font-semibold leading-[21px]">升级差异预览 · {upgradePreview.pluginName}</h3>
            {upgradePreview.result.diff.isDowngrade ? (
              <span className={pluginBadgeWarn}>降级</span>
            ) : (
              <span className={pluginBadgeInfo}>升级</span>
            )}
            <span className="ml-auto flex flex-wrap items-center gap-2 text-[13px]">
              <code className="text-[rgb(0_0_0_/_90%)]">{upgradePreview.result.diff.currentVersion}</code>
              <span className="text-[#66758b]">→</span>
              <code className="text-[rgb(0_0_0_/_90%)]">{upgradePreview.result.diff.candidateVersion}</code>
              <Button type="button" onClick={() => setUpgradePreview(null)}>
                关闭
              </Button>
            </span>
          </div>
          <div className="grid gap-2 p-3 text-[13px] leading-[20px]">
            <div className="flex flex-wrap items-center gap-2">
              {upgradePreview.result.diff.endpointChanged ? (
                <span className={pluginBadgeWarn}>端点已变化</span>
              ) : (
                <span className={pluginBadgeMuted}>端点未变化</span>
              )}
              <code className="min-w-0 [overflow-wrap:anywhere] text-[#66758b]">{upgradePreview.result.diff.currentEndpoint}</code>
              {upgradePreview.result.diff.endpointChanged ? (
                <>
                  <span className="text-[#66758b]">→</span>
                  <code className="min-w-0 [overflow-wrap:anywhere] text-[rgb(0_0_0_/_90%)]">{upgradePreview.result.diff.candidateEndpoint}</code>
                </>
              ) : null}
            </div>
          </div>
          <div className="grid gap-3 border-t border-[#eef1f5] p-3">
            <DiffToolTable caption="新增工具" rows={upgradePreview.result.diff.addedTools} />
            <DiffToolTable caption="移除工具" rows={upgradePreview.result.diff.removedTools} />
            <div className="overflow-x-auto">
              <h4 className="m-0 mb-1 text-[13px] font-medium">变更工具（{upgradePreview.result.diff.changedTools.length}）</h4>
              <table className="w-full border-collapse text-left text-[13px] leading-[20px]">
                <thead>
                  <tr className="border-b border-[#eef1f5] text-[12px] text-[#66758b]">
                    <th className="py-1 pr-3 font-medium">工具</th>
                    <th className="py-1 pr-3 font-medium">变更理由</th>
                    <th className="py-1 pr-3 font-medium">现版</th>
                    <th className="py-1 font-medium">候选版</th>
                  </tr>
                </thead>
                <tbody>
                  {upgradePreview.result.diff.changedTools.map((change) => (
                    <tr key={change.name} className="border-b border-[#f3f5f8] align-top last:border-b-0">
                      <td className="py-2 pr-3 font-medium [overflow-wrap:anywhere]">{change.name}</td>
                      <td className="flex flex-wrap gap-1 py-2 pr-3">{changeReasonBadges(change)}</td>
                      <td className="py-2 pr-3 text-[#66758b] [overflow-wrap:anywhere]">{snapshotSummary(change.current)}</td>
                      <td className="py-2 text-[#66758b] [overflow-wrap:anywhere]">{snapshotSummary(change.candidate)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <div className="grid gap-1 text-[12px] leading-[18px] text-[#66758b]">
              <span className="min-w-0 [overflow-wrap:anywhere]">
                候选身份指纹 <code className="text-[rgb(0_0_0_/_90%)]">{upgradePreview.result.candidateFingerprint}</code>
              </span>
              <span className="min-w-0 [overflow-wrap:anywhere]">
                候选工具目录摘要 <code className="text-[rgb(0_0_0_/_90%)]">{upgradePreview.result.candidateToolsDigest}</code>
              </span>
            </div>
            <p className="wk-muted m-0 text-[12px] leading-[18px] text-[#66758b]">
              预览为只读操作：在管理员另行确认接受前，本空间继续使用当前已接受版本；候选端点不可达或清单核验不符时预览直接失败，不影响已安装插件。
            </p>
          </div>
        </article>
      ) : null}
    </section>
  );
}
