#!/usr/bin/env python3
"""Internal-only listener that accepts once and deliberately sends no bytes."""

from __future__ import annotations

import json
import socket
import sys
from pathlib import Path


def serve_once(server: socket.socket, log_path: Path) -> None:
    connection, peer = server.accept()
    with connection:
        request = connection.recv(1024)
        row = {
            "event": "accepted",
            "peer": f"{peer[0]}:{peer[1]}",
            "request_bytes": len(request),
            "response_bytes": 0,
        }
        with log_path.open("a", encoding="utf-8") as output:
            output.write(json.dumps(row, sort_keys=True) + "\n")
    server.close()


def main() -> int:
    if len(sys.argv) != 3:
        print("usage: loss_listener.py HOST PORT", file=sys.stderr)
        return 2
    server = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    server.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    server.bind((sys.argv[1], int(sys.argv[2])))
    server.listen(4)
    serve_once(server, Path("/dev/stdout"))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
