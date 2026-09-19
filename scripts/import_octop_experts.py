#!/usr/bin/env python3
"""One-time importer: convert 4 pilot Octop experts (manifest.json + assets)
to WeKnora config/experts/ layout (manifest.yaml). Re-runnable; verbatim assets."""
import json, shutil, sys
from pathlib import Path

OCTOP_LIB = Path("/Users/wuyongjun/trea/Octop/src/octop/infra/agents/experts/library")
OUT = Path(__file__).resolve().parents[1] / "config" / "experts"
PILOTS = {
    "general-assistant": {"skip_skills": ["octop-assistant"]},  # Octop-product skill, meaningless in WeKnora
    "ops-engineer": {},
    "stock-assistant": {},
    "news-trend": {},
}
# manifest.json → manifest.yaml field mapping (see plan Global Constraints):
# id→id; label→label; description→description; icon_name→icon_name; color→color;
# prompt_files→prompt_files; quick_prompts[{title,description,prompt,color,icon_name}]→quick_prompts
# (all LocaleText passthrough zh/en); skills discovered from skills/ dirs minus skip_skills;
# persona_mbti omitted (no Octop equivalent); agent_config: fixed per-expert below.
AGENT_CONFIG = {
    "general-assistant": {"agent_mode": "smart-reasoning", "kb_selection_mode": "all"},
    "ops-engineer":      {"agent_mode": "smart-reasoning", "kb_selection_mode": "all", "web_search_enabled": True},
    "stock-assistant":   {"agent_mode": "smart-reasoning", "web_search_enabled": True},
    "news-trend":        {"agent_mode": "smart-reasoning", "web_search_enabled": True},
}

def to_yaml_obj(m, skills):
    return {
        "id": m["id"],
        "label": m["label"], "description": m["description"],
        "icon_name": m.get("icon_name", "sparkles"), "color": m.get("color", "#6366f1"),
        "prompt_files": m["prompt_files"],
        "quick_prompts": [
            {"title": q["title"], "description": q.get("description", {}),
             "prompt": q["prompt"], "color": q.get("color", ""), "icon_name": q.get("icon_name", "")}
            for q in m.get("quick_prompts", [])
        ],
        "skills": skills,
        "agent_config": AGENT_CONFIG[m["id"]],
    }

for slug, opts in PILOTS.items():
    src = OCTOP_LIB / slug
    m = json.loads((src / "manifest.json").read_text(encoding="utf-8"))
    dst = OUT / slug
    dst.mkdir(parents=True, exist_ok=True)
    for f in m["prompt_files"]:
        shutil.copyfile(src / f, dst / f)
    skills = []
    skdir = src / "skills"
    if skdir.is_dir():
        for d in sorted(skdir.iterdir()):
            if d.is_dir() and (d / "SKILL.md").exists() and d.name not in opts.get("skip_skills", []):
                shutil.copytree(d, dst / "skills" / d.name, dirs_exist_ok=True)
                skills.append(d.name)
    import yaml  # pyyaml
    (dst / "manifest.yaml").write_text(
        yaml.safe_dump(to_yaml_obj(m, skills), allow_unicode=True, sort_keys=False), encoding="utf-8")
    print(f"{slug}: {len(m['prompt_files'])} persona files, skills={skills}")
