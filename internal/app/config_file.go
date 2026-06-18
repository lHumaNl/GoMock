package app

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/titanous/json5"
	"gopkg.in/yaml.v3"
)

type rawConfig struct {
	Root                *string       `json:"root" yaml:"root"`
	Host                *string       `json:"host" yaml:"host"`
	Port                *int          `json:"port" yaml:"port"`
	MetricsPort         *int          `json:"metricsPort" yaml:"metricsPort"`
	LogLevel            *string       `json:"logLevel" yaml:"logLevel"`
	Strict              *bool         `json:"strict" yaml:"strict"`
	Verbose             *string       `json:"verbose" yaml:"verbose"`
	VerboseBodyLimit    *int          `json:"verboseBodyLimit" yaml:"verboseBodyLimit"`
	VerbosePreviewLimit *int          `json:"verbosePreviewLimit" yaml:"verbosePreviewLimit"`
	VerboseRedact       *bool         `json:"verboseRedact" yaml:"verboseRedact"`
	TLS                 *rawTLSConfig `json:"tls" yaml:"tls"`
}

type rawTLSConfig struct {
	Enabled    *bool   `json:"enabled" yaml:"enabled"`
	CertFile   *string `json:"certFile" yaml:"certFile"`
	KeyFile    *string `json:"keyFile" yaml:"keyFile"`
	MinVersion *string `json:"minVersion" yaml:"minVersion"`
}

func LoadConfigFile(path string, base Config) (Config, error) {
	raw, err := loadRawConfig(path)
	if err != nil {
		return Config{}, err
	}
	return applyRawConfig(base, raw), nil
}

func loadRawConfig(path string) (rawConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return rawConfig{}, fmt.Errorf("read config file: %w", err)
	}
	return decodeRawConfig(path, data)
}

func decodeRawConfig(path string, data []byte) (rawConfig, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json", ".json5":
		return decodeJSONConfig(data)
	case ".yaml", ".yml":
		return decodeYAMLConfig(data)
	default:
		return rawConfig{}, fmt.Errorf("unsupported config extension")
	}
}

func decodeJSONConfig(data []byte) (rawConfig, error) {
	var raw rawConfig
	decoder := json5.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&raw); err != nil {
		return rawConfig{}, err
	}
	return raw, nil
}

func decodeYAMLConfig(data []byte) (rawConfig, error) {
	var raw rawConfig
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&raw); err != nil {
		return rawConfig{}, err
	}
	return raw, nil
}

func applyRawConfig(config Config, raw rawConfig) Config {
	applyCoreConfig(&config, raw)
	applyVerboseConfig(&config, raw)
	applyTLSConfig(&config.TLS, raw.TLS)
	return config
}

func applyCoreConfig(config *Config, raw rawConfig) {
	applyString(raw.Root, &config.Root)
	applyString(raw.Host, &config.Host)
	applyInt(raw.Port, &config.Port)
	applyInt(raw.MetricsPort, &config.MetricsPort)
	applyString(raw.LogLevel, &config.LogLevel)
	applyBool(raw.Strict, &config.Strict)
}

func applyVerboseConfig(config *Config, raw rawConfig) {
	applyString(raw.Verbose, &config.Verbose)
	applyInt(raw.VerboseBodyLimit, &config.VerboseBodyLimit)
	applyInt(raw.VerbosePreviewLimit, &config.VerbosePreviewLimit)
	applyBool(raw.VerboseRedact, &config.VerboseRedact)
}

func applyTLSConfig(config *TLSConfig, raw *rawTLSConfig) {
	if raw == nil {
		return
	}
	applyBool(raw.Enabled, &config.Enabled)
	applyString(raw.CertFile, &config.CertFile)
	applyString(raw.KeyFile, &config.KeyFile)
	applyString(raw.MinVersion, &config.MinVersion)
}

func applyString(source *string, target *string) {
	if source != nil {
		*target = *source
	}
}

func applyInt(source *int, target *int) {
	if source != nil {
		*target = *source
	}
}

func applyBool(source *bool, target *bool) {
	if source != nil {
		*target = *source
	}
}
