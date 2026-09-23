/** Resource Backend Port（spec §6.3）：WeKnora Adapter 与 in-memory 场景 Adapter 的结构端口。 */
export interface ResourceRemote {
  /**
   * GET /api/v1/agents 语义行投影（见 api-client createMobileResourceRemote）。
   * HTTP 错误契约：以携带可选 status: number 的 Error 抛出（ApiError 结构性满足）。
   */
  availableAgents(accessToken: string): Promise<{ rows: ReadonlyArray<Record<string, unknown>>; disabledOwnAgentIds: ReadonlySet<string> }>;
  knowledgeBases(accessToken: string): Promise<ReadonlyArray<Record<string, unknown>>>;
  connections(accessToken: string): Promise<ReadonlyArray<Record<string, unknown>>>;
}

export interface ResourceShelfPorts {
  remote: ResourceRemote;
  /**
   * 包内 seam：由 createMobileRuntime 铸造（持有当前 scope 凭据与 refresh 单飞），
   * token 不出 mobile-core；browse 对 401 以 { refresh: true } 恰好重取一次。
   */
  accessTokenFor(origin: string, options?: { refresh?: boolean }): Promise<string>;
}
