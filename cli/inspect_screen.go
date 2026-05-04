package cli

import "github.com/spf13/cobra"

func newInspectScreenCommand(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "screen <screen-id-or-alias>",
		Short: "Inspect a saved screening run",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			runtime, err := newAppRuntime(opts, false)
			if err != nil {
				return err
			}
			defer closeAppRuntime(runtime, &err)

			result, err := runtime.Services.Strategy.InspectScreen(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return writeScreenRunDetail(cmd.OutOrStdout(), opts.Output, result)
		},
	}
}
