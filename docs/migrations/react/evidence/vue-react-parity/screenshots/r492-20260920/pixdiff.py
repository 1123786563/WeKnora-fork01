#!/usr/bin/env python3
"""Pixel diff for Vue/React parity screenshots.

Usage: python3 pixdiff.py <vue.png> <react.png> [diff_out.png]
Prints JSON: overall diff pct, per-row-band diff pct, and bounding box of diffs.
"""
import sys, json
import numpy as np
from PIL import Image, ImageChops

def main(vue_p, react_p, diff_out=None):
    a = Image.open(vue_p).convert("RGB")
    b = Image.open(react_p).convert("RGB")
    if a.size != b.size:
        print(json.dumps({"error": "size mismatch", "vue": a.size, "react": b.size}))
        return
    A = np.asarray(a, dtype=np.int16)
    B = np.asarray(b, dtype=np.int16)
    D = np.abs(A - B).max(axis=2)  # per-pixel max channel delta
    diff_mask = D > 8  # tolerance for antialiasing
    total = diff_mask.size
    ndiff = int(diff_mask.sum())
    pct = round(ndiff / total * 100, 3)

    # per-band stats (12 horizontal bands)
    h = A.shape[0]
    bands = []
    for i in range(12):
        seg = diff_mask[i*h//12:(i+1)*h//12]
        bands.append(round(float(seg.sum())/seg.size*100, 2))

    # bounding box of diff region
    bbox = None
    if ndiff:
        rows = np.any(diff_mask, axis=1)
        cols = np.any(diff_mask, axis=0)
        y0, y1 = np.argmax(rows), h - 1 - np.argmax(rows[::-1])
        x0, x1 = np.argmax(cols), A.shape[1] - 1 - np.argmax(cols[::-1])
        bbox = [int(x0), int(y0), int(x1), int(y1)]

    # cluster approximation: split into 8x8 grid, report cells > 1%
    grid = []
    gh, gw = A.shape[0]//8, A.shape[1]//8
    for gy in range(8):
        for gx in range(8):
            seg = diff_mask[gy*gh:(gy+1)*gh, gx*gw:(gx+1)*gw]
            p = float(seg.sum())/seg.size*100
            if p > 1:
                grid.append({"cell": f"{gy},{gx}", "pct": round(p, 1)})

    if diff_out and ndiff:
        # heatmap: red where diff, dimmed base
        base = (np.asarray(a, dtype=np.float32) * 0.35).astype(np.uint8)
        heat = base.copy()
        heat[diff_mask] = [255, 0, 0]
        Image.fromarray(heat).save(diff_out)

    print(json.dumps({"diff_pct": pct, "diff_pixels": ndiff,
                      "bbox": bbox, "bands_pct": bands, "hot_cells": grid}))

if __name__ == "__main__":
    main(*sys.argv[1:4] if len(sys.argv) > 3 else sys.argv[1:3] + [None])
