package builtin

import (
	provider "github.com/ev3rlit/mwosa/providers/core"
	"github.com/ev3rlit/mwosa/providers/datago"
)

func Builders() []provider.ProviderBuilder {
	return []provider.ProviderBuilder{
		datago.NewBuilder(),
		datago.NewCorporateFinanceBuilder(),
	}
}
