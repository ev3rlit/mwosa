package cli

import (
	"fmt"
	"io"
	"runtime"

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
	Output string
}

func NewRootCommand(build BuildInfo) *cobra.Command {
	opts := Options{Output: "table"}

	cmd := &cobra.Command{
		Use:           "mwosa",
		Short:         "Investment research CLI for provider-backed market data",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.PersistentFlags().StringVarP(
		&opts.Output,
		"output",
		"o",
		opts.Output,
		"output format: table, json, ndjson, csv",
	)

	cmd.AddCommand(newVersionCommand(build))

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
