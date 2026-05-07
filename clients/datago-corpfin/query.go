package corpfin

import "net/url"

type Query struct {
	NumOfRows int
	PageNo    int
	Crno      string
	BizYear   string
}

func (q Query) values() url.Values {
	values := url.Values{}
	setIfNotEmpty(values, "crno", q.Crno)
	setIfNotEmpty(values, "bizYear", q.BizYear)
	return values
}

func setIfNotEmpty(values url.Values, key string, value string) {
	if value != "" {
		values.Set(key, value)
	}
}

func (q Query) numOfRows() int {
	if q.NumOfRows > 0 {
		return q.NumOfRows
	}
	return DefaultNumOfRows
}

func (q Query) pageNo() int {
	if q.PageNo > 0 {
		return q.PageNo
	}
	return 1
}

func (q Query) forAllPages() Query {
	q.PageNo = 1
	if q.NumOfRows <= 0 {
		q.NumOfRows = DefaultAllNumOfRows
	}
	return q
}
