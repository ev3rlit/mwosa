# Datago daily JSON collector

공공데이터포털 `datago`의 `securitiesProductPrice` 그룹에서 ETF/ETN 시세를 날짜 단위 JSON 파일로 수집하는 실험용 스크립트입니다.

원본 수집을 우선하기 위해 SQLite 정규화는 하지 않습니다. 대신 하루에 하나의 스냅샷 파일을 만들고, 실행 이력은 `manifest.jsonl`에 남깁니다. 같은 날짜 파일은 기본적으로 덮어씁니다.

## 실행

기본값은 오늘부터 1년 전까지, 최신 날짜부터 역순으로 수집합니다.

```bash
go run ./testing/experiments/datago_daily_json_collector \
  --service-key "$DATAGO_SERVICE_KEY"
```

## 환경 변수

`--service-key`를 매번 직접 쓰지 않으려면 `DATAGO_SERVICE_KEY` 환경변수를 지정합니다. 공공데이터포털에서 받은 일반 인증키나 URL 인코딩된 인증키 모두 사용할 수 있습니다.

현재 터미널 세션에 지정하려면:

```bash
export DATAGO_SERVICE_KEY='공공데이터포털_인증키'
```

한 번의 실행에만 적용하려면:

```bash
DATAGO_SERVICE_KEY='공공데이터포털_인증키' \
  go run ./testing/experiments/datago_daily_json_collector
```

`.env` 파일에 보관해두고 불러오려면:

```bash
printf 'DATAGO_SERVICE_KEY=%s\n' '공공데이터포털_인증키' > .env
set -a
source .env
set +a
```

이 저장소의 `.gitignore`에는 `.env`가 포함되어 있으므로 로컬 인증키 파일은 기본적으로 Git 추적 대상에서 제외됩니다.

기간과 출력 경로를 지정할 수 있습니다.

```bash
go run ./testing/experiments/datago_daily_json_collector \
  --service-key "$DATAGO_SERVICE_KEY" \
  --start-date 2025-04-28 \
  --end-date 2026-04-28 \
  --workers 2 \
  --overwrite=false \
  --output-dir tmp/testing/datago-daily-json-collector/raw
```

압축해서 저장하려면 `--compress`를 추가합니다.

```bash
go run ./testing/experiments/datago_daily_json_collector \
  --service-key "$DATAGO_SERVICE_KEY" \
  --compress
```

## 주요 옵션

- `--service-key`: 필수. 직접 넘기거나 `DATAGO_SERVICE_KEY` 환경변수를 사용합니다.
- `--start-date`, `--end-date`: `YYYY-MM-DD` 또는 `YYYYMMDD` 형식입니다.
- `--products`: 기본값은 `etf,etn`입니다.
- `--num-rows`: 페이지당 요청 건수입니다. 기본값은 `1000`입니다.
- `--workers`: 날짜 단위 병렬 워커 수입니다. 기본값은 `1`입니다.
- `--retries`: 페이지 요청별 재시도 횟수입니다. 기본값은 `3`입니다.
- `--retry-delay`: 첫 재시도 대기 시간입니다. 기본값은 `1s`입니다.
- `--retry-max-delay`: 재시도 대기 시간 상한입니다. 기본값은 `10s`입니다.
- `--compress`: 날짜 파일을 `gzip`으로 압축합니다. 기본값은 비활성화입니다.
- `--compression`: `none` 또는 `gzip`을 직접 지정합니다. 기본값은 `none`입니다.
- `--overwrite`: 기존 날짜 파일을 덮어쓸지 결정합니다. 기본값은 `true`입니다.
- `--direction`: 기본값은 `desc`입니다. 최신 날짜부터 수집합니다.

## 출력 구조

기본 출력 경로는 `tmp/testing/datago-daily-json-collector/raw`입니다.

```text
tmp/testing/datago-daily-json-collector/raw/
  2026/
    04/
      20260428.json
      20260427.json
  manifest.jsonl
```

각 날짜 파일은 ETF와 ETN을 함께 담습니다.

```json
{
  "schemaVersion": 1,
  "provider": "datago",
  "group": "securitiesProductPrice",
  "basDt": "20260428",
  "products": [
    {
      "product": "etf",
      "operation": "getETFPriceInfo",
      "rowCount": 1000,
      "items": []
    },
    {
      "product": "etn",
      "operation": "getETNPriceInfo",
      "rowCount": 300,
      "items": []
    }
  ]
}
```

## 이어받기

수집은 날짜 파일 단위로 원자적으로 저장합니다. 실행 중 실패해도 이미 완성된 날짜 파일은 남고, 실패 날짜는 `manifest.jsonl`에 `error` 상태로 기록됩니다.

같은 기간을 다시 실행하면 기본적으로 기존 날짜 파일을 덮어씁니다. 이미 성공한 날짜를 건너뛰고 실패 지점 이후만 채우고 싶다면 `--overwrite=false`를 사용합니다.

## 재시도와 병렬 처리

페이지 요청은 네트워크 오류, HTTP `408`, `429`, `5xx`, 일부 일시적 API 오류에 대해 재시도합니다. 인증키 오류나 잘못된 파라미터처럼 재시도로 해결되지 않는 오류는 바로 실패합니다.

병렬 처리는 날짜 단위로만 수행합니다. 한 날짜 안에서는 ETF/ETN과 페이지를 순차적으로 수집하므로, 하루 스냅샷 파일의 구조는 안정적으로 유지됩니다.

## 저변동 우상향 ETF 후보 추출

수집된 원본 JSON으로 저변동 우상향 ETF 후보를 추출할 수 있습니다. 실행 방법과 수식은 [scripts/README.md](scripts/README.md)에 따로 정리했습니다.
