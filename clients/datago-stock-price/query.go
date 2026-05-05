package stockprice

import (
	"net/url"
	"strings"
)

type StockPriceInfoQuery struct {
	NumOfRows       int
	PageNo          int
	Workers         int
	BasDt           string
	BeginBasDt      string
	EndBasDt        string
	LikeBasDt       string
	LikeSrtnCd      string
	IsinCd          string
	LikeIsinCd      string
	ItmsNm          string
	LikeItmsNm      string
	MrktCls         string
	BeginVs         string
	EndVs           string
	BeginFltRt      string
	EndFltRt        string
	BeginTrqu       string
	EndTrqu         string
	BeginTrPrc      string
	EndTrPrc        string
	BeginLstgStCnt  string
	EndLstgStCnt    string
	BeginMrktTotAmt string
	EndMrktTotAmt   string
}

func (q StockPriceInfoQuery) values() url.Values {
	values := url.Values{}
	setIfNotEmpty(values, "basDt", q.BasDt)
	setIfNotEmpty(values, "beginBasDt", q.BeginBasDt)
	setIfNotEmpty(values, "endBasDt", q.EndBasDt)
	setIfNotEmpty(values, "likeBasDt", q.LikeBasDt)
	setIfNotEmpty(values, "likeSrtnCd", q.LikeSrtnCd)
	setIfNotEmpty(values, "isinCd", q.IsinCd)
	setIfNotEmpty(values, "likeIsinCd", q.LikeIsinCd)
	setIfNotEmpty(values, "itmsNm", q.ItmsNm)
	setIfNotEmpty(values, "likeItmsNm", q.LikeItmsNm)
	setIfNotEmpty(values, "mrktCls", q.MrktCls)
	setIfNotEmpty(values, "beginVs", q.BeginVs)
	setIfNotEmpty(values, "endVs", q.EndVs)
	setIfNotEmpty(values, "beginFltRt", q.BeginFltRt)
	setIfNotEmpty(values, "endFltRt", q.EndFltRt)
	setIfNotEmpty(values, "beginTrqu", q.BeginTrqu)
	setIfNotEmpty(values, "endTrqu", q.EndTrqu)
	setIfNotEmpty(values, "beginTrPrc", q.BeginTrPrc)
	setIfNotEmpty(values, "endTrPrc", q.EndTrPrc)
	setIfNotEmpty(values, "beginLstgStCnt", q.BeginLstgStCnt)
	setIfNotEmpty(values, "endLstgStCnt", q.EndLstgStCnt)
	setIfNotEmpty(values, "beginMrktTotAmt", q.BeginMrktTotAmt)
	setIfNotEmpty(values, "endMrktTotAmt", q.EndMrktTotAmt)
	return values
}

func (q StockPriceInfoQuery) WithInstrumentSearch(search string) StockPriceInfoQuery {
	return q.withInstrumentFilter(search, true)
}

func (q StockPriceInfoQuery) WithInstrumentLookup(search string) StockPriceInfoQuery {
	return q.withInstrumentFilter(search, false)
}

func (q StockPriceInfoQuery) withInstrumentFilter(search string, fuzzy bool) StockPriceInfoQuery {
	search = strings.TrimSpace(search)
	if search == "" {
		return q
	}
	switch {
	case looksLikeISIN(search):
		if fuzzy {
			q.LikeIsinCd = search
		} else {
			q.IsinCd = search
		}
	case looksLikeShortCode(search):
		q.LikeSrtnCd = search
	case fuzzy:
		q.LikeItmsNm = search
	default:
		q.ItmsNm = search
	}
	return q
}

func setIfNotEmpty(values url.Values, key string, value string) {
	if value != "" {
		values.Set(key, value)
	}
}

func (q StockPriceInfoQuery) numOfRows() int {
	if q.NumOfRows > 0 {
		return q.NumOfRows
	}
	return DefaultNumOfRows
}

func (q StockPriceInfoQuery) pageNo() int {
	if q.PageNo > 0 {
		return q.PageNo
	}
	return 1
}

func (q StockPriceInfoQuery) forAllPages() StockPriceInfoQuery {
	q.PageNo = 1
	if q.NumOfRows <= 0 {
		q.NumOfRows = DefaultAllNumOfRows
	}
	return q
}

func (q StockPriceInfoQuery) workers() int {
	if q.Workers > 0 {
		return q.Workers
	}
	return 1
}

func looksLikeShortCode(search string) bool {
	if len(search) != 6 {
		return false
	}
	for _, r := range search {
		if !isASCIIAlnum(r) {
			return false
		}
	}
	return true
}

func looksLikeISIN(search string) bool {
	if len(search) != 12 {
		return false
	}
	for index, r := range search {
		switch {
		case index < 2:
			if !isASCIIAlpha(r) {
				return false
			}
		default:
			if !isASCIIAlnum(r) {
				return false
			}
		}
	}
	return true
}

func isASCIIAlpha(r rune) bool {
	return (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')
}

func isASCIIAlnum(r rune) bool {
	return isASCIIAlpha(r) || (r >= '0' && r <= '9')
}
