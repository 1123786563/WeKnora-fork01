import type { WeKnoraApi } from "@/api/weknora";

export interface CloudWorkspaceScope {
  readonly backend: string;
  readonly accountId: string;
  readonly tenantId: string;
  readonly generation: number;
}

/**
 * Mobile feature modules receive this client instead of interpreting request
 * headers, tokens, or workspace identifiers as authorization authority.
 */
export class CloudWorkspaceClient {
  readonly scope: CloudWorkspaceScope;

  constructor(scope: CloudWorkspaceScope, private readonly api: WeKnoraApi) {
    this.scope = Object.freeze({ ...scope });
  }

  assertCurrent(generation: number): void {
    if (generation !== this.scope.generation) throw new Error("stale cloud workspace scope");
  }
}
