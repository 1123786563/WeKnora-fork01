// Pass A compatibility alias for github.com/Tencent/WeKnora/internal/modules/airesource/models/utils — zero logic. Deleted by Pass B task B-airesource.
package utils

import "github.com/Tencent/WeKnora/internal/modules/airesource/models/utils"

// Functions forwarded to the moved package (function values; zero logic).
var Sign = utils.Sign

// Generic functions forwarded to the moved package (one-line delegations; zero logic).
func ChunkSlice[T any](slice []T, chunkSize int) [][]T { return utils.ChunkSlice(slice, chunkSize) }
func MapSlice[A any, B any](in []A, f func(A) B) []B   { return utils.MapSlice(in, f) }
