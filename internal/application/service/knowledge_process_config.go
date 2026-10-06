package service

import (
	"context"
	"reflect"
	"strconv"
	"strings"

	werrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
)

const xlsxFirstRowAsHeaderOverride = "xlsx_first_row_as_header"

func applyParserRuleOverrides(
	overrides map[string]string,
	config types.ChunkingConfig,
	fileType string,
) {
	fileType = normalizeParserFileType(fileType)
	if fileType != "xlsx" && fileType != "xls" {
		return
	}
	rule := config.ResolveParserEngineRule(fileType)
	if rule == nil || rule.XLSXFirstRowAsHeader == nil {
		return
	}
	engine := strings.TrimSpace(rule.Engine)
	if engine != "" && engine != "builtin" {
		return
	}
	overrides[xlsxFirstRowAsHeaderOverride] = strconv.FormatBool(*rule.XLSXFirstRowAsHeader)
}

func normalizeParserFileType(fileType string) string {
	return strings.TrimPrefix(strings.ToLower(strings.TrimSpace(fileType)), ".")
}

// ResolveProcessConfig merges KB defaults with per-upload overrides for the parse pipeline.
func ResolveProcessConfig(kb *types.KnowledgeBase, overrides *types.KnowledgeProcessOverrides) types.EffectiveProcessConfig {
	imageCfg := kb.ImageProcessingConfig
	eff := types.EffectiveProcessConfig{
		SummaryEnabled:           true,
		ChunkingConfig:           kb.ChunkingConfig,
		EnableMultimodel:         kb.IsMultimodalEnabled(),
		VLMConfig:                kb.VLMConfig,
		ASRConfig:                kb.ASRConfig,
		QuestionGenerationConfig: defaultQuestionGenerationConfig(kb),
		GraphEnabled:             kb.IsGraphEnabled(),
		ExtractConfig:            derefExtractConfig(kb.ExtractConfig),
		ImageAttrsEnabled:        imageCfg.ImageAttrsEnabled,
		ImageActions:             types.ResolveImageActions(imageCfg.ImageActions),
	}
	if overrides == nil {
		return eff
	}
	if overrides.SummaryEnabled != nil {
		eff.SummaryEnabled = *overrides.SummaryEnabled
	}

	if overrides.ChunkingConfig != nil {
		eff.ChunkingConfig = mergeChunkingConfig(eff.ChunkingConfig, overrides.ChunkingConfig)
	}
	if len(overrides.ParserEngineRules) > 0 {
		eff.ChunkingConfig.ParserEngineRules = overrides.ParserEngineRules
	}
	if overrides.EnableMultimodel != nil {
		eff.EnableMultimodel = *overrides.EnableMultimodel
	}
	if overrides.VLMConfig != nil {
		base := eff.VLMConfig
		eff.VLMConfig = *overrides.VLMConfig
		if eff.VLMConfig.DescriptionLanguage == "" {
			eff.VLMConfig.DescriptionLanguage = base.DescriptionLanguage
		}
		if eff.VLMConfig.CustomInstructions == "" {
			eff.VLMConfig.CustomInstructions = base.CustomInstructions
		}
	}
	if overrides.ASRConfig != nil {
		eff.ASRConfig = *overrides.ASRConfig
	}
	if overrides.QuestionGenerationConfig != nil {
		base := eff.QuestionGenerationConfig
		eff.QuestionGenerationConfig = *overrides.QuestionGenerationConfig
		if eff.QuestionGenerationConfig.CustomInstructions == "" {
			eff.QuestionGenerationConfig.CustomInstructions = base.CustomInstructions
		}
	}
	if overrides.GraphEnabled != nil {
		eff.GraphEnabled = *overrides.GraphEnabled
	}
	if overrides.ImageAttrsEnabled != nil {
		eff.ImageAttrsEnabled = *overrides.ImageAttrsEnabled
	}
	if overrides.ImageActions != nil {
		base := eff.ImageActions
		// Same rule as types.MergeImageActions: the OCR clause is a unit keyed
		// on a non-empty On. OnUnobserved is a plain bool, so an override that
		// omits On cannot tell "false" from "unset" and is ignored rather than
		// flipping the conservative default.
		if len(overrides.ImageActions.OCR.On) > 0 {
			base.OCR = overrides.ImageActions.OCR
		}
		eff.ImageActions = base
	}
	if overrides.ExtractConfig != nil {
		eff.ExtractConfig = mergeExtractConfig(eff.ExtractConfig, overrides.ExtractConfig)
	}

	// Match KnowledgeBase.IsGraphEnabled: graph fan-out requires extract to be on.
	eff.GraphEnabled = eff.GraphEnabled && eff.ExtractConfig.Enabled

	return eff
}

// validateDefaultFileImportRequirements enforces the VLM/ASR prerequisites that
// ValidateProcessOverrides would otherwise cover, for imports that ship no
// per-import overrides and therefore fall back to the KB defaults.
func validateDefaultFileImportRequirements(
	ctx context.Context,
	kb *types.KnowledgeBase,
	eff types.EffectiveProcessConfig,
	fileType string,
) error {
	fileType = normalizeFileExtension(fileType)
	if IsImageType(fileType) && !eff.VLMConfig.IsEnabled() {
		logger.Error(ctx, "VLM model is not configured")
		return werrors.NewBadRequestError("上传图片文件需要设置VLM模型")
	}
	if IsAudioType(fileType) && !kb.ASRConfig.IsASREnabled() {
		logger.Error(ctx, "ASR model is not configured")
		return werrors.NewBadRequestError("上传音频文件需要设置ASR语音识别模型")
	}
	return nil
}

// explicitProcessOverrides narrows request overrides to the subset that
// actually deviates from what the knowledge base would resolve on its own.
//
// Upload confirm dialogs POST their whole form state as process_config — every
// section, prefilled from KB defaults. Persisting that verbatim froze the
// upload-time KB settings as per-document overrides, so a later KB change
// (e.g. disabling question generation) never reached the document on reparse:
// ResolveProcessConfig kept replaying the frozen snapshot (#3851). Recording
// only the deviations keeps the contract reparse assumes: fields the user
// explicitly changed stay pinned to the document, everything else keeps
// following the KB's latest config.
//
// Resolution is untouched: ResolveProcessConfig(kb, explicit) equals
// ResolveProcessConfig(kb, overrides) because every dropped field resolves to
// the KB value it was compared against. nil means "no deviation" — nothing
// worth persisting.
func explicitProcessOverrides(
	kb *types.KnowledgeBase,
	overrides *types.KnowledgeProcessOverrides,
) *types.KnowledgeProcessOverrides {
	if overrides == nil {
		return nil
	}
	base := ResolveProcessConfig(kb, nil)
	eff := ResolveProcessConfig(kb, overrides)
	explicit := types.KnowledgeProcessOverrides{}
	deviates := false

	if overrides.SummaryEnabled != nil && eff.SummaryEnabled != base.SummaryEnabled {
		explicit.SummaryEnabled = overrides.SummaryEnabled
		deviates = true
	}
	if !reflect.DeepEqual(eff.ChunkingConfig, base.ChunkingConfig) {
		if len(overrides.ParserEngineRules) > 0 {
			explicit.ParserEngineRules = overrides.ParserEngineRules
			deviates = true
		}
		if overrides.ChunkingConfig != nil {
			explicit.ChunkingConfig = overrides.ChunkingConfig
			deviates = true
		}
	}
	if overrides.EnableMultimodel != nil && eff.EnableMultimodel != base.EnableMultimodel {
		explicit.EnableMultimodel = overrides.EnableMultimodel
		deviates = true
	}
	if overrides.VLMConfig != nil && !reflect.DeepEqual(eff.VLMConfig, base.VLMConfig) {
		explicit.VLMConfig = overrides.VLMConfig
		deviates = true
	}
	if overrides.ASRConfig != nil && !reflect.DeepEqual(eff.ASRConfig, base.ASRConfig) {
		explicit.ASRConfig = overrides.ASRConfig
		deviates = true
	}
	if overrides.QuestionGenerationConfig != nil && !reflect.DeepEqual(eff.QuestionGenerationConfig, base.QuestionGenerationConfig) {
		explicit.QuestionGenerationConfig = overrides.QuestionGenerationConfig
		deviates = true
	}
	if overrides.GraphEnabled != nil && eff.GraphEnabled != base.GraphEnabled {
		explicit.GraphEnabled = overrides.GraphEnabled
		deviates = true
	}
	if overrides.ImageAttrsEnabled != nil && eff.ImageAttrsEnabled != base.ImageAttrsEnabled {
		explicit.ImageAttrsEnabled = overrides.ImageAttrsEnabled
		deviates = true
	}
	if overrides.ImageActions != nil && !reflect.DeepEqual(eff.ImageActions, base.ImageActions) {
		explicit.ImageActions = overrides.ImageActions
		deviates = true
	}
	if overrides.ExtractConfig != nil && !reflect.DeepEqual(eff.ExtractConfig, base.ExtractConfig) {
		explicit.ExtractConfig = overrides.ExtractConfig
		deviates = true
	}
	// Parser-engine key/value overrides have no KB-level baseline; the request
	// is their only source, so any non-empty map is an explicit choice.
	if len(overrides.ParserEngineOverrides) > 0 {
		explicit.ParserEngineOverrides = overrides.ParserEngineOverrides
		deviates = true
	}

	if !deviates {
		return nil
	}
	return &explicit
}

// mergeProcessOverrides layers reparse-request overrides on top of the
// overrides a knowledge already stores: a field the request supplies replaces
// the stored choice, a field it omits keeps the upload-time explicit override.
// Nil operands fall through to the other side.
func mergeProcessOverrides(stored, request *types.KnowledgeProcessOverrides) *types.KnowledgeProcessOverrides {
	if stored == nil {
		return request
	}
	if request == nil {
		return stored
	}
	merged := *stored
	if request.SummaryEnabled != nil {
		merged.SummaryEnabled = request.SummaryEnabled
	}
	if len(request.ParserEngineRules) > 0 {
		merged.ParserEngineRules = request.ParserEngineRules
	}
	if request.ChunkingConfig != nil {
		merged.ChunkingConfig = request.ChunkingConfig
	}
	if request.EnableMultimodel != nil {
		merged.EnableMultimodel = request.EnableMultimodel
	}
	if request.VLMConfig != nil {
		merged.VLMConfig = request.VLMConfig
	}
	if request.ASRConfig != nil {
		merged.ASRConfig = request.ASRConfig
	}
	if request.QuestionGenerationConfig != nil {
		merged.QuestionGenerationConfig = request.QuestionGenerationConfig
	}
	if request.GraphEnabled != nil {
		merged.GraphEnabled = request.GraphEnabled
	}
	if request.ImageAttrsEnabled != nil {
		merged.ImageAttrsEnabled = request.ImageAttrsEnabled
	}
	if request.ImageActions != nil {
		merged.ImageActions = request.ImageActions
	}
	if request.ExtractConfig != nil {
		merged.ExtractConfig = request.ExtractConfig
	}
	if len(request.ParserEngineOverrides) > 0 {
		merged.ParserEngineOverrides = request.ParserEngineOverrides
	}
	return &merged
}

// resolveFileImportProcessConfig is the single gate every file import passes
// through: it rejects unsupported extensions, enforces the VLM/ASR
// prerequisites for the resolved type, and returns the effective processing
// config for task enqueue. Persisting overrides onto the knowledge record stays
// with the caller, which owns the record's lifecycle — but every caller
// persists the very struct passed here, so this gate narrows it to the explicit
// deviations (#3851): what callers store is the deviation record, never the
// dialog's prefilled full snapshot.
func resolveFileImportProcessConfig(
	ctx context.Context,
	kb *types.KnowledgeBase,
	fileType string,
	processOverrides *types.KnowledgeProcessOverrides,
	enableMultimodel *bool,
) (types.EffectiveProcessConfig, error) {
	if err := validateImportFileType(fileType); err != nil {
		return types.EffectiveProcessConfig{}, err
	}

	eff := ResolveProcessConfig(kb, processOverrides)
	if enableMultimodel != nil && (processOverrides == nil || processOverrides.EnableMultimodel == nil) {
		eff.EnableMultimodel = *enableMultimodel
	}

	if processOverrides != nil {
		if err := ValidateProcessOverrides(ctx, kb, processOverrides, []string{fileType}); err != nil {
			return eff, err
		}
		if explicit := explicitProcessOverrides(kb, processOverrides); explicit != nil {
			*processOverrides = *explicit
		} else {
			*processOverrides = types.KnowledgeProcessOverrides{}
		}
	} else if err := validateDefaultFileImportRequirements(ctx, kb, eff, fileType); err != nil {
		return eff, err
	}

	return eff, nil
}

// ValidateProcessOverrides validates batch overrides against file types in the upload.
func ValidateProcessOverrides(
	ctx context.Context,
	kb *types.KnowledgeBase,
	overrides *types.KnowledgeProcessOverrides,
	fileTypes []string,
) error {
	if overrides == nil {
		return nil
	}

	hasImage := false
	hasAudio := false
	for _, ft := range fileTypes {
		if IsImageType(ft) {
			hasImage = true
		}
		if IsAudioType(ft) {
			hasAudio = true
		}
	}

	eff := ResolveProcessConfig(kb, overrides)

	if hasImage {
		if !eff.VLMConfig.IsEnabled() {
			return werrors.NewBadRequestError("上传图片文件需要设置VLM模型")
		}
	}

	if hasAudio && !eff.ASRConfig.IsASREnabled() {
		return werrors.NewBadRequestError("上传音频文件需要设置ASR语音识别模型")
	}

	if err := types.ValidateEffectiveProcessPromptInstructions(eff); err != nil {
		return werrors.NewBadRequestError(err.Error())
	}

	return nil
}

// ApplyKnowledgeProcessOverrides validates optional overrides, persists them on the
// knowledge record, and returns the effective config for task enqueue. Only
// the fields that deviate from the KB's own config are persisted (#3851), so
// the stored overrides stay the document's explicit choices rather than a
// frozen copy of the upload-time KB settings.
func ApplyKnowledgeProcessOverrides(
	ctx context.Context,
	kb *types.KnowledgeBase,
	knowledge *types.Knowledge,
	processOverrides *types.KnowledgeProcessOverrides,
	fileTypes []string,
	enableMultimodel *bool,
) (types.EffectiveProcessConfig, error) {
	eff := ResolveProcessConfig(kb, processOverrides)
	if enableMultimodel != nil && (processOverrides == nil || processOverrides.EnableMultimodel == nil) {
		eff.EnableMultimodel = *enableMultimodel
	}
	if processOverrides == nil {
		return eff, nil
	}
	if err := ValidateProcessOverrides(ctx, kb, processOverrides, fileTypes); err != nil {
		return eff, err
	}
	if err := knowledge.SetProcessOverrides(explicitProcessOverrides(kb, processOverrides)); err != nil {
		return eff, err
	}
	return eff, nil
}

// reparseFileTypes derives the file types used to validate overrides on reparse.
// Manual knowledge has no file; URL imports validate as html.
func reparseFileTypes(k *types.Knowledge) []string {
	if k == nil || k.IsManual() {
		return nil
	}
	if k.Type == "url" {
		return []string{"html"}
	}
	ft := k.FileType
	if ft == "" && k.FileName != "" {
		ft = getFileType(k.FileName)
	}
	if ft == "" {
		return nil
	}
	return []string{ft}
}

func defaultQuestionGenerationConfig(kb *types.KnowledgeBase) types.QuestionGenerationConfig {
	if kb == nil || kb.QuestionGenerationConfig == nil {
		return types.QuestionGenerationConfig{}
	}
	return *kb.QuestionGenerationConfig
}

func derefExtractConfig(cfg *types.ExtractConfig) types.ExtractConfig {
	if cfg == nil {
		return types.ExtractConfig{}
	}
	return *cfg
}

func mergeChunkingConfig(base types.ChunkingConfig, override *types.ChunkingConfig) types.ChunkingConfig {
	if override == nil {
		return base
	}
	result := base
	if override.ChunkSize != 0 {
		result.ChunkSize = override.ChunkSize
	}
	if override.ChunkOverlap != 0 {
		result.ChunkOverlap = override.ChunkOverlap
	}
	if len(override.Separators) > 0 {
		result.Separators = override.Separators
	}
	if len(override.ParserEngineRules) > 0 {
		result.ParserEngineRules = override.ParserEngineRules
	}
	// EnableParentChild is authoritative: callers send a full chunking snapshot,
	// so an explicit false must be able to turn parent-child off (not just on).
	result.EnableParentChild = override.EnableParentChild
	if override.ParentChunkSize != 0 {
		result.ParentChunkSize = override.ParentChunkSize
	}
	if override.ChildChunkSize != 0 {
		result.ChildChunkSize = override.ChildChunkSize
	}
	if override.Strategy != "" {
		result.Strategy = override.Strategy
	}
	if override.TokenLimit != 0 {
		result.TokenLimit = override.TokenLimit
	}
	if len(override.Languages) > 0 {
		result.Languages = override.Languages
	}
	if override.TableMetadataInstructions != "" {
		result.TableMetadataInstructions = override.TableMetadataInstructions
	}
	return result
}

func mergeExtractConfig(base types.ExtractConfig, override *types.ExtractConfig) types.ExtractConfig {
	if override == nil {
		return base
	}
	result := base
	result.Enabled = override.Enabled
	if override.Text != "" {
		result.Text = override.Text
	}
	if len(override.Tags) > 0 {
		result.Tags = override.Tags
	}
	if len(override.Nodes) > 0 {
		result.Nodes = override.Nodes
	}
	if len(override.Relations) > 0 {
		result.Relations = override.Relations
	}
	if override.CustomInstructions != "" {
		result.CustomInstructions = override.CustomInstructions
	}
	return result
}

// MergeParserEngineOverrides merges upload overrides on top of tenant overrides safely.
func MergeParserEngineOverrides(tenantOverrides map[string]string, uploadOverrides map[string]string) map[string]string {
	merged := make(map[string]string)
	for k, v := range tenantOverrides {
		merged[k] = v
	}
	for k, v := range uploadOverrides {
		merged[k] = v
	}
	return merged
}
