package projects

import (
	"fmt"
	"regexp"
	"strings"
)

// checklistLine matches a GitHub task-list item, capturing the indent/bullet
// prefix, the checkbox state, and the trailing text.
var checklistLine = regexp.MustCompile(`^(\s*[-*+]\s+)\[([ xX])\](\s+.*)$`)

// ToggleChecklist sets the checkbox of the single task-list item whose text
// contains substr (case-insensitive) to checked. It returns the rewritten body
// and the matched item text. It errors if zero or more than one item matches,
// so an agent never silently toggles the wrong box.
func ToggleChecklist(body, substr string, checked bool) (newBody, matched string, err error) {
	lines := strings.Split(body, "\n")
	var hits []int
	for i, line := range lines {
		m := checklistLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		if strings.Contains(strings.ToLower(m[3]), strings.ToLower(substr)) {
			hits = append(hits, i)
		}
	}
	switch len(hits) {
	case 0:
		return "", "", fmt.Errorf("no task-list item matching %q", substr)
	case 1:
		// ok
	default:
		var texts []string
		for _, i := range hits {
			texts = append(texts, strings.TrimSpace(checklistLine.FindStringSubmatch(lines[i])[3]))
		}
		return "", "", fmt.Errorf("%d task-list items match %q; be more specific:\n  - %s",
			len(hits), substr, strings.Join(texts, "\n  - "))
	}

	i := hits[0]
	m := checklistLine.FindStringSubmatch(lines[i])
	box := " "
	if checked {
		box = "x"
	}
	lines[i] = fmt.Sprintf("%s[%s]%s", m[1], box, m[3])
	return strings.Join(lines, "\n"), strings.TrimSpace(m[3]), nil
}
