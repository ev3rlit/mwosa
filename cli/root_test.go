package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersionCommand(t *testing.T) {
	cmd := NewRootCommand(BuildInfo{
		Version: "test-version",
		Commit:  "abc123",
		Date:    "2026-04-26",
	})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"version"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute version: %v", err)
	}

	got := out.String()
	for _, want := range []string{
		"mwosa test-version",
		"schema dev",
		"commit abc123",
		"built 2026-04-26",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("version output missing %q in:\n%s", want, got)
		}
	}
}

func TestRootHelpHasOutputFlag(t *testing.T) {
	cmd := NewRootCommand(BuildInfo{})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute help: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "--output") {
		t.Fatalf("help output should include --output flag:\n%s", got)
	}
}
