#!/usr/bin/env python3
"""逐带残差分析：把 pixdiff 输出的差异像素按连续 y 带聚合，输出每带的
y 范围 / x 范围 / 差异像素数。用法：band-report.py <diff-png>（差异图为
白底、差异像素非白）。容差与 pixdiff.py 一致由 diff 图直接反映。"""
import sys
import numpy as np
from PIL import Image

def main(path):
    img = np.asarray(Image.open(path).convert('RGB')).astype(np.int16)
    # pixdiff.py 输出热图：差异像素标红 (255,0,0)，底图为 0.35x 原图
    mask = (img[:, :, 0] > 200) & (img[:, :, 1] < 80) & (img[:, :, 2] < 80)
    rows = mask.any(axis=1)
    bands = []
    y = 0
    h = mask.shape[0]
    while y < h:
        if not rows[y]:
            y += 1
            continue
        y0 = y
        while y < h and rows[y]:
            y += 1
        seg = mask[y0:y]
        cols = seg.any(axis=0)
        xs = np.where(cols)[0]
        bands.append((y0, y - 1, int(xs.min()), int(xs.max()), int(seg.sum())))
    total = int(mask.sum())
    print(f"total diff px: {total} / {mask.shape[0]*mask.shape[1]} = {100*total/(mask.shape[0]*mask.shape[1]):.3f}%")
    print("bands (y0-y1, x0-x1, px):")
    for (y0, y1, x0, x1, n) in bands:
        print(f"  y[{y0}-{y1}] x[{x0}-{x1}] {n}px")

if __name__ == '__main__':
    main(sys.argv[1])
