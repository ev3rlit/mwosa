package daily

import (
	"time"

	"github.com/samber/oops"
)

const (
	apiDateLayout = "20060102"
	isoDateLayout = "2006-01-02"
)

func parseDate(value string, field string) (time.Time, error) {
	if value == "" {
		return time.Time{}, oops.In("daily_service").With("field", field).Errorf("%s is required", field)
	}
	for _, layout := range []string{apiDateLayout, isoDateLayout} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, oops.In("daily_service").With("field", field, "value", value).Errorf("%s must be YYYYMMDD or YYYY-MM-DD: %q", field, value)
}

func parseOptionalDate(value string, field string) (time.Time, bool, error) {
	if value == "" {
		return time.Time{}, false, nil
	}
	parsed, err := parseDate(value, field)
	if err != nil {
		return time.Time{}, false, err
	}
	return parsed, true, nil
}

func resolveDateRange(from string, to string, asOf string) ([]time.Time, error) {
	if asOf != "" {
		if from != "" || to != "" {
			return nil, oops.In("daily_service").With("as_of", asOf, "from", from, "to", to).New("--as-of cannot be combined with --from or --to")
		}
		date, err := parseDate(asOf, "--as-of")
		if err != nil {
			return nil, err
		}
		return []time.Time{date}, nil
	}

	fromDate, hasFrom, err := parseOptionalDate(from, "--from")
	if err != nil {
		return nil, err
	}
	toDate, hasTo, err := parseOptionalDate(to, "--to")
	if err != nil {
		return nil, err
	}

	switch {
	case !hasFrom && !hasTo:
		return nil, nil
	case hasFrom && !hasTo:
		toDate = fromDate
	case !hasFrom && hasTo:
		fromDate = toDate
	}

	if fromDate.After(toDate) {
		return nil, oops.In("daily_service").With("from", from, "to", to).New("--from must be on or before --to")
	}

	dates := make([]time.Time, 0)
	for date := fromDate; !date.After(toDate); date = date.AddDate(0, 0, 1) {
		dates = append(dates, date)
	}
	return dates, nil
}

func apiDate(date time.Time) string {
	return date.Format(apiDateLayout)
}

func isoDate(date time.Time) string {
	return date.Format(isoDateLayout)
}
