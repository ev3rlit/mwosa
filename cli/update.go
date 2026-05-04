package cli

import "github.com/spf13/cobra"

func newUpdateCommand(opts *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update mwosa resources",
	}
	cmd.AddCommand(newUpdateStrategyCommand(opts))
	return cmd
}
