"""Focused config and raw-inspect checks for the volume-free E0 fixture."""
from __future__ import annotations

import hashlib
import json
import unittest
from pathlib import Path

HERE = Path(__file__).resolve().parent
DOCKERFILE = HERE / "Dockerfile.volume-free"
EVIDENCE = HERE / "volume-free"


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
    if inspects["helper"].get("Mounts", []) != []:
        errors.append("helper must have no mounts")
    client = inspects["client"].get("Mounts", [])
    expected = {"/home/craft/.config/opencode", "/home/craft/.local/share/opencode"}
    actual = {m.get("Destination") for m in client if m.get("Type") == "volume" and m.get("RW") is True and m.get("Name")}
    if len(client) != 2 or actual != expected:
        errors.append("client must have exactly the two explicit named writable XDG volumes")
    return errors


class VolumeFreeImageTests(unittest.TestCase):
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
                  for role in ("client", "direct", "adapter", "helper")}
        self.assertEqual({role: item["Config"]["User"] for role, item in actual.items()},
                         {role: "10001:10001" for role in actual})
        self.assertEqual({item["Image"] for item in actual.values()}, {image["Id"]})
        self.assertEqual(topology_errors(actual), [])
        self.assertEqual((EVIDENCE / "topology-writer-checks.txt").read_text().count("10001:10001"), 2)
        cleanup = (EVIDENCE / "topology-cleanup.txt").read_text()
        self.assertEqual(cleanup.count("[]"), 4 + 4 + 2)

    def test_fixed_disposable_mount_topology_accepts_and_rejects_malformed_inputs(self):
        valid = {
            "direct": {"Mounts": [{"Type": "volume", "Destination": "/ledger", "Name": "d-ledger", "RW": True}]},
            "adapter": {"Mounts": [{"Type": "volume", "Destination": "/ledger", "Name": "a-ledger", "RW": True}]},
            "helper": {"Mounts": []},
            "client": {"Mounts": [
                {"Type": "volume", "Destination": "/home/craft/.config/opencode", "Name": "c-config", "RW": True},
                {"Type": "volume", "Destination": "/home/craft/.local/share/opencode", "Name": "c-data", "RW": True},
            ]},
        }
        self.assertEqual(topology_errors(valid), [])
        malformed = json.loads(json.dumps(valid))
        malformed["helper"]["Mounts"] = [{"Type": "volume", "Destination": "/ledger", "Name": "d-ledger", "RW": True}]
        self.assertIn("helper must have no mounts", topology_errors(malformed))
        malformed = json.loads(json.dumps(valid))
        malformed["direct"]["Mounts"][0]["Type"] = "bind"
        self.assertTrue(any("direct must have exactly one writable named volume" in e for e in topology_errors(malformed)))
        malformed = json.loads(json.dumps(valid))
        malformed["client"]["Mounts"].append({"Type": "volume", "Destination": "/home/craft/.config/opencode", "Name": "anonymous", "RW": True})
        self.assertTrue(any("exactly the two explicit named writable XDG volumes" in e for e in topology_errors(malformed)))
