#!/usr/bin/env python3
"""Source-container workload for one bounded process-attempt fixture."""

from __future__ import annotations

import argparse
import errno
import json
import os
import socket
import sys
from pathlib import Path


def netns_id() -> str:
    return os.readlink("/proc/self/ns/net")


def receipt(mode: str, connected: bool | None, response_bytes: int | None, error: str | None = None) -> dict:
    return {
        "mode": mode,
        "pid": os.getpid(),
        "uid": os.getuid(),
        "uid": os.getuid(),
        "uid": os.getuid(),
        "netns": netns_id(),
        "observer": False,
        "connected": connected,
        "response_bytes": response_bytes,
        "error": error,
    }


def local_loss(host: str, port: int) -> dict:
    try:
        with socket.create_connection((host, port), timeout=3) as connection:
            connection.sendall(b"process-attempt-probe")
            connection.shutdown(socket.SHUT_WR)
            response = connection.recv(1024)
        return receipt("local-response-loss", True, len(response))
    except OSError as error:
        name = errno.errorcode.get(error.errno, str(error.errno))
        return receipt("local-response-loss", False, None, name)


def denied(host: str, port: int) -> dict:
    try:
        connection = socket.create_connection((host, port), timeout=1)
        connection.close()
        return receipt("denied", True, None)
    except OSError as error:
        name = errno.errorcode.get(error.errno, str(error.errno))
        return receipt("denied", False, None, name)


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("mode", choices=["no-packet", "local-response-loss", "denied", "unrelated"])
    parser.add_argument("--host", default="127.0.0.1")
    parser.add_argument("--port", type=int, default=18080)
    args = parser.parse_args()
    if args.mode == "no-packet":
        output = receipt("no-packet", None, None)
    elif args.mode == "local-response-loss":
        output = local_loss(args.host, args.port)
    elif args.mode == "denied":
        output = denied(args.host, args.port)
    else:
        output = local_loss(args.host, args.port)
    print(json.dumps(output, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
