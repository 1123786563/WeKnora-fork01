// HTTP 客户端核心（RW-006）：
// - origin 规范化：仅 http/https；拒绝 localhost/环回/私有/保留地址（不可信 origin）
// - 401 单飞刷新一次（共享同一 Promise）；403 不刷新（当前权限不允许）
// - 错误分类为类型化异常；AbortSignal 全链路支持
import { parseErrorBody } from "@/contracts/wire";

export type ApiErrorKind =
  | "network"
  | "unauthorized"
  | "forbidden"
  | "not_found"
  | "conflict"
  | "validation"
  | "server"
  | "cursor_expired"
  | "aborted"
  | "decode";

export class ApiError extends Error {
  constructor(
    public readonly kind: ApiErrorKind,
    message: string,
    public readonly status?: number,
    public readonly code?: string,
    public readonly details?: unknown,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

// ---- origin 规范化与信任边界 ----
export function canonicalOrigin(input: string): string {
  let u: URL;
  try {
    u = new URL(input.trim());
  } catch {
    throw new ApiError("validation", `服务器地址无效：${input}`);
  }
  if (u.protocol !== "https:" && u.protocol !== "http:") {
    throw new ApiError("validation", "仅支持 http/https 服务器地址");
  }
  // http 协议仅在显式 dev 模式放行（本地后端联调）；生产要求 https
  if (u.protocol === "http:" && !__DEV_ALLOW_LOCAL_HTTP__) {
    throw new ApiError("validation", "生产环境仅支持 https 服务器");
  }
  return `${u.protocol}//${u.host}`; // 去路径/查询/碎片，保留协议+端口
}

export function assertTrustedHost(origin: string): void {
  const u = new URL(origin);
  const host = u.hostname.toLowerCase();
  const isIpV4 = /^\d{1,3}(\.\d{1,3}){3}$/.test(host);
  if (
    host === "localhost" ||
    host === "0.0.0.0" ||
    host.endsWith(".local") ||
    host === "[::1]" ||
    host === "::1" ||
    (isIpV4 && (host.startsWith("10.") || host.startsWith("192.168.") || /^172\.(1[6-9]|2\d|3[01])\./.test(host) || host.startsWith("127.")))
  ) {
    if (!__DEV_ALLOW_LOCAL_HTTP__) {
      throw new ApiError("validation", `不可信的服务器地址：${host}`);
    }
  }
}

// dev 显式开关（测试与本地开发）；由 Host 装配时设置
export let __DEV_ALLOW_LOCAL_HTTP__ = false;
export function setAllowLocalHttpForDev(allow: boolean) {
  __DEV_ALLOW_LOCAL_HTTP__ = allow;
}

// ---- 凭证与租户注入接口（platform 层实现）----
export interface CredentialStore {
  read(): Promise<{ access: string; refresh: string } | null>;
  write(creds: { access: string; refresh: string }): Promise<void>;
  clear(): Promise<void>;
}

export interface HttpClientOptions {
  getOrigin(): string;
  getTenantId(): string | null;
  credentials: CredentialStore;
  fetchImpl?: typeof fetch;
  onAuthExpired?: () => void;
  refreshPath?: string;
}

export interface RequestInit2 {
  method?: "GET" | "POST" | "PUT" | "DELETE";
  body?: unknown;
  signal?: AbortSignal;
  headers?: Record<string, string>;
  /** 内部：刷新中重放时标记，避免二次刷新循环 */
  _retry?: boolean;
}

export class HttpClient {
  private opts: HttpClientOptions;
  private fetchImpl: typeof fetch;
  private refreshInFlight: Promise<boolean> | null = null;

  constructor(opts: HttpClientOptions) {
    this.opts = opts;
    this.fetchImpl = opts.fetchImpl ?? (globalThis.fetch as typeof fetch);
  }

  async request<T>(path: string, init: RequestInit2, decode?: (v: unknown) => T): Promise<T> {
    const origin = canonicalOrigin(this.opts.getOrigin());
    assertTrustedHost(origin);
    const url = `${origin}/api/v1${path}`;
    const creds = await this.opts.credentials.read();

    const headers: Record<string, string> = {
      "Content-Type": "application/json",
      ...(init.headers ?? {}),
    };
    if (creds?.access) headers.Authorization = `Bearer ${creds.access}`;
    const tenant = this.opts.getTenantId();
    if (tenant) headers["X-Tenant-ID"] = tenant;

    let res: Response;
    try {
      res = await this.fetchImpl(url, {
        method: init.method ?? "GET",
        headers,
        body: init.body === undefined ? undefined : JSON.stringify(init.body),
        signal: init.signal,
      });
    } catch (e) {
      if ((e as Error).name === "AbortError") throw new ApiError("aborted", "请求已取消");
      throw new ApiError("network", `无法连接服务器（${(e as Error).message}）`);
    }

    if (res.status === 401 && !init._retry) {
      const refreshed = await this.refreshOnce();
      if (refreshed) {
        return this.request<T>(path, { ...init, _retry: true }, decode);
      }
      this.opts.onAuthExpired?.();
      throw new ApiError("unauthorized", "登录已过期，请重新登录");
    }

    let body: unknown = null;
    const text = await res.text();
    if (text) {
      try {
        body = JSON.parse(text);
      } catch {
        body = { raw: text };
      }
    }

    if (!res.ok) {
      const wire = parseErrorBody(body, `请求失败（HTTP ${res.status}）`);
      const kind: ApiErrorKind =
        res.status === 401 ? "unauthorized"
        : res.status === 403 ? "forbidden"
        : res.status === 404 ? "not_found"
        : res.status === 409 ? (wire.code === "cursor_expired" ? "cursor_expired" : "conflict")
        : res.status === 422 || res.status === 400 ? "validation"
        : res.status >= 500 ? "server"
        : "server";
      throw new ApiError(kind, wire.message, res.status, wire.code, wire.details);
    }

    if (!decode) return body as T;
    try {
      const data = body && typeof body === "object" && "data" in (body as Record<string, unknown>) && (body as Record<string, unknown>).data !== undefined
        ? (body as Record<string, unknown>).data
        : body;
      return decode(data);
    } catch (e) {
      if (e instanceof ApiError) throw e;
      throw new ApiError("decode", `响应格式异常：${(e as Error).message}`);
    }
  }

  // 401 → 全局单飞刷新一次；失败返回 false
  private refreshOnce(): Promise<boolean> {
    if (this.refreshInFlight) return this.refreshInFlight;
    this.refreshInFlight = (async () => {
      try {
        const creds = await this.opts.credentials.read();
        if (!creds?.refresh) return false;
        const origin = canonicalOrigin(this.opts.getOrigin());
        const res = await this.fetchImpl(`${origin}/api/v1${this.opts.refreshPath ?? "/auth/refresh"}`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ refreshToken: creds.refresh }),
        });
        if (!res.ok) return false;
        const body = (await res.json()) as Record<string, unknown>;
        const access = String(body.access_token ?? body.token ?? "");
        const refresh = String(body.refresh_token ?? creds.refresh ?? "");
        if (!access) return false;
        await this.opts.credentials.write({ access, refresh });
        return true;
      } catch {
        return false;
      } finally {
        this.refreshInFlight = null;
      }
    })();
    return this.refreshInFlight;
  }

  // SSE/流式传输字节流（platform 层注入实现；此处仅定义端口）
  stream(
    _input: { url: string; headers: Record<string, string>; signal: AbortSignal },
    _onBytes: (chunk: Uint8Array) => void,
  ): Promise<void> {
    throw new Error("stream 由 platform 层注入（expo/fetch 或测试内存实现）");
  }
}
