package etp

import (
	"net/url"
	"strings"
)

type SecuritiesProductPriceQuery struct {
	NumOfRows       int
	PageNo          int
	BasDt           string
	BeginBasDt      string
	EndBasDt        string
	LikeBasDt       string
	LikeSrtnCd      string
	IsinCd          string
	LikeIsinCd      string
	ItmsNm          string
	LikeItmsNm      string
	BeginVs         string
	EndVs           string
	BeginTrqu       string
	EndTrqu         string
	BeginTrPrc      string
	EndTrPrc        string
	BeginMrktTotAmt string
	EndMrktTotAmt   string
}

type ETFPriceInfoQuery struct {
	SecuritiesProductPriceQuery

	BeginFltRt      string
	EndFltRt        string
	BeginNav        string
	EndNav          string
	BssIdxIdxNm     string
	LikeBssIdxIdxNm string
}

type ETNPriceInfoQuery struct {
	SecuritiesProductPriceQuery

	BeginFltRt      string
	EndFltRt        string
	BeginIndcVal    string
	EndIndcVal      string
	BssIdxIdxNm     string
	LikeBssIdxIdxNm string
}

type ELWPriceInfoQuery struct {
	SecuritiesProductPriceQuery

	UdasAstNm     string
	LikeUdasAstNm string
}

func (q SecuritiesProductPriceQuery) values() url.Values {
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
	setIfNotEmpty(values, "beginVs", q.BeginVs)
	setIfNotEmpty(values, "endVs", q.EndVs)
	setIfNotEmpty(values, "beginTrqu", q.BeginTrqu)
	setIfNotEmpty(values, "endTrqu", q.EndTrqu)
	setIfNotEmpty(values, "beginTrPrc", q.BeginTrPrc)
	setIfNotEmpty(values, "endTrPrc", q.EndTrPrc)
	setIfNotEmpty(values, "beginMrktTotAmt", q.BeginMrktTotAmt)
	setIfNotEmpty(values, "endMrktTotAmt", q.EndMrktTotAmt)
	return values
}

func (q SecuritiesProductPriceQuery) WithInstrumentSearch(search string) SecuritiesProductPriceQuery {
	return q.withInstrumentFilter(search, true)
}

func (q SecuritiesProductPriceQuery) WithInstrumentLookup(search string) SecuritiesProductPriceQuery {
	return q.withInstrumentFilter(search, false)
}

func (q SecuritiesProductPriceQuery) withInstrumentFilter(search string, fuzzy bool) SecuritiesProductPriceQuery {
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

func (q ETFPriceInfoQuery) values() url.Values {
	values := q.SecuritiesProductPriceQuery.values()
	setIfNotEmpty(values, "beginFltRt", q.BeginFltRt)
	setIfNotEmpty(values, "endFltRt", q.EndFltRt)
	setIfNotEmpty(values, "beginNav", q.BeginNav)
	setIfNotEmpty(values, "endNav", q.EndNav)
	setIfNotEmpty(values, "bssIdxIdxNm", q.BssIdxIdxNm)
	setIfNotEmpty(values, "likeBssIdxIdxNm", q.LikeBssIdxIdxNm)
	return values
}

func (q ETNPriceInfoQuery) values() url.Values {
	values := q.SecuritiesProductPriceQuery.values()
	setIfNotEmpty(values, "beginFltRt", q.BeginFltRt)
	setIfNotEmpty(values, "endFltRt", q.EndFltRt)
	setIfNotEmpty(values, "beginIndcVal", q.BeginIndcVal)
	setIfNotEmpty(values, "endIndcVal", q.EndIndcVal)
	setIfNotEmpty(values, "bssIdxIdxNm", q.BssIdxIdxNm)
	setIfNotEmpty(values, "likeBssIdxIdxNm", q.LikeBssIdxIdxNm)
	return values
}

func (q ELWPriceInfoQuery) values() url.Values {
	values := q.SecuritiesProductPriceQuery.values()
	setIfNotEmpty(values, "udasAstNm", q.UdasAstNm)
	setIfNotEmpty(values, "likeUdasAstNm", q.LikeUdasAstNm)
	return values
}

func setIfNotEmpty(values url.Values, key string, value string) {
	if value != "" {
		values.Set(key, value)
	}
}

func (q SecuritiesProductPriceQuery) numOfRows() int {
	if q.NumOfRows > 0 {
		return q.NumOfRows
	}
	return DefaultNumOfRows
}

func (q SecuritiesProductPriceQuery) pageNo() int {
	if q.PageNo > 0 {
		return q.PageNo
	}
	return 1
}

func (q SecuritiesProductPriceQuery) forAllPages() SecuritiesProductPriceQuery {
	q.PageNo = 1
	if q.NumOfRows <= 0 {
		q.NumOfRows = DefaultAllNumOfRows
	}
	return q
}

func (q SecuritiesProductPriceQuery) forMetadataProbe() (SecuritiesProductPriceQuery, int) {
	pageSize := q.NumOfRows
	if pageSize <= 0 {
		pageSize = DefaultAllNumOfRows
	}
	q.PageNo = 1
	q.NumOfRows = 1
	return q, pageSize
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
