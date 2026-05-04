package strategy

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/ev3rlit/mwosa/packages/hashutil"
	"github.com/ev3rlit/mwosa/packages/idgen"
	provider "github.com/ev3rlit/mwosa/providers/core"
	"github.com/ev3rlit/mwosa/providers/core/dailybar"
	"github.com/ev3rlit/mwosa/service/daily"
	"github.com/itchyny/gojq"
	"github.com/samber/oops"
)

type Engine string

const EngineJQ Engine = "jq"

type ScreenRunStatus string

const (
	ScreenRunSucceeded ScreenRunStatus = "succeeded"
	ScreenRunFailed    ScreenRunStatus = "failed"
)

const defaultInputSchemaVersion = 1

type Strategy struct {
	ID              string     `json:"id" csv:"id"`
	Name            string     `json:"name" csv:"name"`
	Engine          Engine     `json:"engine" csv:"engine"`
	ActiveVersionID string     `json:"active_version_id" csv:"active_version_id"`
	CreatedAt       time.Time  `json:"created_at" csv:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at" csv:"updated_at"`
	ArchivedAt      *time.Time `json:"archived_at,omitempty" csv:"archived_at"`
}

type StrategyVersion struct {
	ID                 string          `json:"id" csv:"id"`
	StrategyID         string          `json:"strategy_id" csv:"strategy_id"`
	Version            int             `json:"version" csv:"version"`
	QueryText          string          `json:"query_text" csv:"query_text"`
	QueryHash          string          `json:"query_hash" csv:"query_hash"`
	InputDataset       string          `json:"input_dataset" csv:"input_dataset"`
	InputSchemaVersion int             `json:"input_schema_version" csv:"input_schema_version"`
	ParamsJSON         json.RawMessage `json:"params,omitempty" csv:"-"`
	CreatedAt          time.Time       `json:"created_at" csv:"created_at"`
	Note               string          `json:"note,omitempty" csv:"note"`
}

type ScreenRun struct {
	ID                 string          `json:"id" csv:"id"`
	Alias              string          `json:"alias,omitempty" csv:"alias"`
	StrategyID         string          `json:"strategy_id" csv:"strategy_id"`
	StrategyVersionID  string          `json:"strategy_version_id" csv:"strategy_version_id"`
	QueryHash          string          `json:"query_hash" csv:"query_hash"`
	InputDataset       string          `json:"input_dataset" csv:"input_dataset"`
	InputSchemaVersion int             `json:"input_schema_version" csv:"input_schema_version"`
	ParamsJSON         json.RawMessage `json:"params,omitempty" csv:"-"`
	DataFrom           string          `json:"data_from,omitempty" csv:"data_from"`
	DataTo             string          `json:"data_to,omitempty" csv:"data_to"`
	DataAsOf           string          `json:"data_as_of,omitempty" csv:"data_as_of"`
	StartedAt          time.Time       `json:"started_at" csv:"started_at"`
	FinishedAt         *time.Time      `json:"finished_at,omitempty" csv:"finished_at"`
	Status             ScreenRunStatus `json:"status" csv:"status"`
	ResultCount        int             `json:"result_count" csv:"result_count"`
	ResultHash         string          `json:"result_hash" csv:"result_hash"`
	ResultSizeBytes    int64           `json:"result_size_bytes" csv:"result_size_bytes"`
	SummaryJSON        json.RawMessage `json:"summary,omitempty" csv:"-"`
	ErrorMessage       string          `json:"error_message,omitempty" csv:"error_message"`
}

type ScreenRunItem struct {
	ID          string          `json:"id" csv:"id"`
	ScreenRunID string          `json:"screen_run_id" csv:"screen_run_id"`
	Ordinal     int             `json:"ordinal" csv:"ordinal"`
	Symbol      string          `json:"symbol,omitempty" csv:"symbol"`
	PayloadJSON json.RawMessage `json:"payload" csv:"-"`
}

type ScreenResultItem struct {
	Ordinal     int             `json:"ordinal" csv:"ordinal"`
	Symbol      string          `json:"symbol,omitempty" csv:"symbol"`
	PayloadJSON json.RawMessage `json:"payload" csv:"-"`
}

type ScreenResult struct {
	QueryHash          string             `json:"query_hash" csv:"query_hash"`
	InputDataset       string             `json:"input_dataset" csv:"input_dataset"`
	InputSchemaVersion int                `json:"input_schema_version" csv:"input_schema_version"`
	ResultCount        int                `json:"result_count" csv:"result_count"`
	Items              []ScreenResultItem `json:"items" csv:"-"`
}

type StrategyDetail struct {
	Strategy      Strategy        `json:"strategy"`
	ActiveVersion StrategyVersion `json:"active_version"`
}

type ScreenRunDetail struct {
	Run             ScreenRun       `json:"run"`
	Strategy        Strategy        `json:"strategy"`
	StrategyVersion StrategyVersion `json:"strategy_version"`
	Items           []ScreenRunItem `json:"items"`
}

type Repository interface {
	CreateStrategyWithVersion(ctx context.Context, strategy Strategy, version StrategyVersion) (StrategyDetail, error)
	ListStrategies(ctx context.Context) ([]StrategyDetail, error)
	GetStrategy(ctx context.Context, name string) (StrategyDetail, error)
	AddStrategyVersion(ctx context.Context, name string, version StrategyVersion, now time.Time) (StrategyDetail, error)
	ArchiveStrategy(ctx context.Context, name string, archivedAt time.Time) error
	CreateScreenRun(ctx context.Context, run ScreenRun, items []ScreenRunItem) (ScreenRunDetail, error)
	ListScreenRuns(ctx context.Context, limit int) ([]ScreenRun, error)
	GetScreenRun(ctx context.Context, ref string) (ScreenRunDetail, error)
}

type Dataset struct {
	Name          string
	SchemaVersion int
	Records       []json.RawMessage
}

type DatasetReader interface {
	ReadDataset(ctx context.Context, name string) (Dataset, error)
}

type Service struct {
	repo    Repository
	dataset DatasetReader
	now     func() time.Time
}

func NewService(repo Repository, dataset DatasetReader) (Service, error) {
	errb := oops.In("strategy_service")
	if repo == nil {
		return Service{}, errb.New("strategy repository is nil")
	}
	if dataset == nil {
		return Service{}, errb.New("strategy dataset reader is nil")
	}
	return Service{
		repo:    repo,
		dataset: dataset,
		now:     time.Now,
	}, nil
}

type CreateStrategyRequest struct {
	Name         string
	Engine       Engine
	InputDataset string
	QueryText    string
}

func (s Service) Create(ctx context.Context, req CreateStrategyRequest) (StrategyDetail, error) {
	errb := oops.In("strategy_service").With("name", req.Name, "engine", req.Engine, "input_dataset", req.InputDataset)
	if s.repo == nil {
		return StrategyDetail{}, errb.New("strategy repository is nil")
	}
	if err := validateStrategySource(req.Name, req.Engine, req.InputDataset, req.QueryText); err != nil {
		return StrategyDetail{}, errb.Wrap(err)
	}

	strategyID, err := idgen.NewUUIDV7()
	if err != nil {
		return StrategyDetail{}, errb.Wrapf(err, "generate strategy id")
	}
	versionID, err := idgen.NewUUIDV7()
	if err != nil {
		return StrategyDetail{}, errb.Wrapf(err, "generate strategy version id")
	}
	now := s.now()
	queryHash := hashutil.SHA256([]byte(req.QueryText))
	strategy := Strategy{
		ID:              strategyID,
		Name:            req.Name,
		Engine:          req.Engine,
		ActiveVersionID: versionID,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	version := StrategyVersion{
		ID:                 versionID,
		StrategyID:         strategyID,
		Version:            1,
		QueryText:          req.QueryText,
		QueryHash:          queryHash,
		InputDataset:       req.InputDataset,
		InputSchemaVersion: defaultInputSchemaVersion,
		ParamsJSON:         json.RawMessage(`{}`),
		CreatedAt:          now,
	}
	return s.repo.CreateStrategyWithVersion(ctx, strategy, version)
}

func (s Service) List(ctx context.Context) ([]StrategyDetail, error) {
	if s.repo == nil {
		return nil, oops.In("strategy_service").New("strategy repository is nil")
	}
	return s.repo.ListStrategies(ctx)
}

func (s Service) Inspect(ctx context.Context, name string) (StrategyDetail, error) {
	if strings.TrimSpace(name) == "" {
		return StrategyDetail{}, oops.In("strategy_service").New("inspect strategy requires name")
	}
	return s.repo.GetStrategy(ctx, name)
}

type UpdateStrategyRequest struct {
	Name      string
	QueryText string
}

func (s Service) Update(ctx context.Context, req UpdateStrategyRequest) (StrategyDetail, error) {
	errb := oops.In("strategy_service").With("name", req.Name)
	if strings.TrimSpace(req.Name) == "" {
		return StrategyDetail{}, errb.New("update strategy requires name")
	}
	detail, err := s.repo.GetStrategy(ctx, req.Name)
	if err != nil {
		return StrategyDetail{}, errb.Wrapf(err, "load strategy before update")
	}
	if err := validateStrategySource(detail.Strategy.Name, detail.Strategy.Engine, detail.ActiveVersion.InputDataset, req.QueryText); err != nil {
		return StrategyDetail{}, errb.Wrap(err)
	}
	versionID, err := idgen.NewUUIDV7()
	if err != nil {
		return StrategyDetail{}, errb.Wrapf(err, "generate strategy version id")
	}
	now := s.now()
	version := StrategyVersion{
		ID:                 versionID,
		StrategyID:         detail.Strategy.ID,
		Version:            detail.ActiveVersion.Version + 1,
		QueryText:          req.QueryText,
		QueryHash:          hashutil.SHA256([]byte(req.QueryText)),
		InputDataset:       detail.ActiveVersion.InputDataset,
		InputSchemaVersion: detail.ActiveVersion.InputSchemaVersion,
		ParamsJSON:         normalizeJSON(detail.ActiveVersion.ParamsJSON),
		CreatedAt:          now,
	}
	return s.repo.AddStrategyVersion(ctx, req.Name, version, now)
}

func (s Service) Delete(ctx context.Context, name string) error {
	if strings.TrimSpace(name) == "" {
		return oops.In("strategy_service").New("delete strategy requires name")
	}
	return s.repo.ArchiveStrategy(ctx, name, s.now())
}

type ScreenStrategyRequest struct {
	Name  string
	Alias string
}

type ScreenJQRequest struct {
	InputDataset string
	QueryText    string
}

func (s Service) ScreenJQ(ctx context.Context, req ScreenJQRequest) (ScreenResult, error) {
	errb := oops.In("strategy_service").With("input_dataset", req.InputDataset)
	if s.dataset == nil {
		return ScreenResult{}, errb.New("strategy dataset reader is nil")
	}
	if strings.TrimSpace(req.InputDataset) == "" {
		return ScreenResult{}, errb.New("screen jq input dataset is required")
	}
	if strings.TrimSpace(req.QueryText) == "" {
		return ScreenResult{}, errb.New("screen jq query is required")
	}
	dataset, rows, err := s.executeJQAgainstDataset(ctx, req.InputDataset, req.QueryText)
	if err != nil {
		return ScreenResult{}, errb.Wrap(err)
	}
	return screenResultFromRows(req.QueryText, dataset, rows), nil
}

func (s Service) Screen(ctx context.Context, req ScreenStrategyRequest) (ScreenRunDetail, error) {
	errb := oops.In("strategy_service").With("name", req.Name, "alias", req.Alias)
	if strings.TrimSpace(req.Name) == "" {
		return ScreenRunDetail{}, errb.New("screen strategy requires name")
	}
	detail, err := s.repo.GetStrategy(ctx, req.Name)
	if err != nil {
		return ScreenRunDetail{}, errb.Wrapf(err, "load strategy")
	}
	started := s.now()
	dataset, rows, err := s.executeJQAgainstDataset(ctx, detail.ActiveVersion.InputDataset, detail.ActiveVersion.QueryText)
	if err != nil {
		return s.recordFailedRun(ctx, detail, req.Alias, started, errb.Wrapf(err, "execute jq strategy"))
	}
	return s.recordSucceededRun(ctx, detail, req.Alias, started, dataset, rows)
}

func (s Service) History(ctx context.Context, limit int) ([]ScreenRun, error) {
	if limit <= 0 {
		limit = 50
	}
	return s.repo.ListScreenRuns(ctx, limit)
}

func (s Service) InspectScreen(ctx context.Context, ref string) (ScreenRunDetail, error) {
	if strings.TrimSpace(ref) == "" {
		return ScreenRunDetail{}, oops.In("strategy_service").New("inspect screen requires id or alias")
	}
	return s.repo.GetScreenRun(ctx, ref)
}

func (s Service) executeJQAgainstDataset(ctx context.Context, inputDataset string, queryText string) (Dataset, []json.RawMessage, error) {
	errb := oops.In("strategy_service").With("input_dataset", inputDataset)
	dataset, err := s.dataset.ReadDataset(ctx, inputDataset)
	if err != nil {
		return Dataset{}, nil, errb.Wrapf(err, "read input dataset")
	}
	input, err := datasetInputValue(dataset.Records)
	if err != nil {
		return Dataset{}, nil, errb.Wrapf(err, "decode input dataset")
	}
	rows, err := executeJQ(ctx, queryText, input)
	if err != nil {
		return Dataset{}, nil, errb.Wrapf(err, "execute jq")
	}
	return dataset, rows, nil
}

func (s Service) recordSucceededRun(ctx context.Context, detail StrategyDetail, alias string, started time.Time, dataset Dataset, rows []json.RawMessage) (ScreenRunDetail, error) {
	errb := oops.In("strategy_service").With("strategy_id", detail.Strategy.ID, "alias", alias)
	resultJSON, err := json.Marshal(rows)
	if err != nil {
		return ScreenRunDetail{}, errb.Wrapf(err, "encode jq result rows")
	}
	summaryJSON, err := summarizeRows(rows)
	if err != nil {
		return ScreenRunDetail{}, err
	}
	runID, err := idgen.NewUUIDV7()
	if err != nil {
		return ScreenRunDetail{}, errb.Wrapf(err, "generate screen run id")
	}
	finished := s.now()
	run := screenRunFromStrategy(runID, alias, detail, dataset.SchemaVersion, started, &finished)
	run.Status = ScreenRunSucceeded
	run.ResultCount = len(rows)
	run.ResultHash = hashutil.SHA256(resultJSON)
	run.ResultSizeBytes = int64(len(resultJSON))
	run.SummaryJSON = summaryJSON

	items := make([]ScreenRunItem, 0, len(rows))
	for i, row := range rows {
		itemID, err := idgen.NewUUIDV7()
		if err != nil {
			return ScreenRunDetail{}, errb.With("ordinal", i).Wrapf(err, "generate screen run item id")
		}
		items = append(items, ScreenRunItem{
			ID:          itemID,
			ScreenRunID: run.ID,
			Ordinal:     i,
			Symbol:      extractSymbol(row),
			PayloadJSON: row,
		})
	}
	return s.repo.CreateScreenRun(ctx, run, items)
}

func screenResultFromRows(queryText string, dataset Dataset, rows []json.RawMessage) ScreenResult {
	items := make([]ScreenResultItem, 0, len(rows))
	for i, row := range rows {
		items = append(items, ScreenResultItem{
			Ordinal:     i,
			Symbol:      extractSymbol(row),
			PayloadJSON: row,
		})
	}
	return ScreenResult{
		QueryHash:          hashutil.SHA256([]byte(queryText)),
		InputDataset:       dataset.Name,
		InputSchemaVersion: dataset.SchemaVersion,
		ResultCount:        len(rows),
		Items:              items,
	}
}

func (s Service) recordFailedRun(ctx context.Context, detail StrategyDetail, alias string, started time.Time, runErr error) (ScreenRunDetail, error) {
	runID, err := idgen.NewUUIDV7()
	if err != nil {
		return ScreenRunDetail{}, oops.Join(runErr, oops.In("strategy_service").With("strategy_id", detail.Strategy.ID, "alias", alias).Wrapf(err, "generate failed screen run id"))
	}
	finished := s.now()
	run := screenRunFromStrategy(runID, alias, detail, detail.ActiveVersion.InputSchemaVersion, started, &finished)
	run.Status = ScreenRunFailed
	run.ErrorMessage = runErr.Error()
	saved, saveErr := s.repo.CreateScreenRun(ctx, run, nil)
	if saveErr != nil {
		return saved, oops.Join(runErr, oops.In("strategy_service").Wrapf(saveErr, "save failed screen run"))
	}
	return saved, runErr
}

func screenRunFromStrategy(id string, alias string, detail StrategyDetail, schemaVersion int, started time.Time, finished *time.Time) ScreenRun {
	return ScreenRun{
		ID:                 id,
		Alias:              strings.TrimSpace(alias),
		StrategyID:         detail.Strategy.ID,
		StrategyVersionID:  detail.ActiveVersion.ID,
		QueryHash:          detail.ActiveVersion.QueryHash,
		InputDataset:       detail.ActiveVersion.InputDataset,
		InputSchemaVersion: schemaVersion,
		ParamsJSON:         normalizeJSON(detail.ActiveVersion.ParamsJSON),
		StartedAt:          started,
		FinishedAt:         finished,
	}
}

func validateStrategySource(name string, engine Engine, inputDataset string, queryText string) error {
	errb := oops.In("strategy_service").With("name", name, "engine", engine, "input_dataset", inputDataset)
	if strings.TrimSpace(name) == "" {
		return errb.New("strategy name is required")
	}
	if engine != EngineJQ {
		return errb.Errorf("unsupported strategy engine: %s", engine)
	}
	if strings.TrimSpace(inputDataset) == "" {
		return errb.New("strategy input dataset is required")
	}
	if strings.TrimSpace(queryText) == "" {
		return errb.New("strategy jq query is required")
	}
	return nil
}

func normalizeJSON(raw json.RawMessage) json.RawMessage {
	if len(bytes.TrimSpace(raw)) == 0 {
		return json.RawMessage(`{}`)
	}
	return raw
}

type screenSummary struct {
	ResultCount int               `json:"result_count"`
	Preview     []json.RawMessage `json:"preview,omitempty"`
}

func summarizeRows(rows []json.RawMessage) (json.RawMessage, error) {
	limit := len(rows)
	if limit > 5 {
		limit = 5
	}
	summary := screenSummary{
		ResultCount: len(rows),
		Preview:     rows[:limit],
	}
	data, err := json.Marshal(summary)
	if err != nil {
		return nil, oops.In("strategy_service").Wrapf(err, "encode screen run summary")
	}
	return data, nil
}

func extractSymbol(raw json.RawMessage) string {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return ""
	}
	for _, key := range []string{"symbol", "security_code", "srtn_cd", "srtnCd"} {
		value, ok := object[key]
		if !ok {
			continue
		}
		var text string
		if err := json.Unmarshal(value, &text); err == nil {
			return text
		}
	}
	return ""
}

type DailyBarDatasetReader struct {
	reader daily.ReadRepository
	market provider.Market
}

func NewDailyBarDatasetReader(reader daily.ReadRepository, market provider.Market) (DailyBarDatasetReader, error) {
	if reader == nil {
		return DailyBarDatasetReader{}, oops.In("strategy_service").New("daily bar dataset reader repository is nil")
	}
	if market == "" {
		market = provider.MarketKRX
	}
	return DailyBarDatasetReader{reader: reader, market: market}, nil
}

func (r DailyBarDatasetReader) ReadDataset(ctx context.Context, name string) (Dataset, error) {
	errb := oops.In("strategy_service").With("input_dataset", name)
	records, err := r.readDailyBars(ctx, name)
	if err != nil {
		return Dataset{}, errb.Wrap(err)
	}
	return Dataset{
		Name:          name,
		SchemaVersion: defaultInputSchemaVersion,
		Records:       records,
	}, nil
}

func (r DailyBarDatasetReader) readDailyBars(ctx context.Context, name string) ([]json.RawMessage, error) {
	query := daily.Query{Market: r.market}
	switch name {
	case "daily_bar", "daily_bars":
	case "etf_daily_metrics":
		query.SecurityType = provider.SecurityTypeETF
	default:
		return nil, oops.In("strategy_service").With("input_dataset", name).Errorf("unsupported input dataset: %s", name)
	}
	bars, err := r.reader.QueryDailyBars(ctx, query)
	if err != nil {
		return nil, oops.In("strategy_service").With("input_dataset", name).Wrapf(err, "query daily bar dataset")
	}
	return dailyBarsToRawMessages(bars)
}

func dailyBarsToRawMessages(bars []dailybar.Bar) ([]json.RawMessage, error) {
	records := make([]json.RawMessage, 0, len(bars))
	for _, bar := range bars {
		data, err := json.Marshal(bar)
		if err != nil {
			return nil, oops.In("strategy_service").With("symbol", bar.Symbol).Wrapf(err, "encode daily bar record")
		}
		records = append(records, data)
	}
	return records, nil
}

func executeJQ(ctx context.Context, queryText string, input any) ([]json.RawMessage, error) {
	errb := oops.In("strategy_service").With("query_hash", hashutil.SHA256([]byte(queryText)))
	query, err := gojq.Parse(queryText)
	if err != nil {
		return nil, errb.Wrapf(err, "parse jq query")
	}
	iter := query.RunWithContext(ctx, input)
	rows := make([]json.RawMessage, 0)
	for {
		value, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := value.(error); ok {
			if halt, ok := err.(*gojq.HaltError); ok && halt.Value() == nil {
				break
			}
			return nil, errb.Wrapf(err, "run jq query")
		}
		flattened, err := flattenJQValue(value)
		if err != nil {
			return nil, errb.Wrap(err)
		}
		rows = append(rows, flattened...)
	}
	return rows, nil
}

func datasetInputValue(records []json.RawMessage) ([]any, error) {
	values := make([]any, 0, len(records))
	for _, record := range records {
		var value any
		if err := json.Unmarshal(record, &value); err != nil {
			return nil, oops.In("strategy_service").Wrapf(err, "decode input record")
		}
		values = append(values, value)
	}
	return values, nil
}

func flattenJQValue(value any) ([]json.RawMessage, error) {
	if array, ok := value.([]any); ok {
		rows := make([]json.RawMessage, 0, len(array))
		for _, item := range array {
			row, err := marshalJQValue(item)
			if err != nil {
				return nil, err
			}
			rows = append(rows, row)
		}
		return rows, nil
	}
	row, err := marshalJQValue(value)
	if err != nil {
		return nil, err
	}
	return []json.RawMessage{row}, nil
}

func marshalJQValue(value any) (json.RawMessage, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, oops.In("strategy_service").Wrapf(err, "encode jq result")
	}
	var buffer bytes.Buffer
	if err := json.Compact(&buffer, data); err != nil {
		return nil, oops.In("strategy_service").Wrapf(err, "compact json payload")
	}
	return buffer.Bytes(), nil
}
