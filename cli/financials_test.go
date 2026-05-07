package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetFinancialsHasCommandSurface(t *testing.T) {
	cmd := NewRootCommand(BuildInfo{})
	get, _, err := cmd.Find([]string{"get", "financials"})
	if err != nil {
		t.Fatalf("find get financials: %v", err)
	}
	if get == nil || get.Use != "financials <company>" {
		t.Fatalf("get financials command = %#v", get)
	}
}

func TestGetFinancialsWithoutProviderReportsFinancialsCapability(t *testing.T) {
	var out bytes.Buffer
	cmd := NewRootCommand(BuildInfo{})
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := executeForTest(t, context.Background(), cmd,
		"--database", filepath.Join(t.TempDir(), "mwosa.db"),
		"get", "financials", "005930",
		"--year", "2025",
	)
	if err == nil {
		t.Fatal("get financials error = nil, want no provider error")
	}
	for _, want := range []string{"financials", "no provider candidate", "symbol=005930"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error missing %q in %q", want, err.Error())
		}
	}
}
