import { CloudWorkspaceClient } from "@/cloud-workspace/CloudWorkspaceClient";
import type { WeKnoraApi } from "@/api/weknora";

describe("CloudWorkspaceClient", () => {
  it("rejects a delayed old-generation result after a trusted tenant switch", () => {
    const api = {} as WeKnoraApi;
    const client = new CloudWorkspaceClient(
      { backend: "https://api", accountId: "u1", tenantId: "t1", generation: 4 },
      api,
    );

    expect(() => client.assertCurrent(5)).toThrow("stale cloud workspace scope");
    expect(client.scope).toEqual({ backend: "https://api", accountId: "u1", tenantId: "t1", generation: 4 });
  });

  it("keeps an immutable scope snapshot", () => {
    const scope = { backend: "https://api", accountId: "u1", tenantId: "t1", generation: 4 };
    const client = new CloudWorkspaceClient(scope, {} as WeKnoraApi);

    scope.tenantId = "t2";

    expect(client.scope.tenantId).toBe("t1");
  });
});
