"""Focused tests for source-owned E0 recorder streams and host phase control."""
import json
import tempfile
import unittest
import sys
import socket
import time
import threading
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import server_v2


class RecorderTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(prefix="e0-recorder-")
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        self.recorder = server_v2.SourceRecorder(
            run_id="run-test", source="direct", ports=[8080, 8081],
            evidence_dir=self.root / "direct", control_socket=self.root / "private" / "control.sock",
        )
        self.recorder.start()
        self.addCleanup(self.recorder.stop_control)

    def events(self):
        path = self.root / "direct" / "events.jsonl"
        return [json.loads(line) for line in path.read_text().splitlines()]

    def controller(self, request):
        with socket.socket(socket.AF_UNIX, socket.SOCK_STREAM) as client:
            client.connect(str(self.recorder.control_socket))
            client.sendall(json.dumps(request).encode() + b"\n")
            return json.loads(client.makefile("rb").readline())

    def test_phase_commands_are_ordered_source_owned_and_fsynced(self):
        ack = self.controller({"ordinal": 1, "op": "begin_phase", "phase_id": "negative-1", "mode": "negative"})
        self.assertEqual(ack["source"], "direct")
        self.assertEqual(ack["phase_id"], "negative-1")
        self.controller({"ordinal": 2, "op": "barrier", "phase_id": "negative-1"})
        kinds = [event["event"] for event in self.events()]
        self.assertEqual(kinds, ["stream_start", "phase_begin", "phase_barrier"])
        self.assertEqual([event["seq"] for event in self.events()], [1, 2, 3])

    def test_barrier_refuses_live_connection_until_source_close(self):
        self.recorder.command({"ordinal": 1, "op": "begin_phase", "phase_id": "negative-1", "mode": "negative"})
        conn = self.recorder.accept("127.0.0.1:41000", "127.0.0.1:8081")
        with self.assertRaisesRegex(server_v2.RecorderError, "active"):
            self.recorder.command({"ordinal": 2, "op": "barrier", "phase_id": "negative-1"})
        self.recorder.close_connection(conn, "eof")
        self.recorder.command({"ordinal": 2, "op": "barrier", "phase_id": "negative-1"})
        self.assertIn("connection_close", [event["event"] for event in self.events()])

    def test_caller_labels_are_observations_and_cannot_change_phase(self):
        self.recorder.command({"ordinal": 1, "op": "begin_phase", "phase_id": "negative-1", "mode": "negative"})
        conn = self.recorder.accept("127.0.0.1:41000", "127.0.0.1:8081")
        request = self.recorder.request_start(conn, "GET", "/v1/models", "control:forged")
        self.assertEqual(request["phase_id"], "negative-1")
        self.assertEqual(request["untrusted_case_header"], "control:forged")
        self.assertNotIn("case", request)

    def test_append_failure_poison_blocks_commands_and_seal(self):
        self.recorder.events_path.unlink()
        self.recorder.events_path.mkdir()
        with self.assertRaises(server_v2.RecorderError):
            self.recorder.append({"event": "test"})
        self.assertIsNotNone(self.recorder.fatal_error)
        with self.assertRaisesRegex(server_v2.RecorderError, "poisoned"):
            self.recorder.command({"ordinal": 1, "op": "seal"})

    def test_seal_is_a_fsynced_final_event_and_poison_is_terminal(self):
        ack = self.recorder.command({"ordinal": 1, "op": "seal", "phase_id": None})
        self.assertTrue(ack["sealed"])
        events = self.events()
        self.assertEqual(events[-1]["event"], "stream_end")
        self.assertEqual(events[-1]["final_seq"], len(events))
        with self.assertRaisesRegex(server_v2.RecorderError, "already sealed"):
            self.recorder.append({"event": "late"})

    def test_command_ordinal_reuse_is_rejected(self):
        self.recorder.command({"ordinal": 1, "op": "begin_phase", "phase_id": "negative-1", "mode": "negative"})
        with self.assertRaisesRegex(server_v2.RecorderError, "increase by exactly one"):
            self.recorder.command({"ordinal": 1, "op": "barrier", "phase_id": "negative-1"})

    def test_redirect_location_is_host_commanded_and_phase_scoped(self):
        self.recorder.command({"ordinal": 1, "op": "begin_phase", "phase_id": "redirect-1", "mode": "redirect307|http://192.0.2.2:8081/v1/chat/completions"})
        self.assertEqual(self.recorder.mode, "redirect307")
        self.assertEqual(self.recorder.redirect_location, "http://192.0.2.2:8081/v1/chat/completions")
        self.recorder.command({"ordinal": 2, "op": "barrier", "phase_id": "redirect-1"})
        with self.assertRaisesRegex(server_v2.RecorderError, "host-selected HTTP Location"):
            self.recorder.command({"ordinal": 3, "op": "begin_phase", "phase_id": "bad", "mode": "redirect308|file:///tmp/x"})

    def test_same_stream_directory_cannot_have_two_source_writers(self):
        second = server_v2.SourceRecorder("run-test", "adapter", [8080], self.root / "direct", self.root / "other" / "control.sock")
        with self.assertRaisesRegex(server_v2.RecorderError, "evidence path already exists"):
            second.start()

    def test_phase_identity_cannot_be_reused_after_barrier(self):
        self.recorder.command({"ordinal": 1, "op": "begin_phase", "phase_id": "negative-1", "mode": "negative"})
        self.recorder.command({"ordinal": 2, "op": "barrier", "phase_id": "negative-1"})
        with self.assertRaisesRegex(server_v2.RecorderError, "reused phase identity"):
            self.recorder.command({"ordinal": 3, "op": "begin_phase", "phase_id": "negative-1", "mode": "negative"})

    def test_real_partial_http_body_is_recorded_and_blocks_until_closed(self):
        previous_recorder, previous_role = server_v2.RECORDER, server_v2.ROLE
        server_v2.RECORDER, server_v2.ROLE = self.recorder, "direct"
        self.addCleanup(setattr, server_v2, "RECORDER", previous_recorder)
        self.addCleanup(setattr, server_v2, "ROLE", previous_role)
        listener = server_v2.LedgerHTTPServer(("127.0.0.1", 0), server_v2.Handler)
        thread = threading.Thread(target=listener.serve_forever, daemon=True)
        thread.start()
        def stop_listener():
            listener.shutdown()
            listener.server_close()
            thread.join(timeout=2)
        self.addCleanup(stop_listener)
        self.recorder.command({"ordinal": 1, "op": "begin_phase", "phase_id": "partial-body", "mode": "negative"})
        client = socket.create_connection(listener.server_address, timeout=2)
        client.sendall(b"POST /v1/chat/completions HTTP/1.1\r\nHost: test\r\nContent-Length: 10\r\n\r\nabc")
        client.shutdown(socket.SHUT_WR)
        client.close()
        deadline = time.monotonic() + 2
        rows = self.events()
        while (self.recorder.connections or not any(event.get("event") == "connection_close" for event in rows)) and time.monotonic() < deadline:
            time.sleep(0.01)
            rows = self.events()
        self.assertFalse(self.recorder.connections, "listener did not close the partial-body connection")
        kinds = [event["event"] for event in rows]
        self.assertLess(kinds.index("request_start"), kinds.index("request_body"))
        body = next(event for event in rows if event["event"] == "request_body")
        self.assertFalse(body["body_complete"])
        self.assertEqual(body["body_length"], 3)
        self.assertIn("request_error", kinds)
        self.assertIn("connection_close", kinds)
        self.recorder.command({"ordinal": 2, "op": "barrier", "phase_id": "partial-body"})


if __name__ == "__main__":
    unittest.main()
