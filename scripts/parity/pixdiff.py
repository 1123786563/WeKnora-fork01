#!/usr/bin/env python3
"""Pixel diff for Vue/React parity screenshots.

Usage: python3 pixdiff.py <vue.png> <react.png> [diff_out.png]
Prints JSON: overall diff pct, per-band diff pct, bounding box, hot cells.
Lives in scripts/parity/ (git-tracked). Uses numpy when available; falls back
to a pure-PIL implementation (diff pct + bbox only) so the daily cron keeps
working on hosts whose default python3 lacks numpy.
"""
import sys, json
from PIL import Image, ImageChops

try:
    import numpy as np
except ImportError:
    np = None


def main_numpy(vue_p, react_p, diff_out):
    a = Image.open(vue_p).convert("RGB")
    b = Image.open(react_p).convert("RGB")
    if a.size != b.size:
        return {"error": "size mismatch", "vue": a.size, "react": b.size}
    A = np.asarray(a, dtype=np.int16)
    B = np.asarray(b, dtype=np.int16)
    D = np.abs(A - B).max(axis=2)
    diff_mask = D > 8
    total = diff_mask.size
    ndiff = int(diff_mask.sum())
    pct = round(ndiff / total * 100, 3)

    h = A.shape[0]
    bands = []
    for i in range(12):
        seg = diff_mask[i*h//12:(i+1)*h//12]
        bands.append(round(float(seg.sum())/seg.size*100, 2))

    bbox = None
    if ndiff:
        rows = np.any(diff_mask, axis=1)
        cols = np.any(diff_mask, axis=0)
        y0, y1 = np.argmax(rows), h - 1 - np.argmax(rows[::-1])
        x0, x1 = np.argmax(cols), A.shape[1] - 1 - np.argmax(cols[::-1])
        bbox = [int(x0), int(y0), int(x1), int(y1)]

    grid = []
    gh, gw = A.shape[0]//8, A.shape[1]//8
    for gy in range(8):
        for gx in range(8):
            seg = diff_mask[gy*gh:(gy+1)*gh, gx*gw:(gx+1)*gw]
            p = float(seg.sum())/seg.size*100
            if p > 1:
                grid.append({"cell": f"{gy},{gx}", "pct": round(p, 1)})

    if diff_out and ndiff:
        base = (np.asarray(a, dtype=np.float32) * 0.35).astype(np.uint8)
        heat = base.copy()
        heat[diff_mask] = [255, 0, 0]
        Image.fromarray(heat).save(diff_out)

    return {"diff_pct": pct, "diff_pixels": ndiff,
            "bbox": bbox, "bands_pct": bands, "hot_cells": grid}


def main_pil(vue_p, react_p, diff_out):
    """numpy-free fallback: threshold-8 diff count via histogram + bbox.

    ImageChops.difference max channel == the numpy D matrix exactly; the
    256-bin histogram of its grayscale approximation differs from the true
    max-channel count, so use the per-channel split and take per-pixel max
    via point() composition (still exact, no numpy).
    """
    a = Image.open(vue_p).convert("RGB")
    b = Image.open(react_p).convert("RGB")
    if a.size != b.size:
        return {"error": "size mismatch", "vue": a.size, "react": b.size}
    diff = ImageChops.difference(a, b)
    # per-pixel max channel delta: max(max(r,g),b) via ImageChops.lighter
    r, g, bl = diff.split()
    rg = ImageChops.lighter(r, g)
    maxd = ImageChops.lighter(rg, bl)
    hist = maxd.histogram()
    ndiff = sum(hist[9:])  # delta > 8
    total = a.size[0] * a.size[1]
    pct = round(ndiff / total * 100, 3)
    bbox = None
    if ndiff:
        x0, y0, x1, y1 = maxd.point(lambda v: 255 if v > 8 else 0).getbbox()
        bbox = [x0, y0, x1, y1]
    if diff_out and ndiff:
        base = a.point(lambda v: int(v * 0.35))
        mask = maxd.point(lambda v: 255 if v > 8 else 0)
        red = Image.new("RGB", a.size, (255, 0, 0))
        base.paste(red, mask=mask)
        base.save(diff_out)
    return {"diff_pct": pct, "diff_pixels": ndiff, "bbox": bbox,
            "bands_pct": None, "hot_cells": [], "engine": "pil-fallback"}


def main(vue_p, react_p, diff_out=None):
    if np is not None:
        out = main_numpy(vue_p, react_p, diff_out)
    else:
        out = main_pil(vue_p, react_p, diff_out)
    print(json.dumps(out))


if __name__ == "__main__":
    main(*sys.argv[1:4] if len(sys.argv) > 3 else sys.argv[1:3] + [None])
