"""C03 evidence position validation tests.

Spans are Unicode codepoint half-open ranges over the ORIGINAL chunk text;
both endpoints exist or neither does; quotes without a span must still be
verbatim substrings; tampered hashes/quotes fail loudly.
"""

import pytest

from semantic_service.contracts import ChunkSnapshot, Evidence
from semantic_service.evidence import extract_quote, validate_evidence, validate_span


def test_span_uses_unicode_codepoints():
    assert extract_quote("甲😀乙", 1, 2) == "😀"
    with pytest.raises(ValueError):
        validate_span("甲😀乙", 1, 2, "乙")


def test_invalid_span_is_not_silently_repaired():
    with pytest.raises(ValueError):
        validate_span("甲乙", 2, 1, "")


def test_span_out_of_bounds_rejected():
    with pytest.raises(ValueError):
        extract_quote("甲乙", 1, 3)
    with pytest.raises(ValueError):
        extract_quote("甲乙", -1, 1)
    # zero-width span at end-of-text is a valid empty range
    assert extract_quote("甲乙", 2, 2) == ""


def test_quote_without_span_must_be_substring():
    validate_span("甲公司控股乙公司。", None, None, "控股")
    with pytest.raises(ValueError):
        validate_span("甲公司控股乙公司。", None, None, "不存在")


def test_validate_evidence_happy_path_with_span():
    chunk = ChunkSnapshot(chunk_id="c1", text="甲公司控股乙公司。", content_hash="h1")
    evidence = Evidence(
        evidence_id="e1", document_id="d1", revision=2, chunk_id="c1",
        content_hash="h1", quote="控股", start_char=3, end_char=5)
    validate_evidence(chunk, evidence)


def test_validate_evidence_happy_path_without_span():
    chunk = ChunkSnapshot(chunk_id="c1", text="甲公司控股乙公司。", content_hash="h1")
    evidence = Evidence(
        evidence_id="e1", document_id="d1", revision=1, chunk_id="c1",
        content_hash="h1", quote="控股乙公司", start_char=None, end_char=None)
    validate_evidence(chunk, evidence)


def test_validate_evidence_rejects_wrong_hash():
    chunk = ChunkSnapshot(chunk_id="c1", text="甲公司控股乙公司。", content_hash="h1")
    evidence = Evidence(
        evidence_id="e1", document_id="d1", revision=1, chunk_id="c1",
        content_hash="TAMPERED", quote="控股", start_char=3, end_char=5)
    with pytest.raises(ValueError, match="hash"):
        validate_evidence(chunk, evidence)


def test_validate_evidence_rejects_half_span():
    chunk = ChunkSnapshot(chunk_id="c1", text="甲公司控股乙公司。", content_hash="h1")
    evidence = Evidence(
        evidence_id="e1", document_id="d1", revision=1, chunk_id="c1",
        content_hash="h1", quote="控股", start_char=3, end_char=None)
    with pytest.raises(ValueError, match="both"):
        validate_evidence(chunk, evidence)


def test_validate_evidence_rejects_tampered_quote():
    chunk = ChunkSnapshot(chunk_id="c1", text="甲公司控股乙公司。", content_hash="h1")
    evidence = Evidence(
        evidence_id="e1", document_id="d1", revision=1, chunk_id="c1",
        content_hash="h1", quote="伪造内容", start_char=3, end_char=5)
    with pytest.raises(ValueError, match="mismatch"):
        validate_evidence(chunk, evidence)


def test_validate_evidence_rejects_wrong_chunk():
    chunk = ChunkSnapshot(chunk_id="c1", text="甲公司控股乙公司。", content_hash="h1")
    evidence = Evidence(
        evidence_id="e1", document_id="d1", revision=1, chunk_id="OTHER",
        content_hash="h1", quote="控股", start_char=3, end_char=5)
    with pytest.raises(ValueError, match="chunk"):
        validate_evidence(chunk, evidence)


def test_whitespace_only_quote_rejected():
    chunk = ChunkSnapshot(chunk_id="c1", text="甲公司 控股 乙公司。", content_hash="h1")
    evidence = Evidence(
        evidence_id="e1", document_id="d1", revision=1, chunk_id="c1",
        content_hash="h1", quote=" ", start_char=None, end_char=None)
    with pytest.raises(ValueError, match="quote"):
        validate_evidence(chunk, evidence)


def test_zero_width_span_with_empty_quote_rejected_at_span_layer():
    with pytest.raises(ValueError, match="quote"):
        validate_span("甲乙", 1, 1, "")


def test_validate_evidence_rejects_empty_quote():
    chunk = ChunkSnapshot(chunk_id="c1", text="甲公司控股乙公司。", content_hash="h1")
    evidence = Evidence(
        evidence_id="e1", document_id="d1", revision=1, chunk_id="c1",
        content_hash="h1", quote="", start_char=None, end_char=None)
    with pytest.raises(ValueError, match="quote"):
        validate_evidence(chunk, evidence)
