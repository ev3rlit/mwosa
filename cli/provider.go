package cli

import (
	"fmt"
	"strings"

	appconfig "github.com/ev3rlit/mwosa/app/config"
	"github.com/ev3rlit/mwosa/providers/builtin"
	provider "github.com/ev3rlit/mwosa/providers/core"
	"github.com/samber/oops"
	"github.com/spf13/cobra"
)

type providerAddResult struct {
	ConfigFile string                   `json:"config_file"`
	Provider   string                   `json:"provider"`
	Enabled    bool                     `json:"enabled"`
	Fields     []providerAddFieldResult `json:"fields"`
}

type providerAddFieldResult struct {
	Path       string `json:"path"`
	Configured bool   `json:"configured"`
	Secret     bool   `json:"secret"`
}

type providerDoctorResult struct {
	ConfigFile string                   `json:"config_file"`
	Providers  []providerDoctorProvider `json:"providers"`
}

type providerDoctorProvider struct {
	ID      string                `json:"id"`
	Enabled bool                  `json:"enabled"`
	Status  string                `json:"status"`
	Fields  []providerDoctorField `json:"fields"`
	Issues  []providerDoctorIssue `json:"issues"`
}

type providerDoctorField struct {
	Path       string `json:"path"`
	Required   bool   `json:"required"`
	Secret     bool   `json:"secret"`
	Configured bool   `json:"configured"`
	Source     string `json:"source,omitempty"`
}

type providerDoctorIssue struct {
	Severity string `json:"severity"`
	Path     string `json:"path,omitempty"`
	Message  string `json:"message"`
}

func newProviderCommand(opts *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "provider",
		Short: "Manage provider config and diagnostics",
	}
	cmd.AddCommand(newProviderAddCommand(opts))
	cmd.AddCommand(newProviderDoctorCommand(opts))
	return cmd
}

func newProviderAddCommand(opts *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add or update a provider config",
	}
	for _, builder := range builtin.Builders() {
		cmd.AddCommand(newProviderAddProviderCommand(opts, builder))
	}
	return cmd
}

func newProviderAddProviderCommand(opts *Options, builder provider.ProviderBuilder) *cobra.Command {
	spec := builder.ConfigSpec()
	values := make(map[string]*string, len(spec.Fields))
	cmd := &cobra.Command{
		Use:   string(builder.ID()),
		Short: fmt.Sprintf("Add or update %s provider config", builder.ID()),
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			updates := map[string]string{
				providerSettingPath(builder.ID(), "enabled"): "true",
			}
			fields := make([]providerAddFieldResult, 0, len(spec.Fields))
			for _, field := range spec.Fields {
				rawValue := strings.TrimSpace(*values[field.Path])
				flagChanged := cmd.Flags().Changed(field.Flag)
				if field.Required && rawValue == "" {
					return oops.In("cli").
						With("provider", builder.ID(), "setting", field.Path, "flag", field.Flag).
						Errorf("provider %s requires --%s", builder.ID(), field.Flag)
				}
				if rawValue != "" || flagChanged {
					updates[providerSettingPath(builder.ID(), field.Path)] = rawValue
				}
				fields = append(fields, providerAddFieldResult{
					Path:       field.Path,
					Configured: rawValue != "",
					Secret:     field.Secret,
				})
			}
			resolved, err := appconfig.SetValues(appconfig.Options{
				ConfigPath:       opts.Config,
				Market:           opts.Market,
				ProviderDefaults: providerDefaults(),
			}, updates)
			if err != nil {
				return oops.In("cli").With("provider", builder.ID()).Wrapf(err, "add provider config")
			}
			opts.Config = resolved.ConfigPath
			opts.Database = resolved.DatabasePath
			opts.ProviderConfig = resolved.ProviderConfig
			opts.ConfigState = resolved
			opts.configLoaded = true
			return writeConfigOutput(cmd.OutOrStdout(), providerAddResult{
				ConfigFile: resolved.ConfigPath,
				Provider:   string(builder.ID()),
				Enabled:    true,
				Fields:     fields,
			})
		},
	}
	for _, field := range spec.Fields {
		field := field
		values[field.Path] = new(string)
		cmd.Flags().StringVar(values[field.Path], field.Flag, "", field.Description)
	}
	return cmd
}

func newProviderDoctorCommand(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor [provider]",
		Short: "Diagnose provider config",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := loadConfig(opts); err != nil {
				return err
			}
			builders := builtin.Builders()
			if len(args) == 1 {
				var ok bool
				builders, ok = selectProviderBuilder(builders, provider.ProviderID(args[0]))
				if !ok {
					return oops.In("cli").With("provider", args[0]).Errorf("unknown provider: %s", args[0])
				}
			}
			result := providerDoctorResult{
				ConfigFile: opts.ConfigState.ConfigPath,
				Providers:  make([]providerDoctorProvider, 0, len(builders)),
			}
			for _, builder := range builders {
				result.Providers = append(result.Providers, doctorProvider(builder, opts.ProviderConfig))
			}
			return writeConfigOutput(cmd.OutOrStdout(), result)
		},
	}
}

func selectProviderBuilder(builders []provider.ProviderBuilder, id provider.ProviderID) ([]provider.ProviderBuilder, bool) {
	for _, builder := range builders {
		if builder.ID() == id {
			return []provider.ProviderBuilder{builder}, true
		}
	}
	return nil, false
}

func doctorProvider(builder provider.ProviderBuilder, config provider.Config) providerDoctorProvider {
	spec := builder.ConfigSpec()
	result := providerDoctorProvider{
		ID:      string(builder.ID()),
		Enabled: providerEnabled(config, builder.ID()),
		Status:  "ok",
		Fields:  make([]providerDoctorField, 0, len(spec.Fields)),
		Issues:  []providerDoctorIssue{},
	}
	if !result.Enabled {
		result.Status = "disabled"
		result.Issues = append(result.Issues, providerDoctorIssue{
			Severity: "warning",
			Path:     providerSettingPath(builder.ID(), "enabled"),
			Message:  "provider is disabled",
		})
	}
	for _, field := range spec.Fields {
		value, source := providerFieldValue(config, builder.ID(), field)
		configured := strings.TrimSpace(value) != ""
		result.Fields = append(result.Fields, providerDoctorField{
			Path:       field.Path,
			Required:   field.Required,
			Secret:     field.Secret,
			Configured: configured,
			Source:     source,
		})
		if result.Enabled && field.Required && !configured {
			result.Status = "error"
			result.Issues = append(result.Issues, providerDoctorIssue{
				Severity: "error",
				Path:     providerSettingPath(builder.ID(), field.Path),
				Message:  fmt.Sprintf("required provider config is missing; run `mwosa provider add %s --%s VALUE`", builder.ID(), field.Flag),
			})
		}
	}
	if result.Enabled && result.Status == "ok" {
		if _, err := builder.Build(config); err != nil {
			result.Status = "error"
			result.Issues = append(result.Issues, providerDoctorIssue{
				Severity: "error",
				Message:  err.Error(),
			})
		}
	}
	return result
}

func providerEnabled(config provider.Config, id provider.ProviderID) bool {
	enabled, ok := config.Bool("providers", string(id), "enabled")
	return !ok || enabled
}

func providerFieldValue(config provider.Config, id provider.ProviderID, field provider.ConfigField) (string, string) {
	path := append([]string{"providers", string(id)}, splitDotPath(field.Path)...)
	if value := strings.TrimSpace(config.String(path...)); value != "" {
		return value, "config_file"
	}
	for _, env := range field.Env {
		if value := strings.TrimSpace(config.Env(env)); value != "" {
			return value, "env:" + env
		}
	}
	return "", ""
}

func providerSettingPath(id provider.ProviderID, path string) string {
	return "providers." + string(id) + "." + path
}

func splitDotPath(path string) []string {
	raw := strings.Split(path, ".")
	parts := make([]string, 0, len(raw))
	for _, part := range raw {
		part = strings.TrimSpace(part)
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}
