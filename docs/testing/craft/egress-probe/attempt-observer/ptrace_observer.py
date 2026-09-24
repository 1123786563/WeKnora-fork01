#!/usr/bin/env python3
"""Trace a child's IPv4 connect syscalls into a per-PID raw evidence file."""

from __future__ import annotations

import ctypes
import errno
import os
import signal
import socket
import struct
import sys
import time
from pathlib import Path


PTRACE_TRACEME = 0
PTRACE_SETOPTIONS = 0x4200
PTRACE_GETREGSET = 0x4204
PTRACE_SYSCALL = 24
PTRACE_O_TRACESYSGOOD = 1
PTRACE_O_EXITKILL = 0x100000
SIGTRAP = 5
SYS_CONNECT = 203


class Iovec(ctypes.Structure):
    _fields_ = [("iov_base", ctypes.c_void_p), ("iov_len", ctypes.c_size_t)]


def decode_sockaddr(data: bytes) -> str | None:
    if len(data) < 8 or struct.unpack_from("<H", data)[0] != socket.AF_INET:
        return None
    port = struct.unpack_from(">H", data, 2)[0]
    address = socket.inet_ntoa(data[4:8])
    return f"{address}:{port}"


def format_connect(timestamp: float, fd: int, destination: str | None, result: int) -> str:
    address, port = destination.rsplit(":", 1)
    if result == 0:
        suffix = " = 0"
    else:
        error_number = -result
        name = errno.errorcode.get(error_number, f"ERRNO_{error_number}")
        suffix = f" = -1 {name} ({os.strerror(error_number)})"
    return (
        f"{timestamp:.6f} connect({fd}, {{sa_family=AF_INET, "
        f'sin_port=htons({port}), sin_addr=inet_addr("{address}")}}, 16){suffix}'
    )


def _syscall_registers(pid: int, libc) -> tuple[int, ...] | None:
    registers = (ctypes.c_ulonglong * 35)()
    local = Iovec(ctypes.cast(registers, ctypes.c_void_p), ctypes.sizeof(registers))
    result = libc.ptrace(
        PTRACE_GETREGSET,
        ctypes.c_int(pid),
        ctypes.c_ulonglong(1),
        ctypes.byref(local),
    )
    if result != 0:
        return None
    return tuple(registers)


def _read_sockaddr(pid: int, address: int, libc) -> bytes:
    buffer = ctypes.create_string_buffer(16)
    local = Iovec(ctypes.cast(buffer, ctypes.c_void_p), ctypes.sizeof(buffer))
    remote = Iovec(ctypes.c_void_p(address), ctypes.sizeof(buffer))
    result = libc.process_vm_readv(
        ctypes.c_int(pid),
        ctypes.byref(local),
        ctypes.c_ulong(1),
        ctypes.byref(remote),
        ctypes.c_ulong(1),
        ctypes.c_ulong(0),
    )
    return buffer.raw if result >= 0 else b""


def _resume(libc, pid: int, signal_to_deliver: int = 0) -> None:
    result = libc.ptrace(
        PTRACE_SYSCALL,
        ctypes.c_int(pid),
        ctypes.c_int(0),
        ctypes.c_int(signal_to_deliver),
    )
    if result != 0:
        raise OSError(ctypes.get_errno(), "PTRACE_SYSCALL failed")


def observe(argv: list[str], trace_path: Path) -> int:
    libc = ctypes.CDLL(None, use_errno=True)
    trace_path.parent.mkdir(parents=True, exist_ok=True)
    pid = os.fork()
    if pid == 0:
        if libc.ptrace(PTRACE_TRACEME, 0, 0, 0) != 0:
            os._exit(126)
        os.kill(os.getpid(), signal.SIGSTOP)
        os.execvp(argv[0], argv)
        os._exit(127)

    _, status = os.waitpid(pid, 0)
    if not os.WIFSTOPPED(status):
        raise RuntimeError("traced child exited before initial stop")
    options = PTRACE_O_TRACESYSGOOD | PTRACE_O_EXITKILL
    if libc.ptrace(PTRACE_SETOPTIONS, pid, 0, options) != 0:
        raise OSError(ctypes.get_errno(), "PTRACE_SETOPTIONS failed")

    syscall_exit = False
    pending: tuple[float, int, str | None] | None = None
    _resume(libc, pid)
    while True:
        waited, status = os.waitpid(pid, 0)
        if waited != pid:
            continue
        if os.WIFEXITED(status):
            exit_code = os.WEXITSTATUS(status)
            break
        if os.WIFSIGNALED(status):
            exit_code = 128 + os.WTERMSIG(status)
            break
        if not os.WIFSTOPPED(status):
            continue
        stop_signal = os.WSTOPSIG(status)
        if stop_signal != (SIGTRAP | 0x80):
            _resume(libc, pid, 0 if stop_signal == signal.SIGSTOP else stop_signal)
            continue

        registers = _syscall_registers(pid, libc)
        if registers is None:
            _resume(libc, pid)
            continue
        syscall_number = registers[8]
        if syscall_number == SYS_CONNECT and not syscall_exit:
            timestamp = time.monotonic_ns() / 1_000_000_000
            destination = decode_sockaddr(_read_sockaddr(pid, registers[1], libc))
            pending = (timestamp, registers[0], destination)
        elif syscall_number == SYS_CONNECT and syscall_exit and pending is not None:
            raw_result = registers[0]
            result = raw_result - (1 << 64) if raw_result >= (1 << 63) else raw_result
            with trace_path.open("a", encoding="utf-8") as output:
                output.write(format_connect(pending[0], pending[1], pending[2], result) + "\n")
            pending = None
        _resume(libc, pid)
        syscall_exit = not syscall_exit

    with trace_path.open("a", encoding="utf-8") as output:
        output.write(f"observer child_exit {pid} {exit_code}\n")
    return exit_code


def main() -> int:
    if len(sys.argv) < 3:
        print("usage: ptrace_observer.py TRACE_PATH COMMAND [ARG...]", file=sys.stderr)
        return 2
    return observe(sys.argv[2:], Path(sys.argv[1]))


if __name__ == "__main__":
    raise SystemExit(main())
