#!/usr/bin/env python3
"""Run and preserve the one Task1 process-attempt feasibility fixture."""

from __future__ import annotations

import datetime as dt
import hashlib
import json
import os
import subprocess
import sys
import time
from pathlib import Path

import attempt_observer


IMAGE_ID = "sha256:87d2f57937114373d64c804a4a7fc44c0f0c1e70e340b88b14da83215e4f4615"
OPENCODE_SHA256 = "3557e87db8c7db70e8ebd42157df1246554120896b115c462b760ff248cf751e"
DENIED_DESTINATION = "203.0.113.19:443"
PORT = 18080


class CaptureError(RuntimeError):
    pass


def utc_now() -> str:
    return dt.datetime.now(dt.timezone.utc).isoformat().replace("+00:00", "Z")


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for chunk in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def command(argv: list[str], label: str, evidence: Path, timeout: int = 30) -> subprocess.CompletedProcess:
    started = utc_now()
    try:
        result = subprocess.run(
            argv,
            check=False,
            text=True,
            capture_output=True,
            timeout=timeout,
        )
    except subprocess.TimeoutExpired as error:
        result = subprocess.CompletedProcess(argv, 124, error.stdout or "", error.stderr or "timed out")
    row = {
        "kind": "command",
        "label": label,
        "started_utc": started,
        "ended_utc": utc_now(),
        "argv": argv,
        "exit_code": result.returncode,
        "stdout": result.stdout,
        "stderr": result.stderr,
    }
    with (evidence / "host-commands.jsonl").open("a", encoding="utf-8") as output:
        output.write(json.dumps(row, sort_keys=True) + "\n")
    return result


def require_success(result: subprocess.CompletedProcess, label: str) -> None:
    if result.returncode != 0:
        raise CaptureError(f"{label} failed ({result.returncode}): {result.stderr.strip()}")


def inspect(name: str, evidence: Path, label: str) -> dict:
    result = command(["docker", "inspect", name], label, evidence)
    require_success(result, label)
    return json.loads(result.stdout)[0]


def write_json(path: Path, value: object) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, sort_keys=True, indent=2) + "\n", encoding="utf-8")


def parse_receipt(stdout: str) -> dict:
    try:
        return json.loads(stdout.splitlines()[-1])
    except (IndexError, json.JSONDecodeError) as error:
        raise CaptureError(f"workload receipt is malformed: {stdout!r}") from error


def rename_trace(
    source: str,
    phase: str,
    pid: int,
    fixture_root: str,
    evidence: Path,
) -> None:
    result = command(
        [
            "docker",
            "exec",
            "-u",
            "10001:10001",
            source,
            "mv",
            f"/evidence/observer/{phase}/trace.pending",
            f"/evidence/observer/{phase}/trace.{pid}",
        ],
        f"rename-{phase}-trace",
        evidence,
    )
    require_success(result, f"rename-{phase}-trace")


def run_traced(
    source: str,
    phase: str,
    workload_args: list[str],
    fixture_root: str,
    evidence: Path,
) -> dict:
    result = command(
        [
            "docker",
            "exec",
            "-u",
            "10001:10001",
            source,
            "/usr/bin/python3",
            "/fixture/ptrace_observer.py",
            f"/evidence/observer/{phase}/trace.pending",
            "/fixture/fixture_workload.py",
            *workload_args,
        ],
        f"observer-{phase}",
        evidence,
        timeout=20,
    )
    require_success(result, f"observer-{phase}")
    receipt = parse_receipt(result.stdout)
    rename_trace(source, phase, receipt["pid"], fixture_root, evidence)
    (evidence / "observer-commands" / f"{phase}.stdout").write_text(result.stdout, encoding="utf-8")
    (evidence / "observer-commands" / f"{phase}.stderr").write_text(result.stderr, encoding="utf-8")
    return receipt


def network_address(network: str, container: str, evidence: Path) -> str:
    result = command(["docker", "network", "inspect", network], "network-address", evidence)
    require_success(result, "network-address")
    payload = json.loads(result.stdout)[0]
    containers = payload.get("Containers", {})
    for row in containers.values():
        if row.get("Name") == container:
            address = row.get("IPv4Address", "").split("/")[0]
            if address:
                return address
    raise CaptureError(f"listener address missing on {network}")


def cleanup(
    resources: list[tuple[str, str]],
    evidence: Path,
) -> None:
    for kind, name in reversed(resources):
        if kind == "container":
            command(["docker", "stop", "--time", "2", name], f"cleanup-stop-{name}", evidence, timeout=12)
            command(["docker", "rm", "-f", name], f"cleanup-rm-{name}", evidence, timeout=12)
        elif kind == "network":
            command(["docker", "network", "rm", name], f"cleanup-network-{name}", evidence, timeout=12)
        elif kind == "volume":
            command(["docker", "volume", "rm", name], f"cleanup-volume-{name}", evidence, timeout=12)
    absent = []
    for kind, name in reversed(resources):
        result = command(["docker", "inspect", name], f"cleanup-absent-{name}", evidence)
        absent.append({"kind": kind, "name": name, "exit_code": result.returncode})
    write_json(evidence / "cleanup-evidence.json", {"absent": absent})


def file_hashes(root: Path) -> dict[str, str]:
    rows = {}
    for path in sorted(item for item in root.rglob("*") if item.is_file()):
        rows[str(path.relative_to(root))] = sha256(path)
    return rows


def main() -> int:
    fixture_root = Path(__file__).resolve().parent
    run_id = time.strftime("%Y%m%dT%H%M%S") + f"-{os.getpid()}"
    evidence = fixture_root / "evidence" / run_id
    evidence.mkdir(parents=True)
    prefix = f"craft-e0-attempt-{run_id}"
    network = f"{prefix}-net"
    listener = f"{prefix}-listener"
    source = f"{prefix}-source"
    volume = f"{prefix}-evidence"
    resources: list[tuple[str, str]] = []
    source_inspect = None
    listener_address = None
    receipts: dict[str, dict] = {}
    listener_pre_logs = ""

    try:
        require_success(
            command(["docker", "network", "create", "--internal", network], "create-network", evidence),
            "create-network",
        )
        resources.append(("network", network))
        require_success(
            command(["docker", "volume", "create", volume], "create-evidence-volume", evidence),
            "create-evidence-volume",
        )
        resources.append(("volume", volume))

        common = [
            "--network",
            network,
            "--mount",
            f"type=bind,source={fixture_root},target=/fixture,readonly",
        ]
        listener_result = command(
            [
                "docker",
                "run",
                "-d",
                "--name",
                listener,
                *common,
                "--entrypoint",
                "/usr/bin/python3",
                IMAGE_ID,
                "/fixture/loss_listener.py",
                "0.0.0.0",
                str(PORT),
            ],
            "create-listener",
            evidence,
        )
        require_success(listener_result, "create-listener")
        resources.append(("container", listener))
        source_result = command(
            [
                "docker",
                "run",
                "-d",
                "--name",
                source,
                *common,
                "--mount",
                f"type=volume,source={volume},target=/evidence",
                "--cap-drop",
                "ALL",
                "--cap-add",
                "SYS_PTRACE",
                "--security-opt",
                "seccomp=unconfined",
                "--user",
                "0:0",
                "--entrypoint",
                "/usr/bin/python3",
                IMAGE_ID,
                "-c",
                "import time; time.sleep(300)",
            ],
            "create-source",
            evidence,
        )
        require_success(source_result, "create-source")
        resources.append(("container", source))
        require_success(
            command(
                [
                    "docker",
                    "exec",
                    source,
                    "/usr/bin/install",
                    "-d",
                    "-o",
                    "10001",
                    "-g",
                    "10001",
                    "-m",
                    "0700",
                    "/evidence",
                    "/evidence/observer",
                    "/evidence/observer/no-packet",
                    "/evidence/observer/local-response-loss",
                    "/evidence/observer/denied",
                    "/evidence/observer/unrelated",
                ],
                "provision-evidence-volume",
                evidence,
            ),
            "provision-evidence-volume",
        )

        source_inspect = inspect(source, evidence, "source-inspect")
        listener_inspect = inspect(listener, evidence, "listener-inspect")
        if source_inspect.get("Image") != IMAGE_ID or listener_inspect.get("Image") != IMAGE_ID:
            raise CaptureError("participant does not use the pinned derivative image")
        binary = command(
            [
                "docker",
                "exec",
                "-u",
                "10001:10001",
                source,
                "sha256sum",
                "/usr/local/bin/opencode",
            ],
            "opencode-binary-hash",
            evidence,
        )
        require_success(binary, "opencode-binary-hash")
        actual_binary = binary.stdout.split()[0]
        if actual_binary != OPENCODE_SHA256:
            raise CaptureError("OpenCode binary hash does not match reviewed pin")
        listener_address = network_address(network, listener, evidence)
        listener_pre_logs = command(["docker", "logs", listener], "listener-pre-logs", evidence).stdout

        without_observer = command(
            [
                "docker",
                "exec",
                "-u",
                "10001:10001",
                source,
                "/usr/bin/python3",
                "/fixture/fixture_workload.py",
                "no-packet",
            ],
            "without-observer-no-packet",
            evidence,
        )
        require_success(without_observer, "without-observer-no-packet")
        (evidence / "without-observer").mkdir()
        (evidence / "without-observer" / "no-packet.stdout").write_text(
            without_observer.stdout, encoding="utf-8"
        )

        receipts["no-packet"] = run_traced(
            source, "no-packet", ["no-packet"], str(fixture_root), evidence
        )
        local_destination = f"{listener_address}:{PORT}"
        receipts["local-response-loss"] = run_traced(
            source,
            "local-response-loss",
            ["local-response-loss", "--host", listener_address, "--port", str(PORT)],
            str(fixture_root),
            evidence,
        )
        if receipts["local-response-loss"].get("response_bytes") != 0:
            raise CaptureError("local response-loss control did not lose the response")
        require_success(
            command(["docker", "start", listener], "restart-listener-unrelated", evidence),
            "restart-listener-unrelated",
        )
        receipts["unrelated"] = run_traced(
            source,
            "unrelated",
            ["unrelated", "--host", listener_address, "--port", str(PORT)],
            str(fixture_root),
            evidence,
        )
        receipts["denied"] = run_traced(
            source,
            "denied",
            ["denied", "--host", DENIED_DESTINATION.rsplit(":", 1)[0], "--port", "443"],
            str(fixture_root),
            evidence,
        )

        listener_logs = command(["docker", "logs", listener], "listener-final-logs", evidence).stdout
        (evidence / "listener.jsonl").write_text(listener_logs, encoding="utf-8")
        (evidence / "listener-pre.jsonl").write_text(listener_pre_logs, encoding="utf-8")
        copied = command(
            ["docker", "cp", f"{source}:/evidence/.", str(evidence)],
            "copy-evidence",
            evidence,
            timeout=20,
        )
        require_success(copied, "copy-evidence")
        write_json(
            evidence / "source" / "local-response-loss.json",
            receipts["local-response-loss"],
        )
        write_json(
            evidence / "identity.json",
            {
                "image_id": IMAGE_ID,
                "container_id": source_inspect["Id"],
                "source_pid": receipts["local-response-loss"]["pid"],
                "source_uid": receipts["local-response-loss"]["uid"],
                "unrelated_pid": receipts["unrelated"]["pid"],
                "unrelated_uid": receipts["unrelated"]["uid"],
                "netns": receipts["local-response-loss"]["netns"],
                "local_destination": local_destination,
                "denied_destination": DENIED_DESTINATION,
            },
        )
        summary = attempt_observer.validate_feasibility(evidence, IMAGE_ID)
        write_json(evidence / "validation.json", summary)
        if summary["verdict"] != "PASS":
            raise CaptureError("feasibility validation failed: " + "; ".join(summary["errors"]))
    except Exception as error:
        write_json(
            evidence / "capture-error.json",
            {"error": str(error), "captured_utc": utc_now()},
        )
        cleanup(resources, evidence)
        return 1

    cleanup(resources, evidence)
    hashes = file_hashes(evidence)
    write_json(evidence / "hashes.json", hashes)
    checkpoint = {
        "checkpoint_id": f"t19-e0-attempt-feasibility-{run_id}",
        "image_id": IMAGE_ID,
        "opencode_sha256": OPENCODE_SHA256,
        "validation": "PASS",
        "paid_egress": False,
        "network_mode": "docker-internal-only",
        "cleanup": "all run-owned resources absent",
        "evidence": str(evidence),
    }
    write_json(evidence / "checkpoint.json", checkpoint)
    report = f"""# Process-aware attempt feasibility

Checkpoint: `{checkpoint['checkpoint_id']}`

Result: **PASS for Task1 feasibility only; no E0 PASS.** The unchanged pinned derivative image and OpenCode binary were used. One internal-only Docker network carried the local listener/source traffic; the denied control targeted reserved invalid address `{DENIED_DESTINATION}`. No external provider or paid endpoint was contacted.

Raw evidence, exact host commands, hashes, and cleanup receipts are in `{evidence}`. Validation credited exactly the source PID's local connect and denied connect; wrong PID, packet-free, unrelated process, missing observer, and response-loss controls did not receive attempt credit.
"""
    (evidence / "report.md").write_text(report, encoding="utf-8")
    print(json.dumps(checkpoint, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
