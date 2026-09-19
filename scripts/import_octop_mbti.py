#!/usr/bin/env python3
"""One-time importer: dump Octop MBTI profiles + test questions to JSON for go:embed.

Re-runnable for auditing data fidelity. Output byte-compares in data_test.go
only for structure; the JSON is the single source for the Go package.
"""
import json
import sys
from pathlib import Path

OCTOP = Path("/Users/wuyongjun/trea/Octop/src/octop")
sys.path.insert(0, str(OCTOP.parent))  # Octop src dir, so `octop.*` resolves

from octop.infra.agents.mbti_profiles import get_all_profiles  # noqa: E402

# Questions live in the router module.
import importlib.util  # noqa: E402
spec = importlib.util.spec_from_file_location(
    "mbti_router", OCTOP / "api" / "routers" / "mbti.py")
mbti_router = importlib.util.module_from_spec(spec)
spec.loader.exec_module(mbti_router)

OUT = Path(__file__).resolve().parents[1] / "internal" / "agent" / "persona" / "data"
OUT.mkdir(parents=True, exist_ok=True)

def axis(t):
    return {"pole": t[0], "percent": t[1]}

profiles = []
for p in get_all_profiles():
    profiles.append({
        "code": p.code, "name_zh": p.name_zh, "name_en": p.name_en,
        "nickname_zh": p.nickname_zh, "summary_zh": p.summary_zh,
        "summary_en": p.summary_en, "descriptors_zh": p.descriptors_zh,
        "descriptors_en": p.descriptors_en,
        "dimensions": {
            "ei": axis(p.dimensions.ei), "sn": axis(p.dimensions.sn),
            "tf": axis(p.dimensions.tf), "jp": axis(p.dimensions.jp)},
        "behavior": {
            "answer_style": p.behavior.answer_style,
            "casual_chat": p.behavior.casual_chat,
            "conflict": p.behavior.conflict,
            "creativity": p.behavior.creativity,
            "emotion": p.behavior.emotion,
            "planning": p.behavior.planning,
            "answer_style_zh": p.behavior.answer_style_zh,
            "casual_chat_zh": p.behavior.casual_chat_zh,
            "conflict_zh": p.behavior.conflict_zh,
            "creativity_zh": p.behavior.creativity_zh,
            "emotion_zh": p.behavior.emotion_zh,
            "planning_zh": p.behavior.planning_zh},
        "color": p.color, "symbol": p.symbol,
    })

questions = [{
    "id": q.id, "dimension": q.dimension, "a_pole": q.a_pole, "b_pole": q.b_pole,
    "question_zh": q.question_zh, "option_a_zh": q.option_a_zh, "option_b_zh": q.option_b_zh,
    "question_en": q.question_en, "option_a_en": q.option_a_en, "option_b_en": q.option_b_en,
} for q in mbti_router._QUESTIONS]

(OUT / "profiles.json").write_text(
    json.dumps(profiles, ensure_ascii=False, indent=1) + "\n", encoding="utf-8")
(OUT / "questions.json").write_text(
    json.dumps(questions, ensure_ascii=False, indent=1) + "\n", encoding="utf-8")
print(f"wrote {len(profiles)} profiles, {len(questions)} questions -> {OUT}")
