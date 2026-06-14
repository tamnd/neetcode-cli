package cli

import (
	"github.com/spf13/cobra"
)

func (a *App) searchCmd() *cobra.Command {
	var (
		difficulty string
		category   string
		list       string
	)
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search NeetCode problems by title or category",
		Long: `Search NeetCode problems whose title or category contains <query>.

The search is case-insensitive. Use --difficulty, --category, and --list to
narrow results further after the text match.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			n := a.effectiveLimit(20)
			a.progressf("searching for %q...", args[0])
			probs, err := a.client.Search(cmd.Context(), args[0], 0)
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
