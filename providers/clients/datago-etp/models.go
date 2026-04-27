package etp

import (
	"bytes"
	"encoding/json"
	"strconv"

	"github.com/samber/oops"
)

type CommonPriceInfo struct {
	BasDt      string
	SrtnCd     string
	IsinCd     string
	ItmsNm     string
	Clpr       string
	Vs         string
	FltRt      string
	Mkp        string
	Hipr       string
	Lopr       string
	Trqu       string
	TrPrc      string
	MrktTotAmt string
}

type ETFPriceInfo struct {
	CommonPriceInfo

	NPptTotAmt  string
	StLstgCnt   string
	Nav         string
	BssIdxIdxNm string
	BssIdxClpr  string

	fields map[string]string
}

type ETNPriceInfo struct {
	CommonPriceInfo

	IndcVal       string
	IndcValTotAmt string
	LstgScrtCnt   string
	BssIdxIdxNm   string
	BssIdxClpr    string

	fields map[string]string
}

type ELWPriceInfo struct {
	CommonPriceInfo

	LstgScrtCnt string
	UdasAstNm   string
	UdasAstClpr string

	fields map[string]string
}

func (i *ETFPriceInfo) UnmarshalJSON(data []byte) error {
	fields, err := decodeStringFields(data)
	if err != nil {
		return err
	}
	i.CommonPriceInfo = commonPriceInfoFromFields(fields)
	i.NPptTotAmt = fields["nPptTotAmt"]
	i.StLstgCnt = fields["stLstgCnt"]
	i.Nav = fields["nav"]
	i.BssIdxIdxNm = fields["bssIdxIdxNm"]
	i.BssIdxClpr = fields["bssIdxClpr"]
	i.fields = fields
	return nil
}

func (i ETFPriceInfo) Fields() map[string]string {
	return cloneStringMap(i.fields)
}

func (i *ETNPriceInfo) UnmarshalJSON(data []byte) error {
	fields, err := decodeStringFields(data)
	if err != nil {
		return err
	}
	i.CommonPriceInfo = commonPriceInfoFromFields(fields)
	i.IndcVal = fields["indcVal"]
	i.IndcValTotAmt = fields["indcValTotAmt"]
	i.LstgScrtCnt = fields["lstgScrtCnt"]
	i.BssIdxIdxNm = fields["bssIdxIdxNm"]
	i.BssIdxClpr = fields["bssIdxClpr"]
	i.fields = fields
	return nil
}

func (i ETNPriceInfo) Fields() map[string]string {
	return cloneStringMap(i.fields)
}

func (i *ELWPriceInfo) UnmarshalJSON(data []byte) error {
	fields, err := decodeStringFields(data)
	if err != nil {
		return err
	}
	i.CommonPriceInfo = commonPriceInfoFromFields(fields)
	i.LstgScrtCnt = fields["lstgScrtCnt"]
	i.UdasAstNm = fields["udasAstNm"]
	i.UdasAstClpr = fields["udasAstClpr"]
	i.fields = fields
	return nil
}

func (i ELWPriceInfo) Fields() map[string]string {
	return cloneStringMap(i.fields)
}

func commonPriceInfoFromFields(fields map[string]string) CommonPriceInfo {
	return CommonPriceInfo{
		BasDt:      fields["basDt"],
		SrtnCd:     fields["srtnCd"],
		IsinCd:     fields["isinCd"],
		ItmsNm:     fields["itmsNm"],
		Clpr:       fields["clpr"],
		Vs:         fields["vs"],
		FltRt:      fields["fltRt"],
		Mkp:        fields["mkp"],
		Hipr:       fields["hipr"],
		Lopr:       fields["lopr"],
		Trqu:       fields["trqu"],
		TrPrc:      fields["trPrc"],
		MrktTotAmt: fields["mrktTotAmt"],
	}
}

func decodeStringFields(data []byte) (map[string]string, error) {
	errb := oops.In("datago_client")
	var raw map[string]any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&raw); err != nil {
		return nil, errb.Wrapf(err, "decode datago price info item")
	}

	fields := make(map[string]string, len(raw))
	for key, value := range raw {
		text, err := jsonValueAsString(value)
		if err != nil {
			return nil, errb.With("field", key).Wrapf(err, "decode datago price info field")
		}
		if text != "" {
			fields[key] = text
		}
	}
	return fields, nil
}

func jsonValueAsString(value any) (string, error) {
	switch typed := value.(type) {
	case nil:
		return "", nil
	case string:
		return typed, nil
	case json.Number:
		return typed.String(), nil
	case bool:
		return strconv.FormatBool(typed), nil
	default:
		return "", oops.In("datago_client").With("value_type", typed).New("unsupported datago price info field shape")
	}
}

func cloneStringMap(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}
