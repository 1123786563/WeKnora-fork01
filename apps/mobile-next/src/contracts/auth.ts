import { arr, bool, enumOr, isoDate, num, obj, optNum, optStr, str, type Decoder } from "./wire";

// ---- Auth（api-facts §1）----

export interface MembershipWire {
  tenant_id: string;
  role: string;
  billing_role: string | null;
  tenant_name?: string;
}

export interface UserWire {
  id: string;
  email: string;
  display_name: string;
}

// 兼容 user.name / user.display_name / user.username 字段名差异
export const decodeUserLoose: Decoder<UserWire> = (v) => {
  if (!v || typeof v !== "object") throw new Error("user: 期望对象");
  const r = v as Record<string, unknown>;
  return {
    id: String(r.id ?? r.user_id ?? ""),
    email: String(r.email ?? ""),
    display_name: String(r.display_name ?? r.name ?? r.username ?? r.email ?? ""),
  };
};

export interface LoginResponseWire {
  success: boolean;
  user: UserWire;
  active_tenant: { id: string; name: string } | null;
  memberships: MembershipWire[];
  token: string;
  refresh_token: string;
}

export const decodeMembership = (v: unknown): MembershipWire => {
  if (!v || typeof v !== "object") throw new Error("membership: 期望对象");
  const r = v as Record<string, unknown>;
  const tenant = (r.tenant && typeof r.tenant === "object" ? r.tenant : null) as Record<string, unknown> | null;
  return {
    tenant_id: String(r.tenant_id ?? r.tenantId ?? tenant?.id ?? ""),
    role: String(r.role ?? "member"),
    billing_role: optStr()(r.billing_role ?? r.billingRole ?? null),
    tenant_name: tenant?.name != null ? String(tenant.name) : r.tenant_name != null ? String(r.tenant_name) : undefined,
  };
};

export const decodeLoginResponse: Decoder<LoginResponseWire> = (v) => {
  if (!v || typeof v !== "object") throw new Error("login: 期望对象");
  const r = v as Record<string, unknown>;
  const active = (r.active_tenant ?? null) as Record<string, unknown> | null;
  return {
    success: bool(true)(r.success),
    user: decodeUserLoose(r.user),
    active_tenant:
      active && typeof active === "object"
        ? { id: String(active.id ?? ""), name: String(active.name ?? "") }
        : null,
    memberships: arr(decodeMembership)(r.memberships ?? []),
    token: String(r.token ?? r.access_token ?? ""),
    refresh_token: String(r.refresh_token ?? ""),
  };
};

export interface RefreshResponseWire {
  success: boolean;
  access_token: string;
  refresh_token: string;
}

export const decodeRefreshResponse: Decoder<RefreshResponseWire> = (v) => {
  if (!v || typeof v !== "object") throw new Error("refresh: 期望对象");
  const r = v as Record<string, unknown>;
  return {
    success: bool(true)(r.success),
    access_token: String(r.access_token ?? r.token ?? ""),
    refresh_token: String(r.refresh_token ?? ""),
  };
};

// ---- Tenant / Membership 列表 ----
export interface TenantWire {
  id: string;
  name: string;
  role: string;
  billing_role: string | null;
}

export const decodeTenantList: Decoder<TenantWire[]> = (v) => {
  const list = Array.isArray(v) ? v : ((v as Record<string, unknown>)?.tenants as unknown[]) ?? [];
  return list.map((t) => {
    const r = (t ?? {}) as Record<string, unknown>;
    return {
      id: String(r.id ?? ""),
      name: String(r.name ?? ""),
      role: String(r.role ?? "member"),
      billing_role: optStr()(r.billing_role ?? null),
    };
  });
};

// ---- Session（api-facts §3）----
export interface SessionWire {
  id: string;
  title: string;
  agent_id: string | null;
  updated_at: string;
}

export const decodeSession: Decoder<SessionWire> = (v) => {
  if (!v || typeof v !== "object") throw new Error("session: 期望对象");
  const r = v as Record<string, unknown>;
  return {
    id: String(r.id ?? r.session_id ?? ""),
    title: String(r.title ?? r.name ?? "未命名会话"),
    agent_id: optStr()(r.agent_id ?? r.agentId ?? null),
    updated_at: String(r.updated_at ?? r.updatedAt ?? r.last_message_at ?? ""),
  };
};

export interface SessionListWire {
  sessions: SessionWire[];
  total: number;
}

export const decodeSessionList: Decoder<SessionListWire> = (v) => {
  const r = (v ?? {}) as Record<string, unknown>;
  const list = Array.isArray(r.sessions) ? r.sessions : Array.isArray(r.items) ? r.items : Array.isArray(v) ? (v as unknown[]) : [];
  return {
    sessions: arr(decodeSession)(list),
    total: r.total != null ? num()(r.total) : list.length,
  };
};

// ---- Agent（api-facts §4）----
export interface AgentWire {
  id: string;
  name: string;
  description: string;
  builtin: boolean;
}

export const decodeAgent: Decoder<AgentWire> = (v) => {
  const r = (v ?? {}) as Record<string, unknown>;
  return {
    id: String(r.id ?? r.agent_id ?? ""),
    name: String(r.name ?? r.title ?? ""),
    description: String(r.description ?? r.subtitle ?? ""),
    builtin: bool(false)(r.builtin ?? r.is_builtin),
  };
};

export const decodeAgentList: Decoder<AgentWire[]> = (v) => {
  const list = Array.isArray(v) ? v : ((v as Record<string, unknown>)?.agents as unknown[]) ?? [];
  return arr(decodeAgent)(list);
};
