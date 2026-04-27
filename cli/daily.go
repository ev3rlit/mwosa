package cli

import (
	"fmt"
	"os"
	"strconv"

	provider "github.com/ev3rlit/mwosa/providers/core"
	"github.com/ev3rlit/mwosa/providers/core/dailybar"
	"github.com/ev3rlit/mwosa/providers/datago"
	"github.com/ev3rlit/mwosa/service/daily"
	"github.com/ev3rlit/mwosa/storage/sqlite"
	"github.com/spf13/cobra"
)

type dailyFlags struct {
	SecurityType string
	From         string
	To           string
	AsOf         string
}

func newGetCommand(opts *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Read source-like data from local storage",
	}
	cmd.AddCommand(newGetDailyCommand(opts))
	return cmd
}

func newEnsureCommand(opts *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ensure",
		Short: "Fetch missing data and store it locally",
	}
	cmd.AddCommand(newEnsureDailyCommand(opts))
	return cmd
}

func newSyncCommand(opts *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Refresh provider-backed data batches",
	}
	cmd.AddCommand(newSyncDailyCommand(opts))
	return cmd
}

func newBackfillCommand(opts *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backfill",
		Short: "Collect historical data ranges",
	}
	cmd.AddCommand(newBackfillDailyCommand(opts))
	return cmd
}

func newGetDailyCommand(opts *Options) *cobra.Command {
	flags := dailyFlags{SecurityType: string(provider.SecurityTypeETF)}
	cmd := &cobra.Command{
		Use:   "daily <symbol>",
		Short: "Read stored daily bars for a symbol",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			service, err := newDailyService(opts, false)
			if err != nil {
				return err
			}
			result, err := service.Get(cmd.Context(), daily.Request{
				Market:       provider.Market(opts.Market),
				SecurityType: provider.SecurityType(flags.SecurityType),
				Symbol:       args[0],
				From:         flags.From,
				To:           flags.To,
				AsOf:         flags.AsOf,
			})
			if err != nil {
				return err
			}
			return writeBars(cmd.OutOrStdout(), opts.Output, result.Bars)
		},
	}
	addDailyRangeFlags(cmd, &flags)
	return cmd
}

func newEnsureDailyCommand(opts *Options) *cobra.Command {
	flags := dailyFlags{SecurityType: string(provider.SecurityTypeETF)}
	cmd := &cobra.Command{
		Use:   "daily <symbol>",
		Short: "Fetch missing daily bars for a symbol and store them locally",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			service, err := newDailyService(opts, true)
			if err != nil {
				return err
			}
			result, err := service.Ensure(cmd.Context(), daily.Request{
				ProviderID:     provider.ProviderID(opts.Provider),
				PreferProvider: provider.ProviderID(opts.PreferProvider),
				Market:         provider.Market(opts.Market),
				SecurityType:   provider.SecurityType(flags.SecurityType),
				Symbol:         args[0],
				From:           flags.From,
				To:             flags.To,
				AsOf:           flags.AsOf,
			})
			if err != nil {
				return err
			}
			return writeBars(cmd.OutOrStdout(), opts.Output, result.Bars)
		},
	}
	addDailyRangeFlags(cmd, &flags)
	return cmd
}

func newSyncDailyCommand(opts *Options) *cobra.Command {
	flags := dailyFlags{SecurityType: string(provider.SecurityTypeETF)}
	cmd := &cobra.Command{
		Use:   "daily",
		Short: "Collect one provider daily batch for a date",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			service, err := newDailyService(opts, true)
			if err != nil {
				return err
			}
			result, err := service.Sync(cmd.Context(), daily.Request{
				ProviderID:     provider.ProviderID(opts.Provider),
				PreferProvider: provider.ProviderID(opts.PreferProvider),
				Market:         provider.Market(opts.Market),
				SecurityType:   provider.SecurityType(flags.SecurityType),
				AsOf:           flags.AsOf,
			})
			if err != nil {
				return err
			}
			return writeCollectResult(cmd.OutOrStdout(), opts.Output, result)
		},
	}
	addSecurityTypeFlag(cmd, &flags)
	cmd.Flags().StringVar(&flags.AsOf, "as-of", flags.AsOf, "trading date to collect, YYYYMMDD or YYYY-MM-DD")
	return cmd
}

func newBackfillDailyCommand(opts *Options) *cobra.Command {
	flags := dailyFlags{SecurityType: string(provider.SecurityTypeETF)}
	cmd := &cobra.Command{
		Use:   "daily",
		Short: "Collect provider daily batches for a date range",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			service, err := newDailyService(opts, true)
			if err != nil {
				return err
			}
			result, err := service.Backfill(cmd.Context(), daily.Request{
				ProviderID:     provider.ProviderID(opts.Provider),
				PreferProvider: provider.ProviderID(opts.PreferProvider),
				Market:         provider.Market(opts.Market),
				SecurityType:   provider.SecurityType(flags.SecurityType),
				From:           flags.From,
				To:             flags.To,
			})
			if err != nil {
				return err
			}
			return writeCollectResult(cmd.OutOrStdout(), opts.Output, result)
		},
	}
	addSecurityTypeFlag(cmd, &flags)
	cmd.Flags().StringVar(&flags.From, "from", flags.From, "start trading date, YYYYMMDD or YYYY-MM-DD")
	cmd.Flags().StringVar(&flags.To, "to", flags.To, "end trading date, YYYYMMDD or YYYY-MM-DD")
	return cmd
}

func addDailyRangeFlags(cmd *cobra.Command, flags *dailyFlags) {
	addSecurityTypeFlag(cmd, flags)
	cmd.Flags().StringVar(&flags.From, "from", flags.From, "start trading date, YYYYMMDD or YYYY-MM-DD")
	cmd.Flags().StringVar(&flags.To, "to", flags.To, "end trading date, YYYYMMDD or YYYY-MM-DD")
	cmd.Flags().StringVar(&flags.AsOf, "as-of", flags.AsOf, "single trading date, YYYYMMDD or YYYY-MM-DD")
}

func addSecurityTypeFlag(cmd *cobra.Command, flags *dailyFlags) {
	cmd.Flags().StringVar(&flags.SecurityType, "security-type", flags.SecurityType, "security type: etf, etn, elw")
}

func newDailyService(opts *Options, withProvider bool) (daily.Service, error) {
	reader, writer := sqlite.NewDailyBarRepositories(opts.Database)
	service := daily.Service{
		Reader: reader,
		Writer: writer,
	}
	if !withProvider {
		return service, nil
	}

	registry := provider.NewRegistry()
	shouldRegisterDataGo := opts.Provider == "" || opts.Provider == string(provider.ProviderDataGo) || opts.PreferProvider == string(provider.ProviderDataGo)
	if shouldRegisterDataGo {
		p, err := newDataGoProviderFromEnv()
		if err != nil {
			return daily.Service{}, err
		}
		if err := datago.Register(registry, p); err != nil {
			return daily.Service{}, err
		}
	}
	service.Router = dailybar.NewRouter(provider.NewRouter(registry))
	return service, nil
}

func newDataGoProviderFromEnv() (*datago.Provider, error) {
	serviceKey := os.Getenv("MWOSA_DATAGO_SERVICE_KEY")
	if serviceKey == "" {
		serviceKey = os.Getenv("DATAGO_SERVICE_KEY")
	}
	if serviceKey == "" {
		return nil, fmt.Errorf("datago service key is required: set MWOSA_DATAGO_SERVICE_KEY or DATAGO_SERVICE_KEY")
	}

	config := datago.Config{
		ServiceKey: serviceKey,
		BaseURL:    os.Getenv("MWOSA_DATAGO_BASE_URL"),
	}
	if value := os.Getenv("MWOSA_DATAGO_NUM_OF_ROWS"); value != "" {
		numOfRows, err := strconv.Atoi(value)
		if err != nil {
			return nil, fmt.Errorf("MWOSA_DATAGO_NUM_OF_ROWS must be an integer: %w", err)
		}
		config.NumOfRows = numOfRows
	}
	return datago.New(config)
}
