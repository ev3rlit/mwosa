package cli

import "github.com/spf13/cobra"

func newScreenCommand(opts *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "screen",
		Short: "Run screening workflows",
	}
	cmd.AddCommand(newScreenStrategyCommand(opts))
	return cmd
}
