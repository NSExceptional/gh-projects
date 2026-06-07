package cmd

import (
	"fmt"
	"strings"

	"github.com/NSExceptional/gh-projects/internal/projects"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(createCmd, draftCmd, addCmd, rmCmd, convertCmd)

	createCmd.Flags().String("repo", "", "target repository (owner/name); defaults to the project's sole linked repo")
	createCmd.Flags().String("title", "", "issue title (required)")
	createCmd.Flags().String("body", "", "issue body")
	createCmd.Flags().StringSlice("label", nil, "label to apply (repeatable)")
	createCmd.Flags().String("status", "", "Status column to set after creating")
	createCmd.MarkFlagRequired("title")

	draftCmd.Flags().String("title", "", "draft title (required)")
	draftCmd.Flags().String("body", "", "draft body")
	draftCmd.Flags().String("status", "", "Status column to set after creating")
	draftCmd.MarkFlagRequired("title")

	addCmd.Flags().String("repo", "", "repository (owner/name) when passing a bare issue number")
	convertCmd.Flags().String("repo", "", "target repository (owner/name) for the new issue")
	convertCmd.MarkFlagRequired("repo")
}

var createCmd = &cobra.Command{
	Use:   "create <project>",
	Short: "Create a real issue and add it to the project",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		n, err := parseNumber(args[0])
		if err != nil {
			return err
		}
		title, _ := cmd.Flags().GetString("title")
		body, _ := cmd.Flags().GetString("body")
		labels, _ := cmd.Flags().GetStringSlice("label")
		repoFlag, _ := cmd.Flags().GetString("repo")
		status, _ := cmd.Flags().GetString("status")

		c, p, err := openProject(n)
		if err != nil {
			return err
		}

		repoName, err := defaultRepo(p, repoFlag)
		if err != nil {
			return err
		}
		repo, err := c.Repo(repoName)
		if err != nil {
			return err
		}
		labelIDs, err := c.LabelIDs(repoName, labels)
		if err != nil {
			return err
		}
		issue, err := c.CreateIssue(repo.ID, title, body, labelIDs)
		if err != nil {
			return err
		}
		itemID, err := p.AddItem(issue.ID)
		if err != nil {
			return fmt.Errorf("issue %s created but adding to project failed: %w", issue.URL, err)
		}
		if status != "" {
			if err := setStatus(p, itemID, status); err != nil {
				return fmt.Errorf("issue created and added, but setting status failed: %w", err)
			}
		}
		fmt.Printf("created %s#%d and added to project #%d\n%s\n", repo.NameWithOwner, issue.Number, p.Number, issue.URL)
		return nil
	},
}

var draftCmd = &cobra.Command{
	Use:   "draft <project>",
	Short: "Add a draft issue (project-only) to the project",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		n, err := parseNumber(args[0])
		if err != nil {
			return err
		}
		title, _ := cmd.Flags().GetString("title")
		body, _ := cmd.Flags().GetString("body")
		status, _ := cmd.Flags().GetString("status")

		_, p, err := openProject(n)
		if err != nil {
			return err
		}
		itemID, err := p.AddDraft(title, body)
		if err != nil {
			return err
		}
		if status != "" {
			if err := setStatus(p, itemID, status); err != nil {
				return fmt.Errorf("draft created, but setting status failed: %w", err)
			}
		}
		fmt.Printf("added draft %q to project #%d\n", title, p.Number)
		return nil
	},
}

var addCmd = &cobra.Command{
	Use:   "add <project> <issue-url-or-number>",
	Short: "Add an existing issue or PR to the project",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		n, err := parseNumber(args[0])
		if err != nil {
			return err
		}
		repoFlag, _ := cmd.Flags().GetString("repo")

		c, p, err := openProject(n)
		if err != nil {
			return err
		}
		repoName := repoFlag
		if repoName == "" {
			// fall back to the sole linked repo for bare numbers
			repoName, _ = defaultRepo(p, "")
		}
		content, err := c.Content(repoName, args[1])
		if err != nil {
			return err
		}
		if _, err := p.AddItem(content.ID); err != nil {
			return err
		}
		fmt.Printf("added #%d (%s) to project #%d\n", content.Number, content.Title, p.Number)
		return nil
	},
}

var rmCmd = &cobra.Command{
	Use:   "rm <project> <issue-number-or-item-id>",
	Short: "Remove an item from the project (does not delete the issue)",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		n, err := parseNumber(args[0])
		if err != nil {
			return err
		}
		_, p, err := openProject(n)
		if err != nil {
			return err
		}
		itemID, desc, err := resolveItemID(p, args[1])
		if err != nil {
			return err
		}
		if err := p.RemoveItem(itemID); err != nil {
			return err
		}
		fmt.Printf("removed %s from project #%d\n", desc, p.Number)
		return nil
	},
}

var convertCmd = &cobra.Command{
	Use:   "convert <project> <draft-title-or-item-id>",
	Short: "Convert a draft issue into a real issue",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		n, err := parseNumber(args[0])
		if err != nil {
			return err
		}
		repoName, _ := cmd.Flags().GetString("repo")
		c, p, err := openProject(n)
		if err != nil {
			return err
		}
		repo, err := c.Repo(repoName)
		if err != nil {
			return err
		}
		item, err := findDraft(p, args[1])
		if err != nil {
			return err
		}
		if err := p.ConvertDraft(item.ID, repo.ID); err != nil {
			return err
		}
		fmt.Printf("converted draft %q to an issue in %s\n", item.Title, repo.NameWithOwner)
		return nil
	},
}

// defaultRepo returns the explicit repo flag if set, otherwise the project's
// sole linked repository, erroring if the choice is ambiguous.
func defaultRepo(p *projects.Project, flag string) (string, error) {
	if flag != "" {
		return flag, nil
	}
	repos, err := p.LinkedRepos()
	if err != nil {
		return "", err
	}
	switch len(repos) {
	case 1:
		return repos[0], nil
	case 0:
		return "", fmt.Errorf("project #%d is not linked to a repo; pass --repo owner/name", p.Number)
	default:
		return "", fmt.Errorf("project #%d is linked to %d repos; pass --repo owner/name", p.Number, len(repos))
	}
}

// resolveItemID maps a user-supplied reference (issue number or item node id)
// to an item id and a human description.
func resolveItemID(p *projects.Project, ref string) (id, desc string, err error) {
	if strings.HasPrefix(ref, "PVTI_") {
		return ref, ref, nil
	}
	n, err := parseNumber(ref)
	if err != nil {
		return "", "", fmt.Errorf("expected an issue number or PVTI_ item id, got %q", ref)
	}
	it, err := p.ItemByNumber(n)
	if err != nil {
		return "", "", err
	}
	return it.ID, fmt.Sprintf("#%d (%s)", it.Number, it.Title), nil
}

// findDraft locates a draft item by node id or unique title substring.
func findDraft(p *projects.Project, ref string) (projects.Item, error) {
	items, err := p.Items()
	if err != nil {
		return projects.Item{}, err
	}
	if strings.HasPrefix(ref, "PVTI_") {
		for _, it := range items {
			if it.ID == ref {
				return it, nil
			}
		}
		return projects.Item{}, fmt.Errorf("no item with id %q", ref)
	}
	var matches []projects.Item
	for _, it := range items {
		if it.IsDraft() && strings.Contains(strings.ToLower(it.Title), strings.ToLower(ref)) {
			matches = append(matches, it)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return projects.Item{}, fmt.Errorf("no draft matching %q", ref)
	default:
		return projects.Item{}, fmt.Errorf("%d drafts match %q; be more specific", len(matches), ref)
	}
}

// setStatus sets an item's Status field to the named column.
func setStatus(p *projects.Project, itemID, column string) error {
	sf, err := p.StatusField()
	if err != nil {
		return err
	}
	return p.SetFieldValue(itemID, sf, column)
}
