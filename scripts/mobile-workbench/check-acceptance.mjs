#!/usr/bin/env node
// W37 evidence-gated acceptance CLI for the staged mobile-workbench release.
//
// A release capability may only be claimed "delivered" when its required
// evidence KINDS each have a passing, independently reviewed record whose
// command, exit code, artifact file and baseline SHA all check out — the
// baseline must be an ANCESTOR of the candidate HEAD, so a commit that only
// lives on a lane branch or survives as a dangling object can never anchor
// acceptance evidence. Mock and skipped runs never satisfy a kind, an
// implementer's own report is never a review, and the release report
// itself must not contain skip wording. Every gap — including honest
// not_in_release / pending declarations — stays visible; only an explicit
// not_in_release declaration stops a gap from failing the gate, and that
// declaration itself must carry its reason.
//
// Usage:
//   node scripts/mobile-workbench/check-acceptance.mjs \
//     --acceptance docs/evidence/mobile-workbench/acceptance.json
//   node --test scripts/mobile-workbench/check-acceptance.test.mjs

import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import process from 'node:process';

/** Evidence kinds the gate understands (SDD volume-06 W37 Interfaces). */
export const EVIDENCE_KINDS = [
  'unit',
  'database',
  'backend_model',
  'ios_native',
  'android_native',
  'remote',
  'billing',
  'security',
  'recovery',
];

/** Statuses that can never satisfy a required kind. */
export const NON_ACCEPTING_STATUSES = new Set(['skip', 'skipped', 'mock', 'mocked', 'todo']);

/**
 * Required kinds that have no passing row. Rows whose status is anything but
 * an exact 'pass' (skipped, mock, blocked-env, pending, …) do not count.
 */
export function missingEvidence(rows, required) {
  return required.filter((kind) => !rows.some((row) => row.kind === kind && row.status === 'pass'));
}

function isSha(value) {
  return typeof value === 'string' && /^[0-9a-f]{7,40}$/i.test(value.trim());
}

/**
 * Validates ONE evidence record. `pass` rows carry the full burden of proof
 * (command, exit code, artifact, baseline SHA, independent review); honest
 * non-pass rows (blocked-env, not_in_release, pending) only need to be
 * well-formed declarations, never fabricated proof.
 */
export function validateEvidenceRecord(record, deps, index = 0) {
  const at = `evidence[${index}]`;
  const violations = [];
  if (!record || typeof record !== 'object') return [`${at}: record must be an object`];
  if (!EVIDENCE_KINDS.includes(record.kind)) violations.push(`${at}: unknown kind '${record.kind}'`);

  const status = String(record.status ?? '');
  if (NON_ACCEPTING_STATUSES.has(status)) {
    violations.push(`${at}: status '${status}' can never be recorded as accepted evidence`);
  }
  if (status !== 'pass') {
    // Honest declaration rows: no fabrication requirements. They still must
    // carry their provenance so the report can trace them.
    if (record.baseline_sha !== undefined && !isSha(record.baseline_sha)) {
      violations.push(`${at}: baseline_sha is not a commit SHA`);
    }
    return violations;
  }

  // status === 'pass' — the full gate.
  if (typeof record.command !== 'string' || record.command.trim() === '') {
    violations.push(`${at}: command must be a non-empty string`);
  }
  if (record.exit_code !== 0) {
    violations.push(`${at}: exit_code must be 0 for pass rows (got ${JSON.stringify(record.exit_code)})`);
  }
  if (typeof record.artifact_path !== 'string' || record.artifact_path.trim() === '') {
    violations.push(`${at}: artifact_path is required for pass rows`);
  } else if (!deps.fileExists(path.join(deps.repoRoot ?? '', record.artifact_path))) {
    violations.push(`${at}: artifact file missing: ${record.artifact_path}`);
  }
  if (typeof record.artifact_hash !== 'string' || record.artifact_hash.trim() === '') {
    violations.push(`${at}: artifact_hash is required for pass rows`);
  }
  if (typeof record.observed_at !== 'string' || record.observed_at.trim() === '') {
    violations.push(`${at}: observed_at is required for pass rows`);
  }
  if (!isSha(record.baseline_sha ?? '') || !deps.commitInCandidateHistory(record.baseline_sha)) {
    violations.push(`${at}: baseline_sha '${record.baseline_sha}' is not an ancestor of the candidate HEAD (lane-branch or dangling SHA?)`);
  }
  // Only an INDEPENDENT review backs a pass. An implementer's own task report
  // (task-*-report.md) is never a review.
  const reviewRef = typeof record.review_ref === 'string' ? record.review_ref.trim() : '';
  const implementerReport = /task-[^/]*-report\.md$/i.test(reviewRef) || /task-[^/]*-brief\.md$/i.test(reviewRef);
  if (reviewRef === '') {
    violations.push(`${at}: review_ref is required for pass rows (implementer-only evidence is not acceptance)`);
  } else if (implementerReport || record.review_independent !== true) {
    violations.push(`${at}: review_ref must point to an independent review (got '${reviewRef}', review_independent=${JSON.stringify(record.review_independent)})`);
  } else if (!deps.fileExists(path.join(deps.repoRoot ?? '', reviewRef))) {
    violations.push(`${at}: review file missing: ${reviewRef}`);
  }
  return violations;
}

/**
 * Validates the whole acceptance document:
 *  - every evidence record is well-formed per its status,
 *  - every profile's required kinds are either evidenced or explicitly
 *    declared not_in_release (with a reason) / pending,
 *  - the release report contains no skip wording.
 *
 * Returns { violations, missing, profile_status }. The `missing` map keeps
 * EVERY gap visible (delivered, pending and declared alike) so the staged
 * report cannot lose track of what is not shipped; which of those gaps fail
 * the gate is decided by collectGateFailures from `profile_status`.
 */
export function validateAcceptance(doc, deps) {
  const violations = [];
  const missing = {};
  const profile_status = {};
  if (!doc || typeof doc !== 'object') return { violations: ['acceptance document must be an object'], missing, profile_status };
  if (!isSha(doc.baseline_sha ?? '') || !deps.commitInCandidateHistory(doc.baseline_sha)) {
    violations.push(`baseline_sha '${doc.baseline_sha}' is not an ancestor of the candidate HEAD`);
  }
  const evidence = Array.isArray(doc.evidence) ? doc.evidence : [];
  evidence.forEach((record, index) => violations.push(...validateEvidenceRecord(record, deps, index)));
  const profiles = Array.isArray(doc.profiles) ? doc.profiles : [];
  for (const profile of profiles) {
    if (!profile || typeof profile.release_status !== 'string') {
      violations.push(`profile ${JSON.stringify(profile?.profile)} has no release_status`);
      continue;
    }
    profile_status[profile.profile] = profile.release_status;
    const required = Array.isArray(profile.required_kinds) ? profile.required_kinds : [];
    const gaps = missingEvidence(evidence, required);
    if (gaps.length > 0) missing[profile.profile] = gaps;
    if (profile.release_status === 'not_in_release') {
      if (typeof profile.not_in_release_reason !== 'string' || profile.not_in_release_reason.trim() === '') {
        violations.push(`profile ${profile.profile}: not_in_release requires not_in_release_reason`);
      }
    } else if (profile.release_status !== 'delivered' && profile.release_status !== 'pending') {
      violations.push(`profile ${profile.profile}: unknown release_status '${profile.release_status}'`);
    }
  }
  const reportPath = typeof doc.release_report === 'string' ? doc.release_report : 'docs/evidence/mobile-workbench/release-report.md';
  if (!deps.fileExists(path.join(deps.repoRoot ?? '', reportPath))) {
    violations.push(`release report missing: ${reportPath}`);
  } else {
    const report = deps.readFile(path.join(deps.repoRoot ?? '', reportPath));
    if (/\bskip(?:ped|ping)?\b/i.test(report)) {
      violations.push(`release-report must not contain skip wording (${reportPath})`);
    }
  }
  return { violations, missing, profile_status };
}

/**
 * Gate failures = record violations + missing kinds of every profile that is
 * not an explicit not_in_release declaration. not_in_release gaps stay in
 * `missing` for the staged report but do not fail the gate — the reasoned
 * declaration IS the staged-delivery contract. A delivered or pending
 * profile with gaps fails: pending means "planned for this release, not
 * closed yet", which the gate must not wave through.
 */
export function collectGateFailures(result) {
  const failures = [...result.violations];
  for (const [profile, kinds] of Object.entries(result.missing ?? {})) {
    if (result.profile_status?.[profile] === 'not_in_release') continue;
    failures.push(`profile ${profile} missing evidence kinds: ${kinds.join(', ')}`);
  }
  return failures;
}

/**
 * Real dependency set: filesystem + git history of this worktree. A baseline
 * only counts when it is an ANCESTOR of the candidate (`HEAD` by default):
 * `git merge-base --is-ancestor` returns non-zero for stale SHAs, for commits
 * confined to a lane branch, and for dangling objects alike, so mere
 * presence in the object database (cat-file) is never acceptance-grade.
 */
export function realDeps(repoRoot = process.cwd(), candidate = 'HEAD') {
  return {
    repoRoot,
    fileExists: (p) => fs.existsSync(p),
    readFile: (p) => fs.readFileSync(p, 'utf8'),
    commitInCandidateHistory: (sha) => {
      if (!isSha(sha)) return false;
      try {
        execFileSync('git', ['-C', repoRoot, 'merge-base', '--is-ancestor', sha, candidate], { stdio: 'ignore' });
        return true;
      } catch {
        return false;
      }
    },
  };
}

function parseArgv(argv) {
  const args = { acceptance: 'docs/evidence/mobile-workbench/acceptance.json' };
  for (let i = 0; i < argv.length; i += 1) {
    if (argv[i] === '--acceptance') args.acceptance = argv[i + 1];
  }
  return args;
}

/** CLI entry. Exit 0 only when every delivered claim is fully evidenced. */
export function main(argv = process.argv.slice(2)) {
  const { acceptance } = parseArgv(argv);
  const repoRoot = process.cwd();
  const deps = realDeps(repoRoot);
  const docPath = path.join(repoRoot, acceptance);
  if (!deps.fileExists(docPath)) {
    process.stderr.write(`acceptance document missing: ${acceptance}\n`);
    return 2;
  }
  let doc;
  try {
    doc = JSON.parse(deps.readFile(docPath));
  } catch (error) {
    process.stderr.write(`acceptance document is not valid JSON: ${error.message}\n`);
    return 2;
  }
  const result = validateAcceptance(doc, deps);
  const failures = collectGateFailures(result);
  for (const failure of failures) process.stderr.write(`ACCEPTANCE GATE: ${failure}\n`);
  for (const [profile, kinds] of Object.entries(result.missing)) {
    process.stdout.write(`STAGED REPORT: profile ${profile} not evidenced this release: ${kinds.join(', ')}\n`);
  }
  if (failures.length > 0) {
    process.stderr.write(`ACCEPTANCE GATE: FAILED with ${failures.length} blocking finding(s)\n`);
    return 1;
  }
  process.stdout.write('ACCEPTANCE GATE: PASSED — every delivered claim is independently evidenced\n');
  return 0;
}

if (process.argv[1] && import.meta.url === new URL(`file://${path.resolve(process.argv[1])}`).href) {
  process.exit(main());
}
