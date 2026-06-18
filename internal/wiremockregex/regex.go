package wiremockregex

import (
	"regexp"
	"time"

	"github.com/dlclark/regexp2"
)

const matchTimeout = 100 * time.Millisecond

type Regex struct {
	standard *regexp.Regexp
	compat   *regexp2.Regexp
}

func Compile(expression string) (*Regex, error) {
	if compiled, err := regexp.Compile(expression); err == nil {
		return &Regex{standard: compiled}, nil
	}
	compiled, err := regexp2.Compile(expression, 0)
	if err != nil {
		return nil, err
	}
	compiled.MatchTimeout = matchTimeout
	return &Regex{compat: compiled}, nil
}

func Validate(expression string) error {
	_, err := Compile(expression)
	return err
}

func MatchString(expression string, value string) (bool, error) {
	compiled, err := Compile(expression)
	if err != nil {
		return false, err
	}
	return compiled.MatchString(value)
}

func (r *Regex) MatchString(value string) (bool, error) {
	if r.standard != nil {
		return r.standard.MatchString(value), nil
	}
	return r.compat.MatchString(value)
}
