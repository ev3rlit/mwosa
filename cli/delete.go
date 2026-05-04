package cli

import "github.com/spf13/cobra"

func newDeleteCommand(opts *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete mwosa resources",
	}
	cmd.AddCommand(newDeleteStrategyCommand(opts))
	return cmd
}
