package persona

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
)

//go:embed data/profiles.json
var profilesJSON []byte

//go:embed data/questions.json
var questionsJSON []byte

// ErrProfileNotFound is returned by lookups on unknown MBTI codes.
var ErrProfileNotFound = errors.New("persona: mbti profile not found")

var (
	loadOnce sync.Once
	loaded   []MBTIProfile
	byCode   map[string]MBTIProfile
	loadedQs []TestQuestion
	loadErr  error
)

func load() {
	loadOnce.Do(func() {
		if err := json.Unmarshal(profilesJSON, &loaded); err != nil {
			loadErr = fmt.Errorf("persona: decode profiles: %w", err)
			return
		}
		byCode = make(map[string]MBTIProfile, len(loaded))
		for _, p := range loaded {
			byCode[p.Code] = p
		}
		if err := json.Unmarshal(questionsJSON, &loadedQs); err != nil {
			loadErr = fmt.Errorf("persona: decode questions: %w", err)
		}
	})
}

// Profile returns the profile for an uppercase MBTI code.
func Profile(code string) (MBTIProfile, bool) {
	load()
	if loadErr != nil {
		return MBTIProfile{}, false
	}
	p, ok := byCode[code]
	return p, ok
}

// AllProfiles returns all 16 profiles in canonical NT/NF/SJ/SP order (the
// import preserves Octop's registration order).
func AllProfiles() []MBTIProfile {
	load()
	if loadErr != nil {
		return nil
	}
	return append([]MBTIProfile(nil), loaded...)
}

// Questions returns the 28 test questions.
func Questions() []TestQuestion {
	load()
	if loadErr != nil {
		return nil
	}
	return append([]TestQuestion(nil), loadedQs...)
}
