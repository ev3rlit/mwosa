package cli

import (
	strategyservice "github.com/ev3rlit/mwosa/service/strategy"
	"github.com/spf13/cobra"
)

func newUpdateStrategyCommand(opts *Options) *cobra.Command {
	flags := strategySourceFlags{}
	cmd := &cobra.Command{
		Use:   "strategy <name>",
		Short: "Create a new version of a saved screening strategy",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			queryText, err := resolveJQSource(flags)
			if err != nil {
				return err
			}
			runtime, err := newAppRuntime(opts, false)
			if err != nil {
				return err
			}
			defer closeAppRuntime(runtime, &err)

			result, err := runtime.Services.Strategy.Update(cmd.Context(), strategyservice.UpdateStrategyRequest{
				Name:      args[0],
				QueryText: queryText,
			})
			if err != nil {
				return err
			}
			return writeStrategyDetail(cmd.OutOrStdout(), opts.Output, result)
		},
	}
	addJQFlags(cmd, &flags)
	return cmd
}
