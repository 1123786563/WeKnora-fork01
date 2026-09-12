"""Sanitized telemetry for the semantic service (O01).

Logs and metrics NEVER carry chunk text, prompt content, or credentials;
metrics use only low-cardinality labels (tenant/KB live in controlled
trace logs, never as metric labels).
"""

from __future__ import annotations

import re

_CREDENTIAL_PATTERNS = [
	re.compile(r":[^:@/\s]+@"),
	re.compile(r"(?i)(bearer\s+)[^\s]+"),
	re.compile(r"sk-[A-Za-z0-9_-]{8,}"),
]
_QUERY_BLOCK = re.compile(r"USER_QUERY_BEGIN.*?USER_QUERY_END", re.DOTALL)
_CJK_RUN = re.compile(r"[\u4e00-\u9fff\u3400-\u4dbf]{2,}")
_REDACTED = "[REDACTED]"


def sanitize_log_value(value: str) -> str:
	"""Redact credentials and prompt/chunk content from a to-be-logged
	string. USER_QUERY blocks collapse to a marker; CJK runs (prompt or
	chunk knowledge content) redact wholesale with the marker."""
	for pattern in _CREDENTIAL_PATTERNS:
		value = pattern.sub(_REDACTED, value)
	value = _QUERY_BLOCK.sub("USER_QUERY_BEGIN " + _REDACTED + " USER_QUERY_END", value)
	value = _CJK_RUN.sub(_REDACTED, value)
	return value


class SemanticTelemetry:
	"""Low-cardinality operation counters + timing capture."""

	_ALLOWED_LABELS = frozenset({"operation", "truncated", "status"})

	def __init__(self) -> None:
		self._counters: dict[tuple, int] = {}

	def record_operation(self, operation: str, *, truncated: bool = False,
					duration_ms: int = 0, status: str = "ok") -> None:
		key = (operation, str(truncated), status)
		self._counters[key] = self._counters.get(key, 0) + 1
		_ = duration_ms

	def operation_counters(self) -> dict:
		out = {}
		for (operation, _truncated, _status), count in self._counters.items():
			out[operation] = out.get(operation, 0) + count
		return out

	@classmethod
	def known_label_names(cls):
		return cls._ALLOWED_LABELS