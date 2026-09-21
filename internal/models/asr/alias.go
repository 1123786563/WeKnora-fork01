// Pass A compatibility alias for github.com/Tencent/WeKnora/internal/modules/airesource/models/asr — zero logic. Deleted by Pass B task B-airesource.
package asr

import "github.com/Tencent/WeKnora/internal/modules/airesource/models/asr"

// Type aliases to the moved package.
type ASR = asr.ASR
type Config = asr.Config
type OpenAIASR = asr.OpenAIASR
type Segment = asr.Segment
type TranscriptionResult = asr.TranscriptionResult

// Functions forwarded to the moved package (function values; zero logic).
var ConfigFromModel = asr.ConfigFromModel
var DetectAudioFormat = asr.DetectAudioFormat
var NewASR = asr.NewASR
var NewOpenAIASR = asr.NewOpenAIASR
