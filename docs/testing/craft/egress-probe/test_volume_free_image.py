"""Focused config and raw-inspect checks for the volume-free E0 fixture."""
from __future__ import annotations

import hashlib
import importlib.util
import json
import tempfile
import unittest
from pathlib import Path

HERE = Path(__file__).resolve().parent
DOCKERFILE = HERE / "Dockerfile.volume-free"
EVIDENCE = HERE / "volume-free"
VERIFIER_PATH = HERE / "assert_v2.py"


def topology_errors(inspects: dict[str, dict]) -> list[str]:
    errors = []
    for role in ("direct", "adapter"):
        mounts = inspects[role].get("Mounts", [])
        if len(mounts) != 1 or mounts[0].get("Type") != "volume" or mounts[0].get("Destination", "").rstrip("/") != "/ledger" or mounts[0].get("RW") is not True or not mounts[0].get("Name"):
            errors.append(f"{role} must have exactly one writable named volume at /ledger")
    direct = inspects["direct"].get("Mounts", [])[0].get("Name") if inspects["direct"].get("Mounts") else None
    adapter = inspects["adapter"].get("Mounts", [])[0].get("Name") if inspects["adapter"].get("Mounts") else None
    if direct and adapter and direct == adapter:
        errors.append("D and A must have separate recorder volumes")
    direct_source = inspects["direct"].get("Mounts", [])[0].get("Source") if inspects["direct"].get("Mounts") else None
    adapter_source = inspects["adapter"].get("Mounts", [])[0].get("Source") if inspects["adapter"].get("Mounts") else None
    if not isinstance(direct_source, str) or not isinstance(adapter_source, str):
        errors.append("D/A must expose one source-owned recorder mount each")
    elif (
        direct_source == adapter_source
        or direct_source.startswith(adapter_source.rstrip("/") + "/")
        or adapter_source.startswith(direct_source.rstrip("/") + "/")
    ):
        errors.append("A can write the direct recorder evidence mount")
    if inspects["helper"].get("Mounts", []) != []:
        errors.append("helper must have no mounts")
    client = inspects["client"].get("Mounts", [])
    expected = {"/home/craft/.config/opencode", "/home/craft/.local/share/opencode"}
    actual = {m.get("Destination") for m in client if m.get("Type") == "volume" and m.get("RW") is True and m.get("Name")}
    if len(client) != 2 or actual != expected:
        errors.append("client must have exactly the two explicit named writable XDG volumes")
    return errors


class VolumeFreeImageTests(unittest.TestCase):
    def test_provisioner_evidence_contains_bounded_commands_exit_and_absence(self):
        bounds = json.loads((EVIDENCE / "topology-fix1-run-bounds.json").read_text())
        self.assertEqual(bounds["docker_cli_timeout_s"], 15)
        self.assertEqual(bounds["overall_deadline_s"], 100)
        self.assertEqual(bounds["container_network_mode"], "none")
        self.assertEqual(bounds["created_networks"], [])
        provisioners = json.loads((EVIDENCE / "topology-provisioners.json").read_text())
        self.assertEqual(set(provisioners), {"direct", "adapter"})
        for role, row in provisioners.items():
            self.assertEqual(row["name"], f"t19vf-fix1-{role}-provisioner")
            self.assertEqual(row["volume"], f"t19vf-fix1-{role}-ledger")
            self.assertEqual(row["volume_create"]["argv"], ["docker", "volume", "create", row["volume"]])
            self.assertEqual(row["volume_create"]["exit_code"], 0)
            self.assertFalse(row["volume_create"]["timed_out"])
            expected_create = [
                "docker", "create", "--name", row["name"], "--network", "none",
                "--cap-drop", "ALL", "--cap-add", "CHOWN", "--user", "0:0",
                "--mount", f"type=volume,src={row['volume']},dst=/ledger",
                "--entrypoint", "/usr/bin/chown", row["image"], "10001:10001", "/ledger",
            ]
            self.assertEqual(row["create"]["argv"], expected_create)
            self.assertEqual(row["create"]["exit_code"], 0)
            self.assertFalse(row["create"]["timed_out"])
            self.assertIn("--network", row["create"]["argv"])
            self.assertIn("none", row["create"]["argv"])
            self.assertIn("--cap-drop", row["create"]["argv"])
            self.assertIn("ALL", row["create"]["argv"])
            self.assertIn("--cap-add", row["create"]["argv"])
            self.assertIn("CHOWN", row["create"]["argv"])
            self.assertIn("--entrypoint", row["create"]["argv"])
            self.assertIn("/usr/bin/chown", row["create"]["argv"])
            self.assertIn("--mount", row["create"]["argv"])
            self.assertIn(f"type=volume,src={row['volume']},dst=/ledger", row["create"]["argv"])
            image_index = row["create"]["argv"].index(row["image"])
            self.assertEqual(row["create"]["argv"][image_index + 1:], ["10001:10001", "/ledger"])
            self.assertEqual(row["start"]["argv"], ["docker", "start", "--attach", row["name"]])
            self.assertEqual(row["start"]["exit_code"], 0)
            self.assertFalse(row["start"]["timed_out"])
            self.assertEqual(row["inspect"]["State"]["Status"], "exited")
            self.assertEqual(row["inspect"]["State"]["ExitCode"], 0)
            self.assertEqual(row["inspect"]["Config"]["User"], "0:0")
            self.assertEqual(row["inspect_argv"], ["docker", "inspect", row["name"]])
            self.assertEqual(row["inspect_exit_code"], 0)
            self.assertEqual(row["inspect"]["HostConfig"]["NetworkMode"], "none")
            self.assertEqual(row["inspect"]["HostConfig"]["CapDrop"], ["ALL"])
            self.assertEqual(row["inspect"]["HostConfig"]["CapAdd"], ["CAP_CHOWN"])
            self.assertIs(row["inspect"]["HostConfig"]["Privileged"], False)
            self.assertEqual(row["inspect"]["Mounts"][0]["Name"], row["volume"])
            self.assertEqual(row["inspect"]["Mounts"][0]["Destination"], "/ledger")
            self.assertIs(row["inspect"]["Mounts"][0]["RW"], True)
            self.assertEqual(row["volume_inspect"]["Name"], row["volume"])
            self.assertIn("/var/lib/docker/volumes/", row["volume_inspect"]["Mountpoint"])
            self.assertIn("10001:10001 /ledger/uid-check", row["writer_check"])
            self.assertEqual(row["remove"]["argv"], ["docker", "rm", row["name"]])
            self.assertEqual(row["remove"]["exit_code"], 0)
            self.assertFalse(row["remove"]["timed_out"])
            self.assertEqual(row["absence"]["argv"], ["docker", "inspect", row["name"]])
            self.assertEqual(row["absence"]["exit_code"], 1)
            self.assertFalse(row["absence"]["timed_out"])
            self.assertIn("no such object", row["absence"]["stderr"].lower())
            self.assertEqual(row["participant_create"]["exit_code"], 0)
            self.assertEqual(row["participant_start"]["exit_code"], 0)
            self.assertEqual(row["participant_remove"]["exit_code"], 0)
            self.assertEqual(row["participant_absence"]["exit_code"], 1)
            self.assertIn("no such object", row["participant_absence"]["stderr"].lower())
            self.assertEqual(row["volume_remove"]["exit_code"], 0)
            self.assertEqual(row["volume_absence"]["exit_code"], 1)
            self.assertIn("no such volume", row["volume_absence"]["stderr"].lower())
            for command in (row["volume_create"], row["create"], row["start"], row["remove"], row["absence"], row["participant_create"], row["participant_start"], row["participant_remove"], row["participant_absence"], row["volume_remove"], row["volume_absence"]):
                self.assertFalse(command["timed_out"])
                self.assertLess(command["elapsed_s"], bounds["docker_cli_timeout_s"])

    def test_base_manifest_shows_why_fixed_topology_rejected_it(self):
        base = json.loads((EVIDENCE / "base-image-inspect.json").read_text())[0]
        self.assertEqual(set(base["Config"]["Volumes"]), {
            "/home/craft/.config/opencode", "/home/craft/.local/share/opencode",
        })
        inherited = {role: json.loads((EVIDENCE / f"base-topology-{role}-inspect.json").read_text())[0]
                     for role in ("direct", "adapter", "helper", "client")}
        for role in inherited:
            mounts = inherited[role]["Mounts"]
            self.assertEqual(len(mounts), 2)
            self.assertEqual({m["Destination"] for m in mounts}, set(base["Config"]["Volumes"]))
            self.assertTrue(all(m["Type"] == "volume" and m["RW"] for m in mounts))
        self.assertTrue(any("exactly one writable named volume" in e for e in topology_errors(inherited)))
        self.assertIn("helper must have no mounts", topology_errors(inherited))

    def test_derivative_dockerfile_drops_volume_metadata_and_restores_identity(self):
        dockerfile = DOCKERFILE.read_text()
        self.assertIn("FROM scratch", dockerfile)
        self.assertIn("COPY --from=pinned / /", dockerfile)
        self.assertIn("COPY --chmod=0444 server_v2.py /usr/local/bin/craft_source_recorder.py", dockerfile)
        self.assertIn("USER 10001:10001", dockerfile)
        self.assertIn('ENTRYPOINT [\"/usr/local/bin/opencode\"]', dockerfile)
        self.assertNotRegex(dockerfile, r"(?m)^VOLUME\b")

    def test_built_derivative_and_raw_topology_evidence_match_pins(self):
        image = json.loads((EVIDENCE / "derivative-image-inspect.json").read_text())[0]
        base = json.loads((EVIDENCE / "base-image-inspect.json").read_text())[0]
        self.assertEqual(image["Os"], "linux")
        self.assertEqual(image["Architecture"], "arm64")
        self.assertIsNone(image["Config"].get("Volumes"))
        self.assertEqual(image["Config"]["User"], "10001:10001")
        self.assertEqual(image["Config"]["Env"], base["Config"]["Env"])
        self.assertEqual(image["Config"]["Entrypoint"], base["Config"]["Entrypoint"])
        self.assertEqual(image["Config"]["Cmd"], base["Config"]["Cmd"])
        self.assertEqual(image["Config"]["WorkingDir"], base["Config"]["WorkingDir"])
        self.assertEqual(image["Config"]["ExposedPorts"], base["Config"]["ExposedPorts"])
        self.assertEqual((EVIDENCE / "derivative-version.txt").read_text().strip(), "1.18.4")
        expected_binary = "3557e87db8c7db70e8ebd42157df1246554120896b115c462b760ff248cf751e"
        self.assertIn(expected_binary, (EVIDENCE / "derivative-binary-sha256.txt").read_text())
        recorder_hash = hashlib.sha256((HERE / "server_v2.py").read_bytes()).hexdigest()
        self.assertEqual(recorder_hash, "d1e7b870b35ede69f6c4e95435ffc11e6ce689a5a556b047e679204400962463")
        self.assertEqual(recorder_hash, (EVIDENCE / "derivative-recorder-sha256.txt").read_text().split()[0])
        self.assertIn("usage:", (EVIDENCE / "derivative-recorder-uid10001-help.txt").read_text())

        actual = {role: json.loads((EVIDENCE / f"topology-{role}-inspect.json").read_text())[0]
                  for role in ("client", "helper")}
        actual.update({role: json.loads((EVIDENCE / f"topology-fix1-{role}-inspect.json").read_text())[0]
                       for role in ("direct", "adapter")})
        self.assertEqual({role: item["Config"]["User"] for role, item in actual.items()},
                         {role: "10001:10001" for role in actual})
        self.assertEqual({item["Image"] for item in actual.values()}, {image["Id"]})
        self.assertEqual(topology_errors(actual), [])
        for role in ("direct", "adapter"):
            host_config = actual[role]["HostConfig"]
            self.assertEqual(host_config["NetworkMode"], "none")
            self.assertIs(host_config["Privileged"], False)
            self.assertIn("ALL", host_config["CapDrop"])
            self.assertIn(host_config["CapAdd"], (None, []))
            self.assertEqual(set(actual[role]["NetworkSettings"]["Networks"]), {"none"})
        provisioners = json.loads((EVIDENCE / "topology-provisioners.json").read_text())
        for role in ("direct", "adapter"):
            participant_mount = actual[role]["Mounts"][0]
            provisioner = provisioners[role]
            self.assertEqual(participant_mount["Name"], provisioner["volume"])
            self.assertEqual(participant_mount["Source"], provisioner["volume_inspect"]["Mountpoint"])
        self.assertEqual((EVIDENCE / "topology-writer-checks.txt").read_text().count("10001:10001"), 2)
        cleanup = (EVIDENCE / "topology-cleanup.txt").read_text()
        self.assertEqual(cleanup.count("[]"), 4 + 4 + 2)

    def test_fixed_disposable_mount_topology_accepts_and_rejects_malformed_inputs(self):
        valid = {
            "direct": {"Mounts": [{"Type": "volume", "Destination": "/ledger", "Name": "d-ledger", "Source": "/var/lib/docker/volumes/d-ledger/_data", "RW": True}]},
            "adapter": {"Mounts": [{"Type": "volume", "Destination": "/ledger", "Name": "a-ledger", "Source": "/var/lib/docker/volumes/a-ledger/_data", "RW": True}]},
            "helper": {"Mounts": []},
            "client": {"Mounts": [
                {"Type": "volume", "Destination": "/home/craft/.config/opencode", "Name": "c-config", "RW": True},
                {"Type": "volume", "Destination": "/home/craft/.local/share/opencode", "Name": "c-data", "RW": True},
            ]},
        }
        self.assertEqual(topology_errors(valid), [])
        malformed = json.loads(json.dumps(valid))
        malformed["adapter"]["Mounts"][0]["Source"] = malformed["direct"]["Mounts"][0]["Source"]
        self.assertIn("A can write the direct recorder evidence mount", topology_errors(malformed))
        malformed = json.loads(json.dumps(valid))
        malformed["adapter"]["Mounts"][0]["Source"] = malformed["direct"]["Mounts"][0]["Source"] + "/nested"
        self.assertIn("A can write the direct recorder evidence mount", topology_errors(malformed))
        malformed = json.loads(json.dumps(valid))
        malformed["direct"]["Mounts"][0]["Source"] = malformed["adapter"]["Mounts"][0]["Source"] + "/nested"
        self.assertIn("A can write the direct recorder evidence mount", topology_errors(malformed))
        malformed = json.loads(json.dumps(valid))
        malformed["adapter"]["Mounts"][0].pop("Source")
        self.assertIn("D/A must expose one source-owned recorder mount each", topology_errors(malformed))
        malformed = json.loads(json.dumps(valid))
        malformed["helper"]["Mounts"] = [{"Type": "volume", "Destination": "/ledger", "Name": "d-ledger", "RW": True}]
        self.assertIn("helper must have no mounts", topology_errors(malformed))
        malformed = json.loads(json.dumps(valid))
        malformed["direct"]["Mounts"][0]["Type"] = "bind"
        self.assertTrue(any("direct must have exactly one writable named volume" in e for e in topology_errors(malformed)))
        malformed = json.loads(json.dumps(valid))
        malformed["client"]["Mounts"].append({"Type": "volume", "Destination": "/home/craft/.config/opencode", "Name": "anonymous", "RW": True})
        self.assertTrue(any("exactly the two explicit named writable XDG volumes" in e for e in topology_errors(malformed)))

    def test_unmodified_fix6_predicate_rejects_source_alias_and_ancestor_overlap(self):
        spec = importlib.util.spec_from_file_location("volume_free_assert_v2", VERIFIER_PATH)
        verifier = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(verifier)
        for adapter_source in (
            "/var/lib/docker/volumes/direct-ledger/_data",
            "/var/lib/docker/volumes/direct-ledger/_data/nested",
        ):
            with self.subTest(adapter_source=adapter_source), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                (root / "commands.jsonl").write_text(json.dumps({"kind": "operation", "operation": "resource_create", "operation_id": "create:x", "controller_ordinal": 1, "resource": "x", "resource_type": "volume", "argv": ["docker", "volume", "create", "x"], "exit_code": 0, "timed_out": False}) + "\n")
                (root / "cleanup.jsonl").write_text("{}\n")
                names = {"client": "client", "adapter": "adapter", "direct": "direct", "helper": "helper", "private": "private", "external": "external", "config": "config", "data": "data"}
                snapshots = {}
                for stage in ("pre", "post_restart"):
                    snapshots[stage] = {}
                    for role in ("client", "adapter", "direct", "helper"):
                        mounts = []
                        if role == "client":
                            mounts = [
                                {"Type": "volume", "Source": "config", "Name": "config", "Destination": "/home/craft/.config/opencode", "RW": True},
                                {"Type": "volume", "Source": "data", "Name": "data", "Destination": "/home/craft/.local/share/opencode", "RW": True},
                            ]
                        elif role in {"direct", "adapter"}:
                            source = "/var/lib/docker/volumes/direct-ledger/_data" if role == "direct" else adapter_source
                            mounts = [{"Type": "volume", "Source": source, "Name": f"{role}-ledger", "Destination": "/ledger", "RW": True}]
                        role_networks = {
                            "client": {"private"},
                            "adapter": {"private", "external"},
                            "direct": {"external"},
                            "helper": {"external"},
                        }[role]
                        container = {"Id": role, "Image": "pinned", "Config": {"User": "10001:10001"}, "HostConfig": {"Privileged": False, "CapAdd": None, "CapDrop": ["ALL"], "NetworkMode": "private" if role in {"client", "adapter"} else "external", "PortBindings": {"8083/tcp": [{}]} if role == "direct" else {}, "Sysctls": {"net.ipv4.ip_forward": "0"} if role == "adapter" else {}}, "NetworkSettings": {"Networks": {network: {} for network in role_networks}}, "Mounts": mounts}
                        snapshots[stage][role] = {"container": container}
                    snapshots[stage]["private_network"] = {"Name": "private", "Internal": True, "Options": {"com.docker.network.bridge.gateway_mode_ipv4": "isolated"}, "Containers": {"client": {}, "adapter": {}}}
                    snapshots[stage]["external_network"] = {"Name": "external", "Internal": False, "Containers": {"direct": {"IPv4Address": "172.20.0.2/24"}, "adapter": {"IPv4Address": "172.20.0.4/24"}, "helper": {"IPv4Address": "172.20.0.3/24"}}}
                (root / "inspect.json").write_text(json.dumps(snapshots))
                manifest = {"host_commands_file": "commands.jsonl", "cleanup_evidence_file": "cleanup.jsonl", "inspect_evidence_file": "inspect.json", "names": names, "image": {"id": "pinned"}}
                errors = verifier.validate_host_evidence(root, manifest)
                self.assertIn("raw pre A can write the direct recorder evidence mount", errors)
