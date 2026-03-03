package config

import (
	"errors"
	"flag"
	"fmt"
	"os"
)

const defaultConfigFile = ".tapbox.yaml"

type Config struct {
	HTTPTarget string
	HTTPListen string

	GRPCTarget string
	GRPCListen string

	SQLTarget string
	SQLListen string

	UIListen string

	MaxBodySize int
	MaxTraces   int
	ExplainDSN  string
	EnableGRPC  bool
	EnableSQL   bool
}

func Parse(args []string) (Config, error) {
	fs := flag.NewFlagSet("tapbox", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var configPath string
	c := Config{}

	fs.StringVar(&configPath, "config", "", "path to YAML config file (default: .tapbox.yaml if present)")

	fs.StringVar(&c.HTTPTarget, "http-target", "", "upstream HTTP server (required)")
	fs.StringVar(&c.HTTPListen, "http-listen", ":8080", "HTTP proxy listen address")

	fs.StringVar(&c.GRPCTarget, "grpc-target", "", "upstream gRPC server (empty = disabled)")
	fs.StringVar(&c.GRPCListen, "grpc-listen", ":9090", "gRPC proxy listen address")

	fs.StringVar(&c.SQLTarget, "sql-target", "", "upstream PostgreSQL server (empty = disabled)")
	fs.StringVar(&c.SQLListen, "sql-listen", ":5433", "SQL proxy listen address")

	fs.StringVar(&c.UIListen, "ui-listen", ":3080", "UI server listen address")

	fs.IntVar(&c.MaxBodySize, "max-body-size", 64*1024, "max request/response body capture size in bytes")
	fs.IntVar(&c.MaxTraces, "max-traces", 1000, "max traces to keep in memory")
	fs.StringVar(&c.ExplainDSN, "explain-dsn", "", "PostgreSQL DSN for EXPLAIN queries (defaults to sql-target)")

	if err := fs.Parse(args); err != nil {
		return Config{}, fmt.Errorf("parsing flags: %w", err)
	}

	explicit := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) {
		explicit[f.Name] = true
	})

	if configPath == "" && !explicit["config"] {
		if _, err := os.Stat(defaultConfigFile); err == nil {
			configPath = defaultConfigFile
		}
	}

	if configPath != "" {
		fc, err := loadFile(configPath)
		if err != nil {
			return Config{}, err
		}
		applyFileConfig(&c, fc, explicit)
	}

	if c.HTTPTarget == "" {
		return Config{}, errors.New("--http-target is required")
	}

	c.EnableGRPC = c.GRPCTarget != ""
	c.EnableSQL = c.SQLTarget != ""

	if c.ExplainDSN == "" && c.SQLTarget != "" {
		c.ExplainDSN = "postgres://" + c.SQLTarget + "?sslmode=disable"
	}

	return c, nil
}
