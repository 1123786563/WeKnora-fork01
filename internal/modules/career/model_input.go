package career

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var identityNumberPattern = regexp.MustCompile(`(?i)\d(?:[ -]?\d){14,17}[\dX]?`)
var identityPassportPattern = regexp.MustCompile(`(?i)[A-Z]{1,2}[ -]?\d(?:[ -]?\d){5,9}`)
var extractedModelKeyPattern = regexp.MustCompile(`^(education|experience|project|skill|achievement|certificate)\.item_[0-9a-f]{12}\.details$`)

var safeModelKeys = map[string]string{
	"degree": "education", "graduation_year": "education", "city": "preference", "target_role": "preference", "internship_months": "experience",
	"毕业时间": "education", "学历": "education", "城市": "preference", "意向": "preference",
	"education.school": "education", "education.degree": "education", "education.major": "education", "education.graduation_year": "education", "education.details": "education",
	"experience.company": "experience", "experience.role": "experience", "experience.duration": "experience", "experience.details": "experience", "experience.achievement": "experience",
	"internship.company": "experience",
	"project.name":       "project", "project.role": "project", "project.details": "project",
	"skill.name": "skill", "skill.details": "skill", "skill.go": "skill",
	"achievement.details": "achievement", "achievement.latency": "achievement",
	"certificate.name": "certificate", "certificate.details": "certificate",
	"preference.city": "preference", "preference.target_role": "preference", "preference.details": "preference",
}

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
	if purpose == "qualification_evaluation" {
		allowed = map[string]bool{"education": true, "experience": true, "skill": true, "preference": true}
	}
	input := PurposeModelInput{Purpose: purpose, Facts: []ModelFact{}}
	for _, fact := range view.Facts {
		category, safe := safeModelKeys[fact.Key]
		if !safe && extractedModelKeyPattern.MatchString(fact.Key) {
			category = strings.SplitN(fact.Key, ".", 2)[0]
			safe = true
		}
		if !safe || !allowed[category] {
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
