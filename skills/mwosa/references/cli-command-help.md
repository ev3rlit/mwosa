# mwosa CLI Command Help

Generated from `mwosa`. Use this when you need the complete installed or built CLI command surface instead of relying on source-code assumptions.

## Refresh Command

Run this from the repository root when the CLI changes:

```bash
skills/mwosa/references/generate-cli-command-help.sh

# Or use a freshly built binary:
MWOSA_HELP_COMMAND=./bin/mwosa skills/mwosa/references/generate-cli-command-help.sh

# If running the global skill copy outside the repo:
MWOSA_HELP_REPO_ROOT=/path/to/mwosa skills/mwosa/references/generate-cli-command-help.sh
```

## Captured Help

```text
mwosa v0.1.0
schema dev
commit unknown
built unknown
go go1.25.6
Investment research CLI for provider-backed market data

Usage:
  mwosa [command]

Available Commands:
  backfill    Collect historical data ranges
  completion  Generate the autocompletion script for the specified shell
  config      Manage mwosa config file
  create      Create mwosa resources
  delete      Delete mwosa resources
  ensure      Fetch missing data and store it locally
  get         Read source-like data from local storage
  help        Help about any command
  history     List mwosa execution history
  inspect     Inspect mwosa resources and local state
  list        List mwosa resources
  provider    Manage provider config and diagnostics
  screen      Run screening workflows
  sync        Refresh provider-backed data batches
  update      Update mwosa resources
  version     Print mwosa build information

Flags:
      --config string            config file path
      --database string          local SQLite database path
  -h, --help                     help for mwosa
      --market string            market id (default "krx")
  -o, --output output            output format: table, json, ndjson, csv (default table)
      --prefer-provider string   prefer a provider when multiple candidates match
      --provider string          force a provider by id

Use "mwosa [command] --help" for more information about a command.


### mwosa backfill --help
Collect historical data ranges

Usage:
  mwosa backfill [command]

Available Commands:
  daily       Collect provider daily batches for a date range

Flags:
  -h, --help   help for backfill

Global Flags:
      --config string            config file path
      --database string          local SQLite database path
      --market string            market id (default "krx")
  -o, --output output            output format: table, json, ndjson, csv (default table)
      --prefer-provider string   prefer a provider when multiple candidates match
      --provider string          force a provider by id

Use "mwosa backfill [command] --help" for more information about a command.


### mwosa backfill daily --help
Collect provider daily batches for a date range

Usage:
  mwosa backfill daily [flags]

Flags:
      --from string            start trading date, YYYYMMDD or YYYY-MM-DD
  -h, --help                   help for daily
      --security-type string   security type: etf, etn, elw (default "etf")
      --to string              end trading date, YYYYMMDD or YYYY-MM-DD
      --workers int            number of page fetch workers for range-capable providers (default 1)

Global Flags:
      --config string            config file path
      --database string          local SQLite database path
      --market string            market id (default "krx")
  -o, --output output            output format: table, json, ndjson, csv (default table)
      --prefer-provider string   prefer a provider when multiple candidates match
      --provider string          force a provider by id


### mwosa completion --help
Generate the autocompletion script for mwosa for the specified shell.
See each sub-command's help for details on how to use the generated script.

Usage:
  mwosa completion [command]

Available Commands:
  bash        Generate the autocompletion script for bash
  fish        Generate the autocompletion script for fish
  powershell  Generate the autocompletion script for powershell
  zsh         Generate the autocompletion script for zsh

Flags:
  -h, --help   help for completion

Global Flags:
      --config string            config file path
      --database string          local SQLite database path
      --market string            market id (default "krx")
  -o, --output output            output format: table, json, ndjson, csv (default table)
      --prefer-provider string   prefer a provider when multiple candidates match
      --provider string          force a provider by id

Use "mwosa completion [command] --help" for more information about a command.


### mwosa completion bash --help
Generate the autocompletion script for the bash shell.

This script depends on the 'bash-completion' package.
If it is not installed already, you can install it via your OS's package manager.

To load completions in your current shell session:

	source <(mwosa completion bash)

To load completions for every new session, execute once:

#### Linux:

	mwosa completion bash > /etc/bash_completion.d/mwosa

#### macOS:

	mwosa completion bash > $(brew --prefix)/etc/bash_completion.d/mwosa

You will need to start a new shell for this setup to take effect.

Usage:
  mwosa completion bash

Flags:
  -h, --help              help for bash
      --no-descriptions   disable completion descriptions

Global Flags:
      --config string            config file path
      --database string          local SQLite database path
      --market string            market id (default "krx")
  -o, --output output            output format: table, json, ndjson, csv (default table)
      --prefer-provider string   prefer a provider when multiple candidates match
      --provider string          force a provider by id


### mwosa completion fish --help
Generate the autocompletion script for the fish shell.

To load completions in your current shell session:

	mwosa completion fish | source

To load completions for every new session, execute once:

	mwosa completion fish > ~/.config/fish/completions/mwosa.fish

You will need to start a new shell for this setup to take effect.

Usage:
  mwosa completion fish [flags]

Flags:
  -h, --help              help for fish
      --no-descriptions   disable completion descriptions

Global Flags:
      --config string            config file path
      --database string          local SQLite database path
      --market string            market id (default "krx")
  -o, --output output            output format: table, json, ndjson, csv (default table)
      --prefer-provider string   prefer a provider when multiple candidates match
      --provider string          force a provider by id


### mwosa completion powershell --help
Generate the autocompletion script for powershell.

To load completions in your current shell session:

	mwosa completion powershell | Out-String | Invoke-Expression

To load completions for every new session, add the output of the above command
to your powershell profile.

Usage:
  mwosa completion powershell [flags]

Flags:
  -h, --help              help for powershell
      --no-descriptions   disable completion descriptions

Global Flags:
      --config string            config file path
      --database string          local SQLite database path
      --market string            market id (default "krx")
  -o, --output output            output format: table, json, ndjson, csv (default table)
      --prefer-provider string   prefer a provider when multiple candidates match
      --provider string          force a provider by id


### mwosa completion zsh --help
Generate the autocompletion script for the zsh shell.

If shell completion is not already enabled in your environment you will need
to enable it.  You can execute the following once:

	echo "autoload -U compinit; compinit" >> ~/.zshrc

To load completions in your current shell session:

	source <(mwosa completion zsh)

To load completions for every new session, execute once:

#### Linux:

	mwosa completion zsh > "${fpath[1]}/_mwosa"

#### macOS:

	mwosa completion zsh > $(brew --prefix)/share/zsh/site-functions/_mwosa

You will need to start a new shell for this setup to take effect.

Usage:
  mwosa completion zsh [flags]

Flags:
  -h, --help              help for zsh
      --no-descriptions   disable completion descriptions

Global Flags:
      --config string            config file path
      --database string          local SQLite database path
      --market string            market id (default "krx")
  -o, --output output            output format: table, json, ndjson, csv (default table)
      --prefer-provider string   prefer a provider when multiple candidates match
      --provider string          force a provider by id


### mwosa config --help
Manage mwosa config file

Usage:
  mwosa config [command]

Available Commands:
  set         Set a config value

Flags:
  -h, --help   help for config

Global Flags:
      --config string            config file path
      --database string          local SQLite database path
      --market string            market id (default "krx")
  -o, --output output            output format: table, json, ndjson, csv (default table)
      --prefer-provider string   prefer a provider when multiple candidates match
      --provider string          force a provider by id

Use "mwosa config [command] --help" for more information about a command.


### mwosa config set --help
Set a config value

Usage:
  mwosa config set <path> <value> [flags]

Flags:
  -h, --help   help for set

Global Flags:
      --config string            config file path
      --database string          local SQLite database path
      --market string            market id (default "krx")
  -o, --output output            output format: table, json, ndjson, csv (default table)
      --prefer-provider string   prefer a provider when multiple candidates match
      --provider string          force a provider by id


### mwosa create --help
Create mwosa resources

Usage:
  mwosa create [command]

Available Commands:
  strategy    Create a saved screening strategy

Flags:
  -h, --help   help for create

Global Flags:
      --config string            config file path
      --database string          local SQLite database path
      --market string            market id (default "krx")
  -o, --output output            output format: table, json, ndjson, csv (default table)
      --prefer-provider string   prefer a provider when multiple candidates match
      --provider string          force a provider by id

Use "mwosa create [command] --help" for more information about a command.


### mwosa create strategy --help
Create a saved screening strategy

Usage:
  mwosa create strategy <name> [flags]

Flags:
      --engine string    strategy engine: jq (default "jq")
  -h, --help             help for strategy
      --input string     input dataset name
      --jq string        inline jq query
      --jq-file string   path to a jq query file

Global Flags:
      --config string            config file path
      --database string          local SQLite database path
      --market string            market id (default "krx")
  -o, --output output            output format: table, json, ndjson, csv (default table)
      --prefer-provider string   prefer a provider when multiple candidates match
      --provider string          force a provider by id


### mwosa delete --help
Delete mwosa resources

Usage:
  mwosa delete [command]

Available Commands:
  strategy    Soft delete a saved screening strategy

Flags:
  -h, --help   help for delete

Global Flags:
      --config string            config file path
      --database string          local SQLite database path
      --market string            market id (default "krx")
  -o, --output output            output format: table, json, ndjson, csv (default table)
      --prefer-provider string   prefer a provider when multiple candidates match
      --provider string          force a provider by id

Use "mwosa delete [command] --help" for more information about a command.


### mwosa delete strategy --help
Soft delete a saved screening strategy

Usage:
  mwosa delete strategy <name> [flags]

Flags:
  -h, --help   help for strategy

Global Flags:
      --config string            config file path
      --database string          local SQLite database path
      --market string            market id (default "krx")
  -o, --output output            output format: table, json, ndjson, csv (default table)
      --prefer-provider string   prefer a provider when multiple candidates match
      --provider string          force a provider by id


### mwosa ensure --help
Fetch missing data and store it locally

Usage:
  mwosa ensure [command]

Available Commands:
  daily       Fetch missing daily bars for a symbol and store them locally

Flags:
  -h, --help   help for ensure

Global Flags:
      --config string            config file path
      --database string          local SQLite database path
      --market string            market id (default "krx")
  -o, --output output            output format: table, json, ndjson, csv (default table)
      --prefer-provider string   prefer a provider when multiple candidates match
      --provider string          force a provider by id

Use "mwosa ensure [command] --help" for more information about a command.


### mwosa ensure daily --help
Fetch missing daily bars for a symbol and store them locally

Usage:
  mwosa ensure daily <symbol> [flags]

Flags:
      --as-of string           single trading date, YYYYMMDD or YYYY-MM-DD
      --from string            start trading date, YYYYMMDD or YYYY-MM-DD
  -h, --help                   help for daily
      --security-type string   security type: etf, etn, elw (default "etf")
      --to string              end trading date, YYYYMMDD or YYYY-MM-DD

Global Flags:
      --config string            config file path
      --database string          local SQLite database path
      --market string            market id (default "krx")
  -o, --output output            output format: table, json, ndjson, csv (default table)
      --prefer-provider string   prefer a provider when multiple candidates match
      --provider string          force a provider by id


### mwosa get --help
Read source-like data from local storage

Usage:
  mwosa get [command]

Available Commands:
  daily       Read stored daily bars for a symbol

Flags:
  -h, --help   help for get

Global Flags:
      --config string            config file path
      --database string          local SQLite database path
      --market string            market id (default "krx")
  -o, --output output            output format: table, json, ndjson, csv (default table)
      --prefer-provider string   prefer a provider when multiple candidates match
      --provider string          force a provider by id

Use "mwosa get [command] --help" for more information about a command.


### mwosa get daily --help
Read stored daily bars for a symbol

Usage:
  mwosa get daily <symbol> [flags]

Flags:
      --as-of string           single trading date, YYYYMMDD or YYYY-MM-DD
      --from string            start trading date, YYYYMMDD or YYYY-MM-DD
  -h, --help                   help for daily
      --security-type string   security type: etf, etn, elw (default "etf")
      --to string              end trading date, YYYYMMDD or YYYY-MM-DD

Global Flags:
      --config string            config file path
      --database string          local SQLite database path
      --market string            market id (default "krx")
  -o, --output output            output format: table, json, ndjson, csv (default table)
      --prefer-provider string   prefer a provider when multiple candidates match
      --provider string          force a provider by id


### mwosa help --help
Help provides help for any command in the application.
Simply type mwosa help [path to command] for full details.

Usage:
  mwosa help [command] [flags]

Flags:
  -h, --help   help for help

Global Flags:
      --config string            config file path
      --database string          local SQLite database path
      --market string            market id (default "krx")
  -o, --output output            output format: table, json, ndjson, csv (default table)
      --prefer-provider string   prefer a provider when multiple candidates match
      --provider string          force a provider by id


### mwosa history --help
List mwosa execution history

Usage:
  mwosa history [command]

Available Commands:
  screen      List saved screening runs

Flags:
  -h, --help   help for history

Global Flags:
      --config string            config file path
      --database string          local SQLite database path
      --market string            market id (default "krx")
  -o, --output output            output format: table, json, ndjson, csv (default table)
      --prefer-provider string   prefer a provider when multiple candidates match
      --provider string          force a provider by id

Use "mwosa history [command] --help" for more information about a command.


### mwosa history screen --help
List saved screening runs

Usage:
  mwosa history screen [flags]

Flags:
  -h, --help        help for screen
      --limit int   maximum number of screen runs to list (default 50)

Global Flags:
      --config string            config file path
      --database string          local SQLite database path
      --market string            market id (default "krx")
  -o, --output output            output format: table, json, ndjson, csv (default table)
      --prefer-provider string   prefer a provider when multiple candidates match
      --provider string          force a provider by id


### mwosa inspect --help
Inspect mwosa resources and local state

Usage:
  mwosa inspect [command]

Available Commands:
  config      Inspect resolved config and data paths
  screen      Inspect a saved screening run
  strategy    Inspect a saved screening strategy

Flags:
  -h, --help   help for inspect

Global Flags:
      --config string            config file path
      --database string          local SQLite database path
      --market string            market id (default "krx")
  -o, --output output            output format: table, json, ndjson, csv (default table)
      --prefer-provider string   prefer a provider when multiple candidates match
      --provider string          force a provider by id

Use "mwosa inspect [command] --help" for more information about a command.


### mwosa inspect config --help
Inspect resolved config and data paths

Usage:
  mwosa inspect config [flags]

Flags:
  -h, --help   help for config

Global Flags:
      --config string            config file path
      --database string          local SQLite database path
      --market string            market id (default "krx")
  -o, --output output            output format: table, json, ndjson, csv (default table)
      --prefer-provider string   prefer a provider when multiple candidates match
      --provider string          force a provider by id


### mwosa inspect screen --help
Inspect a saved screening run

Usage:
  mwosa inspect screen <screen-id-or-alias> [flags]

Flags:
  -h, --help   help for screen

Global Flags:
      --config string            config file path
      --database string          local SQLite database path
      --market string            market id (default "krx")
  -o, --output output            output format: table, json, ndjson, csv (default table)
      --prefer-provider string   prefer a provider when multiple candidates match
      --provider string          force a provider by id


### mwosa inspect strategy --help
Inspect a saved screening strategy

Usage:
  mwosa inspect strategy <name> [flags]

Flags:
  -h, --help   help for strategy

Global Flags:
      --config string            config file path
      --database string          local SQLite database path
      --market string            market id (default "krx")
  -o, --output output            output format: table, json, ndjson, csv (default table)
      --prefer-provider string   prefer a provider when multiple candidates match
      --provider string          force a provider by id


### mwosa list --help
List mwosa resources

Usage:
  mwosa list [command]

Available Commands:
  strategies  List saved screening strategies

Flags:
  -h, --help   help for list

Global Flags:
      --config string            config file path
      --database string          local SQLite database path
      --market string            market id (default "krx")
  -o, --output output            output format: table, json, ndjson, csv (default table)
      --prefer-provider string   prefer a provider when multiple candidates match
      --provider string          force a provider by id

Use "mwosa list [command] --help" for more information about a command.


### mwosa list strategies --help
List saved screening strategies

Usage:
  mwosa list strategies [flags]

Flags:
  -h, --help   help for strategies

Global Flags:
      --config string            config file path
      --database string          local SQLite database path
      --market string            market id (default "krx")
  -o, --output output            output format: table, json, ndjson, csv (default table)
      --prefer-provider string   prefer a provider when multiple candidates match
      --provider string          force a provider by id


### mwosa provider --help
Manage provider config and diagnostics

Usage:
  mwosa provider [command]

Available Commands:
  add         Add or update a provider config
  doctor      Diagnose provider config

Flags:
  -h, --help   help for provider

Global Flags:
      --config string            config file path
      --database string          local SQLite database path
      --market string            market id (default "krx")
  -o, --output output            output format: table, json, ndjson, csv (default table)
      --prefer-provider string   prefer a provider when multiple candidates match
      --provider string          force a provider by id

Use "mwosa provider [command] --help" for more information about a command.


### mwosa provider add --help
Add or update a provider config

Usage:
  mwosa provider add [command]

Available Commands:
  datago      Add or update datago provider config

Flags:
  -h, --help   help for add

Global Flags:
      --config string            config file path
      --database string          local SQLite database path
      --market string            market id (default "krx")
  -o, --output output            output format: table, json, ndjson, csv (default table)
      --prefer-provider string   prefer a provider when multiple candidates match
      --provider string          force a provider by id

Use "mwosa provider add [command] --help" for more information about a command.


### mwosa provider add datago --help
Add or update datago provider config

Usage:
  mwosa provider add datago [flags]

Flags:
      --base-url string      override datago API base URL
  -h, --help                 help for datago
      --service-key string   공공데이터포털 service key

Global Flags:
      --config string            config file path
      --database string          local SQLite database path
      --market string            market id (default "krx")
  -o, --output output            output format: table, json, ndjson, csv (default table)
      --prefer-provider string   prefer a provider when multiple candidates match
      --provider string          force a provider by id


### mwosa provider doctor --help
Diagnose provider config

Usage:
  mwosa provider doctor [provider] [flags]

Flags:
  -h, --help   help for doctor

Global Flags:
      --config string            config file path
      --database string          local SQLite database path
      --market string            market id (default "krx")
  -o, --output output            output format: table, json, ndjson, csv (default table)
      --prefer-provider string   prefer a provider when multiple candidates match
      --provider string          force a provider by id


### mwosa screen --help
Run screening workflows

Usage:
  mwosa screen [command]

Available Commands:
  strategy    Run a saved screening strategy

Flags:
  -h, --help   help for screen

Global Flags:
      --config string            config file path
      --database string          local SQLite database path
      --market string            market id (default "krx")
  -o, --output output            output format: table, json, ndjson, csv (default table)
      --prefer-provider string   prefer a provider when multiple candidates match
      --provider string          force a provider by id

Use "mwosa screen [command] --help" for more information about a command.


### mwosa screen strategy --help
Run a saved screening strategy

Usage:
  mwosa screen strategy <name> [flags]

Flags:
      --alias string   optional screen run alias
  -h, --help           help for strategy

Global Flags:
      --config string            config file path
      --database string          local SQLite database path
      --market string            market id (default "krx")
  -o, --output output            output format: table, json, ndjson, csv (default table)
      --prefer-provider string   prefer a provider when multiple candidates match
      --provider string          force a provider by id


### mwosa sync --help
Refresh provider-backed data batches

Usage:
  mwosa sync [command]

Available Commands:
  daily       Collect one provider daily batch for a date

Flags:
  -h, --help   help for sync

Global Flags:
      --config string            config file path
      --database string          local SQLite database path
      --market string            market id (default "krx")
  -o, --output output            output format: table, json, ndjson, csv (default table)
      --prefer-provider string   prefer a provider when multiple candidates match
      --provider string          force a provider by id

Use "mwosa sync [command] --help" for more information about a command.


### mwosa sync daily --help
Collect one provider daily batch for a date

Usage:
  mwosa sync daily [flags]

Flags:
      --as-of string           trading date to collect, YYYYMMDD or YYYY-MM-DD
  -h, --help                   help for daily
      --security-type string   security type: etf, etn, elw (default "etf")

Global Flags:
      --config string            config file path
      --database string          local SQLite database path
      --market string            market id (default "krx")
  -o, --output output            output format: table, json, ndjson, csv (default table)
      --prefer-provider string   prefer a provider when multiple candidates match
      --provider string          force a provider by id


### mwosa update --help
Update mwosa resources

Usage:
  mwosa update [command]

Available Commands:
  strategy    Create a new version of a saved screening strategy

Flags:
  -h, --help   help for update

Global Flags:
      --config string            config file path
      --database string          local SQLite database path
      --market string            market id (default "krx")
  -o, --output output            output format: table, json, ndjson, csv (default table)
      --prefer-provider string   prefer a provider when multiple candidates match
      --provider string          force a provider by id

Use "mwosa update [command] --help" for more information about a command.


### mwosa update strategy --help
Create a new version of a saved screening strategy

Usage:
  mwosa update strategy <name> [flags]

Flags:
  -h, --help             help for strategy
      --jq string        inline jq query
      --jq-file string   path to a jq query file

Global Flags:
      --config string            config file path
      --database string          local SQLite database path
      --market string            market id (default "krx")
  -o, --output output            output format: table, json, ndjson, csv (default table)
      --prefer-provider string   prefer a provider when multiple candidates match
      --provider string          force a provider by id


### mwosa version --help
Print mwosa build information

Usage:
  mwosa version [flags]

Flags:
  -h, --help   help for version

Global Flags:
      --config string            config file path
      --database string          local SQLite database path
      --market string            market id (default "krx")
  -o, --output output            output format: table, json, ndjson, csv (default table)
      --prefer-provider string   prefer a provider when multiple candidates match
      --provider string          force a provider by id
```
