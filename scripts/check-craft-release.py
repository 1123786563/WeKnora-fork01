#!/usr/bin/env python3
"""Craft release capability gate (O05).

Reads docs/testing/craft/release-evidence.json and refuses a release unless
the evidence is REAL:

  * head must equal the commit being released (default: git HEAD of this
    checkout; override with --commit). A mismatch means the evidence predates
    the tree under release and must be regenerated - never hand-edited.
  * every one of the 26 program tasks must list a dependency SHA that is a
    real ancestor of that commit;
  * every REQUIRED scenario must be status=passed AND its evidence file must
    exist, be non-empty, carry a positive exit marker (EXIT=0 / SPEC_EXIT=0),
    carry the pass markers its kind demands (--- PASS for go tests, "N passed"
    / "N skipped" for browser specs), and contain no failure markers
    (EXIT=<nonzero>, "--- FAIL", "N failed");
  * the default capability releases ONLY the tested PostgreSQL+Docker
    combination; SQLite is a separate capability that must carry its own
    evidence to be enabled; Cube and E2B sandboxes are separate entries that
    stay disabled/waiting without craft-specific evidence;
  * the known external gaps (G4 commercial schema, unwired O02 model gateway,
    unexposed /metrics) must be present in limitations - the gate never lets
    them silently become capabilities.

Usage:
  python3 scripts/check-craft-release.py docs/testing/craft/release-evidence.json
  python3 scripts/check-craft-release.py evidence.json --commit <release-sha>

Exit codes: 0 = release evidence verified; 1 = verification failed (reasons
printed); 2 = usage error.
"""

from __future__ import annotations

import argparse
import json
import re
import subprocess
import sys
from pathlib import Path

REQUIRED_TASKS = [
    "R01", "R02", "R03", "R04", "R05", "R06", "R07",
    "W01", "W02", "W03", "W04", "W05", "W06",
    "C01", "C02", "C03", "C04", "C05", "C06",
    "D01", "D02", "D03",
    "O01", "O02", "O03", "O04",
]

REQUIRED_SCENARIOS = [
    "drill-01-main-service-sigkill",
    "drill-02-oc-crash-snapshot-restore",
    "drill-03-sandbox-deletion-lifecycle",
    "drill-04-storage-failure",
    "drill-05-two-worker-contention-sqlite",
    "drill-05b-two-worker-contention-postgresql",
    "drill-06-browser-offline-reconnect",
    "drill-07-decision-contention",
    "drill-08-late-usage",
    "drill-09-budget-exhausted",
    "drill-10-preview-privilege-escalation",
    "regression-legacy-builtin",
    "regression-backend-packages",
    "regression-shared-ts-web-build",
    "regression-e2e-full-open",
    "regression-e2e-fail-closed",
]

REQUIRED_LIMITATION_KEYWORDS = [
    "commercial_reservations",
    "AutoMigrate",
    "gateway",
    "/metrics",
]

POSITIVE_EXIT = re.compile(r"(?:EXIT|SPEC_EXIT|SHARED_EXIT|TYPECHECK_EXIT|BUILD_EXIT|SESSION_EXIT|MCP_EXIT|RECOVERY_EXIT)=0")
NEGATIVE_EXIT = re.compile(r"(?:EXIT|SPEC_EXIT)=[1-9][0-9]*")
GO_PASS = re.compile(r"--- PASS")
GO_FAIL = re.compile(r"--- FAIL")
E2E_PASS = re.compile(r"\b\d+ (?:passed|skipped)\b")
E2E_FAIL = re.compile(r"\b\d+ (?:failed|flaky)\b")


def git(repo, *args):
    return subprocess.run(
        ["git", "-C", str(repo), *args], capture_output=True, text=True
    )


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("evidence", help="path to release-evidence.json")
    parser.add_argument("--repo", default=None, help="repository root (default: parent of scripts/)")
    parser.add_argument("--commit", default=None, help="commit being released (default: git HEAD)")
    args = parser.parse_args()

    repo = Path(args.repo).resolve() if args.repo else Path(__file__).resolve().parent.parent
    evidence_path = Path(args.evidence)
    if not evidence_path.is_absolute():
        evidence_path = (Path.cwd() / evidence_path).resolve()
    if not evidence_path.is_file():
        print("FAIL: evidence file not found: %s" % evidence_path)
        return 1

    try:
        evidence = json.loads(evidence_path.read_text(encoding="utf-8"))
    except json.JSONDecodeError as exc:
        print("FAIL: evidence is not valid JSON: %s" % exc)
        return 1

    if args.commit:
        expected = args.commit
    else:
        head = git(repo, "rev-parse", "HEAD")
        if head.returncode != 0:
            print("FAIL: cannot resolve git HEAD in %s: %s" % (repo, head.stderr.strip()))
            return 1
        expected = head.stdout.strip()

    failures = []

    def fail(msg):
        failures.append(msg)

    for field in ("head", "dependencies", "commands", "scenarios", "capabilities", "limitations"):
        if field not in evidence:
            fail("schema: missing required field %r" % field)
    if failures:
        for line in failures:
            print("FAIL: %s" % line)
        return 1

    # --- head must equal the commit being released -------------------------
    head = evidence["head"]
    if not isinstance(head, str) or not re.fullmatch(r"[0-9a-f]{40}", head):
        fail("head: %r is not a full 40-hex commit sha" % (head,))
    elif head != expected:
        fail(
            "head: evidence was generated on %s but the commit under release is %s; "
            "regenerate the evidence on the release commit (never hand-edit head)" % (head, expected)
        )

    # --- dependency SHAs: all 26 tasks, each an ancestor of the release ----
    dependencies = evidence["dependencies"]
    if not isinstance(dependencies, dict) or not dependencies:
        fail("dependencies: must be a non-empty object of task -> sha")
    else:
        for task in REQUIRED_TASKS:
            if task not in dependencies:
                fail("dependencies: missing SHA for task %s" % task)
        for task, sha in sorted(dependencies.items()):
            if not isinstance(sha, str) or not re.fullmatch(r"[0-9a-f]{7,40}", sha):
                fail("dependencies: %s sha %r is not a commit sha" % (task, sha))
                continue
            check = git(repo, "merge-base", "--is-ancestor", sha, expected)
            if check.returncode != 0:
                fail("dependencies: %s sha %s is not an ancestor of the release commit %s" % (task, sha, expected))

    # --- commands: the runnable command list must be non-empty strings -----
    commands = evidence["commands"]
    if not isinstance(commands, list) or not commands or not all(
        isinstance(c, str) and c.strip() for c in commands
    ):
        fail("commands: must be a non-empty list of runnable command strings")

    # --- scenarios: required ones passed with real evidence files ---------
    scenarios = evidence["scenarios"]
    if not isinstance(scenarios, list) or not scenarios:
        fail("scenarios: must be a non-empty list")
    else:
        by_id = {}
        for scenario in scenarios:
            if not isinstance(scenario, dict) or "id" not in scenario:
                fail("scenarios: entry without an id")
                continue
            by_id[scenario["id"]] = scenario
        for scenario_id in REQUIRED_SCENARIOS:
            scenario = by_id.get(scenario_id)
            if scenario is None:
                fail("scenarios: required scenario %s missing" % scenario_id)
                continue
            status = scenario.get("status")
            if status != "passed":
                fail("scenarios: %s status=%r (required scenarios must be passed; not_run never passes the gate)" % (scenario_id, status))
                continue
            evidence_rel = scenario.get("evidence")
            if not isinstance(evidence_rel, str) or not evidence_rel.strip():
                fail("scenarios: %s has no evidence path" % scenario_id)
                continue
            evidence_file = repo / evidence_rel
            if not evidence_file.is_file():
                fail("scenarios: %s evidence file missing: %s" % (scenario_id, evidence_rel))
                continue
            text = evidence_file.read_text(encoding="utf-8", errors="replace")
            if len(text.strip()) == 0:
                fail("scenarios: %s evidence file is empty: %s" % (scenario_id, evidence_rel))
                continue
            if not POSITIVE_EXIT.search(text):
                fail("scenarios: %s evidence has no positive exit marker (EXIT=0); a hand-written summary is not run evidence" % scenario_id)
            if NEGATIVE_EXIT.search(text):
                fail("scenarios: %s evidence contains a non-zero exit marker" % scenario_id)
            kind = scenario.get("kind", "go")
            if kind == "go":
                if not GO_PASS.search(text):
                    fail("scenarios: %s (go) evidence has no '--- PASS' marker" % scenario_id)
                if GO_FAIL.search(text):
                    fail("scenarios: %s (go) evidence contains FAIL output" % scenario_id)
            elif kind == "shell":
                # A shell-level regression (build/typecheck/package): the exit
                # markers above already carry the verdict.
                pass
            elif kind in ("e2e", "matrix"):
                if not E2E_PASS.search(text):
                    fail("scenarios: %s (%s) evidence has no 'N passed/skipped' marker" % (scenario_id, kind))
                if E2E_FAIL.search(text):
                    fail("scenarios: %s (%s) evidence contains failed/flaky tests" % (scenario_id, kind))
            else:
                fail("scenarios: %s unknown kind %r (go|e2e|matrix|shell)" % (scenario_id, kind))

    # --- capabilities -------------------------------------------------------
    capabilities = evidence["capabilities"]
    if not isinstance(capabilities, list) or not capabilities:
        fail("capabilities: must be a non-empty list")
    else:
        by_name = {}
        for capability in capabilities:
            if not isinstance(capability, dict) or "name" not in capability:
                fail("capabilities: entry without a name")
                continue
            by_name[capability["name"]] = capability
        core = by_name.get("core")
        if core is None:
            fail("capabilities: the default 'core' entry is missing")
        else:
            if core.get("status") != "enabled":
                fail("capabilities: core must be enabled for a release")
            providers = core.get("providers") or []
            if "postgresql" not in providers or "docker" not in providers:
                fail("capabilities: core may only enable the TESTED postgresql+docker combination; got providers=%r" % (providers,))
        for name in ("sqlite", "cube", "e2b"):
            entry = by_name.get(name)
            if entry is None:
                fail("capabilities: separate capability entry %r is missing" % name)
                continue
            status = entry.get("status")
            if status not in ("enabled", "disabled", "waiting"):
                fail("capabilities: %s status %r is not enabled/disabled/waiting" % (name, status))
                continue
            evidence_list = entry.get("evidence") or []
            if status == "enabled":
                if not evidence_list:
                    fail("capabilities: %s is enabled without evidence - disable it or keep it waiting" % name)
                for rel in evidence_list:
                    if not (repo / rel).is_file():
                        fail("capabilities: %s evidence file missing: %s" % (name, rel))
            else:
                reason = entry.get("reason")
                if not isinstance(reason, str) or not reason.strip():
                    fail("capabilities: %s is %s without an explicit reason" % (name, status))

    # --- limitations: the known external gaps must stay visible -------------
    limitations = evidence["limitations"]
    if not isinstance(limitations, list) or not limitations or not all(
        isinstance(item, str) and item.strip() for item in limitations
    ):
        fail("limitations: must be a non-empty list of strings")
    else:
        joined = "\n".join(limitations)
        for keyword in REQUIRED_LIMITATION_KEYWORDS:
            if keyword not in joined:
                fail("limitations: the known external gap %r is not recorded - release limitations must reflect it" % keyword)

    # --- facts / release mode (mirrors internal/craft CanRelease) ----------
    facts = evidence.get("facts")
    release_mode = evidence.get("release_mode", "controlled")
    if facts is not None:
        axes = [
            "Web", "Permissions", "Cancel", "Reconnect", "Recovery",
            "Versioning", "Isolation", "Quota", "Usage",
        ]
        for axis in axes:
            if facts.get(axis) is not True:
                fail("facts: axis %s must be true for any release" % axis)
        if release_mode == "paid":
            if facts.get("Billing") is not True:
                fail("facts: a paid release requires the Billing fact")
            commercial = next(
                (c for c in capabilities if isinstance(c, dict) and c.get("name") == "commercial"), None
            )
            if commercial is None or commercial.get("status") != "enabled":
                fail("facts: a paid release requires the commercial capability to be enabled")
        else:
            if facts.get("Billing") is True:
                fail("facts: Billing=true is only meaningful with real commercial evidence; keep it false for a controlled release")

    if failures:
        print("craft release gate: %d problem(s):" % len(failures))
        for line in failures:
            print("  FAIL: %s" % line)
        return 1

    print("craft release gate: evidence verified")
    print("  head: %s" % head)
    print("  dependencies: %d task SHAs, all ancestors of the release commit" % len(REQUIRED_TASKS))
    print("  scenarios: %d required, all passed with real evidence files" % len(REQUIRED_SCENARIOS))
    print("  capabilities: core=postgresql+docker (tested); sqlite/cube/e2b tracked separately")
    print("  limitations: %d recorded (incl. G4 commercial gap, unwired O02 gateway, /metrics)" % len(limitations))
    print("  release mode: %s (paid=%s)" % (release_mode, "yes" if release_mode == "paid" else "no"))
    return 0


if __name__ == "__main__":
    sys.exit(main())
