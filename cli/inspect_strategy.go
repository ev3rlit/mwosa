package cli

import "github.com/spf13/cobra"

func newInspectStrategyCommand(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "strategy <name>",
		Short: "Inspect a saved screening strategy",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			runtime, err := newAppRuntime(opts, false)
			if err != nil {
				return err
			}
			defer closeAppRuntime(runtime, &err)

			result, err := runtime.Services.Strategy.Inspect(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return writeStrategyDetail(cmd.OutOrStdout(), opts.Output, result)
		},
	}
}
