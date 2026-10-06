package persona

import (
	"fmt"
	"strings"
)

// AxisResult is the scored outcome of one MBTI axis.
type AxisResult struct {
	Dominant string `json:"dominant"`
	Percent  int    `json:"percent"`
}

// ScoreResult is the outcome of the 28-question test.
type ScoreResult struct {
	Code string     `json:"code"`
	EI   AxisResult `json:"ei"`
	SN   AxisResult `json:"sn"`
	TF   AxisResult `json:"tf"`
	JP   AxisResult `json:"jp"`
}

var axes = []struct {
	key        string
	first, sec string
	set        func(*ScoreResult, AxisResult)
	get        func(ScoreResult) AxisResult
}{
	{"EI", "E", "I", func(r *ScoreResult, a AxisResult) { r.EI = a }, func(r ScoreResult) AxisResult { return r.EI }},
	{"SN", "S", "N", func(r *ScoreResult, a AxisResult) { r.SN = a }, func(r ScoreResult) AxisResult { return r.SN }},
	{"TF", "T", "F", func(r *ScoreResult, a AxisResult) { r.TF = a }, func(r ScoreResult) AxisResult { return r.TF }},
	{"JP", "J", "P", func(r *ScoreResult, a AxisResult) { r.JP = a }, func(r ScoreResult) AxisResult { return r.JP }},
}

// ScoreAnswers scores submitted test answers, mirroring Octop _score_answers:
// ≥20 valid answers required; pct = round(50 + dominant/total*35) clamped to
// 50–85; tie takes the first pole; an unanswered axis defaults to the first
// pole at 50.
func ScoreAnswers(answers map[string]string) (ScoreResult, error) {
	counts := map[string]map[string]int{}
	for _, ax := range axes {
		counts[ax.key] = map[string]int{ax.first: 0, ax.sec: 0}
	}
	qByID := map[int]TestQuestion{}
	for _, q := range Questions() {
		qByID[q.ID] = q
	}
	valid := 0
	for id, choice := range answers {
		var qid int
		if _, err := fmt.Sscanf(id, "%d", &qid); err != nil {
			continue
		}
		q, ok := qByID[qid]
		if !ok {
			continue
		}
		switch strings.ToUpper(choice) {
		case "A":
			counts[q.Dimension][q.APole]++
			valid++
		case "B":
			counts[q.Dimension][q.BPole]++
			valid++
		}
	}
	if valid < 20 {
		return ScoreResult{}, fmt.Errorf("persona: too few answers (%d/28), at least 20 required", valid)
	}
	var res ScoreResult
	for _, ax := range axes {
		a, b := counts[ax.key][ax.first], counts[ax.key][ax.sec]
		if a+b == 0 {
			ax.set(&res, AxisResult{Dominant: ax.first, Percent: 50})
			continue
		}
		dominant, dominantCount := ax.first, a
		if b > a {
			dominant, dominantCount = ax.sec, b
		}
		pct := 50 + (dominantCount*35+(a+b)/2)/(a+b)
		if pct > 85 {
			pct = 85
		}
		ax.set(&res, AxisResult{Dominant: dominant, Percent: pct})
	}
	res.Code = res.EI.Dominant + res.SN.Dominant + res.TF.Dominant + res.JP.Dominant
	return res, nil
}
