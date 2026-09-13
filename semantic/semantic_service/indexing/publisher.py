"""Publisher: staged artifact persistence + CAS generation swap (I03).

Sequencing (each step its own transaction; single-tx lease+CAS enforcement
is deferred to the I05 wiring where the operation pipeline is completed):
1. fenced staged->publishing transition (operation lease check),
2. manifest persistence (complete),
3. CAS active-pointer swap in the store (conditional UPDATE),
4. operation terminal state (succeeded / superseded on CAS loss).

Crash safety: a crash between steps leaves the operation in publishing
with an expiring lease; claim() reclaims expired staged/publishing
operations (with fencing token bump), and the retaken worker reconciles
against the current active pointer before re-publishing.
"""

from __future__ import annotations

from typing import Optional

from ..contracts import ScopeKey
from .manifest import IndexManifest
from .store import IndexStore


class GenerationConflict(Exception):
	"""The active pointer moved between build and publish (CAS failure)."""


class LostLeaseError(Exception):
	"""The worker lost the operation lease before the pointer swap."""


class Publisher:
	def __init__(self, store: IndexStore, operations):
		self._store = store
		self._operations = operations  # OperationStore from I01

	def publish(self, scope: ScopeKey, base_generation: Optional[str], manifest: IndexManifest, lease_token: int, *, operation_id: str) -> bool:
		# 1. Operation must be live under OUR lease (fencing).
		if not self._operations.transition(operation_id, lease_token, "staged", "publishing"):
			raise LostLeaseError(
				f"lost operation lease or state race for {operation_id}; must not publish")
		# 2. Persist the COMPLETE manifest, then CAS the active pointer.
		self._store.save_manifest(manifest)
		changed = self._store.publish(scope, base_generation, manifest, lease_token=lease_token)
		if not changed:
			# Someone else published: superseded carries that signal (retry
			# policy must rebuild from the new base, not re-run blindly).
			if not self._operations.transition(operation_id, lease_token, "publishing", "superseded"):
				raise LostLeaseError(f"lease lost while marking {operation_id} superseded")
			raise GenerationConflict(
				f"active pointer moved from base {base_generation!r}; generation {manifest.generation!r} superseded")
		# 3. Terminal state records the published generation.
		if not self._operations.mark_result(
			operation_id, lease_token, "publishing", "succeeded",
			result_generation=manifest.generation):
			raise LostLeaseError(f"lease lost while finishing {operation_id}; the pointer DID move - reconcile")
		return True