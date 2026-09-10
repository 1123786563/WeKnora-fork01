export interface HttpRequest {
  method: string;
  url: string;
  headers: Record<string, string>;
  body?: unknown;
  signal?: AbortSignal;
}

export interface HttpResult {
  status: number;
  headers: Record<string, string>;
  body: unknown;
}

export interface HttpTransport {
  send(request: HttpRequest): Promise<HttpResult>;
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
