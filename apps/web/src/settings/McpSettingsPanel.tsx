import { useEffect, useRef, useState } from "react";
import * as React from "react";
import type { McpConfiguration, WeKnoraClient } from "@weknora/api-client";
// S6 抽屉收编：MCP 编辑面离开 packages/ui 旧栈 表单栈（T15 硬前置）。
import { Button as TButton, Checkbox as TCheckbox, Input as TInput, Select as TSelect, Textarea as TTextarea } from "tdesign-react";
import { WkCard as Card, WkStatus as Status } from "../shared/wk-legacy.tsx";
import { Icon as TIcon } from "tdesign-icons-react";
import { McpToolsDirectory } from "./McpToolsDirectory.tsx";
import { EmptyState } from "./EmptyState.tsx";
import { pushSettingsToast } from "./settings-toast.tsx";
import { roleAtLeast } from "@weknora/views/settings/registry";
import { createTranslator, useAppLocale } from "../i18n.ts";

/* S6 Tailwind 收编：原 .wk-mcp-* utility 串（migrated 1:1 from the deleted
   styles.css rules）→ settings-wrapper.css 的 scoped 规则；取值 = utilities
   字面量。落在 tdesign 组件 className 上的 utility（TButton h-7/px-3 等）
   在 v4 utilities layer 中本就敌不过 unlayered 的 .t-button* 规则——为保
   现渲染不移植，仅保留语义 hook 类。 */

function McpCardIcon({ name, size = 14 }: { name: "tools" | "edit" | "delete" | "add" | "chevron-right" | "error"; size?: number }) {
  const paths = {
    tools: <><path d="M14.7 6.3a4.5 4.5 0 0 0 6 6l-7.4 7.4a2.1 2.1 0 0 1-3-3z" /><path d="M14.7 6.3l3-3 3 3-3 3" /></>,
    edit: <><path d="M12 20h9" /><path d="M16.5 3.5a2.12 2.12 0 0 1 3 3L8 18l-4 1 1-4z" /></>,
    delete: <><path d="M3 6h18M8 6V4h8v2m-9 0 1 15h8l1-15" /><path d="M10 11v6M14 11v6" /></>,
    add: <><path d="M12 5v14M5 12h14" /></>,
    "chevron-right": <path d="m9 18 6-6-6-6" />,
    error: <><circle cx="12" cy="12" r="9" /><path d="M12 8v5M12 16h.01" /></>,
  }[name];
  return <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">{paths}</svg>;
}

function McpTransportIcon({ transport }: { transport: "sse" | "http-streamable" }) {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      {transport === "sse" ? <><path d="M5 12h14" /><path d="M8 8c2.5-2.5 5.5-2.5 8 0" /><path d="M8 16c2.5 2.5 5.5 2.5 8 0" /></> : <><path d="M5 12h14" /><path d="m13 6 6 6-6 6" /><path d="M5 6h3" /><path d="M5 18h3" /></>}
    </svg>
  );
}

const MCP_DRAWER_SPEC = {
  storageKey: "setting-drawer:width:mcp-config-v2",
  defaultWidth: 680,
  minWidth: 560,
  maxWidth: 920,
} as const;

export function clampMcpDrawerWidth(width: number, viewportWidth = typeof window === "undefined" ? MCP_DRAWER_SPEC.maxWidth : window.innerWidth): number {
  const viewport = viewportWidth;
  const cap = Math.min(MCP_DRAWER_SPEC.maxWidth, viewport);
  const floor = Math.min(MCP_DRAWER_SPEC.minWidth, cap);
  return Math.max(floor, Math.min(cap, Math.round(width)));
}

function readMcpDrawerWidth(): number {
  try {
    const raw = typeof window === "undefined" ? null : window.localStorage.getItem(MCP_DRAWER_SPEC.storageKey);
    const parsed = raw ? Number(raw) : Number.NaN;
    return Number.isFinite(parsed) ? clampMcpDrawerWidth(parsed) : MCP_DRAWER_SPEC.defaultWidth;
  } catch {
    return MCP_DRAWER_SPEC.defaultWidth;
  }
}

function McpDrawerShell({ children }: { children: React.ReactElement<{ children?: React.ReactNode }> }) {
  const [width, setWidth] = useState<number>(MCP_DRAWER_SPEC.defaultWidth);
  const [resizing, setResizing] = useState(false);
  const widthRef = useRef(width);
  widthRef.current = width;

  useEffect(() => {
    setWidth(readMcpDrawerWidth());
    const onWindowResize = () => setWidth((current) => clampMcpDrawerWidth(current));
    window.addEventListener("resize", onWindowResize);
    return () => window.removeEventListener("resize", onWindowResize);
  }, []);

  function onHandleDown(event: React.MouseEvent<HTMLDivElement>) {
    event.preventDefault();
    const startX = event.clientX;
    const startWidth = widthRef.current;
    setResizing(true);
    document.body.style.cursor = "col-resize";
    document.body.style.userSelect = "none";
    const onMove = (move: MouseEvent) => setWidth(clampMcpDrawerWidth(startWidth + startX - move.clientX));
    const onUp = () => {
      document.removeEventListener("mousemove", onMove);
      document.removeEventListener("mouseup", onUp);
      document.body.style.cursor = "";
      document.body.style.userSelect = "";
      setResizing(false);
      try {
        window.localStorage.setItem(MCP_DRAWER_SPEC.storageKey, String(clampMcpDrawerWidth(widthRef.current)));
      } catch {
        // localStorage is optional in private mode and restricted embeds.
      }
    };
    document.addEventListener("mousemove", onMove);
    document.addEventListener("mouseup", onUp);
  }

  const handle = (
    <div
      className={'wk-mcp-resize-handle' + (resizing ? " is-active" : "")}
      role="separator"
      aria-orientation="vertical"
      aria-label="调整抽屉宽度"
      onMouseDown={onHandleDown}
    >
      <div className={'wk-mcp-resize-bar' + (resizing ? ' is-active' : '')} />
    </div>
  );
  const content = React.isValidElement(children)
    ? React.cloneElement(children, {}, children.props.children, handle)
    : children;
  return (
    <div
      className="wk-settings-context-contents"
      data-resizing={resizing || undefined}
      style={{ "--mcp-drawer-width": `${width}px` } as React.CSSProperties}
    >
      {content}
    </div>
  );
}

type McpService = McpConfiguration & {
  description?: string;
  usage_instructions?: string;
  is_builtin?: boolean;
  headers?: Record<string, string>;
  advanced_config?: {
    timeout?: number;
    retry_count?: number;
    retry_delay?: number;
  };
  auth_config?: {
    auth_type?: string;
    api_key_header?: string;
    scopes?: string[];
    [key: string]: unknown;
  };
  stdio_config?: Record<string, unknown>;
  credentials?: { api_key?: { configured?: boolean }; token?: { configured?: boolean } };
  catalog?: { tool_count?: number; stale?: boolean };
};
type Props = {
  client: WeKnoraClient;
  role: "viewer" | "admin" | "owner" | "system-admin";
  initialServices?: readonly McpConfiguration[];
};
type Draft = {
  id?: string;
  name: string;
  description: string;
  usageInstructions: string;
  url: string;
  transportType: "sse" | "http-streamable" | "stdio";
  enabled: boolean;
  authType: "" | "api_key" | "oauth";
  apiKeyHeader: string;
  apiKey: string;
  oauthScopes: string;
  headers: Array<{ key: string; value: string }>;
  timeout: number | "";
  retryCount: number | "";
  retryDelay: number | "";
  codeImport: string;
  codeImportError: string;
  authConfig: Record<string, unknown>;
  credentialConfigured?: boolean;
};

export type McpDraftValidationError =
  | "nameRequired"
  | "urlRequired"
  | "urlInvalid"
  | "stdioUnsupported"
  | "usageRequired";

/** Mirrors the Vue form rules before any network mutation is attempted. */
export function validateMcpDraft(
  draft: Draft,
  step: 0 | 1,
): McpDraftValidationError | null {
  if (!draft.name.trim()) return "nameRequired";
  if (step === 1 && !draft.usageInstructions.trim()) return "usageRequired";
  if (draft.transportType === "stdio") return "stdioUnsupported";
  if (!draft.url.trim()) return "urlRequired";
  try {
    const url = new URL(draft.url.trim());
    if (!["http:", "https:"].includes(url.protocol)) return "urlInvalid";
  } catch {
    return "urlInvalid";
  }
  return null;
}

function asService(value: McpConfiguration): McpService {
  return value as McpService;
}
function serviceDescription(service: McpService): string {
  return (
    service.usage_instructions?.trim() || service.description?.trim() || ""
  );
}
function draftFrom(service?: McpService): Draft {
  const authConfig = service?.auth_config ? { ...service.auth_config } : {};
  const authType =
    authConfig.auth_type === "oauth"
      ? "oauth"
      : authConfig.auth_type === "api_key" || authConfig.auth_type === "bearer"
        ? "api_key"
        : "";
  if (authConfig.auth_type === "bearer" && !authConfig.api_key_header)
    authConfig.api_key_header = "Authorization";
  return {
    ...(service?.id ? { id: service.id } : {}),
    name: service?.name ?? "",
    description: service?.description ?? "",
    usageInstructions: service?.usage_instructions ?? "",
    url: service?.url ?? "",
    // Vue McpServiceDialog.vue:855 — a stdio service is coerced to SSE when
    // loaded into the remote editor; the drawer never offers a stdio option.
    transportType:
      service?.transport_type === "http-streamable" ? "http-streamable" : "sse",
    enabled: service?.enabled !== false,
    authType,
    apiKeyHeader:
      typeof authConfig.api_key_header === "string"
        ? authConfig.api_key_header
        : "",
    apiKey: "",
    credentialConfigured: service?.credentials?.api_key?.configured === true || service?.credentials?.token?.configured === true,
    oauthScopes: Array.isArray(authConfig.scopes)
      ? authConfig.scopes.join(" ")
      : "",
    headers: Object.entries(service?.headers ?? {}).map(([key, value]) => ({
      key,
      value: String(value),
    })),
    timeout: service?.advanced_config?.timeout ?? 30,
    retryCount: service?.advanced_config?.retry_count ?? 3,
    retryDelay: service?.advanced_config?.retry_delay ?? 1,
    codeImport: "",
    codeImportError: "",
    authConfig,
  };
}

export function importMcpConfig(raw: string, current: Draft): Draft {
  let parsed: unknown;
  try {
    parsed = JSON.parse(raw);
  } catch {
    return { ...current, codeImportError: "Invalid JSON." };
  }
  if (!parsed || typeof parsed !== "object")
    return {
      ...current,
      codeImportError: "No MCP server configuration found.",
    };
  const object = parsed as Record<string, unknown>;
  const servers =
    object.mcpServers && typeof object.mcpServers === "object"
      ? (object.mcpServers as Record<string, unknown>)
      : object;
  const entry = Object.entries(servers).find(
    ([, value]) => value && typeof value === "object",
  ) as [string, Record<string, unknown>] | undefined;
  if (!entry)
    return {
      ...current,
      codeImportError: "No MCP server configuration found.",
    };
  const [name, config] = entry;
  if (config.command)
    return {
      ...current,
      codeImportError:
        "stdio MCP configurations are not supported by this remote editor.",
    };
  const headers =
    config.headers && typeof config.headers === "object"
      ? Object.entries(config.headers as Record<string, unknown>)
      : [];
  let apiKey = "";
  let apiKeyHeader = "";
  const customHeaders: Array<{ key: string; value: string }> = [];
  for (const [key, value] of headers) {
    const text = String(value ?? "");
    if (
      ["authorization", "x-api-key", "api-key", "apikey"].includes(
        key.toLowerCase(),
      )
    ) {
      apiKey = text;
      apiKeyHeader = key.toLowerCase() === "x-api-key" ? "" : key;
    } else customHeaders.push({ key, value: text });
  }
  const url = typeof config.url === "string" ? config.url.trim() : "";
  if (!url)
    return {
      ...current,
      codeImportError: "The imported MCP server has no URL.",
    };
  const transportType =
    String(config.type ?? config.transport ?? "")
      .toLowerCase()
      .includes("sse") || /\/sse\/?($|\?)/i.test(url)
      ? "sse"
      : "http-streamable";
  return {
    ...current,
    name: name || current.name,
    url,
    transportType,
    headers: customHeaders,
    authType: apiKey ? "api_key" : current.authType,
    apiKeyHeader,
    apiKey,
    codeImport: raw,
    codeImportError: "",
  };
}

type MetadataSnapshot = Awaited<
  ReturnType<WeKnoraClient["configuration"]["mcp"]["metadata"]["get"]>
>;
type ToolApprovalRow = Awaited<
  ReturnType<WeKnoraClient["configuration"]["mcp"]["toolApprovals"]["list"]>
>[number];

/**
 * Step-2 tools/metadata panel — port of Vue McpMetadataPanel.vue. The Vue
 * baseline mounts it only inside the drawer step 2; step 0 never loads tools.
 * The Vue test-connection surface (testMCPService + McpTestResultBody) is
 * orphan code with no reachable UI in the baseline, so the React port must
 * not expose a test-connection button either.
 */
function McpMetadataSection({
  client,
  serviceId,
  disabled,
  onSyncedChange,
  onBusyChange,
}: {
  client: WeKnoraClient;
  serviceId: string;
  disabled: boolean;
  onSyncedChange: (synced: boolean) => void;
  onBusyChange: (busy: boolean) => void;
}) {
  const t = createTranslator(useAppLocale());
  const [metadata, setMetadata] = useState<MetadataSnapshot | null>(null);
  const [approvals, setApprovals] = useState<ToolApprovalRow[]>([]);
  const [policyError, setPolicyError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [policyBusy, setPolicyBusy] = useState(false);
  const [busyTools, setBusyTools] = useState<Set<string>>(new Set());
  const busyToolsRef = useRef(new Set<string>());
  const [error, setError] = useState<string | null>(null);
  const [docsOpen, setDocsOpen] = useState(false);
  const loadGeneration = useRef(0);
  async function load(refresh = false) {
    const generation = ++loadGeneration.current;
    setBusy(true);
    setPolicyBusy(!refresh);
    onBusyChange(true);
    setError(null);
    setPolicyError(null);
    const metadataRequest = refresh
      ? () => client.configuration.mcp.metadata.refresh(serviceId)
      : async () => {
          // Vue McpMetadataPanel.vue treats a missing cache as an initial sync
          // condition, not as the final "not synced" state.
          const saved = await client.configuration.mcp.metadata.get(serviceId);
          return saved ?? client.configuration.mcp.metadata.refresh(serviceId);
        };
    // Vue renders the metadata snapshot as soon as it resolves; the nested
    // McpToolsList loads policy rows independently and disables only its
    // switches while that request is pending. Do not make a slow policy
    // request hide an already available tool directory.
    const metadataPromise = metadataRequest()
      .then((value) => {
        if (generation === loadGeneration.current) setMetadata(value);
      })
      .catch((cause) => {
        if (generation === loadGeneration.current)
          setError(
            cause instanceof Error ? cause.message : t("mcpMetadata.failed"),
          );
      });
    const approvalsPromise = refresh
      ? Promise.resolve()
      : client.configuration.mcp.toolApprovals.list(serviceId)
        .then((rows) => {
          if (generation === loadGeneration.current) setApprovals(rows);
        })
        .catch(() => {
          if (generation === loadGeneration.current) {
            setApprovals([]);
            setPolicyError(t("mcpMetadata.policyLoadFailed"));
          }
        });
    await Promise.all([metadataPromise, approvalsPromise]);
    if (generation === loadGeneration.current) {
      setBusy(false);
      setPolicyBusy(false);
      onBusyChange(false);
    }
  }
  useEffect(() => {
    void load();
    return () => {
      loadGeneration.current += 1;
      onSyncedChange(false);
      onBusyChange(false);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [client, serviceId]);
  useEffect(() => {
    onSyncedChange(Boolean(metadata && !metadata.stale));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [metadata]);
  useEffect(() => { setDocsOpen(false); }, [metadata?.serviceId]);
  async function updateTool(
    toolName: string,
    field: "enabled" | "requireApproval",
    value: boolean,
  ) {
    if (busy || busyToolsRef.current.has(toolName) || metadata?.stale || policyError) return;
    busyToolsRef.current.add(toolName);
    setBusyTools(new Set(busyToolsRef.current));
    onBusyChange(true);
    setError(null);
    try {
      await client.configuration.mcp.toolApprovals.update(serviceId, toolName, {
        [field]: value,
      });
      setApprovals((current) => {
        const existing = current.find((row) => row.toolName === toolName);
        if (existing)
          return current.map((row) =>
            row.toolName === toolName ? { ...row, [field]: value } : row,
          );
        return [
          ...current,
          {
            id: serviceId + ":" + toolName,
            serviceId,
            toolName,
            enabled: field === "enabled" ? value : true,
            requireApproval: field === "requireApproval" ? value : false,
          },
        ];
      });
    } catch (cause) {
      setError(
        cause instanceof Error ? cause.message : t("mcpMetadata.policySaveFailed"),
      );
    } finally {
      busyToolsRef.current.delete(toolName);
      setBusyTools(new Set(busyToolsRef.current));
      onBusyChange(busyToolsRef.current.size > 0);
    }
  }
  let syncedAt = "";
  if (metadata?.syncedAt) {
    const date = new Date(metadata.syncedAt as unknown as string);
    syncedAt = Number.isNaN(date.getTime())
      ? String(metadata.syncedAt)
      : date.toLocaleString();
  }
  return (
    <section className="wk-mcp-metadata" aria-label={t("mcpMetadata.tools")}>
      <div className="wk-settings-panel-heading wk-mcp-sticky-head">
        <div>
          <h4>{t("mcpMetadata.tools")}</h4>
          <p className="wk-muted wk-mcp-head-desc">{t("mcpMetadata.cacheHint")}</p>
        </div>
        <div className="wk-list-actions">
          <TButton
            type="button"
            disabled={busy || disabled}
            onClick={() => void load(true)}
          >
            {metadata ? t("mcpMetadata.refresh") : t("mcpMetadata.fetch")}
          </TButton>
        </div>
      </div>
      {error ? <Status tone="error">{error}</Status> : null}
      {metadata ? (
        <>
          <div className={'wk-mcp-sync-line' + (metadata.stale ? ' is-stale' : '')}>
            {t("mcpMetadata.toolCount", { count: metadata.tools.length })}
            {metadata.serverName
              ? " · " + metadata.serverName + " " + (metadata.serverVersion ?? "")
              : ""}
            {syncedAt ? " · " + t("mcpMetadata.syncedAt") + syncedAt : ""}
            {metadata.stale ? " · " + t("mcpMetadata.stale") : ""}
            {metadata.instructions || metadata.serverDescription ? <><button type="button" className={'wk-mcp-server-docs-trigger wk-mcp-docs-btn'} aria-expanded={docsOpen} onClick={() => setDocsOpen((open) => !open)}>{t("mcpMetadata.serverDocumentation")}⌄</button>{docsOpen ? <div className="wk-mcp-docs-popover" role="dialog"><strong>{t("mcpMetadata.serverDocumentation")}</strong>{metadata.serverDescription ? <span>{metadata.serverDescription}</span> : null}{metadata.instructions ? <pre>{metadata.instructions}</pre> : null}</div> : null}</> : <button type="button" className="wk-mcp-docs-btn" aria-label={t("mcpMetadata.noServerDocumentation")}>ⓘ</button>}
          </div>
          <p className="wk-muted">{t("mcpMetadata.policyHint")}</p>
          <McpToolsDirectory
            tools={metadata.tools}
            serviceId={metadata.stale ? undefined : serviceId}
            approvals={approvals}
            busy={policyBusy || metadata.stale}
            busyTools={busyTools}
            policyError={policyError}
            onRetryPolicies={() => void load()}
            onPolicyChange={(name, field, value) =>
              void updateTool(name, field, value)
            }
          />
        </>
      ) : (
        <Status>
          {busy ? t("mcpMetadata.fetching") : t("mcpMetadata.notSynced")}
        </Status>
      )}
    </section>
  );
}

type OauthStatus = Awaited<
  ReturnType<WeKnoraClient["configuration"]["mcp"]["oauth"]["status"]>
>;

/**
 * OAuth authorization block — port of the Vue auth-config section
 * (McpServiceDialog.vue:201-241). Lives in step 0 next to the scopes field,
 * not in the tools step. Authorize always persists the connection first,
 * exactly like Vue handleAuthorize().
 */
function McpOAuthControl({
  client,
  serviceId,
  busy,
  onRequestAuthorize,
}: {
  client: WeKnoraClient;
  serviceId: string;
  busy: boolean;
  onRequestAuthorize: () => Promise<void>;
}) {
  const t = createTranslator(useAppLocale());
  const [oauth, setOauth] = useState<OauthStatus | null>(null);
  const [authorizing, setAuthorizing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const generation = useRef(0);
  useEffect(() => {
    const current = ++generation.current;
    client.configuration.mcp
      .oauth.status(serviceId)
      .then((status) => {
        if (current === generation.current) setOauth(status);
      })
      .catch(() => {
        /* Vue logs and keeps the reauth_required default */
      });
    return () => {
      generation.current += 1;
    };
  }, [client, serviceId]);
  async function revoke() {
    if (busy || authorizing) return;
    setAuthorizing(true);
    setError(null);
    try {
      await client.configuration.mcp.oauth.revoke(serviceId);
      setOauth({ authorized: false, state: "reauth_required", refreshAvailable: false });
    } catch (cause) {
      setError(
        cause instanceof Error ? cause.message : t("mcpSettings.toasts.updateFailed"),
      );
    } finally {
      setAuthorizing(false);
    }
  }
  return (
    <div className="wk-mcp-oauth">
      <label>{t("mcpServiceDialog.oauthAuthorization")}</label>
      <div className="wk-mcp-oauth-status">
        <span className={'wk-mcp-badge ' + (oauth?.authorized ? 'wk-mcp-badge--ok' : oauth?.state === "refreshable" ? 'wk-mcp-badge--info' : 'wk-mcp-badge--warn')}>
          {oauth?.authorized ? t("mcpServiceDialog.oauthAuthorized") : oauth?.state === "refreshable" ? t("mcpServiceDialog.oauthRefreshable") : t("mcpServiceDialog.oauthUnauthorized")}
        </span>
        <TButton
          type="button"
          className="wk-mcp-oauth-btn"
          disabled={busy || authorizing}
          loading={authorizing}
          onClick={() => void onRequestAuthorize()}
        >
          {oauth?.state === "reauth_required" ? t("mcpServiceDialog.oauthAuthorize") : t("mcpServiceDialog.oauthReauthorize")}
        </TButton>
        {oauth && oauth.state !== "reauth_required" ? (
          <TButton type="button" className="wk-mcp-oauth-btn" disabled={busy || authorizing} onClick={() => void revoke()}>
            {t("mcpServiceDialog.oauthRevoke")}
          </TButton>
        ) : null}
      </div>
      {error ? <Status tone="error">{error}</Status> : null}
      <p className="wk-muted wk-mcp-oauth-hint">{t("mcpServiceDialog.oauthAuthorizeHint")}</p>
    </div>
  );
}

/**
 * Connection-step save payload, ported from Vue McpServiceDialog.buildPayload
 * (frontend/src/views/settings/components/McpServiceDialog.vue:898-938).
 * Vue never sends description or usage_instructions here: the backend PUT
 * rejects empty usage_instructions ("must contain between 1 and 16000
 * characters"), so including them breaks edit/create for services without
 * saved instructions. Usage instructions persist only through the step-2
 * save (update { usage_instructions }).
 *
 * The second argument mirrors Vue buildPayload(asCreate): on create the
 * add-mode secret rides along inline (auth_config.api_key); on edit,
 * secrets go through the /credentials subresource instead. api_key_header
 * is sent for the api_key strategy exactly like Vue (trimmed, may be "").
 */
export function buildMcpConnectionPayload(
  draft: Draft,
  asCreate = false,
): Record<string, unknown> {
  const headers = Object.fromEntries(
    draft.headers
      .map(({ key, value }) => [key.trim(), value.trim()])
      .filter(([key, value]) => key && value),
  );
  const auth: Record<string, unknown> = { auth_type: draft.authType };
  if (draft.authType === "api_key")
    auth.api_key_header = draft.apiKeyHeader.trim();
  if (draft.authType === "oauth")
    auth.scopes = draft.oauthScopes.split(/[\s,]+/).filter(Boolean);
  if (asCreate && draft.authType !== "oauth") {
    if (draft.apiKey.trim()) auth.api_key = draft.apiKey.trim();
  }
  return {
    name: draft.name.trim(),
    enabled: draft.enabled,
    transport_type: draft.transportType,
    advanced_config: {
      timeout: normalizeMcpAdvancedNumber(draft.timeout, 30, 1, 300),
      retry_count: normalizeMcpAdvancedNumber(draft.retryCount, 3, 0, 10),
      retry_delay: normalizeMcpAdvancedNumber(draft.retryDelay, 1, 0, 60),
    },
    url: draft.url.trim() || undefined,
    headers,
    auth_config: auth,
  };
}

export function normalizeMcpAdvancedNumber(value: number | "", fallback: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, typeof value === "number" && Number.isFinite(value) ? value : fallback));
}

export function McpSettingsPanel({ client, role, initialServices }: Props) {
  const t = createTranslator(useAppLocale());
  const locale = useAppLocale();
  // Vue McpSettings.vue gates every MCP mutation and management control with
  // authStore.hasRole('admin') (frontend/src/stores/auth.ts:179) — a rank
  // comparison (viewer < contributor < admin < owner), so owners pass too and
  // keep the add-service tile in the empty state (R428 evidence). Only
  // viewer/contributor fall back to the plain "no services" empty state.
  const canEdit = roleAtLeast(role, "admin");
  const [services, setServices] = useState<McpService[]>(() =>
    (initialServices ?? []).map(asService),
  );
  const [loading, setLoading] = useState(initialServices === undefined);
  const [error, setError] = useState<string | null>(null);
  // R472 A2 — 加载失败态与草稿校验错误分离：loadError 走 Vue 对齐的
  // Toast + 中央空态 + 重试（McpSettings.vue:144-147），不复用草稿 error。
  const [loadError, setLoadError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [draft, setDraft] = useState<Draft | null>(null);
  const [step, setStep] = useState<0 | 1>(0);
  const [busyId, setBusyId] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [generatingUsage, setGeneratingUsage] = useState(false);
  const [toolsSynced, setToolsSynced] = useState(false);
  const [metadataBusy, setMetadataBusy] = useState(false);
  const formRef = useRef<HTMLFormElement | null>(null);
  async function load() {
    setLoading(true);
    setError(null);
    setLoadError(null);
    try {
      setServices((await client.configuration.mcp.list()).map(asService));
    } catch {
      // Vue McpSettings.vue:145 — MessagePlugin.error(t('mcpSettings.toasts.loadFailed'))：
      // 纯本地化 toast，不透传后端原文；列表区由中央空态 + 重试替代。
      setLoadError(t("mcpSettings.toasts.loadFailed"));
      pushSettingsToast(t("mcpSettings.toasts.loadFailed"));
    } finally {
      setLoading(false);
    }
  }
  useEffect(() => {
    if (initialServices === undefined) void load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [client, initialServices]);
  function setField<K extends keyof Draft>(key: K, value: Draft[K]) {
    setDraft((current) => (current ? { ...current, [key]: value } : current));
  }
  /** Vue saveConnection(): validate, PUT/POST, keep the drawer open. */
  async function saveConnection(): Promise<{ id: string } | null> {
    if (!draft || saving) return null;
    const validation = validateMcpDraft(draft, 0);
    if (validation) {
      const messageKey: Record<McpDraftValidationError, string> = {
        nameRequired: "mcpServiceDialog.rules.nameRequired",
        urlRequired: "mcpServiceDialog.rules.urlRequired",
        urlInvalid: "mcpServiceDialog.rules.urlInvalid",
        stdioUnsupported: "mcpServiceDialog.codeImport.errors.stdioUnsupported",
        usageRequired: "mcpMetadata.instructionsRequired",
      };
      setError(t(messageKey[validation]));
      return null;
    }
    setSaving(true);
    setError(null);
    setNotice(null);
    try {
      const asCreate = !draft.id;
      const payload = buildMcpConnectionPayload(draft, asCreate);
      const saved = draft.id
        ? await client.configuration.mcp.update(draft.id, payload)
        : await client.configuration.mcp.create(payload);
      if (!asCreate && draft.apiKey.trim()) {
        try {
          await client.configuration.mcp.credentials.put(saved.id, {
            apiKey: draft.apiKey.trim(),
          });
        } catch (cause) {
          setDraft({ ...draft, id: saved.id });
          throw cause;
        }
      }
      setDraft((current) => (current ? { ...current, id: saved.id } : current));
      await load();
      return saved;
    } catch (cause) {
      setError(
        cause instanceof Error ? cause.message : t("mcpServiceDialog.toasts.updateFailed"),
      );
      return null;
    } finally {
      setSaving(false);
    }
  }
  async function clearMcpCredential() {
    if (!draft?.id || saving) return;
    setSaving(true); setError(null);
    try {
      await client.configuration.mcp.credentials.remove(draft.id, "api_key");
      setDraft((current) => current ? { ...current, apiKey: "", credentialConfigured: false } : current);
      setNotice(t("common.success"));
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : t("mcpServiceDialog.toasts.updateFailed"));
    } finally { setSaving(false); }
  }
  async function save(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!draft || saving) return;
    if (step === 0) {
      // Vue handleNext(): persist the connection, then reveal the tools step.
      const saved = await saveConnection();
      if (saved) {
        setToolsSynced(false);
        setStep(1);
      }
      return;
    }
    if (!draft.id || metadataBusy || generatingUsage) return;
    const instructions = draft.usageInstructions.trim();
    if (!instructions) {
      setError(t("mcpMetadata.instructionsRequired"));
      return;
    }
    if (!toolsSynced) {
      setError(t("mcpMetadata.syncRequired"));
      return;
    }
    setSaving(true);
    setError(null);
    setNotice(null);
    try {
      await client.configuration.mcp.update(draft.id, {
        usage_instructions: instructions,
      });
      setDraft(null);
      setStep(0);
      setNotice(t("mcpServiceDialog.toasts.updated"));
      await load();
    } catch (cause) {
      setError(
        cause instanceof Error ? cause.message : t("mcpServiceDialog.toasts.updateFailed"),
      );
    } finally {
      setSaving(false);
    }
  }
  /** Vue handleGenerateUsage(): fill the textarea only; save persists it. */
  async function generateUsage() {
    if (!draft?.id || generatingUsage || metadataBusy || !toolsSynced) return;
    setGeneratingUsage(true);
    setError(null);
    try {
      const generated = await client.configuration.mcp.usageInstructions.generate(
        draft.id,
        locale,
      );
      setField("usageInstructions", generated);
      setNotice(t("mcpMetadata.generated"));
    } catch {
      setError(t("mcpMetadata.generateFailed"));
    } finally {
      setGeneratingUsage(false);
    }
  }
  /** Vue startAuthorize(): popup + poll against the saved service id. */
  async function startAuthorize(savedId: string) {
    if (typeof window === "undefined") return;
    setSaving(true);
    setError(null);
    try {
      const result = await client.configuration.mcp.oauth.authorizeUrl(savedId, {
        redirectURI: window.location.origin + "/api/v1/mcp-oauth/callback",
        frontendRedirect: window.location.href,
      });
      const popup = window.open(
        result.authorizationUrl,
        "weknora_mcp_oauth",
        "width=600,height=720",
      );
      if (!popup) throw new Error(t("mcpServiceDialog.toasts.updateFailed"));
      for (let attempt = 0; attempt < 40; attempt += 1) {
        await new Promise((resolve) => window.setTimeout(resolve, 1500));
        const next = await client.configuration.mcp.oauth.status(
          savedId,
          result.authorizationAttempt,
        );
        if (next.authorized) break;
        if (popup.closed) break;
      }
    } catch (cause) {
      setError(
        cause instanceof Error ? cause.message : t("mcpServiceDialog.toasts.updateFailed"),
      );
    } finally {
      setSaving(false);
    }
  }
  /** Vue handleAuthorize(): ALWAYS save the connection first, then authorize. */
  async function authorizeOAuth() {
    const saved = await saveConnection();
    if (saved) await startAuthorize(saved.id);
  }
  async function toggle(service: McpService) {
    if (!canEdit || service.is_builtin || busyId) return;
    setBusyId(service.id);
    setError(null);
    setNotice(null);
    try {
      const enabled = service.enabled === false;
      await client.configuration.mcp.update(service.id, { enabled });
      setServices((current) =>
        current.map((item) =>
          item.id === service.id ? { ...item, enabled } : item,
        ),
      );
      setNotice(enabled ? t("mcpSettings.toasts.enabled") : t("mcpSettings.toasts.disabled"));
    } catch (cause) {
      setError(
        cause instanceof Error ? cause.message : t("mcpSettings.toasts.updateFailed"),
      );
    } finally {
      setBusyId(null);
    }
  }
  async function remove(service: McpService) {
    if (!canEdit || service.is_builtin || busyId) return;
    if (
      typeof window !== "undefined" &&
      !window.confirm(t("mcpSettings.deleteConfirmBody", { name: service.name }))
    )
      return;
    setBusyId(service.id);
    setError(null);
    setNotice(null);
    try {
      await client.configuration.mcp.remove(service.id);
      setServices((current) =>
        current.filter((item) => item.id !== service.id),
      );
      setNotice(t("mcpSettings.toasts.deleted"));
    } catch (cause) {
      setError(
        cause instanceof Error ? cause.message : t("mcpSettings.toasts.deleteFailed"),
      );
    } finally {
      setBusyId(null);
    }
  }
  function openEditor(service?: McpService, initialStep: 0 | 1 = 0) {
    setDraft(draftFrom(service));
    setStep(initialStep);
    setToolsSynced(false);
    setMetadataBusy(false);
    setGeneratingUsage(false);
    setError(null);
    setNotice(null);
  }
  function closeEditor() {
    setDraft(null);
    setStep(0);
  }
  const dialogBusy = saving || generatingUsage || metadataBusy;
  if (loading)
    return (
      <Card data-testid="mcp-settings">
        <Status>{t("common.loading")}</Status>
      </Card>
    );
  return (
    <section className="wk-mcp-page" data-testid="mcp-settings">
      <div className="wk-mcp-page-header">
        <div>
          <h2>{t("mcpSettings.title")}</h2>
          <p className="wk-muted">
            {t("mcpSettings.description")}
          </p>
        </div>
      </div>
      {error ? <Status tone="error">{error}</Status> : null}
      {notice ? <Status tone="success">{notice}</Status> : null}
      {/* R472 A2 — Vue McpSettings.vue 加载失败：列表区替换为中央空态 +
          重试（toast 已在 load catch 推送）；标题与说明保持渲染。 */}
      {loadError ? (
        <div data-testid="settings-load-empty" className="wk-mcp-load-empty">
          <EmptyState description={loadError}>
            <TButton type="button" theme="primary" onClick={() => { void load(); }}>{t("common.retry")}</TButton>
          </EmptyState>
        </div>
      ) : services.length === 0 && !canEdit ? (
        <Status>{t("mcpSettings.empty")}</Status>
      ) : (
        <div className="wk-mcp-card-grid">
          {services.map((service) => (
            <article key={service.id} className="wk-mcp-service-card">
              <div className="wk-mcp-service-card-main">
                <span className="wk-mcp-card-icon" aria-hidden="true"><McpCardIcon name="tools" /></span>
                <div className="wk-mcp-service-card-body">
                  <div className="wk-mcp-service-card-header">
                    <h4 title={service.name}>{service.name}</h4>
                  {service.is_builtin ? (
                    <span className="wk-mcp-builtin">{t("mcpSettings.builtin")}</span>
                  ) : null}
                    {canEdit ? (
                      <div className="wk-mcp-card-actions">
                        <button type="button" className="wk-mcp-icon-btn" title={t("common.edit")} aria-label={`${service.name} · ${t("common.edit")}`} onClick={() => openEditor(service)}><McpCardIcon name="edit" /><span className="wk-sr-only">{t("common.edit")}</span></button>
                        {service.is_builtin ? null : <button type="button" className="wk-mcp-icon-btn wk-mcp-icon-btn--danger" title={t("common.delete")} aria-label={`${service.name} · ${t("common.delete")}`} onClick={() => void remove(service)}><McpCardIcon name="delete" /><span className="wk-sr-only">{t("common.delete")}</span></button>}
                      </div>
                    ) : null}
                  </div>
                  {serviceDescription(service) ? <p className="wk-mcp-service-card-desc" title={serviceDescription(service)}>{serviceDescription(service).replace(/\s+/g, " ")}</p> : canEdit && !service.is_builtin ? <button type="button" className="wk-mcp-add-usage" onClick={() => openEditor(service, 1)}>＋ {t("mcpSettings.addUsageInstructions")}</button> : <span className="wk-mcp-no-usage">{t("mcpSettings.noUsageInstructions")}</span>}
                  <div className="wk-mcp-service-card-footer">
                    <div className="wk-mcp-card-meta">
                      <button type="button" className={'wk-mcp-tools-link' + (service.catalog?.stale ? ' is-stale' : '')} title={t("mcpMetadata.toolsAndUsage")} onClick={() => canEdit && openEditor(service, 1)} disabled={!canEdit}>
                        {service.catalog?.stale ? <McpCardIcon name="error" /> : null}{service.catalog ? t("mcpSettings.toolCount", { count: service.catalog.tool_count ?? 0 }) : t("mcpSettings.toolsNotSynced")} {service.catalog?.stale ? ` · ${t("mcpSettings.toolsStale")}` : ""} {canEdit ? <McpCardIcon name="chevron-right" /> : null}
                      </button>
                      <span className="wk-mcp-type">{service.transport_type === "http-streamable" ? "HTTP Streamable" : service.transport_type === "stdio" ? "Stdio" : "SSE"}</span>
                    </div>
                    {canEdit && !service.is_builtin ? <button type="button" className={'wk-mcp-status-btn' + (service.enabled === false ? '' : ' is-on')} role="switch" aria-checked={service.enabled !== false} disabled={busyId === service.id} onClick={() => void toggle(service)}><span className={'wk-mcp-status-dot' + (service.enabled === false ? '' : ' is-on')} aria-hidden="true" />{service.enabled === false ? t("mcpSettings.disabled") : t("mcpSettings.enabled")}</button> : <span className={'wk-mcp-status-btn' + (service.enabled === false ? '' : ' is-on')}><span className={'wk-mcp-status-dot' + (service.enabled === false ? '' : ' is-on')} aria-hidden="true" />{service.enabled === false ? t("mcpSettings.disabled") : t("mcpSettings.enabled")}</span>}
                  </div>
                </div>
              </div>
            </article>
          ))}
          {/* Vue McpSettings.vue:308-334 — add 卡为 --td-component-stroke #e7e7e7
              虚线、占位色文字 rgba(0,0,0,.4)；图标块 #f3f3f3 底 + secondary 色。 */}
          {canEdit ? <button type="button" className="wk-mcp-add-tile" onClick={() => openEditor()}><span className="wk-mcp-add-tile__icon" aria-hidden="true">{/* Vue add-icon（tdesign AddIcon，strokeWidth 2 square cap）——TIcon 直译（台账 #10 glyph 同源）。 */}<TIcon name="add" /></span><span className="wk-mcp-add-tile__label">{t("mcpSettings.addService")}</span></button> : null}
        </div>
      )}
      {draft ? (
        <McpDrawerShell>
        <div
          className="wks-overlay wks-mcp-overlay"
          data-testid="mcp-editor-overlay"
        >
          <div
            className="wks-modal wks-mcp-drawer"
            role="dialog"
            aria-modal="true"
            aria-label={draft.id ? t("mcpServiceDialog.editTitle") : t("mcpServiceDialog.addTitle")}
          >
            <div className="wk-settings-panel-heading wk-mcp-drawer-head">
              <div className="wk-mcp-drawer-title-row">
                <span className={'wk-mcp-drawer-icon' + (draft.transportType === "http-streamable" ? ' wk-mcp-drawer-icon--http' : ' wk-mcp-drawer-icon--sse')} aria-hidden="true"><McpTransportIcon transport={draft.transportType === "http-streamable" ? "http-streamable" : "sse"} /></span>
                <div className="wk-mcp-drawer-title-text">
                  <h3>{draft.id ? t("mcpServiceDialog.editTitle") : t("mcpServiceDialog.addTitle")}</h3>
                  <p className="wk-muted">
                    {draft.transportType === "http-streamable" ? "HTTP Streamable" : "SSE"}
                    <span className={'wk-mcp-badge ' + (draft.enabled ? 'wk-mcp-badge--ok' : 'wk-mcp-badge--muted')}>
                      {draft.enabled ? t("mcpSettings.enabled") : t("mcpSettings.disabled")}
                    </span>
                  </p>
                </div>
              </div>
              <nav className="wk-mcp-drawer-steps" aria-label={t("mcpMetadata.setupProgress")}>
                  <button
                    type="button"
                    className={'wk-mcp-step-btn wk-mcp-step-btn--grow' + (step === 0 ? ' is-active' : step > 0 ? ' is-done' : '')}
                    aria-current={step === 0 ? "step" : undefined}
                    disabled={dialogBusy}
                    onClick={() => setStep(0)}
                  >
                    <span className={'wk-mcp-step-marker' + (step === 0 ? ' is-current' : ' is-done')}>{step > 0 ? "✓" : "1"}</span>
                    <span className="wk-mcp-step-label">{t("mcpMetadata.connection")}</span>
                    <span className={'wk-mcp-step-line' + (step > 0 ? ' is-done' : '')} aria-hidden="true" />
                  </button>
                  <button
                    type="button"
                    className={'wk-mcp-step-btn' + (step === 1 ? ' is-active' : '')}
                    aria-current={step === 1 ? "step" : undefined}
                    disabled={dialogBusy}
                    onClick={() => {
                      if (step === 0) formRef.current?.requestSubmit();
                    }}
                  >
                    <span className={'wk-mcp-step-marker' + (step === 1 ? ' is-current' : ' is-idle')}>2</span>
                    <span className="wk-mcp-step-label">{t("mcpMetadata.toolsAndUsage")}</span>
                  </button>
              </nav>
            </div>
            <form
              ref={formRef}
              className="wk-mcp-form wk-settings-editor wk-mcp-editor-form"
              onSubmit={(event) => void save(event)}
            >
              {step === 0 ? (
                <>
                  <details className="wk-mcp-code-import">
                    <summary>{t("mcpServiceDialog.codeImport.toggle")}</summary>
                    <p className="wk-muted">
                      {t("mcpServiceDialog.codeImport.hint")}
                    </p>
                    <TTextarea
                      rows={5}
                      value={draft.codeImport}
                      placeholder={'{\n  "mcpServers": { "my-server": { "url": "https://example.com/sse" } }\n}'}
                      onChange={(value) => setField("codeImport", String(value))}
                    />
                    <TButton
                      type="button"
                      onClick={() =>
                        setDraft((current) =>
                          current
                            ? importMcpConfig(current.codeImport, current)
                            : current,
                        )
                      }
                    >
                      {t("mcpServiceDialog.codeImport.parse")}
                    </TButton>
                    {draft.codeImportError ? (
                      <Status tone="error">{draft.codeImportError}</Status>
                    ) : null}
                  </details>
                  <fieldset className="wk-mcp-group">
                    <legend className="wk-mcp-group-title">{t("mcpServiceDialog.basicSection")}</legend>
                    <label className="wk-mcp-required">
                      <span className="wk-mcp-label-text">{t("mcpServiceDialog.name")}</span>
                      <TInput
                        maxlength={128}
                        value={draft.name}
                        placeholder={t("mcpServiceDialog.namePlaceholder")}
                        onChange={(value) => setField("name", String(value))}
                      />
                    </label>
                    <div className="wk-mcp-enable-row">
                      <label className="wk-checkbox">
                        <TCheckbox
                          checked={draft.enabled}
                          onChange={(value) => setField("enabled", Boolean(value))}
                        />{" "}
                        {t("mcpServiceDialog.enableService")}
                      </label>
                      <span className="wk-muted">
                        {t("mcpServiceDialog.enableServiceDesc")}
                      </span>
                    </div>
                  </fieldset>
                  <fieldset className="wk-mcp-group">
                    <legend className="wk-mcp-group-title">{t("mcpServiceDialog.connectionSection")}</legend>
                    <label className="wk-mcp-required">
                      <span className="wk-mcp-label-text">{t("mcpServiceDialog.transportType")}</span>
                      <div className="wk-mcp-source-options" role="radiogroup" aria-label={t("mcpServiceDialog.transportType")}>
                        {(["sse", "http-streamable"] as const).map((transport) => {
                          const active = draft.transportType === transport;
                          return <button key={transport} type="button" role="radio" aria-checked={active} className={'wk-mcp-source-option' + (active ? ' is-active' : '')} onClick={() => setField("transportType", transport)}><McpTransportIcon transport={transport} /><span>{transport === "http-streamable" ? "HTTP Streamable" : "SSE"}</span></button>;
                        })}
                      </div>
                    </label>
                    <label className="wk-mcp-required">
                      <span className="wk-mcp-label-text">{t("mcpServiceDialog.serviceUrl")}</span>
                      <TInput
                        type="url"
                        value={draft.url}
                        placeholder={t("mcpServiceDialog.serviceUrlPlaceholder")}
                        onChange={(value) => setField("url", String(value))}
                        />
                    </label>
                    <fieldset className="wk-mcp-custom-headers">
                      <legend>
                        <span>{t("mcpServiceDialog.customHeaders.label")}</span>
                        <TButton
                          type="button"
                          className="wk-mcp-header-add-btn"
                          onClick={() =>
                            setField("headers", [
                              ...draft.headers,
                              { key: "", value: "" },
                            ])
                          }
                        >
                          <McpCardIcon name="add" size={14} /> {t("mcpServiceDialog.customHeaders.add")}
                        </TButton>
                      </legend>
                      <p className="wk-muted">{t("mcpServiceDialog.customHeaders.desc")}</p>
                      {draft.headers.map((header, index) => (
                        <div className="wk-mcp-header-row" key={index}>
                          <TInput
                            placeholder={t("mcpServiceDialog.customHeaders.keyPlaceholder")}
                            value={header.key}
                            onChange={(value) =>
                              setField(
                                "headers",
                                draft.headers.map((item, itemIndex) =>
                                  itemIndex === index
                                    ? { ...item, key: String(value) }
                                    : item,
                                ),
                              )
                            }
                        />
                          <TInput
                            placeholder={t("mcpServiceDialog.customHeaders.valuePlaceholder")}
                            value={header.value}
                            onChange={(value) =>
                              setField(
                                "headers",
                                draft.headers.map((item, itemIndex) =>
                                  itemIndex === index
                                    ? { ...item, value: String(value) }
                                    : item,
                                ),
                              )
                            }
                          />
                          <TButton
                            type="button"
                            className="wk-mcp-header-remove-btn"
                            onClick={() =>
                              setField(
                                "headers",
                                draft.headers.filter(
                                  (_, itemIndex) => itemIndex !== index,
                                ),
                              )
                            }
                          >
                            <McpCardIcon name="delete" size={14} /><span className="wk-sr-only">{t("common.delete")}</span>
                          </TButton>
                        </div>
                      ))}
                    </fieldset>
                  </fieldset>
                  <fieldset className="wk-mcp-group">
                    <legend className="wk-mcp-group-title">{t("mcpServiceDialog.authConfig")}</legend>
                    <label>
                      {t("mcpServiceDialog.authType")}
                      <div className="wk-mcp-source-options wk-mcp-source-options--wrap" role="radiogroup" aria-label={t("mcpServiceDialog.authType")}>
                        {(["", "api_key", "oauth"] as const).map((authType) => {
                          const active = draft.authType === authType;
                          const label = authType === "" ? t("mcpServiceDialog.authTypeNone") : authType === "api_key" ? t("mcpServiceDialog.authTypeApiKey") : t("mcpServiceDialog.authTypeOAuth");
                          return <button key={authType || "none"} type="button" role="radio" aria-checked={active} className={'wk-mcp-source-option' + (active ? ' is-active' : '')} onClick={() => setField("authType", authType)}>{label}</button>;
                        })}
                      </div>
                    </label>
                    {draft.authType === "oauth" ? (
                      <>
                        <label>
                          {t("mcpServiceDialog.oauthScopes")}
                          <TInput
                            value={draft.oauthScopes}
                            placeholder={t("mcpServiceDialog.optional")}
                            onChange={(value) =>
                              setField("oauthScopes", String(value))
                            }
                          />
                        </label>
                        {draft.id ? (
                          <McpOAuthControl
                            client={client}
                            serviceId={draft.id}
                            busy={dialogBusy}
                            onRequestAuthorize={authorizeOAuth}
                          />
                        ) : null}
                      </>
                    ) : null}
                    {draft.authType === "api_key" ? (
                      <>
                        <label>
                          {t("mcpServiceDialog.apiKeyHeader")}
                          <TInput
                            value={draft.apiKeyHeader}
                            placeholder="X-API-Key"
                            onChange={(value) =>
                              setField("apiKeyHeader", String(value))
                            }
                          />
                        </label>
                        <p className="wk-muted">{t("mcpServiceDialog.apiKeyHeaderDesc")}</p>
                        {draft.id ? <div className="wk-mcp-credential-card"><div className="wk-mcp-credential-head"><strong>{t("mcpServiceDialog.credentialValue")}</strong><span className={'wk-mcp-credential-state' + (draft.credentialConfigured ? ' is-set' : '')}>{draft.credentialConfigured ? "✓ " + t("common.success") : t("mcpServiceDialog.optional")}</span></div><label>{draft.credentialConfigured ? t("common.replaceValue") : t("mcpServiceDialog.credentialValue")}<TInput type="password" autocomplete="new-password" value={draft.apiKey} placeholder={t("mcpServiceDialog.optional")} onChange={(value) => setField("apiKey", String(value))} /></label>{draft.credentialConfigured ? <TButton type="button" disabled={saving} onClick={() => void clearMcpCredential()}>{t("common.delete")}</TButton> : null}</div> : <label>{t("mcpServiceDialog.credentialValue")}<TInput type="password" autocomplete="new-password" value={draft.apiKey} placeholder={t("mcpServiceDialog.optional")} onChange={(value) => setField("apiKey", String(value))} /></label>}
                      </>
                    ) : null}
                  </fieldset>
                  <fieldset className="wk-mcp-group">
                    <legend className="wk-mcp-group-title">{t("mcpServiceDialog.advancedConfig")}</legend>
                    <label>
                      {t("mcpServiceDialog.timeoutSec")}
                      <div className="wk-mcp-number-field">
                        <TInput
className="wk-mcp-number-input"
                          value={draft.timeout === "" ? "" : String(draft.timeout)}
                          onChange={(value) => setField("timeout", String(value) === "" ? "" : Number(String(value)))}
                          onBlur={() => setField("timeout", normalizeMcpAdvancedNumber(draft.timeout, 30, 1, 300))}
                        />
                        <span className="wk-mcp-unit">{t("mcpServiceDialog.unitSecond")}</span>
                      </div>
                    </label>
                    <label>
                      {t("mcpServiceDialog.retryCount")}
                      <div className="wk-mcp-number-field">
                        <TInput
className="wk-mcp-number-input"
                          value={draft.retryCount === "" ? "" : String(draft.retryCount)}
                          onChange={(value) => setField("retryCount", String(value) === "" ? "" : Number(String(value)))}
                          onBlur={() => setField("retryCount", normalizeMcpAdvancedNumber(draft.retryCount, 3, 0, 10))}
                        />
                        <span className="wk-mcp-unit">{t("mcpServiceDialog.unitTimes")}</span>
                      </div>
                    </label>
                    <label>
                      {t("mcpServiceDialog.retryDelaySec")}
                      <div className="wk-mcp-number-field">
                        <TInput
className="wk-mcp-number-input"
                          value={draft.retryDelay === "" ? "" : String(draft.retryDelay)}
                          onChange={(value) => setField("retryDelay", String(value) === "" ? "" : Number(String(value)))}
                          onBlur={() => setField("retryDelay", normalizeMcpAdvancedNumber(draft.retryDelay, 1, 0, 60))}
                        />
                        <span className="wk-mcp-unit">{t("mcpServiceDialog.unitSecond")}</span>
                      </div>
                    </label>
                  </fieldset>
                </>
              ) : (
                <>
                  <fieldset className="wk-mcp-group">
                    <div className="wk-settings-panel-heading wk-mcp-sticky-head">
                      <div>
                        <h4>{t("mcpMetadata.usage")}</h4>
                        <p className="wk-muted wk-mcp-head-desc">{t("mcpMetadata.usageHint")}</p>
                      </div>
                    </div>
                    <div className="wk-mcp-usage-row">
                      <label className="wk-form-label wk-mcp-required">
                        <span className="wk-mcp-label-text">{t("mcpMetadata.usageInstructions")}</span>
                      </label>
                      <TButton
                        type="button"
                        disabled={
                          !toolsSynced || metadataBusy || saving || generatingUsage
                        }
                        loading={generatingUsage}
                        onClick={() => void generateUsage()}
                      >
                        {t("mcpMetadata.generateUsage")}
                      </TButton>
                    </div>
                    <TTextarea
                      rows={5}
                      maxLength={16000}
                      value={draft.usageInstructions}
                      placeholder={t("mcpMetadata.instructionsPlaceholder")}
                      onChange={(value) =>
                        setField("usageInstructions", String(value))
                      }
                    />
                    <span className="wk-muted wk-mcp-usage-count">
                      {draft.usageInstructions.length}/16000
                    </span>
                    <p className="wk-muted">{t("mcpMetadata.generateHint")}</p>
                  </fieldset>
                  {draft.id ? (
                    <McpMetadataSection
                      client={client}
                      serviceId={draft.id}
                      disabled={saving || generatingUsage}
                      onSyncedChange={setToolsSynced}
                      onBusyChange={setMetadataBusy}
                    />
                  ) : null}
                </>
              )}
            <div className="wk-mcp-footer">
                <div className="wk-mcp-footer-prev">
                  {step === 1 ? (
                    <TButton
                      type="button"
                      disabled={dialogBusy}
                      onClick={() => setStep(0)}
                    >
                      {t("mcpMetadata.previous")}
                    </TButton>
                  ) : null}
                </div>
                <div className="wk-mcp-footer-actions">
                  <TButton
                    type="button"
                    disabled={saving}
                    onClick={closeEditor}
                  >
                    {t("common.cancel")}
                  </TButton>
                  <TButton
                    type="submit"
                    loading={saving}
                    disabled={
                      metadataBusy ||
                      generatingUsage ||
                      (step === 1 && !toolsSynced)
                    }
                  >
                    {step === 0 ? t("mcpMetadata.saveNext") : t("common.save")}
                  </TButton>
                </div>
              </div>
            </form>
          </div>
        </div>
        </McpDrawerShell>
      ) : null}
    </section>
  );
}
