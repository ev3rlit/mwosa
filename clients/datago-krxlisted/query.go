package krxlisted

import "net/url"

type Query struct {
	NumOfRows  int
	PageNo     int
	BasDt      string
	BeginBasDt string
	EndBasDt   string
	LikeBasDt  string
	LikeSrtnCd string
	IsinCd     string
	LikeIsinCd string
	ItmsNm     string
	LikeItmsNm string
	Crno       string
	CorpNm     string
	LikeCorpNm string
}

func (q Query) values() url.Values {
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
	setIfNotEmpty(values, "crno", q.Crno)
	setIfNotEmpty(values, "corpNm", q.CorpNm)
	setIfNotEmpty(values, "likeCorpNm", q.LikeCorpNm)
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
