export interface HttpRequest {
  method: string;
  url: string;
  headers: Record<string, string>;
  body?: unknown;
  signal?: AbortSignal;
}

/** A platform-owned file reference. URI semantics never cross into DTOs. */
export interface NativeFileSource {
  uri: string;
  name: string;
  type: string;
  size?: number;
}

/** Byte-level upload progress, mirroring the browser XHR upload progress event. */
export interface UploadProgressEvent {
  loaded: number;
  total: number;
}

export interface NativeMultipartFileRequest {
  method: string;
  url: string;
  headers: Record<string, string>;
  file: NativeFileSource;
  fields: Record<string, string>;
  signal?: AbortSignal;
  /**
   * Optional sink for byte-granular upload progress. When provided, browser
   * transports use their XHR upload path; without it they keep the fetch path.
   */
  onProgress?: (progress: UploadProgressEvent) => void;
}

export interface HttpResult {
  status: number;
  headers: Record<string, string>;
  body: unknown;
}

export interface HttpStreamResult {
  status: number;
  headers: Record<string, string>;
  chunks: AsyncIterable<string>;
}

export interface HttpTransport {
  send(request: HttpRequest): Promise<HttpResult>;
  /** Optional binary response path; JSON/native callers can keep using send. */
  sendBinary?(request: HttpRequest): Promise<HttpResult>;
  sendMultipartFile?(request: NativeMultipartFileRequest): Promise<HttpResult>;
  sendStream?(request: HttpRequest): Promise<HttpStreamResult>;
}

export interface RequestScope {
  origin: string;
  userId: string | null;
  tenantId: string | null;
  generation: number;
}

export interface BearerCredential {
  kind: 'bearer';
  accessToken: string;
  refreshToken?: string;
}

export interface EmbedCredential {
  kind: 'embed';
  token: string;
  sessionSig?: string;
  visitorId?: string;
}

export type Credential = BearerCredential | EmbedCredential | { kind: 'anonymous' };

export interface CredentialAdapter {
  read(): Promise<Credential>;
  write(value: Credential): Promise<void>;
  clear(): Promise<void>;
}
