package career

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var identityNumberPattern = regexp.MustCompile(`(?i)\d{15,}[\dX]?`)
var identityPassportPattern = regexp.MustCompile(`(?i)[A-Z]{1,2}\d{6,10}`)

type PurposeModelInput struct {
	Purpose string      `json:"purpose"`
	Facts   []ModelFact `json:"facts"`
}

// ModelFact deliberately excludes source and confirmation metadata. Those fields
// are useful to the application but are not needed by model consumers.
type ModelFact struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func (input PurposeModelInput) String() string {
	encoded, _ := json.Marshal(input)
	return string(encoded)
}

func (o *Office) BuildModelInput(ctx context.Context, purpose string) (PurposeModelInput, error) {
	purpose = strings.TrimSpace(purpose)
	if purpose != "profile_summary" && purpose != "qualification_evaluation" && purpose != "material_generation" {
		return PurposeModelInput{}, fmt.Errorf("unsupported model purpose: %w", ErrInvalidRequest)
	}
	if _, err := getScope(ctx); err != nil {
		return PurposeModelInput{}, err
	}
	view, err := o.Open(ctx)
	if err != nil {
		return PurposeModelInput{}, err
	}
	allowed := map[string]bool{"education": true, "experience": true, "project": true, "skill": true, "achievement": true, "certificate": true, "preference": true}
	legacy := map[string]string{"degree": "education", "graduation_year": "education", "city": "preference", "target_role": "preference", "internship_months": "experience"}
	if purpose == "qualification_evaluation" {
		allowed = map[string]bool{"education": true, "experience": true, "skill": true, "preference": true}
	}
	input := PurposeModelInput{Purpose: purpose, Facts: []ModelFact{}}
	for _, fact := range view.Facts {
		category := strings.SplitN(fact.Key, ".", 2)[0]
		if legacyCategory, ok := legacy[fact.Key]; ok {
			category = legacyCategory
		}
		if strings.HasPrefix(fact.Key, "identity.") || !allowed[category] {
			continue
		}
		key := redactModelString(fact.Key)
		value := redactModelString(fact.Value)
		input.Facts = append(input.Facts, ModelFact{Key: key, Value: value})
	}
	return input, nil
}

func redactModelString(value string) string {
	value = identityNumberPattern.ReplaceAllString(value, "[REDACTED]")
	return identityPassportPattern.ReplaceAllString(value, "[REDACTED]")
}
