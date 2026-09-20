"""Wiring tests for the T03 lab lifecycle script (``lab.sh``).

Functional tests copy the shared ``deploy/lago`` asset plus the lab into a
sandbox tree and run the real bash script against a stubbed ``docker`` binary,
so delegation, isolation, and read-only treatment of the shared asset are all
verified without Docker and without touching the real worktree files.
"""

import hashlib
import json
import os
import shutil
import stat
import subprocess
import tempfile
import unittest
from pathlib import Path

LAB_DIR = Path(__file__).resolve().parent
REPO_ROOT = LAB_DIR.parent.parent.parent

LAB_SH = LAB_DIR / "lab.sh"
LAGO_DIR = REPO_ROOT / "deploy" / "lago"

# Official Lago sample defaults that generated secrets must never equal
# (mirrors deploy/lago/lago.sh).
SAMPLES = {
    "POSTGRES_PASSWORD": "changeme",
    "SECRET_KEY_BASE": "your-secret-key-base-hex-64",
    "LAGO_ENCRYPTION_PRIMARY_KEY": "your-encryption-primary-key",
    "LAGO_ENCRYPTION_DETERMINISTIC_KEY": "your-encryption-deterministic-key",
    "LAGO_ENCRYPTION_KEY_DERIVATION_SALT": "your-encryption-derivation-salt",
}

DOCKER_STUB = """#!/bin/sh
# Records every invocation as one JSON argv array per line, then exits 0.
python3 -c 'import json,sys; print(json.dumps(sys.argv[1:]))' "$@" >> "$DOCKER_LOG"
exit 0
"""


def parse_env(path):
    values = {}
    for line in Path(path).read_text(encoding="utf-8").splitlines():
        line = line.strip()
        if not line or line.startswith("#") or "=" not in line:
            continue
        key, _, value = line.partition("=")
        values[key.strip()] = value.strip().strip('"')
    return values


def sandbox_repo():
    """Copy deploy/lago and the lab into a temp tree with the same layout."""
    tmp = Path(tempfile.mkdtemp(prefix="t03-lab-"))
    (tmp / "deploy").mkdir()
    shutil.copytree(LAGO_DIR, tmp / "deploy" / "lago",
                    ignore=shutil.ignore_patterns("__pycache__", ".env*", "evidence"))
    shutil.copytree(LAB_DIR.parent, tmp / "deploy" / "lago-lab",
                    ignore=shutil.ignore_patterns("__pycache__", "evidence"))
    return tmp


def digest_tree(root):
    digests = {}
    for path in sorted(Path(root).rglob("*")):
        if path.is_file() and ".env" not in path.name and "__pycache__" not in str(path):
            digests[str(path.relative_to(root))] = hashlib.sha256(
                path.read_bytes()).hexdigest()
    return digests


class LabScriptTestCase(unittest.TestCase):
    def setUp(self):
        self.sandbox = sandbox_repo()
        self.addCleanup(shutil.rmtree, self.sandbox, ignore_errors=True)
        self.lab_sh = self.sandbox / "deploy" / "lago-lab" / "wallet-semantics" / "lab.sh"
        self.env_file = self.sandbox / "deploy" / "lago" / ".env"

    def run_lab(self, *args, env=None, stub_docker=False):
        environment = dict(os.environ)
        environment.pop("LAGO_API_KEY", None)
        if env:
            environment.update(env)
        bin_dir = None
        if stub_docker:
            bin_dir = Path(tempfile.mkdtemp(prefix="t03-bin-"))
            self.addCleanup(shutil.rmtree, bin_dir, ignore_errors=True)
            log = bin_dir / "docker.log"
            (bin_dir / "docker").write_text(DOCKER_STUB.replace("$DOCKER_LOG", str(log)))
            (bin_dir / "docker").chmod(0o755)
            environment["PATH"] = f"{bin_dir}{os.pathsep}{environment['PATH']}"
            self.docker_log = bin_dir / "docker.log"
        result = subprocess.run(
            ["bash", str(self.lab_sh), *args],
            capture_output=True, text=True, env=environment, timeout=120,
        )
        return result

    def docker_calls(self):
        calls = []
        for line in self.docker_log.read_text(encoding="utf-8").splitlines():
            if line.strip():
                calls.append(json.loads(line))
        return calls


class EnvCommandTests(LabScriptTestCase):
    def test_env_writes_isolated_600_env_with_lab_project_ports_urls_and_seeds(self):
        result = self.run_lab("env")
        self.assertEqual(result.returncode, 0, result.stderr)

        self.assertTrue(self.env_file.exists())
        mode = stat.S_IMODE(self.env_file.stat().st_mode)
        self.assertEqual(mode, 0o600)

        values = parse_env(self.env_file)
        self.assertEqual(values["COMPOSE_PROJECT_NAME"], "weknora-lago-75")
        self.assertEqual(values["LAGO_API_PORT"], "48893")
        self.assertEqual(values["LAGO_FRONT_PORT"], "48894")
        self.assertEqual(values["LAGO_API_URL"], "http://127.0.0.1:48893")
        self.assertEqual(values["LAGO_FRONT_URL"], "http://127.0.0.1:48894")

        for key, sample in SAMPLES.items():
            self.assertTrue(values.get(key), f"{key} must not be empty")
            self.assertNotEqual(values[key], sample, f"{key} must differ from the Lago sample")
        self.assertTrue(values.get("LAGO_RSA_PRIVATE_KEY"), "RSA key must be present")

        self.assertEqual(values["LAGO_CREATE_ORG"], "true")
        self.assertEqual(values["LAGO_ORG_USER_EMAIL"], "ops-t03@weknora.local")
        self.assertTrue(values["LAGO_ORG_USER_PASSWORD"])
        self.assertEqual(values["LAGO_ORG_NAME"], "WeKnora T03")
        self.assertTrue(values["LAGO_ORG_API_KEY"])

    def test_env_generates_fresh_random_secrets_per_run(self):
        first = sandbox_repo()
        second = sandbox_repo()
        self.addCleanup(shutil.rmtree, first, ignore_errors=True)
        self.addCleanup(shutil.rmtree, second, ignore_errors=True)
        for tree in (first, second):
            subprocess.run(
                ["bash", str(tree / "deploy" / "lago-lab" / "wallet-semantics" / "lab.sh"), "env"],
                check=True, capture_output=True, timeout=120,
            )
        one = parse_env(first / "deploy" / "lago" / ".env")
        two = parse_env(second / "deploy" / "lago" / ".env")
        for key in ("POSTGRES_PASSWORD", "SECRET_KEY_BASE", "LAGO_ORG_API_KEY",
                    "LAGO_ORG_USER_PASSWORD"):
            self.assertNotEqual(one[key], two[key], f"{key} must be random per run")

    def test_env_refuses_to_overwrite_an_existing_env_file(self):
        self.env_file.write_text("COMPOSE_PROJECT_NAME=marker\n", encoding="utf-8")
        result = self.run_lab("env")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("refusing to overwrite", result.stderr)
        self.assertEqual(self.env_file.read_text(encoding="utf-8"),
                         "COMPOSE_PROJECT_NAME=marker\n")

    def test_env_never_writes_anywhere_else_under_deploy_lago(self):
        before = digest_tree(self.sandbox / "deploy" / "lago")
        self.run_lab("env")
        after = digest_tree(self.sandbox / "deploy" / "lago")
        self.assertEqual(before, after)  # only the git-ignored .env appears


class DelegationTests(LabScriptTestCase):
    def test_up_delegates_to_shared_lago_sh_and_modifies_no_tracked_file(self):
        self.run_lab("env")
        before = digest_tree(self.sandbox / "deploy" / "lago")
        result = self.run_lab("up", stub_docker=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn(["compose", "up", "-d", "--wait"], self.docker_calls())
        self.assertEqual(before, digest_tree(self.sandbox / "deploy" / "lago"))

    def test_status_delegates_to_lago_sh_health_snapshot_with_lab_port(self):
        self.run_lab("env")
        result = self.run_lab("status", stub_docker=True)
        # With a stubbed docker the snapshot is honestly "unavailable" and
        # health.py exits 1; the JSON proves the delegation ran end-to-end.
        self.assertNotEqual(result.returncode, 0)
        snapshot = json.loads(result.stdout)
        self.assertEqual(snapshot["release"], "v1.53.0")
        self.assertEqual(snapshot["overall"], "unavailable")

    def test_down_delegates_to_compose_down_and_preserves_volumes(self):
        self.run_lab("env")
        result = self.run_lab("down", stub_docker=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn(["compose", "down"], self.docker_calls())
        for call in self.docker_calls():
            self.assertNotIn("-v", call)  # volumes must be preserved

    def test_clean_removes_the_whole_project_including_volumes_and_the_env(self):
        self.run_lab("env")
        result = self.run_lab("clean", stub_docker=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        calls = [" ".join(call) for call in self.docker_calls()]
        self.assertTrue(any("down" in call and "-v" in call for call in calls), calls)
        self.assertTrue(any("weknora-lago-75" in call for call in calls), calls)
        self.assertFalse(self.env_file.exists())


class RunCommandTests(LabScriptTestCase):
    def test_run_requires_an_experiment_argument(self):
        result = self.run_lab("run")
        self.assertNotEqual(result.returncode, 0)

    def test_script_never_reads_logs_or_stores_the_api_key(self):
        text = LAB_SH.read_text(encoding="utf-8")
        self.assertNotIn("LAGO_API_KEY", text)

    def test_run_execs_the_python_runner_from_the_lab_directory(self):
        text = LAB_SH.read_text(encoding="utf-8")
        self.assertIn('exec python3 "$LAB_DIR/run_experiment.py"', text)


if __name__ == "__main__":
    unittest.main()
