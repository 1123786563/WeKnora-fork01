package vlm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"strconv"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
)

// buildQuadrantJPEG encodes a w×h JPEG with one distinct color per quadrant:
//
//	top-left red    | top-right blue
//	bottom-left green | bottom-right yellow
func buildQuadrantJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	fill := func(x0, y0, x1, y1 int, c color.Color) {
		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				img.Set(x, y, c)
			}
		}
	}
	fill(0, 0, w/2, h/2, color.RGBA{R: 255, A: 255})         // top-left: red
	fill(w/2, 0, w, h/2, color.RGBA{B: 255, A: 255})         // top-right: blue
	fill(0, h/2, w/2, h, color.RGBA{G: 180, A: 255})         // bottom-left: green
	fill(w/2, h/2, w, h, color.RGBA{R: 255, G: 255, A: 255}) // bottom-right: yellow
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: jpegQuality95}); err != nil {
		t.Fatalf("encode source JPEG: %v", err)
	}
	return buf.Bytes()
}

// buildEXIFJPEG inserts an APP1 "Exif" segment carrying the given Orientation
// value right after the SOI marker of a plain JPEG. bigEndian selects the
// "MM" TIFF byte order to exercise both paths.
func buildEXIFJPEG(t *testing.T, plainJPEG []byte, orientation uint16, bigEndian bool) []byte {
	t.Helper()
	bo := binary.ByteOrder(binary.LittleEndian)
	if bigEndian {
		bo = binary.BigEndian
	}
	u16 := func(v uint16) []byte { b := make([]byte, 2); bo.PutUint16(b, v); return b }
	u32 := func(v uint32) []byte { b := make([]byte, 4); bo.PutUint32(b, v); return b }

	tiff := &bytes.Buffer{}
	if bigEndian {
		tiff.WriteString("MM")
	} else {
		tiff.WriteString("II")
	}
	tiff.Write(u16(0x002A)) // TIFF magic
	tiff.Write(u32(8))      // IFD0 offset (relative to TIFF header)
	tiff.Write(u16(1))      // one IFD0 entry
	tiff.Write(u16(exifOrientationTag))
	tiff.Write(u16(3)) // SHORT
	tiff.Write(u32(1)) // count
	tiff.Write(u16(orientation))
	tiff.Write(u16(0)) // value padding
	tiff.Write(u32(0)) // next IFD offset

	seg := &bytes.Buffer{}
	seg.Write([]byte{0xFF, 0xE1})
	// JPEG segment lengths are always big-endian, unlike the TIFF payload.
	seg.Write([]byte{0x00, byte(2 + 6 + tiff.Len())}) // length includes itself
	seg.WriteString("Exif\x00\x00")
	seg.Write(tiff.Bytes())

	out := make([]byte, 0, len(plainJPEG)+seg.Len())
	out = append(out, plainJPEG[:2]...) // SOI
	out = append(out, seg.Bytes()...)
	out = append(out, plainJPEG[2:]...)
	return out
}

// assertQuadrants decodes img and checks the center pixel of each quadrant
// against the expected colors (top-left, top-right, bottom-left, bottom-right).
func assertQuadrants(t *testing.T, img image.Image, wantTL, wantTR, wantBL, wantBR color.RGBA) {
	t.Helper()
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	samples := []struct {
		x, y int
		want color.RGBA
		name string
	}{
		{w / 4, h / 4, wantTL, "top-left"},
		{3 * w / 4, h / 4, wantTR, "top-right"},
		{w / 4, 3 * h / 4, wantBL, "bottom-left"},
		{3 * w / 4, 3 * h / 4, wantBR, "bottom-right"},
	}
	const tol = 64 // JPEG round-trip drift on saturated solid colors
	for _, s := range samples {
		r, g, b, _ := img.At(s.x, s.y).RGBA()
		dr := int(uint8(r>>8)) - int(s.want.R)
		dg := int(uint8(g>>8)) - int(s.want.G)
		db := int(uint8(b>>8)) - int(s.want.B)
		if dr < -tol || dr > tol || dg < -tol || dg > tol || db < -tol || db > tol {
			t.Errorf("%s quadrant at (%d,%d) = rgba(%d,%d,%d), want rgba(%d,%d,%d) (tol %d)",
				s.name, s.x, s.y, uint8(r>>8), uint8(g>>8), uint8(b>>8), s.want.R, s.want.G, s.want.B, tol)
		}
	}
}

var (
	srcRed    = color.RGBA{R: 255, A: 255}
	srcBlue   = color.RGBA{B: 255, A: 255}
	srcGreen  = color.RGBA{G: 180, A: 255}
	srcYellow = color.RGBA{R: 255, G: 255, A: 255}
)

// TestNormalizeImageOrientation_RotatesAllOrientations feeds a non-square
// quadrant JPEG tagged with each EXIF orientation 2-8 and verifies the output
// dimensions (swapped for 5-8) and the quadrant content against hand-derived
// expectations, so the mapping is validated independently of the transform
// implementation.
func TestNormalizeImageOrientation_RotatesAllOrientations(t *testing.T) {
	const w, h = 64, 32
	cases := []struct {
		orientation uint16
		swapDims    bool
		// expected output quadrants (TL, TR, BL, BR) in terms of source quadrants
		tl, tr, bl, br color.RGBA
	}{
		{2, false, srcBlue, srcRed, srcYellow, srcGreen}, // flip horizontal
		{3, false, srcYellow, srcGreen, srcBlue, srcRed}, // rotate 180
		{4, false, srcGreen, srcYellow, srcRed, srcBlue}, // flip vertical
		{5, true, srcRed, srcGreen, srcBlue, srcYellow},  // transpose
		{6, true, srcGreen, srcRed, srcYellow, srcBlue},  // rotate 90 CW
		{7, true, srcYellow, srcBlue, srcGreen, srcRed},  // anti-transpose
		{8, true, srcBlue, srcYellow, srcRed, srcGreen},  // rotate 270 CW
	}
	for _, tc := range cases {
		for _, be := range []bool{false, true} {
			name := "LE"
			if be {
				name = "BE"
			}
			t.Run(strconv.Itoa(int(tc.orientation))+"_"+name, func(t *testing.T) {
				tagged := buildEXIFJPEG(t, buildQuadrantJPEG(t, w, h), tc.orientation, be)
				out := normalizeImageOrientation(tagged)
				img, err := jpeg.Decode(bytes.NewReader(out))
				if err != nil {
					t.Fatalf("normalized output is not decodable JPEG: %v", err)
				}
				wantW, wantH := w, h
				if tc.swapDims {
					wantW, wantH = h, w
				}
				if dw, dh := img.Bounds().Dx(), img.Bounds().Dy(); dw != wantW || dh != wantH {
					t.Fatalf("output dims = %dx%d, want %dx%d", dw, dh, wantW, wantH)
				}
				assertQuadrants(t, img, tc.tl, tc.tr, tc.bl, tc.br)
			})
		}
	}
}

// TestNormalizeImageOrientation_PassThrough guards the never-block-the-VLM
// contract: everything that is not a confidence-checked rotate-able JPEG comes
// back byte-identical.
func TestNormalizeImageOrientation_PassThrough(t *testing.T) {
	plain := buildQuadrantJPEG(t, 64, 32)

	// PNG bytes (never EXIF-normalized).
	pngBuf := &bytes.Buffer{}
	if err := png.Encode(pngBuf, image.NewRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatalf("encode PNG: %v", err)
	}

	badEXIF := func(payload []byte) []byte {
		segLen := 2 + len(payload)
		out := append([]byte{0xFF, 0xD8, 0xFF, 0xE1, byte(segLen >> 8), byte(segLen)}, payload...)
		return append(out, plain[2:]...)
	}

	cases := []struct {
		name string
		data []byte
	}{
		{"orientation_1", buildEXIFJPEG(t, plain, 1, false)},
		{"orientation_0_invalid", buildEXIFJPEG(t, plain, 0, false)},
		{"orientation_9_invalid", buildEXIFJPEG(t, plain, 9, false)},
		{"no_exif", plain},
		{"png", pngBuf.Bytes()},
		{"empty", nil},
		{"jpeg_magic_garbage", []byte{0xFF, 0xD8, 0xFF, 0xDA, 0x00, 0x02}},
		{"truncated_tiff_header", badEXIF([]byte("Exif\x00\x00II\x2A"))},
		{"bad_byte_order", badEXIF(append([]byte("Exif\x00\x00XX\x00\x2A\x00\x00\x00\x08"), make([]byte, 16)...))},
		{"bad_tiff_magic", badEXIF(append([]byte("Exif\x00\x00II\x00\x2B\x00\x00\x00\x08"), make([]byte, 16)...))},
		{"ifd_offset_out_of_range", badEXIF(append([]byte("Exif\x00\x00II\x2A\x00\xFF\x00\x00\x08"), make([]byte, 16)...))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizeImageOrientation(tc.data); !bytes.Equal(got, tc.data) {
				t.Errorf("pass-through case returned different bytes: len %d -> %d", len(tc.data), len(got))
			}
		})
	}
}

// recordingVLM is a fake inner VLM that records what the decorator forwards.
type recordingVLM struct {
	calls   [][][]byte
	prompts []string
	err     error
}

func (f *recordingVLM) Predict(_ context.Context, imgBytes [][]byte, prompt string) (string, error) {
	f.calls = append(f.calls, imgBytes)
	f.prompts = append(f.prompts, prompt)
	return "recorded", f.err
}
func (f *recordingVLM) GetModelName() string { return "fake-vlm" }
func (f *recordingVLM) GetModelID() string   { return "fake-vlm-id" }

func TestOrientationVLMNormalizesBeforePredict(t *testing.T) {
	const w, h = 64, 32
	landscape := buildEXIFJPEG(t, buildQuadrantJPEG(t, w, h), 6, false)
	passthrough := buildEXIFJPEG(t, buildQuadrantJPEG(t, w, h), 1, false)

	fake := &recordingVLM{}
	wrapped := wrapVLMOrientation(fake)

	if _, err := wrapped.Predict(t.Context(), [][]byte{landscape, passthrough}, "ocr this"); err != nil {
		t.Fatalf("Predict: %v", err)
	}
	if len(fake.calls) != 1 {
		t.Fatalf("inner Predict calls = %d, want 1", len(fake.calls))
	}
	if fake.prompts[0] != "ocr this" {
		t.Errorf("prompt = %q, want forwarded unchanged", fake.prompts[0])
	}
	got := fake.calls[0]
	if len(got) != 2 {
		t.Fatalf("forwarded image count = %d, want 2", len(got))
	}

	// Orientation 6 image must arrive upright (dimensions swapped).
	img, err := jpeg.Decode(bytes.NewReader(got[0]))
	if err != nil {
		t.Fatalf("first image not decodable: %v", err)
	}
	if dw, dh := img.Bounds().Dx(), img.Bounds().Dy(); dw != h || dh != w {
		t.Errorf("orientation-6 image dims = %dx%d, want %dx%d", dw, dh, h, w)
	}

	// Orientation 1 image must arrive byte-identical.
	if !bytes.Equal(got[1], passthrough) {
		t.Errorf("orientation-1 image was rewritten despite needing no rotation")
	}

	// Metadata must delegate to the inner VLM.
	if wrapped.GetModelName() != "fake-vlm" || wrapped.GetModelID() != "fake-vlm-id" {
		t.Errorf("metadata delegation broken: %q / %q", wrapped.GetModelName(), wrapped.GetModelID())
	}
}

func TestOrientationVLMPreservesEmptyImages(t *testing.T) {
	fake := &recordingVLM{}
	wrapped := wrapVLMOrientation(fake)
	if _, err := wrapped.Predict(t.Context(), nil, "prompt"); err != nil {
		t.Fatalf("Predict: %v", err)
	}
	if len(fake.calls) != 1 || len(fake.calls[0]) != 0 {
		t.Fatalf("empty image list not preserved: %+v", fake.calls)
	}
}

// TestNewVLMWiresOrientationNormalization verifies the assembly point itself
// (issue #3911 fix): a VLM built through NewVLM must send upright pixels over
// the wire, proving the decorator sits in front of the real backend.
func TestNewVLMWiresOrientationNormalization(t *testing.T) {
	withVLMSSRFWhitelist(t, "127.0.0.1")

	var lastRequest map[string]interface{}
	server := newVLMChatTestServer(t, &lastRequest)
	defer server.Close()

	v, err := NewVLM(&Config{
		BaseURL:       server.URL,
		ModelName:     "qwen2.5-vl-7b-instruct",
		APIKey:        "sk-test",
		Source:        types.ModelSourceRemote,
		InterfaceType: "openai",
	}, nil)
	if err != nil {
		t.Fatalf("NewVLM: %v", err)
	}

	tagged := buildEXIFJPEG(t, buildQuadrantJPEG(t, 64, 32), 6, false)
	if _, err := v.Predict(t.Context(), [][]byte{tagged}, "extract the text"); err != nil {
		t.Fatalf("Predict: %v", err)
	}

	messages := lastRequest["messages"].([]interface{})
	parts := messages[0].(map[string]interface{})["content"].([]interface{})
	image := parts[1].(map[string]interface{})["image_url"].(map[string]interface{})
	dataURI := image["url"].(string)
	const prefix = "data:image/jpeg;base64,"
	if len(dataURI) <= len(prefix) || dataURI[:len(prefix)] != prefix {
		t.Fatalf("image url prefix = %q, want JPEG data URI", dataURI[:min(len(dataURI), 40)])
	}
	raw, err := base64.StdEncoding.DecodeString(dataURI[len(prefix):])
	if err != nil {
		t.Fatalf("decode data URI: %v", err)
	}
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("sent image is not decodable JPEG: %v", err)
	}
	if cfg.Width != 32 || cfg.Height != 64 {
		t.Errorf("sent image dims = %dx%d, want 32x64 (orientation 6 applied)", cfg.Width, cfg.Height)
	}
}
