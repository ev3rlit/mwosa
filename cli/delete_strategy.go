package cli

import "github.com/spf13/cobra"

func newDeleteStrategyCommand(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "strategy <name>",
		Short: "Soft delete a saved screening strategy",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			runtime, err := newAppRuntime(opts, false)
			if err != nil {
				return err
			}
			defer closeAppRuntime(runtime, &err)

			if err := runtime.Services.Strategy.Delete(cmd.Context(), args[0]); err != nil {
				return err
			}
			return writeDeleteStrategyResult(cmd.OutOrStdout(), opts.Output, deleteStrategyResult{Name: args[0], Deleted: true})
		},
	}
}
