package krxlisted

import (
	"bytes"
	"encoding/json"
	"strconv"

	"github.com/samber/oops"
)

type ListedItem struct {
	BasDt   string
	SrtnCd  string
	IsinCd  string
	MrktCtg string
	ItmsNm  string
	Crno    string
	CorpNm  string

	fields map[string]string
}

func (i *ListedItem) UnmarshalJSON(data []byte) error {
	fields, err := decodeStringFields(data)
	if err != nil {
		return err
	}
	i.BasDt = fields["basDt"]
	i.SrtnCd = fields["srtnCd"]
	i.IsinCd = fields["isinCd"]
	i.MrktCtg = fields["mrktCtg"]
	i.ItmsNm = fields["itmsNm"]
	i.Crno = fields["crno"]
	i.CorpNm = fields["corpNm"]
	i.fields = fields
	return nil
}

func (i ListedItem) Fields() map[string]string {
	return cloneStringMap(i.fields)
}

func decodeStringFields(data []byte) (map[string]string, error) {
	errb := oops.In("datago_client")
	var raw map[string]any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&raw); err != nil {
		return nil, errb.Wrapf(err, "decode datago krx listed item")
	}

	fields := make(map[string]string, len(raw))
	for key, value := range raw {
		text, err := jsonValueAsString(value)
		if err != nil {
			return nil, errb.With("field", key).Wrapf(err, "decode datago krx listed field")
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
		return "", oops.In("datago_client").With("value_type", typed).New("unsupported datago krx listed field shape")
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
