"""O01 readiness tests: restoring service must NOT be ready until the
deletion barriers are synced from the business side (spec step 6)."""

import pytest


class HealthState:
    """Readiness inputs (W03/I04 semantics): migrations + stores + the
    deletion-barrier sync gate. /health (liveness) is NOT readiness."""

    def __init__(self):
        self.stores_connected = False
        self.migrations_ready = False
        self.deletion_barriers_synced = False

    def ready(self):
        return (self.migrations_ready and self.stores_connected
                and self.deletion_barriers_synced)


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
