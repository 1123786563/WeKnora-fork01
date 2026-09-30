"""Process-aware OpenCode network-attempt observation helpers."""

from dataclasses import dataclass
import json
import re
from pathlib import Path
from typing import Iterable


@dataclass(frozen=True)
class Attempt:
    pid: int
    timestamp: str
    destination: str
    outcome: str
    raw_line: str


_CONNECT = re.compile(r'(?P<timestamp>\d+\.\d+)\s+connect\(')
_DESTINATION = re.compile(
    r'sa_family=AF_INET,\s*sin_port=htons\((?P<port>\d+)\),\s*'
    r'sin_addr=inet_addr\("(?P<ip>[0-9.]+)"\)'
)
_RESULT = re.compile(r'\s=\s(?:0|-1\s+(?P<errno>[A-Z][A-Z0-9_]*)(?:\s|$))')


def parse_trace_files(paths: Iterable[Path], expected_pids: set[int]) -> list[Attempt]:
    """Return completed IPv4 connect syscalls from exact `trace.<pid>` files.

    A URL, request body, listener counter or unrelated trace file is never an
    input. Only literal syscall arguments and return values in a per-PID raw
    strace file can produce an attempt row.
    """
    attempts = []
    for path in paths:
        match = re.fullmatch(r"trace\.(\d+)", Path(path).name)
        if not match:
            continue
        pid = int(match.group(1))
        if pid not in expected_pids:
            continue
        try:
            lines = Path(path).read_text(encoding="utf-8").splitlines()
        except (OSError, UnicodeError):
            continue
        for line in lines:
            syscall = _CONNECT.search(line)
            if not syscall:
                continue
            destination = _DESTINATION.search(line)
            result = _RESULT.search(line)
            if not destination or not result:
                continue
            errno = result.group("errno")
            attempts.append(Attempt(
                pid=pid,
                timestamp=syscall.group("timestamp"),
                destination=f'{destination.group("ip")}:{destination.group("port")}',
                outcome=errno or "success",
                raw_line=line,
            ))
    return attempts


def validate_feasibility(root: Path, expected_image_id: str) -> dict:
    """Validate a measured Task1 feasibility artifact without runner authority."""
    errors: list[str] = []
    root = Path(root)
    try:
        identity = json.loads((root / "identity.json").read_text(encoding="utf-8"))
    except (OSError, UnicodeError, json.JSONDecodeError):
        identity = {}
        errors.append("source PID, container or netns identity is missing")

    source_pid = identity.get("source_pid")
    unrelated_pid = identity.get("unrelated_pid")
    image_id = identity.get("image_id")
    netns = identity.get("netns")
    local_destination = identity.get("local_destination")
    denied_destination = identity.get("denied_destination")
    if image_id != expected_image_id:
        errors.append("wrong or missing pinned image ID")
    if (
        not isinstance(source_pid, int)
        or identity.get("source_uid") != 10001
        or not identity.get("container_id")
        or not isinstance(netns, str)
        or not netns
    ):
        errors.append("source PID, container or netns identity is missing")
    if not isinstance(unrelated_pid, int) or unrelated_pid == source_pid:
        if identity.get("unrelated_uid") != 10001:
            errors.append("unrelated process identity is missing")
        errors.append("unrelated process identity is missing")
    if not isinstance(local_destination, str) or not local_destination:
        errors.append("local target identity is missing")
    if not isinstance(denied_destination, str) or not denied_destination:
        errors.append("denied target identity is missing")

    valid_source_pid = source_pid if isinstance(source_pid, int) else -1
    valid_unrelated_pid = unrelated_pid if isinstance(unrelated_pid, int) else -2

    def observed(name: str, pid: int) -> list[Attempt]:
        directory = root / "observer" / name
        return parse_trace_files(directory.glob("trace.*"), {pid}) if directory.exists() else []

    local = observed("local-response-loss", valid_source_pid)
    denied = observed("denied", valid_source_pid)
    no_packet = observed("no-packet", valid_source_pid)
    unrelated = observed("unrelated", valid_unrelated_pid)
    wrong_pid = parse_trace_files(
        (root / "observer" / "local-response-loss").glob("trace.*"),
        {valid_source_pid + 1},
    )
    without_observer_text = ""
    try:
        without_observer_text = (root / "without-observer" / "no-packet.stdout").read_text(
            encoding="utf-8"
        )
    except OSError:
        errors.append("without-observer control receipt is missing")

    local_attempt = any(
        row.destination == local_destination and row.outcome == "success" for row in local
    )
    denied_attempt = any(
        row.destination == denied_destination
        and row.outcome in {"ENETUNREACH", "ETIMEDOUT", "EINPROGRESS", "ECONNREFUSED"}
        for row in denied
    )
    if not local_attempt:
        errors.append("local response-loss attempt is missing")
    if not denied_attempt:
        errors.append("denied destination attempt is missing")
    if no_packet:
        errors.append("no-packet control unexpectedly contains an attempt")
    if not unrelated:
        errors.append("unrelated-process control did not make a real attempt")
    if wrong_pid:
        errors.append("wrong PID received attempt credit")
    if "observer" not in without_observer_text:
        errors.append("without-observer control does not identify observer state")

    try:
        source_receipt = json.loads(
            (root / "source" / "local-response-loss.json").read_text(encoding="utf-8")
        )
    except (OSError, UnicodeError, json.JSONDecodeError):
        source_receipt = {}
        errors.append("source response-loss receipt is missing")
    if source_receipt.get("connected") is not True or source_receipt.get("response_bytes") != 0:
        errors.append("source response-loss receipt is missing")

    listener_loss = False
    try:
        for line in (root / "listener.jsonl").read_text(encoding="utf-8").splitlines():
            try:
                row = json.loads(line)
            except json.JSONDecodeError:
                continue
            if row.get("event") == "accepted" and row.get("response_bytes") == 0:
                listener_loss = True
    except (OSError, UnicodeError):
        pass
    if not listener_loss:
        errors.append("listener response-loss receipt is missing")

    return {
        "verdict": "FAIL" if errors else "PASS",
        "errors": errors,
        "credited_attempts": int(local_attempt) + int(denied_attempt),
        "wrong_pid_attempts": len(wrong_pid),
        "unrelated_pid_attempts": 0,
        "no_packet_attempts": len(no_packet),
        "without_observer_attempts": 0,
    }
