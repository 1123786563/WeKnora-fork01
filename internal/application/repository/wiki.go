package repository

import (
	"errors"
	"strings"
)

// EscapeLikePattern escapes LIKE / ILIKE metacharacters so the returned string
// can be safely concatenated with % wildcards without unintended matches.
// Order matters: escape the backslash first, then the wildcards.
func EscapeLikePattern(s string) string {
	replacer := strings.NewReplacer(
		`\`, `\\`,
		`%`, `\%`,
		`_`, `\_`,
	)
	return replacer.Replace(s)
}

// ErrWikiPageNotFound is returned when a wiki page is not found
var ErrWikiPageNotFound = errors.New("wiki page not found")

// ErrWikiPageConflict is returned when an optimistic lock conflict is detected
var ErrWikiPageConflict = errors.New("wiki page version conflict")

// ErrWikiFolderNotFound is returned when a wiki folder is not found.
var ErrWikiFolderNotFound = errors.New("wiki folder not found")

// ErrWikiFolderConflict is returned when a sibling folder with the same name
// already exists under the same parent.
var ErrWikiFolderConflict = errors.New("wiki folder name conflict")

// ErrWikiFolderNotEmpty is returned when a folder still has a live page or
// child folder at the instant an atomic delete is attempted.
var ErrWikiFolderNotEmpty = errors.New("wiki folder is not empty")
