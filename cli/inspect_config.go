package cli

import "github.com/spf13/cobra"

func newInspectConfigCommand(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Inspect resolved config and data paths",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := loadConfig(opts); err != nil {
				return err
			}
			return writeConfigOutput(cmd.OutOrStdout(), configInspectFromResolved(opts.ConfigState))
		},
	}
}
