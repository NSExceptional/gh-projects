package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(linkCmd, unlinkCmd)
}

var linkCmd = &cobra.Command{
	Use:   "link <project> <owner/repo>",
	Short: "Link the project to a repository (shows on that repo's Projects tab)",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		n, err := parseNumber(args[0])
		if err != nil {
			return err
		}
		c, p, err := openProject(n)
		if err != nil {
			return err
		}
		repo, err := c.Repo(args[1])
		if err != nil {
			return err
		}
		if err := p.LinkRepo(repo.ID); err != nil {
			return err
		}
		fmt.Printf("linked project #%d to %s\n", p.Number, repo.NameWithOwner)
		return nil
	},
}

var unlinkCmd = &cobra.Command{
	Use:   "unlink <project> <owner/repo>",
	Short: "Unlink the project from a repository",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		n, err := parseNumber(args[0])
		if err != nil {
			return err
		}
		c, p, err := openProject(n)
		if err != nil {
			return err
		}
		repo, err := c.Repo(args[1])
		if err != nil {
			return err
		}
		if err := p.UnlinkRepo(repo.ID); err != nil {
			return err
		}
		fmt.Printf("unlinked project #%d from %s\n", p.Number, repo.NameWithOwner)
		return nil
	},
}
