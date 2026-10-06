package vlm

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/jpeg"
)

// EXIF orientation normalization (issue #3911).
//
// Browsers honor the EXIF Orientation tag when rendering, so the UI shows
// scanned pages upright. VLM backends consume the raw pixel matrix and broadly
// ignore the tag, so photos/scans carrying Orientation 3/6/8 reach the model
// upside down or sideways and full-page OCR comes out garbled.
//
// The fix bakes the orientation into the pixels before the bytes reach any
// backend. Everything here is stdlib-only: a minimal JPEG APP1 / TIFF IFD0
// reader for tag 0x0112 plus pixel-space flips/rotations. Any image we cannot
// confidently read or rewrite is passed through untouched — a broken EXIF
// block must never block the VLM call itself.

const (
	exifOrientationTag = 0x0112
	jpegQuality95      = 95
)

// normalizeImageOrientation returns data with its EXIF orientation baked into
// the pixels. It is pass-through safe: non-JPEG data, a missing or malformed
// EXIF block, Orientation 1/absent/invalid, or any decode/encode failure all
// return the original bytes unchanged.
func normalizeImageOrientation(data []byte) []byte {
	orientation, ok := jpegEXIFOrientation(data)
	if !ok || orientation < 2 || orientation > 8 {
		return data
	}
	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		return data
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, applyOrientation(img, orientation), &jpeg.Options{Quality: jpegQuality95}); err != nil {
		return data
	}
	return buf.Bytes()
}

// jpegEXIFOrientation extracts the EXIF Orientation tag from the first APP1
// "Exif" segment of a JPEG. ok is false for anything the parser cannot
// confidently read: non-JPEG data, no Exif APP1 segment, or a malformed
// TIFF header / IFD.
func jpegEXIFOrientation(data []byte) (orientation uint16, ok bool) {
	// JPEG SOI marker.
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return 0, false
	}
	offset := 2
	for offset+4 <= len(data) {
		if data[offset] != 0xFF {
			return 0, false // desynchronized marker stream
		}
		marker := data[offset+1]
		if marker == 0xD8 || marker == 0x01 || (marker >= 0xD0 && marker <= 0xD7) {
			// Standalone markers carry no length field.
			offset += 2
			continue
		}
		if marker == 0xD9 || marker == 0xDA {
			// EOI / start of scan: no metadata beyond this point.
			return 0, false
		}
		segLen := int(data[offset+2])<<8 | int(data[offset+3])
		if segLen < 2 || offset+2+segLen > len(data) {
			return 0, false
		}
		payload := data[offset+4 : offset+2+segLen]
		if marker == 0xE1 && len(payload) >= 6 && bytes.Equal(payload[:6], []byte("Exif\x00\x00")) {
			return tiffIFD0Orientation(payload[6:])
		}
		offset += 2 + segLen
	}
	return 0, false
}

// tiffIFD0Orientation reads the Orientation tag from the TIFF header / IFD0 at
// the start of tiff. Orientation lives in IFD0, so thumbnail IFDs are never
// followed.
func tiffIFD0Orientation(tiff []byte) (orientation uint16, ok bool) {
	if len(tiff) < 8 {
		return 0, false
	}
	var bo binary.ByteOrder
	switch {
	case tiff[0] == 'I' && tiff[1] == 'I':
		bo = binary.LittleEndian
	case tiff[0] == 'M' && tiff[1] == 'M':
		bo = binary.BigEndian
	default:
		return 0, false
	}
	if bo.Uint16(tiff[2:4]) != 0x002A {
		return 0, false
	}
	ifdOff := bo.Uint32(tiff[4:8])
	if ifdOff < 8 || ifdOff+2 > uint32(len(tiff)) {
		return 0, false
	}
	count := int(bo.Uint16(tiff[ifdOff : ifdOff+2]))
	entriesStart := int(ifdOff) + 2
	if entriesStart+count*12+4 > len(tiff) {
		return 0, false // IFD overruns the segment: corrupt
	}
	for i := 0; i < count; i++ {
		entry := tiff[entriesStart+i*12 : entriesStart+i*12+12]
		if bo.Uint16(entry[0:2]) != exifOrientationTag {
			continue
		}
		if bo.Uint32(entry[4:8]) != 1 {
			return 0, false
		}
		// A count-1 value fits in the 4-byte inline value field.
		switch bo.Uint16(entry[2:4]) {
		case 1: // BYTE
			return uint16(entry[8]), true
		case 3: // SHORT
			return bo.Uint16(entry[8:10]), true
		case 4: // LONG
			return uint16(bo.Uint32(entry[8:12])), true
		}
		return 0, false
	}
	return 0, false
}

// applyOrientation transforms src so it appears upright when displayed,
// following the EXIF Orientation semantics:
//
//	2 = flip horizontal      3 = rotate 180          4 = flip vertical
//	5 = transpose            6 = rotate 90 CW        7 = anti-transpose
//	8 = rotate 270 CW (90 CCW)
//
// Orientations 5-8 swap the width and height.
func applyOrientation(src image.Image, orientation uint16) *image.RGBA {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	newW, newH := w, h
	if orientation >= 5 && orientation <= 8 {
		newW, newH = h, w
	}
	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var dx, dy int
			switch orientation {
			case 2:
				dx, dy = w-1-x, y
			case 3:
				dx, dy = w-1-x, h-1-y
			case 4:
				dx, dy = x, h-1-y
			case 5:
				dx, dy = y, x
			case 6:
				dx, dy = h-1-y, x
			case 7:
				dx, dy = h-1-y, w-1-x
			case 8:
				dx, dy = y, w-1-x
			default:
				dx, dy = x, y
			}
			dst.Set(dx, dy, src.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}
