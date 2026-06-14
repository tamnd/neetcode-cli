package cli

import (
	"strings"

	"github.com/tamnd/neetcode-cli/neetcode"
)

// filterProblems applies optional server-side filters on a problem slice.
// difficulty is case-insensitive prefix match ("easy", "medium", "hard").
// category is a case-insensitive substring match on the Category field.
// list restricts to one of: neetcode150, blind75, neetcode250.
func filterProblems(probs []neetcode.Problem, difficulty, category, list string) []neetcode.Problem {
	if difficulty == "" && category == "" && list == "" {
		return probs
	}
	out := probs[:0:0]
	diff := strings.ToLower(difficulty)
	cat := strings.ToLower(category)
	lst := strings.ToLower(list)
	for _, p := range probs {
		if diff != "" && strings.ToLower(p.Difficulty) != diff {
			continue
		}
		if cat != "" && !strings.Contains(strings.ToLower(p.Category), cat) {
			continue
		}
		if lst != "" {
			switch lst {
			case "neetcode150":
				if !p.IsNeetcode150 {
					continue
				}
			case "blind75":
				if !p.IsBlind75 {
					continue
				}
			case "neetcode250":
				if !p.IsNeetcode250 {
					continue
				}
			}
		}
		out = append(out, p)
	}
	return out
}
