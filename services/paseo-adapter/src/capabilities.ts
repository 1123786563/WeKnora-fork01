/** The capabilities that the adapter is allowed to claim for one pinned daemon. */
export interface PaseoCapabilities {
  create: boolean;
  observe: boolean;
  events: boolean;
  cancel: boolean;
  approval: boolean;
  steer: boolean;
  exportArtifact: boolean;
  lookupByRequest: boolean;
}

export type CapabilityState = 'supported' | 'unavailable' | 'forbidden';

export type CapabilityObservation =
  | { state: 'supported'; method: string }
  | { state: 'unavailable' | 'forbidden'; reason: string };

export type PaseoCapabilityProbe = Record<keyof PaseoCapabilities, CapabilityObservation>;

const CORE_CAPABILITIES = ['create', 'observe', 'events', 'cancel'] as const satisfies readonly (keyof PaseoCapabilities)[];

/**
 * Reject a daemon before dispatch when it cannot create, observe, stream, and cancel.
 * The error code is intentionally stable because the caller maps it to an admission reason.
 */
export function requireCore(capabilities: PaseoCapabilities): void {
  const missing = CORE_CAPABILITIES.filter((capability) => !capabilities[capability]);
  if (missing.length > 0) {
    throw new Error(`PASEO_CORE_UNAVAILABLE:${missing.join(',')}`);
  }
}

/**
 * Preserve probe evidence exactly. No SDK method is inferred from a boolean or a missing field.
 */
export function probeCapabilities(input: PaseoCapabilityProbe): PaseoCapabilityProbe {
  const output = {} as PaseoCapabilityProbe;
  for (const capability of Object.keys(input) as Array<keyof PaseoCapabilities>) {
    const observation = input[capability];
    if (observation.state === 'supported') {
      if (observation.method.trim() === '') {
        throw new Error(`PASEO_INVALID_PROBE:${capability}:supported_requires_method`);
      }
      output[capability] = { state: 'supported', method: observation.method };
      continue;
    }
    if (observation.reason.trim() === '') {
      throw new Error(`PASEO_INVALID_PROBE:${capability}:${observation.state}_requires_reason`);
    }
    output[capability] = { state: observation.state, reason: observation.reason };
  }
  return output;
}

export function capabilitiesFromProbe(probe: PaseoCapabilityProbe): PaseoCapabilities {
  return Object.fromEntries(
    (Object.keys(probe) as Array<keyof PaseoCapabilities>).map((capability) => [
      capability,
      probe[capability].state === 'supported',
    ]),
  ) as unknown as PaseoCapabilities;
}
