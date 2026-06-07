package projects

import "testing"

func TestToggleChecklist(t *testing.T) {
	body := "Intro line\n\n- [ ] write tests\n- [x] design API\n* [ ] ship it\n"

	t.Run("check unchecked", func(t *testing.T) {
		out, matched, err := ToggleChecklist(body, "write tests", true)
		if err != nil {
			t.Fatal(err)
		}
		if matched != "write tests" {
			t.Errorf("matched = %q", matched)
		}
		if want := "- [x] write tests"; !contains(out, want) {
			t.Errorf("expected %q in:\n%s", want, out)
		}
	})

	t.Run("uncheck checked, alt bullet", func(t *testing.T) {
		out, _, err := ToggleChecklist(body, "ship it", true)
		if err != nil {
			t.Fatal(err)
		}
		if want := "* [x] ship it"; !contains(out, want) {
			t.Errorf("expected %q in:\n%s", want, out)
		}
	})

	t.Run("ambiguous match errors", func(t *testing.T) {
		_, _, err := ToggleChecklist("- [ ] test a\n- [ ] test b\n", "test", true)
		if err == nil {
			t.Fatal("expected ambiguity error")
		}
	})

	t.Run("no match errors", func(t *testing.T) {
		if _, _, err := ToggleChecklist(body, "nonexistent", true); err == nil {
			t.Fatal("expected no-match error")
		}
	})

	t.Run("case-insensitive", func(t *testing.T) {
		if _, _, err := ToggleChecklist(body, "WRITE TESTS", true); err != nil {
			t.Errorf("expected case-insensitive match: %v", err)
		}
	})
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
