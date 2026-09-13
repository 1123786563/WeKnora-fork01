"""O01 readiness tests: restoring service must NOT be ready until the
deletion barriers are synced from the business side (spec step 6)."""

import pytest

from semantic_service.server_entry import compute_readiness


class HealthState:
    """Readiness inputs delegating to the SHIPPED fold (server_entry):
    the tests pin the production computation, not a test-local copy."""

    def __init__(self):
        self.stores_connected = False
        self.migrations_ready = False
        self.deletion_barriers_synced = False

    def ready(self):
        return compute_readiness(self.migrations_ready, self.stores_connected,
                                 self.deletion_barriers_synced)


@pytest.fixture()
def health_state():
    return HealthState()


def test_restoring_service_is_not_ready(health_state):
    health_state.stores_connected = True
    health_state.migrations_ready = True
    health_state.deletion_barriers_synced = False
    assert health_state.ready() is False


def test_fully_synced_service_is_ready(health_state):
    health_state.stores_connected = True
    health_state.migrations_ready = True
    health_state.deletion_barriers_synced = True
    assert health_state.ready() is True


def test_liveness_is_not_readiness(health_state):
    # Even with NOTHING ready, liveness (process up) is a separate signal:
    # readiness gates queries, liveness only restarts the container.
    assert health_state.ready() is False


def test_store_outage_is_not_ready(health_state):
    health_state.migrations_ready = True
    health_state.deletion_barriers_synced = True
    health_state.stores_connected = False
    assert health_state.ready() is False


def test_ready_endpoint_reports_not_ready_over_http():
    """HTTP-level: /ready must 503 while barriers are unsynced and
    /healthz must 200 - the handler itself is under test."""
    import threading
    import time
    import urllib.error
    import urllib.request
    from http.server import ThreadingHTTPServer

    from semantic_service.server_entry import _make_probe_handler

    # Fail-closed readiness: the barrier gate is deliberately False in the
    # shipped placeholder inputs, so /ready must be 503.
    handler = _make_probe_handler(lambda: False)
    server = ThreadingHTTPServer(("127.0.0.1", 0), handler)
    port = server.server_address[1]
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        with urllib.request.urlopen(f"http://127.0.0.1:{port}/healthz", timeout=5) as response:
            assert response.status == 200
        try:
            urllib.request.urlopen(f"http://127.0.0.1:{port}/ready", timeout=5)
            raise AssertionError("/ready must not be 200 while barriers unsynced")
        except urllib.error.HTTPError as error:
            assert error.code == 503
    finally:
        server.shutdown()
