package app

import "fmt"

const (
	DefaultRoot     = "."
	DefaultHost     = "0.0.0.0"
	DefaultPort     = 8080
	DefaultLogLevel = "info"
	DefaultVersion  = "dev"
	DefaultCommit   = "unknown"
)

type Config struct {
	Root        string
	Host        string
	Port        int
	MetricsPort int
	LogLevel    string
	Strict      bool
	Version     string
	Commit      string
}

func DefaultConfig() Config {
	return Config{
		Root:     DefaultRoot,
		Host:     DefaultHost,
		Port:     DefaultPort,
		LogLevel: DefaultLogLevel,
		Version:  DefaultVersion,
		Commit:   DefaultCommit,
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
	return validateLogLevel(c.LogLevel)
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

func validateLogLevel(value string) error {
	switch value {
	case "debug", "info", "warn", "error":
		return nil
	default:
		return fmt.Errorf("log-level must be debug, info, warn, or error")
	}
}
