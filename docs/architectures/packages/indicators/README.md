# Indicators Package Architecture

## 목적

이 문서는 `packages/indicators` 가 `mwosa` 안에서 어떤 역할을 맡는지 설명한다.

`packages/indicators` 는 투자 보조지표를 계산하는 코어 라이브러리다. 여기서 말하는 보조지표는 MACD 나 일목균형표에만 한정되지 않는다. 추세, 모멘텀, 변동성, 거래량, 수익률, 리스크, 상대강도처럼 투자 리서치에 필요한 계산 지표를 넓게 다룬다.

이 패키지는 종목을 추천하지 않고, provider 에서 데이터를 가져오지도 않는다. 정렬된 시계열을 입력으로 받아 계산 결과만 돌려준다.

`packages/indicators` 는 순수 계산 라이브러리다.

## 위치

논의 중인 위치는 다음과 같다.

```text
packages/
  indicators/
    go.mod
    series/
    catalog/
    trend/
    momentum/
    volatility/
    volume/
    risk/
```

`packages/indicators` 는 독립 Go module 로 둘 수 있다. root `go.work` 는 CLI module, provider client module, indicators module 을 함께 묶는다.

## 다룰 지표

처음부터 모두 구현하겠다는 뜻은 아니다. 다만 패키지의 범위는 아래 정도까지 열어둔다.

| 범주 | 예시 |
| --- | --- |
| trend | SMA, EMA, WMA, MACD, Ichimoku, moving high/low |
| momentum | RSI, stochastic, rate of change, Williams R |
| volatility | ATR, Bollinger Bands, Donchian Channel, standard deviation |
| volume | OBV, VWAP, MFI, volume moving average |
| return | period return, cumulative return, rolling return |
| risk | drawdown, downside deviation, rolling volatility |
| relative | relative strength, benchmark spread, correlation |

실제로 어떤 지표를 넣을지는 데이터 확보 난이도, 공식의 안정성, 테스트 fixture 준비 가능성, `mwosa` 리서치 흐름에서의 사용 빈도를 보고 정한다.

## 역할

`packages/indicators` 가 맡는 일:

- 보조지표 계산 파라미터를 정의한다.
- 보조지표 목록과 채택 조건을 관리한다.
- 입력 시계열이 계산 가능한 상태인지 확인한다.
- warm-up 구간과 결측 결과를 드러낸다.
- 채택된 보조지표를 계산한다.
- 계산 결과를 안정적인 Go type 으로 반환한다.
- golden test 와 benchmark 를 유지한다.

`packages/indicators` 가 맡지 않는 일:

- provider API 호출
- provider fallback 판단
- canonical record 저장과 조회
- Cobra command, flag, stdin 처리
- table, json, ndjson, csv 렌더링
- 매수, 매도, 추천, 점수화

## 계산 흐름

`mwosa` 에서 보조지표를 계산할 때는 아래 흐름을 따른다.

```text
provider/storage
  -> canonical candle series
  -> service/calc
  -> packages/indicators
  -> indicator result series
  -> presentation
```

service 는 필요한 candle 데이터를 확보하고, canonical record 를 `packages/indicators` 입력 타입으로 바꾼다. `packages/indicators` 는 계산만 한다. 결과를 JSON, CSV, table 로 보여주는 일은 presentation layer 가 맡는다.

service layer 는 보조지표 계산 use case 를 조립하고, `packages/indicators` 는 계산 공식만 소유한다.

## 입력 모델

`packages/indicators` 는 `mwosa` 의 canonical package 를 직접 import 하지 않는다. 계산에 필요한 최소 입력 타입을 자체적으로 가진다.

```go
type Candle struct {
	Time   time.Time
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume float64
}
```

이렇게 하면 provider, storage, canonical schema 가 바뀌어도 계산 패키지의 public API 는 크게 흔들리지 않는다. `mwosa` service layer 는 canonical daily bar 를 indicators candle 로 바꾸는 얇은 adapter 를 둔다.

가격만 필요한 지표는 `[]TimedValue` 처럼 더 작은 입력 타입을 받을 수 있다. OHLCV 가 필요한 지표만 `[]Candle` 을 요구한다. 모든 지표가 같은 입력 구조를 억지로 공유하지 않는다.

## 결과 모델

warm-up 구간처럼 아직 계산값이 없는 시점은 `0` 으로 숨기지 않는다. 결과 type 은 값이 있는지 없는지를 드러낸다.

```go
type Value struct {
	Value float64
	Valid bool
}
```

MACD 처럼 여러 선을 가진 지표는 전용 결과 type 을 가진다.

```go
type MACDPoint struct {
	Time      time.Time
	MACD      Value
	Signal    Value
	Histogram Value
}
```

일목균형표처럼 계산 시점과 차트에 그리는 시점이 다른 지표는 시간 의미를 더 분명히 둔다.

```go
type IchimokuPoint struct {
	SourceTime  time.Time
	PlotTime    time.Time
	Conversion  Value
	Base        Value
	LeadingA    Value
	LeadingB    Value
	Lagging     Value
}
```

`SourceTime` 은 어떤 candle 까지 보고 계산했는지를 뜻한다. `PlotTime` 은 차트에서 값이 놓이는 시점이다. 이 둘을 나눠야 일목균형표의 선행스팬과 후행스팬을 JSON 으로 내보낼 때 헷갈리지 않는다.

## 구현 방식

`packages/indicators` 는 외부 라이브러리를 전제로 하지 않는다. 단순한 함수형 계산 구조는 참고할 수 있지만, `mwosa` 의 public type, 결측 표현, 시간 정렬 규칙은 직접 소유한다.

외부 보조지표 라이브러리는 구현 후보이거나 교차 검증 대상일 뿐이다. public contract 가 되면 안 된다.

구현할 때 지킬 원칙:

- 단순한 공식은 직접 구현하는 쪽을 우선 검토한다.
- 널리 쓰이는 지표는 공식과 reference fixture 를 고정한다.
- 외부 라이브러리를 쓰더라도 `packages/indicators` 내부 adapter 뒤에 숨긴다.
- 라이선스가 `mwosa` 배포 조건과 맞지 않으면 직접 의존하지 않는다.
- 같은 지표라도 라이브러리마다 warm-up, EMA 초기값, 결측 표현이 다를 수 있으므로 golden test 로 결과를 고정한다.

## 계산과 해석

`packages/indicators` 는 숫자를 만든다. 해석은 다른 레이어에서 다룬다.

예를 들어 MACD 계산은 아래 값을 만들 수 있다.

- MACD line
- signal line
- histogram

하지만 아래 판단은 하지 않는다.

- bullish
- bearish
- buy
- sell
- swing entry
- score

이런 해석은 나중에 `screen`, `strategy`, `research rule` 같은 별도 개념으로 다룬다. 그래야 `mwosa` 가 자동매매나 종목 추천 도구처럼 흐르지 않고, 리서치에 필요한 계산값을 일관되게 제공하는 CLI 로 남을 수 있다.

## 채택 조건

보조지표를 구현 대상으로 올릴 때는 아래 내용을 확인한다.

- 공식이 공개되어 있고 구현 차이를 설명할 수 있다.
- 필요한 입력 데이터가 `mwosa` 의 provider/storage 흐름에서 확보 가능하다.
- warm-up, 결측, 기간 파라미터를 명확히 표현할 수 있다.
- JSON/CSV 출력에서 사람이 오해하지 않을 이름을 붙일 수 있다.
- 다른 구현체나 fixture 로 교차 검증할 수 있다.

## 테스트

`packages/indicators` 에서 단위 테스트는 선택 사항이 아니다. 각 계산 함수는 기준 fixture 와 비교하는 단위 테스트를 반드시 가진다.

테스트 도구:

- Go 표준 `testing`
- `github.com/stretchr/testify/assert`
- `github.com/stretchr/testify/require`

필수 테스트:

- 공개 계산 함수마다 정상 입력 결과 테스트를 둔다.
- 공개 계산 함수마다 invalid input 테스트를 둔다.
- 입력 candle 이 시간순으로 정렬되어 있지 않으면 실패한다.
- high, low, close 같은 필수 값이 유효하지 않으면 실패한다.
- warm-up 구간은 `Valid=false` 로 표현된다.
- 각 범주의 대표 지표는 golden fixture 로 결과를 고정한다.
- MACD 기본 파라미터 `12, 26, 9` 와 일목균형표 기본 파라미터 `9, 26, 52, 26` 은 fixture 후보로 둔다.
- 외부 라이브러리를 쓰더라도 public result type 은 바뀌지 않는다.

테스트 작성 원칙:

- 테스트 이름은 지표, 조건, 기대 동작이 드러나게 쓴다.
- fixture 는 임의 숫자보다 계산 근거를 설명할 수 있는 작은 데이터셋을 우선한다.
- floating point 비교는 지표별 허용 오차를 함께 적는다.
- 입력 준비가 실패하면 `require` 로 중단한다.
- 여러 결과 point 를 비교할 때는 `assert` 로 실패를 한 번에 확인할 수 있게 한다.

선택 테스트:

- 대량 candle 입력 benchmark
- 다른 구현체와의 교차 검증
- weekly, monthly candle 입력 fixture

## 남은 결정

아직 정해야 할 항목:

- `packages/indicators` 를 독립 Go module 로 만들지 여부
- `float64` 만 쓸지, decimal type 을 일부 허용할지 여부
- 입력 결측 candle 을 error 로 볼지, gap-aware 계산을 지원할지 여부
- 일목균형표의 `PlotTime` 을 거래일 calendar 기준으로 밀지, 단순 index 기준으로 밀지 여부
- 보조지표 카탈로그를 코드 registry 로 둘지 문서 목록으로만 둘지 여부
- trend, momentum, volatility 같은 하위 package 구분을 둘지 여부
- 외부 라이브러리 후보를 실제 구현체로 채택할지 여부

## 관련 문서

- `docs/architectures/packages/README.md`
- `docs/architectures/layers/README.md`
- `docs/architectures/tech-stack/README.md`
