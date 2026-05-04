package app

import (
	"github.com/ev3rlit/mwosa/providers/builtin"
	provider "github.com/ev3rlit/mwosa/providers/core"
	"github.com/ev3rlit/mwosa/providers/core/dailybar"
	"github.com/ev3rlit/mwosa/providers/core/instrument"
	"github.com/ev3rlit/mwosa/providers/core/quote"
	"github.com/ev3rlit/mwosa/service/daily"
	providerservice "github.com/ev3rlit/mwosa/service/providers"
	strategyservice "github.com/ev3rlit/mwosa/service/strategy"
	"github.com/ev3rlit/mwosa/storage"
	dailybarstorage "github.com/ev3rlit/mwosa/storage/dailybar"
	strategystorage "github.com/ev3rlit/mwosa/storage/strategy"
	"github.com/samber/oops"
)

type Options struct {
	Database          string
	Market            provider.Market
	ProviderID        provider.ProviderID
	PreferProvider    provider.ProviderID
	ProviderConfig    provider.Config
	ActivateProviders bool
}

type Runtime struct {
	Storage   StorageRuntime
	Providers ProviderRuntime
	Services  ServiceRuntime
}

type StorageRuntime struct {
	Database   *storage.Database
	DailyBars  DailyBarStorage
	Strategies strategyservice.Repository
}

type DailyBarStorage struct {
	Reader daily.ReadRepository
	Writer daily.WriteRepository
}

type ProviderRuntime struct {
	Registry    *provider.Registry
	Router      *provider.Router
	DailyBars   dailybar.Router
	Quotes      quote.Router
	Instruments instrument.Router
}

type ServiceRuntime struct {
	Daily     DailyServices
	Providers providerservice.Service
	Strategy  strategyservice.Service
}

type DailyServices struct {
	Reader    daily.ReadService
	Collector daily.Service
}

func NewRuntime(opts Options) (*Runtime, error) {
	return NewRuntimeWithProviderBuilders(opts, builtin.Builders()...)
}

func NewRuntimeWithProviderBuilders(opts Options, builders ...provider.ProviderBuilder) (*Runtime, error) {
	errb := oops.In("app_runtime")

	database := storage.NewDatabase(opts.Database)
	reader, writer, err := dailybarstorage.NewRepositories(database)
	if err != nil {
		return nil, errb.Wrapf(err, "create daily bar repositories")
	}
	strategyRepository, err := strategystorage.NewRepository(database)
	if err != nil {
		return nil, errb.Wrapf(err, "create strategy repository")
	}

	registry := provider.NewRegistry()
	if opts.ActivateProviders {
		config := opts.ProviderConfig
		if config == nil {
			config = provider.ConfigFromEnv()
		}
		if err := registry.RegisterConfigured(provider.RegisterOptions{
			ProviderID:     opts.ProviderID,
			PreferProvider: opts.PreferProvider,
		}, config, builders...); err != nil {
			return nil, oops.Join(
				errb.Wrapf(err, "register configured providers"),
				database.Close(),
			)
		}
	}

	coreRouter := provider.NewRouter(registry)
	providerRuntime := ProviderRuntime{
		Registry:    registry,
		Router:      coreRouter,
		DailyBars:   dailybar.NewRouter(coreRouter),
		Quotes:      quote.NewRouter(coreRouter),
		Instruments: instrument.NewRouter(coreRouter),
	}

	dailyReader, err := daily.NewReadService(reader)
	if err != nil {
		return nil, oops.Join(
			errb.Wrapf(err, "create daily read service"),
			database.Close(),
		)
	}
	dailyCollector, err := daily.NewService(reader, writer, providerRuntime.DailyBars)
	if err != nil {
		return nil, oops.Join(
			errb.Wrapf(err, "create daily collect service"),
			database.Close(),
		)
	}
	providersService, err := providerservice.NewService(registry)
	if err != nil {
		return nil, oops.Join(
			errb.Wrapf(err, "create providers service"),
			database.Close(),
		)
	}
	datasetReader, err := strategyservice.NewDailyBarDatasetReader(reader, opts.Market)
	if err != nil {
		return nil, oops.Join(
			errb.Wrapf(err, "create strategy dataset reader"),
			database.Close(),
		)
	}
	strategyService, err := strategyservice.NewService(strategyRepository, datasetReader)
	if err != nil {
		return nil, oops.Join(
			errb.Wrapf(err, "create strategy service"),
			database.Close(),
		)
	}

	return &Runtime{
		Storage: StorageRuntime{
			Database: database,
			DailyBars: DailyBarStorage{
				Reader: reader,
				Writer: writer,
			},
			Strategies: strategyRepository,
		},
		Providers: providerRuntime,
		Services: ServiceRuntime{
			Daily: DailyServices{
				Reader:    dailyReader,
				Collector: dailyCollector,
			},
			Providers: providersService,
			Strategy:  strategyService,
		},
	}, nil
}

func (r *Runtime) Close() error {
	if r == nil || r.Storage.Database == nil {
		return nil
	}
	return r.Storage.Database.Close()
}
