# Shell Completion Architecture

## 목적

이 문서는 `mwosa` CLI 의 shell completion 설계를 정의한다.

`mwosa` 는 Cobra 를 사용하므로 command 이름, flag 이름, 기본 help 는 Cobra command tree 에서 자동으로 completion 후보를 만들 수 있다. 하지만 provider 이름, market, symbol, portfolio 이름처럼 실행 환경과 로컬 데이터에 따라 달라지는 값은 `mwosa` 가 명시적으로 completion source 를 제공해야 한다.

목표는 다음과 같다.

- Bash, Zsh, Fish, PowerShell 에서 같은 명령 체계를 completion 으로 탐색할 수 있게 한다.
- completion 실행 중 stdout 에 일반 로그나 progress 를 섞지 않는다.
- provider API 호출처럼 느리거나 비용이 있는 작업을 Tab 입력마다 수행하지 않는다.
- 도메인별 command package 가 자기 argument completion 을 소유하게 한다.
- 패키지 설치 시 shell completion 파일을 표준 경로에 배치할 수 있게 한다.

## 사용자 표면

사용자는 아래 명령으로 shell completion script 를 출력한다.

```text
mwosa completion bash
mwosa completion zsh
mwosa completion fish
mwosa completion powershell
```

`completion` 명령은 script 를 stdout 으로만 출력한다. 사용자의 shell 설정 파일을 직접 수정하지 않는다.

일회성 로딩 예:

```bash
source <(mwosa completion bash)
```

사용자 단위 Bash 설치 예:

```bash
mkdir -p ~/.local/share/bash-completion/completions
mwosa completion bash > ~/.local/share/bash-completion/completions/mwosa
```

사용자 단위 Zsh 설치 예:

```zsh
mkdir -p ~/.zsh/completions
mwosa completion zsh > ~/.zsh/completions/_mwosa
```

패키지 배포에서는 실행 시점에 shell 설정을 바꾸기보다, 빌드 또는 설치 단계에서 생성한 completion script 를 각 shell 의 표준 completion 디렉터리에 배치한다.

## 동작 원리

Cobra 의 shell completion 은 크게 두 단계로 나뉜다.

```text
user presses Tab
  -> shell completion function
  -> generated mwosa completion script
  -> mwosa __complete <current command line>
  -> Cobra command tree lookup
  -> command/flag/static/dynamic completion function
  -> candidates + directive
  -> shell renders candidates
```

중요한 점은 shell script 가 모든 후보를 스스로 계산하지 않는다는 것이다. script 는 현재 입력 상태를 `mwosa __complete` 로 넘기고, 실제 후보 계산은 실행 파일 내부 Cobra command tree 와 completion function 이 담당한다.

따라서 `mwosa completion bash` 는 설치용 script 를 출력하는 명령이고, `mwosa __complete ...` 는 Cobra 가 내부적으로 사용하는 hidden command 로 본다. 사용자가 직접 호출하는 public command 로 문서화하지 않는다.

## 설계 결정

### 1. Cobra 기본 completion command 를 우선 사용한다

Cobra 는 `bash`, `zsh`, `fish`, `powershell` completion script 생성을 지원한다. `mwosa` 는 별도 shell script template 을 직접 관리하지 않고 Cobra 가 생성한 script 를 사용한다.

직접 구현이 필요한 경우에도 public surface 는 유지한다.

```text
mwosa completion <shell>
```

직접 구현할 때도 command 는 다음 원칙을 지킨다.

- 지원 shell 이 아니면 명시적인 에러를 반환한다.
- script 는 stdout 으로만 출력한다.
- 진단 메시지는 stderr 로만 출력한다.
- 설치까지 대신 수행하는 `--install` 옵션은 초기 범위에 넣지 않는다.

### 2. static completion 은 command 정의 근처에 둔다

command 이름과 flag 이름은 Cobra 가 command tree 에서 자동으로 completion 한다.

고정된 flag value 는 해당 flag 를 선언하는 위치에서 함께 등록한다.

예:

```go
cmd.RegisterFlagCompletionFunc("output", func(
	cmd *cobra.Command,
	args []string,
	toComplete string,
) ([]string, cobra.ShellCompDirective) {
	return []string{
		"table\tHuman-readable table",
		"json\tMachine-readable JSON",
		"ndjson\tNewline-delimited JSON",
		"csv\tComma-separated values",
	}, cobra.ShellCompDirectiveNoFileComp
})
```

초기 static completion 후보:

- `--output`: `table`, `json`, `ndjson`, `csv`
- `completion`: `bash`, `zsh`, `fish`, `powershell`
- `--market`: `krx`, `nasdaq`, `nyse`, `amex`
- `--currency`: `KRW`, `USD`
- `--provider`: registry 에 등록된 provider name

### 3. dynamic completion 은 local source 만 사용한다

Tab 입력은 매우 자주 발생하므로 completion 함수가 외부 API 를 호출하면 안 된다.

동적 후보는 다음 source 에서만 읽는다.

| 후보 종류 | source | network 사용 |
| --- | --- | --- |
| provider name | provider registry metadata | 안 함 |
| market/exchange | static catalog 또는 local index | 안 함 |
| symbol/instrument | SurrealDB index 또는 local canonical files | 안 함 |
| portfolio/universe/strategy | local user data | 안 함 |
| config path | shell file completion directive | shell 에 위임 |

provider API 검색이 필요한 경우에는 completion 이 아니라 명시적인 명령으로 처리한다.

```text
mwosa search instruments <query>
mwosa sync instruments --market krx
```

즉, completion 은 "이미 알고 있는 것"을 빠르게 보여주고, `search` 와 `sync` 는 "아직 모르는 것"을 가져오는 역할을 맡는다.

### 4. 도메인 package 가 자기 argument completion 을 소유한다

`internal/cli` 는 root command, 공통 flag, completion command 를 관리한다.

도메인별 argument completion 은 해당 command package 가 관리한다.

예상 위치:

```text
internal/
  cli/
    root.go
    flags.go
    completion.go

  command/
    instrument/
      routes.go
      completion.go

    provider/
      routes.go
      completion.go

    portfolio/
      routes.go
      completion.go
```

이렇게 나누면 `instrument` package 는 symbol 검색 규칙을, `portfolio` package 는 portfolio name 검색 규칙을 스스로 소유한다. `internal/cli` 는 각 도메인의 후보 계산 세부사항을 모른다.

## Completion source contract

동적 completion source 는 command 에 직접 storage/index 구현체를 노출하지 않고 작은 read-only interface 로 연결한다.

예:

```go
type InstrumentCompletionSource interface {
	CompleteSymbols(ctx context.Context, prefix string, limit int) ([]InstrumentCompletion, error)
}

type InstrumentCompletion struct {
	Symbol      string
	Name        string
	Market      string
	AssetType   string
	Provider    string
}
```

Cobra adapter 는 이 source 를 shell completion protocol 에 맞게 변환한다.

```go
func CompleteInstrumentArgs(source InstrumentCompletionSource) cobra.CompletionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		ctx := cmd.Context()
		items, err := source.CompleteSymbols(ctx, toComplete, 50)
		if err != nil {
			cobra.CompErrorln(err.Error())
			return nil, cobra.ShellCompDirectiveError
		}

		completions := make([]string, 0, len(items))
		for _, item := range items {
			desc := item.Name
			if item.Market != "" {
				desc = desc + " · " + item.Market
			}
			completions = append(completions, cobra.CompletionWithDesc(item.Symbol, desc))
		}

		return completions, cobra.ShellCompDirectiveNoFileComp
	}
}
```

계약:

- `prefix` 는 shell 이 넘긴 현재 입력 조각이다.
- source 는 prefix 기반 후보만 반환한다.
- 기본 limit 은 작게 유지한다. 초기값은 `50` 개를 권장한다.
- source 는 read-only 여야 한다.
- source 는 provider API 를 호출하지 않는다.
- source error 는 빈 후보로 숨기지 않고 `ShellCompDirectiveError` 로 드러낸다.

## Directive 사용 규칙

Cobra completion function 은 후보 목록과 함께 shell 에게 후속 동작을 알려주는 directive 를 반환한다.

`mwosa` 에서 사용하는 기본 규칙:

| 상황 | directive |
| --- | --- |
| symbol, provider, market, portfolio 이름 | `ShellCompDirectiveNoFileComp` |
| path 를 받아야 하는 flag | `MarkFlagFilename` 또는 `ShellCompDirectiveFilterFileExt` |
| 디렉터리만 받아야 하는 flag | `ShellCompDirectiveFilterDirs` |
| `--flag=` 형태에서 공백을 붙이면 안 되는 경우 | `ShellCompDirectiveNoSpace` |
| source error 로 completion 을 신뢰할 수 없는 경우 | `ShellCompDirectiveError` |

파일 경로를 받는 flag 는 shell 의 file completion 을 활용한다.

예:

```go
cmd.MarkFlagFilename("config", "yaml", "yml", "json", "toml")
cmd.MarkFlagFilename("input", "json", "ndjson", "csv")
```

symbol, provider, market 처럼 파일 경로가 아닌 값은 `ShellCompDirectiveNoFileComp` 를 반환해서 shell 이 현재 디렉터리 파일명을 섞지 않게 한다.

## 명령별 completion 초안

### 공통 flag

| 입력 위치 | 후보 |
| --- | --- |
| `--output <TAB>` | `table`, `json`, `ndjson`, `csv` |
| `--provider <TAB>` | 등록된 provider |
| `--prefer-provider <TAB>` | 등록된 provider |
| `--market <TAB>` | 지원 market |
| `--currency <TAB>` | 지원 currency |
| `--from`, `--to`, `--as-of` | completion 없음 |
| `--config <TAB>` | config 확장자 file completion |

### `inspect`

| 명령 | completion |
| --- | --- |
| `mwosa inspect <TAB>` | resource 또는 symbol 후보 |
| `mwosa inspect instrument <TAB>` | local symbol 후보 |
| `mwosa inspect provider <TAB>` | provider 후보 |
| `mwosa inspect market <TAB>` | market 후보 |
| `mwosa inspect portfolio <TAB>` | local portfolio 후보 |
| `mwosa inspect strategy <TAB>` | local strategy 후보 |
| `mwosa inspect tool <TAB>` | agent tool 후보 |
| `mwosa inspect schema <TAB>` | canonical record type 후보 |

`mwosa inspect <symbol...>` 축약형이 있으므로 첫 번째 인자 completion 은 resource 이름과 symbol 후보가 섞일 수 있다. 이때 resource 이름을 먼저 보여주고, symbol 후보는 local index 에서 prefix 가 있을 때만 보여준다.

예:

```text
mwosa inspect p<TAB>
portfolio    Inspect a portfolio
provider     Inspect a provider
005930       Samsung Electronics · krx
```

### `list`

`list` 는 대부분 resource 이름만 completion 한다.

```text
providers
provider-capabilities
markets
exchanges
asset-types
universes
portfolios
strategies
trades
alerts
tools
```

### `search`

`search` 는 검색 query 자체를 completion 하지 않는다.

첫 번째 resource 만 completion 한다.

```text
instruments
news
filings
indicators
```

검색어는 사용자의 자유 입력이므로 file completion 도 비활성화한다.

### `get`

| 명령 | completion |
| --- | --- |
| `mwosa get quote <TAB>` | local symbol 후보 |
| `mwosa get candles <TAB>` | local symbol 후보 |
| `mwosa get fundamentals <TAB>` | local symbol 후보 |
| `mwosa get provider-raw <TAB>` | provider 후보 |

### `record`

`record` 는 사용자가 새 값을 입력하는 경우가 많으므로 completion 을 과하게 제공하지 않는다.

| 명령 | completion |
| --- | --- |
| `mwosa record trade --symbol <TAB>` | local symbol 후보 |
| `mwosa record trade --portfolio <TAB>` | local portfolio 후보 |
| `mwosa record note --trade <TAB>` | local trade id 후보 |

## 성능과 안정성

completion 은 일반 명령보다 더 보수적으로 동작한다.

- 기본적으로 network 를 사용하지 않는다.
- index 가 비어 있으면 빈 후보를 반환한다.
- index 손상이나 query error 는 `ShellCompDirectiveError` 로 알린다.
- stdout 에 completion 후보 외의 텍스트를 쓰지 않는다.
- 진단은 `cobra.CompErrorln` 또는 stderr 를 사용한다.
- completion source 는 read-only 로 유지한다.
- 한 번의 completion 요청에서 너무 많은 후보를 반환하지 않는다.

completion 품질을 높이기 위한 데이터 갱신은 별도 명령으로 수행한다.

```text
mwosa sync instruments --market krx
mwosa sync provider-metadata
```

## 패키징 전략

### Homebrew

Homebrew formula 에서는 설치 후 script 를 생성해 completion 디렉터리에 배치한다.

예상 흐름:

```ruby
generate_completions_from_executable(bin/"mwosa", "completion")
```

### Debian / Ubuntu

패키지 빌드 시 Bash completion 파일을 생성해서 아래 경로에 포함한다.

```text
/usr/share/bash-completion/completions/mwosa
```

Zsh completion 은 아래 경로를 사용한다.

```text
/usr/share/zsh/vendor-completions/_mwosa
```

### macOS 수동 설치

Homebrew 를 쓰지 않는 사용자는 다음 경로를 사용한다.

```text
$(brew --prefix)/etc/bash_completion.d/mwosa
$(brew --prefix)/share/zsh/site-functions/_mwosa
```

## 구현 순서

1. root command 에 Cobra completion command 를 노출한다.
2. 공통 flag completion 을 `internal/cli` 에 등록한다.
3. provider, market, output format 같은 static/local 후보를 연결한다.
4. `instrument`, `portfolio`, `strategy` 순서로 도메인별 argument completion 을 추가한다.
5. completion source 가 network 를 사용하지 않는지 테스트한다.
6. 패키징 문서에 shell 별 설치 경로를 추가한다.

## 테스트 관점

completion 은 일반 CLI 테스트와 별도로 protocol 출력을 확인한다.

예:

```bash
mwosa __complete completion ""
mwosa __complete inspect ""
mwosa __complete inspect instrument 00
mwosa __complete get quote --output ""
```

테스트에서 확인할 것:

- 마지막 줄에 directive 가 포함된다.
- stdout 에 후보와 directive 외 텍스트가 섞이지 않는다.
- symbol completion 은 파일명을 섞지 않는다.
- provider registry error 는 성공처럼 숨겨지지 않는다.
- `--output` 은 고정 후보만 반환한다.
- `--config` 는 파일 completion 으로 위임된다.

## 관련 문서

- `docs/architectures/tech-stack/README.md`
- `docs/architectures/directory/README.md`
- `docs/architectures/layers/README.md`
- `README.md`
