package app

import "fmt"

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
	return validateVerboseLimits(c.VerboseBodyLimit, c.VerbosePreviewLimit)
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
