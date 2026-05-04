# Cobra Completion Guide

## 핵심 답변

Bash completion 을 지속적으로 관리한다고 해서 모든 명령어마다 shell script 를 직접 작성하지는 않는다.

Cobra 에서는 command tree 자체가 completion 의 기본 source 다. 명령 이름, 하위 명령 이름, flag 이름은 Cobra 가 자동으로 후보로 만든다. 개발자가 직접 관리해야 하는 부분은 command tree 만으로 알 수 없는 값이다.

예를 들면 다음 값은 명시적으로 completion 을 붙인다.

- `mwosa completion <shell>` 의 `bash`, `zsh`, `fish`, `powershell`
- `--output` 의 `table`, `json`, `ndjson`, `csv`
- `--provider` 의 등록된 provider id
- `--market` 의 지원 market id
- `--security-type` 의 `etf`, `etn`, `elw`
- symbol, strategy id, portfolio name 처럼 local storage 에서 읽어야 하는 argument

반대로 아래 항목은 보통 따로 관리하지 않는다.

- command 이름: `get`, `ensure`, `sync`, `backfill`
- subcommand 이름: `daily`, `config`, `provider`
- flag 이름: `--config`, `--output`, `--from`, `--to`
- 자유 입력값: 검색어, 새 alias, 새 strategy 이름

즉, 새 명령을 추가할 때마다 completion 코드를 기계적으로 추가하는 것이 아니라, 그 명령에 "값 후보"가 필요한지 판단한다.

## Cobra 동작 흐름

사용자가 Tab 을 누르면 Bash completion script 가 모든 후보를 계산하는 것이 아니다.

```text
user presses Tab
  -> generated bash completion function
  -> mwosa __complete <current words>
  -> Cobra command tree lookup
  -> command / flag / argument completion function
  -> candidates + directive
  -> shell renders candidates
```

`mwosa completion bash` 는 shell 에 설치할 script 를 출력하는 명령이다. 실제 후보 계산은 이후 Tab 을 누를 때마다 `mwosa __complete ...` 로 실행되는 Cobra command tree 가 담당한다.

그래서 관리 단위는 generated script 가 아니라 Go 코드의 command tree 다.

## 기본 구조

root command 에 public `completion` 명령을 붙인다.

```go
func NewRootCommand(build BuildInfo) *cobra.Command {
	opts := Options{
		Output: DefaultOutputMode,
		Market: string(provider.MarketKRX),
	}

	cmd := &cobra.Command{
		Use:           "mwosa",
		Short:         "Investment research CLI for provider-backed market data",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			if skipConfigLoadForCompletion(cmd) {
				return nil
			}
			return loadConfig(&opts)
		},
	}

	addPersistentFlags(cmd, &opts)
	registerRootCompletions(cmd)

	cmd.AddCommand(newCompletionCommand())
	cmd.AddCommand(newVersionCommand(build))
	cmd.AddCommand(newGetCommand(&opts))

	return cmd
}
```

completion 실행 중에는 일반 명령처럼 config 생성, provider 초기화, network 호출을 하지 않는다. completion 은 자주 실행되므로 빠르고 read-only 여야 한다.

## Script 생성 명령

shell script 생성은 Cobra generator 에 맡긴다.

```go
func newCompletionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion <shell>",
		Short: "Generate shell completion script",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 1 {
				return oops.In("cli").With("args", len(args)).New("completion requires one shell argument")
			}
			if !completionShellSupported(args[0]) {
				return oops.In("cli").With("shell", args[0]).Errorf("unsupported completion shell: %s", args[0])
			}
			return nil
		},
		ValidArgsFunction: cobra.FixedCompletions(
			completionShellChoices(),
			cobra.ShellCompDirectiveNoFileComp,
		),
		RunE: func(cmd *cobra.Command, args []string) error {
			return generateCompletion(cmd, args[0])
		},
	}
	return cmd
}
```

지원 shell 별 generator 는 다음처럼 분기한다.

```go
func generateCompletion(cmd *cobra.Command, shell string) error {
	root := cmd.Root()
	out := cmd.OutOrStdout()

	switch shell {
	case "bash":
		return root.GenBashCompletionV2(out, true)
	case "zsh":
		return root.GenZshCompletion(out)
	case "fish":
		return root.GenFishCompletion(out, true)
	case "powershell":
		return root.GenPowerShellCompletionWithDesc(out)
	default:
		return oops.In("cli").With("shell", shell).Errorf("unsupported completion shell: %s", shell)
	}
}
```

사용자는 아래처럼 설치한다.

```bash
source <(mwosa completion bash)
```

또는 사용자 completion 디렉터리에 저장한다.

```bash
mkdir -p ~/.local/share/bash-completion/completions
mwosa completion bash > ~/.local/share/bash-completion/completions/mwosa
```

## Flag 값 completion

flag 이름 자체는 Cobra 가 안다. 직접 등록해야 하는 것은 flag 의 값 후보 다.

예를 들어 `--output` 은 임의 문자열이 아니라 정해진 format 만 받는다.

```go
func registerRootCompletions(cmd *cobra.Command) {
	mustRegisterFlagCompletion(cmd, "output", completeOutputModes)
	mustRegisterFlagCompletion(cmd, "provider", completeProviderIDs)
	mustRegisterFlagCompletion(cmd, "market", completeMarkets)
}

func completeOutputModes(_ *cobra.Command, _ []string, _ string) ([]cobra.Completion, cobra.ShellCompDirective) {
	return []cobra.Completion{
		cobra.CompletionWithDesc("table", "Human-readable table"),
		cobra.CompletionWithDesc("json", "Machine-readable JSON"),
		cobra.CompletionWithDesc("ndjson", "Newline-delimited JSON"),
		cobra.CompletionWithDesc("csv", "Comma-separated values"),
	}, cobra.ShellCompDirectiveNoFileComp
}
```

`ShellCompDirectiveNoFileComp` 는 shell 이 현재 디렉터리의 파일명을 후보에 섞지 않게 한다.

## 파일 경로 flag

파일 경로를 받는 flag 는 직접 후보를 만들지 말고 shell file completion 에 맡긴다.

root persistent flag 는 `MarkPersistentFlagFilename` 을 쓴다.

```go
func registerRootCompletions(cmd *cobra.Command) {
	mustMarkPersistentFlagFilename(cmd, "config", "json")
	mustMarkPersistentFlagFilename(cmd, "database", "db", "sqlite", "sqlite3")
}
```

local flag 는 `MarkFlagFilename` 을 쓴다.

```go
func addStrategySourceFlags(cmd *cobra.Command, flags *strategySourceFlags) {
	cmd.Flags().StringVar(&flags.JQFile, "jq-file", flags.JQFile, "path to a jq query file")
	mustMarkFlagFilename(cmd, "jq-file", "jq")
}
```

## Argument completion

argument 후보는 해당 command 를 만드는 함수 근처에 둔다. 그래야 command 의 의미와 completion source 가 같이 보인다.

```go
func newGetDailyCommand(opts *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "daily <symbol>",
		Short: "Read stored daily bars for a symbol",
		Args:  cobra.ExactArgs(1),
		RunE:  runGetDaily(opts),
		ValidArgsFunction: completeLocalSymbols(opts),
	}
	return cmd
}
```

동적 completion source 는 local read-only interface 로 작게 둔다.

```go
type SymbolCompletionSource interface {
	CompleteSymbols(ctx context.Context, prefix string, limit int) ([]SymbolCompletion, error)
}

type SymbolCompletion struct {
	Symbol string
	Name   string
	Market string
}
```

Cobra adapter 는 source 결과를 completion protocol 로 변환한다.

```go
func completeSymbols(source SymbolCompletionSource) cobra.CompletionFunc {
	return func(cmd *cobra.Command, _ []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
		items, err := source.CompleteSymbols(cmd.Context(), toComplete, 50)
		if err != nil {
			cobra.CompErrorln(err.Error())
			return nil, cobra.ShellCompDirectiveError
		}

		completions := make([]cobra.Completion, 0, len(items))
		for _, item := range items {
			desc := item.Name
			if item.Market != "" {
				desc = desc + " " + item.Market
			}
			completions = append(completions, cobra.CompletionWithDesc(item.Symbol, desc))
		}

		return completions, cobra.ShellCompDirectiveNoFileComp
	}
}
```

이 source 는 provider API 를 호출하지 않는다. local SQLite, local index, in-memory registry 같은 이미 알고 있는 데이터만 읽는다.

## Subcommand 후보

subcommand 후보는 보통 아무 코드를 추가하지 않아도 된다.

```go
func newGetCommand(opts *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Read source-like data from local storage",
	}
	cmd.AddCommand(newGetDailyCommand(opts))
	return cmd
}
```

이렇게만 해도 `mwosa get <TAB>` 에서 `daily` 는 Cobra 가 자동으로 보여준다.

명시적인 `ValidArgsFunction` 이 필요한 경우는 subcommand 가 아니라 resource 이름이나 target 값이 argument 로 들어올 때다.

## 관리 기준

새 command 를 추가할 때는 아래 순서로 판단한다.

1. subcommand 만 추가했는가? 그러면 별도 completion 코드가 없어도 된다.
2. 새 flag 가 enum 값을 받는가? `RegisterFlagCompletionFunc` 를 붙인다.
3. 새 flag 가 파일 경로를 받는가? `MarkFlagFilename` 또는 `MarkPersistentFlagFilename` 를 붙인다.
4. argument 가 local data id 를 받는가? `ValidArgsFunction` 을 붙인다.
5. argument 가 자유 입력인가? completion 을 붙이지 않거나 `ShellCompDirectiveNoFileComp` 만 검토한다.
6. 후보를 얻기 위해 network 가 필요한가? completion 에 넣지 말고 `search`, `sync`, `ensure` 같은 명령으로 분리한다.

## 테스트 샘플

completion 은 일반 command 실행 테스트와 별도로 protocol 출력을 확인한다.

```go
func TestCompletionProtocolCompletesOutputFlagValues(t *testing.T) {
	cmd := NewRootCommand(BuildInfo{})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{cobra.ShellCompRequestCmd, "version", "--output", ""})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute output flag completion: %v\n%s", err, out.String())
	}

	got := out.String()
	for _, want := range []string{
		"table\tHuman-readable table",
		"json\tMachine-readable JSON",
		"ndjson\tNewline-delimited JSON",
		"csv\tComma-separated values",
		":4",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("output flag completion missing %q in:\n%s", want, got)
		}
	}
}
```

마지막 줄의 `:4` 는 Cobra directive 다. `4` 는 `ShellCompDirectiveNoFileComp` 를 뜻한다.

Bash script 생성도 별도로 확인한다.

```go
func TestCompletionBashGeneratesScriptWithoutConfigLoad(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	cmd := NewRootCommand(BuildInfo{})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--config", configPath, "completion", "bash"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute completion bash: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "__start_mwosa") {
		t.Fatalf("bash completion script was not generated:\n%s", out.String())
	}
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Fatalf("completion should not create config file, stat error = %v", err)
	}
}
```

## mwosa 에서의 위치

현재 기준은 다음과 같다.

- public completion command: `cli/completion.go`
- root persistent flag completion: `cli/completion.go`
- command-specific flag completion: 해당 command 파일 근처
- argument completion: 해당 command 파일 근처
- dynamic completion source: local storage 또는 registry 를 읽는 작은 read-only interface

completion 후보가 domain 지식을 많이 요구하면 `cli/completion.go` 로 끌어올리지 않는다. 예를 들어 symbol completion 은 `daily`, `instrument`, `screen` 같은 command 쪽에서 source interface 를 받아 처리한다.

## 관련 문서

- `docs/architectures/completion/README.md`
- `docs/development/README.md`
