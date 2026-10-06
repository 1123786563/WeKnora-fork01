// Package chunker - profiler.go scans a document once to gather structure
// indicators that drive strategy selection (heading-aware vs. heuristic vs.
// recursive). Profiling is cheap (a few regex passes plus rune counting)
// and runs before any chunking decision is made.
package chunker

import (
	"math"
	"strings"
)

// DocProfile holds the document-level signals used to choose a chunking tier.
//
// The JSON shape (snake_case via struct tags) is part of the public preview
// endpoint API — keep field names stable. Internal callers should use the Go
// field names; only the preview handler relies on the wire format.
type DocProfile struct {
	TotalChars int     `json:"total_chars"`
	TotalLines int     `json:"total_lines"`
	AvgLineLen float64 `json:"avg_line_len"`
	StdLineLen float64 `json:"std_line_len"`

	// Markdown structure
	MdHeadingCounts map[int]int `json:"md_heading_counts"` // level (1..6) → count
	MdHeadingTotal  int         `json:"md_heading_total"`

	// Heuristic indicators
	NumberedSectionCount  int `json:"numbered_section_count"`
	AllCapsShortLineCount int `json:"all_caps_short_line_count"`
	BlankParagraphBreaks  int `json:"blank_paragraph_breaks"`
	FormFeedCount         int `json:"form_feed_count"`
	VisualSepCount        int `json:"visual_sep_count"`
	GermanChapterCount    int `json:"german_chapter_count"`
	EnglishChapterCount   int `json:"english_chapter_count"`
	ChineseChapterCount   int `json:"chinese_chapter_count"`
	RepeatedFooterCount   int `json:"repeated_footer_count"`

	// Content characteristics
	HasTables bool    `json:"has_tables"`
	HasCode   bool    `json:"has_code"`
	CodeRatio float64 `json:"code_ratio"`

	// Detected language hints (best-effort)
	DetectedLangs []string `json:"detected_langs"`
}

// HeadingDensity returns the share of lines that are Markdown headings.
func (p *DocProfile) HeadingDensity() float64 {
	if p.TotalLines == 0 {
		return 0
	}
	return float64(p.MdHeadingTotal) / float64(p.TotalLines)
}

// DominantHeadingLevel returns the heading level (1..6) that should drive
// section splitting. Preference order:
//  1. The lowest level (closest to root) that has at least 3 occurrences —
//     a "real" structural backbone of the document.
//  2. Otherwise the deepest level present at least once — gives finer-grained
//     boundaries for small documents that just have an H1 + a few H2s.
//
// Returns 0 when no Markdown headings exist.
func (p *DocProfile) DominantHeadingLevel() int {
	if p.MdHeadingTotal == 0 {
		return 0
	}
	for level := 1; level <= 6; level++ {
		if p.MdHeadingCounts[level] >= 3 {
			return level
		}
	}
	for level := 6; level >= 1; level-- {
		if p.MdHeadingCounts[level] > 0 {
			return level
		}
	}
	return 0
}

// HeuristicMarkerTotal sums the non-Markdown structural markers.
func (p *DocProfile) HeuristicMarkerTotal() int {
	return p.NumberedSectionCount +
		p.GermanChapterCount + p.EnglishChapterCount + p.ChineseChapterCount +
		p.AllCapsShortLineCount + p.VisualSepCount + p.FormFeedCount
}

// fenceState tracks the fenced-code-block scanning state shared by the
// profiler and the heading / heuristic boundary scanners. It follows
// CommonMark 0.31.2 §4.5:
//   - A fence opens with ≥3 backticks or ≥3 tildes (up to 3 leading spaces).
//     The info string of a backtick fence must not contain a backtick; tilde
//     fences have no such restriction.
//   - A fence closes only with a run of the SAME character that is at least
//     as long as the opening fence and is followed by nothing but
//     spaces/tabs — so a shorter or "decorated" run inside a longer fence
//     stays literal content.
//   - An unclosed fence extends to the end of the document.
//
// fenceState must be fed lines in document order, one update call per line.
type fenceState struct {
	active bool
	ch     byte // '`' or '~' — character of the opening fence
	length int  // opening fence length; a closing fence must be at least this long
}

// update advances the state by one line and reports whether the line itself
// is a fence delimiter (opening or closing). Callers must treat delimiter
// lines and the content they surround as structure-free text: no heading,
// boundary, or other line-shape signals may be derived from them.
func (f *fenceState) update(line string) bool {
	// At most 3 leading spaces (or tabs) of indentation are allowed.
	i := 0
	for i < len(line) && i < 3 && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	rest := line[i:]
	if !f.active {
		ch, n := fenceRun(rest)
		if n < 3 {
			return false
		}
		info := strings.TrimSpace(rest[n:])
		if ch == '`' && strings.ContainsAny(info, "`") {
			// e.g. the inline-code wrapper "`` `code` ``" is not a fence.
			return false
		}
		f.active, f.ch, f.length = true, ch, n
		return true
	}
	// Closing fence: same character, run ≥ opening length, nothing but
	// spaces/tabs after it.
	ch, n := fenceRun(rest)
	if ch == f.ch && n >= f.length && strings.TrimSpace(rest[n:]) == "" {
		f.active = false
		return true
	}
	return false
}

// fenceRun returns the fence character ('`' or '~') and the length of the run
// at the start of s, or (0, 0) when s does not start with a fence character.
func fenceRun(s string) (byte, int) {
	if len(s) == 0 || (s[0] != '`' && s[0] != '~') {
		return 0, 0
	}
	n := 1
	for n < len(s) && s[n] == s[0] {
		n++
	}
	return s[0], n
}

// ProfileDocument runs a single pass over text and returns its profile.
func ProfileDocument(text string) *DocProfile {
	p := &DocProfile{
		MdHeadingCounts: make(map[int]int),
	}
	if text == "" {
		return p
	}

	p.TotalChars = len([]rune(text))
	p.FormFeedCount = strings.Count(text, "\f")

	lines := strings.Split(text, "\n")
	p.TotalLines = len(lines)

	// First pass: per-line markers and length stats
	var lengths []float64
	var fence fenceState
	codeChars := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Fence delimiter lines (opening or closing) set HasCode and
		// contribute no line-shape signals. The shared fenceState helper
		// (CommonMark §4.5) recognizes backtick and tilde fences, refuses
		// fake closes (shorter runs, trailing text), and keeps an unclosed
		// fence open to the end of the document.
		if fence.update(line) {
			p.HasCode = true
			continue
		}
		if fence.active {
			codeChars += len([]rune(line))
			continue
		}

		runeLen := len([]rune(line))
		lengths = append(lengths, float64(runeLen))

		if matchHeading(line, &p.MdHeadingCounts) {
			p.MdHeadingTotal++
			continue
		}
		if NumberedSectionPattern.MatchString(line) {
			p.NumberedSectionCount++
		}
		if GermanChapterPattern.MatchString(line) {
			p.GermanChapterCount++
		}
		if EnglishChapterPattern.MatchString(line) {
			p.EnglishChapterCount++
		}
		if ChineseChapterPattern.MatchString(line) {
			p.ChineseChapterCount++
		}
		if AllCapsHeadingPattern.MatchString(line) {
			p.AllCapsShortLineCount++
		}
		if VisualSeparatorPattern.MatchString(line) {
			p.VisualSepCount++
		}
		if PageFooterPattern.MatchString(line) {
			p.RepeatedFooterCount++
		}
		if strings.HasPrefix(trimmed, "|") && strings.HasSuffix(trimmed, "|") {
			p.HasTables = true
		}
	}

	if len(lengths) > 0 {
		var sum float64
		for _, l := range lengths {
			sum += l
		}
		p.AvgLineLen = sum / float64(len(lengths))
		var variance float64
		for _, l := range lengths {
			d := l - p.AvgLineLen
			variance += d * d
		}
		variance /= float64(len(lengths))
		p.StdLineLen = math.Sqrt(variance)
	}

	if p.TotalChars > 0 {
		p.CodeRatio = float64(codeChars) / float64(p.TotalChars)
	}

	p.BlankParagraphBreaks = strings.Count(text, "\n\n\n")

	// Sample a slice of the document for language detection — avoids paying
	// O(N) scan cost on huge inputs while still giving a stable signal.
	sample := text
	if len(sample) > 4096 {
		sample = sample[:4096]
	}
	lang := DetectLanguage(sample)
	p.DetectedLangs = []string{lang}
	if lang == LangMixed {
		// Provide all three for downstream pattern selection.
		p.DetectedLangs = []string{LangEnglish, LangGerman, LangChinese}
	}

	return p
}

// matchHeading checks whether line is an ATX heading and increments the
// appropriate level counter when so. Returns true on match.
func matchHeading(line string, counts *map[int]int) bool {
	m := MarkdownHeadingPattern.FindStringSubmatch(line)
	if m == nil {
		return false
	}
	level := len(m[1])
	if level < 1 || level > 6 {
		return false
	}
	(*counts)[level]++
	return true
}

// StrategyTier identifies which chunking implementation should run.
type StrategyTier string

const (
	TierHeading   StrategyTier = "heading"
	TierHeuristic StrategyTier = "heuristic"
	TierLegacy    StrategyTier = "legacy"
)

// SelectStrategy returns the ordered tier chain to attempt for this document.
// The first tier is the primary choice; subsequent tiers are fallbacks if
// validation rejects the previous output. The "legacy" tier is appended as
// a final safety net so callers always receive at least one chunk-set.
func SelectStrategy(p *DocProfile) []StrategyTier {
	if p == nil {
		return []StrategyTier{TierLegacy}
	}
	var chain []StrategyTier

	// Tier 1 candidate: Markdown heading-aware
	if p.MdHeadingTotal >= 3 && p.HeadingDensity() > 0.005 && p.DominantHeadingLevel() > 0 {
		chain = append(chain, TierHeading)
	}

	// Tier 2 candidate: heuristic boundary detection
	if p.HeuristicMarkerTotal() >= 5 || p.FormFeedCount > 0 ||
		p.GermanChapterCount+p.EnglishChapterCount+p.ChineseChapterCount > 0 {
		chain = append(chain, TierHeuristic)
	}

	// Legacy is the ultimate fallback: always returns chunks even when
	// validation fails, so callers never get an empty result.
	chain = append(chain, TierLegacy)
	return chain
}
