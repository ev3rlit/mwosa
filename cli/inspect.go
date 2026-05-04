package cli

import "github.com/spf13/cobra"

func newInspectCommand(opts *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "inspect",
		Short: "Inspect mwosa resources and local state",
	}
	cmd.AddCommand(newInspectConfigCommand(opts))
	cmd.AddCommand(newInspectStrategyCommand(opts))
	cmd.AddCommand(newInspectScreenCommand(opts))
	return cmd
}
