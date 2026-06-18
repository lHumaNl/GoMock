package app

import (
	"crypto/tls"
	"fmt"
	"os"
)

const (
	DefaultRoot                = "."
	DefaultHost                = "0.0.0.0"
	DefaultPort                = 8080
	DefaultLogLevel            = "info"
	DefaultVersion             = "dev"
	DefaultCommit              = "unknown"
	DefaultVerbose             = "off"
	DefaultVerboseBodyLimit    = 4096
	DefaultVerbosePreviewLimit = 160
	DefaultTLSMinVersion       = "1.2"
)

type Config struct {
	Root                string
	Host                string
	Port                int
	MetricsPort         int
	LogLevel            string
	Strict              bool
	Version             string
	Commit              string
	Verbose             string
	VerboseBodyLimit    int
	VerbosePreviewLimit int
	VerboseRedact       bool
	TLS                 TLSConfig
}

type TLSConfig struct {
	Enabled    bool
	CertFile   string
	KeyFile    string
	MinVersion string
}

func DefaultConfig() Config {
	return Config{
		Root:                DefaultRoot,
		Host:                DefaultHost,
		Port:                DefaultPort,
		LogLevel:            DefaultLogLevel,
		Version:             DefaultVersion,
		Commit:              DefaultCommit,
		Verbose:             DefaultVerbose,
		VerboseBodyLimit:    DefaultVerboseBodyLimit,
		VerbosePreviewLimit: DefaultVerbosePreviewLimit,
		TLS: TLSConfig{
			MinVersion: DefaultTLSMinVersion,
		},
	}
}

func (c Config) Validate() error {
	if c.Root == "" {
		return fmt.Errorf("root is required")
	}
	if err := validatePort("port", c.Port, true); err != nil {
		return err
	}
	if err := validatePort("metrics-port", c.MetricsPort, false); err != nil {
		return err
	}
	if err := validateLogLevel(c.LogLevel); err != nil {
		return err
	}
	if err := validateVerbose(c.Verbose); err != nil {
		return err
	}
	if err := validateVerboseLimits(c.VerboseBodyLimit, c.VerbosePreviewLimit); err != nil {
		return err
	}
	return validateTLSConfig(c.TLS)
}

func (c Config) TLSMinVersion() uint16 {
	if c.TLS.MinVersion == "1.3" {
		return tls.VersionTLS13
	}
	return tls.VersionTLS12
}

func validatePort(name string, value int, required bool) error {
	if !required && value == 0 {
		return nil
	}
	if value < 1 || value > 65535 {
		return fmt.Errorf("%s must be between 1 and 65535", name)
	}
	return nil
}

func validateVerbose(value string) error {
	switch value {
	case "", "off", "summary", "full":
		return nil
	default:
		return fmt.Errorf("verbose must be off, summary, or full")
	}
}

func validateVerboseLimits(bodyLimit int, previewLimit int) error {
	if bodyLimit <= 0 {
		return fmt.Errorf("verbose-body-limit must be positive")
	}
	if previewLimit <= 0 {
		return fmt.Errorf("verbose-preview-limit must be positive")
	}
	return nil
}

func validateLogLevel(value string) error {
	switch value {
	case "debug", "info", "warn", "error":
		return nil
	default:
		return fmt.Errorf("log-level must be debug, info, warn, or error")
	}
}

func validateTLSConfig(config TLSConfig) error {
	if err := validateTLSMinVersion(config.MinVersion); err != nil {
		return err
	}
	if !config.Enabled {
		return nil
	}
	if config.CertFile == "" {
		return fmt.Errorf("tls-cert-file is required when TLS is enabled")
	}
	if config.KeyFile == "" {
		return fmt.Errorf("tls-key-file is required when TLS is enabled")
	}
	return validateTLSFiles(config.CertFile, config.KeyFile)
}

func validateTLSMinVersion(value string) error {
	switch value {
	case "", "1.2", "1.3":
		return nil
	default:
		return fmt.Errorf("tls-min-version must be 1.2 or 1.3")
	}
}

func validateTLSFiles(certFile string, keyFile string) error {
	if err := validateReadableFile("tls-cert-file", certFile); err != nil {
		return err
	}
	return validateReadableFile("tls-key-file", keyFile)
}

func validateReadableFile(name string, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("%s must be readable: %w", name, err)
	}
	info, err := file.Stat()
	closeErr := file.Close()
	if err != nil {
		return fmt.Errorf("%s must be readable: %w", name, err)
	}
	if closeErr != nil {
		return fmt.Errorf("%s must be readable: %w", name, closeErr)
	}
	if info.IsDir() {
		return fmt.Errorf("%s must be a file", name)
	}
	return nil
}
