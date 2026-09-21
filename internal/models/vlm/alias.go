// Pass A compatibility alias for github.com/Tencent/WeKnora/internal/modules/airesource/models/vlm — zero logic. Deleted by Pass B task B-airesource.
package vlm

import "github.com/Tencent/WeKnora/internal/modules/airesource/models/vlm"

// Type aliases to the moved package.
type Config = vlm.Config
type OllamaVLM = vlm.OllamaVLM
type RemoteAPIVLM = vlm.RemoteAPIVLM
type VLM = vlm.VLM
type WeKnoraCloudVLM = vlm.WeKnoraCloudVLM

// Functions forwarded to the moved package (function values; zero logic).
var ConfigFromModel = vlm.ConfigFromModel
var NewOllamaVLM = vlm.NewOllamaVLM
var NewRemoteAPIVLM = vlm.NewRemoteAPIVLM
var NewVLM = vlm.NewVLM
var NewVLMFromLegacyConfig = vlm.NewVLMFromLegacyConfig
var NewWeKnoraCloudVLM = vlm.NewWeKnoraCloudVLM
