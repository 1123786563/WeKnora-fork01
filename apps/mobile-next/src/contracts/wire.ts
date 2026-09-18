// 错误 envelope 三形态（api-facts §8）：
// 1) {success:false,error:{code,message,details}}
// 2) {error:"string"}（auth 401/403）
// 3) {success:false,error:"string"}（workbench 启动）
// 统一解析为 ApiWireError。

export interface ApiWireError {
  code: string;
  message: string;
  details?: unknown;
  raw: unknown;
}

export function parseErrorBody(body: unknown, fallbackMessage: string): ApiWireError {
  if (body && typeof body === "object") {
    const b = body as Record<string, unknown>;
    const topCode = typeof b.code === "string" ? b.code : undefined;
    if (typeof b.error === "string") {
      return { code: topCode ?? guessCode(b.error), message: b.error, raw: body };
    }
    if (b.error && typeof b.error === "object") {
      const e = b.error as Record<string, unknown>;
      return {
        code: typeof e.code === "string" ? e.code : guessCode(String(e.message ?? "")),
        message: typeof e.message === "string" ? e.message : fallbackMessage,
        details: e.details,
        raw: body,
      };
    }
    if (typeof b.message === "string") {
      return { code: topCode ?? guessCode(b.message), message: b.message, raw: body };
    }
    if (topCode) {
      return { code: topCode, message: fallbackMessage, raw: body };
    }
  }
  return { code: "unknown", message: fallbackMessage, raw: body };
}

function guessCode(message: string): string {
  const m = message.toLowerCase();
  if (m.startsWith("unauthorized")) return "unauthorized";
  if (m.includes("forbidden") || m.includes("不是该空间") || m.includes("无权限")) return "forbidden";
  if (m.includes("not found") || m.includes("不存在")) return "not_found";
  if (m.includes("conflict") || m.includes("已存在")) return "conflict";
  if (m.includes("cursor_expired")) return "cursor_expired";
  return "unknown";
}

// 成功响应包装：多数 handler 返回裸 JSON 或 {success:true,data}
export function unwrapData<T>(body: unknown, decode: (v: unknown) => T, where: string): T {
  if (body && typeof body === "object" && "data" in (body as Record<string, unknown>)) {
    const data = (body as Record<string, unknown>).data;
    return decode(data);
  }
  try {
    return decode(body);
  } catch (e) {
    throw new WireDecodeError(`${where}: ${(e as Error).message}`, body);
  }
}

export class WireDecodeError extends Error {
  constructor(
    message: string,
    public readonly body: unknown,
  ) {
    super(message);
    this.name = "WireDecodeError";
  }
}

// ---- 基础校验工具（手写轻量 decoder，零依赖）----
export type Decoder<T> = (v: unknown) => T;

export const str = (fallback?: string): Decoder<string> => (v) => {
  if (typeof v === "string") return v;
  if (typeof v === "number" || typeof v === "boolean") return String(v);
  if (fallback !== undefined) return fallback;
  throw new Error(`期望 string，得到 ${typeof v}`);
};

export const optStr = (): Decoder<string | null> => (v) => (v == null ? null : str()(v));

export const num = (): Decoder<number> => (v) => {
  if (typeof v === "number" && Number.isFinite(v)) return v;
  if (typeof v === "string" && v.trim() !== "" && Number.isFinite(Number(v))) return Number(v);
  throw new Error(`期望 number，得到 ${typeof v}`);
};

export const optNum = (): Decoder<number | null> => (v) => (v == null ? null : num()(v));

export const bool = (fallback?: boolean): Decoder<boolean> => (v) => {
  if (typeof v === "boolean") return v;
  if (typeof v === "number") return v !== 0;
  if (typeof v === "string" && (v === "true" || v === "false")) return v === "true";
  if (fallback !== undefined) return fallback;
  throw new Error(`期望 boolean，得到 ${typeof v}`);
};

export const isoDate = (): Decoder<string> => (v) => {
  const s = str()(v);
  if (Number.isNaN(Date.parse(s)) && !/^\d{4}-\d{2}-\d{2}/.test(s)) {
    // 允许非 ISO 的显示型时间串（如 "今天 14:32"），但不允许空串冒充时间
    if (s.trim() === "") throw new Error("期望非空时间字符串");
  }
  return s;
};

export function obj<T>(fields: Record<string, Decoder<unknown>>, name: string): Decoder<T> {
  return (v) => {
    if (!v || typeof v !== "object" || Array.isArray(v)) throw new Error(`${name}: 期望对象`);
    const out: Record<string, unknown> = {};
    for (const [k, d] of Object.entries(fields)) {
      out[k] = d((v as Record<string, unknown>)[k]);
    }
    return out as T;
  };
}

export function arr<T>(item: Decoder<T>): Decoder<T[]> {
  return (v) => {
    if (!Array.isArray(v)) throw new Error(`期望数组，得到 ${typeof v}`);
    return v.map(item);
  };
}

export function recordOf<T>(item: Decoder<T>): Decoder<Record<string, T>> {
  return (v) => {
    if (!v || typeof v !== "object" || Array.isArray(v)) throw new Error("期望字符串键对象");
    const out: Record<string, T> = {};
    for (const [k, val] of Object.entries(v as Record<string, unknown>)) out[k] = item(val);
    return out;
  };
}

// 保留未知枚举原始值：不崩溃、不伪装成已知状态（contracts 模块规则）
export const enumOr = <K extends string>(known: readonly K[], fallback: "unknown"): Decoder<K | "unknown"> => (v) => {
  const s = str()(v);
  return (known as readonly string[]).includes(s) ? (s as K) : fallback;
};
