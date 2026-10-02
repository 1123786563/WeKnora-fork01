#!/usr/bin/env python3
"""Host-side orchestrator for the trusted T14 renderer policy helper.

Every durable receipt is written under --evidence-dir with a monotonic
sequence number; the helper container itself is the only component that
mutates the renderer namespace.
"""
import argparse
import fcntl
import hashlib
import importlib.util
import json
import os
import pathlib
import selectors
import subprocess
import sys
import time
import uuid


HERE = pathlib.Path(__file__).resolve().parent
MAX_OUTPUT = 256 * 1024
HELPER_RUN_TIMEOUT = 40
DOCKER_TIMEOUT = 15
EXEC_TIMEOUT = 10
PREVIEW_PORT_FORBIDDEN = 8081
HELPER_ENV_PASSTHROUGH = ("T14_POLICY_HELPER_TEST_PAUSE_AFTER_ROUTE",)

RAW_REF_NAMES = {"nft-rules": "raw_rules", "nft-ruleset": "raw_ruleset",
                 "routes": "raw_route_json", "links": "raw_link_json",
                 "container-inspect-before": "container_inspect_raw",
                 "container-inspect-after": "container_inspect_after_raw"}
HELPER_RAW_FIELDS = {"raw_rules": "nft-rules", "raw_ruleset": "nft-ruleset",
                     "raw_route_json": "routes", "raw_link_json": "links"}


def _load_flow_module():
    spec = importlib.util.spec_from_file_location("t14_barrier_adapter", HERE / "barrier_adapter.py")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


_FLOW_MODULE = _load_flow_module()
select_webdriver_flow = _FLOW_MODULE.select_webdriver_flow
WEBDRIVER_DISCOVERY = _FLOW_MODULE.WEBDRIVER_DISCOVERY


def run(args, *, timeout=DOCKER_TIMEOUT, output_limit=MAX_OUTPUT, input_text=None):
    """Run a command with bounded output capture; never retain unbounded data."""
    proc = subprocess.Popen(args, stdin=subprocess.PIPE if input_text is not None else subprocess.DEVNULL,
                            stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=False)
    if input_text is not None:
        try:
            proc.stdin.write(input_text.encode())
            proc.stdin.close()
        except BrokenPipeError:
            pass
    selector = selectors.DefaultSelector()
    buffers = {proc.stdout: bytearray(), proc.stderr: bytearray()}
    for stream in buffers:
        selector.register(stream, selectors.EVENT_READ)
    deadline = time.monotonic() + timeout
    try:
        while selector.get_map():
            remaining = deadline - time.monotonic()
            if remaining <= 0:
                raise subprocess.TimeoutExpired(args, timeout)
            for key, _mask in selector.select(min(remaining, .2)):
                chunk = os.read(key.fd, min(8192, output_limit + 1 - len(buffers[key.fileobj])))
                if not chunk:
                    selector.unregister(key.fileobj)
                    continue
                buffers[key.fileobj].extend(chunk)
                if len(buffers[key.fileobj]) > output_limit:
                    raise RuntimeError(f"child output exceeded bounded capture limit: {' '.join(map(str, args))}")
        return subprocess.CompletedProcess(
            args, proc.wait(timeout=max(.01, deadline - time.monotonic())),
            buffers[proc.stdout].decode(errors="replace"), buffers[proc.stderr].decode(errors="replace"))
    except BaseException:
        proc.kill()
        proc.wait()
        raise
    finally:
        selector.close()
        proc.stdout.close()
        proc.stderr.close()


def wait_for_reuse_marker(read_marker, renderer_running, *, expected, timeout,
                          interval=.05, now=time.monotonic, sleep=time.sleep):
    deadline = now() + timeout
    observed = None
    while True:
        remaining = deadline - now()
        if remaining <= 0:
            if observed is not None and observed != expected:
                raise RuntimeError(f"marker contents mismatch at deadline; observed={observed!r}")
            raise TimeoutError(f"marker deadline expired; expected={expected!r}, observed={observed!r}")
        if not renderer_running():
            raise RuntimeError(f"renderer exited before marker was confirmed; observed={observed!r}")
        if now() >= deadline:
            raise TimeoutError(f"marker deadline expired after renderer check; observed={observed!r}")
        observed = read_marker()
        if now() >= deadline:
            raise TimeoutError(f"marker deadline expired before accepting marker; observed={observed!r}")
        if observed == expected:
            return observed
        remaining = deadline - now()
        if remaining <= 0:
            if observed is not None:
                raise RuntimeError(f"marker contents mismatch at deadline; observed={observed!r}")
            raise TimeoutError(f"marker deadline expired; observed={observed!r}")
        sleep(min(interval, remaining))


def same_listener_canary_code(family, address, port, avoid_source_port):
    family_expr = "socket.AF_INET" if family == 4 else "socket.AF_INET6"
    return (
        "import socket,sys,json,errno,time;"
        f"family={family_expr};address={address!r};port={int(port)};"
        "s=socket.socket(family,socket.SOCK_STREAM);s.settimeout(.5);"
        "s.bind((('127.0.0.1' if family==socket.AF_INET else '::1'),0));"
        "local=s.getsockname()[1];"
        "out={'destination':address,'destination_port':port,'source_port':local,'connect_attempted':False};\n"
        f"if local=={int(avoid_source_port)}:print(json.dumps(out));sys.exit(3)\n"
        "out['connect_attempted']=True;started=time.monotonic();\n"
        "try:s.connect((address,port));out.update({'connected':True,'error_type':None,'errno':None});print(json.dumps(out));sys.exit(0)\n"
        "except OSError as exc:out.update({'connected':False,'error_type':type(exc).__name__,'errno':exc.errno,"
        "'connect_elapsed_ms':int((time.monotonic()-started)*1000)});print(json.dumps(out));sys.exit(1)"
    )


def same_listener_canary_passed(evidence, *, address, port, source_port, returncode, delta_packets):
    """Accept only a distinct-source fresh SYN that was actually denied and counted."""
    if not isinstance(evidence, dict) or returncode != 1:
        return False
    if not isinstance(delta_packets, int) or delta_packets <= 0:
        return False
    if evidence.get("destination") != address or evidence.get("destination_port") != port:
        return False
    local_port = evidence.get("source_port")
    if not isinstance(local_port, int) or local_port == source_port:
        return False
    if evidence.get("connect_attempted") is not True or evidence.get("connected") is not False:
        return False
    elapsed = evidence.get("connect_elapsed_ms")
    if not isinstance(elapsed, int):
        return False
    errno = evidence.get("errno")
    timeout_denial = evidence.get("error_type") == "TimeoutError" and elapsed >= 400
    errno_denial = isinstance(errno, int) and errno > 0
    return timeout_denial or errno_denial


class Controller:
    def __init__(self, helper_image, evidence_dir):
        self.image = helper_image
        self.evidence = pathlib.Path(evidence_dir)
        self.evidence.mkdir(parents=True, exist_ok=True)
        self.active_helper_name = ""
        self.last_helper_command = []
        self.last_helper_meta = {}
        self.cleanup_actions = []
        self.target = {}
        self._helper_image_id = self._inspect_helper_image()
        self._stop_fault_injected = False

    def _inspect_helper_image(self):
        payload = json.loads(run(["docker", "image", "inspect", self.image, "--format", "{{json .}}"]).stdout)
        if isinstance(payload, list):
            payload = payload[0]
        return payload.get("Id")

    def _evidence_lock(self):
        handle = open(self.evidence / ".controller.lock", "a+")
        return handle

    def _locked(self):
        handle = self._evidence_lock()
        fcntl.flock(handle, fcntl.LOCK_EX)
        return handle

    def _next_sequence(self):
        counter = self.evidence / ".sequence"
        value = 0
        if counter.exists():
            value = int(counter.read_text().strip() or 0)
        value += 1
        counter.write_text(f"{value}\n")
        return value

    def reserve_sequence(self):
        with self._locked() as _handle:
            return self._next_sequence()

    def inspect(self, container):
        result = run(["docker", "inspect", container])
        return result.stdout, json.loads(result.stdout)[0]

    def helper(self, container, action, policy_id="", expected_netns="", preview_port=8081, flow=None):
        """Run the trusted helper image once inside the renderer namespace."""
        image_payload = json.loads(run(["docker", "image", "inspect", self.image, "--format", "{{json .}}"]).stdout)
        if isinstance(image_payload, list):
            image_payload = image_payload[0]
        raw_before, data_before = self.inspect(container)
        name = f"t14ph-{uuid.uuid4().hex[:12]}"
        argv = ["docker", "run", "--rm", "--name", name,
                "--network", f"container:{container}",
                "--cap-drop", "ALL", "--cap-add", "NET_ADMIN",
                "--security-opt", "no-new-privileges", "--read-only", "--user", "0:0",
                "--tmpfs", "/tmp:rw,noexec,nosuid,size=16m",
                "--label", "io.weknora.craft.policy-helper=true",
                "--label", f"io.weknora.craft.policy-id={policy_id}",
                "--label", f"io.weknora.craft.renderer-id={container}"]
        for key in HELPER_ENV_PASSTHROUGH:
            value = os.environ.get(key)
            if value is not None:
                argv += ["--env", f"{key}={value}"]
        argv.append(self.image)
        argv.append(action)
        if expected_netns:
            argv += ["--expected-netns", expected_netns]
        if policy_id:
            argv += ["--policy-id", policy_id]
        if action == "install":
            argv += ["--preview-port", str(preview_port)]
        if flow is not None:
            argv += ["--webdriver-flow-json", json.dumps(flow, sort_keys=True)]
        self.active_helper_name = name
        self.last_helper_command = argv
        timeout = int(os.environ.get("T14_POLICY_HELPER_TEST_HELPER_TIMEOUT", HELPER_RUN_TIMEOUT))
        result = run(argv, timeout=timeout)
        if result.returncode:
            raise RuntimeError(f"helper {action} failed ({result.returncode}): {result.stderr.strip()}")
        parsed = json.loads(result.stdout or "{}")
        raw_after, _data_after = self.inspect(container)
        actions = getattr(self, "cleanup_actions", None)
        if actions is not None:
            actions.append(action)
        meta = {"container": data_before["Id"], "container_pid": data_before["State"]["Pid"],
                "container_image_id": data_before["Image"],
                "helper_image_ref": self.image, "helper_image_id": image_payload["Id"],
                "container_inspect_raw": raw_before, "container_inspect_after_raw": raw_after,
                "helper_result": parsed, "helper_returncode": result.returncode,
                "helper_argv": argv}
        self.last_helper_meta = meta
        netns = parsed.get("netns") if isinstance(parsed, dict) else None
        return {"netns": netns}, meta

    def helper_raws(self, meta):
        result = meta.get("helper_result", {})
        raws = {suffix: result[field] for field, suffix in HELPER_RAW_FIELDS.items() if field in result}
        raws["container-inspect-before"] = meta.get("container_inspect_raw", "")
        raws["container-inspect-after"] = meta.get("container_inspect_after_raw", "")
        return raws

    def write_capture(self, action, payload, raws=None, sequence=None):
        with self._locked() as _handle:
            if sequence is None:
                sequence = self._next_sequence()
            raw_refs = {}
            for suffix, content in (raws or {}).items():
                path = f"{sequence:04d}-{suffix}.json"
                (self.evidence / path).write_text(content)
                raw_refs[RAW_REF_NAMES[suffix]] = {
                    "path": path, "bytes": len(content.encode()),
                    "sha256": hashlib.sha256(content.encode()).hexdigest()}
            record = {"action": action,
                      "helper": {"image_ref": self.image, "image_id": self._helper_image_id},
                      "monotonic_ns": time.monotonic_ns(),
                      "payload": payload, "raw_refs": raw_refs,
                      "sequence": sequence, "target": dict(self.target)}
            (self.evidence / f"{sequence:04d}-{action}.json").write_text(
                json.dumps(record, indent=2, sort_keys=True) + "\n")
            return record

    def stop_active_helper(self):
        if not self.active_helper_name:
            return {"attempted": False, "confirmed_removed": True, "reason": "no active helper"}
        argv = ["docker", "rm", "-f", self.active_helper_name]
        fault = os.environ.get("T14_POLICY_HELPER_TEST_STOP_FAULT")
        try:
            if fault and not getattr(self, "_stop_fault_injected", False):
                self._stop_fault_injected = True
                if fault == "exception":
                    raise OSError("injected Docker client failure")
                if fault == "timeout":
                    raise subprocess.TimeoutExpired(argv, 10)
                if fault == "false":
                    return {"attempted": True, "confirmed_removed": False,
                            "argv": argv, "returncode": 1}
            result = run(argv, timeout=DOCKER_TIMEOUT)
        except (OSError, subprocess.SubprocessError) as exc:
            return {"attempted": True, "confirmed_removed": False, "argv": argv,
                    "error": {"type": type(exc).__name__, "message": str(exc)}}
        removed = result.returncode == 0 or self._container_absent(self.active_helper_name)
        return {"attempted": True, "confirmed_removed": removed, "argv": argv,
                "returncode": result.returncode, "stdout": result.stdout, "stderr": result.stderr}

    def _container_absent(self, container):
        return run(["docker", "inspect", container], timeout=DOCKER_TIMEOUT).returncode != 0

    def destroy_renderer(self, container):
        argv = ["docker", "rm", "-f", container]
        try:
            result = run(argv, timeout=20)
        except (OSError, subprocess.SubprocessError) as exc:
            return {"confirmed_destroyed": False, "argv": argv,
                    "error": {"type": type(exc).__name__, "message": str(exc)},
                    "renderer_id": container}
        confirmed = result.returncode == 0 or self._container_absent(container)
        return {"confirmed_destroyed": confirmed, "argv": argv, "renderer_id": container,
                "returncode": result.returncode, "stdout": result.stdout, "stderr": result.stderr}

    def _destroy_after_cleanup_failure(self, container, stop):
        destroy = self.destroy_renderer(container)
        if not destroy["confirmed_destroyed"]:
            return {"cleanup_status": "incomplete", "helper_stop": stop, "destroy": destroy}
        destroy["stopped_helper_after_destroy"] = self.stop_active_helper()
        return {"cleanup_status": "renderer-destroyed", "helper_stop": stop, "destroy": destroy}

    def install_failure_cleanup(self, container, policy_id, netns, preview_port, error):
        """Fail closed: an unverified policy must leave no helper mutation behind."""
        try:
            stop = self.stop_active_helper()
        except Exception as exc:
            stop = {"attempted": True, "confirmed_removed": False,
                    "error": {"type": type(exc).__name__, "message": str(exc)}}
        if not stop.get("confirmed_removed"):
            return self._destroy_after_cleanup_failure(container, stop)
        if os.environ.get("T14_POLICY_HELPER_TEST_FAIL_CLEANUP") == "1":
            return self._destroy_after_cleanup_failure(container, stop)
        try:
            _attestation, cleanup_meta = self.helper(container, "cleanup", policy_id, netns)
            _attestation, baseline_meta = self.helper(container, "baseline", policy_id, netns)
            baseline_record = self.write_capture("cleanup-baseline",
                                                 baseline_meta["helper_result"],
                                                 self.helper_raws(baseline_meta))
            return {"cleanup_status": "verified-clean", "helper_stop": stop,
                    "cleanup": cleanup_meta, "baseline_record": baseline_record}
        except (OSError, subprocess.SubprocessError, RuntimeError, ValueError):
            return self._destroy_after_cleanup_failure(container, stop)

    def _attest_renderer_netns(self, container, expected_netns):
        argv = ["docker", "exec", "--user", "10001:10001", container,
                "readlink", "/proc/self/ns/net"]
        started = time.monotonic_ns()
        result = run(argv, timeout=EXEC_TIMEOUT)
        ended = time.monotonic_ns()
        _raw_before, data_before = self.inspect(container)
        attestation = {"source": "host-issued-docker-exec-readlink-proc-self-ns-net",
                       "argv": argv, "container_id": data_before["Id"],
                       "container_image_id": data_before["Image"],
                       "container_pid_before": data_before["State"]["Pid"],
                       "returncode": result.returncode,
                       "stdout": result.stdout, "stderr": result.stderr,
                       "monotonic_started_ns": started, "monotonic_ended_ns": ended,
                       "netns_inode": result.stdout.strip()}
        if result.returncode:
            raise RuntimeError(f"renderer netns attestation failed: {result.stderr.strip()}")
        _raw_after, data_after = self.inspect(container)
        attestation["container_pid_after"] = data_after["State"]["Pid"]
        attestation["matches_expected"] = bool(expected_netns) and result.stdout.strip() == expected_netns
        if expected_netns and result.stdout.strip() != expected_netns:
            raise RuntimeError(f"renderer namespace mismatch: expected {expected_netns}, "
                               f"attested {result.stdout.strip()}")
        if attestation["container_pid_before"] != attestation["container_pid_after"]:
            raise RuntimeError("renderer identity changed during netns attestation")
        return attestation

    def _attest_preview_port(self, container, preview_port):
        code = ("import json,socket,sys;port=int(sys.argv[1]);"
                "s=socket.create_connection(('127.0.0.1',port),1);s.close();"
                "print(json.dumps({'connected':True,'port':port}))")
        argv = ["docker", "exec", "--user", "10001:10001", container,
                "python3", "-c", code, str(preview_port)]
        started = time.monotonic_ns()
        result = run(argv, timeout=EXEC_TIMEOUT)
        ended = time.monotonic_ns()
        _raw_before, data_before = self.inspect(container)
        _raw_after, data_after = self.inspect(container)
        attestation = {"source": "host-issued-docker-exec-loopback-listener-connect",
                       "argv": argv, "container_id": data_before["Id"],
                       "container_image_id": data_before["Image"],
                       "container_pid_before": data_before["State"]["Pid"],
                       "container_pid_after": data_after["State"]["Pid"],
                       "port": preview_port, "returncode": result.returncode,
                       "stdout": result.stdout, "stderr": result.stderr,
                       "monotonic_started_ns": started, "monotonic_ended_ns": ended,
                       "connected": result.returncode == 0}
        if result.returncode:
            raise RuntimeError(f"preview port {preview_port} did not accept a loopback connection: "
                               f"{result.stderr.strip()}")
        return attestation

    def _discover_webdriver_flow(self, container):
        argv = ["docker", "exec", "--user", "10001:10001", container,
                "python3", "-c", WEBDRIVER_DISCOVERY]
        started = time.monotonic_ns()
        result = run(argv, timeout=EXEC_TIMEOUT)
        ended = time.monotonic_ns()
        raw_before, data_before = self.inspect(container)
        _raw_after, data_after = self.inspect(container)
        if result.returncode:
            raise RuntimeError(f"webdriver socket discovery failed: {result.stderr.strip()}")
        inventory = json.loads(result.stdout)
        try:
            flow = select_webdriver_flow(inventory.get("clients"), inventory.get("listeners"))
        except ValueError as exc:
            raise RuntimeError(f"discovered socket records do not join: {exc}") from exc
        flow_json = json.dumps(flow, sort_keys=True)
        attestation = {"source": "host-issued-docker-exec-proc-fd-and-net-tcp",
                       "argv": ["docker", "exec", "--user", "10001:10001", container, "python3",
                                "-c", "<bounded proc socket inventory script>"],
                       "container_id": data_before["Id"],
                       "container_image_id": data_before["Image"],
                       "container_pid": data_before["State"]["Pid"],
                       "container_inspect_before_raw": raw_before,
                       "container_inspect_after_raw": _raw_after,
                       "returncode": result.returncode, "flow": flow,
                       "flow_sha256": hashlib.sha256(flow_json.encode()).hexdigest(),
                       "monotonic_started_ns": started, "monotonic_ended_ns": ended}
        return flow, attestation

    def _snapshot_payload(self, container, policy_id, netns):
        _attestation, meta = self.helper(container, "snapshot", policy_id, netns)
        return meta["helper_result"], meta

    def _loopback_drop_key(self, flow):
        return "loopback_v4_drop" if flow["family"] == 4 else "loopback_v6_drop"

    def _mandatory_canary(self, container, policy_id, netns, flow):
        before_payload, before_meta = self._snapshot_payload(container, policy_id, netns)
        before_record = self.write_capture("mandatory-canary-before", before_payload,
                                           self.helper_raws(before_meta))
        key = self._loopback_drop_key(flow)
        counter_id = "policy-loopback-v4-drop" if flow["family"] == 4 else "policy-loopback-v6-drop"
        probe_code = ("import socket,sys;s=socket.socket(socket.AF_INET,socket.SOCK_STREAM);"
                      "s.settimeout(.35);\n"
                      "try:s.connect(('127.0.0.1',8081));print('unexpected-connect');sys.exit(0)\n"
                      "except OSError as exc:print(type(exc).__name__+':'+str(exc));sys.exit(1)")
        probe_argv = ["docker", "exec", "--user", "10001:10001", container,
                      "python3", "-c", probe_code]
        started = time.monotonic_ns()
        probe = run(probe_argv, timeout=5)
        ended = time.monotonic_ns()
        after_payload, after_meta = self._snapshot_payload(container, policy_id, netns)
        after_record = self.write_capture("mandatory-canary-after", after_payload,
                                          self.helper_raws(after_meta))
        before_packets = before_payload["counters"][key]["packets"]
        after_packets = after_payload["counters"][key]["packets"]
        delta = after_packets - before_packets
        denied = probe.returncode != 0 and "unexpected-connect" not in probe.stdout
        canary = {"kind": "post-policy-loopback-drop-canary",
                  "destination": "127.0.0.1:8081/tcp", "counter_id": counter_id,
                  "before_packets": before_packets, "after_packets": after_packets,
                  "before_record": before_record["sequence"],
                  "after_record": after_record["sequence"], "delta_packets": delta,
                  "probe_argv": probe_argv,
                  "probe": {"returncode": probe.returncode, "stdout": probe.stdout,
                            "stderr": probe.stderr},
                  "monotonic_started_ns": started, "monotonic_ended_ns": ended,
                  "passed": bool(denied and delta > 0)}
        canary["webdriver_same_listener_canary"] = self._webdriver_canary(
            container, policy_id, netns, flow, after_payload, after_record)
        return canary

    def _webdriver_canary(self, container, policy_id, netns, flow, before_payload, before_record):
        key = self._loopback_drop_key(flow)
        code = same_listener_canary_code(flow["family"], flow["client_addr"],
                                         flow["driver_port"], flow["client_port"])
        probe_argv = ["docker", "exec", "--user", "10001:10001", container,
                      "python3", "-c", code]
        started = time.monotonic_ns()
        probe = run(probe_argv, timeout=5)
        ended = time.monotonic_ns()
        evidence = None
        try:
            evidence = json.loads(probe.stdout)
        except ValueError:
            evidence = None
        after_payload, after_meta = self._snapshot_payload(container, policy_id, netns)
        after_record = self.write_capture("webdriver-canary-after", after_payload,
                                          self.helper_raws(after_meta))
        before_packets = before_payload["counters"][key]["packets"]
        after_packets = after_payload["counters"][key]["packets"]
        delta = after_packets - before_packets
        passed = same_listener_canary_passed(
            evidence, address=flow["client_addr"], port=flow["driver_port"],
            source_port=flow["client_port"], returncode=probe.returncode,
            delta_packets=delta)
        canary = {"destination": f"{flow['driver_addr']}:{flow['driver_port']}/tcp",
                  "counter_id": "policy-loopback-v4-drop" if flow["family"] == 4 else "policy-loopback-v6-drop",
                  "before_packets": before_packets, "after_packets": after_packets,
                  "delta_packets": delta, "probe_argv": probe_argv,
                  "returncode": probe.returncode, "stdout": probe.stdout,
                  "stderr": probe.stderr, "monotonic_started_ns": started,
                  "monotonic_ended_ns": ended, "passed": passed}
        if isinstance(evidence, dict):
            canary.update({name: evidence.get(name) for name in
                           ("source_port", "connect_attempted", "connected",
                            "error_type", "errno", "connect_elapsed_ms")})
        return canary

    def _read_policy(self, policy_id):
        path = self.evidence / "policy.json"
        if not path.exists():
            raise RuntimeError(f"policy not installed for {policy_id}")
        policy = json.loads(path.read_text())
        if not policy.get("active"):
            raise RuntimeError(f"policy not installed for {policy_id}")
        return policy

    def cmd_install(self, args):
        if args.preview_port == PREVIEW_PORT_FORBIDDEN:
            raise RuntimeError(f"preview port {PREVIEW_PORT_FORBIDDEN} is reserved for "
                               "diagnostics and cannot carry the render preview")
        _raw, data = self.inspect(args.container)
        self.target = {"container_id": data["Id"], "container_image_id": data["Image"],
                       "container_pid": data["State"]["Pid"]}
        netns_attestation = self._attest_renderer_netns(args.container, args.expected_netns)
        self.target["netns"] = netns_attestation["netns_inode"]
        (self.evidence / "renderer-netns-attestation.json").write_text(
            json.dumps(netns_attestation, indent=2, sort_keys=True) + "\n")
        preview_attestation = self._attest_preview_port(args.container, args.preview_port)
        (self.evidence / "preview-port-attestation.json").write_text(
            json.dumps(preview_attestation, indent=2, sort_keys=True) + "\n")
        flow, flow_attestation = self._discover_webdriver_flow(args.container)
        (self.evidence / "webdriver-flow-attestation.json").write_text(
            json.dumps(flow_attestation, indent=2, sort_keys=True) + "\n")
        netns = netns_attestation["netns_inode"]
        _att, baseline_meta = self.helper(args.container, "baseline", args.policy_id, netns)
        baseline_record = self.write_capture("baseline", baseline_meta["helper_result"],
                                             self.helper_raws(baseline_meta))
        _att, install_meta = self.helper(args.container, "install", args.policy_id, netns,
                                         args.preview_port, flow)
        install_payload = install_meta["helper_result"]
        install_sequence = self.reserve_sequence()
        canary = self._mandatory_canary(args.container, args.policy_id, netns, flow)
        if not canary["passed"]:
            raise RuntimeError(f"mandatory post-policy loopback-drop canary failed: {canary}")
        if not canary["webdriver_same_listener_canary"]["passed"]:
            raise RuntimeError(f"webdriver same-listener fresh SYN canary failed: "
                               f"{canary['webdriver_same_listener_canary']}")
        payload = dict(install_payload)
        payload["mandatory_canary"] = canary
        install_record = self.write_capture("install", payload,
                                            self.helper_raws(install_meta),
                                            sequence=install_sequence)
        (self.evidence / "mandatory-canary.json").write_text(
            json.dumps(canary, indent=2, sort_keys=True) + "\n")
        flow_json = json.dumps(flow, sort_keys=True)
        policy = {"active": True, "policy_id": args.policy_id,
                  "container_id": data["Id"], "container_image_id": data["Image"],
                  "container_pid": data["State"]["Pid"],
                  "netns": netns, "renderer_netns": netns,
                  "renderer_netns_attestation": "renderer-netns-attestation.json",
                  "helper_netns": install_payload.get("netns"),
                  "preview_port": args.preview_port,
                  "preview_port_attestation": "preview-port-attestation.json",
                  "webdriver_flow": flow,
                  "webdriver_flow_attestation": "webdriver-flow-attestation.json",
                  "webdriver_flow_sha256": hashlib.sha256(flow_json.encode()).hexdigest(),
                  "webdriver_attestation_sha256": hashlib.sha256(
                      (self.evidence / "webdriver-flow-attestation.json").read_bytes()).hexdigest(),
                  "mandatory_canary": canary,
                  "baseline_record": baseline_record["sequence"],
                  "install_record": install_record["sequence"],
                  "helper_image_ref": self.image,
                  "helper_image_id": install_meta["helper_image_id"],
                  "installed_monotonic_ns": time.monotonic_ns()}
        (self.evidence / "policy.json").write_text(json.dumps(policy, indent=2, sort_keys=True) + "\n")
        print(json.dumps(install_record, indent=2, sort_keys=True))

    def cmd_snapshot(self, args):
        policy = self._read_policy(args.policy_id)
        if args.preview_port is not None and args.preview_port != policy["preview_port"]:
            raise RuntimeError(f"preview port mismatch: policy pins {policy['preview_port']}, "
                               f"requested {args.preview_port}")
        netns_attestation = self._attest_renderer_netns(args.container, args.expected_netns)
        payload, meta = self._snapshot_payload(args.container, args.policy_id,
                                               netns_attestation["netns_inode"])
        record = self.write_capture("snapshot", payload, self.helper_raws(meta))
        print(json.dumps(record, indent=2, sort_keys=True))

    def cmd_verify(self, args):
        policy = self._read_policy(args.policy_id)
        netns_attestation = self._attest_renderer_netns(args.container, args.expected_netns)
        payload, meta = self._snapshot_payload(args.container, args.policy_id,
                                               netns_attestation["netns_inode"])
        record = self.write_capture("verify", payload, self.helper_raws(meta))
        record["verified"] = payload["netns"] == policy["helper_netns"]
        print(json.dumps(record, indent=2, sort_keys=True))

    def cmd_cleanup(self, args):
        netns_attestation = self._attest_renderer_netns(args.container, args.expected_netns)
        _att, meta = self.helper(args.container, "cleanup", args.policy_id,
                                 netns_attestation["netns_inode"])
        record = self.write_capture("cleanup", meta["helper_result"], self.helper_raws(meta))
        policy_path = self.evidence / "policy.json"
        if policy_path.exists():
            policy_path.unlink()
        print(json.dumps({"cleanup_verified": True, "policy_id": args.policy_id,
                          "record": record["sequence"],
                          "netns": netns_attestation["netns_inode"]}, indent=2, sort_keys=True))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--helper-image", required=True)
    parser.add_argument("--evidence-dir", required=True)
    commands = parser.add_subparsers(dest="command", required=True)
    install = commands.add_parser("install")
    install.add_argument("--container", required=True)
    install.add_argument("--policy-id", required=True)
    install.add_argument("--preview-port", type=int, required=True)
    install.add_argument("--expected-netns", required=True)
    for name in ("snapshot", "verify", "cleanup"):
        sub = commands.add_parser(name)
        sub.add_argument("--container", required=True)
        sub.add_argument("--policy-id", required=True)
        sub.add_argument("--expected-netns", default="")
        sub.add_argument("--preview-port", type=int)
    args = parser.parse_args()
    controller = Controller(args.helper_image, args.evidence_dir)
    handler = {"install": controller.cmd_install, "snapshot": controller.cmd_snapshot,
               "verify": controller.cmd_verify, "cleanup": controller.cmd_cleanup}[args.command]
    try:
        handler(args)
    except Exception as exc:
        if args.command == "install":
            outcome = controller.install_failure_cleanup(
                args.container, args.policy_id, args.expected_netns, args.preview_port, exc)
            failure = {"verified_policy": False, "expected_netns": args.expected_netns,
                       "cleanup_status": outcome.get("cleanup_status"), "cleanup": outcome,
                       "error": {"type": type(exc).__name__, "message": str(exc)}}
            (controller.evidence / "install-failure.json").write_text(
                json.dumps(failure, indent=2, sort_keys=True) + "\n")
            print(json.dumps(failure, indent=2, sort_keys=True), file=sys.stderr)
        print(f"t14-policy-controller: {exc}", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
