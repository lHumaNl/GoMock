package configloader

import "fmt"

type ConfigError struct {
	File   string
	Field  string
	Reason string
}

func (e ConfigError) Error() string {
	return fmt.Sprintf("%s: %s %s", e.File, e.Field, e.Reason)
}

func configError(file string, field string, reason string) error {
	return ConfigError{File: file, Field: field, Reason: reason}
}
