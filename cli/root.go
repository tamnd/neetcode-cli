// Package cli builds the nc command tree on top of the neetcode library.
package cli

import (
	"fmt"
	"os"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
	"github.com/tamnd/neetcode-cli/neetcode"
)

// Build metadata, set via -ldflags at release time.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// App holds shared state threaded through every command.
type App struct {
	client *neetcode.Client
	cfg    neetcode.Config

	output   string
	fields   []string
	noHeader bool
	template string
	limit    int
	quiet    bool
}

// Root builds the root command and its subtree.
func Root() *cobra.Command {
	app := &App{cfg: neetcode.DefaultConfig()}

	root := &cobra.Command{
		Use:   "nc",
		Short: "Browse NeetCode curated LeetCode problems",
		Long: `nc reads problem data from neetcode.io — a curated set of LeetCode problems
with video solutions organized by pattern.

Data is fetched from the public NeetCode site; no API key is required.
nc returns records as table, JSON, JSONL, CSV, TSV, or URLs.

nc is an independent tool and is not affiliated with NeetCode or LeetCode.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			return app.setup()
		},
	}

	pf := root.PersistentFlags()
	pf.StringVarP(&app.output, "output", "o", "auto",
		"output format: table|json|jsonl|csv|tsv|url|raw (auto=table on TTY, jsonl piped)")
	pf.StringSliceVar(&app.fields, "fields", nil, "comma-separated columns to include")
	pf.BoolVar(&app.noHeader, "no-header", false, "omit the header row in table/csv/tsv")
	pf.StringVar(&app.template, "template", "", "Go text/template applied per record")
	pf.IntVarP(&app.limit, "limit", "n", 0, "limit number of records (0 = command default)")
	pf.BoolVarP(&app.quiet, "quiet", "q", false, "suppress progress messages on stderr")

	pf.StringVar(&app.cfg.BaseURL, "base-url", app.cfg.BaseURL, "NeetCode base URL")
	pf.StringVar(&app.cfg.UserAgent, "user-agent", app.cfg.UserAgent,
		"User-Agent sent with each request")
	pf.DurationVar(&app.cfg.Rate, "delay", app.cfg.Rate, "minimum spacing between requests")
	pf.DurationVar(&app.cfg.Timeout, "timeout", app.cfg.Timeout, "per-request timeout")
	pf.IntVar(&app.cfg.Retries, "retries", app.cfg.Retries, "retry attempts on 429/5xx")

	root.AddCommand(
		app.problemsCmd(),
		app.searchCmd(),
		newVersionCmd(),
	)
	return root
}

func (a *App) setup() error {
	if a.output == "" || a.output == "auto" {
		if isatty.IsTerminal(os.Stdout.Fd()) {
			a.output = string(FormatTable)
		} else {
			a.output = string(FormatJSONL)
		}
	}
	if !Format(a.output).Valid() {
		return codeError(exitUsage, fmt.Errorf("unknown output format %q", a.output))
	}
	a.client = neetcode.NewClient(a.cfg)
	return nil
}

func (a *App) renderRecords(records any, n int) error {
	r := newRenderer(os.Stdout, Format(a.output), a.fields, a.noHeader, a.template)
	if err := r.render(records); err != nil {
		return err
	}
	if n == 0 {
		return codeError(exitNoData, nil)
	}
	return nil
}

func (a *App) progressf(format string, args ...any) {
	if a.quiet {
		return
	}
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

func (a *App) effectiveLimit(def int) int {
	if a.limit > 0 {
		return a.limit
	}
	return def
}
