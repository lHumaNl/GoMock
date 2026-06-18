package responsetemplate

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strconv"
	"strings"
)

const (
	defaultRandomValueLength = 32
	randomTypeAlphabetic     = "ALPHABETIC"
	randomTypeAlphanumeric   = "ALPHANUMERIC"
	randomTypeNumeric        = "NUMERIC"
)

const (
	alphabeticChars   = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	numericChars      = "0123456789"
	alphanumericChars = alphabeticChars + numericChars
)

func randomInt(params map[string]string) (int, error) {
	lower, err := intParam(params, "lower", 0)
	if err != nil {
		return 0, err
	}
	upper, err := intParam(params, "upper", lower)
	if err != nil {
		return 0, err
	}
	if lower > upper {
		return 0, fmt.Errorf("randomInt lower must be less than or equal to upper")
	}
	return secureInt(lower, upper)
}

func randomValue(params map[string]string) (string, error) {
	length, err := intParam(params, "length", defaultRandomValueLength)
	if err != nil {
		return "", err
	}
	if length < 0 {
		return "", fmt.Errorf("randomValue length must be non-negative")
	}
	return randomString(length, randomCharset(params["type"]))
}

func randomCharset(kind string) string {
	switch strings.ToUpper(kind) {
	case randomTypeAlphabetic:
		return alphabeticChars
	case randomTypeNumeric:
		return numericChars
	default:
		return alphanumericChars
	}
}

func randomString(length int, charset string) (string, error) {
	var builder strings.Builder
	for builder.Len() < length {
		index, err := secureInt(0, len(charset)-1)
		if err != nil {
			return "", err
		}
		builder.WriteByte(charset[index])
	}
	return builder.String(), nil
}

func secureInt(lower int, upper int) (int, error) {
	span := big.NewInt(int64(upper - lower + 1))
	value, err := rand.Int(rand.Reader, span)
	if err != nil {
		return 0, err
	}
	return lower + int(value.Int64()), nil
}

func intParam(params map[string]string, name string, fallback int) (int, error) {
	value := params[name]
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer", name)
	}
	return parsed, nil
}
