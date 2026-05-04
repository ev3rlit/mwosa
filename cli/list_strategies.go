package cli

import "github.com/spf13/cobra"

func newListStrategiesCommand(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "strategies",
		Short: "List saved screening strategies",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) (err error) {
			runtime, err := newAppRuntime(opts, false)
			if err != nil {
				return err
			}
			defer closeAppRuntime(runtime, &err)

			result, err := runtime.Services.Strategy.List(cmd.Context())
			if err != nil {
				return err
			}
			return writeStrategyList(cmd.OutOrStdout(), opts.Output, result)
		},
	}
}
