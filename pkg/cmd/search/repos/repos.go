package repos

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/cli/cli/v2/internal/config"
	"github.com/cli/cli/v2/pkg/cmd/search/shared"
	"github.com/cli/cli/v2/pkg/cmdutil"
	"github.com/cli/cli/v2/pkg/export"
	"github.com/cli/cli/v2/pkg/iostreams"
	"github.com/cli/cli/v2/pkg/search"
	"github.com/cli/cli/v2/pkg/text"
	"github.com/cli/cli/v2/utils"
	"github.com/spf13/cobra"
)

// These regexs are not perfect and mostly used to give some input validation.
// They are more permissive than the server so we can provide early feedback
// to the user in most cases but there are some inputs that will pass the
// regexs and then be rejected by the server.
var rangeRE = regexp.MustCompile(`^(>|>=|<|<=|\*\.\.)?\d+(\.\.(\*|\d+))?$`)
var dateTime = `(\d|-|\+|:|T)+`
var dateTimeRangeRE = regexp.MustCompile(fmt.Sprintf(`^(>|>=|<|<=|\*\.\.)?%s(\.\.(\*|%s))?$`, dateTime, dateTime))

type ReposOptions struct {
	Browser      cmdutil.Browser
	Config       func() (config.Config, error)
	Exporter     cmdutil.Exporter
	GoTemplate   string
	HttpClient   func() (*http.Client, error)
	IO           *iostreams.IOStreams
	JqExpression string
	Query        search.Query
	WebMode      bool
}

func NewCmdRepos(f *cmdutil.Factory, runF func(*ReposOptions) error) *cobra.Command {
	opts := &ReposOptions{
		Browser:    f.Browser,
		Config:     f.Config,
		HttpClient: f.HttpClient,
		IO:         f.IOStreams,
		Query:      search.Query{Kind: "repositories"},
	}

	cmd := &cobra.Command{
		Use:   "repos [<query>]",
		Short: "Search repositories",
		RunE: func(c *cobra.Command, args []string) error {
			opts.Query.Keywords = args
			err := cmdutil.MutuallyExclusive("expected exactly one of `--jq`, `--template`, or `--web`",
				opts.GoTemplate != "",
				opts.JqExpression != "",
				opts.WebMode)
			if err != nil {
				return err
			}
			if opts.Query.Limit < 1 || opts.Query.Limit > 1000 {
				return cmdutil.FlagErrorf("`--limit` must be between 1 and 1000")
			}
			if runF != nil {
				return runF(opts)
			}
			return reposRun(opts)
		},
	}

	// Output flags
	cmd.Flags().StringVarP(&opts.GoTemplate, "template", "t", "", "Format JSON output using a Go template")
	cmd.Flags().StringVarP(&opts.JqExpression, "jq", "q", "", "Format JSON output using a jq `expression`")
	cmd.Flags().BoolVarP(&opts.WebMode, "web", "w", false, "Open the query in the web browser")

	// Query parameter flags
	cmd.Flags().IntVarP(&opts.Query.Limit, "limit", "L", 30, "Maximum number of repositories to fetch")
	cmdutil.StringEnumFlag(cmd, &opts.Query.Order, "order", "", "", []string{"asc", "desc"}, "Order of repositories returned, ignored unless '--sort' is specified")
	cmdutil.StringEnumFlag(cmd, &opts.Query.Sort, "sort", "", "", []string{"forks", "help-wanted-issues", "stars", "updated"}, "Sorts the repositories by stars, forks, help-wanted-issues, or updated")

	// Query qualifier flags
	cmdutil.NilBoolFlag(cmd, &opts.Query.Qualifiers.Archived, "archived", "", "Filter based on archive state")
	cmdutil.StringRegexpFlag(cmd, &opts.Query.Qualifiers.Created, "created", "", "", dateTimeRangeRE, "Filter based on created at date")
	cmdutil.StringRegexpFlag(cmd, &opts.Query.Qualifiers.Followers, "followers", "", "", rangeRE, "Filter based on number of followers")
	cmdutil.StringEnumFlag(cmd, &opts.Query.Qualifiers.Fork, "include-forks", "", "", []string{"false", "true", "only"}, "Include forks in search")
	cmdutil.StringRegexpFlag(cmd, &opts.Query.Qualifiers.Forks, "forks", "", "", rangeRE, "Filter on number of forks")
	cmdutil.StringRegexpFlag(cmd, &opts.Query.Qualifiers.GoodFirstIssues,
		"good-first-issues", "", "", rangeRE, "Filter on number of issues with the 'good first issue' label")
	cmdutil.StringRegexpFlag(cmd, &opts.Query.Qualifiers.HelpWantedIssues,
		"help-wanted-issues", "", "", rangeRE, "Filter on number of issues with the 'help wanted' label")
	cmdutil.StringSliceEnumFlag(cmd, &opts.Query.Qualifiers.In,
		"in", "", nil, []string{"name", "description", "readme"}, "Restrict search to the name, description, or README file")
	cmd.Flags().StringSliceVar(&opts.Query.Qualifiers.Language, "language", nil, "Filter based on the coding language")
	cmd.Flags().StringSliceVar(&opts.Query.Qualifiers.License, "license", nil, "Filter based on license type")
	cmdutil.NilBoolFlag(cmd, &opts.Query.Qualifiers.Mirror, "mirror", "", "Filter based on mirror state")
	cmd.Flags().StringVar(&opts.Query.Qualifiers.Org, "org", "", "Filter on organization")
	cmdutil.StringRegexpFlag(cmd, &opts.Query.Qualifiers.Pushed, "updated", "", "", dateTimeRangeRE, "Filter on last updated at date")
	cmd.Flags().StringVar(&opts.Query.Qualifiers.Repo, "repo", "", "Filter on repository name")
	cmdutil.StringRegexpFlag(cmd, &opts.Query.Qualifiers.Size, "size", "", "", rangeRE, "Filter on a size range, in kilobytes")
	cmdutil.StringRegexpFlag(cmd, &opts.Query.Qualifiers.Stars, "stars", "", "", rangeRE, "Filter on number of stars")
	cmd.Flags().StringSliceVar(&opts.Query.Qualifiers.Topic, "topic", nil, "Filter on topic")
	cmdutil.StringRegexpFlag(cmd, &opts.Query.Qualifiers.Topics, "number-topics", "", "", rangeRE, "Filter on number of topics")
	cmd.Flags().StringVar(&opts.Query.Qualifiers.User, "user", "", "Filter based on user")
	cmdutil.StringEnumFlag(cmd, &opts.Query.Qualifiers.Is, "visibility", "", "", []string{"public", "private"}, "Filter based on visibility")

	return cmd
}

func reposRun(opts *ReposOptions) error {
	io := opts.IO
	cfg, err := opts.Config()
	if err != nil {
		return err
	}
	host, err := cfg.DefaultHost()
	if err != nil {
		return err
	}
	client, err := opts.HttpClient()
	if err != nil {
		return err
	}
	searcher := shared.NewSearcher(client, host)
	if opts.WebMode {
		url := searcher.URL(opts.Query)
		if io.IsStdoutTTY() {
			fmt.Fprintf(io.ErrOut, "Opening %s in your browser.\n", utils.DisplayURL(url))
		}
		return opts.Browser.Browse(url)
	}
	opts.IO.StartProgressIndicator()
	result, err := searcher.Repositories(opts.Query)
	opts.IO.StopProgressIndicator()
	if err != nil {
		return err
	}
	if err := opts.IO.StartPager(); err == nil {
		defer opts.IO.StopPager()
	} else {
		fmt.Fprintf(opts.IO.ErrOut, "failed to start pager: %v\n", err)
	}
	if opts.JqExpression != "" {
		j, err := json.Marshal(result.Items)
		if err != nil {
			return err
		}
		err = export.FilterJSON(io.Out, bytes.NewReader(j), opts.JqExpression)
		if err != nil {
			return err
		}
	} else if opts.GoTemplate != "" {
		t := export.NewTemplate(opts.IO, opts.GoTemplate)
		j, err := json.Marshal(result.Items)
		if err != nil {
			return err
		}
		err = t.Execute(bytes.NewReader(j))
		if err != nil {
			return err
		}
	} else {
		err := displayResults(opts.IO, result)
		if err != nil {
			return err
		}
	}
	return nil
}

func displayResults(io *iostreams.IOStreams, results search.RepositoriesResult) error {
	cs := io.ColorScheme()
	tp := utils.NewTablePrinter(io)
	for _, repo := range results.Items {
		var tags []string
		if repo.Private {
			tags = append(tags, "private")
		} else {
			tags = append(tags, "public")
		}
		if repo.Fork {
			tags = append(tags, "fork")
		}
		if repo.Archived {
			tags = append(tags, "archived")
		}
		info := strings.Join(tags, ", ")
		infoColor := cs.Gray
		if repo.Private {
			infoColor = cs.Yellow
		}
		tp.AddField(repo.FullName, nil, cs.Bold)
		description := repo.Description
		tp.AddField(text.ReplaceExcessiveWhitespace(description), nil, nil)
		tp.AddField(info, nil, infoColor)
		if tp.IsTTY() {
			tp.AddField(utils.FuzzyAgoAbbr(time.Now(), repo.UpdatedAt), nil, cs.Gray)
		} else {
			tp.AddField(repo.UpdatedAt.Format(time.RFC3339), nil, nil)
		}
		tp.EndRow()
	}
	if io.IsStdoutTTY() {
		header := "No repositories matched your search\n"
		if len(results.Items) > 0 {
			header = fmt.Sprintf("Showing %d of %d repositories\n\n", len(results.Items), results.Total)
		}
		fmt.Fprintf(io.Out, "\n%s", header)
	}
	return tp.Render()
}
