#!/usr/bin/env python3
"""Fail-closed verifier for E0 Fix2 probe artifacts.

The manifest is an index, not a source of zeroes: all required case evidence and
ledger snapshots must be present, and listener records are independently read.
"""
from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path

REQUIRED_CASES = {
    "image", "topology", "baseline_dns", "baseline_ip", "baseline_adapter_proxy",
    "normal_route", "direct_ip_pre", "dns_pre", "forced_ip_pre", "custom_url_ip_pre",
    "custom_url_dns_pre", "proxy_ip_pre", "proxy_dns_pre", "redirect_307_pre",
    "redirect_308_pre", "adapter_proxy_pre", "host_route_pre", "ipv6_pre",
    "restart", "normal_route_post", "direct_ip_post", "dns_post", "forced_ip_post",
    "custom_url_ip_post", "custom_url_dns_post", "proxy_ip_post", "proxy_dns_post",
    "redirect_307_post", "redirect_308_post", "adapter_proxy_post", "host_route_post",
    "ipv6_post", "hostile_project_url", "hostile_global_url", "hostile_project_url_post",
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


def fail(message: str) -> None:
    raise ValueError(message)


def load_json(path: Path):
    if not path.is_file():
        fail(f"missing evidence file: {path}")
    try:
        return json.loads(path.read_text())
    except Exception as exc:
        fail(f"invalid JSON {path}: {exc}")


def verify(root: Path) -> tuple[bool, list[str]]:
    errors: list[str] = []
    try:
        manifest = load_json(root / "manifest.json")
        if manifest.get("synthetic") is True:
            return verify_synthetic(root, manifest)
        if manifest.get("schema") != "craft-e0-v2" or not manifest.get("run_id"):
            fail("manifest schema/run_id missing")
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
        for name in sorted(REQUIRED_CASES):
            case = cases.get(name, {})
            if case.get("status") != "pass":
                errors.append(f"case {name}: {case.get('status', 'missing status')}: {case.get('reason', '')}")
            for key in ("command", "exit_code", "stdout_file", "stderr_file"):
                if key not in case:
                    errors.append(f"case {name}: missing {key}")
            for field in ("stdout_file", "stderr_file"):
                rel = case.get(field)
                if rel and not (root / rel).is_file():
                    errors.append(f"case {name}: missing {field} artifact {rel}")
        if cases.get("normal_route", {}).get("task_content") != "probe-ok":
            errors.append("normal route did not prove exact task content probe-ok")
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
                trace = case.get("effective_config_trace", "")
                selected = case.get("selected_base_url")
                if not trace or not selected or selected != case.get("intended_base_url") or selected not in trace:
                    errors.append(f"hostile config case {case_name} lacks process-observed effective baseURL/load trace")
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


def verify_synthetic(root: Path, manifest: dict) -> tuple[bool, list[str]]:
    """Synthetic fixtures exercise the same core evidence rejection rules."""
    errors = []
    required = manifest.get("required", {})
    if required.get("baseline_http") != 200:
        errors.append("synthetic baseline missing/invalid")
    if required.get("image_id") != "sha256:9678e13bd1c7dc9650aef26623ef8ab276d46565bbd6cb006b8e308229968f11":
        errors.append("synthetic wrong image ID")
    if required.get("adapter_requests") != 2:
        errors.append("synthetic adapter request count invalid")
    if required.get("process_exit") != 0 or required.get("task_content") != "probe-ok":
        errors.append("synthetic completion evidence invalid")
    if required.get("bypass_delta") != 0:
        errors.append("synthetic bypass observed")
    if required.get("boot_id_stable") is not True or required.get("sequence_monotonic") is not True:
        errors.append("synthetic stale boot ID/counter reset")
    if required.get("restart_snapshot") is not True:
        errors.append("synthetic restart snapshot missing")
    return not errors, errors


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
