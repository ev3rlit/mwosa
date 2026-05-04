package strategy

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	strategyservice "github.com/ev3rlit/mwosa/service/strategy"
	"github.com/ev3rlit/mwosa/storage"
	entdb "github.com/ev3rlit/mwosa/storage/ent"
	screenrunent "github.com/ev3rlit/mwosa/storage/ent/screenrun"
	screenrunitement "github.com/ev3rlit/mwosa/storage/ent/screenrunitem"
	strategyent "github.com/ev3rlit/mwosa/storage/ent/strategy"
	"github.com/samber/oops"
)

type repository struct {
	database *storage.Database
}

var _ strategyservice.Repository = (*repository)(nil)

func NewRepository(database *storage.Database) (strategyservice.Repository, error) {
	if database == nil {
		return nil, oops.In("strategy_repository").New("strategy repository database is nil")
	}
	return &repository{database: database}, nil
}

func (r *repository) CreateStrategyWithVersion(ctx context.Context, in strategyservice.Strategy, version strategyservice.StrategyVersion) (strategyservice.StrategyDetail, error) {
	errb := oops.In("strategy_repository").With("name", in.Name, "strategy_id", in.ID, "version_id", version.ID)
	client, err := r.database.Client(ctx)
	if err != nil {
		return strategyservice.StrategyDetail{}, errb.Wrap(err)
	}
	tx, err := client.Tx(ctx)
	if err != nil {
		return strategyservice.StrategyDetail{}, errb.Wrapf(err, "begin strategy transaction")
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	row, err := tx.Strategy.Create().
		SetID(in.ID).
		SetName(in.Name).
		SetEngine(string(in.Engine)).
		SetActiveVersionID(in.ActiveVersionID).
		SetCreatedAt(in.CreatedAt).
		SetUpdatedAt(in.UpdatedAt).
		Save(ctx)
	if err != nil {
		return strategyservice.StrategyDetail{}, errb.Wrapf(err, "create strategy sqlite row")
	}
	versionRow, err := createStrategyVersion(tx.StrategyVersion.Create(), version).Save(ctx)
	if err != nil {
		return strategyservice.StrategyDetail{}, errb.Wrapf(err, "create strategy version sqlite row")
	}
	if err := tx.Commit(); err != nil {
		return strategyservice.StrategyDetail{}, errb.Wrapf(err, "commit strategy transaction")
	}
	committed = true
	return strategyservice.StrategyDetail{
		Strategy:      strategyFromEnt(row),
		ActiveVersion: strategyVersionFromEnt(versionRow),
	}, nil
}

func (r *repository) ListStrategies(ctx context.Context) ([]strategyservice.StrategyDetail, error) {
	errb := oops.In("strategy_repository")
	client, err := r.database.Client(ctx)
	if err != nil {
		return nil, errb.Wrap(err)
	}
	rows, err := client.Strategy.Query().
		Where(strategyent.ArchivedAtIsNil()).
		Order(entdb.Asc(strategyent.FieldName)).
		All(ctx)
	if err != nil {
		return nil, errb.Wrapf(err, "list strategy sqlite rows")
	}
	details := make([]strategyservice.StrategyDetail, 0, len(rows))
	for _, row := range rows {
		version, err := client.StrategyVersion.Get(ctx, row.ActiveVersionID)
		if err != nil {
			return nil, errb.With("strategy_id", row.ID, "version_id", row.ActiveVersionID).Wrapf(err, "load active strategy version")
		}
		details = append(details, strategyservice.StrategyDetail{
			Strategy:      strategyFromEnt(row),
			ActiveVersion: strategyVersionFromEnt(version),
		})
	}
	return details, nil
}

func (r *repository) GetStrategy(ctx context.Context, name string) (strategyservice.StrategyDetail, error) {
	errb := oops.In("strategy_repository").With("name", name)
	client, err := r.database.Client(ctx)
	if err != nil {
		return strategyservice.StrategyDetail{}, errb.Wrap(err)
	}
	row, err := client.Strategy.Query().
		Where(strategyent.NameEQ(name), strategyent.ArchivedAtIsNil()).
		Only(ctx)
	if err != nil {
		if entdb.IsNotFound(err) {
			return strategyservice.StrategyDetail{}, errb.Errorf("strategy not found: %s", name)
		}
		return strategyservice.StrategyDetail{}, errb.Wrapf(err, "get strategy sqlite row")
	}
	version, err := client.StrategyVersion.Get(ctx, row.ActiveVersionID)
	if err != nil {
		return strategyservice.StrategyDetail{}, errb.With("strategy_id", row.ID, "version_id", row.ActiveVersionID).Wrapf(err, "load active strategy version")
	}
	return strategyservice.StrategyDetail{
		Strategy:      strategyFromEnt(row),
		ActiveVersion: strategyVersionFromEnt(version),
	}, nil
}

func (r *repository) AddStrategyVersion(ctx context.Context, name string, version strategyservice.StrategyVersion, now time.Time) (strategyservice.StrategyDetail, error) {
	errb := oops.In("strategy_repository").With("name", name, "version_id", version.ID)
	client, err := r.database.Client(ctx)
	if err != nil {
		return strategyservice.StrategyDetail{}, errb.Wrap(err)
	}
	tx, err := client.Tx(ctx)
	if err != nil {
		return strategyservice.StrategyDetail{}, errb.Wrapf(err, "begin update strategy transaction")
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	row, err := tx.Strategy.Query().
		Where(strategyent.NameEQ(name), strategyent.ArchivedAtIsNil()).
		Only(ctx)
	if err != nil {
		if entdb.IsNotFound(err) {
			return strategyservice.StrategyDetail{}, errb.Errorf("strategy not found: %s", name)
		}
		return strategyservice.StrategyDetail{}, errb.Wrapf(err, "load strategy for version update")
	}
	version.StrategyID = row.ID
	versionRow, err := createStrategyVersion(tx.StrategyVersion.Create(), version).Save(ctx)
	if err != nil {
		return strategyservice.StrategyDetail{}, errb.Wrapf(err, "create updated strategy version")
	}
	row, err = tx.Strategy.UpdateOneID(row.ID).
		SetActiveVersionID(version.ID).
		SetUpdatedAt(now).
		Save(ctx)
	if err != nil {
		return strategyservice.StrategyDetail{}, errb.Wrapf(err, "update active strategy version")
	}
	if err := tx.Commit(); err != nil {
		return strategyservice.StrategyDetail{}, errb.Wrapf(err, "commit update strategy transaction")
	}
	committed = true
	return strategyservice.StrategyDetail{
		Strategy:      strategyFromEnt(row),
		ActiveVersion: strategyVersionFromEnt(versionRow),
	}, nil
}

func (r *repository) ArchiveStrategy(ctx context.Context, name string, archivedAt time.Time) error {
	errb := oops.In("strategy_repository").With("name", name)
	client, err := r.database.Client(ctx)
	if err != nil {
		return errb.Wrap(err)
	}
	affected, err := client.Strategy.Update().
		Where(strategyent.NameEQ(name), strategyent.ArchivedAtIsNil()).
		SetArchivedAt(archivedAt).
		SetUpdatedAt(archivedAt).
		Save(ctx)
	if err != nil {
		return errb.Wrapf(err, "archive strategy sqlite row")
	}
	if affected == 0 {
		return errb.Errorf("strategy not found: %s", name)
	}
	return nil
}

func (r *repository) CreateScreenRun(ctx context.Context, run strategyservice.ScreenRun, items []strategyservice.ScreenRunItem) (strategyservice.ScreenRunDetail, error) {
	errb := oops.In("strategy_repository").With("screen_run_id", run.ID, "alias", run.Alias, "strategy_id", run.StrategyID)
	client, err := r.database.Client(ctx)
	if err != nil {
		return strategyservice.ScreenRunDetail{}, errb.Wrap(err)
	}
	tx, err := client.Tx(ctx)
	if err != nil {
		return strategyservice.ScreenRunDetail{}, errb.Wrapf(err, "begin screen run transaction")
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	runRow, err := createScreenRun(tx.ScreenRun.Create(), run).Save(ctx)
	if err != nil {
		return strategyservice.ScreenRunDetail{}, errb.Wrapf(err, "create screen run sqlite row")
	}
	itemRows := make([]*entdb.ScreenRunItem, 0, len(items))
	for _, item := range items {
		row, err := tx.ScreenRunItem.Create().
			SetID(item.ID).
			SetScreenRunID(item.ScreenRunID).
			SetOrdinal(item.Ordinal).
			SetSymbol(item.Symbol).
			SetPayloadJSON(item.PayloadJSON).
			Save(ctx)
		if err != nil {
			return strategyservice.ScreenRunDetail{}, errb.With("ordinal", item.Ordinal).Wrapf(err, "create screen run item sqlite row")
		}
		itemRows = append(itemRows, row)
	}
	strategyRow, err := tx.Strategy.Get(ctx, run.StrategyID)
	if err != nil {
		return strategyservice.ScreenRunDetail{}, errb.Wrapf(err, "load screen run strategy")
	}
	versionRow, err := tx.StrategyVersion.Get(ctx, run.StrategyVersionID)
	if err != nil {
		return strategyservice.ScreenRunDetail{}, errb.Wrapf(err, "load screen run strategy version")
	}
	if err := tx.Commit(); err != nil {
		return strategyservice.ScreenRunDetail{}, errb.Wrapf(err, "commit screen run transaction")
	}
	committed = true
	return screenRunDetailFromEnt(runRow, strategyRow, versionRow, itemRows), nil
}

func (r *repository) ListScreenRuns(ctx context.Context, limit int) ([]strategyservice.ScreenRun, error) {
	errb := oops.In("strategy_repository").With("limit", limit)
	client, err := r.database.Client(ctx)
	if err != nil {
		return nil, errb.Wrap(err)
	}
	rows, err := client.ScreenRun.Query().
		Order(entdb.Desc(screenrunent.FieldStartedAt)).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, errb.Wrapf(err, "list screen runs sqlite rows")
	}
	runs := make([]strategyservice.ScreenRun, 0, len(rows))
	for _, row := range rows {
		runs = append(runs, screenRunFromEnt(row))
	}
	return runs, nil
}

func (r *repository) GetScreenRun(ctx context.Context, ref string) (strategyservice.ScreenRunDetail, error) {
	errb := oops.In("strategy_repository").With("ref", ref)
	client, err := r.database.Client(ctx)
	if err != nil {
		return strategyservice.ScreenRunDetail{}, errb.Wrap(err)
	}
	runRow, err := client.ScreenRun.Query().
		Where(screenrunent.Or(screenrunent.IDEQ(ref), screenrunent.AliasEQ(ref))).
		Only(ctx)
	if err != nil {
		if entdb.IsNotFound(err) {
			return strategyservice.ScreenRunDetail{}, errb.Errorf("screen run not found: %s", ref)
		}
		return strategyservice.ScreenRunDetail{}, errb.Wrapf(err, "get screen run sqlite row")
	}
	strategyRow, err := client.Strategy.Get(ctx, runRow.StrategyID)
	if err != nil {
		return strategyservice.ScreenRunDetail{}, errb.With("strategy_id", runRow.StrategyID).Wrapf(err, "load screen run strategy")
	}
	versionRow, err := client.StrategyVersion.Get(ctx, runRow.StrategyVersionID)
	if err != nil {
		return strategyservice.ScreenRunDetail{}, errb.With("strategy_version_id", runRow.StrategyVersionID).Wrapf(err, "load screen run strategy version")
	}
	itemRows, err := client.ScreenRunItem.Query().
		Where(screenrunitement.ScreenRunIDEQ(runRow.ID)).
		Order(entdb.Asc(screenrunitement.FieldOrdinal)).
		All(ctx)
	if err != nil {
		return strategyservice.ScreenRunDetail{}, errb.With("screen_run_id", runRow.ID).Wrapf(err, "load screen run items")
	}
	return screenRunDetailFromEnt(runRow, strategyRow, versionRow, itemRows), nil
}

func createStrategyVersion(create *entdb.StrategyVersionCreate, version strategyservice.StrategyVersion) *entdb.StrategyVersionCreate {
	return create.
		SetID(version.ID).
		SetStrategyID(version.StrategyID).
		SetVersion(version.Version).
		SetQueryText(version.QueryText).
		SetQueryHash(version.QueryHash).
		SetInputDataset(version.InputDataset).
		SetInputSchemaVersion(version.InputSchemaVersion).
		SetParamsJSON(normalizeRawMessage(version.ParamsJSON)).
		SetCreatedAt(version.CreatedAt).
		SetNote(version.Note)
}

func createScreenRun(create *entdb.ScreenRunCreate, run strategyservice.ScreenRun) *entdb.ScreenRunCreate {
	builder := create.
		SetID(run.ID).
		SetStrategyID(run.StrategyID).
		SetStrategyVersionID(run.StrategyVersionID).
		SetQueryHash(run.QueryHash).
		SetInputDataset(run.InputDataset).
		SetInputSchemaVersion(run.InputSchemaVersion).
		SetParamsJSON(normalizeRawMessage(run.ParamsJSON)).
		SetDataFrom(run.DataFrom).
		SetDataTo(run.DataTo).
		SetDataAsOf(run.DataAsOf).
		SetStartedAt(run.StartedAt).
		SetStatus(string(run.Status)).
		SetResultCount(run.ResultCount).
		SetResultHash(run.ResultHash).
		SetResultSizeBytes(run.ResultSizeBytes).
		SetSummaryJSON(normalizeRawMessage(run.SummaryJSON)).
		SetErrorMessage(run.ErrorMessage)
	if strings.TrimSpace(run.Alias) != "" {
		builder.SetAlias(run.Alias)
	}
	if run.FinishedAt != nil {
		builder.SetFinishedAt(*run.FinishedAt)
	}
	return builder
}

func normalizeRawMessage(raw json.RawMessage) []byte {
	if len(raw) == 0 {
		return []byte("{}")
	}
	return raw
}

func strategyFromEnt(row *entdb.Strategy) strategyservice.Strategy {
	return strategyservice.Strategy{
		ID:              row.ID,
		Name:            row.Name,
		Engine:          strategyservice.Engine(row.Engine),
		ActiveVersionID: row.ActiveVersionID,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
		ArchivedAt:      row.ArchivedAt,
	}
}

func strategyVersionFromEnt(row *entdb.StrategyVersion) strategyservice.StrategyVersion {
	return strategyservice.StrategyVersion{
		ID:                 row.ID,
		StrategyID:         row.StrategyID,
		Version:            row.Version,
		QueryText:          row.QueryText,
		QueryHash:          row.QueryHash,
		InputDataset:       row.InputDataset,
		InputSchemaVersion: row.InputSchemaVersion,
		ParamsJSON:         row.ParamsJSON,
		CreatedAt:          row.CreatedAt,
		Note:               row.Note,
	}
}

func screenRunFromEnt(row *entdb.ScreenRun) strategyservice.ScreenRun {
	run := strategyservice.ScreenRun{
		ID:                 row.ID,
		StrategyID:         row.StrategyID,
		StrategyVersionID:  row.StrategyVersionID,
		QueryHash:          row.QueryHash,
		InputDataset:       row.InputDataset,
		InputSchemaVersion: row.InputSchemaVersion,
		ParamsJSON:         row.ParamsJSON,
		DataFrom:           row.DataFrom,
		DataTo:             row.DataTo,
		DataAsOf:           row.DataAsOf,
		StartedAt:          row.StartedAt,
		FinishedAt:         row.FinishedAt,
		Status:             strategyservice.ScreenRunStatus(row.Status),
		ResultCount:        row.ResultCount,
		ResultHash:         row.ResultHash,
		ResultSizeBytes:    row.ResultSizeBytes,
		SummaryJSON:        row.SummaryJSON,
		ErrorMessage:       row.ErrorMessage,
	}
	if row.Alias != nil {
		run.Alias = *row.Alias
	}
	return run
}

func screenRunItemFromEnt(row *entdb.ScreenRunItem) strategyservice.ScreenRunItem {
	return strategyservice.ScreenRunItem{
		ID:          row.ID,
		ScreenRunID: row.ScreenRunID,
		Ordinal:     row.Ordinal,
		Symbol:      row.Symbol,
		PayloadJSON: row.PayloadJSON,
	}
}

func screenRunDetailFromEnt(run *entdb.ScreenRun, strategy *entdb.Strategy, version *entdb.StrategyVersion, items []*entdb.ScreenRunItem) strategyservice.ScreenRunDetail {
	detail := strategyservice.ScreenRunDetail{
		Run:             screenRunFromEnt(run),
		Strategy:        strategyFromEnt(strategy),
		StrategyVersion: strategyVersionFromEnt(version),
		Items:           make([]strategyservice.ScreenRunItem, 0, len(items)),
	}
	for _, item := range items {
		detail.Items = append(detail.Items, screenRunItemFromEnt(item))
	}
	return detail
}
