// 深链控制器（RW-028，详细设计 §9）：
// 点击通知/深链：受信 scheme → 解析白名单路径 → 认证检查 → 跨空间显式确认 → 页面。
// 安全规则：
//  - URI 不得携带批准动作（action=approve/reject/decide 等一律剥离，仅导航到只读详情）
//  - 通知只是提示；深链后必须重新完成身份与资源授权，不能从通知直接批准
//  - 跨空间资源 → 显式确认（spaces），不静默切换
// 路由参数以对象形式传递（expo-router push({pathname, params})），不做 query 字符串拼接。
import type { AuthStage } from "@/features/auth/AuthController";

export interface DeepLink {
  path: string; // 如 "executions/run_9"（无 scheme、无首尾斜杠）
  params: Record<string, string>; // 剥离危险参数后的安全参数
  raw: string;
}

/** 允许深链直达的页面（资源详情类；登录/空间等流程页也允许） */
const PATH_RULES: Array<{ pattern: RegExp; route: string; key: "resourceId" | "runId" | "sessionId" | null }> = [
  { pattern: /^sessions\/([A-Za-z0-9_-]+)$/, route: "/sessions/[sessionId]", key: "sessionId" },
  { pattern: /^executions\/([A-Za-z0-9_-]+)$/, route: "/executions/[runId]", key: "runId" },
  { pattern: /^interactions\/([A-Za-z0-9_-]+)$/, route: "/interactions/[interactionId]", key: "resourceId" },
  { pattern: /^knowledge\/([A-Za-z0-9_-]+)$/, route: "/knowledge/[id]", key: "resourceId" },
  { pattern: /^connections\/([A-Za-z0-9_-]+)$/, route: "/connections/[id]", key: "resourceId" },
  { pattern: /^artifacts\/([A-Za-z0-9_-]+)$/, route: "/artifacts/[id]", key: "resourceId" },
  { pattern: /^inbox$/, route: "/inbox", key: null },
  { pattern: /^spaces$/, route: "/spaces", key: null },
  { pattern: /^login$/, route: "/login", key: null },
  { pattern: /^new-task$/, route: "/new-task", key: null },
  { pattern: /^voice$/, route: "/voice", key: null },
];

/** 危险动作/凭证参数：出现即剥离（导航降级为只读详情），不执行任何业务动作 */
const FORBIDDEN_ACTION_PARAMS = ["action", "approve", "reject", "decide", "decision", "confirm", "token", "access_token"];

export interface ParsedLink {
  kind: "ok";
  link: DeepLink;
  /** expo-router pathname（动态段占位形式） */
  pathname: string;
  /** 路由参数（含资源 id 与安全 query） */
  routeParams: Record<string, string>;
  strippedParams: string[];
}

export type DeepLinkParseResult =
  | ParsedLink
  | { kind: "rejected"; reason: "unsupported_scheme" | "unknown_path" | "empty" };

const SCHEME_MATCH = /^weknora:\/\/\/?([^?#]*)(?:\?([^#]*))?/;

export function parseDeepLink(url: string): DeepLinkParseResult {
  const trimmed = url.trim();
  if (!trimmed) return { kind: "rejected", reason: "empty" };
  // 受信 scheme：weknora://（app.config scheme）；https 域名链接暂无关联域名配置，拒绝
  const m = trimmed.match(SCHEME_MATCH);
  if (!m) return { kind: "rejected", reason: "unsupported_scheme" };
  const path = (m[1] ?? "").replace(/^\/+|\/+$/g, "");
  const params: Record<string, string> = {};
  if (m[2]) {
    for (const [k, v] of new URLSearchParams(m[2])) params[k] = v;
  }
  const stripped: string[] = [];
  for (const bad of FORBIDDEN_ACTION_PARAMS) {
    if (bad in params) {
      stripped.push(bad);
      delete params[bad];
    }
  }
  for (const rule of PATH_RULES) {
    const hit = path.match(rule.pattern);
    if (!hit) continue;
    const routeParams: Record<string, string> = { ...params };
    if (rule.key && hit[1]) {
      // 资源 id 只允许安全字符（pattern 已限定 [A-Za-z0-9_-]）
      routeParams[rule.key] = hit[1];
    }
    return {
      kind: "ok",
      link: { path, params, raw: trimmed },
      pathname: rule.route,
      routeParams,
      strippedParams: stripped,
    };
  }
  return { kind: "rejected", reason: "unknown_path" };
}

export type DeepLinkDecision =
  | { kind: "navigate"; pathname: string; routeParams: Record<string, string>; stripped?: string[] }
  | { kind: "defer_to_login"; pathname: string } // 未认证：登录后处理（不携带动作）
  | { kind: "pick_space_first"; pathname: string } // 资源属于其他空间：显式确认
  | { kind: "dropped" }; // 解析失败/空链

export interface DeepLinkContext {
  stage: AuthStage["kind"];
  /** 深链资源所属空间；null 表示未知（与当前空间同域或全局页） */
  resourceTenantId?: string | null;
  currentTenantId: string | null;
}

/** 组合入口：解析 + 决策（App 接线用；返回对象式路由参数供 router.push({pathname, params})） */
export function handleDeepLink(url: string, ctx: DeepLinkContext): DeepLinkDecision {
  const parse = parseDeepLink(url);
  if (parse.kind === "rejected") return { kind: "dropped" };
  if (ctx.stage !== "ready") {
    // 未登录/恢复中：进登录流程，登录完成后由 gate 落位（不跳过授权）
    return { kind: "defer_to_login", pathname: parse.pathname };
  }
  // 跨空间资源：显式确认，不静默切换
  if (ctx.resourceTenantId && ctx.currentTenantId && ctx.resourceTenantId !== ctx.currentTenantId) {
    return { kind: "pick_space_first", pathname: parse.pathname };
  }
  return { kind: "navigate", pathname: parse.pathname, routeParams: parse.routeParams, stripped: parse.strippedParams };
}
