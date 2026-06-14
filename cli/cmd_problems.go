package cli

import (
	"github.com/spf13/cobra"
)

func (a *App) problemsCmd() *cobra.Command {
	var (
		difficulty string
		category   string
		list       string
	)
	cmd := &cobra.Command{
		Use:   "problems",
		Short: "List NeetCode problems",
		Long: `List problems from neetcode.io.

Use --difficulty to filter by Easy/Medium/Hard, --category to filter by topic
pattern (e.g. "Arrays & Hashing"), and --list to restrict to a curated list
(neetcode150, blind75, neetcode250).`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			n := a.effectiveLimit(20)
			a.progressf("fetching problems...")
			probs, err := a.client.Problems(cmd.Context(), 0)
			if err != nil {
				return mapFetchErr(err)
			}
			probs = filterProblems(probs, difficulty, category, list)
			if n > 0 && n < len(probs) {
				probs = probs[:n]
			}
			return a.renderRecords(probs, len(probs))
		},
	}
	cmd.Flags().StringVarP(&difficulty, "difficulty", "d", "",
		"filter by difficulty: Easy, Medium, Hard")
	cmd.Flags().StringVarP(&category, "category", "c", "",
		"filter by category/pattern (case-insensitive substring)")
	cmd.Flags().StringVarP(&list, "list", "l", "",
		"restrict to curated list: neetcode150, blind75, neetcode250")
	return cmd
}
