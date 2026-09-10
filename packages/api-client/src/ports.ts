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
