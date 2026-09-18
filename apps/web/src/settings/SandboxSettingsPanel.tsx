import { useCallback, useEffect, useMemo, useRef, useState, type KeyboardEvent } from 'react';
import './sandbox-settings.css';
import {
  parseSandboxConfigurationConflict,
  type CubeSandboxConfig,
  type DockerSandboxConfig,
  type E2BSandboxConfig,
  type SandboxBackendType,
  type SandboxConfigRecord,
  type SandboxConfigUpsert,
  type SandboxInventory,
  type TenantSandboxConfig,
  type WeKnoraClient,
} from '@weknora/api-client';
import { formatMessage, type Locale } from '@weknora/i18n';
import { Button, Card, Checkbox, Input, NumberInput, Radio, Select, Status } from '@weknora/ui';
import { roleAtLeast, type SettingsRole } from '../../../../packages/views/src/settings/registry.ts';
import { useAppLocale } from '../i18n.ts';

/*
 * React port of frontend/src/views/settings/SandboxSettings.vue and
 * frontend/src/components/SandboxConfigEditorDrawer.vue (wizard steps,
 * template catalog, deep check, field validation, localization). Vue
 * file:line references in comments point at the authoritative behavior.
 */

export type SandboxStepKey = 'connection' | 'template' | 'runtime';

/** frontend/src/api/system.ts SandboxTemplate (masked to what the drawer renders). */
export interface SandboxTemplateItem {
  id: string;
  name?: string;
  status?: string;
  version?: string;
  image?: string;
  created_at?: string;
  standard: boolean;
  error?: string;
  instance_type?: string;
  network_type?: string;
  allow_internet_access?: boolean;
}

/** frontend/src/api/system.ts SandboxCheckItem / SandboxCheckResult. */
export interface SandboxCheckItemView { name: string; ok: boolean | null; message?: string; reason?: string; latency_ms?: number }
export interface SandboxCheckResultView { ok: boolean; provider?: string; checks: SandboxCheckItemView[] }

export interface SandboxEnvRow { key: string; value: string; stored?: boolean }
export interface SandboxInjectRow { header: string; secret: string; format: string; stored?: boolean; originalRuleName?: string; originalHeader?: string }
export interface SandboxCubeRuleRow {
  key: string; name: string; scheme: string; sni: string; host: string;
  methodsText: string; path: string; deny: boolean; audit: string; inject: SandboxInjectRow[];
}
export interface SandboxE2BHeaderRow { name: string; value: string; stored?: boolean; originalHost?: string; originalName?: string }
export interface SandboxE2BHostRuleRow { host: string; headers: SandboxE2BHeaderRow[] }

export interface SandboxEditorForm {
  name: string;
  description: string;
  backend: SandboxBackendType;
  defaultTimeoutSec?: number;
  terminalIdleDisconnectSec?: number;
  allowPrivateEndpoints: boolean;
  cube: CubeSandboxConfig;
  e2b: E2BSandboxConfig;
  docker: DockerSandboxConfig;
  storedCubeKey: boolean;
  storedE2BKey: boolean;
  envRows: SandboxEnvRow[];
  skillRollout: 'next_turn' | 'new_session';
  denyEgressByDefault: boolean;
  allowOutRows: string[];
  denyOutRows: string[];
  cubeRules: SandboxCubeRuleRow[];
  e2bHostRules: SandboxE2BHostRuleRow[];
}

export interface CubeEgressRulePayload {
  name: string; scheme?: string; sni?: string; host?: string; methods?: string[];
  path?: string; deny?: boolean; audit?: string;
  inject?: Array<{ header: string; secret?: string; format?: string }>;
}

export interface SandboxNetworkPolicyPayload {
  deny_egress_by_default?: boolean;
  allow_out?: string[];
  deny_out?: string[];
  cube_rules?: CubeEgressRulePayload[];
  e2b_host_rules?: Array<{ host: string; headers: Record<string, string> }>;
}

// --- Constants (SandboxConfigEditorDrawer.vue:790-799, 1064-1068) ---

/** The backend echoes stored secrets as this placeholder (drawer.vue:790). */
export const SECRET_PLACEHOLDER = '***';
/** Mirrors DefaultDockerImage on the server (drawer.vue:796). */
export const DEFAULT_DOCKER_IMAGE = 'wechatopenai/weknora-sandbox:main';
export const CLUSTER_GUIDE_URL = 'https://github.com/Tencent/WeKnora/blob/main/docs/sandbox-cluster.md';
export const E2B_API_KEYS_URL = 'https://e2b.dev/dashboard?tab=keys';

export const SANDBOX_BACKENDS: readonly SandboxBackendType[] = ['cube', 'e2b', 'docker'];

/** Mirrors sandbox.MissingRequiredFields on the server (drawer.vue:1064-1068). */
export const REQUIRED_FIELDS: Record<SandboxBackendType, readonly string[]> = {
  cube: ['api_url', 'proxy_url', 'sandbox_domain', 'template_id'],
  e2b: ['api_key', 'template_id'],
  docker: ['image'],
};

const REMOTE_BACKENDS = new Set(['cube', 'e2b']);
/** Probes the shallow check defers to the deep check (drawer.vue:1028). */
const PENDING_SKIP_REASON = 'needs_deep_check';
const EGRESS_RESTRICTED_REASON = 'egress_restricted_by_policy';

export const isNamedSandboxBackend = (type: string): boolean => (SANDBOX_BACKENDS as readonly string[]).includes(type);
export const isRemoteSandboxBackend = (type: string): boolean => REMOTE_BACKENDS.has(type);

// --- Wizard steps (drawer.vue:953-1006) ---

/** connection → template (remote only) → runtime (drawer.vue:953-962). */
export function wizardStepKeys(backend: string): SandboxStepKey[] {
  const steps: SandboxStepKey[] = ['connection'];
  if (REMOTE_BACKENDS.has(backend)) steps.push('template');
  steps.push('runtime');
  return steps;
}

/**
 * Jumping is what separates editing from creating: a config that does not
 * exist yet has to be built in order, while visited steps stay clickable as a
 * way back (drawer.vue:1003-1006).
 */
export function canJumpToStep(index: number, current: number, hasRecord: boolean): boolean {
  if (index === current) return false;
  return hasRecord || index < current;
}

/** Deep check waits for the fields it actually probes (drawer.vue:973-977). */
export function canDeepCheckOnStep(step: SandboxStepKey, backend: string): boolean {
  if (step === 'template') return true;
  return step === 'connection' && !REMOTE_BACKENDS.has(backend);
}

export function primaryTextKey(step: SandboxStepKey): string {
  if (step === 'connection') return 'settings.sandbox.connectAndContinue';
  if (step === 'template') return 'common.next';
  return 'common.save';
}

// --- Validation (drawer.vue:1075-1109, 1234-1248) ---

const isMaskedSecret = (value?: string): boolean => value === SECRET_PLACEHOLDER;

/** Re-attaches the redaction placeholder to a blank-but-stored secret (drawer.vue:1452-1455). */
function withStoredSecret<T extends { api_key?: string }>(block: T, stored: boolean): T {
  if (stored && !block.api_key?.trim()) block.api_key = SECRET_PLACEHOLDER;
  return block;
}

/** Snapshot of the active backend block as it will be submitted (drawer.vue:1091-1095). */
export function submittedBackendValues(form: SandboxEditorForm): Record<string, unknown> {
  if (form.backend === 'cube') return { ...withStoredSecret({ ...form.cube }, form.storedCubeKey) };
  if (form.backend === 'e2b') return { ...withStoredSecret({ ...form.e2b }, form.storedE2BKey) };
  return { ...form.docker };
}

/** Fields whose submitted value is empty (drawer.vue:1097-1109). */
export function missingRequiredFields(form: SandboxEditorForm, includeTemplate = true): string[] {
  const required = (REQUIRED_FIELDS[form.backend] ?? []).filter((field) => includeTemplate || field !== 'template_id');
  const values = submittedBackendValues(form);
  return required.filter((field) => {
    const value = values[field];
    return typeof value !== 'string' || value.trim() === '';
  });
}

/** Connection-step gate before templates are fetched (drawer.vue:1234-1248). */
export function connectionMissingFields(form: SandboxEditorForm): string[] {
  if (!REMOTE_BACKENDS.has(form.backend)) return [];
  const required = form.backend === 'cube' ? ['api_url', 'proxy_url', 'sandbox_domain'] : ['api_key'];
  const values = submittedBackendValues(form);
  return required.filter((field) => typeof values[field] !== 'string' || String(values[field]).trim() === '');
}

// --- Template status semantics (drawer.vue:1329-1374) ---

export function isTemplateSelectable(item: SandboxTemplateItem): boolean {
  const status = item.status?.trim().toLowerCase();
  if (!status) return true;
  return ['ready', 'available', 'complete', 'completed', 'success', 'succeeded'].includes(status);
}

export function isTemplatePending(item: SandboxTemplateItem): boolean {
  const status = item.status?.trim().toLowerCase();
  return ['building', 'waiting', 'pending', 'queued', 'processing', 'running'].includes(status ?? '');
}

export function isTemplateUntagged(item: SandboxTemplateItem): boolean {
  return item.status?.trim().toLowerCase() === 'untagged';
}

export function isTemplateFailed(item: SandboxTemplateItem): boolean {
  const status = item.status?.trim().toLowerCase();
  return ['failed', 'failure', 'error', 'cancelled', 'canceled'].includes(status ?? '');
}

export type TemplateStatusKey = 'ready' | 'building' | 'untagged' | 'failed' | 'unknown';

export function templateStatusKey(item: SandboxTemplateItem): TemplateStatusKey {
  if (isTemplateSelectable(item)) return 'ready';
  if (isTemplateUntagged(item)) return 'untagged';
  if (isTemplatePending(item)) return 'building';
  if (isTemplateFailed(item)) return 'failed';
  return 'unknown';
}

/** Display name: an empty name or one equal to the id reads as unnamed (drawer.vue:1272-1277). */
export function templateDisplayName(item: SandboxTemplateItem): string {
  const name = item.name?.trim() ?? '';
  const id = item.id?.trim() ?? '';
  if (!name || name === id) return '';
  return name;
}

export interface TemplateFieldRow { key: string; labelKey: string; value: string; mono?: boolean }

function formatTemplateTime(value?: string): string {
  const raw = value?.trim();
  if (!raw) return '';
  const ms = Date.parse(raw);
  if (Number.isNaN(ms)) return raw;
  return new Date(ms).toLocaleString();
}

/** Detail rows under each template card (drawer.vue:1287-1327). */
export function templateFieldRows(item: SandboxTemplateItem): TemplateFieldRow[] {
  const rows: TemplateFieldRow[] = [];
  const image = item.image?.trim();
  if (image) rows.push({ key: 'image', labelKey: 'settings.sandbox.templateFieldImage', value: image, mono: true });
  const version = item.version?.trim();
  if (version) rows.push({ key: 'version', labelKey: 'settings.sandbox.templateFieldVersion', value: version });
  const id = item.id?.trim();
  if (id) rows.push({ key: 'id', labelKey: 'settings.sandbox.templateFieldId', value: id, mono: true });
  const created = formatTemplateTime(item.created_at);
  if (created) rows.push({ key: 'created', labelKey: 'settings.sandbox.templateFieldCreated', value: created });
  const instanceType = item.instance_type?.trim();
  if (instanceType) rows.push({ key: 'instance', labelKey: 'settings.sandbox.templateFieldInstance', value: instanceType });
  const networkType = item.network_type?.trim();
  if (networkType) rows.push({ key: 'network', labelKey: 'settings.sandbox.templateFieldNetwork', value: networkType });
  if (item.allow_internet_access === true) {
    rows.push({ key: 'internet', labelKey: 'settings.sandbox.templateFieldInternet', value: 'settings.sandbox.templateInternetOn' });
  } else if (item.allow_internet_access === false) {
    rows.push({ key: 'internet', labelKey: 'settings.sandbox.templateFieldInternet', value: 'settings.sandbox.templateInternetOff' });
  }
  return rows;
}

// --- Egress warning (drawer.vue:916-932) ---

/** Mirrors types.DenyOutCoversAllIPv4: any IPv4 /0 collapses to 0.0.0.0/0. */
export function denyOutRowCoversAllIPv4(row: string): boolean {
  const value = row.trim();
  if (value === '0.0.0.0/0') return true;
  return /^\d{1,3}(?:\.\d{1,3}){3}\/0$/.test(value);
}

/** A domain allow-list without a deny-all fallback is decorative (drawer.vue:916-924). */
export function domainAllowNeedsDenyAll(allowOutRows: readonly string[], denyOutRows: readonly string[], denyEgressByDefault: boolean): boolean {
  if (denyEgressByDefault) return false;
  if (denyOutRows.some((row) => denyOutRowCoversAllIPv4(row))) return false;
  return allowOutRows.some((row) => {
    const value = row.trim();
    if (!value) return false;
    return !/^[0-9./]+$/.test(value);
  });
}

/** A stored network secret survives only under its original identity (drawer.vue:879-889). */
export function isStoredNetworkSecretRecoverable(
  row: { stored?: boolean },
  originalParentIdentity: string | undefined,
  originalChildIdentity: string | undefined,
  currentParentIdentity: string,
  currentChildIdentity: string,
): boolean {
  return row.stored === true
    && originalParentIdentity === currentParentIdentity.trim()
    && originalChildIdentity === currentChildIdentity.trim();
}

// --- Payload collection (drawer.vue:1457-1555) ---

export function collectNetworkPolicy(form: SandboxEditorForm): SandboxNetworkPolicyPayload {
  const policy: SandboxNetworkPolicyPayload = {};
  // Docker can only honour network_mode on the docker block (drawer.vue:1486-1488).
  if (form.backend === 'docker') return policy;
  if (form.denyEgressByDefault) policy.deny_egress_by_default = true;
  const allowOut = form.allowOutRows.map((row) => row.trim()).filter(Boolean);
  const denyOut = form.denyOutRows.map((row) => row.trim()).filter(Boolean);
  if (allowOut.length) policy.allow_out = allowOut;
  if (denyOut.length) policy.deny_out = denyOut;
  if (form.backend === 'cube' && form.cubeRules.length) {
    policy.cube_rules = form.cubeRules.map((rule) => ({
      name: rule.name.trim(),
      scheme: rule.scheme || undefined,
      sni: rule.sni?.trim() || undefined,
      host: rule.host?.trim() || undefined,
      methods: rule.methodsText.split(',').map((method) => method.trim().toUpperCase()).filter(Boolean),
      path: rule.path?.trim() || undefined,
      deny: rule.deny || undefined,
      audit: rule.audit || undefined,
      inject: rule.deny
        ? undefined
        : rule.inject
          .filter((inject) => inject.header.trim())
          .map((inject) => ({
            header: inject.header.trim(),
            secret: isStoredNetworkSecretRecoverable(inject, inject.originalRuleName, inject.originalHeader, rule.name, inject.header) && inject.secret === ''
              ? SECRET_PLACEHOLDER
              : inject.secret,
            format: inject.format?.trim() || undefined,
          })),
    }));
  }
  if (form.backend === 'e2b' && form.e2bHostRules.length) {
    policy.e2b_host_rules = form.e2bHostRules.map((rule) => {
      const headers: Record<string, string> = {};
      for (const header of rule.headers) {
        const name = header.name.trim();
        if (!name) continue;
        headers[name] = isStoredNetworkSecretRecoverable(header, header.originalHost, header.originalName, rule.host, header.name) && header.value === ''
          ? SECRET_PLACEHOLDER
          : header.value;
      }
      return { host: rule.host.trim(), headers };
    });
  }
  return policy;
}

export function collectSandboxConfig(form: SandboxEditorForm): TenantSandboxConfig {
  const envVars: Record<string, string> = {};
  for (const row of form.envRows) {
    const key = row.key.trim();
    if (!key) continue;
    envVars[key] = row.stored && row.value === '' ? SECRET_PLACEHOLDER : row.value;
  }
  const payload: TenantSandboxConfig = {
    sandbox_type: form.backend,
    default_timeout_sec: form.defaultTimeoutSec || undefined,
    terminal_idle_disconnect_sec: form.terminalIdleDisconnectSec || undefined,
    allow_private_endpoints: form.allowPrivateEndpoints || undefined,
    env_vars: envVars,
    skill_rollout: form.skillRollout,
    network: collectNetworkPolicy(form),
  };
  // Only the selected backend's block is sent (drawer.vue:1473-1477).
  if (form.backend === 'cube') payload.cube = withStoredSecret({ ...form.cube }, form.storedCubeKey);
  if (form.backend === 'e2b') payload.e2b = withStoredSecret({ ...form.e2b }, form.storedE2BKey);
  if (form.backend === 'docker') payload.docker = { ...form.docker };
  return payload;
}

// --- Wire parsers (endpoint shapes per frontend/src/api/system.ts) ---

export interface SandboxTemplateCatalogView { templates: SandboxTemplateItem[]; standardTemplateId: string; provisioned: boolean }

export function parseTemplateCatalog(value: unknown): SandboxTemplateCatalogView {
  const envelope = value as { data?: { templates?: unknown; standard_template_id?: unknown; provisioned?: unknown } } | null;
  const data = envelope && typeof envelope === 'object' ? envelope.data : undefined;
  const row = data && typeof data === 'object' ? data as Record<string, unknown> : {};
  const rawList = row.templates;
  const templates: SandboxTemplateItem[] = Array.isArray(rawList)
    ? rawList.filter((item): item is SandboxTemplateItem => typeof item === 'object' && item !== null && typeof (item as SandboxTemplateItem).id === 'string')
    : [];
  const standardId = row.standard_template_id;
  return {
    templates,
    standardTemplateId: typeof standardId === 'string' ? standardId : '',
    provisioned: row.provisioned === true,
  };
}

export function parseCheckResult(value: unknown): SandboxCheckResultView {
  const envelope = value as { data?: { ok?: unknown; provider?: unknown; checks?: unknown } } | null;
  const data = envelope && typeof envelope === 'object' ? envelope.data : undefined;
  const row = data && typeof data === 'object' ? data as Record<string, unknown> : {};
  const rawChecks = Array.isArray(row.checks) ? row.checks : [];
  const checks: SandboxCheckItemView[] = rawChecks
    .filter((item): item is Record<string, unknown> => typeof item === 'object' && item !== null)
    .map((item) => ({
      name: typeof item.name === 'string' ? item.name : '',
      ok: typeof item.ok === 'boolean' ? item.ok : null,
      ...(typeof item.message === 'string' ? { message: item.message } : {}),
      ...(typeof item.reason === 'string' ? { reason: item.reason } : {}),
      ...(typeof item.latency_ms === 'number' ? { latency_ms: item.latency_ms } : {}),
    }));
  return { ok: row.ok === true, ...(typeof row.provider === 'string' ? { provider: row.provider } : {}), checks };
}

// --- Localization -----------------------------------------------------------
// settingsT pattern plus a fallback: the drawer's settings.sandbox.* keys are
// not all in packages/i18n yet (the bulk migration covered the list page), so
// the zh-CN strings from frontend/src/i18n/locales/zh-CN.ts are mirrored
// verbatim here until the shared package catches up. formatMessage wins for
// any key it knows, so adding a key upstream automatically localizes this
// panel for all five locales.

const SANDBOX_FALLBACK_STRINGS: Record<string, string> = {
  'settings.sandbox.createTitle': '添加沙箱',
  'settings.sandbox.editTitle': '编辑沙箱',
  'settings.sandbox.setupProgress': '沙箱配置进度',
  'settings.sandbox.stepConnection': '连接',
  'settings.sandbox.stepTemplate': '模板',
  'settings.sandbox.stepRuntime': '运行配置',
  'settings.sandbox.stepDescriptions.connection': '先配置后端并验证连接，通过后再从集群加载模板。',
  'settings.sandbox.stepDescriptions.template': '选择当前集群返回且已经就绪的运行模板。',
  'settings.sandbox.stepDescriptions.runtime': '配置执行参数、环境变量，以及技能镜像如何生效，然后保存。',
  'settings.sandbox.back': '上一步',
  'settings.sandbox.connectAndContinue': '连接并继续',
  'settings.sandbox.saveFailed': '保存失败',
  'settings.sandbox.loading': '加载沙箱配置...',
  'settings.sandbox.enableScripts': '启用沙箱执行',
  'settings.sandbox.sectionBasic': '基本信息',
  'settings.sandbox.backendType': '沙箱类型',
  'settings.sandbox.backendTypePlaceholder': '选择沙箱类型',
  'settings.sandbox.backendDescriptions.cube': '适合私有化或内网部署的自建 MicroVM 集群',
  'settings.sandbox.backendDescriptions.e2b': 'E2B 托管服务或兼容 E2B 的集群',
  'settings.sandbox.backendDescriptions.docker': '在本机 Docker 上为每个会话保留一个长驻容器，脚本和文件都落在同一容器里',
  'settings.sandbox.configName': '配置名称',
  'settings.sandbox.configNamePlaceholder': '如：生产 E2B',
  'settings.sandbox.configNameRequired': '请填写配置名称',
  'settings.sandbox.configDescription': '描述',
  'settings.sandbox.configDescriptionPlaceholder': '选填，用于区分多份同类型配置',
  'settings.sandbox.sectionConnection': '集群连接',
  'settings.sandbox.identityFieldHint': '该配置下有沙箱运行时，以下项无法修改：后端类型、API 端点、API Key、沙箱域名、Proxy 端点。',
  'settings.sandbox.connectionLockedBySkills': '该沙箱已安装 Skill。连接地址、凭据和 DNS 会改变技能快照所属环境，且 DNS 需重建模板才能生效；请新建一份沙箱。',
  'settings.sandbox.connectionLockedByInFlight': '该沙箱正在安装或移除 Skill，完成前不能更换连接、凭据或 DNS。',
  'settings.sandbox.apiUrl': 'API 端点',
  'settings.sandbox.proxyUrl': 'Proxy 端点',
  'settings.sandbox.sandboxDomain': '沙箱域名',
  'settings.sandbox.apiKey': 'API Key',
  'settings.sandbox.apiKeyPlaceholder': '粘贴 API Key',
  'settings.sandbox.secretConfigured': '已配置密钥（不可回看）；输入新值可轮换',
  'settings.sandbox.secretKeepHint': '已配置，留空表示不修改',
  'settings.sandbox.cubeApiKeyOptional': '可留空 —— 自建 CubeSandbox 通常无鉴权',
  'settings.sandbox.cubeApiKeyWhere': '自建集群如何开启鉴权',
  'settings.sandbox.cubeDnsServers': 'DNS 服务器',
  'settings.sandbox.cubeDnsServersHelp': '可选。写入 WeKnora 标准模板的 nameserver（须为 IP）。留空则使用集群默认（常见 119.29.29.29）。私网或云上 UDP 53 出不去时，填 Cube 宿主机 /etc/resolv.conf 里能用的地址，并避开 10/8、172.16/12、192.168/16。已有标准模板需在模板卡片上点「重建」才会生效。',
  'settings.sandbox.cubeDnsServersPlaceholder': '例如 8.8.8.8，回车添加',
  'settings.sandbox.e2bApiKeyHelp': '在 E2B 控制台的 API Keys 页面创建，通常以 e2b_ 开头。',
  'settings.sandbox.e2bApiKeyWhere': '前往 E2B 控制台获取 API Key',
  'settings.sandbox.e2bApiUrlOptional': '可留空 —— 留空时使用 SDK 默认值',
  'settings.sandbox.e2bDomainOptional': '可留空 —— 留空时使用 SDK 默认值',
  'settings.sandbox.e2bProxyUrlOptional': '自建 E2B 兼容集群的数据面网关地址；留空表示按 sandbox domain 直连（E2B Cloud 用法）',
  'settings.sandbox.allowPrivateEndpoints': '允许访问私网集群地址',
  'settings.sandbox.allowPrivateEndpointsHint': '仅用于自建 Cube 等私网控制面；云元数据等链路本地地址始终禁止访问。',
  'settings.sandbox.sectionRuntimeEnvironment': '运行环境',
  'settings.sandbox.weknoraDockerImage': 'WeKnora 标准镜像',
  'settings.sandbox.weknoraDockerImageHint': '每次会话独占一个长驻容器，脚本、shell 命令与文件都在同一个容器内，会话结束或空闲超时后回收。',
  'settings.sandbox.recommendedTag': '推荐',
  'settings.sandbox.dockerImage': 'Docker 镜像',
  'settings.sandbox.dockerHost': 'Docker 守护进程地址',
  'settings.sandbox.dockerHostHelp': '留空跟随本机 docker CLI（DOCKER_HOST 或当前 docker context），不必手填 /var/run/docker.sock。远程守护进程填 tcp://host:2376，必须同时填写 TLS 证书目录，私网地址还要开启「允许访问私网地址」。',
  'settings.sandbox.dockerTlsCertPath': 'TLS 证书目录',
  'settings.sandbox.dockerTlsCertPathHelp': 'WeKnora 所在主机上包含 ca.pem、cert.pem、key.pem 的目录。远程守护进程必填，证书不入库，由部署方挂载。',
  'settings.sandbox.dockerHostRisk': '留空或 unix:// 会使用 WeKnora 所在机器的 Docker 守护进程，权限等同该机 root，只适合私有化单机。多套空间共用同一主机时请改用 Cube 或 E2B。远程 tcp:// 必须填写 TLS 证书目录。',
  'settings.sandbox.templateLockedBySkills': '该沙箱已安装 Skill，技能环境绑在当前快照上，不能更换或重建运行模板。请新建一份沙箱，从新模板再装 Skill。',
  'settings.sandbox.templateLockedByInFlight': '该沙箱正在安装或移除 Skill，完成前不能更换或重建运行模板。',
  'settings.sandbox.sectionTemplate': '运行模板',
  'settings.sandbox.refreshTemplates': '刷新模板',
  'settings.sandbox.loadingTemplates': '正在从集群加载模板…',
  'settings.sandbox.noTemplates': '当前集群未返回可用模板。',
  'settings.sandbox.howToBuildTemplate': '沙箱集群搭建与模板说明',
  'settings.sandbox.weknoraStandardTemplate': 'WeKnora 标准模板',
  'settings.sandbox.createStandardTemplate': '创建',
  'settings.sandbox.createStandardTemplateHint': '按当前连接配置构建，包含 DNS。之后改配置可在卡片上重建。',
  'settings.sandbox.replaceStandardTemplate': '重建',
  'settings.sandbox.replaceStandardTemplateConfirm': '将用当前配置（含 DNS）重建 WeKnora 标准模板。新模板就绪前不会删除仍可启动的旧模板。',
  'settings.sandbox.templateUnnamed': '未命名模板',
  'settings.sandbox.templateStatuses.ready': '已就绪',
  'settings.sandbox.templateStatuses.building': '构建中',
  'settings.sandbox.templateStatuses.untagged': '缺少 default 标签',
  'settings.sandbox.templateStatuses.failed': '失败',
  'settings.sandbox.templateStatuses.unknown': '未知',
  'settings.sandbox.templateFieldImage': '镜像',
  'settings.sandbox.templateFieldVersion': '版本',
  'settings.sandbox.templateFieldId': 'ID',
  'settings.sandbox.templateFieldCreated': '创建时间',
  'settings.sandbox.templateFieldInstance': '规格',
  'settings.sandbox.templateFieldNetwork': '网络',
  'settings.sandbox.templateFieldInternet': '公网访问',
  'settings.sandbox.templateInternetOn': '已开启',
  'settings.sandbox.templateInternetOff': '已关闭',
  'settings.sandbox.templateUntaggedHint': '构建已完成，但没有构建带 default 标签，创建沙箱时无法解析。请在 E2B 删除该模板，刷新后 WeKnora 会重新构建。',
  'settings.sandbox.templateFailedReason': '构建失败：{reason}',
  'settings.sandbox.templateBuildingHint': '标准模板正在构建，列表会自动刷新。',
  'settings.sandbox.templateNotReady': '所选模板尚未构建完成，请刷新并等待状态就绪',
  'settings.sandbox.templateLoadFailed': '模板列表加载失败',
  'settings.sandbox.standardTemplateProvisioning': '已开始在集群中创建 WeKnora 标准模板，请稍后刷新查看状态',
  'settings.sandbox.standardTemplateReplaced': '已删除原标准模板并开始重建，请等待状态就绪',
  'settings.sandbox.sectionRuntime': '执行设置',
  'settings.sandbox.httpTimeout': 'HTTP 超时（秒）',
  'settings.sandbox.httpTimeoutHelp': '调用沙箱管理接口的等待上限，超过即视为端点不可用。留空按 30 秒。',
  'settings.sandbox.sandboxTtl': '沙箱 TTL（秒）',
  'settings.sandbox.sandboxTtlHelp': '沙箱多久之后会暂停',
  'settings.sandbox.dockerIdleTtl': '空闲回收（秒）',
  'settings.sandbox.dockerIdleTtlHelp': 'Docker 守护进程本身没有空闲超时。容器多久没有执行任何命令就会被 WeKnora 回收，会话继续时重建。留空按 1800 秒。',
  'settings.sandbox.dockerCpuLimit': 'CPU 核数上限',
  'settings.sandbox.dockerCpuLimitHelp': '单个沙箱可用的 CPU 核数，0 使用内置默认。',
  'settings.sandbox.dockerMemoryLimit': '内存上限（MB）',
  'settings.sandbox.dockerMemoryLimitHelp': '单个沙箱的内存上限（MB），0 使用内置默认。',
  'settings.sandbox.dockerPidsLimit': '进程数上限',
  'settings.sandbox.dockerPidsLimitHelp': '单个沙箱可创建的进程数上限，0 使用内置默认。',
  'settings.sandbox.defaultTimeout': '执行超时（秒）',
  'settings.sandbox.defaultTimeoutHelp': '单次技能脚本允许运行的最长时间，超时会被强制终止。留空按 60 秒。',
  'settings.sandbox.terminalIdleDisconnect': '交互式终端空闲断开（秒）',
  'settings.sandbox.terminalIdleDisconnectHelp': '打开终端后，这段时间内没有键盘输入或终端输出就断开连接，沙箱随后按 TTL 自行暂停。留空按 900 秒；最短 60 秒，最长 24 小时。',
  'settings.sandbox.sectionNetwork': '网络策略',
  'settings.sandbox.networkHint': '控制该配置下所有沙箱的出网。改动只影响之后新建的沙箱，已有沙箱按原策略运行到回收。',
  'settings.sandbox.egressDefault': '出站默认',
  'settings.sandbox.egressAllowAll': '允许公网（默认）',
  'settings.sandbox.egressDenyAll': '默认拒绝',
  'settings.sandbox.egressPrecedence': '判定顺序：放行 → 拒绝 → 默认值。放行优先于拒绝。',
  'settings.sandbox.allowOut': '放行目标',
  'settings.sandbox.allowOutPlaceholder': '域名 / IP / CIDR，例如 *.example.com',
  'settings.sandbox.allowOutHelp': '支持 IPv4、CIDR、域名和单层通配 *.example.com（通配不匹配主域）。',
  'settings.sandbox.denyOut': '拒绝目标',
  'settings.sandbox.denyOutPlaceholder': '仅支持 IP / CIDR，例如 169.254.169.254/32',
  'settings.sandbox.denyOutHelp': '拒绝只按目的 IP 匹配，因此不支持域名。',
  'settings.sandbox.domainAllowNeedsDenyAll': '放行目标里有域名时，必须同时选择「默认拒绝」或在拒绝目标中加入 0.0.0.0/0，否则白名单不生效。',
  'settings.sandbox.addTarget': '添加目标',
  'settings.sandbox.dockerNetworkMode': '网络模式',
  'settings.sandbox.dockerNetworkModeHelp': '默认 bridge，技能安装依赖需要出网。选择 none 表示完全禁止出网。Docker 只能按网络隔离，无法按域名放行。',
  'settings.sandbox.dockerNetworkBridge': 'bridge（允许出网）',
  'settings.sandbox.dockerNetworkNone': 'none（禁止出网）',
  'settings.sandbox.cubeL7Rules': 'HTTP 访问规则（L7）',
  'settings.sandbox.cubeL7RulesHelp': '每条规则必须填 host 或 sni，网络层只从这两个字段提取放行目标。字段之间是 AND，method 列表内部是 OR。仅 HTTP 80 / HTTPS 443 生效。规则从上到下先匹配先生效。',
  'settings.sandbox.addRule': '添加规则',
  'settings.sandbox.ruleUntitled': '未命名规则',
  'settings.sandbox.expandRule': '展开规则',
  'settings.sandbox.collapseRule': '收起规则',
  'settings.sandbox.moveRuleUp': '上移规则',
  'settings.sandbox.moveRuleDown': '下移规则',
  'settings.sandbox.ruleName': '规则名',
  'settings.sandbox.ruleScheme': '协议',
  'settings.sandbox.ruleSni': 'SNI',
  'settings.sandbox.ruleHost': 'Host',
  'settings.sandbox.ruleMethods': 'HTTP 方法',
  'settings.sandbox.rulePath': '路径',
  'settings.sandbox.ruleAction': '动作',
  'settings.sandbox.ruleAllow': '放行',
  'settings.sandbox.ruleDeny': '拒绝',
  'settings.sandbox.ruleAudit': '审计级别',
  'settings.sandbox.ruleInject': '注入 Header',
  'settings.sandbox.headerName': 'Header 名称',
  'settings.sandbox.headerValue': 'Header 值',
  'settings.sandbox.addHeader': '添加 Header',
  'settings.sandbox.e2bHostRules': 'Host 请求变换',
  'settings.sandbox.e2bHostRulesHelp': '按 host 注入 header。规则本身不授权出网，host 必须同时出现在放行目标里。',
  'settings.sandbox.sectionEnvironment': '环境变量',
  'settings.sandbox.envVarsHint': '注入到该配置下所有沙箱；值加密存储，但对沙箱内脚本可见。\\n只会在沙箱创建时注入，如果要实时注入请在「沙箱密钥」页面添加。',
  'settings.sandbox.envKey': '变量名',
  'settings.sandbox.envValue': '变量值',
  'settings.sandbox.addRow': '添加变量',
  'settings.sandbox.noEnvVars': '暂无额外环境变量。',
  'settings.sandbox.skillRollout': '技能镜像如何生效',
  'settings.sandbox.skillRolloutHint': '安装或删除技能会生成新镜像。这里决定已经打开的会话要不要换到新镜像。',
  'settings.sandbox.skillRolloutNextTurn': '已有会话在下一轮提问时重建沙箱',
  'settings.sandbox.skillRolloutNewSession': '已有会话保持原沙箱，仅新会话使用新镜像',
  'settings.sandbox.deepCheck': '完整验证',
  'settings.sandbox.recheck': '重新验证',
  'settings.sandbox.deepCheckConfirm': '完整验证会执行一次临时脚本；远端后端还会真实创建并销毁一个沙箱，可能消耗少量沙箱时长。是否继续？',
  'settings.sandbox.checkPassed': '检测通过',
  'settings.sandbox.checkFailed': '检测未通过',
  'settings.sandbox.checkScopeConnection': '本次只验证了控制面：端点可达且凭据有效。是否真的能跑起脚本还没有验证。',
  'settings.sandbox.checkScopeFull': '端点、凭据、模板、沙箱内执行与出网均已真实验证。',
  'settings.sandbox.checkScopePolicyRestricted': '出网按策略受限，未做真实出网探测；端点、凭据、模板与沙箱内执行已验证。',
  'settings.sandbox.checkPendingHint': '{names} 需要「完整验证」才能确认：会真实创建一个临时沙箱、执行一次脚本再销毁。',
  'settings.sandbox.skipReasons.needs_deep_check': '需完整验证',
  'settings.sandbox.skipReasons.control_plane_unreachable': '控制面不可达，已跳过',
  'settings.sandbox.skipReasons.sandbox_not_created': '沙箱未创建，已跳过',
  'settings.sandbox.skipReasons.sandbox_exec_failed': '沙箱内执行失败，已跳过',
  'settings.sandbox.skipReasons.egress_restricted_by_policy': '按网络策略受限（该配置默认拒绝出网）',
  'settings.sandbox.checks.client_build': '客户端构建',
  'settings.sandbox.checks.api_url_reachable': '端点可达性',
  'settings.sandbox.checks.credential_valid': '凭据有效性',
  'settings.sandbox.checks.template_exists': '模板存在性',
  'settings.sandbox.checks.sandbox_exec': '沙箱内执行',
  'settings.sandbox.checks.egress_available': '出网可用',
  'settings.sandbox.unverifiableBlocked': '无法连接该后端核实是否仍有沙箱，因此不能覆盖现有凭据。',
  'settings.sandbox.unverifiableSaveHint': '请先恢复该后端的连通性；若已废弃，可新建一份配置并把智能体指过去。',
  'settings.sandbox.affectedSessions': '影响 {count} 个会话。',
  'settings.sandbox.fieldRequired': '必填',
  'settings.sandbox.backends.cube': 'CubeSandbox',
  'settings.sandbox.backends.e2b': 'E2B',
  'settings.sandbox.backends.docker': 'Docker',
  'settings.sandbox.legacyConfig': '已废弃',
  'common.saveSuccess': '保存成功',
};

function interpolate(template: string, values?: Record<string, string | number>): string {
  if (!values) return template;
  return template.replace(/\{(\w+)\}/g, (_match, name: string) => String(values[name] ?? `{${name}}`));
}

export type SandboxT = (key: string, values?: Record<string, string | number>, fallback?: string) => string;

export function sandboxT(locale: Locale): SandboxT {
  return (key, values, fallback) => {
    const shared = formatMessage(locale, key, values);
    if (shared !== key) return shared;
    const template = SANDBOX_FALLBACK_STRINGS[key];
    if (template !== undefined) return interpolate(template, values);
    if (fallback !== undefined) return interpolate(fallback, values);
    return key;
  };
}

// --- Editor form construction (drawer.vue reset(), 1119-1196) ---

let cubeRuleKeySeq = 0;
function newCubeRuleKey(): string { cubeRuleKeySeq += 1; return `cube-rule-${cubeRuleKeySeq}`; }

export function initialEditorForm(record: SandboxConfigRecord | null, presetType: string): SandboxEditorForm {
  const cfg: TenantSandboxConfig = record?.config || {};
  const fromRecord = record?.config?.sandbox_type || presetType || '';
  const backend: SandboxBackendType = isNamedSandboxBackend(fromRecord) ? fromRecord as SandboxBackendType : 'cube';
  const cube: CubeSandboxConfig = { ...(cfg.cube || {}) };
  const e2b: E2BSandboxConfig = { ...(cfg.e2b || {}) };
  const docker: DockerSandboxConfig = { ...(cfg.docker || {}) };
  if (!Array.isArray(cube.dns_servers)) cube.dns_servers = [];
  if (backend === 'docker' && !docker.image) docker.image = DEFAULT_DOCKER_IMAGE;
  const storedCubeKey = isMaskedSecret(cube.api_key);
  const storedE2BKey = isMaskedSecret(e2b.api_key);
  if (storedCubeKey) cube.api_key = '';
  if (storedE2BKey) e2b.api_key = '';
  const net = (cfg.network || {}) as {
    deny_egress_by_default?: boolean; allow_out?: string[]; deny_out?: string[];
    cube_rules?: Array<{ name?: string; scheme?: string; sni?: string; host?: string; methods?: string[]; path?: string; deny?: boolean; audit?: string; inject?: Array<{ header?: string; secret?: string; format?: string }> }>;
    e2b_host_rules?: Array<{ host?: string; headers?: Record<string, string> }>;
  };
  return {
    name: record?.name || '',
    description: record?.description || '',
    backend,
    defaultTimeoutSec: (cfg as { default_timeout_sec?: number }).default_timeout_sec || undefined,
    terminalIdleDisconnectSec: (cfg as { terminal_idle_disconnect_sec?: number }).terminal_idle_disconnect_sec || undefined,
    allowPrivateEndpoints: (cfg as { allow_private_endpoints?: boolean }).allow_private_endpoints === true,
    cube,
    e2b,
    docker,
    storedCubeKey,
    storedE2BKey,
    envRows: Object.entries((cfg as { env_vars?: Record<string, string> }).env_vars || {}).map(([key, value]) => (
      isMaskedSecret(value) ? { key, value: '', stored: true } : { key, value }
    )),
    skillRollout: (cfg as { skill_rollout?: 'next_turn' | 'new_session' }).skill_rollout === 'new_session' ? 'new_session' : 'next_turn',
    denyEgressByDefault: net.deny_egress_by_default === true,
    allowOutRows: [...(net.allow_out || [])],
    denyOutRows: [...(net.deny_out || [])],
    cubeRules: (net.cube_rules || []).map((rule) => ({
      key: newCubeRuleKey(),
      name: rule.name || '',
      scheme: rule.scheme || '',
      sni: rule.sni || '',
      host: rule.host || '',
      methodsText: (rule.methods || []).join(', '),
      path: rule.path || '',
      deny: rule.deny === true,
      audit: rule.audit || '',
      inject: (rule.inject || []).map((inject) => ({
        header: inject.header || '',
        secret: isMaskedSecret(inject.secret) ? '' : (inject.secret || ''),
        format: inject.format || '',
        stored: isMaskedSecret(inject.secret),
        originalRuleName: rule.name?.trim() || '',
        originalHeader: inject.header?.trim() || '',
      })),
    })),
    e2bHostRules: (net.e2b_host_rules || []).map((rule) => ({
      host: rule.host || '',
      headers: Object.entries(rule.headers || {}).map(([name, value]) => ({
        name,
        value: isMaskedSecret(value) ? '' : value,
        stored: isMaskedSecret(value),
        originalHost: rule.host?.trim() || '',
        originalName: name.trim(),
      })),
    })),
  };
}

// --- Check result view helpers (drawer.vue:1026-1056) ---

export function reportedChecks(result: SandboxCheckResultView | null): SandboxCheckItemView[] {
  return (result?.checks || []).filter((item) => item.ok !== null || item.reason !== PENDING_SKIP_REASON);
}

export function pendingCheckItems(result: SandboxCheckResultView | null): SandboxCheckItemView[] {
  return (result?.checks || []).filter((item) => item.ok === null && item.reason === PENDING_SKIP_REASON);
}

export function egressRestrictedByPolicy(result: SandboxCheckResultView | null): boolean {
  return (result?.checks || []).some((item) => item.reason === EGRESS_RESTRICTED_REASON);
}

export type CheckScopeKey = 'settings.sandbox.checkScopeConnection' | 'settings.sandbox.checkScopePolicyRestricted' | 'settings.sandbox.checkScopeFull';

export function checkScopeKey(result: SandboxCheckResultView | null): CheckScopeKey {
  if (pendingCheckItems(result).length) return 'settings.sandbox.checkScopeConnection';
  if (egressRestrictedByPolicy(result)) return 'settings.sandbox.checkScopePolicyRestricted';
  return 'settings.sandbox.checkScopeFull';
}

// --- Editor drawer component -------------------------------------------------

type NumberField = 'defaultTimeoutSec' | 'terminalIdleDisconnectSec';

interface EditorProps {
  client: WeKnoraClient;
  locale: Locale;
  record: SandboxConfigRecord | null;
  presetType: string;
  dockerBackendEnabled: boolean;
  onClose: () => void;
  onSaved: () => void;
}

function fieldIds(missing: readonly string[]): Record<string, true> {
  return Object.fromEntries(missing.map((field) => [field, true as const]));
}

function SandboxConfigEditor({ client, locale, record, presetType, dockerBackendEnabled, onClose, onSaved }: EditorProps) {
  const t = useMemo(() => sandboxT(locale), [locale]);
  const [form, setForm] = useState<SandboxEditorForm>(() => initialEditorForm(record, presetType));
  const [step, setStep] = useState(0);
  const [fieldErrors, setFieldErrors] = useState<Record<string, true>>({});
  const [nameError, setNameError] = useState(false);
  const [templates, setTemplates] = useState<SandboxTemplateItem[]>([]);
  const [templatesLoading, setTemplatesLoading] = useState(false);
  const [templatesLoaded, setTemplatesLoaded] = useState(false);
  const [templatesError, setTemplatesError] = useState('');
  const [templatesInfo, setTemplatesInfo] = useState('');
  const [checking, setChecking] = useState(false);
  const [checkResult, setCheckResult] = useState<SandboxCheckResultView | null>(null);
  const [lastCheckWasDeep, setLastCheckWasDeep] = useState(false);
  const [saving, setSaving] = useState(false);
  const [actionError, setActionError] = useState('');
  const [conflict, setConflict] = useState<{ code: string; inventory?: SandboxInventory } | null>(null);
  const [inFlightSkill, setInFlightSkill] = useState(false);

  // Live refs so polling timers and deferred loads read the latest state.
  const formRef = useRef(form);
  formRef.current = form;
  useEffect(() => { formRef.current = form; }, [form]);

  const steps = wizardStepKeys(form.backend);
  const stepKey = steps[step] ?? 'connection';
  const isRemote = isRemoteSandboxBackend(form.backend);
  const currentTemplateId = ((form.backend === 'cube' ? form.cube.template_id : form.backend === 'e2b' ? form.e2b.template_id : '') ?? '').trim() || '';
  const currentTemplateIdRef = useRef(currentTemplateId);
  useEffect(() => { currentTemplateIdRef.current = currentTemplateId; }, [currentTemplateId]);

  const selectedTemplate = templates.find((item) => item.id === currentTemplateId) || null;
  const standardTemplate = templates.find((item) => item.standard && item.id) || null;
  const hasSkillSnapshot = Boolean((record?.config as { skill_image?: { snapshot_id?: string } } | undefined)?.skill_image?.snapshot_id?.trim());
  const retargetFrozen = hasSkillSnapshot || inFlightSkill;
  const dockerBackendOff = form.backend === 'docker' && !dockerBackendEnabled;
  const canCreateStandard = isRemote && templatesLoaded && !standardTemplate && !retargetFrozen;
  const canDeepCheck = !dockerBackendOff && canDeepCheckOnStep(stepKey, form.backend);
  const primaryDisabled = dockerBackendOff
    || (stepKey === 'template' && (!selectedTemplate || !isTemplateSelectable(selectedTemplate)));
  const hasPendingTemplates = templates.some(isTemplatePending);

  const updateForm = useCallback((patch: Partial<SandboxEditorForm> | ((current: SandboxEditorForm) => Partial<SandboxEditorForm>)) => {
    setForm((current) => ({ ...current, ...(typeof patch === 'function' ? patch(current) : patch) }));
  }, []);

  // In-flight skill installs freeze identity edits (drawer.vue:1207-1221).
  useEffect(() => {
    const id = record?.id;
    if (!id) return;
    let cancelled = false;
    client.request({ method: 'GET', path: `/api/v1/sandbox-configs/${encodeURIComponent(id)}/skills` })
      .then((value) => {
        if (cancelled) return;
        const rows = (value as { data?: Array<{ status?: string }> } | null)?.data;
        setInFlightSkill(Array.isArray(rows) && rows.some((skill) => skill?.status === 'installing' || skill?.status === 'removing'));
      })
      .catch(() => { if (!cancelled) setInFlightSkill(false); });
    return () => { cancelled = true; };
  }, [client, record?.id]);

  const clearTemplateSelection = useCallback(() => {
    updateForm((current) => current.backend === 'cube'
      ? { cube: { ...current.cube, template_id: '' } }
      : current.backend === 'e2b'
        ? { e2b: { ...current.e2b, template_id: '' } }
        : {});
    setFieldErrors((current) => { const next = { ...current }; delete next.template_id; return next; });
  }, [updateForm]);

  const selectTemplate = useCallback((id: string) => {
    updateForm((current) => current.backend === 'cube'
      ? { cube: { ...current.cube, template_id: id } }
      : current.backend === 'e2b'
        ? { e2b: { ...current.e2b, template_id: id } }
        : {});
    setFieldErrors((current) => { const next = { ...current }; delete next.template_id; return next; });
  }, [updateForm]);

  /** Everything a connection edit invalidates (drawer.vue:1683-1690). */
  const invalidateConnection = useCallback(() => {
    setCheckResult(null);
    setTemplates([]);
    setTemplatesLoaded(false);
    setTemplatesError('');
    clearTemplateSelection();
  }, [clearTemplateSelection]);

  const loadTemplates = useCallback(async (options: { ensureStandard?: boolean; silent?: boolean; replaceStandard?: boolean } = {}): Promise<boolean> => {
    const { ensureStandard = false, silent = false, replaceStandard = false } = options;
    if (!silent) setTemplatesLoading(true);
    setTemplatesError('');
    try {
      const value = await client.request({
        method: 'POST',
        path: '/api/v1/sandbox-configs/templates/query',
        body: {
          config: collectSandboxConfig(formRef.current),
          ...(record?.id ? { config_id: record.id } : {}),
          ensure_standard: ensureStandard,
          replace_standard: replaceStandard,
        },
      });
      const catalog = parseTemplateCatalog(value);
      setTemplates(catalog.templates);
      setTemplatesLoaded(true);
      // Selection fix-up mirrors drawer.vue:1403-1425.
      const previousId = currentTemplateIdRef.current;
      const current = catalog.templates.find((item) => item.id === previousId);
      let nextId = previousId;
      if (replaceStandard && catalog.standardTemplateId) {
        nextId = catalog.standardTemplateId;
      } else if (previousId && (!current || (!isTemplateSelectable(current) && !isTemplatePending(current))) && !retargetFrozen) {
        if (catalog.standardTemplateId) {
          const candidate = catalog.templates.find((item) => item.id === catalog.standardTemplateId);
          nextId = candidate && (isTemplateSelectable(candidate) || isTemplatePending(candidate)) ? catalog.standardTemplateId : '';
        } else {
          nextId = '';
        }
      }
      if (!nextId) {
        const readyStandard = catalog.templates.find((item) => item.id === catalog.standardTemplateId && isTemplateSelectable(item))
          || catalog.templates.find((item) => item.standard && isTemplateSelectable(item));
        if (readyStandard) nextId = readyStandard.id;
      }
      if (nextId !== previousId) {
        if (nextId) selectTemplate(nextId);
        else clearTemplateSelection();
      }
      if (catalog.provisioned && !silent) {
        setTemplatesInfo(t(replaceStandard ? 'settings.sandbox.standardTemplateReplaced' : 'settings.sandbox.standardTemplateProvisioning'));
      }
      return true;
    } catch (cause) {
      setTemplatesError(cause instanceof Error && cause.message ? cause.message : t('settings.sandbox.templateLoadFailed'));
      return false;
    } finally {
      if (!silent) setTemplatesLoading(false);
    }
  }, [client, record?.id, retargetFrozen, selectTemplate, clearTemplateSelection, t]);

  const loadTemplatesRef = useRef(loadTemplates);
  useEffect(() => { loadTemplatesRef.current = loadTemplates; }, [loadTemplates]);

  // Poll while a template build is in flight (drawer.vue:1376-1387).
  useEffect(() => {
    if (stepKey !== 'template' || !hasPendingTemplates) return;
    const timer = setTimeout(() => { void loadTemplatesRef.current({ silent: true }); }, 3000);
    return () => clearTimeout(timer);
  }, [stepKey, hasPendingTemplates, templates]);

  /** Vue validateName (drawer.vue:1562-1569). */
  function validateName(): boolean {
    if (!formRef.current.name.trim()) { setNameError(true); return false; }
    setNameError(false);
    return true;
  }

  /** Vue runCheck (drawer.vue:1649-1676). */
  async function runCheck(deep: boolean): Promise<boolean> {
    const missing = missingRequiredFields(formRef.current, deep);
    if (missing.length) { setFieldErrors((current) => ({ ...current, ...fieldIds(missing) })); return false; }
    setChecking(true);
    setCheckResult(null);
    setActionError('');
    try {
      const value = await client.request({
        method: 'POST',
        path: '/api/v1/system/sandbox-check',
        body: {
          config: collectSandboxConfig(formRef.current),
          ...(record?.id ? { config_id: record.id } : {}),
          deep,
        },
      });
      const result = parseCheckResult(value);
      setCheckResult(result);
      setLastCheckWasDeep(deep);
      return result.ok === true;
    } catch (cause) {
      setActionError(cause instanceof Error && cause.message ? cause.message : t('settings.sandbox.checkFailed'));
      return false;
    } finally {
      setChecking(false);
    }
  }

  /** Vue save (drawer.vue:1610-1647). */
  async function save(): Promise<void> {
    if (!validateName()) return;
    const missing = missingRequiredFields(formRef.current, true);
    if (missing.length) { setFieldErrors((current) => ({ ...current, ...fieldIds(missing) })); return; }
    if (isRemote && selectedTemplate && !isTemplateSelectable(selectedTemplate)) {
      setFieldErrors((current) => ({ ...current, template_id: true }));
      return;
    }
    setSaving(true);
    setConflict(null);
    setActionError('');
    try {
      const payload: SandboxConfigUpsert = {
        name: formRef.current.name.trim(),
        description: formRef.current.description,
        config: collectSandboxConfig(formRef.current),
      };
      if (record) await client.sandboxConfigurations.update(record.id, payload);
      else await client.sandboxConfigurations.create(payload);
      onSaved();
      onClose();
    } catch (cause) {
      const error = cause as Error & { code?: string; details?: unknown };
      const refusal = parseSandboxConfigurationConflict({ error: { code: error.code, message: error.message, data: error.details } });
      if (refusal) {
        // Keep the drawer open with the form intact (drawer.vue:1636-1641).
        setConflict({ code: refusal.code, ...(refusal.inventory ? { inventory: refusal.inventory } : {}) });
        return;
      }
      setActionError(error.message || t('settings.sandbox.saveFailed'));
    } finally {
      setSaving(false);
    }
  }

  /** Vue handlePrimaryAction (drawer.vue:1571-1601). */
  async function handlePrimary(): Promise<void> {
    if (dockerBackendOff) return;
    if (stepKey === 'connection') {
      if (!validateName()) return;
      const missing = missingRequiredFields(formRef.current, false);
      if (missing.length) { setFieldErrors((current) => ({ ...current, ...fieldIds(missing) })); return; }
      if (!(await runCheck(false))) return;
      if (isRemote) {
        setCheckResult(null);
        setStep(step + 1);
        await loadTemplates();
        return;
      }
      // Docker kicks a background pull so the first session does not block on
      // a cold registry fetch (drawer.vue:1583-1586).
      if (form.backend === 'docker') void loadTemplates({ ensureStandard: true });
      setCheckResult(null);
      setStep(step + 1);
      return;
    }
    if (stepKey === 'template') {
      if (!selectedTemplate || !isTemplateSelectable(selectedTemplate)) {
        setFieldErrors((current) => ({ ...current, template_id: true }));
        return;
      }
      setStep(step + 1);
      return;
    }
    await save();
  }

  /** Vue goToStep (drawer.vue:1008-1020). */
  function goToStep(index: number): void {
    if (!canJumpToStep(index, step, Boolean(record))) return;
    const nextKey = steps[index] ?? 'connection';
    setStep(index);
    if (nextKey === 'template' && !templatesLoaded) void loadTemplates();
  }

  function previousStep(): void {
    if (step <= 0) return;
    setStep(step - 1);
  }

  /** Vue onFieldInput (drawer.vue:1079-1082): clearing, not re-validating. */
  function onFieldInput(field: string): void {
    setFieldErrors((current) => { const next = { ...current }; delete next[field]; return next; });
    setCheckResult(null);
  }

  /** Vue onConnectionInput (drawer.vue:1084-1087). */
  function onConnectionInput(field: string): void {
    setFieldErrors((current) => { const next = { ...current }; delete next[field]; return next; });
    invalidateConnection();
  }

  /** Vue selectBackend / onBackendChange (drawer.vue:1198-1205, 1694-1697). */
  function selectBackend(value: string): void {
    if (form.backend === value) return;
    const docker = value === 'docker' && !form.docker.image ? { ...form.docker, image: DEFAULT_DOCKER_IMAGE } : form.docker;
    setForm((current) => ({ ...current, backend: value as SandboxBackendType, docker }));
    setFieldErrors({});
    invalidateConnection();
  }

  const onTemplateCardClick = (item: SandboxTemplateItem): void => {
    if (retargetFrozen && item.id !== currentTemplateId) return;
    if (!isTemplateSelectable(item)) return;
    selectTemplate(item.id);
  };

  const requiredLabel = (labelKey: string): string => `${t(labelKey)} *`;
  const secretInputPlaceholder = (target: 'cube' | 'e2b'): string => (
    (target === 'cube' ? form.storedCubeKey : form.storedE2BKey) ? t('settings.sandbox.secretKeepHint') : t('settings.sandbox.apiKeyPlaceholder')
  );

  function setCube(patch: Partial<CubeSandboxConfig>, field?: string): void {
    updateForm((current) => ({ cube: { ...current.cube, ...patch } }));
    if (field) onFieldInput(field);
  }

  function setE2B(patch: Partial<E2BSandboxConfig>, field?: string): void {
    updateForm((current) => ({ e2b: { ...current.e2b, ...patch } }));
    if (field) onFieldInput(field);
  }

  function setDocker(patch: Partial<DockerSandboxConfig>, field?: string): void {
    updateForm((current) => ({ docker: { ...current.docker, ...patch } }));
    if (field) onFieldInput(field);
  }

  const numberValue = (value: number | undefined): string => (value === undefined ? '' : String(value));
  const numberInputValue = (value: number | undefined): number | '' => value ?? '';
  const parseNumber = (raw: string): number | undefined => (raw.trim() === '' ? undefined : Number(raw));
  const setNumberField = (field: NumberField, raw: string): void => {
    updateForm({ [field]: parseNumber(raw) } as Partial<SandboxEditorForm>);
  };

  const renderFieldError = (field: string) => (
    fieldErrors[field] ? <p className="wk-field-error text-xs leading-[1.4] text-[#c23434]" role="alert">{t('settings.sandbox.fieldRequired')}</p> : null
  );

  const title = record ? t('settings.sandbox.editTitle') : t('settings.sandbox.createTitle');
  const stepTitleKey = (key: SandboxStepKey): string => (
    key === 'connection' ? 'settings.sandbox.stepConnection' : key === 'template' ? 'settings.sandbox.stepTemplate' : 'settings.sandbox.stepRuntime'
  );

  return (
    <section className="wk-sandbox-editor" role="dialog" aria-modal="true" aria-label={title} data-testid="sandbox-editor">
      <div className="wk-settings-panel-heading flex items-start justify-between gap-4 border-b border-[#eef1f5] pb-4 mb-4 max-[720px]:flex-col">
        <div>
          <h3>{title}</h3>
          <p className="wk-muted text-muted m-0">{t(`settings.sandbox.stepDescriptions.${stepKey}`)}</p>
        </div>
        <Button type="button" onClick={onClose}>{t('common.cancel')}</Button>
      </div>

      {/* Step rail (drawer.vue:34-57). */}
      <nav className="wk-sandbox-steps" aria-label={t('settings.sandbox.setupProgress')}>
        {steps.map((key, index) => {
          const clickable = canJumpToStep(index, step, Boolean(record));
          const className = `wk-sandbox-step${step === index ? ' is-active' : ''}${step > index ? ' is-done' : ''}${clickable ? ' is-clickable' : ''}`;
          const marker = <span className="wk-sandbox-step__marker" aria-hidden="true">{step > index ? '✓' : index + 1}</span>;
          const label = <span className="wk-sandbox-step__title">{t(stepTitleKey(key))}</span>;
          return clickable ? (
            <button type="button" key={key} className={className} aria-current={step === index ? 'step' : undefined} onClick={() => goToStep(index)}>{marker}{label}</button>
          ) : (
            <div key={key} className={className} aria-current={step === index ? 'step' : undefined}>{marker}{label}</div>
          );
        })}
      </nav>

      {/* Identity-change refusals sit at the top (drawer.vue:59-81). */}
      {conflict ? (
        <div className="wk-sandbox-conflict" role="alert" data-testid="sandbox-editor-conflict">
          {conflict.code === 'sandboxes_still_live' ? (
            <Status tone="warning">{t('settings.sandbox.sandboxesStillLive', { count: conflict.inventory?.sandboxCount ?? 0 })}</Status>
          ) : conflict.code === 'skill_snapshot_blocks_template' ? (
            <Status tone="warning">{t('settings.sandbox.templateLockedBySkills')}</Status>
          ) : (
            <Status tone="warning">{t('settings.sandbox.unverifiableBlocked')}</Status>
          )}
          {conflict.code === 'sandboxes_still_live' ? (<>
            {(conflict.inventory?.sessionIds.length ?? 0) > 0 ? <p>{t('settings.sandbox.affectedSessions', { count: conflict.inventory?.sessionIds.length ?? 0 })}</p> : null}
            {conflict.inventory?.agentNames.length ? <p>{t('settings.sandbox.affectedAgents', { names: conflict.inventory.agentNames.join('、') })}</p> : null}
            <p>{t('settings.sandbox.blockedHint')}</p>
          </>) : conflict.code === 'sandbox_inventory_unverifiable' ? (
            <p>{t('settings.sandbox.unverifiableSaveHint')}</p>
          ) : null}
        </div>
      ) : null}
      {actionError ? <Status tone="error">{actionError}</Status> : null}

      <form className="wk-settings-editor my-4 grid gap-[.8rem] max-w-[620px] [&_label]:grid [&_label]:gap-[.35rem] [&_label]:text-[#27364d] [&_label]:font-semibold [&_input]:w-full [&_input]:box-border [&_input]:border [&_input]:border-[#cbd5e1] [&_input]:rounded-control [&_input]:bg-white [&_input]:text-ink [&_input]:[font:inherit] [&_input]:px-[.65rem] [&_input]:py-[.55rem] [&_textarea]:w-full [&_textarea]:box-border [&_textarea]:border [&_textarea]:border-[#cbd5e1] [&_textarea]:rounded-control [&_textarea]:bg-white [&_textarea]:text-ink [&_textarea]:[font:inherit] [&_textarea]:px-[.65rem] [&_textarea]:py-[.55rem] [&_select]:w-full [&_select]:[font:inherit]" onSubmit={(event) => { event.preventDefault(); void handlePrimary(); }}>
        {stepKey === 'connection' ? (<>
          <section className="wk-sandbox-editor-section">
            <h4>{t('settings.sandbox.sectionBasic')}</h4>
            <label>
              {t('settings.sandbox.backendType')}
              <Select value={form.backend} disabled={retargetFrozen} onChange={(event) => selectBackend(event.target.value)}>
                {SANDBOX_BACKENDS.filter((type) => dockerBackendEnabled || type !== 'docker' || form.backend === 'docker').map((type) => (
                  <option key={type} value={type}>{t(`settings.sandbox.backends.${type}`)}</option>
                ))}
              </Select>
            </label>
            <p className="wk-muted text-muted">{t(`settings.sandbox.backendDescriptions.${form.backend}`)}</p>
            {dockerBackendOff ? <Status tone="warning">{t('settings.sandbox.dockerDisabledAlert')}{t('settings.sandbox.dockerDisabledHint')}</Status> : null}
            <label>
              {t('settings.sandbox.configName')}
              <Input value={form.name} placeholder={t('settings.sandbox.configNamePlaceholder')} aria-invalid={nameError || undefined} onChange={(event) => { updateForm({ name: event.target.value }); setNameError(false); }} />
              {nameError ? <p className="wk-field-error text-xs leading-[1.4] text-[#c23434]" role="alert">{t('settings.sandbox.configNameRequired')}</p> : null}
            </label>
            <label>
              {t('settings.sandbox.configDescription')}
              <Input value={form.description} placeholder={t('settings.sandbox.configDescriptionPlaceholder')} onChange={(event) => updateForm({ description: event.target.value })} />
            </label>
          </section>

          {isRemote ? (
            <section className="wk-sandbox-editor-section">
              <h4>{t('settings.sandbox.sectionConnection')}</h4>
              {hasSkillSnapshot ? <Status tone="warning">{t('settings.sandbox.connectionLockedBySkills')}</Status>
                : inFlightSkill ? <Status tone="warning">{t('settings.sandbox.connectionLockedByInFlight')}</Status>
                  : record ? <p className="wk-muted text-muted">{t('settings.sandbox.identityFieldHint')}</p> : null}
              {form.backend === 'cube' ? (<>
                <label>{requiredLabel('settings.sandbox.apiUrl')}
                  <Input value={form.cube.api_url ?? ''} placeholder="http://cube.example.com:33000" disabled={retargetFrozen} onChange={(event) => setCube({ api_url: event.target.value }, 'api_url')} />
                  {renderFieldError('api_url')}
                </label>
                <div className="wk-form-grid grid grid-cols-2 gap-4 max-[720px]:grid-cols-1">
                  <label>{requiredLabel('settings.sandbox.proxyUrl')}
                    <Input value={form.cube.proxy_url ?? ''} placeholder="http://cube.example.com:80" disabled={retargetFrozen} onChange={(event) => setCube({ proxy_url: event.target.value }, 'proxy_url')} />
                    {renderFieldError('proxy_url')}
                  </label>
                  <label>{requiredLabel('settings.sandbox.sandboxDomain')}
                    <Input value={form.cube.sandbox_domain ?? ''} placeholder="cube.app" disabled={retargetFrozen} onChange={(event) => setCube({ sandbox_domain: event.target.value }, 'sandbox_domain')} />
                    {renderFieldError('sandbox_domain')}
                  </label>
                </div>
                <label>{t('settings.sandbox.apiKey')}
                  <Input type="password" value={form.cube.api_key ?? ''} placeholder={secretInputPlaceholder('cube')} disabled={retargetFrozen} onChange={(event) => setCube({ api_key: event.target.value })} />
                </label>
                <p className="wk-muted text-muted">{form.storedCubeKey ? t('settings.sandbox.secretConfigured') : t('settings.sandbox.cubeApiKeyOptional')}</p>
                <a href={CLUSTER_GUIDE_URL} target="_blank" rel="noopener noreferrer">{t('settings.sandbox.cubeApiKeyWhere')}</a>
                <label>{t('settings.sandbox.cubeDnsServers')}
                  <Input value={(form.cube.dns_servers ?? []).join(', ')} placeholder={t('settings.sandbox.cubeDnsServersPlaceholder')} disabled={retargetFrozen}
                    onChange={(event) => setCube({ dns_servers: event.target.value.split(/[,\s]+/).map((item) => item.trim()).filter(Boolean) })} />
                </label>
                <p className="wk-muted text-muted">{t('settings.sandbox.cubeDnsServersHelp')}</p>
              </>) : (<>
                <label>{requiredLabel('settings.sandbox.apiKey')}
                  <Input type="password" value={form.e2b.api_key ?? ''} placeholder={secretInputPlaceholder('e2b')} disabled={retargetFrozen} onChange={(event) => setE2B({ api_key: event.target.value }, 'api_key')} />
                  {renderFieldError('api_key')}
                </label>
                <p className="wk-muted text-muted">{form.storedE2BKey ? t('settings.sandbox.secretConfigured') : t('settings.sandbox.e2bApiKeyHelp')}</p>
                <a href={E2B_API_KEYS_URL} target="_blank" rel="noopener noreferrer">{t('settings.sandbox.e2bApiKeyWhere')}</a>
                <div className="wk-form-grid grid grid-cols-2 gap-4 max-[720px]:grid-cols-1">
                  <label>{t('settings.sandbox.apiUrl')}
                    <Input value={form.e2b.api_url ?? ''} placeholder="https://api.e2b.app" disabled={retargetFrozen} onChange={(event) => setE2B({ api_url: event.target.value })} />
                  </label>
                  <label>{t('settings.sandbox.sandboxDomain')}
                    <Input value={form.e2b.sandbox_domain ?? ''} placeholder="e2b.app" disabled={retargetFrozen} onChange={(event) => setE2B({ sandbox_domain: event.target.value })} />
                  </label>
                </div>
                <p className="wk-muted text-muted">{t('settings.sandbox.e2bApiUrlOptional')}</p>
                <label>{t('settings.sandbox.proxyUrl')}
                  <Input value={form.e2b.proxy_url ?? ''} placeholder="http://sandbox-gateway.example.com" disabled={retargetFrozen} onChange={(event) => setE2B({ proxy_url: event.target.value })} />
                </label>
                <p className="wk-muted text-muted">{t('settings.sandbox.e2bProxyUrlOptional')}</p>
              </>)}
              <div className="wk-sandbox-switch-row">
                <div>
                  <p><strong>{t('settings.sandbox.allowPrivateEndpoints')}</strong></p>
                  <p className="wk-muted text-muted">{t('settings.sandbox.allowPrivateEndpointsHint')}</p>
                </div>
                <Checkbox checked={form.allowPrivateEndpoints} disabled={retargetFrozen}
                  onChange={(event) => { updateForm({ allowPrivateEndpoints: event.target.checked }); setCheckResult(null); }} aria-label={t('settings.sandbox.allowPrivateEndpoints')} />
              </div>
            </section>
          ) : (
            <section className="wk-sandbox-editor-section">
              <h4>{t('settings.sandbox.sectionRuntimeEnvironment')}</h4>
              <div className="wk-template-card is-active">
                <strong>{t('settings.sandbox.weknoraDockerImage')}</strong> <span className="wk-tag inline-flex items-center shrink-0 py-[1px]! px-[8px]! leading-[1.6]">{t('settings.sandbox.recommendedTag')}</span>
                <p className="wk-muted text-muted">{t('settings.sandbox.weknoraDockerImageHint')}</p>
              </div>
              <label>{requiredLabel('settings.sandbox.dockerImage')}
                <Input value={form.docker.image ?? ''} placeholder={DEFAULT_DOCKER_IMAGE} disabled={retargetFrozen} onChange={(event) => setDocker({ image: event.target.value }, 'image')} />
                {renderFieldError('image')}
              </label>
              {retargetFrozen ? <p className="wk-muted text-muted">{hasSkillSnapshot ? t('settings.sandbox.templateLockedBySkills') : t('settings.sandbox.templateLockedByInFlight')}</p> : null}
              <label>{t('settings.sandbox.dockerHost')}
                <Input value={form.docker.host ?? ''} placeholder="unix:///var/run/docker.sock" disabled={retargetFrozen} onChange={(event) => setDocker({ host: event.target.value }, 'host')} />
              </label>
              <p className="wk-muted text-muted">{t('settings.sandbox.dockerHostHelp')}</p>
              <label>{t('settings.sandbox.dockerTlsCertPath')}
                <Input value={form.docker.tls_cert_path ?? ''} placeholder="/etc/weknora/docker-certs" disabled={retargetFrozen} onChange={(event) => setDocker({ tls_cert_path: event.target.value }, 'tls_cert_path')} />
              </label>
              <p className="wk-muted text-muted">{t('settings.sandbox.dockerTlsCertPathHelp')}</p>
              <Status tone="warning">{t('settings.sandbox.dockerHostRisk')}</Status>
              <div className="wk-sandbox-switch-row">
                <div>
                  <p><strong>{t('settings.sandbox.allowPrivateEndpoints')}</strong></p>
                  <p className="wk-muted text-muted">{t('settings.sandbox.allowPrivateEndpointsHint')}</p>
                </div>
                <Checkbox checked={form.allowPrivateEndpoints} disabled={retargetFrozen}
                  onChange={(event) => { updateForm({ allowPrivateEndpoints: event.target.checked }); setCheckResult(null); }} aria-label={t('settings.sandbox.allowPrivateEndpoints')} />
              </div>
            </section>
          )}
        </>) : null}

        {stepKey === 'template' ? (
          <section className="wk-sandbox-editor-section">
            <div className="wk-sandbox-section-row">
              <h4>{t('settings.sandbox.sectionTemplate')}</h4>
              <Button type="button" loading={templatesLoading} onClick={() => void loadTemplates()}>{t('settings.sandbox.refreshTemplates')}</Button>
            </div>
            {hasSkillSnapshot ? <Status tone="warning">{t('settings.sandbox.templateLockedBySkills')}</Status>
              : inFlightSkill ? <Status tone="warning">{t('settings.sandbox.templateLockedByInFlight')}</Status> : null}
            {templatesLoading && !templatesLoaded ? <Status>{t('settings.sandbox.loadingTemplates')}</Status> : (
              <div className="wk-template-list" role="radiogroup" aria-label={t('settings.sandbox.sectionTemplate')}>
                {canCreateStandard ? (
                  <div className="wk-template-row wk-template-row--offer">
                    <div>
                      <strong>{t('settings.sandbox.weknoraStandardTemplate')}</strong> <span className="wk-tag inline-flex items-center shrink-0 py-[1px]! px-[8px]! leading-[1.6]">{t('settings.sandbox.recommendedTag')}</span>{' '}
                      <Button type="button" loading={templatesLoading} onClick={() => void loadTemplates({ ensureStandard: true })}>{t('settings.sandbox.createStandardTemplate')}</Button>
                      <p className="wk-muted text-muted">{t('settings.sandbox.createStandardTemplateHint')}</p>
                    </div>
                  </div>
                ) : null}
                {templates.map((item) => {
                  const disabled = !isTemplateSelectable(item) || (retargetFrozen && item.id !== currentTemplateId);
                  const name = templateDisplayName(item);
                  const failureReason = isTemplateFailed(item) && item.error?.trim() ? t('settings.sandbox.templateFailedReason', { reason: item.error }) : '';
                  const rows = templateFieldRows(item);
                  return (
                    <div key={item.id}
                      className={`wk-template-row${currentTemplateId === item.id ? ' is-active' : ''}${isTemplatePending(item) ? ' is-pending' : ''}${disabled ? ' is-disabled' : ''}`}
                      role="radio" aria-checked={currentTemplateId === item.id} aria-disabled={disabled}
                      tabIndex={disabled ? -1 : 0}
                      onClick={() => onTemplateCardClick(item)}
                      onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') onTemplateCardClick(item); }}>
                      <div>
                        <strong title={name || undefined}>{name || t('settings.sandbox.templateUnnamed')}</strong>
                        {item.standard ? <span className="wk-tag inline-flex items-center shrink-0 py-[1px]! px-[8px]! leading-[1.6]">{t('settings.sandbox.recommendedTag')}</span> : null}{' '}
                        <span className={`wk-tag wk-tag--${templateStatusKey(item)} inline-flex items-center shrink-0 py-[1px]! px-[8px]! leading-[1.6]`}>{t(`settings.sandbox.templateStatuses.${templateStatusKey(item)}`)}</span>{' '}
                        {item.standard && item.id && !isTemplatePending(item) && !retargetFrozen ? (
                          <Button type="button" loading={templatesLoading} onClick={(event) => { event.stopPropagation(); if (window.confirm(t('settings.sandbox.replaceStandardTemplateConfirm'))) void loadTemplates({ replaceStandard: true }); }}>
                            {t('settings.sandbox.replaceStandardTemplate')}
                          </Button>
                        ) : null}
                        {rows.length ? (
                          <dl className="wk-template-fields">
                            {rows.map((row) => (
                              <div key={row.key}><dt>{t(row.labelKey)}</dt><dd className={row.mono ? 'is-mono' : undefined}>{row.value.startsWith('settings.sandbox.') ? t(row.value) : row.value}</dd></div>
                            ))}
                          </dl>
                        ) : null}
                        {isTemplateUntagged(item) ? <p className="wk-field-error text-xs leading-[1.4] text-[#c23434]">{t('settings.sandbox.templateUntaggedHint')}</p>
                          : failureReason ? <p className="wk-field-error text-xs leading-[1.4] text-[#c23434]">{failureReason}</p>
                            : isTemplatePending(item) && item.standard ? <p className="wk-muted text-muted">{t('settings.sandbox.templateBuildingHint')}</p> : null}
                      </div>
                    </div>
                  );
                })}
                {templatesLoaded && templates.length === 0 && !canCreateStandard && !templatesError ? <Status>{t('settings.sandbox.noTemplates')}</Status> : null}
              </div>
            )}
            {templatesError ? <Status tone="warning">{templatesError}</Status> : null}
            {templatesInfo ? <Status>{templatesInfo}</Status> : null}
            {fieldErrors.template_id ? <p className="wk-field-error text-xs leading-[1.4] text-[#c23434]" role="alert">{t('settings.sandbox.templateNotReady')}</p> : null}
            <a href={CLUSTER_GUIDE_URL} target="_blank" rel="noopener noreferrer">{t('settings.sandbox.howToBuildTemplate')}</a>
          </section>
        ) : null}

        {stepKey === 'runtime' ? (<>
          <section className="wk-sandbox-editor-section">
            <h4>{t('settings.sandbox.sectionRuntime')}</h4>
            <div className="wk-form-grid grid grid-cols-2 gap-4 max-[720px]:grid-cols-1">
              {isRemote ? (<>
                <label>{t('settings.sandbox.httpTimeout')}
                  <NumberInput min={0} max={Number.MAX_SAFE_INTEGER} placeholder="30" value={numberInputValue(form.backend === 'cube' ? form.cube.http_timeout_sec : form.e2b.http_timeout_sec)}
                    onValueChange={(value) => { const next = value === '' ? undefined : value; if (form.backend === 'cube') setCube({ http_timeout_sec: next }); else setE2B({ http_timeout_sec: next }); }} />
                </label>
                <label>{t('settings.sandbox.sandboxTtl')}
                  <NumberInput min={0} max={Number.MAX_SAFE_INTEGER} placeholder={form.backend === 'cube' ? '1800' : '300'} value={numberInputValue(form.backend === 'cube' ? form.cube.cube_sandbox_ttl_seconds : form.e2b.e2b_sandbox_ttl_seconds)}
                    onValueChange={(value) => { const next = value === '' ? undefined : value; if (form.backend === 'cube') setCube({ cube_sandbox_ttl_seconds: next }); else setE2B({ e2b_sandbox_ttl_seconds: next }); }} />
                </label>
              </>) : (<>
                <label>{t('settings.sandbox.dockerIdleTtl')}
                  <NumberInput min={0} max={Number.MAX_SAFE_INTEGER} placeholder="1800" value={numberInputValue(form.docker.idle_ttl_seconds)} onValueChange={(value) => setDocker({ idle_ttl_seconds: value === '' ? undefined : value })} />
                </label>
                <label>{t('settings.sandbox.dockerCpuLimit')}
                  <NumberInput min={0} max={Number.MAX_SAFE_INTEGER} step={0.5} placeholder="2" value={numberInputValue(form.docker.cpu_limit)} onValueChange={(value) => setDocker({ cpu_limit: value === '' ? undefined : value })} />
                </label>
                <label>{t('settings.sandbox.dockerMemoryLimit')}
                  <NumberInput min={0} max={Number.MAX_SAFE_INTEGER} placeholder="2048" value={numberInputValue(form.docker.memory_limit_mb)} onValueChange={(value) => setDocker({ memory_limit_mb: value === '' ? undefined : value })} />
                </label>
                <label>{t('settings.sandbox.dockerPidsLimit')}
                  <NumberInput min={0} max={Number.MAX_SAFE_INTEGER} placeholder="512" value={numberInputValue(form.docker.pids_limit)} onValueChange={(value) => setDocker({ pids_limit: value === '' ? undefined : value })} />
                </label>
              </>)}
              <label>{t('settings.sandbox.defaultTimeout')}
                <NumberInput min={0} max={Number.MAX_SAFE_INTEGER} placeholder="60" value={numberInputValue(form.defaultTimeoutSec)} onValueChange={(value) => setNumberField('defaultTimeoutSec', value === '' ? '' : String(value))} />
              </label>
              <label>{t('settings.sandbox.terminalIdleDisconnect')}
                <NumberInput min={0} max={86400} placeholder="900" value={numberInputValue(form.terminalIdleDisconnectSec)} onValueChange={(value) => setNumberField('terminalIdleDisconnectSec', value === '' ? '' : String(value))} />
              </label>
            </div>
            <p className="wk-muted text-muted">{t('settings.sandbox.httpTimeoutHelp')}</p>
            <p className="wk-muted text-muted">{t('settings.sandbox.sandboxTtlHelp')}</p>
            <p className="wk-muted text-muted">{t('settings.sandbox.defaultTimeoutHelp')}</p>
            <p className="wk-muted text-muted">{t('settings.sandbox.terminalIdleDisconnectHelp')}</p>
          </section>

          <section className="wk-sandbox-editor-section">
            <h4>{t('settings.sandbox.sectionNetwork')}</h4>
            <p className="wk-muted text-muted">{t('settings.sandbox.networkHint')}</p>
            {form.backend !== 'docker' ? (<>
              <fieldset>
                <legend>{t('settings.sandbox.egressDefault')}</legend>
                <label><Radio name="sandbox-egress-policy" checked={!form.denyEgressByDefault} onChange={() => updateForm({ denyEgressByDefault: false })} /> {t('settings.sandbox.egressAllowAll')}</label>
                <label><Radio name="sandbox-egress-policy" checked={form.denyEgressByDefault} onChange={() => updateForm({ denyEgressByDefault: true })} /> {t('settings.sandbox.egressDenyAll')}</label>
              </fieldset>
              <p className="wk-muted text-muted">{t('settings.sandbox.egressPrecedence')}</p>
              <div className="wk-sandbox-rows">
                <div className="wk-sandbox-section-row">
                  <strong>{t('settings.sandbox.allowOut')}</strong>
                  <Button type="button" onClick={() => updateForm((current) => ({ allowOutRows: [...current.allowOutRows, ''] }))}>{t('settings.sandbox.addTarget')}</Button>
                </div>
                {form.allowOutRows.map((row, index) => (
                  <div className="wk-net-row" key={`allow-${index}`}>
                    <Input value={row} placeholder={t('settings.sandbox.allowOutPlaceholder')}
                      onChange={(event) => updateForm((current) => ({ allowOutRows: current.allowOutRows.map((item, i) => i === index ? event.target.value : item) }))} />
                    <Button type="button" aria-label={t('common.delete')} onClick={() => updateForm((current) => ({ allowOutRows: current.allowOutRows.filter((_, i) => i !== index) }))}>×</Button>
                  </div>
                ))}
                <p className="wk-muted text-muted">{t('settings.sandbox.allowOutHelp')}</p>
                {domainAllowNeedsDenyAll(form.allowOutRows, form.denyOutRows, form.denyEgressByDefault)
                  ? <span data-testid="domain-allow-warning"><Status tone="warning">{t('settings.sandbox.domainAllowNeedsDenyAll')}</Status></span> : null}
              </div>
              <div className="wk-sandbox-rows">
                <div className="wk-sandbox-section-row">
                  <strong>{t('settings.sandbox.denyOut')}</strong>
                  <Button type="button" onClick={() => updateForm((current) => ({ denyOutRows: [...current.denyOutRows, ''] }))}>{t('settings.sandbox.addTarget')}</Button>
                </div>
                {form.denyOutRows.map((row, index) => (
                  <div className="wk-net-row" key={`deny-${index}`}>
                    <Input value={row} placeholder={t('settings.sandbox.denyOutPlaceholder')}
                      onChange={(event) => updateForm((current) => ({ denyOutRows: current.denyOutRows.map((item, i) => i === index ? event.target.value : item) }))} />
                    <Button type="button" aria-label={t('common.delete')} onClick={() => updateForm((current) => ({ denyOutRows: current.denyOutRows.filter((_, i) => i !== index) }))}>×</Button>
                  </div>
                ))}
                <p className="wk-muted text-muted">{t('settings.sandbox.denyOutHelp')}</p>
              </div>
            </>) : (
              <label>{t('settings.sandbox.dockerNetworkMode')}
                <Select value={form.docker.network_mode ?? ''} onChange={(event) => setDocker({ network_mode: event.target.value || undefined })}>
                  <option value="">{t('settings.sandbox.dockerNetworkBridge')}</option>
                  <option value="bridge">{t('settings.sandbox.dockerNetworkBridge')}</option>
                  <option value="none">{t('settings.sandbox.dockerNetworkNone')}</option>
                </Select>
              </label>
            )}
            {form.backend === 'docker' ? <p className="wk-muted text-muted">{t('settings.sandbox.dockerNetworkModeHelp')}</p> : null}

            {form.backend === 'cube' ? (
              <div className="wk-sandbox-rows">
                <div className="wk-sandbox-section-row">
                  <strong>{t('settings.sandbox.cubeL7Rules')}</strong>
                  <Button type="button" onClick={() => updateForm((current) => ({ cubeRules: [...current.cubeRules, { key: newCubeRuleKey(), name: '', scheme: 'https', sni: '', host: '', methodsText: '', path: '', deny: false, audit: '', inject: [] }] }))}>{t('settings.sandbox.addRule')}</Button>
                </div>
                <p className="wk-muted text-muted">{t('settings.sandbox.cubeL7RulesHelp')}</p>
                {form.cubeRules.map((rule, index) => (
                  <details className="wk-net-rule" key={rule.key}>
                    <summary>{rule.name.trim() || t('settings.sandbox.ruleUntitled')}</summary>
                    <div className="wk-form-grid grid grid-cols-2 gap-4 max-[720px]:grid-cols-1">
                      <label>{t('settings.sandbox.ruleName')}<Input value={rule.name} placeholder="allow-payment-api" onChange={(event) => updateForm((current) => ({ cubeRules: current.cubeRules.map((item, i) => i === index ? { ...item, name: event.target.value } : item) }))} /></label>
                      <label>{t('settings.sandbox.ruleScheme')}
                        <Select value={rule.scheme} onChange={(event) => updateForm((current) => ({ cubeRules: current.cubeRules.map((item, i) => i === index ? { ...item, scheme: event.target.value } : item) }))}>
                          <option value=""></option><option value="https">https</option><option value="http">http</option>
                        </Select>
                      </label>
                      <label>{t('settings.sandbox.ruleSni')}<Input value={rule.sni} placeholder="api.example.com" onChange={(event) => updateForm((current) => ({ cubeRules: current.cubeRules.map((item, i) => i === index ? { ...item, sni: event.target.value } : item) }))} /></label>
                      <label>{t('settings.sandbox.ruleHost')}<Input value={rule.host} placeholder="api.example.com" onChange={(event) => updateForm((current) => ({ cubeRules: current.cubeRules.map((item, i) => i === index ? { ...item, host: event.target.value } : item) }))} /></label>
                      <label>{t('settings.sandbox.ruleMethods')}<Input value={rule.methodsText} placeholder="POST, GET" onChange={(event) => updateForm((current) => ({ cubeRules: current.cubeRules.map((item, i) => i === index ? { ...item, methodsText: event.target.value } : item) }))} /></label>
                      <label>{t('settings.sandbox.rulePath')}<Input value={rule.path} placeholder="/v1/*" onChange={(event) => updateForm((current) => ({ cubeRules: current.cubeRules.map((item, i) => i === index ? { ...item, path: event.target.value } : item) }))} /></label>
                      <label>{t('settings.sandbox.ruleAction')}
                        <Select value={rule.deny ? 'deny' : 'allow'} onChange={(event) => updateForm((current) => ({ cubeRules: current.cubeRules.map((item, i) => i === index ? { ...item, deny: event.target.value === 'deny' } : item) }))}>
                          <option value="allow">{t('settings.sandbox.ruleAllow')}</option><option value="deny">{t('settings.sandbox.ruleDeny')}</option>
                        </Select>
                      </label>
                      <label>{t('settings.sandbox.ruleAudit')}
                        <Select value={rule.audit} onChange={(event) => updateForm((current) => ({ cubeRules: current.cubeRules.map((item, i) => i === index ? { ...item, audit: event.target.value } : item) }))}>
                          <option value=""></option><option value="metadata">metadata</option><option value="full">full</option><option value="none">none</option>
                        </Select>
                      </label>
                    </div>
                    {!rule.deny ? (
                      <div className="wk-sandbox-rows">
                        <strong>{t('settings.sandbox.ruleInject')}</strong>
                        {rule.inject.map((inject, injectIndex) => (
                          <div className="wk-net-row" key={`inject-${injectIndex}`}>
                            <Input value={inject.header} placeholder={t('settings.sandbox.headerName')}
                              onChange={(event) => updateForm((current) => ({ cubeRules: current.cubeRules.map((item, i) => i === index ? { ...item, inject: item.inject.map((row, j) => j === injectIndex ? { ...row, header: event.target.value } : row) } : item) }))} />
                            <Input type="password" value={inject.secret}
                              placeholder={isStoredNetworkSecretRecoverable(inject, inject.originalRuleName, inject.originalHeader, rule.name, inject.header) ? t('settings.sandbox.secretKeepHint') : t('settings.sandbox.headerValue')}
                              onChange={(event) => updateForm((current) => ({ cubeRules: current.cubeRules.map((item, i) => i === index ? { ...item, inject: item.inject.map((row, j) => j === injectIndex ? { ...row, secret: event.target.value } : row) } : item) }))} />
                            <Input value={inject.format} placeholder="Bearer ${SECRET}"
                              onChange={(event) => updateForm((current) => ({ cubeRules: current.cubeRules.map((item, i) => i === index ? { ...item, inject: item.inject.map((row, j) => j === injectIndex ? { ...row, format: event.target.value } : row) } : item) }))} />
                            <Button type="button" aria-label={t('common.delete')} onClick={() => updateForm((current) => ({ cubeRules: current.cubeRules.map((item, i) => i === index ? { ...item, inject: item.inject.filter((_, j) => j !== injectIndex) } : item) }))}>×</Button>
                          </div>
                        ))}
                        <Button type="button" onClick={() => updateForm((current) => ({ cubeRules: current.cubeRules.map((item, i) => i === index ? { ...item, inject: [...item.inject, { header: '', secret: '', format: '' }] } : item) }))}>{t('settings.sandbox.addHeader')}</Button>
                      </div>
                    ) : null}
                    <div className="wk-list-actions mb-[0.75rem] flex items-center justify-end gap-[0.5rem]">
                      <Button type="button" disabled={index === 0} aria-label={t('settings.sandbox.moveRuleUp')} onClick={() => updateForm((current) => {
                        if (index <= 0) return {};
                        const rows = [...current.cubeRules]; const [row] = rows.splice(index, 1); rows.splice(index - 1, 0, row); return { cubeRules: rows };
                      })}>↑</Button>
                      <Button type="button" disabled={index === form.cubeRules.length - 1} aria-label={t('settings.sandbox.moveRuleDown')} onClick={() => updateForm((current) => {
                        if (index >= current.cubeRules.length - 1) return {};
                        const rows = [...current.cubeRules]; const [row] = rows.splice(index, 1); rows.splice(index + 1, 0, row); return { cubeRules: rows };
                      })}>↓</Button>
                      <Button type="button" aria-label={t('common.delete')} onClick={() => updateForm((current) => ({ cubeRules: current.cubeRules.filter((_, i) => i !== index) }))}>×</Button>
                    </div>
                  </details>
                ))}
              </div>
            ) : null}

            {form.backend === 'e2b' ? (
              <div className="wk-sandbox-rows">
                <div className="wk-sandbox-section-row">
                  <strong>{t('settings.sandbox.e2bHostRules')}</strong>
                  <Button type="button" onClick={() => updateForm((current) => ({ e2bHostRules: [...current.e2bHostRules, { host: '', headers: [] }] }))}>{t('settings.sandbox.addRule')}</Button>
                </div>
                <p className="wk-muted text-muted">{t('settings.sandbox.e2bHostRulesHelp')}</p>
                {form.e2bHostRules.map((rule, index) => (
                  <details className="wk-net-rule" key={`e2b-rule-${index}`}>
                    <summary>{rule.host.trim() || t('settings.sandbox.ruleUntitled')}</summary>
                    <label>{t('settings.sandbox.ruleHost')}<Input value={rule.host} placeholder="api.example.com" onChange={(event) => updateForm((current) => ({ e2bHostRules: current.e2bHostRules.map((item, i) => i === index ? { ...item, host: event.target.value } : item) }))} /></label>
                    {rule.headers.map((header, headerIndex) => (
                      <div className="wk-net-row" key={`header-${headerIndex}`}>
                        <Input value={header.name} placeholder={t('settings.sandbox.headerName')}
                          onChange={(event) => updateForm((current) => ({ e2bHostRules: current.e2bHostRules.map((item, i) => i === index ? { ...item, headers: item.headers.map((row, j) => j === headerIndex ? { ...row, name: event.target.value } : row) } : item) }))} />
                        <Input type="password" value={header.value}
                          placeholder={isStoredNetworkSecretRecoverable(header, header.originalHost, header.originalName, rule.host, header.name) ? t('settings.sandbox.secretKeepHint') : t('settings.sandbox.headerValue')}
                          onChange={(event) => updateForm((current) => ({ e2bHostRules: current.e2bHostRules.map((item, i) => i === index ? { ...item, headers: item.headers.map((row, j) => j === headerIndex ? { ...row, value: event.target.value } : row) } : item) }))} />
                        <Button type="button" aria-label={t('common.delete')} onClick={() => updateForm((current) => ({ e2bHostRules: current.e2bHostRules.map((item, i) => i === index ? { ...item, headers: item.headers.filter((_, j) => j !== headerIndex) } : item) }))}>×</Button>
                      </div>
                    ))}
                    <Button type="button" onClick={() => updateForm((current) => ({ e2bHostRules: current.e2bHostRules.map((item, i) => i === index ? { ...item, headers: [...item.headers, { name: '', value: '' }] } : item) }))}>{t('settings.sandbox.addHeader')}</Button>
                    <Button type="button" aria-label={t('common.delete')} onClick={() => updateForm((current) => ({ e2bHostRules: current.e2bHostRules.filter((_, i) => i !== index) }))}>×</Button>
                  </details>
                ))}
              </div>
            ) : null}
          </section>

          <section className="wk-sandbox-editor-section">
            <div className="wk-sandbox-section-row">
              <h4>{t('settings.sandbox.sectionEnvironment')}</h4>
              <Button type="button" onClick={() => updateForm((current) => ({ envRows: [...current.envRows, { key: '', value: '' }] }))}>{t('settings.sandbox.addRow')}</Button>
            </div>
            <p className="wk-muted text-muted">{t('settings.sandbox.envVarsHint')}</p>
            {form.envRows.length ? form.envRows.map((row, index) => (
              <div className="wk-net-row" key={`env-${index}`}>
                <Input value={row.key} placeholder={t('settings.sandbox.envKey')} className="env-key"
                  onChange={(event) => updateForm((current) => ({ envRows: current.envRows.map((item, i) => i === index ? { ...item, key: event.target.value } : item) }))} />
                <Input type="password" value={row.value} placeholder={row.stored ? t('settings.sandbox.secretKeepHint') : t('settings.sandbox.envValue')} className="env-value"
                  onChange={(event) => updateForm((current) => ({ envRows: current.envRows.map((item, i) => i === index ? { ...item, value: event.target.value } : item) }))} />
                <Button type="button" aria-label={t('common.delete')} onClick={() => updateForm((current) => ({ envRows: current.envRows.filter((_, i) => i !== index) }))}>×</Button>
              </div>
            )) : <p className="wk-muted text-muted">{t('settings.sandbox.noEnvVars')}</p>}
          </section>

          <section className="wk-sandbox-editor-section">
            <h4>{t('settings.sandbox.skillRollout')}</h4>
            <p className="wk-muted text-muted">{t('settings.sandbox.skillRolloutHint')}</p>
            <fieldset>
              <label><Radio name="sandbox-skill-rollout" checked={form.skillRollout === 'next_turn'} onChange={() => updateForm({ skillRollout: 'next_turn' })} /> {t('settings.sandbox.skillRolloutNextTurn')}</label>
              <label><Radio name="sandbox-skill-rollout" checked={form.skillRollout === 'new_session'} onChange={() => updateForm({ skillRollout: 'new_session' })} /> {t('settings.sandbox.skillRolloutNewSession')}</label>
            </fieldset>
          </section>
        </>) : null}

        <div className="wk-list-actions mb-[0.75rem] flex items-center justify-end gap-[0.5rem]">
          {step > 0 ? <Button type="button" onClick={previousStep}>{t('settings.sandbox.back')}</Button> : null}
          {canDeepCheck ? (
            <Button type="button" loading={checking} data-testid="sandbox-deep-check"
              onClick={() => { if (window.confirm(t('settings.sandbox.deepCheckConfirm'))) void runCheck(true); }}>
              {lastCheckWasDeep ? t('settings.sandbox.recheck') : t('settings.sandbox.deepCheck')}
            </Button>
          ) : null}
          <Button type="submit" loading={saving || checking || templatesLoading} disabled={primaryDisabled} data-testid="sandbox-editor-primary">{t(primaryTextKey(stepKey))}</Button>
        </div>
      </form>

      {/* Check result (drawer.vue:712-735). */}
      {checkResult && canDeepCheck ? (
        <div className="wk-check-result" data-testid="sandbox-check-result">
          <p className={checkResult.ok ? 'is-success' : 'is-error'}>
            {checkResult.ok ? '✓ ' : '✕ '}{checkResult.ok ? t('settings.sandbox.checkPassed') : t('settings.sandbox.checkFailed')}
          </p>
          <p className="wk-muted text-muted">{t(checkScopeKey(checkResult))}</p>
          <ul>
            {reportedChecks(checkResult).map((item) => (
              <li key={item.name} className={item.ok === true ? 'ok' : item.ok === false ? 'err' : 'skip'}>
                <span aria-hidden="true">{item.ok === true ? '✓' : item.ok === false ? '✕' : '–'}</span>
                <span className="wk-check-name">{t(`settings.sandbox.checks.${item.name}`, undefined, item.name)}</span>
                {item.latency_ms ? <span className="wk-check-latency">{item.latency_ms} ms</span> : null}
                {item.message ? <span className="wk-check-message">{item.message}</span>
                  : item.reason ? <span className="wk-check-message">{t(`settings.sandbox.skipReasons.${item.reason}`, undefined, item.reason)}</span> : null}
              </li>
            ))}
          </ul>
          {pendingCheckItems(checkResult).length ? (
            <p className="wk-muted text-muted">{t('settings.sandbox.checkPendingHint', { names: pendingCheckItems(checkResult).map((item) => t(`settings.sandbox.checks.${item.name}`, undefined, item.name)).join('、') })}</p>
          ) : null}
        </div>
      ) : null}
    </section>
  );
}

// --- List panel (SandboxSettings.vue) ----------------------------------------

type Props = {
  client: WeKnoraClient;
  role: SettingsRole;
  initialData?: { items: SandboxConfigRecord[]; workspaceScriptsDisabled: boolean };
  /** Vue gates Docker behind the settings.sandbox.docker capability (SandboxSettings.vue:237-247). */
  dockerBackendEnabled?: boolean;
  /** SandboxSettings.vue openSession (341-344): an inventory row click opens
   *  /platform/chat/:id. Optional so embeds without a host navigation keep the
   *  rows inert. Vue closes the drawer first; the host's full-page assign makes
   *  that step a no-op here. */
  onOpenSession?: (sessionId: string) => void;
};

/** SandboxSettings.vue endpointHost (348-356). */
function endpointHost(record: SandboxConfigRecord): string {
  const raw = record.config?.e2b?.api_url || record.config?.cube?.api_url || '';
  if (!raw) return '';
  try { return new URL(raw).host; } catch { return raw; }
}

/** SandboxSettings.vue targetSummary (401-406). */
function targetSummary(record: SandboxConfigRecord): string {
  if (record.sandbox_type === 'docker') return record.config?.docker?.image || '';
  return endpointHost(record);
}

/** SandboxSettings.vue loadSessionTitles (323-330): blank ids are dropped and
 *  the rest deduped before one GET per session. */
export function uniqueSessionIds(ids: readonly string[]): string[] {
  return [...new Set(ids.map((id) => id.trim()).filter(Boolean))];
}

/** SandboxSettings.vue sessionTitle (317-321): a loaded title wins; anything
 *  else reads as the localized "untitled session" label. */
export function sessionTitleText(titles: Record<string, string> | undefined, id: string, untitledLabel: string): string {
  const title = titles?.[id];
  return title ? title : untitledLabel;
}

interface CardWarning { key: string; textKey: string }

/** SandboxSettings.vue buildCardWarnings (426-459). */
function buildCardWarnings(record: SandboxConfigRecord, dockerBackendEnabled: boolean): CardWarning[] {
  const warnings: CardWarning[] = [];
  const config = record.config || {};
  const remote = config.cube || config.e2b;
  if (isRemoteSandboxBackend(record.sandbox_type)) {
    if (!remote?.template_id?.trim()) warnings.push({ key: 'template', textKey: 'settings.sandbox.templateNotConfigured' });
    // Cube API keys are optional; only E2B fails at runtime without one.
    if (record.sandbox_type === 'e2b' && !remote?.api_key?.trim()) warnings.push({ key: 'credential', textKey: 'settings.sandbox.cardCredentialMissing' });
  }
  if (record.sandbox_type === 'docker' && !dockerBackendEnabled) warnings.push({ key: 'docker-disabled', textKey: 'settings.sandbox.dockerDisabledCard' });
  if (record.sandbox_type === 'docker' && !config.docker?.image?.trim()) warnings.push({ key: 'image', textKey: 'settings.sandbox.imageNotConfigured' });
  return warnings;
}

export function SandboxSettingsPanel({ client, role, initialData, dockerBackendEnabled = true, onOpenSession }: Props) {
  const canEdit = roleAtLeast(role, 'admin');
  const locale = useAppLocale();
  const t = useMemo(() => sandboxT(locale), [locale]);
  const [data, setData] = useState(initialData);
  const [loading, setLoading] = useState(initialData === undefined);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [filter, setFilter] = useState<'all' | SandboxBackendType>('all');
  const [editing, setEditing] = useState<{ record: SandboxConfigRecord | null; presetType: string } | null>(null);
  const [busy, setBusy] = useState(false);
  const [deleteAgents, setDeleteAgents] = useState<Record<string, string[]>>({});
  /** SandboxSettings.vue:43-55 — only switching execution OFF pops the warning confirm. */
  const [confirmingDisable, setConfirmingDisable] = useState(false);
  const [inventory, setInventory] = useState<{ record: SandboxConfigRecord; data: SandboxInventory; notice: 'blocked' | 'unverifiable' } | null>(null);
  /** SandboxSettings.vue sessionTitles (315): id -> trimmed title. */
  const [sessionTitles, setSessionTitles] = useState<Record<string, string>>({});

  const load = useCallback(async () => {
    setLoading(true); setError(null);
    try { setData(await client.sandboxConfigurations.list()); }
    catch (cause) { setError(cause instanceof Error && cause.message ? cause.message : t('settings.sandbox.loadFailed')); }
    finally { setLoading(false); }
  }, [client, t]);

  useEffect(() => { if (initialData === undefined) void load(); }, [client, initialData, load]);

  const items = data?.items ?? [];
  const filtered = filter === 'all' ? items : items.filter((item) => item.sandbox_type === filter);
  /** countByType only counts named backends (SandboxSettings.vue:286-287). */
  const countByType = (type: string): number => items.filter((item) => item.sandbox_type === type && isNamedSandboxBackend(item.sandbox_type)).length;
  const isLegacyRecord = (record: SandboxConfigRecord): boolean => !isNamedSandboxBackend(record.sandbox_type);
  const backendLabel = (value: string): string => t(`settings.sandbox.backends.${value}`, undefined, value);

  function openCreate(): void {
    // Vue createPresetType (251-255): the active tab presets the backend.
    if (filter !== 'all' && isNamedSandboxBackend(filter) && !(filter === 'docker' && !dockerBackendEnabled)) {
      setEditing({ record: null, presetType: filter });
      return;
    }
    setEditing({ record: null, presetType: '' });
  }

  /** Vue openEdit refuses legacy rows (SandboxSettings.vue:389-393). */
  function openEdit(record: SandboxConfigRecord): void {
    if (isLegacyRecord(record)) return;
    setEditing({ record, presetType: '' });
  }

  /** SandboxSettings.vue loadSessionTitles (323-339): one GET per unique id in
   *  parallel; a failed lookup resolves to '' and renders as "untitled". */
  const loadSessionTitles = useCallback(async (ids: readonly string[]) => {
    const unique = uniqueSessionIds(ids);
    if (!unique.length) { setSessionTitles({}); return; }
    const next: Record<string, string> = {};
    await Promise.all(unique.map(async (id) => {
      try {
        next[id] = (await client.sessions.get(id)).title.trim();
      } catch { next[id] = ''; }
    }));
    setSessionTitles(next);
  }, [client]);

  /** Vue openInventory (511-528): the titles load before the spinner clears,
   *  so the list never flashes raw ids. */
  async function inspect(record: SandboxConfigRecord): Promise<void> {
    setBusy(true); setError(null); setInventory(null); setSessionTitles({});
    try {
      const data = await client.sandboxConfigurations.inventory(record.id);
      await loadSessionTitles(data.sessionIds);
      setInventory({ record, data, notice: 'blocked' });
    } catch (cause) {
      setError(cause instanceof Error && cause.message ? cause.message : t('settings.sandbox.inventoryFailed'));
    } finally { setBusy(false); }
  }

  /** Vue removeRecord (530-553): conflicts land in the occupancy drawer. */
  async function removeRecord(record: SandboxConfigRecord, force = false): Promise<void> {
    setBusy(true); setError(null);
    try {
      await client.sandboxConfigurations.remove(record.id, undefined, force);
      setInventory(null);
      setNotice(t('settings.sandbox.deleted'));
      await load();
    } catch (cause) {
      const failure = cause as Error & { code?: string; details?: unknown };
      const refusal = parseSandboxConfigurationConflict({ error: { code: failure.code, message: failure.message, data: failure.details } });
      if (refusal?.inventory) {
        setInventory({
          record,
          data: refusal.inventory,
          notice: refusal.code === 'sandbox_inventory_unverifiable' ? 'unverifiable' : 'blocked',
        });
        // Vue showRefusal (555-566) renders the drawer immediately and fills
        // the titles in as their lookups land.
        void loadSessionTitles(refusal.inventory.sessionIds);
      } else {
        setError(failure.message || t('settings.sandbox.deleteFailed'));
      }
    } finally { setBusy(false); }
  }

  /** Vue confirmRemove (503-509): agent names join the confirmation copy. */
  async function requestRemove(record: SandboxConfigRecord): Promise<void> {
    if (!canEdit || busy) return;
    let agents = deleteAgents[record.id];
    if (agents === undefined) {
      try {
        agents = (await client.sandboxConfigurations.inventory(record.id)).agentNames;
      } catch { agents = []; }
      setDeleteAgents((current) => ({ ...current, [record.id]: agents }));
    }
    const message = agents.length
      ? t('settings.sandbox.confirmDeleteWithAgents', {
        name: record.name,
        agents: `${t('settings.sandbox.affectedAgents', { names: agents.join('、') })} `,
      })
      : t('settings.sandbox.confirmDelete', { name: record.name });
    if (!window.confirm(message)) return;
    await removeRecord(record);
  }

  async function forceRemove(): Promise<void> {
    if (!inventory || inventory.notice !== 'unverifiable' || busy) return;
    if (!window.confirm(t('settings.sandbox.forceDeleteConfirm'))) return;
    await removeRecord(inventory.record, true);
  }

  async function setScriptsDisabled(disabled: boolean): Promise<void> {
    if (!canEdit || busy) return;
    setBusy(true); setError(null); setNotice(null);
    try {
      const result = await client.sandboxConfigurations.setWorkspacePolicy(disabled);
      setData((current) => current ? { ...current, workspaceScriptsDisabled: result.workspaceScriptsDisabled } : current);
      setNotice(t(disabled ? 'settings.sandbox.scriptsDisabled' : 'settings.sandbox.scriptsEnabled'));
    } catch (cause) {
      setError(cause instanceof Error && cause.message ? cause.message : t('settings.sandbox.policySaveFailed'));
    } finally { setBusy(false); }
  }

  if (loading) {
    return <Card data-testid="sandbox-settings"><Status>{t('settings.sandbox.loading')}</Status></Card>;
  }

  return (
    <section className="wk-sandbox-settings" data-testid="sandbox-settings">
      <div className="wk-settings-panel-heading flex items-start justify-between gap-4 border-b border-[#eef1f5] pb-4 mb-4 max-[720px]:flex-col">
        <div>
          <h3>{t('settings.sandbox.title')}</h3>
          {/* Page hint popover (SandboxSettings.vue:8-19). */}
          <details className="wk-sandbox-hint relative ml-[6px] inline-block">
            <summary aria-label={t('settings.sandbox.pageHintTitle')}><svg aria-hidden="true" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round"><circle cx="12" cy="12" r="10" /><path d="M12 8h.01M12 12v4" /></svg></summary>
            <div>
              <p className="m-0"><strong>{t('settings.sandbox.pageHintTitle')}</strong></p>
              <p className="wk-muted text-muted m-0">{t('settings.sandbox.pageHint')}</p>
            </div>
          </details>
          <p className="wk-muted text-muted m-0">{t('settings.sandbox.description')}</p>
        </div>
        <div className="wk-list-actions mb-[0.75rem] flex items-center justify-end gap-[0.5rem]">
          <a href={CLUSTER_GUIDE_URL} target="_blank" rel="noopener noreferrer">{t('settings.sandbox.viewClusterGuide')}</a>
          {canEdit ? <Button type="button" onClick={openCreate}>{t('settings.sandbox.addConfig')}</Button> : null}
        </div>
      </div>
      {error ? <Status tone="error">{error}</Status> : null}
      {notice ? <Status tone="success">{notice}</Status> : null}

      {canEdit ? (
        <div className="flex items-center justify-between gap-4 border-b border-[var(--wks-border,#f0f0f0)] py-3">
          <div>
            <strong className="text-[14px] text-[var(--wks-text-primary,#1f2937)]">{t('settings.sandbox.scriptPolicyLabel')}</strong>
            <p className="wk-muted text-muted">{t('settings.sandbox.scriptPolicyDesc')}</p>
          </div>
          {data?.workspaceScriptsDisabled ? (
            <button type="button" role="switch" aria-checked="false"
              className="relative h-[22px] w-10 flex-none cursor-pointer rounded-[11px] border-none bg-[var(--wks-border-strong,#d9d9d9)] p-0 transition-[background] duration-200 ease-[ease] disabled:cursor-not-allowed disabled:opacity-60"
              disabled={busy}
              aria-label={t('settings.sandbox.scriptPolicyLabel')}
              onClick={() => void setScriptsDisabled(false)}>
              <span className="absolute left-[2px] top-[2px] h-[18px] w-[18px] rounded-full bg-surface shadow-[0_1px_2px_rgba(0,0,0,0.2)] transition-[transform] duration-200 ease-[ease]" aria-hidden="true" />
            </button>
          ) : confirmingDisable ? (
            <div role="alertdialog" aria-label={t('settings.sandbox.disableScriptsConfirm')} data-confirm="disable-scripts"
              className="flex items-center gap-[10px] rounded-[8px] border border-[var(--wks-warning-border,#ffe1c7)] bg-[var(--wks-warning-bg,#fff7ec)] px-[12px] py-[8px] text-[13px] text-[var(--wks-text-primary,#1f2937)]">
              <p className="m-0">{t('settings.sandbox.disableScriptsConfirm')}</p>
              <div className="flex gap-2">
                <button type="button" className="cursor-pointer rounded-[6px] border border-[var(--wks-border,#e5e7eb)] bg-surface px-[10px] py-[3px]" disabled={busy} onClick={() => setConfirmingDisable(false)}>{t('common.cancel')}</button>
                <button type="button" className="cursor-pointer rounded-[6px] border-none bg-[var(--wks-danger,#e34d59)] px-[10px] py-1 text-white" disabled={busy}
                  onClick={() => { setConfirmingDisable(false); void setScriptsDisabled(true); }}>{t('settings.sandbox.disableScripts')}</button>
              </div>
            </div>
          ) : (
            <button type="button" role="switch" aria-checked="true"
              className="relative h-[22px] w-10 flex-none cursor-pointer rounded-[11px] border-none bg-[var(--wks-primary,#00a870)] p-0 transition-[background] duration-200 ease-[ease] disabled:cursor-not-allowed disabled:opacity-60"
              disabled={busy}
              aria-label={t('settings.sandbox.scriptPolicyLabel')}
              onClick={() => setConfirmingDisable(true)}>
              <span className="absolute left-[2px] top-[2px] h-[18px] w-[18px] translate-x-[18px] rounded-full bg-surface shadow-[0_1px_2px_rgba(0,0,0,0.2)] transition-[transform] duration-200 ease-[ease]" aria-hidden="true" />
            </button>
          )}
        </div>
      ) : null}

      <nav className="wk-model-tabs flex flex-wrap gap-[0.35rem] border-b border-b-[#edf0f5]" aria-label={t('settings.sandbox.backendType')}>
        <button type="button" className={filter === 'all' ? 'border-b-[#0a8f4c]! text-[13px] text-[#0a8f4c]! [font-weight:650] is-active bg-transparent border-0 border-b-2 border-b-transparent text-[#506078] cursor-pointer px-[0.75rem] py-[0.65rem]' : 'text-[13px] bg-transparent border-0 border-b-2 border-b-transparent text-[#506078] cursor-pointer px-[0.75rem] py-[0.65rem]'} onClick={() => setFilter('all')}>{t('common.all')} ({items.length})</button>
        {SANDBOX_BACKENDS.map((type) => (
          <button type="button" key={type} className={filter === type ? 'border-b-[#0a8f4c]! text-[13px] text-[#0a8f4c]! [font-weight:650] is-active bg-transparent border-0 border-b-2 border-b-transparent text-[#506078] cursor-pointer px-[0.75rem] py-[0.65rem]' : 'text-[13px] bg-transparent border-0 border-b-2 border-b-transparent text-[#506078] cursor-pointer px-[0.75rem] py-[0.65rem]'} onClick={() => setFilter(type)}>
            {backendLabel(type)} ({countByType(type)})
          </button>
        ))}
      </nav>

      {items.length === 0 ? <Status>{t('settings.sandbox.noConfigs')}</Status> : (
        <div className="wk-sandbox-grid">
          {filtered.map((item) => {
            const warnings = buildCardWarnings(item, dockerBackendEnabled);
            const summary = targetSummary(item);
            const cardInteractive = canEdit && !isLegacyRecord(item);
            return (
              <Card
                key={item.id}
                className={`wk-sandbox-card${cardInteractive ? ' cursor-pointer' : ''}`}
                {...(cardInteractive ? {
                  role: 'button',
                  tabIndex: 0,
                  onClick: () => openEdit(item),
                  onKeyDown: (event: KeyboardEvent) => {
                    if (event.key === 'Enter' || event.key === ' ') {
                      event.preventDefault();
                      openEdit(item);
                    }
                  },
                } : {})}
              >
                <div className="wk-sandbox-card-header">
                  <div>
                    <span className="wk-muted text-muted">{backendLabel(item.sandbox_type)}</span>
                    <h4>{item.name}</h4>
                    {isLegacyRecord(item) ? <span className="wk-tag inline-flex items-center shrink-0 py-[1px]! px-[8px]! leading-[1.6]">{t('settings.sandbox.legacyConfig')}</span> : null}
                  </div>
                  {canEdit ? (
                    <div
                      className="wk-list-actions mb-[0.75rem] flex items-center justify-end gap-[0.5rem]"
                      onClick={(event) => event.stopPropagation()}
                      onKeyDown={(event) => event.stopPropagation()}
                    >
                      {!isLegacyRecord(item) ? <Button type="button" onClick={() => openEdit(item)}>{t('common.edit')}</Button> : null}
                      {/* Vue cardMenu offers inventory for cube/e2b only (SandboxSettings.vue:299-301). */}
                      {item.sandbox_type === 'cube' || item.sandbox_type === 'e2b' ? (
                        <Button type="button" disabled={busy} onClick={() => void inspect(item)}>{t('settings.sandbox.viewSandboxes')}</Button>
                      ) : null}
                      <Button type="button" disabled={busy} onClick={() => void requestRemove(item)}>{t('common.delete')}</Button>
                    </div>
                  ) : null}
                </div>
                <p className="wk-muted text-muted">{item.description || ''}</p>
                {summary ? <p className="wk-sandbox-target" title={summary}>{summary}</p> : null}
                {warnings.length ? (
                  <ul className="wk-sandbox-warnings">
                    {warnings.map((warning) => <li key={warning.key}>⚠ <span>{t(warning.textKey)}</span></li>)}
                  </ul>
                ) : null}
              </Card>
            );
          })}
        </div>
      )}

      {inventory ? (
        <div className="wk-sandbox-inventory-overlay fixed inset-0 z-[1250] bg-[rgb(23_32_51_/_35%)]" role="presentation" onMouseDown={(event) => { if (event.target === event.currentTarget) setInventory(null); }}>
        <section className="wk-sandbox-inventory wk-sandbox-inventory-drawer box-border absolute top-0 right-0 bottom-0 left-auto w-[min(400px,100%)] max-h-[100vh] overflow-y-auto border-l border-line bg-surface p-5 shadow-[-18px_0_48px_rgb(23_32_51_/_16%)] animate-[wk-sandbox-inventory-enter_.18s_ease-out] max-[640px]:w-full" role="dialog" aria-modal="true" aria-label={t('settings.sandbox.inventoryTitle')}>
          <div className="wk-settings-panel-heading flex items-start justify-between gap-4 border-b border-[#eef1f5] pb-4 mb-4 max-[720px]:flex-col">
            <div>
              <h3>{t('settings.sandbox.inventoryTitle')}: {inventory.record.name}</h3>
              <p className="wk-muted text-muted m-0">
                {inventory.notice === 'blocked'
                  ? t('settings.sandbox.sandboxesStillLive', { count: inventory.data.sandboxCount })
                  : t('settings.sandbox.inventoryUnverifiableHint')}
              </p>
            </div>
            <Button type="button" onClick={() => setInventory(null)}>{t('common.cancel')}</Button>
          </div>
          <h4>{t('settings.sandbox.inventorySessions')}</h4>
          {inventory.data.sessionIds.length > 0 ? (
            <ul className="wk-list m-0 list-none p-0">
              {/* Vue inventory row (171-178): the raw id stays on the title
                  tooltip; the label shows the resolved session title. Rows are
                  buttons that open the session (openSession, 341-344) only when
                  the host provides onOpenSession; embeds stay inert without it. */}
              {inventory.data.sessionIds.map((id) => {
                const label = (
                  <>
                    <strong title={id}>{sessionTitleText(sessionTitles, id, t('settings.sandbox.inventoryUntitledSession'))}</strong> <span className="wk-muted text-muted font-mono text-[0.8rem]!">{t('settings.sandbox.inventorySessionKind')}</span>
                  </>
                );
                return (
                  <li key={id} className="flex items-baseline justify-between gap-4 border-b border-line-soft py-[0.9rem]">
                    {onOpenSession ? (
                      <button type="button" className="wk-sandbox-inventory-row" onClick={() => { setInventory(null); onOpenSession(id); }}>
                        {label}
                        <span aria-hidden="true" className="font-mono text-[0.8rem] text-muted">›</span>
                      </button>
                    ) : label}
                  </li>
                );
              })}
            </ul>
          ) : <Status>{t('settings.sandbox.inventoryEmpty')}</Status>}
          {inventory.data.agentNames.length > 0 ? <p>{t('settings.sandbox.inventoryAgentsTitle')}: {inventory.data.agentNames.join('、')}</p> : null}
          {inventory.notice === 'unverifiable' ? (
            <Button type="button" disabled={busy} onClick={() => void forceRemove()}>{t('settings.sandbox.forceDelete')}</Button>
          ) : null}
        </section>
        </div>
      ) : null}

      {editing ? (
        <SandboxConfigEditor
          client={client}
          locale={locale}
          record={editing.record}
          presetType={editing.presetType}
          dockerBackendEnabled={dockerBackendEnabled}
          onClose={() => setEditing(null)}
          onSaved={() => { setNotice(t('common.saveSuccess')); void load(); }}
        />
      ) : null}
    </section>
  );
}
