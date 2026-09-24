import * as React from "react";
import { useState } from "react";
import type { ClientRequest, WeKnoraClient } from "@weknora/api-client";
import { Button, Input, Status } from "@weknora/ui";
import { roleAtLeast } from "@weknora/views/settings/registry";

/**
 * 管理端插件清单预览面板（Issue #108 / 计划 T03）。
 *
 * 空间管理员粘贴插件清单 URL → POST /api/v1/plugins/installations/preview
 * → 展示平台核验后的预览卡（插件名/版本/端点/工具表/读写与需授权徽标/
 * 有效期）与错误态。本面板不含“确认安装”按钮（T08 加入）。
 *
 * schema 展示取舍（Spec US9，计划 Task 3 Step 7 明示）：平台核验、界面
 * 不渲染原文——后端仅在清单声明的 input_schema_digest 与远端实际目录的
 * digest 完全一致时才产生此预览（不一致整卡拒绝），因此界面只展示工具
 * 元数据与“schema 指纹已核验”徽标，远端 schema JSON 原文不进管理界面。
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

const PREVIEW_PATH = "/api/v1/plugins/installations/preview";

type RecordValue = Record<string, unknown>;

/** 面板层预览视图（与 api-client PluginPreviewResult 同构）。 */
export interface PluginPreviewView {
  readonly previewId: string;
  readonly pluginId: string;
  readonly version: string;
  readonly name: string;
  readonly description: string;
  readonly transportType: string;
  readonly endpointUrl: string;
  readonly tools: ReadonlyArray<{
    readonly name: string;
    readonly description: string;
    readonly readOnly: boolean;
    readonly requiresPersonalAuth: boolean;
    readonly scopes: readonly string[];
  }>;
  readonly identityFingerprint: string;
  readonly expiresAt: string;
}

function record(value: unknown, path: string): RecordValue {
  if (value === null || typeof value !== "object" || Array.isArray(value)) throw new Error(`${path} must be an object`);
  return value as RecordValue;
}

function required(value: unknown, path: string): string {
  if (typeof value !== "string" || value.trim() === "") throw new Error(`${path} must be a non-empty string`);
  return value;
}

function text(value: unknown, path: string): string {
  if (typeof value !== "string") throw new Error(`${path} must be a string`);
  return value;
}

function flag(value: unknown, path: string): boolean {
  if (typeof value !== "boolean") throw new Error(`${path} must be a boolean`);
  return value;
}

function scopes(value: unknown, path: string): string[] {
  // Go nil slice 序列化为 null：声明无 scope 的工具合法。
  if (value === null || value === undefined) return [];
  if (!Array.isArray(value) || value.some((item) => typeof item !== "string")) throw new Error(`${path} must be a string array`);
  return value as string[];
}

/**
 * 面板层严格 envelope 解析器——镜像 packages/api-client/src/plugins.ts 的
 * parsePluginPreview（该导出尚未挂进 @weknora/api-client 入口清单，T08
 * 挂载 client.plugins 后收敛为单一来源）。拒绝非 success envelope 与任何
 * 缺失/畸形字段，远端不可信数据不直接进入渲染树。
 */
export function parsePluginPreviewEnvelope(value: unknown): PluginPreviewView {
  const envelope = record(value, PREVIEW_PATH);
  if (envelope.success !== true) throw new Error(`${PREVIEW_PATH}.success must be true`);
  const data = record(envelope.data, `${PREVIEW_PATH}.data`);
  const transportType = required(data.transport_type, `${PREVIEW_PATH}.data.transport_type`);
  if (transportType !== "http-streamable" && transportType !== "sse") throw new Error(`${PREVIEW_PATH}.data.transport_type is invalid`);
  const toolRows = data.tools;
  if (!Array.isArray(toolRows)) throw new Error(`${PREVIEW_PATH}.data.tools must be an array`);
  const tools = toolRows.map((item, index) => {
    const row = record(item, `${PREVIEW_PATH}.data.tools[${index}]`);
    return {
      name: required(row.name, `${PREVIEW_PATH}.data.tools[${index}].name`),
      description: text(row.description, `${PREVIEW_PATH}.data.tools[${index}].description`),
      readOnly: flag(row.read_only, `${PREVIEW_PATH}.data.tools[${index}].read_only`),
      requiresPersonalAuth: flag(row.requires_personal_auth, `${PREVIEW_PATH}.data.tools[${index}].requires_personal_auth`),
      scopes: scopes(row.scopes, `${PREVIEW_PATH}.data.tools[${index}].scopes`),
    };
  });
  return {
    previewId: required(data.preview_id, `${PREVIEW_PATH}.data.preview_id`),
    pluginId: required(data.plugin_id, `${PREVIEW_PATH}.data.plugin_id`),
    version: required(data.version, `${PREVIEW_PATH}.data.version`),
    name: required(data.name, `${PREVIEW_PATH}.data.name`),
    description: text(data.description, `${PREVIEW_PATH}.data.description`),
    transportType,
    endpointUrl: required(data.endpoint_url, `${PREVIEW_PATH}.data.endpoint_url`),
    tools,
    identityFingerprint: required(data.identity_fingerprint, `${PREVIEW_PATH}.data.identity_fingerprint`),
    expiresAt: required(data.expires_at, `${PREVIEW_PATH}.data.expires_at`),
  };
}

/** 构造预览 POST 请求（纯函数，供测试与组件共用）。 */
export function pluginPreviewRequest(manifestUrl: string): ClientRequest {
  const url = manifestUrl.trim();
  if (url === "") throw new Error("manifestUrl must not be empty");
  return { method: "POST", path: PREVIEW_PATH, body: { manifest_url: url } };
}

/** 预览有效期本地化；非法时间串原样回显（不猜测远端数据语义）。 */
export function formatPreviewExpiry(expiresAt: string, locale = "zh-CN"): string {
  const date = new Date(expiresAt);
  return Number.isNaN(date.getTime()) ? expiresAt : date.toLocaleString(locale);
}

type Props = {
  client: WeKnoraClient;
  role: "viewer" | "admin" | "owner" | "system-admin";
};

export function PluginsSettingsPanel({ client, role }: Props) {
  // registry 条目 minRole=admin：viewer 不给提交面（rank 比较，owner 同样可预览）。
  const canEdit = roleAtLeast(role, "admin");
  const [manifestUrl, setManifestUrl] = useState("");
  const [preview, setPreview] = useState<PluginPreviewView | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

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
    try {
      const envelope = await client.request(pluginPreviewRequest(url));
      setPreview(parsePluginPreviewEnvelope(envelope));
    } catch (cause) {
      setPreview(null);
      setError(cause instanceof Error ? cause.message : "插件清单预览失败");
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="grid gap-4" data-testid="plugins-settings">
      <div className="wk-mcp-page-header flex items-start justify-between gap-4 mb-3 max-[720px]:flex-col">
        <div>
          <h2 className="m-0 mb-2 text-[20px] font-semibold leading-[normal] text-[rgb(0_0_0_/_90%)]">插件管理</h2>
          <p className="wk-muted m-0 text-[14px] leading-[1.6] text-[rgb(0_0_0_/_60%)]">
            粘贴插件清单地址，核验并预览远端声明的插件版本与工具目录
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
        </article>
      ) : null}
    </section>
  );
}
