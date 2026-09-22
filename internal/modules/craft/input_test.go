package craft

import "testing"

// TestInputPathCannotEscape pins the staging-name rule: a canonical single
// path element is accepted; anything that could escape the inputs directory —
// traversal, absolute paths, separators, NUL — is rejected before delegation.
func TestInputPathCannotEscape(t *testing.T) {
	for _, p := range []string{"../secret", "/etc/passwd", `a\b`, "", "a/b"} {
		if ValidateInputName(p) == nil {
			t.Fatalf("accepted %q", p)
		}
	}
	if err := ValidateInputName("sales.csv"); err != nil {
		t.Fatal(err)
	}
}

func TestValidateInputNameRejectsDotEntries(t *testing.T) {
	for _, p := range []string{".", ".."} {
		if ValidateInputName(p) == nil {
			t.Fatalf("accepted %q", p)
		}
	}
	if err := ValidateInputName(".hidden"); err != nil {
		t.Fatalf("canonical dotfile must stage: %v", err)
	}
}
