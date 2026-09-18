import {
  createPersonalNodeConnector,
  type PersonalNodeComposition,
} from './personal-node-composition.ts';
import type { PersonalNodeConnector } from './node-connector.ts';

/**
 * The single production entry point used by a mobile/desktop agent runtime.
 * Callers provide deployment-owned Paseo configuration and an OS-backed
 * credential store; they cannot bypass the endpoint and credential checks in
 * createPersonalNodeConnector.
 */
export function createPersonalNodeRuntime(input: PersonalNodeComposition): PersonalNodeConnector {
  return createPersonalNodeConnector(input);
}
