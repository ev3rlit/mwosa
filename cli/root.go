package cli

import (
	"fmt"
	"io"
	"runtime"

	provider "github.com/ev3rlit/mwosa/providers/core"
	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/samber/oops"
	"github.com/spf13/cobra"
)

const (
	defaultVersion = "dev"
	schemaVersion  = "dev"
)

type BuildInfo struct {
	Version string
	Commit  string
	Date    string
}

type Options struct {
	// 필수. 명령 결과를 출력할 형식이다.
	Output OutputMode

	// 선택. 비어 있으면 provider router 가 요청에 맞는 provider 를 고른다.
	Provider string

	// 선택. 비어 있으면 provider router 의 기본 우선순위를 따른다.
	PreferProvider string

	// 필수. provider routing 과 storage query 에 사용할 시장 ID 다.
	Market string

	// 필수. 로컬 SQLite database 경로다.
	Database string
}

func (opts Options) Validate() error {
	return validation.ValidateStruct(&opts,
		validation.Field(&opts.Output, validation.Required, validation.By(validateOutputMode)),
		validation.Field(&opts.Provider),
		validation.Field(&opts.PreferProvider),
		validation.Field(&opts.Market, validation.Required),
		validation.Field(&opts.Database, validation.Required),
	)
}

func validateOutputMode(value any) error {
	mode, ok := value.(OutputMode)
	if !ok {
		return oops.In("cli").New("output mode has invalid type")
	}
	_, err := ParseOutputMode(string(mode))
	return err
}

func NewRootCommand(build BuildInfo) *cobra.Command {
	opts := Options{
		Output:   DefaultOutputMode,
		Market:   string(provider.MarketKRX),
		Database: ".mwosa-data/mwosa.db",
	}

	cmd := &cobra.Command{
		Use:           "mwosa",
		Short:         "Investment research CLI for provider-backed market data",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.PersistentFlags().VarP(
		&opts.Output,
		"output",
		"o",
		OutputModeHelp(),
	)
	cmd.PersistentFlags().StringVar(
		&opts.Provider,
		"provider",
		opts.Provider,
		"force a provider by id",
	)
	cmd.PersistentFlags().StringVar(
		&opts.PreferProvider,
		"prefer-provider",
		opts.PreferProvider,
		"prefer a provider when multiple candidates match",
	)
	cmd.PersistentFlags().StringVar(
		&opts.Market,
		"market",
		opts.Market,
		"market id",
	)
	cmd.PersistentFlags().StringVar(
		&opts.Database,
		"database",
		opts.Database,
		"local SQLite database path",
	)

	cmd.AddCommand(newVersionCommand(build))
	cmd.AddCommand(newGetCommand(&opts))
	cmd.AddCommand(newEnsureCommand(&opts))
	cmd.AddCommand(newSyncCommand(&opts))
	cmd.AddCommand(newBackfillCommand(&opts))

	return cmd
}

func newVersionCommand(build BuildInfo) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print mwosa build information",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			writeVersion(cmd.OutOrStdout(), normalizeBuildInfo(build))
			return nil
		},
	}
}

func normalizeBuildInfo(build BuildInfo) BuildInfo {
	if build.Version == "" {
		build.Version = defaultVersion
	}
	if build.Commit == "" {
		build.Commit = "unknown"
	}
	if build.Date == "" {
		build.Date = "unknown"
	}
	return build
}

func writeVersion(w io.Writer, build BuildInfo) {
	_, _ = fmt.Fprintf(w, "mwosa %s\n", build.Version)
	_, _ = fmt.Fprintf(w, "schema %s\n", schemaVersion)
	_, _ = fmt.Fprintf(w, "commit %s\n", build.Commit)
	_, _ = fmt.Fprintf(w, "built %s\n", build.Date)
	_, _ = fmt.Fprintf(w, "go %s\n", runtime.Version())
}
