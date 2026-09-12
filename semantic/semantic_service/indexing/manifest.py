"""Immutable generation manifests (I03).

A manifest is the complete closure of one published index state: every
document revision it contains, the config digest it was built with, and
the artifacts (graph/vector) backing it. Manifests are never mutated;
a new generation references unchanged documents by their existing
immutable artifact ids and only stages NEW artifacts for changed ones.
"""

from __future__ import annotations

from dataclasses import dataclass, field, replace
from typing import Optional

from ..contracts import ScopeKey, scope_from_json, scope_to_json


@dataclass(frozen=True)
class DocumentEntry:
	document_id: str
	revision: int
	content_hash: str
	artifact_ids: tuple[str, ...] = ()


@dataclass(frozen=True)
class ArtifactRef:
	kind: str  # "graph" | "vector" | ...
	artifact_id: str
	hash: str


@dataclass(frozen=True)
class IndexManifest:
	scope: ScopeKey
	generation: str
	base_generation: Optional[str]
	documents: dict[str, DocumentEntry] = field(default_factory=dict)
	config_digest: str = ""
	artifacts: tuple[ArtifactRef, ...] = ()
	complete: bool = False

	def to_json(self) -> dict:
		return {
			"scope": scope_to_json(self.scope),
			"generation": self.generation,
			"base_generation": self.base_generation,
			"documents": {
				doc_id: {"revision": entry.revision, "content_hash": entry.content_hash,
						 "artifact_ids": list(entry.artifact_ids)}
				for doc_id, entry in self.documents.items()},
			"config_digest": self.config_digest,
			"artifacts": [{"kind": a.kind, "artifact_id": a.artifact_id, "hash": a.hash} for a in self.artifacts],
			"complete": self.complete,
		}

	@classmethod
	def from_json(cls, data: dict) -> "IndexManifest":
		return cls(
			scope=scope_from_json(data["scope"]),
			generation=data["generation"],
			base_generation=data.get("base_generation"),
			documents={doc_id: DocumentEntry(
				document_id=doc_id,
				revision=entry["revision"], content_hash=entry["content_hash"],
				artifact_ids=tuple(entry.get("artifact_ids", ())),
			) for doc_id, entry in data.get("documents", {}).items()},
			config_digest=data.get("config_digest", ""),
			artifacts=tuple(ArtifactRef(kind=a["kind"], artifact_id=a["artifact_id"], hash=a["hash"])
						  for a in data.get("artifacts", [])),
			complete=data.get("complete", False),
		)

	def with_complete(self) -> "IndexManifest":
		return replace(self, complete=True)