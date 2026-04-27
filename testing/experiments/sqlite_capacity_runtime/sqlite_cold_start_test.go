package sqlite_capacity_runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

const sqliteColdStartProbeConfigJSON = `
{
  "enabled": true,
  "runs": 25,
  "warmup_runs": 5,
  "timeout_seconds": 180,
  "sqlite_module_version": "v1.50.0"
}
`

type sqliteColdStartProbeConfig struct {
	Enabled             bool   `json:"enabled"`
	Runs                int    `json:"runs"`
	WarmupRuns          int    `json:"warmup_runs"`
	TimeoutSeconds      int    `json:"timeout_seconds"`
	SQLiteModuleVersion string `json:"sqlite_module_version"`
}

type coldStartProbeCase struct {
	Name      string
	Source    string
	Arguments []string
}

type coldStartStats struct {
	Min    time.Duration
	P50    time.Duration
	Mean   time.Duration
	Max    time.Duration
	Sample []time.Duration
}

func TestSQLiteColdStartCost(t *testing.T) {
	config := loadSQLiteColdStartProbeConfig(t)
	if !config.Enabled {
		t.Skip("set enabled=true in sqliteColdStartProbeConfigJSON to measure SQLite cold start cost")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skipf("go command is required for cold start probe: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(positiveOrDefault(config.TimeoutSeconds, 180))*time.Second)
	defer cancel()

	workDir := t.TempDir()
	writeColdStartProbeModule(t, workDir, config)

	cases := []coldStartProbeCase{
		{
			Name: "baseline_no_sqlite",
			Source: `package main

func main() {}
`,
		},
		{
			Name: "sqlite_linked_unopened",
			Source: `package main

import _ "modernc.org/sqlite"

func main() {}
`,
		},
		{
			Name: "sqlite_open_memory",
			Source: `package main

import (
	"database/sql"
	"log"

	_ "modernc.org/sqlite"
)

func main() {
	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec("SELECT 1"); err != nil {
		log.Fatal(err)
	}
}
`,
		},
		{
			Name:      "sqlite_open_file",
			Arguments: []string{filepath.Join(workDir, "probe.db")},
			Source: `package main

import (
	"database/sql"
	"log"
	"os"

	_ "modernc.org/sqlite"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("expected sqlite database path")
	}
	db, err := sql.Open("sqlite", os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec("CREATE TABLE IF NOT EXISTS probe (id INTEGER PRIMARY KEY, value TEXT)"); err != nil {
		log.Fatal(err)
	}
}
`,
		},
	}

	for _, probeCase := range cases {
		writeColdStartProbeCommand(t, workDir, probeCase)
	}

	binDir := filepath.Join(workDir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}

	for _, probeCase := range cases {
		binaryPath := filepath.Join(binDir, probeCase.Name)
		buildColdStartProbeBinary(t, ctx, workDir, probeCase.Name, binaryPath)

		stats := measureColdStartProbeBinary(t, ctx, binaryPath, probeCase.Arguments, config)
		binaryBytes, err := fileSize(binaryPath)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("%s binary_bytes=%d runs=%d warmup_runs=%d min=%s p50=%s mean=%s max=%s",
			probeCase.Name,
			binaryBytes,
			positiveOrDefault(config.Runs, 25),
			positiveOrDefault(config.WarmupRuns, 5),
			formatDuration(stats.Min),
			formatDuration(stats.P50),
			formatDuration(stats.Mean),
			formatDuration(stats.Max),
		)
	}
}

func loadSQLiteColdStartProbeConfig(t *testing.T) sqliteColdStartProbeConfig {
	t.Helper()
	var config sqliteColdStartProbeConfig
	if err := json.Unmarshal([]byte(sqliteColdStartProbeConfigJSON), &config); err != nil {
		t.Fatalf("decode sqliteColdStartProbeConfigJSON: %v", err)
	}
	if config.SQLiteModuleVersion == "" {
		config.SQLiteModuleVersion = "v1.50.0"
	}
	return config
}

func writeColdStartProbeModule(t *testing.T, workDir string, config sqliteColdStartProbeConfig) {
	t.Helper()
	goMod := fmt.Sprintf(`module mwosa_sqlite_cold_start_probe

go 1.25.6

require modernc.org/sqlite %s
`, config.SQLiteModuleVersion)
	if err := os.WriteFile(filepath.Join(workDir, "go.mod"), []byte(goMod), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeColdStartProbeCommand(t *testing.T, workDir string, probeCase coldStartProbeCase) {
	t.Helper()
	commandDir := filepath.Join(workDir, "cmd", probeCase.Name)
	if err := os.MkdirAll(commandDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(commandDir, "main.go"), []byte(probeCase.Source), 0o644); err != nil {
		t.Fatal(err)
	}
}

func buildColdStartProbeBinary(t *testing.T, ctx context.Context, workDir, name, binaryPath string) {
	t.Helper()
	command := exec.CommandContext(ctx, "go", "build", "-mod=mod", "-trimpath", "-o", binaryPath, "./cmd/"+name)
	command.Dir = workDir
	command.Env = append(os.Environ(), "GOWORK=off")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("build %s: %v\n%s", name, err, output)
	}
}

func measureColdStartProbeBinary(t *testing.T, ctx context.Context, binaryPath string, arguments []string, config sqliteColdStartProbeConfig) coldStartStats {
	t.Helper()
	warmupRuns := positiveOrDefault(config.WarmupRuns, 5)
	runs := positiveOrDefault(config.Runs, 25)

	for range warmupRuns {
		runColdStartProbeOnce(t, ctx, binaryPath, arguments)
	}

	samples := make([]time.Duration, 0, runs)
	for range runs {
		samples = append(samples, runColdStartProbeOnce(t, ctx, binaryPath, arguments))
	}
	return summarizeColdStartSamples(samples)
}

func runColdStartProbeOnce(t *testing.T, ctx context.Context, binaryPath string, arguments []string) time.Duration {
	t.Helper()
	command := exec.CommandContext(ctx, binaryPath, arguments...)
	startedAt := time.Now()
	output, err := command.CombinedOutput()
	elapsed := time.Since(startedAt)
	if err != nil {
		t.Fatalf("run %s %s: %v\n%s", binaryPath, strings.Join(arguments, " "), err, output)
	}
	return elapsed
}

func summarizeColdStartSamples(samples []time.Duration) coldStartStats {
	if len(samples) == 0 {
		return coldStartStats{}
	}
	sorted := append([]time.Duration(nil), samples...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})

	var total time.Duration
	for _, sample := range sorted {
		total += sample
	}

	return coldStartStats{
		Min:    sorted[0],
		P50:    sorted[len(sorted)/2],
		Mean:   total / time.Duration(len(sorted)),
		Max:    sorted[len(sorted)-1],
		Sample: sorted,
	}
}

func formatDuration(duration time.Duration) string {
	return fmt.Sprintf("%.2fms", float64(duration.Microseconds())/1000)
}
