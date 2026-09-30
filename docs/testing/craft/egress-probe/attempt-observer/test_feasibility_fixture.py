import json
import os
import socket
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import fixture_workload
import loss_listener


class FeasibilityFixtureTests(unittest.TestCase):
    @unittest.skipUnless(Path("/proc/self/ns/net").exists(), "Linux PID/netns fixture")
    def test_workload_receipts_bind_process_and_netns_without_claiming_observer(self):
        receipt = fixture_workload.receipt("no-packet", connected=None, response_bytes=None)

        self.assertEqual(receipt["mode"], "no-packet")
        self.assertEqual(receipt["pid"], os.getpid())
        self.assertEqual(receipt["uid"], os.getuid())
        self.assertEqual(receipt["observer"], False)
        self.assertTrue(receipt["netns"].startswith("net:["))

    def test_listener_accepts_request_and_records_zero_response_bytes(self):
        with tempfile.TemporaryDirectory() as directory:
            log_path = Path(directory) / "listener.jsonl"
            server = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
            server.bind(("127.0.0.1", 0))
            server.listen(1)
            port = server.getsockname()[1]
            import threading

            thread = threading.Thread(
                target=loss_listener.serve_once,
                args=(server, log_path),
                daemon=True,
            )
            thread.start()
            client = socket.create_connection(("127.0.0.1", port), timeout=2)
            client.sendall(b"probe")
            self.assertEqual(client.recv(16), b"")
            client.close()
            thread.join(2)
            server.close()

            row = json.loads(log_path.read_text(encoding="utf-8"))

        self.assertEqual(row["event"], "accepted")
        self.assertEqual(row["request_bytes"], 5)
        self.assertEqual(row["response_bytes"], 0)


if __name__ == "__main__":
    unittest.main()
