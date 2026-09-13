---
name: craft-document
description: Turn staged knowledge material into a report document: author the editable Markdown source (output/report.md), render the DOCX export from that same source (output/report.docx) and verify both (manifest.json). python-docx renders; citations are C01 kc_ ids only. Use when the user asks for a report, 客户介绍, 白皮书 or Word/DOCX export from uploaded knowledge material.
version: 1
---

# Craft Document（D01 报告文档与 DOCX 导出）

You are generating one deliverable round for a craft "craft" document artwork.
Everything you write goes into the workspace "output/" directory. The round
succeeds only when "output/manifest.json" validates against
`internal/craft`'s ValidateDocumentManifest — 生成(generate)/修改(modify)/
预览(preview)/导出(export) must ALL have passed. Anything less is a failed
round: report it as failed, never ship a half-verified document.

## Outputs (exact names)

| file | meaning |
|---|---|
| `output/report.md` | the editable Markdown source (the continuation point of the NEXT round) |
| `output/report.docx` | the SAME-RUN DOCX export rendered from exactly that source |
| `manifest.json` | headings, citation ids, gate checks |

## Non-negotiable rules

1. **Citations are C01 knowledge citation ids ONLY** (`kc_` + 24 lowercase
   hex, the ids the staged material manifest carries). Never invent a
   source, never cite material the run did not stage. A claim without a
   staged source is dropped, not decorated.
2. **Markdown first, DOCX from it, same round.** The DOCX is rendered from
   the exact Markdown you authored; the numbers in both must match
   verbatim. A DOCX without its Markdown source is not a document round.
3. **PDF is not promised.** Do not claim PDF export; do not silently
   produce one instead of the DOCX.

## Pipeline (in order, no shortcuts)

1. **Read the staged material** from `inputs/` (read-only). Build the
   citation set from the material manifest's kc_ ids. Refuse anything over
   20 MiB or an outline over 200 headings / 200 citations: write
   `manifest.json` with the offending check `failed` and stop.
2. **Author `output/report.md`**: a title (`#`), an abstract paragraph,
   sections (`##`, at least 3 for a real introduction), at least one table
   (`|` rows; a LONG table when the material carries many rows) and a
   trailing 来源 section listing every cited kc_ id with its human title.
   Numbers come from the material only.
3. **Render `output/report.docx` with python-docx** (image:
   python-docx 1.1.2). Map the Markdown structurally: `#`→Title heading,
   `##`→Heading 1, paragraphs→paragraphs, tables→`Table Grid` tables.
   Set the Normal/Title/Heading 1 fonts to a CJK font present in the image
   (Noto Sans CJK SC, including the `w:eastAsia` attribute) so Chinese
   runs are not tofu.
4. **Run the export check before anything passes**: re-open the STORED
   `report.docx` with python-docx and verify against the Markdown —
   every heading present (in order), every number verbatim, every kc_
   citation present in the body AND the sources section, table shape
   equal. Also verify the OOXML package itself: styles declare the CJK
   font, every `word/_rels/document.xml.rels` target resolves to a real
   zip part (no dangling images), and `word/document.xml` decodes as
   UTF-8. Any miss → `export: failed`.
5. **Write `output/manifest.json`** exactly in this shape:

```json
{
  "kind": "document",
  "markdown": "report.md",
  "docx": "report.docx",
  "markdown_ref": "resource://report.md",
  "docx_ref": "resource://report.docx",
  "headings": ["智绘云图客户介绍", "公司概况", "产品能力", "实施效果", "来源"],
  "citation_ids": ["kc_8833411d564eabdf5b60e012", "kc_9d6cf870db056f0bb909a0d6"],
  "checks": [
    {"name": "generate", "status": "passed", "detail": "report.md authored: title/abstract/3 sections/8-row table/sources"},
    {"name": "modify", "status": "passed", "detail": "markdown is the continuation source; docx keeps the numbers verbatim"},
    {"name": "preview", "status": "passed", "detail": "markdown + manifest loadable by the document view (safe renderer)"},
    {"name": "export", "status": "passed", "detail": "stored report.docx re-opened and verified against the markdown"}
  ]
}
```

## Failure states (write them, never fake a pass)

- over-limit input (20 MiB / 200 headings / 200 citations) →
  `generate: failed`;
- a citation that is not a staged kc_ id → drop the claim or `generate:
  failed` — never keep an invented id;
- DOCX re-open fails, headings/numbers/citations drift, missing CJK font,
  dangling relationship, mojibake, empty file, corrupt ZIP →
  `export: failed`;
- inconsistent markdown/docx numbers → `modify: failed`.

## Not promised

PDF export, online Word editing, macros/VBA/docm, embedded active content,
or auto-generated citations from thin air. The deliverables are exactly
the verified Markdown source and its same-run DOCX export.