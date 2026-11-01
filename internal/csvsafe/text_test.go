package csvsafe

import "testing"

func TestTextFormulaPrefixesAndOrdinaryMetadata(t *testing.T) {
	for _, value := range []string{"=1+1", "+SUM(1)", "-1+1", "@SUM(1)", "  =1", "\t=1", "\n=1", "\r=1"} {
		if got := Text(value); got != "'"+value {
			t.Errorf("unsafe text %q became %q", value, got)
		}
	}
	for _, value := range []string{"", "project-name", "openai", "42", " leading space", "a\nb"} {
		if got := Text(value); got != value {
			t.Errorf("ordinary metadata changed: %q became %q", value, got)
		}
	}
}
