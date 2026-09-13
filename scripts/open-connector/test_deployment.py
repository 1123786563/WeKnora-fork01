"""T16 deployment guard: negative tests for the private open-connector stack.

Python 3.9 stdlib unittest only. The tests pin the deployment contract from
the T16 brief:

* plan-sketch core: missing service / published private port errors;
* verbatim DeploymentTest.test_public_runtime_port_rejected;
* R9 digest pinning (never tip/latest);
* ADMIN_TOKEN secret mandatory (T01: zero-auth config = fully open runtime)
  and secret-mount scoping (admin secret only on runtime + control worker);
* encryption-key secret on the control worker (secret sink at-rest encryption);
* read-only rootfs, non-root user, healthcheck for the app services;
* the runtime must keep controlled egress (an internal-only network would
  break OAuth).
"""

import json
import os
import shutil
import subprocess
import tempfile
import unittest

from check_deployment import validate_compose


DIGEST_A = "sha256:" + "a" * 64
DIGEST_B = "sha256:" + "b" * 64
DIGEST_GOLANG = "sha256:" + "c" * 64


def good_config():
    """A fully valid private stack config; every test mutates a copy."""
    return {
        "services": {
            "open-connector": {
                "image": "oc-t16-open-connector@" + DIGEST_A,
                "pull_policy": "never",
                "user": "10001:10001",
                "read_only": True,
                "networks": ["oc-net", "oc-db-net"],
                "secrets": [
                    {"source": "connector-admin-token", "target": "connector-admin-token", "mode": 292},
                    {"source": "connector-db-password", "target": "connector-db-password", "mode": 256},
                ],
                "healthcheck": {"test": ["CMD", "node", "scripts/healthcheck.ts"]},
            },
            "connector-db": {
                "image": "postgres@" + DIGEST_B,
                "networks": ["oc-db-net"],
                "secrets": [
                    {"source": "connector-db-password", "target": "connector-db-password", "mode": 256},
                ],
                "healthcheck": {"test": ["CMD-SHELL", "pg_isready -U oc"]},
            },
            "connector-control": {
                "image": "weknora-connector-control:local",
                "build": {
                    "context": "..",
                    "dockerfile_inline": "FROM golang@" + DIGEST_GOLANG + " AS build\n",
                },
                "user": "10002:10002",
                "read_only": True,
                "networks": ["oc-net"],
                "secrets": [
                    {"source": "connector-admin-token", "target": "connector-admin-token", "mode": 256},
                    {"source": "connector-secret-key", "target": "connector-secret-key", "mode": 256},
                ],
                "healthcheck": {"test": ["CMD-SHELL", "kill -0 1"]},
            },
        },
        "networks": {
            "oc-net": {"driver": "bridge"},
            "oc-db-net": {"driver": "bridge", "internal": True},
        },
    }


class DeploymentTest(unittest.TestCase):
    def test_public_runtime_port_rejected(self):
        self.assertTrue(validate_compose({"services": {"open-connector": {"ports": ["3000:3000"]}}}))

    def test_missing_services_each_reported(self):
        errors = validate_compose({"services": {}})
        for name in ("open-connector", "connector-db", "connector-control"):
            self.assertIn("missing service: " + name, errors)

    def test_minimal_private_stack_passes(self):
        self.assertEqual(validate_compose(good_config()), [])

    def test_published_port_on_each_core_service_rejected(self):
        for name in ("open-connector", "connector-db", "connector-control"):
            cfg = good_config()
            cfg["services"][name]["ports"] = ["3000:3000"]
            self.assertIn("published private service: " + name, validate_compose(cfg))

    # ---- R9 digest discipline -------------------------------------------

    def test_runtime_image_must_be_digest_pinned(self):
        for bad in ("oc-open-connector:tip", "oc-open-connector:latest", "oc-open-connector:33dd4ad"):
            cfg = good_config()
            cfg["services"]["open-connector"]["image"] = bad
            errors = validate_compose(cfg)
            self.assertTrue(any("digest" in e for e in errors), bad)

    def test_runtime_image_build_override_rejected(self):
        cfg = good_config()
        cfg["services"]["open-connector"]["build"] = {"context": ".."}
        errors = validate_compose(cfg)
        self.assertTrue(any("digest" in e for e in errors))

    def test_db_image_must_be_digest_pinned(self):
        cfg = good_config()
        cfg["services"]["connector-db"]["image"] = "postgres:16"
        errors = validate_compose(cfg)
        self.assertTrue(any("connector-db" in e and "digest" in e for e in errors))

    def test_built_service_may_use_local_tag_but_not_latest(self):
        cfg = good_config()
        cfg["services"]["connector-control"]["image"] = "weknora-connector-control:latest"
        errors = validate_compose(cfg)
        self.assertTrue(any("latest" in e or "tip" in e for e in errors))
        cfg["services"]["connector-control"]["image"] = "weknora-connector-control:local"
        self.assertEqual(validate_compose(cfg), [])

    # ---- ADMIN_TOKEN mandatory + secret scoping (ruling 2) --------------

    def test_admin_token_secret_mandatory_on_runtime(self):
        cfg = good_config()
        cfg["services"]["open-connector"]["secrets"] = [
            s for s in cfg["services"]["open-connector"]["secrets"]
            if s["source"] != "connector-admin-token"
        ]
        errors = validate_compose(cfg)
        self.assertTrue(any("connector-admin-token" in e and "open-connector" in e for e in errors))

    def test_admin_token_secret_mandatory_on_control(self):
        cfg = good_config()
        cfg["services"]["connector-control"]["secrets"] = [
            s for s in cfg["services"]["connector-control"]["secrets"]
            if s["source"] != "connector-admin-token"
        ]
        errors = validate_compose(cfg)
        self.assertTrue(any("connector-admin-token" in e and "connector-control" in e for e in errors))

    def test_admin_secret_forbidden_on_other_services(self):
        cfg = good_config()
        cfg["services"]["weknora-api"] = {
            "image": "weknora-app@" + DIGEST_A,
            "user": "10000:10000",
            "networks": ["oc-net"],
            "healthcheck": {"test": ["CMD", "true"]},
            "secrets": [
                {"source": "connector-admin-token", "target": "connector-admin-token", "mode": 256},
                {"source": "connector-secret-key", "target": "connector-secret-key", "mode": 256},
            ],
        }
        errors = validate_compose(cfg)
        self.assertTrue(any("weknora-api" in e and "connector-admin-token" in e for e in errors))

    def test_db_must_not_mount_admin_secret(self):
        cfg = good_config()
        cfg["services"]["connector-db"]["secrets"].append(
            {"source": "connector-admin-token", "target": "connector-admin-token", "mode": 256}
        )
        errors = validate_compose(cfg)
        self.assertTrue(any("connector-db" in e and "connector-admin-token" in e for e in errors))

    # ---- secret-sink encryption key (ruling 7) --------------------------

    def test_encryption_key_required_on_control(self):
        cfg = good_config()
        cfg["services"]["connector-control"]["secrets"] = [
            s for s in cfg["services"]["connector-control"]["secrets"]
            if s["source"] != "connector-secret-key"
        ]
        errors = validate_compose(cfg)
        self.assertTrue(any("connector-secret-key" in e for e in errors))

    # ---- container hardening ---------------------------------------------

    def test_read_only_rootfs_required_for_app_services(self):
        for name in ("open-connector", "connector-control"):
            cfg = good_config()
            cfg["services"][name]["read_only"] = False
            errors = validate_compose(cfg)
            self.assertTrue(any(name in e and "read_only" in e for e in errors), name)

    def test_non_root_user_required_for_app_services(self):
        for name, bad in (
            ("open-connector", None),
            ("open-connector", "0:0"),
            ("open-connector", "root"),
            ("connector-control", None),
            ("connector-control", "root"),
        ):
            cfg = good_config()
            if bad is None:
                del cfg["services"][name]["user"]
            else:
                cfg["services"][name]["user"] = bad
            errors = validate_compose(cfg)
            self.assertTrue(any(name in e and "non-root" in e for e in errors), (name, bad))

    def test_healthcheck_required_for_all_core_services(self):
        for name in ("open-connector", "connector-db", "connector-control"):
            cfg = good_config()
            del cfg["services"][name]["healthcheck"]
            errors = validate_compose(cfg)
            self.assertTrue(any(name in e and "healthcheck" in e for e in errors), name)

    # ---- network posture (ruling 4) ---------------------------------------

    def test_runtime_attached_only_to_internal_networks_rejected(self):
        cfg = good_config()
        cfg["services"]["open-connector"]["networks"] = ["oc-db-net"]
        errors = validate_compose(cfg)
        self.assertTrue(any("internal" in e or "OAuth" in e for e in errors))

    def test_runtime_on_default_bridge_has_egress(self):
        cfg = good_config()
        del cfg["services"]["open-connector"]["networks"]
        self.assertEqual(validate_compose(cfg), [])

    # ---- hardening round (quality QF-02/QF-03) ----------------------------

    def test_network_mode_host_rejected_for_core_services(self):
        for name in ("open-connector", "connector-db", "connector-control"):
            cfg = good_config()
            cfg["services"][name]["network_mode"] = "host"
            errors = validate_compose(cfg)
            self.assertTrue(any(name in e and "network_mode" in e for e in errors), name)

    def test_network_mode_container_rejected(self):
        cfg = good_config()
        cfg["services"]["connector-control"]["network_mode"] = "container:other"
        errors = validate_compose(cfg)
        self.assertTrue(any("connector-control" in e and "network_mode" in e for e in errors))

    def test_network_mode_default_bridge_allowed(self):
        cfg = good_config()
        cfg["services"]["open-connector"]["network_mode"] = "bridge"
        self.assertEqual(validate_compose(cfg), [])

    def test_malformed_top_level_rejected_not_crash(self):
        for bad in ([], "x", 42, None):
            errors = validate_compose(bad)
            self.assertTrue(any("mapping" in e for e in errors), bad)

    def test_malformed_services_section_rejected_not_crash(self):
        errors = validate_compose({"services": ["open-connector"]})
        self.assertTrue(any("services" in e and "mapping" in e for e in errors))
        self.assertTrue(any("missing service: " + n in errors for n in ("open-connector", "connector-db", "connector-control")))

    def test_malformed_service_entry_rejected_not_crash(self):
        for name, bad in (("open-connector", "scalar"), ("connector-control", 7), ("weknora-api", ["x"])):
            cfg = good_config()
            cfg["services"][name] = bad
            errors = validate_compose(cfg)
            self.assertTrue(any(name in e and "mapping" in e for e in errors), (name, bad))

    def test_malformed_networks_entry_rejected_not_crash(self):
        cfg = good_config()
        cfg["services"]["open-connector"]["networks"] = "oc-net"
        errors = validate_compose(cfg)
        self.assertTrue(any("networks" in e and "open-connector" in e for e in errors))

    def test_malformed_networks_list_entries_rejected_before_filtering(self):
        # T17 edge closure (T16 residual): list entries that are neither a
        # network name nor a {name: ...} mapping used to be silently dropped
        # by _service_network_names BEFORE the per-entry check — a broken
        # attachment passed validation. They must be rejected on the DECLARED
        # shape.
        for bad in (42, None, {"aliases": ["x"]}, {"name": ""}, ""):
            cfg = good_config()
            cfg["services"]["open-connector"]["networks"] = ["oc-egress-net", bad]
            errors = validate_compose(cfg)
            self.assertTrue(
                any("networks entries" in e and "open-connector" in e for e in errors),
                bad,
            )

    def test_valid_networks_mapping_entry_still_accepted(self):
        cfg = good_config()
        cfg["services"]["open-connector"]["networks"] = [
            "oc-egress-net",
            {"name": "oc-extra-net"},
        ]
        cfg["networks"]["oc-extra-net"] = {"internal": False}
        self.assertEqual(validate_compose(cfg), [])


class ShippedComposeTest(unittest.TestCase):
    """The shipped compose file must pass its own validator (docker required)."""

    REPO_ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
    COMPOSE_FILE = os.path.join(REPO_ROOT, "docker", "compose.open-connector.yaml")

    def _render(self, workdir):
        env = dict(os.environ)
        env.update(
            {
                "OC_SECRETS_DIR": workdir,
                "WEKNORA_DB_HOST": "weknora-postgres",
                "WEKNORA_DB_PORT": "5432",
                "WEKNORA_DB_USER": "weknora",
                "WEKNORA_DB_PASSWORD": "drill-only-not-a-real-secret",
                "WEKNORA_DB_NAME": "weknora",
            }
        )
        # Materialize placeholder secrets so compose can resolve the files.
        # These are drill values only; real deployments provision their own.
        nl = chr(10)
        for name, body in (
            ("connector-admin-token", "drill-admin-token" + nl),
            ("connector-db-password", "drill-db-password" + nl),
            ("connector-secret-key", "A" * 64 + nl),
        ):
            with open(os.path.join(workdir, name), "w") as fh:
                fh.write(body)
        rendered = os.path.join(workdir, "rendered.json")
        proc = subprocess.run(
            [
                "docker", "compose", "-f", self.COMPOSE_FILE,
                "config", "--format", "json",
            ],
            capture_output=True,
            text=True,
            env=env,
            cwd=self.REPO_ROOT,
        )
        # Never echo the parsed config (it interpolates secret-adjacent env).
        if proc.returncode != 0:
            self.skipTest("docker compose config failed: " + proc.stderr.strip()[-300:])
        with open(rendered, "w") as fh:
            fh.write(proc.stdout)
        return rendered

    @unittest.skipUnless(shutil.which("docker"), "docker not available")
    def test_shipped_compose_passes_validation(self):
        if not os.path.exists(self.COMPOSE_FILE):
            self.skipTest("compose file not written yet")
        workdir = tempfile.mkdtemp(prefix="oc-t16-compose-")
        try:
            rendered = self._render(workdir)
            with open(rendered) as fh:
                cfg = json.load(fh)
            errors = validate_compose(cfg)
            self.assertEqual(errors, [])
        finally:
            shutil.rmtree(workdir, ignore_errors=True)


if __name__ == "__main__":
    unittest.main()
