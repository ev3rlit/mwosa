package core

import (
	"errors"
	"fmt"
	"strings"
)

var ErrNoProvider = errors.New("no provider candidate")

type UnsupportedError struct {
	Capability   Role
	ProviderID   ProviderID
	GroupID      GroupID
	OperationID  OperationID
	Market       Market
	SecurityType SecurityType
	Symbol       string
	Reason       string
}

func (e *UnsupportedError) Error() string {
	parts := []string{"unsupported provider capability"}
	if e.Capability != "" {
		parts = append(parts, fmt.Sprintf("capability=%s", e.Capability))
	}
	if e.ProviderID != "" {
		parts = append(parts, fmt.Sprintf("provider=%s", e.ProviderID))
	}
	if e.GroupID != "" {
		parts = append(parts, fmt.Sprintf("group=%s", e.GroupID))
	}
	if e.OperationID != "" {
		parts = append(parts, fmt.Sprintf("operation=%s", e.OperationID))
	}
	if e.Market != "" {
		parts = append(parts, fmt.Sprintf("market=%s", e.Market))
	}
	if e.SecurityType != "" {
		parts = append(parts, fmt.Sprintf("security_type=%s", e.SecurityType))
	}
	if e.Symbol != "" {
		parts = append(parts, fmt.Sprintf("symbol=%s", e.Symbol))
	}
	if e.Reason != "" {
		parts = append(parts, fmt.Sprintf("reason=%s", e.Reason))
	}
	return strings.Join(parts, " ")
}

func (e *UnsupportedError) Is(target error) bool {
	_, ok := target.(*UnsupportedError)
	return ok
}

func NewUnsupported(input UnsupportedError) error {
	return &input
}
