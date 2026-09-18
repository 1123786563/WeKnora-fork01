// CFT-S00-T005: view capabilities as a pure projection of server-owned facts.
//
// Sources (verified in T001): the deployment gate WEKNORA_CRAFT_ENABLED
// (zero value = closed) + WEKNORA_CRAFT_KINDS (per-kind opening, default
// ["web"]) live in internal/container craftFeatureGateFromEnv and are
// re-validated by CraftFeatureGate.Allows on EVERY create/run — what follows
// only decides what the UI offers and why, never what the server accepts.
// History reading never depends on the gate: an already-open workbench stays
// viewable (A06 recovery semantics).
export const CRAFT_CAPABILITY_REASONS = {
  gateOff: 'craft_not_enabled_on_this_deployment',
  readOnly: 'session_is_read_only_for_this_user',
} as const;

export interface CraftCapabilityInput {
  /** WEKNORA_CRAFT_ENABLED resolved server-side. */
  gateEnabled: boolean;
  /** WEKNORA_CRAFT_KINDS resolved server-side (empty = nothing open). */
  allowedKinds: readonly string[];
  /** The session-level write permission for this user. */
  canWrite: boolean;
  /** Upload ceiling from server config; null = unknown, never fabricated. */
  maxInputBytes: number | null;
}

export interface CraftViewCapabilities {
  readonly canRead: boolean;
  readonly canWrite: boolean;
  readonly allowedKinds: readonly string[];
  readonly maxInputBytes: number | null;
  readonly disabledReason: string | null;
}

export function craftViewCapabilities(input: CraftCapabilityInput): CraftViewCapabilities {
  const kinds = input.gateEnabled ? [...input.allowedKinds] : [];
  let disabledReason: string | null = null;
  if (!input.gateEnabled) {
    disabledReason = CRAFT_CAPABILITY_REASONS.gateOff;
  } else if (!input.canWrite) {
    disabledReason = CRAFT_CAPABILITY_REASONS.readOnly;
  }
  return {
    canRead: true,
    canWrite: input.gateEnabled && input.canWrite,
    allowedKinds: kinds,
    maxInputBytes: input.maxInputBytes,
    disabledReason,
  };
}

/**
 * Why one artifact kind is not selectable. null = selectable. Disabling
 * projects a reason — it never clears the user's draft (T005 assertion 3).
 */
export function craftKindDisabledReason(kind: string, caps: CraftViewCapabilities): string | null {
  if (caps.disabledReason !== null) return caps.disabledReason;
  if (!caps.allowedKinds.includes(kind)) {
    return `kind_${kind}_is_not_open_on_this_deployment`;
  }
  return null;
}
