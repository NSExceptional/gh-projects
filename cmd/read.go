package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/NSExceptional/gh-projects/internal/projects"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(listCmd, viewCmd, boardCmd, itemsCmd, readyCmd, fieldsCmd, linksCmd)
	itemsCmd.Flags().String("status", "", "only items in this Status column")
}

func newTab() *tabwriter.Writer {
	return tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List projects owned by the owner",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := projects.New()
		if err != nil {
			return err
		}
		owner := flagOwner
		if owner == "" {
			if owner, err = c.Viewer(); err != nil {
				return err
			}
		}
		ps, err := c.List(owner)
		if err != nil {
			return err
		}
		if flagJSON {
			return printJSON(ps)
		}
		tw := newTab()
		fmt.Fprintln(tw, "#\tTITLE\tITEMS\tLINKED REPOS")
		for _, p := range ps {
			state := ""
			if p.Closed {
				state = " (closed)"
			}
			fmt.Fprintf(tw, "%d\t%s%s\t%d\t%s\n", p.Number, p.Title, state, p.ItemCount, strings.Join(p.Repos, ", "))
		}
		return tw.Flush()
	},
}

var viewCmd = &cobra.Command{
	Use:   "view <project>",
	Short: "Show a project's details, columns, and counts",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		n, err := parseNumber(args[0])
		if err != nil {
			return err
		}
		_, p, err := openProject(n)
		if err != nil {
			return err
		}
		items, err := p.Items()
		if err != nil {
			return err
		}
		repos, err := p.LinkedRepos()
		if err != nil {
			return err
		}
		if flagJSON {
			return printJSON(map[string]any{
				"number": p.Number, "title": p.Title, "description": p.Description,
				"url": p.URL, "closed": p.Closed, "itemCount": len(items),
				"linkedRepos": repos, "fields": p.Fields,
			})
		}
		fmt.Printf("#%d  %s\n", p.Number, p.Title)
		if p.Description != "" {
			fmt.Printf("%s\n", p.Description)
		}
		fmt.Printf("%s\n\n", p.URL)
		fmt.Printf("Items:        %d\n", len(items))
		fmt.Printf("Linked repos: %s\n", orNone(strings.Join(repos, ", ")))
		if sf, err := p.StatusField(); err == nil {
			var cols []string
			for _, o := range sf.Options {
				cols = append(cols, o.Name)
			}
			fmt.Printf("Columns:      %s\n", strings.Join(cols, " | "))
		}
		var fieldNames []string
		for _, f := range p.Fields {
			fieldNames = append(fieldNames, f.Name)
		}
		fmt.Printf("Fields:       %s\n", strings.Join(fieldNames, ", "))
		return nil
	},
}

var boardCmd = &cobra.Command{
	Use:   "board <project>",
	Short: "Show items grouped by Status column",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		n, err := parseNumber(args[0])
		if err != nil {
			return err
		}
		_, p, err := openProject(n)
		if err != nil {
			return err
		}
		items, err := p.Items()
		if err != nil {
			return err
		}
		if flagJSON {
			return printJSON(items)
		}

		// Column order follows the Status field's option order; unknowns last.
		order := map[string]int{}
		if sf, err := p.StatusField(); err == nil {
			for i, o := range sf.Options {
				order[o.Name] = i
			}
		}
		const noStatus = "(no status)"
		groups := map[string][]projects.Item{}
		for _, it := range items {
			s := it.Status()
			if s == "" {
				s = noStatus
			}
			groups[s] = append(groups[s], it)
		}
		cols := make([]string, 0, len(groups))
		for s := range groups {
			cols = append(cols, s)
		}
		sort.Slice(cols, func(i, j int) bool {
			oi, oki := order[cols[i]]
			oj, okj := order[cols[j]]
			if oki != okj {
				return oki // known columns before unknown
			}
			if oki && okj && oi != oj {
				return oi < oj
			}
			return cols[i] < cols[j]
		})
		for _, col := range cols {
			fmt.Printf("\n%s (%d)\n", col, len(groups[col]))
			for _, it := range groups[col] {
				fmt.Printf("  %s\n", itemLine(it))
			}
		}
		return nil
	},
}

var itemsCmd = &cobra.Command{
	Use:   "items <project>",
	Short: "List items, optionally filtered by Status column",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		status, _ := cmd.Flags().GetString("status")
		return listItems(args[0], status)
	},
}

var readyCmd = &cobra.Command{
	Use:   "ready <project>",
	Short: `List items ready for work (Status = "Todo")`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return listItems(args[0], "Todo")
	},
}

// listItems prints the project's items, optionally filtered to a Status column.
func listItems(projectArg, status string) error {
	n, err := parseNumber(projectArg)
	if err != nil {
		return err
	}
	_, p, err := openProject(n)
	if err != nil {
		return err
	}
	items, err := p.Items()
	if err != nil {
		return err
	}
	if status != "" {
		var filtered []projects.Item
		for _, it := range items {
			if strings.EqualFold(it.Status(), status) {
				filtered = append(filtered, it)
			}
		}
		items = filtered
	}
	if flagJSON {
		return printJSON(items)
	}
	if len(items) == 0 {
		fmt.Println("(no matching items)")
		return nil
	}
	for _, it := range items {
		fmt.Println(itemLine(it))
	}
	return nil
}

var fieldsCmd = &cobra.Command{
	Use:   "fields <project>",
	Short: "List the project's fields and single-select options",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		n, err := parseNumber(args[0])
		if err != nil {
			return err
		}
		_, p, err := openProject(n)
		if err != nil {
			return err
		}
		if flagJSON {
			return printJSON(p.Fields)
		}
		tw := newTab()
		fmt.Fprintln(tw, "FIELD\tTYPE\tOPTIONS")
		for _, f := range p.Fields {
			var opts []string
			for _, o := range f.Options {
				opts = append(opts, o.Name)
			}
			for _, it := range f.Iterations {
				opts = append(opts, it.Title)
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\n", f.Name, f.DataType, strings.Join(opts, ", "))
		}
		return tw.Flush()
	},
}

var linksCmd = &cobra.Command{
	Use:   "links <project>",
	Short: "List repositories the project is linked to",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		n, err := parseNumber(args[0])
		if err != nil {
			return err
		}
		_, p, err := openProject(n)
		if err != nil {
			return err
		}
		repos, err := p.LinkedRepos()
		if err != nil {
			return err
		}
		if flagJSON {
			return printJSON(repos)
		}
		if len(repos) == 0 {
			fmt.Println("(not linked to any repositories)")
			return nil
		}
		for _, r := range repos {
			fmt.Println(r)
		}
		return nil
	},
}

// itemLine renders a one-line summary of an item.
func itemLine(it projects.Item) string {
	var ref string
	switch {
	case it.IsDraft():
		ref = "draft"
	case it.Repo != "":
		ref = fmt.Sprintf("%s#%d", it.Repo, it.Number)
	default:
		ref = fmt.Sprintf("#%d", it.Number)
	}
	line := fmt.Sprintf("%-22s %s", ref, it.Title)
	var tags []string
	if s := it.Status(); s != "" {
		tags = append(tags, s)
	}
	if len(it.Labels) > 0 {
		tags = append(tags, strings.Join(it.Labels, ","))
	}
	if len(tags) > 0 {
		line += fmt.Sprintf("  [%s]", strings.Join(tags, " | "))
	}
	return line
}

func orNone(s string) string {
	if s == "" {
		return "(none)"
	}
	return s
}
