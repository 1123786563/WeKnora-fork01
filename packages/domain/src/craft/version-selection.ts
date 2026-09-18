// CFT-S01-T011: version/source selection as pure rules.
//
// The three ids are INDEPENDENT facts (hifi design §5.1):
//   viewVersionId  — which immutable version the panel is displaying
//   baseVersionId  — the editing baseline the next submit continues from
//   workspaceRevision — the CAS counter the server owns
// Viewing moves only the first. A successful restore moves the baseline and
// advances the revision exactly once — and publishes NOTHING (the next
// successful delivery publishes the new version, not the restore itself).
// Published manifests are immutable: no later source selection rewrites a
// version's recorded citations.
export interface CraftVersionFact {
  id: string;
  published: boolean;
  fileCount: number;
  hasSnapshot: boolean;
  citations: readonly string[];
}

export interface CraftVersionSelectionState {
  viewVersionId: string;
  baseVersionId: string;
  workspaceRevision: number;
  versions: readonly CraftVersionFact[];
  /** Server-computed restorable set (complete snapshots, writable, quiescent). */
  restorableVersionIds: readonly string[];
}

export type CraftVersionSelectionAction =
  | { type: 'view'; versionId: string }
  | { type: 'restore'; versionId: string }
  | { type: 'select-sources'; citations: readonly string[] };

export function craftVersionSelection(
  state: CraftVersionSelectionState,
  action: CraftVersionSelectionAction,
): CraftVersionSelectionState {
  switch (action.type) {
    case 'view':
      // Viewing is read-only: baseline, revision and every manifest stay put.
      return state.viewVersionId === action.versionId
        ? state
        : { ...state, viewVersionId: action.versionId };
    case 'restore':
      // The restore itself never mints a version; the baseline moves and the
      // revision advances exactly once (server CAS owns the real bump — this
      // projects the post-restore shape).
      return {
        ...state,
        viewVersionId: action.versionId,
        baseVersionId: action.versionId,
        workspaceRevision: state.workspaceRevision + 1,
        versions: state.versions,
      };
    case 'select-sources':
      // The next round's knowledge pick is FUTURE input; published manifests
      // are immutable and never rewritten by a selection.
      return state;
  }
}

export type CraftSourceAcl = 'granted' | 'revoked';

/**
 * A revoked source stays as a citable PLACEHOLDER — the excerpt/title payload
 * must not render through. Returns 'open' or 'revoked'; never the payload.
 */
export function sourceAccessibility(source: { citationId: string }, acl: CraftSourceAcl): 'open' | 'revoked' {
  return acl === 'revoked' ? 'revoked' : 'open';
}

export interface RestoreEligibilityInput {
  canWrite: boolean;
  hasActiveRun: boolean;
}

export interface RestoreEligibility {
  restorable: boolean;
  downloadable: boolean;
  blockReason: string | null;
}

/**
 * Downloadable ≠ restorable (hifi design §4.6): files are immutable and stay
 * downloadable forever; restore additionally requires write permission, a
 * quiescent workspace and the version being in the server's restorable set
 * (complete snapshot).
 */
export function restoreEligibility(
  state: CraftVersionSelectionState,
  versionId: string,
  input: RestoreEligibilityInput,
): RestoreEligibility {
  const version = state.versions.find((v) => v.id === versionId);
  const downloadable = version !== undefined && version.published && version.fileCount > 0;
  if (version === undefined) {
    return { restorable: false, downloadable, blockReason: 'unknown_version' };
  }
  if (!input.canWrite) {
    return { restorable: false, downloadable, blockReason: 'session_is_read_only_for_this_user' };
  }
  if (input.hasActiveRun) {
    return { restorable: false, downloadable, blockReason: 'an_active_run_holds_the_workspace' };
  }
  if (!state.restorableVersionIds.includes(versionId) || !version.hasSnapshot) {
    return { restorable: false, downloadable, blockReason: 'no_complete_recovery_snapshot' };
  }
  return { restorable: true, downloadable, blockReason: null };
}
