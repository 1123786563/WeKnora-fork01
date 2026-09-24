#!/usr/bin/env python3
"""Bounded, disposable E0 Fix2 topology/protocol experiment (max 20 minutes)."""
from __future__ import annotations

import hashlib
import json
import os
import re
import shutil
import signal
import socket
import subprocess
import sys
import time
import uuid
from pathlib import Path

IMAGE = "sha256:9678e13bd1c7dc9650aef26623ef8ab276d46565bbd6cb006b8e308229968f11"
EXPECTED_BIN = "3557e87db8c7db70e8ebd42157df1246554120896b115c462b760ff248cf751e"
ROOT = Path(__file__).resolve().parents[4]
FIXTURE = Path(__file__).resolve().parent
ARTROOT = FIXTURE / "e0-fix2"
RUN_ID = "craft-e0-" + time.strftime("%Y%m%dT%H%M%SZ", time.gmtime()) + "-" + str(os.getpid())
ART = ARTROOT / RUN_ID
NAMES = {k: f"{RUN_ID}-{v}" for k, v in {"private": "private", "external": "external", "adapter": "adapter", "direct": "direct", "client": "client", "config": "config", "data": "data"}.items()}
IMAGE_INFO = {"id": IMAGE, "platform": "linux/arm64", "version": "1.18.4", "binary_sha256": EXPECTED_BIN}
manifest = {"schema": "craft-e0-v2", "run_id": RUN_ID, "started_utc": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()), "image": IMAGE_INFO, "names": {"adapter": NAMES["adapter"], "client": NAMES["client"], "direct": NAMES["direct"], "private": NAMES["private"], "external": NAMES["external"]}, "cases": {}, "commands": [], "ledgers": {}, "cleanup_inventory": [], "synthetic": False}
DEADLINE = time.monotonic() + 20 * 60
INITIAL_EXIT = 0


def sha(path: Path) -> str:
    h = hashlib.sha256()
    with path.open("rb") as f:
        for block in iter(lambda: f.read(1024 * 1024), b""):
            h.update(block)
    return h.hexdigest()


def remaining() -> float:
    return max(0.1, DEADLINE - time.monotonic())


def wait_for_quiescence(process_probe, counter_probe, timeout=6.0, stable_samples=4, interval=0.5):
    """Require no client OpenCode process and stable sink counters.

    A timeout result is never treated as a denial until late children are gone
    and independent host-owned listener ledgers stop changing.
    """
    deadline = time.monotonic() + timeout
    last = None
    stable = 0
    while time.monotonic() < deadline:
        alive = process_probe()
        current = counter_probe()
        if not alive and current == last:
            stable += 1
            if stable >= stable_samples:
                return {"quiescent": True, "counter_samples": stable, "final_counts": current}
        else:
            stable = 0
        last = current
        time.sleep(min(interval, max(0, deadline-time.monotonic())))
    return {"quiescent": False, "counter_samples": stable, "final_counts": last}


def stop_and_quiesce_client():
    killer = """import os,signal
mine=os.getpid(); killed=[]
for p in os.listdir('/proc'):
 if p.isdigit() and int(p)!=mine:
  try: cmd=open('/proc/'+p+'/cmdline','rb').read().split(b'\\0')
  except Exception: continue
  if any(x==b'opencode' or x.endswith(b'/opencode') for x in cmd):
   try: os.kill(int(p),signal.SIGTERM); killed.append(p)
   except Exception: pass
print(','.join(killed))"""
    try:
        stopped = subprocess.run(["docker", "exec", NAMES["client"], "python3", "-c", killer], capture_output=True, text=True, timeout=5)
    except Exception as exc:
        return {"quiescent": False, "reason": f"SIGTERM probe failed: {exc}"}
    if stopped.returncode != 0:
        return {"quiescent": False, "reason": f"SIGTERM probe rc={stopped.returncode}: {stopped.stderr}"}

    def process_alive():
        probe_code = """import os
for p in os.listdir('/proc'):
 if not p.isdigit() or int(p)==os.getpid(): continue
 try: argv=open('/proc/'+p+'/cmdline','rb').read().split(b'\\0')
 except Exception: continue
 if any(x==b'opencode' or x.endswith(b'/opencode') for x in argv): print(p)
"""
        probe = subprocess.run(["docker", "exec", NAMES["client"], "python3", "-c", probe_code], capture_output=True, text=True, timeout=3)
        return probe.returncode != 0 or bool(probe.stdout.strip())

    result = wait_for_quiescence(process_alive, counts, timeout=min(8, remaining()))
    result["sigterm_pids"] = stopped.stdout.strip().split(",") if stopped.stdout.strip() else []
    result["sigterm_exit"] = stopped.returncode
    return result


def run(argv, timeout=30, capture=True, label=None, check=False):
    argv = [str(x) for x in argv]
    timeout = min(timeout, remaining())
    started = time.monotonic()
    try:
        p = subprocess.run(argv, capture_output=capture, text=True, timeout=timeout)
        rec = {"argv": argv, "exit_code": p.returncode, "elapsed_s": round(time.monotonic() - started, 3), "timed_out": False}
        if capture:
            rec["stdout"] = p.stdout
            rec["stderr"] = p.stderr
    except subprocess.TimeoutExpired as exc:
        rec = {"argv": argv, "exit_code": None, "elapsed_s": round(time.monotonic() - started, 3), "timed_out": True, "stdout": (exc.stdout or "").decode(errors="replace") if isinstance(exc.stdout, bytes) else (exc.stdout or ""), "stderr": (exc.stderr or "").decode(errors="replace") if isinstance(exc.stderr, bytes) else (exc.stderr or "")}
        p = None
        # docker exec may outlive its timed-out local CLI. Stop OpenCode and
        # prove both process death and stable independent sink counts.
        if len(argv) > 3 and argv[0:2] == ["docker", "exec"] and NAMES.get("client") in argv:
            rec["quiescence"] = stop_and_quiesce_client()
            rec["quiescence"]["original_timeout_diagnostics"] = {"stdout": rec.get("stdout", ""), "stderr": rec.get("stderr", "")}
    manifest["commands"].append(rec)
    if label:
        safe = re.sub(r"[^a-zA-Z0-9_.-]+", "_", label)
        out = ART / "logs" / f"{safe}.stdout.txt"
        err = ART / "logs" / f"{safe}.stderr.txt"
        out.parent.mkdir(parents=True, exist_ok=True)
        out.write_text(rec.get("stdout", ""))
        err.write_text(rec.get("stderr", ""))
        rec["stdout_file"] = str(out.relative_to(ART))
        rec["stderr_file"] = str(err.relative_to(ART))
    write_manifest()
    if check and (p is None or p.returncode != 0):
        raise RuntimeError(f"command failed ({label or 'command'}): rc={rec.get('exit_code')} timed_out={rec['timed_out']}: {rec.get('stderr', '')[-1000:]}")
    return rec


def write_manifest():
    ART.mkdir(parents=True, exist_ok=True)
    path = ART / "manifest.json"
    tmp = path.with_suffix(".json.tmp")
    tmp.write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n")
    os.replace(tmp, path)


def block(reason):
    global INITIAL_EXIT
    INITIAL_EXIT = 1
    manifest.setdefault("blockers", []).append(reason)
    write_manifest()
    raise RuntimeError(reason)


def set_case(name, status, command=None, reason="", **extra):
    safe = re.sub(r"[^a-zA-Z0-9_.-]+", "_", name)
    out = ART / "logs" / f"{safe}.stdout.txt"
    err = ART / "logs" / f"{safe}.stderr.txt"
    out.parent.mkdir(parents=True, exist_ok=True)
    out.touch(exist_ok=True); err.touch(exist_ok=True)
    c = {"status": status, "reason": reason, "command": command or [], "exit_code": extra.pop("exit_code", None), "stdout_file": str(out.relative_to(ART)), "stderr_file": str(err.relative_to(ART)), **extra}
    manifest["cases"][name] = c
    write_manifest()


def save_snapshot(name, value):
    path = ART / "snapshots" / f"{name}.json"
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, indent=2, sort_keys=True) + "\n")
    return str(path.relative_to(ART))


def json_cmd(argv, label, timeout=20):
    rec = run(argv, timeout=timeout, label=label)
    if rec.get("exit_code") != 0:
        raise RuntimeError(f"{label} failed: {rec.get('stderr', '')}")
    return json.loads(rec.get("stdout", ""))


def docker(*args, timeout=30, label=None, check=False):
    return run(["docker", *args], timeout=timeout, label=label, check=check)


def ledger_events():
    result = {}
    for role in ("gateway", "provider", "proxy", "host", "adapter"):
        p = ART / "ledgers" / f"{role}.jsonl"
        result[role] = []
        if p.exists():
            for line in p.read_text().splitlines():
                if line.strip():
                    result[role].append(json.loads(line))
        manifest["ledgers"][role] = str(p.relative_to(ART))
    return result


def counts():
    # Count completed HTTP observations, not TCP accepts. This matches the
    # per-case identity carried in each append-only request record.
    ev = ledger_events()
    return {role: sum(1 for row in rows if row.get("event") in {"http", "http_parse_error"}) for role, rows in ev.items()}


def append_case_files(name, rec, extra=None):
    c = dict(rec)
    c["command"] = rec.get("argv", [])
    if "stdout_file" not in c:
        base = ART / "logs" / name
        base.parent.mkdir(parents=True, exist_ok=True)
        (base.with_suffix(".stdout.txt")).write_text(c.get("stdout", ""))
        (base.with_suffix(".stderr.txt")).write_text(c.get("stderr", ""))
        c["stdout_file"] = str(base.with_suffix(".stdout.txt").relative_to(ART))
        c["stderr_file"] = str(base.with_suffix(".stderr.txt").relative_to(ART))
    if extra:
        c.update(extra)
    return c


def ready(role, container, timeout=20):
    deadline = time.monotonic() + min(timeout, remaining())
    while time.monotonic() < deadline:
        rec = docker("exec", container, "python3", "-c", f"import json;print(json.dumps(json.load(open('/state/{role}-ready.json'))))", label=f"ready-{role}")
        if rec.get("exit_code") == 0:
            try:
                data = json.loads(rec["stdout"])
                if data.get("run_id") == RUN_ID and data.get("role") == role:
                    return data
            except Exception:
                pass
        time.sleep(min(0.25, max(0, deadline - time.monotonic())))
    raise RuntimeError(f"{role} listener readiness timed out")


def set_mode(mode="normal", case="", **extra):
    p = ART / "state" / "adapter-mode.json"
    data = {"mode": mode, "case": case, **extra}
    tmp = p.with_suffix(".tmp")
    tmp.write_text(json.dumps(data))
    os.replace(tmp, p)


def mark_all(case, command, rec, before, after, expected, **extra):
    deltas = {r: after.get(r, 0) - before.get(r, 0) for r in ("gateway", "provider", "proxy", "host", "adapter")}
    base = append_case_files(case, rec, {"ledger_start": before, "ledger_end": after, "ledger_delta": deltas, **extra})
    ok, reason = expected(rec, deltas, extra) if callable(expected) else (bool(expected), "expectation failed")
    base["status"] = "pass" if ok else "blocked"
    base["reason"] = "" if ok else reason
    manifest["cases"][case] = base
    write_manifest()
    return ok


def helper(command, label, timeout=8):
    return docker("run", "--rm", "--network", NAMES["external"], "--entrypoint", "sh", IMAGE, "-c", command, timeout=timeout, label=label)


def curl(url, label, extra=""):
    return helper(f"curl -sS --noproxy '*' --connect-timeout 1 --max-time 3 -o /tmp/e0body -w 'HTTP=%{{http_code}} EXIT=%{{exitcode}}\\n' {extra} '{url}'", label, 6)


def controls(tag):
    # Same direct sink, reachable from X by both DNS and literal address. Every
    # request is tagged and controls are bracketed outside negative deltas.
    ip = manifest["runtime"]["direct_ip"]
    for role, port, alias in (("gateway", 8080, "mock-gateway"), ("provider", 8081, "mock-provider"), ("proxy", 8082, "mock-proxy")):
        for target_kind, target in (("dns", alias), ("ip", ip)):
            before = counts()
            label = f"control-{tag}-{role}-{target_kind}"
            if role == "proxy":
                cmd = f"curl -sS --connect-timeout 1 --max-time 3 -o /tmp/e0body -w 'HTTP=%{{http_code}} EXIT=%{{exitcode}}\\n' --proxy http://{target}:8082 --noproxy '' -H 'X-E0-Case: control:{tag}:{role}:{target_kind}' http://probe.invalid/control"
            else:
                cmd = f"curl -sS --noproxy '*' --connect-timeout 1 --max-time 3 -o /tmp/e0body -w 'HTTP=%{{http_code}} EXIT=%{{exitcode}}\\n' -H 'X-E0-Case: control:{tag}:{role}:{target_kind}' 'http://{target}:{port}/control/{tag}'"
            rec = helper(cmd, label, 6)
            after = counts()
            events = ledger_events()[role]
            matching = [e for e in events if e.get("event") == "http" and e.get("case") == f"control:{tag}:{role}:{target_kind}"]
            expected_code = re.search(r"HTTP=(\d+)", rec.get("stdout", ""))
            if rec.get("exit_code") != 0 or not expected_code or expected_code.group(1) != "200" or len(matching) != 1 or after[role] <= before[role]:
                raise RuntimeError(f"positive control failed for {role}/{target_kind}: {rec.get('stdout')} {rec.get('stderr')} matches={len(matching)}")
    # The sink must accept CONNECT without relaying it.
    before = counts()
    cmd = "curl -vk --connect-timeout 1 --max-time 3 --proxy http://mock-proxy:8082 --noproxy '' --proxy-header 'X-E0-Case: control:" + tag + ":proxy:connect' https://probe.invalid/ 2>&1"
    rec = helper(cmd, f"control-{tag}-proxy-connect", 6)
    after = counts()
    connect = [e for e in ledger_events()["proxy"] if e.get("event") == "http" and e.get("method") == "CONNECT" and e.get("case") == f"control:{tag}:proxy:connect"]
    if not connect or after["proxy"] <= before["proxy"]:
        raise RuntimeError(f"CONNECT positive control missing: {rec.get('stdout')} {rec.get('stderr')}")
    return True


def attempt(name, argv, timeout=8, expected_delta=None, expected_status=None, extra=None):
    before = counts()
    rec = run(argv, timeout=timeout, label=f"case-{name}")
    after = counts()
    ext_delta = sum(after[r] - before[r] for r in ("gateway", "provider", "proxy", "host"))
    ok = (rec.get("exit_code") not in (None, 0)) and ext_delta == 0 if expected_delta is None else expected_delta(rec, after, before)
    addition = dict(extra or {})
    if expected_status:
        addition["http_000_required"] = "HTTP=000" in (rec.get("stdout", "") + rec.get("stderr", ""))
        ok = ok and addition["http_000_required"]
    c = append_case_files(name, rec, {"ledger_start": before, "ledger_end": after, "ledger_delta": {r: after[r] - before[r] for r in after}, **addition})
    c["status"] = "pass" if ok else "blocked"
    c["reason"] = "" if ok else f"attempt did not demonstrate denial with zero sink delta (rc={rec.get('exit_code')}, external_delta={ext_delta})"
    manifest["cases"][name] = c
    write_manifest()
    return c


def check_no_bypass_command(name, cmd):
    # Public attempts are bracketed by controls on the same live sink.
    controls(name + "-before")
    rec = run(["docker", "exec", NAMES["client"], "sh", "-c", cmd], timeout=8, label=f"case-{name}")
    before_after_attempt = counts()
    controls(name + "-after")
    final = counts()
    # controls are accounted for by tagged records; negative window is derived
    # from the snap taken immediately after the command and pre-control snapshot.
    start = manifest.get("_negative_start", counts())
    # _negative_start is assigned after pre-controls by caller through a marker.
    return rec, start, before_after_attempt, final


def negative(name, cmd, require_http000=True, extra=None):
    controls(name + "-before")
    before = counts()
    tagged_cmd = cmd.replace("curl ", f"curl -H 'X-E0-Case: {name}' ", 1)
    rec = run(["docker", "exec", NAMES["client"], "sh", "-c", tagged_cmd], timeout=8, label=f"case-{name}")
    after = counts()
    controls(name + "-after")
    ext = sum(after[r] - before[r] for r in ("gateway", "provider", "proxy", "host"))
    text = rec.get("stdout", "") + rec.get("stderr", "")
    success = rec.get("exit_code") not in (None, 0) and ext == 0
    if require_http000:
        success = success and "HTTP=000" in text
    item = append_case_files(name, rec, {"ledger_start": before, "ledger_end": after, "ledger_delta": {r: after[r] - before[r] for r in after}, "intended_attempt": cmd, "http_000_required": "HTTP=000" in text if require_http000 else None, **(extra or {})})
    item["status"] = "pass" if success else "blocked"
    item["reason"] = "" if success else f"no demonstrated denied attempt or external listener delta={ext}; output={text[-600:]}"
    manifest["cases"][name] = item
    write_manifest()
    return item


def opencode(name, args, mode="normal", timeout=45, env=None):
    set_mode(mode, name, **({} if mode not in ("redirect307", "redirect308") else {"location": f"http://{manifest['runtime']['direct_ip']}:8081/v1/chat/completions"}))
    before = counts()
    command = ["docker", "exec"]
    if env:
        for k, v in env.items():
            command += ["-e", f"{k}={v}"]
    command += ["--workdir", "/workspace/output/e0", NAMES["client"], "opencode", "--print-logs", "--model", args[0], "run", args[1]]
    rec = run(command, timeout=timeout, label=f"case-{name}")
    after = counts()
    return rec, before, after


def case_opencode(name, args, mode="normal", expect_content=None, timeout=45, env=None, zero_external=True, minimum_adapter=1, **extra):
    rec, before, after = opencode(name, args, mode, timeout, env)
    delta = {r: after[r] - before[r] for r in after}
    text = rec.get("stdout", "") + rec.get("stderr", "")
    good = rec.get("exit_code") == 0
    if expect_content:
        good = good and expect_content in rec.get("stdout", "")
    good = good and delta["adapter"] >= minimum_adapter
    if zero_external:
        good = good and sum(delta[r] for r in ("gateway", "provider", "proxy", "host")) == 0
    item = append_case_files(name, rec, {"ledger_start": before, "ledger_end": after, "ledger_delta": delta, "task_content": expect_content if expect_content and expect_content in rec.get("stdout", "") else None, "adapter_requests": delta["adapter"], **extra})
    item["status"] = "pass" if good else "blocked"
    item["reason"] = "" if good else f"unexpected OpenCode route/result: rc={rec.get('exit_code')} adapter={delta['adapter']} external={sum(delta[r] for r in ('gateway','provider','proxy','host'))}"
    manifest["cases"][name] = item
    write_manifest()
    return item


def snapshot_container(label):
    for name in (NAMES["client"], NAMES["adapter"], NAMES["direct"]):
        data = json_cmd(["docker", "inspect", name], f"inspect-{label}-{name}")
        save_snapshot(f"{label}-{name}-inspect", data)
    for network in (NAMES["private"], NAMES["external"]):
        data = json_cmd(["docker", "network", "inspect", network], f"network-{label}-{network}")
        save_snapshot(f"{label}-{network}-network", data)


def docker_file(container, path):
    rec = run(["docker", "exec", container, "sh", "-c", f"cat '{path}'"], timeout=10)
    if rec.get("exit_code") != 0:
        return ""
    return rec["stdout"]


def main():
    global INITIAL_EXIT
    ART.mkdir(parents=True, exist_ok=False)
    for sub in ("logs", "snapshots", "ledgers", "state"):
        p = ART / sub
        p.mkdir(parents=True, exist_ok=True)
        p.chmod(0o777)
    manifest["artifact_dir"] = str(ART)
    manifest["runtime"] = {}
    write_manifest()
    owned = []
    try:
        info = json_cmd(["docker", "image", "inspect", IMAGE], "image-inspect")
        image = info[0]
        actual = {"id": image["Id"], "platform": image["Os"] + "/" + image["Architecture"]}
        version = run(["docker", "run", "--rm", "--entrypoint", "opencode", IMAGE, "--version"], timeout=20, label="image-version")
        if actual != {"id": IMAGE, "platform": "linux/arm64"} or version.get("stdout", "").strip() != "1.18.4":
            set_case("image", "blocked", version.get("argv"), "pinned image identity/version mismatch", exit_code=version.get("exit_code"), actual=actual)
            block(f"pinned image mismatch: {actual}, version={version.get('stdout')}")
        IMAGE_INFO["image_declared_volumes"] = image.get("Config", {}).get("Volumes", {})
        set_case("image", "pass", version["argv"], exit_code=version["exit_code"], actual=actual, binary_sha256_expected=EXPECTED_BIN)

        docker("network", "create", "--internal", "-o", "com.docker.network.bridge.gateway_mode_ipv4=isolated", NAMES["private"], check=True); owned.append(("network", NAMES["private"]))
        docker("network", "create", NAMES["external"], check=True); owned.append(("network", NAMES["external"]))
        docker("volume", "create", NAMES["config"], check=True); owned.append(("volume", NAMES["config"]))
        docker("volume", "create", NAMES["data"], check=True); owned.append(("volume", NAMES["data"]))
        fixture_mount = str(FIXTURE)
        state_mount = str(ART / "state")
        ledger_mount = str(ART / "ledgers")
        docker("run", "-d", "--name", NAMES["direct"], "--network", NAMES["external"], "--network-alias", "mock-gateway", "--network-alias", "mock-provider", "--network-alias", "mock-proxy", "-p", "0.0.0.0::8083", "--cap-drop", "ALL", "--security-opt", "no-new-privileges", "-v", fixture_mount + ":/probe:ro", "-v", state_mount + ":/state", "-v", ledger_mount + ":/ledger", "--entrypoint", "python3", IMAGE, "/probe/server_v2.py", "--role", "direct", "--run-id", RUN_ID, check=True); owned.append(("container", NAMES["direct"]))
        docker("run", "-d", "--name", NAMES["adapter"], "--network", NAMES["private"], "--network-alias", "mock-adapter", "--cap-drop", "ALL", "--security-opt", "no-new-privileges", "--sysctl", "net.ipv4.ip_forward=0", "-v", fixture_mount + ":/probe:ro", "-v", state_mount + ":/state", "-v", ledger_mount + ":/ledger", "--entrypoint", "python3", IMAGE, "/probe/server_v2.py", "--role", "adapter", "--run-id", RUN_ID, check=True); owned.append(("container", NAMES["adapter"]))
        docker("network", "connect", "--alias", "mock-adapter-x", NAMES["external"], NAMES["adapter"], check=True)
        ready("direct", NAMES["direct"]); ready("adapter", NAMES["adapter"])
        direct_ip_rec = run(["docker", "inspect", NAMES["direct"], "--format", "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}"], label="direct-ip")
        if direct_ip_rec.get("exit_code") != 0 or not direct_ip_rec.get("stdout", "").strip():
            block("could not inspect the direct sink IP")
        direct_ip = direct_ip_rec["stdout"].strip()
        adapter_net = json_cmd(["docker", "inspect", NAMES["adapter"], "--format", "{{json .NetworkSettings.Networks}}"], "adapter-ips")
        adapter_ip_x = adapter_net[NAMES["external"]]["IPAddress"]
        mapping = docker("port", NAMES["direct"], "8083/tcp", label="host-port")
        match = re.search(r":(\d+)\s*$", mapping.get("stdout", ""))
        if not match:
            block("host published listener did not obtain an owned host port")
        host_port = int(match.group(1))
        manifest["runtime"].update(direct_ip=direct_ip, adapter_ip_external=adapter_ip_x, published_host_port=host_port)
        write_manifest()
        docker("run", "-d", "--name", NAMES["client"], "--network", NAMES["private"], "--add-host", f"mock-proxy:{direct_ip}", "--add-host", f"mock-hostile-provider:{direct_ip}", "--add-host", "probe-host:host-gateway", "--cap-drop", "ALL", "--security-opt", "no-new-privileges", "-e", "HOME=/home/craft", "-e", "XDG_CONFIG_HOME=/home/craft/.config", "-e", "XDG_DATA_HOME=/home/craft/.local/share", "--mount", f"type=volume,source={NAMES['config']},target=/home/craft/.config/opencode", "--mount", f"type=volume,source={NAMES['data']},target=/home/craft/.local/share/opencode", "-v", fixture_mount + ":/probe:ro", "-v", state_mount + ":/state", "--entrypoint", "sleep", IMAGE, "infinity", check=True); owned.append(("container", NAMES["client"]))
        # The fixed hosts mappings make custom-URL/proxy-name attempts resolve to
        # the controlled sink; ordinary provider/gateway names remain true DNS tests.
        docker("exec", NAMES["client"], "mkdir", "-p", "/workspace/output/e0", check=True)
        docker("exec", NAMES["client"], "cp", "/probe/opencode.json", "/workspace/output/e0/opencode.json", check=True)
        docker("exec", NAMES["client"], "sh", "-c", "printf 'e0-restart-marker-%s\\n' '" + RUN_ID + "' > /home/craft/.local/share/opencode/e0-marker", check=True)
        # Inspect exact runtime facts before any negative evidence is considered.
        facts_cmd = "id; printf 'HOME=%s\\nXDG_CONFIG_HOME=%s\\nXDG_DATA_HOME=%s\\n' \"$HOME\" \"$XDG_CONFIG_HOME\" \"$XDG_DATA_HOME\"; cat /etc/resolv.conf; cat /etc/hosts; cat /proc/net/route; cat /proc/net/ipv6_route; cat /proc/self/mountinfo; grep '^Cap' /proc/self/status; opencode --version; sha256sum $(command -v opencode); find /home/craft/.config/opencode -type f -maxdepth 4 -print -exec sha256sum {} \\; 2>/dev/null; find /home/craft/.local/share/opencode -type f -maxdepth 4 -print -exec sha256sum {} \\; 2>/dev/null"
        facts = run(["docker", "exec", NAMES["client"], "sh", "-c", facts_cmd], timeout=15, label="client-runtime-facts")
        save_snapshot("client-runtime-facts", facts)
        manifest["runtime"].update(client_runtime_exit=facts.get("exit_code"), client_runtime_sha256=hashlib.sha256(facts.get("stdout", "").encode()).hexdigest())
        snapshot_container("pre")
        net = json_cmd(["docker", "network", "inspect", NAMES["private"]], "private-network-facts")[0]
        clients = sorted(net.get("Containers", {}).values(), key=lambda x: x.get("Name", ""))
        cinspect = json_cmd(["docker", "inspect", NAMES["client"]], "client-inspect-facts")[0]
        mounts = cinspect.get("Mounts", [])
        attached = cinspect.get("NetworkSettings", {}).get("Networks", {})
        routes = run(["docker", "exec", NAMES["client"], "cat", "/proc/net/ipv6_route"], label="ipv6-routes")
        nonloop = [line for line in routes.get("stdout", "").splitlines()[1:] if line.strip() and line.split()[-1] != "lo"]
        adapter_inspect = json_cmd(["docker", "inspect", NAMES["adapter"]], "adapter-inspect-facts")[0]
        direct_inspect = json_cmd(["docker", "inspect", NAMES["direct"]], "direct-inspect-facts")[0]
        adapter_attached = adapter_inspect.get("NetworkSettings", {}).get("Networks", {})
        direct_attached = direct_inspect.get("NetworkSettings", {}).get("Networks", {})
        adapter_ip_forward_disabled = adapter_inspect.get("HostConfig", {}).get("Sysctls", {}).get("net.ipv4.ip_forward") == "0"
        private_internal = net.get("Internal") is True
        isolated_mode = net.get("Options", {}).get("com.docker.network.bridge.gateway_mode_ipv4") == "isolated"
        binary_hash_match = EXPECTED_BIN in facts.get("stdout", "")
        client_ledger_mount = any(m.get("Destination") == "/ledger" for m in mounts)
        ledger_write = docker("exec", NAMES["client"], "sh", "-c", "if touch /ledger/e0-client-forbidden-write 2>/dev/null; then rm -f /ledger/e0-client-forbidden-write; echo writable; exit 1; else echo denied; fi", label="client-ledger-write-denied")
        ledger_write_denied = ledger_write.get("exit_code") == 0 and ledger_write.get("stdout", "").strip() == "denied"
        topology_ok = (not client_ledger_mount and ledger_write_denied and len(attached) == 1 and sorted(x.get("Name") for x in clients) == sorted([NAMES["client"], NAMES["adapter"]]) and net.get("EnableIPv6") is False and not nonloop and attached[NAMES["private"]].get("Gateway", "") == "" and private_internal and isolated_mode and set(adapter_attached) == {NAMES["private"],NAMES["external"]} and set(direct_attached) == {NAMES["external"]} and adapter_ip_forward_disabled and binary_hash_match and not cinspect.get("HostConfig",{}).get("Privileged",True) and not cinspect.get("HostConfig",{}).get("CapAdd") and cinspect.get("HostConfig",{}).get("PortBindings") in (None,{}))
        set_case("topology", "pass" if topology_ok else "blocked", ["docker inspect/network inspect + route inventory"], "" if topology_ok else "client/private membership, ledger isolation, privileges, binary hash, or IPv6 policy mismatch", client_attachment_count=len(attached), private_members=sorted(x.get("Name") for x in clients), ipv6_disabled=net.get("EnableIPv6") is False, non_loopback_ipv6_routes=len(nonloop), private_internal=private_internal, isolated_gateway_mode=isolated_mode, adapter_ip_forward_disabled=adapter_ip_forward_disabled, binary_sha256_match=binary_hash_match, client_mounts_ledger=client_ledger_mount, client_ledger_write_denied=ledger_write_denied, client_ledger_write_stdout=ledger_write.get("stdout", ""), client_networks=sorted(attached), adapter_networks=sorted(adapter_attached), direct_networks=sorted(direct_attached), client_privileged=cinspect.get("HostConfig",{}).get("Privileged"), client_cap_add=cinspect.get("HostConfig",{}).get("CapAdd"), client_port_bindings=cinspect.get("HostConfig",{}).get("PortBindings"), same_container_id=cinspect.get("Id"), mount_sources={m["Destination"]: m.get("Name", m.get("Source")) for m in mounts}, runtime_facts_file="snapshots/client-runtime-facts.json")
        if not topology_ok:
            block("isolated topology facts failed; not weakening policy")
        controls("initial")
        # DNS and literal-address baseline controls against same D.
        baseline_ok = True
        baseline_start = counts()
        for role, alias, port in (("gateway", "mock-gateway", 8080), ("provider", "mock-provider", 8081)):
            for target_kind, host in (("dns", alias), ("ip", direct_ip)):
                label = f"baseline-{role}-{target_kind}"
                before = counts()
                rec = helper(f"curl -sS --noproxy '*' --connect-timeout 1 --max-time 3 -o /tmp/e0body -w 'HTTP=%{{http_code}} EXIT=%{{exitcode}}\\n' -H 'X-E0-Case: baseline:{role}:{target_kind}' 'http://{host}:{port}/baseline'", label, 6)
                after = counts()
                match_events = [e for e in ledger_events()[role] if e.get("event") == "http" and e.get("case") == f"baseline:{role}:{target_kind}"]
                ok = rec.get("exit_code") == 0 and "HTTP=200" in rec.get("stdout", "") and len(match_events) == 1 and after[role] > before[role]
                set_case(label, "pass" if ok else "blocked", rec["argv"], "" if ok else f"same-D positive baseline failed: {rec.get('stdout')} {rec.get('stderr')}", exit_code=rec.get("exit_code"), ledger_start=before, ledger_end=after, stdout_file=rec["stdout_file"], stderr_file=rec["stderr_file"])
                baseline_ok &= ok
        baseline_end = counts()
        set_case("baseline_dns", "pass" if baseline_ok else "blocked", ["same pinned image on external bridge; gateway/provider DNS controls"], "" if baseline_ok else "one or more DNS baseline controls failed", exit_code=0 if baseline_ok else 1, ledger_start=baseline_start, ledger_end=baseline_end, successful_controls=["gateway DNS HTTP 200", "provider DNS HTTP 200"])
        set_case("baseline_ip", "pass" if baseline_ok else "blocked", ["same pinned image on external bridge; gateway/provider literal-IP controls"], "" if baseline_ok else "one or more literal-IP baseline controls failed", exit_code=0 if baseline_ok else 1, ledger_start=baseline_start, ledger_end=baseline_end, successful_controls=["gateway IP HTTP 200", "provider IP HTTP 200"])
        # Proxy baseline is the required same-D open proxy-style sink, including
        # absolute form and CONNECT, followed by counted no-forward semantics.
        bstart = counts()
        pr = helper("curl -sS --connect-timeout 1 --max-time 3 -o /tmp/e0body -w 'HTTP=%{http_code} EXIT=%{exitcode}\\n' --proxy http://mock-proxy:8082 --noproxy '' -H 'X-E0-Case: baseline:proxy:absolute' http://probe.invalid/baseline", "baseline-proxy-absolute", 6)
        bend = counts()
        p_event = [e for e in ledger_events()["proxy"] if e.get("event") == "http" and e.get("case") == "baseline:proxy:absolute"]
        proxy_ok = pr.get("exit_code") == 0 and "HTTP=200" in pr.get("stdout", "") and len(p_event) == 1 and bend["proxy"] > bstart["proxy"]
        set_case("baseline_adapter_proxy", "pass" if proxy_ok else "blocked", pr["argv"], "" if proxy_ok else "proxy baseline failed", exit_code=pr.get("exit_code"), ledger_start=bstart, ledger_end=bend, stdout_file=pr["stdout_file"], stderr_file=pr["stderr_file"])
        baseline_ok &= proxy_ok
        if not baseline_ok:
            block("same-sink direct/proxy positive baseline failed")
        # A fresh host-facing baseline is mandatory. Resolve Docker Desktop's
        # special host name inside an external-bridge helper, then use the owned
        # published port. This is never a pre-existing host service.
        hostresolve = helper("getent hosts host.docker.internal || true", "host-address-resolution", 5)
        save_snapshot("host-address-resolution", hostresolve)
        host_ips = re.findall(r"\b(?:\d{1,3}\.){3}\d{1,3}\b", hostresolve.get("stdout", ""))
        host_ip = host_ips[0] if host_ips else ""
        manifest["runtime"].update(host_ip=host_ip, host_address_resolution=hostresolve.get("stdout", ""))
        if not host_ip:
            for case in ("host_route_pre", "host_route_post"):
                set_case(case, "blocked", hostresolve.get("argv"), "external baseline cannot resolve Docker host address; host positive control unavailable", exit_code=hostresolve.get("exit_code"), positive_control_http=None, bypass_delta=None)
            block("host-facing published-port positive control unavailable")
        hbefore = counts()
        hc = helper(f"curl -sS --connect-timeout 1 --max-time 3 -o /tmp/e0body -w 'HTTP=%{{http_code}}\\n' -H 'X-E0-Case: host-control:pre' http://{host_ip}:{host_port}/host-control", "host-positive-control", 6)
        hafter = counts()
        host_events = [e for e in ledger_events()["host"] if e.get("event") == "http" and e.get("case") == "host-control:pre"]
        host_http = int(re.search(r"HTTP=(\d+)", hc.get("stdout", "HTTP=000")).group(1))
        host_ok = hc.get("exit_code") == 0 and host_http == 200 and len(host_events) == 1 and hafter["host"] > hbefore["host"]
        manifest["runtime"]["host_positive_control_http"] = host_http
        if not host_ok:
            for case in ("host_route_pre", "host_route_post"):
                set_case(case, "blocked", hc.get("argv"), "disposable published host listener did not provide reachable baseline", exit_code=hc.get("exit_code"), positive_control_http=host_http, bypass_delta=None)
            block("published host positive control failed")

        def normal(name, suffix="pre"):
            return case_opencode(name, ["mock/mock-model", "E0 normal: reply with one word."], expect_content="probe-ok", timeout=45, completion_id=name, invocation="fresh opencode run")

        # Clear counter ranges are established by prior positive controls.
        normal_case = normal("normal_route")
        adapter_events = [e for e in ledger_events()["adapter"] if e.get("event") == "http" and e.get("case") == "normal_route" and e.get("method") == "POST"]
        sess = {e.get("session_id") for e in adapter_events}
        purposes = {e.get("purpose") for e in adapter_events}
        exact_normal = len(adapter_events) == 2 and purposes == {"title", "task"} and len(sess) == 1 and None not in sess and all(e.get("path") == "/v1/chat/completions" and e.get("model") == "mock-model" and e.get("stream") is True for e in adapter_events)
        normal_case.update(adapter_post_count=len(adapter_events), session_ids=sorted(sess), purposes=sorted(purposes), exact_route=exact_normal)
        if not exact_normal:
            normal_case["status"] = "blocked"; normal_case["reason"] = "fresh model turn did not produce exactly one title plus one task streaming POST in one session"
        write_manifest()

        # Denial group before restart: every attempted target uses the actual
        # inspected direct IP; forced IP controls distinguish DNS from policy.
        ip = direct_ip
        for name, cmd in [
            ("direct_ip_pre", f"curl -sS --noproxy '*' --connect-timeout 1 --max-time 3 -o /tmp/b -w 'HTTP=%{{http_code}} EXIT=%{{exitcode}}\\n' http://{ip}:8080/direct"),
            ("dns_pre", "curl -sS --noproxy '*' --connect-timeout 1 --max-time 3 -o /tmp/b -w 'HTTP=%{http_code} EXIT=%{exitcode}\\n' http://mock-gateway:8080/dns"),
            ("forced_ip_pre", f"curl -sS --noproxy '*' --connect-timeout 1 --max-time 3 --resolve mock-provider:8081:{ip} -o /tmp/b -w 'HTTP=%{{http_code}} EXIT=%{{exitcode}}\\n' http://mock-provider:8081/forced"),
            ("custom_url_ip_pre", f"curl -sS --noproxy '*' --connect-timeout 1 --max-time 3 -o /tmp/b -w 'HTTP=%{{http_code}} EXIT=%{{exitcode}}\\n' http://{ip}:8081/v1/models"),
            ("custom_url_dns_pre", "curl -sS --noproxy '*' --connect-timeout 1 --max-time 3 -o /tmp/b -w 'HTTP=%{http_code} EXIT=%{exitcode}\\n' http://mock-provider:8081/v1/models"),
            ("proxy_ip_pre", f"env HTTP_PROXY=http://{ip}:8082 HTTPS_PROXY=http://{ip}:8082 ALL_PROXY=http://{ip}:8082 http_proxy=http://{ip}:8082 https_proxy=http://{ip}:8082 all_proxy=http://{ip}:8082 NO_PROXY= no_proxy= curl -sS --connect-timeout 1 --max-time 3 -o /tmp/b -w 'HTTP=%{{http_code}} EXIT=%{{exitcode}}\\n' --proxy http://{ip}:8082 --noproxy '' http://probe.invalid/proxy-ip"),
            ("proxy_dns_pre", "env HTTP_PROXY=http://mock-proxy:8082 HTTPS_PROXY=http://mock-proxy:8082 ALL_PROXY=http://mock-proxy:8082 http_proxy=http://mock-proxy:8082 https_proxy=http://mock-proxy:8082 all_proxy=http://mock-proxy:8082 NO_PROXY= no_proxy= curl -sS --connect-timeout 1 --max-time 3 -o /tmp/b -w 'HTTP=%{http_code} EXIT=%{exitcode}\\n' --proxy http://mock-proxy:8082 --noproxy '' http://probe.invalid/proxy-dns"),
        ]:
            negative(name, cmd, True)
        # OpenCode inherits hostile proxy env but still must route only to A;
        # an explicit curl attempt above demonstrates actual proxy transport.
        case_opencode("proxy_opencode_pre", ["mock/mock-model", "E0 proxy env check"], expect_content="probe-ok", env={"HTTP_PROXY": f"http://{ip}:8082", "HTTPS_PROXY": f"http://{ip}:8082", "ALL_PROXY": f"http://{ip}:8082", "http_proxy": f"http://{ip}:8082", "https_proxy": f"http://{ip}:8082", "all_proxy": f"http://{ip}:8082", "NO_PROXY": "", "no_proxy": ""}, proxy_env_observation="OpenCode route measured separately from explicit curl")

        # Redirects exercise POST-follow behavior. Curl baseline proves the same
        # 307/308 is followed to D; C then records whether the SDK follows it.
        for mode, case_name, code in (("redirect307", "redirect_307_pre", 307), ("redirect308", "redirect_308_pre", 308)):
            controls(case_name + "-before")
            set_mode(mode, "redirect-control", location=f"http://{direct_ip}:8081/v1/chat/completions")
            control_start = counts()
            control = helper("curl -sS -L --max-redirs 2 --connect-timeout 1 --max-time 4 -o /tmp/e0body -w 'HTTP=%{http_code} EXIT=%{exitcode}\\n' -H 'Content-Type: application/json' -H 'X-E0-Case: redirect-control' -d '{\"model\":\"mock-model\",\"stream\":false,\"messages\":[{\"role\":\"user\",\"content\":\"redirect control\"}]}' http://mock-adapter-x:8080/v1/chat/completions", f"{case_name}-curl-follow-control", 6)
            d_after = counts()
            followed = [e for e in ledger_events()["provider"] if e.get("event") in ("tcp_accept", "http") and e.get("seq",0)>control_start["provider"]]
            controls(case_name + "-after-control")
            set_mode(mode, case_name, location=f"http://{direct_ip}:8081/v1/chat/completions")
            before = counts()
            op = run(["docker", "exec", "--workdir", "/workspace/output/e0", NAMES["client"], "opencode", "--print-logs", "--model", "mock/mock-model", "run", f"E0 {code} redirect probe"], timeout=45, label=f"case-{case_name}")
            after = counts()
            redirect_events = [e for e in ledger_events()["adapter"] if e.get("event") == "http" and e.get("case") == case_name and e.get("purpose") == "task"]
            delta = {r: after[r] - before[r] for r in after}
            ok = bool(followed) and len(redirect_events) >= 1 and delta["provider"] == 0 and op.get("exit_code") is not None
            item = append_case_files(case_name, op, {"ledger_start": before, "ledger_end": after, "ledger_delta": delta, "curl_follow_control_exit": control.get("exit_code"), "curl_follow_control_http": re.findall(r"HTTP=(\d+)", control.get("stdout", "")), "curl_follow_provider_requests": d_after["provider"]-control_start["provider"], "redirect_status": code, "adapter_original_posts": len(redirect_events), "sdk_exit_code": op.get("exit_code")})
            item["status"] = "pass" if ok else "blocked"; item["reason"] = "" if ok else "POST redirect positive follow control or adapter observation missing"
            manifest["cases"][case_name] = item; write_manifest()
            controls(case_name + "-after")

        # Adapter endpoint is not a generic HTTP/SOCKS relay.
        controls("adapter-proxy-pre-before")
        set_mode("normal", "adapter_proxy_pre")
        before = counts()
        pr = run(["docker", "exec", NAMES["client"], "sh", "-c", "curl -H 'X-E0-Case: adapter_proxy_pre' -sS --connect-timeout 1 --max-time 3 -o /tmp/b -w 'ABS_HTTP=%{http_code}\\n' --proxy http://mock-adapter:8080 --noproxy '' http://probe.invalid/absolute; curl -H 'X-E0-Case: adapter_proxy_pre' -skv --connect-timeout 1 --max-time 3 --proxy http://mock-adapter:8080 --noproxy '' https://probe.invalid/ 2>&1"], timeout=8, label="case-adapter-proxy-pre")
        after = counts(); controls("adapter-proxy-pre-after")
        adproxy = [e for e in ledger_events()["adapter"] if e.get("event") == "http" and e.get("case") == "normal_route"]
        # The adapter should reject proxy syntax with 403; no direct sink is reachable.
        ptexts = pr.get("stdout", "") + pr.get("stderr", "")
        pzero = sum(after[r]-before[r] for r in ("gateway","provider","proxy","host")) == 0
        item = append_case_files("adapter_proxy_pre", pr, {"ledger_start": before, "ledger_end": after, "ledger_delta": {r:after[r]-before[r] for r in after}, "expected_rejection": "403/connection failure"})
        item["status"] = "pass" if pzero and pr.get("exit_code") is not None and "403" in ptexts else "blocked"; item["reason"] = "" if item["status"] == "pass" else "adapter proxy attempt did not show rejection or reached an external listener"
        manifest["cases"]["adapter_proxy_pre"] = item; write_manifest()

        # Host/published-address targets on the isolated client. Baseline D is
        # demonstrably reachable via the host published port from X above.
        route = run(["docker", "exec", NAMES["client"], "cat", "/proc/net/route"], label="client-routes-v4")
        gateways = []
        for line in route.get("stdout", "").splitlines()[1:]:
            cols = line.split()
            if len(cols) > 2 and cols[1] == "00000000":
                gateways.append(socket.inet_ntoa(bytes.fromhex(cols[2])[::-1]))
        candidates = ["host.docker.internal", "gateway.docker.internal", "probe-host"] + gateways + [host_ip]
        controls("host-route-pre-before")
        hp0 = counts()
        hp = helper(f"curl -sS --connect-timeout 1 --max-time 3 -o /tmp/e0body -w 'HTTP=%{{http_code}}\\n' -H 'X-E0-Case: host-control:pre-row' http://{host_ip}:{host_port}/host-control-pre-row", "host-positive-control-pre-row", 6)
        hp_http = int(re.search(r"HTTP=(\d+)",hp.get("stdout","HTTP=000")).group(1))
        hp1 = counts()
        hp_events = [e for e in ledger_events()["host"] if e.get("event")=="http" and e.get("case")=="host-control:pre-row"]
        if hp.get("exit_code") != 0 or hp_http != 200 or not hp_events or hp1["host"] <= hp0["host"]:
            block("host route positive control failed immediately before host denial matrix")
        host_attempts = []
        hroute_start = counts()
        for target in dict.fromkeys(candidates):
            cmd = f"curl -sS -H 'X-E0-Case: host_route_pre' --noproxy '*' --connect-timeout 1 --max-time 3 -o /tmp/b -w 'HTTP=%{{http_code}} EXIT=%{{exitcode}}\\n' http://{target}:{host_port}/host-bypass"
            rec = run(["docker", "exec", NAMES["client"], "sh", "-c", cmd], timeout=5, label=f"host-attempt-{target.replace('.', '_')}")
            host_attempts.append({"target":target,"exit_code":rec.get("exit_code"),"stdout":rec.get("stdout"),"stderr":rec.get("stderr")})
        hafterclient = counts()
        hostdelta = hafterclient["host"] - hroute_start["host"]
        controls("host-route-pre-after")
        hp_post0 = counts()
        hp_post = helper(f"curl -sS --connect-timeout 1 --max-time 3 -o /tmp/e0body -w 'HTTP=%{{http_code}}\\n' -H 'X-E0-Case: host-control:pre-row-after' http://{host_ip}:{host_port}/host-control-pre-row-after", "host-positive-control-pre-row-after", 6)
        hp_post_http = int(re.search(r"HTTP=(\d+)",hp_post.get("stdout","HTTP=000")).group(1))
        hp_post1 = counts()
        hp_post_events = [e for e in ledger_events()["host"] if e.get("event")=="http" and e.get("case")=="host-control:pre-row-after"]
        host_control_span_ok = hp_post.get("exit_code")==0 and hp_post_http==200 and bool(hp_post_events) and hp_post1["host"]>hp_post0["host"]
        hr_ok = all(a["exit_code"] not in (None,0) and "HTTP=000" in (a["stdout"] or "") for a in host_attempts) and hostdelta == 0
        rec = {"argv":["docker exec client curl host/published route matrix"],"exit_code":1 if not hr_ok else 0,"stdout":json.dumps(host_attempts,indent=2),"stderr":""}
        item = append_case_files("host_route_pre", rec, {"ledger_start":hroute_start,"ledger_end":hafterclient,"ledger_delta":{r:hafterclient[r]-hroute_start[r] for r in hroute_start},"positive_control_http":hp_http,"positive_control_after_http":hp_post_http,"bypass_delta":hostdelta,"host_attempts":host_attempts})
        item["status"]="pass" if hr_ok and host_control_span_ok else "blocked"; item["reason"]="" if item["status"]=="pass" else "host/published route not denied or positive control returned unexpected response"
        manifest["cases"]["host_route_pre"]=item; write_manifest()

        # Project baseURL forced to direct IP, then controlled host-mapped DNS.
        original_project = (ART / "state" / "project-config-original.json")
        shutil.copyfile(FIXTURE / "opencode.json", original_project)
        def project_config(base):
            config={"$schema":"https://opencode.ai/config.json","provider":{"mock":{"npm":"@ai-sdk/openai-compatible","name":"E0 controlled","options":{"baseURL":base,"apiKey":"probe-only"},"models":{"mock-model":{"name":"mock-model"}}}}}
            p=ART/"state"/"hostile-project.json"; p.write_text(json.dumps(config));
            docker("exec",NAMES["client"],"cp","/probe/opencode.json","/workspace/output/e0/original-config.json",check=True)
            docker("cp",str(p),f"{NAMES['client']}:/workspace/output/e0/opencode.json",check=True)
        for name, base in (("hostile_project_url", f"http://{direct_ip}:8081/v1"),("custom_url_dns_pre", "http://mock-hostile-provider:8081/v1")):
            project_config(base)
            cfg_sha=hashlib.sha256(json.dumps(json.loads((ART/"state"/"hostile-project.json").read_text()),sort_keys=True).encode()).hexdigest()
            set_mode("normal", name)
            in_container_hash=docker("exec",NAMES["client"],"sha256sum","/workspace/output/e0/opencode.json",label=f"{name}-effective-config-hash")
            controls("project-config-"+name+"-before"); before=counts()
            op=run(["docker","exec","--workdir","/workspace/output/e0",NAMES["client"],"opencode","--print-logs","--model","mock/mock-model","run",f"E0 hostile config {name}"],timeout=45,label=f"case-{name}")
            after=counts(); controls("project-config-"+name+"-after")
            ext=sum(after[r]-before[r] for r in ("gateway","provider","proxy","host"))
            sink_attempts=[e for r in ("gateway","provider") for e in ledger_events()[r] if e.get("event")=="tcp_accept" and e.get("time_ns",0)>0]
            text=op.get("stdout","")+op.get("stderr","")
            # Loaded configuration evidence is the exact client-side file hash
            # plus selected mock/mock-model argv and OpenCode config-load logs.
            trace="\n".join(line for line in text.splitlines() if any(key in line for key in ("loading path=", "providerID=", base)))
            loaded=("loading path=/workspace/output/e0/opencode.json" in trace and "providerID=mock" in trace and base in trace and "mock/mock-model" in op.get("argv",[]) and in_container_hash.get("exit_code")==0 and in_container_hash.get("stdout","").split()[0]==sha(ART/"state"/"hostile-project.json"))
            delta={r:after[r]-before[r] for r in after}
            good=op.get("exit_code") not in (None,0) and ext==0 and delta["adapter"]==0 and loaded
            item=append_case_files(name,op,{"ledger_start":before,"ledger_end":after,"ledger_delta":delta,"effective_project_config_sha256":cfg_sha,"effective_config_trace":trace,"selected_base_url":base if loaded else None,"loaded_config_proof":"OpenCode process log load/provider/baseURL trace plus in-container config hash","intended_base_url":base,"no_model_answer_expected":True})
            item["status"]="pass" if good else "blocked";item["reason"]="" if good else f"hostile config not denied or config evidence incomplete (rc={op.get('exit_code')}, external={ext}, adapter={delta['adapter']})"
            manifest["cases"][name]=item;write_manifest()
        # Host-mapped hostile provider is also repeated against raw DNS by name;
        # the mapping proves the request would have used D instead of failing DNS.
        # restore global config baseline and record hashes before restart
        docker("cp",str(FIXTURE / "opencode.json"),f"{NAMES['client']}:/workspace/output/e0/opencode.json",check=True)
        config_mount = "/home/craft/.config/opencode/opencode.json"
        old_global = docker("exec",NAMES["client"],"sh","-c",f"if test -f {config_mount}; then cat {config_mount}; else printf '__ABSENT__'; fi",label="global-config-original")
        (ART/"state"/"global-config-original.txt").write_text(old_global.get("stdout",""))
        hostile_global={"$schema":"https://opencode.ai/config.json","provider":{"evil":{"npm":"@ai-sdk/openai-compatible","name":"hostile global","options":{"baseURL":f"http://{direct_ip}:8081/v1","apiKey":"probe-only"},"models":{"model":{"name":"model"}}}}}
        (ART/"state"/"hostile-global.json").write_text(json.dumps(hostile_global))
        docker("exec",NAMES["client"],"mkdir","-p","/home/craft/.config/opencode",check=True)
        docker("cp",str(ART/"state"/"hostile-global.json"),f"{NAMES['client']}:{config_mount}",check=True)
        set_mode("normal", "hostile_global_url")
        global_config_actual=docker("exec",NAMES["client"],"sha256sum",config_mount,label="hostile-global-effective-config-hash")
        controls("hostile-global-before"); before=counts()
        op=run(["docker","exec","--workdir","/workspace/output/e0",NAMES["client"],"opencode","--print-logs","--model","evil/model","run","E0 hostile global configuration"],timeout=45,label="case-hostile-global-url")
        after=counts();controls("hostile-global-after")
        global_text=op.get("stdout","")+op.get("stderr","")
        global_trace="\n".join(line for line in global_text.splitlines() if any(key in line for key in ("loading path=", "providerID=evil", hostile_global["provider"]["evil"]["options"]["baseURL"])))
        hostile_url=hostile_global["provider"]["evil"]["options"]["baseURL"]
        globalloaded=("loading path="+config_mount in global_trace and "providerID=evil" in global_trace and hostile_url in global_trace and "evil/model" in op.get("argv",[]) and global_config_actual.get("exit_code")==0 and global_config_actual.get("stdout","").split()[0]==sha(ART/"state"/"hostile-global.json"))
        delta={r:after[r]-before[r] for r in after};good=op.get("exit_code") not in (None,0) and sum(delta[r] for r in ("gateway","provider","proxy","host"))==0 and globalloaded
        item=append_case_files("hostile_global_url",op,{"ledger_start":before,"ledger_end":after,"ledger_delta":delta,"selected_model":"evil/model","effective_global_config_sha256":sha(ART/"state"/"hostile-global.json"),"effective_config_trace":global_trace,"selected_base_url":hostile_url if globalloaded else None,"intended_base_url":hostile_url,"loaded_config_proof":"OpenCode process log load/provider/baseURL trace plus in-container XDG config hash"})
        item["status"]="pass" if good else "blocked";item["reason"]="" if good else "hostile global URL not denied or selected configuration missing"
        manifest["cases"]["hostile_global_url"]=item;write_manifest()
        if old_global.get("stdout","")!="__ABSENT__":
            (ART/"state"/"restore-global.json").write_text(old_global["stdout"])
            docker("cp",str(ART/"state"/"restore-global.json"),f"{NAMES['client']}:{config_mount}",check=True)
        else:
            docker("exec",NAMES["client"],"rm","-f",config_mount,check=True)
        # Persisted data/config identity marker and same-container restart.
        pre_id=cinspect["Id"]
        pre_mounts={m["Destination"]:m.get("Name",m.get("Source")) for m in mounts}
        marker_before=docker_file(NAMES["client"],"/home/craft/.local/share/opencode/e0-marker")
        marker_sha=hashlib.sha256(marker_before.encode()).hexdigest()
        global_hash_before=run(["docker","exec",NAMES["client"],"sha256sum","/home/craft/.config/opencode/opencode.json"],timeout=10,label="global-config-hash-pre-restart")
        global_hash_pre=global_hash_before.get("stdout","").split()[0] if global_hash_before.get("exit_code")==0 else None
        docker("restart",NAMES["client"],timeout=20,check=True)
        ready_client=run(["docker","exec",NAMES["client"],"sh","-c","id; echo $HOME; echo $XDG_CONFIG_HOME; echo $XDG_DATA_HOME"],timeout=10,label="post-restart-client-ready")
        post=json_cmd(["docker","inspect",NAMES["client"]],"post-restart-client-inspect")[0]
        post_mounts={m["Destination"]:m.get("Name",m.get("Source")) for m in post.get("Mounts",[])}
        marker_after=docker_file(NAMES["client"],"/home/craft/.local/share/opencode/e0-marker")
        global_hash_after=run(["docker","exec",NAMES["client"],"sha256sum","/home/craft/.config/opencode/opencode.json"],timeout=10,label="global-config-hash-post-restart")
        global_hash_post=global_hash_after.get("stdout","").split()[0] if global_hash_after.get("exit_code")==0 else None
        restart_ok=post.get("Id")==pre_id and post_mounts==pre_mounts and marker_after==marker_before and bool(marker_before) and global_hash_pre is not None and global_hash_pre==global_hash_post
        set_case("restart","pass" if restart_ok else "blocked",["docker restart + inspect same C and mounted XDG volumes"],"" if restart_ok else "container/mount/marker/global config identity changed",same_container=post.get("Id")==pre_id,same_mounts=post_mounts==pre_mounts,marker_persisted=marker_after==marker_before and bool(marker_before),marker_sha256=marker_sha,global_config_hash_pre=global_hash_pre,global_config_hash_post=global_hash_post,global_config_persisted=global_hash_pre is not None and global_hash_pre==global_hash_post,container_id=post.get("Id"),mount_sources=post_mounts,ledger_start=counts(),ledger_end=counts())
        snapshot_container("post-restart")
        # Repeat normal model route.
        normal_post=case_opencode("normal_route_post",["mock/mock-model","E0 post-restart: reply with one word."],expect_content="probe-ok",timeout=45,completion_id="normal_route_post")

        # Repeat direct/DNS/forced-IP/proxy/redirect/adapter/host/IPv6 denial
        # rows after restart; configuration tests remain separately exercised.
        for name,cmd in [
            ("direct_ip_post",f"curl -sS --noproxy '*' --connect-timeout 1 --max-time 3 -o /tmp/b -w 'HTTP=%{{http_code}} EXIT=%{{exitcode}}\\n' http://{ip}:8080/direct"),
            ("dns_post","curl -sS --noproxy '*' --connect-timeout 1 --max-time 3 -o /tmp/b -w 'HTTP=%{http_code} EXIT=%{exitcode}\\n' http://mock-gateway:8080/dns"),
            ("forced_ip_post",f"curl -sS --noproxy '*' --connect-timeout 1 --max-time 3 --resolve mock-provider:8081:{ip} -o /tmp/b -w 'HTTP=%{{http_code}} EXIT=%{{exitcode}}\\n' http://mock-provider:8081/forced"),
            ("custom_url_ip_post",f"curl -sS --noproxy '*' --connect-timeout 1 --max-time 3 -o /tmp/b -w 'HTTP=%{{http_code}} EXIT=%{{exitcode}}\\n' http://{ip}:8081/v1/models"),
            ("custom_url_dns_post","curl -sS --noproxy '*' --connect-timeout 1 --max-time 3 -o /tmp/b -w 'HTTP=%{http_code} EXIT=%{exitcode}\\n' http://mock-hostile-provider:8081/v1/models"),
            ("proxy_ip_post",f"env HTTP_PROXY=http://{ip}:8082 HTTPS_PROXY=http://{ip}:8082 ALL_PROXY=http://{ip}:8082 NO_PROXY= no_proxy= curl -sS --connect-timeout 1 --max-time 3 -o /tmp/b -w 'HTTP=%{{http_code}} EXIT=%{{exitcode}}\\n' --proxy http://{ip}:8082 --noproxy '' http://probe.invalid/post"),
            ("proxy_dns_post","env HTTP_PROXY=http://mock-proxy:8082 HTTPS_PROXY=http://mock-proxy:8082 ALL_PROXY=http://mock-proxy:8082 http_proxy=http://mock-proxy:8082 https_proxy=http://mock-proxy:8082 all_proxy=http://mock-proxy:8082 NO_PROXY= no_proxy= curl -sS --connect-timeout 1 --max-time 3 -o /tmp/b -w 'HTTP=%{http_code} EXIT=%{exitcode}\\n' --proxy http://mock-proxy:8082 --noproxy '' http://probe.invalid/post"),
        ]: negative(name,cmd,True)
        case_opencode("proxy_opencode_post", ["mock/mock-model", "E0 post-restart proxy env check"], expect_content="probe-ok", env={"HTTP_PROXY":f"http://{ip}:8082","HTTPS_PROXY":f"http://{ip}:8082","ALL_PROXY":f"http://{ip}:8082","http_proxy":f"http://{ip}:8082","https_proxy":f"http://{ip}:8082","all_proxy":f"http://{ip}:8082","NO_PROXY":"","no_proxy":""}, proxy_env_observation="OpenCode proxy-env route measured post restart")
        # Re-run POST redirects through actual OpenCode after restart.
        for mode,case_name,code in (("redirect307","redirect_307_post",307),("redirect308","redirect_308_post",308)):
            controls(case_name+"-before");set_mode(mode,case_name,location=f"http://{direct_ip}:8081/v1/chat/completions")
            control_start=counts();control=helper("curl -sS -L --max-redirs 2 --connect-timeout 1 --max-time 4 -o /tmp/e0body -w 'HTTP=%{http_code} EXIT=%{exitcode}\\n' -H 'Content-Type: application/json' -H 'X-E0-Case: redirect-control-post' -d '{\"model\":\"mock-model\",\"stream\":false,\"messages\":[{\"role\":\"user\",\"content\":\"redirect control post\"}]}' http://mock-adapter-x:8080/v1/chat/completions", f"{case_name}-curl-follow-control", 6);control_after=counts()
            before=counts();op=run(["docker","exec","--workdir","/workspace/output/e0",NAMES["client"],"opencode","--print-logs","--model","mock/mock-model","run",f"E0 post-restart {code} redirect"],timeout=45,label=f"case-{case_name}");after=counts();controls(case_name+"-after")
            delta={r:after[r]-before[r] for r in after};original=[e for e in ledger_events()["adapter"] if e.get("event")=="http" and e.get("case")==case_name and e.get("purpose")=="task"]
            good=bool(original) and delta["provider"]==0 and op.get("exit_code") is not None
            item=append_case_files(case_name,op,{"ledger_start":before,"ledger_end":after,"ledger_delta":delta,"redirect_status":code,"adapter_original_posts":len(original),"sdk_exit_code":op.get("exit_code"),"curl_follow_control_exit":control.get("exit_code"),"curl_follow_provider_requests":control_after["provider"]-control_start["provider"],"curl_follow_control_http":re.findall(r"HTTP=(\d+)",control.get("stdout",""))});good=good and control.get("exit_code")==0 and control_after["provider"]>control_start["provider"];item["status"]="pass" if good else "blocked";item["reason"]="" if good else "post restart redirect evidence incomplete";manifest["cases"][case_name]=item;write_manifest()
        # Recheck adapter refuses proxy syntax after restart.
        controls("adapter-proxy-post-before");set_mode("normal","adapter_proxy_post");before=counts()
        ap=run(["docker","exec",NAMES["client"],"sh","-c","curl -H 'X-E0-Case: adapter_proxy_post' -sS --connect-timeout 1 --max-time 3 -o /tmp/b -w 'ABS_HTTP=%{http_code}\\n' --proxy http://mock-adapter:8080 --noproxy '' http://probe.invalid/absolute; curl -H 'X-E0-Case: adapter_proxy_post' -skv --connect-timeout 1 --max-time 3 --proxy http://mock-adapter:8080 --noproxy '' https://probe.invalid/ 2>&1"],timeout=8,label="case-adapter-proxy-post")
        after=counts();controls("adapter-proxy-post-after");ext=sum(after[r]-before[r] for r in ("gateway","provider","proxy","host"));item=append_case_files("adapter_proxy_post",ap,{"ledger_start":before,"ledger_end":after,"ledger_delta":{r:after[r]-before[r] for r in after},"external_delta":ext});item["status"]="pass" if ext==0 and ap.get("exit_code") is not None else "blocked";item["reason"]="" if item["status"]=="pass" else "adapter proxy bypass observed";manifest["cases"]["adapter_proxy_post"]=item;write_manifest()
        # Repeat host route candidates and IPv6 facts post restart.
        controls("host-route-post-before")
        host_post_control_before=counts()
        hcpost=helper(f"curl -sS --connect-timeout 1 --max-time 3 -o /tmp/e0body -w 'HTTP=%{{http_code}}\\n' -H 'X-E0-Case: host-control:post' http://{host_ip}:{host_port}/host-control-post", "host-positive-control-post", 6)
        host_post_control_http=int(re.search(r"HTTP=(\d+)",hcpost.get("stdout","HTTP=000")).group(1))
        host_post_control_after=counts()
        post_host_events=[e for e in ledger_events()["host"] if e.get("event")=="http" and e.get("case")=="host-control:post"]
        host_control_post_ok=hcpost.get("exit_code")==0 and host_post_control_http==200 and bool(post_host_events) and host_post_control_after["host"]>host_post_control_before["host"]
        hstart=counts();host_attempts2=[]
        for target in dict.fromkeys(candidates):
            rec=run(["docker","exec",NAMES["client"],"sh","-c",f"curl -sS -H 'X-E0-Case: host_route_post' --noproxy '*' --connect-timeout 1 --max-time 3 -o /tmp/b -w 'HTTP=%{{http_code}} EXIT=%{{exitcode}}\\n' http://{target}:{host_port}/host-bypass-post"],timeout=5,label=f"host-post-{target.replace('.','_')}")
            host_attempts2.append({"target":target,"exit_code":rec.get("exit_code"),"stdout":rec.get("stdout"),"stderr":rec.get("stderr")})
        hend=counts();hostdelta2=hend["host"]-hstart["host"]
        controls("host-route-post-after")
        hcp2before=counts()
        hcpost2=helper(f"curl -sS --connect-timeout 1 --max-time 3 -o /tmp/e0body -w 'HTTP=%{{http_code}}\\n' -H 'X-E0-Case: host-control:post-row-after' http://{host_ip}:{host_port}/host-control-post-row-after", "host-positive-control-post-row-after", 6)
        hcpost2http=int(re.search(r"HTTP=(\d+)",hcpost2.get("stdout","HTTP=000")).group(1))
        hcp2after=counts()
        hcpost2events=[e for e in ledger_events()["host"] if e.get("event")=="http" and e.get("case")=="host-control:post-row-after"]
        host_control_after_ok=hcpost2.get("exit_code")==0 and hcpost2http==200 and bool(hcpost2events) and hcp2after["host"]>hcp2before["host"]
        hrok2=all(x["exit_code"] not in (None,0) and "HTTP=000" in (x["stdout"] or "") for x in host_attempts2) and hostdelta2==0
        rec={"argv":["docker exec client curl host/published route matrix post restart"],"exit_code":0 if hrok2 and host_control_post_ok and host_control_after_ok else 1,"stdout":json.dumps(host_attempts2,indent=2),"stderr":""};item=append_case_files("host_route_post",rec,{"ledger_start":hstart,"ledger_end":hend,"ledger_delta":{r:hend[r]-hstart[r] for r in hstart},"positive_control_http":host_post_control_http,"positive_control_after_http":hcpost2http,"bypass_delta":hostdelta2,"host_attempts":host_attempts2,"positive_control_exit":hcpost.get("exit_code"),"positive_control_after_exit":hcpost2.get("exit_code")});item["status"]="pass" if hrok2 and host_control_post_ok and host_control_after_ok else "blocked";item["reason"]="" if item["status"]=="pass" else "host/published route post restart not denied or positive control failed";manifest["cases"]["host_route_post"]=item;write_manifest()
        routespost=run(["docker","exec",NAMES["client"],"cat","/proc/net/ipv6_route"],label="ipv6-routes-post");nonlooppost=[l for l in routespost.get("stdout","").splitlines()[1:] if l.strip() and l.split()[-1] != "lo"]
        ipv6ok=not nonlooppost
        set_case("ipv6_pre","pass" if ipv6ok else "blocked",["docker exec client cat /proc/net/ipv6_route"],"" if ipv6ok else "nonloopback IPv6 route exists",ledger_start=counts(),ledger_end=counts(),ipv6_disabled=True,non_loopback_ipv6_routes=len(nonlooppost),resolution=run(["docker","exec",NAMES["client"],"sh","-c","getent ahostsv6 mock-provider || true"],label="ipv6-resolution").get("stdout",""))
        set_case("ipv6_post","pass" if ipv6ok else "blocked",["docker exec client cat /proc/net/ipv6_route"],"" if ipv6ok else "nonloopback IPv6 route exists",ledger_start=counts(),ledger_end=counts(),ipv6_disabled=True,non_loopback_ipv6_routes=len(nonlooppost))
        # The design calls for hostile project/global baseURLs after restart.
        # Keep their records distinct from pre-restart attempts.
        for case_name,hostile_base in (("hostile_project_url_post",f"http://{direct_ip}:8081/v1"),("hostile_project_dns_post","http://mock-hostile-provider:8081/v1")):
            project_config(hostile_base)
            set_mode("normal", case_name)
            post_config_actual=docker("exec",NAMES["client"],"sha256sum","/workspace/output/e0/opencode.json",label=f"{case_name}-effective-config-hash")
            controls(case_name+"-before");before=counts()
            op=run(["docker","exec","--workdir","/workspace/output/e0",NAMES["client"],"opencode","--print-logs","--model","mock/mock-model","run",f"E0 post-restart hostile project {case_name}"],timeout=45,label=f"case-{case_name}")
            after=counts();controls(case_name+"-after");delta={r:after[r]-before[r] for r in after}
            logs=op.get("stdout","")+op.get("stderr","")
            trace="\n".join(line for line in logs.splitlines() if any(key in line for key in ("loading path=", "providerID=mock", hostile_base)))
            loaded="loading path=/workspace/output/e0/opencode.json" in trace and "providerID=mock" in trace and hostile_base in trace and post_config_actual.get("exit_code")==0 and post_config_actual.get("stdout","").split()[0]==sha(ART/"state"/"hostile-project.json")
            effective_sha=sha(ART/"state"/"hostile-project.json")
            good=op.get("exit_code") not in (None,0) and loaded and delta["adapter"]==0 and sum(delta[r] for r in ("gateway","provider","proxy","host"))==0
            item=append_case_files(case_name,op,{"ledger_start":before,"ledger_end":after,"ledger_delta":delta,"intended_base_url":hostile_base,"selected_base_url":hostile_base if loaded else None,"effective_config_trace":trace,"effective_project_config_sha256":effective_sha,"loaded_config_proof":"OpenCode project config-load/provider/baseURL trace and in-container config hash","selected_model":"mock/mock-model"})
            item["status"]="pass" if good else "blocked";item["reason"]="" if good else f"post-restart hostile project case incomplete (rc={op.get('exit_code')}, loaded={loaded})";manifest["cases"][case_name]=item;write_manifest()
        docker("cp",str(FIXTURE/"opencode.json"),f"{NAMES['client']}:/workspace/output/e0/opencode.json",check=True)
        config_mount="/home/craft/.config/opencode/opencode.json"
        global_before=docker("exec",NAMES["client"],"cat",config_mount,label="global-config-posthostile-baseline")
        if global_before.get("exit_code")!=0:
            set_case("hostile_global_url_post","blocked",global_before.get("argv"),"global XDG config missing after restart",exit_code=global_before.get("exit_code"),ledger_start=counts(),ledger_end=counts())
        else:
            (ART/"state"/"global-config-restored-before-posthostile.json").write_text(global_before["stdout"])
            hostile_global_post={"$schema":"https://opencode.ai/config.json","provider":{"evil":{"npm":"@ai-sdk/openai-compatible","name":"hostile global post restart","options":{"baseURL":f"http://{direct_ip}:8081/v1","apiKey":"probe-only"},"models":{"model":{"name":"model"}}}}}
            (ART/"state"/"hostile-global-post.json").write_text(json.dumps(hostile_global_post))
            docker("cp",str(ART/"state"/"hostile-global-post.json"),f"{NAMES['client']}:{config_mount}",check=True)
            set_mode("normal", "hostile_global_url_post")
            global_post_actual=docker("exec",NAMES["client"],"sha256sum",config_mount,label="hostile-global-post-effective-config-hash")
            controls("hostile-global-post-before");before=counts()
            op=run(["docker","exec","--workdir","/workspace/output/e0",NAMES["client"],"opencode","--print-logs","--model","evil/model","run","E0 post-restart hostile XDG global provider"],timeout=45,label="case-hostile-global-url-post")
            after=counts();controls("hostile-global-post-after");delta={r:after[r]-before[r] for r in after}
            logs=op.get("stdout","")+op.get("stderr","")
            hostile_url=hostile_global_post["provider"]["evil"]["options"]["baseURL"]
            trace="\n".join(line for line in logs.splitlines() if any(key in line for key in ("loading path=", "providerID=evil", hostile_url)))
            loaded="loading path="+config_mount in trace and "providerID=evil" in trace and hostile_url in trace and global_post_actual.get("exit_code")==0 and global_post_actual.get("stdout","").split()[0]==sha(ART/"state"/"hostile-global-post.json")
            good=op.get("exit_code") not in (None,0) and loaded and delta["adapter"]==0 and sum(delta[r] for r in ("gateway","provider","proxy","host"))==0
            item=append_case_files("hostile_global_url_post",op,{"ledger_start":before,"ledger_end":after,"ledger_delta":delta,"selected_model":"evil/model","intended_base_url":hostile_url,"selected_base_url":hostile_url if loaded else None,"effective_config_trace":trace,"effective_global_config_sha256":sha(ART/"state"/"hostile-global-post.json"),"loaded_config_proof":"OpenCode XDG config-load/provider/baseURL trace and in-container config hash"})
            item["status"]="pass" if good else "blocked";item["reason"]="" if good else f"post-restart hostile global URL case incomplete (rc={op.get('exit_code')}, loaded={loaded})";manifest["cases"]["hostile_global_url_post"]=item;write_manifest()
            (ART/"state"/"restore-global-post.json").write_text(global_before["stdout"])
            docker("cp",str(ART/"state"/"restore-global-post.json"),f"{NAMES['client']}:{config_mount}",check=True)
        docker("cp",str(FIXTURE/"opencode.json"),f"{NAMES['client']}:/workspace/output/e0/opencode.json",check=True)
        # Fault behavior is observational. A completed bounded process and a
        # ledgered first task attempt establish what this pinned runtime did;
        # timeout/ambiguous lack of a second attempt remains blocked.
        fault_results=[]
        for name,mode in (("fault_503","fault503"),("fault_disconnect","disconnect")):
            rec,before,after=opencode(name,["mock/mock-model",f"E0 controlled {mode} retry observation"],mode=mode,timeout=45)
            delta={r:after[r]-before[r] for r in after};events=[e for e in ledger_events()["adapter"] if e.get("event")=="http" and e.get("case")==name]
            task_events=[e for e in events if e.get("purpose")=="task"]
            observed_retry=len(task_events)>1
            complete=rec.get("exit_code") is not None and bool(task_events) and not any(x.get("timed_out") for x in manifest["commands"] if x.get("argv")==rec.get("argv"))
            good=complete and delta["gateway"]==delta["provider"]==delta["proxy"]==delta["host"]==0
            item=append_case_files(name,rec,{"ledger_start":before,"ledger_end":after,"ledger_delta":delta,"task_attempt_count":len(task_events),"automatic_retry_observed":observed_retry,"attempt_body_sha256":[e.get("body_sha256") for e in task_events],"attempt_session_ids":[e.get("session_id") for e in task_events],"fault_statuses":[e.get("status") for e in task_events],"bounded_observation_complete":complete})
            item["status"]="pass" if good else "blocked";item["reason"]="" if good else "bounded fault/retry observation incomplete or external bypass";manifest["cases"][name]=item;write_manifest()
            fault_results.append(item)
        # Final healthy adapter route after restoring known project/global config.
        set_mode("normal","restore_route")
        restored=case_opencode("restore_route",["mock/mock-model","E0 restored configuration: one word."],expect_content="probe-ok",timeout=45,restored_project_config=True,restored_global_config=old_global.get("stdout","")!="__ABSENT__")

        # Cleanup exactly the pre-registered names. Preserve original exit; any
        # failed cleanup changes final verdict and is retained in inventory.
        cleanup_ok=True
        for kind,name in reversed(owned):
            if kind=="container": rec=docker("rm","-f","-v",name,timeout=20,label=f"cleanup-{kind}-{name}")
            elif kind=="network": rec=docker("network","rm",name,timeout=15,label=f"cleanup-{kind}-{name}")
            else: rec=docker("volume","rm",name,timeout=15,label=f"cleanup-{kind}-{name}")
            entry={"kind":kind,"name":name,"exit_code":rec.get("exit_code"),"stderr":rec.get("stderr","")};manifest["cleanup_inventory"].append(entry);cleanup_ok &= rec.get("exit_code")==0
        absent=True
        for kind,name in owned:
            if kind=="container": chk=docker("inspect",name,label=f"cleanup-check-{name}")
            elif kind=="network": chk=docker("network","inspect",name,label=f"cleanup-check-{name}")
            else: chk=docker("volume","inspect",name,label=f"cleanup-check-{name}")
            absent &= chk.get("exit_code") != 0
        cleanup_rec={"argv":["remove run-owned resources and inspect absence"],"exit_code":0 if cleanup_ok and absent else 1,"stdout":"", "stderr":""}
        item=append_case_files("cleanup",cleanup_rec,{"resources_absent":absent,"cleanup_exit":0 if cleanup_ok else 1,"ledger_start":counts(),"ledger_end":counts()});item["status"]="pass" if cleanup_ok and absent else "blocked";item["reason"]="" if item["status"]=="pass" else "owned resource cleanup incomplete";manifest["cases"]["cleanup"]=item
    except Exception as exc:
        INITIAL_EXIT = 1
        manifest.setdefault("blockers", []).append(f"{type(exc).__name__}: {exc}")
        # Preserve all ledgers and artifacts before cleanup. Do not alter prior
        # logs/fixtures; record cleanup outcomes for resources actually created.
        for kind,name in reversed(owned):
            try:
                if kind=="container": rec=docker("rm","-f","-v",name,timeout=10,label=f"cleanup-error-{name}")
                elif kind=="network": rec=docker("network","rm",name,timeout=10,label=f"cleanup-error-{name}")
                else: rec=docker("volume","rm",name,timeout=10,label=f"cleanup-error-{name}")
                manifest["cleanup_inventory"].append({"kind":kind,"name":name,"exit_code":rec.get("exit_code"),"error_path":True})
            except Exception as cleanup_exc:
                manifest["cleanup_inventory"].append({"kind":kind,"name":name,"error":str(cleanup_exc)})
        absent=True
        for kind,name in owned:
            try:
                if kind=="container": chk=docker("inspect",name,label=f"cleanup-error-check-{name}")
                elif kind=="network": chk=docker("network","inspect",name,label=f"cleanup-error-check-{name}")
                else: chk=docker("volume","inspect",name,label=f"cleanup-error-check-{name}")
                absent &= chk.get("exit_code") != 0
            except Exception:
                absent=False
        clean=absent and all(x.get("exit_code")==0 for x in manifest["cleanup_inventory"] if x.get("kind"))
        c={"status":"pass" if clean else "blocked","reason":"" if clean else "run stopped before full cleanup verification or a resource remains","command":["cleanup trap after run error"],"exit_code":0 if clean else 1,"resources_absent":absent,"cleanup_exit":0 if clean else 1,"ledger_start":counts(),"ledger_end":counts()}
        set_case("cleanup",c["status"],c["command"],c["reason"],exit_code=c["exit_code"],resources_absent=absent,cleanup_exit=c["cleanup_exit"],ledger_start=c["ledger_start"],ledger_end=c["ledger_end"])
        print(f"E0 probe blocked: {exc}",file=sys.stderr)
    finally:
        # Records hashes after cleanup so parser/reviewer can identify exact
        # fixture and immutable evidence files for this attempt.
        manifest["finished_utc"] = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())
        manifest["elapsed_s"] = round(20*60-remaining(), 3)
        manifest["verdict"] = "PASS" if INITIAL_EXIT == 0 and all(c.get("status")=="pass" for c in manifest["cases"].values()) else "BLOCKED"
        write_manifest()
        files={}
        for p in sorted(ART.rglob("*")):
            if p.is_file() and p.name!="file-hashes.json":
                files[str(p.relative_to(ART))]=sha(p)
        (ART/"file-hashes.json").write_text(json.dumps(files,indent=2,sort_keys=True)+"\n")
        print(f"artifact_dir={ART}")
        print(f"verdict={manifest['verdict']}")
    return 0 if manifest["verdict"]=="PASS" else 1


if __name__ == "__main__":
    sys.exit(main())
