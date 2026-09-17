/**
 * Shared Mermaid fullscreen viewer toolbar — R463/A3 port of the Vue viewer
 * chrome (frontend/src/utils/mermaidViewer.ts openMermaidFullscreen):
 *
 *   - toolbar buttons (Vue order): zoomIn, zoomOut, reset, download
 *   - zoom STEP 0.2 clamped to [0.2, 10], both from the buttons and the wheel
 *   - reset restores scale 1 and clears the drag translation
 *   - wheel zoom anchors on the pointer and prevents the page scroll
 *   - drag pans the diagram (grab/grabbing cursor, window-level tracking)
 *   - download exports the diagram (PNG at the rendered size in browsers,
 *     serialized SVG Blob when canvas rasterization is unavailable) through a
 *     Blob URL + a[download] click, revoked right after — no network requests
 *
 * The mount is engine-level so every fullscreen consumer (embed answers and
 * the wiki face) shares one implementation; callers keep owning the overlay,
 * the close control, and the i18n labels.
 */

export interface MermaidViewerToolbarLabels {
  zoomIn: string;
  zoomOut: string;
  reset: string;
  download: string;
  downloading: string;
}

export interface MermaidViewerToolbarHandle {
  toolbar: HTMLElement;
  detach: () => void;
}

/** Vue openMermaidFullscreen STEP (frontend/src/utils/mermaidViewer.ts L77). */
const ZOOM_STEP = 0.2;
const ZOOM_MIN = 0.2;
const ZOOM_MAX = 10;
const DOWNLOAD_BASENAME = 'mermaid-diagram';

/** Clamp to the Vue zoom range and round away float dust (0.2 steps). */
const clampScale = (value: number): number => Math.min(ZOOM_MAX, Math.max(ZOOM_MIN, Math.round(value * 100) / 100));

// Vue button chrome (mermaidViewer.ts createBtn L87-95 + the icon markup).
const BUTTON_STYLE =
  'display:flex;align-items:center;justify-content:center;width:36px;height:36px;border:1px solid #e5e7eb;border-radius:6px;background:rgba(255,255,255,0.95);color:#6b7280;cursor:pointer;padding:0;box-shadow:0 2px 8px rgba(0,0,0,0.15);';

const ICONS = {
  zoomIn:
    '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/><line x1="11" y1="8" x2="11" y2="14"/><line x1="8" y1="11" x2="14" y2="11"/></svg>',
  zoomOut:
    '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/><line x1="8" y1="11" x2="14" y2="11"/></svg>',
  reset:
    '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path d="M3 12a9 9 0 1 0 9-9 9.75 9.75 0 0 0-6.74 2.74L3 8"/><path d="M3 3v5h5"/></svg>',
  download:
    '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"></path><polyline points="7 10 12 15 17 10"></polyline><line x1="12" y1="15" x2="12" y2="3"></line></svg>',
} as const;

function createToolbarButton(title: string, icon: string): HTMLButtonElement {
  const button = document.createElement('button');
  button.type = 'button';
  button.title = title;
  button.setAttribute('aria-label', title);
  button.style.cssText = BUTTON_STYLE;
  button.innerHTML = icon;
  // Vue hover feedback (mermaidViewer.ts L92-93).
  button.onmouseenter = () => {
    button.style.background = '#f3f4f6';
    button.style.color = '#374151';
  };
  button.onmouseleave = () => {
    button.style.background = 'rgba(255,255,255,0.95)';
    button.style.color = '#6b7280';
  };
  return button;
}

/** Serialize an SVG subtree (window-scoped so jsdom/SSR tests can run it). */
function serializeSvg(svg: SVGElement): string {
  const Serializer =
    (typeof window !== 'undefined' && window.XMLSerializer) ||
    (globalThis as typeof globalThis & { XMLSerializer?: typeof XMLSerializer }).XMLSerializer;
  if (!Serializer) return svg.outerHTML;
  return new Serializer().serializeToString(svg);
}

/** Trigger the browser download for one Blob (Vue link.click + revoke path). */
function downloadBlob(blob: Blob, filename: string): void {
  const link = document.createElement('a');
  link.download = filename;
  link.href = URL.createObjectURL(blob);
  link.click();
  URL.revokeObjectURL(link.href);
}

/** Vue downloadSvgAsImage (mermaidViewer.ts L10-49): rasterize the svg to PNG
 * on a white background at the rendered size. */
async function downloadSvgAsPng(svgElement: SVGElement, ctx: CanvasRenderingContext2D): Promise<void> {
  const bbox = svgElement.getBoundingClientRect();
  const w = Math.round(bbox.width);
  const h = Math.round(bbox.height);

  const svgClone = svgElement.cloneNode(true) as SVGElement;
  svgClone.setAttribute('width', String(w));
  svgClone.setAttribute('height', String(h));
  const svgData = serializeSvg(svgClone);
  const svgDataUri = `data:image/svg+xml;charset=utf-8,${encodeURIComponent(svgData)}`;

  const canvas = ctx.canvas;
  canvas.width = w;
  canvas.height = h;

  await new Promise<void>((resolve) => {
    const img = new Image();
    img.onload = () => {
      ctx.fillStyle = '#ffffff';
      ctx.fillRect(0, 0, w, h);
      ctx.drawImage(img, 0, 0, w, h);
      canvas.toBlob((blob) => {
        if (blob) downloadBlob(blob, `${DOWNLOAD_BASENAME}.png`);
        resolve();
      }, 'image/png');
    };
    img.onerror = () => resolve();
    img.src = svgDataUri;
  });
}

/** Fallback export: the sanitized svg itself as a Blob download. */
function downloadSvgBlob(svgElement: SVGElement): void {
  const svgData = serializeSvg(svgElement);
  downloadBlob(new Blob([svgData], { type: 'image/svg+xml' }), `${DOWNLOAD_BASENAME}.svg`);
}

/**
 * Mount the Vue viewer toolbar on a fullscreen overlay. The `stage` is the
 * diagram surface the caller laid out centered in the overlay; the toolbar
 * zooms/pans it with `translate(...) scale(...)` (transform-origin: center,
 * which keeps the Vue wheel-anchor arithmetic from mermaidViewer.ts L166-177
 * exact).
 */
export function attachMermaidViewerToolbar(
  overlay: HTMLElement,
  stage: HTMLElement,
  labels: MermaidViewerToolbarLabels,
): MermaidViewerToolbarHandle {
  let scale = 1;
  let translateX = 0;
  let translateY = 0;
  let isDragging = false;
  let dragStartX = 0;
  let dragStartY = 0;
  let dragStartTX = 0;
  let dragStartTY = 0;
  let feedbackTimer: ReturnType<typeof setTimeout> | undefined;

  const toolbar = document.createElement('div');
  toolbar.className = 'wk-mermaid-viewer-toolbar';
  toolbar.style.cssText = 'position:fixed;top:20px;right:20px;display:flex;gap:6px;z-index:10001;';

  const zoomInBtn = createToolbarButton(labels.zoomIn, ICONS.zoomIn);
  const zoomOutBtn = createToolbarButton(labels.zoomOut, ICONS.zoomOut);
  const resetBtn = createToolbarButton(labels.reset, ICONS.reset);
  const downloadBtn = createToolbarButton(labels.download, ICONS.download);
  toolbar.append(zoomInBtn, zoomOutBtn, resetBtn, downloadBtn);

  // Vue auto-fit (mermaidViewer.ts L119-126): shrink oversized diagrams into
  // the viewport with a 60px margin, clamped to [0.5, 10]. Surfaces with no
  // layout (SSR/tests) keep scale 1.
  const margin = 60;
  const viewW = window.innerWidth - margin * 2;
  const viewH = window.innerHeight - margin * 2;
  if (stage.offsetWidth > 0 && stage.offsetHeight > 0) {
    const fitScale = Math.min(viewW / stage.offsetWidth, viewH / stage.offsetHeight);
    scale = Math.max(0.5, Math.min(fitScale, ZOOM_MAX));
  }

  const applyTransform = () => {
    stage.style.transform = `translate(${translateX}px, ${translateY}px) scale(${scale})`;
  };
  applyTransform();

  zoomInBtn.addEventListener('click', (event) => {
    event.stopPropagation();
    scale = clampScale(scale + ZOOM_STEP);
    applyTransform();
  });
  zoomOutBtn.addEventListener('click', (event) => {
    event.stopPropagation();
    scale = clampScale(scale - ZOOM_STEP);
    applyTransform();
  });
  resetBtn.addEventListener('click', (event) => {
    event.stopPropagation();
    scale = 1;
    translateX = 0;
    translateY = 0;
    applyTransform();
  });

  // Download — PNG at the rendered size (Vue) when canvas rasterization is
  // available, otherwise the serialized SVG Blob; the button flashes the
  // "downloading" title (Vue showBtnFeedback).
  downloadBtn.addEventListener('click', (event) => {
    event.stopPropagation();
    const svgEl = stage.querySelector('svg');
    if (!svgEl) return;
    const ctx = document.createElement('canvas').getContext('2d');
    if (ctx) void downloadSvgAsPng(svgEl, ctx);
    else downloadSvgBlob(svgEl);
    const originalTitle = downloadBtn.title;
    downloadBtn.title = labels.downloading;
    if (feedbackTimer) clearTimeout(feedbackTimer);
    feedbackTimer = setTimeout(() => {
      downloadBtn.title = originalTitle;
      feedbackTimer = undefined;
    }, 1500);
  });

  // Wheel zoom anchored on the pointer (mermaidViewer.ts L166-177).
  const onWheel = (event: WheelEvent) => {
    event.preventDefault();
    const oldScale = scale;
    scale = event.deltaY < 0 ? clampScale(scale + ZOOM_STEP) : clampScale(scale - ZOOM_STEP);
    const rect = overlay.getBoundingClientRect();
    const mx = event.clientX - rect.left - rect.width / 2;
    const my = event.clientY - rect.top - rect.height / 2;
    const ratio = 1 - scale / oldScale;
    translateX += (mx - translateX) * ratio;
    translateY += (my - translateY) * ratio;
    applyTransform();
  };
  overlay.addEventListener('wheel', onWheel, { passive: false });

  // Drag panning (mermaidViewer.ts L180-202): press anywhere outside a button.
  const onMouseMove = (event: MouseEvent) => {
    if (!isDragging) return;
    translateX = dragStartTX + (event.clientX - dragStartX);
    translateY = dragStartTY + (event.clientY - dragStartY);
    applyTransform();
  };
  const onMouseUp = () => {
    isDragging = false;
    overlay.style.cursor = 'grab';
  };
  const onMouseDown = (event: MouseEvent) => {
    const target = event.target as Element | null;
    if (target?.closest('button')) return;
    isDragging = true;
    dragStartX = event.clientX;
    dragStartY = event.clientY;
    dragStartTX = translateX;
    dragStartTY = translateY;
    overlay.style.cursor = 'grabbing';
    event.preventDefault();
  };
  overlay.addEventListener('mousedown', onMouseDown);
  window.addEventListener('mousemove', onMouseMove);
  window.addEventListener('mouseup', onMouseUp);

  const detach = () => {
    if (feedbackTimer) clearTimeout(feedbackTimer);
    overlay.removeEventListener('wheel', onWheel);
    overlay.removeEventListener('mousedown', onMouseDown);
    window.removeEventListener('mousemove', onMouseMove);
    window.removeEventListener('mouseup', onMouseUp);
    toolbar.remove();
  };

  return { toolbar, detach };
}
