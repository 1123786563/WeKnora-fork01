package career

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	secutils "github.com/Tencent/WeKnora/internal/utils"
	"github.com/google/uuid"
)

const careerSourceOwner = "career_source"

type UploadResult struct {
	Upload            SourceUpload
	SourceID          string
	Fields            []ExtractedField
	MissingCategories []string
	ReviewFlags       []string
}

// UploadAdapter owns the narrow, non-session file validation, storage and document-reader seam.
type UploadAdapter struct {
	files   interfaces.FileService
	catalog interfaces.ResourceCatalog
	reader  interfaces.DocumentReader
}

func NewUploadAdapter(files interfaces.FileService, catalog interfaces.ResourceCatalog, reader interfaces.DocumentReader) *UploadAdapter {
	return &UploadAdapter{files: files, catalog: catalog, reader: reader}
}

func (a *UploadAdapter) StoreAndParse(ctx context.Context, tenantID uint64, fileName, declaredMIME string, data []byte, onStored func(UploadResult) error) (UploadResult, error) {
	return a.StoreAndParseWithID(ctx, tenantID, uuid.NewString(), fileName, declaredMIME, data, onStored)
}

func (a *UploadAdapter) StoreAndParseWithID(ctx context.Context, tenantID uint64, sourceID, fileName, declaredMIME string, data []byte, onStored func(UploadResult) error) (UploadResult, error) {
	return a.parseClaim(ctx, tenantID, sourceID, fileName, declaredMIME, data, "", onStored)
}

func (a *UploadAdapter) ResumeAndParse(ctx context.Context, tenantID uint64, sourceID, fileName, declaredMIME string, data []byte, resourceRef string) (UploadResult, error) {
	return a.parseClaim(ctx, tenantID, sourceID, fileName, declaredMIME, data, resourceRef, nil)
}

func (a *UploadAdapter) parseClaim(ctx context.Context, tenantID uint64, sourceID, fileName, declaredMIME string, data []byte, existingRef string, onStored func(UploadResult) error) (UploadResult, error) {
	if a == nil || a.files == nil || a.catalog == nil || a.reader == nil || tenantID == 0 {
		return UploadResult{}, fmt.Errorf("career upload dependencies unavailable")
	}
	safeName, valid := secutils.ValidateInput(strings.TrimSpace(fileName))
	if !valid {
		return UploadResult{}, fmt.Errorf("invalid file name")
	}
	baseName, err := secutils.SafeFileName(safeName)
	if err != nil {
		return UploadResult{}, fmt.Errorf("unsafe file name: %w", err)
	}
	maxSize := int64(secutils.GetMaxFileSizeMB()) * 1024 * 1024
	if len(data) == 0 || int64(len(data)) > maxSize {
		return UploadResult{}, fmt.Errorf("file size must be between 1 byte and %dMB", secutils.GetMaxFileSizeMB())
	}
	ext := strings.ToLower(filepath.Ext(baseName))
	if ext != ".pdf" && ext != ".docx" && ext != ".txt" && ext != ".doc" {
		return UploadResult{}, fmt.Errorf("unsupported resume file type")
	}
	detected := http.DetectContentType(data)
	if err = validateResumeContent(ext, declaredMIME, detected, data); err != nil {
		return UploadResult{}, err
	}
	digest := sha256.Sum256(data)
	result := UploadResult{SourceID: sourceID}
	result.Upload = SourceUpload{ID: result.SourceID, FileName: baseName, MIMEType: strings.TrimSpace(declaredMIME), Size: int64(len(data)), Digest: hex.EncodeToString(digest[:])}
	storedData := data
	resourceRef := existingRef
	if existingRef != "" {
		reader, readErr := a.files.GetFile(ctx, existingRef)
		if readErr != nil {
			return result, fmt.Errorf("read claimed resume: %w", readErr)
		}
		storedData, readErr = io.ReadAll(io.LimitReader(reader, maxSize+1))
		closeErr := reader.Close()
		if readErr != nil {
			return result, readErr
		}
		if closeErr != nil {
			return result, closeErr
		}
		if int64(len(storedData)) > maxSize {
			return result, fmt.Errorf("stored resume exceeds configured limit")
		}
		storedDigest := sha256.Sum256(storedData)
		if hex.EncodeToString(storedDigest[:]) != hex.EncodeToString(digest[:]) {
			return result, ErrIdempotencyConflict
		}
	} else {
		resourceRef, err = a.files.SaveBytes(ctx, data, tenantID, "career_source_"+result.SourceID+ext, false)
		if err != nil {
			return result, &OutcomeUnknownError{RequestID: result.SourceID}
		}
		result.Upload.ResourceRef = resourceRef
		if onStored != nil {
			if err := onStored(result); err != nil {
				return result, &OutcomeUnknownError{RequestID: result.SourceID}
			}
		}
		if err := a.catalog.Bind(ctx, resourceRef, careerSourceOwner, result.SourceID, types.ResourceRelationSourceFile); err != nil {
			return result, fmt.Errorf("bind stored resume source: %w", err)
		}
	}
	result.Upload.ResourceRef = resourceRef
	if existingRef != "" {
		if err := a.catalog.Bind(ctx, resourceRef, careerSourceOwner, result.SourceID, types.ResourceRelationSourceFile); err != nil {
			return result, fmt.Errorf("bind resume source: %w", err)
		}
	}
	var extractedText string
	if ext == ".txt" {
		extractedText = string(storedData)
	} else {
		parsed, parseErr := a.reader.Read(ctx, &types.ReadRequest{FileContent: storedData, FileName: baseName, FileType: strings.TrimPrefix(ext, "."), RequestID: result.SourceID})
		if parseErr != nil {
			return result, fmt.Errorf("parse resume: %w", parseErr)
		}
		if parsed == nil || strings.TrimSpace(parsed.Error) != "" || strings.TrimSpace(parsed.MarkdownContent) == "" {
			if parsed != nil && strings.TrimSpace(parsed.Error) != "" {
				return result, fmt.Errorf("parse resume: %s", parsed.Error)
			}
			return result, fmt.Errorf("parse resume: document reader returned no text")
		}
		extractedText = parsed.MarkdownContent
	}
	if strings.TrimSpace(extractedText) == "" {
		return result, fmt.Errorf("parse resume: extracted text is empty")
	}
	result.Upload.Text = extractedText
	result.Fields, result.MissingCategories, result.ReviewFlags = ExtractResumeFields(extractedText)
	return result, nil
}

func (a *UploadAdapter) Release(ctx context.Context, reference, sourceID string) error {
	if a == nil || a.catalog == nil || a.files == nil {
		return nil
	}
	remaining, err := a.catalog.Release(ctx, reference, careerSourceOwner, sourceID)
	if _, cataloged := types.ParseResourcePath(reference); !cataloged {
		if err != nil {
			return err
		}
		if remaining > 0 {
			return nil
		}
		return a.files.DeleteFile(ctx, reference)
	}
	if err == nil && remaining > 0 {
		return nil
	}
	scope, scopeErr := getScope(ctx)
	if scopeErr != nil {
		return scopeErr
	}
	guarded, ok := a.files.(interfaces.UnboundResourceDeleter)
	if !ok {
		return fmt.Errorf("guarded resource deletion unavailable")
	}
	deleted, deleteErr := guarded.DeleteUnbound(ctx, scope.TenantID, reference)
	if err != nil {
		if deleteErr != nil {
			return errors.Join(err, deleteErr)
		}
		if !deleted {
			return err
		}
	}
	return deleteErr
}

func validateResumeContent(ext, declared, detected string, data []byte) error {
	declared = strings.ToLower(strings.TrimSpace(strings.Split(declared, ";")[0]))
	switch ext {
	case ".pdf":
		if !bytes.HasPrefix(data, []byte("%PDF-")) {
			return fmt.Errorf("resume content does not match PDF type")
		}
		if declared != "" && declared != "application/pdf" {
			return fmt.Errorf("resume MIME type does not match PDF type")
		}
	case ".docx":
		if !bytes.HasPrefix(data, []byte("PK\x03\x04")) {
			return fmt.Errorf("resume content does not match DOCX type")
		}
		if declared != "" && declared != "application/vnd.openxmlformats-officedocument.wordprocessingml.document" && declared != "application/zip" {
			return fmt.Errorf("resume MIME type does not match DOCX type")
		}
	case ".doc":
		if len(data) < 8 || !bytes.Equal(data[:8], []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}) {
			return fmt.Errorf("resume content does not match DOC type")
		}
		if declared != "" && declared != "application/msword" {
			return fmt.Errorf("resume MIME type does not match DOC type")
		}
	case ".txt":
		if strings.Contains(detected, "application/octet-stream") || !utf8.Valid(data) || !utf8Text(data) {
			return fmt.Errorf("resume content does not match text type")
		}
		if declared != "" && declared != "text/plain" && mime.TypeByExtension(ext) != declared {
			return fmt.Errorf("resume MIME type does not match text type")
		}
	}
	return nil
}

func utf8Text(data []byte) bool {
	// Reject binary control bytes while allowing tab, CR, LF and UTF-8 bytes.
	for _, b := range data {
		if b == 0 || b < 0x09 || b > 0x0D && b < 0x20 {
			return false
		}
	}
	return true
}
