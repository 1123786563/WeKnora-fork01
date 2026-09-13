---
name: craft-slides
description: Turn staged material and an outline into a rendered slide deck (report.pptx) with a print PDF (report.pdf), per-page preview images (pages/page-N.svg), server-converted preview.json and a verification manifest. python-pptx writes the deck; LibreOffice Impress headless renders; pdftocairo exports pages. Use when the user asks for a presentation, slides or PPTX.
version: 1
---

# Craft Slides（D03 演示稿与分页预览）

You are generating one deliverable round for a craft "slides" artwork.
Everything you write goes into the workspace "output/" directory. The round
succeeds only when "output/manifest.json" validates against
`internal/craft`'s ValidateSlidesManifest — render, pages and sources must
ALL have passed and every slide must have a rendered page. Producing the
PPTX ZIP alone is NEVER visual acceptance; anything less is a failed round.

## Outputs (exact names)

| file | meaning |
|---|---|
| `output/report.pptx` | the deliverable deck (plain .pptx, no macros) |
| `output/report.pdf` | the LibreOffice render of the same deck (print fidelity) |
| `output/pages/page-N.svg` | per-page preview image, one for EVERY slide (N = 1..SlideCount, no zero padding) |
| `output/preview.json` | server-converted preview data (page list for the workbench view) |
| `output/index.html` | the static deck viewer page (thumbnails, prev/next, page number, downloads, keyboard arrows, modify-request) |
| `manifest.json` | slide manifest + per-page summaries + gate checks |

## Pipeline (in order, no shortcuts)

1. **Read the staged material** from `inputs/` and the knowledge package
   from `knowledge/manifest.json` (read-only; the citation ids are the
   `kc_…` CitationID values it lists). Build the PAGE OUTLINE FIRST: one
   entry per slide with a working title, the claims it carries and the
   citation ids those claims come from. An outline over 30 pages is NOT
   silently trimmed: record the page numbers beyond the cap in
   `overflow_pages`, render only the first 30, write checks
   `pages: failed` and report the overflow to the main agent for fixing
   (split the deck or trim the outline).
2. **Write the deck with python-pptx** (image: python-pptx 1.0.2 wheel,
   sha256-pinned, over Debian lxml/Pillow). Body text never below 12pt;
   CJK text uses the slide fonts (fonts-noto-cjk is installed). Charts are
   native python-pptx charts (`add_chart`) or declared pictures — a page
   whose outline declares a chart but whose deck carries none fails the
   pages check. Every page's citations go into its SPEAKER NOTES
   (`notes_slide`), and the LAST page is a 来源 page listing every cited
   id + title, so each page is traceable to the original knowledge sources.
3. **Render with LibreOffice Impress headless** (image:
   libreoffice-impress-nogui 7.4.7). Always seed an isolated profile first
   and always run under a timeout budget — a conversion killed by the
   budget (exit >= 124) records `render: failed` and stops; never retry in
   a loop, never ship an unrendered deck:

   ```bash
   mkdir -p /tmp/lo-profile/user
   timeout -k 5 180 soffice --headless --norestore \
     -env:UserInstallation=file:///tmp/lo-profile \
     --convert-to pdf --outdir "${OUT}" "${WORK}/report.pptx"
   ```

4. **Verify the render**: `pdfinfo` page count MUST equal the slide count;
   `pdftotext -f N -l N` per page must contain that page's key strings
   (missing text = fonts/boundary failure). Export EVERY page with
   pdftocairo — SVG output is single-page, so loop:

   ```bash
   n=1
   while [ "$n" -le "$SLIDES" ]; do
     pdftocairo -svg -f "$n" -l "$n" report.pdf pages/page-"$n".svg
     n=$((n+1))
   done
   ```

5. **Machine checks on the deck itself** (python-pptx readback): shapes
   stay inside the slide box (left/top >= 0, right/bottom <= slide size);
   smallest run font >= 12pt; every declared chart/picture exists. A
   failure anywhere records the offending check `failed` — fix and re-run
   the whole pipeline rather than shipping a flagged deck.
6. **Write `output/preview.json`**: `{kind: "slides", slide_count, pages:
   [{index, title, image: "pages/page-N.svg", notes, sources}], overflow_pages}`.
   This file is the ONLY preview source — the browser never parses the
   deck. The static `index.html` viewer projects it: thumbnails, current
   page image, 上一页/下一页, page counter (第 N/M 页), download links for
   report.pptx and report.pdf (version-relative, so the preview origin's
   capability authorizes them), ArrowLeft/ArrowRight keyboard navigation,
   and a 修改此页 button that posts
   `{type: "craft:slides:modify", page: N}` to the parent window — the
   assembly turns it into a request carrying the explicit page number and
   the current version id for the main agent.
7. **Write `output/manifest.json`** exactly in this shape (refs are
   `resource://` + the version-relative path):

   ```json
   {
     "kind": "slides",
     "pptx": "report.pptx",
     "pdf": "report.pdf",
     "preview": "preview.json",
     "pptx_ref": "resource://report.pptx",
     "page_refs": ["resource://pages/page-1.svg", "resource://pages/page-2.svg"],
     "slide_count": 2,
     "overflow_pages": [],
     "pages": [
       {"index": 1, "title": "封面", "notes": "", "sources": [], "min_font_pt": 28, "images": 0},
       {"index": 2, "title": "来源", "notes": "kc_a", "sources": ["kc_a"], "min_font_pt": 12, "images": 0}
     ],
     "sources": [{"id": "kc_a", "title": "素材A"}],
     "checks": [
       {"name": "render", "status": "passed", "detail": "soffice 7.4.7 impress_pdf_Export + pdftocairo 22.12"},
       {"name": "pages", "status": "passed", "detail": "页数/文本边界/缺图/字号全过"},
       {"name": "sources", "status": "passed", "detail": "每页引用均能解析到 knowledge/manifest.json 的 kc_ id"},
       {"name": "visual", "status": "not_run", "detail": "排版美观/视觉层次需要人工验收"}
     ]
   }
   ```

   `sources` lists every citation id the deck uses (they must exist in
   the staged knowledge manifest); a page citing an id outside it fails
   `sources`. The `visual` check stays honestly `not_run` — aesthetics
   are human acceptance; never fake it as passed.

## Failure states (write them, never fake a pass)

- outline over 30 pages → render first 30 only, `overflow_pages: [31, …]`,
  `pages: failed`, report to the main agent for outline fixing;
- conversion killed by its budget → `render: failed`, no PDF/page claims;
- pdfinfo count != slide count → `render: failed` (missing pages are a
  visible failure, never a pass);
- per-page key text missing (fonts/boundary) → `pages: failed`;
- shape out of the slide box / font < 12pt / declared image missing →
  `pages: failed`;
- a page citing an unknown source id → `sources: failed`;
- every rendered page image must exist and be non-empty — a missing
  `pages/page-N.svg` keeps the manifest invalid (SlidesReady requires
  every PageRef).

## Not promised

Online slide editing, animations/transitions, embedded multimedia, macros
(.pptm/VBA), external hyperlinks that execute, or auto-play. The
deliverable is a plain rendered .pptx + .pdf + per-page SVG previews; the
deck's look beyond the machine checks (visual hierarchy, aesthetics) is
explicitly human acceptance (`visual: not_run`).
