# Bun Document Storage A-to-Z

## 목적

이 문서는 `mwosa` 의 jq screening storage 를 **SQLite 를 document collection 처럼
사용하고, Go ORM 으로 `github.com/uptrace/bun` 을 쓰는 관점**에서 검토한다.

여기서 말하는 Bun 은 JavaScript runtime `bun` 이 아니라, Go ORM / SQL builder 인
`github.com/uptrace/bun` 이다.

검토 우선순위는 다음과 같다.

1. 생산성
2. 코드 레벨 사용성
3. SQLite 단일 파일 저장소와의 궁합
4. JSON/JSONB payload 를 document 처럼 다루는 편의성
5. repository 경계 안에 storage 구현을 가두기 쉬운가

## 판단 요약

Bun 은 GORM 처럼 full ORM 으로 모든 것을 감추기보다, Go struct model 과 SQL
builder 사이에 얇게 서는 느낌이 강하다. `mwosa` 처럼 SQLite 를 유지하면서
`screen_runs`, `screen_run_items` 를 collection 처럼 다루려면 생산성과 통제의
균형이 좋다.

다만 Bun 을 선택해도 schema, index, JSONB 저장 방식은 결국 우리가 명시적으로
결정해야 한다. 이 점은 단점이 아니라, `mwosa` 의 storage 정책을 코드에서
읽히게 만드는 장점으로 볼 수 있다.

## 설치와 연결

Bun 은 `database/sql` 위에서 동작한다. SQLite 는 `sqliteshim` 을 쓰면 대부분의
플랫폼에서 pure Go `modernc.org/sqlite` 를 기본으로 사용할 수 있다.

```go
package storagebun

import (
	"database/sql"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
)

func Open(path string) (*bun.DB, error) {
	dsn := "file:" + path + "?cache=shared&mode=rwc&_busy_timeout=5000&_journal_mode=WAL"
	sqldb, err := sql.Open(sqliteshim.ShimName, dsn)
	if err != nil {
		return nil, err
	}

	db := bun.NewDB(sqldb, sqlitedialect.New())
	return db, nil
}
```

이 방식은 `mwosa` 의 기존 SQLite 방향과 잘 맞는다. CGO 없는 기본 경로를 유지할
수 있고, 필요하면 나중에 SQLite driver 를 바꿀 수 있다.

## Document Collection 모델

SQLite table 을 전통적인 업무 테이블이 아니라 collection 처럼 본다.

| table | document 관점 |
| --- | --- |
| `strategies` | 저장된 jq strategy collection |
| `strategy_versions` | strategy source snapshot collection |
| `screen_runs` | 스크리닝 실행 요약 collection |
| `screen_run_items` | 스크리닝 결과 row document collection |

핵심은 모든 필드를 column 으로 빼지 않는 것이다. 조회에 필요한 키만 column 으로
두고, 전략별 의미가 다른 값은 JSON payload 로 보관한다.

```go
package storagebun

import (
	"encoding/json"
	"time"

	"github.com/uptrace/bun"
)

type Strategy struct {
	bun.BaseModel `bun:"table:strategies,alias:s"`

	ID              string     `bun:"id,pk"`
	Name            string     `bun:"name,notnull,unique"`
	Engine          string     `bun:"engine,notnull"`
	ActiveVersionID string     `bun:"active_version_id,notnull"`
	CreatedAt       time.Time  `bun:"created_at,notnull"`
	UpdatedAt       time.Time  `bun:"updated_at,notnull"`
	ArchivedAt      *time.Time `bun:"archived_at"`
}

type StrategyVersion struct {
	bun.BaseModel `bun:"table:strategy_versions,alias:sv"`

	ID                 string          `bun:"id,pk"`
	StrategyID         string          `bun:"strategy_id,notnull"`
	Version            int             `bun:"version,notnull"`
	QueryText          string          `bun:"query_text,notnull"`
	QueryHash          string          `bun:"query_hash,notnull"`
	InputDataset       string          `bun:"input_dataset,notnull"`
	InputSchemaVersion int             `bun:"input_schema_version,notnull"`
	ParamsJSON         json.RawMessage `bun:"params_json,type:blob"`
	CreatedAt          time.Time       `bun:"created_at,notnull"`
	Note               string          `bun:"note"`
}

type ScreenRun struct {
	bun.BaseModel `bun:"table:screen_runs,alias:sr"`

	ID                 string          `bun:"id,pk"`
	Alias              *string         `bun:"alias,unique"`
	StrategyID         string          `bun:"strategy_id,notnull"`
	StrategyVersionID  string          `bun:"strategy_version_id,notnull"`
	QueryHash          string          `bun:"query_hash,notnull"`
	InputDataset       string          `bun:"input_dataset,notnull"`
	InputSchemaVersion int             `bun:"input_schema_version,notnull"`
	ParamsJSON         json.RawMessage `bun:"params_json,type:blob"`
	DataFrom           string          `bun:"data_from"`
	DataTo             string          `bun:"data_to"`
	DataAsOf           string          `bun:"data_as_of"`
	StartedAt          time.Time       `bun:"started_at,notnull"`
	FinishedAt         *time.Time      `bun:"finished_at"`
	Status             string          `bun:"status,notnull"`
	ResultCount        int             `bun:"result_count,notnull"`
	ResultHash         string          `bun:"result_hash,notnull"`
	ResultSizeBytes    int64           `bun:"result_size_bytes,notnull"`
	SummaryJSON        json.RawMessage `bun:"summary_json,type:blob"`
	ErrorMessage       string          `bun:"error_message"`

	Items []ScreenRunItem `bun:"rel:has-many,join:id=screen_run_id"`
}

type ScreenRunItem struct {
	bun.BaseModel `bun:"table:screen_run_items,alias:sri"`

	ID          string          `bun:"id,pk"`
	ScreenRunID string          `bun:"screen_run_id,notnull"`
	Ordinal     int             `bun:"ordinal,notnull"`
	Symbol      string          `bun:"symbol"`
	PayloadJSON json.RawMessage `bun:"payload_json,type:blob,notnull"`
}
```

`Alias` 는 optional 이므로 `*string` 으로 둔다. SQLite unique index 는 `NULL` 을
여러 개 허용하므로, alias 없는 ScreenRun 이 여러 개 존재할 수 있다.

## Schema 생성

Bun 은 `NewCreateTable` 로 model 기반 table 생성을 할 수 있다. 실제 제품에서는
버전 migration 을 쓰더라도, 라이브러리 검토 단계에서는 이 코드가 생산성을 잘
보여준다.

```go
func CreateSchema(ctx context.Context, db *bun.DB) error {
	models := []any{
		(*Strategy)(nil),
		(*StrategyVersion)(nil),
		(*ScreenRun)(nil),
		(*ScreenRunItem)(nil),
	}

	for _, model := range models {
		if _, err := db.NewCreateTable().
			Model(model).
			IfNotExists().
			Exec(ctx); err != nil {
			return err
		}
	}

	return CreateIndexes(ctx, db)
}
```

## Index 선언

Bun tag 로 단순 unique 는 표현할 수 있지만, 복합 index 와 partial index 는 코드로
명시하는 편이 좋다.

```go
func CreateIndexes(ctx context.Context, db *bun.DB) error {
	indexes := []struct {
		name  string
		table string
		expr  string
	}{
		{
			name:  "idx_strategy_versions_strategy_version",
			table: "strategy_versions",
			expr:  "strategy_id, version",
		},
		{
			name:  "idx_strategy_versions_query_hash",
			table: "strategy_versions",
			expr:  "query_hash",
		},
		{
			name:  "idx_screen_runs_started_at",
			table: "screen_runs",
			expr:  "started_at",
		},
		{
			name:  "idx_screen_runs_strategy_started",
			table: "screen_runs",
			expr:  "strategy_id, started_at",
		},
		{
			name:  "idx_screen_run_items_run_ordinal",
			table: "screen_run_items",
			expr:  "screen_run_id, ordinal",
		},
		{
			name:  "idx_screen_run_items_symbol",
			table: "screen_run_items",
			expr:  "symbol",
		},
	}

	for _, idx := range indexes {
		_, err := db.NewCreateIndex().
			IfNotExists().
			Index(idx.name).
			Table(idx.table).
			ColumnExpr(idx.expr).
			Exec(ctx)
		if err != nil {
			return err
		}
	}

	_, err := db.NewCreateIndex().
		IfNotExists().
		Unique().
		Index("idx_strategy_versions_strategy_version_unique").
		Table("strategy_versions").
		ColumnExpr("strategy_id, version").
		Exec(ctx)
	return err
}
```

SQLite 의 `alias TEXT UNIQUE` 는 alias 가 `NULL` 일 때 여러 row 를 허용한다.
빈 문자열을 alias 없음으로 쓰지 말고, 반드시 `NULL` 로 저장한다.

## SQLite JSONB 저장

application model 은 `json.RawMessage` 로 둔다. storage 구현에서 SQLite JSONB 로
변환할지, 표준 JSON text bytes 로 저장할지를 결정한다.

가장 단순한 방식은 그대로 BLOB 에 넣는 것이다.

```go
func InsertItemAsJSON(ctx context.Context, db bun.IDB, item *ScreenRunItem) error {
	_, err := db.NewInsert().Model(item).Exec(ctx)
	return err
}
```

SQLite JSONB 를 쓰려면 insert/update 시점에 `jsonb(?)` 를 적용한다. Bun 의
`Value` 를 쓰면 model field 값 대신 DB expression 을 넣을 수 있다.

```go
func InsertItemAsJSONB(ctx context.Context, db bun.IDB, item *ScreenRunItem) error {
	_, err := db.NewInsert().
		Model(item).
		Value("payload_json", "jsonb(?)", []byte(item.PayloadJSON)).
		Exec(ctx)
	return err
}
```

CLI 출력, export, pipeline 에서는 항상 표준 JSON/NDJSON text 로 변환한다.
JSONB 는 SQLite 내부 저장 최적화로만 취급한다.

## Strategy 생성

`--jq` 또는 `--jq-file` 은 입력 방식만 다르다. repository 는 최종 `query_text` 와
`query_hash` 를 받는다.

```go
type CreateStrategyInput struct {
	ID                 string
	Name               string
	QueryText          string
	QueryHash          string
	InputDataset       string
	InputSchemaVersion int
	ParamsJSON         json.RawMessage
	Now                time.Time
}

func CreateStrategy(ctx context.Context, db *bun.DB, in CreateStrategyInput) error {
	strategy := &Strategy{
		ID:              in.ID,
		Name:            in.Name,
		Engine:          "jq",
		ActiveVersionID: in.ID + "-v1",
		CreatedAt:       in.Now,
		UpdatedAt:       in.Now,
	}

	version := &StrategyVersion{
		ID:                 strategy.ActiveVersionID,
		StrategyID:         strategy.ID,
		Version:            1,
		QueryText:          in.QueryText,
		QueryHash:          in.QueryHash,
		InputDataset:       in.InputDataset,
		InputSchemaVersion: in.InputSchemaVersion,
		ParamsJSON:         in.ParamsJSON,
		CreatedAt:          in.Now,
	}

	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.NewInsert().Model(strategy).Exec(ctx); err != nil {
			return err
		}
		if _, err := tx.NewInsert().Model(version).Exec(ctx); err != nil {
			return err
		}
		return nil
	})
}
```

`RunInTx` 를 쓰면 strategy 와 version 이 따로 저장되는 중간 상태를 피할 수 있다.

## Strategy update

`update strategy` 는 기존 version 을 덮어쓰지 않고 새 version 을 만든다.

```go
func UpdateStrategyQuery(ctx context.Context, db *bun.DB, strategyID string, next StrategyVersion) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.NewInsert().Model(&next).Exec(ctx); err != nil {
			return err
		}

		_, err := tx.NewUpdate().
			Model((*Strategy)(nil)).
			Set("active_version_id = ?", next.ID).
			Set("updated_at = ?", next.CreatedAt).
			Where("id = ?", strategyID).
			Exec(ctx)
		return err
	})
}
```

같은 `query_hash` 로 update 하려는 경우는 service layer 에서 막는 것이 좋다.

## ScreenRun 저장

스크리닝 결과 저장은 transaction 으로 묶는다. 먼저 `screen_runs` 요약 row 를
넣고, 결과는 `screen_run_items` 로 bulk insert 한다.

```go
type SaveScreenRunInput struct {
	Run   ScreenRun
	Items []ScreenRunItem
}

func SaveScreenRun(ctx context.Context, db *bun.DB, in SaveScreenRunInput) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.NewInsert().Model(&in.Run).Exec(ctx); err != nil {
			return err
		}

		const chunkSize = 500
		for start := 0; start < len(in.Items); start += chunkSize {
			end := start + chunkSize
			if end > len(in.Items) {
				end = len(in.Items)
			}

			if _, err := tx.NewInsert().Model(&in.Items[start:end]).Exec(ctx); err != nil {
				return err
			}
		}

		return nil
	})
}
```

이 코드는 생산성 측면에서 중요하다. application service 는 aggregate 처럼 넘기고,
repository 내부에서만 table split 과 chunk insert 를 신경 쓴다.

## history screen

최근 스크리닝 목록은 `screen_runs` 만 읽는다. item payload 를 같이 읽지 않는다.

```go
func ListScreenHistory(ctx context.Context, db bun.IDB, limit int) ([]ScreenRun, error) {
	var runs []ScreenRun
	err := db.NewSelect().
		Model(&runs).
		Column(
			"id",
			"alias",
			"strategy_id",
			"strategy_version_id",
			"query_hash",
			"input_dataset",
			"started_at",
			"finished_at",
			"status",
			"result_count",
			"summary_json",
		).
		OrderExpr("started_at DESC").
		Limit(limit).
		Scan(ctx)
	return runs, err
}
```

이 경로가 가벼워야 `history screen` 이 자주 쓰이는 CLI 명령이 될 수 있다.

## inspect screen

단일 screen 은 id 또는 alias 로 찾고, item 은 paging 으로 읽는다.

```go
type InspectScreenResult struct {
	Run   ScreenRun
	Items []ScreenRunItem
}

func InspectScreen(ctx context.Context, db bun.IDB, key string, limit, offset int) (InspectScreenResult, error) {
	var run ScreenRun
	if err := db.NewSelect().
		Model(&run).
		Where("id = ? OR alias = ?", key, key).
		Scan(ctx); err != nil {
		return InspectScreenResult{}, err
	}

	var items []ScreenRunItem
	if err := db.NewSelect().
		Model(&items).
		Where("screen_run_id = ?", run.ID).
		OrderExpr("ordinal ASC").
		Limit(limit).
		Offset(offset).
		Scan(ctx); err != nil {
		return InspectScreenResult{}, err
	}

	return InspectScreenResult{Run: run, Items: items}, nil
}
```

`Relation("Items")` 로 한 번에 불러올 수도 있지만, 결과가 커질 수 있으므로 CLI
에서는 paging 을 기본으로 둔다.

## symbol 역조회

특정 symbol 이 과거 어떤 screen 에 나왔는지 찾으려면 `screen_run_items.symbol`
index 가 필요하다.

```go
type SymbolHit struct {
	ScreenRunID string    `bun:"screen_run_id"`
	Alias       *string   `bun:"alias"`
	StrategyID  string    `bun:"strategy_id"`
	StartedAt   time.Time `bun:"started_at"`
	Ordinal     int       `bun:"ordinal"`
}

func FindScreensBySymbol(ctx context.Context, db bun.IDB, symbol string, limit int) ([]SymbolHit, error) {
	var hits []SymbolHit
	err := db.NewSelect().
		TableExpr("screen_run_items AS sri").
		ColumnExpr("sri.screen_run_id, sr.alias, sr.strategy_id, sr.started_at, sri.ordinal").
		Join("JOIN screen_runs AS sr ON sr.id = sri.screen_run_id").
		Where("sri.symbol = ?", symbol).
		OrderExpr("sr.started_at DESC").
		Limit(limit).
		Scan(ctx, &hits)
	return hits, err
}
```

이것은 document-style storage 에서도 검색용 scalar field 를 몇 개 빼야 하는
이유다.

## Raw query escape hatch

SQLite JSON 함수나 `json_each` 처럼 query builder 로 표현하면 오히려 읽기 어려운
기능은 raw query 로 둔다. Bun 은 raw SQL 을 완전히 막지 않기 때문에, document DB
처럼 쓰다가 SQLite 고유 기능이 필요한 순간에도 우회로가 있다.

```go
type PayloadSymbol struct {
	ScreenRunID string `bun:"screen_run_id"`
	Symbol      string `bun:"symbol"`
}

func FindSymbolsFromPayload(ctx context.Context, db bun.IDB, runID string) ([]PayloadSymbol, error) {
	var rows []PayloadSymbol
	err := db.NewRaw(`
		SELECT
			screen_run_id,
			json_extract(payload_json, '$.symbol') AS symbol
		FROM screen_run_items
		WHERE screen_run_id = ?
		  AND json_extract(payload_json, '$.symbol') IS NOT NULL
		ORDER BY ordinal ASC
	`, runID).Scan(ctx, &rows)
	return rows, err
}
```

이 방식은 남용하면 repository 가 SQL 조각으로 흩어진다. 하지만 JSON payload
내부를 임시로 검증하거나, 특정 SQLite 기능을 실험할 때는 생산성이 좋다.

## Upsert 와 idempotency

CLI 는 같은 command 가 재시도될 수 있다. 이때 `query_hash`, `alias`, `screen_id`
같은 key 에 대해 중복 저장 정책을 명시해야 한다.

```go
func CreateStrategyVersionIfMissing(ctx context.Context, db bun.IDB, version *StrategyVersion) (bool, error) {
	result, err := db.NewInsert().
		Model(version).
		On("CONFLICT(strategy_id, version) DO NOTHING").
		Exec(ctx)
	if err != nil {
		return false, err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected == 1, nil
}
```

중요한 점은 `DO NOTHING` 을 조용한 성공으로 취급하지 않는 것이다. repository 는
insert 되었는지 여부를 돌려주고, service layer 가 사용자에게 `already exists`
또는 `same version` 같은 의미를 붙인다.

## Relation 사용

Bun relation 은 편하지만 항상 쓰면 안 된다. 결과가 작고 명확할 때만 사용한다.

```go
func InspectScreenSmall(ctx context.Context, db bun.IDB, key string) (*ScreenRun, error) {
	run := new(ScreenRun)
	err := db.NewSelect().
		Model(run).
		Relation("Items", func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.OrderExpr("ordinal ASC").Limit(100)
		}).
		Where("sr.id = ? OR sr.alias = ?", key, key).
		Scan(ctx)
	return run, err
}
```

`history screen` 에서는 relation 을 쓰지 않는다.

## Query hook 과 관측성

Bun 은 query hook 으로 실행 SQL 을 로깅하거나 tracing 할 수 있다. 라이브러리
검토 단계에서는 매우 중요하다. ORM 이 어떤 SQL 을 만들고 있는지 바로 확인할 수
있어야 생산성이 유지된다.

```go
import "github.com/uptrace/bun/extra/bundebug"

func EnableSQLDebug(db *bun.DB, verbose bool) {
	db.AddQueryHook(bundebug.NewQueryHook(
		bundebug.WithEnabled(true),
		bundebug.WithVerbose(verbose),
	))
}
```

제품 코드에서는 debug hook 을 항상 켜기보다 `MWOSA_SQL_DEBUG=1` 같은 개발용
옵션으로 묶는다.

## Hook 사용

Bun 은 model hook 을 지원한다. 생산성에는 도움이 되지만, 저장 정책이 숨을 수
있으므로 가볍게만 쓴다.

```go
func (r *ScreenRun) BeforeAppendModel(ctx context.Context, query bun.Query) error {
	switch query.(type) {
	case *bun.InsertQuery:
		if r.StartedAt.IsZero() {
			r.StartedAt = time.Now()
		}
	}
	return nil
}
```

`query_hash`, `result_hash`, `result_size_bytes` 같은 값은 hook 안에서 몰래 만들기
보다 service layer 에서 계산해 명시적으로 넘기는 편이 좋다.

## Migration

Bun 은 Go 기반 migration 과 SQL 파일 migration 을 모두 지원한다. 앱 레벨 schema
관리를 중시한다면 Go 기반 migration 이 더 자연스럽다.

```go
package migrations

import "github.com/uptrace/bun/migrate"

var Migrations = migrate.NewMigrations()
```

```go
package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		return CreateSchema(ctx, db)
	}, func(ctx context.Context, db *bun.DB) error {
		for _, table := range []string{
			"screen_run_items",
			"screen_runs",
			"strategy_versions",
			"strategies",
		} {
			if _, err := db.NewDropTable().IfExists().Table(table).Exec(ctx); err != nil {
				return err
			}
		}
		return nil
	})
}
```

주의할 점은 migration 도 application code 라는 점이다. 한번 배포된 migration 은
나중에 model 이 바뀌어도 과거 의미를 보존해야 한다.

## Testability

SQLite file 하나로 테스트할 수 있다.

```go
func newTestDB(t *testing.T) *bun.DB {
	t.Helper()

	path := filepath.Join(t.TempDir(), "mwosa-test.db")
	db, err := Open(path)
	require.NoError(t, err)

	require.NoError(t, CreateSchema(context.Background(), db))
	t.Cleanup(func() {
		require.NoError(t, db.Close())
	})

	return db
}
```

검증해야 할 테스트는 다음이다.

- alias 없이 여러 ScreenRun 을 저장할 수 있다.
- 같은 alias 는 중복 저장되지 않는다.
- `history screen` 은 item payload 를 읽지 않는다.
- `inspect screen` 은 item 을 ordinal 순서로 paging 한다.
- `screen_run_items.symbol` 로 역조회할 수 있다.
- JSONB 를 사용하더라도 CLI 출력은 JSON text 로 복원된다.

## Senior Review Checklist

검토할 때는 다음 질문을 먼저 본다.

| 관점 | 확인할 질문 |
| --- | --- |
| 생산성 | model 과 query 코드가 하루 안에 실제 기능으로 이어지는가 |
| 명시성 | transaction, index, paging, JSONB 정책이 코드에서 읽히는가 |
| 경계 | Bun type 이 service layer 로 새지 않는가 |
| 데이터 보존 | ScreenRun 과 ScreenRunItem 이 나중에 판단을 복원할 만큼 정보를 남기는가 |
| 성능 | history 는 가볍고 inspect 는 paging 되는가 |
| 확장성 | 나중에 symbol, strategy, schema version 기준 조회를 추가하기 쉬운가 |
| 이식성 | SQLite driver 를 바꿔도 repository 외부 영향이 작은가 |

## 결론

Bun 은 `mwosa` 의 jq screening storage 에 꽤 잘 맞는다.

좋은 점:

- Go struct 로 collection-like table 을 정의하기 쉽다.
- relation 은 필요할 때만 쓸 수 있다.
- query builder 가 있어서 history, inspect, symbol 역조회가 코드로 잘 읽힌다.
- transaction 과 bulk insert 를 application aggregate 저장 흐름에 맞추기 쉽다.
- SQLite JSONB 같은 세부 정책을 숨기지 않고 repository 안에서 제어할 수 있다.

주의할 점:

- JSONB 저장은 Bun 이 알아서 해결해주는 것이 아니라 insert/update 경로에서
  명시해야 한다.
- hook 을 많이 쓰면 저장 정책이 숨어서 디버깅이 어려워진다.
- migration 은 model 자동 생성에만 기대지 말고 버전별 의미를 보존해야 한다.
- service layer 는 Bun model 을 직접 보지 않고 repository interface 만 봐야 한다.

현재 생산성을 1순위로 보면, `SQLite + Bun + document collection style` 은
검토할 가치가 높다. 단, 최종 선택 전에는 실제로 `create strategy`, `screen
strategy`, `history screen`, `inspect screen` 네 흐름을 작은 spike 로 구현해보는
것이 좋다.

## 참고

- [Bun models](https://bun.uptrace.dev/guide/models.html)
- [Bun drivers and dialects](https://bun.uptrace.dev/guide/drivers.html)
- [Bun writing queries](https://bun.uptrace.dev/guide/queries.html)
- [Bun placeholders](https://bun.uptrace.dev/guide/placeholders.html)
- [Bun select query](https://bun.uptrace.dev/guide/query-select.html)
- [Bun insert query](https://bun.uptrace.dev/guide/query-insert.html)
- [Bun transactions](https://bun.uptrace.dev/guide/transactions.html)
- [Bun migrations](https://bun.uptrace.dev/guide/migrations.html)
- [Bun hooks](https://bun.uptrace.dev/guide/hooks.html)
- [Bun custom types](https://bun.uptrace.dev/guide/custom-types.html)
