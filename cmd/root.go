// Package cmd implements the gh-projects extension's command-line interface.
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/NSExceptional/gh-projects/internal/projects"
	"github.com/spf13/cobra"
)

var (
	flagOwner string
	flagJSON  bool
)

var rootCmd = &cobra.Command{
	Use:   "projects",
	Short: "Concise, scriptable access to GitHub Projects (v2)",
	Long: `gh-projects is a gh extension for working with GitHub Projects (v2) from the
command line and from agents.

It resolves the opaque node IDs the GraphQL API requires internally, so you
address projects by number, items by issue number, and columns/fields by name.

Examples:
  gh projects list
  gh projects ready 4
  gh projects board 4
  gh projects move 4 12 "Needs review"
  gh projects create 4 --repo owner/name --title "Add caching" --label enhancement
  gh projects check 4 12 "write tests"`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&flagOwner, "owner", "", "project owner login (default: authenticated user)")
	rootCmd.PersistentFlags().BoolVar(&flagJSON, "json", false, "emit JSON instead of text")
}

// openProject resolves the owner (defaulting to the authenticated user) and
// fetches the project by number along with its fields.
func openProject(number int) (*projects.Client, *projects.Project, error) {
	c, err := projects.New()
	if err != nil {
		return nil, nil, err
	}
	owner := flagOwner
	if owner == "" {
		owner, err = c.Viewer()
		if err != nil {
			return nil, nil, fmt.Errorf("resolving authenticated user: %w", err)
		}
	}
	p, err := c.Project(owner, number)
	if err != nil {
		return nil, nil, err
	}
	return c, p, nil
}

// parseNumber parses a project/issue number, tolerating a leading '#'.
func parseNumber(s string) (int, error) {
	n, err := strconv.Atoi(strings.TrimPrefix(s, "#"))
	if err != nil {
		return 0, fmt.Errorf("invalid number %q", s)
	}
	return n, nil
}

// printJSON writes v as indented JSON to stdout.
func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
