"""Generation store: manifests, CAS active pointer, read leases (I03).

Backed by the service's own PostgreSQL schema. The active pointer swaps
atomically via a conditional UPDATE (compare-and-swap on the previous
generation); incomplete manifests are NEVER publishable; read leases let
queries pin a generation while GC waits.
"""

from __future__ import annotations

import json
import uuid
from dataclasses import dataclass
from datetime import datetime, timedelta, timezone
from typing import Optional

import psycopg
from psycopg.rows import dict_row

from ..contracts import ScopeKey
from .manifest import IndexManifest


@dataclass(frozen=True)
class ReadLease:
	lease_id: str
	generation: str
	expires_at: datetime


def _connect(dsn: str):
	return psycopg.connect(dsn, row_factory=dict_row)


class IndexStore:
	def __init__(self, dsn: str):
		self._dsn = dsn

	def close(self) -> None:
		return None

	def save_manifest(self, manifest: IndexManifest) -> None:
		payload = json.dumps(manifest.to_json(), ensure_ascii=False, sort_keys=True)
		with _connect(self._dsn) as conn:
			inserted = conn.execute(
				"INSERT INTO semantic.generations (tenant_id, kb_id, generation, base_generation, complete, manifest) VALUES (%s, %s, %s, %s, %s, %s) ON CONFLICT (tenant_id, kb_id, generation) DO NOTHING RETURNING generation",
				(manifest.scope.tenant_id, manifest.scope.kb_id, manifest.generation,
				 manifest.base_generation, manifest.complete, payload),
			).fetchone()
			if inserted is None:
				# Same generation id must mean the SAME manifest; a divergent
				# payload is a bug, never silently dropped.
				stored = conn.execute(
					"SELECT manifest FROM semantic.generations WHERE tenant_id = %s AND kb_id = %s AND generation = %s",
					(manifest.scope.tenant_id, manifest.scope.kb_id, manifest.generation),
				).fetchone()
				stored_payload = json.dumps(stored["manifest"], ensure_ascii=False, sort_keys=True) if stored else ""
				if stored_payload != payload:
					raise ValueError(
						f"divergent manifest for generation {manifest.generation!r}: "
						"stored payload differs from the incoming one")
			conn.commit()

	def load_manifest(self, scope: ScopeKey, generation: str) -> Optional[IndexManifest]:
		with _connect(self._dsn) as conn:
			row = conn.execute(
				"SELECT manifest FROM semantic.generations WHERE tenant_id = %s AND kb_id = %s AND generation = %s",
				(scope.tenant_id, scope.kb_id, generation),
			).fetchone()
		if row is None:
			return None
		stored = row["manifest"]
		if isinstance(stored, str):
			stored = json.loads(stored)
		return IndexManifest.from_json(stored)

	def active(self, scope: ScopeKey) -> Optional[str]:
		with _connect(self._dsn) as conn:
			row = conn.execute(
				"SELECT generation FROM semantic.active_generations WHERE tenant_id = %s AND kb_id = %s",
				(scope.tenant_id, scope.kb_id),
			).fetchone()
		if row is None:
			return None
		return row["generation"]

	def publish(self, scope: ScopeKey, base_generation: Optional[str], manifest: IndexManifest, *, lease_token: int) -> bool:
		"""CAS-publish a COMPLETE manifest as the active generation.

		Raises ValueError for incomplete manifests or missing persistence.
		Returns False when the CAS comparison fails (another publisher won).
		Only complete, persisted generations become active; queries never
		observe a partial state.
		"""
		if not manifest.complete:
			raise ValueError("incomplete manifest must never become active")
		with _connect(self._dsn) as conn:
			stored = conn.execute(
				"SELECT manifest FROM semantic.generations WHERE tenant_id = %s AND kb_id = %s AND generation = %s AND complete = TRUE",
				(scope.tenant_id, scope.kb_id, manifest.generation),
			).fetchone()
			if stored is None:
				raise ValueError("manifest not persisted complete before publish")
			if base_generation is None:
				changed = conn.execute(
					"INSERT INTO semantic.active_generations (tenant_id, kb_id, generation) VALUES (%s, %s, %s) ON CONFLICT (tenant_id, kb_id) DO NOTHING",
					(scope.tenant_id, scope.kb_id, manifest.generation),
				).rowcount
			else:
				changed = conn.execute(
					"UPDATE semantic.active_generations SET generation = %s, updated_at = now() WHERE tenant_id = %s AND kb_id = %s AND generation = %s",
					(manifest.generation, scope.tenant_id, scope.kb_id, base_generation),
				).rowcount
			conn.commit()
		_ = lease_token  # operation-lease enforcement lands with the I05 wiring
		return changed == 1

	def pin(self, scope: ScopeKey, *, lease_seconds: int = 300) -> ReadLease:
		"""Pin the CURRENT active generation for a query (bounded lease)."""
		generation = self.active(scope)
		if generation is None:
			raise RuntimeError("no active generation to pin")
		lease_id = str(uuid.uuid4())
		expires = datetime.now(timezone.utc) + timedelta(seconds=lease_seconds)
		with _connect(self._dsn) as conn:
			conn.execute(
				"INSERT INTO semantic.read_leases (lease_id, tenant_id, kb_id, generation, expires_at) VALUES (%s, %s, %s, %s, %s)",
				(lease_id, scope.tenant_id, scope.kb_id, generation, expires),
			)
			conn.commit()
		return ReadLease(lease_id=lease_id, generation=generation, expires_at=expires)

	def renew(self, lease_id: str, lease_seconds: int = 300) -> bool:
		"""Bounded lease renewal for long-running queries (I03 step 6)."""
		expires = datetime.now(timezone.utc) + timedelta(seconds=lease_seconds)
		with _connect(self._dsn) as conn:
			changed = conn.execute(
				"UPDATE semantic.read_leases SET expires_at = %s WHERE lease_id = %s AND expires_at > now()",
				(expires, lease_id),
			).rowcount
			conn.commit()
		return changed == 1

	def release(self, lease_id: str) -> None:
		with _connect(self._dsn) as conn:
			conn.execute("DELETE FROM semantic.read_leases WHERE lease_id = %s", (lease_id,))
			conn.commit()

	def lease_holds(self, scope: ScopeKey, generation: str) -> bool:
		with _connect(self._dsn) as conn:
			row = conn.execute(
				"SELECT 1 FROM semantic.read_leases WHERE tenant_id = %s AND kb_id = %s AND generation = %s AND expires_at > now() LIMIT 1",
				(scope.tenant_id, scope.kb_id, generation),
			).fetchone()
		return row is not None