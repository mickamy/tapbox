package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickamy/tapbox/internal/config"
)

func writeConfigFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writing config file: %v", err)
	}
	return path
}

func TestParse_WithHTTPTarget(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	cfg, err := config.Parse([]string{"--http-target", "http://localhost:3000"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.HTTPTarget != "http://localhost:3000" {
		t.Errorf("HTTPTarget = %q, want %q", cfg.HTTPTarget, "http://localhost:3000")
	}
	if cfg.HTTPListen != ":8080" {
		t.Errorf("HTTPListen = %q, want %q", cfg.HTTPListen, ":8080")
	}
	if cfg.UIListen != ":3080" {
		t.Errorf("UIListen = %q, want %q", cfg.UIListen, ":3080")
	}
}

func TestParse_MissingHTTPTarget(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	_, err := config.Parse([]string{})
	if err == nil {
		t.Fatal("expected error for missing --http-target")
	}
}

func TestParse_GRPCEnabled(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	cfg, err := config.Parse([]string{
		"--http-target", "http://localhost:3000",
		"--grpc-target", "localhost:50051",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.EnableGRPC {
		t.Error("EnableGRPC should be true when --grpc-target is set")
	}
	if cfg.GRPCTarget != "localhost:50051" {
		t.Errorf("GRPCTarget = %q, want %q", cfg.GRPCTarget, "localhost:50051")
	}
	if cfg.GRPCListen != ":9090" {
		t.Errorf("GRPCListen = %q, want %q", cfg.GRPCListen, ":9090")
	}
}

func TestParse_SQLEnabled(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	cfg, err := config.Parse([]string{
		"--http-target", "http://localhost:3000",
		"--sql-target", "localhost:5432",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.EnableSQL {
		t.Error("EnableSQL should be true when --sql-target is set")
	}
	if cfg.SQLTarget != "localhost:5432" {
		t.Errorf("SQLTarget = %q, want %q", cfg.SQLTarget, "localhost:5432")
	}
	if cfg.SQLListen != ":5433" {
		t.Errorf("SQLListen = %q, want %q", cfg.SQLListen, ":5433")
	}
	if cfg.ExplainDSN != "postgres://localhost:5432?sslmode=disable" {
		t.Errorf("ExplainDSN = %q, want auto-generated DSN", cfg.ExplainDSN)
	}
}

func TestParse_GRPCDisabledByDefault(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	cfg, err := config.Parse([]string{"--http-target", "http://localhost:3000"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.EnableGRPC {
		t.Error("EnableGRPC should be false when --grpc-target is not set")
	}
	if cfg.EnableSQL {
		t.Error("EnableSQL should be false when --sql-target is not set")
	}
}

func TestParse_CustomPorts(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	cfg, err := config.Parse([]string{
		"--http-target", "http://localhost:3000",
		"--http-listen", ":9999",
		"--ui-listen", ":4000",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.HTTPListen != ":9999" {
		t.Errorf("HTTPListen = %q, want %q", cfg.HTTPListen, ":9999")
	}
	if cfg.UIListen != ":4000" {
		t.Errorf("UIListen = %q, want %q", cfg.UIListen, ":4000")
	}
}

func TestParse_MaxBodySize(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	cfg, err := config.Parse([]string{
		"--http-target", "http://localhost:3000",
		"--max-body-size", "1024",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.MaxBodySize != 1024 {
		t.Errorf("MaxBodySize = %d, want %d", cfg.MaxBodySize, 1024)
	}
}

func TestParse_DefaultMaxBodySize(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	cfg, err := config.Parse([]string{"--http-target", "http://localhost:3000"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.MaxBodySize != 64*1024 {
		t.Errorf("MaxBodySize = %d, want %d", cfg.MaxBodySize, 64*1024)
	}
}

func TestParse_ExplainDSNOverride(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	explainDSN := "postgres://localhost:5432/mydb?sslmode=disable"
	cfg, err := config.Parse([]string{
		"--http-target", "http://localhost:3000",
		"--sql-target", "localhost:5432",
		"--explain-dsn", explainDSN,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ExplainDSN != explainDSN {
		t.Errorf("ExplainDSN = %q, want explicit DSN", cfg.ExplainDSN)
	}
}

func TestParse_MaxTracesZero(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	_, err := config.Parse([]string{
		"--http-target", "http://localhost:3000",
		"--max-traces", "0",
	})
	if err == nil {
		t.Fatal("expected error for --max-traces 0")
	}
}

func TestParse_MaxTracesNegative(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	_, err := config.Parse([]string{
		"--http-target", "http://localhost:3000",
		"--max-traces", "-5",
	})
	if err == nil {
		t.Fatal("expected error for negative --max-traces")
	}
}

func TestParse_InvalidFlag(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	_, err := config.Parse([]string{"--nonexistent-flag", "value"})
	if err == nil {
		t.Fatal("expected error for invalid flag")
	}
}

func TestParse_ConfigFileOnly(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	path := writeConfigFile(t, `
http:
  target: "http://localhost:3000"
  listen: ":8081"
grpc:
  target: "localhost:50051"
  listen: ":9091"
sql:
  target: "localhost:5432"
  listen: ":5434"
ui:
  listen: ":3081"
max_body_size: 65536
max_traces: 500
explain_dsn: "postgres://custom:5432/db"
`)
	cfg, err := config.Parse([]string{"--config", path})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.HTTPTarget != "http://localhost:3000" {
		t.Errorf("HTTPTarget = %q, want %q", cfg.HTTPTarget, "http://localhost:3000")
	}
	if cfg.HTTPListen != ":8081" {
		t.Errorf("HTTPListen = %q, want %q", cfg.HTTPListen, ":8081")
	}
	if cfg.GRPCTarget != "localhost:50051" {
		t.Errorf("GRPCTarget = %q, want %q", cfg.GRPCTarget, "localhost:50051")
	}
	if cfg.GRPCListen != ":9091" {
		t.Errorf("GRPCListen = %q, want %q", cfg.GRPCListen, ":9091")
	}
	if cfg.SQLTarget != "localhost:5432" {
		t.Errorf("SQLTarget = %q, want %q", cfg.SQLTarget, "localhost:5432")
	}
	if cfg.SQLListen != ":5434" {
		t.Errorf("SQLListen = %q, want %q", cfg.SQLListen, ":5434")
	}
	if cfg.UIListen != ":3081" {
		t.Errorf("UIListen = %q, want %q", cfg.UIListen, ":3081")
	}
	if cfg.MaxBodySize != 65536 {
		t.Errorf("MaxBodySize = %d, want %d", cfg.MaxBodySize, 65536)
	}
	if cfg.MaxTraces != 500 {
		t.Errorf("MaxTraces = %d, want %d", cfg.MaxTraces, 500)
	}
	if cfg.ExplainDSN != "postgres://custom:5432/db" {
		t.Errorf("ExplainDSN = %q, want %q", cfg.ExplainDSN, "postgres://custom:5432/db")
	}
	if !cfg.EnableGRPC {
		t.Error("EnableGRPC should be true")
	}
	if !cfg.EnableSQL {
		t.Error("EnableSQL should be true")
	}
}

func TestParse_ConfigFileMissingHTTPTarget(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	path := writeConfigFile(t, `
grpc:
  target: "localhost:50051"
`)
	_, err := config.Parse([]string{"--config", path})
	if err == nil {
		t.Fatal("expected error for missing http.target")
	}
}

func TestParse_FlagOverridesYAML(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	path := writeConfigFile(t, `
http:
  target: "http://yaml-target:3000"
  listen: ":8081"
max_body_size: 9999
`)
	cfg, err := config.Parse([]string{
		"--config", path,
		"--http-target", "http://flag-target:3000",
		"--max-body-size", "1234",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.HTTPTarget != "http://flag-target:3000" {
		t.Errorf("HTTPTarget = %q, want flag value", cfg.HTTPTarget)
	}
	if cfg.HTTPListen != ":8081" {
		t.Errorf("HTTPListen = %q, want YAML value", cfg.HTTPListen)
	}
	if cfg.MaxBodySize != 1234 {
		t.Errorf("MaxBodySize = %d, want flag value 1234", cfg.MaxBodySize)
	}
}

func TestParse_YAMLOverridesDefault(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	path := writeConfigFile(t, `
http:
  target: "http://localhost:3000"
  listen: ":9999"
max_traces: 42
`)
	cfg, err := config.Parse([]string{"--config", path})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.HTTPListen != ":9999" {
		t.Errorf("HTTPListen = %q, want YAML value %q", cfg.HTTPListen, ":9999")
	}
	if cfg.MaxTraces != 42 {
		t.Errorf("MaxTraces = %d, want YAML value 42", cfg.MaxTraces)
	}
}

func TestParse_ConfigFilePartial(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	path := writeConfigFile(t, `
http:
  target: "http://localhost:3000"
`)
	cfg, err := config.Parse([]string{"--config", path})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.HTTPListen != ":8080" {
		t.Errorf("HTTPListen = %q, want default %q", cfg.HTTPListen, ":8080")
	}
	if cfg.GRPCListen != ":9090" {
		t.Errorf("GRPCListen = %q, want default %q", cfg.GRPCListen, ":9090")
	}
	if cfg.MaxBodySize != 64*1024 {
		t.Errorf("MaxBodySize = %d, want default %d", cfg.MaxBodySize, 64*1024)
	}
}

func TestParse_ConfigFileWithSQLAutoExplainDSN(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	path := writeConfigFile(t, `
http:
  target: "http://localhost:3000"
sql:
  target: "localhost:5432"
`)
	cfg, err := config.Parse([]string{"--config", path})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ExplainDSN != "postgres://localhost:5432?sslmode=disable" {
		t.Errorf("ExplainDSN = %q, want auto-generated DSN", cfg.ExplainDSN)
	}
}

func TestParse_ConfigFileNotFound(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	_, err := config.Parse([]string{"--config", "/nonexistent/path/config.yaml"})
	if err == nil {
		t.Fatal("expected error for missing config file")
	}
}

func TestParse_ConfigFileInvalidYAML(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	path := writeConfigFile(t, `{{{invalid yaml!!!`)
	_, err := config.Parse([]string{"--config", path})
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestParse_ConfigFileUnknownKey(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	path := writeConfigFile(t, `
http:
  target: "http://localhost:3000"
unknown_key: "value"
`)
	_, err := config.Parse([]string{"--config", path})
	if err == nil {
		t.Fatal("expected error for unknown key in config file")
	}
}

func TestParse_FlagExplicitlySetToDefault(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	path := writeConfigFile(t, `
http:
  target: "http://localhost:3000"
  listen: ":9999"
`)
	cfg, err := config.Parse([]string{
		"--config", path,
		"--http-listen", ":8080",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.HTTPListen != ":8080" {
		t.Errorf("HTTPListen = %q, want flag value %q (even though it matches default)", cfg.HTTPListen, ":8080")
	}
}

func TestParse_NoConfigFlag(t *testing.T) {
	t.Parallel()
	_ = t.Context()

	cfg, err := config.Parse([]string{"--http-target", "http://localhost:3000"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.HTTPTarget != "http://localhost:3000" {
		t.Errorf("HTTPTarget = %q, want %q", cfg.HTTPTarget, "http://localhost:3000")
	}
}

func TestParse_AutoLoadDotTapboxYAML(t *testing.T) { //nolint:paralleltest // t.Chdir is incompatible with t.Parallel
	dir := t.TempDir()
	content := `
http:
  target: "http://auto-loaded:3000"
  listen: ":8181"
`
	if err := os.WriteFile(filepath.Join(dir, ".tapbox.yaml"), []byte(content), 0o600); err != nil {
		t.Fatalf("writing .tapbox.yaml: %v", err)
	}

	t.Chdir(dir)

	cfg, err := config.Parse([]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.HTTPTarget != "http://auto-loaded:3000" {
		t.Errorf("HTTPTarget = %q, want %q", cfg.HTTPTarget, "http://auto-loaded:3000")
	}
	if cfg.HTTPListen != ":8181" {
		t.Errorf("HTTPListen = %q, want %q", cfg.HTTPListen, ":8181")
	}
}
