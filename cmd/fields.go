package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(fieldCreateCmd, fieldDeleteCmd)
	fieldCreateCmd.Flags().String("name", "", "field name (required)")
	fieldCreateCmd.Flags().String("type", "", "field type: text, number, date, single_select (required)")
	fieldCreateCmd.Flags().StringSlice("option", nil, "single-select option (repeatable)")
	fieldCreateCmd.MarkFlagRequired("name")
	fieldCreateCmd.MarkFlagRequired("type")
}

var fieldTypeAliases = map[string]string{
	"text":          "TEXT",
	"number":        "NUMBER",
	"date":          "DATE",
	"single_select": "SINGLE_SELECT",
	"select":        "SINGLE_SELECT",
	"single-select": "SINGLE_SELECT",
}

var fieldCreateCmd = &cobra.Command{
	Use:   "field-create <project>",
	Short: "Create a custom field",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		n, err := parseNumber(args[0])
		if err != nil {
			return err
		}
		name, _ := cmd.Flags().GetString("name")
		typ, _ := cmd.Flags().GetString("type")
		options, _ := cmd.Flags().GetStringSlice("option")

		dataType, ok := fieldTypeAliases[strings.ToLower(typ)]
		if !ok {
			return fmt.Errorf("unknown field type %q (want text, number, date, or single_select)", typ)
		}
		if dataType == "SINGLE_SELECT" && len(options) == 0 {
			return fmt.Errorf("single_select fields need at least one --option")
		}

		_, p, err := openProject(n)
		if err != nil {
			return err
		}
		f, err := p.CreateField(name, dataType, options)
		if err != nil {
			return err
		}
		fmt.Printf("created field %q (%s) on project #%d\n", f.Name, f.DataType, p.Number)
		return nil
	},
}

var fieldDeleteCmd = &cobra.Command{
	Use:   "field-delete <project> <field-name>",
	Short: "Delete a custom field",
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
		f, err := p.FieldByName(args[1])
		if err != nil {
			return err
		}
		if err := p.DeleteField(f.ID); err != nil {
			return err
		}
		fmt.Printf("deleted field %q from project #%d\n", f.Name, p.Number)
		return nil
	},
}
