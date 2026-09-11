"""Evidence position validation (C03).

Quotes are verified against the ORIGINAL chunk text with Unicode codepoint
half-open spans [start, end). Spans exist on both endpoints or neither; a
quote without a span must still be a verbatim substring. Nothing here
repairs bad input - mismatches raise.
"""

from __future__ import annotations

from .contracts import ChunkSnapshot, Evidence


def extract_quote(text: str, start: int | None, end: int | None) -> str:
    """Return text[start:end] as a codepoint half-open range."""
    if start is None or end is None:
        raise ValueError("span endpoints must both be present for extraction")
    if not 0 <= start <= end <= len(text):
        raise ValueError("invalid codepoint span")
    return text[start:end]


def validate_span(text: str, start: int | None, end: int | None, quote: str) -> None:
    """Validate a quote against the original text.

    With a span: the extracted range must equal the quote exactly.
    Without a span: the quote must appear verbatim somewhere in the text
    (no fabricated positions, no paraphrases). Empty and whitespace-only
    quotes are rejected in both modes.
    """
    if not quote.strip():
        raise ValueError("quote must not be empty or whitespace-only")
    if start is None and end is None:
        if quote not in text:
            raise ValueError("quote not found in original text")
        return
    if start is None or end is None:
        raise ValueError("span endpoints must both be present or both absent")
    if extract_quote(text, start, end) != quote:
        raise ValueError("quote mismatch: span does not reproduce the quote")


def validate_evidence(chunk: ChunkSnapshot, evidence: Evidence) -> None:
    """Full evidence validation against its supporting chunk.

    Rejects: wrong chunk, wrong content hash, non-positive revision,
    half-present spans, tampered quotes (span or substring semantics).
    """
    if evidence.chunk_id != chunk.chunk_id:
        raise ValueError(f"evidence {evidence.evidence_id} cites chunk {evidence.chunk_id!r}, not {chunk.chunk_id!r}")
    if evidence.content_hash != chunk.content_hash:
        raise ValueError(f"evidence {evidence.evidence_id} content hash mismatch")
    if evidence.revision < 1:
        raise ValueError(f"evidence {evidence.evidence_id} revision must be >= 1")
    if not evidence.quote:
        raise ValueError(f"evidence {evidence.evidence_id} quote must not be empty")
    if (evidence.start_char is None) != (evidence.end_char is None):
        raise ValueError(f"evidence {evidence.evidence_id} span endpoints must both be present or both absent")
    validate_span(chunk.text, evidence.start_char, evidence.end_char, evidence.quote)
