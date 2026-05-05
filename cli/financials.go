package cli

import (
	provider "github.com/ev3rlit/mwosa/providers/core"
	"github.com/ev3rlit/mwosa/providers/core/financials"
	financialsservice "github.com/ev3rlit/mwosa/service/financials"
	"github.com/spf13/cobra"
)

type financialsFlags struct {
	SecurityType string
	FiscalYear   string
	Period       string
	Statement    string
	Limit        int
}

func registerFinancialsCommands(roots commandRoots, opts *Options) {
	roots.Get.AddCommand(newGetFinancialsCommand(opts))
}

func newGetFinancialsCommand(opts *Options) *cobra.Command {
	flags := financialsFlags{
		SecurityType: string(provider.SecurityTypeStock),
		Period:       string(financials.PeriodTypeAnnual),
	}
	cmd := &cobra.Command{
		Use:   "financials <company>",
		Short: "Fetch provider-backed financial statements by company name or KRX code",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			runtime, err := newAppRuntime(opts, true)
			if err != nil {
				return err
			}
			defer closeAppRuntime(runtime, &err)

			result, err := runtime.Services.Financials.Get(cmd.Context(), financialsservice.Request{
				ProviderID:     provider.ProviderID(opts.Provider),
				PreferProvider: provider.ProviderID(opts.PreferProvider),
				Market:         provider.Market(opts.Market),
				SecurityType:   provider.SecurityType(flags.SecurityType),
				Symbol:         args[0],
				FiscalYear:     flags.FiscalYear,
				Period:         financials.PeriodType(flags.Period),
				Statement:      financials.StatementType(flags.Statement),
				Limit:          flags.Limit,
			})
			if err != nil {
				return err
			}
			return writeFinancialStatements(cmd.OutOrStdout(), opts.Output, result.Statements)
		},
	}
	cmd.Flags().StringVar(&flags.SecurityType, "security-type", flags.SecurityType, "security type: stock, etf, etn, elw")
	cmd.Flags().StringVar(&flags.FiscalYear, "year", flags.FiscalYear, "fiscal year, for example 2025")
	cmd.Flags().StringVar(&flags.Period, "period", flags.Period, "financial period: annual, quarter")
	cmd.Flags().StringVar(&flags.Statement, "statement", flags.Statement, "statement type: summary, income_statement, balance_sheet, cash_flow; empty fetches all")
	cmd.Flags().IntVar(&flags.Limit, "limit", flags.Limit, "maximum number of statement rows to fetch")
	mustRegisterFlagCompletion(cmd, "security-type", completeSecurityTypes)
	mustRegisterFlagCompletion(cmd, "period", completeFinancialPeriods)
	mustRegisterFlagCompletion(cmd, "statement", completeFinancialStatementTypes)
	return cmd
}
