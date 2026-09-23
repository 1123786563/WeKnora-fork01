package career

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var identityNumberPattern = regexp.MustCompile(`(?i)\b(?:[A-Z]{1,2}\d{6,10}|\d{15,19}[\dX]?)\b`)

type PurposeModelInput struct {
	Purpose string `json:"purpose"`
	Facts   []Fact `json:"facts"`
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
	input := PurposeModelInput{Purpose: purpose, Facts: []Fact{}}
	for _, fact := range view.Facts {
		category := strings.SplitN(fact.Key, ".", 2)[0]
		if legacyCategory, ok := legacy[fact.Key]; ok {
			category = legacyCategory
		}
		if strings.HasPrefix(fact.Key, "identity.") || !allowed[category] {
			continue
		}
		fact.Value = identityNumberPattern.ReplaceAllString(fact.Value, "[REDACTED]")
		input.Facts = append(input.Facts, fact)
	}
	return input, nil
}
