import pytest

from semantic_service.contracts import ChunkSnapshot, Evidence
from semantic_service.evidence import extract_quote, validate_evidence, validate_span


def test_span_uses_unicode_codepoints():
    assert extract_quote("甲😀乙", 1, 2) == "😀"
    assert extract_quote("e\u0301", 1, 2) == "\u0301"
    with pytest.raises(ValueError):
        validate_span("甲😀乙", 1, 2, "乙")


def test_span_rejects_invalid_bounds_without_repair():
    for start, end in ((-1, 0), (2, 1), (0, 4), (True, 1)):
        with pytest.raises(ValueError):
            extract_quote("甲😀乙", start, end)


def test_evidence_requires_matching_chunk_hash_and_quote():
    chunk = ChunkSnapshot("c1", "甲😀乙", "opaque-hash")
    with pytest.raises(ValueError):
        validate_evidence(chunk, Evidence("e1", "d1", 1, "c1", "other-hash", "😀", 1, 2))
    with pytest.raises(ValueError):
        validate_evidence(chunk, Evidence("e2", "d1", 1, "c1", "opaque-hash", "乙", 1, 2))
    with pytest.raises(ValueError):
        validate_evidence(chunk, Evidence("e3", "d1", 1, "wrong-chunk", "opaque-hash", "😀", 1, 2))


def test_unanchored_evidence_must_be_an_exact_substring():
    chunk = ChunkSnapshot("c1", "原文 😀", "h")
    validate_evidence(chunk, Evidence("e1", "d1", 1, "c1", "h", "😀", None, None))
    with pytest.raises(ValueError):
        validate_evidence(chunk, Evidence("e2", "d1", 1, "c1", "h", "缺失", None, None))
    decomposed = ChunkSnapshot("c2", "e\u0301", "h2")
    validate_evidence(decomposed, Evidence("e3", "d1", 1, "c2", "h2", "e\u0301", None, None))
    with pytest.raises(ValueError):
        validate_evidence(decomposed, Evidence("e4", "d1", 1, "c2", "h2", "\u00e9", None, None))


def test_unanchored_evidence_rejects_non_string_text_and_quote():
    with pytest.raises(ValueError):
        validate_evidence(ChunkSnapshot("c1", 1, "h"), Evidence("e1", "d1", 1, "c1", "h", "q", None, None))
    with pytest.raises(ValueError):
        validate_evidence(ChunkSnapshot("c1", "text", "h"), Evidence("e2", "d1", 1, "c1", "h", 1, None, None))
