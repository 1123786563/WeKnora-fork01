"""O01 telemetry tests: sanitized logging (no chunk text, no prompts,
no credentials in logs or metrics labels)."""

import pytest

from semantic_service.telemetry import sanitize_log_value, SemanticTelemetry


def test_chunk_text_is_redacted():
    value = sanitize_log_value("prefix chunk=甲公司控股乙公司。 full body suffix")
    assert "甲公司控股乙公司" not in value


def test_credentials_are_redacted():
    value = sanitize_log_value("connect postgres://semantic:SECRET_PASS@host/db")
    assert "SECRET_PASS" not in value
    value2 = sanitize_log_value("Authorization: Bearer sk-live-abc123def456")
    assert "sk-live-abc123def456" not in value2


def test_prompt_content_is_redacted():
    value = sanitize_log_value("prompt=You are a knowledge-graph analyst. USER_QUERY_BEGIN 甲控制丙? USER_QUERY_END")
    assert "甲控制丙" not in value


def test_metrics_use_only_low_cardinality_labels():
    telemetry = SemanticTelemetry()
    telemetry.record_operation("search", truncated=False, duration_ms=42)
    telemetry.record_operation("search", truncated=True, duration_ms=99)
    counters = telemetry.operation_counters()
    assert counters["search"] == 2
    # Labels are fixed (operation/truncated) - tenant/KB NEVER become
    # metric labels (they live in controlled trace logs only).
    labels = telemetry.known_label_names()
    assert "tenant" not in labels and "kb" not in labels
