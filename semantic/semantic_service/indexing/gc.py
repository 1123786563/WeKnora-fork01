"""GC: retention windows and read-lease protection (I04).

protect() reports whether a generation is still servable (active or held
by a live read lease); sweep_tombstones() clears expired tombstones ONLY
when no persisted generation (active, historical or rollback target)
still references the deleted revision. Artifact deletion itself is not
implemented here - it lands with the store adapters.
"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import timedelta

import psycopg
from psycopg.rows import dict_row

from ..contracts import ScopeKey
from .store import IndexStore


@dataclass(frozen=True)
class GCConfig:
    """Mandatory configuration: retention and replay windows are never
    defaulted - the caller must state them explicitly (plan step 5)."""
    retention: timedelta
    replay_window: timedelta


def _connect(dsn: str):
    return psycopg.connect(dsn, row_factory=dict_row)


class GarbageCollector:
    def __init__(self, dsn: str, store: IndexStore, config: GCConfig):
        self._dsn = dsn
        self._store = store
        self._config = config

    def protect(self, scope: ScopeKey, generation: str) -> bool:
        """An artifact set is protected while active or lease-held."""
        if self._store.active(scope) == generation:
            return True
        return self._store.lease_holds(scope, generation)

    def sweep_tombstones(self, scope: ScopeKey) -> int:
        """Clear expired tombstones - but ONLY when no persisted manifest in
        the scope still references the deleted document revision.

        The tombstone is the deletion barrier. Sweeping it while any
        generation (active or historical) still carries the deleted rows
        would resurrect deleted data from that generation. The sweep is
        therefore conditional on the manifest closure no longer referencing
        the (document, revision) pair.
        """
        floor = max(self._config.retention, self._config.replay_window)
        with _connect(self._dsn) as conn:
            deleted = conn.execute(
                """
                DELETE FROM semantic.tombstones t
                WHERE t.tenant_id = %s AND t.kb_id = %s
                  AND t.created_at < now() - make_interval(secs => %s)
                  AND NOT EXISTS (
                    SELECT 1
                    FROM semantic.generations g,
                         jsonb_each(g.manifest -> 'documents') AS doc(doc_id, entry)
                    WHERE g.tenant_id = t.tenant_id AND g.kb_id = t.kb_id
                      AND doc.doc_id = t.document_id
                      AND (entry ->> 'revision')::int <= t.revision)
                """,
                (scope.tenant_id, scope.kb_id, floor.total_seconds()),
            ).rowcount
            conn.commit()
        return deleted
