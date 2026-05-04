package cli

import (
	strategyservice "github.com/ev3rlit/mwosa/service/strategy"
	"github.com/spf13/cobra"
)

func newScreenStrategyCommand(opts *Options) *cobra.Command {
	flags := strategySourceFlags{}
	cmd := &cobra.Command{
		Use:   "strategy <name>",
		Short: "Run a saved screening strategy",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			runtime, err := newAppRuntime(opts, false)
			if err != nil {
				return err
			}
			defer closeAppRuntime(runtime, &err)

			result, err := runtime.Services.Strategy.Screen(cmd.Context(), strategyservice.ScreenStrategyRequest{
				Name:  args[0],
				Alias: flags.Alias,
			})
			if err != nil {
				return err
			}
			return writeScreenRunDetail(cmd.OutOrStdout(), opts.Output, result)
		},
	}
	cmd.Flags().StringVar(&flags.Alias, "alias", flags.Alias, "optional screen run alias")
	return cmd
}
