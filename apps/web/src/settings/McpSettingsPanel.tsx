import { useEffect, useRef, useState } from "react";
import * as React from "react";
import type { McpConfiguration, WeKnoraClient } from "@weknora/api-client";
import { Button, Card, Status } from "@weknora/ui";
import { McpTestResultBody } from "./McpTestResultBody.tsx";
import { McpToolsDirectory } from "./McpToolsDirectory.tsx";
import { createTranslator, useAppLocale } from "../i18n.ts";

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
    transportType:
      service?.transport_type === "http-streamable"
        ? "http-streamable"
        : service?.transport_type === "stdio"
          ? "stdio"
          : "sse",
    enabled: service?.enabled !== false,
    authType,
    apiKeyHeader:
      typeof authConfig.api_key_header === "string"
        ? authConfig.api_key_header
        : "",
    apiKey: "",
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

function McpServiceDetails({
  client,
  serviceId,
  oauthEnabled,
  usageInstructions = "",
}: {
  client: WeKnoraClient;
  serviceId: string;
  oauthEnabled: boolean;
  usageInstructions?: string;
}) {
  const t = createTranslator(useAppLocale());
  const [metadata, setMetadata] =
    useState<
      Awaited<
        ReturnType<WeKnoraClient["configuration"]["mcp"]["metadata"]["get"]>
      >
    >(null);
  const [approvals, setApprovals] = useState<
    Awaited<
      ReturnType<WeKnoraClient["configuration"]["mcp"]["toolApprovals"]["list"]>
    >
  >([]);
  const [policyError, setPolicyError] = useState<string | null>(null);
  const [testResult, setTestResult] = useState<Awaited<
    ReturnType<WeKnoraClient["configuration"]["mcp"]["test"]>
  > | null>(null);
  const [oauth, setOAuth] = useState<Awaited<
    ReturnType<WeKnoraClient["configuration"]["mcp"]["oauth"]["status"]>
  > | null>(null);
  const [usage, setUsage] = useState(usageInstructions);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const loadGeneration = useRef(0);
  async function load() {
    const generation = ++loadGeneration.current;
    setBusy(true);
    setError(null);
    setPolicyError(null);
    const [metadataResult, approvalsResult] = await Promise.allSettled([
      client.configuration.mcp.metadata.get(serviceId),
      client.configuration.mcp.toolApprovals.list(serviceId),
    ]);
    if (generation !== loadGeneration.current) return;
    if (metadataResult.status === "fulfilled")
      setMetadata(metadataResult.value);
    if (approvalsResult.status === "fulfilled")
      setApprovals(approvalsResult.value);
    else {
      setApprovals([]);
      setPolicyError(
        approvalsResult.reason instanceof Error
          ? approvalsResult.reason.message
          : "Unable to load MCP tool policies",
      );
    }
    const failure =
      metadataResult.status === "rejected" ? metadataResult.reason : null;
    if (failure)
      setError(
          failure instanceof Error ? failure.message : t("mcpMetadata.failed"),
      );
    if (oauthEnabled) {
      try {
        setOAuth(await client.configuration.mcp.oauth.status(serviceId));
      } catch (cause) {
        if (generation === loadGeneration.current)
          setError(
          cause instanceof Error ? cause.message : t("mcpMetadata.failed"),
          );
      }
    }
    if (generation === loadGeneration.current) setBusy(false);
  }
  useEffect(() => {
    void load();
    return () => {
      loadGeneration.current += 1;
    };
  }, [client, serviceId, oauthEnabled]);
  async function refresh() {
    setBusy(true);
    setError(null);
    try {
      setMetadata(await client.configuration.mcp.metadata.refresh(serviceId));
    } catch (cause) {
      setError(
        cause instanceof Error ? cause.message : t("mcpMetadata.failed"),
      );
    } finally {
      setBusy(false);
    }
  }
  async function testConnection() {
    setBusy(true);
    setError(null);
    try {
      setTestResult(await client.configuration.mcp.test(serviceId));
    } catch (cause) {
      setError(
        cause instanceof Error ? cause.message : t("mcp.testResult.connectionFailed"),
      );
    } finally {
      setBusy(false);
    }
  }
  async function generateUsage() {
    if (!metadata || metadata.stale || busy) return;
    setBusy(true);
    setError(null);
    try {
      const generated =
        await client.configuration.mcp.usageInstructions.generate(
          serviceId,
          "zh-CN",
        );
      await client.configuration.mcp.update(serviceId, {
        usage_instructions: generated,
      });
      setUsage(generated);
    } catch (cause) {
      setError(
        cause instanceof Error ? cause.message : t("mcpMetadata.generateFailed"),
      );
    } finally {
      setBusy(false);
    }
  }
  async function updateTool(
    toolName: string,
    field: "enabled" | "requireApproval",
    value: boolean,
  ) {
    if (busy || metadata?.stale || policyError) return;
    setBusy(true);
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
            id: `${serviceId}:${toolName}`,
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
    }
  }
  async function authorize() {
    if (typeof window === "undefined" || busy) return;
    setBusy(true);
    setError(null);
    try {
      const result = await client.configuration.mcp.oauth.authorizeUrl(
        serviceId,
        {
          redirectURI: `${window.location.origin}/api/v1/mcp-oauth/callback`,
          frontendRedirect: window.location.href,
        },
      );
      const popup = window.open(
        result.authorizationUrl,
        "weknora_mcp_oauth",
        "width=600,height=720",
      );
      if (!popup) throw new Error("OAuth popup was blocked");
      for (let attempt = 0; attempt < 60; attempt += 1) {
        await new Promise((resolve) => window.setTimeout(resolve, 500));
        const next = await client.configuration.mcp.oauth.status(
          serviceId,
          result.authorizationAttempt,
        );
        if (next.authorized) {
          setOAuth(next);
          break;
        }
        if (popup.closed) break;
      }
    } catch (cause) {
      setError(
        cause instanceof Error ? cause.message : t("mcpServiceDialog.oauthAuthorizeHint"),
      );
    } finally {
      setBusy(false);
    }
  }
  async function revoke() {
    if (busy) return;
    setBusy(true);
    setError(null);
    try {
      await client.configuration.mcp.oauth.revoke(serviceId);
      setOAuth({
        authorized: false,
        state: "reauth_required",
        refreshAvailable: false,
      });
    } catch (cause) {
      setError(
        cause instanceof Error ? cause.message : t("mcpMetadata.failed"),
      );
    } finally {
      setBusy(false);
    }
  }
  return (
    <section className="wk-mcp-details">
      <div className="wk-settings-panel-heading">
        <div>
          <h4>{t("mcpMetadata.toolsAndUsage")}</h4>
          <p className="wk-muted">
            {t("mcpMetadata.cacheHint")}
          </p>
        </div>
        <div className="wk-list-actions">
          <Button type="button" disabled={busy} onClick={() => void refresh()}>
            {t("common.refresh")}
          </Button>
          <Button
            type="button"
            disabled={busy}
            onClick={() => void testConnection()}
          >
            {t("mcpServiceDialog.testConnection")}
          </Button>
        </div>
      </div>
      {error ? <Status tone="error">{error}</Status> : null}
      {oauthEnabled ? (
        <div className="wk-mcp-oauth">
          <strong>{t("mcpServiceDialog.authTypeOAuth")}</strong>
          <span>{oauth?.state ?? "checking"}</span>
          <Button
            type="button"
            disabled={busy}
            onClick={() => void authorize()}
          >
            {oauth?.authorized ? t("mcpServiceDialog.oauthReauthorize") : t("mcpServiceDialog.oauthAuthorize")}
          </Button>
          {oauth?.authorized || oauth?.state === "refreshable" ? (
            <Button type="button" disabled={busy} onClick={() => void revoke()}>
            {t("mcpServiceDialog.oauthRevoke")}
            </Button>
          ) : null}
        </div>
      ) : null}
      {metadata ? (
        <>
          <p className={metadata.stale ? "wk-mcp-stale" : "wk-muted"}>
            {t("mcpMetadata.toolCount", { count: metadata.tools.length })} · {metadata.serverName}{" "}
            {metadata.serverVersion} ·{" "}
            {metadata.stale ? t("mcpMetadata.stale") : `${t("mcpMetadata.syncedAt")}${metadata.syncedAt}`}
          </p>
          <McpToolsDirectory
            tools={metadata.tools}
            serviceId={serviceId}
            approvals={approvals}
            busy={busy || metadata.stale}
            policyError={policyError}
            onRetryPolicies={() => void load()}
            onPolicyChange={(name, field, value) =>
              void updateTool(name, field, value)
            }
          />
          <div className="wk-mcp-usage">
            <Button
              type="button"
              disabled={busy || metadata.stale || !metadata}
              onClick={() => void generateUsage()}
            >
              {t("mcpMetadata.generateUsage")}
            </Button>
            {usage ? (
              <pre>{usage}</pre>
            ) : metadata.instructions ? (
              <pre>{metadata.instructions}</pre>
            ) : (
              <p className="wk-muted">{t("mcpMetadata.instructionsPlaceholder")}</p>
            )}
          </div>
        </>
      ) : (
        <Status>
          {busy ? t("mcpMetadata.fetching") : t("mcpMetadata.notSynced")}
        </Status>
      )}
      <McpTestResultBody result={testResult} approvals={approvals} busy={busy || Boolean(metadata?.stale) || Boolean(policyError)} onPolicyChange={(name, field, value) => void updateTool(name, field, value)} />
    </section>
  );
}

export function McpSettingsPanel({ client, role, initialServices }: Props) {
  const t = createTranslator(useAppLocale());
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
  }, [client, initialServices]);
  function setField<K extends keyof Draft>(key: K, value: Draft[K]) {
    setDraft((current) => (current ? { ...current, [key]: value } : current));
  }
  async function save(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!draft || saving) return;
    const validation = validateMcpDraft(draft, step);
    if (validation) {
      const messageKey: Record<McpDraftValidationError, string> = {
        nameRequired: "mcpServiceDialog.rules.nameRequired",
        urlRequired: "mcpServiceDialog.rules.urlRequired",
        urlInvalid: "mcpServiceDialog.rules.urlInvalid",
        stdioUnsupported: "mcpServiceDialog.codeImport.errors.stdioUnsupported",
        usageRequired: "mcpMetadata.instructionsRequired",
      };
      setError(t(messageKey[validation]));
      return;
    }
    setSaving(true);
    setError(null);
    setNotice(null);
    try {
      if (step === 1) {
        const instructions = draft.usageInstructions.trim();
        if (!draft.id) throw new Error("Save the connection before editing usage instructions");
        if (!instructions) throw new Error("Usage instructions are required");
        await client.configuration.mcp.update(draft.id, { usage_instructions: instructions });
        setDraft(null);
        setNotice("MCP service saved.");
        await load();
        return;
      }
      const headers = Object.fromEntries(
        draft.headers
          .map(({ key, value }) => [key.trim(), value.trim()])
          .filter(([key, value]) => key && value),
      );
      const payload: Record<string, unknown> = {
        name: draft.name.trim(),
        description: draft.description.trim(),
        usage_instructions: draft.usageInstructions.trim(),
        url: draft.url.trim(),
        transport_type: draft.transportType,
        enabled: draft.enabled,
        headers,
        advanced_config: {
          timeout: draft.timeout,
          retry_count: draft.retryCount,
          retry_delay: draft.retryDelay,
        },
        auth_config: {
          ...draft.authConfig,
          auth_type: draft.authType,
          ...(draft.apiKeyHeader.trim()
            ? { api_key_header: draft.apiKeyHeader.trim() }
            : {}),
          ...(draft.authType === "oauth"
            ? { scopes: draft.oauthScopes.split(/[\s,]+/).filter(Boolean) }
            : {}),
        },
      };
      const saved = draft.id
        ? await client.configuration.mcp.update(draft.id, payload)
        : await client.configuration.mcp.create(payload);
      if (draft.apiKey.trim()) {
        try {
          await client.configuration.mcp.credentials.put(saved.id, {
            apiKey: draft.apiKey.trim(),
          });
        } catch (cause) {
          setDraft({ ...draft, id: saved.id });
          throw cause;
        }
      }
      setDraft((current) => current ? { ...current, id: saved.id } : current);
      setStep(1);
      setNotice("Connection saved. Sync tools and complete usage instructions.");
      await load();
    } catch (cause) {
      setError(
        cause instanceof Error ? cause.message : t("mcpServiceDialog.toasts.updateFailed"),
      );
    } finally {
      setSaving(false);
    }
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
      setNotice(enabled ? "MCP service enabled." : "MCP service disabled.");
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
      !window.confirm(`Delete MCP service “${service.name}”?`)
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
      setNotice("MCP service deleted.");
    } catch (cause) {
      setError(
        cause instanceof Error ? cause.message : t("mcpSettings.toasts.deleteFailed"),
      );
    } finally {
      setBusyId(null);
    }
  }
  if (loading)
    return (
      <Card data-testid="mcp-settings">
        <Status>{t("common.loading")}</Status>
      </Card>
    );
  return (
    <section className="wk-mcp-settings" data-testid="mcp-settings">
      <div className="wk-settings-panel-heading">
        <div>
          <h3>{t("mcpSettings.title")}</h3>
          <p className="wk-muted">
            {t("mcpSettings.description")}
          </p>
        </div>
        {canEdit ? (
          <Button type="button" onClick={() => { setDraft(draftFrom()); setStep(0); }}>
            {t("mcpSettings.addService")}
          </Button>
        ) : null}
      </div>
      {error ? <Status tone="error">{error}</Status> : null}
      {notice ? <Status tone="success">{notice}</Status> : null}
      {services.length === 0 ? (
        <Status>{t("mcpSettings.empty")}</Status>
      ) : (
        <div className="wk-mcp-grid">
          {services.map((service) => (
            <Card key={service.id} className="wk-mcp-card">
              <div className="wk-mcp-card-header">
                <div>
                  <h4 title={service.name}>{service.name}</h4>
                  {service.is_builtin ? (
                    <span className="wk-mcp-badge">{t("mcpSettings.builtin")}</span>
                  ) : null}
                </div>
                {canEdit ? (
                  <div className="wk-list-actions">
                    <Button
                      type="button"
                      onClick={() => { setDraft(draftFrom(service)); setStep(0); }}
                    >
                      {t("common.edit")}
                    </Button>
                    {service.is_builtin ? null : (
                      <Button
                        type="button"
                        onClick={() => void remove(service)}
                      >
                        {t("common.delete")}
                      </Button>
                    )}
                  </div>
                ) : null}
              </div>
              <p className="wk-muted wk-mcp-description">
                {serviceDescription(service) || t("mcpSettings.noUsageInstructions")}
              </p>
              <div className="wk-mcp-card-footer">
                <span>
                  {service.transport_type === "http-streamable"
                      ? "HTTP Streamable"
                    : service.transport_type === "stdio"
                      ? "stdio (unsupported editor)"
                      : "SSE"}
                </span>
                {canEdit && !service.is_builtin ? (
                  <Button
                    type="button"
                    disabled={busyId === service.id}
                    onClick={() => void toggle(service)}
                  >
                    {service.enabled === false ? t("mcpSettings.disabled") : t("mcpSettings.enabled")}
                  </Button>
                ) : (
                  <span>{service.enabled === false ? t("mcpSettings.disabled") : t("mcpSettings.enabled")}</span>
                )}
              </div>
            </Card>
          ))}
        </div>
      )}
      {draft ? (
        <div
          className="wks-overlay wk-mcp-overlay"
          data-testid="mcp-editor-overlay"
        >
          <div
            className="wks-modal wks-mcp-drawer"
            role="dialog"
            aria-modal="true"
            aria-label={draft.id ? t("mcpServiceDialog.editTitle") : t("mcpServiceDialog.addTitle")}
          >
            <div className="wk-settings-panel-heading">
              <div>
                <h3>{draft.id ? t("mcpServiceDialog.editTitle") : t("mcpServiceDialog.addTitle")}</h3>
                <p className="wk-muted">
                  {t("mcpMetadata.connection")} · {t("mcpMetadata.cacheHint")}
                </p>
              </div>
              <Button
                type="button"
                disabled={saving}
                onClick={() => { setDraft(null); setStep(0); }}
              >
                {t("common.close")}
              </Button>
            </div>
            <div className="wk-mcp-steps" aria-label={t("mcpMetadata.setupProgress")}><span className={step === 0 ? "is-active" : "is-done"}>1. {t("mcpMetadata.connection")}</span><span aria-hidden="true"> → </span><span className={step === 1 ? "is-active" : ""}>2. {t("mcpMetadata.toolsAndUsage")}</span></div>
            <form
              className="wk-settings-editor"
              onSubmit={(event) => void save(event)}
            >
            <details className="wk-mcp-code-import">
              <summary>Import MCP JSON</summary>
              <p className="wk-muted">
                Paste a standard mcpServers JSON object; it fills the form
                without saving.
              </p>
              <textarea
                rows={5}
                value={draft.codeImport}
                placeholder={
                  '{\n  "mcpServers": { "my-server": { "url": "https://example.com/sse" } }\n}'
                }
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
                Parse JSON
              </Button>
              {draft.codeImportError ? (
                <Status tone="error">{draft.codeImportError}</Status>
              ) : null}
            </details>
            <label>
              Name
              <input
                required
                maxLength={128}
                value={draft.name}
                onChange={(event) => setField("name", event.target.value)}
              />
            </label>
            <label>
              Description
              <textarea
                rows={3}
                value={draft.description}
                onChange={(event) =>
                  setField("description", event.target.value)
                }
              />
            </label>
            <label>
              Transport
              <select
                value={draft.transportType}
                onChange={(event) =>
                  setField(
                    "transportType",
                    event.target.value as Draft["transportType"],
                  )
                }
              >
                <option value="sse">SSE</option>
                <option value="http-streamable">HTTP Streamable</option>
                <option value="stdio">stdio (read-only)</option>
              </select>
            </label>
            <label>
              Service URL
              <input
                type="url"
                required={draft.transportType !== "stdio"}
                value={draft.url}
                onChange={(event) => setField("url", event.target.value)}
              />
            </label>
            {draft.transportType === "stdio" ? (
              <Status tone="warning">
                stdio is preserved for display but cannot be edited by this
                remote-service form.
              </Status>
            ) : null}
            <fieldset>
              <legend>Custom headers</legend>
              {draft.headers.map((header, index) => (
                <div className="wk-mcp-header-row" key={index}>
                  <input
                    placeholder="Header name"
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
                  <input
                    placeholder="Header value"
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
                    Remove
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
                Add header
              </Button>
            </fieldset>
            <label>
              Authentication
              <select
                value={draft.authType}
                onChange={(event) =>
                  setField("authType", event.target.value as Draft["authType"])
                }
              >
                <option value="">None</option>
                <option value="api_key">API key / token</option>
                <option value="oauth">OAuth 2.0</option>
              </select>
            </label>
            {draft.authType === "oauth" ? (
              <label>
                OAuth scopes
                <input
                  value={draft.oauthScopes}
                  placeholder="openid profile"
                  onChange={(event) =>
                    setField("oauthScopes", event.target.value)
                  }
                />
              </label>
            ) : null}
            {draft.authType === "api_key" ? (
              <>
                <label>
                  API key header
                  <input
                    value={draft.apiKeyHeader}
                    placeholder="X-API-Key"
                    onChange={(event) =>
                      setField("apiKeyHeader", event.target.value)
                    }
                  />
                </label>
                <label>
                  API key
                  <input
                    type="password"
                    autoComplete="new-password"
                    value={draft.apiKey}
                    placeholder={
                      draft.id
                        ? "Leave blank to keep configured key"
                        : "Optional"
                    }
                    onChange={(event) => setField("apiKey", event.target.value)}
                  />
                </label>
              </>
            ) : null}
            <fieldset>
              <legend>Advanced configuration</legend>
              <label>
                Timeout (seconds)
                <input
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
              </label>
              <label>
                Retry count
                <input
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
              </label>
              <label>
                Retry delay (seconds)
                <input
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
              </label>
            </fieldset>
            <label className="wk-checkbox">
              <input
                type="checkbox"
                checked={draft.enabled}
                onChange={(event) => setField("enabled", event.target.checked)}
              />{" "}
              Enabled
            </label>
            {step === 1 ? <label>
              Usage instructions
              <textarea required rows={5} value={draft.usageInstructions} placeholder="Explain when and how this MCP service should be used." onChange={(event) => setField("usageInstructions", event.target.value)} />
            </label> : null}
            <div className="wk-list-actions">
              {step === 1 ? <Button type="button" disabled={saving} onClick={() => setStep(0)}>Previous</Button> : null}
              <Button type="submit" loading={saving}>
                {step === 0 ? "Save and continue" : "Save"}
              </Button>
              <Button
                type="button"
                disabled={saving}
                onClick={() => { setDraft(null); setStep(0); }}
              >
                Cancel
              </Button>
            </div>
            </form>
            {draft.id ? (
              <McpServiceDetails
                client={client}
                serviceId={draft.id}
                oauthEnabled={draft.authType === "oauth"}
                usageInstructions={draft.usageInstructions}
              />
            ) : null}
          </div>
        </div>
      ) : null}
    </section>
  );
}
