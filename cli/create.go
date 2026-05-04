package cli

import "github.com/spf13/cobra"

func newCreateCommand(opts *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create mwosa resources",
	}
	cmd.AddCommand(newCreateStrategyCommand(opts))
	return cmd
}
