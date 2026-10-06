package vlm

import (
	"context"
)

// orientationVLM bakes the EXIF orientation of every image into its pixels
// before the bytes reach the backend (issue #3911). Browsers render by the
// Orientation tag while VLM backends read the raw pixel matrix, so images
// tagged 3/6/8 arrived upside down or sideways and full-page OCR came out
// garbled. Normalizing here — the single assembly point shared by all callers
// and all backends (remote API / Ollama / WeKnoraCloud) — leaves the eight
// production call sites untouched.
//
// Normalization is pass-through safe: any image that is not a JPEG, carries no
// EXIF, has Orientation 1, or fails to decode/encode is forwarded unchanged,
// so a broken image never blocks the VLM call itself.
type orientationVLM struct {
	inner VLM
}

func (w *orientationVLM) GetModelName() string { return w.inner.GetModelName() }
func (w *orientationVLM) GetModelID() string   { return w.inner.GetModelID() }

func (w *orientationVLM) Predict(ctx context.Context, imgBytes [][]byte, prompt string) (string, error) {
	if len(imgBytes) > 0 {
		normalized := make([][]byte, len(imgBytes))
		for i, img := range imgBytes {
			normalized[i] = normalizeImageOrientation(img)
		}
		imgBytes = normalized
	}
	return w.inner.Predict(ctx, imgBytes, prompt)
}

// wrapVLMOrientation installs the EXIF orientation normalizer as the innermost
// VLM decorator, so debug logging and tracing observe exactly the bytes sent
// to the backend.
func wrapVLMOrientation(v VLM) VLM {
	return &orientationVLM{inner: v}
}
