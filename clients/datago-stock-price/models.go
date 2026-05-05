package stockprice

import (
	"bytes"
	"encoding/json"
	"strconv"

	"github.com/samber/oops"
)

type StockPriceInfo struct {
	BasDt      string
	SrtnCd     string
	IsinCd     string
	ItmsNm     string
	MrktCtg    string
	MrktCls    string
	Clpr       string
	Vs         string
	FltRt      string
	Mkp        string
	Hipr       string
	Lopr       string
	Trqu       string
	TrPrc      string
	LstgStCnt  string
	MrktTotAmt string

	fields map[string]string
}

func (i *StockPriceInfo) UnmarshalJSON(data []byte) error {
	fields, err := decodeStringFields(data)
	if err != nil {
		return err
	}
	i.BasDt = fields["basDt"]
	i.SrtnCd = fields["srtnCd"]
	i.IsinCd = fields["isinCd"]
	i.ItmsNm = fields["itmsNm"]
	i.MrktCtg = fields["mrktCtg"]
	i.MrktCls = fields["mrktCls"]
	i.Clpr = fields["clpr"]
	i.Vs = fields["vs"]
	i.FltRt = fields["fltRt"]
	i.Mkp = fields["mkp"]
	i.Hipr = fields["hipr"]
	i.Lopr = fields["lopr"]
	i.Trqu = fields["trqu"]
	i.TrPrc = fields["trPrc"]
	i.LstgStCnt = fields["lstgStCnt"]
	i.MrktTotAmt = fields["mrktTotAmt"]
	i.fields = fields
	return nil
}

func (i StockPriceInfo) Fields() map[string]string {
	return cloneStringMap(i.fields)
}

func decodeStringFields(data []byte) (map[string]string, error) {
	errb := oops.In("datago_stock_price_client")
	var raw map[string]any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&raw); err != nil {
		return nil, errb.Wrapf(err, "decode datago stock price info item")
	}

	fields := make(map[string]string, len(raw))
	for key, value := range raw {
		text, err := jsonValueAsString(value)
		if err != nil {
			return nil, errb.With("field", key).Wrapf(err, "decode datago stock price info field")
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
		return "", oops.In("datago_stock_price_client").With("value_type", typed).New("unsupported datago stock price info field shape")
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
