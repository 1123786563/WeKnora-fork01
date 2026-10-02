"""Disposable OrbStack integration checks for the trusted policy helper.

Run with POLICY_HELPER_RUN_DOCKER_TESTS=1 python3 -m unittest discover -s tests.
"""
import json
import os
import importlib.util
import ipaddress
import pathlib
import shutil
import subprocess
import tempfile
import socket
import struct
import time
import unittest
from unittest import mock
import uuid
import subprocess

HERE = pathlib.Path(__file__).resolve().parents[1]
CONTROLLER = HERE / "controller.py"
RENDERER_IMAGE = "craft-t14-task3-validation:2026-09-24-e2f335dc"
HELPER_IMAGE = "craft-t14-policy-helper:2026-09-24"


def run(*args, timeout=30, check=True):
    return subprocess.run(args, check=check, text=True, capture_output=True, timeout=timeout)


def run_before_deadline(deadline, *args, runner=run, now=time.monotonic, **kwargs):
    remaining = deadline - now()
    if remaining <= 0:
        raise TimeoutError("Docker callback deadline expired before subprocess start")
    result = runner(*args, timeout=remaining, **kwargs)
    if now() >= deadline:
        raise TimeoutError("Docker callback completed after marker deadline")
    return result


def wait_for_reuse_marker_bounded(read_marker, renderer_running, *, deadline,
                                  expected=b"reuse-ok", interval=0.05,
                                  now=time.monotonic, sleep=time.sleep):
    observed = None
    while True:
        remaining = deadline - now()
        if remaining <= 0:
            raise TimeoutError(f"reuse marker deadline expired; observed={observed!r}")
        if not renderer_running():
            raise RuntimeError(f"renderer exited before reuse marker was confirmed; observed={observed!r}")
        if now() >= deadline:
            raise TimeoutError(f"reuse marker deadline expired after renderer check; observed={observed!r}")
        observed = read_marker()
        if now() >= deadline:
            raise TimeoutError(f"reuse marker deadline expired before accepting marker; observed={observed!r}")
        if observed == expected:
            return observed
        remaining = deadline - now()
        if remaining <= 0:
            if observed is not None:
                raise RuntimeError(f"reuse marker contents mismatch at deadline; observed={observed!r}")
            raise TimeoutError(f"reuse marker deadline expired; observed={observed!r}")
        sleep(min(interval, remaining))


def _nft_match(field, value, *, protocol):
    return {"match": {"op": "==", "left": {"payload": {
        "protocol": protocol, "field": field}}, "right": value}}


def _webdriver_rule(src, dst, sport, dport, *, handle=1, ack=True):
    flags = {"match": {"op": "==", "left": {"&": [
        {"payload": {"protocol": "tcp", "field": "flags"}}, {"|": ["syn", "ack"]}]},
        "right": "ack" if ack else "syn"}}
    return {"chain": "output", "handle": handle, "expr": [
        _nft_match("saddr", src, protocol="ip"),
        _nft_match("sport", sport, protocol="tcp"),
        _nft_match("daddr", dst, protocol="ip"),
        _nft_match("dport", dport, protocol="tcp"),
        flags, {"counter": {"packets": 7, "bytes": 512}}, {"accept": None}]}


def select_webdriver_nft_rules(nft_json, flow):
    """Return both exact directional rules and their packet counters or fail closed."""
    if not isinstance(nft_json, dict) or not isinstance(nft_json.get("nftables"), list):
        raise ValueError("malformed nft JSON ruleset")
    family = int(flow["family"])
    family_protocol = "ip" if family == 4 else "ip6" if family == 6 else None
    if family_protocol is None:
        raise ValueError("invalid WebDriver address family")
    client, driver = flow["client_addr"], flow["driver_addr"]
    client_port, driver_port = int(flow["client_port"]), int(flow["driver_port"])
    expected = {
        "client_to_driver": {("saddr", family_protocol): client,
                             ("sport", "tcp"): client_port,
                             ("daddr", family_protocol): driver,
                             ("dport", "tcp"): driver_port},
        "driver_to_client": {("saddr", family_protocol): driver,
                             ("sport", "tcp"): driver_port,
                             ("daddr", family_protocol): client,
                             ("dport", "tcp"): client_port},
    }

    def payload_match(expr):
        match = expr.get("match") if isinstance(expr, dict) else None
        if not isinstance(match, dict) or match.get("op") != "==":
            return None
        left = match.get("left")
        if not isinstance(left, dict) or not isinstance(left.get("payload"), dict):
            return None
        payload = left["payload"]
        protocol, field = payload.get("protocol"), payload.get("field")
        if protocol not in ("ip", "ip6", "tcp") or field not in ("saddr", "daddr", "sport", "dport"):
            return None
        return (field, protocol), match.get("right")

    candidates = []
    def contains_flags(node):
        if isinstance(node, dict):
            if node.get("payload") == {"protocol": "tcp", "field": "flags"}:
                return True
            return any(contains_flags(value) for value in node.values())
        if isinstance(node, list):
            return any(contains_flags(value) for value in node)
        return False

    for item in nft_json["nftables"]:
        rule = item.get("rule") if isinstance(item, dict) else None
        if not isinstance(rule, dict) or rule.get("chain") != "output":
            continue
        exprs = rule.get("expr")
        if not isinstance(exprs, list):
            continue
        if any(contains_flags(expr) for expr in exprs):
            candidates.append(rule)
    if len(candidates) != 2:
        raise ValueError(f"expected exactly two WebDriver exception rules, found {len(candidates)}")

    selected = {}
    for rule in candidates:
        exprs = rule["expr"]
        matches = {}
        flag_exprs = []
        for expr in exprs:
            parsed = payload_match(expr)
            if parsed is not None:
                key, value = parsed
                matches.setdefault(key, []).append(value)
            if isinstance(expr, dict) and isinstance(expr.get("match"), dict):
                left = expr["match"].get("left")
                if isinstance(left, dict) and left.get("&") is not None:
                    flag_exprs.append(expr["match"])
        scored = [(sum(matches.get(key) == [value] for key, value in tuple_fields.items()), name)
                  for name, tuple_fields in expected.items()]
        best = max(score for score, _name in scored)
        directions = [name for score, name in scored if score == best]
        if best == 0:
            raise ValueError("WebDriver exception rule lacks a directional tuple")
        if len(directions) != 1:
            raise ValueError("ambiguous WebDriver rule direction")
        direction = directions[0]
        for key, value in expected[direction].items():
            if matches.get(key) != [value]:
                raise ValueError(f"WebDriver {direction} tuple mismatch at {key}")
        tuple_keys = set(expected[direction])
        if any(key in matches for key in set().union(*(set(fields) for fields in expected.values())) - tuple_keys):
            raise ValueError("WebDriver rule contains mixed direction tuple fields")
        if len(flag_exprs) != 1 or flag_exprs[0] != {
                "op": "==", "left": {"&": [
                    {"payload": {"protocol": "tcp", "field": "flags"}},
                    {"|": ["syn", "ack"]}]}, "right": "ack"}:
            raise ValueError("WebDriver rule lacks exact ACK mask AST")
        counters = [expr["counter"] for expr in exprs
                    if isinstance(expr, dict) and isinstance(expr.get("counter"), dict)]
        handle = rule.get("handle")
        if type(handle) is not int or handle < 0:
            raise ValueError("WebDriver rule must have one integer handle")
        if len(counters) != 1 or type(counters[0].get("packets")) is not int or counters[0]["packets"] < 0:
            raise ValueError("WebDriver rule must have one valid packet counter")
        if sum(1 for expr in exprs if isinstance(expr, dict) and "accept" in expr) != 1:
            raise ValueError("WebDriver rule must accept exactly once")
        if direction in selected:
            raise ValueError(f"duplicate WebDriver {direction} rule")
        selected[direction] = {"handle": handle, "packets": counters[0]["packets"]}
    if set(selected) != set(expected):
        raise ValueError("WebDriver exception direction set is incomplete")
    if len({entry["handle"] for entry in selected.values()}) != 2:
        raise ValueError("WebDriver exception rules must have distinct handles")
    return selected


def same_port_probe_passed(evidence, *, flow, loopback_drop_delta, ack_counter_deltas):
    """Validate a successful old-port bind followed by denied SYN and exact counters."""
    if not isinstance(evidence, dict) or not isinstance(ack_counter_deltas, dict):
        return False
    expected_keys = {"client_to_driver", "driver_to_client"}
    errno_denial = (type(evidence.get("connect_errno")) is int and evidence["connect_errno"] > 0)
    bounded_timeout = (evidence.get("connect_errno") is None
                      and evidence.get("connect_error_type") in ("TimeoutError", "timeout")
                      and evidence.get("connect_timeout_ms") == 500
                      and type(evidence.get("elapsed_ms")) is int
                      and 0 < evidence["elapsed_ms"] <= 1000)
    return (evidence.get("bind_succeeded") is True
            and evidence.get("connect_attempted") is True
            and evidence.get("connected") is False
            and (errno_denial or bounded_timeout)
            and evidence.get("source_addr") == flow.get("client_addr")
            and evidence.get("source_port") == int(flow["client_port"])
            and evidence.get("destination_addr") == flow.get("driver_addr")
            and evidence.get("destination_port") == int(flow["driver_port"])
            and type(evidence.get("elapsed_ms")) is int
            and evidence["elapsed_ms"] >= 0
            and type(loopback_drop_delta) is int and loopback_drop_delta > 0
            and set(ack_counter_deltas) == expected_keys
            and all(type(value) is int and value == 0 for value in ack_counter_deltas.values()))


def write_failed_probe_receipt(path, *, command, returncode, stdout, stderr,
                               started_ns, ended_ns, error):
    if isinstance(stdout, bytes):
        stdout = stdout.decode("utf-8", errors="replace")
    if isinstance(stderr, bytes):
        stderr = stderr.decode("utf-8", errors="replace")
    receipt = {"command": command, "returncode": returncode, "stdout": stdout,
               "stderr": stderr, "monotonic_started_ns": started_ns,
               "monotonic_ended_ns": ended_ns, "error": error, "passed": False}
    pathlib.Path(path).write_text(json.dumps(receipt, indent=2, sort_keys=True) + "\n")
    return receipt


def same_port_probe_code(flow):
    return (
        "import json,socket,time;"
        f"e={{'source_addr':{flow['client_addr']!r},'source_port':{int(flow['client_port'])},"
        f"'destination_addr':{flow['driver_addr']!r},'destination_port':{int(flow['driver_port'])},"
        "'bind_succeeded':False,'connect_attempted':False,'connected':False,'connect_errno':None,"
        "'connect_timeout_ms':500,'elapsed_ms':0};"
        "s=socket.socket(socket.AF_INET,socket.SOCK_STREAM);s.settimeout(.5);\n"
        "try:\n s.bind((e['source_addr'],e['source_port']));e['bind_succeeded']=True\n"
        "except OSError as x:\n e['bind_errno']=x.errno\n"
        "if e['bind_succeeded']:\n"
        " t=time.monotonic();e['connect_attempted']=True\n"
        " try:\n  s.connect((e['destination_addr'],e['destination_port']));e['connected']=True\n"
        " except OSError as x:\n  e['connect_errno']=x.errno;e['connect_error_type']=type(x).__name__;e['connect_error']=str(x)\n"
        " finally:e['elapsed_ms']=int((time.monotonic()-t)*1000)\n"
        "s.close();print(json.dumps(e,sort_keys=True))"
    )


class PolicyTargetCounterUnitTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        spec = importlib.util.spec_from_file_location("t14_policy_helper_under_test", HERE / "helper.py")
        cls.module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(cls.module)

    def counter_fixture(self, omit=None):
        comments = ["policy-loopback-v4-drop", "policy-loopback-v6-drop",
                    "policy-isolated-ipv4-drop", "policy-isolated-ipv6-drop"]
        comments.extend(target["comment"] for target in self.module.FIXED_TARGETS.values())
        return {"nftables": [
            {"rule": {"chain": "output", "comment": comment,
                       "expr": [{"counter": {"packets": index + 1, "bytes": (index + 1) * 64}}]}}
            for index, comment in enumerate(comments) if comment != omit
        ]}

    def _attr(self, kind, value):
        length = 4 + len(value)
        return struct.pack("=HH", length, kind) + value + (b"\0" * ((-length) & 3))

    def _tuple(self, family, src, dst, sport, dport, protocol=6):
        if family == 4:
            ip = self._attr(1, socket.inet_pton(socket.AF_INET, src)) + self._attr(2, socket.inet_pton(socket.AF_INET, dst))
        else:
            ip = self._attr(3, socket.inet_pton(socket.AF_INET6, src)) + self._attr(4, socket.inet_pton(socket.AF_INET6, dst))
        proto = (self._attr(1, bytes([protocol])) + self._attr(2, struct.pack("!H", sport))
                 + self._attr(3, struct.pack("!H", dport)))
        return self._attr(1, ip) + self._attr(2, proto)

    def sock_diag_test_dump(self, flow, sequence, *, local_port_id=321, family=None,
                            client_addr=None, driver_addr=None, client_port=None,
                            driver_port=None, state=1, inode=None, message_pid=None,
                            message_sequence=None, sender_pid=0, flags=2):
        family = flow["family"] if family is None else family
        family_code = socket.AF_INET if family == 4 else socket.AF_INET6
        client_addr = flow["client_addr"] if client_addr is None else client_addr
        driver_addr = flow["driver_addr"] if driver_addr is None else driver_addr
        client_port = flow["client_port"] if client_port is None else client_port
        driver_port = flow["driver_port"] if driver_port is None else driver_port
        inode = int(flow["client_socket_inode"]) if inode is None else inode
        message_pid = local_port_id if message_pid is None else message_pid
        message_sequence = sequence if message_sequence is None else message_sequence

        def address_bytes(value):
            packed = socket.inet_pton(socket.AF_INET if family == 4 else socket.AF_INET6, value)
            return packed + (b"\0" * 12 if family == 4 else b"")

        payload = (struct.pack("=BBBB", family_code, state, 0, 0)
                   + struct.pack("!HH", client_port, driver_port)
                   + address_bytes(client_addr) + address_bytes(driver_addr)
                   + struct.pack("=I2I5I", 0, 0x80000005, 0, 0, 0, 0, 0, inode)
                   + self._attr(8, b"\0"))
        record = (struct.pack("=IHHII", 16 + len(payload), 20, flags,
                              message_sequence, message_pid) + payload)
        done = struct.pack("=IHHIIi", 20, 3, 2, sequence, message_pid, 0)
        return [{"data": record, "sender_pid": sender_pid, "sequence": sequence},
                {"data": done, "sender_pid": sender_pid, "sequence": sequence}]

    def sock_diag_flow(self, family=4):
        if family == 4:
            client, driver = "127.0.0.1", "127.0.0.1"
        else:
            client, driver = "::1", "::1"
        return {"family": family, "client_addr": client, "client_port": 49152,
                "driver_addr": driver, "driver_port": 9515,
                "state": "ESTABLISHED", "client_socket_inode": "5717721"}

    def test_sock_diag_parser_attests_exact_ipv4_and_ipv6_client_records(self):
        for family in (4, 6):
            with self.subTest(family=family):
                flow = self.sock_diag_flow(family)
                messages = self.sock_diag_test_dump(flow, sequence=44)
                proof = self.module.parse_sock_diag_dump(
                    flow, messages, sequence=44, local_port_id=321)
                self.assertEqual(proof["tuple"]["sport"], "49152")
                self.assertEqual(proof["state"], "ESTABLISHED")
                self.assertEqual(proof["inode"], "5717721")
                self.assertEqual(proof["cookie"], [0x80000005, 0])

    def test_sock_diag_parser_normalizes_expanded_ipv6_loopback_before_exact_match(self):
        flow = self.sock_diag_flow(6)
        flow["client_addr"] = "0:0:0:0:0:0:0:1"
        flow["driver_addr"] = "0:0:0:0:0:0:0:1"
        messages = self.sock_diag_test_dump(flow, sequence=47,
                                            client_addr="::1", driver_addr="::1")

        proof = self.module.parse_sock_diag_dump(
            flow, messages, sequence=47, local_port_id=321)

        self.assertEqual(proof["tuple"], {
            "src": "::1", "dst": "::1", "sport": "49152", "dport": "9515",
        })

    def test_sock_diag_parser_rejects_bad_or_incomplete_dump_evidence(self):
        flow = self.sock_diag_flow()
        valid = self.sock_diag_test_dump(flow, sequence=45)
        wrong_family = self.sock_diag_test_dump(flow, sequence=45, family=6,
                                                client_addr="::1", driver_addr="::1")
        wrong_state = self.sock_diag_test_dump(flow, sequence=45, state=10)
        wrong_tuple = self.sock_diag_test_dump(flow, sequence=45, client_port=49153)
        wrong_inode = self.sock_diag_test_dump(flow, sequence=45, inode=123)
        wrong_port_id = self.sock_diag_test_dump(flow, sequence=45, message_pid=322)
        wrong_sequence = self.sock_diag_test_dump(flow, sequence=45, message_sequence=46)
        wrong_outer_sequence = [dict(valid[0], sequence=46), valid[1]]
        wrong_type_header = list(struct.unpack_from("=IHHII", valid[0]["data"]))
        wrong_type_header[1] = 21
        wrong_type = [dict(valid[0], data=struct.pack("=IHHII", *wrong_type_header)
                            + valid[0]["data"][16:]), valid[1]]
        bad_attribute = [dict(valid[0], data=(valid[0]["data"][:-8]
                                              + struct.pack("=HH", 0, 1) + b"\0" * 4)), valid[1]]
        bad_done = [valid[0], dict(valid[1], data=struct.pack("=IHHIIi", 20, 3, 2, 45, 321, -1))]
        malformed = dict(valid[0], data=valid[0]["data"][:15])
        interrupted_header = list(struct.unpack_from("=IHHII", valid[0]["data"]))
        interrupted_header[2] |= 0x10
        interrupted = dict(valid[0], data=struct.pack("=IHHII", *interrupted_header)
                           + valid[0]["data"][16:])
        error = {"data": struct.pack("=IHHIIi", 20, 2, 0, 45, 321, -2),
                 "sender_pid": 0, "sequence": 45}
        for label, messages in (
                ("wrong source pid", [dict(valid[0], sender_pid=9), valid[1]]),
                ("wrong sequence", wrong_sequence),
                ("wrong outer sequence", wrong_outer_sequence),
                ("wrong type", wrong_type),
                ("wrong header port id", wrong_port_id),
                ("wrong family", wrong_family),
                ("wrong state", wrong_state),
                ("wrong tuple", wrong_tuple),
                ("wrong inode", wrong_inode),
                ("malformed framing", [malformed, valid[1]]),
                ("malformed attribute", bad_attribute),
                ("nonzero completion", bad_done),
                ("interrupted", [interrupted, valid[1]]),
                ("netlink error", [error, valid[1]]),
                ("missing completion", valid[:1]),
                ("missing exact socket", valid[1:]),
                ("ambiguous exact tuple", valid[:1] + valid[:1] + valid[1:]),
                ("truncated", [dict(valid[0], truncated=True), valid[1]]),
                ("datagram limit", [dict(valid[0], datagram_bytes=65537), valid[1]]),
                ("total bytes limit", [dict(valid[0], aggregate_bytes=1024 * 1024 + 1), valid[1]]),
                ("message limit", [dict(valid[1], data=b"")] * 8193),
        ):
            with self.subTest(label=label):
                with self.assertRaises(RuntimeError):
                    self.module.parse_sock_diag_dump(
                        flow, messages, sequence=45, local_port_id=321, now=lambda: 0)

    def test_sock_diag_query_uses_tcp_dump_and_closes_socket(self):
        flow = self.sock_diag_flow()
        query = {"responses": ["success"]}
        self_test = self

        class FakeSocket:
            def __init__(self, scenario="success"):
                self.scenario = scenario
                self.closed = False
                self.calls = 0

            def bind(self, address):
                self.bound = address

            def getsockname(self):
                if getattr(self, "bound", None) != (0, 0):
                    raise AssertionError("getsockname must follow bind")
                return (321, 0)

            def settimeout(self, timeout):
                self.timeout = timeout

            def sendto(self, data, address):
                query["request"] = data
                query["destination"] = address

            def recvmsg(self, max_size):
                if self.scenario == "timeout":
                    raise socket.timeout()
                header = struct.unpack_from("=IHHII", query["request"])
                sequence = header[3]
                frames = self_test.sock_diag_test_dump(flow, sequence)
                self.calls += 1
                return frames[self.calls - 1]["data"], [], 0, (0, 0)

            def close(self):
                self.closed = True

        fake_socket = FakeSocket()
        with mock.patch.object(self.module.socket, "AF_NETLINK", 16, create=True), \
                mock.patch.object(self.module.socket, "socket", return_value=fake_socket) as create_socket:
            proof = self.module.require_attested_webdriver_socket(flow)
        create_socket.assert_called_once_with(16, socket.SOCK_DGRAM, 4)
        self.assertEqual(query["destination"], (0, 0))
        request = query["request"]
        length, msg_type, flags, sequence, pid = struct.unpack_from("=IHHII", request)
        self.assertEqual(length, 72)
        self.assertEqual(msg_type, 20)
        self.assertEqual(flags, 1 | 0x300)
        self.assertGreater(sequence, 0)
        self.assertEqual(pid, 0)
        self.assertEqual(struct.unpack_from("=BBBBI", request, 16), (2, 6, 0, 0, 0xFFFFFFFF))
        self.assertEqual(proof["inode"], flow["client_socket_inode"])
        self.assertTrue(fake_socket.closed)

        timed_out = FakeSocket("timeout")
        with mock.patch.object(self.module.socket, "AF_NETLINK", 16, create=True), \
                mock.patch.object(self.module.socket, "socket", return_value=timed_out):
            with self.assertRaisesRegex(RuntimeError, "timed out"):
                self.module.require_attested_webdriver_socket(flow)
        self.assertTrue(timed_out.closed)

    def test_webdriver_flow_rules_are_exact_ack_guarded_tuples(self):
        for family in (4, 6):
            with self.subTest(family=family):
                flow = self.sock_diag_flow(family)
                rules = self.module.webdriver_rules(flow)
                prefix = "ip" if family == 4 else "ip6"
                self.assertEqual(rules, [
                    f"  {prefix} saddr {flow['client_addr']} tcp sport 49152 "
                    f"{prefix} daddr {flow['driver_addr']} tcp dport 9515 "
                    "tcp flags & (syn | ack) == ack counter accept",
                    f"  {prefix} saddr {flow['driver_addr']} tcp sport 9515 "
                    f"{prefix} daddr {flow['client_addr']} tcp dport 49152 "
                    "tcp flags & (syn | ack) == ack counter accept",
                ])
                self.assertTrue(all("ct state" not in rule for rule in rules))

    def test_nft_selector_requires_two_exact_directional_ack_rules_and_counters(self):
        flow = self.sock_diag_flow()
        forward = _webdriver_rule("127.0.0.1", "127.0.0.1", 49152, 9515, handle=101)
        reverse = _webdriver_rule("127.0.0.1", "127.0.0.1", 9515, 49152, handle=102)
        ruleset = {"nftables": [{"rule": forward}, {"rule": reverse}]}

        selected = select_webdriver_nft_rules(ruleset, flow)

        self.assertEqual(selected, {"client_to_driver": {"handle": 101, "packets": 7},
                                    "driver_to_client": {"handle": 102, "packets": 7}})
        invalid_sets = (
            ("missing direction", {"nftables": [{"rule": forward}]}),
            ("duplicate direction", {"nftables": [{"rule": forward}, {"rule": forward}, {"rule": reverse}]}),
            ("broad source", {"nftables": [{"rule": _webdriver_rule("127.0.0.0/8", "127.0.0.1", 49152, 9515)},
                                           {"rule": reverse}]}),
            ("missing counter", {"nftables": [{"rule": {**forward, "expr": forward["expr"][:-3] + [{"accept": None}]}},
                                               {"rule": reverse}]}),
            ("malformed counter", {"nftables": [{"rule": {**forward, "expr": [
                *forward["expr"][:-2], {"counter": {"packets": "7"}}, forward["expr"][-1]]}},
                                                {"rule": reverse}]}),
            ("missing handle", {"nftables": [{"rule": {k: v for k, v in forward.items() if k != "handle"}},
                                               {"rule": reverse}]}),
            ("wrong ACK AST", {"nftables": [{"rule": _webdriver_rule("127.0.0.1", "127.0.0.1", 49152, 9515, ack=False)},
                                             {"rule": reverse}]}),
            ("malformed ruleset", {"rules": []}),
            ("duplicate handles", {"nftables": [{"rule": forward},
                                                  {"rule": _webdriver_rule("127.0.0.1", "127.0.0.1", 9515, 49152, handle=101)}]}),
        )
        for label, invalid in invalid_sets:
            with self.subTest(label=label), self.assertRaises((AssertionError, RuntimeError, ValueError)):
                select_webdriver_nft_rules(invalid, flow)

    def test_same_port_probe_validator_requires_bind_denial_and_counter_isolation(self):
        flow = self.sock_diag_flow()
        compile(same_port_probe_code(flow), "same_port_probe", "exec")
        evidence = {"bind_succeeded": True, "connect_attempted": True,
                    "connected": False, "connect_errno": 111,
                    "source_addr": "127.0.0.1", "source_port": 49152,
                    "destination_addr": "127.0.0.1", "destination_port": 9515,
                    "elapsed_ms": 12}
        self.assertTrue(same_port_probe_passed(
            evidence, flow=flow, loopback_drop_delta=1,
            ack_counter_deltas={"client_to_driver": 0, "driver_to_client": 0}))
        for changes in (
                {"bind_succeeded": False}, {"connect_attempted": False},
                {"connected": True}, {"source_port": 49153},
                {"destination_port": 80}, {"connect_errno": None}):
            with self.subTest(changes=changes):
                self.assertFalse(same_port_probe_passed(
                    {**evidence, **changes}, flow=flow, loopback_drop_delta=1,
                    ack_counter_deltas={"client_to_driver": 0, "driver_to_client": 0}))
        self.assertFalse(same_port_probe_passed(
            evidence, flow=flow, loopback_drop_delta=0,
            ack_counter_deltas={"client_to_driver": 0, "driver_to_client": 0}))
        self.assertFalse(same_port_probe_passed(
            evidence, flow=flow, loopback_drop_delta=1,
            ack_counter_deltas={"client_to_driver": 1, "driver_to_client": 0}))

    def test_same_port_probe_accepts_only_bounded_errno_or_timeout_denial(self):
        flow = self.sock_diag_flow()
        base = {"bind_succeeded": True, "connect_attempted": True, "connected": False,
                "source_addr": "127.0.0.1", "source_port": 49152,
                "destination_addr": "127.0.0.1", "destination_port": 9515,
                "elapsed_ms": 500}
        kwargs = {"flow": flow, "loopback_drop_delta": 1,
                  "ack_counter_deltas": {"client_to_driver": 0, "driver_to_client": 0}}
        timeout = {**base, "connect_errno": None, "connect_timeout_ms": 500,
                   "connect_error_type": "TimeoutError",
                   "connect_error": "timed out"}
        self.assertTrue(same_port_probe_passed(timeout, **kwargs))
        self.assertTrue(same_port_probe_passed(
            {**base, "connect_errno": 111, "connect_error_type": "ConnectionRefusedError"}, **kwargs))
        invalid = (
            ({**timeout, "elapsed_ms": 1001}, kwargs),
            (timeout, {**kwargs, "loopback_drop_delta": 0}),
            ({**timeout, "connected": True}, kwargs),
            ({**timeout, "connect_error_type": "OSError"}, kwargs),
            ({**timeout, "source_port": 49153}, kwargs),
            ({**timeout, "destination_addr": "127.0.0.2"}, kwargs),
            (timeout, {**kwargs, "ack_counter_deltas": {"client_to_driver": 1, "driver_to_client": 0}}),
        )
        for evidence, invalid_kwargs in invalid:
            with self.subTest(evidence=evidence, kwargs=invalid_kwargs):
                self.assertFalse(same_port_probe_passed(evidence, **invalid_kwargs))

    def test_failed_subprocess_and_invalid_json_receipts_are_durable(self):
        with tempfile.TemporaryDirectory() as directory:
            receipt_path = pathlib.Path(directory) / "same-port-syn-proof.json"
            receipt = write_failed_probe_receipt(
                receipt_path, command=["docker", "exec", "probe"], returncode=2,
                stdout="partial", stderr="failed", started_ns=10, ended_ns=20,
                error="subprocess returned nonzero")
            self.assertFalse(receipt["passed"])
            self.assertEqual(json.loads(receipt_path.read_text()), receipt)
            receipt = write_failed_probe_receipt(
                receipt_path, command=["docker", "exec", "probe"], returncode=0,
                stdout="{bad", stderr="", started_ns=30, ended_ns=40,
                error="invalid JSON: Expecting property name")
            self.assertFalse(json.loads(receipt_path.read_text())["passed"])

    def test_counters_keep_each_fixed_target_distinct(self):
        result = self.module.counters(self.counter_fixture())
        for index, (name, target) in enumerate(self.module.FIXED_TARGETS.items(), start=4):
            metric = result["target_counters"][name]
            self.assertEqual(metric["packets"], index + 1)
            self.assertEqual(metric["counter_id"], target["comment"])
            self.assertEqual(metric["destination"], target["destination"])
            self.assertEqual(metric["protocol"], target["protocol"])
            self.assertEqual(metric["destination_ports"], target["ports"])
        self.assertEqual(result["loopback_drop"]["packets"], 3)

    def test_missing_fixed_target_counter_fails_closed(self):
        with self.assertRaisesRegex(RuntimeError, "counter set is incomplete"):
            self.module.counters(self.counter_fixture("policy-target-gateway-http-drop"))

    def test_public_counter_destination_matches_fixed_process_probe(self):
        self.assertEqual(self.module.FIXED_TARGETS["public_https"]["destination"], "1.1.1.1")



class PolicyControllerStopFailureTests(unittest.TestCase):
    def make_controller(self, stop_result=None, stop_error=None):
        spec = importlib.util.spec_from_file_location("t14_policy_controller_under_test", CONTROLLER)
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        controller = module.Controller.__new__(module.Controller)
        controller.active_helper_name = "t14ph-active"
        controller.last_helper_command = ["docker", "run", "t14ph-active"]
        controller.last_helper_meta = {}
        controller.cleanup_actions = []
        controller.stop_calls = 0
        def stop_active_helper():
            controller.stop_calls += 1
            if controller.stop_calls == 1 and stop_error is not None:
                raise stop_error
            if controller.stop_calls == 1:
                return stop_result
            return {"attempted": True, "confirmed_removed": True}
        controller.stop_active_helper = stop_active_helper
        def helper(_container, action, *_args, **_kwargs):
            controller.cleanup_actions.append(action)
            return ({"netns": "net:[42]"}, {"container": {"Id": "a" * 64},
                    "container_pid": 123, "container_image_id": "sha256:renderer",
                    "helper_image_ref": "policy-helper", "helper_image_id": "sha256:helper",
                    "container_inspect_raw": "{}", "container_inspect_after_raw": "{}"})
        controller.helper = helper
        controller.write_capture = lambda *_args: {"sequence": 1}
        controller.destroy_renderer = lambda _container: {"confirmed_destroyed": True, "argv": ["docker", "rm", "-f"]}
        return controller

    def test_unconfirmed_renderer_absence_proof_stays_incomplete(self):
        controller = self.make_controller(
            {"attempted": True, "confirmed_removed": False, "returncode": 1}, None)
        controller.destroy_renderer = lambda _container: {
            "confirmed_destroyed": False, "error": {"type": "TimeoutExpired", "message": "docker rm timeout"},
            "renderer_id": "a" * 64}
        outcome = controller.install_failure_cleanup(
            "a" * 64, "t14_policy", "net:[42]", 18081, RuntimeError("install interrupted"))
        self.assertEqual(outcome["cleanup_status"], "incomplete")
        self.assertFalse(outcome["destroy"]["confirmed_destroyed"])
        self.assertEqual(controller.cleanup_actions, [])

    def test_unconfirmed_helper_stop_never_claims_clean_namespace(self):
        cases = (
            ("exception", None, OSError("injected Docker client failure")),
            ("timeout", None, subprocess.TimeoutExpired(["docker", "rm", "-f"], 10)),
            ("false", {"attempted": True, "confirmed_removed": False, "returncode": 1}, None),
        )
        for name, stop_result, stop_error in cases:
            with self.subTest(name=name):
                controller = self.make_controller(stop_result, stop_error)
                outcome = controller.install_failure_cleanup(
                    "a" * 64, "t14_policy", "net:[42]", 18081, RuntimeError("install interrupted"))
                self.assertEqual(outcome["cleanup_status"], "renderer-destroyed")
                self.assertTrue(outcome["destroy"]["confirmed_destroyed"])
                self.assertEqual(controller.cleanup_actions, [], "must not run cleanup or baseline while helper may still mutate")
                self.assertFalse(outcome.get("baseline_record"))
                self.assertTrue(outcome["helper_stop"].get("error") or
                                outcome["helper_stop"].get("confirmed_removed") is False)


class PolicyControllerBoundedEvidenceTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        spec = importlib.util.spec_from_file_location("t14_controller_bounded", CONTROLLER)
        cls.module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(cls.module)

    def test_discovery_selects_normalized_kernel_states(self):
        script = self.module.WEBDRIVER_DISCOVERY
        self.assertIn("x['state']=='ESTABLISHED'", script)
        self.assertIn("x['state']=='LISTEN'", script)
        self.assertNotIn("x['state']=='01'", script)
        self.assertNotIn("x['state']=='0A'", script)

    def test_child_output_is_capped_and_process_is_terminated(self):
        with self.assertRaisesRegex(RuntimeError, "bounded capture limit"):
            self.module.run(["python3", "-c", "import os;os.write(1,b'x'*1000000)"], output_limit=1024)

    def test_child_timeout_is_enforced(self):
        with self.assertRaises(subprocess.TimeoutExpired):
            self.module.run(["python3", "-c", "import time;time.sleep(10)"], timeout=.05)

    def test_generated_same_listener_canary_compiles_for_ipv4_and_ipv6(self):
        for family, address in ((4, "127.0.0.1"), (6, "::1")):
            with self.subTest(family=family):
                code = self.module.same_listener_canary_code(family, address, 9515, 49152)
                compile(code, f"same-listener-ipv{family}", "exec")

    def test_docker_marker_callbacks_receive_remaining_budget_and_reject_late_result(self):
        now = [10.0]
        observed_timeouts = []

        def slow_runner(*args, timeout, **kwargs):
            observed_timeouts.append(timeout)
            now[0] += timeout + .001
            return subprocess.CompletedProcess(args, 0, stdout="reuse-ok", stderr="")

        with self.assertRaisesRegex(TimeoutError, "after marker deadline"):
            run_before_deadline(10.25, "docker", "exec", runner=slow_runner, now=lambda: now[0])
        self.assertEqual(observed_timeouts, [.25])
        with self.assertRaisesRegex(TimeoutError, "before subprocess start"):
            run_before_deadline(now[0], "docker", "inspect", runner=slow_runner, now=lambda: now[0])

        callback_clock = [20.0]
        with self.assertRaisesRegex(TimeoutError, "before accepting marker"):
            wait_for_reuse_marker_bounded(
                lambda: (callback_clock.__setitem__(0, 20.251) or b"reuse-ok"),
                lambda: True, deadline=20.25, now=lambda: callback_clock[0], sleep=lambda _: None)

    def test_reuse_marker_wait_is_bounded_detects_exit_and_requires_exact_bytes(self):
        reads = iter((None, None, b"reuse-ok\n"))
        self.assertEqual(self.module.wait_for_reuse_marker(
            lambda: next(reads), lambda: True, expected=b"reuse-ok\n", timeout=.2, interval=.001),
            b"reuse-ok\n")
        with self.assertRaisesRegex(RuntimeError, "renderer exited"):
            self.module.wait_for_reuse_marker(lambda: None, lambda: False,
                                              expected=b"reuse-ok\n", timeout=.2, interval=.001)
        with self.assertRaisesRegex(RuntimeError, "marker contents mismatch"):
            self.module.wait_for_reuse_marker(lambda: b"reuse-bad", lambda: True,
                                              expected=b"reuse-ok\n", timeout=.2, interval=.001)
        with self.assertRaisesRegex(TimeoutError, "deadline"):
            self.module.wait_for_reuse_marker(lambda: None, lambda: True,
                                              expected=b"reuse-ok\n", timeout=.005, interval=.001)

    def test_same_listener_canary_requires_actual_distinct_failed_connect_and_counter_delta(self):
        valid = {"destination": "127.0.0.1", "destination_port": 9515, "source_port": 43000,
                 "connect_attempted": True, "connected": False, "error_type": "TimeoutError",
                 "connect_elapsed_ms": 500}
        kwargs = {"address": "127.0.0.1", "port": 9515, "source_port": 49152,
                  "returncode": 1, "delta_packets": 1}
        self.assertTrue(self.module.same_listener_canary_passed(valid, **kwargs))
        invalid = (
            None,
            {**valid, "destination_port": 9516},
            {**valid, "source_port": 49152},
            {**valid, "connect_attempted": False},
            {**valid, "connected": True},
            {**valid, "error_type": "ConnectionRefusedError"},
            {**valid, "connect_elapsed_ms": 20},
        )
        for evidence in invalid:
            with self.subTest(evidence=evidence):
                self.assertFalse(self.module.same_listener_canary_passed(evidence, **kwargs))
        self.assertFalse(self.module.same_listener_canary_passed(valid, **{**kwargs, "delta_packets": 0}))
        self.assertFalse(self.module.same_listener_canary_passed(valid, **{**kwargs, "returncode": 2}))


@unittest.skipUnless(os.environ.get("POLICY_HELPER_RUN_DOCKER_TESTS") == "1", "set POLICY_HELPER_RUN_DOCKER_TESTS=1 for disposable Docker proof")
class PolicyHelperInvocationUnitTests(unittest.TestCase):
    def test_install_helper_argv_carries_only_the_exact_host_flow(self):
        spec = importlib.util.spec_from_file_location("t14_policy_controller_argv", CONTROLLER)
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        controller = module.Controller.__new__(module.Controller)
        controller.image = "helper:test"
        controller.active_helper_name = ""
        controller.last_helper_command = []
        controller.last_helper_meta = {}
        data = {"Id": "a" * 64, "Image": "sha256:renderer", "State": {"Pid": 77}}
        controller.inspect = lambda _container: ("{}", data)
        flow = {"family": 4, "client_addr": "127.0.0.1", "client_port": 49152,
                "driver_addr": "127.0.0.1", "driver_port": 9515,
                "state": "ESTABLISHED", "client_socket_inode": "12345"}
        calls = []
        def fake_run(argv, **_kwargs):
            calls.append(argv)
            if argv[:3] == ["docker", "image", "inspect"]:
                payload = [{"Id": "sha256:helper", "Os": "linux", "Architecture": "arm64"}]
                return subprocess.CompletedProcess(argv, 0, json.dumps(payload), "")
            return subprocess.CompletedProcess(argv, 0, "{}", "")
        with patch.object(module, "run", side_effect=fake_run):
            controller.helper("a" * 64, "install", "t14_policy", "net:[42]", 18081, flow)
        argv = calls[-1]
        self.assertIn("--webdriver-flow-json", argv)
        value = json.loads(argv[argv.index("--webdriver-flow-json") + 1])
        self.assertEqual(value, flow)
        self.assertNotIn("--webdriver-port", argv)
        self.assertNotIn("--allow-established", argv)


class PolicyControllerIntegrationTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.build = run("docker", "build", "--platform", "linux/arm64", "-t", HELPER_IMAGE, str(HERE), timeout=180)
        cls.image = json.loads(run("docker", "image", "inspect", HELPER_IMAGE, "--format", "{{json .}}").stdout)
        if cls.image["Architecture"] != "arm64" or cls.image["Os"] != "linux":
            raise AssertionError(f"unexpected helper platform: {cls.image['Os']}/{cls.image['Architecture']}")
        evidence = os.environ.get("POLICY_HELPER_EVIDENCE_DIR")
        if evidence:
            destination = pathlib.Path(evidence).resolve()
            destination.mkdir(parents=True, exist_ok=True)
            (destination / "helper-image-inspect.json").write_text(json.dumps(cls.image, indent=2, sort_keys=True) + "\n")
            packages = run("docker", "run", "--rm", "--network", "none", "--cap-drop", "ALL", "--read-only",
                           "--entrypoint", "cat", HELPER_IMAGE, "/usr/share/policy-helper-package-provenance.txt")
            (destination / "helper-package-provenance.txt").write_text(packages.stdout)

    def setUp(self):
        self.container = ""
        self.container_pid = None
        self.netns_inode = ""
        self.policy_id = "t14_" + uuid.uuid4().hex[:12]
        retained = os.environ.get("POLICY_HELPER_EVIDENCE_DIR")
        if retained:
            self.evidence = pathlib.Path(retained).resolve() / self._testMethodName
            shutil.rmtree(self.evidence, ignore_errors=True)
            self.evidence.mkdir(parents=True, mode=0o700)
            self.tmp = None
        else:
            self.tmp = tempfile.TemporaryDirectory(prefix="t14-policy-evidence-")
            self.evidence = pathlib.Path(self.tmp.name)

    def tearDown(self):
        if self.container:
            run("docker", "rm", "-f", self.container, check=False)
        if self.tmp:
            self.tmp.cleanup()

    def start_renderer(self, preview_port=18081):
        self.preview_port = preview_port
        command = (
            "import ctypes,os,socket; pid=os.fork()\n"
            "if pid==0:\n"
            " libc=ctypes.CDLL(None); libc.prctl(15,b'chromedriver',0,0,0); "
            " d=socket.socket(); d.setsockopt(socket.SOL_SOCKET,socket.SO_REUSEADDR,1); "
            " d.bind(('127.0.0.1',9515)); d.listen(); exec('while True:\\n c,a=d.accept(); data=c.recv(32); c.sendall(b\\\"reuse-ok\\\"); c.close()'); os._exit(0)\n"
            "w=socket.create_connection(('127.0.0.1',9515)); "
            "import signal\n"
            "def reuse(signum,frame):\n w.sendall(b'reuse'); result=w.recv(32); open('/tmp/reuse-proof','wb').write(result); signal.signal(signal.SIGUSR1,signal.SIG_IGN)\n"
            "def close_client(signum,frame):\n w.close(); open('/tmp/close-proof','wb').write(b'client-fd-closed'); signal.signal(signal.SIGUSR2,signal.SIG_IGN)\n"
            "signal.signal(signal.SIGUSR1,reuse); signal.signal(signal.SIGUSR2,close_client); "
            "s=socket.socket(); s.setsockopt(socket.SOL_SOCKET,socket.SO_REUSEADDR,1); "
            f"s.bind(('127.0.0.1',{preview_port})); s.listen(); "
            "exec('while True:\\n c,a=s.accept(); c.sendall(b\\\"preview-ok\\\"); c.close()')"
        )
        self.container = run(
            "docker", "run", "-d", "--name", self.policy_id,
            "--network", "none", "--read-only", "--user", "10001:10001",
            "--cap-drop", "ALL", "--security-opt", "no-new-privileges",
            "--tmpfs", "/tmp:rw,noexec,nosuid,size=16m",
            "--entrypoint", "python3", RENDERER_IMAGE, "-c", command,
        ).stdout.strip()
        for _ in range(50):
            status = run("docker", "inspect", "--format", "{{.State.Running}}", self.container).stdout.strip()
            if status == "true":
                break
            time.sleep(0.05)
        inspected = json.loads(run("docker", "inspect", self.container).stdout)[0]
        self.assertEqual(inspected["Config"]["User"], "10001:10001")
        self.assertTrue(inspected["HostConfig"]["ReadonlyRootfs"])
        self.assertEqual(inspected["HostConfig"]["CapDrop"], ["ALL"])
        self.assertEqual(inspected["HostConfig"]["NetworkMode"], "none")
        self.assertIn("no-new-privileges", inspected["HostConfig"]["SecurityOpt"])
        self.container_pid = inspected["State"]["Pid"]
        controller_spec = importlib.util.spec_from_file_location("t14_controller_discovery_test", CONTROLLER)
        controller_module = importlib.util.module_from_spec(controller_spec)
        controller_spec.loader.exec_module(controller_module)
        inventory = run("docker", "exec", "--user", "10001:10001", self.container,
                        "python3", "-c", controller_module.WEBDRIVER_DISCOVERY)
        parsed_inventory = json.loads(inventory.stdout)
        self.assertEqual(len(parsed_inventory["clients"]), 1, inventory.stdout)
        self.assertEqual(len(parsed_inventory["listeners"]), 1, inventory.stdout)
        try:
            controller_module._FLOW_MODULE.select_webdriver_flow(parsed_inventory["clients"], parsed_inventory["listeners"])
        except ValueError as exc:
            self.fail(f"discovered socket records do not join: {exc}; inventory={parsed_inventory}")
        self.netns_inode = run("docker", "exec", "--user", "10001:10001", self.container,
                               "readlink", "/proc/self/ns/net").stdout.strip()
        self.assertRegex(self.netns_inode, r"^net:\[\d+\]$")
        for _ in range(30):
            ready = run("docker", "exec", self.container, "python3", "-c",
                        f"import socket;s=socket.create_connection(('127.0.0.1',{preview_port}),.2);print(s.recv(32).decode())", check=False)
            if ready.returncode == 0 and ready.stdout.strip() == "preview-ok":
                return self.container_pid
            time.sleep(.05)
        raise AssertionError(f"preview fixture failed to listen: {run('docker','logs',self.container,check=False).stderr}")

    def ctl(self, *args, check=True):
        if "--expected-netns" not in args and self.netns_inode:
            args = (*args, "--expected-netns", self.netns_inode)
        result = run("python3", str(CONTROLLER), "--helper-image", HELPER_IMAGE,
                     "--evidence-dir", str(self.evidence), *args, check=False, timeout=45)
        if check and result.returncode:
            raise AssertionError(f"controller failed: {result.stderr.strip()} {result.stdout.strip()}")
        return result

    def interrupt_after_route(self, fail_cleanup=False):
        command = ["python3", str(CONTROLLER), "--helper-image", HELPER_IMAGE,
                   "--evidence-dir", str(self.evidence), "install", "--container", self.container,
                   "--policy-id", self.policy_id, "--preview-port", "18081",
                   "--expected-netns", self.netns_inode]
        env = dict(os.environ, T14_POLICY_HELPER_TEST_PAUSE_AFTER_ROUTE="1")
        if fail_cleanup:
            env["T14_POLICY_HELPER_TEST_FAIL_CLEANUP"] = "1"
        process = subprocess.Popen(command, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, env=env)
        helper_id = ""
        deadline = time.monotonic() + 15
        while time.monotonic() < deadline and process.poll() is None:
            containers = run("docker", "ps", "-q", "--filter", f"ancestor={HELPER_IMAGE}").stdout.split()
            for candidate in containers:
                info_result = run("docker", "inspect", candidate, check=False)
                if info_result.returncode:
                    continue
                info = json.loads(info_result.stdout)[0]
                if info["Config"].get("Cmd", [None])[0] != "install":
                    continue
                route_result = run("docker", "exec", candidate, "ip", "-j", "-4", "route", "show", check=False)
                if route_result.returncode:
                    continue
                routes = json.loads(route_result.stdout)
                if any(route.get("dst") == "198.18.10.0/24" for route in routes):
                    killed = run("docker", "kill", info["Id"], check=False)
                    if killed.returncode == 0:
                        helper_id = info["Id"]
                    break
            if not helper_id:
                time.sleep(.03)
        stdout, stderr = process.communicate(timeout=50)
        return helper_id, process.returncode, stdout, stderr

    def test_install_requires_an_explicit_independent_inode(self):
        self.start_renderer()
        result = run("python3", str(CONTROLLER), "--helper-image", HELPER_IMAGE,
                     "--evidence-dir", str(self.evidence), "install", "--container", self.container,
                     "--policy-id", self.policy_id, "--preview-port", "18081", check=False)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("expected-netns", result.stderr)
        self.assertFalse((self.evidence / "policy.json").exists())

    def test_later_operations_reject_a_conflicting_inode(self):
        self.start_renderer()
        self.ctl("install", "--container", self.container, "--policy-id", self.policy_id, "--preview-port", "18081")
        for action in ("snapshot", "verify", "cleanup"):
            result = self.ctl(action, "--container", self.container, "--policy-id", self.policy_id,
                              "--expected-netns", "net:[1]", check=False)
            self.assertNotEqual(result.returncode, 0, action)
            self.assertIn("namespace", (result.stderr + result.stdout).lower(), action)
        self.ctl("cleanup", "--container", self.container, "--policy-id", self.policy_id)

    def random_preview_port(self):
        with socket.socket() as candidate:
            candidate.bind(("127.0.0.1", 0))
            port = candidate.getsockname()[1]
        self.assertNotEqual(port, 8081)
        return port

    def start_diagnostic_8081(self):
        listener = (
            "import socket;s=socket.socket();s.setsockopt(socket.SOL_SOCKET,socket.SO_REUSEADDR,1);"
            "s.bind(('127.0.0.1',8081));s.listen(1);c,a=s.accept();c.sendall(b'diagnostic-ok');c.close();s.close()"
        )
        run("docker", "exec", "-d", self.container, "python3", "-c", listener)
        for _ in range(30):
            result = run("docker", "exec", self.container, "python3", "-c",
                         "import socket;s=socket.create_connection(('127.0.0.1',8081),.3);print(s.recv(32).decode())",
                         check=False)
            if result.returncode == 0 and result.stdout.strip() == "diagnostic-ok":
                return result
            time.sleep(.03)
        raise AssertionError("pre-policy 8081 diagnostic canary did not succeed")

    def test_install_requires_and_attests_an_actual_non_8081_preview_port(self):
        preview_port = self.random_preview_port()
        self.start_renderer(preview_port)
        missing = run("python3", str(CONTROLLER), "--helper-image", HELPER_IMAGE,
                      "--evidence-dir", str(self.evidence), "install", "--container", self.container,
                      "--policy-id", self.policy_id, "--expected-netns", self.netns_inode, check=False)
        self.assertNotEqual(missing.returncode, 0)
        self.assertIn("--preview-port", missing.stderr)
        self.assertFalse((self.evidence / "policy.json").exists())
        wrong = self.random_preview_port()
        while wrong == preview_port:
            wrong = self.random_preview_port()
        rejected = self.ctl("install", "--container", self.container, "--policy-id", self.policy_id,
                            "--preview-port", str(wrong), check=False)
        self.assertNotEqual(rejected.returncode, 0)
        self.assertIn("preview", (rejected.stdout + rejected.stderr).lower())
        denied_port = self.ctl("install", "--container", self.container, "--policy-id", self.policy_id,
                               "--preview-port", "8081", check=False)
        self.assertNotEqual(denied_port.returncode, 0)
        self.assertIn("8081", (denied_port.stdout + denied_port.stderr))
        self.assertFalse((self.evidence / "policy.json").exists())

    def test_target_counters_cover_random_preview_loopback_and_controlled_dns(self):
        preview_port = self.random_preview_port()
        self.start_renderer(preview_port)
        diagnostic = self.start_diagnostic_8081()
        self.assertEqual(diagnostic.stdout.strip(), "diagnostic-ok")
        installed = self.ctl("install", "--container", self.container, "--policy-id", self.policy_id,
                             "--preview-port", str(preview_port))
        self.assertEqual(run("docker", "exec", self.container, "python3", "-c",
                             f"import socket;s=socket.create_connection(('127.0.0.1',{preview_port}),.5);print(s.recv(32).decode())").stdout.strip(),
                         "preview-ok")
        install_record = json.loads(installed.stdout)
        self.assertTrue(install_record["payload"]["mandatory_canary"]["passed"])
        self.assertGreater(install_record["payload"]["mandatory_canary"]["delta_packets"], 0)
        self.assertEqual(install_record["payload"]["mandatory_canary"]["destination"], "127.0.0.1:8081/tcp")
        canary = install_record["payload"]["mandatory_canary"]["webdriver_same_listener_canary"]
        self.assertTrue(canary["connect_attempted"], canary)
        self.assertFalse(canary["connected"], canary)
        attested_flow = json.loads((self.evidence / "webdriver-flow-attestation.json").read_text())["flow"]
        self.assertNotEqual(canary["source_port"], attested_flow["client_port"])
        self.assertGreater(canary["delta_packets"], 0)
        def snapshot_record():
            result = self.ctl("snapshot", "--container", self.container, "--policy-id", self.policy_id)
            record = json.loads(result.stdout)
            raw_ref = record["raw_refs"]["raw_rules"]["path"]
            raw_rules = json.loads((self.evidence / raw_ref).read_text())
            counters = select_webdriver_nft_rules(raw_rules, attested_flow)
            return record, counters

        marker_before, marker_before_ack = snapshot_record()
        run("docker", "kill", "--signal", "USR1", self.container)
        reuse_deadline = time.monotonic() + 2.0

        def read_reuse_marker():
            result = run_before_deadline(reuse_deadline, "docker", "exec", self.container,
                                         "cat", "/tmp/reuse-proof", check=False)
            return result.stdout.encode() if result.returncode == 0 else None

        def renderer_running():
            result = run_before_deadline(reuse_deadline, "docker", "inspect", "--format",
                                         "{{.State.Running}}", self.container, check=False)
            return result.returncode == 0 and result.stdout.strip() == "true"

        reuse_result = wait_for_reuse_marker_bounded(
            read_reuse_marker, renderer_running, deadline=reuse_deadline,
            expected=b"reuse-ok", interval=0.05)
        self.assertEqual(reuse_result, b"reuse-ok")
        marker_after, marker_after_ack = snapshot_record()
        marker_handles_match = ({key: marker_after_ack[key]["handle"] for key in marker_after_ack}
                                == {key: marker_before_ack[key]["handle"] for key in marker_before_ack})
        marker_ack_deltas = {key: marker_after_ack[key]["packets"] - marker_before_ack[key]["packets"]
                             for key in marker_before_ack}
        marker_ack_passed = marker_handles_match and all(delta > 0 for delta in marker_ack_deltas.values())
        (self.evidence / "webdriver-ack-marker-proof.json").write_text(json.dumps({
            "before_record": marker_before["sequence"], "after_record": marker_after["sequence"],
            "ack_before": marker_before_ack, "ack_after": marker_after_ack,
            "handles_match": marker_handles_match, "ack_counter_deltas": marker_ack_deltas,
            "passed": marker_ack_passed}, indent=2, sort_keys=True) + "\n")
        self.assertTrue(marker_ack_passed,
                        {"before": marker_before, "after": marker_after, "deltas": marker_ack_deltas})

        run("docker", "kill", "--signal", "USR2", self.container)
        close_deadline = time.monotonic() + 2.0
        def read_close_marker():
            result = run_before_deadline(close_deadline, "docker", "exec", self.container,
                                         "cat", "/tmp/close-proof", check=False)
            return result.stdout.encode() if result.returncode == 0 else None
        close_result = wait_for_reuse_marker_bounded(
            read_close_marker, renderer_running=lambda: (
                run_before_deadline(close_deadline, "docker", "inspect", "--format",
                                    "{{.State.Running}}", self.container, check=False).stdout.strip() == "true"),
            deadline=close_deadline, expected=b"client-fd-closed", interval=0.05)
        self.assertEqual(close_result, b"client-fd-closed")

        same_port_before, same_port_ack_before = snapshot_record()
        probe_code = same_port_probe_code(attested_flow)
        probe_started_ns = time.monotonic_ns()
        probe_command = ["docker", "exec", "--user", "10001:10001", self.container,
                         "python3", "-c", probe_code]
        try:
            probe = run(*probe_command, check=False, timeout=5)
        except (OSError, subprocess.SubprocessError) as exc:
            probe_ended_ns = time.monotonic_ns()
            receipt = write_failed_probe_receipt(
                self.evidence / "same-port-syn-proof.json", command=probe_command,
                returncode=getattr(exc, "returncode", None),
                stdout=getattr(exc, "stdout", "") or "", stderr=getattr(exc, "stderr", "") or "",
                started_ns=probe_started_ns, ended_ns=probe_ended_ns, error=repr(exc))
            self.fail({"same_port_probe": receipt})
        probe_ended_ns = time.monotonic_ns()
        try:
            same_port_evidence = json.loads(probe.stdout)
        except (TypeError, json.JSONDecodeError) as exc:
            receipt = write_failed_probe_receipt(
                self.evidence / "same-port-syn-proof.json", command=probe_command,
                returncode=probe.returncode, stdout=probe.stdout, stderr=probe.stderr,
                started_ns=probe_started_ns, ended_ns=probe_ended_ns,
                error=f"invalid probe JSON: {exc}")
            self.fail({"same_port_probe": receipt})
        if probe.returncode != 0:
            receipt = write_failed_probe_receipt(
                self.evidence / "same-port-syn-proof.json", command=probe_command,
                returncode=probe.returncode, stdout=probe.stdout, stderr=probe.stderr,
                started_ns=probe_started_ns, ended_ns=probe_ended_ns,
                error="probe subprocess returned nonzero")
            self.fail({"same_port_probe": receipt})
        same_port_after, same_port_ack_after = snapshot_record()
        same_port_handles_match = ({key: same_port_ack_after[key]["handle"] for key in same_port_ack_after}
                                   == {key: same_port_ack_before[key]["handle"] for key in same_port_ack_before})
        same_port_ack_deltas = {key: same_port_ack_after[key]["packets"] - same_port_ack_before[key]["packets"]
                                for key in same_port_ack_before}
        drop_key = "loopback_v4_drop" if attested_flow["family"] == 4 else "loopback_v6_drop"
        same_port_drop_delta = (same_port_after["payload"]["counters"][drop_key]["packets"]
                                - same_port_before["payload"]["counters"][drop_key]["packets"])
        same_port_passed = same_port_handles_match and same_port_probe_passed(
            same_port_evidence, flow=attested_flow, loopback_drop_delta=same_port_drop_delta,
            ack_counter_deltas=same_port_ack_deltas)
        (self.evidence / "same-port-syn-proof.json").write_text(json.dumps({
            "evidence": same_port_evidence, "before_record": same_port_before["sequence"],
            "after_record": same_port_after["sequence"], "loopback_drop_delta": same_port_drop_delta,
            "ack_before": same_port_ack_before, "ack_after": same_port_ack_after,
            "handles_match": same_port_handles_match, "ack_counter_deltas": same_port_ack_deltas,
            "probe_command": probe_command, "probe_returncode": probe.returncode,
            "probe_stdout": probe.stdout, "probe_stderr": probe.stderr,
            "monotonic_started_ns": probe_started_ns, "monotonic_ended_ns": probe_ended_ns,
            "passed": same_port_passed}, indent=2, sort_keys=True) + "\n")
        self.assertTrue(same_port_passed,
                        {"evidence": same_port_evidence, "loopback_drop_delta": same_port_drop_delta,
                         "ack_counter_deltas": same_port_ack_deltas})
        state = json.loads((self.evidence / "policy.json").read_text())
        self.assertEqual(state["preview_port"], preview_port)
        wrong_port_snapshot = self.ctl("snapshot", "--container", self.container, "--policy-id", self.policy_id,
                                       "--preview-port", "8081", check=False)
        self.assertNotEqual(wrong_port_snapshot.returncode, 0)
        self.assertIn("preview port mismatch", (wrong_port_snapshot.stdout + wrong_port_snapshot.stderr).lower())
        attestation = json.loads((self.evidence / "preview-port-attestation.json").read_text())
        self.assertEqual(attestation["port"], preview_port)
        self.assertEqual(attestation["returncode"], 0)
        canary_file = json.loads((self.evidence / "mandatory-canary.json").read_text())
        self.assertTrue(canary_file["passed"])
        snapshot = json.loads(self.ctl("snapshot", "--container", self.container,
                                       "--policy-id", self.policy_id).stdout)["payload"]
        self.assertEqual(snapshot["default_routes"], [])
        routes = {(str(ipaddress.ip_network(item["dst"], strict=False)), item.get("dev"))
                  for item in snapshot["routes"]["ipv4"] if item.get("dst")}
        expected = {
            ("10.0.0.1/32", "policy0"), ("169.254.169.254/32", "policy0"),
            ("172.17.0.1/32", "policy0"), ("1.1.1.1/32", "policy0"),
            ("203.0.113.53/32", "policy0"), ("198.18.10.0/24", "policy0"),
        }
        self.assertEqual(routes, expected)
        target_specs = (
            ("private_http", "10.0.0.1", 80),
            ("metadata_http", "169.254.169.254", 80),
            ("gateway_http", "172.17.0.1", 80),
            ("public_https", "1.1.1.1", 443),
        )
        events = []
        counter_names = [item[0] for item in target_specs] + ["dns_invalid"]
        for counter_name, address, port in target_specs:
            before = json.loads(self.ctl("snapshot", "--container", self.container,
                                         "--policy-id", self.policy_id).stdout)["payload"]["target_counters"]
            probe = (f"import socket;s=socket.socket();s.settimeout(.3);"
                     f"exec(\"try:\\n s.connect(('{address}',{port}))\\n print('connected')"
                     "\\nexcept OSError as e:\\n print(type(e).__name__)\")")
            result = run("docker", "exec", self.container, "python3", "-c", probe, check=False, timeout=4)
            after = json.loads(self.ctl("snapshot", "--container", self.container,
                                        "--policy-id", self.policy_id).stdout)["payload"]["target_counters"]
            self.assertGreater(after[counter_name]["packets"], before[counter_name]["packets"], counter_name)
            for other in counter_names:
                if other != counter_name:
                    self.assertEqual(after[other]["packets"], before[other]["packets"], (counter_name, other))
            events.append({"counter": counter_name, "address": address, "port": port,
                           "command": ["docker", "exec", self.container, "python3", "-c", probe],
                           "returncode": result.returncode, "stdout": result.stdout, "stderr": result.stderr,
                           "before": before[counter_name], "after": after[counter_name]})

        before = json.loads(self.ctl("snapshot", "--container", self.container,
                                     "--policy-id", self.policy_id).stdout)["payload"]["target_counters"]
        no_packet = "import socket;socket.getaddrinfo('craft-render-boundary-probe.invalid',None,flags=socket.AI_NUMERICHOST)"
        refused = run("docker", "exec", self.container, "python3", "-c", no_packet, check=False, timeout=4)
        self.assertNotEqual(refused.returncode, 0)
        after_no_packet = json.loads(self.ctl("snapshot", "--container", self.container,
                                              "--policy-id", self.policy_id).stdout)["payload"]["target_counters"]
        self.assertEqual(after_no_packet["dns_invalid"]["packets"], before["dns_invalid"]["packets"])
        query = (
            "import json,socket,struct\n"
            "name='craft-render-boundary-probe.invalid'\n"
            "q=b''.join(bytes([len(x)])+x.encode() for x in name.split('.'))+b'\\x00'\n"
            "packet=struct.pack('!HHHHHH',0x4a31,0x0100,1,0,0,0)+q+struct.pack('!HH',1,1)\n"
            "s=socket.socket(socket.AF_INET,socket.SOCK_DGRAM)\n"
            "try:\n n=s.sendto(packet,('203.0.113.53',53))\n e=None\n"
            "except OSError as exc:\n n=0\n e={'type':type(exc).__name__,'errno':exc.errno,'message':str(exc)}\n"
            "print(json.dumps({'qname':name,'destination':'203.0.113.53:53/udp','bytes':n,"
            "'send_error':e,'query_id':0x4a31}))\n"
        )
        dns = run("docker", "exec", self.container, "python3", "-c", query, check=False, timeout=4)
        self.assertEqual(dns.returncode, 0, dns.stderr)
        dns_receipt = json.loads(dns.stdout)
        self.assertEqual(dns_receipt["qname"], "craft-render-boundary-probe.invalid")
        after_dns = json.loads(self.ctl("snapshot", "--container", self.container,
                                        "--policy-id", self.policy_id).stdout)["payload"]["target_counters"]
        self.assertGreater(after_dns["dns_invalid"]["packets"], after_no_packet["dns_invalid"]["packets"])
        events.append({"name_only_failure": {"command": no_packet, "returncode": refused.returncode,
                                              "dns_counter_before": before["dns_invalid"],
                                              "dns_counter_after": after_no_packet["dns_invalid"]},
                       "dns_packet": {"command": query, "returncode": dns.returncode,
                                      "stdout": dns.stdout, "counter_after": after_dns["dns_invalid"]}})
        (self.evidence / "target-probe-events.json").write_text(json.dumps(events, indent=2, sort_keys=True) + "\n")

    def test_unconfirmed_helper_stop_after_route_mutation_destroys_renderer(self):
        base_evidence = self.evidence
        for mode in ("exception", "timeout", "false"):
            with self.subTest(mode=mode):
                self.evidence = base_evidence / mode
                self.evidence.mkdir(parents=True, exist_ok=True)
                self.policy_id = "t14_" + uuid.uuid4().hex[:12]
                self.start_renderer()
                command = ["python3", str(CONTROLLER), "--helper-image", HELPER_IMAGE,
                           "--evidence-dir", str(self.evidence), "install", "--container", self.container,
                           "--policy-id", self.policy_id, "--preview-port", "18081",
                           "--expected-netns", self.netns_inode]
                env = dict(os.environ, T14_POLICY_HELPER_TEST_PAUSE_AFTER_ROUTE="1",
                           T14_POLICY_HELPER_TEST_HELPER_TIMEOUT="2",
                           T14_POLICY_HELPER_TEST_STOP_FAULT=mode)
                process = subprocess.Popen(command, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                                           text=True, env=env)
                helper_id = ""
                route_raw = ""
                deadline = time.monotonic() + 12
                while time.monotonic() < deadline and process.poll() is None:
                    candidates = run("docker", "ps", "-q", "--filter", f"ancestor={HELPER_IMAGE}").stdout.split()
                    for candidate in candidates:
                        inspected = run("docker", "inspect", candidate, check=False)
                        if inspected.returncode:
                            continue
                        info = json.loads(inspected.stdout)[0]
                        if info["Config"].get("Cmd", [None])[0] != "install":
                            continue
                        if info["Config"].get("Labels", {}).get("io.weknora.craft.renderer-id") != self.container:
                            continue
                        routes = run("docker", "exec", candidate, "ip", "-j", "-4", "route", "show", check=False)
                        if routes.returncode:
                            continue
                        route_raw = routes.stdout
                        if any(item.get("dst") == "198.18.10.0/24" for item in json.loads(route_raw)):
                            helper_id = info["Id"]
                            (self.evidence / "stop-fault-pre-stop-route.json").write_text(route_raw)
                            break
                    if helper_id:
                        break
                    time.sleep(.03)
                self.assertTrue(helper_id, "test did not observe the actual isolated route before stop failure")
                stdout, stderr = process.communicate(timeout=30)
                self.assertNotEqual(process.returncode, 0, stdout + stderr)
                self.assertFalse((self.evidence / "policy.json").exists())
                failure = json.loads((self.evidence / "install-failure.json").read_text())
                self.assertEqual(failure["cleanup_status"], "renderer-destroyed", failure)
                self.assertFalse(failure["verified_policy"])
                self.assertFalse(failure["cleanup"].get("baseline_record"))
                stop = failure["cleanup"]["helper_stop"]
                if mode == "false":
                    self.assertIs(stop["confirmed_removed"], False)
                else:
                    self.assertIn("error", stop)
                self.assertTrue(failure["cleanup"]["destroy"]["confirmed_destroyed"])
                self.assertTrue(failure["cleanup"]["destroy"]["stopped_helper_after_destroy"]["confirmed_removed"])
                self.assertNotEqual(run("docker", "inspect", self.container, check=False).returncode, 0)
                self.assertNotEqual(run("docker", "inspect", helper_id, check=False).returncode, 0)
                self.container = ""

    def test_killed_install_helper_is_cleaned_and_never_verified(self):
        self.start_renderer()
        helper_id, returncode, stdout, stderr = self.interrupt_after_route()
        self.assertTrue(helper_id, "did not interrupt helper after its route mutation")
        self.assertNotEqual(returncode, 0, stdout + stderr)
        self.assertFalse((self.evidence / "policy.json").exists())
        failure = json.loads((self.evidence / "install-failure.json").read_text())
        self.assertFalse(failure["verified_policy"])
        self.assertEqual(failure["cleanup_status"], "verified-clean")
        self.assertEqual(failure["expected_netns"], self.netns_inode)

    def test_failed_failure_cleanup_destroys_renderer(self):
        self.start_renderer()
        helper_id, returncode, stdout, stderr = self.interrupt_after_route(fail_cleanup=True)
        self.assertTrue(helper_id, "did not interrupt helper after its route mutation")
        self.assertNotEqual(returncode, 0, stdout + stderr)
        failure = json.loads((self.evidence / "install-failure.json").read_text())
        self.assertFalse(failure["verified_policy"])
        self.assertEqual(failure["cleanup_status"], "renderer-destroyed")
        inspect = run("docker", "inspect", self.container, check=False)
        self.assertNotEqual(inspect.returncode, 0)

    def test_expected_netns_mismatch_fails_before_policy_install(self):
        self.start_renderer()
        result = self.ctl("install", "--container", self.container, "--expected-netns", "net:[1]",
                          "--policy-id", self.policy_id, "--preview-port", "18081", check=False)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("namespace", (result.stdout + result.stderr).lower())
        (self.evidence / "identity-mismatch.txt").write_text(result.stdout + result.stderr)
        self.assertFalse((self.evidence / "policy.json").exists())

    def test_policy_allows_preview_and_counts_post_policy_loopback_drop(self):
        self.start_renderer()
        self.ctl("install", "--container", self.container, "--policy-id", self.policy_id, "--preview-port", "18081")
        attestation = json.loads((self.evidence / "renderer-netns-attestation.json").read_text())
        state = json.loads((self.evidence / "policy.json").read_text())
        self.assertEqual(attestation["source"], "host-issued-docker-exec-readlink-proc-self-ns-net")
        self.assertEqual(attestation["argv"], ["docker", "exec", "--user", "10001:10001", self.container,
                                               "readlink", "/proc/self/ns/net"])
        self.assertEqual(attestation["returncode"], 0)
        self.assertEqual(attestation["netns_inode"], self.netns_inode)
        self.assertEqual(attestation["netns_inode"], state["renderer_netns"])
        self.assertEqual(state["renderer_netns"], state["helper_netns"])
        accepted = run("docker", "exec", self.container, "python3", "-c",
                       "import socket;s=socket.create_connection(('127.0.0.1',18081),2);print(s.recv(32).decode())", check=False)
        latest = json.loads(self.ctl("snapshot", "--container", self.container, "--policy-id", self.policy_id).stdout)
        raw_path = self.evidence / latest["raw_refs"]["raw_rules"]["path"]
        self.assertEqual(accepted.returncode, 0, accepted.stderr + "\n" + raw_path.read_text())
        self.assertEqual(accepted.stdout.strip(), "preview-ok")
        rules = [item["rule"] for item in json.loads(raw_path.read_text())["nftables"] if "rule" in item]
        allow_packets = []
        for rule in rules:
            matched_preview_port = any(
                expr.get("match", {}).get("right") == 18081
                and expr.get("match", {}).get("left", {}).get("payload", {}).get("field") in ("dport", "sport")
                for expr in rule.get("expr", []))
            matched_ipv4_loopback = any(
                expr.get("match", {}).get("right") == "127.0.0.1"
                and expr.get("match", {}).get("left", {}).get("payload", {}).get("field") == "daddr"
                for expr in rule.get("expr", []))
            if matched_preview_port and matched_ipv4_loopback:
                counter = next(expr["counter"]["packets"] for expr in rule["expr"] if "counter" in expr)
                allow_packets.append(counter)
        self.assertTrue(allow_packets and all(count > 0 for count in allow_packets), allow_packets)
        before = json.loads(self.ctl("snapshot", "--container", self.container, "--policy-id", self.policy_id).stdout)["payload"]
        denied = run("docker", "exec", self.container, "python3", "-c",
                     "import socket; socket.create_connection(('127.0.0.1',18082),1)", check=False, timeout=4)
        self.assertNotEqual(denied.returncode, 0)
        after = json.loads(self.ctl("snapshot", "--container", self.container, "--policy-id", self.policy_id).stdout)["payload"]
        self.assertGreater(after["counters"]["loopback_drop"]["packets"], before["counters"]["loopback_drop"]["packets"])

    def test_isolated_nonlocal_route_is_dropped_and_counted_without_uplink(self):
        self.start_renderer()
        self.ctl("install", "--container", self.container, "--policy-id", self.policy_id, "--preview-port", "18081")
        snapshot = json.loads(self.ctl("snapshot", "--container", self.container, "--policy-id", self.policy_id).stdout)["payload"]
        self.assertEqual(snapshot["default_routes"], [])
        self.assertEqual(snapshot["interfaces"], ["lo", "policy0"])
        before = snapshot["counters"]["isolated_ipv4_drop"]["packets"]
        before_v6 = snapshot["counters"]["isolated_ipv6_drop"]["packets"]
        run("docker", "exec", self.container, "python3", "-c",
            "import socket;s=socket.socket(socket.AF_INET,socket.SOCK_DGRAM);s.sendto(b'x',('198.18.10.10',9))", check=False)
        run("docker", "exec", self.container, "python3", "-c",
            "import socket;s=socket.socket(socket.AF_INET6,socket.SOCK_DGRAM);s.sendto(b'x',('2001:db8:10::10',9))", check=False)
        after = json.loads(self.ctl("snapshot", "--container", self.container, "--policy-id", self.policy_id).stdout)["payload"]
        self.assertGreater(after["counters"]["isolated_ipv4_drop"]["packets"], before)
        self.assertGreater(after["counters"]["isolated_ipv6_drop"]["packets"], before_v6)

    def test_fixed_public_process_target_hits_public_counter_and_route(self):
        self.start_renderer()
        self.ctl("install", "--container", self.container, "--policy-id", self.policy_id, "--preview-port", "18081")
        before = json.loads(self.ctl("snapshot", "--container", self.container,
                                     "--policy-id", self.policy_id).stdout)["payload"]
        probe = "import socket;s=socket.socket();s.settimeout(.3);exec(\"try:\\n s.connect(('1.1.1.1',443))\\n print('connected')\\nexcept OSError as e:\\n print(type(e).__name__)\")"
        result = run("docker", "exec", self.container, "python3", "-c", probe, check=False, timeout=4)
        after = json.loads(self.ctl("snapshot", "--container", self.container,
                                    "--policy-id", self.policy_id).stdout)["payload"]
        routes = {(str(ipaddress.ip_network(item["dst"], strict=False)), item.get("dev"))
                  for item in after["routes"]["ipv4"] if item.get("dst")}
        public_before = before["target_counters"]["public_https"]
        public_after = after["target_counters"]["public_https"]
        other_counter_deltas = {
            name: after["target_counters"][name]["packets"] - before["target_counters"][name]["packets"]
            for name in before["target_counters"] if name != "public_https"
        }
        (self.evidence / "public-target-probe.json").write_text(json.dumps({
            "attempt": {"address": "1.1.1.1", "port": 443, "protocol": "tcp",
                        "command": ["docker", "exec", self.container, "python3", "-c", probe],
                        "returncode": result.returncode, "stdout": result.stdout, "stderr": result.stderr},
            "public_counter_before": public_before, "public_counter_after": public_after,
            "other_target_counter_deltas": other_counter_deltas,
            "ipv4_routes": after["routes"]["ipv4"], "default_routes": after["default_routes"],
            "interfaces": after["interfaces"],
        }, indent=2, sort_keys=True) + "\n")
        self.assertIn(("1.1.1.1/32", "policy0"), routes, routes)
        self.assertEqual(after["default_routes"], [])
        self.assertEqual(after["interfaces"], ["lo", "policy0"])
        self.assertGreater(public_after["packets"], public_before["packets"])
        self.assertEqual(other_counter_deltas, {name: 0 for name in other_counter_deltas})

    def test_policy_disabled_negative_control_does_not_verify(self):
        self.start_renderer()
        refused = run("docker", "exec", self.container, "python3", "-c",
                      "import socket; socket.create_connection(('127.0.0.1',18082),.5)", check=False)
        result = self.ctl("verify", "--container", self.container, "--policy-id", self.policy_id, check=False)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("not installed", (result.stdout + result.stderr).lower())
        (self.evidence / "policy-disabled-negative-control.txt").write_text(
            f"unserved port exit={refused.returncode}\n{refused.stdout}{refused.stderr}\ncontroller result:\n{result.stdout}{result.stderr}")

    def test_helper_missing_nft_fails_closed(self):
        self.start_renderer()
        result = run("docker", "run", "--rm", "--network", f"container:{self.container}",
                     "--cap-drop", "ALL", "--cap-add", "NET_ADMIN", "--security-opt", "no-new-privileges",
                     "--read-only", "--user", "0:0", "--tmpfs", "/tmp:rw,noexec,nosuid,size=16m",
                     HELPER_IMAGE, "preflight", "--nft-bin", "/missing/nft", check=False)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("missing nft", result.stderr.lower())
        (self.evidence / "missing-nft.txt").write_text(result.stdout + result.stderr)

    def test_cleanup_removes_policy_and_isolated_route(self):
        self.start_renderer()
        self.ctl("install", "--container", self.container, "--policy-id", self.policy_id, "--preview-port", "18081")
        self.ctl("cleanup", "--container", self.container, "--policy-id", self.policy_id)
        result = self.ctl("verify", "--container", self.container, "--policy-id", self.policy_id, check=False)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("not installed", (result.stdout + result.stderr).lower())

    def test_cleanup_recovers_partial_install_without_controller_state(self):
        self.start_renderer()
        identity = run("docker", "run", "--rm", "--network", f"container:{self.container}",
                       "--cap-drop", "ALL", "--security-opt", "no-new-privileges", "--read-only",
                       HELPER_IMAGE, "identity")
        netns = json.loads(identity.stdout)["netns"]
        result = self.ctl("cleanup", "--container", self.container, "--policy-id", self.policy_id,
                          "--expected-netns", netns)
        self.assertIn("cleanup_verified", result.stdout)
