import * as React from "react";
import { useEffect, useMemo, useState } from "react";
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
                  <span className="ml-auto">
                    <Button
                      type="button"
                      loading={actionBusyId === item.installationId}
                      onClick={() => void toggleState(item)}
                    >
                      {item.state === "active" ? "停用" : "启用"}
                    </Button>
                  </span>
                ) : null}
              </li>
            ))}
          </ul>
        )}
      </section>
    </section>
  );
}
