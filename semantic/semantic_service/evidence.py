"""Validation helpers for evidence against standard chunk snapshots."""

from __future__ import annotations

from semantic_service.contracts import ChunkSnapshot, Evidence


def extract_quote(text: str, start: int, end: int) -> str:
    """Extract a quote using Python string (Unicode codepoint) offsets."""
    if not isinstance(text, str):
        raise ValueError("text must be a string")
    if type(start) is not int or type(end) is not int:
        raise ValueError("span offsets must be integers")
    if start < 0 or start > end or end > len(text):
        raise ValueError("span is outside the source text")
    return text[start:end]


def validate_span(text: str, start: int, end: int, quote: str) -> None:
    """Require the supplied quote to equal the exact source span."""
    if not isinstance(quote, str):
        raise ValueError("quote must be a string")
    if extract_quote(text, start, end) != quote:
        raise ValueError("quote does not match source span")


def validate_evidence(chunk: ChunkSnapshot, evidence: Evidence) -> None:
    """Validate chunk identity, opaque hash equality, and quote provenance."""
    if not isinstance(chunk.text, str):
        raise ValueError("chunk text must be a string")
    if not isinstance(evidence.quote, str):
        raise ValueError("evidence quote must be a string")
    if evidence.chunk_id != chunk.chunk_id:
        raise ValueError("evidence references a different chunk")
    if evidence.content_hash != chunk.content_hash:
        raise ValueError("evidence content hash does not match chunk")

    if evidence.start_char is None and evidence.end_char is None:
        if evidence.quote not in chunk.text:
            raise ValueError("unanchored quote is absent from chunk text")
        return

    if evidence.start_char is None or evidence.end_char is None:
        raise ValueError("evidence span must be fully absent or present")
    validate_span(chunk.text, evidence.start_char, evidence.end_char, evidence.quote)
