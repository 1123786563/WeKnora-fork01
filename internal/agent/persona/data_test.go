package persona

import "testing"

func TestAllProfilesSixteenTypes(t *testing.T) {
	all := AllProfiles()
	if len(all) != 16 {
		t.Fatalf("want 16 profiles, got %d", len(all))
	}
	seen := map[string]bool{}
	for _, p := range all {
		if seen[p.Code] {
			t.Errorf("duplicate code %s", p.Code)
		}
		seen[p.Code] = true
		for _, a := range []Axis{p.Dimensions.EI, p.Dimensions.SN, p.Dimensions.TF, p.Dimensions.JP} {
			if a.Percent < 50 || a.Percent > 85 {
				t.Errorf("%s axis %% out of 50-85: %+v", p.Code, a)
			}
		}
		if p.Behavior.AnswerStyle == "" || p.SummaryEn == "" || p.Color == "" {
			t.Errorf("%s missing core fields", p.Code)
		}
	}
	if !seen["INTJ"] || !seen["ESFP"] {
		t.Error("canonical corner types missing")
	}
}

func TestQuestionsTwentyEightBalanced(t *testing.T) {
	qs := Questions()
	if len(qs) != 28 {
		t.Fatalf("want 28 questions, got %d", len(qs))
	}
	perDim := map[string]int{}
	for _, q := range qs {
		perDim[q.Dimension]++
		if q.QuestionZh == "" || q.QuestionEn == "" || q.OptionAZh == "" || q.OptionBEn == "" {
			t.Errorf("question %d missing bilingual fields", q.ID)
		}
	}
	for _, dim := range []string{"EI", "SN", "TF", "JP"} {
		if perDim[dim] != 7 {
			t.Errorf("dimension %s: want 7 questions, got %d", dim, perDim[dim])
		}
	}
}

func TestProfileLookup(t *testing.T) {
	if _, ok := Profile("INTJ"); !ok {
		t.Error("INTJ should resolve")
	}
	if _, ok := Profile("intj"); ok {
		t.Error("lookup must be case-sensitive (uppercase codes only)")
	}
	if _, ok := Profile("XXYY"); ok {
		t.Error("unknown code should not resolve")
	}
}
