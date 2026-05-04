package cli

import (
	strategyservice "github.com/ev3rlit/mwosa/service/strategy"
	"github.com/spf13/cobra"
)

func registerStrategyCommands(roots commandRoots, opts *Options) {
	roots.Create.AddCommand(newCreateStrategyCommand(opts))
	roots.List.AddCommand(newListStrategiesCommand(opts))
	roots.Update.AddCommand(newUpdateStrategyCommand(opts))
	roots.Delete.AddCommand(newDeleteStrategyCommand(opts))
	roots.Screen.AddCommand(newScreenStrategyCommand(opts))
	roots.History.AddCommand(newHistoryScreenCommand(opts))
	roots.Inspect.AddCommand(newInspectStrategyCommand(opts))
	roots.Inspect.AddCommand(newInspectScreenCommand(opts))
}

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
