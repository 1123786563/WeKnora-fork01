"""Tests for process-owned network attempts parsed from per-PID strace files."""

import sys
import tempfile
import unittest
import json
import errno
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import attempt_observer
import ptrace_observer


class AttemptObserverTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(prefix="e0-attempt-observer-")
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)

    def trace(self, pid: int, *lines: str) -> Path:
        path = self.root / f"trace.{pid}"
        path.write_text("\n".join(lines) + "\n")
        return path

    def test_source_pid_connect_success_and_denial_are_kept_as_distinct_attempts(self):
        path = self.trace(
            4201,
            '1727100000.000001 connect(3, {sa_family=AF_INET, sin_port=htons(8080), sin_addr=inet_addr("127.0.0.1")}, 16) = 0',
            '1727100000.000002 connect(4, {sa_family=AF_INET, sin_port=htons(443), sin_addr=inet_addr("192.0.2.19")}, 16) = -1 ENETUNREACH (Network is unreachable)',
            '1727100000.000003 connect(5, {sa_family=AF_INET, sin_port=htons(443), sin_addr=inet_addr("192.0.2.20")}, 16) = -1 EINPROGRESS (Operation now in progress)',
        )

        attempts = attempt_observer.parse_trace_files([path], expected_pids={4201})

        self.assertEqual(
            [(row.pid, row.destination, row.outcome) for row in attempts],
            [(4201, "127.0.0.1:8080", "success"), (4201, "192.0.2.19:443", "ENETUNREACH"), (4201, "192.0.2.20:443", "EINPROGRESS")],
        )

    def test_later_response_loss_does_not_rewrite_the_observed_connect_result(self):
        path = self.trace(
            4201,
            '1727100000.000001 connect(3, {sa_family=AF_INET, sin_port=htons(8080), sin_addr=inet_addr("127.0.0.1")}, 16) = 0',
            '1727100000.000002 recvfrom(3, 0x7fff, 1024, 0, NULL, NULL) = -1 ECONNRESET (Connection reset by peer)',
        )

        attempts = attempt_observer.parse_trace_files([path], expected_pids={4201})

        self.assertEqual([(row.destination, row.outcome) for row in attempts], [("127.0.0.1:8080", "success")])

    def test_wrong_pid_cannot_receive_attempt_credit(self):
        path = self.trace(
            4202,
            '1727100000.000001 connect(3, {sa_family=AF_INET, sin_port=htons(8080), sin_addr=inet_addr("127.0.0.1")}, 16) = 0',
        )

        attempts = attempt_observer.parse_trace_files([path], expected_pids={4201})

        self.assertEqual(attempts, [])

    def test_unrelated_process_and_non_network_syscalls_receive_no_attempt_credit(self):
        unrelated = self.trace(
            5101,
            '1727100000.000001 connect(3, {sa_family=AF_INET, sin_port=htons(443), sin_addr=inet_addr("192.0.2.19")}, 16) = -1 ENETUNREACH (Network is unreachable)',
        )
        source = self.trace(4201, '1727100000.000002 read(0, "", 1) = 0')

        attempts = attempt_observer.parse_trace_files([unrelated, source], expected_pids={4201})

        self.assertEqual(attempts, [])

    def test_empty_observer_output_proves_no_attempt(self):
        path = self.trace(4201, '1727100000.000001 read(0, "", 1) = 0')

        attempts = attempt_observer.parse_trace_files([path], expected_pids={4201})

        self.assertEqual(attempts, [])

    def test_malformed_destination_or_unresolved_result_is_not_promoted(self):
        path = self.trace(
            4201,
            '1727100000.000001 connect(3, {sa_family=AF_INET, sin_port=htons(443), sin_addr=inet_addr("not-an-ip")}, 16) = -1 EIO (Input/output error)',
            '1727100000.000002 connect(4, {sa_family=AF_INET, sin_port=htons(443), sin_addr=inet_addr("192.0.2.19")}, 16) = ? ERESTARTSYS (To be restarted)',
        )

        attempts = attempt_observer.parse_trace_files([path], expected_pids={4201})

        self.assertEqual(attempts, [])


class FeasibilityEvidenceTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(prefix="e0-attempt-feasibility-")
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        self.image = "sha256:" + "0" * 64

    def write(self, relative: str, text: str) -> Path:
        path = self.root / relative
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(text, encoding="utf-8")
        return path

    def fixture(self) -> None:
        self.write(
            "identity.json",
            json.dumps(
                {
                    "image_id": self.image,
                    "container_id": "container-source",
                    "source_pid": 4201,
                    "source_uid": 10001,
                    "unrelated_pid": 5101,
                    "unrelated_uid": 10001,
                    "netns": "/var/run/netns/source",
                    "local_destination": "127.0.0.1:18080",
                    "denied_destination": "203.0.113.19:443",
                }
            ),
        )
        self.write(
            "observer/local-response-loss/trace.4201",
            '1760000000.000001 connect(3, {sa_family=AF_INET, sin_port=htons(18080), sin_addr=inet_addr("127.0.0.1")}, 16) = 0\n',
        )
        self.write(
            "observer/denied/trace.4201",
            '1760000000.000002 connect(3, {sa_family=AF_INET, sin_port=htons(443), sin_addr=inet_addr("203.0.113.19")}, 16) = -1 ENETUNREACH (Network is unreachable)\n',
        )
        self.write("observer/no-packet/trace.4201", "1760000000.000003 read(0, \"\", 1) = 0\n")
        self.write(
            "observer/unrelated/trace.5101",
            '1760000000.000004 connect(3, {sa_family=AF_INET, sin_port=htons(18080), sin_addr=inet_addr("127.0.0.1")}, 16) = 0\n',
        )
        self.write("without-observer/no-packet.stdout", json.dumps({"observer": False, "connects": 0}) + "\n")
        self.write(
            "source/local-response-loss.json",
            json.dumps({"destination": "127.0.0.1:18080", "connected": True, "response_bytes": 0}),
        )
        self.write(
            "listener.jsonl",
            json.dumps({"event": "accepted", "peer": "127.0.0.1:49152", "request_bytes": 3, "response_bytes": 0}) + "\n",
        )

    def test_complete_matrix_credits_only_expected_pid_attempt(self):
        self.fixture()
        summary = attempt_observer.validate_feasibility(self.root, self.image)

        self.assertEqual(summary["verdict"], "PASS")
        self.assertEqual(summary["credited_attempts"], 2)
        self.assertEqual(summary["wrong_pid_attempts"], 0)
        self.assertEqual(summary["unrelated_pid_attempts"], 0)
        self.assertEqual(summary["no_packet_attempts"], 0)
        self.assertEqual(summary["without_observer_attempts"], 0)

    def test_missing_observer_missing_netns_wrong_image_or_wrong_pid_are_rejected(self):
        self.fixture()
        (self.root / "observer/local-response-loss/trace.4201").unlink()
        identity = json.loads((self.root / "identity.json").read_text())
        identity["netns"] = ""
        identity["image_id"] = "sha256:" + "9" * 64
        (self.root / "identity.json").write_text(json.dumps(identity))

        summary = attempt_observer.validate_feasibility(self.root, self.image)

        self.assertEqual(summary["verdict"], "FAIL")
        self.assertIn("wrong or missing pinned image ID", summary["errors"])
        self.assertIn("source PID, container or netns identity is missing", summary["errors"])
        self.assertIn("local response-loss attempt is missing", summary["errors"])

    def test_response_loss_needs_both_attempt_and_loss_receipt(self):
        self.fixture()
        self.write(
            "observer/local-response-loss/trace.4201",
            '1760000000.000001 connect(3, {sa_family=AF_INET, sin_port=htons(18080), sin_addr=inet_addr("127.0.0.1")}, 16) = -1 ECONNREFUSED (Connection refused)\n',
        )
        self.write("listener.jsonl", "")

        summary = attempt_observer.validate_feasibility(self.root, self.image)

        self.assertEqual(summary["verdict"], "FAIL")
        self.assertIn("local response-loss attempt is missing", summary["errors"])
        self.assertIn("listener response-loss receipt is missing", summary["errors"])


class PtraceObserverFormatTests(unittest.TestCase):
    def test_decodes_ipv4_sockaddr_and_formats_exact_strace_row(self):
        sockaddr = bytes([0x02, 0x00, 0x46, 0x50, 127, 0, 0, 1])

        destination = ptrace_observer.decode_sockaddr(sockaddr)
        raw_line = ptrace_observer.format_connect(
            timestamp=1760000000.000001,
            fd=3,
            destination=destination,
            result=0,
        )

        self.assertEqual(destination, "127.0.0.1:18000")
        self.assertEqual(
            raw_line,
            '1760000000.000001 connect(3, {sa_family=AF_INET, sin_port=htons(18000), sin_addr=inet_addr("127.0.0.1")}, 16) = 0',
        )

    def test_error_returns_use_kernel_errno_name(self):
        raw_line = ptrace_observer.format_connect(
            timestamp=1760000000.000002,
            fd=4,
            destination="203.0.113.19:443",
            result=-errno.ENETUNREACH,
        )

        self.assertIn(" = -1 ENETUNREACH (Network is unreachable)", raw_line)


if __name__ == "__main__":
    unittest.main()
