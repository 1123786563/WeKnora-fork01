// T15 (#130): the default preview version as a pure rule.
//
// The default seat only ever goes to the newest version whose FOUR web
// checks — build, entry, preview_reachable, page_loaded — each independently
// passed. Preview reachability can never substitute for the actual page
// load, and a version without the four-check evidence (a legacy row, a
// failed or never-observed round) never takes the seat: generated files
// without a usable page stay a draft, not the delivered default.
import type { CraftVersionView } from '@weknora/contracts';

/** The four independently recorded web-gate outcomes of one version. */
export type WebCheckEvidenceFact = NonNullable<CraftVersionView['web_evidence']>;

/** One version candidate for the default seat. */
export interface WebVersionEvidenceFact {
  id: string;
  webEvidence: WebCheckEvidenceFact | null;
  /**
   * Optional ordering witness (creation timestamp of the version, newest
   * wins). When callers supply it, the picker no longer depends on their
   * array order alone: the seat goes to the NEWEST ready version even if the
   * list arrives shuffled, which is exactly what #107's "the newest version
   * passing the four checks becomes the default" requires.
   */
  createdAt?: string;
}

/**
 * A promotion is ready only when all four facts independently passed. A
 * single failed or not_run fact — above all a page load that was never
 * observed — is never ready, whatever the other three say.
 */
export function webCheckEvidenceReady(evidence: WebCheckEvidenceFact | null): boolean {
  if (evidence === null) return false;
  return (
    evidence.build === 'passed' &&
    evidence.entry === 'passed' &&
    evidence.preview_reachable === 'passed' &&
    evidence.page_loaded === 'passed'
  );
}

/**
 * Picks the default preview version: the newest four-check ready one. The
 * documented precondition is a newest-first list; when facts carry createdAt
 * the function enforces the rule itself (max createdAt among ready facts)
 * instead of trusting caller order. A newer version that is not ready never
 * displaces the prior default — it stays out of the seat until its page
 * actually loaded.
 */
export function defaultPreviewVersion(
  versions: readonly WebVersionEvidenceFact[],
): WebVersionEvidenceFact | null {
  let best: WebVersionEvidenceFact | null = null;
  for (const version of versions) {
    if (!webCheckEvidenceReady(version.webEvidence)) continue;
    if (best === null) {
      best = version;
      continue;
    }
    const versionTime = Date.parse(version.createdAt ?? '');
    const bestTime = Date.parse(best.createdAt ?? '');
    if (!Number.isNaN(versionTime) && !Number.isNaN(bestTime)) {
      if (versionTime > bestTime) best = version;
      continue;
    }
    // Without comparable createdAt witnesses the newest-first array order
    // remains the ordering contract.
  }
  return best;
}
