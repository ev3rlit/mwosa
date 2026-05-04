package cli

import "github.com/spf13/cobra"

func newHistoryCommand(opts *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "history",
		Short: "List mwosa execution history",
	}
	cmd.AddCommand(newHistoryScreenCommand(opts))
	return cmd
}
