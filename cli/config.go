package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"

	appconfig "github.com/ev3rlit/mwosa/app/config"
	provider "github.com/ev3rlit/mwosa/providers/core"
	"github.com/samber/oops"
	"github.com/spf13/cobra"
)

type configInspectResult struct {
	ConfigFile   configFileInspect     `json:"config_file"`
	DatabaseFile databaseFileInspect   `json:"database_file"`
	DataDir      dataDirectoryInspect  `json:"data_directory"`
	App          appConfigInspect      `json:"app"`
	Providers    []providerInspectItem `json:"providers"`
}

type configFileInspect struct {
	Path    string `json:"path"`
	Source  string `json:"source"`
	Exists  bool   `json:"exists"`
	Created bool   `json:"created"`
}

type databaseFileInspect struct {
	Path   string `json:"path"`
	Source string `json:"source"`
}

type dataDirectoryInspect struct {
	Path   string `json:"path"`
	Exists bool   `json:"exists"`
}

type appConfigInspect struct {
	Market string `json:"market"`
}

type providerInspectItem struct {
	ID      string              `json:"id"`
	Enabled bool                `json:"enabled"`
	Groups  []providerGroupItem `json:"groups"`
	Auth    map[string]bool     `json:"auth"`
}

type providerGroupItem struct {
	ID      string `json:"id"`
	Enabled bool   `json:"enabled"`
}

type configSetResult struct {
	ConfigFile string `json:"config_file"`
	Setting    string `json:"setting"`
	Value      string `json:"value"`
}

func newInspectCommand(opts *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "inspect",
		Short: "Inspect mwosa resources and local state",
	}
	cmd.AddCommand(newInspectConfigCommand(opts))
	return cmd
}

func newInspectConfigCommand(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Inspect resolved config and data paths",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := loadConfig(opts); err != nil {
				return err
			}
			return writeConfigInspect(cmd.OutOrStdout(), opts.Output, configInspectFromResolved(opts.ConfigState))
		},
	}
}

func newConfigCommand(opts *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage mwosa config file",
	}
	cmd.AddCommand(newConfigSetCommand(opts))
	return cmd
}

func newConfigSetCommand(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "set <path> <value>",
		Short: "Set a config value",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, err := appconfig.SetValue(appconfig.Options{
				ConfigPath:       opts.Config,
				Market:           opts.Market,
				ProviderDefaults: providerDefaults(),
			}, args[0], args[1])
			if err != nil {
				return oops.In("cli").Wrapf(err, "set config")
			}
			opts.Config = resolved.ConfigPath
			opts.Database = resolved.DatabasePath
			opts.ProviderConfig = resolved.ProviderConfig
			opts.ConfigState = resolved
			opts.configLoaded = true
			return writeConfigSetResult(cmd.OutOrStdout(), opts.Output, configSetResult{
				ConfigFile: resolved.ConfigPath,
				Setting:    args[0],
				Value:      maskedConfigSetValue(args[0], args[1]),
			})
		},
	}
}

func configInspectFromResolved(resolved appconfig.Resolved) configInspectResult {
	result := configInspectResult{
		ConfigFile: configFileInspect{
			Path:    resolved.ConfigPath,
			Source:  string(resolved.ConfigPathSource),
			Exists:  resolved.ConfigFileExists,
			Created: resolved.ConfigFileCreated,
		},
		DatabaseFile: databaseFileInspect{
			Path:   resolved.DatabasePath,
			Source: string(resolved.DatabasePathSource),
		},
		DataDir: dataDirectoryInspect{
			Path:   resolved.DataDirectory,
			Exists: resolved.DataDirectoryExists,
		},
		App: appConfigInspect{
			Market: resolved.File.App.Market,
		},
	}
	for _, item := range resolved.File.Providers {
		result.Providers = append(result.Providers, providerInspectFromConfig(item))
	}
	return result
}

func providerInspectFromConfig(config provider.Config) providerInspectItem {
	enabled, ok := config.Bool("enabled")
	if !ok {
		enabled = true
	}
	item := providerInspectItem{
		ID:      config.String("id"),
		Enabled: enabled,
		Auth:    map[string]bool{},
	}
	if auth, ok := config.Lookup("auth"); ok {
		if values, ok := auth.(map[string]any); ok {
			for key, value := range values {
				item.Auth[key] = fmt.Sprint(value) != ""
			}
		}
	}
	if groups, ok := config.Lookup("groups"); ok {
		if values, ok := groups.(map[string]any); ok {
			for key, value := range values {
				group := providerGroupItem{ID: key, Enabled: true}
				if config, ok := value.(map[string]any); ok {
					if enabled, ok := provider.Config(config).Bool("enabled"); ok {
						group.Enabled = enabled
					}
				}
				item.Groups = append(item.Groups, group)
			}
		}
	}
	return item
}

func writeConfigInspect(w io.Writer, output OutputMode, result configInspectResult) error {
	errb := oops.In("cli_output").With("format", output)
	switch output {
	case "", OutputModeTable:
		_, _ = fmt.Fprintln(w, "section\tkey\tvalue\tsource")
		_, _ = fmt.Fprintf(w, "config\tpath\t%s\t%s\n", result.ConfigFile.Path, result.ConfigFile.Source)
		_, _ = fmt.Fprintf(w, "config\texists\t%t\t\n", result.ConfigFile.Exists)
		_, _ = fmt.Fprintf(w, "config\tcreated\t%t\t\n", result.ConfigFile.Created)
		_, _ = fmt.Fprintf(w, "database\tpath\t%s\t%s\n", result.DatabaseFile.Path, result.DatabaseFile.Source)
		_, _ = fmt.Fprintf(w, "data\tpath\t%s\t\n", result.DataDir.Path)
		_, _ = fmt.Fprintf(w, "data\texists\t%t\t\n", result.DataDir.Exists)
		_, _ = fmt.Fprintf(w, "app\tmarket\t%s\tconfig_file\n", result.App.Market)
		for _, provider := range result.Providers {
			_, _ = fmt.Fprintf(w, "provider.%s\tenabled\t%t\tconfig_file\n", provider.ID, provider.Enabled)
			for key, configured := range provider.Auth {
				_, _ = fmt.Fprintf(w, "provider.%s.auth\t%s\tconfigured=%t\tconfig_file\n", provider.ID, key, configured)
			}
			for _, group := range provider.Groups {
				_, _ = fmt.Fprintf(w, "provider.%s.group.%s\tenabled\t%t\tconfig_file\n", provider.ID, group.ID, group.Enabled)
			}
		}
		return nil
	case OutputModeJSON:
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return errb.Wrap(encoder.Encode(result))
	case OutputModeNDJSON:
		return errb.Wrap(json.NewEncoder(w).Encode(result))
	case OutputModeCSV:
		writer := csv.NewWriter(w)
		if err := writer.Write([]string{"section", "key", "value", "source"}); err != nil {
			return errb.Wrapf(err, "write config inspect csv header")
		}
		rows := [][]string{
			{"config", "path", result.ConfigFile.Path, result.ConfigFile.Source},
			{"config", "exists", fmt.Sprint(result.ConfigFile.Exists), ""},
			{"config", "created", fmt.Sprint(result.ConfigFile.Created), ""},
			{"database", "path", result.DatabaseFile.Path, result.DatabaseFile.Source},
			{"data", "path", result.DataDir.Path, ""},
			{"data", "exists", fmt.Sprint(result.DataDir.Exists), ""},
			{"app", "market", result.App.Market, "config_file"},
		}
		for _, row := range rows {
			if err := writer.Write(row); err != nil {
				return errb.Wrapf(err, "write config inspect csv row")
			}
		}
		writer.Flush()
		return errb.Wrap(writer.Error())
	default:
		return errb.Errorf("unsupported output format: %s", output)
	}
}

func writeConfigSetResult(w io.Writer, output OutputMode, result configSetResult) error {
	errb := oops.In("cli_output").With("format", output)
	switch output {
	case "", OutputModeTable:
		_, _ = fmt.Fprintln(w, "config_file\tsetting\tvalue")
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", result.ConfigFile, result.Setting, result.Value)
		return nil
	case OutputModeJSON:
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return errb.Wrap(encoder.Encode(result))
	case OutputModeNDJSON:
		return errb.Wrap(json.NewEncoder(w).Encode(result))
	case OutputModeCSV:
		writer := csv.NewWriter(w)
		if err := writer.Write([]string{"config_file", "setting", "value"}); err != nil {
			return errb.Wrapf(err, "write config set csv header")
		}
		if err := writer.Write([]string{result.ConfigFile, result.Setting, result.Value}); err != nil {
			return errb.Wrapf(err, "write config set csv row")
		}
		writer.Flush()
		return errb.Wrap(writer.Error())
	default:
		return errb.Errorf("unsupported output format: %s", output)
	}
}

func maskedConfigSetValue(path string, value string) string {
	if isSecretConfigPath(path) && value != "" {
		return "<configured>"
	}
	return value
}

func isSecretConfigPath(path string) bool {
	return path == "providers.datago.auth.service_key"
}
