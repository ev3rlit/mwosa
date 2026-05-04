package cli

import "github.com/spf13/cobra"

func newHistoryScreenCommand(opts *Options) *cobra.Command {
	flags := strategySourceFlags{History: 50}
	cmd := &cobra.Command{
		Use:   "screen",
		Short: "List saved screening runs",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) (err error) {
			runtime, err := newAppRuntime(opts, false)
			if err != nil {
				return err
			}
			defer closeAppRuntime(runtime, &err)

			result, err := runtime.Services.Strategy.History(cmd.Context(), flags.History)
			if err != nil {
				return err
			}
			return writeScreenRunHistory(cmd.OutOrStdout(), opts.Output, result)
		},
	}
	cmd.Flags().IntVar(&flags.History, "limit", flags.History, "maximum number of screen runs to list")
	return cmd
}
