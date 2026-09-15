import { useEffect, useRef, useState } from "react";
import * as React from "react";
import type { McpConfiguration, WeKnoraClient } from "@weknora/api-client";
import { Button, Card, Checkbox, Input, Select, Status, Textarea } from "@weknora/ui";
import { McpToolsDirectory } from "./McpToolsDirectory.tsx";
import { createTranslator, useAppLocale } from "../i18n.ts";

/* Tailwind utilities migrated from the deleted .wk-mcp-* rules in styles.css
   (see docs/plans/tailwind-shadcn-conventions.md). Values encode the effective
   cascade result, including ! where a retained unlayered rule competes. */
const mcpServerDocsButton = "ml-2 rounded-[4px] border-0 bg-transparent px-1 py-[0.1rem] text-[#2e6de6] cursor-pointer [font:inherit] hover:bg-[#eef4ff]";
const mcpIconButton = "h-6 w-6 cursor-pointer rounded-[6px] border-0 bg-transparent p-0 text-[16px] text-[#66758b] hover:bg-[#f3f5f8] hover:text-[#245a9b] hover:outline-none focus-visible:bg-[#f3f5f8] focus-visible:text-[#245a9b] focus-visible:outline-none";
const mcpStepButton = "flex min-w-0 items-center gap-2 cursor-pointer border-0 bg-transparent p-0 [font:inherit] text-[#98a2b8] disabled:cursor-default disabled:opacity-60";
const mcpMetadata = "flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1.5";
const mcpToolsLink = "inline-flex min-w-0 max-w-full items-center gap-1 rounded-[6px] border-0 bg-[#f3f5f8] px-1.5 py-0.5 text-left [font:inherit] text-[12px] leading-[18px] text-[#66758b] hover:bg-[#f3f5f8] hover:text-[rgb(0_0_0_/_90%)] hover:outline-none focus-visible:text-[#245a9b] focus-visible:outline-none";
const mcpType = "shrink-0 text-[11px] leading-[18px] text-[#66758b]";
const mcpStatus = "inline-flex items-center gap-[5px] whitespace-nowrap cursor-pointer rounded-[6px] border-0 bg-transparent px-1 py-0.5 [font:inherit] text-[12px] leading-[18px] text-[#66758b] hover:bg-[#f3f5f8] disabled:cursor-wait";
const mcpBadgeOk = "rounded-full px-2 py-[0.1rem] text-[.72rem] bg-[#ecfdf3] text-[#137333]";
const mcpBadgeInfo = "rounded-full px-2 py-[0.1rem] text-[.72rem] bg-[#e8f1ff] text-[#2e6de6]";
const mcpBadgeWarn = "rounded-full px-2 py-[0.1rem] text-[.72rem] bg-[#fffaeb] text-[#b54708]";
const mcpBadgeMuted = "rounded-full px-2 py-[0.1rem] text-[.72rem] bg-[#f2f4f8] text-[#66758b]";

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
  timeout: number;
  retryCount: number;
  retryDelay: number;
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
  const [error, setError] = useState<string | null>(null);
  const [docsOpen, setDocsOpen] = useState(false);
  const loadGeneration = useRef(0);
  async function load(refresh = false) {
    const generation = ++loadGeneration.current;
    setBusy(true);
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
    const [metadataResult, approvalsResult] = await Promise.allSettled([
      metadataRequest(),
      client.configuration.mcp.toolApprovals.list(serviceId),
    ]);
    if (generation !== loadGeneration.current) return;
    if (metadataResult.status === "fulfilled") setMetadata(metadataResult.value);
    if (approvalsResult.status === "fulfilled")
      setApprovals(approvalsResult.value);
    else {
      setApprovals([]);
      setPolicyError(t("mcpMetadata.policyLoadFailed"));
    }
    if (metadataResult.status === "rejected")
      setError(
        metadataResult.reason instanceof Error
          ? metadataResult.reason.message
          : t("mcpMetadata.failed"),
      );
    setBusy(false);
    onBusyChange(false);
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
    if (busy || metadata?.stale || policyError) return;
    setBusy(true);
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
      setBusy(false);
      onBusyChange(false);
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
    <section className="wk-mcp-metadata mt-[1.1rem] border-t border-[#edf0f5] pt-4" aria-label={t("mcpMetadata.tools")}>
      <div className="wk-settings-panel-heading flex items-start justify-between gap-4 border-b border-[#eef1f5] pb-4 mb-4 max-[720px]:flex-col sticky top-0 z-[1] bg-white pt-[.25rem]">
        <div>
          <h4>{t("mcpMetadata.tools")}</h4>
          <p className="wk-muted text-muted m-0">{t("mcpMetadata.cacheHint")}</p>
        </div>
        <div className="wk-list-actions mb-[0.75rem] flex items-center justify-end gap-[0.5rem]">
          <Button
            type="button"
            disabled={busy || disabled}
            onClick={() => void load(true)}
          >
            {metadata ? t("mcpMetadata.refresh") : t("mcpMetadata.fetch")}
          </Button>
        </div>
      </div>
      {error ? <Status tone="error">{error}</Status> : null}
      {metadata ? (
        <>
          <div className={`relative ${metadata.stale ? "text-[#b54708] text-[.85rem]" : "wk-muted text-muted"}`}>
            {t("mcpMetadata.toolCount", { count: metadata.tools.length })}
            {metadata.serverName
              ? " · " + metadata.serverName + " " + (metadata.serverVersion ?? "")
              : ""}
            {syncedAt ? " · " + t("mcpMetadata.syncedAt") + syncedAt : ""}
            {metadata.stale ? " · " + t("mcpMetadata.stale") : ""}
            {metadata.instructions || metadata.serverDescription ? <><button type="button" className={`wk-mcp-server-docs-trigger ${mcpServerDocsButton}`} aria-expanded={docsOpen} onClick={() => setDocsOpen((open) => !open)}>{t("mcpMetadata.serverDocumentation")}⌄</button>{docsOpen ? <div className="absolute z-20 mt-[.4rem] grid w-[min(360px,calc(100vw_-_2rem))] gap-2 border border-[#dce3ed] rounded-[7px] bg-white p-3 shadow-[0_12px_30px_rgb(23_32_51_/_18%)] whitespace-normal text-[#172033]" role="dialog"><strong>{t("mcpMetadata.serverDocumentation")}</strong>{metadata.serverDescription ? <span>{metadata.serverDescription}</span> : null}{metadata.instructions ? <pre className="m-0 max-h-[180px] overflow-auto whitespace-pre-wrap [font:inherit]">{metadata.instructions}</pre> : null}</div> : null}</> : <button type="button" className={mcpServerDocsButton} aria-label={t("mcpMetadata.noServerDocumentation")}>ⓘ</button>}
          </div>
          <p className="wk-muted text-muted">{t("mcpMetadata.policyHint")}</p>
          <McpToolsDirectory
            tools={metadata.tools}
            serviceId={metadata.stale ? undefined : serviceId}
            approvals={approvals}
            busy={busy || metadata.stale}
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
    <div className="my-[.8rem] flex flex-wrap items-center gap-[.6rem]">
      <span className="wk-form-label text-ink font-semibold">{t("mcpServiceDialog.oauthAuthorization")}</span>
      <span
        className={
          oauth?.authorized
            ? mcpBadgeOk
            : oauth?.state === "refreshable"
              ? mcpBadgeInfo
              : mcpBadgeWarn
        }
      >
        {oauth?.authorized
          ? t("mcpServiceDialog.oauthAuthorized")
          : oauth?.state === "refreshable"
            ? t("mcpServiceDialog.oauthRefreshable")
            : t("mcpServiceDialog.oauthUnauthorized")}
      </span>
      <Button
        type="button"
        disabled={busy || authorizing}
        loading={authorizing}
        onClick={() => void onRequestAuthorize()}
      >
        {oauth?.state === "reauth_required"
          ? t("mcpServiceDialog.oauthAuthorize")
          : t("mcpServiceDialog.oauthReauthorize")}
      </Button>
      {oauth && oauth.state !== "reauth_required" ? (
        <Button type="button" disabled={busy || authorizing} onClick={() => void revoke()}>
          {t("mcpServiceDialog.oauthRevoke")}
        </Button>
      ) : null}
      {error ? <Status tone="error">{error}</Status> : null}
      <p className="wk-muted text-muted">{t("mcpServiceDialog.oauthAuthorizeHint")}</p>
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
      timeout: draft.timeout,
      retry_count: draft.retryCount,
      retry_delay: draft.retryDelay,
    },
    url: draft.url.trim() || undefined,
    headers,
    auth_config: auth,
  };
}

export function McpSettingsPanel({ client, role, initialServices }: Props) {
  const t = createTranslator(useAppLocale());
  const locale = useAppLocale();
  const canEdit = role === "admin" || role === "owner";
  const [services, setServices] = useState<McpService[]>(() =>
    (initialServices ?? []).map(asService),
  );
  const [loading, setLoading] = useState(initialServices === undefined);
  const [error, setError] = useState<string | null>(null);
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
    try {
      setServices((await client.configuration.mcp.list()).map(asService));
    } catch (cause) {
      setError(
        cause instanceof Error ? cause.message : t("mcpSettings.toasts.loadFailed"),
      );
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
    <section className="grid gap-4" data-testid="mcp-settings">
      <div className="wk-mcp-page-header flex items-start justify-between gap-4 mb-7 max-[720px]:flex-col">
        <div>
          <h2 className="m-0 mb-2 text-[20px] font-semibold leading-[1.2] text-[rgb(0_0_0_/_90%)]">{t("mcpSettings.title")}</h2>
          <p className="wk-muted m-0 text-[14px] leading-[1.6] text-[rgb(0_0_0_/_60%)]">
            {t("mcpSettings.description")}
          </p>
        </div>
      </div>
      {error ? <Status tone="error">{error}</Status> : null}
      {notice ? <Status tone="success">{notice}</Status> : null}
      {services.length === 0 && !canEdit ? (
        <Status>{t("mcpSettings.empty")}</Status>
      ) : (
        <div className="grid items-stretch gap-2.5 grid-cols-[repeat(auto-fill,minmax(min(100%,320px),1fr))] max-[720px]:grid-cols-1">
          {services.map((service) => (
            <article key={service.id} className="wk-mcp-service-card min-w-0 overflow-hidden rounded-[10px] border border-[#dce3ed] bg-white">
              <div className="wk-mcp-service-card-main flex min-w-0 flex-1 items-stretch p-3">
                <span className="inline-flex h-[26px] w-[26px] shrink-0 items-center justify-center self-start rounded-[7px] bg-[#f3f5f8] text-[#66758b]" aria-hidden="true"><McpCardIcon name="tools" /></span>
                <div className="wk-mcp-service-card-body flex min-w-0 flex-1 flex-col gap-2">
                  <div className="wk-mcp-service-card-header flex min-h-[28px] items-center justify-between gap-[.7rem]">
                    <h4 title={service.name} className="m-0 min-w-0 flex-1 overflow-hidden text-ellipsis whitespace-nowrap text-[14px] font-semibold leading-5 text-[rgb(0_0_0_/_90%)]">{service.name}</h4>
                  {service.is_builtin ? (
                    <span className="text-[.75rem] text-[#2e6de6]">{t("mcpSettings.builtin")}</span>
                  ) : null}
                    {canEdit ? (
                      <div className="flex shrink-0 items-center gap-0.5">
                        <button type="button" className={mcpIconButton} title={t("common.edit")} aria-label={`${service.name} · ${t("common.edit")}`} onClick={() => openEditor(service)}><McpCardIcon name="edit" /><span className="wk-sr-only">{t("common.edit")}</span></button>
                        {service.is_builtin ? null : <button type="button" className={`${mcpIconButton} hover:text-[#b42318]! focus-visible:text-[#b42318]!`} title={t("common.delete")} aria-label={`${service.name} · ${t("common.delete")}`} onClick={() => void remove(service)}><McpCardIcon name="delete" /><span className="wk-sr-only">{t("common.delete")}</span></button>}
                      </div>
                    ) : null}
                  </div>
                  {serviceDescription(service) ? <p className="wk-mcp-service-card-desc min-h-[2.6rem] [overflow-wrap:anywhere]" title={serviceDescription(service)}>{serviceDescription(service).replace(/\s+/g, " ")}</p> : canEdit && !service.is_builtin ? <button type="button" className="self-start items-center border-0 bg-transparent p-0 text-[.82rem] text-[#245a9b] cursor-pointer [font:inherit] hover:underline focus-visible:underline" onClick={() => openEditor(service, 1)}>＋ {t("mcpSettings.addUsageInstructions")}</button> : <span className="text-[.82rem] text-[#66758b]">{t("mcpSettings.noUsageInstructions")}</span>}
                  <div className="wk-mcp-service-card-footer flex items-center justify-between gap-[.7rem]">
                    <div className={mcpMetadata}>
                      <button type="button" className={`${mcpToolsLink} ${service.catalog?.stale ? "bg-[#fffaeb] text-[#b54708]!" : ""}`} title={t("mcpMetadata.toolsAndUsage")} onClick={() => canEdit && openEditor(service, 1)} disabled={!canEdit}>
                        {service.catalog?.stale ? <McpCardIcon name="error" /> : null}{service.catalog ? t("mcpSettings.toolCount", { count: service.catalog.tool_count ?? 0 }) : t("mcpSettings.toolsNotSynced")} {service.catalog?.stale ? ` · ${t("mcpSettings.toolsStale")}` : ""} {canEdit ? <McpCardIcon name="chevron-right" /> : null}
                      </button>
                      <span className={mcpType}>{service.transport_type === "http-streamable" ? "HTTP Streamable" : service.transport_type === "stdio" ? "Stdio" : "SSE"}</span>
                    </div>
                    {canEdit && !service.is_builtin ? <button type="button" className={`${mcpStatus} ${service.enabled === false ? "" : "text-[#137333]!"}`} role="switch" aria-checked={service.enabled !== false} disabled={busyId === service.id} onClick={() => void toggle(service)}><span className={`inline-block h-[5px] w-[5px] rounded-full ${service.enabled === false ? "bg-[#98a2b8]" : "bg-[#07c05f]"}`} aria-hidden="true" />{service.enabled === false ? t("mcpSettings.disabled") : t("mcpSettings.enabled")}</button> : <span className={`${mcpStatus} ${service.enabled === false ? "" : "text-[#137333]!"}`}><span className={`inline-block h-[5px] w-[5px] rounded-full ${service.enabled === false ? "bg-[#98a2b8]" : "bg-[#07c05f]"}`} aria-hidden="true" />{service.enabled === false ? t("mcpSettings.disabled") : t("mcpSettings.enabled")}</span>}
                  </div>
                </div>
              </div>
            </article>
          ))}
          {canEdit ? <button type="button" className="flex min-h-[88px] cursor-pointer flex-col items-center justify-center gap-1.5 rounded-[10px] border border-[#dce3ed] border-dashed bg-transparent p-3 text-center text-[#66758b] [font:inherit] [transition:border-color_.18s_ease,background_.18s_ease] hover:border-[#07c05f] hover:bg-[rgba(7,192,95,.06)] hover:text-[#07c05f] hover:outline-none focus-visible:border-[#07c05f] focus-visible:bg-[rgba(7,192,95,.06)] focus-visible:text-[#07c05f] focus-visible:outline-none" onClick={() => openEditor()}><span className="flex h-8 w-8 items-center justify-center rounded-[8px] bg-[#f3f5f8] text-[#66758b]" aria-hidden="true"><McpCardIcon name="add" size={18} /></span><span className="text-[13px] font-medium leading-[1.4]">{t("mcpSettings.addService")}</span></button> : null}
        </div>
      )}
      {draft ? (
        <div
          className="wks-overlay wks-mcp-overlay z-[1200]!"
          data-testid="mcp-editor-overlay"
        >
          <div
            className="wks-modal wks-mcp-drawer"
            role="dialog"
            aria-modal="true"
            aria-label={draft.id ? t("mcpServiceDialog.editTitle") : t("mcpServiceDialog.addTitle")}
          >
            <div className="wk-settings-panel-heading relative -mx-[18px] box-border flex h-[104px] shrink-0 flex-col gap-2 border-b border-[#eef1f5] mb-0 max-[720px]:flex-col sticky top-0 z-[1] bg-white px-[18px] pb-3 pt-[14px]">
              <div className="flex min-w-0 items-center gap-2.5">
                <span className={`inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-[9px] ${draft.transportType === "http-streamable" ? "bg-[rgba(0,82,217,.1)] text-[#0052d9]" : "bg-[rgba(17,128,83,.12)] text-[#118053]"}`} aria-hidden="true"><McpTransportIcon transport={draft.transportType === "http-streamable" ? "http-streamable" : "sse"} /></span>
                <div className="flex min-w-0 flex-1 flex-col gap-px">
                  <h3 className="m-0 overflow-hidden text-ellipsis whitespace-nowrap text-[15px] font-semibold leading-[21px]">{draft.id ? t("mcpServiceDialog.editTitle") : t("mcpServiceDialog.addTitle")}</h3>
                  <p className="wk-muted text-muted m-0 flex items-center gap-2 text-[12px] leading-[18px]">
                    {draft.transportType === "http-streamable" ? "HTTP Streamable" : "SSE"}
                    <span className={draft.enabled ? mcpBadgeOk : mcpBadgeMuted}>
                      {draft.enabled ? t("mcpSettings.enabled") : t("mcpSettings.disabled")}
                    </span>
                  </p>
                </div>
              </div>
              <nav className="flex h-6 items-center gap-2" aria-label={t("mcpMetadata.setupProgress")}>
                  <button
                    type="button"
                    className={`${mcpStepButton} flex-1 text-[13px] font-medium ${step === 0 ? "text-[#07c05f]!" : step > 0 ? "text-[#506078]!" : ""}`}
                    aria-current={step === 0 ? "step" : undefined}
                    disabled={dialogBusy}
                    onClick={() => setStep(0)}
                  >
                    <span className={`inline-flex h-[22px] w-[22px] shrink-0 items-center justify-center rounded-full border text-[11px] font-semibold leading-none ${step === 0 ? "border-[#07c05f] bg-[#07c05f] text-white" : "border-[#07c05f] bg-[rgba(7,192,95,.12)] text-[#07c05f]"}`}>{step > 0 ? "✓" : "1"}</span>
                    <span className="min-w-0 overflow-hidden text-ellipsis whitespace-nowrap">{t("mcpMetadata.connection")}</span>
                    <span className={`mx-1 h-px min-w-4 flex-1 bg-[#dcdcdc] ${step > 0 ? "bg-[rgba(7,192,95,.35)]" : ""}`} aria-hidden="true" />
                  </button>
                  <span aria-hidden="true"> → </span>
                  <button
                    type="button"
                    className={`${mcpStepButton} text-[13px] font-medium ${step === 1 ? "text-[#07c05f]!" : ""}`}
                    aria-current={step === 1 ? "step" : undefined}
                    disabled={dialogBusy}
                    onClick={() => {
                      if (step === 0) formRef.current?.requestSubmit();
                    }}
                  >
                    <span className={`inline-flex h-[22px] w-[22px] shrink-0 items-center justify-center rounded-full border text-[11px] font-semibold leading-none ${step === 1 ? "border-[#07c05f] bg-[#07c05f] text-white" : "border-[#cbd5e1] text-[#98a2b8]"}`}>2</span>
                    <span className="min-w-0 overflow-hidden text-ellipsis whitespace-nowrap">{t("mcpMetadata.toolsAndUsage")}</span>
                  </button>
              </nav>
              <Button
                type="button"
                className="absolute right-0 top-[14px]"
                disabled={saving}
                onClick={closeEditor}
              >
                {t("common.close")}
              </Button>
            </div>
            <form
              ref={formRef}
              className="wk-settings-editor my-4 grid gap-[.8rem] max-w-none! [&_label]:grid [&_label]:gap-[.35rem] [&_label]:text-[#27364d] [&_label]:font-semibold [&_input]:w-full [&_input]:box-border [&_input]:border [&_input]:border-[#cbd5e1] [&_input]:rounded-control [&_input]:bg-white [&_input]:text-ink [&_input]:[font:inherit] [&_input]:px-[.65rem] [&_input]:py-[.55rem] [&_textarea]:w-full [&_textarea]:box-border [&_textarea]:border [&_textarea]:border-[#cbd5e1] [&_textarea]:rounded-control [&_textarea]:bg-white [&_textarea]:text-ink [&_textarea]:[font:inherit] [&_textarea]:px-[.65rem] [&_textarea]:py-[.55rem] [&_select]:w-full [&_select]:[font:inherit]"
              onSubmit={(event) => void save(event)}
            >
              {step === 0 ? (
                <>
                  <details className="wk-mcp-code-import">
                    <summary>{t("mcpServiceDialog.codeImport.toggle")}</summary>
                    <p className="wk-muted text-muted">
                      {t("mcpServiceDialog.codeImport.hint")}
                    </p>
                    <Textarea
                      rows={5}
                      value={draft.codeImport}
                      placeholder={'{\n  "mcpServers": { "my-server": { "url": "https://example.com/sse" } }\n}'}
                      onChange={(event) => setField("codeImport", event.target.value)}
                    />
                    <Button
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
                    </Button>
                    {draft.codeImportError ? (
                      <Status tone="error">{draft.codeImportError}</Status>
                    ) : null}
                  </details>
                  <fieldset className="wk-mcp-group rounded-card border border-[#edf0f5] grid gap-[.7rem] p-[.8rem]">
                    <legend>{t("mcpServiceDialog.basicSection")}</legend>
                    <label>
                      {t("mcpServiceDialog.name")}
                      <Input
                        required
                        maxLength={128}
                        value={draft.name}
                        placeholder={t("mcpServiceDialog.namePlaceholder")}
                        onChange={(event) => setField("name", event.target.value)}
                      />
                    </label>
                    <div className="flex flex-wrap items-center gap-[.6rem]">
                      <label className="wk-checkbox flex-none mt-0 whitespace-nowrap">
                        <Checkbox
                          type="checkbox"
                          checked={draft.enabled}
                          onChange={(event) => setField("enabled", event.target.checked)}
                        />{" "}
                        {t("mcpServiceDialog.enableService")}
                      </label>
                      <span className="wk-muted text-muted">
                        {t("mcpServiceDialog.enableServiceDesc")}
                      </span>
                    </div>
                  </fieldset>
                  <fieldset className="wk-mcp-group rounded-card border border-[#edf0f5] grid gap-[.7rem] p-[.8rem]">
                    <legend>{t("mcpServiceDialog.connectionSection")}</legend>
                    <label>
                      {t("mcpServiceDialog.transportType")}
                      <Select
                        className="w-full box-border"
                        value={draft.transportType === "http-streamable" ? "http-streamable" : "sse"}
                        onChange={(event) =>
                          setField(
                            "transportType",
                            event.target.value as Draft["transportType"],
                          )
                        }
                      >
                        <option value="sse">SSE</option>
                        <option value="http-streamable">HTTP Streamable</option>
                      </Select>
                    </label>
                    <label>
                      {t("mcpServiceDialog.serviceUrl")}
                      <Input
                        type="url"
                        required
                        value={draft.url}
                        placeholder={t("mcpServiceDialog.serviceUrlPlaceholder")}
                        onChange={(event) => setField("url", event.target.value)}
                        />
                    </label>
                    <fieldset className="rounded-card border border-[#edf0f5] grid gap-[.7rem] p-[.8rem]">
                      <legend>{t("mcpServiceDialog.customHeaders.label")}</legend>
                      <p className="wk-muted text-muted">{t("mcpServiceDialog.customHeaders.desc")}</p>
                      {draft.headers.map((header, index) => (
                        <div className="wk-mcp-header-row" key={index}>
                          <Input
                            placeholder={t("mcpServiceDialog.customHeaders.keyPlaceholder")}
                            value={header.key}
                            onChange={(event) =>
                              setField(
                                "headers",
                                draft.headers.map((item, itemIndex) =>
                                  itemIndex === index
                                    ? { ...item, key: event.target.value }
                                    : item,
                                ),
                              )
                            }
                        />
                          <Input
                            placeholder={t("mcpServiceDialog.customHeaders.valuePlaceholder")}
                            value={header.value}
                            onChange={(event) =>
                              setField(
                                "headers",
                                draft.headers.map((item, itemIndex) =>
                                  itemIndex === index
                                    ? { ...item, value: event.target.value }
                                    : item,
                                ),
                              )
                            }
                          />
                          <Button
                            type="button"
                            onClick={() =>
                              setField(
                                "headers",
                                draft.headers.filter(
                                  (_, itemIndex) => itemIndex !== index,
                                ),
                              )
                            }
                          >
                            {t("common.delete")}
                          </Button>
                        </div>
                      ))}
                      <Button
                        type="button"
                        onClick={() =>
                          setField("headers", [
                            ...draft.headers,
                            { key: "", value: "" },
                          ])
                        }
                      >
                        {t("mcpServiceDialog.customHeaders.add")}
                      </Button>
                    </fieldset>
                  </fieldset>
                  <fieldset className="wk-mcp-group rounded-card border border-[#edf0f5] grid gap-[.7rem] p-[.8rem]">
                    <legend>{t("mcpServiceDialog.authConfig")}</legend>
                    <label>
                      {t("mcpServiceDialog.authType")}
                      <Select
                        className="w-full box-border"
                        value={draft.authType}
                        onChange={(event) =>
                          setField("authType", event.target.value as Draft["authType"])
                        }
                      >
                        <option value="">{t("mcpServiceDialog.authTypeNone")}</option>
                        <option value="api_key">{t("mcpServiceDialog.authTypeApiKey")}</option>
                        <option value="oauth">{t("mcpServiceDialog.authTypeOAuth")}</option>
                      </Select>
                    </label>
                    {draft.authType === "oauth" ? (
                      <>
                        <label>
                          {t("mcpServiceDialog.oauthScopes")}
                          <Input
                            value={draft.oauthScopes}
                            placeholder={t("mcpServiceDialog.optional")}
                            onChange={(event) =>
                              setField("oauthScopes", event.target.value)
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
                          <Input
                            value={draft.apiKeyHeader}
                            placeholder="X-API-Key"
                            onChange={(event) =>
                              setField("apiKeyHeader", event.target.value)
                            }
                          />
                        </label>
                        <p className="wk-muted text-muted">{t("mcpServiceDialog.apiKeyHeaderDesc")}</p>
                        {draft.id ? <div className="wk-mcp-credential-card grid gap-[.65rem] rounded-[7px] border border-[#dce3ed] bg-[#f7f9fc] p-3"><div className="flex items-center justify-between gap-3"><strong>{t("mcpServiceDialog.credentialValue")}</strong><span className={`text-[.8rem] text-[#66758b] ${draft.credentialConfigured ? "text-[#16845b]! font-semibold" : ""}`}>{draft.credentialConfigured ? "✓ " + t("common.success") : t("mcpServiceDialog.optional")}</span></div><label>{draft.credentialConfigured ? t("common.replaceValue") : t("mcpServiceDialog.credentialValue")}<Input type="password" autoComplete="new-password" value={draft.apiKey} placeholder={t("mcpServiceDialog.optional")} onChange={(event) => setField("apiKey", event.target.value)} /></label>{draft.credentialConfigured ? <Button type="button" disabled={saving} onClick={() => void clearMcpCredential()}>{t("common.delete")}</Button> : null}</div> : <label>{t("mcpServiceDialog.credentialValue")}<Input type="password" autoComplete="new-password" value={draft.apiKey} placeholder={t("mcpServiceDialog.optional")} onChange={(event) => setField("apiKey", event.target.value)} /></label>}
                      </>
                    ) : null}
                  </fieldset>
                  <fieldset className="wk-mcp-group rounded-card border border-[#edf0f5] grid gap-[.7rem] p-[.8rem]">
                    <legend>{t("mcpServiceDialog.advancedConfig")}</legend>
                    <label>
                      {t("mcpServiceDialog.timeoutSec")}
                      <div className="relative">
                        <Input
                          className="pr-9"
                          type="number"
                          min={1}
                          max={300}
                          value={draft.timeout}
                          onChange={(event) =>
                            setField(
                              "timeout",
                              Math.min(
                                300,
                                Math.max(1, Number(event.target.value) || 30),
                              ),
                            )
                          }
                        />
                        <span className="pointer-events-none absolute inset-y-0 right-3 inline-flex items-center text-[12px] text-[#8a96a8]">{t("mcpServiceDialog.unitSecond")}</span>
                      </div>
                    </label>
                    <label>
                      {t("mcpServiceDialog.retryCount")}
                      <div className="relative">
                        <Input
                          className="pr-9"
                          type="number"
                          min={0}
                          max={10}
                          value={draft.retryCount}
                          onChange={(event) =>
                            setField(
                              "retryCount",
                              Math.min(
                                10,
                                Math.max(0, Number(event.target.value) || 0),
                              ),
                            )
                          }
                        />
                        <span className="pointer-events-none absolute inset-y-0 right-3 inline-flex items-center text-[12px] text-[#8a96a8]">{t("mcpServiceDialog.unitTimes")}</span>
                      </div>
                    </label>
                    <label>
                      {t("mcpServiceDialog.retryDelaySec")}
                      <div className="relative">
                        <Input
                          className="pr-9"
                          type="number"
                          min={0}
                          max={60}
                          value={draft.retryDelay}
                          onChange={(event) =>
                            setField(
                              "retryDelay",
                              Math.min(
                                60,
                                Math.max(0, Number(event.target.value) || 0),
                              ),
                            )
                          }
                        />
                        <span className="pointer-events-none absolute inset-y-0 right-3 inline-flex items-center text-[12px] text-[#8a96a8]">{t("mcpServiceDialog.unitSecond")}</span>
                      </div>
                    </label>
                  </fieldset>
                </>
              ) : (
                <>
                  <fieldset className="wk-mcp-group rounded-card border border-[#edf0f5] grid gap-[.7rem] p-[.8rem]">
                    <div className="wk-settings-panel-heading flex items-start justify-between gap-4 border-b border-[#eef1f5] pb-4 mb-4 max-[720px]:flex-col sticky top-0 z-[1] bg-white pt-[.25rem]">
                      <div>
                        <h4>{t("mcpMetadata.usage")}</h4>
                        <p className="wk-muted text-muted m-0">{t("mcpMetadata.usageHint")}</p>
                      </div>
                    </div>
                    <div className="flex items-center justify-between gap-[.8rem]">
                      <label className="wk-form-label text-ink font-semibold">
                        {t("mcpMetadata.usageInstructions")}
                      </label>
                      <Button
                        type="button"
                        disabled={
                          !toolsSynced || metadataBusy || saving || generatingUsage
                        }
                        loading={generatingUsage}
                        onClick={() => void generateUsage()}
                      >
                        {t("mcpMetadata.generateUsage")}
                      </Button>
                    </div>
                    <Textarea
                      required
                      rows={5}
                      maxLength={16000}
                      value={draft.usageInstructions}
                      placeholder={t("mcpMetadata.instructionsPlaceholder")}
                      onChange={(event) =>
                        setField("usageInstructions", event.target.value)
                      }
                    />
                    <span className="wk-muted text-muted block text-right text-[.78rem]">
                      {draft.usageInstructions.length}/16000
                    </span>
                    <p className="wk-muted text-muted">{t("mcpMetadata.generateHint")}</p>
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
            <div className="wk-mcp-footer sticky bottom-0 -mx-[18px] mt-[1.2rem] flex min-h-[53px] shrink-0 box-border items-center justify-between gap-3 border-t border-[#edf0f5] bg-white px-[18px] pt-[.6rem] pb-[.2rem]">
                <div className="flex flex-1">
                  {step === 1 ? (
                    <Button
                      type="button"
                      disabled={dialogBusy}
                      onClick={() => setStep(0)}
                    >
                      {t("mcpMetadata.previous")}
                    </Button>
                  ) : null}
                </div>
                <div className="flex gap-[.6rem]">
                  <Button
                    type="button"
                    disabled={saving}
                    onClick={closeEditor}
                  >
                    {t("common.cancel")}
                  </Button>
                  <Button
                    type="submit"
                    loading={saving}
                    disabled={
                      metadataBusy ||
                      generatingUsage ||
                      (step === 1 && !toolsSynced)
                    }
                  >
                    {step === 0 ? t("mcpMetadata.saveNext") : t("common.save")}
                  </Button>
                </div>
              </div>
            </form>
          </div>
        </div>
      ) : null}
    </section>
  );
}
