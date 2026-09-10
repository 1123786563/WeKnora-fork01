# T24 static performance sample — 2026-09-11

## Scope and method

- Worktree commit: `a62ad2d`.
- Same local Nginx image/config and same host-network execution for both
  renderers; only the document-root artifact changed.
- Five uncached `curl` samples were taken for `/` and the primary JS entry.
- This is a static transport sample, not a browser paint, real API first-token,
  long-message rendering, or application memory benchmark.

## Results

| Renderer | HTML entry | HTML bytes | HTML median TTFB | Primary JS | JS bytes | JS median TTFB |
|---|---:|---:|---:|---|---:|---:|
| React candidate | `/` | 408 | 1.382 ms | `/assets/index-CCoUCqKy.js` | 311,536 | 1.748 ms |
| Vue fallback | `/` | 4,657 | 0.651 ms | `/assets/main-BOfxS23q.js` | 158,489 | 0.580 ms |

Artifact directory samples:

```text
React static tree: 572 KiB, 8 files
Vue static tree:   26,356 KiB, 184 files
```

All sampled requests returned `200`. The React and Vue primary entry paths
were discovered from their respective HTML rather than assumed from the
source tree.

## Interpretation and missing evidence

The sample is useful for transport/package comparison only. It shows the
candidate's static tree is much smaller, while the primary React entry is
larger than the primary Vue entry; neither number predicts full browser startup
or feature parity because the React candidate has a different chunk graph.

Not collected here: browser FCP/LCP, memory during long-message rendering,
real backend first token, SSE recovery latency, upload latency, or a role/tenant
matrix. Those remain T24 evidence gaps and no regression threshold is claimed
from this sample.
