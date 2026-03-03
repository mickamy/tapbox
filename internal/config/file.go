package config

import (
	"errors"
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

type httpConfig struct {
	Target *string `yaml:"target"`
	Listen *string `yaml:"listen"`
}

type grpcConfig struct {
	Target *string `yaml:"target"`
	Listen *string `yaml:"listen"`
}

type sqlConfig struct {
	Target *string `yaml:"target"`
	Listen *string `yaml:"listen"`
}

type uiConfig struct {
	Listen *string `yaml:"listen"`
}

type fileConfig struct {
	HTTP        *httpConfig `yaml:"http"`
	GRPC        *grpcConfig `yaml:"grpc"`
	SQL         *sqlConfig  `yaml:"sql"`
	UI          *uiConfig   `yaml:"ui"`
	MaxBodySize *int        `yaml:"max_body_size"`
	MaxTraces   *int        `yaml:"max_traces"`
	ExplainDSN  *string     `yaml:"explain_dsn"`
}

func loadFile(path string) (fileConfig, error) {
	f, err := os.Open(path) //nolint:gosec // path is controlled by the user via --config flag
	if err != nil {
		return fileConfig{}, fmt.Errorf("opening config file: %w", err)
	}
	defer func() { _ = f.Close() }()

	var fc fileConfig
	dec := yaml.NewDecoder(f)
	dec.KnownFields(true)
	if err := dec.Decode(&fc); err != nil {
		if errors.Is(err, io.EOF) {
			return fileConfig{}, nil
		}
		return fileConfig{}, fmt.Errorf("decoding config file: %w", err)
	}
	return fc, nil
}

func applyFileConfig(c *Config, fc fileConfig, explicit map[string]bool) {
	setStr := func(flag string, dst *string, val *string) {
		if val != nil && !explicit[flag] {
			*dst = *val
		}
	}
	setInt := func(flag string, dst *int, val *int) {
		if val != nil && !explicit[flag] {
			*dst = *val
		}
	}

	if fc.HTTP != nil {
		setStr("http-target", &c.HTTPTarget, fc.HTTP.Target)
		setStr("http-listen", &c.HTTPListen, fc.HTTP.Listen)
	}
	if fc.GRPC != nil {
		setStr("grpc-target", &c.GRPCTarget, fc.GRPC.Target)
		setStr("grpc-listen", &c.GRPCListen, fc.GRPC.Listen)
	}
	if fc.SQL != nil {
		setStr("sql-target", &c.SQLTarget, fc.SQL.Target)
		setStr("sql-listen", &c.SQLListen, fc.SQL.Listen)
	}
	if fc.UI != nil {
		setStr("ui-listen", &c.UIListen, fc.UI.Listen)
	}
	setInt("max-body-size", &c.MaxBodySize, fc.MaxBodySize)
	setInt("max-traces", &c.MaxTraces, fc.MaxTraces)
	setStr("explain-dsn", &c.ExplainDSN, fc.ExplainDSN)
}
