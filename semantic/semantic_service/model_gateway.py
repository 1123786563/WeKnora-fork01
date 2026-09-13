"""Controlled model gateway client for the semantic service (A03).

Every model call from the semantic pipeline goes through the Go business
system's internal endpoint (admission, budget, raw usage). No provider
URLs, no long-term keys, no direct provider access from Python.
"""

from __future__ import annotations

import json
import os
import urllib.request

MESSAGES = [{"role": "user", "content": "甲公司控股乙公司？"}]


class ModelGateway:
    """HTTP client for the Go internal model endpoint."""

    def __init__(self, base_url: str, token: str, invocation_counter_url: str | None = None):
        self._base_url = base_url.rstrip("/")
        self._token = token
        self._counter_url = invocation_counter_url

    def invoke(
        self,
        invocation_id: str,
        operation_id: str,
        model_profile_ref: str,
        messages: list,
        budget_ref: str,
    ) -> dict:
        payload = json.dumps({
            "invocation_id": invocation_id,
            "operation_id": operation_id,
            "model_profile_ref": model_profile_ref,
            "messages": messages,
            "budget_ref": budget_ref,
        }).encode("utf-8")
        request = urllib.request.Request(
            self._base_url + "/internal/semantic/model/invoke",
            data=payload,
            headers={
                "Content-Type": "application/json",
                "X-Semantic-Internal-Token": self._token,
            },
            method="POST",
        )
        try:
            with urllib.request.urlopen(request, timeout=30) as response:
                return json.loads(response.read().decode("utf-8"))
        except urllib.error.HTTPError as exc:
            body = exc.read().decode("utf-8", "replace")
            raise RuntimeError(f"model gateway HTTP {exc.code}: {body}") from exc

    def recorded_invocations(self, invocation_id: str) -> int:
        """Count durable rows for the invocation (persisted ledger)."""
        if self._counter_url is None:
            raise RuntimeError("no counter endpoint configured")
        request = urllib.request.Request(
            self._counter_url + "?invocation_id=" + invocation_id,
            headers={"X-Semantic-Internal-Token": self._token},
        )
        with urllib.request.urlopen(request, timeout=10) as response:
            return int(json.loads(response.read().decode("utf-8"))["count"])