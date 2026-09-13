"""Permission/version-partitioned query cache (A02).

Cache identity ALWAYS includes the scope hash and the permission epoch,
so a permission change (epoch bump from A01 wiring) structurally cannot
serve stale authorized results. Hits re-verify scope and deny state;
cross-scope summary sharing is impossible by key construction.
"""

from __future__ import annotations

from typing import Any, Optional


class ScopedCache:
    """In-memory, scope-partitioned store (single-threaded test/fallback
    use; production swaps in the shared cache backend keyed identically).
    No TTL or size bound: entries live until epoch mismatch or verify
    rejection drops them - swap-in backends must add their own bounds."""

    def __init__(self) -> None:
        self._store: dict[str, tuple[int, Any]] = {}

    def get(self, key: str, permission_epoch: int, *, verify=None) -> Optional[Any]:
        entry = self._store.get(key)
        if entry is None:
            return None
        stored_epoch, value = entry
        if stored_epoch != permission_epoch:
            # Epoch moved: the entry is from a DIFFERENT authorization era -
            # drop it rather than serve it.
            self._store.pop(key, None)
            return None
        if verify is not None and not verify(value):
            self._store.pop(key, None)
            return None
        return value

    def put(self, key: str, permission_epoch: int, value: Any) -> None:
        self._store[key] = (permission_epoch, value)
