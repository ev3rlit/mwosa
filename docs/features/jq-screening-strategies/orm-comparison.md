# ORM Comparison for Screening Storage

## 목적

이 문서는 [jq Screening Strategies](README.md) 의 `Strategy`,
`StrategyVersion`, `ScreenRun`, `ScreenRunItem` 저장 모델을 기준으로 Go
storage 선택지를 비교한다.

목표는 ORM 을 바로 결정하는 것이 아니라, 같은 모델이 GORM, Ent, Bun, sqlc 에서
어떤 코드 모양이 되는지 보는 것이다.

## 기준 모델

비교 기준은 다음 도메인 모델이다. `ScreenRun` 은 실행 요약만 가지고, 큰 결과는
`ScreenRunItem` 으로 row 단위 저장한다.

```go
type Strategy struct {
	ID              string
	Name            string
	Engine          Engine
	ActiveVersionID string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	ArchivedAt      *time.Time
}

type StrategyVersion struct {
	ID                 string
	StrategyID         string
	Version            int
	QueryText          string
	QueryHash          string
	InputDataset       string
	InputSchemaVersion int
	ParamsJSON         []byte
	CreatedAt          time.Time
	Note               string
}

type ScreenRun struct {
	ID                 string
	Alias              string
	StrategyID         string
	StrategyVersionID  string
	QueryHash          string
	InputDataset       string
	InputSchemaVersion int
	ParamsJSON         []byte
	DataFrom           string
	DataTo             string
	DataAsOf           string
	StartedAt          time.Time
	FinishedAt         *time.Time
	Status             ScreenRunStatus
	ResultCount        int
	ResultHash         string
	ResultSizeBytes    int64
	SummaryJSON        []byte
	ErrorMessage       string
}

type ScreenRunItem struct {
	ID          string
	ScreenRunID string
	Ordinal     int
	Symbol      string
	PayloadJSON []byte
}
```

## 비교 요약

| 선택지 | 코드 모양 | 장점 | 부담 |
| --- | --- | --- | --- |
| GORM | struct tag 가 DB schema 를 겸함 | 빠르게 모델을 만들기 쉽다 | tag 에 제약이 몰리고 query shape 가 숨기 쉽다 |
| Ent | schema code 에서 field/edge/index 를 선언 | 관계, migration, generated API 가 선명하다 | generated code 가 많고 service 로 새기 쉽다 |
| Bun | struct tag + SQL builder | SQL 에 가깝고 model tag 가 비교적 얇다 | 새 의존성이고 SQLite JSONB 세부는 직접 맞춰야 한다 |
| sqlc + database/sql | SQL schema/query 가 기준이고 Go 코드 생성 | 저장소 쿼리와 성능을 가장 명시적으로 제어한다 | CRUD 코드를 편하게 추상화해주지는 않는다 |

현재 `mwosa` 에서는 SQLite 를 정본 저장소로 두고, repository 뒤에 storage 구현을
숨기는 방향이 가장 중요하다. 따라서 ORM 을 쓰더라도 service layer 가 ORM 타입을
직접 보지 않게 해야 한다.

## GORM

GORM 은 Go struct 자체가 DB model 이 된다. 관계는 struct field 와 `gorm` tag 로
표현한다.

```go
package storagegorm

import "time"

type Strategy struct {
	ID              string     `gorm:"primaryKey;type:text"`
	Name            string     `gorm:"uniqueIndex;not null;type:text"`
	Engine          string     `gorm:"not null;type:text"`
	ActiveVersionID string     `gorm:"not null;type:text"`
	CreatedAt       time.Time  `gorm:"not null"`
	UpdatedAt       time.Time  `gorm:"not null"`
	ArchivedAt      *time.Time `gorm:"index"`

	Versions []StrategyVersion `gorm:"foreignKey:StrategyID"`
	ScreenRuns []ScreenRun     `gorm:"foreignKey:StrategyID"`
}

type StrategyVersion struct {
	ID                 string    `gorm:"primaryKey;type:text"`
	StrategyID         string    `gorm:"not null;index;type:text"`
	Version            int       `gorm:"not null"`
	QueryText          string    `gorm:"not null;type:text"`
	QueryHash          string    `gorm:"not null;index;type:text"`
	InputDataset       string    `gorm:"not null;type:text"`
	InputSchemaVersion int       `gorm:"not null"`
	ParamsJSON         []byte    `gorm:"type:blob"`
	CreatedAt          time.Time `gorm:"not null"`
	Note               string    `gorm:"type:text"`
}

type ScreenRun struct {
	ID                 string     `gorm:"primaryKey;type:text"`
	Alias              *string    `gorm:"uniqueIndex;type:text"`
	StrategyID         string     `gorm:"not null;index;type:text"`
	StrategyVersionID  string     `gorm:"not null;index;type:text"`
	QueryHash          string     `gorm:"not null;index;type:text"`
	InputDataset       string     `gorm:"not null;type:text"`
	InputSchemaVersion int        `gorm:"not null"`
	ParamsJSON         []byte     `gorm:"type:blob"`
	DataFrom           string     `gorm:"type:text"`
	DataTo             string     `gorm:"type:text"`
	DataAsOf           string     `gorm:"type:text"`
	StartedAt          time.Time  `gorm:"not null;index"`
	FinishedAt         *time.Time
	Status             string     `gorm:"not null;type:text"`
	ResultCount        int        `gorm:"not null"`
	ResultHash         string     `gorm:"not null;type:text"`
	ResultSizeBytes    int64      `gorm:"not null"`
	SummaryJSON        []byte     `gorm:"type:blob"`
	ErrorMessage       string     `gorm:"type:text"`

	Items []ScreenRunItem `gorm:"foreignKey:ScreenRunID"`
}

type ScreenRunItem struct {
	ID          string `gorm:"primaryKey;type:text"`
	ScreenRunID string `gorm:"not null;index;type:text"`
	Ordinal     int    `gorm:"not null;index"`
	Symbol      string `gorm:"index;type:text"`
	PayloadJSON []byte `gorm:"not null;type:blob"`
}
```

GORM 을 쓰면 `AutoMigrate` 와 `Preload` 로 빠르게 시작할 수 있다.

```go
func migrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&Strategy{},
		&StrategyVersion{},
		&ScreenRun{},
		&ScreenRunItem{},
	)
}

func inspectScreen(ctx context.Context, db *gorm.DB, id string) (ScreenRun, error) {
	var run ScreenRun
	err := db.WithContext(ctx).
		Preload("Items", func(tx *gorm.DB) *gorm.DB {
			return tx.Order("ordinal ASC")
		}).
		First(&run, "id = ? OR alias = ?", id, id).
		Error
	return run, err
}
```

이 방식은 빠르지만, DB 제약과 관계가 tag 안에 섞인다. `PayloadJSON` 을
SQLite JSONB 로 저장하려면 migration 또는 insert 시점에서 `jsonb(?)` 같은 SQLite
함수 적용을 별도로 고려해야 한다.

## Ent

Ent 는 Go struct 에 tag 를 붙이는 방식이 아니라 schema package 에 field, edge,
index 를 선언하고 generated code 를 만든다.

```go
package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type Strategy struct {
	ent.Schema
}

func (Strategy) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").Unique().Immutable(),
		field.String("name").Unique(),
		field.String("engine"),
		field.String("active_version_id"),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
		field.Time("archived_at").Optional().Nillable(),
	}
}

func (Strategy) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("versions", StrategyVersion.Type),
		edge.To("screen_runs", ScreenRun.Type),
	}
}

type StrategyVersion struct {
	ent.Schema
}

func (StrategyVersion) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").Unique().Immutable(),
		field.String("strategy_id"),
		field.Int("version"),
		field.String("query_text"),
		field.String("query_hash"),
		field.String("input_dataset"),
		field.Int("input_schema_version"),
		field.Bytes("params_json").Optional(),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.String("note").Optional(),
	}
}

func (StrategyVersion) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("strategy_id", "version").Unique(),
		index.Fields("query_hash"),
	}
}

type ScreenRun struct {
	ent.Schema
}

func (ScreenRun) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").Unique().Immutable(),
		field.String("alias").Optional().Nillable().Unique(),
		field.String("strategy_id"),
		field.String("strategy_version_id"),
		field.String("query_hash"),
		field.String("input_dataset"),
		field.Int("input_schema_version"),
		field.Bytes("params_json").Optional(),
		field.String("data_from").Optional(),
		field.String("data_to").Optional(),
		field.String("data_as_of").Optional(),
		field.Time("started_at").Default(time.Now).Immutable(),
		field.Time("finished_at").Optional().Nillable(),
		field.String("status"),
		field.Int("result_count"),
		field.String("result_hash"),
		field.Int64("result_size_bytes"),
		field.Bytes("summary_json").Optional(),
		field.String("error_message").Optional(),
	}
}

func (ScreenRun) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("items", ScreenRunItem.Type),
	}
}

func (ScreenRun) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("started_at"),
		index.Fields("strategy_id", "started_at"),
	}
}

type ScreenRunItem struct {
	ent.Schema
}

func (ScreenRunItem) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").Unique().Immutable(),
		field.String("screen_run_id"),
		field.Int("ordinal"),
		field.String("symbol").Optional(),
		field.Bytes("payload_json"),
	}
}

func (ScreenRunItem) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("screen_run_id", "ordinal").Unique(),
		index.Fields("symbol"),
	}
}
```

Ent 는 schema 와 generated query API 가 강하다. 다만 `mwosa` 에서는 generated
Ent entity 가 service layer 로 새지 않도록 repository 구현 안에 가두는 것이
중요하다.

## Bun

Bun 은 struct tag 로 table 과 relation 을 선언하지만, GORM 보다 SQL builder
감각이 강하다.

```go
package storagebun

import (
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

	Versions []StrategyVersion `bun:"rel:has-many,join:id=strategy_id"`
}

type StrategyVersion struct {
	bun.BaseModel `bun:"table:strategy_versions,alias:sv"`

	ID                 string    `bun:"id,pk"`
	StrategyID         string    `bun:"strategy_id,notnull"`
	Version            int       `bun:"version,notnull"`
	QueryText          string    `bun:"query_text,notnull"`
	QueryHash          string    `bun:"query_hash,notnull"`
	InputDataset       string    `bun:"input_dataset,notnull"`
	InputSchemaVersion int       `bun:"input_schema_version,notnull"`
	ParamsJSON         []byte    `bun:"params_json,type:blob"`
	CreatedAt          time.Time `bun:"created_at,notnull"`
	Note               string    `bun:"note"`
}

type ScreenRun struct {
	bun.BaseModel `bun:"table:screen_runs,alias:sr"`

	ID                 string     `bun:"id,pk"`
	Alias              *string    `bun:"alias,unique"`
	StrategyID         string     `bun:"strategy_id,notnull"`
	StrategyVersionID  string     `bun:"strategy_version_id,notnull"`
	QueryHash          string     `bun:"query_hash,notnull"`
	InputDataset       string     `bun:"input_dataset,notnull"`
	InputSchemaVersion int        `bun:"input_schema_version,notnull"`
	ParamsJSON         []byte     `bun:"params_json,type:blob"`
	DataFrom           string     `bun:"data_from"`
	DataTo             string     `bun:"data_to"`
	DataAsOf           string     `bun:"data_as_of"`
	StartedAt          time.Time  `bun:"started_at,notnull"`
	FinishedAt         *time.Time `bun:"finished_at"`
	Status             string     `bun:"status,notnull"`
	ResultCount        int        `bun:"result_count,notnull"`
	ResultHash         string     `bun:"result_hash,notnull"`
	ResultSizeBytes    int64      `bun:"result_size_bytes,notnull"`
	SummaryJSON        []byte     `bun:"summary_json,type:blob"`
	ErrorMessage       string     `bun:"error_message"`

	Items []ScreenRunItem `bun:"rel:has-many,join:id=screen_run_id"`
}

type ScreenRunItem struct {
	bun.BaseModel `bun:"table:screen_run_items,alias:sri"`

	ID          string `bun:"id,pk"`
	ScreenRunID string `bun:"screen_run_id,notnull"`
	Ordinal     int    `bun:"ordinal,notnull"`
	Symbol      string `bun:"symbol"`
	PayloadJSON []byte `bun:"payload_json,type:blob"`
}
```

Bun 은 query 를 직접 제어하기 쉽다.

```go
func inspectScreen(ctx context.Context, db *bun.DB, key string) (*ScreenRun, error) {
	run := new(ScreenRun)
	err := db.NewSelect().
		Model(run).
		Relation("Items", func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Order("ordinal ASC")
		}).
		Where("sr.id = ? OR sr.alias = ?", key, key).
		Scan(ctx)
	return run, err
}
```

SQLite 에서 JSONB 로 저장하려면 `[]byte` + BLOB 저장을 기본으로 두고, 쓰기
경로에서 SQLite `jsonb()` 결과를 bind 할지 별도로 결정한다.

## sqlc + database/sql

sqlc 는 ORM 이 아니다. SQL schema 와 query 를 먼저 쓰고, 그에 맞는 Go 코드를
생성한다. 현재 `mwosa` 의 SQLite 방향과 가장 잘 맞는 비교 기준이다.

```sql
CREATE TABLE strategies (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL UNIQUE,
  engine TEXT NOT NULL,
  active_version_id TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  archived_at TEXT
);

CREATE TABLE strategy_versions (
  id TEXT PRIMARY KEY,
  strategy_id TEXT NOT NULL,
  version INTEGER NOT NULL,
  query_text TEXT NOT NULL,
  query_hash TEXT NOT NULL,
  input_dataset TEXT NOT NULL,
  input_schema_version INTEGER NOT NULL,
  params_json BLOB,
  created_at TEXT NOT NULL,
  note TEXT NOT NULL DEFAULT '',
  UNIQUE(strategy_id, version)
);

CREATE TABLE screen_runs (
  id TEXT PRIMARY KEY,
  alias TEXT UNIQUE,
  strategy_id TEXT NOT NULL,
  strategy_version_id TEXT NOT NULL,
  query_hash TEXT NOT NULL,
  input_dataset TEXT NOT NULL,
  input_schema_version INTEGER NOT NULL,
  params_json BLOB,
  data_from TEXT,
  data_to TEXT,
  data_as_of TEXT,
  started_at TEXT NOT NULL,
  finished_at TEXT,
  status TEXT NOT NULL,
  result_count INTEGER NOT NULL,
  result_hash TEXT NOT NULL,
  result_size_bytes INTEGER NOT NULL,
  summary_json BLOB,
  error_message TEXT NOT NULL DEFAULT ''
);

CREATE TABLE screen_run_items (
  id TEXT PRIMARY KEY,
  screen_run_id TEXT NOT NULL,
  ordinal INTEGER NOT NULL,
  symbol TEXT,
  payload_json BLOB NOT NULL,
  UNIQUE(screen_run_id, ordinal)
);
```

대표 query 는 다음처럼 명시적으로 둔다.

```sql
-- name: CreateStrategy :exec
INSERT INTO strategies (
  id, name, engine, active_version_id, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?);

-- name: CreateStrategyVersion :exec
INSERT INTO strategy_versions (
  id, strategy_id, version, query_text, query_hash, input_dataset,
  input_schema_version, params_json, created_at, note
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: CreateScreenRun :exec
INSERT INTO screen_runs (
  id, alias, strategy_id, strategy_version_id, query_hash, input_dataset,
  input_schema_version, params_json, data_from, data_to, data_as_of,
  started_at, status, result_count, result_hash, result_size_bytes,
  summary_json, error_message
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: CreateScreenRunItem :exec
INSERT INTO screen_run_items (
  id, screen_run_id, ordinal, symbol, payload_json
) VALUES (?, ?, ?, ?, ?);

-- name: ListScreenHistory :many
SELECT
  id, alias, strategy_id, strategy_version_id, status,
  started_at, finished_at, result_count, summary_json
FROM screen_runs
ORDER BY started_at DESC
LIMIT ?;

-- name: ListScreenRunItems :many
SELECT id, screen_run_id, ordinal, symbol, payload_json
FROM screen_run_items
WHERE screen_run_id = ?
ORDER BY ordinal ASC
LIMIT ? OFFSET ?;
```

이 방식은 가장 덜 마법적이다. SQLite JSONB, chunk insert, transaction, index,
retention policy 를 직접 제어하기 쉽다. 대신 migration 과 query 작성 책임이
더 직접적으로 온다.

## 판단

이 기능은 다음 특성이 강하다.

- 실행 이력과 결과 row 를 정확히 보존해야 한다.
- 큰 JSON payload 를 다루므로 insert/query/삭제 정책이 중요하다.
- `history screen` 과 `inspect screen` 은 서로 다른 조회 패턴을 가진다.
- service layer 에 storage 구현이 새면 안 된다.

따라서 현재 기준에서는 `sqlc + database/sql` 또는 직접 작성한 repository 가
가장 잘 맞는다. Ent 는 CRUD 관리 화면이나 관계 탐색이 커질 때 후보가 될 수
있다. GORM 은 빠른 프로토타입에는 편하지만, 이 기능의 핵심인 실행 이력,
JSONB 저장, 결과 row paging 을 명시적으로 통제하기에는 너무 많은 의미가 tag
안에 들어간다. Bun 은 GORM 보다 SQL 감각이 살아 있지만, 지금 repo 에 새 ORM
의존성을 추가할 만큼의 이점은 아직 분명하지 않다.

## 참고

- [GORM models](https://gorm.io/docs/models.html)
- [GORM has many](https://gorm.io/docs/has_many.html)
- [Ent fields](https://entgo.io/docs/schema-fields/)
- [Ent edges](https://entgo.io/docs/schema-edges/)
- [Ent indexes](https://entgo.io/docs/schema-indexes)
- [Bun models](https://bun.uptrace.dev/guide/models.html)
- [Bun relations](https://bun.uptrace.dev/guide/relations.html)
- [sqlc query annotations](https://docs.sqlc.dev/en/v1.19.0/reference/query-annotations.html)
