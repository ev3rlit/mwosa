package corpfin

import (
	"bytes"
	"encoding/json"
	"strconv"

	"github.com/samber/oops"
)

type SummaryFinancialStatement struct {
	BasDt         string
	BizYear       string
	Crno          string
	CurCd         string
	EnpBzopPft    string
	EnpCptlAmt    string
	EnpCrtmNpf    string
	EnpSaleAmt    string
	EnpTastAmt    string
	EnpTdbtAmt    string
	EnpTcptAmt    string
	FnclDcd       string
	FnclDcdNm     string
	FnclDebtRto   string
	IclsPalClcAmt string

	fields map[string]string
}

type AccountStatementItem struct {
	AcitID       string
	AcitNm       string
	BasDt        string
	BizYear      string
	BpvtrAcitAmt string
	Crno         string
	CrtmAcitAmt  string
	CurCd        string
	FnclDcd      string
	FnclDcdNm    string
	LsqtAcitAmt  string
	PvtrAcitAmt  string
	ThqrAcitAmt  string

	fields map[string]string
}

type BalanceSheetItem struct {
	AccountStatementItem
}

type IncomeStatementItem struct {
	AccountStatementItem
}

func (i *SummaryFinancialStatement) UnmarshalJSON(data []byte) error {
	fields, err := decodeStringFields(data)
	if err != nil {
		return err
	}
	i.BasDt = fields["basDt"]
	i.BizYear = fields["bizYear"]
	i.Crno = fields["crno"]
	i.CurCd = fields["curCd"]
	i.EnpBzopPft = fields["enpBzopPft"]
	i.EnpCptlAmt = fields["enpCptlAmt"]
	i.EnpCrtmNpf = fields["enpCrtmNpf"]
	i.EnpSaleAmt = fields["enpSaleAmt"]
	i.EnpTastAmt = fields["enpTastAmt"]
	i.EnpTdbtAmt = fields["enpTdbtAmt"]
	i.EnpTcptAmt = fields["enpTcptAmt"]
	i.FnclDcd = fields["fnclDcd"]
	i.FnclDcdNm = fields["fnclDcdNm"]
	i.FnclDebtRto = fields["fnclDebtRto"]
	i.IclsPalClcAmt = fields["iclsPalClcAmt"]
	i.fields = fields
	return nil
}

func (i SummaryFinancialStatement) Fields() map[string]string {
	return cloneStringMap(i.fields)
}

func (i AccountStatementItem) Fields() map[string]string {
	return cloneStringMap(i.fields)
}

func (i *BalanceSheetItem) UnmarshalJSON(data []byte) error {
	item, err := accountStatementItemFromJSON(data)
	if err != nil {
		return err
	}
	i.AccountStatementItem = item
	return nil
}

func (i BalanceSheetItem) Fields() map[string]string {
	return cloneStringMap(i.fields)
}

func (i *IncomeStatementItem) UnmarshalJSON(data []byte) error {
	item, err := accountStatementItemFromJSON(data)
	if err != nil {
		return err
	}
	i.AccountStatementItem = item
	return nil
}

func (i IncomeStatementItem) Fields() map[string]string {
	return cloneStringMap(i.fields)
}

func accountStatementItemFromJSON(data []byte) (AccountStatementItem, error) {
	fields, err := decodeStringFields(data)
	if err != nil {
		return AccountStatementItem{}, err
	}
	return AccountStatementItem{
		AcitID:       fields["acitId"],
		AcitNm:       fields["acitNm"],
		BasDt:        fields["basDt"],
		BizYear:      fields["bizYear"],
		BpvtrAcitAmt: fields["bpvtrAcitAmt"],
		Crno:         fields["crno"],
		CrtmAcitAmt:  fields["crtmAcitAmt"],
		CurCd:        fields["curCd"],
		FnclDcd:      fields["fnclDcd"],
		FnclDcdNm:    fields["fnclDcdNm"],
		LsqtAcitAmt:  fields["lsqtAcitAmt"],
		PvtrAcitAmt:  fields["pvtrAcitAmt"],
		ThqrAcitAmt:  fields["thqrAcitAmt"],
		fields:       fields,
	}, nil
}

func decodeStringFields(data []byte) (map[string]string, error) {
	errb := oops.In("datago_client")
	var raw map[string]any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&raw); err != nil {
		return nil, errb.Wrapf(err, "decode datago corporate finance item")
	}

	fields := make(map[string]string, len(raw))
	for key, value := range raw {
		text, err := jsonValueAsString(value)
		if err != nil {
			return nil, errb.With("field", key).Wrapf(err, "decode datago corporate finance field")
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
		return "", oops.In("datago_client").With("value_type", typed).New("unsupported datago corporate finance field shape")
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
