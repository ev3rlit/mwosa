package files

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	provider "github.com/ev3rlit/mwosa/providers/core"
	"github.com/ev3rlit/mwosa/providers/core/dailybar"
	"github.com/ev3rlit/mwosa/service/daily"
)

type DailyBarStore struct {
	root string
}

func NewDailyBarStore(root string) *DailyBarStore {
	return &DailyBarStore{root: root}
}

func (s *DailyBarStore) UpsertDailyBars(ctx context.Context, bars []dailybar.Bar) (daily.WriteResult, error) {
	if s == nil || s.root == "" {
		return daily.WriteResult{}, fmt.Errorf("daily bar store root is empty")
	}

	grouped := make(map[string][]dailybar.Bar)
	for _, bar := range bars {
		if err := ctx.Err(); err != nil {
			return daily.WriteResult{}, err
		}
		if bar.Market == "" || bar.SecurityType == "" || bar.TradingDate == "" {
			return daily.WriteResult{}, fmt.Errorf("daily bar missing storage key market=%s security_type=%s date=%s symbol=%s", bar.Market, bar.SecurityType, bar.TradingDate, bar.Symbol)
		}
		path := s.pathFor(bar.Market, bar.SecurityType, bar.TradingDate)
		grouped[path] = append(grouped[path], bar)
	}

	result := daily.WriteResult{BarsWritten: len(bars)}
	for path, incoming := range grouped {
		if err := upsertFile(path, incoming); err != nil {
			return daily.WriteResult{}, err
		}
		result.FilesWritten++
	}
	return result, nil
}

func (s *DailyBarStore) QueryDailyBars(ctx context.Context, query daily.Query) ([]dailybar.Bar, error) {
	if s == nil || s.root == "" {
		return nil, fmt.Errorf("daily bar store root is empty")
	}

	paths, err := s.queryPaths(query)
	if err != nil {
		return nil, err
	}

	bars := make([]dailybar.Bar, 0)
	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		fileBars, err := readBars(path)
		if err != nil {
			return nil, err
		}
		for _, bar := range fileBars {
			if matchesQuery(bar, query) {
				bars = append(bars, bar)
			}
		}
	}

	sort.SliceStable(bars, func(i, j int) bool {
		if bars[i].TradingDate != bars[j].TradingDate {
			return bars[i].TradingDate < bars[j].TradingDate
		}
		return bars[i].Symbol < bars[j].Symbol
	})
	return bars, nil
}

func (s *DailyBarStore) queryPaths(query daily.Query) ([]string, error) {
	market := query.Market
	if market == "" {
		market = provider.MarketKRX
	}

	if query.From != "" || query.To != "" {
		from, to, err := resolveQueryRange(query.From, query.To)
		if err != nil {
			return nil, err
		}
		paths := make([]string, 0)
		for date := from; !date.After(to); date = date.AddDate(0, 0, 1) {
			dateValue := date.Format("2006-01-02")
			if query.SecurityType != "" {
				path := s.pathFor(market, query.SecurityType, dateValue)
				if fileExists(path) {
					paths = append(paths, path)
				}
				continue
			}
			securityPaths, err := filepath.Glob(filepath.Join(s.root, "daily_bar", fmt.Sprintf("market=%s", market), "security_type=*", fmt.Sprintf("date=%s.ndjson", dateValue)))
			if err != nil {
				return nil, err
			}
			paths = append(paths, securityPaths...)
		}
		sort.Strings(paths)
		return paths, nil
	}

	var pattern string
	if query.SecurityType != "" {
		pattern = filepath.Join(s.root, "daily_bar", fmt.Sprintf("market=%s", market), fmt.Sprintf("security_type=%s", query.SecurityType), "date=*.ndjson")
	} else {
		pattern = filepath.Join(s.root, "daily_bar", fmt.Sprintf("market=%s", market), "security_type=*", "date=*.ndjson")
	}
	paths, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	return paths, nil
}

func (s *DailyBarStore) pathFor(market provider.Market, securityType provider.SecurityType, tradingDate string) string {
	return filepath.Join(
		s.root,
		"daily_bar",
		fmt.Sprintf("market=%s", market),
		fmt.Sprintf("security_type=%s", securityType),
		fmt.Sprintf("date=%s.ndjson", tradingDate),
	)
}

func upsertFile(path string, incoming []dailybar.Bar) error {
	existing, err := readBars(path)
	if err != nil {
		return err
	}

	byKey := make(map[string]dailybar.Bar, len(existing)+len(incoming))
	for _, bar := range existing {
		byKey[barKey(bar)] = bar
	}
	for _, bar := range incoming {
		byKey[barKey(bar)] = bar
	}

	bars := make([]dailybar.Bar, 0, len(byKey))
	for _, bar := range byKey {
		bars = append(bars, bar)
	}
	sort.SliceStable(bars, func(i, j int) bool {
		return barKey(bars[i]) < barKey(bars[j])
	})

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create daily bar storage directory: %w", err)
	}
	tmpPath := path + ".tmp"
	file, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create daily bar storage file: %w", err)
	}
	encoder := json.NewEncoder(file)
	for _, bar := range bars {
		if err := encoder.Encode(bar); err != nil {
			_ = file.Close()
			return fmt.Errorf("write daily bar storage file: %w", err)
		}
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close daily bar storage file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("replace daily bar storage file: %w", err)
	}
	return nil
}

func readBars(path string) ([]dailybar.Bar, error) {
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open daily bar storage file %s: %w", path, err)
	}
	defer file.Close()

	bars := make([]dailybar.Bar, 0)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var bar dailybar.Bar
		if err := json.Unmarshal([]byte(line), &bar); err != nil {
			return nil, fmt.Errorf("decode daily bar storage file %s: %w", path, err)
		}
		bars = append(bars, bar)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read daily bar storage file %s: %w", path, err)
	}
	return bars, nil
}

func matchesQuery(bar dailybar.Bar, query daily.Query) bool {
	if query.Market != "" && bar.Market != query.Market {
		return false
	}
	if query.SecurityType != "" && bar.SecurityType != query.SecurityType {
		return false
	}
	if query.Symbol != "" && bar.Symbol != query.Symbol {
		return false
	}
	if query.From != "" && bar.TradingDate < query.From {
		return false
	}
	if query.To != "" && bar.TradingDate > query.To {
		return false
	}
	return true
}

func barKey(bar dailybar.Bar) string {
	return fmt.Sprintf("%s/%s/%s/%s/%s/%s", bar.Market, bar.SecurityType, bar.TradingDate, bar.Symbol, bar.Provider, bar.Group)
}

func resolveQueryRange(from string, to string) (time.Time, time.Time, error) {
	if from == "" {
		from = to
	}
	if to == "" {
		to = from
	}
	fromDate, err := time.Parse("2006-01-02", from)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("query from must be YYYY-MM-DD: %q", from)
	}
	toDate, err := time.Parse("2006-01-02", to)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("query to must be YYYY-MM-DD: %q", to)
	}
	if fromDate.After(toDate) {
		return time.Time{}, time.Time{}, fmt.Errorf("query from must be on or before query to")
	}
	return fromDate, toDate, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
