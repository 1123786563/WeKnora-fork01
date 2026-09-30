#!/usr/bin/env python3
"""Fail-closed verifier for E0 Fix2 probe artifacts.

The manifest is an index, not a source of zeroes: all required case evidence and
ledger snapshots must be present, and listener records are independently read.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import sys
from pathlib import Path
from urllib.parse import urlsplit

REQUIRED_CASES = {
    "image", "topology", "baseline_dns", "baseline_ip", "baseline_adapter_proxy",
    "normal_route", "direct_ip_pre", "dns_pre", "forced_ip_pre", "custom_url_ip_pre",
    "custom_url_dns_pre", "proxy_ip_pre", "proxy_dns_pre", "redirect_307_pre",
    "redirect_308_pre", "adapter_proxy_pre", "host_route_pre", "ipv6_pre",
    "restart", "normal_route_post", "direct_ip_post", "dns_post", "forced_ip_post",
    "custom_url_ip_post", "custom_url_dns_post", "proxy_ip_post", "proxy_dns_post",
    "redirect_307_post", "redirect_308_post", "adapter_proxy_post", "host_route_post",
    "ipv6_post", "proxy_opencode_pre", "proxy_opencode_post", "hostile_project_url", "hostile_global_url", "hostile_project_url_post",
    "hostile_project_dns_post", "hostile_global_url_post", "restore_route",
    "fault_503", "fault_disconnect", "cleanup",
}
REQUIRED_LISTENERS = {"gateway", "provider", "proxy", "host", "adapter"}
EXTERNAL_SINKS = ("gateway", "provider", "proxy", "host")
NEGATIVE_CASES = {
    "direct_ip_pre", "dns_pre", "forced_ip_pre", "custom_url_ip_pre",
    "proxy_ip_pre", "proxy_dns_pre", "direct_ip_post", "dns_post", "forced_ip_post",
    "custom_url_ip_post", "custom_url_dns_post", "proxy_ip_post", "proxy_dns_post",
}
CONFIG_CASES = {
    "hostile_project_url", "hostile_global_url", "hostile_project_url_post",
    "hostile_project_dns_post", "hostile_global_url_post", "custom_url_dns_pre",
}
CONTROLLED_CASES = {
    "direct_ip_pre", "dns_pre", "forced_ip_pre", "custom_url_ip_pre", "custom_url_dns_pre",
    "proxy_ip_pre", "proxy_dns_pre", "redirect_307_pre", "redirect_308_pre", "adapter_proxy_pre",
    "host_route_pre", "ipv6_pre", "proxy_opencode_pre", "hostile_project_url", "hostile_global_url",
    "direct_ip_post", "dns_post", "forced_ip_post", "custom_url_ip_post", "custom_url_dns_post",
    "proxy_ip_post", "proxy_dns_post", "redirect_307_post", "redirect_308_post", "adapter_proxy_post",
    "host_route_post", "ipv6_post", "proxy_opencode_post", "hostile_project_url_post",
    "hostile_project_dns_post", "hostile_global_url_post",
}
DIRECT_CONTROL_EVENTS = (
    (8080, "GET", "/v1/models", 200),
    (8081, "GET", "/v1/models", 200),
    (8082, "GET", "http://mock-provider:8081/v1/models", 200),
    (8082, "CONNECT", "mock-provider:443", 200),
)
PRE_CONTROL_CASES = {name for name in CONTROLLED_CASES if name.endswith("_pre") or name in {"hostile_project_url", "hostile_global_url"}}
POST_CONTROL_CASES = CONTROLLED_CASES - PRE_CONTROL_CASES


def expected_phase_schedule() -> list[tuple[str, str, str | None, str | None]]:
    schedule = [("positive-controls", "control", "baseline_dns", "initial")]
    for case_name in sorted(PRE_CONTROL_CASES):
        schedule.extend([
            (f"control:{case_name}:before", "control", case_name, "before"),
            (case_name, "negative", None, None),
            (f"control:{case_name}:after", "control", case_name, "after"),
        ])
    schedule.extend([("normal_route", "normal", None, None), ("restart", "restart", None, None)])
    for case_name in sorted(POST_CONTROL_CASES):
        schedule.extend([
            (f"control:{case_name}:before", "control", case_name, "before"),
            (case_name, "negative", None, None),
            (f"control:{case_name}:after", "control", case_name, "after"),
        ])
    schedule.extend([("normal_route_post", "normal", None, None), ("restore_route", "normal", None, None)])
    return schedule


def fail(message: str) -> None:
    raise ValueError(message)


def load_json(path: Path):
    if not path.is_file():
        fail(f"missing evidence file: {path}")
    try:
        return json.loads(path.read_text())
    except Exception as exc:
        fail(f"invalid JSON {path}: {exc}")


def _safe_evidence_path(root: Path, relative: str) -> Path:
    if root.is_symlink():
        fail("artifact root cannot be a symlink")
    if not isinstance(relative, str) or not relative or Path(relative).is_absolute() or any(part in {".", ".."} for part in Path(relative).parts):
        fail(f"invalid evidence path: {relative!r}")
    candidate = root / relative
    cursor = root
    for part in Path(relative).parts:
        cursor = cursor / part
        if cursor.is_symlink():
            fail(f"symlink is forbidden in evidence path: {relative}")
    resolved_root = root.resolve()
    resolved = candidate.resolve(strict=True)
    if resolved != resolved_root and resolved_root not in resolved.parents:
        fail(f"evidence path escapes artifact root: {relative}")
    return resolved


def _read_jsonl(path: Path, label: str) -> list[dict]:
    data = path.read_bytes()
    if not data or not data.endswith(b"\n"):
        fail(f"{label} is empty or has a truncated final record")
    rows = []
    for index, line in enumerate(data.splitlines(), 1):
        try:
            item = json.loads(line)
        except Exception as exc:
            fail(f"{label} line {index} invalid JSON: {exc}")
        if not isinstance(item, dict):
            fail(f"{label} line {index} is not an object")
        rows.append(item)
    return rows


def validate_source_streams(root: Path, manifest: dict) -> list[str]:
    """Validate one fixed D and A stream plus the host command/acknowledgment log."""
    errors = []
    try:
        streams = manifest.get("source_streams")
        if not isinstance(streams, dict) or set(streams) != {"direct", "adapter"}:
            fail("source_streams must name exactly direct and adapter")
        paths = {}
        events = {}
        for source in ("direct", "adapter"):
            path = _safe_evidence_path(root, streams[source])
            rows = _read_jsonl(path, f"{source} source stream")
            if len(rows) < 2 or rows[0].get("event") != "stream_start" or rows[-1].get("event") != "stream_end":
                errors.append(f"{source} stream is missing its start or final seal")
            declared_ports = set(rows[0].get("ports", [])) if rows and rows[0].get("event") == "stream_start" else set()
            seqs = [row.get("seq") for row in rows]
            if seqs != list(range(1, len(rows) + 1)):
                errors.append(f"{source} source sequence is not contiguous from 1")
            boots = {row.get("boot_id") for row in rows}
            if len(boots) != 1 or not next(iter(boots), None):
                errors.append(f"{source} source boot identity changed or is absent")
            monotonic = [row.get("monotonic_ns") for row in rows]
            if any(not isinstance(value, int) for value in monotonic) or monotonic != sorted(monotonic) or len(set(monotonic)) != len(monotonic):
                errors.append(f"{source} monotonic event order is missing or not strictly increasing")
            schemas = {
                "stream_start": {"source_identity", "ports", "initial_mode"},
                "phase_begin": {"ordinal", "phase_id", "phase_class", "mode", "active_connections", "active_requests"},
                "phase_barrier": {"ordinal", "phase_id", "phase_class", "active_connections", "active_requests"},
                "tcp_accept": {"connection_id", "peer", "local", "local_port"},
                "request_start": {"connection_id", "request_id", "request_ordinal", "method", "path", "untrusted_case_header"},
                "request_body": {"request_id", "body_length", "body_sha256", "body_complete", "observations"},
                "response_sent": {"request_id", "status"},
                "request_error": {"request_id", "kind", "detail"},
                "parse_error": {"connection_id", "detail"},
                "connection_close": {"connection_id", "reason"},
                "stream_end": {"ordinal", "final_seq", "event_count", "active_connections", "active_requests"},
            }
            common = {"run_id", "source", "boot_id", "seq", "time_ns", "monotonic_ns", "event", "phase_id"}
            for row in rows:
                kind = row.get("event")
                if row.get("run_id") != manifest.get("run_id") or row.get("source") != source:
                    errors.append(f"{source} source record has incorrect run/source ownership")
                    break
                if kind not in schemas:
                    errors.append(f"{source} source contains unknown event {kind!r}")
                    break
                if set(row) != common | schemas[kind]:
                    errors.append(f"{source} {kind} record does not match the fixed raw event schema")
                    break
                if not isinstance(row.get("time_ns"), int) or not isinstance(row.get("monotonic_ns"), int):
                    errors.append(f"{source} {kind} record lacks integer timestamps")
                    break
            expected_ports = {"direct": {8080, 8081, 8082, 8083}, "adapter": {8080}}[source]
            if set(rows[0].get("ports", [])) != expected_ports:
                errors.append(f"{source} stream_start port set is unexpected")
            end = rows[-1]
            if end.get("event") == "stream_end" and (end.get("final_seq") != len(rows) or end.get("event_count") != len(rows) or end.get("active_connections") != 0 or end.get("active_requests") != 0):
                errors.append(f"{source} final seal does not match stream length or zero-active state")
            paths[source], events[source] = path, rows

        command_path = _safe_evidence_path(root, manifest.get("phase_commands_file"))
        commands = _read_jsonl(command_path, "host phase command log")
        hash_index = manifest.get("raw_evidence_sha256")
        if not isinstance(hash_index, dict):
            fail("raw_evidence_sha256 index missing")
        expected_hash_paths = {streams["direct"], streams["adapter"], manifest.get("phase_commands_file"), manifest.get("host_commands_file"), manifest.get("inspect_evidence_file"), manifest.get("cleanup_evidence_file")}
        if set(hash_index) != expected_hash_paths:
            fail("raw evidence hash index does not cover exactly both source streams and the command log")
        for relative in expected_hash_paths:
            artifact_path = _safe_evidence_path(root, relative)
            actual_hash = hashlib.sha256(artifact_path.read_bytes()).hexdigest()
            if hash_index.get(relative) != actual_hash:
                errors.append(f"raw evidence SHA256 mismatch: {relative}")
        command_by_ordinal = {}
        inspected_helper_peer = _helper_peer_ip(root, manifest)
        last_ordinal = 0
        for command in commands:
            ordinal = command.get("ordinal")
            if not isinstance(ordinal, int) or ordinal != last_ordinal + 1:
                errors.append("host phase command ordinals are not contiguous")
                break
            last_ordinal = ordinal
            command_by_ordinal[ordinal] = command
            if command.get("op") not in {"begin_phase", "barrier", "seal"}:
                errors.append(f"host command {ordinal} has unknown operation")
            expected_command_keys = {"ordinal", "op", "phase_id", "source_cursors"}
            if command.get("op") == "begin_phase":
                expected_command_keys |= {"mode"}
                if command.get("mode") == "control":
                    expected_command_keys |= {"case_id", "window", "stop_command_id", "state_inspect_id", "helper_start_id", "helper_stop_id"}
                elif command.get("mode") == "restart":
                    expected_command_keys |= {"restart_command_id", "post_restart_inspect_id"}
            if set(command) != expected_command_keys:
                errors.append(f"host command {ordinal} does not match the fixed command-log schema")
            if set(command.get("source_cursors", {})) != {"direct", "adapter"}:
                errors.append(f"host command {ordinal} lacks both source acknowledgments")
        for source, rows in events.items():
            declared_ports = set(rows[0].get("ports", [])) if rows and rows[0].get("event") == "stream_start" else set()
            phase = None
            open_connections = {}
            all_connections = {}
            request_index = {}
            active_requests = set()
            source_ordinal = 0
            for row in rows:
                kind = row.get("event")
                if kind == "stream_start":
                    if row.get("source_identity") != source or not isinstance(row.get("ports"), list):
                        errors.append(f"{source} stream_start lacks fixed identity/ports")
                elif kind == "phase_begin":
                    command = command_by_ordinal.get(row.get("ordinal"), {})
                    source_ordinal += 1
                    if phase is not None or open_connections or active_requests:
                        errors.append(f"{source} phase began before prior phase drained")
                    if command.get("op") != "begin_phase" or command.get("phase_id") != row.get("phase_id") or command.get("mode") != row.get("mode"):
                        errors.append(f"{source} phase_begin lacks matching host command")
                    if command.get("source_cursors", {}).get(source) != row.get("seq"):
                        errors.append(f"{source} phase_begin acknowledgment cursor mismatch")
                    phase = {"id": row.get("phase_id"), "class": row.get("phase_class"), "command": command, "connections": {}}
                elif kind == "tcp_accept":
                    conn_id = row.get("connection_id")
                    if phase is None or not isinstance(conn_id, str) or not conn_id or conn_id in all_connections or row.get("phase_id") != phase["id"]:
                        errors.append(f"{source} TCP accept is unphased, duplicate, or has changed phase")
                    else:
                        expected_conn_id = f"{source}-c{len(all_connections) + 1}"
                        if conn_id != expected_conn_id:
                            errors.append(f"{source} connection ID/ordinal is not the generated contiguous value")
                        if row.get("local_port") not in declared_ports:
                            errors.append(f"{source} TCP accept used a port absent from stream_start")
                        try:
                            local_port = int(row.get("local", "").rsplit(":", 1)[-1].rstrip("]"))
                        except (TypeError, ValueError):
                            local_port = None
                        if local_port != row.get("local_port"):
                            errors.append(f"{source} TCP accept local address/port is inconsistent")
                        conn = {"id": conn_id, "phase_id": phase["id"], "local_port": row.get("local_port"), "peer": row.get("peer"), "requests": [], "close_count": 0}
                        open_connections[conn_id] = conn
                        all_connections[conn_id] = conn
                        phase["connections"][conn_id] = conn
                        if source == "direct" and phase["class"] != "control":
                            errors.append(f"direct sink accepted TCP during non-control phase {phase['id']}")
                elif kind == "request_start":
                    conn = open_connections.get(row.get("connection_id"))
                    request_id = row.get("request_id")
                    if conn is None or not isinstance(request_id, str) or request_id in request_index or row.get("phase_id") != conn.get("phase_id"):
                        errors.append(f"{source} request_start is unowned, duplicate, or has changed phase")
                    else:
                        ordinal = len(conn["requests"]) + 1
                        if row.get("request_ordinal") != ordinal or request_id != f"{conn['id']}-r{ordinal}":
                            errors.append(f"{source} request ID/ordinal is not the generated contiguous value")
                        if conn["requests"] and conn["requests"][-1].get("terminal") is None:
                            errors.append(f"{source} keep-alive request started before previous request terminal")
                        request = {"id": request_id, "ordinal": ordinal, "method": row.get("method"), "path": row.get("path"), "body": False, "status": None, "error": False, "terminal": None}
                        conn["requests"].append(request)
                        request_index[request_id] = (conn, request)
                        active_requests.add(request_id)
                        if "case" in row:
                            errors.append(f"{source} request_start contains an authoritative case field")
                elif kind == "request_body":
                    owner = request_index.get(row.get("request_id"))
                    if owner is None:
                        errors.append(f"{source} request_body is unowned")
                        continue
                    conn, request = owner
                    if request["body"] or request["terminal"] is not None or row.get("phase_id") != conn.get("phase_id") or not isinstance(row.get("body_complete"), bool) or not isinstance(row.get("body_sha256"), str):
                        errors.append(f"{source} request_body is missing, duplicated, or inconsistent")
                    else:
                        request["body"] = True
                        request["body_complete"] = row.get("body_complete")
                        request["observations"] = row.get("observations")
                        if not isinstance(row.get("body_length"), int) or row.get("body_length") < 0 or len(row.get("body_sha256", "")) != 64:
                            errors.append(f"{source} request_body lacks a bounded length or SHA256")
                elif kind in {"response_sent", "request_error"}:
                    if source == "direct" and kind == "request_error":
                        errors.append("direct sink request_error is unconditionally fail-closed")
                    owner = request_index.get(row.get("request_id"))
                    if owner is None:
                        errors.append(f"{source} {kind} is unowned")
                        continue
                    _, request = owner
                    if request["terminal"] is not None:
                        errors.append(f"{source} request has more than one terminal event")
                    elif kind == "response_sent":
                        if row.get("request_id") not in active_requests or not request["body"] or request.get("body_complete") is not True:
                            errors.append(f"{source} response is not tied to one active complete request body")
                        request["status"] = row.get("status")
                        request["terminal"] = "response"
                        active_requests.discard(row.get("request_id"))
                    else:
                        request["error"] = True
                        request["terminal"] = "error"
                        active_requests.discard(row.get("request_id"))
                elif kind == "parse_error":
                    if source == "direct":
                        errors.append("direct sink parse_error is unconditionally fail-closed")
                    if row.get("connection_id") not in open_connections:
                        errors.append(f"{source} parse_error is unowned")
                elif kind == "connection_close":
                    conn = open_connections.pop(row.get("connection_id"), None)
                    if conn is None:
                        errors.append(f"{source} connection close is unowned or duplicated")
                    else:
                        conn["close_count"] += 1
                        if any(request["terminal"] is None for request in conn["requests"]):
                            errors.append(f"{source} connection closed without exactly one terminal per request")
                elif kind == "phase_barrier":
                    command = command_by_ordinal.get(row.get("ordinal"), {})
                    source_ordinal += 1
                    if phase is None or phase["id"] != row.get("phase_id") or open_connections or active_requests or row.get("active_connections") != 0 or row.get("active_requests") != 0:
                        errors.append(f"{source} barrier does not prove a drained matching phase")
                    if command.get("op") != "barrier" or command.get("phase_id") != row.get("phase_id") or command.get("source_cursors", {}).get(source) != row.get("seq"):
                        errors.append(f"{source} phase_barrier lacks matching host acknowledgment")
                    if phase is not None:
                        _verify_direct_phase(source, phase, errors, inspected_helper_peer)
                        _verify_adapter_phase(source, phase, errors)
                    phase = None
                elif kind == "stream_end":
                    command = command_by_ordinal.get(row.get("ordinal"), {})
                    source_ordinal += 1
                    if phase is not None or open_connections or active_requests or command.get("op") != "seal" or command.get("source_cursors", {}).get(source) != row.get("seq"):
                        errors.append(f"{source} stream_end lacks a drained host seal acknowledgment")
                if kind in {"phase_begin", "phase_barrier", "stream_end"} and row.get("ordinal") != source_ordinal:
                    errors.append(f"{source} source command ordinal mismatch")
            for conn in all_connections.values():
                if conn["close_count"] != 1:
                    errors.append(f"{source} accepted connection lacks exactly one close")
        if len(command_by_ordinal) != last_ordinal or not commands or commands[-1].get("op") != "seal":
            errors.append("host command log is missing its final seal")
        host_rows = _read_jsonl(_safe_evidence_path(root, manifest.get("host_commands_file")), "host command log")
        errors.extend(_validate_fixed_schedule_and_control_proofs(root, manifest, commands, host_rows, events))
    except (ValueError, OSError, TypeError, AttributeError, KeyError) as exc:
        errors.append(f"malformed source evidence: {exc}")
    return errors


def _fixed_invocation_errors(name, argv, names, direct_address):
    """Bind the high-risk transport attempts to verifier-owned target/env facts."""
    target = {
        "direct_ip_pre": f"http://{direct_address}:8080/direct",
        "direct_ip_post": f"http://{direct_address}:8080/direct",
        "dns_pre": "http://mock-gateway:8080/dns",
        "dns_post": "http://mock-gateway:8080/dns",
        "forced_ip_pre": [f"--resolve mock-provider:8081:{direct_address}", "http://mock-provider:8081/forced"],
        "forced_ip_post": [f"--resolve mock-provider:8081:{direct_address}", "http://mock-provider:8081/forced"],
        "custom_url_ip_pre": f"http://{direct_address}:8081/v1/models",
        "custom_url_ip_post": f"http://{direct_address}:8081/v1/models",
        "custom_url_dns_pre": "http://mock-provider:8081/v1/models",
        "custom_url_dns_post": "http://mock-hostile-provider:8081/v1/models",
        "proxy_ip_pre": [f"HTTP_PROXY=http://{direct_address}:8082", f"HTTPS_PROXY=http://{direct_address}:8082", f"ALL_PROXY=http://{direct_address}:8082", f"http_proxy=http://{direct_address}:8082", f"https_proxy=http://{direct_address}:8082", f"all_proxy=http://{direct_address}:8082", "NO_PROXY=", "no_proxy=", f"--proxy http://{direct_address}:8082", "http://probe.invalid/proxy-ip"],
        "proxy_ip_post": [f"HTTP_PROXY=http://{direct_address}:8082", f"HTTPS_PROXY=http://{direct_address}:8082", f"ALL_PROXY=http://{direct_address}:8082", "NO_PROXY=", "no_proxy=", f"--proxy http://{direct_address}:8082", "http://probe.invalid/post"],
        "proxy_dns_pre": ["HTTP_PROXY=http://mock-proxy:8082", "HTTPS_PROXY=http://mock-proxy:8082", "ALL_PROXY=http://mock-proxy:8082", "http_proxy=http://mock-proxy:8082", "https_proxy=http://mock-proxy:8082", "all_proxy=http://mock-proxy:8082", "NO_PROXY=", "no_proxy=", "--proxy http://mock-proxy:8082", "http://probe.invalid/proxy-dns"],
        "proxy_dns_post": ["HTTP_PROXY=http://mock-proxy:8082", "HTTPS_PROXY=http://mock-proxy:8082", "ALL_PROXY=http://mock-proxy:8082", "http_proxy=http://mock-proxy:8082", "https_proxy=http://mock-proxy:8082", "all_proxy=http://mock-proxy:8082", "NO_PROXY=", "no_proxy=", "--proxy http://mock-proxy:8082", "http://probe.invalid/post"],
    }.get(name)
    if target is None:
        return []
    if not isinstance(argv, list) or len(argv) < 6 or argv[:3] != ["docker", "exec", names.get("client")] or argv[3:5] != ["sh", "-c"]:
        return [f"raw host command for {name} does not use the fixed client transport invocation"]
    shell = argv[5]
    # These probe commands are serialized as a single shell argument; exact
    # target and all proxy variables are verifier-owned, not manifest strings.
    flattened = " ".join(shell.split())
    fixed_tokens = target if isinstance(target, list) else [target]
    if any(token not in flattened for token in fixed_tokens):
        return [f"raw host command for {name} changed the verifier-owned target or proxy environment"]
    if name in {"direct_ip_pre", "direct_ip_post", "dns_pre", "dns_post", "forced_ip_pre", "forced_ip_post", "custom_url_ip_pre", "custom_url_ip_post", "custom_url_dns_pre", "custom_url_dns_post"} and "--noproxy '*'" not in shell:
        return [f"raw host command for {name} changed its verifier-owned no-proxy policy"]
    if "curl" not in shell or "--max-time 3" not in shell:
        return [f"raw host command for {name} lacks the fixed bounded curl attempt"]
    return []


def validate_host_evidence(root: Path, manifest: dict) -> list[str]:
    """Reconstruct host commands, config bytes, Docker inspection and cleanup."""
    errors = []
    try:
        command_rel = manifest.get("host_commands_file")
        command_rows = _read_jsonl(_safe_evidence_path(root, command_rel), "host command log")
        allowed_host_kinds = {"case", "config", "operation"}
        if any(row.get("kind") not in allowed_host_kinds for row in command_rows):
            errors.append("raw host command log contains an unknown row kind")
        for operation in (row for row in command_rows if row.get("kind") == "operation"):
            op = operation.get("operation")
            expected_schema = {
                "resource_create": {"kind", "operation", "operation_id", "resource", "resource_type", "argv", "exit_code", "timed_out"},
                "docker_stop": {"kind", "operation", "operation_id", "argv", "exit_code", "timed_out", "container_id"},
                "client_state_inspect": {"kind", "operation", "operation_id", "argv", "exit_code", "timed_out", "container_id", "state"},
                "docker_start": {"kind", "operation", "operation_id", "argv", "exit_code", "timed_out", "container_id", "peer_ip"},
                "container_restart": {"kind", "operation", "operation_id", "argv", "exit_code", "timed_out"},
                "client_post_restart_inspect": {"kind", "operation", "operation_id", "argv", "exit_code", "container_id"},
            }.get(op)
            if expected_schema is None or set(operation) != expected_schema:
                errors.append(f"raw host operation {operation.get('operation_id')} has an unknown operation or non-fixed schema")
        try:
            raw_inspect_for_argv = load_json(_safe_evidence_path(root, manifest.get("inspect_evidence_file")))
            names_for_argv = manifest.get("names", {})
            direct_address = raw_inspect_for_argv["pre"]["external_network"]["Containers"][names_for_argv.get("direct")]["IPv4Address"].split("/", 1)[0]
        except (ValueError, OSError, TypeError, KeyError, AttributeError):
            raw_inspect_for_argv, names_for_argv, direct_address = {}, manifest.get("names", {}), None
        case_rows = [row for row in command_rows if row.get("kind") == "case"]
        if {row.get("case_id") for row in case_rows} != REQUIRED_CASES or len(case_rows) != len(REQUIRED_CASES):
            errors.append("raw host command log must contain exactly one row for every fixed required case")
        case_by_id = {row.get("case_id"): row for row in case_rows}
        for name in sorted(REQUIRED_CASES):
            row = case_by_id.get(name)
            case = manifest.get("cases", {}).get(name, {})
            if row is None:
                continue
            if set(row) != {"kind", "case_id", "argv", "exit_code", "timed_out", "stdout_file", "stderr_file", "elapsed_s"}:
                errors.append(f"raw host command for {name} has an unexpected schema")
                continue
            if (row["argv"] != case.get("command") or row["exit_code"] != case.get("exit_code") or
                    row["timed_out"] != case.get("timed_out", False) or row["stdout_file"] != case.get("stdout_file") or
                    row["stderr_file"] != case.get("stderr_file")):
                errors.append(f"case {name}: manifest command summary differs from raw host command")
            if not isinstance(row["argv"], list) or not row["argv"] or not isinstance(row["elapsed_s"], (int, float)) or row["elapsed_s"] < 0:
                errors.append(f"raw host command for {name} lacks exact argv or elapsed result")
            errors.extend(_fixed_invocation_errors(name, row.get("argv"), names_for_argv, direct_address))
            if row["timed_out"] is not False or not isinstance(row["exit_code"], int):
                errors.append(f"raw host command for {name} timed out or lacks an exit code")
            for field in ("stdout_file", "stderr_file"):
                raw_path = _safe_evidence_path(root, row[field])
                raw_bytes = raw_path.read_bytes()
                if field == "stdout_file":
                    case_stdout = raw_bytes.decode(errors="replace")
                    if name in {"normal_route", "normal_route_post", "restore_route"} and "probe-ok" not in case_stdout:
                        errors.append(f"raw {name} command output lacks probe-ok")

        config_rows = [row for row in command_rows if row.get("kind") == "config" ]
        if {row.get("case_id") for row in config_rows} != CONFIG_CASES or len(config_rows) != len(CONFIG_CASES):
            errors.append("raw host command log lacks the fixed config evidence case set")
        config_by_case = {row.get("case_id"): row for row in config_rows}
        for name in CONFIG_CASES:
            row = config_by_case.get(name)
            if row is None:
                continue
            if set(row) != {"kind", "case_id", "config_file", "config_sha256", "trace_file"}:
                errors.append(f"config evidence for {name} has an unexpected schema")
                continue
            config_path = _safe_evidence_path(root, row["config_file"])
            trace_path = _safe_evidence_path(root, row["trace_file"])
            config_bytes = config_path.read_bytes()
            actual_hash = hashlib.sha256(config_bytes).hexdigest()
            wanted = manifest.get("cases", {}).get(name, {}).get("intended_base_url")
            if row["config_sha256"] != actual_hash:
                errors.append(f"config bytes SHA256 mismatch for {name}")
            try:
                document = json.loads(config_bytes)
            except (ValueError, TypeError):
                document = {}
                errors.append(f"retained config bytes are not JSON for {name}")
            trace_rows = _read_jsonl(trace_path, f"structured OpenCode trace for {name}")
            provider_id, model = ("evil", "model") if name in {"hostile_global_url", "hostile_global_url_post"} else ("mock", "mock-model")
            process_rows = [item for item in trace_rows if item.get("event") == "process_start"]
            selection_rows = [item for item in trace_rows if item.get("event") == "provider_selection"]
            attempt_rows = [item for item in trace_rows if item.get("event") == "network_attempt"]
            if len(trace_rows) != 3 or len(process_rows) != 1 or len(selection_rows) != 1 or len(attempt_rows) != 1:
                errors.append(f"OpenCode structured process/provider/network observability is unavailable for {name}; selected URL and attempt cannot be proven")
                continue
            process, selection, attempt = process_rows[0], selection_rows[0], attempt_rows[0]
            if set(process) != {"event", "case_id", "pid", "argv_sha256", "config_file", "config_sha256"} or process.get("case_id") != name or not isinstance(process.get("pid"), int) or process.get("config_file") != row["config_file"] or process.get("config_sha256") != actual_hash or not isinstance(process.get("argv_sha256"), str) or len(process.get("argv_sha256", "")) != 64:
                errors.append(f"OpenCode process-start trace is not bound to retained config bytes for {name}")
            if set(selection) != {"event", "case_id", "pid", "provider_id", "model", "selected_base_url"} or selection.get("case_id") != name or selection.get("pid") != process.get("pid") or selection.get("provider_id") != provider_id or selection.get("model") != model or selection.get("selected_base_url") != wanted:
                errors.append(f"OpenCode provider-selection trace does not prove the fixed provider/model/baseURL for {name}")
            if set(attempt) != {"event", "case_id", "pid", "target_url", "outcome"} or attempt.get("case_id") != name or attempt.get("pid") != process.get("pid"):
                errors.append(f"OpenCode network-attempt trace is not tied to selected process for {name}")
            provider = document.get("provider", {}).get(provider_id, {}) if isinstance(document, dict) else {}
            configured_base = provider.get("options", {}).get("baseURL") if isinstance(provider, dict) else None
            configured_models = provider.get("models", {}) if isinstance(provider, dict) else {}
            if configured_base != wanted or model not in configured_models:
                errors.append(f"retained config bytes do not define the traced provider/model/baseURL for {name}")
            try:
                base_url = urlsplit(wanted or "")
                target_url = urlsplit(attempt.get("target_url", ""))
                if not wanted or not target_url.scheme or target_url.netloc != base_url.netloc or target_url.scheme != base_url.scheme or attempt.get("outcome") not in {"connection_refused", "connect_timeout", "network_unreachable"}:
                    errors.append(f"OpenCode network-attempt trace does not prove denied traffic to selected baseURL for {name}")
            except (TypeError, ValueError):
                errors.append(f"OpenCode network-attempt target is malformed for {name}")
            if selection.get("selected_base_url") != manifest.get("cases", {}).get(name, {}).get("selected_base_url"):
                errors.append(f"case {name}: selected baseURL summary differs from raw process trace")

        inspect_rel = manifest.get("inspect_evidence_file")
        inspect = load_json(_safe_evidence_path(root, inspect_rel))
        participants = {"client", "adapter", "direct", "helper"}
        if not isinstance(inspect, dict) or set(inspect) != {"pre", "post_restart"}:
            errors.append("raw inspect evidence must include pre and post_restart snapshots")
        else:
            for stage in ("pre", "post_restart"):
                snapshots = inspect[stage]
                if not isinstance(snapshots, dict) or set(snapshots) != participants | {"private_network", "external_network"}:
                    errors.append(f"raw {stage} inspect does not cover all fixed participants and networks")
                    continue
                for participant in participants:
                    value = snapshots[participant]
                    if not isinstance(value, dict) or not isinstance(value.get("container"), dict) or not value.get("container").get("Id"):
                        errors.append(f"raw {stage} inspect lacks container identity for {participant}")
                for network in ("private_network", "external_network"):
                    value = snapshots[network]
                    if not isinstance(value, dict) or not value.get("Name") or not isinstance(value.get("Containers"), dict):
                        errors.append(f"raw {stage} inspect lacks network membership for {network}")
                client = snapshots.get("client", {}).get("container", {})
                mounts = client.get("Mounts", [])
                host_config = client.get("HostConfig", {})
                if any(m.get("RW") is True and m.get("Type") == "bind" for m in mounts):
                    errors.append(f"raw {stage} client inspect exposes a writable host bind")
                if host_config.get("Privileged") or host_config.get("CapAdd") or host_config.get("NetworkMode") == "host":
                    errors.append(f"raw {stage} client inspect has unsafe privilege/network mode")
                client_name = manifest.get("names", {}).get("client")
                adapter_name = manifest.get("names", {}).get("adapter")
                direct_name = manifest.get("names", {}).get("direct")
                helper_name = manifest.get("names", {}).get("helper")
                private_name = manifest.get("names", {}).get("private")
                external_name = manifest.get("names", {}).get("external")
                private = snapshots.get("private_network", {})
                external = snapshots.get("external_network", {})
                if private.get("Name") != private_name or set(private.get("Containers", {})) != {client_name, adapter_name} or private.get("Internal") is not True or private.get("Options", {}).get("com.docker.network.bridge.gateway_mode_ipv4") != "isolated":
                    errors.append(f"raw {stage} private-network membership/internal gateway mode is not the fixed topology")
                if external.get("Name") != external_name or set(external.get("Containers", {})) != {adapter_name, direct_name, helper_name} or external.get("Internal") is not False:
                    errors.append(f"raw {stage} external-network membership differs from the fixed topology")
                expected_mounts = {"/home/craft/.config/opencode": manifest.get("names", {}).get("config"), "/home/craft/.local/share/opencode": manifest.get("names", {}).get("data")}
                actual_mounts = {mount.get("Destination"): mount.get("Name", mount.get("Source")) for mount in mounts if mount.get("Type") == "volume"}
                if len(mounts) != 2 or actual_mounts != expected_mounts:
                    errors.append(f"raw {stage} client mount set is not the fixed XDG config/data volume pair")
                adapter = snapshots.get("adapter", {}).get("container", {})
                if adapter.get("HostConfig", {}).get("Sysctls", {}).get("net.ipv4.ip_forward") != "0":
                    errors.append(f"raw {stage} adapter inspect does not disable IP forwarding")
                helper_member = external.get("Containers", {}).get(helper_name, {})
                if not isinstance(helper_member.get("IPv4Address"), str) or not helper_member.get("IPv4Address"):
                    errors.append(f"raw {stage} external network lacks inspected helper peer identity")
            try:
                if inspect["pre"]["client"]["container"]["Id"] != inspect["post_restart"]["client"]["container"]["Id"]:
                    errors.append("raw restart inspect does not preserve the same immutable client container ID")
                if inspect["pre"]["client"]["container"].get("Mounts") != inspect["post_restart"]["client"]["container"].get("Mounts"):
                    errors.append("raw restart inspect does not preserve exact client mounts")
            except (KeyError, TypeError):
                errors.append("raw restart inspect cannot prove client ID/mount continuity")

        cleanup_rel = manifest.get("cleanup_evidence_file")
        cleanup_rows = _read_jsonl(_safe_evidence_path(root, cleanup_rel), "cleanup evidence")
        removals = [row for row in cleanup_rows if row.get("kind") == "remove"]
        absent = [row for row in cleanup_rows if row.get("kind") == "not_found_inspect"]
        creations = [row for row in command_rows if row.get("kind") == "operation" and row.get("operation") == "resource_create"]
        expected_types = {row.get("resource"): row.get("resource_type") for row in creations}
        expected_resources = set(expected_types)
        names = manifest.get("names", {})
        fixed_inventory = {names.get(key) for key in ("client", "adapter", "direct", "helper", "private", "external", "config", "data")}
        if None in fixed_inventory or expected_resources != fixed_inventory:
            errors.append("raw creation records do not cover exactly the fixed participant/network/XDG volume inventory")
        fixed_types = {names.get(key): resource_type for key, resource_type in {
            "client": "container", "adapter": "container", "direct": "container", "helper": "container",
            "private": "network", "external": "network", "config": "volume", "data": "volume",
        }.items()}
        if expected_types != fixed_types:
            errors.append("raw creation records changed a fixed resource type")
        if (not expected_resources or len(expected_types) != len(creations) or
                {row.get("resource") for row in removals} != expected_resources or len(removals) != len(expected_resources) or
                {row.get("resource") for row in absent} != expected_resources or len(absent) != len(expected_resources)):
            errors.append("raw cleanup inventory must be derived from unique successful resource creation records")
        for row in creations:
            if row.get("exit_code") != 0 or row.get("timed_out") is not False or row.get("resource_type") not in {"container", "network", "volume"}:
                errors.append("raw resource creation record is not a successful fixed-type allocation")
            resource, resource_type = row.get("resource"), row.get("resource_type")
            argv = row.get("argv", [])
            if resource_type == "container" and (argv[:4] != ["docker", "run", "-d", "--name"] or len(argv) < 5 or argv[4] != resource):
                errors.append("raw container creation argv does not register the fixed resource name")
            elif resource_type == "network" and (argv[:3] != ["docker", "network", "create"] or not argv or argv[-1] != resource):
                errors.append("raw network creation argv does not register the fixed resource name")
            elif resource_type == "volume" and argv != ["docker", "volume", "create", resource]:
                errors.append("raw volume creation argv does not register the fixed resource name")
        for row in removals:
            resource_type = expected_types.get(row.get("resource"))
            expected_argv = {"container": ["docker", "rm", "-f", "-v", row.get("resource")], "network": ["docker", "network", "rm", row.get("resource")], "volume": ["docker", "volume", "rm", row.get("resource")]} .get(resource_type)
            if set(row) != {"kind", "resource", "resource_type", "argv", "exit_code"} or row.get("resource_type") != resource_type or row.get("argv") != expected_argv or row.get("exit_code") != 0:
                errors.append("raw cleanup removal command failed or has an unexpected schema")
        for row in absent:
            resource_type = expected_types.get(row.get("resource"))
            inspect_cmd = {"container": ["docker", "inspect", row.get("resource")], "network": ["docker", "network", "inspect", row.get("resource")], "volume": ["docker", "volume", "inspect", row.get("resource")]} .get(resource_type)
            expected_diag = {"container": "no such container", "network": "no such network", "volume": "no such volume"}.get(resource_type, "")
            if set(row) != {"kind", "resource", "resource_type", "argv", "exit_code", "stderr"} or row.get("resource_type") != resource_type or row.get("argv") != inspect_cmd or row.get("exit_code") != 1 or expected_diag not in row.get("stderr", "").lower():
                errors.append("raw cleanup not-found inspection did not prove resource absence")
    except (ValueError, OSError, TypeError, AttributeError, KeyError) as exc:
        errors.append(f"malformed host evidence: {exc}")
    return errors


def _helper_peer_ip(root: Path, manifest: dict) -> str | None:
    """Derive the permitted direct-sink peer from the inspected helper member."""
    try:
        inspect = load_json(_safe_evidence_path(root, manifest.get("inspect_evidence_file")))
        helper_name = manifest.get("names", {}).get("helper")
        address = inspect["pre"]["external_network"]["Containers"][helper_name]["IPv4Address"]
        return address.split("/", 1)[0]
    except (ValueError, OSError, TypeError, KeyError, AttributeError):
        return None


def _validate_fixed_schedule_and_control_proofs(root, manifest, commands, host_rows, events):
    errors = []
    begins = [row for row in commands if row.get("op") == "begin_phase"]
    actual = []
    for row in begins:
        mode = row.get("mode")
        if mode == "control":
            actual.append((row.get("phase_id"), "control", row.get("case_id"), row.get("window")))
        elif mode == "normal":
            actual.append((row.get("phase_id"), "normal", None, None))
        elif mode == "restart":
            actual.append((row.get("phase_id"), "restart", None, None))
        else:
            actual.append((row.get("phase_id"), "negative", None, None))
    expected = expected_phase_schedule()
    if actual != expected:
        errors.append("host phase sequence differs from verifier-owned paired control schedule")

    operations = {row.get("operation_id"): row for row in host_rows if row.get("kind") == "operation"}
    if len(operations) != sum(1 for row in host_rows if row.get("kind") == "operation"):
        errors.append("raw host operation IDs are missing or duplicated")
    client_id = None
    helper_id = None
    helper_peer_ip = None
    try:
        inspect = load_json(_safe_evidence_path(root, manifest.get("inspect_evidence_file")))
        client_id = inspect["pre"]["client"]["container"]["Id"]
        helper_id = inspect["pre"]["helper"]["container"]["Id"]
        helper_peer_ip = _helper_peer_ip(root, manifest)
    except (ValueError, OSError, TypeError, KeyError, AttributeError):
        errors.append("raw inspect cannot establish immutable client/helper identities")

    for phase in (row for row in begins if row.get("mode") == "control"):
        refs = {
            "stop_command_id": ("docker_stop", ["docker", "stop", "--timeout", "2", manifest.get("names", {}).get("client")]),
            "state_inspect_id": ("client_state_inspect", ["docker", "inspect", manifest.get("names", {}).get("client")]),
            "helper_start_id": ("docker_start", ["docker", "start", manifest.get("names", {}).get("helper")]),
            "helper_stop_id": ("docker_stop", ["docker", "stop", "--timeout", "2", manifest.get("names", {}).get("helper")]),
        }
        for ref, (kind, argv) in refs.items():
            operation = operations.get(phase.get(ref))
            if operation is None or operation.get("operation") != kind or operation.get("argv") != argv or operation.get("exit_code") != 0 or operation.get("timed_out") is not False:
                errors.append(f"control phase {phase.get('phase_id')} lacks successful raw {kind} proof")
                continue
            if ref == "state_inspect_id" and (operation.get("container_id") != client_id or operation.get("state") != {"Running": False, "Restarting": False, "Pid": 0}):
                errors.append(f"control phase {phase.get('phase_id')} did not prove the inspected client stopped")
            if ref == "helper_start_id" and operation.get("container_id") != helper_id:
                errors.append(f"control phase {phase.get('phase_id')} started a different helper identity")
            if ref == "helper_start_id" and operation.get("peer_ip") != helper_peer_ip:
                errors.append(f"control phase {phase.get('phase_id')} helper peer differs from raw inspected network identity")
            if ref == "helper_stop_id" and operation.get("container_id") != helper_id:
                errors.append(f"control phase {phase.get('phase_id')} stopped a different helper identity")
        valid_initial = phase.get("phase_id") == "positive-controls" and phase.get("case_id") == "baseline_dns" and phase.get("window") == "initial"
        valid_pair = phase.get("case_id") in CONTROLLED_CASES and phase.get("window") in {"before", "after"}
        if not (valid_initial or valid_pair):
            errors.append(f"control phase {phase.get('phase_id')} has unknown verifier-owned control identity")

    restart_phase = next((row for row in begins if row.get("mode") == "restart"), None)
    if restart_phase is None:
        errors.append("fixed schedule lacks the same-client restart checkpoint")
    else:
        restart = operations.get(restart_phase.get("restart_command_id"))
        post_inspect = operations.get(restart_phase.get("post_restart_inspect_id"))
        if restart is None or restart.get("operation") != "container_restart" or restart.get("argv") != ["docker", "restart", manifest.get("names", {}).get("client")] or restart.get("exit_code") != 0 or restart.get("timed_out") is not False:
            errors.append("raw restart command does not prove bounded success on the fixed client")
        if post_inspect is None or post_inspect.get("operation") != "client_post_restart_inspect" or post_inspect.get("argv") != ["docker", "inspect", manifest.get("names", {}).get("client")] or post_inspect.get("exit_code") != 0 or post_inspect.get("container_id") != client_id:
            errors.append("raw post-restart inspect does not prove same client identity")

    for source, rows in events.items():
        phase_ids = [row.get("phase_id") for row in rows if row.get("event") == "phase_begin"]
        if phase_ids != [item[0] for item in expected]:
            errors.append(f"{source} stream phase schedule differs from verifier-owned fixed sequence")
    return errors


def _verify_direct_phase(source, phase, errors, helper_peer_ip=None):
    if source != "direct":
        return
    command = phase["command"]
    if phase["class"] != "control":
        return
    if not helper_peer_ip:
        errors.append(f"direct control phase {phase['id']} lacks raw inspected helper peer address")
        return
    observed = []
    for conn in phase["connections"].values():
        requests = conn.get("requests", [])
        if len(requests) != 1 or conn.get("close_count") != 1:
            errors.append(f"direct control phase {phase['id']} requires exactly one request and close per connection")
        for request in requests:
            if request.get("terminal") != "response":
                errors.append(f"direct control phase {phase['id']} request lacks one successful response terminal")
            observed.append((conn.get("local_port"), request.get("method"), request.get("path"), request.get("status"), conn.get("peer")))
    wanted = [(port, method, path, status, helper_peer_ip) for port, method, path, status in DIRECT_CONTROL_EVENTS]
    normalized = []
    for port, method, path, status, peer in observed:
        try:
            peer_ip = peer.rsplit(":", 1)[0].strip("[]")
        except Exception:
            peer_ip = None
        normalized.append((port, method, path, status, peer_ip))
    if sorted(normalized, key=str) != sorted(wanted, key=str):
        errors.append(f"direct control phase {phase['id']} differs from verifier-owned exact events/helper peer")


def _verify_adapter_phase(source, phase, errors):
    if source != "adapter" or phase["id"] not in {"normal_route", "normal_route_post", "restore_route"}:
        return
    observations = []
    for connection in phase["connections"].values():
        observations.extend(connection.get("requests", []))
    if len(observations) != 2 or {row.get("observations", {}).get("purpose") for row in observations} != {"title", "task"}:
        errors.append(f"adapter normal phase {phase['id']} must contain exactly one title and one task request")
        return
    sessions = set()
    for row in observations:
        details = row.get("observations") or {}
        if row.get("method") != "POST" or row.get("path") != "/v1/chat/completions" or row.get("status") != 200 or row.get("body_complete") is not True:
            errors.append(f"adapter normal phase {phase['id']} has an incomplete or unexpected request/response")
        if details.get("model") != "mock-model" or details.get("stream") is not True:
            errors.append(f"adapter normal phase {phase['id']} has an unexpected model or streaming mode")
        if not isinstance(details.get("session_id"), str) or not details.get("session_id"):
            errors.append(f"adapter normal phase {phase['id']} lacks a source-observed session id")
        sessions.add(details.get("session_id"))
    if len(sessions) != 1:
        errors.append(f"adapter normal phase {phase['id']} did not use one observed session")


def verify(root: Path) -> tuple[bool, list[str]]:
    errors: list[str] = []
    try:
        manifest = load_json(root / "manifest.json")
        if manifest.get("schema") != "craft-e0-v2" or not manifest.get("run_id"):
            fail("manifest schema/run_id missing")
        if manifest.get("verdict") != "PASS":
            errors.append("manifest verdict is not PASS")
        errors.extend(validate_source_streams(root, manifest))
        errors.extend(validate_host_evidence(root, manifest))
        if manifest.get("image", {}).get("id") != "sha256:9678e13bd1c7dc9650aef26623ef8ab276d46565bbd6cb006b8e308229968f11":
            fail("wrong or missing pinned image ID")
        if manifest.get("image", {}).get("platform") != "linux/arm64" or manifest.get("image", {}).get("version") != "1.18.4":
            fail("wrong or missing image platform/version")
        cases = manifest.get("cases")
        if not isinstance(cases, dict):
            fail("manifest cases missing")
        missing = REQUIRED_CASES - cases.keys()
        if missing:
            errors.append("missing required cases: " + ", ".join(sorted(missing)))
        extra = cases.keys() - REQUIRED_CASES
        if extra:
            errors.append("unexpected cases outside fixed schedule: " + ", ".join(sorted(extra)))
        for name in sorted(REQUIRED_CASES):
            case = cases.get(name, {})
            if case.get("status") != "pass":
                errors.append(f"case {name}: {case.get('status', 'missing status')}: {case.get('reason', '')}")
            for key in ("command", "exit_code", "stdout_file", "stderr_file"):
                if key not in case:
                    errors.append(f"case {name}: missing {key}")
            if case.get("timed_out") is True or case.get("exit_code") is None:
                errors.append(f"case {name}: timed-out or missing process result")
            for field in ("stdout_file", "stderr_file"):
                rel = case.get(field)
                if rel:
                    try:
                        _safe_evidence_path(root, rel)
                    except (ValueError, OSError) as exc:
                        errors.append(f"case {name}: {field} path invalid: {exc}")
        if cases.get("normal_route", {}).get("task_content") != "probe-ok":
            errors.append("normal route did not prove exact task content probe-ok")
        normal = cases.get("normal_route", {})
        normal_out = ""
        if normal.get("stdout_file"):
            try:
                normal_out = _safe_evidence_path(root, normal["stdout_file"]).read_text(errors="replace")
            except (ValueError, OSError):
                pass
        if normal.get("exit_code") != 0 or normal.get("timed_out") is True or "probe-ok" not in normal_out:
            errors.append("normal route lacks actual successful process output containing probe-ok")
        if cases.get("normal_route_post", {}).get("task_content") != "probe-ok":
            errors.append("post-restart route did not prove exact task content probe-ok")
        for case_name in ("baseline_dns", "baseline_ip", "baseline_adapter_proxy"):
            if cases.get(case_name, {}).get("status") != "pass":
                errors.append(f"required positive control failed: {case_name}")
        if cases.get("topology", {}).get("client_attachment_count") != 1:
            errors.append("client must have exactly one private network attachment")
        topology = cases.get("topology", {})
        if topology.get("client_mounts_ledger") is not False or "/ledger" in topology.get("mount_sources", {}):
            errors.append("client can mount the host-owned append-only ledgers")
        client_name = manifest.get("names", {}).get("client")
        for snapshot_label in ("pre", "post-restart"):
            inspect_path = root / "snapshots" / f"{snapshot_label}-{client_name}-inspect.json"
            if not inspect_path.is_file():
                errors.append(f"raw {snapshot_label} client inspect is missing")
                continue
            try:
                inspected = load_json(inspect_path)
                container = inspected[0]
                mounts = container.get("Mounts", [])
                host_config = container.get("HostConfig", {})
                forbidden = [m.get("Destination", "") for m in mounts if any(m.get("Destination", "").rstrip("/") == base or m.get("Destination", "").startswith(base + "/") for base in ("/state", "/ledger", "/probe"))]
                if forbidden:
                    errors.append(f"raw {snapshot_label} client inspect exposes recorder/control fixture mounts: {sorted(forbidden)}")
                if host_config.get("Privileged") is True or host_config.get("CapAdd") or host_config.get("NetworkMode") == "host":
                    errors.append(f"raw {snapshot_label} client inspect has unsafe privilege/network mode")
            except (IndexError, TypeError, ValueError) as exc:
                errors.append(f"raw {snapshot_label} client inspect is malformed: {exc}")
        if topology.get("client_ledger_write_denied") is not True or topology.get("client_ledger_write_stdout", "").strip() != "denied":
            errors.append("client ledger write denial was not observed")
        if cases.get("topology", {}).get("private_members") != sorted([manifest.get("names", {}).get("client"), manifest.get("names", {}).get("adapter")]):
            errors.append("private network membership is not exactly client plus adapter")
        topology = cases.get("topology", {})
        private_files = sorted((root / "snapshots").glob("pre-*-private-network.json"))
        external_files = sorted((root / "snapshots").glob("pre-*-external-network.json"))
        private_name = manifest.get("names", {}).get("private")
        external_name = manifest.get("names", {}).get("external")
        if private_files:
            private_name = load_json(private_files[0])[0].get("Name", private_name)
        if external_files:
            external_name = load_json(external_files[0])[0].get("Name", external_name)
        if topology.get("private_internal") is not True or topology.get("isolated_gateway_mode") is not True:
            errors.append("private network internal/isolated gateway mode not verified")
        adapter_snapshot = root / "snapshots" / f"pre-{manifest.get('names', {}).get('adapter')}-inspect.json"
        adapter_inspect = load_json(adapter_snapshot) if adapter_snapshot.is_file() else []
        adapter_sysctls = adapter_inspect[0].get("HostConfig", {}).get("Sysctls", {}) if adapter_inspect else {}
        adapter_forward_disabled = topology.get("adapter_ip_forward_disabled") is True or adapter_sysctls.get("net.ipv4.ip_forward") == "0"
        if not adapter_forward_disabled:
            errors.append("adapter IP forwarding is not disabled by runtime inspect")
        if topology.get("client_networks") != [private_name]:
            errors.append("client has an unexpected network attachment")
        if set(topology.get("adapter_networks", [])) != {private_name, external_name}:
            errors.append("adapter network attachment set is unexpected")
        if topology.get("direct_networks") != [external_name]:
            errors.append("direct listener is attached to a non-external network")
        if topology.get("client_privileged") is not False or topology.get("client_cap_add") or topology.get("client_port_bindings") not in (None, {}):
            errors.append("client container has unexpected privilege, capabilities, or published ports")
        if cases.get("restart", {}).get("same_container") is not True or cases.get("restart", {}).get("same_mounts") is not True or cases.get("restart", {}).get("marker_persisted") is not True or cases.get("restart", {}).get("global_config_persisted") is not True:
            errors.append("restart persistence identity/marker evidence failed")
        if cases.get("topology", {}).get("ipv6_disabled") is not True or cases.get("topology", {}).get("non_loopback_ipv6_routes") != 0:
            errors.append("IPv6 route policy is missing or failed")
        facts_path = topology.get("runtime_facts_file")
        facts = load_json(root / facts_path) if facts_path else {}
        facts_stdout = facts.get("stdout", "")
        if "1.18.4" not in facts_stdout or "3557e87db8c7db70e8ebd42157df1246554120896b115c462b760ff248cf751e" not in facts_stdout:
            errors.append("client runtime facts do not prove exact in-container OpenCode version and binary hash")
        if topology.get("binary_sha256_match") is not True and "3557e87db8c7db70e8ebd42157df1246554120896b115c462b760ff248cf751e" not in facts_stdout:
            errors.append("manifest did not record the pinned in-container binary hash check")
        if cases.get("host_route_pre", {}).get("positive_control_http") != 200 or cases.get("host_route_pre", {}).get("positive_control_after_http") != 200 or cases.get("host_route_post", {}).get("positive_control_http") != 200 or cases.get("host_route_post", {}).get("positive_control_after_http") != 200:
            errors.append("host-facing positive control did not succeed before and after restart")
        if cases.get("host_route_pre", {}).get("bypass_delta") != 0 or cases.get("host_route_post", {}).get("bypass_delta") != 0:
            errors.append("host-route bypass listener observed traffic")
        for case_name in NEGATIVE_CASES:
            case = cases.get(case_name, {})
            if case.get("status") == "pass":
                stdout_file, stderr_file = case.get("stdout_file"), case.get("stderr_file")
                combined = ""
                if stdout_file and (root / stdout_file).is_file():
                    combined += (root / stdout_file).read_text(errors="replace")
                if stderr_file and (root / stderr_file).is_file():
                    combined += (root / stderr_file).read_text(errors="replace")
                http000 = case.get("http_000_required") is True or "HTTP=000" in combined
                if case.get("exit_code") in (None, 0) or not http000:
                    errors.append(f"negative case {case_name} lacks a concrete failed transport observation")
                if not isinstance(case.get("intended_attempt"), str) or not case["intended_attempt"]:
                    errors.append(f"negative case {case_name} lacks an intended route command")
                # Zero is reconstructed from case-tagged raw sink events below;
                # do not let the manifest's ledger_delta establish denial.
        for case_name in ("hostile_project_url", "hostile_global_url", "hostile_project_url_post", "hostile_project_dns_post", "hostile_global_url_post"):
            case = cases.get(case_name, {})
            if case.get("status") == "pass":
                selected = case.get("selected_base_url")
                if not selected or selected != case.get("intended_base_url"):
                    errors.append(f"hostile config case {case_name} summary differs from raw structured provider trace")
            if case.get("status") == "pass" and any(case.get("ledger_delta", {}).get(role) != 0 for role in EXTERNAL_SINKS):
                errors.append(f"hostile config case {case_name} reached an external listener")
        dns_config = cases.get("custom_url_dns_pre", {})
        if dns_config.get("status") == "pass":
            if dns_config.get("intended_base_url") != "http://mock-hostile-provider:8081/v1" or dns_config.get("adapter_requests") != 0:
                errors.append("host-mapped hostile DNS baseURL was not selected or still reached adapter")
            if any(dns_config.get("ledger_delta", {}).get(role) != 0 for role in EXTERNAL_SINKS):
                errors.append("host-mapped hostile DNS baseURL reached an external listener")
        for case_name in ("redirect_307_pre", "redirect_308_pre", "redirect_307_post", "redirect_308_post"):
            case = cases.get(case_name, {})
            if case.get("status") == "pass":
                if case.get("curl_follow_control_exit") != 0 or case.get("curl_follow_provider_requests", 0) < 1:
                    errors.append(f"redirect case {case_name} lacks a positive POST-follow control")
                if case.get("adapter_original_posts", 0) < 1 or case.get("ledger_delta", {}).get("provider") != 0:
                    errors.append(f"redirect case {case_name} lacks adapter POST or observed bypass")
        for case_name in ("adapter_proxy_pre", "adapter_proxy_post"):
            case = cases.get(case_name, {})
            delta = case.get("ledger_delta", {})
            external_delta = case.get("external_delta", sum(delta.get(role, 0) for role in EXTERNAL_SINKS))
            if case.get("status") == "pass" and external_delta != 0:
                errors.append(f"adapter proxy case {case_name} reached an external sink")
        ledgers = manifest.get("ledgers")
        if not isinstance(ledgers, dict) or REQUIRED_LISTENERS - ledgers.keys():
            errors.append("listener ledger index missing a required role")
        else:
            parsed = {}
            for role in sorted(REQUIRED_LISTENERS):
                rel = ledgers[role]
                p = root / rel
                if not p.is_file():
                    errors.append(f"missing {role} append-only ledger: {rel}")
                    continue
                events = []
                for lineno, line in enumerate(p.read_text().splitlines(), 1):
                    try:
                        event = json.loads(line)
                    except Exception as exc:
                        errors.append(f"{role} ledger line {lineno} invalid: {exc}")
                        continue
                    if event.get("run_id") != manifest["run_id"] or not event.get("boot_id") or not isinstance(event.get("seq"), int):
                        errors.append(f"{role} ledger line {lineno} missing run/boot/sequence identity")
                    events.append(event)
                seqs = [e["seq"] for e in events if isinstance(e.get("seq"), int)]
                boots = {e.get("boot_id") for e in events}
                if seqs and (seqs != sorted(seqs) or len(set(seqs)) != len(seqs)):
                    errors.append(f"{role} ledger sequence is not monotonic")
                if len(boots) != 1:
                    errors.append(f"{role} ledger boot ID changed or is absent")
                parsed[role] = events
            if "adapter" in parsed:
                ad = [e for e in parsed["adapter"] if e.get("event") == "http" and e.get("case") == "normal_route" and e.get("method") == "POST"]
                if len(ad) != 2 or {e.get("purpose") for e in ad} != {"title", "task"}:
                    errors.append(f"normal adapter route expected one title and one task POST; observed {len(ad)}")
                task = [e for e in ad if e.get("purpose") == "task"]
                if not task or task[0].get("stream") is not True or task[0].get("model") != "mock-model":
                    errors.append("normal task request did not use expected streaming mock-model route")
                if cases.get("normal_route", {}).get("adapter_requests") != len(ad):
                    errors.append("normal route adapter count differs from raw adapter ledger")
            positive_baselines = {"baseline_dns", "baseline_ip", "baseline_adapter_proxy"}
            for case_name in REQUIRED_CASES - positive_baselines:
                for role in EXTERNAL_SINKS:
                    hits = [e for e in parsed.get(role, []) if e.get("case") == case_name and e.get("event") in {"http", "http_parse_error"}]
                    if hits:
                        errors.append(f"case {case_name}: raw {role} ledger records {len(hits)} external request(s)")
            raw_http = [e for events in parsed.values() for e in events if e.get("event") == "http"]
            for role in ("gateway", "provider", "proxy"):
                if not any(e.get("role") == role and str(e.get("case", "")).startswith("control:") and e.get("method") == "GET" and e.get("status") == 200 and e.get("complete") is True for e in raw_http):
                    errors.append(f"raw {role} HTTP positive control missing")
            if not any(e.get("role") == "proxy" and str(e.get("case", "")).startswith("control:") and e.get("method") == "CONNECT" and e.get("status") == 200 and e.get("complete") is True for e in raw_http):
                errors.append("raw proxy CONNECT positive control missing")
            def case_event_count(role, case_name):
                if case_name in {"baseline_dns", "baseline_ip"}:
                    accepted = {f"baseline:{sink}:{kind}" for sink in ("gateway", "provider") for kind in ("dns", "ip")}
                elif case_name == "baseline_adapter_proxy":
                    accepted = {"baseline:proxy:absolute"}
                else:
                    accepted = {case_name}
                return sum(1 for event in parsed.get(role, []) if event.get("case") in accepted and event.get("event") in {"http", "http_parse_error"})

            reconstructed = {name: {role: case_event_count(role, name) for role in REQUIRED_LISTENERS} for name in REQUIRED_CASES}
            for case_name in REQUIRED_CASES:
                case = cases.get(case_name, {})
                declared = case.get("ledger_delta")
                if isinstance(declared, dict) and case.get("status") == "pass":
                    for role in REQUIRED_LISTENERS:
                        if declared.get(role) != reconstructed[case_name][role]:
                            errors.append(f"case {case_name}: manifest {role} delta {declared.get(role)} differs from raw tagged event count {reconstructed[case_name][role]}")
                lo, hi = case.get("ledger_start"), case.get("ledger_end")
                if case.get("status") == "pass" and case_name not in {"image", "topology"} and (not isinstance(lo, dict) or not isinstance(hi, dict)):
                    errors.append(f"case {case_name}: successful case lacks ledger ranges")
                if isinstance(lo, dict) and isinstance(hi, dict):
                    for role in REQUIRED_LISTENERS:
                        if role not in lo or role not in hi or not isinstance(lo[role], int) or not isinstance(hi[role], int) or hi[role] < lo[role] or hi[role] > sum(1 for e in parsed.get(role, []) if e.get("event") in {"http", "http_parse_error"}):
                            errors.append(f"case {case_name}: missing/stale/reset/out-of-bounds {role} ledger range")
                    for role in REQUIRED_LISTENERS:
                        if role in lo and role in hi and hi[role] - lo[role] != reconstructed[case_name][role]:
                            errors.append(f"case {case_name}: {role} range delta differs from raw tagged event count")
                    if case_name in {"host_route_pre", "host_route_post"}:
                        bypass = case.get("bypass_delta")
                    elif case_name.endswith(("_pre", "_post")) or case_name in {"custom_url_ip_pre", "custom_url_dns_pre", "hostile_project_url", "hostile_global_url", "proxy_ip_pre", "proxy_dns_pre", "adapter_proxy_pre", "direct_ip_pre", "dns_pre", "forced_ip_pre", "redirect_307_pre", "redirect_308_pre", "ipv6_pre"}:
                        bypass = sum(hi[r] - lo[r] for r in EXTERNAL_SINKS)
                    else:
                        bypass = 0
                    if bypass != 0:
                        errors.append(f"case {case_name}: external listener delta is {bypass}, expected zero")
        cleanup = cases.get("cleanup", {})
        if cleanup.get("resources_absent") is not True or cleanup.get("cleanup_exit") != 0:
            errors.append("cleanup evidence incomplete or failed")
        return not errors, errors
    except ValueError as exc:
        return False, [str(exc)]


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("artifact_dir", type=Path)
    args = parser.parse_args()
    passed, errors = verify(args.artifact_dir)
    checked = load_json(args.artifact_dir / "manifest.json")
    print(json.dumps({"verdict": "PASS" if passed else "BLOCKED", "synthetic": bool(checked.get("synthetic") or checked.get("evidence_type") == "synthetic"), "errors": errors}, indent=2))
    return 0 if passed else 1


if __name__ == "__main__":
    sys.exit(main())
