package cli

import "github.com/spf13/cobra"

func newListCommand(opts *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List mwosa resources",
	}
	cmd.AddCommand(newListStrategiesCommand(opts))
	return cmd
}
