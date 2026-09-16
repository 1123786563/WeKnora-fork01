import type { PaseoClient } from '@getpaseo/client';
import type { PaseoPort } from './protocol.ts';
import { BridgeError } from './protocol.ts';

/** Narrow adapter around public @getpaseo/client methods only. */
export function createSdkPort(client: PaseoClient): PaseoPort {
  return {
    supportsCancellation: false,
    async create({ cwd, prompt, provider }, options) {
      if (options?.signal?.aborted) throw new BridgeError('PASEO_CANCEL_UNAVAILABLE');
      try {
        const agent = await client.agents.create({ config: { provider }, cwd, prompt });
        return { id: agent.id };
      } catch {
        throw new BridgeError('PASEO_UNAVAILABLE');
      }
    },
    async observe(id) {
      try {
        const agent = client.agents.ref(id);
        const result = await agent.waitForFinish(0);
        return { state: String(result.status ?? agent.status ?? 'unknown') };
      } catch {
        throw new BridgeError('PASEO_UNAVAILABLE');
      }
    },
    async cancel() {
      // v0.7.2 has no public cancel operation. Internal DaemonClient methods
      // are intentionally not imported or used as a security boundary.
      throw new BridgeError('PASEO_CANCEL_UNAVAILABLE');
    },
  };
}
