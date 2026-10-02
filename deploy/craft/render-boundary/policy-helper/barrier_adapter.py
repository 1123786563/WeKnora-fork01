#!/usr/bin/env python3
"""Renderer-side WebDriver socket inventory joining for the render boundary.

The discovery script runs unprivileged inside the renderer namespace and
reports the chromedriver listener plus the established client sockets that
join it. select_webdriver_flow validates that inventory and emits the frozen
seven-field flow contract consumed by the trusted policy helper.
"""
import ipaddress


WEBDRIVER_DISCOVERY = r'''
import ipaddress, json, os
states = {'01': 'ESTABLISHED', '0A': 'LISTEN'}
def comm(pid):
    try:
        return open('/proc/%d/comm' % pid).read().strip()
    except OSError:
        return ''
owners = {}
for entry in os.listdir('/proc'):
    if not entry.isdigit():
        continue
    pid = int(entry)
    inodes = set()
    try:
        for fd in os.listdir('/proc/%d/fd' % pid):
            try:
                link = os.readlink('/proc/%d/fd/%s' % (pid, fd))
            except OSError:
                continue
            if link.startswith('socket:['):
                inodes.add(link[8:-1])
    except OSError:
        pass
    owners[pid] = (comm(pid), inodes)
def decode4(value):
    raw = bytes.fromhex(value[6:8] + value[4:6] + value[2:4] + value[0:2])
    return '.'.join(str(byte) for byte in raw)
def decode6(value):
    words = [value[index:index + 8] for index in range(0, 32, 8)]
    raw = bytes.fromhex(''.join(word[6:8] + word[4:6] + word[2:4] + word[0:2] for word in words))
    return ipaddress.IPv6Address(raw).compressed
def table(path, family):
    try:
        lines = open(path).read().splitlines()[1:]
    except OSError:
        return []
    rows = []
    for line in lines:
        parts = line.split()
        state = states.get(parts[3])
        if state is None:
            continue
        local, remote = parts[1].rsplit(':', 1)[0], parts[2].rsplit(':', 1)[0]
        decode = decode4 if family == 4 else decode6
        rows.append({'local_addr': decode(local), 'local_port': int(parts[1].rsplit(':', 1)[1], 16),
                     'remote_addr': decode(remote), 'remote_port': int(parts[2].rsplit(':', 1)[1], 16),
                     'state': state, 'inode': parts[9], 'family': family})
    return rows
sockets = table('/proc/net/tcp', 4) + table('/proc/net/tcp6', 6)
driver_inodes = set()
for name, inodes in owners.values():
    if name == 'chromedriver':
        driver_inodes |= inodes
listeners = [x for x in sockets if x['state']=='LISTEN' and x['inode'] in driver_inodes]
endpoints = {(x['local_addr'], x['local_port'], x['family']) for x in listeners}
clients = [x for x in sockets if x['state']=='ESTABLISHED'
           and (x['remote_addr'], x['remote_port'], x['family']) in endpoints
           and x['inode'] not in driver_inodes]
print(json.dumps({
    'listeners': [{'addr': x['local_addr'], 'port': x['local_port'], 'inode': x['inode'],
                   'state': x['state'], 'family': x['family']} for x in listeners],
    'clients': [{'local_addr': x['local_addr'], 'local_port': x['local_port'],
                 'remote_addr': x['remote_addr'], 'remote_port': x['remote_port'],
                 'inode': x['inode'], 'state': x['state'], 'family': x['family']} for x in clients]}))
'''


def _loopback(address, family):
    ip = ipaddress.ip_address(address)
    if ip.version != family or not ip.is_loopback:
        raise ValueError(f"WebDriver flow address {address} is not family-{family} loopback")
    return ip.compressed


def _port(value, label):
    if type(value) is not int or not 1 <= value <= 65535:
        raise ValueError(f"WebDriver {label} port is invalid: {value!r}")
    return value


def select_webdriver_flow(clients, listeners):
    """Join one chromedriver listener with one established client socket."""
    if not isinstance(clients, list) or not isinstance(listeners, list):
        raise ValueError("WebDriver inventory must provide client and listener lists")
    if len(listeners) != 1:
        raise ValueError(f"expected exactly one WebDriver listener, found {len(listeners)}")
    if len(clients) != 1:
        raise ValueError(f"expected exactly one WebDriver client socket, found {len(clients)}")
    listener, client = listeners[0], clients[0]
    if listener.get("state") != "LISTEN":
        raise ValueError("WebDriver listener socket is not LISTEN")
    if client.get("state") != "ESTABLISHED":
        raise ValueError("WebDriver client socket is not ESTABLISHED")
    family = listener.get("family")
    if family not in (4, 6) or client.get("family") != family:
        raise ValueError("WebDriver client and listener families do not match")
    driver_addr = _loopback(listener.get("addr"), family)
    driver_port = _port(listener.get("port"), "driver")
    if client.get("remote_addr") != driver_addr or client.get("remote_port") != driver_port:
        raise ValueError("WebDriver client socket does not join the WebDriver listener")
    client_addr = _loopback(client.get("local_addr"), family)
    client_port = _port(client.get("local_port"), "client")
    inode = client.get("inode")
    if not isinstance(inode, str) or not inode.isdigit() or int(inode) <= 0:
        raise ValueError(f"WebDriver client socket inode is invalid: {inode!r}")
    return {"family": family, "client_addr": client_addr, "client_port": client_port,
            "driver_addr": driver_addr, "driver_port": driver_port,
            "state": "ESTABLISHED", "client_socket_inode": inode}
