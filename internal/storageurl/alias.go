// Pass A compatibility alias for github.com/Tencent/WeKnora/internal/modules/airesource/storageurl — zero logic. Deleted by Pass B task B-airesource.
package storageurl

import "github.com/Tencent/WeKnora/internal/modules/airesource/storageurl"

// Constants forwarded to the moved package.
const EnvVar = storageurl.EnvVar
const ModeHandle = storageurl.ModeHandle
const ModePublic = storageurl.ModePublic
const QueryParam = storageurl.QueryParam

// Type aliases to the moved package.
type FileServiceResolver = storageurl.FileServiceResolver
type Held = storageurl.Held
type Mode = storageurl.Mode
type Resolver = storageurl.Resolver
type Rewriter = storageurl.Rewriter
type StreamRewriter = storageurl.StreamRewriter

// Variables forwarded to the moved package.
var ErrPublicModeForbidden = storageurl.ErrPublicModeForbidden
var Pattern = storageurl.Pattern

// Functions forwarded to the moved package (function values; zero logic).
var BuildFileServiceForProvider = storageurl.BuildFileServiceForProvider
var DefaultMode = storageurl.DefaultMode
var FindIncompleteMarkdownImage = storageurl.FindIncompleteMarkdownImage
var FindIncompleteRef = storageurl.FindIncompleteRef
var HandleModeForced = storageurl.HandleModeForced
var HoldbackCutoff = storageurl.HoldbackCutoff
var IsHTTPURL = storageurl.IsHTTPURL
var LocalStorageBaseDir = storageurl.LocalStorageBaseDir
var NewFileServiceResolver = storageurl.NewFileServiceResolver
var NewRequestRewriter = storageurl.NewRequestRewriter
var NewRewriter = storageurl.NewRewriter
var NewStreamRewriter = storageurl.NewStreamRewriter
var ParseMode = storageurl.ParseMode
var ResolveMode = storageurl.ResolveMode
var Rewrite = storageurl.Rewrite
var WithForcedHandleMode = storageurl.WithForcedHandleMode
