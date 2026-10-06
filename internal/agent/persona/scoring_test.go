package persona

import (
	"fmt"
	"testing"
)

func allAnswers(pick func(q TestQuestion) string) map[string]string {
	out := map[string]string{}
	for _, q := range Questions() {
		out[fmt.Sprint(q.ID)] = pick(q)
	}
	return out
}

func TestScoreAllAIsFirstPoles(t *testing.T) {
	res, err := ScoreAnswers(allAnswers(func(q TestQuestion) string { return "A" }))
	if err != nil {
		t.Fatal(err)
	}
	// All-A: every axis dominant is the a_pole of its questions.
	if res.Code != "ENTJ" && res.Code != "ESTJ" {
		// derive instead of hardcoding: first poles per axis in question data
		t.Logf("code=%s", res.Code)
	}
	// a_pole per axis in the imported data: EI=E, SN=S, TF=T, JP=J.
	if res.EI.Dominant != "E" || res.SN.Dominant != "S" || res.TF.Dominant != "T" || res.JP.Dominant != "J" {
		t.Fatalf("all-A must select first poles, got %s", res.Code)
	}
	if res.EI.Percent != 85 { // 7/7 dominant: 50+35=85
		t.Errorf("EI percent = %d, want 85", res.EI.Percent)
	}
}

func TestScoreTieTakesFirstPole(t *testing.T) {
	// 4×A + 3×B on every axis: pct = round(50 + 4/7*35) = 70, dominant = first pole.
	answers := map[string]string{}
	i := 0
	for _, q := range Questions() {
		choice := "A"
		if i%7 >= 4 {
			choice = "B"
		}
		answers[fmt.Sprint(q.ID)] = choice
		i++
	}
	res, err := ScoreAnswers(answers)
	if err != nil {
		t.Fatal(err)
	}
	if res.EI.Dominant != "E" || res.EI.Percent != 70 {
		t.Errorf("EI = %+v, want E/70", res.EI)
	}
}

func TestScoreTooFewAnswers(t *testing.T) {
	answers := map[string]string{"1": "A", "2": "B"}
	if _, err := ScoreAnswers(answers); err == nil {
		t.Fatal("want error for 2/28 answers")
	}
}

func TestScoreIgnoresUnknownAndInvalid(t *testing.T) {
	answers := allAnswers(func(q TestQuestion) string { return "a" }) // lowercase ok
	answers["999"] = "A"                                              // unknown id
	answers["1"] = "X"                                                // invalid choice ignored
	if _, err := ScoreAnswers(answers); err != nil {
		t.Fatal(err)
	}
}

func TestScoreUnansweredAxisDefaultsFirstPole(t *testing.T) {
	// Answer 21 questions, all EI questions blank.
	answers := map[string]string{}
	count := 0
	for _, q := range Questions() {
		if q.Dimension == "EI" || count >= 21 {
			continue
		}
		answers[fmt.Sprint(q.ID)] = "B"
		count++
	}
	res, err := ScoreAnswers(answers)
	if err != nil {
		t.Fatal(err)
	}
	if res.EI.Dominant != "E" || res.EI.Percent != 50 {
		t.Errorf("unanswered EI = %+v, want E/50", res.EI)
	}
}
