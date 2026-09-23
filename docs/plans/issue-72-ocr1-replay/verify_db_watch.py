#!/usr/bin/env python3
"""Issue #74 flow verification: DB-side projection checks over the observer
TSV (read-only psql samples taken every 2s while run_lab.py executed).

Rails enum for subscriptions.status in Lago v1.53.0 (verified from the api
container's app/models/subscription.rb STATUSES): 0=pending 1=active
2=terminated 3=canceled 4=incomplete.

Exit codes: 0 PASS, 1 CHECK (an assertion failed), 2 MISSING-EVIDENCE (no
observer TSV found under runs/ — that directory is a git-ignored runtime
artifact, so a fresh clone has none until db_watch.sh is replayed).

ocr-1 replay copy: the fixed script, executed against the
docs/plans/issue-72-ocr1-replay/ evidence directory (t02-decline.json
boundary) and the runs/ observer TSV of the replay.
"""
import glob
import json
import re
import sys
from pathlib import Path

EVID = Path(__file__).resolve().parent
RUNS = Path(__file__).resolve().parents[3] / "deploy/lago-lab/payment-activation/runs"
matches = sorted(glob.glob(str(RUNS / "db-watch-verify-*.tsv")))
if not matches:
    print(f"DB-WATCH: no observer TSV under {RUNS} "
          "(db-watch-verify-*.tsv; runs/ is git-ignored)")
    sys.exit(2)
path = matches[-1]
lines = Path(path).read_text().splitlines()

# Mid-run boundary: everything the observer sampled strictly before the
# decline_control report was recorded is the commercial window; samples at or
# after it belong to the tail (decline endgame + cleanup teardown), where
# canceled/terminated states are the runner's own deletion behavior, not a
# business-state regression (plan Task 2 Step 5: "只看运行中段样本").
boundary = json.loads((EVID / "t02-decline.json").read_text())["recorded_at"][:19]


def midrun(ln):
    ts = ln.split("\t", 1)[0]
    return ts < boundary


def max_succeeded():
    best = 0
    for ln in lines:
        m = re.search(r"payments\[([^\]]*)\]", ln)
        if not m:
            continue
        for part in m.group(1).split(";"):
            if part.startswith("succeeded|"):
                best = max(best, int(part.split("|")[1]))
    return best


def rows_for(tag, only_midrun=True):
    states = []
    for ln in lines:
        if only_midrun and not midrun(ln):
            continue
        m = re.search(r"rows\[([^\]]*)\]", ln)
        if not m:
            continue
        for part in m.group(1).split(";"):
            ext, _, st = part.rpartition("|")
            if ext.startswith("weknora-t02-") and ext.endswith(tag):
                states.append(int(st))
    return states


sub_b = rows_for("-sub-b")
sub_c = rows_for("-sub-c")
sub_a = rows_for("-sub-a")

# sub-b reached active (1) in the commercial window and never regressed there
seen_active = False
sub_b_regress = False
for s in sub_b:
    if s == 1:
        seen_active = True
    elif seen_active and s != 1:
        sub_b_regress = True

# A must be SEEN staying incomplete (4) pre-charge window: the existence
# guard keeps an empty sample set (tag drift / unsampled rows) from passing
# vacuously, mirroring sub_c_seen below.
ok_states_a = len(sub_a) > 0 and set(sub_a) <= {4}
sub_c_seen = len(sub_c) > 0
sub_c_no_active = 1 not in sub_c           # declined charge never activates

print(f"samples: {len(lines)} (mid-run before {boundary}Z)  max_succeeded_payments: {max_succeeded()}")
print(f"sub-a states: {sorted(set(sub_a))} (all incomplete=4: {ok_states_a})")
print(f"sub-b states: {sorted(set(sub_b))} reached_active: {seen_active} regressed: {sub_b_regress}")
print(f"sub-c states: {sorted(set(sub_c))} seen: {sub_c_seen} never_active: {sub_c_no_active}")

ok = (
    max_succeeded() == 1
    and seen_active
    and not sub_b_regress
    and sub_c_seen
    and sub_c_no_active
    and ok_states_a
)
print("DB-WATCH:", "PASS" if ok else "CHECK")
sys.exit(0 if ok else 1)
