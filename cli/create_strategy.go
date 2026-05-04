package cli

import (
	strategyservice "github.com/ev3rlit/mwosa/service/strategy"
	"github.com/spf13/cobra"
)

func newCreateStrategyCommand(opts *Options) *cobra.Command {
	flags := strategySourceFlags{Engine: string(strategyservice.EngineJQ)}
	cmd := &cobra.Command{
		Use:   "strategy <name>",
		Short: "Create a saved screening strategy",
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

			result, err := runtime.Services.Strategy.Create(cmd.Context(), strategyservice.CreateStrategyRequest{
				Name:         args[0],
				Engine:       strategyservice.Engine(flags.Engine),
				InputDataset: flags.Input,
				QueryText:    queryText,
			})
			if err != nil {
				return err
			}
			return writeStrategyDetail(cmd.OutOrStdout(), opts.Output, result)
		},
	}
	addStrategySourceFlags(cmd, &flags, true)
	return cmd
}
