import type { CredentialStore, DeploymentRegistry, StoredCredential } from './ports.ts';
import type { Deployment } from './types.ts';

export function createInMemoryCredentialStore(initial: Record<string, StoredCredential | undefined> = {}): CredentialStore {
  const values = new Map(Object.entries(initial));
  return {
    async read(deployment) { return values.get(deployment); },
    async write(deployment, credential) { values.set(deployment, { ...credential }); },
    async clear(deployment) { values.delete(deployment); },
  };
}

export function createInMemoryDeploymentRegistry(initial: Deployment[] = []): DeploymentRegistry {
  const records = initial.map((deployment) => ({ origin: deployment.origin, label: deployment.label }));
  return {
    async list() { return records.map((deployment) => ({ ...deployment })); },
    async upsert(deployment) {
      // 前移语义：最近登记的实例排最前——与 Task 3 createSecureDeploymentRegistry 的顺序契约一致
      // （同一 DeploymentRegistry 端口下两个 Adapter 的顺序语义必须统一）。
      const next = { origin: deployment.origin, label: deployment.label };
      const rest = records.filter((record) => record.origin !== next.origin);
      records.splice(0, records.length, next, ...rest);
    },
    async remove(origin) {
      const index = records.findIndex((record) => record.origin === origin);
      if (index !== -1) records.splice(index, 1);
    },
  };
}
