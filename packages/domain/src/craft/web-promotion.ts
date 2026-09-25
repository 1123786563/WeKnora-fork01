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
 * Picks the default preview version from newest-first versions: the newest
 * four-check ready one. A newer version that is not ready never displaces
 * the prior default — it stays out of the seat until its page actually
 * loaded.
 */
export function defaultPreviewVersion(
  versions: readonly WebVersionEvidenceFact[],
): WebVersionEvidenceFact | null {
  for (const version of versions) {
    if (webCheckEvidenceReady(version.webEvidence)) return version;
  }
  return null;
}
