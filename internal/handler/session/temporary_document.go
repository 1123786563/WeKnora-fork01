package session

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/filetransport"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	secutils "github.com/Tencent/WeKnora/internal/utils"
	"github.com/gin-gonic/gin"
)

// attachmentImageExtensions are extensions whose content sniffs reliably as
// image/*, so a declared mismatch proves client-side MIME spoofing. TIFF is
// deliberately absent: Go's DetectContentType does not recognize it, so
// enforcing the family there would reject legitimate TIFF uploads.
var attachmentImageExtensions = map[string]struct{}{
	".jpg": {}, ".jpeg": {}, ".png": {}, ".gif": {}, ".bmp": {}, ".webp": {},
}

// attachmentSniffableImageMimes mirrors attachmentImageExtensions for
// client-declared MIME types.
var attachmentSniffableImageMimes = map[string]struct{}{
	"image/jpeg": {}, "image/png": {}, "image/gif": {}, "image/bmp": {}, "image/webp": {},
}

// attachmentExecutableSniffs are content types no chat attachment may claim,
// regardless of the declared MIME or file name.
var attachmentExecutableSniffs = map[string]struct{}{
	"application/x-msdownload":    {},
	"application/x-elf":           {},
	"application/x-executable":    {},
	"application/x-sharedlib":     {},
	"application/x-mach-binary":   {},
	"application/x-java-vm":       {},
	"application/x-dosexec":       {},
	"application/x-msdos-program": {},
}

// attachmentExecutableMagics covers binaries Go's DetectContentType does not
// name (it returns application/octet-stream for them): Windows PE ("MZ"),
// Linux ELF, Mach-O and Java class files.
var attachmentExecutableMagics = [][]byte{
	{'M', 'Z'},
	{0x7f, 'E', 'L', 'F'},
	{0xFE, 0xED, 0xFA, 0xCE},
	{0xFE, 0xED, 0xFA, 0xCF},
	{0xCE, 0xFA, 0xED, 0xFE},
	{0xCF, 0xFA, 0xED, 0xFE},
	{0xCA, 0xFE, 0xBA, 0xBE},
}

func hasAttachmentExecutableMagic(head []byte) bool {
	for _, magic := range attachmentExecutableMagics {
		if bytes.HasPrefix(head, magic) {
			return true
		}
	}
	return false
}

// validateAttachmentContent inspects the real bytes of a session attachment
// upload (mobile or web) before anything is persisted: zero-byte payloads,
// oversized bodies, executable content and MIME families that contradict the
// sniffed bytes are rejected, and an optional client-supplied sha256 digest is
// verified against the actual content. It returns the effective MIME type to
// store — the declared type when present and not contradicted, otherwise the
// sniffed one.
func validateAttachmentContent(fileName, declaredMIME string, data []byte, wantDigest string) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("attachment is empty")
	}
	maxSize := secutils.GetMaxFileSizeMB() * 1024 * 1024
	if int64(len(data)) > maxSize {
		return "", fmt.Errorf("file exceeds size limit of %dMB", secutils.GetMaxFileSizeMB())
	}
	head := data
	if len(head) > 512 {
		head = head[:512]
	}
	sniffed := http.DetectContentType(head)
	if _, blocked := attachmentExecutableSniffs[sniffed]; blocked {
		return "", fmt.Errorf("attachment content type %q is not allowed", sniffed)
	}
	if hasAttachmentExecutableMagic(head) {
		return "", fmt.Errorf("attachment contains executable content")
	}
	ext := strings.ToLower(filepath.Ext(fileName))
	_, extIsImage := attachmentImageExtensions[ext]
	declaredIsImage := false
	if declared := strings.ToLower(strings.TrimSpace(declaredMIME)); declared != "" {
		_, declaredIsImage = attachmentSniffableImageMimes[declared]
	}
	sniffedIsImage := strings.HasPrefix(sniffed, "image/")
	if (extIsImage || declaredIsImage) && !sniffedIsImage {
		return "", fmt.Errorf("attachment content (%s) does not match its declared image type", sniffed)
	}
	if sniffedIsImage && !extIsImage && !declaredIsImage {
		return "", fmt.Errorf("attachment content (%s) does not match its declared type %q", sniffed, declaredMIME)
	}
	if wantDigest != "" {
		sum := sha256.Sum256(data)
		if !strings.EqualFold(strings.TrimSpace(wantDigest), hex.EncodeToString(sum[:])) {
			return "", fmt.Errorf("attachment digest mismatch")
		}
	}
	if strings.TrimSpace(declaredMIME) != "" {
		return strings.TrimSpace(declaredMIME), nil
	}
	return sniffed, nil
}

// UploadTemporaryDocument accepts one multipart file and immediately returns
// a session-scoped document ID. Parsing continues in the document worker.
func (h *Handler) UploadTemporaryDocument(c *gin.Context) {
	ctx := c.Request.Context()
	sessionID := c.Param("session_id")
	// Uploading attaches content to the session, so use the strict owner scope:
	// a tenant admin may read an API-key session but must not add attachments.
	if _, err := h.sessionService.GetOwnedSession(ctx, sessionID); err != nil {
		c.Error(apperrors.NewNotFoundError("Session not found"))
		return
	}
	maxBytes := secutils.GetMaxFileSizeMB()*1024*1024 + 1024*1024
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.Error(apperrors.NewBadRequestError(fmt.Sprintf("invalid attachment upload: %v", err)))
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		c.Error(apperrors.NewBadRequestError("failed to open attachment"))
		return
	}
	defer file.Close()

	// Read the real bytes up front so size, MIME family and digest are all
	// verified against the actual content, never the client's declarations.
	data, err := io.ReadAll(io.LimitReader(file, secutils.GetMaxFileSizeMB()*1024*1024+1))
	if err != nil {
		c.Error(apperrors.NewBadRequestError("failed to read attachment"))
		return
	}
	wantDigest := strings.TrimSpace(c.PostForm("sha256"))
	effectiveMIME, validateErr := validateAttachmentContent(fileHeader.Filename, fileHeader.Header.Get("Content-Type"), data, wantDigest)
	if validateErr != nil {
		c.Error(apperrors.NewBadRequestError(validateErr.Error()))
		return
	}

	sourceTenantID, parseErr := types.ParseAgentSourceTenantID(c.PostForm(types.AgentSourceTenantIDParam))
	if parseErr != nil {
		c.Error(apperrors.NewBadRequestError(parseErr.Error()))
		return
	}
	agent, resourceTenantID, _ := h.resolveAgent(ctx, c, c.PostForm("agent_id"), sourceTenantID)
	if sourceTenantID != 0 && agent == nil {
		c.Error(apperrors.NewNotFoundError("Shared agent not found"))
		return
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(fileHeader.Filename)), ".")
	options := types.TemporaryDocumentCreateOptions{ParserEngine: strings.TrimSpace(c.PostForm("parser_engine"))}
	if agent != nil {
		options.ResourceTenantID = resourceTenantID
		if len(agent.Config.SupportedFileTypes) > 0 && !containsFileType(agent.Config.SupportedFileTypes, ext) {
			c.Error(apperrors.NewBadRequestError("file type is not supported by this agent"))
			return
		}
		if isAudioExtension(ext) {
			if !agent.Config.AudioUploadEnabled || agent.Config.ASRModelID == "" {
				c.Error(apperrors.NewBadRequestError("audio upload is not enabled or no ASR model is configured"))
				return
			}
			options.ASRModelID = agent.Config.ASRModelID
		}
		// Resolve the parser engine from the agent's chat rules when the caller
		// did not pass an explicit engine. Tenant-level rules remain the final
		// fallback and are applied in the parse worker.
		if options.ParserEngine == "" || options.ParserEngine == "auto" {
			if engine := agent.Config.ResolveChatParserEngine(ext); engine != "" {
				options.ParserEngine = engine
			}
		}
		// Image understanding (caption/OCR) uses the agent's VLM model. Images
		// always benefit; scanned/image-only documents only OCR when the agent
		// enabled AttachmentImageUnderstanding (see parse worker threshold gate).
		if agent.Config.ImageUploadEnabled && agent.Config.VLMModelID != "" {
			options.VLMModelID = agent.Config.VLMModelID
			options.ImageUnderstanding = agent.Config.AttachmentImageUnderstanding
			options.OCRMaxPages = agent.Config.AttachmentOCRMaxPages
		}
	}
	document, err := h.temporaryDocuments.Create(
		ctx, c.GetUint64(types.TenantIDContextKey.String()), sessionID,
		fileHeader.Filename, effectiveMIME, int64(len(data)), bytes.NewReader(data), options,
	)
	if err != nil {
		c.Error(apperrors.NewBadRequestError(err.Error()))
		return
	}
	if wantDigest != "" {
		sum := sha256.Sum256(data)
		logger.Infof(ctx, "session attachment uploaded: session_id=%s attachment_id=%s size=%d sha256=%s",
			sessionID, document.ID, len(data), hex.EncodeToString(sum[:]))
	}
	c.JSON(http.StatusAccepted, gin.H{"success": true, "data": document})
}

func (h *Handler) ListTemporaryDocuments(c *gin.Context) {
	ctx := c.Request.Context()
	sessionID := sessionIDParam(c)
	if _, err := h.sessionService.GetSession(ctx, sessionID); err != nil {
		c.Error(apperrors.NewNotFoundError("Session not found"))
		return
	}
	documents, err := h.temporaryDocuments.List(ctx, c.GetUint64(types.TenantIDContextKey.String()), sessionID)
	if err != nil {
		c.Error(apperrors.NewInternalServerError(err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": documents})
}

func (h *Handler) GetTemporaryDocument(c *gin.Context) {
	ctx := c.Request.Context()
	sessionID := sessionIDParam(c)
	if _, err := h.sessionService.GetSession(ctx, sessionID); err != nil {
		c.Error(apperrors.NewNotFoundError("Session not found"))
		return
	}
	document, err := h.temporaryDocuments.Get(ctx, c.GetUint64(types.TenantIDContextKey.String()), sessionID, c.Param("attachment_id"))
	if err != nil {
		c.Error(apperrors.NewInternalServerError(err.Error()))
		return
	}
	if document == nil {
		c.Error(apperrors.NewNotFoundError("Attachment not found"))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": document})
}

func (h *Handler) PreviewTemporaryDocument(c *gin.Context) {
	ctx := c.Request.Context()
	sessionID := sessionIDParam(c)
	if _, err := h.sessionService.GetSession(ctx, sessionID); err != nil {
		c.Error(apperrors.NewNotFoundError("Session not found"))
		return
	}
	attachmentID := secutils.SanitizeForLog(c.Param("attachment_id"))
	if attachmentID == "" {
		c.Error(apperrors.NewBadRequestError("Attachment ID cannot be empty"))
		return
	}
	file, filename, err := h.temporaryDocuments.OpenFile(
		ctx, c.GetUint64(types.TenantIDContextKey.String()), sessionID, attachmentID,
	)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			c.Error(apperrors.NewNotFoundError("Attachment not found"))
			return
		}
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(apperrors.NewInternalServerError("Failed to retrieve attachment").WithDetails(err.Error()))
		return
	}
	if err := filetransport.Serve(c.Writer, c.Request, file, filetransport.Options{
		Filename: filename, CacheControl: "private, no-store",
	}); err != nil {
		logger.Errorf(ctx, "Failed to stream attachment preview: %v", err)
	}
}

func (h *Handler) DeleteTemporaryDocument(c *gin.Context) {
	ctx := c.Request.Context()
	sessionID := sessionIDParam(c)
	// Deleting mutates the session's attachments, so use the strict owner scope:
	// a tenant admin may read an API-key session but must not remove attachments.
	if _, err := h.sessionService.GetOwnedSession(ctx, sessionID); err != nil {
		c.Error(apperrors.NewNotFoundError("Session not found"))
		return
	}
	if err := h.temporaryDocuments.Delete(ctx, c.GetUint64(types.TenantIDContextKey.String()), sessionID, c.Param("attachment_id")); err != nil {
		c.Error(apperrors.NewInternalServerError(err.Error()))
		return
	}
	c.Status(http.StatusNoContent)
}

func sessionIDParam(c *gin.Context) string {
	if value := c.Param("session_id"); value != "" {
		return value
	}
	return c.Param("id")
}

func containsFileType(supported []string, ext string) bool {
	for _, item := range supported {
		if strings.TrimPrefix(strings.ToLower(strings.TrimSpace(item)), ".") == ext {
			return true
		}
	}
	return false
}

func isAudioExtension(ext string) bool {
	switch ext {
	case "mp3", "wav", "m4a", "flac", "ogg", "aac":
		return true
	default:
		return false
	}
}
