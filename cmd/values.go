package cmd

import (
	"fmt"

	"github.com/NSExceptional/gh-projects/internal/projects"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(moveCmd, setCmd, checkCmd, uncheckCmd)
}

var moveCmd = &cobra.Command{
	Use:   "move <project> <issue-number> <column>",
	Short: "Move an item to a Status column",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		n, err := parseNumber(args[0])
		if err != nil {
			return err
		}
		issueNum, err := parseNumber(args[1])
		if err != nil {
			return err
		}
		_, p, err := openProject(n)
		if err != nil {
			return err
		}
		it, err := p.ItemByNumber(issueNum)
		if err != nil {
			return err
		}
		if err := setStatus(p, it.ID, args[2]); err != nil {
			return err
		}
		fmt.Printf("moved #%d to %q\n", issueNum, args[2])
		return nil
	},
}

var setCmd = &cobra.Command{
	Use:   "set <project> <issue-number> <field> <value>",
	Short: "Set a field value on an item",
	Args:  cobra.ExactArgs(4),
	RunE: func(cmd *cobra.Command, args []string) error {
		n, err := parseNumber(args[0])
		if err != nil {
			return err
		}
		issueNum, err := parseNumber(args[1])
		if err != nil {
			return err
		}
		_, p, err := openProject(n)
		if err != nil {
			return err
		}
		field, err := p.FieldByName(args[2])
		if err != nil {
			return err
		}
		it, err := p.ItemByNumber(issueNum)
		if err != nil {
			return err
		}
		if err := p.SetFieldValue(it.ID, field, args[3]); err != nil {
			return err
		}
		fmt.Printf("set %s = %q on #%d\n", field.Name, args[3], issueNum)
		return nil
	},
}

var checkCmd = &cobra.Command{
	Use:   "check <project> <issue-number> <task-text>",
	Short: "Check a task-list box in an item's body",
	Args:  cobra.ExactArgs(3),
	RunE:  func(cmd *cobra.Command, args []string) error { return toggle(args, true) },
}

var uncheckCmd = &cobra.Command{
	Use:   "uncheck <project> <issue-number> <task-text>",
	Short: "Uncheck a task-list box in an item's body",
	Args:  cobra.ExactArgs(3),
	RunE:  func(cmd *cobra.Command, args []string) error { return toggle(args, false) },
}

func toggle(args []string, checked bool) error {
	n, err := parseNumber(args[0])
	if err != nil {
		return err
	}
	issueNum, err := parseNumber(args[1])
	if err != nil {
		return err
	}
	c, p, err := openProject(n)
	if err != nil {
		return err
	}
	it, err := p.ItemByNumber(issueNum)
	if err != nil {
		return err
	}
	newBody, matched, err := projects.ToggleChecklist(it.Body, args[2], checked)
	if err != nil {
		return err
	}
	if newBody == it.Body {
		fmt.Printf("already %s: %q\n", checkedWord(checked), matched)
		return nil
	}
	if it.IsDraft() {
		err = c.UpdateDraftBody(it.ContentID, newBody)
	} else {
		err = c.UpdateIssueBody(it.ContentID, newBody)
	}
	if err != nil {
		return err
	}
	fmt.Printf("%s: %q\n", checkedWord(checked), matched)
	return nil
}

func checkedWord(checked bool) string {
	if checked {
		return "checked"
	}
	return "unchecked"
}
