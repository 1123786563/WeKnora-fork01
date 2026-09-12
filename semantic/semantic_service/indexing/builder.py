"""Index builder: staged, sourced graph construction (I03).

Stages a new generation manifest: unchanged documents reference their
EXISTING immutable artifacts; changed documents get new artifact ids.
Scope discipline: the request must match the claiming operation's tenant/
KB (cross-scope requests are rejected here); candidate-level cross-tenant
rejection is the extractor adapter's contract (wired with the graph/vector
adapters in the indexing pipeline tasks).
"""

from __future__ import annotations

import uuid
from typing import Optional, Protocol

from ..contracts import ApplyRequest, Operation, ScopeKey
from .manifest import ArtifactRef, DocumentEntry, IndexManifest
from .store import IndexStore


class ExtractionAdapter(Protocol):
	"""Verified upstream extraction seam (graph/vector from chunks)."""

	def extract(self, scope: ScopeKey, request: ApplyRequest) -> tuple[ArtifactRef, ...]: ...


class IndexBuilder:
	def __init__(self, store: IndexStore, extractor: ExtractionAdapter):
		self._store = store
		self._extractor = extractor

	def stage(self, op: Operation, request: ApplyRequest, *, base_manifest: Optional[IndexManifest] = None) -> IndexManifest:
		scope = request.document.scope
		if scope.tenant_id < 1 or not scope.kb_id:
			raise ValueError("stage requires a valid tenant/KB scope")
		if request.document.scope != op.scope:
			raise ValueError("operation scope does not match the request document")
		generation = f"gen-{uuid.uuid4()}"
		documents: dict[str, DocumentEntry] = dict(base_manifest.documents) if base_manifest else {}
		# The changed document gets NEW immutable artifacts.
		artifacts = self._extractor.extract(scope, request)
		if not request.document.deleted and len(artifacts) == 0:
			raise ValueError("non-deleted document requires at least one artifact")
		documents[request.document.document_id] = DocumentEntry(
			document_id=request.document.document_id,
			revision=request.document.revision,
			content_hash=request.document.content_hash,
			artifact_ids=tuple(a.artifact_id for a in artifacts),
		)
		return IndexManifest(
			scope=scope,
			generation=generation,
			base_generation=base_manifest.generation if base_manifest else None,
			documents=documents,
			config_digest=request.config.config_digest if request.config else "",
			artifacts=tuple(artifacts),
			complete=False,
		)