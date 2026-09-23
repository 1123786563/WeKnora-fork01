package career

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
)

var explicitResumeField = regexp.MustCompile(`^\s*(?:[-*•]\s*)?([^:：]{1,32})\s*[:：]\s*(.+?)\s*$`)
var explicitYear = regexp.MustCompile(`(?:19|20)\d{2}`)
var explicitNumber = regexp.MustCompile(`\d+(?:\.\d+)?\s*(?:%|％|倍|人|项|个|天|月|年|万元|万|ms|秒|次)?`)

var resumeLabels = map[string]string{
	"education": "education", "education background": "education", "education experience": "education", "graduation year": "education", "graduation date": "education", "学历": "education", "教育": "education", "教育背景": "education", "教育经历": "education", "毕业年份": "education", "毕业时间": "education", "毕业届别": "education",
	"experience": "experience", "work experience": "experience", "employment": "experience", "工作经历": "experience", "工作经验": "experience", "实习经历": "experience",
	"project": "project", "project experience": "project", "项目": "project", "项目经历": "project",
	"skill": "skill", "skills": "skill", "技能": "skill", "专业技能": "skill",
	"achievement": "achievement", "result": "achievement", "quantified result": "achievement", "成果": "achievement", "量化成果": "achievement", "工作成果": "achievement",
	"certificate": "certificate", "certification": "certificate", "证书": "certificate", "资格证书": "certificate",
}

// ExtractResumeFields accepts only explicit, single-line labeled statements.
// It copies source text verbatim into Evidence and never infers a missing value.
func ExtractResumeFields(text string) ([]ExtractedField, []string, []string) {
	fields := make([]ExtractedField, 0)
	seen := map[string]map[string]bool{}
	categoryCounts := map[string]int{}
	acceptedCounts := map[string]int{}
	missing := make([]string, 0)
	flags := make([]string, 0)
	graduationYearFound := false
	educationValues := make([]string, 0)
	for _, rawLine := range strings.Split(text, "\n") {
		line := strings.TrimSpace(strings.TrimRight(rawLine, "\r"))
		match := explicitResumeField.FindStringSubmatch(line)
		if len(match) != 3 {
			continue
		}
		label := strings.ToLower(strings.TrimSpace(match[1]))
		category, ok := resumeLabels[label]
		if !ok {
			continue
		}
		value := strings.TrimSpace(match[2])
		if value == "" || isExplicitUnknown(value) {
			continue
		}
		if seen[category] == nil {
			seen[category] = map[string]bool{}
		}
		categoryCounts[category]++
		if seen[category][value] {
			continue
		}
		seen[category][value] = true
		if category == "achievement" && !explicitNumber.MatchString(value) {
			flags = appendUnique(flags, "achievement_without_explicit_number")
			continue
		}
		sum := sha256.Sum256([]byte(strings.ToLower(strings.Join(strings.Fields(line), " "))))
		itemID := hex.EncodeToString(sum[:6])
		key := category + ".item_" + itemID + ".details"
		if category == "education" {
			if isGraduationLabel(label) {
				key = "education.graduation_year"
				graduationYearFound = true
			} else {
				educationValues = append(educationValues, value)
			}
		}
		fields = append(fields, ExtractedField{Key: key, Value: value, Evidence: line})
		acceptedCounts[category]++
	}
	for _, category := range []string{"education", "experience", "project", "skill", "achievement", "certificate"} {
		if categoryCounts[category] == 0 || category == "achievement" && acceptedCounts[category] == 0 {
			missing = append(missing, category)
		}
	}
	if categoryCounts["education"] > 0 && !graduationYearFound && !explicitYear.MatchString(strings.Join(educationValues, " ")) {
		missing = append(missing, "education.graduation_year")
	}
	if categoryCounts["experience"] > 1 {
		flags = appendUnique(flags, "multiple_experience_claims_require_review")
	}
	return fields, missing, flags
}

func isGraduationLabel(label string) bool {
	switch label {
	case "graduation year", "graduation date", "毕业年份", "毕业时间", "毕业届别":
		return true
	default:
		return false
	}
}

func isExplicitUnknown(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "unknown", "n/a", "na", "not provided", "not available", "缺失", "待定", "不详", "未提供", "未填写", "未写":
		return true
	default:
		return false
	}
}

func appendUnique(values []string, value string) []string {
	for _, present := range values {
		if present == value {
			return values
		}
	}
	return append(values, value)
}
