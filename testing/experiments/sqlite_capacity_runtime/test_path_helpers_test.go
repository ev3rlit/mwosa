package sqlite_capacity_runtime

import (
	"os"
	"path/filepath"
	"testing"
)

func resolveProbeOutputDir(t *testing.T, configuredPath, tempName string) string {
	t.Helper()
	if configuredPath == "" {
		return filepath.Join(t.TempDir(), tempName)
	}
	if filepath.IsAbs(configuredPath) {
		return configuredPath
	}
	return filepath.Join(repoRoot(t), configuredPath)
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find repo root from %s", dir)
		}
		dir = parent
	}
}
