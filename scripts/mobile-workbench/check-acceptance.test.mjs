import test from 'node:test';
import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import {
  missingEvidence,
  validateEvidenceRecord,
  validateAcceptance,
  collectGateFailures,
  realDeps,
} from './check-acceptance.mjs';

// Fixed dependencies for the pure validators: the filesystem and git object
// database are injected so every rule below is a decision about THIS task's
// behaviour, never about the machine the test happens to run on.
// `commitExists` (object database) is kept only so tests can construct the
// "object exists but is NOT on the candidate history" situation; the gate
// itself must consult `commitInCandidateHistory`.
const deps = (overrides = {}) => ({
  fileExists: () => true,
  readFile: () => '',
  commitExists: () => true,
  commitInCandidateHistory: () => true,
  repoRoot: '/repo',
  ...overrides,
});

test('mock and skipped runs cannot satisfy native acceptance', () => {
  const rows = [{ kind: 'unit', status: 'pass' }, { kind: 'ios_native', status: 'skipped' }];
  assert.deepEqual(missingEvidence(rows, ['unit', 'ios_native', 'android_native']), ['ios_native', 'android_native']);
});

test('missingEvidence lists each required kind without a passing row', () => {
  const rows = [
    { kind: 'unit', status: 'pass' },
    { kind: 'database', status: 'blocked-env' },
    { kind: 'remote', status: 'pass' },
    { kind: 'billing', status: 'mock' },
  ];
  assert.deepEqual(missingEvidence(rows, ['unit', 'database', 'remote', 'billing', 'security']), [
    'database',
    'billing',
    'security',
  ]);
});

test('a pass row with an empty command is rejected', () => {
  const violations = validateEvidenceRecord(
    { kind: 'unit', status: 'pass', baseline_sha: 'a'.repeat(40), command: '', exit_code: 0, artifact_path: 'docs/evidence/x.md', artifact_hash: 'sha256:1', observed_at: '2026-09-17T00:00:00Z', review_ref: 'task-W06-rereview.md', review_independent: true },
    deps(),
  );
  assert.ok(violations.some((v) => v.includes('command')));
});

test('a pass row with a non-zero exit code is rejected', () => {
  const violations = validateEvidenceRecord(
    { kind: 'unit', status: 'pass', baseline_sha: 'a'.repeat(40), command: 'go test ./...', exit_code: 1, artifact_path: 'docs/evidence/x.md', artifact_hash: 'sha256:1', observed_at: '2026-09-17T00:00:00Z', review_ref: 'task-W06-rereview.md', review_independent: true },
    deps(),
  );
  assert.ok(violations.some((v) => v.includes('exit_code')));
});

test('a pass row whose artifact file is missing is rejected', () => {
  const violations = validateEvidenceRecord(
    { kind: 'recovery', status: 'pass', baseline_sha: 'a'.repeat(40), command: 'node scripts/x.mjs --harness', exit_code: 0, artifact_path: 'docs/evidence/missing.md', artifact_hash: 'sha256:1', observed_at: '2026-09-17T00:00:00Z', review_ref: 'task-W35-review.md', review_independent: true },
    deps({ fileExists: (p) => !p.includes('missing') }),
  );
  assert.ok(violations.some((v) => v.includes('artifact')));
});

test('a pass row whose baseline SHA left the candidate history (stale SHA) is rejected', () => {
  const violations = validateEvidenceRecord(
    { kind: 'unit', status: 'pass', baseline_sha: 'deadbeef', command: 'pnpm test:shared', exit_code: 0, artifact_path: 'docs/evidence/x.md', artifact_hash: 'sha256:1', observed_at: '2026-09-17T00:00:00Z', review_ref: 'task-W06-rereview.md', review_independent: true },
    deps({ commitInCandidateHistory: (sha) => sha !== 'deadbeef' }),
  );
  assert.ok(violations.some((v) => v.includes('baseline_sha')));
});

test('a pass row anchored to a lane-branch or dangling commit (object exists, NOT an ancestor of the candidate HEAD) is rejected', () => {
  const violations = validateEvidenceRecord(
    { kind: 'security', status: 'pass', baseline_sha: '661a7b7c', command: 'go test ./internal/handler -count=1', exit_code: 0, artifact_path: 'docs/evidence/x.md', artifact_hash: 'sha256:1', observed_at: '2026-09-17T00:00:00Z', review_ref: 'task-W13-review.md', review_independent: true },
    // The commit OBJECT exists in the database (cat-file would find it), but
    // it never merged into the candidate: object presence must NOT pass.
    deps({ commitExists: () => true, commitInCandidateHistory: () => false }),
  );
  assert.ok(
    violations.some((v) => v.includes('baseline_sha') && v.includes('ancestor')),
    `expected an ancestor violation, got: ${JSON.stringify(violations)}`,
  );
});

test('skip and mock statuses can never be recorded as accepted evidence', () => {
  for (const status of ['skip', 'skipped', 'mock']) {
    const violations = validateEvidenceRecord(
      { kind: 'ios_native', status, baseline_sha: 'a'.repeat(40), command: 'xcodebuild test', exit_code: 0, artifact_path: 'docs/evidence/x.md', artifact_hash: 'sha256:1', observed_at: '2026-09-17T00:00:00Z', review_ref: 'task-W07-rereview-final2.md', review_independent: true },
      deps(),
    );
    assert.ok(violations.some((v) => v.includes('status')), `status ${status} must be rejected`);
  }
});

test('an implementer-only report cannot back a pass row', () => {
  const noReview = validateEvidenceRecord(
    { kind: 'unit', status: 'pass', baseline_sha: 'a'.repeat(40), command: 'go test ./...', exit_code: 0, artifact_path: 'docs/evidence/x.md', artifact_hash: 'sha256:1', observed_at: '2026-09-17T00:00:00Z', review_ref: '', review_independent: false },
    deps(),
  );
  assert.ok(noReview.some((v) => v.includes('review_ref')));

  const implementerReport = validateEvidenceRecord(
    { kind: 'unit', status: 'pass', baseline_sha: 'a'.repeat(40), command: 'go test ./...', exit_code: 0, artifact_path: 'docs/evidence/x.md', artifact_hash: 'sha256:1', observed_at: '2026-09-17T00:00:00Z', review_ref: 'task-W11-report.md', review_independent: true },
    deps(),
  );
  assert.ok(implementerReport.some((v) => v.includes('review_ref')));
});

test('blocked-env rows stay honest: they never satisfy a required kind and need no fabricated command', () => {
  const violations = validateEvidenceRecord(
    { kind: 'ios_native', status: 'blocked-env', baseline_sha: 'a'.repeat(40), command: '', exit_code: null, artifact_path: 'docs/evidence/mobile-workbench/W36-native-release.md', artifact_hash: 'sha256:1', observed_at: '2026-09-17T00:00:00Z', review_ref: 'task-W36-review.md', review_independent: true, note: 'no signing identity / no ios project' },
    deps(),
  );
  assert.deepEqual(violations, []);
  assert.deepEqual(missingEvidence([{ kind: 'ios_native', status: 'blocked-env' }], ['ios_native']), ['ios_native']);
});

test('validateAcceptance fails a delivered profile with missing kinds', () => {
  const result = validateAcceptance(
    {
      baseline_sha: 'f'.repeat(40),
      evidence: [
        { kind: 'unit', status: 'pass', baseline_sha: 'f'.repeat(40), command: 'pnpm test:shared', exit_code: 0, artifact_path: 'docs/evidence/x.md', artifact_hash: 'sha256:1', observed_at: '2026-09-17T00:00:00Z', review_ref: 'task-W06-rereview.md', review_independent: true },
      ],
      profiles: [{ profile: 'core', release_status: 'delivered', required_kinds: ['unit', 'database', 'ios_native'] }],
    },
    deps(),
  );
  assert.ok(result.missing.core.includes('database'));
  assert.ok(result.missing.core.includes('ios_native'));
});

test('validateAcceptance rejects skip wording in the release report', () => {
  const doc = {
    baseline_sha: 'f'.repeat(40),
    evidence: [],
    profiles: [{ profile: 'core', release_status: 'not_in_release', required_kinds: [] }],
  };
  const result = validateAcceptance(doc, deps({ readFile: (p) => (p.includes('release-report') ? 'delivered; one suite SKIPped' : '') }));
  assert.ok(result.violations.some((v) => v.includes('release-report') && v.toLowerCase().includes('skip')));
});

test('collectGateFailures turns every violation and missing kind into gate failures (non-zero exit)', () => {
  const failures = collectGateFailures({
    violations: ['evidence[0]: command must not be empty'],
    missing: { core: ['database'] },
  });
  assert.deepEqual(failures, ['evidence[0]: command must not be empty', 'profile core missing evidence kinds: database']);
});

test('a fully evidenced delivered profile passes the gate with zero failures', () => {
  const evidence = [
    { kind: 'unit', status: 'pass', baseline_sha: 'f'.repeat(40), command: 'pnpm test:shared', exit_code: 0, artifact_path: 'docs/evidence/a.md', artifact_hash: 'sha256:1', observed_at: '2026-09-17T00:00:00Z', review_ref: 'task-W06-rereview.md', review_independent: true },
    { kind: 'database', status: 'pass', baseline_sha: 'e'.repeat(40), command: 'go test ./internal/application/repository -count=1', exit_code: 0, artifact_path: 'docs/evidence/b.md', artifact_hash: 'sha256:2', observed_at: '2026-09-16T00:00:00Z', review_ref: 'task-W02-rereview.md', review_independent: true },
  ];
  const result = validateAcceptance(
    { baseline_sha: 'f'.repeat(40), evidence, profiles: [{ profile: 'core', release_status: 'delivered', required_kinds: ['unit', 'database'] }] },
    deps(),
  );
  assert.deepEqual(result.violations, []);
  assert.deepEqual(result.missing, {});
  assert.deepEqual(collectGateFailures(result), []);
});

test('a delivered profile with gaps fails the gate; not_in_release is an explicit declaration, not a pass', () => {
  const result = validateAcceptance(
    {
      baseline_sha: 'f'.repeat(40),
      evidence: [
        { kind: 'unit', status: 'pass', baseline_sha: 'f'.repeat(40), command: 'pnpm test:shared', exit_code: 0, artifact_path: 'docs/evidence/a.md', artifact_hash: 'sha256:1', observed_at: '2026-09-17T00:00:00Z', review_ref: 'task-W06-rereview.md', review_independent: true },
      ],
      profiles: [
        // voice still misses W31 (not implemented) — a "delivered" claim can
        // never cover the gap: the gate fails.
        { profile: 'voice', release_status: 'delivered', required_kinds: ['unit', 'ios_native'] },
        // resources declares W28 out of this release: the gap stays visible
        // in the report, but it is an honest staged declaration, not a pass
        // and not a silent drop.
        { profile: 'resources', release_status: 'not_in_release', required_kinds: ['unit', 'android_native'], not_in_release_reason: 'W28 not implemented' },
      ],
    },
    deps(),
  );
  assert.ok(result.missing.voice.includes('ios_native'));
  assert.ok(result.missing.resources.includes('android_native'));
  const failures = collectGateFailures(result);
  assert.ok(failures.some((f) => f.includes('voice')));
  assert.ok(!failures.some((f) => f.includes('resources')));
  // The declaration itself is still verified: a not_in_release profile must
  // carry its reason.
  const silent = validateAcceptance(
    {
      baseline_sha: 'f'.repeat(40),
      evidence: [],
      profiles: [{ profile: 'resources', release_status: 'not_in_release', required_kinds: [] }],
    },
    deps(),
  );
  assert.ok(silent.violations.some((v) => v.includes('not_in_release_reason')));
});

// Builds a throwaway git repository:
//   main: A --- C (HEAD)
//            \
//   lane:      B   (commit object exists, branch ref keeps it alive)
//   dangling:  D   (commit object exists, no ref at all)
function buildLaneRepo() {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'w37-gate-repo-'));
  const git = (...args) => execFileSync('git', ['-C', dir, ...args], { stdio: ['ignore', 'pipe', 'ignore'] });
  const commit = (name) => {
    fs.writeFileSync(path.join(dir, `${name}.txt`), name);
    git('add', '.');
    git('commit', '-q', '-m', name);
  };
  git('init', '-q');
  git('config', 'user.email', 'gate@example.invalid');
  git('config', 'user.name', 'gate test');
  commit('A');
  git('checkout', '-q', '-b', 'lane');
  commit('B');
  const laneSha = git('rev-parse', 'HEAD').toString().trim();
  git('checkout', '-q', '-');
  commit('C');
  const headSha = git('rev-parse', 'HEAD').toString().trim();
  // A dangling commit: parented on HEAD, referenced by nothing.
  const danglingSha = execFileSync(
    'git',
    ['-C', dir, 'commit-tree', `HEAD^{tree}`, '-p', 'HEAD', '-m', 'D'],
    { stdio: ['ignore', 'pipe', 'ignore'] },
  ).toString().trim();
  return { dir, headSha, laneSha, danglingSha };
}

test('realDeps: an object-database existence check is NOT enough — only ancestors of the candidate HEAD count', () => {
  const { dir, headSha, laneSha, danglingSha } = buildLaneRepo();
  try {
    const deps = realDeps(dir);
    // All three commits exist as objects (cat-file finds them)…
    for (const sha of [headSha, laneSha, danglingSha]) {
      execFileSync('git', ['-C', dir, 'cat-file', '-e', `${sha}^{commit}`], { stdio: 'ignore' });
    }
    // …but only the candidate HEAD itself (and its ancestors) passes the gate.
    assert.equal(deps.commitInCandidateHistory(headSha), true);
    assert.equal(deps.commitInCandidateHistory(laneSha), false, 'lane-branch commit must be rejected');
    assert.equal(deps.commitInCandidateHistory(danglingSha), false, 'dangling commit must be rejected');
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
});

test('realDeps: a SHA that is not a commit at all is rejected', () => {
  const { dir } = buildLaneRepo();
  try {
    assert.equal(realDeps(dir).commitInCandidateHistory('0'.repeat(40)), false);
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
});
