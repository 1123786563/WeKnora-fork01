#!/usr/bin/env python3
"""Focused runner tests for source-owned helper-start acknowledgment."""
import importlib.util
import json
import tempfile
import unittest
from pathlib import Path

SCRIPT = Path(__file__).with_name("run_v2.py")
spec = importlib.util.spec_from_file_location("run_v2_source_test", SCRIPT)
run_v2 = importlib.util.module_from_spec(spec)
spec.loader.exec_module(run_v2)


class HelperStartBoundaryTests(unittest.TestCase):
    def receipt(self, argv=None, exit_code=0, timed_out=False):
        return {
            "argv": argv or ["docker", "start", "e0-helper"],
            "exit_code": exit_code,
            "timed_out": timed_out,
            "container_id": "a" * 64,
            "peer_ip": "172.20.0.9",
        }

    def test_successful_start_is_logged_before_both_matching_source_acks(self):
        order = []
        commands = []
        records = []

        def start(name, op_id, ordinal):
            order.append(("start", name, op_id, ordinal))
            return self.receipt()

        def send(source, request):
            order.append(("ack", source))
            commands.append((source, request))
            return {
                "ok": True,
                "source": source,
                "phase_id": "positive-controls",
                "boundary": "helper_start",
                "operation_id": request["operation_id"],
                "controller_ordinal": request["controller_ordinal"],
                "stream_cursor": 7 if source == "direct" else 4,
            }

        def record(row):
            order.append(("record", row["operation_id"]))
            records.append(row)

        def dispatch():
            order.append(("probe", "after-both-acks"))
            return {"exit_code": 0}

        result = run_v2.start_helper_and_ack(
            "positive-controls",
            "e0-helper",
            operation_id="run:docker_start:17",
            controller_ordinal=17,
            start_operation=start,
            send_source=send,
            record_operation=record,
            dispatch=dispatch,
        )

        self.assertEqual([item[0] for item in order], ["start", "record", "ack", "ack", "probe"])
        self.assertEqual([source for source, _ in commands], ["direct", "adapter"])
        for _, request in commands:
            self.assertEqual(request, {
                "op": "helper_start",
                "phase_id": "positive-controls",
                "operation_id": "run:docker_start:17",
                "controller_ordinal": 17,
            })
        self.assertEqual(len(records), 1)
        self.assertEqual(records[0]["operation"], "docker_start")
        self.assertEqual(records[0]["argv"], ["docker", "start", "e0-helper"])
        self.assertEqual(result["source_cursors"], {"direct": 7, "adapter": 4})
        self.assertEqual(result["probe_result"], {"exit_code": 0})

    def test_failed_or_mismatched_host_start_never_sends_source_ack(self):
        calls = []

        def start(name, op_id, ordinal):
            return self.receipt(exit_code=1)

        with self.assertRaisesRegex(RuntimeError, "helper start did not succeed"):
            run_v2.start_helper_and_ack(
                "phase-failed", "e0-helper", operation_id="start-failed", controller_ordinal=2,
                start_operation=start, send_source=lambda *_: calls.append("ack"),
                record_operation=lambda _: None,
            )
        self.assertEqual(calls, [])

        def wrong_argv(name, op_id, ordinal):
            return self.receipt(argv=["docker", "run", "--rm", "some-other-helper"])

        with self.assertRaisesRegex(RuntimeError, "does not match the registered helper"):
            run_v2.start_helper_and_ack(
                "phase-wrong", "e0-helper", operation_id="start-wrong", controller_ordinal=3,
                start_operation=wrong_argv, send_source=lambda *_: calls.append("ack"),
                record_operation=lambda _: None,
            )
        self.assertEqual(calls, [])

    def test_one_sided_or_forged_ack_fails_before_helper_traffic_can_start(self):
        calls = []
        dispatched = []

        def start(name, op_id, ordinal):
            return self.receipt()

        def one_sided(source, request):
            calls.append(source)
            if source == "adapter":
                return {"ok": False, "source": source}
            return {
                "ok": True, "source": source, "phase_id": request["phase_id"],
                "boundary": "helper_start", "operation_id": request["operation_id"],
                "controller_ordinal": request["controller_ordinal"], "stream_cursor": 9,
            }

        with self.assertRaisesRegex(RuntimeError, "adapter helper-start acknowledgment"):
            run_v2.start_helper_and_ack(
                "phase-one-sided", "e0-helper", operation_id="start-one-sided", controller_ordinal=4,
                start_operation=start, send_source=one_sided, record_operation=lambda _: None,
                dispatch=lambda: dispatched.append("probe"),
            )
        self.assertEqual(calls, ["direct", "adapter"])
        self.assertEqual(dispatched, [])

        def forged(source, request):
            return {
                "ok": True, "source": source, "phase_id": request["phase_id"],
                "boundary": "helper_start", "operation_id": "forged-start",
                "controller_ordinal": request["controller_ordinal"], "stream_cursor": 10,
            }

        with self.assertRaisesRegex(RuntimeError, "direct helper-start acknowledgment identity mismatch"):
            run_v2.start_helper_and_ack(
                "phase-forged", "e0-helper", operation_id="start-real", controller_ordinal=5,
                start_operation=start, send_source=forged, record_operation=lambda _: None,
                dispatch=lambda: dispatched.append("probe"),
            )
        self.assertEqual(dispatched, [])

    def test_manifest_c_volume_names_join_raw_inspect_in_existing_verifier(self):
        verifier_path = Path(__file__).with_name("assert_v2.py")
        verifier_spec = importlib.util.spec_from_file_location("volume_free_task2_assert_v2", verifier_path)
        verifier = importlib.util.module_from_spec(verifier_spec)
        verifier_spec.loader.exec_module(verifier)

        names = dict(run_v2.manifest["names"])
        config_volume = run_v2.NAMES["config"]
        data_volume = run_v2.NAMES["data"]
        reviewed_derivative_id = "sha256:87d2f57937114373d64c804a4a7fc44c0f0c1e70e340b88b14da83215e4f4615"
        private, external = names["private"], names["external"]
        participants = {}
        network_sets = {
            "client": {private}, "adapter": {private, external},
            "direct": {external}, "helper": {external},
        }
        mounts = {
            "client": [
                {"Type": "volume", "Name": config_volume, "Source": "/var/lib/docker/volumes/cfg/_data",
                 "Destination": "/home/craft/.config/opencode", "RW": True},
                {"Type": "volume", "Name": data_volume, "Source": "/var/lib/docker/volumes/data/_data",
                 "Destination": "/home/craft/.local/share/opencode", "RW": True},
            ],
            "direct": [{"Type": "volume", "Name": names["direct_evidence"],
                        "Source": "/var/lib/docker/volumes/direct/_data", "Destination": "/ledger", "RW": True}],
            "adapter": [{"Type": "volume", "Name": names["adapter_evidence"],
                          "Source": "/var/lib/docker/volumes/adapter/_data", "Destination": "/ledger", "RW": True}],
            "helper": [],
        }
        for role in ("client", "direct", "adapter", "helper"):
            participants[role] = {
                "container": {
                    "Id": f"representative-{role}-id", "Image": reviewed_derivative_id,
                    "Config": {"User": "10001:10001"}, "Mounts": mounts[role],
                    "HostConfig": {
                        "Privileged": False, "CapAdd": None, "CapDrop": ["ALL"],
                        "NetworkMode": private if role in {"client", "adapter"} else external,
                        "PortBindings": {"8083/tcp": [{"HostIp": "0.0.0.0", "HostPort": "49153"}]} if role == "direct" else {},
                        "Sysctls": {"net.ipv4.ip_forward": "0"} if role == "adapter" else {},
                    },
                    "NetworkSettings": {"Networks": {name: {} for name in network_sets[role]}},
                }
            }
        raw_inspect = {stage: {
            **json.loads(json.dumps(participants)),
            "private_network": {"Name": private, "Internal": True,
                                "Options": {"com.docker.network.bridge.gateway_mode_ipv4": "isolated"},
                                "Containers": {names["client"]: {}, names["adapter"]: {}}},
            "external_network": {"Name": external, "Internal": False,
                                 "Containers": {names["direct"]: {"IPv4Address": "172.20.0.2/24"},
                                                 names["adapter"]: {"IPv4Address": "172.20.0.4/24"},
                                                 names["helper"]: {"IPv4Address": "172.20.0.3/24"}}},
        } for stage in ("pre", "post_restart")}

        def topology_errors(manifest_names):
            with tempfile.TemporaryDirectory(prefix="e0-runner-c-names-") as tmp:
                root = Path(tmp)
                (root / "snapshots").mkdir()
                (root / "snapshots" / "owned-inspect.json").write_text(json.dumps(raw_inspect))
                (root / "host-commands.jsonl").write_text("{}\n")
                manifest = {
                    "schema": "craft-e0-v2", "run_id": "representative-run",
                    "evidence_type": "synthetic", "verdict": "PASS",
                    "image": {"id": reviewed_derivative_id, "platform": "linux/arm64", "version": "1.18.4"},
                    "names": manifest_names, "cases": {}, "inspect_evidence_file": "snapshots/owned-inspect.json",
                    "host_commands_file": "host-commands.jsonl",
                }
                (root / "manifest.json").write_text(json.dumps(manifest))
                return verifier.verify(root)[1]

        c_mount_error = "raw pre client mount set is not the fixed XDG config/data volume pair"
        missing_names = dict(names)
        missing_names.pop("config", None)
        missing_names.pop("data", None)
        missing_name_errors = topology_errors(missing_names)
        self.assertTrue(any(c_mount_error in error for error in missing_name_errors), missing_name_errors)

        errors = topology_errors(names)
        self.assertFalse(any(c_mount_error in error for error in errors), errors)
        self.assertEqual(names.get("config"), config_volume)
        self.assertEqual(names.get("data"), data_volume)

        tampered_names = dict(names)
        tampered_names["config"] = "unowned-config-volume"
        tampered_errors = topology_errors(tampered_names)
        self.assertTrue(any(c_mount_error in error for error in tampered_errors), tampered_errors)

    def test_runner_uses_reviewed_derivative_and_fixed_topology_argv(self):
        self.assertEqual(
            run_v2.IMAGE,
            "sha256:87d2f57937114373d64c804a4a7fc44c0f0c1e70e340b88b14da83215e4f4615",
        )
        self.assertEqual(run_v2.RUN_UID, "10001:10001")
        direct = run_v2.source_evidence_mount("direct", "run-direct-ledger")
        adapter = run_v2.source_evidence_mount("adapter", "run-adapter-ledger")
        direct_argv = run_v2.source_recorder_argv("direct", "run-id", "run-direct-ledger")
        adapter_argv = run_v2.source_recorder_argv("adapter", "run-id", "run-adapter-ledger")
        helper = run_v2.helper_container_argv("e0-helper", "e0-external", run_v2.IMAGE)
        client_mounts = run_v2.client_xdg_mounts("config-vol", "data-vol")

        self.assertEqual(direct, ["--mount", "type=volume,source=run-direct-ledger,target=/ledger"])
        self.assertEqual(adapter, ["--mount", "type=volume,source=run-adapter-ledger,target=/ledger"])
        self.assertNotEqual(direct[1], adapter[1])
        for role, argv in (("direct", direct_argv), ("adapter", adapter_argv)):
            self.assertIn("--user", argv)
            self.assertEqual(argv[argv.index("--user") + 1], "10001:10001")
            self.assertIn("/usr/local/bin/craft_source_recorder.py", argv)
            self.assertNotIn("/probe/server_v2.py", argv)
            self.assertNotIn("-v", argv)
            self.assertNotIn("--volume", argv)
            self.assertIn("target=/ledger", " ".join(argv))
            self.assertIn("--evidence-dir", argv)
            self.assertEqual(argv[argv.index("--evidence-dir") + 1], "/ledger")
            self.assertEqual(argv[argv.index("--control-socket") + 1], "/ledger/control.sock")
        self.assertNotIn("--mount", helper)
        self.assertNotIn("-v", helper)
        self.assertEqual(client_mounts, [
            "--mount", "type=volume,source=config-vol,target=/home/craft/.config/opencode",
            "--mount", "type=volume,source=data-vol,target=/home/craft/.local/share/opencode",
        ])

    def test_ledger_provisioner_is_one_networkless_chown_only_container_and_is_removed(self):
        calls = []
        name = run_v2.NAMES["direct_provisioner"]
        volume = "direct-ledger-test"
        inspect = [{
            "State": {"Status": "exited", "ExitCode": 0},
            "Config": {"User": "0:0"},
            "HostConfig": {"NetworkMode": "none", "CapDrop": ["ALL"], "CapAdd": ["CAP_CHOWN"], "Privileged": False},
            "Mounts": [{"Name": volume, "Destination": "/ledger", "RW": True}],
        }]

        def docker(*args, **kwargs):
            calls.append((args, kwargs))
            return {"argv": ["docker", *args], "exit_code": 0, "stdout": "", "stderr": "", "timed_out": False}

        owned = []
        from unittest.mock import patch
        with patch.object(run_v2, "docker", side_effect=docker), \
             patch.object(run_v2, "json_cmd", return_value=inspect), \
             patch.object(run_v2, "cleanup_owned_resource", return_value={"ok": True, "inspect_absent": True}), \
             patch.object(run_v2, "save_snapshot"):
            result = run_v2.provision_ledger_volume("direct", volume, owned)

        self.assertEqual(owned, [("volume", volume)])
        self.assertEqual(result["inspect"]["Config"]["User"], "0:0")
        create_args = calls[1][0]
        self.assertEqual(create_args, (
            "create", "--name", name, "--network", "none", "--cap-drop", "ALL", "--cap-add", "CHOWN",
            "--user", "0:0", "--mount", f"type=volume,src={volume},dst=/ledger",
            "--entrypoint", "/usr/bin/chown", run_v2.IMAGE, run_v2.RUN_UID, "/ledger",
        ))
        self.assertEqual(calls[2][0], ("start", "--attach", name))

    def test_raw_topology_validator_rejects_source_aliases_and_nonfixed_mounts(self):
        n = run_v2.NAMES
        valid = {
            "client": [{"Image": run_v2.IMAGE, "Config": {"User": run_v2.RUN_UID}, "Mounts": [
                {"Type": "volume", "Name": n["config"], "Destination": "/home/craft/.config/opencode", "RW": True},
                {"Type": "volume", "Name": n["data"], "Destination": "/home/craft/.local/share/opencode", "RW": True},
            ]}],
            "direct": [{"Image": run_v2.IMAGE, "Config": {"User": run_v2.RUN_UID}, "Mounts": [
                {"Type": "volume", "Name": n["direct_evidence"], "Source": "/var/lib/docker/volumes/d/_data", "Destination": "/ledger", "RW": True},
            ]}],
            "adapter": [{"Image": run_v2.IMAGE, "Config": {"User": run_v2.RUN_UID}, "Mounts": [
                {"Type": "volume", "Name": n["adapter_evidence"], "Source": "/var/lib/docker/volumes/a/_data", "Destination": "/ledger", "RW": True},
            ]}],
            "helper": [{"Image": run_v2.IMAGE, "Config": {"User": run_v2.RUN_UID}, "Mounts": []}],
        }
        self.assertEqual(run_v2.topology_mismatches(valid), [])
        import copy
        for source in ("/var/lib/docker/volumes/d/_data", "/var/lib/docker/volumes/d/_data/nested"):
            broken = copy.deepcopy(valid)
            broken["adapter"][0]["Mounts"][0]["Source"] = source
            self.assertIn("D/A source ledger paths alias or overlap", run_v2.topology_mismatches(broken))
        broken = copy.deepcopy(valid)
        broken["client"][0]["Mounts"].append({"Type": "bind", "Source": "/tmp/config", "Destination": "/probe/opencode.json", "RW": False})
        self.assertIn("client must have exactly its two explicit named writable XDG volumes", run_v2.topology_mismatches(broken))
        broken = copy.deepcopy(valid)
        broken["helper"][0]["Mounts"].append({"Type": "volume", "Name": n["data"], "Destination": "/ledger", "RW": True})
        self.assertIn("helper must have zero mounts", run_v2.topology_mismatches(broken))
        broken = copy.deepcopy(valid)
        broken["direct"][0]["Config"]["User"] = "0:0"
        self.assertIn("direct user must be 10001:10001", run_v2.topology_mismatches(broken))

    def test_image_identity_config_and_baked_recorder_are_verified(self):
        inspect = {
            "Id": run_v2.IMAGE, "Os": "linux", "Architecture": "arm64",
            "Config": {
                "Volumes": None, "User": "10001:10001",
                "Env": ["PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
                        "HOME=/home/craft", "XDG_CONFIG_HOME=/home/craft/.config",
                        "XDG_DATA_HOME=/home/craft/.local/share"],
                "Entrypoint": ["/usr/local/bin/opencode"],
                "Cmd": ["serve", "--hostname", "0.0.0.0", "--port", "4096"],
                "WorkingDir": "/workspace/output", "ExposedPorts": {"4096/tcp": {}},
            },
        }
        self.assertEqual(run_v2.validate_image_inspect(inspect), [])
        for mutate, expected in (
            (lambda row: row.update(Id="sha256:wrong"), "image ID"),
            (lambda row: row["Config"].update(Volumes={"/tmp": {}}), "declared volumes"),
            (lambda row: row["Config"].update(User="0:0"), "user"),
            (lambda row: row["Config"].update(Env=["HOME=/wrong"]), "environment"),
        ):
            import copy
            broken = copy.deepcopy(inspect)
            mutate(broken)
            self.assertTrue(any(expected in err for err in run_v2.validate_image_inspect(broken)))
        self.assertEqual(run_v2.RECORDER_SHA256, "d1e7b870b35ede69f6c4e95435ffc11e6ce689a5a556b047e679204400962463")


class CleanupClassificationTests(unittest.TestCase):
    RESOURCE = "e0-cleanup-probe"

    def records(self, remove, inspect):
        queue = [remove, inspect]
        calls = []

        def invoke(argv, timeout=30, label=None):
            calls.append((argv, timeout, label))
            return queue.pop(0)

        result = run_v2.cleanup_owned_resource("container", self.RESOURCE, invoke)
        return result, calls

    def rec(self, argv, exit_code, stdout="", stderr="", timed_out=False):
        return {"argv": argv, "exit_code": exit_code, "stdout": stdout,
                "stderr": stderr, "timed_out": timed_out}

    def test_failed_remove_plus_daemon_inspect_error_is_unresolved(self):
        remove = self.rec(["docker", "rm", "-f", "-v", self.RESOURCE], 1,
                          stderr="permission denied")
        inspect = self.rec(["docker", "inspect", self.RESOURCE], 1,
                           stderr="Cannot connect to the Docker daemon")
        result, _ = self.records(remove, inspect)
        self.assertFalse(result["ok"])
        self.assertEqual(result["resource"], self.RESOURCE)
        self.assertEqual(result["remove"], remove)
        self.assertEqual(result["inspect"], inspect)

    def test_remove_timeout_is_unresolved_even_if_inspect_says_not_found(self):
        remove = self.rec(["docker", "rm", "-f", "-v", self.RESOURCE], None,
                          timed_out=True)
        inspect = self.rec(["docker", "inspect", self.RESOURCE], 1, stdout="[]",
                           stderr=f"error: no such object: {self.RESOURCE}")
        result, _ = self.records(remove, inspect)
        self.assertFalse(result["ok"])
        self.assertTrue(result["remove"]["timed_out"])

    def test_permission_error_is_not_misclassified_as_absence(self):
        remove = self.rec(["docker", "rm", "-f", "-v", self.RESOURCE], 1,
                          stderr="permission denied")
        inspect = self.rec(["docker", "inspect", self.RESOURCE], 1,
                           stderr="permission denied")
        result, _ = self.records(remove, inspect)
        self.assertFalse(result["ok"])

    def test_unrelated_missing_resource_is_not_accepted(self):
        remove = self.rec(["docker", "rm", "-f", "-v", self.RESOURCE], 1,
                          stderr="Error response from daemon: No such container: other")
        inspect = self.rec(["docker", "inspect", self.RESOURCE], 1, stdout="[]",
                           stderr="error: no such object: other")
        result, _ = self.records(remove, inspect)
        self.assertFalse(result["ok"])

    def test_success_and_exact_not_found_inspect_is_resolved(self):
        remove = self.rec(["docker", "rm", "-f", "-v", self.RESOURCE], 0,
                          stdout=self.RESOURCE)
        inspect = self.rec(["docker", "inspect", self.RESOURCE], 1, stdout="[]",
                           stderr=f"error: no such object: {self.RESOURCE}")
        result, calls = self.records(remove, inspect)
        self.assertTrue(result["ok"])
        self.assertTrue(result["inspect_absent"])
        self.assertEqual(len(calls), 2)

    def test_exact_already_absent_remove_still_requires_exact_inspect(self):
        remove = self.rec(["docker", "rm", "-f", "-v", self.RESOURCE], 1,
                          stderr=f"Error response from daemon: No such container: {self.RESOURCE}")
        inspect = self.rec(["docker", "inspect", self.RESOURCE], 1, stdout="[]",
                           stderr=f"error: no such object: {self.RESOURCE}")
        result, _ = self.records(remove, inspect)
        self.assertTrue(result["ok"])
        self.assertTrue(result["already_absent"])

    def test_successful_inspect_or_generic_inspect_error_is_unresolved(self):
        remove = self.rec(["docker", "rm", "-f", "-v", self.RESOURCE], 0)
        for inspect in (
            self.rec(["docker", "inspect", self.RESOURCE], 0, stdout='{"Id":"x"}'),
            self.rec(["docker", "inspect", self.RESOURCE], 1, stderr="daemon unavailable"),
        ):
            with self.subTest(inspect=inspect):
                result, _ = self.records(remove, inspect)
                self.assertFalse(result["ok"])

    def test_network_and_volume_require_their_own_exact_not_found_responses(self):
        cases = (
            ("network", "docker network rm e0-cleanup-probe", "docker network inspect e0-cleanup-probe",
             "Error response from daemon: network e0-cleanup-probe not found"),
            ("volume", "docker volume rm e0-cleanup-probe", "docker volume inspect e0-cleanup-probe",
             "Error response from daemon: get e0-cleanup-probe: no such volume"),
        )
        for kind, _, _, not_found in cases:
            with self.subTest(kind=kind):
                queue = [
                    self.rec([], 1, stderr=not_found),
                    self.rec([], 1, stdout="[]", stderr=not_found),
                ]

                def invoke(argv, timeout=30, label=None):
                    row = queue.pop(0)
                    row["argv"] = argv
                    return row

                result = run_v2.cleanup_owned_resource(kind, self.RESOURCE, invoke)
                self.assertTrue(result["ok"])
                self.assertEqual(result["remove"]["argv"][0], "docker")

                queue = [self.rec([], 1, stderr=not_found.replace(self.RESOURCE, "other")),
                         self.rec([], 1, stdout="[]", stderr=not_found)]
                result = run_v2.cleanup_owned_resource(kind, self.RESOURCE, invoke)
                self.assertFalse(result["ok"])

if __name__ == "__main__":
    unittest.main(verbosity=2)
