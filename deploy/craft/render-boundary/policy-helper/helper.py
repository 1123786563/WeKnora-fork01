#!/usr/bin/env python3
"""Trusted one-shot namespace policy operation. All durable evidence is returned to the host."""
import argparse
import base64
import hashlib
import ipaddress
import json
import os
import socket
import selectors
import struct
import threading
import shutil
import subprocess
import sys
import time


MAX_CHILD_OUTPUT = 256 * 1024


def bounded_run(args, *, input_text=None, timeout=8, output_limit=MAX_CHILD_OUTPUT):
    """Run a helper command without retaining unbounded stdout or stderr."""
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
            for key, _ in selector.select(min(remaining, .2)):
                chunk = os.read(key.fd, min(8192, output_limit + 1 - len(buffers[key.fileobj])))
                if not chunk:
                    selector.unregister(key.fileobj)
                    continue
                buffers[key.fileobj].extend(chunk)
                if len(buffers[key.fileobj]) > output_limit:
                    raise RuntimeError("child output exceeded bounded capture limit")
        return subprocess.CompletedProcess(args, proc.wait(timeout=max(.01, deadline-time.monotonic())),
                                           buffers[proc.stdout].decode(errors="replace"),
                                           buffers[proc.stderr].decode(errors="replace"))
    except BaseException:
        proc.kill()
        proc.wait()
        raise
    finally:
        selector.close()
        proc.stdout.close()
        proc.stderr.close()


def command(args, *, input_text=None):
    result = bounded_run(args, input_text=input_text)
    if result.returncode:
        raise RuntimeError(f"command failed ({result.returncode}): {' '.join(args)}: {result.stderr.strip()}")
    return result.stdout


def expect_namespace(expected):
    self_ns = os.readlink("/proc/self/ns/net")
    if expected and self_ns != expected:
        raise RuntimeError(f"expected namespace {expected}, got {self_ns}")
    return self_ns


def policy_table(policy_id):
    if not policy_id.startswith("t14_") or not policy_id[4:].isalnum() or len(policy_id) > 20:
        raise RuntimeError("invalid policy id")
    return "t14_" + policy_id[4:].lower()


def nft_json(table):
    raw = command(["nft", "-j", "list", "table", "inet", table])
    return raw, json.loads(raw)


FIXED_TARGETS = {
    "private_http": {"destination": "10.0.0.1", "protocol": "tcp", "ports": [80, 443],
                     "comment": "policy-target-private-http-drop"},
    "metadata_http": {"destination": "169.254.169.254", "protocol": "tcp", "ports": [80, 443],
                       "comment": "policy-target-metadata-http-drop"},
    "gateway_http": {"destination": "172.17.0.1", "protocol": "tcp", "ports": [80, 443],
                      "comment": "policy-target-gateway-http-drop"},
    "public_https": {"destination": "1.1.1.1", "protocol": "tcp", "ports": [80, 443],
                     "comment": "policy-target-public-http-drop"},
    "dns_invalid": {"destination": "203.0.113.53", "protocol": "udp", "ports": [53],
                    "domain": "craft-render-boundary-probe.invalid",
                    "comment": "policy-target-invalid-dns-drop"},
}

COUNTER_LABELS = {
    "policy-loopback-v4-drop": "loopback_v4_drop",
    "policy-loopback-v6-drop": "loopback_v6_drop",
    "policy-isolated-ipv4-drop": "isolated_ipv4_drop",
    "policy-isolated-ipv6-drop": "isolated_ipv6_drop",
    **{target["comment"]: name for name, target in FIXED_TARGETS.items()},
}


def webdriver_rules(flow):
    """Build only two ACK-guarded, full-tuple WebDriver exceptions."""
    import ipaddress
    if not isinstance(flow, dict) or set(flow) != {
            "family", "client_addr", "client_port", "driver_addr", "driver_port", "state", "client_socket_inode"}:
        raise RuntimeError("WebDriver flow has unexpected field set")
    family = flow["family"]
    if type(family) is not int or family not in (4, 6) or flow["state"] != "ESTABLISHED":
        raise RuntimeError("WebDriver flow family or state is invalid")
    try:
        client = ipaddress.ip_address(flow["client_addr"])
        driver = ipaddress.ip_address(flow["driver_addr"])
    except (TypeError, ValueError) as exc:
        raise RuntimeError("WebDriver flow address is invalid") from exc
    if client.version != family or driver.version != family or not client.is_loopback or not driver.is_loopback:
        raise RuntimeError("WebDriver flow must use matching-family loopback addresses")
    for key in ("client_port", "driver_port"):
        if type(flow[key]) is not int or not 1 <= flow[key] <= 65535:
            raise RuntimeError("WebDriver flow port is invalid")
    if not isinstance(flow["client_socket_inode"], str) or not flow["client_socket_inode"].isdigit() or int(flow["client_socket_inode"]) <= 0:
        raise RuntimeError("WebDriver client socket inode is invalid")
    prefix = "ip" if family == 4 else "ip6"
    ack_guard = "tcp flags & (syn | ack) == ack"
    c2d = (f"  {prefix} saddr {client} tcp sport {flow['client_port']} "
           f"{prefix} daddr {driver} tcp dport {flow['driver_port']} {ack_guard} counter accept")
    d2c = (f"  {prefix} saddr {driver} tcp sport {flow['driver_port']} "
           f"{prefix} daddr {client} tcp dport {flow['client_port']} {ack_guard} counter accept")
    return [c2d, d2c]


_NLMSG_HDR = struct.Struct("=IHHII")
_INET_DIAG_REQ_V2 = struct.Struct("=BBBBIHH16s16sI2I")
_INET_DIAG_MSG = struct.Struct("=BBBB2s2s16s16sI2I5I")
_RTATTR_HDR = struct.Struct("=HH")
_NETLINK_SOCK_DIAG = getattr(socket, "NETLINK_SOCK_DIAG", 4)
_SOCK_DIAG_BY_FAMILY = 20
_NLMSG_ERROR, _NLMSG_DONE = 2, 3
_NLM_F_REQUEST, _NLM_F_DUMP, _NLM_F_DUMP_INTR, _NLM_F_MULTI = 1, 0x300, 0x10, 0x2
_IPPROTO_TCP, _TCP_ESTABLISHED = 6, 1
_SOCK_DIAG_TIMEOUT, _SOCK_DIAG_MAX_DATAGRAM = 2.0, 64 * 1024
_SOCK_DIAG_MAX_TOTAL, _SOCK_DIAG_MAX_RECORDS, _SOCK_DIAG_MAX_MESSAGES = 1024 * 1024, 4096, 8192


def _sock_diag_request(sequence, family):
    if family not in (4, 6):
        raise RuntimeError("invalid WebDriver socket family")
    af = socket.AF_INET if family == 4 else socket.AF_INET6
    request = _INET_DIAG_REQ_V2.pack(af, _IPPROTO_TCP, 0, 0, 0xFFFFFFFF,
                                     0, 0, b"\0" * 16, b"\0" * 16,
                                     0, 0xFFFFFFFF, 0xFFFFFFFF)
    return _NLMSG_HDR.pack(_NLMSG_HDR.size + len(request), _SOCK_DIAG_BY_FAMILY,
                           _NLM_F_REQUEST | _NLM_F_DUMP, sequence, 0) + request


def _sock_diag_attrs(raw):
    offset = 0
    while offset < len(raw):
        if len(raw) - offset < _RTATTR_HDR.size:
            raise ValueError("short socket diagnostic attribute header")
        length, _kind = _RTATTR_HDR.unpack_from(raw, offset)
        if length < _RTATTR_HDR.size or offset + length > len(raw):
            raise ValueError("invalid socket diagnostic attribute length")
        offset += (length + 3) & ~3
        if offset > len(raw):
            raise ValueError("invalid socket diagnostic attribute alignment")


def _parse_sock_diag_record(msg, expected_family):
    minimum = _NLMSG_HDR.size + _INET_DIAG_MSG.size
    if len(msg) < minimum:
        raise ValueError("short inet_diag_msg")
    (family, state, _timer, _retrans, sport_raw, dport_raw, source, destination,
     _ifindex, cookie0, cookie1, _expires, _rqueue, _wqueue, _uid, inode) = _INET_DIAG_MSG.unpack_from(
         msg, _NLMSG_HDR.size)
    sport = struct.unpack("!H", sport_raw)[0]
    dport = struct.unpack("!H", dport_raw)[0]
    expected_af = socket.AF_INET if expected_family == 4 else socket.AF_INET6
    if family != expected_af:
        raise ValueError("socket diagnostic family mismatch")
    if expected_family == 4:
        if source[4:] != b"\0" * 12 or destination[4:] != b"\0" * 12:
            raise ValueError("invalid IPv4 socket diagnostic address padding")
        source, destination = source[:4], destination[:4]
    _sock_diag_attrs(msg[minimum:])
    return {
        "tuple": {"src": ipaddress.ip_address(source).compressed,
                  "dst": ipaddress.ip_address(destination).compressed,
                  "sport": str(sport), "dport": str(dport)},
        "state": state,
        "inode": inode,
        "cookie": [cookie0, cookie1],
    }


def parse_sock_diag_dump(flow, messages, *, sequence, local_port_id, now=time.monotonic):
    """Validate a bounded SOCK_DIAG dump and attest one exact host-owned TCP socket."""
    webdriver_rules(flow)  # Validate the complete host-attested seven-field contract.
    # Match the canonical spelling emitted by inet_diag even when the host
    # attested a different valid textual representation of the same address.
    expected_tuple = {"src": ipaddress.ip_address(flow["client_addr"]).compressed,
                      "dst": ipaddress.ip_address(flow["driver_addr"]).compressed,
                      "sport": str(flow["client_port"]), "dport": str(flow["driver_port"])}
    expected_inode = int(flow["client_socket_inode"])
    deadline, total, records, message_count, datagram_count = now() + _SOCK_DIAG_TIMEOUT, 0, [], 0, 0
    done = False
    for item in messages:
        datagram_count += 1
        if datagram_count > _SOCK_DIAG_MAX_MESSAGES:
            raise RuntimeError("socket diagnostic datagram limit exceeded")
        if now() > deadline:
            raise RuntimeError("socket diagnostic dump exceeded time limit")
        if item.get("sender_pid") != 0 or item.get("sequence") != sequence:
            raise RuntimeError("socket diagnostic sender or sequence mismatch")
        if item.get("truncated") or item.get("dump_intr"):
            raise RuntimeError("socket diagnostic dump was truncated or interrupted")
        raw = item.get("data", b"")
        size = item.get("datagram_bytes", len(raw))
        aggregate = item.get("aggregate_bytes")
        total = max(total, aggregate) if aggregate is not None else total + size
        if size > _SOCK_DIAG_MAX_DATAGRAM or total > _SOCK_DIAG_MAX_TOTAL:
            raise RuntimeError("socket diagnostic dump exceeded byte limit")
        if item.get("received_at", now()) > deadline:
            raise RuntimeError("socket diagnostic response exceeded deadline")
        if item.get("error"):
            raise RuntimeError("socket diagnostic returned NLMSG_ERROR")
        offset = 0
        while offset < len(raw):
            if done:
                raise RuntimeError("socket diagnostic data received after NLMSG_DONE")
            if len(raw) - offset < _NLMSG_HDR.size:
                raise RuntimeError("malformed socket diagnostic framing")
            length, msg_type, flags, msg_seq, msg_pid = _NLMSG_HDR.unpack_from(raw, offset)
            if (length < _NLMSG_HDR.size or offset + length > len(raw)
                    or msg_seq != sequence or msg_pid != local_port_id):
                raise RuntimeError("malformed socket diagnostic message header")
            msg = raw[offset:offset + length]
            aligned = (length + 3) & ~3
            if offset + aligned > len(raw):
                raise RuntimeError("malformed socket diagnostic message alignment")
            offset += aligned
            message_count += 1
            if message_count > _SOCK_DIAG_MAX_MESSAGES:
                raise RuntimeError("socket diagnostic message limit exceeded")
            if flags & _NLM_F_DUMP_INTR:
                raise RuntimeError("socket diagnostic dump interrupted")
            if msg_type == _NLMSG_ERROR:
                raise RuntimeError("socket diagnostic returned NLMSG_ERROR")
            if msg_type == _NLMSG_DONE:
                if len(msg) not in (_NLMSG_HDR.size, _NLMSG_HDR.size + 4):
                    raise RuntimeError("malformed socket diagnostic NLMSG_DONE")
                if (len(msg) == _NLMSG_HDR.size + 4
                        and struct.unpack_from("=i", msg, _NLMSG_HDR.size)[0] != 0):
                    raise RuntimeError("socket diagnostic NLMSG_DONE reported an error")
                done = True
            elif msg_type == _SOCK_DIAG_BY_FAMILY:
                if not flags & _NLM_F_MULTI:
                    raise RuntimeError("socket diagnostic record lacks multipart flag")
                try:
                    records.append(_parse_sock_diag_record(msg, flow["family"]))
                except (ValueError, struct.error) as exc:
                    raise RuntimeError(f"malformed inet_diag_msg: {exc}") from exc
                if len(records) > _SOCK_DIAG_MAX_RECORDS:
                    raise RuntimeError("socket diagnostic record limit exceeded")
            else:
                raise RuntimeError("unexpected socket diagnostic message type")
    if not done:
        raise RuntimeError("socket diagnostic dump lacks NLMSG_DONE")
    matches = [record for record in records
               if record["tuple"] == expected_tuple
               and record["state"] == _TCP_ESTABLISHED
               and record["inode"] == expected_inode]
    if len(matches) != 1:
        raise RuntimeError("exact host-attested ESTABLISHED socket is absent or ambiguous")
    record = matches[0]
    return {"source": "sockdiag", "tuple": record["tuple"], "state": "ESTABLISHED",
            "inode": str(record["inode"]), "cookie": record["cookie"]}


def require_attested_webdriver_socket(flow):
    """Require one exact TCP socket matching the host-attested tuple and inode."""
    webdriver_rules(flow)
    sequence = int.from_bytes(os.urandom(4), "little") or 1
    sock = socket.socket(socket.AF_NETLINK, socket.SOCK_DGRAM, _NETLINK_SOCK_DIAG)
    try:
        sock.bind((0, 0))
        local_port_id = sock.getsockname()[0]
        sock.settimeout(_SOCK_DIAG_TIMEOUT)
        sock.sendto(_sock_diag_request(sequence, flow["family"]), (0, 0))
        deadline, messages, total = time.monotonic() + _SOCK_DIAG_TIMEOUT, [], 0
        while True:
            remaining = deadline - time.monotonic()
            if remaining <= 0:
                raise RuntimeError("socket diagnostic dump timed out")
            sock.settimeout(remaining)
            try:
                data, ancillary, flags, address = sock.recvmsg(_SOCK_DIAG_MAX_DATAGRAM)
            except socket.timeout as exc:
                raise RuntimeError("socket diagnostic dump timed out") from exc
            if flags & getattr(socket, "MSG_TRUNC", 0):
                raise RuntimeError("socket diagnostic datagram was truncated")
            total += len(data)
            if len(data) > _SOCK_DIAG_MAX_DATAGRAM or total > _SOCK_DIAG_MAX_TOTAL:
                raise RuntimeError("socket diagnostic dump exceeded byte limit")
            messages.append({"data": data, "sender_pid": address[0], "sequence": sequence,
                             "truncated": False, "aggregate_bytes": total,
                             "datagram_bytes": len(data)})
            if len(messages) > _SOCK_DIAG_MAX_MESSAGES:
                raise RuntimeError("socket diagnostic message limit exceeded")
            offset, found_done = 0, False
            while offset < len(data):
                if len(data) - offset < _NLMSG_HDR.size:
                    raise RuntimeError("malformed socket diagnostic framing")
                length, msg_type, _msg_flags, _msg_seq, _msg_pid = _NLMSG_HDR.unpack_from(data, offset)
                if length < _NLMSG_HDR.size or offset + length > len(data):
                    raise RuntimeError("malformed socket diagnostic message framing")
                if msg_type == _NLMSG_DONE:
                    found_done = True
                offset += (length + 3) & ~3
            if found_done:
                break
        return parse_sock_diag_dump(flow, messages, sequence=sequence, local_port_id=local_port_id)
    except OSError as exc:
        raise RuntimeError(f"socket diagnostic query failed: {exc}") from exc
    finally:
        sock.close()

def counters(data):
    found = {key: {"packets": 0, "bytes": 0}
             for key in ("loopback_v4_drop", "loopback_v6_drop", "isolated_ipv4_drop", "isolated_ipv6_drop")}
    target_counts = {name: {"packets": 0, "bytes": 0, "counter_id": target["comment"],
                            "destination": target["destination"], "protocol": target["protocol"],
                            "destination_ports": target["ports"]}
                     for name, target in FIXED_TARGETS.items()}
    seen = set()
    for item in data.get("nftables", []):
        rule = item.get("rule")
        if not rule or rule.get("chain") != "output":
            continue
        comment = rule.get("comment")
        key = COUNTER_LABELS.get(comment)
        if not key:
            continue
        count = next((expr["counter"] for expr in rule.get("expr", []) if "counter" in expr), None)
        if count is None or comment in seen:
            raise RuntimeError(f"missing or duplicate policy counter {comment}")
        seen.add(comment)
        metric = {"packets": int(count["packets"]), "bytes": int(count["bytes"])}
        if key in target_counts:
            target_counts[key].update(metric)
        else:
            found[key] = metric
    if seen != set(COUNTER_LABELS):
        raise RuntimeError("policy counter set is incomplete")
    found["loopback_drop"] = {
        "packets": found["loopback_v4_drop"]["packets"] + found["loopback_v6_drop"]["packets"],
        "bytes": found["loopback_v4_drop"]["bytes"] + found["loopback_v6_drop"]["bytes"],
    }
    found["target_counters"] = target_counts
    return found


def all_routes():
    return (json.loads(command(["ip", "-j", "-4", "route", "show"])),
            json.loads(command(["ip", "-j", "-6", "route", "show"])))


def snapshot(table, policy_id, expected_netns):
    netns = expect_namespace(expected_netns)
    raw_rules, parsed = nft_json(table)
    routes4, routes6 = all_routes()
    routes = {"ipv4": routes4, "ipv6": routes6}
    links = json.loads(command(["ip", "-j", "link", "show"]))
    default_routes = [r for r in routes4 + routes6 if not r.get("dst") or r.get("dst") == "default"]
    names = sorted(link["ifname"] for link in links)
    if default_routes or names != ["lo", "policy0"]:
        raise RuntimeError(f"isolation invariant failed: defaults={default_routes}, interfaces={names}")
    expected_routes4 = {
        ("198.18.10.0/24", "policy0"),
        *((target["destination"] + "/32", "policy0") for target in FIXED_TARGETS.values()),
    }
    # Linux creates this link-local connected route when the dummy interface is
    # brought up. It remains confined to policy0 and is part of the exact inventory.
    expected_routes6 = {("2001:db8:10::/64", "policy0"), ("fe80::/64", "policy0")}
    actual_routes4 = {(str(ipaddress.ip_network(route.get("dst"), strict=False)), route.get("dev"))
                      for route in routes4 if route.get("dst") and route.get("dst") != "default"}
    actual_routes6 = {(str(ipaddress.ip_network(route.get("dst"), strict=False)), route.get("dev"))
                      for route in routes6 if route.get("dst") and route.get("dst") != "default"}
    if actual_routes4 != expected_routes4 or actual_routes6 != expected_routes6:
        raise RuntimeError(f"fixed isolated route inventory mismatch: ipv4={actual_routes4}, ipv6={actual_routes6}")
    counter_values = counters(parsed)
    return {
        "policy_id": policy_id,
        "netns": netns,
        "counters": counter_values,
        "target_counters": counter_values["target_counters"],
        "default_routes": default_routes,
        "interfaces": names,
        "routes": routes,
        "raw_rules": raw_rules,
        "raw_rules_sha256": hashlib.sha256(raw_rules.encode()).hexdigest(),
        "raw_route_json": json.dumps(routes, sort_keys=True),
        "raw_link_json": json.dumps(links, sort_keys=True),
    }


def install(args):
    netns = expect_namespace(args.expected_netns)
    links = json.loads(command(["ip", "-j", "link", "show"]))
    if sorted(link["ifname"] for link in links) != ["lo"]:
        raise RuntimeError("target namespace has unexpected pre-existing interfaces")
    routes4, routes6 = all_routes()
    if routes4 or routes6:
        raise RuntimeError(f"target namespace has pre-existing routes: ipv4={routes4}, ipv6={routes6}")
    if shutil.which("nft") is None:
        raise RuntimeError("helper missing nft executable")
    if shutil.which("ip") is None:
        raise RuntimeError("helper missing ip executable")
    table = policy_table(args.policy_id)
    webdriver = json.loads(args.webdriver_flow_json)
    require_attested_webdriver_socket(webdriver)
    webdriver_accepts = "\n".join(webdriver_rules(webdriver))
    target_rules = "\n".join(
        f'  ip daddr {target["destination"]} {target["protocol"]} dport {{ {", ".join(map(str, target["ports"]))} }} counter drop comment "{target["comment"]}"'
        for target in FIXED_TARGETS.values())
    rules = f'''table inet {table} {{
 chain output {{
  type filter hook output priority -150; policy drop;
  ip daddr 127.0.0.1 tcp dport {args.preview_port} counter accept
  ip saddr 127.0.0.1 tcp sport {args.preview_port} counter accept
  ip6 daddr ::1 tcp dport {args.preview_port} counter accept
  ip6 saddr ::1 tcp sport {args.preview_port} counter accept
{webdriver_accepts}
  ip daddr 127.0.0.0/8 counter drop comment "policy-loopback-v4-drop"
  ip6 daddr ::1 counter drop comment "policy-loopback-v6-drop"
  ip daddr 198.18.10.0/24 counter drop comment "policy-isolated-ipv4-drop"
  ip6 daddr 2001:db8:10::/64 counter drop comment "policy-isolated-ipv6-drop"
{target_rules}
 }}
}}'''

    try:
        command(["ip", "link", "add", "policy0", "type", "dummy"])
        command(["ip", "link", "set", "policy0", "up"])
        command(["ip", "addr", "add", "198.18.10.1/32", "dev", "policy0"])
        command(["ip", "route", "add", "198.18.10.0/24", "dev", "policy0"])
        if os.environ.get("T14_POLICY_HELPER_TEST_PAUSE_AFTER_ROUTE") == "1":
            print("test fault injection: paused after IPv4 route mutation", flush=True)
            time.sleep(60)
        for target in FIXED_TARGETS.values():
            command(["ip", "route", "add", target["destination"] + "/32", "dev", "policy0"])
        command(["ip", "-6", "route", "add", "2001:db8:10::/64", "dev", "policy0"])
        command(["nft", "-f", "-"], input_text=rules)
        return snapshot(table, args.policy_id, netns)
    except Exception:
        bounded_run(["nft", "delete", "table", "inet", table], timeout=8)
        for target in FIXED_TARGETS.values():
            bounded_run(["ip", "route", "del", target["destination"] + "/32", "dev", "policy0"], timeout=8)
        bounded_run(["ip", "route", "del", "198.18.10.0/24", "dev", "policy0"], timeout=8)
        bounded_run(["ip", "-6", "route", "del", "2001:db8:10::/64", "dev", "policy0"], timeout=8)
        bounded_run(["ip", "link", "del", "policy0"], timeout=8)
        raise


def cleanup(args):
    netns = expect_namespace(args.expected_netns)
    table = policy_table(args.policy_id)
    has_table = bounded_run(["nft", "list", "table", "inet", table], timeout=8).returncode == 0
    if has_table:
        command(["nft", "delete", "table", "inet", table])
    routes4, routes6 = all_routes()
    for target in FIXED_TARGETS.values():
        destination = target["destination"] + "/32"
        if any(route.get("dst") == destination and route.get("dev") == "policy0" for route in routes4):
            command(["ip", "route", "del", destination, "dev", "policy0"])
    if any(route.get("dst") == "198.18.10.0/24" and route.get("dev") == "policy0" for route in routes4):
        command(["ip", "route", "del", "198.18.10.0/24", "dev", "policy0"])
    if any(route.get("dst") == "2001:db8:10::/64" and route.get("dev") == "policy0" for route in routes6):
        command(["ip", "-6", "route", "del", "2001:db8:10::/64", "dev", "policy0"])
    links = json.loads(command(["ip", "-j", "link", "show"]))
    if any(link.get("ifname") == "policy0" for link in links):
        command(["ip", "link", "del", "policy0"])
    return {"policy_id": args.policy_id, "netns": netns, "cleanup": "complete"}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("action", choices=["identity", "baseline", "install", "snapshot", "verify", "cleanup", "preflight"])
    parser.add_argument("--expected-netns", default="")
    parser.add_argument("--policy-id", default="")
    parser.add_argument("--preview-port", type=int, default=8081)
    parser.add_argument("--webdriver-flow-json", default="{}")
    parser.add_argument("--nft-bin", default="nft")
    parser.add_argument("--ip-bin", default="ip")
    args = parser.parse_args()
    if args.action == "baseline":
        netns = expect_namespace(args.expected_netns)
        ruleset = command(["nft", "-j", "list", "ruleset"])
        nft_state = json.loads(ruleset)
        tables = [item["table"] for item in nft_state.get("nftables", []) if "table" in item]
        routes4, routes6 = all_routes()
        routes = {"ipv4": routes4, "ipv6": routes6}
        links = json.loads(command(["ip", "-j", "link", "show"]))
        names = sorted(link["ifname"] for link in links)
        if names != ["lo"] or routes4 or routes6 or tables:
            raise RuntimeError(f"target namespace is not isolated: interfaces={names}, routes={routes}, tables={tables}")
        result = {"netns": netns, "interfaces": names, "routes": routes, "default_routes": [],
                  "raw_ruleset": ruleset, "raw_route_json": json.dumps(routes, sort_keys=True),
                  "raw_link_json": json.dumps(links, sort_keys=True)}
    elif args.action == "preflight":
        if not shutil.which(args.nft_bin):
            raise RuntimeError("helper missing nft executable")
        if not shutil.which(args.ip_bin):
            raise RuntimeError("helper missing ip executable")
        result = {"nft": shutil.which(args.nft_bin), "ip": shutil.which(args.ip_bin),
                  "nft_version": command([args.nft_bin, "--version"]).strip(),
                  "ip_version": command([args.ip_bin, "-Version"]).strip()}
    elif args.action == "identity":
        result = {"netns": expect_namespace(args.expected_netns)}
    elif args.action == "install":
        if not args.policy_id or not 1 <= args.preview_port <= 65535 or args.preview_port == 8081:
            raise RuntimeError("policy id and non-8081 preview port are required")
        result = install(args)
    elif args.action in ("snapshot", "verify"):
        if not args.policy_id:
            raise RuntimeError("policy id required")
        result = snapshot(policy_table(args.policy_id), args.policy_id, args.expected_netns)
    else:
        if not args.policy_id:
            raise RuntimeError("policy id required")
        result = cleanup(args)
    print(json.dumps(result, sort_keys=True))


if __name__ == "__main__":
    try:
        main()
    except Exception as exc:
        print(f"policy-helper: {exc}", file=sys.stderr)
        sys.exit(2)
