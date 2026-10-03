package career

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Export statuses and receipt kinds (T16). One publish renders a PDF and a
// DOCX from the same structured body of one immutable material version; both
// files record the same content digest and version binding. Only an export
// whose two formats passed independent verification is submittable.
const (
	ExportStatusStaged      = "staged"
	ExportStatusSubmittable = "submittable"
	ExportStatusFailed      = "failed"
	ExportStatusRevoked     = "revoked"

	MaterialKindPublished     = "material_published"
	MaterialKindExportRevoked = "material_export_revoked"

	ExportFormatPDF  = "pdf"
	ExportFormatDOCX = "docx"

	ExportFailureVerification = "export_verification_failed"

	materialFingerprintPublish = "publish_material"
	materialFingerprintRevoke  = "revoke_material_export"

	pdfExportExtension  = ".pdf"
	docxExportExtension = ".docx"
)

// MaxExportGrantTTL caps how far in the future a career export grant may be
// issued, mirroring the Workbench artifact grant policy (T04).
const MaxExportGrantTTL = 15 * time.Minute

var (
	ErrExportNotFound           = errors.New("career material export not found")
	ErrExportNotSubmittable     = errors.New("career material export is not submittable")
	ErrExportGrantInvalid       = errors.New("career material export grant invalid")
	ErrExportStorageUnavailable = errors.New("career material export storage unavailable")
	ErrExportSigningKeyMissing  = errors.New("career export signing key not configured")
)

// ExportSigningKeyEnvVar is the only source of the career export HMAC secret.
// At least 32 bytes of hex entropy are required; values are read at wiring
// time so rotation only needs a restart-free env change plus new grants.
const ExportSigningKeyEnvVar = "WEKNORA_CAREER_EXPORT_SIGNING_KEY"

// ExportSigningKeyFromEnv loads the career export HMAC secret.
func ExportSigningKeyFromEnv() ([]byte, error) {
	raw := strings.TrimSpace(os.Getenv(ExportSigningKeyEnvVar))
	if raw == "" {
		return nil, ErrExportSigningKeyMissing
	}
	decoded, err := hex.DecodeString(raw)
	if err != nil || len(decoded) < 32 {
		return nil, fmt.Errorf("%s must be at least 32 bytes of hex entropy", ExportSigningKeyEnvVar)
	}
	return decoded, nil
}

// PublishMaterialInput publishes one immutable material version as a same-body
// PDF/DOCX pair. The request ID and expected revision follow the house
// idempotency and CAS semantics.
type PublishMaterialInput struct {
	RequestID        string `json:"requestId"`
	MaterialID       string `json:"materialId"`
	Version          uint64 `json:"version"`
	ExpectedRevision uint64 `json:"expectedRevision"`
}

// RevokeMaterialExportInput revokes one export; already-issued download grants
// fail immediately afterwards.
type RevokeMaterialExportInput struct {
	RequestID        string `json:"requestId"`
	MaterialID       string `json:"materialId"`
	ExportID         string `json:"exportId"`
	ExpectedRevision uint64 `json:"expectedRevision"`
}

// ExportedFile is one rendered format of an export: the immutable binding
// (material, version, content digest) plus the stored bytes' own digest.
type ExportedFile struct {
	Format        string `json:"format"`
	MaterialID    string `json:"materialId"`
	Version       uint64 `json:"version"`
	ContentDigest string `json:"contentDigest"`
	ObjectKey     string `json:"objectKey,omitempty"`
	FileDigest    string `json:"fileDigest,omitempty"`
	Size          int64  `json:"size,omitempty"`
	Verified      bool   `json:"verified"`
	Error         string `json:"error,omitempty"`
}

// ExportReceipt is the frozen publish/revoke receipt of the export seam.
type ExportReceipt struct {
	Kind           string         `json:"kind"`
	RequestID      string         `json:"requestId"`
	ExportID       string         `json:"exportId"`
	MaterialID     string         `json:"materialId"`
	Version        uint64         `json:"version"`
	Status         string         `json:"status"`
	Submittable    bool           `json:"submittable"`
	ContentDigest  string         `json:"contentDigest"`
	Files          []ExportedFile `json:"files"`
	FailureCode    string         `json:"failureCode,omitempty"`
	FailureMessage string         `json:"failureMessage,omitempty"`
	CreatedAt      time.Time      `json:"createdAt"`
	RevokedAt      *time.Time     `json:"revokedAt,omitempty"`
}

// ExportDownload is a short-lived download grant bound to the issuing owner,
// the export, and one format.
type ExportDownload struct {
	ExportID   string `json:"exportId"`
	MaterialID string `json:"materialId"`
	Version    uint64 `json:"version"`
	Format     string `json:"format"`
	Digest     string `json:"digest"`
	Size       int64  `json:"size"`
	ExpiresAt  int64  `json:"expiresAt"`
	Signature  string `json:"signature"`
	URL        string `json:"url"`
}

// materialExportRecord is the durable export: both format records, the shared
// content digest, the state machine position, and the revocation audit.
type materialExportRecord struct {
	ID            string `gorm:"primaryKey;size:36"`
	TenantID      uint64 `gorm:"uniqueIndex:career_material_export_scope_request;index:idx_career_material_export_scope"`
	UserID        string `gorm:"uniqueIndex:career_material_export_scope_request;index:idx_career_material_export_scope;size:512"`
	MaterialID    string `gorm:"size:36;index:idx_career_material_export_scope"`
	RequestID     string `gorm:"size:128;uniqueIndex:career_material_export_scope_request"`
	Version       uint64 `gorm:"not null"`
	Fingerprint   string `gorm:"size:64;not null"`
	ContentDigest string `gorm:"size:64;not null"`
	Status        string `gorm:"size:16;not null"`
	PDFObjectKey  string `gorm:"type:text;not null;default:''"`
	PDFDigest     string `gorm:"size:64;not null;default:''"`
	PDFSize       int64  `gorm:"not null;default:0"`
	PDFError      string `gorm:"type:text;not null;default:''"`
	DOCXObjectKey string `gorm:"type:text;not null;default:''"`
	DOCXDigest    string `gorm:"size:64;not null;default:''"`
	DOCXSize      int64  `gorm:"not null;default:0"`
	DOCXError     string `gorm:"type:text;not null;default:''"`
	RevokedAt     *time.Time
	ReceiptBody   string    `gorm:"type:text;not null"`
	CreatedAt     time.Time `gorm:"not null"`
	UpdatedAt     time.Time `gorm:"not null"`
}

func (materialExportRecord) TableName() string { return "career_material_exports" }

// materialExportStorage is the narrow storage seam: object keys follow the
// existing local:// pattern owned by the file service. DeleteExport is
// idempotent: removing an already-missing object succeeds, because a retry
// of a deletion step must never wedge on its own earlier progress.
type materialExportStorage interface {
	SaveExport(ctx context.Context, tenantID uint64, name string, data []byte) (string, error)
	ReadExport(ctx context.Context, objectKey string) ([]byte, error)
	DeleteExport(ctx context.Context, objectKey string) error
}

// SetExportStorage binds the export storage seam. Production wires the shared
// file service; tests inject a hermetic store.
func (o *Office) SetExportStorage(storage materialExportStorage) { o.exportStorage = storage }

// cleanupOrphanExportObjects removes stored objects whose durable rows never
// committed (conflict, replay, or failure). It is best effort by design: a
// leaked object is recoverable garbage, and cleanup must never mask the
// original error the caller is about to report.
func (o *Office) cleanupOrphanExportObjects(ctx context.Context, keys ...string) {
	if o.exportStorage == nil {
		return
	}
	for _, key := range keys {
		if key == "" {
			continue
		}
		if err := o.exportStorage.DeleteExport(ctx, key); err != nil {
			slog.Warn("career export object cleanup pending", "object_key", key, "error", err.Error())
		}
	}
}

func (o *Office) deleteExportObjects(ctx context.Context, keys ...string) error {
	if o.exportStorage == nil {
		return ErrExportStorageUnavailable
	}
	for _, key := range keys {
		if key == "" {
			continue
		}
		if err := o.exportStorage.DeleteExport(ctx, key); err != nil {
			return fmt.Errorf("compensate career export object %s: %w", key, err)
		}
	}
	return nil
}

// SetExportSigningKey binds the HMAC secret for download grants. Grants fail
// closed while no key is configured.
func (o *Office) SetExportSigningKey(key []byte) { o.exportSigningKey = key }

// fileServiceExportStorage adapts interfaces.FileService to the export seam.
type fileServiceExportStorage struct {
	files interfaces.FileService
}

func newFileExportStorage(files interfaces.FileService) *fileServiceExportStorage {
	return &fileServiceExportStorage{files: files}
}

func (f *fileServiceExportStorage) SaveExport(ctx context.Context, tenantID uint64, name string, data []byte) (string, error) {
	return f.files.SaveBytes(ctx, data, tenantID, name, false)
}

func (f *fileServiceExportStorage) ReadExport(ctx context.Context, objectKey string) ([]byte, error) {
	reader, err := f.files.GetFile(ctx, objectKey)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

// DeleteExport physically removes one stored export object so a deletion or
// a failed publish cannot leave resume-content bytes on disk forever. An
// object that is already gone counts as deleted.
func (f *fileServiceExportStorage) DeleteExport(ctx context.Context, objectKey string) error {
	if err := f.files.DeleteFile(ctx, objectKey); err != nil {
		if errors.Is(err, fs.ErrNotExist) || os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return nil
}

// careerExportGrant is the authorization fact behind a short-lived export
// download link. The HMAC binds tenant, owner, material, export, format, and
// expiry; redemption re-checks the durable export state.
type careerExportGrant struct {
	TenantID   uint64
	UserID     string
	MaterialID string
	ExportID   string
	Format     string
	ExpiresAt  int64
}

// Canonical returns a versioned, unambiguous representation of the grant.
func (g careerExportGrant) Canonical() (string, error) {
	if g.TenantID == 0 || g.UserID == "" || g.MaterialID == "" || g.ExportID == "" || g.Format == "" || g.ExpiresAt <= 0 {
		return "", errors.New("career export grant: tenant, owner, material, export, format and expiry required")
	}
	for _, value := range []string{g.UserID, g.MaterialID, g.ExportID, g.Format} {
		if strings.ContainsAny(value, "|") {
			return "", errors.New("career export grant: field contains forbidden character")
		}
	}
	return strings.Join([]string{
		"wk-career-export-v1",
		strconv.FormatUint(g.TenantID, 10), g.UserID, g.MaterialID, g.ExportID, g.Format,
		strconv.FormatInt(g.ExpiresAt, 10),
	}, "|"), nil
}

func careerExportGrantSignature(secret []byte, grant careerExportGrant) (string, error) {
	if len(secret) < 32 {
		return "", errors.New("career export signing key too short")
	}
	canonical, err := grant.Canonical()
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(canonical))
	return hex.EncodeToString(mac.Sum(nil)), nil
}

func verifyCareerExportGrant(secret []byte, grant careerExportGrant, signature string, now time.Time) error {
	if grant.ExpiresAt <= now.Unix() {
		return ErrExportGrantInvalid
	}
	expected, err := careerExportGrantSignature(secret, grant)
	if err != nil {
		return err
	}
	if !hmac.Equal([]byte(strings.ToLower(signature)), []byte(expected)) {
		return ErrExportGrantInvalid
	}
	return nil
}

// materialContentDigest is the canonical digest of one structured body: both
// rendered files of an export record exactly this value.
func materialContentDigest(body MaterialBody) string {
	sum := sha256.Sum256(mustJSON(body))
	return hex.EncodeToString(sum[:])
}

// ---- PDF rendering (pure Go stdlib, deterministic) ----

// The PDF writer emits a minimal, deterministic PDF 1.7 file: a Type0 font
// with Identity-H encoding whose CIDs are the UTF-16 code units of the text
// and a generated identity ToUnicode CMap, one content stream per page. No
// timestamps or random identifiers appear anywhere, so the same body renders
// to byte-identical output.
const (
	pdfPageWidth    = 595.0
	pdfPageHeight   = 842.0
	pdfMarginX      = 50.0
	pdfMarginTop    = 48.0
	pdfMarginBottom = 48.0
	pdfBodyFont     = 11
	pdfHeadingFont  = 13
)

type pdfLine struct {
	text   string
	font   int
	before float64
}

// isPDFWideRune reports whether a rune occupies a full-width cell for layout.
func isPDFWideRune(r rune) bool {
	switch {
	case r >= 0x1100 && r <= 0x115F, // Hangul Jamo
		r >= 0x2E80 && r <= 0x9FFF, // CJK blocks
		r >= 0x3000 && r <= 0x303F, // CJK punctuation
		r >= 0xF900 && r <= 0xFAFF, // CJK compatibility ideographs
		r >= 0xFE30 && r <= 0xFE4F, // CJK compatibility forms
		r >= 0xFF00 && r <= 0xFF60, // full-width forms
		r >= 0x2018 && r <= 0x201D: // curly quotes
		return true
	}
	return false
}

func pdfRuneWidth(r rune, font int) float64 {
	if isPDFWideRune(r) {
		return float64(font)
	}
	return float64(font) * 0.5
}

// pdfWrapText breaks one paragraph into lines that fit the text column.
func pdfWrapText(text string, font int) []string {
	if text == "" {
		return []string{""}
	}
	maxWidth := pdfPageWidth - 2*pdfMarginX
	var lines []string
	var current strings.Builder
	width := 0.0
	for _, r := range text {
		runeWidth := pdfRuneWidth(r, font)
		if width > 0 && width+runeWidth > maxWidth {
			lines = append(lines, current.String())
			current.Reset()
			width = 0
		}
		current.WriteRune(r)
		width += runeWidth
	}
	return append(lines, current.String())
}

func pdfLineHeight(font int) float64 {
	if font >= pdfHeadingFont {
		return 18
	}
	return 15
}

// layoutMaterialLines flattens the structured body into laid-out lines.
func layoutMaterialLines(body MaterialBody) []pdfLine {
	lines := make([]pdfLine, 0, 16)
	for index, section := range body.Sections {
		before := 0.0
		if index > 0 {
			before = 8
		}
		for _, text := range pdfWrapText(section.Heading, pdfHeadingFont) {
			lines = append(lines, pdfLine{text: text, font: pdfHeadingFont, before: before})
			before = 0
		}
		for _, text := range pdfWrapText(section.Content, pdfBodyFont) {
			lines = append(lines, pdfLine{text: text, font: pdfBodyFont})
		}
		for _, claim := range section.Claims {
			for _, text := range pdfWrapText("• "+claim.Text, pdfBodyFont) {
				lines = append(lines, pdfLine{text: text, font: pdfBodyFont})
			}
		}
	}
	return lines
}

// paginateMaterialLines assigns lines to pages top-down.
func paginateMaterialLines(lines []pdfLine) [][]pdfLine {
	var pages [][]pdfLine
	var page []pdfLine
	y := pdfPageHeight - pdfMarginTop
	flush := func() {
		pages = append(pages, page)
		page = nil
		y = pdfPageHeight - pdfMarginTop
	}
	for _, line := range lines {
		height := pdfLineHeight(line.font) + line.before
		if len(page) > 0 && y-height < pdfMarginBottom {
			flush()
		}
		y -= height
		page = append(page, line)
	}
	if len(page) > 0 {
		pages = append(pages, page)
	}
	return pages
}

func pdfTextCodes(text string) []uint16 {
	return utf16.Encode([]rune(text))
}

// pdfToUnicodeCMap renders an identity ToUnicode CMap over the used codes.
func pdfToUnicodeCMap(codes []uint16) string {
	sorted := append([]uint16(nil), codes...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	unique := make([]uint16, 0, len(sorted))
	for i, code := range sorted {
		if i > 0 && code == sorted[i-1] {
			continue
		}
		unique = append(unique, code)
	}
	type run struct{ low, high uint16 }
	var runs []run
	for _, code := range unique {
		if len(runs) > 0 && code == runs[len(runs)-1].high+1 {
			runs[len(runs)-1].high = code
			continue
		}
		runs = append(runs, run{low: code, high: code})
	}
	var out strings.Builder
	out.WriteString("/CIDInit /ProcSet findresource begin\n12 dict begin\nbegincmap\n")
	out.WriteString("/CMapName /Adobe-Identity-UCS def\n/CMapType 2 def\n")
	out.WriteString("1 begincodespacerange\n<0000> <FFFF>\nendcodespacerange\n")
	for start := 0; start < len(runs); start += 100 {
		end := start + 100
		if end > len(runs) {
			end = len(runs)
		}
		chunk := runs[start:end]
		out.WriteString(strconv.Itoa(len(chunk)) + " beginbfrange\n")
		for _, r := range chunk {
			fmt.Fprintf(&out, "<%04X> <%04X> <%04X>\n", r.low, r.high, r.low)
		}
		out.WriteString("endbfrange\n")
	}
	out.WriteString("endcmap\nCMapName currentdict /CMap defineresource pop\nend\nend\n")
	return out.String()
}

// renderMaterialPDF writes the deterministic PDF for one structured body.
func renderMaterialPDF(body MaterialBody) ([]byte, error) {
	lines := layoutMaterialLines(body)
	pages := paginateMaterialLines(lines)

	usedCodes := map[uint16]bool{}
	for _, line := range lines {
		for _, code := range pdfTextCodes(line.text) {
			usedCodes[code] = true
		}
	}
	var codes []uint16
	for code := range usedCodes {
		codes = append(codes, code)
	}
	cmap := pdfToUnicodeCMap(codes)

	// Object layout: 1 catalog, 2 pages, 3 font, 4 descendant, 5 descriptor,
	// 6 ToUnicode CMap, then two objects per page (page + content stream).
	type object struct {
		number int
		body   string
		stream string
	}
	objects := []object{
		{number: 1, body: "<< /Type /Catalog /Pages 2 0 R >>"},
	}
	kids := make([]string, 0, len(pages))
	for i := range pages {
		kids = append(kids, strconv.Itoa(7+2*i)+" 0 R")
	}
	objects = append(objects,
		object{number: 2, body: "<< /Type /Pages /Count " + strconv.Itoa(len(pages)) + " /Kids [" + strings.Join(kids, " ") + "] >>"},
		object{number: 3, body: "<< /Type /Font /Subtype /Type0 /BaseFont /WeKnoraCareerDoc /Encoding /Identity-H /DescendantFonts [4 0 R] /ToUnicode 6 0 R >>"},
		object{number: 4, body: "<< /Type /Font /Subtype /CIDFontType2 /BaseFont /WeKnoraCareerDoc /CIDSystemInfo << /Registry (Adobe) /Ordering (Identity) /Supplement 0 >> /FontDescriptor 5 0 R /DW 1000 /CIDToGIDMap /Identity >>"},
		object{number: 5, body: "<< /Type /FontDescriptor /FontName /WeKnoraCareerDoc /Flags 4 /FontBBox [0 -200 1000 900] /ItalicAngle 0 /Ascent 800 /Descent -200 /CapHeight 700 /StemV 80 >>"},
		object{number: 6, body: "<< /Length " + strconv.Itoa(len(cmap)) + " >>", stream: cmap},
	)
	for pageIndex, page := range pages {
		content := &strings.Builder{}
		pageNumber := 7 + 2*pageIndex
		contentsNumber := pageNumber + 1
		objects = append(objects, object{number: pageNumber, body: "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F1 3 0 R >> >> /Contents " + strconv.Itoa(contentsNumber) + " 0 R >>"})
		lineY := pdfPageHeight - pdfMarginTop
		for _, line := range page {
			lineY -= pdfLineHeight(line.font) + line.before
			codes := pdfTextCodes(line.text)
			codeBytes := make([]byte, 0, len(codes)*2)
			for _, code := range codes {
				codeBytes = append(codeBytes, byte(code>>8), byte(code))
			}
			fmt.Fprintf(content, "BT\n/F1 %d Tf\n1 0 0 1 %.1f %.1f Tm\n<%s> Tj\nET\n", line.font, pdfMarginX, lineY, strings.ToUpper(hex.EncodeToString(codeBytes)))
		}
		stream := content.String()
		objects = append(objects, object{number: contentsNumber, body: "<< /Length " + strconv.Itoa(len(stream)) + " >>", stream: stream})
	}

	sort.Slice(objects, func(i, j int) bool { return objects[i].number < objects[j].number })
	out := &bytes.Buffer{}
	out.WriteString("%PDF-1.7\n")
	offsets := make(map[int]int, len(objects))
	for _, obj := range objects {
		offsets[obj.number] = out.Len()
		fmt.Fprintf(out, "%d 0 obj\n%s\n", obj.number, obj.body)
		if obj.stream != "" {
			fmt.Fprintf(out, "stream\n%s\nendstream\n", obj.stream)
		}
		out.WriteString("endobj\n")
	}
	xrefOffset := out.Len()
	maxObject := objects[len(objects)-1].number
	fmt.Fprintf(out, "xref\n0 %d\n", maxObject+1)
	out.WriteString("0000000000 65535 f \n")
	for number := 1; number <= maxObject; number++ {
		offset, ok := offsets[number]
		if !ok {
			return nil, fmt.Errorf("pdf object %d missing", number)
		}
		fmt.Fprintf(out, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(out, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", maxObject+1, xrefOffset)
	return out.Bytes(), nil
}

// ---- independent PDF verification ----

// verifyMaterialPDF re-parses a rendered PDF through a path independent of the
// writer: it scans objects, walks the page tree, decodes the ToUnicode CMap,
// and decodes the text-showing operators. It fails on unmapped glyph codes,
// text drawn outside the page box, page-count inconsistencies, or body
// content that cannot be recovered from the file.
var (
	pdfObjectRe    = regexp.MustCompile(`(?s)(\d+) 0 obj\s*(.*?)\s*endobj`)
	pdfStreamRe    = regexp.MustCompile(`(?s)stream\n(.*?)\nendstream`)
	pdfCountRe     = regexp.MustCompile(`/Count (\d+)`)
	pdfKidsRe      = regexp.MustCompile(`/Kids \[([^\]]*)\]`)
	pdfRefRe       = regexp.MustCompile(`(\d+) 0 R`)
	pdfToUnicodeRe = regexp.MustCompile(`/ToUnicode (\d+) 0 R`)
	pdfMediaBoxRe  = regexp.MustCompile(`/MediaBox \[\s*([0-9.-]+)\s+([0-9.-]+)\s+([0-9.-]+)\s+([0-9.-]+)\s*\]`)
	pdfContentsRe  = regexp.MustCompile(`/Contents (\d+) 0 R`)
	pdfTextOpRe    = regexp.MustCompile(`1 0 0 1 ([0-9]+(?:\.[0-9]+)?) ([0-9]+(?:\.[0-9]+)?) Tm\s*<([0-9A-Fa-f]+)> Tj`)
	pdfBfRangeRe   = regexp.MustCompile(`(?s)beginbfrange(.*?)endbfrange`)
	pdfBfRangeRow  = regexp.MustCompile(`<([0-9A-Fa-f]{4})>\s*<([0-9A-Fa-f]{4})>\s*<([0-9A-Fa-f]{4})>`)
	pdfBfCharRe    = regexp.MustCompile(`(?s)beginbfchar(.*?)endbfchar`)
	pdfBfCharRow   = regexp.MustCompile(`<([0-9A-Fa-f]{4})>\s*<([0-9A-Fa-f]{4,8})>`)
)

type pdfParsedCMap struct {
	mapping map[uint16]string // code → UTF-16BE destination bytes
}

func (c pdfParsedCMap) decode(codes []uint16) (string, error) {
	var units []uint16
	for _, code := range codes {
		dst, ok := c.mapping[code]
		if !ok {
			return "", fmt.Errorf("pdf text uses glyph code %04X absent from the ToUnicode CMap", code)
		}
		if len(dst)%2 != 0 {
			return "", errors.New("pdf ToUnicode destination is not UTF-16BE")
		}
		for i := 0; i+1 < len(dst); i += 2 {
			units = append(units, uint16(dst[i])<<8|uint16(dst[i+1]))
		}
	}
	return string(utf16.Decode(units)), nil
}

func parsePDFToUnicode(stream string) (pdfParsedCMap, error) {
	mapping := map[uint16]string{}
	for _, block := range pdfBfRangeRe.FindAllStringSubmatch(stream, -1) {
		for _, row := range pdfBfRangeRow.FindAllStringSubmatch(block[1], -1) {
			low, errLow := strconv.ParseUint(row[1], 16, 16)
			high, errHigh := strconv.ParseUint(row[2], 16, 16)
			dst, errDst := hex.DecodeString(row[3])
			if errLow != nil || errHigh != nil || errDst != nil || high < low || len(dst) != 2 {
				return pdfParsedCMap{}, fmt.Errorf("malformed ToUnicode bfrange row %q", row[0])
			}
			for code := low; ; code++ {
				offset := int(code - low)
				mapping[uint16(code)] = string([]byte{dst[0] + byte(offset>>8), dst[1] + byte(offset)})
				if code == high {
					break
				}
			}
		}
	}
	for _, block := range pdfBfCharRe.FindAllStringSubmatch(stream, -1) {
		for _, row := range pdfBfCharRow.FindAllStringSubmatch(block[1], -1) {
			code, errCode := strconv.ParseUint(row[1], 16, 16)
			dst, errDst := hex.DecodeString(row[2])
			if errCode != nil || errDst != nil || len(dst) == 0 || len(dst)%2 != 0 {
				return pdfParsedCMap{}, fmt.Errorf("malformed ToUnicode bfchar row %q", row[0])
			}
			mapping[uint16(code)] = string(dst)
		}
	}
	if len(mapping) == 0 {
		return pdfParsedCMap{}, errors.New("pdf ToUnicode CMap maps no codes")
	}
	return pdfParsedCMap{mapping: mapping}, nil
}

func verifyMaterialPDF(data []byte, body MaterialBody) error {
	if !bytes.HasPrefix(data, []byte("%PDF-")) {
		return errors.New("pdf verification: missing %PDF header")
	}
	if !bytes.HasSuffix(data, []byte("%%EOF\n")) {
		return errors.New("pdf verification: missing %%EOF trailer")
	}
	objects := map[int]string{}
	for _, match := range pdfObjectRe.FindAllStringSubmatch(string(data), -1) {
		number, err := strconv.Atoi(match[1])
		if err != nil {
			return fmt.Errorf("pdf verification: bad object number %q", match[1])
		}
		objects[number] = match[2]
	}
	if len(objects) == 0 {
		return errors.New("pdf verification: no objects found")
	}
	var catalog string
	for _, body := range objects {
		if strings.Contains(body, "/Type /Catalog") {
			catalog = body
			break
		}
	}
	if catalog == "" {
		return errors.New("pdf verification: catalog object missing")
	}
	pagesMatch := pdfRefRe.FindStringSubmatch(strings.SplitN(catalog, "/Pages", 2)[1])
	if pagesMatch == nil {
		return errors.New("pdf verification: catalog has no pages reference")
	}
	pagesNumber, err := strconv.Atoi(pagesMatch[1])
	if err != nil {
		return err
	}
	pages, ok := objects[pagesNumber]
	if !ok || !strings.Contains(pages, "/Type /Pages") {
		return errors.New("pdf verification: pages object missing")
	}
	countMatch := pdfCountRe.FindStringSubmatch(pages)
	if countMatch == nil {
		return errors.New("pdf verification: pages object has no /Count")
	}
	count, err := strconv.Atoi(countMatch[1])
	if err != nil {
		return err
	}
	kidsMatch := pdfKidsRe.FindStringSubmatch(pages)
	if kidsMatch == nil {
		return errors.New("pdf verification: pages object has no /Kids")
	}
	kids := pdfRefRe.FindAllStringSubmatch(kidsMatch[1], -1)
	if count != len(kids) {
		return fmt.Errorf("pdf verification: page count %d does not match %d page objects", count, len(kids))
	}
	if count < 1 {
		return errors.New("pdf verification: document has no pages")
	}

	// ToUnicode CMap from the shared font object.
	toUnicodeNumber := 0
	for _, objectBody := range objects {
		if m := pdfToUnicodeRe.FindStringSubmatch(objectBody); m != nil {
			toUnicodeNumber, _ = strconv.Atoi(m[1])
			break
		}
	}
	if toUnicodeNumber == 0 {
		return errors.New("pdf verification: font has no ToUnicode CMap")
	}
	cmapObject, ok := objects[toUnicodeNumber]
	if !ok {
		return fmt.Errorf("pdf verification: ToUnicode object %d missing", toUnicodeNumber)
	}
	streamMatch := pdfStreamRe.FindStringSubmatch(cmapObject)
	if streamMatch == nil {
		return errors.New("pdf verification: ToUnicode CMap stream missing")
	}
	cmap, err := parsePDFToUnicode(streamMatch[1])
	if err != nil {
		return err
	}

	var extracted []string
	for _, kid := range kids {
		pageNumber, _ := strconv.Atoi(kid[1])
		page, ok := objects[pageNumber]
		if !ok || !strings.Contains(page, "/Type /Page") {
			return fmt.Errorf("pdf verification: page object %d missing", pageNumber)
		}
		box := pdfMediaBoxRe.FindStringSubmatch(page)
		if box == nil {
			return errors.New("pdf verification: page has no MediaBox")
		}
		vals := make([]float64, 4)
		for i := range vals {
			v, err := strconv.ParseFloat(box[i+1], 64)
			if err != nil {
				return err
			}
			vals[i] = v
		}
		if vals[0] != 0 || vals[1] != 0 || vals[2] != pdfPageWidth || vals[3] != pdfPageHeight {
			return fmt.Errorf("pdf verification: unexpected MediaBox %v", vals)
		}
		contents := pdfContentsRe.FindStringSubmatch(page)
		if contents == nil {
			return errors.New("pdf verification: page has no contents")
		}
		contentsNumber, _ := strconv.Atoi(contents[1])
		contentsObject, ok := objects[contentsNumber]
		if !ok {
			return fmt.Errorf("pdf verification: contents object %d missing", contentsNumber)
		}
		stream := pdfStreamRe.FindStringSubmatch(contentsObject)
		if stream == nil {
			return errors.New("pdf verification: content stream missing")
		}
		pageLines := 0
		for _, op := range pdfTextOpRe.FindAllStringSubmatch(stream[1], -1) {
			x, errX := strconv.ParseFloat(op[1], 64)
			y, errY := strconv.ParseFloat(op[2], 64)
			if errX != nil || errY != nil {
				return errors.New("pdf verification: malformed text matrix")
			}
			if x < vals[0] || x >= vals[2] || y <= vals[1] || y > vals[3] {
				return fmt.Errorf("pdf verification: text drawn at (%.1f, %.1f) outside the page box", x, y)
			}
			raw, err := hex.DecodeString(op[3])
			if err != nil || len(raw)%2 != 0 {
				return errors.New("pdf verification: malformed hex string")
			}
			codes := make([]uint16, 0, len(raw)/2)
			for i := 0; i < len(raw); i += 2 {
				codes = append(codes, uint16(raw[i])<<8|uint16(raw[i+1]))
			}
			text, err := cmap.decode(codes)
			if err != nil {
				return err
			}
			extracted = append(extracted, text)
			pageLines++
		}
		if pageLines == 0 {
			return errors.New("pdf verification: page carries no text lines")
		}
	}
	joined := strings.Join(extracted, "")
	if strings.TrimSpace(joined) == "" {
		return errors.New("pdf verification: extracted text is empty")
	}
	for _, section := range body.Sections {
		if !strings.Contains(joined, section.Heading) {
			return fmt.Errorf("pdf verification: heading %q missing from extracted text", section.Heading)
		}
		if section.Content != "" && !strings.Contains(joined, section.Content) {
			return fmt.Errorf("pdf verification: section content missing from extracted text")
		}
		for _, claim := range section.Claims {
			if !strings.Contains(joined, claim.Text) {
				return fmt.Errorf("pdf verification: claim %s missing from extracted text", claim.ClaimID)
			}
		}
	}
	return nil
}

// ---- DOCX rendering (archive/zip + encoding/xml, deterministic) ----

const docxRelationOfficeDocument = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument"
const docxContentTypeDocument = "application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"
const docxContentTypeStyles = "application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"

var docxZipEpoch = time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)

func xmlEscape(text string) string {
	var out strings.Builder
	if err := xml.EscapeText(&out, []byte(text)); err != nil {
		return ""
	}
	return out.String()
}

const docxContentTypesXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="` + docxContentTypeDocument + `"/><Override PartName="/word/styles.xml" ContentType="` + docxContentTypeStyles + `"/></Types>`

const docxRootRelsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="` + docxRelationOfficeDocument + `" Target="word/document.xml"/></Relationships>`

const docxDocumentRelsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/></Relationships>`

const docxStylesXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:docDefaults><w:rPrDefault><w:rPr><w:rFonts w:ascii="Calibri" w:eastAsia="DengXian" w:hAnsi="Calibri"/><w:sz w:val="22"/></w:rPr></w:rPrDefault><w:pPrDefault><w:pPr><w:spacing w:after="120"/></w:pPr></w:pPrDefault></w:docDefaults><w:style w:type="paragraph" w:default="1" w:styleId="Normal"><w:name w:val="Normal"/></w:style><w:style w:type="paragraph" w:styleId="Heading1"><w:name w:val="heading 1"/><w:basedOn w:val="Normal"/><w:pPr><w:keepNext/><w:outlineLvl w:val="0"/></w:pPr><w:rPr><w:b/><w:sz w:val="28"/></w:rPr></w:style><w:style w:type="paragraph" w:styleId="ListParagraph"><w:name w:val="List Paragraph"/><w:basedOn w:val="Normal"/><w:pPr><w:ind w:left="480"/></w:pPr></w:style></w:styles>`

// renderMaterialDOCX writes a deterministic minimal OOXML package: fixed
// entry order, fixed timestamps, no variable metadata.
func renderMaterialDOCX(body MaterialBody) ([]byte, error) {
	document := &strings.Builder{}
	document.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n")
	document.WriteString(`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>`)
	paragraph := func(text, style string) {
		document.WriteString(`<w:p><w:pPr><w:pStyle w:val="` + style + `"/></w:pPr><w:r><w:t xml:space="preserve">` + xmlEscape(text) + `</w:t></w:r></w:p>`)
	}
	for _, section := range body.Sections {
		paragraph(section.Heading, "Heading1")
		if section.Content != "" {
			paragraph(section.Content, "Normal")
		}
		for _, claim := range section.Claims {
			paragraph("• "+claim.Text, "ListParagraph")
		}
	}
	document.WriteString(`<w:sectPr><w:pgSz w:w="11906" w:h="16838"/><w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440" w:header="720" w:footer="720" w:gutter="0"/></w:sectPr>`)
	document.WriteString(`</w:body></w:document>`)

	entries := []struct{ name, content string }{
		{name: "[Content_Types].xml", content: docxContentTypesXML},
		{name: "_rels/.rels", content: docxRootRelsXML},
		{name: "word/document.xml", content: document.String()},
		{name: "word/_rels/document.xml.rels", content: docxDocumentRelsXML},
		{name: "word/styles.xml", content: docxStylesXML},
	}
	out := &bytes.Buffer{}
	writer := zip.NewWriter(out)
	for _, entry := range entries {
		header := &zip.FileHeader{Name: entry.name, Method: zip.Deflate}
		header.Modified = docxZipEpoch
		file, err := writer.CreateHeader(header)
		if err != nil {
			return nil, err
		}
		if _, err = file.Write([]byte(entry.content)); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// ---- independent DOCX verification ----

type docxContentTypes struct {
	Defaults []struct {
		Extension   string `xml:"Extension,attr"`
		ContentType string `xml:"ContentType,attr"`
	} `xml:"Default"`
	Overrides []struct {
		PartName    string `xml:"PartName,attr"`
		ContentType string `xml:"ContentType,attr"`
	} `xml:"Override"`
}

type docxRelationships struct {
	Relationships []struct {
		ID     string `xml:"Id,attr"`
		Type   string `xml:"Type,attr"`
		Target string `xml:"Target,attr"`
	} `xml:"Relationship"`
}

type docxDocument struct {
	Body struct {
		Paragraphs []struct {
			Runs []struct {
				Text []string `xml:"t"`
			} `xml:"r"`
		} `xml:"p"`
		SectPr *struct{} `xml:"sectPr"`
	} `xml:"body"`
}

// verifyMaterialDOCX checks the OOXML package independently: zip integrity
// (structure and CRC via the archive reader), the content-type and
// relationship essentials an editor needs, the sectPr section properties, and
// that the document text carries the full structured body.
func verifyMaterialDOCX(data []byte, body MaterialBody) error {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("docx verification: package is not a readable zip: %w", err)
	}
	files := map[string][]byte{}
	for _, file := range reader.File {
		rc, openErr := file.Open()
		if openErr != nil {
			return fmt.Errorf("docx verification: entry %s unreadable: %w", file.Name, openErr)
		}
		content, readErr := io.ReadAll(rc)
		closeErr := rc.Close()
		if readErr != nil {
			return fmt.Errorf("docx verification: entry %s failed its checksum: %w", file.Name, readErr)
		}
		if closeErr != nil {
			return closeErr
		}
		files[file.Name] = content
	}
	for _, required := range []string{"[Content_Types].xml", "_rels/.rels", "word/document.xml", "word/_rels/document.xml.rels", "word/styles.xml"} {
		if _, ok := files[required]; !ok {
			return fmt.Errorf("docx verification: required entry %s missing", required)
		}
	}
	var contentTypes docxContentTypes
	if err = xml.Unmarshal(files["[Content_Types].xml"], &contentTypes); err != nil {
		return fmt.Errorf("docx verification: [Content_Types].xml malformed: %w", err)
	}
	documentTypeKnown := false
	for _, override := range contentTypes.Overrides {
		if override.PartName == "/word/document.xml" && override.ContentType == docxContentTypeDocument {
			documentTypeKnown = true
		}
	}
	if !documentTypeKnown {
		return errors.New("docx verification: document part not declared with the wordprocessing main content type")
	}
	var rootRels docxRelationships
	if err = xml.Unmarshal(files["_rels/.rels"], &rootRels); err != nil {
		return fmt.Errorf("docx verification: _rels/.rels malformed: %w", err)
	}
	linked := false
	for _, rel := range rootRels.Relationships {
		if rel.Type == docxRelationOfficeDocument && strings.TrimPrefix(rel.Target, "/") == "word/document.xml" {
			linked = true
		}
	}
	if !linked {
		return errors.New("docx verification: root relationships do not link word/document.xml")
	}
	var docRels docxRelationships
	if err = xml.Unmarshal(files["word/_rels/document.xml.rels"], &docRels); err != nil {
		return fmt.Errorf("docx verification: word/_rels/document.xml.rels malformed: %w", err)
	}
	stylesLinked := false
	for _, rel := range docRels.Relationships {
		// document.xml.rels targets are relative to the word/ part directory.
		if strings.HasSuffix(rel.Type, "/styles") && (rel.Target == "styles.xml" || strings.TrimPrefix(rel.Target, "/") == "word/styles.xml") {
			stylesLinked = true
		}
	}
	if !stylesLinked {
		return errors.New("docx verification: document relationships do not link word/styles.xml")
	}

	var document docxDocument
	if err = xml.Unmarshal(files["word/document.xml"], &document); err != nil {
		return fmt.Errorf("docx verification: word/document.xml malformed: %w", err)
	}
	if document.Body.SectPr == nil {
		return errors.New("docx verification: document body lacks w:sectPr section properties")
	}
	var texts []string
	for _, paragraph := range document.Body.Paragraphs {
		for _, run := range paragraph.Runs {
			texts = append(texts, run.Text...)
		}
	}
	joined := strings.Join(texts, "")
	for _, section := range body.Sections {
		if !strings.Contains(joined, section.Heading) {
			return fmt.Errorf("docx verification: heading %q missing from document text", section.Heading)
		}
		if section.Content != "" && !strings.Contains(joined, section.Content) {
			return fmt.Errorf("docx verification: section content missing from document text")
		}
		for _, claim := range section.Claims {
			if !strings.Contains(joined, claim.Text) {
				return fmt.Errorf("docx verification: claim %s missing from document text", claim.ClaimID)
			}
		}
	}
	return nil
}

// ---- Office export service ----

func (o *Office) exportClock() time.Time {
	if o.exportNow != nil {
		return o.exportNow()
	}
	return time.Now().UTC()
}

func validExportFormat(format string) bool {
	return format == ExportFormatPDF || format == ExportFormatDOCX
}

// verifyExportFormat runs the format's independent verifier; the hook only
// synchronizes error-path tests.
func (o *Office) verifyExportFormat(format string, data []byte, body MaterialBody) error {
	if o.failExportVerify != nil {
		if err := o.failExportVerify(format); err != nil {
			return err
		}
	}
	if format == ExportFormatPDF {
		return verifyMaterialPDF(data, body)
	}
	return verifyMaterialDOCX(data, body)
}

func (o *Office) replayExportReceipt(ctx context.Context, s Scope, requestID, fingerprint, kind string) (ExportReceipt, bool, error) {
	var row materialReceiptRecord
	err := o.db.WithContext(ctx).
		Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, requestID).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ExportReceipt{}, false, nil
	}
	if err != nil {
		return ExportReceipt{}, false, err
	}
	if row.Fingerprint != fingerprint {
		return ExportReceipt{}, true, ErrIdempotencyConflict
	}
	var receipt ExportReceipt
	if err = json.Unmarshal([]byte(row.Body), &receipt); err != nil {
		return ExportReceipt{}, true, fmt.Errorf("decode career material export receipt: %w", err)
	}
	if receipt.Kind != kind {
		return ExportReceipt{}, true, ErrIdempotencyConflict
	}
	return receipt, true, nil
}

func (o *Office) loadExportRow(ctx context.Context, s Scope, materialID, exportID string) (materialExportRecord, error) {
	if exportID == "" || len(exportID) > 36 {
		return materialExportRecord{}, ErrInvalidRequest
	}
	var row materialExportRecord
	err := o.db.WithContext(ctx).
		Where("tenant_id=? AND user_id=? AND material_id=? AND id=?", s.TenantID, s.UserID, materialID, exportID).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return materialExportRecord{}, ErrExportNotFound
	}
	if err != nil {
		return materialExportRecord{}, err
	}
	return row, nil
}

// PublishMaterial renders the PDF and DOCX of one immutable material version
// from the same structured body, verifies both files through independent
// parsers, and persists the export. Only exports whose two formats both
// verified become submittable; a single-format failure keeps the staged state
// with the typed error and never publishes half a pair as deliverable.
func (o *Office) PublishMaterial(ctx context.Context, input PublishMaterialInput) (ExportReceipt, error) {
	o.lifecycleMu.RLock()
	defer o.lifecycleMu.RUnlock()
	s, err := getScope(ctx)
	if err != nil {
		return ExportReceipt{}, err
	}
	if err = o.requireSpace(ctx, s); err != nil {
		return ExportReceipt{}, err
	}
	input.RequestID = strings.TrimSpace(input.RequestID)
	input.MaterialID = strings.TrimSpace(input.MaterialID)
	if input.RequestID == "" || len(input.RequestID) > 128 || input.MaterialID == "" || len(input.MaterialID) > 36 || input.Version == 0 {
		return ExportReceipt{}, ErrInvalidRequest
	}
	fingerprint, err := materialFingerprint(materialFingerprintPublish, input.RequestID, input.MaterialID, input.Version, input.ExpectedRevision)
	if err != nil {
		return ExportReceipt{}, err
	}
	if replay, found, lookupErr := o.replayExportReceipt(ctx, s, input.RequestID, fingerprint, MaterialKindPublished); lookupErr != nil {
		return ExportReceipt{}, lookupErr
	} else if found {
		var existing lifecycleClaim
		if lookup := o.db.WithContext(ctx).Where("tenant_id=? AND user_id=? AND operation=? AND request_id=?", s.TenantID, s.UserID, "material_publish", input.RequestID).First(&existing).Error; lookup == nil {
			ownerToken, unlockAttempt, claimErr := o.acquireLifecycleClaim(ctx, s, "material_publish", input.RequestID, fingerprint)
			if claimErr != nil {
				return ExportReceipt{}, claimErr
			}
			defer unlockAttempt()
			if releaseErr := o.resolveLifecycleClaimOwned(context.Background(), s, "material_publish", input.RequestID, ownerToken); releaseErr != nil {
				return ExportReceipt{}, &OutcomeUnknownError{RequestID: input.RequestID}
			}
		}
		return replay, nil
	}
	if o.exportStorage == nil {
		return ExportReceipt{}, ErrExportStorageUnavailable
	}
	view, err := o.MaterialVersion(ctx, input.MaterialID, input.Version)
	if err != nil {
		return ExportReceipt{}, err
	}
	contentDigest := materialContentDigest(view.Body)

	// Stable per-scope request identity makes a retry find/overwrite the same
	// objects after a crash between storage and locator persistence.
	exportID := uuid.NewSHA1(uuid.NameSpaceURL, []byte(fmt.Sprintf("career-export:%d:%s:%s:%s", s.TenantID, s.UserID, input.RequestID, fingerprint))).String()
	pdfBytes, renderErr := renderMaterialPDF(view.Body)
	if renderErr != nil {
		return ExportReceipt{}, fmt.Errorf("render career material pdf: %w", renderErr)
	}
	docxBytes, renderErr := renderMaterialDOCX(view.Body)
	if renderErr != nil {
		return ExportReceipt{}, fmt.Errorf("render career material docx: %w", renderErr)
	}
	ownerToken, unlockAttempt, claimErr := o.acquireLifecycleClaim(ctx, s, "material_publish", input.RequestID, fingerprint)
	if claimErr != nil {
		return ExportReceipt{}, claimErr
	}
	defer unlockAttempt()
	finishClaim := func() error {
		return o.resolveLifecycleClaimOwned(context.Background(), s, "material_publish", input.RequestID, ownerToken)
	}
	pdfKey, err := o.exportStorage.SaveExport(ctx, s.TenantID, "career_export_"+exportID+pdfExportExtension, pdfBytes)
	if err != nil {
		// Storage errors may be an unknown outcome; retain the claim so the
		// original request can reconcile or compensate before deletion.
		return ExportReceipt{}, &OutcomeUnknownError{RequestID: input.RequestID}
	}
	docxKey, err := o.exportStorage.SaveExport(ctx, s.TenantID, "career_export_"+exportID+docxExportExtension, docxBytes)
	if err != nil {
		// The adapter may have stored the object despite returning an error.
		// Keep the claim and replay the stable request-derived object names;
		// only a complete locator receipt or proven compensation may release it.
		return ExportReceipt{}, &OutcomeUnknownError{RequestID: input.RequestID}
	}
	pdfVerifyErr := o.verifyExportFormat(ExportFormatPDF, pdfBytes, view.Body)
	docxVerifyErr := o.verifyExportFormat(ExportFormatDOCX, docxBytes, view.Body)

	pdfDigest := sha256.Sum256(pdfBytes)
	docxDigest := sha256.Sum256(docxBytes)
	files := []ExportedFile{
		{
			Format: ExportFormatPDF, MaterialID: input.MaterialID, Version: input.Version,
			ContentDigest: contentDigest, ObjectKey: pdfKey,
			FileDigest: hex.EncodeToString(pdfDigest[:]), Size: int64(len(pdfBytes)),
			Verified: pdfVerifyErr == nil, Error: errorText(pdfVerifyErr),
		},
		{
			Format: ExportFormatDOCX, MaterialID: input.MaterialID, Version: input.Version,
			ContentDigest: contentDigest, ObjectKey: docxKey,
			FileDigest: hex.EncodeToString(docxDigest[:]), Size: int64(len(docxBytes)),
			Verified: docxVerifyErr == nil, Error: errorText(docxVerifyErr),
		},
	}
	status := ExportStatusStaged
	switch {
	case pdfVerifyErr == nil && docxVerifyErr == nil:
		status = ExportStatusSubmittable
	case pdfVerifyErr != nil && docxVerifyErr != nil:
		status = ExportStatusFailed
	}
	now := o.exportClock()
	receipt := ExportReceipt{
		Kind: MaterialKindPublished, RequestID: input.RequestID, ExportID: exportID,
		MaterialID: input.MaterialID, Version: input.Version,
		Status: status, Submittable: status == ExportStatusSubmittable,
		ContentDigest: contentDigest, Files: files, CreatedAt: now,
	}
	if status != ExportStatusSubmittable {
		receipt.FailureCode = ExportFailureVerification
		receipt.FailureMessage = strings.TrimSpace(strings.TrimSpace(errorText(pdfVerifyErr)) + " " + errorText(docxVerifyErr))
	}

	persisted := false
	storedExisting := false
	err = o.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var stored materialReceiptRecord
		e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, input.RequestID).
			First(&stored).Error
		if e == nil {
			storedExisting = true
			if stored.Fingerprint != fingerprint {
				return ErrIdempotencyConflict
			}
			var replayed ExportReceipt
			if e = json.Unmarshal([]byte(stored.Body), &replayed); e != nil {
				return e
			}
			if replayed.Kind != MaterialKindPublished {
				return ErrIdempotencyConflict
			}
			receipt = replayed
			return nil
		}
		if !errors.Is(e, gorm.ErrRecordNotFound) {
			return e
		}
		var head profile
		e = tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tenant_id=? AND user_id=?", s.TenantID, s.UserID).First(&head).Error
		if errors.Is(e, gorm.ErrRecordNotFound) {
			head.Revision = 0
		} else if e != nil {
			return e
		}
		if head.Revision != input.ExpectedRevision {
			return &RevisionConflictError{CurrentRevision: head.Revision}
		}
		if e = tx.Create(&materialExportRecord{
			ID: exportID, TenantID: s.TenantID, UserID: s.UserID, MaterialID: input.MaterialID,
			RequestID: input.RequestID, Version: input.Version, Fingerprint: fingerprint,
			ContentDigest: contentDigest, Status: status,
			PDFObjectKey: pdfKey, PDFDigest: hex.EncodeToString(pdfDigest[:]), PDFSize: int64(len(pdfBytes)), PDFError: errorText(pdfVerifyErr),
			DOCXObjectKey: docxKey, DOCXDigest: hex.EncodeToString(docxDigest[:]), DOCXSize: int64(len(docxBytes)), DOCXError: errorText(docxVerifyErr),
			ReceiptBody: string(mustJSON(receipt)), CreatedAt: now, UpdatedAt: now,
		}).Error; e != nil {
			return e
		}
		if e = tx.Create(&materialReceiptRecord{
			TenantID: s.TenantID, UserID: s.UserID, RequestID: input.RequestID,
			Fingerprint: fingerprint, Body: string(mustJSON(receipt)), CreatedAt: now,
		}).Error; e != nil {
			return e
		}
		persisted = true
		return nil
	})
	if persisted && err == nil {
		if releaseErr := finishClaim(); releaseErr != nil {
			return ExportReceipt{}, &OutcomeUnknownError{RequestID: input.RequestID}
		}
		return receipt, nil
	}
	if storedExisting && err == nil {
		if releaseErr := finishClaim(); releaseErr != nil {
			return ExportReceipt{}, &OutcomeUnknownError{RequestID: input.RequestID}
		}
		return receipt, nil
	}
	if err != nil {
		if replay, found, lookupErr := o.replayExportReceipt(ctx, s, input.RequestID, fingerprint, MaterialKindPublished); lookupErr == nil && found {
			if releaseErr := finishClaim(); releaseErr != nil {
				return ExportReceipt{}, &OutcomeUnknownError{RequestID: input.RequestID}
			}
			return replay, nil
		} else if lookupErr != nil {
			return ExportReceipt{}, &OutcomeUnknownError{RequestID: input.RequestID}
		}
	}
	// No terminal receipt exists. Remove deterministic objects; only a proven
	// compensation permits dropping the durable claim.
	if cleanupErr := o.deleteExportObjects(ctx, pdfKey, docxKey); cleanupErr != nil {
		return ExportReceipt{}, &OutcomeUnknownError{RequestID: input.RequestID}
	}
	if releaseErr := finishClaim(); releaseErr != nil {
		return ExportReceipt{}, &OutcomeUnknownError{RequestID: input.RequestID}
	}
	if err != nil {
		if errors.Is(err, ErrIdempotencyConflict) || errors.Is(err, ErrInvalidRequest) ||
			errors.Is(err, ErrMaterialNotFound) || errors.Is(err, ErrMaterialVersionNotFound) ||
			errors.Is(err, ErrExportStorageUnavailable) {
			return ExportReceipt{}, err
		}
		var revisionConflict *RevisionConflictError
		if errors.As(err, &revisionConflict) {
			return ExportReceipt{}, err
		}
		if isReceiptRaceError(err) {
			if replay, found, lookupErr := o.replayExportReceipt(ctx, s, input.RequestID, fingerprint, MaterialKindPublished); lookupErr != nil {
				return ExportReceipt{}, lookupErr
			} else if found {
				return replay, nil
			}
		}
		if ctx.Err() != nil {
			return ExportReceipt{}, &OutcomeUnknownError{RequestID: input.RequestID}
		}
		return ExportReceipt{}, err
	}
	return receipt, nil
}

// MaterialExports lists every export of one material under the authenticated
// scope; old versions keep their exports forever.
func (o *Office) MaterialExports(ctx context.Context, materialID string) ([]ExportReceipt, error) {
	s, err := getScope(ctx)
	if err != nil {
		return nil, err
	}
	if err = o.requireSpace(ctx, s); err != nil {
		return nil, err
	}
	materialID = strings.TrimSpace(materialID)
	if _, err = o.loadMaterialRow(ctx, s, materialID); err != nil {
		return nil, err
	}
	var rows []materialExportRecord
	err = o.db.WithContext(ctx).
		Where("tenant_id=? AND user_id=? AND material_id=?", s.TenantID, s.UserID, materialID).
		Order("created_at ASC").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]ExportReceipt, 0, len(rows))
	for _, row := range rows {
		var receipt ExportReceipt
		if err = json.Unmarshal([]byte(row.ReceiptBody), &receipt); err != nil {
			return nil, fmt.Errorf("decode career material export receipt: %w", err)
		}
		out = append(out, receipt)
	}
	return out, nil
}

// MaterialExportGrant issues a short-lived download grant for one format of a
// submittable export. Grants are bound to the issuing owner and fail closed
// without a configured signing key.
func (o *Office) MaterialExportGrant(ctx context.Context, materialID, exportID, format string, ttl time.Duration) (ExportDownload, error) {
	s, err := getScope(ctx)
	if err != nil {
		return ExportDownload{}, err
	}
	if err = o.requireSpace(ctx, s); err != nil {
		return ExportDownload{}, err
	}
	if !validExportFormat(format) {
		return ExportDownload{}, ErrInvalidRequest
	}
	if ttl <= 0 || ttl > MaxExportGrantTTL {
		return ExportDownload{}, ErrInvalidRequest
	}
	if len(o.exportSigningKey) < 32 {
		return ExportDownload{}, ErrExportSigningKeyMissing
	}
	row, err := o.loadExportRow(ctx, s, strings.TrimSpace(materialID), strings.TrimSpace(exportID))
	if err != nil {
		return ExportDownload{}, err
	}
	if row.Status != ExportStatusSubmittable || row.RevokedAt != nil {
		return ExportDownload{}, ErrExportNotSubmittable
	}
	expiresAt := o.exportClock().Add(ttl).Unix()
	grant := careerExportGrant{TenantID: s.TenantID, UserID: s.UserID, MaterialID: row.MaterialID, ExportID: row.ID, Format: format, ExpiresAt: expiresAt}
	signature, err := careerExportGrantSignature(o.exportSigningKey, grant)
	if err != nil {
		return ExportDownload{}, err
	}
	download := ExportDownload{
		ExportID: row.ID, MaterialID: row.MaterialID, Version: row.Version, Format: format,
		ExpiresAt: expiresAt, Signature: signature,
		URL: fmt.Sprintf("/api/v1/career/materials/%s/exports/%s/download?format=%s&expires=%d&signature=%s",
			row.MaterialID, row.ID, format, expiresAt, signature),
	}
	if format == ExportFormatPDF {
		download.Digest, download.Size = row.PDFDigest, row.PDFSize
	} else {
		download.Digest, download.Size = row.DOCXDigest, row.DOCXSize
	}
	return download, nil
}

// DownloadMaterialExport redeems a download grant: the HMAC signature and
// expiry are checked first, then the durable export state is re-loaded so a
// revocation invalidates already-issued grants immediately, and the streamed
// bytes must match the recorded file digest.
func (o *Office) DownloadMaterialExport(ctx context.Context, materialID, exportID, format, expires, signature string) ([]byte, error) {
	s, err := getScope(ctx)
	if err != nil {
		return nil, err
	}
	if err = o.requireSpace(ctx, s); err != nil {
		return nil, err
	}
	if !validExportFormat(format) {
		return nil, ErrInvalidRequest
	}
	expiresAt, err := strconv.ParseInt(strings.TrimSpace(expires), 10, 64)
	if err != nil || expiresAt <= 0 {
		return nil, ErrExportGrantInvalid
	}
	if len(o.exportSigningKey) < 32 {
		return nil, ErrExportGrantInvalid
	}
	grant := careerExportGrant{
		TenantID: s.TenantID, UserID: s.UserID,
		MaterialID: strings.TrimSpace(materialID), ExportID: strings.TrimSpace(exportID),
		Format: format, ExpiresAt: expiresAt,
	}
	if err = verifyCareerExportGrant(o.exportSigningKey, grant, signature, o.exportClock()); err != nil {
		return nil, err
	}
	if o.exportStorage == nil {
		return nil, ErrExportStorageUnavailable
	}
	row, err := o.loadExportRow(ctx, s, grant.MaterialID, grant.ExportID)
	if err != nil {
		if errors.Is(err, ErrExportNotFound) {
			return nil, ErrExportGrantInvalid
		}
		return nil, err
	}
	// Redemption failures after a valid signature deliberately collapse into
	// one typed error: revoked, no longer submittable, or missing exports all
	// fail without disclosing which state applied.
	if row.Status != ExportStatusSubmittable || row.RevokedAt != nil {
		return nil, ErrExportGrantInvalid
	}
	objectKey, wantDigest := row.PDFObjectKey, row.PDFDigest
	if format == ExportFormatDOCX {
		objectKey, wantDigest = row.DOCXObjectKey, row.DOCXDigest
	}
	if objectKey == "" {
		return nil, ErrExportGrantInvalid
	}
	data, err := o.exportStorage.ReadExport(ctx, objectKey)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != wantDigest {
		return nil, fmt.Errorf("career material export bytes do not match the recorded digest")
	}
	return data, nil
}

// RevokeMaterialExport revokes one export under the house request-ID and
// revision semantics; already-issued grants fail immediately afterwards.
func (o *Office) RevokeMaterialExport(ctx context.Context, input RevokeMaterialExportInput) (ExportReceipt, error) {
	s, err := getScope(ctx)
	if err != nil {
		return ExportReceipt{}, err
	}
	if err = o.requireSpace(ctx, s); err != nil {
		return ExportReceipt{}, err
	}
	input.RequestID = strings.TrimSpace(input.RequestID)
	input.MaterialID = strings.TrimSpace(input.MaterialID)
	input.ExportID = strings.TrimSpace(input.ExportID)
	if input.RequestID == "" || len(input.RequestID) > 128 || input.MaterialID == "" || len(input.MaterialID) > 36 || input.ExportID == "" || len(input.ExportID) > 36 {
		return ExportReceipt{}, ErrInvalidRequest
	}
	fingerprint, err := materialFingerprint(materialFingerprintRevoke, input.RequestID, input.MaterialID, input.ExportID, input.ExpectedRevision)
	if err != nil {
		return ExportReceipt{}, err
	}
	if replay, found, lookupErr := o.replayExportReceipt(ctx, s, input.RequestID, fingerprint, MaterialKindExportRevoked); lookupErr != nil {
		return ExportReceipt{}, lookupErr
	} else if found {
		return replay, nil
	}

	var receipt ExportReceipt
	err = o.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var stored materialReceiptRecord
		e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tenant_id=? AND user_id=? AND request_id=?", s.TenantID, s.UserID, input.RequestID).
			First(&stored).Error
		if e == nil {
			if stored.Fingerprint != fingerprint {
				return ErrIdempotencyConflict
			}
			var replayed ExportReceipt
			if e = json.Unmarshal([]byte(stored.Body), &replayed); e != nil {
				return e
			}
			if replayed.Kind != MaterialKindExportRevoked {
				return ErrIdempotencyConflict
			}
			receipt = replayed
			return nil
		}
		if !errors.Is(e, gorm.ErrRecordNotFound) {
			return e
		}
		if e := requireGateActiveTx(tx, s); e != nil {
			return e
		}
		var head profile
		e = tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tenant_id=? AND user_id=?", s.TenantID, s.UserID).First(&head).Error
		if errors.Is(e, gorm.ErrRecordNotFound) {
			head.Revision = 0
		} else if e != nil {
			return e
		}
		if head.Revision != input.ExpectedRevision {
			return &RevisionConflictError{CurrentRevision: head.Revision}
		}
		var row materialExportRecord
		e = tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tenant_id=? AND user_id=? AND material_id=? AND id=?", s.TenantID, s.UserID, input.MaterialID, input.ExportID).
			First(&row).Error
		if errors.Is(e, gorm.ErrRecordNotFound) {
			return ErrExportNotFound
		}
		if e != nil {
			return e
		}
		var prior ExportReceipt
		if e = json.Unmarshal([]byte(row.ReceiptBody), &prior); e != nil {
			return fmt.Errorf("decode career material export receipt: %w", e)
		}
		now := o.exportClock()
		receipt = prior
		receipt.Kind = MaterialKindExportRevoked
		receipt.RequestID = input.RequestID
		receipt.Status = ExportStatusRevoked
		receipt.Submittable = false
		revokedAt := now
		receipt.RevokedAt = &revokedAt
		if e = tx.Model(&materialExportRecord{}).
			Where("tenant_id=? AND user_id=? AND material_id=? AND id=?", s.TenantID, s.UserID, input.MaterialID, input.ExportID).
			Updates(map[string]any{
				"status":       ExportStatusRevoked,
				"revoked_at":   revokedAt,
				"receipt_body": string(mustJSON(receipt)),
				"updated_at":   now,
			}).Error; e != nil {
			return e
		}
		return tx.Create(&materialReceiptRecord{
			TenantID: s.TenantID, UserID: s.UserID, RequestID: input.RequestID,
			Fingerprint: fingerprint, Body: string(mustJSON(receipt)), CreatedAt: now,
		}).Error
	})
	if err != nil {
		if errors.Is(err, ErrIdempotencyConflict) || errors.Is(err, ErrInvalidRequest) || errors.Is(err, ErrExportNotFound) {
			return ExportReceipt{}, err
		}
		var revisionConflict *RevisionConflictError
		if errors.As(err, &revisionConflict) {
			return ExportReceipt{}, err
		}
		if isReceiptRaceError(err) {
			if replay, found, lookupErr := o.replayExportReceipt(ctx, s, input.RequestID, fingerprint, MaterialKindExportRevoked); lookupErr != nil {
				return ExportReceipt{}, lookupErr
			} else if found {
				return replay, nil
			}
		}
		if ctx.Err() != nil {
			return ExportReceipt{}, &OutcomeUnknownError{RequestID: input.RequestID}
		}
		return ExportReceipt{}, err
	}
	return receipt, nil
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
